package game

import (
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests : mode RAFALE — les 4 modes (SOLO/CHACUN_SON_TOUR/TANT_QUE_JE_GAGNE/
// MAILLON_FAIBLE), compteurs par équipe + meilleur score, RAFALE_SET_TEAMS
// (milestone v8.0.0 #16, issue #199, tâches 29-31, contrat
// contracts/rafale.md §3.4/§6.1).
//
// Complète rafale_test.go (Batch 1, réservoir/pioche/pool) et
// engine_memotion_test.go's TestDoneMotionCard_Rotation_* (le patron de
// style suivi ici : SetTeams → Ready → SetXxxParticipatingTeams →
// StartImmediate → agir → vérifier via GetState()).
// ---------------------------------------------------------------------------

// makeRafaleQuestion builds a RAFALE round-configuration Question — mirrors
// makeMotionQuestion's role in engine_memotion_test.go. questionTime<=0
// defaults to 3 inside the engine (advanceRafaleUnsafe/startRafaleRoundUnsafe),
// so 0 here exercises that default path deliberately.
func makeRafaleQuestion(id string, mode string, category QuestionCategory, difficulty int) *Question {
	return &Question{
		ID:       id,
		Question: "RAFALE round " + id,
		Type:     QuestionTypeRafale,
		Category: category,
		Points:   "10",
		Time:     "120",
		TypedContent: TypedContent{
			RafaleDifficulty:   difficulty,
			RafaleMode:         mode,
			RafaleQuestionTime: 3,
			RafaleMaxQuestions: 100,
		},
	}
}

// seedRafaleReservoirBulk seeds n reservoir questions r-1..r-n, all matching
// the given category/difficulty — enough headroom so a test exercising
// several advances never hits ErrRafalePoolEmpty and confuses "round ended
// because the mode logic is wrong" with "round ended because the pool ran
// out".
func seedRafaleReservoirBulk(t *testing.T, e *Engine, n int, category QuestionCategory, difficulty int) {
	t.Helper()
	questions := make([]RafaleQuestion, 0, n)
	for i := 1; i <= n; i++ {
		questions = append(questions, RafaleQuestion{
			ID: fmt.Sprintf("r-%d", i), Question: "Q", Answer: "A",
			Category: category, Difficulty: difficulty,
		})
	}
	seedRafaleReservoir(t, e, questions)
}

// ---------------------------------------------------------------------------
// SetRafaleParticipatingTeams (tâche 31)
// ---------------------------------------------------------------------------

func TestSetRafaleParticipatingTeams_FirstTeamBecomesCurrent(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red", Color: []int{255, 0, 0}},
		"blue": {Name: "Team Blue", Color: []int{0, 0, 255}},
	})
	seedRafaleReservoirBulk(t, e, 5, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeChacunSonTour), CategoryHistory, 1)
	e.Ready("rq1", q)

	if err := e.SetRafaleParticipatingTeams([]string{"blue", "red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams failed: %v", err)
	}

	state := e.GetState()
	if state.RafaleCurrentTeam != "blue" {
		t.Errorf("expected first team 'blue' to become current, got %q", state.RafaleCurrentTeam)
	}
	if len(state.RafaleCurrentTeamColor) != 3 || state.RafaleCurrentTeamColor[2] != 255 {
		t.Errorf("expected RafaleCurrentTeamColor to be blue's color, got %v", state.RafaleCurrentTeamColor)
	}
	if len(state.RafaleParticipatingTeams) != 2 {
		t.Errorf("expected 2 participating teams, got %v", state.RafaleParticipatingTeams)
	}
}

func TestSetRafaleParticipatingTeams_RejectsUnknownTeam(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	seedRafaleReservoirBulk(t, e, 5, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q)

	err := e.SetRafaleParticipatingTeams([]string{"red", "ghost"})
	if err == nil {
		t.Fatal("expected an error for an unknown team, got nil")
	}
	if _, ok := err.(*RafaleError); !ok {
		t.Errorf("expected *RafaleError, got %T (%v)", err, err)
	}
}

func TestSetRafaleParticipatingTeams_RejectsWrongPhase(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	seedRafaleReservoirBulk(t, e, 5, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q)
	e.StartImmediate(0) // now STARTED, not PREPARE/READY
	defer e.Stop()

	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err == nil {
		t.Error("expected an error when called outside PREPARE/READY, got nil")
	}
}

func TestSetRafaleParticipatingTeams_RejectsNonRafaleQuestion(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "")
	e.Ready("mq1", q)

	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err == nil {
		t.Error("expected an error for a non-RAFALE question, got nil")
	}
}

// ---------------------------------------------------------------------------
// Garde READY sur CATEGORY (bugfix 2026-08-30, participantsConform) — QUALIF
// root cause : une manche RAFALE sans CATEGORY (question de config
// résiduelle d'avant la migration catégorie-unique du 2026-08-29, ou jamais
// configurée) atteignait quand même STARTED puis mourait immédiatement
// (drawRafaleQuestionUnsafe : pool vide sur category=="" → roundEnded →
// Stop() dans le même tick que actualStart()) — countdown 3s visible, puis
// plus rien, sans qu'aucun signal clair n'indique pourquoi. Ces tests
// prouvent que la manche reste désormais bloquée en PREPARE (jamais READY,
// donc Start() la refuse structurellement — engine.go:1144) au lieu
// d'atteindre STARTED puis de mourir silencieusement.
// ---------------------------------------------------------------------------

