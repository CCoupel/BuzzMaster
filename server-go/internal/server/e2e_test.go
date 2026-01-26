package server

import (
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// MockBuzzer simulates a BuzzClick v1 buzzer client
type MockBuzzer struct {
	ID     string
	Team   string
	conn   net.Conn
	parser *protocol.Parser
}

// NewMockBuzzer creates a mock buzzer client
func NewMockBuzzer(id, team string) *MockBuzzer {
	return &MockBuzzer{
		ID:     id,
		Team:   team,
		parser: protocol.NewParser(),
	}
}

// Connect connects to the TCP server
func (b *MockBuzzer) Connect(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	b.conn = conn
	return nil
}

// SendHello sends a HELLO message
func (b *MockBuzzer) SendHello() error {
	msg := `{"ACTION":"HELLO","ID":"` + b.ID + `","MSG":{"VERSION":"1.0.0","TEAM":"` + b.Team + `"}}` + "\x00"
	_, err := b.conn.Write([]byte(msg))
	return err
}

// SendButton sends a BUTTON press
func (b *MockBuzzer) SendButton(button string) error {
	msg := fmt.Sprintf(`{"ACTION":"BUTTON","ID":"%s","MSG":{"button":"%s"},"TIME_EVENT":%d}`, b.ID, button, time.Now().UnixMicro()) + "\x00"
	_, err := b.conn.Write([]byte(msg))
	return err
}

// SendPong sends a PONG response
func (b *MockBuzzer) SendPong() error {
	msg := `{"ACTION":"PONG","ID":"` + b.ID + `"}` + "\x00"
	_, err := b.conn.Write([]byte(msg))
	return err
}

// ReadMessage reads and parses a message
func (b *MockBuzzer) ReadMessage(timeout time.Duration) (*protocol.Message, error) {
	b.conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 4096)
	n, err := b.conn.Read(buf)
	if err != nil {
		return nil, err
	}

	b.parser.Append(buf[:n])
	messages, err := b.parser.Parse()
	if err != nil {
		return nil, err
	}

	if len(messages) > 0 {
		return messages[0], nil
	}
	return nil, nil
}

// Close disconnects the buzzer
func (b *MockBuzzer) Close() {
	if b.conn != nil {
		b.conn.Close()
	}
}

