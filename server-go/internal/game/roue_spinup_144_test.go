// #144 (v12.0.0) — montée en 2 tours et arrêt seulement à pleine vitesse (contracts/roue.md §4, §5, §5.1, §6, §8).
// Le harnais de roue_144_test.go raccourcit la montée (SpinUpMs = 40 ms) et la durée maximale (60 ms) ; les tests
// qui ont besoin d'une montée plus longue la fixent eux-mêmes.
package game

import (
	"encoding/json"
	"testing"
	"time"
)

func (r *roueRig) setUp(upMs, maxMs int) {
	r.e.mu.Lock()
	r.e.state.RoueWheel.SpinUpMs = upMs
	r.e.state.RoueWheel.SpinMaxMs = maxMs
	r.e.mu.Unlock()
}

func (r *roueRig) fullSpeed() bool { return r.st().RoueWheel.FullSpeed }

// ---- champs -------------------------------------------------------------------------------------------------

func TestRoueSpinUp144_SpinUpMsIs4TimesThePeriod_AndFullSpeedStartsFalse(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	if r.initialPeriod <= 0 || r.initialSpinUp != 4*r.initialPeriod {
		t.Errorf("SPIN_UP_MS doit valoir 4 × SPIN_PERIOD_MS au START : %d / %d", r.initialSpinUp, r.initialPeriod)
	}
	if r.fullSpeed() {
		t.Errorf("FULL_SPEED est false avant le lancer")
	}
}

func TestRoueSpinUp144_WheelJSON_AlwaysCarriesBothFields(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	raw, _ := json.Marshal(r.st().RoueWheel)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	if _, ok := m["SPIN_UP_MS"]; !ok {
		t.Errorf("SPIN_UP_MS doit toujours être sérialisé (pas d'omitempty)")
	}
	if v, ok := m["FULL_SPEED"]; !ok || v != false {
		t.Errorf("FULL_SPEED doit toujours être sérialisé, false hors montée terminée : %v", v)
	}
}

// ---- lancer : montée, puis pleine vitesse --------------------------------------------------------------------

func TestRoueSpinUp144_Spin_StartsTheClimb_NoMaxTimerYet(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.setUp(10000, 60) // durée maximale très courte : si elle était armée au lancer, la roue s'arrêterait
	r.mustOK(r.e.RoueSpin())
	if r.fullSpeed() || r.st().RoueSubPhase != RoueSubSpinning {
		t.Fatalf("après ROUE_SPIN : SPINNING, FULL_SPEED=false (montée) — %q %v", r.st().RoueSubPhase, r.fullSpeed())
	}
	time.Sleep(250 * time.Millisecond)
	if r.st().RoueSubPhase != RoueSubSpinning || r.draws.Load() != 0 {
		t.Errorf("la durée maximale n'est pas armée pendant la montée : %q tirages=%d", r.st().RoueSubPhase, r.draws.Load())
	}
}

func TestRoueSpinUp144_EndOfClimb_FullSpeed_BroadcastsAnUpdate_ThenMaxIsArmed(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.setUp(80, 120)
	r.idx.Store(wIdx90)
	start := time.Now()
	r.mustOK(r.e.RoueSpin())
	seq := r.st().RoueLastEvent.Seq
	before := r.changes.Load()
	r.waitClimbOver()
	if r.changes.Load() <= before {
		t.Errorf("la fin de la montée diffuse un UPDATE (OnRoueChange)")
	}
	if r.st().RoueLastEvent.Seq != seq {
		t.Errorf("la fin de la montée n'émet aucun événement : SEQ %d → %d", seq, r.st().RoueLastEvent.Seq)
	}
	if r.st().RoueSubPhase != RoueSubSpinning {
		t.Errorf("à pleine vitesse la roue tourne encore : %q", r.st().RoueSubPhase)
	}
	r.waitSub(RoueSubStopping)
	if el := time.Since(start); el < 185*time.Millisecond {
		t.Errorf("arrêt automatique à SPIN_UP_MS + SPIN_MAX_MS (80 + 120 ms) après le lancer, obtenu %v", el)
	}
	if r.st().RoueWheel.Stop == nil || r.st().RoueWheel.Stop.To != wIdx90 {
		t.Errorf("l'arrêt automatique tire la case : %+v", r.st().RoueWheel.Stop)
	}
	if r.fullSpeed() {
		t.Errorf("FULL_SPEED repasse à false dès l'arrêt")
	}
}

// ---- appui joueur ---------------------------------------------------------------------------------------------