func TestRafaleReady_EmptyCategory_NeverReachesReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	seedRafaleReservoirBulk(t, e, 5, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryNone, 1)
	e.Ready("rq1", q)

	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Fatalf("sanity: expected PhasePrepare right after Ready(), got %s", state.Phase)
	}

	e.ForceReady()

	state := e.GetState()
	if state.Phase != PhasePrepare {
		t.Errorf("RAFALE with an empty CATEGORY must stay stuck in PREPARE (never reach READY), got %s", state.Phase)
	}

	// Start() itself refuses any phase other than READY (engine.go:1144) —
	// belt-and-braces confirmation that the mis-configured round can never
	// actually begin the countdown at all, not just that ForceReady no-ops.
	e.Start(30)
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("Start() must be refused while stuck in PREPARE, got phase=%s", state.Phase)
	}
}

func TestRafaleReady_ValidCategory_ReachesReadyNormally(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	seedRafaleReservoirBulk(t, e, 5, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q)

	// #201: SOLO now requires exactly one selected team — select it before
	// ForceReady(), same as MEMORY SOLO already required.
	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}

	e.ForceReady()

	if state := e.GetState(); state.Phase != PhaseReady {
		t.Errorf("RAFALE with a valid CATEGORY and its SOLO team selected must reach READY via ForceReady, got %s", state.Phase)
	}
}

// TestParticipantsConform_Rafale covers both static round-config guards on
// the same function/gate: CATEGORY (bugfix 2026-08-30, cycle 1 of the "3s
// countdown then nothing" QUALIF symptom) and RAFALE_DIFFICULTY (bugfix
// 2026-08-31, cycle 2 of the SAME external symptom — a genuinely different
// root cause: handleUploadQuestion never persisted RAFALE_DIFFICULTY at all,
// so a round-config question saved via the normal editor always came back
// with RafaleDifficulty == 0, which is not a valid difficulty (1-3), and the
// round died in the same tick it started for the exact same underlying
// reason as cycle 1 — drawRafaleQuestionUnsafe's pool-empty safety net firing
// inside actualStart()).
func TestParticipantsConform_Rafale(t *testing.T) {
	tests := []struct {
		name       string
		category   QuestionCategory
		difficulty int
		mode       string
		teams      []string
		want       bool
	}{
		{"empty category does not conform", CategoryNone, 2, string(RafaleModeSolo), nil, false},
		{"zero difficulty does not conform (the 2026-08-31 bug)", CategoryHistory, 0, string(RafaleModeSolo), nil, false},
		{"difficulty below range does not conform", CategoryHistory, -1, string(RafaleModeSolo), nil, false},
		{"difficulty above range does not conform", CategoryHistory, 4, string(RafaleModeSolo), nil, false},
		// #201: SOLO now requires exactly one team (was exempt) — these three
		// isolate the CATEGORY/DIFFICULTY dimension specifically, so a team
		// is selected to keep that isolation (team count is NOT what's under
		// test in these three rows).
		{"set category and valid difficulty=1 conforms", CategoryHistory, 1, string(RafaleModeSolo), []string{"red"}, true},
		{"set category and valid difficulty=2 conforms", CategoryHistory, 2, string(RafaleModeSolo), []string{"red"}, true},
		{"set category and valid difficulty=3 conforms", CategoryHistory, 3, string(RafaleModeSolo), []string{"red"}, true},
		{"empty category AND zero difficulty does not conform", CategoryNone, 0, string(RafaleModeSolo), nil, false},
		// --- 2026-09-02, #199: multi-team mode requires a minimum of selected
		// teams; #201, 2026-09-01/2026-09-02: threshold corrected to >=2 (was
		// >=1) for multi modes, AND SOLO no longer exempt — both now share
		// participantsCountConform (SOLO==1, multi>=2), same rule as MEMORY. ---
		{"SOLO with no teams does not conform (#201 — was exempt before the fix)", CategoryHistory, 1, string(RafaleModeSolo), nil, false},
		{"SOLO with one team conforms (#201)", CategoryHistory, 1, string(RafaleModeSolo), []string{"red"}, true},
		{"SOLO with two teams does not conform (#201 — exactly one required)", CategoryHistory, 1, string(RafaleModeSolo), []string{"red", "blue"}, false},
		{"empty RAFALE_MODE defaults to SOLO, no teams does not conform (#201)", CategoryHistory, 1, "", nil, false},
		{"empty RAFALE_MODE defaults to SOLO, one team conforms (#201)", CategoryHistory, 1, "", []string{"red"}, true},
		{"CHACUN_SON_TOUR with no teams does not conform", CategoryHistory, 1, string(RafaleModeChacunSonTour), nil, false},
		{"CHACUN_SON_TOUR with one team does not conform (#201 — was wrongly true before the >=2 fix)", CategoryHistory, 1, string(RafaleModeChacunSonTour), []string{"red"}, false},
		{"CHACUN_SON_TOUR with two teams conforms", CategoryHistory, 1, string(RafaleModeChacunSonTour), []string{"red", "blue"}, true},
		{"TANT_QUE_JE_GAGNE with no teams does not conform", CategoryHistory, 1, string(RafaleModeTantQueJeGagne), nil, false},
		{"TANT_QUE_JE_GAGNE with one team does not conform (#201)", CategoryHistory, 1, string(RafaleModeTantQueJeGagne), []string{"red"}, false},
		{"MAILLON_FAIBLE with no teams does not conform", CategoryHistory, 1, string(RafaleModeMaillonFaible), nil, false},
		{"MAILLON_FAIBLE with one team does not conform (#201)", CategoryHistory, 1, string(RafaleModeMaillonFaible), []string{"red"}, false},
		{"MAILLON_FAIBLE with two teams conforms", CategoryHistory, 1, string(RafaleModeMaillonFaible), []string{"red", "blue"}, true},
		{"multi mode with no teams AND zero difficulty does not conform (both reasons)", CategoryHistory, 0, string(RafaleModeChacunSonTour), nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &GameState{RafaleParticipatingTeams: tt.teams}
			q := &Question{Type: QuestionTypeRafale, Category: tt.category, TypedContent: TypedContent{RafaleDifficulty: tt.difficulty, RafaleMode: tt.mode}}
			if got := participantsConform(q, state); got != tt.want {
				t.Errorf("participantsConform(CATEGORY=%q, RAFALE_DIFFICULTY=%d, RAFALE_MODE=%q, TEAMS=%v) = %v, want %v",
					tt.category, tt.difficulty, tt.mode, tt.teams, got, tt.want)
			}
		})
	}
}

