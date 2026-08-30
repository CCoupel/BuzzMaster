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

	e.ForceReady()

	if state := e.GetState(); state.Phase != PhaseReady {
		t.Errorf("RAFALE with a valid CATEGORY must reach READY via ForceReady, got %s", state.Phase)
	}
}

func TestParticipantsConform_Rafale(t *testing.T) {
	state := &GameState{}
	tests := []struct {
		name     string
		category QuestionCategory
		want     bool
	}{
		{"empty category does not conform", CategoryNone, false},
		{"set category conforms", CategoryHistory, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &Question{Type: QuestionTypeRafale, Category: tt.category}
			if got := participantsConform(q, state); got != tt.want {
				t.Errorf("participantsConform(CATEGORY=%q) = %v, want %v", tt.category, got, tt.want)
			}
		})
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
