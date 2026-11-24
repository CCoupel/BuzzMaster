package main

// ---------------------------------------------------------------------------
// #194 (v13.0.0, LOT-2.3) — fin de manche, révélation, crédit, relance, déconnexions de PICTO, écrit depuis
// contracts/picto.md §6.3 à §6.7, §7, §15 (buzzers ignorés), §16 (non-régression). Le code n'était pas final : seuls les
// noms d'actions du contrat sont utilisés (littéraux) ; harnais de picto_wire_192_193_test.go (vraie WebSocket).
// Quatre issues : FOUND, TIMEOUT, STOPPED, NO_WORD ; « Trouvé » sur le gong (I5) ; relance (I6). Critical : un seul
// chemin de révélation, le mot n'entre dans l'état qu'à REVEALED, jamais deux issues.
// ---------------------------------------------------------------------------

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
)

const pictoActFound = "PICTO_FOUND"

func gameStr(t *testing.T, c *wireClient, field string) string {
	t.Helper()
	v, _ := gameField(t, c, field).(string)
	return v
}

func waitPhase(t *testing.T, w *pictoWire, phase string) {
	t.Helper()
	waitGameField(t, w.admin, "PHASE", phase, 5*time.Second)
}

// roundReady : équipe + dessinateur choisis, manche lancée jusqu'à STARTED ; retourne le mot de la manche.
func (w *pictoWire) roundStartedWithDelay(t *testing.T, delay int) string {
	t.Helper()
	w.chooseTeamAndDrawer(t, w.aliceID)
	return w.startRoundWithDelay(t, w.alice, delay)
}

func (w *pictoWire) teamScore(team string) float64 {
	var gd struct {
		Teams map[string]struct {
			Score float64 `json:"SCORE"`
		} `json:"teams"`
	}
	_ = json.Unmarshal(w.app.engine.GetGameJSON(), &gd)
	return gd.Teams[team].Score
}

// ---------------------------------------------------------------- §6.4 FOUND : un seul chemin d'arrêt + révélation

func TestPictoRound194_Critical_Found_StopsAndRevealsInOneAction(t *testing.T) {
	w := newPictoWire(t)
	word := w.roundStartedWithDelay(t, 120)
	ep := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 1, [2]int{100, 100}, [2]int{200, 200}))
	w.tv.waitAction(t, pictoActStrokes, 1, 3*time.Second)
	// Avant la fin : le mot n'est dans aucun état.
	for _, c := range []*wireClient{w.admin, w.tv, w.bob, w.carol} {
		if v := gameFieldMaybe(c, "PICTO_REVEALED_WORD"); v != nil && v != "" {
			t.Fatalf("%s : mot révélé avant la fin de manche : %v", c.name, v)
		}
	}

	w.send(t, w.admin, pictoActFound, struct{}{})
	waitPhase(t, w, "REVEALED") // arrêt ET révélation dans la même action : jamais de passage visible par STOPPED
	for _, c := range []*wireClient{w.admin, w.anim, w.tv, w.bob, w.carol} {
		waitGameField(t, c, "PICTO_OUTCOME", "FOUND", 3*time.Second)
		waitGameField(t, c, "PICTO_REVEALED_WORD", word, 3*time.Second)
		if gameStr(t, c, "PICTO_REVEALED_CATEGORY") != string(game.CategoryHistory) {
			t.Errorf("%s : PICTO_REVEALED_CATEGORY = %q, attendu la catégorie de la question (§7)", c.name, gameStr(t, c, "PICTO_REVEALED_CATEGORY"))
		}
	}
	for _, f := range w.admin.of(protocol.ActionUpdate) {
		if strings.Contains(f, `"PHASE":"STOPPED"`) && strings.Contains(f, `"PICTO_OUTCOME":"FOUND"`) {
			t.Errorf("FOUND ne doit pas diffuser d'état STOPPED intermédiaire avec l'issue FOUND")
		}
	}
	// Seul le mot de la manche est révélé : jamais les autres mots d'une question à plusieurs mots (§6.5, §10).
	for _, other := range pictoWireWords {
		if other == word {
			continue
		}
		for _, c := range []*wireClient{w.tv, w.bob, w.carol} {
			if c.containsAnywhere(other) {
				t.Errorf("%s : un autre mot de la question (%q) ne doit jamais être montré, même à REVEALED", c.name, other)
			}
		}
	}
	// La feuille reste affichée en REVEALED ; le dessinateur ne peut plus dessiner (WRONG_PHASE).
	time.Sleep(1100 * time.Millisecond)
	w.send(t, w.tv, pictoActSync, struct{}{})
	cv := w.tv.waitAction(t, pictoActCanvas, 1, 3*time.Second)
	if !strings.Contains(cv[len(cv)-1], `"PTS"`) {
		t.Errorf("la feuille reste conservée à REVEALED (§6.5)")
	}
	n := len(w.alice.of(pictoActError))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 2, [2]int{9, 9}))
	if got := w.alice.waitAction(t, pictoActError, n+1, 3*time.Second); errorCode(got[len(got)-1]) != "WRONG_PHASE" {
		t.Errorf("trait après REVEALED : WRONG_PHASE attendu, got %v", got[len(got)-1])
	}
}

