package ambiance

import (
	"context"
	"errors"
	"strings"
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
type Mode string

const (
	ModeAuto Mode = "AUTO"
	ModeOn   Mode = "ON"
	ModeOff  Mode = "OFF"
)

// lightingOnColor/-Intensity is the ON position's forced scene: full
// intensity, neutral white (contract §10.1 table — "forcée à pleine
// intensité, blanc neutre").
var OnColor = [3]int{255, 255, 255}

const OnIntensity = 255

// lightingFlashColor and the two blink phase durations drive the Flash
// bascule (contract §10.1.2 / "Implémentation du clignotement (Flash)") —
// SERVER-driven: an onlooker's closed tab must never leave the room
// blinking. 400 ms/400 ms is a plainly visible, unhurried blink — nothing
// in the contract fixes an exact cadence, so this is a named constant, not
// a literal, ready to be recalibrated without touching the loop itself.
var FlashColor = [3]int{255, 255, 255}

const (
	FlashOnPhase  = 400 * time.Millisecond
	FlashOffPhase = 400 * time.Millisecond
)

// scoreFlashGoldPhase/-TeamPhase drive the SCORE flicker's own cadence
// (planner-v10-score-rhythm-20260908-105438.md §1.1, confirmed by the user
// 2026-09-08) — DEDICATED constants, never lightingFlashOnPhase/-OffPhase:
// the two blinks read differently on purpose (a sharp 250 ms gold burst,
// the room "claque" instead of "respire", against a longer 500 ms rest on
// the team's own colour — asymmetric 1:2, not the Flash bascule's even
// 400/400). 250 ms is the hard floor, not a stylistic choice: the Hue
// driver paces every Apply at hue.RecommendedMinInterval (250 ms, contract
// hue-bridge.md §5.4) — a shorter phase would be delayed or silently
// absorbed, so what would actually render in the room stops matching what
// was scheduled.
const (
	ScoreFlashGoldPhase = 250 * time.Millisecond
	ScoreFlashTeamPhase = 500 * time.Millisecond
)

// flashPhaseOverride lets a test shorten the blink cadences of ONE App (zero
// field = production constant). #291, Phase A.
type FlashPhaseOverride struct {
	LightingOn, LightingOff, ScoreGold, ScoreTeam time.Duration
}

func OrDefault(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

// lightingMode reads the current selector position. AUTO is the zero value
// (an unset atomic.Value.Load() returns nil, which the type assertion below
// turns into "") — contract §10.1.1 point 5 needs no explicit
// initialisation at server start for AUTO to hold.
func (c *Controller) lightingMode() Mode {
	v, _ := c.lightingModeState.Load().(Mode)
	if v == "" {
		return ModeAuto
	}
	return v
}

// setLightingMode sets the selector and immediately re-derives the room —
// the SAME NotifyState() path as every other ambiance transition (contract
// §10.1.1 point 3: "aucune mémorisation de la scène d'avant, tout est
// re-dérivé").
func (c *Controller) SetMode(m Mode) {
	c.lightingModeState.Store(m)
	c.Writer().NotifyState()
}

// isLightingFlashOn reports whether Flash is currently engaged.
func (c *Controller) isLightingFlashOn() bool {
	return c.LightingFlashOn.Load()
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
func (c *Controller) SetFlash(on bool) {
	c.ambianceMu.Lock()
	if on == c.LightingFlashOn.Load() {
		c.ambianceMu.Unlock()
		return
	}
	c.LightingFlashOn.Store(on)
	if on {
		parent := c.h.Ctx()
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithCancel(parent)
		c.lightingFlashCancel = cancel
		c.ambianceMu.Unlock()
		go c.runLightingFlash(ctx)
		return
	}
	cancel := c.lightingFlashCancel
	c.lightingFlashCancel = nil
	c.ambianceMu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.Writer().NotifyState() // back to the selector's own position
}

// runLightingFlash is the Flash blink loop (contract §10.1.2): alternates
// the phase lightingOverrideGeneral reads and re-derives on every tick —
// white (lit phase) alternating with the room's own current colour (2026-
// 09-08 revision, see lightingOverrideGeneral's own doc comment; NOT a
// dark/off phase any more). Stops the instant ctx is cancelled — by
// setLightingFlash(false), or by a.cancelCtx() at server shutdown
// ((*App).stop(), main.go) — never left blinking after a browser tab closes
// or the process exits.
func (c *Controller) runLightingFlash(ctx context.Context) {
	phase := true // start lit — an operator pressing Flash wants to SEE it fire
	for {
		c.LightingFlashPhaseOn.Store(phase)
		c.Writer().NotifyState()
		wait := OrDefault(c.FlashPhases.LightingOn, FlashOnPhase)
		if !phase {
			wait = OrDefault(c.FlashPhases.LightingOff, FlashOffPhase)
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
//
// 2026-09-08 revision: Flash's un-lit phase used to be a dark/off state
// ({0,0,0}, 0) — it now alternates white with the room's own CURRENT
// colour, at full intensity throughout, so an operator identifying the
// installation always sees two lit, distinguishable phases rather than a
// blink to black. "The room's own colour" reuses ambianceThemeColor()
// verbatim (never autoColor, the scene ALREADY on screen when Flash was
// engaged — SCORE's own team colour or ENTRACTE's warm white, for
// instance, are not what "la couleur de la salle" means here) — the exact
// same resolution POST /api/lighting/preview's "general" role already
// uses (PreviewColor, above), never a second copy of the logic. A
// themeless room (no question active, or no category) degenerates to
// white/white, which is an accepted, unremarkable case, not a special one.
func (c *Controller) OverrideGeneral(autoColor [3]int, autoIntensity int) ([3]int, int) {
	if c.isLightingFlashOn() {
		if c.LightingFlashPhaseOn.Load() {
			return FlashColor, OnIntensity
		}
		return c.ThemeColor(), OnIntensity
	}
	switch c.lightingMode() {
	case ModeOn:
		return OnColor, OnIntensity
	case ModeOff:
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
// clamp(ceil(points/5), 1, 6) cycles of scoreFlashGoldPhase/-TeamPhase
// (250/500 ms), gold then the team's own colour, after which the team's
// light settles on its own colour for the remainder of the pulse. Grouping
// by 5 points per pulsation (planner-v10-score-rhythm-20260908-105438.md
// §1.2, confirmed by the user 2026-09-08): 1-5 points ⇒ 1 pulsation, 6-10 ⇒
// 2, ..., 30+ ⇒ 6 (ceiling, never integer division — a plain points/5 would
// render the single most common score, 1 point, as ZERO pulsations).
//
// ScorePulseDuration stays FIXED at 4800 ms regardless of the cycle count —
// an explicit, confirmed user choice against the planner's own suggestion
// of a variable N×750ms duration: a 1-5 point score deliberately renders as
// one 750 ms pulsation followed by ~4 s of the team's own colour before the
// pulse ends, rather than a shorter celebration. Do not "fix" this into a
// variable duration without a new decision — see the report referenced
// above for the tradeoff as stated to the user.
//
// Called from every NotifyPulse(KindScore, ...) site in main.go, ALONGSIDE
// it, never instead of it — NotifyPulse drives the "general" zone's own
// COMET scene (unchanged, §2.5), this drives only the credited team's zone.
//
// A newer call (a second score arriving before the first's flicker ends)
// supersedes the running one — same "last one wins" rule as the pulse
// register itself (contract §4.2) — by cancelling its context; team=""
// is a no-op (nothing to celebrate, contract §2.3's "0 pour tout genre
// autre que SCORE" case reached defensively).
func (c *Controller) StartScoreFlash(team string, points int) {
	if team == "" {
		return
	}
	// ceil(points/5) via integer arithmetic — no float round-trip — then
	// clamped to [1, 6]. points<=0 (defensive: every real call site already
	// gates on points>0) falls through to the floor of 1.
	cycles := 1
	if points > 0 {
		cycles = (points + 4) / 5
	}
	if cycles > 6 {
		cycles = 6
	}
	c.ambianceMu.Lock()
	if cancel := c.scoreFlashCancel; cancel != nil {
		cancel()
	}
	parent := c.h.Ctx()
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	c.scoreFlashCancel = cancel
	epoch := c.scoreFlashEpoch.Add(1)
	// code-reviewer (Batch C, MINEUR 1): Store belongs INSIDE the critical
	// section, not after Unlock — two SCORE events for two different teams
	// arriving a few instructions apart (engine callbacks are not
	// serialised, contract lighting.md §5) could otherwise interleave their
	// two Store calls out of order relative to their two critical sections,
	// leaving scoreFlashTeam naming the STALE team even though the newer
	// (higher-epoch) goroutine is the one actually running. Publishing the
	// team name atomically with cancel/epoch under the same lock makes the
	// whole start sequence indivisible with respect to a concurrent call.
	c.scoreFlashTeam.Store(team)
	c.ambianceMu.Unlock()
	go c.runScoreFlash(ctx, cycles, epoch)
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
func (c *Controller) runScoreFlash(ctx context.Context, cycles int, epoch int64) {
	for i := 0; i < cycles; i++ {
		c.scoreFlashPhaseGold.Store(true)
		c.Writer().NotifyState()
		select {
		case <-ctx.Done():
			return
		case <-time.After(OrDefault(c.FlashPhases.ScoreGold, ScoreFlashGoldPhase)):
		}
		c.scoreFlashPhaseGold.Store(false)
		c.Writer().NotifyState()
		select {
		case <-ctx.Done():
			return
		case <-time.After(OrDefault(c.FlashPhases.ScoreTeam, ScoreFlashTeamPhase)):
		}
	}
	// Same critical section as startScoreFlash's own team publication (code-
	// reviewer, Batch C, MINEUR 1): checking scoreFlashEpoch and clearing
	// scoreFlashTeam must be atomic with respect to a brand new
	// startScoreFlash call landing in the narrow window between the check
	// and the clear — otherwise this goroutine's natural end-of-run could
	// wipe out a newer flicker's just-published team name.
	c.ambianceMu.Lock()
	if c.scoreFlashEpoch.Load() == epoch {
		c.scoreFlashTeam.Store("")
	}
	c.ambianceMu.Unlock()
	c.Writer().NotifyState() // harmless even if a newer flicker is now live: always re-derives from current state, never buffers (contract §4.1)
}

// ---------------------------------------------------------------------------
// LightingProvider (internal/server/http_lighting.go) — mode/flash surface
// for POST /api/lighting/mode and /api/lighting/flash.
// ---------------------------------------------------------------------------

// LightingMode returns the selector's current position as a string.
func (c *Controller) LightingMode() string {
	return string(c.lightingMode())
}

// SetLightingMode validates and applies a new selector position. The string
// boundary (rather than the lightingMode type) is what LightingProvider can
// expose without internal/server importing package main.
func (c *Controller) SetLightingMode(mode string) error {
	switch Mode(mode) {
	case ModeOn, ModeAuto, ModeOff:
		c.SetMode(Mode(mode))
		return nil
	default:
		return errLightingInvalidMode
	}
}

// LightingFlash reports whether Flash is currently engaged.
func (c *Controller) LightingFlash() bool {
	return c.isLightingFlashOn()
}

// SetLightingFlash engages/disengages Flash.
func (c *Controller) SetLightingFlash(on bool) {
	c.SetFlash(on)
}

// previewTeamRolePrefix is the "team:<name>" encoding POST /api/lighting/
// preview's role field uses (contract hue-bridge.md §7, 2026-09-08) — the
// row's currently-SELECTED role in the admin UI, not yet saved.
const previewTeamRolePrefix = "team:"

// PreviewColor implements LightingProvider (internal/server/http_lighting.go):
// resolves what colour POST /api/lighting/preview should light a bulb with
// for the given role. "team:<name>" -> that team's own palette colour
// (teamNameToRGB, main.go — the SAME palette as everywhere else, never a
// second one); anything else (empty, "general", or unrecognised — the safe
// default) -> the general zone's own CURRENT colour, reusing
// ambianceThemeColor() verbatim rather than duplicating its resolution
// order (live question theme, white if none/unknown/custom category or no
// question at all — contract lighting.md §8.2's own doc comment).
func (c *Controller) PreviewColor(role string) [3]int {
	if team, ok := strings.CutPrefix(role, previewTeamRolePrefix); ok {
		return c.h.TeamNameColor(team)
	}
	return c.ThemeColor()
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

func (c *Controller) ShutdownExtinguish() {
	c.quiesceAmbiance()
	c.extinguishFreshDriver(true) // historical behaviour: a switched-off installation is left untouched
}

// quiesceAmbiance is steps 1 and 2: detach the writer's driver
// synchronously and cancel the Flash goroutine.
func (c *Controller) quiesceAmbiance() {
	c.Writer().SetDriver(nil) // step 1 — nil-safe, synchronously quiesces the old driver

	c.ambianceMu.Lock()
	cancel := c.lightingFlashCancel
	c.lightingFlashCancel = nil
	c.LightingFlashOn.Store(false)
	c.ambianceMu.Unlock()
	if cancel != nil {
		cancel() // step 2
	}
}

// extinguishOnLightingDisabled is the `lighting.enabled=false` gesture of the
// master switch (#286, contract sound.md §11.4): same extinction as the
// shutdown (§10.4) — general + team bulbs — then the writer stays detached
// (OnConfigUpdate's reconfigureAmbiance finds no usable config). The detach is
// synchronous (no further command can reach the bulbs after this returns);
// the network write runs in the background so the HTTP response is not held
// for the bridge timeout.
func (c *Controller) ExtinguishOnDisabled() {
	c.quiesceAmbiance()
	go func() {
		// Re-enabled meanwhile (fast off→on): the bulbs must not be switched
		// off behind the new driver's back — skip entirely.
		if config.Get().Lighting.Enabled {
			return
		}
		c.extinguishFreshDriver(false)
		// A re-enable landing during the write: re-derive the live scene so the
		// last word is the game's, never the extinction (nil-safe, no-op if off).
		c.Writer().NotifyState()
	}()
}

// extinguishFreshDriver is step 3: a fresh driver (config.json unchanged)
// writes intensity 0 to the general zone and every team zone. requireEnabled
// is true at server shutdown (a `lighting.enabled=false` installation must
// not be touched) and false when the master switch has just been turned off.
func (c *Controller) extinguishFreshDriver(requireEnabled bool) {
	d := c.BuildHueDriverOpt(requireEnabled)
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
