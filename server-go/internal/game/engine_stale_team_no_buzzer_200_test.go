// Tests for #200 cycle 6 — the ACTUAL root cause traced by dev-frontend
// (handoff _work/handoff/dev-frontend-20260831-214522.md): GamePage.jsx's
// team-chip rendering filters candidates through teamsWithBuzzers BEFORE
// splitting them into "selected"/"available" columns, so a team that stays
// in MEMORY_PARTICIPATING_TEAMS server-side but loses its ONLY bumper
// (reassigned from TeamsPage, an entirely ordinary admin action — no
// Stop/Ready/replay involved at all) becomes invisible in BOTH columns.
// The user sees "no team selected"; the server still thinks a team is
// selected, because participantsConform (engine.go) only ever counted
// MemoryParticipatingTeams/MotionParticipatingTeams/RafaleParticipatingTeams
// by NAME — never checked whether those teams still have any buzzer.
//
// Confirmed empirically (throwaway test, prior to this fix) reproducible in
// TWO lines: SetMemoryParticipatingTeams(["red"]), then
// UpdateBumper(b1, {"TEAM":"blue"}) reassigning red's only bumper away —
// ParticipantsConform() stayed true, and Start() succeeded.
//
// Run: go test ./internal/game/... -run TestMemorySolo_ReassignSelectedTeamBumper\|TestSetBumpers_BulkReassign\|TestPurgeInactiveParticipant -v
package game

import "testing"

// TestMemorySolo_ReassignSelectedTeamBumper_PurgesStaleSelection is the
// direct, minimal reproduction: select "red" as the SOLO participating
// team, then reassign its only bumper away via UpdateBumper (single-bumper
// path, e.g. GamePage's per-bumper team dropdown) — the stale name must be
// purged, and the PREPARE<->READY gate re-evaluated immediately.
func TestMemorySolo_ReassignSelectedTeamBumper_PurgesStaleSelection(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})

	q := &Question{ID: "mq1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}}
	e.Ready("mq1", q)
	if err := e.SetMemoryParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams: %v", err)
	}
	if !e.ParticipantsConform() {
		t.Fatalf("precondition: red selected alone in SOLO mode should conform")
	}

	// Reassign red's ONLY bumper away to blue — an ordinary TeamsPage/GamePage
	// action, nothing to do with Stop/Ready/replay.
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "blue"})

	state := e.GetState()
	if len(state.MemoryParticipatingTeams) != 0 {
		t.Errorf("expected stale 'red' (zero buzzers now) to be purged from MEMORY_PARTICIPATING_TEAMS, got %v", state.MemoryParticipatingTeams)
	}
	if state.MemoryCurrentTeam != "" {
		t.Errorf("expected MEMORY_CURRENT_TEAM to be cleared alongside the purge, got %q", state.MemoryCurrentTeam)
	}
	if e.ParticipantsConform() {
		t.Errorf("BUG: ParticipantsConform() still true after the selected team lost its last bumper")
	}

	// End-to-end: the combined PREPARE->READY gate (handlePong's own logic)
	// must correctly refuse now, and so must Start().
	e.SetBumperReady("b1")
	e.SetBumperReady("b2")
	if e.AreAllTeamsReady() && e.ParticipantsConform() {
		e.TransitionToReady()
	}
	if e.GetPhase() != PhasePrepare {
		t.Errorf("expected to stay in PREPARE (blue was never explicitly selected for MEMORY), got %s", e.GetPhase())
	}
	e.Start(10)
	if e.GetPhase() == PhaseCountdown || e.GetPhase() == PhaseStarted {
		t.Errorf("BUG: Start() succeeded (phase=%s) with a MEMORY SOLO selection pointing at a team with zero buzzers", e.GetPhase())
	}
}

// TestMemorySolo_ReassignSelectedTeamBumper_ImmediateGateRecheck verifies
// the purge ALSO immediately re-evaluates READY->PREPARE if the round had
// already reached READY before the reassignment (not just the PREPARE case
// above) — mirrors reevaluatePrepareReadyUnsafe's own READY branch.
func TestMemorySolo_ReassignSelectedTeamBumper_ImmediateGateRecheck(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})

	q := &Question{ID: "mq1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}}
	e.Ready("mq1", q)
	if err := e.SetMemoryParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams: %v", err)
	}
	e.ForceReady()
	if e.GetPhase() != PhaseReady {
		t.Fatalf("sanity: expected READY, got %s", e.GetPhase())
	}

	// Reassign red's only bumper away WHILE already in READY.
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "blue"})

	if e.GetPhase() != PhasePrepare {
		t.Errorf("expected an immediate READY->PREPARE reversal (reevaluatePrepareReadyUnsafe) once the selected team lost its last bumper, got %s", e.GetPhase())
	}
}

