// #282 (v11.4.0, QUALIF 11.4.0.2) — « le clic admin OK/KO n'est pas pris en compte » : frontière WS,
// l'admin envoie RAFALE_VALIDATE / RAFALE_INVALIDATE ; la question doit avancer (classique ET alphabétique).
package main

import (
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
)

func tw282JudgeApp(t *testing.T, alpha bool) *App {
	t.Helper()
	app := newAnimTestApp(t)
	for _, rq := range []game.RafaleQuestion{
		{ID: "j-1", Question: "Q1 ?", Answer: "Ail", Category: game.CategoryHistory, Difficulty: 1},
		{ID: "j-2", Question: "Q2 ?", Answer: "Banane", Category: game.CategoryHistory, Difficulty: 1},
		{ID: "j-3", Question: "Q3 ?", Answer: "Cerise", Category: game.CategoryHistory, Difficulty: 1},
	} {
		if _, err := app.engine.UpsertRafaleQuestion(rq); err != nil {
			t.Fatalf("UpsertRafaleQuestion : %v", err)
		}
	}
	q := &game.Question{ID: "judge-q", Question: "RAFALE", Type: game.QuestionTypeRafale,
		Category: game.CategoryHistory, Points: "10", Time: "120",
		TypedContent: game.TypedContent{RafaleDifficulty: 1, RafaleMode: string(game.RafaleModeSolo),
			RafaleQuestionTime: 30, RafaleMaxQuestions: 100, RafaleAlphabetical: alpha}}
	app.engine.Ready(q.ID, q)
	app.engine.StartImmediate(0)
	t.Cleanup(app.engine.Stop)
	return app
}

func TestRafaleAdminJudge_AdvancesQuestion(t *testing.T) {
	for _, alpha := range []bool{false, true} {
		for _, action := range []string{protocol.ActionRafaleValidate, protocol.ActionRafaleInvalidate} {
			name := action
			if alpha {
				name += "/alpha"
			}
			t.Run(name, func(t *testing.T) {
				app := tw282JudgeApp(t, alpha)
				baseURL := startAnimAllowlistTestServer(t, app)
				admin := dialWS(t, baseURL, "/ws/admin")
				learnClientID(t, app, admin)

				before := app.engine.GetState().RafaleCurrentQuestion.ID
				sendAction(t, app, admin, action, struct{}{})

				deadline := time.Now().Add(2 * time.Second)
				for time.Now().Before(deadline) {
					if app.engine.GetState().RafaleCurrentQuestion.ID != before {
						return
					}
					time.Sleep(20 * time.Millisecond)
				}
				t.Fatalf("%s admin non pris en compte : la question courante reste %q", action, before)
			})
		}
	}
}
