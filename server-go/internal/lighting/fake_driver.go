package lighting

import (
	"context"
	"sync"
	"time"
)

// AppliedState is one State received by FakeDriver, with its receive time.
type AppliedState struct {
	At    time.Time
	State State
}

// FakeDriver records every State it receives. It is the only driver shipped
// by #205 (contract §9): it makes the whole mechanism testable without any
// hardware. Safe for concurrent use even though the writer never needs it.
//
// Knobs (set before use):
//   - Delay: Apply sleeps this long (slow hardware).
//   - Err:   Apply returns this error after recording.
//   - Gate:  if non-nil, Apply blocks until Gate is closed or receives — lets
//     a test prove Notify* never blocks while Apply is in flight.
type FakeDriver struct {
	Delay time.Duration
	Err   error
	Gate  chan struct{}

	mu      sync.Mutex
	applied []AppliedState
	closed  bool
	now     func() time.Time
}

// NewFakeDriver returns an empty recorder.
func NewFakeDriver() *FakeDriver { return &FakeDriver{now: time.Now} }

// Apply records s (blocking on Gate / Delay first if configured).
func (f *FakeDriver) Apply(ctx context.Context, s State) error {
	if f.Gate != nil {
		select {
		case <-f.Gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if f.Delay > 0 {
		select {
		case <-time.After(f.Delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f.mu.Lock()
	f.applied = append(f.applied, AppliedState{At: f.now(), State: cloneState(s)})
	err := f.Err
	f.mu.Unlock()
	return err
}

// Close marks the driver closed (idempotent).
func (f *FakeDriver) Close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	return nil
}

// Closed reports whether Close was called.
func (f *FakeDriver) Closed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// Count returns the number of Apply calls that completed.
func (f *FakeDriver) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.applied)
}

// States returns a copy of every applied State, in order.
func (f *FakeDriver) States() []State {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]State, len(f.applied))
	for i, a := range f.applied {
		out[i] = cloneState(a.State)
	}
	return out
}

// Applied returns a copy of every applied State with its timestamp.
func (f *FakeDriver) Applied() []AppliedState {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]AppliedState, len(f.applied))
	for i, a := range f.applied {
		out[i] = AppliedState{At: a.At, State: cloneState(a.State)}
	}
	return out
}

// Last returns the most recent State, ok=false if none.
func (f *FakeDriver) Last() (State, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.applied) == 0 {
		return State{}, false
	}
	return cloneState(f.applied[len(f.applied)-1].State), true
}

func cloneState(s State) State {
	return State{Zones: append([]ZoneState(nil), s.Zones...)}
}
