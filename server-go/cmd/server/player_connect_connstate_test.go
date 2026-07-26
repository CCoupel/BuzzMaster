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
// [CHANGED — fix R1, _work/reports/planner-20260725-143029-r1-fix.md]
// La consolidation par nom (matching + suppression des doublons) a été
// retirée : elle provoquait une perte de données silencieuse sur une vraie
// collision de noms (code-review CRITIQUE). L'identité repose désormais sur
// l'ID renvoyé par le serveur à l'enrôlement (`PLAYER_CONNECTED.ID`), que le
// client doit renvoyer dans `PLAYER_CONNECT.ID` pour se reconnecter sans
// ambiguïté. Sans ID, un nom déjà pris est REJETÉ (`NAME_TAKEN`), jamais
// fusionné/remplacé — voir `engine.ReconnectOrCreateVirtualPlayer` (matrice
// §3.3 du plan). Les 2 tests ci-dessous ciblant l'ancien comportement
// (reconnexion par nom seul) ont été mis à jour en conséquence.
//
// Ces tests utilisent newTestApp (cmd/server/testhelpers_test.go) et ajoutent
// un wsHub réel (sans client enregistré) pour pouvoir appeler
// handlePlayerConnect directement — SendToClient/SetClientPlayerID sont no-op
// sûrs sans client connecté (voir internal/server/websocket.go).
// ---------------------------------------------------------------------------

// newTestAppWithHub extends newTestApp with a real (client-less) WebSocketHub,
// required by handlePlayerConnect (a.wsHub.SendToClient / SetClientPlayerID).
func newTestAppWithHub(t *testing.T) *App {
	t.Helper()
	app := newTestApp(t)
	app.wsHub = server.NewWebSocketHub()
	return app
}

// playerConnectMsg builds a PLAYER_CONNECT message. id may be "" for a first
// enrollment / a client that never stored one (fix R1: identity by ID).
func playerConnectMsg(t *testing.T, name, id string) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionPlayerConnect, protocol.PlayerConnectPayload{Name: name, ID: id})
	if err != nil {
		t.Fatalf("failed to build PLAYER_CONNECT message: %v", err)
	}
	return msg
}

// TestPlayerConnect_ReconnectByID_ClearsConnStateBadge is the core #109
// regression test, updated for the ID-based flow (fix R1): a VJoueur that
// disconnects (badge → orange) and then reconnects via PLAYER_CONNECT
// *with its stored ID* must see its ConnState leave the orange/red range,
// keep its team/score, and reuse the exact same bumper ID (no ghost).
func TestPlayerConnect_ReconnectByID_ClearsConnStateBadge(t *testing.T) {
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
	app.engine.UpdateBumperScore(id, 40) // score must survive the reconnection

	// Simulate the disconnect path (main.go OnPlayerDisconnected, ~line 444):
	// marks CONNECTED=false and fires the Disconnect transition.
	app.engine.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	app.engine.TransitionConn(id, game.ConnEventDisconnect)

	if got := app.engine.GetBumper(id).ConnState; got != "orange" {
		t.Fatalf("setup failed: expected 'orange' after simulated disconnect, got %q", got)
	}

	// Reconnection: the client echoes back the ID it received in
	// PLAYER_CONNECTED at enrollment time (fix R1 — no more name matching).
	app.handlePlayerConnect("client-reco-1", playerConnectMsg(t, "Alice", id))

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
	if reconnected.Team != "TeamA" {
		t.Errorf("R1: team lost on reconnection, expected 'TeamA', got %q", reconnected.Team)
	}
	if reconnected.Score != 40 {
		t.Errorf("R1: score lost on reconnection, expected 40, got %d", reconnected.Score)
	}

	// No ghost: still exactly one 'Alice' bumper, at the original ID.
	count := 0
	for _, b := range app.engine.GetTeamsAndBumpers().Bumpers {
		if b.IsVirtual && b.Name == "Alice" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("R1: expected exactly 1 bumper for 'Alice' after reconnection, got %d (ghost duplicate)", count)
	}
}

// TestPlayerConnect_NoID_NameFree_CreatesNewEnrollment covers matrix case 4
// (§3.3): a first PLAYER_CONNECT with no ID and a name nobody else has must
// succeed as a brand-new enrollment.
func TestPlayerConnect_NoID_NameFree_CreatesNewEnrollment(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)

	app.handlePlayerConnect("client-1", playerConnectMsg(t, "Charlie", ""))

	count := 0
	for _, b := range app.engine.GetTeamsAndBumpers().Bumpers {
		if b.IsVirtual && b.Name == "Charlie" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 'Charlie' bumper after a fresh enrollment, got %d", count)
	}
}

