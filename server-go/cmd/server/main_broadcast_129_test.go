package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// #129 — Ciblage des broadcasts par événement (connexion/déconnexion,
// ARDOISE_INPUT, rafale per-PONG résiduelle vers la TV)
//
// Plan : _work/reports/planner-20260803-170653.md (T1.1-T1.7)
// Contrat : contracts/vplayer-payload-filter.md §5
// Handoff : _work/handoff/task-dev-backend-20260803-173502.md
//
// Étend le harnais de main_broadcast_127_test.go (newBroadcast127TestApp,
// startEvictionTestServer, dialWS, learnClientID, collectActions,
// countActions) plutôt que de le dupliquer, comme demandé en T1.7.
// ---------------------------------------------------------------------------

// setupVirtualPlayer creates and assigns a virtual player bumper directly via
// the engine (bypassing handlePlayerConnect's ENROLL-phase gating, which
// Case 1 of ReconnectOrCreateVirtualPlayer doesn't need for a subsequent
// reconnection by ID — see engine.go:2774-2789), ready to be disconnected/
// reconnected by these tests.
func setupVirtualPlayer(t *testing.T, app *App, name, team string) string {
	t.Helper()
	id, _, err := app.engine.CreateVirtualPlayer(name)
	if err != nil {
		t.Fatalf("CreateVirtualPlayer(%s) failed: %v", name, err)
	}
	if err := app.engine.AssignVirtualPlayer(id, team, game.AnswerColorNone); err != nil {
		t.Fatalf("AssignVirtualPlayer(%s) failed: %v", id, err)
	}
	return id
}

func ardoiseInputMsg(t *testing.T, bumperID, text string) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionArdoiseInput, protocol.ArdoiseInputPayload{ID: bumperID, Text: text})
	if err != nil {
		t.Fatalf("failed to build ARDOISE_INPUT message: %v", err)
	}
	return msg
}

// ---------------------------------------------------------------------------
// CA1 / CA3 — 8 déconnexions puis 8 reconnexions : un VJoueur resté connecté
// reçoit 0 UPDATE de ces 16 événements ; l'admin en reçoit 16.
// ---------------------------------------------------------------------------

func TestBroadcast129_ObserverVPlayer_ReceivesZeroUpdatesFromPeerConnectDisconnect(t *testing.T) {
	const nPeers = 8
	app := newBroadcast127TestApp(t)
	app.engine.SetPhase(game.PhaseEnroll) // CreateVirtualPlayer requires PhaseEnroll
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}, "TeamB": {Name: "TeamB"}})

	baseURL := startEvictionTestServer(t, app)
	observerConn := dialWS(t, baseURL, "/ws/player")
	observerClientID := learnClientID(t, app, observerConn)
	observerID := setupVirtualPlayer(t, app, "Observer", "TeamA")
	app.wsHub.SetClientPlayerID(observerClientID, observerID)

	adminConn := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, adminConn)

	peerIDs := make([]string, 0, nPeers)
	for i := 0; i < nPeers; i++ {
		team := "TeamA"
		if i%2 == 1 {
			team = "TeamB"
		}
		peerIDs = append(peerIDs, setupVirtualPlayer(t, app, fmt.Sprintf("Peer%d", i), team))
	}

	// Realistic scenario per the plan: this class of event happens "at any
	// moment of the game", not just during enrollment.
	app.engine.SetPhase(game.PhaseStarted)

	for _, id := range peerIDs {
		app.onPlayerDisconnected(id)
	}
	for _, id := range peerIDs {
		// No real WS client for these peers — SetClientPlayerID/SendToClient/
		// SendRawToPlayerID are all safe, tolerant no-ops for an unregistered
		// clientID (same style as the rest of this codebase); we only care
		// what the OBSERVER and admin receive from these 16 events.
		app.handlePlayerConnect(fmt.Sprintf("peer-client-%s", id), playerConnectMsg(t, "irrelevant", id))
	}

	observerActions := collectActions(observerConn, 500*time.Millisecond)
	observerUpdates := countActions(observerActions, protocol.ActionUpdate)
	t.Logf("observer received %d UPDATE from %d disconnects + %d reconnects (actions=%v)", observerUpdates, nPeers, nPeers, observerActions)
	if observerUpdates != 0 {
		t.Errorf("#129 CA1: observer VJoueur should receive 0 UPDATE from %d peer disconnects + %d peer reconnects, got %d", nPeers, nPeers, observerUpdates)
	}

	adminActions := collectActions(adminConn, 500*time.Millisecond)
	adminUpdates := countActions(adminActions, protocol.ActionUpdate)
	t.Logf("admin received %d UPDATE (actions=%v)", adminUpdates, adminActions)
	if adminUpdates != 2*nPeers {
		t.Errorf("#129 CA3: admin should receive exactly %d UPDATE (1 per disconnect + 1 per reconnect), got %d", 2*nPeers, adminUpdates)
	}
}

