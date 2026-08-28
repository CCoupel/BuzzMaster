// Tests for the QUALIF v7.1.0.7 bugfix — "AI generation service rate limit
// exceeded" reported on the very FIRST MEMOTION_PLUS generation attempt.
//
// Root cause (see buildQuestionSchema's and aiRateLimitError's doc comments
// for the full narrative): buildQuestionSchema unconditionally built an
// anyOf branch for EVERY generable type on EVERY batch call, regardless of
// the admin's actual distribution. That was already wasteful; #196's
// MEMOTION_PLUS variant (a nested discriminated union, larger than any
// prior branch) was the drop that pushed an already-marginal Groq 8000 TPM
// request over the provider's 413 "too large" pre-check — on the very first
// call, with zero prior usage that session, which is why the generic
// "rate limit exceeded" message was actively misleading (it reads as quota
// exhaustion, not "this one request is too big").
//
// Two-part fix covered here:
//  1. activeGenerableTypes/buildQuestionSchema now only include anyOf
//     branches for types actually requested in the distribution — this test
//     file checks the filtering itself (ai_generator_memotion_plus_196_test.go
//     already covers MEMOTION_PLUS's own schema shape and is unaffected,
//     since it always passes generableQuestionTypes — i.e. "all active").
//  2. aiRateLimitError.Error() now distinguishes 413 (request too large)
//     from 429 (true rate limit) and surfaces the provider's own detail
//     message when available, instead of one generic string for both.
//
// Run: go test ./internal/server/... -run TestSchemaFiltering -v
// Run: go test ./internal/server/... -run TestAIRateLimitError -v
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ============================================================================
// activeGenerableTypes
// ============================================================================

func TestSchemaFiltering_ActiveGenerableTypes_OnlyPositiveDistributionEntries(t *testing.T) {
	tests := []struct {
		name         string
		distribution map[string]int
		want         []string
	}{
		{
			name:         "single type requested",
			distribution: map[string]int{"MEMOTION_PLUS": 5},
			want:         []string{"MEMOTION_PLUS"},
		},
		{
			name:         "several types requested, preserves generableQuestionTypes order",
			distribution: map[string]int{"ARDOISE": 2, "SPEEDY": 3, "MEMOTION": 1},
			want:         []string{"SPEEDY", "MEMOTION", "ARDOISE"},
		},
		{
			name:         "zero-valued and negative entries excluded",
			distribution: map[string]int{"SPEEDY": 0, "QCM": -1, "MEMORY": 4},
			want:         []string{"MEMORY"},
		},
		{
			name:         "unknown keys in distribution ignored",
			distribution: map[string]int{"NOT_A_REAL_TYPE": 10, "SPEEDY": 1},
			want:         []string{"SPEEDY"},
		},
		{
			name:         "empty distribution yields empty (never reached in production — validateGenerateRequest guards this)",
			distribution: map[string]int{},
			want:         nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := activeGenerableTypes(tt.distribution)
			if len(got) != len(tt.want) {
				t.Fatalf("activeGenerableTypes(%v) = %v, want %v", tt.distribution, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("activeGenerableTypes(%v) = %v, want %v", tt.distribution, got, tt.want)
				}
			}
		})
	}
}

// ============================================================================
// buildQuestionSchema — anyOf actually shrinks
// ============================================================================

func schemaAnyOfTypeConsts(t *testing.T, schema map[string]any) []string {
	t.Helper()
	anyOf := schema["properties"].(map[string]any)["questions"].(map[string]any)["items"].(map[string]any)["anyOf"].([]any)
	var consts []string
	for _, v := range anyOf {
		variant, ok := v.(map[string]any)
		if !ok {
			continue
		}
		props, ok := variant["properties"].(map[string]any)
		if !ok {
			continue
		}
		typeSchema, ok := props["TYPE"].(map[string]any)
		if !ok {
			continue
		}
		if c, _ := typeSchema["const"].(string); c != "" {
			consts = append(consts, c)
		}
	}
	return consts
}

