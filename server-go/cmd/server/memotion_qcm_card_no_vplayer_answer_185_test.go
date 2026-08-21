// Test for #185 C-B2's negative guarantee (contract §7.1): VPLAYER_QCM_ANSWER
// must have zero effect on a MEMOTION question, even when the active card is
// QCM-typed — handleVPlayerQCMAnswer gates on state.Question.Type==QCM
// (main.go), which is never true for a MEMOTION question regardless of which
// card is selected, so this is structurally guaranteed rather than something
// #185 had to add — this test pins it down explicitly as the task requested.
//
// Run: go test ./cmd/server/... -run TestHandleVPlayerQCMAnswer_MEMOTION -v

package main

import (
	"buzzcontrol/internal/game"
	"testing"
)

func TestHandleVPlayerQCMAnswer_MEMOTIONQuestion_QCMCard_NoEffect(t *testing.T) {
	app := newBroadcast127TestApp(t)
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	vplayerID := setupVirtualPlayer(t, app, "Player1", "TeamA") // CreateVirtualPlayer already sets IsVPlayer=true

	app.engine.SetPhase(game.PhaseStopped)
	qcmCard := game.MotionCard{
		ID: "mc-1", RectoTheme: "Capitales", Difficulty: 1, Type: game.QuestionTypeQCM,
		TypedContent: game.TypedContent{
			QCMAnswers: &game.QCMAnswers{Red: "Paris", Green: "Lyon", Yellow: "Nice", Blue: "Metz"},
			QCMCorrect: "RED",
		},
	}
	app.engine.Ready("mq1", &game.Question{
		ID: "mq1", Type: game.QuestionTypeMemotion,
		MotionCards: []game.MotionCard{qcmCard},
		MotionMode:  "SOLO",
	})
	app.engine.SetPhase(game.PhaseStarted)
	app.engine.InitMotionState()
	if err := app.engine.SelectMotionCard("mc-1"); err != nil {
		t.Fatalf("SelectMotionCard failed: %v", err)
	}
	if err := app.engine.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard failed: %v", err)
	}

	before := app.engine.GetState()

	app.handleVPlayerQCMAnswer(vplayerID, qcmAnswerMsg(t, vplayerID, "RED"))

	after := app.engine.GetState()
	if after.MotionSubPhase != before.MotionSubPhase {
		t.Errorf("MotionSubPhase changed after VPLAYER_QCM_ANSWER on a QCM MEMOTION card: %q → %q", before.MotionSubPhase, after.MotionSubPhase)
	}
	if len(after.QcmInvalidated) != 0 {
		t.Errorf("question-scoped QcmInvalidated should stay empty (no QCM question is in play), got %v", after.QcmInvalidated)
	}
	if v, exists := after.MotionActive.State["QCM_INVALIDATED"]; exists {
		t.Errorf("card-scoped QCM_INVALIDATED should be untouched (absent) by VPLAYER_QCM_ANSWER, got %v", v)
	}

	bumper := app.engine.GetBumper(vplayerID)
	if bumper == nil {
		t.Fatal("VPlayer bumper disappeared")
	}
	if bumper.AnswerColor != game.AnswerColorNone {
		t.Errorf("bumper.AnswerColor = %q, want unset — VPLAYER_QCM_ANSWER must be rejected before recording anything", bumper.AnswerColor)
	}
}
