package server

import (
	"fmt"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests : EvictionRegistry (#123, B3)
//
// Plan : _work/reports/plan-20260730-094500.md — tâche B3. Registre borné
// qui mémorise brièvement le motif de disparition d'un VJoueur (PLAYER_REMOVED
// ou GAME_RESET), consulté par handlePlayerConnect (cmd/server) quand un ID
// fourni est introuvable — pour répondre avec la vraie raison au lieu d'un
// ENROLLMENT_CLOSED générique. Ce n'est PAS un historique : borné par TTL
// (~1h), plafond de taille, et remise à zéro complète sur NEW_GAME.
// ---------------------------------------------------------------------------

func TestEvictionRegistry_RecordAndLookup(t *testing.T) {
	r := NewEvictionRegistry()
	r.Record("vjoueur_alice", "PLAYER_REMOVED")

	reason, ok := r.Lookup("vjoueur_alice")
	if !ok {
		t.Fatal("expected an entry for vjoueur_alice")
	}
	if reason != "PLAYER_REMOVED" {
		t.Errorf("expected reason=PLAYER_REMOVED, got %q", reason)
	}
}

func TestEvictionRegistry_LookupUnknownID_ReturnsFalse(t *testing.T) {
	r := NewEvictionRegistry()
	r.Record("vjoueur_alice", "PLAYER_REMOVED")

	if _, ok := r.Lookup("vjoueur_someone_else"); ok {
		t.Error("expected ok=false for an ID never recorded")
	}
}

// TestEvictionRegistry_LookupExpiredEntry_ReturnsFalse verifies the ~1h TTL:
// an entry recorded more than an hour ago must no longer answer, and gets
// lazily purged from Lookup. Backdates the entry directly (same package,
// unexported fields) rather than waiting a real hour.
func TestEvictionRegistry_LookupExpiredEntry_ReturnsFalse(t *testing.T) {
	r := NewEvictionRegistry()
	r.entries["vjoueur_alice"] = evictionEntry{
		Reason: "PLAYER_REMOVED",
		At:     time.Now().Add(-2 * time.Hour), // well past the ~1h TTL
	}

	if _, ok := r.Lookup("vjoueur_alice"); ok {
		t.Error("expected an entry older than the TTL to no longer answer")
	}
	if r.Len() != 0 {
		t.Errorf("expected the expired entry to be purged by Lookup, Len()=%d", r.Len())
	}
}

// TestEvictionRegistry_LookupFreshEntry_StillAnswers is the boundary sanity
// check: an entry well within the TTL must still answer (guards against an
// overly aggressive expiry check).
func TestEvictionRegistry_LookupFreshEntry_StillAnswers(t *testing.T) {
	r := NewEvictionRegistry()
	r.entries["vjoueur_alice"] = evictionEntry{
		Reason: "GAME_RESET",
		At:     time.Now().Add(-10 * time.Minute), // well within the ~1h TTL
	}

	reason, ok := r.Lookup("vjoueur_alice")
	if !ok {
		t.Fatal("expected an entry well within the TTL to still answer")
	}
	if reason != "GAME_RESET" {
		t.Errorf("expected reason=GAME_RESET, got %q", reason)
	}
}

// TestEvictionRegistry_MaxSize_CapsGrowth verifies the registry never grows
// past its size cap — a very long, very active session must not turn this
// into an unbounded history.
func TestEvictionRegistry_MaxSize_CapsGrowth(t *testing.T) {
	r := NewEvictionRegistry()
	for i := 0; i < evictionRegistryMaxSize+50; i++ {
		r.Record(idFor(i), "PLAYER_REMOVED")
	}

	if r.Len() > evictionRegistryMaxSize {
		t.Errorf("expected Len() to stay capped at %d, got %d", evictionRegistryMaxSize, r.Len())
	}
}

// TestEvictionRegistry_MaxSize_KeepsMostRecentAnswerable is the behavioral
// counterpart: once the cap forces evictions, the entry just recorded must
// still be answerable (only OLDER entries should ever be dropped).
func TestEvictionRegistry_MaxSize_KeepsMostRecentAnswerable(t *testing.T) {
	r := NewEvictionRegistry()
	for i := 0; i < evictionRegistryMaxSize+50; i++ {
		r.Record(idFor(i), "PLAYER_REMOVED")
	}

	lastID := idFor(evictionRegistryMaxSize + 49)
	if _, ok := r.Lookup(lastID); !ok {
		t.Errorf("expected the most recently recorded entry (%s) to survive the size cap", lastID)
	}
}

func TestEvictionRegistry_Reset_ClearsEverything(t *testing.T) {
	r := NewEvictionRegistry()
	r.Record("vjoueur_alice", "PLAYER_REMOVED")
	r.Record("vjoueur_bob", "PLAYER_REMOVED")

	r.Reset()

	if r.Len() != 0 {
		t.Errorf("expected Len()=0 after Reset(), got %d", r.Len())
	}
	if _, ok := r.Lookup("vjoueur_alice"); ok {
		t.Error("expected vjoueur_alice to be gone after Reset()")
	}
	if _, ok := r.Lookup("vjoueur_bob"); ok {
		t.Error("expected vjoueur_bob to be gone after Reset()")
	}
}

// TestEvictionRegistry_NilSafe verifies every method no-ops/returns a zero
// value on a nil *EvictionRegistry instead of panicking — documented
// explicitly so callers (including tests building an App without wiring one
// up) never need their own nil check.
func TestEvictionRegistry_NilSafe(t *testing.T) {
	var r *EvictionRegistry

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("expected no panic on a nil *EvictionRegistry, got: %v", rec)
		}
	}()

	r.Record("vjoueur_alice", "PLAYER_REMOVED")
	if _, ok := r.Lookup("vjoueur_alice"); ok {
		t.Error("expected Lookup on a nil registry to return ok=false")
	}
	r.Reset()
	if got := r.Len(); got != 0 {
		t.Errorf("expected Len()=0 on a nil registry, got %d", got)
	}
}

func idFor(i int) string {
	return fmt.Sprintf("vjoueur_%d", i)
}
