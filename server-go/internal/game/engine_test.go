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

	// Use StartImmediate to skip countdown for testing
	e.StartImmediate(20)

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

func TestEngine_Start_WithCountdown(t *testing.T) {
	e := NewEngine()

	// Test that Start() enters COUNTDOWN phase
	e.Start(20)

	if e.GetPhase() != PhaseCountdown {
		t.Errorf("Expected phase COUNTDOWN, got %s", e.GetPhase())
	}

	state := e.GetState()
	if state.CountdownTime <= 0 {
		t.Errorf("Expected countdown time > 0, got %d", state.CountdownTime)
	}

	// Clean up
	e.Stop()
}

func TestEngine_Stop(t *testing.T) {
	e := NewEngine()

	e.StartImmediate(30)
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

	e.StartImmediate(30)
	e.Pause()

	if e.GetPhase() != PhasePaused {
		t.Errorf("Expected phase PAUSED, got %s", e.GetPhase())
	}

	e.Stop() // cleanup
}

func TestEngine_Continue(t *testing.T) {
	e := NewEngine()

	e.StartImmediate(30)
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

	// Must be in START phase - use StartImmediate to skip countdown
	e.StartImmediate(30)

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
	e.StartImmediate(30)

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
	e.StartImmediate(30)

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

func TestEngine_ProcessButtonPress_IgnoresMemoryQuestions(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	// Set a MEMORY question
	memoryQuestion := &Question{
		ID:       "m1",
		Question: "Memory game",
		Type:     "MEMORY",
		Time:     "120",
	}
	e.state.Question = memoryQuestion

	// Start the game
	e.StartImmediate(120)

	// Try to buzz - should be ignored for MEMORY questions
	pressTime := int64(1000000)
	e.ProcessButtonPress("b1", pressTime, "A")

	// Verify the buzz was ignored
	bumper := e.GetBumper("b1")
	if bumper.Time != 0 {
		t.Errorf("Buzz should be ignored for MEMORY questions, but time was recorded: %d", bumper.Time)
	}

	team := e.GetTeam("red")
	if team.Time != 0 {
		t.Errorf("Team time should not be set for MEMORY questions, got %d", team.Time)
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

	// ClearBumpers keeps teams intact (only clears bumper references from teams)
	if e.GetTeam("red") == nil {
		t.Error("Team should be kept after ClearBumpers")
	}
	// Team's bumper reference should be reset
	if e.GetTeam("red").Bumper != "" {
		t.Error("Team's bumper reference should be cleared")
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

	// With question: Reveal requires STOPPED or PAUSED phase.
	// Ready() sets phase to PREPARE, so we must stop first.
	e.Ready("q1", &Question{ID: "q1", Answer: "42"})
	e.Stop()
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

	// Start (use StartImmediate to skip countdown)
	e.StartImmediate(30)
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

	// Use StartImmediate to skip countdown
	e.StartImmediate(30)
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

// TestEngine_QCM_VPlayerInvalidation tests that physical buzzers are invalidated
// when a team has a VPlayer for QCM questions only
func TestEngine_QCM_VPlayerInvalidation(t *testing.T) {
	e := NewEngine()

	// Setup teams
	teams := map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	}
	e.SetTeams(teams)

	// Setup bumpers: one VPlayer and one physical in the same team
	bumpers := map[string]*Bumper{
		"vplayer1": {Name: "VPlayer1", Team: "red", IsVirtual: true, IsVPlayer: true},
		"buzzer1":  {Name: "Buzzer1", Team: "red", IsVirtual: false, IsVPlayer: false},
	}
	e.SetBumpers(bumpers)

	// Setup QCM question
	question := &Question{
		ID:       "q1",
		Question: "Test QCM?",
		Type:     QuestionTypeQCM,
		QCMAnswers: &QCMAnswers{
			Red:    "Answer A",
			Green:  "Answer B",
			Yellow: "Answer C",
			Blue:   "Answer D",
		},
		QCMCorrect: "RED",
	}

	// Start game with QCM question
	e.Ready("q1", question)
	e.StartImmediate(30)

	// Physical buzzer tries to buzz - should be IGNORED (team has VPlayer)
	e.ProcessButtonPress("buzzer1", time.Now().UnixMicro(), "A")

	// Verify physical buzzer was NOT recorded
	bumper := e.GetBumper("buzzer1")
	if bumper.Time != 0 {
		t.Errorf("Physical buzzer should be ignored when team has VPlayer, got Time=%d", bumper.Time)
	}

	// Verify team was NOT recorded either
	team := e.GetTeam("red")
	if team.Time != 0 {
		t.Errorf("Team should not have Time set when physical buzzer ignored, got Time=%d", team.Time)
	}

	// Now VPlayer buzzes - should be ACCEPTED
	e.ProcessButtonPress("vplayer1", time.Now().UnixMicro(), "RED")

	// Verify VPlayer was recorded
	vplayer := e.GetBumper("vplayer1")
	if vplayer.Time == 0 {
		t.Error("VPlayer should be accepted and have Time set")
	}

	// Verify team was recorded
	team = e.GetTeam("red")
	if team.Time == 0 {
		t.Error("Team should have Time set when VPlayer buzzed")
	}
	if team.Bumper != "vplayer1" {
		t.Errorf("Team should reference VPlayer, got Bumper=%s", team.Bumper)
	}
}

// TestEngine_QCM_VPlayerInvalidation_NormalQuestion tests that physical buzzers
// are NOT invalidated for NORMAL questions (only QCM)
func TestEngine_QCM_VPlayerInvalidation_NormalQuestion(t *testing.T) {
	e := NewEngine()

	// Setup teams
	teams := map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	}
	e.SetTeams(teams)

	// Setup bumpers: one VPlayer and one physical in the same team
	bumpers := map[string]*Bumper{
		"vplayer1": {Name: "VPlayer1", Team: "red", IsVirtual: true, IsVPlayer: true},
		"buzzer1":  {Name: "Buzzer1", Team: "red", IsVirtual: false, IsVPlayer: false},
	}
	e.SetBumpers(bumpers)

	// Setup NORMAL question (not QCM)
	question := &Question{
		ID:       "q1",
		Question: "Test Normal?",
		Type:     QuestionTypeNormal,
		Answer:   "42",
	}

	// Start game with NORMAL question
	e.Ready("q1", question)
	e.StartImmediate(30)

	// Physical buzzer tries to buzz - should be ACCEPTED (not QCM question)
	e.ProcessButtonPress("buzzer1", time.Now().UnixMicro(), "A")

	// Verify physical buzzer WAS recorded
	bumper := e.GetBumper("buzzer1")
	if bumper.Time == 0 {
		t.Error("Physical buzzer should be accepted for NORMAL question, got Time=0")
	}

	// Verify team WAS recorded
	team := e.GetTeam("red")
	if team.Time == 0 {
		t.Error("Team should have Time set for NORMAL question")
	}
	if team.Bumper != "buzzer1" {
		t.Errorf("Team should reference physical buzzer, got Bumper=%s", team.Bumper)
	}
}

// TestEngine_QCM_VPlayerInvalidation_NoTeam tests that physical buzzers
// without a team are NOT invalidated (no team = no VPlayer check)
func TestEngine_QCM_VPlayerInvalidation_NoTeam(t *testing.T) {
	e := NewEngine()

	// Setup physical buzzer WITHOUT team
	bumpers := map[string]*Bumper{
		"buzzer1": {Name: "Buzzer1", Team: "", IsVirtual: false, IsVPlayer: false},
	}
	e.SetBumpers(bumpers)

	// Setup QCM question
	question := &Question{
		ID:       "q1",
		Question: "Test QCM?",
		Type:     QuestionTypeQCM,
		QCMAnswers: &QCMAnswers{
			Red:    "Answer A",
			Green:  "Answer B",
			Yellow: "Answer C",
			Blue:   "Answer D",
		},
		QCMCorrect: "RED",
	}

	// Start game with QCM question
	e.Ready("q1", question)
	e.StartImmediate(30)

	// Physical buzzer WITHOUT team tries to buzz - should be ACCEPTED
	e.ProcessButtonPress("buzzer1", time.Now().UnixMicro(), "A")

	// Verify physical buzzer WAS recorded (no team = no invalidation)
	bumper := e.GetBumper("buzzer1")
	if bumper.Time == 0 {
		t.Error("Physical buzzer without team should be accepted, got Time=0")
	}
}

// TestEngine_QCM_VPlayerInvalidation_NoVPlayer tests that physical buzzers
// are NOT invalidated when the team has no VPlayer
func TestEngine_QCM_VPlayerInvalidation_NoVPlayer(t *testing.T) {
	e := NewEngine()

	// Setup teams
	teams := map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	}
	e.SetTeams(teams)

	// Setup bumpers: only physical buzzers (no VPlayer)
	bumpers := map[string]*Bumper{
		"buzzer1": {Name: "Buzzer1", Team: "red", IsVirtual: false, IsVPlayer: false},
		"buzzer2": {Name: "Buzzer2", Team: "red", IsVirtual: false, IsVPlayer: false},
	}
	e.SetBumpers(bumpers)

	// Setup QCM question
	question := &Question{
		ID:       "q1",
		Question: "Test QCM?",
		Type:     QuestionTypeQCM,
		QCMAnswers: &QCMAnswers{
			Red:    "Answer A",
			Green:  "Answer B",
			Yellow: "Answer C",
			Blue:   "Answer D",
		},
		QCMCorrect: "RED",
	}

	// Start game with QCM question
	e.Ready("q1", question)
	e.StartImmediate(30)

	// Physical buzzer tries to buzz - should be ACCEPTED (no VPlayer in team)
	e.ProcessButtonPress("buzzer1", time.Now().UnixMicro(), "A")

	// Verify physical buzzer WAS recorded
	bumper := e.GetBumper("buzzer1")
	if bumper.Time == 0 {
		t.Error("Physical buzzer should be accepted when team has no VPlayer, got Time=0")
	}

	// Verify team WAS recorded
	team := e.GetTeam("red")
	if team.Time == 0 {
		t.Error("Team should have Time set")
	}
	if team.Bumper != "buzzer1" {
		t.Errorf("Team should reference physical buzzer, got Bumper=%s", team.Bumper)
	}
}

// TestEngine_QCM_VPlayerInvalidation_MultipleVPlayers tests that physical buzzers
// are invalidated even when multiple VPlayers exist in the team
func TestEngine_QCM_VPlayerInvalidation_MultipleVPlayers(t *testing.T) {
	e := NewEngine()

	// Setup teams
	teams := map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	}
	e.SetTeams(teams)

	// Setup bumpers: multiple VPlayers and one physical in the same team
	bumpers := map[string]*Bumper{
		"vplayer1": {Name: "VPlayer1", Team: "red", IsVirtual: true, IsVPlayer: true},
		"vplayer2": {Name: "VPlayer2", Team: "red", IsVirtual: true, IsVPlayer: true},
		"buzzer1":  {Name: "Buzzer1", Team: "red", IsVirtual: false, IsVPlayer: false},
	}
	e.SetBumpers(bumpers)

	// Setup QCM question
	question := &Question{
		ID:       "q1",
		Question: "Test QCM?",
		Type:     QuestionTypeQCM,
		QCMAnswers: &QCMAnswers{
			Red:    "Answer A",
			Green:  "Answer B",
			Yellow: "Answer C",
			Blue:   "Answer D",
		},
		QCMCorrect: "RED",
	}

	// Start game with QCM question
	e.Ready("q1", question)
	e.StartImmediate(30)

	// Physical buzzer tries to buzz - should be IGNORED (team has multiple VPlayers)
	e.ProcessButtonPress("buzzer1", time.Now().UnixMicro(), "A")

	// Verify physical buzzer was NOT recorded
	bumper := e.GetBumper("buzzer1")
	if bumper.Time != 0 {
		t.Errorf("Physical buzzer should be ignored when team has multiple VPlayers, got Time=%d", bumper.Time)
	}

	// Verify team was NOT recorded
	team := e.GetTeam("red")
	if team.Time != 0 {
		t.Errorf("Team should not have Time set, got Time=%d", team.Time)
	}
}

