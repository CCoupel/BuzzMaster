package server

import (
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// --- Helper functions for BuzzerWebSocketHub ---

// startTestBuzzerWSServer starts a test HTTP server with a BuzzerWebSocketHub.
// Returns the server, buzzer hub, and a cleanup function.
func startTestBuzzerWSServer(t *testing.T) (*httptest.Server, *BuzzerWebSocketHub, func()) {
	t.Helper()

	hub := NewBuzzerWebSocketHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleConnection(w, r)
	}))

	cleanup := func() {
		server.Close()
	}

	return server, hub, cleanup
}

// dialBuzzerWS connects to the test BuzzerWebSocketHub server without MAC query param.
func dialBuzzerWS(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/buzzer"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect buzzer WebSocket: %v", err)
	}
	return conn
}

// dialBuzzerWSWithMAC connects to the test BuzzerWebSocketHub server with MAC in query param.
func dialBuzzerWSWithMAC(t *testing.T, server *httptest.Server, mac string) *websocket.Conn {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/buzzer?mac=" + mac
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect buzzer WebSocket with MAC: %v", err)
	}
	return conn
}

// sendBuzzerMsg sends a JSON protocol message over WebSocket.
func sendBuzzerMsg(t *testing.T, conn *websocket.Conn, msg *protocol.Message) {
	t.Helper()

	data, err := msg.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("Failed to serialize message: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("Failed to send WebSocket message: %v", err)
	}
}

