package main

// #253 (v11.2, lot V2-4) — verrou serveur LAUNCH_LOCK pendant la popup « Lancer la partie ».
// Contrats : contracts/websocket-actions.md §LAUNCH_LOCK (+ LAUNCH_LOCK_REFUSED / ACTION_REFUSED),
// contracts/game-state.md §LAUNCH_LOCK, contracts/logging.md §3.2 sexies.
// Scénarios du plan (_work/reports/planner-20261011-113000.md §#253) : 1 à 7 (l'expiration par horloge injectée
// est dans launch_lock_expiry_253_test.go). Statut `regression` ; `critical` : refus pour TOUS les postes (2),
// déconnexion du propriétaire (6).
//
// Les actions sont désignées par leur nom de protocole (littéraux) : le test ne dépend d'aucun symbole à créer.

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type env253 struct {
	*env246
	admin1, admin2, anim, tv string
	a1, a2, an, tvc          *websocket.Conn
	paulConn                 *websocket.Conn
	paulCID                  string
}

// newEnv253 : deux équipes, un VJoueur lié par équipe, question ARDOISE en cours (STARTED) ou révélée (revealed),
// deux postes admin, une tablette anim, un écran TV.
func newEnv253(t *testing.T, revealed bool) *env253 { return buildEnv253(t, revealed, false) }

// helloEnv253 : comme newEnv253 mais avec un serveur HTTP (sendStateToClient — rejeu d'état au HELLO — lit l'image de
// question par défaut via lui). Il doit exister AVANT le démarrage du serveur de test (NewHTTPServer écrit un global).
func helloEnv253(t *testing.T) *env253 { return buildEnv253(t, true, true) }

func buildEnv253(t *testing.T, revealed, withHTTP bool) *env253 {
	t.Helper()
	app := newBroadcast127TestApp(t)
	serverSetGlobalLogger249(t, app)
	if withHTTP {
		app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	}
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{
		"Les Bleus":  {Name: "Les Bleus", Color: []int{0, 0, 255}},
		"Les Rouges": {Name: "Les Rouges", Color: []int{255, 0, 0}},
	})
	paul := setupVirtualPlayer(t, app, "Paul", "Les Bleus")
	mamie := setupVirtualPlayer(t, app, "Mamie", "Les Rouges")
	app.engine.SetPhase(game.PhaseStopped)
	app.engine.Ready("", &game.Question{ID: "q-ardoise", Type: game.QuestionTypeArdoise})
	app.engine.SetPhase(game.PhaseStarted)
	e := &env246{app: app, base: startAnimAllowlistTestServer(t, app), paul: paul, mamie: mamie}

	// câblé par setupCallbacks en production (setupCallbacks consomme hub.Incoming : inutilisable ici) :
	// libère le verrou du propriétaire à la fermeture de sa connexion.
	app.wsHub.SetOnClientClosed(app.onClientClosed)
	a1, id1 := e.dial(t, "/ws/admin")
	a2, id2 := e.dial(t, "/ws/admin")
	an, ida := e.dial(t, "/ws/anim")
	tv, idt := e.dial(t, "/ws/tv")
	pc, pid := e.dial(t, "/ws/player")
	for _, c := range []*websocket.Conn{a1, a2, an, tv, pc} {
		startTap253(c)
	}
	e.connect(t, pid, "Paul", e.paul)
	if revealed {
		e.app.engine.Stop()
		e.app.engine.Reveal()
	}
	return &env253{env246: e, admin1: id1, admin2: id2, anim: ida, tv: idt, a1: a1, a2: a2, an: an, tvc: tv, paulConn: pc, paulCID: pid}
}

type lockState253 struct {
	Present   bool
	Active    bool
	Owner     string
	Since     int64
	ExpiresAt int64
}

func (e *env253) lock() lockState253 {
	var raw struct {
		Game struct {
			Lock *struct {
				Active    bool   `json:"ACTIVE"`
				Owner     string `json:"OWNER"`
				Since     int64  `json:"SINCE"`
				ExpiresAt int64  `json:"EXPIRES_AT"`
			} `json:"LAUNCH_LOCK"`
		} `json:"GAME"`
	}
	_ = json.Unmarshal(e.app.engine.GetGameJSON(), &raw)
	if raw.Game.Lock == nil {
		return lockState253{}
	}
	l := raw.Game.Lock
	return lockState253{Present: true, Active: l.Active, Owner: l.Owner, Since: l.Since, ExpiresAt: l.ExpiresAt}
}

func (e *env253) openLock(t *testing.T, id string) {
	t.Helper()
	e.send(t, id, "admin", "LAUNCH_LOCK", map[string]interface{}{"STATE": "OPEN"})
}

