package protocol

// #258 (v11.2) — filtrage par phase et par destinataire des réponses vers les tablettes (contrat ab93c53d,
// vplayer-payload-filter.md §7). Réponses ouvertes (PREPARE, READY, COUNTDOWN, STARTED, PAUSED) : les champs de réponse des
// bumpers et des équipes d'une AUTRE équipe sont neutres (0 / ""), clés conservées ; QCM_CORRECT et ANSWER absents de
// GAME.QUESTION dans toute phase sauf REVEALED. Boîte noire par les sérialiseurs publics (aucune dépendance au nom de la fonction
// interne filterVPlayerAnswers). Scénarios 1-6 du plan (_work/reports/planner-20261012-200000.md §2). `critical` : 1, 4.

import (
	"encoding/json"
	"testing"
)

var answerFields258 = []string{"TIME", "BUTTON", "STATUS", "ANSWER_COLOR", "ANSWERED_COLOR", "HINTS_AT_BUZZ"}

// update258 : A (Paul, Jade — coéquipiers) et B (Mamie) ; B et Jade ont répondu.
func update258(phase string) *Message {
	player := func(team, name string, answered bool) map[string]interface{} {
		b := map[string]interface{}{"NAME": name, "TEAM": team, "SCORE": 7, "CONNECTED": true, "CONN_STATE": "green", "IS_VPLAYER": true,
			"TIME": 0, "BUTTON": "", "STATUS": "", "ANSWER_COLOR": "RED", "ANSWERED_COLOR": "", "HINTS_AT_BUZZ": 0}
		if answered {
			b["TIME"], b["BUTTON"], b["STATUS"], b["ANSWER_COLOR"], b["ANSWERED_COLOR"], b["HINTS_AT_BUZZ"] = 123456, "B", "PAUSE", "GREEN", "GREEN", 2
		}
		return b
	}
	team := func(answered bool) map[string]interface{} {
		t := map[string]interface{}{"NAME": "x", "SCORE": 7, "TIME": 0, "BUMPER": "", "STATUS": ""}
		if answered {
			t["TIME"], t["BUMPER"], t["STATUS"] = 123456, "mamie", "PAUSE"
		}
		return t
	}
	m, _ := NewMessage(ActionUpdate, map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE":    phase,
			"QUESTION": map[string]interface{}{"ID": "9", "TYPE": "QCM", "QUESTION": "Q ?", "QCM_CORRECT": "GREEN", "ANSWER": "La Seine", "POINTS": "3", "QCM_ANSWERS": map[string]string{"RED": "a", "GREEN": "b"}},
		},
		"bumpers": map[string]interface{}{"paul": player("Bleus", "Paul", false), "jade": player("Bleus", "Jade", true), "mamie": player("Rouges", "Mamie", true)},
		"teams":   map[string]interface{}{"Bleus": team(true), "Rouges": team(true)},
	})
	return m
}

type decoded258 struct {
	Game    map[string]interface{}
	Bumpers map[string]map[string]interface{}
	Teams   map[string]map[string]interface{}
}

func decode258(t *testing.T) func([]byte, error) decoded258 {
	return func(raw []byte, err error) decoded258 {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		var env struct {
			Msg struct {
				Game    map[string]interface{}            `json:"GAME"`
				Bumpers map[string]map[string]interface{} `json:"bumpers"`
				Teams   map[string]map[string]interface{} `json:"teams"`
			} `json:"MSG"`
		}
		if e := json.Unmarshal(raw, &env); e != nil {
			t.Fatalf("payload illisible : %v (%s)", e, raw)
		}
		return decoded258{env.Msg.Game, env.Msg.Bumpers, env.Msg.Teams}
	}
}

func isNeutral258(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case float64:
		return x == 0
	case string:
		return x == ""
	}
	return false
}

var openPhases258 = []string{"PREPARE", "READY", "COUNTDOWN", "STARTED", "PAUSED"}

// --- 1. Réponses ouvertes : l'autre équipe est neutralisée, la sienne et celle des coéquipiers non (critical) -------------------