// readBuzzerMsg reads a JSON protocol message from WebSocket with timeout.
func readBuzzerMsg(t *testing.T, conn *websocket.Conn, timeout time.Duration) *protocol.Message {
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

// ============================================================================
// Tests: BuzzerWebSocketHub - Connection / Disconnection
// ============================================================================

func TestBuzzerWSHub_Connect(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn := dialBuzzerWS(t, server)
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	if hub.BuzzerCount() != 1 {
		t.Errorf("Expected 1 buzzer, got %d", hub.BuzzerCount())
	}
}

func TestBuzzerWSHub_Disconnect(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn := dialBuzzerWS(t, server)

	time.Sleep(100 * time.Millisecond)
	if hub.BuzzerCount() != 1 {
		t.Errorf("Expected 1 buzzer before disconnect, got %d", hub.BuzzerCount())
	}

	conn.Close()
	time.Sleep(200 * time.Millisecond)

	if hub.BuzzerCount() != 0 {
		t.Errorf("Expected 0 buzzers after disconnect, got %d", hub.BuzzerCount())
	}
}

func TestBuzzerWSHub_MultipleBuzzers(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn1 := dialBuzzerWS(t, server)
	defer conn1.Close()
	conn2 := dialBuzzerWS(t, server)
	defer conn2.Close()
	conn3 := dialBuzzerWS(t, server)
	defer conn3.Close()

	time.Sleep(100 * time.Millisecond)

	if hub.BuzzerCount() != 3 {
		t.Errorf("Expected 3 buzzers, got %d", hub.BuzzerCount())
	}
}

// ============================================================================
// Tests: HandleConnection with MAC in query param
// ============================================================================

func TestBuzzerWSHub_HandleConnection_MACQueryParam(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	mac := "AA:BB:CC:DD:EE:01"
	conn := dialBuzzerWSWithMAC(t, server, mac)
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	// Verify GetClients returns the MAC address
	clients := hub.GetClients()
	if len(clients) != 1 {
		t.Fatalf("Expected 1 client, got %d", len(clients))
	}
	if clients[0] != mac {
		t.Errorf("Expected client MAC %s, got %s", mac, clients[0])
	}
}

func TestBuzzerWSHub_HandleConnection_NoMAC_UsesRemoteAddr(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn := dialBuzzerWS(t, server)
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	clients := hub.GetClients()
	if len(clients) != 1 {
		t.Fatalf("Expected 1 client, got %d", len(clients))
	}
	// Without MAC param, ID should be the remote address (non-empty)
	if clients[0] == "" {
		t.Error("Client ID should not be empty when no MAC provided")
	}
}

// ============================================================================
// Tests: HandleConnection with MAC in first HELLO message
// ============================================================================

func TestBuzzerWSHub_MACIdentificationViaHello(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	// Connect without MAC query param
	conn := dialBuzzerWS(t, server)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Send HELLO with MAC in ID field
	mac := "AA:BB:CC:DD:EE:FF"
	helloPayload := protocol.HelloPayload{
		Version: "3.0.0",
		Name:    "TestBuzzer",
	}
	payloadBytes, _ := json.Marshal(helloPayload)
	msg := &protocol.Message{
		Action: protocol.ActionHello,
		ID:     mac,
		Msg:    payloadBytes,
	}
	sendBuzzerMsg(t, conn, msg)

	// Read from Incoming - the readPump should have set the MAC from the message
	select {
	case incoming := <-hub.Incoming:
		if incoming.ClientID != mac {
			t.Errorf("Expected clientID %s (MAC from HELLO), got %s", mac, incoming.ClientID)
		}
		if incoming.Data.ID != mac {
			t.Errorf("Expected message ID %s, got %s", mac, incoming.Data.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for HELLO message")
	}

	// Verify GetClients now returns the MAC
	time.Sleep(50 * time.Millisecond)
	clients := hub.GetClients()
	if len(clients) != 1 {
		t.Fatalf("Expected 1 client, got %d", len(clients))
	}
	if clients[0] != mac {
		t.Errorf("Expected client MAC %s after HELLO, got %s", mac, clients[0])
	}
}

// ============================================================================
// Tests: readPump - JSON Message Parsing
// ============================================================================

func TestBuzzerWSHub_ReadPump_HelloMessage(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:01")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	helloPayload := protocol.HelloPayload{
		Version: "3.0.0",
		Name:    "Buzzer-1",
	}
	payloadBytes, _ := json.Marshal(helloPayload)
	msg := &protocol.Message{
		Action: protocol.ActionHello,
		ID:     "AA:BB:CC:DD:EE:01",
		Msg:    payloadBytes,
	}
	sendBuzzerMsg(t, conn, msg)

	select {
	case incoming := <-hub.Incoming:
		if incoming.Data.Action != protocol.ActionHello {
			t.Errorf("Expected action HELLO, got %s", incoming.Data.Action)
		}
		if incoming.Data.ID != "AA:BB:CC:DD:EE:01" {
			t.Errorf("Expected ID AA:BB:CC:DD:EE:01, got %s", incoming.Data.ID)
		}
		if incoming.Source != "WebSocket-Buzzer" {
			t.Errorf("Expected source 'WebSocket-Buzzer', got '%s'", incoming.Source)
		}
		if incoming.ClientID != "AA:BB:CC:DD:EE:01" {
			t.Errorf("Expected clientID AA:BB:CC:DD:EE:01, got %s", incoming.ClientID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for HELLO message")
	}
}

func TestBuzzerWSHub_ReadPump_ButtonMessage(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:01")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	buttonPayload := protocol.ButtonPayload{
		Button: "A",
	}
	payloadBytes, _ := json.Marshal(buttonPayload)
	pressTime := time.Now().UnixMicro()
	msg := &protocol.Message{
		Action:    protocol.ActionButton,
		ID:        "AA:BB:CC:DD:EE:01",
		TimeEvent: pressTime,
		Msg:       payloadBytes,
	}
	sendBuzzerMsg(t, conn, msg)

	select {
	case incoming := <-hub.Incoming:
		if incoming.Data.Action != protocol.ActionButton {
			t.Errorf("Expected action BUTTON, got %s", incoming.Data.Action)
		}
		if incoming.Data.TimeEvent == 0 {
			t.Error("Expected TimeEvent to be set")
		}
		bp, err := incoming.Data.ParseButtonPayload()
		if err != nil {
			t.Fatalf("Failed to parse button payload: %v", err)
		}
		if bp.Button != "A" {
			t.Errorf("Expected button A, got %s", bp.Button)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for BUTTON message")
	}
}

func TestBuzzerWSHub_ReadPump_PongMessage(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:01")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	msg := &protocol.Message{
		Action: protocol.ActionPong,
		ID:     "AA:BB:CC:DD:EE:01",
	}
	sendBuzzerMsg(t, conn, msg)

	select {
	case incoming := <-hub.Incoming:
		if incoming.Data.Action != protocol.ActionPong {
			t.Errorf("Expected action PONG, got %s", incoming.Data.Action)
		}
		if incoming.ClientID != "AA:BB:CC:DD:EE:01" {
			t.Errorf("Expected clientID AA:BB:CC:DD:EE:01, got %s", incoming.ClientID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for PONG message")
	}
}

func TestBuzzerWSHub_ReadPump_MultipleMessages(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:01")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Send 3 messages rapidly
	actions := []string{protocol.ActionHello, protocol.ActionPong, protocol.ActionButton}
	for i, action := range actions {
		msg := &protocol.Message{
			Action: action,
			ID:     "AA:BB:CC:DD:EE:01",
			Seq:    i + 1,
		}
		sendBuzzerMsg(t, conn, msg)
	}

	// Receive all 3
	receivedCount := 0
	timeout := time.After(2 * time.Second)
	for receivedCount < 3 {
		select {
		case <-hub.Incoming:
			receivedCount++
		case <-timeout:
			t.Fatalf("Timeout: only received %d/3 messages", receivedCount)
		}
	}
}

func TestBuzzerWSHub_ReadPump_SourceIsWebSocketBuzzer(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:01")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	msg := &protocol.Message{
		Action: protocol.ActionButton,
		ID:     "AA:BB:CC:DD:EE:01",
	}
	sendBuzzerMsg(t, conn, msg)

	select {
	case incoming := <-hub.Incoming:
		if incoming.Source != "WebSocket-Buzzer" {
			t.Errorf("Expected source 'WebSocket-Buzzer', got '%s'", incoming.Source)
		}
		if incoming.Timestamp.IsZero() {
			t.Error("Expected timestamp to be set")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

// ============================================================================
// Tests: writePump - Sending Messages to Buzzers
// ============================================================================

func TestBuzzerWSHub_SendToClient_ByMAC(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	mac := "AA:BB:CC:DD:EE:01"
	conn := dialBuzzerWSWithMAC(t, server, mac)
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	// Send PING to buzzer by MAC (like LED command)
	msg, err := protocol.NewMessage(protocol.ActionPing, nil)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	err = hub.SendToClient(mac, msg)
	if err != nil {
		t.Fatalf("SendToClient failed: %v", err)
	}

	// Client should receive the message
	received := readBuzzerMsg(t, conn, 2*time.Second)
	if received.Action != protocol.ActionPing {
		t.Errorf("Expected PING action, got %s", received.Action)
	}
}

func TestBuzzerWSHub_SendToClient_ByClientID(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	// Connect without MAC (uses remote addr as ID)
	conn := dialBuzzerWS(t, server)
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	// Get the client ID
	clients := hub.GetClients()
	if len(clients) != 1 {
		t.Fatalf("Expected 1 client, got %d", len(clients))
	}
	clientID := clients[0]

	msg, err := protocol.NewMessage(protocol.ActionPing, nil)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	err = hub.SendToClient(clientID, msg)
	if err != nil {
		t.Fatalf("SendToClient failed: %v", err)
	}

	received := readBuzzerMsg(t, conn, 2*time.Second)
	if received.Action != protocol.ActionPing {
		t.Errorf("Expected PING action, got %s", received.Action)
	}
}

func TestBuzzerWSHub_Broadcast(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn1 := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:01")
	defer conn1.Close()
	conn2 := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:02")
	defer conn2.Close()

	time.Sleep(100 * time.Millisecond)

	msg, err := protocol.NewMessage(protocol.ActionStart, nil)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	hub.Broadcast(msg)

	// Both buzzers should receive
	msg1 := readBuzzerMsg(t, conn1, 2*time.Second)
	msg2 := readBuzzerMsg(t, conn2, 2*time.Second)

	if msg1.Action != protocol.ActionStart {
		t.Errorf("Buzzer 1 expected START, got %s", msg1.Action)
	}
	if msg2.Action != protocol.ActionStart {
		t.Errorf("Buzzer 2 expected START, got %s", msg2.Action)
	}
}

func TestBuzzerWSHub_BroadcastRaw(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:01")
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	rawMsg := []byte(`{"ACTION":"STOP","MSG":{}}`)
	hub.BroadcastRaw(rawMsg)

	received := readBuzzerMsg(t, conn, 2*time.Second)
	if received.Action != protocol.ActionStop {
		t.Errorf("Expected STOP action, got %s", received.Action)
	}
}

// ============================================================================
// Tests: SetClientMAC
// ============================================================================

func TestBuzzerWSHub_SetClientMAC(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	// Connect without MAC
	conn := dialBuzzerWS(t, server)
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	// Get the initial client ID (remote addr)
	clients := hub.GetClients()
	if len(clients) != 1 {
		t.Fatalf("Expected 1 client, got %d", len(clients))
	}
	initialID := clients[0]

	// Set MAC for the client
	newMAC := "AA:BB:CC:DD:EE:FF"
	hub.SetClientMAC(initialID, newMAC)

	// GetClients should now return the MAC
	clients = hub.GetClients()
	if len(clients) != 1 {
		t.Fatalf("Expected 1 client after SetClientMAC, got %d", len(clients))
	}
	if clients[0] != newMAC {
		t.Errorf("Expected client MAC %s after SetClientMAC, got %s", newMAC, clients[0])
	}

	// SendToClient by MAC should work
	msg, _ := protocol.NewMessage(protocol.ActionPing, nil)
	err := hub.SendToClient(newMAC, msg)
	if err != nil {
		t.Fatalf("SendToClient by new MAC failed: %v", err)
	}

	received := readBuzzerMsg(t, conn, 2*time.Second)
	if received.Action != protocol.ActionPing {
		t.Errorf("Expected PING, got %s", received.Action)
	}
}

func TestBuzzerWSHub_SetClientMAC_UnknownClient(t *testing.T) {
	_, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	// Should not panic when setting MAC for unknown client
	hub.SetClientMAC("nonexistent-id", "AA:BB:CC:DD:EE:FF")

	// No clients should exist
	clients := hub.GetClients()
	if len(clients) != 0 {
		t.Errorf("Expected 0 clients, got %d", len(clients))
	}
}

// ============================================================================
// Tests: GetClients
// ============================================================================

func TestBuzzerWSHub_GetClients_Empty(t *testing.T) {
	_, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	clients := hub.GetClients()
	if len(clients) != 0 {
		t.Errorf("Expected 0 clients initially, got %d", len(clients))
	}
}

func TestBuzzerWSHub_GetClients_ReturnsMACs(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	mac1 := "AA:BB:CC:DD:EE:01"
	mac2 := "AA:BB:CC:DD:EE:02"

	conn1 := dialBuzzerWSWithMAC(t, server, mac1)
	defer conn1.Close()
	conn2 := dialBuzzerWSWithMAC(t, server, mac2)
	defer conn2.Close()

	time.Sleep(100 * time.Millisecond)

	clients := hub.GetClients()
	if len(clients) != 2 {
		t.Fatalf("Expected 2 clients, got %d", len(clients))
	}

	// Both MACs should be in the list (order not guaranteed)
	foundMAC1, foundMAC2 := false, false
	for _, c := range clients {
		if c == mac1 {
			foundMAC1 = true
		}
		if c == mac2 {
			foundMAC2 = true
		}
	}
	if !foundMAC1 {
		t.Errorf("MAC %s not found in clients list", mac1)
	}
	if !foundMAC2 {
		t.Errorf("MAC %s not found in clients list", mac2)
	}
}

func TestBuzzerWSHub_GetClients_FallsBackToID(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	// Connect without MAC
	conn := dialBuzzerWS(t, server)
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	clients := hub.GetClients()
	if len(clients) != 1 {
		t.Fatalf("Expected 1 client, got %d", len(clients))
	}
	// Without MAC, should fall back to non-empty ID (remote address)
	if clients[0] == "" {
		t.Error("Client ID should not be empty when no MAC provided")
	}
}

// ============================================================================
// Tests: OnBuzzerChange Callback
// ============================================================================

func TestBuzzerWSHub_OnBuzzerChange_Connect(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	var mu sync.Mutex
	var callCount int
	var lastBuzzerCount int

	hub.OnBuzzerChange = func(count int) {
		mu.Lock()
		callCount++
		lastBuzzerCount = count
		mu.Unlock()
	}

	conn := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:01")
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	cc := callCount
	lbc := lastBuzzerCount
	mu.Unlock()

	if cc == 0 {
		t.Error("OnBuzzerChange should have been called on connect")
	}
	if lbc != 1 {
		t.Errorf("Expected buzzer count 1 after connect, got %d", lbc)
	}
}

func TestBuzzerWSHub_OnBuzzerChange_Disconnect(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	var mu sync.Mutex
	var lastBuzzerCount int

	hub.OnBuzzerChange = func(count int) {
		mu.Lock()
		lastBuzzerCount = count
		mu.Unlock()
	}

	conn := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:01")
	time.Sleep(100 * time.Millisecond)

	conn.Close()
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	lbc := lastBuzzerCount
	mu.Unlock()

	if lbc != 0 {
		t.Errorf("Expected buzzer count 0 after disconnect, got %d", lbc)
	}
}

func TestBuzzerWSHub_OnBuzzerChange_MultipleConnects(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	var mu sync.Mutex
	var counts []int

	hub.OnBuzzerChange = func(count int) {
		mu.Lock()
		counts = append(counts, count)
		mu.Unlock()
	}

	conn1 := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:01")
	defer conn1.Close()
	time.Sleep(100 * time.Millisecond)

	conn2 := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:02")
	defer conn2.Close()
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	cc := len(counts)
	mu.Unlock()

	if cc < 2 {
		t.Fatalf("Expected at least 2 callback calls, got %d", cc)
	}

	mu.Lock()
	// Last count should be 2
	lastCount := counts[len(counts)-1]
	mu.Unlock()

	if lastCount != 2 {
		t.Errorf("Expected last count 2, got %d", lastCount)
	}
}

// ============================================================================
// Tests: Concurrent Connections (Stress Test)
// ============================================================================

func TestBuzzerWSHub_ConcurrentConnections(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	const numBuzzers = 10
	var wg sync.WaitGroup
	conns := make([]*websocket.Conn, numBuzzers)

	for i := 0; i < numBuzzers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mac := "AA:BB:CC:DD:EE:" + string(rune('0'+idx/10)) + string(rune('0'+idx%10))
			conn := dialBuzzerWSWithMAC(t, server, mac)
			conns[idx] = conn
		}(i)
	}
	wg.Wait()

	time.Sleep(200 * time.Millisecond)

	if hub.BuzzerCount() != numBuzzers {
		t.Errorf("Expected %d buzzers, got %d", numBuzzers, hub.BuzzerCount())
	}

	// Clean up
	for _, conn := range conns {
		if conn != nil {
			conn.Close()
		}
	}
}

func TestBuzzerWSHub_ConcurrentBroadcast(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:01")
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	// Broadcast concurrently (tests thread safety)
	const numBroadcasts = 50
	var wg sync.WaitGroup
	for i := 0; i < numBroadcasts; i++ {
		wg.Add(1)
		go func(seq int) {
			defer wg.Done()
			msg, _ := protocol.NewMessage(protocol.ActionUpdate, nil)
			msg.Seq = seq
			hub.Broadcast(msg)
		}(i)
	}
	wg.Wait()

	// Read at least some messages (verify no panic/deadlock)
	receivedCount := 0
	for {
		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
		receivedCount++
		if receivedCount >= numBroadcasts {
			break
		}
	}

	if receivedCount == 0 {
		t.Error("Should have received at least some broadcast messages")
	}
}

// ============================================================================
// Tests: Incoming Channel Capacity
// ============================================================================

func TestBuzzerWSHub_IncomingChannelFull(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:01")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Send 105 messages without draining (channel capacity is 100)
	for i := 0; i < 105; i++ {
		msg := &protocol.Message{
			Action: protocol.ActionButton,
			ID:     "AA:BB:CC:DD:EE:01",
			Seq:    i,
		}
		sendBuzzerMsg(t, conn, msg)
		time.Sleep(5 * time.Millisecond)
	}

	// Drain and count - should not deadlock
	drainCount := 0
	timeout := time.After(3 * time.Second)
	for {
		select {
		case <-hub.Incoming:
			drainCount++
		case <-timeout:
			if drainCount < 50 {
				t.Errorf("Expected at least 50 messages, got %d", drainCount)
			}
			return
		}
	}
}

// ============================================================================
// Tests: Send Channel Full - Client Removal
// ============================================================================

func TestBuzzerWSHub_SendChannelFull_ClientRemoved(t *testing.T) {
	server, hub, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn := dialBuzzerWSWithMAC(t, server, "AA:BB:CC:DD:EE:01")
	// Don't read from conn - let send channel fill up

	time.Sleep(100 * time.Millisecond)

	if hub.BuzzerCount() != 1 {
		t.Fatalf("Expected 1 buzzer, got %d", hub.BuzzerCount())
	}

	// Flood broadcasts (send channel capacity is 256)
	for i := 0; i < 300; i++ {
		msg, _ := protocol.NewMessage(protocol.ActionUpdate, nil)
		hub.Broadcast(msg)
	}

	time.Sleep(500 * time.Millisecond)

	conn.Close()
	time.Sleep(200 * time.Millisecond)

	if hub.BuzzerCount() != 0 {
		t.Errorf("Expected 0 buzzers after flood + close, got %d", hub.BuzzerCount())
	}
}

// ============================================================================
// Tests: Message Serialization (TCP vs WebSocket format)
// ============================================================================

func TestBuzzerMessage_SerializeForWebSocket_NoNullTerminator(t *testing.T) {
	msg, err := protocol.NewMessage(protocol.ActionPing, nil)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	data, err := msg.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	// WebSocket buzzer messages should NOT have null terminator
	if len(data) > 0 && data[len(data)-1] == 0 {
		t.Error("WebSocket serialization should NOT have null terminator")
	}

	// Should be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("WebSocket serialized message is not valid JSON: %v", err)
	}
}

func TestBuzzerMessage_SerializeForTCP_HasNullTerminator(t *testing.T) {
	msg, err := protocol.NewMessage(protocol.ActionPing, nil)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	data, err := msg.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	// TCP buzzer messages MUST have null terminator
	if len(data) == 0 || data[len(data)-1] != 0 {
		t.Error("TCP serialization MUST have null terminator")
	}
}

// ============================================================================
// Tests: ParseSingle (Protocol Compatibility)
// ============================================================================

func TestBuzzerParseSingle_ValidJSON(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		action string
		id     string
	}{
		{
			name:   "simple hello",
			input:  `{"ACTION":"HELLO","ID":"AA:BB:CC:DD:EE:01"}`,
			action: "HELLO",
			id:     "AA:BB:CC:DD:EE:01",
		},
		{
			name:   "button press with payload",
			input:  `{"ACTION":"BUTTON","ID":"AA:BB:CC:DD:EE:01","MSG":{"button":"A"}}`,
			action: "BUTTON",
			id:     "AA:BB:CC:DD:EE:01",
		},
		{
			name:   "pong response",
			input:  `{"ACTION":"PONG","ID":"AA:BB:CC:DD:EE:01"}`,
			action: "PONG",
			id:     "AA:BB:CC:DD:EE:01",
		},
		{
			name:   "with null terminator (TCP compat)",
			input:  `{"ACTION":"HELLO","ID":"test"}` + "\x00",
			action: "HELLO",
			id:     "test",
		},
		{
			name:   "with newline and null (TCP compat)",
			input:  `{"ACTION":"HELLO","ID":"test"}` + "\n\x00",
			action: "HELLO",
			id:     "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := protocol.ParseSingle([]byte(tt.input))
			if err != nil {
				t.Fatalf("ParseSingle failed: %v", err)
			}
			if msg.Action != tt.action {
				t.Errorf("Expected action %s, got %s", tt.action, msg.Action)
			}
			if msg.ID != tt.id {
				t.Errorf("Expected ID %s, got %s", tt.id, msg.ID)
			}
		})
	}
}

func TestBuzzerParseSingle_InvalidJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"only whitespace", "   "},
		{"only null", "\x00"},
		{"invalid json", "{broken}"},
		{"incomplete json", `{"ACTION":`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := protocol.ParseSingle([]byte(tt.input))
			if err == nil {
				t.Error("Expected error for invalid input")
			}
		})
	}
}

