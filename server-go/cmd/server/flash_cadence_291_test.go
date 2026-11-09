package main

import (
	"testing"
	"time"

	"buzzcontrol/internal/audio"
)

// #291 Phase A (revue, M2/m3) : les cadences et délais de PRODUCTION ne sont plus exercés par les tests de
// durée (ils injectent des valeurs courtes) ; ces tests, sans aucune attente, les verrouillent.

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
