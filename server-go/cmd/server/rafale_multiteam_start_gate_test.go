package main

// ---------------------------------------------------------------------------
// Régression — retour QUALIF 8.0.0.14 (#199) : "je peux encore lancer un
// START alors que je n'ai pas défini d'équipe pour la manche RAFALE",
// malgré la garde participantsConform (SHA 393c6dc7, reviewée, QA validée).
//
// Investigation (dev-backend) — voir le rapport complet pour le détail :
//   - AUCUN chemin backend ne contourne la garde. Les 3 sites qui font
//     `Phase = PhaseReady` dans engine.go (reevaluatePrepareReadyUnsafe,
//     TransitionToReady, ForceReady) sont tous gardés, directement ou par
//     leur appelant (handlePong, main.go:1608 — le seul appelant de
//     TransitionToReady). Confirmé ici via dispatchAs(ClientTypeAnim, ...),
//     PAS ClientTypeAdmin comme le reste de la suite RAFALE existante — la
//     demande explicite du CDP était de reproduire depuis /anim en
//     particulier, une action que rafale_107_test.go et
//     rafale_modes_test.go ne couvrent jamais (ils utilisent uniquement
//     ForceReady/StartImmediate/l'appel direct Engine, jamais un dispatch
//     WS réel en tant qu'ANIM).
//   - EFFET DE BORD réel trouvé pendant l'audit (corrigé dans la même
//     commit, main.go handleStart) : broadcastStart() (et son
//     sendLEDSetAllBuzzers()) se déclenchait INCONDITIONNELLEMENT après
//     Engine.Start(), même quand celui-ci refusait silencieusement — pour
//     TOUT type de question, pas seulement RAFALE. Pas la cause du
//     symptôme rapporté (le payload transportait quand même la vraie PHASE
//     inchangée), mais un comportement incorrect indépendant, resserré ici.
//   - Hypothèse retenue pour le symptôme lui-même : RAFALE_MODE périmé
//     (question de configuration sauvegardée avant le fix http.go
//     8.0.0.11, jamais re-sauvegardée depuis) — défaut "vide ⇒ SOLO" du
//     même type que CATEGORY (cycle 1) et RAFALE_DIFFICULTY (cycle 2),
//     mais cette fois un comportement CORRECT du code face à une donnée
//     périmée, pas un bug de la garde. Voir
//     internal/server/rafale_multiteam_gate_test.go pour la preuve dédiée
//     (upload HTTP réel, mode vide ⇒ SOLO légitime vs. mode réellement
//     persisté ⇒ garde appliquée).
// ---------------------------------------------------------------------------