// ============================================================================
// Tests for VPlayer QCM Multicolor Feature (v2.53.0)
// ============================================================================

// TestVPlayerBumperCreation tests VPlayer bumper creation with correct flags
func TestVPlayerBumperCreation(t *testing.T) {
	e := NewEngine()

	// Create a VPlayer bumper (virtual player with QCM multi-color capability)
	bumpers := map[string]*Bumper{
		"vplayer1": {
			Name:      "TestVPlayer",
			Team:      "red",
			IsVirtual: true,
			IsVPlayer: true,
		},
	}
	e.SetBumpers(bumpers)

	// Verify bumper was created correctly
	vplayer := e.GetBumper("vplayer1")
	if vplayer == nil {
		t.Fatal("VPlayer bumper should exist")
	}

	// Verify IS_VPLAYER flag
	if !vplayer.IsVPlayer {
		t.Error("VPlayer should have IsVPlayer=true")
	}

	// Verify IS_VIRTUAL flag (VPlayers are also virtual)
	if !vplayer.IsVirtual {
		t.Error("VPlayer should have IsVirtual=true")
	}

	// Verify virtual player count
	if e.GetVirtualPlayerCount() != 1 {
		t.Errorf("Expected virtual player count 1, got %d", e.GetVirtualPlayerCount())
	}
}

