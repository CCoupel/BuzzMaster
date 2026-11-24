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

func strokesMsg193(t *testing.T) *protocol.Message { return strokesMsgPad193(t, 0) }

// strokesMsgPad193 : trame réaliste (~2,4 Ko, §9.1) ; pad > 0 la gonfle (remplissage rapide d'un tampon noyau).
func strokesMsgPad193(t *testing.T, pad int) *protocol.Message {
	t.Helper()
	pts := make([][2]int, 200)
	for i := range pts {
		pts[i] = [2]int{i * 40, i * 40}
	}
	msg, err := protocol.NewMessage("PICTO_STROKES", map[string]interface{}{
		"EPOCH": 1, "SEQ": 1,
		"STROKES": []map[string]interface{}{{"ID": 1, "PTS": pts, "END": true}},
		"PAD":     strings.Repeat("x", pad),
	})
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// Tampon plein : la trame de traits est perdue, le client est GARDÉ (jamais CLIENT_EVICTED_SLOW).
//
// Déterministe, indépendant de la vitesse du runner : (1) peu de trames, juste de quoi dépasser la file d'envoi (256) —
// la boucle s'arrête dès que la file est PLEINE (preuve structurelle, mesurée sur le client du hub), puis une salve de
// trames excédentaires est jetée ; (2) la non-éviction est constatée tout de suite, bien avant le délai d'écriture de 3 s
// (CLIENT_WRITE_TIMEOUT, coupure par timeout du serveur, distincte de l'éviction : si un runner exceptionnellement lent
// le laisse expirer pendant le remplissage, le test s'annule au lieu d'accuser TrySendToTypes).
func TestPictoRelay193_TrySend_FullBufferDropsFrameAndKeepsClient(t *testing.T) {
	hub, bl, url := newHub246(t, ClientTypeTV)
	dialStalled246(t, url)
	if !waitFor246(15*time.Second, func() bool { return hub.ClientCount() == 1 }) {
		t.Fatal("précondition : le client doit être enregistré")
	}
	ts := trySender(t, hub)
	msg := strokesMsgPad193(t, 8*1024) // 8 Ko : le tampon noyau (4 Ko de chaque côté) se remplit en quelques trames
	start := time.Now()

	queue := func() (n, capacity int) {
		hub.mu.RLock()
		defer hub.mu.RUnlock()
		for c := range hub.clients {
			return len(c.Send), cap(c.Send)
		}
		return 0, 0
	}
	// 1) remplir la file d'envoi (256) ; chaque envoi supplémentaire est jeté (pas d'éviction).
	sent := 0
	for ; sent < 5000; sent++ {
		if n, c := queue(); c > 0 && n == c {
			break
		}
		ts.TrySendToTypes(msg, ClientTypeTV)
	}
	n, c := queue()
	if c == 0 || n != c {
		t.Fatalf("précondition : la file d'envoi du client lent doit être pleine après %d trames (len=%d cap=%d)", sent, n, c)
	}
	for i := 0; i < 50; i++ { // salve excédentaire : toutes jetées, aucun blocage, aucune éviction
		ts.TrySendToTypes(msg, ClientTypeTV)
	}
	// 2) constat immédiat : bien avant le délai d'écriture de 3 s du serveur.
	elapsed := time.Since(start)
	stayed := hub.ClientCount() == 1
	evicted := logHas246(bl, "CLIENT_EVICTED_SLOW")
	if !stayed && logHas246(bl, "CLIENT_WRITE_TIMEOUT") && elapsed > 2*time.Second {
		t.Skipf("runner trop lent (%v) : le délai d'écriture de 3 s a coupé le client avant la fin du remplissage — coupure par timeout, pas une éviction", elapsed)
	}
	if !stayed {
		t.Errorf("le client au tampon plein doit RESTER connecté (clients=%d, écoulé %v)", hub.ClientCount(), elapsed)
	}
	if evicted {
		t.Errorf("TrySendToTypes ne doit jamais évincer un client lent ; logs=%v", bl.GetRecent(5))
	}
	if n2, _ := queue(); n2 != c {
		t.Errorf("la file reste pleine (les trames excédentaires sont perdues, pas empilées) : len=%d cap=%d", n2, c)
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
			if hub.ClientCount() == 0 {
				break
			}
		}
	}
	evicted = evicted || waitFor246(5*time.Second, func() bool { return logHas246(bl, "CLIENT_EVICTED_SLOW") })
	if !evicted && logHas246(bl, "CLIENT_WRITE_TIMEOUT") {
		// Runner exceptionnellement lent : le délai d'écriture de 3 s a coupé le client avant que la file (256) ne se remplisse.
		t.Skip("coupure par CLIENT_WRITE_TIMEOUT avant l'éviction : environnement trop lent pour ce scénario")
	}
	if !evicted {
		t.Fatal("BroadcastToTypes : un client au tampon plein doit toujours être coupé et journalisé (#127)")
	}
	if !waitFor246(15*time.Second, func() bool { return hub.ClientCount() == 0 }) {
		t.Errorf("le client lent doit être retiré du hub (%d restant)", hub.ClientCount())
	}
}
