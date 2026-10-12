package game

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// #265 — SCORE_CHANGE.GAINS (contracts/game-state.md §SCORE_CHANGE.GAINS, v11.2.1) :
// variation NETTE du score de chaque équipe pour la dernière modification de score MANUELLE
// (même SEQ), écrite dans la même section critique que les scores, toujours sérialisée.
// Les 4 chemins qui incrémentent SEQ : UpdateTeamScoreDisplay (TEAM_POINTS),
// UpdateBumperScoreDisplay (BUMPER_POINTS), ApplyTeamPointsBatch, ApplyBumperPointsBatch.
// On lit le résultat par le JSON de GetGameJSON() (c'est ce qui part vers les clients) : les tests
// compilent avant le correctif et ÉCHOUENT parce que GAINS est absent.

type gain265 struct {
	Team   string `json:"TEAM"`
	Points int    `json:"POINTS"`
}

type scoreChange265 struct {
	Seq      int
	Display  bool
	Teams    []string
	Gains    []gain265
	HasGains bool
	RawGains string
}

func readScoreChange265(t *testing.T, e *Engine) scoreChange265 {
	t.Helper()
	var env struct {
		Game struct {
			ScoreChange map[string]json.RawMessage `json:"SCORE_CHANGE"`
		} `json:"GAME"`
	}
	if err := json.Unmarshal(e.GetGameJSON(), &env); err != nil {
		t.Fatalf("GetGameJSON illisible : %v", err)
	}
	sc := env.Game.ScoreChange
	out := scoreChange265{}
	_ = json.Unmarshal(sc["SEQ"], &out.Seq)
	_ = json.Unmarshal(sc["DISPLAY"], &out.Display)
	_ = json.Unmarshal(sc["TEAMS"], &out.Teams)
	if raw, ok := sc["GAINS"]; ok {
		out.HasGains = true
		out.RawGains = string(raw)
		_ = json.Unmarshal(raw, &out.Gains)
	}
	return out
}

// TeamA : TeamPoints 10 + a1 (5) + a2 (3) = 18 ; TeamB : b1 (2) = 2 ; TeamC : c1 (0) = 0 ; n1 sans équipe.
func gainsEngine265(t *testing.T) *Engine {
	t.Helper()
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"TeamA": {Name: "TeamA", TeamPoints: 10, Score: 18},
		"TeamB": {Name: "TeamB", Score: 2},
		"TeamC": {Name: "TeamC"},
	})
	e.SetBumpers(map[string]*Bumper{
		"a1": {Name: "A1", Team: "TeamA", Score: 5},
		"a2": {Name: "A2", Team: "TeamA", Score: 3},
		"b1": {Name: "B1", Team: "TeamB", Score: 2},
		"c1": {Name: "C1", Team: "TeamC"},
		"n1": {Name: "N1"},
	})
	return e
}

// phase REVEALED sur une question, comme avant une attribution groupée.
func revealed265(e *Engine) {
	e.mu.Lock()
	e.state.Question = &Question{ID: "q265", Type: QuestionTypeSpeedy}
	e.state.Phase = PhaseRevealed
	e.mu.Unlock()
}

func wantGains(t *testing.T, got scoreChange265, want []gain265) {
	t.Helper()
	if !got.HasGains {
		t.Fatalf("SCORE_CHANGE.GAINS absent du JSON (toujours sérialisé, contrat v11.2.1)")
	}
	if len(want) == 0 && len(got.Gains) == 0 {
		return
	}
	if !reflect.DeepEqual(got.Gains, want) {
		t.Errorf("GAINS = %+v, attendu %+v (variation nette par équipe, ordre de première apparition)", got.Gains, want)
	}
}

// --- Défaut / sérialisation -------------------------------------------------------------------

func TestGains265_DefaultIsEmptyArrayNeverNull(t *testing.T) {
	e := NewEngine()
	sc := readScoreChange265(t, e)
	if !sc.HasGains {
		t.Fatal("GAINS absent par défaut : doit valoir [] (jamais omis, jamais null)")
	}
	if sc.RawGains != "[]" {
		t.Errorf("GAINS par défaut = %s, attendu []", sc.RawGains)
	}
	if strings.Contains(string(e.GetGameJSON()), `"GAINS":null`) {
		t.Error("GAINS ne doit jamais être null")
	}
}

// --- TEAM_POINTS ------------------------------------------------------------------------------

func TestGains265_TeamPoints_PositiveNegativeAndZero(t *testing.T) {
	e := gainsEngine265(t)
	e.UpdateTeamScoreDisplay("TeamA", 4, true)
	sc := readScoreChange265(t, e)
	if sc.Seq != 1 {
		t.Fatalf("SEQ = %d, attendu 1", sc.Seq)
	}
	wantGains(t, sc, []gain265{{"TeamA", 4}})

	e.UpdateTeamScoreDisplay("TeamB", -3, true)
	wantGains(t, readScoreChange265(t, e), []gain265{{"TeamB", -3}}) // pénalité : entrée présente, POINTS < 0

	e.UpdateTeamScoreDisplay("TeamC", 0, true)
	wantGains(t, readScoreChange265(t, e), []gain265{{"TeamC", 0}}) // gain nul : entrée présente
}

