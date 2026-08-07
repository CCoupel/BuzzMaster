package server

// Tests dérivés de contracts/ai-multi-provider.md (#137, v6.1.0) §9 à §12, et de
// _work/reports/planner-20260805-204318-plan-137.md, phase 2.
//
// Fichier NEUF — même paquet que ai_batching_test.go, réutilise ses helpers
// (batchingAIConfig, sequencedAnthropicServer, batchOutcome, twoUniqueSpeedyQuestions,
// dialAdminWS, dialWS, readNextAIProgress, waitForJobTerminalState, expectAccepted,
// collectQuestionIDs, withAnthropicBaseURL, withGroqBaseURL) et celles de
// ai_generation_test.go (setupTestHTTPServer, setAIConfig, validAIConfig, llmQuestion,
// baseGenerateRequest, postGenerateQuestions, decodeJSONBody, countQuestionDirs).
// AUCUNE redéfinition. ⚠️ Pas de t.Parallel() (config.SetInstance mute un global).
//
// Portée : le REGISTRE de job (un seul à la fois, annulation, compteurs cumulatifs,
// ré-attachement à la connexion) — la mécanique de découpage en lots elle-même est
// couverte par ai_batching_test.go. Même hypothèse d'intégration (GROQ_BASE_URL /
// ANTHROPIC_BASE_URL, cf. en-têtes de ai_generation_test.go et ai_batching_test.go).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// assertNoActionReceived fails the test if a message with the given ACTION arrives
// on conn before the timeout elapses. Other actions are silently ignored (a real
// admin/TV connection may receive unrelated sync traffic).
func assertNoActionReceived(t *testing.T, conn interface {
	SetReadDeadline(time.Time) error
	ReadMessage() (int, []byte, error)
}, action string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return // timeout elapsed with no matching message — success
		}
		conn.SetReadDeadline(time.Now().Add(remaining))
		_, data, err := conn.ReadMessage()
		if err != nil {
			return // read deadline exceeded (or connection closed) — no message arrived
		}
		var envelope struct {
			Action string `json:"ACTION"`
		}
		if json.Unmarshal(data, &envelope) == nil && envelope.Action == action {
			t.Fatalf("Expected no %q message on this connection, but received one", action)
		}
	}
}

// ================================================================================
// Un seul job à la fois (contract §9, CA9)
// ================================================================================

func TestAIJob_SecondGenerateWhileRunning_Returns409(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 10, 2))

	// The first batch deliberately takes a moment to respond, giving the test a
	// reliable window in which the job is provably still RUNNING.
	slow := blockingAnthropicServer(t, 500*time.Millisecond, []map[string]interface{}{
		llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": "Job 1", "ANSWER": "ok"}),
	})
	withAnthropicBaseURL(t, slow.URL)

	conn := dialAdminWS(t, server)

	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 1}
	first := expectAccepted(t, postGenerateQuestions(server, req))
	firstJobID, _ := first["job_id"].(string)

	// Fired immediately after — the first batch is still sleeping.
	w := postGenerateQuestions(server, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("Expected 409 while a job is running, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if body["code"] != "generation_in_progress" {
		t.Errorf("Expected code=generation_in_progress, got %v", body["code"])
	}

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "DONE" {
		t.Fatalf("Expected the FIRST job to complete DONE undisturbed, got %v", final["STATE"])
	}
	if final["JOB_ID"] != firstJobID {
		t.Errorf("Expected the terminal message to reference the first job (%s), got %v", firstJobID, final["JOB_ID"])
	}
	if countQuestionDirs(t, dataDir) != 1 {
		t.Errorf("Expected exactly 1 question created (only the first job ran), got %d", countQuestionDirs(t, dataDir))
	}
}

