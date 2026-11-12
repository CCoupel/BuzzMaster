package main

// Réserve de revue LOT-1 (#225, code-review-lot1-20261111-150000.md), révision 4 :
// contracts/light-effects.md §2.5 — seuls l'interrupteur général
// (lighting.enabled → false) et l'arrêt du serveur éteignent la zone animateur
// et la zone public ; une zone public sans effet n'est jamais écrite. Faux pont HTTP, pilote Hue réel (même patron que
// TestDevShutdownExtinguishHueLighting_AppliesOffToEveryConfiguredZone).

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"buzzcontrol/internal/config"
)

type tw225Bridge struct {
	srv *httptest.Server
	mu  sync.Mutex
	// light id -> corps de chaque PUT /lights/<id>/state reçu
	puts map[string][]map[string]any
	// failLights : GET /lights répond 500 (lecture de l'état impossible)
	failLights atomic.Bool
}

func newTW225Bridge(t *testing.T) *tw225Bridge {
	t.Helper()
	b := &tw225Bridge{puts: map[string][]map[string]any{}}
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case strings.HasSuffix(p, "/config"):
			_, _ = w.Write([]byte(`{"bridgeid":"fffe0000deadbeef","modelid":"BSB002"}`))
		case strings.HasSuffix(p, "/lights") && r.Method == "GET" && b.failLights.Load():
			w.WriteHeader(500)
		case strings.HasSuffix(p, "/lights") && r.Method == "GET":
			_, _ = w.Write([]byte(`{"8":{"name":"General1","state":{"on":true,"bri":100,"xy":[0.3,0.3],"reachable":true}},` +
				`"9":{"name":"Team1","state":{"on":true,"bri":100,"xy":[0.3,0.3],"reachable":true}},` +
				`"10":{"name":"Host1","state":{"on":true,"bri":254,"xy":[0.3,0.3],"reachable":true}},` +
				`"11":{"name":"Host2","state":{"on":true,"bri":180,"xy":[0.45,0.41],"reachable":true}}}`))
		case strings.HasSuffix(p, "/state") && r.Method == "PUT":
			parts := strings.Split(strings.Trim(p, "/"), "/") // api/<key>/lights/<id>/state
			body, _ := io.ReadAll(r.Body)
			var m map[string]any
			_ = json.Unmarshal(body, &m)
			b.mu.Lock()
			b.puts[parts[3]] = append(b.puts[parts[3]], m)
			b.mu.Unlock()
			_, _ = w.Write([]byte(`[{"success":{"on":false}}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(b.srv.Close)
	return b
}

// wroteOff : une écriture on:false a-t-elle été reçue pour cette ampoule ?
func (b *tw225Bridge) wroteOff(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, m := range b.puts[id] {
		if on, ok := m["on"].(bool); ok && !on {
			return true
		}
	}
	return false
}

// wrote : une écriture (quelconque) a-t-elle été reçue pour cette ampoule ?
func (b *tw225Bridge) wrote(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.puts[id]) > 0
}

func tw225ConfigureHost(t *testing.T, b *tw225Bridge, enabled bool, withHost bool) {
	t.Helper()
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })
	cfg := saved
	cfg.Lighting = config.LightingConfig{
		Enabled: enabled, BridgeIP: b.srv.URL, BridgeID: "fffe0000deadbeef", APIKey: "k",
		Lights: []config.LightingLightEntry{
			{Name: "General1", Role: "general"},
			{Name: "Team1", Role: "team", Team: "TeamA"},
			{Name: "Host1", Role: "general"},
			{Name: "Host2", Role: "general"},
		},
	}
	if withHost {
		cfg.Lighting.Groups = []config.LightingGroupEntry{
			{ID: "ha", Role: "animateur", Lights: []string{"Host1"}},
			{ID: "pb", Role: "public", Lights: []string{"Host2"}},
		}
	}
	config.SetInstance(&cfg)
}

func TestTW225_ShutdownExtinguish_SwitchesAnimateurAndPublicZonesOff(t *testing.T) {
	b := newTW225Bridge(t)
	app := newTestApp(t)
	tw225ConfigureHost(t, b, true, true)
	app.setupAmbiance()
	if app.LightingDriver() == nil {
		t.Fatal("pilote attendu depuis une configuration valide")
	}
	app.shutdownExtinguishHueLighting()
	for id, name := range map[string]string{"8": "salle", "9": "équipe", "10": "zone animateur", "11": "zone public"} {
		if !b.wroteOff(id) {
			t.Errorf("l'arrêt doit écrire on:false sur l'ampoule %s (%s) ; écritures : %v", id, name, b.puts[id])
		}
	}
}

func TestTW225_ShutdownExtinguish_NoHostGroup_HostNamedBulbIsJustGeneral(t *testing.T) {
	// Sans groupe animateur, Host1 est une ampoule générale ordinaire : éteinte par Group 0 (aucune régression).
	b := newTW225Bridge(t)
	app := newTestApp(t)
	tw225ConfigureHost(t, b, true, false)
	app.setupAmbiance()
	app.shutdownExtinguishHueLighting()
	if !b.wroteOff("10") || !b.wroteOff("11") || !b.wroteOff("8") {
		t.Fatal("sans groupe animateur, toutes les ampoules générales sont éteintes à l'arrêt")
	}
}

func TestTW225_MasterSwitchOff_SwitchesAnimateurAndPublicZonesOff(t *testing.T) {
	// Interrupteur général runtime : lighting.enabled passe à false, puis le
	// serveur appelle extinguishOnLightingDisabled (main.go OnLightingEnabledChange).
	b := newTW225Bridge(t)
	app := newTestApp(t)
	tw225ConfigureHost(t, b, true, true)
	app.setupAmbiance()
	if app.LightingDriver() == nil {
		t.Fatal("pilote attendu")
	}
	cfg := *config.Get()
	cfg.Lighting.Enabled = false
	config.SetInstance(&cfg)

	app.extinguishOnLightingDisabled() // l'écriture réseau part en arrière-plan

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !(b.wroteOff("10") && b.wroteOff("11") && b.wroteOff("8")) {
		time.Sleep(10 * time.Millisecond)
	}
	if !b.wroteOff("8") {
		t.Error("l'interrupteur général doit éteindre les ampoules de salle")
	}
	for id, name := range map[string]string{"10": "animateur", "11": "public"} {
		if !b.wroteOff(id) {
			t.Errorf("l'interrupteur général doit AUSSI éteindre la zone %s (contrat §2.5) ; écritures : %v", name, b.puts[id])
		}
	}
}

// lastPut : le dernier corps PUT reçu pour cette ampoule (nil si aucun) et le nombre d'écritures.
func (b *tw225Bridge) lastPut(id string) (map[string]any, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(b.puts[id])
	if n == 0 {
		return nil, 0
	}
	return b.puts[id][n-1], n
}

// tw225PublicTest lance l'essai du groupe 1 de la zone public (pas de 40 ms) et attend, par sondage borné, que la séquence de six
// pas soit finie ET que l'état rendu ait été réécrit : prédicat sur la dernière écriture de l'ampoule publique (« Host2 », id 11).
func tw225PublicTest(t *testing.T, b *tw225Bridge, beforeTest func(), done func(last map[string]any) bool) {
	t.Helper()
	app := newTestApp(t)
	tw225ConfigureHost(t, b, true, true)
	app.setupAmbiance()
	if app.LightingDriver() == nil {
		t.Fatal("pilote attendu")
	}
	// l'état des lampes doit être connu du pilote (inventaire) avant l'essai, comme en production après le démarrage
	if err := app.LightingDriver().RefreshInventory(context.Background()); err != nil {
		t.Fatalf("inventaire : %v", err)
	}
	if beforeTest != nil {
		beforeTest()
	}
	ctl := app.ambCtl()
	ctl.GroupTestStep = 40 * time.Millisecond
	t.Cleanup(ctl.StopGroupTest)
	if err := app.StartGroupTest("public1"); err != nil {
		t.Fatalf("essai du groupe 1 de la zone public : %v", err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if last, n := b.lastPut("11"); n >= 1 && done(last) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	last, n := b.lastPut("11")
	t.Fatalf("zone public : état attendu non rétabli après l'essai (%d écritures, dernière : %v)", n, last)
}

func TestTW225_PublicGroupTest_StateReadOnTheBridge_ThenWrittenBack(t *testing.T) {
	// §2.5 : la zone public sans effet n'est jamais pilotée ; après l'essai, l'état LU sur le pont avant l'essai est réécrit.
	// Le faux pont annonce « Host2 » allumée à bri 180 : c'est cet état qui doit revenir (pas un blanc fixe, pas éteint).
	b := newTW225Bridge(t)
	tw225PublicTest(t, b, nil, func(last map[string]any) bool {
		on, _ := last["on"].(bool)
		bri, _ := last["bri"].(float64)
		return on && bri == 180
	})
}

func TestTW225_PublicGroupTest_StateUnreadable_BulbsSwitchedOff(t *testing.T) {
	// lecture impossible (le pont répond 500 à GET /lights au début de l'essai) ⇒ lampes éteintes à la fin
	b := newTW225Bridge(t)
	tw225PublicTest(t, b, func() { b.failLights.Store(true) }, func(last map[string]any) bool {
		on, ok := last["on"].(bool)
		return ok && !on
	})
}
