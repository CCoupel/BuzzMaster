package main

// #254 (v11.2) — durcissement de revue (contrat d14d9eaa) : BUMPER_POINTS_BATCH ignore un joueur dont l'ÉQUIPE a reçu un
// crédit d'équipe (WINNER_TYPE = TEAM) pour la question (motif TEAM_ALREADY_AWARDED) ; AWARDED_TEAMS.TEAMS[].TEAM_LEVEL ;
// décodage tolérant de QCM_WRONG_PENALTY. Plan : _work/reports/planner-20261012-143000.md (test-writer 1 à 6).
// Statut `regression` ; `critical` : 1.

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// teamLevels254 lit AWARDED_TEAMS.TEAMS[].TEAM_LEVEL : équipe → (valeur, présent).
func teamLevels254(t *testing.T, conn *websocket.Conn) map[string]*bool {
	t.Helper()
	raw := readActionMatchingWithin(t, conn, "AWARDED_TEAMS", 2*time.Second)
	var p struct {
		Teams []struct {
			Team      string `json:"TEAM"`
			TeamLevel *bool  `json:"TEAM_LEVEL"`
		} `json:"TEAMS"`
	}
	_ = json.Unmarshal(raw, &p)
	out := map[string]*bool{}
	for _, x := range p.Teams {
		out[x.Team] = x.TeamLevel
	}
	return out
}

func skippedReasons254(r bresult254) map[string]string {
	m := map[string]string{}
	for _, s := range r.Skipped {
		m[s.ID] = s.Reason
	}
	return m
}

// --- 1. Crédit d'équipe puis lot joueur (critical) ------------------------------------------------------

func TestBumperPointsBatch254_Critical_PlayersOfATeamAlreadyCreditedAsATeamAreSkipped(t *testing.T) {
	for name, creditTeam := range map[string]func(e *env244){
		"clic unitaire TEAM_POINTS": func(e *env244) {
			e.send(t, e.admin, "admin", protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Bleus", Points: 2})
		},
		"lot TEAM_POINTS_BATCH": func(e *env244) {
			e.batch(t, qid244, []entry244{{"Les Bleus", 2}}, nil)
			_ = e.result(t)
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv244(t)
			paul, buzz, mamie := e.threePlayers() // Paul et le buzzer : Bleus ; Mamie : Rouges
			creditTeam(e)
			before := e.playerScores()

			e.bbatch(t, qid244, []pentry254{{paul, 3}, {buzz, 3}, {mamie, 3}}, nil)

			r := e.bresult(t)
			if r.Status != "APPLIED" || len(r.Applied) != 1 || r.Applied[0].ID != mamie {
				t.Fatalf("seule Mamie (Rouges) est créditée : %+v", r)
			}
			reasons := skippedReasons254(r)
			if reasons[paul] != "TEAM_ALREADY_AWARDED" || reasons[buzz] != "TEAM_ALREADY_AWARDED" || len(r.Skipped) != 2 {
				t.Errorf("les joueurs des Bleus sont ignorés avec TEAM_ALREADY_AWARDED : %+v", r.Skipped)
			}
			after := e.playerScores()
			if after[paul] != before[paul] || after[buzz] != before[buzz] || after[mamie]-before[mamie] != 3 {
				t.Errorf("aucun double crédit : %v → %v", before, after)
			}
			if n, _ := e.playerEvents(paul); n != 0 {
				t.Error("aucune entrée POINTS_AWARDED joueur pour un joueur ignoré")
			}
		})
	}
}

// --- 2. Crédit d'équipe caché : ne bloque pas -------------------------------------------------------------

