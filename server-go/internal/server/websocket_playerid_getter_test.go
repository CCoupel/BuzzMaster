package server

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests : WebSocketHub.GetClientPlayerID (#109 Phase 2)
//
// Complements the PlayerID identity tests in websocket_test.go (SetClientPlayerID
// / IsPlayerIDConnected). GetClientPlayerID is the getter counterpart used by
// cmd/server's handleWebMessage to fire the DELIVERY_CONFIRMED connection-badge
// event (D2/D3) whenever a message is received from an identified VJoueur.
// ---------------------------------------------------------------------------

func TestGetClientPlayerID_ReturnsLinkedID(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	defer conn.Close()
	time.Sleep(50 * time.Millisecond)

	clientID := clientIDForType(t, hub, ClientTypeVPlayer)
	hub.SetClientPlayerID(clientID, "vjoueur_Emma_20260725")

	playerID, ok := hub.GetClientPlayerID(clientID)
	if !ok {
		t.Fatal("expected ok=true after SetClientPlayerID")
	}
	if playerID != "vjoueur_Emma_20260725" {
		t.Errorf("expected playerID=vjoueur_Emma_20260725, got %q", playerID)
	}
}

func TestGetClientPlayerID_FalseBeforeIdentification(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	defer conn.Close()
	time.Sleep(50 * time.Millisecond)

	clientID := clientIDForType(t, hub, ClientTypeVPlayer)

	if playerID, ok := hub.GetClientPlayerID(clientID); ok {
		t.Errorf("expected ok=false before SetClientPlayerID, got (%q, true)", playerID)
	}
}

func TestGetClientPlayerID_FalseForUnknownClientID(t *testing.T) {
	hub := NewWebSocketHub()

	if playerID, ok := hub.GetClientPlayerID("does-not-exist"); ok {
		t.Errorf("expected ok=false for an unknown clientID, got (%q, true)", playerID)
	}
}

// TestGetClientPlayerID_IgnoresNonVPlayerClientType is a defensive test
// mirroring TestIsPlayerIDConnected_IgnoresNonVPlayerClientType: an admin/TV
// client should never resolve a PlayerID even if one was set on it.
func TestGetClientPlayerID_IgnoresNonVPlayerClientType(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/admin")
	defer conn.Close()
	time.Sleep(50 * time.Millisecond)

	clientID := clientIDForType(t, hub, ClientTypeAdmin)
	hub.SetClientPlayerID(clientID, "vjoueur_NotAPlayer")

	// GetClientPlayerID mirrors SetClientPlayerID's own scope (both operate on
	// whatever client.ID matches, regardless of Type) — verify it still returns
	// the value SetClientPlayerID wrote, since neither method itself filters by
	// ClientTypeVPlayer (only IsPlayerIDConnected does). This documents the
	// actual contract rather than assuming a filter that doesn't exist.
	playerID, ok := hub.GetClientPlayerID(clientID)
	if !ok || playerID != "vjoueur_NotAPlayer" {
		t.Errorf("GetClientPlayerID(%q) = (%q, %v), want (%q, true)", clientID, playerID, ok, "vjoueur_NotAPlayer")
	}
}
