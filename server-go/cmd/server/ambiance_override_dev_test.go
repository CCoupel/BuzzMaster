package main

// Developer-side tests for #208 (T2.3, contract lighting.md §10.1/§10.4):
// the ON/AUTO/OFF selector, the Flash bascule, and the shutdown extinction —
// cmd/server/ambiance_override.go.

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
	"buzzcontrol/internal/lighting/hue"
	"buzzcontrol/internal/server"
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
	// 2026-09-08 revision: the un-lit phase is no longer dark — it shows the
	// room's own current colour (ambianceThemeColor(), white here since
	// newTestApp sets no httpServer/live question), at the SAME full
	// intensity as the white phase.
	app.lightingFlashPhaseOn.Store(false)
	white := [3]int{255, 255, 255}
	if color, intensity := app.lightingOverrideGeneral(auto, 160); color != white || intensity != lightingOnIntensity {
		t.Fatalf("Flash's other phase must show the room's own colour (white here) at full intensity, got %v/%d", color, intensity)
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
	// Batch A/P1: TeamA is distinguished by ev.Teams here, so its own zone
	// is at FULL intensity (255) — not ambianceSceneTeamTurn.Intensity
	// (200), which is the general zone's own value, unaffected.
	wantTeamA := app.teamNameToRGB("TeamA")
	if teamA.Color != wantTeamA || teamA.Intensity != 255 {
		t.Fatalf("TeamA's own zone must stay auto-derived regardless of the OFF selector, got %+v want colour=%v intensity=255", teamA, wantTeamA)
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
	// A real, NON-white theme so the 2026-09-08 white<->room-colour
	// alternation is actually observable (a themeless room degenerates to
	// white/white, an accepted but non-discriminating case — see
	// lightingOverrideGeneral's own doc comment).
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	app.engine.Ready("q1", &game.Question{ID: "q1", Type: game.QuestionTypeQCM, Category: game.CategoryGeography})
	geography := [3]int{0x3b, 0x82, 0xf6}
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
	// Wait for at least one full white/room-colour cycle: two more applies
	// after the initial OFF, alternating COLOUR (2026-09-08 revision — no
	// longer intensity, both phases are now full/lit).
	waitForCount(t, fake, 3)
	first, _ := fake.Last()
	waitForCount(t, fake, 4)
	second, _ := fake.Last()
	if first.Zones[0].Color == second.Zones[0].Color {
		t.Fatalf("Flash must alternate colour between ticks, got %v then %v", first.Zones[0].Color, second.Zones[0].Color)
	}
	if first.Zones[0].Intensity != lightingOnIntensity || second.Zones[0].Intensity != lightingOnIntensity {
		t.Fatalf("both Flash phases must stay at full intensity, got %d then %d", first.Zones[0].Intensity, second.Zones[0].Intensity)
	}
	for _, z := range []lighting.ZoneState{first.Zones[0], second.Zones[0]} {
		if z.Color != lightingFlashColor && z.Color != geography {
			t.Fatalf("Flash phase colour must be white or the room's theme, got %v", z.Color)
		}
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

// TestDevStop_FlashEngagedAtShutdown_FinalStateIsOff is code-reviewer's own
// regression test for MAJEUR 1 (v10 Batch 2 review,
// _work/reports/code-reviewer-v10-batch2-20260907-124816.md): Flash still
// ticking (every ~400 ms) at the moment shutdownExtinguishHueLighting()
// runs must never re-light the room after the forced OFF. See that
// function's own doc comment for the mechanism — SetDriver(nil)
// synchronously quiesces the writer's driver (waiting out any in-flight
// Apply via hue.Driver's own opMu, then marking it permanently closed)
// BEFORE the forced OFF is written on a fresh driver instance — so this is
// a structural guarantee, not a timing one; this test proves it holds in
// practice against the real blink goroutine, not just by inspection.
func TestDevStop_FlashEngagedAtShutdown_FinalStateIsOff(t *testing.T) {
	app, bridge, cancel := devReconnectApp(t)
	defer cancel()

	app.engine.SetPhase(game.PhaseStarted) // partie en cours, comme au vrai arrêt serveur
	app.setLightingFlash(true)
	devWaitPutCount(t, bridge, 1) // au moins un tick de blink a réellement écrit

	app.shutdownExtinguishHueLighting()
	countAfterShutdown := bridge.count()
	if !strings.Contains(bridge.last(), `"on":false`) {
		t.Fatalf("l'état juste après l'extinction forcée doit être OFF, got %q", bridge.last())
	}

	// Le goroutine de blink doit être bel et bien mort : aucun nouveau PUT,
	// même en attendant largement plus qu'un cycle complet — pas un simple
	// sommeil symbolique.
	time.Sleep(3 * (lightingFlashOnPhase + lightingFlashOffPhase))
	if got := bridge.count(); got != countAfterShutdown {
		t.Fatalf("Flash a continué d'écrire après shutdownExtinguishHueLighting() : %d PUT(s) juste après l'extinction, %d après stabilisation — la salle a pu se rallumer, dernier corps %q", countAfterShutdown, got, bridge.last())
	}
	if !strings.Contains(bridge.last(), `"on":false`) {
		t.Fatalf("l'état final observé doit rester OFF après stabilisation, got %q", bridge.last())
	}
	if app.LightingFlash() {
		t.Fatal("Flash doit être signalé désengagé après l'extinction")
	}
}

// ---------------------------------------------------------------------------
// §10.3 — resync au retour du pont (gap fermé après le rapport initial de
// Batch 2 : hue.Driver.Config.OnReconnect, câblé dans buildHueDriver()
// (ambiance.go) sur a.ambiance().NotifyState()). Ces deux tests pilotent le
// VRAI *hue.Driver contre un faux pont dont on peut couper/rétablir la
// joignabilité en cours de route — pas un lighting.FakeDriver, qui ne
// saurait pas produire une vraie transition unreachable→ok.
// ---------------------------------------------------------------------------

// devReconnectBridge is a Hue v1 fake whose reachability can be toggled
// mid-test: "down" resets the TCP connection immediately (no timeout to
// wait out — classify() still maps it to ErrUnreachable, "dial/network
// errors"), "up" answers normally and records every /state PUT body.
type devReconnectBridge struct {
	mu    sync.Mutex
	up    bool
	puts  []string
	light string
}

func newDevReconnectBridge(t *testing.T, lightName string) (*httptest.Server, *devReconnectBridge) {
	t.Helper()
	b := &devReconnectBridge{up: true, light: lightName}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		up := b.up
		b.mu.Unlock()
		if !up {
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					_ = conn.Close()
					return
				}
			}
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			_, _ = w.Write([]byte(`{"bridgeid":"fffe0000deadbeef","modelid":"BSB002"}`))
		case strings.HasSuffix(r.URL.Path, "/lights"):
			_, _ = w.Write([]byte(`{"8":{"name":"` + b.light + `","state":{"on":true,"bri":100,"xy":[0.3,0.3],"reachable":true}}}`))
		case strings.HasSuffix(r.URL.Path, "/state") && r.Method == "PUT":
			body, _ := io.ReadAll(r.Body)
			b.mu.Lock()
			b.puts = append(b.puts, string(body))
			b.mu.Unlock()
			_, _ = w.Write([]byte(`[{"success":{"on":true}}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, b
}

func (b *devReconnectBridge) setUp(up bool) {
	b.mu.Lock()
	b.up = up
	b.mu.Unlock()
}

func (b *devReconnectBridge) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.puts)
}

func (b *devReconnectBridge) last() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.puts) == 0 {
		return ""
	}
	return b.puts[len(b.puts)-1]
}

// devReconnectApp wires an App against a devReconnectBridge with one
// "general" light, real driver + real writer running.
func devReconnectApp(t *testing.T) (*App, *devReconnectBridge, context.CancelFunc) {
	t.Helper()
	srv, bridge := newDevReconnectBridge(t, "General1")
	app := newTestApp(t)
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })
	cfg := saved
	cfg.Lighting = config.LightingConfig{
		// Deliberately NO BridgeID: on a failed contact, a configured
		// BridgeID makes the driver attempt a REAL mDNS/SSDP re-discovery
		// (contract §4.1) before giving up — several seconds against no
		// real bridge on the test network. BridgeIP alone is enough to
		// satisfy ambianceIsConfigured() and keeps the outage/recovery
		// cycle below fast and deterministic (same pattern as the
		// "d2"/dead-bridge case in TestDevRefusedAndUnreachableAreDistinct,
		// internal/lighting/hue/driver_dev_test.go).
		Enabled: true, BridgeIP: srv.URL, APIKey: "k",
		Lights: []config.LightingLightEntry{{Name: "General1", Role: "general"}},
	}
	config.SetInstance(&cfg)
	app.setupAmbiance()
	if app.LightingDriver() == nil {
		t.Fatal("driver must be built from a valid, enabled config")
	}
	ctx, cancel := context.WithCancel(context.Background())
	go app.ambiance().Start(ctx)
	return app, bridge, cancel
}

func devWaitPutCount(t *testing.T, b *devReconnectBridge, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if b.count() >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("bridge received %d PUT(s), want >= %d", b.count(), n)
}

func devWaitDriverState(t *testing.T, d *hue.Driver, want hue.BridgeState) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if d.Status().State == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("driver state = %v after 2s, want %v", d.Status().State, want)
}

// TestDevLightingReconnect_AUTO_ResyncsFromLiveGameState is the AUTO half
// of contract §10.3: a bridge outage during a live game, followed by its
// recovery, must re-apply the CURRENT live game state — not a stale scene
// from before the outage, and not merely "whatever happened to be the last
// successful write". Recovery is observed by RefreshInventory (the same
// call GET /api/lighting/lights makes) — nothing polls the bridge on its
// own (contract lighting.md §4, purely event-driven); this mirrors the
// ordinary "admin reopens the ambiance screen" gesture.
//
// ⚠️ Review fix (code-reviewer, v10 Batch 2, MAJEUR 2): TWO bugs, not one.
//  1. Outage detection used to rely on a NotifyState() spontaneously
//     reaching the writer's goroutine (subject to the real 250 ms
//     MinInterval throttle of the Hue driver, then a fixed 2 s poll
//     deadline) — flaky under a loaded `go test -race ./cmd/server/...`
//     (observed ~1/3), since a busy scheduler can delay the writer's own
//     timer past the deadline. Fixed the same way
//     TestDevLightingReconnect_EngagedOverride_IsReappliedNotTheGame
//     already did for its own outage: force the contact explicitly via
//     RefreshInventory instead of waiting on spontaneous timing.
//  2. A SECOND, independent bug, found while chasing #1: the live state at
//     reconnect time (PhaseStarted, no active team) was IDENTICAL to the
//     PRE-outage baseline. Whether the write attempted DURING the outage
//     manages to invalidate the driver's per-light "last applied" cache
//     (contract hue-bridge.md §5.3) before or after the outage is itself
//     detected is a genuine race — one interleaving invalidates it (a
//     fresh write follows recovery), the other leaves it untouched (the
//     resync's own desired state then equals the cached one, so §5.3
//     CORRECTLY skips writing — not a bug in the driver, a genuinely
//     ambiguous test). Fixed by reconnecting on a DIFFERENT phase
//     (PhasePaused → KindPauseAll, amber) than the baseline (KindRunning,
//     blue): the resync's desired colour then differs from the cached one
//     NO MATTER which way that inner race went, so a fresh PUT is now the
//     only possible correct outcome — deterministic by construction, not
//     by timing.
func TestDevLightingReconnect_AUTO_ResyncsFromLiveGameState(t *testing.T) {
	app, bridge, cancel := devReconnectApp(t)
	defer cancel()

	// Partie en cours, pont joignable : KindRunning (bleu neutre, pas
	// d'équipe active).
	app.engine.SetPhase(game.PhaseStarted)
	app.ambiance().NotifyState()
	devWaitPutCount(t, bridge, 1)
	if !strings.Contains(bridge.last(), `"on":true`) {
		t.Fatalf("baseline Apply while up must be lit, got %q", bridge.last())
	}
	baselinePUT := bridge.last()

	// Coupure du pont EN COURS DE PARTIE — la partie change de phase
	// PENDANT la coupure et Y RESTE (PhasePaused, KindPauseAll ambre),
	// délibérément DIFFÉRENTE de la scène de base ci-dessus.
	bridge.setUp(false)
	app.engine.SetPhase(game.PhasePaused)
	app.ambiance().NotifyState()
	_ = app.LightingDriver().RefreshInventory(context.Background()) // force le constat, erreur attendue : c'est la coupure
	devWaitDriverState(t, app.LightingDriver(), hue.StateUnreachable)

	// Rétablissement, observé par un geste normal (inventaire) — le jeu est
	// resté en PhasePaused pendant toute la coupure, pas de nouvel
	// événement à l'instant précis de la reconnexion.
	statsBeforeRecovery := app.LightingDriver().Status().Stats
	bridge.setUp(true)
	if err := app.LightingDriver().RefreshInventory(context.Background()); err != nil {
		t.Fatalf("RefreshInventory after recovery: %v", err)
	}
	devWaitDriverState(t, app.LightingDriver(), hue.StateOK)

	// §10.3 : la resync doit produire un NOUVEL Apply, réappliquant l'état
	// de jeu VIVANT au moment de la reconnexion (PhasePaused ⇒
	// KindPauseAll, ambre) — jamais la scène bleue d'avant la coupure.
	deadline := time.Now().Add(2 * time.Second)
	var applies int
	for time.Now().Before(deadline) {
		applies = app.LightingDriver().Status().Stats.Applies
		if applies > statsBeforeRecovery.Applies {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if applies <= statsBeforeRecovery.Applies {
		t.Fatal("resync must trigger a fresh Apply (OnReconnect → NotifyState)")
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && bridge.last() == baselinePUT {
		time.Sleep(5 * time.Millisecond)
	}
	if bridge.last() == baselinePUT {
		t.Fatalf("resync must have written a NEW state (KindPauseAll, ambre) distinct from the pre-outage baseline, got the same PUT body %q", baselinePUT)
	}
	if !strings.Contains(bridge.last(), `"on":true`) {
		t.Fatalf("resync AUTO must re-derive a lit KindPauseAll scene from the live game state, got %q", bridge.last())
	}
}

// TestDevLightingReconnect_EngagedOverride_IsReappliedNotTheGame is the
// override half of contract §10.3/§10.1.1 point 7: "si un override est
// engagé, il est réappliqué tel quel" — a bridge recovering while the
// selector is OFF must come back OFF, even though the live game state (if
// AUTO were in effect) would show a lit scene. One chemin de calcul, one
// source of truth: (mode courant, état de jeu vivant).
func TestDevLightingReconnect_EngagedOverride_IsReappliedNotTheGame(t *testing.T) {
	app, bridge, cancel := devReconnectApp(t)
	defer cancel()

	app.engine.SetPhase(game.PhaseStarted) // partie en cours — AUTO montrerait une scène allumée
	app.setLightingMode(lightingModeOff)   // override engagé AVANT la coupure
	devWaitPutCount(t, bridge, 1)
	if !strings.Contains(bridge.last(), `"on":false`) {
		t.Fatalf("OFF must be applied while up, got %q", bridge.last())
	}

	bridge.setUp(false)
	app.engine.SetPhase(game.PhasePaused) // le jeu continue de bouger pendant la coupure
	app.ambiance().NotifyState()
	// L'override OFF est déjà l'état désiré ET déjà appliqué (§5.3, "n'écrire
	// que ce qui change") : NotifyState() seul ne produit ICI aucune
	// tentative d'écriture réseau, donc aucune détection de la coupure — un
	// geste qui force réellement un contact (RefreshInventory, `force`
	// bypass la fraîcheur du cache) est nécessaire pour l'observer, comme
	// dans la vraie vie (ex: l'admin rouvre l'écran Ambiance).
	_ = app.LightingDriver().RefreshInventory(context.Background()) // erreur attendue : c'est la coupure
	devWaitDriverState(t, app.LightingDriver(), hue.StateUnreachable)
	countDuringOutage := bridge.count()

	statsBeforeRecovery := app.LightingDriver().Status().Stats

	bridge.setUp(true)
	if err := app.LightingDriver().RefreshInventory(context.Background()); err != nil {
		t.Fatalf("RefreshInventory after recovery: %v", err)
	}
	devWaitDriverState(t, app.LightingDriver(), hue.StateOK)

	// §10.3 : la resync (OnReconnect → NotifyState()) doit avoir produit un
	// NOUVEL Apply — sinon le fix ne serait tout simplement pas câblé. Mais
	// puisque l'override OFF était DÉJÀ l'état appliqué avant la coupure, cet
	// Apply ne doit écrire RIEN de nouveau sur le pont (§5.3, "n'écrire que
	// ce qui change") : c'est la preuve que la resync a bien recalculé OFF,
	// et non la scène de jeu vivante (PhaseStarted sans override aurait
	// exigé une écriture "on":true différente, donc un PUT supplémentaire).
	deadline := time.Now().Add(2 * time.Second)
	var applies int
	for time.Now().Before(deadline) {
		applies = app.LightingDriver().Status().Stats.Applies
		if applies > statsBeforeRecovery.Applies {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if applies <= statsBeforeRecovery.Applies {
		t.Fatal("resync must trigger a fresh Apply (OnReconnect → NotifyState) even when nothing ends up needing to be written")
	}
	if got := bridge.count(); got != countDuringOutage {
		t.Fatalf("resync with an engaged OFF override must NOT re-write (already applied) — got %d PUT(s) after reconnect (was %d before) — the live game's AUTO scene must have leaked through", got, countDuringOutage)
	}
	if got := app.LightingMode(); got != "OFF" {
		t.Fatalf("the selector itself must still read OFF after the resync, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// PreviewColor (task-dev-backend-preview-real-color-20260908.md): POST
// /api/lighting/preview shows the REAL colour a bulb will carry once
// assigned, not a fixed white — team palette for "team:<name>", the
// general zone's own current colour otherwise.
// ---------------------------------------------------------------------------

// TestDevPreviewColor_TeamRoleUsesTeamPalette pins the "team:<name>" branch:
// the SAME palette as everywhere else (teamNameToRGB), never a second one.
func TestDevPreviewColor_TeamRoleUsesTeamPalette(t *testing.T) {
	app := newTestApp(t) // TeamA/TeamB/TeamC configured — see newTestApp's own comment
	want := app.teamNameToRGB("TeamA")
	if got := app.PreviewColor("team:TeamA"); got != want {
		t.Fatalf(`PreviewColor("team:TeamA") = %v, want teamNameToRGB("TeamA") = %v`, got, want)
	}
	// Unknown team name: teamNameToRGB's own documented gray fallback, not
	// a special case here.
	gray := [3]int{128, 128, 128}
	if got := app.PreviewColor("team:NePasExister"); got != gray {
		t.Fatalf(`PreviewColor of an unknown team must fall back to gray like teamNameToRGB itself, got %v`, got)
	}
}

// TestDevPreviewColor_GeneralRoleUsesCurrentTheme pins the "general" (and
// default/unrecognised) branch: ambianceThemeColor()'s OWN live resolution,
// never a duplicated copy of its logic — a themed question changes what
// PreviewColor("general") returns, and no question at all falls back to
// white, exactly like ambianceThemeColor's own dedicated coverage
// (TestDevAmbianceThemeColor_ResolutionOrder, ambiance_dev_test.go).
func TestDevPreviewColor_GeneralRoleUsesCurrentTheme(t *testing.T) {
	app := newTestApp(t)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	white := [3]int{255, 255, 255}

	for _, role := range []string{"general", "", "n'importe-quoi"} {
		if got := app.PreviewColor(role); got != white {
			t.Fatalf("PreviewColor(%q) with no live question must be white, got %v", role, got)
		}
	}

	geography := [3]int{0x3b, 0x82, 0xf6}
	app.engine.Ready("q1", &game.Question{ID: "q1", Type: game.QuestionTypeQCM, Category: game.CategoryGeography})
	for _, role := range []string{"general", "", "n'importe-quoi"} {
		if got := app.PreviewColor(role); got != geography {
			t.Fatalf("PreviewColor(%q) must reflect the live question's theme, got %v want %v", role, got, geography)
		}
	}
	// Sanity: this must be the SAME value ambianceThemeColor() itself
	// returns right now — never a second, independently-computed copy.
	if got, want := app.PreviewColor("general"), app.ambianceThemeColor(); got != want {
		t.Fatalf("PreviewColor(\"general\") = %v must equal ambianceThemeColor() = %v exactly", got, want)
	}
}