import (
	"encoding/json"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// TestRafaleMultiTeamGate_AnimDispatch_StartRefused_StaysInPrepare reproduces
// the EXACT scenario the CDP asked for: a real ACTION_START dispatched as
// ClientTypeAnim (not Admin), through the real handleWebMessage/allow-list,
// against a multi-mode RAFALE round with no team selected.
func TestRafaleMultiTeamGate_AnimDispatch_StartRefused_StaysInPrepare(t *testing.T) {
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster() // broadcastStart's a.broadcast(viaTCP=true) needs a non-nil (unstarted is fine) broadcaster
	app.engine.SetTeams(map[string]*game.Team{
		"red": {Name: "red", Color: []int{255, 0, 0}}, "blue": {Name: "blue", Color: []int{0, 0, 255}},
	})
	for i := 1; i <= 5; i++ {
		if _, err := app.engine.UpsertRafaleQuestion(game.RafaleQuestion{
			ID: "r-" + string(rune('a'+i)), Question: "Q", Answer: "A",
			Category: game.CategoryHistory, Difficulty: 1,
		}); err != nil {
			t.Fatalf("seed reservoir: %v", err)
		}
	}
	q := &game.Question{
		ID: "rq1", Question: "RAFALE round", Type: game.QuestionTypeRafale,
		Category: game.CategoryHistory, Points: "10", Time: "120",
		TypedContent: game.TypedContent{
			RafaleDifficulty: 1, RafaleMode: string(game.RafaleModeChacunSonTour),
			RafaleQuestionTime: 3, RafaleMaxQuestions: 100,
		},
	}
	app.engine.Ready("rq1", q)
	app.engine.ForceReady() // no RAFALE_SET_TEAMS — must stay in PREPARE

	if state := app.engine.GetState(); state.Phase != game.PhasePrepare {
		t.Fatalf("sanity: expected PhasePrepare after ForceReady with no team, got %s", state.Phase)
	}

	// The real repro: START dispatched as ANIM, exactly as AnimConductPanel's
	// LANCER button would (useWebSocket.js's startGame -> ACTION_START),
	// through the real allow-list (ActionStart: {Admin, Anim}) and switch.
	dispatchAs(t, app, server.ClientTypeAnim, protocol.ActionStart, protocol.StartPayload{Delay: 30})

	state := app.engine.GetState()
	if state.Phase == game.PhaseCountdown || state.Phase == game.PhaseStarted {
		t.Fatalf("BUG: ANIM-dispatched START succeeded (phase=%s) on a multi-mode RAFALE round with no team selected", state.Phase)
	}
	if state.Phase != game.PhasePrepare {
		t.Errorf("expected the engine to stay in PREPARE, got %s", state.Phase)
	}
}

// TestHandleStart_RefusedStart_DoesNotBroadcastStart is the regression test
// for the independent side-effect bug found during this investigation:
// handleStart used to call broadcastStart() unconditionally, even when
// Engine.Start() silently refused — sending every connected client a real
// ACTION:"START" WS frame although nothing actually started. Generic — not
// RAFALE-specific in the fix itself — reproduced here with the RAFALE
// scenario since that's what surfaced it. Uses a real WS connection
// (httptest.Server + gorilla/websocket, same harness as
// rafale_countdown_wire_test.go) to observe the wire directly, not just
// engine-internal state — a callback-based assertion wouldn't actually
// exercise broadcastStart() itself.
func TestHandleStart_RefusedStart_DoesNotBroadcastStart(t *testing.T) {
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.SetTeams(map[string]*game.Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
	for i := 1; i <= 5; i++ {
		if _, err := app.engine.UpsertRafaleQuestion(game.RafaleQuestion{
			ID: "r-" + string(rune('a'+i)), Question: "Q", Answer: "A",
			Category: game.CategoryHistory, Difficulty: 1,
		}); err != nil {
			t.Fatalf("seed reservoir: %v", err)
		}
	}
	q := &game.Question{
		ID: "rq1", Question: "RAFALE round", Type: game.QuestionTypeRafale,
		Category: game.CategoryHistory, Points: "10", Time: "120",
		TypedContent: game.TypedContent{
			RafaleDifficulty: 1, RafaleMode: string(game.RafaleModeChacunSonTour),
			RafaleQuestionTime: 3, RafaleMaxQuestions: 100,
		},
	}
	app.engine.Ready("rq1", q)
	app.engine.ForceReady() // stuck in PREPARE, no team

	baseURL := startEvictionTestServer(t, app) // routes /ws/tv -> ClientTypeTV
	tvConn := dialWS(t, baseURL, "/ws/tv")

	dispatchAs(t, app, server.ClientTypeAnim, protocol.ActionStart, protocol.StartPayload{Delay: 30})

	// No ACTION:"START" (nor anything else) should reach the wire — give it
	// a short, generous window rather than asserting on an instant read.
	tvConn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, data, err := tvConn.ReadMessage()
	if err == nil {
		var envelope struct {
			Action string `json:"ACTION"`
		}
		json.Unmarshal(data, &envelope)
		t.Fatalf("expected no WS message after a refused START, got ACTION=%q (raw: %s)", envelope.Action, data)
	}

	if state := app.engine.GetState(); state.Phase != game.PhasePrepare {
		t.Fatalf("sanity: expected PhasePrepare, got %s", state.Phase)
	}
}

// TestHandleStart_AcceptedStart_StillBroadcasts is the positive control for
// the test above: a successful Start() (READY reached, conform) must still
// broadcast ACTION:"START" as before — proving the fix narrows the
// broadcast to actual successes, it doesn't silently suppress it.
func TestHandleStart_AcceptedStart_StillBroadcasts(t *testing.T) {
	app := newTestAppWithHub(t)
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.SetTeams(map[string]*game.Team{"red": {Name: "red"}})
	for i := 1; i <= 5; i++ {
		if _, err := app.engine.UpsertRafaleQuestion(game.RafaleQuestion{
			ID: "r-" + string(rune('a'+i)), Question: "Q", Answer: "A",
			Category: game.CategoryHistory, Difficulty: 1,
		}); err != nil {
			t.Fatalf("seed reservoir: %v", err)
		}
	}
	q := &game.Question{
		ID: "rq1", Question: "RAFALE round", Type: game.QuestionTypeRafale,
		Category: game.CategoryHistory, Points: "10", Time: "120",
		TypedContent: game.TypedContent{
			RafaleDifficulty: 1, RafaleMode: string(game.RafaleModeSolo), // SOLO — no team required
			RafaleQuestionTime: 3, RafaleMaxQuestions: 100,
		},
	}
	app.engine.Ready("rq1", q)
	app.engine.ForceReady()
	if state := app.engine.GetState(); state.Phase != game.PhaseReady {
		t.Fatalf("sanity: expected PhaseReady (SOLO, no team required), got %s", state.Phase)
	}

	baseURL := startEvictionTestServer(t, app)
	tvConn := dialWS(t, baseURL, "/ws/tv")

	dispatchAs(t, app, server.ClientTypeAnim, protocol.ActionStart, protocol.StartPayload{Delay: 30})
	defer app.engine.Stop()

	action, _ := readAction(t, tvConn)
	if action != protocol.ActionStart {
		t.Errorf("expected ACTION=%q on a successful START, got %q", protocol.ActionStart, action)
	}
	if state := app.engine.GetState(); state.Phase != game.PhaseCountdown {
		t.Errorf("expected PhaseCountdown after an accepted START, got %s", state.Phase)
	}
}