// ---------------------------------------------------------------------------
// CA2 — le VJoueur qui se reconnecte reçoit exactement un UPDATE ciblé
// contenant son propre bumper avec CONNECTED=true.
// ---------------------------------------------------------------------------

func TestBroadcast129_ReconnectingPlayer_ReceivesExactlyOneTargetedEchoWithConnectedTrue(t *testing.T) {
	app := newBroadcast127TestApp(t)
	app.engine.SetPhase(game.PhaseEnroll) // CreateVirtualPlayer requires PhaseEnroll
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})

	id := setupVirtualPlayer(t, app, "Alice", "TeamA")
	app.engine.SetPhase(game.PhaseStarted) // realistic: reconnection at any moment of the game
	// Simulate the disconnect (no real socket ever existed for `id` before this).
	app.engine.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/player")
	clientID := learnClientID(t, app, conn)

	app.handlePlayerConnect(clientID, playerConnectMsg(t, "Alice", id))

	actions := collectActions(conn, 500*time.Millisecond)
	got := countActions(actions, protocol.ActionUpdate)
	t.Logf("reconnecting player received %d UPDATE (actions=%v)", got, actions)
	if got != 1 {
		t.Fatalf("#129 CA2: reconnecting VJoueur should receive exactly 1 targeted UPDATE echo, got %d (actions=%v)", got, actions)
	}
}

