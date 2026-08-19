package main

// Regression test for #151 (plan _work/reports/plan-20260819-160602.md,
// Phase 1), site 5/5 — the MEMORY auto-flip-back goroutine spawned by
// handleFlipMemoryCard (main.go, FLIP_MEMORY_CARD handler) when two flipped
// cards don't match. This is the plan's "priorité 1" site: unlike the 4
// engine.go ticker sites (covered by
// internal/game/engine_panic_recovery_test.go), it is reachable directly by
// a single client WebSocket message (FLIP_MEMORY_CARD) — a malicious client
// or a corrupted engine state hitting a panic in
// ClearMemoryFlippedCards/broadcastUpdate/sendLEDSetAllBuzzers inside that
// goroutine would previously take down the entire process, silencing every
// connected client (admin/TV/VJoueurs/buzzers), not just the sender.
//
// Unlike the 4 engine.go sites, this goroutine never holds a manual
// e.mu.Lock()/Unlock() pair of its own — ClearMemoryFlippedCards already
// uses `e.mu.Lock(); defer e.mu.Unlock()` internally, so a panic there was
// already mutex-safe even before #151. The fix here is simpler: wrap the
// whole goroutine body in `defer recover()` (see the diff on main.go around
// handleFlipMemoryCard). This test still proves the same two things the
// team-lead's brief requires uniformly across all 5 sites: (a) the process
// survives the injected panic, and (b) the engine's mutex remains fully
// usable afterward — here demonstrated by the engine staying responsive to
// both a read (GetState) and a brand new client action dispatched right
// after, rather than by inspecting engine.mu directly (unexported in
// package game, not reachable from this package).
//
// Injection point: main.go's setTestInjectMemoryFlipBackPanic/
// clearTestInjectMemoryFlipBackPanic hook (added by dev-backend as part of
// the #151 fix — nil in production, called right after
// ClearMemoryFlippedCards, before broadcastUpdate/sendLEDSetAllBuzzers, so
// MemoryFlippedCards is left correctly cleared instead of permanently
// stranded at 2 flipped cards). Without it there is no naturally reachable
// panic in ClearMemoryFlippedCards/broadcastUpdate/sendLEDSetAllBuzzers
// with a valid-but-adversarial engine state.
//
// ⚠️ EXPECTED TO CRASH THE TEST BINARY if run against pre-#151 code (no
// recover() around this goroutine) — same intentional RED-before-fix /
// GREEN-after-fix shape as cmd/server/dispatch_panic_recovery_test.go and
// internal/server/readpump_panic_recovery_test.go (#131). Do not add
// t.Skip() to hide it.

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"testing"
	"time"
)

// flipMemoryCardDirect dispatches a FLIP_MEMORY_CARD message through the
// real handleWebMessage path (not a direct engine.FlipMemoryCard call) so
// the allow-list gate (ClientTypeTV is allowed, #159/B1) and the goroutine
// spawn inside handleFlipMemoryCard are actually exercised — mirrors
// quiz_meta_test.go's sendQuizMeta helper.
func flipMemoryCardDirect(t *testing.T, app *App, cardID string) {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{CardID: cardID})
	if err != nil {
		t.Fatalf("failed to build FLIP_MEMORY_CARD message: %v", err)
	}
	app.handleWebMessage(&protocol.IncomingMessage{ClientID: "test-tv", ClientType: string(server.ClientTypeTV), Data: msg})
}

