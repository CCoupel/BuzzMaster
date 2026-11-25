package game

import (
	"slices"
	"sync/atomic"
	"testing"
)

// #318 (v13.1.0, LOT-3 T19/T20) — moteur Rafale Picto : mot suivant, passes, LOOP / POOL_END, EPOCH, crédit suggéré.
// Les tirages sont testés par propriétés (le générateur global n'est pas injectable, #188).

type rafalePictoRig struct {
	*pictoRig
	cues atomic.Value // []string
}

func (r *rafalePictoRig) cue(c string) { r.cues.Store(append(r.cueList(), c)) }
func (r *rafalePictoRig) cueList() []string {
	v, _ := r.cues.Load().([]string)
	return slices.Clone(v)
}

func newRafalePictoRig(t *testing.T, tc TypedContent, points string) *rafalePictoRig {
	t.Helper()
	r := &rafalePictoRig{pictoRig: &pictoRig{t: t, e: NewEngine()}}
	r.e.SetTeams(map[string]*Team{"red": {Name: "red", Color: []int{255, 0, 0}, Ready: true}})
	r.e.SetBumpers(map[string]*Bumper{"v-alice": {Name: "Alice", Team: "red", IsVPlayer: true, IsVirtual: true, Connected: true, Ready: true}})
	r.e.OnPictoWord = func(w PictoWordInfo) { r.words = append(r.words, w) }
	r.e.OnPictoCanvasReset = func(ep int) { r.resets = append(r.resets, ep) }
	r.e.OnRafaleCorrect = func() { r.cue("bonne") }
	r.e.OnRafaleCardInvalid = func() { r.cue("mauvaise") }
	r.e.OnRafaleLotEnd = func() { r.cue("fin-lot") }
	q := &Question{ID: "rp", Question: "Rafale Picto", Type: QuestionTypeRafalePicto, Category: "ANIMALS", Time: "90", Points: points, TypedContent: tc}
	r.e.Ready("rp", q)
	r.e.SetBumperReady("v-alice")
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if err := r.e.PictoSetDrawer("v-alice"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.e.Stop() })
	r.e.StartImmediate(60)
	return r
}

func (r *rafalePictoRig) epoch() *int { v := r.st().PictoCanvasEpoch; return &v }

func wordsTC(passLimit, passCost int, loop bool, words ...string) TypedContent {
	tc := TypedContent{RafalePictoWords: words, RafalePictoPassLimit: passLimit, RafalePictoPassCost: passCost}
	if loop {
		tc.RafalePictoPoolEnd = PictoPoolEndLoop
	}
	return tc
}

func TestRafalePicto318_FoundMovesToNextWord_NewEpochEachWord_NoRepeat(t *testing.T) {
	r := newRafalePictoRig(t, wordsTC(0, 0, false, "a", "b", "c"), "10")
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		info, ok := r.e.PictoWord()
		if !ok || seen[info.Word] {
			t.Fatalf("mot %d : %+v ok=%v (déjà vu : %v)", i, info, ok, seen)
		}
		seen[info.Word] = true
		epBefore := r.st().PictoCanvasEpoch
		res, err := r.e.PictoFoundAt(r.epoch())
		if err != nil {
			t.Fatal(err)
		}
		if i < 2 && (res.Phase != PhaseStarted || r.st().PictoCanvasEpoch != epBefore+1) {
			t.Fatalf("après trouvé %d : phase=%s epoch %d→%d (attendu STARTED, +1)", i, res.Phase, epBefore, r.st().PictoCanvasEpoch)
		}
	}
	st := r.st()
	if st.PictoFoundCount != 3 || len(st.PictoFoundWords) != 3 || st.PictoRoundPoints != 30 {
		t.Errorf("trouvés=%d mots=%v points=%d (attendu 3, 3 mots, 30)", st.PictoFoundCount, st.PictoFoundWords, st.PictoRoundPoints)
	}
	if st.Phase != PhaseStopped || st.PictoOutcome != PictoOutcomePoolEnd {
		t.Errorf("plus aucun mot frais, END : phase=%s outcome=%q (attendu STOPPED, POOL_END)", st.Phase, st.PictoOutcome)
	}
	if got := r.cueList(); !slices.Equal(got, []string{"bonne", "bonne", "bonne", "fin-lot"}) {
		t.Errorf("cues = %v (le verdict précède fin-lot)", got)
	}
	if _, ok := r.e.PictoWord(); ok {
		t.Error("après POOL_END aucun mot n'est en jeu")
	}
}

