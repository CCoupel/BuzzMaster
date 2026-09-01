// Test for #200 cycle 5 — empirical, end-to-end reproduction of the QUALIF
// report (v8.0.0.18): the LITERAL START button starts a MEMORY manche with
// NO team ever selected, on BOTH /admin and /anim. NOT the CONTINUE bypass
// (cycles 3/4, already fixed and covered) — this must go through the REAL
// WS message sequence a client sends (READY, PONG from each bumper, START),
// through the REAL production code path (handleWebMessage -> handleReady ->
// a.loadQuestion() reading an ACTUAL question.json from disk, exactly like
// handleUploadQuestion writes it — every prior test for this class of bug
// hand-built a *game.Question{Type: QuestionTypeMemory, ...} directly in Go,
// which can never catch a TYPE-field/serialization-shaped gap), NEVER
// calling SetMemoryParticipatingTeams.
//
// Run: go test ./cmd/server/... -run TestReproMemoryStart_NoTeamSelected -v
package main

import (
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
)

// TestReproMemoryStart_NoTeamSelected_RealDiskQuestion_RealWSSequence is the
// literal repro: write a MEMORY question.json to disk exactly as
// handleUploadQuestion would produce it, dispatch the real WS sequence
// (READY, PONG from the one registered bumper, START) through
// handleWebMessage, NEVER touching SetMemoryParticipatingTeams — and check
// GameState.MEMORY_PARTICIPATING_TEAMS and Phase at every step, from BOTH
// /admin and /anim (per the user's report).
func TestReproMemoryStart_NoTeamSelected_RealDiskQuestion_RealWSSequence(t *testing.T) {
	app := newNextQuestionTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{"red": {Name: "red"}})
	app.engine.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "mq1", nil, map[string]interface{}{
		"TYPE":        "MEMORY",
		"MEMORY_MODE": "SOLO",
		"MEMORY_PAIRS": []map[string]interface{}{
			{
				"ID":    1,
				"CARD1": map[string]interface{}{"TEXT": "Paris"},
				"CARD2": map[string]interface{}{"TEXT": "France"},
			},
		},
	})

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	logState := func(label string) game.GameState {
		state := app.engine.GetState()
		qType := game.QuestionType("")
		qMode := ""
		if state.Question != nil {
			qType = state.Question.Type
			qMode = state.Question.MemoryMode
		}
		t.Logf("%s: Phase=%s Question.Type=%q MemoryMode=%q MEMORY_PARTICIPATING_TEAMS=%v",
			label, state.Phase, qType, qMode, state.MemoryParticipatingTeams)
		return state
	}

	// Step 1: READY, exactly as GamePage.jsx's question-selection click sends
	// — goes through the REAL a.loadQuestion() disk-reading path, not a
	// hand-built Go struct.
	sendAction(t, app, admin, protocol.ActionReady, protocol.ReadyPayload{Question: "mq1"})

	state := logState("after READY (/admin)")
	if state.Phase != game.PhasePrepare {
		t.Fatalf("sanity: expected PREPARE after READY, got %s", state.Phase)
	}
	if state.Question == nil || state.Question.Type != game.QuestionTypeMemory {
		t.Fatalf("sanity: loadQuestion() did not produce a MEMORY question — Question=%+v (TYPE field/serialization bug?)", state.Question)
	}
	if len(state.MemoryParticipatingTeams) != 0 {
		t.Fatalf("BUG (unexpected at this step): MEMORY_PARTICIPATING_TEAMS is non-empty right after READY, before ANY selection was ever made: %v", state.MemoryParticipatingTeams)
	}
	if app.engine.ParticipantsConform() {
		t.Errorf("BUG: ParticipantsConform() is TRUE right after READY with zero teams ever selected — question.Type=%q, MemoryMode=%q", state.Question.Type, state.Question.MemoryMode)
	}

	// Step 2: PONG from the one registered bumper — exactly what a real
	// buzzer (or a web client simulating one) sends, driving handlePong's
	// AreAllTeamsReady()+ParticipantsConform() combined gate. Never call
	// SetMemoryParticipatingTeams — this is the whole point of the repro.
	sendAction(t, app, admin, protocol.ActionPong, map[string]interface{}{"ID": "b1"})

	state = logState("after PONG(b1) (/admin)")
	if state.Phase == game.PhaseReady {
		t.Errorf("BUG CONFIRMED: PONG alone moved the engine to READY with MEMORY_PARTICIPATING_TEAMS=%v — participantsConform gate bypassed", state.MemoryParticipatingTeams)
	}
	if state.Phase != game.PhasePrepare {
		t.Fatalf("unexpected phase after PONG: %s (want PREPARE, unless the bug above already fired)", state.Phase)
	}

	// Step 3: the literal START button, exactly as GamePage.jsx/AnimPage send it.
	sendAction(t, app, admin, protocol.ActionStart, protocol.StartPayload{Delay: 10})

	state = logState("after START (/admin)")
	if state.Phase == game.PhaseCountdown || state.Phase == game.PhaseStarted {
		t.Errorf("BUG CONFIRMED: literal ACTION:\"START\" started a MEMORY manche with ZERO teams ever selected (MEMORY_PARTICIPATING_TEAMS=%v, Phase=%s) — exact QUALIF v8.0.0.18 report reproduced end-to-end via /admin", state.MemoryParticipatingTeams, state.Phase)
	} else {
		t.Logf("Start() correctly refused — engine stayed in %s", state.Phase)
	}
	app.engine.Stop()

	// Same repro again, this time driving READY/PONG/START from /anim
	// instead of /admin, per the user's report ("les deux interfaces /admin
	// et /anim").
	app.engine.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	sendAction(t, app, anim, protocol.ActionReady, protocol.ReadyPayload{Question: "mq1"})
	sendAction(t, app, anim, protocol.ActionPong, map[string]interface{}{"ID": "b1"})
	sendAction(t, app, anim, protocol.ActionStart, protocol.StartPayload{Delay: 10})

	state = logState("after /anim READY+PONG+START")
	if state.Phase == game.PhaseCountdown || state.Phase == game.PhaseStarted {
		t.Errorf("BUG CONFIRMED via /anim as well: MEMORY_PARTICIPATING_TEAMS=%v, Phase=%s", state.MemoryParticipatingTeams, state.Phase)
	}
	if state.Phase == game.PhaseCountdown || state.Phase == game.PhaseStarted {
		app.engine.Stop()
	}
}

