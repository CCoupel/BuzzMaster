package main

// #275 (v11.2.3) — NETWORK (noms et adresses IP des interfaces du serveur) est réservé aux clients admin.
// Contrat : contracts/game-state.md §`NETWORK` ; le miroir hérité NETWORK_ONLY_LOCALHOST reste diffusé à tous.
//
// Réutilise le harnais de network_267_test.go (newEnv267, netLost267, parseNetwork267…) : vraie App,
// vrai hub WebSocket, interfaces système injectées. Attentes par événement, aucune fenêtre fixe (#264/#266).

import (
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

type client275 struct {
	name string
	path string
	typ  server.ClientType
}

var (
	nonAdmin275 = []client275{
		{"TV", "/ws/tv", server.ClientTypeTV},
		{"anim", "/ws/anim", server.ClientTypeAnim},
		{"VJoueur", "/ws/player", server.ClientTypeVPlayer},
	}
)

// assertMirrorOnly275 : le nœud GAME d'un client non-admin n'a ni NETWORK ni IPS/IGNORED_INTERFACES
// (nulle part dans la trame brute), mais garde le miroir NETWORK_ONLY_LOCALHOST exact.
func assertMirrorOnly275(t *testing.T, who, raw string, wantOnlyLocalhost bool) {
	t.Helper()
	game := gameNodeOfRaw(t, raw)
	if _, present := game["NETWORK"]; present {
		t.Errorf("%s : GAME.NETWORK ne doit pas être livré (#275) : %v", who, game["NETWORK"])
	}
	for _, leaked := range []string{`"IPS"`, `"IGNORED_INTERFACES"`, `"CHECKED_AT"`} {
		if strings.Contains(raw, leaked) {
			t.Errorf("%s : %s ne doit apparaître nulle part dans la trame (#275) : %s", who, leaked, raw)
		}
	}
	mirror, present := game["NETWORK_ONLY_LOCALHOST"].(bool)
	if !present {
		t.Errorf("%s : le miroir NETWORK_ONLY_LOCALHOST doit rester diffusé à tous : %v", who, game)
	} else if mirror != wantOnlyLocalhost {
		t.Errorf("%s : NETWORK_ONLY_LOCALHOST=%v, attendu %v", who, mirror, wantOnlyLocalhost)
	}
}

// rawNext275 attend (par événement) la prochaine trame `action` de conn et la rend brute (enveloppe complète).
func rawNext275(t *testing.T, conn *websocket.Conn, action string) string {
	t.Helper()
	_, raw := readActionMatching(t, conn, action)
	return raw
}

// ── 1. HELLO : TV, anim, VJoueur sans NETWORK, miroir exact ─────────────────────────────────

func TestNetworkAdminOnly275_Hello_NonAdminClientsReceiveNoNetworkButTheMirror(t *testing.T) {
	for _, tc := range nonAdmin275 {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv267(t, netLost267)
			e.pass() // démarrage en panne : le miroir vaut true
			conn, id := e.dial(t, tc.path)

			e.app.sendStateToClient(id, tc.typ)

			assertMirrorOnly275(t, tc.name+" (HELLO)", rawNext275(t, conn, protocol.ActionUpdate), true)
		})
	}
}

// ── 2. HELLO admin : NETWORK complet inchangé (4 champs) ────────────────────────────────────

func TestNetworkAdminOnly275_Hello_AdminKeepsTheFullNetworkState(t *testing.T) {
	e := newEnv267(t, netLost267)
	e.pass()
	conn, id := e.dial(t, "/ws/admin")

	e.app.sendStateToClient(id, server.ClientTypeAdmin)

	raw := rawNext275(t, conn, protocol.ActionUpdate)
	n := parseNetwork267(t, mustMsg275(t, raw)) // exige GAME.NETWORK présent + miroir exact
	if !n.OnlyLocalhost {
		t.Error("admin : ONLY_LOCALHOST=true attendu pendant la panne")
	}
	if len(n.IPs) != 0 {
		t.Errorf("admin : IPS doit être vide pendant la panne, obtenu %v", n.IPs)
	}
	if len(n.Ignored) != 1 || n.Ignored[0].Name != "vEthernet (WSL)" || n.Ignored[0].IP != "172.21.0.1" {
		t.Errorf("admin : IGNORED_INTERFACES inchangé attendu (vEthernet (WSL) 172.21.0.1), obtenu %+v", n.Ignored)
	}
	if n.CheckedAt <= 0 {
		t.Errorf("admin : CHECKED_AT doit rester renseigné, obtenu %d", n.CheckedAt)
	}
}

