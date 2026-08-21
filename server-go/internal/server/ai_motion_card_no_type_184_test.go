// Tests for #184 B-B8 — AI generation non-regression: the LLM-facing
// MOTION_CARDS schema must stay exactly what it was before #184 (no TYPE
// property), so every AI-generated MEMOTION card is implicitly SPEEDY —
// contract §11 ("Une carte TYPE=SPEEDY explicite est indistinguable d'une
// carte sans TYPE") applies just as much to generated cards as to
// hand-edited ones.
// Run: go test ./internal/server/... -run TestAIGeneratedMotionCards -v

package server

import (
	"buzzcontrol/internal/game"
	"encoding/json"
	"testing"
)

// TestAIGeneratedMotionCards_SchemaHasNoTypeProperty locks in the JSON
// schema sent to the LLM: the MOTION_CARDS item schema has
// additionalProperties:false and its properties are exactly
// RECTO_THEME/QUESTION_TEXT/ANSWER_TEXT/DIFFICULTY — no TYPE. A future
// change that adds TYPE here would let the LLM emit typed cards without any
// of #184's validation (registry check, CARD_TYPE_CONTENT_MISMATCH)
// designed for that path, so this must fail loudly if it ever happens.
func TestAIGeneratedMotionCards_SchemaHasNoTypeProperty(t *testing.T) {
	schema := buildQuestionSchema([]string{"GEOGRAPHY"}, []string{"Facile"})

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema[\"properties\"] is not a map")
	}
	questions, ok := props["questions"].(map[string]any)
	if !ok {
		t.Fatal("schema.properties.questions is not a map")
	}
	items, ok := questions["items"].(map[string]any)
	if !ok {
		t.Fatal("schema.properties.questions.items is not a map")
	}
	anyOf, ok := items["anyOf"].([]any)
	if !ok {
		t.Fatal("schema...items.anyOf is not a slice")
	}

	var motionVariant map[string]any
	for _, v := range anyOf {
		variant, ok := v.(map[string]any)
		if !ok {
			continue
		}
		variantProps, ok := variant["properties"].(map[string]any)
		if !ok {
			continue
		}
		if _, hasMotionCards := variantProps["MOTION_CARDS"]; hasMotionCards {
			motionVariant = variant
			break
		}
	}
	if motionVariant == nil {
		t.Fatal("no anyOf variant declares MOTION_CARDS — MEMOTION schema variant not found")
	}

	motionCardsField := motionVariant["properties"].(map[string]any)["MOTION_CARDS"].(map[string]any)
	itemSchema, ok := motionCardsField["items"].(map[string]any)
	if !ok {
		t.Fatal("MOTION_CARDS.items is not a map")
	}

	if additionalProps, _ := itemSchema["additionalProperties"].(bool); additionalProps != false {
		t.Error("MOTION_CARDS item schema must keep additionalProperties:false — otherwise the LLM could emit an unvalidated TYPE (or any other field)")
	}

	cardProps, ok := itemSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("MOTION_CARDS item schema has no properties map")
	}
	if _, hasType := cardProps["TYPE"]; hasType {
		t.Error("MOTION_CARDS item schema must NOT declare a TYPE property (#184, B-B8) — AI-generated cards stay implicitly SPEEDY")
	}

	wantKeys := map[string]bool{"RECTO_THEME": true, "QUESTION_TEXT": true, "ANSWER_TEXT": true, "DIFFICULTY": true}
	if len(cardProps) != len(wantKeys) {
		t.Errorf("MOTION_CARDS item schema properties = %v, want exactly %v", cardProps, wantKeys)
	}
	for k := range wantKeys {
		if _, ok := cardProps[k]; !ok {
			t.Errorf("MOTION_CARDS item schema missing expected property %q", k)
		}
	}
}

// TestAIGeneratedMotionCards_MappedWithoutType exercises the actual
// conversion path (mapGeneratedQuestion) with a synthetic LLM response and
// verifies no generated card carries a TYPE key, and that the resulting
// question.json shape unmarshals into game.MotionCard with
// EffectiveType()==SPEEDY, exactly like every hand-authored MEMOTION card
// saved before #184.
func TestAIGeneratedMotionCards_MappedWithoutType(t *testing.T) {
	q := llmRawQuestion{
		Type:       "MEMOTION",
		Category:   "GEOGRAPHY",
		Question:   "MEMOTION generated test",
		Time:       30,
		Difficulty: "Facile",
		MotionCards: []llmMotionCard{
			{RectoTheme: "Capitales", QuestionText: "Quelle est la capitale ?", AnswerText: "Paris", Difficulty: 1},
			{RectoTheme: "Fleuves", QuestionText: "Quel est le plus long ?", AnswerText: "Nil", Difficulty: 2},
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
	for i, c := range cards {
		if _, hasType := c["TYPE"]; hasType {
			t.Errorf("card %d carries a TYPE key %v — AI-generated cards must stay untyped (implicitly SPEEDY)", i, c["TYPE"])
		}
	}

	// Full round-trip: marshal the mapped question like the real writer
	// does (map[string]interface{} → JSON), then unmarshal into the actual
	// game.Question/MotionCard structs, as loadQuestion does when reading
	// it back for play.
	raw, err := json.Marshal(mapped)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var question game.Question
	if err := json.Unmarshal(raw, &question); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if len(question.MotionCards) != 2 {
		t.Fatalf("expected 2 motion cards after round-trip, got %d", len(question.MotionCards))
	}
	for i, c := range question.MotionCards {
		if c.Type != "" {
			t.Errorf("card %d: Type = %q, want \"\" (absent) for an AI-generated card", i, c.Type)
		}
		if c.EffectiveType() != game.QuestionTypeSpeedy {
			t.Errorf("card %d: EffectiveType() = %q, want SPEEDY", i, c.EffectiveType())
		}
		if !game.IsNestableInMotionCard(c.EffectiveType()) {
			t.Errorf("card %d: EffectiveType() %q should be nestable (it's SPEEDY)", i, c.EffectiveType())
		}
	}
}