func TestSchemaFiltering_BuildQuestionSchema_AllTypesActive_SixBranches(t *testing.T) {
	schema := buildQuestionSchema([]string{"GEOGRAPHY"}, []string{"Facile"}, generableQuestionTypes)
	consts := schemaAnyOfTypeConsts(t, schema)
	if len(consts) != 6 {
		t.Fatalf("expected 6 anyOf branches with all types active, got %d: %v", len(consts), consts)
	}
}

// TestSchemaFiltering_BuildQuestionSchema_SingleTypeActive_OneBranch is the
// direct regression test for the QUALIF v7.1.0.7 root cause: a distribution
// requesting only MEMOTION_PLUS must produce a schema with exactly ONE anyOf
// branch (MEMOTION_PLUS's own, the largest single branch) — not all 6 — so
// the request stays well under Groq's per-request size ceiling regardless of
// how large any one type's own schema is.
func TestSchemaFiltering_BuildQuestionSchema_SingleTypeActive_OneBranch(t *testing.T) {
	schema := buildQuestionSchema([]string{"GEOGRAPHY"}, []string{"Facile"}, []string{"MEMOTION_PLUS"})
	consts := schemaAnyOfTypeConsts(t, schema)
	if len(consts) != 1 || consts[0] != "MEMOTION_PLUS" {
		t.Fatalf("expected exactly [MEMOTION_PLUS], got %v", consts)
	}
}

func TestSchemaFiltering_BuildQuestionSchema_TwoTypesActive_TwoBranchesInDeclaredOrder(t *testing.T) {
	schema := buildQuestionSchema([]string{"GEOGRAPHY"}, []string{"Facile"}, []string{"MEMOTION", "SPEEDY"})
	consts := schemaAnyOfTypeConsts(t, schema)
	if len(consts) != 2 {
		t.Fatalf("expected 2 anyOf branches, got %d: %v", len(consts), consts)
	}
	// Order follows allVariants' fixed declaration order (SPEEDY, QCM, MEMORY,
	// MEMOTION, MEMOTION_PLUS, ARDOISE), not activeTypes' order — asserting
	// this pins the current (harmless, unspecified-by-contract) behavior so a
	// future refactor notices if it silently changes.
	if consts[0] != "SPEEDY" || consts[1] != "MEMOTION" {
		t.Fatalf("expected [SPEEDY MEMOTION] in declared order, got %v", consts)
	}
}

// TestSchemaFiltering_BuildQuestionSchema_EmptyActiveTypes_EmptyAnyOf documents
// the (never hit on the production path — validateGenerateRequest guarantees
// at least one positive distribution entry) degenerate case rather than
// leaving it unspecified: an empty activeTypes yields an empty anyOf, not a
// panic and not "all types" by some hidden fallback.
func TestSchemaFiltering_BuildQuestionSchema_EmptyActiveTypes_EmptyAnyOf(t *testing.T) {
	schema := buildQuestionSchema([]string{"GEOGRAPHY"}, []string{"Facile"}, nil)
	consts := schemaAnyOfTypeConsts(t, schema)
	if len(consts) != 0 {
		t.Fatalf("expected empty anyOf, got %v", consts)
	}
}

// ============================================================================
// aiRateLimitError.Error() — 413 vs 429 differentiation + message surfacing
// ============================================================================

func TestAIRateLimitError_Error_413DiffersFrom429(t *testing.T) {
	err413 := &aiRateLimitError{statusCode: 413}
	err429 := &aiRateLimitError{statusCode: 429}

	if err413.Error() == err429.Error() {
		t.Fatalf("413 and 429 must produce different messages, both got %q", err413.Error())
	}
	if got := err413.Error(); jsonIndexOf(got, "too large") < 0 {
		t.Errorf("413 message should mention the request being too large, got %q", got)
	}
	if got := err429.Error(); jsonIndexOf(got, "rate limit") < 0 {
		t.Errorf("429 message should mention rate limiting, got %q", got)
	}
}

