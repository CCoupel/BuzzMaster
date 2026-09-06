package game

// Tests for #216 — RAFALE multi-catégorie / multi-difficulté (milestone
// v9.0.0, Batch 1, Lot 1B). Written contract-first from the plan
// (_work/reports/plan-v900-consolide-20260904-150000.md §4/§6 Lot 1B) in
// parallel with dev-backend's implementation — see the coordination note
// below for the naming this file commits to.
//
// Contract under test (plan §4, #216):
//   - A round now accepts N categories and N difficulties
//     (TypedContent.RafaleCategories []string, RafaleDifficulties []int —
//     replacing the single Category/RafaleDifficulty pair a RAFALE round
//     used to filter on).
//   - Existing mono-category/mono-difficulty questions must keep working
//     unmodified (retro-compatible reading).
//   - The draw is a random PERMUTATION of the category×difficulty cartesian
//     product, replayed every cycle: every couple must be drawn once before
//     any of them repeats — not a flat uniform draw across the union (which
//     would statistically favor couples with a larger pool).
//   - An empty or exhausted couple is dropped from the active list and the
//     remaining couples rebalance — identical handling whether the couple
//     was already empty at round start or became exhausted mid-round.
//
// Naming: TypedContent.RafaleCategories []string (json RAFALE_CATEGORIES),
// RafaleDifficulties []int (json RAFALE_DIFFICULTIES) and
// RafalePointsByDifficulty map[int]int (json RAFALE_POINTS_BY_DIFFICULTY) —
// confirmed against contracts/rafale.md §3.3/§7 (dev-backend's contract-first
// commit, read in parallel while writing this file). RafaleCurrent gains a
// Points int field (§4) — the barème resolved for the CURRENT question
// (RAFALE_POINTS_BY_DIFFICULTY[difficulty] if defined and >0, else the
// round's generic POINTS), broadcast for TV/animateur display. Every test
// below otherwise drives ONLY the existing public surface (Ready,
// SetRafaleParticipatingTeams, StartImmediate, RafaleValidate,
// RafaleInvalidate, GetState) so it stays valid regardless of how the
// permutation/rebalancing logic (contract §7.2's unexported rafaleDrawState)
// is implemented internally.

import (
	"fmt"
	"testing"
)

// makeRafaleQuestionMulti builds a RAFALE round-config Question using the new
// list-based filter fields. mode defaults to SOLO semantics via
// RafaleModeSolo when empty is not meaningful here — every test in this file
// uses SOLO explicitly to keep team-rotation out of scope (already covered
// by rafale_modes_test.go).
func makeRafaleQuestionMulti(id string, categories []string, difficulties []int) *Question {
	return &Question{
		ID:       id,
		Question: "RAFALE round " + id,
		Type:     QuestionTypeRafale,
		Points:   "10",
		Time:     "120",
		TypedContent: TypedContent{
			RafaleCategories:   categories,
			RafaleDifficulties: difficulties,
			RafaleMode:         string(RafaleModeSolo),
			RafaleQuestionTime: 3,
			RafaleMaxQuestions: 100,
		},
	}
}

// seedRafaleReservoirCouple seeds n reservoir questions for exactly one
// category/difficulty couple, IDs unique across the whole test (prefix
// disambiguates couples sharing a reservoir).
func seedRafaleReservoirCouple(t *testing.T, e *Engine, prefix string, n int, category QuestionCategory, difficulty int) {
	t.Helper()
	questions := make([]RafaleQuestion, 0, n)
	for i := 1; i <= n; i++ {
		questions = append(questions, RafaleQuestion{
			ID: fmt.Sprintf("%s-%d", prefix, i), Question: "Q", Answer: "A",
			Category: category, Difficulty: difficulty,
		})
	}
	seedRafaleReservoir(t, e, questions)
}

func startSoloRafaleRound(t *testing.T, e *Engine, q *Question) {
	t.Helper()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	e.Ready(q.ID, q)
	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams failed: %v", err)
	}
	e.StartImmediate(0)
}

// ---------------------------------------------------------------------------
// Retro-compatibility: existing mono-category/mono-difficulty questions.
// ---------------------------------------------------------------------------

