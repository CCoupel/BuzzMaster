package main

// ---------------------------------------------------------------------------
// #318 (v13.1.0, LOT-3, T30) — RAFALE PICTO de bout en bout sur la vraie WebSocket, écrit depuis contracts/picto.md
// §17 (le code n'était pas final : seuls les noms d'actions et de champs du contrat sont utilisés, en littéraux ; la
// question est construite en JSON, donc aucun symbole Go à venir n'est référencé). Harnais de
// picto_wire_192_193_test.go (copié car sa question est figée). Tests `critical` : EPOCH / STALE_WORD, limite de
// passes, PICTO_PASS d'une tablette rejeté, non-fuite des mots, plancher 0 du crédit.
// ---------------------------------------------------------------------------

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

const pictoActPass = "PICTO_PASS"

var rafalePictoWords = []string{"Zèbre-secret", "Pont-levis-secret", "Cornemuse-secret", "Phare-secret", "Tricorne-secret"}

// rafaleQuestion construit une question RAFALE_PICTO par JSON (champs du contrat §17.2).
func rafaleQuestion(t *testing.T, over map[string]interface{}) *game.Question {
	t.Helper()
	m := map[string]interface{}{
		"ID": "rpicto-wire-q", "QUESTION": "Faites deviner", "TYPE": "RAFALE_PICTO", "CATEGORY": "HISTORY",
		"POINTS": "10", "TIME": "120",
		"RAFALE_PICTO_SOURCE": "WORDS", "RAFALE_PICTO_WORDS": rafalePictoWords,
		"RAFALE_PICTO_PASS_COST": 0, "RAFALE_PICTO_PASS_LIMIT": 0, "RAFALE_PICTO_POOL_END": "END",
	}
	for k, v := range over {
		m[k] = v
	}
	raw, _ := json.Marshal(m)
	var q game.Question
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	return &q
}

// newRafalePictoWire monte le même décor que newPictoWire mais avec la question donnée (3 joueurs, admin, anim, TV).
func newRafalePictoWire(t *testing.T, q *game.Question, seed func(*App)) *pictoWire {
	t.Helper()
	app := newBroadcast127TestApp(t)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	app.setupPictoCallbacks()
	app.wsHub.OnPlayerDisconnected = app.onPlayerDisconnected
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}, "TeamB": {Name: "TeamB"}})
	w := &pictoWire{app: app}
	w.aliceID = setupVirtualPlayer(t, app, "Alice", "TeamA")
	w.bobID = setupVirtualPlayer(t, app, "Bob", "TeamA")
	w.carolID = setupVirtualPlayer(t, app, "Carol", "TeamB")
	app.engine.SetPhase(game.PhaseStopped)
	if seed != nil {
		seed(app)
	}
	app.engine.Ready(q.ID, q)
	w.question = q

	base := startAnimAllowlistTestServer(t, app)
	dial := func(name, path string) *wireClient {
		c := dialWS(t, base, path)
		learnClientID(t, app, c)
		return newWireClient(t, name, c)
	}
	w.admin = dial("admin", "/ws/admin")
	w.anim = dial("anim", "/ws/anim")
	w.tv = dial("tv", "/ws/tv")
	w.alice = dial("alice", "/ws/player")
	w.bob = dial("bob", "/ws/player")
	w.carol = dial("carol", "/ws/player")
	for _, p := range []struct {
		c    *wireClient
		name string
		id   string
	}{{w.alice, "Alice", w.aliceID}, {w.bob, "Bob", w.bobID}, {w.carol, "Carol", w.carolID}} {
		w.send(t, p.c, protocol.ActionPlayerConnect, protocol.PlayerConnectPayload{Name: p.name, ID: p.id})
		p.c.waitAction(t, "PLAYER_CONNECTED", 1, 3*time.Second)
	}
	return w
}

// startedRafale : question, équipe et dessinateur choisis, manche lancée ; retourne le premier mot.
func startedRafale(t *testing.T, over map[string]interface{}) (*pictoWire, string) {
	t.Helper()
	w := newRafalePictoWire(t, rafaleQuestion(t, over), nil)
	w.chooseTeamAndDrawer(t, w.aliceID)
	return w, w.startRoundWithDelay(t, w.alice, 120)
}

func (w *pictoWire) epoch(t *testing.T) int {
	t.Helper()
	return int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
}

func (w *pictoWire) num(t *testing.T, field string) int {
	t.Helper()
	v, _ := gameField(t, w.admin, field).(float64)
	return int(v)
}

func (w *pictoWire) waitNum(t *testing.T, field string, want int) {
	t.Helper()
	waitGameField(t, w.admin, field, float64(want), 3*time.Second)
}

