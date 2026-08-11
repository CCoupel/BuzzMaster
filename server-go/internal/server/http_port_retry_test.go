package server

// Non-regression tests for commit f625bbe — HTTPServer.Start() port-busy retry loop.
//
// Bug context: during a self-update the old server process may still hold port 80
// while the new process starts. Without the retry loop the new server would fail
// immediately with "address already in use". The fix adds a 500 ms retry inside
// the Start() goroutine.

import (
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

// TestIsPortInUse verifies that the helper correctly classifies port-busy errors.
func TestIsPortInUse(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    bool
	}{
		{
			name: "Linux address already in use",
			err:  fmt.Errorf("listen tcp :80: bind: address already in use"),
			want: true,
		},
		{
			name: "Windows Only one usage",
			err:  fmt.Errorf("listen tcp 0.0.0.0:80: bind: Only one usage of each socket address (protocol/network address/port) is normally permitted."),
			want: true,
		},
		{
			name: "Unrelated error — connection refused",
			err:  fmt.Errorf("dial tcp 127.0.0.1:80: connect: connection refused"),
			want: false,
		},
		{
			name: "Unrelated error — server closed",
			err:  http.ErrServerClosed,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isPortInUse(tc.err)
			if got != tc.want {
				t.Errorf("isPortInUse(%q) = %v, want %v", tc.err.Error(), got, tc.want)
			}
		})
	}
}

// TestHTTPServer_Start_RetriesOnPortBusy is the non-regression test for commit f625bbe.
//
// Scenario:
//  1. A temporary listener occupies a free TCP port (simulates the old server still running).
//  2. HTTPServer.Start() is called on that port — it must NOT return an error even though
//     the port is busy; it should enter its retry loop silently.
//  3. After 300 ms the blocker is closed (simulates the old process finally releasing the port).
//  4. The test polls GET /version until the server becomes reachable (up to 3 s).
//
// Before the fix this scenario would have required external intervention because
// ListenAndServe exited immediately on the first EADDRINUSE without retrying.
func TestHTTPServer_Start_RetriesOnPortBusy(t *testing.T) {
	// Step 1 — grab a free port and keep it occupied.
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not grab a free port: %v", err)
	}
	port := blocker.Addr().(*net.TCPAddr).Port

	// Step 2 — configure the server for this test.
	// Isolate config from the tracked fixture (bugfix #143) — see
	// setupTestHTTPServer's comment in http_test.go for why.
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

	// Step 3 — Start() must return nil immediately (async retry loop).
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() returned unexpected error: %v", err)
	}
	defer srv.Stop()

	// Step 4 — release the port after 300 ms (shorter than the 500 ms retry interval,
	// so the server is guaranteed to encounter the busy port at least once before binding).
	time.AfterFunc(300*time.Millisecond, func() {
		blocker.Close()
	})

	// Step 5 — poll until the server responds on /version (timeout 3 s).
	url := fmt.Sprintf("http://127.0.0.1:%d/version", port)
	deadline := time.Now().Add(3 * time.Second)
	client := &http.Client{Timeout: 500 * time.Millisecond}

	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return // server is up — test passes
			}
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}

	t.Errorf("HTTPServer did not start after port was freed within 3s (last error: %v)", lastErr)
}
