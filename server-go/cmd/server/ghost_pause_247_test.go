package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"

)

// #247 — PAUSE fantôme. Un PAUSE ne doit être diffusé (admin, TV, VPlayer,
// anim) que pour un appui ACCEPTÉ par le moteur, et exactement UNE fois.
// Le câblage de production (setupCallbacks, donc le callback OnBuzzerPress) est
// utilisé tel quel : les tests ne dépendent pas de l'endroit où le correctif
// place la diffusion. Attentes par événement : un message témoin diffusé après
// l'action ferme la fenêtre d'observation (l'ordre est conservé par connexion).

const sentinelPrefix247 = "TEST_SENTINEL_247_"

// observers247 : une goroutine de lecture par connexion (gorilla/websocket
// rend la connexion inutilisable après un timeout de lecture : on ne lit donc
// jamais avec échéance, on consomme un canal).
type observers247 struct {
	msgs map[string]chan string
	seq  int
}

func newApp247(t *testing.T) (*App, *observers247) {
	t.Helper()
	app := newBroadcast127TestApp(t)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	app.setupCallbacks() // câblage réel (OnBuzzerPress inclus)

	go app.wsHub.Run()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := server.ClientTypeVPlayer
		switch {
		case strings.HasSuffix(r.URL.Path, "/admin"):
			ct = server.ClientTypeAdmin
		case strings.HasSuffix(r.URL.Path, "/tv"):
			ct = server.ClientTypeTV
		case strings.HasSuffix(r.URL.Path, "/anim"):
			ct = server.ClientTypeAnim
		}
		app.wsHub.HandleConnectionWithType(w, r, ct)
	}))
	t.Cleanup(srv.Close)

	obs := &observers247{msgs: map[string]chan string{}}
	for _, p := range []string{"admin", "tv", "player", "anim"} {
		conn := dialWS(t, srv.URL, "/ws/"+p)
		ch := make(chan string, 1024)
		obs.msgs[p] = ch
		go func() {
			defer close(ch)
			for {
				_, data, err := conn.ReadMessage()
				if err != nil {
					return
				}
				var env struct {
					Action string `json:"ACTION"`
				}
				if json.Unmarshal(data, &env) == nil {
					ch <- env.Action
				}
			}
		}()
	}
	// Les connexions doivent être enregistrées par le hub avant l'action testée.
	obs.observe(t, app)
	return app, obs
}

// observe diffuse un message témoin et retourne, par client, les actions reçues
// avant lui. Le témoin est renvoyé jusqu'à réception (le hub enregistre les
// connexions de façon asynchrone) ; échéance globale 10 s.
func (o *observers247) observe(t *testing.T, app *App) map[string][]string {
	t.Helper()
	o.seq++
	want := sentinelPrefix247 + strconv.Itoa(o.seq)
	got := map[string][]string{}
	deadline := time.After(10 * time.Second)
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	send := func() {
		app.broadcast(want, json.RawMessage(`{}`), true,
			server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer, server.ClientTypeAnim)
	}
	send()
	pending := map[string]bool{}
	for name := range o.msgs {
		pending[name] = true
	}
	for len(pending) > 0 {
		progressed := false
		for name := range pending {
			select {
			case action, ok := <-o.msgs[name]:
				progressed = true
				if !ok {
					t.Fatalf("%s : connexion fermée avant le témoin", name)
				}
				if action == want {
					delete(pending, name)
				} else if !strings.HasPrefix(action, sentinelPrefix247) {
					got[name] = append(got[name], action)
				}
			default:
			}
		}
		if progressed {
			continue
		}
		select {
		case <-tick.C:
			send()
		case <-deadline:
			t.Fatalf("témoin non reçu avant l'échéance par : %v", pending)
		}
	}
	return got
}

