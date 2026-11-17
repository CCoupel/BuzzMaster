package main

import (
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
)

// #144 (v12.0.0) — montée en 2 tours par le vrai chemin WebSocket (contracts/roue.md §5.1, §8) : l'origine de
// ROUE_STOP (admin / animateur) est transmise au moteur, le refus NOT_FULL_SPEED va à l'émetteur seul, et la
// TV comme la tablette reçoivent SPIN_UP_MS et FULL_SPEED. La montée par défaut dure 10 s : on reste dedans.

func TestRoueWire144_SpinUp_StopRefusedDuringClimb_AdminAndAnim_ErrorToTheEmitterOnly(t *testing.T) {
	for _, who := range []string{"admin", "anim"} {
		t.Run(who, func(t *testing.T) {
			w := newRoueWire(t, game.RoueModeTantQueJeGagne)
			w.send(w.admin, protocol.ActionRoueSpin, struct{}{})
			for _, tp := range []*tap{w.adminTap, w.animTap, w.tvTap, w.vpTap} {
				tp.take(30 * time.Millisecond)
			}
			emitter, other := w.admin, w.anim
			emitterTap, otherTap := w.adminTap, w.animTap
			if who == "anim" {
				emitter, other = w.anim, w.admin
				emitterTap, otherTap = w.animTap, w.adminTap
			}
			_ = other
			w.send(emitter, protocol.ActionRoueStop, struct{}{})
			if code, ok := roueErrorOf(emitterTap.take(200 * time.Millisecond)); !ok || code != game.RoueErrNotFullSpeed {
				t.Errorf("%s doit recevoir ROUE_ERROR NOT_FULL_SPEED, got %q found=%v", who, code, ok)
			}
			if _, ok := roueErrorOf(otherTap.take(50 * time.Millisecond)); ok {
				t.Errorf("l'autre poste ne doit pas recevoir ce ROUE_ERROR")
			}
			for name, tp := range map[string]*tap{"tv": w.tvTap, "tablette": w.vpTap} {
				if _, ok := roueErrorOf(tp.take(50 * time.Millisecond)); ok {
					t.Errorf("%s ne reçoit jamais de ROUE_ERROR", name)
				}
			}
			s := w.app.engine.GetState()
			if s.RoueSubPhase != game.RoueSubSpinning || s.RoueWheel.Stop != nil || s.RoueWheel.FullSpeed {
				t.Errorf("le refus laisse l'état inchangé : %q stop=%+v full=%v", s.RoueSubPhase, s.RoueWheel.Stop, s.RoueWheel.FullSpeed)
			}
		})
	}
}

// La TV et la tablette reçoivent la montée : SPIN_UP_MS dès le départ, FULL_SPEED=false pendant la montée.
func TestRoueWire144_SpinUp_TVAndTabletReceiveSpinUpMsAndFullSpeedFalse(t *testing.T) {
	w := newRoueWire(t, game.RoueModeTantQueJeGagne)
	for _, tp := range []*tap{w.tvTap, w.vpTap} {
		tp.take(30 * time.Millisecond)
	}
	w.send(w.admin, protocol.ActionRoueSpin, struct{}{})
	for name, tp := range map[string]*tap{"tv": w.tvTap, "tablette": w.vpTap} {
		ok := false
		for _, f := range tp.take(250 * time.Millisecond) {
			if actionOf(f) == protocol.ActionUpdate && strings.Contains(f, `"SPIN_UP_MS":`) && strings.Contains(f, `"FULL_SPEED":false`) &&
				strings.Contains(f, `"ROUE_SUBPHASE":"SPINNING"`) {
				ok = true
			}
		}
		if !ok {
			t.Errorf("%s doit recevoir un UPDATE SPINNING avec SPIN_UP_MS et FULL_SPEED=false", name)
		}
	}
}
