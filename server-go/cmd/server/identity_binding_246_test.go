package main

// #246 (v11.2) — identité des VJoueurs : liaison connexion ↔ bumper, refus des actions
// non liées / d'un autre ID, PLAYER_CONNECT cas 1 sans renommage, unicité des noms,
// DUPLICATE_SESSION. Contrat : contracts/websocket-actions.md §PLAYER_CONNECT
// « Liaison connexion ↔ bumper », §PLAYER_EVICTED ; événements : contracts/logging.md §3.2.
//
// Reproductions du bug (statut `regression` dans tests/INDEX.md) :
//   - harness246_identity_server : un onglet périmé envoie PLAYER_CONNECT {ID: Paul, NAME: "Mamie"}
//     et renomme le joueur actuel ;
//   - harness246_name_binding (côté serveur) : une connexion non liée écrit pour un autre joueur.
// Tous ces tests doivent ÉCHOUER avant le DEV (B-ID) et passer après.

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"testing"
	"time"
)

// playerActions246 : les actions joueur liées émises par un client vplayer (ARDOISE_FINAL : Batch 2).
func playerActions246(id string) []struct {
	action  string
	payload interface{}
} {
	return []struct {
		action  string
		payload interface{}
	}{
		{protocol.ActionArdoiseInput, protocol.ArdoiseInputPayload{ID: id, Text: "HACK"}},
		{protocol.ActionVPlayerQCMAnswer, protocol.VPlayerQCMAnswerPayload{ID: id, AnswerColor: "RED"}},
		{protocol.ActionButton, map[string]interface{}{"ID": id, "button": "A"}},
		{protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{CardID: "1-1", ID: id}},
	}
}

// --- Liaison -----------------------------------------------------------------

func TestBinding246_PlayerConnectBindsTheConnectionToItsBumper(t *testing.T) {
	e := newEnv246(t)
	_, cid := e.dial(t, "/ws/player")

	if _, ok := e.app.wsHub.GetClientPlayerID(cid); ok {
		t.Fatal("précondition : la connexion ne doit pas être liée avant PLAYER_CONNECT")
	}
	e.connect(t, cid, "Paul", e.paul)

	got, ok := e.app.wsHub.GetClientPlayerID(cid)
	if !ok || got != e.paul {
		t.Fatalf("après PLAYER_CONNECT {ID: paul}, la connexion doit être liée à %s ; obtenu (%q, %v)", e.paul, got, ok)
	}
}

// Critical — liaison/refus. Une connexion vplayer NON liée ne peut écrire pour personne.
func TestBinding246_Critical_UnboundConnectionActionsAreRefused(t *testing.T) {
	for _, tc := range playerActions246("") {
		t.Run(tc.action, func(t *testing.T) {
			e := newEnv246(t)
			_, cid := e.dial(t, "/ws/player") // jamais de PLAYER_CONNECT : connexion non liée

			payload := playerActions246(e.paul)
			for _, p := range payload {
				if p.action == tc.action {
					e.send(t, cid, "vplayer", p.action, p.payload)
				}
			}

			if !e.waitLog(t, game.LogLevelWarn, "PLAYER_ACTION_UNBOUND") {
				t.Fatalf("%s d'une connexion non liée : WARN PLAYER_ACTION_UNBOUND attendu ; logs=%v", tc.action, e.logLines(game.LogLevelWarn))
			}
			if !e.hasLog(game.LogLevelWarn, "PLAYER_ACTION_UNBOUND", tc.action) {
				t.Errorf("le WARN doit nommer l'action %s (champ action) ; logs=%v", tc.action, e.logLines(game.LogLevelWarn))
			}
			text, _ := e.answers()
			if text["Les Bleus"] != "" {
				t.Errorf("une action refusée ne doit rien écrire : ARDOISE_ANSWERS=%v", text)
			}
		})
	}
}

