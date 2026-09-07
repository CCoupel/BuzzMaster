package game

// C2 regression — retour QUALIF v9.0.0.4
// (plan-v900-correctifs-qualif-20260906-104500.md §5 Lot C, §7 "dette de
// test #217"): engine.go:2539's `.(int)` type assertion on
// MotionActive.State["RAFALE_QUESTION_TIME"] fails silently (ok=false) the
// moment that value has passed through a JSON encode/decode boundary —
// Go's encoding/json decodes every JSON number into float64 when the
// target is `interface{}`, never int. `qt` then stays at its zero value,
// `qt--` makes it -1, and `qt > 0` is false: the question is treated as
// EXPIRED on the very next tick, silently and without any log — the same
// class of "assertion succeeds for a freshly-set Go int, breaks the moment
// the same value round-trips through JSON" bug this file's sibling
// (motionActiveQCMInvalidated, engine.go:6937, the tolerant-helper
// precedent dev-backend's own C2 handoff points to) already guards against
// for a different field.
//
// This test injects the EXACT corruption a round-trip produces
// (`float64(n)` instead of `int(n)` in the map slot) directly, rather than
// depending on which internal code path actually performs that round-trip
// in production — the invariant under test ("the tick logic must tolerate
// either representation") holds regardless of how the value got there, and
// this keeps the test valid even if that call path changes later.

import "testing"

func TestRafaleCardQuestionTick_TwoConsecutiveTicks_ToleratesFloat64AfterJSONRoundTrip(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h", 10, CategoryHistory, 1)
	card := rafaleMotionCard("mc-c2", []string{string(CategoryHistory)}, []int{1}, 3, 10)
	_, _, questionTime := startMEMOTIONAtRafaleCardQuestion(t, e, card)
	defer e.Stop()

	// StartRafaleQuestionTimer seeds MotionActive.State["RAFALE_QUESTION_TIME"]
	// — production's own responsibility (handleMotionFlip, main.go), not
	// StartRafaleMotionCardRound itself (dev-backend's own
	// TestStartRafaleQuestionTimer_CardHost_SeedsMotionActiveState_NotGlobalField
	// documents this split) — startMEMOTIONAtRafaleCardQuestion only does the
	// latter, so this call is required here too.
	e.StartRafaleQuestionTimer(questionTime)

	if qt, _ := e.GetState().MotionActive.State["RAFALE_QUESTION_TIME"].(int); qt != 3 {
		t.Fatalf("setup: RAFALE_QUESTION_TIME = %v, want 3 (int)", e.GetState().MotionActive.State["RAFALE_QUESTION_TIME"])
	}

	// Tick #1 — normal case, the value is still a freshly-set Go int.
	e.processRafaleQuestionTick()
	state := e.GetState()
	qt1, ok1 := state.MotionActive.State["RAFALE_QUESTION_TIME"].(int)
	if !ok1 || qt1 != 2 {
		t.Fatalf("after tick #1: RAFALE_QUESTION_TIME = %v (ok=%v), want 2 (int)", state.MotionActive.State["RAFALE_QUESTION_TIME"], ok1)
	}
	if sub, _ := state.MotionActive.State["RAFALE_SUBPHASE"].(string); sub != string(RafaleSubPhaseQuestion) {
		t.Fatalf("after tick #1: RAFALE_SUBPHASE = %q, want %q (not yet expired)", sub, RafaleSubPhaseQuestion)
	}

	// Simulate exactly what a JSON round-trip does to this map slot —
	// encoding/json always decodes a JSON number into float64 for an
	// interface{} target, never int. C2's whole point: the SAME numeric
	// value (2), just the OTHER concrete type encoding/json would produce.
	// MotionActive.State is a map (reference type) — GetState()'s value
	// copy of GameState still shares the SAME underlying map, so mutating
	// it here reaches the engine's real internal state.
	state.MotionActive.State["RAFALE_QUESTION_TIME"] = float64(qt1)

	// Tick #2 — must still correctly decrement (2 -> 1), NOT treat the
	// float64-typed 2 as "absent" (defaulting to Go zero value 0, then
	// qt-- => -1 => qt > 0 is false => wrongly expired). Read tolerantly
	// here (int OR float64) since asserting which concrete type the FIX
	// leaves behind would over-specify dev-backend's implementation choice
	// — the invariant under test is "never wrongly expired", not "stored
	// as exactly this Go type".
	e.processRafaleQuestionTick()
	state = e.GetState()
	var qt2 float64
	switch v := state.MotionActive.State["RAFALE_QUESTION_TIME"].(type) {
	case int:
		qt2 = float64(v)
	case float64:
		qt2 = v
	default:
		t.Fatalf("after tick #2 (post round-trip): RAFALE_QUESTION_TIME has unexpected type %T (value %v)", v, v)
	}
	if qt2 != 1 {
		t.Errorf("after tick #2 (post round-trip): RAFALE_QUESTION_TIME = %v, want 1 — C2: the fragile .(int) assertion must not silently expire the question", qt2)
	}
	if sub, _ := state.MotionActive.State["RAFALE_SUBPHASE"].(string); sub != string(RafaleSubPhaseQuestion) {
		t.Errorf("after tick #2 (post round-trip): RAFALE_SUBPHASE = %q, want %q — must NOT have expired/redrawn (the exact C2 symptom: a live question silently treated as expired)", sub, RafaleSubPhaseQuestion)
	}
}
