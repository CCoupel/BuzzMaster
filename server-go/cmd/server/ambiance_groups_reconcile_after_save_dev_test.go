package main

// End-to-end regression test for QUALIF round 7's "Bug A" investigation
// (_work/handoff/task-dev-backend-investigate-2bugs-20260908.md): "décocher
// une ampoule sur /admin/ambiance ne la sort pas du groupe". Investigated
// and found NOT to reproduce at the backend level — this test is the
// concrete proof, kept as a permanent regression guard for the exact
// mechanism that closes it, since nothing else in the suite exercised the
// FULL chain end to end (POST /config.json's real handler → OnConfigUpdate
// → reconfigureAmbiance → a genuinely fresh *hue.Driver → the writer's own
// NotifyState()-triggered Apply, with NO explicit forced RefreshInventory).
//
// Mechanism confirmed: reconfigureAmbiance() (cmd/server/ambiance.go) calls
// buildHueDriver() UNCONDITIONALLY on every invocation, which always builds
// a brand new *hue.Driver (hue.New()) with a zero-value lastInventory. Its
// very first Apply() therefore always finds ensureResolved's freshness gate
// false (internal/lighting/hue/driver.go: "!d.lastInventory.IsZero()" is
// false on a fresh driver, regardless of the 5-minute RefreshEvery clock),
// so resolve()+reconcileGroups() run — satisfying contract hue-bridge.md
// §5.8 rule 2 ("réconciliation... après chaque enregistrement de
// configuration") without any extra wiring. StateOK on the driver's Status()
// is sufficient proof this ran, since ok() is only reached after a
// successful Apply, which itself only runs after ensureResolved.
//
// This is therefore very likely a FRONTEND-side or physical-bulb-state
// question, not a backend defect — see the investigation's own DONE report
// (_work/reports/dev-backend-investigate-2bugs-20260908.md) for the two
// most likely explanations handed back to the CDP.

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
	"buzzcontrol/internal/lighting/hue"
)

// devGroupBridge2 is a minimal Hue v1 fake with /groups support (GET/POST,
// PUT/DELETE one, PUT its action) — named -2 to avoid colliding with the
// unrelated devBridge/devGroupBridge helpers of other _test.go files in
// this package (each test file in cmd/server keeps its own fixtures, see
// ambiance_dev_test.go's own header comment on that convention).
type devGroupBridge2 struct {
	mu        sync.Mutex
	lights    map[string]map[string]any
	groups    map[string]map[string]any // id -> {name, lights}
	nextGroup int
}

func newDevGroupBridge2(names ...string) *devGroupBridge2 {
	b := &devGroupBridge2{lights: map[string]map[string]any{}, groups: map[string]map[string]any{}}
	for i, n := range names {
		b.lights[fmt.Sprint(i+1)] = map[string]any{"name": n, "type": "Extended color light", "modelid": "LCA001",
			"state": map[string]any{"on": false, "bri": 1, "xy": []float64{0.3, 0.3}, "reachable": true}}
	}
	return b
}