// TestRafaleReady_ZeroDifficulty_NeverReachesReady mirrors
// TestRafaleReady_EmptyCategory_NeverReachesReady above but for the
// RAFALE_DIFFICULTY dimension of the same gate (bugfix 2026-08-31) — proves
// the fix at the Ready()/ForceReady() level, not just at the raw
// participantsConform() function level covered above.
func TestRafaleReady_ZeroDifficulty_NeverReachesReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	seedRafaleReservoirBulk(t, e, 5, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 0) // difficulty=0, as if handleUploadQuestion had dropped it
	e.Ready("rq1", q)

	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Fatalf("sanity: expected PhasePrepare right after Ready(), got %s", state.Phase)
	}

	e.ForceReady()

	state := e.GetState()
	if state.Phase != PhasePrepare {
		t.Errorf("RAFALE with RAFALE_DIFFICULTY=0 must stay stuck in PREPARE (never reach READY), got %s", state.Phase)
	}

	e.Start(30)
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("Start() must be refused while stuck in PREPARE, got phase=%s", state.Phase)
	}
}

// TestRafaleReady_MultiModeOneTeam_NeverReachesReady is the dedicated #201
// regression test requested by the CDP: a multi-team mode
// (CHACUN_SON_TOUR here, but the rule is identical for TANT_QUE_JE_GAGNE/
// MAILLON_FAIBLE — see TestParticipantsConform_Rafale's own table) with
// only ONE team selected must stay stuck in PREPARE and refuse Start() —
// exactly like TestRafaleReady_ZeroDifficulty_NeverReachesReady/
// TestRafaleReady_EmptyCategory_NeverReachesReady prove the CATEGORY/
// RAFALE_DIFFICULTY dimensions of the same gate at the Ready()/Start()
// level, not just the raw participantsConform() table above. Before #201
// (>=1 threshold), this exact scenario silently reached READY and START —
// confirmed by temporarily reverting the fix locally and observing this
// test fail, exactly as the CDP's dispatch described.
func TestRafaleReady_MultiModeOneTeam_NeverReachesReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}, "blue": {Name: "Team Blue"}})
	seedRafaleReservoirBulk(t, e, 5, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeChacunSonTour), CategoryHistory, 1)
	e.Ready("rq1", q)

	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	if e.ParticipantsConform() {
		t.Fatalf("precondition: exactly one team in CHACUN_SON_TOUR must not conform (#201 — needs >=2)")
	}

	e.ForceReady()
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("BUG: CHACUN_SON_TOUR with only ONE team selected reached %s, want stuck in PREPARE (#201)", state.Phase)
	}

	e.Start(30)
	if state := e.GetState(); state.Phase == PhaseCountdown || state.Phase == PhaseStarted {
		t.Errorf("BUG: Start() succeeded (phase=%s) for CHACUN_SON_TOUR with only ONE team selected — #201 requires >=2", state.Phase)
	}

	// Positive control: adding the second team must let the round reach
	// READY and START normally.
	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams (2 teams): %v", err)
	}
	if !e.ParticipantsConform() {
		t.Fatalf("expected two teams in CHACUN_SON_TOUR to conform")
	}
	e.ForceReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Fatalf("expected READY with two teams selected, got %s", state.Phase)
	}
	e.Start(30)
	if state := e.GetState(); state.Phase != PhaseCountdown {
		t.Errorf("expected Start() to succeed with two teams selected, got phase=%s", state.Phase)
	}
	e.Stop()
}

// ---------------------------------------------------------------------------
// Garde READY sur RAFALE_PARTICIPATING_TEAMS en mode multi (feature
// 2026-09-02, #199) — retour QUALIF 8.0.0.13 : "je ne dois pas pouvoir faire
// START si aucune équipe n'est sélectionnée". Contrairement aux gardes
// CATEGORY/RAFALE_DIFFICULTY ci-dessus (bugfixes contre une mort silencieuse
// en STARTED), celle-ci répond à une demande fonctionnelle directe — mais
// verrouille le même mécanisme (blocage propre en PREPARE, Start() refusé
// structurellement).
// ---------------------------------------------------------------------------

func TestRafaleReady_MultiModeNoTeams_NeverReachesReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}, "blue": {Name: "Team Blue"}})
	seedRafaleReservoirBulk(t, e, 5, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeChacunSonTour), CategoryHistory, 1)
	e.Ready("rq1", q)

	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Fatalf("sanity: expected PhasePrepare right after Ready(), got %s", state.Phase)
	}

	e.ForceReady() // no RAFALE_SET_TEAMS called — RAFALE_PARTICIPATING_TEAMS stays empty

	state := e.GetState()
	if state.Phase != PhasePrepare {
		t.Errorf("CHACUN_SON_TOUR with no participating team must stay stuck in PREPARE (never reach READY), got %s", state.Phase)
	}

	e.Start(30)
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("Start() must be refused while stuck in PREPARE, got phase=%s", state.Phase)
	}
}

func TestRafaleReady_MultiModeWithTeams_ReachesReadyNormally(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}, "blue": {Name: "Team Blue"}})
	seedRafaleReservoirBulk(t, e, 5, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeChacunSonTour), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams failed: %v", err)
	}

	e.ForceReady()

	if state := e.GetState(); state.Phase != PhaseReady {
		t.Errorf("CHACUN_SON_TOUR with participating teams must reach READY, got %s", state.Phase)
	}
}