// TestRafaleRound_RetroCompat_MonoCategoryDifficultyStillWorks verifies that
// a round configured the OLD way (single Category + RafaleDifficulty, no
// RafaleCategories/RafaleDifficulties set at all) still runs exactly as
// before #216 — the plan's explicit non-regression requirement ("questions
// RAFALE existantes converties automatiquement en listes à un élément").
func TestRafaleRound_RetroCompat_MonoCategoryDifficultyStillWorks(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "hist1", 5, CategoryHistory, 1)
	seedRafaleReservoirCouple(t, e, "sci1", 5, CategoryScience, 1) // decoy — must never be drawn

	q := &Question{
		ID:       "rq-mono",
		Question: "RAFALE round mono",
		Type:     QuestionTypeRafale,
		Category: CategoryHistory,
		Points:   "10",
		Time:     "120",
		TypedContent: TypedContent{
			RafaleDifficulty:   1, // OLD singular field only
			RafaleMode:         string(RafaleModeSolo),
			RafaleQuestionTime: 3,
			RafaleMaxQuestions: 100,
		},
	}
	startSoloRafaleRound(t, e, q)
	defer e.Stop()

	state := e.GetState()
	if state.RafaleSubPhase == RafaleSubPhaseRoundEnd {
		t.Fatal("round ended immediately — mono-format question failed to start (retro-compat broken)")
	}
	if state.RafaleCurrentQuestion.Category != string(CategoryHistory) || state.RafaleCurrentQuestion.Difficulty != 1 {
		t.Fatalf("first question = %+v, want CATEGORY=HISTORY DIFFICULTY=1 (mono config)", state.RafaleCurrentQuestion)
	}

	for i := 0; i < 3; i++ {
		if err := e.RafaleValidate(); err != nil {
			t.Fatalf("RafaleValidate #%d failed: %v", i, err)
		}
		state = e.GetState()
		if state.RafaleSubPhase == RafaleSubPhaseRoundEnd {
			break
		}
		if state.RafaleCurrentQuestion.Category != string(CategoryHistory) || state.RafaleCurrentQuestion.Difficulty != 1 {
			t.Errorf("draw #%d = %+v, want CATEGORY=HISTORY DIFFICULTY=1 only — SCIENCE decoy must never leak in via mono config", i, state.RafaleCurrentQuestion)
		}
	}
}

// ---------------------------------------------------------------------------
// Multi-category/multi-difficulty: ensemblist membership.
// ---------------------------------------------------------------------------

// TestRafaleRound_MultiCategoryDifficulty_UnionOnly verifies that a round
// configured with N categories and N difficulties only ever draws questions
// whose (category, difficulty) is one of the requested couples — a decoy
// couple outside the set must never be drawn, mirroring the intersection
// guarantee already established for the mono case (#107).
func TestRafaleRound_MultiCategoryDifficulty_UnionOnly(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h1", 5, CategoryHistory, 1)
	seedRafaleReservoirCouple(t, e, "h2", 5, CategoryHistory, 2)
	seedRafaleReservoirCouple(t, e, "s1", 5, CategoryScience, 1)
	seedRafaleReservoirCouple(t, e, "s2", 5, CategoryScience, 2)
	seedRafaleReservoirCouple(t, e, "decoy", 5, CategorySports, 1) // outside the requested set

	q := makeRafaleQuestionMulti("rq-multi",
		[]string{string(CategoryHistory), string(CategoryScience)},
		[]int{1, 2},
	)
	startSoloRafaleRound(t, e, q)
	defer e.Stop()

	allowed := map[string]bool{
		string(CategoryHistory) + "/1": true,
		string(CategoryHistory) + "/2": true,
		string(CategoryScience) + "/1": true,
		string(CategoryScience) + "/2": true,
	}

	check := func(label string, cur RafaleCurrent) {
		key := cur.Category + "/" + fmt.Sprint(cur.Difficulty)
		if !allowed[key] {
			t.Errorf("%s: drew %+v — outside the requested categories×difficulties set (decoy SPORTS/1 must never leak in)", label, cur)
		}
	}

	state := e.GetState()
	check("first question", state.RafaleCurrentQuestion)

	for i := 0; i < 9; i++ { // covers more than 2 full cycles of 4 couples
		if err := e.RafaleValidate(); err != nil {
			t.Fatalf("RafaleValidate #%d failed: %v", i, err)
		}
		state = e.GetState()
		if state.RafaleSubPhase == RafaleSubPhaseRoundEnd {
			t.Fatalf("round ended prematurely after %d advances — increase reservoir headroom", i+1)
		}
		check(fmt.Sprintf("draw #%d", i), state.RafaleCurrentQuestion)
	}
}