func TestAIRateLimitError_Error_SurfacesProviderMessageWhenPresent(t *testing.T) {
	err := &aiRateLimitError{statusCode: 413, message: "request too large for model `openai/gpt-oss-120b`"}
	got := err.Error()
	if jsonIndexOf(got, "request too large for model") < 0 {
		t.Fatalf("expected the provider detail to be surfaced, got %q", got)
	}
}

func TestAIRateLimitError_Error_FallsBackToGenericWhenNoProviderMessage(t *testing.T) {
	err := &aiRateLimitError{statusCode: 429}
	got := err.Error()
	if got == "" {
		t.Fatal("Error() must never return an empty string")
	}
	if jsonIndexOf(got, ":") >= 0 {
		t.Fatalf("with no provider message, no trailing ': ' separator should appear, got %q", got)
	}
}

// TestClassifyGroqError_413And429_SurfaceEnvelopeMessage is the direct
// regression test for the QUALIF v7.1.0.7 hypothesis #3 (misleading error
// message): before this fix, classifyGroqError's 429/413 branch discarded
// envelope.Error.Message entirely (only the generic fallback path a few
// lines below it read it) — an admin hitting Groq's 413 pre-check saw only
// "AI generation service rate limit exceeded" with no indication the
// request itself was oversized, nor Groq's own stated reason.
func TestClassifyGroqError_413And429_SurfaceEnvelopeMessage(t *testing.T) {
	body413 := []byte(`{"error":{"message":"Request too large for model ` + "`openai/gpt-oss-120b`" + ` on tokens per minute (TPM): Limit 8000, Requested 8954","type":"tokens","code":"rate_limit_exceeded"}}`)
	resp413 := &http.Response{StatusCode: http.StatusRequestEntityTooLarge, Header: http.Header{}}

	err := classifyGroqError(resp413, body413)
	rle, ok := err.(*aiRateLimitError)
	if !ok {
		t.Fatalf("expected *aiRateLimitError for a 413 response, got %T (%v)", err, err)
	}
	if rle.statusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("statusCode = %d, want %d", rle.statusCode, http.StatusRequestEntityTooLarge)
	}
	if jsonIndexOf(rle.Error(), "Requested 8954") < 0 {
		t.Errorf("expected Groq's own detail message to be surfaced, got %q", rle.Error())
	}
	if jsonIndexOf(rle.Error(), "too large") < 0 {
		t.Errorf("expected the 413 case to say the request is too large, got %q", rle.Error())
	}

	body429 := []byte(`{"error":{"message":"Rate limit reached, please try again later","type":"requests","code":"rate_limit_exceeded"}}`)
	resp429 := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}

	err = classifyGroqError(resp429, body429)
	rle, ok = err.(*aiRateLimitError)
	if !ok {
		t.Fatalf("expected *aiRateLimitError for a 429 response, got %T (%v)", err, err)
	}
	if jsonIndexOf(rle.Error(), "please try again later") < 0 {
		t.Errorf("expected Groq's own detail message to be surfaced, got %q", rle.Error())
	}
}

