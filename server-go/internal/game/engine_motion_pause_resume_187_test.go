// Tests for #187 cycle 6 — QUALIF v7.1.0.3 bug report: during a MEMOTION
// round, PAUSE stops the chrono correctly, but CONTINUER never resumes the
// countdown — the game appears stuck. Root cause: PhasePaused used to fall
// into processMotionCardTick's (and processMotionMemorizeTick's) combined
// "game state changed unexpectedly" guard, which makes the ticker goroutine
// call ticker.Stop() and RETURN FROM THE GOROUTINE ENTIRELY. Engine.Continue()
// only flips Phase back to STARTED — it has no notion of a MEMOTION card (or
// MEMORIZE) timer, so nothing ever restarted the dead goroutine.
//
// Pre-existing since #151 (this ticker infrastructure predates #183-#187
// entirely) — NOT a regression introduced by any #187 cycle. Simply never
// exercised by manual QUALIF testing before MEMORY-in-card gave pause/resume
// during a card timer real attention.
//
// Run: go test ./internal/game/... -run TestProcessMotionCardTick_Paused\|TestProcessMotionMemorizeTick_Paused\|TestStartMotionCardTimer_PauseThenContinue\|TestStartMotionMemorizeTimer_PauseThenContinue -v
package game

import (
	"testing"
	"time"
)

// ============================================================================
// Direct engine-level reproduction (fast, deterministic — no real ticker)
// ============================================================================

// TestProcessMotionCardTick_PausedTick_IsInertNotGuardFailed is the direct
// unit test for the fix: a tick while Phase==PAUSED must report
// paused=true, NOT guardFailed=true — and must not decrement CurrentTime
// at all (a paused tick is a pure no-op, not a "the round genuinely ended"
// signal).
func TestProcessMotionCardTick_PausedTick_IsInertNotGuardFailed(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 4, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	e.mu.Lock()
	e.state.CurrentTime = 10
	e.state.Delay = 30
	e.mu.Unlock()

	e.Pause() // the real, production Pause() — sets Phase=PAUSED, nothing more

	result := e.processMotionCardTick()
	if result.guardFailed {
		t.Errorf("processMotionCardTick() during PAUSE: guardFailed=true, want false — this is what killed the ticker goroutine (cycle 6 bug)")
	}
	if !result.paused {
		t.Errorf("processMotionCardTick() during PAUSE: paused=false, want true")
	}
	if got := e.GetState().CurrentTime; got != 10 {
		t.Errorf("CurrentTime = %d after a paused tick, want 10 unchanged — a paused tick must never decrement", got)
	}
}

// TestProcessMotionMemorizeTick_PausedTick_IsInertNotGuardFailed mirrors the
// card-timer test for the MEMORIZE (Secret Mode) countdown — same
// vulnerable guard pattern, fixed proactively alongside the reported bug.
func TestProcessMotionMemorizeTick_PausedTick_IsInertNotGuardFailed(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	q.MotionMemorizeDuration = 30
	e.Ready("mq1", q)
	e.StartImmediate(0)
	e.InitMotionState() // MotionMemorizeDuration>0 → MotionSubPhase=MEMORIZE
	defer e.Stop()

	if got := e.GetState().MotionSubPhase; got != MotionSubPhaseMemorize {
		t.Fatalf("setup invalide : MotionSubPhase = %s, attendu MEMORIZE", got)
	}

	e.mu.Lock()
	e.state.CurrentTime = 10
	e.mu.Unlock()

	e.Pause()

	result := e.processMotionMemorizeTick()
	if result.guardFailed {
		t.Errorf("processMotionMemorizeTick() during PAUSE: guardFailed=true, want false")
	}
	if !result.paused {
		t.Errorf("processMotionMemorizeTick() during PAUSE: paused=false, want true")
	}
	if got := e.GetState().CurrentTime; got != 10 {
		t.Errorf("CurrentTime = %d after a paused tick, want 10 unchanged", got)
	}
	if got := e.GetState().MotionSubPhase; got != MotionSubPhaseMemorize {
		t.Errorf("MotionSubPhase = %s after a paused tick, want MEMORIZE unchanged (must not auto-expire to GRID while paused)", got)
	}
}

// ============================================================================
// End-to-end regression, real ticker (proves the GOROUTINE itself survives
// pause — an engine-only test calling processMotionCardTick directly cannot
// catch "the goroutine exited", only the real StartMotionCardTimer path can)
// ============================================================================