func TestPictoRound194_Found_AcceptedByAnimAndInPause(t *testing.T) {
	w := newPictoWire(t)
	w.roundStartedWithDelay(t, 120)
	w.send(t, w.admin, protocol.ActionPause, struct{}{})
	waitPhase(t, w, "PAUSED")
	w.send(t, w.anim, pictoActFound, struct{}{}) // l'animateur aussi, y compris en pause (§6.3)
	waitPhase(t, w, "REVEALED")
	waitGameField(t, w.admin, "PICTO_OUTCOME", "FOUND", 3*time.Second)
}

func TestPictoRound194_Critical_FoundTwice_OnlyOneOutcome(t *testing.T) {
	w := newPictoWire(t)
	w.roundStartedWithDelay(t, 120)
	w.send(t, w.admin, pictoActFound, struct{}{})
	w.send(t, w.anim, pictoActFound, struct{}{}) // admin et animateur en même temps : la première est appliquée, la suivante refusée
	got := w.anim.waitAction(t, pictoActError, 1, 3*time.Second)
	if errorCode(got[0]) != "WRONG_PHASE" {
		t.Errorf("second « Trouvé » : WRONG_PHASE attendu, got %v", got[0])
	}
	w.admin.never(t, pictoActError, 100*time.Millisecond) // la première action est valable
	waitGameField(t, w.admin, "PICTO_OUTCOME", "FOUND", 3*time.Second)
}

func TestPictoRound194_FoundRefusedFromTabletsAndOutsideARound(t *testing.T) {
	w := newPictoWire(t)
	// Hors manche (PREPARE) : refusé.
	n := len(w.admin.of(pictoActError))
	w.send(t, w.admin, pictoActFound, struct{}{})
	if got := w.admin.waitAction(t, pictoActError, n+1, 3*time.Second); errorCode(got[len(got)-1]) != "WRONG_PHASE" {
		t.Errorf("« Trouvé » en PREPARE : WRONG_PHASE attendu, got %v", got[len(got)-1])
	}
	w.roundStartedWithDelay(t, 120)
	// Une tablette (même le dessinateur) ne peut pas déclarer « Trouvé ».
	w.send(t, w.alice, pictoActFound, struct{}{})
	w.send(t, w.bob, pictoActFound, struct{}{})
	time.Sleep(300 * time.Millisecond)
	if ph := gameField(t, w.admin, "PHASE"); ph != "STARTED" {
		t.Errorf("une tablette ne déclare pas « Trouvé » (liste blanche admin/anim) : phase = %v", ph)
	}
}

// ---------------------------------------------------------------- §6.4 TIMEOUT puis « Trouvé » sur le gong (I5) ou REVEAL

func (w *pictoWire) waitTimeout(t *testing.T) {
	t.Helper()
	waitPhase(t, w, "STOPPED")
	waitGameField(t, w.admin, "PICTO_OUTCOME", "TIMEOUT", 3*time.Second)
}

