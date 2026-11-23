package main

// ---------------------------------------------------------------------------
// #319 (v13.0.0, LOT-2.2) — jeton de session des tablettes (VJoueur), audit security M1 : usurpation d'un VJoueur par
// PLAYER_CONNECT avec son seul ID. Écrit depuis contracts/vplayer-session.md §3, §4, §7, §9 (le code n'était pas
// final) : seuls les noms d'actions et le champ TOKEN du contrat sont utilisés, en littéraux (payloads en map).
// Vraie WebSocket (hub), dispatch synchrone par handleWebMessage comme env246. Tests `critical` : matrice 1b (refus
// sans liaison ni éviction), non-fuite du jeton.
// ---------------------------------------------------------------------------

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

var tokenShape319 = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`) // 32 octets crypto/rand en base64url sans remplissage (§3)

type sessEnv319 struct {
	app             *App
	base            string
	admin, anim, tv *wireClient
	adminCid        string
}

func newSessEnv319(t *testing.T) *sessEnv319 {
	t.Helper()
	app := newBroadcast127TestApp(t)
	app.evictionRegistry = server.NewEvictionRegistry()
	server.SetGlobalLogger(app.logger)
	t.Cleanup(func() { server.SetGlobalLogger(nil) })
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}, "TeamB": {Name: "TeamB"}})
	e := &sessEnv319{app: app, base: startAnimAllowlistTestServer(t, app)}
	dial := func(name, path string) (*wireClient, string) {
		c := dialWS(t, e.base, path)
		cid := learnClientID(t, app, c)
		return newWireClient(t, name, c), cid
	}
	e.admin, e.adminCid = dial("admin", "/ws/admin")
	e.anim, _ = dial("anim", "/ws/anim")
	e.tv, _ = dial("tv", "/ws/tv")
	return e
}

// tablet ouvre une connexion /ws/player : (collecteur, identifiant de connexion du hub).
func (e *sessEnv319) tablet(t *testing.T, name string) (*wireClient, string) {
	t.Helper()
	c := dialWS(t, e.base, "/ws/player")
	cid := learnClientID(t, e.app, c)
	return newWireClient(t, name, c), cid
}

// connect fait traiter PLAYER_CONNECT {NAME, ID?, TOKEN?} par le vrai dispatcher (champs absents = non envoyés).
func (e *sessEnv319) connect(t *testing.T, cid, name, id, token string) {
	t.Helper()
	p := map[string]interface{}{"NAME": name}
	if id != "" {
		p["ID"] = id
	}
	if token != "" {
		p["TOKEN"] = token
	}
	msg, err := protocol.NewMessage(protocol.ActionPlayerConnect, p)
	if err != nil {
		t.Fatal(err)
	}
	e.app.handleWebMessage(&protocol.IncomingMessage{Source: "WebSocket", Data: msg, ClientID: cid, ClientType: "vplayer"})
}

type connected319 struct{ ID, Name, Token string }

func lastConnected319(t *testing.T, c *wireClient, wantCount int) connected319 {
	t.Helper()
	fr := c.waitAction(t, "PLAYER_CONNECTED", wantCount, 3*time.Second)
	var p struct {
		Msg struct {
			ID    string `json:"ID"`
			Name  string `json:"NAME"`
			Token string `json:"TOKEN"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(fr[len(fr)-1]), &p); err != nil {
		t.Fatal(err)
	}
	return connected319{p.Msg.ID, p.Msg.Name, p.Msg.Token}
}

func rejectedReason319(t *testing.T, c *wireClient, wantCount int) string {
	t.Helper()
	fr := c.waitAction(t, "PLAYER_REJECTED", wantCount, 3*time.Second)
	var p struct {
		Msg struct {
			Reason string `json:"REASON"`
		} `json:"MSG"`
	}
	_ = json.Unmarshal([]byte(fr[len(fr)-1]), &p)
	return p.Msg.Reason
}

// enroll : premier enrôlement par le nom seul → (ID, jeton) reçus dans PLAYER_CONNECTED.
func (e *sessEnv319) enroll(t *testing.T, name string) (*wireClient, string, connected319) {
	t.Helper()
	c, cid := e.tablet(t, name)
	e.connect(t, cid, name, "", "")
	return c, cid, lastConnected319(t, c, 1)
}

