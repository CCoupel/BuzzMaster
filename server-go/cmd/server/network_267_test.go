package main

// #267 (v11.2.2) — état réseau du serveur, de bout en bout côté `cmd/server`.
// Contrats : contracts/game-state.md §`NETWORK`, contracts/logging.md §3.2 nonies.
//
// L'App de test est la vraie (hub WebSocket, dispatcher, journal) ; seule la liste des
// interfaces système est injectée (server.SetListInterfaces, test-only) : aucune dépendance
// à la machine qui exécute le test. Le watchdog de 30 s n'est jamais attendu : on appelle
// directement app.updateNetworkState(), ce que fait un passage du watchdog.
//
// Attentes : jamais de fenêtre fixe (#264/#266). Présence d'un UPDATE = attente par
// événement (readActionMatchingWithin) ; absence = message témoin FIFO (assertNoAction264).
//
// La détection pure (table d'interfaces, liste virtuelle, classification) et la sérialisation
// du moteur sont couvertes par internal/server/netdetect_267_test.go et
// internal/game/network_267_test.go : ce fichier ne les duplique pas.

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

// ── Jeux d'interfaces ────────────────────────────────────────────────────────────────────────

func ni267(name string, ips ...string) server.NetInterface {
	addrs := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		addrs = append(addrs, net.ParseIP(ip))
	}
	return server.NetInterface{Name: name, Up: true, Addrs: addrs}
}

var (
	loopback267 = server.NetInterface{Name: "lo", Up: true, Loopback: true, Addrs: []net.IP{net.ParseIP("127.0.0.1")}}
	wifi267     = ni267("Wi-Fi", "192.168.1.84")
	wsl267      = ni267("vEthernet (WSL)", "172.21.0.1")
	// Réseau normal / panne (seule une interface virtuelle reste) / adresse retenue différente.
	netOK267   = []server.NetInterface{loopback267, wifi267, wsl267}
	netLost267 = []server.NetInterface{loopback267, wsl267}
	netNew267  = []server.NetInterface{loopback267, ni267("Ethernet", "10.0.0.7"), wsl267}
)

// ── Harnais ──────────────────────────────────────────────────────────────────────────────────

type env267 struct {
	app  *App
	base string

	mu     sync.Mutex
	ifaces []server.NetInterface
}

// newEnv267 construit l'App de test, route le logger global vers app.logger (updateNetworkState
// journalise via server.LogWarn/LogInfo) et injecte la liste d'interfaces initiale. Aucun
// updateNetworkState n'est appelé : chaque test choisit son « démarrage ».
func newEnv267(t *testing.T, initial []server.NetInterface) *env267 {
	t.Helper()
	app := newBroadcast127TestApp(t)
	server.SetGlobalLogger(app.logger)
	t.Cleanup(func() { server.SetGlobalLogger(nil) })
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))

	e := &env267{app: app, ifaces: initial}
	restore := server.SetListInterfaces(func() ([]server.NetInterface, error) {
		e.mu.Lock()
		defer e.mu.Unlock()
		return append([]server.NetInterface(nil), e.ifaces...), nil
	})
	t.Cleanup(restore)
	e.base = startAnimAllowlistTestServer(t, app)
	return e
}

// setNet change les interfaces « système » vues par le prochain passage de détection.
func (e *env267) setNet(ifaces []server.NetInterface) {
	e.mu.Lock()
	e.ifaces = ifaces
	e.mu.Unlock()
}

// pass = un passage du watchdog avec les interfaces courantes.
func (e *env267) pass() { e.app.updateNetworkState() }

// dial ouvre une WebSocket du type donné et attend son inscription dans le hub.
func (e *env267) dial(t *testing.T, path string) (*websocket.Conn, string) {
	t.Helper()
	c := dialWS(t, e.base, path)
	return c, learnClientID(t, e.app, c)
}

// drain consomme tout ce qui est déjà en file sur conns (témoin FIFO) pour partir d'une file vide.
func (e *env267) drain(t *testing.T, conns ...*websocket.Conn) {
	t.Helper()
	collectAfterSentinel264(t, e.app, conns...)
}

