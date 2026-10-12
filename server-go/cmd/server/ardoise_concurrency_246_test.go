package main

// #246 (v11.2) — tests PERMANENTS de non-régression « le texte ARDOISE vu par
// l'admin ne recule jamais » (reprise des harnais H1 et H6 du planner,
// _work/reports/harness-246/).
//
// 10 VJoueurs tapent en parallèle via de vraies WebSockets /ws/player (chacun
// se lie d'abord par PLAYER_CONNECT : la liaison connexion ↔ bumper est
// obligatoire depuis v11.2), des admins /ws/admin enregistrent tous les UPDATE,
// et on vérifie que le texte de chaque équipe vu par l'admin ne régresse jamais
// (jamais plus court, jamais vide après non vide) — les joueurs ne font qu'ajouter
// des caractères — et que l'état final admin = serveur = tablette.
//
// Durée : courte par défaut (H246_SECS pour allonger H6 ; ignoré avec -short).
// Tests de non-régression : ils passent déjà avant le DEV du lot A (la cause
// racine n'est pas dans la diffusion) et doivent continuer à passer avec la
// liaison obligatoire.

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type h246Obs struct {
	at    time.Time
	seq   int
	texts map[string]string // team -> text ("<absent>" if missing)
	phase string
}

func h246Setup(t *testing.T, nTeams int) (*App, string, []string, []string) {
	app := newBroadcast127TestApp(t)
	app.engine.OnStateChange = func(phase game.GamePhase) {
		app.ardoiseCoalescer.Flush()
		app.broadcastGameState(string(phase))
	}
	app.engine.SetPhase(game.PhaseEnroll)
	teams := map[string]*game.Team{}
	names := []string{}
	for i := 0; i < nTeams; i++ {
		n := fmt.Sprintf("Equipe%02d", i)
		teams[n] = &game.Team{Name: n}
		names = append(names, n)
	}
	app.engine.SetTeams(teams)
	ids := []string{}
	for i, n := range names {
		ids = append(ids, setupVirtualPlayer(t, app, fmt.Sprintf("Joueur%02d", i), n))
	}
	app.engine.SetPhase(game.PhaseStopped)
	app.engine.Ready("", &game.Question{ID: "q-ardoise", Type: game.QuestionTypeArdoise})
	app.engine.SetPhase(game.PhaseStarted)

	base := startAnimAllowlistTestServer(t, app)
	// real dispatch loop, as in main.go:586
	go func() {
		for msg := range app.wsHub.Incoming {
			app.handleWebMessage(msg)
		}
	}()
	return app, base, names, ids
}

func h246Dial(t *testing.T, base, path string) *websocket.Conn {
	wsURL := "ws" + strings.TrimPrefix(base, "http") + path
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return c
}

func h246Send(c *websocket.Conn, mu *sync.Mutex, action string, payload interface{}) error {
	msg, _ := protocol.NewMessage(action, payload)
	data, _ := msg.SerializeForWebSocket()
	mu.Lock()
	defer mu.Unlock()
	return c.WriteMessage(websocket.TextMessage, data)
}

// admin reader: records every message carrying GAME.ARDOISE_ANSWERS
func h246Admin(t *testing.T, c *websocket.Conn, names []string, stop <-chan struct{}) (*[]h246Obs, *sync.Mutex, *map[string]int) {
	obs := []h246Obs{}
	actions := map[string]int{}
	mu := &sync.Mutex{}
	go func() {
		seq := 0
		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			var env struct {
				Action string `json:"ACTION"`
				Msg    struct {
					Game map[string]json.RawMessage `json:"GAME"`
				} `json:"MSG"`
			}
			if json.Unmarshal(data, &env) != nil {
				continue
			}
			mu.Lock()
			actions[env.Action]++
			mu.Unlock()
			raw, ok := env.Msg.Game["ARDOISE_ANSWERS"]
			if !ok || env.Action != "UPDATE" {
				continue
			}
			var ans map[string]struct {
				Text string `json:"TEXT"`
			}
			json.Unmarshal(raw, &ans)
			var phase string
			json.Unmarshal(env.Msg.Game["PHASE"], &phase)
			o := h246Obs{at: time.Now(), seq: seq, texts: map[string]string{}, phase: phase}
			seq++
			for _, n := range names {
				if a, ok := ans[n]; ok {
					o.texts[n] = a.Text
				} else {
					o.texts[n] = "<absent>"
				}
			}
			mu.Lock()
			obs = append(obs, o)
			mu.Unlock()
		}
	}()
	return &obs, mu, &actions
}

