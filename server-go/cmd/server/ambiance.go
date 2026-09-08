package main

// Ambiance lighting adapter (#205, contracts/lighting.md). This file is the
// ONLY bridge between the game (cmd/server) and internal/lighting:
//
//   - the site registry (§6/§6.1): every (enclosing function, sendLEDSet*
//     call) pair in main.go with its ambiance decision — checked by the AST
//     exhaustiveness test (§7);
//   - the derivation of the live GameState into an Event (§6.2);
//   - the scene table v1 (§8) — Event to State, reusing the team palette;
//   - the lifecycle (§4.5/§9): construction in setup(), goroutine in start().
//
// Founding rule (contract §1): the ambiance is wired on the EVENT layer
// (handle*/broadcast*/setupCallbacks), never on the rendering layer
// (sendLEDSet* functions, which run once PER BUZZER).

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
	"buzzcontrol/internal/lighting/hue"
	"buzzcontrol/internal/server"
)

// ---------------------------------------------------------------------------
// Site registry (contract §6, §6.1, §7)
// ---------------------------------------------------------------------------

// ambianceSite identifies one LED emission site by the pair (enclosing
// function, LED function called) — stable across edits, unlike line numbers.
type ambianceSite struct {
	Func string // enclosing *App method (closures count for their enclosing method)
	LED  string // sendLEDSet* selector called there
}

// ambianceDecision is what the site does for the room.
type ambianceDecision struct {
	Kind   string // ambianceNotifyState | ambianceNotifyPulse | ambianceNoAmbiance
	Reason string // game event, or why there is deliberately no ambiance
}

const (
	ambianceNotifyState = "NotifyState"
	ambianceNotifyPulse = "NotifyPulse"
	ambianceNoAmbiance  = "NoAmbiance"
)

// ambianceIsRenderingLayer tells whether an enclosing function belongs to
// the LED rendering layer (contract §1): every sendLEDSet* method computes
// per-buzzer colours and knows no game event. The exhaustiveness test
// skips these enclosing functions; registry entries for them (below) are
// documentation of the §6.1 decisions, not sites to wire.
func ambianceIsRenderingLayer(enclosingFunc string) bool {
	return strings.HasPrefix(enclosingFunc, "sendLEDSet")
}

// ambianceSiteRegistry is the CLOSED list of LED sites and their ambiance
// decision. Adding a sendLEDSet* call in a new function of main.go without
// an entry here fails cmd/server/ambiance_sites_test.go — on purpose: one
// forgotten site means "the buzzers light up, the room does not".
var ambianceSiteRegistry = map[ambianceSite]ambianceDecision{
	// --- Event layer: state scenes (NotifyState, re-derived from live state) ---
	{"broadcastReady", "sendLEDSetAllBuzzers"}:        {ambianceNotifyState, "PREPARE→READY"},
	{"broadcastStart", "sendLEDSetAllBuzzers"}:        {ambianceNotifyState, "START (→COUNTDOWN/STARTED)"},
	{"broadcastStop", "sendLEDSetStop"}:               {ambianceNotifyState, "STOP"},
	{"broadcastPause", "sendLEDSetPause"}:             {ambianceNotifyState, "buzz (STARTED→PAUSED) — team derived from state"},
	{"broadcastPauseAll", "sendLEDSetPauseAll"}:       {ambianceNotifyState, "admin pause"},
	{"broadcastContinue", "sendLEDSetContinue"}:       {ambianceNotifyState, "CONTINUE"},
	{"broadcastReveal", "sendLEDSetReveal"}:           {ambianceNotifyState, "REVEAL — correct teams derived from state"},
	{"handleMotionDone", "sendLEDSetAllBuzzers"}:      {ambianceNotifyState, "MEMOTION complete (auto-stop)"},
	{"handleMotionSetTeams", "sendLEDSetAllBuzzers"}:  {ambianceNotifyState, "MEMOTION teams set — active team"},
	{"handleFlipMemoryCard", "sendLEDSetAllBuzzers"}:  {ambianceNotifyState, "MEMORY flip-back / match / complete (3 calls, one site)"},
	{"handleMemorySetTeams", "sendLEDSetAllBuzzers"}:  {ambianceNotifyState, "MEMORY teams set — active team"},
	{"setupCallbacks", "sendLEDSetRafaleTeams"}:       {ambianceNotifyState, "RAFALE next team (OnRafaleTeamsChanged)"},
	{"handleEntracteSet", "sendLEDSetAllEntracteOff"}: {ambianceNotifyState, "ENTRACTE on"},
	{"handleEntracteSet", "sendLEDSetAllBuzzers"}:     {ambianceNotifyState, "ENTRACTE off"},
	{"handleFullUpdate", "sendLEDSetAllBuzzers"}:      {ambianceNotifyState, "teams/bumpers edited — team colours may have changed"},
	{"onPhaseStarted", "sendLEDSetAllEntracteOff"}:    {ambianceNotifyState, "ENTRACTE programmée (#214) — actualStart()/StartImmediate()→STARTED, symétrie avec handleEntracteSet (T2.1, contract §10.5)"},

	// --- Event layer: pulses (NotifyPulse KindScore, 4800 ms) ---
	{"handlePoints", "sendLEDSetComet"}:       {ambianceNotifyPulse, "points awarded — credited team"},
	{"handleBumperPoints", "sendLEDSetComet"}: {ambianceNotifyPulse, "bumper points — credited team"},
	{"handleTeamPoints", "sendLEDSetComet"}:   {ambianceNotifyPulse, "team points — credited team"},
	{"handleMotionDone", "sendLEDSetComet"}:   {ambianceNotifyPulse, "MEMOTION winner — winning team"},

	// --- Deliberately no ambiance (contract §6.1) ---
	{"resendLEDOnReconnect", "sendLEDSetForBuzzer"}: {ambianceNoAmbiance, "resync of ONE reconnecting device; nothing changed in the game"},
	{"sendLEDSetComet", "sendLEDSetAllBuzzers"}:     {ambianceNoAmbiance, "AfterFunc +4.8 s restore — the SCORE pulse deadline already ends the scene at the same instant"},
	{"broadcastLEDSet", "sendLEDSet"}:               {ambianceNoAmbiance, "DEAD CODE (audit #132, zero call sites) — decision to revisit if ever reactivated"},
	{"sendLEDSetToTeam", "sendLEDSet"}:              {ambianceNoAmbiance, "DEAD CODE (audit #132, zero call sites) — decision to revisit if ever reactivated"},
	{"stop", "sendLEDSetAllEntracteOff"}:            {ambianceNoAmbiance, "server shutdown (#208, contract §10.4) — the process is exiting, nothing left to notify; the Hue side is switched off directly via LightingDriver().Apply() (shutdownExtinguishHueLighting), not through NotifyState()"},
}