func (e *env253) closeLock(t *testing.T, id string) {
	t.Helper()
	e.send(t, id, "admin", "LAUNCH_LOCK", map[string]interface{}{"STATE": "CLOSE"})
}

// waitLock attend (≤ 2 s) que la condition sur le verrou soit vraie.
func (e *env253) waitLock(cond func(lockState253) bool) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond(e.lock()) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond(e.lock())
}

// gameFingerprint : tout ce que les actions suspendues sont susceptibles de modifier.
func (e *env253) gameFingerprint() string {
	st := e.state249()
	b, _ := json.Marshal(map[string]interface{}{"teams": st.Teams, "bumpers": st.Bumpers, "phase": e.app.engine.GetPhase(), "status": e.app.engine.GetQuestionStatus("q-ardoise")})
	return string(b)
}

func (e *env253) teamScore(team string) int {
	var raw struct {
		Teams map[string]struct {
			Score int `json:"SCORE"`
		} `json:"teams"`
	}
	_ = json.Unmarshal(e.app.engine.GetGameJSON(), &raw)
	return raw.Teams[team].Score
}

// tap253 lit une connexion en continu dans une goroutine (une lecture expirée rend la connexion gorilla inutilisable :
// on ne lit donc jamais avec échéance). Les trames sont consommées par curseur.
type tap253 struct {
	mu     sync.Mutex
	frames []string
	cursor int
}

var taps253 sync.Map // *websocket.Conn → *tap253

func startTap253(conn *websocket.Conn) {
	tp := &tap253{}
	taps253.Store(conn, tp)
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			tp.mu.Lock()
			tp.frames = append(tp.frames, string(data))
			tp.mu.Unlock()
		}
	}()
}

func tapOf253(conn *websocket.Conn) *tap253 {
	v, _ := taps253.Load(conn)
	tp, _ := v.(*tap253)
	return tp
}

// take retourne (et consomme) les trames reçues depuis le dernier appel.
func (tp *tap253) take() []string {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	out := append([]string(nil), tp.frames[tp.cursor:]...)
	tp.cursor = len(tp.frames)
	return out
}

func envelope253(frame string) (action string, msg json.RawMessage) {
	var env struct {
		Action string          `json:"ACTION"`
		Msg    json.RawMessage `json:"MSG"`
	}
	if json.Unmarshal([]byte(frame), &env) != nil {
		return "", nil
	}
	return env.Action, env.Msg
}

