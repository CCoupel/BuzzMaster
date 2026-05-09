package server

// Non-regression tests for bugfix #81 — HTTPServer.Stop() must use Shutdown(ctx)
// instead of Close().
//
// Bug: Stop() called h.server.Close(), which caused two problems:
//   1. In-flight requests were abruptly terminated (connection reset by peer).
//   2. TCP connections could be left in TIME_WAIT, delaying port rebinding.
//
// Fix: Stop() now calls h.server.Shutdown(ctx) with a 3-second timeout, which:
//   - Waits for active handlers to return before closing the socket.
//   - Releases the listening socket cleanly (no TIME_WAIT on the listener).

import (
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// newStopTestServer creates and starts a real HTTPServer on an ephemeral port.
// It blocks until the server is ready to accept connections (up to 3 s).
// The caller is responsible for calling Stop() when done (or the test server
// will be stopped via t.Cleanup if not stopped explicitly).
func newStopTestServer(t *testing.T) (*HTTPServer, int) {
	t.Helper()

	// Grab a free port then release it so HTTPServer can bind to it.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newStopTestServer: could not find a free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	// Initialise a minimal config. config.Get() must be called first so that
	// the internal once.Do has already fired before SetInstance is used.
	_ = config.Get()
	dataDir := t.TempDir()
	cfg := &config.Config{
		Server: config.ServerConfig{HTTPPort: port},
		Storage: config.StorageConfig{
			DataDir:      dataDir,
			QuestionsDir: filepath.Join(dataDir, "files", "questions"),
		},
		Version: "test-stop",
	}
	config.SetInstance(cfg)

	engine := game.NewEngine()
	wsHub := NewWebSocketHub()
	go wsHub.Run()
	logsHub := NewLogsWebSocketHub(100)
	go logsHub.Run()

	srv := NewHTTPServer(port, engine, wsHub, NewBuzzerWebSocketHub(), logsHub)
	srv.SetWebDir(dataDir)

	if err := srv.Start(); err != nil {
		t.Fatalf("newStopTestServer: Start() returned error: %v", err)
	}

	// Poll until the server is ready (GET /version returns 200).
	client := &http.Client{Timeout: 300 * time.Millisecond}
	versionURL := fmt.Sprintf("http://127.0.0.1:%d/version", port)
	deadline := time.Now().Add(3 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		resp, err := client.Get(versionURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	if !ready {
		t.Fatal("newStopTestServer: server did not become ready within 3 s")
	}

	return srv, port
}

// TestHTTPServerStop_UsesShutdown verifies that Stop() terminates the server cleanly
// and that the port can be immediately rebound — no TIME_WAIT residue.
//
// Regression scenario: with h.server.Close() (old bug), the server socket could be
// left with outstanding TCP connections in TIME_WAIT, which on some systems
// (notably Linux without SO_REUSEADDR applied to the new listener) prevents a new
// server process from binding the same port right away.
//
// With h.server.Shutdown(ctx) (fix) the listening socket is closed gracefully:
// rebind is immediate regardless of the platform.
func TestHTTPServerStop_UsesShutdown(t *testing.T) {
	srv, port := newStopTestServer(t)

	// Confirm the server is reachable before we stop it.
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/version", port))
	if err != nil {
		t.Fatalf("server not reachable before Stop(): %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status before Stop(): %d", resp.StatusCode)
	}

	// Stop the server.
	srv.Stop()

	// Brief OS scheduling pause to ensure the socket close is processed.
	time.Sleep(50 * time.Millisecond)

	// Rebind the same port immediately — must succeed.
	// With Shutdown this is guaranteed; with Close (bug) TIME_WAIT may block it.
	l, bindErr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if bindErr != nil {
		t.Fatalf("port %d could not be rebound after Stop() — possible TIME_WAIT residue: %v", port, bindErr)
	}
	l.Close()

	// Additionally verify the server is truly stopped: new HTTP requests must fail.
	_, connErr := client.Get(fmt.Sprintf("http://127.0.0.1:%d/version", port))
	if connErr == nil {
		t.Errorf("server still reachable after Stop() — not properly shut down")
	}
}

// TestHTTPServerStop_GracefulDrain verifies that Stop() waits for in-flight
// request handlers to complete before returning.
//
// Key behavioural difference between Close() and Shutdown():
//
//   - h.server.Close() (old bug): returns immediately — active handler goroutines
//     lose their connection before they can write the response. The client receives
//     a "connection reset by peer" or EOF instead of the full response.
//
//   - h.server.Shutdown(ctx) (fix): blocks until every active handler has returned
//     (or the 3 s context deadline is reached). The client receives a complete,
//     successful response.
//
// The discriminating assertion is: after Stop() returns, the slow handler MUST have
// already finished. With Shutdown this is a hard guarantee; with Close the handler
// is still sleeping when Stop() returns.
func TestHTTPServerStop_GracefulDrain(t *testing.T) {
	// Grab a free port.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not find a free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	// Minimal config.
	_ = config.Get()
	dataDir := t.TempDir()
	cfg := &config.Config{
		Server: config.ServerConfig{HTTPPort: port},
		Storage: config.StorageConfig{
			DataDir:      dataDir,
			QuestionsDir: filepath.Join(dataDir, "files", "questions"),
		},
		Version: "test-drain",
	}
	config.SetInstance(cfg)

	engine := game.NewEngine()
	wsHub := NewWebSocketHub()
	go wsHub.Run()
	logsHub := NewLogsWebSocketHub(100)
	go logsHub.Run()

	srv := NewHTTPServer(port, engine, wsHub, NewBuzzerWebSocketHub(), logsHub)
	srv.SetWebDir(dataDir)

	// Register a slow handler on the mux BEFORE Start() (which calls setupRoutes).
	// /test-slow is not registered by setupRoutes, so there is no duplicate panic.
	// ServeMux routes by longest prefix, so /test-slow beats the catch-all "/".
	handlerStarted := make(chan struct{})
	handlerDone := make(chan struct{})

	srv.mux.HandleFunc("/test-slow", func(w http.ResponseWriter, r *http.Request) {
		close(handlerStarted)              // signal: handler is executing
		time.Sleep(100 * time.Millisecond) // simulate work in progress
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))   //nolint:errcheck
		close(handlerDone)      // signal: handler completed and response was written
	})

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	// Wait for server to be ready.
	readyClient := &http.Client{Timeout: 300 * time.Millisecond}
	versionURL := fmt.Sprintf("http://127.0.0.1:%d/version", port)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := readyClient.Get(versionURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
	}

	// Launch the slow request in a background goroutine.
	var (
		reqErr    error
		reqStatus int
		wg        sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		slowClient := &http.Client{Timeout: 10 * time.Second}
		resp, err := slowClient.Get(fmt.Sprintf("http://127.0.0.1:%d/test-slow", port))
		if err != nil {
			reqErr = err
			return
		}
		defer resp.Body.Close()
		io.ReadAll(resp.Body) //nolint:errcheck
		reqStatus = resp.StatusCode
	}()

	// Wait until the handler is executing before we call Stop().
	select {
	case <-handlerStarted:
		// good — request is in flight
	case <-time.After(2 * time.Second):
		t.Fatal("slow handler did not start within 2 s")
	}

	// Call Stop() while the 100 ms handler is still sleeping.
	//
	// With Shutdown(ctx, 3 s) [fix]:  Stop() blocks here for ~100 ms until the
	//   handler returns, then returns itself.  handlerDone is guaranteed to be
	//   closed before Stop() returns.
	//
	// With Close() [bug]:  Stop() returns immediately (~0 ms).  The handler is
	//   still sleeping; handlerDone is NOT yet closed.
	srv.Stop()

	// Discriminating assertion: after Stop() returns, handlerDone must be closed.
	// A 20 ms window is generous — if Shutdown was used, the channel is already
	// closed; if Close was used, the handler is still sleeping for ~90 ms more.
	select {
	case <-handlerDone:
		// good — Shutdown waited for the handler
	case <-time.After(20 * time.Millisecond):
		t.Error("Stop() returned before in-flight handler completed — " +
			"Stop() likely called Close() instead of Shutdown(ctx)")
	}

	// Wait for the client goroutine and verify the response was received intact.
	wg.Wait()
	if reqErr != nil {
		t.Errorf("in-flight request received an error (connection likely cut by Close): %v", reqErr)
	}
	if reqStatus != http.StatusOK {
		t.Errorf("in-flight request got status %d, want %d", reqStatus, http.StatusOK)
	}
}
