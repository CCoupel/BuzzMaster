// Tests for #187 — MEMORY imbriquée dans une carte MEMOTION.
// Run: go test ./internal/game/... -run TestFlipMotionMemoryCard\|TestClearMotionMemoryFlippedCards\|TestDoneMotionCard_MemoryCard -v
//
// Plan de référence : _work/reports/plan-memotion-v710-memory-20260824-154844.md
// (tâches 3-8), -v2-20260824-161449.md (§2, FLIP_MEMORY_CARD),
// -value-20260824-163512.md (§1, STARS_PRORATA).

package game

import (
	"fmt"
	"testing"
)

// memoryMotionCard builds a single MEMOTION card of TYPE=MEMORY with
// nPairs pairs and an optional flip delay (seconds; 0 = use the engine
// default of 3000ms).
func memoryMotionCard(cardID string, difficulty, nPairs int, flipDelaySeconds float64) MotionCard {
	pairs := make([]MemoryPair, nPairs)
	for i := 0; i < nPairs; i++ {
		pairs[i] = MemoryPair{
			ID:    i + 1,
			Card1: MemoryCard{Text: "A"},
			Card2: MemoryCard{Text: "A"},
		}
	}
	card := MotionCard{
		ID:         cardID,
		Type:       QuestionTypeMemory,
		RectoTheme: "T",
		Difficulty: difficulty,
		TypedContent: TypedContent{
			MemoryPairs: pairs,
		},
	}
	if flipDelaySeconds > 0 {
		card.MemoryConfig = &MemoryConfig{FlipDelay: flipDelaySeconds}
	}
	return card
}

// cardIDFor returns the flip-target card ID ("pairID-cardNum") for the
// first or second physical card of pairID.
func cardIDFor(pairID, cardNum int) string {
	return fmt.Sprintf("%d-%d", pairID, cardNum)
}

// startMEMOTIONAtMemoryCardQuestion prepares an engine with a single MEMORY
// card selected and flipped to QUESTION subphase — the only subphase in
// which FlipMotionMemoryCard accepts a flip (HostContext.Playable, contract
// §4).
func startMEMOTIONAtMemoryCardQuestion(t *testing.T, e *Engine, card MotionCard) {
	t.Helper()
	q := makeMotionQuestion("mq-memory", []MotionCard{card}, "SOLO")
	startMEMOTION(t, e, "mq-memory", q)
	if err := e.SelectMotionCard(card.ID); err != nil {
		t.Fatalf("SelectMotionCard(%s): %v", card.ID, err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard(): %v", err)
	}
}

// ============================================================================
// FlipMotionMemoryCard
// ============================================================================

func TestFlipMotionMemoryCard_Match_UpdatesCardScopedStateOnly(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 2, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	isMatch, shouldFlipBack, _, isComplete := e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 1))
	if isMatch || shouldFlipBack || isComplete {
		t.Fatalf("first flip: got isMatch=%v shouldFlipBack=%v isComplete=%v, want all false", isMatch, shouldFlipBack, isComplete)
	}
	isMatch, shouldFlipBack, _, isComplete = e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 2))
	if !isMatch || shouldFlipBack || isComplete {
		t.Fatalf("second flip (match, grid not complete — 1/2 pairs): got isMatch=%v shouldFlipBack=%v isComplete=%v, want isMatch=true, rest false",
			isMatch, shouldFlipBack, isComplete)
	}

	state := e.GetState()
	matched := motionActiveMemoryMatchedPairs(state.MotionActive.State)
	if len(matched) != 1 || matched[0] != 1 {
		t.Errorf("MEMOTION_ACTIVE.STATE.MEMORY_MATCHED_PAIRS = %v, want [1]", matched)
	}

	// §5.4 — the card-scoped state must NEVER touch the question-scoped
	// Memory* fields, which stay at their zero value for a MEMOTION round.
	if len(state.MemoryMatchedPairs) != 0 {
		t.Errorf("question-scoped MemoryMatchedPairs = %v, want empty — card state must not leak into it", state.MemoryMatchedPairs)
	}
}

