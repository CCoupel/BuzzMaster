package server

// Tests for issue #142 (Groq anyOf discriminator ambiguity) — root cause
// confirmed by live-testing against the real Groq API (2026-08-07, see the
// handoff for the repro/fix transcript, not committed here since it depends
// on network access and a real API key). These tests are offline, structural
// checks of the schema AdaptSchema produces, per the task's explicit ask:
// "au moins un test qui valide que le schéma généré passe la validation
// attendue pour les 5 types, si possible sans dépendre d'un vrai appel
// réseau Groq pour les tests automatisés."

import (
	"testing"
)

// groqSchemaAnyOfBranches is a small test-local helper mirroring
// groqAnyOfBranches (ai_provider.go) but returning []map[string]any directly
// — tests want to iterate branch properties, not re-navigate the raw shape
// each time.
func groqSchemaAnyOfBranches(t *testing.T, schema map[string]any) []map[string]any {
	t.Helper()
	branches, ok := groqAnyOfBranches(schema)
	if !ok {
		t.Fatalf("Expected buildQuestionSchema's output to navigate to properties.questions.items.anyOf, got schema=%v", schema)
	}
	out := make([]map[string]any, 0, len(branches))
	for _, b := range branches {
		bm, ok := b.(map[string]any)
		if !ok {
			t.Fatalf("Expected each anyOf entry to be a map, got %T", b)
		}
		out = append(out, bm)
	}
	return out
}

// TestGroqProvider_AdaptSchema_StripsEnumFromCategoryAndDifficultyInEveryBranch
// is the direct regression guard for #142's confirmed root cause: Groq's
// strict-mode anyOf validator rejects a schema with more than one required,
// enum/const-bearing property shared across every branch. Before this fix,
// TYPE (const), CATEGORY (enum), and DIFFICULTY (enum) were all three such
// properties — exactly the three named in Groq's real error
// ("multiple candidate properties CATEGORY, DIFFICULTY, TYPE"). After
// AdaptSchema, only TYPE.const should remain: CATEGORY/DIFFICULTY keep
// "type":"string" and stay required, just without "enum".
func TestGroqProvider_AdaptSchema_StripsEnumFromCategoryAndDifficultyInEveryBranch(t *testing.T) {
	p := &groqProvider{}
	schema := p.AdaptSchema(buildQuestionSchema([]string{"ENTERTAINMENT", "HISTORY"}, []string{"Facile", "Moyen"}, generableQuestionTypes))

	branches := groqSchemaAnyOfBranches(t, schema)
	// #196 — MEMOTION_PLUS joined the top-level anyOf as its own branch
	// (contract ai-generation.md §3ter: a distinct discriminated variant,
	// not a flag under MEMOTION — see buildQuestionSchema). It shares the
	// exact same common{TYPE,CATEGORY,QUESTION,TIME,DIFFICULTY} base as
	// every other branch, so it must pass this exact same check.
	if len(branches) != 6 {
		t.Fatalf("Expected 6 anyOf branches (SPEEDY/QCM/MEMORY/MEMOTION/MEMOTION_PLUS/ARDOISE), got %d", len(branches))
	}

	for _, branch := range branches {
		props, ok := branch["properties"].(map[string]any)
		if !ok {
			t.Fatalf("Expected branch.properties to be a map, got %T (branch=%v)", branch["properties"], branch)
		}
		typeField, ok := props["TYPE"].(map[string]any)
		if !ok {
			t.Fatalf("Expected branch.properties.TYPE to be a map, got %T", props["TYPE"])
		}
		typeName, _ := typeField["const"].(string)

		category, ok := props["CATEGORY"].(map[string]any)
		if !ok {
			t.Fatalf("[%s] Expected properties.CATEGORY to be a map, got %T", typeName, props["CATEGORY"])
		}
		if _, hasEnum := category["enum"]; hasEnum {
			t.Errorf("[%s] Expected CATEGORY.enum to be stripped by AdaptSchema, still present: %v", typeName, category["enum"])
		}
		if category["type"] != "string" {
			t.Errorf("[%s] Expected CATEGORY to remain type=string after stripping enum, got %v", typeName, category["type"])
		}

		difficulty, ok := props["DIFFICULTY"].(map[string]any)
		if !ok {
			t.Fatalf("[%s] Expected properties.DIFFICULTY to be a map, got %T", typeName, props["DIFFICULTY"])
		}
		if _, hasEnum := difficulty["enum"]; hasEnum {
			t.Errorf("[%s] Expected DIFFICULTY.enum to be stripped by AdaptSchema, still present: %v", typeName, difficulty["enum"])
		}
		if difficulty["type"] != "string" {
			t.Errorf("[%s] Expected DIFFICULTY to remain type=string after stripping enum, got %v", typeName, difficulty["type"])
		}

		// TYPE itself is untouched — it's the intended, sole discriminator.
		if typeField["const"] == nil || typeField["const"] == "" {
			t.Errorf("Expected TYPE.const to remain set as the sole discriminator, got %v", typeField["const"])
		}

		required, ok := branch["required"].([]string)
		if !ok {
			t.Fatalf("[%s] Expected branch.required to be a []string, got %T", typeName, branch["required"])
		}
		for _, must := range []string{"TYPE", "CATEGORY", "DIFFICULTY"} {
			if !stringInSlice(required, must) {
				t.Errorf("[%s] Expected %s to remain required after AdaptSchema (only the enum constraint is dropped, not the field), got required=%v", typeName, must, required)
			}
		}
	}
}