// TestPlayerConnect_NoID_NameTaken_RejectsRepeatedAttempts covers matrix
// case 3 (§3.3) end-to-end through handlePlayerConnect: once a name is
// taken, further PLAYER_CONNECT attempts *without* the owning ID must be
// rejected rather than silently reconnecting to (or duplicating) the
// existing bumper — repeated retries (e.g. a flaky client resending the
// same name-only payload) must never create a ghost NOR merge into the
// existing player.
func TestPlayerConnect_NoID_NameTaken_RejectsRepeatedAttempts(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)

	// First connection creates the bumper (case 4).
	app.handlePlayerConnect("client-1", playerConnectMsg(t, "Charlie", ""))
	// Two more name-only attempts (no ID) — must be rejected (case 3), not
	// merged into the existing "Charlie".
	app.handlePlayerConnect("client-2", playerConnectMsg(t, "Charlie", ""))
	app.handlePlayerConnect("client-3", playerConnectMsg(t, "Charlie", ""))

	count := 0
	for _, b := range app.engine.GetTeamsAndBumpers().Bumpers {
		if b.IsVirtual && b.Name == "Charlie" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("R1: expected exactly 1 bumper for 'Charlie' after repeated name-only PLAYER_CONNECT, got %d", count)
	}
}

// TestPlayerConnect_ContradictoryHomonyms_NeverMergedNeverDeleted is the
// "code-reviewer scenario" (fix R1, cas contradictoire): two distinct VJoueur
// bumpers that happen to share the same (normalized) name, with DIFFERENT
// team/score data — e.g. "Emma" from a previous session (still on disk,
// disconnected) and a brand-new "Emma" trying to join tonight. A PLAYER_CONNECT
// with that name and no ID must be rejected (NAME_TAKEN); neither existing
// bumper may ever be deleted or have its data overwritten as a side effect.
func TestPlayerConnect_ContradictoryHomonyms_NeverMergedNeverDeleted(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}, "TeamB": {Name: "TeamB"}})
	app.engine.SetPhase(game.PhaseEnroll)

	app.engine.SetBumpers(map[string]*game.Bumper{
		// Legacy "Emma" from a prior session: disconnected, but has real data.
		"vjoueur_emma_legacy": {Name: "Emma", IsVirtual: true, IsVPlayer: true, Team: "TeamA", Score: 40, Connected: false, ConnState: "orange"},
		// A different, currently-connected "Emma" (e.g. two people, same first name).
		"vjoueur_emma_tonight": {Name: "Emma", IsVirtual: true, IsVPlayer: true, Team: "TeamB", Score: 5, Connected: true, ConnState: ""},
	})

	app.handlePlayerConnect("client-new-emma", playerConnectMsg(t, "Emma", ""))

	bumpers := app.engine.GetTeamsAndBumpers().Bumpers
	if len(bumpers) != 2 {
		t.Fatalf("R1: expected both pre-existing 'Emma' bumpers to still exist (no delete), got %d bumpers total", len(bumpers))
	}

	legacy, ok := bumpers["vjoueur_emma_legacy"]
	if !ok {
		t.Fatal("R1: legacy 'Emma' bumper was deleted — data loss")
	}
	if legacy.Team != "TeamA" || legacy.Score != 40 {
		t.Errorf("R1: legacy 'Emma' data altered — expected TeamA/40, got %s/%d", legacy.Team, legacy.Score)
	}

	tonight, ok := bumpers["vjoueur_emma_tonight"]
	if !ok {
		t.Fatal("R1: connected 'Emma' bumper was deleted — data loss")
	}
	if tonight.Team != "TeamB" || tonight.Score != 5 {
		t.Errorf("R1: connected 'Emma' data altered — expected TeamB/5, got %s/%d", tonight.Team, tonight.Score)
	}

	// The rejected attempt must not have spawned a third bumper either.
	emmaCount := 0
	for _, b := range bumpers {
		if b.Name == "Emma" {
			emmaCount++
		}
	}
	if emmaCount != 2 {
		t.Errorf("R1: expected exactly 2 'Emma' bumpers (no new one created on rejection), got %d", emmaCount)
	}
}

// TestPlayerConnect_StaleID_FallsBackToNameCheck covers matrix case 2 (§3.3):
// an ID that doesn't resolve to any bumper (e.g. deleted by admin, or from a
// server restart before persistence) must be treated as if no ID was given
// at all — not as an error, not as a crash.
func TestPlayerConnect_StaleID_FallsBackToNameCheck(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)

	app.handlePlayerConnect("client-1", playerConnectMsg(t, "Dora", "vjoueur_does_not_exist"))

	count := 0
	for _, b := range app.engine.GetTeamsAndBumpers().Bumpers {
		if b.IsVirtual && b.Name == "Dora" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected a fresh enrollment for 'Dora' despite the stale ID, got %d bumper(s)", count)
	}
}