func TestGains265_TeamPoints_HiddenCorrectionStillFillsGains(t *testing.T) {
	e := gainsEngine265(t)
	e.UpdateTeamScoreDisplay("TeamA", 7, false)
	sc := readScoreChange265(t, e)
	if sc.Display {
		t.Error("DISPLAY doit valoir false pour une correction cachée")
	}
	wantGains(t, sc, []gain265{{"TeamA", 7}}) // GAINS décrit le fait ; DISPLAY seul interdit l'animation
}

// --- BUMPER_POINTS ----------------------------------------------------------------------------

func TestGains265_BumperPoints_GainGoesToTheBumpersTeam(t *testing.T) {
	e := gainsEngine265(t)
	e.UpdateBumperScoreDisplay("a1", 6, true)
	wantGains(t, readScoreChange265(t, e), []gain265{{"TeamA", 6}})
}

func TestGains265_BumperPoints_NegativeAndBumperWithoutTeam(t *testing.T) {
	e := gainsEngine265(t)
	e.UpdateBumperScoreDisplay("b1", -1, true)
	wantGains(t, readScoreChange265(t, e), []gain265{{"TeamB", -1}})

	e.UpdateBumperScoreDisplay("n1", 5, true) // joueur sans équipe : SEQ avance, aucune entrée
	sc := readScoreChange265(t, e)
	if sc.Seq != 2 {
		t.Errorf("SEQ = %d, attendu 2", sc.Seq)
	}
	wantGains(t, sc, nil)
	if sc.RawGains != "[]" {
		t.Errorf("sans équipe concernée GAINS doit valoir [] (pas null), a %s", sc.RawGains)
	}
}

// --- TEAM_POINTS_BATCH ------------------------------------------------------------------------

func TestGains265_TeamPointsBatch_OneEntryPerTeamInOrderOfAppearance(t *testing.T) {
	e := gainsEngine265(t)
	revealed265(e)
	res := e.ApplyTeamPointsBatch("q265", []TeamPointsEntry{
		{Team: "TeamB", Points: 4}, {Team: "TeamA", Points: -2}, {Team: "TeamC", Points: 1},
	}, CategoryMeta{}, true)
	if res.Status != "APPLIED" {
		t.Fatalf("lot refusé : %+v", res)
	}
	sc := readScoreChange265(t, e)
	if sc.Seq != 1 {
		t.Errorf("SEQ = %d, attendu 1 (un seul incrément pour tout le lot)", sc.Seq)
	}
	wantGains(t, sc, []gain265{{"TeamB", 4}, {"TeamA", -2}, {"TeamC", 1}})
}

func TestGains265_TeamPointsBatch_SkippedTeamHasNoEntry(t *testing.T) {
	e := gainsEngine265(t)
	revealed265(e)
	e.ApplyTeamPointsBatch("q265", []TeamPointsEntry{{Team: "TeamA", Points: 1}}, CategoryMeta{}, true)
	res := e.ApplyTeamPointsBatch("q265", []TeamPointsEntry{{Team: "TeamA", Points: 5}, {Team: "TeamB", Points: 2}}, CategoryMeta{}, true)
	if len(res.Skipped) != 1 {
		t.Fatalf("TeamA devait être ignorée (déjà créditée) : %+v", res)
	}
	wantGains(t, readScoreChange265(t, e), []gain265{{"TeamB", 2}}) // seule la variation réellement appliquée
}

func TestGains265_TeamPointsBatch_HiddenKeepsGains(t *testing.T) {
	e := gainsEngine265(t)
	revealed265(e)
	e.ApplyTeamPointsBatch("q265", []TeamPointsEntry{{Team: "TeamA", Points: 3}}, CategoryMeta{}, false)
	sc := readScoreChange265(t, e)
	if sc.Display {
		t.Error("DISPLAY doit valoir false")
	}
	wantGains(t, sc, []gain265{{"TeamA", 3}})
}

// --- BUMPER_POINTS_BATCH (incrémente déjà SEQ, explicitement couvert par le contrat) ----------

func TestGains265_BumperPointsBatch_PlayersOfSameTeamAreSummed(t *testing.T) {
	e := gainsEngine265(t)
	revealed265(e)
	res := e.ApplyBumperPointsBatch("q265", []BumperPointsEntry{
		{ID: "a1", Points: 2}, {ID: "b1", Points: 1}, {ID: "a2", Points: 3},
	}, CategoryMeta{}, true)
	if res.Status != "APPLIED" {
		t.Fatalf("lot refusé : %+v", res)
	}
	sc := readScoreChange265(t, e)
	if sc.Seq != 1 {
		t.Errorf("SEQ = %d, attendu 1", sc.Seq)
	}
	wantGains(t, sc, []gain265{{"TeamA", 5}, {"TeamB", 1}}) // a1 + a2 additionnés, ordre de première apparition
}

