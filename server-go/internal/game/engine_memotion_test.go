// Tests for MEMOTION game mode (v5.0.0) — Issues #71/#72
//
// Run: go test ./internal/game/... -v -run TestInitMotionState\|TestSelect\|TestReveal\|TestDoneMotion\|TestSetMotion\|TestProcessButtonPress_Ignores\|TestQuestionType\|TestMotionCard\|TestInitGame_Reset
//
// NOTE: Ready() calls initMotionStateUnsafe() for MEMOTION questions, so card states
// are initialized before StartImmediate(). Tests using setupMotionEngine() have cards
// in "UNPLAYED" state after setup, ready for SelectMotionCard().

package game

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// ============================================================================
// Helpers
// ============================================================================

// makeMotionQuestion builds a MEMOTION question with the given cards and mode.
// mode defaults to "SOLO" (MemoryModeSolo) if empty.
func makeMotionQuestion(id string, cards []MotionCard, mode string) *Question {
	if mode == "" {
		mode = string(MemoryModeSolo)
	}
	return &Question{
		ID:          id,
		Question:    "MEMOTION question " + id,
		Type:        QuestionTypeMemotion,
		MotionCards: cards,
		MotionMode:  mode,
		Points:      "10",
		Time:        "0",
	}
}

// defaultMotionCards returns 3 cards with difficulties 1, 2, 3.
func defaultMotionCards() []MotionCard {
	return []MotionCard{
		{ID: "mc-1", RectoTheme: "Science", Difficulty: 1, QuestionText: "Q1", AnswerText: "A1"},
		{ID: "mc-2", RectoTheme: "Histoire", Difficulty: 2, QuestionText: "Q2", AnswerText: "A2"},
		{ID: "mc-3", RectoTheme: "Sport", Difficulty: 3, QuestionText: "Q3", AnswerText: "A3"},
	}
}

// singleCardMotion returns a single-card MEMOTION question for easy completion tests.
func singleCardMotion(id string, cardID string, difficulty int) *Question {
	return makeMotionQuestion(id, []MotionCard{
		{ID: cardID, RectoTheme: "Test", Difficulty: difficulty, QuestionText: "Q", AnswerText: "A"},
	}, "SOLO")
}

// startMEMOTION prepares an engine for MEMOTION play:
//  1. Calls Ready(id, q)
//  2. Calls StartImmediate(0)
//  3. Calls InitMotionState() — required because StartImmediate bypasses actualStart
//
// Returns the engine ready for SelectMotionCard calls.
func startMEMOTION(t *testing.T, e *Engine, id string, q *Question) {
	t.Helper()
	e.Ready(id, q)
	e.StartImmediate(0)
	e.InitMotionState() // populate MotionCardStates + set SubPhase=GRID
}

// setupMotionEngineAtQuestion brings the engine to QUESTION subphase for the given card.
// It calls startMEMOTION, then SelectMotionCard, then FlipMotionCard.
// Use this in tests that need the engine in QUESTION state (skipping the SELECTED step).
func setupMotionEngineAtQuestion(t *testing.T, e *Engine, id string, q *Question, cardID string) {
	t.Helper()
	startMEMOTION(t, e, id, q)
	if err := e.SelectMotionCard(cardID); err != nil {
		t.Fatalf("setupMotionEngineAtQuestion: SelectMotionCard(%s) failed: %v", cardID, err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("setupMotionEngineAtQuestion: FlipMotionCard() failed: %v", err)
	}
}

// ============================================================================
// InitMotionState — exported method
// ============================================================================

// TestInitMotionState_AllCardsUnplayed verifies that after InitMotionState(),
// every card in MotionCards is set to "UNPLAYED" in MotionCardStates.
func TestInitMotionState_AllCardsUnplayed(t *testing.T) {
	e := NewEngine()
	cards := defaultMotionCards()
	q := makeMotionQuestion("mq1", cards, "SOLO")
	e.Ready("mq1", q)
	e.InitMotionState() // explicit call — production path: Ready→Start→actualStart→initMotionStateUnsafe

	state := e.GetState()
	if len(state.MotionCardStates) != len(cards) {
		t.Errorf("MotionCardStates should have %d entries (one per card), got %d",
			len(cards), len(state.MotionCardStates))
	}
	for _, card := range cards {
		s, ok := state.MotionCardStates[card.ID]
		if !ok {
			t.Errorf("Card %s is missing from MotionCardStates", card.ID)
			continue
		}
		if s != "UNPLAYED" {
			t.Errorf("Card %s should be UNPLAYED after init, got %q", card.ID, s)
		}
	}
}

// TestInitMotionState_SubPhaseGrid verifies that MotionSubPhase is "GRID" and
// MotionSelected is "" after InitMotionState() is called.
func TestInitMotionState_SubPhaseGrid(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	e.Ready("mq1", q)
	e.InitMotionState()

	state := e.GetState()
	if state.MotionSubPhase != "GRID" {
		t.Errorf("MotionSubPhase should be GRID after InitMotionState, got %q", state.MotionSubPhase)
	}
	if state.MotionSelected != "" {
		t.Errorf("MotionSelected should be empty after InitMotionState, got %q", state.MotionSelected)
	}
}

// TestInitMotionState_EmptyMapsNotNil verifies that Motion maps are initialized
// as empty maps (not nil) so JSON serialization sends "{}" not "null".
// This is critical for the "no omitempty" rule: frontend must receive the reset.
func TestInitMotionState_EmptyMapsNotNil(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", []MotionCard{}, "SOLO") // no cards
	e.Ready("mq1", q)
	e.InitMotionState()

	state := e.GetState()
	if state.MotionCardStates == nil {
		t.Error("MotionCardStates should be an empty map, not nil (no omitempty)")
	}
	if state.MotionCardTeams == nil {
		t.Error("MotionCardTeams should be an empty map, not nil (no omitempty)")
	}
}

// ============================================================================
// SelectMotionCard
// ============================================================================

// TestSelectMotionCard_Valid verifies the nominal selection flow:
// SubPhase → SELECTED, card state → SELECTED, MotionSelected = cardID (no timer starts).
func TestSelectMotionCard_Valid(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	err := e.SelectMotionCard("mc-1")
	if err != nil {
		t.Fatalf("SelectMotionCard should succeed for UNPLAYED card, got error: %v", err)
	}

	state := e.GetState()
	if state.MotionSubPhase != "SELECTED" {
		t.Errorf("MotionSubPhase should be SELECTED after select, got %q", state.MotionSubPhase)
	}
	if state.MotionSelected != "mc-1" {
		t.Errorf("MotionSelected should be mc-1, got %q", state.MotionSelected)
	}
	if state.MotionCardStates["mc-1"] != "SELECTED" {
		t.Errorf("Card mc-1 state should be SELECTED, got %q", state.MotionCardStates["mc-1"])
	}
}

// TestSelectMotionCard_ErrorWhenSubPhaseNotGrid verifies that selecting a card
// when SubPhase is already SELECTED (another card selected) returns an error.
func TestSelectMotionCard_ErrorWhenSubPhaseNotGrid(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	// First selection puts us in SELECTED sub-phase
	_ = e.SelectMotionCard("mc-1")

	// Second selection — SubPhase is SELECTED, not GRID → must fail
	err := e.SelectMotionCard("mc-2")
	if err == nil {
		t.Error("SelectMotionCard should return error when SubPhase is not GRID (already in SELECTED)")
	}
}

// TestSelectMotionCard_ErrorWhenCardDone verifies that selecting a DONE card fails.
func TestSelectMotionCard_ErrorWhenCardDone(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	// Play mc-1 to completion
	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()
	_, _, _ = e.DoneMotionCard("mc-1", "")

	// Now mc-1 is DONE — must not be selectable again
	err := e.SelectMotionCard("mc-1")
	if err == nil {
		t.Error("SelectMotionCard should return error for a DONE card")
	}
}

// TestSelectMotionCard_ErrorWhenCardNotFound verifies that an unknown card ID
// returns an error (does not panic).
func TestSelectMotionCard_ErrorWhenCardNotFound(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	err := e.SelectMotionCard("mc-99999")
	if err == nil {
		t.Error("SelectMotionCard should return error for unknown card ID")
	}
}

// TestSelectMotionCard_ErrorWhenNotStarted verifies that card selection is
// rejected when the game phase is not STARTED.
func TestSelectMotionCard_ErrorWhenNotStarted(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	e.Ready("mq1", q)
	// Phase is PREPARE — game has not started

	err := e.SelectMotionCard("mc-1")
	if err == nil {
		t.Error("SelectMotionCard should return error when phase is not STARTED")
	}
}

// ============================================================================
// RevealMotionCard
// ============================================================================

// TestRevealMotionCard_Valid verifies the nominal reveal flow:
// SubPhase → REVEAL, card state → REVEALED.
func TestRevealMotionCard_Valid(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()

	err := e.RevealMotionCard()
	if err != nil {
		t.Fatalf("RevealMotionCard should succeed after SelectMotionCard, got error: %v", err)
	}

	state := e.GetState()
	if state.MotionSubPhase != "REVEAL" {
		t.Errorf("MotionSubPhase should be REVEAL after reveal, got %q", state.MotionSubPhase)
	}
	if state.MotionCardStates["mc-1"] != "REVEALED" {
		t.Errorf("Card mc-1 should be REVEALED, got %q", state.MotionCardStates["mc-1"])
	}
}

// TestRevealMotionCard_ErrorWhenSubPhaseNotQuestion verifies that Reveal called
// while SubPhase is GRID (no card selected) returns an error.
func TestRevealMotionCard_ErrorWhenSubPhaseNotQuestion(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	// SubPhase is GRID — Reveal must fail
	err := e.RevealMotionCard()
	if err == nil {
		t.Error("RevealMotionCard should return error when SubPhase is not QUESTION")
	}
}

// TestRevealMotionCard_ErrorWhenSubPhaseReveal verifies that calling Reveal twice
// (already in REVEAL state) returns an error.
func TestRevealMotionCard_ErrorWhenSubPhaseReveal(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard() // First reveal — OK

	// Second reveal — SubPhase is REVEAL, not QUESTION → must fail
	err := e.RevealMotionCard()
	if err == nil {
		t.Error("RevealMotionCard should return error when already in REVEAL sub-phase")
	}
}

// ============================================================================
// DoneMotionCard — points attribution by difficulty
// ============================================================================

// TestDoneMotionCard_DifficultyPoints is a table-driven test for the full
// difficulty → points mapping: 1→1pt, 2→3pts, 3→5pts.
func TestDoneMotionCard_DifficultyPoints(t *testing.T) {
	tests := []struct {
		difficulty     int
		expectedPoints int
	}{
		{1, 1},
		{2, 3},
		{3, 5},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Difficulty%d", tt.difficulty), func(t *testing.T) {
			e := NewEngine()
			e.SetTeams(map[string]*Team{
				"red": {Name: "Team Red", Color: []int{255, 0, 0}},
			})
			q := singleCardMotion("mq1", "mc-1", tt.difficulty)
			startMEMOTION(t, e, "mq1", q)
			defer e.Stop()

			_ = e.SelectMotionCard("mc-1")
			_ = e.FlipMotionCard()
			_ = e.RevealMotionCard()

			points, _, err := e.DoneMotionCard("mc-1", "red")
			if err != nil {
				t.Fatalf("DoneMotionCard should succeed, got error: %v", err)
			}
			if points != tt.expectedPoints {
				t.Errorf("Difficulty %d: expected %d points, got %d",
					tt.difficulty, tt.expectedPoints, points)
			}
		})
	}
}