// lastWord : mot et époque du dernier PICTO_WORD reçu par la tablette du dessinateur.
func (w *pictoWire) lastWord(t *testing.T) (string, int, int) {
	t.Helper()
	fr := w.alice.of(pictoActWord)
	var p struct {
		Msg struct {
			Word  string `json:"WORD"`
			Epoch int    `json:"EPOCH"`
		} `json:"MSG"`
	}
	if len(fr) == 0 || json.Unmarshal([]byte(fr[len(fr)-1]), &p) != nil {
		t.Fatalf("aucun PICTO_WORD reçu par le dessinateur")
	}
	return p.Msg.Word, p.Msg.Epoch, len(fr)
}

func (w *pictoWire) nextWord(t *testing.T, afterCount int) (string, int) {
	t.Helper()
	fr := w.alice.waitAction(t, pictoActWord, afterCount+1, 5*time.Second)
	_ = fr
	word, ep, _ := w.lastWord(t)
	return word, ep
}

func (w *pictoWire) foundBy(t *testing.T, c *wireClient, epoch int) {
	t.Helper()
	w.send(t, c, pictoActFound, map[string]interface{}{"EPOCH": epoch})
}

func (w *pictoWire) passBy(t *testing.T, c *wireClient, epoch int) {
	t.Helper()
	w.send(t, c, pictoActPass, map[string]interface{}{"EPOCH": epoch})
}

func stringList(v interface{}) []string {
	var out []string
	l, _ := v.([]interface{})
	for _, x := range l {
		s, _ := x.(string)
		out = append(out, s)
	}
	return out
}

// ---------------------------------------------------------------- §17.5 Trouvé : mot suivant dans la même action

func TestRafalePicto318_Critical_Found_AdvancesToNextWordInSameAction(t *testing.T) {
	w, first := startedRafale(t, nil)
	ep1 := w.epoch(t)
	_, _, n := w.lastWord(t)
	w.foundBy(t, w.admin, ep1)
	second, ep2 := w.nextWord(t, n)
	if second == first {
		t.Errorf("mot suivant = mot trouvé (%q) : un mot trouvé ne revient jamais", first)
	}
	if ep2 != ep1+1 {
		t.Errorf("époque %d → %d, attendu +1 par mot", ep1, ep2)
	}
	w.waitNum(t, "PICTO_FOUND_COUNT", 1)
	if got := stringList(gameField(t, w.admin, "PICTO_FOUND_WORDS")); len(got) != 1 || got[0] != first {
		t.Errorf("PICTO_FOUND_WORDS = %v, attendu [%s]", got, first)
	}
	if ph := gameField(t, w.admin, "PHASE"); ph != "STARTED" {
		t.Errorf("la manche continue : phase = %v", ph)
	}
	// le mot suivant n'est donné qu'au dessinateur, à l'admin et à l'anim
	for _, c := range []*wireClient{w.tv, w.bob, w.carol} {
		if c.containsAnywhere(second) {
			t.Errorf("%s a vu le mot en cours %q", c.name, second)
		}
	}
	w.anim.waitAction(t, pictoActWord, 1, 3*time.Second)
}

func TestRafalePicto318_RoundPoints_FoundTimesPointsMinusPassesTimesCost_Floor0(t *testing.T) {
	w, _ := startedRafale(t, map[string]interface{}{"RAFALE_PICTO_PASS_COST": 15})
	w.waitNum(t, "PICTO_ROUND_POINTS", 0)
	_, ep, n := w.lastWord(t)
	w.foundBy(t, w.admin, ep)
	w.waitNum(t, "PICTO_ROUND_POINTS", 10)
	_, ep = w.nextWord(t, n)
	_, _, n = w.lastWord(t)
	w.passBy(t, w.anim, ep) // 10 − 15 < 0 : plancher 0
	w.waitNum(t, "PICTO_PASS_COUNT", 1)
	w.waitNum(t, "PICTO_ROUND_POINTS", 0)
	_, ep = w.nextWord(t, n)
	_, _, n = w.lastWord(t)
	w.foundBy(t, w.admin, ep)
	w.foundBy(t, w.admin, 0) // époque périmée : sans effet
	w.waitNum(t, "PICTO_FOUND_COUNT", 2)
	w.waitNum(t, "PICTO_ROUND_POINTS", 5) // 2×10 − 1×15
	if v := w.num(t, "PICTO_ROUND_POINTS"); v < 0 {
		t.Errorf("PICTO_ROUND_POINTS négatif : %d", v)
	}
}

