package game

import "testing"

// ---------------------------------------------------------------------------
// Tests : reset manuel du flag « déjà utilisée » RAFALE — individuel
// (MarkRafaleQuestionAvailable) et global (ResetAllRafaleUsed) — milestone
// v8.0.0, issue #197, contrat contracts/rafale.md §9. Retour utilisateur
// après test manuel du binaire QUALIF 8.0.0.12 : le seul point de remise à
// zéro existant du flag était NEW_GAME (automatique) — ces deux méthodes
// ajoutent des points d'entrée manuels, SANS jamais toucher au réservoir
// lui-même (ni créer, ni modifier, ni supprimer une question) — à ne pas
// confondre avec ClearRafaleReservoir/ClearRafaleUsed (utilisées par la
// réinitialisation sélective destructive /reset-select?rafale=true, qui
// purge le réservoir EN PLUS du flag).
//
// Suit le patron de rafale_test.go (Batch 1) : seedRafaleReservoir (aucun
// rafalePath défini, donc SaveRafale no-op — pas d'E/S disque) et
// drawRafaleQuestionNoAutoSave pour marquer une question utilisée sans
// déclencher la sauvegarde asynchrone de fond (safeGo) que DrawRafaleQuestion
// utilise normalement.
// ---------------------------------------------------------------------------

func TestMarkRafaleQuestionAvailable_RemovesFromUsedFlag(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 1},
	})
	drawn := drawRafaleQuestionNoAutoSave(t, e, string(CategoryHistory), 1)
	if drawn.ID != "r-1" {
		t.Fatalf("sanity: expected to draw r-1, got %q", drawn.ID)
	}

	// Sanity: r-1 is now marked used, and the pool for this filter is empty.
	available, used, total := e.CountRafalePool(string(CategoryHistory), 1)
	if available != 0 || used != 1 || total != 1 {
		t.Fatalf("sanity: expected available=0 used=1 total=1 after draw, got available=%d used=%d total=%d", available, used, total)
	}

	if err := e.MarkRafaleQuestionAvailable("r-1"); err != nil {
		t.Fatalf("MarkRafaleQuestionAvailable failed: %v", err)
	}

	available, used, total = e.CountRafalePool(string(CategoryHistory), 1)
	if available != 1 || used != 0 || total != 1 {
		t.Errorf("expected available=1 used=0 total=1 after reset, got available=%d used=%d total=%d", available, used, total)
	}

	// The reservoir question itself must be untouched (still exists, same content).
	q, ok := e.GetRafaleQuestion("r-1")
	if !ok {
		t.Fatal("expected r-1 to still exist in the reservoir after a used-flag reset")
	}
	if q.Question != "Q1" || q.Answer != "A1" {
		t.Errorf("expected r-1's content unchanged, got Question=%q Answer=%q", q.Question, q.Answer)
	}
}

func TestMarkRafaleQuestionAvailable_UnknownID_ReturnsErrRafaleQuestionNotFound(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 1},
	})

	err := e.MarkRafaleQuestionAvailable("ghost")
	if err != ErrRafaleQuestionNotFound {
		t.Errorf("expected ErrRafaleQuestionNotFound for an unknown ID, got %v", err)
	}
}

func TestMarkRafaleQuestionAvailable_AlreadyAvailable_IsANoOpNotAnError(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 1},
	})

	// r-1 was never drawn — never marked used.
	if err := e.MarkRafaleQuestionAvailable("r-1"); err != nil {
		t.Errorf("expected no error resetting an already-available question, got %v", err)
	}

	available, _, _ := e.CountRafalePool(string(CategoryHistory), 1)
	if available != 1 {
		t.Errorf("expected r-1 still available, got available=%d", available)
	}
}

func TestResetAllRafaleUsed_ClearsEveryEntry_ReservoirUntouched(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 1},
		{ID: "r-2", Question: "Q2", Answer: "A2", Category: CategoryHistory, Difficulty: 1},
		{ID: "r-3", Question: "Q3", Answer: "A3", Category: CategoryHistory, Difficulty: 1},
	})
	drawRafaleQuestionNoAutoSave(t, e, string(CategoryHistory), 1)
	drawRafaleQuestionNoAutoSave(t, e, string(CategoryHistory), 1)
	// r-3 deliberately left available, to prove the count reflects only what
	// was ACTUALLY cleared, not the full reservoir size.

	available, used, total := e.CountRafalePool(string(CategoryHistory), 1)
	if available != 1 || used != 2 || total != 3 {
		t.Fatalf("sanity: expected available=1 used=2 total=3 before reset, got available=%d used=%d total=%d", available, used, total)
	}

	n, err := e.ResetAllRafaleUsed()
	if err != nil {
		t.Fatalf("ResetAllRafaleUsed failed: %v", err)
	}
	if n != 2 {
		t.Errorf("expected ResetAllRafaleUsed to report 2 cleared entries, got %d", n)
	}

	available, used, total = e.CountRafalePool(string(CategoryHistory), 1)
	if available != 3 || used != 0 || total != 3 {
		t.Errorf("expected available=3 used=0 total=3 after global reset, got available=%d used=%d total=%d", available, used, total)
	}

	// The reservoir itself (all 3 questions) must be untouched — this is
	// NOT the destructive /reset-select?rafale=true path.
	for _, id := range []string{"r-1", "r-2", "r-3"} {
		if _, ok := e.GetRafaleQuestion(id); !ok {
			t.Errorf("expected %s to still exist in the reservoir after a used-flag reset", id)
		}
	}
}

func TestResetAllRafaleUsed_EmptyFlag_ReturnsZero(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "A1", Category: CategoryHistory, Difficulty: 1},
	})

	n, err := e.ResetAllRafaleUsed()
	if err != nil {
		t.Fatalf("ResetAllRafaleUsed failed: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 cleared entries on an already-empty flag, got %d", n)
	}
}
