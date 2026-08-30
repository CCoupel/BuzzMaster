package main

// ---------------------------------------------------------------------------
// Régression — transition RÉELLE COUNTDOWN→STARTED pour RAFALE, jusqu'à la
// livraison effective sur le fil (milestone v8.0.0, #107).
//
// Contexte : deux cycles QUALIF consécutifs ont rapporté EXACTEMENT le même
// symptôme externe ("après le décompte de 3s, aucun compteur ne se lance,
// aucune question ne s'affiche"), pour DEUX causes racines distinctes :
//   - cycle 1 (2026-08-30) : CATEGORY vide sur une question de config
//     résiduelle d'avant la migration catégorie-unique.
//   - cycle 2 (2026-08-31) : RAFALE_DIFFICULTY jamais persisté par
//     handleUploadQuestion (bloc manquant), donc toujours 0 au chargement.
//
// Les deux fois, le mécanisme de mort était le même : startRafaleRoundUnsafe
// (appelé depuis actualStart(), lui-même déclenché par le VRAI timer de
// countdown 3s, PAS StartImmediate) tirait son filet de sécurité
// (drawRafaleQuestionUnsafe échoue sur un pool vide pour la catégorie/
// difficulté demandée → finishRafaleRoundStart appelle Stop() dans le même
// cycle) — silencieusement, sans qu'aucun log ne soit visible côté client.
//
// rafale_107_test.go documente explicitement (voir son en-tête) qu'il
// n'exerce PAS cette transition : il utilise StartImmediate pour éviter
// d'ajouter ~3s réelles à la suite, en jugeant (a posteriori, à tort) que le
// countdown générique de 3s n'était "pas le risque que ce fichier cherche à
// couvrir". C'est précisément le trou de couverture qui a laissé passer les
// deux récidives : l'interaction entre le countdown GÉNÉRIQUE et le filet de
// sécurité SPÉCIFIQUE à RAFALE dans actualStart() n'était testée nulle part.
//
// Ce fichier comble ce trou, une fois pour toutes, en tant que test PERMANENT
// (pas un test jetable de diagnostic) : vrai dispatch de START
// (handleWebMessage, pas un appel direct à Engine.Start), vrai countdown 3s
// (aucun raccourci), vrais callbacks (mirroring de la tranche RAFALE de
// setupCallbacks, main.go), et vraie connexion WebSocket TV pour vérifier que
// la question atteint effectivement le fil — pas seulement la mémoire du
// moteur.
// ---------------------------------------------------------------------------

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// newRafaleCountdownWireTestApp wires the RAFALE-relevant slice of main.go's
// setupCallbacks (main.go ~line 418) — not the full function, which also
// requires a.httpServer (nil in every test App, see testhelpers_test.go) and
// touches disk via broadcastQuestions(), irrelevant here. Same "mirror the
// relevant slice" convention already used by newVPlayerIntegrationApp
// (vplayer_broadcast_integration_test.go) and newBroadcast127TestApp
// (main_broadcast_127_test.go).
//
// The dispatch goroutine below (`for msg := range app.wsHub.Incoming`) is a
// faithful reproduction of main.go:432-436 — the SAME shape
// dispatch_panic_recovery_test.go already uses — so that a dispatched START
// truly goes through handleWebMessage's allow-list and switch, exactly as a
// real admin client's message would.
func newRafaleCountdownWireTestApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppWithHub(t)
	// broadcastGameState/broadcastCountdownUpdate/broadcastTimerUpdate all
	// call a.broadcast(..., viaTCP=true, ...) -> a.udpBcast.Broadcast(msg) —
	// an unstarted NewUDPBroadcaster() no-ops safely (conn == nil), same
	// technique as vplayer_broadcast_integration_test.go.
	app.udpBcast = server.NewUDPBroadcaster()

	app.engine.OnStateChange = func(phase game.GamePhase) {
		app.broadcastGameState(string(phase))
	}
	app.engine.OnCountdownTick = func(countdownTime int) {
		app.broadcastCountdownUpdate(countdownTime)
	}
	app.engine.OnTimerTick = func(currentTime int) {
		app.broadcastTimerUpdate(currentTime)
	}
	app.engine.OnRafaleAnswer = func(id, answer string) {
		app.broadcastRafaleAnswer(id, answer)
	}
	app.engine.OnRafaleQuestionTick = func(questionTime int) {
		app.broadcastRafaleTick(questionTime)
	}
	app.engine.OnRafaleTeamsChanged = func() {
		app.sendLEDSetRafaleTeams()
	}

	go func() {
		for msg := range app.wsHub.Incoming {
			app.handleWebMessage(msg)
		}
	}()

	return app
}

