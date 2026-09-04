package hue

// Developer-side tests (dev-backend, #206). test-writer owns driver_test.go
// and color_test.go; every identifier here is prefixed dev/Dev to coexist.
// Covers: change-only writes, name resolution (missing/ambiguous/renamed),
// refused vs unreachable with backoff and single log line, per-light
// failures, team zones, bridge identity re-discovery, inventory/test flash,
// registration, and the §8 measurements against the fake bridge.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"buzzcontrol/internal/lighting"
)

type devBridge struct {
	mu        sync.Mutex
	lights    map[string]map[string]any // id → light object
	bridgeID  string
	key       string
	latency   time.Duration
	offLights map[string]bool // ids answering error 201 on writes
	requests  []devRecorded
	srv       *httptest.Server
}

type devRecorded struct {
	at     time.Time
	method string
	path   string
	body   string
}

func newDevBridge(t *testing.T, names ...string) *devBridge {
	t.Helper()
	f := &devBridge{lights: map[string]map[string]any{}, bridgeID: "fffe0000deadbeef", key: "k", offLights: map[string]bool{}}
	for i, n := range names {
		f.lights[fmt.Sprint(i+1)] = map[string]any{"name": n, "type": "Extended color light", "modelid": "LCA001",
			"state": map[string]any{"on": false, "bri": 1, "xy": []float64{0.3, 0.3}, "reachable": true}}
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *devBridge) handle(w http.ResponseWriter, r *http.Request) {
	body := new(strings.Builder)
	if r.Body != nil {
		b := make([]byte, 4096)
		n, _ := r.Body.Read(b)
		body.Write(b[:n])
	}
	f.mu.Lock()
	f.requests = append(f.requests, devRecorded{time.Now(), r.Method, r.URL.Path, body.String()})
	lat := f.latency
	f.mu.Unlock()
	if lat > 0 {
		time.Sleep(lat)
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "api" {
		w.WriteHeader(404)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if parts[1] != f.key {
		_, _ = w.Write([]byte(`[{"error":{"type":1,"address":"/","description":"unauthorized user"}}]`))
		return
	}
	switch {
	case len(parts) == 3 && parts[2] == "config" && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"bridgeid": f.bridgeID, "name": "fake", "modelid": "BSB002", "swversion": "1", "apiversion": "1.60"})
	case len(parts) == 3 && parts[2] == "lights" && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(f.lights)
	case len(parts) == 4 && parts[2] == "lights" && r.Method == "GET":
		l, ok := f.lights[parts[3]]
		if !ok {
			_, _ = w.Write([]byte(`[{"error":{"type":3,"address":"/lights/` + parts[3] + `","description":"resource not available"}}]`))
			return
		}
		_ = json.NewEncoder(w).Encode(l)
	case len(parts) == 5 && parts[2] == "lights" && parts[4] == "state" && r.Method == "PUT":
		id := parts[3]
		l, ok := f.lights[id]
		if !ok {
			_, _ = w.Write([]byte(`[{"error":{"type":3,"address":"/lights/` + id + `/state","description":"resource not available"}}]`))
			return
		}
		if f.offLights[id] {
			_, _ = w.Write([]byte(`[{"error":{"type":201,"address":"/lights/` + id + `/state/bri","description":"parameter, bri, is not modifiable. Device is set to off."}}]`))
			return
		}
		var st map[string]any
		_ = json.Unmarshal([]byte(body.String()), &st)
		state := l["state"].(map[string]any)
		for k, v := range st {
			if k != "transitiontime" {
				state[k] = v
			}
		}
		_, _ = w.Write([]byte(`[{"success":{"/lights/` + id + `/state/on":true}}]`))
	default:
		w.WriteHeader(404)
	}
}

func (f *devBridge) puts() []devRecorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []devRecorded
	for _, r := range f.requests {
		if r.method == "PUT" {
			out = append(out, r)
		}
	}
	return out
}

func (f *devBridge) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.requests {
		out = append(out, r.method+" "+r.path)
	}
	return out
}

func devGeneral(color [3]int, intensity int) lighting.State {
	return lighting.State{Zones: []lighting.ZoneState{{Zone: lighting.ZoneGeneral, Color: color, Intensity: intensity}}}
}

type devLogSink struct {
	mu    sync.Mutex
	lines []string
}

func (l *devLogSink) logf(f string, a ...any) {
	l.mu.Lock()
	l.lines = append(l.lines, fmt.Sprintf(f, a...))
	l.mu.Unlock()
}