func TestFlipMotionMemoryCard_Complete_AllPairsFound(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 1, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	_, _, _, isComplete := e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 1))
	if isComplete {
		t.Fatalf("first card of the only pair: isComplete=true too early")
	}
	_, _, _, isComplete = e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 2))
	if !isComplete {
		t.Fatalf("last pair of the grid found: isComplete=false, want true")
	}

	// Engine-level responsibility ends at reporting isComplete=true — it
	// must NOT itself touch Phase or MotionSubPhase. Plan §5 tâche 4: the
	// caller (main.go) is the one that hands control to REVEAL, and must
	// never call Stop().
	state := e.GetState()
	if state.Phase != PhaseStarted {
		t.Errorf("Phase = %s, want STARTED — FlipMotionMemoryCard must never stop the game itself", state.Phase)
	}
	if state.MotionSubPhase != MotionSubPhaseQuestion {
		t.Errorf("MotionSubPhase = %s, want QUESTION — FlipMotionMemoryCard must not itself transition sub-phase", state.MotionSubPhase)
	}
}

func TestFlipMotionMemoryCard_NoMatch_SchedulesFlipBackFromCardOwnConfig(t *testing.T) {
	e := NewEngine()
	// Two distinct pairs guarantees the 1st card of pair 1 and the 1st card
	// of pair 2 don't match.
	card := memoryMotionCard("mc-mem", 1, 2, 1.5) // FLIP_DELAY 1.5s → 1500ms
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 1))
	isMatch, shouldFlipBack, flipDelay, isComplete := e.FlipMotionMemoryCard("mc-mem", cardIDFor(2, 1))
	if isMatch || !shouldFlipBack || isComplete {
		t.Fatalf("mismatch: got isMatch=%v shouldFlipBack=%v isComplete=%v, want isMatch=false shouldFlipBack=true isComplete=false",
			isMatch, shouldFlipBack, isComplete)
	}
	if flipDelay != 1500 {
		t.Errorf("flipDelay = %d, want 1500 (card's own MEMORY_CONFIG.FLIP_DELAY, not the question-scoped one)", flipDelay)
	}

	state := e.GetState()
	if n := motionActiveMemoryErrors(state.MotionActive.State); n != 1 {
		t.Errorf("MEMOTION_ACTIVE.STATE.MEMORY_ERRORS = %d, want 1", n)
	}
	flipped := motionActiveMemoryFlippedCards(state.MotionActive.State)
	if len(flipped) != 2 {
		t.Errorf("MEMOTION_ACTIVE.STATE.MEMORY_FLIPPED_CARDS = %v, want 2 entries (cleared only by ClearMotionMemoryFlippedCards)", flipped)
	}
}

func TestFlipMotionMemoryCard_WrongMotionCardID_Ignored(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 2, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	isMatch, shouldFlipBack, flipDelay, isComplete := e.FlipMotionMemoryCard("some-other-card", cardIDFor(1, 1))
	if isMatch || shouldFlipBack || flipDelay != 0 || isComplete {
		t.Fatalf("flip against a non-active card ID: got a non-zero result, want a total no-op")
	}
	state := e.GetState()
	if flipped := motionActiveMemoryFlippedCards(state.MotionActive.State); len(flipped) != 0 {
		t.Errorf("MEMORY_FLIPPED_CARDS = %v, want empty — a mismatched MOTION_CARD_ID must not mutate the active card's state", flipped)
	}
}

func TestFlipMotionMemoryCard_NonMemoryCard_Ignored(t *testing.T) {
	e := NewEngine()
	// SPEEDY card selected+flipped — FlipMotionMemoryCard must refuse to
	// act on it even if somehow called (defense in depth; main.go only
	// calls this for CardScope-carrying payloads, but the guard belongs to
	// the engine, not the caller).
	card := MotionCard{ID: "mc-speedy", RectoTheme: "T", Difficulty: 1}
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	isMatch, shouldFlipBack, _, isComplete := e.FlipMotionMemoryCard("mc-speedy", cardIDFor(1, 1))
	if isMatch || shouldFlipBack || isComplete {
		t.Fatalf("flip against a non-MEMORY active card: got a non-zero result, want a total no-op")
	}
}