// Critical — liaison/refus, motif 2 : l'ID de l'action n'est pas celui du bumper lié.
func TestBinding246_Critical_ForeignIDIsRefused(t *testing.T) {
	for _, tc := range playerActions246("") {
		t.Run(tc.action, func(t *testing.T) {
			e := newEnv246(t)
			_, cid := e.dial(t, "/ws/player")
			e.connect(t, cid, "Paul", e.paul) // liée à Paul…

			for _, p := range playerActions246(e.mamie) { // …mais écrit avec l'ID de Mamie
				if p.action == tc.action {
					e.send(t, cid, "vplayer", p.action, p.payload)
				}
			}

			if !e.waitLog(t, game.LogLevelWarn, "PLAYER_FOREIGN_ID") {
				t.Fatalf("%s avec l'ID d'un autre : WARN PLAYER_FOREIGN_ID attendu ; logs=%v", tc.action, e.logLines(game.LogLevelWarn))
			}
			text, _ := e.answers()
			if text["Les Rouges"] != "" || text["Les Bleus"] != "" {
				t.Errorf("rien ne doit être écrit (ni chez Mamie, ni chez Paul) : %v", text)
			}
		})
	}
}

// Reproduction harness246_name_binding côté serveur : la tablette de Mamie (équipe Rouges),
// jamais liée, n'écrit pas dans l'équipe d'un autre joueur.
func TestBinding246_Regression_UnboundTabletCannotWriteIntoAnotherTeam(t *testing.T) {
	e := newEnv246(t)
	_, cid := e.dial(t, "/ws/player")
	e.send(t, cid, "vplayer", protocol.ActionArdoiseInput, protocol.ArdoiseInputPayload{ID: e.paul, Text: "LYON"})

	text, _ := e.answers()
	if text["Les Bleus"] != "" {
		t.Fatalf("une connexion non liée a écrit dans l'équipe de Paul : %v", text)
	}
}

func TestBinding246_BoundConnectionWithItsOwnIDIsAccepted(t *testing.T) {
	e := newEnv246(t)
	_, cid := e.dial(t, "/ws/player")
	e.connect(t, cid, "Paul", e.paul)

	e.send(t, cid, "vplayer", protocol.ActionArdoiseInput, protocol.ArdoiseInputPayload{ID: e.paul, Text: "PARIS"})

	text, _ := e.answers()
	if text["Les Bleus"] != "PARIS" {
		t.Fatalf("non-régression : l'action d'une connexion liée avec son propre ID doit passer ; ARDOISE_ANSWERS=%v", text)
	}
	if e.countLog(game.LogLevelWarn, "PLAYER_ACTION_UNBOUND")+e.countLog(game.LogLevelWarn, "PLAYER_FOREIGN_ID") != 0 {
		t.Errorf("aucun refus attendu ; logs=%v", e.logLines(game.LogLevelWarn))
	}
}

// Les simulations admin (BUTTON) et les actions anim/TV ne sont pas concernées par la liaison.
func TestBinding246_AdminSimulatedButtonAndAnimFlipAreNotAffected(t *testing.T) {
	e := newEnv246(t)
	_, admin := e.dial(t, "/ws/admin")
	_, anim := e.dial(t, "/ws/anim")

	e.send(t, admin, "admin", protocol.ActionButton, map[string]interface{}{"ID": e.paul, "button": "A"})
	e.send(t, anim, "anim", protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{CardID: "1-1"})

	if n := e.countLog(game.LogLevelWarn, "PLAYER_ACTION_UNBOUND") + e.countLog(game.LogLevelWarn, "PLAYER_FOREIGN_ID"); n != 0 {
		t.Fatalf("les actions admin/anim ne doivent déclencher aucun refus de liaison ; logs=%v", e.logLines(game.LogLevelWarn))
	}
}

// --- PLAYER_CONNECT cas 1 : jamais de renommage -----------------------------------