// TestDoneMotionCard_Difficulty1_1Point verifies difficulty-1 awards exactly 1 point.
func TestDoneMotionCard_Difficulty1_1Point(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	q := singleCardMotion("mq1", "mc-1", 1)
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()

	points, _, err := e.DoneMotionCard("mc-1", "red")
	if err != nil {
		t.Fatalf("DoneMotionCard error: %v", err)
	}
	if points != 1 {
		t.Errorf("Difficulty 1 should award 1 point, got %d", points)
	}
}

// TestDoneMotionCard_Difficulty2_3Points verifies difficulty-2 awards exactly 3 points.
func TestDoneMotionCard_Difficulty2_3Points(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	q := singleCardMotion("mq1", "mc-1", 2)
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()

	points, _, err := e.DoneMotionCard("mc-1", "red")
	if err != nil {
		t.Fatalf("DoneMotionCard error: %v", err)
	}
	if points != 3 {
		t.Errorf("Difficulty 2 should award 3 points, got %d", points)
	}
}

// TestDoneMotionCard_Difficulty3_5Points verifies difficulty-3 awards exactly 5 points.
func TestDoneMotionCard_Difficulty3_5Points(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	q := singleCardMotion("mq1", "mc-1", 3)
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()

	points, _, err := e.DoneMotionCard("mc-1", "red")
	if err != nil {
		t.Fatalf("DoneMotionCard error: %v", err)
	}
	if points != 5 {
		t.Errorf("Difficulty 3 should award 5 points, got %d", points)
	}
}

// TestDoneMotionCard_NoWinner_ZeroPoints verifies that an empty WinnerTeam awards
// 0 points and still transitions the card to DONE and SubPhase to GRID.
func TestDoneMotionCard_NoWinner_ZeroPoints(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-2")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()

	points, _, err := e.DoneMotionCard("mc-2", "") // empty winner
	if err != nil {
		t.Fatalf("DoneMotionCard should succeed with no winner, got error: %v", err)
	}
	if points != 0 {
		t.Errorf("Empty winner should award 0 points, got %d", points)
	}

	state := e.GetState()
	if state.MotionCardStates["mc-2"] != "DONE" {
		t.Errorf("Card mc-2 should be DONE, got %q", state.MotionCardStates["mc-2"])
	}
	if state.MotionSubPhase != "GRID" {
		t.Errorf("SubPhase should return to GRID, got %q", state.MotionSubPhase)
	}
	if state.MotionSelected != "" {
		t.Errorf("MotionSelected should be empty after done, got %q", state.MotionSelected)
	}
}