// TestVPlayerQCMBuzzAllColors tests that a VPlayer can buzz with all QCM colors
func TestVPlayerQCMBuzzAllColors(t *testing.T) {
	// Test all four QCM colors: RED, GREEN, YELLOW, BLUE
	colors := []string{"RED", "GREEN", "YELLOW", "BLUE"}

	for _, color := range colors {
		t.Run("Color_"+color, func(t *testing.T) {
			e := NewEngine()

			// Setup team
			teams := map[string]*Team{
				"red": {Name: "Team Red", Color: []int{255, 0, 0}},
			}
			e.SetTeams(teams)

			// Setup VPlayer bumper
			bumpers := map[string]*Bumper{
				"vplayer1": {
					Name:      "TestVPlayer",
					Team:      "red",
					IsVirtual: true,
					IsVPlayer: true,
				},
			}
			e.SetBumpers(bumpers)

			// Setup QCM question
			question := &Question{
				ID:       "q1",
				Question: "Test QCM?",
				Type:     QuestionTypeQCM,
				QCMAnswers: &QCMAnswers{
					Red:    "Answer A",
					Green:  "Answer B",
					Yellow: "Answer C",
					Blue:   "Answer D",
				},
				QCMCorrect: "GREEN", // Doesn't matter for this test
			}

			// Start game with QCM question
			e.Ready("q1", question)
			e.StartImmediate(30)

			// VPlayer buzzes with the color
			pressTime := time.Now().UnixMicro()
			e.ProcessButtonPress("vplayer1", pressTime, color)

			// Verify VPlayer buzz was ACCEPTED
			vplayer := e.GetBumper("vplayer1")
			if vplayer.Time == 0 {
				t.Errorf("VPlayer should be able to buzz with color %s, but Time=0", color)
			}
			if vplayer.Button != color {
				t.Errorf("VPlayer button should be %s, got %s", color, vplayer.Button)
			}

			// Verify team was updated
			team := e.GetTeam("red")
			if team.Time == 0 {
				t.Errorf("Team should have Time set when VPlayer buzzed with %s", color)
			}
			if team.Bumper != "vplayer1" {
				t.Errorf("Team should reference VPlayer, got Bumper=%s", team.Bumper)
			}
		})
	}
}

