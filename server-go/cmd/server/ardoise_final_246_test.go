package main

// #246 (v11.2) — garde-fous au STOP : ARDOISE_FINAL / ARDOISE_FINAL_ACK, RECEIVED, garde de
// REVEAL (équipes attendues, tablette hors ligne comprise), FORCE, REVEAL_REFUSED.
// Contrat : contracts/websocket-actions.md §ARDOISE_FINAL, §ARDOISE_FINAL_ACK, §REVEAL,
// §REVEAL_REFUSED ; contracts/game-state.md §ARDOISE_ANSWERS ; contracts/logging.md §3.1.
//
// Tests du BATCH 2 (B-STOP), écrits depuis les contrats ; permanents depuis la livraison de B-STOP.

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ardoiseHashVectors : les 6 vecteurs du contrat (Go et JS doivent produire exactement ces valeurs).
var ardoiseHashVectors = []struct{ text, hash string }{
	{"", "811c9dc5"},
	{"a", "e40c292c"},
	{"Paris", "246f9d40"},
	{"PARIS EN FRANCE", "066a01e6"},
	{"Équipe à côté", "c3ff86c2"},
	{"  espaces  ", "1e009bcf"},
}

func fnv1a246(text string) string {
	h := fnv.New32a()
	h.Write([]byte(text)) // UTF-8
	return fmt.Sprintf("%08x", h.Sum32())
}

// Toujours actif : le hash que le serveur recalculera (FNV-1a 32 bits, UTF-8) égale les vecteurs du contrat.
func TestArdoiseFinal246_HashVectorsMatchTheContract(t *testing.T) {
	for _, v := range ardoiseHashVectors {
		if got := fnv1a246(v.text); got != v.hash {
			t.Errorf("FNV-1a(%q) = %s, contrat = %s", v.text, got, v.hash)
		}
	}
}

// --- aides ---------------------------------------------------------------------------

func (e *env246) stop() { e.app.engine.Stop() }

// setOffline marque le bumper déconnecté par la voie publique (UPDATE admin avec CONNECTED=false) :
// GetBumper renvoie une copie, une mutation directe serait sans effet.
func (e *env246) setOffline(t *testing.T, id string) {
	t.Helper()
	fullUpdateBumpers246(t, e, func(b map[string]*game.Bumper) { b[id].Connected = false })
	if b := e.app.engine.GetBumper(id); b == nil || b.Connected {
		t.Skip("précondition : le bumper ne peut pas être mis hors ligne par UPDATE dans ce montage")
	}
}

// finalMsg : payload ARDOISE_FINAL valide pour le bumper `id` (hash recalculé).
func finalPayload246(id, questionID, text string) map[string]interface{} {
	return map[string]interface{}{"ID": id, "QUESTION_ID": questionID, "TEXT": text, "HASH": fnv1a246(text)}
}

// readUpdateWhere lit la connexion jusqu'à un UPDATE dont GAME.ARDOISE_ANSWERS[team].RECEIVED == want.
func readUpdateWithReceived(t *testing.T, conn *websocket.Conn, team string, want bool, within time.Duration) (json.RawMessage, time.Duration) {
	t.Helper()
	start := time.Now()
	deadline := start.Add(within)
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("pas d'UPDATE avec %s.RECEIVED=%v dans %v : %v", team, want, within, err)
		}
		var env struct {
			Action string `json:"ACTION"`
			Msg    struct {
				Game struct {
					A map[string]struct {
						Received bool `json:"RECEIVED"`
					} `json:"ARDOISE_ANSWERS"`
				} `json:"GAME"`
			} `json:"MSG"`
		}
		if json.Unmarshal(data, &env) == nil && env.Action == "UPDATE" {
			if a, ok := env.Msg.Game.A[team]; ok && a.Received == want {
				return data, time.Since(start)
			}
		}
	}
	t.Fatalf("timeout : pas d'UPDATE avec %s.RECEIVED=%v", team, want)
	return nil, 0
}

func noAction246(t *testing.T, conn *websocket.Conn, action string, window time.Duration) {
	t.Helper()
	for _, a := range collectActions(conn, window) {
		if a == action {
			t.Fatalf("%s inattendu", action)
		}
	}
}

// --- ARDOISE_FINAL ----------------------------------------------------------------------