// E2E Test: Single buzzer game flow (avoids localhost IP conflict)
func TestE2E_SingleBuzzerGameFlow(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			HTTPPort: 0,
			TCPPort:  0,
		},
		Storage: config.StorageConfig{
			DataDir:      t.TempDir(),
			QuestionsDir: t.TempDir(),
		},
		Game: config.GameConfig{
			DefaultDelay: 30,
		},
		Version: "2.0.0-test",
	}
	config.SetInstance(cfg)

	engine := game.NewEngine()
	tcpServer := NewTCPServer(0)

	if err := tcpServer.Start(); err != nil {
		t.Fatalf("Failed to start TCP server: %v", err)
	}
	defer tcpServer.Stop()

	tcpAddr := tcpServer.listener.Addr().String()

	// Handle incoming messages
	go func() {
		for msg := range tcpServer.Incoming {
			switch msg.Data.Action {
			case protocol.ActionHello:
				var payload map[string]interface{}
				json.Unmarshal(msg.Data.Msg, &payload)
				engine.UpdateBumper(msg.ClientID, payload)
			case protocol.ActionPong:
				if engine.IsGamePrepare() {
					engine.SetBumperReady(msg.ClientID)
				}
			case protocol.ActionButton:
				engine.ProcessButtonPress(msg.ClientID, msg.Timestamp.UnixMicro(), "A")
			}
		}
	}()

	// Create team
	engine.SetTeams(map[string]*game.Team{
		"red": {Name: "Team Red"},
	})

	// Create single mock buzzer
	buzzer := NewMockBuzzer("MAC-001", "red")
	defer buzzer.Close()

	if err := buzzer.Connect(tcpAddr); err != nil {
		t.Fatalf("Buzzer connect failed: %v", err)
	}

	if err := buzzer.SendHello(); err != nil {
		t.Fatalf("Buzzer hello failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify buzzer registered
	b := engine.GetBumper("MAC-001")
	if b == nil {
		t.Fatal("Buzzer not registered")
	}

	// Start game preparation
	engine.Ready("q1", &game.Question{ID: "q1", Answer: "42"})

	if !engine.IsGamePrepare() {
		t.Error("Game should be in PREPARE phase")
	}

	// Send PONG
	if err := buzzer.SendPong(); err != nil {
		t.Fatalf("Buzzer pong failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Check if ready
	if !engine.AreAllTeamsReady() {
		t.Error("Team should be ready")
	}

	// Transition to READY
	engine.TransitionToReady()
	if !engine.IsGameReady() {
		t.Error("Game should be in READY phase")
	}

	// Start the game
	engine.Start(30)
	if !engine.IsGameStarted() {
		t.Error("Game should be started")
	}

	// Buzzer presses button
	if err := buzzer.SendButton("A"); err != nil {
		t.Fatalf("Buzzer button failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify button press registered
	b = engine.GetBumper("MAC-001")
	if b.Time == 0 {
		t.Error("Button press time should be recorded")
	}

	// Stop game
	engine.Stop()
	if !engine.IsGameStopped() {
		t.Error("Game should be stopped")
	}

	// Update scores
	engine.UpdateScore("MAC-001", 10)
	b = engine.GetBumper("MAC-001")
	if b.Score != 10 {
		t.Errorf("Expected score 10, got %d", b.Score)
	}
}

// E2E Test: WebSocket client communication
func TestE2E_WebSocketClient(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			HTTPPort: 0,
		},
		Storage: config.StorageConfig{
			DataDir: t.TempDir(),
		},
		Version: "2.0.0",
	}
	config.SetInstance(cfg)

	engine := game.NewEngine()
	wsHub := NewWebSocketHub()
	logsHub := NewLogsWebSocketHub(100)
	httpServer := NewHTTPServer(0, engine, wsHub, logsHub)

	go wsHub.Run()
	go logsHub.Run()
	httpServer.setupRoutes()

	// Create test server
	server := httptest.NewServer(httpServer.mux)
	defer server.Close()

	// Convert HTTP to WS URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect WebSocket client
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer ws.Close()

	// Wait for connection to register
	time.Sleep(100 * time.Millisecond)

	// Verify client count
	if wsHub.ClientCount() != 1 {
		t.Errorf("Expected 1 WebSocket client, got %d", wsHub.ClientCount())
	}

	// Send HELLO message
	helloMsg := protocol.Message{Action: protocol.ActionHello}
	ws.WriteJSON(helloMsg)

	// Hub should receive the message
	select {
	case incoming := <-wsHub.Incoming:
		if incoming.Data.Action != protocol.ActionHello {
			t.Errorf("Expected HELLO, got %s", incoming.Data.Action)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for WebSocket message")
	}

	// Broadcast from hub
	msg, _ := protocol.NewMessage(protocol.ActionUpdate, nil)
	wsHub.Broadcast(msg)

	// Client should receive
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read broadcast: %v", err)
	}

	var received protocol.Message
	if err := json.Unmarshal(data, &received); err != nil {
		t.Fatalf("Failed to parse broadcast: %v", err)
	}

	if received.Action != protocol.ActionUpdate {
		t.Errorf("Expected UPDATE, got %s", received.Action)
	}
}

// NOTE: Multi-buzzer race condition test skipped because TCP server
// removes duplicate IP connections (all localhost connections appear as same IP).
// This test would require real network devices or a modified TCP server for testing.

// E2E Test: Game state machine flow
func TestE2E_GameStateMachine(t *testing.T) {
	engine := game.NewEngine()

	// Track state changes
	var stateChanges []game.GamePhase
	engine.OnStateChange = func(phase game.GamePhase) {
		stateChanges = append(stateChanges, phase)
	}

	// Initial state
	if !engine.IsGameStopped() {
		t.Error("Should start in STOP phase")
	}

	// STOP -> PREPARE
	engine.Ready("q1", nil)
	if !engine.IsGamePrepare() {
		t.Error("Should be in PREPARE phase")
	}

	// PREPARE -> READY (manual transition)
	engine.TransitionToReady()
	if !engine.IsGameReady() {
		t.Error("Should be in READY phase")
	}

	// READY -> START
	engine.Start(10)
	if !engine.IsGameStarted() {
		t.Error("Should be in START phase")
	}

	// START -> PAUSE
	engine.Pause()
	if engine.GetPhase() != game.PhasePaused {
		t.Error("Should be in PAUSED phase")
	}

	// PAUSE -> START (continue)
	engine.Continue()
	if !engine.IsGameStarted() {
		t.Error("Should be back in START phase")
	}

	// START -> STOP
	engine.Stop()
	if !engine.IsGameStopped() {
		t.Error("Should be in STOP phase")
	}

	// Verify state change sequence
	expectedSequence := []game.GamePhase{
		game.PhasePrepare,
		game.PhaseReady,
		game.PhaseCountdown,
		game.PhasePaused,
		game.PhaseStarted,
		game.PhaseStopped,
	}

	if len(stateChanges) != len(expectedSequence) {
		t.Errorf("Expected %d state changes, got %d", len(expectedSequence), len(stateChanges))
	}

	for i, expected := range expectedSequence {
		if i < len(stateChanges) && stateChanges[i] != expected {
			t.Errorf("State change %d: expected %s, got %s", i, expected, stateChanges[i])
		}
	}
}

// E2E Test: HTTP API with game engine
func TestE2E_HTTPWithEngine(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			HTTPPort: 0,
		},
		Storage: config.StorageConfig{
			DataDir: t.TempDir(),
		},
		Version: "2.0.0",
	}
	config.SetInstance(cfg)

	engine := game.NewEngine()
	wsHub := NewWebSocketHub()
	logsHub := NewLogsWebSocketHub(100)
	httpServer := NewHTTPServer(0, engine, wsHub, logsHub)

	go wsHub.Run()
	go logsHub.Run()
	httpServer.setupRoutes()

	// Add data to engine
	engine.SetTeams(map[string]*game.Team{
		"red":  {Name: "Team Red", Score: 100},
		"blue": {Name: "Team Blue", Score: 50},
	})
	engine.UpdateBumper("b1", map[string]interface{}{
		"NAME": "Buzzer 1",
		"TEAM": "red",
	})

	// Test /listGame returns engine data
	req := httptest.NewRequest("GET", "/listGame", nil)
	w := httptest.NewRecorder()
	httpServer.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var data map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &data)

	teams, ok := data["teams"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected teams in response")
	}

	if len(teams) != 2 {
		t.Errorf("Expected 2 teams, got %d", len(teams))
	}

	// Test /clearBuzzers
	req = httptest.NewRequest("GET", "/clearBuzzers", nil)
	w = httptest.NewRecorder()
	httpServer.mux.ServeHTTP(w, req)

	// After clear, engine should have no bumpers
	if engine.GetBumper("b1") != nil {
		t.Error("Bumpers should be cleared")
	}
}
