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

// ambianceScene renders an Event into the single "general" zone (#205).
// Team colours go through the SAME palette as the buzzers (teamNameToRGB):
// never a second palette (contract §8).
func (a *App) ambianceScene(ev lighting.Event) lighting.State {
	def := ambianceSceneFor(ev)
	color := def.Color
	if def.UseTeamColor {
		team := ""
		if len(ev.Teams) > 0 {
			team = ev.Teams[0]
		}
		color = a.teamNameToRGB(team)
	}
	return lighting.State{Zones: []lighting.ZoneState{{
		Zone:      lighting.ZoneGeneral,
		Color:     color,
		Intensity: def.Intensity,
	}}}
}
