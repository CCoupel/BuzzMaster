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