// TestDoneMotionCard_CardBecomeDoneReturnsGrid verifies the full state machine transition:
// card → DONE, SubPhase → GRID, MotionSelected → "", MotionCardTeams updated.
func TestDoneMotionCard_CardBecomeDoneReturnsGrid(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-2")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()

	_, _, err := e.DoneMotionCard("mc-2", "red")
	if err != nil {
		t.Fatalf("DoneMotionCard should succeed: %v", err)
	}

	state := e.GetState()
	if state.MotionCardStates["mc-2"] != "DONE" {
		t.Errorf("Card mc-2 should be DONE, got %q", state.MotionCardStates["mc-2"])
	}
	if state.MotionSubPhase != "GRID" {
		t.Errorf("MotionSubPhase should return to GRID, got %q", state.MotionSubPhase)
	}
	if state.MotionSelected != "" {
		t.Errorf("MotionSelected should be empty, got %q", state.MotionSelected)
	}
	if state.MotionCardTeams["mc-2"] != "red" {
		t.Errorf("MotionCardTeams[mc-2] should be 'red', got %q", state.MotionCardTeams["mc-2"])
	}
}

// TestDoneMotionCard_TeamScoreUpdated verifies that UpdateTeamScore is called and
// the winning team's TeamPoints is incremented by the correct number of points.
func TestDoneMotionCard_TeamScoreUpdated(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	q := singleCardMotion("mq1", "mc-1", 2) // Difficulty 2 → 3 pts
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()
	_, _, _ = e.DoneMotionCard("mc-1", "red")

	team := e.GetTeam("red")
	if team == nil {
		t.Fatal("Team 'red' should exist")
	}
	if team.TeamPoints != 3 {
		t.Errorf("Team 'red' TeamPoints should be 3 (difficulty 2 = 3pts), got %d", team.TeamPoints)
	}
}

// TestDoneMotionCard_IsComplete_AllCardsDone verifies that isComplete=true is returned
// when the last card is marked DONE.
func TestDoneMotionCard_IsComplete_AllCardsDone(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	q := singleCardMotion("mq1", "mc-1", 1) // Only 1 card → completion on first done
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()

	_, isComplete, err := e.DoneMotionCard("mc-1", "red")
	if err != nil {
		t.Fatalf("DoneMotionCard should succeed: %v", err)
	}
	if !isComplete {
		t.Error("isComplete should be true when all cards are DONE")
	}
}

// TestDoneMotionCard_IsNotComplete_CardsRemaining verifies that isComplete=false is
// returned when cards remain (not all DONE).
func TestDoneMotionCard_IsNotComplete_CardsRemaining(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO") // 3 cards
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()

	_, isComplete, err := e.DoneMotionCard("mc-1", "red")
	if err != nil {
		t.Fatalf("DoneMotionCard should succeed: %v", err)
	}
	if isComplete {
		t.Error("isComplete should be false — mc-2 and mc-3 are still UNPLAYED")
	}
}

// TestDoneMotionCard_FromQuestionSubPhase verifies that Done can be called directly
// from QUESTION sub-phase (without a Reveal step), as per the spec
// (preconditions: SubPhase ∈ {QUESTION, REVEAL}).
func TestDoneMotionCard_FromQuestionSubPhase(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	q := singleCardMotion("mq1", "mc-1", 1)
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	// SubPhase = QUESTION — skip RevealMotionCard

	_, _, err := e.DoneMotionCard("mc-1", "red")
	if err != nil {
		t.Errorf("DoneMotionCard should work from QUESTION sub-phase (spec: SubPhase ∈ {QUESTION,REVEAL}), got error: %v", err)
	}

	state := e.GetState()
	if state.MotionCardStates["mc-1"] != "DONE" {
		t.Errorf("Card should be DONE, got %q", state.MotionCardStates["mc-1"])
	}
}

// ============================================================================
// DoneMotionCard — team rotation by mode
// ============================================================================

// TestDoneMotionCard_Rotation_ChacunSonTour_Win verifies that CHACUN_SON_TOUR rotates
// to the next team after every card, even when the current team wins.
func TestDoneMotionCard_Rotation_ChacunSonTour_Win(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red", Color: []int{255, 0, 0}},
		"blue": {Name: "Team Blue", Color: []int{0, 0, 255}},
	})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "CHACUN_SON_TOUR")
	e.Ready("mq1", q)
	_ = e.SetMotionParticipatingTeams([]string{"red", "blue"})
	e.StartImmediate(0)
	e.InitMotionState()
	defer e.Stop()

	// red is current team, red wins
	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()
	_, _, _ = e.DoneMotionCard("mc-1", "red")

	state := e.GetState()
	if state.MotionCurrentTeam != "blue" {
		t.Errorf("CHACUN_SON_TOUR: expected rotation to 'blue' after red wins, got %q",
			state.MotionCurrentTeam)
	}
}

// TestDoneMotionCard_Rotation_ChacunSonTour_NoWin verifies that CHACUN_SON_TOUR
// also rotates when there is no winner (empty WinnerTeam).
func TestDoneMotionCard_Rotation_ChacunSonTour_NoWin(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red", Color: []int{255, 0, 0}},
		"blue": {Name: "Team Blue", Color: []int{0, 0, 255}},
	})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "CHACUN_SON_TOUR")
	e.Ready("mq1", q)
	_ = e.SetMotionParticipatingTeams([]string{"red", "blue"})
	e.StartImmediate(0)
	e.InitMotionState()
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()
	_, _, _ = e.DoneMotionCard("mc-1", "") // no winner

	state := e.GetState()
	if state.MotionCurrentTeam != "blue" {
		t.Errorf("CHACUN_SON_TOUR: expected rotation to 'blue' even with no winner, got %q",
			state.MotionCurrentTeam)
	}
}

// TestDoneMotionCard_Rotation_ChacunSonTour_Circular verifies that CHACUN_SON_TOUR
// wraps around back to the first team after all teams have played once.
func TestDoneMotionCard_Rotation_ChacunSonTour_Circular(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red", Color: []int{255, 0, 0}},
		"blue": {Name: "Team Blue", Color: []int{0, 0, 255}},
	})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "CHACUN_SON_TOUR")
	e.Ready("mq1", q)
	_ = e.SetMotionParticipatingTeams([]string{"red", "blue"})
	e.StartImmediate(0)
	e.InitMotionState()
	defer e.Stop()

	// Card 1: red → blue
	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()
	_, _, _ = e.DoneMotionCard("mc-1", "")

	// Card 2: blue → red (wrap-around)
	_ = e.SelectMotionCard("mc-2")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()
	_, _, _ = e.DoneMotionCard("mc-2", "")

	state := e.GetState()
	if state.MotionCurrentTeam != "red" {
		t.Errorf("CHACUN_SON_TOUR: expected circular wrap back to 'red', got %q",
			state.MotionCurrentTeam)
	}
}

// TestDoneMotionCard_Rotation_TantQueJeGagne_Win verifies that TANT_QUE_JE_GAGNE
// keeps the same team when they win (do not rotate).
func TestDoneMotionCard_Rotation_TantQueJeGagne_Win(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red", Color: []int{255, 0, 0}},
		"blue": {Name: "Team Blue", Color: []int{0, 0, 255}},
	})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "TANT_QUE_JE_GAGNE")
	e.Ready("mq1", q)
	_ = e.SetMotionParticipatingTeams([]string{"red", "blue"})
	e.StartImmediate(0)
	e.InitMotionState()
	defer e.Stop()

	// red wins → should keep the hand
	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()
	_, _, _ = e.DoneMotionCard("mc-1", "red")

	state := e.GetState()
	if state.MotionCurrentTeam != "red" {
		t.Errorf("TANT_QUE_JE_GAGNE: team should keep hand after win, expected 'red', got %q",
			state.MotionCurrentTeam)
	}
}

