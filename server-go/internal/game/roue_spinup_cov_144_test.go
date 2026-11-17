package game

import (
	"testing"
	"time"
)

// #144 (v12.0.0) — couverture complémentaire (revue montée) : échéance de montée en vol pendant une pause.

// L'échéance `up` déjà partie (goroutine du timer en vol) quand la pause survient est périmée (génération invalidée) :
// elle ne doit pas passer FULL_SPEED à true ; la montée reprend à la reprise pour sa durée restante.
func TestRoueSpinUp144_Pause_InFlightClimbDeadlineIsIgnored_ThenResumeFinishesTheClimb(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.setUp(300, 10000)
	r.mustOK(r.e.RoueSpin())
	time.Sleep(40 * time.Millisecond)
	r.e.mu.Lock()
	gen, kind := r.e.roue.gen, r.e.roue.timerKind
	r.e.mu.Unlock()
	if kind != roueTimerUp {
		t.Fatalf("pendant la montée la minuterie armée est `up`, obtenu %q", kind)
	}
	r.e.Pause()
	r.e.roueTimerFired(gen, kind) // l'échéance « en vol » arrive après la pause
	if r.fullSpeed() {
		t.Fatalf("une échéance de montée périmée (pause) ne doit pas donner FULL_SPEED")
	}
	time.Sleep(400 * time.Millisecond) // plus que la montée : toujours suspendue
	if r.fullSpeed() {
		t.Fatalf("en pause la montée ne se termine pas")
	}
	resumed := time.Now()
	r.e.Continue()
	r.waitClimbOver()
	if el := time.Since(resumed); el < 150*time.Millisecond {
		t.Errorf("la montée reprend pour sa durée restante (~260 ms), pas pour zéro : %v", el)
	}
	if r.st().RoueSubPhase != RoueSubSpinning {
		t.Errorf("la roue tourne à pleine vitesse après la reprise : %q", r.st().RoueSubPhase)
	}
}
