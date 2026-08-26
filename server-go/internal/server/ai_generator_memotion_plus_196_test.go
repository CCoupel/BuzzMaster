// Tests for #196 — AI generation of nested QCM cards, pseudo-type
// MEMOTION_PLUS (contracts/ai-generation.md §3ter). Mirrors
// ai_motion_card_no_type_184_test.go's structure: schema shape, then the
// actual mapGeneratedQuestion conversion path, then a full round-trip
// through the real game.Question/MotionCard types.
//
// Run: go test ./internal/server/... -run TestMemotionPlus -v
package server

import (
	"encoding/json"
	"testing"

	"buzzcontrol/internal/game"
)

// ============================================================================
// B1 — generableQuestionTypes / distribution validation
// ============================================================================

func TestMemotionPlus_GenerableQuestionTypes_Includes(t *testing.T) {
	if !stringInSlice(generableQuestionTypes, "MEMOTION_PLUS") {
		t.Fatal(`generableQuestionTypes does not include "MEMOTION_PLUS"`)
	}
}

// ============================================================================
// B2 — schema shape
// ============================================================================

// findAnyOfVariantByRequiredKey walks a buildQuestionSchema()'s top-level
// anyOf and returns the first variant whose properties map contains key —
// shared helper, mirrors the inline walk in
// TestAIGeneratedMotionCards_SchemaHasNoTypeProperty (ai_motion_card_no_type_184_test.go)
// but parameterized so this file doesn't duplicate that walk twice for the
// two MOTION_CARDS-bearing variants (classic MEMOTION, MEMOTION_PLUS).
func findAnyOfVariantByTypeConst(t *testing.T, schema map[string]any, typeConst string) map[string]any {
	t.Helper()
	anyOf := schema["properties"].(map[string]any)["questions"].(map[string]any)["items"].(map[string]any)["anyOf"].([]any)
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
		if c, _ := typeSchema["const"].(string); c == typeConst {
			return variant
		}
	}
	t.Fatalf("no anyOf variant found with TYPE const %q", typeConst)
	return nil
}

// TestMemotionPlus_Schema_ClassicMemotionUnchanged is the explicit
// non-regression check the contract calls out ("La variante MEMOTION
// classique garde ses 4 propriétés à plat... aucun changement là"):
// TestAIGeneratedMotionCards_SchemaHasNoTypeProperty (#184) already covers
// this exhaustively by walking to the FIRST variant declaring MOTION_CARDS
// — which, after this change, would find MEMOTION_PLUS's variant too if
// ordering ever changed. This test pins it down by TYPE const instead of
// "first match", immune to anyOf ordering.
func TestMemotionPlus_Schema_ClassicMemotionUnchanged(t *testing.T) {
	schema := buildQuestionSchema([]string{"GEOGRAPHY"}, []string{"Facile"})
	classic := findAnyOfVariantByTypeConst(t, schema, "MEMOTION")

	motionCards := classic["properties"].(map[string]any)["MOTION_CARDS"].(map[string]any)
	itemSchema := motionCards["items"].(map[string]any)
	if additionalProps, _ := itemSchema["additionalProperties"].(bool); additionalProps != false {
		t.Error("classic MEMOTION's MOTION_CARDS item schema must keep additionalProperties:false")
	}
	cardProps, ok := itemSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("classic MEMOTION's MOTION_CARDS item schema has no properties map — did #196 accidentally turn it into an anyOf/oneOf?")
	}
	if _, hasType := cardProps["TYPE"]; hasType {
		t.Error("classic MEMOTION's MOTION_CARDS item schema must NOT declare TYPE — #196 must not have touched this variant")
	}
	wantKeys := []string{"RECTO_THEME", "QUESTION_TEXT", "ANSWER_TEXT", "DIFFICULTY"}
	if len(cardProps) != len(wantKeys) {
		t.Errorf("classic MEMOTION card properties = %v, want exactly %v", cardProps, wantKeys)
	}
}

