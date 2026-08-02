package main

// ---------------------------------------------------------------------------
// #127 — Suite d'intégration bout-en-bout (T4.1, test-writer)
//
// Plan     : _work/reports/planner-20260802-212049.md, Phase 4 (T4.1-T4.2)
// Contrat  : contracts/vplayer-payload-filter.md (référence unique pour "qui
//            reçoit quoi")
// Maquette : _work/mockups/127-broadcast-matrix.md (diagrammes AVANT/APRÈS,
//            matrice §3, invariant §4, points de contrôle V1-V9)
//
// Objectif : mesurer ce qu'un client reçoit RÉELLEMENT sur le fil (vraies
// connexions WebSocket, vrai WebSocketHub.Run()), pas ce qu'une fonction
// isolée est censée faire — complémentaire de main_broadcast_127_test.go
// (T1.4, suite unitaire de dev-backend), pas un doublon :
//   - CA1 est ici vérifié à la valeur EXACTE promise par le contrat §1
//     ("exactement 2"), alors que le test T1.4 correspondant utilise
//     volontairement une garde plus lâche (`got >= n+1` échoue seulement).
//   - CA6 (invariant de restauration de la carte complète) n'est couvert
//     nulle part ailleurs.
//
// Ce fichier définit délibérément son propre harnais (newVPlayerIntegrationApp,
// setupParticipants, buildXxxMsg, collectMessages...) plutôt que de réutiliser
// les helpers de main_broadcast_127_test.go (fichier de dev-backend, en cours
// d'écriture en parallèle — Batch 1 du plan). Les helpers déjà committés et
// stables (newTestAppWithHub, startEvictionTestServer, dialWS, learnClientID,
// collectActions) SONT réutilisés — package-privés, les redéfinir ferait
// échouer la compilation.
//
// HISTORIQUE (résolu, commit 2a39fef) : la première version de cette suite a
// mesuré 4 UPDATE VJoueur sur la fenêtre PREPARE→READY, pas 2 — deux appels
// redondants hors du périmètre T1.1-T1.3 (handleReady() rappelait
// broadcastUpdate() après que OnStateChange l'ait déjà fait ; sendLEDSetAllBuzzers()
// rediffusait inconditionnellement, y compris vers VPlayer, alors que sa
// propre boucle ignore toujours les bumpers VPlayer). dev-backend a corrigé
// les deux sites (2a39fef) : CA1 est maintenant vérifié à la valeur exacte du
// contrat. Ces deux appels continuent en revanche de cibler Admin/TV/Buzzer
// (légitimement, cf. CA2 plus bas) — c'est ce qui porte leur cadence à N+3,
// pas N+2 comme une lecture superficielle du contrat le suggérerait.
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

// ---------------------------------------------------------------------------
// Harnais (propre à ce fichier — voir note en tête de fichier)
// ---------------------------------------------------------------------------

// newVPlayerIntegrationApp builds a real, fully wired App able to drive the
// handler chain used by a live game (handleReady, handlePong, broadcastReady,
// handleSimulatedButton, engine.Stop/Reveal) over real WebSocket connections.
//
//   - app.logger: handleReady() calls a.logger.Info/.Error directly (not the
//     nil-safe server.LogXxx package funcs) — required or it panics.
//   - app.udpBcast: broadcastReady/broadcastStart/broadcastStop/broadcastReveal
//     all call a.broadcast(..., viaTCP=true, ...) → a.udpBcast.Broadcast(msg).
//     newTestAppWithHub leaves it nil; a nil *server.UDPBroadcaster panics on
//     its first field access. An unstarted NewUDPBroadcaster() no-ops safely
//     (conn == nil) — same technique as main_broadcast_127_test.go.
//   - engine.OnStateChange: mirrors the one line of main.go's setupCallbacks
//     this suite needs (a.broadcastGameState(phase)). broadcastQuestions() is
//     deliberately NOT mirrored: Admin-only and touches disk, irrelevant here.
func newVPlayerIntegrationApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppWithHub(t)
	app.logger = server.InitLogger(100)
	app.udpBcast = server.NewUDPBroadcaster()
	app.engine.OnStateChange = func(phase game.GamePhase) {
		app.broadcastGameState(string(phase))
	}
	return app
}

// setupParticipants creates n virtual, connected VJoueur bumpers split across
// two teams (TeamA/TeamB), so AreAllTeamsReady() behaves like a real mixed
// game rather than a single-team edge case.
func setupParticipants(app *App, n int) []string {
	app.engine.SetTeams(map[string]*game.Team{
		"TeamA": {Name: "TeamA"},
		"TeamB": {Name: "TeamB"},
	})
	bumpers := make(map[string]*game.Bumper, n)
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("vp-integration-%d", i)
		team := "TeamA"
		if i%2 == 1 {
			team = "TeamB"
		}
		bumpers[id] = &game.Bumper{
			Name: id, Team: team, Connected: true,
			IsVirtual: true, IsVPlayer: true,
		}
		ids = append(ids, id)
	}
	app.engine.SetBumpers(bumpers)
	return ids
}

