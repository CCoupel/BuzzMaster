package main

// #252 (v11.2, lot V2-3) — champ DISPLAY (« Afficher sur la TV » = correction cachée), SCORE_CHANGE, montant libre.
// Contrats : websocket-actions.md §TEAM_POINTS « Affichage public », §BUMPER_POINTS, §TEAM_POINTS_BATCH ; game-state.md
// §SCORE_CHANGE ; logging.md §3.2 quinquies ; §AWARDED_TEAMS (règle « déjà créditée » : DISPLAY ≠ false).
// Scénarios serveur 1 à 7, 12 et 13 du plan (_work/reports/planner-20261011-213000.md). Réutilise env244 (son, ambiance et
// LED instrumentés). Statut `regression` ; `critical` : canaux sans effet et compatibilité.

import (
	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"testing"
)

type scoreChange252 struct {
	Seq     int      `json:"SEQ"`
	Display bool     `json:"DISPLAY"`
	Teams   []string `json:"TEAMS"`
	Players []string `json:"PLAYERS"`
}

// scoreChange lit GAME.SCORE_CHANGE tel que sérialisé (présence, listes non nulles).
func (e *env244) scoreChange(t *testing.T) (sc scoreChange252, raw string) {
	t.Helper()
	var g struct {
		Game map[string]json.RawMessage `json:"GAME"`
	}
	_ = json.Unmarshal(e.app.engine.GetGameJSON(), &g)
	r, ok := g.Game["SCORE_CHANGE"]
	if !ok {
		t.Fatalf("SCORE_CHANGE doit être TOUJOURS sérialisé dans GAME")
	}
	if err := json.Unmarshal(r, &sc); err != nil {
		t.Fatalf("SCORE_CHANGE illisible : %v (%s)", err, r)
	}
	return sc, string(r)
}

func (e *env244) teamPoints(t *testing.T, team string, points int, display *bool) {
	t.Helper()
	p := map[string]interface{}{"TEAM": team, "POINTS": points}
	if display != nil {
		p["DISPLAY"] = *display
	}
	e.send(t, e.admin, "admin", protocol.ActionTeamPoints, p)
}

func (e *env244) bumperPoints(t *testing.T, id string, points int, display *bool) {
	t.Helper()
	p := map[string]interface{}{"ID": id, "POINTS": points}
	if display != nil {
		p["DISPLAY"] = *display
	}
	e.send(t, e.admin, "admin", protocol.ActionBumperPoints, p)
}

func boolPtr252(b bool) *bool { return &b }

// witnessCue252 : son témoin (#271). Le moteur de son est une file FIFO bornée lue par UNE goroutine : quand le
// témoin est joué, tout son notifié avant lui l'a été aussi. Aucun test n'asserte sur ce son.
const witnessCue252 = audio.CueEntracteFin

// cues : les sons joués, hors témoin.
func (e *env244) cues() []audio.Cue {
	var out []audio.Cue
	for _, c := range e.sound.Played() {
		if c != witnessCue252 {
			out = append(out, c)
		}
	}
	return out
}

// settleSound attend que le moteur de son ait traité tout ce qui lui a été notifié (événement : le témoin est joué).
func (e *env244) settleSound(t *testing.T) {
	t.Helper()
	count := func() int {
		n := 0
		for _, c := range e.sound.Played() {
			if c == witnessCue252 {
				n++
			}
		}
		return n
	}
	before := count()
	e.app.sound().PlayCue(witnessCue252)
	eventually264(t, "son témoin joué par le moteur de son", func() bool { return count() > before })
}

// amb252 : compteurs de l'ambiance avant l'action. notif = NotifyState + NotifyPulse comptés SYNCHRONEMENT par le
// writer (déterministe) ; count = Apply du driver factice, asynchrone.
type amb252 struct{ count, notif int }

func (e *env244) ambianceNotifies() int { return e.app.ambiance().Stats().Notifies }

func (e *env244) ambSnapshot() amb252 {
	return amb252{count: e.ambiance.Count(), notif: e.ambianceNotifies()}
}

// ambChanged : l'ambiance a-t-elle été notifiée depuis le snapshot ? Compteur synchrone : « aucune pulsation ni
// flash » se prouve sans attendre, et sans pouvoir être pris en défaut par un Apply d'amorçage tardif.
func (e *env244) ambChanged(before amb252) bool { return e.ambianceNotifies() != before.notif }

