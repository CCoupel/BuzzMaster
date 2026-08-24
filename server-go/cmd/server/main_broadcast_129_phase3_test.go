package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// #129 Phase 3 (T3.1/T3.2) — handleVPlayerQCMAnswer, handleButton,
// handleSimulatedButton retargeted: Admin/TV/Buzzer see every buzz; the
// buzzing VPlayer gets a targeted echo; every OTHER VPlayer gets nothing.
// broadcastPause (unchanged) still reaches everyone.
//
// Extends the harness of main_broadcast_127_test.go /
// main_broadcast_129_test.go, doesn't duplicate it.
// ---------------------------------------------------------------------------

func qcmAnswerMsg(t *testing.T, bumperID, color string) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionVPlayerQCMAnswer, protocol.VPlayerQCMAnswerPayload{ID: bumperID, AnswerColor: color})
	if err != nil {
		t.Fatalf("failed to build VPLAYER_QCM_ANSWER message: %v", err)
	}
	return msg
}

func simulatedButtonMsg(t *testing.T, bumperID, button string) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionButton, map[string]interface{}{"ID": bumperID, "button": button})
	if err != nil {
		t.Fatalf("failed to build BUTTON message: %v", err)
	}
	return msg
}

// setupQCMStartedGame creates n VPlayer bumpers (one per team, round-robin
// across 2 teams) and puts the engine in STARTED phase with a QCM question —
// the gating handleVPlayerQCMAnswer requires (bumper.IsVPlayer, PhaseStarted,
// Question.Type==QCM).
func setupQCMStartedGame(t *testing.T, app *App, n int) []string {
	t.Helper()
	app.engine.SetPhase(game.PhaseEnroll) // CreateVirtualPlayer requires PhaseEnroll
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}, "TeamB": {Name: "TeamB"}})

	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		team := "TeamA"
		if i%2 == 1 {
			team = "TeamB"
		}
		ids = append(ids, setupVirtualPlayer(t, app, fmt.Sprintf("Player%d", i), team))
	}

	// Ready()/SetPhase(STARTED) mirror TestBroadcast129_ArdoiseInput_*'s
	// setup: Ready() only accepts STOPPED/REVEALED/PREPARE/READY/NEW_GAME as
	// its starting phase — switch away from PhaseEnroll first.
	app.engine.SetPhase(game.PhaseStopped)
	app.engine.Ready("", &game.Question{
		ID: "q-qcm", Type: game.QuestionTypeQCM,
		TypedContent: game.TypedContent{
			QCMAnswers: &game.QCMAnswers{Red: "A", Green: "B", Yellow: "C", Blue: "D"},
		},
	})
	app.engine.SetPhase(game.PhaseStarted)

	return ids
}

// ---------------------------------------------------------------------------
// CA10 — chaque joueur qui répond reçoit la confirmation de SA réponse ;
// les autres VJoueurs ne reçoivent plus d'UPDATE de ce fait.
// ---------------------------------------------------------------------------

func TestBroadcast129_QCMAnswer_EachPlayerReceivesOnlyOwnEchoZeroFromOthers(t *testing.T) {
	const n = 10
	app := newBroadcast127TestApp(t)
	ids := setupQCMStartedGame(t, app, n)

	baseURL := startEvictionTestServer(t, app)
	playerConns := make([]*wsConnWithID, 0, n)
	for _, id := range ids {
		conn := dialWS(t, baseURL, "/ws/player")
		clientID := learnClientID(t, app, conn)
		app.wsHub.SetClientPlayerID(clientID, id)
		playerConns = append(playerConns, &wsConnWithID{conn: conn, playerID: id})
	}

	colors := []string{"RED", "GREEN", "YELLOW", "BLUE"}
	for i, id := range ids {
		app.handleVPlayerQCMAnswer(id, qcmAnswerMsg(t, id, colors[i%len(colors)]))
	}

	for i, pc := range playerConns {
		actions := collectActions(pc.conn, 500*time.Millisecond)
		got := countActions(actions, protocol.ActionUpdate)
		if got != 1 {
			t.Errorf("#129 CA10: player %d (%s) should receive exactly 1 UPDATE (its own echo, %d peers also answered), got %d — actions=%v", i, pc.playerID, n-1, got, actions)
		}
	}
}

type wsConnWithID struct {
	conn     *websocket.Conn
	playerID string
}

// ---------------------------------------------------------------------------
// CA10 — PAUSE reste diffusé à tous, inchangé.
// ---------------------------------------------------------------------------

