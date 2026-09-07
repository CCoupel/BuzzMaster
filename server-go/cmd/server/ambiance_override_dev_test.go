package main

// Developer-side tests for #208 (T2.3, contract lighting.md §10.1/§10.4):
// the ON/AUTO/OFF selector, the Flash bascule, and the shutdown extinction —
// cmd/server/ambiance_override.go.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

// TestDevLightingOverrideGeneral_PureLogic pins lightingOverrideGeneral in
// isolation, every combination — AUTO passes the derived colour through
// unchanged, ON/OFF force it, Flash primes over either.
func TestDevLightingOverrideGeneral_PureLogic(t *testing.T) {
	app := newTestApp(t)
	auto := [3]int{40, 90, 255}

	color, intensity := app.lightingOverrideGeneral(auto, 160)
	if color != auto || intensity != 160 {
		t.Fatalf("AUTO (default) must pass the derived colour through, got %v/%d", color, intensity)
	}

	app.setLightingMode(lightingModeOn)
	if color, intensity := app.lightingOverrideGeneral(auto, 160); color != lightingOnColor || intensity != lightingOnIntensity {
		t.Fatalf("ON must force %v/%d, got %v/%d", lightingOnColor, lightingOnIntensity, color, intensity)
	}

	app.setLightingMode(lightingModeOff)
	if color, intensity := app.lightingOverrideGeneral(auto, 160); color != [3]int{0, 0, 0} || intensity != 0 {
		t.Fatalf("OFF must force {0,0,0}/0, got %v/%d", color, intensity)
	}

	app.setLightingMode(lightingModeAuto)
	if color, intensity := app.lightingOverrideGeneral(auto, 160); color != auto || intensity != 160 {
		t.Fatalf("back to AUTO must pass the derived colour through again, got %v/%d", color, intensity)
	}

	// Flash primes over OFF (contract §10.1.2: "un Flash demandé alors que
	// la salle est sur OFF ne produirait rien de visible" otherwise).
	app.setLightingMode(lightingModeOff)
	app.lightingFlashOn.Store(true)
	app.lightingFlashPhaseOn.Store(true)
	if color, intensity := app.lightingOverrideGeneral(auto, 160); color != lightingFlashColor || intensity != lightingOnIntensity {
		t.Fatalf("Flash ON-phase must win over OFF, got %v/%d", color, intensity)
	}
	app.lightingFlashPhaseOn.Store(false)
	if color, intensity := app.lightingOverrideGeneral(auto, 160); color != [3]int{0, 0, 0} || intensity != 0 {
		t.Fatalf("Flash OFF-phase must be dark, got %v/%d", color, intensity)
	}
}

// TestDevLightingMode_ScopedToGeneralZoneOnly is the #208 "Portée" rule
// (contract §10.1, §10.1's own ⚠️): the selector NEVER touches a team's own
// zone (#213) — only "general".
func TestDevLightingMode_ScopedToGeneralZoneOnly(t *testing.T) {
	app := newTestApp(t)
	app.setLightingMode(lightingModeOff)

	ev := lighting.Event{Kind: lighting.KindTeamTurn, Teams: []string{"TeamA"}}
	st := app.ambianceScene(ev)
	var general, teamA lighting.ZoneState
	for _, z := range st.Zones {
		switch z.Zone {
		case lighting.ZoneGeneral:
			general = z
		case "TeamA":
			teamA = z
		}
	}
	if general.Intensity != 0 {
		t.Fatalf("OFF must force the general zone dark, got %+v", general)
	}
	wantTeamA := app.teamNameToRGB("TeamA")
	if teamA.Color != wantTeamA || teamA.Intensity != ambianceSceneTeamTurn.Intensity {
		t.Fatalf("TeamA's own zone must stay auto-derived regardless of the OFF selector, got %+v want colour=%v intensity=%d", teamA, wantTeamA, ambianceSceneTeamTurn.Intensity)
	}
}

