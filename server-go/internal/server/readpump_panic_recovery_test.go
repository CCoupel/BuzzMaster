package server

// Regression test for #131 (plan _work/reports/plan-20260811-105859.md §7
// task 8), preparing Batch 2. Connection-level counterpart to
// cmd/server/dispatch_panic_recovery_test.go (task 9): that test covers the
// single shared dispatch goroutine, this one covers a PER-CONNECTION
// readPump goroutine (websocket.go:531, cleanup defer at :532-535). The
// blast radius is different by construction: readPump runs once per
// connected WebSocket client, so a panic here must take down only that one
// faulty connection — a second, unrelated client, and the process itself,
// must survive untouched.
//
// Injection point: hub.OnMessage (websocket.go:56), the one exported
// extension point readPump invokes directly inside its own per-connection,
// per-message loop (websocket.go:589), right after decoding and forwarding
// each message. A panic raised there is architecturally equivalent, for
// this test's purposes, to a panic raised inside whatever message-handling
// code readPump reaches — without requiring a change to production code
// just to add a test seam.
//
// ⚠️ EXPECTED TO CRASH THE TEST BINARY until #131 lands. readPump's existing
// defer (websocket.go:532-535: `c.Hub.unregister <- c; c.Conn.Close()`)
// unregisters the client and closes its socket, but deferred functions
// running during an unwinding panic do NOT stop the panic from propagating
// — without an explicit recover() added inside that defer (plan task 8),
// the panic continues past the cleanup and crashes the whole process. This
// is the correct RED state for a crash-class regression test: see the
// longer justification in cmd/server/dispatch_panic_recovery_test.go for
// why t.Skip() must not be added here either — it would silently defeat
// Batch 1's own goal of a clean, green `go test ./...`.
//
// This file is additive: it introduces its own helpers (all package-local
// to this test file) and reuses, without modifying, startTestWSServer /
// dialWSPath from websocket_test.go.

import (
	"buzzcontrol/internal/protocol"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const triggerPanicAction = "TRIGGER_PANIC_131_TEST"

func TestReadPump_PanicInOnMessageClosesOnlyFaultyConnection(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	var mu sync.Mutex
	triggered := false
	hub.OnMessage = func(clientID string, msg *protocol.Message) {
		if msg.Action != triggerPanicAction {
			return
		}
		mu.Lock()
		triggered = true
		mu.Unlock()
		panic("injected panic — #131 regression test (readPump/OnMessage)")
	}

	faulty := dialWSPath(t, srv, "/ws/admin")
	defer faulty.Close()
	survivor := dialWSPath(t, srv, "/ws/admin")
	defer survivor.Close()

	// Wait for both connections to complete registration before proceeding
	// (HandleConnectionWithType registers asynchronously via hub.register).
	deadline := time.Now().Add(2 * time.Second)
	for hub.ClientCount() < 2 {
		if time.Now().After(deadline) {
			t.Fatal("both test clients failed to register with the hub in time")
		}
		time.Sleep(10 * time.Millisecond)
	}

	panicMsg, err := protocol.NewMessage(triggerPanicAction, struct{}{})
	if err != nil {
		t.Fatalf("failed to build panic-trigger message: %v", err)
	}
	data, err := panicMsg.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("failed to serialize panic-trigger message: %v", err)
	}
	if err := faulty.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("failed to send panic-trigger message: %v", err)
	}

	// 1. The faulty connection's readPump must have panicked and been torn
	// down by the server: its side of the socket closes.
	faulty.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := faulty.ReadMessage(); err == nil {
		t.Error("expected the faulty connection to be closed by the server after the injected panic, it is still open")
	}

	mu.Lock()
	wasTriggered := triggered
	mu.Unlock()
	if !wasTriggered {
		t.Fatal("panic-trigger message was never received by hub.OnMessage — test setup is broken, " +
			"not exercising the recovery path at all")
	}

	// 2. The survivor must still be registered and reachable: a broadcast
	// sent through the hub after the panic must still reach it.
	broadcastMsg, err := protocol.NewMessage(protocol.ActionRemote, protocol.RemotePayload{Remote: "SURVIVOR_CHECK_131"})
	if err != nil {
		t.Fatalf("failed to build post-panic broadcast message: %v", err)
	}
	hub.Broadcast(broadcastMsg)

	survivor.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, recv, err := survivor.ReadMessage()
	if err != nil {
		t.Fatalf("survivor connection did not receive the post-panic broadcast (process/hub likely down): %v", err)
	}
	got, err := protocol.ParseSingle(recv)
	if err != nil {
		t.Fatalf("survivor received an unparseable message: %v", err)
	}
	if got.Action != protocol.ActionRemote {
		t.Errorf("survivor expected ACTION=%s, got %s", protocol.ActionRemote, got.Action)
	}

	// 3. Exactly one client — the survivor — remains registered.
	if n := hub.ClientCount(); n != 1 {
		t.Errorf("expected exactly 1 client left registered (the survivor) after the faulty one was torn down, got %d", n)
	}
}
