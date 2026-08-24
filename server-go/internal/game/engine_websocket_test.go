package game

import (
	"testing"
	"time"
)

// ============================================================================
// Engine WebSocket Integration Tests
// Tests that the game engine correctly handles buzzer events regardless of
// whether they come from TCP or WebSocket (source-agnostic behavior).
// ============================================================================

// --- Tests: Buzzer Registration via WebSocket ---

func TestEngine_WebSocket_BuzzerRegistration(t *testing.T) {
	e := NewEngine()

	// Simulate a buzzer registering via WebSocket (HELLO message)
	// The engine treats all bumpers the same regardless of transport
	data := map[string]interface{}{
		"NAME":    "WS-Buzzer-1",
		"TEAM":    "red",
		"VERSION": "3.0.0",
		"IP":      "192.168.1.50",
	}

	// Register with MAC address (standard buzzer identification)
	e.UpdateBumper("AA:BB:CC:DD:EE:01", data)

	bumper := e.GetBumper("AA:BB:CC:DD:EE:01")
	if bumper == nil {
		t.Fatal("Bumper should exist after WebSocket registration")
	}
	if bumper.Name != "WS-Buzzer-1" {
		t.Errorf("Expected name WS-Buzzer-1, got %s", bumper.Name)
	}
	if bumper.Team != "red" {
		t.Errorf("Expected team red, got %s", bumper.Team)
	}
	if bumper.Version != "3.0.0" {
		t.Errorf("Expected version 3.0.0, got %s", bumper.Version)
	}
}

func TestEngine_WebSocket_MultipleBuzzerRegistration(t *testing.T) {
	e := NewEngine()

	// Register multiple buzzers (simulating WebSocket connections)
	buzzers := []struct {
		mac  string
		name string
		team string
	}{
		{"AA:BB:CC:DD:EE:01", "WS-Buzzer-1", "red"},
		{"AA:BB:CC:DD:EE:02", "WS-Buzzer-2", "blue"},
		{"AA:BB:CC:DD:EE:03", "WS-Buzzer-3", "red"},
	}

	for _, b := range buzzers {
		e.UpdateBumper(b.mac, map[string]interface{}{
			"NAME": b.name,
			"TEAM": b.team,
		})
	}

	// Verify all registered
	for _, b := range buzzers {
		bumper := e.GetBumper(b.mac)
		if bumper == nil {
			t.Errorf("Bumper %s should exist", b.mac)
			continue
		}
		if bumper.Name != b.name {
			t.Errorf("Bumper %s: expected name %s, got %s", b.mac, b.name, bumper.Name)
		}
	}
}

// --- Tests: Buzzer PONG (Ready) via WebSocket ---

func TestEngine_WebSocket_BuzzerReady(t *testing.T) {
	e := NewEngine()

	// Setup
	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})
	e.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{
		"NAME": "WS-Buzzer-1",
		"TEAM": "red",
	})

	// Prepare game
	e.Ready("q1", &Question{ID: "q1", Question: "Test?", Answer: "42"})

	if e.GetPhase() != PhasePrepare {
		t.Errorf("Expected PREPARE phase, got %s", e.GetPhase())
	}

	// WebSocket buzzer responds with PONG
	e.SetBumperReady("AA:BB:CC:DD:EE:01")

	bumper := e.GetBumper("AA:BB:CC:DD:EE:01")
	if !bumper.Ready {
		t.Error("WebSocket buzzer should be marked as ready after PONG")
	}

	// Team should be ready (all bumpers in team are ready)
	team := e.GetTeam("red")
	if !team.Ready {
		t.Error("Team should be ready when all WebSocket buzzers responded")
	}

	// All teams ready should trigger transition
	if !e.AreAllTeamsReady() {
		t.Error("All teams should be ready")
	}
}

// --- Tests: Button Press via WebSocket ---

func TestEngine_WebSocket_ButtonPress(t *testing.T) {
	e := NewEngine()

	// Setup
	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})
	e.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{
		"NAME": "WS-Buzzer-1",
		"TEAM": "red",
	})

	// Start game
	e.StartImmediate(30)

	// WebSocket buzzer presses button
	pressTime := time.Now().UnixMicro()
	e.ProcessButtonPress("AA:BB:CC:DD:EE:01", pressTime, "A")

	bumper := e.GetBumper("AA:BB:CC:DD:EE:01")
	if bumper.Time != pressTime {
		t.Errorf("Expected press time %d, got %d", pressTime, bumper.Time)
	}
	if bumper.Button != "A" {
		t.Errorf("Expected button A, got %s", bumper.Button)
	}
	if bumper.Status != "PAUSE" {
		t.Errorf("Expected status PAUSE, got %s", bumper.Status)
	}

	// Team should be updated
	team := e.GetTeam("red")
	if team.Bumper != "AA:BB:CC:DD:EE:01" {
		t.Errorf("Team should reference WebSocket buzzer, got %s", team.Bumper)
	}

	e.Stop()
}