// ambPulsed : l'ambiance a été notifiée ET le driver a appliqué (attente par événement, échéance large).
func (e *env244) ambPulsed(t *testing.T, before amb252) bool {
	t.Helper()
	if e.ambianceNotifies() <= before.notif {
		return false
	}
	eventually264(t, "Apply de l'ambiance", func() bool { return e.ambiance.Count() > before.count })
	return true
}

func (e *env244) gagneCount() int {
	n := 0
	for _, c := range e.cues() {
		if c == audio.CueGagne {
			n++
		}
	}
	return n
}

func (e *env244) cometTeams() map[string]bool {
	out := map[string]bool{}
	for team, mac := range e.buzzers {
		if p, ok := e.app.bumperLEDState[mac]; ok && p.Effect == "COMET" {
			out[team] = true
		}
	}
	return out
}

func (e *env244) lastHistoryEvent(team string) (map[string]interface{}, bool) {
	hist := e.app.engine.GetHistory()
	for i := len(hist) - 1; i >= 0; i-- {
		if hist[i].EventType == "POINTS_AWARDED" && hist[i].TeamName == team {
			raw, _ := json.Marshal(hist[i])
			var m map[string]interface{}
			_ = json.Unmarshal(raw, &m)
			return m, true
		}
	}
	return nil, false
}

// --- 1. Compatibilité : sans DISPLAY = comportement actuel (critical) ---------------------------------------

func TestScoreDisplay252_Critical_WithoutDisplayFieldPublicEffectsAreUnchanged(t *testing.T) {
	e := newEnv244(t)
	ambBefore := e.ambSnapshot()
	seqBefore := mustScoreChange252(t, e).Seq

	e.teamPoints(t, "Les Bleus", 3, nil)

	e.settleSound(t) // #271 : témoin FIFO du moteur de son, pas de pause fixe
	if e.gagneCount() != 1 {
		t.Errorf("sans DISPLAY : son « gagné » joué (%v)", e.cues())
	}
	if !e.cometTeams()["Les Bleus"] {
		t.Error("sans DISPLAY : comète LED de l'équipe")
	}
	if !e.ambPulsed(t, ambBefore) {
		t.Error("sans DISPLAY : pulsation d'ambiance (NotifyPulse)")
	}
	sc, _ := e.scoreChange(t)
	if sc.Seq != seqBefore+1 || !sc.Display || len(sc.Teams) != 1 || sc.Teams[0] != "Les Bleus" {
		t.Errorf("SCORE_CHANGE = %+v (SEQ+1, DISPLAY true, TEAMS [Les Bleus])", sc)
	}
	if ev, ok := e.lastHistoryEvent("Les Bleus"); !ok || ev["DISPLAY"] == false {
		t.Errorf("historique : POINTS_AWARDED affiché (DISPLAY absent ou true) : %v", ev)
	}
	if e.countLog(game.LogLevelWarn, "SCORE_CORRECTION_HIDDEN") != 0 {
		t.Error("aucun WARN SCORE_CORRECTION_HIDDEN sans correction cachée")
	}
}

func mustScoreChange252(t *testing.T, e *env244) scoreChange252 {
	sc, _ := e.scoreChange(t)
	return sc
}

func TestScoreDisplay252_DisplayTrueIsTheSameAsAbsent(t *testing.T) {
	e := newEnv244(t)
	e.teamPoints(t, "Les Bleus", 3, boolPtr252(true))
	e.settleSound(t) // #271 : témoin FIFO du moteur de son, pas de pause fixe
	sc, _ := e.scoreChange(t)
	if e.gagneCount() != 1 || !e.cometTeams()["Les Bleus"] || !sc.Display {
		t.Errorf("DISPLAY:true = comportement actuel (son %d, comètes %v, SCORE_CHANGE %+v)", e.gagneCount(), e.cometTeams(), sc)
	}
}

// --- 2. TEAM_POINTS DISPLAY:false : aucun effet sur aucun canal (critical) -------------------------------------