func TestArdoiseFinal246_Critical_AcceptedFinalSetsReceivedAndAcksOnlyTheSender(t *testing.T) {
	e := newEnv246(t)
	paulConn, paul := e.dial(t, "/ws/player")
	e.connect(t, paul, "Paul", e.paul)
	mamieConn, mamie := e.dial(t, "/ws/player")
	e.connect(t, mamie, "Mamie", e.mamie)
	adminConn, _ := e.dial(t, "/ws/admin")
	animConn, _ := e.dial(t, "/ws/anim")
	e.typeAs(t, paul, e.paul, "PARIS EN FRAN", nil) // la troncature C2 : le final porte le texte COMPLET
	e.stop()

	e.send(t, paul, "vplayer", "ARDOISE_FINAL", finalPayload246(e.paul, "q-ardoise", "PARIS EN FRANCE"))

	raw := readActionMatchingWithin(t, paulConn, "ARDOISE_FINAL_ACK", 2*time.Second)
	var ack struct {
		QuestionID string `json:"QUESTION_ID"`
		Hash       string `json:"HASH"`
	}
	_ = json.Unmarshal(raw, &ack)
	if ack.QuestionID != "q-ardoise" || ack.Hash != "066a01e6" {
		t.Errorf("ACK = %+v", ack)
	}
	if strings.Contains(string(raw), "PARIS") || strings.Contains(strings.ToUpper(string(raw)), "TEXT") {
		t.Errorf("l'accusé ne contient jamais de texte (confidentialité #128) : %s", raw)
	}
	text, received := e.answers()
	if text["Les Bleus"] != "PARIS EN FRANCE" || !received["Les Bleus"] || received["Les Rouges"] {
		t.Errorf("texte complet + RECEIVED seulement pour Paul : %v %v", text, received)
	}
	if h := string(e.answersRaw("Les Bleus")["RECEIVED_HASH"]); h != `"066a01e6"` {
		t.Errorf("RECEIVED_HASH = %s", h)
	}
	// diffusion immédiate (non coalescée, < 150 ms) vers admin ET anim
	for name, c := range map[string]*websocket.Conn{"admin": adminConn, "anim": animConn} {
		_, took := readUpdateWithReceived(t, c, "Les Bleus", true, 2*time.Second)
		if took > 140*time.Millisecond {
			t.Errorf("UPDATE %s non immédiat (coalescé ?) : %v", name, took)
		}
	}
	// accusé ciblé : ni l'autre tablette, ni l'admin, ni l'anim
	noAction246(t, mamieConn, "ARDOISE_FINAL_ACK", 300*time.Millisecond)
	noAction246(t, adminConn, "ARDOISE_FINAL_ACK", 100*time.Millisecond)
	noAction246(t, animConn, "ARDOISE_FINAL_ACK", 100*time.Millisecond)
}

func TestArdoiseFinal246_EmptyTextIsAcceptedAsNothingWritten(t *testing.T) {
	e := newEnv246(t)
	conn, cid := e.dial(t, "/ws/player")
	e.connect(t, cid, "Paul", e.paul)
	e.stop()

	e.send(t, cid, "vplayer", "ARDOISE_FINAL", finalPayload246(e.paul, "q-ardoise", ""))

	raw := readActionMatchingWithin(t, conn, "ARDOISE_FINAL_ACK", 2*time.Second)
	if !strings.Contains(string(raw), "811c9dc5") {
		t.Errorf("hash du texte vide attendu : %s", raw)
	}
	if _, received := e.answers(); !received["Les Bleus"] {
		t.Error("\"\" autorisé (« rien écrit ») : RECEIVED=true")
	}
}

func TestArdoiseFinal246_RefusedCasesAreIgnoredLoggedAndNeverAcked(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(e *env246)
		payload func(e *env246) map[string]interface{}
		event   string
	}{
		{"mauvais hash", func(e *env246) { e.stop() },
			func(e *env246) map[string]interface{} {
				p := finalPayload246(e.paul, "q-ardoise", "PARIS")
				p["HASH"] = "deadbeef"
				return p
			}, "ARDOISE_FINAL_HASH_MISMATCH"},
		{"autre question", func(e *env246) { e.stop() },
			func(e *env246) map[string]interface{} { return finalPayload246(e.paul, "q-autre", "PARIS") }, "ARDOISE_FINAL_STALE"},
		{"hors phase (STARTED)", func(e *env246) {},
			func(e *env246) map[string]interface{} { return finalPayload246(e.paul, "q-ardoise", "PARIS") }, "ARDOISE_FINAL_STALE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv246(t)
			conn, cid := e.dial(t, "/ws/player")
			e.connect(t, cid, "Paul", e.paul)
			tc.prepare(e)

			e.send(t, cid, "vplayer", "ARDOISE_FINAL", tc.payload(e))

			if !e.waitLog(t, game.LogLevelWarn, tc.event) {
				t.Fatalf("WARN %s attendu ; logs=%v", tc.event, e.logLines(game.LogLevelWarn))
			}
			noAction246(t, conn, "ARDOISE_FINAL_ACK", 300*time.Millisecond)
			if _, received := e.answers(); received["Les Bleus"] {
				t.Error("RECEIVED ne doit pas passer à true")
			}
		})
	}
}

