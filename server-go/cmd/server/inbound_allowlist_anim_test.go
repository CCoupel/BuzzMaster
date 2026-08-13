package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests: #155/#156 (v6.2.0) — Interface Animateur, tâches B3 (allow-list) et
// B6 (log régie), exercised end-to-end through handleWebMessage.
//
// Harness: same conventions as cmd/server/inbound_allowlist_test.go
// (startEvictionTestServer/dialWS/learnClientID/sendAction), but that
// helper's routing switch (player_evicted_test.go) predates #155 and has no
// "/anim" case — it would silently fall through to its VPlayer default.
// startAnimAllowlistTestServer below is a local, additive twin (not a
// modification of the existing #154 helper) that adds the anim route.
// ---------------------------------------------------------------------------

// startAnimAllowlistTestServer is startEvictionTestServer's twin, extended
// with a "/anim" suffix routed to ClientTypeAnim (#155). Kept local to this
// file rather than editing player_evicted_test.go's existing helper, per the
// non-regression rule (existing test infra is left alone unless the
// CHANGELOG documents a behavior change it must track).
func startAnimAllowlistTestServer(t *testing.T, app *App) string {
	t.Helper()
	go app.wsHub.Run()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientType := server.ClientTypeVPlayer
		switch {
		case strings.HasSuffix(r.URL.Path, "/admin"):
			clientType = server.ClientTypeAdmin
		case strings.HasSuffix(r.URL.Path, "/tv"):
			clientType = server.ClientTypeTV
		case strings.HasSuffix(r.URL.Path, "/anim"):
			clientType = server.ClientTypeAnim
		}
		app.wsHub.HandleConnectionWithType(w, r, clientType)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// newAnimTestApp is newTestAppWithHub extended with the logger/udpBcast
// every "conduite" handler in handleWebMessage's switch unconditionally
// dereferences (a.logger.Info, a.broadcastX() -> a.broadcast(..., viaTCP:
// true) -> a.udpBcast.Broadcast) — same setup already used by
// TestHandleWebMessage_AllowList_VPlayerCanPong/Button for the same reason.
func newAnimTestApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppWithHub(t)
	app.logger = server.NewBroadcastLogger(100)
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	return app
}

// TestHandleWebMessage_Anim_CanSendConduiteActions is B3's core positive
// case: the "conduite en direct" périmètre (contracts/websocket-actions.md)
// — START/STOP/PAUSE/CONTINUE/REVEAL/READY/BUMPER_POINTS/TEAM_POINTS — sent
// from a /ws/anim connection must actually execute, exactly as if sent by
// admin.
func TestHandleWebMessage_Anim_CanSendConduiteActions(t *testing.T) {
	type step struct {
		name     string
		action   string
		payload  interface{}
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
			name:     "PAUSE",
			action:   protocol.ActionPause,
			payload:  struct{}{},
			setup:    func(t *testing.T, app *App) { app.engine.SetPhase(game.PhaseStarted) },
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetState().Phase },
		},
		{
			name:     "CONTINUE",
			action:   protocol.ActionContinue,
			payload:  struct{}{},
			setup:    func(t *testing.T, app *App) { app.engine.SetPhase(game.PhasePaused) },
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetState().Phase },
		},
		{
			name:     "REVEAL",
			action:   protocol.ActionReveal,
			payload:  struct{}{},
			setup:    func(t *testing.T, app *App) { app.engine.SetPhase(game.PhaseStopped) },
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetState().Phase },
		},
		{
			name:     "READY",
			action:   protocol.ActionReady,
			payload:  protocol.ReadyPayload{Question: ""},
			setup:    func(t *testing.T, app *App) { app.engine.SetPhase(game.PhaseStopped) },
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetState().Phase },
		},
		{
			name:    "BUMPER_POINTS",
			action:  protocol.ActionBumperPoints,
			payload: protocol.BumperPointsPayload{ID: "bumper-anim-pts", Points: 25},
			setup: func(t *testing.T, app *App) {
				app.engine.UpdateBumper("bumper-anim-pts", map[string]interface{}{"TEAM": "TeamA"})
			},
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetBumper("bumper-anim-pts").Score },
		},
		{
			name:     "TEAM_POINTS",
			action:   protocol.ActionTeamPoints,
			payload:  protocol.TeamPointsPayload{Team: "TeamA", Points: 15},
			setup:    func(t *testing.T, app *App) {},
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetTeam("TeamA").Score },
		},
	}

	for _, s := range steps {
		s := s
		t.Run(s.name, func(t *testing.T) {
			app := newAnimTestApp(t)
			s.setup(t, app)
			before := s.snapshot(t, app)

			baseURL := startAnimAllowlistTestServer(t, app)
			conn := dialWS(t, baseURL, "/ws/anim")
			learnClientID(t, app, conn)

			sendAction(t, app, conn, s.action, s.payload)

			after := s.snapshot(t, app)
			if before == after {
				t.Errorf("#155/#156 B3: %s from /ws/anim should have executed — state unchanged (before=after=%v)", s.name, before)
			}
		})
	}
}

