package server

// Tests pour le plafond de longueur [BREAKING mineur] introduit par #203
// (milestone v8.1.0) sur POST /api/rafale/questions — contrat
// contracts/rafale-ai-generation.md §5.3 : les mêmes plafonds que la
// génération IA (rafaleMaxQuestionRunes=100, rafaleMaxAnswerRunes=40,
// ai_generator_rafale.go) s'appliquent désormais à l'éditeur manuel, pour
// qu'un texte ne puisse pas contourner la contrainte "en trois clics".
//
// ⚠️ Aucune donnée existante n'est supprimée ni tronquée par ce changement
// (contract §9) — une question déjà en base plus longue que le plafond reste
// jouable et lisible en LECTURE (GET), elle ne peut simplement plus être
// RÉ-ENREGISTRÉE telle quelle. Vérifié explicitement ci-dessous.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
)

func postRafaleQuestionJSON(server *HTTPServer, body map[string]interface{}) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/rafale/questions", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	return w
}

func validRafaleQuestionBody() map[string]interface{} {
	return map[string]interface{}{
		"QUESTION":   "Une question valide ?",
		"ANSWER":     "Oui",
		"CATEGORY":   string(game.CategoryHistory),
		"DIFFICULTY": 1,
	}
}

func TestPostRafaleQuestion_QuestionAtCap_Accepted(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	body := validRafaleQuestionBody()
	body["QUESTION"] = strings.Repeat("a", 100) // exactly 100 runes
	w := postRafaleQuestionJSON(server, body)
	if w.Code != http.StatusOK {
		t.Fatalf("a 100-rune QUESTION (exactly at the cap) must be accepted, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPostRafaleQuestion_QuestionOverCap_Rejected400(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	body := validRafaleQuestionBody()
	body["QUESTION"] = strings.Repeat("a", 101) // one over the cap
	w := postRafaleQuestionJSON(server, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a 101-rune QUESTION must be rejected with 400 (contract §5.3, BREAKING mineur), got %d: %s", w.Code, w.Body.String())
	}
}

func TestPostRafaleQuestion_AnswerAtCap_Accepted(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	body := validRafaleQuestionBody()
	body["ANSWER"] = strings.Repeat("a", 40) // exactly 40 runes
	w := postRafaleQuestionJSON(server, body)
	if w.Code != http.StatusOK {
		t.Fatalf("a 40-rune ANSWER (exactly at the cap) must be accepted, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPostRafaleQuestion_AnswerOverCap_Rejected400(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	body := validRafaleQuestionBody()
	body["ANSWER"] = strings.Repeat("a", 41) // one over the cap
	w := postRafaleQuestionJSON(server, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a 41-rune ANSWER must be rejected with 400 (contract §5.3, BREAKING mineur), got %d: %s", w.Code, w.Body.String())
	}
}

// 🔴 Runes, not bytes — same rule as the generation path's validator
// (contract §5.1 "jamais en octets"), applied here to the manual editor.
func TestPostRafaleQuestion_LengthCaps_RunesNotBytes(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	body := validRafaleQuestionBody()
	// 100 accented runes = 200 UTF-8 bytes — must be ACCEPTED (rune count).
	body["QUESTION"] = strings.Repeat("é", 100)
	w := postRafaleQuestionJSON(server, body)
	if w.Code != http.StatusOK {
		t.Fatalf("a 100-rune accented QUESTION (200 bytes) must be accepted (rune count, not byte count), got %d: %s", w.Code, w.Body.String())
	}

	server2, _ := setupTestHTTPServer(t)
	body2 := validRafaleQuestionBody()
	body2["QUESTION"] = strings.Repeat("é", 101)
	w2 := postRafaleQuestionJSON(server2, body2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("a 101-rune accented QUESTION must be rejected, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestPostRafaleQuestion_Update_AlsoEnforcesLengthCaps(t *testing.T) {
	// The plafond applies to BOTH create (no ID) and update (ID present) —
	// contract §5.3 doesn't distinguish the two, and the whole point is that
	// the constraint can't be bypassed "en trois clics" via an edit either.
	server, _ := setupTestHTTPServer(t)
	created, err := server.engine.UpsertRafaleQuestion(game.RafaleQuestion{
		Question: "Courte", Answer: "OK", Category: game.CategoryHistory, Difficulty: 1,
	})
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	body := validRafaleQuestionBody()
	body["ID"] = created.ID
	body["QUESTION"] = strings.Repeat("a", 101)
	w := postRafaleQuestionJSON(server, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("updating an existing question with an over-cap QUESTION must be rejected with 400, got %d: %s", w.Code, w.Body.String())
	}

	// The existing entry must be untouched by the rejected update attempt.
	unchanged, ok := server.engine.GetRafaleQuestion(created.ID)
	if !ok || unchanged.Question != "Courte" {
		t.Errorf("a rejected update must not mutate the existing entry, got %+v (ok=%v)", unchanged, ok)
	}
}

// TestPostRafaleQuestion_ExistingOverCapQuestion_StillReadableNeverTruncated
// verifies contract §9's migration guarantee: a question already in the
// reservoir with a length exceeding the NEW cap (written before v8.1.0, or
// seeded directly via the engine, bypassing the HTTP validation entirely)
// is NEVER deleted or truncated by the new constraint — it remains fully
// readable via GET, unmodified. The cap applies to writes, not to what
// already exists.
func TestPostRafaleQuestion_ExistingOverCapQuestion_StillReadableNeverTruncated(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	longQuestion := strings.Repeat("a", 150) // well over the 100-rune cap
	seeded, err := server.engine.UpsertRafaleQuestion(game.RafaleQuestion{
		Question: longQuestion, Answer: "OK", Category: game.CategoryHistory, Difficulty: 1,
	})
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/rafale/questions", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/rafale/questions failed: %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Questions []rafaleQuestionWire `json:"QUESTIONS"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	found := false
	for _, q := range resp.Questions {
		if q.ID == seeded.ID {
			found = true
			if q.Question != longQuestion {
				t.Errorf("an existing over-cap question must be returned VERBATIM, never truncated — got %q (len=%d), want %q (len=%d)", q.Question, len(q.Question), longQuestion, len(longQuestion))
			}
		}
	}
	if !found {
		t.Fatalf("expected the seeded over-cap question %q to still be present in the reservoir", seeded.ID)
	}
}
