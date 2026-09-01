package server

// Tests dérivés de contracts/rafale-ai-generation.md (issue #203, milestone
// v8.1.0) et de _work/reports/plan-20260901-162105.md (tâches 4/5/6) — suite
// Go unitaire pour le socle de génération RAFALE : validation de la requête,
// schéma LLM plat, validation des questions générées (plafonds de brièveté,
// doublons), prompt. dev-backend et test-writer tournent en parallèle sur le
// même contrat (même motif que ai_generation_test.go/ai_validate_test.go).
//
// Écrit contre l'implémentation réelle de internal/server/ai_generator_rafale.go
// (dev-backend a livré ce fichier en parallèle, Batch 1) :
//
//	generateRafaleRequest{Theme, Populations, Language, Instructions,
//	    Categories, Difficulties []int, Count int}
//	validateGenerateRafaleRequest(req *generateRafaleRequest, aiCfg config.AIConfig) string
//	buildRafaleQuestionSchema(categories []string, difficulties []int) map[string]any
//	llmRawRafaleQuestion{Question, Answer, Category string; Difficulty int}
//	validateGeneratedRafaleQuestions(raw []llmRawRafaleQuestion, allowedCategories []string,
//	    allowedDifficulties []int, maxQuestions int, existingNormalized map[string]bool) (valid []llmRawRafaleQuestion, reasons []string)
//	buildRafaleGenerationPrompt(req generateRafaleRequest, batchCount int, targetCategory string, targetDifficulty int, extraContext []string) string
//	allocateRafaleCounts(categories []string, difficulties []int, count int) []rafaleCouple
//	rafaleGenerationPresets = []int{10, 20, 50, 100, 200}          (contract §2bis)
//	rafaleMaxQuestionRunes = 100, rafaleMaxAnswerRunes = 40         (contract §5.1)
//
// ⚠️ Comme le reste du package (config.SetInstance mute un singleton
// global) : pas de t.Parallel() dans ce fichier.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"buzzcontrol/internal/game"
)

// writeQuestionFixture writes a minimal question.json directly under
// dataDir/files/questions/<id>/ — used only to prove the RAFALE prompt's
// anti-duplicate context does NOT read from files/questions/ (contract §5),
// unlike collectExistingQuestionsContext on the Quiz path. No equivalent
// helper existed in this package (only readSavedQuestion, which reads back
// what a real upload wrote) — this is the write-side counterpart, scoped to
// this file.
func writeQuestionFixture(t *testing.T, dataDir, id string, fields map[string]interface{}) {
	t.Helper()
	dir := filepath.Join(dataDir, "files", "questions", id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create question fixture dir: %v", err)
	}
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("failed to marshal question fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "question.json"), data, 0644); err != nil {
		t.Fatalf("failed to write question fixture: %v", err)
	}
}

// ================================================================================
// Constantes partagées (contract §2bis, §5.1)
// ================================================================================

func TestRafaleGenerationPresets_ExactSet(t *testing.T) {
	want := []int{10, 20, 50, 100, 200}
	if len(rafaleGenerationPresets) != len(want) {
		t.Fatalf("expected %d presets, got %d: %v", len(want), len(rafaleGenerationPresets), rafaleGenerationPresets)
	}
	for i, v := range want {
		if rafaleGenerationPresets[i] != v {
			t.Errorf("preset[%d] = %d, want %d (contract §2bis: {10,20,50,100,200})", i, rafaleGenerationPresets[i], v)
		}
	}
}

func TestRafaleLengthCaps_MatchContract(t *testing.T) {
	if rafaleMaxQuestionRunes != 100 {
		t.Errorf("rafaleMaxQuestionRunes = %d, want 100 (contract §5.1)", rafaleMaxQuestionRunes)
	}
	if rafaleMaxAnswerRunes != 40 {
		t.Errorf("rafaleMaxAnswerRunes = %d, want 40 (contract §5.1)", rafaleMaxAnswerRunes)
	}
}

// ================================================================================
// validateGenerateRafaleRequest (contract §2 tableau de validation)
// ================================================================================

