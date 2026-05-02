package server

import (
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// GenerateMsgID creates a random 12-character hex string used as MSG_ID for ACK tracking.
// It is safe to call concurrently.
func GenerateMsgID() string {
	return fmt.Sprintf("%012x", rand.Int63()&0xffffffffffff)
}

// AckEntry tracks a single unacknowledged priority message sent to a buzzer.
type AckEntry struct {
	MAC      string    // Buzzer MAC address
	Action   string    // Action that was sent (LED_SET, OTA_UPDATE, WIFI_CONFIG)
	Attempts int       // Number of send attempts (0 = initial, 1+ = retries)
	SentAt   time.Time // Time the most recent attempt was made
}

// AckManager tracks priority messages sent to buzzers and handles retry/expiry logic.
// When a buzzer does not ACK within ack_timeout_ms, the message is retried up to
// ack_max_retries times. After that, the entry is expired.
//
// Thread-safe: all public methods use internal locking.
type AckManager struct {
	pendingAcks map[string]AckEntry // msgID → entry
	mu          sync.Mutex
	cfg         *config.ServerConfig

	// OnRetry is called when a message needs to be re-sent (attempts < max_retries).
	// The caller should re-send the original message with the same msgID.
	OnRetry func(mac, msgID string)

	// OnExpired is called when max retries are exhausted without an ACK.
	// The caller should clear AckPending on the bumper.
	OnExpired func(mac, msgID string)
}

// NewAckManager creates a new AckManager with the given server config.
// A zero or negative AckTimeoutMs is normalized to 2000ms to prevent
// time.NewTicker panics when config.json is absent or malformed.
func NewAckManager(cfg *config.ServerConfig) *AckManager {
	if cfg.AckTimeoutMs <= 0 {
		cfg.AckTimeoutMs = 2000
	}
	if cfg.AckMaxRetries <= 0 {
		cfg.AckMaxRetries = 3
	}
	return &AckManager{
		pendingAcks: make(map[string]AckEntry),
		cfg:         cfg,
	}
}

// Register records a new outgoing priority message that expects an ACK.
// msgID is the MSG_ID field on the sent message.
func (m *AckManager) Register(mac, msgID, action string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingAcks[msgID] = AckEntry{
		MAC:      mac,
		Action:   action,
		Attempts: 0,
		SentAt:   time.Now(),
	}
}

// Confirm removes the pending ACK entry for the given msgID.
// Returns true if the entry existed (i.e. this was a valid ACK), false if unknown.
func (m *AckManager) Confirm(msgID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.pendingAcks[msgID]
	if exists {
		delete(m.pendingAcks, msgID)
	}
	return exists
}

// ClearByMAC removes all pending ACK entries for a given buzzer MAC address.
// Called when a buzzer disconnects to avoid stale retries.
func (m *AckManager) ClearByMAC(mac string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	removed := 0
	for msgID, entry := range m.pendingAcks {
		if entry.MAC == mac {
			delete(m.pendingAcks, msgID)
			removed++
		}
	}
	return removed
}

// PendingCount returns the number of messages currently awaiting ACK.
func (m *AckManager) PendingCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pendingAcks)
}

// Start launches the AckManager background goroutine that checks for timeouts.
// It ticks every ack_timeout_ms milliseconds and processes expired entries.
// Exits when ctx is cancelled.
func (m *AckManager) Start(ctx context.Context) {
	timeout := time.Duration(m.cfg.AckTimeoutMs) * time.Millisecond
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tick()
		}
	}
}

// tick checks all pending entries for timeout and fires the appropriate callback.
func (m *AckManager) tick() {
	timeout := time.Duration(m.cfg.AckTimeoutMs) * time.Millisecond

	m.mu.Lock()
	// Collect entries that have timed out (avoid calling callbacks under the lock)
	type expiredEntry struct {
		msgID string
		entry AckEntry
	}
	var expired []expiredEntry
	var toRetry []expiredEntry

	for msgID, entry := range m.pendingAcks {
		if time.Since(entry.SentAt) < timeout {
			continue // Not timed out yet
		}

		if entry.Attempts >= m.cfg.AckMaxRetries {
			// Max retries exhausted — expire
			delete(m.pendingAcks, msgID)
			expired = append(expired, expiredEntry{msgID, entry})
		} else {
			// Increment attempt count and update timestamp for next tick
			entry.Attempts++
			entry.SentAt = time.Now()
			m.pendingAcks[msgID] = entry
			toRetry = append(toRetry, expiredEntry{msgID, entry})
		}
	}
	m.mu.Unlock()

	// Fire callbacks outside the lock
	for _, e := range toRetry {
		LogInfo(game.LogComponentApp, "AckManager: retry %d/%d for msgID=%s mac=%s action=%s",
			e.entry.Attempts, m.cfg.AckMaxRetries, e.msgID, e.entry.MAC, e.entry.Action)
		if m.OnRetry != nil {
			m.OnRetry(e.entry.MAC, e.msgID)
		}
	}
	for _, e := range expired {
		LogWarn(game.LogComponentApp, "AckManager: EXPIRED after %d retries for msgID=%s mac=%s action=%s",
			m.cfg.AckMaxRetries, e.msgID, e.entry.MAC, e.entry.Action)
		if m.OnExpired != nil {
			m.OnExpired(e.entry.MAC, e.msgID)
		}
	}
}
