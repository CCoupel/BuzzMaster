package server

// Developer-side tests for /api/lighting/* (dev-backend, #207 — contract
// hue-bridge.md §7). test-writer owns http_lighting_config_test.go; every
// identifier here is prefixed dev/Dev.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/lighting/hue"
)

// devHueBridge is a minimal Hue v1 fake: link button state, one light, /config.
type devHueBridge struct {
	mu       sync.Mutex
	pressed  bool
	key      string
	bridgeID string
	srv      *httptest.Server
	hits     []string
}

func newDevHueBridge(t *testing.T) *devHueBridge {
	t.Helper()
	b := &devHueBridge{key: "devkey123", bridgeID: "fffe0000deadbeef"}
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		b.hits = append(b.hits, r.Method+" "+r.URL.Path)
		pressed, key := b.pressed, b.key
		b.mu.Unlock()
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		switch {
		case r.Method == "POST" && r.URL.Path == "/api":
			if !pressed {
				_, _ = w.Write([]byte(`[{"error":{"type":101,"address":"","description":"link button not pressed"}}]`))
				return
			}
			_, _ = w.Write([]byte(`[{"success":{"username":"` + key + `"}}]`))
		case len(parts) >= 2 && parts[0] == "api" && parts[1] != key:
			_, _ = w.Write([]byte(`[{"error":{"type":1,"address":"/","description":"unauthorized user"}}]`))
		case len(parts) == 3 && parts[2] == "config":
			_ = json.NewEncoder(w).Encode(map[string]any{"bridgeid": b.bridgeID, "modelid": "BSB002", "name": "dev"})
		case len(parts) == 3 && parts[2] == "lights":
			_ = json.NewEncoder(w).Encode(map[string]any{"8": map[string]any{"name": "BuzzHue1", "type": "Extended color light", "modelid": "LCA001",
				"state": map[string]any{"on": false, "bri": 10, "xy": []float64{0.3, 0.3}, "reachable": true}}})
		case len(parts) == 5 && parts[4] == "state" && r.Method == "PUT":
			_, _ = w.Write([]byte(`[{"success":{"/lights/8/state/on":true}}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(b.srv.Close)
	return b
}

// devProvider hands the handlers a driver (or nil), plus a minimal real
// mode/flash state (#208, contract §10.1) — AUTO/false until set, exactly
// LightingProvider's contract, so /api/lighting/mode and /api/lighting/flash
// can be exercised against a real (if trivial) implementation rather than a
// stub that always accepts or always refuses.
type devProvider struct {
	mu    sync.Mutex
	d     *hue.Driver
	mode  string
	flash bool
}

func (p *devProvider) LightingDriver() *hue.Driver { p.mu.Lock(); defer p.mu.Unlock(); return p.d }

func (p *devProvider) LightingMode() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mode == "" {
		return "AUTO"
	}
	return p.mode
}

func (p *devProvider) SetLightingMode(mode string) error {
	switch mode {
	case "ON", "AUTO", "OFF":
		p.mu.Lock()
		p.mode = mode
		p.mu.Unlock()
		return nil
	default:
		return errDevInvalidLightingMode
	}
}

func (p *devProvider) LightingFlash() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.flash }

func (p *devProvider) SetLightingFlash(on bool) { p.mu.Lock(); p.flash = on; p.mu.Unlock() }

var errDevInvalidLightingMode = errors.New("dev: mode must be ON, AUTO or OFF")

func devDo(t *testing.T, srv *HTTPServer, method, url, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestDevLightingStatusDisabledWithoutDriver(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	code, out := devDo(t, srv, "GET", "/api/lighting/status", "")
	if code != 200 || out["state"] != "disabled" {
		t.Fatalf("status without provider: %d %v", code, out)
	}
	srv.Lighting = &devProvider{}
	code, out = devDo(t, srv, "GET", "/api/lighting/status", "")
	if code != 200 || out["state"] != "disabled" {
		t.Fatalf("status with nil driver: %d %v", code, out)
	}
	if code, _ := devDo(t, srv, "POST", "/api/lighting/status", ""); code != 405 {
		t.Errorf("POST status must be 405, got %d", code)
	}
	code, out = devDo(t, srv, "POST", "/api/lighting/test", `{"name":"BuzzHue1"}`)
	if code != 409 || out["result"] != "refused" {
		t.Fatalf("test without driver must be 409 refused, got %d %v", code, out)
	}
}

func TestDevLightingRegisterFlowStoresKeyServerSide(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	bridge := newDevHueBridge(t)
	updates := 0
	srv.OnConfigUpdate = func() { updates++ }
	prov := &devProvider{}
	srv.Lighting = prov

	// Button not pressed yet: nominal 409, nothing stored.
	code, out := devDo(t, srv, "POST", "/api/lighting/register", `{"bridge_ip":"`+bridge.srv.URL+`"}`)
	if code != 409 || out["result"] != "refused" || out["reason"] != "link_button_not_pressed" {
		t.Fatalf("not pressed: %d %v", code, out)
	}
	if config.Get().Lighting.APIKey != "" || updates != 0 {
		t.Fatal("nothing must be stored before the button is pressed")
	}

	// Pressed: 200, key stored server-side, bridge id read, never echoed.
	bridge.mu.Lock()
	bridge.pressed = true
	bridge.mu.Unlock()
	req := httptest.NewRequest("POST", "/api/lighting/register", strings.NewReader(`{"bridge_ip":"`+bridge.srv.URL+`"}`))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if w.Code != 200 || strings.Contains(w.Body.String(), bridge.key) {
		t.Fatalf("register: %d %s (key must never be echoed)", w.Code, w.Body.String())
	}
	lc := config.Get().Lighting
	if lc.APIKey != bridge.key || lc.BridgeID != bridge.bridgeID || lc.BridgeIP != bridge.srv.URL {
		t.Fatalf("config after register: %+v", lc)
	}
	// Bugfix (QUALIF v10.0.0.8, bug 2): a successful pairing enables the
	// module outright — the deliberate 2026-09-07 change from the ORIGINAL
	// #206/#207 design (register leaves `enabled` untouched, a separate
	// step was expected to flip it — never actually wired anywhere, per
	// docs/SERVER_PARAMETERS.md's own "Enregistrement du bridge" §1-5,
	// which never mentions one). Without this, ambianceIsConfigured()
	// (requires lc.Enabled) never turns true from PERSISTED state, so the
	// association silently fails to survive a restart.
	if !lc.Enabled {
		t.Fatal("a successful pairing must enable lighting outright (QUALIF v10.0.0.8 bug 2)")
	}
	if updates != 1 {
		t.Errorf("OnConfigUpdate must fire once, got %d", updates)
	}
	// La persistance elle-même : ce qu'un VRAI redémarrage relirait depuis
	// disque (config.Load, jamais le singleton en mémoire) doit porter les
	// mêmes valeurs — c'est précisément ce qui manquait à l'utilisateur.
	reloaded, err := config.Load(config.ConfigPath())
	if err != nil {
		t.Fatalf("Load (simulated restart): %v", err)
	}
	if !reloaded.Lighting.Enabled || reloaded.Lighting.APIKey != bridge.key || reloaded.Lighting.BridgeIP != bridge.srv.URL {
		t.Fatalf("l'association ne survit pas à un redémarrage simulé (Load depuis disque) : %+v", reloaded.Lighting)
	}
	// GET /config.json never shows the key, but shows it is configured.
	getW := httptest.NewRecorder()
	srv.mux.ServeHTTP(getW, httptest.NewRequest("GET", "/config.json", nil))
	var cfgOut struct {
		Lighting map[string]any `json:"lighting"`
	}
	_ = json.Unmarshal(getW.Body.Bytes(), &cfgOut)
	if cfgOut.Lighting["api_key"] != "" || cfgOut.Lighting["api_key_configured"] != true {
		t.Errorf("GET /config.json lighting: %v", cfgOut.Lighting)
	}

	// Inventory works with the stored key regardless of h.lightingDriver()
	// (prov.d, never populated by this HTTP-layer-only test — the real
	// App/driver wiring is exercised in cmd/server, not here): handleLightingLights
	// falls back to a throwaway driver when none is live.
	code, out = devDo(t, srv, "GET", "/api/lighting/lights", "")
	if code != 200 {
		t.Fatalf("lights: %d %v", code, out)
	}
	lights, _ := out["lights"].([]any)
	if len(lights) != 1 {
		t.Fatalf("lights: %v", out)
	}

	// Dead bridge → unreachable (never "refused").
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL
	dead.Close()
	code, out = devDo(t, srv, "POST", "/api/lighting/register", `{"bridge_ip":"`+deadURL+`"}`)
	if code != 503 || out["result"] != "unreachable" {
		t.Fatalf("dead bridge: %d %v", code, out)
	}
	if code, _ := devDo(t, srv, "POST", "/api/lighting/register", `{}`); code != 400 {
		t.Errorf("missing bridge_ip must be 400, got %d", code)
	}
}

func TestDevLightingStatusAndTestWithDriver(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	bridge := newDevHueBridge(t)
	d, err := hue.New(hue.Config{BridgeIP: bridge.srv.URL, BridgeID: bridge.bridgeID, APIKey: bridge.key, Lights: []hue.LightSpec{{Name: "BuzzHue1"}},
		FindBridge: func(_ context.Context, _ string, _ time.Duration) (hue.Bridge, bool, error) {
			return hue.Bridge{}, false, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	srv.Lighting = &devProvider{d: d}

	code, out := devDo(t, srv, "POST", "/api/lighting/test", `{"name":"BuzzHue1"}`)
	if code != 200 || out["result"] != "ok" {
		t.Fatalf("test flash: %d %v", code, out)
	}
	code, out = devDo(t, srv, "GET", "/api/lighting/status", "")
	if code != 200 || out["state"] != "ok" || out["lights_ok"] != float64(1) || out["lights_total"] != float64(1) {
		t.Fatalf("status with driver: %d %v", code, out)
	}
	code, out = devDo(t, srv, "POST", "/api/lighting/test", `{"name":"Inconnue"}`)
	if code != 500 || out["result"] != "error" {
		t.Fatalf("flashing an unconfigured light: %d %v", code, out)
	}
	// The fake bridge only ever saw guarded paths.
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	for _, h := range bridge.hits {
		if strings.Contains(h, "groups") || strings.HasPrefix(h, "DELETE") {
			t.Errorf("forbidden request reached the bridge: %s", h)
		}
	}
}

func TestDevLightingDevicetype(t *testing.T) {
	dt := lightingDevicetype()
	if !strings.HasPrefix(dt, "buzzmaster#") || len(dt) > len("buzzmaster#")+19 || strings.ContainsAny(dt[len("buzzmaster#"):], " /:") {
		t.Errorf("devicetype %q", dt)
	}
}

// ---------------------------------------------------------------------------
// #208 (T2.3, contract §10.1) — POST /api/lighting/mode, POST
// /api/lighting/flash.
// ---------------------------------------------------------------------------

// TestDevLightingMode_RoundTripAndValidation exercises the tri-state
// selector end to end through devProvider's real (if minimal)
// implementation: default AUTO, a valid POST changes it and is reflected on
// /status, and an invalid value is refused without changing anything.
func TestDevLightingMode_RoundTripAndValidation(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	d, closeD := devDriverForOverrideTests(t)
	defer closeD()
	srv.Lighting = &devProvider{d: d}

	code, out := devDo(t, srv, "GET", "/api/lighting/status", "")
	if code != 200 || out["mode"] != "AUTO" || out["flash"] != false {
		t.Fatalf("default mode/flash: %d %v", code, out)
	}

	code, out = devDo(t, srv, "POST", "/api/lighting/mode", `{"mode":"OFF"}`)
	if code != 200 || out["result"] != "ok" || out["mode"] != "OFF" {
		t.Fatalf("set OFF: %d %v", code, out)
	}
	code, out = devDo(t, srv, "GET", "/api/lighting/status", "")
	if code != 200 || out["mode"] != "OFF" {
		t.Fatalf("status after OFF: %d %v", code, out)
	}

	// Lower-case input is normalised (a real client sends what its selector
	// widget stores; the endpoint must not be a silent no-op for it).
	code, out = devDo(t, srv, "POST", "/api/lighting/mode", `{"mode":"on"}`)
	if code != 200 || out["mode"] != "ON" {
		t.Fatalf("set on (lower-case): %d %v", code, out)
	}

	code, out = devDo(t, srv, "POST", "/api/lighting/mode", `{"mode":"BOGUS"}`)
	if code != 400 {
		t.Fatalf("invalid mode must be refused, got %d %v", code, out)
	}
	code, out = devDo(t, srv, "GET", "/api/lighting/status", "")
	if out["mode"] != "ON" {
		t.Fatalf("a refused POST must not change the mode, got %v", out)
	}

	if code, _ := devDo(t, srv, "GET", "/api/lighting/mode", ""); code != 405 {
		t.Errorf("GET mode must be 405, got %d", code)
	}
	if code, _ := devDo(t, srv, "POST", "/api/lighting/mode", `not json`); code != 400 {
		t.Errorf("malformed JSON body must be 400, got %d", code)
	}

	// Review fix (code-reviewer, v10 Batch 2, MINEUR 1): a provider IS wired
	// but no driver is actually configured/enabled — must be refused too,
	// not just the "no provider at all" case below.
	srv.Lighting = &devProvider{}
	if code, out := devDo(t, srv, "POST", "/api/lighting/mode", `{"mode":"ON"}`); code != 409 || out["result"] != "refused" {
		t.Fatalf("provider wired but no driver configured: %d %v", code, out)
	}

	srv.Lighting = nil
	if code, out := devDo(t, srv, "POST", "/api/lighting/mode", `{"mode":"ON"}`); code != 409 || out["result"] != "refused" {
		t.Fatalf("no provider wired: %d %v", code, out)
	}
}

// devDriverForOverrideTests builds a real, minimal *hue.Driver against a
// fake bridge (same pattern as TestDevLightingStatusAndTestWithDriver) —
// TestDevLightingMode_RoundTripAndValidation/TestDevLightingFlash_RoundTrip
// need h.lightingDriver() != nil since the MINEUR 1 fix above (409 unless a
// driver is actually configured).
func devDriverForOverrideTests(t *testing.T) (*hue.Driver, func() error) {
	t.Helper()
	bridge := newDevHueBridge(t)
	d, err := hue.New(hue.Config{BridgeIP: bridge.srv.URL, BridgeID: bridge.bridgeID, APIKey: bridge.key, Lights: []hue.LightSpec{{Name: "BuzzHue1"}},
		FindBridge: func(_ context.Context, _ string, _ time.Duration) (hue.Bridge, bool, error) {
			return hue.Bridge{}, false, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	return d, d.Close
}

// TestDevLightingFlash_RoundTrip mirrors the mode test for the separate
// Flash bascule (contract §10.1.2) — engaging it must not move the
// selector's own position.
func TestDevLightingFlash_RoundTrip(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	d, closeD := devDriverForOverrideTests(t)
	defer closeD()
	p := &devProvider{d: d}
	srv.Lighting = p
	if err := p.SetLightingMode("OFF"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	code, out := devDo(t, srv, "POST", "/api/lighting/flash", `{"on":true}`)
	if code != 200 || out["result"] != "ok" || out["flash"] != true {
		t.Fatalf("engage flash: %d %v", code, out)
	}
	code, out = devDo(t, srv, "GET", "/api/lighting/status", "")
	if code != 200 || out["flash"] != true || out["mode"] != "OFF" {
		t.Fatalf("flash engaged must not move the selector: %d %v", code, out)
	}

	code, out = devDo(t, srv, "POST", "/api/lighting/flash", `{"on":false}`)
	if code != 200 || out["flash"] != false {
		t.Fatalf("disengage flash: %d %v", code, out)
	}
	code, out = devDo(t, srv, "GET", "/api/lighting/status", "")
	if out["flash"] != false || out["mode"] != "OFF" {
		t.Fatalf("selector must still read OFF after flash off: %v", out)
	}

	if code, _ := devDo(t, srv, "GET", "/api/lighting/flash", ""); code != 405 {
		t.Errorf("GET flash must be 405, got %d", code)
	}

	// Review fix (code-reviewer, v10 Batch 2, MINEUR 1): a provider IS wired
	// but no driver is actually configured/enabled — must be refused too.
	srv.Lighting = &devProvider{}
	if code, out := devDo(t, srv, "POST", "/api/lighting/flash", `{"on":true}`); code != 409 || out["result"] != "refused" {
		t.Fatalf("provider wired but no driver configured: %d %v", code, out)
	}

	srv.Lighting = nil
	if code, out := devDo(t, srv, "POST", "/api/lighting/flash", `{"on":true}`); code != 409 || out["result"] != "refused" {
		t.Fatalf("no provider wired: %d %v", code, out)
	}
}