// logs retourne les messages du journal de l'App pour le niveau donné ("" = tous).
func (e *env267) logs(level game.LogLevel) []string {
	var out []string
	for _, en := range e.app.logger.GetHistory() {
		if level == "" || en.Level == level {
			out = append(out, en.Message)
		}
	}
	return out
}

// networkLines : les entrées du journal qui sont des événements réseau #267.
func (e *env267) networkLines() []string {
	var out []string
	for _, m := range e.logs("") {
		if strings.HasPrefix(m, "NETWORK_") && !strings.HasPrefix(m, "NETWORK_ONLY_LOCALHOST") {
			out = append(out, m)
		}
	}
	return out
}

type network267 struct {
	OnlyLocalhost bool     `json:"ONLY_LOCALHOST"`
	IPs           []string `json:"IPS"`
	Ignored       []struct {
		Name string `json:"NAME"`
		IP   string `json:"IP"`
	} `json:"IGNORED_INTERFACES"`
	CheckedAt int64 `json:"CHECKED_AT"`
}

// parseNetwork267 extrait GAME.NETWORK et le miroir hérité d'un message UPDATE (champ MSG).
// Vérifie au passage que les listes sont des tableaux JSON (jamais null) et que le miroir est exact.
func parseNetwork267(t *testing.T, msg json.RawMessage) network267 {
	t.Helper()
	var u struct {
		Game struct {
			Network json.RawMessage `json:"NETWORK"`
			Mirror  *bool           `json:"NETWORK_ONLY_LOCALHOST"`
		} `json:"GAME"`
	}
	if err := json.Unmarshal(msg, &u); err != nil {
		t.Fatalf("UPDATE illisible : %v (%s)", err, msg)
	}
	if len(u.Game.Network) == 0 {
		t.Fatalf("GAME.NETWORK absent de l'UPDATE : %s", msg)
	}
	raw := string(u.Game.Network)
	if !strings.Contains(raw, `"IPS":[`) || !strings.Contains(raw, `"IGNORED_INTERFACES":[`) {
		t.Errorf("les listes de NETWORK doivent être des tableaux JSON, jamais null : %s", raw)
	}
	var n network267
	if err := json.Unmarshal(u.Game.Network, &n); err != nil {
		t.Fatalf("NETWORK illisible : %v (%s)", err, raw)
	}
	if u.Game.Mirror == nil {
		t.Errorf("le miroir hérité NETWORK_ONLY_LOCALHOST doit rester sérialisé : %s", msg)
	} else if *u.Game.Mirror != n.OnlyLocalhost {
		t.Errorf("NETWORK_ONLY_LOCALHOST=%v doit être le miroir exact de NETWORK.ONLY_LOCALHOST=%v", *u.Game.Mirror, n.OnlyLocalhost)
	}
	return n
}

// parseRestrictedNetwork267 vérifie qu'un UPDATE destiné à un client non admin (#275) ne porte
// PAS GAME.NETWORK (adresses du serveur) mais garde le miroir NETWORK_ONLY_LOCALHOST ; retourne le miroir.
func parseRestrictedNetwork267(t *testing.T, who string, msg json.RawMessage) bool {
	t.Helper()
	var u struct {
		Game map[string]json.RawMessage `json:"GAME"`
	}
	if err := json.Unmarshal(msg, &u); err != nil {
		t.Fatalf("%s : UPDATE illisible : %v", who, err)
	}
	if _, has := u.Game["NETWORK"]; has {
		t.Errorf("%s : GAME.NETWORK ne doit pas être diffusé à un client non admin (#275)", who)
	}
	raw, ok := u.Game["NETWORK_ONLY_LOCALHOST"]
	if !ok {
		t.Fatalf("%s : le miroir NETWORK_ONLY_LOCALHOST doit rester diffusé", who)
	}
	var mirror bool
	if err := json.Unmarshal(raw, &mirror); err != nil {
		t.Fatalf("%s : miroir illisible : %v", who, err)
	}
	return mirror
}

