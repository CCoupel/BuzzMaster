// Tests for #187 QUALIF bugfix — reported in manual validation of v7.1.0.1:
// a MEMOTION card of TYPE=MEMORY kept accepting FLIP_MEMORY_CARD after its
// own per-card timer reached 0. Root cause: processMotionCardTick's expiry
// branch only ever stopped the ticker goroutine (ticker.Stop()) — it never
// moved MotionSubPhase away from QUESTION, so FlipMotionMemoryCard's own
// playability guard (MotionSubPhase==QUESTION) stayed satisfied forever.
// That was correct for QCM/SPEEDY (no player input to block during
// QUESTION — contract's Playable only ever depended on MOTION_SUBPHASE),
// but MEMORY (#187) is the first nested type where a player CAN act during
// QUESTION, and its own plan explicitly required both card-end exits
// (grille complète OU fin décidée par le timer) to hand control to REVEAL
// — only the first exit was actually wired.
//
// Run: go test ./internal/game/... -run TestProcessMotionCardTick_MemoryCard_Expiry\|TestProcessMotionCardTick_.*_StaysInQuestion\|TestFlipMotionMemoryCard_.*AfterExpiry -v
package game

import "testing"

// TestProcessMotionCardTick_MemoryCard_ExpiryAutoRevealsCard is the direct
// regression test for the bug: driving the real per-tick code path
// (processMotionCardTick, not a hand-set subphase) down to CURRENT_TIME=0
// on an active MEMORY card must transition MotionSubPhase to REVEAL and
// mark the card REVEALED — exactly like a completed grid or an explicit
// MEMOTION_REVEAL, and WITHOUT ever calling Stop() (Phase stays STARTED).
func TestProcessMotionCardTick_MemoryCard_ExpiryAutoRevealsCard(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 4, 0) // 4 pairs, deliberately incomplete
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	// Only 1 of 4 pairs found — the card is incomplete when the timer
	// expires, exactly the QUALIF scenario ("il reste possible de
	// retourner de nouvelles cartes... après que le chrono soit arrivé à
	// 0").
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
	if !result.autoRevealed {
		t.Errorf("processMotionCardTick().autoRevealed = false at expiry on a MEMORY card, want true")
	}

	state := e.GetState()
	if state.Phase != PhaseStarted {
		t.Errorf("Phase = %s, want STARTED — expiry must never Stop() the MEMOTION round", state.Phase)
	}
	if state.MotionSubPhase != MotionSubPhaseReveal {
		t.Errorf("MotionSubPhase = %s, want REVEAL — the QUALIF bug: a card must stop accepting flips once its timer expires", state.MotionSubPhase)
	}
	if got := state.MotionCardStates["mc-mem"]; got != MotionCardStateRevealed {
		t.Errorf("MotionCardStates[mc-mem] = %s, want REVEALED", got)
	}
}

// TestFlipMotionMemoryCard_IgnoredAfterTimerExpiry is the end-to-end
// regression test tying the two engine methods together exactly as a real
// game session would: drive the timer to expiry via processMotionCardTick,
// then attempt a flip on the now-expired card — it must be a complete
// no-op (FlipMotionMemoryCard's own MotionSubPhase==QUESTION guard, now
// correctly failing since the tick moved the sub-phase to REVEAL).
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
	e.processMotionCardTick() // → CURRENT_TIME=0, auto-reveal

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

// TestProcessMotionCardTick_MemoryCard_NoAutoRevealBeforeExpiry is the
// negative counterpart: a tick that does NOT reach CURRENT_TIME<=0 must
// leave the card fully playable — the fix must not reveal early.
func TestProcessMotionCardTick_MemoryCard_NoAutoRevealBeforeExpiry(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 4, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	e.mu.Lock()
	e.state.CurrentTime = 10
	e.state.Delay = 30
	e.mu.Unlock()

	result := e.processMotionCardTick() // → CurrentTime=9, still running
	if result.autoRevealed {
		t.Fatalf("autoRevealed=true at CurrentTime=%d, want false — must only fire at expiry", result.currentTime)
	}
	if got := e.GetState().MotionSubPhase; got != MotionSubPhaseQuestion {
		t.Errorf("MotionSubPhase = %s, want QUESTION (timer not yet expired)", got)
	}

	// The card must still accept a flip.
	isMatch, _, _, _ := e.FlipMotionMemoryCard("mc-mem", cardIDFor(1, 1))
	_ = isMatch // no assertion on the value itself — the point is FlipMotionMemoryCard must not be a guard-rejected no-op
	flipped := motionActiveMemoryFlippedCards(e.GetState().MotionActive.State)
	if len(flipped) != 1 {
		t.Errorf("MEMORY_FLIPPED_CARDS = %v after a flip before expiry, want 1 entry — the card must still be playable", flipped)
	}
}

// TestProcessMotionCardTick_QCMCard_StaysInQuestionOnExpiry is the explicit
// non-regression guard for the fix's scope: QCM (and, by the same
// mechanism, SPEEDY) has no player input during QUESTION to block, and its
// pre-#187 behavior on expiry — stay in QUESTION, admin must act via
// MEMOTION_REVEAL/MEMOTION_DONE — must be completely unaffected. The fix is
// deliberately gated on MotionActive.Type == QuestionTypeMemory.
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
	if result.autoRevealed {
		t.Errorf("autoRevealed=true for a QCM card — the fix must be scoped to MEMORY only")
	}
	if got := e.GetState().MotionSubPhase; got != MotionSubPhaseQuestion {
		t.Errorf("MotionSubPhase = %s, want QUESTION unchanged for QCM at expiry (pre-#187 behavior)", got)
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

	result := e.processMotionCardTick()
	if result.autoRevealed {
		t.Errorf("autoRevealed=true for a SPEEDY card — the fix must be scoped to MEMORY only")
	}
	if got := e.GetState().MotionSubPhase; got != MotionSubPhaseQuestion {
		t.Errorf("MotionSubPhase = %s, want QUESTION unchanged for SPEEDY at expiry (pre-#187 behavior)", got)
	}
}
