// Tests for #200 cycle 3 — Engine.Pause()/Continue() had NO phase guard at
// all, unlike every other transition (Start's #172 B4 guard, Reveal's own).
// QUALIF report on v8.0.0.17: "je peux toujours faire START alors qu'aucune
// équipe n'est sélectionnée" for MEMORY, AFTER the Ready()-replay fix
// (142ffc3c) — a DIFFERENT, more direct bypass: ACTION:"CONTINUE"
// (cmd/server/main.go) forwards straight to Continue(), which used to set
// Phase=PhaseStarted unconditionally, reachable even from PREPARE before any
// team was ever selected — skipping participantsConform/AreAllTeamsReady/
// TransitionToReady/ForceReady entirely. See engine.go's own comment on
// Pause()/Continue() for the full root-cause analysis, including why BOTH
// functions needed a guard (Pause() alone being unguarded would still allow
// a 2-step PREPARE->PAUSED->STARTED bypass via Continue()'s own guard).
//
// Run: go test ./internal/game/... -run TestEngineContinue_RefusesOutsidePaused\|TestEnginePause_RefusesOutsideStarted\|TestContinue_CannotBypassParticipantGate\|TestRafaleContinue_CannotBypassParticipantGate -v
package game

import "testing"

// TestEngineContinue_RefusesOutsidePaused mirrors
// TestEngineStart_RefusesOutsideReady (engine_participants_conform_test.go)
// for Continue(): every phase other than PAUSED must be refused, PAUSED
// must be accepted.
func TestEngineContinue_RefusesOutsidePaused(t *testing.T) {
	t.Run("from STOPPED (default)", func(t *testing.T) {
		e := NewEngine()
		if e.GetPhase() != PhaseStopped {
			t.Fatalf("precondition: expected STOPPED, got %s", e.GetPhase())
		}
		e.Continue()
		if e.GetPhase() != PhaseStopped {
			t.Errorf("Continue() must be refused from STOPPED, got %s", e.GetPhase())
		}
	})

	t.Run("from PREPARE — the exact QUALIF bypass", func(t *testing.T) {
		e := NewEngine()
		e.Ready("q1", nil)
		if e.GetPhase() != PhasePrepare {
			t.Fatalf("precondition: expected PREPARE, got %s", e.GetPhase())
		}
		e.Continue()
		if e.GetPhase() != PhasePrepare {
			t.Errorf("Continue() must be refused from PREPARE — this is the exact bypass reported in QUALIF v8.0.0.17, got %s", e.GetPhase())
		}
	})

	t.Run("from READY", func(t *testing.T) {
		e := NewEngine()
		e.Ready("q1", nil)
		e.TransitionToReady()
		if e.GetPhase() != PhaseReady {
			t.Fatalf("precondition: expected READY, got %s", e.GetPhase())
		}
		e.Continue()
		if e.GetPhase() != PhaseReady {
			t.Errorf("Continue() must be refused from READY, got %s", e.GetPhase())
		}
	})

	t.Run("from STARTED (already running, not paused)", func(t *testing.T) {
		e := NewEngine()
		e.StartImmediate(30)
		if e.GetPhase() != PhaseStarted {
			t.Fatalf("precondition: expected STARTED, got %s", e.GetPhase())
		}
		defer e.Stop()
		e.Continue()
		if e.GetPhase() != PhaseStarted {
			t.Errorf("Continue() from an already-STARTED game must be a no-op, got %s", e.GetPhase())
		}
	})

	t.Run("from PAUSED (positive control)", func(t *testing.T) {
		e := NewEngine()
		e.StartImmediate(30)
		e.Pause()
		if e.GetPhase() != PhasePaused {
			t.Fatalf("precondition: expected PAUSED, got %s", e.GetPhase())
		}
		defer e.Stop()
		e.Continue()
		if e.GetPhase() != PhaseStarted {
			t.Errorf("Continue() must be accepted from PAUSED, got %s", e.GetPhase())
		}
	})
}