func TestAIJob_ResponseIs202WithJobIdAndBatchesTotal(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 10, 2))

	upstream, _ := sequencedAnthropicServer(t, []batchOutcome{{questions: twoUniqueSpeedyQuestions(0)}})
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 2}

	w := postGenerateQuestions(server, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("Expected 202, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if body["status"] != "accepted" {
		t.Errorf("Expected status=accepted, got %v", body["status"])
	}
	if jobID, _ := body["job_id"].(string); jobID == "" {
		t.Error("Expected a non-empty job_id")
	}
	if body["batches_total"] != float64(1) {
		t.Errorf("Expected batches_total=1 for volume=2/batch_size=2, got %v", body["batches_total"])
	}
	// #8's synchronous fields must be gone — this endpoint no longer returns a
	// result body (contract §9 "BREAKING").
	if _, has := body["created_count"]; has {
		t.Error("Expected no created_count in the 202 response — the result now arrives via AI_GENERATION_PROGRESS")
	}

	waitForJobTerminalState(t, conn, 5*time.Second)
}

func TestAIJob_NoAPIKeyForSelectedProvider_Returns409(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	cfg := batchingAIConfig("groq", 2, 10, 2)
	cfg.GroqAPIKey = "" // no Groq key — even though Anthropic's key (set by validAIConfig) IS present
	setAIConfig(cfg)

	req := baseGenerateRequest()
	w := postGenerateQuestions(server, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("Expected 409 when the SELECTED provider (groq) has no key, even though anthropic does, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if body["code"] != "no_api_key" {
		t.Errorf("Expected code=no_api_key, got %v", body["code"])
	}
}

// ================================================================================
// Annulation (contract §11, CA6)
// ================================================================================

func TestAIJob_CancelBetweenBatches_KeepsCreatedQuestions(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 150, 2)) // delay long enough to reliably cancel mid-job

	outcomes := []batchOutcome{
		{questions: twoUniqueSpeedyQuestions(0)},
		{questions: twoUniqueSpeedyQuestions(1)},
		{questions: twoUniqueSpeedyQuestions(2)},
		{questions: twoUniqueSpeedyQuestions(3)},
		{questions: twoUniqueSpeedyQuestions(4)},
	}
	upstream, callCount := sequencedAnthropicServer(t, outcomes)
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 10}
	start := expectAccepted(t, postGenerateQuestions(server, req))
	jobID, _ := start["job_id"].(string)

	// Wait for the first batch to land, then cancel — contract §11: takes effect
	// BETWEEN two batches, never mid-call.
	progress := readNextAIProgress(t, conn, 3*time.Second)
	for progress["BATCHES_DONE"] == float64(0) {
		progress = readNextAIProgress(t, conn, 3*time.Second)
	}

	if err := conn.WriteJSON(map[string]interface{}{
		"ACTION": "CANCEL_AI_GENERATION",
		"MSG":    map[string]interface{}{"JOB_ID": jobID},
	}); err != nil {
		t.Fatalf("Failed to send CANCEL_AI_GENERATION: %v", err)
	}

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "CANCELLED" {
		t.Fatalf("Expected STATE=CANCELLED, got %v (msg=%v)", final["STATE"], final)
	}

	batchesDone, _ := final["BATCHES_DONE"].(float64)
	if batchesDone < 1 || batchesDone >= 5 {
		t.Fatalf("Expected cancellation to stop partway through (1..4 batches done), got %v", batchesDone)
	}
	if got := atomic.LoadInt32(callCount); got >= 5 {
		t.Errorf("Expected fewer than 5 provider calls (job stopped before completion), got %d", got)
	}

	// Whatever was created before cancellation must remain on disk (CA6).
	createdCount, _ := final["CREATED_COUNT"].(float64)
	if float64(countQuestionDirs(t, dataDir)) != createdCount {
		t.Errorf("Expected %v question files on disk (matching CREATED_COUNT), got %d", createdCount, countQuestionDirs(t, dataDir))
	}
	if createdCount == 0 {
		t.Error("Expected at least 1 question created before cancellation took effect")
	}
}

