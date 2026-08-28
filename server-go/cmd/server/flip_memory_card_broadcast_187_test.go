// Test for #187 (v7.1.0) — the "no broadcast" half of the turn dérogation
// that flip_memory_card_turn_187_test.go does not cover.
//
// Contract (contracts/websocket-actions.md, fiche FLIP_MEMORY_CARD):
//
//	"🔴 Un flip ignoré ne doit déclencher aucun broadcast. [...] Si chaque tap
//	hors tour diffusait un GameState complet (~11 Ko en MEMOTION) à tous les
//	clients, on recréerait la classe de défaut des tempêtes de broadcast
//	(#127/#129, tests/procedures/bugfix-vjoueur-broadcast-storm.md)."
//
// flip_memory_card_turn_187_test.go asserts the ignored flip produces no
// STATE MUTATION (MEMORY_FLIPPED_CARDS stays empty on the sender's own
// engine read) but never asserts the OBSERVABLE side for other connected
// clients: that no UPDATE is put on the wire at all. That is precisely the
// #127/#129 failure class this contract paragraph exists to prevent, and it
// is a distinct assertion — a handler could avoid mutating state while still
// broadcasting an (unchanged) GameState, which would still be the storm.
//
// Harness reused, not duplicated: newTestAppWithHub, startAnimAllowlistTestServer,
// dialWS, learnClientID, sendAction (inbound_allowlist_test.go /
// inbound_allowlist_anim_test.go), collectActions/countActions
// (player_evicted_test.go / main_broadcast_127_test.go), and this file's own
// newMotionMemoryCard/setupMotionMemoryCardAtQuestion (flip_memory_card_turn_187_test.go).
//
// Run: go test ./cmd/server/... -run TestFlipMemoryCard_Broadcast -v
package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"testing"
	"time"
)

// TestFlipMemoryCard_Turn_VPlayerOffActiveTeam_NoBroadcastToOtherClients is
// the broadcast-observability twin of
// TestFlipMemoryCard_Turn_VPlayerOffActiveTeam_IgnoredSilently (question-host
// MEMORY, payload.MotionCardID == "").
func TestFlipMemoryCard_Turn_VPlayerOffActiveTeam_NoBroadcastToOtherClients(t *testing.T) {
	app := newTestAppWithHub(t)
	question := &game.Question{
		ID:   "q1",
		Type: game.QuestionTypeMemory,
		TypedContent: game.TypedContent{
			MemoryPairs: []game.MemoryPair{{ID: 1, Card1: game.MemoryCard{Text: "A"}, Card2: game.MemoryCard{Text: "A"}}},
		},
	}
	app.engine.Ready("q1", question)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}, "TeamB": {Name: "TeamB"}})
	if err := app.engine.SetMemoryParticipatingTeams([]string{"TeamA", "TeamB"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams: %v", err)
	}
	app.engine.SetPhase(game.PhaseEnroll)
	// TeamA is MemoryCurrentTeam (first) — Bob is on TeamB, out of turn.
	bumperID := setupVirtualPlayer(t, app, "Bob", "TeamB")
	app.engine.SetPhase(game.PhaseStarted)

	baseURL := startAnimAllowlistTestServer(t, app)
	vplayer := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vplayer)
	tv := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tv)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)

	sendAction(t, app, vplayer, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{CardID: "1-1", ID: bumperID})

	tvActions := collectActions(tv, 500*time.Millisecond)
	if got := countActions(tvActions, protocol.ActionUpdate); got != 0 {
		t.Errorf("#187: TV should receive 0 UPDATE from an off-turn FLIP_MEMORY_CARD, got %d (actions=%v)", got, tvActions)
	}
	adminActions := collectActions(admin, 500*time.Millisecond)
	if got := countActions(adminActions, protocol.ActionUpdate); got != 0 {
		t.Errorf("#187: admin should receive 0 UPDATE from an off-turn FLIP_MEMORY_CARD, got %d (actions=%v)", got, adminActions)
	}
}

// TestFlipMemoryCard_CardScoped_VPlayerOffActiveTeam_NoBroadcastToOtherClients
// is the broadcast-observability twin of
// TestFlipMemoryCard_CardScoped_VPlayerOffActiveTeam_IgnoredSilently — the
// card-scoped path, #187's actual new surface, reusing setupMotionMemoryCardAtQuestion
// from flip_memory_card_turn_187_test.go.
func TestFlipMemoryCard_CardScoped_VPlayerOffActiveTeam_NoBroadcastToOtherClients(t *testing.T) {
	app := newTestAppWithHub(t)
	bumperIDs := setupMotionMemoryCardAtQuestion(t, app, []string{"TeamA", "TeamB"}, map[string]string{"Bob": "TeamB"})
	bumperID := bumperIDs["Bob"]

	baseURL := startAnimAllowlistTestServer(t, app)
	vplayer := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vplayer)
	tv := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tv)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)

	sendAction(t, app, vplayer, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{
		CardID:    "1-1",
		ID:        bumperID,
		CardScope: protocol.CardScope{MotionCardID: "mc-mem"},
	})

	tvActions := collectActions(tv, 500*time.Millisecond)
	if got := countActions(tvActions, protocol.ActionUpdate); got != 0 {
		t.Errorf("#187 (card-scoped): TV should receive 0 UPDATE from an off-turn FLIP_MEMORY_CARD, got %d (actions=%v)", got, tvActions)
	}
	adminActions := collectActions(admin, 500*time.Millisecond)
	if got := countActions(adminActions, protocol.ActionUpdate); got != 0 {
		t.Errorf("#187 (card-scoped): admin should receive 0 UPDATE from an off-turn FLIP_MEMORY_CARD, got %d (actions=%v)", got, adminActions)
	}
}

// TestFlipMemoryCard_CardScoped_VPlayerOnActiveTeam_DoesBroadcast is the
// counterpart control case: a legitimate on-turn flip must still broadcast
// as before (#187 does not accidentally silence legitimate updates too —
// the two off-turn tests above would pass trivially if the handler stopped
// broadcasting FLIP_MEMORY_CARD entirely).
func TestFlipMemoryCard_CardScoped_VPlayerOnActiveTeam_DoesBroadcast(t *testing.T) {
	app := newTestAppWithHub(t)
	bumperIDs := setupMotionMemoryCardAtQuestion(t, app, []string{"TeamA", "TeamB"}, map[string]string{"Alice": "TeamA"})
	bumperID := bumperIDs["Alice"]

	baseURL := startAnimAllowlistTestServer(t, app)
	vplayer := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vplayer)
	tv := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tv)

	sendAction(t, app, vplayer, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{
		CardID:    "1-1",
		ID:        bumperID,
		CardScope: protocol.CardScope{MotionCardID: "mc-mem"},
	})

	tvActions := collectActions(tv, 500*time.Millisecond)
	if got := countActions(tvActions, protocol.ActionUpdate); got < 1 {
		t.Errorf("#187 (card-scoped): TV should receive at least 1 UPDATE from an on-turn FLIP_MEMORY_CARD, got %d (actions=%v)", got, tvActions)
	}
}
