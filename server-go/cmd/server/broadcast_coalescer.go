package main

import (
	"sync"
	"time"
)

// coalescerTimerFactory starts a one-shot timer that calls fn after d and
// returns a function that cancels it (idempotent, safe to call even if the
// timer already fired — mirrors time.Timer.Stop()'s contract). Injectable so
// BroadcastCoalescer's tests run deterministically, without a real clock
// (#129 T2.1: "minuteur injectable pour tests déterministes, pas de
// time.Sleep en test").
type coalescerTimerFactory func(d time.Duration, fn func()) (cancel func())

func defaultCoalescerTimerFactory(d time.Duration, fn func()) func() {
	t := time.AfterFunc(d, fn)
	return func() { t.Stop() }
}

// BroadcastCoalescer collapses a burst of Trigger() calls, within a short
// window, into at most one Emit call (#129 T2.1, T2.2 — the ARDOISE_INPUT
// site: 8 teams typing produce ~40 GetGameJSON() calls/sec, each taking the
// engine lock — round 1 investigation's scenario C). Not specific to
// ARDOISE_INPUT; any Trigger-heavy site could reuse it.
//
// Founding invariant, load-bearing for correctness, not just an
// optimization: the coalescer NEVER buffers a payload. It only remembers
// "an emit is due" — emit is a zero-argument callback that must read live
// state itself when it actually runs (e.g. a.engine.GetGameJSON() at call
// time, not at Trigger() time). A delayed emission is therefore always
// redundant with whatever state existed when it was scheduled — never
// stale, never behind a message that was sent in between. This is what
// makes coalescing safe with respect to message ordering: nothing else in
// this codebase needs to know a coalescer sits in front of a given
// broadcast, because what arrives is always "the current state," a bit
// later than it could have been — same guarantee a human would get by
// simply broadcasting less often.
//
// Safe for concurrent use.
type BroadcastCoalescer struct {
	mu       sync.Mutex
	window   time.Duration
	emit     func()
	newTimer coalescerTimerFactory
	cancel   func() // non-nil while a timer is pending; nil otherwise
}

// NewBroadcastCoalescer creates a coalescer that calls emit at most once per
// window: the first Trigger() in a burst arms a window-long timer; every
// Trigger() call while that timer is still pending is coalesced (a no-op
// beyond confirming an emit is already due).
func NewBroadcastCoalescer(window time.Duration, emit func()) *BroadcastCoalescer {
	return &BroadcastCoalescer{
		window:   window,
		emit:     emit,
		newTimer: defaultCoalescerTimerFactory,
	}
}

// Trigger requests an emit within the next window. Coalesced (a no-op) if
// one is already pending — that's the entire mechanism: N calls within a
// window produce exactly one emit, timed off the FIRST call in the burst.
func (c *BroadcastCoalescer) Trigger() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		return // already pending — this call is coalesced into it
	}
	c.cancel = c.newTimer(c.window, c.fire)
}

// fire is the timer callback: clears the pending state, then emits outside
// the lock (emit may itself call back into this coalescer, e.g. via a
// concurrent Trigger from another goroutine — must never deadlock).
func (c *BroadcastCoalescer) fire() {
	c.mu.Lock()
	c.cancel = nil
	c.mu.Unlock()
	c.emit()
}

// Flush emits immediately if a Trigger is currently pending (cancelling its
// timer first — the pending window is skipped, not doubled), and is a no-op
// otherwise. Used on every phase change (#129 CA5/CA6): the last few
// keystrokes before REVEAL must be visible immediately, not held back for
// up to `window` behind a phase transition that already carries the
// complete state.
func (c *BroadcastCoalescer) Flush() {
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.mu.Unlock()

	if cancel == nil {
		return // nothing pending
	}
	cancel()
	c.emit()
}

// Stop cancels any pending timer without emitting — for clean shutdown, so
// no time.AfterFunc goroutine is left dangling once the server has told
// everything else to stop.
func (c *BroadcastCoalescer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}