func TestArdoiseFinal246_Critical_UnboundOrForeignConnectionIsRefused(t *testing.T) {
	e := newEnv246(t)
	_, unbound := e.dial(t, "/ws/player")
	_, bound := e.dial(t, "/ws/player")
	e.connect(t, bound, "Paul", e.paul)
	e.stop()

	e.send(t, unbound, "vplayer", "ARDOISE_FINAL", finalPayload246(e.paul, "q-ardoise", "PIRATE"))
	e.send(t, bound, "vplayer", "ARDOISE_FINAL", finalPayload246(e.mamie, "q-ardoise", "PIRATE"))

	if !e.hasLog(game.LogLevelWarn, "PLAYER_ACTION_UNBOUND", "ARDOISE_FINAL") || !e.hasLog(game.LogLevelWarn, "PLAYER_FOREIGN_ID", "ARDOISE_FINAL") {
		t.Errorf("les deux refus doivent être journalisés ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	if text, received := e.answers(); text["Les Rouges"] != "" || received["Les Rouges"] || received["Les Bleus"] {
		t.Errorf("rien ne doit être écrit : %v %v", text, received)
	}
}

func TestArdoiseFinal246_IsIdempotentOnTheSameHash(t *testing.T) {
	e := newEnv246(t)
	conn, cid := e.dial(t, "/ws/player")
	e.connect(t, cid, "Paul", e.paul)
	e.stop()
	e.send(t, cid, "vplayer", "ARDOISE_FINAL", finalPayload246(e.paul, "q-ardoise", "PARIS"))
	readActionMatchingWithin(t, conn, "ARDOISE_FINAL_ACK", 2*time.Second)
	first := string(e.answersRaw("Les Bleus")["SUBMITTED_AT"])
	time.Sleep(5 * time.Millisecond)

	e.send(t, cid, "vplayer", "ARDOISE_FINAL", finalPayload246(e.paul, "q-ardoise", "PARIS")) // renvoi de la tablette

	readActionMatchingWithin(t, conn, "ARDOISE_FINAL_ACK", 2*time.Second) // la tablette doit encore être accusée
	if again := string(e.answersRaw("Les Bleus")["SUBMITTED_AT"]); again != first {
		t.Errorf("même hash déjà reçu : pas de réécriture (SUBMITTED_AT %s → %s)", first, again)
	}
}

func TestArdoiseFinal246_StartedAtIsSetWhenTheFirstNonEmptyTextArrivesByFinal(t *testing.T) {
	e := newEnv246(t)
	_, cid := e.dial(t, "/ws/player")
	e.connect(t, cid, "Paul", e.paul)
	e.stop()

	e.send(t, cid, "vplayer", "ARDOISE_FINAL", finalPayload246(e.paul, "q-ardoise", "PARIS"))

	var started int64
	_ = json.Unmarshal(e.answersRaw("Les Bleus")["STARTED_AT"], &started)
	if started == 0 {
		t.Error("STARTED_AT doit être fixé si le final porte le 1er texte non vide")
	}
}

func TestArdoiseFinal246_ReceivedAndHashAreAlwaysSerializedAndInputLeavesThemAlone(t *testing.T) {
	e := newEnv246(t)
	_, cid := e.dial(t, "/ws/player")
	e.connect(t, cid, "Paul", e.paul)

	e.typeAs(t, cid, e.paul, "PA", nil)

	raw := e.answersRaw("Les Bleus")
	for _, k := range []string{"RECEIVED", "RECEIVED_HASH"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("%s doit être sérialisé même à false/vide (pas d'omitempty) : %v", k, raw)
		}
	}
	if string(raw["RECEIVED"]) != "false" {
		t.Errorf("RECEIVED = %s", raw["RECEIVED"])
	}
}