// TestGroqProvider_AdaptSchema_OnlyOneSharedEnumCandidateAcrossAllBranches
// encodes the actual invariant Groq's validator enforces (per #142's
// confirmed mechanism, contract ai-multi-provider.md §7 amended): a property
// only competes as a discriminator candidate when it's required AND
// enum/const-constrained in EVERY branch — that's what made CATEGORY/
// DIFFICULTY/TYPE candidates together (all three common to all 5 branches)
// while QCM_CORRECT (enum, but only required in the QCM branch) and
// ARDOISE_KEYBOARD_TYPE (only in ARDOISE) never were. Computed as a set
// intersection across all branches rather than naming fields, so this would
// also catch a REGRESSION from a future 6th question type introducing a new
// enum-constrained field shared with the others.
func TestGroqProvider_AdaptSchema_OnlyOneSharedEnumCandidateAcrossAllBranches(t *testing.T) {
	p := &groqProvider{}
	schema := p.AdaptSchema(buildQuestionSchema([]string{"ENTERTAINMENT"}, []string{"Facile"}, generableQuestionTypes))

	branches := groqSchemaAnyOfBranches(t, schema)
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
			if fieldSchema["type"] != "string" {
				continue // Groq's candidates were all string-typed (TYPE/CATEGORY/DIFFICULTY); a nested array/object property doesn't compete
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
		t.Errorf("Expected exactly one discriminator candidate shared by all 5 branches (TYPE) — got %v", shared)
	}
}

// TestGroqProvider_AdaptSchema_DoesNotTouchMotionCardsNestedDifficulty
// guards against an overly broad implementation of AdaptSchema: MOTION_CARDS'
// per-card DIFFICULTY (an integer 1|2|3, nested one level inside the MEMOTION
// branch's MOTION_CARDS array items) was never named in Groq's error and is
// unrelated to the top-level anyOf discriminator ambiguity — it must survive
// untouched.
func TestGroqProvider_AdaptSchema_DoesNotTouchMotionCardsNestedDifficulty(t *testing.T) {
	p := &groqProvider{}
	schema := p.AdaptSchema(buildQuestionSchema([]string{"HISTORY"}, []string{"Difficile"}, generableQuestionTypes))

	var motionBranch map[string]any
	for _, branch := range groqSchemaAnyOfBranches(t, schema) {
		props, _ := branch["properties"].(map[string]any)
		typeField, _ := props["TYPE"].(map[string]any)
		if typeField["const"] == "MEMOTION" {
			motionBranch = branch
			break
		}
	}
	if motionBranch == nil {
		t.Fatal("Expected a MEMOTION variant (TYPE.const=\"MEMOTION\") in the anyOf list")
	}

	props := motionBranch["properties"].(map[string]any)
	motionCards, ok := props["MOTION_CARDS"].(map[string]any)
	if !ok {
		t.Fatalf("Expected MOTION_CARDS to be a map, got %T", props["MOTION_CARDS"])
	}
	items, ok := motionCards["items"].(map[string]any)
	if !ok {
		t.Fatalf("Expected MOTION_CARDS.items to be a map, got %T", motionCards["items"])
	}
	cardProps, ok := items["properties"].(map[string]any)
	if !ok {
		t.Fatalf("Expected MOTION_CARDS.items.properties to be a map, got %T", items["properties"])
	}
	cardDifficulty, ok := cardProps["DIFFICULTY"].(map[string]any)
	if !ok {
		t.Fatalf("Expected MOTION_CARDS.items.properties.DIFFICULTY to be a map, got %T", cardProps["DIFFICULTY"])
	}
	if cardDifficulty["type"] != "integer" {
		t.Errorf("Expected MOTION_CARDS' nested DIFFICULTY to remain type=integer, got %v", cardDifficulty["type"])
	}
	enumVals, ok := cardDifficulty["enum"].([]int)
	if !ok || len(enumVals) != 3 {
		t.Errorf("Expected MOTION_CARDS' nested DIFFICULTY.enum=[1,2,3] to survive untouched, got %v", cardDifficulty["enum"])
	}
}

