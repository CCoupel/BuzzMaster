package game

import "testing"

// ---------------------------------------------------------------------------
// Tests : InitGame — valeur de retour purgedVPlayerIDs (#120, B1)
//
// InitGame purge intégralement le roster VJoueur sur NEW_GAME (comportement
// déjà couvert par TestInitGame_PurgesAllVirtualPlayers dans
// reconnect_id_test.go, inchangé). La nouveauté #120 est la valeur de retour
// ([]string des IDs purgés), qui permet à main.go de notifier chaque VJoueur
// individuellement (PLAYER_EVICTED{GAME_RESET}) — voir
// player_evicted_test.go (package main) pour la notification elle-même.
//
// La couverture SaveBumpers (#120, B2 — durcissement atomique) vit dans
// save_bumpers_atomic_test.go ; la couverture de concurrence à l'inscription
// vit dans enrollment_concurrency_test.go.
// ---------------------------------------------------------------------------

func TestInitGame_ReturnsPurgedVirtualPlayerIDs(t *testing.T) {
	e := NewEngine()
	e.SetBumpers(map[string]*Bumper{
		"vjoueur_alice":     {Name: "Alice", IsVirtual: true, IsVPlayer: true, Connected: true},
		"vjoueur_bob":       {Name: "Bob", IsVirtual: true, IsVPlayer: true, Connected: false},
		"AA:BB:CC:DD:EE:01": {Name: "Buzzer1", IsVirtual: false, Connected: true},
	})

	purgedIDs := e.InitGame()

	if len(purgedIDs) != 2 {
		t.Fatalf("expected 2 purged VJoueur IDs, got %d: %v", len(purgedIDs), purgedIDs)
	}
	got := map[string]bool{}
	for _, id := range purgedIDs {
		got[id] = true
	}
	if !got["vjoueur_alice"] || !got["vjoueur_bob"] {
		t.Errorf("expected purgedIDs to contain both vjoueur_alice and vjoueur_bob, got %v", purgedIDs)
	}
	if got["AA:BB:CC:DD:EE:01"] {
		t.Errorf("physical buzzer must never appear in purgedIDs, got %v", purgedIDs)
	}
}

// TestInitGame_ReturnsEmptySlice_WhenNoVirtualPlayers is the edge case: a
// game with no VJoueur at all (physical buzzers only, or nothing) must not
// return a spurious non-empty slice — the caller uses len(purgedIDs) > 0 to
// decide whether to emit anything at all.
func TestInitGame_ReturnsEmptySlice_WhenNoVirtualPlayers(t *testing.T) {
	e := NewEngine()
	e.SetBumpers(map[string]*Bumper{
		"AA:BB:CC:DD:EE:01": {Name: "Buzzer1", IsVirtual: false},
	})

	purgedIDs := e.InitGame()

	if len(purgedIDs) != 0 {
		t.Errorf("expected no purged VJoueur IDs (no VJoueur existed), got %v", purgedIDs)
	}
}
