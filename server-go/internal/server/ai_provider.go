package server

import (
	"context"

	"buzzcontrol/internal/config"
)

// keyValidationResult is the outcome of aiProvider.ValidateKey — a RESULT,
// not an error: invalid_key and unreachable are nominal validation outcomes,
// not failures of our own code (contract ai-key-validation.md §3, §6). The
// existing generation error types (*aiTimeoutError/*aiUpstreamError/
// *aiRateLimitError) stay reserved to the Generate path and are never
// returned here.
type keyValidationResult struct {
	// Result is exactly one of "valid" | "invalid_key" | "unreachable"
	// (contract §3).
	Result string
	// HTTPStatus is the raw upstream HTTP status, 0 if no response was ever
	// received (network/DNS/TLS error, timeout).
	HTTPStatus int
	// Detail is an optional, already-sanitized (sanitizeUpstreamMessage)
	// upstream message — never the API key, never a raw response body
	// (contract §8).
	Detail string
}

// aiProvider abstracts the LLM call (contract ai-multi-provider.md §5). One
// method for generation, one for schema adaptation to the provider's
// dialect, one for identification (used in logs and the AI_GENERATION_PROGRESS
// PROVIDER field), one for key validation (contract ai-key-validation.md §6).
// Deliberately minimal — it doesn't prejudge a plugin architecture and
// doesn't preclude one (consigne #135).
type aiProvider interface {
	// Generate calls the provider with prompt/schema and returns the raw
	// text content (expected to unmarshal as {"questions": [...]}) or an
	// error already classified as *aiTimeoutError, *aiRateLimitError, or
	// *aiUpstreamError (ai-generation.md §3, ai-multi-provider.md §3).
	Generate(ctx context.Context, cfg config.AIConfig, prompt string, schema map[string]any) (string, error)
	// AdaptSchema lets the provider adjust the schema to its own dialect
	// (contract §7). Anthropic's is a no-op; Groq's strips the CATEGORY/
	// DIFFICULTY enum from every anyOf branch (issue #142 — see
	// groqProvider.AdaptSchema for the confirmed root cause). The hook
	// existing before #142 needed it is exactly why fixing this required no
	// interface change.
	AdaptSchema(schema map[string]any) map[string]any
	// Name identifies the provider ("anthropic" | "groq") for logs and the
	// PROVIDER field of AI_GENERATION_PROGRESS.
	Name() string
	// ValidateKey verifies authentication with the provider (contract
	// ai-key-validation.md §2) via a zero-token call (GET /models), without
	// ever calling Generate. key empty => the effective stored key
	// (EffectiveAnthropicAPIKey/EffectiveGroqAPIKey, env var takes priority).
	ValidateKey(ctx context.Context, cfg config.AIConfig, key string) keyValidationResult
}

// anthropicProvider wraps the existing (#8) Anthropic call as an aiProvider,
// with NO behavior change — same client, same streaming call, same error
// classification (contract ai-multi-provider.md §5: "Aucune régression
// fonctionnelle attendue sur le chemin anthropic").
type anthropicProvider struct{ h *HTTPServer }

func (p *anthropicProvider) Generate(ctx context.Context, cfg config.AIConfig, prompt string, schema map[string]any) (string, error) {
	return p.h.generateViaAnthropic(ctx, cfg, prompt, schema)
}

// AdaptSchema is a no-op for Anthropic: the schema built by buildQuestionSchema
// already satisfies Anthropic's structured-output rules (unchanged from #8).
func (p *anthropicProvider) AdaptSchema(schema map[string]any) map[string]any { return schema }

func (p *anthropicProvider) Name() string { return "anthropic" }

// ValidateKey checks the given (or effective stored) key against Anthropic's
// GET /v1/models — see ai_validate.go for the request/classification logic
// shared with groqProvider.ValidateKey.
func (p *anthropicProvider) ValidateKey(ctx context.Context, cfg config.AIConfig, key string) keyValidationResult {
	if key == "" {
		key = cfg.EffectiveAnthropicAPIKey()
	}
	return validateAnthropicKey(ctx, key)
}

// groqProvider wraps the Groq stdlib client (ai_groq.go) as an aiProvider.
type groqProvider struct{ h *HTTPServer }

func (p *groqProvider) Generate(ctx context.Context, cfg config.AIConfig, prompt string, schema map[string]any) (string, error) {
	return p.h.generateViaGroq(ctx, cfg, prompt, schema)
}

// groqAnyOfDiscriminatorFields lists the top-level anyOf-branch properties
// whose "enum" constraint AdaptSchema strips (issue #142) — kept as
// "type":"string", still required, just no longer schema-restricted to a
// value set for this provider. validateGeneratedQuestions (ai_generator.go,
// extended alongside this fix) independently checks CATEGORY (contract §5.1)
// and now DIFFICULTY too, so the actual guarantee (only requested
// categories/difficulties reach question.json) is unchanged for Groq output
// — only the layer enforcing it moves from schema to server-side validation.
var groqAnyOfDiscriminatorFields = []string{"CATEGORY", "DIFFICULTY"}

