package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// #132 — audit of the 5 remaining sendLEDSet* functions for the same
// untargeted broadcastUpdate() pattern already found and fixed twice
// (sendLEDSetAllBuzzers #127 T1.6, sendLEDSetPause #129 T3.1-adjacent):
// broadcastLEDSet, sendLEDSetStop, sendLEDSetReveal, sendLEDSetToTeam,
// sendLEDSetComet.
//
// Report: _work/reports/dev-backend-132-audit-20260804.md.
//
// None of these 5 sit behind a single dedicated WS action handler the way
// #127/#129's fixes did — they're plain Go methods called directly (some,
// broadcastLEDSet/sendLEDSetToTeam, currently have ZERO call sites at all —
// see the audit report). Each is exercised directly here with real Admin +
// TV + VPlayer WebSocket connections: Admin must still receive the trailing
// UPDATE (ACK_PENDING/spinner UI), TV and VPlayer must receive NOTHING from
// that specific call — same CA10/CA12-class assertion used throughout
// #127/#129's own test suites.
//
// Single read per connection (collectActions), per the harness's own rule:
// a gorilla *websocket.Conn becomes unusable in read after any timeout.
// ---------------------------------------------------------------------------

// setupLEDBroadcast132TestApp wires a real Admin + TV + VPlayer connection
// and a small roster: one physical (non-virtual) bumper on TeamA (exercises
// the sendLEDSet loop; its firmware side is irrelevant here — no real
// buzzer needs to be connected for these assertions) and one VJoueur bumper
// also on TeamA, proving the loop's `if bumper.IsVPlayer { continue }` guard
// can never leak an unrelated UPDATE to that VJoueur's own connection.
func setupLEDBroadcast132TestApp(t *testing.T) (app *App, adminConn, tvConn, vplayerConn *websocket.Conn) {
	t.Helper()
	app = newTestAppWithHub(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}}})
	app.engine.SetBumpers(map[string]*game.Bumper{
		"AA:BB:CC:DD:EE:01": {Name: "Buzzer1", IsVirtual: false, Team: "TeamA", Connected: true},
	})
	app.engine.SetPhase(game.PhaseEnroll)
	vplayerID, _, err := app.engine.CreateVirtualPlayer("Vic")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	if err := app.engine.AssignVirtualPlayer(vplayerID, "TeamA", game.AnswerColorNone); err != nil {
		t.Fatalf("AssignVirtualPlayer failed: %v", err)
	}

	baseURL := startEvictionTestServer(t, app)
	adminConn = dialWS(t, baseURL, "/ws/admin")
	tvConn = dialWS(t, baseURL, "/ws/tv")
	vplayerConn = dialWS(t, baseURL, "/ws/player")
	vplayerClientID := learnClientID(t, app, vplayerConn)
	app.wsHub.SetClientPlayerID(vplayerClientID, vplayerID)

	return app, adminConn, tvConn, vplayerConn
}

// assertLEDBroadcast132Targeting is the shared assertion for every function
// below: Admin gets the UPDATE, TV and VPlayer get nothing.
func assertLEDBroadcast132Targeting(t *testing.T, fnName string, adminConn, tvConn, vplayerConn *websocket.Conn) {
	t.Helper()

	adminActions := collectActions(adminConn, 300*time.Millisecond)
	if !containsAction(adminActions, protocol.ActionUpdate) {
		t.Errorf("%s: expected Admin to receive %s, got: %v", fnName, protocol.ActionUpdate, adminActions)
	}

	tvActions := collectActions(tvConn, 300*time.Millisecond)
	if containsAction(tvActions, protocol.ActionUpdate) {
		t.Errorf("%s: TV must NOT receive an UPDATE from this call (ACK_PENDING is always stripped from its payload) — got: %v", fnName, tvActions)
	}

	vplayerActions := collectActions(vplayerConn, 300*time.Millisecond)
	if containsAction(vplayerActions, protocol.ActionUpdate) {
		t.Errorf("%s: VPlayer must NOT receive an UPDATE from this call (the loop always skips IsVPlayer bumpers) — got: %v", fnName, vplayerActions)
	}
}

func TestLEDBroadcast132_BroadcastLEDSet_TargetsAdminAndBuzzerOnly(t *testing.T) {
	app, adminConn, tvConn, vplayerConn := setupLEDBroadcast132TestApp(t)

	app.broadcastLEDSet(protocol.LEDSetPayload{Color: [3]int{255, 0, 0}, Intensity: 255, Effect: "SOLID"})

	assertLEDBroadcast132Targeting(t, "broadcastLEDSet", adminConn, tvConn, vplayerConn)
}

func TestLEDBroadcast132_SendLEDSetStop_TargetsAdminAndBuzzerOnly(t *testing.T) {
	app, adminConn, tvConn, vplayerConn := setupLEDBroadcast132TestApp(t)

	app.sendLEDSetStop()

	assertLEDBroadcast132Targeting(t, "sendLEDSetStop", adminConn, tvConn, vplayerConn)
}

func TestLEDBroadcast132_SendLEDSetReveal_TargetsAdminAndBuzzerOnly(t *testing.T) {
	app, adminConn, tvConn, vplayerConn := setupLEDBroadcast132TestApp(t)

	app.sendLEDSetReveal("RED")

	assertLEDBroadcast132Targeting(t, "sendLEDSetReveal", adminConn, tvConn, vplayerConn)
}

func TestLEDBroadcast132_SendLEDSetToTeam_TargetsAdminAndBuzzerOnly(t *testing.T) {
	app, adminConn, tvConn, vplayerConn := setupLEDBroadcast132TestApp(t)

	app.sendLEDSetToTeam("TeamA", protocol.LEDSetPayload{Color: [3]int{0, 255, 0}, Intensity: 255, Effect: "SOLID"})

	assertLEDBroadcast132Targeting(t, "sendLEDSetToTeam", adminConn, tvConn, vplayerConn)
}

func TestLEDBroadcast132_SendLEDSetComet_TargetsAdminAndBuzzerOnly(t *testing.T) {
	app, adminConn, tvConn, vplayerConn := setupLEDBroadcast132TestApp(t)

	app.sendLEDSetComet("TeamA")

	assertLEDBroadcast132Targeting(t, "sendLEDSetComet", adminConn, tvConn, vplayerConn)
}

// TestLEDBroadcast132_SendLEDSetToTeam_EmptyTeamID_StillTargetsAdminAndBuzzerOnly
// covers sendLEDSetToTeam's "no teamID -> all physical buzzers" branch —
// same targeting rule applies regardless of which buzzers were touched.
func TestLEDBroadcast132_SendLEDSetToTeam_EmptyTeamID_StillTargetsAdminAndBuzzerOnly(t *testing.T) {
	app, adminConn, tvConn, vplayerConn := setupLEDBroadcast132TestApp(t)

	app.sendLEDSetToTeam("", protocol.LEDSetPayload{Color: [3]int{0, 0, 255}, Intensity: 255, Effect: "SOLID"})

	assertLEDBroadcast132Targeting(t, "sendLEDSetToTeam(empty teamID)", adminConn, tvConn, vplayerConn)
}