// TestStartMotionCardTimer_PauseThenContinue_ResumesCountdown is the exact
// end-to-end reproduction of the QUALIF report: start a real per-card
// ticker, let it tick once, PAUSE, let a tick elapse while paused (must be
// a no-op, ticker must survive), CONTINUE, let another tick elapse — the
// countdown must resume decrementing. Uses real 1-second ticks (the
// production ticker's own resolution); ~4s wall-clock.
func TestStartMotionCardTimer_PauseThenContinue_ResumesCountdown(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 4, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card)
	defer e.Stop()

	e.StartMotionCardTimer(5)
	time.Sleep(1200 * time.Millisecond) // one real tick: 5 → 4
	afterFirstTick := e.GetState().CurrentTime
	if afterFirstTick != 4 {
		t.Fatalf("setup invalide : CurrentTime après le premier tic = %d, attendu 4", afterFirstTick)
	}

	e.Pause()
	time.Sleep(1200 * time.Millisecond) // one tick elapses WHILE paused — must be a no-op
	duringPause := e.GetState().CurrentTime
	if duringPause != 4 {
		t.Errorf("CurrentTime pendant la pause = %d, attendu 4 inchangé (le tic pendant la pause doit être un no-op)", duringPause)
	}

	e.Continue()
	time.Sleep(1200 * time.Millisecond) // the ticker must still be alive and now resume decrementing
	afterResume := e.GetState().CurrentTime
	if afterResume != 3 {
		t.Errorf("CurrentTime après CONTINUER = %d, attendu 3 — le compte à rebours doit reprendre (bug QUALIF v7.1.0.3 : restait bloqué à %d)", afterResume, duringPause)
	}
}

// TestStartMotionMemorizeTimer_PauseThenContinue_ResumesCountdown mirrors
// the card-timer end-to-end test for the MEMORIZE (Secret Mode) countdown.
func TestStartMotionMemorizeTimer_PauseThenContinue_ResumesCountdown(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	q.MotionMemorizeDuration = 5
	e.Ready("mq1", q)
	e.StartImmediate(0)
	e.InitMotionState()
	defer e.Stop()

	e.StartMotionMemorizeTimer(5)
	time.Sleep(1200 * time.Millisecond) // one real tick: 5 → 4
	if got := e.GetState().CurrentTime; got != 4 {
		t.Fatalf("setup invalide : CurrentTime après le premier tic = %d, attendu 4", got)
	}

	e.Pause()
	time.Sleep(1200 * time.Millisecond)
	if got := e.GetState().CurrentTime; got != 4 {
		t.Errorf("CurrentTime pendant la pause = %d, attendu 4 inchangé", got)
	}
	if got := e.GetState().MotionSubPhase; got != MotionSubPhaseMemorize {
		t.Errorf("MotionSubPhase pendant la pause = %s, attendu MEMORIZE inchangé", got)
	}

	e.Continue()
	time.Sleep(1200 * time.Millisecond)
	if got := e.GetState().CurrentTime; got != 3 {
		t.Errorf("CurrentTime après CONTINUER = %d, attendu 3 — le compte à rebours MEMORIZE doit reprendre", got)
	}
}

// ============================================================================
// Non-regression: a REAL round-ending state change (not pause) must still
// permanently stop the ticker — the guardFailed path is unaffected.
// ============================================================================

// TestStartMotionCardTimer_StopDuringCard_TickerStaysStopped is the
// explicit non-regression companion: STOP (not PAUSE) must still kill the
// ticker for good — verified by ticking through 3 real seconds after STOP
// and confirming CurrentTime never resumes decrementing (it stays at
// whatever Stop() itself set it to).
func TestStartMotionCardTimer_StopDuringCard_TickerStaysStopped(t *testing.T) {
	e := NewEngine()
	card := memoryMotionCard("mc-mem", 1, 4, 0)
	startMEMOTIONAtMemoryCardQuestion(t, e, card)

	e.StartMotionCardTimer(5)
	time.Sleep(1200 * time.Millisecond) // 5 → 4
	if got := e.GetState().CurrentTime; got != 4 {
		t.Fatalf("setup invalide : CurrentTime après le premier tic = %d, attendu 4", got)
	}

	e.Stop() // genuine round end — Phase leaves STARTED for good (STOPPED/REVEALED)
	stoppedAt := e.GetState().CurrentTime

	time.Sleep(1500 * time.Millisecond) // long enough for >1 tick if the ticker were still alive
	if got := e.GetState().CurrentTime; got != stoppedAt {
		t.Errorf("CurrentTime = %d, %ds après Stop(), attendu %d inchangé — la garde guardFailed doit toujours arrêter le ticker pour de bon sur un vrai changement d'état", got, 1, stoppedAt)
	}
}
