package game

import (
	"testing"
	"time"
)

// Montée d'un tour et arrêt seulement à pleine vitesse (contrat roue.md §4, §5.1) — tests moteur du lot serveur.

func bkClimbRig(t *testing.T, upMs, maxMs int) *roueRig {
	t.Helper()
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, "", "red", "blue")
	r.e.mu.Lock()
	r.e.state.RoueWheel.SpinUpMs = upMs
	r.e.state.RoueWheel.SpinMaxMs = maxMs
	r.e.mu.Unlock()
	return r
}

func (r *roueRig) bkTimerKind() string {
	r.e.mu.Lock()
	defer r.e.mu.Unlock()
	if r.e.roue == nil {
		return ""
	}
	return r.e.roue.timerKind
}

func (r *roueRig) bkWaitFullSpeed(want bool) {
	r.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if r.st().RoueWheel.FullSpeed == want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	r.t.Fatalf("FULL_SPEED=%v attendu, état %+v", want, r.st().RoueWheel)
}

func TestRoue144Montee_StartPoseSpinUpMs(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, "", "red", "blue")
	r.e.Stop()
	r.e.SetRoueWheelParams(DefaultRoueWheelParams()) // période 2 500 ms
	r.e.Ready("rq", &Question{ID: "rq", Question: "R", Type: QuestionTypeRoue,
		TypedContent: TypedContent{RouePhrase: roueTestPhrase, RoueMode: RoueModeChacunSonTour}})
	r.mustOK(r.e.RoueSetTeams([]string{"red", "blue"}))
	r.e.StartImmediate(0)
	t.Cleanup(func() { r.e.Stop() })
	if w := r.st().RoueWheel; w.SpinUpMs != 5000 || w.FullSpeed {
		t.Errorf("SPIN_UP_MS = 2 × SPIN_PERIOD_MS (5000) et FULL_SPEED=false au START : %+v", w)
	}
}

func TestRoue144Montee_SpinThenFullSpeedThenAutoStop(t *testing.T) {
	r := bkClimbRig(t, 80, 60)
	r.mustOK(r.e.RoueSpin())
	if w := r.st().RoueWheel; w.FullSpeed || r.st().RoueSubPhase != RoueSubSpinning || w.Stop != nil {
		t.Fatalf("après ROUE_SPIN : SPINNING, FULL_SPEED=false, STOP=null : %+v", w)
	}
	if k := r.bkTimerKind(); k != roueTimerUp {
		t.Fatalf("la minuterie de montée est armée, pas la durée maximale : %q", k)
	}
	changes := r.changes.Load()
	r.bkWaitFullSpeed(true)
	if r.changes.Load() <= changes {
		t.Errorf("la fin de la montée diffuse un UPDATE (OnRoueChange)")
	}
	// La durée maximale est armée dans la même section : l'arrêt automatique suit.
	r.waitSub(RoueSubStopping)
	if r.st().RoueWheel.FullSpeed || r.st().RoueWheel.Stop == nil {
		t.Errorf("STOPPING : FULL_SPEED=false, STOP renseigné : %+v", r.st().RoueWheel)
	}
}

func TestRoue144Montee_PressIgnoredDuringClimbAcceptedAtFullSpeed(t *testing.T) {
	r := bkClimbRig(t, 120, 5000)
	r.mustOK(r.e.RoueSpin())
	seq, changes, draws := r.st().RoueLastEvent.Seq, r.changes.Load(), r.draws.Load()
	if r.e.ProcessButtonPress("b-red", time.Now().UnixMicro(), "A") {
		t.Errorf("un appui ROUE n'est jamais consigné")
	}
	s := r.st()
	if s.RoueSubPhase != RoueSubSpinning || s.RoueLastEvent.Seq != seq || r.changes.Load() != changes || r.draws.Load() != draws {
		t.Fatalf("appui pendant la montée : ignoré (sous-phase, SEQ, diffusion, tirage inchangés) : %+v", s.RoueSubPhase)
	}
	r.bkWaitFullSpeed(true)
	r.e.ProcessButtonPress("b-red", time.Now().UnixMicro(), "A") // non mémorisé : il faut appuyer de nouveau
	if r.st().RoueSubPhase != RoueSubStopping {
		t.Errorf("appui à pleine vitesse : la roue s'arrête, got %s", r.st().RoueSubPhase)
	}
}

