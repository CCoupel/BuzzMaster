// Tests for #187 (v7.1.0) — FLIP_MEMORY_CARD becomes server-authoritative
// on the turn. Two independent checks, contract §9.2 dérogation
// (websocket-actions.md fiche FLIP_MEMORY_CARD):
//  1. Card scope (MOTION_CARD_ID) — refused explicitly, unchanged from #184.
//  2. Whose turn it is — checked ONLY for vplayer, ignored silently
//     (no mutation, no broadcast) on a mismatch. tv/anim always pass.
//
// Run: go test ./cmd/server/... -run TestFlipMemoryCard_Turn -v
package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"testing"
)

// ---------------------------------------------------------------------------
// Question-host MEMORY (payload.MotionCardID == "")
// ---------------------------------------------------------------------------

func TestFlipMemoryCard_Turn_VPlayerOnActiveTeam_Applies(t *testing.T) {
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
	// CreateVirtualPlayer requires PhaseEnroll — a bare SetPhase round-trip
	// (pure setter, no side effect on Question/teams already configured).
	app.engine.SetPhase(game.PhaseEnroll)
	bumperID := setupVirtualPlayer(t, app, "Alice", "TeamA") // TeamA is MemoryCurrentTeam (first)
	app.engine.SetPhase(game.PhaseStarted)

	baseURL := startAnimAllowlistTestServer(t, app)
	vplayer := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vplayer)

	sendAction(t, app, vplayer, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{CardID: "1-1", ID: bumperID})

	flipped := app.engine.GetState().MemoryFlippedCards
	if len(flipped) != 1 || flipped[0] != "1-1" {
		t.Errorf("MemoryFlippedCards = %v, want [1-1] — a VPlayer on the active team must be able to flip", flipped)
	}
}

func TestFlipMemoryCard_Turn_VPlayerOffActiveTeam_IgnoredSilently(t *testing.T) {
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

	sendAction(t, app, vplayer, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{CardID: "1-1", ID: bumperID})

	flipped := app.engine.GetState().MemoryFlippedCards
	if len(flipped) != 0 {
		t.Errorf("MemoryFlippedCards = %v, want empty — a VPlayer off the active team's flip must be ignored (contract §9.2 dérogation)", flipped)
	}
}

// ---------------------------------------------------------------------------
// Card-scoped MEMORY (payload.MotionCardID set) — #187's real target
// ---------------------------------------------------------------------------

func newMotionMemoryCard(cardID string) game.MotionCard {
	return game.MotionCard{
		ID:         cardID,
		Type:       game.QuestionTypeMemory,
		RectoTheme: "T",
		Difficulty: 1,
		TypedContent: game.TypedContent{
			MemoryPairs: []game.MemoryPair{{ID: 1, Card1: game.MemoryCard{Text: "A"}, Card2: game.MemoryCard{Text: "A"}}},
		},
	}
}

// setupMotionMemoryCardAtQuestion prepares app for a card-scoped flip: a
// MEMOTION question with one MEMORY card, teams set, VPlayer bumpers
// created for the given (name -> team) players, started, card selected and
// flipped to QUESTION sub-phase (HostContext.Playable). Returns each
// player's bumperID keyed by name.
//
// Players are created via CreateVirtualPlayer, which requires PhaseEnroll —
// StartImmediate has no phase guard of its own (unlike Ready/
// SetMotionParticipatingTeams), so a bare SetPhase(ENROLL) round-trip
// before it is enough; nothing else in this setup depends on a specific
// phase in between.
func setupMotionMemoryCardAtQuestion(t *testing.T, app *App, teams []string, players map[string]string) map[string]string {
	t.Helper()
	card := newMotionMemoryCard("mc-mem")
	q := &game.Question{
		ID:          "mq1",
		Type:        game.QuestionTypeMemotion,
		MotionCards: []game.MotionCard{card},
		MotionMode:  "SOLO",
		Points:      "10",
		Time:        "0",
	}
	app.engine.Ready("mq1", q)
	teamMap := map[string]*game.Team{}
	for _, name := range teams {
		teamMap[name] = &game.Team{Name: name}
	}
	app.engine.SetTeams(teamMap)
	if err := app.engine.SetMotionParticipatingTeams(teams); err != nil {
		t.Fatalf("SetMotionParticipatingTeams: %v", err)
	}

	app.engine.SetPhase(game.PhaseEnroll)
	bumperIDs := map[string]string{}
	for name, team := range players {
		bumperIDs[name] = setupVirtualPlayer(t, app, name, team)
	}

	app.engine.StartImmediate(0)
	app.engine.InitMotionState()
	if err := app.engine.SelectMotionCard("mc-mem"); err != nil {
		t.Fatalf("SelectMotionCard: %v", err)
	}
	if err := app.engine.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard: %v", err)
	}
	return bumperIDs
}