// waitFrame253 attend (≤ 2 s) une trame d'ACTION `action` vérifiant `match` ; les trames lues sont consommées.
func waitFrame253(conn *websocket.Conn, action string, match func(json.RawMessage) bool) (json.RawMessage, bool) {
	tp := tapOf253(conn)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, f := range tp.take() {
			if a, m := envelope253(f); a == action && (match == nil || match(m)) {
				return m, true
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil, false
}

// readRefused253 : ACTION_REFUSED pour `action` ; ok=false si absent après 2 s.
func readRefused253(conn *websocket.Conn, action string) (reason string, ok bool) {
	m, found := waitFrame253(conn, "ACTION_REFUSED", func(m json.RawMessage) bool {
		var p struct {
			Action string `json:"ACTION"`
		}
		_ = json.Unmarshal(m, &p)
		return p.Action == action
	})
	if !found {
		return "", false
	}
	var p struct {
		Reason string `json:"REASON"`
	}
	_ = json.Unmarshal(m, &p)
	return p.Reason, true
}

// waitAction253 : message MSG de la première trame d'ACTION `action` (échec du test si absente).
func waitAction253(t *testing.T, conn *websocket.Conn, action string) json.RawMessage {
	t.Helper()
	m, ok := waitFrame253(conn, action, nil)
	if !ok {
		t.Fatalf("action %s attendue sur la connexion, rien reçu en 2 s", action)
	}
	return m
}

// collectRaw253 : attend `window` puis retourne (et consomme) les trames reçues depuis le dernier appel.
func collectRaw253(conn *websocket.Conn, window time.Duration) []string {
	time.Sleep(window)
	return tapOf253(conn).take()
}

func anyContains253(frames []string, needle string) bool {
	for _, f := range frames {
		if strings.Contains(f, needle) {
			return true
		}
	}
	return false
}

func countAction253(frames []string, action string) int {
	n := 0
	for _, f := range frames {
		var env struct {
			Action string `json:"ACTION"`
		}
		if json.Unmarshal([]byte(f), &env) == nil && env.Action == action {
			n++
		}
	}
	return n
}

// warnAction253 : WARN ACTION_REFUSED_LAUNCH pour cette action et ce type de poste (valeurs avec ou sans guillemets).
func (e *env253) warnAction(action, clientType string) bool {
	reA := regexp.MustCompile(`action="?` + regexp.QuoteMeta(action) + `"?(\s|$)`)
	reT := regexp.MustCompile(`type="?` + regexp.QuoteMeta(clientType) + `"?(\s|$)`)
	for _, l := range e.logLines(game.LogLevelWarn) {
		if strings.Contains(l, "ACTION_REFUSED_LAUNCH") && reA.MatchString(l) && reT.MatchString(l) {
			return true
		}
	}
	return false
}

// --- 1. Pose du verrou et sérialisation ---------------------------------------------------------------------------------

func TestLaunchLock253_StateIsAlwaysSerializedAndIdleByDefault(t *testing.T) {
	e := newEnv253(t, true)
	l := e.lock()
	if !l.Present {
		t.Fatal("GAME.LAUNCH_LOCK toujours sérialisé (même au repos)")
	}
	if l.Active || l.Owner != "" || l.Since != 0 || l.ExpiresAt != 0 {
		t.Errorf("repos : {ACTIVE:false, OWNER:\"\", SINCE:0, EXPIRES_AT:0} ; obtenu %+v", l)
	}
}

func TestLaunchLock253_OpenByAdminTakesTheLockInTheNameOfThatConnection(t *testing.T) {
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)

	if !e.waitLock(func(l lockState253) bool { return l.Active }) {
		t.Fatalf("OPEN pose le verrou : %+v", e.lock())
	}
	l := e.lock()
	if l.Owner != e.admin1 {
		t.Errorf("OWNER = identifiant de connexion de l'admin (%q), obtenu %q", e.admin1, l.Owner)
	}
	if l.Since <= 0 || l.ExpiresAt-l.Since != 45000 {
		t.Errorf("SINCE > 0 et EXPIRES_AT = SINCE + 45 s (ms epoch) : %+v", l)
	}
	if !e.hasLog(game.LogLevelInfo, "LAUNCH_LOCK_OPENED") {
		t.Errorf("INFO LAUNCH_LOCK_OPENED ; logs=%v", e.logLines(game.LogLevelInfo))
	}
}

func TestLaunchLock253_OnlyAdminAndAnimReceiveTheLockNeverTvNorPlayers(t *testing.T) {
	e := newEnv253(t, true)
	collectRaw253(e.a1, 200*time.Millisecond)
	collectRaw253(e.a2, 50*time.Millisecond)
	collectRaw253(e.an, 50*time.Millisecond)
	collectRaw253(e.tvc, 50*time.Millisecond)
	collectRaw253(e.paulConn, 50*time.Millisecond)

	e.openLock(t, e.admin1)
	e.app.broadcastUpdate()

	adminFrames := collectRaw253(e.a2, 600*time.Millisecond)
	animFrames := collectRaw253(e.an, 300*time.Millisecond)
	tvFrames := collectRaw253(e.tvc, 300*time.Millisecond)
	playerFrames := collectRaw253(e.paulConn, 300*time.Millisecond)
	if !anyContains253(adminFrames, `"LAUNCH_LOCK"`) {
		t.Error("un 2ᵉ poste admin reçoit LAUNCH_LOCK")
	}
	if !anyContains253(animFrames, `"LAUNCH_LOCK"`) {
		t.Error("l'anim reçoit LAUNCH_LOCK")
	}
	if anyContains253(tvFrames, "LAUNCH_LOCK") {
		t.Error("la TV ne reçoit jamais LAUNCH_LOCK")
	}
	if anyContains253(playerFrames, "LAUNCH_LOCK") {
		t.Error("le VJoueur ne reçoit jamais LAUNCH_LOCK")
	}
}

func TestLaunchLock253_OpenFromANonAdminIsRejectedByTheAllowList(t *testing.T) {
	e := newEnv253(t, true)
	e.send(t, e.anim, "anim", "LAUNCH_LOCK", map[string]interface{}{"STATE": "OPEN"})
	e.send(t, e.paulCID, "vplayer", "LAUNCH_LOCK", map[string]interface{}{"STATE": "OPEN"})
	time.Sleep(200 * time.Millisecond)
	if e.lock().Active {
		t.Error("LAUNCH_LOCK est réservé à l'admin")
	}
}

// --- 2. Refus pour TOUS les postes (critical) ----------------------------------------------------------------------------

var adminSuspended253 = []string{
	"READY", "FORCE_READY", "START", "STOP", "PAUSE", "CONTINUE", "REVEAL", "REMOTE", "QUESTION_SOUND", "ENTRACTE_SET",
	"MEMORY_SET_TEAMS", "MEMOTION_SET_TEAMS", "MEMOTION_FLIP", "MEMOTION_STOP_TIMER", "MEMOTION_REVEAL", "MEMOTION_DONE",
	"RAFALE_VALIDATE", "RAFALE_INVALIDATE", "RAFALE_SET_TEAMS",
	"TEAM_POINTS", "BUMPER_POINTS", "TEAM_POINTS_BATCH", "SET_CREDIT_POINTS",
	"UPDATE", "FULL", "DELETE_BUMPER", "RELEASE_BUMPER_NAME",
}

var animSuspended253 = []string{
	"READY", "START", "STOP", "PAUSE", "CONTINUE", "REVEAL", "QUESTION_SOUND",
	"RAFALE_VALIDATE", "RAFALE_INVALIDATE", "TEAM_POINTS", "BUMPER_POINTS",
	"FLIP_MEMORY_CARD", "MEMOTION_SELECT", "MEMOTION_FLIP", "MEMOTION_STOP_TIMER", "MEMOTION_REVEAL", "MEMOTION_DONE",
}

// payload253 : charge utile plausible ; le refus précède toute interprétation, elle n'est là que pour que l'action
// aurait un effet visible si elle était appliquée.
func (e *env253) payload253(action string) interface{} {
	st := e.state249()
	switch action {
	case "TEAM_POINTS":
		return map[string]interface{}{"TEAM": "Les Bleus", "POINTS": 7}
	case "BUMPER_POINTS":
		return map[string]interface{}{"ID": e.paul, "POINTS": 7}
	case "TEAM_POINTS_BATCH":
		return map[string]interface{}{"ENTRIES": []map[string]interface{}{{"TEAM": "Les Bleus", "POINTS": 7}}}
	case "BUMPER_POINTS_BATCH":
		return map[string]interface{}{"ENTRIES": []map[string]interface{}{{"ID": e.paul, "POINTS": 7}}}
	case "UPDATE", "FULL":
		bumpers := map[string]interface{}{}
		for id, b := range st.Bumpers {
			c := map[string]interface{}{}
			for k, v := range b {
				c[k] = v
			}
			if id == e.paul {
				c["SCORE"] = 100
			}
			bumpers[id] = c
		}
		return map[string]interface{}{"teams": teamsMapOf243(st), "bumpers": bumpers}
	case "DELETE_BUMPER":
		return map[string]interface{}{"ID": e.paul}
	case "RELEASE_BUMPER_NAME":
		return map[string]interface{}{"NAME": "Paul"}
	}
	return map[string]interface{}{}
}

func TestLaunchLock253_Critical_EverySuspendedActionIsRefusedForEveryStation(t *testing.T) {
	senders := []struct {
		name, ctype string
		actions     []string
		id          func(*env253) string
		conn        func(*env253) *websocket.Conn
	}{
		{"propriétaire", "admin", adminSuspended253, func(e *env253) string { return e.admin1 }, func(e *env253) *websocket.Conn { return e.a1 }},
		{"2e admin", "admin", adminSuspended253, func(e *env253) string { return e.admin2 }, func(e *env253) *websocket.Conn { return e.a2 }},
		{"anim", "anim", animSuspended253, func(e *env253) string { return e.anim }, func(e *env253) *websocket.Conn { return e.an }},
	}
	for _, s := range senders {
		t.Run(s.name, func(t *testing.T) {
			e := newEnv253(t, true)
			e.openLock(t, e.admin1)
			if !e.waitLock(func(l lockState253) bool { return l.Active }) {
				t.Fatal("précondition : verrou posé")
			}
			before := e.gameFingerprint()
			history := len(e.app.engine.GetHistory())

			for _, action := range s.actions {
				e.send(t, s.id(e), s.ctype, action, e.payload253(action))
				reason, ok := readRefused253(s.conn(e), action)
				if !ok {
					t.Errorf("%s par %s : ACTION_REFUSED {ACTION: %s} attendu sur la connexion de l'émetteur", action, s.name, action)
					continue
				}
				if reason != "LAUNCH_IN_PROGRESS" {
					t.Errorf("%s : REASON = %q (attendu LAUNCH_IN_PROGRESS)", action, reason)
				}
				if !e.warnAction(action, s.ctype) {
					t.Errorf("%s : WARN ACTION_REFUSED_LAUNCH (client, type=%s, action) ; logs=%v", action, s.ctype, e.logLines(game.LogLevelWarn))
				}
			}
			if after := e.gameFingerprint(); after != before {
				t.Errorf("l'état de jeu (équipes, scores, phase, question) doit rester inchangé :\n%s\n→\n%s", before, after)
			}
			if got := len(e.app.engine.GetHistory()); got != history {
				t.Errorf("aucun point attribué : historique %d → %d", history, got)
			}
			if !e.lock().Active || e.lock().Owner != e.admin1 {
				t.Errorf("le verrou reste tenu par le propriétaire : %+v", e.lock())
			}
		})
	}
}

func TestLaunchLock253_RefusalIsTargetedAtTheSenderOnly(t *testing.T) {
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })
	collectRaw253(e.a1, 200*time.Millisecond)
	collectRaw253(e.a2, 50*time.Millisecond)
	collectRaw253(e.an, 50*time.Millisecond)
	collectRaw253(e.tvc, 50*time.Millisecond)

	e.send(t, e.anim, "anim", "TEAM_POINTS", map[string]interface{}{"TEAM": "Les Bleus", "POINTS": 3})
	if _, ok := readRefused253(e.an, "TEAM_POINTS"); !ok {
		t.Fatal("l'émetteur est informé")
	}
	for name, c := range map[string]*websocket.Conn{"admin propriétaire": e.a1, "2e admin": e.a2, "TV": e.tvc} {
		if n := countAction253(collectRaw253(c, 250*time.Millisecond), "ACTION_REFUSED"); n != 0 {
			t.Errorf("%s ne reçoit pas le refus d'un autre poste (%d)", name, n)
		}
	}
}