// TestMemotionPlus_Schema_CardItemIsDiscriminatedUnion verifies the new
// MEMOTION_PLUS variant's MOTION_CARDS items are an anyOf of exactly two
// card shapes (SPEEDY, QCM), each locking its own TYPE via const and
// additionalProperties:false — contract §3ter "Portée du mode
// MEMOTION_PLUS": only the two NestableInMotionCard types.
func TestMemotionPlus_Schema_CardItemIsDiscriminatedUnion(t *testing.T) {
	schema := buildQuestionSchema([]string{"GEOGRAPHY"}, []string{"Facile"})
	plus := findAnyOfVariantByTypeConst(t, schema, "MEMOTION_PLUS")

	motionCards, ok := plus["properties"].(map[string]any)["MOTION_CARDS"].(map[string]any)
	if !ok {
		t.Fatal("MEMOTION_PLUS variant has no MOTION_CARDS property")
	}
	itemSchema, ok := motionCards["items"].(map[string]any)
	if !ok {
		t.Fatal("MEMOTION_PLUS MOTION_CARDS.items is not a map")
	}
	cardAnyOf, ok := itemSchema["anyOf"].([]any)
	if !ok {
		t.Fatalf("MEMOTION_PLUS MOTION_CARDS item schema has no anyOf (got keys via %#v) — cards must be a discriminated union, not a flat object", itemSchema)
	}
	if len(cardAnyOf) != 2 {
		t.Fatalf("MEMOTION_PLUS card anyOf has %d branches, want exactly 2 (SPEEDY, QCM)", len(cardAnyOf))
	}

	seenTypes := map[string]map[string]any{}
	for _, v := range cardAnyOf {
		branch, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("card anyOf branch is not a map: %#v", v)
		}
		if additionalProps, _ := branch["additionalProperties"].(bool); additionalProps != false {
			t.Errorf("card branch %#v must keep additionalProperties:false", branch)
		}
		props, ok := branch["properties"].(map[string]any)
		if !ok {
			t.Fatalf("card branch has no properties map: %#v", branch)
		}
		typeSchema, ok := props["TYPE"].(map[string]any)
		if !ok {
			t.Fatalf("card branch has no TYPE property: %#v", props)
		}
		c, _ := typeSchema["const"].(string)
		if c == "" {
			t.Fatalf("card branch TYPE has no const: %#v", typeSchema)
		}
		seenTypes[c] = props
	}

	speedyProps, ok := seenTypes["SPEEDY"]
	if !ok {
		t.Fatal("no SPEEDY branch in MEMOTION_PLUS card anyOf")
	}
	if _, ok := speedyProps["ANSWER_TEXT"]; !ok {
		t.Error("SPEEDY card branch missing ANSWER_TEXT")
	}
	if _, ok := speedyProps["QCM_ANSWERS"]; ok {
		t.Error("SPEEDY card branch must NOT declare QCM_ANSWERS")
	}

	qcmProps, ok := seenTypes["QCM"]
	if !ok {
		t.Fatal("no QCM branch in MEMOTION_PLUS card anyOf")
	}
	if _, ok := qcmProps["QCM_ANSWERS"]; !ok {
		t.Error("QCM card branch missing QCM_ANSWERS")
	}
	if _, ok := qcmProps["QCM_CORRECT"]; !ok {
		t.Error("QCM card branch missing QCM_CORRECT")
	}
	if _, ok := qcmProps["ANSWER_TEXT"]; ok {
		t.Error("QCM card branch must NOT declare ANSWER_TEXT — contract §3.1/§3.2, a card must never carry another type's owned field")
	}
}

// ============================================================================
// B3/B4 — validateGeneratedQuestions + mapGeneratedQuestion
// ============================================================================

func validQCMPlusCard(rectoTheme string) llmMotionCard {
	return llmMotionCard{
		Type:         "QCM",
		RectoTheme:   rectoTheme,
		QuestionText: "Quelle capitale abrite le Colisée ?",
		Difficulty:   1,
		QCMAnswers:   &llmQCMAnswers{Red: "Rome", Green: "Madrid", Yellow: "Lisbonne", Blue: "Athènes"},
		QCMCorrect:   "RED",
	}
}

