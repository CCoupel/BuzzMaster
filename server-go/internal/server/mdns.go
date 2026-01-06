package server

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/grandcat/zeroconf"
)

// MDNSServer advertises the BuzzControl service via mDNS
// Uses zeroconf for service discovery (used by BuzzClick ESP32)
// Note: Direct hostname resolution (buzzcontrol.local) requires dnsmasq on Raspberry Pi
//       or hosts file entry on Windows/Linux for development
type MDNSServer struct {
	hostname string
	httpPort int
	tcpPort  int
	servers  []*zeroconf.Server
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewMDNSServer creates a new mDNS server
func NewMDNSServer(hostname string, httpPort, tcpPort int) *MDNSServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &MDNSServer{
		hostname: hostname,
		httpPort: httpPort,
		tcpPort:  tcpPort,
		servers:  make([]*zeroconf.Server, 0),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start begins advertising the service via mDNS
func (m *MDNSServer) Start() error {
	// Get the local IP addresses
	ips, err := getLocalIPs()
	if err != nil {
		return err
	}

	// Convert IPs to strings for RegisterProxy
	var ipStrings []string
	for _, ip := range ips {
		ipStrings = append(ipStrings, ip.String())
	}

	// Get machine hostname for logging
	machineHost, _ := os.Hostname()

	// Service info as TXT records
	txtRecords := []string{"BuzzControl Quiz Server", "version=1.0"}

	// Use RegisterProxy to set custom hostname (buzzcontrol instead of machine name)
	// The library will add .local. automatically
	customHost := m.hostname // e.g., "buzzcontrol"

	// Service 1: _http._tcp on port 80 (for web browsers)
	httpServer, err := zeroconf.RegisterProxy(
		m.hostname,                   // Instance name
		"_http._tcp",                 // Service type
		"local.",                     // Domain
		m.httpPort,                   // Port
		customHost,                   // Host name (buzzcontrol.local.)
		ipStrings,                    // IPs as strings
		txtRecords,                   // TXT records
		getInterfacesForIPs(ips),     // Network interfaces
	)
	if err != nil {
		log.Printf("[mDNS] Failed to register _http._tcp: %v", err)
	} else {
		m.servers = append(m.servers, httpServer)
	}

	// Service 2: _sock._tcp on TCP port (for BuzzClick discovery)
	// This is the service that BuzzClick clients query to find the server
	sockServer, err := zeroconf.RegisterProxy(
		m.hostname,
		"_sock._tcp", // BuzzClick queries this service type
		"local.",
		m.tcpPort,
		customHost, // Host name (buzzcontrol.local.)
		ipStrings,
		txtRecords,
		getInterfacesForIPs(ips),
	)
	if err != nil {
		log.Printf("[mDNS] Failed to register _sock._tcp: %v", err)
	} else {
		m.servers = append(m.servers, sockServer)
	}

	// Service 3: _buzzcontrol._tcp (custom service)
	bzServer, err := zeroconf.RegisterProxy(
		m.hostname,
		"_buzzcontrol._tcp",
		"local.",
		m.tcpPort,
		customHost,
		ipStrings,
		txtRecords,
		getInterfacesForIPs(ips),
	)
	if err != nil {
		log.Printf("[mDNS] Failed to register _buzzcontrol._tcp: %v", err)
	} else {
		m.servers = append(m.servers, bzServer)
	}

	log.Printf("[mDNS] Advertising %s.local (machine: %s)", m.hostname, machineHost)
	log.Printf("[mDNS]   _http._tcp on port %d", m.httpPort)
	log.Printf("[mDNS]   _sock._tcp on port %d (for BuzzClick)", m.tcpPort)
	log.Printf("[mDNS]   _buzzcontrol._tcp on port %d", m.tcpPort)
	for _, ip := range ips {
		log.Printf("[mDNS]   IP: %s", ip.String())
	}

	return nil
}

// Stop shuts down the mDNS servers
func (m *MDNSServer) Stop() {
	m.cancel()
	for _, server := range m.servers {
		if server != nil {
			server.Shutdown()
		}
	}
	if len(m.servers) > 0 {
		log.Println("[mDNS] Servers stopped")
	}
}

// getLocalIPs returns all non-loopback IPv4 addresses
func getLocalIPs() ([]net.IP, error) {
	var ips []net.IP

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		// Skip down interfaces and loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
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

			// Only IPv4 addresses
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			ips = append(ips, ip.To4())
		}
	}

	// Fallback to loopback if no other IPs found
	if len(ips) == 0 {
		ips = append(ips, net.ParseIP("127.0.0.1"))
	}

	return ips, nil
}

// getInterfacesForIPs returns network interfaces that have the given IPs
func getInterfacesForIPs(ips []net.IP) []net.Interface {
	var result []net.Interface

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
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

			if ip == nil {
				continue
			}

			// Check if this IP is in our list
			for _, wantedIP := range ips {
				if ip.Equal(wantedIP) {
					result = append(result, iface)
					break
				}
			}
		}
	}

	return result
}