func (e *sessEnv319) logsContain(frag string) bool {
	for _, en := range e.app.logger.GetHistory() {
		if strings.Contains(en.Message, frag) {
			return true
		}
	}
	return false
}

func (e *sessEnv319) updates() int {
	return len(e.admin.of(protocol.ActionUpdate)) + len(e.tv.of(protocol.ActionUpdate))
}

// ---------------------------------------------------------------- §3, §4.3 émission

func TestVPlayerSession319_Enroll_IssuesRandomTokenInPlayerConnected(t *testing.T) {
	e := newSessEnv319(t)
	seen := map[string]bool{}
	for _, n := range []string{"Alice", "Bob", "Carol", "Dave", "Erin", "Frank"} {
		_, _, c := e.enroll(t, n)
		if !tokenShape319.MatchString(c.Token) {
			t.Fatalf("%s : jeton %q — 43 caractères base64url attendus (32 octets crypto/rand, §3)", n, c.Token)
		}
		if c.ID == "" {
			t.Errorf("%s : PLAYER_CONNECTED.ID manquant", n)
		}
		if seen[c.Token] {
			t.Errorf("deux places avec le même jeton %q", c.Token)
		}
		seen[c.Token] = true
	}
	if !e.logsContain("VPLAYER_TOKEN_ISSUED") {
		t.Errorf("l'émission d'un jeton doit être journalisée (VPLAYER_TOKEN_ISSUED) — l'ID et l'issue, jamais le jeton")
	}
}

// ---------------------------------------------------------------- §4.2 1a reconnexion valide

func TestVPlayerSession319_1a_ReconnectWithValidToken_SameTokenOldConnectionEvicted(t *testing.T) {
	e := newSessEnv319(t)
	first, firstCid, c := e.enroll(t, "Alice")

	second, secondCid := e.tablet(t, "alice-2")
	e.connect(t, secondCid, "Alice", c.ID, c.Token)
	again := lastConnected319(t, second, 1)
	if again.ID != c.ID {
		t.Errorf("1a : même place attendue (%s), got %s", c.ID, again.ID)
	}
	if again.Token != c.Token {
		t.Errorf("le jeton est stable tant que le bumper existe et ne tourne pas à chaque reconnexion (§3) : %q ≠ %q", again.Token, c.Token)
	}
	if got, ok := e.app.wsHub.GetClientPlayerID(secondCid); !ok || got != c.ID {
		t.Errorf("la nouvelle connexion doit être liée à %s, got (%q, %v)", c.ID, got, ok)
	}
	// Comportement #246 conservé : une seule connexion liée, la plus récente l'emporte.
	first.waitAction(t, "PLAYER_EVICTED", 1, 3*time.Second)
	if _, ok := e.app.wsHub.GetClientPlayerID(firstCid); ok {
		t.Errorf("l'ancienne connexion doit être déliée (DUPLICATE_SESSION, #246)")
	}
}

// ---------------------------------------------------------------- §4.2 1b refus

func TestVPlayerSession319_Critical_1b_InvalidSession_NoBindNoEvictionNoUpdate(t *testing.T) {
	e := newSessEnv319(t)
	alice, aliceCid, a := e.enroll(t, "Alice")
	_, _, b := e.enroll(t, "Bob")
	time.Sleep(150 * time.Millisecond) // les UPDATE d'enrôlement sont diffusés

	cases := []struct{ name, token string }{
		{"jeton absent", ""},
		{"jeton faux", "x" + a.Token[1:]},
		{"jeton trop court", "abc"},
		{"jeton d'une autre place", b.Token},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attacker, attackerCid := e.tablet(t, "attacker")
			upBefore := e.updates()
			evictedBefore := len(alice.of("PLAYER_EVICTED"))
			e.connect(t, attackerCid, "Alice", a.ID, tc.token)

			if r := rejectedReason319(t, attacker, 1); r != "INVALID_SESSION" {
				t.Errorf("REASON = %q, attendu INVALID_SESSION (§4.4)", r)
			}
			if len(attacker.of("PLAYER_CONNECTED")) != 0 {
				t.Errorf("aucun PLAYER_CONNECTED pour un jeton invalide")
			}
			if id, ok := e.app.wsHub.GetClientPlayerID(attackerCid); ok {
				t.Errorf("1b : aucune liaison attendue, la connexion est liée à %q", id)
			}
			if id, ok := e.app.wsHub.GetClientPlayerID(aliceCid); !ok || id != a.ID {
				t.Errorf("1b : la session légitime ne doit pas être déliée (got %q, %v)", id, ok)
			}
			time.Sleep(200 * time.Millisecond)
			if got := len(alice.of("PLAYER_EVICTED")); got != evictedBefore {
				t.Errorf("1b : aucune éviction (PLAYER_EVICTED / DUPLICATE_SESSION) du client légitime")
			}
			if got := e.updates(); got != upBefore {
				t.Errorf("1b : aucun UPDATE ne doit être diffusé (%d → %d)", upBefore, got)
			}
			if !e.logsContain("VPLAYER_TOKEN_INVALID") || !e.logsContain(a.ID) {
				t.Errorf("1b : WARN VPLAYER_TOKEN_INVALID id=%s attendu", a.ID)
			}
		})
	}
	// La session légitime n'a rien perdu : le bumper reste connecté et son jeton reste valable.
	if bm := e.app.engine.GetBumper(a.ID); bm == nil || !bm.Connected {
		t.Errorf("le bumper légitime doit rester CONNECTED après les tentatives refusées")
	}
	legit, legitCid := e.tablet(t, "legit-again")
	e.connect(t, legitCid, "Alice", a.ID, a.Token)
	if got := lastConnected319(t, legit, 1); got.ID != a.ID {
		t.Errorf("le jeton légitime reste valable après des refus")
	}
}