// TestBroadcast129_ReconnectingPlayer_EchoContainsOwnBumperConnectedTrue
// re-drives the same reconnection but inspects the echo's content directly
// (via readActionMatching, cmd/server/main_broadcast_127_test.go) rather
// than just counting — the other half of CA2.
func TestBroadcast129_ReconnectingPlayer_EchoContainsOwnBumperConnectedTrue(t *testing.T) {
	app := newBroadcast127TestApp(t)
	app.engine.SetPhase(game.PhaseEnroll) // CreateVirtualPlayer requires PhaseEnroll
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})

	id := setupVirtualPlayer(t, app, "Alice", "TeamA")
	app.engine.SetPhase(game.PhaseStarted)
	app.engine.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/player")
	clientID := learnClientID(t, app, conn)

	app.handlePlayerConnect(clientID, playerConnectMsg(t, "Alice", id))

	action, raw := readActionMatching(t, conn, protocol.ActionUpdate)
	if action != protocol.ActionUpdate {
		t.Fatalf("expected an UPDATE echo, got %q", action)
	}
	var envelope struct {
		Msg struct {
			Bumpers map[string]struct {
				Connected bool `json:"CONNECTED"`
			} `json:"bumpers"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("failed to unmarshal echo: %v (raw: %s)", err, raw)
	}
	own, present := envelope.Msg.Bumpers[id]
	if !present {
		t.Fatalf("expected the echo's bumpers map to contain the reconnecting player's own bumper %q, got keys %v", id, envelope.Msg.Bumpers)
	}
	if !own.Connected {
		t.Errorf("#129 CA2: expected the echoed bumper to have CONNECTED=true, got false")
	}
}

// ---------------------------------------------------------------------------
// CA4 — question ARDOISE, plusieurs équipes tapant : un VJoueur (et la TV)
// reçoivent 0 UPDATE issu d'ARDOISE_INPUT ; l'admin les reçoit tous.
// ---------------------------------------------------------------------------

func TestBroadcast129_ArdoiseInput_ZeroUpdatesToVPlayerAndTV(t *testing.T) {
	const nTeams = 8
	const inputsPerTeam = 3 // 24 total ARDOISE_INPUT, well above the 20 in CA4's phrasing
	app := newBroadcast127TestApp(t)
	app.engine.SetPhase(game.PhaseEnroll) // CreateVirtualPlayer requires PhaseEnroll

	teams := map[string]*game.Team{}
	teamNames := make([]string, 0, nTeams)
	for i := 0; i < nTeams; i++ {
		name := fmt.Sprintf("Team%d", i)
		teams[name] = &game.Team{Name: name}
		teamNames = append(teamNames, name)
	}
	app.engine.SetTeams(teams)

	bumperIDs := make([]string, 0, nTeams)
	for i, name := range teamNames {
		bumperIDs = append(bumperIDs, setupVirtualPlayer(t, app, fmt.Sprintf("Player%d", i), name))
	}

	// ARDOISE_INPUT is only accepted during STARTED with an ARDOISE question
	// (engine.SetArdoiseAnswer's phase/type guard, engine.go:2654-2662).
	// Ready() only accepts STOPPED/REVEALED/PREPARE/READY/NEW_GAME as its
	// starting phase (engine.go:513-515) — switch away from PhaseEnroll
	// first, or it silently no-ops and Question stays nil.
	app.engine.SetPhase(game.PhaseStopped)
	app.engine.Ready("", &game.Question{ID: "q-ardoise", Type: game.QuestionTypeArdoise})
	app.engine.SetPhase(game.PhaseStarted)

	baseURL := startEvictionTestServer(t, app)
	vpConn := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vpConn)
	tvConn := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tvConn)
	adminConn := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, adminConn)

	total := 0
	for _, id := range bumperIDs {
		for i := 0; i < inputsPerTeam; i++ {
			app.handleArdoiseInput(id, ardoiseInputMsg(t, id, fmt.Sprintf("réponse %d", i)))
			total++
		}
	}

	vpActions := collectActions(vpConn, 500*time.Millisecond)
	if got := countActions(vpActions, protocol.ActionUpdate); got != 0 {
		t.Errorf("#129 CA4: VJoueur should receive 0 UPDATE from %d ARDOISE_INPUT, got %d (actions=%v)", total, got, vpActions)
	}

	tvActions := collectActions(tvConn, 500*time.Millisecond)
	if got := countActions(tvActions, protocol.ActionUpdate); got != 0 {
		t.Errorf("#129 CA4: TV should receive 0 UPDATE from %d ARDOISE_INPUT, got %d (actions=%v)", total, got, tvActions)
	}

	// #129 T2.2 (Phase 2): all `total` ARDOISE_INPUT calls happen essentially
	// synchronously here, well within one ardoiseCoalesceWindow (150ms) — the
	// coalescer collapses them into a single admin UPDATE. This is the
	// intended Phase 2 behavior (CA5's "retard ajouté ≤150ms"), not a
	// regression of the "admin sees answers build up live" guarantee: a real
	// admin, typing over real seconds rather than a test's tight loop, still
	// sees frequent updates — just capped in the pathological worst case.
	adminActions := collectActions(adminConn, 500*time.Millisecond)
	if got := countActions(adminActions, protocol.ActionUpdate); got != 1 {
		t.Errorf("#129 CA5 (admin, coalesced): admin should receive exactly 1 UPDATE for %d ARDOISE_INPUT arriving within one %s window, got %d (actions=%v)", total, ardoiseCoalesceWindow, got, adminActions)
	}
}

// ---------------------------------------------------------------------------
// CA8 — ApplyVPlayerBroadcastConnEvents n'est jamais appelée sur le chemin
// d'écho ciblé (broadcastUpdateToPlayer).
// ---------------------------------------------------------------------------

func TestBroadcast129_BroadcastUpdateToPlayer_DoesNotEvaluateVPlayerConnEvents(t *testing.T) {
	app := newBroadcast127TestApp(t)
	app.engine.SetPhase(game.PhaseEnroll) // CreateVirtualPlayer requires PhaseEnroll
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})

	// Bob: disconnected, genuinely eligible for MessageLost — #129 removed
	// the disconnect-announcing grace pass (transitionConnUnsafe no longer
	// arms skipNextMessageLost on ConnEventDisconnect), so the very NEXT
	// evaluation of ApplyVPlayerBroadcastConnEvents, whatever triggers it,
	// would immediately turn Bob red. That's exactly what this test must
	// prove does NOT happen from broadcastUpdateToPlayer below.
	bobID := setupVirtualPlayer(t, app, "Bob", "TeamA")
	// Alice: the actual recipient of the targeted echo below.
	aliceID := setupVirtualPlayer(t, app, "Alice", "TeamA")
	app.engine.SetPhase(game.PhaseStarted)

	app.engine.UpdateBumper(bobID, map[string]interface{}{"CONNECTED": false})
	if got := app.engine.GetBumper(bobID).ConnState; got != "orange" {
		t.Fatalf("setup failed: expected Bob orange after disconnect, got %q", got)
	}

	app.broadcastUpdateToPlayer(aliceID)

	if got := app.engine.GetBumper(bobID).ConnState; got != "orange" {
		t.Errorf("#129 CA8 regression: broadcastUpdateToPlayer (targeted at a DIFFERENT player) evaluated VPlayer conn events — Bob's ConnState=%q, expected still 'orange' (would be 'red' if ApplyVPlayerBroadcastConnEvents ran)", got)
	}
}

// ---------------------------------------------------------------------------
// CA9 — sendStateToClient() continue d'envoyer le payload complet non réduit,
// quel que soit le type de client / la phase.
// ---------------------------------------------------------------------------

func TestBroadcast129_SendStateToClient_StillCompletePayload(t *testing.T) {
	app := newBroadcast127TestApp(t)
	app.engine.SetPhase(game.PhaseEnroll) // CreateVirtualPlayer requires PhaseEnroll
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})

	setupVirtualPlayer(t, app, "Alice", "TeamA")
	setupVirtualPlayer(t, app, "Bob", "TeamA")
	app.engine.SetPhase(game.PhaseReady) // a reduced-eligible phase, to make this a real test

	// sendStateToClient reaches into a.httpServer (question storage info,
	// custom default image check) — nil in the minimal test app otherwise.
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))

	baseURL := startEvictionTestServer(t, app)
	conn := dialWS(t, baseURL, "/ws/player")
	clientID := learnClientID(t, app, conn)

	app.sendStateToClient(clientID, server.ClientTypeVPlayer)

	action, raw := readActionMatching(t, conn, protocol.ActionUpdate)
	if action != protocol.ActionUpdate {
		t.Fatalf("expected UPDATE, got %q", action)
	}
	var envelope struct {
		Msg struct {
			Bumpers map[string]interface{} `json:"bumpers"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("failed to unmarshal: %v (raw: %s)", err, raw)
	}
	if len(envelope.Msg.Bumpers) != 2 {
		t.Errorf("#129 CA9: sendStateToClient must send the COMPLETE bumpers map (2 entries) even in phase READY, got %d entries: %v", len(envelope.Msg.Bumpers), envelope.Msg.Bumpers)
	}
}

