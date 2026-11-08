package main

// #291 Phase A — remplacement des fenêtres d'absence de trames à durée fixe.
//
// Une assertion négative (« aucune trame fautive ») est vraie si la trame fautive
// n'est pas arrivée AVANT un point où l'on sait que tout ce que le serveur avait
// émis est arrivé. Ce point est une BARRIÈRE POSITIVE : après l'action, un message
// témoin est diffusé par le même hub ; une connexion WS livre ses trames dans
// l'ordre, donc quand le témoin est reçu, toute trame émise avant lui l'est aussi.
// Le témoin ne couvre pas les envois différés par un saut de goroutine du code sous
// test ; ceux-ci sont couverts par une courte marge de silence (settleQuiet291)
// — 100 ms contre ~1 ms de latence mesurée sur la boucle locale, sans minuteur ni
// coalesceur dans ces scénarios (le coalesceur ARDOISE, 150 ms, n'y intervient pas).

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"

	"github.com/gorilla/websocket"
)

const settleQuiet291 = 100 * time.Millisecond

var barrierSeq291 atomic.Int64

// has reports whether a frame containing needle was received (frames are not consumed).
func (tp *tap253) has291(needle string) bool {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	for _, f := range tp.frames {
		if strings.Contains(f, needle) {
			return true
		}
	}
	return false
}

// dropContaining291 removes the witness frames so assertions never see them.
func (tp *tap253) dropContaining291(needle string) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	kept := tp.frames[:0:0]
	cursor := 0
	for i, f := range tp.frames {
		if strings.Contains(f, needle) {
			continue
		}
		kept = append(kept, f)
		if i < tp.cursor {
			cursor++
		}
	}
	tp.frames = kept
	tp.cursor = cursor
}

// barrier291 diffuse un témoin à tous les types de clients, attend (≤ 2 s) qu'il soit
// arrivé sur chacune des connexions, l'efface des trames puis laisse la marge de silence.
func barrier291(t *testing.T, app *App, conns ...*websocket.Conn) {
	t.Helper()
	id := fmt.Sprintf("BARRIER291-%d", barrierSeq291.Add(1))
	msg, err := protocol.NewMessage("BARRIER_291", map[string]string{"ID": id})
	if err != nil {
		t.Fatalf("barrier291 : %v", err)
	}
	app.wsHub.BroadcastToTypes(msg, server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer, server.ClientTypeAnim)
	deadline := time.Now().Add(2 * time.Second)
	for _, c := range conns {
		tp := tapOf253(c)
		for !tp.has291(id) {
			if time.Now().After(deadline) {
				t.Fatalf("barrier291 : témoin %s jamais reçu sur une connexion", id)
			}
			time.Sleep(2 * time.Millisecond)
		}
	}
	time.Sleep(settleQuiet291)
	for _, c := range conns {
		tapOf253(c).dropContaining291(id)
	}
}

// count291 returns the number of frames received so far (consumed or not).
func (tp *tap253) count291() int {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	return len(tp.frames)
}

// conns253 : les connexions espionnées de l'environnement launch_lock/HELLO.
func (e *env253) conns253() []*websocket.Conn {
	return []*websocket.Conn{e.a1, e.a2, e.an, e.tvc, e.paulConn}
}

// settle253 : barrière positive sur toutes les connexions de l'environnement (voir en-tête). Les actions du test
// sont exécutées de façon synchrone (handleWebMessage appelé en ligne) : leurs envois sont en file avant le témoin.
func (e *env253) settle253() { barrier291(e.t253, e.app, e.conns253()...) }

// purge253 : barrière puis oubli des trames reçues sur toutes les connexions.
func (e *env253) purge253() {
	e.settle253()
	for _, c := range e.conns253() {
		tapOf253(c).take()
	}
}

// collect253 : barrière puis trames reçues par conn depuis le dernier prélèvement (remplace collectRaw253(conn, fenêtre)).
func (e *env253) collect253(conn *websocket.Conn) []string {
	e.settle253()
	return tapOf253(conn).take()
}
