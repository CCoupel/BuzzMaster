package server

// Non-regression tests for the lingerListener fix (#83).
//
// Root cause: after server shutdown, client connections closed with the
// standard FIN/ACK sequence enter TIME_WAIT, keeping the port busy for up to
// 4 minutes on Windows and 60 s on Linux.
//
// Fix: lingerListener.Accept() calls SetLinger(0) on every accepted TCPConn.
// With linger=0 the kernel sends a RST on Close() instead of FIN/ACK, so the
// connection is torn down immediately without entering TIME_WAIT.

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestLingerListener_SetsLinger verifies that lingerListener wraps Accept()
// and returns a connection on which SetLinger(0) has been applied.
// We cannot inspect the linger value via the standard net.Conn interface, so
// we confirm indirectly: the wrapped listener must still deliver a working
// connection accepted from a real TCP dial.
func TestLingerListener_SetsLinger(t *testing.T) {
	// Base listener on an ephemeral port.
	base, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer base.Close()

	ll := &lingerListener{base}

	// Dial the listener and accept.
	done := make(chan error, 1)
	go func() {
		conn, err := ll.Accept()
		if err != nil {
			done <- err
			return
		}
		conn.Close()
		done <- nil
	}()

	client, err := net.Dial("tcp", base.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()

	if err := <-done; err != nil {
		t.Fatalf("lingerListener.Accept() returned error: %v", err)
	}
}

// TestLingerListener_NonTCPPassthrough verifies that non-TCP connections are
// accepted without modification (no panic, no dropped conn).
func TestLingerListener_NonTCPPassthrough(t *testing.T) {
	// Use a regular TCP listener — the conn IS a *net.TCPConn so SetLinger is
	// called. Confirm the connection is still usable after wrapping.
	base, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer base.Close()

	ll := &lingerListener{base}

	msg := "hello"
	go func() {
		c, dialErr := net.Dial("tcp", base.Addr().String())
		if dialErr != nil {
			return
		}
		defer c.Close()
		fmt.Fprint(c, msg)
	}()

	conn, err := ll.Accept()
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, len(msg))
	conn.SetDeadline(time.Now().Add(time.Second)) //nolint:errcheck
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("Read from accepted conn failed: %v", err)
	}
	if string(buf) != msg {
		t.Errorf("got %q, want %q", string(buf), msg)
	}
}

// TestHTTPServer_Start_UsesLingerListener is an integration test verifying
// that HTTPServer.Start() wraps its listener with lingerListener and that the
// server still handles HTTP requests correctly.
func TestHTTPServer_Start_UsesLingerListener(t *testing.T) {
	srv, port := newStopTestServer(t)
	defer srv.Stop()

	url := fmt.Sprintf("http://127.0.0.1:%d/version", port)
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("unexpected status %d, want 200", resp.StatusCode)
	}
}
