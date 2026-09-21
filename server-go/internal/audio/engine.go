package audio

import (
	"bytes"
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

// Bank resolves a Cue to its canonical PCM bytes (contract §3: WAV PCM
// 16-bit, 44100 Hz, stereo, header already stripped — Output.Play only
// ever sees raw samples). #229's FileBank (bank.go) is the real
// implementation, reading `data/files/sounds/<cue>.wav` from disk; disk is
// authoritative, a Bank never caches (contract sound.md §9 cadrage's "le
// disque fait foi"). No entry for a Cue is a silent no-op (contract §5.5).
type Bank interface {
	PCM(c Cue) (data []byte, ok bool)
}

// Config wires an Engine.
type Config struct {
	// Output nil disables the engine entirely: PlayCue is a no-op, Start
	// returns immediately without spawning anything (contract §5.5).
	Output Output
	// Bank resolves each Cue to real PCM bytes (#229). nil keeps #227's
	// placeholder behaviour — the Cue's own name is fed to Output.Play as a
	// stand-in payload — which is what lets #227's own tests
	// (engine_test.go, play_blocks_228_test.go) keep passing unchanged:
	// they were written and coordinated before any real sound bank existed
	// ("aucun vrai catalogue WAV n'existe avant #229", cue.go's own doc
	// comment) and never set this field.
	Bank Bank
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
	NoAsset    int // Bank had no PCM for the requested Cue — silent no-op (contract §5.5)
}

// Engine is the sound-bruitage moteur (contract §5): a single playback
// goroutine reading a bounded, non-coalescing FIFO queue. Deliberately NOT
// modelled on lighting.Writer's single-slot "last state wins" register
// (contract sound.md §5.2) — two different cues are never interchangeable,
// so a queue is used instead of an overwritable pulse.
type Engine struct {
	enabled bool
	output  Output
	bank    Bank
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
		bank:    cfg.Bank,
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

// play plays one cue. With a Bank configured (#229), resolves real PCM
// bytes from disk; a Cue with no asset is a silent no-op (contract §5.5),
// counted in Stats.NoAsset, never propagated. Without a Bank (nil — #227's
// placeholder, kept for tests written before #229 existed), the payload is
// the Cue's own name — see Bank's doc comment. Never returns an error to
// the caller either way: dégradation silencieuse (contract §5.5) — an
// Output error is counted, never propagated into the game.
func (e *Engine) play(ctx context.Context, c Cue) {
	var pcm []byte
	fromBank := false
	if e.bank != nil {
		data, ok := e.bank.PCM(c)
		if !ok {
			e.statsMu.Lock()
			e.stats.NoAsset++
			e.statsMu.Unlock()
			return
		}
		pcm, fromBank = data, true
	}

	var err error
	if fromBank {
		err = e.output.Play(ctx, bytes.NewReader(pcm))
	} else {
		err = e.output.Play(ctx, strings.NewReader(string(c)))
	}
	e.statsMu.Lock()
	e.stats.Played++
	if err != nil {
		e.stats.PlayErrors++
	}
	e.statsMu.Unlock()
}