func TestPictoRound194_Timeout_WordStaysSecret_ThenFoundOnTheGong(t *testing.T) {
	w := newPictoWire(t)
	word := w.roundStartedWithDelay(t, 2)
	w.waitTimeout(t)
	// STOPPED après le gong : le mot n'est PAS encore public.
	for _, c := range []*wireClient{w.admin, w.tv, w.bob} {
		if v := gameFieldMaybe(c, "PICTO_REVEALED_WORD"); v != nil && v != "" {
			t.Errorf("%s : le mot ne doit pas être révélé à TIMEOUT/STOPPED, got %v", c.name, v)
		}
	}
	for _, c := range []*wireClient{w.tv, w.bob, w.carol} {
		if c.containsAnywhere(word) {
			t.Errorf("%s a vu le mot à TIMEOUT (§6.4 : pas encore public)", c.name)
		}
	}
	// « Trouvé » sur le gong : TIMEOUT → FOUND (seule substitution d'issue permise), puis révélation.
	w.send(t, w.anim, pictoActFound, struct{}{})
	waitPhase(t, w, "REVEALED")
	waitGameField(t, w.admin, "PICTO_OUTCOME", "FOUND", 3*time.Second)
	waitGameField(t, w.admin, "PICTO_REVEALED_WORD", word, 3*time.Second)
}

func TestPictoRound194_Timeout_RevealWithoutFound_KeepsTimeout(t *testing.T) {
	w := newPictoWire(t)
	word := w.roundStartedWithDelay(t, 2)
	w.waitTimeout(t)
	w.send(t, w.admin, protocol.ActionReveal, struct{}{}) // REVEAL existant : REVEALED, l'issue reste TIMEOUT
	waitPhase(t, w, "REVEALED")
	waitGameField(t, w.admin, "PICTO_OUTCOME", "TIMEOUT", 3*time.Second)
	waitGameField(t, w.tv, "PICTO_REVEALED_WORD", word, 3*time.Second)
	// Plus de « Trouvé » une fois le mot révélé.
	n := len(w.admin.of(pictoActError))
	w.send(t, w.admin, pictoActFound, struct{}{})
	if got := w.admin.waitAction(t, pictoActError, n+1, 3*time.Second); errorCode(got[len(got)-1]) != "WRONG_PHASE" {
		t.Errorf("« Trouvé » après REVEALED : WRONG_PHASE attendu, got %v", got[len(got)-1])
	}
	if gameStr(t, w.admin, "PICTO_OUTCOME") != "TIMEOUT" {
		t.Errorf("une issue posée n'est jamais remplacée (sauf TIMEOUT → FOUND)")
	}
}

// ---------------------------------------------------------------- §6.4 STOPPED (abandon)

func TestPictoRound194_Stopped_FoundRefused_RevealGivesTheWord(t *testing.T) {
	w := newPictoWire(t)
	word := w.roundStartedWithDelay(t, 120)
	w.send(t, w.admin, protocol.ActionStop, struct{}{})
	waitPhase(t, w, "STOPPED")
	waitGameField(t, w.admin, "PICTO_OUTCOME", "STOPPED", 3*time.Second)
	if v := gameFieldMaybe(w.tv, "PICTO_REVEALED_WORD"); v != nil && v != "" {
		t.Errorf("le mot n'est pas public après un abandon tant que REVEAL n'a pas eu lieu")
	}
	// PICTO_FOUND en STOPPED n'est accepté que si l'issue est TIMEOUT.
	n := len(w.admin.of(pictoActError))
	w.send(t, w.admin, pictoActFound, struct{}{})
	if got := w.admin.waitAction(t, pictoActError, n+1, 3*time.Second); errorCode(got[len(got)-1]) != "WRONG_PHASE" {
		t.Errorf("« Trouvé » après STOP (abandon) : WRONG_PHASE attendu, got %v", got[len(got)-1])
	}
	w.send(t, w.admin, protocol.ActionReveal, struct{}{})
	waitPhase(t, w, "REVEALED")
	waitGameField(t, w.admin, "PICTO_REVEALED_WORD", word, 3*time.Second)
	if gameStr(t, w.admin, "PICTO_OUTCOME") != "STOPPED" {
		t.Errorf("l'issue reste STOPPED après REVEAL")
	}
}

// ---------------------------------------------------------------- §6.2/§6.4 NO_WORD (filet de sécurité)

func TestPictoRound194_NoWord_StopsImmediately_RevealGivesAnEmptyWord(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.send(t, w.admin, protocol.ActionForceReady, struct{}{})
	waitPhase(t, w, "READY")
	w.question.PictoWords = nil // question modifiée à la main entre READY et START
	w.send(t, w.admin, protocol.ActionStart, protocol.StartPayload{Delay: 120})
	waitPhase(t, w, "STOPPED")
	waitGameField(t, w.admin, "PICTO_OUTCOME", "NO_WORD", 10*time.Second)
	w.alice.never(t, pictoActWord, 100*time.Millisecond) // aucun mot à remettre
	w.send(t, w.admin, protocol.ActionReveal, struct{}{})
	waitPhase(t, w, "REVEALED")
	if v := gameStr(t, w.admin, "PICTO_REVEALED_WORD"); v != "" {
		t.Errorf("NO_WORD : mot révélé vide attendu, got %q", v)
	}
	if gameStr(t, w.admin, "PICTO_OUTCOME") != "NO_WORD" {
		t.Errorf("l'issue reste NO_WORD")
	}
}