func TestArdoiseFinal246_PauseKeepsFinalsAndContinueResetsReceived(t *testing.T) {
	e := newEnv246(t)
	_, cid := e.dial(t, "/ws/player")
	e.connect(t, cid, "Paul", e.paul)
	e.typeAs(t, cid, e.paul, "PAR", nil)
	e.app.engine.PauseAll()

	e.send(t, cid, "vplayer", "ARDOISE_FINAL", finalPayload246(e.paul, "q-ardoise", "PARIS"))
	if _, received := e.answers(); !received["Les Bleus"] {
		t.Fatal("PAUSE → le final est accepté (phase PAUSED)")
	}

	e.app.engine.Continue()

	if _, received := e.answers(); received["Les Bleus"] || received["Les Rouges"] {
		t.Errorf("CONTINUE remet RECEIVED à false partout : %v", received)
	}
	if text, _ := e.answers(); text["Les Bleus"] != "PARIS" {
		t.Errorf("le texte est conservé, la saisie reprend dessus : %v", text)
	}
}

func TestArdoiseFinal246_NewQuestionResetsReceived(t *testing.T) {
	e := newEnv246(t)
	_, cid := e.dial(t, "/ws/player")
	e.connect(t, cid, "Paul", e.paul)
	e.stop()
	e.send(t, cid, "vplayer", "ARDOISE_FINAL", finalPayload246(e.paul, "q-ardoise", "PARIS"))

	e.app.engine.Ready("", &game.Question{ID: "q2", Type: game.QuestionTypeArdoise})

	if _, received := e.answers(); received["Les Bleus"] {
		t.Error("une nouvelle question repart avec RECEIVED=false")
	}
}

// --- Garde de REVEAL -------------------------------------------------------------------------

func (e *env246) reveal(t *testing.T, cid, ctype string, force bool) {
	t.Helper()
	var p interface{} = map[string]interface{}{}
	if force {
		p = map[string]interface{}{"FORCE": true}
	}
	e.send(t, cid, ctype, protocol.ActionReveal, p)
}

func (e *env246) finalFrom(t *testing.T, cid, id, text string) {
	t.Helper()
	e.send(t, cid, "vplayer", "ARDOISE_FINAL", finalPayload246(id, "q-ardoise", text))
}

// Critical — garde REVEAL : refus tant qu'une équipe attendue n'est pas reçue.
func TestRevealGuard246_Critical_RefusedWhileAnExpectedTeamIsPending(t *testing.T) {
	e := newEnv246(t)
	_, paul := e.dial(t, "/ws/player")
	e.connect(t, paul, "Paul", e.paul)
	adminConn, admin := e.dial(t, "/ws/admin")
	animConn, _ := e.dial(t, "/ws/anim")
	e.typeAs(t, paul, e.paul, "PARIS", nil)
	e.stop()
	e.finalFrom(t, paul, e.paul, "PARIS") // Mamie n'a rien envoyé

	e.reveal(t, admin, "admin", false)

	if e.app.engine.GetPhase() != game.PhaseStopped {
		t.Fatalf("la phase ne change pas : %s", e.app.engine.GetPhase())
	}
	raw := readActionMatchingWithin(t, adminConn, "REVEAL_REFUSED", 2*time.Second)
	var rr struct {
		Reason  string `json:"REASON"`
		Pending []struct {
			Team    string `json:"TEAM"`
			Offline bool   `json:"OFFLINE"`
		} `json:"PENDING"`
	}
	_ = json.Unmarshal(raw, &rr)
	if rr.Reason != "ARDOISE_PENDING" || len(rr.Pending) != 1 || rr.Pending[0].Team != "Les Rouges" {
		t.Errorf("REVEAL_REFUSED = %+v", rr)
	}
	noAction246(t, animConn, "REVEAL_REFUSED", 300*time.Millisecond) // ciblé sur l'émetteur seul
	if !e.hasLog(game.LogLevelWarn, "ARDOISE_REVEAL_REFUSED", "Les Rouges") {
		t.Errorf("WARN ARDOISE_REVEAL_REFUSED attendu ; logs=%v", e.logLines(game.LogLevelWarn))
	}
}

