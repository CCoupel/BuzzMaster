package server

import "buzzcontrol/internal/protocol"

// inboundActionAllowlist is the INBOUND counterpart of the OUTBOUND filtering
// already established by SerializeForAdmin/SerializeForWebClient/
// SerializeForVPlayer/SerializeForBuzzer (internal/protocol/messages.go): it
// decides, per action, which ClientType(s) a web client (admin/tv/vplayer —
// see cmd/server/main.go's handleWebMessage, the sole consumer fed from
// WebSocketHub.Incoming) is allowed to SEND that action from.
//
// #154 (sec): before this file existed, handleWebMessage's dispatch switch
// never consulted the sending client's type at all — a client connected on
// /ws/tv or /ws/player (dedicated, supposedly reduced-capability endpoints)
// could send START/STOP/RAZ/DELETE/NEW_GAME/BUMPER_POINTS/... and the server
// executed them exactly as if an admin had sent them. This map is the fix:
// cmd/server's handleWebMessage consults IsActionAllowed before dispatching
// each action to its handler (contracts/websocket-actions.md).
//
// Design intent — kept deliberately generic (contracts/websocket-actions.md,
// see also the #155 Interface Animateur groundwork this doubles as): the
// policy is indexed purely by action name, not by handler or by any
// feature-specific concept, so a future ClientType (e.g. a "presenter" role)
// only needs new entries here, never a change to the dispatch mechanism
// itself. Actions absent from this map, or a ClientType absent from a given
// action's entry, are REJECTED — this is a closed allow-list, not an open
// list of exceptions carved out of an otherwise-permissive default. A newly
// added action that's forgotten here is rejected (fails loud, in the logs),
// never silently permitted.
//
// SET_CLIENT_TYPE is deliberately NOT an entry in this map — see
// IsSetClientTypeAllowed below: unlike every other action, whether it's
// permitted depends on the sender's CURRENT type, not a fixed list, so it
// can't be expressed as a static allow-list entry.
var inboundActionAllowlist = map[string][]ClientType{
	// --- Handshake ---------------------------------------------------
	// Every web client type identifies itself and expects its own
	// (type-filtered — see App.sendStateToClient, #154 E1) state in return.
	// ClientTypeAnim added v6.2.0 (#155) — interface animateur.
	protocol.ActionHello: {ClientTypeAdmin, ClientTypeTV, ClientTypeVPlayer, ClientTypeAnim},

	// --- Conduite en direct — admin + anim (v6.2.0, #155/#156) --------
	// Exact périmètre acté for the interface animateur (cadrage
	// _work/reports/plan-20260812-141735.md §1, plan B3
	// _work/reports/plan-20260813-092950.md §3): lancer/arrêter/mettre en
	// pause une question, révéler la réponse, enchaîner, créditer une équipe
	// ou un joueur — nothing else. Was merged into the admin-only block below
	// before #155; split out here so the ClientTypeAnim addition doesn't
	// silently widen anything else in that block (contracts/
	// websocket-actions.md documents this split explicitly).
	protocol.ActionReady:        {ClientTypeAdmin, ClientTypeAnim},
	protocol.ActionStart:        {ClientTypeAdmin, ClientTypeAnim},
	protocol.ActionStop:         {ClientTypeAdmin, ClientTypeAnim},
	protocol.ActionPause:        {ClientTypeAdmin, ClientTypeAnim},
	protocol.ActionContinue:     {ClientTypeAdmin, ClientTypeAnim},
	protocol.ActionReveal:       {ClientTypeAdmin, ClientTypeAnim},
	protocol.ActionBumperPoints: {ClientTypeAdmin, ClientTypeAnim},
	protocol.ActionTeamPoints:   {ClientTypeAdmin, ClientTypeAnim},

	// --- Game control (régie) — admin (/admin) only -------------------
	// Starting/stopping/scoring/resetting/deleting/configuring a game is
	// exclusively a régie action. web/src/hooks/useWebSocket.js only wires
	// these senders up from GamePage.jsx (admin routes) — never PlayerDisplay
	// (TV) nor VPlayerPage/EnrollPage (VPlayer), nor (v6.2.0) AnimPage.
	protocol.ActionFull:                  {ClientTypeAdmin},
	protocol.ActionUpdate:                {ClientTypeAdmin},
	protocol.ActionPoints:                {ClientTypeAdmin},
	protocol.ActionRAZ:                   {ClientTypeAdmin},
	protocol.ActionRemote:                {ClientTypeAdmin},
	protocol.ActionDelete:                {ClientTypeAdmin},
	protocol.ActionDeleteBumper:          {ClientTypeAdmin},
	protocol.ActionReleaseBumperName:     {ClientTypeAdmin},
	protocol.ActionReset:                 {ClientTypeAdmin},
	protocol.ActionReboot:                {ClientTypeAdmin},
	protocol.ActionReorderQuestions:      {ClientTypeAdmin},
	protocol.ActionForceReady:            {ClientTypeAdmin},
	protocol.ActionMemorySetTeams:        {ClientTypeAdmin},
	protocol.ActionMotionSetTeams:        {ClientTypeAdmin},
	protocol.ActionShowQRCode:            {ClientTypeAdmin},
	protocol.ActionHideQRCode:            {ClientTypeAdmin},
	protocol.ActionSetVirtualPlayerLimit: {ClientTypeAdmin},
	protocol.ActionNewGame:               {ClientTypeAdmin},
	protocol.ActionUpdateQuizMeta:        {ClientTypeAdmin},
	// ENTRACTE_SET (v6.5.2, #119, contracts/websocket-actions.md
	// §"ENTRACTE_SET"): admin only — deliberately NOT ClientTypeAnim, unlike
	// the "conduite en direct" block above. A pause engages the whole room,
	// not just the round in progress; the animateur sees it (filter +
	// indicator) but cannot trigger or lift it.
	protocol.ActionEntracteSet: {ClientTypeAdmin},
	// UPDATE_ENTRACTE_CONFIG (v6.5.2, #119, C1, contracts/websocket-actions.md
	// §"UPDATE_ENTRACTE_CONFIG"): saves the panel configuration from the Quiz
	// page — admin only, same reasoning as ENTRACTE_SET above.
	protocol.ActionUpdateEntracteConfig: {ClientTypeAdmin},
	// Code review MAJEUR-1 follow-up (v6.2.0, #155/#156): the admin pushes
	// its adjusted pointsInput to the server so it can be echoed to the
	// animateur via CREDIT_POINTS — anim only ever RECEIVES this value
	// (CREDIT_POINTS, contracts/websocket-actions.md §"Animateur"), it never
	// sends this action itself.
	protocol.ActionSetCreditPoints: {ClientTypeAdmin},
	// Handled directly in websocket.go's readPump (contract
	// ai-multi-provider.md §11), not through handleWebMessage's switch — kept
	// here anyway so IsActionAllowed remains the single source of truth that
	// call site consults, instead of a second hardcoded rule drifting apart
	// from this one.
	protocol.ActionCancelAIGeneration: {ClientTypeAdmin},
	// Messagerie régie (v6.4.x, #167 — contracts/websocket-actions.md
	// §"Messagerie régie"): unidirectional by design (issue #167 — "pas de
	// réponse possible"). Anim has no path to SEND, deliberately: it must
	// not acquire one as a side effect of the CLEAR entry just below.
	protocol.ActionRegieMessageSend: {ClientTypeAdmin},

	// --- Messagerie régie — admin + anim, NOT "conduite en direct" ----
	// (v6.4.x, #167 — contracts/websocket-actions.md §"Messagerie régie")
	// REGIE_MESSAGE_CLEAR is one action for two intents: admin sends it to
	// retire a message sent by mistake, anim sends it to acknowledge ("Vu").
	// The server-side EFFECT is identical either way (clear the single active
	// message, broadcast the clear) — the distinction lives only in
	// ClientType, which the handler uses to deduce CLEARED_BY ("REGIE" vs
	// "ANIM"), never read from the payload itself.
	//
	// ⚠️ CAPABILITY WIDENING, a different class than #159/#160's: those let
	// anim MUTATE game state (still bounded by engine guards). This lets anim
	// mutate a state OUTSIDE the engine, with a GLOBAL effect — one tablet's
	// "Vu" clears the message for every tablet AND for régie. Deliberate
	// (D3, single message → single acknowledgment), not a side effect; no
	// engine guard applies or is added, valid in every phase including
	// NEW_GAME (D5).
	protocol.ActionRegieMessageClear: {ClientTypeAdmin, ClientTypeAnim},

	// --- BUTTON / PONG — dual purpose, NOT admin-only -----------------
	// #154 code review (CRITIQUE 1): these two are the REAL VJoueur gameplay
	// path, not just admin debug tooling — web/src/pages/VPlayerPage.jsx
	// (routed at /player, App.jsx) sends both directly via sendMessage(),
	// bypassing the named simulateButton/simulatePong wrappers that a
	// function-name-only frontend audit would have caught:
	//   - VPlayerPage.jsx:386 — PONG, auto-sent on entering PREPARE (the
	//     readiness handshake every VJoueur performs, handlePong ->
	//     SetBumperReady/TransitionToReady).
	//   - VPlayerPage.jsx:429,560 — BUTTON — the buzzer press itself
	//     (handleBuzz), the central VJoueur gameplay action.
	// Admin also legitimately sends both, via GamePage.jsx's simulateButton/
	// simulatePong debug tools (useWebSocket.js sendButton/sendPong) — hence
	// both types are listed, not VPlayer alone.
	protocol.ActionButton: {ClientTypeAdmin, ClientTypeVPlayer},
	protocol.ActionPong:   {ClientTypeAdmin, ClientTypeVPlayer},

	// --- MEMORY card interaction ---------------------------------------
	// FLIP_MEMORY_CARD: TV carries BOTH the admin preview iframe (/tv?admin=true,
	// embedded read-only in GamePage but connects as a genuine ClientTypeTV —
	// PlayerDisplay.jsx isAdminPreview) AND a real spectator screen that lets
	// the active team's own VPlayer click their own card through
	// (PlayerDisplay.jsx canClick / isVPlayerInActiveTeam, contract
	// game-state.md MEMORY_CURRENT_TEAM).
	//
	// ⚠️ CAPABILITY WIDENING (v6.2.x, #159 B1, contracts/websocket-actions.md
	// §Allow-list, contracts/CHANGELOG.md [20260816-3]): ClientTypeAnim added
	// — the FIRST action where the interface animateur can directly MUTATE
	// game state (flip a MEMORY card) rather than only ARBITRATE it
	// (start/stop/reveal/credit, the "conduite en direct" block above). The
	// engine's own phase guard (flip rejected outside PhaseStarted) applies
	// unchanged to this new sender — no engine change, no additional guard
	// added here. `admin` deliberately stays OUT of this entry: the régie
	// flips cards through its embedded TV preview iframe (a genuine
	// ClientTypeTV connection, not admin itself) — unchanged by this lot.
	// ActionMemorySetTeams (above, admin-only) is NOT touched either.
	protocol.ActionFlipMemoryCard: {ClientTypeTV, ClientTypeVPlayer, ClientTypeAnim},

	// --- MEMOTION card interaction --------------------------------------
	// Mirrors the MEMORY block above, but for the five sub-phases
	// (MEMORIZE -> GRID -> SELECTED -> QUESTION -> REVEAL) of a MEMOTION
	// round. Until #160 these five actions were split across the admin-only
	// "Game control (régie)" block (Flip/StopTimer/Reveal/Done) and this
	// section (Select, TV-only) — moved here as one documented unit so the
	// full MEMOTION conduct surface reads together.
	//
	// ⚠️ CAPABILITY WIDENING (v6.2.x, #160 B1, contracts/websocket-actions.md
	// §Allow-list, contracts/CHANGELOG.md [20260817-3]): ClientTypeAnim added
	// to all five — the interface animateur conducts a MEMOTION round
	// (select a card, flip it, stop the reveal timer, reveal the answer,
	// credit the round) directly from its tablet, exactly as #159 did for
	// MEMORY. The engine's own phase/subphase guards (handleMotion* in
	// cmd/server/main.go) apply unchanged to this new sender — no engine
	// change, no additional guard added here.
	//
	// `tv` stays on ActionMotionSelect only (the admin preview iframe,
	// PlayerDisplay.jsx canSelectCard requires isAdminPreview — a genuine
	// ClientTypeTV connection, the iframe's path is still /tv, never
	// VPlayer). `admin` stays on the four others (régie conduct panel).
	// Neither loses a right — this lot only adds `anim` to each entry.
	// ActionMotionSetTeams (above, admin-only) is deliberately NOT touched:
	// the choice of participating teams stays with the régie, exactly as
	// ActionMemorySetTeams does for MEMORY (#159).
	protocol.ActionMotionSelect:    {ClientTypeTV, ClientTypeAnim},
	protocol.ActionMotionFlip:      {ClientTypeAdmin, ClientTypeAnim},
	protocol.ActionMotionStopTimer: {ClientTypeAdmin, ClientTypeAnim},
	protocol.ActionMotionReveal:    {ClientTypeAdmin, ClientTypeAnim},
	protocol.ActionMotionDone:      {ClientTypeAdmin, ClientTypeAnim},

	// --- VPlayer-only -------------------------------------------------
	protocol.ActionPlayerConnect:    {ClientTypeVPlayer},
	protocol.ActionVPlayerQCMAnswer: {ClientTypeVPlayer},
	protocol.ActionArdoiseInput:     {ClientTypeVPlayer},
}