func TestVPlayerAnswers258_Critical_OtherTeamsAnswerFieldsAreNeutralInEveryOpenPhase(t *testing.T) {
	for _, phase := range openPhases258 {
		t.Run(phase, func(t *testing.T) {
			p := decode258(t)(update258(phase).SerializeForVPlayer("paul"))
			if len(p.Bumpers) == 0 {
				t.Skip("payload réduit à son seul bumper dans cette phase : la règle de l'autre équipe est satisfaite par construction")
			}
			if mamie, ok := p.Bumpers["mamie"]; ok {
				for _, f := range answerFields258 {
					v, present := mamie[f]
					if !present {
						t.Errorf("%s : clé %s de Mamie conservée (neutre), pas retirée", phase, f)
					} else if !isNeutral258(v) {
						t.Errorf("%s : Mamie (autre équipe) %s = %v, attendu neutre", phase, f, v)
					}
				}
				if mamie["NAME"] != "Mamie" || mamie["SCORE"] != float64(7) || mamie["TEAM"] != "Rouges" {
					t.Errorf("%s : NAME, TEAM, SCORE de l'autre équipe intacts : %v", phase, mamie)
				}
			}
			if rouges, ok := p.Teams["Rouges"]; ok {
				for _, f := range []string{"TIME", "BUMPER", "STATUS"} {
					v, present := rouges[f]
					if !present || !isNeutral258(v) {
						t.Errorf("%s : teams[Rouges].%s neutre et présent : %v (présent=%v)", phase, f, v, present)
					}
				}
				if rouges["SCORE"] != float64(7) {
					t.Errorf("%s : le score de l'autre équipe reste visible", phase)
				}
			}
		})
	}
}

func TestVPlayerAnswers258_OwnAndTeammateFieldsStayVisible(t *testing.T) {
	for _, phase := range []string{"STARTED", "PAUSED"} {
		p := decode258(t)(update258(phase).SerializeForVPlayer("paul"))
		jade, ok := p.Bumpers["jade"]
		if !ok {
			t.Skipf("%s : bumper du coéquipier absent du payload", phase)
		}
		if jade["TIME"] != float64(123456) || jade["ANSWERED_COLOR"] != "GREEN" || jade["ANSWER_COLOR"] != "GREEN" || jade["HINTS_AT_BUZZ"] != float64(2) {
			t.Errorf("%s : un coéquipier voit les champs de réponse de l'autre (aucun intérêt à tricher) : %v", phase, jade)
		}
		if bleus := p.Teams["Bleus"]; bleus["TIME"] != float64(123456) || bleus["BUMPER"] != "mamie" {
			t.Errorf("%s : sa propre équipe garde TIME/BUMPER/STATUS : %v", phase, bleus)
		}
	}
	// et le destinataire lui-même
	p := decode258(t)(update258("STARTED").SerializeForVPlayer("jade"))
	if me, ok := p.Bumpers["jade"]; !ok || me["ANSWERED_COLOR"] != "GREEN" || me["TIME"] != float64(123456) {
		t.Errorf("le destinataire voit ses propres valeurs : %v", me)
	}
}

// --- 6. Connexion non liée : tout neutralisé pendant les réponses ouvertes ------------------------------------------------------------

func TestVPlayerAnswers258_UnboundConnectionGetsEverythingNeutralisedAndNoOnlyFields(t *testing.T) {
	for _, phase := range openPhases258 {
		for name, raw := range map[string]func() ([]byte, error){
			"playerID vide":  func() ([]byte, error) { return update258(phase).SerializeForVPlayer("") },
			"joueur inconnu": func() ([]byte, error) { return update258(phase).SerializeForVPlayer("fantome") },
			"chemin commun":  func() ([]byte, error) { return update258(phase).SerializeForVPlayerCommon() },
		} {
			p := decode258(t)(raw())
			for id, b := range p.Bumpers {
				for _, f := range answerFields258 {
					if v, present := b[f]; present && !isNeutral258(v) {
						t.Errorf("%s / %s : %s.%s = %v, tout neutralisé pour une connexion non liée", phase, name, id, f, v)
					}
				}
			}
			for id, tm := range p.Teams {
				for _, f := range []string{"TIME", "BUMPER", "STATUS"} {
					if v, present := tm[f]; present && !isNeutral258(v) {
						t.Errorf("%s / %s : teams[%s].%s = %v", phase, name, id, f, v)
					}
				}
			}
		}
	}
}

