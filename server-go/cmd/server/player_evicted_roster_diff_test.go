package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests : notification d'éviction sur le chemin RÉELLEMENT emprunté par
// l'admin, et registre des motifs (#123, B1/B3)
//
// Plan : _work/reports/plan-20260730-094500.md.
//
// Cause racine A1 : #120's PLAYER_EVICTED notification was wired only into
// DELETE_BUMPER — an action the real admin UI (TeamsPage.jsx) never sends.
// It deletes a player via `updateConfig({ bumpers: newBumpers })`, i.e. a
// plain UPDATE carrying an amputated roster. #120's own tests (still in
// player_evicted_test.go) called handleDeleteBumper DIRECTLY and so never
// caught this. **The test below is the one that matters most in this file**:
// it goes through handleUpdate with the EXACT payload shape TeamsPage.jsx
// produces, never handleDeleteBumper.
// ---------------------------------------------------------------------------

// updateBumpersMsg builds an UPDATE message carrying exactly the payload
// shape TeamsPage.jsx's handleDeleteBumper produces:
// `updateConfig({ bumpers: newBumpers })` → `sendMessage('UPDATE', { bumpers:
// newBumpers })` (useWebSocket.js) — lowercase `bumpers` key, matching
// handleFullUpdate's own unmarshal struct tag.
func updateBumpersMsg(t *testing.T, bumpers map[string]*game.Bumper) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionUpdate, map[string]interface{}{"bumpers": bumpers})
	if err != nil {
		t.Fatalf("failed to build UPDATE message: %v", err)
	}
	return msg
}

// TestHandleUpdate_RosterDiff_EmitsPlayerEvicted_ForDisappearedVirtualPlayer
// is the central test of this plan: it exercises handleUpdate (the action
// TeamsPage.jsx actually sends) with a roster amputated of a virtual
// bumper — NOT a direct call to handleDeleteBumper — and verifies the
// evicted player's own client receives PLAYER_EVICTED{PLAYER_REMOVED}, while
// an unrelated bystander VJoueur (still present in the roster) receives
// nothing of the kind.
func TestHandleUpdate_RosterDiff_EmitsPlayerEvicted_ForDisappearedVirtualPlayer(t *testing.T) {
	app := newTestAppWithHub(t)
	app.evictionRegistry = server.NewEvictionRegistry()
	app.engine.SetPhase(game.PhaseEnroll)

	aliceID, _, err := app.engine.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer(Alice) failed: %v", err)
	}
	bobID, _, err := app.engine.CreateVirtualPlayer("Bob")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer(Bob) failed: %v", err)
	}

	baseURL := startEvictionTestServer(t, app)

	aliceConn := dialWS(t, baseURL, "/ws/player")
	aliceClientID := learnClientID(t, app, aliceConn)
	app.wsHub.SetClientPlayerID(aliceClientID, aliceID)

	bobConn := dialWS(t, baseURL, "/ws/player")
	bobClientID := learnClientID(t, app, bobConn)
	app.wsHub.SetClientPlayerID(bobClientID, bobID)

	// Exactly what TeamsPage.jsx's handleDeleteBumper sends: the full roster
	// minus Alice's entry (`delete newBumpers[mac]`), NOT a DELETE_BUMPER
	// action.
	amputatedRoster := map[string]*game.Bumper{
		bobID: {Name: "Bob", IsVirtual: true, IsVPlayer: true, Connected: true},
	}
	app.handleUpdate(updateBumpersMsg(t, amputatedRoster))

	action, rawMsg := readAction(t, aliceConn)
	if action != protocol.ActionPlayerEvicted {
		t.Fatalf("expected Alice's client to receive %s via the REAL admin path (UPDATE), got %q", protocol.ActionPlayerEvicted, action)
	}
	var payload protocol.PlayerEvictedPayload
	if err := json.Unmarshal(rawMsg, &payload); err != nil {
		t.Fatalf("failed to unmarshal PLAYER_EVICTED payload: %v", err)
	}
	if payload.Reason != "PLAYER_REMOVED" {
		t.Errorf("expected REASON=PLAYER_REMOVED, got %q", payload.Reason)
	}

	bobActions := collectActions(bobConn, 300*time.Millisecond)
	if containsAction(bobActions, protocol.ActionPlayerEvicted) {
		t.Errorf("Bob (still present in the roster) must never receive PLAYER_EVICTED, got actions: %v", bobActions)
	}
}