func h246Check(t *testing.T, obs []h246Obs, names []string, final map[string]string) (regressions int) {
	last := map[string]string{}
	for _, o := range obs {
		for _, n := range names {
			cur := o.texts[n]
			if cur == "<absent>" {
				cur = ""
			}
			prev := last[n]
			if len(cur) < len(prev) {
				regressions++
				if regressions <= 20 {
					t.Logf("REGRESSION seq=%d team=%s prev=%q -> cur=%q (phase=%s)", o.seq, n, prev, cur, o.phase)
				}
			}
			last[n] = cur
		}
	}
	for _, n := range names {
		if last[n] != final[n] {
			t.Logf("FINAL MISMATCH team=%s admin=%q tablet=%q", n, last[n], final[n])
			regressions++
		}
	}
	return
}

// H1 — 10 tablettes tapent en parallèle, cadence réelle (1er caractère immédiat,
// puis un envoi par pause de saisie), + bruit de fond : chrono 1 s (UPDATE_TIMER)
func TestArdoiseConcurrency246_H1_TenTeamsTyping(t *testing.T) {
	if testing.Short() {
		t.Skip("H1 : test lent (≈ 6 s), exclu de la boucle rapide (-short)")
	}
	nTeams := 10
	app, base, names, ids := h246Setup(t, nTeams)
	stop := make(chan struct{})
	admin := h246Dial(t, base, "/ws/admin")
	defer admin.Close()
	obs, omu, actions := h246Admin(t, admin, names, stop)

	// background noise
	var bg sync.WaitGroup
	bgStop := make(chan struct{})
	bg.Add(2)
	go func() { // timer tick goroutine
		defer bg.Done()
		tk := time.NewTicker(1 * time.Second)
		defer tk.Stop()
		i := 30
		for {
			select {
			case <-bgStop:
				return
			case <-tk.C:
				app.broadcastTimerUpdate(i)
				i--
			}
		}
	}()
	go func() { // other full UPDATE sources from other goroutines
		defer bg.Done()
		for {
			select {
			case <-bgStop:
				return
			case <-time.After(time.Duration(h246NoiseMs()) * time.Millisecond):
				app.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeAnim)
			}
		}
	}()

	final := map[string]string{}
	var fmu sync.Mutex
	var wg sync.WaitGroup
	word := "LA REPONSE EST CHARLES DE GAULLE"
	for i := range names {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := h246Dial(t, base, "/ws/player")
			defer c.Close()
			var wmu sync.Mutex
			bound := h246ConnectPlayer(c, &wmu, fmt.Sprintf("Joueur%02d", i), ids[i])
			if !h246WaitBound(t, bound, names[i]) {
				return
			}
			r := rand.New(rand.NewSource(int64(i) + time.Now().UnixNano()))
			text := ""
			for k := 0; k < len(word); k++ {
				text += string(word[k])
				time.Sleep(time.Duration(30+r.Intn(250)) * time.Millisecond)
				if err := h246Send(c, &wmu, protocol.ActionArdoiseInput, protocol.ArdoiseInputPayload{ID: ids[i], Text: text}); err != nil {
					t.Errorf("send: %v", err)
					return
				}
			}
			fmu.Lock()
			final[names[i]] = text
			fmu.Unlock()
		}(i)
	}
	wg.Wait()
	time.Sleep(600 * time.Millisecond)
	close(bgStop)
	bg.Wait()
	close(stop)
	time.Sleep(300 * time.Millisecond)

	omu.Lock()
	defer omu.Unlock()
	t.Logf("admin actions: %v ; UPDATE avec ARDOISE_ANSWERS observés : %d", *actions, len(*obs))
	if r := h246Check(t, *obs, names, final); r > 0 {
		t.Errorf("%d régressions/incohérences observées côté admin", r)
	}
	if os.Getenv("H246_DUMP") != "" {
		for _, o := range *obs {
			t.Logf("%d %s %v", o.seq, o.phase, o.texts)
		}
	}
}

// H2 — idem + chaque tablette subit 0 à 2 reconnexions (nouvelle WS + PLAYER_CONNECT
// avec son ID), sans perte côté client (on renvoie le texte courant après reconnexion)
// h246ConnectPlayer lie la connexion à son bumper (PLAYER_CONNECT {NAME, ID}) et
// démarre la lecture de la connexion ; le canal retourné est fermé à la réception
// de PLAYER_CONNECTED (la tablette met ses actions en attente jusque-là, v11.2).
func h246ConnectPlayer(c *websocket.Conn, wmu *sync.Mutex, name, id string) <-chan struct{} {
	bound := make(chan struct{})
	go func() {
		once := false
		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			if !once && strings.Contains(string(data), `"PLAYER_CONNECTED"`) {
				once = true
				close(bound)
			}
		}
	}()
	h246Send(c, wmu, protocol.ActionPlayerConnect, protocol.PlayerConnectPayload{Name: name, ID: id})
	return bound
}

