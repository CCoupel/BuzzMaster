package game

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine()

	if e.GetPhase() != PhaseStopped {
		t.Errorf("Expected initial phase STOPPED, got %s", e.GetPhase())
	}

	state := e.GetState()
	if state.Delay != 30 {
		t.Errorf("Expected default delay 30, got %d", state.Delay)
	}

	if state.Page != "GAME" {
		t.Errorf("Expected default page GAME, got %s", state.Page)
	}
}

func TestEngine_UpdateBumper(t *testing.T) {
	e := NewEngine()

	data := map[string]interface{}{
		"NAME":    "Buzzer1",
		"TEAM":    "red",
		"VERSION": "1.0.0",
		"IP":      "192.168.4.2",
	}

	e.UpdateBumper("bumper1", data)

	bumper := e.GetBumper("bumper1")
	if bumper == nil {
		t.Fatal("Bumper should exist")
	}

	if bumper.Name != "Buzzer1" {
		t.Errorf("Expected name Buzzer1, got %s", bumper.Name)
	}

	if bumper.Team != "red" {
		t.Errorf("Expected team red, got %s", bumper.Team)
	}

	if bumper.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", bumper.Version)
	}

	if bumper.IP != "192.168.4.2" {
		t.Errorf("Expected IP 192.168.4.2, got %s", bumper.IP)
	}
}

func TestEngine_UpdateBumper_Partial(t *testing.T) {
	e := NewEngine()

	// First update
	e.UpdateBumper("bumper1", map[string]interface{}{
		"NAME": "Buzzer1",
		"TEAM": "red",
	})

	// Second update (partial)
	e.UpdateBumper("bumper1", map[string]interface{}{
		"IP": "192.168.4.5",
	})

	bumper := e.GetBumper("bumper1")

	// Original values should persist
	if bumper.Name != "Buzzer1" {
		t.Errorf("Name should persist, got %s", bumper.Name)
	}

	if bumper.Team != "red" {
		t.Errorf("Team should persist, got %s", bumper.Team)
	}

	// New value should be set
	if bumper.IP != "192.168.4.5" {
		t.Errorf("IP should be updated, got %s", bumper.IP)
	}
}

func TestEngine_SetTeams(t *testing.T) {
	e := NewEngine()

	teams := map[string]*Team{
		"red": {
			Name:  "Team Red",
			Color: []int{255, 0, 0},
			Score: 100,
		},
		"blue": {
			Name:  "Team Blue",
			Color: []int{0, 0, 255},
			Score: 50,
		},
	}

	e.SetTeams(teams)

	redTeam := e.GetTeam("red")
	if redTeam == nil {
		t.Fatal("Red team should exist")
	}

	if redTeam.Score != 100 {
		t.Errorf("Expected red team score 100, got %d", redTeam.Score)
	}
}

func TestEngine_Ready(t *testing.T) {
	e := NewEngine()

	// Add teams and bumpers
	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	// Call Ready
	e.Ready("q1", &Question{ID: "q1", Answer: "42"})

	if e.GetPhase() != PhasePrepare {
		t.Errorf("Expected phase PREPARE, got %s", e.GetPhase())
	}

	// Bumper should be reset
	bumper := e.GetBumper("b1")
	if bumper.Time != 0 {
		t.Error("Bumper time should be reset")
	}

	if bumper.Ready {
		t.Error("Bumper should not be ready yet")
	}
}

func TestEngine_SetBumperReady(t *testing.T) {
	e := NewEngine()

	// Setup team and bumper
	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	// Prepare game
	e.Ready("q1", nil)

	// Mark bumper ready
	e.SetBumperReady("b1")

	bumper := e.GetBumper("b1")
	if !bumper.Ready {
		t.Error("Bumper should be ready")
	}

	// Team should also be ready
	team := e.GetTeam("red")
	if !team.Ready {
		t.Error("Team should be ready when all bumpers are ready")
	}
}

func TestEngine_AreAllTeamsReady(t *testing.T) {
	e := NewEngine()

	// No teams = not ready
	if e.AreAllTeamsReady() {
		t.Error("Should not be ready with no teams")
	}

	// Setup two teams with bumpers
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red"},
		"blue": {Name: "Team Blue"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})

	e.Ready("q1", nil)

	// Only one ready
	e.SetBumperReady("b1")
	if e.AreAllTeamsReady() {
		t.Error("Should not be ready with only one team ready")
	}

	// Both ready
	e.SetBumperReady("b2")
	if !e.AreAllTeamsReady() {
		t.Error("Should be ready when all teams are ready")
	}
}

func TestEngine_Start(t *testing.T) {
	e := NewEngine()

	e.Start(20)

	if e.GetPhase() != PhaseStarted {
		t.Errorf("Expected phase STARTED, got %s", e.GetPhase())
	}

	state := e.GetState()
	if state.CurrentTime != 20 {
		t.Errorf("Expected current time 20, got %d", state.CurrentTime)
	}

	if state.Delay != 20 {
		t.Errorf("Expected delay 20, got %d", state.Delay)
	}

	// Clean up timer
	e.Stop()
}

