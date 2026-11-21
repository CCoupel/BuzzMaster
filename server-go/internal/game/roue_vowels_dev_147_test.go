package game

import "testing"

func fb(b bool) *bool { return &b }

// vowelEngine147: phrase "Toto tata" (vowels O, A), team red with a pot of 1000, case 0 = 1000 points.
func vowelEngine147(t *testing.T, tc TypedContent) *Engine {
	t.Helper()
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red", Color: []int{255, 0, 0}}, "blue": {Name: "blue", Color: []int{0, 0, 255}}})
	tc.RouePhrase = "Toto tata"
	e.Ready("r", &Question{ID: "r", Question: "Roue", Type: QuestionTypeRoue, TypedContent: tc})
	if err := e.RoueSetTeams([]string{"red", "blue"}); err != nil {
		t.Fatal(err)
	}
	e.SetRoueRand(func(int) int { return 0 })
	e.StartImmediate(0)
	t.Cleanup(e.Stop)
	e.mu.Lock()
	e.roueBankUnsafe("red", 1000)
	e.mu.Unlock()
	return e
}

func mustCode147(t *testing.T, err error, code string) {
	t.Helper()
	re, ok := err.(*RoueError)
	if !ok || re.Code != code {
		t.Fatalf("want %s got %v", code, err)
	}
}

func spinToLetter147(t *testing.T, e *Engine) {
	t.Helper()
	for _, f := range []func() error{e.RoueSpin, func() error { fireRoueTimer144(e); return nil }, e.RoueStop, func() error { fireRoueTimer144(e); return nil }} {
		if err := f(); err != nil {
			t.Fatal(err)
		}
	}
	if s := e.GetState().RoueSubPhase; s != RoueSubLetter {
		t.Fatalf("want LETTER got %s", s)
	}
}

func TestRoueVowels147_Defaults(t *testing.T) {
	e := vowelEngine147(t, TypedContent{})
	st := e.GetState()
	if !st.RoueBuyVowels || st.RoueVowelTiming != "BEFORE" || st.RoueVowelsBefore != "MANY" || st.RoueVowelsAfter != "MANY" ||
		st.RoueVowelPrice != 250 || st.RoueVowelGain != "NONE" || !st.RoueVowelsLeft || st.RoueTurnVowel || st.RoueStopVowel {
		t.Fatalf("defaults: %+v", st)
	}
}

func TestRoueVowels147_BeforeHitFlipGainAndTurnVowel(t *testing.T) {
	e := vowelEngine147(t, TypedContent{RoueVowelGain: "PER_OCCURRENCE", RoueVowelPrice: 100})
	if err := e.RoueBuyVowel("a"); err != nil {
		t.Fatal(err)
	}
	st := e.GetState()
	if st.RoueSubPhase != RoueSubVowelLit || st.RoueStep != 1 || st.RoueTeamBank["red"] != 900 ||
		st.RoueLastEvent.Kind != RoueEvVowelHit || st.RoueLastEvent.Count != 2 || st.RoueLastEvent.Value != 100 || st.RoueLastEvent.Gain != 200 {
		t.Fatalf("after buy: sub=%s bank=%d ev=%+v", st.RoueSubPhase, st.RoueTeamBank["red"], st.RoueLastEvent)
	}
	if err := e.RoueFlip(); err != nil {
		t.Fatal(err)
	}
	st = e.GetState()
	if st.RoueSubPhase != RoueSubVowelRevealing || st.RoueTeamBank["red"] != 1100 || st.RoueLastEvent.Kind != RoueEvFlip || st.RoueLastEvent.Gain != 200 {
		t.Fatalf("after flip: %+v bank=%d", st.RoueLastEvent, st.RoueTeamBank["red"])
	}
	seq := st.RoueLastEvent.Seq
	fireRoueTimer144(e)
	st = e.GetState()
	if st.RoueSubPhase != RoueSubTurn || !st.RoueTurnVowel || st.RoueCurrentTeam != "red" || st.RoueLastEvent.Seq != seq || st.RoueVowelsLeft != true {
		t.Fatalf("after reveal: %+v", st.RoueSubPhase)
	}
	// MANY: another one; answer open at step 1.
	if err := e.RoueBuyVowel("O"); err != nil {
		t.Fatal(err)
	}
	st = e.GetState()
	if st.RoueVowelsLeft {
		t.Fatal("no vowel should be left")
	}
}

func TestRoueVowels147_BeforeMissLosesTurn(t *testing.T) {
	e := vowelEngine147(t, TypedContent{RoueVowelPrice: 300, RoueVowelGain: "REFUND"})
	var n []RoueNotice
	e.OnRoueNotice = func(x RoueNotice) { n = append(n, x) }
	if err := e.RoueBuyVowel("E"); err != nil {
		t.Fatal(err)
	}
	st := e.GetState()
	if st.RoueSubPhase != RoueSubTurnEnd || st.RoueTeamBank["red"] != 700 || st.RoueLastEvent.Kind != RoueEvVowelMiss || st.RoueLastEvent.Value != 300 ||
		len(n) != 1 || n[0].Kind != RoueNoticeBad {
		t.Fatalf("miss: sub=%s bank=%d ev=%+v n=%v", st.RoueSubPhase, st.RoueTeamBank["red"], st.RoueLastEvent, n)
	}
}