// nextUpdate attend (par événement) le prochain UPDATE de conn et en retourne le NETWORK.
func nextUpdate267(t *testing.T, conn *websocket.Conn) network267 {
	t.Helper()
	return parseNetwork267(t, readActionMatchingWithin(t, conn, protocol.ActionUpdate, eventDeadline264))
}

// ── 1. NETWORK présent au HELLO, 4 champs, tous les types de client web ─────────────────────

func TestNetwork267_Hello_AdminReceivesFullNetworkStateDuringOutage(t *testing.T) {
	e := newEnv267(t, netLost267)
	e.pass() // démarrage sans réseau utilisable
	conn, id := e.dial(t, "/ws/admin")

	e.app.sendStateToClient(id, server.ClientTypeAdmin)

	n := parseNetwork267(t, readActionMatchingWithin(t, conn, protocol.ActionUpdate, eventDeadline264))
	if !n.OnlyLocalhost {
		t.Error("un admin qui se connecte pendant la panne doit recevoir ONLY_LOCALHOST=true au HELLO")
	}
	if len(n.IPs) != 0 {
		t.Errorf("IPS doit être vide pendant la panne, obtenu %v", n.IPs)
	}
	if len(n.Ignored) != 1 || n.Ignored[0].Name != "vEthernet (WSL)" || n.Ignored[0].IP != "172.21.0.1" {
		t.Errorf("IGNORED_INTERFACES doit lister vEthernet (WSL) 172.21.0.1, obtenu %+v", n.Ignored)
	}
	if n.CheckedAt <= 0 {
		t.Errorf("CHECKED_AT doit être l'horodatage (ms) de la dernière détection, obtenu %d", n.CheckedAt)
	}
}

func TestNetwork267_Hello_HealthyNetworkIsSerializedExplicitly(t *testing.T) {
	e := newEnv267(t, netOK267)
	e.pass()
	conn, id := e.dial(t, "/ws/admin")

	e.app.sendStateToClient(id, server.ClientTypeAdmin)

	n := parseNetwork267(t, readActionMatchingWithin(t, conn, protocol.ActionUpdate, eventDeadline264))
	if n.OnlyLocalhost {
		t.Error("réseau présent : ONLY_LOCALHOST doit être false (et sérialisé, jamais omis)")
	}
	if len(n.IPs) != 1 || n.IPs[0] != "192.168.1.84" {
		t.Errorf("IPS doit contenir l'adresse retenue 192.168.1.84, obtenu %v", n.IPs)
	}
	if len(n.Ignored) != 1 || n.Ignored[0].Name != "vEthernet (WSL)" {
		t.Errorf("l'interface virtuelle reste listée dans IGNORED_INTERFACES même avec un réseau réel : %+v", n.Ignored)
	}
}

func TestNetwork267_Hello_NetworkIsDeliveredToAdminOnly(t *testing.T) {
	// Contrat (v11.2.3, #275) : GAME.NETWORK est réservé à l'admin ; TV, anim et VJoueur reçoivent
	// seulement le miroir NETWORK_ONLY_LOCALHOST.
	for _, tc := range []struct {
		path string
		typ  server.ClientType
	}{
		{"/ws/admin", server.ClientTypeAdmin},
		{"/ws/tv", server.ClientTypeTV},
		{"/ws/anim", server.ClientTypeAnim},
		{"/ws/player", server.ClientTypeVPlayer},
	} {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			e := newEnv267(t, netLost267)
			e.pass()
			conn, id := e.dial(t, tc.path)

			e.app.sendStateToClient(id, tc.typ)

			msg := readActionMatchingWithin(t, conn, protocol.ActionUpdate, eventDeadline264)
			if tc.typ == server.ClientTypeAdmin {
				if n := parseNetwork267(t, msg); !n.OnlyLocalhost {
					t.Errorf("%s : NETWORK.ONLY_LOCALHOST=true attendu au HELLO", tc.path)
				}
				return
			}
			if !parseRestrictedNetwork267(t, tc.path, msg) {
				t.Errorf("%s : NETWORK_ONLY_LOCALHOST=true attendu au HELLO", tc.path)
			}
		})
	}
}