func validSpeedyPlusCard(rectoTheme string) llmMotionCard {
	return llmMotionCard{
		Type:         "SPEEDY",
		RectoTheme:   rectoTheme,
		QuestionText: "Quelle capitale est traversée par la Seine ?",
		AnswerText:   "Paris",
		Difficulty:   1,
	}
}

func fourMixedPlusCards() []llmMotionCard {
	return []llmMotionCard{
		validSpeedyPlusCard("Capitales d'Europe"),
		validQCMPlusCard("Capitales d'Europe"),
		validSpeedyPlusCard("Capitales d'Europe"),
		validQCMPlusCard("Capitales d'Europe"),
	}
}

// TestMemotionPlus_MotionPlusCardsValid_AcceptsMixedSpeedyAndQCM is the
// direct unit test for motionPlusCardsValid: a mix of valid SPEEDY and QCM
// cards passes.
func TestMemotionPlus_MotionPlusCardsValid_AcceptsMixedSpeedyAndQCM(t *testing.T) {
	if !motionPlusCardsValid(fourMixedPlusCards()) {
		t.Error("motionPlusCardsValid rejected a valid mix of SPEEDY and QCM cards")
	}
}

// TestMemotionPlus_MotionPlusCardsValid_RejectsInvalidCases enumerates the
// per-branch failure modes: empty RECTO_THEME, out-of-range DIFFICULTY, a
// SPEEDY card with empty ANSWER_TEXT, a QCM card with fewer than 4 distinct
// answers, a QCM card with an invalid QCM_CORRECT, and an unknown card TYPE
// (structurally unreachable via the real schema, defended anyway).
func TestMemotionPlus_MotionPlusCardsValid_RejectsInvalidCases(t *testing.T) {
	tests := []struct {
		name string
		card llmMotionCard
	}{
		{"empty RECTO_THEME", func() llmMotionCard { c := validSpeedyPlusCard(""); return c }()},
		{"DIFFICULTY out of range", func() llmMotionCard { c := validSpeedyPlusCard("T"); c.Difficulty = 0; return c }()},
		{"SPEEDY with empty ANSWER_TEXT", func() llmMotionCard { c := validSpeedyPlusCard("T"); c.AnswerText = ""; return c }()},
		{"QCM with duplicate answers", func() llmMotionCard {
			c := validQCMPlusCard("T")
			c.QCMAnswers = &llmQCMAnswers{Red: "Rome", Green: "Rome", Yellow: "Lisbonne", Blue: "Athènes"}
			return c
		}()},
		{"QCM with invalid QCM_CORRECT", func() llmMotionCard { c := validQCMPlusCard("T"); c.QCMCorrect = "PURPLE"; return c }()},
		{"unknown card TYPE", func() llmMotionCard { c := validSpeedyPlusCard("T"); c.Type = "MEMORY"; return c }()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if motionPlusCardsValid([]llmMotionCard{tt.card}) {
				t.Errorf("motionPlusCardsValid accepted an invalid card (%s): %+v", tt.name, tt.card)
			}
		})
	}
}

// TestMemotionPlus_ValidateGeneratedQuestions_EndToEnd exercises the full
// per-question validation switch: a well-formed MEMOTION_PLUS question is
// accepted, one with too few cards is rejected (same 4-card floor as
// classic MEMOTION), one with an invalid QCM card is rejected.
func TestMemotionPlus_ValidateGeneratedQuestions_EndToEnd(t *testing.T) {
	base := llmRawQuestion{
		Category:   "GEOGRAPHY",
		Question:   "MEMOTION_PLUS generated test",
		Time:       30,
		Difficulty: "Facile",
	}

	valid := base
	valid.Type = "MEMOTION_PLUS"
	valid.MotionCards = fourMixedPlusCards()

	tooFewCards := base
	tooFewCards.Type = "MEMOTION_PLUS"
	tooFewCards.MotionCards = fourMixedPlusCards()[:2]

	badQCMCard := base
	badQCMCard.Type = "MEMOTION_PLUS"
	badCard := validQCMPlusCard("T")
	badCard.QCMCorrect = "PURPLE"
	badQCMCard.MotionCards = append(fourMixedPlusCards(), badCard)

	accepted, reasons := validateGeneratedQuestions([]llmRawQuestion{valid, tooFewCards, badQCMCard}, []string{"GEOGRAPHY"}, []string{"Facile"}, 200)
	if len(accepted) != 1 {
		t.Fatalf("accepted = %d questions, want exactly 1 (only `valid`) — reasons: %v", len(accepted), reasons)
	}
	if accepted[0].Type != "MEMOTION_PLUS" {
		t.Errorf("accepted[0].Type = %q, want %q (validateGeneratedQuestions must not itself normalize — that's mapGeneratedQuestion's job)", accepted[0].Type, "MEMOTION_PLUS")
	}
	if len(reasons) != 2 {
		t.Errorf("reasons = %v, want 2 entries (tooFewCards, badQCMCard)", reasons)
	}
}

