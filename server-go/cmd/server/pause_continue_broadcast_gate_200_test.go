// Tests for #200 cycle 4 — code-review 20260831-112306 (rapport
// _work/reports/code-review-20260831-112306.md), réserve MAJEURE sur le fix
// cycle 3 (SHA 64b23dff, Pause()/Continue() ont gagné une garde de phase).
//
// handleStart (main.go) avait déjà été corrigé (#199) pour ne broadcastStart()
// que si Engine.Start() a réellement transitionné (GetPhase()==PhaseCountdown)
// — précisément parce qu'un Start() refusé qui diffuse quand même
// ACTION:"START" est "undefined behavior for any client-side logic keyed on
// the action name rather than the payload" (voir son propre commentaire).
// Les handlers ActionPause/ActionContinue n'avaient PAS reçu le même
// traitement au cycle 3 : ils diffusaient inconditionnellement, même quand
// la nouvelle garde (64b23dff) refusait la transition. Le frontend
// (useWebSocket.js) code en dur phase:'STARTED'/'PAUSED' sur la seule
// étiquette d'action reçue — donc un CONTINUE refusé (Phase reste PREPARE
// côté serveur) pouvait quand même faire basculer visuellement TOUS les
// clients connectés vers "STARTED", reproduisant le symptôme initialement
// rapporté au niveau affichage uniquement.
//
// Mirroring de rafale_multiteam_start_gate_test.go's
// TestHandleStart_RefusedStart_DoesNotBroadcastStart /
// TestHandleStart_AcceptedStart_StillBroadcasts, même harnais (WS réel via
// httptest.Server + gorilla/websocket — une assertion sur l'état moteur
// seul ne exercerait pas broadcastPauseAll()/broadcastContinue() eux-mêmes).
//
// Run: go test ./cmd/server/... -run TestHandleContinue\|TestHandlePause -v
package main

import (
	"encoding/json"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// TestHandleContinue_RefusedContinue_DoesNotBroadcast reproduces the exact
// scenario from the review: a MEMORY question with ZERO teams ever selected
// (Phase stays PREPARE, Continue()'s cycle 3 guard refuses) — the handler
// must NOT broadcast ACTION:"CONTINUE" to any connected client.
func TestHandleContinue_RefusedContinue_DoesNotBroadcast(t *testing.T) {
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.SetTeams(map[string]*game.Team{"red": {Name: "red"}})

	q := &game.Question{
		ID: "mq1", Question: "Memory question", Type: game.QuestionTypeMemory,
		Points: "10", Time: "60",
		TypedContent: game.TypedContent{MemoryMode: string(game.MemoryModeSolo)},
	}
	app.engine.Ready("mq1", q)
	if state := app.engine.GetState(); state.Phase != game.PhasePrepare {
		t.Fatalf("sanity: expected PhasePrepare, got %s", state.Phase)
	}

	baseURL := startEvictionTestServer(t, app)
	tvConn := dialWS(t, baseURL, "/ws/tv")

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionContinue, struct{}{})

	// No ACTION:"CONTINUE" (nor anything else) should reach the wire.
	tvConn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, data, err := tvConn.ReadMessage()
	if err == nil {
		var envelope struct {
			Action string `json:"ACTION"`
		}
		json.Unmarshal(data, &envelope)
		t.Fatalf("expected no WS message after a refused CONTINUE, got ACTION=%q (raw: %s)", envelope.Action, data)
	}

	if state := app.engine.GetState(); state.Phase != game.PhasePrepare {
		t.Fatalf("sanity: expected engine to stay in PREPARE, got %s", state.Phase)
	}
}

// TestHandleContinue_AcceptedContinue_StillBroadcasts is the positive
// control: a legitimate Start→Pause→Continue flow must still broadcast
// ACTION:"CONTINUE" as before — proving the fix narrows the broadcast to
// actual successes, it doesn't silently suppress it.
func TestHandleContinue_AcceptedContinue_StillBroadcasts(t *testing.T) {
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.StartImmediate(30)
	app.engine.Pause()
	if state := app.engine.GetState(); state.Phase != game.PhasePaused {
		t.Fatalf("sanity: expected PhasePaused, got %s", state.Phase)
	}
	defer app.engine.Stop()

	baseURL := startEvictionTestServer(t, app)
	tvConn := dialWS(t, baseURL, "/ws/tv")

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionContinue, struct{}{})

	action, _ := readAction(t, tvConn)
	if action != protocol.ActionContinue {
		t.Errorf("expected ACTION=%q on a successful CONTINUE, got %q", protocol.ActionContinue, action)
	}
	if state := app.engine.GetState(); state.Phase != game.PhaseStarted {
		t.Errorf("expected PhaseStarted after an accepted CONTINUE, got %s", state.Phase)
	}
}

// TestHandlePause_RefusedPause_DoesNotBroadcast mirrors the CONTINUE test
// above for ActionPause: Pause()'s own cycle 3 guard (64b23dff) only accepts
// PhaseStarted — dispatched from the engine's default PhaseStopped, it must
// refuse, and the handler must not broadcast.
func TestHandlePause_RefusedPause_DoesNotBroadcast(t *testing.T) {
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster()
	if state := app.engine.GetState(); state.Phase != game.PhaseStopped {
		t.Fatalf("sanity: expected PhaseStopped (fresh engine), got %s", state.Phase)
	}

	baseURL := startEvictionTestServer(t, app)
	tvConn := dialWS(t, baseURL, "/ws/tv")

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionPause, struct{}{})

	tvConn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, data, err := tvConn.ReadMessage()
	if err == nil {
		var envelope struct {
			Action string `json:"ACTION"`
		}
		json.Unmarshal(data, &envelope)
		t.Fatalf("expected no WS message after a refused PAUSE, got ACTION=%q (raw: %s)", envelope.Action, data)
	}

	if state := app.engine.GetState(); state.Phase != game.PhaseStopped {
		t.Fatalf("sanity: expected engine to stay in STOPPED, got %s", state.Phase)
	}
}

// TestHandlePause_AcceptedPause_StillBroadcasts is the positive control for
// ActionPause: a genuinely running game (STARTED) must still broadcast
// ACTION:"PAUSE" as before.
func TestHandlePause_AcceptedPause_StillBroadcasts(t *testing.T) {
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.StartImmediate(30)
	if state := app.engine.GetState(); state.Phase != game.PhaseStarted {
		t.Fatalf("sanity: expected PhaseStarted, got %s", state.Phase)
	}
	defer app.engine.Stop()

	baseURL := startEvictionTestServer(t, app)
	tvConn := dialWS(t, baseURL, "/ws/tv")

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionPause, struct{}{})

	action, _ := readAction(t, tvConn)
	if action != protocol.ActionPause {
		t.Errorf("expected ACTION=%q on a successful PAUSE, got %q", protocol.ActionPause, action)
	}
	if state := app.engine.GetState(); state.Phase != game.PhasePaused {
		t.Errorf("expected PhasePaused after an accepted PAUSE, got %s", state.Phase)
	}
}