func TestRafalePicto318_UnlimitedPassCostFreeFloorsAtZero_NeverNegative(t *testing.T) {
	w, _ := startedRafale(t, map[string]interface{}{"RAFALE_PICTO_PASS_COST": 100, "RAFALE_PICTO_PASS_LIMIT": 0})
	for i := 0; i < 3; i++ {
		_, ep, n := w.lastWord(t)
		w.passBy(t, w.admin, ep)
		w.nextWord(t, n)
	}
	w.waitNum(t, "PICTO_PASS_COUNT", 3)
	w.waitNum(t, "PICTO_ROUND_POINTS", 0)
	w.waitNum(t, "PICTO_PASSES_LEFT", -1) // illimité
}

// ---------------------------------------------------------------- limite de passes

func TestRafalePicto318_Critical_PassLimitOne_SecondPassRefused_SenderOnly(t *testing.T) {
	w, _ := startedRafale(t, map[string]interface{}{"RAFALE_PICTO_PASS_LIMIT": 1})
	w.waitNum(t, "PICTO_PASSES_LEFT", 1)
	_, ep, n := w.lastWord(t)
	w.passBy(t, w.admin, ep)
	w.waitNum(t, "PICTO_PASSES_LEFT", 0)
	w.waitNum(t, "PICTO_PASS_COUNT", 1)
	_, ep = w.nextWord(t, n)
	nErr := len(w.anim.of(pictoActError))
	w.passBy(t, w.anim, ep)
	got := w.anim.waitAction(t, pictoActError, nErr+1, 3*time.Second)
	if errorCode(got[len(got)-1]) != "PASS_LIMIT_REACHED" {
		t.Errorf("2ᵉ passe avec limite 1 : PASS_LIMIT_REACHED attendu, got %v", got[len(got)-1])
	}
	w.admin.never(t, pictoActError, 150*time.Millisecond) // l'erreur va à l'émetteur seul
	w.tv.never(t, pictoActError, 0)
	if w.num(t, "PICTO_PASS_COUNT") != 1 || w.num(t, "PICTO_PASSES_LEFT") != 0 {
		t.Errorf("une passe refusée ne change rien")
	}
	// la passe refusée n'a pas fait avancer le mot
	if _, ep2, _ := w.lastWord(t); ep2 != ep {
		t.Errorf("époque changée par une passe refusée : %d → %d", ep, ep2)
	}
}

func TestRafalePicto318_PassesLeft_StartsAtLimitOrMinusOne(t *testing.T) {
	for limit, want := range map[int]int{0: -1, 1: 1} {
		w, _ := startedRafale(t, map[string]interface{}{"RAFALE_PICTO_PASS_LIMIT": limit})
		w.waitNum(t, "PICTO_PASSES_LEFT", want)
	}
}

// ---------------------------------------------------------------- EPOCH et concurrence admin ‖ anim

func TestRafalePicto318_Critical_StaleEpoch_Refused_STALE_WORD_NoEffect(t *testing.T) {
	w, _ := startedRafale(t, nil)
	_, ep, n := w.lastWord(t)
	w.foundBy(t, w.admin, ep)
	w.nextWord(t, n)
	w.waitNum(t, "PICTO_FOUND_COUNT", 1)
	nErr := len(w.anim.of(pictoActError))
	w.foundBy(t, w.anim, ep) // époque périmée : l'animateur n'avait pas vu le mot suivant
	got := w.anim.waitAction(t, pictoActError, nErr+1, 3*time.Second)
	if errorCode(got[len(got)-1]) != "STALE_WORD" {
		t.Errorf("époque périmée : STALE_WORD attendu, got %v", got[len(got)-1])
	}
	w.admin.never(t, pictoActError, 100*time.Millisecond)
	if w.num(t, "PICTO_FOUND_COUNT") != 1 {
		t.Errorf("une action périmée ne compte rien : PICTO_FOUND_COUNT = %d", w.num(t, "PICTO_FOUND_COUNT"))
	}
	// idem pour une passe périmée
	nErr = len(w.admin.of(pictoActError))
	w.passBy(t, w.admin, ep)
	got = w.admin.waitAction(t, pictoActError, nErr+1, 3*time.Second)
	if errorCode(got[len(got)-1]) != "STALE_WORD" || w.num(t, "PICTO_PASS_COUNT") != 0 {
		t.Errorf("passe périmée : STALE_WORD attendu et aucune passe comptée (got %v, passes=%d)", got[len(got)-1], w.num(t, "PICTO_PASS_COUNT"))
	}
}