// TestMemotionPlus_MapGeneratedQuestion_NormalizesTypeToMemotion is the
// 🔴 CRITICAL test named explicitly by team-lead — contract §3ter's central
// invariant: "MEMOTION_PLUS ne doit JAMAIS apparaître dans un
// question.json généré". Verified on the actual map that gets
// json.MarshalIndent'd and written to disk, not just on an intermediate
// value.
func TestMemotionPlus_MapGeneratedQuestion_NormalizesTypeToMemotion(t *testing.T) {
	// QUESTION text deliberately does NOT contain the substring
	// "MEMOTION_PLUS" — the raw-JSON scan below asserts that string
	// appears NOWHERE in the marshaled output, so the input mustn't
	// introduce it incidentally through an unrelated free-text field.
	q := llmRawQuestion{
		Type:        "MEMOTION_PLUS",
		Category:    "GEOGRAPHY",
		Question:    "Question générée en mode carte mixte",
		Time:        30,
		Difficulty:  "Facile",
		MotionCards: fourMixedPlusCards(),
	}

	mapped := mapGeneratedQuestion(q, "1", 1)

	gotType, _ := mapped["TYPE"].(string)
	if gotType != "MEMOTION" {
		t.Fatalf(`mapped["TYPE"] = %q, want "MEMOTION" — MEMOTION_PLUS must never survive into the persisted question`, gotType)
	}

	// Byte-level guarantee: marshal the whole map (exactly what
	// ai_job.go does before os.WriteFile) and grep the raw JSON for the
	// forbidden string, so this test would catch the pseudo-type leaking
	// from ANY field, not just TYPE, if a future change introduced one.
	raw, err := json.Marshal(mapped)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if bytesContainsString(raw, "MEMOTION_PLUS") {
		t.Errorf("the marshaled question.json contains the string \"MEMOTION_PLUS\" — it must NEVER appear anywhere on disk: %s", raw)
	}
}

func bytesContainsString(b []byte, s string) bool {
	return len(b) > 0 && jsonIndexOf(string(b), s) >= 0
}

// jsonIndexOf is a trivial substring search — avoids importing "strings"
// into this file purely for one call when encoding/json is already here;
// kept local and obviously named rather than reused from elsewhere, this
// file has no other need for string search.
func jsonIndexOf(haystack, needle string) int {
	n, m := len(haystack), len(needle)
	if m == 0 || m > n {
		return -1
	}
	for i := 0; i+m <= n; i++ {
		if haystack[i:i+m] == needle {
			return i
		}
	}
	return -1
}

