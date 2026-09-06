package server

// Tests d'intégration pour POST /api/rafale/generate-questions (issue #203,
// milestone v8.1.0, contrat contracts/rafale-ai-generation.md §2/§6/§7/§8,
// plan _work/reports/plan-20260901-162105.md tâches 7-9). dev-backend et
// test-writer tournent en parallèle sur le même contrat (même motif que
// ai_generation_test.go/ai_batching_test.go/ai_job_test.go).
//
// Réutilise les helpers déjà en place dans ce package (même binaire de
// test) : setupTestHTTPServer, setAIConfig, validAIConfig, decodeJSONBody,
// dialAdminWS, waitForJobTerminalState, readNextAIProgress, expectAccepted,
// mockAnthropicSSEServer/writeSSEQuestions, blockingAnthropicServer,
// withAnthropicBaseURL. writeSSEQuestions n'est pas spécifique à la forme
// Quiz : elle enveloppe simplement la liste donnée dans {"questions": [...]}
// — directement réutilisable pour la forme plate RAFALE (QUESTION/ANSWER/
// CATEGORY/DIFFICULTY).
//
// ⚠️ config.SetInstance mute un singleton global — pas de t.Parallel().

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bytes"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
)

// ================================================================================
// Helpers
// ================================================================================

func postRafaleGenerateQuestions(server *HTTPServer, payload map[string]interface{}) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/rafale/generate-questions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	return w
}

// baseRafaleRequest returns a fresh, fully valid request body per contract
// §2. Tests mutate the returned map before marshaling.
func baseRafaleRequest() map[string]interface{} {
	return map[string]interface{}{
		"theme":        "Culture générale — France",
		"populations":  []string{"Adulte (18-64 ans)"},
		"language":     "Français",
		"instructions": "",
		"categories":   []string{string(game.CategoryHistory)},
		"difficulties": []int{1},
		"count":        10,
	}
}

// rafaleLLMQuestion builds one flat RAFALE-shaped LLM output item (contract
// §4 — no TYPE, no TIME, no anyOf discriminator, unlike the Quiz path's
// llmQuestion helper).
func rafaleLLMQuestion(fields map[string]interface{}) map[string]interface{} {
	base := map[string]interface{}{
		"QUESTION":   "Question générée ?",
		"ANSWER":     "Réponse",
		"CATEGORY":   string(game.CategoryHistory),
		"DIFFICULTY": 1,
	}
	for k, v := range fields {
		base[k] = v
	}
	return base
}

// putRafaleRoundInProgress puts the engine's live GameState into "a RAFALE
// round is currently being played" — the exact condition contract §7
// defines: current question is TYPE=RAFALE and PHASE ∈ {STARTED, PAUSED}.
func putRafaleRoundInProgress(server *HTTPServer, phase game.GamePhase) {
	server.engine.Ready("rafale-round-1", &game.Question{ID: "rafale-round-1", Type: game.QuestionTypeRafale})
	server.engine.SetPhase(phase)
}

// ================================================================================
// Section A — validation synchrone, avant tout job (contrat §2)
// ================================================================================

func TestRafaleGenerateQuestions_MethodNotAllowed(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())

	req := httptest.NewRequest("GET", "/api/rafale/generate-questions", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for GET /api/rafale/generate-questions, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRafaleGenerateQuestions_NotCapturedByQuestionsPrefix guards against a
// registration-order regression (contract §2, "vérifier qu'il ne capte pas
// la nouvelle route" — the same class of bug the reset-all route needed a
// dedicated exact-path registration for, contracts/rafale.md §9). The new
// route's own literal prefix ("/api/rafale/generate-questions") never
// actually collides with "/api/rafale/questions/" in Go's ServeMux (they
// don't share a common path prefix), so this is a regression guard on the
// registration itself, not a router ambiguity test: POSTing a fully valid
// body must reach the generation handler (202 + job_id), never the reservoir
// CRUD handler's DELETE-only 405/404 semantics.
func TestRafaleGenerateQuestions_NotCapturedByQuestionsPrefix(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())
	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{rafaleLLMQuestion(nil)})
	withAnthropicBaseURL(t, upstream.URL)
	conn := dialAdminWS(t, server)

	w := postRafaleGenerateQuestions(server, baseRafaleRequest())
	if w.Code != http.StatusAccepted {
		t.Fatalf("Expected 202 Accepted (proves the request reached the generation handler, not handleRafaleQuestionByID), got %d: %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if _, ok := body["job_id"]; !ok {
		t.Errorf("Expected a job_id in the 202 body, got %v", body)
	}
	waitForJobTerminalState(t, conn, 5*time.Second)
}

