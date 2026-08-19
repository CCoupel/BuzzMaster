package game

// Regression tests for #151 (plan _work/reports/plan-20260819-160602.md,
// Phase 1), preparing the Batch 1 dispatch alongside dev-backend's mutex
// refactor. Covers the 4 long-lived ticker goroutines in this file:
// countdown (startCountdown, engine.go processCountdownTick), game timer
// (startTimer, processTimerTick — "le site le plus exposé" per the plan,
// since it also mutates QcmInvalidated and calls rand.Intn()), MEMOTION
// card timer (StartMotionCardTimer, processMotionCardTick), and MEMOTION
// memorize timer (StartMotionMemorizeTimer, processMotionMemorizeTick). The
// 5th site (cmd/server/main.go:1984, MEMORY auto-flip-back) is covered
// separately by cmd/server/memory_flipback_panic_recovery_test.go — it has
// no manual Lock/Unlock of its own to worry about.
//
// The critical point (team-lead's brief, restated in the plan's own risk
// table): a bare `recover()` wrapped AROUND a goroutine's manual
// `e.mu.Lock() / e.mu.Unlock()` pair is WORSE than the crash it replaces —
// if the panic happens between Lock and Unlock, recover() stops the crash
// but leaves e.mu locked forever, freezing the whole game engine on the
// next Lock() call (total, silent deadlock). The fix (#151) extracts each
// locked section into its own process*Tick method using
// `e.mu.Lock(); defer e.mu.Unlock()`, so Go's normal deferred-unlock-during-
// panic-unwind guarantees the mutex is released before recoverBackgroundPanic
// runs one frame up. A test that only asserts "the process didn't crash"
// cannot catch a regression back to the naive pattern — the mutex could
// still be left locked while the process survives, silently hanging every
// future game action. Each test below therefore proves BOTH: (a) the
// background loop keeps running (processes a further, real tick) after the
// injected panic, AND (b) e.mu is acquirable again shortly after — the
// actual non-deadlock proof.
//
// Injection point: engine.go's setTestInjectPanic/clearTestInjectPanic hook
// (added by dev-backend as part of the #151 refactor specifically for this
// purpose — nil in production, called at the very top of each process*Tick,
// while e.mu is held, before any state is touched). Without it there is no
// naturally reachable panic in these already nil-guarded ticker bodies.
//
// ⚠️ EXPECTED TO CRASH THE TEST BINARY if run against pre-#151 code (no
// recoverBackgroundPanic wrapping the ticker body) — this is intentional,
// the same RED-before-fix / GREEN-after-fix shape as
// cmd/server/dispatch_panic_recovery_test.go and
// internal/server/readpump_panic_recovery_test.go (#131). Do not add
// t.Skip() to hide it.

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// testEngineTickPanicRecovery is shared by the 4 engine.go ticker sites. It
// injects a panic on the ticker's first firing via setTestInjectPanic (filtered
// to `site` so only the goroutine under test reacts), then proves:
//
//  1. e.mu becomes acquirable again within a short deadline (the mutex was
//     not left locked by the panic) — the actual #151 risk.
//  2. the SAME background loop processes a further tick afterward (not just
//     "the process is still up" — the goroutine itself must still be
//     consuming ticks, exactly like dispatch_panic_recovery_test.go's
//     "second message" proof for the dispatch loop).
func testEngineTickPanicRecovery(t *testing.T, site string, start func(e *Engine)) {
	t.Helper()

	e := NewEngine()
	t.Cleanup(func() {
		e.Stop()
		clearTestInjectPanic()
	})

	var calls int32
	panickedCh := make(chan struct{})
	resumedCh := make(chan struct{})
	// setTestInjectPanic (not a direct `testInjectPanicFn = ...` assignment)
	// — the ticker goroutines read the hook from inside process*Tick while
	// e.mu is held, but that doesn't synchronize with a plain assignment
	// from this test goroutine; go test -race caught exactly this before
	// the accessor was introduced (dev-backend, #151 follow-up).
	setTestInjectPanic(func(s string) {
		if s != site {
			return
		}
		if atomic.AddInt32(&calls, 1) == 1 {
			close(panickedCh)
			panic(fmt.Sprintf("injected panic — #151 regression test (%s)", site))
		}
		select {
		case <-resumedCh:
			// already signaled
		default:
			close(resumedCh)
		}
	})

	start(e)

	select {
	case <-panickedCh:
	case <-time.After(3 * time.Second):
		t.Fatalf("%s: injected panic was never triggered — the ticker never fired, test setup is broken", site)
	}

	// (b) THE critical assertion. A recover() posed around the raw
	// Lock()/Unlock() pair (the naive, pre-#151 shape) would still stop the
	// crash but leave e.mu held forever — every subsequent Lock() call
	// anywhere in the engine (i.e. almost every game action) would hang
	// silently. Poll instead of asserting instantly: the unlock happens as
	// part of the panicking goroutine's own stack unwind, which races with
	// this goroutine — polling is the correct way to prove "released
	// promptly", not "released synchronously before we could check".
	deadline := time.Now().Add(2 * time.Second)
	for {
		if e.mu.TryLock() {
			e.mu.Unlock()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: e.mu is still held/deadlocked 2s after the injected panic — "+
				"the recover() did not release the mutex (see #151 risk: a bare recover() "+
				"around a manual Lock/Unlock is worse than the original crash)", site)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// (a) The loop itself survived, not just the process: a further, real
	// tick must still be processed after the injected panic.
	select {
	case <-resumedCh:
	case <-time.After(3 * time.Second):
		t.Fatalf("%s: no further tick was processed after the injected panic — the background "+
			"goroutine likely died instead of continuing its `for { select { ... } }` loop "+
			"(recover() wrapped around the loop instead of around a single iteration)", site)
	}
}

// Site 1/5 — engine.go:936 (line drifted since the plan was written; see
// startCountdown), the 3-2-1 countdown ticker.
func TestCountdownTick_PanicRecovery(t *testing.T) {
	testEngineTickPanicRecovery(t, "countdown", func(e *Engine) {
		e.startCountdown()
	})
}

// Site 2/5 — engine.go:1122 (startTimer), the in-game timer. Per the plan,
// "le site le plus exposé": the locked section also mutates
// e.state.QcmInvalidated and calls rand.Intn() via invalidateRandomWrongAnswer.
func TestTimerTick_PanicRecovery(t *testing.T) {
	testEngineTickPanicRecovery(t, "timer", func(e *Engine) {
		e.startTimer()
	})
}

// Site 3/5 — engine.go:3896 (StartMotionCardTimer), MEMOTION card timer.
func TestMotionCardTick_PanicRecovery(t *testing.T) {
	testEngineTickPanicRecovery(t, "motion-card", func(e *Engine) {
		e.StartMotionCardTimer(30) // delay must be > 0 or the timer never starts (no-op guard)
	})
}

// Site 4/5 — engine.go:3970 (StartMotionMemorizeTimer), MEMOTION MEMORIZE
// timer (auto-transitions to GRID on expiry).
func TestMotionMemorizeTick_PanicRecovery(t *testing.T) {
	testEngineTickPanicRecovery(t, "motion-memorize", func(e *Engine) {
		e.StartMotionMemorizeTimer(30) // duration must be > 0 or the timer never starts (no-op guard)
	})
}
