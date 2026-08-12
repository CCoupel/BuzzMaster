package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// Tests: #154 (sec) — inbound WebSocket allow-list by ClientType, exercised
// end-to-end through handleWebMessage.
//
// Before this fix, handleWebMessage never consulted the sending client's
// type at all: a client connected on /ws/tv or /ws/player (dedicated,
// supposedly reduced-capability endpoints) could send admin-only actions —
// START/STOP/RAZ/DELETE/NEW_GAME/BUMPER_POINTS/... — and the server executed
// them exactly as if an admin had sent them.
//
// Harness: same conventions as player_evicted_test.go
// (startEvictionTestServer/dialWS/learnClientID) — SendToClient-adjacent
// paths, and the allow-list's ClientType lookup, both require a genuinely
// registered hub client, which only a real WebSocket connection provides.
// No background goroutine drains app.wsHub.Incoming in this harness (unlike
// production's setupCallbacks loop) — sendAction below is both the sender
// and the dispatcher, same shape as player_connect_connstate_phase2_test.go's
// sendPong/readIncoming pair.
// ---------------------------------------------------------------------------

// sendAction writes a WS frame for action/payload on conn, then drains and
// dispatches the resulting IncomingMessage through handleWebMessage.
func sendAction(t *testing.T, app *App, conn *websocket.Conn, action string, payload interface{}) {
	t.Helper()
	msg, err := protocol.NewMessage(action, payload)
	if err != nil {
		t.Fatalf("failed to build %s message: %v", action, err)
	}
	data, err := msg.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("failed to serialize %s message: %v", action, err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to send %s: %v", action, err)
	}
	select {
	case incoming := <-app.wsHub.Incoming:
		app.handleWebMessage(incoming)
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s to reach the Incoming channel", action)
	}
}

// TestHandleWebMessage_AllowList_NonAdminCannotSendAdminActions is the core
// #154 regression test: for every action the acceptance criteria explicitly
// name (START/STOP/RAZ/DELETE_BUMPER/NEW_GAME/BUMPER_POINTS/TEAM_POINTS),
// sending it from a TV or VPlayer connection must have NO observable effect.
func TestHandleWebMessage_AllowList_NonAdminCannotSendAdminActions(t *testing.T) {
	type step struct {
		name    string
		action  string
		payload interface{}
		// assertNoEffect runs BEFORE and AFTER the action is sent; state must
		// be identical (the action must not have executed).
		setup    func(t *testing.T, app *App)
		snapshot func(t *testing.T, app *App) interface{}
	}

	steps := []step{
		{
			name:     "START",
			action:   protocol.ActionStart,
			payload:  protocol.StartPayload{Delay: 1},
			setup:    func(t *testing.T, app *App) { app.engine.SetPhase(game.PhaseReady) },
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetState().Phase },
		},
		{
			name:     "STOP",
			action:   protocol.ActionStop,
			payload:  struct{}{},
			setup:    func(t *testing.T, app *App) { app.engine.SetPhase(game.PhaseStarted) },
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetState().Phase },
		},
		{
			name:    "RAZ",
			action:  protocol.ActionRAZ,
			payload: struct{}{},
			setup: func(t *testing.T, app *App) {
				app.engine.UpdateBumper("bumper-raz", map[string]interface{}{"TEAM": "TeamA"})
				app.engine.UpdateBumperScore("bumper-raz", 50)
			},
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetBumper("bumper-raz").Score },
		},
		{
			name:    "DELETE_BUMPER",
			action:  protocol.ActionDeleteBumper,
			payload: protocol.DeletePayload{ID: "bumper-del"},
			setup: func(t *testing.T, app *App) {
				app.engine.UpdateBumper("bumper-del", map[string]interface{}{"TEAM": "TeamA"})
			},
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetBumper("bumper-del") != nil },
		},
		{
			name:    "BUMPER_POINTS",
			action:  protocol.ActionBumperPoints,
			payload: protocol.BumperPointsPayload{ID: "bumper-pts", Points: 25},
			setup: func(t *testing.T, app *App) {
				app.engine.UpdateBumper("bumper-pts", map[string]interface{}{"TEAM": "TeamA"})
			},
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetBumper("bumper-pts").Score },
		},
		{
			name:     "TEAM_POINTS",
			action:   protocol.ActionTeamPoints,
			payload:  protocol.TeamPointsPayload{Team: "TeamA", Points: 15},
			setup:    func(t *testing.T, app *App) {},
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetTeam("TeamA").Score },
		},
		{
			name:     "NEW_GAME",
			action:   protocol.ActionNewGame,
			payload:  struct{}{},
			setup:    func(t *testing.T, app *App) { app.engine.SetPhase(game.PhaseStopped) },
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetState().Phase },
		},
	}

	for _, clientPath := range []string{"/ws/tv", "/ws/player"} {
		clientPath := clientPath
		for _, s := range steps {
			s := s
			t.Run(clientPath+"/"+s.name, func(t *testing.T) {
				app := newTestAppWithHub(t)
				app.logger = server.NewBroadcastLogger(100)
				app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
				s.setup(t, app)
				before := s.snapshot(t, app)

				baseURL := startEvictionTestServer(t, app)
				conn := dialWS(t, baseURL, clientPath)
				learnClientID(t, app, conn)

				sendAction(t, app, conn, s.action, s.payload)

				after := s.snapshot(t, app)
				if before != after {
					t.Errorf("#154: %s from %s must have NO effect — state changed from %v to %v", s.name, clientPath, before, after)
				}
			})
		}
	}
}