func TestRafalePicto318_Pass_Limit1_Cost_Floor0_NeverPublishesPassedWord(t *testing.T) {
	r := newRafalePictoRig(t, wordsTC(1, 7, false, "a", "b", "c"), "10")
	if st := r.st(); st.PictoPassesLeft != 1 {
		t.Fatalf("PICTO_PASSES_LEFT initial = %d, attendu 1", st.PictoPassesLeft)
	}
	passed, _ := r.e.PictoWord()
	if _, err := r.e.PictoPass(r.epoch()); err != nil {
		t.Fatal(err)
	}
	st := r.st()
	if st.PictoPassCount != 1 || st.PictoPassesLeft != 0 || st.PictoRoundPoints != 0 { // 0*10 - 1*7 → plancher 0
		t.Errorf("passes=%d left=%d points=%d (attendu 1, 0, 0 : plancher)", st.PictoPassCount, st.PictoPassesLeft, st.PictoRoundPoints)
	}
	if slices.Contains(st.PictoFoundWords, passed.Word) {
		t.Error("un mot passé n'est jamais dans PICTO_FOUND_WORDS")
	}
	if cur, _ := r.e.PictoWord(); cur.Word == passed.Word {
		t.Error("END : le mot passé ne revient pas tant qu'il reste du frais")
	}
	_, err := r.e.PictoPass(r.epoch())
	wantPictoErr(t, err, PictoErrPassLimitReached)
	if got := r.cueList(); !slices.Equal(got, []string{"mauvaise"}) {
		t.Errorf("cues = %v", got)
	}
}

func TestRafalePicto318_Pass_Unlimited_PassesLeftMinus1(t *testing.T) {
	r := newRafalePictoRig(t, wordsTC(0, 0, true, "a", "b"), "10")
	for i := 0; i < 5; i++ { // LOOP : les mots passés reviennent, on peut passer sans fin
		if _, err := r.e.PictoPass(r.epoch()); err != nil {
			t.Fatalf("passe %d : %v", i, err)
		}
	}
	if st := r.st(); st.PictoPassesLeft != -1 || st.PictoPassCount != 5 || st.Phase != PhaseStarted {
		t.Errorf("left=%d count=%d phase=%s", st.PictoPassesLeft, st.PictoPassCount, st.Phase)
	}
}

func TestRafalePicto318_Loop_RecyclesOldestPassed_FoundNeverReturns(t *testing.T) {
	r := newRafalePictoRig(t, wordsTC(0, 0, true, "a", "b"), "10")
	first, _ := r.e.PictoWord()
	if _, err := r.e.PictoPass(r.epoch()); err != nil { // passe le 1er mot
		t.Fatal(err)
	}
	second, _ := r.e.PictoWord()
	if second.Word == first.Word {
		t.Fatalf("un mot frais existe : %q ne doit pas revenir tout de suite", second.Word)
	}
	if _, err := r.e.PictoFoundAt(r.epoch()); err != nil { // trouve le 2e : plus de frais, le passé revient
		t.Fatal(err)
	}
	back, ok := r.e.PictoWord()
	if !ok || back.Word != first.Word {
		t.Fatalf("LOOP : le mot passé %q revient, got %+v ok=%v", first.Word, back, ok)
	}
	if _, err := r.e.PictoFoundAt(r.epoch()); err != nil {
		t.Fatal(err)
	}
	if st := r.st(); st.Phase != PhaseStopped || st.PictoOutcome != PictoOutcomePoolEnd || st.PictoFoundCount != 2 {
		t.Errorf("tous trouvés : phase=%s outcome=%q found=%d", st.Phase, st.PictoOutcome, st.PictoFoundCount)
	}
}

