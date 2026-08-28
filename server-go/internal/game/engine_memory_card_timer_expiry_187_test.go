// Tests for #187, MEMORY card timer expiry — history across three cycles:
//
//   - QUALIF v7.1.0.1 bug report: a MEMOTION card of TYPE=MEMORY kept
//     accepting FLIP_MEMORY_CARD after its own per-card timer reached 0.
//   - Cycle 3 fix: auto-reveal the card (MotionSubPhase → REVEAL) at
//     expiry, same as a completed grid. Correct server-side, but its own
//     notification (OnTimerTick → ActionUpdateTimer) never reached the
//     frontend's live MEMOTION rendering (code-review 20260825-200416).
//   - Cycle 4 (current): the auto-reveal itself is REVERTED, per
//     plan-memotion-v710-memory-reveal-v2-20260824's user-validated
//     asymmetry — expiry on an INCOMPLETE grid must leave the card in
//     QUESTION, closed to further flips (motionCardRoundClosed) but
//     requiring an explicit animateur MEMOTION_REVEAL to actually uncover
//     it. Only the completed-grid exit (main.go, unchanged) still
//     auto-reveals — see that function's own doc comment for why the two
//     exits are deliberately asymmetric, not a leftover inconsistency.
//
// Run: go test ./internal/game/... -run TestProcessMotionCardTick_MemoryCard\|TestProcessMotionCardTick_.*StaysInQuestion\|TestFlipMotionMemoryCard_.*Expiry\|TestFlipMotionMemoryCard_NoConfiguredTimer\|TestRevealMotionCard_AfterTimerExpiry\|TestMemoryCard_CompleteGridBeforeExpiry -v
package game

import "testing"

// TestProcessMotionCardTick_MemoryCard_ExpiryLeavesCardInQuestion is the
// cycle 4 assertion (inverted from cycle 3): driving the real per-tick code
// path down to CURRENT_TIME=0 on an active, incomplete MEMORY card must
// NOT move MotionSubPhase — it stays QUESTION, Phase stays STARTED, and the
// card is neither marked REVEALED nor DONE. The round is closed
// (motionCardRoundClosed) without any sub-phase transition — that's what
// gates FlipMotionMemoryCard now (B3), not the sub-phase.
func TestProcessMotionCardTick_MemoryCard_ExpiryLeavesCardInQuestion(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 4, 0) // 4 pairs, deliberately incomplete
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	// Only 1 of 4 pairs found — the card is incomplete when the timer
	// expires, exactly the QUALIF scenario.
	e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 1))
	e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 2))

	e.mu.Lock()
	e.state.CurrentTime = 1
	e.state.Delay = 30
	e.mu.Unlock()

	result := e.processMotionCardTick()
	if result.currentTime != 0 {
		t.Fatalf("setup invalide : CurrentTime après le tick = %d, attendu 0", result.currentTime)
	}

	state := e.GetState()
	if state.Phase != PhaseStarted {
		t.Errorf("Phase = %s, want STARTED — expiry must never Stop() the MEMOTION round", state.Phase)
	}
	if state.MotionSubPhase != MotionSubPhaseQuestion {
		t.Errorf("MotionSubPhase = %s, want QUESTION unchanged at expiry — cycle 4 reverted the cycle 3 auto-reveal (deliberate asymmetry with the completed-grid exit)", state.MotionSubPhase)
	}
	if got := state.MotionCardStates["mc-mem"]; got != MotionCardStateQuestion {
		t.Errorf("MotionCardStates[mc-mem] = %s, want QUESTION unchanged", got)
	}
	if !e.motionCardRoundClosed {
		t.Errorf("motionCardRoundClosed = false after expiry, want true — the round must close even though the sub-phase doesn't move")
	}
}

// TestFlipMotionMemoryCard_IgnoredAfterTimerExpiry is the end-to-end
// regression test tying the two mechanisms together exactly as a real game
// session would: drive the timer to expiry via processMotionCardTick, then
// attempt a flip on the now-expired card — it must be a complete no-op.
// Assertion unchanged since the QUALIF fix; only the CAUSE changed (B3's
// motionCardRoundClosed gate, not a sub-phase transition).
func TestFlipMotionMemoryCard_IgnoredAfterTimerExpiry(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 4, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 1))
	e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 2)) // 1/4 pairs found

	e.mu.Lock()
	e.state.CurrentTime = 1
	e.state.Delay = 30
	e.mu.Unlock()
	e.processMotionCardTick() // → CURRENT_TIME=0, round closes

	beforeMatched := motionActiveMemoryMatchedPairs(e.GetState().MotionActive.State)

	// Attempt to flip a THIRD pair after expiry — must be entirely
	// rejected: no match, no flip-back scheduling, no state mutation.
	isMatch, shouldFlipBack, flipDelay, isComplete := e.FlipMotionMemoryCard("mc-mem", cardIDFor(2, 1))
	if isMatch || shouldFlipBack || flipDelay != 0 || isComplete {
		t.Fatalf("flip attempted after timer expiry: got a non-zero result (isMatch=%v shouldFlipBack=%v flipDelay=%d isComplete=%v), want a total no-op",
			isMatch, shouldFlipBack, flipDelay, isComplete)
	}

	afterMatched := motionActiveMemoryMatchedPairs(e.GetState().MotionActive.State)
	if len(afterMatched) != len(beforeMatched) {
		t.Errorf("MEMORY_MATCHED_PAIRS changed after an expired-timer flip attempt: before=%v after=%v", beforeMatched, afterMatched)
	}
	flipped := motionActiveMemoryFlippedCards(e.GetState().MotionActive.State)
	if len(flipped) != 0 {
		t.Errorf("MEMORY_FLIPPED_CARDS = %v after an expired-timer flip attempt, want empty — the attempted flip must never register", flipped)
	}
}