func (b *devGroupBridge2) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := ""
		if r.Body != nil {
			buf := make([]byte, 4096)
			n, _ := r.Body.Read(buf)
			body = string(buf[:n])
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			_, _ = w.Write([]byte(`{"bridgeid":"fffe0000deadbeef","modelid":"BSB002"}`))
		case len(parts) == 3 && parts[2] == "lights" && r.Method == "GET":
			b.mu.Lock()
			defer b.mu.Unlock()
			_ = json.NewEncoder(w).Encode(b.lights)
		case len(parts) == 5 && parts[2] == "lights" && parts[4] == "state" && r.Method == "PUT":
			_, _ = w.Write([]byte(`[{"success":{"/lights/` + parts[3] + `/state/on":true}}]`))
		case len(parts) == 3 && parts[2] == "groups" && r.Method == "GET":
			b.mu.Lock()
			defer b.mu.Unlock()
			_ = json.NewEncoder(w).Encode(b.groups)
		case len(parts) == 3 && parts[2] == "groups" && r.Method == "POST":
			var req struct {
				Name   string   `json:"name"`
				Lights []string `json:"lights"`
			}
			_ = json.Unmarshal([]byte(body), &req)
			b.mu.Lock()
			b.nextGroup++
			id := fmt.Sprint(b.nextGroup)
			b.groups[id] = map[string]any{"name": req.Name, "type": "LightGroup", "lights": req.Lights}
			b.mu.Unlock()
			_, _ = w.Write([]byte(`[{"success":{"id":"` + id + `"}}]`))
		case len(parts) == 4 && parts[2] == "groups" && r.Method == "PUT":
			var req struct {
				Lights []string `json:"lights"`
			}
			_ = json.Unmarshal([]byte(body), &req)
			b.mu.Lock()
			if g, ok := b.groups[parts[3]]; ok {
				g["lights"] = req.Lights
			}
			b.mu.Unlock()
			_, _ = w.Write([]byte(`[{"success":true}]`))
		case len(parts) == 4 && parts[2] == "groups" && r.Method == "DELETE":
			b.mu.Lock()
			delete(b.groups, parts[3])
			b.mu.Unlock()
			_, _ = w.Write([]byte(`[{"success":true}]`))
		case len(parts) == 5 && parts[2] == "groups" && parts[4] == "action" && r.Method == "PUT":
			b.mu.Lock()
			g, ok := b.groups[parts[3]]
			b.mu.Unlock()
			if !ok {
				_, _ = w.Write([]byte(`[{"error":{"type":3,"address":"/groups/` + parts[3] + `/action","description":"resource not available"}}]`))
				return
			}
			var st map[string]any
			_ = json.Unmarshal([]byte(body), &st)
			ls, _ := g["lights"].([]string)
			b.mu.Lock()
			for _, lid := range ls {
				if l, ok := b.lights[lid]; ok {
					state := l["state"].(map[string]any)
					for k, v := range st {
						if k != "transitiontime" {
							state[k] = v
						}
					}
				}
			}
			b.mu.Unlock()
			_, _ = w.Write([]byte(`[{"success":{"/groups/` + parts[3] + `/action/on":true}}]`))
		default:
			w.WriteHeader(404)
		}
	}
}

func (b *devGroupBridge2) groupLights(name string) ([]string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, g := range b.groups {
		if g["name"] == name {
			ls, _ := g["lights"].([]string)
			return append([]string(nil), ls...), true
		}
	}
	return nil, false
}

func TestDevGroupsReconcileAfterConfigSave_RemovingALightCorrectsTheBridgeGroup(t *testing.T) {
	bridge := newDevGroupBridge2("L1", "L2")
	srv := httptest.NewServer(bridge.handler())
	defer srv.Close()

	app := newTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.ctx = ctx
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })

	waitOK := func(wantLightsOK int) hue.Status {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if st := app.LightingDriver().Status(); st.State == hue.StateOK && st.LightsOK == wantLightsOK {
				return st
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("driver never reached StateOK with %d light(s) up: %+v", wantLightsOK, app.LightingDriver().Status())
		return hue.Status{}
	}

	cfg := saved
	cfg.Lighting = config.LightingConfig{Enabled: true, BridgeIP: srv.URL, BridgeID: "fffe0000deadbeef", APIKey: "k",
		Lights: []config.LightingLightEntry{{Name: "L1", Role: "general"}, {Name: "L2", Role: "general"}}}
	config.SetInstance(&cfg)
	app.reconfigureAmbiance()
	waitOK(2)

	// NO explicit RefreshInventory anywhere in this test, on purpose:
	// StateOK is itself the proof that a real Apply() — which internally
	// calls ensureResolved(), hence resolve()+reconcileGroups() — ran via
	// the writer's OWN natural NotifyState()-triggered cycle, exactly what
	// a real POST /config.json triggers. Forcing RefreshInventory would
	// test a different, easier path than the one the bug report describes.
	if lights, ok := bridge.groupLights("buzzmaster-general"); !ok || len(lights) != 2 {
		t.Fatalf("setup invalide : buzzmaster-general doit avoir 2 membres après la 1re réconciliation, got ok=%v %v", ok, lights)
	}

	// Uncheck L2 (remove it from lighting.lights[]) and save — exactly the
	// admin gesture on /admin/ambiance, through the same reconfigureAmbiance
	// entry point POST /config.json's real handler calls via OnConfigUpdate.
	cfg2 := cfg
	cfg2.Lighting.Lights = []config.LightingLightEntry{{Name: "L1", Role: "general"}}
	config.SetInstance(&cfg2)
	app.reconfigureAmbiance()
	waitOK(1)

	lights, ok := bridge.groupLights("buzzmaster-general")
	if !ok {
		t.Fatal("buzzmaster-general a disparu du pont — inattendu, L1 seule doit encore justifier le groupe general")
	}
	if len(lights) != 1 || lights[0] != "1" {
		t.Errorf("buzzmaster-general doit être corrigé à [1] (L1 seule) après le retrait de L2 de la config, got %v — régression du mécanisme de réconciliation (contract hue-bridge.md §5.8 règle 2)", lights)
	}
}
