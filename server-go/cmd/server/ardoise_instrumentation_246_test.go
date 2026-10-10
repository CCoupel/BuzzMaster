package main

// #246 (v11.2) — instrumentation : niveau de la ligne par frappe, ARDOISE_CLEARED, SEQ
// (gap/stale), invariant ARDOISE_STATE_DIVERGED (par injection), CLIENT_TRACE (allow-list,
// limites). Contrats : contracts/logging.md §2-§3, contracts/websocket-actions.md
// §ARDOISE_INPUT / §CLIENT_TRACE. Les événements sont lus dans le tampon app.logger.

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"strings"
	"testing"
	"time"
)

// typeAs246 : l'ID `id` envoie ARDOISE_INPUT (diagnostic compris) depuis la connexion cid.
func (e *env246) typeAs(t *testing.T, cid, id, text string, extra map[string]interface{}) {
	t.Helper()
	p := map[string]interface{}{"ID": id, "TEXT": text}
	for k, v := range extra {
		p[k] = v
	}
	e.send(t, cid, "vplayer", protocol.ActionArdoiseInput, p)
}

func (e *env246) boundPaul(t *testing.T) string {
	t.Helper()
	_, cid := e.dial(t, "/ws/player")
	e.connect(t, cid, "Paul", e.paul)
	return cid
}

// --- Niveau de la ligne par frappe -------------------------------------------------

func TestInstrumentation246_KeystrokeLineIsDebugNotInfo(t *testing.T) {
	e := newEnv246(t)
	cid := e.boundPaul(t)

	e.typeAs(t, cid, e.paul, "PARIS", nil)

	if n := e.countLog(game.LogLevelInfo, "ARDOISE_INPUT: team="); n != 0 {
		t.Fatalf("la ligne par frappe ne doit plus être en INFO (elle noyait le tampon) : %d entrée(s)", n)
	}
	e.app.logger.SetDebugEnabled(true)
	e.typeAs(t, cid, e.paul, "PARIS 2", nil)
	if e.countLog(game.LogLevelDebug, "ARDOISE_INPUT") == 0 {
		t.Errorf("la ligne par frappe doit exister en DEBUG ; logs=%v", e.logLines(""))
	}
}

// --- ARDOISE_CLEARED ---------------------------------------------------------------

func TestInstrumentation246_ArdoiseClearedCarriesTheReason(t *testing.T) {
	cases := []struct{ reason, want string }{
		{"clear", "reason=clear"},
		{"backspace_empty", "reason=backspace_empty"},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			e := newEnv246(t)
			cid := e.boundPaul(t)
			e.typeAs(t, cid, e.paul, "PARIS", map[string]interface{}{"SEQ": 1, "REASON": "key"})
			e.typeAs(t, cid, e.paul, "", map[string]interface{}{"SEQ": 2, "REASON": tc.reason})

			if !e.hasLog(game.LogLevelWarn, "ARDOISE_CLEARED", tc.want) {
				t.Fatalf("WARN ARDOISE_CLEARED %s attendu ; logs=%v", tc.want, e.logLines(game.LogLevelWarn))
			}
			for _, field := range []string{"team=", "bumper=", "client=", "prev_len=5", "seq=2", "question=", "phase="} {
				if !e.hasLog(game.LogLevelWarn, "ARDOISE_CLEARED", field) {
					t.Errorf("ARDOISE_CLEARED doit porter %q ; logs=%v", field, e.logLines(game.LogLevelWarn))
				}
			}
			if text, _ := e.answers(); text["Les Bleus"] != "" {
				t.Errorf("la remise à vide demandée par la tablette reste appliquée : %v", text)
			}
		})
	}
}