func TestLaunchLock253_ActionsAreAcceptedAgainOnceTheLockIsReleased(t *testing.T) {
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })
	e.send(t, e.admin2, "admin", "TEAM_POINTS", map[string]interface{}{"TEAM": "Les Bleus", "POINTS": 5})
	if _, ok := readRefused253(e.a2, "TEAM_POINTS"); !ok {
		t.Fatal("précondition : refusé sous verrou")
	}

	base := e.teamScore("Les Bleus")
	e.closeLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return !l.Active })
	collectRaw253(e.a2, 200*time.Millisecond)
	e.send(t, e.admin2, "admin", "TEAM_POINTS", map[string]interface{}{"TEAM": "Les Bleus", "POINTS": 5})
	if n := countAction253(collectRaw253(e.a2, 400*time.Millisecond), "ACTION_REFUSED"); n != 0 {
		t.Error("après la levée du verrou, plus aucun refus")
	}
	if e.teamScore("Les Bleus") != base+5 {
		t.Errorf("le crédit est appliqué après la levée : score %d → %d", base, e.teamScore("Les Bleus"))
	}
}

func TestLaunchLock253_WithoutLockNothingIsRefused(t *testing.T) {
	e := newEnv253(t, true)
	e.send(t, e.anim, "anim", "TEAM_POINTS", map[string]interface{}{"TEAM": "Les Bleus", "POINTS": 2})
	e.send(t, e.admin2, "admin", "BUMPER_POINTS", map[string]interface{}{"ID": e.paul, "POINTS": 1})
	if n := countAction253(collectRaw253(e.an, 300*time.Millisecond), "ACTION_REFUSED"); n != 0 {
		t.Error("sans verrou, aucun ACTION_REFUSED")
	}
	if e.countLog(game.LogLevelWarn, "ACTION_REFUSED_LAUNCH") != 0 {
		t.Error("sans verrou, aucun WARN ACTION_REFUSED_LAUNCH")
	}
}