// TestReproMemoryReplay_ZeroFlipStartStop_NoTeamSelected_RealDiskQuestion_RealWSSequence
// closes the SAME methodological gap as the test above, but for the ACTUAL
// #200 cycle 5 root cause specifically — not the "never played" case (already
// covered above and, at the engine level, by
// engine_participants_conform_test.go), but the "played-then-immediately-
// stopped-with-zero-gameplay-action, THEN replayed" case that eluded cycles
// 1-4 (engine_ready_replay_200_test.go's own
// TestReady_MemoryReplay_ZeroFlipStartStop_ResetsStaleParticipatingTeams
// covers this at the engine level with hand-built *game.Question{} values —
// this is its real-disk, real-WS-sequence counterpart, following the exact
// lesson of this cycle: a hand-built Go struct can never catch a
// TYPE-field/serialization-shaped gap, only a genuine round-trip can).
//
// Sequence, entirely via handleWebMessage (never calling
// SetMemoryParticipatingTeams, engine.Ready()/StartImmediate()/Stop()
// directly, or any other Go-level shortcut):
//  1. READY, MEMORY_SET_TEAMS, PONG, START — legitimate manche WITH a team
//     selected, waited through the REAL countdown goroutine (not
//     StartImmediate) so questionEverStarted (the cycle 5 fix itself, only
//     set inside actualStart()) genuinely fires.
//  2. STOP immediately — zero FLIP_MEMORY_CARD ever sent (the exact "wrong
//     config, oops, stop" ordinary workflow the cycle 5 handoff describes).
//  3. READY the SAME question ID again, WITHOUT resending MEMORY_SET_TEAMS.
//  4. PONG, START again — must be refused (stay in PREPARE), never reach
//     COUNTDOWN/STARTED on the stale selection from step 1.
func TestReproMemoryReplay_ZeroFlipStartStop_NoTeamSelected_RealDiskQuestion_RealWSSequence(t *testing.T) {
	app := newNextQuestionTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
	app.engine.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	app.engine.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})

	dir := app.config.Storage.QuestionsDir
	writeQuestionFile(t, dir, "mq1", nil, map[string]interface{}{
		"TYPE":        "MEMORY",
		"MEMORY_MODE": "CHACUN_SON_TOUR",
		// MEMORIZE_TIME=1 (default 5) keeps Engine.Start()'s real countdown
		// short — engine.go:1498-1518 computes total countdown as
		// cascade_reveal + memorize_time + cascade_hide; with a single pair
		// (2 cards) cascade rounds up to 1s each, so total=3s here, matching
		// the ordinary 3s countdown of every other question type and kept
		// well within this test's polling deadline below.
		"MEMORY_CONFIG": map[string]interface{}{"MEMORIZE_TIME": 1},
		"MEMORY_PAIRS": []map[string]interface{}{
			{
				"ID":    1,
				"CARD1": map[string]interface{}{"TEXT": "Paris"},
				"CARD2": map[string]interface{}{"TEXT": "France"},
			},
		},
	})

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)

	logState := func(label string) game.GameState {
		state := app.engine.GetState()
		t.Logf("%s: Phase=%s MEMORY_PARTICIPATING_TEAMS=%v", label, state.Phase, state.MemoryParticipatingTeams)
		return state
	}

	// Step 1 — legitimate first manche, WITH a real team selection, driven
	// entirely through the WS protocol (MEMORY_SET_TEAMS is the real
	// action AnimConductPanel/GamePage send — never SetMemoryParticipatingTeams
	// called directly in Go).
	sendAction(t, app, admin, protocol.ActionReady, protocol.ReadyPayload{Question: "mq1"})
	sendAction(t, app, admin, protocol.ActionMemorySetTeams, protocol.MemorySetTeamsPayload{Teams: []string{"red", "blue"}})
	sendAction(t, app, admin, protocol.ActionPong, map[string]interface{}{"ID": "b1"})
	sendAction(t, app, admin, protocol.ActionPong, map[string]interface{}{"ID": "b2"})

	state := logState("after READY+teams+PONGx2 (first manche)")
	if state.Phase != game.PhaseReady {
		t.Fatalf("sanity: expected READY with both teams selected and ready, got %s (MEMORY_PARTICIPATING_TEAMS=%v)", state.Phase, state.MemoryParticipatingTeams)
	}

	sendAction(t, app, admin, protocol.ActionStart, protocol.StartPayload{Delay: 30})
	state = logState("after dispatching START (first manche)")
	if state.Phase != game.PhaseCountdown {
		t.Fatalf("sanity: expected COUNTDOWN right after dispatching START, got %s", state.Phase)
	}

	// The real START goes through a genuine countdown goroutine
	// (Engine.Start -> actualStart(), NOT StartImmediate) — questionEverStarted
	// (the cycle 5 fix itself) is only set inside actualStart(), once the
	// countdown genuinely completes. Poll for it rather than assume Start()
	// is synchronous, same pattern as rafale_countdown_wire_test.go's own
	// wait for GAME.PHASE=="STARTED".
	deadline := time.Now().Add(6 * time.Second)
	for {
		state = app.engine.GetState()
		if state.Phase == game.PhaseStarted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for the real countdown to reach STARTED (last seen phase=%s)", state.Phase)
		}
		time.Sleep(50 * time.Millisecond)
	}
	logState("after real countdown completed (first manche)")

	// Step 2 — STOP immediately, WITHOUT ever sending FLIP_MEMORY_CARD. This
	// is the exact "click START then immediately STOP" workflow identified
	// as the real root cause.
	sendAction(t, app, admin, protocol.ActionStop, struct{}{})
	state = logState("after STOP (zero FLIP_MEMORY_CARD ever sent)")
	if state.Phase != game.PhaseStopped {
		t.Fatalf("sanity: expected STOPPED, got %s", state.Phase)
	}
	if len(state.MemoryFlippedCards) != 0 || len(state.MemoryMatchedPairs) != 0 || state.MemoryErrors != 0 {
		t.Fatalf("sanity: expected zero gameplay progress (the whole point of this repro), got FlippedCards=%v MatchedPairs=%v Errors=%d",
			state.MemoryFlippedCards, state.MemoryMatchedPairs, state.MemoryErrors)
	}

	// Step 3 — replay the SAME question ID for a NEW manche, WITHOUT
	// resending MEMORY_SET_TEAMS.
	sendAction(t, app, admin, protocol.ActionReady, protocol.ReadyPayload{Question: "mq1"})
	state = logState("after READY (replay, no reselection)")
	if len(state.MemoryParticipatingTeams) != 0 {
		t.Errorf("BUG CONFIRMED: MEMORY_PARTICIPATING_TEAMS survived a real WS zero-flip Start+Stop+replay sequence: %v", state.MemoryParticipatingTeams)
	}
	if app.engine.ParticipantsConform() {
		t.Errorf("BUG: ParticipantsConform() is TRUE on replay with a stale selection from a zero-flip aborted manche")
	}

	// Step 4 — PONG both bumpers again, then the literal START button. Must
	// be refused — this is the exact end-to-end reproduction of the QUALIF
	// v8.0.0.18 report's real root cause (cycle 5).
	sendAction(t, app, admin, protocol.ActionPong, map[string]interface{}{"ID": "b1"})
	sendAction(t, app, admin, protocol.ActionPong, map[string]interface{}{"ID": "b2"})
	sendAction(t, app, admin, protocol.ActionStart, protocol.StartPayload{Delay: 0})

	state = logState("after replay PONGx2+START (no reselection)")
	if state.Phase == game.PhaseCountdown || state.Phase == game.PhaseStarted {
		t.Errorf("BUG CONFIRMED: literal ACTION:\"START\" restarted a MEMORY manche on a REPLAY with a stale, zero-flip-aborted team selection (MEMORY_PARTICIPATING_TEAMS=%v, Phase=%s) — exact QUALIF v8.0.0.18 root cause reproduced end-to-end via real disk+WS", state.MemoryParticipatingTeams, state.Phase)
	} else {
		t.Logf("Start() correctly refused on replay — engine stayed in %s", state.Phase)
	}
}