// ---------------------------------------------------------------------------
// Balanced draw: permutation of the cartesian product, replayed every cycle.
// ---------------------------------------------------------------------------

// TestRafaleRound_CoupleSequence_CoversEachCoupleOncePerCycleBeforeRepeating
// is the core #216 test: with 4 couples (2 categories × 2 difficulties) and
// generous per-couple headroom (no couple exhausts across the observed
// window), every block of 4 consecutive draws must be a permutation of the
// SAME 4 couples — none repeated within a block, none missing. A flat
// uniform draw across the union (the naive alternative) would not guarantee
// this and would drift over a long enough run.
//
// This also doubles as the #202 (prefetch) interaction test: the very first
// question is drawn directly at round start while the SECOND is immediately
// pre-fetched (startRafaleRoundUnsafe → prefetchRafaleNextUnsafe, before any
// RafaleValidate() call) — both must draw from the SAME shared cycle, or the
// very first block of 4 in the assertion below would already fail.
func TestRafaleRound_CoupleSequence_CoversEachCoupleOncePerCycleBeforeRepeating(t *testing.T) {
	e := NewEngine()
	const perCouple = 6 // 6 questions/couple: survives well past 2 full cycles (needs only 2/couple)
	seedRafaleReservoirCouple(t, e, "h1", perCouple, CategoryHistory, 1)
	seedRafaleReservoirCouple(t, e, "h2", perCouple, CategoryHistory, 2)
	seedRafaleReservoirCouple(t, e, "s1", perCouple, CategoryScience, 1)
	seedRafaleReservoirCouple(t, e, "s2", perCouple, CategoryScience, 2)

	q := makeRafaleQuestionMulti("rq-cycle",
		[]string{string(CategoryHistory), string(CategoryScience)},
		[]int{1, 2},
	)
	startSoloRafaleRound(t, e, q)
	defer e.Stop()

	coupleKey := func(cat string, diff int) string { return cat + "/" + fmt.Sprint(diff) }

	state := e.GetState()
	sequence := []string{coupleKey(state.RafaleCurrentQuestion.Category, state.RafaleCurrentQuestion.Difficulty)}

	const numAdvances = 7 // 8 draws total = exactly 2 full cycles of 4 couples
	for i := 0; i < numAdvances; i++ {
		if err := e.RafaleValidate(); err != nil {
			t.Fatalf("RafaleValidate #%d failed: %v", i, err)
		}
		state = e.GetState()
		if state.RafaleSubPhase == RafaleSubPhaseRoundEnd {
			t.Fatalf("round ended prematurely after %d advances (pool exhausted?) — increase reservoir headroom", i+1)
		}
		sequence = append(sequence, coupleKey(state.RafaleCurrentQuestion.Category, state.RafaleCurrentQuestion.Difficulty))
	}

	wantCouples := []string{
		coupleKey(string(CategoryHistory), 1),
		coupleKey(string(CategoryHistory), 2),
		coupleKey(string(CategoryScience), 1),
		coupleKey(string(CategoryScience), 2),
	}

	for cycle := 0; (cycle+1)*4 <= len(sequence); cycle++ {
		block := sequence[cycle*4 : cycle*4+4]
		seen := map[string]int{}
		for _, c := range block {
			seen[c]++
		}
		for _, couple := range wantCouples {
			if seen[couple] != 1 {
				t.Errorf("cycle %d (draws %v): couple %s appeared %d times, want exactly 1 — every couple must be drawn once per cycle before any repeats (#216 permutation contract)", cycle, block, couple, seen[couple])
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Empty/exhausted couple: dropped from the active list, others rebalance —
// identical handling at launch and mid-round.
// ---------------------------------------------------------------------------

// TestRafaleRound_CoupleEmptyAtLaunch_NeverBlocksRound verifies that a
// requested couple with NO matching reservoir questions at all (never
// populated) does not block the round from starting or from cycling through
// the remaining couples — it is simply never drawable.
func TestRafaleRound_CoupleEmptyAtLaunch_NeverBlocksRound(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h1", 6, CategoryHistory, 1)
	seedRafaleReservoirCouple(t, e, "h2", 6, CategoryHistory, 2)
	seedRafaleReservoirCouple(t, e, "s1", 6, CategoryScience, 1)
	// CategoryScience/2 deliberately left EMPTY — requested but unpopulated.

	q := makeRafaleQuestionMulti("rq-empty-launch",
		[]string{string(CategoryHistory), string(CategoryScience)},
		[]int{1, 2},
	)
	startSoloRafaleRound(t, e, q)
	defer e.Stop()

	emptyCouple := string(CategoryScience) + "/2"
	state := e.GetState()
	got := state.RafaleCurrentQuestion.Category + "/" + fmt.Sprint(state.RafaleCurrentQuestion.Difficulty)
	if got == emptyCouple {
		t.Fatalf("first question drawn from the empty couple %s — an unpopulated couple must never be drawable", emptyCouple)
	}
	if state.RafaleSubPhase == RafaleSubPhaseRoundEnd {
		t.Fatal("round ended immediately — an empty couple among 4 requested must not block the round (3 others are populated)")
	}

	for i := 0; i < 6; i++ {
		if err := e.RafaleValidate(); err != nil {
			t.Fatalf("RafaleValidate #%d failed: %v", i, err)
		}
		state = e.GetState()
		if state.RafaleSubPhase == RafaleSubPhaseRoundEnd {
			break
		}
		got := state.RafaleCurrentQuestion.Category + "/" + fmt.Sprint(state.RafaleCurrentQuestion.Difficulty)
		if got == emptyCouple {
			t.Errorf("draw #%d came from the empty couple %s", i, emptyCouple)
		}
	}
}

// TestRafaleRound_CoupleExhaustedMidRound_RebalancesLikeEmptyAtLaunch
// verifies the plan's explicit "comportement unifié lancement/cours de
// manche" requirement: a couple that starts populated but runs out of
// questions PARTWAY through the round must be dropped from the active list
// exactly like one that was empty from the start — the round must keep
// cycling through the remaining couples, never block or crash.
func TestRafaleRound_CoupleExhaustedMidRound_RebalancesLikeEmptyAtLaunch(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h1", 6, CategoryHistory, 1)
	seedRafaleReservoirCouple(t, e, "h2", 6, CategoryHistory, 2)
	seedRafaleReservoirCouple(t, e, "s1", 6, CategoryScience, 1)
	seedRafaleReservoirCouple(t, e, "s2", 1, CategoryScience, 2) // exhausts after exactly 1 draw

	q := makeRafaleQuestionMulti("rq-exhaust-mid",
		[]string{string(CategoryHistory), string(CategoryScience)},
		[]int{1, 2},
	)
	startSoloRafaleRound(t, e, q)
	defer e.Stop()

	scarceCouple := string(CategoryScience) + "/2"
	drawsFromScarce := 0
	tally := func(cur RafaleCurrent) {
		if cur.Category+"/"+fmt.Sprint(cur.Difficulty) == scarceCouple {
			drawsFromScarce++
		}
	}

	state := e.GetState()
	tally(state.RafaleCurrentQuestion)

	const numAdvances = 9
	for i := 0; i < numAdvances; i++ {
		if err := e.RafaleValidate(); err != nil {
			t.Fatalf("RafaleValidate #%d failed: %v", i, err)
		}
		state = e.GetState()
		if state.RafaleSubPhase == RafaleSubPhaseRoundEnd {
			t.Fatalf("round ended prematurely after %d advances — exhausting ONE of 4 couples must not end the round (3 others still have plenty)", i+1)
		}
		tally(state.RafaleCurrentQuestion)
	}

	if drawsFromScarce > 1 {
		t.Errorf("scarce couple %s (only 1 reservoir question) was drawn from %d times — must be dropped from the active list once exhausted, never retried", scarceCouple, drawsFromScarce)
	}
}

// ---------------------------------------------------------------------------
// Registry: OwnedFields must expose the new list fields.
// ---------------------------------------------------------------------------

// TestRafaleOwnedFields_IncludesNewListFields verifies that the type
// registry (question_types.go) declares RAFALE_CATEGORIES and
// RAFALE_DIFFICULTIES as RAFALE-owned fields, so the generic
// clear-foreign-fields machinery (used across every question type) knows
// to preserve/clear them correctly for a RAFALE question. Additive
// assertion only — deliberately does not assert on the legacy
// RAFALE_DIFFICULTY entry's presence/absence, since keeping it (for
// retro-compatible reading) is a legitimate design choice this test does
// not need to pin down.
func TestRafaleOwnedFields_IncludesNewListFields(t *testing.T) {
	def, ok := questionTypeRegistry[QuestionTypeRafale]
	if !ok {
		t.Fatal("QuestionTypeRafale missing from questionTypeRegistry")
	}
	owned := map[string]bool{}
	for _, f := range def.OwnedFields {
		owned[f] = true
	}
	for _, want := range []string{"RAFALE_CATEGORIES", "RAFALE_DIFFICULTIES"} {
		if !owned[want] {
			t.Errorf("QuestionTypeRafale.OwnedFields = %v, missing %q (#216)", def.OwnedFields, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Barème par difficulté (contract §3.3/§4, 216-Q7): RAFALE_CURRENT_QUESTION.
// POINTS resolves to RAFALE_POINTS_BY_DIFFICULTY[difficulty] when defined
// and >0, else falls back to the round's generic POINTS — same resolved
// value for the announced NEXT question (#202 prefetch preview).
// ---------------------------------------------------------------------------

// TestRafaleCurrentQuestion_Points_UsesBaremeWhenDefined verifies that the
// per-difficulty barème, when configured, drives RAFALE_CURRENT_QUESTION's
// resolved POINTS instead of the generic round POINTS. Single couple
// (HISTORY/2) keeps which barème entry applies unambiguous.
func TestRafaleCurrentQuestion_Points_UsesBaremeWhenDefined(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h2", 5, CategoryHistory, 2)

	q := makeRafaleQuestionMulti("rq-bareme", []string{string(CategoryHistory)}, []int{2})
	q.Points = "10" // generic fallback — must NOT be the value used here
	q.RafalePointsByDifficulty = map[int]int{2: 15}

	startSoloRafaleRound(t, e, q)
	defer e.Stop()

	state := e.GetState()
	if state.RafaleCurrentQuestion.Points != 15 {
		t.Errorf("RAFALE_CURRENT_QUESTION.POINTS = %d, want 15 (RAFALE_POINTS_BY_DIFFICULTY[2], not the generic POINTS=10)", state.RafaleCurrentQuestion.Points)
	}
}

// TestRafaleCurrentQuestion_Points_FallsBackToGenericPoints verifies that
// without a per-difficulty barème (or with an entry that is absent/<=0 for
// the drawn difficulty), RAFALE_CURRENT_QUESTION.POINTS falls back to the
// round's generic POINTS — the exact pre-#216 behavior, so a RAFALE round
// never reconfigured with a barème shows no regression.
func TestRafaleCurrentQuestion_Points_FallsBackToGenericPoints(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h1", 5, CategoryHistory, 1)

	q := makeRafaleQuestionMulti("rq-fallback", []string{string(CategoryHistory)}, []int{1})
	q.Points = "7" // no RafalePointsByDifficulty set at all

	startSoloRafaleRound(t, e, q)
	defer e.Stop()

	state := e.GetState()
	if state.RafaleCurrentQuestion.Points != 7 {
		t.Errorf("RAFALE_CURRENT_QUESTION.POINTS = %d, want 7 (fallback to the generic POINTS, no barème configured)", state.RafaleCurrentQuestion.Points)
	}
}
