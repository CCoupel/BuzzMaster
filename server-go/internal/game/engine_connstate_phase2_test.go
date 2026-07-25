package game

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests : câblage Phase 2 de la machine CONN_STATE (#109)
//
// Contexte : la table pure (transitionConnUnsafe/TransitionConn) est déjà
// couverte par engine_connstate_test.go (Phase 1). Ce fichier couvre les
// mécanismes ajoutés en Phase 2 (plan §2, D2/D3/D4) :
//   - ConfirmDelivery : entrée gated pour DELIVERY_CONFIRMED, respecte la
//     fenêtre minimale "green" (connGreenMinDuration, raccourcie ici pour les
//     tests — jamais de sleep de 2s réelles).
//   - ApplyVPlayerBroadcastConnEvents : évaluation MessageLost/DeliveryConfirmed
//     pour tous les VJoueurs participants à chaque broadcast GameState (D4).
//
// Le câblage réel des sources (handleBuzzerACK, sendLEDSet, handleWebMessage)
// est testé côté cmd/server (player_connect_connstate_phase2_test.go) — ce
// fichier ne couvre que la logique moteur, indépendante du transport.
// ---------------------------------------------------------------------------

// withShortGreenWindow temporarily shrinks connGreenMinDuration for a test and
// restores it afterward. Keeps tests fast (milliseconds, not real seconds)
// without touching the production default (2s, D2).
func withShortGreenWindow(t *testing.T, d time.Duration) {
	t.Helper()
	original := connGreenMinDuration
	connGreenMinDuration = d
	t.Cleanup(func() { connGreenMinDuration = original })
}

// TestConfirmDelivery_ImmediateWhenWindowElapsed verifies that a
// DeliveryConfirmed arriving after the minimum green window has already
// elapsed applies immediately (no artificial extra delay).
func TestConfirmDelivery_ImmediateWhenWindowElapsed(t *testing.T) {
	withShortGreenWindow(t, 20*time.Millisecond)

	e := NewEngine()
	id := newParticipantBumper(e)
	e.TransitionConn(id, ConnEventDisconnect)
	e.TransitionConn(id, ConnEventReconnect)
	if got := e.GetBumper(id).ConnState; got != ConnStateGreen {
		t.Fatalf("setup failed: expected green, got %q", got)
	}

	time.Sleep(30 * time.Millisecond) // window (20ms) has elapsed

	e.ConfirmDelivery(id)

	if got := e.GetBumper(id).ConnState; got != ConnStateHidden {
		t.Errorf("expected ConnState=HIDDEN after window elapsed, got %q", got)
	}
}

// TestConfirmDelivery_DeferredWithinWindow verifies that a DeliveryConfirmed
// arriving BEFORE the minimum green window elapses does not hide the badge
// immediately, but a timer applies it automatically once the window closes
// (D2/D3: "transition effective vers HIDDEN seulement une fois les 2s écoulées").
func TestConfirmDelivery_DeferredWithinWindow(t *testing.T) {
	withShortGreenWindow(t, 40*time.Millisecond)

	e := NewEngine()
	id := newParticipantBumper(e)
	e.TransitionConn(id, ConnEventDisconnect)
	e.TransitionConn(id, ConnEventReconnect)

	e.ConfirmDelivery(id) // arrives immediately — well within the 40ms window

	if got := e.GetBumper(id).ConnState; got != ConnStateGreen {
		t.Fatalf("expected ConnState to remain green immediately after an early confirm, got %q", got)
	}

	time.Sleep(70 * time.Millisecond) // > window: the scheduled timer should have fired

	if got := e.GetBumper(id).ConnState; got != ConnStateHidden {
		t.Errorf("expected ConnState=HIDDEN once the window elapsed (deferred confirm), got %q", got)
	}
}

// TestConfirmDelivery_NoBoundWithoutConfirmation verifies D7 (no max bound):
// a bumper that stays green without ever receiving a DeliveryConfirmed simply
// stays green — no timer forces it to HIDDEN on its own.
func TestConfirmDelivery_NoBoundWithoutConfirmation(t *testing.T) {
	withShortGreenWindow(t, 20*time.Millisecond)

	e := NewEngine()
	id := newParticipantBumper(e)
	e.TransitionConn(id, ConnEventDisconnect)
	e.TransitionConn(id, ConnEventReconnect)

	time.Sleep(50 * time.Millisecond) // well past the window, but no ConfirmDelivery call

	if got := e.GetBumper(id).ConnState; got != ConnStateGreen {
		t.Errorf("D7: expected ConnState to remain green with no bound absent any confirmation, got %q", got)
	}
}

// TestConfirmDelivery_StaleTimerIgnoredAfterFreshReconnect covers the tie-break
// guard: an early confirm schedules a timer against the CURRENT green period.
// If the bumper disconnects and reconnects again before that timer fires (a
// fresh green period, new greenSince), the stale timer must not hide the new
// period prematurely.
func TestConfirmDelivery_StaleTimerIgnoredAfterFreshReconnect(t *testing.T) {
	withShortGreenWindow(t, 30*time.Millisecond)

	e := NewEngine()
	id := newParticipantBumper(e)
	e.TransitionConn(id, ConnEventDisconnect)
	e.TransitionConn(id, ConnEventReconnect)
	e.ConfirmDelivery(id) // schedules a timer ~30ms out for THIS green period

	// Re-disconnect and reconnect before the stale timer fires — starts a fresh
	// green period with a new greenSince.
	time.Sleep(5 * time.Millisecond)
	e.TransitionConn(id, ConnEventDisconnect)
	e.TransitionConn(id, ConnEventReconnect)
	if got := e.GetBumper(id).ConnState; got != ConnStateGreen {
		t.Fatalf("setup failed: expected green after fresh reconnect, got %q", got)
	}

	// Wait past the stale timer's original firing point.
	time.Sleep(40 * time.Millisecond)

	if got := e.GetBumper(id).ConnState; got != ConnStateGreen {
		t.Errorf("stale timer from the previous green period incorrectly hid the badge during the new one, got %q", got)
	}
}

// TestConfirmDelivery_NonGreenAppliesImmediately verifies that ConfirmDelivery
// falls through to the plain, ungated transition for any bumper that isn't
// currently green (nothing to gate — matches TransitionConn's own table).
func TestConfirmDelivery_NonGreenAppliesImmediately(t *testing.T) {
	e := NewEngine()
	id := newParticipantBumper(e)
	// Bumper is HIDDEN (fresh) — DeliveryConfirmed on HIDDEN is a no-op per table.
	e.ConfirmDelivery(id)
	if got := e.GetBumper(id).ConnState; got != ConnStateHidden {
		t.Errorf("expected HIDDEN to stay HIDDEN, got %q", got)
	}
}

// TestConfirmDelivery_UnknownBumper_NoPanic is a defensive test: an unknown
// bumper ID must be a safe no-op, consistent with TransitionConn.
func TestConfirmDelivery_UnknownBumper_NoPanic(t *testing.T) {
	e := NewEngine()
	e.ConfirmDelivery("does-not-exist") // must not panic
}

// TestApplyVPlayerBroadcastConnEvents_DisconnectedParticipant_MessageLost
// verifies D4 (no restricted list): a disconnected participant VJoueur gets a
// MessageLost transition (orange -> red) on each simulated GameState broadcast.
func TestApplyVPlayerBroadcastConnEvents_DisconnectedParticipant_MessageLost(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"TeamA": {Name: "TeamA"}})
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	if err := e.AssignVirtualPlayer(id, "TeamA", AnswerColorNone); err != nil {
		t.Fatalf("AssignVirtualPlayer failed: %v", err)
	}
	// Simulate disconnect (main.go OnPlayerDisconnected path).
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	if got := e.GetBumper(id).ConnState; got != ConnStateOrange {
		t.Fatalf("setup failed: expected orange after disconnect, got %q", got)
	}

	e.ApplyVPlayerBroadcastConnEvents() // simulates one broadcastUpdate() cycle

	if got := e.GetBumper(id).ConnState; got != ConnStateRed {
		t.Errorf("D4: expected orange -> red after a broadcast while disconnected, got %q", got)
	}

	// Idempotence: a second broadcast while still disconnected must stay red.
	e.ApplyVPlayerBroadcastConnEvents()
	if got := e.GetBumper(id).ConnState; got != ConnStateRed {
		t.Errorf("expected red to stay red on repeated broadcasts while disconnected, got %q", got)
	}
}

