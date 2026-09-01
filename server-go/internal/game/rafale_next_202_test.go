// Tests for #202 — RAFALE pre-fetch of the "next" question (contract
// rafale.md §13). Covers the engine-level cycle of life described in §13.4:
// prefetch on round start/every advance, consumption at the FOLLOWING
// advance (never re-drawn independently), release on every manche-ending
// path, RAFALE_POOL_REMAINING's preserved semantics, RAFALE_EXHAUSTED's
// unchanged timing, and the RAFALE_MAX_QUESTIONS cap's unchanged stop
// point.
//
// Deliberately does NOT modify any expected value in rafale_test.go/
// rafale_modes_test.go (#107/#199) — contract §13.8 requires those suites
// to stay green UNMODIFIED as the primary non-regression signal for this
// feature; confirmed by the full `go test ./internal/game/...` run staying
// green throughout this feature's development, with zero fixture changes
// to any pre-#202 file.
//
// Run: go test ./internal/game/... -run TestRafaleNext -v
package game

import "testing"

// TestRafaleNext_ConsumedQuestionMatchesAnnouncedNext is the central
// correctness property of the whole feature (contract §13.1): the question
// announced as "next" (via OnRafaleAnswer's own next parameter) at one
// advance MUST be exactly the one that becomes RAFALE_CURRENT_QUESTION at
// the FOLLOWING advance — never re-drawn independently, which could
// disagree with the announced preview in the general case.
func TestRafaleNext_ConsumedQuestionMatchesAnnouncedNext(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	seedRafaleReservoirBulk(t, e, 10, CategoryHistory, 1)

	var lastNext *RafaleCurrent
	e.OnRafaleAnswer = func(id, answer string, next *RafaleCurrent) {
		lastNext = next
	}

	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	e.StartImmediate(0)
	defer e.Stop()

	if lastNext == nil {
		t.Fatalf("expected a non-nil pre-fetched next question after round start (ample pool)")
	}
	announcedNextID := lastNext.ID
	announcedNextQuestion := lastNext.Question

	if err := e.RafaleValidate(); err != nil {
		t.Fatalf("RafaleValidate: %v", err)
	}

	state := e.GetState()
	if state.RafaleCurrentQuestion.ID != announcedNextID {
		t.Errorf("BUG: the question that became current (%q) does not match the one announced as NEXT (%q) — a second, independent draw happened", state.RafaleCurrentQuestion.ID, announcedNextID)
	}
	if state.RafaleCurrentQuestion.Question != announcedNextQuestion {
		t.Errorf("BUG: current question statement (%q) does not match the announced NEXT statement (%q)", state.RafaleCurrentQuestion.Question, announcedNextQuestion)
	}

	// Repeat once more (INVALIDATE this time) to prove it's not a
	// one-shot coincidence.
	if lastNext == nil {
		t.Fatalf("expected a non-nil pre-fetched next question after the first advance")
	}
	announcedNextID = lastNext.ID
	if err := e.RafaleInvalidate(); err != nil {
		t.Fatalf("RafaleInvalidate: %v", err)
	}
	if got := e.GetState().RafaleCurrentQuestion.ID; got != announcedNextID {
		t.Errorf("BUG (2nd advance): current question (%q) does not match the announced NEXT (%q)", got, announcedNextID)
	}
}

// TestRafaleNext_NilWhenPoolEmptyAfterFirstDraw covers contract §13.5's
// first row inverted: a pool with exactly ONE question leaves NEXT nil
// right after round start (the only question was drawn as current, nothing
// left to pre-fetch).
func TestRafaleNext_NilWhenPoolEmptyAfterFirstDraw(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	seedRafaleReservoirBulk(t, e, 1, CategoryHistory, 1)

	var lastNext *RafaleCurrent
	nextCalls := 0
	e.OnRafaleAnswer = func(id, answer string, next *RafaleCurrent) {
		lastNext = next
		nextCalls++
	}

	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	e.StartImmediate(0)
	defer e.Stop()

	if nextCalls != 1 {
		t.Fatalf("sanity: expected exactly 1 OnRafaleAnswer call at round start, got %d", nextCalls)
	}
	if lastNext != nil {
		t.Errorf("BUG: expected NEXT=nil with a 1-question pool, got %+v", lastNext)
	}
}

