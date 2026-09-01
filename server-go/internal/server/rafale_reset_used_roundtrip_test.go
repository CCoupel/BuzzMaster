package server

// ---------------------------------------------------------------------------
// Tests HTTP bout-en-bout supplémentaires : reset manuel du flag « déjà
// utilisée » RAFALE (#197), vus depuis GET /api/rafale/questions — la vraie
// vue que RafalePage.jsx recharge après un reset. rafale_reset_used_test.go
// vérifie déjà le routage et les compteurs via engine.CountRafalePool ; ce
// fichier complète en prouvant que la RELECTURE HTTP (pas un accès moteur
// direct) reflète bien USED:false après reset, que les deux endpoints sont
// des no-op HTTP propres (200, pas d'erreur) sur les cas déjà-dans-l'état-
// cible, et que le contenu de la question (QUESTION/ANSWER/CATEGORY/
// DIFFICULTY) est strictement inchangé après un reset — seul USED bouge.
// ---------------------------------------------------------------------------

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"buzzcontrol/internal/game"
)

type rafaleQuestionWire struct {
	ID         string `json:"ID"`
	Question   string `json:"QUESTION"`
	Answer     string `json:"ANSWER"`
	Category   string `json:"CATEGORY"`
	Difficulty int    `json:"DIFFICULTY"`
	Used       bool   `json:"USED"`
}

func getRafaleQuestionsHTTP(t *testing.T, server *HTTPServer) map[string]rafaleQuestionWire {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/rafale/questions", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/rafale/questions: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Questions []rafaleQuestionWire `json:"QUESTIONS"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse GET /api/rafale/questions response: %v (raw: %s)", err, w.Body.String())
	}
	byID := make(map[string]rafaleQuestionWire, len(resp.Questions))
	for _, q := range resp.Questions {
		byID[q.ID] = q
	}
	return byID
}

// TestHTTPServer_RafaleResetOneUsed_ReflectedInRealGETReread is the round-
// trip requested explicitly: reset via POST, then re-fetch via the actual
// GET /api/rafale/questions client-facing endpoint (not an internal engine
// accessor) and confirm USED flips to false while every other field of the
// question is byte-for-byte unchanged.
func TestHTTPServer_RafaleResetOneUsed_ReflectedInRealGETReread(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	if _, err := server.engine.UpsertRafaleQuestion(game.RafaleQuestion{
		ID: "r-1", Question: "Capitale du Pérou ?", Answer: "Lima", Category: game.CategoryGeography, Difficulty: 2,
	}); err != nil {
		t.Fatalf("seed reservoir: UpsertRafaleQuestion failed: %v", err)
	}
	if _, err := server.engine.DrawRafaleQuestion(string(game.CategoryGeography), 2); err != nil {
		t.Fatalf("seed used-flag: DrawRafaleQuestion failed: %v", err)
	}

	before := getRafaleQuestionsHTTP(t, server)["r-1"]
	if !before.Used {
		t.Fatalf("sanity: expected r-1 USED=true via GET before reset, got %+v", before)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/rafale/questions/r-1/reset", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST reset: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	after := getRafaleQuestionsHTTP(t, server)["r-1"]
	if after.Used {
		t.Errorf("expected r-1 USED=false via GET after reset, got %+v", after)
	}
	if after.Question != before.Question || after.Answer != before.Answer ||
		after.Category != before.Category || after.Difficulty != before.Difficulty {
		t.Errorf("expected question content unchanged by reset, before=%+v after=%+v", before, after)
	}
}

// TestHTTPServer_RafaleResetAllUsed_ReflectedInRealGETReread mirrors the
// above for the global endpoint: several questions drawn (used), one left
// untouched — after reset-all, GET must show every question USED=false and
// every field besides USED byte-for-byte identical to before.
func TestHTTPServer_RafaleResetAllUsed_ReflectedInRealGETReread(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	seed := []game.RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: game.CategoryHistory, Difficulty: 1},
		{ID: "r-2", Question: "Q2", Answer: "A2", Category: game.CategoryHistory, Difficulty: 1},
		{ID: "r-3", Question: "Q3", Answer: "A3", Category: game.CategoryHistory, Difficulty: 1},
	}
	for _, q := range seed {
		if _, err := server.engine.UpsertRafaleQuestion(q); err != nil {
			t.Fatalf("seed reservoir: UpsertRafaleQuestion(%s) failed: %v", q.ID, err)
		}
	}
	// Draw twice: DrawRafaleQuestion picks randomly among the still-available
	// pool, so which 2 of the 3 IDs end up used is non-deterministic — assert
	// on the count (2 used, 1 available), not on specific IDs.
	if _, err := server.engine.DrawRafaleQuestion(string(game.CategoryHistory), 1); err != nil {
		t.Fatalf("seed used-flag: DrawRafaleQuestion failed: %v", err)
	}
	if _, err := server.engine.DrawRafaleQuestion(string(game.CategoryHistory), 1); err != nil {
		t.Fatalf("seed used-flag: DrawRafaleQuestion failed: %v", err)
	}

	before := getRafaleQuestionsHTTP(t, server)
	usedCountBefore := 0
	for _, id := range []string{"r-1", "r-2", "r-3"} {
		if before[id].Used {
			usedCountBefore++
		}
	}
	if usedCountBefore != 2 {
		t.Fatalf("sanity: expected exactly 2 of 3 questions USED=true before reset-all, got %d (r-1=%v r-2=%v r-3=%v)",
			usedCountBefore, before["r-1"].Used, before["r-2"].Used, before["r-3"].Used)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/rafale/questions/reset-all", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST reset-all: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	after := getRafaleQuestionsHTTP(t, server)
	for _, id := range []string{"r-1", "r-2", "r-3"} {
		if after[id].Used {
			t.Errorf("expected %s USED=false via GET after reset-all, got %+v", id, after[id])
		}
		b, a := before[id], after[id]
		if a.Question != b.Question || a.Answer != b.Answer || a.Category != b.Category || a.Difficulty != b.Difficulty {
			t.Errorf("expected %s content unchanged by reset-all, before=%+v after=%+v", id, b, a)
		}
	}
}

