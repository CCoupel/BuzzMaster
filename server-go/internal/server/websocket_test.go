package server

import (
	"buzzcontrol/internal/protocol"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ============================================================================
// Helpers
// ============================================================================

// startTestWSServer starts a test HTTP server with a WebSocketHub.
func startTestWSServer(t *testing.T) (*httptest.Server, *WebSocketHub, func()) {
	t.Helper()
	hub := NewWebSocketHub()
	go hub.Run()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Route by path to test dedicated endpoints
		switch r.URL.Path {
		case "/ws/admin":
			hub.HandleConnectionWithType(w, r, ClientTypeAdmin)
		case "/ws/tv":
			hub.HandleConnectionWithType(w, r, ClientTypeTV)
		case "/ws/player":
			hub.HandleConnectionWithType(w, r, ClientTypeVPlayer)
		default:
			hub.HandleConnection(w, r) // legacy: defaults to admin
		}
	}))

	return srv, hub, func() { srv.Close() }
}

// dialWSPath connects to the test server at the given path.
func dialWSPath(t *testing.T, srv *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + path
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket at %s: %v", path, err)
	}
	return conn
}

// readWSMsg reads a WebSocket message with a deadline.
func readWSMsg(t *testing.T, conn *websocket.Conn, timeout time.Duration) *protocol.Message {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read WebSocket message: %v", err)
	}
	msg, err := protocol.ParseSingle(data)
	if err != nil {
		t.Fatalf("Failed to parse WebSocket message: %v", err)
	}
	return msg
}

// expectNoMessage asserts that no message arrives within the timeout.
func expectNoMessage(t *testing.T, conn *websocket.Conn, timeout time.Duration) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Error("Expected no message but received one")
	}
}

// ============================================================================
// Tests: HandleConnectionWithType
// ============================================================================

func TestHandleConnectionWithType_AdminType(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/admin")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	adminCount, tvCount, vplayerCount := hub.GetClientCounts()
	if adminCount != 1 {
		t.Errorf("Expected 1 admin client, got %d", adminCount)
	}
	if tvCount != 0 || vplayerCount != 0 {
		t.Errorf("Expected 0 tv/vplayer, got tv=%d vplayer=%d", tvCount, vplayerCount)
	}
}

func TestHandleConnectionWithType_TVType(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/tv")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	adminCount, tvCount, vplayerCount := hub.GetClientCounts()
	if tvCount != 1 {
		t.Errorf("Expected 1 TV client, got %d", tvCount)
	}
	if adminCount != 0 || vplayerCount != 0 {
		t.Errorf("Expected 0 admin/vplayer, got admin=%d vplayer=%d", adminCount, vplayerCount)
	}
}

func TestHandleConnectionWithType_VPlayerType(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	adminCount, tvCount, vplayerCount := hub.GetClientCounts()
	if vplayerCount != 1 {
		t.Errorf("Expected 1 VPlayer client, got %d", vplayerCount)
	}
	if adminCount != 0 || tvCount != 0 {
		t.Errorf("Expected 0 admin/tv, got admin=%d tv=%d", adminCount, tvCount)
	}
}

func TestHandleConnection_DefaultsToAdmin(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/") // uses default HandleConnection
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	adminCount, tvCount, vplayerCount := hub.GetClientCounts()
	if adminCount != 1 {
		t.Errorf("Expected 1 admin client from legacy /ws, got %d", adminCount)
	}
	if tvCount != 0 || vplayerCount != 0 {
		t.Errorf("Expected 0 tv/vplayer from legacy /ws, got tv=%d vplayer=%d", tvCount, vplayerCount)
	}
}

// ============================================================================
// Tests: BroadcastToTypes — type-filtered routing
// ============================================================================