// TestDoneMotionCard_Rotation_TantQueJeGagne_NoWin verifies that TANT_QUE_JE_GAGNE
// rotates to the next team when there is no winner.
func TestDoneMotionCard_Rotation_TantQueJeGagne_NoWin(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red", Color: []int{255, 0, 0}},
		"blue": {Name: "Team Blue", Color: []int{0, 0, 255}},
	})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "TANT_QUE_JE_GAGNE")
	e.Ready("mq1", q)
	_ = e.SetMotionParticipatingTeams([]string{"red", "blue"})
	e.StartImmediate(0)
	e.InitMotionState()
	defer e.Stop()

	// no winner → rotate
	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()
	_, _, _ = e.DoneMotionCard("mc-1", "")

	state := e.GetState()
	if state.MotionCurrentTeam != "blue" {
		t.Errorf("TANT_QUE_JE_GAGNE: should rotate to 'blue' after no winner, got %q",
			state.MotionCurrentTeam)
	}
}

// TestDoneMotionCard_Rotation_Solo_NoRotation verifies that SOLO mode never rotates,
// even after multiple card completions.
func TestDoneMotionCard_Rotation_Solo_NoRotation(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	e.Ready("mq1", q)
	_ = e.SetMotionParticipatingTeams([]string{"red"})
	e.StartImmediate(0)
	e.InitMotionState()
	defer e.Stop()

	// Complete two cards — SOLO should never change currentTeam
	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()
	_, _, _ = e.DoneMotionCard("mc-1", "red")

	_ = e.SelectMotionCard("mc-2")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()
	_, _, _ = e.DoneMotionCard("mc-2", "")

	state := e.GetState()
	if state.MotionCurrentTeam != "red" {
		t.Errorf("SOLO mode: MotionCurrentTeam should never change, expected 'red', got %q",
			state.MotionCurrentTeam)
	}
}

// ============================================================================
// SetMotionParticipatingTeams
// ============================================================================

// TestSetMotionParticipatingTeams_Valid verifies the nominal case:
// MotionCurrentTeam = teams[0], MotionParticipatingTeams populated.
func TestSetMotionParticipatingTeams_Valid(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red", Color: []int{255, 0, 0}},
		"blue": {Name: "Team Blue", Color: []int{0, 0, 255}},
	})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "CHACUN_SON_TOUR")
	e.Ready("mq1", q)

	err := e.SetMotionParticipatingTeams([]string{"red", "blue"})
	if err != nil {
		t.Fatalf("SetMotionParticipatingTeams should succeed, got error: %v", err)
	}

	state := e.GetState()
	if state.MotionCurrentTeam != "red" {
		t.Errorf("MotionCurrentTeam should be 'red' (first team), got %q",
			state.MotionCurrentTeam)
	}
	if len(state.MotionParticipatingTeams) != 2 {
		t.Errorf("MotionParticipatingTeams should have 2 teams, got %d",
			len(state.MotionParticipatingTeams))
	}
	if state.MotionParticipatingTeams[0] != "red" {
		t.Errorf("First participating team should be 'red', got %q",
			state.MotionParticipatingTeams[0])
	}
}

// TestSetMotionParticipatingTeams_ErrorWhenPhaseNotPrepareOrReady verifies that
// calling SetMotionParticipatingTeams during STARTED phase returns an error.
func TestSetMotionParticipatingTeams_ErrorWhenPhaseNotPrepareOrReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red", Color: []int{255, 0, 0}},
		"blue": {Name: "Team Blue", Color: []int{0, 0, 255}},
	})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "CHACUN_SON_TOUR")
	e.Ready("mq1", q)
	e.StartImmediate(0) // Phase = STARTED

	err := e.SetMotionParticipatingTeams([]string{"red", "blue"})
	if err == nil {
		t.Error("SetMotionParticipatingTeams should return error during STARTED phase")
	}

	e.Stop()
}

// TestSetMotionParticipatingTeams_MotionCurrentTeamColor verifies that
// MotionCurrentTeamColor is set to the first team's RGB color.
func TestSetMotionParticipatingTeams_MotionCurrentTeamColor(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	e.Ready("mq1", q)

	_ = e.SetMotionParticipatingTeams([]string{"red"})

	state := e.GetState()
	if len(state.MotionCurrentTeamColor) != 3 {
		t.Errorf("MotionCurrentTeamColor should have 3 channels (RGB), got %d",
			len(state.MotionCurrentTeamColor))
	}
	if state.MotionCurrentTeamColor[0] != 255 ||
		state.MotionCurrentTeamColor[1] != 0 ||
		state.MotionCurrentTeamColor[2] != 0 {
		t.Errorf("MotionCurrentTeamColor should be [255,0,0] for red team, got %v",
			state.MotionCurrentTeamColor)
	}
}

// TestSetMotionParticipatingTeams_AllowedInReadyPhase verifies that teams can
// be set during the READY phase (not only PREPARE).
func TestSetMotionParticipatingTeams_AllowedInReadyPhase(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	e.Ready("mq1", q)
	e.SetBumperReady("b1") // attempt to transition to READY

	err := e.SetMotionParticipatingTeams([]string{"red"})
	if err != nil {
		t.Errorf("SetMotionParticipatingTeams should succeed in READY phase, got error: %v", err)
	}
}

// ============================================================================
// ProcessButtonPress — MEMOTION ignores physical buzzes
// ============================================================================

// TestProcessButtonPress_IgnoresMEMOTIONQuestion verifies that physical buzzer presses
// are silently ignored for MEMOTION questions, analogous to MEMORY (TestEngine_ProcessButtonPress_IgnoresMemoryQuestions).
func TestProcessButtonPress_IgnoresMEMOTIONQuestion(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	// Inject MEMOTION question directly (same pattern as existing MEMORY test)
	e.state.Question = makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")

	e.StartImmediate(0)
	defer e.Stop()

	pressTime := int64(1000000)
	e.ProcessButtonPress("b1", pressTime, "A")

	// Bumper time must NOT have been recorded
	bumper := e.GetBumper("b1")
	if bumper.Time != 0 {
		t.Errorf("Buzz should be ignored for MEMOTION questions — Time should be 0, got %d",
			bumper.Time)
	}

	// Team time must NOT have been recorded
	team := e.GetTeam("red")
	if team.Time != 0 {
		t.Errorf("Team time should not be set for MEMOTION questions, got %d", team.Time)
	}

	// Phase must remain STARTED (buzz must not trigger a pause)
	if e.GetPhase() != PhaseStarted {
		t.Errorf("Phase should remain STARTED after ignored buzz, got %s", e.GetPhase())
	}
}

