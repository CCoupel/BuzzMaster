package main

// #258 (v11.2) — espion sur TOUTES les trames reçues par des tablettes pendant une question complète : les réponses des
// AUTRES équipes sont neutres pendant que les réponses sont ouvertes, la bonne réponse (QUESTION.QCM_CORRECT / ANSWER) n'atteint
// jamais une tablette avant REVEALED, les coéquipiers se voient, TV et anim sont inchangées. Contrat ab93c53d
// (vplayer-payload-filter.md §7). Scénarios 1 à 8 du plan (_work/reports/planner-20261012-200000.md §2). `critical` : 1, 4.
// Le filtrage est vérifié PAR FORME sur chaque trame (jamais par nom d'action) : tous les chemins sont couverts d'un coup
// (fan-out UPDATE, actions diffusées par type, état initial HELLO).

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type env258 struct {
	*env253
	jadeID, mamieID         string
	tabA2, tabB, tabUnbound *websocket.Conn
	tabA2CID                string
}

const (
	teamA258 = "Les Bleus"
	teamB258 = "Les Rouges"
)

func newEnv258(t *testing.T) *env258 {
	t.Helper()
	e := buildEnv253(t, false, true)
	prev := e.app.engine.GetPhase()
	e.app.engine.SetPhase(game.PhaseEnroll) // l'inscription d'un VJoueur exige la phase ENROLL
	jade := setupVirtualPlayer(t, e.app, "Jade", teamA258)
	e.app.engine.SetPhase(prev)
	a2, a2id := e.dial(t, "/ws/player")
	e.connect(t, a2id, "Jade", jade)
	b, bid := e.dial(t, "/ws/player")
	e.connect(t, bid, "Mamie", e.mamie)
	un, _ := e.dial(t, "/ws/player") // jamais liée (aucun PLAYER_CONNECT)
	for _, c := range []*websocket.Conn{a2, b, un} {
		startTap253(c)
	}
	return &env258{env253: e, jadeID: jade, mamieID: e.mamie, tabA2: a2, tabB: b, tabUnbound: un, tabA2CID: a2id}
}

// stage : purge les trames, exécute, laisse arriver, rend les trames par client.
func (e *env258) stage(do func()) map[string][]string {
	conns := map[string]*websocket.Conn{"A1": e.paulConn, "A2": e.tabA2, "B": e.tabB, "unbound": e.tabUnbound, "tv": e.tvc, "anim": e.an, "admin": e.a1}
	for _, c := range conns {
		tapOf253(c).take()
	}
	do()
	time.Sleep(450 * time.Millisecond)
	out := map[string][]string{}
	for n, c := range conns {
		out[n] = tapOf253(c).take()
	}
	return out
}

// push : diffusion par type d'un message portant l'état complet (forme START/PAUSE/CONTINUE/UPDATE_TIMER/REMOTE…).
func (e *env258) push(action string) {
	msg, _ := protocol.NewMessage(action, json.RawMessage(e.app.engine.GetGameJSON()))
	e.app.wsHub.BroadcastToTypes(msg, server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer, server.ClientTypeAnim)
}

var (
	answerFields258b = []string{"TIME", "BUTTON", "STATUS", "ANSWER_COLOR", "ANSWERED_COLOR", "HINTS_AT_BUZZ"}
	openPhases258b   = map[string]bool{"PREPARE": true, "READY": true, "COUNTDOWN": true, "STARTED": true, "PAUSED": true}
)

func neutral258b(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case float64:
		return x == 0
	case string:
		return x == ""
	}
	return false
}

type frame258 struct {
	Action string
	Game   map[string]interface{}
	Bump   map[string]map[string]interface{}
	Teams  map[string]map[string]interface{}
}

func parse258(f string) (frame258, bool) {
	var env struct {
		Action string `json:"ACTION"`
		Msg    struct {
			Game    map[string]interface{}            `json:"GAME"`
			Bumpers map[string]map[string]interface{} `json:"bumpers"`
			Teams   map[string]map[string]interface{} `json:"teams"`
		} `json:"MSG"`
	}
	if json.Unmarshal([]byte(f), &env) != nil {
		return frame258{}, false
	}
	return frame258{env.Action, env.Msg.Game, env.Msg.Bumpers, env.Msg.Teams}, true
}

