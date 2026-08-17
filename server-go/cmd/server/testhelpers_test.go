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
	// logger — was never initialized here, only patched in by
	// newBroadcast127TestApp (app.logger = server.NewBroadcastLogger(100),
	// main_broadcast_127_test.go) for its own needs. handleWebMessage calls
	// a.logger.Info(...) unconditionally for any action from ClientTypeAnim
	// (#155/#156, main.go:1028) — a nil logger panics there, recovered by
	// the test server's handler goroutine, but BEFORE the real dispatch runs.
	// Diagnosed by QA (_work/reports/qa-20260816-224638.md): this silently
	// broke TestHandleWebMessage_AllowList_AnimCanFlipMemoryCard (real
	// failure) and made TestHandleWebMessage_AllowList_AdminCannotFlipMemoryCard
	// "pass" for the wrong reason (an absence assertion, trivially true when
	// everything panicked before dispatch too). server.NewBroadcastLogger
	// (plain constructor), NOT server.InitLogger (which also mutates a
	// package-level global logger via SetGlobalLogger — every parallel test
	// app sharing that single global would be its own source of flakiness).
	app.logger = server.NewBroadcastLogger(1000)
	// #129: ardoiseCoalescer is nil-unsafe (Trigger/Flush/Stop all deref it) —
	// every test app needs one, even tests that never touch ARDOISE_INPUT,
	// since OnStateChange wiring (when a test wires it) calls Flush()
	// unconditionally. Real window here (not injected/fake): tests exercising
	// coalescing behavior specifically use BroadcastCoalescer's own test file
	// with an injected timer factory instead of this app-level one.
	//
	// #158/B1 — ClientTypeAnim added: this closure had drifted from main.go's
	// real init() wiring (server.ClientTypeAdmin, server.ClientTypeAnim),
	// left over from before #158. Spotted by dev-backend
	// (TestArdoiseInput_Anim_ReceivesCoalescedUpdate, ardoise_input_anim_test.go,
	// #158/T1) failing against the real production wiring — this test-only
	// helper was the one out of sync, not the production code.
	app.ardoiseCoalescer = NewBroadcastCoalescer(ardoiseCoalesceWindow, func() {
		app.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeAnim)
	})

	return app
}
