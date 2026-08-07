package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"buzzcontrol/internal/config"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// aiMaxTokens is the hard ceiling passed to the Anthropic API. Structured
// output for up to ai.max_questions (default 200) questions can be large, and
// max_tokens caps *both* thinking and text — too low a value truncates the
// response mid-JSON (contract ai-generation.md §4).
const aiMaxTokens = 64000

// aiTimeoutError signals that the upstream Anthropic call did not complete
// within config.AIConfig.TimeoutSeconds. Mapped to HTTP 504 (contract §3).
type aiTimeoutError struct{}

func (e *aiTimeoutError) Error() string { return "AI generation timed out" }

// aiUpstreamError wraps any failure originating from the Anthropic API itself
// (authentication, rate limit, overload, malformed/incomplete response) or
// from the network path to it. Mapped to HTTP 502 (contract §3). statusCode
// is the upstream HTTP status when known (0 otherwise, e.g. for a network
// error that never got an HTTP response).
//
// message is caller-safe and admin-facing (contract §8 S2, amended #142-
// adjacent verbosity fix): when the upstream response carried a specific
// error detail (e.g. "invalid JSON schema for response_format: ...", the
// exact shape that made #142 undiagnosable without a temporary debug trace),
// it is surfaced here — sanitized (sanitizeUpstreamMessage: API-key-shaped
// substrings redacted, length-bounded) rather than replaced by a fixed
// generic string. Local, non-upstream failures (couldn't build/reach the
// request) keep a plain locally-authored message, nothing to sanitize.
// Reaches the admin ONLY via AI_GENERATION_PROGRESS.ERROR_MESSAGE
// (/ws/admin-only action) — never a synchronous HTTP response (batching
// moved generation errors off the request/response cycle, contract
// ai-multi-provider.md §10). The API key itself is never included here or
// anywhere the caller can see it (contract §8 S2 — the key isn't part of
// what sanitizeUpstreamMessage exists to catch defensively; it is simply
// never read from response bodies in the first place).
type aiUpstreamError struct {
	statusCode int
	message    string
}

func (e *aiUpstreamError) Error() string { return e.message }

// aiRateLimitError signals a 429 (rate limit) or 413 (request too large —
// Groq's TPM pre-check rejection, contract ai-multi-provider.md §3) upstream
// response. Distinct from aiUpstreamError because the caller (the batch loop,
// ai_job.go) must react differently: wait retryAfter (or apply backoff) and
// retry the SAME batch rather than treating it as an ordinary failure — and
// only escalate to the stable "provider_quota" code after persistent
// rate-limiting (contract §3).
type aiRateLimitError struct {
	statusCode int
	retryAfter time.Duration // 0 if the response didn't specify one
}

func (e *aiRateLimitError) Error() string { return "AI generation service rate limit exceeded" }

// parseRetryAfter reads a Retry-After header value. Only the delay-seconds
// form is handled (what Groq and Anthropic both send in practice); an
// HTTP-date value or empty/unparseable input yields 0 (caller falls back to
// its own backoff).
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