func TestRafalePicto318_Critical_AdminAndAnimSimultaneous_OnlyOneWordAdvances(t *testing.T) {
	w, _ := startedRafale(t, nil)
	ep := w.epoch(t)
	w.foundBy(t, w.admin, ep)
	w.foundBy(t, w.anim, ep) // même mot, même époque : un seul avance
	w.waitNum(t, "PICTO_FOUND_COUNT", 1)
	time.Sleep(300 * time.Millisecond)
	if n := w.num(t, "PICTO_FOUND_COUNT"); n != 1 {
		t.Errorf("deux gestes simultanés : PICTO_FOUND_COUNT = %d, attendu 1", n)
	}
	if got := w.anim.of(pictoActError); len(got) == 0 || errorCode(got[len(got)-1]) != "STALE_WORD" {
		t.Errorf("le second geste doit recevoir STALE_WORD : %v", got)
	}
	// « Trouvé » et « Passer » simultanés sur le même mot : un seul gagne
	w2, _ := startedRafale(t, nil)
	ep = w2.epoch(t)
	w2.foundBy(t, w2.admin, ep)
	w2.passBy(t, w2.anim, ep)
	time.Sleep(300 * time.Millisecond)
	if w2.num(t, "PICTO_FOUND_COUNT")+w2.num(t, "PICTO_PASS_COUNT") != 1 {
		t.Errorf("Trouvé ‖ Passer sur le même mot : un seul doit compter (trouvés=%d passes=%d)", w2.num(t, "PICTO_FOUND_COUNT"), w2.num(t, "PICTO_PASS_COUNT"))
	}
}

func TestRafalePicto318_EpochIsMandatory_MissingEpochRefused(t *testing.T) {
	w, _ := startedRafale(t, nil)
	nErr := len(w.admin.of(pictoActError))
	w.send(t, w.admin, pictoActFound, struct{}{})
	w.admin.waitAction(t, pictoActError, nErr+1, 3*time.Second)
	nErr = len(w.admin.of(pictoActError))
	w.send(t, w.admin, pictoActPass, struct{}{})
	w.admin.waitAction(t, pictoActError, nErr+1, 3*time.Second)
	if w.num(t, "PICTO_FOUND_COUNT") != 0 || w.num(t, "PICTO_PASS_COUNT") != 0 {
		t.Errorf("EPOCH est obligatoire en Rafale Picto : sans lui, aucun effet")
	}
}

// ---------------------------------------------------------------- émetteurs : admin et anim seulement

func TestRafalePicto318_Critical_PassFromTablet_Rejected_NoVplayerAction(t *testing.T) {
	w, _ := startedRafale(t, nil)
	ep := w.epoch(t)
	for _, c := range []*wireClient{w.alice, w.bob, w.carol} {
		w.passBy(t, c, ep)
		w.foundBy(t, c, ep)
	}
	time.Sleep(400 * time.Millisecond)
	if w.num(t, "PICTO_PASS_COUNT") != 0 || w.num(t, "PICTO_FOUND_COUNT") != 0 {
		t.Errorf("une tablette (même le dessinateur) ne passe pas et ne valide pas : le dessinateur annonce à l'oral (passes=%d trouvés=%d)", w.num(t, "PICTO_PASS_COUNT"), w.num(t, "PICTO_FOUND_COUNT"))
	}
	if w.epoch(t) != ep {
		t.Errorf("l'époque ne doit pas avancer sur une action de tablette")
	}
}

func TestRafalePicto318_Pass_OnPictoOneWordQuestion_NOT_RAFALE(t *testing.T) {
	w := newPictoWire(t)
	w.roundStartedWithDelay(t, 120)
	nErr := len(w.admin.of(pictoActError))
	w.send(t, w.admin, pictoActPass, map[string]interface{}{"EPOCH": w.epoch(t)})
	got := w.admin.waitAction(t, pictoActError, nErr+1, 3*time.Second)
	if errorCode(got[len(got)-1]) != "NOT_RAFALE" {
		t.Errorf("PICTO_PASS sur une question Picto : NOT_RAFALE attendu, got %v", got[len(got)-1])
	}
	if ph := gameField(t, w.admin, "PHASE"); ph != "STARTED" {
		t.Errorf("la manche Picto n'est pas affectée : phase = %v", ph)
	}
	if v, _ := gameField(t, w.admin, "PICTO_PASSES_LEFT").(float64); v != -1 {
		t.Errorf("Picto un mot : PICTO_PASSES_LEFT = %v, attendu -1", v)
	}
}

