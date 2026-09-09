package main

// Bridge discovery: mDNS (_hue._tcp.local., what the Hue app uses first) and
// SSDP (UPnP M-SEARCH, the bridge answers with "IpBridge" in its SERVER
// header). No cloud call (discovery.meethue.com) — a home network detail
// does not need to leave the LAN for a spike.

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

// Bridge is one discovered bridge.
type Bridge struct {
	IP       string `json:"ip"`
	ID       string `json:"bridgeid,omitempty"`
	Model    string `json:"modelid,omitempty"`
	Source   string `json:"source"` // mdns | ssdp
	Instance string `json:"instance,omitempty"`
}

// discoverMDNS browses _hue._tcp for up to timeout.
func discoverMDNS(ctx context.Context, timeout time.Duration, log func(string, ...any)) ([]Bridge, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("mdns resolver: %w", err)
	}
	entries := make(chan *zeroconf.ServiceEntry, 8)
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := resolver.Browse(cctx, "_hue._tcp", "local.", entries); err != nil {
		return nil, fmt.Errorf("mdns browse: %w", err)
	}
	var out []Bridge
	seen := map[string]bool{}
	for {
		select {
		case <-cctx.Done():
			return out, nil
		case e, ok := <-entries:
			if !ok {
				return out, nil
			}
			if e == nil || len(e.AddrIPv4) == 0 {
				continue
			}
			b := Bridge{IP: e.AddrIPv4[0].String(), Source: "mdns", Instance: e.Instance}
			for _, t := range e.Text {
				kv := strings.SplitN(t, "=", 2)
				if len(kv) != 2 {
					continue
				}
				switch strings.ToLower(kv[0]) {
				case "bridgeid":
					b.ID = kv[1]
				case "modelid":
					b.Model = kv[1]
				}
			}
			if !seen[b.IP] {
				seen[b.IP] = true
				out = append(out, b)
				log("  mDNS: %s (%s, %s)", b.IP, b.ID, b.Model)
			}
		}
	}
}

var (
	reLocation = regexp.MustCompile(`(?im)^LOCATION:\s*http://([0-9.]+)(?::\d+)?/`)
	reServer   = regexp.MustCompile(`(?im)^SERVER:\s*(.*)$`)
	reBridgeID = regexp.MustCompile(`(?i)hue-bridgeid:\s*([0-9A-Fa-f]+)`)
)

// parseSSDPResponse extracts the bridge IP when the responder is a Hue bridge.
func parseSSDPResponse(resp string) (Bridge, bool) {
	srv := reServer.FindStringSubmatch(resp)
	if srv == nil || !strings.Contains(strings.ToLower(srv[1]), "ipbridge") {
		return Bridge{}, false
	}
	loc := reLocation.FindStringSubmatch(resp)
	if loc == nil {
		return Bridge{}, false
	}
	b := Bridge{IP: loc[1], Source: "ssdp"}
	if id := reBridgeID.FindStringSubmatch(resp); id != nil {
		b.ID = id[1]
	}
	return b, true
}

// discoverSSDP sends an M-SEARCH and collects Hue answers for timeout.
func discoverSSDP(timeout time.Duration, log func(string, ...any)) ([]Bridge, error) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	dst := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}
	msg := "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: ssdp:all\r\n\r\n"
	for i := 0; i < 2; i++ {
		if _, err := conn.WriteTo([]byte(msg), dst); err != nil {
			return nil, err
		}
	}
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 4096)
	var out []Bridge
	seen := map[string]bool{}
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			return out, nil // deadline
		}
		if b, ok := parseSSDPResponse(string(buf[:n])); ok && !seen[b.IP] {
			seen[b.IP] = true
			out = append(out, b)
			log("  SSDP: %s (%s)", b.IP, b.ID)
		}
	}
}