// TestRafaleNext_NilOnlyOnceCapActuallyReached covers contract §13.4's "no
// pre-fetch when the cap is about to stop the manche" rule precisely:
// RAFALE_MAX_QUESTIONS=2 with an ample pool must still show a real NEXT
// question after asking question 1 (question 2 IS going to be legitimately
// asked — the cap allows 2 total), and only become nil once question 2
// itself has been asked (RafaleAskedCount==2==cap, nothing more will ever
// be asked). Deliberately NOT "nil as soon as AskedCount+1==cap" — that
// off-by-one would skip pre-fetching a question the cap still allows,
// caught during development by TestRafaleNext_MaxQuestionsCapUnchanged
// (ending the round one question short of the configured cap) — see
// prefetchRafaleNextUnsafe's own doc comment for the full explanation.
func TestRafaleNext_NilOnlyOnceCapActuallyReached(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	seedRafaleReservoirBulk(t, e, 10, CategoryHistory, 1)

	var lastNext *RafaleCurrent
	e.OnRafaleAnswer = func(id, answer string, next *RafaleCurrent) {
		lastNext = next
	}

	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	q.RafaleMaxQuestions = 2
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	e.StartImmediate(0)
	defer e.Stop()

	if lastNext == nil {
		t.Fatalf("expected a real NEXT question after asking question 1 (cap=2 still allows a 2nd question)")
	}

	if err := e.RafaleValidate(); err != nil {
		t.Fatalf("RafaleValidate: %v", err)
	}
	if state := e.GetState(); state.RafaleAskedCount != 2 {
		t.Fatalf("sanity: expected AskedCount=2 (the cap, question 2 now current), got %d", state.RafaleAskedCount)
	}
	if lastNext != nil {
		t.Errorf("BUG: expected NEXT=nil once RAFALE_MAX_QUESTIONS=2 was actually reached (question 2 is the last one), got %+v", lastNext)
	}

	// Sanity: the pool itself is NOT the reason (plenty of questions remain unused).
	available, _, _ := e.CountRafalePool(string(CategoryHistory), 1)
	if available < 5 {
		t.Fatalf("sanity: expected plenty of available reservoir questions, got %d", available)
	}
}

// TestRafaleNext_ReleasedOnStop is the R1-critical test (plan's own
// wording): a manche STOPPED mid-round — manual STOP or the global round
// timer's own expiry, both funnel through the exact same stopUnsafe() call
// — must NOT consume the pre-fetched "on deck" question from the
// reservoir's point of view. Without release, the pool would show ONE
// FEWER available question than a manche asking the exact same number of
// questions did before #202.
func TestRafaleNext_ReleasedOnStop(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	seedRafaleReservoirBulk(t, e, 10, CategoryHistory, 1)

	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	e.StartImmediate(0)

	availableBeforeStop, _, total := e.CountRafalePool(string(CategoryHistory), 1)
	// 1 question asked + 1 pre-fetched = 2 marked used at this point.
	if availableBeforeStop != total-2 {
		t.Fatalf("sanity: expected 2 questions marked used (1 asked + 1 pre-fetched), got available=%d total=%d", availableBeforeStop, total)
	}

	e.Stop()

	availableAfterStop, _, _ := e.CountRafalePool(string(CategoryHistory), 1)
	// Only the ONE question actually asked should remain marked used — the
	// pre-fetch must be released back to the pool.
	if availableAfterStop != total-1 {
		t.Errorf("BUG (R1): pre-fetched question not released on Stop() — available=%d, want %d (only the 1 ACTUALLY ASKED question should stay used)", availableAfterStop, total-1)
	}
}

