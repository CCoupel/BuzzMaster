package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests : RELEASE_BUMPER_NAME — reprise de place assistée (#122, B3)
//
// Plan : _work/reports/plan-20260730-123000.md.
//
// The core reclaim logic (authorization, single-use, expiry, concurrency,
// score/team preservation) is exhaustively covered at the engine level in
// internal/game/name_recovery_test.go. This file is the thin integration
// layer: the real WS action (RELEASE_BUMPER_NAME) dispatched through
// handleReleaseBumperName, followed by a real PLAYER_CONNECT through
// handlePlayerConnect — end to end, the same way an admin click and a
// player's retry actually reach the server.
// ---------------------------------------------------------------------------

func releaseBumperNameMsg(t *testing.T, id string) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionReleaseBumperName, protocol.ReleaseBumperNamePayload{ID: id})
	if err != nil {
		t.Fatalf("failed to build RELEASE_BUMPER_NAME message: %v", err)
	}
	return msg
}

func TestHandleReleaseBumperName_AllowsSubsequentNamelessReclaim(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)

	id, _, err := app.engine.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	app.engine.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})

	baseURL := startEvictionTestServer(t, app)

	// The admin grants the release for Emma's bumper.
	app.handleReleaseBumperName(releaseBumperNameMsg(t, id))

	// Emma's phone (no stored ID — it lost it, that's the whole scenario)
	// retries her pseudo.
	conn := dialWS(t, baseURL, "/ws/player")
	clientID := learnClientID(t, app, conn)
	app.handlePlayerConnect(clientID, playerConnectMsg(t, "Emma", ""))

	action, rawMsg := readAction(t, conn)
	if action != protocol.ActionPlayerConnected {
		t.Fatalf("expected the reclaim to succeed end-to-end (%s), got %q", protocol.ActionPlayerConnected, action)
	}
	var payload protocol.PlayerConnectedPayload
	if err := json.Unmarshal(rawMsg, &payload); err != nil {
		t.Fatalf("failed to unmarshal PLAYER_CONNECTED payload: %v", err)
	}
	if payload.Name != "Emma" {
		t.Errorf("expected NAME=Emma, got %q", payload.Name)
	}
	if payload.ID == "" {
		t.Error("expected a non-empty bumper ID in PLAYER_CONNECTED")
	}
}

func TestHandlePlayerConnect_NoRelease_StillRejected(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)

	_, _, err := app.engine.CreateVirtualPlayer("Emma")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	// Emma's bumper is left Connected=true (CreateVirtualPlayer's default) —
	// this test is about the NO-RELEASE case, not the offline variant.

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/player")
	clientID := learnClientID(t, app, conn)

	// No RELEASE_BUMPER_NAME was ever sent — the #109 guarantee must hold.
	app.handlePlayerConnect(clientID, playerConnectMsg(t, "Emma", ""))

	action, rawMsg := readAction(t, conn)
	if action != protocol.ActionPlayerRejected {
		t.Fatalf("expected %s, got %q", protocol.ActionPlayerRejected, action)
	}
	var payload protocol.PlayerRejectedPayload
	if err := json.Unmarshal(rawMsg, &payload); err != nil {
		t.Fatalf("failed to unmarshal PLAYER_REJECTED payload: %v", err)
	}
	// Emma's own bumper is still Connected=true (CreateVirtualPlayer default)
	// at this point, so this is the ordinary (connected) NAME_TAKEN — not yet
	// NAME_TAKEN_OFFLINE. The offline variant itself is covered by
	// internal/game/reconnect_id_test.go; this test's point is narrower and
	// purely about the integration wiring: no release, no reattachment.
	if payload.Reason != "NAME_TAKEN" {
		t.Errorf("expected REASON=NAME_TAKEN (no release granted), got %q", payload.Reason)
	}
}

func TestHandleReleaseBumperName_UnknownID_NoPanicNoBroadcastCrash(t *testing.T) {
	app := newTestAppWithHub(t)

	// Must be a safe no-op: log a warning, nothing else.
	app.handleReleaseBumperName(releaseBumperNameMsg(t, "does-not-exist"))
}