func TestRafalePicto318_Pass_OutsideStarted_WrongPhase(t *testing.T) {
	w := newRafalePictoWire(t, rafaleQuestion(t, nil), nil)
	nErr := len(w.admin.of(pictoActError))
	w.send(t, w.admin, pictoActPass, map[string]interface{}{"EPOCH": 0})
	got := w.admin.waitAction(t, pictoActError, nErr+1, 3*time.Second)
	if errorCode(got[len(got)-1]) != "WRONG_PHASE" {
		t.Errorf("PICTO_PASS en PREPARE : WRONG_PHASE attendu, got %v", got[len(got)-1])
	}
}

// ---------------------------------------------------------------- rebouclage LOOP / POOL_END

func TestRafalePicto318_PoolEnd_LOOP_OldestPassedComesBack_FIFO_FoundNeverReturns(t *testing.T) {
	w, _ := startedRafale(t, map[string]interface{}{"RAFALE_PICTO_WORDS": []string{"Alpha-un", "Beta-deux", "Gamma-trois"}, "RAFALE_PICTO_POOL_END": "LOOP"})
	var order []string
	pass := func() {
		word, ep, n := w.lastWord(t)
		order = append(order, word)
		w.passBy(t, w.admin, ep)
		w.nextWord(t, n)
	}
	// trouver le premier mot, passer les deux autres : plus aucun mot frais
	word, ep, n := w.lastWord(t)
	found := word
	w.foundBy(t, w.admin, ep)
	w.nextWord(t, n)
	pass()
	pass()
	back, _, _ := w.lastWord(t)
	if back != order[0] {
		t.Errorf("LOOP : le plus ancien mot passé (%q) doit revenir en premier (FIFO), reçu %q (passés : %v)", order[0], back, order)
	}
	if back == found {
		t.Errorf("un mot trouvé ne revient jamais (%q)", found)
	}
	if ph := gameField(t, w.admin, "PHASE"); ph != "STARTED" {
		t.Errorf("avec LOOP la manche continue : phase = %v", ph)
	}
}

func TestRafalePicto318_PoolEnd_END_EndsRound_OutcomePOOL_END_CueFinLot(t *testing.T) {
	w, _ := startedRafale(t, map[string]interface{}{"RAFALE_PICTO_WORDS": []string{"Alpha-un", "Beta-deux"}, "RAFALE_PICTO_POOL_END": "END"})
	for i := 0; i < 2; i++ {
		_, ep, n := w.lastWord(t)
		w.foundBy(t, w.admin, ep)
		if i == 0 {
			w.nextWord(t, n)
		}
	}
	waitGameField(t, w.admin, "PICTO_OUTCOME", "POOL_END", 3*time.Second)
	waitPhase(t, w, "STOPPED")
	w.waitNum(t, "PICTO_FOUND_COUNT", 2)
	w.waitNum(t, "PICTO_ROUND_POINTS", 20)
	// aucun mot fusionné dans l'état public sauf les mots trouvés
	for _, c := range []*wireClient{w.tv, w.bob, w.carol} {
		if c.containsAnywhere(`"PICTO_REVEALED_WORD":"Alpha`) || c.containsAnywhere(`"PICTO_REVEALED_WORD":"Beta`) {
			t.Errorf("%s : après POOL_END, PICTO_REVEALED_WORD doit rester vide (aucun mot en cours)", c.name)
		}
	}
}

// ---------------------------------------------------------------- gong, fin de manche, relance, déconnexion

func TestRafalePicto318_Gong_FoundOnTimeout_CountsCurrentWordOnce_ThenReveals(t *testing.T) {
	w := newRafalePictoWire(t, rafaleQuestion(t, map[string]interface{}{"TIME": "30"}), nil)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRoundWithDelay(t, w.alice, 2)
	w.waitTimeout(t)
	_, ep, _ := w.lastWord(t)
	w.foundBy(t, w.anim, ep) // « Trouvé » sur le gong : compte le mot en cours et révèle dans la même action
	waitPhase(t, w, "REVEALED")
	w.waitNum(t, "PICTO_FOUND_COUNT", 1)
	waitGameField(t, w.admin, "PICTO_OUTCOME", "TIMEOUT", 3*time.Second) // l'issue de la manche reste le temps écoulé
	w.foundBy(t, w.admin, ep)                                            // une seule fois
	time.Sleep(200 * time.Millisecond)
	if w.num(t, "PICTO_FOUND_COUNT") != 1 {
		t.Errorf("« Trouvé » sur le gong ne compte qu'une fois : %d", w.num(t, "PICTO_FOUND_COUNT"))
	}
}

