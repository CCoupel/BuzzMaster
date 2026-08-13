package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests: #155/#156 code review MAJEUR-1 — live sync of the admin's adjusted
// pointsInput to /anim (SET_CREDIT_POINTS / CREDIT_POINTS).
//
// Reviewer's exact scenario (_work/reports/code-reviewer-20260813-120457.md):
// a SPEEDY question at 10 points is selected (admin and anim both start at
// 10). The admin bumps pointsInput to 20 for a bonus round WITHOUT
// reselecting the question. Crediting from /admin now awards 20; crediting
// from /anim, before this fix, silently still awarded 10 (question.POINTS,
// read directly, never told about the adjustment). These tests exercise
// that exact sequence end-to-end.
// ---------------------------------------------------------------------------

func TestResetCreditPointsForQuestion_UsesQuestionPoints(t *testing.T) {
	app := newTestAppWithHub(t)
	app.logger = server.NewBroadcastLogger(100)

	app.resetCreditPointsForQuestion(&game.Question{ID: "1", Points: "10"})

	if app.currentCreditPoints != 10 {
		t.Errorf("currentCreditPoints = %d, want 10", app.currentCreditPoints)
	}
}

func TestResetCreditPointsForQuestion_FallsBackTo1WhenInvalid(t *testing.T) {
	app := newTestAppWithHub(t)
	app.logger = server.NewBroadcastLogger(100)

	for _, points := range []string{"", "not-a-number", "0", "-5"} {
		app.resetCreditPointsForQuestion(&game.Question{ID: "1", Points: points})
		if app.currentCreditPoints != 1 {
			t.Errorf("Points=%q: currentCreditPoints = %d, want 1 (GamePage.jsx's `|| 1` fallback)", points, app.currentCreditPoints)
		}
	}
}

func TestResetCreditPointsForQuestion_NilQuestionResetsToZero(t *testing.T) {
	app := newTestAppWithHub(t)
	app.logger = server.NewBroadcastLogger(100)
	app.currentCreditPoints = 42 // simulate a stale value from a previous question

	app.resetCreditPointsForQuestion(nil)

	if app.currentCreditPoints != 0 {
		t.Errorf("currentCreditPoints = %d, want 0 (no current question, nothing to credit)", app.currentCreditPoints)
	}
}

// TestHandleWebMessage_Anim_ReceivesCreditPointsOnReady mirrors the
// equivalent NEXT_QUESTION test — READY resets and broadcasts the credit
// amount, exercised through the real dispatch path.
func TestHandleWebMessage_Anim_ReceivesCreditPointsOnReady(t *testing.T) {
	app := newNextQuestionTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, map[string]interface{}{"POINTS": "10"})

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)

	sendAction(t, app, admin, protocol.ActionReady, protocol.ReadyPayload{Question: "1"})

	_, raw := readActionMatching(t, anim, protocol.ActionCreditPoints)
	var payload struct {
		Msg protocol.CreditPointsPayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("failed to unmarshal CREDIT_POINTS: %v (raw: %s)", err, raw)
	}
	if payload.Msg.Points != 10 {
		t.Errorf("expected CREDIT_POINTS=10 after READY (question.POINTS), got %+v", payload.Msg)
	}
}

// TestHandleWebMessage_Anim_ReceivesUpdatedCreditPointsAfterAdminAdjusts is
// the reviewer's exact scenario end-to-end: question selected at 10 points
// (anim confirms 10), admin bumps to 20 WITHOUT reselecting the question,
// anim must receive the updated 20 — the same amount /admin would now award.
func TestHandleWebMessage_Anim_ReceivesUpdatedCreditPointsAfterAdminAdjusts(t *testing.T) {
	app := newNextQuestionTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, map[string]interface{}{"POINTS": "10"})

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)

	sendAction(t, app, admin, protocol.ActionReady, protocol.ReadyPayload{Question: "1"})
	_, raw := readActionMatching(t, anim, protocol.ActionCreditPoints)
	var initial struct {
		Msg protocol.CreditPointsPayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &initial); err != nil {
		t.Fatalf("failed to unmarshal initial CREDIT_POINTS: %v", err)
	}
	if initial.Msg.Points != 10 {
		t.Fatalf("setup failed: expected initial CREDIT_POINTS=10, got %+v", initial.Msg)
	}

	// Admin bumps pointsInput to 20 for a bonus round — no question reselection.
	sendAction(t, app, admin, protocol.ActionSetCreditPoints, protocol.CreditPointsPayload{Points: 20})

	_, raw = readActionMatching(t, anim, protocol.ActionCreditPoints)
	var updated struct {
		Msg protocol.CreditPointsPayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &updated); err != nil {
		t.Fatalf("failed to unmarshal updated CREDIT_POINTS: %v", err)
	}
	if updated.Msg.Points != 20 {
		t.Errorf("#155/#156 MAJEUR-1: anim must receive the admin's adjusted amount (20), got %+v", updated.Msg)
	}

	if app.currentCreditPoints != 20 {
		t.Errorf("app.currentCreditPoints = %d, want 20 (the value /admin would now award)", app.currentCreditPoints)
	}
}

