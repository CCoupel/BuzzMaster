package server

import (
	"buzzcontrol/internal/game"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"
)

// #267 — détection réseau du serveur (contracts/game-state.md §NETWORK) : DetectNetwork est pure,
// donc testée sur des descriptions d'interfaces, sans appel système ni attente.

func ips(ss ...string) []net.IP {
	out := make([]net.IP, 0, len(ss))
	for _, s := range ss {
		out = append(out, net.ParseIP(s))
	}
	return out
}

func iface(name string, up bool, addrs ...string) NetInterface {
	return NetInterface{Name: name, Up: up, Addrs: ips(addrs...)}
}

func TestDetectNetwork267_Table(t *testing.T) {
	now := time.UnixMilli(1760345000123)
	loop := NetInterface{Name: "lo", Up: true, Loopback: true, Addrs: ips("127.0.0.1", "::1")}

	cases := []struct {
		name        string
		in          []NetInterface
		wantOnly    bool
		wantIPs     []string
		wantIgnored []game.IgnoredInterface
	}{
		{"réelle seule", []NetInterface{iface("Ethernet", true, "192.168.1.84")}, false, []string{"192.168.1.84"}, nil},
		{"aucune interface", nil, true, nil, nil},
		{"loopback seule", []NetInterface{loop}, true, nil, nil},
		{"APIPA seule (PC sans DHCP)", []NetInterface{iface("Ethernet", true, "169.254.10.20")}, true, nil, nil},
		{"virtuelle seule (WSL)", []NetInterface{iface("vEthernet (WSL)", true, "172.20.0.1")},
			true, nil, []game.IgnoredInterface{{Name: "vEthernet (WSL)", IP: "172.20.0.1"}}},
		{"mixte réelle + virtuelle", []NetInterface{iface("vEthernet (WSL)", true, "172.20.0.1"), iface("Wi-Fi", true, "192.168.1.84")},
			false, []string{"192.168.1.84"}, []game.IgnoredInterface{{Name: "vEthernet (WSL)", IP: "172.20.0.1"}}},
		{"nom virtuel sur IP routable = ignorée et listée", []NetInterface{iface("Hyper-V Switch", true, "10.0.0.5")},
			true, nil, []game.IgnoredInterface{{Name: "Hyper-V Switch", IP: "10.0.0.5"}}},
		{"interface down ignorée sans être listée", []NetInterface{iface("Ethernet", false, "192.168.1.84")}, true, nil, nil},
		{"IPv6 seule", []NetInterface{iface("Ethernet", true, "fe80::1", "2001:db8::1")}, true, nil, nil},
		{"ordre de net.Interfaces() conservé, plusieurs adresses",
			[]NetInterface{iface("eth0", true, "192.168.1.10", "10.1.1.1"), iface("wlan0", true, "192.168.4.1")},
			false, []string{"192.168.1.10", "10.1.1.1", "192.168.4.1"}, nil},
		{"Linux : docker0 et veth ignorées, wlan0 retenue (point d'accès Raspberry)",
			[]NetInterface{iface("docker0", true, "172.17.0.1"), iface("veth12ab", true, "172.18.0.1"), iface("wlan0", true, "192.168.4.1")},
			false, []string{"192.168.4.1"},
			[]game.IgnoredInterface{{Name: "docker0", IP: "172.17.0.1"}, {Name: "veth12ab", IP: "172.18.0.1"}}},
		{"adresse 127.x sur une interface non marquée loopback", []NetInterface{iface("Ethernet", true, "127.0.1.1")}, true, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectNetwork(tc.in, now)
			if got.OnlyLocalhost != tc.wantOnly {
				t.Errorf("OnlyLocalhost = %v, attendu %v", got.OnlyLocalhost, tc.wantOnly)
			}
			wantIPs := tc.wantIPs
			if wantIPs == nil {
				wantIPs = []string{}
			}
			if !reflect.DeepEqual(got.IPs, wantIPs) {
				t.Errorf("IPs = %#v, attendu %#v", got.IPs, wantIPs)
			}
			wantIgn := tc.wantIgnored
			if wantIgn == nil {
				wantIgn = []game.IgnoredInterface{}
			}
			if !reflect.DeepEqual(got.IgnoredInterfaces, wantIgn) {
				t.Errorf("IgnoredInterfaces = %#v, attendu %#v", got.IgnoredInterfaces, wantIgn)
			}
			if got.CheckedAt != 1760345000123 {
				t.Errorf("CheckedAt = %d (epoch ms attendu)", got.CheckedAt)
			}
			if got.IPs == nil || got.IgnoredInterfaces == nil {
				t.Error("les listes ne sont jamais nil")
			}
		})
	}
}

