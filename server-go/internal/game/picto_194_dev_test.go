package game

import (
	"sync"
	"testing"
	"time"
)

// #194 (v13.0.0, LOT-2.3) — fin de manche, révélation, NO_WORD, relance (contracts/picto.md §6.2 à §6.6).

func startedPictoRig(t *testing.T, words ...string) *pictoRig {
	t.Helper()
	r := newPictoRig(t, words...)
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if err := r.e.PictoSetDrawer("v-alice"); err != nil {
		t.Fatal(err)
	}
	r.e.StartImmediate(60)
	if r.st().Phase != PhaseStarted || len(r.words) != 1 {
		t.Fatalf("manche non démarrée : phase=%s mots remis=%d", r.st().Phase, len(r.words))
	}
	return r
}

func TestPicto194_Found_StopsAndRevealsInOneAction(t *testing.T) {
	for _, pause := range []bool{false, true} {
		r := startedPictoRig(t, "chat", "chien", "pont-levis")
		word := r.words[0].Word
		if st := r.st(); st.PictoRevealedWord != "" || st.PictoOutcome != "" {
			t.Fatalf("le mot ne doit pas être public avant la révélation : %q %q", st.PictoRevealedWord, st.PictoOutcome)
		}
		if pause {
			r.e.PauseAll()
		}
		var phases []GamePhase
		r.e.OnStateChange = func(p GamePhase) { phases = append(phases, p) }
		if err := r.e.PictoFound(); err != nil {
			t.Fatalf("pause=%v : %v", pause, err)
		}
		st := r.st()
		if st.Phase != PhaseRevealed || st.PictoOutcome != PictoOutcomeFound || st.PictoRevealedWord != word || st.PictoRevealedCategory != "ANIMALS" {
			t.Errorf("pause=%v : phase=%s issue=%q mot=%q catégorie=%q", pause, st.Phase, st.PictoOutcome, st.PictoRevealedWord, st.PictoRevealedCategory)
		}
		if len(phases) != 1 || phases[0] != PhaseRevealed {
			t.Errorf("une seule notification REVEALED attendue : %v", phases)
		}
		// Un second « Trouvé » (admin et anim en même temps) est refusé : une seule issue.
		wantPictoErr(t, r.e.PictoFound(), PictoErrWrongPhase)
	}
}

func TestPicto194_Found_RefusedWhenNotInRound(t *testing.T) {
	r := newPictoRig(t)
	wantPictoErr(t, r.e.PictoFound(), PictoErrWrongPhase) // PREPARE
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	wantPictoErr(t, r.e.PictoFound(), PictoErrWrongPhase) // READY
}

func TestPicto194_Stop_IsAbandonment_ThenRevealShowsTheWord_FoundRefused(t *testing.T) {
	r := startedPictoRig(t, "chat")
	r.e.Stop()
	st := r.st()
	if st.Phase != PhaseStopped || st.PictoOutcome != PictoOutcomeStopped || st.PictoRevealedWord != "" {
		t.Fatalf("après STOP : phase=%s issue=%q mot=%q (le mot reste secret)", st.Phase, st.PictoOutcome, st.PictoRevealedWord)
	}
	wantPictoErr(t, r.e.PictoFound(), PictoErrWrongPhase) // refusé après STOP (§6.4)
	r.e.Reveal()
	st = r.st()
	if st.Phase != PhaseRevealed || st.PictoOutcome != PictoOutcomeStopped || st.PictoRevealedWord != "chat" || st.PictoRevealedCategory != "ANIMALS" {
		t.Errorf("après REVEAL : %+v", st)
	}
}

func TestPicto194_RevealFromPaused_IsAbandonment(t *testing.T) {
	r := startedPictoRig(t, "chat")
	r.e.PauseAll()
	r.e.Reveal()
	if st := r.st(); st.Phase != PhaseRevealed || st.PictoOutcome != PictoOutcomeStopped || st.PictoRevealedWord != "chat" {
		t.Errorf("REVEAL depuis PAUSED : %+v", st.PictoOutcome)
	}
}