// TestAnthropicProvider_AdaptSchema_IsUnaffectedByGroqFix is the explicit
// non-regression check the task called for (point 5): Anthropic's schema
// must keep the CATEGORY/DIFFICULTY enum exactly as before #142's fix —
// anthropicProvider.AdaptSchema is a completely separate no-op code path,
// this test proves it stayed that way rather than trusting the comment.
func TestAnthropicProvider_AdaptSchema_IsUnaffectedByGroqFix(t *testing.T) {
	p := &anthropicProvider{}
	schema := p.AdaptSchema(buildQuestionSchema([]string{"ENTERTAINMENT"}, []string{"Facile", "Moyen"}, generableQuestionTypes))

	for _, branch := range groqSchemaAnyOfBranches(t, schema) {
		props := branch["properties"].(map[string]any)
		category, ok := props["CATEGORY"].(map[string]any)
		if !ok {
			t.Fatalf("Expected CATEGORY to be a map, got %T", props["CATEGORY"])
		}
		if _, hasEnum := category["enum"]; !hasEnum {
			t.Error("Expected Anthropic's CATEGORY to keep its enum constraint (AdaptSchema must remain a no-op for Anthropic)")
		}
		difficulty, ok := props["DIFFICULTY"].(map[string]any)
		if !ok {
			t.Fatalf("Expected DIFFICULTY to be a map, got %T", props["DIFFICULTY"])
		}
		if _, hasEnum := difficulty["enum"]; !hasEnum {
			t.Error("Expected Anthropic's DIFFICULTY to keep its enum constraint (AdaptSchema must remain a no-op for Anthropic)")
		}
	}
}

// TestValidateGeneratedQuestions_RejectsOffListDifficulty covers the
// compensating server-side control added alongside the schema fix: with
// Groq's schema no longer restricting DIFFICULTY to the requested set, this
// check is now the only guarantee left for that provider (contract
// ai-generation.md §5.1, amended).
func TestValidateGeneratedQuestions_RejectsOffListDifficulty(t *testing.T) {
	raw := []llmRawQuestion{
		{Type: "SPEEDY", Category: "ENTERTAINMENT", Question: "Q1", Time: 20, Difficulty: "Moyen", Answer: "A1"},
		{Type: "SPEEDY", Category: "ENTERTAINMENT", Question: "Q2", Time: 20, Difficulty: "Legendaire", Answer: "A2"}, // not in the requested set
	}
	valid, skipped := validateGeneratedQuestions(raw, []string{"ENTERTAINMENT"}, []string{"Moyen", "Difficile"}, 200)

	if len(valid) != 1 {
		t.Fatalf("Expected exactly 1 valid question (the off-list difficulty one dropped), got %d: %v", len(valid), valid)
	}
	if valid[0].Difficulty != "Moyen" {
		t.Errorf("Expected the surviving question to be the Moyen one, got %q", valid[0].Difficulty)
	}
	if len(skipped) != 1 {
		t.Fatalf("Expected exactly 1 skip reason, got %d: %v", len(skipped), skipped)
	}
}
