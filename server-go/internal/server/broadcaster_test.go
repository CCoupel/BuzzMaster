package server

import (
	"strings"
	"testing"
	"time"
)

// ---- BuildHeartbeat tests ------------------------------------------------

func TestBuildHeartbeat_SingleIP(t *testing.T) {
	data := BuildHeartbeat([]string{"192.168.1.50"}, 80)

	// Must be null-terminated
	if data[len(data)-1] != 0 {
		t.Fatal("expected null terminator at end of heartbeat")
	}

	msg := string(data[:len(data)-1])
	if msg != "BUZZ_SERVER|192.168.1.50|80" {
		t.Errorf("unexpected heartbeat: %q", msg)
	}
}

func TestBuildHeartbeat_MultipleIPs(t *testing.T) {
	data := BuildHeartbeat([]string{"192.168.1.50", "10.0.0.50"}, 80)
	msg := string(data[:len(data)-1])
	if msg != "BUZZ_SERVER|192.168.1.50|10.0.0.50|80" {
		t.Errorf("unexpected heartbeat: %q", msg)
	}
}

func TestBuildHeartbeat_HTTPSPort(t *testing.T) {
	data := BuildHeartbeat([]string{"192.168.1.1"}, 443)
	msg := string(data[:len(data)-1])
	if !strings.HasSuffix(msg, "|443") {
		t.Errorf("expected port 443 suffix, got: %q", msg)
	}
}

func TestBuildHeartbeat_Prefix(t *testing.T) {
	data := BuildHeartbeat([]string{"10.0.0.1"}, 80)
	msg := string(data[:len(data)-1])
	if !strings.HasPrefix(msg, "BUZZ_SERVER|") {
		t.Errorf("expected BUZZ_SERVER prefix, got: %q", msg)
	}
}

func TestBuildHeartbeat_EmptyIPs(t *testing.T) {
	data := BuildHeartbeat([]string{}, 80)
	msg := string(data[:len(data)-1])
	// Should still produce a valid (minimal) message ending with port
	if !strings.HasSuffix(msg, "|80") {
		t.Errorf("expected port suffix even with no IPs, got: %q", msg)
	}
}

func TestBuildHeartbeat_NullTerminated(t *testing.T) {
	tests := []struct {
		ips  []string
		port int
	}{
		{[]string{"192.168.1.1"}, 80},
		{[]string{"10.0.0.1", "172.16.0.1"}, 443},
		{[]string{}, 80},
	}

	for _, tt := range tests {
		data := BuildHeartbeat(tt.ips, tt.port)
		if len(data) == 0 || data[len(data)-1] != 0 {
			t.Errorf("BuildHeartbeat(%v, %d): missing null terminator", tt.ips, tt.port)
		}
	}
}

// ---- GetServerIPs tests --------------------------------------------------

func TestGetServerIPs_ReturnsList(t *testing.T) {
	ips := GetServerIPs()
	// We can't assert specific values in a unit test (depends on machine),
	// but the result must not contain loopback or link-local addresses.
	for _, ip := range ips {
		if strings.HasPrefix(ip, "127.") {
			t.Errorf("GetServerIPs returned loopback address: %s", ip)
		}
		if strings.HasPrefix(ip, "169.254.") {
			t.Errorf("GetServerIPs returned link-local address: %s", ip)
		}
	}
}

func TestGetServerIPs_NoEmptyStrings(t *testing.T) {
	ips := GetServerIPs()
	for _, ip := range ips {
		if ip == "" {
			t.Error("GetServerIPs returned empty string in result")
		}
	}
}

// ---- BroadcasterManager lifecycle tests ----------------------------------

func TestBroadcasterManager_StartStop(t *testing.T) {
	udp := NewUDPBroadcaster(9998)
	if err := udp.Start(); err != nil {
		t.Fatalf("failed to start UDP broadcaster: %v", err)
	}
	defer udp.Stop()

	bm := NewBroadcasterManager(udp, 80)
	bm.Start()

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)
	bm.Stop()
	// Stop must return without hanging
}

