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
// Tests : RELEASE_BUMPER_NAME sur un VJoueur CONNECTÉ (#134)
//
// Contrat : contracts/seat-release.md. Plan : _work/reports/planner-20260804-115318.md
// (T2.2, T2.5). CA1-CA3, CA5, CA7 ici — CA4/CA6/CA11 côté moteur, voir
// internal/game/reconnect_id_test.go.
//
// Réutilise releaseBumperNameMsg (name_recovery_test.go, même package) et le
// harnais WebSocket réel de player_evicted_test.go (startEvictionTestServer,
// dialWS, learnClientID, readAction, collectActions, containsAction).
//
// Piège de harnais (déjà rencontré en #129, documenté dans le plan) : une
// *websocket.Conn gorilla devient définitivement inutilisable en lecture
// après un premier timeout — chaque connexion n'est lue qu'UNE seule fois
// ci-dessous.
// ---------------------------------------------------------------------------

// TestHandleReleaseBumperName_Disconnected_NoPlayerEvicted is CA1 at the
// handler level (engine-level non-regression already covered by
// internal/game/reconnect_id_test.go and cmd/server/name_recovery_test.go):
// releasing a DISCONNECTED bumper must never emit PLAYER_EVICTED to anyone
// — the #122 case has no live client to notify in the first place.
func TestHandleReleaseBumperName_Disconnected_NoPlayerEvicted(t *testing.T) {
	app := newTestAppWithHub(t)
	app.evictionRegistry = server.NewEvictionRegistry()
	app.engine.SetPhase(game.PhaseEnroll)

	id, _, err := app.engine.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	app.engine.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})

	baseURL := startEvictionTestServer(t, app)
	bystanderConn := dialWS(t, baseURL, "/ws/player")

	app.handleReleaseBumperName(releaseBumperNameMsg(t, id))

	actions := collectActions(bystanderConn, 300*time.Millisecond)
	if containsAction(actions, protocol.ActionPlayerEvicted) {
		t.Errorf("releasing a disconnected bumper must never emit PLAYER_EVICTED, got actions: %v", actions)
	}

	// #122 unchanged: the bumper still resolves under its ORIGINAL id.
	if app.engine.GetBumper(id) == nil {
		t.Errorf("expected the bumper to still exist under its original id %q — a disconnected release never re-keys", id)
	}
}

// TestHandleReleaseBumperName_Connected_EmitsPlayerEvictedToTargetOnly is
// CA2/CA3: releasing a CONNECTED bumper's seat must notify THAT VJoueur's
// own client — via its OLD PlayerID, which is exactly what CA3's ordering
// (notify before re-key) exists to guarantee stays resolvable — and only
// it; an unrelated bystander VJoueur must receive nothing of the kind.
func TestHandleReleaseBumperName_Connected_EmitsPlayerEvictedToTargetOnly(t *testing.T) {
	app := newTestAppWithHub(t)
	app.evictionRegistry = server.NewEvictionRegistry()
	app.engine.SetPhase(game.PhaseEnroll)

	aliceID, _, err := app.engine.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer(Alice) failed: %v", err)
	}
	app.engine.UpdateBumper(aliceID, map[string]interface{}{"CONNECTED": true})

	baseURL := startEvictionTestServer(t, app)

	targetConn := dialWS(t, baseURL, "/ws/player")
	targetClientID := learnClientID(t, app, targetConn)
	app.wsHub.SetClientPlayerID(targetClientID, aliceID)

	bystanderConn := dialWS(t, baseURL, "/ws/player")

	app.handleReleaseBumperName(releaseBumperNameMsg(t, aliceID))

	action, rawMsg := readAction(t, targetConn)
	if action != protocol.ActionPlayerEvicted {
		t.Fatalf("expected the released VJoueur's client to receive %s, got %q", protocol.ActionPlayerEvicted, action)
	}
	var payload protocol.PlayerEvictedPayload
	if err := json.Unmarshal(rawMsg, &payload); err != nil {
		t.Fatalf("failed to unmarshal PLAYER_EVICTED payload: %v", err)
	}
	if payload.Reason != "SEAT_RELEASED" {
		t.Errorf("expected REASON=SEAT_RELEASED, got %q", payload.Reason)
	}

	bystanderActions := collectActions(bystanderConn, 300*time.Millisecond)
	if containsAction(bystanderActions, protocol.ActionPlayerEvicted) {
		t.Errorf("an unrelated VJoueur must never receive PLAYER_EVICTED, got actions: %v", bystanderActions)
	}
}

