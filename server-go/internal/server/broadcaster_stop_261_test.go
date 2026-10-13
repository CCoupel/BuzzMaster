package server

import (
	"testing"
	"time"
)

// #261 — à l'arrêt par /shutdown, main.stop() est appelé deux fois (hook
// OnShutdown puis fin de main) : BroadcasterManager.Stop() faisait
// close(b.stopCh) deux fois → « panic: close of closed channel ».
//
// Ces tests vérifient le COMPORTEMENT (un double arrêt ne panique pas et ne
// bloque pas), pas le mécanisme choisi pour le garantir.

// stopTwiceWithin appelle stop deux fois et échoue (au lieu de planter tout le
// paquet) si l'un des appels panique ou ne rend pas la main avant l'échéance.
func stopTwiceWithin(t *testing.T, name string, stop func()) {
	t.Helper()
	done := make(chan interface{}, 1)
	go func() {
		defer func() { done <- recover() }()
		stop()
		stop()
	}()
	select {
	case r := <-done:
		if r != nil {
			t.Fatalf("%s : le double arrêt a paniqué : %v", name, r)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("%s : le double arrêt ne rend pas la main (échéance 10 s)", name)
	}
}

// Arrêt double d'un broadcaster démarré (cas réel : /shutdown puis fin de main).
func TestBroadcasterManager_Stop_Twice_NoPanic_261(t *testing.T) {
	udp := NewUDPBroadcaster()
	bm := NewBroadcasterManager(udp, 80)
	bm.Start()
	stopTwiceWithin(t, "BroadcasterManager.Stop (démarré)", bm.Stop)
}

// Arrêt double d'un broadcaster jamais démarré.
func TestBroadcasterManager_Stop_Twice_NeverStarted_NoPanic_261(t *testing.T) {
	bm := NewBroadcasterManager(NewUDPBroadcaster(), 80)
	stopTwiceWithin(t, "BroadcasterManager.Stop (jamais démarré)", bm.Stop)
}
