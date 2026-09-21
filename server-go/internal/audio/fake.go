package audio

import (
	"context"
	"io"
	"sync"
	"time"
)

// FakeOutput records every Play call — for tests only, never touches real
// hardware. Play does not receive the Cue identity (same limitation as the
// real interface, contract §4) — Played() decodes it back from the
// placeholder payload the engine currently writes (§227, no real bank
// before #229 — see Output's doc comment in cue.go).
type FakeOutput struct {
	mu     sync.Mutex
	plays  [][]byte
	closed bool

	// Err, if non-nil, is returned by every Play call — for exercising
	// Engine.Stats().PlayErrors without a real failing device.
	Err error
	// Delay, if > 0, makes Play wait this long before returning — for
	// exercising latency-sensitive call sites without real hardware.
	Delay time.Duration
	// Gate, if non-nil, makes Play block reading from it before returning
	// (closing Gate releases every blocked and future call) — mirrors
	// spike/audio/robustness.go's wedgedWriter for contract §5.3 tests.
	Gate chan struct{}
}

// NewFakeOutput builds a ready-to-use FakeOutput (no knobs engaged: Play
// records and returns nil immediately).
func NewFakeOutput() *FakeOutput { return &FakeOutput{} }

func (f *FakeOutput) Play(ctx context.Context, pcm io.Reader) error {
	data, _ := io.ReadAll(pcm)
	f.mu.Lock()
	f.plays = append(f.plays, data)
	f.mu.Unlock()

	if f.Delay > 0 {
		select {
		case <-time.After(f.Delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if f.Gate != nil {
		select {
		case <-f.Gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.Err
}

func (f *FakeOutput) Close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	return nil
}

// Count returns how many times Play has been called so far.
func (f *FakeOutput) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.plays)
}

// Closed reports whether Close was called.
func (f *FakeOutput) Closed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// Played decodes, in order, which Cue reached each Play call — see the
// type doc comment.
func (f *FakeOutput) Played() []Cue {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Cue, len(f.plays))
	for i, d := range f.plays {
		out[i] = Cue(d)
	}
	return out
}
