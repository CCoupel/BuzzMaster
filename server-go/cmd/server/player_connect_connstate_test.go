package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"testing"
)

// ---------------------------------------------------------------------------
// Régression #109 — badge de connexion VJoueur + bumper fantôme (R1)
//
// Contexte (plan _work/reports/planner-20260725-105503-final.md §1/§2/§10-R1) :
// la reconnexion d'un VJoueur (handlePlayerConnect, main.go ~1857-1911) matche
// l'existant par NOM SEUL. Avant le fix R1, une course entre déconnexion et
// reconnexion pouvait laisser un second bumper "fantôme" pour le même joueur,
// avec un badge de connexion resté bloqué en orange/rouge.
//
// Ces tests utilisent newTestApp (cmd/server/testhelpers_test.go) et ajoutent
// un wsHub réel (sans client enregistré) pour pouvoir appeler
// handlePlayerConnect directement — SendToClient/SetClientPlayerID sont no-op
// sûrs sans client connecté (voir internal/server/websocket.go).
//
// Dépend du contrat CONN_STATE défini dans engine_connstate_test.go (Phase 1
// du plan, en cours d'implémentation par dev-backend en parallèle de ce lot).
// ---------------------------------------------------------------------------

// newTestAppWithHub extends newTestApp with a real (client-less) WebSocketHub,
// required by handlePlayerConnect (a.wsHub.SendToClient / SetClientPlayerID).
func newTestAppWithHub(t *testing.T) *App {
	t.Helper()
	app := newTestApp(t)
	app.wsHub = server.NewWebSocketHub()
	return app
}

func playerConnectMsg(t *testing.T, name string) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionPlayerConnect, protocol.PlayerConnectPayload{Name: name})
	if err != nil {
		t.Fatalf("failed to build PLAYER_CONNECT message: %v", err)
	}
	return msg
}

// TestPlayerConnect_Reconnect_ClearsConnStateBadge is the core #109 regression
// test: a VJoueur that disconnects (badge → orange) and then reconnects via
// PLAYER_CONNECT must see its ConnState leave the orange/red range — it must
// NOT remain stuck showing a disconnected badge after a successful reco.
func TestPlayerConnect_Reconnect_ClearsConnStateBadge(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.SetPhase(game.PhaseEnroll)

	id, _, err := app.engine.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	if err := app.engine.AssignVirtualPlayer(id, "TeamA", game.AnswerColorNone); err != nil {
		t.Fatalf("AssignVirtualPlayer failed: %v", err)
	}

	// Simulate the disconnect path (main.go OnPlayerDisconnected, ~line 444):
	// marks CONNECTED=false and fires the Disconnect transition.
	app.engine.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	app.engine.TransitionConn(id, game.ConnEventDisconnect)

	if got := app.engine.GetBumper(id).ConnState; got != "orange" {
		t.Fatalf("setup failed: expected 'orange' after simulated disconnect, got %q", got)
	}

	// Reconnection: same flow as the real client (PLAYER_CONNECT with the
	// same name) going through the existing by-name matching branch.
	app.handlePlayerConnect("client-reco-1", playerConnectMsg(t, "Alice"))

	reconnected := app.engine.GetBumper(id)
	if reconnected == nil {
		t.Fatalf("bumper %q disappeared after reconnection", id)
	}
	if !reconnected.Connected {
		t.Errorf("expected Connected==true after PLAYER_CONNECT reconnection")
	}
	if reconnected.ConnState == "orange" || reconnected.ConnState == "red" {
		t.Errorf("#109 regression: badge still shows disconnected (ConnState=%q) after successful reconnection", reconnected.ConnState)
	}
}

// TestPlayerConnect_Reconnect_NoGhostDuplicate verifies that reconnecting by
// name never creates a second bumper for the same player: exactly one
// bumper with IsVirtual==true and the given name must exist after
// handlePlayerConnect runs, whether it's a first connection or a reconnect.
func TestPlayerConnect_Reconnect_NoGhostDuplicate(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)

	// First connection creates the bumper.
	app.handlePlayerConnect("client-1", playerConnectMsg(t, "Charlie"))
	// Reconnection (same name, different client — e.g. app restart on phone).
	app.handlePlayerConnect("client-2", playerConnectMsg(t, "Charlie"))
	// A third "reconnection" burst (simulates a flaky network retry).
	app.handlePlayerConnect("client-3", playerConnectMsg(t, "Charlie"))

	count := 0
	for _, b := range app.engine.GetTeamsAndBumpers().Bumpers {
		if b.IsVirtual && b.Name == "Charlie" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("R1: expected exactly 1 bumper for 'Charlie' after repeated PLAYER_CONNECT, got %d (ghost duplicate)", count)
	}
}

// TestPlayerConnect_PreexistingGhost_NoResidualBadge covers the R1 mitigation
// itself: if two bumpers already share the same VJoueur name (simulating a
// ghost created by a prior race, before the R1 fix existed), reconnecting
// under that name must not leave any of them displaying a stuck
// orange/red badge (plan §5: "pas de badge résiduel après reconnexion").
//
// NOTE: this does not assert on the exact dedup mechanism (delete vs. merge)
// dev-backend chooses — only on the observable acceptance criterion.
func TestPlayerConnect_PreexistingGhost_NoResidualBadge(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.SetPhase(game.PhaseEnroll)

	app.engine.SetBumpers(map[string]*game.Bumper{
		"vjoueur_alice_ghost": {Name: "Alice", IsVirtual: true, IsVPlayer: true, Team: "TeamA", Connected: false, ConnState: "orange"},
		"vjoueur_alice_real":  {Name: "Alice", IsVirtual: true, IsVPlayer: true, Team: "TeamA", Connected: false, ConnState: "orange"},
	})

	app.handlePlayerConnect("client-reco", playerConnectMsg(t, "Alice"))

	residual := 0
	for id, b := range app.engine.GetTeamsAndBumpers().Bumpers {
		if b.Name == "Alice" && b.IsVirtual && (b.ConnState == "orange" || b.ConnState == "red") {
			residual++
			t.Logf("residual disconnected badge on bumper id=%s ConnState=%s", id, b.ConnState)
		}
	}
	if residual > 0 {
		t.Errorf("R1: %d bumper(s) named 'Alice' still show a disconnected badge after reconnection, expected 0", residual)
	}
}