// TestMemotionPlus_MapGeneratedQuestion_CardShapes verifies each card's
// mapped shape depends on its OWN c.Type, matching contract §3.1/§3.2
// exactly — a SPEEDY card gets ANSWER_TEXT and no TYPE key at all (byte-
// identical to a classic MEMOTION card, contract's non-regression
// requirement), a QCM card gets TYPE="QCM" + QCM_ANSWERS + QCM_CORRECT and
// NEVER ANSWER_TEXT.
func TestMemotionPlus_MapGeneratedQuestion_CardShapes(t *testing.T) {
	q := llmRawQuestion{
		Type:       "MEMOTION_PLUS",
		Category:   "GEOGRAPHY",
		Question:   "test",
		Time:       30,
		Difficulty: "Facile",
		MotionCards: []llmMotionCard{
			validSpeedyPlusCard("Thème"),
			validQCMPlusCard("Thème"),
		},
	}

	mapped := mapGeneratedQuestion(q, "1", 1)
	cards, ok := mapped["MOTION_CARDS"].([]map[string]interface{})
	if !ok {
		t.Fatalf("mapped[\"MOTION_CARDS\"] is not []map[string]interface{}, got %T", mapped["MOTION_CARDS"])
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(cards))
	}

	speedyCard := cards[0]
	if _, hasType := speedyCard["TYPE"]; hasType {
		t.Errorf("SPEEDY card carries a TYPE key %v — must stay untyped, byte-identical to a classic MEMOTION card", speedyCard["TYPE"])
	}
	if speedyCard["ANSWER_TEXT"] != "Paris" {
		t.Errorf("SPEEDY card ANSWER_TEXT = %v, want \"Paris\"", speedyCard["ANSWER_TEXT"])
	}
	if _, hasQCM := speedyCard["QCM_ANSWERS"]; hasQCM {
		t.Error("SPEEDY card must not carry QCM_ANSWERS")
	}

	qcmCard := cards[1]
	if qcmCard["TYPE"] != "QCM" {
		t.Errorf("QCM card TYPE = %v, want \"QCM\"", qcmCard["TYPE"])
	}
	if _, hasAnswerText := qcmCard["ANSWER_TEXT"]; hasAnswerText {
		t.Error("QCM card must NOT carry ANSWER_TEXT (contract §3.1/§3.2 — CARD_TYPE_CONTENT_MISMATCH territory if it ever reached the editor's validation)")
	}
	qcmAnswers, ok := qcmCard["QCM_ANSWERS"].(map[string]string)
	if !ok {
		t.Fatalf("QCM card QCM_ANSWERS is not map[string]string, got %T", qcmCard["QCM_ANSWERS"])
	}
	if qcmAnswers["RED"] != "Rome" {
		t.Errorf("QCM card QCM_ANSWERS[RED] = %q, want \"Rome\"", qcmAnswers["RED"])
	}
	if qcmCard["QCM_CORRECT"] != "RED" {
		t.Errorf("QCM card QCM_CORRECT = %v, want \"RED\"", qcmCard["QCM_CORRECT"])
	}
}

// ============================================================================
// Full round-trip through the real game types — proves a MEMOTION_PLUS-
// generated question is playable exactly like a hand-authored one: the QCM
// card resolves to EffectiveType()==QCM and is nestable, the SPEEDY card to
// EffectiveType()==SPEEDY, mirroring TestAIGeneratedMotionCards_MappedWithoutType's
// (#184) round-trip for the classic case.
// ============================================================================

