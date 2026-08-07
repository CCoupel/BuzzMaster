package server

// Unit tests for sanitizeUpstreamMessage and extractAnthropicErrorMessage
// (ai_client.go) — the #142-adjacent verbosity fix (contract ai-generation.md
// §8 S2, amended). End-to-end coverage of the full provider→job→WS chain
// lives in ai_batching_test.go (TestAIBatching_*SurfacesProviderMessage,
// TestAIBatching_GroqUpstreamError_MessageRedactsAPIKeyShape); this file
// isolates the sanitization logic itself.

import (
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestSanitizeUpstreamMessage_EmptyInputStaysEmpty(t *testing.T) {
	if got := sanitizeUpstreamMessage(""); got != "" {
		t.Errorf("expected empty string to stay empty, got %q", got)
	}
	if got := sanitizeUpstreamMessage("   "); got != "" {
		t.Errorf("expected whitespace-only string to become empty, got %q", got)
	}
}

func TestSanitizeUpstreamMessage_PassesThroughOrdinaryText(t *testing.T) {
	const msg = "invalid JSON schema for response_format: discriminator: multiple candidate properties"
	if got := sanitizeUpstreamMessage(msg); got != msg {
		t.Errorf("expected ordinary provider text to pass through unchanged, got %q", got)
	}
}

func TestSanitizeUpstreamMessage_RedactsAnthropicKeyShape(t *testing.T) {
	msg := "some detail mentioning sk-ant-api03-abcDEF1234567890 in passing"
	got := sanitizeUpstreamMessage(msg)
	if strings.Contains(got, "sk-ant-api03-abcDEF1234567890") {
		t.Errorf("expected the sk-ant- key shape to be redacted, got %q", got)
	}
	if !strings.Contains(got, "[redacted]") {
		t.Errorf("expected a [redacted] marker in place of the key shape, got %q", got)
	}
}

func TestSanitizeUpstreamMessage_RedactsGroqKeyShape(t *testing.T) {
	msg := "some detail mentioning gsk_abcdEFGH12345678ijklMNOPqrstUVWX in passing"
	got := sanitizeUpstreamMessage(msg)
	if strings.Contains(got, "gsk_abcdEFGH12345678ijklMNOPqrstUVWX") {
		t.Errorf("expected the gsk_ key shape to be redacted, got %q", got)
	}
	if !strings.Contains(got, "[redacted]") {
		t.Errorf("expected a [redacted] marker in place of the key shape, got %q", got)
	}
}

func TestSanitizeUpstreamMessage_TruncatesOverlongMessages(t *testing.T) {
	long := strings.Repeat("a", aiUpstreamMessageMaxLen+200)
	got := sanitizeUpstreamMessage(long)
	// +1 rune for the trailing "…" marker.
	if runeLen := len([]rune(got)); runeLen != aiUpstreamMessageMaxLen+1 {
		t.Errorf("expected truncation to %d runes + ellipsis, got %d runes: %q", aiUpstreamMessageMaxLen, runeLen, got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected a truncated message to end with an ellipsis marker, got %q", got)
	}
}

func TestSanitizeUpstreamMessage_TruncationIsRuneSafe(t *testing.T) {
	// A multi-byte rune sitting right at the truncation boundary must not be
	// split into invalid UTF-8 (byte-slicing a string, rather than a rune
	// slice, would risk exactly that for non-ASCII provider text).
	long := strings.Repeat("é", aiUpstreamMessageMaxLen+50) // each 'é' is 2 bytes in UTF-8
	got := sanitizeUpstreamMessage(long)
	if !utf8.ValidString(got) {
		t.Errorf("expected truncated output to remain valid UTF-8, got invalid bytes in %q", got)
	}
}

func TestExtractAnthropicErrorMessage_NilError(t *testing.T) {
	if got := extractAnthropicErrorMessage(nil); got != "" {
		t.Errorf("expected empty string for a nil *anthropic.Error, got %q", got)
	}
}

func TestExtractAnthropicErrorMessage_MalformedBody(t *testing.T) {
	apiErr := &anthropic.Error{}
	if got := extractAnthropicErrorMessage(apiErr); got != "" {
		t.Errorf("expected empty string when RawJSON() doesn't parse (zero-value Error has no body), got %q", got)
	}
}

func TestClassifyGroqError_UnknownStatus_FallsBackToGenericWhenNoMessage(t *testing.T) {
	// A malformed/empty error body (envelope.Error.Message == "") must not
	// leave ERROR_MESSAGE empty on a real failure — the generic fallback
	// text from before this fix still applies when there's genuinely nothing
	// to surface.
	resp := &http.Response{StatusCode: 500, Header: http.Header{}}
	err := classifyGroqError(resp, []byte(`not json`))
	upstreamErr, ok := err.(*aiUpstreamError)
	if !ok {
		t.Fatalf("expected *aiUpstreamError, got %T", err)
	}
	if upstreamErr.message != "the AI generation service returned an error" {
		t.Errorf("expected the generic fallback message when the body has no parseable detail, got %q", upstreamErr.message)
	}
}

func TestClassifyGroqError_SurfacesRealMessage(t *testing.T) {
	resp := &http.Response{StatusCode: 400, Header: http.Header{}}
	body := []byte(`{"error":{"message":"invalid JSON schema for response_format: discriminator ambiguous","type":"invalid_request_error"}}`)
	err := classifyGroqError(resp, body)
	upstreamErr, ok := err.(*aiUpstreamError)
	if !ok {
		t.Fatalf("expected *aiUpstreamError, got %T", err)
	}
	if upstreamErr.message != "invalid JSON schema for response_format: discriminator ambiguous" {
		t.Errorf("expected the real Groq message to be surfaced verbatim (no secret to redact here), got %q", upstreamErr.message)
	}
}
