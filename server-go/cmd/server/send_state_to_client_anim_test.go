package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// Tests: #155/#156 (v6.2.0) — sendStateToClient (HELLO) for ClientTypeAnim
// (plan tâche B1/B4), plus the B4 regression on TV/VPlayer.
//
// contracts/CHANGELOG.md [20260813] documents B4 as a [CHANGED] contract
// entry: the QUESTIONS block, sent unconditionally today, becomes admin-only
// — TV and VPlayer stop receiving it at HELLO (an explicitly intended side
// effect, aligning with broadcastQuestions' already-admin-only policy). That
// authorizes covering the NEW (post-B4) TV/VPlayer behavior here, in an
// additive file, rather than editing the existing assertions in
// send_state_to_client_test.go (which currently read ActionQuestions for
// VPlayer/Admin/TV — dev-backend's own B4 change is expected to update
// those three call sites; this file does not touch them).
//
// IMPORTANT: deliberately does NOT use readActionMatching (main_broadcast_
// 127_test.go) for the "must NOT receive" assertions — that helper skips
// past any action that isn't the one it's looking for, which would silently
// swallow a QUESTIONS message instead of catching it. collectActionsT below
// drains everything sendStateToClient sent within a short window instead.
//
// The anim-specific tests below dial through startAnimAllowlistTestServer
// (defined in inbound_allowlist_anim_test.go, same package) rather than
// startEvictionTestServer (player_evicted_test.go) — that shared #154
// helper's path-suffix switch predates #155 and has no "/anim" case, so an
// "/anim"-suffixed path would silently fall through to its VPlayer default.
// ---------------------------------------------------------------------------

// collectActionsT drains every frame conn receives before timeout elapses,
// returning their ACTION values in arrival order. Unlike readActionMatching,
// it never stops early on a match — every message in the window is recorded,
// so a message the caller does NOT want to see is actually caught, not
// silently skipped past.
func collectActionsT(t *testing.T, conn *websocket.Conn, timeout time.Duration) []string {
	t.Helper()
	var actions []string
	deadline := time.Now().Add(timeout)
	for {
		conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			return actions
		}
		var envelope struct {
			Action string `json:"ACTION"`
		}
		if json.Unmarshal(data, &envelope) == nil {
			actions = append(actions, envelope.Action)
		}
	}
}

// containsAction is shared with player_evicted_test.go (package main) — same
// signature, no local redefinition needed here.

// TestSendStateToClient_Anim_StripsAdminFieldsAndExcludesQuestionsClientsConfig
// is B1+B4's combined acceptance criterion for the animateur's HELLO: the
// UPDATE it receives must be the web-client-filtered payload (no
// FIRMWARE_VERSION), and it must never receive QUESTIONS, CLIENTS nor
// CONFIG_UPDATE (contracts/websocket-endpoints.md "Filtres de diffusion par
// type": UPDATE ✓ partiel, QUESTIONS ✗, CLIENTS ✗, CONFIG_UPDATE ✗ for the
// animateur column).
//
// CLIENTS assertion added post-code-review (MAJEUR-2, verdict APPROUVÉ AVEC
// RÉSERVES sur `835cd07`..`d7336ed`): sendStateToClient's CLIENTS block sent
// unconditionally to every connecting type until that fix, contradicting
// contracts/websocket-endpoints.md (written in this same lot) and B6's own
// acceptance criterion ("aucune information de concurrence n'est envoyée au
// client animateur" — CLIENTS, who else is connected, is exactly that).
func TestSendStateToClient_Anim_StripsAdminFieldsAndExcludesQuestionsClientsConfig(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.UpdateBumper("bumper-1", map[string]interface{}{
		"TEAM":             "TeamA",
		"FIRMWARE_VERSION": "1.2.3",
	})
	// sendStateToClient's CONFIG_UPDATE gate only reaches Admin/TV in its
	// `if` at all — anim is excluded by construction (not even evaluated),
	// so a.httpServer is deliberately left nil here (same reasoning as the
	// existing VPlayer test in send_state_to_client_test.go).

	baseURL := startAnimAllowlistTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/anim")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeAnim)

	actions := collectActionsT(t, conn, 400*time.Millisecond)

	if !containsAction(actions, protocol.ActionUpdate) {
		t.Fatalf("anim never received UPDATE at HELLO — got actions: %v", actions)
	}
	if containsAction(actions, protocol.ActionQuestions) {
		t.Errorf("#155/#156 B4: anim must NEVER receive QUESTIONS at HELLO — got actions: %v", actions)
	}
	if containsAction(actions, protocol.ActionConfigUpdate) {
		t.Errorf("#155/#156 B1: anim must NEVER receive CONFIG_UPDATE — got actions: %v", actions)
	}
	if containsAction(actions, protocol.ActionClients) {
		t.Errorf("#155/#156 code review MAJEUR-2: anim must NEVER receive CLIENTS (concurrency info) — got actions: %v", actions)
	}
}