// TestHandleWebMessage_Anim_ReceivesCreditPointsOnHello is the HELLO
// integration case, mirroring NEXT_QUESTION's targeted send.
func TestHandleWebMessage_Anim_ReceivesCreditPointsOnHello(t *testing.T) {
	app := newNextQuestionTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, map[string]interface{}{"POINTS": "7"})
	question := &game.Question{ID: "1", Points: "7"}
	app.engine.Ready("1", question)
	// engine.Ready alone doesn't touch a.currentCreditPoints — that's
	// handleReady's job (main.go), not the engine's. Direct engine call here
	// (rather than going through handleReady) needs the same follow-up this
	// test is specifically about verifying at HELLO time.
	app.resetCreditPointsForQuestion(question)

	baseURL := startAnimAllowlistTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/anim")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeAnim)

	_, raw := readActionMatching(t, conn, protocol.ActionCreditPoints)
	var payload struct {
		Msg protocol.CreditPointsPayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("failed to unmarshal CREDIT_POINTS: %v (raw: %s)", err, raw)
	}
	if payload.Msg.Points != 7 {
		t.Errorf("expected CREDIT_POINTS=7 at HELLO, got %+v", payload.Msg)
	}
}

// TestHandleWebMessage_Anim_CannotSendSetCreditPoints is the allow-list
// negative case: only admin may send SET_CREDIT_POINTS — the animateur only
// ever receives CREDIT_POINTS, it never sets it.
func TestHandleWebMessage_Anim_CannotSendSetCreditPoints(t *testing.T) {
	app := newAnimTestApp(t)

	baseURL := startAnimAllowlistTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/anim")
	clientID := learnClientID(t, app, conn)

	before := app.currentCreditPoints
	sendAction(t, app, conn, protocol.ActionSetCreditPoints, protocol.CreditPointsPayload{Points: 99})

	if app.currentCreditPoints != before {
		t.Errorf("#155/#156 MAJEUR-1: anim must not be able to set credit points — currentCreditPoints changed from %d to %d", before, app.currentCreditPoints)
	}

	found := false
	for _, entry := range app.logger.GetRecent(50) {
		if entry.Level == game.LogLevelWarn &&
			strings.Contains(entry.Message, protocol.ActionSetCreditPoints) &&
			strings.Contains(entry.Message, clientID) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a WARN log entry rejecting SET_CREDIT_POINTS from client %s (type=anim)", clientID)
	}
}

// TestBroadcastCreditPoints_OnlyReachesAnim confirms CREDIT_POINTS is never
// sent to admin/TV/VPlayer — same exclusivity as NEXT_QUESTION.
func TestBroadcastCreditPoints_OnlyReachesAnim(t *testing.T) {
	app := newNextQuestionTestApp(t)
	app.currentCreditPoints = 5

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	tv := dialWS(t, baseURL, "/ws/tv")
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, admin)
	learnClientID(t, app, tv)
	learnClientID(t, app, anim)

	app.broadcastCreditPoints()

	readActionMatching(t, anim, protocol.ActionCreditPoints)

	for _, conn := range []struct {
		name string
		c    interface {
			SetReadDeadline(time.Time) error
			ReadMessage() (int, []byte, error)
		}
	}{{"admin", admin}, {"tv", tv}} {
		conn.c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		if _, data, err := conn.c.ReadMessage(); err == nil {
			var envelope struct {
				Action string `json:"ACTION"`
			}
			if json.Unmarshal(data, &envelope) == nil && envelope.Action == protocol.ActionCreditPoints {
				t.Errorf("%s must never receive CREDIT_POINTS", conn.name)
			}
		}
	}
}