// TestHandleUpdate_RosterDiff_PhysicalBuzzerDisappearing_NoNotification
// verifies that a physical buzzer disappearing from an UPDATE's roster never
// triggers a notification — physical buzzers have no WebSocket client of
// their own to notify, and are never candidates for PLAYER_EVICTED.
func TestHandleUpdate_RosterDiff_PhysicalBuzzerDisappearing_NoNotification(t *testing.T) {
	app := newTestAppWithHub(t)
	app.evictionRegistry = server.NewEvictionRegistry()
	app.engine.SetBumpers(map[string]*game.Bumper{
		"AA:BB:CC:DD:EE:01": {Name: "Buzzer1", IsVirtual: false, Connected: true},
	})

	baseURL := startEvictionTestServer(t, app)
	bystanderConn := dialWS(t, baseURL, "/ws/player")

	// Roster now empty: the physical buzzer "disappeared".
	app.handleUpdate(updateBumpersMsg(t, map[string]*game.Bumper{}))

	actions := collectActions(bystanderConn, 300*time.Millisecond)
	if containsAction(actions, protocol.ActionPlayerEvicted) {
		t.Errorf("a physical buzzer disappearing from the roster must never emit PLAYER_EVICTED, got actions: %v", actions)
	}
}

// TestHandleUpdate_RosterDiff_NoRemoval_NoNotification verifies that an
// UPDATE carrying the SAME roster (nothing actually removed — e.g. an
// unrelated field edit such as ANSWER_COLOR) never emits a spurious
// PLAYER_EVICTED.
func TestHandleUpdate_RosterDiff_NoRemoval_NoNotification(t *testing.T) {
	app := newTestAppWithHub(t)
	app.evictionRegistry = server.NewEvictionRegistry()
	app.engine.SetPhase(game.PhaseEnroll)

	aliceID, _, err := app.engine.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer(Alice) failed: %v", err)
	}

	baseURL := startEvictionTestServer(t, app)
	aliceConn := dialWS(t, baseURL, "/ws/player")
	aliceClientID := learnClientID(t, app, aliceConn)
	app.wsHub.SetClientPlayerID(aliceClientID, aliceID)

	// Same roster, Alice still present (e.g. an unrelated ANSWER_COLOR edit).
	sameRoster := map[string]*game.Bumper{
		aliceID: {Name: "Alice", IsVirtual: true, IsVPlayer: true, Connected: true},
	}
	app.handleUpdate(updateBumpersMsg(t, sameRoster))

	actions := collectActions(aliceConn, 300*time.Millisecond)
	if containsAction(actions, protocol.ActionPlayerEvicted) {
		t.Errorf("an UPDATE that removes nobody must never emit PLAYER_EVICTED, got actions: %v", actions)
	}
}

// ---------------------------------------------------------------------------
// B3 — le registre de motifs répond à un PLAYER_CONNECT portant un ID connu
// comme supprimé, au lieu de laisser deviner ENROLLMENT_CLOSED.
// ---------------------------------------------------------------------------

// TestHandlePlayerConnect_StaleIDRecentlyDeleted_RejectedWithPlayerRemoved is
// the exact bug scenario: a VJoueur is deleted by the admin while the game is
// in progress (enrollment closed), and later tries to reconnect with the
// now-stale ID. Before B3, ReconnectOrCreateVirtualPlayer falls through to a
// brand-new enrollment attempt (the name is free again) which is rejected
// with a generic ENROLLMENT_CLOSED — the registry must intercept this and
// answer with the real reason instead.
func TestHandlePlayerConnect_StaleIDRecentlyDeleted_RejectedWithPlayerRemoved(t *testing.T) {
	app := newTestAppWithHub(t)
	app.evictionRegistry = server.NewEvictionRegistry()
	app.engine.SetPhase(game.PhaseEnroll)

	aliceID, _, err := app.engine.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer(Alice) failed: %v", err)
	}
	app.engine.SetPhase(game.PhaseStarted) // the game is now in progress, enrollment closed

	app.handleDeleteBumper(deleteBumperMsg(t, aliceID)) // admin removes her mid-game

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/player")
	clientID := learnClientID(t, app, conn)

	// Alice's phone (offline at the time of the deletion) comes back and
	// tries to reconnect with her old, now-stale ID.
	app.handlePlayerConnect(clientID, playerConnectMsg(t, "Alice", aliceID))

	action, rawMsg := readAction(t, conn)
	if action != protocol.ActionPlayerRejected {
		t.Fatalf("expected %s, got %q", protocol.ActionPlayerRejected, action)
	}
	var payload protocol.PlayerRejectedPayload
	if err := json.Unmarshal(rawMsg, &payload); err != nil {
		t.Fatalf("failed to unmarshal PLAYER_REJECTED payload: %v", err)
	}
	if payload.Reason != "PLAYER_REMOVED" {
		t.Errorf("expected REASON=PLAYER_REMOVED (from the eviction registry), got %q — the ENROLLMENT_CLOSED guess must not resurface here", payload.Reason)
	}
}