func baseRafaleGenerateRequest() generateRafaleRequest {
	return generateRafaleRequest{
		Theme:        "Culture générale — France",
		Populations:  []string{"Adulte (18-64 ans)"},
		Language:     "Français",
		Instructions: "",
		Categories:   []string{string(game.CategoryHistory), string(game.CategoryScience)},
		Difficulties: []int{1, 2},
		Count:        50,
	}
}

func TestValidateGenerateRafaleRequest_ValidRequest_NoError(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	req := baseRafaleGenerateRequest()
	if msg := server.validateGenerateRafaleRequest(&req, validAIConfig()); msg != "" {
		t.Errorf("expected no validation error for a well-formed request, got %q", msg)
	}
}

func TestValidateGenerateRafaleRequest_Rules(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(r *generateRafaleRequest)
	}{
		{"empty theme", func(r *generateRafaleRequest) { r.Theme = "  " }},
		{"theme too long", func(r *generateRafaleRequest) { r.Theme = strings.Repeat("a", 201) }},
		{"no populations", func(r *generateRafaleRequest) { r.Populations = nil }},
		{"unknown population", func(r *generateRafaleRequest) { r.Populations = []string{"Extraterrestre"} }},
		{"duplicate population", func(r *generateRafaleRequest) {
			r.Populations = []string{"Adulte (18-64 ans)", "Adulte (18-64 ans)"}
		}},
		{"unknown language", func(r *generateRafaleRequest) { r.Language = "Klingon" }},
		{"instructions too long", func(r *generateRafaleRequest) { r.Instructions = strings.Repeat("a", 2001) }},
		{"no categories", func(r *generateRafaleRequest) { r.Categories = nil }},
		{"unknown category", func(r *generateRafaleRequest) { r.Categories = []string{"NOT_A_REAL_CATEGORY"} }},
		{"duplicate category", func(r *generateRafaleRequest) {
			r.Categories = []string{string(game.CategoryHistory), string(game.CategoryHistory)}
		}},
		{"no difficulties", func(r *generateRafaleRequest) { r.Difficulties = nil }},
		{"difficulty below range", func(r *generateRafaleRequest) { r.Difficulties = []int{0} }},
		{"difficulty above range", func(r *generateRafaleRequest) { r.Difficulties = []int{4} }},
		{"duplicate difficulty", func(r *generateRafaleRequest) { r.Difficulties = []int{1, 1} }},
		{"count not a preset (0)", func(r *generateRafaleRequest) { r.Count = 0 }},
		{"count not a preset (7)", func(r *generateRafaleRequest) { r.Count = 7 }},
		{"count not a preset (10000)", func(r *generateRafaleRequest) { r.Count = 10000 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := setupTestHTTPServer(t)
			req := baseRafaleGenerateRequest()
			tt.mutate(&req)
			if msg := server.validateGenerateRafaleRequest(&req, validAIConfig()); msg == "" {
				t.Errorf("expected a validation error for case %q, got none", tt.name)
			}
		})
	}
}

func TestValidateGenerateRafaleRequest_PresetAbovMaxQuestions_Rejected(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	req := baseRafaleGenerateRequest()
	req.Count = 200
	ai := validAIConfig()
	ai.MaxQuestions = 100 // 200 is a valid preset, but above this server's configured ceiling
	if msg := server.validateGenerateRafaleRequest(&req, ai); msg == "" {
		t.Error("expected a validation error when the requested preset exceeds ai.max_questions")
	}
}

func TestValidateGenerateRafaleRequest_PresetAtOrBelowMaxQuestions_Accepted(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	req := baseRafaleGenerateRequest()
	req.Count = 50
	ai := validAIConfig()
	ai.MaxQuestions = 50
	if msg := server.validateGenerateRafaleRequest(&req, ai); msg != "" {
		t.Errorf("expected no error when the preset equals ai.max_questions exactly, got %q", msg)
	}
}

func TestValidateGenerateRafaleRequest_UsesIsKnownRafaleCategory_NotResolveCategoryMeta(t *testing.T) {
	// contract §3: the RAFALE path validates categories against
	// isKnownRafaleCategory (the same check as POST /api/rafale/questions),
	// NOT ResolveCategoryMeta (the Quiz path's check) — the 8 hardcoded
	// categories are known to both, so this test exercises a value that is
	// only meaningful through isKnownRafaleCategory's exact semantics: an
	// empty/blank category key must be rejected the same way the reservoir
	// editor already rejects it.
	server, _ := setupTestHTTPServer(t)
	req := baseRafaleGenerateRequest()
	req.Categories = []string{""}
	if msg := server.validateGenerateRafaleRequest(&req, validAIConfig()); msg == "" {
		t.Error("expected an empty category key to be rejected")
	}
}

