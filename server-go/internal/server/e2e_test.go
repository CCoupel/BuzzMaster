package server

import (
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

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
	// Defensive isolation from the tracked fixture (bugfix #143), even though
	// this test only calls config.SetInstance (no disk I/O today) — see
	// setupTestHTTPServer's comment in http_test.go for why.
	t.Chdir(t.TempDir())
	config.SetInstance(cfg)

	engine := game.NewEngine()
	wsHub := NewWebSocketHub()
	logsHub := NewLogsWebSocketHub(100)
	httpServer := NewHTTPServer(0, engine, wsHub, NewBuzzerWebSocketHub(), logsHub)

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

	// READY -> START (Start() launches a 3s countdown goroutine before entering STARTED)
	engine.Start(10)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !engine.IsGameStarted() {
		time.Sleep(50 * time.Millisecond)
	}
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

	// Verify state change sequence.
	// Start() emits COUNTDOWN immediately, then STARTED when the countdown goroutine finishes.
	expectedSequence := []game.GamePhase{
		game.PhasePrepare,
		game.PhaseReady,
		game.PhaseCountdown,
		game.PhaseStarted, // emitted by actualStart() after countdown
		game.PhasePaused,
		game.PhaseStarted, // emitted by Continue()
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
	// Defensive isolation from the tracked fixture (bugfix #143), even though
	// this test only calls config.SetInstance (no disk I/O today) — see
	// setupTestHTTPServer's comment in http_test.go for why.
	t.Chdir(t.TempDir())
	config.SetInstance(cfg)

	engine := game.NewEngine()
	wsHub := NewWebSocketHub()
	logsHub := NewLogsWebSocketHub(100)
	httpServer := NewHTTPServer(0, engine, wsHub, NewBuzzerWebSocketHub(), logsHub)

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
