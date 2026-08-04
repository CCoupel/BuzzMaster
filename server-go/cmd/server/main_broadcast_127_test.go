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
// #127 — rafale de broadcasts non groupée à la transition PREPARE→READY
//
// Plan : _work/reports/planner-20260802-212049.md (T1.1-T1.4)
// Contrat : contracts/vplayer-payload-filter.md §1 (matrice de diffusion)
// Handoff : _work/handoff/task-dev-backend-20260802-222654.md
//
// Ces tests exercent le vrai chemin de code (handleReady -> engine.Ready ->
// OnStateChange -> broadcastGameState, handlePong, TransitionToReady) via de
// vraies connexions WebSocket (même harnais que player_evicted_test.go :
// startEvictionTestServer/dialWS/learnClientID/readAction/collectActions),
// pas des appels directs aux fonctions internes — pour mesurer ce qu'un
// client reçoit réellement sur le fil, pas ce que le code est censé faire.
// ---------------------------------------------------------------------------

// wireOnStateChange mirrors the relevant slice of main.go's setupCallbacks
// (a.engine.OnStateChange = func(phase) { a.broadcastGameState(...); ... }),
// which newTestAppWithHub does NOT wire by default (it builds a minimal App).
// broadcastQuestions() is deliberately NOT mirrored here: it is Admin-only
// (main.go:3377, verified in round-2 investigation) and touches disk
// (a.loadQuestions()), irrelevant to the UPDATE-counting assertions below.
func wireOnStateChange(app *App) {
	app.engine.OnStateChange = func(phase game.GamePhase) {
		app.broadcastGameState(string(phase))
	}
}

// newBroadcast127TestApp builds on newTestAppWithHub with the extra wiring
// this file's tests need: a real (but unstarted) UDPBroadcaster — required
// because AreAllTeamsReady() can trigger broadcastReady(), which calls
// a.udpBcast.Broadcast(); newTestApp leaves udpBcast nil (a *UDPBroadcaster
// method call on a nil receiver panics on its first field access) — and the
// OnStateChange callback (see wireOnStateChange).
func newBroadcast127TestApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppWithHub(t)
	app.logger = server.NewBroadcastLogger(100)
	app.udpBcast = server.NewUDPBroadcaster() // unstarted: Broadcast() no-ops safely (conn == nil)
	wireOnStateChange(app)
	return app
}

// readyMsg builds a READY message with an empty QUESTION id — handleReady
// only calls loadQuestion (disk I/O) when payload.Question != "", so this
// exercises the real handler without needing a question fixture on disk.
func readyMsg(t *testing.T) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionReady, protocol.ReadyPayload{Question: ""})
	if err != nil {
		t.Fatalf("failed to build READY message: %v", err)
	}
	return msg
}

func pongMsg(t *testing.T, bumperID string) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionPong, map[string]interface{}{"ID": bumperID})
	if err != nil {
		t.Fatalf("failed to build PONG message: %v", err)
	}
	return msg
}

// setupPrepareReadyGame creates n participants split across two teams
// (TeamA/TeamB), all connected, ready to receive PONGs.
func setupPrepareReadyGame(app *App, n int) []string {
	teams := map[string]*game.Team{
		"TeamA": {Name: "TeamA"},
		"TeamB": {Name: "TeamB"},
	}
	app.engine.SetTeams(teams)

	bumpers := map[string]*game.Bumper{}
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("bumper-%d", i)
		teamName := "TeamA"
		if i%2 == 1 {
			teamName = "TeamB"
		}
		bumpers[id] = &game.Bumper{Name: id, Team: teamName, Connected: true}
		ids = append(ids, id)
	}
	app.engine.SetBumpers(bumpers)
	return ids
}

// countActions counts occurrences of want among a list of collected ACTIONs
// (see collectActions).
func countActions(actions []string, want string) int {
	n := 0
	for _, a := range actions {
		if a == want {
			n++
		}
	}
	return n
}

// runPrepareToReadySequence drives the real handlers for n participants and
// returns, for each of the given connections, every ACTION observed within
// the collection window (long enough for N pongs + the READY transition to
// settle, short enough to keep the suite fast).
func runPrepareToReadySequence(t *testing.T, app *App, n int) []string {
	t.Helper()
	ids := setupPrepareReadyGame(app, n)

	app.handleReady(readyMsg(t))
	for _, id := range ids {
		app.handlePong(id, pongMsg(t, id))
	}
	return ids
}

// ---------------------------------------------------------------------------
// CA1 — un VJoueur reçoit un nombre constant de messages UPDATE entre
// l'entrée en PREPARE et la transition en READY, quel que soit N.
// ---------------------------------------------------------------------------

