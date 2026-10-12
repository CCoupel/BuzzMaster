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
	"path/filepath"
	"sync"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/audio/synth"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
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

// newSoundEngine builds an Engine wired to output, WITHOUT a Bank — mirrors
// a.newAmbianceWriter(drv)'s exact role for the lighting writer. Signature
// preserved exactly as #227 left it (Bank-less): test-writer's
// sound_cues_chain_test.go (#227) calls this directly with a FakeOutput and
// asserts on the placeholder cue-name-as-payload behaviour (audio.Bank's
// own doc comment) — changing this signature or its behaviour would break
// that whole suite. Production (setupSound, below) does NOT use this
// helper: it builds the real Engine directly, Bank included.
func (a *App) newSoundEngine(output audio.Output) *audio.Engine {
	return audio.NewEngine(audio.Config{Output: output})
}

// soundsDir is where sound files live on disk — data/files/sounds/, same
// filesDir/fallback convention as backgrounds/entracte (createDemoBackgrounds,
// ensureCategoriesDir). Shared by buildSoundBank below and by the #229
// startup generation / manifest reconciliation (main.go).
func soundsDir(cfg *config.Config) string {
	filesDir := cfg.Storage.FilesDir
	if filesDir == "" {
		filesDir = "./data/files"
	}
	return filepath.Join(filesDir, "sounds")
}

// buildSoundBank builds the real Bank (#229) reading from soundsDir.
func (a *App) buildSoundBank() audio.Bank {
	return audio.NewFileBank(soundsDir(a.config))
}

// createDefaultSounds generates any MISSING default sound (#229, plan de
// dev §2.1/B.4) — modelled structurally on createDemoBackgrounds's
// per-file existence check, but invoked unconditionally at every real
// server startup (setup()), not only when demo data is loaded. Never
// overwrites: an existing file, default or a user's own customised sound
// (#230), is always left untouched — see synth.WriteAll's own doc comment.
// Reconciles data/files/sounds/sounds.json afterwards either way, so it
// always reflects what's actually on disk (plan de cadrage §2.2's "le
// disque fait foi").
func (a *App) createDefaultSounds() {
	dir := soundsDir(a.config)
	written, err := synth.WriteAll(dir, false)
	if err != nil {
		server.LogError(game.LogComponentApp, "Sound: failed to write default sound(s): %v", err)
	}
	if len(written) > 0 {
		server.LogInfo(game.LogComponentApp, "Sound: generated %d default sound(s): %v", len(written), written)
	}
	if _, err := synth.ReconcileManifest(dir); err != nil {
		server.LogWarn(game.LogComponentApp, "Sound: failed to write sounds.json manifest: %v", err)
	}
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
// §5.5) otherwise — WITH the real Bank (#229), so PlayCue resolves actual
// WAV bytes from soundsDir instead of #227's cue-name placeholder. Builds
// audio.Engine directly (not via newSoundEngine, kept Bank-less above for
// test-writer's #227 suite).
func (a *App) setupSound() {
	a.soundEngine.Store(audio.NewEngine(audio.Config{
		Output: a.buildAudioOutput(),
		Bank:   a.buildSoundBank(),
	}))
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
//
// The `enabled` guard below is deliberately asymmetric in effect, NOT in
// code (contract §6.3 "Interaction avec l'interrupteur général", arbitrage
// planner 2026-09-21) — a single early return, read live like CuesDisabled,
// is enough to make BOTH directions correct without ever touching the
// engine's lifecycle:
//   - was disabled at startup ⇒ buildAudioOutput returned nil ⇒ the engine
//     itself is already a no-op (contract §5.5) — flipping `enabled` back
//     to true here changes nothing, because nothing was ever built to
//     un-mute. Re-enabling therefore requires a restart, with NO special
//     case coded for it: it falls out of #227/#228/#229's "built once at
//     setupSound()" design, untouched by this guard.
//   - was enabled at startup ⇒ the engine is built and its goroutine
//     running — flipping `enabled` to false makes this guard return
//     immediately, silencing every cue AT ONCE, with the engine, its
//     goroutine and its queue left exactly as they are (no teardown, no
//     reconstruction: tearing down a live device mid-game was explicitly
//     rejected, limited benefit for real risk).
//
// Never move this into audio.Engine or gate PlayCue itself: /test
// (POST /api/sounds/{cue}/test) calls PlayCue directly and must keep
// working — a manual test is explicit user intent and stays available even
// with cues disabled or (per contract §Sound "disabled vérifié avant tout
// accès au moteur") reported separately from this switch.
//
// CuesDisabled filtering (#230, contract §6.3) is applied HERE, before
// PlayCue — never inside internal/audio (which imports nothing from the
// server, contract §2.1) and never by skipping the call at each of the 22
// game-event sites individually. Read from config.Get() on every call,
// never cached on the App: a toggle flipped via POST /config.json must
// take effect on the very next cue, not after a restart. This is an
// ADDITIVE extension of #227's already-reviewed body — the call to
// a.sound().PlayCue(c) itself is untouched; at the zero value
// (CuesDisabled absent/empty/nil), the condition is always false and the
// behaviour is identical to the bit to what #227 shipped.
func (a *App) notifySound(c audio.Cue) {
	if !config.Get().Sound.Enabled {
		return
	}
	if config.Get().Sound.CuesDisabled[string(c)] {
		return
	}
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

// ---------------------------------------------------------------------------
// server.SoundProvider (internal/server/http_sound.go) — #230
// ---------------------------------------------------------------------------

// SoundEnabled implements server.SoundProvider: read directly from
// configuration, never from the engine — this is what lets
// POST /api/sounds/{cue}/test decide "disabled" WITHOUT touching the
// engine at all (contracts/http-endpoints.md §Sound: "disabled est
// vérifié avant tout accès au moteur").
func (a *App) SoundEnabled() bool {
	return config.Get().Sound.Enabled
}

// SoundOutputAvailable implements server.SoundProvider: true iff a REAL
// (non-neutral) Output is attached to the engine (contract sound.md §4
// amendment, audio.IsNeutral). Well-defined even when SoundEnabled() is
// false (Engine.OutputAvailable's own doc comment) — GET /api/sound/status
// still calls SoundEnabled() first for clarity, matching the contract's
// prose, but the order is not load-bearing here.
func (a *App) SoundOutputAvailable() bool {
	return a.sound().OutputAvailable()
}

// TestSoundCue implements server.SoundProvider: calls the engine
// DIRECTLY (audio.Engine.PlayCue), never through notifySound — a cue
// individually disabled via CuesDisabled must stay testable (contract
// §6.3: "tester est un geste explicite, seul le déroulé de la partie est
// muet"). c is assumed already validated against the closed catalogue by
// the HTTP layer (contracts/http-endpoints.md §Sound "Sécurité").
func (a *App) TestSoundCue(c audio.Cue) (accepted bool) {
	return a.sound().PlayCue(c)
}
