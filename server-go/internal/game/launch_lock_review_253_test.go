package game

import (
	"sync/atomic"
	"testing"
	"time"
)

// #253 review M1: a reader never releases a past-deadline lock (no silent release);
// the release is delivered (callback = journal + broadcast) by the timer or the next OPEN.
func TestLaunchLock_PastDeadlineIsInactiveForReadersButNotReleasedByThem(t *testing.T) {
	e := NewEngine()
	var expired atomic.Int32
	if _, reason := e.OpenLaunchLock("A", func(LaunchLockReleased) { expired.Add(1) }); reason != "" {
		t.Fatal(reason)
	}
	e.mu.Lock()
	e.state.LaunchLock.ExpiresAt = time.Now().Add(-time.Second).UnixMilli() // deadline passed, timer not fired
	e.mu.Unlock()

	if got := e.LaunchLockOwner(); got != "" {
		t.Fatalf("past deadline = inactive for the gate, got owner %q", got)
	}
	e.mu.RLock()
	stillActive := e.state.LaunchLock.Active
	e.mu.RUnlock()
	if !stillActive {
		t.Fatal("a reader must NOT release the lock (no silent release, timer kept)")
	}
	if expired.Load() != 0 {
		t.Fatal("no callback from a read")
	}

	// the next OPEN (another station) takes over and delivers the pending expiry
	_, reason := e.OpenLaunchLock("B", nil)
	if reason != "" {
		t.Fatalf("an expired lock never blocks another station: %s", reason)
	}
	deadline := time.Now().Add(time.Second)
	for expired.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if expired.Load() != 1 {
		t.Fatalf("the old OPEN's expiry callback must fire once on take-over, got %d", expired.Load())
	}
	if e.LaunchLockOwner() != "B" {
		t.Fatal("B owns the lock")
	}
}

func TestLaunchLock_TimerReleasesAndNotifies(t *testing.T) {
	old := LaunchLockTTL
	LaunchLockTTL = 80 * time.Millisecond
	defer func() { LaunchLockTTL = old }()
	e := NewEngine()
	got := make(chan LaunchLockReleased, 1)
	if _, reason := e.OpenLaunchLock("A", func(r LaunchLockReleased) { got <- r }); reason != "" {
		t.Fatal(reason)
	}
	select {
	case r := <-got:
		if r.Reason != "expired" || r.Owner != "A" {
			t.Fatalf("release = %+v", r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timer did not release the lock")
	}
}