// ============================================================================
// classifyAnthropicError — closes the coverage asymmetry flagged by
// code-review-20260827-210604.md (Point 2, INFO non-bloquant, verdict
// APPROUVE) : classifyGroqError above has a direct test constructing a
// *http.Response + body by hand; classifyAnthropicError had none — `grep -rn
// "classifyAnthropicError" internal/server/*_test.go` returned zero matches
// before this test, only indirect end-to-end coverage through the full job
// pipeline (TestGenerateQuestions_UpstreamErrors/"429 rate limit maps to
// provider_quota", ai_generation_test.go) that asserts on ERROR_CODE, never
// on the specific 413-vs-429 message text this cycle's fix introduced.
//
// classifyAnthropicError takes an `error`, not a (*http.Response, []byte)
// pair like classifyGroqError — there is no low-friction way to hand-build a
// populated *anthropic.Error from this package (its RawJSON() backing field
// is unexported inside the SDK's internal/apierror package, only ever
// populated by the SDK's own JSON decoding path). Rather than reach for
// unsafe/reflect tricks to fake one, this test drives the REAL SDK error
// path end-to-end at the classifyAnthropicError boundary: a local
// httptest.Server speaks Anthropic's documented error envelope shape (same
// shape mockAnthropicErrorServer in ai_generation_test.go already uses for
// its own indirect job-level tests — kept local here instead of reusing that
// helper because this test needs a DISTINCT, realistic message per status
// code, whereas the shared helper hardcodes one fixed message for both), the
// SDK client (generateViaAnthropic) makes a real call against it, and the
// *anthropic.Error the SDK constructs from the real response flows through
// classifyAnthropicError exactly as production does — the most direct test
// possible without depending on SDK-internal fields.
func TestClassifyAnthropicError_413And429_SurfaceEnvelopeMessage(t *testing.T) {
	aiCfg := validAIConfig()
	h := &HTTPServer{}

	newAnthropicErrorServer := func(t *testing.T, statusCode int, message string) *httptest.Server {
		t.Helper()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"type": "error",
				"error": map[string]string{
					"type":    "rate_limit_error",
					"message": message,
				},
			})
		}))
		t.Cleanup(server.Close)
		return server
	}

	t.Run("413 request too large", func(t *testing.T) {
		upstream := newAnthropicErrorServer(t, http.StatusRequestEntityTooLarge, "prompt is too long: 215000 tokens > 200000 maximum")
		withAnthropicBaseURL(t, upstream.URL)

		_, err := h.generateViaAnthropic(context.Background(), aiCfg, "prompt", map[string]any{})
		rle, ok := err.(*aiRateLimitError)
		if !ok {
			t.Fatalf("expected *aiRateLimitError for a 413 response, got %T (%v)", err, err)
		}
		if rle.statusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("statusCode = %d, want %d", rle.statusCode, http.StatusRequestEntityTooLarge)
		}
		if jsonIndexOf(rle.Error(), "215000 tokens") < 0 {
			t.Errorf("expected Anthropic's own detail message to be surfaced, got %q", rle.Error())
		}
		if jsonIndexOf(rle.Error(), "too large") < 0 {
			t.Errorf("expected the 413 case to say the request is too large, got %q", rle.Error())
		}
		if jsonIndexOf(rle.Error(), "rate limit exceeded") >= 0 {
			t.Errorf("expected the 413 case NOT to say the generic rate-limit text, got %q", rle.Error())
		}
	})

	t.Run("429 rate limit", func(t *testing.T) {
		upstream := newAnthropicErrorServer(t, http.StatusTooManyRequests, "Number of requests has exceeded your organization's rate limit")
		withAnthropicBaseURL(t, upstream.URL)

		_, err := h.generateViaAnthropic(context.Background(), aiCfg, "prompt", map[string]any{})
		rle, ok := err.(*aiRateLimitError)
		if !ok {
			t.Fatalf("expected *aiRateLimitError for a 429 response, got %T (%v)", err, err)
		}
		if rle.statusCode != http.StatusTooManyRequests {
			t.Errorf("statusCode = %d, want %d", rle.statusCode, http.StatusTooManyRequests)
		}
		if jsonIndexOf(rle.Error(), "organization's rate limit") < 0 {
			t.Errorf("expected Anthropic's own detail message to be surfaced, got %q", rle.Error())
		}
		if jsonIndexOf(rle.Error(), "too large") >= 0 {
			t.Errorf("expected the 429 case NOT to say the 413 'too large' text, got %q", rle.Error())
		}
	})
}
