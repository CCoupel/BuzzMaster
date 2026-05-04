// Tests for MEMOTION game mode (v5.0.0) — Issues #71/#72
//
// Run: go test ./internal/game/... -v -run TestInitMotionState\|TestSelect\|TestReveal\|TestDoneMotion\|TestSetMotion\|TestProcessButtonPress_Ignores\|TestQuestionType\|TestMotionCard\|TestInitGame_Reset
//
// NOTE: Ready() calls initMotionStateUnsafe() for MEMOTION questions, so card states
// are initialized before StartImmediate(). Tests using setupMotionEngine() have cards
// in "UNPLAYED" state after setup, ready for SelectMotionCard().

package game

import (
	"fmt"
	"testing"
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
