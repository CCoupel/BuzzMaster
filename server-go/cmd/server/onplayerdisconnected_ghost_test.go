package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// Test : garde anti-fantôme dans onPlayerDisconnected (code-review, suite au
// fix R1 purge NEW_GAME — _work/handoff/task-dev-backend-20260725-183000.md)
//
// Scénario : admin lance NEW_GAME pendant qu'un VJoueur est connecté -> son
// bumper est purgé (purge inconditionnelle sur InitGame) -> son navigateur
// est redirigé mais son WebSocket reste ouvert un instant (le PlayerID du
// client pointe toujours vers l'ID purgé) -> s'il ferme l'onglet ou perd le
// réseau avant de se ré-enrôler, OnPlayerDisconnected se déclenche avec un ID
// périmé. Sans garde, UpdateBumper (comportement générique pensé pour
// l'auto-enregistrement des buzzers physiques par MAC) recrée un bumper vide
// -> fantôme persisté sur disque.
// ---------------------------------------------------------------------------

// TestOnPlayerDisconnected_PurgedBumper_DoesNotResurrectGhost is the core
// regression test: disconnecting a PlayerID whose bumper no longer exists
// (purged) must not recreate it.
func TestOnPlayerDisconnected_PurgedBumper_DoesNotResurrectGhost(t *testing.T) {
	app := newTestAppWithHub(t)

	purgedID := "vjoueur_purged_by_new_game"
	if app.engine.GetBumper(purgedID) != nil {
		t.Fatalf("setup failed: bumper %q should not exist yet", purgedID)
	}

	app.onPlayerDisconnected(purgedID)

	if got := app.engine.GetBumper(purgedID); got != nil {
		t.Errorf("code-review regression: onPlayerDisconnected resurrected a ghost bumper for a purged PlayerID: %+v", got)
	}
}

// TestOnPlayerDisconnected_ExistingBumper_SetsDisconnected is the inverse
// sanity check: the guard must not break the normal disconnect path for a
// bumper that still genuinely exists.
func TestOnPlayerDisconnected_ExistingBumper_SetsDisconnected(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)

	id, _, err := app.engine.CreateVirtualPlayer("Fiona")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	if !app.engine.GetBumper(id).Connected {
		t.Fatalf("setup failed: expected Connected==true right after creation")
	}

	app.onPlayerDisconnected(id)

	bumper := app.engine.GetBumper(id)
	if bumper == nil {
		t.Fatalf("bumper %q unexpectedly disappeared", id)
	}
	if bumper.Connected {
		t.Errorf("expected Connected==false after onPlayerDisconnected, got true")
	}
}

// TestOnPlayerDisconnected_ZombieGuard_SkipsIfReconnected verifies the
// existing #109 anti-zombie guard still works after extracting the closure
// into a named method: if another (new) client already reconnected under the
// same PlayerID before the old client's disconnect callback fires, the old
// disconnect must be a no-op — it must not flip Connected back to false and
// clobber the live reconnection.
//
// Uses two real WebSocket connections (SetClientPlayerID/IsPlayerIDConnected
// require a genuinely registered hub client — see
// player_connect_connstate_phase2_test.go for the same pattern).
func TestOnPlayerDisconnected_ZombieGuard_SkipsIfReconnected(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)

	id, _, err := app.engine.CreateVirtualPlayer("Gino")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}

	go app.wsHub.Run()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.wsHub.HandleConnectionWithType(w, r, server.ClientTypeVPlayer)
	}))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/player"

	learnClientID := func() string {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to dial test WS server: %v", err)
		}
		t.Cleanup(func() { conn.Close() })
		msg, err := protocol.NewMessage("PONG", map[string]interface{}{})
		if err != nil {
			t.Fatalf("failed to build message: %v", err)
		}
		data, err := msg.SerializeForWebSocket()
		if err != nil {
			t.Fatalf("failed to serialize message: %v", err)
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			t.Fatalf("failed to send WS message: %v", err)
		}
		select {
		case incoming := <-app.wsHub.Incoming:
			return incoming.ClientID
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for an incoming message")
			return ""
		}
	}

	oldClientID := learnClientID()
	newClientID := learnClientID()

	app.wsHub.SetClientPlayerID(oldClientID, id)
	app.wsHub.SetClientPlayerID(newClientID, id) // the "reconnection" that took over

	// Simulate the new client's own reconnect flow already having marked the
	// bumper connected again (main.go's real PLAYER_CONNECT reconnection path).
	app.engine.UpdateBumper(id, map[string]interface{}{"CONNECTED": true})

	// The OLD client's disconnect fires late. Without the zombie guard, this
	// would incorrectly flip Connected back to false even though the NEW
	// client (still registered under the same PlayerID) is live.
	app.onPlayerDisconnected(id)

	bumper := app.engine.GetBumper(id)
	if bumper == nil {
		t.Fatalf("bumper %q unexpectedly disappeared", id)
	}
	if !bumper.Connected {
		t.Errorf("#109 zombie guard regression: a late disconnect from a superseded client flipped Connected to false even though a newer client still holds this PlayerID")
	}
}