// readActionMatchingWithin drains messages on conn until one whose ACTION
// equals want arrives, or timeout elapses — a longer-deadline sibling of
// main_broadcast_127_test.go's readActionMatching (hardcoded 2s, too short
// here: a real 3s countdown must elapse before STARTED's UPDATE, and the
// first RAFALE_TICK needs up to 1 more real second after that).
func readActionMatchingWithin(t *testing.T, conn interface {
	SetReadDeadline(time.Time) error
	ReadMessage() (int, []byte, error)
}, want string, timeout time.Duration) json.RawMessage {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("expected action %s, got error: %v", want, err)
		}
		var envelope struct {
			Action string          `json:"ACTION"`
			Msg    json.RawMessage `json:"MSG"`
		}
		if json.Unmarshal(data, &envelope) == nil && envelope.Action == want {
			return envelope.Msg
		}
	}
	t.Fatalf("timed out waiting for action %s", want)
	return nil
}

// TestRafaleCountdownToStarted_RealDispatch_RealCountdown_DeliversQuestionOnTheWire
// is the permanent regression test for the exact transition both QUALIF
// cycles broke: a real admin START, through the real 3s countdown, must
// result in a real TV-connected client receiving (1) a GameState UPDATE with
// PHASE=STARTED and a non-empty RAFALE_CURRENT_QUESTION, and (2) at least one
// RAFALE_TICK — i.e. "the round counter starts, a question displays, and the
// question counter starts", verbatim the QUALIF report's wording.
func TestRafaleCountdownToStarted_RealDispatch_RealCountdown_DeliversQuestionOnTheWire(t *testing.T) {
	app := newRafaleCountdownWireTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{
		"red": {Name: "red", Color: []int{255, 0, 0}},
	})

	// Abundant reservoir, valid CATEGORY+DIFFICULTY — the normal case (NOT
	// the residual/empty edge case already covered by
	// rafale_modes_test.go's TestRafaleReady_*_NeverReachesReady tests).
	for i := 1; i <= 10; i++ {
		if _, err := app.engine.UpsertRafaleQuestion(game.RafaleQuestion{
			ID: fmt.Sprintf("r-%d", i), Question: fmt.Sprintf("Q%d", i), Answer: fmt.Sprintf("A%d", i),
			Category: game.CategoryHistory, Difficulty: 1,
		}); err != nil {
			t.Fatalf("seed reservoir: UpsertRafaleQuestion failed: %v", err)
		}
	}

	q := &game.Question{
		ID: "rq1", Question: "RAFALE round", Type: game.QuestionTypeRafale,
		Category: game.CategoryHistory,
		Points:   "10", Time: "120",
		TypedContent: game.TypedContent{
			RafaleDifficulty:   1,
			RafaleMode:         string(game.RafaleModeSolo),
			RafaleQuestionTime: 3, // contract default — RafaleQuestionTime=1 was tried and never
			// broadcasts a positive-value RAFALE_TICK at all: processRafaleQuestionTick
			// decrements 1->0 on its very first tick and goes straight to "expired"
			// (advance/redraw), skipping the `qt > 0` branch that fires OnRafaleQuestionTick.
			RafaleMaxQuestions: 100,
		},
	}
	// Setup via direct engine calls (Ready/ForceReady), same convention as
	// rafale_107_test.go/rafale_modes_test.go — this test's own value is the
	// START->countdown->STARTED transition below, not the READY path
	// (already covered by TestRafaleIntegration_ValidCategory_ReachesReadyViaRealDispatch).
	app.engine.Ready("rq1", q)
	app.engine.ForceReady()
	if state := app.engine.GetState(); state.Phase != game.PhaseReady {
		t.Fatalf("sanity: expected PhaseReady before START, got %s", state.Phase)
	}

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleSetTeams, protocol.RafaleSetTeamsPayload{Teams: []string{"red"}})
	if state := app.engine.GetState(); state.RafaleCurrentTeam != "red" {
		t.Fatalf("sanity: expected RAFALE_SET_TEAMS to select 'red', got %q", state.RafaleCurrentTeam)
	}

	baseURL := startEvictionTestServer(t, app) // routes /ws/tv -> ClientTypeTV
	tvConn := dialWS(t, baseURL, "/ws/tv")

	// The real dispatch path: a genuine ACTION START message through
	// handleWebMessage's allow-list and switch — NOT app.engine.Start(...)
	// directly, and definitely not StartImmediate. This is what actually
	// exercises Engine.Start's 3-second countdown goroutine and, at its end,
	// actualStart().
	//
	// Delay here is NOT just cosmetic: Engine.Start sets state.CurrentTime =
	// delay, which seeds the GLOBAL round timer (startTimer(), unrelated to
	// the 3s countdown) — a small value like 1 was first tried and made the
	// round's own timer expire and call Stop() a mere ~1s after STARTED,
	// racing the RAFALE_TICK assertion below. 30s gives ample real headroom.
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionStart, protocol.StartPayload{Delay: 30})
	// Stop the round's global timer/tickers at the end of the test rather
	// than let them run for the full 30s budget above in the background.
	defer app.engine.Stop()

	if state := app.engine.GetState(); state.Phase != game.PhaseCountdown {
		t.Fatalf("sanity: expected PhaseCountdown right after dispatching START, got %s", state.Phase)
	}

	// The engine emits SEVERAL "UPDATE" frames before STARTED: at least one
	// for entering COUNTDOWN itself (OnStateChange(PhaseCountdown) inside
	// Engine.Start), on top of the per-second UPDATE_TIMER countdown ticks
	// (broadcastCountdownUpdate — a different ACTION). So this loops over
	// "UPDATE" frames specifically until GAME.PHASE=="STARTED", rather than
	// taking the first one — budget generously above the real 3s countdown.
	type gameEnvelope struct {
		Game struct {
			Phase                 string             `json:"PHASE"`
			RafaleCurrentQuestion game.RafaleCurrent `json:"RAFALE_CURRENT_QUESTION"`
		} `json:"GAME"`
	}
	var startedState gameEnvelope
	deadline := time.Now().Add(6 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for an UPDATE with GAME.PHASE=STARTED (last seen phase=%q)", startedState.Game.Phase)
		}
		msg := readActionMatchingWithin(t, tvConn, protocol.ActionUpdate, time.Until(deadline))
		if err := json.Unmarshal(msg, &startedState); err != nil {
			t.Fatalf("failed to unmarshal UPDATE: %v (raw: %s)", err, msg)
		}
		if startedState.Game.Phase == string(game.PhaseStarted) {
			break
		}
	}
	if startedState.Game.RafaleCurrentQuestion.ID == "" {
		t.Fatal("RAFALE_CURRENT_QUESTION.ID is empty in the STARTED UPDATE — the exact QUALIF symptom: 'no question displays' after the countdown")
	}
	if startedState.Game.RafaleCurrentQuestion.Category != string(game.CategoryHistory) {
		t.Errorf("expected RAFALE_CURRENT_QUESTION.CATEGORY=HISTORY, got %q", startedState.Game.RafaleCurrentQuestion.Category)
	}
	if startedState.Game.RafaleCurrentQuestion.Difficulty != 1 {
		t.Errorf("expected RAFALE_CURRENT_QUESTION.DIFFICULTY=1, got %d", startedState.Game.RafaleCurrentQuestion.Difficulty)
	}

	// The question TIMER actually starts: at least one RAFALE_TICK reaches
	// the wire (StartRafaleQuestionTimer's first tick fires ~1s after
	// actualStart, RafaleQuestionTime=1 above keeps this fast).
	tickMsg := readActionMatchingWithin(t, tvConn, protocol.ActionRafaleTick, 3*time.Second)
	var tick protocol.RafaleTickPayload
	if err := json.Unmarshal(tickMsg, &tick); err != nil {
		t.Fatalf("failed to unmarshal RAFALE_TICK: %v (raw: %s)", err, tickMsg)
	}
	if tick.QuestionTime <= 0 {
		t.Errorf("expected a positive RAFALE_TICK QUESTION_TIME, got %d — 'the question counter never starts' symptom", tick.QuestionTime)
	}

	// Also confirm, at the Engine level, that the round genuinely survived
	// into STARTED and was NOT killed by the Stop() safety net in the same
	// tick (both QUALIF cycles' actual failure mode) — belt-and-braces on
	// top of the wire-level assertions above.
	finalState := app.engine.GetState()
	if finalState.Phase != game.PhaseStarted {
		t.Errorf("expected the engine to still be in STARTED (not killed by the roundEnded->Stop() safety net), got %s", finalState.Phase)
	}
	if finalState.RafaleExhausted {
		t.Error("RAFALE_EXHAUSTED is true — the pool-empty safety net fired, meaning the round died in the same tick it started (the exact mechanism behind both QUALIF cycles)")
	}
}
