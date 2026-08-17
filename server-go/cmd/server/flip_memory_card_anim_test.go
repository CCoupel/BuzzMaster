package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests: #159/B1 — FLIP_MEMORY_CARD widened to ClientTypeAnim (T1 end-to-end
// complement to internal/server's TestIsActionAllowed table row, T2 phase
// guard non-regression). Mirrors
// TestHandleWebMessage_AllowList_TVCanFlipMemoryCard
// (inbound_allowlist_test.go) for the new sender — same real dispatch path
// (handleWebMessage), not a direct engine call, so the allow-list gate is
// actually exercised.
//
// Uses startAnimAllowlistTestServer (inbound_allowlist_anim_test.go), not
// startEvictionTestServer: the latter has no "/anim" route case and falls
// through to ClientTypeVPlayer (same false-negative already fixed for
// #158/T1, see ardoise_input_anim_test.go's own comment on this).
// ---------------------------------------------------------------------------

// T1 — the capability widening actually works end-to-end: a flip sent from
// /ws/anim during STARTED mutates MemoryFlippedCards, same as TV/VPlayer.
func TestHandleWebMessage_AllowList_AnimCanFlipMemoryCard(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseStarted)

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, anim, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{CardID: "1-1"})

	flipped := app.engine.GetState().MemoryFlippedCards
	found := false
	for _, id := range flipped {
		if id == "1-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("#159/B1: anim must be able to send FLIP_MEMORY_CARD — MemoryFlippedCards=%v, want to contain 1-1", flipped)
	}
}

// T1 — admin stays explicitly rejected (unchanged by #159, régie flips
// through its embedded TV preview iframe instead). Complements the pure
// IsActionAllowed table row (internal/server) with the real dispatch path.
func TestHandleWebMessage_AllowList_AdminCannotFlipMemoryCard(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseStarted)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)

	sendAction(t, app, admin, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{CardID: "1-1"})

	flipped := app.engine.GetState().MemoryFlippedCards
	for _, id := range flipped {
		if id == "1-1" {
			t.Fatalf("#159/B1: admin must still be rejected for FLIP_MEMORY_CARD (flips through its TV preview iframe), got MemoryFlippedCards=%v", flipped)
		}
	}
}

// T2 — non-régression du garde-fou de phase : le moteur ignore un
// retournement hors STARTED, quel que soit l'émetteur — inchangé par #159,
// vérifié explicitement pour anim (le nouvel émetteur) via le vrai chemin
// de dispatch.
func TestFlipMemoryCard_Anim_IgnoredOutsideStarted(t *testing.T) {
	for _, phase := range []game.GamePhase{game.PhaseReady, game.PhasePrepare, game.PhaseStopped, game.PhaseRevealed, game.PhaseNewGame} {
		t.Run(string(phase), func(t *testing.T) {
			app := newTestAppWithHub(t)
			app.engine.SetPhase(phase)

			baseURL := startAnimAllowlistTestServer(t, app)
			anim := dialWS(t, baseURL, "/ws/anim")
			learnClientID(t, app, anim)

			sendAction(t, app, anim, protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{CardID: "1-1"})

			flipped := app.engine.GetState().MemoryFlippedCards
			for _, id := range flipped {
				if id == "1-1" {
					t.Fatalf("#159/T2: flip from anim outside STARTED (phase=%s) must be ignored by the engine, got MemoryFlippedCards=%v", phase, flipped)
				}
			}
		})
	}
}