// ================================================================================
// buildRafaleQuestionSchema (contract §4 — schéma plat)
// ================================================================================

func TestBuildRafaleQuestionSchema_NoAnyOf(t *testing.T) {
	schema := buildRafaleQuestionSchema([]string{"HISTORY", "SCIENCE"}, []int{1, 2, 3})
	if containsAnyOfKey(schema) {
		t.Error("buildRafaleQuestionSchema must never produce an \"anyOf\" node (contract §4 — the whole point of the flat schema is to make Groq's discriminator ambiguity structurally impossible)")
	}
}

func TestBuildRafaleQuestionSchema_AdditionalPropertiesFalse_AtEveryObjectLevel(t *testing.T) {
	schema := buildRafaleQuestionSchema([]string{"HISTORY"}, []int{1})
	if schema["additionalProperties"] != false {
		t.Errorf("root schema must have additionalProperties:false, got %v", schema["additionalProperties"])
	}
	props, _ := schema["properties"].(map[string]any)
	questions, _ := props["questions"].(map[string]any)
	items, _ := questions["items"].(map[string]any)
	if items["additionalProperties"] != false {
		t.Errorf("questions[].items must have additionalProperties:false, got %v", items["additionalProperties"])
	}
}

func TestBuildRafaleQuestionSchema_NoIDField(t *testing.T) {
	schema := buildRafaleQuestionSchema([]string{"HISTORY"}, []int{1})
	props, _ := schema["properties"].(map[string]any)
	questions, _ := props["questions"].(map[string]any)
	items, _ := questions["items"].(map[string]any)
	itemProps, _ := items["properties"].(map[string]any)
	if _, hasID := itemProps["ID"]; hasID {
		t.Error("buildRafaleQuestionSchema must NEVER expose an ID field — contract §4's \"règle d'or\": the model must be structurally unable to designate an existing reservoir entry")
	}
	for _, required := range []string{"QUESTION", "ANSWER", "CATEGORY", "DIFFICULTY"} {
		if _, ok := itemProps[required]; !ok {
			t.Errorf("expected schema property %q, missing from %v", required, itemProps)
		}
	}
}

func TestBuildRafaleQuestionSchema_EnumsMatchRequest(t *testing.T) {
	categories := []string{"HISTORY", "SCIENCE"}
	difficulties := []int{2, 3}
	schema := buildRafaleQuestionSchema(categories, difficulties)
	props, _ := schema["properties"].(map[string]any)
	questions, _ := props["questions"].(map[string]any)
	items, _ := questions["items"].(map[string]any)
	itemProps, _ := items["properties"].(map[string]any)

	catProp, _ := itemProps["CATEGORY"].(map[string]any)
	catEnum, _ := catProp["enum"].([]any)
	if len(catEnum) != 2 || catEnum[0] != "HISTORY" || catEnum[1] != "SCIENCE" {
		t.Errorf("CATEGORY enum must mirror the request's categories exactly, got %v", catProp["enum"])
	}

	diffProp, _ := itemProps["DIFFICULTY"].(map[string]any)
	if diffProp["type"] != "integer" {
		t.Errorf("DIFFICULTY must be an integer type, got %v", diffProp["type"])
	}
}

