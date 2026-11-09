package rounds

import (
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
)

// #291 Phase B : tests unitaires du paquet extrait (fonctions pures, sans App). Les scénarios complets
// ROUNDS_UPDATE (rounds_*_245_test.go) passent par l'App et restent dans cmd/server.

func TestComputeRoundContext_NoRoundOrNoCurrentIsZero(t *testing.T) {
	t.Parallel()
	ordered := []server.OrderedQuestion{{ID: "1", Group: 1, Type: "SPEEDY"}}
	if got := ComputeRoundContext(ordered, nil, "1"); got != (game.RoundContext{}) {
		t.Errorf("sans manche : %+v", got)
	}
	if got := ComputeRoundContext(ordered, []game.Round{{ID: "r1"}}, ""); got != (game.RoundContext{}) {
		t.Errorf("sans question courante : %+v", got)
	}
	if got := ComputeRoundContext(ordered, []game.Round{{ID: "r1"}}, "inconnue"); got != (game.RoundContext{}) {
		t.Errorf("question inconnue : %+v", got)
	}
}

func TestComputeRoundContext_PositionTotalAndFirstOfRoundIgnoreEntractes(t *testing.T) {
	t.Parallel()
	ordered := []server.OrderedQuestion{
		{ID: "e1", Group: 1, RoundID: "r1", Type: string(game.QuestionTypeEntracte)},
		{ID: "q1", Group: 1, RoundID: "r1", Type: "SPEEDY"},
		{ID: "q2", Group: 1, RoundID: "r1", Type: "SPEEDY"},
		{ID: "q3", Group: 2, RoundID: "r2", Type: "SPEEDY"},
	}
	rs := []game.Round{{ID: "r1"}, {ID: "r2"}}

	got := ComputeRoundContext(ordered, rs, "q1")
	if got.RoundID != "r1" || got.RoundCount != 2 || got.Total != 2 || got.Position != 1 || !got.FirstOfRound {
		t.Errorf("q1 : %+v", got)
	}
	got = ComputeRoundContext(ordered, rs, "q2")
	if got.Position != 2 || got.FirstOfRound {
		t.Errorf("q2 : %+v", got)
	}
	got = ComputeRoundContext(ordered, rs, "e1")
	if got.Position != 0 || got.FirstOfRound || got.Total != 2 {
		t.Errorf("une entracte n'a ni position ni FirstOfRound : %+v", got)
	}
}

func TestAllocateQuestionDir_TakesTheLowestFreeNumericID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, n := range []string{"1", "3", "texte"} {
		if err := os.Mkdir(filepath.Join(dir, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if taken := takenQuestionIDs(dir); !taken[1] || !taken[3] || taken[2] || len(taken) != 2 {
		t.Errorf("takenQuestionIDs = %v", taken)
	}
	id, p, err := allocateQuestionDir(dir)
	if err != nil || id != "2" || p != filepath.Join(dir, "2") {
		t.Fatalf("allocateQuestionDir = %q %q %v, attendu l'id 2", id, p, err)
	}
	if st, err := os.Stat(p); err != nil || !st.IsDir() {
		t.Errorf("le dossier réservé doit exister : %v", err)
	}
	if id2, _, _ := allocateQuestionDir(dir); id2 != "4" {
		t.Errorf("deuxième réservation = %q, attendu 4", id2)
	}
}

func TestCloneQuestion_IsAShallowCopy(t *testing.T) {
	t.Parallel()
	orig := map[string]interface{}{"ID": "1", "QUESTION": "?"}
	c := cloneQuestion(orig)
	c["QUESTION"] = "autre"
	if orig["QUESTION"] != "?" || c["ID"] != "1" {
		t.Errorf("orig=%v clone=%v", orig, c)
	}
}

func TestRoundResult_SlicesAreNeverNil(t *testing.T) {
	t.Parallel()
	r := RoundResult("REFUSED", "INVALID_NAME")
	if r.Status != "REFUSED" || r.Reason != "INVALID_NAME" || r.Rounds == nil || r.Questions == nil || r.CreatedEntractes == nil || r.DeletedQuestions == nil {
		t.Errorf("RoundResult = %+v (les listes doivent être sérialisées « [] », jamais null)", r)
	}
}
