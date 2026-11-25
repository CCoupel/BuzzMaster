package game

import (
	"os"
	"path/filepath"
	"testing"
)

// LOT-2 (v13.1.0, issue fille #324) — T16 : NON-RÉGRESSION RAFALE SPEEDY « octet pour octet ».
// Contrat contracts/picto-pool.md §1, §2, §9 : le moteur de pool est généralisé (EntryPool, alias RafaleQuestion,
// deux instances) mais le pool RAFALE doit rester identique : mêmes fichiers disque (reservoir.json,
// rafale_used.json), mêmes ID « r-NNN », même pioche, mêmes remises à zéro. Ces tests passent AVANT la
// refactorisation et DOIVENT passer APRÈS, sans modification. Ils n'utilisent que l'API publique du moteur
// existante (SetRafalePath, Upsert/Append, Draw*, Reset*), jamais les champs internes.

const rafaleReservoirGolden = `{
  "QUESTIONS": [
    {
      "ID": "r-001",
      "QUESTION": "Capitale du Pérou ?",
      "ANSWER": "Lima",
      "CATEGORY": "GEOGRAPHY",
      "DIFFICULTY": 2
    },
    {
      "ID": "r-002",
      "QUESTION": "Éléments « chimiques » ?",
      "ANSWER": "Fe",
      "CATEGORY": "SCIENCE",
      "DIFFICULTY": 1
    }
  ]
}`

func nrEngine(t *testing.T) (*Engine, string, string) {
	t.Helper()
	dir := t.TempDir()
	res := filepath.Join(dir, "files", "rafale", "reservoir.json")
	used := filepath.Join(dir, "config", "rafale_used.json")
	e := NewEngine()
	e.SetRafalePath(res)
	e.SetRafaleUsedPath(used)
	return e, res, used
}