// ---------------------------------------------------------------- §4.2 1c migration

func TestVPlayerSession319_1c_LegacySeatWithoutFingerprint_AcceptedThenTokenIssued(t *testing.T) {
	e := newSessEnv319(t)
	// Place créée avant v13.0.0 : un bumper VJoueur du moteur auquel aucune empreinte n'a jamais été attribuée.
	id := setupVirtualPlayer(t, e.app, "Legacy", "TeamA")
	e.app.engine.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})

	first, firstCid := e.tablet(t, "legacy-owner")
	e.connect(t, firstCid, "Legacy", id, "")
	got := lastConnected319(t, first, 1)
	if got.ID != id {
		t.Fatalf("1c : reconnexion acceptée à la place %s, got %s", id, got.ID)
	}
	if !tokenShape319.MatchString(got.Token) {
		t.Errorf("1c : un nouveau jeton doit être émis (premier arrivé), got %q", got.Token)
	}
	if !e.logsContain("VPLAYER_TOKEN_ISSUED") || !e.logsContain("legacy") {
		t.Errorf("1c : journal VPLAYER_TOKEN_ISSUED reason=legacy attendu")
	}
	// Dès lors la place est protégée : un second arrivant sans jeton est refusé (1b).
	second, secondCid := e.tablet(t, "late-arrival")
	e.connect(t, secondCid, "Legacy", id, "")
	if r := rejectedReason319(t, second, 1); r != "INVALID_SESSION" {
		t.Errorf("après l'émission, 1b attendu pour un client sans jeton, got %q", r)
	}
}

// ---------------------------------------------------------------- §4.2 2 à 4, §7 re-clé : reprise #122 / libération #134

func TestVPlayerSession319_Release134_RekeyIssuesNewTokenOldOneDead(t *testing.T) {
	e := newSessEnv319(t)
	owner, ownerCid, a := e.enroll(t, "Alice")
	_ = ownerCid

	// L'animateur libère la place d'un joueur connecté (#134 : re-clé, nouvel ID).
	e.app.handleReleaseBumperName(releaseBumperNameMsg(t, a.ID))
	owner.waitAction(t, "PLAYER_EVICTED", 1, 3*time.Second)

	// Reprise par le nom, sans ID (#122) : nouvelle place, nouveau jeton.
	taker, takerCid := e.tablet(t, "taker")
	e.connect(t, takerCid, "Alice", "", "")
	n := lastConnected319(t, taker, 1)
	if n.Token == "" || n.Token == a.Token {
		t.Errorf("la place re-clée reçoit un NOUVEAU jeton, got %q (ancien %q)", n.Token, a.Token)
	}
	// L'ancien jeton est mort : il ne valide pas la nouvelle place.
	thief, thiefCid := e.tablet(t, "thief")
	e.connect(t, thiefCid, "Alice", n.ID, a.Token)
	if r := rejectedReason319(t, thief, 1); r != "INVALID_SESSION" {
		t.Errorf("l'ancien jeton ne doit plus valider la place : REASON = %q", r)
	}
	// Le nouveau jeton est valable.
	again, againCid := e.tablet(t, "again")
	e.connect(t, againCid, "Alice", n.ID, n.Token)
	if lastConnected319(t, again, 1).ID != n.ID {
		t.Errorf("le nouveau jeton doit valider la place %s", n.ID)
	}
}

