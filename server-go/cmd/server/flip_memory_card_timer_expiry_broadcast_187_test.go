// Test for #187 QUALIF follow-up (code-review 20260825-200416) — the
// engine-only tests in internal/game/engine_memory_card_timer_expiry_187_test.go
// prove the SERVER STATE transitions correctly at timer expiry
// (MotionSubPhase → REVEAL), but never prove a connected client actually
// RECEIVES that transition on the wire. code-reviewer found the gap: the
// per-tick OnTimerTick callback broadcasts ActionUpdateTimer, whose reduced
// frontend handler (useWebSocket.js, case 'UPDATE_TIMER') deliberately only
// copies phase/timer/countdownTime/gameTime — it never reads
// MEMOTION_SUBPHASE/MEMOTION_CARD_STATES/MEMOTION_ACTIVE. Without the fix
// (engine.OnMotionCardAutoRevealed → a.broadcastUpdate(), same pattern as
// OnQCMHint/broadcastQCMHint), TV/anim/vplayer stay visually stuck showing
// the card as still playable even though the server has already stopped
// accepting flips — exactly the "rien ne se passe" symptom a QUALIF
// re-tester would see.
//
// Uses a REAL per-card ticker (StartMotionCardTimer, 1-second ticks) rather
// than calling processMotionCardTick directly, specifically so this test
// exercises the actual goroutine wiring (OnTimerTick / OnMotionCardAutoRevealed
// invoked outside the lock) that the engine-only tests cannot reach.
//
// Run: go test ./cmd/server/... -run TestFlipMemoryCard_CardScoped_TimerExpiry_BroadcastsAutoReveal -v
package main

import (
	"encoding/json"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// TestFlipMemoryCard_CardScoped_TimerExpiry_BroadcastsAutoReveal is the
// wire-level regression test for the QUALIF report: a client connected
// before the card's timer expires must receive an ActionUpdate whose
// GAME.MEMOTION_SUBPHASE has become "REVEAL" — not just the reduced
// ActionUpdateTimer ticks.
func TestFlipMemoryCard_CardScoped_TimerExpiry_BroadcastsAutoReveal(t *testing.T) {
	app := newTestAppWithHub(t)
	setupMotionMemoryCardAtQuestion(t, app, []string{"TeamA", "TeamB"}, nil)

	// newTestApp does NOT call the real setupCallbacks (established
	// convention in this package — see wireOnStateChange,
	// main_broadcast_127_test.go — "mirror the relevant slice, not the
	// whole function", since setupCallbacks also touches a.httpServer,
	// nil in this harness). Mirror only the one callback this test needs,
	// exactly as main.go's setupCallbacks wires it.
	app.engine.OnMotionCardAutoRevealed = func(cardID string) {
		server.LogInfo(game.LogComponentEngine, "MEMOTION memory card auto-revealed at timer expiry (cardId=%s)", cardID)
		app.broadcastUpdate()
	}

	baseURL := startAnimAllowlistTestServer(t, app)
	tv := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tv)

	// Real 1-second ticker; delay=1 so the very first real tick already
	// expires the card, keeping the test fast (~1s) instead of needing a
	// long countdown.
	app.engine.StartMotionCardTimer(1)

	deadline := time.Now().Add(3 * time.Second)
	var gotSubPhase string
	found := false
	for time.Now().Before(deadline) {
		tv.SetReadDeadline(time.Now().Add(time.Until(deadline)))
		_, data, err := tv.ReadMessage()
		if err != nil {
			break // timeout or closed — stop draining
		}
		var envelope struct {
			Action string `json:"ACTION"`
			Msg    struct {
				Game struct {
					MotionSubPhase string `json:"MEMOTION_SUBPHASE"`
				} `json:"GAME"`
			} `json:"MSG"`
		}
		if json.Unmarshal(data, &envelope) != nil {
			continue
		}
		if envelope.Action != protocol.ActionUpdate {
			continue // ActionUpdateTimer and others don't count — must be a full UPDATE
		}
		gotSubPhase = envelope.Msg.Game.MotionSubPhase
		if gotSubPhase == string(game.MotionSubPhaseReveal) {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("TV never received an ActionUpdate with MEMOTION_SUBPHASE=REVEAL after the card's timer expired "+
			"(last MEMOTION_SUBPHASE seen on an UPDATE: %q) — the server-side state transition may be correct "+
			"but never reached the client, reproducing the QUALIF v7.1.0.1 report", gotSubPhase)
	}

	// Server-side state must agree with what the client just received —
	// belt-and-suspenders against a test that would pass on a stale echo.
	if got := app.engine.GetState().MotionSubPhase; got != game.MotionSubPhaseReveal {
		t.Errorf("server-side MotionSubPhase = %s, want REVEAL", got)
	}
}