func TestRoue144Montee_StopRefusedDuringClimbWhoeverSends(t *testing.T) {
	r := bkClimbRig(t, 5000, 5000)
	r.mustOK(r.e.RoueSpin())
	before := r.st()
	changes, draws := r.changes.Load(), r.draws.Load()
	r.wantErr(r.e.RoueStop(), RoueErrNotFullSpeed) // admin comme animateur : un seul chemin, aucune exception
	if s := r.st(); s.RoueSubPhase != RoueSubSpinning || s.RoueWheel.Stop != nil || s.RoueLastEvent != before.RoueLastEvent ||
		r.changes.Load() != changes || r.draws.Load() != draws {
		t.Errorf("refus : état, événement, diffusion et tirage inchangés")
	}
	if k := r.bkTimerKind(); k != roueTimerUp {
		t.Errorf("la minuterie de montée est inchangée après un refus : %q", k)
	}
}

func TestRoue144Montee_StopAcceptedAtFullSpeed(t *testing.T) {
	r := bkClimbRig(t, 40, 5000)
	r.mustOK(r.e.RoueSpin())
	r.bkWaitFullSpeed(true)
	r.mustOK(r.e.RoueStop())
	if r.st().RoueSubPhase != RoueSubStopping {
		t.Errorf("à pleine vitesse : ROUE_STOP accepté")
	}
}

func TestRoue144Montee_PauseDuringClimbKeepsRemainingTime(t *testing.T) {
	r := bkClimbRig(t, 150, 5000)
	r.mustOK(r.e.RoueSpin())
	r.e.Pause()
	r.e.mu.Lock()
	kind, rem, running := r.e.roue.timerKind, r.e.roue.remaining, r.e.roue.running
	r.e.mu.Unlock()
	if kind != roueTimerUp || running || rem <= 0 || rem > 150*time.Millisecond {
		t.Fatalf("pause : montée suspendue avec sa durée restante (kind=%q running=%v rem=%v)", kind, running, rem)
	}
	time.Sleep(250 * time.Millisecond) // plus que la durée de montée : rien ne se passe en pause
	if r.st().RoueWheel.FullSpeed {
		t.Fatalf("FULL_SPEED reste false en pause")
	}
	r.e.Continue()
	r.bkWaitFullSpeed(true)
}

func TestRoue144Montee_StaleExpiryIgnoredAfterStopOrReveal(t *testing.T) {
	r := bkClimbRig(t, 60, 5000)
	r.mustOK(r.e.RoueSpin())
	r.e.Stop() // STOP : minuterie annulée, état ROUE remis à zéro
	time.Sleep(150 * time.Millisecond)
	if w := r.st().RoueWheel; w.FullSpeed {
		t.Errorf("une échéance périmée ne passe pas FULL_SPEED à true après STOP : %+v", w)
	}

	r2 := bkClimbRig(t, 60, 5000)
	r2.mustOK(r2.e.RoueSpin())
	r2.mustOK(r2.e.RoueReveal())
	time.Sleep(150 * time.Millisecond)
	if s := r2.st(); s.RoueWheel.FullSpeed || s.RoueSubPhase != RoueSubSolved {
		t.Errorf("ROUE_REVEAL annule la montée : %s FULL_SPEED=%v", s.RoueSubPhase, s.RoueWheel.FullSpeed)
	}
}

func TestRoue144Montee_FullSpeedFalseOutsideSpinning(t *testing.T) {
	r := bkClimbRig(t, 20, 5000)
	if r.st().RoueWheel.FullSpeed {
		t.Fatalf("TURN : FULL_SPEED=false")
	}
	r.mustOK(r.e.RoueSpin())
	r.bkWaitFullSpeed(true)
	r.mustOK(r.e.RoueStop())
	if r.st().RoueWheel.FullSpeed {
		t.Errorf("STOPPING : FULL_SPEED=false")
	}
	r.waitSub(afterLanding(r, int(r.idx.Load())))
	if r.st().RoueWheel.FullSpeed {
		t.Errorf("après l'arrêt : FULL_SPEED=false")
	}
}
