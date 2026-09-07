package server

// Tests for HTTPServer.Start()'s port-busy retry loop.
//
// History: the loop was introduced by commit f625bbe for the auto-update
// scenario — updater.go relaunches the new binary while the old process may
// still hold the port for a moment, so the new process must retry the bind
// instead of dying immediately.
//
// #220 rewrite (macevache retour terrain) — this is an intentional contract
// change, flagged to code-reviewer as such, not a regression:
//
//	BEFORE: Start() always returned nil immediately, even on a busy port. The
//	  retry happened silently in a detached goroutine — non-observable, and
//	  the caller (main.go) logged "Server started successfully" and opened
//	  the browser on a URL that was not actually listening yet.
//	AFTER:  Start(ctx) performs a SYNCHRONOUS, ctx-interruptible pre-bind: it
//	  blocks the calling goroutine until the port is actually bound (or ctx
//	  is cancelled), and only then returns. The Serve() loop itself still
//	  runs in a background goroutine once bind succeeds.
//	isPortInUse: string comparison ("address already in use" / "Only one
//	  usage") replaced by errors.Is(err, syscall.EADDRINUSE) (+ Windows
//	  equivalent). EACCES (permission denied, e.g. port <1024 without
//	  privileges) is handled as a SEPARATE branch: never classified as
//	  "port in use", never fatal, never os.Exit — it loops too, just at a
//	  slower, actionable cadence (see dev-backend handoff: 30s).
//
// TestHTTPServer_Start_RetriesOnPortBusy (pre-#220) asserted the OPPOSITE of
// the new contract — that Start() returns nil despite a busy port. It is
// replaced below by TestHTTPServer_Start_BlocksUntilPortFree, which keeps
// covering the exact same auto-update non-regression scenario (busy → freed
// → server eventually serves) but with the corrected polarity.

import (
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// newUnstartedPortRetryServer builds an HTTPServer configured for the given
// port, without calling Start(). Isolates config from the tracked fixture
// (bugfix #143) — see setupTestHTTPServer's comment in http_test.go for why.
func newUnstartedPortRetryServer(t *testing.T, port int) *HTTPServer {
	t.Helper()

	t.Chdir(t.TempDir())
	_ = config.Get() // ensure once.Do has fired
	dataDir := t.TempDir()
	cfg := &config.Config{
		Server: config.ServerConfig{
			HTTPPort: port,
		},
		Storage: config.StorageConfig{
			DataDir:      dataDir,
			QuestionsDir: filepath.Join(dataDir, "files", "questions"),
		},
		Version: "test-retry",
	}
	config.SetInstance(cfg)

	engine := game.NewEngine()
	wsHub := NewWebSocketHub()
	go wsHub.Run()
	logsHub := NewLogsWebSocketHub(100)
	go logsHub.Run()

	srv := NewHTTPServer(port, engine, wsHub, NewBuzzerWebSocketHub(), logsHub)
	srv.SetWebDir(dataDir)
	return srv
}

// TestIsPortInUse verifies that the helper correctly classifies bind errors
// into exactly the three buckets #220 requires distinct handling for:
//   - EADDRINUSE (real bind conflict)              → true  (fast backoff loop)
//   - EACCES (permission denied)                    → false (handled separately,
//     slow-cadence loop — see TestHTTPServer_Start_EACCES_DoesNotExitFatally)
//   - anything else (connection refused, ErrServerClosed, ...) → false
//
// Uses REAL OS errors (obtained by actually provoking a bind conflict / a
// permission failure) rather than crafted string literals — the old
// string-comparison implementation this replaces was broken on non-English
// Windows locales precisely because it matched on error text.
func TestIsPortInUse(t *testing.T) {
	// Real EADDRINUSE: bind the exact same address twice.
	first, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not grab a free port: %v", err)
	}
	defer first.Close()

	_, busyErr := net.Listen("tcp", first.Addr().String())
	if busyErr == nil {
		t.Fatal("expected a bind conflict when listening twice on the same address, got nil error")
	}
	if !isPortInUse(busyErr) {
		t.Errorf("isPortInUse(%v) = false, want true for a real bind conflict", busyErr)
	}

	// Unrelated errors must not be classified as "port busy".
	unrelated := []struct {
		name string
		err  error
	}{
		{"connection refused", fmt.Errorf("dial tcp 127.0.0.1:1: connect: connection refused")},
		{"server already closed", http.ErrServerClosed},
	}
	for _, tc := range unrelated {
		t.Run(tc.name, func(t *testing.T) {
			if isPortInUse(tc.err) {
				t.Errorf("isPortInUse(%v) = true, want false", tc.err)
			}
		})
	}

	// EACCES must NOT be classified as "port in use" — #220 requires it to be
	// handled as its own branch (slow-cadence loop, actionable message),
	// never merged into the fast EADDRINUSE backoff.
	t.Run("permission denied (EACCES) is not port-in-use", func(t *testing.T) {
		probe, permErr := net.Listen("tcp", ":1") // port 1 — reserved, needs privileges
		if permErr == nil {
			probe.Close()
			t.Skip("this environment allows binding low ports without privilege — cannot reproduce EACCES")
		}
		if !errors.Is(permErr, os.ErrPermission) {
			t.Skipf("binding port 1 failed for a reason other than permission (%v) — skipping", permErr)
		}
		if isPortInUse(permErr) {
			t.Errorf("isPortInUse(%v) = true, want false — EACCES must be handled separately from EADDRINUSE", permErr)
		}
	})
}

