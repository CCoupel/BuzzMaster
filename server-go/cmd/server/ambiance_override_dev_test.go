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

	// Coupure du pont EN COURS DE PARTIE.
	bridge.setUp(false)
	app.engine.SetPhase(game.PhasePaused) // un événement de jeu survient PENDANT la coupure
	app.ambiance().NotifyState()
	devWaitDriverState(t, app.LightingDriver(), hue.StateUnreachable)
	countDuringOutage := bridge.count()

	// La partie continue de bouger pendant la coupure — la salle ne
	// bouge PAS (contract §5.5 : "la partie continue sans latence
	// perceptible", l'écriture échoue en silence côté salle).
	app.engine.SetPhase(game.PhaseStarted)
	app.ambiance().NotifyState()

	// Rétablissement, observé par un geste normal (inventaire).
	bridge.setUp(true)
	if err := app.LightingDriver().RefreshInventory(context.Background()); err != nil {
		t.Fatalf("RefreshInventory after recovery: %v", err)
	}
	devWaitDriverState(t, app.LightingDriver(), hue.StateOK)

	// §10.3 : la resync doit produire un NOUVEL Apply, réappliquant l'état
	// de jeu VIVANT (toujours PhaseStarted sans équipe active ⇒
	// KindRunning, allumé) — pas l'état d'avant la coupure rejoué tel quel.
	devWaitPutCount(t, bridge, countDuringOutage+1)
	if !strings.Contains(bridge.last(), `"on":true`) {
		t.Fatalf("resync AUTO must re-derive a lit KindRunning scene from the live game state, got %q", bridge.last())
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