// ============================================================================
// Tests: BroadcastIfRelevant — Buzzer Payload Whitelist (#41)
// ============================================================================

// TestBuzzerActionWhitelist verifies the whitelist contains exactly the right actions.
func TestBuzzerActionWhitelist(t *testing.T) {
	// Actions that MUST be in the whitelist (buzzers need game-state + control actions).
	expectedAllowed := []string{
		// Game-state actions — buzzer state machine synchronisation
		protocol.ActionPing,
		protocol.ActionUpdate,
		protocol.ActionUpdateTimer,
		protocol.ActionStart,
		protocol.ActionContinue,
		protocol.ActionStop,
		protocol.ActionPause,
		protocol.ActionReady,
		protocol.ActionReset,
		// Control actions
		protocol.ActionHello,
		protocol.ActionLEDSet,
		protocol.ActionOTAUpdate,
		protocol.ActionWifiConfig,
	}
	for _, action := range expectedAllowed {
		if !buzzerActionWhitelist[action] {
			t.Errorf("action %q should be in buzzer whitelist", action)
		}
	}

	// Actions that must NOT reach physical buzzers (web-client / admin-only messages).
	expectedBlocked := []string{
		protocol.ActionReveal,
		protocol.ActionQuestions,
		protocol.ActionClients,
		protocol.ActionFirmwareVersion,
		protocol.ActionRemote,
		protocol.ActionBackgroundChange,
		protocol.ActionConfigUpdate,
	}
	for _, action := range expectedBlocked {
		if buzzerActionWhitelist[action] {
			t.Errorf("action %q should NOT be in buzzer whitelist", action)
		}
	}
}