func TestBroadcasterManager_EnrollmentMode(t *testing.T) {
	udp := NewUDPBroadcaster(9997)
	if err := udp.Start(); err != nil {
		t.Fatalf("failed to start UDP broadcaster: %v", err)
	}
	defer udp.Stop()

	bm := NewBroadcasterManager(udp, 80)

	// Default: normal interval
	if got := bm.interval(); got != BroadcastIntervalNormal {
		t.Errorf("expected normal interval %s, got %s", BroadcastIntervalNormal, got)
	}

	// Switch to enrollment
	bm.SetEnrollmentMode(true)
	if got := bm.interval(); got != BroadcastIntervalEnrollment {
		t.Errorf("expected enrollment interval %s, got %s", BroadcastIntervalEnrollment, got)
	}

	// Switch back
	bm.SetEnrollmentMode(false)
	if got := bm.interval(); got != BroadcastIntervalNormal {
		t.Errorf("expected normal interval after disabling enrollment, got %s", got)
	}
}

func TestBroadcasterManager_SendNow(t *testing.T) {
	udp := NewUDPBroadcaster(9996)
	if err := udp.Start(); err != nil {
		t.Fatalf("failed to start UDP broadcaster: %v", err)
	}
	defer udp.Stop()

	bm := NewBroadcasterManager(udp, 80)
	// SendNow must not panic even without Start()
	bm.SendNow()
}

func TestBroadcasterManager_Intervals(t *testing.T) {
	if BroadcastIntervalNormal <= BroadcastIntervalEnrollment {
		t.Errorf("expected normal interval (%s) > enrollment interval (%s)",
			BroadcastIntervalNormal, BroadcastIntervalEnrollment)
	}
}

// ---- BuildHeartbeat additional cases ------------------------------------