// pauses247 retourne, par client, le nombre de PAUSE reçus depuis la dernière observation.
func pauses247(t *testing.T, app *App, obs *observers247) map[string]int {
	t.Helper()
	out := map[string]int{}
	for name, actions := range obs.observe(t, app) {
		out[name] = countActions(actions, protocol.ActionPause)
	}
	return out
}

func expectPauses247(t *testing.T, got map[string]int, want int, ctx string) {
	t.Helper()
	for _, name := range []string{"admin", "tv", "player", "anim"} {
		if got[name] != want {
			t.Errorf("%s : %s doit recevoir %d PAUSE, a reçu %d (tous : %v)", ctx, name, want, got[name], got)
		}
	}
}

func physicalBumpers247(app *App, extra map[string]*game.Bumper) {
	b := map[string]*game.Bumper{
		"AA:BB:CC:01": {Name: "Buzzer1", Team: "TeamA", Connected: true},
		"AA:BB:CC:02": {Name: "Buzzer2", Team: "TeamA", Connected: true},
		"AA:BB:CC:03": {Name: "Buzzer3", Team: "TeamB", Connected: true},
	}
	for k, v := range extra {
		b[k] = v
	}
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}, "TeamB": {Name: "TeamB"}})
	app.engine.SetBumpers(b)
}

func startQuestion247(app *App, q *game.Question) {
	app.engine.SetPhase(game.PhaseStopped)
	app.engine.Ready(q.ID, q)
	app.engine.SetPhase(game.PhaseStarted)
}

func speedy247() *game.Question {
	return &game.Question{ID: "q-speedy", Question: "?", Type: game.QuestionTypeSpeedy}
}

func button247(t *testing.T, app *App, id string) {
	t.Helper()
	app.handleButton(id, buttonMsgFor(t), time.Now().UnixMicro())
}

// --- Appel refusé : aucun PAUSE ---------------------------------------------

func TestGhostPause247_PhysicalPressInArdoise_NoPauseNoStateChange(t *testing.T) {
	app, obs := newApp247(t)
	physicalBumpers247(app, nil)
	startQuestion247(app, &game.Question{ID: "q-ardoise", Question: "?", Type: game.QuestionTypeArdoise})

	button247(t, app, "AA:BB:CC:01")

	expectPauses247(t, pauses247(t, app, obs), 0, "appui physique en ARDOISE")
	if b := app.engine.GetBumper("AA:BB:CC:01"); b.Time != 0 {
		t.Errorf("ARDOISE : l'appui ne doit pas être enregistré, Time=%d", b.Time)
	}
	if app.engine.GetPhase() != game.PhaseStarted {
		t.Errorf("ARDOISE : la phase doit rester STARTED, got %s", app.engine.GetPhase())
	}
}

func TestGhostPause247_PhysicalPressInMemory_NoPause(t *testing.T) {
	app, obs := newApp247(t)
	physicalBumpers247(app, nil)
	startQuestion247(app, &game.Question{ID: "q-memory", Question: "?", Type: game.QuestionTypeMemory})

	button247(t, app, "AA:BB:CC:01")

	expectPauses247(t, pauses247(t, app, obs), 0, "appui physique en MEMORY")
}

func TestGhostPause247_SimulatedButtonInMemory_NoPause(t *testing.T) {
	app, obs := newApp247(t)
	physicalBumpers247(app, nil)
	startQuestion247(app, &game.Question{ID: "q-memory", Question: "?", Type: game.QuestionTypeMemory})

	app.handleSimulatedButton(simulatedButtonMsg(t, "AA:BB:CC:01", "A"))

	expectPauses247(t, pauses247(t, app, obs), 0, "BUTTON simulé/VJoueur en MEMORY")
}

func TestGhostPause247_SecondPressSameBumper_NoExtraPause(t *testing.T) {
	app, obs := newApp247(t)
	physicalBumpers247(app, nil)
	startQuestion247(app, speedy247())

	button247(t, app, "AA:BB:CC:01")
	expectPauses247(t, pauses247(t, app, obs), 1, "premier appui accepté")

	button247(t, app, "AA:BB:CC:01") // bumper déjà appuyé
	expectPauses247(t, pauses247(t, app, obs), 0, "second appui du même bumper")
}

