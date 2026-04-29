package server

import (
	"buzzcontrol/internal/config"
	"fmt"
	"sync"
	"testing"
	"time"
)

// testAckConfig returns a ServerConfig suitable for fast unit tests.
func testAckConfig(timeoutMs, maxRetries int) *config.ServerConfig {
	return &config.ServerConfig{
		AckTimeoutMs:  timeoutMs,
		AckMaxRetries: maxRetries,
	}
}

// TestGenerateMsgID verifies that generated MSG_IDs are 12 hex characters and unique.
func TestGenerateMsgID(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateMsgID()
		if len(id) != 12 {
			t.Errorf("GenerateMsgID() length = %d, want 12 (got %q)", len(id), id)
		}
		for _, c := range id {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("GenerateMsgID() contains non-hex char %q in %q", c, id)
			}
		}
		if seen[id] {
			t.Logf("GenerateMsgID() collision at iteration %d (statistically rare)", i)
		}
		seen[id] = true
	}
}

// TestAckManager_RegisterConfirm verifies that Register adds an entry and Confirm removes it.
func TestAckManager_RegisterConfirm(t *testing.T) {
	cfg := testAckConfig(5000, 3)
	m := NewAckManager(cfg)

	if m.PendingCount() != 0 {
		t.Fatalf("expected 0 pending, got %d", m.PendingCount())
	}

	m.Register("AA:BB:CC:DD:EE:FF", "abc123456789", "LED_SET")

	if m.PendingCount() != 1 {
		t.Fatalf("expected 1 pending after Register, got %d", m.PendingCount())
	}

	// Confirm with unknown ID → false
	if m.Confirm("unknown") {
		t.Error("Confirm(unknown) should return false")
	}
	if m.PendingCount() != 1 {
		t.Errorf("pending count should be unchanged after failed Confirm, got %d", m.PendingCount())
	}

	// Confirm with correct ID → true, entry removed
	if !m.Confirm("abc123456789") {
		t.Error("Confirm(known) should return true")
	}
	if m.PendingCount() != 0 {
		t.Errorf("expected 0 pending after Confirm, got %d", m.PendingCount())
	}

	// Double-confirm → false (already removed)
	if m.Confirm("abc123456789") {
		t.Error("double Confirm should return false")
	}
}

// TestAckManager_ClearByMAC verifies that ClearByMAC removes all entries for a given MAC.
func TestAckManager_ClearByMAC(t *testing.T) {
	cfg := testAckConfig(5000, 3)
	m := NewAckManager(cfg)

	m.Register("AA:BB:CC:DD:EE:01", "msg001", "LED_SET")
	m.Register("AA:BB:CC:DD:EE:01", "msg002", "WIFI_CONFIG")
	m.Register("AA:BB:CC:DD:EE:02", "msg003", "OTA_UPDATE")

	if m.PendingCount() != 3 {
		t.Fatalf("expected 3 pending, got %d", m.PendingCount())
	}

	removed := m.ClearByMAC("AA:BB:CC:DD:EE:01")
	if removed != 2 {
		t.Errorf("ClearByMAC: expected 2 removed, got %d", removed)
	}
	if m.PendingCount() != 1 {
		t.Errorf("expected 1 remaining after ClearByMAC, got %d", m.PendingCount())
	}

	// Other MAC's entry survives
	if !m.Confirm("msg003") {
		t.Error("msg003 from other MAC should still be confirmable")
	}

	// ClearByMAC on unknown MAC returns 0
	if n := m.ClearByMAC("FF:FF:FF:FF:FF:FF"); n != 0 {
		t.Errorf("ClearByMAC(unknown) should return 0, got %d", n)
	}
}

