package game

// #255 (v11.2) — Bumper.ANSWERED_COLOR : couleur RÉPONDUE à la question QCM courante ("" sinon), distincte de ANSWER_COLOR
// (couleur assignée / dernière pressée, inchangée). Écrite par ProcessButtonPress sur un buzz ENREGISTRÉ d'une question QCM,
// remise à "" partout où TIME est remis à 0. Invariant : ANSWERED_COLOR ≠ "" ⇒ TIME > 0.
// Contrats : models.md §Bumper, ws-payload-serialization.md. Scénarios serveur 1 à 6 du plan
// (_work/reports/planner-20261012-171500.md). `critical` : 1, 6.
//
// Les assertions lisent le bumper par son JSON (clé "ANSWERED_COLOR") : le test ne dépend d'aucun nom de champ Go.

import (
	"encoding/json"
	"testing"
	"time"
)

type bumperView255 struct {
	Time         int64
	AnswerColor  string
	Answered     string
	AnsweredSeen bool
}

func view255(t *testing.T, e *Engine, id string) bumperView255 {
	t.Helper()
	b := e.GetBumper(id)
	if b == nil {
		t.Fatalf("bumper %s introuvable", id)
	}
	raw, _ := json.Marshal(b)
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	v := bumperView255{Time: b.Time, AnswerColor: string(b.AnswerColor)}
	if r, ok := m["ANSWERED_COLOR"]; ok {
		v.AnsweredSeen = true
		_ = json.Unmarshal(r, &v.Answered)
	}
	return v
}

func qcm255(id string) *Question {
	return &Question{
		ID: id, Question: "QCM ?", Type: QuestionTypeQCM,
		TypedContent: TypedContent{QCMAnswers: &QCMAnswers{Red: "a", Green: "b", Yellow: "c", Blue: "d"}, QCMCorrect: "GREEN"},
	}
}

// engine255 : teamA (physique b1 assigné RED + physique b2 assigné GREEN), teamB (VJoueur vp, assigné YELLOW + physique pb).
func engine255(t *testing.T) *Engine {
	t.Helper()
	e := NewEngine()
	e.SetTeams(map[string]*Team{"teamA": {Name: "A"}, "teamB": {Name: "B"}})
	e.SetBumpers(map[string]*Bumper{
		"b1": {Name: "P1", Team: "teamA", AnswerColor: AnswerColorRed},
		"b2": {Name: "P2", Team: "teamA", AnswerColor: AnswerColorGreen},
		"vp": {Name: "V", Team: "teamB", IsVPlayer: true, AnswerColor: AnswerColorYellow},
		"pb": {Name: "PB", Team: "teamB", AnswerColor: AnswerColorBlue},
	})
	return e
}

func press255(e *Engine, id, button string) { e.ProcessButtonPress(id, time.Now().UnixMicro(), button) }

// --- 1. Réponse enregistrée puis question suivante (critical) ---------------------------------------------

func TestAnsweredColor255_Critical_BuzzSetsItAndTheNextReadyClearsItKeepingTheAssignedColor(t *testing.T) {
	e := engine255(t)
	e.Ready("q1", qcm255("q1"))
	e.StartImmediate(30)

	press255(e, "b1", "B") // assigné RED, répond GREEN

	v := view255(t, e, "b1")
	if !v.AnsweredSeen || v.Answered != "GREEN" || v.AnswerColor != "GREEN" || v.Time <= 0 {
		t.Fatalf("buzz B : ANSWERED_COLOR et ANSWER_COLOR = GREEN, TIME > 0 : %+v", v)
	}
	e.Stop()
	e.Ready("q2", qcm255("q2"))
	v = view255(t, e, "b1")
	if v.Answered != "" || !v.AnsweredSeen || v.Time != 0 {
		t.Errorf("Ready de la question 2 : ANSWERED_COLOR vide (clé présente), TIME 0 : %+v", v)
	}
	if v.AnswerColor != "GREEN" {
		t.Errorf("ANSWER_COLOR (couleur assignée / dernière pressée) est INCHANGÉE : %q", v.AnswerColor)
	}
}

// --- 2. Joueur qui ne buzze pas --------------------------------------------------------------------------

func TestAnsweredColor255_APlayerWhoNeverBuzzedKeepsItEmptyWhileItsAssignedColorIsSet(t *testing.T) {
	e := engine255(t)
	e.Ready("q1", qcm255("q1"))
	e.StartImmediate(30)
	press255(e, "b1", "A")
	e.Stop()
	e.Reveal()

	for _, id := range []string{"b2", "vp", "pb"} {
		v := view255(t, e, id)
		if !v.AnsweredSeen || v.Answered != "" || v.AnswerColor == "" {
			t.Errorf("%s n'a pas répondu : ANSWERED_COLOR \"\" (clé présente) alors que ANSWER_COLOR assignée reste %q : %+v", id, v.AnswerColor, v)
		}
	}
}

// --- 3. VJoueur ---------------------------------------------------------------------------------------------