// TestRafaleNext_ReleasedOnReadyReplay directly exercises Ready()'s own
// release call (contract §13.4's 3rd row) as an isolated unit: in the real
// end-to-end flow this is defense-in-depth (Ready()'s allowedPhases guard
// already refuses PhaseStarted, so any prior RAFALE round can only reach
// Ready() via stopUnsafe(), which already released the pre-fetch) — this
// test forces the pre-#202-invariant-violating state directly (same
// package, white-box) to prove Ready()'s OWN release logic works correctly
// on its own, independent of whether stopUnsafe() got there first.
func TestRafaleNext_ReleasedOnReadyReplay(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	seedRafaleReservoirBulk(t, e, 10, CategoryHistory, 1)

	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q)

	// Force a pre-fetch to be outstanding, bypassing the normal Start()
	// flow (white-box — same package) — simulates the state Ready()'s own
	// release must handle regardless of how it got there.
	e.mu.Lock()
	drawn, err := e.drawRafaleQuestionUnsafe(string(CategoryHistory), 1)
	if err != nil {
		e.mu.Unlock()
		t.Fatalf("drawRafaleQuestionUnsafe: %v", err)
	}
	e.rafaleNext = drawn
	forcedID := drawn.ID
	e.mu.Unlock()

	e.mu.RLock()
	stillUsed := e.rafaleUsed[forcedID]
	e.mu.RUnlock()
	if !stillUsed {
		t.Fatalf("sanity: expected %q to be marked used after the forced draw", forcedID)
	}

	// Replay the SAME question ID — isNewQuestion is false, but
	// rafaleRoundAlreadyPlayed only fires on RafaleSubPhase != None; force
	// isNewQuestion via a genuinely different ID instead, the simplest way
	// to reliably enter the reset block from this state.
	q2 := makeRafaleQuestion("rq2", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq2", q2)

	e.mu.RLock()
	stillUsed = e.rafaleUsed[forcedID]
	e.mu.RUnlock()
	if stillUsed {
		t.Errorf("BUG: Ready() did not release the outstanding pre-fetch (%q still marked used)", forcedID)
	}
	e.mu.RLock()
	leaked := e.rafaleNext != nil
	e.mu.RUnlock()
	if leaked {
		t.Errorf("BUG: e.rafaleNext still non-nil after Ready() replay")
	}
}

// TestRafaleNext_ReleasedOnInitGame mirrors the ReadyReplay test above for
// InitGame() (contract §13.4's 4th row) — forces an outstanding pre-fetch,
// then verifies InitGame() leaves no dangling e.rafaleNext for the next
// game to silently consume (the rafaleUsed map itself is already wiped
// wholesale by InitGame(), independent of this — see its own comment).
func TestRafaleNext_ReleasedOnInitGame(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	seedRafaleReservoirBulk(t, e, 10, CategoryHistory, 1)

	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q)

	e.mu.Lock()
	drawn, err := e.drawRafaleQuestionUnsafe(string(CategoryHistory), 1)
	if err != nil {
		e.mu.Unlock()
		t.Fatalf("drawRafaleQuestionUnsafe: %v", err)
	}
	e.rafaleNext = drawn
	e.mu.Unlock()

	e.InitGame()

	e.mu.RLock()
	leaked := e.rafaleNext != nil
	e.mu.RUnlock()
	if leaked {
		t.Errorf("BUG: e.rafaleNext still non-nil after InitGame()")
	}
}

// TestRafaleNext_PoolRemainingUnchangedAtSameManchePosition is the R5 test:
// RAFALE_POOL_REMAINING must report the SAME value as before #202 at the
// same manche position — the +1 compensation for the pre-fetch (contract
// §13.4's own formula) must exactly offset the pre-fetch's own consumption
// from the pool, not merely "some plausible number".
func TestRafaleNext_PoolRemainingUnchangedAtSameManchePosition(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	const poolSize = 10
	seedRafaleReservoirBulk(t, e, poolSize, CategoryHistory, 1)

	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	e.StartImmediate(0)
	defer e.Stop()

	// Position: 1 question asked. Before #202: poolSize-1 remaining
	// (nothing pre-fetched). After #202: STILL poolSize-1 — the pre-fetch
	// is accounted for by the +1 compensation, not left showing as "used
	// twice".
	if got := e.GetState().RafalePoolRemaining; got != poolSize-1 {
		t.Errorf("expected RAFALE_POOL_REMAINING=%d after 1 question asked, got %d", poolSize-1, got)
	}

	if err := e.RafaleValidate(); err != nil {
		t.Fatalf("RafaleValidate: %v", err)
	}
	// Position: 2 questions asked.
	if got := e.GetState().RafalePoolRemaining; got != poolSize-2 {
		t.Errorf("expected RAFALE_POOL_REMAINING=%d after 2 questions asked, got %d", poolSize-2, got)
	}
}

