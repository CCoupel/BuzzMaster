package main

// ---------------------------------------------------------------------------
// #192 / #193 (v13.0.0, LOT-2.1) — PICTO de bout en bout sur la vraie WebSocket, écrit depuis
// contracts/picto.md §3.2, §6.1, §6.2, §6.3, §8, §9, §10 (le code n'était pas final : ces tests n'utilisent
// QUE les noms d'actions du contrat, en littéraux, et des symboles déjà existants).
//
// Harnais : newBroadcast127TestApp + startAnimAllowlistTestServer, dispatch synchrone (sendAction), un collecteur de
// trames par connexion. 3 joueurs : Alice et Bob (TeamA), Carol (TeamB) ; admin, anim, TV, 3 tablettes.
// Tests `critical` : fuite du mot, garde de PICTO_STROKES (identité tirée de la CONNEXION, jamais du payload).
// ---------------------------------------------------------------------------

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

const (
	pictoActSetTeam    = "PICTO_SET_TEAM"
	pictoActSetDrawer  = "PICTO_SET_DRAWER"
	pictoActDrawDrawer = "PICTO_DRAW_DRAWER"
	pictoActStrokes    = "PICTO_STROKES"
	pictoActSync       = "PICTO_CANVAS_SYNC"
	pictoActWord       = "PICTO_WORD"
	pictoActCanvas     = "PICTO_CANVAS"
	pictoActError      = "PICTO_ERROR"
)

var pictoWireWords = []string{"Zèbre-secret", "Pont-levis-secret", "Cornemuse-secret"}

// wireClient collecte toutes les trames reçues par une connexion.
type wireClient struct {
	name   string
	conn   *websocket.Conn
	mu     sync.Mutex
	frames []string
}

func newWireClient(t *testing.T, name string, conn *websocket.Conn) *wireClient {
	c := &wireClient{name: name, conn: conn}
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			c.mu.Lock()
			c.frames = append(c.frames, string(data))
			c.mu.Unlock()
		}
	}()
	return c
}

func (c *wireClient) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.frames...)
}

func frameAction(f string) string {
	var e struct {
		Action string `json:"ACTION"`
	}
	_ = json.Unmarshal([]byte(f), &e)
	return e.Action
}

// of retourne les trames d'une action.
func (c *wireClient) of(action string) []string {
	var out []string
	for _, f := range c.snapshot() {
		if frameAction(f) == action {
			out = append(out, f)
		}
	}
	return out
}