func TestGains265_BumperPointsBatch_CompensationGivesZeroEntry(t *testing.T) {
	e := gainsEngine265(t)
	revealed265(e)
	e.ApplyBumperPointsBatch("q265", []BumperPointsEntry{{ID: "a1", Points: 2}, {ID: "a2", Points: -2}}, CategoryMeta{}, true)
	wantGains(t, readScoreChange265(t, e), []gain265{{"TeamA", 0}}) // nul mais présent
}

func TestGains265_BumperPointsBatch_PlayerWithoutTeamProducesNoEntry(t *testing.T) {
	e := gainsEngine265(t)
	revealed265(e)
	e.ApplyBumperPointsBatch("q265", []BumperPointsEntry{{ID: "n1", Points: 4}, {ID: "b1", Points: 2}}, CategoryMeta{}, true)
	wantGains(t, readScoreChange265(t, e), []gain265{{"TeamB", 2}})
}

func TestGains265_BumperPointsBatch_HiddenKeepsGains(t *testing.T) {
	e := gainsEngine265(t)
	revealed265(e)
	e.ApplyBumperPointsBatch("q265", []BumperPointsEntry{{ID: "a1", Points: 3}}, CategoryMeta{}, false)
	sc := readScoreChange265(t, e)
	if sc.Display {
		t.Error("DISPLAY doit valoir false")
	}
	wantGains(t, sc, []gain265{{"TeamA", 3}})
}

// --- Remise à zéro et crédits automatiques ----------------------------------------------------

func TestGains265_NewGameResetsGainsToEmptyArray(t *testing.T) {
	e := gainsEngine265(t)
	e.UpdateTeamScoreDisplay("TeamA", 4, true)
	e.InitGameWith(DefaultNewGameOptions())
	sc := readScoreChange265(t, e)
	if !sc.HasGains || sc.RawGains != "[]" || sc.Seq != 0 {
		t.Errorf("NEW_GAME : attendu SEQ 0 et GAINS [], a SEQ=%d GAINS=%s (présent=%v)", sc.Seq, sc.RawGains, sc.HasGains)
	}
}

func TestGains265_AutomaticCreditTouchesNeitherSeqNorGains(t *testing.T) {
	e := gainsEngine265(t)
	e.UpdateTeamScoreDisplay("TeamB", 2, true)
	before := readScoreChange265(t, e)
	e.UpdateScore("a1", 9) // crédit automatique du moteur
	after := readScoreChange265(t, e)
	if after.Seq != before.Seq {
		t.Errorf("SEQ ne doit pas bouger sur un crédit automatique : %d → %d", before.Seq, after.Seq)
	}
	if !reflect.DeepEqual(after.Gains, before.Gains) {
		t.Errorf("GAINS ne doit pas bouger sur un crédit automatique : %+v → %+v", before.Gains, after.Gains)
	}
	wantGains(t, after, []gain265{{"TeamB", 2}})
}

// --- Même section critique : un instantané ne montre jamais les scores sans leur GAINS --------

func TestGains265_SnapshotNeverShowsScoresWithoutMatchingGains(t *testing.T) {
	e := gainsEngine265(t)
	const n = 400
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < n; i++ {
			e.UpdateTeamScoreDisplay("TeamA", 1, true)
		}
	}()
	failed := false
	check := func() {
		if failed {
			return
		}
		var env struct {
			Game struct {
				ScoreChange map[string]json.RawMessage `json:"SCORE_CHANGE"`
			} `json:"GAME"`
			Teams map[string]struct {
				SCORE int `json:"SCORE"`
			} `json:"teams"`
		}
		if err := json.Unmarshal(e.GetGameJSON(), &env); err != nil {
			t.Errorf("JSON illisible : %v", err)
			return
		}
		var seq int
		_ = json.Unmarshal(env.Game.ScoreChange["SEQ"], &seq)
		var gains []gain265
		raw, has := env.Game.ScoreChange["GAINS"]
		if has {
			_ = json.Unmarshal(raw, &gains)
		}
		if seq > 0 && !(has && len(gains) == 1 && gains[0].Team == "TeamA" && gains[0].Points == 1) {
			failed = true
			t.Errorf("SEQ=%d mais GAINS=%s : scores et GAINS doivent partir ensemble", seq, string(raw))
		}
		if got := env.Teams["TeamA"].SCORE; got != 18+seq {
			failed = true
			t.Errorf("incohérence : SCORE TeamA=%d pour SEQ=%d (attendu %d)", got, seq, 18+seq)
		}
	}
	for {
		select {
		case <-done:
			check()
			return
		default:
			check()
		}
	}
}