func TestRafalePicto318_Revealed_PassedWordsNeverPublished_RecapIsPublic(t *testing.T) {
	w, _ := startedRafale(t, nil)
	_, ep, n := w.lastWord(t)
	passed, _, _ := w.lastWord(t)
	w.passBy(t, w.admin, ep)
	w.nextWord(t, n)
	_, ep, n = w.lastWord(t)
	found, _, _ := w.lastWord(t)
	w.foundBy(t, w.admin, ep)
	w.nextWord(t, n)
	w.send(t, w.admin, protocol.ActionStop, struct{}{})
	waitPhase(t, w, "STOPPED")
	w.send(t, w.admin, protocol.ActionReveal, struct{}{})
	waitPhase(t, w, "REVEALED")
	current, _, _ := w.lastWord(t)
	waitGameField(t, w.tv, "PICTO_REVEALED_WORD", current, 3*time.Second)
	for _, c := range []*wireClient{w.tv, w.bob, w.carol} {
		if c.containsAnywhere(passed) {
			t.Errorf("%s : le mot passé %q ne doit jamais être publié", c.name, passed)
		}
		if !c.containsAnywhere(found) {
			t.Errorf("%s : le mot trouvé %q est public (récapitulatif)", c.name, found)
		}
	}
	waitGameField(t, w.tv, "PICTO_FOUND_COUNT", float64(1), 3*time.Second)
	waitGameField(t, w.tv, "PICTO_PASS_COUNT", float64(1), 3*time.Second)
}

func TestRafalePicto318_Relaunch_ResetsCountersQueueAndFoundWords(t *testing.T) {
	w, _ := startedRafale(t, map[string]interface{}{"RAFALE_PICTO_PASS_LIMIT": 1})
	_, ep, n := w.lastWord(t)
	w.passBy(t, w.admin, ep)
	_, ep = w.nextWord(t, n)
	_, _, n = w.lastWord(t)
	w.foundBy(t, w.admin, ep)
	w.nextWord(t, n)
	w.send(t, w.admin, protocol.ActionStop, struct{}{})
	waitPhase(t, w, "STOPPED")
	w.send(t, w.admin, protocol.ActionReveal, struct{}{})
	waitPhase(t, w, "REVEALED")

	w.app.engine.Ready(w.question.ID, w.question) // relance : resélection de la même question
	waitPhase(t, w, "PREPARE")
	for _, f := range []string{"PICTO_FOUND_COUNT", "PICTO_PASS_COUNT", "PICTO_ROUND_POINTS"} {
		if w.num(t, f) != 0 {
			t.Errorf("relance : %s doit être remis à 0, got %d", f, w.num(t, f))
		}
	}
	if got := stringList(gameField(t, w.admin, "PICTO_FOUND_WORDS")); len(got) != 0 {
		t.Errorf("relance : PICTO_FOUND_WORDS doit être vide, got %v", got)
	}
	if gameStr(t, w.admin, "PICTO_TEAM") != "TeamA" || gameStr(t, w.admin, "PICTO_DRAWER") != w.aliceID {
		t.Errorf("relance : équipe et dessinateur conservés (§6.6)")
	}
	nWords := len(w.alice.of(pictoActWord))
	w.send(t, w.admin, protocol.ActionForceReady, struct{}{})
	waitPhase(t, w, "READY")
	w.send(t, w.admin, protocol.ActionStart, protocol.StartPayload{Delay: 120})
	w.alice.waitAction(t, pictoActWord, nWords+1, 15*time.Second)
	w.waitNum(t, "PICTO_PASSES_LEFT", 1) // la limite est rendue
}

func TestRafalePicto318_DrawerDisconnected_RoundContinues_AdminAnimKeepPlaying_ReconnectResendsWord(t *testing.T) {
	w, _ := startedRafale(t, nil)
	w.alice.conn.Close()
	time.Sleep(400 * time.Millisecond)
	waitGameField(t, w.admin, "PICTO_DRAWER_CONNECTED", false, 3*time.Second)
	ep := w.epoch(t)
	w.foundBy(t, w.admin, ep) // le dessinateur est parti : l'admin et l'anim continuent
	w.waitNum(t, "PICTO_FOUND_COUNT", 1)
	ep = w.epoch(t)
	w.passBy(t, w.anim, ep)
	w.waitNum(t, "PICTO_PASS_COUNT", 1)
	if ph := gameField(t, w.admin, "PHASE"); ph != "STARTED" {
		t.Errorf("la manche continue sans le dessinateur : phase = %v", ph)
	}
	// un admin qui se reconnecte reçoit le mot EN COURS (et sa feuille)
	base := startAnimAllowlistTestServer(t, w.app)
	c := newWireClient(t, "admin-2", dialWS(t, base, "/ws/admin"))
	learnClientID(t, w.app, c.conn)
	w.send(t, c, "HELLO", map[string]interface{}{})
	fr := c.waitAction(t, pictoActWord, 1, 5*time.Second)
	var p struct {
		Msg struct {
			Word  string `json:"WORD"`
			Epoch int    `json:"EPOCH"`
		} `json:"MSG"`
	}
	_ = json.Unmarshal([]byte(fr[0]), &p)
	if p.Msg.Epoch != w.epoch(t) {
		t.Errorf("renvoi à la reconnexion : époque %d, attendu l'époque courante %d", p.Msg.Epoch, w.epoch(t))
	}
}