// TestHandleWebMessage_Anim_RegieActionsRejectedAndLogged is B3's negative
// case + B3's own acceptance criterion ("rejected and logged"): régie-only
// actions sent from /ws/anim must have NO effect, and must produce the
// existing #154 LogWarn rejection entry (handleWebMessage's allow-list gate,
// main.go ~985) — this is the SAME mechanism #154 already established for
// TV/VPlayer, simply exercised for the new anim type.
func TestHandleWebMessage_Anim_RegieActionsRejectedAndLogged(t *testing.T) {
	type step struct {
		name     string
		action   string
		payload  interface{}
		setup    func(t *testing.T, app *App)
		snapshot func(t *testing.T, app *App) interface{}
	}

	steps := []step{
		{
			name:     "NEW_GAME",
			action:   protocol.ActionNewGame,
			payload:  struct{}{},
			setup:    func(t *testing.T, app *App) { app.engine.SetPhase(game.PhaseStopped) },
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetState().Phase },
		},
		{
			name:    "DELETE_BUMPER",
			action:  protocol.ActionDeleteBumper,
			payload: protocol.DeletePayload{ID: "bumper-anim-del"},
			setup: func(t *testing.T, app *App) {
				app.engine.UpdateBumper("bumper-anim-del", map[string]interface{}{"TEAM": "TeamA"})
			},
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetBumper("bumper-anim-del") != nil },
		},
		{
			name:     "RAZ",
			action:   protocol.ActionRAZ,
			payload:  struct{}{},
			setup:    func(t *testing.T, app *App) { app.engine.UpdateTeamScore("TeamA", 50) },
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetTeam("TeamA").Score },
		},
		{
			name:     "REMOTE",
			action:   protocol.ActionRemote,
			payload:  protocol.RemotePayload{Remote: "SCORE"},
			setup:    func(t *testing.T, app *App) { app.engine.SetPage("GAME") },
			snapshot: func(t *testing.T, app *App) interface{} { return app.engine.GetState().Page },
		},
	}

	for _, s := range steps {
		s := s
		t.Run(s.name, func(t *testing.T) {
			app := newAnimTestApp(t)
			s.setup(t, app)
			before := s.snapshot(t, app)

			baseURL := startAnimAllowlistTestServer(t, app)
			conn := dialWS(t, baseURL, "/ws/anim")
			clientID := learnClientID(t, app, conn)

			sendAction(t, app, conn, s.action, s.payload)

			after := s.snapshot(t, app)
			if before != after {
				t.Errorf("#155/#156 B3: %s from /ws/anim must have NO effect — state changed from %v to %v", s.name, before, after)
			}

			found := false
			for _, entry := range app.logger.GetRecent(50) {
				if entry.Level == game.LogLevelWarn &&
					strings.Contains(entry.Message, s.action) &&
					strings.Contains(entry.Message, clientID) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("#155/#156 B3: expected a WARN log entry rejecting %s from client %s (type=anim) — none found in recent logs", s.action, clientID)
			}
		})
	}
}

