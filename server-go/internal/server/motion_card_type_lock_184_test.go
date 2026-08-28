package server

import (
	"buzzcontrol/internal/game"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newMEMOTIONUploadRequest builds a multipart POST /questions request for a
// MEMOTION question whose MOTION_CARDS form field is exactly cardsJSON
// (already-serialized JSON, so callers can freely construct malformed/
// mismatched payloads for negative tests without fighting Go's json.Marshal
// field-name rules).
func newMEMOTIONUploadRequest(t *testing.T, cardsJSON string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("question", "MEMOTION test #184")
	_ = mw.WriteField("answer", "")
	_ = mw.WriteField("points", "1")
	_ = mw.WriteField("time", "30")
	_ = mw.WriteField("type", "MEMOTION")
	_ = mw.WriteField("motion_cards", cardsJSON)
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// TestHTTPServer_MEMOTIONUpload_CardTypeNotNestable is #184 B-B2's server
// half of contract §1/§3: a card declaring TYPE=MEMOTION (re-nesting) or any
// unknown type must be refused with HTTP 400, before any file is written.
// MEMORY became nestable in #187 (v7.1.0, registry NestableInMotionCard:
// true) and is exercised as an ACCEPTED type by
// motion_card_media_slots_184_test.go / engine_memory_card_187_test.go
// instead — it no longer belongs in this "refused" table. ARDOISE stays:
// #186 closed "not planned" (2026-08-24), permanently non-nestable.
func TestHTTPServer_MEMOTIONUpload_CardTypeNotNestable(t *testing.T) {
	tests := []struct {
		name string
		typ  string
	}{
		{"re-nesting MEMOTION", "MEMOTION"},
		{"unknown type", "BOGUS_TYPE"},
		{"non-nestable ARDOISE (#186 closed not planned)", "ARDOISE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, dataDir := setupTestHTTPServer(t)
			cards := []map[string]interface{}{
				{"ID": "mc-1", "RECTO_THEME": "x", "DIFFICULTY": 1, "TYPE": tt.typ},
			}
			cardsJSON, err := json.Marshal(cards)
			if err != nil {
				t.Fatalf("marshal cards: %v", err)
			}

			req := newMEMOTIONUploadRequest(t, string(cardsJSON))
			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 CARD_TYPE_NOT_NESTABLE, got %d: %s", w.Code, w.Body.String())
			}
			if !bytes.Contains(w.Body.Bytes(), []byte("CARD_TYPE_NOT_NESTABLE")) {
				t.Errorf("expected body to name CARD_TYPE_NOT_NESTABLE, got: %s", w.Body.String())
			}

			// No question.json must have been written — validation runs
			// before any card processing/disk write.
			questionsDir := filepath.Join(dataDir, "files", "questions")
			entries, _ := os.ReadDir(questionsDir)
			for _, e := range entries {
				if _, err := os.Stat(filepath.Join(questionsDir, e.Name(), "question.json")); err == nil {
					t.Errorf("expected no question.json written after a rejected upload, found one under %s", e.Name())
				}
			}
		})
	}
}