func TestBroadcastToTypes_AdminOnly(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()
	tv := dialWSPath(t, srv, "/ws/tv")
	defer tv.Close()
	player := dialWSPath(t, srv, "/ws/player")
	defer player.Close()

	time.Sleep(50 * time.Millisecond)

	msg, _ := protocol.NewMessage(protocol.ActionClients, nil)
	hub.BroadcastToTypes(msg, ClientTypeAdmin)

	// Admin should receive
	received := readWSMsg(t, admin, 500*time.Millisecond)
	if received.Action != protocol.ActionClients {
		t.Errorf("Admin: expected CLIENTS, got %s", received.Action)
	}

	// TV and VPlayer should NOT receive
	expectNoMessage(t, tv, 150*time.Millisecond)
	expectNoMessage(t, player, 150*time.Millisecond)
}

func TestBroadcastToTypes_TVOnly(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()
	tv := dialWSPath(t, srv, "/ws/tv")
	defer tv.Close()
	player := dialWSPath(t, srv, "/ws/player")
	defer player.Close()

	time.Sleep(50 * time.Millisecond)

	msg, _ := protocol.NewMessage(protocol.ActionReady, nil)
	hub.BroadcastToTypes(msg, ClientTypeTV)

	// TV should receive
	received := readWSMsg(t, tv, 500*time.Millisecond)
	if received.Action != protocol.ActionReady {
		t.Errorf("TV: expected READY, got %s", received.Action)
	}

	// Admin and VPlayer should NOT receive
	expectNoMessage(t, admin, 150*time.Millisecond)
	expectNoMessage(t, player, 150*time.Millisecond)
}

func TestBroadcastToTypes_AdminAndTV(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()
	tv := dialWSPath(t, srv, "/ws/tv")
	defer tv.Close()
	player := dialWSPath(t, srv, "/ws/player")
	defer player.Close()

	time.Sleep(50 * time.Millisecond)

	msg, _ := protocol.NewMessage(protocol.ActionReady, nil)
	hub.BroadcastToTypes(msg, ClientTypeAdmin, ClientTypeTV)

	// Admin and TV should receive
	adminMsg := readWSMsg(t, admin, 500*time.Millisecond)
	if adminMsg.Action != protocol.ActionReady {
		t.Errorf("Admin: expected READY, got %s", adminMsg.Action)
	}
	tvMsg := readWSMsg(t, tv, 500*time.Millisecond)
	if tvMsg.Action != protocol.ActionReady {
		t.Errorf("TV: expected READY, got %s", tvMsg.Action)
	}

	// VPlayer should NOT receive
	expectNoMessage(t, player, 150*time.Millisecond)
}

func TestBroadcastToTypes_AllTypes(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()
	tv := dialWSPath(t, srv, "/ws/tv")
	defer tv.Close()
	player := dialWSPath(t, srv, "/ws/player")
	defer player.Close()

	time.Sleep(50 * time.Millisecond)

	msg, _ := protocol.NewMessage(protocol.ActionUpdate, nil)
	hub.BroadcastToTypes(msg, ClientTypeAdmin, ClientTypeTV, ClientTypeVPlayer)

	// All three should receive
	adminMsg := readWSMsg(t, admin, 500*time.Millisecond)
	if adminMsg.Action != protocol.ActionUpdate {
		t.Errorf("Admin: expected UPDATE, got %s", adminMsg.Action)
	}
	tvMsg := readWSMsg(t, tv, 500*time.Millisecond)
	if tvMsg.Action != protocol.ActionUpdate {
		t.Errorf("TV: expected UPDATE, got %s", tvMsg.Action)
	}
	playerMsg := readWSMsg(t, player, 500*time.Millisecond)
	if playerMsg.Action != protocol.ActionUpdate {
		t.Errorf("VPlayer: expected UPDATE, got %s", playerMsg.Action)
	}
}

func TestBroadcastToTypes_VPlayerOnly(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()
	tv := dialWSPath(t, srv, "/ws/tv")
	defer tv.Close()
	player := dialWSPath(t, srv, "/ws/player")
	defer player.Close()

	time.Sleep(50 * time.Millisecond)

	msg, _ := protocol.NewMessage(protocol.ActionPlayerRejected, nil)
	hub.BroadcastToTypes(msg, ClientTypeVPlayer)

	received := readWSMsg(t, player, 500*time.Millisecond)
	if received.Action != protocol.ActionPlayerRejected {
		t.Errorf("VPlayer: expected PLAYER_REJECTED, got %s", received.Action)
	}

	// Admin and TV should NOT receive
	expectNoMessage(t, admin, 150*time.Millisecond)
	expectNoMessage(t, tv, 150*time.Millisecond)
}

