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

	"github.com/miekg/dns"
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

	// #263: observe the server by asking it a question, never by trying to bind
	// the port ourselves. The old probe (ListenPacket on the same port) could
	// win the race against the server's own bind (started in a goroutine): the
	// server then failed once with "address already in use" and never retried,
	// so the port stayed free forever (CI run 38239127708). A DNS answer from
	// 127.0.0.1:<port> proves that the server listens on d.port (and not on a
	// hardcoded :53). Generous overall deadline: the bind goroutine may be
	// scheduled late on a loaded 2-vCPU runner.
	c := &dns.Client{Net: "udp", Timeout: time.Second}
	q := new(dns.Msg)
	q.SetQuestion("buzzcontrol.local.", dns.TypeA)
	deadline := time.Now().Add(20 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, _, exErr := c.Exchange(q, fmt.Sprintf("127.0.0.1:%d", port))
		if exErr == nil && len(resp.Answer) == 1 {
			if a, ok := resp.Answer[0].(*dns.A); ok && a.A.Equal(net.ParseIP("127.0.0.1")) {
				return
			}
			t.Fatalf("unexpected DNS answer: %v", resp.Answer)
		}
		lastErr = exErr
	}
	t.Errorf("DNSServer never answered on port %d — Start() is not honoring d.port (still hardcoded to :53?); last error: %v", port, lastErr)
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

	deadline := time.Now().Add(20 * time.Second)
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