func TestEngine_Stop(t *testing.T) {
	e := NewEngine()

	e.Start(30)
	e.Stop()

	if e.GetPhase() != PhaseStopped {
		t.Errorf("Expected phase STOPPED, got %s", e.GetPhase())
	}

	state := e.GetState()
	if state.CurrentTime != 0 {
		t.Errorf("Expected current time 0 after stop, got %d", state.CurrentTime)
	}
}

func TestEngine_Pause(t *testing.T) {
	e := NewEngine()

	e.Start(30)
	e.Pause()

	if e.GetPhase() != PhasePaused {
		t.Errorf("Expected phase PAUSED, got %s", e.GetPhase())
	}

	e.Stop() // cleanup
}

func TestEngine_Continue(t *testing.T) {
	e := NewEngine()

	e.Start(30)
	e.Pause()
	e.Continue()

	if e.GetPhase() != PhaseStarted {
		t.Errorf("Expected phase STARTED after continue, got %s", e.GetPhase())
	}

	e.Stop() // cleanup
}

func TestEngine_ProcessButtonPress(t *testing.T) {
	e := NewEngine()

	// Setup
	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	// Must be in START phase
	e.Start(30)

	pressTime := time.Now().UnixMicro()
	e.ProcessButtonPress("b1", pressTime, "A")

	bumper := e.GetBumper("b1")
	if bumper.Time != pressTime {
		t.Errorf("Expected bumper time %d, got %d", pressTime, bumper.Time)
	}

	if bumper.Button != "A" {
		t.Errorf("Expected button A, got %s", bumper.Button)
	}

	if bumper.Status != "PAUSE" {
		t.Errorf("Expected status PAUSE, got %s", bumper.Status)
	}

	// Team should also be updated
	team := e.GetTeam("red")
	if team.Time != pressTime {
		t.Errorf("Team time should be updated")
	}

	if team.Bumper != "b1" {
		t.Errorf("Team bumper should be b1, got %s", team.Bumper)
	}

	e.Stop()
}

func TestEngine_ProcessButtonPress_IgnoresWhenNotStarted(t *testing.T) {
	e := NewEngine()

	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	// Game is STOP, press should be ignored
	e.ProcessButtonPress("b1", time.Now().UnixMicro(), "A")

	bumper := e.GetBumper("b1")
	if bumper.Time != 0 {
		t.Error("Press should be ignored when game not started")
	}
}

func TestEngine_ProcessButtonPress_IgnoresDoublePress(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.Start(30)

	firstPress := int64(1000000)
	secondPress := int64(2000000)

	e.ProcessButtonPress("b1", firstPress, "A")
	e.ProcessButtonPress("b1", secondPress, "B")

	bumper := e.GetBumper("b1")
	if bumper.Time != firstPress {
		t.Errorf("Time should be first press %d, got %d", firstPress, bumper.Time)
	}

	if bumper.Button != "A" {
		t.Errorf("Button should be A (first press), got %s", bumper.Button)
	}

	e.Stop()
}

func TestEngine_ProcessButtonPress_FastestWins(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("b2", map[string]interface{}{"TEAM": "red"})
	e.Start(30)

	// b2 presses first (lower time = earlier)
	e.ProcessButtonPress("b2", 1000, "A")
	e.ProcessButtonPress("b1", 2000, "A")

	team := e.GetTeam("red")
	if team.Time != 1000 {
		t.Errorf("Team time should be fastest (1000), got %d", team.Time)
	}

	if team.Bumper != "b2" {
		t.Errorf("Team bumper should be b2 (fastest), got %s", team.Bumper)
	}

	e.Stop()
}

func TestEngine_UpdateScore(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	bumperScore, teamScore := e.UpdateScore("b1", 10)

	if bumperScore != 10 {
		t.Errorf("Expected bumper score 10, got %d", bumperScore)
	}

	if teamScore != 10 {
		t.Errorf("Expected team score 10, got %d", teamScore)
	}

	// Add more points
	bumperScore, teamScore = e.UpdateScore("b1", 5)

	if bumperScore != 15 {
		t.Errorf("Expected bumper score 15, got %d", bumperScore)
	}

	if teamScore != 15 {
		t.Errorf("Expected team score 15, got %d", teamScore)
	}
}

func TestEngine_UpdateScore_NegativePoints(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	e.UpdateScore("b1", 10)
	bumperScore, teamScore := e.UpdateScore("b1", -5)

	if bumperScore != 5 {
		t.Errorf("Expected bumper score 5, got %d", bumperScore)
	}

	if teamScore != 5 {
		t.Errorf("Expected team score 5, got %d", teamScore)
	}
}

func TestEngine_RAZScores(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red", Score: 100},
		"blue": {Name: "Team Blue", Score: 50},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateScore("b1", 25)

	e.RAZScores()

	bumper := e.GetBumper("b1")
	if bumper.Score != 0 {
		t.Errorf("Bumper score should be 0, got %d", bumper.Score)
	}

	redTeam := e.GetTeam("red")
	if redTeam.Score != 0 {
		t.Errorf("Red team score should be 0, got %d", redTeam.Score)
	}

	blueTeam := e.GetTeam("blue")
	if blueTeam.Score != 0 {
		t.Errorf("Blue team score should be 0, got %d", blueTeam.Score)
	}
}