// TestBroadcastIfRelevant_DropsNonWhitelisted verifies that non-whitelisted actions
// are silently dropped and never queued on the broadcast channel.
func TestBroadcastIfRelevant_DropsNonWhitelisted(t *testing.T) {
	droppedActions := []string{
		protocol.ActionReveal,
		protocol.ActionQuestions,
		protocol.ActionClients,
		protocol.ActionBackgroundChange,
		protocol.ActionConfigUpdate,
	}

	for _, action := range droppedActions {
		t.Run("drops_"+action, func(t *testing.T) {
			hub := NewBuzzerWebSocketHub()
			msg, _ := protocol.NewMessage(action, nil)

			hub.BroadcastIfRelevant(msg)

			// broadcast channel must remain empty
			select {
			case <-hub.broadcast:
				t.Errorf("action %q should have been dropped but was queued", action)
			default:
				// correct: nothing queued
			}
		})
	}
}

// ============================================================================
// Tests: BroadcastRawIfRelevant — pre-serialized bytes, whitelist filtered (#41)
// ============================================================================

// TestBroadcastRawIfRelevant_DropsNonWhitelisted verifies that non-whitelisted actions
// cause BroadcastRawIfRelevant to silently drop the raw payload.
func TestBroadcastRawIfRelevant_DropsNonWhitelisted(t *testing.T) {
	droppedActions := []string{
		protocol.ActionReveal,
		protocol.ActionQuestions,
		protocol.ActionClients,
		protocol.ActionFirmwareVersion,
		protocol.ActionBackgroundChange,
	}

	for _, action := range droppedActions {
		t.Run("drops_raw_"+action, func(t *testing.T) {
			hub := NewBuzzerWebSocketHub()
			rawData := []byte(`{"ACTION":"` + action + `","MSG":{}}`)

			hub.BroadcastRawIfRelevant(action, rawData)

			select {
			case <-hub.broadcast:
				t.Errorf("raw action %q should have been dropped but was queued", action)
			default:
				// correct: nothing queued
			}
		})
	}
}

