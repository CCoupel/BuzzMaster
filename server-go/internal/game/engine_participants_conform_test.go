package game

// #172 — dev-backend tests (plan `_work/reports/plan-20260817-122307.md` §7 Bloc D:
// D1, D2, D3, D5, D6, D7). D4, D8, D9 are test-writer's (integration-level rollback,
// full-suite non-regression, manual QA) — see engine_prepare_ready_rollback_test.go.
//
// Fichier additif — n'existait pas avant #172, ne modifie aucun test existant.

import "testing"

// ============================================================
// D2 — participantsConform: the five rules + the permissive default.
// ============================================================

func TestParticipantsConform_Rules(t *testing.T) {
	tests := []struct {
		name     string
		question *Question
		state    *GameState
		want     bool
	}{
		{
			name:     "nil question is permissive",
			question: nil,
			state:    &GameState{},
			want:     true,
		},
		{
			name:     "SPEEDY has no requirement of its own",
			question: &Question{Type: QuestionTypeSpeedy},
			state:    &GameState{},
			want:     true,
		},
		{
			name:     "QCM has no requirement of its own",
			question: &Question{Type: QuestionTypeQCM},
			state:    &GameState{},
			want:     true,
		},
		{
			name:     "ARDOISE has no requirement of its own",
			question: &Question{Type: QuestionTypeArdoise},
			state:    &GameState{},
			want:     true,
		},
		{
			name:     "unknown/future question type is permissive",
			question: &Question{Type: QuestionType("SOMETHING_NEW")},
			state:    &GameState{},
			want:     true,
		},
		{
			name:     "MEMORY SOLO with zero teams selected is not conform",
			question: &Question{Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}},
			state:    &GameState{MemoryParticipatingTeams: []string{}},
			want:     false,
		},
		{
			name:     "MEMORY SOLO with exactly one team selected is conform",
			question: &Question{Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}},
			state:    &GameState{MemoryParticipatingTeams: []string{"red"}},
			want:     true,
		},
		{
			name:     "MEMORY SOLO with two teams selected is not conform",
			question: &Question{Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}},
			state:    &GameState{MemoryParticipatingTeams: []string{"red", "blue"}},
			want:     false,
		},
		{
			name:     "MEMORY with empty MemoryMode defaults to SOLO's rule",
			question: &Question{Type: QuestionTypeMemory},
			state:    &GameState{MemoryParticipatingTeams: []string{"red"}},
			want:     true,
		},
		{
			name:     "MEMORY CHACUN_SON_TOUR with one team is not conform",
			question: &Question{Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeChacunSonTour)}},
			state:    &GameState{MemoryParticipatingTeams: []string{"red"}},
			want:     false,
		},
		{
			name:     "MEMORY CHACUN_SON_TOUR with two teams is conform",
			question: &Question{Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeChacunSonTour)}},
			state:    &GameState{MemoryParticipatingTeams: []string{"red", "blue"}},
			want:     true,
		},
		{
			name:     "MEMORY TANT_QUE_JE_GAGNE with zero teams is not conform",
			question: &Question{Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeTantQueJeGagne)}},
			state:    &GameState{MemoryParticipatingTeams: []string{}},
			want:     false,
		},
		{
			name:     "MEMORY TANT_QUE_JE_GAGNE with three teams is conform",
			question: &Question{Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeTantQueJeGagne)}},
			state:    &GameState{MemoryParticipatingTeams: []string{"red", "blue", "green"}},
			want:     true,
		},
		{
			name:     "MEMOTION with zero teams selected is not conform",
			question: &Question{Type: QuestionTypeMemotion},
			state:    &GameState{MotionParticipatingTeams: []string{}},
			want:     false,
		},
		{
			name:     "MEMOTION with one team selected is conform",
			question: &Question{Type: QuestionTypeMemotion},
			state:    &GameState{MotionParticipatingTeams: []string{"red"}},
			want:     true,
		},
		// #201 (test-writer complement): the two cases above rely on the
		// EMPTY-MotionMode default (""->SOLO, same as MEMORY) to exercise
		// SOLO's count==1 rule — coincidentally correct (dev-backend's own
		// handoff note: "aucune fixture MEMOTION n'a eu besoin de correction
		// par coïncidence de leur construction"), but never pinned down with
		// an EXPLICIT MotionMode value, and MEMOTION's multi-mode direction
		// (count>=2, #201's actual new rule for this type) had NO coverage
		// at all before this. Mirrors MEMORY's own explicit SOLO/multi table
		// entries above line-for-line so participantsCountConform's shared
		// rule is pinned for MEMOTION exactly as thoroughly as for MEMORY,
		// independent of the default-mode coincidence.
		{
			name:     "MEMOTION explicit SOLO with zero teams is not conform",
			question: &Question{Type: QuestionTypeMemotion, MotionMode: string(MemoryModeSolo)},
			state:    &GameState{MotionParticipatingTeams: []string{}},
			want:     false,
		},
		{
			name:     "MEMOTION explicit SOLO with exactly one team is conform",
			question: &Question{Type: QuestionTypeMemotion, MotionMode: string(MemoryModeSolo)},
			state:    &GameState{MotionParticipatingTeams: []string{"red"}},
			want:     true,
		},
		{
			name:     "MEMOTION CHACUN_SON_TOUR with one team is not conform",
			question: &Question{Type: QuestionTypeMemotion, MotionMode: string(MemoryModeChacunSonTour)},
			state:    &GameState{MotionParticipatingTeams: []string{"red"}},
			want:     false,
		},
		{
			name:     "MEMOTION CHACUN_SON_TOUR with two teams is conform",
			question: &Question{Type: QuestionTypeMemotion, MotionMode: string(MemoryModeChacunSonTour)},
			state:    &GameState{MotionParticipatingTeams: []string{"red", "blue"}},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := participantsConform(tt.question, tt.state)
			if got != tt.want {
				t.Errorf("participantsConform() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================
// D1 — PREPARE's exit requires both criteria, independently. AreAllTeamsReady is
// untouched; the two stay separate functions, combined only at the call site
// (main.go's handlePong for the production path — mirrored here at engine level).
// ============================================================

func TestPrepareExit_RequiresBothCriteriaIndependently(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Red"},
		"blue": {Name: "Blue"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})

	q := &Question{ID: "q1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}}
	e.Ready("q1", q)

	// Neither criterion met yet.
	if e.AreAllTeamsReady() {
		t.Fatal("precondition: buzzers should not be ready yet")
	}
	if e.ParticipantsConform() {
		t.Fatal("precondition: MEMORY SOLO with no selection should not be conform")
	}

	// Buzzers ready, but selection still missing: AreAllTeamsReady alone must not
	// be treated as sufficient.
	e.SetBumperReady("b1")
	e.SetBumperReady("b2")
	if !e.AreAllTeamsReady() {
		t.Fatal("precondition: both buzzers answered, should be ready")
	}
	if e.ParticipantsConform() {
		t.Fatal("ParticipantsConform must still be false: no team was selected")
	}
	if e.AreAllTeamsReady() && e.ParticipantsConform() {
		e.TransitionToReady()
	}
	if e.GetPhase() != PhasePrepare {
		t.Fatalf("must stay in PREPARE when only AreAllTeamsReady is true, got %s", e.GetPhase())
	}

	// Selection now conform, but buzzers un-readied again: ParticipantsConform
	// alone must not be treated as sufficient either. AreAllTeamsReady reads
	// team.Ready (computed by updateTeamsReady from bumper.Ready), so the team
	// flag is un-set directly here rather than the bumper flag.
	e.GetTeam("red").Ready = false
	if err := e.SetMemoryParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams should succeed in PREPARE, got %v", err)
	}
	if e.AreAllTeamsReady() {
		t.Fatal("precondition: red team was manually un-readied, should not be all-ready")
	}
	if !e.ParticipantsConform() {
		t.Fatal("precondition: exactly one MEMORY SOLO team is now selected, should be conform")
	}
	if e.AreAllTeamsReady() && e.ParticipantsConform() {
		e.TransitionToReady()
	}
	if e.GetPhase() != PhasePrepare {
		t.Fatalf("must stay in PREPARE when only ParticipantsConform is true, got %s", e.GetPhase())
	}

	// Both criteria met: now the combined gate lets PREPARE exit.
	e.SetBumperReady("b1")
	if !e.AreAllTeamsReady() || !e.ParticipantsConform() {
		t.Fatal("precondition: both criteria should now be true")
	}
	if e.AreAllTeamsReady() && e.ParticipantsConform() {
		e.TransitionToReady()
	}
	if e.GetPhase() != PhaseReady {
		t.Fatalf("expected READY once both criteria are true, got %s", e.GetPhase())
	}
}

// ============================================================
// D3 (test central) — SPEEDY, QCM and ARDOISE reach READY exactly as before: a
// single active team, no participant selection of any kind required. This is the
// regression the plan worried most about (R3) — participantsConform must stay a
// no-op for these three types.
// ============================================================

func TestPrepareExit_SimpleModesUnaffected(t *testing.T) {
	simpleTypes := []QuestionType{QuestionTypeSpeedy, QuestionTypeQCM, QuestionTypeArdoise}

	for _, qType := range simpleTypes {
		t.Run(string(qType), func(t *testing.T) {
			e := NewEngine()
			e.SetTeams(map[string]*Team{"red": {Name: "Red"}})
			e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

			q := &Question{ID: "q1", Type: qType}
			e.Ready("q1", q)

			if e.ParticipantsConform() != true {
				t.Fatalf("%s must always be conform (no selection requirement)", qType)
			}

			if e.AreAllTeamsReady() {
				t.Fatal("precondition: single team should not be ready before its bumper answers")
			}

			e.SetBumperReady("b1")

			if !e.AreAllTeamsReady() {
				t.Fatal("single active team with its one bumper ready should satisfy AreAllTeamsReady")
			}
			if e.AreAllTeamsReady() && e.ParticipantsConform() {
				e.TransitionToReady()
			}
			if e.GetPhase() != PhaseReady {
				t.Errorf("%s with a single active team should reach READY exactly as before #172, got %s", qType, e.GetPhase())
			}
		})
	}
}

// ============================================================
// D5 — Engine.Start refuses any phase other than READY (#172 B4).
// ============================================================

func TestEngineStart_RefusesOutsideReady(t *testing.T) {
	t.Run("from STOPPED (default)", func(t *testing.T) {
		e := NewEngine()
		if e.GetPhase() != PhaseStopped {
			t.Fatalf("precondition: new engine should start STOPPED, got %s", e.GetPhase())
		}
		e.Start(10)
		if e.GetPhase() != PhaseStopped {
			t.Errorf("Start() must be refused from STOPPED, got %s", e.GetPhase())
		}
	})

	t.Run("from PREPARE", func(t *testing.T) {
		e := NewEngine()
		e.Ready("q1", nil)
		if e.GetPhase() != PhasePrepare {
			t.Fatalf("precondition: expected PREPARE, got %s", e.GetPhase())
		}
		e.Start(10)
		if e.GetPhase() != PhasePrepare {
			t.Errorf("Start() must be refused from PREPARE — this is the exact bypass the #172 lock closes, got %s", e.GetPhase())
		}
	})

	t.Run("from COUNTDOWN (Start already pending)", func(t *testing.T) {
		e := NewEngine()
		e.Ready("q1", nil)
		e.TransitionToReady()
		e.Start(10)
		if e.GetPhase() != PhaseCountdown {
			t.Fatalf("precondition: expected COUNTDOWN, got %s", e.GetPhase())
		}
		defer e.Stop()

		e.Start(10) // a second Start() call while already counting down
		if e.GetPhase() != PhaseCountdown {
			t.Errorf("a second Start() call must not disturb an in-progress COUNTDOWN, got %s", e.GetPhase())
		}
	})

	t.Run("from READY (positive control)", func(t *testing.T) {
		e := NewEngine()
		e.Ready("q1", nil)
		e.TransitionToReady()
		if e.GetPhase() != PhaseReady {
			t.Fatalf("precondition: expected READY, got %s", e.GetPhase())
		}
		defer e.Stop()

		e.Start(10)
		if e.GetPhase() != PhaseCountdown {
			t.Errorf("Start() must be accepted from READY, got %s", e.GetPhase())
		}
	})
}

// ============================================================
// D6 — ForceReady (arbitrage G): skips the PONG wait, never participant-selection
// conformity.
// ============================================================

func TestForceReady_SkipsPongNotConformity(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Red"},
		"blue": {Name: "Blue"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})

	q := &Question{ID: "q1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}}
	e.Ready("q1", q)

	// No PONG received from either bumper, and no participant selected yet.
	e.ForceReady()

	if e.GetPhase() != PhasePrepare {
		t.Fatalf("ForceReady must not reach READY when participants do not conform, got %s", e.GetPhase())
	}
	// But it must still have skipped the PONG wait.
	if !e.GetBumper("b1").Ready || !e.GetBumper("b2").Ready {
		t.Error("ForceReady must still mark bumpers ready (it only skips the PONG wait, arbitrage G)")
	}
	if !e.AreAllTeamsReady() {
		t.Error("ForceReady must still satisfy AreAllTeamsReady")
	}

	// Selecting a conform participant now must flip straight to READY — "sans
	// geste supplémentaire" (no new SetBumperReady/PONG needed): the bumpers are
	// already marked ready from ForceReady above.
	if err := e.SetMemoryParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams should succeed in PREPARE, got %v", err)
	}
	if e.GetPhase() != PhaseReady {
		t.Errorf("expected READY once ForceReady's skipped PONG meets a conform selection, got %s", e.GetPhase())
	}
}

// TestForceReady_SimpleMode_PositiveControl verifies ForceReady's original
// behavior is unchanged for question types with no selection requirement.
func TestForceReady_SimpleMode_PositiveControl(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Red"}})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	q := &Question{ID: "q1", Type: QuestionTypeSpeedy}
	e.Ready("q1", q)

	e.ForceReady()

	if e.GetPhase() != PhaseReady {
		t.Errorf("ForceReady on a SPEEDY question should still reach READY directly, got %s", e.GetPhase())
	}
}

// ============================================================
// D7 — original bug: no more MEMORY auto-selection (non-deterministic map
// iteration), and MEMOTION can no longer start without a participant.
// ============================================================

// TestStart_MemorySolo_UnreachableWithoutExplicitSelection is the structural
// regression test for the origin bug: MEMORY SOLO can no longer reach STARTED (or
// even READY) without an explicit SetMemoryParticipatingTeams call, regardless of
// how many other teams are active and answering PONG. This makes the old
// auto-pick-from-active-teams behavior (engine.go's former actualStart block,
// removed by #172 A1) structurally impossible to reintroduce unnoticed.
func TestStart_MemorySolo_UnreachableWithoutExplicitSelection(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":   {Name: "Red"},
		"blue":  {Name: "Blue"},
		"green": {Name: "Green"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})
	e.UpdateBumper("b3", map[string]interface{}{"TEAM": "green"})

	q := &Question{ID: "q1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}}
	e.Ready("q1", q)
	e.SetBumperReady("b1")
	e.SetBumperReady("b2")
	e.SetBumperReady("b3")

	if !e.AreAllTeamsReady() {
		t.Fatal("precondition: all three buzzers answered, should be ready")
	}
	if e.ParticipantsConform() {
		t.Fatal("MEMORY SOLO with no explicit selection must never be conform, however many teams are active")
	}
	if e.AreAllTeamsReady() && e.ParticipantsConform() {
		e.TransitionToReady()
	}
	if e.GetPhase() != PhasePrepare {
		t.Fatalf("must stay in PREPARE without an explicit selection, got %s", e.GetPhase())
	}

	e.Start(10)
	if e.GetPhase() != PhasePrepare {
		t.Errorf("Start() must refuse to leave PREPARE — a MEMORY SOLO round can never reach STARTED without an explicit team selection, got %s", e.GetPhase())
	}
}

// TestActualStart_PreservesExplicitMemorySelection exercises actualStart()
// directly (bypassing the countdown) to confirm the removed auto-init block
// (#172 A1) is gone for good: an explicit single-team SOLO selection survives
// actualStart() unchanged, even though other teams are active on the buzzer
// layer. Before A1, the old block would only have fired when the selection was
// empty — this test instead locks in that actualStart() never touches
// MemoryParticipatingTeams/MemoryCurrentTeam at all anymore.
func TestActualStart_PreservesExplicitMemorySelection(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Red"},
		"blue": {Name: "Blue"},
	})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})

	q := &Question{ID: "q1", Type: QuestionTypeMemory, TypedContent: TypedContent{MemoryMode: string(MemoryModeSolo)}}
	e.Ready("q1", q)
	e.SetBumperReady("b1")
	e.SetBumperReady("b2")

	if err := e.SetMemoryParticipatingTeams([]string{"blue"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams should succeed in PREPARE, got %v", err)
	}
	if e.GetPhase() != PhaseReady {
		t.Fatalf("precondition: expected READY, got %s", e.GetPhase())
	}

	e.actualStart()

	state := e.GetState()
	if len(state.MemoryParticipatingTeams) != 1 || state.MemoryParticipatingTeams[0] != "blue" {
		t.Errorf("actualStart must not overwrite the explicit selection with all active teams (old auto-init bug), got %v", state.MemoryParticipatingTeams)
	}
	if state.MemoryCurrentTeam != "blue" {
		t.Errorf("expected MemoryCurrentTeam=blue (deterministic, from the explicit selection), got %q", state.MemoryCurrentTeam)
	}
}

