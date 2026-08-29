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
	if got := state.RafaleTeamBest["red"]; got != 2 {
		t.Fatalf("expected red best=2, got %d", got)
	}

	// blue now answers incorrectly → its counter resets to 0, its best
	// (never set) stays 0, and it still rotates away.
	if err := e.RafaleInvalidate(); err != nil { // blue incorrect → counter=0, rotate to red
		t.Fatalf("RafaleInvalidate failed: %v", err)
	}
	state = e.GetState()
	if got := state.RafaleTeamCounters["blue"]; got != 0 {
		t.Errorf("MAILLON_FAIBLE: expected blue counter reset to 0 after an incorrect answer, got %d", got)
	}
	if got := state.RafaleCurrentTeam; got != "red" {
		t.Errorf("expected rotation to 'red' after blue's incorrect answer, got %q", got)
	}

	// red's own earlier counter/best must be untouched by blue's reset.
	if got := state.RafaleTeamCounters["red"]; got != 2 {
		t.Errorf("MAILLON_FAIBLE: red's counter must not be affected by blue's reset, got %d", got)
	}
	if got := state.RafaleTeamBest["red"]; got != 2 {
		t.Errorf("MAILLON_FAIBLE: red's best must survive, got %d", got)
	}
}