func TestScoreDisplay252_Critical_TeamPointsHiddenCorrectionHasNoPublicEffectOnAnyChannel(t *testing.T) {
	e := newEnv244(t)
	soundBefore, ambBefore := len(e.cues()), e.ambSnapshot()

	e.teamPoints(t, "Les Bleus", 3, boolPtr252(false))

	e.settleSound(t) // #271 : témoin FIFO du moteur de son, pas de pause fixe
	if sc := e.scores(); sc["Les Bleus"][1] != 3 || sc["Les Bleus"][0] != 3 {
		t.Errorf("le score est mis à jour malgré DISPLAY:false : %v", sc["Les Bleus"])
	}
	if got := len(e.cues()); got != soundBefore {
		t.Errorf("aucun son (%d → %d) : %v", soundBefore, got, e.cues())
	}
	if c := e.cometTeams(); len(c) != 0 {
		t.Errorf("aucune comète LED : %v", c)
	}
	if e.ambChanged(ambBefore) {
		t.Errorf("aucune pulsation ni flash d'ambiance (%d → %d)", ambBefore.notif, e.ambianceNotifies())
	}
	sc, _ := e.scoreChange(t)
	if sc.Display || len(sc.Teams) != 1 || sc.Teams[0] != "Les Bleus" || sc.Seq < 1 {
		t.Errorf("SCORE_CHANGE.DISPLAY = false, TEAMS [Les Bleus] : %+v", sc)
	}
	ev, ok := e.lastHistoryEvent("Les Bleus")
	if !ok || ev["DISPLAY"] != false {
		t.Errorf("l'historique enregistre l'entrée avec DISPLAY:false : %v", ev)
	}
	if !e.hasLog(game.LogLevelWarn, "SCORE_CORRECTION_HIDDEN", "TEAM_POINTS") {
		t.Errorf("WARN SCORE_CORRECTION_HIDDEN (client, action, targets, question) ; logs=%v", e.logLines(game.LogLevelWarn))
	}
}

func TestScoreDisplay252_HiddenCorrectionStillUpdatesTheRankingForEveryClient(t *testing.T) {
	e := newEnv244(t)
	e.teamPoints(t, "Les Verts", 5, boolPtr252(false))
	e.teamPoints(t, "Les Bleus", 2, boolPtr252(false))
	sc := e.scores()
	if sc["Les Verts"][0] != 5 || sc["Les Bleus"][0] != 2 {
		t.Errorf("classement à jour sans animation : %v", sc)
	}
}

// --- 3. BUMPER_POINTS -----------------------------------------------------------------------------------------

func TestScoreDisplay252_Critical_BumperPointsHiddenCorrectionHasNoPublicEffect(t *testing.T) {
	e := newEnv244(t)
	mac := e.buzzers["Les Bleus"]
	soundBefore, ambBefore := len(e.cues()), e.ambSnapshot()

	e.bumperPoints(t, mac, 4, boolPtr252(false))

	e.settleSound(t) // #271 : témoin FIFO du moteur de son, pas de pause fixe
	if b := e.app.engine.GetBumper(mac); b == nil || b.Score != 4 {
		t.Errorf("score du joueur mis à jour : %+v", b)
	}
	if len(e.cues()) != soundBefore || len(e.cometTeams()) != 0 || e.ambChanged(ambBefore) {
		t.Errorf("aucun effet public (son %v, comètes %v, ambiance %d→%d)", e.cues(), e.cometTeams(), ambBefore.notif, e.ambianceNotifies())
	}
	sc, _ := e.scoreChange(t)
	if sc.Display || len(sc.Players) != 1 || sc.Players[0] != mac || len(sc.Teams) != 1 || sc.Teams[0] != "Les Bleus" {
		t.Errorf("SCORE_CHANGE : DISPLAY false, PLAYERS [%s], TEAMS [son équipe] : %+v", mac, sc)
	}
	if !e.hasLog(game.LogLevelWarn, "SCORE_CORRECTION_HIDDEN", "BUMPER_POINTS") {
		t.Errorf("WARN SCORE_CORRECTION_HIDDEN ; logs=%v", e.logLines(game.LogLevelWarn))
	}
}

func TestScoreDisplay252_BumperPointsWithoutDisplayKeepsItsEffects(t *testing.T) {
	e := newEnv244(t)
	e.bumperPoints(t, e.buzzers["Les Bleus"], 4, nil)
	e.settleSound(t) // #271 : témoin FIFO du moteur de son, pas de pause fixe
	sc, _ := e.scoreChange(t)
	if e.gagneCount() != 1 || !sc.Display || len(sc.Players) != 1 {
		t.Errorf("sans DISPLAY : comportement actuel (son %d, SCORE_CHANGE %+v)", e.gagneCount(), sc)
	}
}

