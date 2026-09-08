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
	"buzzcontrol/internal/lighting/hue"
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
// SCORE gold/team-colour flicker (Batch C/C2b, contract §2.4/§8.1) — reuses
// the Flash bascule's own pattern (a phase flag + NotifyState() re-derives)
// verbatim, on the credited team's OWN zone instead of "general", and for a
// BOUNDED number of cycles instead of an indefinite toggle. No modification
// to internal/lighting/hue/ at all: the driver keeps receiving ordinary
// State values, unaware anything is blinking (planner-v10-teamcolor-
// changes-20260908-092100.md §2.1).
// ---------------------------------------------------------------------------

// startScoreFlash begins the SCORE flicker for the credited team:
// clamp(points, 1, 6) cycles of lightingFlashOnPhase/-OffPhase (400/400 ms,
// reused — not a second cadence constant), gold then the team's own colour,
// after which the team's light settles on its own colour for the remainder
// of the pulse (contract §2.4 — ScorePulseDuration itself is unchanged,
// 4800 ms = exactly 6 such cycles, so points>=6 fills the whole pulse).
// Called from every NotifyPulse(KindScore, ...) site in main.go, ALONGSIDE
// it, never instead of it — NotifyPulse drives the "general" zone's own
// COMET scene (unchanged, §2.5), this drives only the credited team's zone.
//
// A newer call (a second score arriving before the first's flicker ends)
// supersedes the running one — same "last one wins" rule as the pulse
// register itself (contract §4.2) — by cancelling its context; team=""
// is a no-op (nothing to celebrate, contract §2.3's "0 pour tout genre
// autre que SCORE" case reached defensively).
func (a *App) startScoreFlash(team string, points int) {
	if team == "" {
		return
	}
	cycles := points
	if cycles < 1 {
		cycles = 1
	}
	if cycles > 6 {
		cycles = 6
	}
	a.ambianceMu.Lock()
	if cancel := a.scoreFlashCancel; cancel != nil {
		cancel()
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	a.scoreFlashCancel = cancel
	epoch := a.scoreFlashEpoch.Add(1)
	// code-reviewer (Batch C, MINEUR 1): Store belongs INSIDE the critical
	// section, not after Unlock — two SCORE events for two different teams
	// arriving a few instructions apart (engine callbacks are not
	// serialised, contract lighting.md §5) could otherwise interleave their
	// two Store calls out of order relative to their two critical sections,
	// leaving scoreFlashTeam naming the STALE team even though the newer
	// (higher-epoch) goroutine is the one actually running. Publishing the
	// team name atomically with cancel/epoch under the same lock makes the
	// whole start sequence indivisible with respect to a concurrent call.
	a.scoreFlashTeam.Store(team)
	a.ambianceMu.Unlock()
	go a.runScoreFlash(ctx, cycles, epoch)
}

// runScoreFlash is the flicker loop itself (see startScoreFlash). Stops the
// instant ctx is cancelled — by a newer SCORE pulse superseding it (via
// startScoreFlash), or by a.cancelCtx() at server shutdown — never left
// flashing gold after the process exits or a later score arrives.
//
// epoch guards the natural end-of-run tail (clearing scoreFlashTeam once all
// cycles are done) against a race with a BRAND NEW flicker that started in
// the narrow window between this goroutine's last timer firing and it
// reaching that tail: if scoreFlashEpoch has moved on, a newer flicker is
// already live and this stale goroutine must touch nothing further.
func (a *App) runScoreFlash(ctx context.Context, cycles int, epoch int64) {
	for i := 0; i < cycles; i++ {
		a.scoreFlashPhaseGold.Store(true)
		a.ambiance().NotifyState()
		select {
		case <-ctx.Done():
			return
		case <-time.After(lightingFlashOnPhase):
		}
		a.scoreFlashPhaseGold.Store(false)
		a.ambiance().NotifyState()
		select {
		case <-ctx.Done():
			return
		case <-time.After(lightingFlashOffPhase):
		}
	}
	// Same critical section as startScoreFlash's own team publication (code-
	// reviewer, Batch C, MINEUR 1): checking scoreFlashEpoch and clearing
	// scoreFlashTeam must be atomic with respect to a brand new
	// startScoreFlash call landing in the narrow window between the check
	// and the clear — otherwise this goroutine's natural end-of-run could
	// wipe out a newer flicker's just-published team name.
	a.ambianceMu.Lock()
	if a.scoreFlashEpoch.Load() == epoch {
		a.scoreFlashTeam.Store("")
	}
	a.ambianceMu.Unlock()
	a.ambiance().NotifyState() // harmless even if a newer flicker is now live: always re-derives from current state, never buffers (contract §4.1)
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
// ⚠️ Review fix (code-reviewer, v10 Batch 2, MAJEUR 1): a first version of
// this function wrote the forced OFF directly through a.LightingDriver()
// while the writer's goroutine — and, if engaged, the Flash blink goroutine
// (runLightingFlash, ticking every ~400 ms) — were STILL ALIVE in this
// pre-cancelCtx() window. A late tick could call NotifyState() and the
// writer could issue one more Apply AFTER the forced OFF, RE-LIGHTING the
// room — exactly the "pire cas" the contract names ("du code présent, une
// suite de tests verte, et la salle qui reste allumée").
//
// Fix, in order:
//  1. a.ambiance().SetDriver(nil) — SYNCHRONOUSLY disables the writer
//     (enabled=false, so every further Notify* is inert per contract §4.3)
//     AND closes whatever driver it currently holds. Close() takes that
//     driver's own opMu (internal/lighting/hue/driver.go), the same lock
//     Apply() holds for its entire duration — so this BLOCKS until any
//     Apply already in flight (e.g. a Flash tick that started a moment
//     earlier) has fully finished, and marks the driver `closed` before
//     returning. Every Apply call on THAT driver from then on — including
//     one the writer's goroutine might still attempt a moment later — sees
//     `closed` and returns immediately with an error, WITHOUT ever making
//     another HTTP request. This is a hard guarantee, not a timing hope.
//  2. The Flash goroutine is cancelled too (belt-and-suspenders — with the
//     writer disabled its ticks are already inert via NotifyState's own
//     `!enabled` guard, but there is no reason to leave it running).
//  3. A FRESH, independent *hue.Driver is built from the current config
//     (buildHueDriver, ambiance.go — no network I/O yet) for the actual
//     extinguishing write. It is never a.LightingDriver(): that field still
//     names the instance just closed in step 1, permanently unusable now.
//
// After step 1 returns, nothing already attached to the (old) driver can
// write again — the fresh driver built in step 3 is therefore provably the
// LAST writer of Hue state before the process exits.
//
// Own short-lived context (never a.ctx, contract §10.4 point 2); never
// blocks or retries if the bridge doesn't answer — an unreachable bridge at
// shutdown is a normal case, at most one log line (point 3).
const shutdownLightingTimeout = 2 * time.Second

func (a *App) shutdownExtinguishHueLighting() {
	a.ambiance().SetDriver(nil) // step 1 — nil-safe, synchronously quiesces the old driver

	a.ambianceMu.Lock()
	cancel := a.lightingFlashCancel
	a.lightingFlashCancel = nil
	a.lightingFlashOn.Store(false)
	a.ambianceMu.Unlock()
	if cancel != nil {
		cancel() // step 2
	}

	d := a.buildHueDriver() // step 3 — fresh instance, config.json unchanged
	if d == nil {
		return
	}
	defer d.Close()
	ctx, cancelTimeout := context.WithTimeout(context.Background(), shutdownLightingTimeout)
	defer cancelTimeout()
	off := lighting.State{Zones: []lighting.ZoneState{{Zone: lighting.ZoneGeneral, Intensity: 0}}}
	seen := map[string]bool{}
	for _, l := range config.Get().Lighting.Lights {
		if l.Role == string(hue.RoleTeam) && l.Team != "" && !seen[l.Team] {
			seen[l.Team] = true
			off.Zones = append(off.Zones, lighting.ZoneState{Zone: l.Team, Intensity: 0})
		}
	}
	if err := d.Apply(ctx, off); err != nil {
		server.LogInfo(game.LogComponentApp, "Ambiance: bridge unreachable at shutdown, lights left as-is: %v", err)
	}
}
