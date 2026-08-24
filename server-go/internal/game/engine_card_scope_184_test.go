// Tests for #184 B-B6 — ValidateCardScope, contracts/question-types.md
// §9.2's invariant. Test names mirror the contract table's 4 rows plus the
// generalization to GRID/MEMORIZE (no explicit row in the contract, see
// ValidateCardScope's doc comment for why they fall under "no active
// card").
// Run: go test ./internal/game/... -run TestValidateCardScope -v

package game

import "testing"

// TestValidateCardScope_NoMEMOTIONRound covers contract §9.2's first two
// rows: MEMOTION_SUBPHASE=="" (no round at all).
func TestValidateCardScope_NoMEMOTIONRound(t *testing.T) {
	t.Run("absent MOTION_CARD_ID — accepted", func(t *testing.T) {
		e := NewEngine()
		if err := e.ValidateCardScope(""); err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("present MOTION_CARD_ID — refused CARD_SCOPE_UNEXPECTED", func(t *testing.T) {
		e := NewEngine()
		err := e.ValidateCardScope("mc-1")
		motionErr, ok := err.(*MotionError)
		if !ok || motionErr.Reason != "CARD_SCOPE_UNEXPECTED" {
			t.Errorf("expected MotionError{Reason: CARD_SCOPE_UNEXPECTED}, got %v", err)
		}
	})
}

// TestValidateCardScope_ActiveCard covers contract §9.2's last two rows:
// a MEMOTION round with an active card slot (MotionSelected != "").
func TestValidateCardScope_ActiveCard(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()
	if err := e.SelectMotionCard("mc-1"); err != nil {
		t.Fatalf("SelectMotionCard failed: %v", err)
	}

	t.Run("matching MOTION_CARD_ID — accepted", func(t *testing.T) {
		if err := e.ValidateCardScope("mc-1"); err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("absent MOTION_CARD_ID — refused CARD_SCOPE_MISMATCH", func(t *testing.T) {
		err := e.ValidateCardScope("")
		motionErr, ok := err.(*MotionError)
		if !ok || motionErr.Reason != "CARD_SCOPE_MISMATCH" {
			t.Errorf("expected MotionError{Reason: CARD_SCOPE_MISMATCH}, got %v", err)
		}
	})

	t.Run("mismatched MOTION_CARD_ID — refused CARD_SCOPE_MISMATCH", func(t *testing.T) {
		err := e.ValidateCardScope("mc-2") // mc-2 exists but isn't the active card
		motionErr, ok := err.(*MotionError)
		if !ok || motionErr.Reason != "CARD_SCOPE_MISMATCH" {
			t.Errorf("expected MotionError{Reason: CARD_SCOPE_MISMATCH}, got %v", err)
		}
	})

	t.Run("unknown MOTION_CARD_ID — refused CARD_SCOPE_MISMATCH (no separate CARD_NOT_FOUND here)", func(t *testing.T) {
		err := e.ValidateCardScope("mc-does-not-exist")
		motionErr, ok := err.(*MotionError)
		if !ok || motionErr.Reason != "CARD_SCOPE_MISMATCH" {
			t.Errorf("expected MotionError{Reason: CARD_SCOPE_MISMATCH}, got %v", err)
		}
	})
}

// TestValidateCardScope_MEMOTIONRoundNoActiveCard covers the generalization
// beyond the contract's two explicit rows: a MEMOTION round IS in progress
// (Question.Type==MEMOTION, MotionSubPhase != "") but no card is selected
// yet (GRID/MEMORIZE) — MotionSelected=="", so this must behave exactly
// like "no round at all" per ValidateCardScope's doc comment.
func TestValidateCardScope_MEMOTIONRoundNoActiveCard(t *testing.T) {
	t.Run("GRID, absent MOTION_CARD_ID — accepted", func(t *testing.T) {
		e := NewEngine()
		q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
		startMEMOTION(t, e, "mq1", q)
		defer e.Stop()

		if err := e.ValidateCardScope(""); err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("GRID, present MOTION_CARD_ID — refused CARD_SCOPE_UNEXPECTED", func(t *testing.T) {
		e := NewEngine()
		q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
		startMEMOTION(t, e, "mq1", q)
		defer e.Stop()

		err := e.ValidateCardScope("mc-1")
		motionErr, ok := err.(*MotionError)
		if !ok || motionErr.Reason != "CARD_SCOPE_UNEXPECTED" {
			t.Errorf("expected MotionError{Reason: CARD_SCOPE_UNEXPECTED}, got %v", err)
		}
	})
}
