package main

// #267 — review LOT-3 (mineur 2) : NETWORK.CHECKED_AT est l'instant du DERNIER CHANGEMENT d'état
// (perte, retour, nouvelles adresses), pas du dernier passage du watchdog : l'écran admin affiche
// « Perte détectée il y a N s » à partir de lui. Un passage sans changement ne le déplace pas.
//
// Aucune attente : l'horloge de détection est injectée (server.SetNetworkClock) et avance d'une
// seconde à chaque lecture — deux passages n'ont donc jamais le même horodatage.

import (
	"buzzcontrol/internal/server"
	"sync/atomic"
	"testing"
	"time"
)

// tickingClock267 : chaque lecture de l'horloge avance d'une seconde (epoch ms strictement croissants).
func tickingClock267(t *testing.T) {
	t.Helper()
	var n atomic.Int64
	base := time.UnixMilli(1760345000000)
	restore := server.SetNetworkClock(func() time.Time {
		return base.Add(time.Duration(n.Add(1)) * time.Second)
	})
	t.Cleanup(restore)
}

func TestNetwork267_CheckedAt_StaysAtTheLastChangeAcrossQuietPasses(t *testing.T) {
	tickingClock267(t)
	e := newEnv267(t, netOK267)
	e.pass() // démarrage : première détection
	first := e.app.engine.GetNetworkState()
	if first.CheckedAt <= 0 {
		t.Fatalf("la première détection renseigne CHECKED_AT : %+v", first)
	}

	for i := 0; i < 3; i++ {
		e.pass()
	}
	if got := e.app.engine.GetNetworkState().CheckedAt; got != first.CheckedAt {
		t.Errorf("passages sans changement : CHECKED_AT doit rester %d, obtenu %d", first.CheckedAt, got)
	}

	// Une interface ignorée qui change seule met à jour la liste mais pas l'instant du changement.
	e.setNet(append([]server.NetInterface{ni267("docker0", "172.17.0.1")}, netOK267...))
	e.pass()
	st := e.app.engine.GetNetworkState()
	if st.CheckedAt != first.CheckedAt {
		t.Errorf("IGNORED_INTERFACES seul n'est pas un changement : CHECKED_AT %d → %d", first.CheckedAt, st.CheckedAt)
	}
	found := false
	for _, i := range st.IgnoredInterfaces {
		if i.Name == "docker0" {
			found = true
		}
	}
	if !found {
		t.Errorf("la liste des interfaces ignorées doit rester à jour : %+v", st.IgnoredInterfaces)
	}
}

func TestNetwork267_CheckedAt_MovesOnLossReturnAndAddressChange(t *testing.T) {
	tickingClock267(t)
	e := newEnv267(t, netOK267)
	e.pass()

	steps := []struct {
		name string
		net  []server.NetInterface
	}{
		{"perte", netLost267},
		{"retour", netOK267},
		{"adresse retenue différente", netNew267},
	}
	prevStamp := e.app.engine.GetNetworkState().CheckedAt
	for _, s := range steps {
		e.setNet(s.net)
		e.pass()
		stamp := e.app.engine.GetNetworkState().CheckedAt
		if stamp <= prevStamp {
			t.Errorf("%s : CHECKED_AT doit avancer à l'instant du changement (> %d), obtenu %d", s.name, prevStamp, stamp)
		}
		prevStamp = stamp
		// Les passages calmes qui suivent ne le déplacent pas.
		e.pass()
		e.pass()
		if got := e.app.engine.GetNetworkState().CheckedAt; got != stamp {
			t.Errorf("%s : passages calmes, CHECKED_AT %d → %d", s.name, stamp, got)
		}
	}
}
