package main

// #264 — attentes par événement pour les tests de diffusion WebSocket.
//
// Une fenêtre d'observation fixe (collectActions(conn, 500ms)) est fragile sous
// charge : trop courte, un message en retard est vu comme « absent » ; elle
// ne prouve jamais que tout ce qui devait arriver est arrivé. Ici, après
// l'action testée, on diffuse un message témoin (hub.BroadcastRaw) et on lit
// la connexion jusqu'à lui.
//
// Pourquoi le témoin ne double aucun message (#266, commentaire corrigé) : il
// emprunte h.broadcast puis la file d'envoi PAR CLIENT (client.Send), qui est
// FIFO. Tout ce qui est déjà dans cette file à ce moment le précède :
//   - les envois ciblés (SendRawToPlayerID, SendToClient) et les diffusions par
//     type (BroadcastRawToTypes) faits de façon synchrone par le code testé,
//     car ils écrivent directement dans client.Send avant le retour ;
//   - les diffusions passées par h.broadcast (Broadcast/BroadcastRaw), traitées
//     dans l'ordre par la boucle du hub.
// Ce n'est donc PAS « le même canal que les PAUSE » : les diffusions par type
// ne passent pas par h.broadcast, c'est la file client qui donne l'ordre.
//
// Limite : le témoin ne prouve rien sur un message que le code testé n'a pas
// encore produit (timer du coalescer, goroutine). Pour ceux-là, attendre
// d'abord l'événement attendu (readActionMatchingWithin, échéance large), puis
// le témoin prouve qu'aucun autre ne suit.

import (
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const (
	sentinelAction264 = "TEST_SENTINEL_264"
	// eventDeadline264 : échéance LARGE d'une attente par événement. Elle ne
	// ralentit pas le cas nominal (on sort dès que l'événement arrive).
	eventDeadline264 = 15 * time.Second
)

// broadcastSentinel264 diffuse le témoin à tous les clients du hub.
func broadcastSentinel264(t *testing.T, app *App) {
	t.Helper()
	msg, err := protocol.NewMessage(sentinelAction264, map[string]interface{}{})
	if err != nil {
		t.Fatalf("sentinel: NewMessage: %v", err)
	}
	data, err := msg.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("sentinel: serialize: %v", err)
	}
	app.wsHub.BroadcastRaw(data)
}

// collectUntilSentinel264 lit conn jusqu'au témoin (exclu) et retourne les
// ACTION vues avant lui. Échoue si le témoin n'arrive pas dans eventDeadline264.
func collectUntilSentinel264(t *testing.T, conn *websocket.Conn) []string {
	t.Helper()
	var actions []string
	deadline := time.Now().Add(eventDeadline264)
	for {
		conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("message témoin non reçu (vu jusque-là : %v) : %v", actions, err)
		}
		var env struct {
			Action string `json:"ACTION"`
		}
		if json.Unmarshal(data, &env) != nil {
			continue
		}
		if env.Action == sentinelAction264 {
			return actions
		}
		actions = append(actions, env.Action)
	}
}

// collectAfterSentinel264 (#266) : UN témoin, puis lecture de chaque connexion
// jusqu'à ce témoin. Retourne, dans l'ordre de conns, les ACTION vues avant lui.
//
// Toutes les connexions qu'on lira à CET instant doivent être passées ensemble :
// le témoin est diffusé à tous, donc une connexion non lue garderait un témoin
// en file et le prendrait pour la fin d'une observation ultérieure. Chaque
// connexion doit en outre être déjà enregistrée dans le hub (learnClientID),
// sinon elle ne recevrait jamais le témoin.
func collectAfterSentinel264(t *testing.T, app *App, conns ...*websocket.Conn) [][]string {
	t.Helper()
	broadcastSentinel264(t, app)
	out := make([][]string, len(conns))
	for i, c := range conns {
		out[i] = collectUntilSentinel264(t, c)
	}
	return out
}

// assertNoAction264 (#266) : après le témoin, conn n'a reçu aucun message ACTION
// `action`. Remplace collectActions(conn, <fenêtre fixe>) + recherche de l'action.
func assertNoAction264(t *testing.T, app *App, conn *websocket.Conn, action, why string) {
	t.Helper()
	actions := collectAfterSentinel264(t, app, conn)[0]
	if containsAction(actions, action) {
		t.Errorf("%s : %s ne doit pas être reçu (vu : %v)", why, action, actions)
	}
}

// eventually264 (#271) : attend que cond devienne vraie — par ÉVÉNEMENT, échéance large, échec bruyant. Le pas
// d'interrogation n'est pas une fenêtre d'observation : on sort dès que la condition est vraie. À réserver aux
// effets asynchrones qu'aucun message WebSocket ne signale (son, ambiance) ; ne prouve pas une absence.
func eventually264(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(eventDeadline264)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s : non atteint dans %v", what, eventDeadline264)
		}
		time.Sleep(2 * time.Millisecond)
	}
}
