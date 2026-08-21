package game

import "testing"

// TestSetEntracte_PhaseGuard covers the D4 phase table (contract
// game-state.md §"Phases autorisées", plan B8): activation is allowed only
// from STOPPED/PREPARE/READY/NEW_GAME/REVEALED, refused everywhere else —
// and deactivation always succeeds, from ANY phase, so ENTRACTE can never
// become a dead end.
func TestSetEntracte_PhaseGuard(t *testing.T) {
	tests := []struct {
		name        string
		phase       GamePhase
		wantApplied bool
	}{
		{"STOPPED allowed", PhaseStopped, true},
		{"PREPARE allowed", PhasePrepare, true},
		{"READY allowed", PhaseReady, true},
		{"NEW_GAME allowed", PhaseNewGame, true},
		{"REVEALED allowed", PhaseRevealed, true},
		{"COUNTDOWN refused", PhaseCountdown, false},
		{"STARTED refused", PhaseStarted, false},
		{"PAUSED refused", PhasePaused, false},
		{"ENROLL refused", PhaseEnroll, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine()
			e.SetPhase(tt.phase)

			applied := e.SetEntracte(true)
			if applied != tt.wantApplied {
				t.Errorf("SetEntracte(true) from phase %s: applied=%v, want %v", tt.phase, applied, tt.wantApplied)
			}
			if e.IsEntracte() != tt.wantApplied {
				t.Errorf("IsEntracte() after activation attempt from %s = %v, want %v", tt.phase, e.IsEntracte(), tt.wantApplied)
			}
		})
	}
}

// TestSetEntracte_DeactivationAlwaysSucceeds asserts the "never a dead end"
// invariant explicitly, from every phase in the state machine — including
// the ones where ACTIVATION is refused.
func TestSetEntracte_DeactivationAlwaysSucceeds(t *testing.T) {
	allPhases := []GamePhase{
		PhaseStopped, PhasePrepare, PhaseReady, PhaseCountdown, PhaseStarted,
		PhasePaused, PhaseRevealed, PhaseEnroll, PhaseNewGame,
	}
	for _, phase := range allPhases {
		t.Run(string(phase), func(t *testing.T) {
			e := NewEngine()
			e.SetPhase(phase)
			// Force the flag on directly (bypassing the activation guard —
			// simulates having entered entracte from an eligible phase and
			// then the game moving on, e.g. NEW_GAME -> ENROLL is not a real
			// transition, but the invariant must hold unconditionally).
			e.mu.Lock()
			e.state.Entracte = true
			e.mu.Unlock()

			if !e.SetEntracte(false) {
				t.Fatalf("SetEntracte(false) from phase %s: expected success, got refused", phase)
			}
			if e.IsEntracte() {
				t.Errorf("IsEntracte() after deactivation from %s = true, want false", phase)
			}
		})
	}
}

// TestSetEntracte_RefusedActivationChangesNothingElse (D4/D6): a refused
// activation must not touch any other part of GameState — in particular a
// question already selected in PREPARE must be found intact.
func TestSetEntracte_ActivationDoesNotTouchOtherState(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhasePrepare)
	e.state.Question = &Question{ID: "42", Question: "?"}

	if !e.SetEntracte(true) {
		t.Fatal("expected activation to succeed from PREPARE")
	}
	if e.state.Phase != PhasePrepare {
		t.Errorf("Phase changed by SetEntracte: got %s, want unchanged PREPARE", e.state.Phase)
	}
	if e.state.Question == nil || e.state.Question.ID != "42" {
		t.Errorf("Question was clobbered by SetEntracte: got %+v", e.state.Question)
	}
}

// TestSetEntracte_RefusedActivationLeavesFlagFalse: a refused activation
// (wrong phase) must leave GameState.Entracte at false, not partially set.
func TestSetEntracte_RefusedActivationLeavesFlagFalse(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseStarted)

	if e.SetEntracte(true) {
		t.Fatal("expected activation to be refused from STARTED")
	}
	if e.IsEntracte() {
		t.Error("IsEntracte() = true after a refused activation, want false")
	}
}

// TestSetEntracte_Idempotent (D3): activating twice, or deactivating twice,
// is a no-op each time, never an error — mirrors the "explicit command, not
// a toggle" design: a double-click can never invert the state.
func TestSetEntracte_Idempotent(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseStopped)

	if !e.SetEntracte(true) {
		t.Fatal("first activation should succeed")
	}
	if !e.SetEntracte(true) {
		t.Fatal("second activation (already active) should still report applied")
	}
	if !e.IsEntracte() {
		t.Fatal("expected entracte active after two activations")
	}

	if !e.SetEntracte(false) {
		t.Fatal("first deactivation should succeed")
	}
	if !e.SetEntracte(false) {
		t.Fatal("second deactivation (already inactive) should still report applied")
	}
	if e.IsEntracte() {
		t.Fatal("expected entracte inactive after two deactivations")
	}
}

// TestSetEntracteConfig_Mirrors verifies the plain setter round-trips the
// full struct, matching SetBackgrounds' pattern.
func TestSetEntracteConfig_Mirrors(t *testing.T) {
	e := NewEngine()
	cfg := EntracteConfig{
		Title:         "Pause",
		Subtitle:      "Bientôt de retour",
		ImageIsCustom: true,
		PanelSize:     80,
		AnimPeriod:    5,
		AnimIntensity: 0, // explicitly disabled — must survive the round-trip
	}
	e.SetEntracteConfig(cfg)

	got := e.GetState().EntracteConfig
	if got != cfg {
		t.Errorf("SetEntracteConfig round-trip mismatch: got %+v, want %+v", got, cfg)
	}
}