func TestBroadcast129_QCMAnswer_PauseStillReachesAllVPlayers(t *testing.T) {
	app := newBroadcast127TestApp(t)
	ids := setupQCMStartedGame(t, app, 3)

	baseURL := startEvictionTestServer(t, app)
	// An observer VPlayer that does NOT answer — should still see PAUSE from
	// ids[0]'s answer (broadcastPause is unchanged, still a full broadcast).
	observerConn := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, observerConn)

	app.handleVPlayerQCMAnswer(ids[0], qcmAnswerMsg(t, ids[0], "RED"))

	actions := collectActions(observerConn, 500*time.Millisecond)
	if !containsAction(actions, protocol.ActionPause) {
		t.Errorf("#129: PAUSE must still reach every VPlayer (broadcastPause untouched by T3.1), got actions=%v", actions)
	}
	// The observer must NOT get an UPDATE from a peer's answer (only PAUSE).
	if got := countActions(actions, protocol.ActionUpdate); got != 0 {
		t.Errorf("#129 CA10: observer (didn't answer) should receive 0 UPDATE from a peer's QCM answer, got %d — actions=%v", got, actions)
	}
}

// ---------------------------------------------------------------------------
// Le vrai chemin de buzz VJoueur (BUTTON via handleSimulatedButton,
// VPlayerPage.jsx:429/560) suit le même ciblage — trouvé hors du scope
// explicite T3.1, même justification (voir commentaire du call site).
// ---------------------------------------------------------------------------

func TestBroadcast129_SimulatedButtonBuzz_TargetsOwnPlayerOnlyZeroFromOthers(t *testing.T) {
	app := newBroadcast127TestApp(t)
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}, "TeamB": {Name: "TeamB"}})
	buzzerID := setupVirtualPlayer(t, app, "Buzzer", "TeamA")
	observerID := setupVirtualPlayer(t, app, "Observer", "TeamB")
	app.engine.SetPhase(game.PhaseStarted)

	baseURL := startEvictionTestServer(t, app)
	buzzerConn := dialWS(t, baseURL, "/ws/player")
	buzzerClientID := learnClientID(t, app, buzzerConn)
	app.wsHub.SetClientPlayerID(buzzerClientID, buzzerID)

	observerConn := dialWS(t, baseURL, "/ws/player")
	observerClientID := learnClientID(t, app, observerConn)
	app.wsHub.SetClientPlayerID(observerClientID, observerID)

	app.handleSimulatedButton(simulatedButtonMsg(t, buzzerID, "A"))

	buzzerActions := collectActions(buzzerConn, 500*time.Millisecond)
	if got := countActions(buzzerActions, protocol.ActionUpdate); got != 1 {
		t.Errorf("#129: buzzing VPlayer should receive exactly 1 targeted UPDATE echo, got %d — actions=%v", got, buzzerActions)
	}

	observerActions := collectActions(observerConn, 500*time.Millisecond)
	if got := countActions(observerActions, protocol.ActionUpdate); got != 0 {
		t.Errorf("#129: a VPlayer who didn't buzz should receive 0 UPDATE from a peer's buzz, got %d — actions=%v", got, observerActions)
	}
	if !containsAction(observerActions, protocol.ActionPause) {
		t.Errorf("#129: PAUSE must still reach every VPlayer on a buzz, got actions=%v", observerActions)
	}
}

// ---------------------------------------------------------------------------
// Le chemin buzzer physique (handleButton) ne cible jamais de VPlayer — le
// bumperID y est toujours un MAC de buzzer physique, jamais IsVPlayer.
// ---------------------------------------------------------------------------

func TestBroadcast129_HandleButton_PhysicalBuzzer_NoTargetedEchoToAnyVPlayer(t *testing.T) {
	app := newBroadcast127TestApp(t)
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	observerID := setupVirtualPlayer(t, app, "Observer", "TeamA")
	app.engine.SetBumpers(map[string]*game.Bumper{
		observerID:    app.engine.GetBumper(observerID),
		"AA:BB:CC:01": {Name: "Buzzer1", Team: "TeamA", Connected: true}, // physical, IsVirtual=false
	})
	app.engine.SetPhase(game.PhaseStarted)

	baseURL := startEvictionTestServer(t, app)
	observerConn := dialWS(t, baseURL, "/ws/player")
	observerClientID := learnClientID(t, app, observerConn)
	app.wsHub.SetClientPlayerID(observerClientID, observerID)

	app.handleButton("AA:BB:CC:01", buttonMsgFor(t), time.Now().UnixMicro())

	actions := collectActions(observerConn, 500*time.Millisecond)
	if got := countActions(actions, protocol.ActionUpdate); got != 0 {
		t.Errorf("#129: a physical buzzer press must not produce a targeted UPDATE echo to any VPlayer, got %d — actions=%v", got, actions)
	}
	if !containsAction(actions, protocol.ActionPause) {
		t.Errorf("#129: PAUSE must still reach every VPlayer on a physical buzz, got actions=%v", actions)
	}
}

func buttonMsgFor(t *testing.T) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionButton, protocol.ButtonPayload{Button: "A"})
	if err != nil {
		t.Fatalf("failed to build BUTTON message: %v", err)
	}
	return msg
}