func TestBroadcastToTypes_NoTypes_SendsNothing(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()

	time.Sleep(50 * time.Millisecond)

	msg, _ := protocol.NewMessage(protocol.ActionUpdate, nil)
	hub.BroadcastToTypes(msg) // no types

	expectNoMessage(t, admin, 150*time.Millisecond)
}

func TestBroadcastToTypes_MultipleClientsOfSameType(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin1 := dialWSPath(t, srv, "/ws/admin")
	defer admin1.Close()
	admin2 := dialWSPath(t, srv, "/ws/admin")
	defer admin2.Close()
	tv := dialWSPath(t, srv, "/ws/tv")
	defer tv.Close()

	time.Sleep(50 * time.Millisecond)

	msg, _ := protocol.NewMessage(protocol.ActionClients, nil)
	hub.BroadcastToTypes(msg, ClientTypeAdmin)

	// Both admin clients should receive
	msg1 := readWSMsg(t, admin1, 500*time.Millisecond)
	if msg1.Action != protocol.ActionClients {
		t.Errorf("Admin1: expected CLIENTS, got %s", msg1.Action)
	}
	msg2 := readWSMsg(t, admin2, 500*time.Millisecond)
	if msg2.Action != protocol.ActionClients {
		t.Errorf("Admin2: expected CLIENTS, got %s", msg2.Action)
	}

	// TV should NOT receive
	expectNoMessage(t, tv, 150*time.Millisecond)
}

// ============================================================================
// Tests: SetClientType (retro-compat — still works after v3.8.0)
// ============================================================================

func TestSetClientType_CanChangeType(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	// Connect as admin (default)
	conn := dialWSPath(t, srv, "/ws/admin")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	adminCount, tvCount, _ := hub.GetClientCounts()
	if adminCount != 1 || tvCount != 0 {
		t.Fatalf("Before: expected admin=1 tv=0, got admin=%d tv=%d", adminCount, tvCount)
	}

	// Find the client ID and change type to TV
	hub.mu.RLock()
	var clientID string
	for c := range hub.clients {
		clientID = c.ID
	}
	hub.mu.RUnlock()

	hub.SetClientType(clientID, ClientTypeTV)

	adminCount, tvCount, _ = hub.GetClientCounts()
	if adminCount != 0 || tvCount != 1 {
		t.Errorf("After SetClientType: expected admin=0 tv=1, got admin=%d tv=%d", adminCount, tvCount)
	}
}

// ============================================================================
// Tests: Concurrent BroadcastToTypes (thread safety)
// ============================================================================

// ============================================================================
// Tests: BroadcastRawToTypes — pre-serialized bytes, type-filtered (#41)
// ============================================================================

// TestBroadcastRawToTypes_AdminOnly verifies that raw bytes sent with AdminOnly type
// are received by admin clients and NOT by TV or VPlayer.
func TestBroadcastRawToTypes_AdminOnly(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()
	tv := dialWSPath(t, srv, "/ws/tv")
	defer tv.Close()
	player := dialWSPath(t, srv, "/ws/player")
	defer player.Close()

	time.Sleep(50 * time.Millisecond)

	rawData := []byte(`{"ACTION":"CLIENTS","MSG":{}}`)
	hub.BroadcastRawToTypes(rawData, ClientTypeAdmin)

	// Admin must receive
	admin.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, got, err := admin.ReadMessage()
	if err != nil {
		t.Fatalf("Admin: expected raw message, got error: %v", err)
	}
	if string(got) != string(rawData) {
		t.Errorf("Admin: expected %s, got %s", rawData, got)
	}

	// TV and VPlayer must NOT receive
	expectNoMessage(t, tv, 150*time.Millisecond)
	expectNoMessage(t, player, 150*time.Millisecond)
}