// ── 2. Diffusion d'un UPDATE seulement sur changement d'état ───────────────────────────────

func TestNetwork267_Broadcast_SilentWhileNothingChanges(t *testing.T) {
	e := newEnv267(t, netOK267)
	e.pass() // démarrage avec réseau : ni diffusion ni journal
	admin, _ := e.dial(t, "/ws/admin")
	tv, _ := e.dial(t, "/ws/tv")
	e.drain(t, admin, tv)

	checkedAt := e.app.engine.GetNetworkState().CheckedAt
	for i := 0; i < 3; i++ {
		e.pass() // un passage sans changement : ni diffusion ni journal, CHECKED_AT inchangé (instant du dernier changement)
	}
	if got := e.app.engine.GetNetworkState().CheckedAt; got != checkedAt {
		t.Errorf("CHECKED_AT est l'instant du dernier CHANGEMENT d'état, pas du dernier passage : %d → %d", checkedAt, got)
	}

	assertNoAction264(t, e.app, admin, protocol.ActionUpdate, "watchdog sans changement (admin)")
	assertNoAction264(t, e.app, tv, protocol.ActionUpdate, "watchdog sans changement (TV)")
}

func TestNetwork267_Broadcast_OneUpdateOnLossThenSilentWhileLost(t *testing.T) {
	e := newEnv267(t, netOK267)
	e.pass()
	admin, _ := e.dial(t, "/ws/admin")
	tv, _ := e.dial(t, "/ws/tv")
	e.drain(t, admin, tv)

	e.setNet(netLost267)
	e.pass()

	// Un UPDATE par client web, par événement ; il porte le nouvel état.
	if n := nextUpdate267(t, admin); !n.OnlyLocalhost {
		t.Error("l'admin doit recevoir NETWORK.ONLY_LOCALHOST=true à la perte")
	}
	if !parseRestrictedNetwork267(t, "TV", readActionMatchingWithin(t, tv, protocol.ActionUpdate, eventDeadline264)) {
		t.Error("la TV reçoit l'UPDATE avec le miroir seul (#275) : NETWORK_ONLY_LOCALHOST=true attendu")
	}
	// Ni doublon...
	assertNoAction264(t, e.app, admin, protocol.ActionUpdate, "un seul UPDATE par perte (admin)")
	assertNoAction264(t, e.app, tv, protocol.ActionUpdate, "un seul UPDATE par perte (TV)")

	// ... ni rediffusion pendant que la panne dure (même si une interface ignorée change).
	e.pass()
	e.pass()
	e.setNet([]server.NetInterface{loopback267, wsl267, ni267("docker0", "172.17.0.1")})
	e.pass()
	assertNoAction264(t, e.app, admin, protocol.ActionUpdate, "watchdog pendant la panne, y compris interfaces ignorées modifiées")
}

func TestNetwork267_Broadcast_OneUpdateOnRestoreWithTheNewAddress(t *testing.T) {
	e := newEnv267(t, netLost267)
	e.pass() // démarrage en panne
	admin, _ := e.dial(t, "/ws/admin")
	e.drain(t, admin)

	e.setNet(netOK267)
	e.pass()

	n := nextUpdate267(t, admin)
	if n.OnlyLocalhost {
		t.Error("retour du réseau : ONLY_LOCALHOST doit repasser à false")
	}
	if len(n.IPs) != 1 || n.IPs[0] != "192.168.1.84" {
		t.Errorf("le toast « Réseau rétabli — <IP> » lit IPS : attendu [192.168.1.84], obtenu %v", n.IPs)
	}
	assertNoAction264(t, e.app, admin, protocol.ActionUpdate, "un seul UPDATE au retour du réseau")
}

