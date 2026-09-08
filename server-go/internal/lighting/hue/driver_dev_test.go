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
	mu         sync.Mutex
	lights     map[string]map[string]any // id → light object
	groups     map[string]*devGroup      // id → group object (Batch B, §5.8)
	nextGroup  int
	bridgeID   string
	key        string
	latency    time.Duration
	offLights  map[string]bool // ids answering error 201 on writes
	groupFails bool            // Batch B: every /groups request answers 404 (simulates a bridge/firmware that never serves groups — rule 5 repli test)
	requests   []devRecorded
	srv        *httptest.Server
}

// devGroup is a fake bridge's BuzzMaster-style LightGroup.
type devGroup struct {
	name   string
	lights []string
}

type devRecorded struct {
	at     time.Time
	method string
	path   string
	body   string
}

func newDevBridge(t *testing.T, names ...string) *devBridge {
	t.Helper()
	f := &devBridge{lights: map[string]map[string]any{}, groups: map[string]*devGroup{}, bridgeID: "fffe0000deadbeef", key: "k", offLights: map[string]bool{}}
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

	// --- Batch B (§5.8): the same handful of /groups operations a real Hue
	// bridge answers for a LightGroup — list/create, read one, correct/delete
	// one by id, and write the group's own state (applies to every member,
	// exactly like the real bridge). f.groupFails simulates rule 5's trigger
	// (a bridge that never serves groups at all) with a plain 404 on every one.
	case len(parts) == 3 && parts[2] == "groups" && r.Method == "GET" && !f.groupFails:
		out := map[string]any{}
		for id, g := range f.groups {
			out[id] = map[string]any{"name": g.name, "type": "LightGroup", "lights": g.lights}
		}
		_ = json.NewEncoder(w).Encode(out)
	case len(parts) == 3 && parts[2] == "groups" && r.Method == "POST" && !f.groupFails:
		var req struct {
			Name   string   `json:"name"`
			Lights []string `json:"lights"`
			Type   string   `json:"type"`
		}
		_ = json.Unmarshal([]byte(body.String()), &req)
		if len(req.Lights) == 0 {
			_, _ = w.Write([]byte(`[{"error":{"type":7,"address":"/groups","description":"invalid value, [], for parameter, lights"}}]`))
			return
		}
		f.nextGroup++
		id := fmt.Sprint(f.nextGroup)
		f.groups[id] = &devGroup{name: req.Name, lights: append([]string(nil), req.Lights...)}
		_, _ = w.Write([]byte(`[{"success":{"id":"` + id + `"}}]`))
	case len(parts) == 4 && parts[2] == "groups" && r.Method == "PUT" && !f.groupFails:
		id := parts[3]
		g, ok := f.groups[id]
		if !ok {
			_, _ = w.Write([]byte(`[{"error":{"type":3,"address":"/groups/` + id + `","description":"resource not available"}}]`))
			return
		}
		var req struct {
			Lights []string `json:"lights"`
		}
		_ = json.Unmarshal([]byte(body.String()), &req)
		if req.Lights != nil {
			g.lights = append([]string(nil), req.Lights...)
		}
		_, _ = w.Write([]byte(`[{"success":{"/groups/` + id + `/lights":` + string(mustJSON(g.lights)) + `}}]`))
	case len(parts) == 4 && parts[2] == "groups" && r.Method == "DELETE" && !f.groupFails:
		id := parts[3]
		if _, ok := f.groups[id]; !ok {
			_, _ = w.Write([]byte(`[{"error":{"type":3,"address":"/groups/` + id + `","description":"resource not available"}}]`))
			return
		}
		delete(f.groups, id)
		_, _ = w.Write([]byte(`[{"success":"/groups/` + id + ` deleted"}]`))
	case len(parts) == 5 && parts[2] == "groups" && parts[4] == "action" && r.Method == "PUT" && !f.groupFails:
		id := parts[3]
		g, ok := f.groups[id]
		if !ok {
			_, _ = w.Write([]byte(`[{"error":{"type":3,"address":"/groups/` + id + `/action","description":"resource not available"}}]`))
			return
		}
		var st map[string]any
		_ = json.Unmarshal([]byte(body.String()), &st)
		for _, lid := range g.lights {
			l, ok := f.lights[lid]
			if !ok {
				continue
			}
			state := l["state"].(map[string]any)
			for k, v := range st {
				if k != "transitiontime" {
					state[k] = v
				}
			}
		}
		_, _ = w.Write([]byte(`[{"success":{"/groups/` + id + `/action/on":true}}]`))

	default:
		w.WriteHeader(404)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
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
	// Batch B (§5.8): devBridge (below) predates Hue groups and answers every
	// /groups request with a plain 404 — faithful to no real endpoint being
	// registered, but noisy for the ~40 pre-existing tests in this file that
	// assert exact PUT/log counts and never intended to exercise groups at
	// all. Group reconciliation/writes are therefore off by default here;
	// the dedicated group tests below (TestDevGroups*, TestDevApplyWritesVia*,
	// TestDevMeasureGroupGainAtN30) explicitly flip this back on and use
	// newDevBridgeWithGroups, which does answer /groups for real.
	d.disableGroupsForTest = true
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
	// 2 lines: "Hue lights resolved: ..." and "Hue bridge ok". (The round-3
	// QUALIF bug-1 diagnostic instrumentation that briefly lived here — see
	// TestFlash's own doc comment for the round-5 fix that made it
	// unnecessary — has been removed.)
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

// TestDevBugfixQualif_WhitespacePaddedBridgeNameStillResolves reproduces the
// real QUALIF v10.0.0.8 regression reported by the user against an actual
// Hue bridge: a light ("salon gauche") visible and selectable at
// association time (Inventory/GET /api/lighting/lights) failed a later
// POST /api/lighting/test with "hue: no resolved light matches "salon
// gauche"" — even though it was correctly configured. Root cause: New()
// trims every CONFIGURED light name (strings.TrimSpace), but resolve()
// used to compare against the bridge's OWN light name AS RETURNED, never
// trimmed. A real bridge light named with incidental leading/trailing
// whitespace (the Hue app does not prevent this) could therefore never
// match its own (trimmed) configured counterpart — silently "not found",
// not even logged as a whitespace mismatch.
func TestDevBugfixQualif_WhitespacePaddedBridgeNameStillResolves(t *testing.T) {
	f := newDevBridge(t, "salon gauche ") // bridge-side name: trailing space, exactly the reported case
	d, _ := newDevDriver(t, f, LightSpec{Name: "salon gauche"})

	if err := d.Apply(context.Background(), devGeneral([3]int{255, 0, 0}, 255)); err != nil {
		t.Fatalf("Apply must resolve a bridge name that only differs by whitespace: %v", err)
	}
	if len(f.puts()) != 1 {
		t.Fatalf("expected 1 write once resolved, got %d: %+v", len(f.puts()), f.puts())
	}
	if st := d.Status(); st.LightsOK != 1 || st.Lights[0].LastError != "" {
		t.Fatalf("light must resolve cleanly, not 'not found': %+v", st.Lights)
	}

	// The exact regression: POST /api/lighting/test → TestFlash("salon gauche").
	if err := d.TestFlash(context.Background(), "salon gauche", time.Millisecond, func(time.Duration) {}); err != nil {
		t.Fatalf(`TestFlash must resolve the whitespace-padded bridge name — this is the exact QUALIF regression ("hue: no resolved light matches"): %v`, err)
	}
}

// TestDevNormalizeLightName pins the exact set of invisible characters a
// plain strings.TrimSpace does NOT catch (round 2's own discovery — see
// NormalizeLightName's doc comment): zero-width space, BOM/ZWNBSP, soft
// hyphen, and a stray NUL, none of them "space" by unicode.IsSpace's
// definition and/or not confined to the string's edges. Ordinary ASCII
// whitespace at the edges must still be trimmed, and meaningful characters
// (accents, an internal literal space) must survive untouched.
func TestDevNormalizeLightName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain, no change needed", "Salon", "Salon"},
		{"internal space preserved", "salon gauche", "salon gauche"},
		{"accents preserved", "Éclairage Été", "Éclairage Été"},
		{"ASCII edge whitespace trimmed", "  Salon\t\n", "Salon"},
		{"trailing ASCII space (round 1's own case)", "salon gauche ", "salon gauche"},
		{"trailing NBSP", "salon gauche" + string(rune(0x00A0)), "salon gauche"},
		{"trailing zero-width space (round 2 — TrimSpace misses this)", "salon gauche" + string(rune(0x200B)), "salon gauche"},
		{"leading BOM/ZWNBSP (round 2 — TrimSpace misses this)", string(rune(0xFEFF)) + "salon gauche", "salon gauche"},
		{"internal zero-width space (not just an edge)", "salon" + string(rune(0x200B)) + "gauche", "salongauche"},
		{"soft hyphen anywhere (round 2 — TrimSpace misses this)", "sa" + string(rune(0x00AD)) + "lon", "salon"},
		{"stray NUL (round 2 — TrimSpace misses this)", "salon" + string(rune(0x0000)) + "gauche", "salongauche"},
		{"empty after stripping", string(rune(0x200B)) + string(rune(0x00A0)), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeLightName(tt.in); got != tt.want {
				t.Errorf("NormalizeLightName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestDevBugfixQualifRound2_ZeroWidthSpaceBridgeNameStillResolves is round
// 2's own regression: a plain edge-space trim (round 1) has NO effect on a
// bridge name carrying an invisible zero-width space instead of (here, in
// addition to) a plain one — exactly the class of case the user's
// persisting report pointed at. The bridge keeps returning the trailing
// ZWSP on every fetch (a real device's own quirk, not a one-off
// keystroke); NormalizeLightName is what makes it irrelevant, wherever it
// came from and on every subsequent resolution.
func TestDevBugfixQualifRound2_ZeroWidthSpaceBridgeNameStillResolves(t *testing.T) {
	zwsp := string(rune(0x200B))
	f := newDevBridge(t, "salon gauche"+zwsp) // invisible trailing character, real space preserved
	d, _ := newDevDriver(t, f, LightSpec{Name: "salon gauche"})

	if err := d.Apply(context.Background(), devGeneral([3]int{255, 0, 0}, 255)); err != nil {
		t.Fatalf("Apply must resolve a bridge name differing only by a trailing invisible zero-width space: %v", err)
	}
	if err := d.TestFlash(context.Background(), "salon gauche", time.Millisecond, func(time.Duration) {}); err != nil {
		t.Fatalf("TestFlash must resolve it too — the exact QUALIF round-2 regression: %v", err)
	}
}

// TestDevBugfixQualifRound5_TestFlashResolvesLiveUnconfiguredLight is the
// REAL round-5 fix (see TestFlash's own doc comment): "Tester" must work
// on ANY light the bridge reports LIVE, configured or not — that is the
// whole point of the feature, identifying a physical bulb BEFORE deciding
// where to assign it. Confirmed by the user after 4 rounds diagnosing the
// wrong layer (name normalisation, persistence, startup contact — all
// real, correctly-fixed bugs, none of them this one: d.cfg.Lights was
// simply empty, by design of the feature, not by any of those bugs).
func TestDevBugfixQualifRound5_TestFlashResolvesLiveUnconfiguredLight(t *testing.T) {
	f := newDevBridge(t, "BuzzHue1") // on the bridge, but NEVER saved to config
	d, _ := newDevDriver(t, f)       // zero configured lights — exactly the reported state

	if err := d.TestFlash(context.Background(), "BuzzHue1", time.Millisecond, func(time.Duration) {}); err != nil {
		t.Fatalf("TestFlash must resolve a light present on the bridge even when unconfigured — the exact round-5 regression: %v", err)
	}

	// Apply() (actual gameplay writes) must stay UNCHANGED by this fix: an
	// unconfigured light is never touched during a real game, only by this
	// explicit, single, user-initiated test flash.
	if err := d.Apply(context.Background(), devGeneral([3]int{255, 0, 0}, 255)); err != nil {
		t.Fatalf("Apply on an empty configuration must be a harmless no-op, not an error: %v", err)
	}
	if len(f.puts()) != 2 { // the test flash's own 2 writes (on, then restore) — Apply added none
		t.Fatalf("Apply must never write to an unconfigured light — got %d PUT(s) total, want 2 (test flash only): %+v", len(f.puts()), f.puts())
	}
}

// TestDevBugfixQualifRound5_TestFlashAllStillMeansConfiguredOnly pins the
// OTHER half of the round-5 fix: name=="" ("all") still means "all
// configured/selected lights" (contract hue-bridge.md §7), never
// "everything the bridge reports" — a live-only light must NOT be flashed
// by an unnamed test.
func TestDevBugfixQualifRound5_TestFlashAllStillMeansConfiguredOnly(t *testing.T) {
	f := newDevBridge(t, "Configuree", "NonConfiguree")
	d, _ := newDevDriver(t, f, LightSpec{Name: "Configuree"})

	if err := d.TestFlash(context.Background(), "", time.Millisecond, func(time.Duration) {}); err != nil {
		t.Fatalf(`TestFlash("") must succeed against the one configured light: %v`, err)
	}
	if len(f.puts()) != 2 { // exactly the one configured light: on, then restore
		t.Fatalf(`TestFlash("") must touch ONLY the configured light(s), got %d PUT(s): %+v`, len(f.puts()), f.puts())
	}
}

// TestDevSetLightDirect_ResolvesLiveUnconfiguredLight is P2a's own version
// of the round-5 lesson (contract §7, POST /api/lighting/preview,
// planner-v10-general-theme-toggle-20260908-114420.md §Partie 2): checking
// a bulb BEFORE it is saved to configuration must still light it up — the
// exact case that cost 5 QUALIF rounds when TestFlash got this wrong.
func TestDevSetLightDirect_ResolvesLiveUnconfiguredLight(t *testing.T) {
	f := newDevBridge(t, "BuzzHue1") // on the bridge, but NEVER saved to config
	d, _ := newDevDriver(t, f)       // zero configured lights — exactly the reported state

	if err := d.SetLightDirect(context.Background(), "BuzzHue1", true); err != nil {
		t.Fatalf("SetLightDirect(on) must resolve a light present on the bridge even when unconfigured: %v", err)
	}
	puts := f.puts()
	if len(puts) != 1 || !strings.Contains(puts[0].body, `"on":true`) || !strings.Contains(puts[0].body, `"bri":254`) {
		t.Fatalf("expected one PUT turning the light full white, got %+v", puts)
	}

	if err := d.SetLightDirect(context.Background(), "BuzzHue1", false); err != nil {
		t.Fatalf("SetLightDirect(off): %v", err)
	}
	puts = f.puts()
	if len(puts) != 2 || !strings.Contains(puts[1].body, `"on":false`) || strings.Contains(puts[1].body, `"bri"`) {
		t.Fatalf("expected a second PUT turning the light off (no bri), got %+v", puts)
	}
}

// TestDevSetLightDirect_UnknownNameIsRefused mirrors resolve()'s own "0 ou
// >1 correspondance ⇒ refus" rule (contract §4.2) for the live-inventory
// path: a name matching nothing on the bridge must error, never silently
// succeed or guess.
func TestDevSetLightDirect_UnknownNameIsRefused(t *testing.T) {
	f := newDevBridge(t, "Salon")
	d, _ := newDevDriver(t, f)

	if err := d.SetLightDirect(context.Background(), "Inconnue", true); err == nil {
		t.Fatal("SetLightDirect must refuse a name matching no light on the bridge")
	}
	if len(f.puts()) != 0 {
		t.Fatalf("no write must be attempted when resolution fails, got %+v", f.puts())
	}
}

// TestDevSetLightDirect_InvalidatesWriterDedupCache closes the §5.3 dedup
// pitfall for this out-of-band write path too (same pattern as TestFlash's
// own restore): a light already configured AND already at the state
// SetLightDirect is about to (re)write must still be re-asserted by the
// writer's NEXT ordinary Apply, not skipped as "unchanged" against a cache
// this out-of-band write never went through.
func TestDevSetLightDirect_InvalidatesWriterDedupCache(t *testing.T) {
	f := newDevBridge(t, "L1")
	d, _ := newDevDriver(t, f, LightSpec{Name: "L1"})
	ctx := context.Background()

	// Apply once so appliedState["L1"] is populated with a WHITE value that
	// happens to equal what SetLightDirect(on) will later write physically.
	if err := d.Apply(ctx, devGeneral([3]int{255, 255, 255}, 255)); err != nil {
		t.Fatal(err)
	}
	if len(f.puts()) != 1 {
		t.Fatalf("setup: %+v", f.puts())
	}

	// Out-of-band preview write, bypassing the writer entirely.
	if err := d.SetLightDirect(ctx, "L1", true); err != nil {
		t.Fatal(err)
	}
	if len(f.puts()) != 2 {
		t.Fatalf("SetLightDirect must have written, got %+v", f.puts())
	}

	// Same colour/intensity as before: if the cache were left untouched by
	// SetLightDirect, this would be (wrongly) skipped as "unchanged" — the
	// bug this test exists to catch.
	if err := d.Apply(ctx, devGeneral([3]int{255, 255, 255}, 255)); err != nil {
		t.Fatal(err)
	}
	if len(f.puts()) != 3 {
		t.Fatalf("the writer's next Apply must re-assert L1's state rather than trust a cache SetLightDirect bypassed, got %d PUTs: %+v", len(f.puts()), f.puts())
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
	// Round 5 (QUALIF bug 1, the real cause): a NAMED test must resolve
	// against the LIVE bridge inventory too, not only d.cfg.Lights — see
	// TestFlash's own doc comment. "Salon" is on the bridge (newDevBridge
	// above) but was never configured (only "BuzzHue1" was); testing it
	// must now SUCCEED, the opposite of this test's pre-round-5 assertion.
	if err := d.TestFlash(context.Background(), "Salon", 0, nil); err != nil {
		t.Fatalf("flashing a live-but-unconfigured light must succeed (round 5): %v", err)
	}
	sawSalon := false
	for _, p := range f.puts() {
		if p.path == "/api/k/lights/1/state" {
			sawSalon = true
		}
	}
	if !sawSalon {
		t.Error("Salon (id 1, live but unconfigured) must have been written by the named test flash")
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
	// N=30 added (Batch B, §8): the real installation size ("clause de sortie
	// DÉCLENCHÉE" — §2 was reopened precisely because 30 ≠ "quelques
	// unités"). Groups stay off here (newDevDriver's default) — this measures
	// the SAME no-optimization baseline as N=2,4,6, on purpose: it is the
	// "avant" half of TestDevMeasureGroupGainAtN30's "avant/après" pair.
	for _, n := range []int{2, 4, 6, 30} {
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

// TestDevOnReconnect_FiresOnceOnTransitionToOK pins contract lighting.md
// §10.3 ("au retour du pont, l'éclairage est recalculé et réappliqué") at
// the driver level: OnReconnect fires exactly when the status moves TO
// StateOK from something else, never on every successful call while
// already ok, and never while still refused/unreachable — the owner
// (cmd/server/ambiance.go) wires this straight to a.ambiance().NotifyState().
func TestDevOnReconnect_FiresOnceOnTransitionToOK(t *testing.T) {
	f := newDevBridge(t, "BuzzHue1")
	var reconnects int32
	d, err := New(Config{BridgeIP: f.srv.URL, APIKey: "wrong", Lights: []LightSpec{{Name: "BuzzHue1"}},
		OnReconnect: func() { atomic.AddInt32(&reconnects, 1) },
	})
	if err != nil {
		t.Fatal(err)
	}
	// Wrong key: refused, never ok — OnReconnect must not fire.
	if err := d.Apply(context.Background(), devGeneral([3]int{255, 0, 0}, 255)); !errors.Is(err, ErrRefused) {
		t.Fatalf("setup: %v", err)
	}
	if n := atomic.LoadInt32(&reconnects); n != 0 {
		t.Fatalf("refused must never fire OnReconnect, got %d", n)
	}

	// Fix the key (same pattern as TestDevRefusedAndUnreachableAreDistinct):
	// the transition to ok must fire OnReconnect exactly once.
	f.mu.Lock()
	f.key = "wrong"
	f.mu.Unlock()
	if err := d.RefreshInventory(context.Background()); err != nil {
		t.Fatalf("refresh after fixing the key: %v", err)
	}
	if n := atomic.LoadInt32(&reconnects); n != 1 {
		t.Fatalf("transition to ok must fire OnReconnect exactly once, got %d", n)
	}

	// Staying ok on a further successful call must NOT fire it again.
	if err := d.Apply(context.Background(), devGeneral([3]int{0, 255, 0}, 255)); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&reconnects); n != 1 {
		t.Fatalf("OnReconnect must not fire again while already ok, got %d", n)
	}
}

// TestDevOnReconnect_NilIsSafe is the default (#207/#213 callers that
// predate this field, and every config that leaves it unset): a nil
// OnReconnect must never panic on the ok() transition.
func TestDevOnReconnect_NilIsSafe(t *testing.T) {
	f := newDevBridge(t, "BuzzHue1")
	d, err := New(Config{BridgeIP: f.srv.URL, APIKey: f.key, Lights: []LightSpec{{Name: "BuzzHue1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Apply(context.Background(), devGeneral([3]int{255, 0, 0}, 255)); err != nil {
		t.Fatal(err) // must not panic on a nil OnReconnect
	}
}

// ---------------------------------------------------------------------------
// Batch B (§5.8) — Hue native groups: lifecycle (B1), hybrid write + the
// dedup-cache pitfall (B2), and the N=30 group-gain measurement (B3).
// newDevDriver defaults every OTHER test in this file to
// disableGroupsForTest=true (see its own doc comment) — every test below
// explicitly flips it back on.
// ---------------------------------------------------------------------------

// newDevGroupsEnabled is newDevDriver plus the Batch B opt-in.
func newDevGroupsEnabled(t *testing.T, f *devBridge, lights ...LightSpec) (*Driver, *devLogSink) {
	t.Helper()
	d, sink := newDevDriver(t, f, lights...)
	d.disableGroupsForTest = false
	return d, sink
}

// TestDevGroupsReconciliation_CreatesCorrectsDeletes covers B1's whole
// lifecycle against the fake bridge's own group store (f.groups) — never the
// driver's private cache — so this test would keep meaning the same thing
// even if the driver's internal bookkeeping changed shape.
func TestDevGroupsReconciliation_CreatesCorrectsDeletes(t *testing.T) {
	f := newDevBridge(t, "G1", "G2", "T1a", "T1b", "T2a")
	// A stale group BuzzMaster once created but no longer wants (team "Ghost"
	// removed from config since) — reconciliation must delete it.
	f.groups["99"] = &devGroup{name: "buzzmaster-team-Ghost", lights: []string{"1"}}
	// A group whose composition has drifted (someone re-pointed it in the Hue
	// app) — reconciliation must correct it, not leave it or recreate it
	// under a new id.
	f.groups["7"] = &devGroup{name: "buzzmaster-general", lights: []string{"1"}} // missing "2"

	d, _ := newDevGroupsEnabled(t, f,
		LightSpec{Name: "G1"}, LightSpec{Name: "G2"},
		LightSpec{Name: "T1a", Role: RoleTeam, Team: "A"}, LightSpec{Name: "T1b", Role: RoleTeam, Team: "A"},
		LightSpec{Name: "T2a", Role: RoleTeam, Team: "B"}, // team B has only 1 bulb: no buzzmaster-team-B (table row)
	)
	if err := d.RefreshInventory(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	byName := map[string]*devGroup{}
	for _, g := range f.groups {
		byName[g.name] = g
	}
	if _, ok := byName["buzzmaster-team-Ghost"]; ok {
		t.Error("stale group must have been deleted")
	}
	if len(byName) != 3 {
		t.Fatalf("expected exactly 3 buzzmaster groups (ambiance, general, team-A — no team-B, no ghost), got %d: %+v", len(byName), byName)
	}
	sortedCopy := func(ss []string) []string { s := append([]string(nil), ss...); sort.Strings(s); return s }
	if got := sortedCopy(byName["buzzmaster-ambiance"].lights); fmt.Sprint(got) != fmt.Sprint([]string{"1", "2", "3", "4", "5"}) {
		t.Errorf("buzzmaster-ambiance members = %v, want all 5", got)
	}
	if got := sortedCopy(byName["buzzmaster-general"].lights); fmt.Sprint(got) != fmt.Sprint([]string{"1", "2"}) {
		t.Errorf("buzzmaster-general composition not corrected: %v (drift must have been fixed to G1+G2, not left at just G1, not recreated under a new id)", got)
	}
	if id7, ok := f.groups["7"]; !ok || id7.name != "buzzmaster-general" {
		t.Error("buzzmaster-general must have been corrected IN PLACE (id 7), never deleted+recreated under a new id")
	}
	if got := sortedCopy(byName["buzzmaster-team-A"].lights); fmt.Sprint(got) != fmt.Sprint([]string{"3", "4"}) {
		t.Errorf("buzzmaster-team-A members = %v, want T1a+T1b", got)
	}
	if _, ok := byName["buzzmaster-team-B"]; ok {
		t.Error("team B has only 1 bulb — no group must be created for it (table row, contract §5.8)")
	}

	// Re-run with nothing changed: idempotent, no spurious deletes/creates.
	f.mu.Lock()
	before := len(f.groups)
	f.mu.Unlock()
	if err := d.RefreshInventory(context.Background()); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	f.mu.Lock()
	after := len(f.groups)
	f.mu.Unlock()
	if before != after {
		t.Errorf("idempotent reconciliation changed the group count: %d -> %d", before, after)
	}
}

// TestDevApplyWritesViaGroupWhenTargetHasAtLeastTwoLights is B2's core rule:
// one PUT to the group's action, not N PUTs to N lights, once ≥2 members of
// the same target are dirty — and the ordinary per-light dedup keeps working
// unchanged once nothing is dirty.
func TestDevApplyWritesViaGroupWhenTargetHasAtLeastTwoLights(t *testing.T) {
	f := newDevBridge(t, "G1", "G2")
	d, _ := newDevGroupsEnabled(t, f, LightSpec{Name: "G1"}, LightSpec{Name: "G2"})
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if err := d.Apply(ctx, devGeneral([3]int{255, 0, 0}, 200)); err != nil {
		t.Fatal(err)
	}
	puts := f.puts()
	if len(puts) != 1 || !strings.Contains(puts[0].path, "/groups/") || !strings.HasSuffix(puts[0].path, "/action") {
		t.Fatalf("expected exactly 1 PUT to a group action, got %+v", puts)
	}
	if st := d.Status(); st.Stats.Writes != 1 || st.Stats.GroupWrites != 1 {
		t.Fatalf("stats: %+v", st.Stats)
	}

	// Unchanged: no write at all, group or individual.
	if err := d.Apply(ctx, devGeneral([3]int{255, 0, 0}, 200)); err != nil {
		t.Fatal(err)
	}
	if len(f.puts()) != 1 {
		t.Fatalf("unchanged state must not be re-written, got %d PUTs total", len(f.puts()))
	}

	// A genuine change writes via the group again, once.
	if err := d.Apply(ctx, devGeneral([3]int{0, 0, 255}, 100)); err != nil {
		t.Fatal(err)
	}
	if len(f.puts()) != 2 {
		t.Fatalf("expected a second single group PUT, got %d total", len(f.puts()))
	}
}

// TestDevGroupWriteKeepsPerLightDedupCacheHonest is the dedicated pitfall
// test the handoff asks for verbatim (contracts/hue-bridge.md §5.8,
// "articulation avec §5.3"): a group write must update EVERY member's own
// per-light dedup entry to the value it actually wrote — never leave it
// stale at whatever it held before groups existed for this target. A stale
// entry would let a later Apply wrongly believe a bulb already holds a
// colour it never received (dedup skips a write the bulb still needs).
//
// Sequence: write RED individually (pre-group baseline — groups are held
// OFF via disableGroupsForTest for this one step, the test seam's own
// purpose, since ensureResolved would otherwise reconcile and start using
// the group from the very first Apply) — turn groups on and reconcile —
// write WHITE (both members dirty relative to the RED baseline, written via
// the group in ONE PUT) — write RED again. If step 4's per-light cache were
// left at the step-1 RED value instead of being updated to WHITE by the
// group write, this last Apply would wrongly see "already RED, skip" even
// though the bulbs are physically WHITE — exactly the bug the contract
// describes ("l'ampoule reste sur une couleur que le serveur croit avoir
// changée"). Physical bridge state is asserted at the end, not just PUT
// counts, per this project's standing rule to verify concretely.
func TestDevGroupWriteKeepsPerLightDedupCacheHonest(t *testing.T) {
	f := newDevBridge(t, "L1", "L2")
	d, _ := newDevDriver(t, f, LightSpec{Name: "L1"}, LightSpec{Name: "L2"}) // disableGroupsForTest=true (default)
	ctx := context.Background()

	red := devGeneral([3]int{255, 0, 0}, 255)
	white := devGeneral([3]int{255, 255, 255}, 255)

	// 1. Baseline: groups held off, so this necessarily writes per-light —
	// exercising the OLD (pre-group) per-light cache path.
	if err := d.Apply(ctx, red); err != nil {
		t.Fatal(err)
	}
	if puts := f.puts(); len(puts) != 2 || strings.Contains(puts[0].path, "groups") {
		t.Fatalf("baseline must be 2 individual PUTs, got %+v", puts)
	}

	// 2. Turn groups on and reconcile: buzzmaster-general (L1+L2) now exists.
	// Reconciliation itself must not touch the bulbs (no PUT), only the
	// group object.
	d.disableGroupsForTest = false
	putsBefore := len(f.puts())
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(f.puts()) != putsBefore {
		t.Fatal("reconciliation must never write a light's state")
	}

	// 3. WHITE differs from the RED baseline for both lights -> written via
	// the group, ONE PUT.
	if err := d.Apply(ctx, white); err != nil {
		t.Fatal(err)
	}
	puts := f.puts()
	if len(puts) != 3 || !strings.Contains(puts[2].path, "/groups/") {
		t.Fatalf("step 3 must add exactly 1 group PUT, got %+v", puts)
	}

	// 4. Back to RED: the bulbs are physically WHITE (from step 3). If the
	// per-light cache were still at step 1's RED, this would be (wrongly)
	// skipped as "unchanged". It must NOT be skipped.
	if err := d.Apply(ctx, red); err != nil {
		t.Fatal(err)
	}
	puts = f.puts()
	if len(puts) != 4 {
		t.Fatalf("step 4 (RED again) must produce a write — the per-light dedup cache must have been updated by the step-3 GROUP write, not left stale at step 1's RED; got %d total PUTs: %+v", len(puts), puts)
	}

	// Physical correctness: both bulbs must actually be on RED now (bri 254,
	// contract §5.2's own bri mapping for intensity 255), not just "a PUT was
	// sent".
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range []string{"1", "2"} {
		state := f.lights[id]["state"].(map[string]any)
		on, _ := state["on"].(bool)
		bri, _ := state["bri"].(float64)
		if !on || bri != 254 {
			t.Errorf("light %s not physically RED after step 4: %+v", id, state)
		}
	}
}

// TestDevGroupWriteFallsBackToPerLightOnFailure is B1 rule 5 ("repli
// obligatoire"): a group the driver still trusts (from a prior successful
// reconciliation) that then fails to accept a write — here, simulated by the
// group vanishing from the bridge between reconciliation and the write, a
// realistic "someone deleted it in the Hue app" scenario — must fall back to
// writing every member individually WITHIN THE SAME Apply call, and the
// overall Apply must still succeed.
func TestDevGroupWriteFallsBackToPerLightOnFailure(t *testing.T) {
	f := newDevBridge(t, "L1", "L2")
	d, _ := newDevGroupsEnabled(t, f, LightSpec{Name: "L1"}, LightSpec{Name: "L2"})
	ctx := context.Background()
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var groupID string
	f.mu.Lock()
	for id, g := range f.groups {
		if g.name == groupNameGeneral {
			groupID = id
		}
	}
	// The group vanishes from the bridge — reconciliation is not due again
	// yet (RefreshEvery has not elapsed), so the driver still trusts it.
	delete(f.groups, groupID)
	f.mu.Unlock()

	if err := d.Apply(ctx, devGeneral([3]int{0, 255, 0}, 200)); err != nil {
		t.Fatalf("Apply must still succeed via the per-light fallback: %v", err)
	}
	paths := f.paths()
	sawFailedGroupAction, sawBothLights := false, map[string]bool{}
	for _, p := range paths {
		if strings.Contains(p, "/groups/"+groupID+"/action") {
			sawFailedGroupAction = true
		}
		if p == "PUT /api/k/lights/1/state" || p == "PUT /api/k/lights/2/state" {
			sawBothLights[p] = true
		}
	}
	if !sawFailedGroupAction {
		t.Errorf("expected the (now-vanished) group action to be attempted first, got %v", paths)
	}
	if len(sawBothLights) != 2 {
		t.Errorf("expected both lights individually written after the group failure, got %v", paths)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range []string{"1", "2"} {
		if on, _ := f.lights[id]["state"].(map[string]any)["on"].(bool); !on {
			t.Errorf("light %s must have been reached by the fallback", id)
		}
	}
}

// ---------------------------------------------------------------------------
// B3 — §8 "gain des groupes": measured before/after at N=30, same fake-bridge
// method (injected 40 ms write latency, matching the spike's measured
// 48-59 ms on real hardware) already established and published for N=2,4,6
// by TestDevMeasureSpreadForN, extended below to also cover N=30.
// ---------------------------------------------------------------------------

type devGroupGainResult struct {
	N                  int     `json:"n_lights"`
	LatencyMs          float64 `json:"simulated_write_latency_ms"`
	BeforeWrites       int     `json:"before_writes"`
	BeforeSpreadMs     float64 `json:"before_spread_first_to_last_ms"`
	BeforeApplyTotalMs float64 `json:"before_apply_total_ms"`
	AfterWrites        int     `json:"after_writes"`
	AfterApplyTotalMs  float64 `json:"after_apply_total_ms"`
}

// TestDevMeasureGroupGainAtN30 is the §8 "gain des groupes" row: the exact
// same "toute la salle d'une couleur" scenario measured twice at the real
// installation size (N=30) — once with groups disabled (the §2/pre-§5.8
// baseline: N sequential PUTs), once with buzzmaster-general reconciled and
// used (one PUT to its action). GROUP_GAIN_MEASUREMENT is the report-facing
// JSON line (same convention as SPREAD_MEASUREMENT/BURST_MEASUREMENT).
func TestDevMeasureGroupGainAtN30(t *testing.T) {
	if testing.Short() {
		t.Skip("timing measurement")
	}
	const n = 30
	const lat = 40 * time.Millisecond
	names := make([]string, n)
	cfgs := make([]LightSpec, n)
	for i := range names {
		names[i] = fmt.Sprintf("L%d", i+1)
		cfgs[i] = LightSpec{Name: names[i]}
	}

	// BEFORE.
	fBefore := newDevBridge(t, names...)
	fBefore.latency = lat
	dBefore, _ := newDevDriver(t, fBefore, cfgs...) // disableGroupsForTest=true (default)
	_ = dBefore.Apply(context.Background(), devGeneral([3]int{255, 255, 255}, 100))
	fBefore.mu.Lock()
	fBefore.requests = nil
	fBefore.mu.Unlock()
	startBefore := time.Now()
	if err := dBefore.Apply(context.Background(), devGeneral([3]int{255, 0, 0}, 255)); err != nil {
		t.Fatal(err)
	}
	beforeTotal := time.Since(startBefore)
	beforePuts := f0Sorted(fBefore.puts())
	if len(beforePuts) != n {
		t.Fatalf("before: expected %d individual PUTs, got %d", n, len(beforePuts))
	}
	beforeSpread := beforePuts[len(beforePuts)-1].at.Sub(beforePuts[0].at)

	// AFTER.
	fAfter := newDevBridge(t, names...)
	fAfter.latency = lat
	dAfter, _ := newDevGroupsEnabled(t, fAfter, cfgs...)
	if err := dAfter.RefreshInventory(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var generalMembers int
	fAfter.mu.Lock()
	for _, g := range fAfter.groups {
		if g.name == groupNameGeneral {
			generalMembers = len(g.lights)
		}
	}
	fAfter.mu.Unlock()
	if generalMembers != n {
		t.Fatalf("buzzmaster-general must cover all %d lights before measuring, got %d", n, generalMembers)
	}
	_ = dAfter.Apply(context.Background(), devGeneral([3]int{255, 255, 255}, 100))
	fAfter.mu.Lock()
	fAfter.requests = nil
	fAfter.mu.Unlock()
	startAfter := time.Now()
	if err := dAfter.Apply(context.Background(), devGeneral([3]int{255, 0, 0}, 255)); err != nil {
		t.Fatal(err)
	}
	afterTotal := time.Since(startAfter)
	afterPuts := fAfter.puts()
	if len(afterPuts) != 1 || !strings.Contains(afterPuts[0].path, "/groups/") {
		t.Fatalf("after: expected exactly 1 group PUT at N=%d, got %+v", n, afterPuts)
	}

	result := devGroupGainResult{
		N: n, LatencyMs: devMs(lat),
		BeforeWrites: len(beforePuts), BeforeSpreadMs: devMs(beforeSpread), BeforeApplyTotalMs: devMs(beforeTotal),
		AfterWrites: len(afterPuts), AfterApplyTotalMs: devMs(afterTotal),
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("GROUP_GAIN_MEASUREMENT %s", b)

	if result.AfterApplyTotalMs >= result.BeforeApplyTotalMs {
		t.Errorf("groups must be markedly faster at N=%d: before=%.0fms after=%.0fms", n, result.BeforeApplyTotalMs, result.AfterApplyTotalMs)
	}

	// Physical correctness: the single group command must actually have
	// reached every one of the 30 bulbs, not just the ones a naive
	// "first N members" shortcut might have addressed.
	fAfter.mu.Lock()
	defer fAfter.mu.Unlock()
	for id, l := range fAfter.lights {
		state := l["state"].(map[string]any)
		on, _ := state["on"].(bool)
		bri, _ := state["bri"].(float64)
		if !on || bri != 254 {
			t.Errorf("light %s not reached by the group write: %+v", id, state)
		}
	}
}

// f0Sorted sorts a puts() slice by request time (the spread measurements'
// own repeated idiom, factored out once it is needed a third time).
func f0Sorted(puts []devRecorded) []devRecorded {
	sort.Slice(puts, func(i, j int) bool { return puts[i].at.Before(puts[j].at) })
	return puts
}