func TestEngine_WebSocket_QCMButtonPress(t *testing.T) {
	e := NewEngine()

	// Setup
	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})
	e.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{
		"NAME": "WS-Buzzer-1",
		"TEAM": "red",
	})

	// QCM question
	question := &Question{
		ID:       "q1",
		Question: "Capital of France?",
		Type:     QuestionTypeQCM,
		TypedContent: TypedContent{
			QCMAnswers: &QCMAnswers{
				Red:    "London",
				Green:  "Paris",
				Yellow: "Berlin",
				Blue:   "Madrid",
			},
			QCMCorrect: "GREEN",
		},
	}

	e.Ready("q1", question)
	e.StartImmediate(30)

	// WebSocket buzzer presses button B (mapped to GREEN)
	pressTime := time.Now().UnixMicro()
	e.ProcessButtonPress("AA:BB:CC:DD:EE:01", pressTime, "B")

	bumper := e.GetBumper("AA:BB:CC:DD:EE:01")
	if bumper.AnswerColor != AnswerColorGreen {
		t.Errorf("Expected AnswerColor GREEN, got %s", bumper.AnswerColor)
	}

	e.Stop()
}

// --- Tests: Hybrid Mode (TCP + WebSocket Coexistence) ---

func TestEngine_HybridMode_TCPAndWebSocketBuzzers(t *testing.T) {
	e := NewEngine()

	// Setup teams
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red", Color: []int{255, 0, 0}},
		"blue": {Name: "Team Blue", Color: []int{0, 0, 255}},
	})

	// Register TCP buzzer (identified by IP)
	e.UpdateBumper("192.168.1.10", map[string]interface{}{
		"NAME":    "TCP-Buzzer-1",
		"TEAM":    "red",
		"VERSION": "1.209.3",
		"IP":      "192.168.1.10",
	})

	// Register WebSocket buzzer (identified by MAC)
	e.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{
		"NAME":    "WS-Buzzer-1",
		"TEAM":    "blue",
		"VERSION": "3.0.0",
		"IP":      "192.168.1.50",
	})

	// Verify both registered
	tcpBumper := e.GetBumper("192.168.1.10")
	wsBumper := e.GetBumper("AA:BB:CC:DD:EE:01")

	if tcpBumper == nil {
		t.Fatal("TCP buzzer should be registered")
	}
	if wsBumper == nil {
		t.Fatal("WebSocket buzzer should be registered")
	}

	// Prepare and ready
	e.Ready("q1", &Question{ID: "q1", Answer: "42"})

	e.SetBumperReady("192.168.1.10")
	e.SetBumperReady("AA:BB:CC:DD:EE:01")

	if !e.AreAllTeamsReady() {
		t.Error("All teams should be ready with mixed TCP/WS buzzers")
	}

	// Start game
	e.StartImmediate(30)

	// WS buzzer presses first (faster)
	wsPress := time.Now().UnixMicro()
	e.ProcessButtonPress("AA:BB:CC:DD:EE:01", wsPress, "A")

	// TCP buzzer presses second
	tcpPress := time.Now().UnixMicro()
	e.ProcessButtonPress("192.168.1.10", tcpPress, "A")

	// Verify both were recorded (different teams)
	wsBumper = e.GetBumper("AA:BB:CC:DD:EE:01")
	tcpBumper = e.GetBumper("192.168.1.10")

	if wsBumper.Time != wsPress {
		t.Errorf("WS buzzer time not recorded: expected %d, got %d", wsPress, wsBumper.Time)
	}
	if tcpBumper.Time != tcpPress {
		t.Errorf("TCP buzzer time not recorded: expected %d, got %d", tcpPress, tcpBumper.Time)
	}

	// Blue team (WS) should have WS buzzer
	blueTeam := e.GetTeam("blue")
	if blueTeam.Bumper != "AA:BB:CC:DD:EE:01" {
		t.Errorf("Blue team should reference WS buzzer, got %s", blueTeam.Bumper)
	}

	// Red team (TCP) should have TCP buzzer
	redTeam := e.GetTeam("red")
	if redTeam.Bumper != "192.168.1.10" {
		t.Errorf("Red team should reference TCP buzzer, got %s", redTeam.Bumper)
	}

	e.Stop()
}