func TestRevealGuard246_AnimIsGuardedToo(t *testing.T) {
	e := newEnv246(t)
	animConn, anim := e.dial(t, "/ws/anim")
	e.stop()

	e.reveal(t, anim, "anim", false)

	readActionMatchingWithin(t, animConn, "REVEAL_REFUSED", 2*time.Second)
	if e.app.engine.GetPhase() != game.PhaseStopped {
		t.Errorf("phase = %s", e.app.engine.GetPhase())
	}
}

// Critical — tablette hors ligne comprise (décision utilisateur) : le refus tient, OFFLINE=true.
func TestRevealGuard246_Critical_OfflineTabletStillBlocksAndIsFlagged(t *testing.T) {
	e := newEnv246(t)
	_, paul := e.dial(t, "/ws/player")
	e.connect(t, paul, "Paul", e.paul)
	adminConn, admin := e.dial(t, "/ws/admin")
	e.setOffline(t, e.mamie) // la tablette de Mamie est coupée
	e.stop()
	e.finalFrom(t, paul, e.paul, "PARIS")

	e.reveal(t, admin, "admin", false)

	raw := readActionMatchingWithin(t, adminConn, "REVEAL_REFUSED", 2*time.Second)
	var rr struct {
		Pending []struct {
			Team    string `json:"TEAM"`
			Offline bool   `json:"OFFLINE"`
		} `json:"PENDING"`
	}
	_ = json.Unmarshal(raw, &rr)
	if len(rr.Pending) != 1 || rr.Pending[0].Team != "Les Rouges" || !rr.Pending[0].Offline {
		t.Errorf("PENDING doit contenir Les Rouges avec OFFLINE=true : %s", raw)
	}
	if e.app.engine.GetPhase() != game.PhaseStopped {
		t.Errorf("hors ligne ne lève pas la garde : phase=%s", e.app.engine.GetPhase())
	}
}

// Tablette coupée au STOP puis revenue avant le REVEAL : son final arrive, REVEAL se débloque.
func TestRevealGuard246_ReconnectedTabletUnblocksTheReveal(t *testing.T) {
	e := newEnv246(t)
	_, paul := e.dial(t, "/ws/player")
	e.connect(t, paul, "Paul", e.paul)
	_, admin := e.dial(t, "/ws/admin")
	e.setOffline(t, e.mamie)
	e.stop()
	e.finalFrom(t, paul, e.paul, "PARIS")
	e.reveal(t, admin, "admin", false)
	if e.app.engine.GetPhase() != game.PhaseStopped {
		t.Fatal("précondition : refusé")
	}

	_, mamie := e.dial(t, "/ws/player") // elle revient
	e.connect(t, mamie, "Mamie", e.mamie)
	e.finalFrom(t, mamie, e.mamie, "LYON")
	e.reveal(t, admin, "admin", false)

	if e.app.engine.GetPhase() != game.PhaseRevealed {
		t.Fatalf("toutes les équipes reçues : REVEAL passe, phase=%s", e.app.engine.GetPhase())
	}
	if e.countLog(game.LogLevelWarn, "ARDOISE_REVEAL_FORCED") != 0 {
		t.Error("pas de forçage ici")
	}
}

