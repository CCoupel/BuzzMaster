package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The guard is the safety-critical piece: every forbidden operation of the
// handoff must be rejected at the code level, whatever the caller does.
func TestGuardAllowsOnlyTheThreeOperations(t *testing.T) {
	allowed := []struct{ m, p string }{
		{"GET", "/api/abc/lights"},
		{"GET", "/api/abc/lights/7"},
		{"PUT", "/api/abc/lights/7/state"},
		{"POST", "/api"},
	}
	for _, a := range allowed {
		if err := guardRequest(a.m, a.p); err != nil {
			t.Errorf("%s %s must be allowed: %v", a.m, a.p, err)
		}
	}
	forbidden := []struct{ m, p string }{
		{"PUT", "/api/abc/groups/0/action"}, // ALL lights
		{"PUT", "/api/abc/groups/3/action"}, // zone BuzzMaster1 or any group
		{"GET", "/api/abc/groups"},
		{"PUT", "/api/abc/config"},
		{"GET", "/api/abc/config"},
		{"PUT", "/api/abc/scenes/x"},
		{"POST", "/api/abc/scenes"},
		{"PUT", "/api/abc/rules/1"},
		{"PUT", "/api/abc/schedules/1"},
		{"PUT", "/api/abc/sensors/1/state"},
		{"PUT", "/api/abc/resourcelinks/1"},
		{"DELETE", "/api/abc/lights/7"},
		{"DELETE", "/api/abc/config/whitelist/xyz"},
		{"PUT", "/api/abc/lights/0/state"}, // id 0
		{"PUT", "/api/abc/lights/7"},       // rename = attribute write
		{"PUT", "/api/abc/lights"},
		{"POST", "/api/abc/lights"}, // search for new lights
		{"PUT", "/api/abc/lights/7/state?x"},
		{"GET", "/api/abc/../config"},
		{"GET", "/api"},
		{"POST", "/api/abc/lights/7/state"},
		{"PUT", "/api/abc/lights/07/state"},  // odd id form
		{"PUT", "/clip/v2/resource/light/x"}, // API v2
	}
	for _, f := range forbidden {
		if err := guardRequest(f.m, f.p); err == nil {
			t.Errorf("%s %s MUST be refused", f.m, f.p)
		}
	}
	if lightIDFromStatePath("/api/u/lights/12/state") != "12" || lightIDFromStatePath("/api/u/lights") != "" {
		t.Error("lightIDFromStatePath wrong")
	}
}