func TestPicto194_Timeout_ThenFoundOnTheGong(t *testing.T) {
	r := newPictoRig(t, "chat")
	r.e.SetTickInterval(10 * time.Millisecond)
	if err := r.e.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if err := r.e.PictoSetDrawer("v-alice"); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var stopped bool
	r.e.OnStateChange = func(p GamePhase) {
		if p == PhaseStopped {
			mu.Lock()
			stopped = true
			mu.Unlock()
		}
	}
	r.e.StartImmediate(2) // 2 « secondes » de 10 ms
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		s := stopped
		mu.Unlock()
		if s {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	st := r.st()
	if st.Phase != PhaseStopped || st.PictoOutcome != PictoOutcomeTimeout || st.PictoRevealedWord != "" {
		t.Fatalf("fin du chrono : phase=%s issue=%q mot public=%q", st.Phase, st.PictoOutcome, st.PictoRevealedWord)
	}
	// I5 : « Trouvé » sur le gong, tant que le mot n'est pas révélé : TIMEOUT → FOUND.
	if err := r.e.PictoFound(); err != nil {
		t.Fatal(err)
	}
	if st := r.st(); st.Phase != PhaseRevealed || st.PictoOutcome != PictoOutcomeFound || st.PictoRevealedWord != "chat" {
		t.Errorf("après TIMEOUT puis FOUND : %+v", st.PictoOutcome)
	}
}

func TestPicto194_Timeout_ThenRevealKeepsTimeout(t *testing.T) {
	r := newPictoRig(t, "chat")
	r.e.SetTickInterval(10 * time.Millisecond)
	_ = r.e.PictoSetTeam("red")
	_ = r.e.PictoSetDrawer("v-alice")
	r.e.StartImmediate(1)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && r.st().Phase != PhaseStopped {
		time.Sleep(5 * time.Millisecond)
	}
	r.e.Reveal()
	if st := r.st(); st.PictoOutcome != PictoOutcomeTimeout || st.PictoRevealedWord != "chat" || st.Phase != PhaseRevealed {
		t.Errorf("TIMEOUT conservé jusqu'à la révélation : %+v", st.PictoOutcome)
	}
}

func TestPicto194_NoWord_StopsAtStart(t *testing.T) {
	r := newPictoRig(t, "chat")
	_ = r.e.PictoSetTeam("red")
	_ = r.e.PictoSetDrawer("v-alice")
	// Question modifiée à la main entre READY et START : plus aucun mot.
	r.e.mu.Lock()
	r.e.state.Question.PictoWords = nil
	r.e.mu.Unlock()
	var phases []GamePhase
	r.e.OnStateChange = func(p GamePhase) { phases = append(phases, p) }
	r.e.StartImmediate(60)
	st := r.st()
	if st.Phase != PhaseStopped || st.PictoOutcome != PictoOutcomeNoWord {
		t.Fatalf("NO_WORD : phase=%s issue=%q", st.Phase, st.PictoOutcome)
	}
	if len(r.words) != 0 {
		t.Error("aucun PICTO_WORD ne doit partir sans mot")
	}
	if len(phases) != 1 || phases[0] != PhaseStopped {
		t.Errorf("STOPPED annoncé, jamais STARTED : %v", phases)
	}
	r.e.Reveal()
	if st := r.st(); st.Phase != PhaseRevealed || st.PictoOutcome != PictoOutcomeNoWord || st.PictoRevealedWord != "" {
		t.Errorf("REVEAL après NO_WORD : mot révélé vide attendu, got %+v", st.PictoRevealedWord)
	}
	wantPictoErr(t, r.e.PictoFound(), PictoErrWrongPhase)
}

func TestPicto194_Relaunch_ClearsOutcomeKeepsPreparation_NewEpoch(t *testing.T) {
	r := startedPictoRig(t, "chat")
	r.e.Stop()
	r.e.Reveal()
	q := r.st().Question
	r.e.Ready("pq", q)
	st := r.st()
	if st.PictoOutcome != "" || st.PictoRevealedWord != "" || st.PictoRevealedCategory != "" {
		t.Errorf("relance : issue et mot révélé vidés : %q %q", st.PictoOutcome, st.PictoRevealedWord)
	}
	if st.PictoTeam != "red" || st.PictoDrawer != "v-alice" {
		t.Errorf("relance : équipe et dessinateur conservés : %q %q", st.PictoTeam, st.PictoDrawer)
	}
	if _, ok := r.e.PictoWord(); ok {
		t.Error("le mot secret est vidé à la relance")
	}
}

// Contrat §7 : le crédit de PICTO va à l'équipe seule — la cible par défaut d'une question sans POINTS_TARGET est TEAM.
func TestPicto194_DefaultPointsTargetIsTeam(t *testing.T) {
	if got := DefaultPointsTarget("PICTO"); got != PointsTargetTeam {
		t.Errorf("DefaultPointsTarget(PICTO) = %q, attendu TEAM", got)
	}
	if got := DefaultPointsTarget("SPEEDY"); got != PointsTargetPlayer { // non-régression
		t.Errorf("DefaultPointsTarget(SPEEDY) = %q, attendu PLAYER", got)
	}
}