func buildReadyMsg(t *testing.T) *protocol.Message {
	t.Helper()
	// Question left empty on purpose: handleReady only touches disk
	// (loadQuestion) when payload.Question != "" — this exercises the real
	// handler without needing a question fixture on disk.
	msg, err := protocol.NewMessage(protocol.ActionReady, protocol.ReadyPayload{Question: ""})
	if err != nil {
		t.Fatalf("failed to build READY message: %v", err)
	}
	return msg
}

func buildPongMsg(t *testing.T, bumperID string) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionPong, map[string]string{"ID": bumperID})
	if err != nil {
		t.Fatalf("failed to build PONG message: %v", err)
	}
	return msg
}

func buildButtonMsg(t *testing.T, bumperID string) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionButton, map[string]string{"ID": bumperID, "button": "A"})
	if err != nil {
		t.Fatalf("failed to build simulated BUTTON message: %v", err)
	}
	return msg
}

// collectMessages drains whatever server→client frames arrive on conn within
// window, keeping the full parsed envelope (ACTION + MSG) — same read-loop
// technique as collectActions (player_evicted_test.go), but preserves MSG,
// needed here for CA6 (inspecting the bumpers map of a captured UPDATE).
func collectMessages(conn interface {
	SetReadDeadline(time.Time) error
	ReadMessage() (int, []byte, error)
}, window time.Duration) []*protocol.Message {
	var msgs []*protocol.Message
	deadline := time.Now().Add(window)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return msgs
		}
		conn.SetReadDeadline(time.Now().Add(remaining))
		_, data, err := conn.ReadMessage()
		if err != nil {
			return msgs // timeout (or closed) — no more messages
		}
		if msg, err := protocol.ParseSingle(data); err == nil {
			msgs = append(msgs, msg)
		}
	}
}

func countMsgAction(msgs []*protocol.Message, action string) int {
	n := 0
	for _, m := range msgs {
		if m.Action == action {
			n++
		}
	}
	return n
}

func firstMsgAction(msgs []*protocol.Message, action string) *protocol.Message {
	for _, m := range msgs {
		if m.Action == action {
			return m
		}
	}
	return nil
}

func actionsOf(msgs []*protocol.Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Action
	}
	return out
}

// parseBumperCountAndPhase inspects a captured UPDATE payload (the
// {"GAME":{...},"teams":{...},"bumpers":{...}} shape produced by
// engine.GetGameJSON — see internal/protocol/messages.go SerializeForWebClient
// doc comment) and returns len(bumpers) and GAME.PHASE.
func parseBumperCountAndPhase(t *testing.T, msg *protocol.Message) (int, string) {
	t.Helper()
	var body struct {
		Game struct {
			Phase string `json:"PHASE"`
		} `json:"GAME"`
		Bumpers map[string]json.RawMessage `json:"bumpers"`
	}
	if err := json.Unmarshal(msg.Msg, &body); err != nil {
		t.Fatalf("failed to parse UPDATE payload: %v (raw=%s)", err, msg.Msg)
	}
	return len(body.Bumpers), body.Game.Phase
}

// ---------------------------------------------------------------------------
// T4.1 — CA1 / CA2 / CA6 : séquence complète
// PREPARE → N PONG → READY → START → BUZZ → STOP → REVEAL, N=1 et N=10.
// ---------------------------------------------------------------------------