// ---------------------------------------------------------------- §7 crédit manuel par le mécanisme existant

func TestPictoRound194_Credit_ManualThroughTeamPoints_TeamOnly(t *testing.T) {
	w := newPictoWire(t)
	w.roundStartedWithDelay(t, 120)
	w.send(t, w.admin, pictoActFound, struct{}{})
	waitPhase(t, w, "REVEALED")
	before := w.teamScore("TeamA")
	// Aucun crédit automatique à la révélation (§7 : « le crédit est manuel »).
	time.Sleep(300 * time.Millisecond)
	if got := w.teamScore("TeamA"); got != before {
		t.Fatalf("aucun crédit automatique à la révélation : score %v → %v", before, got)
	}
	w.send(t, w.anim, protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "TeamA", Points: 10})
	deadline := time.Now().Add(3 * time.Second)
	for w.teamScore("TeamA") != before+10 && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if got := w.teamScore("TeamA"); got != before+10 {
		t.Errorf("TEAM_POINTS +10 : score d'équipe = %v, attendu %v", got, before+10)
	}
	// Équipe seule : aucun score individuel pour le dessinateur (§7).
	var gd struct {
		Bumpers map[string]struct {
			Score float64 `json:"SCORE"`
		} `json:"bumpers"`
	}
	_ = json.Unmarshal(w.app.engine.GetGameJSON(), &gd)
	if gd.Bumpers[w.aliceID].Score != 0 {
		t.Errorf("le dessinateur ne reçoit pas de score individuel : %v", gd.Bumpers[w.aliceID].Score)
	}
	// Verrou d'attribution (#170) : AWARDED_TEAMS n'est PAS un champ du GameState mais un message séparé
	// (admin + anim, websocket-actions.md §AWARDED_TEAMS) ; il liste les équipes déjà créditées pour la question.
	var awarded []string
	deadline = time.Now().Add(3 * time.Second)
	for len(awarded) == 0 && time.Now().Before(deadline) {
		fr := w.admin.of(protocol.ActionAwardedTeams)
		if len(fr) > 0 {
			var p struct {
				Msg struct {
					Teams []struct {
						Team   string `json:"TEAM"`
						Points int    `json:"POINTS"`
					} `json:"TEAMS"`
				} `json:"MSG"`
			}
			_ = json.Unmarshal([]byte(fr[len(fr)-1]), &p)
			for _, e := range p.Msg.Teams {
				awarded = append(awarded, e.Team)
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !contains(awarded, "TeamA") {
		t.Errorf("le message AWARDED_TEAMS doit contenir TeamA après le crédit : %v", awarded)
	}
}

// ---------------------------------------------------------------- §6.6 relance

func TestPictoRound194_Critical_Relaunch_KeepsPrepClearsOutcomeNewEpochSameWordIfSingle(t *testing.T) {
	w := newPictoWire(t)
	single := &game.Question{
		ID: "picto-single", Question: "Un seul", Type: game.QuestionTypePicto, Category: game.CategoryHistory, Time: "90",
		TypedContent: game.TypedContent{PictoWords: []string{"Unique-secret"}},
	}
	w.question = single
	w.app.engine.Ready(single.ID, single)
	w.chooseTeamAndDrawer(t, w.aliceID)
	if word := w.startRoundWithDelay(t, w.alice, 120); word != "Unique-secret" {
		t.Fatalf("mot = %q", word)
	}
	ep1 := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep1, 1, [2]int{100, 100}))
	w.tv.waitAction(t, pictoActStrokes, 1, 3*time.Second)
	w.send(t, w.admin, pictoActFound, struct{}{})
	waitPhase(t, w, "REVEALED")

	// Relance : l'admin sélectionne de nouveau la même question (mécanisme existant de resélection).
	w.app.engine.Ready(single.ID, single)
	waitPhase(t, w, "PREPARE")
	if gameStr(t, w.admin, "PICTO_TEAM") != "TeamA" || gameStr(t, w.admin, "PICTO_DRAWER") != w.aliceID {
		t.Errorf("relance : l'équipe et le dessinateur sont CONSERVÉS (§6.6) : %q / %q", gameStr(t, w.admin, "PICTO_TEAM"), gameStr(t, w.admin, "PICTO_DRAWER"))
	}
	for _, f := range []string{"PICTO_OUTCOME", "PICTO_REVEALED_WORD", "PICTO_REVEALED_CATEGORY"} {
		if v := gameStr(t, w.admin, f); v != "" {
			t.Errorf("relance : %s doit être vidé, got %q", f, v)
		}
		if v := gameFieldMaybe(w.tv, f); v != nil && v != "" {
			t.Errorf("relance : TV %s doit être vidé, got %v", f, v)
		}
	}
	// Changement de dessinateur permis en préparation (tirage ou choix direct), sans changer d'équipe.
	w.send(t, w.admin, pictoActSetDrawer, map[string]interface{}{"PLAYER": w.bobID})
	waitGameField(t, w.admin, "PICTO_DRAWER", w.bobID, 3*time.Second)
	w.send(t, w.admin, pictoActSetDrawer, map[string]interface{}{"PLAYER": w.aliceID})
	waitGameField(t, w.admin, "PICTO_DRAWER", w.aliceID, 3*time.Second)

	// Nouveau START : nouvelle époque (+1), feuille vierge, la question n'a qu'un mot : toujours le même.
	n := len(w.alice.of(pictoActWord))
	w.send(t, w.admin, protocol.ActionForceReady, struct{}{})
	waitPhase(t, w, "READY")
	w.send(t, w.admin, protocol.ActionStart, protocol.StartPayload{Delay: 120})
	fr := w.alice.waitAction(t, pictoActWord, n+1, 15*time.Second)
	if !strings.Contains(fr[len(fr)-1], "Unique-secret") {
		t.Errorf("question à un mot : toujours le même mot après relance : %s", fr[len(fr)-1])
	}
	ep2 := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	if ep2 != ep1+1 {
		t.Errorf("relance : époque %d → %d, attendu +1", ep1, ep2)
	}
	time.Sleep(1100 * time.Millisecond)
	w.send(t, w.tv, pictoActSync, struct{}{})
	cv := w.tv.waitAction(t, pictoActCanvas, 1, 3*time.Second)
	if strings.Contains(cv[len(cv)-1], `"PTS"`) {
		t.Errorf("la feuille repart vierge après la relance : %s", cv[len(cv)-1])
	}
}

