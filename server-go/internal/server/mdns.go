package server

import (
	"log"
	"net"
	"os"

	"github.com/hashicorp/mdns"
)

// MDNSServer advertises the BuzzControl service via mDNS
type MDNSServer struct {
	hostname string
	httpPort int
	tcpPort  int
	servers  []*mdns.Server
}

// NewMDNSServer creates a new mDNS server
func NewMDNSServer(hostname string, httpPort, tcpPort int) *MDNSServer {
	return &MDNSServer{
		hostname: hostname,
		httpPort: httpPort,
		tcpPort:  tcpPort,
		servers:  make([]*mdns.Server, 0),
	}
}

// Start begins advertising the service via mDNS
func (m *MDNSServer) Start() error {
	// Get the local IP addresses
	ips, err := getLocalIPs()
	if err != nil {
		return err
	}

	// Get hostname for the service
	host, _ := os.Hostname()

	// Service info
	info := []string{"BuzzControl Quiz Server"}

	// Service 1: _http._tcp on port 80 (for web browsers)
	httpService, err := mdns.NewMDNSService(
		m.hostname,      // Instance name
		"_http._tcp",    // Service type
		"",              // Domain (empty = .local)
		m.hostname+".",  // Host name (with trailing dot)
		m.httpPort,      // Port
		ips,             // IPs to advertise
		info,            // TXT record
	)
	if err != nil {
		return err
	}

	httpServer, err := mdns.NewServer(&mdns.Config{Zone: httpService})
	if err != nil {
		return err
	}
	m.servers = append(m.servers, httpServer)

	// Service 2: _sock._tcp on TCP port (for BuzzClick discovery)
	// This is the service that BuzzClick clients query to find the server
	sockService, err := mdns.NewMDNSService(
		m.hostname,      // Instance name
		"_sock._tcp",    // Service type - BuzzClick queries this!
		"",              // Domain
		m.hostname+".",  // Host name
		m.tcpPort,       // TCP port for buzzers
		ips,
		info,
	)
	if err != nil {
		return err
	}

	sockServer, err := mdns.NewServer(&mdns.Config{Zone: sockService})
	if err != nil {
		return err
	}
	m.servers = append(m.servers, sockServer)

	// Service 3: _buzzcontrol._tcp (custom service)
	bzService, err := mdns.NewMDNSService(
		m.hostname,
		"_buzzcontrol._tcp",
		"",
		m.hostname+".",
		m.tcpPort,
		ips,
		info,
	)
	if err != nil {
		return err
	}

	bzServer, err := mdns.NewServer(&mdns.Config{Zone: bzService})
	if err != nil {
		return err
	}
	m.servers = append(m.servers, bzServer)

	log.Printf("[mDNS] Advertising %s.local (host: %s)", m.hostname, host)
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