func TestVPlayerBroadcastIntegration_PrepareToRevealSequence(t *testing.T) {
	for _, n := range []int{1, 10} {
		n := n
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			app := newVPlayerIntegrationApp(t)
			app.engine.SetPhase(game.PhaseStopped) // READY() only accepts STOPPED/REVEALED/PREPARE/READY/NEW_GAME

			baseURL := startEvictionTestServer(t, app)
			adminConn := dialWS(t, baseURL, "/ws/admin")
			tvConn := dialWS(t, baseURL, "/ws/tv")
			vpConn := dialWS(t, baseURL, "/ws/player")

			// Identify our observing VJoueur connection with the first bumper.
			// Required for CA6 once Phase 2 (SerializeForVPlayer, T2.1-T2.3, not
			// yet implemented at the time this suite was written) individualizes
			// the fan-out per PlayerID (contract §2, condition 3) — without this
			// link the payload would always fall back to "complete", making the
			// restoration invariant trivially true instead of a real check.
			vpClientID := learnClientID(t, app, vpConn)
			ids := setupParticipants(app, n)
			app.wsHub.SetClientPlayerID(vpClientID, ids[0])

			// --- PREPARE entry ------------------------------------------------
			app.handleReady(buildReadyMsg(t))

			// --- N PONG → READY transition -------------------------------------
			for _, id := range ids {
				app.handlePong(id, buildPongMsg(t, id))
			}
			if !app.engine.IsGameReady() {
				t.Fatalf("setup failed: game did not reach READY after %d PONG(s)", n)
			}

			// Collection window: long enough for N direct handlePong() calls
			// (synchronous, no network round-trip) plus the READY transition's
			// broadcasts to flush through the hub's buffered Send channel and
			// the writePump goroutine onto the real loopback socket.
			vpMsgs := collectMessages(vpConn, 500*time.Millisecond)
			adminMsgs := collectMessages(adminConn, 100*time.Millisecond)
			tvMsgs := collectMessages(tvConn, 100*time.Millisecond)

			// --- CA1 — contracts/vplayer-payload-filter.md §1 -------------------
			// "il en reçoit désormais exactement 2 (entrée en PREPARE, puis
			// transition en READY), quel que soit N."
			if got := countMsgAction(vpMsgs, protocol.ActionUpdate); got != 2 {
				t.Errorf("CA1 (contracts/vplayer-payload-filter.md §1): VJoueur devrait recevoir exactement "+
					"2 UPDATE entre l'entrée en PREPARE et la transition en READY (N=%d), reçu %d — actions=%v",
					n, got, actionsOf(vpMsgs))
			}
			// Contract §1 last row: "Le VJoueur ne reçoit toujours jamais
			// l'action READY" — the phase transition is carried by the second
			// UPDATE's GAME.PHASE field, never by a dedicated READY action.
			if got := countMsgAction(vpMsgs, protocol.ActionReady); got != 0 {
				t.Errorf("VJoueur ne doit jamais recevoir l'action READY elle-même (contrat §1), reçu %d fois", got)
			}

			// --- CA2 — admin garde la cadence 1 UPDATE par PONG -----------------
			// entrée(1) + (N-1) PONG réguliers(N-1) + dernier PONG(3) = N+3.
			// Le dernier PONG (celui qui déclenche la transition) porte à lui
			// seul 3 UPDATE Admin/TV, comportement inchangé par #127 (ni
			// handleReady ni sendLEDSetAllBuzzers ne retirent Admin/TV de leur
			// broadcast, seul VPlayer en est retiré — commit 2a39fef) :
			//   1. handlePong() lui-même (tail call, T1.2)
			//   2. TransitionToReady() → OnStateChange(READY) → broadcastGameState()
			//   3. broadcastReady() → sendLEDSetAllBuzzers() (ACK_PENDING sync,
			//      légitimement toujours utile à Admin/TV même sans VPlayer)
			if got := countMsgAction(adminMsgs, protocol.ActionUpdate); got != n+3 {
				t.Errorf("CA2: admin devrait recevoir %d UPDATE (1 entrée + %d PONG réguliers + 3 sur le dernier PONG), reçu %d — actions=%v",
					n+3, n-1, got, actionsOf(adminMsgs))
			}
			// TV suit la même cadence que l'admin (contenu filtré, nombre de
			// messages inchangé — contrat §1, colonne TV).
			if got := countMsgAction(tvMsgs, protocol.ActionUpdate); got != n+3 {
				t.Errorf("TV devrait recevoir la même cadence que l'admin (%d UPDATE), reçu %d — actions=%v",
					n+3, got, actionsOf(tvMsgs))
			}

			// --- START → BUZZ → STOP → REVEAL -----------------------------------
			// Nouvelle connexion dédiée à la lecture post-BUZZ, dialée et
			// identifiée AVANT de déclencher le BUZZ (sinon le broadcast qui
			// nous intéresse partirait avant que ce client ne soit enregistré
			// dans le hub, et on ne le verrait jamais). Nécessaire car
			// collectMessages() termine TOUJOURS sa fenêtre par un timeout de
			// lecture (c'est ainsi qu'il sait qu'il n'y a plus rien à lire) —
			// or un *websocket.Conn gorilla est documenté comme inutilisable
			// pour un futur ReadMessage une fois qu'un timeout de lecture s'est
			// produit dessus. Réutiliser vpConn ici (déjà "consommée" par le
			// premier appel à collectMessages() plus haut) retournerait donc
			// toujours une liste vide, faisant systématiquement échouer CA6 —
			// bug de harnais repéré empiriquement par dev-backend, sans rapport
			// avec le comportement du serveur. On relie la nouvelle connexion
			// au même PlayerID pour observer le même destinataire individualisé
			// (contrat §2, condition 3).
			vpConn2 := dialWS(t, baseURL, "/ws/player")
			vpClientID2 := learnClientID(t, app, vpConn2)
			app.wsHub.SetClientPlayerID(vpClientID2, ids[0])

			// engine.Start() lance un vrai countdown de plusieurs secondes
			// (ticker réel) : on saute directement à STARTED, comme les autres
			// suites du projet (cf. cmd/server/led_test.go), et on émet
			// explicitement le broadcast START correspondant.
			app.engine.SetPhase(game.PhaseStarted)
			app.broadcastStart()
			app.handleSimulatedButton(buildButtonMsg(t, ids[0]))

			postBuzz := collectMessages(vpConn2, 500*time.Millisecond)

			// --- CA6 — invariant de restauration (contrat §2, maquette §4) -----
			firstUpdate := firstMsgAction(postBuzz, protocol.ActionUpdate)
			if firstUpdate == nil {
				t.Fatalf("CA6: aucun UPDATE reçu par le VJoueur après la sortie de READY (attendu : celui déclenché par le BUZZ) — actions=%v", actionsOf(postBuzz))
			}
			bumperCount, phase := parseBumperCountAndPhase(t, firstUpdate)
			if phase == "PREPARE" || phase == "READY" {
				t.Fatalf("bug de séquencement du test : le premier UPDATE post-READY porte encore la phase %q", phase)
			}
			if bumperCount != n {
				t.Errorf("CA6 (contracts/vplayer-payload-filter.md §2, invariant de restauration) : le premier "+
					"UPDATE reçu par le VJoueur après avoir quitté PREPARE/READY (phase=%s) doit porter la carte "+
					"bumpers COMPLETE (%d entrées), reçu %d — un VJoueur resté sur une carte réduite verrait des "+
					"badges d'équipe vides (risque R4 du plan, point V6 de la maquette)", phase, n, bumperCount)
			}

			// Engine-level sanity check on the STOP/REVEAL leg, so a future
			// regression there doesn't silently no-op through this suite.
			app.engine.Stop()
			app.broadcastStop()
			answer := app.engine.Reveal()
			app.broadcastReveal(answer)
			if got := app.engine.GetPhase(); got != game.PhaseRevealed {
				t.Errorf("attendu phase REVEALED après Stop()+Reveal(), obtenu %q", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CA7 — pas de MessageLost fantôme pendant la rafale de PONG.
//
// Couverture complémentaire, à l'échelle intégration : un VJoueur DÉCONNECTÉ
// (jamais de connexion WebSocket réelle dans ce test) ne doit voir aucune
// dégradation de son badge de connexion (#109/#118) simplement parce que
// d'autres joueurs envoient des PONG pendant qu'il est hors-jeu. Déjà couvert
// unitairement par dev-backend (main_broadcast_127_test.go,
// TestBroadcast127_HandlePong_DoesNotEvaluateVPlayerConnEvents) — ce test-ci
// est une vérification indépendante, écrite à partir du contrat plutôt que du
// code, avec un setup légèrement différent (bumper jamais connecté du tout,
// plutôt que déconnecté après coup).
// ---------------------------------------------------------------------------

func TestVPlayerBroadcastIntegration_CA7_OfflineBumperConnStateUnaffectedByPongRafale(t *testing.T) {
	app := newVPlayerIntegrationApp(t)
	app.engine.SetTeams(map[string]*game.Team{
		"TeamA": {Name: "TeamA"},
		"TeamB": {Name: "TeamB"},
	})
	app.engine.SetBumpers(map[string]*game.Bumper{
		"online": {
			Name: "Online", Team: "TeamA", Connected: true,
			IsVirtual: true, IsVPlayer: true,
		},
		"offline": {
			Name: "Offline", Team: "TeamB", Connected: false, ConnState: game.ConnStateOrange,
			IsVirtual: true, IsVPlayer: true,
		},
	})
	app.engine.SetPhase(game.PhasePrepare)

	// 10 PONGs from the online bumper only. TeamB (the offline bumper's team)
	// never reaches "ready" — AreAllTeamsReady() stays false, no READY
	// transition fires. Irrelevant here: this test only exercises the
	// per-PONG path (handlePong's tail call, T1.2), which must never touch
	// VPlayer/ApplyVPlayerBroadcastConnEvents regardless of the transition.
	for i := 0; i < 10; i++ {
		app.handlePong(fmt.Sprintf("test-pong-client-%d", i), buildPongMsg(t, "online"))
	}

	if got := app.engine.GetBumper("offline").ConnState; got != game.ConnStateOrange {
		t.Errorf("CA7 (contracts/vplayer-payload-filter.md, plan risque R1) : le badge du VJoueur hors-ligne "+
			"a dégradé de ORANGE vers %q après 10 broadcasts Admin/TV/Buzzer-only qu'il n'a jamais reçus — "+
			"ApplyVPlayerBroadcastConnEvents() ne doit s'exécuter que sur un broadcast qui cible réellement VPlayer", got)
	}
}
