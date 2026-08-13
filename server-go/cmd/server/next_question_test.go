package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests: #155/#156 (v6.2.0) — NEXT_QUESTION action, tâche B5.
//
// getNextQuestionPayload is a deliberate, field-for-field port of
// GamePage.jsx's nextUnplayedQuestion (~line 215-224), NOT a reinvention —
// see that function's doc comment in main.go for the full rule and why the
// simplified "premier dont STATUS ∉ {...}" prose in the plan/contract is NOT
// what's implemented literally (it omits "after the CURRENT question" and
// "no current question -> no next at all", both real JS behavior). These
// tests are derived directly from reading GamePage.jsx, per the contract's
// own parity requirement (contracts/websocket-actions.md §"Animateur").
// ---------------------------------------------------------------------------

// writeQuestionFile writes a minimal question.json fixture under dir/id/,
// the on-disk shape loadQuestions() reads (data/files/questions/<id>/question.json).
// orderVal may be nil to omit the ORDER field entirely (exercising the
// fallback-to-ID-as-int rule).
func writeQuestionFile(t *testing.T, dir, id string, orderVal interface{}, extra map[string]interface{}) {
	t.Helper()
	qDir := filepath.Join(dir, id)
	if err := os.MkdirAll(qDir, 0755); err != nil {
		t.Fatalf("failed to create question dir: %v", err)
	}
	q := map[string]interface{}{
		"ID":       id,
		"QUESTION": "Q" + id,
		"CATEGORY": "CAT",
		"TYPE":     "SPEEDY",
		"POINTS":   "10",
		"TIME":     "8",
	}
	if orderVal != nil {
		q["ORDER"] = orderVal
	}
	for k, v := range extra {
		q[k] = v
	}
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("failed to marshal question fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(qDir, "question.json"), data, 0644); err != nil {
		t.Fatalf("failed to write question.json: %v", err)
	}
}

// newNextQuestionTestApp sets up a minimal App with a temp QuestionsDir and
// the logger/udpBcast every "conduite" path (READY, STOP, REVEAL, NEW_GAME)
// unconditionally dereferences — same reasoning as newAnimTestApp
// (inbound_allowlist_anim_test.go).
func newNextQuestionTestApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppWithHub(t)
	app.logger = server.NewBroadcastLogger(100)
	app.udpBcast = server.NewUDPBroadcaster()
	app.config.Storage.QuestionsDir = t.TempDir()
	return app
}

func TestGetNextQuestionPayload_NoCurrentQuestion_ReturnsNil(t *testing.T) {
	app := newNextQuestionTestApp(t)
	writeQuestionFile(t, app.config.Storage.QuestionsDir, "1", 1, nil)

	if got := app.getNextQuestionPayload(); got != nil {
		t.Errorf("expected nil (no current question), got %+v", got)
	}
}

func TestGetNextQuestionPayload_ReturnsFirstAfterCurrentInOrder(t *testing.T) {
	app := newNextQuestionTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil)
	writeQuestionFile(t, dir, "3", 3, nil)

	app.engine.Ready("1", &game.Question{ID: "1"})

	got := app.getNextQuestionPayload()
	if got == nil || got.ID != "2" {
		t.Fatalf("expected next question ID=2, got %+v", got)
	}
	if got.Question != "Q2" || got.Category != "CAT" || got.Type != "SPEEDY" || got.Points != 10 || got.Time != 8 {
		t.Errorf("payload fields not correctly populated: %+v", got)
	}
}

func TestGetNextQuestionPayload_NoneAfterCurrent_ReturnsNil(t *testing.T) {
	app := newNextQuestionTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil)

	app.engine.Ready("2", &game.Question{ID: "2"}) // last in order

	if got := app.getNextQuestionPayload(); got != nil {
		t.Errorf("expected nil (current is last), got %+v", got)
	}
}

func TestGetNextQuestionPayload_CurrentNotInList_SearchesFromStart(t *testing.T) {
	app := newNextQuestionTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil)

	// Current question ID "99" was deleted from disk (or never existed there)
	// — sortedQuestions.findIndex returns -1 in JS, so the search starts at
	// index 0, not "nothing found".
	app.engine.Ready("99", &game.Question{ID: "99"})

	got := app.getNextQuestionPayload()
	if got == nil || got.ID != "1" {
		t.Fatalf("expected search to start from the beginning (ID=1), got %+v", got)
	}
}

func TestGetNextQuestionPayload_CustomOrder_RespectsOrderFieldNotID(t *testing.T) {
	app := newNextQuestionTestApp(t)
	dir := app.config.Storage.QuestionsDir
	// ID "5" is numerically smaller than "10", but ORDER puts "10" first.
	writeQuestionFile(t, dir, "10", 1, nil)
	writeQuestionFile(t, dir, "5", 2, nil)

	app.engine.Ready("10", &game.Question{ID: "10"})

	got := app.getNextQuestionPayload()
	if got == nil || got.ID != "5" {
		t.Fatalf("expected ORDER (not ID) to determine sequence, next should be ID=5, got %+v", got)
	}
}

func TestGetNextQuestionPayload_FallsBackToIDWhenOrderAbsent(t *testing.T) {
	app := newNextQuestionTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", nil, nil) // no ORDER field
	writeQuestionFile(t, dir, "2", nil, nil) // no ORDER field

	app.engine.Ready("1", &game.Question{ID: "1"})

	got := app.getNextQuestionPayload()
	if got == nil || got.ID != "2" {
		t.Fatalf("expected ID-as-int fallback ordering, next should be ID=2, got %+v", got)
	}
}

