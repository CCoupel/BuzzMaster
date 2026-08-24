// Tests for #184 B-B7 — media upload driven by the card type's descriptor
// (MediaSlots) instead of a hard-coded recto/question/answer trio.
// Run: go test ./internal/server/... -run TestHTTPServer_MEMOTIONUpload_MediaSlots -v

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

// newMEMOTIONUploadRequestWithImages is newMEMOTIONUploadRequest plus
// per-card image file fields, keyed "motion_card_<cardID>_<slot>" exactly
// as contract §8 requires the form field name to stay.
func newMEMOTIONUploadRequestWithImages(t *testing.T, cardsJSON string, images map[string][]byte) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("question", "MEMOTION media slots test #184")
	_ = mw.WriteField("answer", "")
	_ = mw.WriteField("points", "1")
	_ = mw.WriteField("time", "30")
	_ = mw.WriteField("type", "MEMOTION")
	_ = mw.WriteField("motion_cards", cardsJSON)
	for field, content := range images {
		part, err := mw.CreateFormFile(field, "img.png")
		if err != nil {
			t.Fatalf("CreateFormFile(%s): %v", field, err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("write image for %s: %v", field, err)
		}
	}
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// TestHTTPServer_MEMOTIONUpload_MediaSlots_SPEEDY is the non-regression
// case: a SPEEDY card (recto/question/answer, contract §7) still accepts
// all 3 image slots exactly like before #184.
func TestHTTPServer_MEMOTIONUpload_MediaSlots_SPEEDY(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	cards := []map[string]interface{}{
		{"ID": "mc-1", "RECTO_THEME": "x", "DIFFICULTY": 1},
	}
	cardsJSON, err := json.Marshal(cards)
	if err != nil {
		t.Fatalf("marshal cards: %v", err)
	}

	images := map[string][]byte{
		"motion_card_mc-1_recto":    minimalPNG,
		"motion_card_mc-1_question": minimalPNG,
		"motion_card_mc-1_answer":   minimalPNG,
	}
	req := newMEMOTIONUploadRequestWithImages(t, string(cardsJSON), images)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	q := readWrittenMEMOTIONQuestion(t, dataDir)
	if len(q.MotionCards) != 1 {
		t.Fatalf("expected 1 motion card, got %d", len(q.MotionCards))
	}
	card := q.MotionCards[0]
	if card.RectoImage == "" {
		t.Error("expected RECTO_IMAGE to be set for a SPEEDY card")
	}
	if card.QuestionImage == "" {
		t.Error("expected QUESTION_IMAGE to be set for a SPEEDY card")
	}
	if card.AnswerImage == "" {
		t.Error("expected ANSWER_IMAGE to be set for a SPEEDY card")
	}
}

// TestHTTPServer_MEMOTIONUpload_MediaSlots_QCM_IgnoresAnswerSlot verifies
// the point of B-B7: a QCM card's descriptor declares only recto/question
// (contract §7) — even if the client (a hand-crafted request, bypassing the
// editor UI) also sends an "answer" image field, it must be silently
// ignored, never written as ANSWER_IMAGE. Before B-B7 this would have
// written an orphaned ANSWER_IMAGE (a SPEEDY-owned field, contract §3.1)
// onto a QCM card, only caught as CARD_TYPE_CONTENT_MISMATCH on the NEXT
// save — not this one.
func TestHTTPServer_MEMOTIONUpload_MediaSlots_QCM_IgnoresAnswerSlot(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	cards := []map[string]interface{}{
		{
			"ID": "mc-1", "RECTO_THEME": "x", "DIFFICULTY": 1, "TYPE": "QCM",
			"QCM_ANSWERS": map[string]string{"RED": "a", "GREEN": "b", "YELLOW": "c", "BLUE": "d"},
			"QCM_CORRECT": "RED",
		},
	}
	cardsJSON, err := json.Marshal(cards)
	if err != nil {
		t.Fatalf("marshal cards: %v", err)
	}

	images := map[string][]byte{
		"motion_card_mc-1_recto":    minimalPNG,
		"motion_card_mc-1_question": minimalPNG,
		"motion_card_mc-1_answer":   minimalPNG, // must be ignored — QCM has no "answer" slot
	}
	req := newMEMOTIONUploadRequestWithImages(t, string(cardsJSON), images)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	q := readWrittenMEMOTIONQuestion(t, dataDir)
	if len(q.MotionCards) != 1 {
		t.Fatalf("expected 1 motion card, got %d", len(q.MotionCards))
	}
	card := q.MotionCards[0]
	if card.RectoImage == "" {
		t.Error("expected RECTO_IMAGE to be set for a QCM card (recto is one of its slots)")
	}
	if card.QuestionImage == "" {
		t.Error("expected QUESTION_IMAGE to be set for a QCM card (question is one of its slots)")
	}
	if card.AnswerImage != "" {
		t.Errorf("expected ANSWER_IMAGE to stay unset for a QCM card (not one of its media slots), got %q", card.AnswerImage)
	}

	// The card must still be internally consistent — re-validating it
	// against its own TYPE must not flag CARD_TYPE_CONTENT_MISMATCH, which
	// is exactly the regression B-B7 closes relative to B-B2 alone.
	cardMap := map[string]interface{}{"ID": card.ID}
	if card.AnswerImage != "" {
		cardMap["ANSWER_IMAGE"] = card.AnswerImage
	}
	if err := game.ValidateCardTypeContent(card.Type, cardMap); err != nil {
		t.Errorf("card became internally inconsistent after upload: %v", err)
	}
}

// TestHTTPServer_MEMOTIONUpload_MediaSlots_MEMORY_PairImages is #187
// (v7.1.0)'s server half of the dev-frontend/dev-backend coordination on
// per-pair image upload field naming for a MEMORY-typed MEMOTION card:
// TypeDescriptor.MediaSlots only declares "recto" for MEMORY (its N pairs
// aren't a fixed slot list, contract §7) — the pair images are handled by a
// dedicated branch in handleUploadQuestion, field name
// motion_card_<cardID>_pair_<pairID>_1/2, mirroring the question-host
// MEMORY upload's own memory_card_<pairID>_1/2 convention with the card
// scoped in.
func TestHTTPServer_MEMOTIONUpload_MediaSlots_MEMORY_PairImages(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	cards := []map[string]interface{}{
		{
			"ID": "mc-1", "RECTO_THEME": "x", "DIFFICULTY": 1, "TYPE": "MEMORY",
			"MEMORY_PAIRS": []map[string]interface{}{
				{"ID": 1, "CARD1": map[string]interface{}{"TEXT": "A"}, "CARD2": map[string]interface{}{"TEXT": "A"}},
				{"ID": 2, "CARD1": map[string]interface{}{"TEXT": "B"}, "CARD2": map[string]interface{}{"TEXT": "B"}},
			},
		},
	}
	cardsJSON, err := json.Marshal(cards)
	if err != nil {
		t.Fatalf("marshal cards: %v", err)
	}

	images := map[string][]byte{
		"motion_card_mc-1_recto":    minimalPNG,
		"motion_card_mc-1_pair_1_1": minimalPNG,
		"motion_card_mc-1_pair_1_2": minimalPNG,
		"motion_card_mc-1_pair_2_1": minimalPNG,
		// pair 2's second card deliberately left text-only — must survive
		// unset, not be overwritten by a leftover from another field.
	}
	req := newMEMOTIONUploadRequestWithImages(t, string(cardsJSON), images)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	q := readWrittenMEMOTIONQuestion(t, dataDir)
	if len(q.MotionCards) != 1 {
		t.Fatalf("expected 1 motion card, got %d", len(q.MotionCards))
	}
	card := q.MotionCards[0]
	if card.RectoImage == "" {
		t.Error("expected RECTO_IMAGE to be set for a MEMORY card (recto is its one MediaSlots entry)")
	}
	if len(card.MemoryPairs) != 2 {
		t.Fatalf("expected 2 MEMORY_PAIRS to survive the upload, got %d", len(card.MemoryPairs))
	}
	pair1 := card.MemoryPairs[0]
	if pair1.Card1.Image == "" || !pair1.Card1.IsImage {
		t.Errorf("pair 1 CARD1: expected IMAGE set + IS_IMAGE=true, got IMAGE=%q IS_IMAGE=%v", pair1.Card1.Image, pair1.Card1.IsImage)
	}
	if pair1.Card2.Image == "" || !pair1.Card2.IsImage {
		t.Errorf("pair 1 CARD2: expected IMAGE set + IS_IMAGE=true, got IMAGE=%q IS_IMAGE=%v", pair1.Card2.Image, pair1.Card2.IsImage)
	}
	pair2 := card.MemoryPairs[1]
	if pair2.Card1.Image == "" || !pair2.Card1.IsImage {
		t.Errorf("pair 2 CARD1: expected IMAGE set + IS_IMAGE=true, got IMAGE=%q IS_IMAGE=%v", pair2.Card1.Image, pair2.Card1.IsImage)
	}
	if pair2.Card2.Image != "" || pair2.Card2.IsImage {
		t.Errorf("pair 2 CARD2: expected to stay text-only (no image field sent), got IMAGE=%q IS_IMAGE=%v", pair2.Card2.Image, pair2.Card2.IsImage)
	}
	if pair1.Card1.Image == pair1.Card2.Image {
		t.Errorf("pair 1's two cards must not share the same stored file: CARD1=%q CARD2=%q", pair1.Card1.Image, pair1.Card2.Image)
	}
}

// readWrittenMEMOTIONQuestion reads back the single question.json written
// under dataDir/files/questions by the test's upload request.
func readWrittenMEMOTIONQuestion(t *testing.T, dataDir string) game.Question {
	t.Helper()
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
	return q
}