// TestPhysicalBuzzerInvalidatedWhenTeamHasVPlayer tests that physical buzzers
// are ignored for QCM questions when the team has a VPlayer
func TestPhysicalBuzzerInvalidatedWhenTeamHasVPlayer(t *testing.T) {
	e := NewEngine()

	// Setup team
	teams := map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	}
	e.SetTeams(teams)

	// Setup bumpers: one VPlayer and one physical in same team
	bumpers := map[string]*Bumper{
		"vplayer1": {
			Name:      "VPlayer1",
			Team:      "red",
			IsVirtual: true,
			IsVPlayer: true,
		},
		"buzzer1": {
			Name:      "Buzzer1",
			Team:      "red",
			IsVirtual: false,
			IsVPlayer: false,
		},
	}
	e.SetBumpers(bumpers)

	// Setup QCM question
	question := &Question{
		ID:       "q1",
		Question: "Test QCM?",
		Type:     QuestionTypeQCM,
		QCMAnswers: &QCMAnswers{
			Red:    "Answer A",
			Green:  "Answer B",
			Yellow: "Answer C",
			Blue:   "Answer D",
		},
		QCMCorrect: "RED",
	}

	// Start game with QCM question
	e.Ready("q1", question)
	e.StartImmediate(30)

	// Physical buzzer tries to buzz first - should be IGNORED
	physicalPressTime := time.Now().UnixMicro()
	e.ProcessButtonPress("buzzer1", physicalPressTime, "A")

	// Verify physical buzzer was NOT recorded
	buzzer := e.GetBumper("buzzer1")
	if buzzer.Time != 0 {
		t.Errorf("Physical buzzer should be ignored when team has VPlayer, got Time=%d", buzzer.Time)
	}

	// Verify team was NOT recorded yet
	team := e.GetTeam("red")
	if team.Time != 0 {
		t.Errorf("Team should not have Time set when physical buzzer ignored, got Time=%d", team.Time)
	}

	// Now VPlayer buzzes - should be ACCEPTED
	vplayerPressTime := time.Now().UnixMicro()
	e.ProcessButtonPress("vplayer1", vplayerPressTime, "RED")

	// Verify VPlayer was recorded
	vplayer := e.GetBumper("vplayer1")
	if vplayer.Time == 0 {
		t.Error("VPlayer should be accepted and have Time set")
	}
	if vplayer.Button != "RED" {
		t.Errorf("VPlayer button should be RED, got %s", vplayer.Button)
	}

	// Verify team was recorded with VPlayer
	team = e.GetTeam("red")
	if team.Time == 0 {
		t.Error("Team should have Time set when VPlayer buzzed")
	}
	if team.Bumper != "vplayer1" {
		t.Errorf("Team should reference VPlayer, got Bumper=%s", team.Bumper)
	}
}