// TestBroadcastRawToTypes_AdminAndTV verifies that raw bytes sent to admin+tv
// are received by both, but NOT by VPlayer.
func TestBroadcastRawToTypes_AdminAndTV(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()
	tv := dialWSPath(t, srv, "/ws/tv")
	defer tv.Close()
	player := dialWSPath(t, srv, "/ws/player")
	defer player.Close()

	time.Sleep(50 * time.Millisecond)

	rawData := []byte(`{"ACTION":"UPDATE","MSG":{"PHASE":"STARTED"}}`)
	hub.BroadcastRawToTypes(rawData, ClientTypeAdmin, ClientTypeTV)

	// Admin must receive
	admin.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, gotAdmin, err := admin.ReadMessage()
	if err != nil {
		t.Fatalf("Admin: expected raw message: %v", err)
	}
	if string(gotAdmin) != string(rawData) {
		t.Errorf("Admin: expected %s, got %s", rawData, gotAdmin)
	}

	// TV must receive
	tv.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, gotTV, err := tv.ReadMessage()
	if err != nil {
		t.Fatalf("TV: expected raw message: %v", err)
	}
	if string(gotTV) != string(rawData) {
		t.Errorf("TV: expected %s, got %s", rawData, gotTV)
	}

	// VPlayer must NOT receive
	expectNoMessage(t, player, 150*time.Millisecond)
}

// TestBroadcastRawToTypes_NoTypes verifies that no message is sent when no types
// are provided.
func TestBroadcastRawToTypes_NoTypes(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()

	time.Sleep(50 * time.Millisecond)

	rawData := []byte(`{"ACTION":"UPDATE","MSG":{}}`)
	hub.BroadcastRawToTypes(rawData) // no types

	expectNoMessage(t, admin, 150*time.Millisecond)
}

// TestBroadcastRawToTypes_ConcurrentSafety verifies that concurrent BroadcastRawToTypes
// calls do not panic or deadlock.
func TestBroadcastRawToTypes_ConcurrentSafety(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()

	time.Sleep(50 * time.Millisecond)

	const goroutines = 20
	var wg sync.WaitGroup
	rawData := []byte(`{"ACTION":"UPDATE","MSG":{}}`)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hub.BroadcastRawToTypes(rawData, ClientTypeAdmin)
		}()
	}
	wg.Wait()

	// Drain — must not deadlock
	received := 0
	for {
		admin.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		_, _, err := admin.ReadMessage()
		if err != nil {
			break
		}
		received++
		if received >= goroutines {
			break
		}
	}
	if received == 0 {
		t.Error("Expected to receive at least some raw messages")
	}
}

// ============================================================================
// Tests: BroadcastToTypes — contract-specific routing (#11)
// ============================================================================

// TestBroadcastToTypes_Questions_AdminOnly verifies that QUESTIONS is routed
// exclusively to admin clients, as specified in contracts/websocket-endpoints.md.
func TestBroadcastToTypes_Questions_AdminOnly(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()
	tv := dialWSPath(t, srv, "/ws/tv")
	defer tv.Close()
	player := dialWSPath(t, srv, "/ws/player")
	defer player.Close()

	time.Sleep(50 * time.Millisecond)

	msg, _ := protocol.NewMessage(protocol.ActionQuestions, nil)
	hub.BroadcastToTypes(msg, ClientTypeAdmin)

	// Admin must receive QUESTIONS
	received := readWSMsg(t, admin, 500*time.Millisecond)
	if received.Action != protocol.ActionQuestions {
		t.Errorf("Admin: expected QUESTIONS, got %s", received.Action)
	}

	// TV and VPlayer must NOT receive QUESTIONS
	expectNoMessage(t, tv, 150*time.Millisecond)
	expectNoMessage(t, player, 150*time.Millisecond)
}