// leaks258 : fuites vers une tablette de l'équipe `own` ("" = non liée), `phase` = phase de l'étape quand la trame n'en porte pas.
func leaks258(frames []string, own, phase string) []string {
	var out []string
	for i, raw := range frames {
		f, ok := parse258(raw)
		if !ok {
			continue
		}
		ph := phase
		if p, _ := f.Game["PHASE"].(string); p != "" {
			ph = p
		}
		if openPhases258b[ph] {
			for id, b := range f.Bump {
				if own != "" && b["TEAM"] == own {
					continue
				}
				for _, k := range answerFields258b {
					if v, present := b[k]; present && !neutral258b(v) {
						out = append(out, fmt.Sprintf("trame %d (%s, %s) : bumpers[%s].%s = %v", i, f.Action, ph, id, k, v))
					}
				}
			}
			for name, tm := range f.Teams {
				if own != "" && name == own {
					continue
				}
				for _, k := range []string{"TIME", "BUMPER", "STATUS"} {
					if v, present := tm[k]; present && !neutral258b(v) {
						out = append(out, fmt.Sprintf("trame %d (%s, %s) : teams[%s].%s = %v", i, f.Action, ph, name, k, v))
					}
				}
			}
		}
		if ph != "REVEALED" {
			if q, _ := f.Game["QUESTION"].(map[string]interface{}); q != nil {
				for _, k := range []string{"QCM_CORRECT", "ANSWER"} {
					if _, present := q[k]; present {
						out = append(out, fmt.Sprintf("trame %d (%s, %s) : GAME.QUESTION.%s", i, f.Action, ph, k))
					}
				}
			}
		}
	}
	return out
}

// seen258 : une trame du client porte bumpers[id].field == want.
func seen258(frames []string, id, field string, want interface{}) bool {
	for _, raw := range frames {
		if f, ok := parse258(raw); ok {
			if b, ok := f.Bump[id]; ok && fmt.Sprint(b[field]) == fmt.Sprint(want) {
				return true
			}
		}
	}
	return false
}

func qcm258() *game.Question {
	return &game.Question{ID: "q-qcm", Question: "Capitale ?", Answer: "Paris", Type: game.QuestionTypeQCM, Points: "3",
		TypedContent: game.TypedContent{QCMAnswers: &game.QCMAnswers{Red: "Lyon", Green: "Paris", Yellow: "Nice", Blue: "Metz"}, QCMCorrect: "GREEN"}}
}

func (e *env258) startQCM() {
	e.app.engine.SetPhase(game.PhaseStopped)
	e.app.engine.Ready("", qcm258())
	e.app.engine.StartImmediate(30)
	e.app.broadcastUpdate()
}

// --- 1 + 4 + 5. QCM complète (critical) -----------------------------------------------------------------------------