func TestEngine_HybridMode_SameTeamMixedTransport(t *testing.T) {
	e := NewEngine()

	// Same team with both TCP and WS buzzers
	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})

	// TCP buzzer
	e.UpdateBumper("192.168.1.10", map[string]interface{}{
		"NAME": "TCP-Buzzer",
		"TEAM": "red",
	})

	// WebSocket buzzer
	e.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{
		"NAME": "WS-Buzzer",
		"TEAM": "red",
	})

	e.Ready("q1", &Question{ID: "q1", Answer: "42"})
	e.SetBumperReady("192.168.1.10")
	e.SetBumperReady("AA:BB:CC:DD:EE:01")

	if !e.AreAllTeamsReady() {
		t.Error("Team should be ready with mixed transport buzzers")
	}

	// Start game
	e.StartImmediate(30)

	// TCP buzzer presses first
	e.ProcessButtonPress("192.168.1.10", 1000, "A")

	// WS buzzer in same team tries to press - should be IGNORED (team already buzzed)
	e.ProcessButtonPress("AA:BB:CC:DD:EE:01", 2000, "B")

	wsBumper := e.GetBumper("AA:BB:CC:DD:EE:01")
	if wsBumper.Time != 0 {
		t.Errorf("WS buzzer should be ignored (team already buzzed), got Time=%d", wsBumper.Time)
	}

	team := e.GetTeam("red")
	if team.Bumper != "192.168.1.10" {
		t.Errorf("Team should reference TCP buzzer (first press), got %s", team.Bumper)
	}

	e.Stop()
}

// --- Tests: WebSocket Buzzer with VPlayer Integration ---

func TestEngine_WebSocket_VPlayerBuzzerCoexistence(t *testing.T) {
	e := NewEngine()

	// Setup team
	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})

	// WebSocket physical buzzer + VPlayer in same team
	bumpers := map[string]*Bumper{
		"AA:BB:CC:DD:EE:01": {
			Name:      "WS-Buzzer-1",
			Team:      "red",
			IsVirtual: false,
			IsVPlayer: false,
		},
		"vplayer_alice": {
			Name:      "Alice",
			Team:      "red",
			IsVirtual: true,
			IsVPlayer: true,
		},
	}
	e.SetBumpers(bumpers)

	// QCM question
	question := &Question{
		ID:   "q1",
		Type: QuestionTypeQCM,
		TypedContent: TypedContent{
			QCMAnswers: &QCMAnswers{
				Red: "A", Green: "B", Yellow: "C", Blue: "D",
			},
			QCMCorrect: "RED",
		},
	}

	e.Ready("q1", question)
	e.StartImmediate(30)

	// Physical WS buzzer tries to press - should be IGNORED (team has VPlayer, QCM mode)
	e.ProcessButtonPress("AA:BB:CC:DD:EE:01", time.Now().UnixMicro(), "A")

	wsBumper := e.GetBumper("AA:BB:CC:DD:EE:01")
	if wsBumper.Time != 0 {
		t.Error("WS physical buzzer should be ignored when team has VPlayer for QCM")
	}

	// VPlayer buzzes - should be ACCEPTED
	vplayerPress := time.Now().UnixMicro()
	e.ProcessButtonPress("vplayer_alice", vplayerPress, "RED")

	vplayer := e.GetBumper("vplayer_alice")
	if vplayer.Time == 0 {
		t.Error("VPlayer should be accepted for QCM question")
	}

	team := e.GetTeam("red")
	if team.Bumper != "vplayer_alice" {
		t.Errorf("Team should reference VPlayer, got %s", team.Bumper)
	}

	e.Stop()
}

// --- Tests: Score Tracking for WebSocket Buzzers ---

func TestEngine_WebSocket_ScoreTracking(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})
	e.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{
		"NAME": "WS-Buzzer-1",
		"TEAM": "red",
	})

	// Award points to WS buzzer
	bumperScore, teamScore := e.UpdateScore("AA:BB:CC:DD:EE:01", 10)
	if bumperScore != 10 {
		t.Errorf("Expected bumper score 10, got %d", bumperScore)
	}
	if teamScore != 10 {
		t.Errorf("Expected team score 10, got %d", teamScore)
	}

	// Award more points
	bumperScore, teamScore = e.UpdateScore("AA:BB:CC:DD:EE:01", 5)
	if bumperScore != 15 {
		t.Errorf("Expected bumper score 15, got %d", bumperScore)
	}
	if teamScore != 15 {
		t.Errorf("Expected team score 15, got %d", teamScore)
	}
}