// ---------------------------------------------------------------------------
// Lifecycle (contract §4.5, §9)
// ---------------------------------------------------------------------------

// ambianceIsConfigured reports whether ambiance lighting is usable
// (contract hue-bridge.md §5.5/§6): the `lighting` section is enabled, a key
// is available (stored or BUZZCONTROL_HUE_API_KEY) and the bridge is known
// by IP or by id. #205 returned false unconditionally; #207 reads config.json.
func (a *App) ambianceIsConfigured() bool {
	lc := config.Get().Lighting
	return lc.Enabled && lc.EffectiveAPIKeyConfigured() && (strings.TrimSpace(lc.BridgeIP) != "" || strings.TrimSpace(lc.BridgeID) != "")
}

// buildHueDriver builds the Hue driver from the current configuration, or
// returns nil when lighting is not usable. No network I/O (hue.New). An
// invalid configuration is logged once here and treated as "not configured".
func (a *App) buildHueDriver() *hue.Driver {
	if !a.ambianceIsConfigured() {
		return nil
	}
	lc := config.Get().Lighting
	specs := make([]hue.LightSpec, 0, len(lc.Lights))
	for _, l := range lc.Lights {
		specs = append(specs, hue.LightSpec{Name: l.Name, Role: hue.LightRole(l.Role), Team: l.Team})
	}
	d, err := hue.New(hue.Config{
		BridgeIP: lc.BridgeIP,
		BridgeID: lc.BridgeID,
		APIKey:   lc.EffectiveAPIKey(), // never logged, never echoed
		Lights:   specs,
		Logger: func(format string, args ...any) {
			server.LogInfo(game.LogComponentApp, "Ambiance: "+format, args...)
		},
		// #208/#213 reprise (contract §10.3): "au retour du pont", the room
		// is re-derived and re-applied from the LIVE game state — the
		// existing writer already does exactly that on every NotifyState()
		// (§4.1, never a snapshot to replay), so the reconnection resync
		// reduces to firing this one call. Reads a.ambiance() live (not a
		// captured writer): correct across a reconfigureAmbiance() hot-swap
		// too, since this closure is rebuilt with the new driver each time.
		OnReconnect: func() {
			a.ambiance().NotifyState()
		},
	})
	if err != nil {
		server.LogWarn(game.LogComponentApp, "Ambiance: lighting configuration rejected: %v", err)
		return nil
	}
	return d
}

// newAmbianceWriter builds the writer bound to this App's live state and
// scene table. Shared by setupAmbiance (real driver) and by tests
// (lighting.FakeDriver): a.lightingWriter.Store(a.newAmbianceWriter(fake)); go a.ambiance().Start(ctx).
// A Hue driver gets the writer pacing the contract requires for it
// (hue.RecommendedMinInterval, §5.4); other drivers keep the #205 default.
func (a *App) newAmbianceWriter(drv lighting.Driver) *lighting.Writer {
	cfg := lighting.Config{
		Driver: drv,
		Derive: a.deriveAmbianceEvent,
		Scene:  a.ambianceScene,
		OnError: func(err error) {
			server.LogWarn(game.LogComponentApp, "Ambiance: driver error: %v", err)
		},
	}
	if _, isHue := drv.(*hue.Driver); isHue {
		cfg.MinInterval = hue.RecommendedMinInterval
	}
	return lighting.NewWriter(cfg)
}

// ambiance returns the ambiance-lighting writer, nil when lighting is not
// configured (every Notify* on a nil writer is a no-op, contract §4.3). The
// only accessor of a.lightingWriter: an atomic read, safe from any goroutine.
func (a *App) ambiance() *lighting.Writer {
	return a.lightingWriter.Load()
}