// TestSetBumpers_BulkReassign_PurgesStaleSelection mirrors the direct
// reproduction above but through SetBumpers (the BULK path) — the real
// production entry point for a TeamsPage save (handleFullUpdate ->
// a.engine.SetBumpers(data.Bumpers), cmd/server/main.go), distinct code from
// UpdateBumper's single-bumper path and therefore needing its own coverage.
func TestSetBumpers_BulkReassign_PurgesStaleSelection(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
	e.SetBumpers(map[string]*Bumper{
		"b1": {Team: "red"},
		"b2": {Team: "blue"},
	})

	q := &Question{ID: "mq1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}}
	e.Ready("mq1", q)
	if err := e.SetMemoryParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams: %v", err)
	}

	// Bulk reassignment (TeamsPage save flow) — red's bumper moves to blue.
	e.SetBumpers(map[string]*Bumper{
		"b1": {Team: "blue"},
		"b2": {Team: "blue"},
	})

	state := e.GetState()
	if len(state.MemoryParticipatingTeams) != 0 {
		t.Errorf("expected stale 'red' to be purged via the BULK SetBumpers path too, got %v", state.MemoryParticipatingTeams)
	}
	if e.ParticipantsConform() {
		t.Errorf("BUG: ParticipantsConform() still true after a bulk reassignment removed the selected team's only bumper")
	}
}

// TestPurgeInactiveParticipant_MemotionAndRafale_AlsoCovered extends the
// same reproduction to MEMOTION and RAFALE — the exact same structural gap
// (participantsConform counts *_PARTICIPATING_TEAMS by name only, for all
// three types), fixed by the same purgeInactiveParticipantUnsafe call sites.
func TestPurgeInactiveParticipant_MemotionAndRafale_AlsoCovered(t *testing.T) {
	t.Run("MEMOTION", func(t *testing.T) {
		e := NewEngine()
		e.SetTeams(map[string]*Team{"red": {Name: "red"}})
		e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

		q := makeMotionQuestion("mo1", defaultMotionCards(), "SOLO")
		e.Ready("mo1", q)
		if err := e.SetMotionParticipatingTeams([]string{"red"}); err != nil {
			t.Fatalf("SetMotionParticipatingTeams: %v", err)
		}

		e.UpdateBumper("b1", map[string]interface{}{"TEAM": ""})

		if len(e.GetState().MotionParticipatingTeams) != 0 {
			t.Errorf("expected stale 'red' to be purged from MEMOTION_PARTICIPATING_TEAMS, got %v", e.GetState().MotionParticipatingTeams)
		}
		if e.ParticipantsConform() {
			t.Errorf("BUG: ParticipantsConform() still true for MEMOTION after the only participating team lost its bumper")
		}
	})

	t.Run("RAFALE", func(t *testing.T) {
		e := NewEngine()
		e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
		e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
		e.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})
		seedRafaleReservoirBulk(t, e, 20, CategoryHistory, 1)

		q := makeRafaleQuestion("rq1", string(RafaleModeChacunSonTour), CategoryHistory, 1)
		e.Ready("rq1", q)
		if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
			t.Fatalf("SetRafaleParticipatingTeams: %v", err)
		}

		e.UpdateBumper("b1", map[string]interface{}{"TEAM": "blue"})

		if len(e.GetState().RafaleParticipatingTeams) != 0 {
			t.Errorf("expected stale 'red' to be purged from RAFALE_PARTICIPATING_TEAMS, got %v", e.GetState().RafaleParticipatingTeams)
		}
		if e.ParticipantsConform() {
			t.Errorf("BUG: ParticipantsConform() still true for RAFALE after the only participating team lost its bumper")
		}
	})
}

// TestPurgeInactiveParticipant_NotAppliedOutsidePrepareOrReady is the
// negative control: a reassignment during STARTED/PAUSED (a live round in
// progress) must NEVER touch the participant selection — that is an
// unrelated concern, and purging mid-round data would be actively harmful
// (see purgeInactiveParticipantUnsafe's own doc comment).
func TestPurgeInactiveParticipant_NotAppliedOutsidePrepareOrReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})

	q := &Question{ID: "mq1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}}
	e.Ready("mq1", q)
	if err := e.SetMemoryParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams: %v", err)
	}
	e.ForceReady()
	e.StartImmediate(0)
	if e.GetPhase() != PhaseStarted {
		t.Fatalf("sanity: expected STARTED, got %s", e.GetPhase())
	}
	defer e.Stop()

	// Reassign red's only bumper away WHILE the round is live.
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "blue"})

	if state := e.GetState(); len(state.MemoryParticipatingTeams) != 1 || state.MemoryParticipatingTeams[0] != "red" {
		t.Errorf("expected MEMORY_PARTICIPATING_TEAMS to be left untouched during a live STARTED round, got %v", state.MemoryParticipatingTeams)
	}
	if e.GetPhase() != PhaseStarted {
		t.Errorf("expected Phase to stay STARTED (never reverted mid-round), got %s", e.GetPhase())
	}
}
