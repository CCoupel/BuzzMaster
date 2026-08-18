package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// Tests: #162 (bugfix) — no broadcast reaches /ws/anim except NEXT_QUESTION
// and CREDIT_POINTS. Plan: _work/reports/plan-20260813-174513.md §T3.
//
// T1 (prérequis dur) adds a ClientTypeAnim branch to broadcastUpdateTo
// itself (cmd/server/main.go:2665) — the "chemin A" call sites
// (broadcastStart/Stop/Pause/Continue/Reveal/broadcastTimerUpdate/
// broadcastQCMHint/...) already go through serializeForClientType
// (websocket.go), which has had a ClientTypeAnim case since #155 B1; only
// their type LISTS need Anim added (T2). broadcastUpdate/broadcastUpdateTo
// ("chemin B") is different: it serializes by hand-written branches with NO
// Anim branch at all — adding ClientTypeAnim to a CALLER's type list
// without first adding the internal branch is a silent no-op (plan §2.1,
// R1). TestBroadcastUpdateTo_Anim_ReceivesWebClientPayload is the test that
// catches exactly that failure mode.
//
// Harness: same conventions as inbound_allowlist_anim_test.go
// (newAnimTestApp/startAnimAllowlistTestServer, same package) and
// main_broadcast_127_test.go (newBroadcast127TestApp/setupPrepareReadyGame/
// pongMsg/readyMsg/collectActions/countActions) for the handlePong
// exclusion test — reused directly, not redefined.
// ---------------------------------------------------------------------------

