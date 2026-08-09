package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
)

// POST /api/ai/validate-key (contract ai-key-validation.md §5) — validates an
// API key against its provider with a zero-token call, without persisting
// anything. Kept in its own file, deliberately separate from handleConfig
// (http.go): a dedicated endpoint makes "this handler never writes
// config.json" a structural property of the code (no access to config.Save
// from here) rather than something only branch discipline enforces —
// contract §4 explains the full arbitration.

// aiValidateTimeout is a hard 10s ceiling for this interactive path,
// deliberately NOT config.AIConfig.TimeoutSeconds (default 300s, meant for
// the background batched-generation job) — contract §2.
const aiValidateTimeout = 10 * time.Second

// aiValidateMaxBodyBytes bounds the request body: a provider name and one
// API key, not a document (contract §5).
const aiValidateMaxBodyBytes = 64 << 10

// anthropicKeyPrefix/groqKeyPrefix are the well-formedness prefixes checked
// BEFORE any network call, both here and in handleConfig's POST /config.json
// (http.go) — factored out to one place instead of duplicating the literal
// in both files (contract §5: "contrôle de préfixe... identique à celui de
// POST /config.json").
const (
	anthropicKeyPrefix = "sk-ant-"
	groqKeyPrefix      = "gsk_"
)

// anthropicBaseURLEnvVar/anthropicModelsPath/anthropicAPIVersion mirror the
// groqBaseURL/groqChatPath motif in ai_groq.go — ANTHROPIC_BASE_URL is
// honored explicitly here (unlike generateViaAnthropic, which relies on the
// Anthropic SDK reading it automatically) because this path is plain
// net/http stdlib, per contract §6: "les deux implémentations utilisent
// net/http stdlib... la validation classe sur le statut HTTP brut".
const (
	anthropicBaseURLEnvVar  = "ANTHROPIC_BASE_URL"
	anthropicDefaultBaseURL = "https://api.anthropic.com"
	anthropicModelsPath     = "/v1/models?limit=1"
	anthropicAPIVersion     = "2023-06-01"
	groqModelsPath          = "/openai/v1/models"
)

func anthropicBaseURL() string {
	if v := os.Getenv(anthropicBaseURLEnvVar); v != "" {
		return v
	}
	return anthropicDefaultBaseURL
}

