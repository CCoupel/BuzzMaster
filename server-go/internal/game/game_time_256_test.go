package game

import (
	"testing"
	"time"
)

func awarded256(e *Engine) []AwardedTeam {
	_, aw := e.AwardedTeams()
	return aw
}

// #256: a replayed question starts with no credit between Ready and START.
func TestAwarded_ReplayedQuestionIgnoresThePreviousRoundFromReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"Rouges": {Name: "Rouges"}})
	e.SetPhase(PhaseStopped)
	e.Ready("", &Question{ID: "q1", Type: QuestionTypeQCM})
	e.StartImmediate(5)
	e.AddGameEvent(GameEvent{EventType: "POINTS_AWARDED", QuestionID: "q1", WinnerType: "TEAM", TeamName: "Rouges", Points: 3, Timestamp: time.Now().UnixMicro()})
	if len(awarded256(e)) != 1 {
		t.Fatal("précondition : crédité pendant la manche")
	}

	time.Sleep(2 * time.Millisecond)
	e.SetPhase(PhaseStopped)
	e.Ready("", &Question{ID: "q1", Type: QuestionTypeQCM}) // rejouée

	if aw := awarded256(e); len(aw) != 0 {
		t.Fatalf("entre Ready et START d'une question rejouée, rien n'est crédité : %v", aw)
	}
}

// #256: START -> REVEAL without any credit leaves AWARDED_TEAMS empty.
func TestAwarded_StartToRevealWithoutCreditIsEmpty(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"Rouges": {Name: "Rouges"}, "Verts": {Name: "Verts"}})
	e.SetPhase(PhaseStopped)
	e.Ready("", &Question{ID: "q2", Type: QuestionTypeQCM})
	e.StartImmediate(5)
	e.Stop()
	e.Reveal()
	if aw := awarded256(e); len(aw) != 0 {
		t.Fatalf("aucun crédit attribué : AWARDED_TEAMS vide, obtenu %v", aw)
	}
}

// #256: after a restart (new engine) an old history of the same QuestionID does not count.
func TestAwarded_RestartDoesNotCountTheReloadedHistory(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"Rouges": {Name: "Rouges"}})
	// reloaded history: an old credit for q3, older than this engine's lifetime
	e.history = append(e.history, GameEvent{EventType: "POINTS_AWARDED", QuestionID: "q3", WinnerType: "TEAM", TeamName: "Rouges", Points: 0, Timestamp: time.Now().Add(-time.Hour).UnixMicro()})
	e.SetPhase(PhaseStopped)
	e.state.Question = &Question{ID: "q3", Type: QuestionTypeQCM} // restored current question, not started yet
	if aw := awarded256(e); len(aw) != 0 {
		t.Fatalf("l'historique rechargé ne compte pas après redémarrage : %v", aw)
	}
}
