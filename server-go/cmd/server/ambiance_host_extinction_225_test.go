package main

// Réserve de revue LOT-1 (#225, code-review-lot1-20261111-150000.md) :
// contracts/light-effects.md §2.5 — seuls l'interrupteur général
// (lighting.enabled → false) et l'arrêt du serveur éteignent le groupe
// animateur. Faux pont HTTP, pilote Hue réel (même patron que
// TestDevShutdownExtinguishHueLighting_AppliesOffToEveryConfiguredZone).

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/config"
)

type tw225Bridge struct {
	srv *httptest.Server
	mu  sync.Mutex
	// light id -> corps de chaque PUT /lights/<id>/state reçu
	puts map[string][]map[string]any
}

func newTW225Bridge(t *testing.T) *tw225Bridge {
	t.Helper()
	b := &tw225Bridge{puts: map[string][]map[string]any{}}
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case strings.HasSuffix(p, "/config"):
			_, _ = w.Write([]byte(`{"bridgeid":"fffe0000deadbeef","modelid":"BSB002"}`))
		case strings.HasSuffix(p, "/lights") && r.Method == "GET":
			_, _ = w.Write([]byte(`{"8":{"name":"General1","state":{"on":true,"bri":100,"xy":[0.3,0.3],"reachable":true}},` +
				`"9":{"name":"Team1","state":{"on":true,"bri":100,"xy":[0.3,0.3],"reachable":true}},` +
				`"10":{"name":"Host1","state":{"on":true,"bri":254,"xy":[0.3,0.3],"reachable":true}}}`))
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
		},
	}
	if withHost {
		cfg.Lighting.Groups = []config.LightingGroupEntry{
			{Name: "Pupitre", Role: "animateur", Lights: []string{"Host1"}, NearTeams: []string{}},
		}
	}
	config.SetInstance(&cfg)
}

func TestTW225_ShutdownExtinguish_SwitchesHostGroupOff(t *testing.T) {
	b := newTW225Bridge(t)
	app := newTestApp(t)
	tw225ConfigureHost(t, b, true, true)
	app.setupAmbiance()
	if app.LightingDriver() == nil {
		t.Fatal("pilote attendu depuis une configuration valide")
	}
	app.shutdownExtinguishHueLighting()
	for id, name := range map[string]string{"8": "salle", "9": "équipe", "10": "animateur"} {
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
	if !b.wroteOff("10") || !b.wroteOff("8") {
		t.Fatal("sans groupe animateur, toutes les ampoules générales sont éteintes à l'arrêt")
	}
}

func TestTW225_MasterSwitchOff_SwitchesHostGroupOff(t *testing.T) {
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
	for time.Now().Before(deadline) && !(b.wroteOff("10") && b.wroteOff("8")) {
		time.Sleep(10 * time.Millisecond)
	}
	if !b.wroteOff("8") {
		t.Error("l'interrupteur général doit éteindre les ampoules de salle")
	}
	if !b.wroteOff("10") {
		t.Errorf("l'interrupteur général doit AUSSI éteindre le groupe animateur (contrat §2.5) ; écritures : %v", b.puts["10"])
	}
}