// Critical — reproduction harness246_identity_server : un onglet périmé envoie
// PLAYER_CONNECT {ID: Paul, NAME: "Mamie"} pendant la question.
func TestPlayerConnect246_Critical_Case1NeverRenamesTheBumper(t *testing.T) {
	e := newEnv246(t)
	conn, cid := e.dial(t, "/ws/player")

	e.connect(t, cid, "Mamie", e.paul) // l'onglet périmé prétend s'appeler Mamie

	if got := e.app.engine.GetBumper(e.paul).Name; got != "Paul" {
		t.Fatalf("le NAME envoyé ne doit plus modifier le nom du bumper : %q (attendu %q)", got, "Paul")
	}
	if got := e.app.engine.GetBumper(e.mamie).Name; got != "Mamie" {
		t.Fatalf("le bumper de Mamie ne doit pas avoir changé : %q", got)
	}
	names := map[string]int{}
	for _, id := range []string{e.paul, e.mamie} {
		names[e.app.engine.GetBumper(id).Name]++
	}
	if names["Mamie"] != 1 {
		t.Errorf("jamais deux VJoueurs de même nom : %v", names)
	}

	raw := readActionMatchingWithin(t, conn, protocol.ActionPlayerConnected, 2*time.Second)
	var pc protocol.PlayerConnectedPayload
	if err := json.Unmarshal(raw, &pc); err != nil {
		t.Fatalf("PLAYER_CONNECTED illisible: %v", err)
	}
	if pc.ID != e.paul || pc.Name != "Paul" {
		t.Errorf("PLAYER_CONNECTED doit renvoyer le nom SERVEUR que le client adopte : %+v", pc)
	}
	if !e.waitLog(t, game.LogLevelWarn, "VPLAYER_NAME_MISMATCH") {
		t.Errorf("WARN VPLAYER_NAME_MISMATCH attendu ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	if !e.hasLog(game.LogLevelWarn, "VPLAYER_NAME_MISMATCH", "Mamie", "Paul") {
		t.Errorf("le WARN doit porter server_name=Paul et client_name=Mamie ; logs=%v", e.logLines(game.LogLevelWarn))
	}

	// La frappe suivante de Paul (tablette active) reste sous son équipe, la réponse de Mamie est intacte.
	e.send(t, cid, "vplayer", protocol.ActionArdoiseInput, protocol.ArdoiseInputPayload{ID: e.paul, Text: "PARIS"})
	text, _ := e.answers()
	if text["Les Bleus"] != "PARIS" || text["Les Rouges"] != "" {
		t.Errorf("la réponse reste attachée à l'équipe de Paul : %v", text)
	}
}

func TestPlayerConnect246_Case1SameNameIsSilent(t *testing.T) {
	e := newEnv246(t)
	_, cid := e.dial(t, "/ws/player")

	e.connect(t, cid, "Paul", e.paul)

	if e.countLog(game.LogLevelWarn, "VPLAYER_NAME_MISMATCH") != 0 {
		t.Errorf("même nom que le serveur : pas de VPLAYER_NAME_MISMATCH ; logs=%v", e.logLines(game.LogLevelWarn))
	}
}

// --- Unicité des noms (3 chemins) ----------------------------------------------------

// Chemin 1 — enrôlement : nom déjà porté (sans casse, espaces retirés) → NAME_TAKEN.
func TestNameUniqueness246_Enrollment_TakenNameIsRejected(t *testing.T) {
	for _, name := range []string{"Paul", "paul", "  PAUL "} {
		t.Run(name, func(t *testing.T) {
			e := newEnv246(t)
			e.app.engine.SetPhase(game.PhaseEnroll)
			conn, cid := e.dial(t, "/ws/player")
			e.connect(t, cid, name, "")

			raw := readActionMatchingWithin(t, conn, protocol.ActionPlayerRejected, 2*time.Second)
			var rej protocol.PlayerRejectedPayload
			_ = json.Unmarshal(raw, &rej)
			if rej.Reason != "NAME_TAKEN" {
				t.Fatalf("REASON=%q (attendu NAME_TAKEN)", rej.Reason)
			}
			if _, ok := e.app.wsHub.GetClientPlayerID(cid); ok {
				t.Error("une connexion rejetée ne doit pas être liée")
			}
		})
	}
}

// Chemin 2 — reconnexion avec un ID périmé (inconnu) mais un nom déjà pris : rejeté, jamais fusionné.
func TestNameUniqueness246_StaleIDWithTakenNameIsRejected(t *testing.T) {
	e := newEnv246(t)
	conn, cid := e.dial(t, "/ws/player")
	e.connect(t, cid, "Mamie", "vjoueur_Mamie_ancienne_partie")

	raw := readActionMatchingWithin(t, conn, protocol.ActionPlayerRejected, 2*time.Second)
	var rej protocol.PlayerRejectedPayload
	_ = json.Unmarshal(raw, &rej)
	if rej.Reason != "NAME_TAKEN" {
		t.Fatalf("REASON=%q (attendu NAME_TAKEN)", rej.Reason)
	}
	if got := e.app.engine.GetBumper(e.mamie).Name; got != "Mamie" {
		t.Errorf("le bumper existant ne doit pas être touché : %q", got)
	}
}

func fullUpdateBumpers246(t *testing.T, e *env246, mutate func(map[string]*game.Bumper)) {
	t.Helper()
	bumpers := map[string]*game.Bumper{}
	for _, id := range []string{e.paul, e.mamie} {
		cp := *e.app.engine.GetBumper(id)
		bumpers[id] = &cp
	}
	mutate(bumpers)
	// par le dispatcher public (la signature interne de handleFullUpdate peut évoluer : #249)
	_, admin := e.dial(t, "/ws/admin")
	e.send(t, admin, "admin", protocol.ActionUpdate, map[string]interface{}{"bumpers": bumpers})
}

// Chemin 3 — renommage par l'admin (UPDATE/FULL) vers un nom déjà porté : ignoré pour ce bumper.
func TestNameUniqueness246_AdminRenameToTakenNameIsIgnored(t *testing.T) {
	for _, taken := range []string{"Paul", "paul", " PAUL "} {
		t.Run(taken, func(t *testing.T) {
			e := newEnv246(t)
			fullUpdateBumpers246(t, e, func(b map[string]*game.Bumper) { b[e.mamie].Name = taken })

			if got := e.app.engine.GetBumper(e.mamie).Name; got != "Mamie" {
				t.Fatalf("le renommage vers un nom déjà porté doit être ignoré pour ce bumper : %q", got)
			}
			if got := e.app.engine.GetBumper(e.paul).Name; got != "Paul" {
				t.Errorf("le porteur du nom ne change pas : %q", got)
			}
			if !e.hasLog(game.LogLevelWarn, "VPLAYER_DUPLICATE_NAME") {
				t.Errorf("WARN VPLAYER_DUPLICATE_NAME attendu ; logs=%v", e.logLines(game.LogLevelWarn))
			}
		})
	}
}

func TestNameUniqueness246_AdminRenameToAFreeNameStillWorks(t *testing.T) {
	e := newEnv246(t)
	fullUpdateBumpers246(t, e, func(b map[string]*game.Bumper) { b[e.mamie].Name = "Mamie Nova" })

	if got := e.app.engine.GetBumper(e.mamie).Name; got != "Mamie Nova" {
		t.Fatalf("non-régression : un renommage vers un nom libre doit passer : %q", got)
	}
	if e.countLog(game.LogLevelWarn, "VPLAYER_DUPLICATE_NAME") != 0 {
		t.Errorf("pas d'avertissement pour un nom libre")
	}
}

// --- DUPLICATE_SESSION ----------------------------------------------------------------

func TestDuplicateSession246_MostRecentConnectionWinsAndOldOneIsEvicted(t *testing.T) {
	e := newEnv246(t)
	oldConn, oldID := e.dial(t, "/ws/player")
	e.connect(t, oldID, "Paul", e.paul)
	newConn, newID := e.dial(t, "/ws/player")

	e.connect(t, newID, "Paul", e.paul)

	raw := readActionMatchingWithin(t, oldConn, protocol.ActionPlayerEvicted, 2*time.Second)
	var ev struct {
		Reason string `json:"REASON"`
	}
	_ = json.Unmarshal(raw, &ev)
	if ev.Reason != "DUPLICATE_SESSION" {
		t.Fatalf("l'ancienne connexion doit recevoir PLAYER_EVICTED{DUPLICATE_SESSION} : %q", ev.Reason)
	}
	if got, ok := e.app.wsHub.GetClientPlayerID(oldID); ok {
		t.Errorf("l'ancienne connexion doit être déliée, encore liée à %q", got)
	}
	if got, ok := e.app.wsHub.GetClientPlayerID(newID); !ok || got != e.paul {
		t.Errorf("la connexion la plus récente doit être liée à Paul : (%q, %v)", got, ok)
	}
	readActionMatchingWithin(t, newConn, protocol.ActionPlayerConnected, 2*time.Second)
	if !e.hasLog(game.LogLevelWarn, "VPLAYER_DUPLICATE_SESSION", e.paul) {
		t.Errorf("WARN VPLAYER_DUPLICATE_SESSION attendu ; logs=%v", e.logLines(game.LogLevelWarn))
	}

	// L'ancienne connexion ne peut plus écrire, la nouvelle oui.
	e.send(t, oldID, "vplayer", protocol.ActionArdoiseInput, protocol.ArdoiseInputPayload{ID: e.paul, Text: "ANCIEN"})
	if text, _ := e.answers(); text["Les Bleus"] != "" {
		t.Errorf("l'ancienne connexion évincée ne doit plus écrire : %v", text)
	}
	e.send(t, newID, "vplayer", protocol.ActionArdoiseInput, protocol.ArdoiseInputPayload{ID: e.paul, Text: "NOUVEAU"})
	if text, _ := e.answers(); text["Les Bleus"] != "NOUVEAU" {
		t.Errorf("la nouvelle connexion écrit : %v", text)
	}
}

// L'éviction DUPLICATE_SESSION n'est PAS une suppression : le bumper reste, connecté,
// même après la fermeture de l'ancienne connexion.
func TestDuplicateSession246_BumperSurvivesTheEvictedConnectionClosing(t *testing.T) {
	e := newEnv246(t)
	oldConn, oldID := e.dial(t, "/ws/player")
	e.connect(t, oldID, "Paul", e.paul)
	_, newID := e.dial(t, "/ws/player")
	e.connect(t, newID, "Paul", e.paul)
	readActionMatchingWithin(t, oldConn, protocol.ActionPlayerEvicted, 2*time.Second)

	oldConn.Close()
	deadline := time.Now().Add(600 * time.Millisecond)
	for time.Now().Before(deadline) {
		b := e.app.engine.GetBumper(e.paul)
		if b == nil || !b.Connected {
			t.Fatalf("le bumper ne doit ni disparaître ni passer hors ligne : %+v", b)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Le même client qui renvoie PLAYER_CONNECT (reconnexion de la même connexion, « Reprendre ici »)
// ne s'évince pas lui-même.
func TestDuplicateSession246_SameConnectionReconnectingDoesNotEvictItself(t *testing.T) {
	e := newEnv246(t)
	conn, cid := e.dial(t, "/ws/player")
	e.connect(t, cid, "Paul", e.paul)
	e.connect(t, cid, "Paul", e.paul)

	for _, a := range collectAfterSentinel264(t, e.app, conn)[0] { // #266 : témoin, pas de fenêtre fixe
		if a == protocol.ActionPlayerEvicted {
			t.Fatal("une connexion ne doit pas recevoir PLAYER_EVICTED pour son propre PLAYER_CONNECT")
		}
	}
	if got, ok := e.app.wsHub.GetClientPlayerID(cid); !ok || got != e.paul {
		t.Errorf("la connexion reste liée : (%q, %v)", got, ok)
	}
}
