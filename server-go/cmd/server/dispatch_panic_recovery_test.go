package main

// Regression test for #131 (plan _work/reports/plan-20260811-105859.md §7
// task 9, risk R3), preparing Batch 2.
//
// Scope: the SINGLE shared web-message dispatch goroutine wired in
// setupCallbacks (main.go:312-316: `for msg := range a.wsHub.Incoming {
// a.handleWebMessage(msg) }`). This is a distinct blast radius from the
// per-connection readPump/writePump goroutines covered by
// internal/server/readpump_panic_recovery_test.go (#131 task 8): there is
// exactly one of these dispatch goroutines for the whole server, so it is
// the sole consumer of every web-client message. A panic that kills it
// silently deafens the entire admin/TV/VPlayer surface, not just one
// connection.
//
// R3's trap (planner's own words, task 9): "le recover doit être PAR
// MESSAGE (dans la boucle), pas autour de la boucle — sinon la goroutine
// meurt et le serveur devient sourd en silence, ce qui est pire que le
// crash actuel." A recover() wrapped around the `for` loop instead of
// inside it would stop this test from crashing the process, but the
// goroutine would still die on the very first panic — every message sent
// after that would sit unconsumed in the channel forever. That failure mode
// produces no crash and no error, so a bare "the process is still running"
// assertion cannot catch it. This test therefore does not stop at "no
// crash": it sends a second, well-formed message through the SAME channel
// after the injected panic and asserts it was actually processed by the
// engine — the only way to prove the loop itself (not just the process) is
// still alive.
//
// ⚠️ EXPECTED TO CRASH THE TEST BINARY until #131 lands (Batch 2). This is
// intentional, not a bug in this test: in Go, an unrecovered panic in ANY
// goroutine terminates the whole process, including goroutines the test
// itself spawned to faithfully reproduce main.go's dispatch pattern. There
// is no way to assert "the process survives an injected panic" other than
// the survival mechanism actually being in place — that is the entire
// point of a crash-class regression test (RED before the fix, GREEN after).
// Do NOT add t.Skip() to "fix" the crash: that would hide the regression
// this test exists to catch, defeating Batch 1's own goal (a clean, green
// `go test ./...`). If the rest of the suite needs to run in isolation
// before Batch 2 lands, exclude this test explicitly, e.g.:
//
//	go test ./cmd/server/... -run '^(?!TestWebMessageDispatch_PanicInHandlerDoesNotKillDispatch$)'
//
// Injection point: an *protocol.IncomingMessage with a nil Data field.
// handleWebMessage's very first statements are `msg := incoming.Data`
// followed by `switch msg.Action` (main.go:889-902) — dereferencing a nil
// *protocol.Message panics immediately with "invalid memory address or nil
// pointer dereference". This is not a contrived edge case: it is exactly
// the shape of input a future upstream parsing bug, or a client sending a
// truncated/malformed frame that survives protocol.ParseSingle with a nil
// payload, would produce — the same general class of "malformed message
// reaches the dispatch loop" that #131 exists to make non-fatal.

import (
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"testing"
	"time"
)

func TestWebMessageDispatch_PanicInHandlerDoesNotKillDispatch(t *testing.T) {
	app := newTestApp(t)
	app.wsHub = server.NewWebSocketHub()

	// Faithful reproduction of main.go:312-316's dispatch goroutine — not a
	// direct synchronous call to handleWebMessage (contrast with
	// quiz_meta_test.go's sendQuizMeta helper), because the fix under test
	// belongs specifically to this loop, and a synchronous call would never
	// exercise it.
	go func() {
		for msg := range app.wsHub.Incoming {
			app.handleWebMessage(msg)
		}
	}()

	// 1. Inject the panic.
	app.wsHub.Incoming <- &protocol.IncomingMessage{ClientID: "faulty-client", Data: nil}

	// 2. Send a second, well-formed message through the SAME channel/loop
	// right after, and confirm its effect actually landed in the engine —
	// the R3 assertion. UPDATE_QUIZ_META is a convenient, already-tested
	// action (quiz_meta_test.go) with an observable, easily-asserted side
	// effect (engine.GetState().QuizName).
	updateMsg := &protocol.Message{
		Action: protocol.ActionUpdateQuizMeta,
		Msg:    json.RawMessage(`{"NAME":"After injected panic","THEME":"","NOTES":""}`),
	}
	app.wsHub.Incoming <- &protocol.IncomingMessage{ClientID: "test-admin-after-panic", ClientType: "admin", Data: updateMsg}

	deadline := time.Now().Add(2 * time.Second)
	for {
		if app.engine.GetState().QuizName == "After injected panic" {
			return // dispatch survived the panic and kept consuming messages
		}
		if time.Now().After(deadline) {
			t.Fatal("message sent after the injected panic was never processed — the dispatch goroutine " +
				"likely died (recover() wrapped AROUND the loop instead of INSIDE it, see plan risk R3) " +
				"or the process would have already crashed before reaching this point")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