// --- Tests: Game State Reset for WebSocket Buzzers ---

func TestEngine_WebSocket_StateResetOnReady(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})
	e.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{
		"NAME": "WS-Buzzer-1",
		"TEAM": "red",
	})

	// Start and press
	e.StartImmediate(30)
	e.ProcessButtonPress("AA:BB:CC:DD:EE:01", time.Now().UnixMicro(), "A")
	e.Stop()

	// Verify press was recorded
	bumper := e.GetBumper("AA:BB:CC:DD:EE:01")
	if bumper.Time == 0 {
		t.Fatal("Press should have been recorded")
	}

	// Ready new question - should reset bumper state
	e.Ready("q2", &Question{ID: "q2", Answer: "new answer"})

	bumper = e.GetBumper("AA:BB:CC:DD:EE:01")
	if bumper.Time != 0 {
		t.Errorf("Bumper time should be reset after Ready, got %d", bumper.Time)
	}
	if bumper.Button != "" {
		t.Errorf("Bumper button should be reset after Ready, got %s", bumper.Button)
	}
	if bumper.Status != "" {
		t.Errorf("Bumper status should be reset after Ready, got %s", bumper.Status)
	}
	if bumper.Ready {
		t.Error("Bumper should not be ready after Ready")
	}
}

// --- Tests: Buzzer Press Callback ---

func TestEngine_WebSocket_BuzzerPressCallback(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})
	e.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{
		"NAME": "WS-Buzzer-1",
		"TEAM": "red",
	})

	var callbackCalled bool
	var cbBumperID, cbTeamID, cbButton string
	var cbPressTime int64

	e.OnBuzzerPress = func(bumperID, teamID string, pressTime int64, button string) {
		callbackCalled = true
		cbBumperID = bumperID
		cbTeamID = teamID
		cbPressTime = pressTime
		cbButton = button
	}

	e.StartImmediate(30)

	pressTime := time.Now().UnixMicro()
	e.ProcessButtonPress("AA:BB:CC:DD:EE:01", pressTime, "A")

	if !callbackCalled {
		t.Error("OnBuzzerPress callback should have been called for WebSocket buzzer")
	}
	if cbBumperID != "AA:BB:CC:DD:EE:01" {
		t.Errorf("Callback bumperID should be MAC address, got %s", cbBumperID)
	}
	if cbTeamID != "red" {
		t.Errorf("Callback teamID should be 'red', got %s", cbTeamID)
	}
	if cbPressTime != pressTime {
		t.Errorf("Callback pressTime mismatch: expected %d, got %d", pressTime, cbPressTime)
	}
	if cbButton != "A" {
		t.Errorf("Callback button should be 'A', got %s", cbButton)
	}

	e.Stop()
}

// --- Tests: Concurrent WebSocket Buzzer Presses ---

func TestEngine_WebSocket_ConcurrentPresses(t *testing.T) {
	e := NewEngine()

	// Setup 4 teams with 4 WS buzzers
	teams := map[string]*Team{
		"red":    {Name: "Team Red", Color: []int{255, 0, 0}},
		"blue":   {Name: "Team Blue", Color: []int{0, 0, 255}},
		"green":  {Name: "Team Green", Color: []int{0, 255, 0}},
		"yellow": {Name: "Team Yellow", Color: []int{255, 255, 0}},
	}
	e.SetTeams(teams)

	buzzers := []struct {
		mac  string
		team string
	}{
		{"AA:BB:CC:DD:EE:01", "red"},
		{"AA:BB:CC:DD:EE:02", "blue"},
		{"AA:BB:CC:DD:EE:03", "green"},
		{"AA:BB:CC:DD:EE:04", "yellow"},
	}
	for _, b := range buzzers {
		e.UpdateBumper(b.mac, map[string]interface{}{
			"NAME": "Buzzer-" + b.team,
			"TEAM": b.team,
		})
	}

	e.StartImmediate(30)

	// All buzzers press simultaneously (with -race flag this detects data races)
	done := make(chan struct{})
	for i, b := range buzzers {
		go func(mac string, idx int) {
			e.ProcessButtonPress(mac, int64(1000+idx), "A")
			done <- struct{}{}
		}(b.mac, i)
	}

	// Wait for all presses
	for range buzzers {
		<-done
	}

	// Verify each team recorded a press
	for _, b := range buzzers {
		team := e.GetTeam(b.team)
		if team.Time == 0 {
			t.Errorf("Team %s should have a press recorded", b.team)
		}
	}

	e.Stop()
}