// ================================================================================
// Progression WebSocket (contract §10)
// ================================================================================

func TestAIJob_CumulativeCounters_AcrossBatches(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 10, 2))

	invalid := func(label string) map[string]interface{} {
		return llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": label, "ANSWER": ""})
	}
	valid := func(label string) map[string]interface{} {
		return llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": label, "ANSWER": "ok"})
	}

	outcomes := []batchOutcome{
		{questions: []map[string]interface{}{valid("b1-v"), invalid("b1-i")}},          // +1 created, +1 skipped
		{questions: []map[string]interface{}{valid("b2-v1"), valid("b2-v2")}},          // +2 created, +0 skipped
		{questions: []map[string]interface{}{valid("b3-v"), invalid("b3-i")}},          // +1 created, +1 skipped
	}
	upstream, _ := sequencedAnthropicServer(t, outcomes)
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 6}
	expectAccepted(t, postGenerateQuestions(server, req))

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "DONE" {
		t.Fatalf("Expected DONE, got %v (msg=%v)", final["STATE"], final)
	}
	if final["CREATED_COUNT"] != float64(4) {
		t.Errorf("Expected CREATED_COUNT=4 cumulative across batches (1+2+1), got %v — contract §10 'CREATED_COUNT est cumulatif sur le job'", final["CREATED_COUNT"])
	}
	if final["SKIPPED_COUNT"] != float64(2) {
		t.Errorf("Expected SKIPPED_COUNT=2 cumulative (1+0+1), got %v", final["SKIPPED_COUNT"])
	}
}

func TestAIJob_ProgressEmittedImmediatelyOnAdminConnect_WhenJobRunning(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 1, 10, 2))

	// The single batch sleeps well past the assertion window below, so any
	// progress message the second connection receives quickly cannot be a
	// coincidental "batch just finished" message — it must be the
	// connect-triggered push (contract §10 "émis... à la connexion").
	slow := blockingAnthropicServer(t, 3*time.Second, []map[string]interface{}{
		llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "ANSWER": "ok"}),
	})
	withAnthropicBaseURL(t, slow.URL)

	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 1}
	first := expectAccepted(t, postGenerateQuestions(server, req))
	jobID, _ := first["job_id"].(string)

	// Give the job a brief moment to register as RUNNING, then connect fresh —
	// simulates a page reload / second admin tab (CA5).
	time.Sleep(50 * time.Millisecond)
	reattached := dialAdminWS(t, server)

	msg := readNextAIProgress(t, reattached, 2*time.Second)
	if msg["STATE"] != "RUNNING" {
		t.Fatalf("Expected an immediate RUNNING progress message on connect, got %v (msg=%v)", msg["STATE"], msg)
	}
	if msg["JOB_ID"] != jobID {
		t.Errorf("Expected the reattachment message to reference the running job (%s), got %v", jobID, msg["JOB_ID"])
	}
}

func TestAIJob_NoAdminBroadcastToOtherClientTypes(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 5, 10, 2))

	upstream, _ := sequencedAnthropicServer(t, []batchOutcome{{questions: twoUniqueSpeedyQuestions(0)}})
	withAnthropicBaseURL(t, upstream.URL)

	adminConn := dialAdminWS(t, server)
	tvConn := dialWS(t, server, "/ws/tv")

	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 2}
	expectAccepted(t, postGenerateQuestions(server, req))

	waitForJobTerminalState(t, adminConn, 5*time.Second) // confirms the job (and its broadcasts) actually ran
	assertNoActionReceived(t, tvConn, "AI_GENERATION_PROGRESS", 500*time.Millisecond)
}

// blockingAnthropicServer responds successfully but only after `delay`, letting
// tests reliably observe a job that is still RUNNING.
func blockingAnthropicServer(t *testing.T, delay time.Duration, questions []map[string]interface{}) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		writeSSEQuestions(t, w, questions)
	}))
	t.Cleanup(server.Close)
	return server
}