func TestRafaleGenerateQuestions_NoAPIKey_Returns409(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(config.AIConfig{AnthropicAPIKey: "", Model: "claude-opus-5", TimeoutSeconds: 300, MaxQuestions: 200})

	w := postRafaleGenerateQuestions(server, baseRafaleRequest())

	if w.Code != http.StatusConflict {
		t.Fatalf("Expected 409 when no API key configured, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if body["code"] != "no_api_key" {
		t.Errorf("Expected code=no_api_key, got %v", body["code"])
	}
}

func TestRafaleGenerateQuestions_InvalidPayload_Returns400(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())

	req := baseRafaleRequest()
	req["count"] = 7 // not a preset (contract §2bis)
	w := postRafaleGenerateQuestions(server, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for a non-preset count, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if body["code"] != "invalid_request" {
		t.Errorf("Expected code=invalid_request, got %v", body["code"])
	}
}

// ================================================================================
// Section B — un seul job à la fois, tous chemins confondus (contrat §1.2/§2)
// ================================================================================

func TestRafaleGenerateQuestions_WhileQuizJobRunning_Returns409(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 10, 2))

	slow := blockingAnthropicServer(t, 500*time.Millisecond, []map[string]interface{}{
		llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "ANSWER": "ok"}),
	})
	withAnthropicBaseURL(t, slow.URL)
	conn := dialAdminWS(t, server)

	quizReq := baseGenerateRequest()
	quizReq["volume"] = map[string]interface{}{"mode": "count", "value": 1}
	expectAccepted(t, postGenerateQuestions(server, quizReq))

	w := postRafaleGenerateQuestions(server, baseRafaleRequest())
	if w.Code != http.StatusConflict {
		t.Fatalf("Expected 409 while a QUIZ job is running (contract §1.2 — single job registry, tous chemins confondus), got %d: %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if body["code"] != "generation_in_progress" {
		t.Errorf("Expected code=generation_in_progress, got %v", body["code"])
	}

	waitForJobTerminalState(t, conn, 5*time.Second)
}

func TestRafaleGenerateQuestions_WhileRafaleJobRunning_QuizReturns409(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 10, 2))

	slow := blockingAnthropicServer(t, 500*time.Millisecond, []map[string]interface{}{rafaleLLMQuestion(nil)})
	withAnthropicBaseURL(t, slow.URL)
	conn := dialAdminWS(t, server)

	rafaleReq := baseRafaleRequest()
	rafaleReq["count"] = 10
	expectAccepted(t, postRafaleGenerateQuestions(server, rafaleReq))

	w := postGenerateQuestions(server, baseGenerateRequest())
	if w.Code != http.StatusConflict {
		t.Fatalf("Expected 409 while a RAFALE job is running (contract §1.2 — same shared registry), got %d: %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if body["code"] != "generation_in_progress" {
		t.Errorf("Expected code=generation_in_progress, got %v", body["code"])
	}

	waitForJobTerminalState(t, conn, 5*time.Second)
}

// ================================================================================
// Section C — refus pendant une manche RAFALE en cours (contrat §7)
// ================================================================================

func TestRafaleGenerateQuestions_RoundStarted_Returns409(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())
	putRafaleRoundInProgress(server, game.PhaseStarted)

	w := postRafaleGenerateQuestions(server, baseRafaleRequest())
	if w.Code != http.StatusConflict {
		t.Fatalf("Expected 409 while a RAFALE round is STARTED, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if body["code"] != "rafale_round_in_progress" {
		t.Errorf("Expected code=rafale_round_in_progress, got %v", body["code"])
	}
}

func TestRafaleGenerateQuestions_RoundPaused_Returns409(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())
	putRafaleRoundInProgress(server, game.PhasePaused)

	w := postRafaleGenerateQuestions(server, baseRafaleRequest())
	if w.Code != http.StatusConflict {
		t.Fatalf("Expected 409 while a RAFALE round is PAUSED (contract §7 explicitly includes PAUSED, not just STARTED), got %d: %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if body["code"] != "rafale_round_in_progress" {
		t.Errorf("Expected code=rafale_round_in_progress, got %v", body["code"])
	}
}

func TestRafaleGenerateQuestions_NoRoundInProgress_Accepted(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())
	// Default fresh-engine phase is STOPPED, no current question — must be
	// accepted normally.
	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{rafaleLLMQuestion(nil)})
	withAnthropicBaseURL(t, upstream.URL)
	conn := dialAdminWS(t, server)

	expectAccepted(t, postRafaleGenerateQuestions(server, baseRafaleRequest()))
	waitForJobTerminalState(t, conn, 5*time.Second)
}

