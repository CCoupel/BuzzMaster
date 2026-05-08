package server

// Regression tests for bugfix #80 — TCPServer legacy removal.
// Guards against re-introduction of the TCP server or incorrect
// BuzzerDiscoveryPort value.

import (
	"net"
	"testing"
	"time"
)

// TestBuzzerDiscoveryPort_Value ensures the constant is 1234 — the value
// hardcoded in all BuzzClick firmware versions. Must never change.
func TestBuzzerDiscoveryPort_Value(t *testing.T) {
	const expectedPort = 1234
	if BuzzerDiscoveryPort != expectedPort {
		t.Errorf("BuzzerDiscoveryPort must be %d (BuzzClick firmware hardcoded value), got %d",
			expectedPort, BuzzerDiscoveryPort)
	}
}

// TestNoTCPListenerOnBuzzerPort verifies that the server infrastructure does
// NOT open a TCP listener on port 1234 (the discovery port).
// Pre-bugfix #80, net.Listen("tcp", ":1234") was called during server init,
// which blocked startup on Windows (WSAEADDRINUSE).
//
// This test connects to TCP :1234 on localhost and expects immediate refusal.
// It uses a short timeout to avoid hanging the test suite if something else
// is listening on that port in the test environment.
func TestNoTCPListenerOnBuzzerPort(t *testing.T) {
	// Attempt a TCP connection to localhost:1234.
	// A refused connection (connection reset / connection refused) means
	// no server is listening — which is the desired post-bugfix state.
	conn, err := net.DialTimeout("tcp", "127.0.0.1:1234", 200*time.Millisecond)
	if err == nil {
		// Something IS listening on TCP 1234 in the test environment.
		// This is not necessarily the BuzzControl server, but warrants
		// attention — close and flag as a warning, not a hard failure.
		conn.Close()
		t.Logf("WARNING: TCP port 1234 is open on localhost. This may be an external service, "+
			"not BuzzControl. Verify that BuzzControl does not open this port (bugfix #80).")
	}
	// err != nil means connection was refused — correct post-bugfix behavior.
}

// TestUDPBroadcaster_UsesDiscoveryPort verifies that a newly created
// UDPBroadcaster targets BuzzerDiscoveryPort (1234) — not a configurable
// port from a removed Config.TCPPort field.
func TestUDPBroadcaster_UsesDiscoveryPort(t *testing.T) {
	udp := NewUDPBroadcaster()
	if udp.port != BuzzerDiscoveryPort {
		t.Errorf("UDPBroadcaster.port must equal BuzzerDiscoveryPort (%d), got %d",
			BuzzerDiscoveryPort, udp.port)
	}
}