func TestFlipMotionMemoryCard_NeverRotatesMemoryTeam(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"A": {Name: "A"}, "B": {Name: "B"}})
	card := memoryMotionCard("mc-mem", 1, 1, 0)
	q := makeMotionQuestion("mq-memory", []MotionCard{card}, "SOLO")
	e.Ready("mq-memory", q)
	if err := e.SetMotionParticipatingTeams([]string{"A", "B"}); err != nil {
		t.Fatalf("SetMotionParticipatingTeams: %v", err)
	}
	e.StartImmediate(0)
	e.InitMotionState()
	if err := e.SelectMotionCard(card.ID); err != nil {
		t.Fatalf("SelectMotionCard: %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard: %v", err)
	}
	defer e.Stop()

	before := e.GetState().MotionCurrentTeam

	// Complete the grid — a full MEMORY round (question host) would rotate
	// via ClearMemoryFlippedCards/rotateToNextTeam. A card must not.
	e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 1))
	e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 2))

	after := e.GetState().MotionCurrentTeam
	if before != after {
		t.Errorf("MotionCurrentTeam changed from %q to %q — contract §6.3: MEMORY rotation must never fire inside a card", before, after)
	}
}

// ============================================================================
// ClearMotionMemoryFlippedCards
// ============================================================================

func TestClearMotionMemoryFlippedCards_ClearsWhenStillActive(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 2, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 1))
	e.FlipMotionMemoryCard("mc-mem", cardIDFor(2, 1)) // mismatch, 2 flipped

	e.ClearMotionMemoryFlippedCards("mc-mem")

	flipped := motionActiveMemoryFlippedCards(e.GetState().MotionActive.State)
	if len(flipped) != 0 {
		t.Errorf("MEMORY_FLIPPED_CARDS = %v after clear, want empty", flipped)
	}
}

// TestClearMotionMemoryFlippedCards_NoOpIfCardChanged is the direct test of
// plan Risque R2: a goroutine scheduled for card A must never blank out
// card B's state if the animateur moved on to B before the flip-back delay
// elapsed.
func TestClearMotionMemoryFlippedCards_NoOpIfCardChanged(t *testing.T) {
	e := NewEngine()
	cardA := memoryMotionCard("mc-a", 1, 2, 0)
	cardB := memoryMotionCard("mc-b", 1, 2, 0)
	q := makeMotionQuestion("mq-memory", []MotionCard{cardA, cardB}, "SOLO")
	startMEMOTION(t, e, "mq-memory", q)
	defer e.Stop()

	if err := e.SelectMotionCard("mc-a"); err != nil {
		t.Fatalf("SelectMotionCard(mc-a): %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard: %v", err)
	}
	e.FlipMotionMemoryCard("mc-a", cardIDFor(1, 1))
	e.FlipMotionMemoryCard("mc-a", cardIDFor(2, 1)) // mismatch, 2 flipped on card A

	// Animateur cancels card A (back to GRID) and moves on to card B —
	// simulates the delay elapsing after the active card changed.
	if _, _, err := e.DoneMotionCard("mc-a", "", 0); err != nil {
		t.Fatalf("DoneMotionCard(cancel mc-a): %v", err)
	}
	if err := e.SelectMotionCard("mc-b"); err != nil {
		t.Fatalf("SelectMotionCard(mc-b): %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard: %v", err)
	}
	e.FlipMotionMemoryCard("mc-b", cardIDFor(1, 1)) // 1 flipped on card B

	// The goroutine scheduled for A's flip-back finally wakes up — must be
	// a no-op, must NOT touch B's freshly-flipped card.
	e.ClearMotionMemoryFlippedCards("mc-a")

	flipped := motionActiveMemoryFlippedCards(e.GetState().MotionActive.State)
	if len(flipped) != 1 || flipped[0] != cardIDFor(1, 1) {
		t.Errorf("card B's MEMORY_FLIPPED_CARDS = %v after a stale clear scheduled for card A, want [%s] untouched",
			flipped, cardIDFor(1, 1))
	}
}

