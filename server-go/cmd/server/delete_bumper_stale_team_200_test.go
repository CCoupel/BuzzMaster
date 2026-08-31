// Test for #200 cycle 7 — real end-to-end reproduction of code-review's own
// empirical finding (rapport _work/reports/code-review-20260831-225004.md):
// ACTION:"DELETE_BUMPER", dispatched through the REAL handleWebMessage path
// (not a direct Engine.DeleteBumper() Go call), used to bypass
// purgeInactiveParticipantUnsafe entirely — handleDeleteBumper read the
// engine's LIVE bumper map via GetTeamsAndBumpers() (not a copy), deleted
// the entry IN PLACE, then passed that already-mutated map to SetBumpers,
// whose own old/new diff (#200 cycle 6) then compared a map against itself.
// Following the cycle 5/6 lesson again: only a genuine WS dispatch through
// handleWebMessage exercises this exact aliasing pattern — a hand-built
// map passed directly to Engine.SetBumpers()/DeleteBumper() in Go (as every
// other #200 test in this repo does) can never reproduce it.
//
// Run: go test ./cmd/server/... -run TestDeleteBumper_RealWSDispatch -v
package main

import (
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

func TestDeleteBumper_RealWSDispatch_PurgesStaleSelection(t *testing.T) {
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.SetTeams(map[string]*game.Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
	app.engine.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	app.engine.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})

	q := &game.Question{ID: "mq1", Type: game.QuestionTypeMemory, TypedContent: game.TypedContent{MemoryMode: string(game.MemoryModeSolo)}}
	app.engine.Ready("mq1", q)
	if err := app.engine.SetMemoryParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams: %v", err)
	}
	app.engine.ForceReady()
	if state := app.engine.GetState(); state.Phase != game.PhaseReady {
		t.Fatalf("sanity: expected READY, got %s", state.Phase)
	}

	baseURL := startEvictionTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)

	// The exact production dispatch: ACTION:"DELETE_BUMPER" through the real
	// handleWebMessage switch (handleDeleteBumper -> Engine.DeleteBumper,
	// #200 cycle 7 — was the aliasing GetTeamsAndBumpers()+SetBumpers path).
	sendAction(t, app, admin, protocol.ActionDeleteBumper, protocol.DeletePayload{ID: "b1"})

	state := app.engine.GetState()
	t.Logf("after DELETE_BUMPER(b1): Phase=%s MEMORY_PARTICIPATING_TEAMS=%v", state.Phase, state.MemoryParticipatingTeams)
	if len(state.MemoryParticipatingTeams) != 0 {
		t.Errorf("BUG CONFIRMED: MEMORY_PARTICIPATING_TEAMS still contains a team whose only bumper was DELETED via the real WS path: %v", state.MemoryParticipatingTeams)
	}
	if state.Phase != game.PhasePrepare {
		t.Errorf("expected an immediate READY->PREPARE reversal, got %s", state.Phase)
	}
	if app.engine.ParticipantsConform() {
		t.Errorf("BUG CONFIRMED: ParticipantsConform() still TRUE after DELETE_BUMPER removed the selected team's only bumper")
	}

	// The literal follow-up: a START dispatched now must be refused.
	sendAction(t, app, admin, protocol.ActionStart, protocol.StartPayload{Delay: 10})
	state = app.engine.GetState()
	if state.Phase == game.PhaseCountdown || state.Phase == game.PhaseStarted {
		t.Errorf("BUG CONFIRMED END-TO-END: START succeeded (phase=%s) after DELETE_BUMPER left a stale MEMORY selection, MEMORY_PARTICIPATING_TEAMS=%v", state.Phase, state.MemoryParticipatingTeams)
	}
}
