// Tests for #279 LOT-6 — a MEMOTION QCM card has NO hints (contract
// question-types.md §7.1): the card tick never invalidates an answer nor
// fires OnQCMHint, even for a stored card that still carries hint settings;
// StripCardHints removes the five hint keys from a card payload.
// Run: go test ./internal/game/... -run "NoHints|StripCardHints" -v

package game

import (
	"reflect"
	"testing"
)

// A stored card (question saved before LOT-6) with hints enabled and
// thresholds: over 20 ticks the engine never fires a hint and never writes
// QCM_INVALIDATED in MEMOTION_ACTIVE.STATE.
func TestProcessMotionCardTick_QCMCard_NeverHints_279(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", []MotionCard{qcmMotionCard("mc-1")}, "SOLO") // QCMHintsEnabled: true, 0.5 / 0.25
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	hintCalls := 0
	e.OnQCMHint = func(string, int) { hintCalls++ }

	if err := e.SelectMotionCard("mc-1"); err != nil {
		t.Fatalf("SelectMotionCard: %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard: %v", err)
	}
	e.state.CurrentTime = 21
	e.state.Delay = 21

	for i := 0; i < 20; i++ {
		result := e.processMotionCardTick()
		if result.guardFailed {
			t.Fatalf("guard failed at tick %d", i)
		}
		st := e.GetState()
		if _, has := st.MotionActive.State["QCM_INVALIDATED"]; has {
			t.Fatalf("tick %d: MEMOTION_ACTIVE.STATE carries QCM_INVALIDATED: %v", i, st.MotionActive.State)
		}
		if len(st.QcmInvalidated) != 0 {
			t.Fatalf("tick %d: question-scoped QcmInvalidated = %v, want empty", i, st.QcmInvalidated)
		}
	}
	if hintCalls != 0 {
		t.Errorf("OnQCMHint called %d times for a card, want 0", hintCalls)
	}
	if got := e.GetState().CurrentTime; got != 1 {
		t.Errorf("CurrentTime = %d, want 1 (the tick still decrements the card timer)", got)
	}
	if st := e.GetState().MotionActive.State; len(st) != 0 {
		t.Errorf("MEMOTION_ACTIVE.STATE of a QCM card = %v, want {} (contract game-state)", st)
	}
}

func TestStripCardHints_279(t *testing.T) {
	hintKeys := []string{"QCM_HINTS_ENABLED", "QCM_HINT_THRESHOLD_1", "QCM_HINT_THRESHOLD_2", "QCM_PENALTY_1", "QCM_PENALTY_2"}

	t.Run("five keys removed, any value, other keys intact", func(t *testing.T) {
		card := map[string]interface{}{
			"ID": "mc-1", "TYPE": "QCM", "DIFFICULTY": 2.0,
			"QCM_ANSWERS":          map[string]interface{}{"RED": "a"},
			"QCM_CORRECT":          "RED",
			"POINTS_RULE":          map[string]interface{}{"MODE": "FIXED", "VALUE": 10.0},
			"QCM_HINTS_ENABLED":    true,
			"QCM_HINT_THRESHOLD_1": 0.25, "QCM_HINT_THRESHOLD_2": 0.125,
			"QCM_PENALTY_1": 50.0, "QCM_PENALTY_2": 25.0,
		}
		StripCardHints(card)
		for _, k := range hintKeys {
			if _, has := card[k]; has {
				t.Errorf("%s still present", k)
			}
		}
		want := map[string]interface{}{
			"ID": "mc-1", "TYPE": "QCM", "DIFFICULTY": 2.0,
			"QCM_ANSWERS": map[string]interface{}{"RED": "a"},
			"QCM_CORRECT": "RED",
			"POINTS_RULE": map[string]interface{}{"MODE": "FIXED", "VALUE": 10.0},
		}
		if !reflect.DeepEqual(card, want) {
			t.Errorf("got %v, want %v", card, want)
		}
	})

	t.Run("QCM_HINTS_ENABLED false is removed too", func(t *testing.T) {
		card := map[string]interface{}{"ID": "mc-1", "TYPE": "QCM", "QCM_HINTS_ENABLED": false}
		StripCardHints(card)
		if _, has := card["QCM_HINTS_ENABLED"]; has {
			t.Error("QCM_HINTS_ENABLED:false should not be persisted on a card")
		}
	})

	t.Run("card without hints is unchanged", func(t *testing.T) {
		card := map[string]interface{}{"ID": "mc-1", "TYPE": "QCM", "QCM_CORRECT": "RED"}
		want := map[string]interface{}{"ID": "mc-1", "TYPE": "QCM", "QCM_CORRECT": "RED"}
		StripCardHints(card)
		if !reflect.DeepEqual(card, want) {
			t.Errorf("got %v, want %v", card, want)
		}
	})

	t.Run("nil and empty maps are no-ops", func(t *testing.T) {
		StripCardHints(nil)
		empty := map[string]interface{}{}
		StripCardHints(empty)
		if len(empty) != 0 {
			t.Errorf("got %v", empty)
		}
	})

	// A SPEEDY card with NON-ZERO hint content is still a
	// CARD_TYPE_CONTENT_MISMATCH upstream: the validation runs before
	// StripCardHints in handleUploadQuestion, so the card never reaches it.
	t.Run("non-zero hint field on SPEEDY is still a mismatch upstream", func(t *testing.T) {
		card := map[string]interface{}{"ID": "mc-1", "TYPE": "SPEEDY", "QCM_PENALTY_1": 50.0}
		if err := ValidateCardTypeContent(QuestionTypeSpeedy, card); err == nil {
			t.Fatal("want CARD_TYPE_CONTENT_MISMATCH")
		}
	})
}
