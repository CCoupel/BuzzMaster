package main

import (
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"testing"
)

// newTestApp creates a minimal App instance for unit testing LED_SET logic.
// It initializes the game engine, buzzerHub, and bumperLEDState.
// No TCP/HTTP/WebSocket servers are started.
func newTestApp(t *testing.T) *App {
	t.Helper()

	cfg := config.Get()

	engine := game.NewEngine()

	// Add default teams so teamColorToRGB has something to look up
	teams := map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}},
		"TeamB": {Name: "TeamB", Color: []int{0, 0, 255}},
		"TeamC": {Name: "TeamC", Color: []int{0, 255, 0}},
	}
	engine.SetTeams(teams)

	buzzerHub := server.NewBuzzerWebSocketHub()

	app := &App{
		config:          cfg,
		engine:          engine,
		buzzerHub:       buzzerHub,
		bumperLEDState:  make(map[string]protocol.LEDSetPayload),
		bumperBuzzState: make(map[string]game.BuzzState),
	}
	// #129: ardoiseCoalescer is nil-unsafe (Trigger/Flush/Stop all deref it) —
	// every test app needs one, even tests that never touch ARDOISE_INPUT,
	// since OnStateChange wiring (when a test wires it) calls Flush()
	// unconditionally. Real window here (not injected/fake): tests exercising
	// coalescing behavior specifically use BroadcastCoalescer's own test file
	// with an injected timer factory instead of this app-level one.
	app.ardoiseCoalescer = NewBroadcastCoalescer(ardoiseCoalesceWindow, func() {
		app.broadcastUpdateTo(server.ClientTypeAdmin)
	})

	return app
}