// setupAmbiance is called from (*App).setup(). Not configured ⇒ a.ambiance()
// stays nil, every Notify* on it is a no-op and start() launches nothing
// (contract lighting.md §4.3/§4.5).
func (a *App) setupAmbiance() {
	a.ambianceMu.Lock()
	defer a.ambianceMu.Unlock()
	d := a.buildHueDriver()
	if d == nil {
		return
	}
	a.hueDriver.Store(d)
	a.lightingWriter.Store(a.newAmbianceWriter(d))
}

// startAmbianceWriter starts the writer goroutine built by setupAmbiance
// (called from (*App).start(), same lifecycle as AckManager — stopped by
// a.cancelCtx() in stop()). Not configured ⇒ a.ambiance() is nil ⇒ this
// launches nothing (contract lighting.md §4.3/§4.5 — not even a goroutine
// that returns at once).
//
// Bugfix (QUALIF v10.0.0.10, round 2, bug 2 — "pont toujours injoignable
// après relance", persisting even with a FRESH association and a truly
// reachable bridge): this architecture is deliberately poll-free — the
// writer only ever contacts the bridge in reaction to a NotifyState()/
// NotifyPulse() from a GAME event (contract §4). reconfigureAmbiance()
// below already calls w.NotifyState() once right after building a writer
// (the config-update/register path), for exactly this reason — but nothing
// equivalent ever ran on the STARTUP path: setupAmbiance() only builds and
// stores the driver+writer, it never kicks them. A server that starts with
// a valid, reachable, PERSISTED association therefore never attempted a
// single real contact until some unrelated game event happened to fire
// one: GET /api/lighting/status does no I/O by design (contract §7), so
// the admin screen kept showing the driver's untouched zero-value status —
// StateUnreachable, reason "not contacted yet" — indistinguishable from a
// genuinely dead bridge. The fix is the same one-line kick reconfigureAmbiance
// already had: NotifyState() is nil-safe and a no-op when lighting is not
// configured (contract §4.3), so it needs no `if` guard, matching every
// other Notify* call site in the registry.
func (a *App) startAmbianceWriter() {
	w := a.ambiance()
	if w != nil {
		go w.Start(a.ctx)
		go a.runChronoPulse(a.ctx) // idempotent (chronoPulseStarted) — see its own doc comment
	}
	w.NotifyState()
}

// reconfigureAmbiance is called from OnConfigUpdate (POST /config.json,
// /api/lighting/register): it rebuilds the driver from the new configuration
// and hot-swaps it into the writer (lighting.Writer.SetDriver), starting the
// writer goroutine on the first runtime enable.
//
// Serialised by ambianceMu: concurrent config updates are applied one after
// the other, so exactly one writer is ever started and hueDriver always names
// the driver the writer holds. The 21 event sites keep reading the writer
// through the atomic a.ambiance() without taking the mutex.
func (a *App) reconfigureAmbiance() {
	a.ambianceMu.Lock()
	defer a.ambianceMu.Unlock()
	d := a.buildHueDriver()
	w := a.ambiance()
	if w == nil {
		if d == nil {
			a.hueDriver.Store(nil)
			return
		}
		w = a.newAmbianceWriter(d)
		a.hueDriver.Store(d)
		a.lightingWriter.Store(w) // first runtime enable: publish, then start
		if a.ctx != nil {
			go w.Start(a.ctx)
			go a.runChronoPulse(a.ctx) // idempotent (chronoPulseStarted) — see its own doc comment
		}
		w.NotifyState()
		return
	}
	a.hueDriver.Store(d)
	// A nil *hue.Driver must become a nil INTERFACE (a typed nil would count
	// as an attached driver and panic on Close).
	var drv lighting.Driver
	if d != nil {
		drv = d
	}
	w.SetDriver(drv) // nil disables; closes the previous driver
	if d != nil && !w.Running() && a.ctx != nil {
		go w.Start(a.ctx)
	}
	w.NotifyState()
}

// LightingDriver implements server.LightingProvider for the /api/lighting/*
// handlers: the live driver, nil when disabled. Read without I/O.
func (a *App) LightingDriver() *hue.Driver {
	return a.hueDriver.Load()
}

// ---------------------------------------------------------------------------
// Derivation from the live state (contract §6.2)
// ---------------------------------------------------------------------------

// deriveAmbianceEvent maps the LIVE GameState to an Event. Runs on the
// writer goroutine at apply time, so it reads ONLY through the engine's
// locked getters — never App maps such as bumperBuzzState, which belong to
// the dispatch goroutine (see App struct comments).
func (a *App) deriveAmbianceEvent() lighting.Event {
	state := a.engine.GetState()

	// Entracte is a transverse mode, tested BEFORE the phase.
	if state.Entracte {
		return lighting.Event{Kind: lighting.KindEntracte}
	}

	switch state.Phase {
	case game.PhasePrepare, game.PhaseReady, game.PhaseCountdown:
		// COUNTDOWN has no rendering of its own on buzzers either
		// (sendLEDSetForBuzzerNormal groups it with STOPPED/PREPARE/READY);
		// a countdown scene belongs to v10.1 (#212).
		return lighting.Event{Kind: lighting.KindReady}

	case game.PhaseStarted:
		if team := ambianceActiveTeam(state); team != "" {
			return lighting.Event{Kind: lighting.KindTeamTurn, Teams: []string{team}}
		}
		return lighting.Event{Kind: lighting.KindRunning}

	case game.PhasePaused:
		if team := a.ambianceBuzzTeam(); team != "" {
			return lighting.Event{Kind: lighting.KindBuzz, Teams: []string{team}}
		}
		return lighting.Event{Kind: lighting.KindPauseAll}

	case game.PhaseRevealed:
		return lighting.Event{Kind: lighting.KindReveal, Teams: a.ambianceCorrectTeams(state)}
	}
	// STOPPED, NEW_GAME, ENROLL and anything else: no game in progress.
	return lighting.Event{Kind: lighting.KindIdle}
}

