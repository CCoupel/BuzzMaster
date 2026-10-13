package main

// #264 — attentes par événement pour les tests de diffusion WebSocket.
//
// Une fenêtre d'observation fixe (collectActions(conn, 500ms)) est fragile sous
// charge : trop courte, un message en retard est vu comme « absent » ; elle
// ne prouve jamais que tout ce qui devait arriver est arrivé. Ici, après
// l'action testée, on diffuse un message témoin par le MÊME canal que les
// diffusions applicatives (hub.broadcast, FIFO) : les envois ciblés
// (SendRawToPlayerID) sont faits de façon synchrone avant lui, les diffusions
// asynchrones l'ont précédé dans la file. Quand une connexion lit le témoin,
// elle a donc reçu tout ce qui la concernait — sans Sleep ni fenêtre fixe.

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