func (c *wireClient) waitAction(t *testing.T, action string, n int, within time.Duration) []string {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if got := c.of(action); len(got) >= n {
			return got
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s : %d trame(s) %s attendue(s) en %v, reçu %d", c.name, n, action, within, len(c.of(action)))
	return nil
}

func (c *wireClient) never(t *testing.T, action string, settle time.Duration) {
	t.Helper()
	time.Sleep(settle)
	if got := c.of(action); len(got) > 0 {
		t.Errorf("%s ne doit jamais recevoir %s : %v", c.name, action, got)
	}
}

func (c *wireClient) containsAnywhere(frag string) bool {
	for _, f := range c.snapshot() {
		if strings.Contains(f, frag) {
			return true
		}
	}
	return false
}

func errorCode(f string) string {
	var p struct {
		Msg struct {
			Code string `json:"CODE"`
		} `json:"MSG"`
	}
	_ = json.Unmarshal([]byte(f), &p)
	return p.Msg.Code
}

type pictoWire struct {
	app                      *App
	question                 *game.Question
	admin, anim, tv          *wireClient
	alice, bob, carol        *wireClient
	aliceID, bobID, carolID  string
	adminRaw, tabletRawAlice *websocket.Conn
}

func newPictoWire(t *testing.T) *pictoWire {
	t.Helper()
	app := newBroadcast127TestApp(t)
	app.setupPictoCallbacks()                                 // comme main.go:665
	app.wsHub.OnPlayerDisconnected = app.onPlayerDisconnected // comme main.go:886
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}, "TeamB": {Name: "TeamB"}})
	w := &pictoWire{app: app}
	w.aliceID = setupVirtualPlayer(t, app, "Alice", "TeamA")
	w.bobID = setupVirtualPlayer(t, app, "Bob", "TeamA")
	w.carolID = setupVirtualPlayer(t, app, "Carol", "TeamB")
	app.engine.SetPhase(game.PhaseStopped)
	q := &game.Question{
		ID: "picto-wire-q", Question: "Dessinez", Type: game.QuestionTypePicto, Category: game.CategoryHistory,
		Points: "10", Time: "90",
		TypedContent: game.TypedContent{PictoWords: pictoWireWords},
	}
	app.engine.Ready(q.ID, q) // sélection de la question : PREPARE
	w.question = q

	base := startAnimAllowlistTestServer(t, app)
	dial := func(name, path string) *wireClient {
		c := dialWS(t, base, path)
		learnClientID(t, app, c) // PONG : révèle l'inscription au hub
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

// send envoie depuis la connexion de c et dispatche le message (comme la boucle de main.go).
func (w *pictoWire) send(t *testing.T, c *wireClient, action string, payload interface{}) {
	t.Helper()
	sendAction(t, w.app, c.conn, action, payload)
}

// gameField lit un champ de GAME dans le dernier UPDATE reçu par c.
func gameField(t *testing.T, c *wireClient, field string) interface{} {
	t.Helper()
	ups := c.of(protocol.ActionUpdate)
	if len(ups) == 0 {
		t.Fatalf("%s : aucun UPDATE reçu", c.name)
	}
	var p struct {
		Msg struct {
			Game map[string]interface{} `json:"GAME"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(ups[len(ups)-1]), &p); err != nil {
		t.Fatalf("UPDATE illisible : %v", err)
	}
	return p.Msg.Game[field]
}

func waitGameField(t *testing.T, c *wireClient, field string, want interface{}, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	var got interface{}
	for time.Now().Before(deadline) {
		if len(c.of(protocol.ActionUpdate)) > 0 {
			got = gameField(t, c, field)
			if got == want {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("%s : GAME.%s = %v, attendu %v", c.name, field, got, want)
}

func (w *pictoWire) chooseTeamAndDrawer(t *testing.T, drawerID string) {
	t.Helper()
	w.send(t, w.admin, pictoActSetTeam, map[string]interface{}{"TEAM": "TeamA"})
	waitGameField(t, w.admin, "PICTO_TEAM", "TeamA", 3*time.Second)
	w.send(t, w.admin, pictoActSetDrawer, map[string]interface{}{"PLAYER": drawerID})
	waitGameField(t, w.admin, "PICTO_DRAWER", drawerID, 3*time.Second)
}

// startRound mène la manche jusqu'à STARTED (décompte compris) et attend le mot du dessinateur.
func (w *pictoWire) startRound(t *testing.T, drawer *wireClient) string {
	t.Helper()
	return w.startRoundWithDelay(t, drawer, 0) // 0 = délai par défaut du serveur (DefaultDelay, 30 s)
}

// startRoundWithDelay : `delay` est la durée de la manche (START {DELAY}, main.go handleStart) ; 0 = défaut serveur.
func (w *pictoWire) startRoundWithDelay(t *testing.T, drawer *wireClient, delay int) string {
	t.Helper()
	w.send(t, w.admin, protocol.ActionForceReady, struct{}{})
	waitGameField(t, w.admin, "PHASE", "READY", 3*time.Second)
	w.send(t, w.admin, protocol.ActionStart, protocol.StartPayload{Delay: delay})
	frames := drawer.waitAction(t, pictoActWord, 1, 15*time.Second)
	var p struct {
		Msg struct {
			Word  string `json:"WORD"`
			Epoch int    `json:"EPOCH"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(frames[0]), &p); err != nil {
		t.Fatal(err)
	}
	return p.Msg.Word
}

func strokesPayload(epoch int, id int, pts ...[2]int) map[string]interface{} {
	p := make([][2]int, len(pts))
	copy(p, pts)
	return map[string]interface{}{
		"EPOCH":   epoch,
		"STROKES": []map[string]interface{}{{"ID": id, "PTS": p, "END": true}},
	}
}

// ---------------------------------------------------------------- §6.1 préparation

func TestPictoWire192_Preparation_TeamCandidatesDrawer(t *testing.T) {
	w := newPictoWire(t)

	w.send(t, w.admin, pictoActSetTeam, map[string]interface{}{"TEAM": "Inconnue"})
	if got := w.admin.waitAction(t, pictoActError, 1, 3*time.Second); errorCode(got[0]) != "INVALID_TEAM" {
		t.Errorf("équipe inconnue : PICTO_ERROR INVALID_TEAM attendu, got %v", got)
	}
	w.tv.never(t, pictoActError, 200*time.Millisecond) // l'erreur va à l'émetteur seul
	w.anim.never(t, pictoActError, 0)

	w.send(t, w.anim, pictoActSetTeam, map[string]interface{}{"TEAM": "TeamA"}) // l'anim aussi peut préparer
	waitGameField(t, w.admin, "PICTO_TEAM", "TeamA", 3*time.Second)
	cands, _ := gameField(t, w.admin, "PICTO_CANDIDATES").([]interface{})
	var ids []string
	for _, c := range cands {
		m := c.(map[string]interface{})
		ids = append(ids, m["ID"].(string))
		if m["CONNECTED"] != true {
			t.Errorf("candidat %v : CONNECTED attendu true (tablette liée)", m)
		}
	}
	if len(ids) != 2 || !contains(ids, w.aliceID) || !contains(ids, w.bobID) || contains(ids, w.carolID) {
		t.Errorf("PICTO_CANDIDATES = %v, attendu Alice et Bob seulement (équipe TeamA)", ids)
	}

	w.send(t, w.admin, pictoActSetDrawer, map[string]interface{}{"PLAYER": w.carolID})
	if got := w.admin.waitAction(t, pictoActError, 2, 3*time.Second); errorCode(got[len(got)-1]) != "NOT_A_CANDIDATE" {
		t.Errorf("joueur d'une autre équipe : NOT_A_CANDIDATE attendu, got %v", got)
	}
	w.send(t, w.admin, pictoActSetDrawer, map[string]interface{}{"PLAYER": w.bobID})
	waitGameField(t, w.admin, "PICTO_DRAWER", w.bobID, 3*time.Second)
	if gameField(t, w.admin, "PICTO_DRAWER_NAME") != "Bob" || gameField(t, w.admin, "PICTO_DRAWER_CONNECTED") != true {
		t.Errorf("PICTO_DRAWER_NAME/CONNECTED incohérents après désignation de Bob")
	}
	w.send(t, w.admin, pictoActSetDrawer, map[string]interface{}{"PLAYER": ""}) // vide = efface
	waitGameField(t, w.admin, "PICTO_DRAWER", "", 3*time.Second)
}

func TestPictoWire192_ChangingTeamClearsForeignDrawer(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.send(t, w.admin, pictoActSetTeam, map[string]interface{}{"TEAM": "TeamB"})
	waitGameField(t, w.admin, "PICTO_TEAM", "TeamB", 3*time.Second)
	waitGameField(t, w.admin, "PICTO_DRAWER", "", 3*time.Second) // §6.1 : dessinateur effacé s'il n'est pas de l'équipe
}

// Tirage au hasard : par PROPRIÉTÉ (le générateur global n'est pas injectable, #188) — §6.1, plan « Tests Requis ».
func TestPictoWire192_DrawDrawer_PropertyAndRefusals(t *testing.T) {
	w := newPictoWire(t)

	w.send(t, w.admin, pictoActDrawDrawer, struct{}{}) // sans équipe
	if got := w.admin.waitAction(t, pictoActError, 1, 3*time.Second); errorCode(got[0]) != "INVALID_TEAM" {
		t.Errorf("tirage sans équipe : INVALID_TEAM attendu, got %v", got)
	}

	w.send(t, w.admin, pictoActSetTeam, map[string]interface{}{"TEAM": "TeamA"})
	waitGameField(t, w.admin, "PICTO_TEAM", "TeamA", 3*time.Second)
	seen := map[string]bool{}
	for i := 0; i < 40; i++ { // rejouable à volonté
		w.send(t, w.anim, pictoActDrawDrawer, struct{}{})
		time.Sleep(30 * time.Millisecond)
		id, _ := gameField(t, w.admin, "PICTO_DRAWER").(string)
		if id != w.aliceID && id != w.bobID {
			t.Fatalf("tirage %d : %q n'est pas un joueur connecté de TeamA", i, id)
		}
		seen[id] = true
	}
	if len(seen) != 2 {
		t.Errorf("sur 40 tirages, Alice et Bob devraient sortir au moins une fois : %v", seen)
	}

	// Plus aucun joueur connecté de l'équipe : refus explicite, aucune désignation silencieuse.
	w.alice.conn.Close()
	w.bob.conn.Close()
	time.Sleep(300 * time.Millisecond)
	w.send(t, w.admin, pictoActSetTeam, map[string]interface{}{"TEAM": "TeamA"}) // recalcule les candidats
	w.send(t, w.admin, pictoActSetDrawer, map[string]interface{}{"PLAYER": ""})
	before := len(w.admin.of(pictoActError))
	w.send(t, w.admin, pictoActDrawDrawer, struct{}{})
	got := w.admin.waitAction(t, pictoActError, before+1, 3*time.Second)
	if errorCode(got[len(got)-1]) != "NO_CONNECTED_PLAYER" {
		t.Errorf("aucun joueur connecté : NO_CONNECTED_PLAYER attendu, got %v", got[len(got)-1])
	}
	if d, _ := gameField(t, w.admin, "PICTO_DRAWER").(string); d != "" {
		t.Errorf("aucune désignation silencieuse attendue, PICTO_DRAWER = %q", d)
	}
}

func TestPictoWire192_VPlayerCannotPrepare(t *testing.T) {
	w := newPictoWire(t)
	w.send(t, w.alice, pictoActSetTeam, map[string]interface{}{"TEAM": "TeamA"}) // liste blanche : admin/anim seulement
	time.Sleep(200 * time.Millisecond)
	if team, _ := gameField(t, w.admin, "PICTO_TEAM").(string); team != "" {
		t.Errorf("une tablette ne peut pas choisir l'équipe : PICTO_TEAM = %q", team)
	}
}

// ---------------------------------------------------------------- §6.1 garde de lancement

func TestPictoWire192_StartRefusedWithoutDrawer_NO_DRAWER(t *testing.T) {
	w := newPictoWire(t)
	w.send(t, w.admin, pictoActSetTeam, map[string]interface{}{"TEAM": "TeamA"})
	waitGameField(t, w.admin, "PICTO_TEAM", "TeamA", 3*time.Second)
	w.send(t, w.admin, protocol.ActionForceReady, struct{}{}) // « Le dessinateur n'est pas exigé pour READY »
	waitGameField(t, w.admin, "PHASE", "READY", 3*time.Second)

	w.send(t, w.admin, protocol.ActionStart, protocol.StartPayload{Delay: 0})
	got := w.admin.waitAction(t, pictoActError, 1, 3*time.Second)
	if errorCode(got[0]) != "NO_DRAWER" {
		t.Errorf("START sans dessinateur : NO_DRAWER attendu, got %v", got)
	}
	time.Sleep(200 * time.Millisecond)
	if ph := gameField(t, w.admin, "PHASE"); ph != "READY" {
		t.Errorf("l'état doit rester READY après un START refusé, got %v", ph)
	}
	w.tv.never(t, pictoActWord, 0)
}

// ---------------------------------------------------------------- §6.2 + §10 le mot est secret

func TestPictoWire192_Word_OnlyDrawerAdminAnim_NeverLeaks(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	word := w.startRound(t, w.alice)

	if !contains(pictoWireWords, word) {
		t.Fatalf("le mot de la manche %q doit appartenir à la question (§3.2)", word)
	}
	w.admin.waitAction(t, pictoActWord, 1, 3*time.Second)
	w.anim.waitAction(t, pictoActWord, 1, 3*time.Second)
	for _, c := range []*wireClient{w.tv, w.bob, w.carol} {
		c.never(t, pictoActWord, 100*time.Millisecond)
	}
	// Ni le mot, ni PICTO_WORDS dans AUCUNE trame de la TV ni des tablettes autres que le dessinateur (§10).
	for _, c := range []*wireClient{w.tv, w.bob, w.carol} {
		for _, secret := range append([]string{`"PICTO_WORDS"`}, pictoWireWords...) {
			if c.containsAnywhere(secret) {
				t.Errorf("%s : %q fuit (contrat §10)", c.name, secret)
			}
		}
	}
	// Le mot n'est dans aucun UPDATE, d'aucun client, avant la révélation — y compris pour l'admin.
	for _, c := range []*wireClient{w.admin, w.anim, w.tv, w.alice, w.bob, w.carol} {
		for _, f := range c.of(protocol.ActionUpdate) {
			if strings.Contains(f, word) && !strings.Contains(f, `"PICTO_WORDS"`) { // l'admin/anim voient la liste, jamais le mot seul
				t.Errorf("%s : le mot de la manche figure dans un UPDATE : %s", c.name, f)
			}
		}
		if v := gameFieldMaybe(c, "PICTO_REVEALED_WORD"); v != nil && v != "" {
			t.Errorf("%s : PICTO_REVEALED_WORD doit rester vide avant REVEALED, got %v", c.name, v)
		}
	}
	if gameField(t, w.admin, "PHASE") != "STARTED" {
		t.Errorf("la manche doit être STARTED, got %v", gameField(t, w.admin, "PHASE"))
	}
}

func gameFieldMaybe(c *wireClient, field string) interface{} {
	ups := c.of(protocol.ActionUpdate)
	if len(ups) == 0 {
		return nil
	}
	var p struct {
		Msg struct {
			Game map[string]interface{} `json:"GAME"`
		} `json:"MSG"`
	}
	if json.Unmarshal([]byte(ups[len(ups)-1]), &p) != nil {
		return nil
	}
	return p.Msg.Game[field]
}

// Une question à un mot : c'est toujours ce mot (§3.2).
func TestPictoWire192_SingleWordQuestion_AlwaysThatWord(t *testing.T) {
	w := newPictoWire(t)
	w.app.engine.SetPhase(game.PhaseStopped)
	w.app.engine.Ready("picto-one", &game.Question{
		ID: "picto-one", Question: "Un seul", Type: game.QuestionTypePicto, Category: game.CategoryHistory, Time: "90",
		TypedContent: game.TypedContent{PictoWords: []string{"Unique-secret"}},
	})
	w.chooseTeamAndDrawer(t, w.aliceID)
	if word := w.startRound(t, w.alice); word != "Unique-secret" {
		t.Errorf("question à un mot : mot = %q, attendu Unique-secret", word)
	}
}

// ---------------------------------------------------------------- §8 canal de dessin

func TestPictoWire193_Strokes_GuardRelaySeqAndEpoch(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRound(t, w.alice)
	epoch, _ := gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64)
	if epoch < 1 {
		t.Fatalf("PICTO_CANVAS_EPOCH doit valoir ≥ 1 après le START, got %v", epoch)
	}
	ep := int(epoch)

	// Non-dessinateur (Bob, même équipe) : refus EXPLICITE à lui seul, rien n'est relayé (§8.6).
	w.send(t, w.bob, pictoActStrokes, strokesPayload(ep, 1, [2]int{100, 100}, [2]int{200, 200}))
	if got := w.bob.waitAction(t, pictoActError, 1, 3*time.Second); errorCode(got[0]) != "NOT_DRAWER" {
		t.Errorf("non-dessinateur : NOT_DRAWER attendu, got %v", got)
	}
	// Le payload ne porte jamais d'identité : un faux PLAYER dans le payload ne change rien.
	forged := strokesPayload(ep, 1, [2]int{1, 1})
	forged["PLAYER"], forged["PLAYER_ID"], forged["ID"] = w.aliceID, w.aliceID, w.aliceID
	w.send(t, w.carol, pictoActStrokes, forged)
	if got := w.carol.waitAction(t, pictoActError, 1, 3*time.Second); errorCode(got[0]) != "NOT_DRAWER" {
		t.Errorf("identité forgée dans le payload : NOT_DRAWER attendu, got %v", got)
	}
	w.tv.never(t, pictoActStrokes, 150*time.Millisecond)

	// Dessinateur : relayé à la TV, l'admin, l'anim — SEQ 1, 2, … ; jamais aux tablettes (§8.1).
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 1, [2]int{100, 100}, [2]int{200, 200}))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 2, [2]int{300, 300}))
	for _, c := range []*wireClient{w.tv, w.admin, w.anim} {
		fr := c.waitAction(t, pictoActStrokes, 2, 3*time.Second)
		for i, f := range fr {
			var p struct {
				Msg struct {
					Epoch int `json:"EPOCH"`
					Seq   int `json:"SEQ"`
				} `json:"MSG"`
			}
			_ = json.Unmarshal([]byte(f), &p)
			if p.Msg.Seq != i+1 || p.Msg.Epoch != ep {
				t.Errorf("%s : trame %d : EPOCH/SEQ = %d/%d, attendu %d/%d", c.name, i, p.Msg.Epoch, p.Msg.Seq, ep, i+1)
			}
		}
	}
	for _, c := range []*wireClient{w.alice, w.bob, w.carol} {
		c.never(t, pictoActStrokes, 100*time.Millisecond)
	}
	// Aucun UPDATE ne véhicule les traits (§2 décision 5).
	for _, c := range []*wireClient{w.tv, w.admin, w.anim} {
		for _, f := range c.of(protocol.ActionUpdate) {
			if strings.Contains(f, `"STROKES"`) {
				t.Errorf("%s : les traits ne passent jamais par un UPDATE", c.name)
			}
		}
	}

	// Mauvaise époque : ignorée EN SILENCE (pas d'erreur, pas de relais) — §8.4.
	errsBefore := len(w.alice.of(pictoActError))
	relayedBefore := len(w.tv.of(pictoActStrokes))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep-1, 3, [2]int{5, 5}))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep+7, 3, [2]int{5, 5}))
	time.Sleep(200 * time.Millisecond)
	if len(w.alice.of(pictoActError)) != errsBefore || len(w.tv.of(pictoActStrokes)) != relayedBefore {
		t.Errorf("une trame d'une autre époque doit être ignorée en silence")
	}

	// Formes invalides : INVALID_STROKES à l'émetteur seul, jamais relayées (§9.1).
	bad := []map[string]interface{}{
		{"EPOCH": ep, "STROKES": []interface{}{}},                                                     // 0 trait
		strokesPayload(ep, 0, [2]int{1, 1}),                                                           // ID < 1
		strokesPayload(ep, 4, [2]int{-1, 5}),                                                          // coordonnée < 0
		strokesPayload(ep, 4, [2]int{10001, 5}),                                                       // coordonnée > 10 000
		{"EPOCH": ep, "STROKES": []map[string]interface{}{{"ID": 4, "PTS": [][2]int{}, "END": true}}}, // 0 point
	}
	for i, b := range bad {
		n := len(w.alice.of(pictoActError))
		w.send(t, w.alice, pictoActStrokes, b)
		got := w.alice.waitAction(t, pictoActError, n+1, 3*time.Second)
		if errorCode(got[len(got)-1]) != "INVALID_STROKES" {
			t.Errorf("forme invalide #%d : INVALID_STROKES attendu, got %s", i, got[len(got)-1])
		}
		time.Sleep(1100 * time.Millisecond) // une erreur par seconde au plus (RATE_LIMITED sinon)
	}
	if len(w.tv.of(pictoActStrokes)) != relayedBefore {
		t.Errorf("une trame refusée n'est jamais relayée")
	}
	// Trop de points (201) : forme invalide aussi.
	many := make([][2]int, 201)
	for i := range many {
		many[i] = [2]int{i, i}
	}
	n := len(w.alice.of(pictoActError))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 5, many...))
	if got := w.alice.waitAction(t, pictoActError, n+1, 3*time.Second); errorCode(got[len(got)-1]) != "INVALID_STROKES" {
		t.Errorf("201 points : INVALID_STROKES attendu, got %v", got[len(got)-1])
	}
}

