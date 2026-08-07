package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
)

// Groq client (contract ai-multi-provider.md §6) — stdlib net/http, same
// motif as internal/server/github_client.go (no SDK, no new dependency). The
// Groq endpoint is OpenAI-compatible chat completions with a JSON-schema
// response_format; unlike Anthropic, there is no streaming with structured
// outputs, so this is a single blocking POST per batch.

const (
	groqBaseURLEnvVar  = "GROQ_BASE_URL"
	groqDefaultBaseURL = "https://api.groq.com"
	groqChatPath       = "/openai/v1/chat/completions"
)

func groqBaseURL() string {
	if v := os.Getenv(groqBaseURLEnvVar); v != "" {
		return v
	}
	return groqDefaultBaseURL
}

// estimateTokens approximates a token count from character count (contract
// ai-multi-provider.md §4: "estimation de tokens côté serveur — approximation
// caractères/4 suffisante", applied here to the whole request prompt rather
// than just the injected context, since Groq's TPM gate — per T0.1 — counts
// the full prompt).
func estimateTokens(s string) int {
	n := len(s) / 4
	if n < 1 {
		n = 1
	}
	return n
}

// groqMaxTokensBudget is the TPM ceiling to budget against (Groq free tier,
// 8000 TPM — contract §1). groqTPMSafetyMargin is held back so a slightly
// low token estimate (chars/4 is approximate) doesn't itself trigger a 413.
const (
	groqMaxTokensBudget = 8000
	groqTPMSafetyMargin = 800
	groqMinMaxTokens    = 500
	groqMaxMaxTokens    = 6000
)

// groqRequestMaxTokens computes the max_tokens to REQUEST for this call.
//
// T0.1 calibration (2026-08-06, three live calls against the real Groq API
// with the production schema) found that Groq's TPM rate-limit gate counts
// prompt_tokens + the REQUESTED max_tokens — not actual usage — checked
// BEFORE generation starts: a call with a 954-token prompt and
// max_tokens=8000 was rejected with 413 "Requested 8954" (954+8000), while
// max_tokens=5000 succeeded (consuming exactly 954+5000=5954 against the
// budget) and generated all 20 requested questions without truncation
// (completion_tokens=3545, finish_reason=stop). This means a large fixed
// max_tokens (Anthropic's aiMaxTokens=64000 motif) is actively wrong for
// Groq: the reservation alone can exceed the whole per-minute budget even
// for a tiny prompt. max_tokens must instead be sized to the prompt actually
// being sent, with headroom kept under groqMaxTokensBudget.
func groqRequestMaxTokens(promptTokens int) int {
	budget := groqMaxTokensBudget - groqTPMSafetyMargin - promptTokens
	if budget > groqMaxMaxTokens {
		budget = groqMaxMaxTokens
	}
	if budget < groqMinMaxTokens {
		budget = groqMinMaxTokens
	}
	return budget
}

type groqChatRequest struct {
	Model          string                 `json:"model"`
	Messages       []groqChatMessage      `json:"messages"`
	ResponseFormat groqResponseFormat     `json:"response_format"`
	MaxTokens      int                    `json:"max_tokens"`
}

type groqChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqResponseFormat struct {
	Type       string             `json:"type"`
	JSONSchema groqJSONSchemaSpec `json:"json_schema"`
}

type groqJSONSchemaSpec struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type groqChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// groqErrorEnvelope mirrors Groq's OpenAI-compatible error body:
//
//	{"error":{"message":"invalid JSON schema for response_format: …",
//	  "type":"invalid_request_error","param":"response_format","code":""}}
//
// (real payload from #142's report, contracts.CHANGELOG.md [20260807]).
// .Message is now read (previously ignored — the exact gap that made #142
// undiagnosable without a temporary debug trace, see classifyGroqError).
type groqErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   string `json:"param"`
		Code    string `json:"code"`
	} `json:"error"`
}

// generateViaGroq calls the Groq chat-completions endpoint for one batch
// (contract §6). No streaming (unsupported with structured outputs on
// Groq); a single blocking call, deliberately kept short by the batching
// loop (ai_generator.go) precisely because of this.
func (h *HTTPServer) generateViaGroq(ctx context.Context, aiCfg config.AIConfig, prompt string, schema map[string]any) (string, error) {
	timeout := time.Duration(aiCfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	model := aiCfg.GroqModel
	if model == "" {
		model = "openai/gpt-oss-120b"
	}

	reqBody := groqChatRequest{
		Model:    model,
		Messages: []groqChatMessage{{Role: "user", Content: prompt}},
		ResponseFormat: groqResponseFormat{
			Type: "json_schema",
			JSONSchema: groqJSONSchemaSpec{
				Name:   "buzz_questions",
				Strict: true,
				Schema: schema,
			},
		},
		MaxTokens: groqRequestMaxTokens(estimateTokens(prompt)),
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", &aiUpstreamError{message: "failed to build the Groq request"}
	}

	url := groqBaseURL() + groqChatPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", &aiUpstreamError{message: "failed to build the Groq request"}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// EffectiveGroqAPIKey: BUZZCONTROL_GROQ_API_KEY env var takes priority
	// over config.json's stored value (security incident 2026-08-07 — see
	// config.AIConfig.EffectiveGroqAPIKey's doc comment).
	httpReq.Header.Set("Authorization", "Bearer "+aiCfg.EffectiveGroqAPIKey())

	client := &http.Client{} // no client-level Timeout: the request context already carries the deadline
	resp, err := client.Do(httpReq)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", &aiTimeoutError{}
		}
		// Security M4 (github_client.go motif): log the endpoint and nothing
		// else — never the request/response body, which would include the
		// Authorization header's key if a caller logged more than this.
		LogWarn(game.LogComponentHTTP, "AI generation (groq): request to %s failed (network)", groqChatPath)
		return "", &aiUpstreamError{message: "could not reach the AI generation service"}
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", &aiUpstreamError{message: "failed to read the Groq response"}
	}

	if resp.StatusCode != http.StatusOK {
		return "", classifyGroqError(resp, respBytes)
	}

	var parsed groqChatResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil || len(parsed.Choices) == 0 {
		return "", &aiUpstreamError{message: "malformed response from the AI generation service"}
	}
	content := parsed.Choices[0].Message.Content
	if strings.TrimSpace(content) == "" {
		return "", &aiUpstreamError{message: "empty response from the AI generation service"}
	}
	return content, nil
}

// classifyGroqError maps a non-200 Groq response to one of our stable error
// types (mirrors classifyAnthropicError). Parses the error envelope both to
// distinguish rate-limit responses (429/413) from everything else AND
// (amended #142-adjacent verbosity fix, contract ai-generation.md §8 S2) to
// surface .Error.Message, sanitized, as the admin-facing detail — this is
// the exact field #142 needed a temporary debug trace to see ("invalid JSON
// schema for response_format: … discriminator: multiple candidate
// properties …"), and it is Groq's own generated text, never anything
// containing the caller's Authorization header/API key.
func classifyGroqError(resp *http.Response, body []byte) error {
	var envelope groqErrorEnvelope
	_ = json.Unmarshal(body, &envelope) // best-effort; a malformed/non-JSON body just yields an empty envelope, falling back to the generic message below

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusRequestEntityTooLarge {
		return &aiRateLimitError{
			statusCode: resp.StatusCode,
			retryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}
	message := sanitizeUpstreamMessage(envelope.Error.Message)
	if message == "" {
		message = "the AI generation service returned an error"
	}
	return &aiUpstreamError{statusCode: resp.StatusCode, message: message}
}
