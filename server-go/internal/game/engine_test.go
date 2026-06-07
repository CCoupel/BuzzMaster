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

// TestEngine_UpdateBumper_Connected verifies that UpdateBumper correctly
// sets and clears the Connected field. This test would have caught the missing
// case in UpdateBumper() where CONNECTED was not mapped to bumper.Connected.
func TestEngine_UpdateBumper_Connected(t *testing.T) {
	e := NewEngine()
	e.UpdateBumper("b1", map[string]interface{}{"NAME": "BuzzerTest", "TEAM": "red"})

	// Default: Connected should be false (zero value)
	bumper := e.GetBumper("b1")
	if bumper == nil {
		t.Fatal("Bumper b1 should exist")
	}
	if bumper.Connected {
		t.Error("Connected should be false by default")
	}

	// Set CONNECTED=true
	e.UpdateBumper("b1", map[string]interface{}{"CONNECTED": true})
	bumper = e.GetBumper("b1")
	if !bumper.Connected {
		t.Error("Connected should be true after UpdateBumper with CONNECTED=true")
	}

	// Set CONNECTED=false — must be honoured (false is a meaningful value)
	e.UpdateBumper("b1", map[string]interface{}{"CONNECTED": false})
	bumper = e.GetBumper("b1")
	if bumper.Connected {
		t.Error("Connected should be false after UpdateBumper with CONNECTED=false")
	}

	// Other fields must be preserved across CONNECTED updates
	if bumper.Name != "BuzzerTest" {
		t.Errorf("Name should be preserved, got %s", bumper.Name)
	}
	if bumper.Team != "red" {
		t.Errorf("Team should be preserved, got %s", bumper.Team)
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

func TestEngine_AreAllTeamsReady_EmptyTeamsIgnored(t *testing.T) {
	e := NewEngine()

	// One active team (with buzzer) + one empty team (no buzzer)
	e.SetTeams(map[string]*Team{
		"red":   {Name: "Team Red"},
		"empty": {Name: "Team Empty"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	// "empty" team has no bumper assigned

	e.Ready("q1", nil)
	e.SetBumperReady("b1")

	// Empty team must not block: only active team (red) is ready → should be ready
	if !e.AreAllTeamsReady() {
		t.Error("Empty team should be ignored: active team is ready so AreAllTeamsReady should return true")
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
// are NOT invalidated for SPEEDY questions (only QCM)
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

	// Setup SPEEDY question (not QCM)
	question := &Question{
		ID:       "q1",
		Question: "Test Normal?",
		Type:     QuestionTypeSpeedy,
		Answer:   "42",
	}

	// Start game with SPEEDY question
	e.Ready("q1", question)
	e.StartImmediate(30)

	// Physical buzzer tries to buzz - should be ACCEPTED (not QCM question)
	e.ProcessButtonPress("buzzer1", time.Now().UnixMicro(), "A")

	// Verify physical buzzer WAS recorded
	bumper := e.GetBumper("buzzer1")
	if bumper.Time == 0 {
		t.Error("Physical buzzer should be accepted for SPEEDY question, got Time=0")
	}

	// Verify team WAS recorded
	team := e.GetTeam("red")
	if team.Time == 0 {
		t.Error("Team should have Time set for SPEEDY question")
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

	// Setup SPEEDY question (not QCM)
	question := &Question{
		ID:       "q1",
		Question: "Test Normal?",
		Type:     QuestionTypeSpeedy,
		Answer:   "42",
	}

	// Start game with SPEEDY question
	e.Ready("q1", question)
	e.StartImmediate(30)

	// Physical buzzer tries to buzz - should be ACCEPTED (not QCM)
	physicalPressTime := time.Now().UnixMicro()
	e.ProcessButtonPress("buzzer1", physicalPressTime, "A")

	// Verify physical buzzer WAS recorded
	buzzer := e.GetBumper("buzzer1")
	if buzzer.Time == 0 {
		t.Error("Physical buzzer should be accepted for SPEEDY question, got Time=0")
	}
	if buzzer.Button != "A" {
		t.Errorf("Physical buzzer button should be A, got %s", buzzer.Button)
	}

	// Verify team WAS recorded with physical buzzer
	team := e.GetTeam("red")
	if team.Time == 0 {
		t.Error("Team should have Time set for SPEEDY question")
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

// TestEngine_StartupConnectedReset verifies that bumpers loaded from disk with
// CONNECTED=true are correctly reset to false by iterating and calling
// UpdateBumper(id, {"CONNECTED": false}) — exactly as main.go does on startup.
// This is the non-regression test for bug #48.
func TestEngine_StartupConnectedReset(t *testing.T) {
	e := NewEngine()

	// Simulate bumpers persisted with CONNECTED=true (state from a previous session)
	persistedBumpers := map[string]*Bumper{
		"AA:BB:CC:DD:EE:01": {Name: "Buzzer1", Team: "red", Connected: true},
		"AA:BB:CC:DD:EE:02": {Name: "Buzzer2", Team: "blue", Connected: true},
		"AA:BB:CC:DD:EE:03": {Name: "Buzzer3", Team: "red", Connected: false}, // already false
	}
	e.SetBumpers(persistedBumpers)

	// Pre-condition: two bumpers have Connected=true
	if b := e.GetBumper("AA:BB:CC:DD:EE:01"); !b.Connected {
		t.Fatal("Pre-condition: Buzzer1 should have Connected=true before reset")
	}
	if b := e.GetBumper("AA:BB:CC:DD:EE:02"); !b.Connected {
		t.Fatal("Pre-condition: Buzzer2 should have Connected=true before reset")
	}

	// Simulate the startup reset loop from main.go:
	//   for id := range engine.GetTeamsAndBumpers().Bumpers {
	//       engine.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	//   }
	for id := range e.GetTeamsAndBumpers().Bumpers {
		e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	}

	// All bumpers must now have Connected=false
	tests := []struct {
		id   string
		name string
	}{
		{"AA:BB:CC:DD:EE:01", "Buzzer1"},
		{"AA:BB:CC:DD:EE:02", "Buzzer2"},
		{"AA:BB:CC:DD:EE:03", "Buzzer3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := e.GetBumper(tt.id)
			if b == nil {
				t.Fatalf("Bumper %s should exist", tt.id)
			}
			if b.Connected {
				t.Errorf("Bumper %s: Connected should be false after startup reset, got true", tt.id)
			}
		})
	}

	// Other fields must be preserved across the reset
	if b := e.GetBumper("AA:BB:CC:DD:EE:01"); b.Name != "Buzzer1" {
		t.Errorf("Buzzer1 Name should be preserved, got %q", b.Name)
	}
	if b := e.GetBumper("AA:BB:CC:DD:EE:01"); b.Team != "red" {
		t.Errorf("Buzzer1 Team should be preserved, got %q", b.Team)
	}
}

// ============================================================================
// Tests for InitGame and SetQuizMeta (v4.0.0 — milestone game-init)
// ============================================================================

// TestInitGame_ResetsScores verifies that InitGame clears all bumper and team scores,
// including TeamPoints accumulated via TEAM_POINTS actions.
func TestInitGame_ResetsScores(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red"},
		"blue": {Name: "Team Blue"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})
	e.UpdateScore("b1", 100)
	e.UpdateScore("b2", 50)
	// Also set TeamPoints directly to test that field is also cleared
	e.mu.Lock()
	e.data.Teams["red"].TeamPoints = 30
	e.data.Teams["blue"].TeamPoints = 10
	e.mu.Unlock()

	// Pre-condition: scores must be non-zero before reset
	if e.GetBumper("b1").Score == 0 {
		t.Fatal("Pre-condition: bumper b1 score should be > 0 before InitGame")
	}

	e.InitGame()

	b1 := e.GetBumper("b1")
	if b1.Score != 0 {
		t.Errorf("Bumper b1 Score should be 0 after InitGame, got %d", b1.Score)
	}

	b2 := e.GetBumper("b2")
	if b2.Score != 0 {
		t.Errorf("Bumper b2 Score should be 0 after InitGame, got %d", b2.Score)
	}

	red := e.GetTeam("red")
	if red.Score != 0 {
		t.Errorf("Red team Score should be 0 after InitGame, got %d", red.Score)
	}
	if red.TeamPoints != 0 {
		t.Errorf("Red team TeamPoints should be 0 after InitGame, got %d", red.TeamPoints)
	}

	blue := e.GetTeam("blue")
	if blue.Score != 0 {
		t.Errorf("Blue team Score should be 0 after InitGame, got %d", blue.Score)
	}
	if blue.TeamPoints != 0 {
		t.Errorf("Blue team TeamPoints should be 0 after InitGame, got %d", blue.TeamPoints)
	}
}

// TestInitGame_ClearsHistory verifies that InitGame empties the game event history.
func TestInitGame_ClearsHistory(t *testing.T) {
	e := NewEngine()

	e.AddGameEvent(GameEvent{EventType: "POINTS_AWARDED", WinnerName: "Player1", Points: 10})
	e.AddGameEvent(GameEvent{EventType: "POINTS_AWARDED", WinnerName: "Team Red", Points: 20})

	// Pre-condition: 2 events in history
	if len(e.GetHistory()) != 2 {
		t.Fatalf("Pre-condition: expected 2 events in history, got %d", len(e.GetHistory()))
	}

	e.InitGame()

	history := e.GetHistory()
	if len(history) != 0 {
		t.Errorf("History should be empty after InitGame, got %d events", len(history))
	}
}

// TestInitGame_ResetsQuestionStatuses verifies that InitGame clears all question statuses,
// making previously seen questions AVAILABLE again.
func TestInitGame_ResetsQuestionStatuses(t *testing.T) {
	e := NewEngine()

	q := &Question{ID: "q1", Question: "test?", Points: "10", Time: "30"}

	// Ready() sets status to StatusPrepare
	e.Ready("q1", q)

	// Pre-condition: status should be PREPARE (not AVAILABLE)
	statusBefore := e.GetQuestionStatus("q1")
	if statusBefore == StatusAvailable {
		t.Fatal("Pre-condition: question status should not be AVAILABLE after Ready()")
	}

	e.InitGame()

	statusAfter := e.GetQuestionStatus("q1")
	if statusAfter != StatusAvailable {
		t.Errorf("Question status should be AVAILABLE after InitGame, got %s", statusAfter)
	}
}

// TestInitGame_SetsPhaseNewGame verifies that InitGame transitions the engine to PhaseNewGame.
func TestInitGame_SetsPhaseNewGame(t *testing.T) {
	e := NewEngine()

	// Engine starts in STOPPED — pre-condition
	if e.GetPhase() != PhaseStopped {
		t.Fatalf("Pre-condition: expected initial phase STOPPED, got %s", e.GetPhase())
	}

	e.InitGame()

	if e.GetPhase() != PhaseNewGame {
		t.Errorf("Phase should be NEW_GAME after InitGame, got %s", e.GetPhase())
	}
}

// TestSetQuizMeta verifies that SetQuizMeta stores name, theme and notes in GameState.
func TestSetQuizMeta(t *testing.T) {
	e := NewEngine()

	e.SetQuizMeta("Mon Quiz", "Sciences", "Un quiz éducatif")

	state := e.GetState()
	if state.QuizName != "Mon Quiz" {
		t.Errorf("QuizName should be 'Mon Quiz', got %q", state.QuizName)
	}
	if state.QuizTheme != "Sciences" {
		t.Errorf("QuizTheme should be 'Sciences', got %q", state.QuizTheme)
	}
	if state.QuizNotes != "Un quiz éducatif" {
		t.Errorf("QuizNotes should be 'Un quiz éducatif', got %q", state.QuizNotes)
	}
}

// TestSetQuizMeta_OverwritesPreviousValues verifies that calling SetQuizMeta twice
// fully replaces the previous values.
func TestSetQuizMeta_OverwritesPreviousValues(t *testing.T) {
	e := NewEngine()

	e.SetQuizMeta("Quiz v1", "Histoire", "")
	e.SetQuizMeta("Quiz v2", "Géographie", "Nouveau quiz")

	state := e.GetState()
	if state.QuizName != "Quiz v2" {
		t.Errorf("QuizName should be 'Quiz v2', got %q", state.QuizName)
	}
	if state.QuizTheme != "Géographie" {
		t.Errorf("QuizTheme should be 'Géographie', got %q", state.QuizTheme)
	}
	if state.QuizNotes != "Nouveau quiz" {
		t.Errorf("QuizNotes should be 'Nouveau quiz', got %q", state.QuizNotes)
	}
}

// TestReady_AllowedFromPhaseNewGame verifies that Ready() accepts PhaseNewGame as a
// starting phase and correctly transitions the engine to PhasePrepare.
func TestReady_AllowedFromPhaseNewGame(t *testing.T) {
	e := NewEngine()

	// Force engine into PhaseNewGame (state set by InitGame)
	e.SetPhase(PhaseNewGame)

	if e.GetPhase() != PhaseNewGame {
		t.Fatalf("Pre-condition: expected phase NEW_GAME, got %s", e.GetPhase())
	}

	q := &Question{ID: "q1", Question: "test?", Points: "10", Time: "30"}
	e.Ready("q1", q)

	if e.GetPhase() != PhasePrepare {
		t.Errorf("Phase should be PREPARE after Ready() from NEW_GAME, got %s", e.GetPhase())
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

// ─── ARDOISE tests (v5.6.0) ──────────────────────────────────────────────────

// TestSetArdoiseAnswer_InStartedPhase verifies that answers are stored
// correctly when the engine is in STARTED phase with TYPE=ARDOISE question.
func TestSetArdoiseAnswer_InStartedPhase(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"teamA": {Name: "teamA", Color: []int{255, 0, 0}},
	})
	e.SetBumpers(map[string]*Bumper{
		"b1": {Name: "Player1", Team: "teamA"},
	})

	q := &Question{
		ID:       "aq1",
		Question: "What is the capital?",
		Answer:   "Paris",
		Type:     QuestionTypeArdoise,
		Points:   "10",
		Time:     "30",
	}
	e.Ready("aq1", q)
	e.StartImmediate(30)

	ok := e.SetArdoiseAnswer("teamA", "Paris")
	if !ok {
		t.Fatal("SetArdoiseAnswer should return true in STARTED phase with ARDOISE question")
	}

	state := e.GetState()
	answer, exists := state.ArdoiseAnswers["teamA"]
	if !exists {
		t.Fatal("ArdoiseAnswers should contain teamA after SetArdoiseAnswer")
	}
	if answer.Text != "Paris" {
		t.Errorf("Expected answer text 'Paris', got %q", answer.Text)
	}
	if answer.SubmittedAt <= 0 {
		t.Errorf("SubmittedAt should be > 0, got %d", answer.SubmittedAt)
	}
}

// TestSetArdoiseAnswer_OutsideStartedPhase verifies that answers are ignored
// when the engine is not in STARTED phase (STOPPED, PREPARE, READY, etc.).
func TestSetArdoiseAnswer_OutsideStartedPhase(t *testing.T) {
	phases := []GamePhase{PhaseStopped, PhasePrepare, PhaseReady, PhasePaused, PhaseRevealed}

	q := &Question{
		ID:     "aq1",
		Type:   QuestionTypeArdoise,
		Points: "10",
		Time:   "30",
	}

	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			e := NewEngine()
			e.SetPhase(phase)
			// Manually set a question so type guard would pass if phase were OK
			e.mu.Lock()
			e.state.Question = q
			e.mu.Unlock()

			ok := e.SetArdoiseAnswer("teamA", "some text")
			if ok {
				t.Errorf("SetArdoiseAnswer should return false in phase %s", phase)
			}

			state := e.GetState()
			if len(state.ArdoiseAnswers) != 0 {
				t.Errorf("ArdoiseAnswers should be empty in phase %s, got %v", phase, state.ArdoiseAnswers)
			}
		})
	}
}

// TestSetArdoiseAnswer_NonArdoiseQuestion verifies that answers are ignored
// when the current question type is not ARDOISE.
func TestSetArdoiseAnswer_NonArdoiseQuestion(t *testing.T) {
	nonArdoiseTypes := []QuestionType{QuestionTypeSpeedy, QuestionTypeQCM, QuestionTypeMemory, QuestionTypeMemotion}

	for _, qType := range nonArdoiseTypes {
		t.Run(string(qType), func(t *testing.T) {
			e := NewEngine()
			e.SetTeams(map[string]*Team{
				"teamA": {Name: "teamA", Color: []int{255, 0, 0}},
			})
			e.SetBumpers(map[string]*Bumper{
				"b1": {Name: "Player1", Team: "teamA"},
			})

			q := &Question{
				ID:     "q1",
				Type:   qType,
				Points: "10",
				Time:   "30",
			}
			e.Ready("q1", q)
			e.StartImmediate(30)

			ok := e.SetArdoiseAnswer("teamA", "my answer")
			if ok {
				t.Errorf("SetArdoiseAnswer should return false for question type %s", qType)
			}

			state := e.GetState()
			if len(state.ArdoiseAnswers) != 0 {
				t.Errorf("ArdoiseAnswers should be empty for type %s, got %v", qType, state.ArdoiseAnswers)
			}
		})
	}
}

// TestArdoiseAnswers_ResetOnReady verifies that ArdoiseAnswers is cleared
// when a new question is prepared via Ready().
func TestArdoiseAnswers_ResetOnReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"teamA": {Name: "teamA", Color: []int{255, 0, 0}},
	})
	e.SetBumpers(map[string]*Bumper{
		"b1": {Name: "Player1", Team: "teamA"},
	})

	q1 := &Question{ID: "aq1", Type: QuestionTypeArdoise, Points: "10", Time: "30"}
	e.Ready("aq1", q1)
	e.StartImmediate(30)

	// Populate some answers
	e.SetArdoiseAnswer("teamA", "First answer")
	state := e.GetState()
	if len(state.ArdoiseAnswers) == 0 {
		t.Fatal("Pre-condition: ArdoiseAnswers should contain teamA answer")
	}

	// Move to a new question — ArdoiseAnswers must be reset
	e.Stop()
	q2 := &Question{ID: "aq2", Type: QuestionTypeArdoise, Points: "10", Time: "30"}
	e.Ready("aq2", q2)

	stateAfter := e.GetState()
	if len(stateAfter.ArdoiseAnswers) != 0 {
		t.Errorf("ArdoiseAnswers should be empty after Ready() with new question, got %v", stateAfter.ArdoiseAnswers)
	}
}

// TestArdoiseAnswers_ResetOnInitGame verifies that ArdoiseAnswers is cleared
// when InitGame() (NEW_GAME action) resets the game.
func TestArdoiseAnswers_ResetOnInitGame(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"teamA": {Name: "teamA", Color: []int{255, 0, 0}},
	})
	e.SetBumpers(map[string]*Bumper{
		"b1": {Name: "Player1", Team: "teamA"},
	})

	q := &Question{ID: "aq1", Type: QuestionTypeArdoise, Points: "10", Time: "30"}
	e.Ready("aq1", q)
	e.StartImmediate(30)
	e.SetArdoiseAnswer("teamA", "Some answer")

	// Verify answer is stored
	state := e.GetState()
	if len(state.ArdoiseAnswers) == 0 {
		t.Fatal("Pre-condition: ArdoiseAnswers should contain answer before InitGame")
	}

	// InitGame resets everything
	e.InitGame()

	stateAfter := e.GetState()
	if len(stateAfter.ArdoiseAnswers) != 0 {
		t.Errorf("ArdoiseAnswers should be empty after InitGame(), got %v", stateAfter.ArdoiseAnswers)
	}
}