// TestHandleReleaseBumperName_Connected_SocketNeverClosed is CA7: the
// server must NOT close the evicted VJoueur's WebSocket connection — it
// stays open and keeps receiving broadcasts (mirrors handleDeleteBumper's
// own contract, same reasoning: PLAYER_EVICTED is queued on client.Send,
// closing immediately would risk losing it before writePump flushes it).
func TestHandleReleaseBumperName_Connected_SocketNeverClosed(t *testing.T) {
	app := newTestAppWithHub(t)
	app.evictionRegistry = server.NewEvictionRegistry()
	app.engine.SetPhase(game.PhaseEnroll)

	aliceID, _, err := app.engine.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer(Alice) failed: %v", err)
	}
	app.engine.UpdateBumper(aliceID, map[string]interface{}{"CONNECTED": true})

	baseURL := startEvictionTestServer(t, app)
	targetConn := dialWS(t, baseURL, "/ws/player")
	targetClientID := learnClientID(t, app, targetConn)
	app.wsHub.SetClientPlayerID(targetClientID, aliceID)

	app.handleReleaseBumperName(releaseBumperNameMsg(t, aliceID))

	// Drain the eviction notice plus the roster UPDATE that immediately
	// follows it (broadcastUpdate reaches every VPlayer, this connection
	// included) — a single collectActions call, per the harness's own
	// single-read-per-connection rule (no repeat drains on the same conn).
	actions := collectActions(targetConn, 500*time.Millisecond)
	if !containsAction(actions, protocol.ActionPlayerEvicted) {
		t.Fatalf("expected PLAYER_EVICTED among the traffic on the still-open socket, got: %v", actions)
	}
	// The mere fact that collectActions could read ANY message here (rather
	// than an immediate close error on the very first read) demonstrates the
	// server never force-closed the connection — a closed connection would
	// make every subsequent read fail instantly, collapsing this list to
	// whatever arrived before the close, not after.
	if !containsAction(actions, protocol.ActionUpdate) {
		t.Errorf("expected the connection to still be receiving broadcastUpdate traffic AFTER its own eviction, got: %v — a prematurely closed socket would prevent this", actions)
	}
}

// TestHandleReleaseBumperName_Connected_StaleIDRejectedWithSeatReleased is
// CA5: a PLAYER_CONNECT retried with the now-stale (pre-release) ID must be
// rejected with REASON=SEAT_RELEASED — sourced from the eviction registry —
// never silently reconnected and never a generic ENROLLMENT_CLOSED guess.
func TestHandleReleaseBumperName_Connected_StaleIDRejectedWithSeatReleased(t *testing.T) {
	app := newTestAppWithHub(t)
	app.evictionRegistry = server.NewEvictionRegistry()
	app.engine.SetPhase(game.PhaseEnroll)

	aliceID, _, err := app.engine.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer(Alice) failed: %v", err)
	}
	app.engine.UpdateBumper(aliceID, map[string]interface{}{"CONNECTED": true})
	app.engine.SetPhase(game.PhaseStarted) // enrollment closed — irrelevant here, but representative

	// ID timestamped to the second — wait past the boundary so the release
	// genuinely re-keys onto a DIFFERENT id, making aliceID truly stale
	// (same convention as internal/game/reconnect_id_test.go).
	time.Sleep(1100 * time.Millisecond)
	app.handleReleaseBumperName(releaseBumperNameMsg(t, aliceID))

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/player")
	clientID := learnClientID(t, app, conn)

	// Alice's phone (offline at the time of the release, or the
	// notification was simply lost) comes back with her old, now-stale ID.
	app.handlePlayerConnect(clientID, playerConnectMsg(t, "Alice", aliceID))

	action, rawMsg := readAction(t, conn)
	if action != protocol.ActionPlayerRejected {
		t.Fatalf("expected %s, got %q", protocol.ActionPlayerRejected, action)
	}
	var payload protocol.PlayerRejectedPayload
	if err := json.Unmarshal(rawMsg, &payload); err != nil {
		t.Fatalf("failed to unmarshal PLAYER_REJECTED payload: %v", err)
	}
	if payload.Reason != "SEAT_RELEASED" {
		t.Errorf("expected REASON=SEAT_RELEASED (from the eviction registry), got %q", payload.Reason)
	}
}

// TestHandleReleaseBumperName_UnknownID_NoPanicNoPlayerEvicted is a
// non-regression guard: an unknown/stale ID must behave exactly like
// ReleaseBumperName always did (logged and ignored), never emit
// PLAYER_EVICTED, and never panic — mirrors
// TestHandleReleaseBumperName_UnknownID_NoPanicNoBroadcastCrash in
// name_recovery_test.go, extended to also assert on PLAYER_EVICTED.
func TestHandleReleaseBumperName_UnknownID_NoPanicNoPlayerEvicted(t *testing.T) {
	app := newTestAppWithHub(t)
	app.evictionRegistry = server.NewEvictionRegistry()

	baseURL := startEvictionTestServer(t, app)
	bystanderConn := dialWS(t, baseURL, "/ws/player")

	app.handleReleaseBumperName(releaseBumperNameMsg(t, "does-not-exist"))

	actions := collectActions(bystanderConn, 300*time.Millisecond)
	if containsAction(actions, protocol.ActionPlayerEvicted) {
		t.Errorf("an unknown ID must never emit PLAYER_EVICTED, got actions: %v", actions)
	}
}
