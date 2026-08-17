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
// Tests: #155/#156 (v6.2.0) — NEXT_QUESTION (B5), COMPLEMENTARY to
// next_question_test.go (dev-backend).
//
// next_question_test.go already covers getNextQuestionPayload's pure
// computation rule in depth (order/fallback-to-ID/no-current/current-not-
// in-list/status-exclusion) plus two dispatch-level cases (HELLO, READY).
// What it does NOT exercise through the real dispatch path
// (handleWebMessage → broadcastNextQuestion, the 5 non-READY call sites) is
// STOP/REVEAL/NEW_GAME/REORDER_QUESTIONS/DELETE as genuine trigger
// SEQUENCES — this file fills exactly that gap, plus the one pure-logic
// case their suite doesn't have: an unplayed question sitting BEFORE the
// current one in ORDER, which a literal "first unplayed in the whole list"
// bug (the contract's pre-correction text, contracts/CHANGELOG.md
// [20260813]) would wrongly select instead of searching only after current.
//
// team-lead arbitrage / CDP decision (relayed 2026-08-13): B5 must reproduce
// GamePage.jsx:216-227's nextUnplayedQuestion EXACTLY, confirmed by
// dev-backend's own investigation (next_question_test.go's header comment,
// commit 27e1b96) and the corrected contracts/websocket-actions.md
// (commit 1556f42).
// ---------------------------------------------------------------------------

// newNextQuestionIntegrationTestApp is deliberately NOT named
// newNextQuestionTestApp — that name is already taken by
// next_question_test.go (dev-backend) with a different signature/return
// type; reusing it would collide and break compilation of this package.
func newNextQuestionIntegrationTestApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppWithHub(t)
	app.logger = server.NewBroadcastLogger(100)
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.config.Storage.QuestionsDir = t.TempDir()
	return app
}

// readNextQuestion reads the next NEXT_QUESTION frame conn receives and
// decodes its MSG into protocol.NextQuestionPayload.
func readNextQuestion(t *testing.T, conn interface {
	SetReadDeadline(time.Time) error
	ReadMessage() (int, []byte, error)
}) protocol.NextQuestionPayload {
	t.Helper()
	_, raw := readActionMatching(t, conn, protocol.ActionNextQuestion)
	var envelope struct {
		Msg protocol.NextQuestionPayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("failed to unmarshal NEXT_QUESTION: %v (raw: %s)", err, raw)
	}
	return envelope.Msg
}

// ---------------------------------------------------------------------------
// The arbitrage case — the one next_question_test.go's fixtures never
// construct: every one of its scenarios makes the CURRENT question index 0
// (nothing precedes it). Here Q1 sits BEFORE the current question (Q2) and
// is itself unplayed — a "first unplayed in the whole order" bug would
// return Q1; the correct (post-arbitrage) rule must return Q3.
// ---------------------------------------------------------------------------

func TestNextQuestion_Anim_SearchesOnlyAfterCurrentIndex_NotWholeOrder(t *testing.T) {
	app := newNextQuestionIntegrationTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil) // unplayed, BEFORE current — must be ignored
	writeQuestionFile(t, dir, "2", 2, nil) // becomes current via READY
	writeQuestionFile(t, dir, "3", 3, nil) // unplayed, AFTER current — the right answer

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionReady, protocol.ReadyPayload{Question: "2"})

	got := readNextQuestion(t, anim)
	if got.ID != "3" {
		t.Errorf("arbitrage case: NEXT_QUESTION.ID = %q, want %q — Q1 is unplayed but sits BEFORE current Q2; a whole-order scan (the contract's pre-correction text) would wrongly return it", got.ID, "3")
	}
}

// TestNextQuestion_Anim_HelloNoCurrentQuestion_ReturnsFirstPlayable is the
// HELLO integration case next_question_test.go doesn't have: its own HELLO
// test (TestHandleWebMessage_Anim_ReceivesNextQuestionOnHello) always sets a
// current question via engine.Ready before connecting.
//
// #166/B2/T2 (GATE 2 D2, contracts/CHANGELOG.md [20260815-2]): REPLACES
// TestNextQuestion_Anim_EmptyPayload_NoCurrentQuestion. The old assertion
// ("no current question → empty payload") documented exactly the
// all-or-nothing arbitrage #166 deliberately reverses so the permanent "à
// suivre" button (AnimNextButton, matrice de la maquette : NEW_GAME → vert,
// "1ʳᵉ") has a first question to point at right after startup, before any
// READY has ever happened — not neutralized, rewritten against the new
// contract.
func TestNextQuestion_Anim_HelloNoCurrentQuestion_ReturnsFirstPlayable(t *testing.T) {
	app := newNextQuestionIntegrationTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil)

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	clientID := learnClientID(t, app, anim)

	app.sendStateToClient(clientID, server.ClientTypeAnim)

	got := readNextQuestion(t, anim)
	if got.ID != "1" {
		t.Errorf("expected NEXT_QUESTION to point at the first playable question (ID=1) with no current question, got ID=%q", got.ID)
	}
	if got.CurrentPosition != 0 {
		t.Errorf("expected CurrentPosition=0 (no current question), got %d", got.CurrentPosition)
	}
	if got.TotalQuestions != 2 {
		t.Errorf("expected TotalQuestions=2, got %d", got.TotalQuestions)
	}
}