func TestRafalePicto318_StaleEpoch_Refused_NoEffect(t *testing.T) {
	r := newRafalePictoRig(t, wordsTC(0, 0, false, "a", "b", "c"), "10")
	old := *r.epoch()
	if _, err := r.e.PictoFoundAt(&old); err != nil {
		t.Fatal(err)
	}
	_, err := r.e.PictoFoundAt(&old) // admin ‖ anim : le 2e geste porte l'ancienne époque
	wantPictoErr(t, err, PictoErrStaleWord)
	_, err = r.e.PictoPass(&old)
	wantPictoErr(t, err, PictoErrStaleWord)
	_, err = r.e.PictoFoundAt(nil) // EPOCH obligatoire en Rafale Picto
	wantPictoErr(t, err, PictoErrStaleWord)
	if st := r.st(); st.PictoFoundCount != 1 || st.PictoPassCount != 0 {
		t.Errorf("les gestes refusés n'ont aucun effet : found=%d pass=%d", st.PictoFoundCount, st.PictoPassCount)
	}
}

func TestRafalePicto318_Pass_NotRafale_OnPictoOneWord_AndWrongPhase(t *testing.T) {
	r := newPictoRig(t, "chat")
	_, err := r.e.PictoPass(nil)
	wantPictoErr(t, err, PictoErrNotRafale)

	rr := newRafalePictoRig(t, wordsTC(0, 0, false, "a", "b"), "10")
	rr.e.Pause()
	_, err = rr.e.PictoPass(rr.epoch())
	wantPictoErr(t, err, PictoErrWrongPhase) // la passe n'a lieu qu'en STARTED
	if _, err := rr.e.PictoFoundAt(rr.epoch()); err != nil { // « Trouvé » reste permis en PAUSED
		t.Errorf("trouvé en PAUSED : %v", err)
	}
}

func TestRafalePicto318_Gong_FoundCountsOnce_ThenRevealed_NoRevealedWord(t *testing.T) {
	r := newRafalePictoRig(t, wordsTC(0, 0, false, "a", "b", "c"), "10")
	r.e.mu.Lock()
	r.e.state.PictoOutcome = PictoOutcomeTimeout // le chrono vient de sonner
	r.e.stopUnsafe()
	r.e.mu.Unlock()
	res, err := r.e.PictoFoundAt(r.epoch())
	if err != nil || res.Phase != PhaseRevealed {
		t.Fatalf("trouvé sur le gong : %+v %v", res, err)
	}
	st := r.st()
	if st.PictoFoundCount != 1 || st.PictoRevealedWord != "" || st.PictoOutcome != PictoOutcomeTimeout {
		t.Errorf("found=%d revealed=%q outcome=%q (compté une fois, rien à révéler, issue TIMEOUT)", st.PictoFoundCount, st.PictoRevealedWord, st.PictoOutcome)
	}
	_, err = r.e.PictoFoundAt(r.epoch())
	wantPictoErr(t, err, PictoErrWrongPhase)
}

func TestRafalePicto318_Reveal_ShowsCurrentUnfoundWord_Relaunch_ResetsCounters(t *testing.T) {
	r := newRafalePictoRig(t, wordsTC(0, 0, false, "a", "b", "c"), "10")
	if _, err := r.e.PictoFoundAt(r.epoch()); err != nil {
		t.Fatal(err)
	}
	cur, _ := r.e.PictoWord()
	r.e.Stop()
	r.e.Reveal()
	if st := r.st(); st.PictoRevealedWord != cur.Word || st.PictoFoundCount != 1 {
		t.Errorf("révélé=%q (attendu le mot en cours %q), trouvés=%d", st.PictoRevealedWord, cur.Word, st.PictoFoundCount)
	}
	r.e.Ready("rp", r.e.GetState().Question) // relance de la même question
	if st := r.st(); st.PictoFoundCount != 0 || len(st.PictoFoundWords) != 0 || st.PictoPassCount != 0 || st.PictoRoundPoints != 0 {
		t.Errorf("relance : compteurs remis à zéro, got found=%d words=%v pass=%d pts=%d", st.PictoFoundCount, st.PictoFoundWords, st.PictoPassCount, st.PictoRoundPoints)
	}
}

func TestRafalePicto318_Reservoir_EmptyPoolAtStart_NoWord(t *testing.T) {
	r := newRafalePictoRig(t, TypedContent{RafalePictoSource: PictoSourceReservoir, RafalePictoCategories: []string{"HISTORY"}, RafalePictoDifficulties: []int{1}}, "10")
	// le pool est vide : START → NO_WORD (filet), jamais de panique
	if r.st().PictoOutcome != PictoOutcomeNoWord {
		t.Errorf("pool vide au START : outcome=%q (attendu NO_WORD)", r.st().PictoOutcome)
	}
}