// TestEnginePause_RefusesOutsideStarted mirrors the above for Pause(): only
// a genuinely running game (STARTED) can be paused.
func TestEnginePause_RefusesOutsideStarted(t *testing.T) {
	t.Run("from STOPPED (default)", func(t *testing.T) {
		e := NewEngine()
		e.Pause()
		if e.GetPhase() != PhaseStopped {
			t.Errorf("Pause() must be refused from STOPPED, got %s", e.GetPhase())
		}
	})

	t.Run("from PREPARE — closes the 2-step PREPARE->PAUSED->STARTED bypass", func(t *testing.T) {
		e := NewEngine()
		e.Ready("q1", nil)
		if e.GetPhase() != PhasePrepare {
			t.Fatalf("precondition: expected PREPARE, got %s", e.GetPhase())
		}
		e.Pause()
		if e.GetPhase() != PhasePrepare {
			t.Errorf("Pause() must be refused from PREPARE — without this, Continue()'s own PAUSED-only guard could still be reached via Pause() first, got %s", e.GetPhase())
		}
		// The 2-step bypass itself: Continue() right after must also refuse,
		// since Pause() above never actually reached PAUSED.
		e.Continue()
		if e.GetPhase() != PhasePrepare {
			t.Errorf("Continue() after a refused Pause() must still refuse — 2-step PREPARE->PAUSED->STARTED bypass must be closed, got %s", e.GetPhase())
		}
	})

	t.Run("from READY", func(t *testing.T) {
		e := NewEngine()
		e.Ready("q1", nil)
		e.TransitionToReady()
		if e.GetPhase() != PhaseReady {
			t.Fatalf("precondition: expected READY, got %s", e.GetPhase())
		}
		e.Pause()
		if e.GetPhase() != PhaseReady {
			t.Errorf("Pause() must be refused from READY, got %s", e.GetPhase())
		}
	})

	t.Run("from STARTED (positive control)", func(t *testing.T) {
		e := NewEngine()
		e.StartImmediate(30)
		defer e.Stop()
		e.Pause()
		if e.GetPhase() != PhasePaused {
			t.Errorf("Pause() must be accepted from STARTED, got %s", e.GetPhase())
		}
	})
}

// TestContinue_CannotBypassParticipantGate is the exact end-to-end
// reproduction of the QUALIF report: a MEMORY question with ZERO teams ever
// selected (not a replay — this is the FIRST manche) must never reach
// STARTED via ACTION:"CONTINUE", the same way it already can't via
// ACTION:"START" (TestStart_MemorySolo_UnreachableWithoutExplicitSelection,
// engine_participants_conform_test.go).
func TestContinue_CannotBypassParticipantGate(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Red"}, "blue": {Name: "Blue"}})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})

	q := &Question{ID: "q1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}}
	e.Ready("q1", q)
	e.SetBumperReady("b1")
	e.SetBumperReady("b2")

	if e.ParticipantsConform() {
		t.Fatal("precondition: no team was ever selected, must not be conform")
	}
	if e.GetPhase() != PhasePrepare {
		t.Fatalf("precondition: expected PREPARE, got %s", e.GetPhase())
	}

	// The exact bypass: ACTION:"CONTINUE" straight from PREPARE.
	e.Continue()
	if e.GetPhase() != PhasePrepare {
		t.Errorf("Continue() must never move a MEMORY question with no team selected out of PREPARE, got %s", e.GetPhase())
	}

	// The 2-step variant: PAUSE then CONTINUE.
	e.Pause()
	e.Continue()
	if e.GetPhase() != PhasePrepare {
		t.Errorf("Pause()+Continue() must never move a MEMORY question with no team selected out of PREPARE either, got %s", e.GetPhase())
	}

	// Positive control: the legitimate path still works once a team is
	// actually selected.
	if err := e.SetMemoryParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams: %v", err)
	}
	if e.GetPhase() != PhaseReady {
		t.Fatalf("precondition: expected READY once conform, got %s", e.GetPhase())
	}
	e.StartImmediate(0)
	if e.GetPhase() != PhaseStarted {
		t.Fatalf("precondition: expected STARTED, got %s", e.GetPhase())
	}
	defer e.Stop()
	e.Pause()
	if e.GetPhase() != PhasePaused {
		t.Fatalf("precondition: expected PAUSED, got %s", e.GetPhase())
	}
	e.Continue()
	if e.GetPhase() != PhaseStarted {
		t.Errorf("Continue() must still work normally for a legitimately-started game, got %s", e.GetPhase())
	}
}