func TestInstrumentation246_NoArdoiseClearedWhenTheTextIsNotClearedFromNonEmpty(t *testing.T) {
	e := newEnv246(t)
	cid := e.boundPaul(t)
	e.typeAs(t, cid, e.paul, "", nil)    // vide → vide
	e.typeAs(t, cid, e.paul, "AB", nil)  // vide → non vide
	e.typeAs(t, cid, e.paul, "ABC", nil) // non vide → non vide
	e.typeAs(t, cid, e.paul, "AB", nil)  // non vide → plus court non vide

	if n := e.countLog("", "ARDOISE_CLEARED"); n != 0 {
		t.Fatalf("ARDOISE_CLEARED seulement pour non vide → \"\" : %d entrée(s) ; logs=%v", n, e.logLines(""))
	}
}

// --- SEQ -----------------------------------------------------------------------------

func TestInstrumentation246_SeqGapIsLoggedAndTheMessageApplied(t *testing.T) {
	e := newEnv246(t)
	cid := e.boundPaul(t)
	e.typeAs(t, cid, e.paul, "A", map[string]interface{}{"SEQ": 1})
	e.typeAs(t, cid, e.paul, "AB", map[string]interface{}{"SEQ": 2})

	e.typeAs(t, cid, e.paul, "ABCDE", map[string]interface{}{"SEQ": 5}) // 3 et 4 perdus

	if !e.hasLog(game.LogLevelWarn, "ARDOISE_SEQ_GAP", "expected=3", "got=5") {
		t.Fatalf("WARN ARDOISE_SEQ_GAP expected=3 got=5 attendu ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	if text, _ := e.answers(); text["Les Bleus"] != "ABCDE" {
		t.Errorf("un trou n'empêche pas d'appliquer le message : %v", text)
	}
}

func TestInstrumentation246_SeqStaleIsLoggedAndTheMessageIgnored(t *testing.T) {
	e := newEnv246(t)
	cid := e.boundPaul(t)
	e.typeAs(t, cid, e.paul, "A", map[string]interface{}{"SEQ": 1})
	e.typeAs(t, cid, e.paul, "ABC", map[string]interface{}{"SEQ": 2})

	e.typeAs(t, cid, e.paul, "AB", map[string]interface{}{"SEQ": 2}) // doublon
	e.typeAs(t, cid, e.paul, "", map[string]interface{}{"SEQ": 1})   // plus ancien

	if e.countLog(game.LogLevelWarn, "ARDOISE_SEQ_STALE") != 2 {
		t.Fatalf("2 × WARN ARDOISE_SEQ_STALE attendus ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	if !e.hasLog(game.LogLevelWarn, "ARDOISE_SEQ_STALE", "last=2", "got=1") {
		t.Errorf("champs last/got attendus ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	if text, _ := e.answers(); text["Les Bleus"] != "ABC" {
		t.Errorf("un message périmé est ignoré (le texte reste ABC) : %v", text)
	}
	if e.countLog("", "ARDOISE_CLEARED") != 0 {
		t.Errorf("un message périmé ignoré ne peut pas vider la réponse")
	}
}

func TestInstrumentation246_SeqIsPerConnectionAndPerQuestionAndOptional(t *testing.T) {
	e := newEnv246(t)
	cid := e.boundPaul(t)
	e.typeAs(t, cid, e.paul, "A", map[string]interface{}{"SEQ": 1})
	e.typeAs(t, cid, e.paul, "AB", map[string]interface{}{"SEQ": 2})
	e.typeAs(t, cid, e.paul, "ABC", nil) // SEQ absent : pas de contrôle

	// nouvelle connexion (reconnexion) : le compteur repart à 1, ni gap ni stale
	_, cid2 := e.dial(t, "/ws/player")
	e.connect(t, cid2, "Paul", e.paul)
	e.typeAs(t, cid2, e.paul, "ABCD", map[string]interface{}{"SEQ": 1})

	// nouvelle question : compteur remis à 1 aussi
	e.app.engine.SetPhase(game.PhaseStopped)
	e.app.engine.Ready("", &game.Question{ID: "q-ardoise-2", Type: game.QuestionTypeArdoise})
	e.app.engine.SetPhase(game.PhaseStarted)
	e.typeAs(t, cid2, e.paul, "Z", map[string]interface{}{"SEQ": 1})

	if n := e.countLog(game.LogLevelWarn, "ARDOISE_SEQ_"); n != 0 {
		t.Fatalf("aucun GAP/STALE attendu (SEQ absent, nouvelle connexion, nouvelle question) ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	if text, _ := e.answers(); text["Les Bleus"] != "Z" {
		t.Errorf("SEQ=1 de la nouvelle question appliqué : %v", text)
	}
}

// --- Invariant ARDOISE_STATE_DIVERGED (par injection) ------------------------------------

func (e *env246) injectDirectWrite(team, text string) {
	// Écriture directe hors des chemins autorisés (la map est partagée avec le moteur).
	e.app.engine.GetState().ArdoiseAnswers[team] = game.ArdoiseAnswer{Text: text}
}

func TestInvariant246_DirectWriteIsDetectedAtPhaseChange(t *testing.T) {
	e := newEnv246(t)
	cid := e.boundPaul(t)
	e.typeAs(t, cid, e.paul, "PARIS", map[string]interface{}{"SEQ": 1})

	e.injectDirectWrite("Les Bleus", "CORROMPU")
	e.app.engine.SetPhase(game.PhaseStopped) // changement de phase = point de contrôle

	if !e.hasLog(game.LogLevelError, "ARDOISE_STATE_DIVERGED", `team="Les Bleus"`, "PARIS", "CORROMPU") {
		t.Fatalf("ERROR ARDOISE_STATE_DIVERGED (team, expected=PARIS, actual=CORROMPU) attendu ; logs=%v", e.logLines(""))
	}
	for _, f := range []string{"expected=", "actual=", "site=", "phase=", "question="} {
		if !e.hasLog(game.LogLevelError, "ARDOISE_STATE_DIVERGED", f) {
			t.Errorf("ARDOISE_STATE_DIVERGED doit porter %q", f)
		}
	}
}

func TestInvariant246_DirectWriteIsDetectedAtFullStateBroadcast(t *testing.T) {
	e := newEnv246(t)
	cid := e.boundPaul(t)
	e.typeAs(t, cid, e.paul, "PARIS", nil)
	e.injectDirectWrite("Les Bleus", "CORROMPU")

	e.app.broadcastUpdate() // diffusion d'état complet = point de contrôle

	if !e.hasLog(game.LogLevelError, "ARDOISE_STATE_DIVERGED", "CORROMPU") {
		t.Fatalf("ERROR ARDOISE_STATE_DIVERGED attendu à la diffusion d'état complet ; logs=%v", e.logLines(""))
	}
}

func TestInvariant246_NoFalsePositiveOnLegitimateWrites(t *testing.T) {
	e := newEnv246(t)
	cidP := e.boundPaul(t)
	_, cidM := e.dial(t, "/ws/player")
	e.connect(t, cidM, "Mamie", e.mamie)

	for i, s := range []string{"P", "PA", "PAR", "PARI", "PARIS", "", "PARIS"} {
		e.typeAs(t, cidP, e.paul, s, map[string]interface{}{"SEQ": i + 1})
		e.typeAs(t, cidM, e.mamie, strings.ToUpper(s)+"x", nil)
		e.app.broadcastUpdate()
	}
	e.app.engine.SetPhase(game.PhaseStopped)
	e.app.engine.SetPhase(game.PhaseStarted)
	e.app.engine.SetPhase(game.PhaseStopped)
	e.app.engine.Ready("", &game.Question{ID: "q2", Type: game.QuestionTypeArdoise}) // Ready() remet à zéro : chemin autorisé
	e.app.engine.SetPhase(game.PhaseStarted)
	e.app.broadcastUpdate()

	if n := e.countLog(game.LogLevelError, "ARDOISE_STATE_DIVERGED"); n != 0 {
		t.Fatalf("faux positif : l'invariant se déclenche sur des écritures légitimes ; logs=%v", e.logLines(game.LogLevelError))
	}
}

// --- CLIENT_TRACE ------------------------------------------------------------------------

func TestClientTrace246_IsAllowedForAllFourClientTypes(t *testing.T) {
	for _, ct := range []server.ClientType{server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer, server.ClientTypeAnim} {
		if !server.IsActionAllowed("CLIENT_TRACE", ct) {
			t.Errorf("CLIENT_TRACE doit être autorisé pour %q (allow-list entrante)", ct)
		}
	}
	if server.IsActionAllowed("CLIENT_TRACE", server.ClientTypeBuzzer) {
		t.Errorf("CLIENT_TRACE n'est pas prévu pour les buzzers")
	}
}

func TestClientTrace246_IsLoggedWithItsFieldsAndNeverChangesTheGame(t *testing.T) {
	e := newEnv246(t)
	cid := e.boundPaul(t)
	before := string(e.app.engine.GetGameJSON())

	e.send(t, cid, "vplayer", "CLIENT_TRACE", map[string]interface{}{
		"KIND": "VISIBILITY", "AT": 1791627000123,
		"DETAIL": map[string]interface{}{"STATE": "hidden", "HIDDEN_MS": 1200},
	})
	_, admin := e.dial(t, "/ws/admin")
	e.send(t, admin, "admin", "CLIENT_TRACE", map[string]interface{}{
		"KIND": "ARDOISE_CLEARED_SEEN", "AT": 1791627000456, "TEAM": "Les Verts", "PREV_LEN": 14, "CAUSE_ACTION": "UPDATE",
	})

	if !e.waitLog(t, game.LogLevelWarn, "CLIENT_TRACE") {
		t.Fatalf("WARN CLIENT_TRACE attendu ; logs=%v", e.logLines(""))
	}
	if !e.hasLog(game.LogLevelWarn, "CLIENT_TRACE", "kind=VISIBILITY", e.paul) {
		t.Errorf("la trace de la tablette doit porter kind=VISIBILITY et le player_id lié ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	if !e.hasLog(game.LogLevelWarn, "CLIENT_TRACE", "kind=ARDOISE_CLEARED_SEEN", "Les Verts", "14", "UPDATE") {
		t.Errorf("la trace admin doit porter team, prev_len et cause_action ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	if after := string(e.app.engine.GetGameJSON()); after != before {
		t.Errorf("CLIENT_TRACE n'a aucun effet sur l'état du jeu")
	}
}

func TestClientTrace246_ThrottledAboveTenPerSecondWithASingleWarning(t *testing.T) {
	e := newEnv246(t)
	_, cid := e.dial(t, "/ws/player")
	for i := 0; i < 40; i++ {
		e.send(t, cid, "vplayer", "CLIENT_TRACE", map[string]interface{}{"KIND": "VISIBILITY", "AT": 1791627000000 + i, "DETAIL": map[string]interface{}{"STATE": "visible"}})
	}
	time.Sleep(50 * time.Millisecond)

	logged := 0
	for _, en := range e.app.logger.GetHistory() {
		if strings.Contains(en.Message, "CLIENT_TRACE") && !strings.Contains(en.Message, "CLIENT_TRACE_THROTTLED") {
			logged++
		}
	}
	if logged < 1 || logged > 10 {
		t.Errorf("≤ 10 traces/s journalisées par connexion : %d journalisées sur 40 envoyées", logged)
	}
	if n := e.countLog(game.LogLevelWarn, "CLIENT_TRACE_THROTTLED"); n != 1 {
		t.Errorf("un seul WARN CLIENT_TRACE_THROTTLED par minute et par connexion : %d", n)
	}
}

func TestClientTrace246_PayloadAboveOneKilobyteIsIgnored(t *testing.T) {
	e := newEnv246(t)
	_, cid := e.dial(t, "/ws/player")
	e.send(t, cid, "vplayer", "CLIENT_TRACE", map[string]interface{}{
		"KIND": "WS_GAP", "AT": 1791627000000,
		"DETAIL": map[string]interface{}{"NOTE": strings.Repeat("Z", 2048)},
	})

	for _, en := range e.app.logger.GetHistory() {
		if strings.Contains(en.Message, strings.Repeat("Z", 64)) {
			t.Fatalf("une trace de plus de 1 Ko doit être ignorée, pas journalisée (%d caractères)", len(en.Message))
		}
	}
}
