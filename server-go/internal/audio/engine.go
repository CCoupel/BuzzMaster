package audio

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
)

// DefaultQueueSize is the playback queue's capacity when Config.QueueSize
// is left at 0. Small on purpose: an accepted backlog is audio waiting to
// be heard, not a buffer to grow — contract §5.2 forbids the queue from
// growing dynamically, and a deep queue would just mean stale sounds
// playing out of sync with a game that has already moved on.
const DefaultQueueSize = 8

// Config wires an Engine.
type Config struct {
	// Output nil disables the engine entirely: PlayCue is a no-op, Start
	// returns immediately without spawning anything (contract §5.5).
	Output Output
	// QueueSize overrides DefaultQueueSize (0 = default).
	QueueSize int
}

// Stats is what the engine has done so far (tests and diagnostics) — same
// role as lighting.Writer's Stats.
type Stats struct {
	Accepted   int // PlayCue calls that were queued
	Dropped    int // PlayCue calls rejected — queue was full, never waited on
	Played     int // Output.Play calls attempted
	PlayErrors int
}

// Engine is the sound-bruitage moteur (contract §5): a single playback
// goroutine reading a bounded, non-coalescing FIFO queue. Deliberately NOT
// modelled on lighting.Writer's single-slot "last state wins" register
// (contract sound.md §5.2) — two different cues are never interchangeable,
// so a queue is used instead of an overwritable pulse.
type Engine struct {
	enabled bool
	output  Output
	queue   chan Cue
	running atomic.Bool

	statsMu sync.Mutex
	stats   Stats
}

// NewEngine builds an Engine. Disabled if cfg.Output is nil.
func NewEngine(cfg Config) *Engine {
	size := cfg.QueueSize
	if size <= 0 {
		size = DefaultQueueSize
	}
	return &Engine{
		enabled: cfg.Output != nil,
		output:  cfg.Output,
		queue:   make(chan Cue, size),
	}
}

// Enabled reports whether an Output is attached. Safe on a nil receiver.
func (e *Engine) Enabled() bool { return e != nil && e.enabled }

// Running reports whether Start's goroutine is alive. Safe on a nil receiver.
func (e *Engine) Running() bool { return e != nil && e.running.Load() }

// Stats returns a copy of the counters. Safe on a nil receiver.
func (e *Engine) Stats() Stats {
	if e == nil {
		return Stats{}
	}
	e.statsMu.Lock()
	defer e.statsMu.Unlock()
	return e.stats
}

// PlayCue signals a cue to play. NEVER BLOCKS (contract §5.1) — the only
// send form allowed on the queue is `select { case ch <- v: default: }`,
// exactly like lighting.Writer.NotifyState. Called from the game's
// dispatch goroutine: it must never wait on the queue nor on any device.
// Safe on a nil or disabled engine, so call sites need no guard (mirrors
// contract lighting.md §4.3's "no `if` around a Notify* call").
func (e *Engine) PlayCue(c Cue) (accepted bool) {
	if e == nil || !e.enabled {
		return false
	}
	select {
	case e.queue <- c:
		e.statsMu.Lock()
		e.stats.Accepted++
		e.statsMu.Unlock()
		return true
	default:
		e.statsMu.Lock()
		e.stats.Dropped++
		e.statsMu.Unlock()
		return false
	}
}

// Start runs the single playback goroutine until ctx is done, then closes
// the output. Returns immediately (no goroutine, no Close) when disabled
// (contract §5.5 — "aucune goroutine, pas une qui tourne à vide"). Call as
// `go e.Start(a.ctx)` from (*App).start(), exactly like the lighting
// writer and AckManager before it (contract §5.6).
func (e *Engine) Start(ctx context.Context) {
	if e == nil || !e.Enabled() {
		return
	}
	if !e.running.CompareAndSwap(false, true) {
		return // already running
	}
	defer func() {
		if e.output != nil {
			_ = e.output.Close()
		}
		e.running.Store(false)
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-e.queue:
			e.play(ctx, c)
		}
	}
}

// play plays one cue. #227 has no sound bank yet (#229): the payload is a
// placeholder carrying the Cue's own name, which is all this milestone
// needs to prove the plumbing end to end with a fake Output — see the
// Output doc comment. Never returns an error to the caller: dégradation
// silencieuse (contract §5.5) — an Output error is counted, never
// propagated into the game.
func (e *Engine) play(ctx context.Context, c Cue) {
	err := e.output.Play(ctx, strings.NewReader(string(c)))
	e.statsMu.Lock()
	e.stats.Played++
	if err != nil {
		e.stats.PlayErrors++
	}
	e.statsMu.Unlock()
}
