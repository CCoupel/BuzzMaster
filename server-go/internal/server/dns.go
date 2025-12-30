package server

import (
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
)

// DNSServer handles DNS requests for captive portal functionality
type DNSServer struct {
	port     int
	serverIP net.IP
	server   *dns.Server
}

// NewDNSServer creates a new DNS server
func NewDNSServer(port int, serverIP net.IP) *DNSServer {
	return &DNSServer{
		port:     port,
		serverIP: serverIP,
	}
}

// Start begins the DNS server
func (d *DNSServer) Start() error {
	// If no IP specified, try to get the first non-loopback IP
	if d.serverIP == nil {
		ips, err := getLocalIPs()
		if err != nil || len(ips) == 0 {
			d.serverIP = net.ParseIP("127.0.0.1")
		} else {
			d.serverIP = ips[0]
		}
	}

	// Create DNS handler
	handler := &dnsHandler{serverIP: d.serverIP}

	// Create server
	d.server = &dns.Server{
		Addr:    ":53",
		Net:     "udp",
		Handler: handler,
	}

	log.Printf("[DNS] Starting DNS server on port 53, redirecting to %s", d.serverIP.String())

	// Start in goroutine
	go func() {
		if err := d.server.ListenAndServe(); err != nil {
			log.Printf("[DNS] Server error: %v", err)
		}
	}()

	return nil
}

// Stop shuts down the DNS server
func (d *DNSServer) Stop() {
	if d.server != nil {
		d.server.Shutdown()
		log.Println("[DNS] Server stopped")
	}
}

// dnsHandler handles DNS queries
type dnsHandler struct {
	serverIP net.IP
}

// ServeDNS handles incoming DNS requests
func (h *dnsHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	for _, q := range r.Question {
		name := strings.ToLower(q.Name)

		switch q.Qtype {
		case dns.TypeA:
			// For buzzcontrol.local or any .local domain, or wildcard
			// Return our server IP
			log.Printf("[DNS] Query for %s -> %s", name, h.serverIP.String())

			rr := &dns.A{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				A: h.serverIP,
			}
			msg.Answer = append(msg.Answer, rr)

		case dns.TypeAAAA:
			// IPv6 - return empty (we only support IPv4)
			// Just don't add any answer

		default:
			// For other query types, just respond with what we have
		}
	}

	w.WriteMsg(msg)
}