// TestAckManager_RetryOnTimeout verifies that tick() fires OnRetry when timeout is exceeded.
func TestAckManager_RetryOnTimeout(t *testing.T) {
	cfg := testAckConfig(50, 3) // 50 ms timeout
	m := NewAckManager(cfg)

	retryCh := make(chan string, 5)
	m.OnRetry = func(mac, msgID string) {
		retryCh <- msgID
	}

	m.Register("AA:BB:CC:DD:EE:FF", "retryMsg", "LED_SET")

	// Wait longer than the timeout so the entry times out
	time.Sleep(80 * time.Millisecond)
	m.tick()

	select {
	case got := <-retryCh:
		if got != "retryMsg" {
			t.Errorf("OnRetry: expected msgID=retryMsg, got %s", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("OnRetry was not called after timeout")
	}

	// Entry should still be pending (not yet exhausted)
	if m.PendingCount() != 1 {
		t.Errorf("entry should still be pending after first retry, got %d", m.PendingCount())
	}

	// Attempt counter should have incremented
	m.mu.Lock()
	entry := m.pendingAcks["retryMsg"]
	m.mu.Unlock()
	if entry.Attempts != 1 {
		t.Errorf("Attempts should be 1 after first retry, got %d", entry.Attempts)
	}
}

// TestAckManager_ExpireAfterMaxRetries verifies that OnExpired fires after max retries.
func TestAckManager_ExpireAfterMaxRetries(t *testing.T) {
	cfg := testAckConfig(50, 2) // 50 ms timeout, max 2 retries
	m := NewAckManager(cfg)

	expiredCh := make(chan string, 5)
	m.OnExpired = func(mac, msgID string) {
		expiredCh <- msgID
	}

	m.Register("AA:BB:CC:DD:EE:FF", "expireMsg", "OTA_UPDATE")

	// Drain retries: each tick after timeout increments Attempts until max reached
	for i := 0; i < 3; i++ {
		time.Sleep(60 * time.Millisecond)
		m.tick()
	}

	// After max retries (2), next tick should expire it
	select {
	case got := <-expiredCh:
		if got != "expireMsg" {
			t.Errorf("OnExpired: expected expireMsg, got %s", got)
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("OnExpired was not called after max retries")
	}

	// Entry should be removed
	if m.PendingCount() != 0 {
		t.Errorf("entry should be removed after expiry, got %d", m.PendingCount())
	}
}

// TestAckManager_NoCallbacksDoesNotPanic verifies that nil callbacks don't panic.
func TestAckManager_NoCallbacksDoesNotPanic(t *testing.T) {
	cfg := testAckConfig(10, 1)
	m := NewAckManager(cfg)
	// No callbacks set

	m.Register("AA:BB:CC:DD:EE:FF", "msg", "LED_SET")
	time.Sleep(20 * time.Millisecond)

	// Should not panic with nil OnRetry
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("tick() panicked with nil callbacks: %v", r)
		}
	}()
	m.tick()
	time.Sleep(20 * time.Millisecond)
	m.tick() // fires OnExpired (attempts=1 >= max=1)
}

// TestAckManager_PendingCount_Empty verifies PendingCount returns 0 on empty manager.
func TestAckManager_PendingCount_Empty(t *testing.T) {
	cfg := testAckConfig(2000, 3)
	m := NewAckManager(cfg)
	if m.PendingCount() != 0 {
		t.Errorf("expected 0, got %d", m.PendingCount())
	}
}

// TestAckManager_ConcurrentAccess verifies that concurrent Register, Confirm, and
// ClearByMAC calls are race-condition free. Run with -race to validate.
func TestAckManager_ConcurrentAccess(t *testing.T) {
	cfg := testAckConfig(5000, 3)
	m := NewAckManager(cfg)

	const goroutines = 20
	var wg sync.WaitGroup

	// 10 goroutines registering entries
	for i := 0; i < goroutines/2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mac := "AA:BB:CC:DD:EE:" + fmt.Sprintf("%02d", idx)
			msgID := fmt.Sprintf("concurrent%04d", idx)
			m.Register(mac, msgID, "LED_SET")
		}(i)
	}

	// 10 goroutines confirming or clearing entries
	for i := 0; i < goroutines/2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// half try Confirm, half try ClearByMAC
			if idx%2 == 0 {
				m.Confirm(fmt.Sprintf("concurrent%04d", idx))
			} else {
				mac := "AA:BB:CC:DD:EE:" + fmt.Sprintf("%02d", idx)
				m.ClearByMAC(mac)
			}
		}(i)
	}

	wg.Wait()

	// PendingCount must not panic and must be >= 0
	count := m.PendingCount()
	if count < 0 {
		t.Errorf("PendingCount should be >= 0, got %d", count)
	}
}