func h246WaitBound(t *testing.T, bound <-chan struct{}, who string) bool {
	select {
	case <-bound:
		return true
	case <-time.After(3 * time.Second):
		t.Errorf("%s : pas de PLAYER_CONNECTED en 3 s", who)
		return false
	}
}

func h246NoiseMs() int {
	if os.Getenv("H246_HEAVY") != "" {
		return 1 + rand.Intn(5)
	}
	return 50 + rand.Intn(300)
}

// déconnecté. Régression = texte plus court/vide reçu après un plus long.
// ---------------------------------------------------------------------------
type h246AdminView struct {
	name                       string
	mu                         sync.Mutex
	view                       map[string]string
	connected                  bool
	regress, behind, ticks, up int
	reconnects                 int
	lastTeams, lastBumpers     json.RawMessage
}

func (v *h246AdminView) run(t *testing.T, app *App, base string, names []string, slow time.Duration, stop <-chan struct{}, serverTexts func() map[string]string) {
	for {
		select {
		case <-stop:
			return
		default:
		}
		c := h246Dial(t, base, "/ws/admin")
		var wmu sync.Mutex
		h246Send(c, &wmu, protocol.ActionHello, map[string]interface{}{})
		v.mu.Lock()
		v.connected = true
		v.mu.Unlock()
		go func() { <-stop; c.Close() }()
		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				break
			}
			if slow > 0 {
				time.Sleep(slow)
			}
			var env struct {
				Action string `json:"ACTION"`
				Msg    struct {
					Game    map[string]json.RawMessage `json:"GAME"`
					Teams   json.RawMessage            `json:"teams"`
					Bumpers json.RawMessage            `json:"bumpers"`
				} `json:"MSG"`
			}
			if json.Unmarshal(data, &env) != nil {
				continue
			}
			v.mu.Lock()
			switch env.Action {
			case "UPDATE":
				v.up++
				if len(env.Msg.Teams) > 0 {
					v.lastTeams, v.lastBumpers = env.Msg.Teams, env.Msg.Bumpers
				}
				if raw, ok := env.Msg.Game["ARDOISE_ANSWERS"]; ok {
					var ans map[string]struct {
						Text string `json:"TEXT"`
					}
					json.Unmarshal(raw, &ans)
					for _, n := range names {
						cur := ans[n].Text
						if len(cur) < len(v.view[n]) {
							v.regress++
							if v.regress <= 10 {
								t.Logf("[%s] REGRESSION team=%s %q -> %q", v.name, n, v.view[n], cur)
							}
						}
						v.view[n] = cur
					}
				}
			case "UPDATE_TIMER":
				v.ticks++
				st := serverTexts()
				go func(st map[string]string) {
					time.Sleep(400 * time.Millisecond)
					v.mu.Lock()
					defer v.mu.Unlock()
					if !v.connected || slow > 0 {
						return // admin lent : le retard est attendu, seules les régressions comptent
					}
					for _, n := range names {
						if !strings.HasPrefix(v.view[n], st[n]) {
							v.behind++
							if v.behind <= 10 {
								t.Logf("[%s] BEHIND team=%s server@tick=%q admin=%q", v.name, n, st[n], v.view[n])
							}
						}
					}
				}(st)
			}
			v.mu.Unlock()
		}
		v.mu.Lock()
		v.connected = false
		v.mu.Unlock()
		c.Close()
		select {
		case <-stop:
			return
		case <-time.After(time.Duration(3500+rand.Intn(3000)) * time.Millisecond):
		}
		v.mu.Lock()
		v.reconnects++
		v.mu.Unlock()
	}
}