// mustMsg275 extrait le champ MSG d'une trame brute.
func mustMsg275(t *testing.T, raw string) json.RawMessage {
	t.Helper()
	var env struct {
		Msg json.RawMessage `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil || len(env.Msg) == 0 {
		t.Fatalf("trame illisible (%v) : %s", err, raw)
	}
	return env.Msg
}

// ── 3. UPDATE de perte réseau : admin avec NETWORK, les autres sans ─────────────────────────

func TestNetworkAdminOnly275_Broadcast_LossUpdateIsFilteredForNonAdminClients(t *testing.T) {
	e := newEnv267(t, netOK267)
	e.pass()
	admin, _ := e.dial(t, "/ws/admin")
	others := map[string]*websocket.Conn{}
	conns := []*websocket.Conn{admin}
	for _, tc := range nonAdmin275 {
		c, _ := e.dial(t, tc.path)
		others[tc.name] = c
		conns = append(conns, c)
	}
	e.drain(t, conns...)

	e.setNet(netLost267)
	e.pass()

	n := parseNetwork267(t, mustMsg275(t, rawNext275(t, admin, protocol.ActionUpdate)))
	if !n.OnlyLocalhost || len(n.Ignored) != 1 {
		t.Errorf("admin : l'UPDATE de perte doit porter NETWORK complet (ONLY_LOCALHOST=true, 1 interface ignorée), obtenu %+v", n)
	}
	for _, tc := range nonAdmin275 {
		assertMirrorOnly275(t, tc.name+" (UPDATE perte)", rawNext275(t, others[tc.name], protocol.ActionUpdate), true)
	}
}

// ── 4. Actions porteuses d'un GameState (START / STOP, même forme — #128) ───────────────────

func TestNetworkAdminOnly275_Broadcast_StartAndStopCarryNoNetworkForNonAdminClients(t *testing.T) {
	e := newEnv267(t, netLost267)
	e.pass()
	admin, _ := e.dial(t, "/ws/admin")
	others := map[string]*websocket.Conn{}
	for _, tc := range nonAdmin275 {
		c, _ := e.dial(t, tc.path)
		others[tc.name] = c
	}

	for _, s := range []struct {
		label  string
		action string
		call   func()
	}{
		{"START", protocol.ActionStart, func() { e.app.broadcastStart() }},
		{"STOP", protocol.ActionStop, func() { e.app.broadcastStop() }},
	} {
		s := s
		t.Run(s.label, func(t *testing.T) {
			s.call()

			adminGame := gameNodeOfRaw(t, rawNext275(t, admin, s.action))
			if _, present := adminGame["NETWORK"]; !present {
				t.Errorf("admin (%s) : GAME.NETWORK doit rester livré", s.label)
			}
			for _, tc := range nonAdmin275 {
				assertMirrorOnly275(t, tc.name+" ("+s.label+")", rawNext275(t, others[tc.name], s.action), true)
			}
		})
	}
}

// ── 5. Buzzers : toujours sans NETWORK (non-régression) ─────────────────────────────────────

func TestNetworkAdminOnly275_SerializeForBuzzer_StillCarriesNoNetwork(t *testing.T) {
	full := `{"GAME":{"PHASE":"STOPPED","NETWORK":{"ONLY_LOCALHOST":true,"IPS":[],` +
		`"IGNORED_INTERFACES":[{"NAME":"vEthernet (WSL)","IP":"172.21.0.1"}],"CHECKED_AT":1},` +
		`"NETWORK_ONLY_LOCALHOST":true},"bumpers":{},"teams":{}}`
	msg := &protocol.Message{Action: protocol.ActionUpdate, Msg: json.RawMessage(full)}

	data, err := msg.SerializeForBuzzer()
	if err != nil {
		t.Fatalf("SerializeForBuzzer : %v", err)
	}
	for _, leaked := range []string{"NETWORK", "IPS", "IGNORED_INTERFACES", "172.21.0.1"} {
		if strings.Contains(string(data), leaked) {
			t.Errorf("trame buzzer : %q ne doit pas apparaître : %s", leaked, data)
		}
	}
}
