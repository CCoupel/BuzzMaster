package main

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// #144 (v12.0.0, LOT-1.2) — manche ROUE de bout en bout par le VRAI chemin WebSocket :
// vraies connexions admin / animateur / TV / tablette, vrai handleWebMessage, vraie diffusion
// (contracts/roue.md §5, §8, §9). Le moteur est testé à part (internal/game/roue_144_test.go) ; ici on
// vérifie le câblage : actions, droits par émetteur, ROUE_ERROR à l'émetteur seul, double commande,
// non-fuite de la phrase sur le fil.
// ---------------------------------------------------------------------------

const roueWirePhrase = "BONJOUR LE MONDE"

// tap lit une connexion en continu dans une goroutine (une lecture qui expire casse définitivement une
// connexion gorilla : on ne lit donc jamais avec des délais successifs) et accumule les trames reçues.
type tap struct {
	mu     sync.Mutex
	frames []string
}

func newTap(conn *websocket.Conn) *tap {
	tp := &tap{}
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
	return tp
}

// take renvoie (et vide) les trames reçues, après `settle` pour laisser arriver les diffusions en vol.
func (tp *tap) take(settle time.Duration) []string {
	time.Sleep(settle)
	tp.mu.Lock()
	defer tp.mu.Unlock()
	out := tp.frames
	tp.frames = nil
	return out
}

type roueWire struct {
	t     *testing.T
	app   *App
	admin *websocket.Conn
	anim  *websocket.Conn
	tv    *websocket.Conn
	vp    *websocket.Conn
	// lecteurs continus (démarrés une fois les connexions apprises)
	adminTap, animTap, tvTap, vpTap *tap
}

// newRoueWire monte une partie ROUE démarrée (STARTED, TURN), 2 équipes, tirage de roue figé sur la
// case 0 (100 points) — la roue par défaut est 1000/50/2/1, la case 0 = 1000.
func newRoueWire(t *testing.T, mode string) *roueWire {
	t.Helper()
	app := newBroadcast127TestApp(t)
	app.setupRoueCallbacks()
	app.engine.RoueCategoryMeta = func(string) game.CategoryMeta { return game.CategoryMeta{} }
	app.engine.SetTeams(map[string]*game.Team{
		"red":  {Name: "red", Color: []int{255, 0, 0}, Ready: true},
		"blue": {Name: "blue", Color: []int{0, 0, 255}, Ready: true},
	})
	app.engine.SetBumpers(map[string]*game.Bumper{
		"b-red":  {Name: "Rouge", Team: "red", Connected: true, Ready: true},
		"b-blue": {Name: "Bleu", Team: "blue", Connected: true, Ready: true},
	})
	app.engine.SetRoueRand(func(n int) int { return 0 }) // case 0 : le maximum
	app.engine.Ready("rq", &game.Question{ID: "rq", Question: "Roue", Type: game.QuestionTypeRoue, Category: "ANIMALS",
		TypedContent: game.TypedContent{RouePhrase: roueWirePhrase, RoueMode: mode}})

	base := startAnimAllowlistTestServer(t, app)
	w := &roueWire{t: t, app: app}
	w.admin = dialWS(t, base, "/ws/admin")
	learnClientID(t, app, w.admin)
	w.anim = dialWS(t, base, "/ws/anim")
	learnClientID(t, app, w.anim)
	w.tv = dialWS(t, base, "/ws/tv")
	learnClientID(t, app, w.tv)
	w.vp = dialWS(t, base, "/ws/player")
	learnClientID(t, app, w.vp)

	w.adminTap, w.animTap, w.tvTap, w.vpTap = newTap(w.admin), newTap(w.anim), newTap(w.tv), newTap(w.vp)
	w.send(w.admin, protocol.ActionRoueSetTeams, protocol.RoueSetTeamsPayload{Teams: []string{"red", "blue"}})
	app.engine.StartImmediate(0)
	t.Cleanup(func() { app.engine.Stop() })
	if s := app.engine.GetState(); s.Phase != game.PhaseStarted || s.RoueSubPhase != game.RoueSubTurn {
		t.Fatalf("la manche ROUE doit être démarrée en TURN : phase=%s sous-phase=%q", s.Phase, s.RoueSubPhase)
	}
	return w
}

