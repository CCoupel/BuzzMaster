package server

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	// BroadcastMsgPrefix is the prefix for server discovery UDP messages
	BroadcastMsgPrefix = "BUZZ_SERVER"

	// BroadcastIntervalNormal is the heartbeat interval during normal operation
	BroadcastIntervalNormal = 5 * time.Second

	// BroadcastIntervalEnrollment is the heartbeat interval during buzzer enrollment
	BroadcastIntervalEnrollment = 1 * time.Second

	// BroadcastIntervalHighFrequency is the heartbeat interval when at least one
	// known buzzer is disconnected — helps it rediscover the server quickly (v3.6.5)
	BroadcastIntervalHighFrequency = 500 * time.Millisecond
)

// BroadcasterManager sends periodic UDP heartbeats so BuzzClick buzzers
// can discover the server IP automatically without manual configuration.
//
// Format: BUZZ_SERVER|<IP1>|<IP2>|...|<PORT>\0
// Example: BUZZ_SERVER|192.168.1.50|10.0.0.50|80\0
type BroadcasterManager struct {
	udp        *UDPBroadcaster
	httpPort   int
	enrollment bool
	highFreq   bool // true when at least one known buzzer is disconnected (v3.6.5)
	mu         sync.RWMutex
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewBroadcasterManager creates a new BroadcasterManager.
// httpPort is the HTTP server port to broadcast (80 or 443).
func NewBroadcasterManager(udp *UDPBroadcaster, httpPort int) *BroadcasterManager {
	return &BroadcasterManager{
		udp:      udp,
		httpPort: httpPort,
		stopCh:   make(chan struct{}),
	}
}

// Start begins sending periodic UDP heartbeats in a background goroutine.
func (b *BroadcasterManager) Start() {
	b.wg.Add(1)
	go b.loop()
	log.Printf("[UDP] BroadcasterManager started (interval=%s, port=%d)", BroadcastIntervalNormal, b.httpPort)
}

// Stop signals the background goroutine to exit and waits for it to finish.
func (b *BroadcasterManager) Stop() {
	close(b.stopCh)
	b.wg.Wait()
}

// SetEnrollmentMode switches between fast (1s) and normal (5s) heartbeat intervals.
// Set to true during buzzer enrollment/pairing, false otherwise.
func (b *BroadcasterManager) SetEnrollmentMode(active bool) {
	b.mu.Lock()
	b.enrollment = active
	b.mu.Unlock()
}

// SetHighFrequency switches to a 500ms heartbeat interval when at least one known
// buzzer is disconnected, so it can rediscover the server quickly after reconnect.
// Enrollment mode takes priority over high-frequency mode.
func (b *BroadcasterManager) SetHighFrequency(active bool) {
	b.mu.Lock()
	b.highFreq = active
	b.mu.Unlock()
}

// SendNow triggers an immediate heartbeat broadcast outside of the regular interval.
func (b *BroadcasterManager) SendNow() {
	if err := b.broadcast(); err != nil {
		log.Printf("[UDP] BroadcasterManager SendNow error: %v", err)
	}
}

// BuildHeartbeat assembles the BUZZ_SERVER heartbeat message for the given IPs and port.
// Returns the null-terminated byte slice to send.
func BuildHeartbeat(ips []string, port int) []byte {
	parts := make([]string, 0, len(ips)+1)
	parts = append(parts, BroadcastMsgPrefix)
	parts = append(parts, ips...)
	parts = append(parts, fmt.Sprintf("%d", port))
	msg := strings.Join(parts, "|")
	// Null-terminate (consistent with existing TCP/UDP protocol convention)
	return append([]byte(msg), 0)
}

// GetServerIPs returns all active IPv4 addresses of the server,
// excluding loopback and link-local addresses.
func GetServerIPs() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("[UDP] BroadcasterManager: failed to enumerate interfaces: %v", err)
		return nil
	}

	var ips []string
	for _, iface := range interfaces {
		// Skip loopback and interfaces that are down
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// IPv4 only
			ip4 := ip.To4()
			if ip4 == nil {
				continue
			}

			// Skip loopback (127.x.x.x) and link-local (169.254.x.x)
			if ip4[0] == 127 {
				continue
			}
			if ip4[0] == 169 && ip4[1] == 254 {
				continue
			}

			ips = append(ips, ip4.String())
		}
	}

	return ips
}

// loop is the background goroutine that sends heartbeats on a ticker.
func (b *BroadcasterManager) loop() {
	defer b.wg.Done()

	// Send an immediate heartbeat so buzzers discover the server quickly at startup.
	if err := b.broadcast(); err != nil {
		log.Printf("[UDP] BroadcasterManager initial broadcast error: %v", err)
	}

	ticker := time.NewTicker(b.interval())
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			if err := b.broadcast(); err != nil {
				log.Printf("[UDP] BroadcasterManager broadcast error: %v", err)
			}
			// Adjust ticker period if enrollment mode changed
			ticker.Reset(b.interval())
		}
	}
}

// interval returns the current heartbeat interval.
// Priority: enrollment (1s) > high-frequency (500ms) > normal (5s).
func (b *BroadcasterManager) interval() time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.enrollment {
		return BroadcastIntervalEnrollment
	}
	if b.highFreq {
		return BroadcastIntervalHighFrequency
	}
	return BroadcastIntervalNormal
}

// broadcast detects current IPs, builds the heartbeat, and sends it.
func (b *BroadcasterManager) broadcast() error {
	ips := GetServerIPs()
	if len(ips) == 0 {
		log.Printf("[UDP] BroadcasterManager: no active IPs found, skipping broadcast")
		return nil
	}

	data := BuildHeartbeat(ips, b.httpPort)

	log.Printf("[UDP] Broadcasting BUZZ_SERVER heartbeat: IPs=%v port=%d", ips, b.httpPort)

	return b.udp.BroadcastRaw(data)
}