// TestHTTPServer_RafaleResetOneUsed_AlreadyAvailable_HTTPNoOp confirms the
// HTTP layer itself (not just the engine method underneath) returns a clean
// 200 — not a 404/409/500 — when resetting a question that was never used.
func TestHTTPServer_RafaleResetOneUsed_AlreadyAvailable_HTTPNoOp(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	if _, err := server.engine.UpsertRafaleQuestion(game.RafaleQuestion{
		ID: "r-1", Question: "Q1", Answer: "A1", Category: game.CategoryHistory, Difficulty: 1,
	}); err != nil {
		t.Fatalf("seed reservoir: UpsertRafaleQuestion failed: %v", err)
	}
	// r-1 never drawn — already available.

	req := httptest.NewRequest(http.MethodPost, "/api/rafale/questions/r-1/reset", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (no-op, not an error) resetting an already-available question, got %d: %s", w.Code, w.Body.String())
	}
	if after := getRafaleQuestionsHTTP(t, server)["r-1"]; after.Used {
		t.Errorf("expected r-1 still USED=false after a no-op reset, got %+v", after)
	}
}

// TestHTTPServer_RafaleResetAllUsed_NothingUsed_ReturnsResetZero confirms
// RESET:0 (not an error) is the real wire response when nothing was used.
func TestHTTPServer_RafaleResetAllUsed_NothingUsed_ReturnsResetZero(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	if _, err := server.engine.UpsertRafaleQuestion(game.RafaleQuestion{
		ID: "r-1", Question: "Q1", Answer: "A1", Category: game.CategoryHistory, Difficulty: 1,
	}); err != nil {
		t.Fatalf("seed reservoir: UpsertRafaleQuestion failed: %v", err)
	}
	// Nothing drawn — the used-flag is empty.

	req := httptest.NewRequest(http.MethodPost, "/api/rafale/questions/reset-all", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Reset int `json:"RESET"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v (raw: %s)", err, w.Body.String())
	}
	if resp.Reset != 0 {
		t.Errorf("expected RESET=0 when nothing was used, got %d", resp.Reset)
	}
}