func TestVPlayerSession319_Reprise122_DisconnectedRelease_NewTokenOldOneDead(t *testing.T) {
	e := newSessEnv319(t)
	owner, _, a := e.enroll(t, "Alice")
	owner.conn.Close()
	time.Sleep(300 * time.Millisecond)
	e.app.engine.UpdateBumper(a.ID, map[string]interface{}{"CONNECTED": false})
	e.app.handleReleaseBumperName(releaseBumperNameMsg(t, a.ID)) // #122 : autorisation de reprise, sans re-clé

	taker, takerCid := e.tablet(t, "taker")
	e.connect(t, takerCid, "Alice", "", "")
	n := lastConnected319(t, taker, 1)
	if n.Token == "" || n.Token == a.Token {
		t.Errorf("reprise #122 : nouveau jeton attendu (§4.2, lignes 2 à 4), got %q", n.Token)
	}
	old, oldCid := e.tablet(t, "old-device")
	e.connect(t, oldCid, "Alice", n.ID, a.Token)
	if r := rejectedReason319(t, old, 1); r != "INVALID_SESSION" {
		t.Errorf("après reprise, l'ancien jeton ne vaut plus rien : REASON = %q", r)
	}
}

func TestVPlayerSession319_NewEnrollment_IdNotFound_TreatedAsNoId(t *testing.T) {
	e := newSessEnv319(t)
	c, cid := e.tablet(t, "stale-tab")
	e.connect(t, cid, "Nouveau", "id-qui-n-existe-pas", "n-importe-quoi") // §4.2 ligne 2 : ID introuvable → nouvel enrôlement
	got := lastConnected319(t, c, 1)
	if got.ID == "" || got.ID == "id-qui-n-existe-pas" || !tokenShape319.MatchString(got.Token) {
		t.Errorf("ID introuvable : nouvel enrôlement avec nouveau jeton attendu, got %+v", got)
	}
}

// ---------------------------------------------------------------- §3, §9 non-fuite

func TestVPlayerSession319_Critical_TokenNeverLeaks_Frames_State_Logs(t *testing.T) {
	e := newSessEnv319(t)
	owner, _, a := e.enroll(t, "Alice")
	other, otherCid := e.tablet(t, "bystander")
	e.connect(t, otherCid, "Bob", "", "")
	lastConnected319(t, other, 1)
	// Une reconnexion légitime, un refus, une reprise : tous les chemins qui touchent au jeton.
	re, reCid := e.tablet(t, "re")
	e.connect(t, reCid, "Alice", a.ID, a.Token)
	lastConnected319(t, re, 1)
	bad, badCid := e.tablet(t, "bad")
	e.connect(t, badCid, "Alice", a.ID, "faux")
	rejectedReason319(t, bad, 1)
	e.app.broadcastUpdate()
	time.Sleep(400 * time.Millisecond)

	// Seul le client qui vient d'être lié reçoit le jeton (PLAYER_CONNECTED) — jamais un autre client.
	for _, c := range []*wireClient{e.admin, e.anim, e.tv, other, bad} {
		if c.containsAnywhere(a.Token) {
			t.Errorf("%s a reçu le jeton d'Alice (§3 : seul le client lié le reçoit)", c.name)
		}
	}
	for _, f := range owner.snapshot() {
		if frameAction(f) != "PLAYER_CONNECTED" && strings.Contains(f, a.Token) {
			t.Errorf("le jeton ne doit figurer que dans PLAYER_CONNECTED, trouvé dans %s", frameAction(f))
		}
	}
	for _, f := range re.snapshot() {
		if frameAction(f) != "PLAYER_CONNECTED" && strings.Contains(f, a.Token) {
			t.Errorf("le jeton ne doit figurer que dans PLAYER_CONNECTED, trouvé dans %s", frameAction(f))
		}
	}
	// Jamais dans le GameState complet, ni dans un UPDATE sérialisé pour un type de client.
	if strings.Contains(string(e.app.engine.GetGameJSON()), a.Token) {
		t.Errorf("le jeton ne doit jamais figurer dans le GameState/bumpers")
	}
	if strings.Contains(string(e.app.engine.GetGameJSON()), `"TOKEN"`) {
		t.Errorf("aucun champ TOKEN dans l'état (la structure Bumper ne le porte pas, §3)")
	}
	// Jamais journalisé.
	if e.logsContain(a.Token) {
		t.Errorf("le jeton en clair ne doit jamais être journalisé (§3) : seuls ID et issue le sont")
	}
	for _, line := range []string{"VPLAYER_TOKEN_ISSUED", "VPLAYER_TOKEN_INVALID"} {
		for _, en := range e.app.logger.GetHistory() {
			if strings.Contains(en.Message, line) && strings.Contains(en.Message, a.Token[:12]) {
				t.Errorf("%s ne doit pas contenir le jeton (même partiel) : %s", line, en.Message)
			}
		}
	}
}

