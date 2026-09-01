// Tests for #200 — generalization of the RAFALE Ready()-replay bugfix
// (#199, SHA a7b70057) to MEMORY and MEMOTION.
//
// a7b70057 fixed Ready() only resetting RafaleParticipatingTeams/
// RafaleCurrentTeam/RafaleCurrentTeamColor when isNewQuestion==true — never
// on a same-ID replay of an already-played question — leaving a STALE team
// selection able to silently satisfy participantsConform (engine.go) on a
// replay, even though the UI shows no selection. That commit's own message
// flagged MEMORY/MEMOTION as sharing the exact same isNewQuestion-only
// staleness gap for their own participant-selection fields
// (MemoryParticipatingTeams/MemoryCurrentTeam/... and
// MotionParticipatingTeams/MotionCurrentTeam/...), audited and confirmed by
// #200. The first tests in this file mirror rafale_modes_test.go's own
// TestReady_RafaleReplay_ResetsStaleParticipatingTeams as closely as
// possible: launch a manche WITH teams selected, play it (flip a card /
// select a MEMOTION card), STOP it, then Ready() the SAME question ID again
// WITHOUT reselecting anything.
//
// #200 cycle 5 (2026-09-04) added the *_ZeroFlipStartStop_*/
// *_ZeroActionStartStop_* tests below: the ACTUAL confirmed QUALIF
// v8.0.0.18 root cause was a manche STARTED then STOPPED with ZERO gameplay
// actions — not covered by the tests above, which all flip/select at least
// one card. The reset condition itself (Ready(), engine.go) was generalized
// from a gameplay-progress signal to e.questionEverStarted (its own doc
// comment) to close this — see that test's own comment for the full story.
//
// Run: go test ./internal/game/... -run TestReady_MemoryReplay\|TestReady_MemotionReplay -v
package game

import "testing"

// TestReady_MemoryReplay_ResetsStaleParticipatingTeams reproduces the MEMORY
// analog of a7b70057's RAFALE scenario.
func TestReady_MemoryReplay_ResetsStaleParticipatingTeams(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})

	q := &Question{ID: "mq1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeChacunSonTour)}}
	e.Ready("mq1", q)
	if err := e.SetMemoryParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams: %v", err)
	}
	e.ForceReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Fatalf("sanity: expected READY, got %s", state.Phase)
	}
	e.StartImmediate(0)
	if state := e.GetState(); state.Phase != PhaseStarted {
		t.Fatalf("sanity: expected STARTED, got %s", state.Phase)
	}

	// Actually play the manche: flip two DIFFERENT pairs so the match never
	// succeeds — this is enough to increment MemoryErrors deterministically
	// (memoryRoundAlreadyPlayed's signal) without depending on the
	// question's own MemoryPairs content or completion logic.
	e.FlipMemoryCard(cardIDFor(1, 1))
	e.FlipMemoryCard(cardIDFor(2, 1))
	if state := e.GetState(); state.MemoryErrors == 0 {
		t.Fatalf("sanity: expected MemoryErrors > 0 after flipping two mismatched pairs, got 0")
	}

	e.Stop() // normal end of manche
	if state := e.GetState(); state.Phase != PhaseStopped {
		t.Fatalf("sanity: expected STOPPED, got %s", state.Phase)
	}

	// Replay the SAME question (same ID) for a NEW manche, WITHOUT
	// reselecting any team.
	q2 := &Question{ID: "mq1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeChacunSonTour)}}
	e.Ready("mq1", q2)

	state := e.GetState()
	if len(state.MemoryParticipatingTeams) != 0 {
		t.Errorf("expected MEMORY_PARTICIPATING_TEAMS to be reset on replay, got %v", state.MemoryParticipatingTeams)
	}
	if state.MemoryCurrentTeam != "" {
		t.Errorf("expected MEMORY_CURRENT_TEAM to be reset on replay, got %q", state.MemoryCurrentTeam)
	}
	if state.MemoryErrors != 0 {
		t.Errorf("expected MEMORY_ERRORS to be reset on replay, got %d", state.MemoryErrors)
	}
	if len(state.MemoryTeamErrors) != 0 {
		t.Errorf("expected MEMORY_TEAM_ERRORS to be reset on replay, got %v", state.MemoryTeamErrors)
	}

	e.ForceReady()
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected a replayed CHACUN_SON_TOUR question with no reselected team to stay stuck in PREPARE, got %s", state.Phase)
	}

	e.Start(30)
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected Start() to be refused on replay without reselection, got phase=%s", state.Phase)
	}

	// Positive control: reselecting teams on the replay must still work.
	if err := e.SetMemoryParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams on replay: %v", err)
	}
	e.ForceReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Errorf("expected reselecting teams on replay to reach READY, got %s", state.Phase)
	}
}