// TestArdoiseAnswers_SerializedAsEmptyMapNotNull verifies that ArdoiseAnswers
// serializes as {} (empty JSON object) rather than null when no answers exist.
// This is critical: the frontend relies on the field never being null to reset state.
func TestArdoiseAnswers_SerializedAsEmptyMapNotNull(t *testing.T) {
	e := NewEngine()
	state := e.GetState()

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Failed to marshal GameState: %v", err)
	}

	// Must contain "ARDOISE_ANSWERS":{} (not "ARDOISE_ANSWERS":null)
	jsonStr := string(data)
	if !contains(jsonStr, `"ARDOISE_ANSWERS":{}`) {
		t.Errorf("Expected ARDOISE_ANSWERS to serialize as {}, got: %s", jsonStr)
	}
}

// TestNewEngine_ArdoiseAnswersInitialized verifies that NewEngine initializes
// ArdoiseAnswers as an empty (non-nil) map.
func TestNewEngine_ArdoiseAnswersInitialized(t *testing.T) {
	e := NewEngine()
	state := e.GetState()

	if state.ArdoiseAnswers == nil {
		t.Error("ArdoiseAnswers should be initialized (non-nil) in NewEngine()")
	}
	if len(state.ArdoiseAnswers) != 0 {
		t.Errorf("ArdoiseAnswers should be empty in NewEngine(), got %v", state.ArdoiseAnswers)
	}
}