// ---------------------------------------------------------------- §8 effets sur PICTO : le dessinateur ne peut plus être usurpé

func TestVPlayerSession319_Critical_PictoDrawerCannotBeImpersonated(t *testing.T) {
	w := newPictoWire(t)
	// Les trois tablettes se sont enrôlées par l'ID du harnais ; on relit le jeton de la place du dessinateur.
	tok := func(c *wireClient) string {
		fr := c.of("PLAYER_CONNECTED")
		if len(fr) == 0 {
			t.Fatalf("%s : pas de PLAYER_CONNECTED", c.name)
		}
		var p struct {
			Msg struct {
				Token string `json:"TOKEN"`
			} `json:"MSG"`
		}
		_ = json.Unmarshal([]byte(fr[len(fr)-1]), &p)
		return p.Msg.Token
	}
	aliceToken := tok(w.alice)
	if aliceToken == "" {
		t.Fatalf("la tablette d'Alice doit avoir reçu un jeton (cas 1c ou enrôlement)")
	}
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRound(t, w.alice)
	ep := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 1, [2]int{100, 100}, [2]int{200, 200}))
	w.tv.waitAction(t, pictoActStrokes, 1, 3*time.Second)

	// L'ID du dessinateur est lu sur /ws/tv (PICTO_DRAWER). Un attaquant le rejoue sans jeton.
	drawerID, _ := gameField(t, w.tv, "PICTO_DRAWER").(string)
	if drawerID != w.aliceID {
		t.Fatalf("PICTO_DRAWER (public sur la TV) = %q", drawerID)
	}
	base := startAnimAllowlistTestServer(t, w.app)
	attacker := newWireClient(t, "attacker", dialWS(t, base, "/ws/player"))
	acid := learnClientID(t, w.app, attacker.conn)
	msg, _ := protocol.NewMessage(protocol.ActionPlayerConnect, map[string]interface{}{"NAME": "Alice", "ID": drawerID})
	w.app.handleWebMessage(&protocol.IncomingMessage{Source: "WebSocket", Data: msg, ClientID: acid, ClientType: "vplayer"})

	if r := rejectedReason319(t, attacker, 1); r != "INVALID_SESSION" {
		t.Fatalf("usurpation du dessinateur : INVALID_SESSION attendu, got %q", r)
	}
	time.Sleep(300 * time.Millisecond)
	attacker.never(t, pictoActWord, 0)
	attacker.never(t, pictoActCanvas, 0)
	for _, secret := range pictoWireWords {
		if attacker.containsAnywhere(secret) {
			t.Errorf("l'attaquant ne doit jamais voir le mot (%q)", secret)
		}
	}
	// Les traits envoyés par l'attaquant (non lié) n'ont aucun effet, et le vrai dessinateur n'est pas évincé.
	relayed := len(w.tv.of(pictoActStrokes))
	w.send(t, attacker, pictoActStrokes, strokesPayload(ep, 2, [2]int{9, 9}))
	time.Sleep(250 * time.Millisecond)
	if got := len(w.tv.of(pictoActStrokes)); got != relayed {
		t.Errorf("les traits d'un client non lié ne sont jamais relayés")
	}
	if len(w.alice.of("PLAYER_EVICTED")) != 0 {
		t.Errorf("le dessinateur légitime ne doit pas être évincé")
	}
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 3, [2]int{300, 300}))
	w.tv.waitAction(t, pictoActStrokes, relayed+1, 3*time.Second) // il dessine toujours
}
