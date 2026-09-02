package lighting

import (
	"context"
	"sync"
	"time"
)

// TimerFactory starts a one-shot timer calling fn after d and returns a
// cancel function (idempotent, safe after the timer fired — same contract
// as time.Timer.Stop). Injectable so tests run deterministically without a
// real clock, mirroring cmd/server/broadcast_coalescer.go's
// coalescerTimerFactory (#129 T2.1: "minuteur injectable pour tests
// déterministes, pas de time.Sleep en test").
type TimerFactory func(d time.Duration, fn func()) (cancel func())

func defaultTimerFactory(d time.Duration, fn func()) func() {
	t := time.AfterFunc(d, fn)
	return func() { t.Stop() }
}

// Config wires a Writer. Driver nil ⇒ the writer is DISABLED: every Notify*
// is a no-op and Start returns at once without spawning anything — this is
// how "lighting not configured" costs strictly nothing (contract §9).
type Config struct {
	Driver Driver
	// Derive reads the LIVE game state and returns the current scene event.
	// Called on the writer goroutine, at apply time — never at notify time.
	Derive func() Event
	// Scene maps an Event to the State to render (scene table, team colours).
	Scene func(Event) State
	// MinInterval overrides the package constant (0 = MinInterval).
	MinInterval time.Duration
	// Now and NewTimer are injectable clocks for tests (nil = real time).
	Now      func() time.Time
	NewTimer TimerFactory
	// OnError, if set, receives Driver.Apply errors (called on the writer
	// goroutine). nil = errors are counted only (see Stats).
	OnError func(error)
}

// pulse is the single-slot register for non-derivable events (contract
// §4.2). A new pulse overwrites the previous one — last one wins.
type pulse struct {
	kind     EventKind
	teams    []string
	deadline time.Time
}

// Writer decouples the game sites from the hardware (contract §4).
//
// Founding invariant, taken verbatim from BroadcastCoalescer: the writer
// NEVER buffers a state. It only remembers that a refresh is due
// (refreshDue) and re-derives from the live GameState when it actually
// runs. A deferred emission is therefore always redundant with the state
// that existed when it was scheduled — never stale, never behind an order
// sent in between. Two concurrent Engine callbacks (they run outside the
// engine lock, unserialised — engine.go:220-241, root cause of #121) cannot
// race each other into a wrong order, because there is nothing to order.
//
// The mutex protects ONLY refreshDue and pulse. It is never held during
// Apply nor during a GameState read (contract §5).
type Writer struct {
	enabled     bool
	driver      Driver
	derive      func() Event
	scene       func(Event) State
	minInterval time.Duration
	now         func() time.Time
	newTimer    TimerFactory
	onError     func(error)

	mu         sync.Mutex
	refreshDue bool
	pulse      *pulse
	wake       chan struct{} // capacity 1, carries no data — "look", not "here is"

	// Owned by the Start goroutine only.
	lastApply   time.Time
	cancelTimer func() // pending wake-up timer (throttle or pulse expiry), nil if none

	statsMu sync.Mutex
	stats   Stats
}

// Stats is what the writer has done so far (for tests and diagnostics).
type Stats struct {
	Applies     int // Driver.Apply calls
	ApplyErrors int
	Notifies    int // NotifyState + NotifyPulse calls that were not no-ops
}

// NewWriter builds a writer. Disabled if cfg.Driver is nil.
func NewWriter(cfg Config) *Writer {
	w := &Writer{
		enabled:     cfg.Driver != nil,
		driver:      cfg.Driver,
		derive:      cfg.Derive,
		scene:       cfg.Scene,
		minInterval: cfg.MinInterval,
		now:         cfg.Now,
		newTimer:    cfg.NewTimer,
		onError:     cfg.OnError,
		wake:        make(chan struct{}, 1),
	}
	if w.minInterval <= 0 {
		w.minInterval = MinInterval
	}
	if w.now == nil {
		w.now = time.Now
	}
	if w.newTimer == nil {
		w.newTimer = defaultTimerFactory
	}
	if w.derive == nil {
		w.derive = func() Event { return Event{Kind: KindIdle} }
	}
	if w.scene == nil {
		w.scene = func(Event) State { return State{} }
	}
	return w
}

// Enabled reports whether a driver is attached. Safe on a nil receiver.
func (w *Writer) Enabled() bool { return w != nil && w.enabled }

