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
	protocol.ActionHello: {ClientTypeAdmin, ClientTypeTV, ClientTypeVPlayer},

	// --- Game control (régie) — admin (/admin, /anim) only -----------
	// Starting/stopping/scoring/resetting/deleting/configuring a game is
	// exclusively a régie action. web/src/hooks/useWebSocket.js only wires
	// these senders up from GamePage.jsx (admin routes) — never PlayerDisplay
	// (TV) nor VPlayerPage/EnrollPage (VPlayer).
	protocol.ActionFull:                  {ClientTypeAdmin},
	protocol.ActionUpdate:                {ClientTypeAdmin},
	protocol.ActionPoints:                {ClientTypeAdmin},
	protocol.ActionReady:                 {ClientTypeAdmin},
	protocol.ActionStart:                 {ClientTypeAdmin},
	protocol.ActionStop:                  {ClientTypeAdmin},
	protocol.ActionPause:                 {ClientTypeAdmin},
	protocol.ActionContinue:              {ClientTypeAdmin},
	protocol.ActionReveal:                {ClientTypeAdmin},
	protocol.ActionRAZ:                   {ClientTypeAdmin},
	protocol.ActionRemote:                {ClientTypeAdmin},
	protocol.ActionDelete:                {ClientTypeAdmin},
	protocol.ActionDeleteBumper:          {ClientTypeAdmin},
	protocol.ActionReleaseBumperName:     {ClientTypeAdmin},
	protocol.ActionReset:                 {ClientTypeAdmin},
	protocol.ActionReboot:                {ClientTypeAdmin},
	protocol.ActionBumperPoints:          {ClientTypeAdmin},
	protocol.ActionTeamPoints:            {ClientTypeAdmin},
	protocol.ActionReorderQuestions:      {ClientTypeAdmin},
	protocol.ActionForceReady:            {ClientTypeAdmin},
	protocol.ActionButton:                {ClientTypeAdmin}, // simulated buzzer press (debug/testing)
	protocol.ActionPong:                  {ClientTypeAdmin}, // simulated buzzer PONG (debug/testing)
	protocol.ActionMemorySetTeams:        {ClientTypeAdmin},
	protocol.ActionMotionFlip:            {ClientTypeAdmin},
	protocol.ActionMotionStopTimer:       {ClientTypeAdmin},
	protocol.ActionMotionReveal:          {ClientTypeAdmin},
	protocol.ActionMotionDone:            {ClientTypeAdmin},
	protocol.ActionMotionSetTeams:        {ClientTypeAdmin},
	protocol.ActionShowQRCode:            {ClientTypeAdmin},
	protocol.ActionHideQRCode:            {ClientTypeAdmin},
	protocol.ActionSetVirtualPlayerLimit: {ClientTypeAdmin},
	protocol.ActionNewGame:               {ClientTypeAdmin},
	protocol.ActionUpdateQuizMeta:        {ClientTypeAdmin},
	// Handled directly in websocket.go's readPump (contract
	// ai-multi-provider.md §11), not through handleWebMessage's switch — kept
	// here anyway so IsActionAllowed remains the single source of truth that
	// call site consults, instead of a second hardcoded rule drifting apart
	// from this one.
	protocol.ActionCancelAIGeneration: {ClientTypeAdmin},

	// --- MEMORY / MEMOTION card interaction ---------------------------
	// FLIP_MEMORY_CARD: TV carries BOTH the admin preview iframe (/tv?admin=true,
	// embedded read-only in GamePage but connects as a genuine ClientTypeTV —
	// PlayerDisplay.jsx isAdminPreview) AND a real spectator screen that lets
	// the active team's own VPlayer click their own card through
	// (PlayerDisplay.jsx canClick / isVPlayerInActiveTeam, contract
	// game-state.md MEMORY_CURRENT_TEAM).
	protocol.ActionFlipMemoryCard: {ClientTypeTV, ClientTypeVPlayer},
	// MEMOTION_SELECT: only ever sent from the admin preview iframe
	// (PlayerDisplay.jsx canSelectCard requires isAdminPreview) — that
	// connection is still a genuine ClientTypeTV (the iframe's path is
	// still /tv), never VPlayer.
	protocol.ActionMotionSelect: {ClientTypeTV},

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