// TestProcessButtonPress_IgnoresMEMOTION_AllowsNormal verifies that after switching
// from MEMOTION to a NORMAL question, buzzes are accepted normally.
func TestProcessButtonPress_IgnoresMEMOTION_AllowsNormal(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red": {Name: "Team Red", Color: []int{255, 0, 0}},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	// NORMAL question — buzz should be accepted
	normalQ := &Question{
		ID:       "n1",
		Question: "Normal question",
		Type:     QuestionTypeNormal,
		Answer:   "42",
		Points:   "10",
		Time:     "30",
	}
	e.Ready("n1", normalQ)
	e.StartImmediate(30)
	defer e.Stop()

	e.ProcessButtonPress("b1", int64(2000000), "A")

	bumper := e.GetBumper("b1")
	if bumper.Time == 0 {
		t.Error("Buzz should be accepted for NORMAL questions (not MEMOTION)")
	}
}

// ============================================================================
// InitGame — MEMOTION fields reset
// ============================================================================

// TestInitGame_ResetsMEMOTIONFields verifies that InitGame resets all Motion* fields
// to their zero values (empty strings, empty maps, empty slices).
func TestInitGame_ResetsMEMOTIONFields(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)

	// Perform a partial action to populate some motion state
	_ = e.SelectMotionCard("mc-1")

	e.Stop()
	e.InitGame()

	state := e.GetState()

	if state.MotionSubPhase != "" {
		t.Errorf("MotionSubPhase should be empty after InitGame, got %q", state.MotionSubPhase)
	}
	if state.MotionSelected != "" {
		t.Errorf("MotionSelected should be empty after InitGame, got %q", state.MotionSelected)
	}
	if state.MotionCurrentTeam != "" {
		t.Errorf("MotionCurrentTeam should be empty after InitGame, got %q", state.MotionCurrentTeam)
	}
	if len(state.MotionParticipatingTeams) != 0 {
		t.Errorf("MotionParticipatingTeams should be empty after InitGame, got %v",
			state.MotionParticipatingTeams)
	}
	if len(state.MotionCardStates) != 0 {
		t.Errorf("MotionCardStates should be empty after InitGame, got %v",
			state.MotionCardStates)
	}
	if len(state.MotionCardTeams) != 0 {
		t.Errorf("MotionCardTeams should be empty after InitGame, got %v",
			state.MotionCardTeams)
	}
	if len(state.MotionCurrentTeamColor) != 0 {
		t.Errorf("MotionCurrentTeamColor should be empty after InitGame, got %v",
			state.MotionCurrentTeamColor)
	}
}

// TestInitGame_ResetsMEMOTION_NotNilMaps verifies that after InitGame, the Motion*
// maps/slices are initialized (not nil) so the frontend receives "{}" / "[]" in JSON.
func TestInitGame_ResetsMEMOTION_NotNilMaps(t *testing.T) {
	e := NewEngine()
	e.InitGame()

	state := e.GetState()
	if state.MotionCardStates == nil {
		t.Error("MotionCardStates should be empty map (not nil) after InitGame")
	}
	if state.MotionCardTeams == nil {
		t.Error("MotionCardTeams should be empty map (not nil) after InitGame")
	}
	if state.MotionParticipatingTeams == nil {
		t.Error("MotionParticipatingTeams should be empty slice (not nil) after InitGame")
	}
	if state.MotionCurrentTeamColor == nil {
		t.Error("MotionCurrentTeamColor should be empty slice (not nil) after InitGame")
	}
}

// ============================================================================
// QuestionTypeMemotion — constant and struct validation
// ============================================================================

// TestQuestionTypeMemotion_Defined verifies that QuestionTypeMemotion is defined
// and distinct from existing question types.
func TestQuestionTypeMemotion_Defined(t *testing.T) {
	if QuestionTypeMemotion == QuestionTypeNormal {
		t.Error("QuestionTypeMemotion should not equal QuestionTypeNormal")
	}
	if QuestionTypeMemotion == QuestionTypeQCM {
		t.Error("QuestionTypeMemotion should not equal QuestionTypeQCM")
	}
	if QuestionTypeMemotion == QuestionTypeMemory {
		t.Error("QuestionTypeMemotion should not equal QuestionTypeMemory")
	}
	if string(QuestionTypeMemotion) != "MEMOTION" {
		t.Errorf("QuestionTypeMemotion should be 'MEMOTION', got %q", string(QuestionTypeMemotion))
	}
}

// TestMotionCard_Fields verifies that MotionCard struct has the expected fields
// and can be constructed with all required values from the contract.
func TestMotionCard_Fields(t *testing.T) {
	card := MotionCard{
		ID:            "mc-1",
		RectoTheme:    "Science",
		RectoImage:    "img/science.jpg",
		Difficulty:    2,
		QuestionText:  "What is gravity?",
		QuestionImage: "img/gravity.jpg",
		AnswerText:    "A force",
		AnswerImage:   "img/force.jpg",
	}

	if card.ID != "mc-1" {
		t.Errorf("ID should be mc-1, got %q", card.ID)
	}
	if card.Difficulty != 2 {
		t.Errorf("Difficulty should be 2, got %d", card.Difficulty)
	}
	if card.RectoTheme != "Science" {
		t.Errorf("RectoTheme should be 'Science', got %q", card.RectoTheme)
	}
	if card.QuestionText != "What is gravity?" {
		t.Errorf("QuestionText should be 'What is gravity?', got %q", card.QuestionText)
	}
	if card.AnswerText != "A force" {
		t.Errorf("AnswerText should be 'A force', got %q", card.AnswerText)
	}
}

// ============================================================================
// FlipMotionCard — new action (v5.0.1)
// ============================================================================

// TestSelectMotionCard_SetsSelectedSubphase verifies that SelectMotionCard sets the
// card state to "SELECTED" and subphase to "SELECTED" (not QUESTION) — no timer starts.
func TestSelectMotionCard_SetsSelectedSubphase(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	err := e.SelectMotionCard("mc-1")
	if err != nil {
		t.Fatalf("SelectMotionCard should succeed, got: %v", err)
	}

	state := e.GetState()
	if state.MotionSubPhase != "SELECTED" {
		t.Errorf("MotionSubPhase should be SELECTED after SelectMotionCard, got %q", state.MotionSubPhase)
	}
	if state.MotionCardStates["mc-1"] != "SELECTED" {
		t.Errorf("Card mc-1 state should be SELECTED, got %q", state.MotionCardStates["mc-1"])
	}
	if state.MotionSelected != "mc-1" {
		t.Errorf("MotionSelected should be mc-1, got %q", state.MotionSelected)
	}
}

// TestFlipMotionCard_SetsQuestionSubphase verifies the full two-step flow:
// SelectMotionCard (SELECTED) → FlipMotionCard (QUESTION).
func TestFlipMotionCard_SetsQuestionSubphase(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1") // → SELECTED
	err := e.FlipMotionCard()      // → QUESTION
	if err != nil {
		t.Fatalf("FlipMotionCard should succeed after SelectMotionCard, got: %v", err)
	}

	state := e.GetState()
	if state.MotionSubPhase != "QUESTION" {
		t.Errorf("MotionSubPhase should be QUESTION after FlipMotionCard, got %q", state.MotionSubPhase)
	}
	if state.MotionCardStates["mc-1"] != "QUESTION" {
		t.Errorf("Card mc-1 state should be QUESTION after flip, got %q", state.MotionCardStates["mc-1"])
	}
	if state.MotionSelected != "mc-1" {
		t.Errorf("MotionSelected should still be mc-1, got %q", state.MotionSelected)
	}
}