// --- 2 bis. Actions toujours permises ----------------------------------------------------------------------------------

func TestLaunchLock253_PlayersAndAuxiliaryActionsStayAccepted(t *testing.T) {
	e := newEnv253(t, false) // question STARTED : la frappe ARDOISE est observable
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })
	collectRaw253(e.paulConn, 200*time.Millisecond)

	e.send(t, e.paulCID, "vplayer", protocol.ActionArdoiseInput, map[string]interface{}{"TEXT": "LYON"})
	e.send(t, e.paulCID, "vplayer", "VPLAYER_QCM_ANSWER", map[string]interface{}{"ANSWER": "A"})
	e.send(t, e.paulCID, "vplayer", "PONG", map[string]interface{}{})
	e.send(t, e.paulCID, "vplayer", "BUTTON", map[string]interface{}{})
	e.send(t, e.paulCID, "vplayer", "CLIENT_TRACE", map[string]interface{}{"KIND": "VISIBILITY"})
	e.send(t, e.admin2, "admin", "REGIE_MESSAGE_SEND", map[string]interface{}{"TEXT": "Bienvenue"})
	e.send(t, e.anim, "anim", "REGIE_MESSAGE_CLEAR", map[string]interface{}{})
	e.send(t, e.admin2, "admin", "UPDATE_ENTRACTE_CONFIG", map[string]interface{}{})
	e.send(t, e.admin2, "admin", "UPDATE_QUIZ_META", map[string]interface{}{})
	e.send(t, e.anim, "anim", "CLIENT_TRACE", map[string]interface{}{"KIND": "VISIBILITY"})
	time.Sleep(300 * time.Millisecond)

	if txt, _ := e.answers(); txt["Les Bleus"] != "LYON" {
		t.Errorf("la frappe ARDOISE du joueur est appliquée sous verrou : %v", txt)
	}
	if n := e.countLog(game.LogLevelWarn, "ACTION_REFUSED_LAUNCH"); n != 0 {
		t.Errorf("aucune de ces actions n'est suspendue (%d WARN) ; logs=%v", n, e.logLines(game.LogLevelWarn))
	}
	if n := countAction253(collectRaw253(e.paulConn, 200*time.Millisecond), "ACTION_REFUSED"); n != 0 {
		t.Errorf("le joueur n'est jamais refusé (%d)", n)
	}
}

