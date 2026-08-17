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
// Tests: #170 — AWARDED_TEAMS diffusion (T2), through the REAL dispatch path
// (handleWebMessage), same discipline as next_question_triggers_test.go for
// NEXT_QUESTION: the pure projection (getAwardedTeamsPayload) is covered in
// awarded_teams_test.go, this file exercises the six trigger call sites +
// the two negative guarantees the contract makes explicit (never from a
// generic broadcast path, animateur-exclusive targeting).
// ---------------------------------------------------------------------------

func newAwardedTeamsTriggerTestApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppWithHub(t)
	app.logger = server.NewBroadcastLogger(100)
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.SetTeams(map[string]*game.Team{"Les Rouges": {Name: "Les Rouges"}})
	app.config.Storage.QuestionsDir = t.TempDir()
	return app
}

func readAwardedTeams(t *testing.T, conn interface {
	SetReadDeadline(time.Time) error
	ReadMessage() (int, []byte, error)
}) protocol.AwardedTeamsPayload {
	t.Helper()
	_, raw := readActionMatching(t, conn, protocol.ActionAwardedTeams)
	var envelope struct {
		Msg protocol.AwardedTeamsPayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("failed to unmarshal AWARDED_TEAMS: %v (raw: %s)", err, raw)
	}
	return envelope.Msg
}

// --- Les six déclencheurs du contrat ---------------------------------------

func TestAwardedTeams_TeamPointsTriggersBroadcast(t *testing.T) {
	app := newAwardedTeamsTriggerTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	app.engine.Ready("1", &game.Question{ID: "1"})

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Rouges", Points: 10})

	got := readAwardedTeams(t, anim)
	if len(got.Teams) != 1 || got.Teams[0].Team != "Les Rouges" || got.Teams[0].Points != 10 {
		t.Errorf("expected Les Rouges/10 after TEAM_POINTS, got %+v", got)
	}
}

func TestAwardedTeams_BumperPointsTriggersBroadcast(t *testing.T) {
	app := newAwardedTeamsTriggerTestApp(t)
	app.engine.UpdateBumper("bumper-1", map[string]interface{}{"TEAM": "Les Rouges"})
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	app.engine.Ready("1", &game.Question{ID: "1"})

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionBumperPoints, protocol.BumperPointsPayload{ID: "bumper-1", Points: 5})

	got := readAwardedTeams(t, anim)
	if len(got.Teams) != 1 || got.Teams[0].Team != "Les Rouges" || got.Teams[0].Points != 5 {
		t.Errorf("expected Les Rouges/5 after BUMPER_POINTS (grouped by team), got %+v", got)
	}
}

func TestAwardedTeams_ReadyTriggersBroadcast(t *testing.T) {
	app := newAwardedTeamsTriggerTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionReady, protocol.ReadyPayload{Question: "1"})

	got := readAwardedTeams(t, anim)
	if got.QuestionID != "1" {
		t.Errorf("expected QUESTION_ID=1 after READY, got %+v", got)
	}
	if len(got.Teams) != 0 {
		t.Errorf("expected a fresh (empty) lock state on READY, got %+v", got.Teams)
	}
}

func TestAwardedTeams_NewGameTriggersBroadcast(t *testing.T) {
	app := newAwardedTeamsTriggerTestApp(t)
	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionNewGame, struct{}{})

	got := readAwardedTeams(t, anim)
	if got.QuestionID != "" || len(got.Teams) != 0 {
		t.Errorf("expected an empty AWARDED_TEAMS after NEW_GAME (history cleared), got %+v", got)
	}
}

func TestAwardedTeams_RAZTriggersBroadcast(t *testing.T) {
	app := newAwardedTeamsTriggerTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	app.engine.Ready("1", &game.Question{ID: "1"})
	app.engine.StartImmediate(0)
	app.engine.AddGameEvent(game.GameEvent{
		Timestamp: app.engine.GetState().GameTime + 1000, QuestionID: "1",
		EventType: "POINTS_AWARDED", TeamName: "Les Rouges", Points: 10,
	})

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionRAZ, struct{}{})

	got := readAwardedTeams(t, anim)
	if len(got.Teams) != 0 {
		t.Errorf("expected locks released after RAZ (RAZScores clears history), got %+v", got.Teams)
	}
}

func TestAwardedTeams_HelloTargetsConnectingAnimClient(t *testing.T) {
	app := newAwardedTeamsTriggerTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	app.engine.Ready("1", &game.Question{ID: "1"})
	app.engine.StartImmediate(0)
	app.engine.AddGameEvent(game.GameEvent{
		Timestamp: app.engine.GetState().GameTime + 1000, QuestionID: "1",
		EventType: "POINTS_AWARDED", TeamName: "Les Rouges", Points: 10,
	})

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	clientID := learnClientID(t, app, anim)

	app.sendStateToClient(clientID, server.ClientTypeAnim)

	got := readAwardedTeams(t, anim)
	if len(got.Teams) != 1 || got.Teams[0].Team != "Les Rouges" {
		t.Errorf("expected a freshly-connecting anim client to receive the current lock state immediately, got %+v", got)
	}
}

// --- Garanties négatives du contrat -----------------------------------------

// TestAwardedTeams_NeverFromGenericBroadcastPath guards the "never from
// broadcastUpdate" rule: PAUSE broadcasts state to /ws/anim among others
// (ActionPause, via broadcastPauseAll) WITHOUT being one of the six
// AWARDED_TEAMS triggers — same discipline, same reasoning as NEXT_QUESTION's
// own restriction (a full history scan on every generic state push would be
// a cost that grows with the game).
func TestAwardedTeams_NeverFromGenericBroadcastPath(t *testing.T) {
	app := newAwardedTeamsTriggerTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	app.engine.Ready("1", &game.Question{ID: "1"})
	app.engine.StartImmediate(0)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionPause, struct{}{})

	anim.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	for {
		_, data, err := anim.ReadMessage()
		if err != nil {
			return // timed out without ever seeing AWARDED_TEAMS — expected
		}
		var envelope struct {
			Action string `json:"ACTION"`
		}
		if json.Unmarshal(data, &envelope) == nil && envelope.Action == protocol.ActionAwardedTeams {
			t.Fatal("#170: PAUSE (generic broadcast path) must not also trigger AWARDED_TEAMS")
		}
	}
}

// TestAwardedTeams_OnlyReachesAnim confirms AWARDED_TEAMS is never sent to
// admin/TV/VPlayer — contracts/websocket-endpoints.md exclusivity, same
// pattern as TestBroadcastNextQuestion_OnlyReachesAnim.
func TestAwardedTeams_OnlyReachesAnim(t *testing.T) {
	app := newAwardedTeamsTriggerTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	app.engine.Ready("1", &game.Question{ID: "1"})

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	tv := dialWS(t, baseURL, "/ws/tv")
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, admin)
	learnClientID(t, app, tv)
	learnClientID(t, app, anim)

	app.broadcastAwardedTeams()

	readAwardedTeams(t, anim)

	admin.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, data, err := admin.ReadMessage(); err == nil {
		var envelope struct {
			Action string `json:"ACTION"`
		}
		if json.Unmarshal(data, &envelope) == nil && envelope.Action == protocol.ActionAwardedTeams {
			t.Error("admin must never receive AWARDED_TEAMS")
		}
	}
	tv.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, data, err := tv.ReadMessage(); err == nil {
		var envelope struct {
			Action string `json:"ACTION"`
		}
		if json.Unmarshal(data, &envelope) == nil && envelope.Action == protocol.ActionAwardedTeams {
			t.Error("TV must never receive AWARDED_TEAMS")
		}
	}
}