// ambianceActiveTeam returns the team whose turn it is in the multi-team
// question types, "" otherwise.
func ambianceActiveTeam(state game.GameState) string {
	if state.Question == nil {
		return ""
	}
	switch state.Question.Type {
	case game.QuestionTypeMemory:
		return state.MemoryCurrentTeam
	case game.QuestionTypeMemotion:
		return state.MotionCurrentTeam
	case game.QuestionTypeRafale:
		return state.RafaleCurrentTeam
	}
	return ""
}

// ambianceBuzzTeam identifies the team whose buzz caused the PAUSE: the
// bumper with the most recent press time in the round (Bumper.Time, reset
// on READY). "" when nobody buzzed (admin PAUSE_ALL).
func (a *App) ambianceBuzzTeam() string {
	tb := a.engine.GetTeamsAndBumpersSnapshot()
	var latest int64
	team := ""
	for _, b := range tb.Bumpers {
		if b == nil || b.Time <= 0 {
			continue
		}
		if b.Time > latest {
			latest = b.Time
			team = b.Team
		}
	}
	return team
}

// ambianceCorrectTeams lists, for a QCM, the teams whose buzzed bumpers gave
// the correct answer — ordered by press time (first = principal), no
// duplicates. Empty for other question types or when nobody was right.
func (a *App) ambianceCorrectTeams(state game.GameState) []string {
	if state.Question == nil || state.Question.Type != game.QuestionTypeQCM || state.Question.QCMCorrect == "" {
		return nil
	}
	tb := a.engine.GetTeamsAndBumpersSnapshot()
	type hit struct {
		team string
		at   int64
	}
	var hits []hit
	for _, b := range tb.Bumpers {
		if b == nil || b.Time <= 0 || b.Team == "" {
			continue
		}
		if string(b.AnswerColor) == state.Question.QCMCorrect {
			hits = append(hits, hit{b.Team, b.Time})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].at < hits[j].at })
	var teams []string
	seen := map[string]bool{}
	for _, h := range hits {
		if !seen[h.team] {
			seen[h.team] = true
			teams = append(teams, h.team)
		}
	}
	return teams
}

// ---------------------------------------------------------------------------
// Scene table v1 (contract §8) — hard-wired for #205, editable in v10.1 (#210)
// ---------------------------------------------------------------------------

// ambianceSceneDef is one row of the scene table: colour + intensity.
// UseTeamColor means "the colour is the concerned team's palette colour"
// (SCORE only, as of the 2026-09-08 theme revision below); UseThemeColor
// means "the colour is the current question's category theme, white if
// none" (ambianceThemeColor, below) — the two are mutually exclusive across
// the table, never combined on the same row.
type ambianceSceneDef struct {
	Color         [3]int
	Intensity     int
	UseTeamColor  bool
	UseThemeColor bool
}

var (
	ambianceWarmWhite = [3]int{255, 214, 170}
	// ambianceScoreGold is C2b's transient colour (contract §8.1): a
	// credited team's own light alternates with it during a SCORE pulse.
	// Not a second team palette — see ambianceScene's own doc comment for
	// why this is the one documented exception to C1a "never another
	// colour than the team's own".
	ambianceScoreGold = [3]int{255, 190, 0}

	// Revision 2026-09-08 (planner-v10-general-theme-toggle-20260908-
	// 114420.md §1.3, validated AND EXTENDED by the user — REVEAL and
	// PAUSE_ALL included too, against the planner's own recommendation to
	// leave them as fixed information colours): "general" now shows the
	// CURRENT QUESTION's category theme (ambianceThemeColor, white if none)
	// everywhere a question is relevant, replacing the neutral blue of
	// RUNNING, the amber of PAUSE_ALL, the green/red of REVEAL, and the
	// team colour of TEAM_TURN/BUZZ (redundant since C1a: a team's own
	// identity is now carried permanently by its own bulb). Only IDLE (no
	// question at all — plain white, brighter than before: 200 not the old
	// warm-white 120) and ENTRACTE (deliberate divergence, unchanged) stay
	// outside this rule. SCORE is untouched — it is a pulse, not a phase of
	// this table, and stays the credited team's own colour (§2.5).
	ambianceSceneIdle     = ambianceSceneDef{Color: [3]int{255, 255, 255}, Intensity: 200} // no question at all: plain white
	ambianceSceneReady    = ambianceSceneDef{UseThemeColor: true, Intensity: 200}          // question designated: announce its theme before it starts
	ambianceSceneRunning  = ambianceSceneDef{UseThemeColor: true, Intensity: 200}          // ⭐ the case that motivated this revision
	ambianceSceneBuzz     = ambianceSceneDef{UseThemeColor: true, Intensity: 255}
	ambianceScenePauseAll = ambianceSceneDef{UseThemeColor: true, Intensity: 120}
	ambianceSceneReveal   = ambianceSceneDef{UseThemeColor: true, Intensity: 255} // good/bad no longer distinguished by 'general' — user's explicit extension
	ambianceSceneTeamTurn = ambianceSceneDef{UseThemeColor: true, Intensity: 200}
	ambianceSceneScore    = ambianceSceneDef{UseTeamColor: true, Intensity: 255}       // unchanged — room-side COMET, same duration, credited team's colour
	ambianceSceneEntracte = ambianceSceneDef{Color: ambianceWarmWhite, Intensity: 100} // unchanged — buzzers go dark, the room stays lit
)