// TestHandlePlayerConnect_StaleIDFromNewGamePurge_RejectedWithGameReset is
// the GAME_RESET counterpart: a VJoueur purged by NEW_GAME (while offline)
// reconnecting later with the stale ID must learn the real reason.
func TestHandlePlayerConnect_StaleIDFromNewGamePurge_RejectedWithGameReset(t *testing.T) {
	app := newTestAppWithHub(t)
	app.evictionRegistry = server.NewEvictionRegistry()
	app.logger = server.NewBroadcastLogger(100) // handleWebMessage's NEW_GAME case logs via a.logger
	app.engine.SetPhase(game.PhaseEnroll)

	aliceID, _, err := app.engine.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer(Alice) failed: %v", err)
	}

	// Alice is offline when NEW_GAME purges the whole VJoueur roster.
	app.handleWebMessage(&protocol.IncomingMessage{ClientID: "admin-client", Data: newGameMsg(t)})
	app.engine.SetPhase(game.PhaseStarted) // a fresh game has since started, enrollment closed

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/player")
	clientID := learnClientID(t, app, conn)

	app.handlePlayerConnect(clientID, playerConnectMsg(t, "Alice", aliceID))

	action, rawMsg := readAction(t, conn)
	if action != protocol.ActionPlayerRejected {
		t.Fatalf("expected %s, got %q", protocol.ActionPlayerRejected, action)
	}
	var payload protocol.PlayerRejectedPayload
	if err := json.Unmarshal(rawMsg, &payload); err != nil {
		t.Fatalf("failed to unmarshal PLAYER_REJECTED payload: %v", err)
	}
	if payload.Reason != "GAME_RESET" {
		t.Errorf("expected REASON=GAME_RESET (from the eviction registry), got %q", payload.Reason)
	}
}

// TestHandlePlayerConnect_UnknownIDNoRegistryEntry_UnchangedBehavior is the
// non-regression counterpart: an ID that was never a real bumper (hence
// never recorded in the registry either) must still fall through to the
// EXISTING behavior — ENROLLMENT_CLOSED here, since the game is in progress.
func TestHandlePlayerConnect_UnknownIDNoRegistryEntry_UnchangedBehavior(t *testing.T) {
	app := newTestAppWithHub(t)
	app.evictionRegistry = server.NewEvictionRegistry()
	app.engine.SetPhase(game.PhaseStarted) // enrollment closed, no prior enrollment at all

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/player")
	clientID := learnClientID(t, app, conn)

	app.handlePlayerConnect(clientID, playerConnectMsg(t, "NeverEnrolled", "vjoueur_never_existed"))

	action, rawMsg := readAction(t, conn)
	if action != protocol.ActionPlayerRejected {
		t.Fatalf("expected %s, got %q", protocol.ActionPlayerRejected, action)
	}
	var payload protocol.PlayerRejectedPayload
	if err := json.Unmarshal(rawMsg, &payload); err != nil {
		t.Fatalf("failed to unmarshal PLAYER_REJECTED payload: %v", err)
	}
	if payload.Reason != "ENROLLMENT_CLOSED" {
		t.Errorf("expected unchanged behavior (REASON=ENROLLMENT_CLOSED) for an ID with no registry entry, got %q", payload.Reason)
	}
}

// TestHandlePlayerConnect_NilEvictionRegistry_DoesNotPanic guards against a
// nil a.evictionRegistry (e.g. an App built without calling init(), as many
// existing unit tests do) — EvictionRegistry's methods are documented
// nil-safe, but the CALL SITE in handlePlayerConnect must actually rely on
// that rather than skip the lookup only when non-nil in a way that could
// still panic on a typed-nil method call.
func TestHandlePlayerConnect_NilEvictionRegistry_DoesNotPanic(t *testing.T) {
	app := newTestAppWithHub(t) // evictionRegistry left nil, like most existing tests
	app.engine.SetPhase(game.PhaseStarted)

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/player")
	clientID := learnClientID(t, app, conn)

	app.handlePlayerConnect(clientID, playerConnectMsg(t, "Someone", "vjoueur_unknown"))

	action, _ := readAction(t, conn)
	if action != protocol.ActionPlayerRejected {
		t.Fatalf("expected %s even with a nil eviction registry, got %q", protocol.ActionPlayerRejected, action)
	}
}