// TestStart_Memotion_UnreachableWithoutExplicitSelection mirrors the MEMORY case
// for MEMOTION: no participant selected means PREPARE never exits.
func TestStart_Memotion_UnreachableWithoutExplicitSelection(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Red"}})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})

	q := &Question{ID: "q1", Type: QuestionTypeMemotion}
	e.Ready("q1", q)
	e.SetBumperReady("b1")

	if !e.AreAllTeamsReady() {
		t.Fatal("precondition: single active team, buzzer answered, should be ready")
	}
	if e.ParticipantsConform() {
		t.Fatal("MEMOTION with no explicit selection must never be conform")
	}

	e.Start(10)
	if e.GetPhase() != PhasePrepare {
		t.Errorf("Start() must refuse to leave PREPARE — MEMOTION must never start without a participant, got %s", e.GetPhase())
	}
}

// TestMotionReady_MultiModeOneTeam_NeverReachesReady (#201 test-writer
// complement) mirrors TestRafaleReady_MultiModeOneTeam_NeverReachesReady
// (rafale_modes_test.go) for MEMOTION: the participantsCountConform("multi
// mode" -> count>=2) direction had ZERO end-to-end coverage for MEMOTION
// before this (only the pure participantsConform table above, added
// alongside this test, ever exercised it) — CHACUN_SON_TOUR with a single
// selected team must never let ForceReady()/Start() leave PREPARE. Positive
// control included (2 teams -> READY -> START accepted), same pattern as
// every other #201 regression test in this repo.
func TestMotionReady_MultiModeOneTeam_NeverReachesReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
	e.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})

	cards := defaultMotionCards()
	q := makeMotionQuestion("mo1", cards, "CHACUN_SON_TOUR")
	e.Ready("mo1", q)
	if err := e.SetMotionParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMotionParticipatingTeams: %v", err)
	}
	e.SetBumperReady("b1")
	e.SetBumperReady("b2")

	if e.ParticipantsConform() {
		t.Fatal("MEMOTION CHACUN_SON_TOUR with a single team selected must not be conform (needs >=2)")
	}
	e.ForceReady()
	if e.GetPhase() != PhasePrepare {
		t.Errorf("ForceReady() must refuse to leave PREPARE with only 1 team in a MEMOTION multi mode, got %s", e.GetPhase())
	}
	e.Start(10)
	if e.GetPhase() == PhaseCountdown || e.GetPhase() == PhaseStarted {
		t.Errorf("Start() must refuse a MEMOTION multi-mode round with only 1 team selected, got %s", e.GetPhase())
	}

	// Positive control: 2 teams reaches READY and START is accepted.
	if err := e.SetMotionParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetMotionParticipatingTeams (2 teams): %v", err)
	}
	e.ForceReady()
	if e.GetPhase() != PhaseReady {
		t.Fatalf("expected READY with 2 teams selected in CHACUN_SON_TOUR, got %s", e.GetPhase())
	}
	e.Start(10)
	if e.GetPhase() != PhaseCountdown && e.GetPhase() != PhaseStarted {
		t.Errorf("expected Start() to be accepted with 2 teams selected, got %s", e.GetPhase())
	}
}
