// Tests for the QUALIF v7.1.0.8 cycle 3 bugfix — MEMOTION_PLUS generation
// intermittently failing with Groq's json_validate_failed ("does not
// validate with .../anyOf/0/required: missing properties: 'ANSWER_TEXT'"),
// plus the multi-type rate-limit report investigated in the same cycle.
//
// See ai_provider.go's AdaptSchema/stripGroqAnyOfDiscriminatorFields doc
// comments and ai_groq.go's groqRequestMaxTokens doc comment for the full
// root-cause narrative:
//   - Real bug: the #142 discriminator-ambiguity fix only ever walked the
//     TOP-level anyOf; MEMOTION_PLUS's nested MOTION_CARDS.items.anyOf
//     (SPEEDY-card vs QCM-card) has the exact same shared-required-enum
//     ambiguity (DIFFICULTY, [1,2,3], required in both branches) and was
//     never reached — fixed by making the walk recursive.
//   - Optimization (point 1, multi-type rate limit): max_tokens' budget
//     calculation only ever counted the prompt string, never the schema
//     JSON sent alongside it — a request with several active (and
//     therefore larger) type variants under-reserved max_tokens relative to
//     the true request cost, which biases MULTI-type requests toward the
//     413 ceiling specifically. Fixed by folding the schema's own
//     marshaled size into the same budget estimate.
//
// Run: go test ./internal/server/... -run TestGroqNestedDiscriminator -v
// Run: go test ./internal/server/... -run TestGroqRequestMaxTokens -v
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"buzzcontrol/internal/config"
)

// ============================================================================
// Real bug — nested MOTION_CARDS anyOf discriminator ambiguity (MEMOTION_PLUS)
// ============================================================================

// findMotionPlusCardBranches applies AdaptSchema to a MEMOTION_PLUS-active
// schema and navigates down to its MOTION_CARDS.items.anyOf branches
// (SPEEDY-card, QCM-card) — the exact nested union
// stripGroqAnyOfDiscriminatorFields must now reach.
func findMotionPlusCardBranches(t *testing.T, schema map[string]any) []map[string]any {
	t.Helper()
	memotionPlus := findAnyOfVariantByTypeConst(t, schema, "MEMOTION_PLUS")
	props, ok := memotionPlus["properties"].(map[string]any)
	if !ok {
		t.Fatalf("Expected MEMOTION_PLUS variant.properties to be a map, got %T", memotionPlus["properties"])
	}
	motionCards, ok := props["MOTION_CARDS"].(map[string]any)
	if !ok {
		t.Fatalf("Expected MOTION_CARDS to be a map, got %T", props["MOTION_CARDS"])
	}
	items, ok := motionCards["items"].(map[string]any)
	if !ok {
		t.Fatalf("Expected MOTION_CARDS.items to be a map, got %T", motionCards["items"])
	}
	cardAnyOf, ok := items["anyOf"].([]any)
	if !ok {
		t.Fatalf("Expected MOTION_CARDS.items.anyOf to be a slice, got %T", items["anyOf"])
	}
	out := make([]map[string]any, 0, len(cardAnyOf))
	for _, b := range cardAnyOf {
		bm, ok := b.(map[string]any)
		if !ok {
			t.Fatalf("Expected each MOTION_CARDS anyOf entry to be a map, got %T", b)
		}
		out = append(out, bm)
	}
	return out
}

// TestGroqNestedDiscriminator_StripsDIFFICULTYFromMotionPlusCardBranches is
// the direct regression test for the QUALIF v7.1.0.8 cycle 3 bug: before
// this fix, DIFFICULTY (required, enum [1,2,3] in BOTH the SPEEDY-card and
// QCM-card branches) survived untouched inside the nested MOTION_CARDS
// anyOf — a second discriminator candidate alongside TYPE, exactly the
// ambiguity #142 already named at the top level.
func TestGroqNestedDiscriminator_StripsDIFFICULTYFromMotionPlusCardBranches(t *testing.T) {
	p := &groqProvider{}
	schema := p.AdaptSchema(buildQuestionSchema([]string{"GEOGRAPHY"}, []string{"Facile"}, []string{"MEMOTION_PLUS"}))

	branches := findMotionPlusCardBranches(t, schema)
	if len(branches) != 2 {
		t.Fatalf("Expected 2 MOTION_CARDS anyOf branches (SPEEDY/QCM), got %d", len(branches))
	}

	for _, branch := range branches {
		props, ok := branch["properties"].(map[string]any)
		if !ok {
			t.Fatalf("Expected branch.properties to be a map, got %T", branch["properties"])
		}
		typeField, ok := props["TYPE"].(map[string]any)
		if !ok {
			t.Fatalf("Expected branch.properties.TYPE to be a map, got %T", props["TYPE"])
		}
		typeName, _ := typeField["const"].(string)

		difficulty, ok := props["DIFFICULTY"].(map[string]any)
		if !ok {
			t.Fatalf("[%s] Expected properties.DIFFICULTY to be a map, got %T", typeName, props["DIFFICULTY"])
		}
		if _, hasEnum := difficulty["enum"]; hasEnum {
			t.Errorf("[%s] Expected nested DIFFICULTY.enum to be stripped by AdaptSchema, still present: %v", typeName, difficulty["enum"])
		}
		if difficulty["type"] != "integer" {
			t.Errorf("[%s] Expected nested DIFFICULTY to remain type=integer after stripping enum, got %v", typeName, difficulty["type"])
		}

		// TYPE itself is untouched — the intended, sole discriminator.
		if typeField["const"] == nil || typeField["const"] == "" {
			t.Errorf("Expected TYPE.const to remain set as the sole discriminator, got %v", typeField["const"])
		}

		required, ok := branch["required"].([]string)
		if !ok {
			t.Fatalf("[%s] Expected branch.required to be a []string, got %T", typeName, branch["required"])
		}
		if !stringInSlice(required, "DIFFICULTY") {
			t.Errorf("[%s] Expected DIFFICULTY to remain required after AdaptSchema (only the enum constraint is dropped), got required=%v", typeName, required)
		}
	}
}