// TestReady_MemoryReplay_PreservesSelectionBeforeAnyStart is the negative
// control mirroring a7b70057's own "re-Ready() before any démarrage" case:
// Ready() called twice on the SAME not-yet-started question must still let
// the team selection persist across the PREPARE→READY transition — the
// original behavior this isNewQuestion guard exists to protect.
func TestReady_MemoryReplay_PreservesSelectionBeforeAnyStart(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})

	q := &Question{ID: "mq1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}}
	e.Ready("mq1", q)
	if err := e.SetMemoryParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams: %v", err)
	}

	// Re-Ready() the SAME question ID again — no manche was ever started.
	e.Ready("mq1", q)

	state := e.GetState()
	if len(state.MemoryParticipatingTeams) != 1 || state.MemoryParticipatingTeams[0] != "red" {
		t.Errorf("expected the selection to persist across a re-Ready() before any Start(), got %v", state.MemoryParticipatingTeams)
	}
}

// TestReady_MemoryReplay_ResetsStaleParticipatingTeams_AcrossDifferentModes
// mirrors rafale_modes_test.go's own
// TestReady_RafaleReplay_ResetsStaleParticipatingTeams_AcrossDifferentModes:
// the reset condition (memoryRoundAlreadyPlayed, engine.go) reads only game-
// progress fields (MemoryFlippedCards/MemoryMatchedPairs/MemoryErrors) —
// never the outgoing/incoming question's MemoryMode — so this is a belt-and-
// suspenders regression guard against a future change that could accidentally
// reintroduce a mode comparison: an admin edits the SAME round-config
// question (same ID) between manches to switch mode from CHACUN_SON_TOUR to
// TANT_QUE_JE_GAGNE (both multi, same >=2-teams participant rule) — a
// leftover selection from the FIRST mode must not silently satisfy the
// SECOND mode's gate either.
func TestReady_MemoryReplay_ResetsStaleParticipatingTeams_AcrossDifferentModes(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})

	q1 := &Question{ID: "mq1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeChacunSonTour)}}
	e.Ready("mq1", q1)
	if err := e.SetMemoryParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams: %v", err)
	}
	e.ForceReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Fatalf("sanity: expected READY, got %s", state.Phase)
	}
	e.StartImmediate(0)
	if state := e.GetState(); state.Phase != PhaseStarted {
		t.Fatalf("sanity: expected STARTED, got %s", state.Phase)
	}
	e.FlipMemoryCard(cardIDFor(1, 1))
	e.FlipMemoryCard(cardIDFor(2, 1))
	if state := e.GetState(); state.MemoryErrors == 0 {
		t.Fatalf("sanity: expected MemoryErrors > 0 after flipping two mismatched pairs, got 0")
	}
	e.Stop()
	if state := e.GetState(); state.Phase != PhaseStopped {
		t.Fatalf("sanity: expected STOPPED, got %s", state.Phase)
	}

	// Replay the SAME question ID, but the admin switched MEMORY_MODE to a
	// DIFFERENT multi mode in the meantime.
	q2 := &Question{ID: "mq1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeTantQueJeGagne)}}
	e.Ready("mq1", q2)

	state := e.GetState()
	if len(state.MemoryParticipatingTeams) != 0 {
		t.Errorf("expected MEMORY_PARTICIPATING_TEAMS to be reset on replay regardless of mode change (CHACUN_SON_TOUR->TANT_QUE_JE_GAGNE), got %v", state.MemoryParticipatingTeams)
	}

	e.ForceReady()
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected the replayed TANT_QUE_JE_GAGNE question with no reselected team to stay stuck in PREPARE, got %s", state.Phase)
	}
}

