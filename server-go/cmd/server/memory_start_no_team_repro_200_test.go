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