// TestBroadcastToTypes_BackgroundChange_AdminAndTV verifies that BACKGROUND_CHANGE
// is routed to admin and tv, but not vplayer, per contracts/websocket-endpoints.md.
func TestBroadcastToTypes_BackgroundChange_AdminAndTV(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()
	tv := dialWSPath(t, srv, "/ws/tv")
	defer tv.Close()
	player := dialWSPath(t, srv, "/ws/player")
	defer player.Close()

	time.Sleep(50 * time.Millisecond)

	msg, _ := protocol.NewMessage(protocol.ActionBackgroundChange, nil)
	hub.BroadcastToTypes(msg, ClientTypeAdmin, ClientTypeTV)

	// Admin must receive BACKGROUND_CHANGE
	adminMsg := readWSMsg(t, admin, 500*time.Millisecond)
	if adminMsg.Action != protocol.ActionBackgroundChange {
		t.Errorf("Admin: expected BACKGROUND_CHANGE, got %s", adminMsg.Action)
	}

	// TV must receive BACKGROUND_CHANGE
	tvMsg := readWSMsg(t, tv, 500*time.Millisecond)
	if tvMsg.Action != protocol.ActionBackgroundChange {
		t.Errorf("TV: expected BACKGROUND_CHANGE, got %s", tvMsg.Action)
	}

	// VPlayer must NOT receive BACKGROUND_CHANGE
	expectNoMessage(t, player, 150*time.Millisecond)
}

// ============================================================================
// Tests: Concurrent BroadcastToTypes (thread safety)
// ============================================================================

func TestBroadcastToTypes_ConcurrentSafety(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	admin := dialWSPath(t, srv, "/ws/admin")
	defer admin.Close()

	time.Sleep(50 * time.Millisecond)

	const goroutines = 20
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg, _ := protocol.NewMessage(protocol.ActionUpdate, nil)
			hub.BroadcastToTypes(msg, ClientTypeAdmin)
		}()
	}
	wg.Wait()

	// Drain messages — should not deadlock
	received := 0
	for {
		admin.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		_, _, err := admin.ReadMessage()
		if err != nil {
			break
		}
		received++
		if received >= goroutines {
			break
		}
	}

	if received == 0 {
		t.Error("Expected to receive at least some messages")
	}
}

// ============================================================================
// Tests: PlayerID identity tracking (#109 — icône de déconnexion absente
// pour les VPlayers). Voir plan _work/reports/plan-20260711-160927.md, Phase 1.
// ============================================================================

// clientIDForType returns the ID of a connected client matching the given type.
// Test helper — assumes at least one client of that type is connected and
// returns the first match found.
func clientIDForType(t *testing.T, hub *WebSocketHub, clientType ClientType) string {
	t.Helper()
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	for c := range hub.clients {
		if c.Type == clientType {
			return c.ID
		}
	}
	t.Fatalf("No client of type %s found", clientType)
	return ""
}

func TestSetClientPlayerID_LinksClientToPlayerID(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	defer conn.Close()
	time.Sleep(50 * time.Millisecond)

	clientID := clientIDForType(t, hub, ClientTypeVPlayer)
	hub.SetClientPlayerID(clientID, "vjoueur_Alice_20260711")

	if !hub.IsPlayerIDConnected("vjoueur_Alice_20260711") {
		t.Error("Expected IsPlayerIDConnected to be true after SetClientPlayerID")
	}
}

func TestSetClientPlayerID_UnknownClientID_NoPanic(t *testing.T) {
	_, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	// No client connected at all — must be a safe no-op, not a panic.
	hub.SetClientPlayerID("does-not-exist", "vjoueur_Ghost")

	if hub.IsPlayerIDConnected("vjoueur_Ghost") {
		t.Error("Expected IsPlayerIDConnected to be false — no client was actually linked")
	}
}

func TestIsPlayerIDConnected_FalseWhenNoClient(t *testing.T) {
	_, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	if hub.IsPlayerIDConnected("vjoueur_Unknown") {
		t.Error("Expected false when no client carries this PlayerID")
	}
}

func TestIsPlayerIDConnected_FalseAfterDisconnect(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	time.Sleep(50 * time.Millisecond)

	clientID := clientIDForType(t, hub, ClientTypeVPlayer)
	hub.SetClientPlayerID(clientID, "vjoueur_Bob")

	if !hub.IsPlayerIDConnected("vjoueur_Bob") {
		t.Fatal("Precondition failed: expected connected before closing the WebSocket")
	}

	conn.Close()
	time.Sleep(150 * time.Millisecond)

	if hub.IsPlayerIDConnected("vjoueur_Bob") {
		t.Error("Expected IsPlayerIDConnected to be false after the WebSocket closed")
	}
}