// TestHandleWebMessage_AllowList_AdminCanStillSendAdminActions is the
// control counterpart: the SAME actions must still work when genuinely sent
// by an admin — proving the allow-list rejects by client type, not by
// silently breaking the action itself.
func TestHandleWebMessage_AllowList_AdminCanStillSendAdminActions(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.UpdateBumper("bumper-pts", map[string]interface{}{"TEAM": "TeamA"})

	baseURL := startEvictionTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)

	sendAction(t, app, admin, protocol.ActionBumperPoints, protocol.BumperPointsPayload{ID: "bumper-pts", Points: 25})

	if got := app.engine.GetBumper("bumper-pts").Score; got != 25 {
		t.Errorf("BUMPER_POINTS from admin should still award points — got score %d, want 25", got)
	}
}

// TestHandleWebMessage_AllowList_TVCanFlipMemoryCard verifies the OTHER half
// of #154: the fix must not break legitimate reduced-capability actions.
// FLIP_MEMORY_CARD is deliberately allowed from TV — it carries both the
// admin preview iframe (/tv?admin=true, still a genuine ClientTypeTV
// connection) AND a real spectator's own click.
func TestHandleWebMessage_AllowList_TVCanFlipMemoryCard(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseStarted)

	baseURL := startEvictionTestServer(t, app)
	tv := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tv)

	// Card ID format is "pairID-cardNum" (game.Engine.FlipMemoryCard's
	// extractPairID) — no card content/loading needed to exercise the flip
	// itself, only a valid ID shape and PhaseStarted.
	sendAction(t, app, tv, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{CardID: "1-1"})

	flipped := app.engine.GetState().MemoryFlippedCards
	found := false
	for _, id := range flipped {
		if id == "1-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("#154: TV must still be able to send FLIP_MEMORY_CARD — MemoryFlippedCards=%v, want to contain 1-1", flipped)
	}
}

// TestHandleWebMessage_AllowList_VPlayerCanPong is the code-review CRITIQUE 1
// regression test: PONG is not admin-only debug tooling — VPlayerPage.jsx
// auto-sends it on entering PREPARE (the real VJoueur readiness handshake,
// handlePong -> SetBumperReady). Exercised through handleWebMessage over a
// real /ws/player connection (not a direct app.handlePong call) so the
// allow-list gate is actually on the path being tested.
func TestHandleWebMessage_AllowList_VPlayerCanPong(t *testing.T) {
	app := newTestAppWithHub(t)
	// handlePong -> broadcastReady -> a.broadcast() dereferences a.udpBcast
	// unconditionally — newTestAppWithHub leaves it nil (only wires the
	// WebSocket hub); an unstarted UDPBroadcaster's Broadcast() is a safe
	// no-op (conn == nil), same setup as main_broadcast_127_test.go's
	// newBroadcast127TestApp.
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.UpdateBumper("vplayer-1", map[string]interface{}{"TEAM": "TeamA"})
	app.engine.SetPhase(game.PhasePrepare)

	baseURL := startEvictionTestServer(t, app)
	vplayer := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vplayer)

	sendAction(t, app, vplayer, protocol.ActionPong, map[string]interface{}{"ID": "vplayer-1"})

	if got := app.engine.GetBumper("vplayer-1"); got == nil || !got.Ready {
		t.Errorf("#154 CRITIQUE 1: VPlayer must still be able to PONG (readiness handshake) — bumper=%+v", got)
	}
}

