package main

// #246 — corrections de la revue du Batch 1 : injection de journal via CLIENT_TRACE,
// remise à zéro de SEQ à la liaison, purge à la fermeture, plafond global, doublon via FULL.

import (
	"buzzcontrol/internal/game"
	"strings"
	"testing"
	"time"
)

func TestReview246_ClientTraceCannotForgeLogLines(t *testing.T) {
	e := newEnv246(t)
	cid := e.boundPaul(t)

	e.send(t, cid, "vplayer", "CLIENT_TRACE", map[string]interface{}{
		"KIND": "VISIBILITY\n[ERROR] [App] FAUX_INCIDENT", "AT": 1, "CAUSE_ACTION": "X\r\nY",
		"DETAIL": map[string]interface{}{"k\nz": "v\n[ERROR] forged", "n": 3, "o": map[string]interface{}{"a": "b\nc"}},
	})

	for _, en := range e.app.logger.GetHistory() {
		if strings.Contains(en.Message, "CLIENT_TRACE") {
			if strings.ContainsAny(en.Message, "\r\n") {
				t.Fatalf("une entrée de journal ne contient jamais de saut de ligne : %q", en.Message)
			}
			return
		}
	}
	t.Fatal("CLIENT_TRACE non journalisé")
}

func TestReview246_SeqRestartsAfterRebindingOnTheSameConnection(t *testing.T) {
	e := newEnv246(t)
	_, cid := e.dial(t, "/ws/player")
	e.connect(t, cid, "Paul", e.paul)
	for i := 1; i <= 3; i++ {
		e.typeAs(t, cid, e.paul, strings.Repeat("A", i), map[string]interface{}{"SEQ": i})
	}
	e.connect(t, cid, "Paul", e.paul) // 2e PLAYER_CONNECT : le client repart à SEQ 1

	e.typeAs(t, cid, e.paul, "B", map[string]interface{}{"SEQ": 1})
	e.typeAs(t, cid, e.paul, "BC", map[string]interface{}{"SEQ": 2})

	if n := e.countLog(game.LogLevelWarn, "ARDOISE_SEQ_"); n != 0 {
		t.Fatalf("aucun STALE/GAP après une re-liaison ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	if text, _ := e.answers(); text["Les Bleus"] != "BC" {
		t.Errorf("les frappes SEQ 1..N après re-liaison sont appliquées : %v", text)
	}
}

func TestReview246_DiagnosticStateIsPurgedWhenTheConnectionCloses(t *testing.T) {
	e := newEnv246(t)
	conn, cid := e.dial(t, "/ws/player")
	e.connect(t, cid, "Paul", e.paul)
	e.typeAs(t, cid, e.paul, "A", map[string]interface{}{"SEQ": 1})
	e.send(t, cid, "vplayer", "CLIENT_TRACE", map[string]interface{}{"KIND": "VISIBILITY", "AT": 1})

	e.app.wsHub.SetOnClientClosed(e.app.forgetClientDiag) // câblé par setupCallbacks en production
	conn.Close()
	deadline := 0
	for ; deadline < 100; deadline++ {
		e.app.diagMu.Lock()
		_, a := e.app.ardoiseSq[cid]
		_, b := e.app.traceLim[cid]
		e.app.diagMu.Unlock()
		if !a && !b {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("l'état SEQ / limiteur doit être purgé à la fermeture de la connexion")
}

func TestReview246_GlobalTraceCeilingAcrossConnections(t *testing.T) {
	e := newEnv246(t)
	for c := 0; c < 12; c++ {
		_, cid := e.dial(t, "/ws/player")
		for i := 0; i < 9; i++ { // sous le plafond par connexion (10/s), au-dessus de 50/s au total
			e.send(t, cid, "vplayer", "CLIENT_TRACE", map[string]interface{}{"KIND": "VISIBILITY", "AT": i})
		}
	}
	logged := 0
	for _, en := range e.app.logger.GetHistory() {
		if strings.Contains(en.Message, "CLIENT_TRACE ") {
			logged++
		}
	}
	if logged > 50 {
		t.Errorf("plafond global 50/s : %d traces journalisées", logged)
	}
	if n := e.countLog(game.LogLevelWarn, "CLIENT_TRACE_THROTTLED"); n != 1 {
		t.Errorf("un seul CLIENT_TRACE_THROTTLED global par minute : %d", n)
	}
}

func TestReview246_FullUpdateRejectsANewVJoueurWithATakenName(t *testing.T) {
	e := newEnv246(t)
	incoming := map[string]*game.Bumper{
		e.paul:        {Name: "Paul", IsVirtual: true, IsVPlayer: true, Team: "Les Bleus"},
		"vjoueur_new": {Name: " paul ", IsVirtual: true, IsVPlayer: true, Team: "Les Rouges"},
	}

	e.app.enforceUniqueVirtualNames(incoming)

	if _, ok := incoming["vjoueur_new"]; ok {
		t.Error("un nouveau VJoueur au nom déjà pris (casse/espaces ignorés) ne doit pas entrer")
	}
	if _, ok := incoming[e.paul]; !ok {
		t.Error("le détenteur du nom reste")
	}
	if !e.hasLog(game.LogLevelWarn, "VPLAYER_DUPLICATE_NAME") {
		t.Error("WARN VPLAYER_DUPLICATE_NAME attendu")
	}
}