func TestVPlayerAnswers258_Critical_QCMOtherTeamAnswerAndRightAnswerNeverReachATabletBeforeTheirPhase(t *testing.T) {
	e := newEnv258(t)

	prepare := e.stage(func() {
		e.app.engine.SetPhase(game.PhaseStopped)
		e.app.engine.Ready("", qcm258())
		e.app.broadcastUpdate()
		e.push("REMOTE")
	})
	for name, own := range map[string]string{"A1": teamA258, "A2": teamA258, "B": teamB258, "unbound": ""} {
		if l := leaks258(prepare[name], own, "PREPARE"); len(l) != 0 {
			t.Errorf("PREPARE / %s : %v", name, l)
		}
	}

	started := e.stage(func() {
		e.app.engine.StartImmediate(30)
		e.push("START")
		e.app.broadcastUpdate()
		e.app.engine.ProcessButtonPress(e.mamieID, time.Now().UnixMicro(), "B") // B répond GREEN
		e.app.broadcastUpdate()
		e.push("PAUSE")
		e.push("UPDATE_TIMER")
	})
	// 1. aucune fuite vers A1, A2 et la connexion non liée ; B voit ses propres valeurs
	for name, own := range map[string]string{"A1": teamA258, "A2": teamA258, "unbound": ""} {
		if l := leaks258(started[name], own, "STARTED"); len(l) != 0 {
			t.Errorf("STARTED / %s : %d fuite(s), ex. %v", name, len(l), l[:min(3, len(l))])
		}
		if seen258(started[name], e.mamieID, "ANSWERED_COLOR", "GREEN") {
			t.Errorf("STARTED / %s a vu la réponse de B (ANSWERED_COLOR GREEN)", name)
		}
	}
	if l := leaks258(started["B"], teamB258, "STARTED"); len(l) != 0 {
		t.Errorf("STARTED / B : seuls les champs de l'autre équipe sont neutres : %v", l)
	}
	if !seen258(started["B"], e.mamieID, "ANSWERED_COLOR", "GREEN") || !seen258(started["B"], e.mamieID, "ANSWER_COLOR", "GREEN") {
		t.Error("B reçoit SES PROPRES valeurs (ANSWERED_COLOR et ANSWER_COLOR GREEN)")
	}
	// 7. TV, anim et admin : inchangées (la réponse de B et la bonne réponse y circulent toujours)
	for _, who := range []string{"tv", "anim", "admin"} {
		if !seen258(started[who], e.mamieID, "ANSWERED_COLOR", "GREEN") {
			t.Errorf("STARTED / %s : la réponse de B reste visible (inchangé)", who)
		}
		if !strings.Contains(strings.Join(started[who], ""), `"QCM_CORRECT":"GREEN"`) {
			t.Errorf("STARTED / %s : QCM_CORRECT reste transmis (TV publique, anim, admin)", who)
		}
	}

	// 3. coéquipier : Jade répond, Paul (A1) la voit ; B ne la voit pas
	teammate := e.stage(func() {
		e.app.engine.ProcessButtonPress(e.jadeID, time.Now().UnixMicro(), "A")
		e.app.broadcastUpdate()
		e.push("PAUSE")
	})
	if !seen258(teammate["A1"], e.jadeID, "ANSWERED_COLOR", "RED") && !seen258(teammate["A1"], e.jadeID, "TIME", teammate["A1"]) {
		// TIME est un horodatage inconnu : on se contente de la couleur
		t.Error("A1 voit les champs de réponse de son coéquipier Jade (même équipe)")
	}
	if seen258(teammate["B"], e.jadeID, "ANSWERED_COLOR", "RED") {
		t.Error("B ne voit pas la réponse de l'équipe A")
	}
	if l := leaks258(teammate["B"], teamB258, "STARTED"); len(l) != 0 {
		t.Errorf("B : %v", l)
	}

	// 5. STOPPED : les champs des autres équipes sont transmis (badges), la bonne réponse reste retirée
	stopped := e.stage(func() {
		e.app.engine.Stop()
		e.app.broadcastUpdate()
		e.push("STOP")
	})
	if !seen258(stopped["A1"], e.mamieID, "ANSWERED_COLOR", "GREEN") {
		t.Error("STOPPED : A1 voit la réponse de B (badges des réponses par équipe)")
	}
	if l := leaks258(stopped["A1"], teamA258, "STOPPED"); len(l) != 0 {
		t.Errorf("STOPPED / A1 : seule la bonne réponse reste retirée : %v", l)
	}

	// 4. REVEALED : la trame porte la bonne réponse
	revealed := e.stage(func() {
		e.app.engine.Reveal()
		e.app.broadcastUpdate()
	})
	joined := strings.Join(revealed["A1"], "")
	if !strings.Contains(joined, `"QCM_CORRECT":"GREEN"`) || !strings.Contains(joined, `"ANSWER":"Paris"`) {
		t.Error("REVEALED : la trame reçue par la tablette contient QCM_CORRECT et ANSWER")
	}
}

// --- 2. SPEEDY : buzz et PAUSE, puis CONTINUE ---------------------------------------------------------------------------