// TestPhysicalBuzzerNotInvalidatedForNonQCM tests that physical buzzers
// work normally for non-QCM questions even when team has a VPlayer
func TestPhysicalBuzzerNotInvalidatedForNonQCM(t *testing.T) {
	e := NewEngine()

	// Setup team
	teams := map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	}
	e.SetTeams(teams)

	// Setup bumpers: one VPlayer and one physical in same team
	bumpers := map[string]*Bumper{
		"vplayer1": {
			Name:      "VPlayer1",
			Team:      "red",
			IsVirtual: true,
			IsVPlayer: true,
		},
		"buzzer1": {
			Name:      "Buzzer1",
			Team:      "red",
			IsVirtual: false,
			IsVPlayer: false,
		},
	}
	e.SetBumpers(bumpers)

	// Setup NORMAL question (not QCM)
	question := &Question{
		ID:       "q1",
		Question: "Test Normal?",
		Type:     QuestionTypeNormal,
		Answer:   "42",
	}

	// Start game with NORMAL question
	e.Ready("q1", question)
	e.StartImmediate(30)

	// Physical buzzer tries to buzz - should be ACCEPTED (not QCM)
	physicalPressTime := time.Now().UnixMicro()
	e.ProcessButtonPress("buzzer1", physicalPressTime, "A")

	// Verify physical buzzer WAS recorded
	buzzer := e.GetBumper("buzzer1")
	if buzzer.Time == 0 {
		t.Error("Physical buzzer should be accepted for NORMAL question, got Time=0")
	}
	if buzzer.Button != "A" {
		t.Errorf("Physical buzzer button should be A, got %s", buzzer.Button)
	}

	// Verify team WAS recorded with physical buzzer
	team := e.GetTeam("red")
	if team.Time == 0 {
		t.Error("Team should have Time set for NORMAL question")
	}
	if team.Bumper != "buzzer1" {
		t.Errorf("Team should reference physical buzzer, got Bumper=%s", team.Bumper)
	}
}

