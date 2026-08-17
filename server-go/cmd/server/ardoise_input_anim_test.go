package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"fmt"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Tests: #158/B1 (T1) — ARDOISE_INPUT coalescer target. dev-backend added
// ClientTypeAnim to the coalescer's emit closure (main.go, ardoiseCoalescer
// construction) so a live ARDOISE game conducted from /anim sees answers
// build up in real time, matching /admin. Complements the pre-existing
// TestBroadcast129_ArdoiseInput_ZeroUpdatesToVPlayerAndTV (main_broadcast_129_test.go,
// untouched by #158 — it only asserts VPlayer=0/TV=0/Admin=1, none of which
// changes by adding Anim to the target list) with the two cases it doesn't
// cover: Anim receives the coalesced update, Buzzer receives nothing.
// ---------------------------------------------------------------------------

func TestArdoiseInput_Anim_ReceivesCoalescedUpdate(t *testing.T) {
	const nTeams = 8
	const inputsPerTeam = 3
	app := newBroadcast127TestApp(t)
	app.engine.SetPhase(game.PhaseEnroll)

	teams := map[string]*game.Team{}
	teamNames := make([]string, 0, nTeams)
	for i := 0; i < nTeams; i++ {
		name := fmt.Sprintf("Team%d", i)
		teams[name] = &game.Team{Name: name}
		teamNames = append(teamNames, name)
	}
	app.engine.SetTeams(teams)

	bumperIDs := make([]string, 0, nTeams)
	for i, name := range teamNames {
		bumperIDs = append(bumperIDs, setupVirtualPlayer(t, app, fmt.Sprintf("Player%d", i), name))
	}

	app.engine.SetPhase(game.PhaseStopped)
	app.engine.Ready("", &game.Question{ID: "q-ardoise", Type: game.QuestionTypeArdoise})
	app.engine.SetPhase(game.PhaseStarted)

	// startAnimAllowlistTestServer (inbound_allowlist_anim_test.go), not
	// startEvictionTestServer: the latter's routing switch predates /ws/anim
	// (#154/#120) and has no "/anim" case, silently falling through to its
	// ClientTypeVPlayer default — exactly the false negative QA diagnosed
	// (_work/reports/qa-20260816-163434.md). The established, ALREADY
	// EXISTING fix for this is startAnimAllowlistTestServer, a deliberate
	// additive twin (see its own doc comment) rather than a modification of
	// the widely-shared startEvictionTestServer (14 other test files depend
	// on it staying exactly as-is, per the project's non-regression rule for
	// test infra). Using the twin here instead of editing the shared helper.
	baseURL := startAnimAllowlistTestServer(t, app)
	animConn := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, animConn)
	buzzerConn := dialWS(t, baseURL, "/ws/buzzer")
	learnClientID(t, app, buzzerConn)

	total := 0
	for _, id := range bumperIDs {
		for i := 0; i < inputsPerTeam; i++ {
			app.handleArdoiseInput(id, ardoiseInputMsg(t, id, fmt.Sprintf("réponse %d", i)))
			total++
		}
	}

	// #158 B1: Anim now receives exactly 1 coalesced UPDATE, same discipline
	// as Admin (ardoiseCoalesceWindow = 150ms).
	animActions := collectActions(animConn, 500*time.Millisecond)
	if got := countActions(animActions, protocol.ActionUpdate); got != 1 {
		t.Errorf("#158/B1: Anim should receive exactly 1 coalesced UPDATE for %d ARDOISE_INPUT, got %d (actions=%v)", total, got, animActions)
	}

	// Buzzer stays excluded — SerializeForBuzzer carries no ARDOISE field at
	// all (unchanged reasoning, #129 T1.5), #158 does not add it.
	buzzerActions := collectActions(buzzerConn, 500*time.Millisecond)
	if got := countActions(buzzerActions, protocol.ActionUpdate); got != 0 {
		t.Errorf("#158/B1: Buzzer should receive 0 UPDATE from %d ARDOISE_INPUT, got %d (actions=%v)", total, got, buzzerActions)
	}
}