// IsActionAllowed reports whether clientType may send action, per
// inboundActionAllowlist. Default-deny: an action absent from the map, or a
// clientType absent from that action's entry, is rejected — including an
// empty/unrecognized clientType (e.g. an IncomingMessage a test built
// without setting ClientType, or a future client type nobody has taught this
// map about yet).
//
// SET_CLIENT_TYPE is NOT handled here — see IsSetClientTypeAllowed, whose
// rule depends on the sender's CURRENT type rather than a fixed list.
func IsActionAllowed(action string, clientType ClientType) bool {
	allowed, ok := inboundActionAllowlist[action]
	if !ok {
		return false
	}
	for _, t := range allowed {
		if t == clientType {
			return true
		}
	}
	return false
}

// IsSetClientTypeAllowed reports whether a client currently identified as
// currentType may send SET_CLIENT_TYPE to self-declare a (possibly
// different) type.
//
// Only ClientTypeAdmin — the default a connection on the legacy /ws endpoint
// starts with (HandleConnection), before it has self-identified as tv/
// vplayer via this very message — may use it. This is what actually closes
// #154 E3: cmd/server's handleSetClientType maps any TYPE value it doesn't
// recognize ("tv"/"vplayer") to Admin by default. Without this gate, a
// client connected on a DEDICATED /ws/tv or /ws/player endpoint (fixed type
// at connection time, HandleConnectionWithType — no legitimate reason to
// ever send SET_CLIENT_TYPE at all) could send SET_CLIENT_TYPE with any
// value and be silently promoted to Admin by that default branch.
//
// Once a legacy /ws connection has declared itself tv/vplayer, it can no
// longer re-declare (its currentType is no longer Admin) — closing the same
// escalation path for it too, while still allowing the one legitimate
// handshake legacy clients rely on.
func IsSetClientTypeAllowed(currentType ClientType) bool {
	return currentType == ClientTypeAdmin
}