// TestApplyVPlayerBroadcastConnEvents_ConnectedParticipant_DeliveryConfirmed
// verifies D3: a connected participant VJoueur counts a GameState broadcast as
// a successful delivery, eventually closing the green window.
func TestApplyVPlayerBroadcastConnEvents_ConnectedParticipant_DeliveryConfirmed(t *testing.T) {
	withShortGreenWindow(t, 20*time.Millisecond)

	e := NewEngine()
	e.SetTeams(map[string]*Team{"TeamA": {Name: "TeamA"}})
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Bob")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	if err := e.AssignVirtualPlayer(id, "TeamA", AnswerColorNone); err != nil {
		t.Fatalf("AssignVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	e.TransitionConn(id, ConnEventReconnect) // -> green (simulates a real reconnect)
	if got := e.GetBumper(id).ConnState; got != ConnStateGreen {
		t.Fatalf("setup failed: expected green, got %q", got)
	}

	// Bumper is Connected==true in the engine model at this point (CreateVirtualPlayer
	// sets Connected:true and it was never flipped back after the disconnect simulation
	// above — re-affirm explicitly so the test doesn't depend on that incidental state).
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": true})

	e.ApplyVPlayerBroadcastConnEvents() // early confirm — within the window

	if got := e.GetBumper(id).ConnState; got != ConnStateGreen {
		t.Fatalf("expected ConnState to remain green immediately after an early broadcast-confirm, got %q", got)
	}

	time.Sleep(40 * time.Millisecond)

	if got := e.GetBumper(id).ConnState; got != ConnStateHidden {
		t.Errorf("expected ConnState=HIDDEN once the window elapsed after broadcast-confirm, got %q", got)
	}
}

// TestApplyVPlayerBroadcastConnEvents_IgnoresNonParticipantsAndBuzzers verifies
// the scope filter still applies: non-participant VJoueurs (Team=="") and
// physical buzzers (IsVPlayer==false) are never touched by the broadcast sweep.
func TestApplyVPlayerBroadcastConnEvents_IgnoresNonParticipantsAndBuzzers(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)

	unassignedID, _, err := e.CreateVirtualPlayer("Unassigned")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(unassignedID, map[string]interface{}{"CONNECTED": false})

	// A disconnected, team-assigned physical buzzer legitimately shows "orange"
	// already (Phase 1's UpdateBumper CONNECTED hook) — what this test proves is
	// that the VJoueur-only broadcast sweep does NOT additionally push it to
	// "red" via MessageLost, which is reserved for VJoueurs.
	buzzerID := "AA:BB:CC:DD:EE:01"
	e.UpdateBumper(buzzerID, map[string]interface{}{"TEAM": "TeamA", "CONNECTED": false})
	if got := e.GetBumper(buzzerID).ConnState; got != ConnStateOrange {
		t.Fatalf("setup failed: expected orange from the CONNECTED hook, got %q", got)
	}

	e.ApplyVPlayerBroadcastConnEvents()

	if got := e.GetBumper(unassignedID).ConnState; got != ConnStateHidden {
		t.Errorf("non-participant VJoueur must stay hidden, got %q", got)
	}
	if got := e.GetBumper(buzzerID).ConnState; got != ConnStateOrange {
		t.Errorf("physical buzzer must be untouched by the VJoueur broadcast sweep (no MessageLost), got %q", got)
	}
}