// ---------------------------------------------------------------------------
// Trigger coverage through the REAL dispatch path (handleWebMessage) — the
// gap next_question_test.go leaves open: its non-READY trigger points
// (STOP, REVEAL, NEW_GAME, REORDER_QUESTIONS, DELETE) are only exercised by
// calling engine methods directly before a bare getNextQuestionPayload()
// call, never by actually sending the action and confirming
// broadcastNextQuestion() fires from that specific handler.
// ---------------------------------------------------------------------------

func TestNextQuestion_Anim_StopTriggerRebroadcasts(t *testing.T) {
	app := newNextQuestionIntegrationTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionReady, protocol.ReadyPayload{Question: "1"})
	readNextQuestion(t, anim) // drain READY's own NEXT_QUESTION

	sendAction(t, app, admin, protocol.ActionStart, protocol.StartPayload{Delay: 1})
	sendAction(t, app, admin, protocol.ActionStop, struct{}{})

	got := readNextQuestion(t, anim)
	if got.ID != "2" {
		t.Errorf("STOP: expected a fresh NEXT_QUESTION broadcast (ID=2), got %+v", got)
	}
}

func TestNextQuestion_Anim_RevealTriggerRebroadcasts(t *testing.T) {
	app := newNextQuestionIntegrationTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionReady, protocol.ReadyPayload{Question: "1"})
	readNextQuestion(t, anim) // drain READY's NEXT_QUESTION
	sendAction(t, app, admin, protocol.ActionStart, protocol.StartPayload{Delay: 1})
	sendAction(t, app, admin, protocol.ActionStop, struct{}{})
	readNextQuestion(t, anim) // drain STOP's NEXT_QUESTION

	sendAction(t, app, admin, protocol.ActionReveal, struct{}{})

	got := readNextQuestion(t, anim)
	if got.ID != "2" {
		t.Errorf("REVEAL: expected a fresh NEXT_QUESTION broadcast (ID=2), got %+v", got)
	}
}

func TestNextQuestion_Anim_ReorderQuestionsTriggerChangesAnswer(t *testing.T) {
	app := newNextQuestionIntegrationTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil)
	writeQuestionFile(t, dir, "3", 3, nil)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionReady, protocol.ReadyPayload{Question: "1"})
	got := readNextQuestion(t, anim)
	if got.ID != "2" {
		t.Fatalf("setup failed: expected NEXT_QUESTION=2 before reorder, got %+v", got)
	}

	// Swap "2" and "3" — now "3" immediately follows current ("1").
	sendAction(t, app, admin, protocol.ActionReorderQuestions, protocol.ReorderQuestionsPayload{Order: []string{"1", "3", "2"}})

	got = readNextQuestion(t, anim)
	if got.ID != "3" {
		t.Errorf("REORDER_QUESTIONS: expected NEXT_QUESTION to reflect the new order (ID=3), got %+v", got)
	}
}

func TestNextQuestion_Anim_DeleteTriggerRebroadcasts(t *testing.T) {
	app := newNextQuestionIntegrationTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil)
	writeQuestionFile(t, dir, "3", 3, nil)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionReady, protocol.ReadyPayload{Question: "1"})
	got := readNextQuestion(t, anim)
	if got.ID != "2" {
		t.Fatalf("setup failed: expected NEXT_QUESTION=2 before delete, got %+v", got)
	}

	// Delete "2" (the current answer) — DELETE must trigger a fresh
	// recompute that now skips straight to "3".
	sendAction(t, app, admin, protocol.ActionDelete, protocol.DeletePayload{ID: "2"})

	got = readNextQuestion(t, anim)
	if got.ID != "3" {
		t.Errorf("DELETE: NEXT_QUESTION.ID = %q, want %q (deleted question 2 must no longer be offered)", got.ID, "3")
	}
}

// TestNextQuestion_Anim_NewGameTriggerReturnsFirstPlayable — #166/B2/T2
// (GATE 2 D2, contracts/CHANGELOG.md [20260815-2]), REPLACES
// TestNextQuestion_Anim_NewGameTriggerSendsEmptyPayload. NEW_GAME resets
// GameState.Question to nil — same "no current question" arbitrage as
// HELLO (TestNextQuestion_Anim_HelloNoCurrentQuestion_ReturnsFirstPlayable),
// deliberately reversed by #166: the payload now points at the quiz's
// first playable question (matrice de la maquette : NEW_GAME → "à suivre"
// vert, "1ʳᵉ"), not "the previous question of the finished game" and not
// empty either.
func TestNextQuestion_Anim_NewGameTriggerReturnsFirstPlayable(t *testing.T) {
	app := newNextQuestionIntegrationTestApp(t)
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	writeQuestionFile(t, dir, "2", 2, nil)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	app.engine.SetPhase(game.PhaseStopped)
	sendAction(t, app, admin, protocol.ActionReady, protocol.ReadyPayload{Question: "1"})
	readNextQuestion(t, anim) // drain READY's own NEXT_QUESTION

	sendAction(t, app, admin, protocol.ActionNewGame, struct{}{})

	got := readNextQuestion(t, anim)
	if got.ID != "1" {
		t.Errorf("NEW_GAME: expected NEXT_QUESTION to point at the first playable question (ID=1) after reset, got ID=%q", got.ID)
	}
	if got.CurrentPosition != 0 {
		t.Errorf("NEW_GAME: expected CurrentPosition=0 (no current question after reset), got %d", got.CurrentPosition)
	}
	if got.TotalQuestions != 2 {
		t.Errorf("NEW_GAME: expected TotalQuestions=2, got %d", got.TotalQuestions)
	}
}
