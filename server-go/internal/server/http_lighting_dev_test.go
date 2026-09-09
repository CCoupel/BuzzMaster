package server

// Developer-side tests for /api/lighting/* (dev-backend, #207 — contract
// hue-bridge.md §7). test-writer owns http_lighting_config_test.go; every
// identifier here is prefixed dev/Dev.

import (
	"context"
	"encoding/json"
	"errors"
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

// devHueBridge is a minimal Hue v1 fake: link button state, one light,
// /config, and (Batch B, §5.8) a minimal /groups so the driver's own
// reconciliation — now run from every ensureResolved, including the one
// TestFlash performs — has somewhere real to land instead of a bare 404.
type devHueBridge struct {
	mu        sync.Mutex
	pressed   bool
	key       string
	bridgeID  string
	srv       *httptest.Server
	hits      []string
	groups    map[string]*devHueGroup
	nextGroup int
}

type devHueGroup struct {
	name   string
	lights []string
}

func newDevHueBridge(t *testing.T) *devHueBridge {
	t.Helper()
	return newDevHueBridgeNamed(t, "BuzzHue1")
}

// newDevHueBridgeNamed is newDevHueBridge with a caller-chosen light name —
// used to reproduce the QUALIF round-2 report with a name containing
// whitespace exactly as a real bridge might return it.
func newDevHueBridgeNamed(t *testing.T, lightName string) *devHueBridge {
	t.Helper()
	b := &devHueBridge{key: "devkey123", bridgeID: "fffe0000deadbeef", groups: map[string]*devHueGroup{}}
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := ""
		if r.Body != nil {
			buf := make([]byte, 4096)
			n, _ := r.Body.Read(buf)
			body = string(buf[:n])
		}
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
			_ = json.NewEncoder(w).Encode(map[string]any{"8": map[string]any{"name": lightName, "type": "Extended color light", "modelid": "LCA001",
				"state": map[string]any{"on": false, "bri": 10, "xy": []float64{0.3, 0.3}, "reachable": true}}})
		case len(parts) == 5 && parts[4] == "state" && r.Method == "PUT":
			_, _ = w.Write([]byte(`[{"success":{"/lights/8/state/on":true}}]`))
		// Batch B (§5.8): the small set of /groups operations reconciliation
		// needs — list/create, correct/delete one by id, write its action.
		case len(parts) == 3 && parts[2] == "groups" && r.Method == "GET":
			b.mu.Lock()
			out := map[string]any{}
			for id, g := range b.groups {
				out[id] = map[string]any{"name": g.name, "type": "LightGroup", "lights": g.lights}
			}
			b.mu.Unlock()
			_ = json.NewEncoder(w).Encode(out)
		case len(parts) == 3 && parts[2] == "groups" && r.Method == "POST":
			var req struct {
				Name   string   `json:"name"`
				Lights []string `json:"lights"`
			}
			_ = json.Unmarshal([]byte(body), &req)
			b.mu.Lock()
			b.nextGroup++
			id := fmt.Sprint(b.nextGroup)
			b.groups[id] = &devHueGroup{name: req.Name, lights: req.Lights}
			b.mu.Unlock()
			_, _ = w.Write([]byte(`[{"success":{"id":"` + id + `"}}]`))
		case len(parts) == 4 && parts[2] == "groups" && r.Method == "PUT":
			id := parts[3]
			b.mu.Lock()
			g, ok := b.groups[id]
			b.mu.Unlock()
			if !ok {
				_, _ = w.Write([]byte(`[{"error":{"type":3,"address":"/groups/` + id + `","description":"resource not available"}}]`))
				return
			}
			var req struct {
				Lights []string `json:"lights"`
			}
			_ = json.Unmarshal([]byte(body), &req)
			if req.Lights != nil {
				b.mu.Lock()
				g.lights = req.Lights
				b.mu.Unlock()
			}
			_, _ = w.Write([]byte(`[{"success":{"/groups/` + id + `/lights":true}}]`))
		case len(parts) == 4 && parts[2] == "groups" && r.Method == "DELETE":
			id := parts[3]
			b.mu.Lock()
			_, ok := b.groups[id]
			delete(b.groups, id)
			b.mu.Unlock()
			if !ok {
				_, _ = w.Write([]byte(`[{"error":{"type":3,"address":"/groups/` + id + `","description":"resource not available"}}]`))
				return
			}
			_, _ = w.Write([]byte(`[{"success":"/groups/` + id + ` deleted"}]`))
		case len(parts) == 5 && parts[2] == "groups" && parts[4] == "action" && r.Method == "PUT":
			id := parts[3]
			b.mu.Lock()
			_, ok := b.groups[id]
			b.mu.Unlock()
			if !ok {
				_, _ = w.Write([]byte(`[{"error":{"type":3,"address":"/groups/` + id + `/action","description":"resource not available"}}]`))
				return
			}
			_, _ = w.Write([]byte(`[{"success":{"/groups/` + id + `/action/on":true}}]`))
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
	// previewColor overrides PreviewColor's resolution for a test that
	// cares about the exact colour (e.g. an HTTP-level test checking what
	// reaches the bridge); nil uses devPreviewColor's own minimal default
	// (this fake has no team palette or live question of its own).
	previewColor func(role string) [3]int
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

// devWhite/devTeamMarker are this fake's own minimal PreviewColor default
// (no real team palette or live question here) — a "team:" role gets a
// colour that can never be confused with the general/white default, so a
// test can assert role resolution actually happened without needing a real
// engine.
var (
	devWhite      = [3]int{255, 255, 255}
	devTeamMarker = [3]int{9, 9, 9}
)

func (p *devProvider) PreviewColor(role string) [3]int {
	p.mu.Lock()
	override := p.previewColor
	p.mu.Unlock()
	if override != nil {
		return override(role)
	}
	if strings.HasPrefix(role, "team:") {
		return devTeamMarker
	}
	return devWhite
}

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
	// The fake bridge only ever saw guarded paths — checked against the REAL
	// guard (hue.GuardRequest), not a hardcoded substring list, so this stays
	// meaningful as the allow-list evolves (Batch B, §5.8: TestFlash's own
	// ensureResolved now also reconciles groups, so "groups" requests are
	// legitimately expected here — what must never happen is one the guard
	// itself would refuse).
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	for _, h := range bridge.hits {
		method, path, ok := strings.Cut(h, " ")
		if !ok {
			t.Fatalf("malformed recorded hit %q", h)
		}
		if err := hue.GuardRequest(method, path); err != nil {
			t.Errorf("request reached the bridge but the guard would have refused it: %s (%v)", h, err)
		}
	}
}

// TestDevLightingPreview_TurnsOnAndOffEvenUnconfigured is P2a's own round
// trip (#207, planner-v10-general-theme-toggle-20260908-114420.md §Partie
// 2): the light named is NEVER saved to the driver's configuration (zero
// hue.LightSpec) — exactly the checked-before-registered case the endpoint
// exists for, mirroring TestDevLightingStatusAndTestWithDriver's own
// bridge/driver setup for /test.
func TestDevLightingPreview_TurnsOnAndOffEvenUnconfigured(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	bridge := newDevHueBridge(t)
	d, err := hue.New(hue.Config{BridgeIP: bridge.srv.URL, BridgeID: bridge.bridgeID, APIKey: bridge.key,
		// Deliberately NO Lights entry for "BuzzHue1" — it is on the fake
		// bridge (newDevHueBridge always names its one light "BuzzHue1") but
		// never configured, the exact scenario this endpoint must handle.
		FindBridge: func(_ context.Context, _ string, _ time.Duration) (hue.Bridge, bool, error) {
			return hue.Bridge{}, false, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	srv.Lighting = &devProvider{d: d}

	code, out := devDo(t, srv, "POST", "/api/lighting/preview", `{"name":"BuzzHue1","on":true}`)
	if code != 200 || out["result"] != "ok" {
		t.Fatalf("preview on (unconfigured light): %d %v", code, out)
	}
	code, out = devDo(t, srv, "POST", "/api/lighting/preview", `{"name":"BuzzHue1","on":false}`)
	if code != 200 || out["result"] != "ok" {
		t.Fatalf("preview off: %d %v", code, out)
	}
	code, out = devDo(t, srv, "POST", "/api/lighting/preview", `{"name":"Inconnue","on":true}`)
	if code != 500 || out["result"] != "error" {
		t.Fatalf("previewing a name matching no light: %d %v", code, out)
	}
	code, _ = devDo(t, srv, "POST", "/api/lighting/preview", `{"on":true}`)
	if code != 400 {
		t.Fatalf("empty name must be rejected before any I/O: %d", code)
	}
}

// TestDevLightingPreview_NoDriverIsRefused mirrors /test's own "not
// configured" answer (contract §5.6 taxonomy): no lighting driver at all
// must never attempt a network call.
func TestDevLightingPreview_NoDriverIsRefused(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	code, out := devDo(t, srv, "POST", "/api/lighting/preview", `{"name":"BuzzHue1","on":true}`)
	if code != http.StatusConflict || out["result"] != "refused" || out["reason"] != "not_configured" {
		t.Fatalf("no driver: want 409 refused/not_configured, got %d %v", code, out)
	}
}

// TestDevLightingPreview_BusyGuard mirrors TestDevLightingOneInFlightPerOperation's
// pattern (http_lighting_sec_dev_test.go) for the one operation that table
// cannot cover without its own driver setup: a second /preview call while
// one is already in flight gets 429, never a second network exchange.
func TestDevLightingPreview_BusyGuard(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	bridge := newDevHueBridge(t)
	d, err := hue.New(hue.Config{BridgeIP: bridge.srv.URL, BridgeID: bridge.bridgeID, APIKey: bridge.key,
		FindBridge: func(_ context.Context, _ string, _ time.Duration) (hue.Bridge, bool, error) {
			return hue.Bridge{}, false, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	srv.Lighting = &devProvider{d: d}

	if !lightingBusy.preview.CompareAndSwap(false, true) {
		t.Fatal("flag already held")
	}
	code, out := devDo(t, srv, "POST", "/api/lighting/preview", `{"name":"BuzzHue1","on":true}`)
	lightingBusy.preview.Store(false)
	if code != http.StatusTooManyRequests || out["result"] != "busy" || out["reason"] != "preview_in_progress" {
		t.Fatalf("want 429 busy/preview_in_progress, got %d %v", code, out)
	}
	// Released: a retry is served normally.
	code, out = devDo(t, srv, "POST", "/api/lighting/preview", `{"name":"BuzzHue1","on":true}`)
	if code != 200 || out["result"] != "ok" {
		t.Fatalf("retry after release: %d %v", code, out)
	}
}

// TestDevLightingPreview_RolePassedToPreviewColor pins the 2026-09-08
// revision (task-dev-backend-preview-real-color-20260908.md): the request
// body's "role" reaches LightingProvider.PreviewColor UNCHANGED — the
// actual colour→xy threading from there down to the bridge is
// internal/lighting/hue's own concern, already covered there
// (TestDevSetLightDirect_UsesTheGivenColour); this test is only about the
// HTTP layer's own plumbing, using devProvider's override hook to observe
// exactly what role the handler passed.
func TestDevLightingPreview_RolePassedToPreviewColor(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	bridge := newDevHueBridge(t)
	d, err := hue.New(hue.Config{BridgeIP: bridge.srv.URL, BridgeID: bridge.bridgeID, APIKey: bridge.key,
		FindBridge: func(_ context.Context, _ string, _ time.Duration) (hue.Bridge, bool, error) {
			return hue.Bridge{}, false, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var gotRole string
	provider := &devProvider{d: d, previewColor: func(role string) [3]int {
		gotRole = role
		return [3]int{1, 2, 3} // arbitrary, distinguishable — this test only checks the ROLE reaches here
	}}
	srv.Lighting = provider

	for _, role := range []string{"team:LesBleus", "general", ""} {
		gotRole = "unset"
		body := `{"name":"BuzzHue1","on":true,"role":"` + role + `"}`
		code, out := devDo(t, srv, "POST", "/api/lighting/preview", body)
		if code != 200 || out["result"] != "ok" {
			t.Fatalf("role=%q: %d %v", role, code, out)
		}
		if gotRole != role {
			t.Errorf("role=%q: PreviewColor received %q", role, gotRole)
		}
	}

	// off: PreviewColor is still called (documented as unconditional, no
	// I/O, keeps a single code path), but its result plays no part in the
	// actual write — hue.Driver.SetLightDirect ignores colour when !on.
	gotRole = "unset"
	code, out := devDo(t, srv, "POST", "/api/lighting/preview", `{"name":"BuzzHue1","on":false,"role":"team:LesBleus"}`)
	if code != 200 || out["result"] != "ok" {
		t.Fatalf("off: %d %v", code, out)
	}
	if gotRole != "team:LesBleus" {
		t.Errorf("off: role must still reach PreviewColor, got %q", gotRole)
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

// TestDevQualifBug1Round2_RealEndToEndFlowWithWhitespaceName reproduces the
// QUALIF round-2 report end to end, through the REAL HTTP handlers only —
// register → GET /api/lighting/lights (discovery, RAW bridge name) →
// POST /config.json "lighting.lights" (save, exactly what AmbiancePage.jsx
// sends: web/src/pages/AmbiancePage.jsx's `rows`/`handleSaveLights` build
// the saved name from `inventory.lights[].name`, i.e. the UNTRIMMED
// discovery response, never from a hand-typed value) → POST
// /api/lighting/test with THAT SAME untrimmed name (AmbiancePage.jsx's
// `handleTest(row.name)`, `row.name` also straight from inventory) — never
// a LightSpec constructed directly in the test, per the round-2 ask.
//
// OnConfigUpdate here rebuilds a *hue.Driver from config.Get().Lighting the
// same way cmd/server/ambiance.go's buildHueDriver does (App-level glue
// cannot be imported into internal/server without a cycle) — everything
// else is the production handler code.
func TestDevQualifBug1Round2_RealEndToEndFlowWithWhitespaceName(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	bridge := newDevHueBridgeNamed(t, "salon gauche ") // trailing space, exactly the reported case
	bridge.mu.Lock()
	bridge.pressed = true
	bridge.mu.Unlock()

	prov := &devProvider{}
	srv.Lighting = prov
	srv.OnConfigUpdate = func() {
		lc := config.Get().Lighting
		if !lc.Enabled || lc.EffectiveAPIKey() == "" || (lc.BridgeIP == "" && lc.BridgeID == "") {
			prov.mu.Lock()
			prov.d = nil
			prov.mu.Unlock()
			return
		}
		specs := make([]hue.LightSpec, 0, len(lc.Lights))
		for _, l := range lc.Lights {
			specs = append(specs, hue.LightSpec{Name: l.Name, Role: hue.LightRole(l.Role), Team: l.Team})
		}
		d, err := hue.New(hue.Config{BridgeIP: lc.BridgeIP, BridgeID: lc.BridgeID, APIKey: lc.EffectiveAPIKey(), Lights: specs})
		if err != nil {
			t.Fatalf("hue.New (OnConfigUpdate rebuild): %v", err)
		}
		prov.mu.Lock()
		old := prov.d
		prov.d = d
		prov.mu.Unlock()
		if old != nil {
			_ = old.Close()
		}
	}

	// 1. Association (bouton pressé) — POST /api/lighting/register.
	code, out := devDo(t, srv, "POST", "/api/lighting/register", `{"bridge_ip":"`+bridge.srv.URL+`"}`)
	if code != 200 {
		t.Fatalf("register: %d %v", code, out)
	}

	// 2. Découverte — GET /api/lighting/lights : le nom BRUT du pont, tel
	// que le frontend le reçoit et l'affiche (AmbiancePage.jsx `rows`).
	code, out = devDo(t, srv, "GET", "/api/lighting/lights", "")
	if code != 200 {
		t.Fatalf("lights: %d %v", code, out)
	}
	lights, _ := out["lights"].([]any)
	if len(lights) != 1 {
		t.Fatalf("expected 1 discovered light, got %v", out)
	}
	discovered := lights[0].(map[string]any)
	rawName, _ := discovered["name"].(string)
	if rawName != "salon gauche " {
		t.Fatalf("setup invalide : le nom découvert devrait porter l'espace parasite, got %q", rawName)
	}

	// 3. Sélection + sauvegarde — POST /config.json {"lighting":{"lights":[...]}}
	// avec le nom EXACTEMENT tel que reçu à la découverte (AmbiancePage.jsx
	// handleSaveLights : `effectiveSelected.map(name => ({name, ...}))`,
	// `effectiveSelected` dérivé de `rows`, lui-même de `inventory.lights`).
	code, out = devDo(t, srv, "POST", "/config.json", `{"lighting":{"lights":[{"name":"`+rawName+`","role":"general"}]}}`)
	if code != 200 {
		t.Fatalf("save lights: %d %v", code, out)
	}
	if got := config.Get().Lighting.Lights[0].Name; got != "salon gauche" {
		t.Fatalf("le nom sauvegardé doit être trimé côté serveur, got %q", got)
	}

	// 4. Test — POST /api/lighting/test avec le nom RAW (celui affiché dans
	// la ligne du tableau, `row.name`, jamais re-résolu depuis la config
	// sauvegardée côté frontend).
	code, out = devDo(t, srv, "POST", "/api/lighting/test", `{"name":"`+rawName+`"}`)
	if code != 200 || out["result"] != "ok" {
		t.Fatalf(`régression QUALIF round 2 (bug 1) : POST /api/lighting/test échoue encore avec le nom brut de la découverte — %d %v`, code, out)
	}
}