// TestReady_MemotionReplay_ResetsStaleParticipatingTeams reproduces the
// MEMOTION analog of a7b70057's RAFALE scenario.
func TestReady_MemotionReplay_ResetsStaleParticipatingTeams(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})

	cards := defaultMotionCards()
	q := makeMotionQuestion("mo1", cards, "SOLO")
	e.Ready("mo1", q)
	if err := e.SetMotionParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMotionParticipatingTeams: %v", err)
	}
	e.ForceReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Fatalf("sanity: expected READY, got %s", state.Phase)
	}
	e.StartImmediate(0)
	e.InitMotionState() // mirrors startMEMOTION() helper — populates MotionCardStates/SubPhase
	if state := e.GetState(); state.Phase != PhaseStarted {
		t.Fatalf("sanity: expected STARTED, got %s", state.Phase)
	}

	// Actually play the manche: select a card (Phase==STARTED-gated action —
	// the motionRoundAlreadyPlayed signal, see engine.go's own comment).
	if err := e.SelectMotionCard(cards[0].ID); err != nil {
		t.Fatalf("SelectMotionCard: %v", err)
	}
	if state := e.GetState(); state.MotionCardStates[cards[0].ID] != MotionCardStateSelected {
		t.Fatalf("sanity: expected card %s to be SELECTED, got %s", cards[0].ID, state.MotionCardStates[cards[0].ID])
	}

	e.Stop() // normal end of manche
	if state := e.GetState(); state.Phase != PhaseStopped {
		t.Fatalf("sanity: expected STOPPED, got %s", state.Phase)
	}

	// Replay the SAME question (same ID) for a NEW manche, WITHOUT
	// reselecting any team.
	q2 := makeMotionQuestion("mo1", cards, "SOLO")
	e.Ready("mo1", q2)

	state := e.GetState()
	if len(state.MotionParticipatingTeams) != 0 {
		t.Errorf("expected MEMOTION_PARTICIPATING_TEAMS to be reset on replay, got %v", state.MotionParticipatingTeams)
	}
	if state.MotionCurrentTeam != "" {
		t.Errorf("expected MEMOTION_CURRENT_TEAM to be reset on replay, got %q", state.MotionCurrentTeam)
	}
	for _, card := range cards {
		if got := state.MotionCardStates[card.ID]; got != MotionCardStateUnplayed {
			t.Errorf("expected card %s to be reset to UNPLAYED on replay, got %s", card.ID, got)
		}
	}

	e.ForceReady()
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected a replayed MEMOTION question with no reselected team to stay stuck in PREPARE, got %s", state.Phase)
	}

	e.Start(30)
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected Start() to be refused on replay without reselection, got phase=%s", state.Phase)
	}

	// Positive control: reselecting a team on the replay must still work.
	if err := e.SetMotionParticipatingTeams([]string{"blue"}); err != nil {
		t.Fatalf("SetMotionParticipatingTeams on replay: %v", err)
	}
	e.ForceReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Errorf("expected reselecting a team on replay to reach READY, got %s", state.Phase)
	}
}

// TestReady_MemotionReplay_PreservesSelectionBeforeAnyStart is the negative
// control mirroring a7b70057's own "re-Ready() before any démarrage" case
// for MEMOTION: Ready()'s own PREPARE-phase grid preview (initMotionStateUnsafe,
// #71/#72) sets every card to UNPLAYED and MotionSubPhase to MEMORIZE/GRID —
// this must NOT be mistaken for "already played" and must not wipe a
// selection made before any Start().
func TestReady_MemotionReplay_PreservesSelectionBeforeAnyStart(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})

	cards := defaultMotionCards()
	q := makeMotionQuestion("mo1", cards, "SOLO")
	e.Ready("mo1", q)
	if err := e.SetMotionParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMotionParticipatingTeams: %v", err)
	}

	// Re-Ready() the SAME question ID again — no manche was ever started.
	e.Ready("mo1", q)

	state := e.GetState()
	if len(state.MotionParticipatingTeams) != 1 || state.MotionParticipatingTeams[0] != "red" {
		t.Errorf("expected the selection to persist across a re-Ready() before any Start(), got %v", state.MotionParticipatingTeams)
	}
}