// ambianceSceneFor picks the table row for an event.
func ambianceSceneFor(ev lighting.Event) ambianceSceneDef {
	switch ev.Kind {
	case lighting.KindReady:
		return ambianceSceneReady
	case lighting.KindRunning:
		return ambianceSceneRunning
	case lighting.KindBuzz:
		return ambianceSceneBuzz
	case lighting.KindPauseAll:
		return ambianceScenePauseAll
	case lighting.KindReveal:
		return ambianceSceneReveal
	case lighting.KindTeamTurn:
		return ambianceSceneTeamTurn
	case lighting.KindScore:
		return ambianceSceneScore
	case lighting.KindEntracte:
		return ambianceSceneEntracte
	}
	return ambianceSceneIdle
}

// ---------------------------------------------------------------------------
// Chrono-pulse — 'general' breathes in sync with the active question's
// GLOBAL timer (2026-09-08, planner-v10-chrono-pulse-v2-20260908-120128.md
// §3, design confirmed by the user). One phase-flag/re-derivation loop
// (runChronoPulse), the exact patron of runScoreFlash/runLightingFlash —
// internal/lighting/hue/ is untouched, the driver still only ever sees
// ordinary State values.
// ---------------------------------------------------------------------------

const (
	// chronoPulseTickInterval is how often runChronoPulse re-examines the
	// LIVE game state — never a write cadence. Fine enough to land within
	// a perceptually negligible margin of tier 3's 250 ms phase boundary.
	chronoPulseTickInterval = 100 * time.Millisecond
	// chronoPulseCycle is the pulse's cycle length for EVERY tier — the
	// user's own explicit choice (report §3/§4 option B) over the literal
	// request's 0.5 s at tier 3: the planner's budget analysis showed
	// 0.5 s exceeds both the /groups recommendation (§5.8) and the
	// writer's own 250 ms MinInterval floor with no headroom for HTTP
	// latency, which would make the rhythm irregular rather than merely
	// slower. Urgency at tier 3 is carried by colour dominance and
	// amplitude instead (ambianceChronoPulseColor, below), at the SAME
	// write cost as tiers 1-2.
	chronoPulseCycle = 1 * time.Second
	// chronoPulseHighMsTier3 is tier 3's asymmetric split within its 1 s
	// cycle (250 ms theme / 750 ms red — "rouge dominant"); tiers 1-2 split
	// their cycle evenly (chronoPulseCycle/2).
	chronoPulseHighMsTier3 = 250 * time.Millisecond
	// chronoPulseTransitionMs is "demi-cycle" (half of chronoPulseCycle),
	// sent as transitiontime on EVERY chrono-pulse write, uniformly across
	// all 3 tiers regardless of that tier's own phase split (contract
	// hue-bridge.md §5.2 amendment) — the bridge interpolates the fade
	// itself, so the room breathes instead of stepping between two flat
	// levels.
	chronoPulseTransitionMs = int(chronoPulseCycle / time.Millisecond / 2)

	chronoPulseIntensityFull = 255 // 100%
	chronoPulseIntensityHalf = 128 // 50% of 255, rounded (127.5 -> 128) — tiers 1-2's low phase
	chronoPulseIntensityLow  = 77  // 30% of 255, rounded (76.5 -> 77) — tier 3's low phase, amplitude accrue
)

var (
	// ambianceChronoPulseOrange/-Red are literal colours reused from the
	// PRE-theme PAUSE_ALL/REVEAL scenes (both theme-coloured themselves as
	// of §8's 2026-09-08 revision, above) — chosen here for their cultural
	// meaning (amber = caution, red = urgency), independent of whatever
	// PAUSE_ALL/REVEAL currently render. Never a 3rd/4th colour (the
	// handoff's own explicit constraint): reuse, not invent.
	ambianceChronoPulseOrange = [3]int{255, 170, 0}
	ambianceChronoPulseRed    = [3]int{230, 30, 30}
)