// --- 4. Bonne réponse : retirée avant REVEALED, transmise à REVEALED (critical) -------------------------------------------------------

func TestVPlayerAnswers258_Critical_RightAnswerIsRemovedBeforeRevealedAndSentAtRevealed(t *testing.T) {
	for _, phase := range []string{"PREPARE", "READY", "COUNTDOWN", "STARTED", "PAUSED", "STOPPED", "ENROLL", "NEW_GAME"} {
		for name, raw := range map[string]func() ([]byte, error){
			"identifié": func() ([]byte, error) { return update258(phase).SerializeForVPlayer("paul") },
			"commun":    func() ([]byte, error) { return update258(phase).SerializeForVPlayerCommon() },
		} {
			p := decode258(t)(raw())
			q, _ := p.Game["QUESTION"].(map[string]interface{})
			for _, k := range []string{"QCM_CORRECT", "ANSWER"} {
				if _, present := q[k]; present {
					t.Errorf("%s / %s : GAME.QUESTION.%s ne doit pas atteindre une tablette avant REVEALED", phase, name, k)
				}
			}
			if q["QUESTION"] != "Q ?" || q["POINTS"] != "3" || q["QCM_ANSWERS"] == nil {
				t.Errorf("%s / %s : le reste de la question (texte, points, réponses proposées) est intact : %v", phase, name, q)
			}
		}
	}
	for name, raw := range map[string]func() ([]byte, error){
		"identifié": func() ([]byte, error) { return update258("REVEALED").SerializeForVPlayer("paul") },
		"commun":    func() ([]byte, error) { return update258("REVEALED").SerializeForVPlayerCommon() },
	} {
		q, _ := decode258(t)(raw()).Game["QUESTION"].(map[string]interface{})
		if q["QCM_CORRECT"] != "GREEN" || q["ANSWER"] != "La Seine" {
			t.Errorf("REVEALED / %s : la trame de REVEALED porte la bonne réponse : %v", name, q)
		}
	}
}

// --- 5. STOPPED : champs des autres équipes transmis (badges des réponses par équipe) ---------------------------------------------------

func TestVPlayerAnswers258_StoppedAndRevealedKeepOtherTeamsAnswers(t *testing.T) {
	for _, phase := range []string{"STOPPED", "REVEALED"} {
		p := decode258(t)(update258(phase).SerializeForVPlayer("paul"))
		mamie, ok := p.Bumpers["mamie"]
		if !ok {
			t.Fatalf("%s : bumper de Mamie présent", phase)
		}
		if mamie["ANSWERED_COLOR"] != "GREEN" || mamie["TIME"] != float64(123456) || mamie["HINTS_AT_BUZZ"] != float64(2) {
			t.Errorf("%s : plus personne ne peut répondre, les champs de l'autre équipe sont transmis (badges) : %v", phase, mamie)
		}
		if p.Teams["Rouges"]["TIME"] != float64(123456) {
			t.Errorf("%s : teams[Rouges].TIME transmis", phase)
		}
	}
}

// --- Forme, pas action : le filtre s'applique à tout message GAME/bumpers/teams (START, PAUSE, …) -----------------------------------------

func TestVPlayerAnswers258_Critical_AnyActionCarryingBumpersIsFilteredByShape(t *testing.T) {
	for _, action := range []string{ActionStart, ActionStop, ActionPause, ActionContinue, "UPDATE_TIMER", "REMOTE", "BUMPER"} {
		src := update258("STARTED")
		m, _ := NewMessage(action, json.RawMessage(src.Msg))
		for name, raw := range map[string]func() ([]byte, error){
			"commun": func() ([]byte, error) { return m.SerializeForVPlayerCommon() },
		} {
			p := decode258(t)(raw())
			if mamie, ok := p.Bumpers["mamie"]; ok {
				for _, f := range answerFields258 {
					if v, present := mamie[f]; present && !isNeutral258(v) {
						t.Errorf("%s / %s : %s = %v fuit vers la tablette (filtrage par forme, jamais par nom d'action)", action, name, f, v)
					}
				}
			}
			if q, _ := p.Game["QUESTION"].(map[string]interface{}); q != nil {
				if _, present := q["QCM_CORRECT"]; present {
					t.Errorf("%s : QCM_CORRECT fuit", action)
				}
			}
		}
	}
}