// --- 4. TEAM_POINTS_BATCH ---------------------------------------------------------------------------------------

func TestScoreDisplay252_Critical_BatchWithDisplayFalseHasNoGroupedEffect(t *testing.T) {
	e := newEnv244(t)
	soundBefore, ambBefore := len(e.cues()), e.ambSnapshot()

	e.batch(t, qid244, threeEntries244(), map[string]interface{}{"DISPLAY": false})

	if r := e.result(t); r.Status != "APPLIED" || len(r.Applied) != 3 {
		t.Fatalf("RESULT = %+v", r)
	}
	e.settleSound(t) // #271 : témoin FIFO du moteur de son, pas de pause fixe
	if len(e.cues()) != soundBefore || len(e.cometTeams()) != 0 || e.ambChanged(ambBefore) {
		t.Errorf("aucun effet groupé (son %v, comètes %v, ambiance %d→%d)", e.cues(), e.cometTeams(), ambBefore.notif, e.ambianceNotifies())
	}
	sc, _ := e.scoreChange(t)
	if sc.Display || len(sc.Teams) != 3 {
		t.Errorf("SCORE_CHANGE.TEAMS = les 3 équipes créditées, DISPLAY false : %+v", sc)
	}
	if e.scores()["Les Verts"][0] != 5 {
		t.Errorf("scores mis à jour : %v", e.scores())
	}
}

func TestScoreDisplay252_BatchWithoutDisplayKeepsGroupedEffects(t *testing.T) {
	e := newEnv244(t)
	e.batch(t, qid244, threeEntries244(), nil)
	_ = e.result(t)
	e.settleSound(t) // #271 : témoin FIFO du moteur de son, pas de pause fixe
	sc, _ := e.scoreChange(t)
	if e.gagneCount() != 1 || len(e.cometTeams()) != 3 || !sc.Display {
		t.Errorf("sans DISPLAY : un son, une comète par équipe (%d, %v, %+v)", e.gagneCount(), e.cometTeams(), sc)
	}
}

// --- 5. Négatifs ------------------------------------------------------------------------------------------------

func TestScoreDisplay252_NegativeAmountsUpdateScoresWithNoEffectWithOrWithoutDisplay(t *testing.T) {
	for name, d := range map[string]*bool{"sans DISPLAY": nil, "DISPLAY true": boolPtr252(true), "DISPLAY false": boolPtr252(false)} {
		t.Run(name, func(t *testing.T) {
			e := newEnv244(t)
			soundBefore, ambBefore := len(e.cues()), e.ambSnapshot()

			e.teamPoints(t, "Les Rouges", -4, d)

			e.settleSound(t) // #271 : témoin FIFO du moteur de son, pas de pause fixe
			if sc := e.scores(); sc["Les Rouges"][0] != -4 {
				t.Errorf("score mis à jour : %v", sc["Les Rouges"])
			}
			if len(e.cues()) != soundBefore || len(e.cometTeams()) != 0 || e.ambChanged(ambBefore) {
				t.Errorf("aucun effet pour un montant négatif")
			}
		})
	}
}

// --- Montant libre : bornes du client, pas de nouvelle validation serveur ---------------------------------------

func TestScoreDisplay252_FreeAmountsAtTheUiBoundsAreApplied(t *testing.T) {
	e := newEnv244(t)
	e.teamPoints(t, "Les Bleus", 999, nil)
	e.teamPoints(t, "Les Rouges", -999, nil)
	e.teamPoints(t, "Les Verts", 7, nil)
	sc := e.scores()
	if sc["Les Bleus"][1] != 999 || sc["Les Rouges"][1] != -999 || sc["Les Verts"][1] != 7 {
		t.Errorf("montants libres appliqués tels quels : %v", sc)
	}
}

// --- 6. POINTS = 0 ignoré -----------------------------------------------------------------------------------------