// ambianceChronoPulseColor resolves 'general's colour/intensity for the
// CURRENT chrono-pulse phase (a.chronoPulseTier/-High, written only by
// runChronoPulse), given the already-resolved theme colour. Tier 1's high
// AND low phases are both the theme colour (couleurBasse == couleurHaute,
// "respiration pure") — not a special case, the same shape as tiers 2-3
// with the tier-specific low colour substituted for the theme.
func ambianceChronoPulseColor(theme [3]int, tier int, high bool) ([3]int, int) {
	if high {
		return theme, chronoPulseIntensityFull
	}
	switch tier {
	case 3:
		return ambianceChronoPulseRed, chronoPulseIntensityLow
	case 2:
		return ambianceChronoPulseOrange, chronoPulseIntensityHalf
	default: // tier 1
		return theme, chronoPulseIntensityHalf
	}
}

// runChronoPulse is the always-on evaluation loop backing the chrono-pulse
// (see the section comment above). Idempotent (chronoPulseStarted): called
// from both of startAmbianceWriter's/reconfigureAmbiance's "go w.Start"
// sites, mirroring the writer's own lifecycle exactly — mutually exclusive
// in practice (the writer object is only ever first-started at one of the
// two), but this guard makes that provable rather than assumed, at
// negligible cost (one CompareAndSwap), the same spirit as
// lighting.Writer.Start's own running.CompareAndSwap.
//
// STATELESS by design between ticks besides the local "did anything
// change" bookkeeping below: every tick re-derives eligibility from the
// LIVE GameState via deriveAmbianceEvent() (the exact same function that
// decides every other scene), never a separately-tracked "am I in a
// question" flag of its own. This is what makes the four "s'efface
// devant"/"périmètre" requirements fall out for free, with no dedicated
// code for any of them:
//   - only KindRunning is eligible — BUZZ/PAUSE_ALL/REVEAL/ENTRACTE/TEAM_TURN
//     are simply different Kinds, so the pulse is never even considered;
//   - RAFALE and MEMOTION are excluded too, NOT via a special case: a
//     RAFALE round or an active MEMOTION card always has a current team
//     (ambianceActiveTeam), so deriveAmbianceEvent already returns
//     KindTeamTurn for them, never KindRunning;
//   - "se gèle en pause" needs no freeze logic: PhasePaused derives
//     KindBuzz/KindPauseAll, not KindRunning, so the pulse simply stops
//     being rendered — GameState.CurrentTime itself already stops
//     ticking while paused (existing engine behaviour, unrelated to this
//     effect);
//   - the ON/AUTO/OFF selector/Flash are consulted directly in
//     ambianceScene (below), the same primitives lightingOverrideGeneral
//     is built from, so the two can never drift apart.
func (a *App) runChronoPulse(ctx context.Context) {
	if !a.chronoPulseStarted.CompareAndSwap(false, true) {
		return
	}
	ticker := time.NewTicker(chronoPulseTickInterval)
	defer ticker.Stop()
	lastActive, lastTier, lastHigh := false, 0, true
	cycleMs := int64(chronoPulseCycle / time.Millisecond)
	highMsTier12 := cycleMs / 2
	highMsTier3 := int64(chronoPulseHighMsTier3 / time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		ev := a.deriveAmbianceEvent()
		state := a.engine.GetState()
		active := ev.Kind == lighting.KindRunning && state.Delay > 0
		if !active {
			if lastActive {
				a.chronoPulseActive.Store(false)
				lastActive = false
				a.ambiance().NotifyState() // prompt clear — see doc comment above on why this is belt-and-suspenders, not load-bearing
			}
			continue
		}
		remaining := state.CurrentTime
		tier := 1
		if remaining <= 10 {
			tier = 2
		}
		if remaining <= 5 {
			tier = 3
		}
		highMs := highMsTier12
		if tier == 3 {
			highMs = highMsTier3
		}
		high := time.Now().UnixMilli()%cycleMs < highMs
		if !lastActive || tier != lastTier || high != lastHigh {
			a.chronoPulseTier.Store(int32(tier))
			a.chronoPulseHigh.Store(high)
			a.chronoPulseActive.Store(true)
			lastActive, lastTier, lastHigh = true, tier, high
			a.ambiance().NotifyState()
		}
	}
}

// ambianceTeamZoneDimsWhenUndistinguished reports whether ev's Kind is one
// of the three "active game" phases (STARTED/PAUSED/REVEALED, transposed
// literally from the buzzer's own sendLEDSetForBuzzerNormal switch) in which
// a team not distinguished by the event is attenuated. Every other Kind — no
// game in progress, PREPARE/READY/COUNTDOWN, and the transverse ENTRACTE
// mode, none of them named in that switch — renders every team light at
// full intensity, exactly like a buzzer outside active play (Batch C/C1a,
// planner-v10-teamcolor-changes-20260908-092100.md §1.3).
//
//	STOPPED, PREPARE, READY, COUNTDOWN     -> full, always        (KindIdle, KindReady)
//	STARTED  (no active turn / active turn) -> full if distinguished, else dim (KindRunning, KindTeamTurn)
//	PAUSED   (admin / a buzz)               -> full if distinguished, else dim (KindPauseAll, KindBuzz)
//	REVEALED                                -> full if distinguished, else dim (KindReveal)
//	SCORE (pulse, not a phase of its own)   -> same rule for every OTHER team; the CREDITED team is
//	                                            governed separately by the C2b flicker, always full (KindScore)
//	ENTRACTE (transverse, no phase table row) -> full, always     (KindEntracte)
func ambianceTeamZoneDimsWhenUndistinguished(kind lighting.EventKind) bool {
	switch kind {
	case lighting.KindRunning, lighting.KindTeamTurn, lighting.KindBuzz, lighting.KindPauseAll, lighting.KindReveal, lighting.KindScore:
		return true
	}
	return false
}

