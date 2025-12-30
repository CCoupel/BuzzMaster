package game

import (
	"encoding/json"
	"testing"
)

func TestGamePhase_Values(t *testing.T) {
	phases := []GamePhase{PhaseStop, PhasePrepare, PhaseReady, PhaseStart, PhasePause}
	expected := []string{"STOP", "PREPARE", "READY", "START", "PAUSE"}

	for i, phase := range phases {
		if string(phase) != expected[i] {
			t.Errorf("Expected phase %s, got %s", expected[i], phase)
		}
	}
}

func TestQuestionStatus_Values(t *testing.T) {
	statuses := []QuestionStatus{StatusAvailable, StatusStarted, StatusStopped, StatusRevealed}
	expected := []string{"AVAILABLE", "STARTED", "STOPPED", "REVEALED"}

	for i, status := range statuses {
		if string(status) != expected[i] {
			t.Errorf("Expected status %s, got %s", expected[i], status)
		}
	}
}

func TestTeam_JSONSerialization(t *testing.T) {
	team := &Team{
		Name:   "Team Red",
		Color:  []int{255, 0, 0},
		Score:  100,
		Time:   1234567890,
		Status: "PAUSE",
		Bumper: "b1",
		Ready:  true,
	}

	data, err := json.Marshal(team)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Team
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Name != team.Name {
		t.Errorf("Name mismatch: %s vs %s", decoded.Name, team.Name)
	}

	if decoded.Score != team.Score {
		t.Errorf("Score mismatch: %d vs %d", decoded.Score, team.Score)
	}

	if len(decoded.Color) != 3 {
		t.Errorf("Color should have 3 elements")
	}
}

func TestBumper_JSONSerialization(t *testing.T) {
	bumper := &Bumper{
		Name:    "Buzzer1",
		Team:    "red",
		Score:   50,
		Time:    1234567890,
		Button:  "A",
		Status:  "PAUSE",
		Version: "1.0.0",
		IP:      "192.168.4.2",
		Ready:   true,
	}

	data, err := json.Marshal(bumper)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Bumper
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Name != bumper.Name {
		t.Errorf("Name mismatch")
	}

	if decoded.Team != bumper.Team {
		t.Errorf("Team mismatch")
	}

	if decoded.IP != bumper.IP {
		t.Errorf("IP mismatch")
	}
}

func TestQuestion_JSONSerialization(t *testing.T) {
	question := &Question{
		ID:       "1",
		Question: "What is 2+2?",
		Answer:   "4",
		Points:   10,
		Time:     30,
		Media:    "/question/1/image.jpg",
		Status:   StatusAvailable,
	}

	data, err := json.Marshal(question)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Question
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ID != question.ID {
		t.Errorf("ID mismatch")
	}

	if decoded.Answer != question.Answer {
		t.Errorf("Answer mismatch")
	}

	if decoded.Status != StatusAvailable {
		t.Errorf("Status mismatch: %s", decoded.Status)
	}
}

func TestGameState_JSONSerialization(t *testing.T) {
	state := &GameState{
		Phase:       PhaseStart,
		Delay:       30,
		CurrentTime: 25,
		Question:    &Question{ID: "1"},
		Page:        "GAME",
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded GameState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Phase != PhaseStart {
		t.Errorf("Phase mismatch: %s", decoded.Phase)
	}

	if decoded.CurrentTime != 25 {
		t.Errorf("CurrentTime mismatch: %d", decoded.CurrentTime)
	}
}

func TestNewTeamsAndBumpers(t *testing.T) {
	tb := NewTeamsAndBumpers()

	if tb.Teams == nil {
		t.Error("Teams map should not be nil")
	}

	if tb.Bumpers == nil {
		t.Error("Bumpers map should not be nil")
	}

	if len(tb.Teams) != 0 {
		t.Error("Teams should be empty")
	}

	if len(tb.Bumpers) != 0 {
		t.Error("Bumpers should be empty")
	}
}

func TestTeamsAndBumpers_JSONSerialization(t *testing.T) {
	tb := &TeamsAndBumpers{
		Teams: map[string]*Team{
			"red":  {Name: "Team Red", Score: 100},
			"blue": {Name: "Team Blue", Score: 50},
		},
		Bumpers: map[string]*Bumper{
			"b1": {Name: "Buzzer1", Team: "red"},
			"b2": {Name: "Buzzer2", Team: "blue"},
		},
	}

	data, err := json.Marshal(tb)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded TeamsAndBumpers
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(decoded.Teams) != 2 {
		t.Errorf("Expected 2 teams, got %d", len(decoded.Teams))
	}

	if len(decoded.Bumpers) != 2 {
		t.Errorf("Expected 2 bumpers, got %d", len(decoded.Bumpers))
	}

	if decoded.Teams["red"].Score != 100 {
		t.Errorf("Red team score mismatch")
	}
}

func TestGameData_ToJSON(t *testing.T) {
	gameData := &GameData{
		Game: &GameState{
			Phase:       PhaseStart,
			CurrentTime: 20,
		},
		Teams: map[string]*Team{
			"red": {Name: "Team Red"},
		},
		Bumpers: map[string]*Bumper{
			"b1": {Name: "Buzzer1"},
		},
	}

	data, err := gameData.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, ok := decoded["GAME"]; !ok {
		t.Error("Should contain GAME field")
	}

	if _, ok := decoded["teams"]; !ok {
		t.Error("Should contain teams field")
	}

	if _, ok := decoded["bumpers"]; !ok {
		t.Error("Should contain bumpers field")
	}
}

func TestFullGameState_ToJSON(t *testing.T) {
	fullState := &FullGameState{
		GameState: GameState{
			Phase: PhaseStart,
			Delay: 30,
		},
		Teams: map[string]*Team{
			"red": {Name: "Team Red"},
		},
		Bumpers: map[string]*Bumper{
			"b1": {Name: "Buzzer1"},
		},
	}

	data, err := fullState.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Should have GameState fields at root level
	if decoded["PHASE"] != "START" {
		t.Errorf("PHASE mismatch: %v", decoded["PHASE"])
	}
}

func TestTeam_OmitEmpty(t *testing.T) {
	team := &Team{
		Name:  "Team Red",
		Score: 0, // Should NOT be omitted
	}

	data, err := json.Marshal(team)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Score of 0 should still be present
	if _, ok := decoded["SCORE"]; !ok {
		t.Error("SCORE should be present even when 0")
	}

	// Time of 0 should be omitted (omitempty)
	if _, ok := decoded["TIME"]; ok {
		t.Error("TIME should be omitted when 0")
	}
}