// TestPhysicalBuzzerWorksWithoutVPlayer tests that physical buzzers
// work normally for QCM questions when team has NO VPlayer
func TestPhysicalBuzzerWorksWithoutVPlayer(t *testing.T) {
	e := NewEngine()

	// Setup team
	teams := map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	}
	e.SetTeams(teams)

	// Setup bumpers: only physical buzzers (no VPlayer)
	bumpers := map[string]*Bumper{
		"buzzer1": {
			Name:      "Buzzer1",
			Team:      "red",
			IsVirtual: false,
			IsVPlayer: false,
		},
		"buzzer2": {
			Name:      "Buzzer2",
			Team:      "red",
			IsVirtual: false,
			IsVPlayer: false,
		},
	}
	e.SetBumpers(bumpers)

	// Setup QCM question
	question := &Question{
		ID:       "q1",
		Question: "Test QCM?",
		Type:     QuestionTypeQCM,
		QCMAnswers: &QCMAnswers{
			Red:    "Answer A",
			Green:  "Answer B",
			Yellow: "Answer C",
			Blue:   "Answer D",
		},
		QCMCorrect: "RED",
	}

	// Start game with QCM question
	e.Ready("q1", question)
	e.StartImmediate(30)

	// Physical buzzer tries to buzz - should be ACCEPTED (no VPlayer in team)
	physicalPressTime := time.Now().UnixMicro()
	e.ProcessButtonPress("buzzer1", physicalPressTime, "A")

	// Verify physical buzzer WAS recorded
	buzzer := e.GetBumper("buzzer1")
	if buzzer.Time == 0 {
		t.Error("Physical buzzer should be accepted when team has no VPlayer, got Time=0")
	}
	if buzzer.Button != "A" {
		t.Errorf("Physical buzzer button should be A, got %s", buzzer.Button)
	}

	// Verify team WAS recorded
	team := e.GetTeam("red")
	if team.Time == 0 {
		t.Error("Team should have Time set when physical buzzer pressed")
	}
	if team.Bumper != "buzzer1" {
		t.Errorf("Team should reference physical buzzer, got Bumper=%s", team.Bumper)
	}

	// Second physical buzzer tries to buzz - should be IGNORED (team already buzzed)
	secondPressTime := time.Now().UnixMicro()
	e.ProcessButtonPress("buzzer2", secondPressTime, "B")

	// Verify second buzzer was NOT recorded
	buzzer2 := e.GetBumper("buzzer2")
	if buzzer2.Time != 0 {
		t.Errorf("Second buzzer should be ignored (team already buzzed), got Time=%d", buzzer2.Time)
	}
}

// ============================================================================
// Tests for QCM hints reset at PREPARE (bugfix v3.2.1)
// ============================================================================

func makeQCMQuestion(id string) *Question {
	return &Question{
		ID:       id,
		Question: "Test QCM?",
		Type:     QuestionTypeQCM,
		QCMAnswers: &QCMAnswers{
			Red:    "Answer A",
			Green:  "Answer B",
			Yellow: "Answer C",
			Blue:   "Answer D",
		},
		QCMCorrect:        "RED",
		QCMHintsEnabled:   true,
		QCMHintThreshold1: 0.25,
		QCMHintThreshold2: 0.125,
		Points:            "10",
		Time:              "30",
	}
}

// TestQCMHints_ResetOnNewQuestion verifies hints reset when a NEW question is PREPAREd
func TestQCMHints_ResetOnNewQuestion(t *testing.T) {
	e := NewEngine()

	// Inject invalidated hints manually (simulates hints revealed during previous round)
	e.mu.Lock()
	e.state.QcmInvalidated = []string{"YELLOW", "BLUE"}
	e.mu.Unlock()

	// PREPARE a new question
	e.Ready("q2", makeQCMQuestion("q2"))

	state := e.GetState()
	if len(state.QcmInvalidated) != 0 {
		t.Errorf("QcmInvalidated should be empty after PREPARE of new question, got %v", state.QcmInvalidated)
	}
}

// TestQCMHints_ResetOnSameQuestionRePrepared verifies hints reset when the SAME question is re-PREPAREd
// This is the core bugfix: previously hints were NOT reset in this case.
func TestQCMHints_ResetOnSameQuestionRePrepared(t *testing.T) {
	e := NewEngine()
	q := makeQCMQuestion("q1")

	// First PREPARE cycle
	e.Ready("q1", q)
	e.StartImmediate(30)

	// Inject invalidated hints (simulates timer triggering hints during gameplay)
	e.mu.Lock()
	e.state.QcmInvalidated = []string{"YELLOW", "BLUE"}
	e.mu.Unlock()

	// STOP then PREPARE the SAME question again
	e.Stop()
	e.Ready("q1", q)

	state := e.GetState()
	if len(state.QcmInvalidated) != 0 {
		t.Errorf("QcmInvalidated should be empty after re-PREPARE of same question, got %v", state.QcmInvalidated)
	}
}