// TestDevLightingMode_ReDerivesThroughTheRealWriter is the end-to-end proof
// (contract §10.1.1 point 3, §10.3): setLightingMode goes through the SAME
// NotifyState() path as any other ambiance transition, and AUTO returns the
// room to whatever the live game state says — no separate "restore" step.
func TestDevLightingMode_ReDerivesThroughTheRealWriter(t *testing.T) {
	app := newTestApp(t)
	fake := lighting.NewFakeDriver()
	app.lightingWriter.Store(app.newAmbianceWriter(fake))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.ambiance().Start(ctx)

	app.engine.SetPhase(game.PhasePrepare) // KindReady: {255,255,255}/200

	app.setLightingMode(lightingModeOff)
	waitForCount(t, fake, 1)
	if last, _ := fake.Last(); last.Zones[0].Intensity != 0 {
		t.Fatalf("OFF must have been applied, got %+v", last.Zones[0])
	}

	app.setLightingMode(lightingModeAuto)
	waitForCount(t, fake, 2)
	if last, _ := fake.Last(); last.Zones[0].Color != [3]int{255, 255, 255} || last.Zones[0].Intensity != 200 {
		t.Fatalf("AUTO must re-derive the live KindReady scene, got %+v", last.Zones[0])
	}
}

// TestDevLightingFlash_BlinksAndReturnsToSelector drives the REAL
// runLightingFlash goroutine (not the pure function above): engaging Flash
// must produce alternating lit/dark applies, and disengaging it must return
// exactly to the selector's own position (here OFF) — contract §10.1.2.
func TestDevLightingFlash_BlinksAndReturnsToSelector(t *testing.T) {
	app := newTestApp(t)
	fake := lighting.NewFakeDriver()
	app.lightingWriter.Store(app.newAmbianceWriter(fake))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.ambiance().Start(ctx)

	app.setLightingMode(lightingModeOff)
	waitForCount(t, fake, 1)

	app.setLightingFlash(true)
	if !app.LightingFlash() {
		t.Fatal("LightingFlash() must report engaged")
	}
	// Wait for at least one full on/off cycle: two more applies after the
	// initial OFF, alternating intensity.
	waitForCount(t, fake, 3)
	first, _ := fake.Last()
	waitForCount(t, fake, 4)
	second, _ := fake.Last()
	if first.Zones[0].Intensity == second.Zones[0].Intensity {
		t.Fatalf("Flash must alternate intensity between ticks, got %d then %d", first.Zones[0].Intensity, second.Zones[0].Intensity)
	}

	app.setLightingFlash(false)
	if app.LightingFlash() {
		t.Fatal("LightingFlash() must report disengaged")
	}
	deadline := time.Now().Add(2 * time.Second)
	var last lighting.State
	for time.Now().Before(deadline) {
		last, _ = fake.Last()
		if last.Zones[0].Intensity == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if last.Zones[0].Intensity != 0 {
		t.Fatalf("Flash off must return to the OFF selector, got %+v", last.Zones[0])
	}
	// The blink goroutine must actually have stopped: no further applies
	// after a settling window.
	countAfterOff := fake.Count()
	time.Sleep(2 * (lightingFlashOnPhase + lightingFlashOffPhase))
	if fake.Count() != countAfterOff {
		t.Fatalf("blink goroutine kept running after setLightingFlash(false): %d applies before, %d after", countAfterOff, fake.Count())
	}
}

// TestDevLightingMode_SetLightingModeValidation pins the LightingProvider
// string boundary (internal/server/http_lighting.go): valid values apply
// and normalise to upper-case, anything else is refused without changing
// the current mode.
func TestDevLightingMode_SetLightingModeValidation(t *testing.T) {
	app := newTestApp(t)
	if got := app.LightingMode(); got != "AUTO" {
		t.Fatalf("default mode must be AUTO, got %q", got)
	}
	if err := app.SetLightingMode("OFF"); err != nil {
		t.Fatalf("SetLightingMode(OFF): %v", err)
	}
	if got := app.LightingMode(); got != "OFF" {
		t.Fatalf("mode = %q, want OFF", got)
	}
	if err := app.SetLightingMode("bogus"); err == nil {
		t.Fatal("SetLightingMode(bogus) must be refused")
	}
	if got := app.LightingMode(); got != "OFF" {
		t.Fatalf("a refused SetLightingMode must not change the mode, got %q", got)
	}
}

// TestDevStop_TurnsBuzzersOffOnShutdown is the buzzer half of contract
// §10.4, pinned exactly as (*App).stop() (main.go) actually calls it: the
// SAME function ENTRACTE uses (sendLEDSetAllEntracteOff), directly — not
// through shutdownExtinguishHueLighting (Hue-only, ambiance_override.go;
// see that function's own doc comment for why the buzzer call lives in
// main.go instead: the AST exhaustiveness test, contract §7, only scans
// main.go). a.stop() itself is not exercised end-to-end here: it
// unconditionally dereferences a.dnsServer/a.mdnsServer/a.httpServer/
// a.broadcaster/a.udpBcast, none of which newTestApp populates (same
// reasoning as TestSetupCallbacks_WiresOnShutdownToStop's own comment,
// main_test.go).
func TestDevStop_TurnsBuzzersOffOnShutdown(t *testing.T) {
	app := newTestApp(t)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"m1": {Name: "m1", Team: "TeamA"},
		"m2": {Name: "m2", Team: "TeamB"},
	})

	app.shutdownExtinguishHueLighting() // no driver configured: must be a no-op, never panics
	app.sendLEDSetAllEntracteOff()      // the actual call (*App).stop() makes

	for _, mac := range []string{"m1", "m2"} {
		payload, ok := app.bumperLEDState[mac]
		if !ok || payload.Intensity != 0 || payload.Color != [3]int{0, 0, 0} {
			t.Fatalf("shutdown must turn buzzer %s off, got %+v (ok=%v)", mac, payload, ok)
		}
	}
}

