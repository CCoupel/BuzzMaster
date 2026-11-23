package server

// #193 (v13.0.0, LOT-2.1) — relais au mieux du dessin (contracts/picto.md §2 décision 6, §8.5, §16) — CRITICAL.
// `TrySendToTypes(msg, types...)` est le seul chemin de PICTO_STROKES : un client dont le tampon d'envoi est plein
// PERD la trame et RESTE connecté ; `BroadcastToTypes` est inchangé pour tout autre message (déconnexion au tampon
// plein conservée, #127). Écrit depuis le contrat : la méthode est appelée par une assertion d'interface, pour que
// ce fichier compile avant la livraison de dev-backend (échec explicite tant qu'elle n'existe pas).

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"buzzcontrol/internal/protocol"

	"github.com/gorilla/websocket"
)

type trySender193 interface {
	TrySendToTypes(msg *protocol.Message, types ...ClientType)
}

func trySender(t *testing.T, hub *WebSocketHub) trySender193 {
	t.Helper()
	ts, ok := interface{}(hub).(trySender193)
	if !ok {
		t.Fatal("WebSocketHub.TrySendToTypes(msg *protocol.Message, types ...ClientType) attendu (contrat picto.md §8.5)")
	}
	return ts
}

func strokesMsg193(t *testing.T) *protocol.Message {
	t.Helper()
	pts := make([][2]int, 200)
	for i := range pts {
		pts[i] = [2]int{i * 40, i * 40}
	}
	// ~2,4 Ko par trame (§9.1) ; gonflée à 16 Ko pour remplir vite le tampon d'un client qui ne lit pas.
	msg, err := protocol.NewMessage("PICTO_STROKES", map[string]interface{}{
		"EPOCH": 1, "SEQ": 1,
		"STROKES": []map[string]interface{}{{"ID": 1, "PTS": pts, "END": true}},
		"PAD":     strings.Repeat("x", 16*1024),
	})
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// Tampon plein : la trame de traits est perdue, le client est GARDÉ (jamais CLIENT_EVICTED_SLOW).
func TestPictoRelay193_TrySend_FullBufferDropsFrameAndKeepsClient(t *testing.T) {
	hub, bl, url := newHub246(t, ClientTypeTV)
	dialStalled246(t, url)
	if !waitFor246(15*time.Second, func() bool { return hub.ClientCount() == 1 }) {
		t.Fatal("précondition : le client doit être enregistré")
	}
	ts := trySender(t, hub)
	msg := strokesMsg193(t)
	// Bien au-delà de la file (256) : avec BroadcastToTypes ce client serait déjà coupé.
	for i := 0; i < 1500; i++ {
		ts.TrySendToTypes(msg, ClientTypeTV)
	}
	time.Sleep(300 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Errorf("le client au tampon plein doit RESTER connecté (clients=%d)", hub.ClientCount())
	}
	if logHas246(bl, "CLIENT_EVICTED_SLOW") {
		t.Errorf("TrySendToTypes ne doit jamais évincer un client lent ; logs=%v", bl.GetRecent(5))
	}
}

// Un client sain reçoit les trames ; un client lent du même type n'en prive pas les autres et n'est pas coupé.
func TestPictoRelay193_TrySend_HealthyClientReceives_StalledOneDoesNotBlockIt(t *testing.T) {
	hub, _, url := newHub246(t, ClientTypeTV)
	dialStalled246(t, url)
	healthy, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer healthy.Close()
	var received atomic.Int64
	go func() {
		for {
			if _, _, err := healthy.ReadMessage(); err != nil {
				return
			}
			received.Add(1)
		}
	}()
	if !waitFor246(15*time.Second, func() bool { return hub.ClientCount() == 2 }) {
		t.Fatal("précondition : deux clients enregistrés")
	}
	ts := trySender(t, hub)
	msg := strokesMsg193(t)
	const total = 100
	for i := 0; i < total; i++ {
		ts.TrySendToTypes(msg, ClientTypeTV)
		time.Sleep(2 * time.Millisecond)
	}
	if !waitFor246(10*time.Second, func() bool { return received.Load() >= total/2 }) {
		t.Errorf("le client sain doit recevoir les trames malgré le client lent (reçu %d/%d)", received.Load(), total)
	}
	if hub.ClientCount() != 2 {
		t.Errorf("aucun des deux clients ne doit être coupé (clients=%d)", hub.ClientCount())
	}
}

// TrySendToTypes ne vise que les types demandés : jamais les tablettes (§8.1).
func TestPictoRelay193_TrySend_OnlyRequestedTypes(t *testing.T) {
	hubTV, _, urlTV := newHub246(t, ClientTypeTV)
	tv, _, err := websocket.DefaultDialer.Dial(urlTV, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tv.Close()
	hubTablet, _, urlTablet := newHub246(t, ClientTypeVPlayer)
	tablet, _, err := websocket.DefaultDialer.Dial(urlTablet, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tablet.Close()
	waitFor246(15*time.Second, func() bool { return hubTV.ClientCount() == 1 && hubTablet.ClientCount() == 1 })

	msg := strokesMsg193(t)
	trySender(t, hubTV).TrySendToTypes(msg, ClientTypeTV)
	trySender(t, hubTablet).TrySendToTypes(msg, ClientTypeTV, ClientTypeAdmin, ClientTypeAnim) // aucune tablette ciblée

	tv.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, data, err := tv.ReadMessage(); err != nil || !strings.Contains(string(data), "PICTO_STROKES") {
		t.Errorf("la TV doit recevoir PICTO_STROKES : err=%v", err)
	}
	tablet.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	if _, data, err := tablet.ReadMessage(); err == nil {
		t.Errorf("une tablette ne reçoit jamais PICTO_STROKES : %s", data)
	}
}

// Non-régression (§16) : BroadcastToTypes garde la déconnexion au tampon plein pour tout autre message — la TV
// (type visé par le relais) y compris.
func TestPictoRelay193_BroadcastToTypes_UnchangedForOtherMessages(t *testing.T) {
	hub, bl, url := newHub246(t, ClientTypeTV)
	dialStalled246(t, url)
	if !waitFor246(15*time.Second, func() bool { return hub.ClientCount() == 1 }) {
		t.Fatal("précondition : le client doit être enregistré")
	}
	big, _ := protocol.NewMessage("UPDATE", map[string]string{"X": strings.Repeat("x", 16*1024)})
	evicted := false
	for i, deadline := 0, time.Now().Add(30*time.Second); !evicted && time.Now().Before(deadline); i++ {
		hub.BroadcastToTypes(big, ClientTypeTV)
		if i%8 == 7 {
			evicted = logHas246(bl, "CLIENT_EVICTED_SLOW")
		}
	}
	evicted = evicted || waitFor246(5*time.Second, func() bool { return logHas246(bl, "CLIENT_EVICTED_SLOW") })
	if !evicted {
		t.Fatal("BroadcastToTypes : un client au tampon plein doit toujours être coupé et journalisé (#127)")
	}
	if !waitFor246(15*time.Second, func() bool { return hub.ClientCount() == 0 }) {
		t.Errorf("le client lent doit être retiré du hub (%d restant)", hub.ClientCount())
	}
}