// Au retour en PREPARE le dessinateur n'a plus le mot ; il le reçoit au START (§6.6, dernière puce).
func TestPictoRound194_Relaunch_DrawerGetsTheWordOnlyAtStart(t *testing.T) {
	w := newPictoWire(t)
	w.roundStartedWithDelay(t, 120)
	w.send(t, w.admin, pictoActFound, struct{}{})
	waitPhase(t, w, "REVEALED")
	n := len(w.alice.of(pictoActWord))
	w.app.engine.Ready(w.question.ID, w.question)
	waitPhase(t, w, "PREPARE")
	time.Sleep(300 * time.Millisecond)
	if got := len(w.alice.of(pictoActWord)); got != n {
		t.Errorf("aucun PICTO_WORD en préparation après la relance (§6.1 : le mot n'est pas remis avant START)")
	}
}

// Une question suivante repart de zéro : équipe, dessinateur, candidats et feuille vidés (§5, §6.6).
func TestPictoRound194_NextQuestion_ClearsPreparationAndSheet(t *testing.T) {
	w := newPictoWire(t)
	w.roundStartedWithDelay(t, 120)
	w.send(t, w.admin, pictoActFound, struct{}{})
	waitPhase(t, w, "REVEALED")
	w.app.engine.Ready("q-next", &game.Question{ID: "q-next", Question: "Suivante", Type: game.QuestionTypeSpeedy, Answer: "x", Time: "30", Points: "10"})
	waitPhase(t, w, "PREPARE")
	for _, f := range []string{"PICTO_TEAM", "PICTO_DRAWER", "PICTO_OUTCOME", "PICTO_REVEALED_WORD"} {
		if v := gameStr(t, w.admin, f); v != "" {
			t.Errorf("changement de question : %s doit être vidé, got %q", f, v)
		}
	}
	if c, _ := gameField(t, w.admin, "PICTO_CANDIDATES").([]interface{}); len(c) != 0 {
		t.Errorf("PICTO_CANDIDATES vidé au changement de question : %v", c)
	}
}