// TestFlipMotionCard_PreconditionNotSelected verifies that FlipMotionCard returns an error
// when called from GRID subphase (no card selected).
func TestFlipMotionCard_PreconditionNotSelected(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	// SubPhase is GRID — no card selected
	err := e.FlipMotionCard()
	if err == nil {
		t.Error("FlipMotionCard should return error when SubPhase is not SELECTED (is GRID)")
	}

	// Also fails when already in QUESTION
	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard() // first flip — OK
	err = e.FlipMotionCard() // second flip — should fail (subphase is QUESTION)
	if err == nil {
		t.Error("FlipMotionCard should return error when SubPhase is already QUESTION")
	}
}

// TestDoneMotionCard_CancelFromSelected verifies that calling DoneMotionCard while in
// SELECTED subphase cancels the selection: card returns to UNPLAYED, subphase → GRID,
// returns (0, false, nil).
func TestDoneMotionCard_CancelFromSelected(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-2") // → SELECTED

	points, isComplete, err := e.DoneMotionCard("mc-2", "") // cancel
	if err != nil {
		t.Fatalf("DoneMotionCard from SELECTED should succeed (cancel), got error: %v", err)
	}
	if points != 0 {
		t.Errorf("Cancellation should award 0 points, got %d", points)
	}
	if isComplete {
		t.Error("isComplete should be false after cancellation")
	}

	state := e.GetState()
	if state.MotionSubPhase != "GRID" {
		t.Errorf("SubPhase should return to GRID after cancellation, got %q", state.MotionSubPhase)
	}
	if state.MotionSelected != "" {
		t.Errorf("MotionSelected should be empty after cancellation, got %q", state.MotionSelected)
	}
	if state.MotionCardStates["mc-2"] != "UNPLAYED" {
		t.Errorf("Card mc-2 should be UNPLAYED after cancellation, got %q", state.MotionCardStates["mc-2"])
	}

	// Card should be re-selectable after cancellation
	err = e.SelectMotionCard("mc-2")
	if err != nil {
		t.Errorf("Card mc-2 should be re-selectable after cancellation, got error: %v", err)
	}
}

// ============================================================================
// motionCardPoints — configurable points par difficulté (v5.0.5)
// ============================================================================

// TestMotionCardPoints_DefaultFallback verifies that without MotionConfig,
// motionCardPoints returns the built-in defaults: 1★→1pt, 2★→3pts, 3★→5pts.
func TestMotionCardPoints_DefaultFallback(t *testing.T) {
	tests := []struct {
		difficulty int
		expected   int
	}{
		{1, 1},
		{2, 3},
		{3, 5},
	}

	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	// q.MotionConfig is nil — no custom config
	e.Ready("mq1", q)

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Difficulty%d", tt.difficulty), func(t *testing.T) {
			got := e.motionCardPoints(tt.difficulty)
			if got != tt.expected {
				t.Errorf("motionCardPoints(%d) without config = %d, want %d",
					tt.difficulty, got, tt.expected)
			}
		})
	}
}

// TestMotionCardPoints_CustomConfig verifies that a fully configured MotionConfig
// overrides all default values: 1★→2pts, 2★→6pts, 3★→10pts.
func TestMotionCardPoints_CustomConfig(t *testing.T) {
	tests := []struct {
		difficulty int
		expected   int
	}{
		{1, 2},
		{2, 6},
		{3, 10},
	}

	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	q.MotionConfig = &MotionConfig{Points1Star: 2, Points2Star: 6, Points3Star: 10}
	e.Ready("mq1", q)

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Difficulty%d", tt.difficulty), func(t *testing.T) {
			got := e.motionCardPoints(tt.difficulty)
			if got != tt.expected {
				t.Errorf("motionCardPoints(%d) with config(2/6/10) = %d, want %d",
					tt.difficulty, got, tt.expected)
			}
		})
	}
}

// TestMotionCardPoints_PartialConfig verifies that a zero value in MotionConfig
// is treated as "not configured" and falls back to the default for that difficulty.
// MotionConfig{Points1Star:5, Points2Star:0, Points3Star:0} →
//   1★=5pts (configured), 2★=3pts (fallback), 3★=5pts (fallback).
func TestMotionCardPoints_PartialConfig(t *testing.T) {
	tests := []struct {
		difficulty int
		expected   int
		desc       string
	}{
		{1, 5, "custom (Points1Star=5 > 0)"},
		{2, 3, "fallback (Points2Star=0 → default 3)"},
		{3, 5, "fallback (Points3Star=0 → default 5)"},
	}

	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	q.MotionConfig = &MotionConfig{Points1Star: 5, Points2Star: 0, Points3Star: 0}
	e.Ready("mq1", q)

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Difficulty%d_%s", tt.difficulty, tt.desc), func(t *testing.T) {
			got := e.motionCardPoints(tt.difficulty)
			if got != tt.expected {
				t.Errorf("motionCardPoints(%d) [%s]: got %d, want %d",
					tt.difficulty, tt.desc, got, tt.expected)
			}
		})
	}
}

// TestMotionConfig_JSONSerialization verifies that MotionConfig round-trips cleanly
// through JSON marshal/unmarshal with all fields preserved.
func TestMotionConfig_JSONSerialization(t *testing.T) {
	cfg := MotionConfig{
		Points1Star: 2,
		Points2Star: 6,
		Points3Star: 10,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded MotionConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Points1Star != 2 {
		t.Errorf("Points1Star mismatch: expected 2, got %d", decoded.Points1Star)
	}
	if decoded.Points2Star != 6 {
		t.Errorf("Points2Star mismatch: expected 6, got %d", decoded.Points2Star)
	}
	if decoded.Points3Star != 10 {
		t.Errorf("Points3Star mismatch: expected 10, got %d", decoded.Points3Star)
	}

	// JSON keys must match contract: POINTS_1_STAR / POINTS_2_STAR / POINTS_3_STAR
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to raw map failed: %v", err)
	}
	for _, key := range []string{"POINTS_1_STAR", "POINTS_2_STAR", "POINTS_3_STAR"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("JSON key %q missing from MotionConfig serialization", key)
		}
	}
}

// TestMotionConfig_JSONSerialization_InQuestion verifies that MotionConfig is
// properly embedded in a MEMOTION Question and round-trips through JSON.
func TestMotionConfig_JSONSerialization_InQuestion(t *testing.T) {
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	q.MotionConfig = &MotionConfig{Points1Star: 2, Points2Star: 6, Points3Star: 10}

	data, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Question
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.MotionConfig == nil {
		t.Fatal("MotionConfig should not be nil after round-trip")
	}
	if decoded.MotionConfig.Points1Star != 2 {
		t.Errorf("Points1Star mismatch: expected 2, got %d", decoded.MotionConfig.Points1Star)
	}
	if decoded.MotionConfig.Points2Star != 6 {
		t.Errorf("Points2Star mismatch: expected 6, got %d", decoded.MotionConfig.Points2Star)
	}
	if decoded.MotionConfig.Points3Star != 10 {
		t.Errorf("Points3Star mismatch: expected 10, got %d", decoded.MotionConfig.Points3Star)
	}
}