func TestBumperPointsBatch254_HiddenTeamCreditDoesNotBlockItsPlayers(t *testing.T) {
	e := newEnv244(t)
	paul, _, _ := e.threePlayers()
	e.send(t, e.admin, "admin", protocol.ActionTeamPoints, map[string]interface{}{"TEAM": "Les Bleus", "POINTS": 2, "DISPLAY": false})

	e.bbatch(t, qid244, []pentry254{{paul, 3}}, nil)

	r := e.bresult(t)
	if len(r.Applied) != 1 || len(r.Skipped) != 0 {
		t.Errorf("un crédit d'équipe caché (DISPLAY:false) ne verrouille rien : %+v", r)
	}
}

// --- 3. Crédit d'équipe à 0 de l'anim : bloque --------------------------------------------------------------

func TestBumperPointsBatch254_ZeroPointTeamCreditFromTheAnimBlocksItsPlayers(t *testing.T) {
	e := newEnv244(t)
	paul, _, mamie := e.threePlayers()
	_, anim := e.dial(t, "/ws/anim")
	e.send(t, anim, "anim", protocol.ActionTeamPoints, map[string]interface{}{"TEAM": "Les Bleus", "POINTS": 0})
	time.Sleep(100 * time.Millisecond)

	e.bbatch(t, qid244, []pentry254{{paul, 3}, {mamie, 3}}, nil)

	r := e.bresult(t)
	if skippedReasons254(r)[paul] != "TEAM_ALREADY_AWARDED" || len(r.Applied) != 1 {
		t.Errorf("le geste « 0 pt » de l'anim sur l'équipe est un verrou valide : %+v", r)
	}
}

// --- 4. Équipe supprimée puis recréée : ne bloque pas ----------------------------------------------------------