func TestBroadcast127_VPlayer_UpdateCount_ConstantAcrossN(t *testing.T) {
	for _, n := range []int{1, 10} {
		n := n
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			app := newBroadcast127TestApp(t)
			app.engine.SetPhase(game.PhaseStopped)

			baseURL := startEvictionTestServer(t, app)
			vpConn := dialWS(t, baseURL, "/ws/player")
			learnClientID(t, app, vpConn) // synchronize: guarantees registration in h.clients before any broadcast

			runPrepareToReadySequence(t, app, n)

			actions := collectActions(vpConn, 500*time.Millisecond)
			got := countActions(actions, protocol.ActionUpdate)
			t.Logf("N=%d: VJoueur reçoit %d UPDATE (actions=%v)", n, got, actions)

			// CA1 (contracts/vplayer-payload-filter.md §1): exactly 2 — entrée
			// en PREPARE, puis transition en READY — quel que soit N. Two
			// extra redundant sources had to be fixed beyond T1.1-T1.3's
			// explicit scope to reach this exact count (see handoff/report):
			// handleReady()'s own duplicate broadcastUpdate() call, and
			// sendLEDSetAllBuzzers()'s unconditional trailing broadcastUpdate()
			// (fired via broadcastReady() at the READY transition).
			if got != 2 {
				t.Errorf("#127 CA1: VJoueur devrait recevoir exactement 2 UPDATE entre PREPARE et READY (N=%d), reçu %d — actions=%v", n, got, actions)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CA2 — l'admin continue de recevoir un UPDATE par PONG (cadence inchangée),
// et pas seulement à la fin de la rafale.
// ---------------------------------------------------------------------------

// Note on technique: this does NOT read incrementally after each PONG. A
// gorilla/websocket *Conn is single-shot after any read error (including a
// deadline timeout) — a prior version of this test called collectActions
// (which times out by design to signal "no more messages right now") in a
// loop on the SAME connection, and the very first timeout permanently broke
// all subsequent reads on it (silently returning zero actions from then on,
// which looked exactly like a cadence regression but wasn't one). Instead,
// the whole PREPARE->N PONG->READY sequence runs first, then every action is
// collected in a single pass — same technique as the VPlayer count test.
func TestBroadcast127_Admin_UpdateCadence_OnePerPong(t *testing.T) {
	const n = 5
	app := newBroadcast127TestApp(t)
	app.engine.SetPhase(game.PhaseStopped)

	baseURL := startEvictionTestServer(t, app)
	adminConn := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, adminConn) // synchronize: guarantees registration in h.clients before any broadcast

	runPrepareToReadySequence(t, app, n)

	actions := collectActions(adminConn, 500*time.Millisecond)
	got := countActions(actions, protocol.ActionUpdate)
	t.Logf("admin received %d UPDATE across PREPARE + %d PONG + READY (actions=%v)", got, n, actions)

	// #127 R6 / CA2 regression guard: the per-PONG cadence must not have been
	// collapsed into a single end-of-rafale update — admin must see at least
	// one UPDATE per PONG (TeamCard "prêt" progress, GamePage.jsx), on top of
	// the phase-transition broadcasts. If handlePong's per-PONG call were
	// wrongly moved inside the AreAllTeamsReady() branch (R6), admin would
	// receive far fewer than n UPDATEs for n participants.
	if got < n {
		t.Errorf("#127 R6 regression: admin received only %d UPDATE for %d PONGs — expected at least one per PONG (live 'prêt' progress collapsed to end-of-rafale?)", got, n)
	}
}

// ---------------------------------------------------------------------------
// CA7 — ApplyVPlayerBroadcastConnEvents ne s'exécute pas sur un broadcast qui
// ne cible pas les VJoueurs (handlePong désormais Admin+TV+Buzzer only).
// Vérifié via l'effet observable documenté par
// connstate_protocol_regression_test.go : un VJoueur orange (déconnecté, pas
// encore de message manqué) ne doit PAS passer rouge à cause d'un broadcast
// dont il n'est pas destinataire.
// ---------------------------------------------------------------------------

func TestBroadcast127_HandlePong_DoesNotEvaluateVPlayerConnEvents(t *testing.T) {
	app := newBroadcast127TestApp(t)
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})

	id, _, err := app.engine.CreateVirtualPlayer("Alice")
	if err != nil {
		t.Fatalf("CreateVirtualPlayer failed: %v", err)
	}
	if err := app.engine.AssignVirtualPlayer(id, "TeamA", game.AnswerColorNone); err != nil {
		t.Fatalf("AssignVirtualPlayer failed: %v", err)
	}
	app.onPlayerDisconnected(id)
	if got := app.engine.GetBumper(id).ConnState; got != "orange" {
		t.Fatalf("setup failed: expected orange after disconnect, got %q", got)
	}

	// Move to PREPARE and fire a PONG from an unrelated physical bumper. This
	// only targets Admin/TV/Buzzer (T1.2) — must NOT flip Alice orange->red.
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}, "TeamB": {Name: "TeamB"}})
	app.engine.SetBumpers(map[string]*game.Bumper{
		id:           app.engine.GetBumper(id),
		"buzzer-phy": {Name: "buzzer-phy", Team: "TeamB", Connected: true},
	})
	app.engine.SetPhase(game.PhasePrepare)
	app.handlePong("buzzer-phy", pongMsg(t, "buzzer-phy"))

	if got := app.engine.GetBumper(id).ConnState; got != "orange" {
		t.Errorf("#127 CA7 regression: handlePong's Admin/TV/Buzzer-only broadcast evaluated VJoueur conn events (ConnState=%q, expected still 'orange') — ApplyVPlayerBroadcastConnEvents must only run when VPlayer is targeted", got)
	}
}