// TestBroadcastRawIfRelevant_AllowsWhitelisted verifies that whitelisted actions
// cause the raw payload to be queued on the broadcast channel.
func TestBroadcastRawIfRelevant_AllowsWhitelisted(t *testing.T) {
	allowedActions := []string{
		protocol.ActionUpdate,
		protocol.ActionUpdateTimer,
		protocol.ActionStart,
		protocol.ActionContinue,
		protocol.ActionStop,
		protocol.ActionPause,
		protocol.ActionReady,
		protocol.ActionReset,
		protocol.ActionLEDSet,
		protocol.ActionOTAUpdate,
		protocol.ActionWifiConfig,
		protocol.ActionHello,
	}

	for _, action := range allowedActions {
		t.Run("allows_raw_"+action, func(t *testing.T) {
			hub := NewBuzzerWebSocketHub()
			rawData := []byte(`{"ACTION":"` + action + `","MSG":{}}`)

			hub.BroadcastRawIfRelevant(action, rawData)

			select {
			case got := <-hub.broadcast:
				if string(got) != string(rawData) {
					t.Errorf("raw action %q: expected %s, got %s", action, rawData, got)
				}
			default:
				t.Errorf("raw action %q should have been queued but was dropped", action)
			}
		})
	}
}

// TestBroadcastIfRelevant_AllowsWhitelisted verifies that whitelisted actions
// are forwarded to the broadcast channel.
func TestBroadcastIfRelevant_AllowsWhitelisted(t *testing.T) {
	allowedActions := []string{
		// Game-state actions
		protocol.ActionUpdate,
		protocol.ActionUpdateTimer,
		protocol.ActionStart,
		protocol.ActionContinue,
		protocol.ActionStop,
		protocol.ActionPause,
		protocol.ActionReady,
		protocol.ActionReset,
		// Control actions
		protocol.ActionLEDSet,
		protocol.ActionOTAUpdate,
		protocol.ActionWifiConfig,
		protocol.ActionHello,
	}

	for _, action := range allowedActions {
		t.Run("allows_"+action, func(t *testing.T) {
			hub := NewBuzzerWebSocketHub()
			msg, _ := protocol.NewMessage(action, map[string]interface{}{})

			hub.BroadcastIfRelevant(msg)

			select {
			case data := <-hub.broadcast:
				if len(data) == 0 {
					t.Errorf("action %q: expected non-empty serialized message", action)
				}
			default:
				t.Errorf("action %q should have been queued but was dropped", action)
			}
		})
	}
}