func TestRoueSpinUp144_Buzz_DuringClimbIsIgnoredAndNotRemembered(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.setUp(250, 10000)
	r.mustOK(r.e.RoueSpin())
	seq := r.st().RoueLastEvent.Seq
	before := r.changes.Load()
	if r.e.ProcessButtonPress("b-red", time.Now().UnixMicro(), "A") {
		t.Errorf("un appui pendant la montée n'est pas un buzz accepté")
	}
	if r.st().RoueSubPhase != RoueSubSpinning || r.draws.Load() != 0 || r.st().RoueLastEvent.Seq != seq || r.changes.Load() != before {
		t.Errorf("appui pendant la montée : état, SEQ et diffusion inchangés (%q tirages=%d)", r.st().RoueSubPhase, r.draws.Load())
	}
	r.waitClimbOver()
	time.Sleep(60 * time.Millisecond)
	if r.st().RoueSubPhase != RoueSubSpinning || r.draws.Load() != 0 {
		t.Errorf("l'appui trop tôt n'est pas mémorisé : la roue tourne toujours (%q)", r.st().RoueSubPhase)
	}
	r.e.ProcessButtonPress("b-red", time.Now().UnixMicro(), "A")
	if r.st().RoueSubPhase != RoueSubStopping || r.draws.Load() != 1 {
		t.Errorf("à pleine vitesse l'appui est accepté : %q tirages=%d", r.st().RoueSubPhase, r.draws.Load())
	}
}

func TestRoueSpinUp144_Buzz_AnotherTeamIgnoredEvenAtFullSpeed(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, "", "red", "blue")
	r.setUp(40, 10000)
	r.mustOK(r.e.RoueSpin())
	r.waitClimbOver()
	r.e.ProcessButtonPress("b-blue", time.Now().UnixMicro(), "A")
	if r.st().RoueSubPhase != RoueSubSpinning || r.draws.Load() != 0 {
		t.Errorf("l'appui d'une autre équipe reste ignoré à pleine vitesse")
	}
}

// ---- ROUE_STOP selon l'origine ---------------------------------------------------------------------------------

func TestRoueSpinUp144_Stop_AnimRefusedDuringClimb_AcceptedAtFullSpeed(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.setUp(200, 10000)
	r.mustOK(r.e.RoueSpin())
	seq := r.st().RoueLastEvent.Seq
	r.wantErr(r.e.RoueStopFrom(RoueStopByAnim), RoueErrNotFullSpeed)
	if r.st().RoueSubPhase != RoueSubSpinning || r.draws.Load() != 0 || r.st().RoueLastEvent.Seq != seq {
		t.Errorf("le refus NOT_FULL_SPEED laisse l'état inchangé")
	}
	r.waitClimbOver()
	r.mustOK(r.e.RoueStopFrom(RoueStopByAnim))
	if r.st().RoueSubPhase != RoueSubStopping {
		t.Errorf("animateur à pleine vitesse : accepté (%q)", r.st().RoueSubPhase)
	}
}

func TestRoueSpinUp144_Stop_AdminAcceptedDuringClimb_CancelsTheClimbTimer(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.setUp(120, 10000)
	r.idx.Store(wIdx90)
	r.mustOK(r.e.RoueSpin())
	r.mustOK(r.e.RoueStop()) // admin : « Forcer l'arrêt » à tout moment de SPINNING
	s := r.st()
	if s.RoueSubPhase != RoueSubStopping || s.RoueWheel.Stop == nil || s.RoueWheel.Stop.To != wIdx90 || s.RoueWheel.FullSpeed {
		t.Fatalf("arrêt forcé pendant la montée : STOPPING, STOP posé, FULL_SPEED=false — %q %+v %v", s.RoueSubPhase, s.RoueWheel.Stop, s.RoueWheel.FullSpeed)
	}
	r.waitSub(RoueSubLetter)
	time.Sleep(250 * time.Millisecond) // bien après l'échéance de montée
	if r.fullSpeed() || r.st().RoueSubPhase != RoueSubLetter || r.draws.Load() != 1 {
		t.Errorf("l'échéance de montée annulée ne doit plus agir : %q FULL_SPEED=%v tirages=%d", r.st().RoueSubPhase, r.fullSpeed(), r.draws.Load())
	}
}

// ---- échéances périmées ----------------------------------------------------------------------------------------

func TestRoueSpinUp144_StaleClimbDeadline_IgnoredAfterReveal(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.setUp(100, 10000)
	r.mustOK(r.e.RoueSpin())
	r.mustOK(r.e.RoueReveal())
	time.Sleep(250 * time.Millisecond)
	if s := r.st(); s.RoueSubPhase != RoueSubSolved || s.RoueWheel.FullSpeed {
		t.Errorf("après ROUE_REVEAL, la montée ne doit plus agir : %q FULL_SPEED=%v", s.RoueSubPhase, s.RoueWheel.FullSpeed)
	}
}

