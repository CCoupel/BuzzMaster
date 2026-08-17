package server

import (
	"buzzcontrol/internal/game"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests: #170 — crédit à 0 point ("0 pt", geste de refus), T3 (volet
// historique/PALMARÈS). Complète awarded_teams_zero_point_test.go
// (cmd/server — événement enregistré, score inchangé, aucune LED).
//
// Confirme un comportement déjà présent, inchangé par #170 (plan
// _work/reports/plan-20260816-125123.md §2) : handlePalmares filtre déjà
// les événements à montant nul ou négatif (http.go, `event.Points <= 0`),
// GET /history ne filtre rien — un refus y apparaît comme n'importe quel
// autre événement, trace explicite qu'une réponse a été lue et écartée
// (arbitrage C1 du plan : conservée, pas masquée).
// ---------------------------------------------------------------------------

func TestZeroPointCredit_AbsentFromPalmares(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)

	srv.engine.AddGameEvent(game.GameEvent{
		Timestamp: 1000, QuestionCategory: "SCIENCE",
		EventType: "POINTS_AWARDED", WinnerType: "TEAM",
		TeamName: "Les Bleus", TeamColor: []int{59, 130, 246}, Points: 0, // refus
	})

	req := httptest.NewRequest("GET", "/palmares", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /palmares: status %d — body: %s", w.Code, w.Body.String())
	}

	var entries []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected an empty PALMARÈS (the only event is a 0-point refusal), got %d entries: %+v", len(entries), entries)
	}
}

func TestZeroPointCredit_AbsentFromPalmares_MixedWithRealCredit(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)

	srv.engine.AddGameEvent(game.GameEvent{
		Timestamp: 1000, QuestionCategory: "SCIENCE",
		EventType: "POINTS_AWARDED", WinnerType: "TEAM",
		TeamName: "Les Bleus", TeamColor: []int{59, 130, 246}, Points: 10, // crédité
	})
	srv.engine.AddGameEvent(game.GameEvent{
		Timestamp: 2000, QuestionCategory: "SCIENCE",
		EventType: "POINTS_AWARDED", WinnerType: "TEAM",
		TeamName: "Les Rouges", TeamColor: []int{239, 68, 68}, Points: 0, // refusé
	})

	req := httptest.NewRequest("GET", "/palmares", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	var entries []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 PALMARÈS category (only the credited event counts), got %d: %+v", len(entries), entries)
	}
	teams, _ := entries[0]["teams"].([]interface{})
	if len(teams) != 1 {
		t.Errorf("expected only Les Bleus (credited) in the PALMARÈS team list, got %+v", teams)
	}
}

func TestZeroPointCredit_PresentInHistory(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)

	srv.engine.AddGameEvent(game.GameEvent{
		Timestamp: 1000, QuestionID: "1", QuestionCategory: "SCIENCE",
		EventType: "POINTS_AWARDED", WinnerType: "TEAM",
		TeamName: "Les Bleus", TeamColor: []int{59, 130, 246}, Points: 0, // refus
	})

	req := httptest.NewRequest("GET", "/history", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /history: status %d — body: %s", w.Code, w.Body.String())
	}

	var events []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&events); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected the 0-point refusal to appear in GET /history (arbitrage C1: conservée), got %d events", len(events))
	}
	if events[0]["TEAM_NAME"] != "Les Bleus" {
		t.Errorf("TEAM_NAME = %v, want %q", events[0]["TEAM_NAME"], "Les Bleus")
	}
	if pts, ok := events[0]["POINTS"].(float64); !ok || pts != 0 {
		t.Errorf("POINTS = %v, want 0", events[0]["POINTS"])
	}
}