func TestBumperPointsBatch254_DeletedThenRecreatedTeamDoesNotBlockItsPlayers(t *testing.T) {
	e := newEnv244(t)
	e.send(t, e.admin, "admin", protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Verts", Points: 2})
	e.send(t, e.admin, "admin", protocol.ActionUpdate, e.deletePayload249("Les Verts"))
	e.recreate249(t, e.admin, "Les Verts")
	e.app.engine.SetPhase(game.PhaseRevealed)
	// un joueur dans la nouvelle équipe (le buzzer physique Verts a été désaffecté par la suppression : on le réaffecte)
	vert := e.buzzers["Les Verts"]
	st := e.state249()
	bumpers := map[string]interface{}{}
	for id, b := range st.Bumpers {
		bumpers[id] = b
	}
	if b, ok := st.Bumpers[vert]; ok {
		cp := map[string]interface{}{}
		for k, v := range b {
			cp[k] = v
		}
		cp["TEAM"] = "Les Verts"
		bumpers[vert] = cp
	}
	e.send(t, e.admin, "admin", protocol.ActionUpdate, map[string]interface{}{"teams": teamsMapOf243(e.state249()), "bumpers": bumpers})

	e.bbatch(t, qid244, []pentry254{{vert, 3}}, nil)

	r := e.bresult(t)
	if r.Status != "APPLIED" || len(r.Applied) != 1 || len(r.Skipped) != 0 {
		t.Errorf("une équipe recréée n'hérite pas du verrou d'équipe (TEAM_DELETED) : %+v", r)
	}
}

// --- 5. Non-régression : crédits individuels de plusieurs joueurs d'une équipe -----------------------------------------

func TestBumperPointsBatch254_AnotherPlayerOfTheTeamStaysCreditableAfterAnIndividualCredit(t *testing.T) {
	e := newEnv244(t)
	paul, buzz, _ := e.threePlayers() // même équipe : Bleus
	e.send(t, e.admin, "admin", protocol.ActionBumperPoints, map[string]interface{}{"ID": paul, "POINTS": 1})

	e.bbatch(t, qid244, []pentry254{{buzz, 2}}, nil)

	r := e.bresult(t)
	if len(r.Applied) != 1 || r.Applied[0].ID != buzz || len(r.Skipped) != 0 {
		t.Errorf("les crédits individuels d'un autre joueur de l'équipe ne bloquent pas : %+v", r)
	}
}

// --- 6. AWARDED_TEAMS.TEAMS[].TEAM_LEVEL ---------------------------------------------------------------------------

func TestAwardedTeams254_TeamLevelIsAlwaysPresentTrueOnlyForATeamCredit(t *testing.T) {
	e := newEnv244(t)
	paul, _, _ := e.threePlayers()
	e.send(t, e.admin, "admin", protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Verts", Points: 1}) // crédit d'équipe
	got := teamLevels254(t, e.adminConn)
	if v, ok := got["Les Verts"]; !ok || v == nil || !*v {
		t.Fatalf("TEAM_LEVEL présent et true après un crédit d'équipe : %v", got)
	}

	e.send(t, e.admin, "admin", protocol.ActionBumperPoints, map[string]interface{}{"ID": paul, "POINTS": 2}) // crédit joueur (Bleus)
	got = teamLevels254(t, e.adminConn)
	if v, ok := got["Les Bleus"]; !ok || v == nil || *v {
		t.Errorf("TEAM_LEVEL présent et false pour une équipe créditée seulement par ses joueurs : %v", got)
	}
	if v := got["Les Verts"]; v == nil || !*v {
		t.Errorf("Les Verts reste true : %v", got)
	}
	// l'anim reçoit la même projection
	if v := teamLevels254(t, e.animConn)["Les Verts"]; v == nil || !*v {
		t.Error("TEAM_LEVEL aussi dans l'AWARDED_TEAMS de l'anim")
	}
}

// --- Décodage tolérant de QCM_WRONG_PENALTY ----------------------------------------------------------------------------

func TestQuestion254_Critical_TolerantWrongPenaltyDecodingNeverBlocksALoadedQuestion(t *testing.T) {
	cases := map[string]string{
		"décimal -2.5": `-2.5`, "flottant entier -5.0": `-5.0`, "chaîne \"-5\"": `"-5"`, "texte": `"abc"`, "null": `null`, "positif": `3`, "hors bornes": `-150`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			app := newBroadcast127TestApp(t)
			dir := t.TempDir()
			app.config.Storage.QuestionsDir = dir
			qdir := filepath.Join(dir, "7")
			if err := os.MkdirAll(qdir, 0o755); err != nil {
				t.Fatal(err)
			}
			body := `{"ID":"7","TYPE":"QCM","QUESTION":"Q ?","ANSWER":"A","POINTS":"3","TIME":"30","QCM_ANSWERS":{"RED":"a","GREEN":"b","YELLOW":"c","BLUE":"d"},"QCM_CORRECT":"GREEN","QCM_WRONG_PENALTY":` + raw + `}`
			if err := os.WriteFile(filepath.Join(qdir, "question.json"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}

			q := app.loadQuestion("7")

			if q == nil {
				t.Fatalf("la question se charge malgré QCM_WRONG_PENALTY=%s", raw)
			}
			if int(q.QCMWrongPenalty) != 0 {
				t.Errorf("valeur invalide → pénalité 0, obtenu %d", q.QCMWrongPenalty)
			}
			app.engine.SetPhase(game.PhaseStopped)
			app.engine.Ready("", q)
			if app.engine.GetPhase() == game.PhaseStopped {
				t.Error("READY est possible avec cette question")
			}
		})
	}
	t.Run("valeur valide conservée", func(t *testing.T) {
		app := newBroadcast127TestApp(t)
		dir := t.TempDir()
		app.config.Storage.QuestionsDir = dir
		_ = os.MkdirAll(filepath.Join(dir, "8"), 0o755)
		_ = os.WriteFile(filepath.Join(dir, "8", "question.json"), []byte(`{"ID":"8","TYPE":"QCM","QUESTION":"Q","ANSWER":"A","POINTS":"3","TIME":"30","QCM_WRONG_PENALTY":-5}`), 0o644)
		if q := app.loadQuestion("8"); q == nil || int(q.QCMWrongPenalty) != -5 {
			t.Errorf("-5 est valide : %+v", q)
		}
	})
}
