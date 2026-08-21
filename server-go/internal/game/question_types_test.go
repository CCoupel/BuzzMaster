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
		{QuestionTypeArdoise, false},  // #186, v7.1.0
		{QuestionTypeMemory, false},   // #187, v7.1.0
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
