package game

// #249 (v11.2) — identité d'équipe = clé ; drapeau TEAMS_LOCKED.
// Contrats : contracts/models.md §Identité d'une équipe, contracts/game-state.md §TEAMS_LOCKED,
// contracts/logging.md §3.2 bis. Scénarios 2, 3 et 6 du plan (_work/reports/planner-20261011-001500.md §A.5).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func teamNames249(e *Engine) map[string]string {
	var raw struct {
		Teams map[string]struct {
			Name string `json:"NAME"`
		} `json:"teams"`
	}
	_ = json.Unmarshal(e.GetGameJSON(), &raw)
	out := map[string]string{}
	for k, v := range raw.Teams {
		out[k] = v.Name
	}
	return out
}

func lockedRaw249(e *Engine) (value bool, present bool) {
	var raw struct {
		Game map[string]json.RawMessage `json:"GAME"`
	}
	_ = json.Unmarshal(e.GetGameJSON(), &raw)
	v, ok := raw.Game["TEAMS_LOCKED"]
	if !ok {
		return false, false
	}
	_ = json.Unmarshal(v, &value)
	return value, true
}

// --- Scénario 3 (moteur) : SetTeams réécrit NAME = clé -----------------------------------

func TestTeams249_SetTeamsAlwaysWritesNameEqualToKey(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"Les Bleus":  {Name: "Equipe 1"}, // NAME différent de la clé (ancien renommage)
		"Les Rouges": {Name: ""},         // NAME absent
		"Les Verts":  {Name: "Les Verts"},
	})

	for key, name := range teamNames249(e) {
		if name != key {
			t.Errorf("équipe %q : NAME = %q, attendu la clé", key, name)
		}
	}
}

// --- Scénario 2 : normalisation au chargement + réécriture du fichier -------------------

func TestTeams249_LoadTeamsNormalizesNameAndRewritesTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "teams.json")
	if err := os.WriteFile(path, []byte(`{"Les Bleus":{"NAME":"Equipe 1","SCORE":3,"COLOR":[0,0,255]},"Les Rouges":{"NAME":"Les Rouges","SCORE":0}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	e := NewEngine()
	e.SetTeamsPath(path)

	if err := e.LoadTeams(); err != nil {
		t.Fatalf("LoadTeams: %v", err)
	}

	names := teamNames249(e)
	if names["Les Bleus"] != "Les Bleus" || names["Les Rouges"] != "Les Rouges" {
		t.Fatalf("après chargement NAME = clé : %v", names)
	}
	var onDisk map[string]struct {
		Name  string `json:"NAME"`
		Score int    `json:"SCORE"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatalf("teams.json illisible après réécriture : %v", err)
	}
	if onDisk["Les Bleus"].Name != "Les Bleus" {
		t.Errorf("le fichier doit être réécrit après une normalisation (NAME = %q)", onDisk["Les Bleus"].Name)
	}
	if onDisk["Les Bleus"].Score != 3 {
		t.Errorf("la normalisation ne perd pas le score : %d", onDisk["Les Bleus"].Score)
	}
}

func TestTeams249_LoadTeamsWithConsistentFileLeavesItUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "teams.json")
	content := `{"Les Bleus":{"NAME":"Les Bleus","SCORE":0}}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(path)
	e := NewEngine()
	e.SetTeamsPath(path)

	if err := e.LoadTeams(); err != nil {
		t.Fatal(err)
	}

	after, _ := os.Stat(path)
	got, _ := os.ReadFile(path)
	if string(got) != content || !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("fichier cohérent : aucune réécriture inutile")
	}
}

// --- Scénario 6 (critical) : TEAMS_LOCKED --------------------------------------------------

func TestTeamsLocked249_Critical_FlagFollowsTheQuestionLifecycle(t *testing.T) {
	e := NewEngine()
	if v, present := lockedRaw249(e); !present || v {
		t.Fatalf("au démarrage (STOPPED sans question) : TEAMS_LOCKED présent et false (obtenu %v, présent=%v)", v, present)
	}

	e.Ready("", &Question{ID: "q1", Type: QuestionTypeArdoise})
	if v, _ := lockedRaw249(e); !v {
		t.Fatal("Ready() (entrée en PREPARE) : TEAMS_LOCKED = true")
	}
	e.SetPhase(PhaseStarted)
	if v, _ := lockedRaw249(e); !v {
		t.Error("STARTED : verrou maintenu")
	}
	e.Stop()
	if v, _ := lockedRaw249(e); !v {
		t.Error("STOPPED après une question : verrou maintenu (le drapeau n'est pas déduit de la phase)")
	}
	e.Reveal()
	if e.GetPhase() != PhaseRevealed {
		t.Fatalf("précondition : REVEALED, obtenu %s", e.GetPhase())
	}
	if v, present := lockedRaw249(e); !present || v {
		t.Errorf("REVEALED : TEAMS_LOCKED = false, présent (obtenu %v, présent=%v)", v, present)
	}
}

func TestTeamsLocked249_AbandonedQuestionKeepsTheLockUntilNextSelectionOrNewGame(t *testing.T) {
	e := NewEngine()
	e.Ready("", &Question{ID: "q1", Type: QuestionTypeArdoise})
	e.SetPhase(PhaseStarted)
	e.Stop() // jamais révélée
	if v, _ := lockedRaw249(e); !v {
		t.Fatal("question abandonnée : verrou maintenu")
	}

	e.Ready("", &Question{ID: "q2", Type: QuestionTypeArdoise}) // la sélection suivante repose le verrou
	if v, _ := lockedRaw249(e); !v {
		t.Fatal("sélection suivante : verrou posé")
	}
	e.InitGame() // NEW_GAME
	if v, present := lockedRaw249(e); !present || v {
		t.Errorf("NEW_GAME/InitGame : TEAMS_LOCKED = false (obtenu %v)", v)
	}
}

func TestTeamsLocked249_OtherQuestionTypesLockToo(t *testing.T) {
	for _, qt := range []QuestionType{QuestionTypeSpeedy, QuestionTypeQCM, QuestionTypeMemory} {
		e := NewEngine()
		e.Ready("", &Question{ID: "q", Type: qt})
		if v, _ := lockedRaw249(e); !v {
			t.Errorf("type %s : TEAMS_LOCKED = true après Ready()", qt)
		}
	}
}