func TestVPlayerAnswers258_Critical_SpeedyBuzzPauseAndContinueNeverRevealWhoBuzzedFirst(t *testing.T) {
	e := newEnv258(t)
	speedy := &game.Question{ID: "q-speedy", Question: "Vite !", Answer: "Paris", Type: game.QuestionTypeSpeedy, Points: "2"}

	frames := e.stage(func() {
		e.app.engine.SetPhase(game.PhaseStopped)
		e.app.engine.Ready("", speedy)
		e.app.engine.StartImmediate(30)
		e.app.broadcastUpdate()
		e.app.engine.ProcessButtonPress(e.mamieID, time.Now().UnixMicro(), "A") // B buzze la première
		e.app.broadcastUpdate()
		e.push("PAUSE")
		e.app.engine.Continue()
		e.push("CONTINUE")
		e.app.broadcastUpdate()
	})

	for name, own := range map[string]string{"A1": teamA258, "A2": teamA258, "unbound": ""} {
		if l := leaks258(frames[name], own, "STARTED"); len(l) != 0 {
			t.Errorf("SPEEDY / %s : l'ordre des buzz des autres équipes ne fuit pas : %v", name, l[:min(3, len(l))])
		}
	}
	for _, who := range []string{"tv", "admin"} {
		if !seen258(frames[who], e.mamieID, "STATUS", "PAUSE") && !seen258(frames[who], e.mamieID, "BUTTON", "A") {
			t.Errorf("SPEEDY / %s : le buzz de B reste visible (inchangé)", who)
		}
	}
}

// --- 6. Connexion non liée : état initial (HELLO) -----------------------------------------------------------------------

func TestVPlayerAnswers258_Critical_UnboundTabletInitialStateIsNeutralAndWithoutOnlyFields(t *testing.T) {
	e := newEnv258(t)
	e.startQCM()
	e.app.engine.ProcessButtonPress(e.mamieID, time.Now().UnixMicro(), "B")
	time.Sleep(100 * time.Millisecond)
	conn, id := e.dial(t, "/ws/player")
	startTap253(conn)

	e.send(t, id, "vplayer", "HELLO", map[string]interface{}{})
	time.Sleep(450 * time.Millisecond)
	frames := tapOf253(conn).take()

	if len(frames) == 0 {
		t.Fatal("l'état initial est envoyé au HELLO")
	}
	if l := leaks258(frames, "", "STARTED"); len(l) != 0 {
		t.Errorf("connexion non liée : tout est neutralisé pendant les réponses ouvertes : %v", l[:min(3, len(l))])
	}
	for _, f := range frames {
		if pf, ok := parse258(f); ok {
			for _, k := range []string{"ARDOISE_ANSWERS", "LAUNCH_LOCK"} {
				if _, present := pf.Game[k]; present {
					t.Errorf("GAME.%s ne doit jamais atteindre une tablette (état initial HELLO compris)", k)
				}
			}
		}
	}
}

func TestVPlayerAnswers258_BoundTabletInitialStateAtHelloKeepsItsOwnAndHidesTheOthers(t *testing.T) {
	e := newEnv258(t)
	e.startQCM()
	e.app.engine.ProcessButtonPress(e.mamieID, time.Now().UnixMicro(), "B")
	time.Sleep(100 * time.Millisecond)

	frames := e.stage(func() { e.send(t, e.tabA2CID, "vplayer", "HELLO", map[string]interface{}{}) })

	if l := leaks258(frames["A2"], teamA258, "STARTED"); len(l) != 0 {
		t.Errorf("tablette liée (A2), état initial HELLO : %v", l[:min(3, len(l))])
	}
}

// --- 8. Carte MEMOTION QCM active ------------------------------------------------------------------------------------------

