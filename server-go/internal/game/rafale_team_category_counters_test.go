package game

// Tests for Lot A+1 (retour QUALIF v9.0.0.4) — RafaleTeamCategoryCounters
// is ephemeral, exactly like the 13 other RAFALE_* global fields: excluded
// from PersistedGameState, reset at the same points in the engine lifecycle.

import (
	"path/filepath"
	"testing"
)

func TestRafaleTeamCategoryCounters_ExcludedFromPersistedGameState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "game_state.json")

	e := NewEngine()
	e.SetStatePath(statePath)
	e.mu.Lock()
	e.state.RafaleTeamCategoryCounters = map[string]map[string]int{"red": {"HISTORY": 3}}
	e.mu.Unlock()

	if err := e.SaveState(); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	e2 := NewEngine()
	e2.SetStatePath(statePath)
	if err := e2.LoadState(); err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	if got := e2.GetState().RafaleTeamCategoryCounters; len(got) != 0 {
		t.Errorf("RafaleTeamCategoryCounters survived a save+load round-trip: %v, want empty — this field must be ephemeral, like every other RAFALE_* global", got)
	}
}

func TestRafaleTeamCategoryCounters_ResetOnNewQuestionReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	seedRafaleReservoirCouple(t, e, "h", 10, CategoryHistory, 1)

	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready(q.ID, q)
	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	e.StartImmediate(0)
	if err := e.RafaleValidate(); err != nil {
		t.Fatalf("RafaleValidate: %v", err)
	}
	if e.GetState().RafaleTeamCategoryCounters["red"]["HISTORY"] == 0 {
		t.Fatal("setup: RafaleTeamCategoryCounters[red][HISTORY] still 0 after 1 correct answer")
	}
	e.Stop()

	// Readying a NEW question must reset it, same as RafaleTeamCounters and
	// the other 12 global RAFALE_* fields (engine.go's own
	// `isNewQuestion || rafaleRoundAlreadyPlayed` reset block).
	other := &Question{ID: "q-other", Type: QuestionTypeSpeedy, Category: CategoryHistory}
	e.Ready(other.ID, other)
	defer e.Stop()

	if got := e.GetState().RafaleTeamCategoryCounters; len(got) != 0 {
		t.Errorf("RafaleTeamCategoryCounters = %v after readying a new question, want empty (reset)", got)
	}
}