// TestRafaleReady_SoloNoTeams_NeverReachesReady is the #201 update of this
// test — SOLO used to be exempt from any participant-selection requirement
// (this test's own name/assertion, inverted here). The user confirmed
// RAFALE SOLO must require exactly one team, same as MEMORY SOLO
// (participantsCountConform, engine.go) — a SOLO round with ZERO teams ever
// selected must now stay stuck in PREPARE and refuse Start(), mirroring
// TestStart_MemorySolo_UnreachableWithoutExplicitSelection
// (engine_participants_conform_test.go).
func TestRafaleReady_SoloNoTeams_NeverReachesReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	seedRafaleReservoirBulk(t, e, 5, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q) // no RAFALE_SET_TEAMS

	if e.ParticipantsConform() {
		t.Fatal("precondition: no team was ever selected, SOLO must not conform (#201)")
	}

	e.ForceReady()
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("BUG: SOLO reached %s with ZERO teams ever selected — #201 requires exactly one", state.Phase)
	}

	e.Start(30)
	if state := e.GetState(); state.Phase == PhaseCountdown || state.Phase == PhaseStarted {
		t.Errorf("BUG: Start() succeeded (phase=%s) for a SOLO round with no team selected — #201", state.Phase)
	}
}

// TestRafaleReady_SoloOneTeam_ReachesReady is the #201 positive-control
// regression test explicitly requested by the CDP: RAFALE SOLO with exactly
// ONE team selected must reach READY and accept Start() normally — proving
// the fix above (TestRafaleReady_SoloNoTeams_NeverReachesReady) narrows the
// gate, it doesn't turn SOLO into an unreachable dead end.
func TestRafaleReady_SoloOneTeam_ReachesReady(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	seedRafaleReservoirBulk(t, e, 5, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q)

	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	if !e.ParticipantsConform() {
		t.Fatal("precondition: exactly one team selected in SOLO must conform (#201)")
	}

	e.ForceReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Fatalf("expected READY with one team selected in SOLO, got %s", state.Phase)
	}

	e.Start(30)
	if state := e.GetState(); state.Phase != PhaseCountdown {
		t.Errorf("expected Start() to succeed with one team selected in SOLO, got phase=%s", state.Phase)
	}
	e.Stop()
}

// TestSetRafaleParticipatingTeams_ClearingInMultiMode_RevertsReadyToPrepare
// proves the gate is wired all the way through the SAME live-editing path
// MEMORY/MEMOTION already use (SetRafaleParticipatingTeams already called
// reevaluatePrepareReadyUnsafe before this gate existed, in anticipation —
// see its own comment): clearing the team selection while READY in a multi
// mode must immediately revert to PREPARE, and reselecting at least one team
// must immediately re-promote to READY — no new PONG cycle needed either
// time. Uses real bumpers + TransitionToReady() (not the ForceReady debug
// shortcut, which bypasses areAllTeamsReadyUnsafe entirely) — same pattern
// as engine_prepare_ready_rollback_test.go's setupReadyMemory/setupReadyMotion,
// since reevaluatePrepareReadyUnsafe's PREPARE->READY branch requires BOTH
// areAllTeamsReadyUnsafe() (bumper-based, unaffected by
// RAFALE_PARTICIPATING_TEAMS) AND participantsConform().
func TestSetRafaleParticipatingTeams_ClearingInMultiMode_RevertsReadyToPrepare(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}, "blue": {Name: "Team Blue"}})
	e.UpdateBumper("bumper-red", map[string]interface{}{"TEAM": "red"})
	e.UpdateBumper("bumper-blue", map[string]interface{}{"TEAM": "blue"})
	seedRafaleReservoirBulk(t, e, 5, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeChacunSonTour), CategoryHistory, 1)
	e.Ready("rq1", q)
	e.SetBumperReady("bumper-red")
	e.SetBumperReady("bumper-blue")
	if !e.AreAllTeamsReady() {
		t.Fatal("setup: all active teams should be ready after every bumper answered")
	}
	// #201: a multi mode needs >=2 selected teams to conform — select BOTH
	// available teams to reach READY in the first place.
	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams failed: %v", err)
	}
	e.TransitionToReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Fatalf("sanity: expected PhaseReady, got %s", state.Phase)
	}

	if err := e.SetRafaleParticipatingTeams([]string{}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams (clear) failed: %v", err)
	}
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected clearing teams in a multi mode to revert READY -> PREPARE, got %s", state.Phase)
	}

	// #201: a single team is no longer enough to re-promote — must stay in
	// PREPARE until the >=2 threshold is met again.
	if err := e.SetRafaleParticipatingTeams([]string{"blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams (reselect one) failed: %v", err)
	}
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected a single reselected team (#201: needs >=2 in multi mode) to stay in PREPARE, got %s", state.Phase)
	}

	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams (reselect both) failed: %v", err)
	}
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Errorf("expected reselecting both teams to re-promote PREPARE -> READY, got %s", state.Phase)
	}
}

// ---------------------------------------------------------------------------
// Les 4 modes (tâches 29-30) — contrat §6.1.
// ---------------------------------------------------------------------------

