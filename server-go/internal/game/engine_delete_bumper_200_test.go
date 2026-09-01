// Tests for #200 cycle 7 (code-review 20260831-225004, CRITIQUE on cycle
// 6/7e1a5746) — DELETE_BUMPER used to bypass purgeInactiveParticipantUnsafe
// entirely via a map-aliasing bug: the previous production call site
// (cmd/server/main.go's handleDeleteBumper) read e.data.Bumpers via
// GetTeamsAndBumpers() — which returns e.data directly, NOT a copy —
// deleted the entry IN PLACE on that LIVE map, and only THEN called
// SetBumpers with the ALREADY-mutated map. SetBumpers's own
// oldBumpers/newBumpers diff (#200 cycle 6) then compared two references
// to the exact same map: no diff ever detected, no purge. Engine.DeleteBumper
// (new, this cycle) does delete + diff + purge atomically under its own
// lock, on data nothing outside the engine can alias.
//
// Run: go test ./internal/game/... -run TestDeleteBumper -v
package game

import "testing"

// TestDeleteBumper_PurgesStaleSelection is the engine-level reproduction of
// code-review's own empirical finding: MEMORY SOLO, team selected via its
// only bumper, READY reached, then that bumper is DELETED (not reassigned)
// — the stale selection must be purged and the gate reverted immediately.
func TestDeleteBumper_PurgesStaleSelection(t *testing.T) {
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

	deleted := e.DeleteBumper("b1")
	if deleted == nil {
		t.Fatalf("expected DeleteBumper to return the deleted bumper, got nil")
	}
	if deleted.Team != "red" {
		t.Errorf("expected the returned bumper to still report its team (red) for the caller's own bookkeeping, got %q", deleted.Team)
	}

	state := e.GetState()
	if len(state.MemoryParticipatingTeams) != 0 {
		t.Errorf("BUG: MEMORY_PARTICIPATING_TEAMS still contains a team whose only bumper was DELETED: %v", state.MemoryParticipatingTeams)
	}
	if state.MemoryCurrentTeam != "" {
		t.Errorf("expected MEMORY_CURRENT_TEAM to be cleared, got %q", state.MemoryCurrentTeam)
	}
	if e.GetPhase() != PhasePrepare {
		t.Errorf("expected an immediate READY->PREPARE reversal, got %s", e.GetPhase())
	}
	if e.ParticipantsConform() {
		t.Errorf("BUG: ParticipantsConform() still TRUE after deleting the only bumper of the selected team")
	}

	e.Start(10)
	if e.GetPhase() == PhaseCountdown || e.GetPhase() == PhaseStarted {
		t.Errorf("BUG: Start() succeeded (phase=%s) after DELETE_BUMPER removed the selected team's only bumper", e.GetPhase())
	}
}

// TestDeleteBumper_UnknownID_ReturnsNilNoop is the negative control: an
// unknown bumper ID must be a clean no-op (nil return, no panic, no state
// mutation) — mirrors handleDeleteBumper's own "not found" branch.
func TestDeleteBumper_UnknownID_ReturnsNilNoop(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	if got := e.DeleteBumper("does-not-exist"); got != nil {
		t.Errorf("expected nil for an unknown bumper ID, got %+v", got)
	}
	if _, exists := e.GetTeamsAndBumpers().Bumpers["b1"]; !exists {
		t.Errorf("expected b1 to be untouched by a no-op delete")
	}
}

// TestDeleteBumper_OtherBumperOnSameTeam_DoesNotPurge is the positive
// control: deleting ONE of a team's several bumpers must NOT purge that
// team's participant selection — only losing the LAST one should.
func TestDeleteBumper_OtherBumperOnSameTeam_DoesNotPurge(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("b2", map[string]interface{}{"TEAM": "red"})

	q := &Question{ID: "mq1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}}
	e.Ready("mq1", q)
	if err := e.SetMemoryParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams: %v", err)
	}

	e.DeleteBumper("b1") // red still has b2

	state := e.GetState()
	if len(state.MemoryParticipatingTeams) != 1 || state.MemoryParticipatingTeams[0] != "red" {
		t.Errorf("expected 'red' to remain selected (b2 still assigned to it), got %v", state.MemoryParticipatingTeams)
	}
}

// TestDeleteBumper_DoesNotPurgeDuringLiveRound is the STARTED/PAUSED
// negative control, mirroring purgeInactiveParticipantUnsafe's own scope
// restriction for the reassignment paths (UpdateBumper/SetBumpers).
func TestDeleteBumper_DoesNotPurgeDuringLiveRound(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

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

	e.DeleteBumper("b1")

	if state := e.GetState(); len(state.MemoryParticipatingTeams) != 1 || state.MemoryParticipatingTeams[0] != "red" {
		t.Errorf("expected MEMORY_PARTICIPATING_TEAMS to be left untouched during a live STARTED round, got %v", state.MemoryParticipatingTeams)
	}
	if e.GetPhase() != PhaseStarted {
		t.Errorf("expected Phase to stay STARTED, got %s", e.GetPhase())
	}
}