// TestHandleWebMessage_AllowList_VPlayerCanButton is the code-review
// CRITIQUE 1 regression test: BUTTON is not admin-only debug tooling —
// VPlayerPage.jsx's handleBuzz sends it directly for every real buzzer
// press. Exercised through handleWebMessage over a real /ws/player
// connection (not a direct app.handleSimulatedButton call).
func TestHandleWebMessage_AllowList_VPlayerCanButton(t *testing.T) {
	app := newTestAppWithHub(t)
	// See TestHandleWebMessage_AllowList_VPlayerCanPong's identical comment —
	// handleSimulatedButton -> broadcastPause (on first buzz) also needs a
	// non-nil (if unstarted) udpBcast.
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.UpdateBumper("vplayer-1", map[string]interface{}{"TEAM": "TeamA"})
	app.engine.SetPhase(game.PhaseStarted)

	baseURL := startEvictionTestServer(t, app)
	vplayer := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vplayer)

	// handleSimulatedButton reads ID from the MSG payload (not the
	// ClientID) — VPlayerPage.jsx:429/560 sends exactly this shape
	// (sendMessage('BUTTON', { ID: bumper.id, button: 'A' })).
	// protocol.ButtonPayload has no ID field (it's the buzzer-side payload
	// shape, ID comes from the TCP/WS connection identity there), so build
	// the MSG body as a map instead, same as the PONG test above.
	sendAction(t, app, vplayer, protocol.ActionButton, map[string]interface{}{"ID": "vplayer-1", "button": "A"})

	if got := app.engine.GetBumper("vplayer-1"); got == nil || got.Time == 0 {
		t.Errorf("#154 CRITIQUE 1: VPlayer must still be able to BUTTON (buzz) — bumper=%+v", got)
	}
}

// TestHandleWebMessage_AllowList_SetClientType_EscalationBlocked is #154 E3:
// a client connected on a DEDICATED endpoint (fixed type at connection) must
// not be able to self-promote via SET_CLIENT_TYPE.
func TestHandleWebMessage_AllowList_SetClientType_EscalationBlocked(t *testing.T) {
	app := newTestAppWithHub(t)
	baseURL := startEvictionTestServer(t, app)
	tv := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tv)

	adminBefore, tvBefore, _ := app.wsHub.GetClientCounts()
	if adminBefore != 0 || tvBefore != 1 {
		t.Fatalf("setup failed: expected admin=0 tv=1, got admin=%d tv=%d", adminBefore, tvBefore)
	}

	sendAction(t, app, tv, protocol.ActionSetClientType, protocol.SetClientTypePayload{Type: "admin"})

	adminAfter, tvAfter, _ := app.wsHub.GetClientCounts()
	if adminAfter != 0 || tvAfter != 1 {
		t.Errorf("#154 E3: TV must not self-promote to admin via SET_CLIENT_TYPE — got admin=%d tv=%d, want admin=0 tv=1", adminAfter, tvAfter)
	}
}

// TestHandleWebMessage_AllowList_SetClientType_LegacyHandshakeStillWorks
// proves the fix doesn't break the one legitimate SET_CLIENT_TYPE use: a
// connection whose CURRENT type is Admin (the legacy /ws default) declaring
// itself as a different type.
func TestHandleWebMessage_AllowList_SetClientType_LegacyHandshakeStillWorks(t *testing.T) {
	app := newTestAppWithHub(t)
	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/admin") // currentType == Admin
	learnClientID(t, app, conn)

	sendAction(t, app, conn, protocol.ActionSetClientType, protocol.SetClientTypePayload{Type: "tv"})

	adminAfter, tvAfter, _ := app.wsHub.GetClientCounts()
	if adminAfter != 0 || tvAfter != 1 {
		t.Errorf("SET_CLIENT_TYPE from an Admin-typed connection should still work — got admin=%d tv=%d, want admin=0 tv=1", adminAfter, tvAfter)
	}
}