// containsAnyOfKey walks a decoded JSON-schema-shaped map[string]any tree
// looking for any "anyOf" key at any depth.
func containsAnyOfKey(node any) bool {
	switch v := node.(type) {
	case map[string]any:
		if _, ok := v["anyOf"]; ok {
			return true
		}
		for _, child := range v {
			if containsAnyOfKey(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if containsAnyOfKey(child) {
				return true
			}
		}
	}
	return false
}

// ================================================================================
// validateGeneratedRafaleQuestions (contract §5 — brièveté, §5.4 autres
// contrôles, §5.5 raisons)
// ================================================================================

func rafaleRaw(question, answer, category string, difficulty int) llmRawRafaleQuestion {
	return llmRawRafaleQuestion{Question: question, Answer: answer, Category: category, Difficulty: difficulty}
}

func TestValidateGeneratedRafaleQuestions_AcceptsWellFormedQuestion(t *testing.T) {
	valid, reasons := validateGeneratedRafaleQuestions(
		[]llmRawRafaleQuestion{rafaleRaw("Quelle est la capitale de la France ?", "Paris", "HISTORY", 1)},
		[]string{"HISTORY"}, []int{1}, 200, nil,
	)
	if len(valid) != 1 {
		t.Fatalf("expected 1 accepted question, got %d (reasons: %v)", len(valid), reasons)
	}
}

// 🔴 Rune boundary, not byte boundary — contract §5.1 "jamais en octets".
// "é" and other accented/emoji characters must count as ONE rune each.
func TestValidateGeneratedRafaleQuestions_LengthCaps_RunesNotBytes(t *testing.T) {
	// 100 accented runes ('é' is 2 bytes in UTF-8: 200 bytes, 100 runes) — must be ACCEPTED.
	question100 := strings.Repeat("é", 100)
	if n := utf8.RuneCountInString(question100); n != 100 {
		t.Fatalf("test fixture bug: expected 100 runes, got %d", n)
	}
	valid, reasons := validateGeneratedRafaleQuestions(
		[]llmRawRafaleQuestion{rafaleRaw(question100, "Oui", "HISTORY", 1)},
		[]string{"HISTORY"}, []int{1}, 200, nil,
	)
	if len(valid) != 1 {
		t.Fatalf("a 100-rune (200-byte) question must be ACCEPTED (rune count, not byte count) — got 0 accepted, reasons: %v", reasons)
	}

	// 101 accented runes — must be REJECTED.
	question101 := strings.Repeat("é", 101)
	valid, reasons = validateGeneratedRafaleQuestions(
		[]llmRawRafaleQuestion{rafaleRaw(question101, "Oui", "HISTORY", 1)},
		[]string{"HISTORY"}, []int{1}, 200, nil,
	)
	if len(valid) != 0 {
		t.Fatalf("a 101-rune question must be REJECTED, got %d accepted", len(valid))
	}
	if len(reasons) == 0 {
		t.Error("expected a reason to be recorded for the 101-rune rejection")
	}

	// Answer: 40 runes accepted, 41 rejected — same rune-counting rule,
	// exercised with an emoji (a multi-byte rune) rather than an accent, to
	// cover a different UTF-8 byte width.
	answer40 := strings.Repeat("🎉", 40)
	valid, reasons = validateGeneratedRafaleQuestions(
		[]llmRawRafaleQuestion{rafaleRaw("Une question valide ?", answer40, "HISTORY", 1)},
		[]string{"HISTORY"}, []int{1}, 200, nil,
	)
	if len(valid) != 1 {
		t.Fatalf("a 40-rune emoji answer must be ACCEPTED, got 0 accepted, reasons: %v", reasons)
	}

	answer41 := strings.Repeat("🎉", 41)
	valid, reasons = validateGeneratedRafaleQuestions(
		[]llmRawRafaleQuestion{rafaleRaw("Une question valide ?", answer41, "HISTORY", 1)},
		[]string{"HISTORY"}, []int{1}, 200, nil,
	)
	if len(valid) != 0 {
		t.Fatalf("a 41-rune emoji answer must be REJECTED, got %d accepted", len(valid))
	}
}

// 🔴 The central non-negotiable rule of #203 (contract §5.2): a rejected
// question is ABSENT from the accepted list, never present in a shortened
// form. This test would catch a regression where someone "helpfully"
// truncates instead of dropping.
func TestValidateGeneratedRafaleQuestions_RejectedIsAbsentNeverTruncated(t *testing.T) {
	tooLong := strings.Repeat("x", 150)
	valid, _ := validateGeneratedRafaleQuestions(
		[]llmRawRafaleQuestion{rafaleRaw(tooLong, "Answer", "HISTORY", 1)},
		[]string{"HISTORY"}, []int{1}, 200, nil,
	)
	for _, q := range valid {
		if strings.HasPrefix(tooLong, q.Question) && q.Question != tooLong {
			t.Fatalf("a too-long question must be ENTIRELY ABSENT from the accepted list, not present in a truncated form: got %q (len=%d runes)", q.Question, utf8.RuneCountInString(q.Question))
		}
	}
	if len(valid) != 0 {
		t.Fatalf("expected the over-length question to be entirely rejected, got %d accepted: %+v", len(valid), valid)
	}
}

func TestValidateGeneratedRafaleQuestions_RejectsEmptyFieldsAfterTrim(t *testing.T) {
	valid, reasons := validateGeneratedRafaleQuestions(
		[]llmRawRafaleQuestion{
			rafaleRaw("   ", "Answer", "HISTORY", 1),     // empty question after trim
			rafaleRaw("Question ?", "   ", "HISTORY", 1), // empty answer after trim
		},
		[]string{"HISTORY"}, []int{1}, 200, nil,
	)
	if len(valid) != 0 {
		t.Errorf("expected both blank-after-trim questions to be rejected, got %d accepted", len(valid))
	}
	if len(reasons) != 2 {
		t.Errorf("expected 2 rejection reasons, got %d: %v", len(reasons), reasons)
	}
}

func TestValidateGeneratedRafaleQuestions_RejectsCategoryOutsideFilter(t *testing.T) {
	valid, reasons := validateGeneratedRafaleQuestions(
		[]llmRawRafaleQuestion{rafaleRaw("Q?", "A", "GEOGRAPHY", 1)},
		[]string{"HISTORY"}, []int{1}, 200, nil,
	)
	if len(valid) != 0 {
		t.Errorf("expected a category outside the requested filter to be rejected, got %d accepted", len(valid))
	}
	if len(reasons) == 0 {
		t.Error("expected a rejection reason for the out-of-filter category")
	}
}

func TestValidateGeneratedRafaleQuestions_RejectsDifficultyOutsideFilter(t *testing.T) {
	valid, _ := validateGeneratedRafaleQuestions(
		[]llmRawRafaleQuestion{rafaleRaw("Q?", "A", "HISTORY", 3)},
		[]string{"HISTORY"}, []int{1, 2}, 200, nil,
	)
	if len(valid) != 0 {
		t.Errorf("expected a difficulty outside the requested filter to be rejected, got %d accepted", len(valid))
	}
}

func TestValidateGeneratedRafaleQuestions_DuplicateAgainstReservoir_CaseAndSpaceInsensitive(t *testing.T) {
	existingNormalized := map[string]bool{
		normalizeRafaleQuestionText("Quelle est la capitale de la France ?"): true,
	}
	valid, reasons := validateGeneratedRafaleQuestions(
		[]llmRawRafaleQuestion{rafaleRaw("  quelle EST la capitale de la FRANCE ?  ", "Paris", "HISTORY", 1)},
		[]string{"HISTORY"}, []int{1}, 200, existingNormalized,
	)
	if len(valid) != 0 {
		t.Errorf("expected a case/space-insensitive duplicate of an existing reservoir question to be rejected, got %d accepted", len(valid))
	}
	if len(reasons) == 0 {
		t.Error("expected a rejection reason for the duplicate")
	}
}

func TestValidateGeneratedRafaleQuestions_DuplicateWithinSameJob_Rejected(t *testing.T) {
	valid, _ := validateGeneratedRafaleQuestions(
		[]llmRawRafaleQuestion{
			rafaleRaw("Quelle est la capitale de l'Italie ?", "Rome", "HISTORY", 1),
			rafaleRaw("quelle est la capitale de l'italie ?", "Rome", "HISTORY", 1), // same job, case-insensitive dup
		},
		[]string{"HISTORY"}, []int{1}, 200, nil,
	)
	if len(valid) != 1 {
		t.Errorf("expected exactly 1 of the 2 in-job duplicates to be accepted, got %d", len(valid))
	}
}

func TestValidateGeneratedRafaleQuestions_DistinctQuestions_NotFlaggedAsDuplicates(t *testing.T) {
	existingNormalized := map[string]bool{
		normalizeRafaleQuestionText("Quelle est la capitale de la France ?"): true,
	}
	valid, reasons := validateGeneratedRafaleQuestions(
		[]llmRawRafaleQuestion{rafaleRaw("Quelle est la capitale de l'Espagne ?", "Madrid", "HISTORY", 1)},
		[]string{"HISTORY"}, []int{1}, 200, existingNormalized,
	)
	if len(valid) != 1 {
		t.Errorf("a genuinely distinct question must not be rejected as a duplicate, got %d accepted (reasons: %v)", len(valid), reasons)
	}
}

func TestNormalizeRafaleQuestionText_CaseAndSpaceInsensitive(t *testing.T) {
	a := normalizeRafaleQuestionText("  Quelle EST la Capitale   de la France ?  ")
	b := normalizeRafaleQuestionText("quelle est la capitale de la france ?")
	if a != b {
		t.Errorf("expected case/space-insensitive normalization to produce identical keys, got %q vs %q", a, b)
	}
}

func TestValidateGeneratedRafaleQuestions_VolumeExcessDropped(t *testing.T) {
	raw := []llmRawRafaleQuestion{
		rafaleRaw("Q1 ?", "A1", "HISTORY", 1),
		rafaleRaw("Q2 ?", "A2", "HISTORY", 1),
		rafaleRaw("Q3 ?", "A3", "HISTORY", 1),
	}
	valid, reasons := validateGeneratedRafaleQuestions(raw, []string{"HISTORY"}, []int{1}, 2, nil)
	if len(valid) != 2 {
		t.Errorf("expected exactly 2 accepted questions (maxQuestions=2), got %d", len(valid))
	}
	if len(reasons) == 0 {
		t.Error("expected a \"volume\" reason recorded for the dropped excess question")
	}
}

// ================================================================================
// buildRafaleGenerationPrompt (contract §5 justification, plan tâche 5)
// ================================================================================

func TestBuildRafaleGenerationPrompt_ContainsBrevityInstruction(t *testing.T) {
	req := baseRafaleGenerateRequest()
	server, _ := setupTestHTTPServer(t)
	prompt := server.buildRafaleGenerationPrompt(req, 10, string(game.CategoryHistory), 1, nil)
	if !strings.Contains(prompt, "80") || !strings.Contains(prompt, "25") {
		t.Errorf("expected the brevity instruction to cite the prompt-level targets (≤80 chars question, ≤25 chars answer, contract §5.1), got prompt:\n%s", prompt)
	}
}

func TestBuildRafaleGenerationPrompt_ContainsTargetCouple(t *testing.T) {
	req := baseRafaleGenerateRequest()
	server, _ := setupTestHTTPServer(t)
	prompt := server.buildRafaleGenerationPrompt(req, 10, string(game.CategoryHistory), 2, nil)
	if !strings.Contains(prompt, string(game.CategoryHistory)) {
		t.Errorf("expected the target couple's category to appear in the prompt, got:\n%s", prompt)
	}
}

func TestBuildRafaleGenerationPrompt_AntiDuplicateContext_Included(t *testing.T) {
	req := baseRafaleGenerateRequest()
	server, _ := setupTestHTTPServer(t)
	extra := []string{"Une question déjà dans le réservoir ?"}
	prompt := server.buildRafaleGenerationPrompt(req, 10, string(game.CategoryHistory), 1, extra)
	if !strings.Contains(prompt, "Une question déjà dans le réservoir ?") {
		t.Errorf("expected the anti-duplicate context to be injected into the prompt, got:\n%s", prompt)
	}
}

// ================================================================================
// collectExistingRafaleContext (contract §5 — sourcé du RÉSERVOIR, pas de
// files/questions/, plafonné à 150 entrées)
// ================================================================================

func TestCollectExistingRafaleContext_SourcedFromReservoir_NotQuestionsDir(t *testing.T) {
	// contract §5: anti-duplicate context is fed from the RESERVOIR (targeted
	// couples), never from files/questions/ (the Quiz path's own context
	// source) — a question living only in files/questions/ must not leak
	// into the RAFALE context, while a reservoir-seeded one must appear.
	server, dataDir := setupTestHTTPServer(t)
	writeQuestionFixture(t, dataDir, "1", map[string]interface{}{
		"TYPE": "SPEEDY", "CATEGORY": string(game.CategoryHistory), "QUESTION": "QUIZ-ONLY-MARKER-SHOULD-NOT-LEAK",
	})
	if _, err := server.engine.UpsertRafaleQuestion(game.RafaleQuestion{
		Question: "RESERVOIR-MARKER-SHOULD-APPEAR", Answer: "A", Category: game.CategoryHistory, Difficulty: 1,
	}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	context := server.collectExistingRafaleContext([]string{string(game.CategoryHistory)}, []int{1})

	found := false
	for _, c := range context {
		if strings.Contains(c, "QUIZ-ONLY-MARKER-SHOULD-NOT-LEAK") {
			t.Error("the RAFALE anti-duplicate context must never come from files/questions/ (contract §5)")
		}
		if strings.Contains(c, "RESERVOIR-MARKER-SHOULD-APPEAR") {
			found = true
		}
	}
	if !found {
		t.Error("expected the reservoir-seeded question to appear in the anti-duplicate context")
	}
}

func TestCollectExistingRafaleContext_FiltersByCoupleAndCapsAt150(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	for i := 0; i < 160; i++ {
		if _, err := server.engine.UpsertRafaleQuestion(game.RafaleQuestion{
			Question: "In-filter question", Answer: "A", Category: game.CategoryHistory, Difficulty: 1,
		}); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	if _, err := server.engine.UpsertRafaleQuestion(game.RafaleQuestion{
		Question: "Wrong difficulty — must be excluded", Answer: "A", Category: game.CategoryHistory, Difficulty: 2,
	}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	context := server.collectExistingRafaleContext([]string{string(game.CategoryHistory)}, []int{1})
	if len(context) > 150 {
		t.Errorf("expected the context to be capped at 150 entries, got %d", len(context))
	}
	for _, c := range context {
		if strings.Contains(c, "Wrong difficulty") {
			t.Error("a question outside the requested couple (wrong difficulty) must not appear in the context")
		}
	}
}

// ================================================================================
// allocateRafaleCounts (contract §2bis — répartition du volume sur les
// couples catégorie × difficulté)
// ================================================================================

func TestAllocateRafaleCounts_UniformDistribution_NoRemainder(t *testing.T) {
	couples := allocateRafaleCounts([]string{"HISTORY", "SCIENCE"}, []int{1, 2}, 40)
	if len(couples) != 4 {
		t.Fatalf("expected 4 couples (2 categories x 2 difficulties), got %d: %+v", len(couples), couples)
	}
	for _, c := range couples {
		if c.Count != 10 {
			t.Errorf("expected each of the 4 couples to get 10 (40/4), got %d for %+v", c.Count, c)
		}
	}
}

func TestAllocateRafaleCounts_RemainderGoesToFirstCouplesInRequestOrder(t *testing.T) {
	// 10 questions over 3 couples: 3,3,3 + 1 remainder -> first couple gets 4.
	couples := allocateRafaleCounts([]string{"HISTORY"}, []int{1, 2, 3}, 10)
	if len(couples) != 3 {
		t.Fatalf("expected 3 couples, got %d: %+v", len(couples), couples)
	}
	total := 0
	for _, c := range couples {
		total += c.Count
	}
	if total != 10 {
		t.Fatalf("expected the couples' counts to sum to the requested total (10), got %d: %+v", total, couples)
	}
	if couples[0].Count != 4 {
		t.Errorf("expected the FIRST couple (request order) to absorb the remainder (4 = 3+1), got %d: %+v", couples[0].Count, couples)
	}
	for _, c := range couples[1:] {
		if c.Count != 3 {
			t.Errorf("expected the remaining couples to get the base share (3), got %d: %+v", c.Count, c)
		}
	}
}

func TestAllocateRafaleCounts_ZeroShareCouplesOmitted(t *testing.T) {
	// 2 questions over 4 couples: only the first 2 (request order) get a
	// non-zero share; the other 2 must be OMITTED from the result — nothing
	// to generate for a couple assigned 0.
	couples := allocateRafaleCounts([]string{"HISTORY", "SCIENCE"}, []int{1, 2}, 2)
	if len(couples) != 2 {
		t.Fatalf("expected exactly 2 couples with a non-zero share, got %d: %+v", len(couples), couples)
	}
	for _, c := range couples {
		if c.Count != 1 {
			t.Errorf("expected each of the 2 non-zero couples to get 1, got %d: %+v", c.Count, c)
		}
	}
}