// TestQCMHints_ResetAfterReveal verifies hints reset when PREPARE is called after REVEAL on same question
func TestQCMHints_ResetAfterReveal(t *testing.T) {
	e := NewEngine()

	// Setup a team and bumper so ProcessButtonPress can advance state
	e.SetTeams(map[string]*Team{
		"red": {Name: "Red", Color: []int{255, 0, 0}},
	})
	e.SetBumpers(map[string]*Bumper{
		"b1": {Name: "Buzzer1", Team: "red"},
	})

	q := makeQCMQuestion("q1")

	// First cycle: PREPARE → START → STOP → REVEAL
	e.Ready("q1", q)
	e.StartImmediate(30)

	// Inject hints mid-game
	e.mu.Lock()
	e.state.QcmInvalidated = []string{"BLUE"}
	e.mu.Unlock()

	// Stop then Reveal (Reveal requires STOPPED or PAUSED phase)
	e.Stop()
	e.Reveal()

	// Verify hints still present after REVEAL (Reveal does not clear them)
	stateBefore := e.GetState()
	if len(stateBefore.QcmInvalidated) == 0 {
		t.Log("Warning: QcmInvalidated was already empty before re-PREPARE (hint cleared by Stop/Reveal)")
	}

	// Re-PREPARE the SAME question (this is the bug scenario)
	e.Ready("q1", q)

	state := e.GetState()
	if len(state.QcmInvalidated) != 0 {
		t.Errorf("QcmInvalidated should be empty after re-PREPARE (same question after REVEAL), got %v", state.QcmInvalidated)
	}
}

// TestQCMHints_AlwaysResetRegardlessOfIsNewQuestion verifies the fix across multiple cycles
func TestQCMHints_AlwaysResetRegardlessOfIsNewQuestion(t *testing.T) {
	e := NewEngine()
	q := makeQCMQuestion("q1")

	tests := []struct {
		name            string
		initialHints    []string
		questionID      string
		expectEmptyHints bool
	}{
		{
			name:            "new question clears hints",
			initialHints:    []string{"RED", "GREEN"},
			questionID:      "q1",
			expectEmptyHints: true,
		},
		{
			name:            "same question re-prepared clears hints",
			initialHints:    []string{"YELLOW"},
			questionID:      "q1",
			expectEmptyHints: true,
		},
		{
			name:            "no hints — stays empty",
			initialHints:    []string{},
			questionID:      "q1",
			expectEmptyHints: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Inject initial hint state
			e.mu.Lock()
			e.state.QcmInvalidated = tt.initialHints
			e.mu.Unlock()

			e.Ready(tt.questionID, q)

			state := e.GetState()
			isEmpty := len(state.QcmInvalidated) == 0
			if isEmpty != tt.expectEmptyHints {
				t.Errorf("QcmInvalidated empty=%v, want %v (got %v)", isEmpty, tt.expectEmptyHints, state.QcmInvalidated)
			}
		})
	}
}

// TestEngine_QCM_HintsAtBuzz_NoHints tests that HINTS_AT_BUZZ is 0 when buzzer
// presses before any QCM hint has been revealed.
func TestEngine_QCM_HintsAtBuzz_NoHints(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"teamA": {Name: "Team A", Color: []int{255, 0, 0}},
	})
	e.SetBumpers(map[string]*Bumper{
		"buzzer1": {Name: "Player1", Team: "teamA"},
	})

	question := &Question{
		ID:       "q1",
		Question: "QCM test?",
		Type:     QuestionTypeQCM,
		QCMAnswers: &QCMAnswers{
			Red: "A", Green: "B", Yellow: "C", Blue: "D",
		},
		QCMCorrect:      "RED",
		QCMHintsEnabled: true,
	}
	e.Ready("q1", question)
	e.StartImmediate(30)

	// No hints given yet — buzz immediately
	e.ProcessButtonPress("buzzer1", time.Now().UnixMicro(), "A")

	bumper := e.GetBumper("buzzer1")
	if bumper.HintsAtBuzz != 0 {
		t.Errorf("HintsAtBuzz should be 0 before any hint, got %d", bumper.HintsAtBuzz)
	}
}