// TestIsPlayerIDConnected_IgnoresNonVPlayerClientType is a defensive test that
// matches the method contract described in the plan: IsPlayerIDConnected must
// only consider ClientTypeVPlayer clients, even if another client type somehow
// carried the same PlayerID value (should not happen in practice — SetClientPlayerID
// is only ever called from the PLAYER_CONNECT handler — but the contract is explicit).
func TestIsPlayerIDConnected_IgnoresNonVPlayerClientType(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/admin")
	defer conn.Close()
	time.Sleep(50 * time.Millisecond)

	clientID := clientIDForType(t, hub, ClientTypeAdmin)
	hub.SetClientPlayerID(clientID, "vjoueur_NotAPlayer")

	if hub.IsPlayerIDConnected("vjoueur_NotAPlayer") {
		t.Error("Expected false — the client carrying this PlayerID is admin, not vplayer")
	}
}

// TestIsPlayerIDConnected_TrueAfterReconnectRace covers the hub-level
// invariant the anti-flash "zombie disconnect" guard depends on (guard itself
// wired in main.go's OnPlayerDisconnected callback, out of scope for this hub
// test): when a new connection has already registered under the same PlayerID
// before the old one unregisters, IsPlayerIDConnected must still report true.
func TestIsPlayerIDConnected_TrueAfterReconnectRace(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	oldConn := dialWSPath(t, srv, "/ws/player")
	time.Sleep(50 * time.Millisecond)
	oldClientID := clientIDForType(t, hub, ClientTypeVPlayer)
	hub.SetClientPlayerID(oldClientID, "vjoueur_Dan")

	// A new connection for the same VJoueur arrives before the old one closes
	// (typical fast tab-refresh / reconnect scenario).
	newConn := dialWSPath(t, srv, "/ws/player")
	defer newConn.Close()
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	var newClientID string
	for c := range hub.clients {
		if c.Type == ClientTypeVPlayer && c.ID != oldClientID {
			newClientID = c.ID
		}
	}
	hub.mu.RUnlock()
	if newClientID == "" {
		t.Fatal("Expected a second VPlayer client to be connected")
	}
	hub.SetClientPlayerID(newClientID, "vjoueur_Dan")

	// The stale connection finally closes.
	oldConn.Close()
	time.Sleep(150 * time.Millisecond)

	if !hub.IsPlayerIDConnected("vjoueur_Dan") {
		t.Error("Expected IsPlayerIDConnected to remain true — the new connection still holds this PlayerID")
	}
}

// ============================================================================
// Tests: SendToPlayerID / SendRawToPlayerID duplicate-PlayerID hardening
// (#129 code review)
// ============================================================================