func TestArdoiseConcurrency246_H6_TwoAdminsTwoTV_Critical(t *testing.T) {
	if testing.Short() {
		t.Skip("H6 : test lent (≈ 25 s), exclu de la boucle rapide (-short)")
	}
	dur := 8
	if s := os.Getenv("H246_SECS"); s != "" {
		fmt.Sscanf(s, "%d", &dur)
	}
	slowB := 150 * time.Millisecond
	if s := os.Getenv("H246_SLOWB_MS"); s != "" {
		var ms int
		fmt.Sscanf(s, "%d", &ms)
		slowB = time.Duration(ms) * time.Millisecond
	}
	nTeams := 10
	app, base, names, ids := h246Setup(t, nTeams)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, nil)
	app.engine.OnTimerTick = func(ct int) { app.broadcastTimerUpdate(ct) }
	app.engine.OnCountdownTick = func(ct int) { app.broadcastCountdownUpdate(ct) }
	app.engine.SetPhase(game.PhaseStopped)
	app.engine.Ready("", &game.Question{ID: "q-ardoise", Type: game.QuestionTypeArdoise})
	app.engine.SetPhase(game.PhaseReady)
	app.engine.Start(dur + 15)
	deadline := time.Now().Add(5 * time.Second)
	for app.engine.GetPhase() != game.PhaseStarted && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	serverTexts := func() map[string]string {
		var gd struct {
			Game struct {
				A map[string]struct {
					Text string `json:"TEXT"`
				} `json:"ARDOISE_ANSWERS"`
			} `json:"GAME"`
		}
		json.Unmarshal(app.engine.GetGameJSON(), &gd)
		out := map[string]string{}
		for k, v := range gd.Game.A {
			out[k] = v.Text
		}
		return out
	}
	stop := make(chan struct{})
	A := &h246AdminView{name: "adminA", view: map[string]string{}}
	B := &h246AdminView{name: "adminB-lent", view: map[string]string{}}
	go A.run(t, app, base, names, 0, stop, serverTexts)
	go B.run(t, app, base, names, slowB, stop, serverTexts)
	// 2 TV : une normale, une lente
	for i, slow := range []time.Duration{0, 300 * time.Millisecond} {
		go func(i int, slow time.Duration) {
			c := h246Dial(t, base, "/tv")
			var wmu sync.Mutex
			h246Send(c, &wmu, protocol.ActionHello, map[string]interface{}{})
			go func() { <-stop; c.Close() }()
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
				if slow > 0 {
					time.Sleep(slow)
				}
			}
		}(i, slow)
	}
	time.Sleep(300 * time.Millisecond)

	// admin A emitter on its own connection (actions realistic)
	adminSendConn := h246Dial(t, base, "/ws/admin")
	var amu sync.Mutex
	stopAdmin := make(chan struct{})
	var awg sync.WaitGroup
	adminSends := 0
	awg.Add(1)
	go func() {
		defer awg.Done()
		go func() {
			for {
				if _, _, err := adminSendConn.ReadMessage(); err != nil {
					return
				}
			}
		}()
		i := 0
		for {
			select {
			case <-stopAdmin:
				return
			case <-time.After(time.Duration(300+rand.Intn(1700)) * time.Millisecond):
			}
			i++
			A.mu.Lock()
			tm, bm := A.lastTeams, A.lastBumpers
			A.mu.Unlock()
			switch i % 3 {
			case 0:
				if tm != nil {
					h246Send(adminSendConn, &amu, protocol.ActionUpdate, map[string]json.RawMessage{"teams": tm, "bumpers": bm})
				}
			case 1:
				h246Send(adminSendConn, &amu, protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: names[rand.Intn(len(names))], Points: 0})
			case 2:
				h246Send(adminSendConn, &amu, protocol.ActionSetCreditPoints, protocol.CreditPointsPayload{Points: 1 + rand.Intn(5)})
			}
			adminSends++
		}
	}()

	final := map[string]string{}
	var fmu sync.Mutex
	var wg sync.WaitGroup
	end := time.Now().Add(time.Duration(dur) * time.Second)
	for i := range names {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := h246Dial(t, base, "/ws/player")
			defer c.Close()
			var wmu sync.Mutex
			bound := h246ConnectPlayer(c, &wmu, fmt.Sprintf("Joueur%02d", i), ids[i])
			if !h246WaitBound(t, bound, names[i]) {
				return
			}
			r := rand.New(rand.NewSource(int64(i)*131 + time.Now().UnixNano()))
			text := ""
			for time.Now().Before(end) {
				text += string(rune('A' + r.Intn(26)))
				time.Sleep(time.Duration(80+r.Intn(320)) * time.Millisecond)
				h246Send(c, &wmu, protocol.ActionArdoiseInput, protocol.ArdoiseInputPayload{ID: ids[i], Text: text})
			}
			fmu.Lock()
			final[names[i]] = text
			fmu.Unlock()
		}(i)
	}
	wg.Wait()
	close(stopAdmin)
	awg.Wait()
	// laisser l'admin lent se vider / se reconnecter
	time.Sleep(8 * time.Second)
	st := serverTexts()
	fail := 0
	for _, v := range []*h246AdminView{A, B} {
		v.mu.Lock()
		mism := 0
		for _, n := range names {
			if v.view[n] != st[n] || st[n] != final[n] {
				mism++
				if mism <= 5 {
					t.Logf("[%s] FINAL team=%s tablet=%d server=%d admin=%d", v.name, n, len(final[n]), len(st[n]), len(v.view[n]))
				}
			}
		}
		t.Logf("[%s] ticks=%d updates=%d reconnects=%d regressions=%d behind=%d finalMismatch=%d", v.name, v.ticks, v.up, v.reconnects, v.regress, v.behind, mism)
		fail += v.regress + v.behind + mism
		v.mu.Unlock()
	}
	t.Logf("phase=%s adminSends=%d", app.engine.GetPhase(), adminSends)
	close(stop)
	adminSendConn.Close()
	if fail > 0 {
		t.Errorf("symptôme observé (total=%d)", fail)
	}
}