func TestRafaleGenerateQuestions_NonRafaleQuestionStarted_Accepted(t *testing.T) {
	// A STARTED Quiz question (SPEEDY, not RAFALE) must NOT trigger
	// rafale_round_in_progress — the condition is specifically TYPE=RAFALE
	// (contract §7), not merely "a game is in progress".
	server, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())
	server.engine.Ready("q-1", &game.Question{ID: "q-1", Type: game.QuestionTypeSpeedy})
	server.engine.SetPhase(game.PhaseStarted)

	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{rafaleLLMQuestion(nil)})
	withAnthropicBaseURL(t, upstream.URL)
	conn := dialAdminWS(t, server)

	w := postRafaleGenerateQuestions(server, baseRafaleRequest())
	if w.Code != http.StatusAccepted {
		t.Fatalf("Expected 202 — a STARTED non-RAFALE question must not trigger rafale_round_in_progress, got %d: %s", w.Code, w.Body.String())
	}
	waitForJobTerminalState(t, conn, 5*time.Second)
}

// ================================================================================
// Section D — cycle complet, persistance (contrat §8/§9)
// ================================================================================

func TestRafaleGenerateQuestions_FullCycle_ReservoirGrows_UsedUnaffected_AllNewAvailable(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())

	// Pre-existing reservoir entry, already marked used — must survive
	// untouched, and its USED flag must not move (contract §9).
	seeded, err := server.engine.UpsertRafaleQuestion(game.RafaleQuestion{
		Question: "Pré-existante", Answer: "PA", Category: game.CategoryHistory, Difficulty: 1,
	})
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	if _, err := server.engine.DrawRafaleQuestion([]string{string(game.CategoryHistory)}, []int{1}); err != nil {
		t.Fatalf("failed to mark the seeded question used: %v", err)
	}
	_, usedBefore := server.engine.SnapshotRafaleReservoir()
	if !usedBefore[seeded.ID] {
		t.Fatalf("precondition failed: seeded question should be marked used")
	}

	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{
		rafaleLLMQuestion(map[string]interface{}{"QUESTION": "Générée 1 ?", "ANSWER": "G1"}),
		rafaleLLMQuestion(map[string]interface{}{"QUESTION": "Générée 2 ?", "ANSWER": "G2"}),
	})
	withAnthropicBaseURL(t, upstream.URL)
	conn := dialAdminWS(t, server)

	req := baseRafaleRequest()
	req["count"] = 10
	expectAccepted(t, postRafaleGenerateQuestions(server, req))
	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "DONE" {
		t.Fatalf("expected STATE=DONE, got %v (msg=%v)", final["STATE"], final)
	}

	all, usedAfter := server.engine.SnapshotRafaleReservoir()
	if len(all) < 2 {
		t.Fatalf("expected at least 2 questions added to the reservoir, got total %d", len(all))
	}
	if !usedAfter[seeded.ID] {
		t.Error("the pre-existing seeded question's USED flag must be unaffected by generation")
	}
	newCount := 0
	for _, q := range all {
		if q.ID == seeded.ID {
			continue
		}
		if usedAfter[q.ID] {
			t.Errorf("a newly generated question must be AVAILABLE by construction (never in rafale_used.json), got USED for %q", q.ID)
		}
		newCount++
	}
	if newCount == 0 {
		t.Error("expected at least one newly generated question in the reservoir")
	}
}

func TestRafaleGenerateQuestions_GeneratedQuestion_IsDrawable(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())

	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{
		rafaleLLMQuestion(map[string]interface{}{"QUESTION": "Drawable ?", "ANSWER": "Yes", "CATEGORY": string(game.CategoryHistory), "DIFFICULTY": 1}),
	})
	withAnthropicBaseURL(t, upstream.URL)
	conn := dialAdminWS(t, server)

	req := baseRafaleRequest()
	req["count"] = 10
	expectAccepted(t, postRafaleGenerateQuestions(server, req))
	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "DONE" || final["CREATED_COUNT"] == float64(0) {
		t.Fatalf("expected a successful generation with at least 1 created question, got %v", final)
	}

	drawn, err := server.engine.DrawRafaleQuestion([]string{string(game.CategoryHistory)}, []int{1})
	if err != nil {
		t.Fatalf("expected the generated question to be drawable on its couple, got error: %v", err)
	}
	if drawn == nil {
		t.Fatal("DrawRafaleQuestion returned nil question with no error")
	}
}

// ================================================================================
// Section E — annulation (contrat, réutilise CancelAIJob existant)
// ================================================================================

