package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// Tests : câblage Phase 2 (#109) — sources réelles MessageLost/DeliveryConfirmed
//
// Complète internal/game/engine_connstate_phase2_test.go (logique pure moteur)
// côté transport : ces tests vérifient que les points d'ancrage réels
// (sendLEDSet, handleBuzzerACK, handleWebMessage) appellent bien
// TransitionConn/ConfirmDelivery avec le bon événement — voir
// _work/handoff/task-dev-backend-20260725-094500.md §1/§2.
// ---------------------------------------------------------------------------

// participantBumperFor assigns a bumper to a team (creating the team if
// needed) and sets its CONNECTED state — the minimal setup for a "participant"
// per the plan's scope rule (Team != "").
func participantBumperFor(t *testing.T, app *App, id, team string, connected bool) {
	t.Helper()
	app.engine.SetTeams(map[string]*game.Team{team: {Name: team}})
	app.engine.UpdateBumper(id, map[string]interface{}{"TEAM": team, "CONNECTED": connected})
}

// TestSendLEDSet_DisconnectedBuzzer_FiresMessageLost verifies the #109 Phase 2
// (D4) anchor point: sendLEDSet is the canonical LED_SET emission path, and it
// must flag a lost message when the target buzzer isn't connected.
func TestSendLEDSet_DisconnectedBuzzer_FiresMessageLost(t *testing.T) {
	app := newTestApp(t)
	mac := "AA:BB:CC:DD:EE:01"
	participantBumperFor(t, app, mac, "TeamA", false)
	if got := app.engine.GetBumper(mac).ConnState; got != "orange" {
		t.Fatalf("setup failed: expected orange after disconnect, got %q", got)
	}

	app.sendLEDSet(mac, protocol.LEDSetPayload{Color: [3]int{255, 0, 0}, Effect: "SOLID"})

	if got := app.engine.GetBumper(mac).ConnState; got != "red" {
		t.Errorf("expected orange -> red after LED_SET emitted to a disconnected buzzer, got %q", got)
	}
}

// TestSendWifiConfigToBuzzer_DisconnectedBuzzer_FiresMessageLost mirrors the
// LED_SET test for the WIFI_CONFIG anchor point (sendWifiConfigToBuzzer).
func TestSendWifiConfigToBuzzer_DisconnectedBuzzer_FiresMessageLost(t *testing.T) {
	app := newTestApp(t)
	app.ackManager = server.NewAckManager(&app.config.Server)
	mac := "AA:BB:CC:DD:EE:02"
	participantBumperFor(t, app, mac, "TeamA", false)

	app.sendWifiConfigToBuzzer(mac)

	if got := app.engine.GetBumper(mac).ConnState; got != "red" {
		t.Errorf("expected orange -> red after WIFI_CONFIG emitted to a disconnected buzzer, got %q", got)
	}
}

// TestHandleBuzzerACK_ConfirmsDelivery verifies handleBuzzerACK calls
// ConfirmDelivery for the acknowledging buzzer (D1/D3: a real ACK is the
// buzzer's fidèle delivery confirmation).
func TestHandleBuzzerACK_ConfirmsDelivery(t *testing.T) {
	app := newTestApp(t)
	app.ackManager = server.NewAckManager(&app.config.Server)

	mac := "AA:BB:CC:DD:EE:03"
	participantBumperFor(t, app, mac, "TeamA", false)
	app.engine.TransitionConn(mac, game.ConnEventDisconnect)
	app.engine.TransitionConn(mac, game.ConnEventReconnect)
	if got := app.engine.GetBumper(mac).ConnState; got != "green" {
		t.Fatalf("setup failed: expected green, got %q", got)
	}

	// connGreenMinDuration is unexported (game package) and intentionally not
	// exposed for cross-package overriding — wait past the real, production
	// default (2s, D2) so the assertion below can only pass if handleBuzzerACK
	// genuinely called engine.ConfirmDelivery (DeliveryConfirmed is a no-op on
	// every state except green, so this is the only way to observe the wiring
	// fired at all). Register the ACK only AFTER the wait, so the ack_timeout_ms
	// retry/expiry clock (also ~2s by default) starts fresh and can't race it.
	time.Sleep(2100 * time.Millisecond)

	msgID := "abc123def456"
	app.ackManager.Register(mac, msgID, protocol.ActionLEDSet)

	ackMsg, err := protocol.NewMessage(protocol.ActionACK, protocol.AckPayload{
		AckAction: protocol.ActionLEDSet,
		AckID:     msgID,
	})
	if err != nil {
		t.Fatalf("failed to build ACK message: %v", err)
	}
	ackMsg.ID = mac

	app.handleBuzzerACK(mac, ackMsg)

	if got := app.engine.GetBumper(mac).ConnState; got != "" {
		t.Errorf("expected ConnState=HIDDEN after handleBuzzerACK confirms delivery past the window, got %q", got)
	}
}