func TestIsVirtualInterfaceName267(t *testing.T) {
	virtual := []string{
		// Windows (sous-chaîne, insensible à la casse)
		"vEthernet (WSL)", "vEthernet (Default Switch)", "VMware Network Adapter VMnet8", "VirtualBox Host-Only Network",
		"Hyper-V Virtual Ethernet Adapter", "Loopback Pseudo-Interface 1", "WSL", "VIRTUAL",
		// Linux / macOS (préfixe)
		"docker0", "br-1a2b3c4d5e6f", "veth9f8e7d", "virbr0", "tun0", "tap0", "wg0", "lxcbr0", "vmnet1", "utun3", "DOCKER0",
	}
	for _, n := range virtual {
		if !IsVirtualInterfaceName(n) {
			t.Errorf("%q devrait être une interface virtuelle", n)
		}
	}
	real := []string{"Ethernet", "Ethernet 2", "Wi-Fi", "eth0", "end0", "enp3s0", "wlan0", "wlp2s0", "en0", "bond0"}
	for _, n := range real {
		if IsVirtualInterfaceName(n) {
			t.Errorf("%q ne doit PAS être considérée virtuelle", n)
		}
	}
	// « br- » est un préfixe : un pont réel (Raspberry en point d'accès via bridge) serait ignoré à tort.
	// hostapd utilise wlan0 (retenue) — voir la procédure manuelle (#267, R2).
	if !IsVirtualInterfaceName("br-lan") {
		t.Error("br-lan : préfixe br- ignoré (limite documentée)")
	}
}

func TestClassifyNetworkChange267(t *testing.T) {
	st := func(only bool, ips ...string) game.NetworkState {
		if ips == nil {
			ips = []string{}
		}
		return game.NetworkState{OnlyLocalhost: only, IPs: ips}
	}
	cases := []struct {
		name       string
		prev, next game.NetworkState
		want       NetworkChange
	}{
		{"stable avec réseau", st(false, "192.168.1.84"), st(false, "192.168.1.84"), NetworkChangeNone},
		{"stable sans réseau", st(true), st(true), NetworkChangeNone},
		{"perte", st(false, "192.168.1.84"), st(true), NetworkChangeLost},
		{"retour", st(true), st(false, "192.168.1.84"), NetworkChangeRestored},
		{"changement d'adresse", st(false, "192.168.1.84"), st(false, "10.0.0.7"), NetworkChangeIPsChanged},
		{"adresse ajoutée", st(false, "192.168.1.84"), st(false, "192.168.1.84", "10.0.0.7"), NetworkChangeIPsChanged},
	}
	for _, tc := range cases {
		if got := ClassifyNetworkChange(tc.prev, tc.next); got != tc.want {
			t.Errorf("%s : %q, attendu %q", tc.name, got, tc.want)
		}
	}
	// IGNORED_INTERFACES et CHECKED_AT seuls ne sont jamais un changement (ni diffusion ni journal).
	a := game.NetworkState{IPs: []string{"192.168.1.84"}, CheckedAt: 1}
	b := game.NetworkState{IPs: []string{"192.168.1.84"}, CheckedAt: 99999, IgnoredInterfaces: []game.IgnoredInterface{{Name: "docker0", IP: "172.17.0.1"}}}
	if got := ClassifyNetworkChange(a, b); got != NetworkChangeNone {
		t.Errorf("IGNORED/CHECKED_AT seuls : %q, attendu aucun changement", got)
	}
}

func TestDetectNetworkNow267_InjectedSource(t *testing.T) {
	restore := SetListInterfaces(func() ([]NetInterface, error) {
		return []NetInterface{iface("Ethernet", true, "192.168.1.84")}, nil
	})
	defer restore()
	if got := DetectNetworkNow(); got.OnlyLocalhost || len(got.IPs) != 1 || got.IPs[0] != "192.168.1.84" || got.CheckedAt == 0 {
		t.Errorf("source injectée non utilisée : %+v", got)
	}

	restoreErr := SetListInterfaces(func() ([]NetInterface, error) { return nil, errors.New("boom") })
	if got := DetectNetworkNow(); !got.OnlyLocalhost || len(got.IPs) != 0 || got.IPs == nil {
		t.Errorf("erreur de listage = sans réseau, listes vides : %+v", got)
	}
	restoreErr()
	if got := DetectNetworkNow(); len(got.IPs) != 1 {
		t.Errorf("restore doit remettre la source précédente : %+v", got)
	}
}

func TestFormatIgnoredInterfaces267(t *testing.T) {
	if got := FormatIgnoredInterfaces(nil); got != "" {
		t.Errorf("liste vide = chaîne vide, obtenu %q", got)
	}
	got := FormatIgnoredInterfaces([]game.IgnoredInterface{{Name: "docker0", IP: "172.17.0.1"}, {Name: "vEthernet (WSL)", IP: "172.20.0.1"}})
	if got != "docker0:172.17.0.1,vEthernet (WSL):172.20.0.1" {
		t.Errorf("format = %q", got)
	}
}

func TestTryDetectNetwork267_ListingErrorIsReportedNotConfusedWithNoNetwork(t *testing.T) {
	restore := SetListInterfaces(func() ([]NetInterface, error) { return nil, errors.New("boom") })
	defer restore()
	st, err := TryDetectNetwork()
	if err == nil {
		t.Fatal("l'erreur de listage doit être retournée (« je n'ai pas pu regarder » n'est pas « pas de réseau »)")
	}
	if !reflect.DeepEqual(st, game.NetworkState{}) {
		t.Errorf("état nul attendu avec l'erreur : %+v", st)
	}
}