// TestHTTPServer_Start_BlocksUntilPortFree replaces
// TestHTTPServer_Start_RetriesOnPortBusy (see file header for why the
// polarity flipped) and remains THE non-regression test for the historical
// auto-update scenario:
//
//  1. A temporary listener occupies a free TCP port (simulates the old
//     server process still holding it during a self-update relaunch).
//  2. HTTPServer.Start(ctx) is called on that port — it must BLOCK (not
//     return) and the server must NOT be reachable yet.
//  3. The blocker is closed (simulates the old process finally exiting).
//  4. Start(ctx) must then return nil, and /version must become reachable.
func TestHTTPServer_Start_BlocksUntilPortFree(t *testing.T) {
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not grab a free port: %v", err)
	}
	port := blocker.Addr().(*net.TCPAddr).Port

	srv := newUnstartedPortRetryServer(t, port)
	defer srv.Stop()

	ctx := context.Background()
	startErrCh := make(chan error, 1)
	go func() { startErrCh <- srv.Start(ctx) }()

	// While busy, Start(ctx) must still be blocked...
	select {
	case err := <-startErrCh:
		t.Fatalf("Start(ctx) returned (err=%v) while the port was still busy — it must block until bind succeeds (#220)", err)
	case <-time.After(200 * time.Millisecond):
	}

	// ...and the server must not be answering yet — no premature "started" state.
	url := fmt.Sprintf("http://127.0.0.1:%d/version", port)
	probeClient := &http.Client{Timeout: 200 * time.Millisecond}
	if resp, probeErr := probeClient.Get(url); probeErr == nil {
		resp.Body.Close()
		t.Fatalf("server answered on /version (status %d) while the port was supposedly still busy", resp.StatusCode)
	}

	// Release the port — Start(ctx) must now complete.
	blocker.Close()

	select {
	case err := <-startErrCh:
		if err != nil {
			t.Fatalf("Start(ctx) returned an error after the port was freed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start(ctx) did not return within 3s of the port being freed")
	}

	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, getErr := client.Get(url)
		if getErr == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return // server is up — test passes
			}
		}
		lastErr = getErr
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("server did not become reachable on /version after Start(ctx) returned (last error: %v)", lastErr)
}

