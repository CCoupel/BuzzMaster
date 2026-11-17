package main

import (
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
)

// #144 (v12.0.0) — couverture complémentaire (revue montée) : appui tablette par le vrai chemin WebSocket.

func (w *roueWire) bindTabletTo(bumperID string) {
	w.t.Helper()
	w.app.wsHub.SetClientPlayerID(learnClientID(w.t, w.app, w.vp), bumperID)
}

func updatesOf(frames []string) int {
	n := 0
	for _, f := range frames {
		if actionOf(f) == protocol.ActionUpdate {
			n++
		}
	}
	return n
}

// Pendant la montée, l'appui de la tablette de l'équipe qui a la main est ignoré : ni UPDATE, ni SEQ, ni tirage.
func TestRoueWire144_SpinUp_TabletPressDuringClimb_NoUpdateNoSeq(t *testing.T) {
	w := newRoueWire(t, game.RoueModeTantQueJeGagne)
	w.bindTabletTo("b-red")
	w.send(w.admin, protocol.ActionRoueSpin, struct{}{}) // montée de 5 s par défaut : on reste dedans
	seq := w.app.engine.GetState().RoueLastEvent.Seq
	for _, tp := range []*tap{w.adminTap, w.animTap, w.tvTap, w.vpTap} {
		tp.take(50 * time.Millisecond)
	}
	w.send(w.vp, protocol.ActionButton, map[string]interface{}{"ID": "b-red", "button": "A"})
	for name, tp := range map[string]*tap{"admin": w.adminTap, "anim": w.animTap, "tv": w.tvTap, "tablette": w.vpTap} {
		if n := updatesOf(tp.take(300 * time.Millisecond)); n != 0 {
			t.Errorf("%s : un appui pendant la montée ne doit émettre aucun UPDATE (reçu %d)", name, n)
		}
	}
	s := w.app.engine.GetState()
	if s.RoueSubPhase != game.RoueSubSpinning || s.RoueWheel.Stop != nil || s.RoueLastEvent.Seq != seq || s.RoueWheel.FullSpeed {
		t.Errorf("appui ignoré : %q stop=%+v seq %d→%d", s.RoueSubPhase, s.RoueWheel.Stop, seq, s.RoueLastEvent.Seq)
	}
}

// Contrôle positif : à pleine vitesse le même appui arrête la roue et diffuse un UPDATE.
func TestRoueWire144_SpinUp_TabletPressAtFullSpeed_StopsTheWheel(t *testing.T) {
	w := newRoueWire(t, game.RoueModeTantQueJeGagne)
	w.bindTabletTo("b-red")
	w.app.engine.SetRoueSpinUpMs(30)
	w.send(w.admin, protocol.ActionRoueSpin, struct{}{})
	w.waitFullSpeed(5 * time.Second)
	w.tvTap.take(50 * time.Millisecond)
	w.send(w.vp, protocol.ActionButton, map[string]interface{}{"ID": "b-red", "button": "A"})
	w.waitSub(game.RoueSubStopping, 2*time.Second)
	if updatesOf(w.tvTap.take(200*time.Millisecond)) == 0 {
		t.Errorf("l'arrêt par la tablette diffuse un UPDATE à la TV")
	}
}
