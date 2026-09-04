package server

// Developer-side tests for /api/lighting/* (dev-backend, #207 — contract
// hue-bridge.md §7). test-writer owns http_lighting_config_test.go; every
// identifier here is prefixed dev/Dev.

import (
	"context"
	"encoding/json"
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

// devProvider hands the handlers a driver (or nil).
type devProvider struct {
	mu sync.Mutex
	d  *hue.Driver
}

func (p *devProvider) LightingDriver() *hue.Driver { p.mu.Lock(); defer p.mu.Unlock(); return p.d }

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
	if updates != 1 {
		t.Errorf("OnConfigUpdate must fire once, got %d", updates)
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

	// Inventory works with the stored key even though no driver is enabled yet.
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
