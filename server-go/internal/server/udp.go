package server

import (
	"buzzcontrol/internal/protocol"
	"fmt"
	"log"
	"net"
)

// UDPBroadcaster handles UDP broadcast messages to BuzzClick buzzers
type UDPBroadcaster struct {
	port int
	conn *net.UDPConn
}

// NewUDPBroadcaster creates a new UDP broadcaster
func NewUDPBroadcaster(port int) *UDPBroadcaster {
	return &UDPBroadcaster{
		port: port,
	}
}

// Start initializes the UDP broadcaster
func (u *UDPBroadcaster) Start() error {
	// Create UDP connection for broadcasting
	addr := &net.UDPAddr{
		Port: 0, // Let OS assign port for sending
		IP:   net.IPv4zero,
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return fmt.Errorf("failed to create UDP socket: %w", err)
	}

	u.conn = conn
	log.Printf("[UDP] Broadcaster ready, target port %d", u.port)
	return nil
}

// Stop closes the UDP connection
func (u *UDPBroadcaster) Stop() {
	if u.conn != nil {
		u.conn.Close()
	}
}

// Broadcast sends a message to all devices on the network
func (u *UDPBroadcaster) Broadcast(msg *protocol.Message) error {
	if u.conn == nil {
		return fmt.Errorf("UDP broadcaster not started")
	}

	data, err := msg.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Get all broadcast addresses
	broadcastAddrs := u.getBroadcastAddresses()

	if len(broadcastAddrs) == 0 {
		// Fallback to common broadcast
		broadcastAddrs = []net.IP{net.IPv4(192, 168, 4, 255)}
	}

	var lastErr error
	successCount := 0

	for _, broadcastIP := range broadcastAddrs {
		destAddr := &net.UDPAddr{
			IP:   broadcastIP,
			Port: u.port,
		}

		n, err := u.conn.WriteToUDP(data, destAddr)
		if err != nil {
			log.Printf("[UDP] Failed to broadcast to %s: %v", broadcastIP, err)
			lastErr = err
		} else {
			log.Printf("[UDP] Broadcast %d bytes to %s (ACTION=%s)", n, broadcastIP, msg.Action)
			successCount++
		}
	}

	if successCount == 0 && lastErr != nil {
		return lastErr
	}

	return nil
}

// getBroadcastAddresses returns broadcast addresses for all network interfaces
func (u *UDPBroadcaster) getBroadcastAddresses() []net.IP {
	var broadcasts []net.IP

	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("[UDP] Failed to get interfaces: %v", err)
		return broadcasts
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP.To4()
			if ip == nil {
				continue
			}

			// Calculate broadcast address
			broadcast := calculateBroadcast(ip, ipNet.Mask)
			if broadcast != nil {
				broadcasts = append(broadcasts, broadcast)
			}
		}
	}

	return broadcasts
}

// calculateBroadcast calculates the broadcast address from IP and mask
func calculateBroadcast(ip net.IP, mask net.IPMask) net.IP {
	if len(ip) != 4 || len(mask) != 4 {
		return nil
	}

	broadcast := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		broadcast[i] = ip[i] | ^mask[i]
	}
	return broadcast
}

// BroadcastRaw sends raw bytes via UDP broadcast
func (u *UDPBroadcaster) BroadcastRaw(data []byte) error {
	if u.conn == nil {
		return fmt.Errorf("UDP broadcaster not started")
	}

	broadcastAddrs := u.getBroadcastAddresses()
	if len(broadcastAddrs) == 0 {
		broadcastAddrs = []net.IP{net.IPv4(192, 168, 4, 255)}
	}

	for _, broadcastIP := range broadcastAddrs {
		destAddr := &net.UDPAddr{
			IP:   broadcastIP,
			Port: u.port,
		}

		if _, err := u.conn.WriteToUDP(data, destAddr); err != nil {
			log.Printf("[UDP] Failed to broadcast to %s: %v", broadcastIP, err)
		}
	}

	return nil
}