// --- 3. NEW_GAME -------------------------------------------------------------------------------------------------------

func TestLaunchLock253_NewGameFromTheOwnerIsAppliedAndReleasesTheLock(t *testing.T) {
	e := newEnv253(t, true)
	e.app.engine.UpdateTeamScore("Les Bleus", 9)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })

	e.send(t, e.admin1, "admin", protocol.ActionNewGame, map[string]interface{}{"PURGE_SCORES": true, "RESET_QUESTIONS": true})

	if !e.waitLock(func(l lockState253) bool { return !l.Active }) {
		t.Fatal("NEW_GAME appliqué lève le verrou")
	}
	if l := e.lock(); l.Owner != "" || l.Since != 0 || l.ExpiresAt != 0 {
		t.Errorf("verrou remis à zéro : %+v", l)
	}
	if e.teamScore("Les Bleus") != 0 {
		t.Error("le NEW_GAME du propriétaire est appliqué (scores purgés)")
	}
	if !e.hasLog(game.LogLevelInfo, "LAUNCH_LOCK_RELEASED", "new_game") {
		t.Errorf("INFO LAUNCH_LOCK_RELEASED reason=new_game ; logs=%v", e.logLines(game.LogLevelInfo))
	}
}

func TestLaunchLock253_Critical_NewGameFromAnotherAdminIsRefusedWithTheLaunchReason(t *testing.T) {
	e := newEnv253(t, true)
	e.app.engine.UpdateTeamScore("Les Bleus", 9)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })
	before := e.gameFingerprint()

	e.send(t, e.admin2, "admin", protocol.ActionNewGame, map[string]interface{}{"PURGE_SCORES": true})

	raw := waitAction253(t, e.a2, "NEW_GAME_REFUSED")
	var refused struct {
		Reason string `json:"REASON"`
	}
	_ = json.Unmarshal(raw, &refused)
	if refused.Reason != "LAUNCH_IN_PROGRESS" {
		t.Errorf("REASON = %q (attendu LAUNCH_IN_PROGRESS)", refused.Reason)
	}
	if e.gameFingerprint() != before || e.teamScore("Les Bleus") != 9 {
		t.Error("rien n'est appliqué")
	}
	if !e.lock().Active || e.lock().Owner != e.admin1 {
		t.Errorf("le verrou du propriétaire reste en place : %+v", e.lock())
	}
}

// --- 4. Refus d'ouverture ----------------------------------------------------------------------------------------------

func TestLaunchLock253_OpenByASecondAdminIsRefusedLockedByOther(t *testing.T) {
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })
	before := e.lock()

	e.openLock(t, e.admin2)

	raw := waitAction253(t, e.a2, "LAUNCH_LOCK_REFUSED")
	var refused struct {
		Reason string `json:"REASON"`
	}
	_ = json.Unmarshal(raw, &refused)
	if refused.Reason != "LOCKED_BY_OTHER" {
		t.Errorf("REASON = %q (attendu LOCKED_BY_OTHER)", refused.Reason)
	}
	if after := e.lock(); after.Owner != before.Owner || after.Since != before.Since {
		t.Errorf("rien ne change : %+v → %+v", before, after)
	}
	if !e.hasLog(game.LogLevelWarn, "LAUNCH_LOCK_REFUSED", "LOCKED_BY_OTHER") {
		t.Errorf("WARN LAUNCH_LOCK_REFUSED ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	if n := countAction253(collectRaw253(e.a1, 300*time.Millisecond), "LAUNCH_LOCK_REFUSED"); n != 0 {
		t.Error("le refus est ciblé sur l'émetteur seul")
	}
}

func TestLaunchLock253_OpenDuringAnEntracteIsRefused(t *testing.T) {
	e := newEnv253(t, true)
	if !e.app.engine.SetEntracte(true) || !e.app.engine.IsEntracte() {
		t.Skip("précondition : entracte non activable dans ce montage")
	}
	e.openLock(t, e.admin1)

	raw := waitAction253(t, e.a1, "LAUNCH_LOCK_REFUSED")
	var refused struct {
		Reason string `json:"REASON"`
	}
	_ = json.Unmarshal(raw, &refused)
	if refused.Reason != "ENTRACTE_ACTIVE" {
		t.Errorf("REASON = %q (attendu ENTRACTE_ACTIVE)", refused.Reason)
	}
	if e.lock().Active {
		t.Error("aucun verrou posé pendant une entracte")
	}
}