// ---------------------------------------------------------------- §6.7 déconnexions

// Dessinateur déconnecté en STARTED : la manche continue, chronomètre inchangé, aucun remplacement.
func TestPictoRound194_DrawerDisconnected_RoundContinues_NoReplacement(t *testing.T) {
	w := newPictoWire(t)
	w.roundStartedWithDelay(t, 120)
	w.alice.conn.Close()
	time.Sleep(500 * time.Millisecond)
	if ph := gameField(t, w.admin, "PHASE"); ph != "STARTED" {
		t.Errorf("la manche continue (phase STARTED), got %v", ph)
	}
	if gameStr(t, w.admin, "PICTO_DRAWER") != w.aliceID {
		t.Errorf("aucun remplacement du dessinateur en cours de manche : PICTO_DRAWER = %q", gameStr(t, w.admin, "PICTO_DRAWER"))
	}
	waitGameField(t, w.admin, "PICTO_DRAWER_CONNECTED", false, 3*time.Second) // l'alerte « déconnecté(e) » de l'admin/anim s'appuie dessus
	// Tirer ou changer le dessinateur est refusé hors préparation.
	n := len(w.admin.of(pictoActError))
	w.send(t, w.admin, pictoActSetDrawer, map[string]interface{}{"PLAYER": w.bobID})
	if got := w.admin.waitAction(t, pictoActError, n+1, 3*time.Second); errorCode(got[len(got)-1]) != "WRONG_PHASE" {
		t.Errorf("changer de dessinateur en manche : WRONG_PHASE attendu, got %v", got[len(got)-1])
	}
	w.send(t, w.admin, pictoActDrawDrawer, struct{}{})
	if got := w.admin.waitAction(t, pictoActError, n+2, 3*time.Second); errorCode(got[len(got)-1]) != "WRONG_PHASE" {
		t.Errorf("tirer un dessinateur en manche : WRONG_PHASE attendu, got %v", got[len(got)-1])
	}
}

// TV, admin ou anim qui se (re)connecte pendant la manche : feuille d'office ; admin et anim reçoivent aussi le mot.
func TestPictoRound194_ReconnectingScreens_GetTheSheet_AdminAnimAlsoTheWord(t *testing.T) {
	w := newPictoWire(t)
	word := w.roundStartedWithDelay(t, 120)
	ep := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 1, [2]int{100, 100}, [2]int{200, 200}))
	w.tv.waitAction(t, pictoActStrokes, 1, 3*time.Second)

	base := startAnimAllowlistTestServer(t, w.app)
	for _, tc := range []struct {
		name, path string
		wantWord   bool
	}{{"tv", "/ws/tv", false}, {"admin", "/ws/admin", true}, {"anim", "/ws/anim", true}} {
		t.Run(tc.name, func(t *testing.T) {
			c := newWireClient(t, tc.name+"-2", dialWS(t, base, tc.path))
			cid := learnClientID(t, w.app, c.conn)
			_ = cid
			w.send(t, c, "HELLO", map[string]interface{}{})
			cv := c.waitAction(t, pictoActCanvas, 1, 5*time.Second)
			if !strings.Contains(cv[0], `"PTS"`) {
				t.Errorf("%s reconnecté : la feuille doit lui être envoyée d'office avec ses traits : %s", tc.name, cv[0])
			}
			if tc.wantWord {
				fr := c.waitAction(t, pictoActWord, 1, 5*time.Second)
				if !strings.Contains(fr[0], word) {
					t.Errorf("%s reconnecté : PICTO_WORD attendu avec le mot de la manche", tc.name)
				}
			} else {
				c.never(t, pictoActWord, 200*time.Millisecond)
				if c.containsAnywhere(word) {
					t.Errorf("la TV reconnectée ne doit jamais recevoir le mot")
				}
			}
		})
	}
}

// Hors manche, un écran qui se connecte ne reçoit ni feuille d'office ni mot.
func TestPictoRound194_ScreenConnectingOutsideARound_GetsNothing(t *testing.T) {
	w := newPictoWire(t)
	base := startAnimAllowlistTestServer(t, w.app)
	c := newWireClient(t, "tv-2", dialWS(t, base, "/ws/tv"))
	learnClientID(t, w.app, c.conn)
	w.send(t, c, "HELLO", map[string]interface{}{})
	time.Sleep(400 * time.Millisecond)
	c.never(t, pictoActWord, 0)
	for _, f := range c.of(pictoActCanvas) {
		if strings.Contains(f, `"PTS"`) {
			t.Errorf("hors manche : aucune feuille d'office avec des traits")
		}
	}
}

