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
// MessageLost transition (orange -> red) on the very first GameState
// broadcast evaluated after the disconnect.
//
// #129 UPDATE: prior to #129, the very first broadcast after a disconnect
// got a one-shot grace pass (ORANGE stayed ORANGE) — because that broadcast
// was, at the time, always onPlayerDisconnected's own a.broadcastUpdate(),
// which reached every VJoueur including the one that had just disconnected;
// without the grace pass, ORANGE would never actually be visible before
// immediately flipping to RED. #129 T1.3 retargets that specific broadcast
// to Admin/TV/Buzzer only — it no longer reaches VPlayer and so no longer
// calls ApplyVPlayerBroadcastConnEvents() at all. The self-referential case
// the grace pass protected against is now structurally impossible, so
// transitionConnUnsafe's ConnEventDisconnect case no longer arms it
// (engine.go) — ORANGE is guaranteed visible simply because no VPlayer-
// targeting broadcast fires at the moment of disconnect anymore, not
// because of a pass consumed on the first one. See
// TestApplyVPlayerBroadcastConnEvents_DisconnectAnnouncement_NoLongerReachesVPlayer
// below for the regression test the #129 review requested, and
// cmd/server/connstate_protocol_regression_test.go for the integration-level
// equivalent (onPlayerDisconnected + a genuinely separate broadcastUpdate()).
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
	// Simulate disconnect (main.go onPlayerDisconnected path).
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	if got := e.GetBumper(id).ConnState; got != ConnStateOrange {
		t.Fatalf("setup failed: expected orange after disconnect, got %q", got)
	}

	// #129: no VPlayer-targeting broadcast fires at the moment of disconnect
	// anymore (onPlayerDisconnected targets Admin/TV/Buzzer only), so the
	// FIRST call to ApplyVPlayerBroadcastConnEvents() here is already a
	// genuinely later, missed broadcast — D4 applies immediately.
	e.ApplyVPlayerBroadcastConnEvents()
	if got := e.GetBumper(id).ConnState; got != ConnStateRed {
		t.Errorf("D4: expected orange -> red on the first broadcast evaluated after a disconnect (no grace pass post-#129), got %q", got)
	}

	// Idempotence: a second broadcast while still disconnected must stay red.
	e.ApplyVPlayerBroadcastConnEvents()
	if got := e.GetBumper(id).ConnState; got != ConnStateRed {
		t.Errorf("expected red to stay red on repeated broadcasts while disconnected, got %q", got)
	}
}

// TestApplyVPlayerBroadcastConnEvents_DisconnectAnnouncement_NoLongerReachesVPlayer
// is the #129 regression test requested during review: reproduces the exact
// scenario TestConnStateProtocol_MissedBroadcastWhileDisconnected_StillTurnsRed
// (cmd/server) exercises at the integration level, at the engine level —
// confirms a genuinely missed broadcast unrelated to the disconnect itself
// now correctly turns the badge red, proving the stale grace pass no longer
// absorbs it by mistake (the bug this fix corrects: before it, this exact
// sequence left the badge stuck on ORANGE — see git history for the 1-line
// change in transitionConnUnsafe's ConnEventDisconnect case).
func TestApplyVPlayerBroadcastConnEvents_DisconnectAnnouncement_NoLongerReachesVPlayer(t *testing.T) {
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

	// Disconnect (main.go onPlayerDisconnected: UpdateBumper only — #129 no
	// longer follows it with a VPlayer-targeting broadcast at this call site).
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	if got := e.GetBumper(id).ConnState; got != ConnStateOrange {
		t.Fatalf("expected orange after disconnect, got %q", got)
	}

	// Something else happens in the game while Bob is still disconnected
	// (e.g. another team's score changes) -> a real GameState broadcast that
	// Bob genuinely misses. This is the FIRST call to
	// ApplyVPlayerBroadcastConnEvents() since the disconnect.
	e.ApplyVPlayerBroadcastConnEvents()

	if got := e.GetBumper(id).ConnState; got != ConnStateRed {
		t.Errorf("#129 regression: expected orange -> red on a broadcast genuinely missed while disconnected, got %q (a stale grace pass would incorrectly keep this orange)", got)
	}
}

// TestSetBumpers_PreservesSkipNextMessageLost covers the code-review finding
// (20260726, minor, non-blocking) on the stuck-red badge fix: a bulk
// SetBumpers landing while a one-shot grace pass is armed on a bumper must
// not drop it — the field must survive a client-object round-trip for the
// same ID.
//
// #129 UPDATE: since #129 (see the test above), nothing in production ever
// arms this flag anymore — the disconnect path that used to set it no
// longer does (transitionConnUnsafe's ConnEventDisconnect case). The
// preservation invariant this test protects is still real code
// (ApplyVPlayerBroadcastConnEvents still consumes the flag if it's ever
// true, SetBumpers still carries it across a replacement, see engine.go) —
// only dormant, not removed, per the #129 review's explicit request to keep
// this fix to the one line that caused the regression. This test now arms
// the flag directly (same package, unexported field) rather than via a real
// disconnect, so it keeps proving the preservation invariant independent of
// whatever code path might arm the flag in the future.
func TestSetBumpers_PreservesSkipNextMessageLost(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"TeamA": {Name: "TeamA"}})
	e.SetPhase(PhaseEnroll)
	id, _, err := e.CreateVirtualPlayer("Nina")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	if err := e.AssignVirtualPlayer(id, "TeamA", AnswerColorNone); err != nil {
		t.Fatalf("AssignVirtualPlayer failed: %v", err)
	}
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	if got := e.GetBumper(id).ConnState; got != ConnStateOrange {
		t.Fatalf("setup failed: expected orange after disconnect, got %q", got)
	}
	// Arm the grace pass directly — #129 removed the only production call
	// site that used to do this on disconnect (see comment above).
	e.data.Bumpers[id].skipNextMessageLost = true

	// A bulk SetBumpers lands right in the middle of the window — simulates an
	// admin FULL/UPDATE (e.g. TeamsPage) whose payload round-trips a bumper
	// object built from a client-side snapshot (a *different* struct value,
	// not the live pointer) for the same ID, unaware of the pending grace pass.
	current := e.GetTeamsAndBumpers().Bumpers
	replacement := make(map[string]*Bumper, len(current))
	for bid, b := range current {
		cp := *b // shallow copy — mimics a client-sent object, not the live pointer
		replacement[bid] = &cp
	}
	e.SetBumpers(replacement)

	// The very next broadcast must STILL be the (consumed) grace pass — orange,
	// not red. If skipNextMessageLost had been dropped by SetBumpers, this
	// would incorrectly jump straight to red.
	e.ApplyVPlayerBroadcastConnEvents()
	if got := e.GetBumper(id).ConnState; got != ConnStateOrange {
		t.Errorf("code-review regression: SetBumpers dropped the grace pass mid-window, got %q instead of orange", got)
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