// --- 5. Renouvellement ------------------------------------------------------------------------------------------------

func TestLaunchLock253_OwnerOpenAgainRenewsTheExpiryWithoutRefusal(t *testing.T) {
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })
	first := e.lock()
	time.Sleep(30 * time.Millisecond)

	e.openLock(t, e.admin1)

	if !e.waitLock(func(l lockState253) bool { return l.ExpiresAt > first.ExpiresAt }) {
		t.Fatalf("EXPIRES_AT repoussé de 45 s à chaque renouvellement : %+v → %+v", first, e.lock())
	}
	after := e.lock()
	if after.Since != first.Since || after.Owner != first.Owner {
		t.Errorf("SINCE et OWNER inchangés par un renouvellement : %+v → %+v", first, after)
	}
	if after.ExpiresAt-after.Since < 45000 {
		t.Errorf("EXPIRES_AT ≥ SINCE + 45 s : %+v", after)
	}
	if n := countAction253(collectRaw253(e.a1, 300*time.Millisecond), "LAUNCH_LOCK_REFUSED"); n != 0 {
		t.Error("un OPEN du propriétaire est un renouvellement, jamais refusé")
	}
	if e.countLog(game.LogLevelInfo, "LAUNCH_LOCK_OPENED") != 1 {
		t.Errorf("LAUNCH_LOCK_OPENED n'est journalisé qu'à la pose (pas à chaque renouvellement) : %d", e.countLog(game.LogLevelInfo, "LAUNCH_LOCK_OPENED"))
	}
}

// --- 6. Libération -----------------------------------------------------------------------------------------------------

func TestLaunchLock253_CloseByTheOwnerReleasesTheLock(t *testing.T) {
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })

	e.closeLock(t, e.admin1)

	if !e.waitLock(func(l lockState253) bool { return !l.Active }) {
		t.Fatal("CLOSE du propriétaire lève le verrou")
	}
	if !e.hasLog(game.LogLevelInfo, "LAUNCH_LOCK_RELEASED", "close", "held_ms") {
		t.Errorf("INFO LAUNCH_LOCK_RELEASED reason=close held_ms=… ; logs=%v", e.logLines(game.LogLevelInfo))
	}
	if e.hasLog(game.LogLevelWarn, "LAUNCH_LOCK_RELEASED", e.admin1) { // filtré sur CE propriétaire : une libération tardive d'un test précédent peut écrire dans le logger global
		t.Errorf("une fermeture normale n est pas un WARN ; warns=%v", e.logLines(game.LogLevelWarn))
	}
}

func TestLaunchLock253_CloseByANonOwnerIsIgnored(t *testing.T) {
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })

	e.closeLock(t, e.admin2)
	time.Sleep(200 * time.Millisecond)

	if l := e.lock(); !l.Active || l.Owner != e.admin1 {
		t.Errorf("le verrou du propriétaire est intact : %+v", l)
	}
}

func TestLaunchLock253_Critical_OwnerDisconnectReleasesTheLockImmediately(t *testing.T) {
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })

	e.a1.Close() // onglet fermé, rechargement ou coupure

	if !e.waitLock(func(l lockState253) bool { return !l.Active }) {
		t.Fatal("la déconnexion du propriétaire lève le verrou sans attendre l'expiration")
	}
	if !e.waitLog(t, game.LogLevelWarn, "LAUNCH_LOCK_RELEASED") || !e.hasLog(game.LogLevelWarn, "LAUNCH_LOCK_RELEASED", "disconnect") {
		t.Errorf("WARN LAUNCH_LOCK_RELEASED reason=disconnect ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	// les autres postes retrouvent la main
	collectRaw253(e.a2, 200*time.Millisecond)
	e.send(t, e.admin2, "admin", "TEAM_POINTS", map[string]interface{}{"TEAM": "Les Bleus", "POINTS": 4})
	if n := countAction253(collectRaw253(e.a2, 300*time.Millisecond), "ACTION_REFUSED"); n != 0 {
		t.Error("après la déconnexion du propriétaire, les actions sont de nouveau acceptées")
	}
}

func TestLaunchLock253_DisconnectOfAnotherStationLeavesTheLockInPlace(t *testing.T) {
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })

	e.a2.Close()
	e.an.Close()
	time.Sleep(400 * time.Millisecond)

	if l := e.lock(); !l.Active || l.Owner != e.admin1 {
		t.Errorf("seule la déconnexion du propriétaire lève le verrou : %+v", l)
	}
}

