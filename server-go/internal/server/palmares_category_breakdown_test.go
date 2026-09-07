package server

// Tests for Lot A+1 (retour QUALIF v9.0.0.4) — handlePalmares (http.go)
// fans a GameEvent.CategoryBreakdown out across N palmarès categories;
// an event without one keeps today's single-category credit (non-régression).

import (
	"buzzcontrol/internal/game"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func decodePalmares(t *testing.T, srv *HTTPServer) []PalmaresEntry {
	t.Helper()
	req := httptest.NewRequest("GET", "/palmares", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /palmares: status %d (body: %s)", w.Code, w.Body.String())
	}
	var entries []PalmaresEntry
	if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil {
		t.Fatalf("decode /palmares response: %v (body: %s)", err, w.Body.String())
	}
	return entries
}

func entryFor(entries []PalmaresEntry, category string) *PalmaresEntry {
	for i := range entries {
		if entries[i].Category == category {
			return &entries[i]
		}
	}
	return nil
}

func TestPalmares_EventWithCategoryBreakdown_FansOutAcrossCategories(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)

	srv.engine.AddGameEvent(game.GameEvent{
		Timestamp:         1000000,
		QuestionID:        "rq1",
		QuestionCategory:  "", // #216-era defect: RAFALE events carried no category at all
		EventType:         "POINTS_AWARDED",
		WinnerType:        "TEAM",
		TeamName:          "Les Bleus",
		Points:            12,
		CategoryBreakdown: map[string]int{"HISTORY": 9, "SCIENCE": 3},
	})

	entries := decodePalmares(t, srv)

	hist := entryFor(entries, "HISTORY")
	sci := entryFor(entries, "SCIENCE")
	if hist == nil || hist.TotalPoints != 9 {
		t.Errorf("HISTORY entry = %+v, want TotalPoints=9", hist)
	}
	if sci == nil || sci.TotalPoints != 3 {
		t.Errorf("SCIENCE entry = %+v, want TotalPoints=3", sci)
	}
	if unk := entryFor(entries, "UNKNOWN"); unk != nil {
		t.Errorf("UNKNOWN entry present (%+v) — a CategoryBreakdown must fully replace the empty QUESTION_CATEGORY credit, not add to it", unk)
	}

	// The team's own points must also be split per category, not just the
	// category total (a team playing 2 categories must see 2 separate
	// per-team lines, one in HISTORY, one in SCIENCE).
	if len(hist.Teams) != 1 || hist.Teams[0].Points != 9 {
		t.Errorf("HISTORY.Teams = %+v, want [{Les Bleus, 9}]", hist.Teams)
	}
	if len(sci.Teams) != 1 || sci.Teams[0].Points != 3 {
		t.Errorf("SCIENCE.Teams = %+v, want [{Les Bleus, 3}]", sci.Teams)
	}
}

func TestPalmares_EventWithoutCategoryBreakdown_UnchangedSingleCategoryCredit(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)

	// A regular (non-RAFALE) event — no CategoryBreakdown at all — must
	// keep crediting QuestionCategory directly, exactly as before Lot A+1.
	srv.engine.AddGameEvent(game.GameEvent{
		Timestamp:        2000000,
		QuestionID:       "q2",
		QuestionCategory: "GEOGRAPHY",
		EventType:        "POINTS_AWARDED",
		WinnerType:       "TEAM",
		TeamName:         "Les Rouges",
		Points:           10,
	})

	entries := decodePalmares(t, srv)
	geo := entryFor(entries, "GEOGRAPHY")
	if geo == nil || geo.TotalPoints != 10 {
		t.Fatalf("GEOGRAPHY entry = %+v, want TotalPoints=10 (non-regression: no breakdown -> single-category credit unchanged)", geo)
	}
	if len(entries) != 1 {
		t.Errorf("got %d palmarès entries, want exactly 1 (no breakdown must never fan out)", len(entries))
	}
}

func TestPalmares_MixedEvents_BreakdownAndPlainCoexist(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)

	srv.engine.AddGameEvent(game.GameEvent{
		Timestamp: 1, QuestionCategory: "GEOGRAPHY", EventType: "POINTS_AWARDED",
		WinnerType: "TEAM", TeamName: "A", Points: 5,
	})
	srv.engine.AddGameEvent(game.GameEvent{
		Timestamp: 2, EventType: "POINTS_AWARDED", WinnerType: "TEAM", TeamName: "A",
		Points: 7, CategoryBreakdown: map[string]int{"GEOGRAPHY": 4, "HISTORY": 3},
	})

	entries := decodePalmares(t, srv)
	geo := entryFor(entries, "GEOGRAPHY")
	hist := entryFor(entries, "HISTORY")
	if geo == nil || geo.TotalPoints != 9 { // 5 (plain) + 4 (breakdown share)
		t.Errorf("GEOGRAPHY entry = %+v, want TotalPoints=9 (5 plain + 4 from breakdown)", geo)
	}
	if hist == nil || hist.TotalPoints != 3 {
		t.Errorf("HISTORY entry = %+v, want TotalPoints=3", hist)
	}
}