// TestSendToPlayerID_DuplicateRegistration_ReachesBothConnections reproduces
// the exact real-world window TestIsPlayerIDConnected_TrueAfterReconnectRace
// already exercises for a different method: a fast reconnect where the OLD
// connection hasn't been removed from h.clients yet (its own failure isn't
// detected server-side until a read timeout or failed write, up to a few
// seconds — readPump/writePump) when the NEW connection completes
// PLAYER_CONNECT and links the same PlayerID. Two ClientTypeVPlayer entries
// legitimately share one PlayerID for that window.
//
// Before the #129 hardening, SendToPlayerID returned after the FIRST match
// found via Go's randomized map iteration — non-deterministically landing on
// either the stale or the live connection. A caller needing the live one
// specifically (e.g. a targeted UPDATE echo the reconnecting player depends
// on to recover its session, #118/#120/#122) could silently miss it. This
// test proves the fix: sending to every match means the live connection
// always receives the message, regardless of which one Go's map iteration
// happens to visit first.
func TestSendToPlayerID_DuplicateRegistration_ReachesBothConnections(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	oldConn := dialWSPath(t, srv, "/ws/player")
	defer oldConn.Close()
	time.Sleep(50 * time.Millisecond)
	oldClientID := clientIDForType(t, hub, ClientTypeVPlayer)
	hub.SetClientPlayerID(oldClientID, "vjoueur_Eve")

	// New connection for the same VJoueur arrives before the old one closes.
	newConn := dialWSPath(t, srv, "/ws/player")
	defer newConn.Close()
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	var newClientID string
	for c := range hub.clients {
		if c.Type == ClientTypeVPlayer && c.ID != oldClientID {
			newClientID = c.ID
		}
	}
	hub.mu.RUnlock()
	if newClientID == "" {
		t.Fatal("Expected a second VPlayer client to be connected")
	}
	hub.SetClientPlayerID(newClientID, "vjoueur_Eve")

	msg, err := protocol.NewMessage(protocol.ActionPlayerEvicted, protocol.PlayerEvictedPayload{Reason: "GAME_RESET"})
	if err != nil {
		t.Fatalf("failed to build message: %v", err)
	}
	if err := hub.SendToPlayerID("vjoueur_Eve", msg); err != nil {
		t.Fatalf("SendToPlayerID failed: %v", err)
	}

	oldMsg := readWSMsg(t, oldConn, 500*time.Millisecond)
	if oldMsg == nil || oldMsg.Action != protocol.ActionPlayerEvicted {
		t.Errorf("expected the STALE (old) connection to also receive the message, got %v", oldMsg)
	}
	newMsg := readWSMsg(t, newConn, 500*time.Millisecond)
	if newMsg == nil || newMsg.Action != protocol.ActionPlayerEvicted {
		t.Errorf("expected the LIVE (new) connection to receive the message — this is the one that actually matters, got %v", newMsg)
	}
}

// TestSendRawToPlayerID_DuplicateRegistration_ReachesBothConnections is the
// same scenario for SendRawToPlayerID (#129 T1.1), the raw twin
// broadcastUpdateToPlayer uses for the reconnection echo CA2 depends on.
func TestSendRawToPlayerID_DuplicateRegistration_ReachesBothConnections(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	oldConn := dialWSPath(t, srv, "/ws/player")
	defer oldConn.Close()
	time.Sleep(50 * time.Millisecond)
	oldClientID := clientIDForType(t, hub, ClientTypeVPlayer)
	hub.SetClientPlayerID(oldClientID, "vjoueur_Frank")

	newConn := dialWSPath(t, srv, "/ws/player")
	defer newConn.Close()
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	var newClientID string
	for c := range hub.clients {
		if c.Type == ClientTypeVPlayer && c.ID != oldClientID {
			newClientID = c.ID
		}
	}
	hub.mu.RUnlock()
	if newClientID == "" {
		t.Fatal("Expected a second VPlayer client to be connected")
	}
	hub.SetClientPlayerID(newClientID, "vjoueur_Frank")

	msg, err := protocol.NewMessage(protocol.ActionUpdate, nil)
	if err != nil {
		t.Fatalf("failed to build message: %v", err)
	}
	data, err := msg.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("failed to serialize message: %v", err)
	}
	hub.SendRawToPlayerID("vjoueur_Frank", data)

	oldMsg := readWSMsg(t, oldConn, 500*time.Millisecond)
	if oldMsg == nil || oldMsg.Action != protocol.ActionUpdate {
		t.Errorf("expected the STALE (old) connection to also receive the message, got %v", oldMsg)
	}
	newMsg := readWSMsg(t, newConn, 500*time.Millisecond)
	if newMsg == nil || newMsg.Action != protocol.ActionUpdate {
		t.Errorf("expected the LIVE (new) connection to receive the message — this is the one that actually matters, got %v", newMsg)
	}
}

