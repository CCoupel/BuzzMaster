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
	"sort"
	"strings"

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

// ambianceScene is one row of the scene table: colour + intensity, with
// useTeamColor meaning "the colour is the concerned team's palette colour".
type ambianceSceneDef struct {
	Color        [3]int
	Intensity    int
	UseTeamColor bool
}

var (
	ambianceWarmWhite = [3]int{255, 214, 170}
	// ambianceScoreGold is C2b's transient colour (contract §8.1): a
	// credited team's own light alternates with it during a SCORE pulse.
	// Not a second team palette — see ambianceScene's own doc comment for
	// why this is the one documented exception to C1a "never another
	// colour than the team's own".
	ambianceScoreGold = [3]int{255, 190, 0}

	ambianceSceneIdle       = ambianceSceneDef{Color: ambianceWarmWhite, Intensity: 120}     // room stays usable
	ambianceSceneReady      = ambianceSceneDef{Color: [3]int{255, 255, 255}, Intensity: 200} // attention rising
	ambianceSceneRunning    = ambianceSceneDef{Color: [3]int{40, 90, 255}, Intensity: 160}   // neutral blue, no team colour
	ambianceSceneBuzz       = ambianceSceneDef{UseTeamColor: true, Intensity: 255}           // exactly the buzzers' RGB
	ambianceScenePauseAll   = ambianceSceneDef{Color: [3]int{255, 170, 0}, Intensity: 120}   // amber: nothing is being played
	ambianceSceneRevealGood = ambianceSceneDef{Color: [3]int{0, 220, 60}, Intensity: 255}    // at least one correct answer
	ambianceSceneRevealNone = ambianceSceneDef{Color: [3]int{230, 30, 30}, Intensity: 255}   // nobody found it
	ambianceSceneTeamTurn   = ambianceSceneDef{UseTeamColor: true, Intensity: 200}
	ambianceSceneScore      = ambianceSceneDef{UseTeamColor: true, Intensity: 255}       // room-side COMET, same duration
	ambianceSceneEntracte   = ambianceSceneDef{Color: ambianceWarmWhite, Intensity: 100} // deliberate divergence: buzzers go dark, the room stays lit
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
		if len(ev.Teams) > 0 {
			return ambianceSceneRevealGood
		}
		return ambianceSceneRevealNone
	case lighting.KindTeamTurn:
		return ambianceSceneTeamTurn
	case lighting.KindScore:
		return ambianceSceneScore
	case lighting.KindEntracte:
		return ambianceSceneEntracte
	}
	return ambianceSceneIdle
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
	if def.UseTeamColor {
		team := ""
		if len(ev.Teams) > 0 {
			team = ev.Teams[0]
		}
		color = a.teamNameToRGB(team)
	}
	// #208 (contract §10.1): the manual ON/AUTO/OFF selector and Flash act
	// ONLY on this "general" zone, applied last so they always win over the
	// auto-derived scene — team zones below are never touched (Batch C/C1a:
	// they never fall back to "general" in the first place, so this was
	// already implied, now it is unconditionally true).
	color, intensity = a.lightingOverrideGeneral(color, intensity)
	zones := []lighting.ZoneState{{
		Zone:      lighting.ZoneGeneral,
		Color:     color,
		Intensity: intensity,
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