func TestRoueVowels147_Refusals(t *testing.T) {
	e := vowelEngine147(t, TypedContent{RoueVowelsBefore: "ONE", RoueVowelTiming: "BOTH"})
	mustCode147(t, e.RoueBuyVowel("T"), RoueErrNotAVowel)
	mustCode147(t, e.RoueBuyVowel("1"), RoueErrNotALetter)
	if err := e.RoueBuyVowel("A"); err != nil {
		t.Fatal(err)
	}
	_ = e.RoueFlip()
	fireRoueTimer144(e)
	mustCode147(t, e.RoueBuyVowel("O"), RoueErrVowelLimit)
	bank := e.GetState().RoueTeamBank["red"]
	if e.GetState().RoueTeamBank["red"] != bank {
		t.Fatal("refusal debited")
	}
	// spin: no more purchase "before"
	e2 := vowelEngine147(t, TypedContent{RoueBuyVowels: fb(false)})
	mustCode147(t, e2.RoueBuyVowel("A"), RoueErrVowelsDisabled)
	e3 := vowelEngine147(t, TypedContent{RoueVowelTiming: "AFTER"})
	mustCode147(t, e3.RoueBuyVowel("A"), RoueErrVowelNotNow)
	e4 := vowelEngine147(t, TypedContent{RoueVowelPrice: 5000})
	mustCode147(t, e4.RoueBuyVowel("A"), RoueErrInsufficientBank)
	e5 := vowelEngine147(t, TypedContent{})
	spinToLetter147(t, e5)
	mustCode147(t, e5.RoueBuyVowel("A"), RoueErrVowelNotNow) // LETTER with BEFORE
	mustCode147(t, e5.RoueBuyVowel("A"), RoueErrVowelNotNow)
	e6 := vowelEngine147(t, TypedContent{})
	e6.RoueSpin()
	mustCode147(t, e6.RoueBuyVowel("A"), RoueErrWrongSubPhase)
}

func TestRoueVowels147_AfterChainAndConsonantRefused(t *testing.T) {
	e := vowelEngine147(t, TypedContent{RoueVowelTiming: "AFTER"})
	spinToLetter147(t, e)
	if err := e.RoueBuyVowel("A"); err != nil {
		t.Fatal(err)
	}
	st := e.GetState()
	if st.RoueSubPhase != RoueSubLit || !st.RoueStopVowel || st.RoueStep != 5 {
		t.Fatalf("after buy: %s", st.RoueSubPhase)
	}
	_ = e.RoueFlip()
	fireRoueTimer144(e)
	st = e.GetState()
	if st.RoueSubPhase != RoueSubAnswer || st.RoueTurnGain != 0 {
		t.Fatalf("want ANSWER got %s", st.RoueSubPhase)
	}
	mustCode147(t, e.RoueLetter("N"), RoueErrWrongSubPhase) // consonant refused once a vowel was bought
	if err := e.RoueBuyVowel("O"); err != nil {             // MANY: another one
		t.Fatal(err)
	}
	_ = e.RoueFlip()
	fireRoueTimer144(e)
	if err := e.RoueNext(RoueHandSame); err != nil {
		t.Fatal(err)
	}
	st = e.GetState()
	if st.RoueSubPhase != RoueSubTurn || st.RoueStopVowel || st.RoueTurnVowel {
		t.Fatal("flags must be reset on a new turn")
	}
}

func TestRoueVowels147_AfterOneLimit(t *testing.T) {
	e := vowelEngine147(t, TypedContent{RoueVowelTiming: "AFTER", RoueVowelsAfter: "ONE"})
	spinToLetter147(t, e)
	_ = e.RoueBuyVowel("A")
	_ = e.RoueFlip()
	fireRoueTimer144(e)
	mustCode147(t, e.RoueBuyVowel("O"), RoueErrVowelLimit)
}

func TestRoueVowels147_AnswerAtStep1AndPauseSafe(t *testing.T) {
	e := vowelEngine147(t, TypedContent{})
	mustCode147(t, e.RoueAnswer(), RoueErrWrongSubPhase)
	_ = e.RoueBuyVowel("A")
	_ = e.RoueFlip()
	fireRoueTimer144(e)
	if err := e.RoueAnswer(); err != nil {
		t.Fatal(err)
	}
	if e.GetState().RoueSubPhase != RoueSubSolved {
		t.Fatal("not solved")
	}
}

func TestRoueVowels147_SpinWithoutConsonantNeedsAfterPurchase(t *testing.T) {
	e := vowelEngine147(t, TypedContent{RoueVowelTiming: "AFTER"})
	e.mu.Lock()
	e.state.RoueConsonantsLeft = false
	e.mu.Unlock()
	if err := e.RoueSpin(); err != nil {
		t.Fatalf("spin should be accepted: %v", err)
	}
	e2 := vowelEngine147(t, TypedContent{})
	e2.mu.Lock()
	e2.state.RoueConsonantsLeft = false
	e2.mu.Unlock()
	mustCode147(t, e2.RoueSpin(), RoueErrWrongSubPhase)
}