// Scénario reproduit (planner-20261010-203000) côté serveur : effacement ("") puis flux de ressaisies à
// cadence 200 ms (ce que la tablette doit envoyer pendant une frappe continue) → l'état suivi par l'admin
// reste synchrone EN DIRECT, jamais vide pendant la ressaisie, et l'admin reçoit la dernière valeur avant STOP.
// Vrai serveur (hub WebSocket réel, dispatch réel, coalesceur réel), `-race`.
func TestArdoiseConcurrency246_ClearThenContinuousRetypeIsLiveForTheAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("cadence réelle 200 ms (≈ 3 s), exclu de la boucle rapide (-short)")
	}
	app, base, names, ids := h246Setup(t, 2)
	stop := make(chan struct{})
	admin := h246Dial(t, base, "/ws/admin")
	defer admin.Close()
	obs, omu, _ := h246Admin(t, admin, names, stop)

	c := h246Dial(t, base, "/ws/player")
	defer c.Close()
	var wmu sync.Mutex
	bound := h246ConnectPlayer(c, &wmu, "Joueur00", ids[0])
	if !h246WaitBound(t, bound, names[0]) {
		return
	}
	// « PARIS » posément, effacement, puis ressaisie continue toutes les 200 ms
	send := func(text, reason string) {
		_ = h246Send(c, &wmu, protocol.ActionArdoiseInput, map[string]interface{}{"ID": ids[0], "TEXT": text, "REASON": reason})
	}
	send("PARIS", "key")
	time.Sleep(300 * time.Millisecond)
	send("", "clear")
	time.Sleep(300 * time.Millisecond)
	retype := "LYONNAISE"
	for i := 1; i <= len(retype); i++ {
		send(retype[:i], "key")
		time.Sleep(200 * time.Millisecond)
		// à chaque pas, le serveur détient déjà le texte courant (suivi en direct)
		if got := h246ServerText(app, names[0]); got != retype[:i] {
			t.Fatalf("pas %d : texte serveur = %q, attendu %q (l'admin doit suivre en direct)", i, got, retype[:i])
		}
	}
	app.engine.Stop()
	time.Sleep(400 * time.Millisecond)
	close(stop)

	omu.Lock()
	defer omu.Unlock()
	seenRetype, emptyAfter := false, 0
	for _, o := range *obs {
		if len(o.texts[names[0]]) >= 1 && o.texts[names[0]] != "<absent>" && o.texts[names[0]] != "PARIS" {
			seenRetype = true
		}
		if seenRetype && (o.texts[names[0]] == "" || o.texts[names[0]] == "<absent>") {
			emptyAfter++
		}
	}
	if !seenRetype {
		t.Fatalf("l'admin n'a jamais vu la ressaisie pendant la question (%d UPDATE)", len(*obs))
	}
	if emptyAfter > 0 {
		t.Errorf("l'admin a revu un texte vide %d fois pendant la ressaisie", emptyAfter)
	}
	if last := (*obs)[len(*obs)-1].texts[names[0]]; last != retype {
		t.Errorf("dernier texte vu par l'admin = %q, attendu %q", last, retype)
	}
}

func h246ServerText(app *App, team string) string {
	var gd struct {
		Game struct {
			A map[string]struct {
				Text string `json:"TEXT"`
			} `json:"ARDOISE_ANSWERS"`
		} `json:"GAME"`
	}
	_ = json.Unmarshal(app.engine.GetGameJSON(), &gd)
	return gd.Game.A[team].Text
}
