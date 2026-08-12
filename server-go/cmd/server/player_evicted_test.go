package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// Tests : PLAYER_EVICTED (#120) — fermeture de la course d'inscription VJoueur
//
// Contrat : contracts/websocket-actions.md § PLAYER_EVICTED — notification
// ciblée (jamais broadcast) au client VJoueur dont le bumper vient d'être
// supprimé. Remplace la déduction par balayage de roster qui ouvrait la
// course décrite dans _work/reports/plan-20260728-101500.md (cause A) :
// « l'absence d'un bumper dans une mise à jour de roster n'est jamais, à
// elle seule, un motif de renvoi ».
//
// Harnais : mêmes conventions que
// onplayerdisconnected_ghost_test.go/TestOnPlayerDisconnected_ZombieGuard_SkipsIfReconnected
// (httptest.NewServer + dial réel) — SendToClient/SendToPlayerID exigent un
// client authentiquement enregistré dans le hub (h.clients est privé, seule
// la vraie boucle de connexion WebSocket peut l'alimenter).
// ---------------------------------------------------------------------------

// startEvictionTestServer starts a real WS server routing by path suffix
// (/ws/player, /ws/admin, /ws/tv), mirroring main.go's route registration,
// and starts the hub's Run() loop so registration/broadcast actually happen.
func startEvictionTestServer(t *testing.T, app *App) string {
	t.Helper()
	go app.wsHub.Run()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientType := server.ClientTypeVPlayer
		switch {
		case strings.HasSuffix(r.URL.Path, "/admin"):
			clientType = server.ClientTypeAdmin
		case strings.HasSuffix(r.URL.Path, "/tv"):
			clientType = server.ClientTypeTV
		}
		app.wsHub.HandleConnectionWithType(w, r, clientType)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func dialWS(t *testing.T, baseURL, path string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(baseURL, "http") + path
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial %s: %v", wsURL, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// learnClientID sends a harmless PONG so the hub's Incoming channel reveals
// the clientID gorilla assigned to this connection — same technique as
// onplayerdisconnected_ghost_test.go's learnClientID closure.
func learnClientID(t *testing.T, app *App, conn *websocket.Conn) string {
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
	select {
	case incoming := <-app.wsHub.Incoming:
		return incoming.ClientID
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for an incoming message")
		return ""
	}
}

// readAction reads exactly one server->client frame and returns its ACTION
// plus raw MSG payload, failing the test if none arrives within the timeout.
func readAction(t *testing.T, conn *websocket.Conn) (string, json.RawMessage) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("expected a message, got error: %v", err)
	}
	var envelope struct {
		Action string          `json:"ACTION"`
		Msg    json.RawMessage `json:"MSG"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("failed to unmarshal server message: %v (raw: %s)", err, data)
	}
	return envelope.Action, envelope.Msg
}

// collectActions drains whatever messages arrive on conn within a short
// window, without failing if none do — used to assert the ABSENCE of a given
// action among a client's normal traffic (UPDATE/ENROLLMENT_UPDATE/FULL...)
// instead of requiring an exact message count, which would make these tests
// brittle to unrelated broadcast changes.
func collectActions(conn *websocket.Conn, window time.Duration) []string {
	var actions []string
	deadline := time.Now().Add(window)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return actions
		}
		conn.SetReadDeadline(time.Now().Add(remaining))
		_, data, err := conn.ReadMessage()
		if err != nil {
			return actions // timeout (or closed) — no more messages
		}
		var envelope struct {
			Action string `json:"ACTION"`
		}
		if json.Unmarshal(data, &envelope) == nil {
			actions = append(actions, envelope.Action)
		}
	}
}

func containsAction(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func deleteBumperMsg(t *testing.T, id string) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionDeleteBumper, protocol.DeletePayload{ID: id})
	if err != nil {
		t.Fatalf("failed to build DELETE_BUMPER message: %v", err)
	}
	return msg
}

func newGameMsg(t *testing.T) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionNewGame, map[string]interface{}{})
	if err != nil {
		t.Fatalf("failed to build NEW_GAME message: %v", err)
	}
	return msg
}

// ---------------------------------------------------------------------------
// B1 — suppression par l'animateur : PLAYER_EVICTED{PLAYER_REMOVED}
// ---------------------------------------------------------------------------

// TestHandleDeleteBumper_VirtualPlayer_EmitsPlayerEvictedToTargetOnly is the
// core B1 test: deleting a VJoueur's bumper must notify THAT VJoueur's own
// client — and only it — with REASON=PLAYER_REMOVED.
func TestHandleDeleteBumper_VirtualPlayer_EmitsPlayerEvictedToTargetOnly(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)

	aliceID, _, err := app.engine.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}

	baseURL := startEvictionTestServer(t, app)

	targetConn := dialWS(t, baseURL, "/ws/player")
	targetClientID := learnClientID(t, app, targetConn)
	app.wsHub.SetClientPlayerID(targetClientID, aliceID)

	// A second, unrelated VJoueur must never receive an eviction meant for Alice.
	bystanderConn := dialWS(t, baseURL, "/ws/player")

	app.handleDeleteBumper(deleteBumperMsg(t, aliceID))

	action, rawMsg := readAction(t, targetConn)
	if action != protocol.ActionPlayerEvicted {
		t.Fatalf("expected the evicted VJoueur's client to receive %s, got %q", protocol.ActionPlayerEvicted, action)
	}
	var payload protocol.PlayerEvictedPayload
	if err := json.Unmarshal(rawMsg, &payload); err != nil {
		t.Fatalf("failed to unmarshal PLAYER_EVICTED payload: %v", err)
	}
	if payload.Reason != "PLAYER_REMOVED" {
		t.Errorf("expected REASON=PLAYER_REMOVED, got %q", payload.Reason)
	}

	bystanderActions := collectActions(bystanderConn, 300*time.Millisecond)
	if containsAction(bystanderActions, protocol.ActionPlayerEvicted) {
		t.Errorf("an unrelated VJoueur must never receive PLAYER_EVICTED, got actions: %v", bystanderActions)
	}
}

// TestHandleDeleteBumper_PhysicalBuzzer_NoPlayerEvicted verifies that
// deleting a physical buzzer (no WebSocket client of its own) never emits
// PLAYER_EVICTED to anyone.
func TestHandleDeleteBumper_PhysicalBuzzer_NoPlayerEvicted(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"AA:BB:CC:DD:EE:01": {Name: "Buzzer1", IsVirtual: false},
	})

	baseURL := startEvictionTestServer(t, app)
	bystanderConn := dialWS(t, baseURL, "/ws/player")

	app.handleDeleteBumper(deleteBumperMsg(t, "AA:BB:CC:DD:EE:01"))

	actions := collectActions(bystanderConn, 300*time.Millisecond)
	if containsAction(actions, protocol.ActionPlayerEvicted) {
		t.Errorf("deleting a physical buzzer must never emit PLAYER_EVICTED, got actions: %v", actions)
	}
}

// ---------------------------------------------------------------------------
// B1 — NEW_GAME : PLAYER_EVICTED{GAME_RESET} pour chaque VJoueur purgé
// ---------------------------------------------------------------------------

// TestNewGame_EmitsPlayerEvicted_GameResetToEachPurgedVJoueur verifies that
// InitGame's roster purge (triggered by NEW_GAME) notifies every purged
// VJoueur individually with REASON=GAME_RESET, and that a bystander admin
// client never receives PLAYER_EVICTED itself.
func TestNewGame_EmitsPlayerEvicted_GameResetToEachPurgedVJoueur(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)

	aliceID, _, err := app.engine.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer(Alice) failed: %v", err)
	}
	bobID, _, err := app.engine.CreateVirtualPlayer("Bob")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer(Bob) failed: %v", err)
	}

	baseURL := startEvictionTestServer(t, app)

	aliceConn := dialWS(t, baseURL, "/ws/player")
	aliceClientID := learnClientID(t, app, aliceConn)
	app.wsHub.SetClientPlayerID(aliceClientID, aliceID)

	bobConn := dialWS(t, baseURL, "/ws/player")
	bobClientID := learnClientID(t, app, bobConn)
	app.wsHub.SetClientPlayerID(bobClientID, bobID)

	adminConn := dialWS(t, baseURL, "/ws/admin")

	// handleWebMessage's NEW_GAME case logs via a.logger (the App-level instance,
	// distinct from the nil-safe package-level server.LogInfo/etc used elsewhere) —
	// newTestApp never sets it, so it must be initialized here to avoid a nil
	// pointer dereference.
	app.logger = server.NewBroadcastLogger(100)

	app.handleWebMessage(&protocol.IncomingMessage{ClientID: "admin-client", ClientType: "admin", Data: newGameMsg(t)})

	for name, conn := range map[string]*websocket.Conn{"Alice": aliceConn, "Bob": bobConn} {
		action, rawMsg := readAction(t, conn)
		if action != protocol.ActionPlayerEvicted {
			t.Fatalf("expected %s to receive %s, got %q", name, protocol.ActionPlayerEvicted, action)
		}
		var payload protocol.PlayerEvictedPayload
		if err := json.Unmarshal(rawMsg, &payload); err != nil {
			t.Fatalf("failed to unmarshal PLAYER_EVICTED payload for %s: %v", name, err)
		}
		if payload.Reason != "GAME_RESET" {
			t.Errorf("expected %s to receive REASON=GAME_RESET, got %q", name, payload.Reason)
		}
	}

	adminActions := collectActions(adminConn, 300*time.Millisecond)
	if containsAction(adminActions, protocol.ActionPlayerEvicted) {
		t.Errorf("admin must never receive PLAYER_EVICTED itself, got actions: %v", adminActions)
	}
}

// ---------------------------------------------------------------------------
// Non-régression : handlePlayerConnect garde son ordre PLAYER_CONNECTED puis
// broadcastUpdate (l'inversion de cet ordre est précisément ce qui ouvrait
// la course de #120 côté client — voir cause A du plan).
// ---------------------------------------------------------------------------

func TestHandlePlayerConnect_OrderPreserved_ConnectedThenBroadcastUpdate(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/player")
	clientID := learnClientID(t, app, conn)

	app.handlePlayerConnect(clientID, playerConnectMsg(t, "Alice", ""))

	firstAction, _ := readAction(t, conn)
	if firstAction != protocol.ActionPlayerConnected {
		t.Fatalf("non-regression: expected the FIRST message to be %s, got %q", protocol.ActionPlayerConnected, firstAction)
	}
	secondAction, _ := readAction(t, conn)
	if secondAction != protocol.ActionUpdate {
		t.Fatalf("non-regression: expected the SECOND message to be %s (broadcastUpdate), got %q", protocol.ActionUpdate, secondAction)
	}
}

// ---------------------------------------------------------------------------
// Couverture de concurrence à l'inscription (analyse de #120 : la
// sérialisation par canal à consommateur unique + le verrou moteur ferment
// déjà la course — ces tests verrouillent ces propriétés contre régression).
// ---------------------------------------------------------------------------

func TestHandlePlayerConnect_TwoDifferentNames_TwoDistinctBumperIDs(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)
	baseURL := startEvictionTestServer(t, app)

	conn1 := dialWS(t, baseURL, "/ws/player")
	client1 := learnClientID(t, app, conn1)
	app.handlePlayerConnect(client1, playerConnectMsg(t, "Alice", ""))
	action1, msg1 := readAction(t, conn1)
	if action1 != protocol.ActionPlayerConnected {
		t.Fatalf("expected PLAYER_CONNECTED for Alice, got %q", action1)
	}
	var payload1 protocol.PlayerConnectedPayload
	if err := json.Unmarshal(msg1, &payload1); err != nil {
		t.Fatalf("failed to unmarshal PLAYER_CONNECTED for Alice: %v", err)
	}

	conn2 := dialWS(t, baseURL, "/ws/player")
	client2 := learnClientID(t, app, conn2)
	app.handlePlayerConnect(client2, playerConnectMsg(t, "Bob", ""))
	action2, msg2 := readAction(t, conn2)
	if action2 != protocol.ActionPlayerConnected {
		t.Fatalf("expected PLAYER_CONNECTED for Bob, got %q", action2)
	}
	var payload2 protocol.PlayerConnectedPayload
	if err := json.Unmarshal(msg2, &payload2); err != nil {
		t.Fatalf("failed to unmarshal PLAYER_CONNECTED for Bob: %v", err)
	}

	if payload1.ID == "" || payload2.ID == "" {
		t.Fatal("both bumper IDs must be non-empty")
	}
	if payload1.ID == payload2.ID {
		t.Errorf("two different names must produce two distinct bumper IDs, both got %q", payload1.ID)
	}
	if app.engine.GetBumper(payload1.ID) == nil || app.engine.GetBumper(payload2.ID) == nil {
		t.Error("both bumpers must exist in the engine")
	}
}

func TestHandlePlayerConnect_SameName_SecondRejectedNameTaken(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)
	baseURL := startEvictionTestServer(t, app)

	conn1 := dialWS(t, baseURL, "/ws/player")
	client1 := learnClientID(t, app, conn1)
	app.handlePlayerConnect(client1, playerConnectMsg(t, "Alice", ""))
	if action, _ := readAction(t, conn1); action != protocol.ActionPlayerConnected {
		t.Fatalf("setup failed: expected first Alice to be accepted, got %q", action)
	}

	conn2 := dialWS(t, baseURL, "/ws/player")
	client2 := learnClientID(t, app, conn2)
	app.handlePlayerConnect(client2, playerConnectMsg(t, "Alice", "")) // same name, no ID

	action, rawMsg := readAction(t, conn2)
	if action != protocol.ActionPlayerRejected {
		t.Fatalf("expected the second 'Alice' (no ID) to be rejected, got %q", action)
	}
	var payload protocol.PlayerRejectedPayload
	if err := json.Unmarshal(rawMsg, &payload); err != nil {
		t.Fatalf("failed to unmarshal PLAYER_REJECTED payload: %v", err)
	}
	if payload.Reason != "NAME_TAKEN" {
		t.Errorf("expected REASON=NAME_TAKEN, got %q", payload.Reason)
	}
}

func TestHandlePlayerConnect_LastSlot_SecondRejectedLimitReached(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetVirtualPlayerLimit(1)
	baseURL := startEvictionTestServer(t, app)

	conn1 := dialWS(t, baseURL, "/ws/player")
	client1 := learnClientID(t, app, conn1)
	app.handlePlayerConnect(client1, playerConnectMsg(t, "Alice", ""))
	if action, _ := readAction(t, conn1); action != protocol.ActionPlayerConnected {
		t.Fatalf("setup failed: expected Alice to take the only slot, got %q", action)
	}

	conn2 := dialWS(t, baseURL, "/ws/player")
	client2 := learnClientID(t, app, conn2)
	app.handlePlayerConnect(client2, playerConnectMsg(t, "Bob", ""))

	action, rawMsg := readAction(t, conn2)
	if action != protocol.ActionPlayerRejected {
		t.Fatalf("expected Bob to be rejected once the limit is reached, got %q", action)
	}
	var payload protocol.PlayerRejectedPayload
	if err := json.Unmarshal(rawMsg, &payload); err != nil {
		t.Fatalf("failed to unmarshal PLAYER_REJECTED payload: %v", err)
	}
	if payload.Reason != "LIMIT_REACHED" {
		t.Errorf("expected REASON=LIMIT_REACHED, got %q", payload.Reason)
	}
}