func TestRevealGuard246_ForceRevealsAndMarksMissingTeamsNotReceived(t *testing.T) {
	e := newEnv246(t)
	_, paul := e.dial(t, "/ws/player")
	e.connect(t, paul, "Paul", e.paul)
	_, mamie := e.dial(t, "/ws/player")
	e.connect(t, mamie, "Mamie", e.mamie)
	adminConn, admin := e.dial(t, "/ws/admin")
	e.typeAs(t, mamie, e.mamie, "LYO", nil) // dernier texte reçu pendant STARTED
	e.stop()
	e.finalFrom(t, paul, e.paul, "PARIS")

	e.reveal(t, admin, "admin", true)

	if e.app.engine.GetPhase() != game.PhaseRevealed {
		t.Fatalf("FORCE révèle immédiatement : phase=%s", e.app.engine.GetPhase())
	}
	text, received := e.answers()
	if text["Les Rouges"] != "LYO" || received["Les Rouges"] || !received["Les Bleus"] {
		t.Errorf("l'équipe manquante garde son dernier texte avec RECEIVED=false : %v %v", text, received)
	}
	if !e.hasLog(game.LogLevelWarn, "ARDOISE_REVEAL_FORCED", "Les Rouges") {
		t.Errorf("WARN ARDOISE_REVEAL_FORCED (équipes non reçues) attendu ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	noAction246(t, adminConn, "REVEAL_REFUSED", 200*time.Millisecond)
}

func TestRevealGuard246_ForceWithNothingMissingIsNotLoggedAsForced(t *testing.T) {
	e := newEnv246(t)
	_, paul := e.dial(t, "/ws/player")
	e.connect(t, paul, "Paul", e.paul)
	_, mamie := e.dial(t, "/ws/player")
	e.connect(t, mamie, "Mamie", e.mamie)
	_, admin := e.dial(t, "/ws/admin")
	e.stop()
	e.finalFrom(t, paul, e.paul, "PARIS")
	e.finalFrom(t, mamie, e.mamie, "LYON")

	e.reveal(t, admin, "admin", true)

	if e.app.engine.GetPhase() != game.PhaseRevealed || e.countLog(game.LogLevelWarn, "ARDOISE_REVEAL_FORCED") != 0 {
		t.Errorf("phase=%s ; forçage journalisé à tort", e.app.engine.GetPhase())
	}
}

// Équipe attendue = clé de teams avec ≥ 1 bumper IS_VPLAYER ; une équipe sans VJoueur n'est pas attendue.
func TestRevealGuard246_TeamWithoutVPlayerIsNotExpected(t *testing.T) {
	e := newEnv246(t)
	e.app.engine.SetTeams(map[string]*game.Team{"Les Bleus": {Name: "Les Bleus"}, "Les Rouges": {Name: "Les Rouges"}, "Les Verts": {Name: "Les Verts"}})
	_, paul := e.dial(t, "/ws/player")
	e.connect(t, paul, "Paul", e.paul)
	_, mamie := e.dial(t, "/ws/player")
	e.connect(t, mamie, "Mamie", e.mamie)
	_, admin := e.dial(t, "/ws/admin")
	e.stop()
	e.finalFrom(t, paul, e.paul, "PARIS")
	e.finalFrom(t, mamie, e.mamie, "LYON")

	e.reveal(t, admin, "admin", false)

	if e.app.engine.GetPhase() != game.PhaseRevealed {
		t.Fatalf("« Les Verts » n'a aucun VJoueur : elle n'est pas attendue ; phase=%s", e.app.engine.GetPhase())
	}
}

func TestRevealGuard246_NonArdoiseQuestionsAreUnchanged(t *testing.T) {
	e := newEnv246(t)
	_, admin := e.dial(t, "/ws/admin")
	e.app.engine.SetPhase(game.PhaseStopped)
	e.app.engine.Ready("", &game.Question{ID: "q-speedy", Type: game.QuestionTypeSpeedy})
	e.app.engine.SetPhase(game.PhaseStarted)
	e.stop()

	e.reveal(t, admin, "admin", false)

	if e.app.engine.GetPhase() != game.PhaseRevealed {
		t.Fatalf("hors ARDOISE, REVEAL est inchangé : phase=%s", e.app.engine.GetPhase())
	}
}

func TestRevealGuard246_RevealFromPausedIsAlsoGuarded(t *testing.T) {
	e := newEnv246(t)
	adminConn, admin := e.dial(t, "/ws/admin")
	e.app.engine.PauseAll()

	e.reveal(t, admin, "admin", false)

	readActionMatchingWithin(t, adminConn, "REVEAL_REFUSED", 2*time.Second)
	if e.app.engine.GetPhase() != game.PhasePaused {
		t.Errorf("phase=%s", e.app.engine.GetPhase())
	}
}

// Confidentialité #128 : aucun texte d'une équipe ne parvient aux VJoueurs, accusé compris.
func TestArdoiseFinal246_NoTextEverReachesAVPlayer(t *testing.T) {
	e := newEnv246(t)
	_, paul := e.dial(t, "/ws/player")
	e.connect(t, paul, "Paul", e.paul)
	mamieConn, mamie := e.dial(t, "/ws/player")
	e.connect(t, mamie, "Mamie", e.mamie)
	e.typeAs(t, paul, e.paul, "SECRET-DE-PAUL", nil)
	e.stop()
	e.finalFrom(t, paul, e.paul, "SECRET-DE-PAUL")
	e.app.broadcastUpdate()

	_ = mamie
	mamieConn.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	for {
		_, data, err := mamieConn.ReadMessage()
		if err != nil {
			break
		}
		if strings.Contains(string(data), "SECRET-DE-PAUL") {
			t.Fatalf("fuite du texte d'une équipe vers une tablette : %s", data)
		}
	}
}
