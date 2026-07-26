package main

import (
	"buzzcontrol/internal/game"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Régression PROTOCOLE — badge CONN_STATE bloqué en ROUGE (#109, urgent)
//
// Handoff : _work/handoff/task-dev-backend-20260726-090000.md
// Rapport (root cause + fix) : _work/reports/dev-backend-<ts>-conn-state-fix.md
//
// Repro utilisateur exact (post-QUALIF v5.7.20) :
//  1. Un VJoueur s'enrôle -> pas d'icône (HIDDEN). OK.
//  2. Reload (F5) -> l'icône passe DIRECTEMENT en ROUGE (jamais visible en
//     ORANGE). BUG (hypothèse A confirmée).
//  3. La reconnexion se termine -> l'icône reste bloquée en ROUGE
//     indéfiniment. Le test ci-dessous vérifie qu'après le fix, la
//     reconnexion par ID fait bien GREEN -> HIDDEN comme attendu par la
//     table (`docs/DATA_MODELS.md` § Badge de Connexion).
//
// Root cause confirmée (voir rapport) : `ApplyVPlayerBroadcastConnEvents`,
// appelée par `broadcastUpdate()`, évaluait MessageLost pour CE VJoueur dans
// le MÊME broadcast que celui annonçant sa propre déconnexion (le broadcast
// déclenché par `onPlayerDisconnected` juste après avoir mis CONNECTED=false)
// -> ORANGE puis ROUGE dans le même tick, jamais visible côté admin.
//
// L'hypothèse B (Cas 1 de ReconnectOrCreateVirtualPlayer ne déclencherait
// jamais Reconnect) a été vérifiée FAUSSE par le code et par ce test : la
// reconnexion par ID fonctionne correctement une fois l'ID transmis.
// ---------------------------------------------------------------------------

// TestConnStateProtocol_EnrollDisconnectReconnect_NeverSkipsOrangeNeverStuckRed
// is the exact user-reported scenario, end to end through the real handlers
// (onPlayerDisconnected, handlePlayerConnect) — not just the pure engine
// table.
func TestConnStateProtocol_EnrollDisconnectReconnect_NeverSkipsOrangeNeverStuckRed(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.SetPhase(game.PhaseEnroll)

	// Step 1: enrollment.
	id, _, err := app.engine.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	if err := app.engine.AssignVirtualPlayer(id, "TeamA", game.AnswerColorNone); err != nil {
		t.Fatalf("AssignVirtualPlayer failed: %v", err)
	}
	if got := app.engine.GetBumper(id).ConnState; got != "" {
		t.Fatalf("step 1 failed: expected HIDDEN right after enrollment, got %q", got)
	}

	// Step 2: F5 — the old WebSocket closes, the hub fires onPlayerDisconnected
	// (mirrors the real callback exactly, not a simplified stand-in — see
	// setupCallbacks in main.go: a.wsHub.OnPlayerDisconnected = a.onPlayerDisconnected).
	app.onPlayerDisconnected(id)

	if got := app.engine.GetBumper(id).ConnState; got != "orange" {
		t.Fatalf("#109 regression: expected ORANGE to be visible right after disconnect, got %q "+
			"(if this is \"red\", the disconnect-announcing broadcast is being counted as a lost "+
			"message for the VJoueur it's about — hypothesis A)", got)
	}
	if app.engine.GetBumper(id).Connected {
		t.Errorf("expected Connected==false after onPlayerDisconnected")
	}

	// Step 3: 2s later (per VPlayerPage.jsx's reconnect delay), the new page
	// sends PLAYER_CONNECT with the ID captured at enrollment.
	app.handlePlayerConnect("client-reco", playerConnectMsg(t, "Alice", id))

	if got := app.engine.GetBumper(id).ConnState; got != "green" {
		t.Fatalf("#109 regression: expected GREEN after reconnecting by ID, got %q "+
			"(if this is still \"red\", ReconnectOrCreateVirtualPlayer's case 1 is not firing "+
			"ConnEventReconnect — hypothesis B)", got)
	}
	if !app.engine.GetBumper(id).Connected {
		t.Errorf("expected Connected==true after reconnecting")
	}

	// Step 4: eventually, once the minimum green window elapses and a delivery
	// is confirmed (e.g. the next broadcast, or a message from the client),
	// the badge must settle back to HIDDEN — never stay stuck on any color.
	time.Sleep(2100 * time.Millisecond) // connGreenMinDuration default (2s, D2) — see package game
	app.engine.ApplyVPlayerBroadcastConnEvents()

	if got := app.engine.GetBumper(id).ConnState; got != "" {
		t.Errorf("expected HIDDEN once the green window elapsed and delivery was confirmed, got %q — badge stuck", got)
	}
}

// TestConnStateProtocol_MissedBroadcastWhileDisconnected_StillTurnsRed is the
// D4 sanity counterpart: the fix must not silently disable MessageLost
// altogether — a broadcast that happens AFTER the disconnect was already
// announced still represents a genuinely missed message and must turn the
// badge red.
func TestConnStateProtocol_MissedBroadcastWhileDisconnected_StillTurnsRed(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.SetPhase(game.PhaseEnroll)

	id, _, err := app.engine.CreateVirtualPlayer("Bob")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	if err := app.engine.AssignVirtualPlayer(id, "TeamA", game.AnswerColorNone); err != nil {
		t.Fatalf("AssignVirtualPlayer failed: %v", err)
	}

	app.onPlayerDisconnected(id)
	if got := app.engine.GetBumper(id).ConnState; got != "orange" {
		t.Fatalf("expected orange after disconnect, got %q", got)
	}

	// Something else happens in the game while Bob is still disconnected
	// (e.g. another team's score changes) -> a real GameState broadcast that
	// Bob genuinely misses.
	app.broadcastUpdate()

	if got := app.engine.GetBumper(id).ConnState; got != "red" {
		t.Errorf("D4: expected orange -> red on a broadcast genuinely missed while disconnected, got %q", got)
	}
}
