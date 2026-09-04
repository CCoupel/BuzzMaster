package server

// Tests for DNSServer.Start() honoring the configured port and surfacing
// bind failures visibly (#220 Lot 0, task 7: internal/server/dns.go:42-57).
//
// Bug before #220: the DNS server's Addr was hardcoded to ":53" regardless
// of d.port, and failures went through the stdlib log.Printf — invisible to
// /ws/logs and the in-memory log buffer. Port 53 being occupied by
// systemd-resolved is the single most common DNS-start failure on Linux,
// yet it was the one case nobody could ever see in the admin log viewer
// (main.go:974's LogWarn call for it never fired, because Start() always
// returned nil).

import (
	"buzzcontrol/internal/game"
	"fmt"
	"net"
	"testing"
	"time"
)

// TestDNSServer_Start_UsesConfiguredPort verifies that DNSServer.Start()
// binds on d.port rather than a hardcoded ":53".
func TestDNSServer_Start_UsesConfiguredPort(t *testing.T) {
	// Grab a free UDP port, then release it immediately so the DNS server can
	// bind it.
	probe, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not grab a free UDP port: %v", err)
	}
	port := probe.LocalAddr().(*net.UDPAddr).Port
	probe.Close()

	d := NewDNSServer(port, net.ParseIP("127.0.0.1"))
	if err := d.Start(); err != nil {
		t.Fatalf("DNSServer.Start() returned error: %v", err)
	}
	defer d.Stop()

	// Poll: the configured port must become unavailable (the DNS server is
	// holding it). If the implementation still hardcodes ":53", this port
	// stays free and the rebind below keeps succeeding, failing the test.
	deadline := time.Now().Add(2 * time.Second)
	bound := false
	for time.Now().Before(deadline) {
		ln, rebindErr := net.ListenPacket("udp", fmt.Sprintf("127.0.0.1:%d", port))
		if rebindErr != nil {
			bound = true
			break
		}
		ln.Close()
		time.Sleep(30 * time.Millisecond)
	}
	if !bound {
		t.Errorf("port %d never became occupied by DNSServer — Start() is not honoring d.port (still hardcoded to :53?)", port)
	}
}

// TestDNSServer_Start_FailureIsVisible verifies that a DNS bind failure is
// reported through LogWarn (visible in /ws/logs and the log buffer) rather
// than only through the stdlib log.Printf. DNS/mDNS failures must remain
// non-fatal for HTTP startup, but they must stop being silent.
func TestDNSServer_Start_FailureIsVisible(t *testing.T) {
	previous := GetGlobalLogger()
	bl := NewBroadcastLogger(100)
	SetGlobalLogger(bl)
	t.Cleanup(func() { SetGlobalLogger(previous) })

	// Occupy a UDP port on the wildcard-conflicting loopback address first,
	// so the DNS server's own wildcard bind (":<port>") fails.
	blocker, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not grab a free UDP port: %v", err)
	}
	defer blocker.Close()
	port := blocker.LocalAddr().(*net.UDPAddr).Port

	d := NewDNSServer(port, net.ParseIP("127.0.0.1"))
	// Start() itself may still return nil (ListenAndServe runs fire-and-forget
	// in a goroutine, as it did before #220) — the point of this test is the
	// log side, asserted below, not the synchronous return value.
	_ = d.Start()
	defer d.Stop()

	deadline := time.Now().Add(2 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		for _, entry := range bl.GetHistory() {
			if entry.Level == game.LogLevelWarn {
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
		t.Error("DNS bind failure produced no WARN log entry via the broadcast logger — failure is not visible in /ws/logs (still using stdlib log.Printf?)")
	}
}
