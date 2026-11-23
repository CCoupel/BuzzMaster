package main

import (
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// #192 (v13.0.0, LOT-2.1) — NON-FUITE de PICTO_WORDS au site d'appel cmd/server (fan-out réel de
// broadcastUpdateTo et envoi initial sendStateToClient), réserve de revue LOT-1, contracts/picto.md §10 — CRITICAL.
// Modèle : rafale_answer_leak_test.go (vraie WS, 4 types de client). Les mots ne doivent figurer dans AUCUN
// message de /ws/tv ni /ws/player ; /ws/admin et /ws/anim les gardent.
// ---------------------------------------------------------------------------

var pictoLeakWords192 = []string{"Éléphant-secret", "Pont-levis-secret"}

func newPictoLeakApp192(t *testing.T, phase game.GamePhase) *App {
	t.Helper()
	app := newAnimTestApp(t)
	q := &game.Question{
		ID: "picto-leak-q", Question: "Dessinez", Type: game.QuestionTypePicto, Category: game.CategoryHistory,
		Points: "10", Time: "90",
		TypedContent: game.TypedContent{PictoWords: pictoLeakWords192},
	}
	app.engine.Ready(q.ID, q)
	app.engine.SetPhase(phase)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10)) // sendStateToClient (admin) le déréférence
	return app
}

// drainFrames lit tout ce qui arrive pendant d (rien de plus à lire = timeout).
func drainFrames192(conn *websocket.Conn, d time.Duration) []string {
	var out []string
	deadline := time.Now().Add(d)
	for {
		conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			return out
		}
		out = append(out, string(data))
	}
}

func joinHas192(frames []string, frag string) bool {
	for _, f := range frames {
		if strings.Contains(f, frag) {
			return true
		}
	}
	return false
}

func TestPictoLeakFanout192_RealWS_BroadcastAndInitialState(t *testing.T) {
	for _, phase := range []game.GamePhase{game.PhasePrepare, game.PhaseReady, game.PhaseStarted, game.PhasePaused, game.PhaseStopped, game.PhaseRevealed} {
		t.Run(string(phase), func(t *testing.T) {
			app := newPictoLeakApp192(t, phase)
			baseURL := startAnimAllowlistTestServer(t, app)
			admin := dialWS(t, baseURL, "/ws/admin")
			anim := dialWS(t, baseURL, "/ws/anim")
			tv := dialWS(t, baseURL, "/ws/tv")
			tablet := dialWS(t, baseURL, "/ws/player")
			ids := map[string]string{
				"admin": learnClientID(t, app, admin), "anim": learnClientID(t, app, anim),
				"tv": learnClientID(t, app, tv), "tablet": learnClientID(t, app, tablet),
			}

			app.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeAnim, server.ClientTypeVPlayer)
			app.sendStateToClient(ids["admin"], server.ClientTypeAdmin)
			app.sendStateToClient(ids["anim"], server.ClientTypeAnim)
			app.sendStateToClient(ids["tv"], server.ClientTypeTV)
			app.sendStateToClient(ids["tablet"], server.ClientTypeVPlayer)

			for who, conn := range map[string]*websocket.Conn{"tv": tv, "tablet": tablet} {
				frames := drainFrames192(conn, 400*time.Millisecond)
				if len(frames) == 0 {
					t.Fatalf("%s : aucun message reçu, le test ne prouverait rien", who)
				}
				for _, w := range append([]string{"PICTO_WORDS"}, pictoLeakWords192...) {
					if joinHas192(frames, w) {
						t.Errorf("%s/%s : %q fuit (contrat §10) — fan-out cmd/server", who, phase, w)
					}
				}
			}
			for who, conn := range map[string]*websocket.Conn{"admin": admin, "anim": anim} {
				frames := drainFrames192(conn, 400*time.Millisecond)
				if !joinHas192(frames, pictoLeakWords192[0]) {
					t.Errorf("%s/%s : doit recevoir PICTO_WORDS (conduite de la manche)", who, phase)
				}
			}
		})
	}
}
