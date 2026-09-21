// Sound bruitage adapter (#227, contracts/sound.md). This file is the ONLY
// bridge between the game (cmd/server) and internal/audio:
//
//   - the site registry (§6.2): every enclosing function that calls the
//     fan-out entry point notifySound, with its decision — checked by the
//     AST exhaustiveness test (cmd/server/sound_sites_test.go, Lot C,
//     symmetric to ambiance_sites_test.go / contract lighting.md §7);
//   - notifySound itself (§5.1/§6.1): the SINGLE sound-only entry point.
//     Deliberately NOT a unified "does both light and sound" wrapper — the
//     22 existing a.ambiance().NotifyState()/NotifyPulse() call sites are
//     untouched; notifySound is an ADDITIONAL, sound-only call added at the
//     handful of sites that also carry a cue. Simpler, and exactly what the
//     AST test (single selector "notifySound") verifies;
//   - front detection (§5.4, plan-verif-front-227-20260921-103500.md): the
//     ONE stateful bit the sound layer needs beyond the live GameState, to
//     tell a real START from a resume-after-pause — onPhaseStarted() is
//     reached from THREE engine transitions (actualStart, StartImmediate,
//     Continue) but only the first two should ever sound `depart`;
//   - the lifecycle (§5.6): construction in setup(), goroutine in start().
//
// Founding rule, symmetric to ambiance.go's own (contract §1): sound is
// wired on the EVENT layer, never inferred from polling or from the
// lighting rendering layer.
package main

import (
	"sync"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
)

// ---------------------------------------------------------------------------
// Site registry (contract §6.2)
// ---------------------------------------------------------------------------

// soundSite identifies one fan-out call site by its enclosing function name
// — stable across edits, unlike line numbers (same reasoning as
// contracts/lighting.md §7). Unlike ambianceSite, there is no second field:
// the sound fan-out has exactly ONE entry point (notifySound), not a family
// of sendLEDSet* selectors, so the function name alone disambiguates.
type soundSite struct {
	Func string
}

// soundDecision documents WHY this function is a legitimate sound site —
// which cue(s) it plays and under what condition. Several notifySound
// calls inside the SAME enclosing function, on different branches for
// different cues, collapse to ONE registry entry — the registry documents
// the FUNCTION, not every branch (same accepted limit as contract
// lighting.md §6.3/§7, TestSoundSites_DuplicateCallInSameFuncCollapsesToOneSite).
type soundDecision struct {
	Reason string
}

// soundSiteRegistry is the CLOSED list of functions that call notifySound.
// Adding a notifySound call in a new function without an entry here fails
// cmd/server/sound_sites_test.go — on purpose: one forgotten entry means
// either a stale registry or an unreviewed new sound site.
var soundSiteRegistry = map[soundSite]soundDecision{
	{Func: "handlePoints"}:       {"points awarded — credited team (CueGagne)"},
	{Func: "handleBumperPoints"}: {"bumper points (CueGagne)"},
	{Func: "handleTeamPoints"}:   {"team points (CueGagne)"},
	{Func: "handleMotionDone"}:   {"MEMOTION winner (CueGagne) — the auto-stop branch carries no cue"},
	{Func: "handleFlipMemoryCard"}: {"flip-back branch only (CuePerdu) — match/complete branches carry no cue; " +
		"same per-function collapsing limit as contract lighting.md §6.3/§7, cannot be narrowed further by this test"},
	{Func: "broadcastReveal"}: {"answer revealed (CueReveal)"},
	{Func: "handleEntracteSet"}: {"CueEntracteDebut / CueEntracteFin depending on payload.Active — " +
		"manual toggle, idempotence guard already present (contract lighting.md §10.5 precedent)"},
	{Func: "onPhaseStarted"}: {"CueDepart / CueEntracteDebut / silence, front-detected against the phase " +
		"BEFORE this transition (soundTrackPhase) — a resume-after-PAUSE (Continue()) must never sound " +
		"depart again; see _work/reports/plan-verif-front-227-20260921-103500.md"},
	{Func: "setupCallbacks"}: {"OnRafaleInvalid (CuePerdu, classic RAFALE invalidate/timeout only, never the " +
		"MEMOTION-card-scoped variant) and OnTimeUp (CueTempsEcoule) callbacks wired here"},
}

// ---------------------------------------------------------------------------
// Lifecycle (contract §5.6)
// ---------------------------------------------------------------------------