// TestSendToPlayerID_NoDuplicate_StillWorks is a plain non-regression check:
// the common case (exactly one client for a PlayerID) must keep working
// exactly as before the #129 hardening.
func TestSendToPlayerID_NoDuplicate_StillWorks(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	defer conn.Close()
	time.Sleep(50 * time.Millisecond)
	clientID := clientIDForType(t, hub, ClientTypeVPlayer)
	hub.SetClientPlayerID(clientID, "vjoueur_Gina")

	msg, err := protocol.NewMessage(protocol.ActionPlayerEvicted, protocol.PlayerEvictedPayload{Reason: "PLAYER_REMOVED"})
	if err != nil {
		t.Fatalf("failed to build message: %v", err)
	}
	if err := hub.SendToPlayerID("vjoueur_Gina", msg); err != nil {
		t.Fatalf("SendToPlayerID failed: %v", err)
	}

	got := readWSMsg(t, conn, 500*time.Millisecond)
	if got == nil || got.Action != protocol.ActionPlayerEvicted {
		t.Errorf("expected the single connection to receive the message, got %v", got)
	}
}

// ============================================================================
// Tests: OnPlayerDisconnected callback (#109)
// ============================================================================

func TestOnPlayerDisconnected_FiresForVPlayerWithPlayerID(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	var mu sync.Mutex
	var gotPlayerID string
	fired := make(chan struct{}, 1)
	hub.OnPlayerDisconnected = func(playerID string) {
		mu.Lock()
		gotPlayerID = playerID
		mu.Unlock()
		fired <- struct{}{}
	}

	conn := dialWSPath(t, srv, "/ws/player")
	time.Sleep(50 * time.Millisecond)

	clientID := clientIDForType(t, hub, ClientTypeVPlayer)
	hub.SetClientPlayerID(clientID, "vjoueur_Carla")

	conn.Close()

	select {
	case <-fired:
	case <-time.After(1 * time.Second):
		t.Fatal("Expected OnPlayerDisconnected to fire within 1s of the WebSocket closing")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotPlayerID != "vjoueur_Carla" {
		t.Errorf("Expected callback with playerID=vjoueur_Carla, got %s", gotPlayerID)
	}
}

func TestOnPlayerDisconnected_DoesNotFireForAdmin(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	fired := make(chan struct{}, 1)
	hub.OnPlayerDisconnected = func(playerID string) {
		fired <- struct{}{}
	}

	conn := dialWSPath(t, srv, "/ws/admin")
	time.Sleep(50 * time.Millisecond)
	conn.Close()

	select {
	case <-fired:
		t.Error("OnPlayerDisconnected must not fire for an admin client")
	case <-time.After(300 * time.Millisecond):
		// expected: no callback fired
	}
}

func TestOnPlayerDisconnected_DoesNotFireForTV(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	fired := make(chan struct{}, 1)
	hub.OnPlayerDisconnected = func(playerID string) {
		fired <- struct{}{}
	}

	conn := dialWSPath(t, srv, "/ws/tv")
	time.Sleep(50 * time.Millisecond)
	conn.Close()

	select {
	case <-fired:
		t.Error("OnPlayerDisconnected must not fire for a TV client")
	case <-time.After(300 * time.Millisecond):
		// expected: no callback fired
	}
}

func TestOnPlayerDisconnected_DoesNotFireForVPlayerWithoutPlayerID(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	fired := make(chan struct{}, 1)
	hub.OnPlayerDisconnected = func(playerID string) {
		fired <- struct{}{}
	}

	// Connect as VPlayer but never call SetClientPlayerID — not yet identified
	// (e.g. connection dropped before PLAYER_CONNECT was processed).
	conn := dialWSPath(t, srv, "/ws/player")
	time.Sleep(50 * time.Millisecond)
	conn.Close()

	select {
	case <-fired:
		t.Error("OnPlayerDisconnected must not fire for a VPlayer with an empty PlayerID")
	case <-time.After(300 * time.Millisecond):
		// expected: no callback fired
	}
}

func TestOnPlayerDisconnected_NilCallback_NoPanic(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	// hub.OnPlayerDisconnected intentionally left nil.
	conn := dialWSPath(t, srv, "/ws/player")
	time.Sleep(50 * time.Millisecond)

	clientID := clientIDForType(t, hub, ClientTypeVPlayer)
	hub.SetClientPlayerID(clientID, "vjoueur_NilCallback")

	conn.Close()
	time.Sleep(150 * time.Millisecond) // must not panic with a nil callback
}