// TestDoneMotionCard_WithMotionConfig is an integration test that verifies the full
// DoneMotionCard flow with a custom MotionConfig: the team receives the configured
// points (not the defaults) and the return value matches the configuration.
func TestDoneMotionCard_WithMotionConfig(t *testing.T) {
	tests := []struct {
		desc           string
		difficulty     int
		config         *MotionConfig
		expectedPoints int
	}{
		{
			desc:           "1★ config(2/6/10) → 2pts",
			difficulty:     1,
			config:         &MotionConfig{Points1Star: 2, Points2Star: 6, Points3Star: 10},
			expectedPoints: 2,
		},
		{
			desc:           "2★ config(2/6/10) → 6pts",
			difficulty:     2,
			config:         &MotionConfig{Points1Star: 2, Points2Star: 6, Points3Star: 10},
			expectedPoints: 6,
		},
		{
			desc:           "3★ config(2/6/10) → 10pts",
			difficulty:     3,
			config:         &MotionConfig{Points1Star: 2, Points2Star: 6, Points3Star: 10},
			expectedPoints: 10,
		},
		{
			desc:           "3★ config(0/0/0) → fallback 5pts",
			difficulty:     3,
			config:         &MotionConfig{Points1Star: 0, Points2Star: 0, Points3Star: 0},
			expectedPoints: 5,
		},
		{
			desc:           "1★ partial config(5/0/0) → 5pts (custom 1★)",
			difficulty:     1,
			config:         &MotionConfig{Points1Star: 5, Points2Star: 0, Points3Star: 0},
			expectedPoints: 5,
		},
		{
			desc:           "2★ partial config(5/0/0) → 3pts (fallback 2★)",
			difficulty:     2,
			config:         &MotionConfig{Points1Star: 5, Points2Star: 0, Points3Star: 0},
			expectedPoints: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			e := NewEngine()
			e.SetTeams(map[string]*Team{
				"blue": {Name: "Team Blue", Color: []int{0, 0, 255}},
			})
			q := singleCardMotion("mq1", "mc-1", tt.difficulty)
			q.MotionConfig = tt.config
			startMEMOTION(t, e, "mq1", q)
			defer e.Stop()

			_ = e.SelectMotionCard("mc-1")
			_ = e.FlipMotionCard()
			_ = e.RevealMotionCard()

			points, _, err := e.DoneMotionCard("mc-1", "blue")
			if err != nil {
				t.Fatalf("DoneMotionCard failed: %v", err)
			}
			if points != tt.expectedPoints {
				t.Errorf("DoneMotionCard returned %d points, want %d", points, tt.expectedPoints)
			}

			// Verify the team's score was updated with the configured points
			team := e.GetTeam("blue")
			if team == nil {
				t.Fatal("Team 'blue' should exist")
			}
			if team.TeamPoints != tt.expectedPoints {
				t.Errorf("team.TeamPoints = %d, want %d", team.TeamPoints, tt.expectedPoints)
			}
		})
	}
}

// ============================================================================
// Secret Mode — MEMORIZE subphase tests (v5.5.0 #76)
// ============================================================================

// TestInitMotionState_SecretMode_SubPhaseMemorize verifies that initMotionStateUnsafe
// sets MotionSubPhase to "MEMORIZE" when MotionMemorizeDuration > 0.
func TestInitMotionState_SecretMode_SubPhaseMemorize(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("q1", defaultMotionCards(), "SOLO")
	q.MotionMemorizeDuration = 10
	e.Ready("q1", q)

	// InitMotionState acquires the lock and calls initMotionStateUnsafe
	e.InitMotionState()

	if e.state.MotionSubPhase != "MEMORIZE" {
		t.Errorf("expected MotionSubPhase=MEMORIZE when MotionMemorizeDuration>0, got %q", e.state.MotionSubPhase)
	}
	// Cards should still be initialized
	if len(e.state.MotionCardStates) != len(defaultMotionCards()) {
		t.Errorf("expected %d card states, got %d", len(defaultMotionCards()), len(e.state.MotionCardStates))
	}
}

// TestInitMotionState_SecretMode_CurrentTimeSetBeforeBroadcast verifies that
// initMotionStateUnsafe initialises CurrentTime = MotionMemorizeDuration synchronously,
// so the first broadcastUpdate (fired right after initMotionStateUnsafe but BEFORE
// StartMotionMemorizeTimer) already carries the correct countdown value.
// Regression guard: prevents residual CurrentTime flash on TV at MEMORIZE start (#76).
func TestInitMotionState_SecretMode_CurrentTimeSetBeforeBroadcast(t *testing.T) {
	const duration = 10

	e := NewEngine()
	q := makeMotionQuestion("q1", defaultMotionCards(), "SOLO")
	q.MotionMemorizeDuration = duration
	e.Ready("q1", q)

	// Simulate residual state from a previous question (e.g., CurrentTime=30)
	e.mu.Lock()
	e.state.CurrentTime = 30
	e.state.Delay = 30
	e.mu.Unlock()

	// initMotionStateUnsafe is what actualStart/StartImmediate call under lock
	// before releasing the lock and calling callback(PhaseStarted).
	e.InitMotionState()

	// The first broadcast would read state at this point — verify CurrentTime == duration.
	e.mu.RLock()
	currentTime := e.state.CurrentTime
	delay := e.state.Delay
	subphase := e.state.MotionSubPhase
	e.mu.RUnlock()

	if subphase != "MEMORIZE" {
		t.Errorf("expected MotionSubPhase=MEMORIZE, got %q", subphase)
	}
	if currentTime != duration {
		t.Errorf("expected CurrentTime=%d before first broadcast, got %d (residual flash bug)", duration, currentTime)
	}
	if delay != duration {
		t.Errorf("expected Delay=%d before first broadcast, got %d", duration, delay)
	}
}

// TestInitMotionState_StandardMode_CurrentTimeCleared verifies that initMotionStateUnsafe
// resets CurrentTime to 0 for standard mode (MotionMemorizeDuration == 0),
// preventing residual values from a previous question from being broadcast.
func TestInitMotionState_StandardMode_CurrentTimeCleared(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("q1", defaultMotionCards(), "SOLO")
	// MotionMemorizeDuration == 0 → standard GRID mode
	e.Ready("q1", q)

	// Simulate residual CurrentTime from a previous question
	e.mu.Lock()
	e.state.CurrentTime = 30
	e.mu.Unlock()

	e.InitMotionState()

	e.mu.RLock()
	currentTime := e.state.CurrentTime
	subphase := e.state.MotionSubPhase
	e.mu.RUnlock()

	if subphase != "GRID" {
		t.Errorf("expected MotionSubPhase=GRID, got %q", subphase)
	}
	if currentTime != 0 {
		t.Errorf("expected CurrentTime=0 for standard mode, got %d (residual flash bug)", currentTime)
	}
}

// TestInitMotionState_StandardMode_SubPhaseGrid verifies that initMotionStateUnsafe
// sets MotionSubPhase to "GRID" when MotionMemorizeDuration == 0 (regression test).
func TestInitMotionState_StandardMode_SubPhaseGrid(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("q1", defaultMotionCards(), "SOLO")
	// MotionMemorizeDuration defaults to 0 — standard mode
	e.Ready("q1", q)

	e.InitMotionState()

	if e.state.MotionSubPhase != "GRID" {
		t.Errorf("expected MotionSubPhase=GRID when MotionMemorizeDuration==0, got %q", e.state.MotionSubPhase)
	}
}

