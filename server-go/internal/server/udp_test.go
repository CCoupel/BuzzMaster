package server

import (
	"buzzcontrol/internal/protocol"
	"net"
	"testing"
)

func TestUDPBroadcaster_StartStop(t *testing.T) {
	udp := NewUDPBroadcaster(1234)

	if err := udp.Start(); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	udp.Stop()
}

func TestUDPBroadcaster_Broadcast(t *testing.T) {
	udp := NewUDPBroadcaster(9999) // Use high port

	if err := udp.Start(); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}
	defer udp.Stop()

	msg, _ := protocol.NewMessage(protocol.ActionHello, nil)
	err := udp.Broadcast(msg)
	if err != nil {
		t.Errorf("Broadcast failed: %v", err)
	}
}

func TestUDPBroadcaster_BroadcastNotStarted(t *testing.T) {
	udp := NewUDPBroadcaster(1234)
	// Don't call Start()

	msg, _ := protocol.NewMessage(protocol.ActionHello, nil)
	err := udp.Broadcast(msg)
	if err == nil {
		t.Error("Expected error when broadcasting before start")
	}
}

func TestUDPBroadcaster_BroadcastRaw(t *testing.T) {
	udp := NewUDPBroadcaster(9999)

	if err := udp.Start(); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}
	defer udp.Stop()

	err := udp.BroadcastRaw([]byte("test data"))
	if err != nil {
		t.Errorf("BroadcastRaw failed: %v", err)
	}
}

func TestUDPBroadcaster_BroadcastRawNotStarted(t *testing.T) {
	udp := NewUDPBroadcaster(1234)

	err := udp.BroadcastRaw([]byte("test"))
	if err == nil {
		t.Error("Expected error when broadcasting raw before start")
	}
}

func TestCalculateBroadcast(t *testing.T) {
	tests := []struct {
		ip       net.IP
		mask     net.IPMask
		expected net.IP
	}{
		{
			ip:       net.IP{192, 168, 1, 100},
			mask:     net.IPMask{255, 255, 255, 0},
			expected: net.IP{192, 168, 1, 255},
		},
		{
			ip:       net.IP{192, 168, 4, 1},
			mask:     net.IPMask{255, 255, 255, 0},
			expected: net.IP{192, 168, 4, 255},
		},
		{
			ip:       net.IP{10, 0, 0, 5},
			mask:     net.IPMask{255, 0, 0, 0},
			expected: net.IP{10, 255, 255, 255},
		},
		{
			ip:       net.IP{172, 16, 0, 10},
			mask:     net.IPMask{255, 255, 0, 0},
			expected: net.IP{172, 16, 255, 255},
		},
	}

	for _, test := range tests {
		result := calculateBroadcast(test.ip, test.mask)
		if !result.Equal(test.expected) {
			t.Errorf("For IP %v mask %v, expected %v, got %v",
				test.ip, test.mask, test.expected, result)
		}
	}
}

func TestCalculateBroadcast_InvalidInput(t *testing.T) {
	// IPv6 (too long)
	result := calculateBroadcast(net.ParseIP("::1"), net.IPMask{255, 255, 255, 0})
	if result != nil {
		t.Error("Expected nil for IPv6 address")
	}

	// Nil input
	result = calculateBroadcast(nil, nil)
	if result != nil {
		t.Error("Expected nil for nil input")
	}

	// Wrong mask length
	result = calculateBroadcast(net.IP{192, 168, 1, 1}, net.IPMask{255, 255})
	if result != nil {
		t.Error("Expected nil for wrong mask length")
	}
}