// ---------------------------------------------------------------------------
// CA3 — l'UPDATE émis sur changement de phase (broadcastGameState) ne
// contient plus les champs OTA/ACK pour TV ; l'admin les reçoit toujours.
// ---------------------------------------------------------------------------

func TestBroadcast127_BroadcastGameState_StripsOTAFieldsForTV_KeepsForAdmin(t *testing.T) {
	app := newBroadcast127TestApp(t)
	app.engine.SetPhase(game.PhaseStopped)

	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	app.engine.SetBumpers(map[string]*game.Bumper{
		"buzzer-1": {
			Name: "buzzer-1", Team: "TeamA", Connected: true,
			FirmwareVersion: "3.8.2", OTAStatus: "downloading",
		},
	})

	baseURL := startEvictionTestServer(t, app)
	adminConn := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, adminConn) // synchronize: guarantees registration in h.clients before any broadcast
	tvConn := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tvConn)

	app.handleReady(readyMsg(t))

	adminAction, adminMsg := readActionMatching(t, adminConn, protocol.ActionUpdate)
	if adminAction != protocol.ActionUpdate {
		t.Fatalf("expected admin's first UPDATE, got %q", adminAction)
	}
	if !containsRawSubstring(adminMsg, "FIRMWARE_VERSION") {
		t.Errorf("CA3 regression: admin must still receive FIRMWARE_VERSION on broadcastGameState, got: %s", adminMsg)
	}

	tvAction, tvMsg := readActionMatching(t, tvConn, protocol.ActionUpdate)
	if tvAction != protocol.ActionUpdate {
		t.Fatalf("expected TV's first UPDATE, got %q", tvAction)
	}
	if containsRawSubstring(tvMsg, "FIRMWARE_VERSION") || containsRawSubstring(tvMsg, "OTA_STATUS") {
		t.Errorf("CA3 regression: TV must NOT receive FIRMWARE_VERSION/OTA_STATUS on broadcastGameState (unfiltered path), got: %s", tvMsg)
	}
}

// Note R8 (broadcastGameState must never target buzzerHub): not covered by
// an automated test here — newTestApp's buzzerHub is a real (empty)
// BuzzerWebSocketHub, so a wrongly-targeted broadcast would silently no-op
// rather than panic or fail an assertion, and BuzzerWebSocketHub exposes no
// call-count hook to instrument cheaply. Verified instead by direct code
// inspection: broadcastGameState's target list (cmd/server/main.go) is
// literally broadcastUpdateTo(Admin, TV, VPlayer) — server.ClientTypeBuzzer
// does not appear. Left as an explicit code-review checklist item (R8).

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// readActionMatching drains messages on conn until it finds one whose ACTION
// equals want (or the deadline is hit), skipping any unrelated traffic
// (e.g. HEARTBEAT) in between.
func readActionMatching(t *testing.T, conn interface {
	SetReadDeadline(time.Time) error
	ReadMessage() (int, []byte, error)
}, want string) (string, string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("expected %s, got error: %v", want, err)
		}
		var envelope struct {
			Action string `json:"ACTION"`
		}
		if json.Unmarshal(data, &envelope) == nil && envelope.Action == want {
			return envelope.Action, string(data)
		}
	}
	t.Fatalf("timed out waiting for action %s", want)
	return "", ""
}

func containsRawSubstring(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