// TestHTTPServer_MEMOTIONUpload_CardTypeContentMismatch is #184 B-B2's
// server half of contract §3.2: a card carrying OwnedFields content that
// belongs to a type other than its declared (or defaulted) TYPE must be
// refused with HTTP 400 CARD_TYPE_CONTENT_MISMATCH.
func TestHTTPServer_MEMOTIONUpload_CardTypeContentMismatch(t *testing.T) {
	tests := []struct {
		name string
		card map[string]interface{}
	}{
		{
			name: "SPEEDY card (explicit) carrying QCM_ANSWERS",
			card: map[string]interface{}{
				"ID": "mc-1", "RECTO_THEME": "x", "DIFFICULTY": 1, "TYPE": "SPEEDY",
				"QCM_ANSWERS": map[string]string{"RED": "a", "GREEN": "b", "YELLOW": "c", "BLUE": "d"},
			},
		},
		{
			name: "no TYPE (defaults SPEEDY) carrying QCM_CORRECT",
			card: map[string]interface{}{
				"ID": "mc-1", "RECTO_THEME": "x", "DIFFICULTY": 1,
				"QCM_CORRECT": "RED",
			},
		},
		{
			name: "QCM card carrying ANSWER_TEXT (SPEEDY's own field)",
			card: map[string]interface{}{
				"ID": "mc-1", "RECTO_THEME": "x", "DIFFICULTY": 1, "TYPE": "QCM",
				"ANSWER_TEXT": "Paris",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, dataDir := setupTestHTTPServer(t)
			cardsJSON, err := json.Marshal([]map[string]interface{}{tt.card})
			if err != nil {
				t.Fatalf("marshal cards: %v", err)
			}

			req := newMEMOTIONUploadRequest(t, string(cardsJSON))
			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 CARD_TYPE_CONTENT_MISMATCH, got %d: %s", w.Code, w.Body.String())
			}
			if !bytes.Contains(w.Body.Bytes(), []byte("CARD_TYPE_CONTENT_MISMATCH")) {
				t.Errorf("expected body to name CARD_TYPE_CONTENT_MISMATCH, got: %s", w.Body.String())
			}

			questionsDir := filepath.Join(dataDir, "files", "questions")
			entries, _ := os.ReadDir(questionsDir)
			for _, e := range entries {
				if _, err := os.Stat(filepath.Join(questionsDir, e.Name(), "question.json")); err == nil {
					t.Errorf("expected no question.json written after a rejected upload, found one under %s", e.Name())
				}
			}
		})
	}
}

// TestHTTPServer_MEMOTIONUpload_QCMCard_Accepted is the positive
// counterpart: a QCM-typed card with only its own OwnedFields populated is
// accepted, persists TYPE=QCM verbatim, and — the retro-compat guarantee
// contract §11 makes explicit — a card with no TYPE at all is still
// accepted and round-trips with TYPE entirely absent (⇒ SPEEDY), exactly
// like every MEMOTION question saved before #184.
func TestHTTPServer_MEMOTIONUpload_QCMCard_Accepted(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	cards := []map[string]interface{}{
		{
			"ID": "mc-1", "RECTO_THEME": "Capitales", "DIFFICULTY": 2, "TYPE": "QCM",
			"QCM_ANSWERS": map[string]string{"RED": "Paris", "GREEN": "Lyon", "YELLOW": "Nice", "BLUE": "Metz"},
			"QCM_CORRECT": "RED",
		},
		{
			"ID": "mc-2", "RECTO_THEME": "Histoire", "DIFFICULTY": 1,
			"ANSWER_TEXT": "1789",
		},
	}
	cardsJSON, err := json.Marshal(cards)
	if err != nil {
		t.Fatalf("marshal cards: %v", err)
	}

	req := newMEMOTIONUploadRequest(t, string(cardsJSON))
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	questionsDir := filepath.Join(dataDir, "files", "questions")
	entries, err := os.ReadDir(questionsDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one question directory, got %v (err=%v)", entries, err)
	}

	raw, err := os.ReadFile(filepath.Join(questionsDir, entries[0].Name(), "question.json"))
	if err != nil {
		t.Fatalf("failed to read written question.json: %v", err)
	}

	var q game.Question
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatalf("failed to unmarshal written question.json: %v", err)
	}
	if len(q.MotionCards) != 2 {
		t.Fatalf("expected 2 motion cards, got %d", len(q.MotionCards))
	}
	if q.MotionCards[0].Type != game.QuestionTypeQCM {
		t.Errorf("card 1: TYPE = %q, want QCM", q.MotionCards[0].Type)
	}
	if q.MotionCards[0].QCMCorrect != "RED" {
		t.Errorf("card 1: QCM_CORRECT = %q, want RED", q.MotionCards[0].QCMCorrect)
	}
	if q.MotionCards[1].Type != "" {
		t.Errorf("card 2: TYPE = %q, want absent (retro-compat SPEEDY default)", q.MotionCards[1].Type)
	}
	if q.MotionCards[1].AnswerText != "1789" {
		t.Errorf("card 2: ANSWER_TEXT = %q, want 1789", q.MotionCards[1].AnswerText)
	}
}