// TestRafaleContinue_CannotBypassParticipantGate is the cross-type
// belt-and-suspenders companion to TestContinue_CannotBypassParticipantGate
// above, requested during the #200 confirmation audit (user question: does
// RAFALE share the same MEMORY-only-looking bypass?). Audit conclusion,
// verified here permanently: COVERED by construction, no RAFALE-specific
// gap. Continue()/Pause() (engine.go) are fully generic — a single
// implementation, gating on the shared GameState.Phase field, with no
// QuestionType branch anywhere in either function or in
// broadcastPauseAll()/broadcastContinue() (main.go, #200 cycle 4) — and
// Phase can only ever reach STARTED via Start() (gated on READY, #172 B4)
// or Continue() (gated on PAUSED since 64b23dff, itself only reachable via
// Pause() from STARTED), so any STARTED state structurally traces back to a
// Start() that was gated on participantsConform — regardless of question
// type. This test locks that invariant in for RAFALE specifically, mirroring
// TestReady_RafaleReplay_ResetsStaleParticipatingTeams's own reservoir setup
// (rafale_modes_test.go).
func TestRafaleContinue_CannotBypassParticipantGate(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
	seedRafaleReservoirBulk(t, e, 20, CategoryHistory, 1)

	q := makeRafaleQuestion("rq1", string(RafaleModeChacunSonTour), CategoryHistory, 1)
	e.Ready("rq1", q)
	// No RAFALE_SET_TEAMS call — zero teams ever selected.

	if e.ParticipantsConform() {
		t.Fatal("precondition: no team selected, RAFALE multi-mode must not be conform")
	}
	if e.GetPhase() != PhasePrepare {
		t.Fatalf("precondition: expected PREPARE, got %s", e.GetPhase())
	}

	// The direct bypass: ACTION:"CONTINUE" straight from PREPARE.
	e.Continue()
	if e.GetPhase() != PhasePrepare {
		t.Errorf("Continue() must never move a RAFALE question with no team selected out of PREPARE, got %s", e.GetPhase())
	}

	// The 2-step variant: PAUSE then CONTINUE.
	e.Pause()
	e.Continue()
	if e.GetPhase() != PhasePrepare {
		t.Errorf("Pause()+Continue() must never move a RAFALE question with no team selected out of PREPARE either, got %s", e.GetPhase())
	}

	// Positive control: the legitimate path still works once a team is
	// actually selected.
	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	e.ForceReady()
	if e.GetPhase() != PhaseReady {
		t.Fatalf("precondition: expected READY once conform, got %s", e.GetPhase())
	}
	e.StartImmediate(0)
	if e.GetPhase() != PhaseStarted {
		t.Fatalf("precondition: expected STARTED, got %s", e.GetPhase())
	}
	defer e.Stop()
	e.Pause()
	if e.GetPhase() != PhasePaused {
		t.Fatalf("precondition: expected PAUSED, got %s", e.GetPhase())
	}
	e.Continue()
	if e.GetPhase() != PhaseStarted {
		t.Errorf("Continue() must still work normally for a legitimately-started RAFALE game, got %s", e.GetPhase())
	}
}
