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

import "strings"

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
	{"handleNewGame", "sendLEDSetAllBuzzers"}:         {ambianceNotifyState, "NEW_GAME (#243) — every applied reset refreshes LED and ambiance"},
	{"onPhaseStarted", "sendLEDSetAllEntracteOff"}:    {ambianceNotifyState, "ENTRACTE programmée (#214) — actualStart()/StartImmediate()→STARTED, symétrie avec handleEntracteSet (T2.1, contract §10.5)"},

	// --- Event layer: pulses (NotifyPulse KindScore, 4800 ms) ---
	{"handlePoints", "sendLEDSetComet"}:            {ambianceNotifyPulse, "points awarded — credited team"},
	{"handleBumperPoints", "sendLEDSetComet"}:      {ambianceNotifyPulse, "bumper points — credited team"},
	{"handleTeamPoints", "sendLEDSetComet"}:        {ambianceNotifyPulse, "team points — credited team"},
	{"handleMotionDone", "sendLEDSetComet"}:        {ambianceNotifyPulse, "MEMOTION winner — winning team"},
	{"handleBumperPointsBatch", "sendLEDSetComet"}: {ambianceNotifyPulse, "grouped player award (#254) — one pulse for the teams of the credited players"},
	{"handleTeamPointsBatch", "sendLEDSetComet"}:   {ambianceNotifyPulse, "grouped team award (#244/#251) — one pulse for the credited teams"},

	// --- Deliberately no ambiance (contract §6.1) ---
	{"resendLEDOnReconnect", "sendLEDSetForBuzzer"}: {ambianceNoAmbiance, "resync of ONE reconnecting device; nothing changed in the game"},
	{"sendLEDSetComet", "sendLEDSetAllBuzzers"}:     {ambianceNoAmbiance, "AfterFunc +4.8 s restore — the SCORE pulse deadline already ends the scene at the same instant"},
	{"broadcastLEDSet", "sendLEDSet"}:               {ambianceNoAmbiance, "DEAD CODE (audit #132, zero call sites) — decision to revisit if ever reactivated"},
	{"sendLEDSetToTeam", "sendLEDSet"}:              {ambianceNoAmbiance, "DEAD CODE (audit #132, zero call sites) — decision to revisit if ever reactivated"},
	{"stop", "sendLEDSetAllEntracteOff"}:            {ambianceNoAmbiance, "server shutdown (#208, contract §10.4) — the process is exiting, nothing left to notify; the Hue side is switched off directly via LightingDriver().Apply() (shutdownExtinguishHueLighting), not through NotifyState()"},
}