// BUMPER_POINTS_BATCH (#254) est aussi suspendue (contrat §LAUNCH_LOCK, e2098e70 + b5121933) : testée à part car son
// ouverture dans l'allow-list admin relève du lot #254 (sans elle l'action est rejetée avant le verrou).
func TestLaunchLock253_BumperPointsBatchIsSuspendedToo(t *testing.T) {
	e := newEnv253(t, true)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })
	e.send(t, e.admin2, "admin", "BUMPER_POINTS_BATCH", e.payload253("BUMPER_POINTS_BATCH"))
	if reason, ok := readRefused253(e.a2, "BUMPER_POINTS_BATCH"); !ok || reason != "LAUNCH_IN_PROGRESS" {
		t.Errorf("BUMPER_POINTS_BATCH sous verrou : ACTION_REFUSED LAUNCH_IN_PROGRESS attendu (obtenu %q, ok=%v)", reason, ok)
	}
}

// --- HELLO_ACK : identifiant de connexion (contrat §HELLO_ACK) -------------------------------------------------------------

func TestLaunchLock253_Critical_HelloAckGivesEachAdminAndAnimItsConnectionIDBeforeTheInitialState(t *testing.T) {
	e := helloEnv253(t)
	for _, c := range []struct {
		name, id, ctype string
		conn            *websocket.Conn
	}{{"admin", e.admin2, "admin", e.a2}, {"anim", e.anim, "anim", e.an}} {
		collectRaw253(c.conn, 100*time.Millisecond) // purge des trames de connexion
		e.send(t, c.id, c.ctype, "HELLO", map[string]interface{}{})
		time.Sleep(300 * time.Millisecond)

		frames := tapOf253(c.conn).take()
		ackAt, updateAt := -1, -1
		for i, f := range frames {
			switch a, m := envelope253(f); a {
			case "HELLO_ACK":
				var p struct {
					ClientID string `json:"CLIENT_ID"`
				}
				_ = json.Unmarshal(m, &p)
				if p.ClientID != c.id {
					t.Errorf("%s : CLIENT_ID = %q, identifiant de connexion du hub = %q", c.name, p.ClientID, c.id)
				}
				if ackAt < 0 {
					ackAt = i
				}
			case "UPDATE":
				if updateAt < 0 {
					updateAt = i
				}
			}
		}
		if ackAt < 0 {
			t.Errorf("%s : HELLO_ACK attendu en réponse à HELLO", c.name)
		} else if updateAt >= 0 && ackAt > updateAt {
			t.Errorf("%s : HELLO_ACK doit précéder l'UPDATE initial (ack #%d, update #%d)", c.name, ackAt, updateAt)
		}
	}
}

func TestLaunchLock253_HelloAckIsNeverSentToTvPlayerOrToAnotherStation(t *testing.T) {
	e := helloEnv253(t)
	for _, c := range []*websocket.Conn{e.a1, e.a2, e.an, e.tvc, e.paulConn} {
		collectRaw253(c, 50*time.Millisecond)
	}
	e.send(t, e.tv, "tv", "HELLO", map[string]interface{}{})
	e.send(t, e.paulCID, "vplayer", "HELLO", map[string]interface{}{})
	e.send(t, e.admin2, "admin", "HELLO", map[string]interface{}{})
	time.Sleep(300 * time.Millisecond)

	if countAction253(tapOf253(e.tvc).take(), "HELLO_ACK") != 0 || countAction253(tapOf253(e.paulConn).take(), "HELLO_ACK") != 0 {
		t.Error("jamais de HELLO_ACK vers la TV ni vers un VJoueur")
	}
	if countAction253(tapOf253(e.a1).take(), "HELLO_ACK") != 0 || countAction253(tapOf253(e.an).take(), "HELLO_ACK") != 0 {
		t.Error("HELLO_ACK est ciblé sur l'émetteur du HELLO")
	}
}

func TestLaunchLock253_OwnerIsTheSameStringAsTheHelloAckClientID(t *testing.T) {
	e := helloEnv253(t)
	collectRaw253(e.a1, 50*time.Millisecond)
	e.send(t, e.admin1, "admin", "HELLO", map[string]interface{}{})
	m, ok := waitFrame253(e.a1, "HELLO_ACK", nil)
	if !ok {
		t.Fatal("HELLO_ACK attendu")
	}
	var p struct {
		ClientID string `json:"CLIENT_ID"`
	}
	_ = json.Unmarshal(m, &p)
	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })
	if p.ClientID == "" || e.lock().Owner != p.ClientID {
		t.Errorf("LAUNCH_LOCK.OWNER (%q) == HELLO_ACK.CLIENT_ID (%q) : c'est ainsi que le client se reconnaît propriétaire", e.lock().Owner, p.ClientID)
	}
}
