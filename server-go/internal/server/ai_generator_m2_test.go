package server

// Regression test for code-review finding M-2
// (_work/reports/code-review-20260805-163118.md): a batch that fails partway
// through writing (here: ID exhaustion hit mid-batch, not before any write —
// distinct from TestGenerateQuestions_IDExhausted_Returns507 in
// ai_generation_test.go, which saturates all 999 IDs *before* the call) must
// still broadcast and report the questions that were genuinely written
// before the failure — never a silent partial creation (contract CA10).
//
// Updated for the #137 async job model: the original M-2 fix reported the
// partial "created" list in the synchronous HTTP error body
// (writeGenerateErrorWithCreated) — that response shape no longer exists
// (POST /api/generate-questions is 202-only). The same guarantee now lives in
// AI_GENERATION_PROGRESS: CREATED_COUNT reflects what's really on disk even
// on a FAILED job, and h.OnQuestionUpload() still fires for the batch that
// partially wrote before failing (ai_job.go's writeErr branch).
//
// New file (not an edit to ai_generation_test.go) to avoid any collision
// with test-writer's file, reusing its helpers (setAIConfig, validAIConfig,
// mockAnthropicSSEServer, llmQuestion, baseGenerateRequest,
// postGenerateQuestions, readQuestionFile, dialAdminWS, expectAccepted,
// waitForJobTerminalState) — same package, same test binary.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateQuestions_MidBatchIDExhaustion_ReportsPartialSuccess(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())

	// Saturate IDs 1..998, leaving exactly ID 999 free: the batch's first
	// question will land on 999, its second question will then find nothing
	// free — the failure happens mid-batch, after a real write already
	// happened, not before any write like TestGenerateQuestions_IDExhausted_Returns507.
	questionsDir := filepath.Join(dataDir, "files", "questions")
	for i := 1; i <= 998; i++ {
		id := fmt.Sprintf("%d", i)
		qDir := filepath.Join(questionsDir, id)
		if err := os.MkdirAll(qDir, 0755); err != nil {
			t.Fatalf("Failed to pre-create question dir %s: %v", id, err)
		}
		marker := fmt.Sprintf(`{"ID":"%s","QUESTION":"pre-existing #%s","ANSWER":"x","POINTS":"10","TIME":"20"}`, id, id)
		if err := os.WriteFile(filepath.Join(qDir, "question.json"), []byte(marker), 0644); err != nil {
			t.Fatalf("Failed to write marker for %s: %v", id, err)
		}
	}

	broadcastCalled := 0
	server.OnQuestionUpload = func() { broadcastCalled++ }

	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{
		llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": "Va reussir", "ANSWER": "ok1"}),
		llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": "Va echouer", "ANSWER": "ok2"}),
	})
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 2}
	expectAccepted(t, postGenerateQuestions(server, req))

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "FAILED" {
		t.Fatalf("Expected STATE=FAILED on mid-batch exhaustion, got %v (msg=%v)", final["STATE"], final)
	}
	if final["ERROR_CODE"] != "id_exhausted" {
		t.Errorf("Expected ERROR_CODE=id_exhausted, got %v", final["ERROR_CODE"])
	}

	// M-2: the one question written before the failure must be reported, not
	// silently dropped.
	if final["CREATED_COUNT"] != float64(1) {
		t.Fatalf("Expected CREATED_COUNT=1 (the question written before exhaustion), got %v (msg=%v)", final["CREATED_COUNT"], final)
	}

	// M-2: OnQuestionUpload must still fire so the 1 real question broadcasts
	// to connected clients instead of only appearing after a page reload.
	if broadcastCalled == 0 {
		t.Error("Expected h.OnQuestionUpload() to be called even on a partial-batch failure (M-2 fix) — the created question is real and must be broadcast")
	}

	// The question that WAS written must actually be correct on disk, at the
	// only free ID (999).
	q999 := readQuestionFile(t, dataDir, "999")
	if q999["QUESTION"] != "Va reussir" {
		t.Errorf("Expected question 999 to be the one written before the failure, got QUESTION=%v", q999["QUESTION"])
	}

	// Question 998 (pre-existing, adjacent to the newly-written 999) must be
	// untouched — the partial-failure path must not corrupt neighbors.
	q998 := readQuestionFile(t, dataDir, "998")
	if q998["QUESTION"] != "pre-existing #998" {
		t.Errorf("Expected pre-existing question 998 untouched, got QUESTION=%v", q998["QUESTION"])
	}
}