func TestMemotionPlus_RoundTrip_CardsResolveToRealTypes(t *testing.T) {
	q := llmRawQuestion{
		Type:       "MEMOTION_PLUS",
		Category:   "GEOGRAPHY",
		Question:   "test",
		Time:       30,
		Difficulty: "Facile",
		MotionCards: []llmMotionCard{
			validSpeedyPlusCard("Thème"),
			validQCMPlusCard("Thème"),
		},
	}
	mapped := mapGeneratedQuestion(q, "1", 1)

	raw, err := json.Marshal(mapped)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var question game.Question
	if err := json.Unmarshal(raw, &question); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if question.Type != game.QuestionTypeMemotion {
		t.Fatalf("question.Type = %q, want MEMOTION", question.Type)
	}
	if len(question.MotionCards) != 2 {
		t.Fatalf("expected 2 motion cards, got %d", len(question.MotionCards))
	}

	speedy := question.MotionCards[0]
	if speedy.Type != "" {
		t.Errorf("SPEEDY card: Type = %q, want \"\" (absent)", speedy.Type)
	}
	if speedy.EffectiveType() != game.QuestionTypeSpeedy {
		t.Errorf("SPEEDY card: EffectiveType() = %q, want SPEEDY", speedy.EffectiveType())
	}
	if speedy.AnswerText != "Paris" {
		t.Errorf("SPEEDY card: AnswerText = %q, want \"Paris\"", speedy.AnswerText)
	}

	qcm := question.MotionCards[1]
	if qcm.Type != game.QuestionTypeQCM {
		t.Errorf("QCM card: Type = %q, want QCM", qcm.Type)
	}
	if qcm.EffectiveType() != game.QuestionTypeQCM {
		t.Errorf("QCM card: EffectiveType() = %q, want QCM", qcm.EffectiveType())
	}
	if !game.IsNestableInMotionCard(qcm.EffectiveType()) {
		t.Errorf("QCM card: EffectiveType() %q should be nestable", qcm.EffectiveType())
	}
	if qcm.QCMCorrect != "RED" {
		t.Errorf("QCM card: QCMCorrect = %q, want RED", qcm.QCMCorrect)
	}
	if qcm.QCMAnswers == nil || qcm.QCMAnswers.Red != "Rome" {
		t.Errorf("QCM card: QCMAnswers = %+v, want RED=\"Rome\"", qcm.QCMAnswers)
	}
	if qcm.AnswerText != "" {
		t.Errorf("QCM card: AnswerText = %q, want empty — a QCM card must never carry SPEEDY's owned field", qcm.AnswerText)
	}

	// The card-type-content-mismatch guard a manual editor save would run
	// through (contract §3ter: "aucun élargissement... nécessaire" — this
	// is the proof, not just an assertion of intent).
	qcmCardMap := map[string]interface{}{"ID": qcm.ID, "QCM_ANSWERS": qcm.QCMAnswers, "QCM_CORRECT": qcm.QCMCorrect}
	if err := game.ValidateCardTypeContent(qcm.Type, qcmCardMap); err != nil {
		t.Errorf("QCM card fails ValidateCardTypeContent: %v", err)
	}
}

// ============================================================================
// B5 — prompt guidance
// ============================================================================

func baseMemotionPlusPromptRequest() generateQuestionsRequest {
	return generateQuestionsRequest{
		Theme:        "Test",
		Populations:  []string{"Adulte (18-64 ans)"},
		Language:     "Français",
		Difficulties: []string{"Facile"},
		Categories:   []string{"GEOGRAPHY"},
		Volume:       generateVolume{Mode: "count", Value: 10},
		Distribution: map[string]int{"MEMOTION_PLUS": 100},
	}
}

// TestMemotionPlus_Prompt_IncludesGuidanceWhenRequested verifies B5: the
// dedicated MEMOTION_PLUS prompt section appears when
// distribution["MEMOTION_PLUS"] > 0, mentions the per-card TYPE choice, and
// the "Répartition cible" line names MEMOTION_PLUS explicitly (via the
// generableQuestionTypes loop, unchanged mechanism).
func TestMemotionPlus_Prompt_IncludesGuidanceWhenRequested(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	req := baseMemotionPlusPromptRequest()

	prompt := server.buildGenerationPrompt(req, 10, nil)

	if !bytesContainsString([]byte(prompt), `TYPE="MEMOTION_PLUS"`) {
		t.Error("prompt does not mention the MEMOTION_PLUS format section")
	}
	if !bytesContainsString([]byte(prompt), "TYPE") || !bytesContainsString([]byte(prompt), "SPEEDY") || !bytesContainsString([]byte(prompt), "QCM") {
		t.Error("prompt's MEMOTION_PLUS section should describe both card TYPE choices (SPEEDY/QCM)")
	}
	if !bytesContainsString([]byte(prompt), "MEMOTION_PLUS=100%") {
		t.Error(`prompt's "Répartition cible" line does not name MEMOTION_PLUS`)
	}
}

// TestMemotionPlus_Prompt_OmittedWhenNotRequested is the non-regression
// companion: a job that never requests MEMOTION_PLUS doesn't pay for (or
// confuse the model with) guidance it doesn't need — same pattern as the
// existing MEMOTION/ARDOISE conditional blocks.
func TestMemotionPlus_Prompt_OmittedWhenNotRequested(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	req := baseMemotionPlusPromptRequest()
	req.Distribution = map[string]int{"SPEEDY": 100}

	prompt := server.buildGenerationPrompt(req, 10, nil)

	if bytesContainsString([]byte(prompt), "MEMOTION_PLUS") {
		t.Error("prompt mentions MEMOTION_PLUS even though it wasn't requested")
	}
}
