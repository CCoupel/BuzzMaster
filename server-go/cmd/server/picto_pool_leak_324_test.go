package main

import (
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// #324 (v13.1.0, LOT-2) — NON-FUITE des mots du Pool Picto au site d'appel cmd/server (vraie WS), source RESERVOIR.
// Contrat picto.md §10 et picto-pool.md §3 : le contenu du pool et le mot en cours ne figurent dans AUCUN message de
// /ws/tv ni /ws/player ; seule la CATÉGORIE du mot (PICTO_WORD_CATEGORY, indice public) et le compte
// (PICTO_POOL_AVAILABLE) sont publics. Même modèle que picto_leak_fanout_192_test.go. CRITICAL.
// ---------------------------------------------------------------------------

var poolLeakWords324 = []string{"Mot-pool-secret-un", "Mot-pool-secret-deux", "Mot-pool-secret-trois"}

func newPictoPoolLeakApp324(t *testing.T) *App {
	t.Helper()
	app := newAnimTestApp(t)
	app.setupPictoCallbacks()
	for i, w := range poolLeakWords324 {
		if _, err := app.engine.PictoPool().Upsert(game.PoolEntry{Question: w, Category: game.CategoryHistory, Difficulty: i%3 + 1}); err != nil {
			t.Fatal(err)
		}
	}
	app.engine.SetTeams(map[string]*game.Team{"red": {Name: "red", Color: []int{255, 0, 0}, Ready: true}})
	app.engine.SetBumpers(map[string]*game.Bumper{
		"v-alice": {Name: "Alice", Team: "red", IsVPlayer: true, IsVirtual: true, Connected: true, Ready: true},
	})
	q := &game.Question{
		ID: "picto-pool-leak-q", Question: "Dessinez", Type: game.QuestionTypePicto, Category: game.CategoryHistory,
		Points: "10", Time: "90",
		TypedContent: game.TypedContent{PictoSource: game.PictoSourceReservoir, PictoCategories: []string{"HISTORY"}, PictoDifficulties: []int{1, 2, 3}},
	}
	app.engine.Ready(q.ID, q)
	app.engine.SetBumperReady("v-alice")
	if err := app.engine.PictoSetTeam("red"); err != nil {
		t.Fatal(err)
	}
	if err := app.engine.PictoSetDrawer("v-alice"); err != nil {
		t.Fatal(err)
	}
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	return app
}

func TestPictoPoolLeak324_RealWS_TVAndTabletNeverSeePoolWords_CategoryHintIsPublic(t *testing.T) {
	for _, started := range []bool{false, true} {
		name := "preparation"
		if started {
			name = "manche_demarree"
		}
		t.Run(name, func(t *testing.T) {
			app := newPictoPoolLeakApp324(t)
			if started {
				app.engine.StartImmediate(60)
				if app.engine.GetState().Phase != game.PhaseStarted {
					t.Fatalf("manche non démarrée : %s", app.engine.GetState().Phase)
				}
			}
			baseURL := startAnimAllowlistTestServer(t, app)
			admin := dialWS(t, baseURL, "/ws/admin")
			tv := dialWS(t, baseURL, "/ws/tv")
			tablet := dialWS(t, baseURL, "/ws/player")
			ids := map[string]string{
				"admin": learnClientID(t, app, admin), "tv": learnClientID(t, app, tv), "tablet": learnClientID(t, app, tablet),
			}
			app.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer)
			app.sendStateToClient(ids["admin"], server.ClientTypeAdmin)
			app.sendStateToClient(ids["tv"], server.ClientTypeTV)
			app.sendStateToClient(ids["tablet"], server.ClientTypeVPlayer)

			for who, conn := range map[string]*websocket.Conn{"tv": tv, "tablet": tablet} {
				frames := drainFrames192(conn, 400*time.Millisecond)
				if len(frames) == 0 {
					t.Fatalf("%s : aucun message reçu, le test ne prouverait rien", who)
				}
				for _, w := range append([]string{`"PICTO_WORDS"`}, poolLeakWords324...) {
					if joinHas192(frames, w) {
						t.Errorf("%s/%s : %q fuit (picto.md §10, picto-pool.md §3)", who, name, w)
					}
				}
				if who == "tv" {
					if !joinHas192(frames, `"PICTO_WORD_CATEGORY"`) || !joinHas192(frames, `"PICTO_POOL_AVAILABLE"`) {
						t.Errorf("tv/%s : l'indice de catégorie et le compte du pool sont publics", name)
					}
				}
			}
		})
	}
}
