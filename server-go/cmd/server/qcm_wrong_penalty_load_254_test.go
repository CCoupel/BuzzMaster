package main

import (
	"buzzcontrol/internal/game"
	"os"
	"path/filepath"
	"testing"
)

// #254 review: a question file whose QCM_WRONG_PENALTY is invalid still loads (penalty 0)
// and can reach READY — it must never become unplayable.
func TestLoadQuestion_InvalidQCMWrongPenaltyStillLoadsAndReachesReady(t *testing.T) {
	for _, raw := range []string{`-2.5`, `-5.0`, `"-5"`, `"abc"`, `null`, `7`, `-500`} {
		app := newAwardedTeamsTriggerTestApp(t)
		dir := app.config.Storage.QuestionsDir
		qdir := filepath.Join(dir, "p1")
		if err := os.MkdirAll(qdir, 0o755); err != nil {
			t.Fatal(err)
		}
		doc := `{"ID":"p1","QUESTION":"Q?","ANSWER":"A","TYPE":"QCM","POINTS":"10","TIME":"8","QCM_CORRECT":"RED",` +
			`"QCM_ANSWERS":{"RED":"a","GREEN":"b","YELLOW":"c","BLUE":"d"},"QCM_WRONG_PENALTY":` + raw + `}`
		if err := os.WriteFile(filepath.Join(qdir, "question.json"), []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}

		q := app.loadQuestion("p1")
		if q == nil {
			t.Fatalf("QCM_WRONG_PENALTY=%s : loadQuestion ne doit jamais retourner nil", raw)
		}
		if q.QCMWrongPenalty != 0 {
			t.Errorf("QCM_WRONG_PENALTY=%s -> %d, attendu 0", raw, q.QCMWrongPenalty)
		}
		app.engine.Ready("p1", q)
		if st := app.engine.GetState(); st.Question == nil || st.Question.ID != "p1" || st.Phase != game.PhasePrepare {
			t.Errorf("QCM_WRONG_PENALTY=%s : READY impossible (phase %s)", raw, st.Phase)
		}
	}
}
