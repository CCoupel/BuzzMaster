package lighting

// Developer-side unit tests for the writer (dev-backend, #205). The
// contract-driven suite (CA2..CA6) is test-writer's writer_test.go; these
// TestDev* tests only pin the mechanics that are easiest to get silently
// wrong: nil/disabled safety, last-state-wins, pulse fallback, throttle
// arithmetic with an injected clock, and the lock-free Apply path.

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeClock is a manual clock + timer factory: timers fire only when the
// test advances time past their deadline.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	at        time.Time
	fn        func()
	cancelled bool
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 9, 2, 20, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) NewTimer(d time.Duration, fn func()) func() {
	c.mu.Lock()
	t := &fakeTimer{at: c.now.Add(d), fn: fn}
	c.timers = append(c.timers, t)
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		t.cancelled = true
		c.mu.Unlock()
	}
}

// Advance moves the clock and fires every due timer (outside the lock).
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []*fakeTimer
	var rest []*fakeTimer
	for _, t := range c.timers {
		if !t.cancelled && !t.at.After(c.now) {
			due = append(due, t)
		} else if !t.cancelled {
			rest = append(rest, t)
		}
	}
	c.timers = rest
	c.mu.Unlock()
	for _, t := range due {
		t.fn()
	}
}

// waitFor polls until cond is true or the deadline passes.
func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", msg)
}

func sceneOf(ev Event) State {
	return State{Zones: []ZoneState{{Zone: ZoneGeneral, Color: [3]int{len(ev.Kind), len(ev.Teams), 0}, Intensity: 1}}}
}

func TestDevNilAndDisabledWriterAreNoOps(t *testing.T) {
	var nilW *Writer
	nilW.NotifyState()
	nilW.NotifyPulse(KindScore, []string{"A"}, time.Second)
	nilW.Start(context.Background()) // must return immediately
	if nilW.Enabled() {
		t.Fatal("nil writer must not be enabled")
	}

	w := NewWriter(Config{}) // no driver ⇒ disabled
	if w.Enabled() {
		t.Fatal("writer without driver must be disabled")
	}
	done := make(chan struct{})
	go func() { w.Start(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disabled Start must return without blocking")
	}
	w.NotifyState()
	if w.Stats().Notifies != 0 {
		t.Fatal("disabled writer must not count notifies")
	}
}

func TestDevLastStateWinsAndDerivesAtApplyTime(t *testing.T) {
	clock := newFakeClock()
	drv := NewFakeDriver()
	var mu sync.Mutex
	current := Event{Kind: KindIdle}
	w := NewWriter(Config{
		Driver: drv,
		Derive: func() Event { mu.Lock(); defer mu.Unlock(); return current },
		Scene:  sceneOf,
		Now:    clock.Now, NewTimer: clock.NewTimer,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)

	// 50 notifies while the state keeps changing; the driver must see the
	// LAST live state, not the state at notify time.
	for i := 0; i < 50; i++ {
		w.NotifyState()
	}
	mu.Lock()
	current = Event{Kind: KindRunning}
	mu.Unlock()
	w.NotifyState()

	waitFor(t, func() bool { return drv.Count() >= 1 }, "first apply")
	// Throttled: nothing more until the clock moves.
	time.Sleep(20 * time.Millisecond)
	first := drv.Count()
	clock.Advance(MinInterval)
	waitFor(t, func() bool {
		last, ok := drv.Last()
		return ok && last.Zones[0].Color[0] == len(KindRunning)
	}, "last derived state applied")
	if drv.Count() > first+1 {
		t.Fatalf("burst must coalesce: %d applies after one throttle window", drv.Count())
	}
}

func TestDevThrottleBoundsApplyCount(t *testing.T) {
	clock := newFakeClock()
	drv := NewFakeDriver()
	w := NewWriter(Config{Driver: drv, Derive: func() Event { return Event{Kind: KindReady} }, Scene: sceneOf,
		Now: clock.Now, NewTimer: clock.NewTimer})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)

	// Burst: N=200 notifies spread over T=1s of fake time, 10 ms apart.
	const n, step = 200, 5 * time.Millisecond
	for i := 0; i < n; i++ {
		w.NotifyState()
		clock.Advance(step)
		time.Sleep(200 * time.Microsecond) // let the goroutine observe
	}
	total := time.Duration(n) * step
	clock.Advance(MinInterval) // flush the trailing refresh
	waitFor(t, func() bool { return drv.Count() >= 2 }, "throttled applies")
	time.Sleep(20 * time.Millisecond)
	max := int(total/MinInterval) + 1 + 1 // +1 trailing flush after the burst
	if got := drv.Count(); got > max || got < 2 {
		t.Fatalf("applies = %d, want 2 <= n <= %d for N=%d over %s (CA4)", got, max, n, total)
	}
}

func TestDevPulseRendersThenFallsBack(t *testing.T) {
	clock := newFakeClock()
	drv := NewFakeDriver()
	w := NewWriter(Config{Driver: drv, Derive: func() Event { return Event{Kind: KindRunning} }, Scene: sceneOf,
		Now: clock.Now, NewTimer: clock.NewTimer})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)

	w.NotifyPulse(KindScore, []string{"TeamA"}, ScorePulseDuration)
	waitFor(t, func() bool { return drv.Count() == 1 }, "pulse applied")
	if last, _ := drv.Last(); last.Zones[0].Color != [3]int{len(KindScore), 1, 0} {
		t.Fatalf("pulse scene not rendered: %+v", last)
	}
	// Before the deadline, nothing else happens even after the throttle.
	clock.Advance(ScorePulseDuration / 2)
	time.Sleep(20 * time.Millisecond)
	if drv.Count() != 1 {
		t.Fatalf("no apply expected before pulse deadline, got %d", drv.Count())
	}
	// At the deadline the writer wakes itself and re-derives.
	clock.Advance(ScorePulseDuration/2 + time.Millisecond)
	waitFor(t, func() bool { return drv.Count() == 2 }, "fallback after pulse")
	if last, _ := drv.Last(); last.Zones[0].Color != [3]int{len(KindRunning), 0, 0} {
		t.Fatalf("fallback must re-derive live state: %+v", last)
	}
}

