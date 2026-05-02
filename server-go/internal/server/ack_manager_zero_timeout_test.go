package server

import (
	"context"
	"testing"
	"time"

	"buzzcontrol/internal/config"
)

// TestAckManagerStart_ZeroTimeout_NoPanic is a non-regression test for the panic:
//
//	panic: non-positive interval for NewTicker
//
// Root cause: AckManager.Start() called time.NewTicker(0) when AckTimeoutMs was 0.
// This happened when config.json was absent and config.Get() fell back to a zero-value
// ServerConfig without applying ACK defaults (AckTimeoutMs was left at 0).
//
// Fix (bugfix/ack-manager-zero-timeout): guard added in Start() —
//
//	if timeout <= 0 { timeout = 2 * time.Second }
func TestAckManagerStart_ZeroTimeout_NoPanic(t *testing.T) {
	// Reproduce exact bug condition: AckTimeoutMs=0 (unset, missing config.json scenario).
	cfg := &config.ServerConfig{
		AckTimeoutMs:  0,
		AckMaxRetries: 3,
	}
	m := NewAckManager(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start() is run in a goroutine. Any panic is captured and forwarded to the test.
	// time.NewTicker(0) panics synchronously, so 100 ms is more than enough to detect it.
	panicked := make(chan interface{}, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicked <- r
			}
		}()
		m.Start(ctx)
	}()

	select {
	case r := <-panicked:
		t.Fatalf("AckManager.Start() panicked with AckTimeoutMs=0: %v", r)
	case <-time.After(100 * time.Millisecond):
		// No panic within the observation window — the fix is effective.
	}

	// Cancel context to stop the background goroutine cleanly.
	cancel()
}

// TestAckManagerStart_NegativeTimeout_NoPanic verifies that a negative AckTimeoutMs
// (e.g. malformed config.json with -1) is also handled by the same guard.
func TestAckManagerStart_NegativeTimeout_NoPanic(t *testing.T) {
	cfg := &config.ServerConfig{
		AckTimeoutMs:  -100,
		AckMaxRetries: 3,
	}
	m := NewAckManager(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	panicked := make(chan interface{}, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicked <- r
			}
		}()
		m.Start(ctx)
	}()

	select {
	case r := <-panicked:
		t.Fatalf("AckManager.Start() panicked with AckTimeoutMs=-100: %v", r)
	case <-time.After(100 * time.Millisecond):
		// No panic — guard covers negative values too.
	}

	cancel()
}

// TestAckManagerStart_ContextCancelStops verifies that cancelling the context
// terminates Start() cleanly without blocking, regardless of the timeout value.
func TestAckManagerStart_ContextCancelStops(t *testing.T) {
	cfg := &config.ServerConfig{
		AckTimeoutMs:  0, // zero timeout — uses 2s fallback internally
		AckMaxRetries: 3,
	}
	m := NewAckManager(cfg)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()

	// Cancel immediately after starting.
	cancel()

	select {
	case <-done:
		// Start() returned cleanly after ctx cancel — expected.
	case <-time.After(500 * time.Millisecond):
		t.Error("AckManager.Start() did not exit within 500ms after context cancellation")
	}
}
