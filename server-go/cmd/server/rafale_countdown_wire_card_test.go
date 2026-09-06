package main

// ---------------------------------------------------------------------------
// C1 regression — card twin of rafale_countdown_wire_test.go (retour QUALIF
// v9.0.0.4, plan-v900-correctifs-qualif-20260906-104500.md §7 "dette de
// test #217"). The classic-round wire test does NOT exercise a MEMOTION
// card's own RAFALE mini-round at all — exactly the coverage gap that let
// the "chrono figé" bug (C1) reach QUALIF undetected: RAFALE_TICK never
// carried MOTION_CARD_ID, so a client in card context had no way to tell a
// tick was meant for it.
//
// This file deliberately does NOT modify or reuse
// newRafaleCountdownWireTestApp from rafale_countdown_wire_test.go — that
// helper's own OnRafaleQuestionTick wiring must be mechanically adapted by
// whoever changes the callback's signature (dev-backend, alongside C1
// itself); duplicating a small, self-contained wiring helper here avoids a
// two-agent collision on that shared file (project history: git collisions
// on shared working files during parallel dev passes).
//
// Assumed signature (dev-backend's own C1 handoff, engine.go:2412/
// main.go:548/:4907 — "remonter le CARD_ID"):
//   Engine.OnRafaleQuestionTick func(cardID string, questionTime int)
//   App.broadcastRafaleTick(cardID string, questionTime int)
//   protocol.RafaleTickPayload gains a CardScope (MOTION_CARD_ID)
// If dev-backend lands a different shape, this file needs the same
// mechanical adaptation as any other call site — flagged in the DONE report
// rather than guessed twice.

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// newRafaleCardCountdownWireTestApp mirrors newRafaleCountdownWireTestApp's
// own doc comment (mirrors main.go's setupCallbacks RAFALE-relevant slice)
// but wires OnRafaleQuestionTick with the card-scoped signature C1
// introduces, rather than the classic-round-only one the sibling file still
// uses at the time of writing.
func newRafaleCardCountdownWireTestApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster()

	app.engine.OnStateChange = func(phase game.GamePhase) {
		app.broadcastGameState(string(phase))
	}
	app.engine.OnRafaleQuestionTick = func(cardID string, questionTime int) {
		app.broadcastRafaleTick(cardID, questionTime)
	}

	go func() {
		for msg := range app.wsHub.Incoming {
			app.handleWebMessage(msg)
		}
	}()

	return app
}