// Émetteur ADMIN explicite (e.send(..., e.admin, "admin", ...)) : la règle « 0 ignoré » ne vaut QUE pour l'admin (décision CDP) ;
// l'anim garde son geste « 0 pt » de refus (#170), testé plus bas.
func TestScoreDisplay252_Critical_ZeroPointsFromAnAdminAreIgnoredNoHistoryNoUpdateNoScoreChange(t *testing.T) {
	for _, tc := range []struct {
		name string
		do   func(t *testing.T, e *env244)
	}{
		{"TEAM_POINTS", func(t *testing.T, e *env244) { e.teamPoints(t, "Les Bleus", 0, nil) }},
		{"TEAM_POINTS DISPLAY false", func(t *testing.T, e *env244) { e.teamPoints(t, "Les Bleus", 0, boolPtr252(false)) }},
		{"BUMPER_POINTS", func(t *testing.T, e *env244) { e.bumperPoints(t, e.buzzers["Les Bleus"], 0, nil) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv244(t)
			hist, seq := e.historyLen(), mustScoreChange252(t, e).Seq
			collectAfterSentinel264(t, e.app, e.adminConn) // vide ce qui est déjà arrivé (jusqu'au témoin, #266)

			tc.do(t, e)

			if e.historyLen() != hist {
				t.Errorf("aucune entrée d'historique pour 0 point (%d → %d)", hist, e.historyLen())
			}
			if mustScoreChange252(t, e).Seq != seq {
				t.Error("SCORE_CHANGE.SEQ inchangé")
			}
			for _, a := range collectAfterSentinel264(t, e.app, e.adminConn)[0] { // #266 : témoin, pas de fenêtre fixe
				if a == "UPDATE" {
					t.Error("aucun UPDATE pour un montant nul")
				}
			}
			if e.gagneCount() != 0 {
				t.Error("aucun son")
			}
		})
	}
}

// --- 7. SCORE_CHANGE toujours sérialisé -----------------------------------------------------------------------------

func TestScoreChange252_Critical_AlwaysSerializedWithNonNullListsAndResetByNewGame(t *testing.T) {
	e := newEnv244(t)
	sc0, raw0 := e.scoreChange(t)
	if sc0.Seq != 0 || !sc0.Display {
		t.Errorf("valeur initiale {SEQ 0, DISPLAY true} : %+v", sc0)
	}
	if raw0 == "" || containsNull252(raw0) {
		t.Errorf("listes jamais null : %s", raw0)
	}

	e.teamPoints(t, "Les Bleus", 1, boolPtr252(false))
	if sc, _ := e.scoreChange(t); sc.Seq != 1 {
		t.Errorf("SEQ +1 : %+v", sc)
	}
	e.teamPoints(t, "Les Rouges", 1, nil)
	if sc, _ := e.scoreChange(t); sc.Seq != 2 || !sc.Display {
		t.Errorf("la dernière modification fait foi : %+v", sc)
	}

	e.send(t, e.admin, "admin", protocol.ActionNewGame, map[string]interface{}{"PURGE_SCORES": true, "RESET_QUESTIONS": true})
	if sc, raw := e.scoreChange(t); sc.Seq != 0 || !sc.Display || containsNull252(raw) {
		t.Errorf("SCORE_CHANGE remis à zéro au NEW_GAME : %+v", sc)
	}
}

func containsNull252(s string) bool {
	var m map[string]json.RawMessage
	_ = json.Unmarshal([]byte(s), &m)
	for _, k := range []string{"TEAMS", "PLAYERS"} {
		if string(m[k]) == "null" || m[k] == nil {
			return true
		}
	}
	return false
}

// --- 12-13. Une correction cachée ne rend pas l'équipe « déjà créditée » (décision utilisateur) ---------------------

func TestScoreDisplay252_Critical_HiddenCorrectionDoesNotCountAsAlreadyAwarded(t *testing.T) {
	e := newEnv244(t)
	e.teamPoints(t, "Les Bleus", 2, boolPtr252(false)) // correction cachée pendant REVEALED

	if teamsCredited249(e.env246, "Les Bleus") {
		t.Fatalf("une correction cachée ne rend PAS l'équipe « déjà créditée » : %+v", e.app.getAwardedTeamsPayload())
	}
	e.teamPoints(t, "Les Bleus", 3, nil) // attribution normale de la même question : acceptée
	if !teamsCredited249(e.env246, "Les Bleus") {
		t.Error("après le crédit normal l'équipe devient « déjà créditée »")
	}
	if sc := e.scores(); sc["Les Bleus"][1] != 5 {
		t.Errorf("les deux crédits s'additionnent (2 caché + 3 normal) : %v", sc["Les Bleus"])
	}
}