// TestGroqNestedDiscriminator_OnlyTypeSharedAcrossMotionPlusCardBranches
// mirrors TestGroqProvider_AdaptSchema_OnlyOneSharedEnumCandidateAcrossAllBranches
// but for the nested union: after AdaptSchema, TYPE must be the ONLY
// required+enum/const-constrained property shared by both card branches —
// QCM_CORRECT (enum, but only required in the QCM-card branch) never
// qualifies, same non-candidate logic as the top-level test.
func TestGroqNestedDiscriminator_OnlyTypeSharedAcrossMotionPlusCardBranches(t *testing.T) {
	p := &groqProvider{}
	schema := p.AdaptSchema(buildQuestionSchema([]string{"GEOGRAPHY"}, []string{"Facile"}, []string{"MEMOTION_PLUS"}))

	branches := findMotionPlusCardBranches(t, schema)
	var shared map[string]bool
	for _, branch := range branches {
		props, ok := branch["properties"].(map[string]any)
		if !ok {
			t.Fatalf("Expected branch.properties to be a map, got %T", branch["properties"])
		}
		required, ok := branch["required"].([]string)
		if !ok {
			t.Fatalf("Expected branch.required to be a []string, got %T", branch["required"])
		}
		requiredSet := make(map[string]bool, len(required))
		for _, r := range required {
			requiredSet[r] = true
		}

		candidates := make(map[string]bool)
		for name, raw := range props {
			fieldSchema, ok := raw.(map[string]any)
			if !ok || !requiredSet[name] {
				continue
			}
			_, hasEnum := fieldSchema["enum"]
			_, hasConst := fieldSchema["const"]
			if hasEnum || hasConst {
				candidates[name] = true
			}
		}

		if shared == nil {
			shared = candidates
			continue
		}
		for name := range shared {
			if !candidates[name] {
				delete(shared, name)
			}
		}
	}

	if len(shared) != 1 || !shared["TYPE"] {
		t.Errorf("Expected exactly one discriminator candidate shared by both MOTION_CARDS branches (TYPE) — got %v", shared)
	}
}

// TestGroqNestedDiscriminator_ClassicMemotionMotionCardsStillUntouched is the
// non-regression companion: classic MEMOTION's MOTION_CARDS.items is a
// single flat object (no anyOf at all — see motionProps in ai_generator.go),
// so the new recursive walk must still find nothing to strip there, exactly
// like TestGroqProvider_AdaptSchema_DoesNotTouchMotionCardsNestedDifficulty
// already locks down for the (unrelated) single-type-active default case —
// this variant explicitly builds a schema with BOTH MEMOTION and
// MEMOTION_PLUS active, so classic MEMOTION's branch is exercised alongside
// the one that DOES get touched, in the same call.
func TestGroqNestedDiscriminator_ClassicMemotionMotionCardsStillUntouched(t *testing.T) {
	p := &groqProvider{}
	schema := p.AdaptSchema(buildQuestionSchema([]string{"GEOGRAPHY"}, []string{"Facile"}, []string{"MEMOTION", "MEMOTION_PLUS"}))

	memotion := findAnyOfVariantByTypeConst(t, schema, "MEMOTION")
	props := memotion["properties"].(map[string]any)
	motionCards, ok := props["MOTION_CARDS"].(map[string]any)
	if !ok {
		t.Fatalf("Expected MOTION_CARDS to be a map, got %T", props["MOTION_CARDS"])
	}
	items, ok := motionCards["items"].(map[string]any)
	if !ok {
		t.Fatalf("Expected MOTION_CARDS.items to be a map, got %T", motionCards["items"])
	}
	if _, hasAnyOf := items["anyOf"]; hasAnyOf {
		t.Fatal("Expected classic MEMOTION's MOTION_CARDS.items to have no anyOf — this test's own premise would be void")
	}
	cardProps, ok := items["properties"].(map[string]any)
	if !ok {
		t.Fatalf("Expected MOTION_CARDS.items.properties to be a map, got %T", items["properties"])
	}
	difficulty, ok := cardProps["DIFFICULTY"].(map[string]any)
	if !ok {
		t.Fatalf("Expected MOTION_CARDS.items.properties.DIFFICULTY to be a map, got %T", cardProps["DIFFICULTY"])
	}
	enumVals, ok := difficulty["enum"].([]int)
	if !ok || len(enumVals) != 3 {
		t.Errorf("Expected classic MEMOTION's nested DIFFICULTY.enum=[1,2,3] to survive untouched, got %v", difficulty["enum"])
	}
}

