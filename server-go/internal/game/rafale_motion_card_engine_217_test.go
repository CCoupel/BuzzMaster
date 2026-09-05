// Tests for #217 — the concrete engine entry points for a RAFALE mini-round
// played as a MEMOTION card: StartRafaleMotionCardRound, RafaleValidateCard/
// RafaleInvalidateCard, and DoneMotionCard's server-derived STARS_PRORATA
// scoring for a RAFALE card. Written by dev-backend alongside the
// implementation — the exact entry point rafale_motion_card_217_test.go
// (test-writer) deliberately left undetermined is exercised directly here.
//
// Uses rafaleMotionCard (rafale_motion_card_217_test.go, same package) and
// startMEMOTION/makeMotionQuestion (engine_memotion_test.go) as the shared
// test vocabulary — no duplicate helpers.

package game

import (
	"errors"
	"testing"
)

// startMEMOTIONAtRafaleCardQuestion prepares an engine with a single RAFALE
// card selected, flipped to QUESTION subphase, AND its mini-round actually
// started — mirrors startMEMOTIONAtMemoryCardQuestion, plus the one extra
// step a RAFALE card needs (cmd/server/main.go's handleMotionFlip does the
// same, in production, right after the generic FlipMotionCard()).
func startMEMOTIONAtRafaleCardQuestion(t *testing.T, e *Engine, card MotionCard) (questionID, answer string, questionTime int) {
	t.Helper()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	q := makeMotionQuestion("mq-rafale-card", []MotionCard{card}, "SOLO")
	startMEMOTION(t, e, "mq-rafale-card", q)
	if err := e.SelectMotionCard(card.ID); err != nil {
		t.Fatalf("SelectMotionCard(%s): %v", card.ID, err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard(): %v", err)
	}
	id, ans, qt, err := e.StartRafaleMotionCardRound(card.ID)
	if err != nil {
		t.Fatalf("StartRafaleMotionCardRound(%s): %v", card.ID, err)
	}
	return id, ans, qt
}

// ---------------------------------------------------------------------------
// StartRafaleMotionCardRound — draws the first question into
// MEMOTION_ACTIVE.STATE, never the global GameState.RAFALE_* fields.
// ---------------------------------------------------------------------------

func TestStartRafaleMotionCardRound_PopulatesMotionActiveState_NeverGlobalFields(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h", 5, CategoryHistory, 1)

	card := rafaleMotionCard("mc-r1", []string{string(CategoryHistory)}, []int{1}, 3, 10)
	questionID, answer, questionTime := startMEMOTIONAtRafaleCardQuestion(t, e, card)
	defer e.Stop()

	if questionID == "" || answer == "" {
		t.Fatalf("StartRafaleMotionCardRound returned empty questionID=%q answer=%q", questionID, answer)
	}
	if questionTime != 3 {
		t.Errorf("questionTime = %d, want 3 (card.RAFALE_QUESTION_TIME)", questionTime)
	}

	state := e.GetState()
	if state.MotionActive.CardID != card.ID || state.MotionActive.Type != QuestionTypeRafale {
		t.Fatalf("MotionActive = %+v, want CardID=%s TYPE=RAFALE", state.MotionActive, card.ID)
	}
	sub, _ := state.MotionActive.State["RAFALE_SUBPHASE"].(string)
	if sub != string(RafaleSubPhaseQuestion) {
		t.Errorf("MEMOTION_ACTIVE.STATE.RAFALE_SUBPHASE = %q, want %q", sub, RafaleSubPhaseQuestion)
	}
	cur, ok := state.MotionActive.State["RAFALE_CURRENT_QUESTION"].(RafaleCurrent)
	if !ok || cur.ID != questionID || cur.Category != string(CategoryHistory) || cur.Difficulty != 1 {
		t.Errorf("MEMOTION_ACTIVE.STATE.RAFALE_CURRENT_QUESTION = %+v (ok=%v), want ID=%s CATEGORY=HISTORY DIFFICULTY=1", cur, ok, questionID)
	}
	if asked, _ := state.MotionActive.State["RAFALE_ASKED_COUNT"].(int); asked != 1 {
		t.Errorf("RAFALE_ASKED_COUNT = %v, want 1", state.MotionActive.State["RAFALE_ASKED_COUNT"])
	}
	if correct, _ := state.MotionActive.State["RAFALE_CORRECT_COUNT"].(int); correct != 0 {
		t.Errorf("RAFALE_CORRECT_COUNT = %v, want 0", state.MotionActive.State["RAFALE_CORRECT_COUNT"])
	}

	// The 13 global RAFALE_* GameState fields must stay at rest — contract
	// §14.2, already covered structurally by test-writer's
	// TestRafaleMotionCard_NeverLeaksIntoGlobalGameStateFields, re-asserted
	// here on the fields this specific call path actually touches.
	if state.RafaleAskedCount != 0 || state.RafaleCurrentQuestion != (RafaleCurrent{}) {
		t.Errorf("global RAFALE_ASKED_COUNT/RAFALE_CURRENT_QUESTION touched by a card round: askedCount=%d currentQuestion=%+v", state.RafaleAskedCount, state.RafaleCurrentQuestion)
	}
}

func TestStartRafaleMotionCardRound_EmptyPool_ReturnsErrRafalePoolEmpty(t *testing.T) {
	e := NewEngine()
	// No reservoir seeded at all.
	card := rafaleMotionCard("mc-empty", []string{string(CategoryHistory)}, []int{1}, 3, 10)
	q := makeMotionQuestion("mq-rafale-empty", []MotionCard{card}, "SOLO")
	startMEMOTION(t, e, "mq-rafale-empty", q)
	if err := e.SelectMotionCard(card.ID); err != nil {
		t.Fatalf("SelectMotionCard: %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard: %v", err)
	}

	_, _, _, err := e.StartRafaleMotionCardRound(card.ID)
	if !errors.Is(err, ErrRafalePoolEmpty) {
		t.Errorf("StartRafaleMotionCardRound with an empty pool: err = %v, want ErrRafalePoolEmpty", err)
	}

	state := e.GetState()
	sub, _ := state.MotionActive.State["RAFALE_SUBPHASE"].(string)
	if sub != "" {
		t.Errorf("RAFALE_SUBPHASE = %q after a failed start, want \"\" (round never started, contract §14.3 safety net)", sub)
	}
	if exhausted, _ := state.MotionActive.State["RAFALE_EXHAUSTED"].(bool); !exhausted {
		t.Error("RAFALE_EXHAUSTED = false after an empty-pool start, want true")
	}
}

// ---------------------------------------------------------------------------
// RafaleValidateCard / RafaleInvalidateCard — advance the mini-round.
// ---------------------------------------------------------------------------

func TestRafaleValidateCard_IncrementsCorrectCountAndAskedCount_DrawsNext(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h", 10, CategoryHistory, 1)
	card := rafaleMotionCard("mc-validate", []string{string(CategoryHistory)}, []int{1}, 3, 10)
	firstID, _, _ := startMEMOTIONAtRafaleCardQuestion(t, e, card)
	defer e.Stop()

	if err := e.RafaleValidateCard(card.ID); err != nil {
		t.Fatalf("RafaleValidateCard: %v", err)
	}

	state := e.GetState()
	if asked, _ := state.MotionActive.State["RAFALE_ASKED_COUNT"].(int); asked != 2 {
		t.Errorf("RAFALE_ASKED_COUNT after 1 validate = %v, want 2", state.MotionActive.State["RAFALE_ASKED_COUNT"])
	}
	if correct, _ := state.MotionActive.State["RAFALE_CORRECT_COUNT"].(int); correct != 1 {
		t.Errorf("RAFALE_CORRECT_COUNT after 1 validate = %v, want 1", state.MotionActive.State["RAFALE_CORRECT_COUNT"])
	}
	cur, _ := state.MotionActive.State["RAFALE_CURRENT_QUESTION"].(RafaleCurrent)
	if cur.ID == firstID {
		t.Errorf("RAFALE_CURRENT_QUESTION.ID unchanged after validate (%s) — expected a new question drawn", cur.ID)
	}
}

func TestRafaleInvalidateCard_AsksNext_NeverIncrementsCorrectCount(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h", 10, CategoryHistory, 1)
	card := rafaleMotionCard("mc-invalidate", []string{string(CategoryHistory)}, []int{1}, 3, 10)
	startMEMOTIONAtRafaleCardQuestion(t, e, card)
	defer e.Stop()

	if err := e.RafaleInvalidateCard(card.ID); err != nil {
		t.Fatalf("RafaleInvalidateCard: %v", err)
	}

	state := e.GetState()
	if asked, _ := state.MotionActive.State["RAFALE_ASKED_COUNT"].(int); asked != 2 {
		t.Errorf("RAFALE_ASKED_COUNT after 1 invalidate = %v, want 2 (still asked, just not correct)", state.MotionActive.State["RAFALE_ASKED_COUNT"])
	}
	if correct, _ := state.MotionActive.State["RAFALE_CORRECT_COUNT"].(int); correct != 0 {
		t.Errorf("RAFALE_CORRECT_COUNT after 1 invalidate = %v, want 0", state.MotionActive.State["RAFALE_CORRECT_COUNT"])
	}
}

func TestRafaleValidateCard_WrongCardID_ReturnsErrRafaleCardNotFound(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h", 5, CategoryHistory, 1)
	card := rafaleMotionCard("mc-real", []string{string(CategoryHistory)}, []int{1}, 3, 10)
	startMEMOTIONAtRafaleCardQuestion(t, e, card)
	defer e.Stop()

	if err := e.RafaleValidateCard("mc-does-not-exist"); !errors.Is(err, ErrRafaleCardNotFound) {
		t.Errorf("RafaleValidateCard(wrong cardID) = %v, want ErrRafaleCardNotFound", err)
	}
	// The real card's own state must be untouched by a mismatched call.
	state := e.GetState()
	if asked, _ := state.MotionActive.State["RAFALE_ASKED_COUNT"].(int); asked != 1 {
		t.Errorf("RAFALE_ASKED_COUNT = %v after a mismatched-card call, want unchanged (1)", state.MotionActive.State["RAFALE_ASKED_COUNT"])
	}
}

// ---------------------------------------------------------------------------
// Bounds — durée propre (generic card timer, not exercised here — see
// contracts/rafale.md §14.3, cmd/server integration) AND nombre de
// questions (this engine's own advanceRafaleCardUnsafe check).
// ---------------------------------------------------------------------------

func TestRafaleCard_MaxQuestionsReached_EndsMiniRoundWithoutStoppingTheGame(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h", 10, CategoryHistory, 1)
	card := rafaleMotionCard("mc-maxq", []string{string(CategoryHistory)}, []int{1}, 3, 2) // max 2 questions
	startMEMOTIONAtRafaleCardQuestion(t, e, card)
	defer e.Stop()

	// The RAFALE_MAX_QUESTIONS guard compares the CURRENT asked count
	// (before it would be incremented by a new draw) against the bound —
	// advanceRafaleCardUnsafe. Start draws Q1 (asked=1); validate #1 checks
	// asked(1)>=2 (false), so it still draws Q2 (asked=2); validate #2
	// checks asked(2)>=2 (true) and ends the mini-round without drawing a
	// third question. Two validates are needed to actually hit the bound.
	if err := e.RafaleValidateCard(card.ID); err != nil {
		t.Fatalf("RafaleValidateCard #1: %v", err)
	}
	if err := e.RafaleValidateCard(card.ID); err != nil {
		t.Fatalf("RafaleValidateCard #2: %v", err)
	}

	state := e.GetState()
	sub, _ := state.MotionActive.State["RAFALE_SUBPHASE"].(string)
	if sub != string(RafaleSubPhaseRoundEnd) {
		t.Errorf("RAFALE_SUBPHASE after reaching RAFALE_MAX_QUESTIONS=2 = %q, want %q", sub, RafaleSubPhaseRoundEnd)
	}
	// Ending a card's mini-round must NEVER stop the whole game/question —
	// contract §14.3, the key divergence from the classic round's stopUnsafe().
	if state.Phase != PhaseStarted {
		t.Errorf("GameState.Phase = %q after a card mini-round ended, want STARTED (unchanged — only the card's own sub-state ends)", state.Phase)
	}
	if state.MotionSubPhase != MotionSubPhaseQuestion {
		t.Errorf("MotionSubPhase = %q, want QUESTION (unchanged — asymmetric exit, same as MEMORY's own timer expiry)", state.MotionSubPhase)
	}

	// Further validate/invalidate calls must be refused now.
	if err := e.RafaleValidateCard(card.ID); err == nil {
		t.Error("RafaleValidateCard succeeded after RAFALE_SUBPHASE reached ROUND_END — must be refused")
	}
}

func TestRafaleCard_PoolExhaustedMidRound_EndsGracefully(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h", 1, CategoryHistory, 1) // exactly 1 question
	card := rafaleMotionCard("mc-exhaust", []string{string(CategoryHistory)}, []int{1}, 3, 100)
	startMEMOTIONAtRafaleCardQuestion(t, e, card)
	defer e.Stop()

	if err := e.RafaleValidateCard(card.ID); err != nil {
		t.Fatalf("RafaleValidateCard: %v", err)
	}

	state := e.GetState()
	sub, _ := state.MotionActive.State["RAFALE_SUBPHASE"].(string)
	if sub != string(RafaleSubPhaseRoundEnd) {
		t.Errorf("RAFALE_SUBPHASE after pool exhaustion = %q, want %q", sub, RafaleSubPhaseRoundEnd)
	}
	if exhausted, _ := state.MotionActive.State["RAFALE_EXHAUSTED"].(bool); !exhausted {
		t.Error("RAFALE_EXHAUSTED = false after the sole question was consumed, want true")
	}
	if state.Phase != PhaseStarted {
		t.Errorf("GameState.Phase = %q after pool exhaustion mid-card, want STARTED (unchanged)", state.Phase)
	}
}

// ---------------------------------------------------------------------------
// DoneMotionCard — STARS_PRORATA, server-derived Units/UnitsTotal, contract
// §14.4/§9.3 (never the client-supplied MEMOTION_DONE.UNITS).
// ---------------------------------------------------------------------------

func TestDoneMotionCard_RafaleCard_AwardsStarsProrata_IgnoringClientUnits(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h", 10, CategoryHistory, 1)
	// Difficulty 3 -> 5 points (motionDifficultyPoints), default STARS scale.
	// RAFALE_MAX_QUESTIONS=3, but the round is closed manually below before
	// that bound is ever reached — see the asked-count trace below.
	card := rafaleMotionCard("mc-score", []string{string(CategoryHistory)}, []int{1}, 3, 3)
	card.Difficulty = 3
	startMEMOTIONAtRafaleCardQuestion(t, e, card)
	defer e.Stop()

	// RAFALE_ASKED_COUNT is incremented every time a NEW question is drawn
	// (including the initial draw and the one that follows a validate/
	// invalidate, whether or not that new question ever gets answered) —
	// contract §14.4/advanceRafaleCardUnsafe. Trace: start draws Q1
	// (asked=1); validate #1 scores Q1 correct (correct=1) then draws Q2
	// (asked=2); validate #2 scores Q2 correct (correct=2) then draws Q3
	// (asked=3). The animateur then closes the card manually (below) while
	// Q3 sits unanswered — same "closed mid-round" shape as
	// TestDoneMotionCard_RafaleCard_NeverStartedRound_AwardsZero, just with
	// a non-empty history first.
	if err := e.RafaleValidateCard(card.ID); err != nil {
		t.Fatalf("validate #1: %v", err)
	}
	if err := e.RafaleValidateCard(card.ID); err != nil {
		t.Fatalf("validate #2: %v", err)
	}
	// State now: RAFALE_ASKED_COUNT=3, RAFALE_CORRECT_COUNT=2.

	// Client sends a deliberately WRONG UNITS (999) — must be ignored
	// entirely, server derives its own Units/UnitsTotal (contract §9.3).
	points, isComplete, err := e.DoneMotionCard(card.ID, "red", 999)
	if err != nil {
		t.Fatalf("DoneMotionCard: %v", err)
	}
	_ = isComplete

	// points = floor(5 * 2 / 3) = 3 (STARS_PRORATA, §6.2 "multiplication
	// avant division").
	want := 3
	if points != want {
		t.Errorf("DoneMotionCard points = %d, want %d (STARS_PRORATA: 5*2/3, server-derived Units=2/UnitsTotal=3, client UNITS=999 ignored)", points, want)
	}

	team := e.GetTeamsAndBumpers().Teams["red"]
	if team == nil || team.TeamPoints != want {
		gotPts := -1
		if team != nil {
			gotPts = team.TeamPoints
		}
		t.Errorf("team red TeamPoints = %d, want %d", gotPts, want)
	}
}

func TestDoneMotionCard_RafaleCard_NeverStartedRound_AwardsZero(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	card := rafaleMotionCard("mc-neverstarted", []string{string(CategoryHistory)}, []int{1}, 3, 10)
	q := makeMotionQuestion("mq-rafale-ns", []MotionCard{card}, "SOLO")
	startMEMOTION(t, e, "mq-rafale-ns", q)
	if err := e.SelectMotionCard(card.ID); err != nil {
		t.Fatalf("SelectMotionCard: %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard: %v", err)
	}
	defer e.Stop()
	// Deliberately never call StartRafaleMotionCardRound — the animateur
	// closes the card immediately (UnitsTotal=0 guard, contract §6.2).

	points, _, err := e.DoneMotionCard(card.ID, "red", 1)
	if err != nil {
		t.Fatalf("DoneMotionCard: %v", err)
	}
	if points != 0 {
		t.Errorf("DoneMotionCard points = %d, want 0 (UnitsTotal<=0 guard — a card closed before its round ever started)", points)
	}
}

// ---------------------------------------------------------------------------
// Isolation between two RAFALE cards played in sequence — never mixed
// counters (manual procedure rafale-memotion-217.md, scénario 2 étape 4).
// ---------------------------------------------------------------------------

func TestRafaleCard_TwoCardsPlayedInSequence_CountersNeverMixed(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h", 10, CategoryHistory, 1)
	seedRafaleReservoirCouple(t, e, "s", 10, CategoryScience, 1)
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})

	cardA := rafaleMotionCard("mc-a", []string{string(CategoryHistory)}, []int{1}, 3, 10)
	cardB := rafaleMotionCard("mc-b", []string{string(CategoryScience)}, []int{1}, 3, 10)
	q := makeMotionQuestion("mq-two-cards", []MotionCard{cardA, cardB}, "SOLO")
	startMEMOTION(t, e, "mq-two-cards", q)
	defer e.Stop()

	// Play card A: 2 validates.
	if err := e.SelectMotionCard(cardA.ID); err != nil {
		t.Fatalf("SelectMotionCard(A): %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard(A): %v", err)
	}
	if _, _, _, err := e.StartRafaleMotionCardRound(cardA.ID); err != nil {
		t.Fatalf("StartRafaleMotionCardRound(A): %v", err)
	}
	if err := e.RafaleValidateCard(cardA.ID); err != nil {
		t.Fatalf("validate A #1: %v", err)
	}
	if err := e.RafaleValidateCard(cardA.ID); err != nil {
		t.Fatalf("validate A #2: %v", err)
	}
	if _, _, err := e.DoneMotionCard(cardA.ID, "red", 1); err != nil {
		t.Fatalf("DoneMotionCard(A): %v", err)
	}

	// Play card B fresh: 1 validate only. MEMOTION_ACTIVE resets to empty at
	// every MEMOTION_SELECT (contract §5.1) — the acquired-for-free
	// mechanism this test actually verifies still holds for RAFALE.
	if err := e.SelectMotionCard(cardB.ID); err != nil {
		t.Fatalf("SelectMotionCard(B): %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard(B): %v", err)
	}
	if _, _, _, err := e.StartRafaleMotionCardRound(cardB.ID); err != nil {
		t.Fatalf("StartRafaleMotionCardRound(B): %v", err)
	}

	state := e.GetState()
	if state.MotionActive.CardID != cardB.ID {
		t.Fatalf("MotionActive.CardID = %q, want %q", state.MotionActive.CardID, cardB.ID)
	}
	if asked, _ := state.MotionActive.State["RAFALE_ASKED_COUNT"].(int); asked != 1 {
		t.Errorf("card B's own RAFALE_ASKED_COUNT = %v, want 1 — must never inherit card A's count (3)", state.MotionActive.State["RAFALE_ASKED_COUNT"])
	}
	if correct, _ := state.MotionActive.State["RAFALE_CORRECT_COUNT"].(int); correct != 0 {
		t.Errorf("card B's own RAFALE_CORRECT_COUNT = %v, want 0 — must never inherit card A's count (2)", state.MotionActive.State["RAFALE_CORRECT_COUNT"])
	}
	cur, _ := state.MotionActive.State["RAFALE_CURRENT_QUESTION"].(RafaleCurrent)
	if cur.Category != string(CategoryScience) {
		t.Errorf("card B's own RAFALE_CURRENT_QUESTION.CATEGORY = %q, want SCIENCE — must draw from its own configured couple, never card A's HISTORY", cur.Category)
	}
}