func TestVPlayerAnswers258_MemotionQCMCardRightAnswerDoesNotReachATabletBeforeItsReveal(t *testing.T) {
	e := newEnv258(t)
	card := game.MotionCard{ID: "mc-1", RectoTheme: "Capitales", Difficulty: 1, Type: game.QuestionTypeQCM,
		TypedContent: game.TypedContent{QCMAnswers: &game.QCMAnswers{Red: "Paris", Green: "Lyon", Yellow: "Nice", Blue: "Metz"}, QCMCorrect: "RED"}}

	frames := e.stage(func() {
		e.app.engine.SetPhase(game.PhaseStopped)
		e.app.engine.Ready("", &game.Question{ID: "mq1", Type: game.QuestionTypeMemotion, MotionCards: []game.MotionCard{card}, MotionMode: "SOLO"})
		e.app.engine.SetPhase(game.PhaseStarted)
		e.app.engine.InitMotionState()
		_ = e.app.engine.SelectMotionCard("mc-1")
		_ = e.app.engine.FlipMotionCard()
		e.app.broadcastUpdate()
		e.push("MEMOTION_FLIP")
	})

	// la bonne réponse de la carte (RED / « Paris ») ne doit pas être lisible dans une trame tablette tant que la carte n'est pas révélée
	joined := strings.Join(frames["A1"], "")
	if strings.Contains(joined, `"QCM_CORRECT":"RED"`) {
		t.Error("MEMOTION : QCM_CORRECT de la carte QCM active atteint la tablette avant la révélation de la carte (même règle que QUESTION.QCM_CORRECT, contrat §7.1)")
	}
	if !strings.Contains(strings.Join(frames["admin"], ""), `"QCM_CORRECT":"RED"`) {
		t.Error("l'admin reçoit toujours la bonne réponse (inchangé)")
	}
}

// --- Complément revue #258 (M1 + mineur 2) : médias de réponse avant révélation, UPDATE ciblé après PLAYER_CONNECT ----------------

// keyPaths258 : chemins JSON (a.b.c) où apparaît la clé `key`, dans tout le document.
func keyPaths258(v interface{}, key, path string, out *[]string) {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, c := range x {
			p := path + "." + k
			if k == key {
				*out = append(*out, p)
			}
			keyPaths258(c, key, p, out)
		}
	case []interface{}:
		for i, c := range x {
			keyPaths258(c, key, fmt.Sprintf("%s[%d]", path, i), out)
		}
	}
}

func framesWithKey258(frames []string, key string) []string {
	var hits []string
	for i, raw := range frames {
		var doc interface{}
		if json.Unmarshal([]byte(raw), &doc) != nil {
			continue
		}
		var paths []string
		keyPaths258(doc, key, "", &paths)
		for _, p := range paths {
			hits = append(hits, fmt.Sprintf("trame %d : %s", i, p))
		}
	}
	return hits
}

func qcmWithAnswerMedia258() *game.Question {
	q := qcm258()
	q.MediaAnswer = "answer-media.png"
	return q
}

func TestVPlayerAnswers258_Critical_AnswerMediaNeverReachesATabletBeforeRevealedButStaysForTVAndAnim(t *testing.T) {
	e := newEnv258(t)

	open := e.stage(func() {
		e.app.engine.SetPhase(game.PhaseStopped)
		e.app.engine.Ready("", qcmWithAnswerMedia258())
		e.app.engine.StartImmediate(30)
		e.app.broadcastUpdate()
		e.push("START")
		e.app.engine.Stop()
		e.app.broadcastUpdate()
		e.push("STOP")
	})
	for _, name := range []string{"A1", "A2", "B", "unbound"} {
		if hits := framesWithKey258(open[name], "MEDIA_ANSWER"); len(hits) != 0 {
			t.Errorf("%s : MEDIA_ANSWER (média de la réponse) ne doit pas atteindre une tablette avant REVEALED : %v", name, hits[:min(3, len(hits))])
		}
	}
	for _, who := range []string{"tv", "anim", "admin"} {
		if len(framesWithKey258(open[who], "MEDIA_ANSWER")) == 0 {
			t.Errorf("%s : MEDIA_ANSWER reste transmis (TV publique, anim, admin : inchangés)", who)
		}
	}

	revealed := e.stage(func() {
		e.app.engine.Reveal()
		e.app.broadcastUpdate()
	})
	for _, name := range []string{"A1", "B"} {
		if len(framesWithKey258(revealed[name], "MEDIA_ANSWER")) == 0 {
			t.Errorf("%s : à REVEALED la trame porte MEDIA_ANSWER", name)
		}
	}
}