// ========================================
// #96 — NETWORK_ONLY_LOCALHOST
// ========================================

func TestSetNetworkOnlyLocalhost(t *testing.T) {
	e := NewEngine()

	if e.GetNetworkOnlyLocalhost() != false {
		t.Error("Expected GetNetworkOnlyLocalhost() to return false by default")
	}

	e.SetNetworkOnlyLocalhost(true)
	if !e.GetNetworkOnlyLocalhost() {
		t.Error("Expected GetNetworkOnlyLocalhost() to return true after SetNetworkOnlyLocalhost(true)")
	}

	e.SetNetworkOnlyLocalhost(false)
	if e.GetNetworkOnlyLocalhost() {
		t.Error("Expected GetNetworkOnlyLocalhost() to return false after SetNetworkOnlyLocalhost(false)")
	}
}

func TestGameStateSerializationNetworkField(t *testing.T) {
	e := NewEngine()

	// false case — field must always be present (no omitempty)
	e.SetNetworkOnlyLocalhost(false)
	data, err := json.Marshal(e.GetState())
	if err != nil {
		t.Fatalf("Failed to marshal GameState: %v", err)
	}
	if !contains(string(data), `"NETWORK_ONLY_LOCALHOST":false`) {
		t.Errorf("NETWORK_ONLY_LOCALHOST:false missing from JSON; got: %s", string(data))
	}

	// true case
	e.SetNetworkOnlyLocalhost(true)
	data, err = json.Marshal(e.GetState())
	if err != nil {
		t.Fatalf("Failed to marshal GameState: %v", err)
	}
	if !contains(string(data), `"NETWORK_ONLY_LOCALHOST":true`) {
		t.Errorf("NETWORK_ONLY_LOCALHOST:true missing from JSON; got: %s", string(data))
	}
}

// contains is a simple string containment helper for JSON assertions.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

