package server

// Regression tests for code-review finding M-1
// (_work/reports/code-review-20260806-101822.md): two real data races were
// found and fixed during #137 development (documented in
// _work/handoff/dev-backend-20260806-100800.md) —
//   1. aiJobRegistry.tryStart read r.job.State without the job's own mutex.
//   2. WebSocketHub.OnClientRegistered fired before Run() had actually
//      inserted the client into h.clients (an unbuffered channel send/
//      receive only synchronizes the rendezvous itself, not code after the
//      receive in the receiving goroutine).
//
// Neither fix had a test that reproduces REAL concurrent access (as opposed
// to sequential calls against a slow mock) — under `go test -race`, two
// accesses that never overlap in time can never be flagged as a race no
// matter whether the code is correct, so a future regression on either point
// would go undetected. These two tests use real goroutines + sync.WaitGroup
// specifically to close that gap.
//
// New file (not an edit to ai_job_test.go/ai_batching_test.go) to avoid any
// collision with test-writer's files, reusing their helpers (setupTestHTTPServer,
// setAIConfig, batchingAIConfig, mockAnthropicSSEServer, llmQuestion,
// baseGenerateRequest, postGenerateQuestions, withAnthropicBaseURL,
// dialAdminWS, expectAccepted, waitForJobTerminalState, sequencedAnthropicServer,
// twoUniqueSpeedyQuestions) — same package, same test binary.
//
// Run with `go test ./internal/server/... -run TestAIJob_Concurrent -race`.
//
// ⚠️ config.SetInstance mutates a global (via setAIConfig): no t.Parallel()
// in this file, consistent with every other AI test file in this package.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestAIJob_ConcurrentGenerateRequests_OnlyOneStarts_Race fires N real
// goroutines at POST /api/generate-questions simultaneously (not sequential
// calls against a slow mock) and asserts exactly one gets 202 — the rest
// must get 409 generation_in_progress. This is the scenario that exercises
// aiJobRegistry.tryStart's check-and-install-under-one-lock invariant
// (security finding M1) under real contention.
func TestAIJob_ConcurrentGenerateRequests_OnlyOneStarts_Race(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 1, 10, 2))

	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{
		llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "ANSWER": "ok"}),
	})
	withAnthropicBaseURL(t, upstream.URL)

	// Connected before the race so we can drain the eventual winner's job to
	// a terminal state afterward — otherwise a leftover RUNNING job in the
	// shared globalAIJobRegistry would block every subsequent test in this
	// binary with a spurious 409.
	conn := dialAdminWS(t, server)

	const n = 20
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := postGenerateQuestions(server, baseGenerateRequest())
			codes[i] = w.Code // disjoint index per goroutine — no race on the slice itself
		}(i)
	}
	wg.Wait()

	accepted := 0
	for i, code := range codes {
		switch code {
		case http.StatusAccepted:
			accepted++
		case http.StatusConflict:
			// expected for every loser of the race
		default:
			t.Errorf("goroutine %d: unexpected status %d (expected 202 or 409)", i, code)
		}
	}
	if accepted != 1 {
		t.Errorf("Expected exactly 1 of %d concurrent POSTs to start a job (202 Accepted), got %d — aiJobRegistry.tryStart is no longer atomic (security M1)", n, accepted)
	}

	waitForJobTerminalState(t, conn, 5*time.Second)
}

// TestAIJob_ConcurrentAdminConnectDuringProgress_Race dials N real,
// concurrent admin WebSocket connections WHILE a multi-batch job is actively
// running and broadcasting AI_GENERATION_PROGRESS (sequencedAnthropicServer
// + a real inter-batch delay give the job enough wall-clock span to overlap
// with the dials). This exercises the exact ordering
// WebSocketHub.OnClientRegistered depends on: a connecting client must be
// present in h.clients before the hook's SendToClient can find it, which is
// only guaranteed because the hook now fires from inside Run() itself
// (after h.clients[client]=true), not right after the unbuffered
// h.register<- send in HandleConnectionWithType.
func TestAIJob_ConcurrentAdminConnectDuringProgress_Race(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 30, 2)) // 5 batches from volume=10, spans real time via the inter-batch delay

	outcomes := []batchOutcome{
		{questions: twoUniqueSpeedyQuestions(0)},
		{questions: twoUniqueSpeedyQuestions(1)},
		{questions: twoUniqueSpeedyQuestions(2)},
		{questions: twoUniqueSpeedyQuestions(3)},
		{questions: twoUniqueSpeedyQuestions(4)},
	}
	upstream, _ := sequencedAnthropicServer(t, outcomes)
	withAnthropicBaseURL(t, upstream.URL)

	// Kept alive to drain the job to a terminal state at the end (avoids
	// leaking a RUNNING job into subsequent tests).
	mainConn := dialAdminWS(t, server)

	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 10}
	expectAccepted(t, postGenerateQuestions(server, req))

	wsSrv := httptest.NewServer(server.mux)
	defer wsSrv.Close()
	wsURL := "ws" + strings.TrimPrefix(wsSrv.URL, "http") + "/ws/admin"

	const n = 15
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				errCh <- fmt.Errorf("dial failed: %w", err)
				return
			}
			defer conn.Close()
			// Best-effort read: whether this connection happens to land while
			// the job is RUNNING (immediate progress push, contract §10) or
			// after it already finished (nothing to push) is not what this
			// test asserts — what matters is exercising OnClientRegistered
			// concurrently with the job's own broadcastAIJobProgress calls
			// under -race. A read timeout here is not a test failure.
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			conn.ReadMessage()
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}

	waitForJobTerminalState(t, mainConn, 5*time.Second)
}