func TestNetwork267_Broadcast_OneUpdateWhenRetainedAddressChangesWithoutOutage(t *testing.T) {
	e := newEnv267(t, netOK267)
	e.pass()
	admin, _ := e.dial(t, "/ws/admin")
	e.drain(t, admin)

	e.setNet(netNew267)
	e.pass()

	n := nextUpdate267(t, admin)
	if n.OnlyLocalhost {
		t.Error("changement d'adresse sans perte : ONLY_LOCALHOST reste false")
	}
	if len(n.IPs) != 1 || n.IPs[0] != "10.0.0.7" {
		t.Errorf("IPS doit refléter la nouvelle adresse 10.0.0.7, obtenu %v", n.IPs)
	}
	assertNoAction264(t, e.app, admin, protocol.ActionUpdate, "un seul UPDATE par changement d'adresse")
}

func TestNetwork267_Broadcast_IgnoredInterfaceChangeAloneIsNotAChange(t *testing.T) {
	e := newEnv267(t, netOK267)
	e.pass()
	admin, _ := e.dial(t, "/ws/admin")
	e.drain(t, admin)

	e.setNet([]server.NetInterface{loopback267, wifi267, wsl267, ni267("docker0", "172.17.0.1")})
	e.pass()
	e.setNet([]server.NetInterface{loopback267, wifi267}) // l'interface virtuelle disparaît
	e.pass()

	assertNoAction264(t, e.app, admin, protocol.ActionUpdate, "seules les interfaces ignorées changent : ni diffusion ni journal")
	if lines := e.networkLines(); len(lines) != 0 {
		t.Errorf("aucun événement réseau attendu, journal : %v", lines)
	}
}

func TestNetwork267_Broadcast_StartupWithUsableNetworkBroadcastsNothing(t *testing.T) {
	e := newEnv267(t, netOK267)
	admin, _ := e.dial(t, "/ws/admin") // connecté AVANT le premier passage (démarrage)
	e.drain(t, admin)

	e.pass()

	assertNoAction264(t, e.app, admin, protocol.ActionUpdate, "démarrage avec réseau utilisable : rien à signaler")
	if st := e.app.engine.GetNetworkState(); st.OnlyLocalhost || len(st.IPs) != 1 || st.CheckedAt <= 0 {
		t.Errorf("l'état doit tout de même être calculé au démarrage : %+v", st)
	}
}

// ── 3. Miroir NETWORK_ONLY_LOCALHOST sur toute la séquence ─────────────────────────────────

func TestNetwork267_LegacyMirrorStaysExactAcrossTheWholeCycle(t *testing.T) {
	e := newEnv267(t, netOK267)
	e.pass()
	admin, _ := e.dial(t, "/ws/admin")
	e.drain(t, admin)

	// nextUpdate267 → parseNetwork267 vérifie à chaque UPDATE : miroir présent et égal à NETWORK.ONLY_LOCALHOST.
	e.setNet(netLost267)
	e.pass()
	if n := nextUpdate267(t, admin); !n.OnlyLocalhost {
		t.Error("perte : true attendu")
	}
	e.setNet(netOK267)
	e.pass()
	if n := nextUpdate267(t, admin); n.OnlyLocalhost {
		t.Error("retour : false attendu")
	}
	e.setNet(netLost267)
	e.pass()
	if n := nextUpdate267(t, admin); !n.OnlyLocalhost {
		t.Error("nouvelle perte : true attendu")
	}
}

// ── 4. Non-persistance ─────────────────────────────────────────────────────────────────────

func TestNetwork267_IsNeverPersistedInGameState(t *testing.T) {
	e := newEnv267(t, netLost267)
	e.pass()
	statePath := filepath.Join(t.TempDir(), "config", "game_state.json")
	e.app.engine.SetStatePath(statePath)

	if err := e.app.engine.SaveState(); err != nil {
		t.Fatalf("SaveState : %v", err)
	}
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToUpper(string(raw)), "NETWORK") || strings.Contains(string(raw), "172.21.0.1") {
		t.Errorf("l'état réseau est recalculé au démarrage : il ne doit JAMAIS être écrit dans game_state.json\n%s", raw)
	}
}

