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
	"buzzcontrol/internal/server"

	"github.com/gorilla/websocket"
)

// #247/#248 — buzz SPEEDY d'un VJoueur REFUSÉ par le moteur (son équipe a déjà
// buzzé). Contrat attendu : ni PAUSE ni message vers les autres clients, MAIS le
// VJoueur qui a renvoyé reçoit exactement UN UPDATE ciblé portant l'état de son
// équipe (TIME > 0). Sans lui, la tablette qui renvoie son buzz (accusé perdu)
// reste en « Envoi… » puis affiche un faux « non reçu » : l'état « équipe déjà
// servie » se lit dans teams[équipe].TIME d'un UPDATE.
// Comportement observable (messages reçus par client), pas le mécanisme.
// Pas de setupCallbacks : learnClientID lit wsHub.Incoming, que le consommateur
// de production concurrencerait ; seul le handler est exercé.

const sentinelPrefix247b = "TEST_SENTINEL_247B_"

type frame247 struct {
	Action string
	Raw    []byte
}

type room247 struct {
	app  *App
	ids  map[string]string // p1, p2 (équipe A), p3 (équipe B), p4 (non-VJoueur : buzzer physique, équipe A)
	msgs map[string]chan frame247
	seq  int
}

func (r *room247) start(name string, conn *websocket.Conn) {
	ch := make(chan frame247, 1024)
	r.msgs[name] = ch
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
				ch <- frame247{Action: env.Action, Raw: data}
			}
		}
	}()
}

func newRoom247(t *testing.T) *room247 {
	t.Helper()
	app := newBroadcast127TestApp(t)
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

	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}, "TeamB": {Name: "TeamB"}})
	ids := map[string]string{
		"p1": setupVirtualPlayer(t, app, "P1", "TeamA"),
		"p2": setupVirtualPlayer(t, app, "P2", "TeamA"),
		"p3": setupVirtualPlayer(t, app, "P3", "TeamB"),
	}
	// Un buzzer physique (non VJoueur) de l'équipe A, pour le cas « pas d'écho nécessaire ».
	all := map[string]*game.Bumper{"AA:BB:CC:09": {Name: "Physique", Team: "TeamA", Connected: true}}
	for _, id := range ids {
		all[id] = app.engine.GetBumper(id)
	}
	app.engine.SetBumpers(all)
	ids["phys"] = "AA:BB:CC:09"
	app.engine.SetPhase(game.PhaseStopped)
	app.engine.Ready("q-speedy", &game.Question{ID: "q-speedy", Question: "?", Type: game.QuestionTypeSpeedy})
	app.engine.SetPhase(game.PhaseStarted)

	r := &room247{app: app, ids: ids, msgs: map[string]chan frame247{}}
	for _, name := range []string{"p1", "p2", "p3"} {
		conn := dialWS(t, srv.URL, "/ws/player")
		cid := learnClientID(t, app, conn)
		app.wsHub.SetClientPlayerID(cid, ids[name])
		r.start(name, conn)
	}
	for _, name := range []string{"admin", "tv", "anim"} {
		r.start(name, dialWS(t, srv.URL, "/ws/"+name))
	}
	r.observe(t) // les connexions doivent être enregistrées avant l'action testée
	return r
}