// --- Tests: WebSocket Buzzer Latency Verification ---

func TestEngine_WebSocket_PressTimePreserved(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red"},
	})
	e.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{
		"NAME": "WS-Buzzer-1",
		"TEAM": "red",
	})

	e.StartImmediate(30)

	// Simulate a precise press time from the buzzer firmware
	exactPressTime := int64(1708000000000) // A specific microsecond timestamp
	e.ProcessButtonPress("AA:BB:CC:DD:EE:01", exactPressTime, "A")

	bumper := e.GetBumper("AA:BB:CC:DD:EE:01")
	if bumper.Time != exactPressTime {
		t.Errorf("Press time should be preserved exactly: expected %d, got %d", exactPressTime, bumper.Time)
	}

	team := e.GetTeam("red")
	if team.Time != exactPressTime {
		t.Errorf("Team time should match buzzer press time: expected %d, got %d", exactPressTime, team.Time)
	}

	e.Stop()
}

// --- Tests: QCM Hints with WebSocket Buzzer ---

func TestEngine_WebSocket_QCMHintsAtBuzz(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})

	bumpers := map[string]*Bumper{
		"AA:BB:CC:DD:EE:01": {
			Name: "WS-Buzzer-1",
			Team: "red",
		},
	}
	e.SetBumpers(bumpers)

	// QCM question with hints
	question := &Question{
		ID:   "q1",
		Type: QuestionTypeQCM,
		TypedContent: TypedContent{
			QCMHintsEnabled: true,
			QCMAnswers: &QCMAnswers{
				Red: "A", Green: "B", Yellow: "C", Blue: "D",
			},
			QCMCorrect: "GREEN",
		},
	}

	e.Ready("q1", question)

	// Manually add invalidated hints to state (simulating timer-triggered hints)
	e.mu.Lock()
	e.state.QcmInvalidated = []string{"RED", "YELLOW"}
	e.state.Phase = PhaseStarted
	e.state.GameTime = time.Now().UnixMicro()
	e.state.Delay = 30
	e.state.CurrentTime = 10
	e.mu.Unlock()

	// WebSocket buzzer presses
	e.ProcessButtonPress("AA:BB:CC:DD:EE:01", time.Now().UnixMicro(), "B")

	bumper := e.GetBumper("AA:BB:CC:DD:EE:01")
	if bumper.HintsAtBuzz != 2 {
		t.Errorf("Expected HintsAtBuzz=2 (two hints were active), got %d", bumper.HintsAtBuzz)
	}

	e.Stop()
}

// --- Tests: RAZ Scores Affects WebSocket Buzzers ---

func TestEngine_WebSocket_RAZScores(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})
	e.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{
		"NAME": "WS-Buzzer-1",
		"TEAM": "red",
	})

	// Award points
	e.UpdateScore("AA:BB:CC:DD:EE:01", 100)

	// RAZ (reset all scores)
	e.RAZScores()

	bumper := e.GetBumper("AA:BB:CC:DD:EE:01")
	if bumper.Score != 0 {
		t.Errorf("WS buzzer score should be 0 after RAZ, got %d", bumper.Score)
	}

	team := e.GetTeam("red")
	if team.Score != 0 {
		t.Errorf("Team score should be 0 after RAZ, got %d", team.Score)
	}
}

// --- Tests: ForceReady with WebSocket Buzzers ---

func TestEngine_WebSocket_ForceReady(t *testing.T) {
	e := NewEngine()

	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})
	e.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{
		"NAME": "WS-Buzzer-1",
		"TEAM": "red",
	})

	// Prepare game
	e.Ready("q1", &Question{ID: "q1", Answer: "42"})

	// WS buzzer hasn't responded to PONG yet
	bumper := e.GetBumper("AA:BB:CC:DD:EE:01")
	if bumper.Ready {
		t.Error("WS buzzer should not be ready before PONG")
	}

	// Force ready (debug function)
	e.ForceReady()

	bumper = e.GetBumper("AA:BB:CC:DD:EE:01")
	if !bumper.Ready {
		t.Error("WS buzzer should be ready after ForceReady")
	}

	if e.GetPhase() != PhaseReady {
		t.Errorf("Phase should be READY after ForceReady, got %s", e.GetPhase())
	}
}