// AdaptSchema fixes issue #142 (root cause confirmed by live-testing against
// the real Groq API, 2026-08-07 — see _work/handoff/dev-backend-groq-schema-
// bug-*.md for the repro/fix transcript): Groq's strict-mode anyOf validator
// treats ANY required property carrying an enum/const constraint, present in
// every branch, as a candidate "discriminator" — without checking whether
// the branches' value sets actually differ. buildQuestionSchema's 5-branch
// anyOf has THREE such properties: TYPE (a per-branch "const", the intended
// discriminator) and CATEGORY/DIFFICULTY (an "enum" with the SAME values in
// every branch — contributed by the shared "common" property set, they
// can't discriminate anything since they don't vary across branches, but
// Groq's validator doesn't check that before flagging them too). Three
// candidates → Groq rejects the whole schema: "anyOf disambiguation failed:
// discriminator: multiple candidate properties CATEGORY, DIFFICULTY, TYPE
// [discriminator_multiple_candidates]" — reproduced byte-for-byte against
// the real API before this fix, and confirmed resolved (200, valid question
// generated) after it, for all 5 question types (SPEEDY/QCM/MEMORY/MEMOTION/
// ARDOISE).
//
// Fix: strip "enum" from CATEGORY/DIFFICULTY in every anyOf branch —
// MOTION_CARDS' own nested per-card DIFFICULTY (an unrelated integer field
// one level deeper inside MEMOTION's branch, never part of this ambiguity)
// is untouched. TYPE's const becomes the sole discriminator candidate.
//
// Anthropic is unaffected by construction: anthropicProvider.AdaptSchema
// (above) remains a no-op, a completely separate code path never touched by
// this fix — buildQuestionSchema itself, the contract §5 schema shape, and
// AdaptSchema's Anthropic branch are all unchanged.
//
// Supersedes the previous version of this comment (T0.1 calibration,
// 2026-08-06): that calibration ran BEFORE ARDOISE joined the anyOf as a 5th
// branch (#137 Batch 2a) and so never actually exercised the schema shape
// that triggers this ambiguity — "accepted as-is" was true of the 4-branch
// schema it tested, not of the 5-branch one that shipped afterward.
func (p *groqProvider) AdaptSchema(schema map[string]any) map[string]any {
	branches, ok := groqAnyOfBranches(schema)
	if !ok {
		return schema // unexpected shape — fail open, Groq's own validator will report it
	}
	for _, branch := range branches {
		branchMap, ok := branch.(map[string]any)
		if !ok {
			continue
		}
		props, ok := branchMap["properties"].(map[string]any)
		if !ok {
			continue
		}
		for _, field := range groqAnyOfDiscriminatorFields {
			if fieldSchema, ok := props[field].(map[string]any); ok {
				delete(fieldSchema, "enum")
			}
		}
	}
	return schema
}

// groqAnyOfBranches navigates buildQuestionSchema's fixed shape
// (properties.questions.items.anyOf) down to the branch list — factored out
// so AdaptSchema can fail open (return the schema unmodified) on any
// unexpected structure instead of panicking on a type assertion.
func groqAnyOfBranches(schema map[string]any) ([]any, bool) {
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil, false
	}
	questions, ok := props["questions"].(map[string]any)
	if !ok {
		return nil, false
	}
	items, ok := questions["items"].(map[string]any)
	if !ok {
		return nil, false
	}
	anyOf, ok := items["anyOf"].([]any)
	if !ok {
		return nil, false
	}
	return anyOf, true
}

func (p *groqProvider) Name() string { return "groq" }

// ValidateKey checks the given (or effective stored) key against Groq's
// GET /openai/v1/models — see ai_validate.go for the request/classification
// logic shared with anthropicProvider.ValidateKey.
func (p *groqProvider) ValidateKey(ctx context.Context, cfg config.AIConfig, key string) keyValidationResult {
	if key == "" {
		key = cfg.EffectiveGroqAPIKey()
	}
	return validateGroqKey(ctx, key)
}

// selectProvider returns the aiProvider for cfg.Provider, defaulting to
// Anthropic for "" or any unrecognized value (contract §5: default "anthropic").
func (h *HTTPServer) selectProvider(cfg config.AIConfig) aiProvider {
	if cfg.Provider == "groq" {
		return &groqProvider{h: h}
	}
	return &anthropicProvider{h: h}
}

// providerAPIKeyConfigured reports whether the currently selected provider
// has a usable key — the single source of truth for the "no_api_key" check
// (contract ai-multi-provider.md §9, security finding M3: must read strictly
// the selected provider's key, never OR both). Considers BOTH sources
// (BUZZCONTROL_*_API_KEY env var or config.json, see EffectiveXxxAPIKeyConfigured,
// security incident 2026-08-07) — a PROD deployment configured entirely via
// environment variable, with an empty config.json field, must not be
// mistaken for "no key configured".
func providerAPIKeyConfigured(cfg config.AIConfig) bool {
	if cfg.Provider == "groq" {
		return cfg.EffectiveGroqAPIKeyConfigured()
	}
	return cfg.EffectiveAnthropicAPIKeyConfigured()
}
