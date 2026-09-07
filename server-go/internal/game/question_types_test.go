package game

import "testing"

// TestQuestionTypeRegistry_Exhaustive is the Go half of the §10.1
// exhaustiveness test (B-B8's other half lives in the JS test suite,
// questionTypeMeta.test.js): every value AllQuestionTypes() (#183, A-B2)
// returns must have a registry entry here, and the registry must not carry
// an entry for anything AllQuestionTypes() doesn't know about. A type added
// to one but not the other breaks this test instead of silently falling
// back to SPEEDY behavior — contract §10.1.
func TestQuestionTypeRegistry_Exhaustive(t *testing.T) {
	all := AllQuestionTypes()
	if len(all) == 0 {
		t.Fatal("AllQuestionTypes() returned nothing — registry exhaustiveness can't be checked")
	}

	for _, qt := range all {
		if _, ok := questionTypeRegistry[qt]; !ok {
			t.Errorf("QuestionType %q is in AllQuestionTypes() but has no questionTypeRegistry entry", qt)
		}
	}
	for qt := range questionTypeRegistry {
		found := false
		for _, want := range all {
			if want == qt {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("questionTypeRegistry has an entry for %q, which AllQuestionTypes() does not list", qt)
		}
	}
}

// TestQuestionTypeRegistry_NestableTypesDeclareMediaSlots is B-B8's
// stronger half of the §10.1 exhaustiveness test: any type marked
// NestableInMotionCard must also declare at least one MediaSlots entry —
// a nestable type with zero upload slots would be a silent dead end (the
// card could never render any media), the same class of "falls back
// silently instead of breaking loudly" bug §10.1 exists to catch. MEMOTION
// is the one deliberate, permanent exception (never nestable, so its nil
// MediaSlots is correct, not an oversight).
//
// ⚠️ [CHANGED] #217 (milestone v9.0.0, réouverture assumée, arbitrage
// utilisateur "MediaSlots") — RAFALE becomes a SECOND deliberate exception:
// nestable (NestableInMotionCard: true) yet text-only by design, exactly
// like a classic RAFALE round (contracts/rafale.md §2.4/D3) — no media slot
// was ever added for it, on purpose, not an oversight left behind by this
// invariant. Carved out explicitly here (never silently, per this file's
// own #10.1 discipline) rather than weakening the invariant itself for
// every other type.
func TestQuestionTypeRegistry_NestableTypesDeclareMediaSlots(t *testing.T) {
	// #217 — RAFALE is the second permanent, deliberate exception (see
	// MEMOTION's own carve-out in this test's doc comment above).
	mediaSlotExemptNestableTypes := map[QuestionType]bool{
		QuestionTypeRafale: true,
	}

	for qt, desc := range questionTypeRegistry {
		if !desc.NestableInMotionCard {
			continue
		}
		if mediaSlotExemptNestableTypes[qt] {
			continue
		}
		if len(desc.MediaSlots) == 0 {
			t.Errorf("type %q is NestableInMotionCard but declares no MediaSlots — a nestable type must have at least one upload slot (contract §7/§8), unless explicitly exempted", qt)
		}
	}
}

// TestQuestionTypeRegistry_DefaultPointsRuleIsKnownOrEmpty is #187 cycle 5's
// exhaustiveness extension: every registry entry's DefaultPointsRule
// must be EITHER "" (⇒ inherits PointsRuleModeStars, motionCardPointsForOutcome's
// own fallback) OR one of the four known PointsRuleMode constants — a typo
// or a value from a future, not-yet-wired mode would otherwise fall through
// motionCardPointsForOutcome's own switch to its `default:` (STARS) branch
// silently, exactly the class of "falls back instead of breaking loudly"
// bug §10.1 exists to catch for every other registry field.
func TestQuestionTypeRegistry_DefaultPointsRuleIsKnownOrEmpty(t *testing.T) {
	known := map[PointsRuleMode]bool{
		"":                         true, // absent ⇒ STARS, not itself a mode
		PointsRuleModeStars:        true,
		PointsRuleModeFixed:        true,
		PointsRuleModePerUnit:      true,
		PointsRuleModeStarsProrata: true,
	}
	for qt, desc := range questionTypeRegistry {
		if !known[desc.DefaultPointsRule] {
			t.Errorf("type %q declares DefaultPointsRule=%q, not a known PointsRuleMode (or empty)", qt, desc.DefaultPointsRule)
		}
	}
}

// TestQuestionTypeRegistry_MemoryDefaultPointsRuleIsStarsProrata locks in
// the one non-empty DefaultPointsRule in the registry as of #187 — contract
// §6.3's "on compte les points en fonction du nombre de paires trouvées"
// decision, resolved here rather than written onto any card (see
// TypeDescriptor.DefaultPointsRule's doc comment).
func TestQuestionTypeRegistry_MemoryDefaultPointsRuleIsStarsProrata(t *testing.T) {
	d, ok := TypeDescriptorFor(QuestionTypeMemory)
	if !ok {
		t.Fatal("QuestionTypeMemory has no registry entry")
	}
	if d.DefaultPointsRule != PointsRuleModeStarsProrata {
		t.Errorf("MEMORY's DefaultPointsRule = %q, want %q", d.DefaultPointsRule, PointsRuleModeStarsProrata)
	}
}

// TestQuestionTypeRegistry_OtherTypesHaveNoDefaultPointsRule is the
// explicit non-regression companion: every OTHER type must still fall
// through to STARS (empty DefaultPointsRule) — #187 cycle 5 must not have
// silently changed the barème for SPEEDY/QCM/ARDOISE/MEMOTION.
//
// #217 (v9.0.0): RAFALE joins MEMORY with a non-empty default
// (STARS_PRORATA, 217-Q4 — same barème, same reasoning: a nested RAFALE
// mini-round is a progression type, contracts/rafale.md §14.4) — excluded
// here alongside MEMORY rather than silently widening what this test
// tolerates.
func TestQuestionTypeRegistry_OtherTypesHaveNoDefaultPointsRule(t *testing.T) {
	exempt := map[QuestionType]bool{
		QuestionTypeMemory: true,
		QuestionTypeRafale: true,
	}
	for qt, desc := range questionTypeRegistry {
		if exempt[qt] {
			continue
		}
		if desc.DefaultPointsRule != "" {
			t.Errorf("type %q declares DefaultPointsRule=%q, want empty (STARS, unchanged) — only MEMORY and RAFALE (#217) have a non-empty default", qt, desc.DefaultPointsRule)
		}
	}
}

// TestQuestionTypeRegistry_TypeFieldMatchesMapKey guards against a
// copy-paste registry entry — questionTypeRegistry[qt].Type must equal qt,
// the map key it's stored under, for every entry.
func TestQuestionTypeRegistry_TypeFieldMatchesMapKey(t *testing.T) {
	for qt, desc := range questionTypeRegistry {
		if desc.Type != qt {
			t.Errorf("questionTypeRegistry[%q].Type = %q, want %q (map key and Type field must match)", qt, desc.Type, qt)
		}
	}
}

func TestTypeDescriptorFor(t *testing.T) {
	if d, ok := TypeDescriptorFor(QuestionTypeQCM); !ok || d.Type != QuestionTypeQCM {
		t.Errorf("TypeDescriptorFor(QCM) = %+v, %v — want a QCM descriptor, true", d, ok)
	}
	if _, ok := TypeDescriptorFor(QuestionType("BOGUS")); ok {
		t.Error("TypeDescriptorFor(BOGUS) should report ok=false for an unregistered type")
	}
}

func TestIsNestableInMotionCard(t *testing.T) {
	tests := []struct {
		t    QuestionType
		want bool
	}{
		{QuestionTypeSpeedy, true},
		{QuestionTypeQCM, true},
		{QuestionTypeArdoise, false},  // #186 closed "not planned", 2026-08-24
		{QuestionTypeMemory, true},    // #187, v7.1.0
		{QuestionTypeMemotion, false}, // never — depth capped at 1
		{QuestionType("BOGUS"), false},
	}
	for _, tt := range tests {
		if got := IsNestableInMotionCard(tt.t); got != tt.want {
			t.Errorf("IsNestableInMotionCard(%q) = %v, want %v", tt.t, got, tt.want)
		}
	}
}

func TestValidateCardTypeContent(t *testing.T) {
	tests := []struct {
		name     string
		cardType QuestionType
		card     map[string]interface{}
		wantErr  bool
	}{
		{
			name:     "SPEEDY card with only common fields — ok",
			cardType: QuestionTypeSpeedy,
			card:     map[string]interface{}{"ID": "mc-1", "RECTO_THEME": "x", "DIFFICULTY": float64(1)},
			wantErr:  false,
		},
		{
			name:     "empty TYPE (defaults SPEEDY) with ANSWER_TEXT — ok, that's SPEEDY's own field",
			cardType: "",
			card:     map[string]interface{}{"ID": "mc-1", "ANSWER_TEXT": "Paris"},
			wantErr:  false,
		},
		{
			name:     "empty TYPE (defaults SPEEDY) with QCM_ANSWERS — mismatch",
			cardType: "",
			card:     map[string]interface{}{"ID": "mc-1", "QCM_ANSWERS": map[string]interface{}{"RED": "a"}},
			wantErr:  true,
		},
		{
			name:     "SPEEDY card carrying QCM_ANSWERS — mismatch",
			cardType: QuestionTypeSpeedy,
			card: map[string]interface{}{
				"ID":          "mc-1",
				"QCM_ANSWERS": map[string]interface{}{"RED": "a", "GREEN": "b", "YELLOW": "c", "BLUE": "d"},
			},
			wantErr: true,
		},
		{
			name:     "QCM card with QCM_ANSWERS + QCM_CORRECT — ok, its own fields",
			cardType: QuestionTypeQCM,
			card: map[string]interface{}{
				"ID":          "mc-1",
				"QCM_ANSWERS": map[string]interface{}{"RED": "a", "GREEN": "b", "YELLOW": "c", "BLUE": "d"},
				"QCM_CORRECT": "RED",
			},
			wantErr: false,
		},
		{
			name:     "QCM card carrying ANSWER_TEXT (SPEEDY's field) — mismatch",
			cardType: QuestionTypeQCM,
			card:     map[string]interface{}{"ID": "mc-1", "ANSWER_TEXT": "Paris"},
			wantErr:  true,
		},
		{
			name:     "QCM_HINTS_ENABLED explicitly false on a SPEEDY card — ok, zero value is not content",
			cardType: QuestionTypeSpeedy,
			card:     map[string]interface{}{"ID": "mc-1", "QCM_HINTS_ENABLED": false},
			wantErr:  false,
		},
		{
			name:     "QCM_HINTS_ENABLED true on a SPEEDY card — mismatch, real content",
			cardType: QuestionTypeSpeedy,
			card:     map[string]interface{}{"ID": "mc-1", "QCM_HINTS_ENABLED": true},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCardTypeContent(tt.cardType, tt.card)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCardTypeContent(%q, %v) error = %v, wantErr %v", tt.cardType, tt.card, err, tt.wantErr)
			}
		})
	}
}

func TestMotionCard_EffectiveType(t *testing.T) {
	c := MotionCard{}
	if got := c.EffectiveType(); got != QuestionTypeSpeedy {
		t.Errorf("empty MotionCard.EffectiveType() = %q, want SPEEDY", got)
	}
	c.Type = QuestionTypeQCM
	if got := c.EffectiveType(); got != QuestionTypeQCM {
		t.Errorf("MotionCard.EffectiveType() = %q, want QCM", got)
	}
}