func TestAnsweredColor255_AVPlayerAnswerSetsIt(t *testing.T) {
	e := engine255(t)
	e.Ready("q1", qcm255("q1"))
	e.StartImmediate(30)

	press255(e, "vp", "D") // VPLAYER_QCM_ANSWER BLUE

	if v := view255(t, e, "vp"); v.Answered != "BLUE" || v.AnswerColor != "BLUE" || v.Time <= 0 {
		t.Errorf("réponse BLUE du VJoueur : %+v", v)
	}
}

// --- 4. Buzz ignoré ------------------------------------------------------------------------------------------

func TestAnsweredColor255_IgnoredBuzzesNeverSetIt(t *testing.T) {
	e := engine255(t)
	e.Ready("q1", qcm255("q1"))
	e.StartImmediate(30)

	press255(e, "b1", "A") // premier de l'équipe A : enregistré
	press255(e, "b2", "C") // 2ᵉ joueur de l'équipe : ignoré
	press255(e, "pb", "D") // buzzer physique d'une équipe qui a un VJoueur : ignoré en QCM

	for _, id := range []string{"b2", "pb"} {
		if v := view255(t, e, id); v.Answered != "" || v.Time != 0 {
			t.Errorf("%s : buzz ignoré, ANSWERED_COLOR reste \"\" : %+v", id, v)
		}
	}
	if v := view255(t, e, "b1"); v.Answered != "RED" {
		t.Errorf("le premier buzz est enregistré : %+v", v)
	}
}

// --- 5. SPEEDY -------------------------------------------------------------------------------------------------

func TestAnsweredColor255_SpeedyBuzzNeverSetsIt(t *testing.T) {
	e := engine255(t)
	e.Ready("s1", &Question{ID: "s1", Question: "Q ?", Type: QuestionTypeSpeedy})
	e.StartImmediate(30)

	press255(e, "b1", "A")

	if v := view255(t, e, "b1"); v.Time <= 0 || v.Answered != "" {
		t.Errorf("SPEEDY : TIME > 0 mais ANSWERED_COLOR \"\" : %+v", v)
	}
}

// --- 6. Invariant : tous les chemins qui remettent TIME à 0 (critical) -------------------------------------------

func TestAnsweredColor255_Critical_EveryPathThatResetsTimeAlsoClearsIt(t *testing.T) {
	answered := func(t *testing.T) *Engine {
		e := engine255(t)
		e.Ready("q1", qcm255("q1"))
		e.StartImmediate(30)
		press255(e, "b1", "B")
		press255(e, "vp", "D")
		for _, id := range []string{"b1", "vp"} {
			if v := view255(t, e, id); v.Answered == "" || v.Time <= 0 {
				t.Fatalf("précondition (%s) : %+v", id, v)
			}
		}
		return e
	}
	paths := map[string]func(e *Engine){
		"Ready (nouvelle question)":     func(e *Engine) { e.Stop(); e.Ready("q2", qcm255("q2")) },
		"Ready (même question rejouée)": func(e *Engine) { e.Stop(); e.Ready("q1", qcm255("q1")) },
		"RAZ des scores":                func(e *Engine) { e.RAZScores() },
		"NEW_GAME purge des scores":     func(e *Engine) { e.InitGameWith(NewGameOptions{PurgeScores: true, ResetQuestions: true}) },
		"NEW_GAME sans option":          func(e *Engine) { e.InitGame() },
		"redémarrage StartImmediate":    func(e *Engine) { e.Stop(); e.StartImmediate(30) },
	}
	for name, do := range paths {
		t.Run(name, func(t *testing.T) {
			e := answered(t)
			do(e)
			for _, id := range []string{"b1", "vp", "b2", "pb"} {
				v := view255(t, e, id)
				if v.Time == 0 && v.Answered != "" {
					t.Errorf("%s : TIME remis à 0 ⇒ ANSWERED_COLOR \"\" (invariant ≠\"\" ⇒ TIME>0) : %+v", id, v)
				}
				if v.Answered != "" && v.Time <= 0 {
					t.Errorf("%s : invariant violé : %+v", id, v)
				}
			}
			// au moins les deux qui avaient répondu sont effectivement remis à zéro quand le chemin touche TIME
			if name != "NEW_GAME sans option" {
				for _, id := range []string{"b1", "vp"} {
					if v := view255(t, e, id); v.Answered != "" {
						t.Errorf("%s : ANSWERED_COLOR doit être vidée par « %s » : %+v", id, name, v)
					}
				}
			}
		})
	}
}

func TestAnsweredColor255_AnOldBumperJSONWithoutTheKeyLoadsEmpty(t *testing.T) {
	var b Bumper
	if err := json.Unmarshal([]byte(`{"NAME":"Vieux","TEAM":"A","ANSWER_COLOR":"RED","TIME":0}`), &b); err != nil {
		t.Fatalf("un fichier de bumpers sans la clé se charge : %v", err)
	}
	raw, _ := json.Marshal(&b)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	if v, ok := m["ANSWERED_COLOR"]; !ok || v != "" {
		t.Errorf("clé absente = \"\" et toujours sérialisée (sans omitempty) : %v présent=%v", v, ok)
	}
}