// TestHTTPServer_Start_ContextCancelDuringPortWait verifies the #220
// interruption requirement (Ctrl+C during the wait): cancelling ctx while
// Start(ctx) is blocked on a busy port must return promptly — not wait out
// the current backoff sleep — and must not leave the retry goroutine
// running afterwards (residual-goroutine check).
func TestHTTPServer_Start_ContextCancelDuringPortWait(t *testing.T) {
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not grab a free port: %v", err)
	}
	defer blocker.Close() // keep the port busy for the whole test
	port := blocker.Addr().(*net.TCPAddr).Port

	srv := newUnstartedPortRetryServer(t, port)
	defer srv.Stop()
	baseline := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	startErrCh := make(chan error, 1)
	go func() { startErrCh <- srv.Start(ctx) }()

	select {
	case err := <-startErrCh:
		t.Fatalf("Start(ctx) returned (err=%v) while the port was still busy — expected it to block", err)
	case <-time.After(200 * time.Millisecond):
	}

	cancel()

	select {
	case err := <-startErrCh:
		if err == nil {
			t.Fatal("Start(ctx) returned nil after context cancellation — expected a cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Start(ctx) error = %v, want context.Canceled (or wrapping it)", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Start(ctx) did not return within 1s of context cancellation — retry loop not interruptible (Ctrl+C would hang)")
	}

	// No residual goroutine: allow a brief settle window, then compare.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && runtime.NumGoroutine() > baseline+1 {
		time.Sleep(20 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > baseline+1 {
		t.Errorf("goroutine count did not settle after Start(ctx) cancellation: got %d, baseline %d — possible leaked retry goroutine", got, baseline)
	}
}

// TestHTTPServer_Start_EACCES_DoesNotExitFatally verifies the #220
// requirement that a permission-denied bind (EACCES, e.g. binding <1024
// without privileges) NEVER calls os.Exit and NEVER fails fast — it must
// loop (at a slow, actionable cadence) exactly like a busy port. Explicit
// decision (dev-backend handoff): a double-click launch on Windows would
// close the console before an immediate-exit message could ever be read, so
// silent death is worse than a slow retry.
//
// If the implementation under test called os.Exit on EACCES, this entire
// test binary would abort right here — that abrupt death is itself part of
// the regression signal, on top of the explicit assertions below.
//
// Skips cleanly if this environment does not restrict low ports (e.g.
// running as root, or a platform/sandbox without that restriction) — per
// the test-writer handoff, EACCES coverage falls back to the classification
// unit test alone (TestIsPortInUse) when it cannot be reproduced live.
func TestHTTPServer_Start_EACCES_DoesNotExitFatally(t *testing.T) {
	const privilegedPort = 1 // reserved, requires elevated privileges on every platform we support

	probe, probeErr := net.Listen("tcp", fmt.Sprintf(":%d", privilegedPort))
	if probeErr == nil {
		probe.Close()
		t.Skip("this environment allows binding low ports without privilege — cannot reproduce EACCES")
	}
	if !errors.Is(probeErr, os.ErrPermission) {
		t.Skipf("binding port %d failed for a reason other than permission (%v) — skipping", privilegedPort, probeErr)
	}

	srv := newUnstartedPortRetryServer(t, privilegedPort)
	defer srv.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	startErrCh := make(chan error, 1)
	go func() { startErrCh <- srv.Start(ctx) }()

	// Must NOT return immediately with a fatal permission error — it loops.
	select {
	case err := <-startErrCh:
		t.Fatalf("Start(ctx) returned immediately on EACCES (err=%v) — #220 requires it to loop with an actionable message, never fail fast", err)
	case <-time.After(300 * time.Millisecond):
	}

	// Prove it is interruptible even at the slow EACCES cadence (30s) — the
	// wait must be a select on ctx.Done(), never a blind time.Sleep(30s).
	cancel()
	select {
	case err := <-startErrCh:
		if err == nil {
			t.Fatal("Start(ctx) returned nil after cancellation during an EACCES loop — expected a cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Start(ctx) error = %v, want context.Canceled (or wrapping it)", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Start(ctx) did not return within 1s of cancellation during an EACCES loop — not interruptible (would survive Ctrl+C for up to 30s)")
	}
}

// TestHTTPServer_Start_LogsPortNumberWhileBusy verifies the #220
// observability requirement: while waiting for a busy port, the wait must be
// logged at WARN (visible in /ws/logs) and the message must name the port
// being waited on.
func TestHTTPServer_Start_LogsPortNumberWhileBusy(t *testing.T) {
	previous := GetGlobalLogger()
	bl := NewBroadcastLogger(100)
	SetGlobalLogger(bl)
	t.Cleanup(func() { SetGlobalLogger(previous) })

	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not grab a free port: %v", err)
	}
	defer blocker.Close()
	port := blocker.Addr().(*net.TCPAddr).Port

	srv := newUnstartedPortRetryServer(t, port)
	defer srv.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Start(ctx) }()

	portStr := fmt.Sprintf("%d", port)
	deadline := time.Now().Add(2 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		for _, entry := range bl.GetHistory() {
			if entry.Level == game.LogLevelWarn && strings.Contains(entry.Message, portStr) {
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !found {
		t.Errorf("no WARN log entry mentioning port %d was emitted while the port was busy — the wait must be observable and name the port (#220)", port)
	}
}

// TestHTTPServer_Start_BackoffGrowsOnSustainedPortBusy verifies the #220
// non-saturation requirement: on a sustained busy port, the retry interval
// must grow (500ms → 1s → 5s, capped) instead of hammering the log buffer at
// a flat rate. The historical bug logged at a flat ~2 lines/second and
// saturated the 1000-entry buffer in ~8 minutes, evicting the startup
// messages before any /ws/logs client could read them.
//
// Skipped in -short mode: needs several real seconds of sustained backoff to
// tell "flat 500ms forever" apart from "500ms → 1s → 5s".
func TestHTTPServer_Start_BackoffGrowsOnSustainedPortBusy(t *testing.T) {
	if testing.Short() {
		t.Skip("needs several seconds of sustained backoff — skipped in -short")
	}

	previous := GetGlobalLogger()
	bl := NewBroadcastLogger(1000)
	SetGlobalLogger(bl)
	t.Cleanup(func() { SetGlobalLogger(previous) })

	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not grab a free port: %v", err)
	}
	defer blocker.Close()
	port := blocker.Addr().(*net.TCPAddr).Port

	srv := newUnstartedPortRetryServer(t, port)
	defer srv.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Start(ctx) }()

	// Observe for ~6.5s — long enough to cross into the 5s-capped tier
	// (500ms + 1s + 5s ≈ 6.5s cumulative) if the backoff truly grows. A flat
	// 500ms retry (pre-#220 behaviour) would produce ~13 log lines in this
	// window; a capped progressive backoff produces at most a handful.
	time.Sleep(6500 * time.Millisecond)
	cancel()

	portStr := fmt.Sprintf("%d", port)
	count := 0
	for _, entry := range bl.GetHistory() {
		if entry.Level == game.LogLevelWarn && strings.Contains(entry.Message, portStr) {
			count++
		}
	}

	if count == 0 {
		t.Fatal("no busy-port WARN log entries observed at all — cannot assess backoff behaviour (is the wait even logged?)")
	}
	if count > 6 {
		t.Errorf("got %d busy-port WARN log entries in ~6.5s, want a small number consistent with a growing 500ms→1s→5s backoff (a flat 500ms retry would produce ~13) — buffer-saturation regression", count)
	}
}