// currentScoreFlashTeam is the team currently flickering gold/team-colour
// (C2b, ambiance_override.go's startScoreFlash/runScoreFlash), "" when no
// SCORE flicker is in flight. Safe on the zero atomic.Value (nil -> "").
func (a *App) currentScoreFlashTeam() string {
	v, _ := a.scoreFlashTeam.Load().(string)
	return v
}

// ambianceScene renders an Event into the "general" zone (#205) plus one
// zone per team CURRENTLY ON THE BOARD (#213, extended by Batch A/P1 below,
// then made unconditional by Batch C/C1a below that). Team colours go
// through the SAME palette as the buzzers (teamNameToRGB): never a second
// palette (contract §8, hue-bridge.md §9) — with exactly one transient
// exception, C2b's gold SCORE flicker, documented at the bottom of this
// comment.
//
// The "general" zone's own colour/intensity is computed exactly as before
// #213 (unaffected by the per-team zones added below): for a
// UseTeamColor scene (BUZZ/TEAM_TURN/SCORE) it already carries the
// PRINCIPAL team's colour (ev.Teams[0]) — this is what makes the
// degradation rules of hue-bridge.md §5.7 fall out for free, with no
// special-casing here:
//   - a team with no dedicated light never repaints "general" (its own zone
//     is simply never picked up by any light — hue-bridge.md §5.2's
//     zoneFor, a non-event, not an error);
//   - no team-role light configured at all ⇒ every light is "general" ⇒
//     the room alone carries the active team's colour, exactly the
//     pre-#213 behaviour (lighting.md §6.3).
//
// Bugfix (Batch A/P1, planner diagnostic
// _work/reports/planner-v10-groups-teamcolor-20260907-173831.md §Problème
// 1): a team's OWN dedicated zone used to be emitted only for the teams
// NAMED BY THE EVENT (ev.Teams) — populated for just 4 of the 9 Kinds
// (BUZZ/TEAM_TURN/REVEAL/SCORE). For the other 5 (IDLE/READY/RUNNING/
// PAUSE_ALL/ENTRACTE), ev.Teams is empty, so a team's own light fell back
// to "general" (hue-bridge.md §5.2's zoneFor, correctly applied — the BUG
// was upstream, in what "the event names" was mistaken for) and showed the
// ROOM's colour instead of the team's own — e.g. blue all through RUNNING,
// most of a round. hue-bridge.md §5.2 says "l'équipe n'est pas nommée dans
// l'ÉTAT COURANT" — the LIVE GAME STATE (the board), not the event that
// happens to be firing right now.
//
// Fix, transposing the buzzer's own model (sendLEDSetForBuzzerNormal,
// main.go: rgb computed ONCE outside the phase switch, only intensity/
// effect vary) to the room: a team's ampoule ALWAYS shows its own colour,
// for every team on the current board (a.engine's live team roster) — never
// the scene's fixed colour, never modified by state. Only the INTENSITY
// varies (ambianceTeamZoneDimsWhenUndistinguished, above).
//
// Batch C/C1a (planner-v10-teamcolor-changes-20260908-092100.md §1): the
// "no game in progress ⇒ no team zones at all, falls back to general"
// carve-out above is GONE — a team's own light is now emitted for EVERY
// event Kind, KindIdle included, and correspondingly is now ALWAYS at full
// intensity there (ambianceTeamZoneDimsWhenUndistinguished(KindIdle) is
// false). A team's own light therefore never again falls back to "general"
// — with exactly two documented exceptions elsewhere in the codebase, never
// here: an orphaned team assignment (the light's team absent from
// tb.Teams below — §1.7, a config inconsistency, not a game state, so
// simply falls out of this loop with no entry to append) and the shutdown
// extinction of contract §10.4 (ambiance_override.go's
// shutdownExtinguishHueLighting, untouched — it already forces every zone,
// general AND team, to Intensity 0 directly, never through this function).
// This also means #208's ON/AUTO/OFF selector and Flash NEVER reach a
// team's own light, in game or out — lightingOverrideGeneral below is
// applied to the "general" zone only, same as always.
//
// Batch C/C2b (same report, §2): a SCORE pulse's CREDITED team (the single
// entry of ev.Teams for KindScore) alternates between its own colour and
// ambianceScoreGold — the flicker's phase and which team is flickering are
// read from currentScoreFlashTeam()/a.scoreFlashPhaseGold, both owned by
// ambiance_override.go's startScoreFlash/runScoreFlash (started alongside
// every NotifyPulse(KindScore, ...) call in main.go). This is NOT a
// violation of "a team's ampoule always shows its own colour": the gold is
// TRANSIENT, triggered by THIS team's OWN score, alternates WITH its own
// colour and returns to it — celebrating the team's identity, never
// replacing it with another team's or the room's. It is the only exception
// to that rule besides §10.4's shutdown extinction.
func (a *App) ambianceScene(ev lighting.Event) lighting.State {
	def := ambianceSceneFor(ev)
	color := def.Color
	intensity := def.Intensity
	switch {
	case def.UseTeamColor:
		team := ""
		if len(ev.Teams) > 0 {
			team = ev.Teams[0]
		}
		color = a.teamNameToRGB(team)
	case def.UseThemeColor:
		color = a.ambianceThemeColor()
	}
	// Chrono-pulse (2026-09-08): only for the LIVE KindRunning scene, and
	// only when the selector is in AUTO with Flash off — checked with the
	// EXACT SAME two primitives lightingOverrideGeneral (below) is built
	// from (isLightingFlashOn/lightingMode), so "s'efface devant le
	// sélecteur" can never drift out of sync between the two: whenever this
	// condition is true, lightingOverrideGeneral's own AUTO branch is
	// necessarily a pure passthrough, so transitionMs never survives into a
	// forced ON/OFF/Flash state — no separate reset needed after it runs.
	transitionMs := 0
	if ev.Kind == lighting.KindRunning && a.chronoPulseActive.Load() && !a.isLightingFlashOn() && a.lightingMode() == lightingModeAuto {
		color, intensity = ambianceChronoPulseColor(color, int(a.chronoPulseTier.Load()), a.chronoPulseHigh.Load())
		transitionMs = chronoPulseTransitionMs
	}
	// #208 (contract §10.1): the manual ON/AUTO/OFF selector and Flash act
	// ONLY on this "general" zone, applied last so they always win over the
	// auto-derived scene — team zones below are never touched (Batch C/C1a:
	// they never fall back to "general" in the first place, so this was
	// already implied, now it is unconditionally true).
	color, intensity = a.lightingOverrideGeneral(color, intensity)
	zones := []lighting.ZoneState{{
		Zone:         lighting.ZoneGeneral,
		Color:        color,
		Intensity:    intensity,
		TransitionMs: transitionMs,
	}}
	seen := map[string]bool{lighting.ZoneGeneral: true} // defensive: a team literally named "general" must never shadow it
	distinguished := make(map[string]bool, len(ev.Teams))
	for _, team := range ev.Teams {
		if team != "" {
			distinguished[team] = true
		}
	}
	dims := ambianceTeamZoneDimsWhenUndistinguished(ev.Kind)
	flashTeam := ""
	if ev.Kind == lighting.KindScore {
		flashTeam = a.currentScoreFlashTeam()
	}
	tb := a.engine.GetTeamsAndBumpersSnapshot()
	for teamName := range tb.Teams {
		if teamName == "" || seen[teamName] {
			continue
		}
		seen[teamName] = true
		rgb := a.teamNameToRGB(teamName)
		teamIntensity := 255 // full — the room-side equivalent of the buzzer's own SOLID/BLINK at 255
		if dims && !distinguished[teamName] {
			teamIntensity = dimIntensityFor(rgb)
		}
		if teamName == flashTeam && a.scoreFlashPhaseGold.Load() {
			rgb = ambianceScoreGold // C2b — transient, see this function's own doc comment
		}
		zones = append(zones, lighting.ZoneState{
			Zone:      teamName,
			Color:     rgb,
			Intensity: teamIntensity,
		})
	}
	return lighting.State{Zones: zones}
}

