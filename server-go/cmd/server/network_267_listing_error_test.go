package main

// #267 — review LOT-3 (mineur 1) : une erreur de LISTAGE des interfaces ne dit rien du réseau.
// updateNetworkState garde l'état précédent (aucune diffusion, aucun NETWORK_LOST/RESTORED) et
// journalise une ligne WARN dédiée NETWORK_LISTING_FAILED. Au tout premier passage, sans état
// précédent, le réseau est considéré présent (pas d'alerte) et le passage réussi suivant reste
// traité comme le premier (pas de NETWORK_IPS_CHANGED parasite).
//
// Attentes : jamais de fenêtre fixe — absence d'UPDATE par témoin FIFO (assertNoAction264).

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

// failableNet267 : par-dessus newEnv267, une source qui peut échouer à la demande.
func failableNet267(t *testing.T, e *env267) *atomic.Bool {
	t.Helper()
	fail := &atomic.Bool{}
	restore := server.SetListInterfaces(func() ([]server.NetInterface, error) {
		if fail.Load() {
			return nil, errors.New("listage impossible (test)")
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		return append([]server.NetInterface(nil), e.ifaces...), nil
	})
	t.Cleanup(restore)
	return fail
}

func listingFailures267(e *env267) int {
	n := 0
	for _, m := range e.logs(game.LogLevelWarn) {
		if strings.HasPrefix(m, "NETWORK_LISTING_FAILED") {
			n++
		}
	}
	return n
}

// networkEvents267 : les événements NETWORK_LOST / RESTORED / IPS_CHANGED du journal (pas le WARN de listage).
func networkEvents267(e *env267) []string {
	var out []string
	for _, m := range e.networkLines() {
		if !strings.HasPrefix(m, "NETWORK_LISTING_FAILED") {
			out = append(out, m)
		}
	}
	return out
}

func TestNetwork267_ListingError_AfterNetworkPresent_KeepsStateWithoutFalseAlert(t *testing.T) {
	e := newEnv267(t, netOK267)
	fail := failableNet267(t, e)
	e.pass() // démarrage réseau présent
	admin, _ := e.dial(t, "/ws/admin")
	e.drain(t, admin)
	before := e.app.engine.GetNetworkState()

	fail.Store(true)
	e.pass()
	e.pass()

	assertNoAction264(t, e.app, admin, protocol.ActionUpdate, "erreur de listage : aucune diffusion")
	after := e.app.engine.GetNetworkState()
	if after.OnlyLocalhost || len(after.IPs) != 1 || after.IPs[0] != "192.168.1.84" || after.CheckedAt != before.CheckedAt {
		t.Errorf("l'état précédent doit être conservé tel quel : avant %+v, après %+v", before, after)
	}
	if ev := networkEvents267(e); len(ev) != 0 {
		t.Errorf("aucun NETWORK_LOST ni autre événement sur une erreur de listage : %v", ev)
	}
	if n := listingFailures267(e); n != 2 {
		t.Errorf("une ligne WARN NETWORK_LISTING_FAILED par passage en erreur (2), obtenu %d (journal : %v)", n, e.logs(""))
	}

	// Le service reprend : même réseau → rien ; puis une vraie perte reste détectée.
	fail.Store(false)
	e.pass()
	assertNoAction264(t, e.app, admin, protocol.ActionUpdate, "retour du listage, réseau inchangé")
	e.setNet(netLost267)
	e.pass()
	if n := nextUpdate267(t, admin); !n.OnlyLocalhost {
		t.Error("une vraie perte doit toujours être diffusée après une erreur de listage")
	}
	if ev := networkEvents267(e); len(ev) != 1 || !strings.HasPrefix(ev[0], "NETWORK_LOST") {
		t.Errorf("exactement un NETWORK_LOST pour la vraie perte : %v", ev)
	}
}

func TestNetwork267_ListingError_AfterNetworkLost_StaysLostWithoutRestoredOrSecondLoss(t *testing.T) {
	e := newEnv267(t, netLost267)
	fail := failableNet267(t, e)
	e.pass() // démarrage en panne : un NETWORK_LOST
	admin, _ := e.dial(t, "/ws/admin")
	e.drain(t, admin)

	fail.Store(true)
	e.pass()

	assertNoAction264(t, e.app, admin, protocol.ActionUpdate, "erreur de listage pendant la panne : aucune diffusion")
	if st := e.app.engine.GetNetworkState(); !st.OnlyLocalhost {
		t.Errorf("l'état « perdu » est conservé : %+v", st)
	}
	if ev := networkEvents267(e); len(ev) != 1 || !strings.HasPrefix(ev[0], "NETWORK_LOST") {
		t.Errorf("ni NETWORK_RESTORED ni second NETWORK_LOST sur une erreur de listage : %v", ev)
	}
	if n := listingFailures267(e); n != 1 {
		t.Errorf("une ligne WARN NETWORK_LISTING_FAILED attendue, obtenu %d", n)
	}

	// Le retour réel du réseau est ensuite restauré normalement, une seule fois.
	fail.Store(false)
	e.setNet(netOK267)
	e.pass()
	if n := nextUpdate267(t, admin); n.OnlyLocalhost {
		t.Error("le retour du réseau doit être diffusé")
	}
	ev := networkEvents267(e)
	if len(ev) != 2 || !strings.HasPrefix(ev[1], "NETWORK_RESTORED") {
		t.Errorf("NETWORK_LOST puis un seul NETWORK_RESTORED attendus : %v", ev)
	}
}

func TestNetwork267_ListingError_AtFirstPass_ConsidersTheNetworkPresentAndRemainsFirstPass(t *testing.T) {
	e := newEnv267(t, netOK267)
	fail := failableNet267(t, e)
	admin, _ := e.dial(t, "/ws/admin")
	e.drain(t, admin)

	fail.Store(true)
	e.pass() // tout premier passage, en erreur

	assertNoAction264(t, e.app, admin, protocol.ActionUpdate, "premier passage en erreur : pas d'alerte")
	if st := e.app.engine.GetNetworkState(); st.OnlyLocalhost {
		t.Errorf("sans état précédent le réseau est considéré présent : %+v", st)
	}
	if ev := networkEvents267(e); len(ev) != 0 {
		t.Errorf("aucun événement réseau au premier passage en erreur : %v", ev)
	}
	if n := listingFailures267(e); n != 1 {
		t.Errorf("une ligne WARN NETWORK_LISTING_FAILED attendue, obtenu %d", n)
	}

	// Premier passage RÉUSSI avec un réseau utilisable : c'est le « démarrage » — ni diffusion,
	// ni NETWORK_IPS_CHANGED parasite (l'état par défaut n'a pas d'adresse).
	fail.Store(false)
	e.pass()
	assertNoAction264(t, e.app, admin, protocol.ActionUpdate, "premier passage réussi, réseau utilisable")
	if ev := networkEvents267(e); len(ev) != 0 {
		t.Errorf("premier passage réussi avec réseau : aucun événement : %v", ev)
	}
	if st := e.app.engine.GetNetworkState(); st.OnlyLocalhost || len(st.IPs) != 1 {
		t.Errorf("l'état doit être renseigné par le premier passage réussi : %+v", st)
	}
}

func TestNetwork267_ListingError_AtFirstPass_ThenRealLossIsStillAnEvent(t *testing.T) {
	e := newEnv267(t, netLost267)
	fail := failableNet267(t, e)

	fail.Store(true)
	e.pass()
	fail.Store(false)
	e.pass() // premier passage réussi : réseau réellement perdu → NETWORK_LOST

	if ev := networkEvents267(e); len(ev) != 1 || !strings.HasPrefix(ev[0], "NETWORK_LOST") {
		t.Errorf("la vraie perte découverte au premier passage réussi doit être signalée une fois : %v", ev)
	}
}
