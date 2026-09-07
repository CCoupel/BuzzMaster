package server

// ---------------------------------------------------------------------------
// Tests HTTP : reset manuel du flag « déjà utilisée » RAFALE — individuel
// (POST /api/rafale/questions/{id}/reset) et global
// (POST /api/rafale/questions/reset-all) — milestone v8.0.0, issue #197,
// contrat contracts/rafale.md §9. Complète les tests moteur de
// internal/game/rafale_reset_used_test.go en vérifiant le VRAI routage
// net/http.ServeMux (registration exacte "/api/rafale/questions/reset-all"
// avant le préfixe "/api/rafale/questions/", et parsing du suffixe "/reset"
// dans handleRafaleQuestionByID) — un appel direct à la méthode Engine ne
// peut pas détecter une erreur de route.
// ---------------------------------------------------------------------------

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"buzzcontrol/internal/game"
)

func TestHTTPServer_RafaleResetOneUsed_MakesQuestionAvailableAgain(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	if _, err := server.engine.UpsertRafaleQuestion(game.RafaleQuestion{
		ID: "r-1", Question: "Q1", Answer: "A1", Category: game.CategoryHistory, Difficulty: 1,
	}); err != nil {
		t.Fatalf("seed reservoir: UpsertRafaleQuestion failed: %v", err)
	}
	if _, err := server.engine.DrawRafaleQuestion([]string{string(game.CategoryHistory)}, []int{1}); err != nil {
		t.Fatalf("seed used-flag: DrawRafaleQuestion failed: %v", err)
	}
	if available, _, _ := server.engine.CountRafalePool([]string{string(game.CategoryHistory)}, []int{1}); available != 0 {
		t.Fatalf("sanity: expected r-1 marked used (available=0), got available=%d", available)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/rafale/questions/r-1/reset", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID        string `json:"ID"`
		Available bool   `json:"AVAILABLE"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v (raw: %s)", err, w.Body.String())
	}
	if resp.ID != "r-1" || !resp.Available {
		t.Errorf("expected {ID:r-1 AVAILABLE:true}, got %+v", resp)
	}

	if available, used, _ := server.engine.CountRafalePool([]string{string(game.CategoryHistory)}, []int{1}); available != 1 || used != 0 {
		t.Errorf("expected available=1 used=0 after reset, got available=%d used=%d", available, used)
	}
}

func TestHTTPServer_RafaleResetOneUsed_UnknownID_Returns404(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/rafale/questions/ghost/reset", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for an unknown ID, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHTTPServer_RafaleResetOneUsed_WrongMethod_Returns405(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	if _, err := server.engine.UpsertRafaleQuestion(game.RafaleQuestion{
		ID: "r-1", Question: "Q1", Answer: "A1", Category: game.CategoryHistory, Difficulty: 1,
	}); err != nil {
		t.Fatalf("seed reservoir: UpsertRafaleQuestion failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/rafale/questions/r-1/reset", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET on a reset endpoint, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHTTPServer_RafaleResetOneUsed_DoesNotShadowDelete is the key routing
// regression this file exists to catch: handleRafaleQuestionByID parses
// BOTH "/{id}" (DELETE) and "/{id}/reset" (POST) from the same
// "/api/rafale/questions/" prefix route — a parsing mistake could make one
// swallow the other silently.
func TestHTTPServer_RafaleResetOneUsed_DoesNotShadowDelete(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	if _, err := server.engine.UpsertRafaleQuestion(game.RafaleQuestion{
		ID: "r-1", Question: "Q1", Answer: "A1", Category: game.CategoryHistory, Difficulty: 1,
	}); err != nil {
		t.Fatalf("seed reservoir: UpsertRafaleQuestion failed: %v", err)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/rafale/questions/r-1", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, deleteReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected DELETE /api/rafale/questions/r-1 to still work (200), got %d: %s", w.Code, w.Body.String())
	}
	if _, ok := server.engine.GetRafaleQuestion("r-1"); ok {
		t.Error("expected r-1 to actually be deleted from the reservoir")
	}
}

func TestHTTPServer_RafaleResetAllUsed_ClearsEveryEntry_ReservoirUntouched(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	for _, id := range []string{"r-1", "r-2"} {
		if _, err := server.engine.UpsertRafaleQuestion(game.RafaleQuestion{
			ID: id, Question: "Q", Answer: "A", Category: game.CategoryHistory, Difficulty: 1,
		}); err != nil {
			t.Fatalf("seed reservoir: UpsertRafaleQuestion(%s) failed: %v", id, err)
		}
	}
	if _, err := server.engine.DrawRafaleQuestion([]string{string(game.CategoryHistory)}, []int{1}); err != nil {
		t.Fatalf("seed used-flag: DrawRafaleQuestion failed: %v", err)
	}
	if _, err := server.engine.DrawRafaleQuestion([]string{string(game.CategoryHistory)}, []int{1}); err != nil {
		t.Fatalf("seed used-flag: DrawRafaleQuestion failed: %v", err)
	}
	if available, used, _ := server.engine.CountRafalePool([]string{string(game.CategoryHistory)}, []int{1}); available != 0 || used != 2 {
		t.Fatalf("sanity: expected available=0 used=2 before reset, got available=%d used=%d", available, used)
	}

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
	if resp.Reset != 2 {
		t.Errorf("expected RESET=2, got %d", resp.Reset)
	}

	if available, used, total := server.engine.CountRafalePool([]string{string(game.CategoryHistory)}, []int{1}); available != 2 || used != 0 || total != 2 {
		t.Errorf("expected available=2 used=0 total=2 after global reset, got available=%d used=%d total=%d", available, used, total)
	}
	for _, id := range []string{"r-1", "r-2"} {
		if _, ok := server.engine.GetRafaleQuestion(id); !ok {
			t.Errorf("expected %s to still exist in the reservoir after a used-flag reset", id)
		}
	}
}

func TestHTTPServer_RafaleResetAllUsed_WrongMethod_Returns405(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/rafale/questions/reset-all", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET on reset-all, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHTTPServer_RafaleResetAllUsed_ExactRouteWinsOverPrefix is the key
// routing regression for the reset-all endpoint specifically: it MUST route
// to handleRafaleResetAllUsed (exact registration), not be swallowed by the
// "/api/rafale/questions/" prefix route as if "reset-all" were a question ID.
func TestHTTPServer_RafaleResetAllUsed_ExactRouteWinsOverPrefix(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/rafale/questions/reset-all", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (routed to handleRafaleResetAllUsed), got %d: %s — likely misrouted to handleRafaleQuestionByID (DELETE-only, 'reset-all' treated as an unknown ID)", w.Code, w.Body.String())
	}
	var resp struct {
		Reset int `json:"RESET"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v (raw: %s) — a {\"DELETED\":...} or 404 body here would confirm misrouting", err, w.Body.String())
	}
}