// ============================================================================
// DoneMotionCard — STARS_PRORATA, server-derived Units/UnitsTotal (#187 §9.3)
// ============================================================================

func TestDoneMotionCard_MemoryCard_StarsProrata_ServerDerivesUnits(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	card := memoryMotionCard("mc-mem", 1, 8, 0) // 8 pairs
	card.PointsRule = &PointsRule{Mode: PointsRuleModeStarsProrata}
	q := makeMotionQuestion("mq-memory", []MotionCard{card}, "SOLO")
	q.MotionConfig = &MotionConfig{Points1Star: 5} // "5 points / 8 pairs" — the mandatory named case
	startMEMOTION(t, e, "mq-memory", q)
	defer e.Stop()

	if err := e.SelectMotionCard("mc-mem"); err != nil {
		t.Fatalf("SelectMotionCard: %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard: %v", err)
	}

	// Find 4 of the 8 pairs.
	for pairID := 1; pairID <= 4; pairID++ {
		e.FlipMotionMemoryCard("mc-mem", cardIDFor(pairID, 1))
		e.FlipMotionMemoryCard("mc-mem", cardIDFor(pairID, 2))
	}
	if err := e.RevealMotionCard(); err != nil {
		t.Fatalf("RevealMotionCard: %v", err)
	}

	// Client sends a bogus UNITS=999 — contract §9.3: must be ignored for a
	// MEMORY card. The server must derive units=4 (found) / unitsTotal=8
	// (card's own pair count) on its own.
	points, _, err := e.DoneMotionCard("mc-mem", "red", 999)
	if err != nil {
		t.Fatalf("DoneMotionCard: %v", err)
	}
	if points != 2 {
		t.Errorf("points awarded = %d, want 2 (5×4/8, client UNITS=999 must be ignored)", points)
	}
	team := e.GetTeamsAndBumpers().Teams["red"]
	if team.TeamPoints != 2 {
		t.Errorf("team.TeamPoints = %d, want 2", team.TeamPoints)
	}
}

func TestDoneMotionCard_MemoryCard_CompleteGridAwardsFullValue(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	card := memoryMotionCard("mc-mem", 1, 8, 0)
	card.PointsRule = &PointsRule{Mode: PointsRuleModeStarsProrata}
	q := makeMotionQuestion("mq-memory", []MotionCard{card}, "SOLO")
	q.MotionConfig = &MotionConfig{Points1Star: 5}
	startMEMOTION(t, e, "mq-memory", q)
	defer e.Stop()

	if err := e.SelectMotionCard("mc-mem"); err != nil {
		t.Fatalf("SelectMotionCard: %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard: %v", err)
	}
	var isComplete bool
	for pairID := 1; pairID <= 8; pairID++ {
		e.FlipMotionMemoryCard("mc-mem", cardIDFor(pairID, 1))
		_, _, _, isComplete = e.FlipMotionMemoryCard("mc-mem", cardIDFor(pairID, 2))
	}
	if !isComplete {
		t.Fatalf("grid should report isComplete=true once all 8 pairs are found")
	}
	if err := e.RevealMotionCard(); err != nil {
		t.Fatalf("RevealMotionCard: %v", err)
	}

	points, _, err := e.DoneMotionCard("mc-mem", "red", 1)
	if err != nil {
		t.Fatalf("DoneMotionCard: %v", err)
	}
	if points != 5 {
		t.Errorf("points awarded = %d, want 5 — a complete grid must reward the card's exact nominal value, no rounding loss", points)
	}
}