// ambianceThemeColor resolves the CURRENT question's category theme colour
// (2026-09-08 revision, contract §8.1) — read live at render time, like
// every other derivation in this file, never memoised. Resolution order:
//  1. a drawn RAFALE question's OWN category (state.RafaleCurrentQuestion —
//     "the theme follows each question drawn", livelier than the round's
//     multi-category list, #216);
//  2. else the host question's own category (state.Question.Category,
//     including a MEMOTION host question);
//  3. empty/unknown/custom-category colour, or no question at all ⇒ white.
//
// Reuses ResolveCategoryMeta (internal/server, already called 4× elsewhere
// in main.go) rather than a second category→colour table — these are UI
// accent colours repurposed as light colours (contract's own documented
// caveat: calibrated for a screen, not a room; hex values taken as-is for
// this revision, not passed through nearestPaletteColorByHue).
func (a *App) ambianceThemeColor() [3]int {
	white := [3]int{255, 255, 255}
	if a.httpServer == nil { // defensive: nil in some minimal test harnesses, never in production (setup() before any scene is ever rendered)
		return white
	}
	state := a.engine.GetState()
	category := ""
	switch {
	case state.Question != nil && state.Question.Type == game.QuestionTypeRafale && state.RafaleCurrentQuestion.ID != "":
		category = state.RafaleCurrentQuestion.Category
	case state.Question != nil:
		category = string(state.Question.Category)
	}
	if category == "" {
		return white
	}
	_, _, hex := a.httpServer.ResolveCategoryMeta(category)
	rgb, ok := hexToRGB(hex)
	if !ok {
		return white
	}
	return rgb
}

// hexToRGB parses a "#rrggbb" colour (ResolveCategoryMeta's own format)
// into the project's [3]int 0-255 RGB. false for anything else (empty —
// custom category or none, malformed, wrong length).
func hexToRGB(s string) ([3]int, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return [3]int{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return [3]int{}, false
	}
	return [3]int{int(v >> 16 & 0xff), int(v >> 8 & 0xff), int(v & 0xff)}, true
}
