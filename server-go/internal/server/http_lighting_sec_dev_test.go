package server

// Security regression tests for /api/lighting/* (SSRF audit 2026-09-04).
// Owner: dev-backend (TestDev*/dev* identifiers, *_dev_test.go).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"buzzcontrol/internal/config"
)

// The httptest bridges of this package live on 127.0.0.1: allow loopback for
// the whole test binary (a dedicated test flips it back to check production).
func init() { lightingAllowLoopback.Store(true) }

func TestDevValidateBridgeAddressRanges(t *testing.T) {
	ctx := context.Background()
	ok := []string{
		"192.168.1.10", "10.0.0.1", "172.16.0.1", "172.31.255.254", "169.254.10.10",
		"192.168.1.10:80", "http://192.168.1.10", "https://192.168.1.10:443", "http://192.168.1.10/",
		"[fe80::1]", "[fd00::1]:80", "http://[fe80::1%25eth0]",
	}
	for _, in := range ok {
		if _, err := validateBridgeAddress(ctx, in); err != nil {
			t.Errorf("%q must be accepted: %v", in, err)
		}
	}
	bad := []string{
		"", "8.8.8.8", "1.1.1.1:80", "http://8.8.8.8", "172.32.0.1", "172.15.255.255", "11.0.0.1",
		"0.0.0.0", "224.0.0.1", "255.255.255.255", "[2001:db8::1]", "[::]", "[::ffff:8.8.8.8]",
		"ftp://192.168.1.10", "http://user:pw@192.168.1.10", "192.168.1.10/api", "http://192.168.1.10/api",
		"192.168.1.10?x=1", "192.168.1.10#f", "192.168.1.10 8.8.8.8", "http://192.168.1.10\r\nX: y",
		"metadata.invalid", // unresolvable hostname
	}
	for _, in := range bad {
		if got, err := validateBridgeAddress(ctx, in); err == nil {
			t.Errorf("%q must be refused, got %q", in, got)
		}
	}
	// Loopback: allowed only through the test hook.
	lightingAllowLoopback.Store(false)
	t.Cleanup(func() { lightingAllowLoopback.Store(true) })
	for _, in := range []string{"127.0.0.1", "http://127.0.0.1:8080", "[::1]", "localhost"} {
		if _, err := validateBridgeAddress(ctx, in); err == nil {
			t.Errorf("%q must be refused when the loopback hook is off", in)
		}
	}
}

func TestDevValidateBridgeAddressNormalises(t *testing.T) {
	ctx := context.Background()
	cases := map[string]string{
		" 192.168.1.10 ":           "192.168.1.10",
		"192.168.1.10:8080":        "192.168.1.10:8080",
		"http://192.168.1.10/":     "http://192.168.1.10",
		"https://192.168.1.10:443": "https://192.168.1.10:443",
	}
	for in, want := range cases {
		got, err := validateBridgeAddress(ctx, in)
		if err != nil || got != want {
			t.Errorf("%q → %q, %v; want %q", in, got, err, want)
		}
	}
}

// A public target is refused BEFORE any outbound call: the answer is
// immediate and the reason is generic (no reflection of the input).
func TestDevLightingRegisterRefusesPublicTargetWithoutNetwork(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	for _, target := range []string{"8.8.8.8", "http://1.1.1.1", "[2001:db8::1]", "http://user@192.168.1.10", "192.168.1.10/api"} {
		start := time.Now()
		code, out := devDo(t, srv, "POST", "/api/lighting/register", `{"bridge_ip":"`+strings.ReplaceAll(target, `"`, ``)+`"}`)
		if code != http.StatusBadRequest || out["reason"] != "bridge_ip_not_private" || out["result"] != "error" {
			t.Errorf("%s: want 400 bridge_ip_not_private, got %d %v", target, code, out)
		}
		if d := time.Since(start); d > 500*time.Millisecond {
			t.Errorf("%s: refused only after %v — an outbound call happened", target, d)
		}
		for _, v := range out {
			if s, _ := v.(string); strings.Contains(s, target) {
				t.Errorf("%s: input reflected in the answer: %v", target, out)
			}
		}
	}
	if lc := config.Get().Lighting; lc.BridgeIP != "" || lc.APIKey != "" {
		t.Errorf("config must be untouched, got %+v", lc)
	}
}

// The target's body never reaches the client, whatever it answers.
func TestDevLightingRegisterNeverEchoesTargetBody(t *testing.T) {
	const marker = "SECRET-INTERNAL-BANNER-4242"
	bodies := []struct {
		code int
		body string
	}{
		{200, marker + " not json at all"},
		{200, `{"foo":"` + marker + `"}`},
		{200, `[{"error":{"type":7,"address":"/","description":"` + marker + `"}}]`},
		{500, marker},
		{404, "<html>" + marker + "</html>"},
	}
	for _, b := range bodies {
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(b.code)
			_, _ = w.Write([]byte(b.body))
		}))
		srv, _ := setupTestHTTPServer(t)
		req := httptest.NewRequest("POST", "/api/lighting/register", strings.NewReader(`{"bridge_ip":"`+target.URL+`"}`))
		rec := httptest.NewRecorder()
		srv.handleLightingRegister(rec, req)
		target.Close()
		if rec.Code == http.StatusOK {
			t.Errorf("body %q: registration must not succeed", b.body)
		}
		if strings.Contains(rec.Body.String(), marker) || strings.Contains(rec.Body.String(), "not json") {
			t.Errorf("body %q reflected to the client: %s", b.body, rec.Body.String())
		}
		if lc := config.Get().Lighting; lc.APIKey != "" || lc.BridgeIP != "" {
			t.Errorf("body %q: config must be untouched, got %+v", b.body, lc)
		}
	}
}

