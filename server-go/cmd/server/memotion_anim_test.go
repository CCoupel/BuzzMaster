package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests: #160/B1 — the five MEMOTION conduct actions (Select/Flip/StopTimer/
// Reveal/Done) widened to ClientTypeAnim (T8c end-to-end complement to
// internal/server's TestIsActionAllowed_Anim table rows, T8a). Mirrors
// flip_memory_card_anim_test.go (#159/B1): a message sent as
// ClientType=anim must traverse the real handleWebMessage dispatch path
// (not a direct engine call) so the allow-list gate is actually exercised,
// and must genuinely mutate MEMOTION_SUBPHASE.
//
// Uses newAnimTestApp / startAnimAllowlistTestServer
// (cmd/server/inbound_allowlist_anim_test.go) — newAnimTestApp initialises
// app.logger and app.udpBcast, both unconditionally dereferenced by
// handleWebMessage for any ClientTypeAnim sender (#155/#156, main.go ~1028)
// and by the "conduite" handlers' broadcast path — the exact pitfall #159
// hit (commit f1dc097): a nil logger panics before the real dispatch runs.
// ---------------------------------------------------------------------------

// motionTestQuestion returns a 2-card MEMOTION question, mode SOLO, ready to
// drive through the five sub-phases (mirrors internal/game's
// defaultMotionCards/makeMotionQuestion helpers, duplicated locally since
// cmd/server cannot import internal/game's _test.go helpers).
func motionTestQuestion(id string) *game.Question {
	return &game.Question{
		ID:       id,
		Question: "MEMOTION question " + id,
		Type:     game.QuestionTypeMemotion,
		MotionCards: []game.MotionCard{
			{ID: "mc-1", RectoTheme: "Science", Difficulty: 1, QuestionText: "Q1", AnswerText: "A1"},
			{ID: "mc-2", RectoTheme: "Histoire", Difficulty: 2, QuestionText: "Q2", AnswerText: "A2"},
		},
		MotionMode: string(game.MemoryModeSolo),
		Points:     "10",
		Time:       "0",
	}
}

// startMotionAtGrid brings app.engine to Phase=STARTED, MotionSubPhase=GRID,
// mirroring internal/game's startMEMOTION helper (Ready -> StartImmediate ->
// InitMotionState) via the exported Engine API from cmd/server.
func startMotionAtGrid(t *testing.T, app *App, id string, q *game.Question) {
	t.Helper()
	app.engine.Ready(id, q)
	app.engine.StartImmediate(0)
	app.engine.InitMotionState()
}

// T8c — the capability widening actually works end-to-end: MEMOTION_SELECT
// sent from /ws/anim during GRID subphase mutates MEMOTION_SUBPHASE, same as
// TV (the admin preview iframe).
func TestHandleWebMessage_AllowList_AnimCanSelectMotionCard(t *testing.T) {
	app := newAnimTestApp(t)
	startMotionAtGrid(t, app, "mq-1", motionTestQuestion("mq-1"))

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, anim, protocol.ActionMotionSelect, protocol.MotionSelectPayload{CardID: "mc-1"})

	state := app.engine.GetState()
	if state.MotionSubPhase != "SELECTED" {
		t.Errorf("#160/B1: anim must be able to send MEMOTION_SELECT — MEMOTION_SUBPHASE=%q, want %q", state.MotionSubPhase, "SELECTED")
	}
	if state.MotionSelected != "mc-1" {
		t.Errorf("#160/B1: MEMOTION_SELECTED=%q, want %q", state.MotionSelected, "mc-1")
	}
}

// T8c — anim can drive the full SELECTED -> QUESTION -> REVEAL chain
// (MEMOTION_FLIP / MEMOTION_REVEAL), each mutating MEMOTION_SUBPHASE in
// turn, exactly as /admin already can.
func TestHandleWebMessage_AllowList_AnimCanFlipAndRevealMotionCard(t *testing.T) {
	app := newAnimTestApp(t)
	startMotionAtGrid(t, app, "mq-1", motionTestQuestion("mq-1"))
	if err := app.engine.SelectMotionCard("mc-1"); err != nil {
		t.Fatalf("setup: SelectMotionCard failed: %v", err)
	}

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, anim, protocol.ActionMotionFlip, struct{}{})
	if got := app.engine.GetState().MotionSubPhase; got != "QUESTION" {
		t.Fatalf("#160/B1: MEMOTION_FLIP from anim — MEMOTION_SUBPHASE=%q, want %q", got, "QUESTION")
	}

	sendAction(t, app, anim, protocol.ActionMotionReveal, struct{}{})
	if got := app.engine.GetState().MotionSubPhase; got != "REVEAL" {
		t.Errorf("#160/B1: MEMOTION_REVEAL from anim — MEMOTION_SUBPHASE=%q, want %q", got, "REVEAL")
	}
}

// T8c — MEMOTION_DONE from anim closes the card and rotates state, same as
// admin (AC7 is verified separately by the frontend test suite: no manual
// team-credit control renders during a MEMOTION round — this test only
// covers the backend allow-list widening).
func TestHandleWebMessage_AllowList_AnimCanDoneMotionCard(t *testing.T) {
	app := newAnimTestApp(t)
	startMotionAtGrid(t, app, "mq-1", motionTestQuestion("mq-1"))
	if err := app.engine.SelectMotionCard("mc-1"); err != nil {
		t.Fatalf("setup: SelectMotionCard failed: %v", err)
	}
	if err := app.engine.FlipMotionCard(); err != nil {
		t.Fatalf("setup: FlipMotionCard failed: %v", err)
	}
	if err := app.engine.RevealMotionCard(); err != nil {
		t.Fatalf("setup: RevealMotionCard failed: %v", err)
	}

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, anim, protocol.ActionMotionDone, protocol.MotionDonePayload{CardID: "mc-1", WinnerTeam: "TeamA"})

	state := app.engine.GetState()
	if state.MotionCardStates["mc-1"] != "DONE" {
		t.Errorf("#160/B1: MEMOTION_DONE from anim — card mc-1 state=%q, want %q", state.MotionCardStates["mc-1"], "DONE")
	}
}

// T8c — MEMOTION_STOP_TIMER from anim stops the per-card timer without
// erroring (does not itself change MEMOTION_SUBPHASE — matches
// handleMotionStopTimer's contract) — exercised end-to-end to confirm the
// allow-list gate lets it through for anim.
func TestHandleWebMessage_AllowList_AnimCanStopMotionTimer(t *testing.T) {
	app := newAnimTestApp(t)
	startMotionAtGrid(t, app, "mq-1", motionTestQuestion("mq-1"))
	if err := app.engine.SelectMotionCard("mc-1"); err != nil {
		t.Fatalf("setup: SelectMotionCard failed: %v", err)
	}
	if err := app.engine.FlipMotionCard(); err != nil {
		t.Fatalf("setup: FlipMotionCard failed: %v", err)
	}
	app.engine.StartMotionCardTimer(30)

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, anim, protocol.ActionMotionStopTimer, struct{}{})

	if got := app.engine.GetState().MotionSubPhase; got != "QUESTION" {
		t.Errorf("#160/B1: MEMOTION_STOP_TIMER from anim must not change MEMOTION_SUBPHASE — got %q, want %q", got, "QUESTION")
	}
}