// TestRafaleCardCountdown_TickCarriesMotionCardID_QuestionTimeDecreasesOnTheWire
// is the permanent regression test for C1: MEMOTION_SELECT → MEMOTION_FLIP
// (real dispatch, exactly what handleMotionFlip does for a RAFALE card) must
// produce RAFALE_TICK frames on the wire that (1) carry MOTION_CARD_ID
// matching the active card, and (2) show QUESTION_TIME actually decreasing
// (3 → 2 → 1) — the exact "chrono figé" symptom, verified on a REAL TV
// connection rather than only at the engine/MotionActive.State level
// (already covered by internal/game's own #217 suite).
func TestRafaleCardCountdown_TickCarriesMotionCardID_QuestionTimeDecreasesOnTheWire(t *testing.T) {
	app := newRafaleCardCountdownWireTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{"red": {Name: "red", Color: []int{255, 0, 0}}})

	for i := 1; i <= 10; i++ {
		if _, err := app.engine.UpsertRafaleQuestion(game.RafaleQuestion{
			ID: fmt.Sprintf("r-%d", i), Question: fmt.Sprintf("Q%d", i), Answer: fmt.Sprintf("A%d", i),
			Category: game.CategoryHistory, Difficulty: 1,
		}); err != nil {
			t.Fatalf("seed reservoir: UpsertRafaleQuestion failed: %v", err)
		}
	}

	cardID := "mc-rafale-1"
	card := game.MotionCard{
		ID: cardID, Type: game.QuestionTypeRafale, RectoTheme: "Rafale éclair",
		TypedContent: game.TypedContent{
			RafaleCategories:   []string{string(game.CategoryHistory)},
			RafaleDifficulties: []int{1},
			RafaleMode:         string(game.RafaleModeSolo),
			RafaleQuestionTime: 3, // real 3->2->1 decrease observed below, no shortcut
			RafaleMaxQuestions: 10,
			RafaleRoundTime:    60,
		},
	}
	q := &game.Question{
		ID: "mq1", Question: "MEMOTION+ round", Type: game.QuestionTypeMemotion,
		MotionCards: []game.MotionCard{card}, MotionMode: string(game.MemoryModeSolo),
		Points: "10", Time: "0",
	}
	app.engine.Ready(q.ID, q)
	app.engine.StartImmediate(0)

	baseURL := startEvictionTestServer(t, app) // routes /ws/tv -> ClientTypeTV
	tvConn := dialWS(t, baseURL, "/ws/tv")
	defer app.engine.Stop()

	// MEMOTION_SELECT is allowed from {TV, Anim} only (inbound_allowlist.go)
	// — not Admin, despite this being an admin-driven flow conceptually;
	// MEMOTION_FLIP, right after, IS admin-allowed.
	dispatchAs(t, app, server.ClientTypeAnim, protocol.ActionMotionSelect, protocol.MotionSelectPayload{CardID: cardID})
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionFlip, nil)

	if state := app.engine.GetState(); state.MotionActive.CardID != cardID || state.MotionActive.Type != game.QuestionTypeRafale {
		t.Fatalf("sanity: MotionActive = %+v, want CardID=%s TYPE=RAFALE", state.MotionActive, cardID)
	}

	// Collect RAFALE_TICK frames on the wire for ~3.5s — long enough to
	// observe the seed value (3, set synchronously at FLIP, never itself
	// broadcast as a tick) decrease via the real 1s ticker: 2, then 1. A
	// graceful, non-fataling read loop (unlike readActionMatchingWithin,
	// which the classic-round sibling test uses for exactly ONE expected
	// tick) — this test asserts on however many arrive within the window,
	// rather than pinning an exact count.
	var ticks []protocol.RafaleTickPayload
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		tvConn.SetReadDeadline(deadline)
		_, data, err := tvConn.ReadMessage()
		if err != nil {
			break // timeout — collected whatever arrived, assessed below
		}
		var envelope struct {
			Action string          `json:"ACTION"`
			Msg    json.RawMessage `json:"MSG"`
		}
		if json.Unmarshal(data, &envelope) != nil || envelope.Action != protocol.ActionRafaleTick {
			continue
		}
		var tick protocol.RafaleTickPayload
		if err := json.Unmarshal(envelope.Msg, &tick); err != nil {
			t.Fatalf("failed to unmarshal RAFALE_TICK: %v (raw: %s)", err, envelope.Msg)
		}
		ticks = append(ticks, tick)
	}

	if len(ticks) < 2 {
		t.Fatalf("got %d RAFALE_TICK frames on the wire in 4s, want at least 2 (3->2->1 sequence) — the 'chrono figé' symptom", len(ticks))
	}
	for i, tick := range ticks {
		if tick.MotionCardID != cardID {
			t.Errorf("tick #%d: MOTION_CARD_ID = %q, want %q — C1: a card-hosted tick must carry the card's own ID", i, tick.MotionCardID, cardID)
		}
	}
	for i := 1; i < len(ticks); i++ {
		if ticks[i].QuestionTime >= ticks[i-1].QuestionTime {
			t.Errorf("QUESTION_TIME did not decrease: tick #%d = %d, tick #%d = %d — the exact 'chrono figé' symptom", i-1, ticks[i-1].QuestionTime, i, ticks[i].QuestionTime)
		}
	}
}