// ---------------------------------------------------------------------------
// CA12 — Séquence PREPARE -> N PONG -> READY : la TV reçoit exactement 2
// UPDATE, quel que soit N (contre N+2 avant #129).
// ---------------------------------------------------------------------------

func TestBroadcast129_TV_UpdateCount_ConstantAcrossN(t *testing.T) {
	for _, n := range []int{1, 10} {
		n := n
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			app := newBroadcast127TestApp(t)
			app.engine.SetPhase(game.PhaseStopped)

			baseURL := startEvictionTestServer(t, app)
			tvConn := dialWS(t, baseURL, "/ws/tv")
			learnClientID(t, app, tvConn)

			runPrepareToReadySequence(t, app, n)

			actions := collectActions(tvConn, 500*time.Millisecond)
			got := countActions(actions, protocol.ActionUpdate)
			t.Logf("N=%d: TV reçoit %d UPDATE (actions=%v)", n, got, actions)

			if got != 2 {
				t.Errorf("#129 CA12: TV devrait recevoir exactement 2 UPDATE entre PREPARE et READY (N=%d), reçu %d — actions=%v", n, got, actions)
			}
		})
	}
}

// Note R11 (buzzers must stay targeted on the per-PONG site, T1.6): not
// covered by an automated test here. BuzzerWebSocketHub is a separate hub/
// route (/ws/buzzer) from WebSocketHub; building a dedicated harness for one
// narrow assertion wasn't judged worth the added surface given the guarantee
// is a single, explicit, already-documented argument list at the call site
// (handlePong's #129 T1.6 comment) — verified instead by direct code
// inspection and left as an explicit code-review checklist item for Batch 3,
// same treatment as R8 in #127's own test file.