// ============================================================================
// Point 1 — schema size folded into the Groq max_tokens budget
// ============================================================================

func TestGroqRequestMaxTokens_LargerRequestEstimateReducesBudget(t *testing.T) {
	small := groqRequestMaxTokens(500)
	large := groqRequestMaxTokens(4000)
	if !(large < small) {
		t.Fatalf("expected a larger request-token estimate to reduce the max_tokens budget, got small=%d large=%d", small, large)
	}
}

func TestGroqRequestMaxTokens_NeverBelowFloor(t *testing.T) {
	got := groqRequestMaxTokens(groqMaxTokensBudget * 10) // pathologically oversized estimate
	if got != groqMinMaxTokens {
		t.Errorf("expected the floor (%d) to hold even for a huge estimate, got %d", groqMinMaxTokens, got)
	}
}

// TestGenerateViaGroq_MaxTokensAccountsForSchemaSize is the direct regression
// test for QUALIF v7.1.0.8 point 1: two real generateViaGroq calls, an
// identical prompt, but a small (1 active type) vs a large (all 6 active
// types) schema — the large-schema call MUST request fewer max_tokens than
// the small-schema call, because the schema itself now counts toward the
// budget estimate. Before this fix, both calls would have requested the
// EXACT SAME max_tokens (the schema was never part of the estimate) even
// though the large-schema call's true (prompt + schema + max_tokens) cost
// is measurably higher — the multi-type case QUALIF reported.
func TestGenerateViaGroq_MaxTokensAccountsForSchemaSize(t *testing.T) {
	var captured []int // MaxTokens from each captured request, in call order
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if mt, ok := body["max_tokens"].(float64); ok {
			captured = append(captured, int(mt))
		} else {
			t.Fatalf("expected max_tokens in request body, got %v", body["max_tokens"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": `{"questions":[]}`}}},
		})
	}))
	defer upstream.Close()
	t.Setenv(groqBaseURLEnvVar, upstream.URL)

	h := &HTTPServer{}
	aiCfg := config.AIConfig{GroqAPIKey: "gsk_test", GroqModel: "openai/gpt-oss-120b", TimeoutSeconds: 5}

	// A long-ish prompt (~1200 estimated tokens, matching a real multi-batch
	// generation prompt with extraContext/anti-duplicate guidance) is
	// deliberate: it keeps groqRequestMaxTokens' result for BOTH schemas
	// comfortably under groqMaxMaxTokens (6000) and above groqMinMaxTokens
	// (500), so the schema-size delta between the two calls is actually
	// visible in the captured max_tokens instead of being masked by either
	// clamp.
	prompt := strings.Repeat("Génère des questions sur le thème du cinéma français des années 80, niveau Facile. ", 70)

	smallSchema := buildQuestionSchema([]string{"ENTERTAINMENT"}, []string{"Facile"}, []string{"SPEEDY"})
	if _, err := h.generateViaGroq(context.Background(), aiCfg, prompt, smallSchema); err != nil {
		t.Fatalf("small-schema call failed: %v", err)
	}

	largeSchema := buildQuestionSchema([]string{"ENTERTAINMENT"}, []string{"Facile"}, generableQuestionTypes)
	if _, err := h.generateViaGroq(context.Background(), aiCfg, prompt, largeSchema); err != nil {
		t.Fatalf("large-schema call failed: %v", err)
	}

	if len(captured) != 2 {
		t.Fatalf("expected 2 captured requests, got %d", len(captured))
	}
	smallMaxTokens, largeMaxTokens := captured[0], captured[1]
	if !(largeMaxTokens < smallMaxTokens) {
		t.Errorf("expected the 6-type schema's max_tokens (%d) to be lower than the 1-type schema's (%d) — the schema size must reduce the budget", largeMaxTokens, smallMaxTokens)
	}
}
