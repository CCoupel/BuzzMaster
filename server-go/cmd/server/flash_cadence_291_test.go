package main

import (
	"testing"
	"time"

	"buzzcontrol/internal/audio"
)

// #291 Phase A (revue, M2/m3) : les cadences et délais de PRODUCTION ne sont plus exercés par les tests de
// durée (ils injectent des valeurs courtes) ; ces tests, sans aucune attente, les verrouillent.

func TestFlashCadenceConstants_291(t *testing.T) {
	t.Parallel()
	if scoreFlashGoldPhase != 250*time.Millisecond || scoreFlashTeamPhase != 500*time.Millisecond {
		t.Errorf("clignotement SCORE = %s or / %s équipe, attendu 250 ms / 500 ms (décision 2026-09-08, jamais 400/400)", scoreFlashGoldPhase, scoreFlashTeamPhase)
	}
	if lightingFlashOnPhase != 400*time.Millisecond || lightingFlashOffPhase != 400*time.Millisecond {
		t.Errorf("Flash général = %s / %s, attendu 400 ms / 400 ms", lightingFlashOnPhase, lightingFlashOffPhase)
	}
}

func TestFlashPhaseOverride_ZeroValueIsProduction_291(t *testing.T) {
	t.Parallel()
	var zero flashPhaseOverride
	if orDefault(zero.lightingOn, lightingFlashOnPhase) != lightingFlashOnPhase ||
		orDefault(zero.lightingOff, lightingFlashOffPhase) != lightingFlashOffPhase ||
		orDefault(zero.scoreGold, scoreFlashGoldPhase) != scoreFlashGoldPhase ||
		orDefault(zero.scoreTeam, scoreFlashTeamPhase) != scoreFlashTeamPhase {
		t.Error("un flashPhaseOverride nul doit rendre les cadences de production")
	}
	if got := orDefault(10*time.Millisecond, time.Second); got != 10*time.Millisecond {
		t.Errorf("une valeur injectée l'emporte : %s", got)
	}
}

func TestWatchdogPeriod_DefaultIsMaxSoundDurationPlusGrace_291(t *testing.T) {
	t.Parallel()
	qs := &questionSoundAdapter{}
	if got, want := qs.watchdogPeriod(), audio.MaxQuestionSoundDuration+2*time.Second; got != want || got != 32*time.Second {
		t.Errorf("période par défaut = %s, attendu %s (32 s)", got, want)
	}
	qs.watchdogDelay = 300 * time.Millisecond
	if got := qs.watchdogPeriod(); got != 300*time.Millisecond {
		t.Errorf("délai injecté = %s", got)
	}
}