// TestProcessMotionCardTick_MemoryCard_StaysOpenBeforeExpiry is the
// negative counterpart: a tick that does NOT reach CURRENT_TIME<=0 must
// leave the round open and the card fully playable.
func TestProcessMotionCardTick_MemoryCard_StaysOpenBeforeExpiry(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 4, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	e.mu.Lock()
	e.state.CurrentTime = 10
	e.state.Delay = 30
	e.mu.Unlock()

	e.processMotionCardTick() // → CurrentTime=9, still running
	if e.motionCardRoundClosed {
		t.Fatalf("motionCardRoundClosed = true at CurrentTime=9, want false — must only close at expiry")
	}
	if got := e.GetState().MotionSubPhase; got != MotionSubPhaseQuestion {
		t.Errorf("MotionSubPhase = %s, want QUESTION (timer not yet expired)", got)
	}

	// The card must still accept a flip.
	e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 1))
	flipped := motionActiveMemoryFlippedCards(e.GetState().MotionActive.State)
	if len(flipped) != 1 {
		t.Errorf("MEMORY_FLIPPED_CARDS = %v after a flip before expiry, want 1 entry — the card must still be playable", flipped)
	}
}

// TestFlipMotionMemoryCard_NoConfiguredTimer_StaysPlayableIndefinitely is
// the mandatory trap test named explicitly in the plan (B3): a MEMOTION
// question with no configured timer (Question.TIME<=0) never starts one —
// StartMotionCardTimer's own delay<=0 guard returns before touching
// anything. CURRENT_TIME therefore stays 0 for the entire card, exactly
// the same VALUE it would show at a real expiry. A guard keyed on
// CURRENT_TIME==0 alone would make such a card permanently unplayable,
// silently, with no failing test — this is that test. Reproduced here by
// simply never calling StartMotionCardTimer/processMotionCardTick at all
// (the real no-timer path never ticks), and driving several flips through.
func TestFlipMotionMemoryCard_NoConfiguredTimer_StaysPlayableIndefinitely(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 4, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card) // CurrentTime left at 0 by StartImmediate(0) — no timer ever started, matching Question.TIME="0"
	defer e.Stop()

	if e.GetState().CurrentTime != 0 {
		t.Fatalf("setup invalide : CurrentTime = %d, attendu 0 (aucun chrono démarré)", e.GetState().CurrentTime)
	}
	if e.motionCardRoundClosed {
		t.Fatalf("motionCardRoundClosed = true with no timer ever started — a card with no configured timer must never be considered closed")
	}

	// Find all 4 pairs — every single flip must be accepted, none refused
	// by a round-closed guard that mistook CURRENT_TIME==0 for expiry.
	var isComplete bool
	for pairID := 1; pairID <= 4; pairID++ {
		e.FlipMotionMemoryCard("mc-mem", cardIDFor(pairID, 1))
		_, _, _, isComplete = e.FlipMotionMemoryCard("mc-mem", cardIDFor(pairID, 2))
	}
	if !isComplete {
		t.Errorf("grid never reported complete — a flip was likely rejected by a spurious round-closed guard")
	}
	matched := motionActiveMemoryMatchedPairs(e.GetState().MotionActive.State)
	if len(matched) != 4 {
		t.Errorf("MEMORY_MATCHED_PAIRS = %v, want 4 pairs found — a timerless card must stay playable through the whole grid", matched)
	}
}

// TestRevealMotionCard_AfterTimerExpiry_RevealsIncompleteGrid proves the
// OTHER half of the cycle 4 design: once the round is closed by expiry, the
// explicit animateur gesture (MEMOTION_REVEAL → RevealMotionCard) must
// still succeed and actually uncover the grid — the round being "closed"
// (no more flips) is not the same as the card being "revealed" (still
// gated on MotionSubPhase==QUESTION, which expiry never touches).
func TestRevealMotionCard_AfterTimerExpiry_RevealsIncompleteGrid(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 4, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 1))
	e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 2)) // 1/4 pairs found

	e.mu.Lock()
	e.state.CurrentTime = 1
	e.state.Delay = 30
	e.mu.Unlock()
	e.processMotionCardTick() // → expiry, round closed, still QUESTION

	if err := e.RevealMotionCard(); err != nil {
		t.Fatalf("RevealMotionCard after expiry: %v — the animateur's explicit gesture must still work on a closed-but-not-revealed round", err)
	}
	state := e.GetState()
	if state.MotionSubPhase != MotionSubPhaseReveal {
		t.Errorf("MotionSubPhase = %s, want REVEAL after an explicit MEMOTION_REVEAL post-expiry", state.MotionSubPhase)
	}
	if got := state.MotionCardStates["mc-mem"]; got != MotionCardStateRevealed {
		t.Errorf("MotionCardStates[mc-mem] = %s, want REVEALED", got)
	}
}