// TestSendStateToClient_Anim_UpdateStripsAdminOnlyFields is the field-level
// counterpart: confirms the UPDATE payload itself (not just which actions
// arrive) is web-client-filtered for anim, same as TV/VPlayer.
func TestSendStateToClient_Anim_UpdateStripsAdminOnlyFields(t *testing.T) {
	app := newTestAppWithHub(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.UpdateBumper("bumper-1", map[string]interface{}{
		"TEAM":             "TeamA",
		"FIRMWARE_VERSION": "1.2.3",
	})

	baseURL := startAnimAllowlistTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/anim")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeAnim)

	_, updateRaw := readActionMatching(t, conn, protocol.ActionUpdate)
	var updateEnvelope struct {
		Msg struct {
			Bumpers map[string]map[string]interface{} `json:"bumpers"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(updateRaw), &updateEnvelope); err != nil {
		t.Fatalf("failed to unmarshal UPDATE: %v (raw: %s)", err, updateRaw)
	}
	if _, ok := updateEnvelope.Msg.Bumpers["bumper-1"]["FIRMWARE_VERSION"]; ok {
		t.Error("anim must not receive FIRMWARE_VERSION on its HELLO UPDATE (SerializeForWebClient)")
	}
}

// TestSendStateToClient_TV_NoLongerReceivesQuestionsAtHello is B4's
// documented [CHANGED] regression on TV — contracts/CHANGELOG.md [20260813]:
// "TV et VPlayer cessent de recevoir QUESTIONS au HELLO (ils ne le
// recevaient déjà plus ensuite)". Currently red (QUESTIONS is still sent
// unconditionally) until B4 lands.
func TestSendStateToClient_TV_NoLongerReceivesQuestionsAtHello(t *testing.T) {
	app := newTestAppWithHub(t)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/tv")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeTV)

	actions := collectActionsT(t, conn, 400*time.Millisecond)
	if containsAction(actions, protocol.ActionQuestions) {
		t.Errorf("#155/#156 B4 [CHANGED]: TV must no longer receive QUESTIONS at HELLO — got actions: %v", actions)
	}
	// TV keeps receiving CONFIG_UPDATE — that gate is untouched by B4.
	if !containsAction(actions, protocol.ActionConfigUpdate) {
		t.Errorf("TV should still receive CONFIG_UPDATE (unaffected by B4) — got actions: %v", actions)
	}
}

// TestSendStateToClient_VPlayer_NoLongerReceivesQuestionsAtHello is the
// VPlayer half of the same B4 [CHANGED] regression.
func TestSendStateToClient_VPlayer_NoLongerReceivesQuestionsAtHello(t *testing.T) {
	app := newTestAppWithHub(t)

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/player")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeVPlayer)

	actions := collectActionsT(t, conn, 400*time.Millisecond)
	if containsAction(actions, protocol.ActionQuestions) {
		t.Errorf("#155/#156 B4 [CHANGED]: VPlayer must no longer receive QUESTIONS at HELLO — got actions: %v", actions)
	}
}

// TestSendStateToClient_Admin_StillReceivesQuestionsAtHello is the control:
// B4 only narrows the gate to admin-only, it must not remove it for admin
// itself (contracts/CHANGELOG.md: "alignement sur broadcastQuestions, déjà
// admin-only en continu").
// ---------------------------------------------------------------------------
// Tests: v6.4.x (#167) — REGIE_MESSAGE replay at HELLO (plan tâche B7, T6,
// contracts/websocket-actions.md §"Messagerie régie" D6: "livraison différée
// plutôt que perdue" — a tablette in standby / reconnecting Wi-Fi must not
// miss an active consigne).
//
// IMPORTANT (B7's own warning): this must be a TARGETED SendToClient, never
// a.broadcastRegieMessage() — that would re-push to every OTHER already-
// connected admin/anim connection too, exactly the NEXT_QUESTION/
// CREDIT_POINTS/AWARDED_TEAMS mistake already guarded against just above in
// this same file. TestSendStateToClient_RegieMessage_TargetedNotRebroadcast
// below is the test that would catch a re-broadcast implementation.
// ---------------------------------------------------------------------------

func TestSendStateToClient_Anim_ReplaysActiveRegieMessageAtHello(t *testing.T) {
	app := newAnimTestApp(t)
	app.regieMessage = &protocol.RegieMessagePayload{
		Active: true,
		Text:   "Question 12 annulée — enchaîne sur la 13",
		SentAt: 1755511234567,
	}

	baseURL := startAnimAllowlistTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/anim")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeAnim)

	_, raw := readActionMatching(t, conn, protocol.ActionRegieMessage)
	var envelope struct {
		Msg protocol.RegieMessagePayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("failed to unmarshal REGIE_MESSAGE: %v (raw: %s)", err, raw)
	}
	if !envelope.Msg.Active || envelope.Msg.Text != "Question 12 annulée — enchaîne sur la 13" {
		t.Errorf("#167 D6: a reconnecting anim must receive the active regie message at HELLO — got %+v", envelope.Msg)
	}
}

// TestSendStateToClient_Admin_ReplaysActiveRegieMessageAtHello — the régie
// itself must also see its own still-pending message when reconnecting
// (contracts/websocket-actions.md: REGIE_MESSAGE targets "admin ET anim"),
// so it can tell "still awaiting acquittement" apart from "already cleared".
func TestSendStateToClient_Admin_ReplaysActiveRegieMessageAtHello(t *testing.T) {
	app := newAnimTestApp(t)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	app.regieMessage = &protocol.RegieMessagePayload{Active: true, Text: "En attente de retour", SentAt: 42}

	baseURL := startAnimAllowlistTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/admin")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeAdmin)

	readActionMatching(t, conn, protocol.ActionRegieMessage) // fails (timeout) if never received
}

// TestSendStateToClient_Anim_NoRegieMessageSentWhenInactive is B7's negative
// case: no active message at all means nothing is sent — a freshly
// (re)connecting tablet with no pending consigne must not receive a
// REGIE_MESSAGE frame it would have to interpret as "nothing to see", it
// simply receives none.
func TestSendStateToClient_Anim_NoRegieMessageSentWhenInactive(t *testing.T) {
	app := newAnimTestApp(t)
	// app.regieMessage left nil — no message was ever sent this session.

	baseURL := startAnimAllowlistTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/anim")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeAnim)

	actions := collectActionsT(t, conn, 400*time.Millisecond)
	if containsAction(actions, protocol.ActionRegieMessage) {
		t.Errorf("#167 B7: no REGIE_MESSAGE should be sent at HELLO when no message is active — got actions: %v", actions)
	}
}

// TestSendStateToClient_Anim_NoRegieMessageReplayWhenAlreadyCleared is B7's
// gate written precisely: "si regieMessage != nil ET Active" — a
// (re)connecting tablet must not be replayed a STALE already-cleared
// message either. Distinct from the nil case above: here a.regieMessage is
// non-nil (a message was sent and acknowledged earlier this session) but
// Active is false — the reconnecting client's own default rest state
// already covers this case, nothing needs to be pushed.
func TestSendStateToClient_Anim_NoRegieMessageReplayWhenAlreadyCleared(t *testing.T) {
	app := newAnimTestApp(t)
	app.regieMessage = &protocol.RegieMessagePayload{Active: false, Text: "Ancienne consigne acquittée", SentAt: 0, ClearedBy: "ANIM"}

	baseURL := startAnimAllowlistTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/anim")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeAnim)

	actions := collectActionsT(t, conn, 400*time.Millisecond)
	if containsAction(actions, protocol.ActionRegieMessage) {
		t.Errorf("#167 B7: a (re)connecting client must not be replayed an already-cleared message — got actions: %v", actions)
	}
}

// TestSendStateToClient_RegieMessage_TargetedNotRebroadcast is B7's explicit
// warning, made concrete: a SECOND anim tablet connecting while a first one
// is already connected must not cause the FIRST tablet to receive a
// duplicate REGIE_MESSAGE — the replay is a targeted SendToClient(new
// clientID, ...), never a.broadcastRegieMessage() (which would fan out to
// every already-connected admin/anim connection, not just the new one).
func TestSendStateToClient_RegieMessage_TargetedNotRebroadcast(t *testing.T) {
	app := newAnimTestApp(t)
	app.regieMessage = &protocol.RegieMessagePayload{Active: true, Text: "Consigne unique", SentAt: 1}

	baseURL := startAnimAllowlistTestServer(t, app)
	firstAnim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, firstAnim)
	// Drain the HELLO REGIE_MESSAGE the first tablet legitimately receives
	// for itself (dialWS doesn't send HELLO — learnClientID uses PONG — so
	// nothing has been sent to firstAnim yet at this point).

	secondClientID := learnClientID(t, app, dialWS(t, baseURL, "/ws/anim"))

	app.sendStateToClient(secondClientID, server.ClientTypeAnim)

	firstAnimActions := collectActionsT(t, firstAnim, 300*time.Millisecond)
	if containsAction(firstAnimActions, protocol.ActionRegieMessage) {
		t.Errorf("#167 B7: sendStateToClient for a NEW connection must not re-push REGIE_MESSAGE to an already-connected tablet — got actions on the first tablet: %v", firstAnimActions)
	}
}

func TestSendStateToClient_Admin_StillReceivesQuestionsAtHello(t *testing.T) {
	app := newTestAppWithHub(t)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/admin")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeAdmin)

	actions := collectActionsT(t, conn, 400*time.Millisecond)
	if !containsAction(actions, protocol.ActionQuestions) {
		t.Errorf("admin must still receive QUESTIONS at HELLO — got actions: %v", actions)
	}
}
