// Tests for #184 B-B2 — SelectMotionCard's own TYPE-nestability guard,
// belt-and-suspenders alongside http.go's upload-time validation (a card's
// TYPE could reach the engine as non-nestable via a path that bypassed the
// HTTP check — e.g. a hand-edited question.json, or a future registry
// change after the file was saved).
//
// Run: go test ./internal/game/... -run TestSelectMotionCard_TypeLock -v

package game

import "testing"

// TestSelectMotionCard_RefusesNonNestableType verifies SelectMotionCard
// rejects a card whose declared TYPE cannot be nested in a MEMOTION card —
// contract §1 (MEMOTION re-nesting) and §7 (ARDOISE/MEMORY not yet
// nestable, v7.1.0) — without ever changing MotionSubPhase/MotionCardStates.
func TestSelectMotionCard_RefusesNonNestableType(t *testing.T) {
	tests := []struct {
		name     string
		cardType QuestionType
	}{
		{"re-nesting MEMOTION", QuestionTypeMemotion},
		{"not-yet-nestable ARDOISE (#186)", QuestionTypeArdoise},
		{"not-yet-nestable MEMORY (#187)", QuestionTypeMemory},
		{"unknown/bogus type", QuestionType("BOGUS")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine()
			cards := defaultMotionCards()
			cards[0].Type = tt.cardType
			q := makeMotionQuestion("mq1", cards, "SOLO")
			startMEMOTION(t, e, "mq1", q)
			defer e.Stop()

			err := e.SelectMotionCard("mc-1")
			if err == nil {
				t.Fatalf("SelectMotionCard should refuse a %s card", tt.cardType)
			}
			motionErr, ok := err.(*MotionError)
			if !ok || motionErr.Reason != "CARD_TYPE_NOT_NESTABLE" {
				t.Errorf("expected MotionError{Reason: CARD_TYPE_NOT_NESTABLE}, got %v", err)
			}

			// State must be untouched — a refused selection is a no-op.
			state := e.GetState()
			if state.MotionSubPhase != MotionSubPhaseGrid {
				t.Errorf("MotionSubPhase should remain GRID after a refused select, got %q", state.MotionSubPhase)
			}
			if state.MotionCardStates["mc-1"] != MotionCardStateUnplayed {
				t.Errorf("card mc-1 state should remain UNPLAYED after a refused select, got %q", state.MotionCardStates["mc-1"])
			}
			if state.MotionSelected != "" {
				t.Errorf("MotionSelected should remain empty after a refused select, got %q", state.MotionSelected)
			}
		})
	}
}

// TestSelectMotionCard_AllowsQCMType verifies the positive counterpart: a
// QCM-typed card (#184/#185, nestable as of v7.0.0) selects exactly like a
// SPEEDY one.
func TestSelectMotionCard_AllowsQCMType(t *testing.T) {
	e := NewEngine()
	cards := defaultMotionCards()
	cards[0].Type = QuestionTypeQCM
	q := makeMotionQuestion("mq1", cards, "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	if err := e.SelectMotionCard("mc-1"); err != nil {
		t.Fatalf("SelectMotionCard should accept a QCM card, got error: %v", err)
	}

	state := e.GetState()
	if state.MotionSubPhase != MotionSubPhaseSelected {
		t.Errorf("MotionSubPhase should be SELECTED, got %q", state.MotionSubPhase)
	}
	if state.MotionSelected != "mc-1" {
		t.Errorf("MotionSelected should be mc-1, got %q", state.MotionSelected)
	}
}

// TestSelectMotionCard_AllowsAbsentType verifies the retro-compat case: a
// card with no TYPE at all (every MEMOTION question saved before #184)
// still selects — EffectiveType() defaults it to SPEEDY, which is nestable.
func TestSelectMotionCard_AllowsAbsentType(t *testing.T) {
	e := NewEngine()
	cards := defaultMotionCards() // Type left unset on all of them
	q := makeMotionQuestion("mq1", cards, "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	if err := e.SelectMotionCard("mc-1"); err != nil {
		t.Fatalf("SelectMotionCard should accept a TYPE-less (SPEEDY) card, got error: %v", err)
	}
}