func TestPictoWire193_Strokes_RateLimited(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRound(t, w.alice)
	ep := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	// 40 trames/s par connexion ; au-delà RATE_LIMITED, avec au plus une erreur par seconde (§8.6).
	for i := 0; i < 200; i++ {
		w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, i+1, [2]int{i, i}))
	}
	time.Sleep(300 * time.Millisecond)
	var limited int
	for _, f := range w.alice.of(pictoActError) {
		if errorCode(f) == "RATE_LIMITED" {
			limited++
		}
	}
	if limited < 1 {
		t.Errorf("200 trames en rafale : RATE_LIMITED attendu au moins une fois")
	}
	if limited > 2 {
		t.Errorf("au plus une erreur RATE_LIMITED par seconde, got %d", limited)
	}
	if got := len(w.tv.of(pictoActStrokes)); got >= 200 {
		t.Errorf("les trames au-delà du débit ne sont pas relayées, la TV en a reçu %d", got)
	}
}

func TestPictoWire193_CanvasSync_ReturnsWholeSheetToRequesterOnly(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRound(t, w.alice)
	ep := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 1, [2]int{100, 100}, [2]int{200, 200}))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 2, [2]int{300, 300}))
	w.tv.waitAction(t, pictoActStrokes, 2, 3*time.Second)

	w.send(t, w.tv, pictoActSync, struct{}{})
	fr := w.tv.waitAction(t, pictoActCanvas, 1, 3*time.Second)
	var p struct {
		Msg struct {
			Epoch   int `json:"EPOCH"`
			Seq     int `json:"SEQ"`
			Strokes []struct {
				ID  int      `json:"ID"`
				Pts [][2]int `json:"PTS"`
				End bool     `json:"END"`
			} `json:"STROKES"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(fr[0]), &p); err != nil {
		t.Fatal(err)
	}
	if p.Msg.Epoch != ep || p.Msg.Seq != 2 || len(p.Msg.Strokes) != 2 || len(p.Msg.Strokes[0].Pts) != 2 {
		t.Errorf("PICTO_CANVAS = %+v, attendu époque %d, SEQ 2, 2 traits (2 + 1 points)", p.Msg, ep)
	}
	for _, c := range []*wireClient{w.admin, w.anim, w.bob, w.carol} {
		c.never(t, pictoActCanvas, 100*time.Millisecond) // au demandeur seul
	}
	// Hors manche : réponse vide (§9.1) — pas d'erreur.
}

// Une nouvelle manche = nouvelle époque, feuille vierge ; les traits en vol de l'ancienne époque sont ignorés (§8.4, §6.6).
func TestPictoWire193_NewStart_NewEpoch_EmptySheet(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRound(t, w.alice)
	ep1 := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep1, 1, [2]int{100, 100}))
	w.tv.waitAction(t, pictoActStrokes, 1, 3*time.Second)

	w.send(t, w.admin, protocol.ActionStop, struct{}{})
	waitGameField(t, w.admin, "PHASE", "STOPPED", 3*time.Second)
	w.app.engine.Ready(w.question.ID, w.question) // relance : resélection de la même question (ce que handleReady fait après loadQuestion)
	w.send(t, w.admin, protocol.ActionForceReady, struct{}{})
	waitGameField(t, w.admin, "PHASE", "READY", 3*time.Second)
	w.send(t, w.admin, pictoActSetDrawer, map[string]interface{}{"PLAYER": w.aliceID}) // conservé ou rejoué
	w.send(t, w.admin, protocol.ActionStart, protocol.StartPayload{Delay: 0})
	w.alice.waitAction(t, pictoActWord, 2, 15*time.Second)
	ep2 := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	if ep2 != ep1+1 {
		t.Errorf("époque après relance = %d, attendu %d (+1 à chaque START, §5)", ep2, ep1+1)
	}
	w.send(t, w.tv, pictoActSync, struct{}{})
	fr := w.tv.waitAction(t, pictoActCanvas, 1, 3*time.Second)
	if strings.Contains(fr[len(fr)-1], `"PTS"`) {
		t.Errorf("la feuille repart vierge après un nouveau START : %s", fr[len(fr)-1])
	}
	// Trait en vol de l'ancienne époque : ignoré en silence.
	n := len(w.tv.of(pictoActStrokes))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep1, 9, [2]int{1, 1}))
	time.Sleep(200 * time.Millisecond)
	if len(w.tv.of(pictoActStrokes)) != n {
		t.Errorf("trait d'une ancienne époque relayé")
	}
}

// En PAUSED la feuille du dessinateur est verrouillée : WRONG_PHASE (§6.3, §8.6).
func TestPictoWire193_Strokes_RefusedWhenPaused(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRound(t, w.alice)
	ep := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	w.send(t, w.admin, protocol.ActionPause, struct{}{})
	waitGameField(t, w.admin, "PHASE", "PAUSED", 3*time.Second)
	n := len(w.alice.of(pictoActError))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 1, [2]int{1, 1}))
	if got := w.alice.waitAction(t, pictoActError, n+1, 3*time.Second); errorCode(got[len(got)-1]) != "WRONG_PHASE" {
		t.Errorf("PAUSED : WRONG_PHASE attendu, got %v", got[len(got)-1])
	}
	w.tv.never(t, pictoActStrokes, 100*time.Millisecond)
}

// Les buzzers/appuis sont ignorés pendant toute la manche : aucun effet, aucun UPDATE général (§6.3).
func TestPictoWire192_ButtonPressIgnored(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRound(t, w.alice)
	time.Sleep(200 * time.Millisecond)
	before := len(w.admin.of(protocol.ActionUpdate))
	w.send(t, w.bob, protocol.ActionButton, map[string]interface{}{"ID": w.bobID})
	time.Sleep(300 * time.Millisecond)
	if gameField(t, w.admin, "PHASE") != "STARTED" {
		t.Errorf("un appui ne doit pas changer la phase")
	}
	if after := len(w.admin.of(protocol.ActionUpdate)); after != before {
		t.Logf("note : %d UPDATE reçu(s) après l'appui (l'écho ciblé #248 est permis, pas un UPDATE général)", after-before)
	}
}

// Le dessinateur qui se reconnecte retrouve son mot et sa feuille (§6.7).
func TestPictoWire193_DrawerReconnect_GetsWordAndCanvasBack(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRound(t, w.alice)
	ep := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 1, [2]int{100, 100}, [2]int{200, 200}))
	w.tv.waitAction(t, pictoActStrokes, 1, 3*time.Second)
	w.alice.conn.Close()
	time.Sleep(300 * time.Millisecond)
	if ph := gameField(t, w.admin, "PHASE"); ph != "STARTED" {
		t.Errorf("le dessinateur déconnecté ne change pas la phase (la manche continue), got %v", ph)
	}

	base := startAnimAllowlistTestServer(t, w.app)
	again := newWireClient(t, "alice-2", dialWS(t, base, "/ws/player"))
	w.send(t, again, protocol.ActionPlayerConnect, protocol.PlayerConnectPayload{Name: "Alice", ID: w.aliceID})
	again.waitAction(t, pictoActWord, 1, 5*time.Second)
	cv := again.waitAction(t, pictoActCanvas, 1, 5*time.Second)
	if !strings.Contains(cv[0], `"PTS"`) {
		t.Errorf("la feuille doit être renvoyée avec ses traits : %s", cv[0])
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