// Stats returns a copy of the counters. Safe on a nil receiver.
func (w *Writer) Stats() Stats {
	if w == nil {
		return Stats{}
	}
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	return w.stats
}

// NotifyState signals that the scene must be re-derived from the live game
// state. NEVER blocks; safe on a nil or disabled writer, so call sites need
// no guard (contract §4.3).
func (w *Writer) NotifyState() {
	if w == nil || !w.enabled {
		return
	}
	w.mu.Lock()
	w.refreshDue = true
	w.mu.Unlock()
	w.countNotify()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// NotifyPulse registers a non-derivable event (SCORE) rendered for d, then
// the room falls back to the derived state on its own. Last pulse wins.
// NEVER blocks; safe on a nil or disabled writer.
func (w *Writer) NotifyPulse(kind EventKind, teams []string, d time.Duration) {
	if w == nil || !w.enabled {
		return
	}
	w.mu.Lock()
	w.pulse = &pulse{kind: kind, teams: append([]string(nil), teams...), deadline: w.now().Add(d)}
	w.refreshDue = true
	w.mu.Unlock()
	w.countNotify()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *Writer) countNotify() {
	w.statsMu.Lock()
	w.stats.Notifies++
	w.statsMu.Unlock()
}

// Start runs the single writer goroutine until ctx is done, then closes the
// driver. Returns immediately (no goroutine, no Close) when disabled. Call as
// `go w.Start(ctx)` from (*App).start(), exactly like AckManager.
func (w *Writer) Start(ctx context.Context) {
	if w == nil || !w.enabled {
		return
	}
	defer func() {
		if w.cancelTimer != nil {
			w.cancelTimer()
			w.cancelTimer = nil
		}
		_ = w.driver.Close()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		}
		w.drain(ctx)
	}
}

// drain applies at most one state per wake-up, re-arming a timer when the
// throttle or a pulse expiry requires a later pass. It never holds w.mu
// while calling derive/scene/Apply.
func (w *Writer) drain(ctx context.Context) {
	for {
		now := w.now()

		w.mu.Lock()
		due := w.refreshDue
		var p *pulse
		if w.pulse != nil {
			if w.pulse.deadline.After(now) {
				p = w.pulse
			} else {
				w.pulse = nil // expired: fall back to derived state
			}
		}
		if !due {
			w.mu.Unlock()
			return
		}
		// Throttle: at least minInterval between two Apply calls. Leave the
		// refresh due and come back when the interval has elapsed.
		if !w.lastApply.IsZero() {
			if wait := w.minInterval - now.Sub(w.lastApply); wait > 0 {
				w.mu.Unlock()
				w.armWake(wait, false)
				return
			}
		}
		w.refreshDue = false
		w.mu.Unlock()

		// Outside the lock: derive from live state (or render the pulse).
		var ev Event
		if p != nil {
			ev = Event{Kind: p.kind, Teams: append([]string(nil), p.teams...)}
		} else {
			ev = w.derive()
		}
		st := w.scene(ev)

		if ctx.Err() != nil {
			return
		}
		err := w.driver.Apply(ctx, st)
		w.lastApply = w.now()
		w.statsMu.Lock()
		w.stats.Applies++
		if err != nil {
			w.stats.ApplyErrors++
		}
		w.statsMu.Unlock()
		if err != nil && w.onError != nil {
			w.onError(err)
		}

		// A rendered pulse must end on its own: schedule the fallback
		// refresh at its deadline (contract §4.2).
		if p != nil {
			if wait := p.deadline.Sub(w.lastApply); wait > 0 {
				w.armWake(wait, true)
			} else {
				w.mu.Lock()
				w.refreshDue = true
				w.mu.Unlock()
			}
		}
		// Loop: a notify may have landed during Apply; it will hit the
		// throttle branch and re-arm a timer, or return if nothing is due.
	}
}

// armWake schedules a wake-up after d. markDue=true also flags a refresh
// (pulse expiry); false only re-signals an already-due refresh (throttle).
// One pending timer at a time: a new one replaces the previous — the
// earliest need is always re-evaluated by drain on the next pass, and a
// pulse deadline never precedes the throttle it follows.
func (w *Writer) armWake(d time.Duration, markDue bool) {
	if w.cancelTimer != nil {
		w.cancelTimer()
	}
	w.cancelTimer = w.newTimer(d, func() {
		if markDue {
			w.mu.Lock()
			w.refreshDue = true
			w.mu.Unlock()
		}
		select {
		case w.wake <- struct{}{}:
		default:
		}
	})
}