func TestVPlayerAnswers258_MemotionCardAnswerTextAndImageNeverReachATabletBeforeRevealed(t *testing.T) {
	e := newEnv258(t)
	card := game.MotionCard{ID: "mc-1", RectoTheme: "Capitales", Difficulty: 1, Type: game.QuestionTypeSpeedy,
		QuestionText: "Capitale de la France ?", AnswerText: "Paris-secret", AnswerImage: "secret-answer.png"}
	memotion := func() *game.Question {
		return &game.Question{ID: "mq1", Type: game.QuestionTypeMemotion, MotionCards: []game.MotionCard{card}, MotionMode: "SOLO"}
	}

	open := e.stage(func() {
		e.app.engine.SetPhase(game.PhaseStopped)
		e.app.engine.Ready("", memotion())
		e.app.engine.SetPhase(game.PhaseStarted)
		e.app.engine.InitMotionState()
		_ = e.app.engine.SelectMotionCard("mc-1")
		_ = e.app.engine.FlipMotionCard()
		e.app.broadcastUpdate()
		e.push("MEMOTION_FLIP")
		e.app.engine.Stop()
		e.app.broadcastUpdate()
	})
	for _, name := range []string{"A1", "A2", "B", "unbound"} {
		all := strings.Join(open[name], "")
		for _, secret := range []string{"Paris-secret", "secret-answer.png"} {
			if strings.Contains(all, secret) {
				t.Errorf("%s : %q (ANSWER_TEXT / ANSWER_IMAGE de la carte, ou MEMOTION_ACTIVE.STATE) ne doit pas atteindre une tablette avant REVEALED", name, secret)
			}
		}
	}
	for _, who := range []string{"tv", "admin"} {
		if !strings.Contains(strings.Join(open[who], ""), "Paris-secret") {
			t.Errorf("%s : la réponse de la carte reste transmise (inchangé)", who)
		}
	}

	revealed := e.stage(func() {
		e.app.engine.Reveal()
		e.app.broadcastUpdate()
	})
	if !strings.Contains(strings.Join(revealed["A1"], ""), "Paris-secret") {
		t.Error("à REVEALED la trame tablette porte la réponse de la carte (ANSWER_TEXT)")
	}
}

func TestVPlayerAnswers258_Critical_AfterPlayerConnectATargetedUpdateGivesTheTabletItsTeamData(t *testing.T) {
	e := newEnv258(t)
	e.startQCM()
	e.app.engine.ProcessButtonPress(e.jadeID, time.Now().UnixMicro(), "A")  // Jade (équipe A) a répondu RED
	e.app.engine.ProcessButtonPress(e.mamieID, time.Now().UnixMicro(), "B") // Mamie (équipe B) aussi (GREEN)
	e.app.broadcastUpdate()
	time.Sleep(150 * time.Millisecond)

	// une NOUVELLE connexion non liée, puis PLAYER_CONNECT de Jade : état initial neutre, puis UPDATE ciblé par destinataire
	conn, id := e.dial(t, "/ws/player")
	startTap253(conn)
	e.send(t, id, "vplayer", "HELLO", map[string]interface{}{})
	time.Sleep(300 * time.Millisecond)
	before := tapOf253(conn).take()
	if seen258(before, e.jadeID, "ANSWERED_COLOR", "RED") {
		t.Fatal("précondition : avant PLAYER_CONNECT la connexion non liée ne voit pas la réponse de Jade")
	}

	e.connect(t, id, "Jade", e.jadeID)
	time.Sleep(450 * time.Millisecond)
	after := tapOf253(conn).take()

	if !seen258(after, e.jadeID, "ANSWERED_COLOR", "RED") {
		t.Error("après PLAYER_CONNECT, un UPDATE ciblé rend à la tablette les données de son équipe (Jade voit sa propre réponse)")
	}
	if l := leaks258(after, teamA258, "STARTED"); len(l) != 0 {
		t.Errorf("et l'autre équipe reste neutralisée : %v", l[:min(3, len(l))])
	}
}
