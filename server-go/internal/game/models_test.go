package game

import (
	"encoding/json"
	"testing"
)

func TestGamePhase_Values(t *testing.T) {
	phases := []GamePhase{PhaseStopped, PhasePrepare, PhaseReady, PhaseStarted, PhasePaused}
	expected := []string{"STOPPED", "PREPARE", "READY", "STARTED", "PAUSED"}

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
		Points:   "10",
		Time:     "30",
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
		Phase:       PhaseStarted,
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

	if decoded.Phase != PhaseStarted {
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
			Phase:       PhaseStarted,
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
			Phase: PhaseStarted,
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

// ========================================
// Memory Game Tests - Phase 1
// ========================================

func TestQuestionType_MemoryConstant(t *testing.T) {
	if string(QuestionTypeMemory) != "MEMORY" {
		t.Errorf("Expected QuestionTypeMemory to be 'MEMORY', got %s", QuestionTypeMemory)
	}
}

func TestMemoryCard_TextSerialization(t *testing.T) {
	card := MemoryCard{
		Text:    "Paris",
		IsImage: false,
	}

	data, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded MemoryCard
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Text != "Paris" {
		t.Errorf("Text mismatch: expected 'Paris', got '%s'", decoded.Text)
	}
	if decoded.IsImage != false {
		t.Error("IsImage should be false for text card")
	}
}

func TestMemoryCard_ImageSerialization(t *testing.T) {
	card := MemoryCard{
		Image:   "/question/1/memory_1_1_4521.jpg",
		IsImage: true,
	}

	data, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded MemoryCard
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Image != "/question/1/memory_1_1_4521.jpg" {
		t.Errorf("Image path mismatch: got '%s'", decoded.Image)
	}
	if decoded.IsImage != true {
		t.Error("IsImage should be true for image card")
	}
}

func TestMemoryPair_JSONSerialization(t *testing.T) {
	pair := MemoryPair{
		ID: 1,
		Card1: MemoryCard{
			Text:    "Paris",
			IsImage: false,
		},
		Card2: MemoryCard{
			Text:    "France",
			IsImage: false,
		},
	}

	data, err := json.Marshal(pair)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded MemoryPair
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ID != 1 {
		t.Errorf("ID mismatch: expected 1, got %d", decoded.ID)
	}
	if decoded.Card1.Text != "Paris" {
		t.Errorf("Card1 text mismatch")
	}
	if decoded.Card2.Text != "France" {
		t.Errorf("Card2 text mismatch")
	}
}

func TestMemoryConfig_JSONSerialization(t *testing.T) {
	config := MemoryConfig{
		FlipDelay:       3,
		PointsPerPair:   10,
		ErrorPenalty:    5,
		CompletionBonus: 50,
		UseTimer:        true,
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded MemoryConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.FlipDelay != 3 {
		t.Errorf("FlipDelay mismatch: expected 3, got %v", decoded.FlipDelay)
	}
	if decoded.PointsPerPair != 10 {
		t.Errorf("PointsPerPair mismatch: expected 10, got %d", decoded.PointsPerPair)
	}
	if decoded.ErrorPenalty != 5 {
		t.Errorf("ErrorPenalty mismatch: expected 5, got %d", decoded.ErrorPenalty)
	}
	if decoded.CompletionBonus != 50 {
		t.Errorf("CompletionBonus mismatch: expected 50, got %d", decoded.CompletionBonus)
	}
	if decoded.UseTimer != true {
		t.Error("UseTimer should be true")
	}
}

func TestMemoryConfig_UseTimerFalse(t *testing.T) {
	config := MemoryConfig{
		FlipDelay:       3,
		PointsPerPair:   10,
		ErrorPenalty:    0,
		CompletionBonus: 0,
		UseTimer:        false,
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded MemoryConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.UseTimer != false {
		t.Error("UseTimer should be false")
	}
}

func TestQuestion_MemoryType_JSONSerialization(t *testing.T) {
	question := &Question{
		ID:       "5",
		Question: "Match capitals with countries",
		Answer:   "3 paires",
		Type:     QuestionTypeMemory,
		Points:   "30",
		Time:     "60",
		MemoryPairs: []MemoryPair{
			{
				ID:    1,
				Card1: MemoryCard{Text: "Paris", IsImage: false},
				Card2: MemoryCard{Text: "France", IsImage: false},
			},
			{
				ID:    2,
				Card1: MemoryCard{Text: "Berlin", IsImage: false},
				Card2: MemoryCard{Text: "Germany", IsImage: false},
			},
			{
				ID:    3,
				Card1: MemoryCard{Image: "/question/5/memory_3_1_1234.jpg", IsImage: true},
				Card2: MemoryCard{Text: "Tour Eiffel", IsImage: false},
			},
		},
		MemoryConfig: &MemoryConfig{
			FlipDelay:       3,
			PointsPerPair:   10,
			ErrorPenalty:    0,
			CompletionBonus: 0,
			UseTimer:        true,
		},
	}

	data, err := json.Marshal(question)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Question
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != QuestionTypeMemory {
		t.Errorf("Type mismatch: expected MEMORY, got %s", decoded.Type)
	}
	if len(decoded.MemoryPairs) != 3 {
		t.Errorf("Expected 3 pairs, got %d", len(decoded.MemoryPairs))
	}
	if decoded.MemoryConfig == nil {
		t.Fatal("MemoryConfig should not be nil")
	}
	if decoded.MemoryConfig.FlipDelay != 3 {
		t.Errorf("FlipDelay mismatch in decoded question")
	}

	// Verify pair 3 has image card
	if !decoded.MemoryPairs[2].Card1.IsImage {
		t.Error("Pair 3 Card1 should be an image")
	}
	if decoded.MemoryPairs[2].Card1.Image != "/question/5/memory_3_1_1234.jpg" {
		t.Error("Pair 3 Card1 image path mismatch")
	}
}

func TestQuestion_MemoryOmitEmpty(t *testing.T) {
	// NORMAL question should not have MEMORY_PAIRS or MEMORY_CONFIG
	question := &Question{
		ID:       "1",
		Question: "Normal question",
		Answer:   "Answer",
		Type:     QuestionTypeNormal,
		Points:   "10",
		Time:     "30",
	}

	data, err := json.Marshal(question)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, ok := decoded["MEMORY_PAIRS"]; ok {
		t.Error("MEMORY_PAIRS should be omitted for NORMAL question")
	}
	if _, ok := decoded["MEMORY_CONFIG"]; ok {
		t.Error("MEMORY_CONFIG should be omitted for NORMAL question")
	}
}