// TestBroadcastUpdateTo_Anim_ReceivesWebClientPayload is the piège-closing
// test (plan §2.1, §3 T3): two phases in one test.
//
//  1. broadcastUpdateTo(ClientTypeAnim) — Anim ALONE, no TV/VPlayer in the
//     type list — must still deliver an UPDATE. If T1's fix only widens the
//     boolean guard on ClientTypeAnim without genuinely gating dataWeb's
//     computation on it (or forgets the guard extension entirely), this
//     phase fails: a bug where dataWeb is only computed "if targetTV ||
//     targetVPlayer" would leave Anim-only broadcasts silently empty.
//  2. broadcastUpdateTo(ClientTypeAnim, ClientTypeTV) together — both
//     branches reuse the exact same dataWeb bytes, so the two clients must
//     receive BYTE-IDENTICAL frames. This is the genuine parity guarantee
//     (not just "some stripped payload", but literally TV's payload) —
//     comparing across two SEPARATE calls would be flaky (each
//     protocol.NewMessage stamps a fresh TIME_EVENT), so both frames come
//     from the same call/same msg on purpose.
func TestBroadcastUpdateTo_Anim_ReceivesWebClientPayload(t *testing.T) {
	app := newAnimTestApp(t)
	app.engine.UpdateBumper("bumper-1", map[string]interface{}{
		"TEAM":             "TeamA",
		"FIRMWARE_VERSION": "1.2.3", // admin-only field — must be stripped for Anim, like TV
	})

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	// Phase 1 — Anim alone.
	app.broadcastUpdateTo(server.ClientTypeAnim)
	_, animOnlyRaw := readActionMatching(t, anim, protocol.ActionUpdate)
	var animEnvelope struct {
		Msg struct {
			Bumpers map[string]map[string]interface{} `json:"bumpers"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(animOnlyRaw), &animEnvelope); err != nil {
		t.Fatalf("failed to unmarshal anim-only UPDATE: %v (raw: %s)", err, animOnlyRaw)
	}
	if _, ok := animEnvelope.Msg.Bumpers["bumper-1"]["FIRMWARE_VERSION"]; ok {
		t.Error("#162: anim (targeted alone) must receive the web-client-filtered payload (no FIRMWARE_VERSION), not the admin payload")
	}

	// Phase 2 — Anim and TV together, byte-for-byte comparison.
	tv := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tv)
	app.broadcastUpdateTo(server.ClientTypeAnim, server.ClientTypeTV)
	_, animRaw := readActionMatching(t, anim, protocol.ActionUpdate)
	_, tvRaw := readActionMatching(t, tv, protocol.ActionUpdate)
	if animRaw != tvRaw {
		t.Errorf("#162: anim and TV must receive byte-identical UPDATE frames (same dataWeb) — anim: %s\ntv: %s", animRaw, tvRaw)
	}
}

// TestHandleTeamPoints_BroadcastsToAnim reproduces the exact scenario from
// the deployer's report (_work/reports/deployer-20260813-171200.md): an
// admin credits a team, and the connected animateur must see the updated
// score without reconnecting.
func TestHandleTeamPoints_BroadcastsToAnim(t *testing.T) {
	app := newAnimTestApp(t)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "TeamA", Points: 25})

	_, raw := readActionMatching(t, anim, protocol.ActionUpdate)
	var envelope struct {
		Msg struct {
			Teams map[string]map[string]interface{} `json:"teams"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("failed to unmarshal UPDATE: %v (raw: %s)", err, raw)
	}
	score, _ := envelope.Msg.Teams["TeamA"]["SCORE"].(float64)
	if score != 25 {
		t.Errorf("#162: anim did not see TeamA's updated score after TEAM_POINTS — got %v, want 25 (raw: %s)", envelope.Msg.Teams["TeamA"]["SCORE"], raw)
	}
}

// TestHandleBumperPoints_BroadcastsToAnim is BUMPER_POINTS' half of the same
// deployer scenario.
func TestHandleBumperPoints_BroadcastsToAnim(t *testing.T) {
	app := newAnimTestApp(t)
	app.engine.UpdateBumper("bumper-1", map[string]interface{}{"TEAM": "TeamA"})

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionBumperPoints, protocol.BumperPointsPayload{ID: "bumper-1", Points: 15})

	_, raw := readActionMatching(t, anim, protocol.ActionUpdate)
	var envelope struct {
		Msg struct {
			Bumpers map[string]map[string]interface{} `json:"bumpers"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("failed to unmarshal UPDATE: %v (raw: %s)", err, raw)
	}
	score, _ := envelope.Msg.Bumpers["bumper-1"]["SCORE"].(float64)
	if score != 15 {
		t.Errorf("#162: anim did not see bumper-1's updated score after BUMPER_POINTS — got %v, want 15 (raw: %s)", envelope.Msg.Bumpers["bumper-1"]["SCORE"], raw)
	}
}

// assertActionReachesAnim drives action (sent by admin) through the real
// dispatch path and confirms the connected anim client receives that same
// ACTION back — "chemin A" (broadcastStart/Stop/Pause/Continue/Reveal),
// already routed through serializeForClientType (has a ClientTypeAnim case
// since #155 B1); only T2's type-list addition is under test here.
func assertActionReachesAnim(t *testing.T, action string, payload interface{}, setupPhase game.GamePhase) {
	t.Helper()
	app := newAnimTestApp(t)
	app.engine.SetPhase(setupPhase)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	sendAction(t, app, admin, action, payload)

	readActionMatching(t, anim, action) // fails the test (timeout) if never received
}

func TestBroadcastStart_IncludesAnim(t *testing.T) {
	assertActionReachesAnim(t, protocol.ActionStart, protocol.StartPayload{Delay: 1}, game.PhaseReady)
}

func TestBroadcastStop_IncludesAnim(t *testing.T) {
	assertActionReachesAnim(t, protocol.ActionStop, struct{}{}, game.PhaseStarted)
}

func TestBroadcastPause_IncludesAnim(t *testing.T) {
	assertActionReachesAnim(t, protocol.ActionPause, struct{}{}, game.PhaseStarted)
}

func TestBroadcastContinue_IncludesAnim(t *testing.T) {
	assertActionReachesAnim(t, protocol.ActionContinue, struct{}{}, game.PhasePaused)
}

func TestBroadcastReveal_IncludesAnim(t *testing.T) {
	assertActionReachesAnim(t, protocol.ActionReveal, struct{}{}, game.PhaseStopped)
}

// TestBroadcastTimerUpdate_IncludesAnim — the ticking chronometer (zone A)
// depends on this: without it, "la tablette est figée" per the plan's
// symptom description.
func TestBroadcastTimerUpdate_IncludesAnim(t *testing.T) {
	app := newAnimTestApp(t)

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	app.broadcastTimerUpdate(17)

	readActionMatching(t, anim, protocol.ActionUpdateTimer)
}

// TestBroadcastQCMHint_IncludesAnim — #157's fallback-to-current-invalidated-
// hints branch (calcQcmTeamAward) reads gameState.qcmInvalidated, populated
// client-side by QCM_HINT (useWebSocket.js:444-450). Without this, #157's
// repli silently computes on an empty array (plan §1, "condition de
// justesse de #157").
func TestBroadcastQCMHint_IncludesAnim(t *testing.T) {
	app := newAnimTestApp(t)

	baseURL := startAnimAllowlistTestServer(t, app)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	app.broadcastQCMHint("RED", 2)

	_, raw := readActionMatching(t, anim, protocol.ActionQCMHint)
	var envelope struct {
		Msg protocol.QCMHintPayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("failed to unmarshal QCM_HINT: %v (raw: %s)", err, raw)
	}
	if envelope.Msg.Color != "RED" || envelope.Msg.Remaining != 2 {
		t.Errorf("QCM_HINT payload mismatch: got %+v", envelope.Msg)
	}
}

// ---------------------------------------------------------------------------
// Exclusion guards (plan §2.2/§4 D1-D2, R2/R3) — Anim must NOT start
// receiving what it was never meant to, just because T1/T2 wire it into the
// generic broadcast paths.
// ---------------------------------------------------------------------------

// TestBroadcastQuestions_StillAdminOnly guards #155 B4's admin-only policy —
// broadcastQuestions has its own dedicated type list, untouched by #162.
func TestBroadcastQuestions_StillAdminOnly(t *testing.T) {
	app := newAnimTestApp(t)
	app.config.Storage.QuestionsDir = t.TempDir() // empty on purpose — content is irrelevant here

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)
	tv := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tv)

	app.broadcastQuestions()

	readActionMatching(t, admin, protocol.ActionQuestions) // admin must still receive it

	animActions := collectActionsT(t, anim, 300*time.Millisecond)
	if containsAction(animActions, protocol.ActionQuestions) {
		t.Errorf("#162: anim must NOT receive QUESTIONS (#155 B4 admin-only policy) — got actions: %v", animActions)
	}
	tvActions := collectActionsT(t, tv, 300*time.Millisecond)
	if containsAction(tvActions, protocol.ActionQuestions) {
		t.Errorf("TV must NOT receive QUESTIONS — got actions: %v", tvActions)
	}
}

// TestBroadcastClientCounts_StillAdminOnly guards MAJEUR-1's admin-only
// policy for CLIENTS.
func TestBroadcastClientCounts_StillAdminOnly(t *testing.T) {
	app := newAnimTestApp(t)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	app.broadcastClientCounts()

	readActionMatching(t, admin, protocol.ActionClients)

	animActions := collectActionsT(t, anim, 300*time.Millisecond)
	if containsAction(animActions, protocol.ActionClients) {
		t.Errorf("#162: anim must NOT receive CLIENTS — got actions: %v", animActions)
	}
}

// TestBroadcastConfigUpdate_StillAdminAndTVOnly guards the established
// Admin+TV-only policy (#154 E1) — /anim displays no server configuration.
func TestBroadcastConfigUpdate_StillAdminAndTVOnly(t *testing.T) {
	app := newAnimTestApp(t)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	tv := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tv)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)
	vplayer := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vplayer)

	app.broadcastConfigUpdate()

	readActionMatching(t, admin, protocol.ActionConfigUpdate)
	readActionMatching(t, tv, protocol.ActionConfigUpdate)

	animActions := collectActionsT(t, anim, 300*time.Millisecond)
	if containsAction(animActions, protocol.ActionConfigUpdate) {
		t.Errorf("#162: anim must NOT receive CONFIG_UPDATE — got actions: %v", animActions)
	}
	vplayerActions := collectActionsT(t, vplayer, 300*time.Millisecond)
	if containsAction(vplayerActions, protocol.ActionConfigUpdate) {
		t.Errorf("vplayer must NOT receive CONFIG_UPDATE (unaffected by #162) — got actions: %v", vplayerActions)
	}
}

// TestHandlePong_StillExcludesAnim protects #127 T1.2's deliberate exclusion
// (plan D1): the per-PONG broadcastUpdateTo(Admin, Buzzer) rafale must never
// reach Anim, no matter how many participants PONG in. Mirrors the existing
// CA1 invariant for VPlayer (main_broadcast_127_test.go,
// TestBroadcast127_VPlayer_UpdateCount_ConstantAcrossN): once T1/T2 land,
// Anim is targeted by broadcastGameState exactly like VPlayer (D3 — READY
// entry into PREPARE + the READY transition itself), so its UPDATE count
// across a full PREPARE->N-PONGs->READY sequence must be exactly 2,
// regardless of N — if handlePong's per-PONG call ever regains Anim, this
// count inflates by N.
// ---------------------------------------------------------------------------
// Tests: v6.4.x (#167) — REGIE_MESSAGE diffusion (plan tâche B5, T5,
// contracts/websocket-actions.md §"Messagerie régie" → REGIE_MESSAGE).
//
// REGIE_MESSAGE is the FIRST outbound action shared by exactly admin+anim
// (see the contract's own note) — every other "Animateur" action
// (NEXT_QUESTION/CREDIT_POINTS/AWARDED_TEAMS) is anim-exclusive. tv and
// vplayer must never receive it (they don't even read the régie/anim
// channel); buzzer is architecturally unreachable by any wsHub broadcast
// (a distinct hub, app.buzzerHub — verified by code inspection, same
// reasoning as TestHandlePong_StillExcludesAnim's own comment on
// ClientTypeBuzzer, not exercised live here).
// ---------------------------------------------------------------------------

func TestBroadcastRegieMessage_ReachesAdminAndAnim_NeverTVOrVPlayer(t *testing.T) {
	app := newAnimTestApp(t)
	app.regieMessage = &protocol.RegieMessagePayload{
		Active: true,
		Text:   "Question 12 annulée — enchaîne sur la 13",
		SentAt: 1755511234567,
	}

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)
	tv := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tv)
	vplayer := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vplayer)

	app.broadcastRegieMessage()

	for _, conn := range []struct {
		name string
		c    *websocket.Conn
	}{{"admin", admin}, {"anim", anim}} {
		_, raw := readActionMatching(t, conn.c, protocol.ActionRegieMessage)
		var envelope struct {
			Msg protocol.RegieMessagePayload `json:"MSG"`
		}
		if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
			t.Fatalf("%s: failed to unmarshal REGIE_MESSAGE: %v (raw: %s)", conn.name, err, raw)
		}
		if !envelope.Msg.Active || envelope.Msg.Text != "Question 12 annulée — enchaîne sur la 13" {
			t.Errorf("%s: REGIE_MESSAGE payload mismatch: got %+v", conn.name, envelope.Msg)
		}
	}

	tvActions := collectActionsT(t, tv, 300*time.Millisecond)
	if containsAction(tvActions, protocol.ActionRegieMessage) {
		t.Errorf("#167: tv must NEVER receive REGIE_MESSAGE — got actions: %v", tvActions)
	}
	vplayerActions := collectActionsT(t, vplayer, 300*time.Millisecond)
	if containsAction(vplayerActions, protocol.ActionRegieMessage) {
		t.Errorf("#167: vplayer must NEVER receive REGIE_MESSAGE — got actions: %v", vplayerActions)
	}
}

// TestHandleWebMessage_RegieMessageSend_BroadcastsToAdminAndAnim exercises
// the same guarantee end-to-end through the real dispatch path (REGIE_
// MESSAGE_SEND sent by an admin), rather than calling
// app.broadcastRegieMessage() directly.
func TestHandleWebMessage_RegieMessageSend_BroadcastsToAdminAndAnim(t *testing.T) {
	app := newAnimTestApp(t)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, admin)
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, anim)

	// RegieMessagePayload is reused for the SEND request body — same "shared
	// by both directions" precedent as CreditPointsPayload (SET_CREDIT_POINTS
	// / CREDIT_POINTS): the server only reads .Text from an incoming SEND,
	// Active/SentAt/ClearedBy are irrelevant on this side and ignored.
	sendAction(t, app, admin, protocol.ActionRegieMessageSend, protocol.RegieMessagePayload{Text: "Pause technique 2 minutes"})

	for _, conn := range []*websocket.Conn{admin, anim} {
		_, raw := readActionMatching(t, conn, protocol.ActionRegieMessage)
		var envelope struct {
			Msg protocol.RegieMessagePayload `json:"MSG"`
		}
		if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
			t.Fatalf("failed to unmarshal REGIE_MESSAGE: %v (raw: %s)", err, raw)
		}
		if !envelope.Msg.Active || envelope.Msg.Text != "Pause technique 2 minutes" {
			t.Errorf("REGIE_MESSAGE payload mismatch after REGIE_MESSAGE_SEND: got %+v", envelope.Msg)
		}
	}
}

func TestHandlePong_StillExcludesAnim(t *testing.T) {
	for _, n := range []int{1, 10} {
		n := n
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			app := newBroadcast127TestApp(t)
			app.engine.SetPhase(game.PhaseStopped)

			baseURL := startAnimAllowlistTestServer(t, app)
			anim := dialWS(t, baseURL, "/ws/anim")
			learnClientID(t, app, anim)

			runPrepareToReadySequence(t, app, n)

			actions := collectActions(anim, 500*time.Millisecond)
			got := countActions(actions, protocol.ActionUpdate)
			if got != 2 {
				t.Errorf("#162/#127 D1: anim should receive exactly 2 UPDATE (PREPARE entry + READY transition) regardless of N — N=%d, got %d (actions=%v)", n, got, actions)
			}
		})
	}
}