// TestRafaleNext_ExhaustedSetAtConsumptionNotPrefetch is the R6-adjacent
// test explicitly required by the plan: RAFALE_EXHAUSTED must be posed at
// the CONSUMPTION of a nil pre-fetch (i.e. one advance AFTER the pool
// actually ran dry), never at the prefetch attempt that discovers the pool
// is empty — contract §13.4's "timing identique à l'existant".
func TestRafaleNext_ExhaustedSetAtConsumptionNotPrefetch(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	// Exactly 2 questions: 1 becomes current at round start, 1 is
	// pre-fetched immediately after — the pool is now empty, but nothing
	// has YET tried to consume a nil pre-fetch.
	seedRafaleReservoirBulk(t, e, 2, CategoryHistory, 1)

	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	e.StartImmediate(0)

	if e.GetState().RafaleExhausted {
		t.Fatalf("BUG: RAFALE_EXHAUSTED already true right after round start (pool has 1 pre-fetched question in flight)")
	}

	// Advance once: consumes the pre-fetched question (pool now has 0 left
	// to prefetch FROM — the prefetch attempt right after this consumption
	// finds an empty pool and leaves e.rafaleNext nil). RAFALE_EXHAUSTED
	// must STILL be false here — nothing has consumed a NIL pre-fetch yet.
	if err := e.RafaleValidate(); err != nil {
		t.Fatalf("RafaleValidate (1st advance): %v", err)
	}
	if e.GetState().RafaleExhausted {
		t.Fatalf("BUG: RAFALE_EXHAUSTED set at the PREFETCH attempt (pool discovered empty), not at consumption — contract §13.4 violated")
	}
	if e.GetState().Phase != PhaseStarted {
		t.Fatalf("sanity: round must still be running (2nd question just became current), got phase=%s", e.GetState().Phase)
	}

	// Advance again: THIS is the consumption of a nil e.rafaleNext — round
	// ends HERE, RAFALE_EXHAUSTED becomes true NOW.
	if err := e.RafaleValidate(); err != nil {
		t.Fatalf("RafaleValidate (2nd advance): %v", err)
	}
	state := e.GetState()
	if !state.RafaleExhausted {
		t.Errorf("expected RAFALE_EXHAUSTED=true after consuming a nil pre-fetch (pool truly exhausted), got false")
	}
	if state.Phase != PhaseStopped || state.RafaleSubPhase != RafaleSubPhaseRoundEnd {
		t.Errorf("expected the round to have ended (Phase=STOPPED, RAFALE_SUBPHASE=ROUND_END), got Phase=%s RAFALE_SUBPHASE=%s", state.Phase, state.RafaleSubPhase)
	}
}

// TestRafaleNext_MaxQuestionsCapUnchanged is the R6 test: the number of
// questions actually asked before the round stops at RAFALE_MAX_QUESTIONS
// must be EXACTLY the cap, same as before #202 — the pre-fetch mechanism
// must never let the round ask one more (or one fewer) question than the
// configured cap.
func TestRafaleNext_MaxQuestionsCapUnchanged(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	seedRafaleReservoirBulk(t, e, 20, CategoryHistory, 1)

	q := makeRafaleQuestion("rq1", string(RafaleModeSolo), CategoryHistory, 1)
	q.RafaleMaxQuestions = 3
	e.Ready("rq1", q)
	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	e.StartImmediate(0) // question 1

	if err := e.RafaleValidate(); err != nil { // question 2
		t.Fatalf("RafaleValidate (1st): %v", err)
	}
	if state := e.GetState(); state.Phase != PhaseStarted || state.RafaleAskedCount != 2 {
		t.Fatalf("sanity: expected STARTED with AskedCount=2, got Phase=%s AskedCount=%d", state.Phase, state.RafaleAskedCount)
	}

	if err := e.RafaleValidate(); err != nil { // question 3
		t.Fatalf("RafaleValidate (2nd): %v", err)
	}
	if state := e.GetState(); state.Phase != PhaseStarted || state.RafaleAskedCount != 3 {
		t.Fatalf("sanity: expected STARTED with AskedCount=3 (the cap, still running its 3rd question), got Phase=%s AskedCount=%d", state.Phase, state.RafaleAskedCount)
	}

	if err := e.RafaleValidate(); err != nil { // cap reached — round ends
		t.Fatalf("RafaleValidate (3rd, hits cap): %v", err)
	}
	state := e.GetState()
	if state.RafaleAskedCount != 3 {
		t.Errorf("BUG (R6): expected exactly 3 questions asked (the configured cap), got %d", state.RafaleAskedCount)
	}
	if state.Phase != PhaseStopped || state.RafaleSubPhase != RafaleSubPhaseRoundEnd {
		t.Errorf("expected the round to have ended at the cap (Phase=STOPPED, RAFALE_SUBPHASE=ROUND_END), got Phase=%s RAFALE_SUBPHASE=%s", state.Phase, state.RafaleSubPhase)
	}
}