func TestScoreDisplay252_Critical_HiddenCorrectionThenBatchCreditsTheTeamInsteadOfSkippingIt(t *testing.T) {
	e := newEnv244(t)
	e.teamPoints(t, "Les Bleus", 2, boolPtr252(false))

	e.batch(t, qid244, []entry244{{"Les Bleus", 3}, {"Les Rouges", 1}}, nil)

	r := e.result(t)
	if r.Status != "APPLIED" || len(r.Applied) != 2 || len(r.Skipped) != 0 {
		t.Fatalf("la correction cachée n'est pas vue comme créditée par le lot (pas de SKIPPED) : %+v", r)
	}
	if sc := e.scores(); sc["Les Bleus"][1] != 5 {
		t.Errorf("2 (caché) + 3 (lot) : %v", sc["Les Bleus"])
	}
}

func TestScoreDisplay252_NormalCreditStillLocksTheTeamForTheBatch(t *testing.T) {
	e := newEnv244(t)
	e.teamPoints(t, "Les Bleus", 3, nil) // crédit normal (visible)
	e.batch(t, qid244, []entry244{{"Les Bleus", 3}}, nil)
	r := e.result(t)
	if len(r.Skipped) != 1 || r.Skipped[0].Reason != "ALREADY_AWARDED" {
		t.Errorf("non-régression : un crédit visible verrouille l'équipe : %+v", r)
	}
}

func TestScoreDisplay252_HiddenCorrectionIsNotInTheAwardedTeamsBroadcast(t *testing.T) {
	e := newEnv244(t)
	e.teamPoints(t, "Les Bleus", 2, boolPtr252(false))
	got := awardedTeamsOf244(t, e.animConn)
	if _, locked := got["Les Bleus"]; locked {
		t.Errorf("AWARDED_TEAMS diffusé à l'anim ne contient pas la correction cachée : %v", got)
	}
}

// --- Cas inverse : le geste « 0 pt » de l'ANIM (#170) reste enregistré et verrouille « déjà créditée » ---------------

func TestScoreDisplay252_Critical_ZeroPointsFromTheAnimAreRecordedAndLockTheTeam(t *testing.T) {
	e := newEnv244(t)
	_, anim := e.dial(t, "/ws/anim") // client de type ANIM (pas admin)
	hist := e.historyLen()
	seq := mustScoreChange252(t, e).Seq

	e.send(t, anim, "anim", protocol.ActionTeamPoints, map[string]interface{}{"TEAM": "Les Rouges", "POINTS": 0})

	if e.historyLen() != hist+1 {
		t.Errorf("le refus « 0 pt » de l'anim est enregistré dans l'historique (%d → %d)", hist, e.historyLen())
	}
	if n, sum := e.awardedEvents("Les Rouges"); n != 1 || sum != 0 {
		t.Errorf("un POINTS_AWARDED à 0 pour Les Rouges : %d événement(s), somme %d", n, sum)
	}
	if !teamsCredited249(e.env246, "Les Rouges") {
		t.Errorf("l'équipe est « déjà créditée » (0 pt verrouille, #170) : %+v", e.app.getAwardedTeamsPayload())
	}
	if mustScoreChange252(t, e).Seq <= seq {
		t.Error("le crédit à 0 de l'anim est un crédit appliqué (SCORE_CHANGE.SEQ avance)")
	}
	// et le lot de l'admin l'ignore ensuite (verrou)
	e.batch(t, qid244, []entry244{{"Les Rouges", 2}}, nil)
	if r := e.result(t); len(r.Skipped) != 1 || r.Skipped[0].Reason != "ALREADY_AWARDED" {
		t.Errorf("le lot ignore l'équipe verrouillée par le 0 pt de l'anim : %+v", r)
	}
}

// --- DISPLAY:false n'est honoré que pour l'émetteur ADMIN (décision CDP, revue #252 mineur 1) -------------------------
// Envoyé par l'anim, il est ignoré : le crédit est traité comme visible (effets normaux, SCORE_CHANGE.DISPLAY true,
// historique visible, équipe « déjà créditée »), avec un WARN journalisé.