func TestRafaleAdvance_Solo_CounterIncrementsOnCorrectOnly_NeverRotates(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	seedRafaleReservoirBulk(t, e, 10, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams failed: %v", err)
	}
	e.StartImmediate(0)
	defer e.Stop()

	if err := e.RafaleValidate(); err != nil {
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	if err := e.RafaleInvalidate(); err != nil {
		t.Fatalf("RafaleInvalidate failed: %v", err)
	}
	if err := e.RafaleValidate(); err != nil {
		t.Fatalf("RafaleValidate failed: %v", err)
	}

	state := e.GetState()
	if state.RafaleCurrentTeam != "red" {
		t.Errorf("SOLO must never rotate — expected current team to stay 'red', got %q", state.RafaleCurrentTeam)
	}
	if got := state.RafaleTeamCounters["red"]; got != 2 {
		t.Errorf("expected counter=2 (2 correct answers, 1 incorrect ignored), got %d", got)
	}
}

// TestRafaleAdvance_Solo_StreakErrorsBest closes the coverage gap the
// bugfix (2026-08-30) left open: RafaleTeamStreak/RafaleTeamErrors/
// RafaleTeamBest were only asserted for MAILLON_FAIBLE (already tracked
// RafaleTeamBest before the redefinition) and CHACUN_SON_TOUR (the new
// streak-vs-counter distinguishing test) — SOLO had none. advanceRafaleUnsafe
// computes streak/errors/best identically for all 4 modes (mode-agnostic
// block, engine.go) BEFORE the per-mode counter switch, so this is a
// genuinely different code path than SOLO's own counter test above, not a
// copy — SOLO's counter is the ONLY mode whose policy fully ignores
// incorrect answers (§6.1: "—" for every SOLO row but the first), yet
// streak/errors must still react to them exactly like every other mode.
func TestRafaleAdvance_Solo_StreakErrorsBest(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	seedRafaleReservoirBulk(t, e, 10, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams failed: %v", err)
	}
	e.StartImmediate(0)
	defer e.Stop()

	if err := e.RafaleValidate(); err != nil { // streak=1, best=1, counter=1
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	if err := e.RafaleValidate(); err != nil { // streak=2, best=2, counter=2
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	state := e.GetState()
	if got := state.RafaleTeamStreak["red"]; got != 2 {
		t.Fatalf("expected red streak=2 after 2 correct answers, got %d", got)
	}
	if got := state.RafaleTeamBest["red"]; got != 2 {
		t.Fatalf("expected red best=2, got %d", got)
	}

	if err := e.RafaleInvalidate(); err != nil { // streak resets to 0, errors=1 — counter UNAFFECTED (SOLO ignores incorrect)
		t.Fatalf("RafaleInvalidate failed: %v", err)
	}
	state = e.GetState()
	if got := state.RafaleTeamStreak["red"]; got != 0 {
		t.Errorf("SOLO: streak must reset to 0 after an incorrect answer (mode-agnostic rule), got %d", got)
	}
	if got := state.RafaleTeamErrors["red"]; got != 1 {
		t.Errorf("SOLO: errors must still increment on an incorrect answer, got %d", got)
	}
	if got := state.RafaleTeamCounters["red"]; got != 2 {
		t.Errorf("SOLO: counter must stay UNCHANGED by an incorrect answer (contract §6.1, SOLO ignores it), got %d", got)
	}
	if got := state.RafaleTeamBest["red"]; got != 2 {
		t.Errorf("SOLO: best must survive the streak reset (historical max), got %d", got)
	}

	if err := e.RafaleValidate(); err != nil { // streak=1 again, counter=3, best stays 2
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	state = e.GetState()
	if got := state.RafaleTeamStreak["red"]; got != 1 {
		t.Errorf("expected red streak=1 (restarted after the reset), got %d", got)
	}
	if got := state.RafaleTeamCounters["red"]; got != 3 {
		t.Errorf("expected red counter=3 (3rd correct answer), got %d", got)
	}
	if got := state.RafaleTeamBest["red"]; got != 2 {
		t.Errorf("expected red best to stay at its historical max 2 (current streak 1 is lower), got %d", got)
	}
}

func TestRafaleAdvance_ChacunSonTour_RotatesRegardlessOfOutcome(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red", Color: []int{255, 0, 0}},
		"blue": {Name: "Team Blue", Color: []int{0, 0, 255}},
	})
	seedRafaleReservoirBulk(t, e, 10, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeChacunSonTour), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams failed: %v", err)
	}
	e.StartImmediate(0)
	defer e.Stop()

	// red correct → rotates to blue
	if err := e.RafaleValidate(); err != nil {
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	if got := e.GetState().RafaleCurrentTeam; got != "blue" {
		t.Fatalf("after 1st advance (correct), expected 'blue', got %q", got)
	}

	// blue incorrect → rotates back to red (wrap-around)
	if err := e.RafaleInvalidate(); err != nil {
		t.Fatalf("RafaleInvalidate failed: %v", err)
	}
	state := e.GetState()
	if state.RafaleCurrentTeam != "red" {
		t.Errorf("CHACUN_SON_TOUR: expected circular wrap back to 'red', got %q", state.RafaleCurrentTeam)
	}
	if got := state.RafaleTeamCounters["red"]; got != 1 {
		t.Errorf("expected red counter=1 (its one correct answer), got %d", got)
	}
	if got := state.RafaleTeamCounters["blue"]; got != 0 {
		t.Errorf("expected blue counter=0 (its answer was incorrect), got %d", got)
	}
}

func TestRafaleAdvance_TantQueJeGagne_KeepsHandOnCorrect_RotatesOnIncorrect(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red"},
		"blue": {Name: "Team Blue"},
	})
	seedRafaleReservoirBulk(t, e, 10, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeTantQueJeGagne), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams failed: %v", err)
	}
	e.StartImmediate(0)
	defer e.Stop()

	// red wins twice in a row → keeps the hand both times
	if err := e.RafaleValidate(); err != nil {
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	if err := e.RafaleValidate(); err != nil {
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	if got := e.GetState().RafaleCurrentTeam; got != "red" {
		t.Fatalf("TANT_QUE_JE_GAGNE: winner must keep the hand, expected 'red', got %q", got)
	}
	if got := e.GetState().RafaleTeamCounters["red"]; got != 2 {
		t.Errorf("expected red counter=2, got %d", got)
	}

	// red loses → rotates to blue
	if err := e.RafaleInvalidate(); err != nil {
		t.Fatalf("RafaleInvalidate failed: %v", err)
	}
	if got := e.GetState().RafaleCurrentTeam; got != "blue" {
		t.Errorf("TANT_QUE_JE_GAGNE: loser must rotate away, expected 'blue', got %q", got)
	}
}

// TestRafaleAdvance_TantQueJeGagne_StreakErrorsBest closes the same
// coverage gap as TestRafaleAdvance_Solo_StreakErrorsBest, for the one
// other mode the bugfix's own test additions (2026-08-30) left untouched.
// TANT_QUE_JE_GAGNE's counter policy (increments on correct, UNCHANGED —
// never reset — on incorrect, contract §6.1) makes it the mode where
// streak/counter diverge the MOST visibly: the losing team's counter
// survives an incorrect answer intact while its streak is wiped, on the
// very same event that also rotates the hand away.
func TestRafaleAdvance_TantQueJeGagne_StreakErrorsBest(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red"},
		"blue": {Name: "Team Blue"},
	})
	seedRafaleReservoirBulk(t, e, 10, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeTantQueJeGagne), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams failed: %v", err)
	}
	e.StartImmediate(0)
	defer e.Stop()

	if err := e.RafaleValidate(); err != nil { // red keeps the hand: streak=1, best=1, counter=1
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	if err := e.RafaleValidate(); err != nil { // red keeps the hand: streak=2, best=2, counter=2
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	state := e.GetState()
	if got := state.RafaleTeamStreak["red"]; got != 2 {
		t.Fatalf("expected red streak=2, got %d", got)
	}
	if got := state.RafaleTeamBest["red"]; got != 2 {
		t.Fatalf("expected red best=2, got %d", got)
	}

	if err := e.RafaleInvalidate(); err != nil { // red loses the hand -> rotates to blue
		t.Fatalf("RafaleInvalidate failed: %v", err)
	}
	state = e.GetState()
	if got := state.RafaleCurrentTeam; got != "blue" {
		t.Fatalf("sanity: expected rotation to 'blue', got %q", got)
	}
	if got := state.RafaleTeamStreak["red"]; got != 0 {
		t.Errorf("red's streak must reset to 0 on its incorrect answer (mode-agnostic rule), got %d", got)
	}
	if got := state.RafaleTeamErrors["red"]; got != 1 {
		t.Errorf("red's errors must increment, got %d", got)
	}
	if got := state.RafaleTeamCounters["red"]; got != 2 {
		t.Errorf("TANT_QUE_JE_GAGNE: red's counter must stay UNCHANGED by its incorrect answer (contract §6.1 — only SOLO's rotation stops, the counter itself is never reset by this mode), got %d", got)
	}
	if got := state.RafaleTeamBest["red"]; got != 2 {
		t.Errorf("red's best must survive (historical max), got %d", got)
	}

	if err := e.RafaleValidate(); err != nil { // blue keeps the hand: streak=1, best=1, counter=1
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	state = e.GetState()
	if got := state.RafaleTeamStreak["blue"]; got != 1 {
		t.Errorf("expected blue streak=1 (its first correct answer), got %d", got)
	}
	if got := state.RafaleTeamBest["blue"]; got != 1 {
		t.Errorf("expected blue best=1, got %d", got)
	}
	if got := state.RafaleTeamCounters["blue"]; got != 1 {
		t.Errorf("expected blue counter=1, got %d", got)
	}
}

func TestRafaleAdvance_MaillonFaible_ResetsCounterOnIncorrect_TracksBest(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red"},
		"blue": {Name: "Team Blue"},
	})
	seedRafaleReservoirBulk(t, e, 10, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeMaillonFaible), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams failed: %v", err)
	}
	e.StartImmediate(0)
	defer e.Stop()

	// red correct twice (rotates to blue and back to red each time it's
	// its turn again) — counter climbs to 2, best tracks 2.
	if err := e.RafaleValidate(); err != nil { // red correct (1) → rotate to blue
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	if err := e.RafaleValidate(); err != nil { // blue correct (1) → rotate to red
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	if err := e.RafaleValidate(); err != nil { // red correct (2) → rotate to blue
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	state := e.GetState()
	if got := state.RafaleTeamCounters["red"]; got != 2 {
		t.Fatalf("expected red counter=2 after 2 correct answers, got %d", got)
	}
	// RafaleTeamBest is now RafaleTeamStreak's historical maximum (bugfix,
	// 2026-08-30, generic across all 4 modes) — in this scenario red never
	// answers incorrectly, so its streak equals its counter (2) and best
	// tracks that same value, same numbers as before the redefinition.
	if got := state.RafaleTeamStreak["red"]; got != 2 {
		t.Fatalf("expected red streak=2 (2 correct answers in a row, never reset), got %d", got)
	}
	if got := state.RafaleTeamBest["red"]; got != 2 {
		t.Fatalf("expected red best=2 (max streak reached), got %d", got)
	}
	if got := state.RafaleTeamErrors["red"]; got != 0 {
		t.Fatalf("expected red errors=0 (never answered incorrectly), got %d", got)
	}

	// blue now answers incorrectly → its COUNTER resets to 0 (MAILLON_FAIBLE's
	// own rule), its STREAK also resets to 0 (generic rule, all modes), its
	// ERRORS tally bumps to 1, its best (never set) stays 0, and it still
	// rotates away.
	if err := e.RafaleInvalidate(); err != nil { // blue incorrect → counter=0, rotate to red
		t.Fatalf("RafaleInvalidate failed: %v", err)
	}
	state = e.GetState()
	if got := state.RafaleTeamCounters["blue"]; got != 0 {
		t.Errorf("MAILLON_FAIBLE: expected blue counter reset to 0 after an incorrect answer, got %d", got)
	}
	if got := state.RafaleTeamStreak["blue"]; got != 0 {
		t.Errorf("expected blue streak reset to 0 after an incorrect answer, got %d", got)
	}
	if got := state.RafaleTeamErrors["blue"]; got != 1 {
		t.Errorf("expected blue errors=1 after its first incorrect answer, got %d", got)
	}
	// blue DID answer correctly once (its 2nd turn, above) before this
	// incorrect answer — best keeps that historical streak of 1 even though
	// blue's CURRENT streak just reset to 0.
	if got := state.RafaleTeamBest["blue"]; got != 1 {
		t.Errorf("expected blue best=1 (its one earlier correct answer's streak), got %d", got)
	}
	if got := state.RafaleCurrentTeam; got != "red" {
		t.Errorf("expected rotation to 'red' after blue's incorrect answer, got %q", got)
	}

	// red's own earlier counter/streak/best must be untouched by blue's reset.
	if got := state.RafaleTeamCounters["red"]; got != 2 {
		t.Errorf("MAILLON_FAIBLE: red's counter must not be affected by blue's reset, got %d", got)
	}
	if got := state.RafaleTeamStreak["red"]; got != 2 {
		t.Errorf("red's streak must not be affected by blue's reset, got %d", got)
	}
	if got := state.RafaleTeamBest["red"]; got != 2 {
		t.Errorf("MAILLON_FAIBLE: red's best must survive, got %d", got)
	}
}

// TestRafaleAdvance_Streak_ResetsOnIncorrect_EvenWhenCounterKeepsAccumulating
// is the key distinguishing case the bugfix (2026-08-30) introduces:
// CHACUN_SON_TOUR's RafaleTeamCounters is cumulative (never resets, contract
// §6.1) even after an incorrect answer, but the NEW RafaleTeamStreak field
// resets to 0 regardless of mode — the two are genuinely different fields
// with different semantics, not a renamed duplicate.
func TestRafaleAdvance_Streak_ResetsOnIncorrect_EvenWhenCounterKeepsAccumulating(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{
		"red":  {Name: "Team Red"},
		"blue": {Name: "Team Blue"},
	})
	seedRafaleReservoirBulk(t, e, 10, CategoryHistory, 1)
	q := makeRafaleQuestion("rq1", string(RafaleModeChacunSonTour), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams failed: %v", err)
	}
	e.StartImmediate(0)
	defer e.Stop()

	// red correct (streak=1, counter=1) → rotate to blue
	if err := e.RafaleValidate(); err != nil {
		t.Fatalf("RafaleValidate failed: %v", err)
	}
	// blue incorrect (streak stays 0, errors=1) → rotate to red
	if err := e.RafaleInvalidate(); err != nil {
		t.Fatalf("RafaleInvalidate failed: %v", err)
	}
	// red incorrect this time (streak resets 0->0, counter STAYS 1 — CHACUN_SON_TOUR
	// never resets the counter, unlike MAILLON_FAIBLE) → rotate to blue
	if err := e.RafaleInvalidate(); err != nil {
		t.Fatalf("RafaleInvalidate failed: %v", err)
	}

	state := e.GetState()
	if got := state.RafaleTeamCounters["red"]; got != 1 {
		t.Errorf("CHACUN_SON_TOUR: counter must stay cumulative (1) even after an incorrect answer, got %d", got)
	}
	if got := state.RafaleTeamStreak["red"]; got != 0 {
		t.Errorf("streak must reset to 0 after an incorrect answer, regardless of mode, got %d", got)
	}
	if got := state.RafaleTeamErrors["red"]; got != 1 {
		t.Errorf("expected red errors=1 after its one incorrect answer, got %d", got)
	}
	if got := state.RafaleTeamBest["red"]; got != 1 {
		t.Errorf("expected red best=1 (its one streak of 1, before the reset), got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Régression — retour QUALIF 8.0.0.14, 3e cycle (#199) : "je peux encore
// lancer un START alors que je n'ai pas défini d'équipe" — persistait même
// après le fix de RAFALE_MODE périmé (SHA 6b995229, invalidé par
// l'utilisateur : rouvert ET re-sauvegardé la question, symptôme
// inchangé). Cause réelle : Ready() ne réinitialise RafaleParticipatingTeams
// (et RafaleCurrentTeam/RafaleCurrentTeamColor) QUE quand
// isNewQuestion==true (ID différent) — mais une question de configuration
// RAFALE est CONÇUE pour être rejouée pour plusieurs manches dans la même
// partie avec le MÊME ID (contrairement à QCM/MEMORY/MEMOTION, jouées une
// fois chacune normalement). Stop() ne touche jamais ces champs non plus
// (par conception — la sélection doit survivre à un countdown, pas à une
// manche entièrement terminée). Conséquence : la sélection d'équipes de la
// manche PRÉCÉDENTE restait en mémoire côté serveur, satisfaisant
// silencieusement participantsConform à la manche SUIVANTE, sans que
// l'utilisateur n'ait rien resélectionné — alors même que l'interface
// affichait "aucune équipe sélectionnée" (état local frontend, déconnecté
// du GameState réel).
// ---------------------------------------------------------------------------

// TestReady_RafaleReplay_ResetsStaleParticipatingTeams reproduces the exact
// realistic scenario the CDP asked for: launch a multi-mode manche WITH
// teams selected, let it run to a normal STOP, then Ready() the SAME
// question ID again for a new manche WITHOUT reselecting anything — the
// server must genuinely block START, not silently reuse the previous
// manche's selection.
func TestReady_RafaleReplay_ResetsStaleParticipatingTeams(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
	seedRafaleReservoirBulk(t, e, 20, CategoryHistory, 1)

	q := makeRafaleQuestion("rq1", string(RafaleModeChacunSonTour), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	e.ForceReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Fatalf("sanity: expected READY, got %s", state.Phase)
	}
	e.StartImmediate(0)
	if state := e.GetState(); state.Phase != PhaseStarted {
		t.Fatalf("sanity: expected STARTED, got %s", state.Phase)
	}
	e.Stop() // normal end of manche — e.g. admin stops it, or it runs to completion
	if state := e.GetState(); state.Phase != PhaseStopped {
		t.Fatalf("sanity: expected STOPPED, got %s", state.Phase)
	}

	// Replay the SAME round-config question (same ID) for a NEW manche,
	// WITHOUT reselecting any team — the realistic "another round of the
	// same RAFALE category" flow.
	q2 := makeRafaleQuestion("rq1", string(RafaleModeChacunSonTour), CategoryHistory, 1)
	e.Ready("rq1", q2)

	state := e.GetState()
	if len(state.RafaleParticipatingTeams) != 0 {
		t.Errorf("expected RAFALE_PARTICIPATING_TEAMS to be reset on replay, got %v", state.RafaleParticipatingTeams)
	}
	if state.RafaleCurrentTeam != "" {
		t.Errorf("expected RAFALE_CURRENT_TEAM to be reset on replay, got %q", state.RafaleCurrentTeam)
	}

	e.ForceReady()
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected a replayed CHACUN_SON_TOUR question with no reselected team to stay stuck in PREPARE, got %s", state.Phase)
	}

	e.Start(30)
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected Start() to be refused on replay without reselection, got phase=%s", state.Phase)
	}

	// Positive control: reselecting teams on the replay must still work
	// normally. ForceReady() again rather than relying on
	// SetRafaleParticipatingTeams's own reevaluatePrepareReadyUnsafe
	// side effect — that path additionally requires areAllTeamsReadyUnsafe()
	// (real bumpers marked ready), irrelevant to what this control is
	// checking (participantsConform specifically). #201: CHACUN_SON_TOUR
	// needs >=2 teams — a single team ("blue" alone) would no longer
	// satisfy the gate.
	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams on replay: %v", err)
	}
	e.ForceReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Errorf("expected reselecting both teams on replay to reach READY, got %s", state.Phase)
	}
}