func TestPoolRafaleNR_ReservoirFile_GoldenBytes(t *testing.T) {
	e, res, _ := nrEngine(t)
	// IDs alloués par le serveur, dans l'ordre : r-001, r-002 (préfixe « r- », 3 chiffres)
	stored, err := e.AppendRafaleQuestions([]RafaleQuestion{
		{Question: "Capitale du Pérou ?", Answer: "Lima", Category: CategoryGeography, Difficulty: 2},
		{Question: "Éléments « chimiques » ?", Answer: "Fe", Category: CategoryScience, Difficulty: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stored[0].ID != "r-001" || stored[1].ID != "r-002" {
		t.Fatalf("IDs = %q, %q ; attendu r-001, r-002", stored[0].ID, stored[1].ID)
	}
	got, err := os.ReadFile(res)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != rafaleReservoirGolden {
		t.Errorf("reservoir.json ≠ golden (octet pour octet).\n--- obtenu ---\n%s\n--- attendu ---\n%s", got, rafaleReservoirGolden)
	}
}

func TestPoolRafaleNR_ReservoirFile_LoadThenSave_IsByteIdentical(t *testing.T) {
	e, res, _ := nrEngine(t)
	if err := os.MkdirAll(filepath.Dir(res), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(res, []byte(rafaleReservoirGolden), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.LoadRafale(); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveRafale(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(res)
	if string(got) != rafaleReservoirGolden {
		t.Errorf("relecture puis réécriture : fichier modifié.\n%s", got)
	}
}

func TestPoolRafaleNR_ReservoirFile_WrittenSortedByID(t *testing.T) {
	e, res, _ := nrEngine(t)
	for _, id := range []string{"r-010", "r-002", "r-001"} {
		if _, err := e.UpsertRafaleQuestion(RafaleQuestion{ID: id, Question: "Q" + id, Answer: "A", Category: CategoryHistory, Difficulty: 1}); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := os.ReadFile(res)
	s := string(data)
	i1, i2, i10 := indexOf(s, `"r-001"`), indexOf(s, `"r-002"`), indexOf(s, `"r-010"`)
	if !(i1 >= 0 && i1 < i2 && i2 < i10) {
		t.Errorf("entrées non triées par ID : %s", s)
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestPoolRafaleNR_NextID_SkipsGapsAndIgnoresForeignShapes(t *testing.T) {
	e, _, _ := nrEngine(t)
	for _, id := range []string{"r-001", "r-003", "custom-x"} {
		if _, err := e.UpsertRafaleQuestion(RafaleQuestion{ID: id, Question: "Q", Answer: "A", Category: CategoryHistory, Difficulty: 1}); err != nil {
			t.Fatal(err)
		}
	}
	q, err := e.UpsertRafaleQuestion(RafaleQuestion{Question: "Nouvelle", Answer: "A", Category: CategoryHistory, Difficulty: 1})
	if err != nil {
		t.Fatal(err)
	}
	if q.ID != "r-004" {
		t.Errorf("ID alloué = %q, attendu r-004 (max numérique + 1, ID « custom-x » ignoré)", q.ID)
	}
}

func TestPoolRafaleNR_UsedFile_GoldenBytes_AndMarkingAtDraw(t *testing.T) {
	e, _, used := nrEngine(t)
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-001", Question: "Q", Answer: "A", Category: CategoryHistory, Difficulty: 1},
	})
	drawn := drawRafaleQuestionNoAutoSave(t, e, string(CategoryHistory), 1)
	if drawn.ID != "r-001" {
		t.Fatalf("tiré = %q", drawn.ID)
	}
	if err := e.SaveRafaleUsed(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(used)
	want := "{\n  \"USED\": {\n    \"r-001\": true\n  }\n}"
	if string(got) != want {
		t.Errorf("rafale_used.json ≠ golden.\n%s", got)
	}
}

func TestPoolRafaleNR_UsedFile_LoadThenSave_IsByteIdentical(t *testing.T) {
	e, _, used := nrEngine(t)
	if err := os.MkdirAll(filepath.Dir(used), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "{\n  \"USED\": {\n    \"r-001\": true,\n    \"r-007\": true\n  }\n}"
	if err := os.WriteFile(used, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.LoadRafaleUsed(); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveRafaleUsed(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(used)
	if string(got) != src {
		t.Errorf("relecture puis réécriture : fichier modifié.\n%s", got)
	}
}

func TestPoolRafaleNR_Draw_MarksUsed_ExhaustsThenPoolEmpty_ResetRestores(t *testing.T) {
	e, _, _ := nrEngine(t)
	seedRafaleReservoir(t, e, []RafaleQuestion{
		{ID: "r-001", Question: "Q1", Answer: "A", Category: CategoryHistory, Difficulty: 1},
		{ID: "r-002", Question: "Q2", Answer: "A", Category: CategoryHistory, Difficulty: 1},
		{ID: "r-003", Question: "Q3", Answer: "A", Category: CategoryScience, Difficulty: 1}, // hors filtre
	})
	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		d := drawRafaleQuestionNoAutoSave(t, e, string(CategoryHistory), 1)
		if d.Category != CategoryHistory {
			t.Fatalf("tirage hors filtre : %+v", d)
		}
		if seen[d.ID] {
			t.Fatalf("%s tirée deux fois : une question utilisée n'est jamais retirée", d.ID)
		}
		seen[d.ID] = true
	}
	if _, err := e.DrawRafaleQuestion([]string{string(CategoryHistory)}, []int{1}); err != ErrRafalePoolEmpty {
		t.Errorf("pool épuisé : err = %v, attendu ErrRafalePoolEmpty", err)
	}
	n, err := e.ResetAllRafaleUsed()
	if err != nil || n != 2 {
		t.Fatalf("ResetAllRafaleUsed = %d, %v ; attendu 2, nil", n, err)
	}
	if _, err := e.DrawRafaleQuestion([]string{string(CategoryHistory)}, []int{1}); err != nil {
		t.Errorf("après remise à zéro, la pioche doit refonctionner : %v", err)
	}
}

func TestPoolRafaleNR_MarkAvailable_UnknownID_NotFound_KnownUnusedIsNoop(t *testing.T) {
	e, _, _ := nrEngine(t)
	seedRafaleReservoir(t, e, []RafaleQuestion{{ID: "r-001", Question: "Q", Answer: "A", Category: CategoryHistory, Difficulty: 1}})
	if err := e.MarkRafaleQuestionAvailable("r-999"); err != ErrRafaleQuestionNotFound {
		t.Errorf("ID inconnu : err = %v", err)
	}
	if err := e.MarkRafaleQuestionAvailable("r-001"); err != nil {
		t.Errorf("question connue non utilisée : no-op attendu, err = %v", err)
	}
	if err := e.DeleteRafaleQuestion("r-999"); err != ErrRafaleQuestionNotFound {
		t.Errorf("suppression d'un ID inconnu : err = %v", err)
	}
}

func TestPoolRafaleNR_ClearRafaleUsed_IsNewGameReset_ReservoirUntouched(t *testing.T) {
	e, _, _ := nrEngine(t)
	seedRafaleReservoir(t, e, []RafaleQuestion{{ID: "r-001", Question: "Q", Answer: "A", Category: CategoryHistory, Difficulty: 1}})
	drawRafaleQuestionNoAutoSave(t, e, string(CategoryHistory), 1)
	e.ClearRafaleUsed()
	qs, usedMap := e.SnapshotRafaleReservoir()
	if len(qs) != 1 || len(usedMap) != 0 {
		t.Errorf("après ClearRafaleUsed : %d question(s), %d utilisée(s) ; attendu 1 et 0", len(qs), len(usedMap))
	}
}