func TestDevNotifyNeverBlocksWhileApplyIsStuck(t *testing.T) {
	drv := NewFakeDriver()
	drv.Gate = make(chan struct{})
	w := NewWriter(Config{Driver: drv, Derive: func() Event { return Event{Kind: KindIdle} }, Scene: sceneOf})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)
	w.NotifyState()
	time.Sleep(10 * time.Millisecond) // Apply is now blocked on Gate

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			w.NotifyState()
			w.NotifyPulse(KindScore, []string{"X"}, time.Second)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Notify* blocked while Apply was in flight")
	}
	close(drv.Gate)
	waitFor(t, func() bool { return drv.Count() >= 1 }, "apply released")
}

func TestDevConcurrentNotifiesRaceFree(t *testing.T) {
	drv := NewFakeDriver()
	w := NewWriter(Config{Driver: drv, Derive: func() Event { return Event{Kind: KindIdle} }, Scene: sceneOf,
		MinInterval: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				if (i+g)%3 == 0 {
					w.NotifyPulse(KindScore, []string{"T"}, 50*time.Millisecond)
				} else {
					w.NotifyState()
				}
			}
		}(g)
	}
	wg.Wait()
	waitFor(t, func() bool { return drv.Count() >= 1 }, "some apply")
	cancel()
	waitFor(t, drv.Closed, "driver closed on ctx cancel")
	if s := w.Stats(); s.Notifies != 16*500 || s.Applies == 0 || s.Applies > s.Notifies {
		t.Fatalf("stats = %+v", s)
	}
}

func TestDevApplyErrorIsCountedAndReported(t *testing.T) {
	drv := NewFakeDriver()
	drv.Err = context.DeadlineExceeded
	var got error
	var mu sync.Mutex
	w := NewWriter(Config{Driver: drv, Derive: func() Event { return Event{Kind: KindIdle} }, Scene: sceneOf,
		OnError: func(err error) { mu.Lock(); got = err; mu.Unlock() }})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)
	w.NotifyState()
	waitFor(t, func() bool { mu.Lock(); defer mu.Unlock(); return got != nil }, "error reported")
	if w.Stats().ApplyErrors != 1 {
		t.Fatalf("stats = %+v", w.Stats())
	}
}

// #207 — SetDriver hot-swaps the driver without touching the Writer pointer
// the 21 sites read: nil disables (Notify* no-op, goroutine idles), a new
// driver renders the current scene once, a re-enabled writer resumes.
func TestDevSetDriverHotSwap(t *testing.T) {
	w := NewWriter(Config{Derive: func() Event { return Event{Kind: KindReady} }, Scene: sceneOf, MinInterval: time.Millisecond})
	if w.Enabled() || w.Running() {
		t.Fatal("writer without driver must be disabled and not running")
	}
	w.Start(context.Background()) // returns at once: disabled
	if w.Running() {
		t.Fatal("Start on a disabled writer must not run")
	}

	first := NewFakeDriver()
	w.SetDriver(first)
	if !w.Enabled() {
		t.Fatal("SetDriver must enable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)
	waitFor(t, func() bool { return first.Count() >= 1 }, "first driver renders the current scene after SetDriver")
	waitFor(t, w.Running, "running")
	if w.Stats().Notifies != 0 {
		t.Fatal("the SetDriver refresh is not a notify")
	}

	// Swap: the old driver is closed, the new one gets the scene, notifies go to it.
	second := NewFakeDriver()
	w.SetDriver(second)
	waitFor(t, first.Closed, "previous driver closed on swap")
	waitFor(t, func() bool { return second.Count() >= 1 }, "second driver rendered")
	w.NotifyState()
	waitFor(t, func() bool { return second.Count() >= 2 }, "notify reaches the second driver")
	firstCount := first.Count()

	// Disable: notifies are no-ops, nothing reaches either driver, goroutine idles.
	w.SetDriver(nil)
	if w.Enabled() {
		t.Fatal("SetDriver(nil) must disable")
	}
	w.NotifyState()
	w.NotifyPulse(KindScore, []string{"A"}, time.Second)
	time.Sleep(20 * time.Millisecond)
	if first.Count() != firstCount || second.Count() != 2 {
		t.Fatalf("disabled writer must not apply: first=%d second=%d", first.Count(), second.Count())
	}
	if !w.Running() {
		t.Fatal("goroutine must keep idling while disabled")
	}

	// Re-enable on the same goroutine.
	third := NewFakeDriver()
	w.SetDriver(third)
	waitFor(t, func() bool { return third.Count() >= 1 }, "re-enabled writer renders again")
	cancel()
	waitFor(t, third.Closed, "driver closed on ctx cancel")
	waitFor(t, func() bool { return !w.Running() }, "running flag cleared")
}