func (w *roueWire) send(conn *websocket.Conn, action string, payload interface{}) {
	w.t.Helper()
	sendAction(w.t, w.app, conn, action, payload)
}

func (w *roueWire) sub() string { return w.app.engine.GetState().RoueSubPhase }

func (w *roueWire) waitSub(sub string, timeout time.Duration) {
	w.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if w.sub() == sub {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	w.t.Fatalf("sous-phase %s non atteinte (actuelle %q)", sub, w.sub())
}

func actionOf(raw string) string {
	var env struct {
		Action string `json:"ACTION"`
	}
	_ = json.Unmarshal([]byte(raw), &env)
	return env.Action
}

// ---- manche complète ---------------------------------------------------------------------------------------

func TestRoueWire144_FullRound_ThroughRealWebSockets(t *testing.T) {
	w := newRoueWire(t, game.RoueModeTantQueJeGagne)
	step := func() int { return w.app.engine.GetState().RoueStep }

	w.send(w.admin, protocol.ActionRoueSpin, struct{}{})
	if w.sub() != game.RoueSubSpinning || step() != 2 {
		t.Fatalf("après ROUE_SPIN : %q step %d", w.sub(), step())
	}
	w.send(w.admin, protocol.ActionRoueStop, struct{}{})
	if w.sub() != game.RoueSubStopping || step() != 3 {
		t.Fatalf("après ROUE_STOP : %q step %d", w.sub(), step())
	}
	w.waitSub(game.RoueSubLetter, 5*time.Second) // freinage serveur, ≥ 1,5 s
	w.send(w.admin, protocol.ActionRoueLetter, protocol.RoueLetterPayload{Letter: "N"})
	if w.sub() != game.RoueSubLit || step() != 5 {
		t.Fatalf("après ROUE_LETTER : %q step %d", w.sub(), step())
	}
	w.send(w.admin, protocol.ActionRoueFlip, struct{}{})
	w.waitSub(game.RoueSubAnswer, 5*time.Second)
	if step() != 6 {
		t.Fatalf("ANSWER = étape 6, got %d", step())
	}
	w.send(w.admin, protocol.ActionRoueAnswer, struct{}{})
	s := w.app.engine.GetState()
	if s.RoueSubPhase != game.RoueSubSolved || s.RoueWinner != "red" {
		t.Fatalf("manche résolue pour red : %q winner=%q", s.RoueSubPhase, s.RoueWinner)
	}
	if got := s.RoueAwards["red"]; got != 2000 { // 2 × N à 1000 (case 0 de la roue par défaut)
		t.Errorf("crédit red = %d, attendu 2000", got)
	}
	if w.app.engine.GetTeam("red").Score != 2000 || w.app.engine.GetTeam("blue").Score != 0 {
		t.Errorf("scores red/blue = %d/%d", w.app.engine.GetTeam("red").Score, w.app.engine.GetTeam("blue").Score)
	}
}

// ---- non-fuite sur le fil (critical) --------------------------------------------------------------------------

// Pour chaque étape d'une manche, aucune trame reçue par la TV ou la tablette ne contient la phrase ;
// l'admin et l'animateur, eux, la reçoivent (contrat §9).
func TestRoueWire144_PhraseNeverReachesTVOrTablet_AdminAndAnimKeepIt(t *testing.T) {
	w := newRoueWire(t, game.RoueModeTantQueJeGagne)
	frames := map[string][]string{}
	collect := func() {
		frames["tv"] = append(frames["tv"], w.tvTap.take(80*time.Millisecond)...)
		frames["vp"] = append(frames["vp"], w.vpTap.take(0)...)
		frames["admin"] = append(frames["admin"], w.adminTap.take(0)...)
		frames["anim"] = append(frames["anim"], w.animTap.take(0)...)
	}
	collect()
	w.send(w.admin, protocol.ActionRoueSpin, struct{}{})
	collect()
	w.send(w.anim, protocol.ActionRoueStop, struct{}{})
	collect()
	w.waitSub(game.RoueSubLetter, 5*time.Second)
	collect()
	w.send(w.admin, protocol.ActionRoueLetter, protocol.RoueLetterPayload{Letter: "N"})
	collect()
	w.send(w.admin, protocol.ActionRoueFlip, struct{}{})
	collect()
	w.waitSub(game.RoueSubAnswer, 5*time.Second)
	collect()
	// Après résolution le plateau est entièrement visible (légitime) : on ne cherche plus les fragments de
	// lignes, seulement la clé de la phrase et la phrase entière.
	w.send(w.admin, protocol.ActionRoueReveal, struct{}{})
	solved := map[string][]string{"tv": w.tvTap.take(150 * time.Millisecond), "vp": w.vpTap.take(0)}
	for who, fs := range solved {
		if len(fs) == 0 {
			t.Errorf("%s doit recevoir l'UPDATE de résolution", who)
		}
		for _, f := range fs {
			if strings.Contains(f, "ROUE_PHRASE") || strings.Contains(f, roueWirePhrase) {
				t.Fatalf("FUITE vers %s après résolution : %s", who, f)
			}
		}
	}

	for _, who := range []string{"tv", "vp"} {
		if len(frames[who]) < 6 { // au moins un UPDATE par étape jouée : sinon le test serait vide
			t.Fatalf("%s n'a reçu que %d trame(s) : test sans portée", who, len(frames[who]))
		}
		for _, f := range frames[who] {
			for _, frag := range []string{"ROUE_PHRASE", roueWirePhrase, "BONJOUR LE", "MONDE"} {
				if strings.Contains(f, frag) {
					t.Fatalf("FUITE vers %s (%s) : %q trouvé dans %s", who, actionOf(f), frag, f)
				}
			}
		}
	}
	for _, who := range []string{"admin", "anim"} {
		found := false
		for _, f := range frames[who] {
			if strings.Contains(f, roueWirePhrase) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s doit recevoir la phrase", who)
		}
	}
}

// Le plateau diffusé à la TV est fidèle à l'avancement : lettre révélée visible, les autres cachées.
func TestRoueWire144_TVBoardShowsOnlyRevealedLetters(t *testing.T) {
	w2 := newRoueWire(t, game.RoueModeTantQueJeGagne)
	w2.send(w2.admin, protocol.ActionRoueSpin, struct{}{})
	w2.send(w2.admin, protocol.ActionRoueStop, struct{}{})
	w2.waitSub(game.RoueSubLetter, 5*time.Second)
	w2.send(w2.admin, protocol.ActionRoueLetter, protocol.RoueLetterPayload{Letter: "N"})
	w2.send(w2.admin, protocol.ActionRoueFlip, struct{}{})
	w2.waitSub(game.RoueSubAnswer, 5*time.Second)
	frames := w2.tvTap.take(150 * time.Millisecond)
	var last string
	for _, f := range frames {
		if actionOf(f) == protocol.ActionUpdate && strings.Contains(f, "ROUE_BOARD") {
			last = f
		}
	}
	if last == "" {
		t.Fatalf("aucune trame UPDATE avec ROUE_BOARD reçue par la TV")
	}
	var env struct {
		Msg struct {
			Game struct {
				Board struct {
					Rows []string `json:"ROWS"`
				} `json:"ROUE_BOARD"`
			} `json:"GAME"`
		} `json:"MSG"`
	}
	_ = json.Unmarshal([]byte(last), &env)
	joined := strings.Join(env.Msg.Game.Board.Rows, "")
	for _, ch := range joined {
		if ch >= 'A' && ch <= 'Z' && ch != 'N' {
			t.Errorf("la TV voit une lettre non révélée %q : %v", ch, env.Msg.Game.Board.Rows)
		}
	}
	if !strings.Contains(joined, "N") {
		t.Errorf("la lettre N révélée doit être visible : %v", env.Msg.Game.Board.Rows)
	}
}

// ---- droits par émetteur --------------------------------------------------------------------------------------

// L'animateur conduit les 6 boutons d'étape ; jamais ROUE_SET_TEAMS / ROUE_PASS / ROUE_REVEAL.
func TestRoueWire144_Anim_CanSendStepButtons(t *testing.T) {
	w := newRoueWire(t, game.RoueModeTantQueJeGagne)
	w.send(w.anim, protocol.ActionRoueSpin, struct{}{})
	if w.sub() != game.RoueSubSpinning {
		t.Fatalf("ROUE_SPIN de l'animateur doit s'exécuter : %q", w.sub())
	}
	w.send(w.anim, protocol.ActionRoueStop, struct{}{})
	w.waitSub(game.RoueSubLetter, 5*time.Second)
	w.send(w.anim, protocol.ActionRoueLetter, protocol.RoueLetterPayload{Letter: "N"})
	w.send(w.anim, protocol.ActionRoueFlip, struct{}{})
	w.waitSub(game.RoueSubAnswer, 5*time.Second)
	w.send(w.anim, protocol.ActionRoueNext, protocol.RoueNextPayload{Hand: game.RoueHandSame})
	if w.sub() != game.RoueSubTurn {
		t.Errorf("ROUE_NEXT de l'animateur doit s'exécuter : %q", w.sub())
	}
}

func TestRoueWire144_Anim_CannotSetTeamsPassOrReveal(t *testing.T) {
	w := newRoueWire(t, game.RoueModeChacunSonTour)
	before := w.app.engine.GetState()
	w.send(w.anim, protocol.ActionRoueSetTeams, protocol.RoueSetTeamsPayload{Teams: []string{"blue", "red"}})
	w.send(w.anim, protocol.ActionRouePass, struct{}{})
	w.send(w.anim, protocol.ActionRoueReveal, struct{}{})
	after := w.app.engine.GetState()
	if after.RoueCurrentTeam != before.RoueCurrentTeam || after.RoueSubPhase != game.RoueSubTurn || after.RoueLastEvent.Seq != before.RoueLastEvent.Seq {
		t.Errorf("les actions admin-seul de l'animateur n'ont aucun effet : main %s→%s sous-phase=%s event=%+v",
			before.RoueCurrentTeam, after.RoueCurrentTeam, after.RoueSubPhase, after.RoueLastEvent)
	}
}

// TV et tablette n'envoient aucune action ROUE_* (la tablette n'envoie que BUTTON).
func TestRoueWire144_TVAndTablet_CannotSendRoueActions(t *testing.T) {
	w := newRoueWire(t, game.RoueModeTantQueJeGagne)
	for _, conn := range []*websocket.Conn{w.tv, w.vp} {
		w.send(conn, protocol.ActionRoueSpin, struct{}{})
		w.send(conn, protocol.ActionRoueReveal, struct{}{})
	}
	if w.sub() != game.RoueSubTurn {
		t.Errorf("aucune action ROUE_* d'une TV / tablette ne doit s'exécuter : %q", w.sub())
	}
}

// ---- ROUE_ERROR : à l'émetteur seul ------------------------------------------------------------------------------

func roueErrorOf(frames []string) (code string, found bool) {
	for _, f := range frames {
		if actionOf(f) == protocol.ActionRoueError {
			var env struct {
				Msg struct {
					Code string `json:"CODE"`
				} `json:"MSG"`
			}
			_ = json.Unmarshal([]byte(f), &env)
			return env.Msg.Code, true
		}
	}
	return "", false
}

func TestRoueWire144_RefusalAnswersROUE_ERROR_ToTheEmitterOnly(t *testing.T) {
	w := newRoueWire(t, game.RoueModeTantQueJeGagne)
	w.send(w.admin, protocol.ActionRoueSpin, struct{}{})
	w.send(w.admin, protocol.ActionRoueStop, struct{}{})
	w.waitSub(game.RoueSubLetter, 5*time.Second)
	w.adminTap.take(30 * time.Millisecond)
	w.animTap.take(0)
	w.tvTap.take(0)

	w.send(w.admin, protocol.ActionRoueLetter, protocol.RoueLetterPayload{Letter: "A"}) // voyelle
	if code, ok := roueErrorOf(w.adminTap.take(200 * time.Millisecond)); !ok || code != game.RoueErrVowelNotAvailable {
		t.Errorf("l'admin doit recevoir ROUE_ERROR VOWEL_NOT_AVAILABLE, got %q found=%v", code, ok)
	}
	if _, ok := roueErrorOf(w.animTap.take(50 * time.Millisecond)); ok {
		t.Errorf("l'animateur ne doit pas recevoir le ROUE_ERROR de l'admin")
	}
	if _, ok := roueErrorOf(w.tvTap.take(50 * time.Millisecond)); ok {
		t.Errorf("la TV ne doit jamais recevoir de ROUE_ERROR")
	}
	if w.sub() != game.RoueSubLetter {
		t.Errorf("un refus ne change pas l'état : %q", w.sub())
	}
	w.send(w.admin, protocol.ActionRoueLetter, protocol.RoueLetterPayload{Letter: "7"})
	if code, ok := roueErrorOf(w.adminTap.take(200 * time.Millisecond)); !ok || code != game.RoueErrNotALetter {
		t.Errorf("NOT_A_LETTER attendu, got %q found=%v", code, ok)
	}
}

// Double commande : le même geste d'admin et d'animateur — appliqué une fois, l'autre reçoit WRONG_SUBPHASE.
func TestRoueWire144_DoubleCommand_SecondEmitterGetsWRONG_SUBPHASE(t *testing.T) {
	w := newRoueWire(t, game.RoueModeTantQueJeGagne)
	w.adminTap.take(30 * time.Millisecond)
	w.animTap.take(0)
	w.send(w.admin, protocol.ActionRoueSpin, struct{}{})
	w.send(w.anim, protocol.ActionRoueSpin, struct{}{}) // arrive après : refusé
	if w.app.engine.GetState().RoueLastEvent.Seq != 1 {
		t.Errorf("un seul lancer appliqué : seq=%d", w.app.engine.GetState().RoueLastEvent.Seq)
	}
	if _, ok := roueErrorOf(w.adminTap.take(150 * time.Millisecond)); ok {
		t.Errorf("le premier émetteur n'a pas d'erreur")
	}
	if code, ok := roueErrorOf(w.animTap.take(150 * time.Millisecond)); !ok || code != game.RoueErrWrongSubPhase {
		t.Errorf("le second émetteur reçoit WRONG_SUBPHASE, got %q found=%v", code, ok)
	}
}

// ---- diffusion --------------------------------------------------------------------------------------------------

// Chaque transition diffuse un UPDATE à tous les clients web (admin, anim, TV, tablette) avec ROUE_SUBPHASE.
func TestRoueWire144_EveryTransitionBroadcastsAnUpdateToAllWebClients(t *testing.T) {
	w := newRoueWire(t, game.RoueModeTantQueJeGagne)
	for _, tp := range []*tap{w.adminTap, w.animTap, w.tvTap, w.vpTap} {
		tp.take(30 * time.Millisecond)
	}
	w.send(w.admin, protocol.ActionRoueSpin, struct{}{})
	for name, tp := range map[string]*tap{"admin": w.adminTap, "anim": w.animTap, "tv": w.tvTap, "vp": w.vpTap} {
		got := false
		for _, f := range tp.take(250 * time.Millisecond) {
			if actionOf(f) == protocol.ActionUpdate && strings.Contains(f, `"ROUE_SUBPHASE":"SPINNING"`) {
				got = true
			}
		}
		if !got {
			t.Errorf("%s doit recevoir un UPDATE SPINNING après ROUE_SPIN", name)
		}
	}
}

// Les champs ROUE_* ne sont jamais diffusés aux buzzers (contrat §4).
func TestRoueWire144_BuzzerFramesCarryNoRoueField(t *testing.T) {
	w := newRoueWire(t, game.RoueModeSolo)
	data, err := func() ([]byte, error) {
		msg, _ := protocol.NewMessage(protocol.ActionUpdate, nil)
		msg.Msg = w.app.engine.GetGameJSON()
		return msg.SerializeForBuzzer()
	}()
	if err != nil {
		t.Fatalf("SerializeForBuzzer : %v", err)
	}
	if strings.Contains(string(data), "ROUE_") || strings.Contains(string(data), roueWirePhrase) {
		t.Errorf("un buzzer ne reçoit jamais de champ ROUE_* ni la phrase : %s", data)
	}
}