// entracteAllowedActions is the SECOND, orthogonal gate applied while
// GameState.ENTRACTE is true (v6.5.2, #119, D6, contracts/websocket-actions.md
// §"Actions refusées pendant l'entracte") — checked by cmd/server/main.go's
// handleWebMessage AFTER inboundActionAllowlist/IsActionAllowed above, never
// instead of it. A CLOSED allow-list, same default-deny discipline as
// inboundActionAllowlist: an action absent here is refused while entracte is
// active, logged, and never reaches its handler — including any action added
// to the protocol later without an explicit decision recorded here.
//
// Rationale for each entry (contract, D6):
//   - ENTRACTE_SET: the only way OUT — without it entracte would be a dead
//     end.
//   - HELLO, SET_CLIENT_TYPE: handshake — a screen that reloads mid-pause
//     must be able to reconnect and receive state (which now includes
//     ENTRACTE/ENTRACTE_CONFIG, same UPDATE).
//   - PLAYER_CONNECT: a VJoueur reloading their phone mid-pause must recover
//     their seat.
//   - REGIE_MESSAGE_SEND/CLEAR: régie → anim messaging is exactly as useful
//     during a pause as during play — arguably more so.
//   - UPDATE_ENTRACTE_CONFIG (v6.5.2, C1/C4): preparing the NEXT pause's
//     panel while the current one is showing is explicitly the point of the
//     freeze design (contract game-state.md §"Configuration gelée à
//     l'activation") — the save succeeds and persists, but has NO effect on
//     the panel already displayed (Engine.SetEntracteConfig only refreshes
//     the diffused config when !Entracte). This is the ONLY configuration
//     action admitted here, and it's admitted without an exception carved
//     into the general rule below — it simply never changes anything about
//     the pause in progress.
//   - PONG: harmless (no transition depends on it outside PREPARE) and
//     avoids making a buzzer look dead to whatever polls it.
//
// Everything else — READY/START/STOP/PAUSE/CONTINUE/REVEAL/REMOTE/NEW_GAME/
// RAZ/SHOW_QR_CODE/UPDATE_QUIZ_META/point credits/MEMORY*/MEMOTION*/
// ARDOISE_INPUT/... — is refused. Physical buzz presses need no entry here:
// handleButton already no-ops outside PhaseStarted, and entracte is only
// reachable outside PhaseStarted (D4) — see the dedicated non-regression
// test instead of a guard here.
var entracteAllowedActions = map[string]bool{
	protocol.ActionEntracteSet:          true,
	protocol.ActionHello:                true,
	protocol.ActionSetClientType:        true,
	protocol.ActionPlayerConnect:        true,
	protocol.ActionRegieMessageSend:     true,
	protocol.ActionRegieMessageClear:    true,
	protocol.ActionUpdateEntracteConfig: true,
	protocol.ActionPong:                 true,
}

// IsActionAllowedDuringEntracte reports whether action may proceed while
// GameState.ENTRACTE is true. Called only when entracte is actually active;
// callers must apply the normal IsActionAllowed/IsSetClientTypeAllowed gate
// first — this is a second, narrower gate on top, not a replacement.
func IsActionAllowedDuringEntracte(action string) bool {
	return entracteAllowedActions[action]
}
