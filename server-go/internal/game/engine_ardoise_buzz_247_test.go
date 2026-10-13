package game

import (
	"sync/atomic"
	"testing"
)

// #247 — un appui sur buzzer physique pendant une ARDOISE doit être refusé par
// le moteur, comme en MEMORY, MEMOTION, RAFALE et ENTRACTE : aucun temps
// enregistré, statut inchangé, callback OnBuzzerPress non appelé, phase STARTED.
// Comportement observable uniquement (indépendant de la signature de
// ProcessButtonPress).

func ardoise247(id string) *Question {
	return &Question{ID: id, Question: "Quel est ... ?", Type: QuestionTypeArdoise}
}

func engine247(t *testing.T) (*Engine, *int32) {
	t.Helper()
	e := NewEngine()
	e.SetTeams(map[string]*Team{"teamA": {Name: "A"}})
	e.SetBumpers(map[string]*Bumper{
		"b1": {Name: "P1", Team: "teamA"},
	})
	var calls int32
	e.OnBuzzerPress = func(string, string, int64, string) { atomic.AddInt32(&calls, 1) }
	return e, &calls
}

func TestProcessButtonPress_IgnoresArdoise_247(t *testing.T) {
	e, calls := engine247(t)
	e.Ready("qa", ardoise247("qa"))
	e.StartImmediate(30)
	defer e.Stop()

	statusBefore := e.GetTeam("teamA").Status
	e.ProcessButtonPress("b1", 1000000, "A")

	if b := e.GetBumper("b1"); b.Time != 0 || b.Status == "PAUSE" {
		t.Errorf("appui en ARDOISE : Time doit rester 0 et Status != PAUSE, got Time=%d Status=%q", b.Time, b.Status)
	}
	team := e.GetTeam("teamA")
	if team.Time != 0 || team.Status != statusBefore {
		t.Errorf("appui en ARDOISE : l'équipe ne doit pas changer, got Time=%d Status=%q (avant %q)", team.Time, team.Status, statusBefore)
	}
	if e.GetPhase() != PhaseStarted {
		t.Errorf("la phase doit rester STARTED, got %s", e.GetPhase())
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Errorf("OnBuzzerPress ne doit pas être appelé pour un appui refusé en ARDOISE, appels=%d", n)
	}
}

// Non-régression : hors ARDOISE (SPEEDY) l'appui reste accepté.
func TestProcessButtonPress_StillAcceptedOnSpeedy_247(t *testing.T) {
	e, _ := engine247(t)
	e.Ready("qs", &Question{ID: "qs", Question: "?", Type: QuestionTypeSpeedy})
	e.StartImmediate(30)
	defer e.Stop()

	e.ProcessButtonPress("b1", 1000000, "A")
	if b := e.GetBumper("b1"); b.Time != 1000000 || b.Status != "PAUSE" {
		t.Errorf("SPEEDY : appui accepté attendu, got Time=%d Status=%q", b.Time, b.Status)
	}
}
