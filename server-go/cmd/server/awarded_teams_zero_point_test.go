package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests: #170 — crédit à 0 point ("0 pt", le geste de refus), de bout en
// bout (T3). Le plan (_work/reports/plan-20260816-125123.md §2) documente
// que ce comportement est déjà correct dans le code EXISTANT et inchangé
// par #170 (AddGameEvent inconditionnel, moteur indifférent au montant,
// LED conditionnée à Points > 0) — ces tests CONFIRMENT ce constat plutôt
// que d'exercer une logique nouvelle, pour qu'une régression future sur ce
// chemin partagé (crédit ordinaire ET refus) soit détectée.
// ---------------------------------------------------------------------------

func TestZeroPointCredit_TeamPoints_EventRecordedScoreUnchangedNoLED(t *testing.T) {
	app := newAwardedTeamsTriggerTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{"Les Rouges": {Name: "Les Rouges", Color: []int{239, 68, 68}}})
	app.engine.UpdateBumper("aa:bb:cc:dd:ee:ff", map[string]interface{}{"TEAM": "Les Rouges"})
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	app.engine.Ready("1", &game.Question{ID: "1"})

	scoreBefore := app.engine.GetTeam("Les Rouges").Score

	msg, _ := protocol.NewMessage(protocol.ActionTeamPoints, nil)
	payloadData, _ := json.Marshal(protocol.TeamPointsPayload{Team: "Les Rouges", Points: 0})
	msg.Msg = payloadData
	app.handleTeamPoints(msg)

	// 1. Événement enregistré, Points=0, sans condition.
	history := app.engine.GetHistory()
	found := false
	for _, ev := range history {
		if ev.EventType == "POINTS_AWARDED" && ev.TeamName == "Les Rouges" && ev.QuestionID == "1" {
			found = true
			if ev.Points != 0 {
				t.Errorf("expected recorded Points=0, got %d", ev.Points)
			}
		}
	}
	if !found {
		t.Fatal("expected a POINTS_AWARDED event to be recorded even for a 0-point credit (refusal)")
	}

	// 2. Score inchangé.
	scoreAfter := app.engine.GetTeam("Les Rouges").Score
	if scoreAfter != scoreBefore {
		t.Errorf("expected score unchanged (%d), got %d", scoreBefore, scoreAfter)
	}

	// 3. Aucune LED déclenchée — sendLEDSetComet est conditionné à
	// Points > 0 (handleTeamPoints, code inchangé par #170) : aucun état LED
	// n'a dû être posé pour ce bumper.
	if _, ok := app.bumperLEDState["aa:bb:cc:dd:ee:ff"]; ok {
		t.Error("expected no LED state set for the bumper after a 0-point credit (no comet effect)")
	}
}

func TestZeroPointCredit_BumperPoints_EventRecordedScoreUnchangedNoLED(t *testing.T) {
	app := newAwardedTeamsTriggerTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{"Les Rouges": {Name: "Les Rouges", Color: []int{239, 68, 68}}})
	app.engine.UpdateBumper("aa:bb:cc:dd:ee:ff", map[string]interface{}{"TEAM": "Les Rouges", "NAME": "Alice"})
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	app.engine.Ready("1", &game.Question{ID: "1"})

	scoreBefore := app.engine.GetTeam("Les Rouges").Score

	msg, _ := protocol.NewMessage(protocol.ActionBumperPoints, nil)
	payloadData, _ := json.Marshal(protocol.BumperPointsPayload{ID: "aa:bb:cc:dd:ee:ff", Points: 0})
	msg.Msg = payloadData
	app.handleBumperPoints(msg)

	history := app.engine.GetHistory()
	found := false
	for _, ev := range history {
		if ev.EventType == "POINTS_AWARDED" && ev.TeamName == "Les Rouges" && ev.QuestionID == "1" {
			found = true
			if ev.Points != 0 {
				t.Errorf("expected recorded Points=0, got %d", ev.Points)
			}
		}
	}
	if !found {
		t.Fatal("expected a POINTS_AWARDED event to be recorded for a 0-point BUMPER_POINTS credit (SPEEDY refusal)")
	}

	scoreAfter := app.engine.GetTeam("Les Rouges").Score
	if scoreAfter != scoreBefore {
		t.Errorf("expected score unchanged (%d), got %d", scoreBefore, scoreAfter)
	}

	if _, ok := app.bumperLEDState["aa:bb:cc:dd:ee:ff"]; ok {
		t.Error("expected no LED state set for the bumper after a 0-point BUMPER_POINTS credit")
	}
}

// TestZeroPointCredit_LocksLikeAnyOtherCredit is the direct link to T1/R1:
// a 0-point credit must appear in AWARDED_TEAMS exactly like a positive one
// — the lock is on presence, not on the amount.
func TestZeroPointCredit_LocksLikeAnyOtherCredit(t *testing.T) {
	app := newAwardedTeamsTriggerTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{"Les Rouges": {Name: "Les Rouges"}})
	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "1", 1, nil)
	app.engine.Ready("1", &game.Question{ID: "1"})

	msg, _ := protocol.NewMessage(protocol.ActionTeamPoints, nil)
	payloadData, _ := json.Marshal(protocol.TeamPointsPayload{Team: "Les Rouges", Points: 0})
	msg.Msg = payloadData
	app.handleTeamPoints(msg)

	got := app.getAwardedTeamsPayload()
	if len(got.Teams) != 1 || got.Teams[0].Team != "Les Rouges" || got.Teams[0].Points != 0 {
		t.Errorf("expected Les Rouges locked with Points=0, got %+v", got.Teams)
	}
}
