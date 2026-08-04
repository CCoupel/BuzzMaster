package main

import (
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// BroadcastCoalescer (#129 T2.1/T2.3)
//
// Uses an injected, manually-fired fake timer throughout — no time.Sleep, no
// real clock, fully deterministic (per T2.1's explicit requirement).
// ---------------------------------------------------------------------------

// fakeCoalescerTimer lets a test control exactly when a coalescer's pending
// timer "fires", without waiting on a real clock. Only one timer can be
// pending at a time in BroadcastCoalescer's own design (a fresh Trigger()
// while one is pending is coalesced, never starts a second) — this fake
// mirrors that by only ever tracking the single most recent one.
type fakeCoalescerTimer struct {
	mu        sync.Mutex
	pending   func()
	cancelled bool
	starts    int // number of times the factory was invoked — proves coalescing
}

func newFakeCoalescerTimer() *fakeCoalescerTimer {
	return &fakeCoalescerTimer{}
}

func (f *fakeCoalescerTimer) factory(_ time.Duration, fn func()) func() {
	f.mu.Lock()
	f.pending = fn
	f.cancelled = false
	f.starts++
	f.mu.Unlock()
	return func() {
		f.mu.Lock()
		f.cancelled = true
		f.pending = nil
		f.mu.Unlock()
	}
}

// fire invokes the currently-pending callback, if any, exactly as a real
// timer would when its duration elapses. No-op if nothing is pending.
func (f *fakeCoalescerTimer) fire() {
	f.mu.Lock()
	fn := f.pending
	f.pending = nil
	f.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (f *fakeCoalescerTimer) startCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts
}

func newCoalescerForTest(emit func()) (*BroadcastCoalescer, *fakeCoalescerTimer) {
	timer := newFakeCoalescerTimer()
	c := NewBroadcastCoalescer(150*time.Millisecond, emit)
	c.newTimer = timer.factory
	return c, timer
}

// ---------------------------------------------------------------------------
// 8 entrées dans la fenêtre -> 1 seule émission
// ---------------------------------------------------------------------------

func TestBroadcastCoalescer_BurstWithinWindow_SingleEmit(t *testing.T) {
	var emits int
	c, timer := newCoalescerForTest(func() { emits++ })

	for i := 0; i < 8; i++ {
		c.Trigger()
	}
	if got := timer.startCount(); got != 1 {
		t.Fatalf("expected exactly 1 timer started for 8 Trigger() calls within the window (coalesced), got %d", got)
	}
	if emits != 0 {
		t.Fatalf("expected 0 emits before the timer fires, got %d", emits)
	}

	timer.fire()

	if emits != 1 {
		t.Errorf("expected exactly 1 emit for 8 Trigger() calls within one window, got %d", emits)
	}
}

// TestBroadcastCoalescer_SeparateWindows_TwoEmits proves the coalescer
// doesn't just permanently swallow calls after the first: once a window's
// timer has fired, the NEXT Trigger() arms a fresh one.
func TestBroadcastCoalescer_SeparateWindows_TwoEmits(t *testing.T) {
	var emits int
	c, timer := newCoalescerForTest(func() { emits++ })

	c.Trigger()
	c.Trigger()
	timer.fire()
	if emits != 1 {
		t.Fatalf("expected 1 emit after the first window fired, got %d", emits)
	}

	c.Trigger()
	c.Trigger()
	c.Trigger()
	timer.fire()
	if emits != 2 {
		t.Errorf("expected 2 emits total across two separate windows, got %d", emits)
	}
	if got := timer.startCount(); got != 2 {
		t.Errorf("expected exactly 2 timers started total (one per window), got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Vidage sur changement de phase (Flush)
// ---------------------------------------------------------------------------

func TestBroadcastCoalescer_Flush_EmitsImmediatelyAndCancelsPendingTimer(t *testing.T) {
	var emits int
	c, timer := newCoalescerForTest(func() { emits++ })

	c.Trigger()
	if emits != 0 {
		t.Fatalf("expected 0 emits before Flush, got %d", emits)
	}

	c.Flush()

	if emits != 1 {
		t.Errorf("expected Flush to emit immediately for a pending Trigger, got %d emits", emits)
	}
	timer.mu.Lock()
	cancelled := timer.cancelled
	timer.mu.Unlock()
	if !cancelled {
		t.Errorf("expected Flush to cancel the pending timer, but it wasn't cancelled")
	}

	// The (now-cancelled) timer firing after Flush must not double-emit —
	// mirrors what a real time.Timer.Stop() + already-fired race would do
	// were BroadcastCoalescer not defensive about it (fire() reads whatever
	// the CURRENT c.cancel is, which Flush already cleared to nil).
	timer.fire() // no-op: fakeCoalescerTimer.pending was cleared by Flush's cancel()
	if emits != 1 {
		t.Errorf("expected no double-emit from a stale timer firing after Flush, got %d emits", emits)
	}
}

func TestBroadcastCoalescer_Flush_NoOpWhenNothingPending(t *testing.T) {
	var emits int
	c, _ := newCoalescerForTest(func() { emits++ })

	c.Flush()

	if emits != 0 {
		t.Errorf("expected Flush to be a no-op with nothing pending, got %d emits", emits)
	}
}

// TestBroadcastCoalescer_TriggerAfterFlush_ArmsAFreshWindow proves Flush
// doesn't leave the coalescer stuck: a Trigger() right after a Flush starts
// a brand new window rather than being silently absorbed.
func TestBroadcastCoalescer_TriggerAfterFlush_ArmsAFreshWindow(t *testing.T) {
	var emits int
	c, timer := newCoalescerForTest(func() { emits++ })

	c.Trigger()
	c.Flush()
	if emits != 1 {
		t.Fatalf("setup failed: expected 1 emit from Flush, got %d", emits)
	}

	c.Trigger()
	timer.fire()
	if emits != 2 {
		t.Errorf("expected a Trigger() after Flush to arm a fresh window and emit, got %d emits total", emits)
	}
}

// ---------------------------------------------------------------------------
// Contenu de l'émission différée = état au moment de l'émission (invariant
// de non-péremption — la propriété fondatrice du coalescer)
// ---------------------------------------------------------------------------

// TestBroadcastCoalescer_EmitReadsStateAtFireTime_NeverStale is the test
// that proves the founding invariant documented on BroadcastCoalescer: emit
// is a closure that reads LIVE, mutable state — a value that changes between
// Trigger() and the timer firing must be reflected in what's actually
// emitted, never the value that existed at Trigger() time (the coalescer
// never buffers a payload, only a "please emit" flag).
func TestBroadcastCoalescer_EmitReadsStateAtFireTime_NeverStale(t *testing.T) {
	state := "initial"
	var observed []string
	c, timer := newCoalescerForTest(func() { observed = append(observed, state) })

	c.Trigger()
	state = "changed-before-fire" // mutated AFTER Trigger(), BEFORE the timer fires
	timer.fire()

	if len(observed) != 1 || observed[0] != "changed-before-fire" {
		t.Errorf("expected the emit to observe live state at fire time (%q), got %v — a stale/buffered payload would show %q", "changed-before-fire", observed, "initial")
	}
}

// ---------------------------------------------------------------------------
// Arrêt propre (Stop)
// ---------------------------------------------------------------------------

func TestBroadcastCoalescer_Stop_CancelsPendingTimerWithoutEmitting(t *testing.T) {
	var emits int
	c, timer := newCoalescerForTest(func() { emits++ })

	c.Trigger()
	c.Stop()

	if emits != 0 {
		t.Errorf("expected Stop to never emit, got %d emits", emits)
	}
	timer.mu.Lock()
	cancelled := timer.cancelled
	timer.mu.Unlock()
	if !cancelled {
		t.Errorf("expected Stop to cancel the pending timer")
	}
}

func TestBroadcastCoalescer_Stop_NoOpWhenNothingPending(t *testing.T) {
	c, _ := newCoalescerForTest(func() {})
	c.Stop() // must not panic on a nil c.cancel
}

// ---------------------------------------------------------------------------
// Concurrence — go test -race
// ---------------------------------------------------------------------------

// TestBroadcastCoalescer_ConcurrentTriggerAndFlush_NoRace exercises the real
// (unfaked) timer factory under concurrent access: many goroutines calling
// Trigger()/Flush()/Stop() simultaneously must never race or panic. This is
// the one test in this file that uses BroadcastCoalescer's default
// (non-injected) timer — it needs a REAL scheduler to genuinely race against.
func TestBroadcastCoalescer_ConcurrentTriggerAndFlush_NoRace(t *testing.T) {
	var mu sync.Mutex
	emits := 0
	c := NewBroadcastCoalescer(2*time.Millisecond, func() {
		mu.Lock()
		emits++
		mu.Unlock()
	})
	defer c.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch i % 3 {
			case 0:
				c.Trigger()
			case 1:
				c.Flush()
			case 2:
				c.Stop()
			}
		}(i)
	}
	wg.Wait()
	// No assertion on the exact emit count (inherently racy by design — the
	// point is the absence of a data race/panic, verified by `go test -race`).
}
