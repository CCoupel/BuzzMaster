package main

// #256 (v11.2) — la fenêtre « déjà créditée » s'ouvre à la SÉLECTION de la question (Ready) : une question rejouée repart
// sans crédit, vu des clients (AWARDED_TEAMS) et de l'attribution groupée. Compléments aux tests moteur
// internal/game/game_time_256_test.go (dev-backend) : chemin de bout en bout par les handlers.

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"testing"
	"time"
)

func TestAwardedReplay256_Critical_AReplayedQuestionIsCreditableAgainAndAnnouncesNoCredit(t *testing.T) {
	e := newEnv244(t)
	e.batch(t, qid244, []entry244{{"Les Bleus", 3}}, nil)
	if r := e.result(t); r.Status != "APPLIED" || len(r.Applied) != 1 {
		t.Fatalf("précondition (manche 1) : %+v", r)
	}
	if got := awardedTeamsOf244(t, e.adminConn); got["Les Bleus"] != 3 { // consomme l'annonce de la manche 1
		t.Fatalf("précondition : Bleus crédité en manche 1 : %v", got)
	}
	time.Sleep(5 * time.Millisecond)

	// même question rejouée : Ready, START, STOP, REVEAL
	e.app.engine.SetPhase(game.PhaseStopped)
	e.app.engine.Ready("", &game.Question{ID: qid244, Type: game.QuestionTypeArdoise, Points: "3"})
	e.app.broadcastAwardedTeams()
	if got := awardedTeamsOf244(t, e.adminConn); len(got) != 0 {
		t.Fatalf("entre Ready et START d'une question rejouée, AWARDED_TEAMS est vide (admin) : %v", got)
	}
	e.app.engine.SetPhase(game.PhaseStarted)
	e.app.engine.Stop()
	e.app.engine.Reveal()

	e.batch(t, qid244, []entry244{{"Les Bleus", 2}}, nil)

	r := e.result(t)
	if r.Status != "APPLIED" || len(r.Applied) != 1 || len(r.Skipped) != 0 {
		t.Errorf("la manche 2 n'hérite pas du verrou « déjà créditée » de la manche 1 : %+v", r)
	}
}

func TestAwardedReplay256_CreditsOfTheNewRoundAreCountedAfterStart(t *testing.T) {
	e := newEnv244(t)
	e.app.engine.SetPhase(game.PhaseStopped)
	e.app.engine.Ready("", &game.Question{ID: qid244, Type: game.QuestionTypeArdoise, Points: "3"})
	e.app.engine.SetPhase(game.PhaseStarted)
	e.app.engine.Stop()
	e.app.engine.Reveal()

	e.send(t, e.admin, "admin", protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Rouges", Points: 2})

	if got := awardedTeamsOf244(t, e.adminConn); got["Les Rouges"] != 2 {
		t.Errorf("un crédit de la manche courante est compté : %v", got)
	}
}
