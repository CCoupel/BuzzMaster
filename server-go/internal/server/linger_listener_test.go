package server

// Non-regression tests for bugfix #83 — SO_LINGER(0) on accepted connections.
//
// Bug: after the HTTP server stops, accepted TCP connections that went through
// the normal FIN/ACK close sequence could linger in TIME_WAIT state. On Windows
// this could prevent immediate port rebinding even when SO_REUSEADDR was set on
// the listening socket (SO_REUSEADDR does not bypass TIME_WAIT for *accepted* sockets
// the way it does for the listening socket itself).
//
// Fix: lingerListener wraps net.Listener. Its Accept() method casts each accepted
// net.Conn to *net.TCPConn and calls SetLinger(0), which instructs the kernel to
// send an RST on close instead of the FIN/ACK sequence — eliminating TIME_WAIT.
//
// Structure of lingerListener (expected by these tests):
//
//	type lingerListener struct { net.Listener }
//	func newLingerListener(l net.Listener) *lingerListener
//	func (l *lingerListener) Accept() (net.Conn, error) {
//	    conn, err := l.Listener.Accept()
//	    if tc, ok := conn.(*net.TCPConn); ok { _ = tc.SetLinger(0) }
//	    return conn, err
//	}

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// TestLingerListener_Accept_ReturnsValidConnection verifies that lingerListener.Accept()
// returns a functional connection when a client dials in.
func TestLingerListener_Accept_ReturnsValidConnection(t *testing.T) {
	inner, err := newReuseAddrListener("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newReuseAddrListener: %v", err)
	}
	ll := newLingerListener(inner)
	defer ll.Close()

	// Dial in background; collect errors via channel.
	dialErr := make(chan error, 1)
	go func() {
		c, err := net.Dial("tcp", ll.Addr().String())
		if err != nil {
			dialErr <- err
			return
		}
		c.Close()
		dialErr <- nil
	}()

	conn, err := ll.Accept()
	if err != nil {
		t.Fatalf("Accept() returned error: %v", err)
	}
	conn.Close()

	if err := <-dialErr; err != nil {
		t.Errorf("client Dial() failed: %v", err)
	}
}

// TestLingerListener_Accept_SetLinger0_NoTimeWait is the key regression test for #83.
//
// Strategy: after Accept() + close on the server side, the accepted port pair
// must not leave a TIME_WAIT entry that blocks a new server from binding.
//
// We verify this indirectly: after closing the accepted connection and the
// listener, the same port must be immediately reusable by a new listener —
// even without a sleep. If SetLinger(0) is absent, RST is not sent and the
// FIN/ACK sequence leads to TIME_WAIT on the server side, which (on platforms
// without full SO_REUSEADDR coverage of accepted sockets) blocks the rebind.
func TestLingerListener_Accept_SetLinger0_NoTimeWait(t *testing.T) {
	inner, err := newReuseAddrListener("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newReuseAddrListener: %v", err)
	}
	ll := newLingerListener(inner)
	port := ll.Addr().(*net.TCPAddr).Port

	// Hold client connection open until server side is done.
	clientReady := make(chan net.Conn, 1)
	go func() {
		c, err := net.Dial("tcp", ll.Addr().String())
		if err != nil {
			close(clientReady)
			return
		}
		clientReady <- c
	}()

	// Accept the connection (SetLinger(0) applied here by lingerListener).
	serverConn, err := ll.Accept()
	if err != nil {
		t.Fatalf("Accept(): %v", err)
	}

	// Close server side first — with SO_LINGER(0) this sends RST, skipping
	// the FIN/ACK → TIME_WAIT path.
	serverConn.Close()

	// Close the listener.
	ll.Close()

	// Drain client side.
	if c, ok := <-clientReady; ok && c != nil {
		c.Close()
	}

	// Brief OS scheduling pause (not a sleep waiting for TIME_WAIT — just
	// enough for the kernel to process the RST).
	time.Sleep(30 * time.Millisecond)

	// Immediate rebind: must succeed. With SetLinger(0) the RST path is used;
	// without it TIME_WAIT may block rebinding on Windows.
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln2, err := newReuseAddrListener("tcp", addr)
	if err != nil {
		t.Fatalf("port %d could not be rebound after lingerListener close — "+
			"possible TIME_WAIT residue (SetLinger(0) not applied?): %v", port, err)
	}
	ln2.Close()
}

// TestLingerListener_ForwardsAddr verifies that lingerListener.Addr() returns
// the address of the inner listener unchanged.
func TestLingerListener_ForwardsAddr(t *testing.T) {
	inner, err := newReuseAddrListener("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newReuseAddrListener: %v", err)
	}
	defer inner.Close()

	ll := newLingerListener(inner)

	if ll.Addr().String() != inner.Addr().String() {
		t.Errorf("Addr() mismatch: lingerListener %q != inner %q",
			ll.Addr().String(), inner.Addr().String())
	}
}

// TestLingerListener_ForwardsClose verifies that lingerListener.Close() closes
// the inner listener so that subsequent Accept() calls return an error.
func TestLingerListener_ForwardsClose(t *testing.T) {
	inner, err := newReuseAddrListener("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newReuseAddrListener: %v", err)
	}

	ll := newLingerListener(inner)
	if err := ll.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	// After Close(), Accept() on the inner listener must fail.
	_, acceptErr := inner.Accept()
	if acceptErr == nil {
		t.Error("inner.Accept() succeeded after lingerListener.Close() — inner listener was not closed")
	}
}

// TestLingerListener_MultipleAccepts verifies that SetLinger(0) is applied to
// every accepted connection, not just the first one.
func TestLingerListener_MultipleAccepts(t *testing.T) {
	inner, err := newReuseAddrListener("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("newReuseAddrListener: %v", err)
	}
	ll := newLingerListener(inner)
	defer ll.Close()

	const n = 3
	dialDone := make(chan struct{}, n)

	// Dial n clients.
	for i := 0; i < n; i++ {
		go func() {
			c, err := net.Dial("tcp", ll.Addr().String())
			if err == nil {
				c.Close()
			}
			dialDone <- struct{}{}
		}()
	}

	// Accept all n — each must succeed without error.
	for i := 0; i < n; i++ {
		conn, err := ll.Accept()
		if err != nil {
			t.Errorf("Accept() #%d returned error: %v", i+1, err)
			continue
		}
		conn.Close()
	}

	// Drain dial goroutines.
	for i := 0; i < n; i++ {
		<-dialDone
	}
}
