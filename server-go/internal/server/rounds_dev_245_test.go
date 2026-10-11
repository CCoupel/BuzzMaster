package server

import (
	"buzzcontrol/internal/game"
	"testing"
)

// #245: a non-numeric ID without a valid ORDER has NO rank and sorts AFTER every ranked question of
// its group (contracts/models.md §Manches); ORDER normalisation is the documented one.
func TestOrderQuestions_UnrankedQuestionsComeAfterTheRankedOnes(t *testing.T) {
	raw := []map[string]interface{}{
		{"ID": "abc"},
		{"ID": "z", "ORDER": " 2 "},
		{"ID": "7"},
		{"ID": "b", "ORDER": "5abc"}, // invalid string, non-numeric ID: unranked
		{"ID": "9", "ORDER": nil},    // null: falls back on the numeric ID
		{"ID": "k", "ORDER": float64(-1.9)},
	}
	got := OrderQuestions(raw, []game.Round{})
	var ids []string
	for _, q := range got {
		ids = append(ids, q.ID)
	}
	want := []string{"k", "z", "7", "9", "abc", "b"}
	// k: ORDER -1 (truncated), z: 2, 7: 7, 9: 9 (null -> ID), then the unranked abc, b (lexicographic)
	if len(ids) != len(want) {
		t.Fatalf("got %v", ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ordre = %v, attendu %v", ids, want)
		}
	}
}
