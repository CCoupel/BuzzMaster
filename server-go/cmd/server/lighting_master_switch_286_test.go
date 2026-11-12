// #286 (v11.4.0, LOT-1) — interrupteur général de l'éclairage d'ambiance (`lighting.enabled`).
// Contrat : contracts/sound.md §11.4. `lighting.enabled=false` ⇒ aucune scène n'est envoyée aux
// ampoules (pilote détaché) ; `true` ⇒ pilote reconstruit et scène ré-appliquée.
//
// Frontière : config posée par JSON, bascule déclenchée par le rappel public
// httpServer.OnConfigUpdate (celui que POST /config.json appelle), jamais par un symbole du dev.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting/hue"
	"buzzcontrol/internal/server"
)

// tw286Bridge est un pont Hue v1 minimal qui COMPTE les écritures (tout sauf GET).
type tw286Bridge struct {
	mu     sync.Mutex
	writes int
	nextID int
}

func (b *tw286Bridge) writeCount() int { b.mu.Lock(); defer b.mu.Unlock(); return b.writes }

func (b *tw286Bridge) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			buf := make([]byte, 4096)
			_, _ = r.Body.Read(buf)
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		isStateWrite := r.Method != "GET"
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			_, _ = w.Write([]byte(`{"bridgeid":"fffe0000deadbeef","modelid":"BSB002"}`))
		case len(parts) == 3 && parts[2] == "lights" && r.Method == "GET":
			_ = json.NewEncoder(w).Encode(map[string]any{"1": map[string]any{"name": "L1", "type": "Extended color light",
				"modelid": "LCA001", "state": map[string]any{"on": false, "bri": 1, "xy": []float64{0.3, 0.3}, "reachable": true}}})
		case len(parts) == 3 && parts[2] == "groups" && r.Method == "GET":
			_, _ = w.Write([]byte(`{}`))
		case len(parts) == 3 && parts[2] == "groups" && r.Method == "POST":
			b.mu.Lock()
			b.nextID++
			id := b.nextID
			b.mu.Unlock()
			_, _ = w.Write([]byte(fmt.Sprintf(`[{"success":{"id":"%d"}}]`, id)))
		case len(parts) == 5 && parts[4] == "action" && r.Method == "PUT":
			b.mu.Lock()
			b.writes++
			b.mu.Unlock()
			_, _ = w.Write([]byte(`[{"success":{"/groups/1/action/on":true}}]`))
			return
		case len(parts) == 5 && parts[4] == "state" && r.Method == "PUT":
			b.mu.Lock()
			b.writes++
			b.mu.Unlock()
			_, _ = w.Write([]byte(`[{"success":{"/lights/1/state/on":true}}]`))
			return
		case isStateWrite:
			_, _ = w.Write([]byte(`[{"success":true}]`))
		default:
			w.WriteHeader(404)
		}
	}
}

// tw286SetLightingEnabled flips lighting.enabled the way the server does — by publishing a NEW
// config instance (POST /config.json copies the live one and calls SetInstance), never by writing
// into the live object: the ambiance writer and the effect reader read config.Get() from their own
// goroutines, and an in-place write is a data race that production code never has.
func tw286SetLightingEnabled(t *testing.T, enabled bool) {
	t.Helper()
	next := *config.Get()
	next.Lighting.Enabled = enabled
	config.SetInstance(&next)
}

func tw286WaitDriver(t *testing.T, app *App, wantNil bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if (app.LightingDriver() == nil) == wantNil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("pilote Hue : nil=%v attendu, jamais atteint", wantNil)
}

// §11.4 : enabled=false ⇒ le pilote est détaché ET aucune écriture Hue n'est émise ensuite,
// quels que soient les événements de jeu ; enabled=true ⇒ pilote reconstruit.
func TestLightingMasterSwitch286_Disabled_NoHueWrites_ThenReenabled(t *testing.T) {
	bridge := &tw286Bridge{}
	srv := httptest.NewServer(bridge.handler())
	defer srv.Close()

	app := newTestAppWithHub(t) // OnConfigUpdate → broadcastConfigUpdate déréférence wsHub
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	app.logger = server.NewBroadcastLogger(100)
	app.setupCallbacks()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.ctx = ctx
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })

	cfg := saved
	cfg.Lighting = config.LightingConfig{Enabled: true, BridgeIP: srv.URL, BridgeID: "fffe0000deadbeef", APIKey: "k",
		Lights: []config.LightingLightEntry{{Name: "L1", Role: "general"}}}
	config.SetInstance(&cfg)
	app.httpServer.OnConfigUpdate()
	tw286WaitDriver(t, app, false)

	// Interrupteur coupé — exactement ce que POST /config.json {"lighting":{"enabled":false}} déclenche.
	tw286SetLightingEnabled(t, false)
	app.httpServer.OnConfigUpdate()
	tw286WaitDriver(t, app, true)

	// Laisse passer une éventuelle écriture d'extinction asynchrone (permise), puis fige le compteur.
	time.Sleep(300 * time.Millisecond)
	frozen := bridge.writeCount()

	// Événements de jeu qui déclenchent normalement des scènes d'ambiance.
	q := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready(q.ID, q)
	if w := app.ambiance(); w != nil {
		w.NotifyState()
	}
	app.engine.StartImmediate(0)
	defer app.engine.Stop()
	time.Sleep(300 * time.Millisecond)

	if got := bridge.writeCount(); got != frozen {
		t.Fatalf("lighting.enabled=false : aucune écriture Hue ne doit suivre (compteur %d → %d)", frozen, got)
	}

	// Réactivation : le pilote est reconstruit depuis la configuration.
	tw286SetLightingEnabled(t, true)
	app.httpServer.OnConfigUpdate()
	tw286WaitDriver(t, app, false)
}

// §11.4 : désactivé, le pilote n'est pas exposé à /api/lighting/* (LightingDriver() nil ⇒ 409 not_configured côté HTTP).
func TestLightingMasterSwitch286_Disabled_LightingDriverIsNil(t *testing.T) {
	bridge := &tw286Bridge{}
	srv := httptest.NewServer(bridge.handler())
	defer srv.Close()

	app := newTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.ctx = ctx
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })

	cfg := saved
	cfg.Lighting = config.LightingConfig{Enabled: false, BridgeIP: srv.URL, BridgeID: "fffe0000deadbeef", APIKey: "k",
		Lights: []config.LightingLightEntry{{Name: "L1", Role: "general"}}}
	config.SetInstance(&cfg)
	app.reconfigureAmbiance()

	if d := app.LightingDriver(); d != nil {
		t.Fatalf("enabled=false avec pont et ampoules configurés : aucun pilote ne doit être construit, got %v", d.Status().State == hue.StateOK)
	}
	time.Sleep(200 * time.Millisecond)
	if n := bridge.writeCount(); n != 0 {
		t.Errorf("enabled=false : aucune écriture Hue à la reconfiguration, got %d", n)
	}
}