// TestReady_MemotionReplay_ResetsStaleParticipatingTeams_AcrossDifferentModes
// mirrors TestReady_MemoryReplay_ResetsStaleParticipatingTeams_AcrossDifferentModes
// (itself mirroring rafale_modes_test.go's own AcrossDifferentModes guard):
// motionRoundAlreadyPlayed (engine.go) reads only card-progress state
// (any MotionCardStates entry != UNPLAYED) — never the outgoing/incoming
// question's MotionMode — so the reset must not silently depend on it.
func TestReady_MemotionReplay_ResetsStaleParticipatingTeams_AcrossDifferentModes(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})

	cards := defaultMotionCards()
	q1 := makeMotionQuestion("mo1", cards, "CHACUN_SON_TOUR")
	e.Ready("mo1", q1)
	if err := e.SetMotionParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetMotionParticipatingTeams: %v", err)
	}
	e.ForceReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Fatalf("sanity: expected READY, got %s", state.Phase)
	}
	e.StartImmediate(0)
	e.InitMotionState()
	if state := e.GetState(); state.Phase != PhaseStarted {
		t.Fatalf("sanity: expected STARTED, got %s", state.Phase)
	}
	if err := e.SelectMotionCard(cards[0].ID); err != nil {
		t.Fatalf("SelectMotionCard: %v", err)
	}
	e.Stop()
	if state := e.GetState(); state.Phase != PhaseStopped {
		t.Fatalf("sanity: expected STOPPED, got %s", state.Phase)
	}

	// Replay the SAME question ID, but the admin switched MOTION_MODE to a
	// DIFFERENT mode in the meantime.
	q2 := makeMotionQuestion("mo1", cards, "TANT_QUE_JE_GAGNE")
	e.Ready("mo1", q2)

	state := e.GetState()
	if len(state.MotionParticipatingTeams) != 0 {
		t.Errorf("expected MEMOTION_PARTICIPATING_TEAMS to be reset on replay regardless of mode change (CHACUN_SON_TOUR->TANT_QUE_JE_GAGNE), got %v", state.MotionParticipatingTeams)
	}
	for _, card := range cards {
		if got := state.MotionCardStates[card.ID]; got != MotionCardStateUnplayed {
			t.Errorf("expected card %s to be reset to UNPLAYED on replay, got %s", card.ID, got)
		}
	}

	e.ForceReady()
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected the replayed TANT_QUE_JE_GAGNE question with no reselected team to stay stuck in PREPARE, got %s", state.Phase)
	}
}