func TestMemoryAutoFlipBack_PanicRecovery(t *testing.T) {
	app := newTestApp(t)
	// handleWebMessage dereferences a.wsHub (GetClientPlayerID, main.go:1038)
	// unconditionally — newTestApp leaves it nil by default (see
	// testhelpers_test.go); without this, the call panics on a nil pointer
	// before ever reaching handleFlipMemoryCard, and #131's own top-level
	// recover on handleWebMessage silently swallows it, making this test
	// report "injected panic was never triggered" for the wrong reason.
	// Same pattern as quiz_meta_test.go / player_connect_connstate_test.go.
	app.wsHub = server.NewWebSocketHub()
	app.engine.SetPhase(game.PhaseStarted)

	triggered := make(chan struct{})
	t.Cleanup(func() { clearTestInjectMemoryFlipBackPanic() })
	// setTestInjectMemoryFlipBackPanic (not a direct assignment) — the
	// goroutine under test reads this hook from its own goroutine, which a
	// bare `testInjectMemoryFlipBackPanicFn = ...` assignment from this test
	// goroutine wouldn't synchronize with; go test -race caught exactly this
	// before the accessor was introduced (dev-backend, #151 follow-up).
	setTestInjectMemoryFlipBackPanic(func() {
		close(triggered)
		panic("injected panic — #151 regression test (memory auto-flip-back)")
	})

	// Two cards from different pairs -> no match -> handleFlipMemoryCard
	// schedules the auto-flip-back goroutine under test.
	flipMemoryCardDirect(t, app, "1-1")
	flipMemoryCardDirect(t, app, "2-1")

	select {
	case <-triggered:
	// #151 follow-up: the hook now fires after time.Sleep(flipDelay) +
	// ClearMemoryFlippedCards (see callTestInjectMemoryFlipBackPanic's call
	// site in main.go) rather than before, so the wait must clear the
	// engine's default 3s flipDelay (FlipMemoryCard, no MemoryConfig set by
	// this test) with margin, not just network/scheduling jitter.
	case <-time.After(5 * time.Second):
		t.Fatal("injected panic was never triggered — the auto-flip-back goroutine was never scheduled, test setup is broken " +
			"(expected a mismatch on cards 1-1/2-1 to set shouldFlipBack=true)")
	}

	// #151 follow-up: disable the hook now that its one job (proving this
	// site recovers) is done, as defense in depth for whatever runs next.
	clearTestInjectMemoryFlipBackPanic()

	// (b) The engine's mutex must remain fully usable after the injected
	// panic — the actual non-deadlock proof for this site. GetState()
	// internally does e.mu.RLock(); wrapped with a timeout since a stuck
	// lock would otherwise hang the whole test run instead of failing it
	// cleanly.
	stateRead := make(chan struct{})
	go func() {
		app.engine.GetState()
		close(stateRead)
	}()
	select {
	case <-stateRead:
	case <-time.After(2 * time.Second):
		t.Fatal("engine.GetState() hung after the injected panic in the MEMORY auto-flip-back goroutine — " +
			"engine mutex likely left stuck")
	}

	// (a) The process (and the WebSocket dispatch path) survived: a brand
	// new client action right after must still be processed correctly —
	// mirrors dispatch_panic_recovery_test.go's "second message" proof.
	//
	// Deliberately a MATCHING pair (3-1/3-2, same pairID), not another
	// mismatch: a mismatch schedules its own auto-flip-back goroutine, and
	// nothing in this test waits for THAT one — it would sleep flipDelay
	// (3s) in the background and could still be pending when this test
	// function returns, becoming an orphan. Under `go test -count>1` (or
	// just a full package run) that orphan reads whatever hook a LATER
	// test has since installed and fires it prematurely — the actual cause
	// of an intermittent "before=1 after=1" failure observed while
	// developing this test (see git history). A match is fully synchronous
	// (FlipMemoryCard mutates MemoryMatchedPairs directly, no goroutine
	// scheduled), so this assertion has no timing dependency at all.
	before := len(app.engine.GetState().MemoryMatchedPairs)
	flipMemoryCardDirect(t, app, "3-1")
	flipMemoryCardDirect(t, app, "3-2")

	after := len(app.engine.GetState().MemoryMatchedPairs)
	if after != before+1 {
		t.Fatalf("a fresh FLIP_MEMORY_CARD match sent right after the injected panic was not processed "+
			"(MemoryMatchedPairs count before=%d after=%d, want after=before+1) — the WebSocket dispatch path or "+
			"the engine likely did not survive cleanly", before, after)
	}
}