func TestNetwork267_StaleNetworkKeyInStateFileIsIgnoredAtLoad(t *testing.T) {
	e := newEnv267(t, netOK267)
	e.pass()
	statePath := filepath.Join(t.TempDir(), "config", "game_state.json")
	e.app.engine.SetStatePath(statePath)
	if err := e.app.engine.SaveState(); err != nil {
		t.Fatalf("SaveState : %v", err)
	}
	// Fichier d'un autre outil / d'une version bugguée : il porte un NETWORK « panne ».
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc["NETWORK"] = json.RawMessage(`{"ONLY_LOCALHOST":true,"IPS":[],"IGNORED_INTERFACES":[{"NAME":"stale","IP":"1.2.3.4"}],"CHECKED_AT":1}`)
	doc["NETWORK_ONLY_LOCALHOST"] = json.RawMessage(`true`)
	patched, _ := json.Marshal(doc)
	if err := os.WriteFile(statePath, patched, 0o644); err != nil {
		t.Fatal(err)
	}

	fresh := game.NewEngine()
	fresh.SetStatePath(statePath)
	if err := fresh.LoadState(); err != nil {
		t.Fatalf("LoadState : %v", err)
	}
	if st := fresh.GetNetworkState(); st.OnlyLocalhost || st.CheckedAt != 0 || len(st.IgnoredInterfaces) != 0 {
		t.Errorf("un NETWORK relu depuis le disque ne doit jamais être pris en compte (état au démarrage = défaut) : %+v", st)
	}
}

func TestNetwork267_StartupRecomputesTheStateFromTheInterfaces(t *testing.T) {
	// Un redémarrage en panne retrouve l'alerte : l'état vient de la détection, pas du disque.
	e := newEnv267(t, netLost267)
	if st := e.app.engine.GetNetworkState(); st.OnlyLocalhost {
		t.Fatalf("avant la première détection l'état est le défaut (réseau présumé présent) : %+v", st)
	}
	e.pass()
	if st := e.app.engine.GetNetworkState(); !st.OnlyLocalhost || st.CheckedAt <= 0 {
		t.Errorf("la première détection doit calculer l'état : %+v", st)
	}
}

// ── 5. Journal : un événement par transition (contracts/logging.md §3.2 nonies) ─────────────

func TestNetwork267_Log_StartupWithoutNetworkWritesOneNetworkLostWarning(t *testing.T) {
	e := newEnv267(t, netLost267)

	e.pass()

	warns := e.logs(game.LogLevelWarn)
	var lost []string
	for _, m := range warns {
		if strings.HasPrefix(m, "NETWORK_LOST") {
			lost = append(lost, m)
		}
	}
	if len(lost) != 1 {
		t.Fatalf("une seule ligne NETWORK_LOST (WARN) attendue au démarrage sans réseau, obtenu %v (journal : %v)", lost, e.logs(""))
	}
	for _, want := range []string{"ips=[]", "vEthernet (WSL):172.21.0.1", "interval_s=30"} {
		if !strings.Contains(lost[0], want) {
			t.Errorf("NETWORK_LOST doit contenir %q : %s", want, lost[0])
		}
	}
}

func TestNetwork267_Log_StartupWithUsableNetworkWritesNoNetworkEvent(t *testing.T) {
	e := newEnv267(t, netOK267)

	e.pass()

	if lines := e.networkLines(); len(lines) != 0 {
		t.Errorf("démarrage avec réseau : aucun événement NETWORK_* attendu, obtenu %v", lines)
	}
}

func TestNetwork267_Log_WatchdogPassesAreSilentWhileNothingChanges(t *testing.T) {
	e := newEnv267(t, netLost267)
	e.pass() // NETWORK_LOST
	for i := 0; i < 5; i++ {
		e.pass() // 5 passages du watchdog, panne inchangée
	}
	if lines := e.networkLines(); len(lines) != 1 {
		t.Errorf("« un seul événement par transition : pas de ligne à chaque passage du watchdog » — obtenu %d lignes : %v", len(lines), lines)
	}
}