// TestDevShutdownExtinguishHueLighting_NoDriverNeverPanics is the "no
// bridge configured" case (contract §5.5: strictly identical behaviour to a
// server with no lighting at all) — must be a pure no-op on the Hue side.
func TestDevShutdownExtinguishHueLighting_NoDriverNeverPanics(t *testing.T) {
	app := newTestApp(t)
	if app.LightingDriver() != nil {
		t.Fatal("test app must start with no Hue driver")
	}
	app.shutdownExtinguishHueLighting() // must not panic
}

// TestDevShutdownExtinguishLighting_AppliesOffToEveryConfiguredZone is the
// Hue half of §10.4: unlike the #208 override (general-only), the shutdown
// extinction must cover EVERY configured zone — general AND every
// configured team. hueDriver is a concrete *hue.Driver (not the lighting.
// Driver interface FakeDriver implements), so this test drives a real
// driver against a fake HTTP bridge — same pattern as
// TestDevAmbianceReconfigureFromConfig (ambiance_dev_test.go).
func TestDevShutdownExtinguishHueLighting_AppliesOffToEveryConfiguredZone(t *testing.T) {
	var mu sync.Mutex
	var stateWrites int
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			_, _ = w.Write([]byte(`{"bridgeid":"fffe0000deadbeef","modelid":"BSB002"}`))
		case strings.HasSuffix(r.URL.Path, "/lights"):
			_, _ = w.Write([]byte(`{"8":{"name":"General1","state":{"on":true,"bri":100,"xy":[0.3,0.3],"reachable":true}},"9":{"name":"Team1","state":{"on":true,"bri":100,"xy":[0.3,0.3],"reachable":true}}}`))
		case strings.HasSuffix(r.URL.Path, "/state") && r.Method == "PUT":
			mu.Lock()
			stateWrites++
			mu.Unlock()
			_, _ = w.Write([]byte(`[{"success":{"on":false}}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer bridge.Close()

	app := newTestApp(t)
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })
	cfg := saved
	cfg.Lighting = config.LightingConfig{
		Enabled: true, BridgeIP: bridge.URL, BridgeID: "fffe0000deadbeef", APIKey: "k",
		Lights: []config.LightingLightEntry{
			{Name: "General1", Role: "general"},
			{Name: "Team1", Role: "team", Team: "TeamA"},
		},
	}
	config.SetInstance(&cfg)
	app.setupAmbiance()
	if app.LightingDriver() == nil {
		t.Fatal("driver must be built from a valid, enabled config")
	}

	app.shutdownExtinguishHueLighting()

	mu.Lock()
	n := stateWrites
	mu.Unlock()
	if n != 2 {
		t.Fatalf("shutdown must write OFF to every configured light (general + TeamA), got %d PUT /state call(s)", n)
	}
}
