package game

import "testing"

// #255: ANSWERED_COLOR is the answer to the CURRENT question; reset with Time.
func TestAnsweredColor_DevMinimal_SetOnQCMBuzzAndClearedOnReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"A": {Name: "A"}})
	e.SetBumpers(map[string]*Bumper{"m1": {Name: "m1", Team: "A", AnswerColor: AnswerColorRed}})
	e.SetPhase(PhaseStopped)
	e.Ready("", &Question{ID: "q1", Type: QuestionTypeQCM})
	if b := e.GetBumper("m1"); b.AnsweredColor != "" || b.AnswerColor != AnswerColorRed {
		t.Fatalf("avant réponse : ANSWERED_COLOR vide, couleur assignée conservée : %+v", b)
	}
	e.StartImmediate(5)

	e.ProcessButtonPress("m1", 1000, "B") // GREEN

	b := e.GetBumper("m1")
	if b.AnsweredColor != AnswerColorGreen || b.Time == 0 {
		t.Fatalf("buzz QCM enregistré : ANSWERED_COLOR=GREEN, Time>0 : %+v", b)
	}
	e.Stop()
	e.SetPhase(PhaseStopped)
	e.Ready("", &Question{ID: "q2", Type: QuestionTypeQCM})
	b = e.GetBumper("m1")
	if b.AnsweredColor != "" || b.Time != 0 {
		t.Fatalf("Ready remet ANSWERED_COLOR et Time à zéro : %+v", b)
	}
	if b.AnswerColor != AnswerColorGreen {
		t.Errorf("ANSWER_COLOR (dernière couleur / assignée) inchangé par Ready : %s", b.AnswerColor)
	}
}