// TestReady_MemoryReplay_ZeroFlipStartStop_ResetsStaleParticipatingTeams is
// the #200 cycle 5 regression test: the ACTUAL confirmed root cause of the
// QUALIF v8.0.0.18 report ("je peux toujours faire START alors qu'aucune
// équipe n'est sélectionnée" for MEMORY), found via an empirical end-to-end
// reproduction (cmd/server/memory_start_no_team_repro_200_test.go) after the
// cycle 1-4 fixes failed to reproduce it. The 2026-08-31 first-attempt
// signal (memoryRoundAlreadyPlayed, superseded — see Ready()'s own comment)
// only fired on actual GAMEPLAY progress (a card flip). A manche that
// reaches STARTED and is STOPPED again with ZERO gameplay actions — no card
// ever flipped, e.g. an admin/anim who clicks START then immediately STOP to
// fix a wrong config, an entirely ordinary workflow — was indistinguishable
// from "never started" by that signal, leaving the team selection from that
// first (aborted) manche in place. A same-ID replay then silently satisfied
// participantsConform with a selection the user never made for THIS manche.
// e.questionEverStarted (engine.go) closes this precisely.
func TestReady_MemoryReplay_ZeroFlipStartStop_ResetsStaleParticipatingTeams(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})

	q := &Question{ID: "mq1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeChacunSonTour)}}
	e.Ready("mq1", q)
	if err := e.SetMemoryParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams: %v", err)
	}
	e.ForceReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Fatalf("sanity: expected READY, got %s", state.Phase)
	}
	e.StartImmediate(0)
	if state := e.GetState(); state.Phase != PhaseStarted {
		t.Fatalf("sanity: expected STARTED, got %s", state.Phase)
	}

	// ZERO gameplay actions — no FlipMemoryCard call at all — before
	// stopping. This is the exact scenario the superseded signal missed.
	e.Stop()
	if state := e.GetState(); state.Phase != PhaseStopped {
		t.Fatalf("sanity: expected STOPPED, got %s", state.Phase)
	}
	if state := e.GetState(); len(state.MemoryFlippedCards) != 0 || len(state.MemoryMatchedPairs) != 0 || state.MemoryErrors != 0 {
		t.Fatalf("sanity: expected zero gameplay progress (the whole point of this test), got FlippedCards=%v MatchedPairs=%v Errors=%d",
			state.MemoryFlippedCards, state.MemoryMatchedPairs, state.MemoryErrors)
	}

	// Replay the SAME question ID for a NEW manche, WITHOUT reselecting.
	q2 := &Question{ID: "mq1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeChacunSonTour)}}
	e.Ready("mq1", q2)

	state := e.GetState()
	if len(state.MemoryParticipatingTeams) != 0 {
		t.Errorf("BUG: MEMORY_PARTICIPATING_TEAMS survived a zero-flip Start+Stop+replay: %v", state.MemoryParticipatingTeams)
	}

	e.ForceReady()
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected a replayed CHACUN_SON_TOUR question (zero-flip prior manche) with no reselected team to stay stuck in PREPARE, got %s", state.Phase)
	}
	e.Start(30)
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected Start() to be refused, got phase=%s", state.Phase)
	}
}

// TestReady_MemotionReplay_ZeroActionStartStop_ResetsStaleParticipatingTeams
// mirrors the test above for MEMOTION: STARTED then STOPPED with zero cards
// ever selected.
func TestReady_MemotionReplay_ZeroActionStartStop_ResetsStaleParticipatingTeams(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})

	cards := defaultMotionCards()
	q := makeMotionQuestion("mo1", cards, "SOLO")
	e.Ready("mo1", q)
	if err := e.SetMotionParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMotionParticipatingTeams: %v", err)
	}
	e.ForceReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Fatalf("sanity: expected READY, got %s", state.Phase)
	}
	e.StartImmediate(0)
	e.InitMotionState()
	if state := e.GetState(); state.Phase != PhaseStarted {
		t.Fatalf("sanity: expected STARTED, got %s", state.Phase)
	}

	// ZERO gameplay actions — no SelectMotionCard call at all.
	e.Stop()
	if state := e.GetState(); state.Phase != PhaseStopped {
		t.Fatalf("sanity: expected STOPPED, got %s", state.Phase)
	}
	for _, card := range cards {
		if got := e.GetState().MotionCardStates[card.ID]; got != MotionCardStateUnplayed {
			t.Fatalf("sanity: expected zero gameplay progress (card %s), got %s", card.ID, got)
		}
	}

	q2 := makeMotionQuestion("mo1", cards, "SOLO")
	e.Ready("mo1", q2)

	state := e.GetState()
	if len(state.MotionParticipatingTeams) != 0 {
		t.Errorf("BUG: MEMOTION_PARTICIPATING_TEAMS survived a zero-action Start+Stop+replay: %v", state.MotionParticipatingTeams)
	}

	e.ForceReady()
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected a replayed MEMOTION question (zero-action prior manche) with no reselected team to stay stuck in PREPARE, got %s", state.Phase)
	}
}