// A key handed back by the bridge is validated before config.Save.
func TestDevLightingRegisterRejectsMalformedKeyBeforeSave(t *testing.T) {
	for _, key := range []string{"a/b/c", "short", `k"k"kkkkkk`, "key with space", strings.Repeat("x", 200), "k?k#kkkkkkk"} {
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[{"success":{"username":` + jsonString(key) + `}}]`))
		}))
		srv, _ := setupTestHTTPServer(t)
		updates := 0
		srv.OnConfigUpdate = func() { updates++ }
		code, out := devDo(t, srv, "POST", "/api/lighting/register", `{"bridge_ip":"`+target.URL+`"}`)
		target.Close()
		if code != http.StatusBadGateway || out["reason"] != "invalid_key_from_bridge" {
			t.Errorf("key %q: want 502 invalid_key_from_bridge, got %d %v", key, code, out)
		}
		if lc := config.Get().Lighting; lc.APIKey != "" || lc.BridgeIP != "" || updates != 0 {
			t.Errorf("key %q: nothing must be persisted (config %+v, updates %d)", key, lc, updates)
		}
		for _, v := range out {
			if s, _ := v.(string); strings.Contains(s, key) {
				t.Errorf("key %q reflected: %v", key, out)
			}
		}
	}
}

func jsonString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// The default error branch is generic whatever the wrapped message says.
func TestDevWriteLightingErrorIsGeneric(t *testing.T) {
	rec := httptest.NewRecorder()
	writeLightingError(rec, errDevWrapped("hue: unexpected response \"TOP-SECRET body\": boom"))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "TOP-SECRET") || strings.Contains(rec.Body.String(), "message") {
		t.Errorf("generic answer expected, got %d %s", rec.Code, rec.Body.String())
	}
}

type errDevWrapped string

func (e errDevWrapped) Error() string { return string(e) }

// One register / discover / test in flight at a time → 429 for the second.
func TestDevLightingOneInFlightPerOperation(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	ops := []struct {
		flag         *atomic.Bool
		method, path string
		body         string
		reason       string
	}{
		{&lightingBusy.register, "POST", "/api/lighting/register", `{"bridge_ip":"192.168.1.10"}`, "register_in_progress"},
		{&lightingBusy.discover, "POST", "/api/lighting/discover", ``, "discover_in_progress"},
	}
	for _, op := range ops {
		if !op.flag.CompareAndSwap(false, true) {
			t.Fatalf("%s: flag already held", op.path)
		}
		code, out := devDo(t, srv, op.method, op.path, op.body)
		op.flag.Store(false)
		if code != http.StatusTooManyRequests || out["result"] != "busy" || out["reason"] != op.reason {
			t.Errorf("%s: want 429 busy/%s, got %d %v", op.path, op.reason, code, out)
		}
	}
	// The flag is released after a (refused) attempt: a retry is served.
	if code, _ := devDo(t, srv, "POST", "/api/lighting/register", `{"bridge_ip":"8.8.8.8"}`); code != http.StatusBadRequest {
		t.Errorf("retry after release: want 400, got %d", code)
	}
	if lightingBusy.register.Load() || lightingBusy.discover.Load() || lightingBusy.test.Load() {
		t.Error("in-flight flags must all be released")
	}
}

// Two truly concurrent registrations against a slow bridge: exactly one
// exchange reaches the bridge, the other caller gets 429.
func TestDevLightingRegisterConcurrentIsSerialised(t *testing.T) {
	var hits atomic.Int32
	entered := make(chan struct{})
	block := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		close(entered)
		<-block
		_, _ = w.Write([]byte(`[{"error":{"type":101,"address":"","description":"link button not pressed"}}]`))
	}))
	defer slow.Close()
	srv, _ := setupTestHTTPServer(t)
	first := make(chan int)
	go func() {
		code, _ := devDo(t, srv, "POST", "/api/lighting/register", `{"bridge_ip":"`+slow.URL+`"}`)
		first <- code
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("first registration never reached the bridge")
	}
	code2, out2 := devDo(t, srv, "POST", "/api/lighting/register", `{"bridge_ip":"`+slow.URL+`"}`)
	close(block)
	if code2 != http.StatusTooManyRequests || out2["reason"] != "register_in_progress" {
		t.Errorf("second concurrent register: want 429, got %d %v", code2, out2)
	}
	if c := <-first; c != http.StatusConflict {
		t.Errorf("first register: want 409 link_button_not_pressed, got %d", c)
	}
	if hits.Load() != 1 {
		t.Errorf("bridge must be contacted exactly once, got %d", hits.Load())
	}
}

// POST /config.json applies the same private-network rule to bridge_ip.
func TestDevConfigLightingBridgeIPMustBePrivate(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	for _, ip := range []string{"8.8.8.8", "http://1.1.1.1", "192.168.1.10/api"} {
		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"lighting":{"bridge_ip":"`+ip+`"}}`))
		rec := httptest.NewRecorder()
		srv.handleConfig(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("bridge_ip %q via /config.json: want 400, got %d %s", ip, rec.Code, rec.Body.String())
		}
		if config.Get().Lighting.BridgeIP != "" {
			t.Errorf("bridge_ip %q must not be stored", ip)
		}
	}
}
