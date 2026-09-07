package main

// Manual override on the "general" ambiance zone, and shutdown extinction
// (#208, contracts/lighting.md §10 — reprise du milestone v10.0.0 du
// 2026-09-07, `_work/reports/planner-v10-etat-courant-20260907.md`).
//
// Two controls, and two only (§10.1):
//   - a tri-state selector ON | AUTO | OFF, exclusive by construction;
//   - a separate Flash bascule, which PRIMES over the selector while
//     engaged, WITHOUT moving it.
//
// Both are SERVER state (never the browser's — two admins see the same
// position), not persisted (§10.1.1 point 5/6: AUTO at every server start),
// and apply ONLY to the "general" zone — the per-team zones added by #213
// always follow the game, whatever the selector says (§10.1, "Portée").
//
// AUTO's return to the game and the reconnection resync of §10.3 are
// deliberately the SAME code path: lightingOverrideGeneral is consulted by
// ambianceScene on every re-derivation, so there is nothing to "restore"
// when a mode ends — the room is simply re-derived from whatever
// (mode, live game state) says right now (§10.1.1 point 3).

import (
	"context"
	"errors"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
	"buzzcontrol/internal/server"
)

// errLightingInvalidMode is returned by SetLightingMode for anything other
// than ON/AUTO/OFF — the HTTP handler turns it into 400.
var errLightingInvalidMode = errors.New("lighting: mode must be ON, AUTO or OFF")

// lightingMode is the tri-state selector's value (contract §10.1 table).
type lightingMode string

const (
	lightingModeAuto lightingMode = "AUTO"
	lightingModeOn   lightingMode = "ON"
	lightingModeOff  lightingMode = "OFF"
)

// lightingOnColor/-Intensity is the ON position's forced scene: full
// intensity, neutral white (contract §10.1 table — "forcée à pleine
// intensité, blanc neutre").
var lightingOnColor = [3]int{255, 255, 255}

const lightingOnIntensity = 255

// lightingFlashColor and the two blink phase durations drive the Flash
// bascule (contract §10.1.2 / "Implémentation du clignotement (Flash)") —
// SERVER-driven: an onlooker's closed tab must never leave the room
// blinking. 400 ms/400 ms is a plainly visible, unhurried blink — nothing
// in the contract fixes an exact cadence, so this is a named constant, not
// a literal, ready to be recalibrated without touching the loop itself.
var lightingFlashColor = [3]int{255, 255, 255}

const (
	lightingFlashOnPhase  = 400 * time.Millisecond
	lightingFlashOffPhase = 400 * time.Millisecond
)

// lightingMode reads the current selector position. AUTO is the zero value
// (an unset atomic.Value.Load() returns nil, which the type assertion below
// turns into "") — contract §10.1.1 point 5 needs no explicit
// initialisation at server start for AUTO to hold.
func (a *App) lightingMode() lightingMode {
	v, _ := a.lightingModeState.Load().(lightingMode)
	if v == "" {
		return lightingModeAuto
	}
	return v
}

// setLightingMode sets the selector and immediately re-derives the room —
// the SAME NotifyState() path as every other ambiance transition (contract
// §10.1.1 point 3: "aucune mémorisation de la scène d'avant, tout est
// re-dérivé").
func (a *App) setLightingMode(m lightingMode) {
	a.lightingModeState.Store(m)
	a.ambiance().NotifyState()
}

// isLightingFlashOn reports whether Flash is currently engaged.
func (a *App) isLightingFlashOn() bool {
	return a.lightingFlashOn.Load()
}

// setLightingFlash engages/disengages Flash (contract §10.1.2). Engaging
// starts a dedicated server-side blink goroutine (never the browser's
// responsibility), scoped to the APPLICATION context so a server shutdown
// (a.cancelCtx(), in (*App).stop()) stops it exactly like every other
// ctx-aware background component — no separate cleanup step needed for
// "never left blinking after the process exits" to hold. Falls back to
// context.Background() when a.ctx is nil (minimal test harnesses only —
// context.WithCancel panics on a nil parent; never nil in production, set
// once in NewApp before setup()). Disengaging stops the goroutine and
// re-derives once more so the room falls back to whatever the selector
// says (ON/OFF forced, or AUTO re-derived from the live game) — same
// NotifyState() call, no separate "restore" path.
func (a *App) setLightingFlash(on bool) {
	a.ambianceMu.Lock()
	if on == a.lightingFlashOn.Load() {
		a.ambianceMu.Unlock()
		return
	}
	a.lightingFlashOn.Store(on)
	if on {
		parent := a.ctx
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithCancel(parent)
		a.lightingFlashCancel = cancel
		a.ambianceMu.Unlock()
		go a.runLightingFlash(ctx)
		return
	}
	cancel := a.lightingFlashCancel
	a.lightingFlashCancel = nil
	a.ambianceMu.Unlock()
	if cancel != nil {
		cancel()
	}
	a.ambiance().NotifyState() // back to the selector's own position
}