// TestReady_RafaleSameQuestion_StillConfiguring_PreservesSelection is the
// control case proving the fix above is narrowly targeted: reselecting the
// SAME question ID while STILL configuring it (no round ever started yet,
// RafaleSubPhase never left RafaleSubPhaseNone) must keep behaving exactly
// as before — team selection persists across a same-ID Ready() re-call, the
// original intent of isNewQuestion's "persist during PREPARE->READY
// transition" comment.
func TestReady_RafaleSameQuestion_StillConfiguring_PreservesSelection(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
	seedRafaleReservoirBulk(t, e, 20, CategoryHistory, 1)

	q := makeRafaleQuestion("rq1", string(RafaleModeChacunSonTour), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}

	// Re-Ready() the SAME question, SAME ID, before ever starting a round —
	// e.g. the admin re-clicks the question in the list, or the UI re-syncs.
	e.Ready("rq1", q)

	state := e.GetState()
	if len(state.RafaleParticipatingTeams) != 2 {
		t.Errorf("expected the team selection to survive a same-ID Ready() re-call before any round started, got %v", state.RafaleParticipatingTeams)
	}
}

// TestReady_RafaleReplay_ResetsStaleParticipatingTeams_AcrossDifferentModes
// proves the reset introduced by SHA a7b70057 is independent of RAFALE_MODE.
// The fix's own condition (rafaleRoundAlreadyPlayed, engine.go) reads only
// e.state.RafaleSubPhase — never the outgoing/incoming question's mode — so
// this is a belt-and-suspenders regression guard against a future change
// that could accidentally reintroduce a mode comparison: an admin edits the
// SAME round-config question (same ID) between manches to switch mode from
// CHACUN_SON_TOUR (2 teams needed) to MAILLON_FAIBLE (also multi, same
// participant rule) — a leftover multi-team selection from the FIRST mode
// must not silently satisfy the SECOND mode's gate either.
func TestReady_RafaleReplay_ResetsStaleParticipatingTeams_AcrossDifferentModes(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
	seedRafaleReservoirBulk(t, e, 20, CategoryHistory, 1)

	q1 := makeRafaleQuestion("rq1", string(RafaleModeChacunSonTour), CategoryHistory, 1)
	e.Ready("rq1", q1)
	if err := e.SetRafaleParticipatingTeams([]string{"red", "blue"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	e.ForceReady()
	if state := e.GetState(); state.Phase != PhaseReady {
		t.Fatalf("sanity: expected READY, got %s", state.Phase)
	}
	e.StartImmediate(0)
	e.Stop()
	if state := e.GetState(); state.Phase != PhaseStopped {
		t.Fatalf("sanity: expected STOPPED, got %s", state.Phase)
	}

	// Replay the SAME question ID, but the admin switched RAFALE_MODE to a
	// DIFFERENT multi mode in the meantime (e.g. re-edited the round-config
	// question in RafalePage before the next manche).
	q2 := makeRafaleQuestion("rq1", string(RafaleModeMaillonFaible), CategoryHistory, 1)
	e.Ready("rq1", q2)

	state := e.GetState()
	if len(state.RafaleParticipatingTeams) != 0 {
		t.Errorf("expected RAFALE_PARTICIPATING_TEAMS to be reset on replay regardless of mode change (CHACUN_SON_TOUR->MAILLON_FAIBLE), got %v", state.RafaleParticipatingTeams)
	}

	e.ForceReady()
	if state := e.GetState(); state.Phase != PhasePrepare {
		t.Errorf("expected the replayed MAILLON_FAIBLE question with no reselected team to stay stuck in PREPARE, got %s", state.Phase)
	}
}
