package hue

// Bridge discovery: mDNS (_hue._tcp.local.) then SSDP (UPnP M-SEARCH, the
// bridge answers with "IpBridge" in its SERVER header). No cloud call —
// taken from spike/hue-bridge/discover.go, validated on a real bridge
// (mDNS answered in 0.2 s).

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
	IP     string `json:"ip"`
	ID     string `json:"id,omitempty"`
	Model  string `json:"model,omitempty"`
	Source string `json:"source"` // mdns | ssdp
}

// Discover runs mDNS then SSDP, each for at most timeout, and merges the
// results by IP (mDNS first). Never returns an error for "nothing found".
func Discover(ctx context.Context, timeout time.Duration) ([]Bridge, error) {
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	var out []Bridge
	seen := map[string]bool{}
	add := func(bs []Bridge) {
		for _, b := range bs {
			if !seen[b.IP] {
				seen[b.IP] = true
				out = append(out, b)
			}
		}
	}
	m, merr := discoverMDNS(ctx, timeout)
	add(m)
	s, serr := discoverSSDP(timeout)
	add(s)
	if len(out) == 0 && merr != nil && serr != nil {
		return nil, fmt.Errorf("hue discovery: mdns: %v; ssdp: %v", merr, serr)
	}
	return out, nil
}

// FindByID discovers and returns the bridge whose id matches (case-insensitive).
func FindByID(ctx context.Context, id string, timeout time.Duration) (Bridge, bool, error) {
	bridges, err := Discover(ctx, timeout)
	if err != nil {
		return Bridge{}, false, err
	}
	for _, b := range bridges {
		if strings.EqualFold(b.ID, id) {
			return b, true, nil
		}
	}
	return Bridge{}, false, nil
}

func discoverMDNS(ctx context.Context, timeout time.Duration) ([]Bridge, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}
	entries := make(chan *zeroconf.ServiceEntry, 8)
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := resolver.Browse(cctx, "_hue._tcp", "local.", entries); err != nil {
		return nil, err
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
			b := Bridge{IP: e.AddrIPv4[0].String(), Source: "mdns"}
			for _, t := range e.Text {
				kv := strings.SplitN(t, "=", 2)
				if len(kv) != 2 {
					continue
				}
				switch strings.ToLower(kv[0]) {
				case "bridgeid":
					b.ID = strings.ToLower(kv[1])
				case "modelid":
					b.Model = kv[1]
				}
			}
			if !seen[b.IP] {
				seen[b.IP] = true
				out = append(out, b)
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
		b.ID = strings.ToLower(id[1])
	}
	return b, true
}

func discoverSSDP(timeout time.Duration) ([]Bridge, error) {
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
			return out, nil // deadline reached
		}
		if b, ok := parseSSDPResponse(string(buf[:n])); ok && !seen[b.IP] {
			seen[b.IP] = true
			out = append(out, b)
		}
	}
}