func TestGhostPause247_PressByTeammateAlreadyBuzzed_NoExtraPause(t *testing.T) {
	app, obs := newApp247(t)
	physicalBumpers247(app, nil)
	startQuestion247(app, speedy247())

	button247(t, app, "AA:BB:CC:01")
	expectPauses247(t, pauses247(t, app, obs), 1, "premier appui accepté")

	button247(t, app, "AA:BB:CC:02") // même équipe, déjà buzzée
	expectPauses247(t, pauses247(t, app, obs), 0, "appui d'un coéquipier (équipe déjà buzzée)")
}

func TestGhostPause247_QCM_PhysicalBuzzerOfTeamWithVPlayer_NoPause(t *testing.T) {
	app, obs := newApp247(t)
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	vp := setupVirtualPlayer(t, app, "VP", "TeamA")
	app.engine.SetBumpers(map[string]*game.Bumper{
		vp:            app.engine.GetBumper(vp),
		"AA:BB:CC:01": {Name: "Buzzer1", Team: "TeamA", Connected: true},
	})
	startQuestion247(app, &game.Question{
		ID: "q-qcm", Type: game.QuestionTypeQCM,
		TypedContent: game.TypedContent{QCMAnswers: &game.QCMAnswers{Red: "A", Green: "B", Yellow: "C", Blue: "D"}},
	})

	button247(t, app, "AA:BB:CC:01")

	expectPauses247(t, pauses247(t, app, obs), 0, "buzzer physique d'une équipe avec VPlayer en QCM")
}

func TestGhostPause247_QCM_RepeatedIdenticalVPlayerAnswer_OnlyOnePause(t *testing.T) {
	app, obs := newApp247(t)
	ids := setupQCMStartedGame(t, app, 2)

	app.handleVPlayerQCMAnswer(ids[0], qcmAnswerMsg(t, ids[0], "RED"))
	expectPauses247(t, pauses247(t, app, obs), 1, "première réponse QCM")

	app.handleVPlayerQCMAnswer(ids[0], qcmAnswerMsg(t, ids[0], "RED")) // renvoi identique
	expectPauses247(t, pauses247(t, app, obs), 0, "renvoi du même VPLAYER_QCM_ANSWER")
}

// --- Appui accepté : exactement un PAUSE ------------------------------------

func TestGhostPause247_AcceptedPhysicalPress_ExactlyOnePause(t *testing.T) {
	app, obs := newApp247(t)
	physicalBumpers247(app, nil)
	startQuestion247(app, speedy247())

	button247(t, app, "AA:BB:CC:01")

	expectPauses247(t, pauses247(t, app, obs), 1, "appui physique accepté (SPEEDY)")
	if b := app.engine.GetBumper("AA:BB:CC:01"); b.Time == 0 {
		t.Error("l'appui accepté doit être enregistré (Time > 0)")
	}
}

func TestGhostPause247_AcceptedSimulatedButton_ExactlyOnePause(t *testing.T) {
	app, obs := newApp247(t)
	physicalBumpers247(app, nil)
	startQuestion247(app, speedy247())

	app.handleSimulatedButton(simulatedButtonMsg(t, "AA:BB:CC:01", "A"))

	expectPauses247(t, pauses247(t, app, obs), 1, "BUTTON simulé/VJoueur accepté (SPEEDY)")
}

func TestGhostPause247_AcceptedVPlayerQCMAnswer_ExactlyOnePause(t *testing.T) {
	app, obs := newApp247(t)
	ids := setupQCMStartedGame(t, app, 2)

	app.handleVPlayerQCMAnswer(ids[0], qcmAnswerMsg(t, ids[0], "GREEN"))

	expectPauses247(t, pauses247(t, app, obs), 1, "VPLAYER_QCM_ANSWER accepté")
}