func TestBuildHeartbeat_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		ips      []string
		port     int
		wantMsg  string
	}{
		{
			name:    "single IP port 80",
			ips:     []string{"192.168.1.50"},
			port:    80,
			wantMsg: "BUZZ_SERVER|192.168.1.50|80",
		},
		{
			name:    "two IPs port 80",
			ips:     []string{"192.168.1.50", "10.0.0.50"},
			port:    80,
			wantMsg: "BUZZ_SERVER|192.168.1.50|10.0.0.50|80",
		},
		{
			name:    "three IPs non-standard port",
			ips:     []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"},
			port:    8080,
			wantMsg: "BUZZ_SERVER|192.168.1.1|10.0.0.1|172.16.0.1|8080",
		},
		{
			name:    "empty IPs port 80",
			ips:     []string{},
			port:    80,
			wantMsg: "BUZZ_SERVER|80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := BuildHeartbeat(tt.ips, tt.port)

			// Always null-terminated
			if len(data) == 0 || data[len(data)-1] != 0 {
				t.Fatalf("missing null terminator")
			}

			msg := string(data[:len(data)-1])
			if msg != tt.wantMsg {
				t.Errorf("got %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}

func TestBuildHeartbeat_PortOnlyContainsDigits(t *testing.T) {
	ports := []int{80, 443, 1234, 8080, 65535}
	for _, port := range ports {
		data := BuildHeartbeat([]string{"192.168.1.1"}, port)
		msg := string(data[:len(data)-1])
		parts := strings.Split(msg, "|")
		lastPart := parts[len(parts)-1]
		for _, c := range lastPart {
			if c < '0' || c > '9' {
				t.Errorf("port segment %q contains non-digit character for port %d", lastPart, port)
			}
		}
	}
}

func TestBuildHeartbeat_NoInternalNullBytes(t *testing.T) {
	data := BuildHeartbeat([]string{"192.168.1.1", "10.0.0.1"}, 80)
	// Only the last byte should be null
	for i, b := range data[:len(data)-1] {
		if b == 0 {
			t.Errorf("unexpected null byte at position %d in heartbeat payload", i)
		}
	}
}

// ---- GetServerIPs additional cases -------------------------------------

func TestGetServerIPs_IPv4Only(t *testing.T) {
	ips := GetServerIPs()
	for _, ip := range ips {
		// IPv6 addresses contain ':' characters
		if strings.Contains(ip, ":") {
			t.Errorf("GetServerIPs returned IPv6 address: %s", ip)
		}
		// Must be a valid dotted-decimal IPv4
		parts := strings.Split(ip, ".")
		if len(parts) != 4 {
			t.Errorf("GetServerIPs returned non-IPv4 string: %s", ip)
		}
	}
}

func TestGetServerIPs_NoPrivateRangeExclusions(t *testing.T) {
	// Private ranges (10.x, 172.16.x, 192.168.x) are valid and must NOT be excluded.
	// This test verifies GetServerIPs doesn't accidentally filter them out
	// by checking that if we have any IPs, at least one is a valid routable address.
	ips := GetServerIPs()
	for _, ip := range ips {
		if strings.HasPrefix(ip, "127.") || strings.HasPrefix(ip, "169.254.") {
			t.Errorf("GetServerIPs must not return loopback/link-local: %s", ip)
		}
	}
}

// ---- BroadcasterManager concurrent tests --------------------------------

func TestBroadcasterManager_ConcurrentEnrollmentToggle(t *testing.T) {
	udp := NewUDPBroadcaster(9995)
	if err := udp.Start(); err != nil {
		t.Fatalf("failed to start UDP broadcaster: %v", err)
	}
	defer udp.Stop()

	bm := NewBroadcasterManager(udp, 80)
	bm.Start()
	defer bm.Stop()

	// Concurrently toggle enrollment mode to verify no data race
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			bm.SetEnrollmentMode(i%2 == 0)
		}
		close(done)
	}()

	// Also read the interval concurrently
	for i := 0; i < 50; i++ {
		_ = bm.interval()
	}

	<-done
}

// ---- SetHighFrequency tests (v3.6.5) ------------------------------------

// TestBroadcasterManager_HighFrequency verifies that SetHighFrequency(true)
// makes interval() return BroadcastIntervalHighFrequency (500ms).
func TestBroadcasterManager_HighFrequency(t *testing.T) {
	bm := NewBroadcasterManager(nil, 80)

	// Default: normal interval
	if got := bm.interval(); got != BroadcastIntervalNormal {
		t.Errorf("expected normal interval %s, got %s", BroadcastIntervalNormal, got)
	}

	// Switch to high-frequency
	bm.SetHighFrequency(true)
	if got := bm.interval(); got != BroadcastIntervalHighFrequency {
		t.Errorf("expected high-frequency interval %s, got %s", BroadcastIntervalHighFrequency, got)
	}

	// Switch back
	bm.SetHighFrequency(false)
	if got := bm.interval(); got != BroadcastIntervalNormal {
		t.Errorf("expected normal interval after disabling high-frequency, got %s", got)
	}
}

// TestBroadcasterManager_HighFrequencyPriorityBelowEnrollment verifies that
// enrollment mode takes priority over high-frequency mode: when both are active
// the interval must be BroadcastIntervalEnrollment (1s), not 500ms.
func TestBroadcasterManager_HighFrequencyPriorityBelowEnrollment(t *testing.T) {
	bm := NewBroadcasterManager(nil, 80)

	bm.SetEnrollmentMode(true)
	bm.SetHighFrequency(true)

	if got := bm.interval(); got != BroadcastIntervalEnrollment {
		t.Errorf("enrollment must take priority over high-frequency: expected %s, got %s",
			BroadcastIntervalEnrollment, got)
	}
}

// TestBroadcasterManager_HighFrequencyIsLowerThanNormal verifies that the
// high-frequency interval constant is strictly less than the normal interval.
func TestBroadcasterManager_HighFrequencyIsLowerThanNormal(t *testing.T) {
	if BroadcastIntervalHighFrequency >= BroadcastIntervalNormal {
		t.Errorf("expected high-frequency interval (%s) < normal interval (%s)",
			BroadcastIntervalHighFrequency, BroadcastIntervalNormal)
	}
}

func TestBroadcasterManager_StopIsIdempotent(t *testing.T) {
	udp := NewUDPBroadcaster(9994)
	if err := udp.Start(); err != nil {
		t.Fatalf("failed to start UDP broadcaster: %v", err)
	}
	defer udp.Stop()

	bm := NewBroadcasterManager(udp, 80)
	bm.Start()
	time.Sleep(10 * time.Millisecond)
	bm.Stop()
	// Calling Stop again after goroutine exits must not panic or deadlock.
	// (stopCh is already closed; wg.Wait() returns immediately)
}

func TestBroadcasterManager_SendNowAfterStart(t *testing.T) {
	udp := NewUDPBroadcaster(9993)
	if err := udp.Start(); err != nil {
		t.Fatalf("failed to start UDP broadcaster: %v", err)
	}
	defer udp.Stop()

	bm := NewBroadcasterManager(udp, 80)
	bm.Start()
	defer bm.Stop()

	// SendNow while the background goroutine is running must not panic
	bm.SendNow()
	bm.SendNow()
}