func TestRafaleGenerateQuestions_Cancellation_KeepsCreatedQuestions(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 1, 30*1000, 2)) // 1/batch, long inter-batch delay to give time to cancel

	outcomes := []batchOutcome{
		{questions: []map[string]interface{}{rafaleLLMQuestion(map[string]interface{}{"QUESTION": "Batch 0 ?", "ANSWER": "A0"})}},
		{questions: []map[string]interface{}{rafaleLLMQuestion(map[string]interface{}{"QUESTION": "Batch 1 ?", "ANSWER": "A1"})}},
	}
	upstream, _ := sequencedAnthropicServer(t, outcomes)
	withAnthropicBaseURL(t, upstream.URL)
	conn := dialAdminWS(t, server)

	req := baseRafaleRequest()
	req["count"] = 20 // 2 batches at batch_size=1... actually batch_size clamps distribution; only need >1 batch
	accepted := expectAccepted(t, postRafaleGenerateQuestions(server, req))
	jobID, _ := accepted["job_id"].(string)

	// Wait for the first batch's progress (at least 1 created), then cancel
	// before the (long) inter-batch delay elapses.
	msg := readNextAIProgress(t, conn, 5*time.Second)
	if msg["STATE"] != "RUNNING" {
		t.Fatalf("expected the first progress message to be RUNNING, got %v", msg)
	}
	if !CancelAIJob(jobID) {
		t.Fatal("CancelAIJob returned false for a job that should still be running")
	}

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "CANCELLED" {
		t.Fatalf("expected STATE=CANCELLED, got %v", final["STATE"])
	}
	if final["CREATED_COUNT"] == float64(0) {
		t.Error("expected at least the first batch's question(s) to be conserved after cancellation")
	}

	all, _ := server.engine.SnapshotRafaleReservoir()
	if len(all) == 0 {
		t.Error("questions created before cancellation must remain in the reservoir")
	}
}

// ================================================================================
// Section F — champ TARGET (contrat §6)
// ================================================================================

func TestRafaleGenerateQuestions_ProgressCarriesTargetRafale(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())
	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{rafaleLLMQuestion(nil)})
	withAnthropicBaseURL(t, upstream.URL)
	conn := dialAdminWS(t, server)

	expectAccepted(t, postRafaleGenerateQuestions(server, baseRafaleRequest()))
	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["TARGET"] != "RAFALE" {
		t.Errorf(`Expected TARGET="RAFALE" on a RAFALE job's progress message (contract §6), got %v (msg=%v)`, final["TARGET"], final)
	}
}

func TestGenerateQuestions_ProgressCarriesTargetQuiz_NonRegression(t *testing.T) {
	// Non-regression: the PRE-EXISTING Quiz path must now carry
	// TARGET="QUIZ" explicitly (contract §6: "absent ⇒ QUIZ" is the
	// backward-compat reading for an OLD client; a server implementing #203
	// sets it explicitly on every progress message it emits).
	server, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())
	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{
		llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "ANSWER": "ok"}),
	})
	withAnthropicBaseURL(t, upstream.URL)
	conn := dialAdminWS(t, server)

	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 1}
	expectAccepted(t, postGenerateQuestions(server, req))
	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["TARGET"] != "QUIZ" {
		t.Errorf(`Expected TARGET="QUIZ" on the pre-existing Quiz job's progress message, got %v (msg=%v)`, final["TARGET"], final)
	}
}

func TestRafaleGenerateQuestions_ReplayedToNewAdmin_CarriesTarget(t *testing.T) {
	// contract §6: "Rejoué à la connexion d'un client admin si un job est en
	// cours" — a SECOND admin connecting mid-job must also see TARGET.
	server, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig()) // default batch size (20) — a single couple's count=10 fits in ONE batch
	slow := blockingAnthropicServer(t, 300*time.Millisecond, []map[string]interface{}{rafaleLLMQuestion(nil)})
	withAnthropicBaseURL(t, slow.URL)

	firstConn := dialAdminWS(t, server)
	expectAccepted(t, postRafaleGenerateQuestions(server, baseRafaleRequest()))

	// A second admin connects WHILE the job is running.
	secondConn := dialAdminWS(t, server)
	msg := readNextAIProgress(t, secondConn, 5*time.Second)
	if msg["TARGET"] != "RAFALE" {
		t.Errorf(`Expected the replayed progress pushed to a newly-connected admin to carry TARGET="RAFALE", got %v`, msg["TARGET"])
	}

	waitForJobTerminalState(t, firstConn, 5*time.Second)
}
