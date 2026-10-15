package server

import (
	"buzzcontrol/internal/game"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

// Server network detection (v11.2.2, #267 — contracts/game-state.md §NETWORK).
//
// "No network" = no usable IPv4 on an UP interface once the loopback (127/8), APIPA
// (169.254/16) and the KNOWN VIRTUAL INTERFACES are excluded. The virtual-interface list is
// best-effort, by interface NAME, case-insensitive: it can miss an exotic adapter and is
// documented (docs/DATA_MODELS.md) without any claim of exhaustiveness.
//
// GetServerIPs() (UDP heartbeats, resolveServerIP) is NOT changed: the alert criterion is
// stricter than what the heartbeats broadcast, never the other way round.

// NetInterface is the pure description of one network interface handed to DetectNetwork
// (no system call inside the detection itself).
type NetInterface struct {
	Name     string
	Up       bool
	Loopback bool     // net.FlagLoopback
	Addrs    []net.IP // every address of the interface (IPv4 and IPv6)
}

// VirtualInterfaceSubstrings are matched anywhere in the lower-cased interface name
// (Windows adapter names: "vEthernet (WSL)", "VMware Network Adapter VMnet1", …).
var VirtualInterfaceSubstrings = []string{
	"vethernet", "virtual", "wsl", "hyper-v", "vmware", "virtualbox", "loopback",
}

// VirtualInterfacePrefixes are matched at the start of the lower-cased interface name
// (Linux / macOS: docker0, br-1a2b…, veth…, virbr0, tun0, tap0, wg0, lxcbr0, vmnet1, utun3).
var VirtualInterfacePrefixes = []string{
	"docker", "br-", "veth", "virbr", "tun", "tap", "wg", "lxc", "vmnet", "utun",
}

// IsVirtualInterfaceName reports whether an interface name belongs to the known virtual list.
func IsVirtualInterfaceName(name string) bool {
	n := strings.ToLower(name)
	for _, s := range VirtualInterfaceSubstrings {
		if strings.Contains(n, s) {
			return true
		}
	}
	for _, p := range VirtualInterfacePrefixes {
		if strings.HasPrefix(n, p) {
			return true
		}
	}
	return false
}

// DetectNetwork computes the server network status from a description of its interfaces.
// Pure: same input, same output (now only stamps CheckedAt, epoch milliseconds).
//
//   - interface down or flagged loopback → skipped entirely;
//   - IPv4 only; 127/8 and 169.254/16 are skipped;
//   - interface with a known virtual name → each of its IPv4 goes to IgnoredInterfaces, never to IPs;
//   - otherwise → IPs, in the order of the input (net.Interfaces() order);
//   - OnlyLocalhost = no retained IP.
//
// Lists are never nil.
func DetectNetwork(ifaces []NetInterface, now time.Time) game.NetworkState {
	st := game.NetworkState{
		IPs:               []string{},
		IgnoredInterfaces: []game.IgnoredInterface{},
		CheckedAt:         now.UnixMilli(),
	}
	for _, iface := range ifaces {
		if !iface.Up || iface.Loopback {
			continue
		}
		virtual := IsVirtualInterfaceName(iface.Name)
		for _, addr := range iface.Addrs {
			ip4 := addr.To4()
			if ip4 == nil {
				continue
			}
			if ip4[0] == 127 || (ip4[0] == 169 && ip4[1] == 254) {
				continue
			}
			if virtual {
				st.IgnoredInterfaces = append(st.IgnoredInterfaces, game.IgnoredInterface{Name: iface.Name, IP: ip4.String()})
				continue
			}
			st.IPs = append(st.IPs, ip4.String())
		}
	}
	st.OnlyLocalhost = len(st.IPs) == 0
	return st
}

// systemInterfaces lists the real interfaces through the net package.
func systemInterfaces() ([]NetInterface, error) {
	sys, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := make([]NetInterface, 0, len(sys))
	for _, iface := range sys {
		ni := NetInterface{
			Name:     iface.Name,
			Up:       iface.Flags&net.FlagUp != 0,
			Loopback: iface.Flags&net.FlagLoopback != 0,
		}
		if addrs, err := iface.Addrs(); err == nil {
			for _, addr := range addrs {
				switch v := addr.(type) {
				case *net.IPNet:
					ni.Addrs = append(ni.Addrs, v.IP)
				case *net.IPAddr:
					ni.Addrs = append(ni.Addrs, v.IP)
				}
			}
		}
		out = append(out, ni)
	}
	return out, nil
}

// listInterfaces is the injectable interface source (tests replace it via SetListInterfaces).
var listInterfaces atomic.Pointer[func() ([]NetInterface, error)]

func init() {
	f := systemInterfaces
	listInterfaces.Store(&f)
}

// SetListInterfaces replaces the interface source (TEST-ONLY) and returns a function restoring
// the previous one. Passing nil restores the system source.
func SetListInterfaces(f func() ([]NetInterface, error)) (restore func()) {
	if f == nil {
		f = systemInterfaces
	}
	prev := listInterfaces.Swap(&f)
	return func() { listInterfaces.Store(prev) }
}

// netClock is the injectable clock stamping CheckedAt (tests drive it; production uses time.Now).
var netClock atomic.Pointer[func() time.Time]

// SetNetworkClock replaces the clock used to stamp NETWORK.CHECKED_AT (TEST-ONLY) and returns a
// function restoring the previous one. nil restores time.Now.
func SetNetworkClock(f func() time.Time) (restore func()) {
	if f == nil {
		f = time.Now
	}
	prev := netClock.Swap(&f)
	return func() { netClock.Store(prev) }
}

func netNow() time.Time {
	if f := netClock.Load(); f != nil {
		return (*f)()
	}
	return time.Now()
}

// TryDetectNetwork lists the interfaces through the injectable source and runs DetectNetwork.
// A listing ERROR is returned as such and the returned state is the zero value: the caller
// must keep its previous state — "I could not look" is not "there is no network" (#267 review).
func TryDetectNetwork() (game.NetworkState, error) {
	ifaces, err := (*listInterfaces.Load())()
	if err != nil {
		return game.NetworkState{}, err
	}
	return DetectNetwork(ifaces, netNow()), nil
}

// DetectNetworkNow is TryDetectNetwork for callers that only want a state: on a listing error it
// returns "no network" with empty lists. updateNetworkState does NOT use it (it would raise a
// false NETWORK_LOST); it uses TryDetectNetwork.
func DetectNetworkNow() game.NetworkState {
	st, err := TryDetectNetwork()
	if err != nil {
		return DetectNetwork(nil, netNow())
	}
	return st
}

// NetworkChange is the kind of transition between two network states (contracts/logging.md §3.2 nonies).
type NetworkChange string

const (
	NetworkChangeNone       NetworkChange = ""
	NetworkChangeLost       NetworkChange = "NETWORK_LOST"        // OnlyLocalhost false → true
	NetworkChangeRestored   NetworkChange = "NETWORK_RESTORED"    // OnlyLocalhost true → false
	NetworkChangeIPsChanged NetworkChange = "NETWORK_IPS_CHANGED" // same OnlyLocalhost, different IPS
)

// ClassifyNetworkChange compares two states. Only OnlyLocalhost and IPs count: a change of
// IgnoredInterfaces or CheckedAt alone is NOT a change (no broadcast, no log line).
func ClassifyNetworkChange(prev, next game.NetworkState) NetworkChange {
	switch {
	case !prev.OnlyLocalhost && next.OnlyLocalhost:
		return NetworkChangeLost
	case prev.OnlyLocalhost && !next.OnlyLocalhost:
		return NetworkChangeRestored
	case sameStrings(prev.IPs, next.IPs):
		return NetworkChangeNone
	default:
		return NetworkChangeIPsChanged
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// FormatIgnoredInterfaces renders the ignored list as "name:ip,name:ip" for the journal.
func FormatIgnoredInterfaces(list []game.IgnoredInterface) string {
	parts := make([]string, 0, len(list))
	for _, i := range list {
		parts = append(parts, i.Name+":"+i.IP)
	}
	return strings.Join(parts, ",")
}