// observe diffuse un message témoin à tous les types de client et retourne, par
// client, les trames reçues avant lui (échéance 10 s, renvoi du témoin tant que
// l'enregistrement asynchrone des connexions n'est pas fini).
func (r *room247) observe(t *testing.T) map[string][]frame247 {
	t.Helper()
	r.seq++
	want := sentinelPrefix247b + strconv.Itoa(r.seq)
	got := map[string][]frame247{}
	deadline := time.After(10 * time.Second)
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	send := func() {
		r.app.broadcast(want, json.RawMessage(`{}`), true,
			server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer, server.ClientTypeAnim)
	}
	send()
	pending := map[string]bool{}
	for name := range r.msgs {
		pending[name] = true
	}
	for len(pending) > 0 {
		progressed := false
		for name := range pending {
			select {
			case f, ok := <-r.msgs[name]:
				progressed = true
				if !ok {
					t.Fatalf("%s : connexion fermée avant le témoin", name)
				}
				if f.Action == want {
					delete(pending, name)
				} else if !strings.HasPrefix(f.Action, sentinelPrefix247b) {
					got[name] = append(got[name], f)
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

func framesOf247(fs []frame247, action string) []frame247 {
	var out []frame247
	for _, f := range fs {
		if f.Action == action {
			out = append(out, f)
		}
	}
	return out
}

// teamTime247 lit teams[team].TIME dans une trame UPDATE.
func teamTime247(t *testing.T, f frame247, team string) float64 {
	t.Helper()
	var env struct {
		Msg struct {
			Teams map[string]struct {
				Time float64 `json:"TIME"`
			} `json:"teams"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal(f.Raw, &env); err != nil {
		t.Fatalf("UPDATE illisible : %v", err)
	}
	tm, ok := env.Msg.Teams[team]
	if !ok {
		t.Fatalf("UPDATE sans teams[%s] : %s", team, string(f.Raw))
	}
	return tm.Time
}

func (r *room247) buzz(t *testing.T, who string) {
	t.Helper()
	r.app.handleSimulatedButton(simulatedButtonMsg(t, r.ids[who], "A"))
}

func TestRefusedSpeedyBuzz247_VPlayerOfServedTeam_GetsOneTargetedUpdateAndNoPause(t *testing.T) {
	r := newRoom247(t)
	r.buzz(t, "p1") // accepté : l'équipe A est servie
	r.observe(t)    // on repart d'une fenêtre vide

	r.buzz(t, "p2") // même équipe : refusé par le moteur
	got := r.observe(t)

	if n := len(framesOf247(got["p2"], "UPDATE")); n != 1 {
		t.Fatalf("p2 (VJoueur dont l'équipe a déjà buzzé) doit recevoir exactement 1 UPDATE ciblé, a reçu %d (trames : %v)", n, got["p2"])
	}
	if tm := teamTime247(t, framesOf247(got["p2"], "UPDATE")[0], "TeamA"); tm <= 0 {
		t.Errorf("l'UPDATE ciblé doit porter le TIME > 0 de son équipe, a %v", tm)
	}
	for name, fs := range got {
		if len(framesOf247(fs, "PAUSE")) != 0 {
			t.Errorf("%s ne doit recevoir aucun PAUSE pour un buzz refusé : %v", name, fs)
		}
		if name != "p2" && len(fs) != 0 {
			t.Errorf("%s ne doit recevoir AUCUN message (buzz refusé de p2) : %v", name, fs)
		}
	}
}

// Garde de non-régression : un buzz ACCEPTÉ garde son comportement (un PAUSE vers
// admin/tv/anim et les VJoueurs, un UPDATE ciblé pour le buzzeur).
func TestAcceptedSpeedyBuzz247_StillOnePauseForEveryone(t *testing.T) {
	r := newRoom247(t)
	r.buzz(t, "p1")
	got := r.observe(t)
	for _, name := range []string{"admin", "tv", "anim", "p1", "p2", "p3"} {
		if n := len(framesOf247(got[name], "PAUSE")); n != 1 {
			t.Errorf("%s doit recevoir exactement 1 PAUSE sur un buzz accepté, a reçu %d", name, n)
		}
	}
}

// Appui refusé d'un bouton NON VJoueur (buzzer physique de l'équipe déjà servie,
// via le chemin simulé) : aucun écho nécessaire, aucun PAUSE, aucun message.
func TestRefusedSpeedyBuzz247_NonVPlayer_NoEchoNoPause(t *testing.T) {
	r := newRoom247(t)
	r.buzz(t, "p1")
	r.observe(t)

	r.buzz(t, "phys") // buzzer physique de l'équipe A, déjà servie : refusé
	got := r.observe(t)
	for name, fs := range got {
		if len(fs) != 0 {
			t.Errorf("%s ne doit recevoir aucun message pour le buzz refusé d'un non-VJoueur : %v", name, fs)
		}
	}
}