// TestHandleWebMessage_ConfirmsDeliveryForIdentifiedVPlayer verifies the D2/D3
// "any message received from the VJoueur" anchor point: handleWebMessage must
// call ConfirmDelivery whenever the sending client is linked to a PlayerID.
//
// Uses a real WebSocket connection (SetClientPlayerID no-ops on an unknown
// clientID, so the client must genuinely be registered in the hub) — mirrors
// internal/server/websocket_test.go's startTestWSServer/dialWSPath pattern,
// duplicated here since handleWebMessage/App are unexported in package main.
func TestHandleWebMessage_ConfirmsDeliveryForIdentifiedVPlayer(t *testing.T) {
	app := newTestAppWithHub(t)
	go app.wsHub.Run()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.wsHub.HandleConnectionWithType(w, r, server.ClientTypeVPlayer)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/player"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial test WS server: %v", err)
	}
	defer conn.Close()

	sendPong := func() {
		t.Helper()
		msg, err := protocol.NewMessage("PONG", map[string]interface{}{})
		if err != nil {
			t.Fatalf("failed to build message: %v", err)
		}
		data, err := msg.SerializeForWebSocket()
		if err != nil {
			t.Fatalf("failed to serialize message: %v", err)
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			t.Fatalf("failed to send WS message: %v", err)
		}
	}
	readIncoming := func() *protocol.IncomingMessage {
		t.Helper()
		select {
		case incoming := <-app.wsHub.Incoming:
			return incoming
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for an incoming message")
			return nil
		}
	}

	// First message: only used to learn the clientID the hub assigned.
	sendPong()
	clientID := readIncoming().ClientID
	if clientID == "" {
		t.Fatal("did not learn a clientID from the first incoming message")
	}

	playerID := "vjoueur_Dana_20260725"
	participantBumperFor(t, app, playerID, "TeamA", true)
	app.engine.TransitionConn(playerID, game.ConnEventDisconnect)
	app.engine.TransitionConn(playerID, game.ConnEventReconnect)
	if got := app.engine.GetBumper(playerID).ConnState; got != "green" {
		t.Fatalf("setup failed: expected green, got %q", got)
	}

	app.wsHub.SetClientPlayerID(clientID, playerID)
	if got, ok := app.wsHub.GetClientPlayerID(clientID); !ok || got != playerID {
		t.Fatalf("setup failed: GetClientPlayerID = (%q, %v), want (%q, true)", got, ok, playerID)
	}

	// connGreenMinDuration is unexported (game package) and intentionally not
	// exposed for cross-package overriding — wait past the real, production
	// default (2s, D2) so the assertion below can only pass if handleWebMessage
	// genuinely called engine.ConfirmDelivery (DeliveryConfirmed is a no-op on
	// every state except green, so "stays green" alone wouldn't prove the wiring
	// fired at all; the elapsed window makes the immediate-apply branch the only
	// way to observe a HIDDEN transition here).
	time.Sleep(2100 * time.Millisecond)

	// This is the message under test — handleWebMessage must call ConfirmDelivery
	// for it since the client is now linked to playerID.
	sendPong()
	app.handleWebMessage(readIncoming())

	if got := app.engine.GetBumper(playerID).ConnState; got != "" {
		t.Errorf("expected ConnState=HIDDEN after handleWebMessage confirms delivery past the window, got %q", got)
	}
}

// TestHandleWebMessage_UnidentifiedClient_NoPanic is a defensive test: a
// client with no linked PlayerID (admin, TV, or a VPlayer before
// PLAYER_CONNECT) must not cause handleWebMessage to touch any bumper.
func TestHandleWebMessage_UnidentifiedClient_NoPanic(t *testing.T) {
	app := newTestAppWithHub(t)
	// This message's empty ClientType is rejected by the #154 allow-list gate
	// (main.go), which since #155/#156 logs via a.logger (not the nil-safe
	// package-level server.LogWarn) — must be initialized to avoid a nil
	// pointer dereference, same reasoning as player_evicted_test.go's
	// identical setup for its own a.logger-using handleWebMessage path.
	app.logger = server.NewBroadcastLogger(100)
	pongMsg, err := protocol.NewMessage("PONG", map[string]interface{}{})
	if err != nil {
		t.Fatalf("failed to build message: %v", err)
	}
	app.handleWebMessage(&protocol.IncomingMessage{ClientID: "client-unknown", Data: pongMsg})
}
