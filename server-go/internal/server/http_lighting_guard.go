package server

// Security guards for /api/lighting/* (security audit 2026-09-04,
// _work/reports/security-issues206-207-20260904-105938.md):
//
//   HAUTE  — SSRF on register: bridge_ip was only checked syntactically and
//            fragments of the target's HTTP body were reflected to the client.
//            Fix: (1) the bridge address must resolve to a private/link-local
//            range BEFORE any outbound call, (2) no target-controlled text
//            ever reaches the HTTP client (logged server-side instead),
//            (3) a key handed back by the target is validated before it is
//            persisted.
//   MOYENNE — no rate limit: one register / discover / test in flight at a
//            time (the admin screen never runs two in parallel).

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// lightingAllowLoopback lets TESTS point the bridge at 127.0.0.1/::1
// (httptest servers). In production a loopback bridge address is refused: the
// server's own ports are the first SSRF target.
var lightingAllowLoopback atomic.Bool

var errBridgeAddrNotPrivate = errors.New("bridge address is not on a private network")

// privateNets are the ranges a Hue Bridge can legitimately live in.
var privateNets = mustCIDRs(
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", // RFC 1918
	"169.254.0.0/16", // IPv4 link-local
	"fe80::/10",      // IPv6 link-local
	"fc00::/7",       // IPv6 unique local
)

func mustCIDRs(cidrs ...string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic(err)
		}
		out = append(out, n)
	}
	return out
}

// isPrivateBridgeIP reports whether ip is in an allowed range.
func isPrivateBridgeIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return lightingAllowLoopback.Load()
	}
	if ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	for _, n := range privateNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// validateBridgeAddress accepts "host", "host:port" or "scheme://host[:port]"
// and returns it normalised, or an error when the host is not a private
// address. A hostname is resolved (2 s) and EVERY resolved address must be
// private. Nothing else about the URL is allowed (no path, query, userinfo).
func validateBridgeAddress(ctx context.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("bridge address is empty")
	}
	if strings.ContainsAny(raw, " \t\r\n\"'<>\\") {
		return "", errBridgeAddrNotPrivate
	}
	withScheme := raw
	if !strings.Contains(raw, "://") {
		withScheme = "http://" + raw
	}
	u, err := url.Parse(withScheme)
	if err != nil || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", errBridgeAddrNotPrivate
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errBridgeAddrNotPrivate
	}
	host := u.Hostname()
	if host == "" {
		return "", errBridgeAddrNotPrivate
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPrivateBridgeIP(ip) {
			return "", errBridgeAddrNotPrivate
		}
	} else {
		rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		ips, err := net.DefaultResolver.LookupIPAddr(rctx, host)
		if err != nil || len(ips) == 0 {
			return "", fmt.Errorf("%w (unresolvable host)", errBridgeAddrNotPrivate)
		}
		for _, a := range ips {
			if !isPrivateBridgeIP(a.IP) {
				return "", errBridgeAddrNotPrivate
			}
		}
	}
	if strings.Contains(raw, "://") {
		return u.Scheme + "://" + u.Host, nil
	}
	return u.Host, nil
}

// validHueAPIKey is the rule already applied on POST /config.json (http.go),
// plus a length bound (real keys are 40 alphanumerics).
func validHueAPIKey(key string) bool {
	if len(key) < 8 || len(key) > 128 {
		return false
	}
	return !strings.ContainsAny(key, "/?#\" \t\r\n")
}

// lightingInFlight serialises the three outbound operations: a second
// concurrent request gets 429 instead of starting another network exchange.
type lightingInFlight struct {
	register, discover, test atomic.Bool
}

var lightingBusy lightingInFlight

// acquire returns false when the operation is already running; the returned
// release must be deferred otherwise.
func (l *lightingInFlight) acquire(flag *atomic.Bool) (release func(), ok bool) {
	if !flag.CompareAndSwap(false, true) {
		return nil, false
	}
	return func() { flag.Store(false) }, true
}