func TestNetwork267_Log_RestoredIsInfoWithIpsAndLostDuration(t *testing.T) {
	e := newEnv267(t, netLost267)
	e.pass()
	e.setNet(netOK267)

	e.pass()

	var restored []string
	for _, m := range e.logs(game.LogLevelInfo) {
		if strings.HasPrefix(m, "NETWORK_RESTORED") {
			restored = append(restored, m)
		}
	}
	if len(restored) != 1 {
		t.Fatalf("une seule ligne NETWORK_RESTORED (INFO) attendue, obtenu %v (journal : %v)", restored, e.logs(""))
	}
	if !strings.Contains(restored[0], "ips=[192.168.1.84]") {
		t.Errorf("NETWORK_RESTORED doit porter ips=[192.168.1.84] : %s", restored[0])
	}
	if !strings.Contains(restored[0], "vEthernet (WSL):172.21.0.1") {
		t.Errorf("NETWORK_RESTORED porte aussi ignored=… : %s", restored[0])
	}
	if !regexp.MustCompile(`lost_for_s=\d+`).MatchString(restored[0]) {
		t.Errorf("NETWORK_RESTORED doit porter lost_for_s=<secondes> : %s", restored[0])
	}
}

func TestNetwork267_Log_IpsChangedIsInfoWithPreviousAddresses(t *testing.T) {
	e := newEnv267(t, netOK267)
	e.pass()
	e.setNet(netNew267)

	e.pass()

	var changed []string
	for _, m := range e.logs(game.LogLevelInfo) {
		if strings.HasPrefix(m, "NETWORK_IPS_CHANGED") {
			changed = append(changed, m)
		}
	}
	if len(changed) != 1 {
		t.Fatalf("une seule ligne NETWORK_IPS_CHANGED (INFO) attendue, obtenu %v (journal : %v)", changed, e.logs(""))
	}
	if !strings.Contains(changed[0], "ips=[10.0.0.7]") || !strings.Contains(changed[0], "prev_ips=[192.168.1.84]") {
		t.Errorf("NETWORK_IPS_CHANGED doit porter ips=[10.0.0.7] et prev_ips=[192.168.1.84] : %s", changed[0])
	}
	for _, m := range e.networkLines() {
		if strings.HasPrefix(m, "NETWORK_LOST") || strings.HasPrefix(m, "NETWORK_RESTORED") {
			t.Errorf("changement d'adresse sans transition de ONLY_LOCALHOST : ni LOST ni RESTORED (%s)", m)
		}
	}
}

func TestNetwork267_Log_FullCycleWritesOneLinePerTransitionInOrder(t *testing.T) {
	e := newEnv267(t, netOK267)
	e.pass()
	e.setNet(netLost267)
	e.pass()
	e.pass()
	e.setNet(netOK267)
	e.pass()
	e.setNet(netLost267)
	e.pass()

	want := []string{"NETWORK_LOST", "NETWORK_RESTORED", "NETWORK_LOST"}
	got := e.networkLines()
	if len(got) != len(want) {
		t.Fatalf("attendu %d événements %v, obtenu %v", len(want), want, got)
	}
	for i, w := range want {
		if !strings.HasPrefix(got[i], w) {
			t.Errorf("événement %d : attendu %s, obtenu %s", i, w, got[i])
		}
	}
}

func TestNetwork267_Log_ReplacesTheLegacyInfoLine(t *testing.T) {
	e := newEnv267(t, netLost267)
	e.pass()
	e.setNet(netOK267)
	e.pass()

	for _, m := range e.logs("") {
		if strings.HasPrefix(m, "Network state updated") {
			t.Errorf("la ligne INFO historique (v5.6.2) est remplacée par les événements #267 : %s", m)
		}
	}
}
