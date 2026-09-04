package server

// Test for #216 Lot 1B task 10 — GET /api/rafale/pool becomes plural:
// ?categories=A,B&difficulties=1,2 instead of the current
// ?category=A&difficulty=1. Written contract-first, in parallel with
// dev-backend's implementation of internal/server/http.go's
// handleRafalePool (currently singular-only).
//
// Query-string format mirrors the ALREADY-established sibling endpoint
// handleGetRafaleQuestions (GET /api/rafale/questions?categories=A,B,
// comma-separated, strings.Split + TrimSpace) — same file, same package,
// same convention — rather than inventing a new parsing style for this
// endpoint alone.

import (
	"buzzcontrol/internal/game"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleRafalePool_PluralParams verifies that the endpoint accepts
// comma-separated categories/difficulties and reports the union count
// across every requested couple (available/used/total), consistent with
// the engine-level union semantics tested in
// internal/game/rafale_multi_216_test.go.
func TestHandleRafalePool_PluralParams(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	seed := []game.RafaleQuestion{
		{ID: "h1-1", Question: "Q", Answer: "A", Category: game.CategoryHistory, Difficulty: 1},
		{ID: "h1-2", Question: "Q", Answer: "A", Category: game.CategoryHistory, Difficulty: 1},
		{ID: "h2-1", Question: "Q", Answer: "A", Category: game.CategoryHistory, Difficulty: 2},
		{ID: "s1-1", Question: "Q", Answer: "A", Category: game.CategoryScience, Difficulty: 1},
		{ID: "decoy-1", Question: "Q", Answer: "A", Category: game.CategorySports, Difficulty: 1}, // outside the request
	}
	for _, q := range seed {
		if _, err := server.engine.UpsertRafaleQuestion(q); err != nil {
			t.Fatalf("UpsertRafaleQuestion(%q) failed: %v", q.ID, err)
		}
	}

	req := httptest.NewRequest("GET", "/api/rafale/pool?categories=HISTORY,SCIENCE&difficulties=1,2", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	var got map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("could not decode response body %q: %v", w.Body.String(), err)
	}

	// h1-1, h1-2, h2-1, s1-1 = 4 questions across the requested union;
	// decoy-1 (SPORTS/1) must NOT be counted.
	if got["AVAILABLE"] != 4 {
		t.Errorf("AVAILABLE = %d, want 4 (union of HISTORY×{1,2} ∪ SCIENCE×{1,2}, decoy excluded)", got["AVAILABLE"])
	}
	if got["TOTAL"] != 4 {
		t.Errorf("TOTAL = %d, want 4", got["TOTAL"])
	}
	if got["USED"] != 0 {
		t.Errorf("USED = %d, want 0 (nothing drawn yet)", got["USED"])
	}
}

// TestHandleRafalePool_PluralParams_MissingRequired verifies the endpoint
// still rejects a request missing categories/difficulties, matching the
// existing singular endpoint's "category required" / "Invalid difficulty"
// validation discipline — just renamed to the plural params.
func TestHandleRafalePool_PluralParams_MissingRequired(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("GET", "/api/rafale/pool", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 when categories/difficulties are both absent (body: %s)", w.Code, w.Body.String())
	}
}