// TestEngine_QCM_HintsAtBuzz_WithHints tests that HINTS_AT_BUZZ captures the
// number of hints already revealed at the moment the buzzer presses.
func TestEngine_QCM_HintsAtBuzz_WithHints(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"teamA": {Name: "Team A", Color: []int{255, 0, 0}},
	})
	e.SetBumpers(map[string]*Bumper{
		"buzzer1": {Name: "Player1", Team: "teamA"},
	})

	question := &Question{
		ID:       "q1",
		Question: "QCM test hints?",
		Type:     QuestionTypeQCM,
		QCMAnswers: &QCMAnswers{
			Red: "A", Green: "B", Yellow: "C", Blue: "D",
		},
		QCMCorrect:        "RED",
		QCMHintsEnabled:   true,
		QCMHintThreshold1: 0.25,
		QCMHintThreshold2: 0.125,
	}
	e.Ready("q1", question)
	e.StartImmediate(30)

	// Simulate 1 hint already given by injecting QcmInvalidated directly
	e.mu.Lock()
	e.state.QcmInvalidated = []string{"GREEN"}
	e.mu.Unlock()

	// Buzzer presses after 1 hint
	e.ProcessButtonPress("buzzer1", time.Now().UnixMicro(), "A")

	bumper := e.GetBumper("buzzer1")
	if bumper.HintsAtBuzz != 1 {
		t.Errorf("HintsAtBuzz should be 1 after 1 hint, got %d", bumper.HintsAtBuzz)
	}
}

// TestEngine_QCM_HintsAtBuzz_TwoHints tests that HINTS_AT_BUZZ captures 2 hints
// when two hints have been revealed before the buzz.
func TestEngine_QCM_HintsAtBuzz_TwoHints(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"teamA": {Name: "Team A", Color: []int{255, 0, 0}},
	})
	e.SetBumpers(map[string]*Bumper{
		"buzzer1": {Name: "Player1", Team: "teamA"},
	})

	question := &Question{
		ID:       "q1",
		Question: "QCM 2 hints?",
		Type:     QuestionTypeQCM,
		QCMAnswers: &QCMAnswers{
			Red: "A", Green: "B", Yellow: "C", Blue: "D",
		},
		QCMCorrect:      "RED",
		QCMHintsEnabled: true,
	}
	e.Ready("q1", question)
	e.StartImmediate(30)

	// Simulate 2 hints already given
	e.mu.Lock()
	e.state.QcmInvalidated = []string{"GREEN", "YELLOW"}
	e.mu.Unlock()

	e.ProcessButtonPress("buzzer1", time.Now().UnixMicro(), "A")

	bumper := e.GetBumper("buzzer1")
	if bumper.HintsAtBuzz != 2 {
		t.Errorf("HintsAtBuzz should be 2 after 2 hints, got %d", bumper.HintsAtBuzz)
	}
}

// TestEngine_QCM_HintsAtBuzz_ResetOnPrepare tests that HINTS_AT_BUZZ is cleared
// when the engine transitions to PREPARE for a new question.
func TestEngine_QCM_HintsAtBuzz_ResetOnPrepare(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"teamA": {Name: "Team A", Color: []int{255, 0, 0}},
	})
	e.SetBumpers(map[string]*Bumper{
		"buzzer1": {Name: "Player1", Team: "teamA"},
	})

	question := &Question{
		ID:       "q1",
		Question: "QCM reset?",
		Type:     QuestionTypeQCM,
		QCMAnswers: &QCMAnswers{
			Red: "A", Green: "B", Yellow: "C", Blue: "D",
		},
		QCMCorrect:      "RED",
		QCMHintsEnabled: true,
	}
	e.Ready("q1", question)
	e.StartImmediate(30)

	// Simulate 2 hints and buzz
	e.mu.Lock()
	e.state.QcmInvalidated = []string{"GREEN", "YELLOW"}
	e.mu.Unlock()
	e.ProcessButtonPress("buzzer1", time.Now().UnixMicro(), "A")

	// Verify hints stored
	bumper := e.GetBumper("buzzer1")
	if bumper.HintsAtBuzz != 2 {
		t.Fatalf("Pre-condition: HintsAtBuzz should be 2, got %d", bumper.HintsAtBuzz)
	}

	// Stop the game first, then move to PREPARE — HintsAtBuzz should be reset
	e.Stop()
	e.Ready("q1", question)

	bumperAfter := e.GetBumper("buzzer1")
	if bumperAfter.HintsAtBuzz != 0 {
		t.Errorf("HintsAtBuzz should be 0 after PREPARE, got %d", bumperAfter.HintsAtBuzz)
	}
}