// ---------------------------------------------------------------- confidentialité : manche de 5 mots, deux sources

func assertRafaleNoLeak(t *testing.T, w *pictoWire, all []string, label string) {
	t.Helper()
	publicFound := map[string]bool{}
	for _, wd := range stringList(gameField(t, w.tv, "PICTO_FOUND_WORDS")) {
		publicFound[wd] = true
	}
	for _, c := range []*wireClient{w.tv, w.bob, w.carol} {
		for _, key := range []string{`"RAFALE_PICTO_WORDS"`, `"PICTO_WORDS"`} {
			if c.containsAnywhere(key) {
				t.Errorf("%s/%s : %s fuit sur %s", label, c.name, key, c.name)
			}
		}
		for _, wd := range all {
			if publicFound[wd] {
				continue // un mot TROUVÉ est public (dit à voix haute, §17.9)
			}
			if c.containsAnywhere(wd) {
				t.Errorf("%s/%s : le mot %q (non trouvé) fuit", label, c.name, wd)
			}
		}
	}
}

func TestRafalePicto318_Critical_NoWordLeak_FiveWords_BothSources(t *testing.T) {
	for _, source := range []string{"WORDS", "RESERVOIR"} {
		t.Run(source, func(t *testing.T) {
			over := map[string]interface{}{"RAFALE_PICTO_POOL_END": "LOOP"}
			var seed func(*App)
			if source == "RESERVOIR" {
				over["RAFALE_PICTO_SOURCE"] = "RESERVOIR"
				over["RAFALE_PICTO_CATEGORIES"] = []string{"HISTORY"}
				over["RAFALE_PICTO_DIFFICULTIES"] = []int{1, 2, 3}
				delete(over, "RAFALE_PICTO_WORDS")
				over["RAFALE_PICTO_WORDS"] = nil
				seed = func(app *App) {
					for i, wd := range rafalePictoWords {
						if _, err := app.engine.PictoPool().Upsert(game.PoolEntry{Question: wd, Category: game.CategoryHistory, Difficulty: i%3 + 1}); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			w := newRafalePictoWire(t, rafaleQuestion(t, over), seed)
			w.chooseTeamAndDrawer(t, w.aliceID)
			w.startRoundWithDelay(t, w.alice, 120)
			// trouvés, passés, rebouclés : 5 mots frais puis rebouclage
			for i := 0; i < 7; i++ {
				_, ep, n := w.lastWord(t)
				if i%2 == 0 {
					w.foundBy(t, w.admin, ep)
				} else {
					w.passBy(t, w.anim, ep)
				}
				if ph := gameField(t, w.admin, "PHASE"); ph == "STOPPED" {
					break
				}
				w.alice.waitAction(t, pictoActWord, n+1, 5*time.Second)
			}
			time.Sleep(300 * time.Millisecond)
			assertRafaleNoLeak(t, w, rafalePictoWords, source)
			// admin et anim, eux, reçoivent le mot en cours
			if !w.admin.containsAnywhere(`"PICTO_WORD"`) || !w.anim.containsAnywhere(`"PICTO_WORD"`) {
				t.Errorf("%s : admin et anim reçoivent PICTO_WORD", source)
			}
		})
	}
}

// ---------------------------------------------------------------- test de famille : les règles Picto valent pour RAFALE_PICTO

func TestRafalePicto318_Family_PictoRulesApplyToRafalePicto(t *testing.T) {
	// 1. la garde de START : un dessinateur est exigé (NO_DRAWER)
	w := newRafalePictoWire(t, rafaleQuestion(t, nil), nil)
	w.send(t, w.admin, pictoActSetTeam, map[string]interface{}{"TEAM": "TeamA"})
	waitGameField(t, w.admin, "PICTO_TEAM", "TeamA", 3*time.Second)
	w.send(t, w.admin, protocol.ActionForceReady, struct{}{})
	waitGameField(t, w.admin, "PHASE", "READY", 3*time.Second)
	nErr := len(w.admin.of(pictoActError))
	w.send(t, w.admin, protocol.ActionStart, protocol.StartPayload{Delay: 120})
	// NO_DRAWER parmi les erreurs NOUVELLES (index ≥ nErr), attente bornée : une autre erreur ne doit pas masquer celle-ci
	deadline := time.Now().Add(5 * time.Second)
	sawNoDrawer := false
	var seen []string
	for time.Now().Before(deadline) && !sawNoDrawer {
		errs := w.admin.of(pictoActError)
		seen = seen[:0]
		for _, f := range errs[min(nErr, len(errs)):] {
			seen = append(seen, errorCode(f))
			if errorCode(f) == "NO_DRAWER" {
				sawNoDrawer = true
			}
		}
		if !sawNoDrawer {
			time.Sleep(25 * time.Millisecond)
		}
	}
	if !sawNoDrawer {
		t.Errorf("START sans dessinateur : NO_DRAWER attendu parmi les erreurs nouvelles, reçu %v", seen)
	}

	// 2. relais de dessin : seul le dessinateur dessine, la TV reçoit ses traits, un trait d'une autre époque est refusé
	w2, _ := startedRafale(t, nil)
	ep := w2.epoch(t)
	w2.send(t, w2.alice, pictoActStrokes, strokesPayload(ep, 1, [2]int{100, 100}, [2]int{200, 200}))
	w2.tv.waitAction(t, pictoActStrokes, 1, 3*time.Second)
	nTV := len(w2.tv.of(pictoActStrokes))
	w2.send(t, w2.bob, pictoActStrokes, strokesPayload(ep, 2, [2]int{5, 5}))
	time.Sleep(300 * time.Millisecond)
	if len(w2.tv.of(pictoActStrokes)) != nTV {
		t.Errorf("un trait d'un joueur autre que le dessinateur ne doit pas être relayé")
	}
	// 3. un nouvel EPOCH par mot : aucun trait du mot précédent sur la feuille suivante
	_, _, n := w2.lastWord(t)
	w2.foundBy(t, w2.admin, ep)
	w2.alice.waitAction(t, pictoActWord, n+1, 5*time.Second)
	nTV = len(w2.tv.of(pictoActStrokes))
	w2.send(t, w2.alice, pictoActStrokes, strokesPayload(ep, 3, [2]int{9, 9})) // ancienne époque : ignoré (§8.4)
	time.Sleep(1200 * time.Millisecond)
	if len(w2.tv.of(pictoActStrokes)) != nTV {
		t.Errorf("un trait d'une époque précédente ne doit pas être relayé à la TV")
	}
	w2.send(t, w2.tv, pictoActSync, struct{}{})
	cv := w2.tv.waitAction(t, pictoActCanvas, 1, 3*time.Second)
	if strings.Contains(cv[len(cv)-1], `"PTS"`) {
		t.Errorf("la feuille du mot suivant doit être vierge : %s", cv[len(cv)-1])
	}
}

func TestRafalePicto318_BuzzerPress_Ignored_RoundContinues(t *testing.T) {
	w, _ := startedRafale(t, nil)
	w.send(t, w.bob, protocol.ActionButton, map[string]interface{}{"ID": w.bobID})
	w.send(t, w.admin, protocol.ActionButton, map[string]interface{}{"ID": w.bobID}) // simulation admin
	time.Sleep(300 * time.Millisecond)
	if ph := gameField(t, w.admin, "PHASE"); ph != "STARTED" {
		t.Errorf("appui buzzer pendant une manche Rafale Picto : phase = %v, attendu STARTED (buzzers ignorés, famille Picto)", ph)
	}
	if n := w.num(t, "PICTO_FOUND_COUNT") + w.num(t, "PICTO_PASS_COUNT"); n != 0 {
		t.Errorf("un buzzer ne compte ni trouvé ni passe : %d", n)
	}
}

func TestRafalePicto318_NonRegression_PictoOneWord_FoundStillStopsAndReveals(t *testing.T) {
	w := newPictoWire(t)
	word := w.roundStartedWithDelay(t, 120)
	w.send(t, w.admin, pictoActFound, struct{}{}) // EPOCH facultatif en Picto un mot
	waitPhase(t, w, "REVEALED")
	waitGameField(t, w.admin, "PICTO_REVEALED_WORD", word, 3*time.Second)
	if n := w.num(t, "PICTO_FOUND_COUNT"); n != 0 {
		t.Errorf("Picto un mot : les compteurs de Rafale Picto restent à 0, got %d", n)
	}
	_ = fmt.Sprint
}