func TestRoueSpinUp144_StaleClimbDeadline_IgnoredAfterStop(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.setUp(100, 10000)
	r.mustOK(r.e.RoueSpin())
	r.e.Stop()
	time.Sleep(250 * time.Millisecond)
	if s := r.st(); s.RoueSubPhase != "" || s.RoueWheel.FullSpeed {
		t.Errorf("après STOP, la montée ne doit plus agir : %q FULL_SPEED=%v", s.RoueSubPhase, s.RoueWheel.FullSpeed)
	}
}

func TestRoueSpinUp144_StaleClimbDeadline_IgnoredAfterNewGame(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.setUp(100, 10000)
	r.mustOK(r.e.RoueSpin())
	r.e.InitGame()
	time.Sleep(250 * time.Millisecond)
	if s := r.st(); s.RoueSubPhase != "" || s.RoueWheel.FullSpeed {
		t.Errorf("après NEW_GAME, la montée ne doit plus agir : %q FULL_SPEED=%v", s.RoueSubPhase, s.RoueWheel.FullSpeed)
	}
}

// ---- pause pendant la montée -----------------------------------------------------------------------------------

func TestRoueSpinUp144_Pause_FreezesTheClimbTimer_RemainingDurationOnResume(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.setUp(200, 10000)
	r.mustOK(r.e.RoueSpin())
	time.Sleep(60 * time.Millisecond)
	r.e.Pause()
	time.Sleep(350 * time.Millisecond) // bien plus que la montée
	if r.fullSpeed() {
		t.Fatalf("en pause la montée ne se termine pas : FULL_SPEED doit rester false")
	}
	resumed := time.Now()
	r.e.Continue()
	r.waitClimbOver()
	if el := time.Since(resumed); el < 90*time.Millisecond {
		t.Errorf("la montée reprend pour sa durée restante (~140 ms), pas pour zéro : %v", el)
	}
}

// ---- FULL_SPEED seulement en SPINNING après la montée ------------------------------------------------------------

func TestRoueSpinUp144_FullSpeed_TrueOnlyInSpinningAfterTheClimb(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	if r.fullSpeed() {
		t.Errorf("TURN : FULL_SPEED=false")
	}
	r.setUp(40, 10000)
	r.mustOK(r.e.RoueSpin())
	if r.fullSpeed() {
		t.Errorf("SPINNING pendant la montée : FULL_SPEED=false")
	}
	r.waitClimbOver()
	if r.st().RoueSubPhase != RoueSubSpinning {
		t.Fatalf("FULL_SPEED=true seulement en SPINNING : %q", r.st().RoueSubPhase)
	}
	r.mustOK(r.e.RoueStop())
	for _, step := range []func(){
		func() {},
		func() { r.waitSub(RoueSubLetter) },
		func() { r.mustOK(r.e.RoueLetter("N")) },
		func() { r.mustOK(r.e.RoueFlip()); r.waitSub(RoueSubAnswer) },
		func() { r.mustOK(r.e.RoueReveal()) },
	} {
		step()
		if r.fullSpeed() {
			t.Errorf("hors SPINNING (%q) : FULL_SPEED doit être false", r.st().RoueSubPhase)
		}
	}
}

func TestRoueSpinUp144_FullSpeed_FalseAgainAfterAnAutomaticStop_AndNextSpinClimbsAgain(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.setUp(40, 60)
	r.idx.Store(wIdx100)
	r.mustOK(r.e.RoueSpin())
	r.waitSub(RoueSubStopping)
	r.waitSub(RoueSubLetter)
	r.mustOK(r.e.RoueLetter("Z")) // absente → fin du tour
	r.waitSub(RoueSubTurnEnd)
	r.mustOK(r.e.RoueNext("SAME"))
	r.setUp(200, 10000)
	r.mustOK(r.e.RoueSpin())
	if r.fullSpeed() {
		t.Errorf("un nouveau lancer repart de la montée : FULL_SPEED=false")
	}
}

// waitClimbOver attend FULL_SPEED=true (fin de la montée, diffusée par le serveur).
func (r *roueRig) waitClimbOver() {
	r.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if r.st().RoueWheel.FullSpeed {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	r.t.Fatalf("pleine vitesse non atteinte (sous-phase %q)", r.st().RoueSubPhase)
}