func TestEngine_ClearBumpers(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	e.ClearBumpers()

	if e.GetBumper("b1") != nil {
		t.Error("Bumper should be cleared")
	}

	if e.GetTeam("red") != nil {
		t.Error("Team should be cleared")
	}
}

func TestEngine_SetPage(t *testing.T) {
	e := NewEngine()

	e.SetPage("CONFIG")
	state := e.GetState()
	if state.Page != "CONFIG" {
		t.Errorf("Expected page CONFIG, got %s", state.Page)
	}

	// Empty string should default to GAME
	e.SetPage("")
	state = e.GetState()
	if state.Page != "GAME" {
		t.Errorf("Empty string should default to GAME, got %s", state.Page)
	}

	// null should default to GAME
	e.SetPage("null")
	state = e.GetState()
	if state.Page != "GAME" {
		t.Errorf("null should default to GAME, got %s", state.Page)
	}
}

func TestEngine_Reveal(t *testing.T) {
	e := NewEngine()

	// No question
	answer := e.Reveal()
	if answer != "" {
		t.Errorf("Expected empty answer with no question, got %s", answer)
	}

	// With question
	e.Ready("q1", &Question{ID: "q1", Answer: "42"})
	answer = e.Reveal()

	if answer != "42" {
		t.Errorf("Expected answer 42, got %s", answer)
	}
}

func TestEngine_GetGameJSON(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Score: 100},
	})

	jsonData := e.GetGameJSON()

	if len(jsonData) == 0 {
		t.Error("JSON should not be empty")
	}

	// Verify it's valid JSON
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		t.Errorf("Invalid JSON: %v", err)
	}
}

func TestEngine_GetTeamsAndBumpersJSON(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	jsonData := e.GetTeamsAndBumpersJSON()

	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		t.Errorf("Invalid JSON: %v", err)
	}

	if _, ok := data["teams"]; !ok {
		t.Error("JSON should contain teams")
	}

	if _, ok := data["bumpers"]; !ok {
		t.Error("JSON should contain bumpers")
	}
}

func TestEngine_PhaseChecks(t *testing.T) {
	e := NewEngine()

	// Initial state
	if !e.IsGameStopped() {
		t.Error("Should be stopped initially")
	}

	// Prepare
	e.Ready("q1", nil)
	if !e.IsGamePrepare() {
		t.Error("Should be in prepare phase")
	}

	// Start
	e.Start(30)
	if !e.IsGameStarted() {
		t.Error("Should be started")
	}

	e.Stop()
}

func TestEngine_StateChangeCallback(t *testing.T) {
	e := NewEngine()

	var lastPhase GamePhase
	e.OnStateChange = func(phase GamePhase) {
		lastPhase = phase
	}

	e.Ready("q1", nil)
	if lastPhase != PhasePrepare {
		t.Errorf("Callback should receive PREPARE, got %s", lastPhase)
	}

	e.Start(30)
	if lastPhase != PhaseStarted {
		t.Errorf("Callback should receive STARTED, got %s", lastPhase)
	}

	e.Pause()
	if lastPhase != PhasePaused {
		t.Errorf("Callback should receive PAUSED, got %s", lastPhase)
	}

	e.Stop()
	if lastPhase != PhaseStopped {
		t.Errorf("Callback should receive STOPPED, got %s", lastPhase)
	}
}

func TestEngine_SetBumpers_SyncsVirtualPlayerCount(t *testing.T) {
	e := NewEngine()

	// Start with no bumpers
	if e.GetVirtualPlayerCount() != 0 {
		t.Errorf("Expected initial virtual player count 0, got %d", e.GetVirtualPlayerCount())
	}

	// Add 2 virtual and 1 physical bumper
	bumpers := map[string]*Bumper{
		"virtual1": {Name: "Player1", IsVirtual: true},
		"virtual2": {Name: "Player2", IsVirtual: true},
		"buzzer1":  {Name: "Buzzer1", IsVirtual: false},
	}
	e.SetBumpers(bumpers)

	// Should have 2 virtual players
	if e.GetVirtualPlayerCount() != 2 {
		t.Errorf("Expected virtual player count 2, got %d", e.GetVirtualPlayerCount())
	}

	// Remove one virtual player
	delete(bumpers, "virtual1")
	e.SetBumpers(bumpers)

	// Should have 1 virtual player
	if e.GetVirtualPlayerCount() != 1 {
		t.Errorf("Expected virtual player count 1 after deletion, got %d", e.GetVirtualPlayerCount())
	}

	// Remove all virtual players
	delete(bumpers, "virtual2")
	e.SetBumpers(bumpers)

	// Should have 0 virtual players (only physical buzzer remains)
	if e.GetVirtualPlayerCount() != 0 {
		t.Errorf("Expected virtual player count 0, got %d", e.GetVirtualPlayerCount())
	}
}