// TestHandleWebMessage_Anim_SetClientTypeRejected mirrors #154 E3's TV/
// VPlayer coverage for the new anim type: a /ws/anim connection (fixed type
// at connection time) has no legitimate reason to send SET_CLIENT_TYPE.
func TestHandleWebMessage_Anim_SetClientTypeRejected(t *testing.T) {
	app := newAnimTestApp(t)
	baseURL := startAnimAllowlistTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, conn)

	adminBefore, _, _, animBefore := app.wsHub.GetClientCounts()
	if adminBefore != 0 || animBefore != 1 {
		t.Fatalf("setup failed: expected admin=0 anim=1, got admin=%d anim=%d", adminBefore, animBefore)
	}

	sendAction(t, app, conn, protocol.ActionSetClientType, protocol.SetClientTypePayload{Type: "admin"})

	adminAfter, _, _, animAfter := app.wsHub.GetClientCounts()
	if adminAfter != 0 || animAfter != 1 {
		t.Errorf("#155/#156: anim must not self-promote to admin via SET_CLIENT_TYPE — got admin=%d anim=%d, want admin=0 anim=1", adminAfter, animAfter)
	}
}

// TestHandleWebMessage_Anim_AllowedActionLogsRegieNotice is B6: any action
// that DOES pass the allow-list from /ws/anim must produce a régie-visible
// LogInfo entry (main.go, "juste après le contrôle allow-list") — distinct
// from the LogWarn rejection entry above, and combined with ANIM_COUNT (B2)
// this is the whole v1 régie signalement (D2), without any new admin UI.
func TestHandleWebMessage_Anim_AllowedActionLogsRegieNotice(t *testing.T) {
	app := newAnimTestApp(t)
	app.engine.SetPhase(game.PhaseStarted)

	baseURL := startAnimAllowlistTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/anim")
	clientID := learnClientID(t, app, conn)

	sendAction(t, app, conn, protocol.ActionStop, struct{}{})

	found := false
	for _, entry := range app.logger.GetRecent(50) {
		if entry.Level == game.LogLevelInfo &&
			strings.Contains(entry.Message, protocol.ActionStop) &&
			strings.Contains(entry.Message, clientID) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("#155/#156 B6: expected an INFO log entry noting STOP triggered from the animateur interface (client %s) — none found in recent logs", clientID)
	}
}

// TestHandleWebMessage_Anim_RejectedActionDoesNotAlsoLogRegieNotice guards
// the ordering B6 depends on: the régie LogInfo notice fires only AFTER the
// allow-list check passes (main.go, "juste après le contrôle allow-list") —
// a rejected action must produce the WARN rejection entry only, never an
// additional INFO "triggered from animateur" entry for an action that never
// actually ran.
func TestHandleWebMessage_Anim_RejectedActionDoesNotAlsoLogRegieNotice(t *testing.T) {
	app := newAnimTestApp(t)
	app.engine.SetPhase(game.PhaseStopped)

	baseURL := startAnimAllowlistTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/anim")
	clientID := learnClientID(t, app, conn)

	sendAction(t, app, conn, protocol.ActionNewGame, struct{}{})

	for _, entry := range app.logger.GetRecent(50) {
		if entry.Level == game.LogLevelInfo &&
			strings.Contains(entry.Message, protocol.ActionNewGame) &&
			strings.Contains(entry.Message, clientID) {
			t.Errorf("#155/#156 B6: rejected NEW_GAME must not also produce a régie INFO notice as if it had executed — got: %q", entry.Message)
		}
	}
}