// TestSelectMotionCard_ErrorWhenSubPhaseMemorize verifies that card selection is
// rejected during the MEMORIZE subphase with NOT_IN_GRID_SUBPHASE.
func TestSelectMotionCard_ErrorWhenSubPhaseMemorize(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("q1", defaultMotionCards(), "SOLO")
	q.MotionMemorizeDuration = 10
	e.Ready("q1", q)
	e.StartImmediate(0)
	// InitMotionState resets subphase based on MotionMemorizeDuration → "MEMORIZE"
	e.InitMotionState()

	err := e.SelectMotionCard("mc-1")
	if err == nil {
		t.Fatal("expected error when selecting card in MEMORIZE subphase, got nil")
	}

	motionErr, ok := err.(*MotionError)
	if !ok {
		t.Fatalf("expected *MotionError, got %T: %v", err, err)
	}
	if motionErr.Reason != "NOT_IN_GRID_SUBPHASE" {
		t.Errorf("expected reason NOT_IN_GRID_SUBPHASE, got %q", motionErr.Reason)
	}
}

// TestStartMotionMemorizeTimer_ZeroDuration_NoOp verifies that StartMotionMemorizeTimer
// with duration=0 is a no-op (subphase unchanged, no goroutine started).
func TestStartMotionMemorizeTimer_ZeroDuration_NoOp(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("q1", defaultMotionCards(), "SOLO")
	e.Ready("q1", q)
	e.StartImmediate(0)
	e.InitMotionState() // subphase = GRID (MotionMemorizeDuration == 0)

	e.StartMotionMemorizeTimer(0) // no-op

	if e.state.MotionSubPhase != "GRID" {
		t.Errorf("expected MotionSubPhase=GRID after no-op timer call, got %q", e.state.MotionSubPhase)
	}
}

// TestStartMotionMemorizeTimer_TransitionsToGrid verifies that after StartMotionMemorizeTimer(1),
// MotionSubPhase transitions from "MEMORIZE" to "GRID" after ~1 second.
func TestStartMotionMemorizeTimer_TransitionsToGrid(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("q1", defaultMotionCards(), "SOLO")
	q.MotionMemorizeDuration = 1
	e.Ready("q1", q)

	// Manually set engine to STARTED + MEMORIZE to avoid startTimer side effects
	e.mu.Lock()
	e.state.Phase = PhaseStarted
	e.state.MotionSubPhase = "MEMORIZE"
	e.mu.Unlock()

	e.StartMotionMemorizeTimer(1)

	// Wait for timer to fire (1s tick + buffer)
	time.Sleep(1500 * time.Millisecond)

	e.mu.RLock()
	subphase := e.state.MotionSubPhase
	currentTime := e.state.CurrentTime
	e.mu.RUnlock()

	if subphase != "GRID" {
		t.Errorf("expected MotionSubPhase=GRID after timer expiry, got %q", subphase)
	}
	if currentTime != 0 {
		t.Errorf("expected CurrentTime=0 after timer expiry, got %d", currentTime)
	}
}

// TestStartMotionMemorizeTimer_TransitionsToGrid_MotionSelectedCleared verifies that
// the automatic MEMORIZE→GRID transition clears MotionSelected (prevents stale selection).
func TestStartMotionMemorizeTimer_TransitionsToGrid_MotionSelectedCleared(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("q1", defaultMotionCards(), "SOLO")
	q.MotionMemorizeDuration = 1
	e.Ready("q1", q)

	// Manually set engine state — same pattern as TestStartMotionMemorizeTimer_TransitionsToGrid
	e.mu.Lock()
	e.state.Phase = PhaseStarted
	e.state.MotionSubPhase = "MEMORIZE"
	e.state.MotionSelected = "mc-1" // defensive: ensure field is cleared by transition
	e.mu.Unlock()

	e.StartMotionMemorizeTimer(1)

	// Wait for timer to fire (1s tick + buffer)
	time.Sleep(1500 * time.Millisecond)

	e.mu.RLock()
	subphase := e.state.MotionSubPhase
	selected := e.state.MotionSelected
	e.mu.RUnlock()

	if subphase != "GRID" {
		t.Errorf("expected MotionSubPhase=GRID after timer expiry, got %q", subphase)
	}
	if selected != "" {
		t.Errorf("expected MotionSelected=\"\" after MEMORIZE→GRID transition, got %q", selected)
	}
}

// TestStopGame_DuringMEMORIZE_NoPanic verifies that calling Stop() while the MEMORIZE
// timer goroutine is active terminates the goroutine cleanly without panic.
// Regression guard: goroutine must not continue writing to state after Stop().
func TestStopGame_DuringMEMORIZE_NoPanic(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("q1", defaultMotionCards(), "SOLO")
	q.MotionMemorizeDuration = 5 // long timer — would not expire during test
	e.Ready("q1", q)

	// StartImmediate sets MotionSubPhase=MEMORIZE and launches the goroutine
	e.StartImmediate(0)

	// Immediately stop — stopCh close must signal goroutine exit
	e.Stop()

	// Brief sleep to let goroutine observe the closed channel
	time.Sleep(100 * time.Millisecond)

	e.mu.RLock()
	phase := e.state.Phase
	subphase := e.state.MotionSubPhase
	e.mu.RUnlock()

	if phase != PhaseStopped {
		t.Errorf("expected Phase=STOPPED after Stop(), got %q", phase)
	}
	// Timer was cut short at 5s — subphase must NOT have transitioned to GRID
	if subphase == "GRID" {
		t.Errorf("MotionSubPhase must not transition to GRID when Stop() is called before timer expiry, got %q", subphase)
	}

	// Double-stop must not panic (idempotent)
	e.Stop()
}

// TestStartImmediate_SecretMode_StartsMemorizeTimer verifies the end-to-end integration:
// StartImmediate() with MotionMemorizeDuration > 0 sets MotionSubPhase=MEMORIZE immediately,
// then automatically transitions to GRID after the timer expires — same as actualStart().
func TestStartImmediate_SecretMode_StartsMemorizeTimer(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("q1", defaultMotionCards(), "SOLO")
	q.MotionMemorizeDuration = 1
	e.Ready("q1", q)

	e.StartImmediate(0)

	// Immediately after StartImmediate: subphase must be MEMORIZE
	e.mu.RLock()
	immediateSubphase := e.state.MotionSubPhase
	e.mu.RUnlock()

	if immediateSubphase != "MEMORIZE" {
		t.Errorf("expected MotionSubPhase=MEMORIZE immediately after StartImmediate, got %q", immediateSubphase)
	}

	// After ~1.5s: timer fires, automatic transition to GRID
	time.Sleep(1500 * time.Millisecond)

	e.mu.RLock()
	subphase := e.state.MotionSubPhase
	currentTime := e.state.CurrentTime
	e.mu.RUnlock()

	if subphase != "GRID" {
		t.Errorf("expected MotionSubPhase=GRID after timer expiry, got %q", subphase)
	}
	if currentTime != 0 {
		t.Errorf("expected CurrentTime=0 after timer expiry, got %d", currentTime)
	}

	e.Stop()
}