func (e *env244) animSend(t *testing.T, action string, payload map[string]interface{}) {
	t.Helper()
	_, anim := e.dial(t, "/ws/anim")
	e.send(t, anim, "anim", action, payload)
}

// warnHiddenRefused : WARN SCORE_CORRECTION_HIDDEN_REFUSED (contracts/logging.md) avec type=anim et l'action.
func warnHiddenRefused(e *env244, action string) bool {
	return e.hasLog(game.LogLevelWarn, "SCORE_CORRECTION_HIDDEN_REFUSED", `type="anim"`, "action="+action)
}

func TestScoreDisplay252_Critical_DisplayFalseFromTheAnimIsIgnoredAndTreatedAsVisible(t *testing.T) {
	e := newEnv244(t)
	ambBefore := e.ambSnapshot()

	e.animSend(t, protocol.ActionTeamPoints, map[string]interface{}{"TEAM": "Les Bleus", "POINTS": 3, "DISPLAY": false})

	e.settleSound(t) // #271 : témoin FIFO du moteur de son, pas de pause fixe
	if e.gagneCount() != 1 {
		t.Errorf("effets publics NORMAUX malgré DISPLAY:false de l'anim : son « gagné » (%v)", e.cues())
	}
	if !e.cometTeams()["Les Bleus"] {
		t.Error("comète LED jouée")
	}
	if !e.ambPulsed(t, ambBefore) {
		t.Error("pulsation d'ambiance jouée")
	}
	sc, _ := e.scoreChange(t)
	if !sc.Display {
		t.Errorf("SCORE_CHANGE.DISPLAY = true (le DISPLAY:false de l'anim est ignoré) : %+v", sc)
	}
	if ev, ok := e.lastHistoryEvent("Les Bleus"); !ok || ev["DISPLAY"] == false {
		t.Errorf("historique visible (pas DISPLAY:false) : %v", ev)
	}
	if !teamsCredited249(e.env246, "Les Bleus") {
		t.Error("l'équipe est « déjà créditée » comme pour un crédit normal")
	}
	if !warnHiddenRefused(e, `"`+protocol.ActionTeamPoints+`"`) {
		t.Errorf("WARN SCORE_CORRECTION_HIDDEN_REFUSED type=anim action=TEAM_POINTS ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	if e.countLog(game.LogLevelWarn, "SCORE_CORRECTION_HIDDEN ") > 0 {
		t.Error("ce n'est PAS une correction cachée : pas de SCORE_CORRECTION_HIDDEN")
	}
}

func TestScoreDisplay252_DisplayFalseFromTheAnimOnBumperPointsIsIgnoredToo(t *testing.T) {
	e := newEnv244(t)
	mac := e.buzzers["Les Bleus"]

	e.animSend(t, protocol.ActionBumperPoints, map[string]interface{}{"ID": mac, "POINTS": 4, "DISPLAY": false})

	e.settleSound(t) // #271 : témoin FIFO du moteur de son, pas de pause fixe
	sc, _ := e.scoreChange(t)
	if e.gagneCount() != 1 || !sc.Display || len(sc.Players) != 1 {
		t.Errorf("effets normaux et SCORE_CHANGE.DISPLAY true (son %d, %+v)", e.gagneCount(), sc)
	}
	if !warnHiddenRefused(e, `"`+protocol.ActionBumperPoints+`"`) {
		t.Errorf("WARN SCORE_CORRECTION_HIDDEN_REFUSED type=anim action=BUMPER_POINTS ; logs=%v", e.logLines(game.LogLevelWarn))
	}
}

func TestScoreDisplay252_DisplayFalseFromTheAdminIsStillHonoured(t *testing.T) {
	e := newEnv244(t)
	e.teamPoints(t, "Les Bleus", 3, boolPtr252(false))
	sc, _ := e.scoreChange(t)
	if sc.Display || e.gagneCount() != 0 || !e.hasLog(game.LogLevelWarn, "SCORE_CORRECTION_HIDDEN") {
		t.Errorf("non-régression : l'admin garde la correction cachée (DISPLAY %v, son %d)", sc.Display, e.gagneCount())
	}
	if e.countLog(game.LogLevelWarn, "SCORE_CORRECTION_HIDDEN_REFUSED") > 0 {
		t.Error("aucun WARN _REFUSED pour l'admin")
	}
}
