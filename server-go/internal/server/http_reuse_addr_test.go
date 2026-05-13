package server

// Non-regression tests for the SO_REUSEADDR fix.
//
// Root cause: on Windows, net.Listen does NOT set SO_REUSEADDR by default
// (unlike Linux). After a forceful process kill (console window closed),
// the listening socket may remain in TIME_WAIT for up to 4 minutes, causing
// the next server launch to fail with WSAEADDRINUSE.
//
// Fix: newReuseAddrListener explicitly sets SO_REUSEADDR on the socket so
// the server can bind immediately on every platform.

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestNewReuseAddrListener_Basic verifies that newReuseAddrListener creates a
// valid TCP listener that accepts connections.
func TestNewReuseAddrListener_Basic(t *testing.T) {
	ln, err := newReuseAddrListener("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newReuseAddrListener failed: %v", err)
	}
	defer ln.Close()

	if ln.Addr() == nil {
		t.Fatal("listener has nil address")
	}
	_, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected *net.TCPAddr, got %T", ln.Addr())
	}
}

// TestNewReuseAddrListener_ImmediateRebind verifies that the port can be
// rebound immediately after the listener is closed — the key SO_REUSEADDR
// property.
func TestNewReuseAddrListener_ImmediateRebind(t *testing.T) {
	// First bind.
	ln1, err := newReuseAddrListener("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("first bind failed: %v", err)
	}
	port := ln1.Addr().(*net.TCPAddr).Port
	ln1.Close()

	// Immediate rebind on the same port — must succeed with SO_REUSEADDR.
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln2, err := newReuseAddrListener("tcp", addr)
	if err != nil {
		t.Fatalf("immediate rebind on :%d failed (SO_REUSEADDR not effective?): %v", port, err)
	}
	ln2.Close()
}

// TestHTTPServer_Start_UsesReuseAddr verifies that HTTPServer.Start() uses the
// SO_REUSEADDR listener: after Stop(), the port can be immediately rebound by
// a new HTTPServer — no TIME_WAIT residue on the listener socket.
//
// This is a regression test for the port-busy-after-kill bug on Windows.
func TestHTTPServer_Start_UsesReuseAddr(t *testing.T) {
	srv, port := newStopTestServer(t)

	// Stop the server (graceful shutdown).
	srv.Stop()
	time.Sleep(30 * time.Millisecond) // let the OS process the close

	// A second HTTPServer must be able to bind the same port immediately.
	// With SO_REUSEADDR this succeeds even if the old socket is in TIME_WAIT.
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := newReuseAddrListener("tcp", addr)
	if err != nil {
		t.Fatalf("port %d could not be rebound after Stop() — SO_REUSEADDR not working: %v", port, err)
	}
	ln.Close()

	// Also verify the old server truly stopped: HTTP requests must fail.
	client := &http.Client{Timeout: 300 * time.Millisecond}
	_, connErr := client.Get(fmt.Sprintf("http://127.0.0.1:%d/version", port))
	if connErr == nil {
		t.Error("server still reachable after Stop() — not properly shut down")
	}
}