// TestMemoryCard_CompleteGridBeforeExpiry_RevealsWithoutGesture locks in
// the OTHER exit of the plan's asymmetry (§1/§2 — unchanged by cycle 4,
// verified here as a non-regression): a grid completed BEFORE the timer
// expires reveals immediately, CURRENT_TIME reset to 0, with no animateur
// gesture at all. This is the exact sequence main.go's handleFlipMemoryCard
// (cardScoped, isComplete branch) performs — reproduced here at the engine
// level since that branch's own logic is untouched by this cycle.
func TestMemoryCard_CompleteGridBeforeExpiry_RevealsWithoutGesture(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 1, 0) // single pair — completes on the first match
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	e.mu.Lock()
	e.state.CurrentTime = 30
	e.state.Delay = 30
	e.mu.Unlock()

	e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 1))
	_, _, _, isComplete := e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 2))
	if !isComplete {
		t.Fatalf("setup invalide : grille non complète après la seule paire")
	}

	// main.go's exact sequence for this branch (cycle 2, unchanged):
	e.StopMotionCardTimer()
	if err := e.RevealMotionCard(); err != nil {
		t.Fatalf("RevealMotionCard on a completed grid: %v", err)
	}

	state := e.GetState()
	if state.MotionSubPhase != MotionSubPhaseReveal {
		t.Errorf("MotionSubPhase = %s, want REVEAL — a completed grid reveals immediately, no gesture required", state.MotionSubPhase)
	}
	if state.CurrentTime != 0 {
		t.Errorf("CurrentTime = %d, want 0 after StopMotionCardTimer on grid completion", state.CurrentTime)
	}
	if state.Phase != PhaseStarted {
		t.Errorf("Phase = %s, want STARTED — completing a card must never Stop() the MEMOTION round", state.Phase)
	}
}

// TestProcessMotionCardTick_QCMCard_StaysInQuestionOnExpiry is the general
// (not MEMORY-specific — B1 removed the type gate entirely) rule: no card
// type's sub-phase moves on plain timer expiry. QCM has no player input to
// block during QUESTION, so this was already true pre-#187 and stays true.
func TestProcessMotionCardTick_QCMCard_StaysInQuestionOnExpiry(t *testing.T) {
	e := NewEngine()
	card := MotionCard{
		ID: "mc-qcm", Type: QuestionTypeQCM, RectoTheme: "T", Difficulty: 1,
		TypedContent: TypedContent{
			QCMAnswers: &QCMAnswers{Red: "a", Green: "b", Yellow: "c", Blue: "d"},
			QCMCorrect: "GREEN",
		},
	}
	q := makeMotionQuestion("mq-qcm", []MotionCard{card}, "SOLO")
	startMEMOTION(t, e, "mq-qcm", q)
	if err := e.SelectMotionCard("mc-qcm"); err != nil {
		t.Fatalf("SelectMotionCard: %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard: %v", err)
	}
	defer e.Stop()

	e.mu.Lock()
	e.state.CurrentTime = 1
	e.state.Delay = 30
	e.mu.Unlock()

	result := e.processMotionCardTick()
	if result.currentTime != 0 {
		t.Fatalf("setup invalide : CurrentTime après le tick = %d, attendu 0", result.currentTime)
	}
	if got := e.GetState().MotionSubPhase; got != MotionSubPhaseQuestion {
		t.Errorf("MotionSubPhase = %s, want QUESTION unchanged for QCM at expiry", got)
	}
	// The round-closed flag itself is now generic (B3) — QCM has no
	// FlipMotionMemoryCard to gate, so this is inert for QCM, but assert
	// it anyway to document the (harmless) universal behavior.
	if !e.motionCardRoundClosed {
		t.Errorf("motionCardRoundClosed = false after expiry — expected true universally (harmless for QCM, no flip action reads it)")
	}
}

// TestProcessMotionCardTick_SpeedyCard_StaysInQuestionOnExpiry mirrors the
// QCM guard for SPEEDY (the default type, no player input either).
func TestProcessMotionCardTick_SpeedyCard_StaysInQuestionOnExpiry(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO") // SPEEDY cards
	startMEMOTION(t, e, "mq1", q)
	if err := e.SelectMotionCard("mc-1"); err != nil {
		t.Fatalf("SelectMotionCard: %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard: %v", err)
	}
	defer e.Stop()

	e.mu.Lock()
	e.state.CurrentTime = 1
	e.state.Delay = 30
	e.mu.Unlock()

	e.processMotionCardTick()
	if got := e.GetState().MotionSubPhase; got != MotionSubPhaseQuestion {
		t.Errorf("MotionSubPhase = %s, want QUESTION unchanged for SPEEDY at expiry", got)
	}
}