func (l *devLogSink) count() int { l.mu.Lock(); defer l.mu.Unlock(); return len(l.lines) }

func newDevDriver(t *testing.T, f *devBridge, lights ...LightSpec) (*Driver, *devLogSink) {
	t.Helper()
	sink := &devLogSink{}
	d, err := New(Config{BridgeIP: f.srv.URL, BridgeID: f.bridgeID, APIKey: f.key, Lights: lights, Logger: sink.logf, DiscoverTimeout: 10 * time.Millisecond,
		FindBridge: func(context.Context, string, time.Duration) (Bridge, bool, error) { return Bridge{}, false, nil }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d, sink
}

func devMs(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func TestDevNewValidatesWithoutIO(t *testing.T) {
	bad := []Config{
		{BridgeIP: "1.2.3.4"},
		{APIKey: "k"},
		{BridgeIP: "1.2.3.4", APIKey: "k", Lights: []LightSpec{{Name: ""}}},
		{BridgeIP: "1.2.3.4", APIKey: "k", Lights: []LightSpec{{Name: "A"}, {Name: "A"}}},
		{BridgeIP: "1.2.3.4", APIKey: "k", Lights: []LightSpec{{Name: "A", Role: "team"}}},
		{BridgeIP: "1.2.3.4", APIKey: "k", Lights: []LightSpec{{Name: "A", Role: "zone"}}},
		{BridgeIP: "1.2.3.4", APIKey: "k/x"},
		{BridgeIP: "http://1.2.3.4/path", APIKey: "k"},
	}
	for i, c := range bad {
		if _, err := New(c); err == nil {
			t.Errorf("config %d must be rejected", i)
		}
	}
	d, err := New(Config{BridgeIP: "192.168.1.101", BridgeID: "fffe0000deadbeef", APIKey: "k", Lights: []LightSpec{{Name: "BuzzHue1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Status().State != StateUnreachable || d.cfg.Lights[0].Role != RoleGeneral {
		t.Fatalf("fresh driver: %+v", d.Status())
	}
}

func TestDevApplyResolvesByNameAndWritesOnlyChanges(t *testing.T) {
	f := newDevBridge(t, "Salon", "BuzzHue1", "Cuisine")
	d, sink := newDevDriver(t, f, LightSpec{Name: "BuzzHue1"})
	ctx := context.Background()

	if err := d.Apply(ctx, devGeneral([3]int{255, 0, 0}, 255)); err != nil {
		t.Fatal(err)
	}
	puts := f.puts()
	if len(puts) != 1 || puts[0].path != "/api/k/lights/2/state" {
		t.Fatalf("expected one PUT on light 2 (BuzzHue1), got %+v", puts)
	}
	if !strings.Contains(puts[0].body, `"on":true`) || !strings.Contains(puts[0].body, `"bri":254`) || !strings.Contains(puts[0].body, `"transitiontime":0`) {
		t.Errorf("payload: %s", puts[0].body)
	}
	if err := d.Apply(ctx, devGeneral([3]int{255, 0, 0}, 255)); err != nil {
		t.Fatal(err)
	}
	if len(f.puts()) != 1 {
		t.Fatalf("unchanged state must not be written, got %d PUTs", len(f.puts()))
	}
	_ = d.Apply(ctx, devGeneral([3]int{0, 0, 255}, 200))
	_ = d.Apply(ctx, devGeneral([3]int{0, 0, 255}, 0))
	puts = f.puts()
	if len(puts) != 3 || !strings.Contains(puts[2].body, `"on":false`) || strings.Contains(puts[2].body, `"bri"`) {
		t.Fatalf("PUTs: %+v", puts)
	}
	st := d.Status()
	if st.State != StateOK || st.LightsOK != 1 || st.Lights[0].ID != "2" || st.Stats.Skipped != 1 || st.Stats.Inventories != 1 {
		t.Fatalf("status: %+v", st)
	}
	for _, p := range f.paths() {
		if strings.Contains(p, "groups") || strings.HasPrefix(p, "DELETE") {
			t.Errorf("forbidden request reached the bridge: %s", p)
		}
	}
	if n := sink.count(); n != 2 {
		t.Errorf("expected 2 log lines (resolution, ok), got %d: %v", n, sink.lines)
	}
}

func TestDevMissingAndAmbiguousNamesNeverFallBack(t *testing.T) {
	f := newDevBridge(t, "Salon", "BuzzHue1", "BuzzHue1")
	d, sink := newDevDriver(t, f, LightSpec{Name: "BuzzHue1"}, LightSpec{Name: "Absente"})
	if err := d.Apply(context.Background(), devGeneral([3]int{255, 255, 255}, 255)); err != nil {
		t.Fatalf("missing/ambiguous lights must not fail Apply: %v", err)
	}
	if len(f.puts()) != 0 {
		t.Fatalf("no write allowed when the name is ambiguous or missing, got %+v", f.puts())
	}
	st := d.Status()
	if st.State != StateOK || st.LightsOK != 0 {
		t.Fatalf("status: %+v", st)
	}
	byName := map[string]LightStatus{}
	for _, l := range st.Lights {
		byName[l.Name] = l
	}
	if !byName["BuzzHue1"].Ambiguous || byName["BuzzHue1"].Resolved || !strings.HasPrefix(byName["BuzzHue1"].LastError, "ambiguous") {
		t.Errorf("BuzzHue1 must be ambiguous: %+v", byName["BuzzHue1"])
	}
	if byName["Absente"].Resolved || byName["Absente"].LastError != "not found" {
		t.Errorf("Absente must be not found: %+v", byName["Absente"])
	}
	if sink.count() != 2 {
		t.Errorf("log lines: %v", sink.lines)
	}
}

func TestDevRenamedLightStopsBeingWritten(t *testing.T) {
	f := newDevBridge(t, "BuzzHue1")
	d, _ := newDevDriver(t, f, LightSpec{Name: "BuzzHue1"})
	clock := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	d.now = func() time.Time { return clock }
	ctx := context.Background()
	_ = d.Apply(ctx, devGeneral([3]int{255, 0, 0}, 255))
	if len(f.puts()) != 1 {
		t.Fatal("first write expected")
	}
	f.mu.Lock()
	f.lights["1"]["name"] = "Chambre"
	f.mu.Unlock()
	clock = clock.Add(DefaultRefreshEvery + time.Second)
	if err := d.Apply(ctx, devGeneral([3]int{0, 255, 0}, 255)); err != nil {
		t.Fatal(err)
	}
	if len(f.puts()) != 1 {
		t.Fatalf("renamed light must never be written, got %d PUTs", len(f.puts()))
	}
	if st := d.Status(); st.LightsOK != 0 || st.Lights[0].LastError != "not found" {
		t.Fatalf("status: %+v", st)
	}
}

func TestDevRefusedAndUnreachableAreDistinct(t *testing.T) {
	f := newDevBridge(t, "BuzzHue1")
	sink := &devLogSink{}
	d, err := New(Config{BridgeIP: f.srv.URL, APIKey: "wrong", Lights: []LightSpec{{Name: "BuzzHue1"}}, Logger: sink.logf})
	if err != nil {
		t.Fatal(err)
	}
	err = d.Apply(context.Background(), devGeneral([3]int{255, 0, 0}, 255))
	if !errors.Is(err, ErrRefused) || errors.Is(err, ErrUnreachable) {
		t.Fatalf("wrong key must be refused, got %v", err)
	}
	if st := d.Status(); st.State != StateRefused {
		t.Fatalf("status: %+v", st)
	}
	before := len(f.paths())
	for i := 0; i < 5; i++ {
		if err := d.Apply(context.Background(), devGeneral([3]int{0, 255, 0}, 255)); !errors.Is(err, ErrRefused) {
			t.Fatal(err)
		}
	}
	if len(f.paths()) != before {
		t.Fatalf("refused must not retry: %d new requests", len(f.paths())-before)
	}
	if sink.count() != 1 {
		t.Errorf("exactly one log line for refused, got %v", sink.lines)
	}
	f.mu.Lock()
	f.key = "wrong"
	f.mu.Unlock()
	if err := d.RefreshInventory(context.Background()); err != nil {
		t.Fatalf("refresh after fixing the key: %v", err)
	}
	if st := d.Status(); st.State != StateOK {
		t.Fatalf("status after refresh: %+v", st)
	}

	f2 := newDevBridge(t, "BuzzHue1")
	url := f2.srv.URL
	f2.srv.Close()
	sink2 := &devLogSink{}
	clock := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	d2, err := New(Config{BridgeIP: url, APIKey: "k", Lights: []LightSpec{{Name: "BuzzHue1"}}, Logger: sink2.logf, Timeout: 500 * time.Millisecond, Now: func() time.Time { return clock }})
	if err != nil {
		t.Fatal(err)
	}
	err = d2.Apply(context.Background(), devGeneral([3]int{255, 0, 0}, 255))
	if !errors.Is(err, ErrUnreachable) || errors.Is(err, ErrRefused) {
		t.Fatalf("dead bridge must be unreachable, got %v", err)
	}
	st := d2.Status()
	if st.State != StateUnreachable || st.NextRetry.Sub(clock) != backoffMin {
		t.Fatalf("status: %+v", st)
	}
	start := time.Now()
	for i := 0; i < 50; i++ {
		if err := d2.Apply(context.Background(), devGeneral([3]int{0, 0, 255}, 255)); !errors.Is(err, ErrUnreachable) {
			t.Fatal(err)
		}
	}
	if el := time.Since(start); el > 50*time.Millisecond {
		t.Errorf("50 Applies during backoff took %s — must be near-instant", el)
	}
	if sink2.count() != 1 {
		t.Errorf("a dead bridge must log exactly once, got %v", sink2.lines)
	}
	waits := []time.Duration{}
	for i := 0; i < 8; i++ {
		clock = d2.Status().NextRetry.Add(time.Millisecond)
		_ = d2.Apply(context.Background(), devGeneral([3]int{0, 0, 255}, 255))
		waits = append(waits, d2.Status().NextRetry.Sub(clock))
	}
	if waits[0] != 2*time.Second || waits[1] != 4*time.Second || waits[len(waits)-1] != backoffMax {
		t.Errorf("backoff sequence = %v", waits)
	}
	if sink2.count() != 1 {
		t.Errorf("still exactly one log line after %d failed retries, got %v", len(waits), sink2.lines)
	}
}

func TestDevPerLightFailureDoesNotAbortOthers(t *testing.T) {
	f := newDevBridge(t, "A", "B", "C")
	f.offLights["2"] = true
	d, _ := newDevDriver(t, f, LightSpec{Name: "A"}, LightSpec{Name: "B"}, LightSpec{Name: "C"})
	if err := d.Apply(context.Background(), devGeneral([3]int{255, 0, 0}, 255)); err != nil {
		t.Fatalf("a per-light error must not fail Apply: %v", err)
	}
	if n := len(f.puts()); n != 3 {
		t.Fatalf("all three lights must be attempted, got %d", n)
	}
	st := d.Status()
	if st.State != StateOK || st.LightsOK != 2 || st.Stats.WriteErrors != 1 {
		t.Fatalf("status: %+v", st)
	}
	_ = d.Apply(context.Background(), devGeneral([3]int{255, 0, 0}, 255))
	puts := f.puts()
	if len(puts) != 4 || puts[3].path != "/api/k/lights/2/state" {
		t.Fatalf("only the failed light must be retried: %+v", puts)
	}
}

func TestDevTeamLightsFollowTheirZoneElseGeneral(t *testing.T) {
	f := newDevBridge(t, "G", "R", "B")
	d, _ := newDevDriver(t, f, LightSpec{Name: "G"}, LightSpec{Name: "R", Role: RoleTeam, Team: "Rouges"}, LightSpec{Name: "B", Role: RoleTeam, Team: "Bleus"})
	st := lighting.State{Zones: []lighting.ZoneState{
		{Zone: lighting.ZoneGeneral, Color: [3]int{255, 255, 255}, Intensity: 100},
		{Zone: "Rouges", Color: [3]int{255, 0, 0}, Intensity: 255},
	}}
	if err := d.Apply(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, p := range f.puts() {
		byPath[p.path] = p.body
	}
	if len(byPath) != 3 {
		t.Fatalf("3 writes expected: %+v", byPath)
	}
	if !strings.Contains(byPath["/api/k/lights/2/state"], `"bri":254`) {
		t.Errorf("R: %s", byPath["/api/k/lights/2/state"])
	}
	if !strings.Contains(byPath["/api/k/lights/3/state"], `"bri":100`) {
		t.Errorf("B: %s", byPath["/api/k/lights/3/state"])
	}
	f.mu.Lock()
	f.requests = nil
	f.mu.Unlock()
	if err := d.Apply(context.Background(), lighting.State{Zones: []lighting.ZoneState{{Zone: "Verts", Color: [3]int{0, 255, 0}, Intensity: 255}}}); err != nil {
		t.Fatal(err)
	}
	if len(f.puts()) != 0 {
		t.Errorf("no configured light for zone Verts → no write, got %+v", f.puts())
	}
}

func TestDevBridgeIdentityAndRediscovery(t *testing.T) {
	right := newDevBridge(t, "BuzzHue1")
	wrong := newDevBridge(t, "BuzzHue1")
	wrong.bridgeID = "deadbeef00000000"
	sink := &devLogSink{}
	d, err := New(Config{BridgeIP: wrong.srv.URL, BridgeID: right.bridgeID, APIKey: "k", Lights: []LightSpec{{Name: "BuzzHue1"}}, Logger: sink.logf,
		FindBridge: func(_ context.Context, id string, _ time.Duration) (Bridge, bool, error) {
			if strings.EqualFold(id, right.bridgeID) {
				return Bridge{IP: right.srv.URL, ID: id}, true, nil
			}
			return Bridge{}, false, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(context.Background(), devGeneral([3]int{255, 0, 0}, 255)); err != nil {
		t.Fatal(err)
	}
	if len(wrong.puts()) != 0 || len(right.puts()) != 1 {
		t.Fatalf("write must go to the right bridge: wrong=%d right=%d", len(wrong.puts()), len(right.puts()))
	}
	if st := d.Status(); st.BridgeIP != right.srv.URL || st.BridgeID != right.bridgeID {
		t.Fatalf("status: %+v", st)
	}
	moved := false
	for _, l := range sink.lines {
		if strings.Contains(l, "moved from") {
			moved = true
		}
	}
	if !moved {
		t.Errorf("IP change must be logged once: %v", sink.lines)
	}

	d2, err := New(Config{BridgeID: right.bridgeID, APIKey: "k", Lights: []LightSpec{{Name: "BuzzHue1"}},
		FindBridge: func(context.Context, string, time.Duration) (Bridge, bool, error) {
			return Bridge{IP: right.srv.URL, ID: right.bridgeID}, true, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	if err := d2.Apply(context.Background(), devGeneral([3]int{0, 255, 0}, 255)); err != nil {
		t.Fatal(err)
	}
	if len(right.puts()) != 2 {
		t.Fatalf("id-only config must write after discovery, got %d PUTs", len(right.puts()))
	}
}

func TestDevInventoryAndTestFlashRestore(t *testing.T) {
	f := newDevBridge(t, "Salon", "BuzzHue1")
	d, _ := newDevDriver(t, f, LightSpec{Name: "BuzzHue1"})
	inv, err := d.Inventory(context.Background())
	if err != nil || len(inv) != 2 || inv[0].ID != "1" || inv[1].Name != "BuzzHue1" {
		t.Fatalf("inventory: %+v %v", inv, err)
	}
	_ = d.Apply(context.Background(), devGeneral([3]int{0, 0, 255}, 120))
	f.mu.Lock()
	f.requests = nil
	f.mu.Unlock()
	slept := time.Duration(0)
	if err := d.TestFlash(context.Background(), "BuzzHue1", 300*time.Millisecond, func(d time.Duration) { slept = d }); err != nil {
		t.Fatal(err)
	}
	puts := f.puts()
	if len(puts) != 2 || slept != 300*time.Millisecond {
		t.Fatalf("flash = %d PUTs, slept %s", len(puts), slept)
	}
	if !strings.Contains(puts[0].body, `"bri":254`) || !strings.Contains(puts[1].body, `"bri":120`) {
		t.Errorf("flash then restore expected: %s | %s", puts[0].body, puts[1].body)
	}
	if err := d.TestFlash(context.Background(), "Salon", 0, func(time.Duration) {}); err == nil {
		t.Error("flashing a light that is not configured must be refused")
	}
	for _, p := range f.puts() {
		if p.path == "/api/k/lights/1/state" {
			t.Errorf("Salon (not configured) was written: %+v", p)
		}
	}
}

func TestDevOffPayloadAndClassify(t *testing.T) {
	a := desired(lighting.ZoneState{Zone: "general", Color: [3]int{10, 20, 30}, Intensity: 0})
	if a.on {
		t.Error("intensity 0 must be off")
	}
	b, _ := json.Marshal(a.toV1())
	if string(b) != `{"on":false,"transitiontime":0}` {
		t.Errorf("off payload = %s", b)
	}
	if !errors.Is(classify(hueError{Type: 1}), ErrRefused) || !errors.Is(classify(hueError{Type: 101}), ErrRefused) {
		t.Error("hue 1/101 must be refused")
	}
	if e := classify(hueError{Type: 201}); errors.Is(e, ErrRefused) || errors.Is(e, ErrUnreachable) {
		t.Error("hue 201 (per-light) must be neither refused nor unreachable")
	}
	if !errors.Is(classify(httpStatusError{Code: 401}), ErrRefused) || !errors.Is(classify(httpStatusError{Code: 503}), ErrUnreachable) {
		t.Error("HTTP 401 → refused, 503 → unreachable")
	}
	if !errors.Is(classify(errors.New("dial tcp: connection refused")), ErrUnreachable) {
		t.Error("transport errors → unreachable")
	}
	if b, _ := bridgeBase("192.168.1.10", true); b != "https://192.168.1.10" {
		t.Errorf("https base = %s", b)
	}
	if _, err := bridgeBase("ftp://x", false); err == nil {
		t.Error("ftp must be rejected")
	}
}

func TestDevRegisterFlow(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		calls++
		if calls < 2 {
			_, _ = w.Write([]byte(`[{"error":{"type":101,"address":"","description":"link button not pressed"}}]`))
			return
		}
		_, _ = w.Write([]byte(`[{"success":{"username":"key-xyz"}}]`))
	}))
	defer srv.Close()
	_, err := Register(context.Background(), srv.URL, false, "buzzmaster#test", 0)
	if !errors.Is(err, ErrLinkButtonNotPressed) || !errors.Is(err, ErrRefused) {
		t.Fatalf("first attempt: %v", err)
	}
	key, err := Register(context.Background(), srv.URL, false, "buzzmaster#test", 0)
	if err != nil || key != "key-xyz" {
		t.Fatalf("second attempt: %q %v", key, err)
	}
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := dead.URL
	dead.Close()
	if _, err := Register(context.Background(), url, false, "x", 0); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("dead bridge: %v", err)
	}
}

func TestDevSSDPParsing(t *testing.T) {
	resp := "HTTP/1.1 200 OK\r\nLOCATION: http://192.168.1.42:80/description.xml\r\nSERVER: Hue/1.0 UPnP/1.0 IpBridge/1.67.0\r\nhue-bridgeid: 001788FFFE123456\r\n\r\n"
	b, ok := parseSSDPResponse(resp)
	if !ok || b.IP != "192.168.1.42" || b.ID != "001788fffe123456" {
		t.Errorf("ssdp = %+v %v", b, ok)
	}
	if _, ok := parseSSDPResponse("HTTP/1.1 200 OK\r\nSERVER: Sonos\r\nLOCATION: http://10.0.0.5/x\r\n\r\n"); ok {
		t.Error("non-Hue responder must be ignored")
	}
}

// ---------------------------------------------------------------------------
// Contract §8 measurements against the fake bridge (published in the report).
// ---------------------------------------------------------------------------

type devSpreadResult struct {
	N            int     `json:"n_lights"`
	LatencyMs    float64 `json:"simulated_write_latency_ms"`
	SpreadMs     float64 `json:"spread_first_to_last_ms"`
	ApplyTotalMs float64 `json:"apply_total_ms"`
}

func TestDevMeasureSpreadForN(t *testing.T) {
	if testing.Short() {
		t.Skip("timing measurement")
	}
	const lat = 40 * time.Millisecond
	var results []devSpreadResult
	for _, n := range []int{2, 4, 6} {
		names := make([]string, n)
		cfgs := make([]LightSpec, n)
		for i := range names {
			names[i] = fmt.Sprintf("L%d", i+1)
			cfgs[i] = LightSpec{Name: names[i]}
		}
		f := newDevBridge(t, names...)
		f.latency = lat
		d, _ := newDevDriver(t, f, cfgs...)
		_ = d.Apply(context.Background(), devGeneral([3]int{255, 255, 255}, 100))
		f.mu.Lock()
		f.requests = nil
		f.mu.Unlock()
		start := time.Now()
		if err := d.Apply(context.Background(), devGeneral([3]int{255, 0, 0}, 255)); err != nil {
			t.Fatal(err)
		}
		total := time.Since(start)
		puts := f.puts()
		if len(puts) != n {
			t.Fatalf("N=%d: %d PUTs", n, len(puts))
		}
		sort.Slice(puts, func(i, j int) bool { return puts[i].at.Before(puts[j].at) })
		spread := puts[len(puts)-1].at.Sub(puts[0].at)
		results = append(results, devSpreadResult{N: n, LatencyMs: devMs(lat), SpreadMs: devMs(spread), ApplyTotalMs: devMs(total)})
	}
	b, _ := json.MarshalIndent(results, "", "  ")
	t.Logf("SPREAD_MEASUREMENT %s", b)
	for _, r := range results {
		if r.SpreadMs < float64(r.N-1)*devMs(lat)*0.8 {
			t.Errorf("N=%d spread %.0f ms implausible", r.N, r.SpreadMs)
		}
	}
}

type devBurstResult struct {
	Events       int     `json:"events"`
	WindowMs     float64 `json:"window_ms"`
	Lights       int     `json:"n_lights"`
	Applies      int     `json:"applies"`
	Writes       int     `json:"writes"`
	WritesPerSec float64 `json:"writes_per_sec"`
	FailedWrites int     `json:"failed_writes"`
}

func TestDevMeasureRafaleBurstThroughWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("timing measurement")
	}
	const events, window = 40, 2 * time.Second
	var results []devBurstResult
	for _, n := range []int{1, 2, 6} {
		names := make([]string, n)
		cfgs := make([]LightSpec, n)
		for i := range names {
			names[i] = fmt.Sprintf("L%d", i+1)
			cfgs[i] = LightSpec{Name: names[i]}
		}
		f := newDevBridge(t, names...)
		f.latency = 40 * time.Millisecond
		d, _ := newDevDriver(t, f, cfgs...)
		var mu sync.Mutex
		flip := 0
		derive := func() lighting.Event {
			mu.Lock()
			defer mu.Unlock()
			if flip%2 == 0 {
				return lighting.Event{Kind: lighting.KindScore, Teams: []string{"A"}}
			}
			return lighting.Event{Kind: lighting.KindRunning}
		}
		scene := func(ev lighting.Event) lighting.State {
			if ev.Kind == lighting.KindScore {
				return devGeneral([3]int{255, 26, 26}, 255)
			}
			return devGeneral([3]int{40, 90, 255}, 160)
		}
		w := lighting.NewWriter(lighting.Config{Driver: d, Derive: derive, Scene: scene, MinInterval: RecommendedMinInterval})
		ctx, cancel := context.WithCancel(context.Background())
		go w.Start(ctx)
		start := time.Now()
		for i := 0; i < events; i++ {
			mu.Lock()
			flip++
			mu.Unlock()
			w.NotifyState()
			time.Sleep(window / events)
		}
		time.Sleep(RecommendedMinInterval + 200*time.Millisecond)
		elapsed := time.Since(start)
		cancel()
		st := d.Status()
		res := devBurstResult{Events: events, WindowMs: devMs(window), Lights: n, Applies: st.Stats.Applies, Writes: st.Stats.Writes,
			WritesPerSec: float64(st.Stats.Writes) / elapsed.Seconds(), FailedWrites: st.Stats.WriteErrors}
		results = append(results, res)
		if res.FailedWrites != 0 {
			t.Errorf("N=%d: %d failed writes", n, res.FailedWrites)
		}
		maxApplies := int(window/RecommendedMinInterval) + 2
		if res.Applies > maxApplies {
			t.Errorf("N=%d: %d applies for %d events, writer pacing broken (max %d)", n, res.Applies, events, maxApplies)
		}
	}
	b, _ := json.MarshalIndent(results, "", "  ")
	t.Logf("BURST_MEASUREMENT %s", b)
}

// ---------------------------------------------------------------------------
// Review #206/#207 (CRITIQUE): Apply (writer goroutine) runs concurrently with
// Inventory / RefreshInventory / TestFlash / Status / Close (HTTP goroutines)
// on the SAME live driver, including re-discovery by id in the middle.
// The race detector is the oracle: `go test -race` fails on any unsynchronised
// access to client/base/bridgeInfo.
// ---------------------------------------------------------------------------

func TestDevConcurrentApplyInventoryTestFlashWithRediscovery(t *testing.T) {
	a := newDevBridge(t, "BuzzHue1", "BuzzHue2")
	b := newDevBridge(t, "BuzzHue1", "BuzzHue2")
	var target atomic.Pointer[devBridge]
	target.Store(a)
	sink := &devLogSink{}
	// Known by id only (contract §4.1): the first contact re-discovers, and a
	// later unreachable answer re-discovers again — to bridge b.
	d, err := New(Config{BridgeID: a.bridgeID, APIKey: a.key, Logger: sink.logf, DiscoverTimeout: 10 * time.Millisecond,
		Lights: []LightSpec{{Name: "BuzzHue1"}, {Name: "BuzzHue2", Role: RoleTeam, Team: "Rouge"}},
		FindBridge: func(context.Context, string, time.Duration) (Bridge, bool, error) {
			return Bridge{IP: target.Load().srv.URL, ID: a.bridgeID}, true, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	run := func(f func(i int)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
					f(i)
				}
			}
		}()
	}
	run(func(i int) { // the writer
		st := devGeneral([3]int{255, 0, 0}, 255)
		if i%2 == 1 {
			st = devGeneral([3]int{0, 0, 255}, 128)
		}
		st.Zones = append(st.Zones, lighting.ZoneState{Zone: "Rouge", Color: [3]int{255, 0, 0}, Intensity: 200})
		_ = d.Apply(ctx, st)
	})
	run(func(int) { _, _ = d.Inventory(ctx) })                                     // GET /lights
	run(func(int) { _ = d.RefreshInventory(ctx) })                                 // register handler
	run(func(int) { _ = d.TestFlash(ctx, "BuzzHue1", 0, func(time.Duration) {}) }) // test handler
	run(func(int) { _ = d.Status() })                                              // status handler
	// Mid-run: bridge a dies, the id is now answered by b (DHCP move).
	time.Sleep(60 * time.Millisecond)
	target.Store(b)
	a.srv.Close()
	time.Sleep(120 * time.Millisecond)
	close(stop)
	wg.Wait()

	// After the move: the driver may sit in its backoff (contract §5.5 — the
	// loops above accumulated failures on the dead bridge a). The user
	// gesture (RefreshInventory) re-discovers by id at once, lifting the
	// backoff; the writer then follows.
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatalf("RefreshInventory after the move: %v (status %+v)", err, d.Status())
	}
	if err := d.Apply(ctx, devGeneral([3]int{0, 255, 0}, 255)); err != nil {
		t.Fatalf("Apply after the move: %v (status %+v)", err, d.Status())
	}
	if st := d.Status(); st.State != StateOK || st.BridgeIP != b.srv.URL {
		t.Fatalf("driver must follow the bridge to %s, status %+v", b.srv.URL, st)
	}
	if len(b.puts()) == 0 {
		t.Fatal("no write reached bridge b")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(ctx, devGeneral([3]int{0, 255, 0}, 255)); err == nil {
		t.Fatal("Apply after Close must fail")
	}
}

// A scene write can never slip between the flash and its restore: while
// TestFlash holds (sleep hook blocked), Apply from the writer goroutine waits.
func TestDevApplyWaitsBehindTestFlashRestore(t *testing.T) {
	f := newDevBridge(t, "BuzzHue1")
	d, _ := newDevDriver(t, f, LightSpec{Name: "BuzzHue1"})
	ctx := context.Background()
	if err := d.Apply(ctx, devGeneral([3]int{255, 0, 0}, 255)); err != nil {
		t.Fatal(err)
	}
	holding := make(chan struct{})
	release := make(chan struct{})
	flashDone := make(chan error, 1)
	go func() {
		flashDone <- d.TestFlash(ctx, "BuzzHue1", time.Millisecond, func(time.Duration) {
			close(holding)
			<-release
		})
	}()
	<-holding
	applyDone := make(chan error, 1)
	go func() { applyDone <- d.Apply(ctx, devGeneral([3]int{0, 0, 255}, 255)) }()
	select {
	case err := <-applyDone:
		t.Fatalf("Apply completed during the flash hold (err=%v) — the scene overwrote the flash and the restore will undo it", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-flashDone; err != nil {
		t.Fatal(err)
	}
	if err := <-applyDone; err != nil {
		t.Fatal(err)
	}
	puts := f.puts()
	if len(puts) != 4 { // scene, flash on, restore, scene
		t.Fatalf("want 4 PUTs (scene, flash, restore, scene), got %d", len(puts))
	}
	if !strings.Contains(puts[3].body, `"xy"`) || strings.Contains(puts[2].body, `"xy":[0.`) && puts[2].body == puts[3].body {
		t.Fatalf("last PUT must be the writer's scene after the restore: %+v", puts)
	}
}