// aiSecretRedactionPatterns match the API key shapes used by supported
// providers (config.AIConfig.AnthropicAPIKey "sk-ant-…", GroqAPIKey "gsk_…").
// A provider's own error body should never echo back the caller's
// Authorization header value in the first place, so this is defense in
// depth, not the primary safeguard (contract §8 S2) — applied by
// sanitizeUpstreamMessage before ANY upstream-supplied text is stored or
// broadcast to an admin.
var aiSecretRedactionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]+`),
	regexp.MustCompile(`gsk_[A-Za-z0-9_-]+`),
}

// aiUpstreamMessageMaxLen bounds AI_GENERATION_PROGRESS.ERROR_MESSAGE — an
// upstream error body is untrusted in length as much as in content; nothing
// requires it to be short.
const aiUpstreamMessageMaxLen = 500

// sanitizeUpstreamMessage prepares a provider-supplied (or locally-authored)
// error detail for display to the admin (contract ai-generation.md §8 S2,
// amended #142-adjacent verbosity fix — see aiUpstreamError's doc comment for
// the full rationale): redacts anything matching a known API key shape, then
// truncates to a bounded length. "" in, "" out — callers fall back to a
// generic message themselves when this returns empty (e.g. the upstream body
// didn't parse, or carried no message field).
func sanitizeUpstreamMessage(raw string) string {
	msg := strings.TrimSpace(raw)
	if msg == "" {
		return ""
	}
	for _, pattern := range aiSecretRedactionPatterns {
		msg = pattern.ReplaceAllString(msg, "[redacted]")
	}
	if runes := []rune(msg); len(runes) > aiUpstreamMessageMaxLen {
		msg = string(runes[:aiUpstreamMessageMaxLen]) + "…"
	}
	return msg
}

// anthropicErrorEnvelope mirrors the standard Anthropic API error body
// ({"type":"error","error":{"type":"...","message":"..."}}) — only .Error.Message
// is read; the rest is unclassified low-value detail already reflected in
// apiErr.StatusCode / the aiRateLimitError vs aiUpstreamError split.
type anthropicErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// extractAnthropicErrorMessage pulls the human-readable detail out of an
// *anthropic.Error's raw response body (apiErr.RawJSON(), the unmodified
// bytes the SDK received — apierror.Error.Error() already assembles a
// similar string for logs, but re-parses the same raw JSON here to isolate
// just the message field rather than method/URL/request-ID noise, for a
// cleaner admin-facing string). Returns "" if the body doesn't parse or
// carries no message — callers fall back to a generic message.
func extractAnthropicErrorMessage(apiErr *anthropic.Error) string {
	if apiErr == nil {
		return ""
	}
	var envelope anthropicErrorEnvelope
	if err := json.Unmarshal([]byte(apiErr.RawJSON()), &envelope); err != nil {
		return ""
	}
	return envelope.Error.Message
}

// generateViaAnthropic calls the Anthropic Messages API in streaming mode
// (contract §4 — "Streaming obligatoire": the response for a full batch can
// be large enough to exceed a non-streamed request's timeout) with a JSON
// schema structured-output format, and returns the accumulated text content
// (expected to be a single JSON object matching schema).
//
// The client picks up ANTHROPIC_BASE_URL from the environment automatically
// (anthropic.NewClient's documented behavior) — this is how tests redirect
// calls to a local httptest mock without any extra plumbing here.
func (h *HTTPServer) generateViaAnthropic(ctx context.Context, aiCfg config.AIConfig, prompt string, schema map[string]any) (string, error) {
	timeout := time.Duration(aiCfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// option.WithMaxRetries(0): the SDK retries 5xx/429 internally by default
	// (2 retries — internal/requestconfig, cfg.MaxRetries). That's now
	// actively wrong here: the batching job loop (ai_job.go) owns retry/
	// consecutive-failure semantics (contract ai-multi-provider.md §2-§3) —
	// letting the SDK ALSO retry silently absorbs failures the job needs to
	// see and count, and consumes extra HTTP calls the job's own backoff
	// doesn't know about. Discovered via a test whose mock served a fixed
	// failure sequence: the SDK's internal retries silently skipped past the
	// intended failures before the job ever saw an error.
	// EffectiveAnthropicAPIKey: BUZZCONTROL_ANTHROPIC_API_KEY env var takes
	// priority over config.json's stored value (security incident
	// 2026-08-07 — see config.AIConfig.EffectiveAnthropicAPIKey's doc comment).
	client := anthropic.NewClient(option.WithAPIKey(aiCfg.EffectiveAnthropicAPIKey()), option.WithMaxRetries(0))

	params := anthropic.MessageNewParams{
		Model:     aiCfg.Model,
		MaxTokens: aiMaxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortMedium,
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		},
	}

	stream := client.Messages.NewStreaming(ctx, params)

	var text strings.Builder
	for stream.Next() {
		event := stream.Current()
		if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" {
			text.WriteString(event.Delta.Text)
		}
	}
	if err := stream.Err(); err != nil {
		return "", classifyAnthropicError(err)
	}
	if text.Len() == 0 {
		return "", &aiUpstreamError{message: "empty response from the AI generation service"}
	}
	return text.String(), nil
}

// classifyAnthropicError maps an error from the SDK to one of our two stable
// outcomes (timeout vs. upstream_error, contract §3). The upstream HTTP
// status crosses that boundary via aiUpstreamError.statusCode; the upstream
// error body's .error.message, sanitized, crosses it via .message (contract
// §8 S2, amended — see aiUpstreamError's doc comment).
func classifyAnthropicError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &aiTimeoutError{}
	}
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode == http.StatusRequestEntityTooLarge {
			var retryAfter time.Duration
			if apiErr.Response != nil {
				retryAfter = parseRetryAfter(apiErr.Response.Header.Get("Retry-After"))
			}
			return &aiRateLimitError{statusCode: apiErr.StatusCode, retryAfter: retryAfter}
		}
		message := sanitizeUpstreamMessage(extractAnthropicErrorMessage(apiErr))
		if message == "" {
			message = "the AI generation service returned an error"
		}
		return &aiUpstreamError{statusCode: apiErr.StatusCode, message: message}
	}
	// Network error, connection refused, DNS failure, etc. — no HTTP response
	// was ever received, but it's still an upstream-reachability problem.
	return &aiUpstreamError{message: "could not reach the AI generation service"}
}