// Suppression du bumper du dessinateur : PICTO_DRAWER vidé (§6.7 purge).
func TestPictoRound194_DrawerBumperDeleted_DrawerCleared(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.send(t, w.admin, protocol.ActionDeleteBumper, map[string]interface{}{"ID": w.aliceID})
	waitGameField(t, w.admin, "PICTO_DRAWER", "", 5*time.Second)
	if gameStr(t, w.admin, "PICTO_TEAM") != "TeamA" {
		t.Errorf("seul le dessinateur est purgé, l'équipe est conservée")
	}
	cands, _ := gameField(t, w.admin, "PICTO_CANDIDATES").([]interface{})
	for _, c := range cands {
		if c.(map[string]interface{})["ID"] == w.aliceID {
			t.Errorf("le joueur supprimé ne doit plus figurer dans PICTO_CANDIDATES")
		}
	}
}

// ---------------------------------------------------------------- §15 buzzers ignorés pendant toute la manche

func TestPictoRound194_ButtonsIgnoredInEveryPhaseOfTheRound(t *testing.T) {
	w := newPictoWire(t)
	press := func(label string) {
		w.send(t, w.bob, protocol.ActionButton, map[string]interface{}{"ID": w.bobID})
		w.send(t, w.admin, protocol.ActionButton, map[string]interface{}{"ID": w.bobID}) // simulation admin
		time.Sleep(150 * time.Millisecond)
		_ = label
	}
	phase := func() string { return gameStr(t, w.admin, "PHASE") }
	w.chooseTeamAndDrawer(t, w.aliceID)
	press("prepare")
	if phase() != "PREPARE" {
		t.Errorf("un appui en PREPARE ne change rien : %s", phase())
	}
	w.startRoundWithDelay(t, w.alice, 120)
	press("started")
	if phase() != "STARTED" {
		t.Errorf("un appui en STARTED est ignoré (la devinette est orale) : %s", phase())
	}
	w.send(t, w.admin, protocol.ActionPause, struct{}{})
	waitPhase(t, w, "PAUSED")
	press("paused")
	if phase() != "PAUSED" {
		t.Errorf("un appui en PAUSED est ignoré : %s", phase())
	}
	w.send(t, w.admin, pictoActFound, struct{}{})
	waitPhase(t, w, "REVEALED")
	press("revealed")
	if phase() != "REVEALED" {
		t.Errorf("un appui en REVEALED est ignoré : %s", phase())
	}
	if gameStr(t, w.admin, "PICTO_OUTCOME") != "FOUND" {
		t.Errorf("les appuis ne modifient jamais l'issue")
	}
}

// ---------------------------------------------------------------- §16 non-régression : fin de chronomètre inchangée pour les autres types

func TestPictoRound194_NR_OtherTypesTimeoutUnchanged_NoPictoOutcome(t *testing.T) {
	w := newPictoWire(t)
	w.app.engine.SetPhase(game.PhaseStopped)
	w.app.engine.Ready("q-speedy", &game.Question{ID: "q-speedy", Question: "Q", Answer: "R", Type: game.QuestionTypeSpeedy, Time: "30", Points: "10"})
	waitPhase(t, w, "PREPARE")
	w.send(t, w.admin, protocol.ActionForceReady, struct{}{})
	waitPhase(t, w, "READY")
	w.send(t, w.admin, protocol.ActionStart, protocol.StartPayload{Delay: 2})
	waitGameField(t, w.admin, "PHASE", "STARTED", 15*time.Second)
	waitGameField(t, w.admin, "PHASE", "STOPPED", 10*time.Second) // fin de chronomètre : OnTimeUp puis Stop(), inchangé
	if v := gameStr(t, w.admin, "PICTO_OUTCOME"); v != "" {
		t.Errorf("une question SPEEDY ne pose jamais d'issue PICTO : %q", v)
	}
	if gameStr(t, w.admin, "PICTO_REVEALED_WORD") != "" {
		t.Errorf("une question SPEEDY ne révèle aucun mot PICTO")
	}
}