func TestGetNextQuestionPayload_SkipsExcludedStatuses(t *testing.T) {
	app := newNextQuestionTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil) // will be marked STOPPED
	writeQuestionFile(t, dir, "3", 3, nil)

	app.engine.Ready("1", &game.Question{ID: "1"})
	// Make "2" the current question, stop it (STATUS -> STOPPED for "2"),
	// then move back to "1" as current — same technique as a real
	// STOPPED->READY->new-question sequence.
	app.engine.Ready("2", &game.Question{ID: "2"})
	app.engine.Stop()
	app.engine.Ready("1", &game.Question{ID: "1"})

	got := app.getNextQuestionPayload()
	if got == nil || got.ID != "3" {
		t.Fatalf("expected #2 (STOPPED) to be skipped, next should be ID=3, got %+v", got)
	}
}

// TestHandleWebMessage_Anim_ReceivesNextQuestionOnHello is the HELLO
// integration case: a connecting animateur must receive NEXT_QUESTION
// immediately, targeted (not waiting for the next trigger event).
func TestHandleWebMessage_Anim_ReceivesNextQuestionOnHello(t *testing.T) {
	app := newNextQuestionTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil)
	app.engine.Ready("1", &game.Question{ID: "1"})

	baseURL := startAnimAllowlistTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/anim")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeAnim)

	_, raw := readActionMatching(t, conn, protocol.ActionNextQuestion)
	var payload struct {
		Msg protocol.NextQuestionPayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("failed to unmarshal NEXT_QUESTION: %v (raw: %s)", err, raw)
	}
	if payload.Msg.ID != "2" {
		t.Errorf("expected NEXT_QUESTION ID=2 at HELLO, got %+v", payload.Msg)
	}
}

// TestHandleWebMessage_Anim_ReceivesNextQuestionOnReady is the READY trigger
// integration case, exercised through the real dispatch path (handleReady),
// not a direct getNextQuestionPayload() call.
func TestHandleWebMessage_Anim_ReceivesNextQuestionOnReady(t *testing.T) {
	app := newNextQuestionTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil)

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)

	sendAction(t, app, admin, protocol.ActionReady, protocol.ReadyPayload{Question: "1"})

	_, raw := readActionMatching(t, anim, protocol.ActionNextQuestion)
	var payload struct {
		Msg protocol.NextQuestionPayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("failed to unmarshal NEXT_QUESTION: %v (raw: %s)", err, raw)
	}
	if payload.Msg.ID != "2" {
		t.Errorf("#155/#156 B5: expected NEXT_QUESTION ID=2 broadcast after READY, got %+v", payload.Msg)
	}
}

// TestBroadcastNextQuestion_OnlyReachesAnim confirms NEXT_QUESTION is never
// sent to admin/TV/VPlayer — contracts/websocket-endpoints.md "Filtres de
// diffusion par type": NEXT_QUESTION ✓ exclusif animateur.
func TestBroadcastNextQuestion_OnlyReachesAnim(t *testing.T) {
	app := newNextQuestionTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil)
	app.engine.Ready("1", &game.Question{ID: "1"})

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	tv := dialWS(t, baseURL, "/ws/tv")
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, admin)
	learnClientID(t, app, tv)
	learnClientID(t, app, anim)

	app.broadcastNextQuestion()

	readActionMatching(t, anim, protocol.ActionNextQuestion)

	admin.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, data, err := admin.ReadMessage(); err == nil {
		var envelope struct {
			Action string `json:"ACTION"`
		}
		if json.Unmarshal(data, &envelope) == nil && envelope.Action == protocol.ActionNextQuestion {
			t.Error("admin must never receive NEXT_QUESTION")
		}
	}
	tv.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, data, err := tv.ReadMessage(); err == nil {
		var envelope struct {
			Action string `json:"ACTION"`
		}
		if json.Unmarshal(data, &envelope) == nil && envelope.Action == protocol.ActionNextQuestion {
			t.Error("TV must never receive NEXT_QUESTION")
		}
	}
}

// TestBroadcastUpdate_DoesNotTriggerNextQuestion guards plan §2.2's explicit
// prohibition: NEXT_QUESTION must never be recomputed/resent from
// broadcastUpdate/broadcastUpdateTo (a disk read — loadQuestions — on every
// timer tick). Deleting the QuestionsDir after setup and confirming a plain
// UPDATE-only broadcast doesn't error/panic isn't enough to prove this by
// itself, so instead: trigger a broadcastUpdate()-only path (BUMPER_POINTS)
// and confirm NO NEXT_QUESTION arrives for an already-connected anim client
// (only the actions on B5's explicit trigger list do that).
func TestBroadcastUpdate_DoesNotTriggerNextQuestion(t *testing.T) {
	app := newNextQuestionTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.UpdateBumper("bumper-1", map[string]interface{}{"TEAM": "TeamA"})
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil)
	app.engine.Ready("1", &game.Question{ID: "1"})

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionBumperPoints, protocol.BumperPointsPayload{ID: "bumper-1", Points: 10})

	anim.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	for {
		_, data, err := anim.ReadMessage()
		if err != nil {
			return // timed out without ever seeing NEXT_QUESTION — expected
		}
		var envelope struct {
			Action string `json:"ACTION"`
		}
		if json.Unmarshal(data, &envelope) == nil && envelope.Action == protocol.ActionNextQuestion {
			t.Fatal("#155/#156 B5: BUMPER_POINTS (broadcastUpdate only) must not also trigger NEXT_QUESTION")
		}
	}
}