func TestFlipMemoryCard_CardScoped_VPlayerOnActiveTeam_Applies(t *testing.T) {
	app := newTestAppWithHub(t)
	// MotionCurrentTeam is the first of SetMotionParticipatingTeams — TeamA.
	bumperIDs := setupMotionMemoryCardAtQuestion(t, app, []string{"TeamA", "TeamB"}, map[string]string{"Alice": "TeamA"})
	bumperID := bumperIDs["Alice"]

	baseURL := startAnimAllowlistTestServer(t, app)
	vplayer := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vplayer)

	sendAction(t, app, vplayer, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{
		CardID:    "1-1",
		ID:        bumperID,
		CardScope: protocol.CardScope{MotionCardID: "mc-mem"},
	})

	flipped := motionActiveMemoryFlippedCardsForTest(app)
	if len(flipped) != 1 || flipped[0] != "1-1" {
		t.Errorf("MEMOTION_ACTIVE.STATE.MEMORY_FLIPPED_CARDS = %v, want [1-1]", flipped)
	}
}

func TestFlipMemoryCard_CardScoped_VPlayerOffActiveTeam_IgnoredSilently(t *testing.T) {
	app := newTestAppWithHub(t)
	// TeamA is MotionCurrentTeam (first) — Bob is on TeamB, out of turn.
	bumperIDs := setupMotionMemoryCardAtQuestion(t, app, []string{"TeamA", "TeamB"}, map[string]string{"Bob": "TeamB"})
	bumperID := bumperIDs["Bob"]

	baseURL := startAnimAllowlistTestServer(t, app)
	vplayer := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vplayer)

	sendAction(t, app, vplayer, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{
		CardID:    "1-1",
		ID:        bumperID,
		CardScope: protocol.CardScope{MotionCardID: "mc-mem"},
	})

	flipped := motionActiveMemoryFlippedCardsForTest(app)
	if len(flipped) != 0 {
		t.Errorf("MEMOTION_ACTIVE.STATE.MEMORY_FLIPPED_CARDS = %v, want empty — off-turn flip must be ignored", flipped)
	}
}

// TestFlipMemoryCard_CardScoped_AnimAlwaysApplies is the regression guard
// for plan §2.3 "piège 1": anim plays for the table and has no team of its
// own — the turn check must never apply to it, card-scoped or not.
func TestFlipMemoryCard_CardScoped_AnimAlwaysApplies(t *testing.T) {
	app := newTestAppWithHub(t)
	setupMotionMemoryCardAtQuestion(t, app, []string{"TeamA", "TeamB"}, nil)

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, anim, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{
		CardID:    "1-1",
		CardScope: protocol.CardScope{MotionCardID: "mc-mem"},
	})

	flipped := motionActiveMemoryFlippedCardsForTest(app)
	if len(flipped) != 1 || flipped[0] != "1-1" {
		t.Errorf("MEMOTION_ACTIVE.STATE.MEMORY_FLIPPED_CARDS = %v, want [1-1] — anim must always be able to flip", flipped)
	}
}

// TestFlipMemoryCard_CardScoped_TVAlwaysApplies mirrors the anim case for
// tv (régie iframe preview / public screen click) — no team of its own
// either.
func TestFlipMemoryCard_CardScoped_TVAlwaysApplies(t *testing.T) {
	app := newTestAppWithHub(t)
	setupMotionMemoryCardAtQuestion(t, app, []string{"TeamA", "TeamB"}, nil)

	baseURL := startAnimAllowlistTestServer(t, app)
	tv := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tv)

	sendAction(t, app, tv, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{
		CardID:    "1-1",
		CardScope: protocol.CardScope{MotionCardID: "mc-mem"},
	})

	flipped := motionActiveMemoryFlippedCardsForTest(app)
	if len(flipped) != 1 || flipped[0] != "1-1" {
		t.Errorf("MEMOTION_ACTIVE.STATE.MEMORY_FLIPPED_CARDS = %v, want [1-1] — tv must always be able to flip", flipped)
	}
}

// TestFlipMemoryCard_CardScoped_WrongMotionCardID_RefusedExplicitly is the
// distinct "portée de carte" check (contract §9.2, refused explicitly —
// not the silent turn ignore): a MOTION_CARD_ID that doesn't match the
// active card must produce no mutation, whichever client sends it.
func TestFlipMemoryCard_CardScoped_WrongMotionCardID_RefusedExplicitly(t *testing.T) {
	app := newTestAppWithHub(t)
	setupMotionMemoryCardAtQuestion(t, app, []string{"TeamA", "TeamB"}, nil)

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, anim, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{
		CardID:    "1-1",
		CardScope: protocol.CardScope{MotionCardID: "not-the-active-card"},
	})

	flipped := motionActiveMemoryFlippedCardsForTest(app)
	if len(flipped) != 0 {
		t.Errorf("MEMOTION_ACTIVE.STATE.MEMORY_FLIPPED_CARDS = %v, want empty — a MOTION_CARD_ID mismatch must be refused", flipped)
	}
}

// motionActiveMemoryFlippedCardsForTest reads MEMOTION_ACTIVE.STATE.
// MEMORY_FLIPPED_CARDS from the live engine state — package-external
// helper (this file is package main, MotionActive.State is a plain
// map[string]interface{}, exported field) mirroring internal/game's own
// unexported accessor for the same key.
func motionActiveMemoryFlippedCardsForTest(app *App) []string {
	v, ok := app.engine.GetState().MotionActive.State["MEMORY_FLIPPED_CARDS"]
	if !ok {
		return nil
	}
	s, ok := v.([]string)
	if !ok {
		return nil
	}
	return s
}