// sound reads the live sound engine. Never nil after setup() (contract §5.5
// handles "not configured" internally via Enabled()==false, exactly like
// a.ambiance() handles a nil *lighting.Writer) — but Load()'s zero value on
// an untouched atomic.Pointer IS nil, and every Engine method is documented
// safe on a nil receiver, so callers never need a guard either way. Mirrors
// a.ambiance()'s exact role for a.lightingWriter.
func (a *App) sound() *audio.Engine {
	return a.soundEngine.Load()
}

// newSoundEngine builds an Engine wired to output — mirrors
// a.newAmbianceWriter(drv)'s exact role for the lighting writer. output nil
// (setupSound's production default until #228) constructs a disabled
// engine (audio.NewEngine's own contract).
func (a *App) newSoundEngine(output audio.Output) *audio.Engine {
	return audio.NewEngine(audio.Config{Output: output})
}

// buildAudioOutput builds the real Output from the current configuration,
// or nil when sound is disabled — mirrors a.buildHueDriver()'s exact role
// for the lighting driver (contract §5.5: `Enabled: false` ⇒ Output nil ⇒
// audio.NewEngine constructs a fully disabled engine, no goroutine at all,
// contrast with "enabled but hardware unreachable" below, which DOES run
// the goroutine — same distinction lighting already makes for an
// unconfigured vs. an unreachable bridge).
//
// audio.NewOutput itself NEVER hard-fails (contract §5.5's construction-
// time degradation) — a missing audio subsystem, permission error, or a
// context that never becomes ready all degrade to a silent no-op Output
// instead of an error, so this never blocks setup() or start().
func (a *App) buildAudioOutput() audio.Output {
	sc := config.Get().Sound
	if !sc.Enabled {
		return nil
	}
	return audio.NewOutput(audio.OutputConfig{Device: sc.Device})
}

// setupSound builds the sound engine from configuration — real Output
// (#228) when `sound.enabled` is true, nil (fully disabled, contract
// §5.5) otherwise.
func (a *App) setupSound() {
	a.soundEngine.Store(a.newSoundEngine(a.buildAudioOutput()))
}

// startSoundEngine starts the engine's playback goroutine — same lifecycle
// as AckManager and the ambiance writer (contract §5.6), stopped by
// a.cancelCtx() in stop(). `sound.enabled: false` ⇒ launches nothing
// (contract §5.5).
func (a *App) startSoundEngine() {
	go a.sound().Start(a.ctx)
}

// ---------------------------------------------------------------------------
// Fan-out (contract §5.1/§6.1)
// ---------------------------------------------------------------------------

// notifySound is the sound fan-out's SINGLE entry point (contract §5.1):
// game-event sites call this unconditionally, never audio.Engine.PlayCue
// directly, and never guard it with an `if` — safe on a disabled engine,
// exactly like a.ambiance().NotifyState() is safe on a disabled writer
// (contract lighting.md §4.3). This is what keeps
// cmd/server/sound_sites_test.go's registry lisible et vérifiable: ONE
// selector name to scan for, contract §6.2.
func (a *App) notifySound(c audio.Cue) {
	a.sound().PlayCue(c)
}

// ---------------------------------------------------------------------------
// Front detection (contract §5.4, plan-verif-front-227-20260921-103500.md)
// ---------------------------------------------------------------------------

// soundPhaseTracker is the ONE stateful bit the sound layer needs beyond
// the live GameState: OnStateChange only ever carries the NEW phase, never
// the previous one, and "a real start" (actualStart/StartImmediate) is
// otherwise indistinguishable from "a resume after a buzz pause"
// (Continue()) — both call onPhaseStarted() with the identical new phase
// PhaseStarted. A mutex-guarded field, nothing more (verification report's
// own conclusion: no generic machinery needed) — deliberately NOT reusing
// OnStateChange's signature or any existing App field (both would leak a
// sound-only concern into code shared by everything else).
//
// Engine.OnStateChange (main.go) is invoked hors lock depuis une douzaine
// de sites sans aucune sérialisation (internal/game/engine.go:220-241,
// cause racine #121, contract lighting.md §5) — this tracker's own mutex is
// what makes reading-then-updating it safe under that constraint, same
// discipline as lighting.Writer's refreshDue/pulse mutex.
type soundPhaseTracker struct {
	mu   sync.Mutex
	last game.GamePhase
}

// soundTrackPhase records phase as the new "last known phase" and returns
// what was there immediately before — called ONCE per OnStateChange
// invocation (every phase, not just STARTED), from the same closure in
// main.go, so the tracker always reflects the phase the game was in just
// before the CURRENT transition.
func (a *App) soundTrackPhase(phase game.GamePhase) (previous game.GamePhase) {
	a.soundPhase.mu.Lock()
	previous = a.soundPhase.last
	a.soundPhase.last = phase
	a.soundPhase.mu.Unlock()
	return previous
}