// End-to-end against a fake bridge: the client must refuse to write when the
// light name no longer matches, must never hit any other path, and must
// stop on ambiguous/missing names.
func TestClientWritesOnlyTheNamedLight(t *testing.T) {
	var hits []string
	lights := map[string]map[string]any{
		"1": {"name": "Salon", "state": map[string]any{"on": true}},
		"4": {"name": "BuzzHue1", "state": map[string]any{"on": false, "bri": 10}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/u/lights":
			_ = json.NewEncoder(w).Encode(lights)
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/u/lights/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/u/lights/")
			_ = json.NewEncoder(w).Encode(lights[id])
		case r.Method == "PUT" && r.URL.Path == "/api/u/lights/4/state":
			_, _ = w.Write([]byte(`[{"success":{"/lights/4/state/on":true}}]`))
		default:
			t.Errorf("fake bridge received an unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "u", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	target, _, err := c.FindTarget(ctx, "BuzzHue1")
	if err != nil || target.ID != "4" {
		t.Fatalf("FindTarget = %+v, %v", target, err)
	}
	if _, err := c.SetState(ctx, target, State{On: boolp(true)}); err != nil {
		t.Fatalf("SetState on the target must succeed: %v", err)
	}
	// Someone renames light 4 between two writes → refuse.
	lights["4"]["name"] = "Autre"
	if _, err := c.SetState(ctx, target, State{On: boolp(false)}); err == nil || !strings.Contains(err.Error(), "SAFETY GUARD") {
		t.Fatalf("write after rename must be refused, got %v", err)
	}
	lights["4"]["name"] = "BuzzHue1"
	// Forged targets are refused before any request.
	for _, bad := range []Target{{ID: "0", Name: "BuzzHue1"}, {ID: "", Name: "BuzzHue1"}, {ID: "4", Name: ""}} {
		if _, err := c.SetState(ctx, bad, State{On: boolp(true)}); err == nil {
			t.Errorf("forged target %+v must be refused", bad)
		}
	}
	// Missing / ambiguous names stop the program.
	if _, _, err := c.FindTarget(ctx, "Nope"); err == nil {
		t.Error("missing name must error")
	}
	lights["1"]["name"] = "BuzzHue1"
	if _, _, err := c.FindTarget(ctx, "BuzzHue1"); err != errTargetAmbiguous {
		t.Errorf("ambiguous name must error, got %v", err)
	}
	for _, h := range hits {
		if strings.Contains(h, "groups") || strings.HasPrefix(h, "DELETE") || strings.Contains(h, "config") {
			t.Errorf("forbidden request reached the bridge: %s", h)
		}
	}
}

func TestParseResultArrayAndRegistration(t *testing.T) {
	succ, errs, err := parseResultArray([]byte(`[{"error":{"type":101,"address":"","description":"link button not pressed"}}]`))
	if err != nil || len(succ) != 0 || len(errs) != 1 || errs[0].Type != 101 {
		t.Fatalf("101 parse: %v %v %v", succ, errs, err)
	}
	succ, errs, err = parseResultArray([]byte(`[{"success":{"username":"abc123"}}]`))
	if err != nil || len(errs) != 0 || len(succ) != 1 {
		t.Fatalf("success parse: %v %v %v", succ, errs, err)
	}
	if _, _, err := parseResultArray([]byte(`{"not":"an array"}`)); err == nil {
		t.Error("object must not parse as result array")
	}

	// Registration flow: two 101s then success.
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		calls++
		if calls < 3 {
			_, _ = w.Write([]byte(`[{"error":{"type":101,"description":"link button not pressed"}}]`))
			return
		}
		_, _ = w.Write([]byte(`[{"success":{"username":"key-xyz"}}]`))
	}))
	defer srv.Close()
	user, err := Register(context.Background(), srv.URL, "buzzmaster-spike#test", time.Second, 30*time.Second, func(string, ...any) {})
	if err != nil || user != "key-xyz" || calls != 3 {
		t.Fatalf("Register = %q, %v (calls %d)", user, err, calls)
	}
}

func TestColourAndSSDPParsing(t *testing.T) {
	xy, err := colorXY("red")
	if err != nil || xy[0] < 0.6 || xy[1] > 0.35 {
		t.Errorf("red xy = %v %v", xy, err)
	}
	if _, err := colorXY("plaid"); err == nil {
		t.Error("unknown colour must error")
	}
	resp := "HTTP/1.1 200 OK\r\nHOST: 239.255.255.250:1900\r\nLOCATION: http://192.168.1.42:80/description.xml\r\nSERVER: Hue/1.0 UPnP/1.0 IpBridge/1.67.0\r\nhue-bridgeid: 001788FFFE123456\r\n\r\n"
	b, ok := parseSSDPResponse(resp)
	if !ok || b.IP != "192.168.1.42" || b.ID != "001788FFFE123456" || b.Source != "ssdp" {
		t.Errorf("ssdp parse = %+v %v", b, ok)
	}
	if _, ok := parseSSDPResponse("HTTP/1.1 200 OK\r\nSERVER: Linux UPnP/1.0 Sonos\r\nLOCATION: http://10.0.0.5/x\r\n\r\n"); ok {
		t.Error("non-Hue responder must be ignored")
	}
	if _, err := NewClient("192.168.1.10", "u", time.Second); err == nil {
		t.Error("bridge URL without scheme must be rejected by NewClient")
	}
}