// runLightingFlash is the Flash blink loop (contract §10.1.2): alternates
// the phase lightingOverrideGeneral reads and re-derives on every tick.
// Stops the instant ctx is cancelled — by setLightingFlash(false), or by
// a.cancelCtx() at server shutdown ((*App).stop(), main.go) — never left
// blinking after a browser tab closes or the process exits.
func (a *App) runLightingFlash(ctx context.Context) {
	phase := true // start lit — an operator pressing Flash wants to SEE it fire
	for {
		a.lightingFlashPhaseOn.Store(phase)
		a.ambiance().NotifyState()
		wait := lightingFlashOnPhase
		if !phase {
			wait = lightingFlashOffPhase
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		phase = !phase
	}
}

// lightingOverrideGeneral applies the selector/Flash override to the
// auto-derived "general" zone colour/intensity — the ONLY place either
// control acts (contract §10.1 "Portée" — team zones, #213, are never
// touched). Called from ambianceScene on every re-derivation, so ON/OFF/
// Flash are always exactly "ce que l'éclairage doit montrer maintenant":
// one chemin de calcul, shared with the reconnection resync of §10.3 and
// with AUTO's own return (§10.1.1).
func (a *App) lightingOverrideGeneral(autoColor [3]int, autoIntensity int) ([3]int, int) {
	if a.isLightingFlashOn() {
		if a.lightingFlashPhaseOn.Load() {
			return lightingFlashColor, lightingOnIntensity
		}
		return [3]int{0, 0, 0}, 0
	}
	switch a.lightingMode() {
	case lightingModeOn:
		return lightingOnColor, lightingOnIntensity
	case lightingModeOff:
		return [3]int{0, 0, 0}, 0
	default: // AUTO
		return autoColor, autoIntensity
	}
}

// ---------------------------------------------------------------------------
// LightingProvider (internal/server/http_lighting.go) — mode/flash surface
// for POST /api/lighting/mode and /api/lighting/flash.
// ---------------------------------------------------------------------------

// LightingMode returns the selector's current position as a string.
func (a *App) LightingMode() string {
	return string(a.lightingMode())
}

// SetLightingMode validates and applies a new selector position. The string
// boundary (rather than the lightingMode type) is what LightingProvider can
// expose without internal/server importing package main.
func (a *App) SetLightingMode(mode string) error {
	switch lightingMode(mode) {
	case lightingModeOn, lightingModeAuto, lightingModeOff:
		a.setLightingMode(lightingMode(mode))
		return nil
	default:
		return errLightingInvalidMode
	}
}

// LightingFlash reports whether Flash is currently engaged.
func (a *App) LightingFlash() bool {
	return a.isLightingFlashOn()
}

// SetLightingFlash engages/disengages Flash.
func (a *App) SetLightingFlash(on bool) {
	a.setLightingFlash(on)
}

// ---------------------------------------------------------------------------
// Shutdown extinction (contract §10.4)
// ---------------------------------------------------------------------------

// shutdownExtinguishHueLighting turns the Hue lights off at server shutdown
// (contract §10.4) — unlike the #208 override above, this covers EVERY
// configured light, general AND team zones alike: the whole installation
// goes dark, not just the room's general scene. The buzzer half of §10.4
// (sendLEDSetAllEntracteOff) is called directly from (*App).stop() in
// main.go, not from here — the AST exhaustiveness test (contract §7) only
// scans main.go, and every sendLEDSet* call site belongs there for the
// registry (cmd/server/ambiance.go) to track it.
//
// MUST run BEFORE a.cancelCtx() (see (*App).stop()'s own comment for the
// ordering trap this fixes: a.ctx backs both the HTTP client to the bridge
// and the buzzer WebSocket hubs, so an extinction issued after cancelCtx()
// is annulled the instant it is emitted).
//
// The Hue write goes DIRECTLY through the driver's own Apply — not through
// NotifyState()/the writer — deliberately: the writer's goroutine is still
// alive in this narrow pre-cancelCtx() window and could otherwise race this
// forced OFF with a fresh derivation of its own. hue.Driver.Apply is safe
// for this: contracts/lighting.md §5's "only the writer's single goroutine"
// is a permission the driver does not need to rely on, and hue.Driver
// internally serialises Apply against Inventory/TestFlash/Close via its own
// opMu (internal/lighting/hue/driver.go) — a second, direct Apply call is
// exactly what /api/lighting/test already does concurrently with the
// writer today.
//
// Own short-lived context (never a.ctx, contract §10.4 point 2); never
// blocks or retries if the bridge doesn't answer — an unreachable bridge at
// shutdown is a normal case, at most one log line (point 3).
const shutdownLightingTimeout = 2 * time.Second

func (a *App) shutdownExtinguishHueLighting() {
	d := a.LightingDriver()
	if d == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownLightingTimeout)
	defer cancel()
	off := lighting.State{Zones: []lighting.ZoneState{{Zone: lighting.ZoneGeneral, Intensity: 0}}}
	seen := map[string]bool{}
	for _, l := range config.Get().Lighting.Lights {
		if l.Role == "team" && l.Team != "" && !seen[l.Team] {
			seen[l.Team] = true
			off.Zones = append(off.Zones, lighting.ZoneState{Zone: l.Team, Intensity: 0})
		}
	}
	if err := d.Apply(ctx, off); err != nil {
		server.LogInfo(game.LogComponentApp, "Ambiance: bridge unreachable at shutdown, lights left as-is: %v", err)
	}
}
