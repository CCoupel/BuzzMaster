package server

import (
	"sync"
	"time"
)

// evictionEntryTTL bounds how long a recorded eviction reason stays
// answerable — "de l'ordre de l'heure" per the plan (#123 B3): long enough to
// cover a player reloading the page minutes later, short enough that this
// never becomes a de-facto history.
const evictionEntryTTL = time.Hour

// evictionRegistryMaxSize caps the registry so a very long, very active
// session can never grow it unbounded. Generous relative to any realistic
// VJoueur roster (tens of players): oldest entries are evicted first once
// the cap is hit.
const evictionRegistryMaxSize = 500

// EvictionRegistry remembers, for a short bounded window, why a VJoueur's
// bumper was removed — a "PLAYER_CONNECT" carrying that now-unknown ID can
// then be told the real reason (PLAYER_REMOVED or GAME_RESET) instead of
// falling through to a generic ENROLLMENT_CLOSED guess (#123 B3, closes #118
// R3 along the way: a player disconnected at the moment of a NEW_GAME purge
// learns the true reason on return).
//
// Not a history: bounded by TTL, size cap, and a full Reset() on NEW_GAME.
// Safe to use on a nil *EvictionRegistry — every method no-ops/returns a
// zero value instead of panicking, so callers (including tests that build an
// App without wiring one up) never need a nil check of their own.
type EvictionRegistry struct {
	mu      sync.Mutex
	entries map[string]evictionEntry
}

type evictionEntry struct {
	Reason string
	At     time.Time
}

// NewEvictionRegistry creates an empty registry.
func NewEvictionRegistry() *EvictionRegistry {
	return &EvictionRegistry{entries: make(map[string]evictionEntry)}
}

// Record remembers that playerID was just evicted for reason. A no-op if the
// registry is nil or playerID is empty.
func (r *EvictionRegistry) Record(playerID, reason string) {
	if r == nil || playerID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.purgeExpiredLocked()
	if len(r.entries) >= evictionRegistryMaxSize {
		r.evictOldestLocked()
	}
	r.entries[playerID] = evictionEntry{Reason: reason, At: time.Now()}
}

// Lookup returns the recorded eviction reason for playerID, if any and not
// expired. ok is false if the registry is nil, playerID is empty, no entry
// exists, or the entry has aged past evictionEntryTTL (lazily purged here).
func (r *EvictionRegistry) Lookup(playerID string) (reason string, ok bool) {
	if r == nil || playerID == "" {
		return "", false
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.entries[playerID]
	if !exists {
		return "", false
	}
	if time.Since(entry.At) > evictionEntryTTL {
		delete(r.entries, playerID)
		return "", false
	}
	return entry.Reason, true
}

// Reset clears the registry entirely. Called on NEW_GAME: a fresh game
// invalidates any eviction context left over from before it — the caller is
// expected to Record() fresh GAME_RESET entries for this NEW_GAME's own
// purge immediately afterward, so reconnecting players still get an answer.
// Safe to call on a nil registry.
func (r *EvictionRegistry) Reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = make(map[string]evictionEntry)
}

// Len returns the current entry count. Test-only introspection.
func (r *EvictionRegistry) Len() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}

// purgeExpiredLocked removes every entry older than evictionEntryTTL.
// Caller must hold r.mu.
func (r *EvictionRegistry) purgeExpiredLocked() {
	now := time.Now()
	for id, e := range r.entries {
		if now.Sub(e.At) > evictionEntryTTL {
			delete(r.entries, id)
		}
	}
}

// evictOldestLocked removes the single oldest entry (by recorded time), used
// to enforce evictionRegistryMaxSize. Caller must hold r.mu.
func (r *EvictionRegistry) evictOldestLocked() {
	var oldestID string
	var oldestAt time.Time
	first := true
	for id, e := range r.entries {
		if first || e.At.Before(oldestAt) {
			oldestID = id
			oldestAt = e.At
			first = false
		}
	}
	if oldestID != "" {
		delete(r.entries, oldestID)
	}
}