// validateAnthropicKey performs the GET /v1/models?limit=1 call (contract
// §2) that classifies a key as valid/invalid_key/unreachable without
// consuming any token — Generate is never called on this path.
func validateAnthropicKey(ctx context.Context, key string) keyValidationResult {
	ctx, cancel := context.WithTimeout(ctx, aiValidateTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, anthropicBaseURL()+anthropicModelsPath, nil)
	if err != nil {
		return keyValidationResult{Result: "unreachable"}
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	return doKeyValidationRequest(req, func(body []byte) string {
		var envelope anthropicErrorEnvelope
		_ = json.Unmarshal(body, &envelope) // best-effort; malformed/non-JSON body just yields an empty envelope
		return envelope.Error.Message
	})
}

// validateGroqKey performs the GET /openai/v1/models call (contract §2),
// Groq's counterpart to validateAnthropicKey.
func validateGroqKey(ctx context.Context, key string) keyValidationResult {
	ctx, cancel := context.WithTimeout(ctx, aiValidateTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, groqBaseURL()+groqModelsPath, nil)
	if err != nil {
		return keyValidationResult{Result: "unreachable"}
	}
	req.Header.Set("Authorization", "Bearer "+key)

	return doKeyValidationRequest(req, func(body []byte) string {
		var envelope groqErrorEnvelope
		_ = json.Unmarshal(body, &envelope) // best-effort, same motif as classifyGroqError
		return envelope.Error.Message
	})
}

// doKeyValidationRequest issues the request and classifies the outcome —
// shared by both providers. No retry, no client-level Timeout (the request
// context already carries aiValidateTimeout's deadline, same motif as
// generateViaGroq). A network/DNS/TLS error or a timeout never reaches an
// HTTP status at all, which is exactly "unreachable" (contract §3): the key
// was neither confirmed nor refused.
func doKeyValidationRequest(req *http.Request, extractDetail func([]byte) string) keyValidationResult {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return keyValidationResult{Result: "unreachable"}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	result := classifyValidationStatus(resp.StatusCode)
	detail := ""
	if result != "valid" {
		detail = sanitizeUpstreamMessage(extractDetail(body))
	}
	return keyValidationResult{Result: result, HTTPStatus: resp.StatusCode, Detail: detail}
}

// classifyValidationStatus is the taxonomy of contract §3: exactly 200 is
// "valid", 401/403 are an explicit provider refusal ("invalid_key"), and
// EVERYTHING else — 5xx, 429, any other non-2xx — is "unreachable". 429 is
// deliberately NOT "invalid_key": a rate-limited call likely means auth
// passed, but nothing is confirmed (contract §3, "429 → unreachable,
// délibérément").
func classifyValidationStatus(statusCode int) string {
	switch statusCode {
	case http.StatusOK:
		return "valid"
	case http.StatusUnauthorized, http.StatusForbidden:
		return "invalid_key"
	default:
		return "unreachable"
	}
}

// aiValidateCooldown is the minimum spacing between two validation attempts,
// global to the whole server (contract §8: "1 validation / 2 s, global au
// serveur") — not per-caller, since the BuzzControl admin surface has no
// authentication to key a per-caller limit on in the first place.
const aiValidateCooldown = 2 * time.Second

// aiValidateCooldownState guards the single "last attempt" timestamp.
// tryAcquire does the "has the cooldown elapsed?" check and the "record this
// attempt" write under one lock acquisition — no TOCTOU gap for two
// concurrent POSTs to both slip through, same motif as
// aiJobRegistry.tryStart (ai_job.go).
type aiValidateCooldownState struct {
	mu     sync.Mutex
	lastAt time.Time
}

var globalAIValidateCooldown = &aiValidateCooldownState{}

func (c *aiValidateCooldownState) tryAcquire(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.lastAt.IsZero() && now.Sub(c.lastAt) < aiValidateCooldown {
		return false
	}
	c.lastAt = now
	return true
}

// validateKeyRequest is the POST /api/ai/validate-key request body (contract
// §5).
type validateKeyRequest struct {
	Provider string `json:"provider"`
	// APIKey empty => validate the effective stored key
	// (EffectiveAnthropicAPIKey/EffectiveGroqAPIKey).
	APIKey string `json:"api_key"`
}

// validateKeyResponse is the POST /api/ai/validate-key response body
// (contract §5) — ALWAYS HTTP 200 on a completed validation, even when
// Result is "invalid_key": the verdict lives in the body, never the status,
// so the frontend never confuses "the provider refused this key" with "our
// own server failed" (contract §5, "Response 200").
type validateKeyResponse struct {
	Result     string `json:"result"`
	Provider   string `json:"provider"`
	HTTPStatus int    `json:"http_status"`
	Detail     string `json:"detail,omitempty"`
}

// handleValidateAPIKey implements POST /api/ai/validate-key. No
// authentication (consistent with the rest of the unauthenticated
// BuzzControl admin surface, contract §8). No side effect: it never touches
// config.Save or config.SetInstance — the only two ways to persist anything
// in this codebase.
func (h *HTTPServer) handleValidateAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Hard size limit before decoding (same motif as handleConfig/
	// handleGenerateQuestions) — a provider name and one key, 64 KB is
	// generous headroom; a body over that limit fails the subsequent Decode
	// with an error that MaxBytesReader turns into a 413-shaped read error.
	r.Body = http.MaxBytesReader(w, r.Body, aiValidateMaxBodyBytes)

	var req validateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var prefix string
	switch req.Provider {
	case "anthropic":
		prefix = anthropicKeyPrefix
	case "groq":
		prefix = groqKeyPrefix
	default:
		http.Error(w, `provider must be "anthropic" or "groq"`, http.StatusBadRequest)
		return
	}

	// Prefix check BEFORE any network call (contract §5) — no point
	// disturbing the provider, or spending a cooldown slot, for a string
	// that cannot possibly be a key of the requested shape.
	if req.APIKey != "" && !strings.HasPrefix(req.APIKey, prefix) {
		http.Error(w, "Format de clé API invalide", http.StatusBadRequest)
		return
	}

	if !globalAIValidateCooldown.tryAcquire(time.Now()) {
		http.Error(w, "Trop de tentatives de validation, réessayez dans quelques secondes.", http.StatusTooManyRequests)
		return
	}

	// Selected deliberately by req.Provider, NOT h.selectProvider(cfg) —
	// selectProvider keys off cfg.Provider, the CURRENTLY ACTIVE provider,
	// whereas a validation request names the provider it wants validated
	// (e.g. testing a Groq key while Anthropic is the active provider).
	var provider aiProvider
	if req.Provider == "groq" {
		provider = &groqProvider{h: h}
	} else {
		provider = &anthropicProvider{h: h}
	}

	aiCfg := config.Get().AI
	result := provider.ValidateKey(r.Context(), aiCfg, req.APIKey)

	// Security motif M4 (ai_groq.go, github_client.go): log provider +
	// result + http_status only — never the key, never the raw detail text
	// before sanitization (result.Detail is already sanitized at this
	// point).
	LogInfo(game.LogComponentHTTP, "AI key validation: provider=%s result=%s http_status=%d",
		req.Provider, result.Result, result.HTTPStatus)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(validateKeyResponse{
		Result:     result.Result,
		Provider:   req.Provider,
		HTTPStatus: result.HTTPStatus,
		Detail:     result.Detail,
	})
}
