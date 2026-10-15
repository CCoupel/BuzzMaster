package main

// #249 (v11.2) — identité d'équipe = clé ; équipes verrouillées pendant une question.
// Contrats : contracts/models.md §Identité d'une équipe, contracts/game-state.md §TEAMS_LOCKED,
// contracts/websocket-actions.md §UPDATE (renommage/suppression) et §TEAMS_UPDATE_REFUSED,
// contracts/logging.md §3.2 bis. Scénarios 1, 3 à 8 du plan (_work/reports/planner-20261011-001500.md §A.5).
// Reproduction (statut `regression`) : harness246_rename_server_test — le message exact de
// TeamsPage (clé = nouveau nom, objet équipe conservé donc NAME = ancien nom).

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// newEnv249 : deux équipes ("Les Bleus"/"Les Rouges") avec un VJoueur chacun. started=false : aucune question
// sélectionnée (liste d'équipes libre) ; started=true : question ARDOISE en cours (STARTED).
func newEnv249(t *testing.T, started bool) *env246 {
	t.Helper()
	app := newBroadcast127TestApp(t)
	serverSetGlobalLogger249(t, app)
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{
		"Les Bleus":  {Name: "Les Bleus", Color: []int{0, 0, 255}},
		"Les Rouges": {Name: "Les Rouges", Color: []int{255, 0, 0}},
	})
	paul := setupVirtualPlayer(t, app, "Paul", "Les Bleus")
	mamie := setupVirtualPlayer(t, app, "Mamie", "Les Rouges")
	app.engine.SetPhase(game.PhaseStopped)
	if started {
		app.engine.Ready("", &game.Question{ID: "q-ardoise", Type: game.QuestionTypeArdoise})
		app.engine.SetPhase(game.PhaseStarted)
	}
	return &env246{app: app, base: startAnimAllowlistTestServer(t, app), paul: paul, mamie: mamie}
}

// teamsState : clés d'équipe, NAME de chaque équipe, TEAM de chaque bumper, TEAMS_LOCKED, réponses ARDOISE.
type teamsState249 struct {
	Teams   map[string]map[string]interface{}
	Bumpers map[string]map[string]interface{}
	Locked  *bool
	Ardoise map[string]struct {
		Text string `json:"TEXT"`
	}
}

func (e *env246) state249() teamsState249 {
	var raw struct {
		Teams   map[string]map[string]interface{} `json:"teams"`
		Bumpers map[string]map[string]interface{} `json:"bumpers"`
		Game    struct {
			Locked  *bool `json:"TEAMS_LOCKED"`
			Ardoise map[string]struct {
				Text string `json:"TEXT"`
			} `json:"ARDOISE_ANSWERS"`
		} `json:"GAME"`
	}
	_ = json.Unmarshal(e.app.engine.GetGameJSON(), &raw)
	return teamsState249{Teams: raw.Teams, Bumpers: raw.Bumpers, Locked: raw.Game.Locked, Ardoise: raw.Game.Ardoise}
}

func (s teamsState249) keys() []string {
	var out []string
	for k := range s.Teams {
		out = append(out, k)
	}
	return out
}

// updatePayload249 construit le message exact de TeamsPage.handleRenameTeam : la clé change, l'objet équipe est
// conservé avec `nameInObject` (le client corrigé envoie la nouvelle clé ; un ancien client l'ancien nom), les
// joueurs de l'équipe passent à la nouvelle clé.
func (e *env246) renamePayload249(old, newKey, nameInObject string) map[string]interface{} {
	st := e.state249()
	teams := map[string]interface{}{}
	for k, v := range st.Teams {
		if k == old {
			cp := map[string]interface{}{}
			for kk, vv := range v {
				cp[kk] = vv
			}
			cp["NAME"] = nameInObject
			teams[newKey] = cp
			continue
		}
		teams[k] = v
	}
	bumpers := map[string]interface{}{}
	for id, b := range st.Bumpers {
		cp := map[string]interface{}{}
		for kk, vv := range b {
			cp[kk] = vv
		}
		if b["TEAM"] == old {
			cp["TEAM"] = newKey
		}
		bumpers[id] = cp
	}
	return map[string]interface{}{"teams": teams, "bumpers": bumpers}
}

func (e *env246) deletePayload249(team string) map[string]interface{} {
	st := e.state249()
	teams := map[string]interface{}{}
	for k, v := range st.Teams {
		if k != team {
			teams[k] = v
		}
	}
	bumpers := map[string]interface{}{}
	for id, b := range st.Bumpers {
		cp := map[string]interface{}{}
		for kk, vv := range b {
			cp[kk] = vv
		}
		if b["TEAM"] == team {
			cp["TEAM"] = ""
		}
		bumpers[id] = cp
	}
	return map[string]interface{}{"teams": teams, "bumpers": bumpers}
}

// --- Scénario 1 (critical, regression) : renommage hors question ---------------------------

func TestTeamsRename249_Critical_Regression_RenameOutsideAQuestionKeepsNameEqualToKey(t *testing.T) {
	e := newEnv249(t, false)
	_, admin := e.dial(t, "/ws/admin")

	// message de l'ANCIEN client (objet équipe conservé : NAME = ancien nom) — incident de la semaine bleue
	e.send(t, admin, "admin", protocol.ActionUpdate, e.renamePayload249("Les Bleus", "Les Azur", "Les Bleus"))

	st := e.state249()
	if _, ok := st.Teams["Les Bleus"]; ok {
		t.Fatalf("l'ancienne clé doit avoir disparu : %v", st.keys())
	}
	if got := st.Teams["Les Azur"]["NAME"]; got != "Les Azur" {
		t.Fatalf("NAME doit valoir la clé (« Les Azur »), obtenu %v — admin/anim/TV chercheraient les réponses sous l'ancien nom", got)
	}
	if st.Bumpers[e.paul]["TEAM"] != "Les Azur" {
		t.Errorf("le joueur suit la nouvelle clé : %v", st.Bumpers[e.paul]["TEAM"])
	}
}

func TestTeamsRename249_Critical_Regression_ArdoiseAnswersOfTheNextQuestionLandUnderTheNewKey(t *testing.T) {
	e := newEnv249(t, false)
	_, admin := e.dial(t, "/ws/admin")
	_, cid := e.dial(t, "/ws/player")
	e.send(t, admin, "admin", protocol.ActionUpdate, e.renamePayload249("Les Bleus", "Les Azur", "Les Bleus"))
	e.connect(t, cid, "Paul", e.paul)

	e.app.engine.Ready("", &game.Question{ID: "q-ardoise", Type: game.QuestionTypeArdoise})
	e.app.engine.SetPhase(game.PhaseStarted)
	e.typeAs(t, cid, e.paul, "NAPOLEON", nil)

	st := e.state249()
	if st.Ardoise["Les Azur"].Text != "NAPOLEON" {
		t.Fatalf("la réponse est sous la clé de l'équipe renommée : %v", st.Ardoise)
	}
	if got := st.Teams["Les Azur"]["NAME"]; got != "Les Azur" {
		t.Errorf("NAME = %v : l'écran cherche ARDOISE_ANSWERS[NAME]", got)
	}
}

// --- Scénario 3 : normalisation à l'enregistrement ------------------------------------------

func TestTeamsNormalize249_UpdateWithNameDifferentFromKeyIsCorrectedAndLogged(t *testing.T) {
	e := newEnv249(t, false)
	_, admin := e.dial(t, "/ws/admin")
	payload := e.renamePayload249("Les Bleus", "Les Bleus", "Equipe 1") // même clé, NAME erroné
	payload["teams"].(map[string]interface{})["Les Verts"] = map[string]interface{}{"NAME": "Equipe 3", "SCORE": 0, "COLOR": []int{0, 255, 0}}

	e.send(t, admin, "admin", protocol.ActionUpdate, payload)

	st := e.state249()
	if st.Teams["Les Bleus"]["NAME"] != "Les Bleus" || st.Teams["Les Verts"]["NAME"] != "Les Verts" {
		t.Fatalf("NAME doit être réécrit = clé : %v / %v", st.Teams["Les Bleus"]["NAME"], st.Teams["Les Verts"]["NAME"])
	}
	if !e.hasLog(game.LogLevelWarn, "TEAM_NAME_NORMALIZED", "Les Verts", "Equipe 3", "source=update") {
		t.Errorf("WARN TEAM_NAME_NORMALIZED key/old_name/source=update attendu ; logs=%v", e.logLines(game.LogLevelWarn))
	}
}

func TestTeamsNormalize249_ConsistentTeamsProduceNoWarning(t *testing.T) {
	e := newEnv249(t, false)
	_, admin := e.dial(t, "/ws/admin")
	e.send(t, admin, "admin", protocol.ActionUpdate, e.renamePayload249("Les Bleus", "Les Bleus", "Les Bleus"))
	if n := e.countLog(game.LogLevelWarn, "TEAM_NAME_NORMALIZED"); n != 0 {
		t.Errorf("aucun avertissement quand NAME = clé (%d)", n)
	}
}

// --- Scénario 4 (critical) : renommage refusé pendant une question --------------------------

func TestTeamsLock249_Critical_RenameDuringAQuestionIsRefusedAsAWholeAndOnlyTheSenderIsTold(t *testing.T) {
	e := newEnv249(t, true)
	senderConn, sender := e.dial(t, "/ws/admin")
	otherConn, _ := e.dial(t, "/ws/admin")
	_, pc := e.dial(t, "/ws/player")
	e.connect(t, pc, "Paul", e.paul)
	e.typeAs(t, pc, e.paul, "PARIS", nil)
	before := e.state249()

	e.send(t, sender, "admin", protocol.ActionUpdate, e.renamePayload249("Les Bleus", "Les Azur", "Les Azur"))

	after := e.state249()
	if _, ok := after.Teams["Les Azur"]; ok {
		t.Fatalf("rien ne doit être appliqué (équipes) : %v", after.keys())
	}
	if _, ok := after.Teams["Les Bleus"]; !ok {
		t.Fatalf("l'équipe d'origine doit rester : %v", after.keys())
	}
	if after.Bumpers[e.paul]["TEAM"] != "Les Bleus" {
		t.Errorf("rien ne doit être appliqué (joueurs) : TEAM=%v", after.Bumpers[e.paul]["TEAM"])
	}
	if after.Ardoise["Les Bleus"].Text != "PARIS" || len(after.Ardoise) != len(before.Ardoise) {
		t.Errorf("ARDOISE_ANSWERS inchangé : %v", after.Ardoise)
	}

	raw := readActionMatchingWithin(t, senderConn, "TEAMS_UPDATE_REFUSED", 2*time.Second)
	var refused struct {
		Reason string   `json:"REASON"`
		Teams  []string `json:"TEAMS"`
	}
	_ = json.Unmarshal(raw, &refused)
	if refused.Reason != "TEAMS_LOCKED_DURING_QUESTION" || len(refused.Teams) != 1 || refused.Teams[0] != "Les Bleus" {
		t.Errorf("TEAMS_UPDATE_REFUSED = %+v", refused)
	}
	for _, a := range collectAfterSentinel264(t, e.app, otherConn)[0] { // #266 : témoin, pas de fenêtre fixe
		if a == "TEAMS_UPDATE_REFUSED" {
			t.Fatal("TEAMS_UPDATE_REFUSED est ciblé sur l'émetteur seul")
		}
	}
	if !e.hasLog(game.LogLevelWarn, "TEAMS_CHANGE_REFUSED", "Les Bleus") {
		t.Errorf("WARN TEAMS_CHANGE_REFUSED (client, teams, phase, question) attendu ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	// l'état complet est renvoyé à l'émetteur, qui réaffiche les équipes inchangées
	readActionMatchingWithin(t, senderConn, "UPDATE", 2*time.Second)
}

func TestTeamsLock249_RenameIsRefusedInEveryPhaseFromSelectionToReveal(t *testing.T) {
	for _, ph := range []game.GamePhase{game.PhasePrepare, game.PhaseReady, game.PhaseStarted, game.PhasePaused, game.PhaseStopped} {
		t.Run(string(ph), func(t *testing.T) {
			e := newEnv249(t, false)
			_, admin := e.dial(t, "/ws/admin")
			e.app.engine.Ready("", &game.Question{ID: "q1", Type: game.QuestionTypeArdoise}) // sélection = PREPARE + verrou
			e.app.engine.SetPhase(ph)

			e.send(t, admin, "admin", protocol.ActionUpdate, e.renamePayload249("Les Bleus", "Les Azur", "Les Azur"))

			if _, ok := e.state249().Teams["Les Azur"]; ok {
				t.Errorf("phase %s : renommage refusé attendu", ph)
			}
		})
	}
}

func TestTeamsLock249_FullActionIsGuardedLikeUpdate(t *testing.T) {
	e := newEnv249(t, true)
	_, admin := e.dial(t, "/ws/admin")
	e.send(t, admin, "admin", protocol.ActionFull, e.renamePayload249("Les Bleus", "Les Azur", "Les Azur"))
	if _, ok := e.state249().Teams["Les Azur"]; ok {
		t.Error("FULL est refusé comme UPDATE pendant une question")
	}
}

// --- Scénario 5 : suppression refusée ; ajout, couleur, affectation permis -------------------

func TestTeamsLock249_DeleteDuringAQuestionIsRefused(t *testing.T) {
	e := newEnv249(t, true)
	conn, admin := e.dial(t, "/ws/admin")

	e.send(t, admin, "admin", protocol.ActionUpdate, e.deletePayload249("Les Rouges"))

	st := e.state249()
	if _, ok := st.Teams["Les Rouges"]; !ok {
		t.Fatalf("suppression refusée : %v", st.keys())
	}
	if st.Bumpers[e.mamie]["TEAM"] != "Les Rouges" {
		t.Errorf("le joueur reste dans son équipe : %v", st.Bumpers[e.mamie]["TEAM"])
	}
	readActionMatchingWithin(t, conn, "TEAMS_UPDATE_REFUSED", 2*time.Second)
}

func TestTeamsLock249_AddColorAndAssignmentStayAllowedDuringAQuestion(t *testing.T) {
	e := newEnv249(t, true)
	conn, admin := e.dial(t, "/ws/admin")
	st := e.state249()
	teams := map[string]interface{}{}
	for k, v := range st.Teams {
		teams[k] = v
	}
	teams["Les Verts"] = map[string]interface{}{"NAME": "Les Verts", "SCORE": 0, "COLOR": []int{0, 255, 0}} // ajout
	blues := map[string]interface{}{}
	for k, v := range st.Teams["Les Bleus"] {
		blues[k] = v
	}
	blues["COLOR"] = []int{1, 2, 3} // couleur
	teams["Les Bleus"] = blues
	bumpers := map[string]interface{}{}
	for id, b := range st.Bumpers {
		cp := map[string]interface{}{}
		for k, v := range b {
			cp[k] = v
		}
		if id == e.mamie {
			cp["TEAM"] = "Les Verts" // affectation d'un joueur
		}
		bumpers[id] = cp
	}

	e.send(t, admin, "admin", protocol.ActionUpdate, map[string]interface{}{"teams": teams, "bumpers": bumpers})

	after := e.state249()
	if _, ok := after.Teams["Les Verts"]; !ok {
		t.Errorf("ajout d'équipe permis : %v", after.keys())
	}
	if after.Bumpers[e.mamie]["TEAM"] != "Les Verts" {
		t.Errorf("affectation d'un joueur permise : %v", after.Bumpers[e.mamie]["TEAM"])
	}
	if fmtColor(after.Teams["Les Bleus"]["COLOR"]) != "[1 2 3]" {
		t.Errorf("changement de couleur permis : %v", after.Teams["Les Bleus"]["COLOR"])
	}
	for _, a := range collectAfterSentinel264(t, e.app, conn)[0] { // #266 : témoin, pas de fenêtre fixe
		if a == "TEAMS_UPDATE_REFUSED" {
			t.Error("aucun refus pour un message qui ne retire aucune clé")
		}
	}
	if e.countLog(game.LogLevelWarn, "TEAMS_CHANGE_REFUSED") != 0 {
		t.Error("pas de TEAMS_CHANGE_REFUSED")
	}
}

func fmtColor(v interface{}) string {
	b, _ := json.Marshal(v)
	return strings.NewReplacer(",", " ", "[", "[", "]", "]").Replace(string(b))
}

// --- Scénario 6 (critical) : TEAMS_LOCKED ----------------------------------------------------

func TestTeamsLocked249_Critical_LifecycleAndAlwaysSerialized(t *testing.T) {
	e := newEnv249(t, false)
	locked := func() (bool, bool) {
		st := e.state249()
		if st.Locked == nil {
			return false, false
		}
		return *st.Locked, true
	}
	if v, present := locked(); !present || v {
		t.Fatalf("démarrage (STOPPED sans question) : TEAMS_LOCKED présent et false (obtenu %v, présent=%v)", v, present)
	}

	e.app.engine.Ready("", &game.Question{ID: "q1", Type: game.QuestionTypeArdoise})
	for _, step := range []struct {
		name string
		do   func()
	}{
		{"PREPARE (après Ready)", func() {}},
		{"STARTED", func() { e.app.engine.SetPhase(game.PhaseStarted) }},
		{"STOPPED après une question", func() { e.app.engine.Stop() }},
	} {
		step.do()
		if v, present := locked(); !present || !v {
			t.Fatalf("%s : TEAMS_LOCKED doit être true (obtenu %v)", step.name, v)
		}
	}

	e.app.engine.Reveal()
	if e.app.engine.GetPhase() != game.PhaseRevealed {
		t.Fatalf("précondition : REVEALED, phase=%s", e.app.engine.GetPhase())
	}
	if v, present := locked(); !present || v {
		t.Errorf("REVEALED : TEAMS_LOCKED doit être false (obtenu %v, présent=%v)", v, present)
	}

	e.app.engine.Ready("", &game.Question{ID: "q2", Type: game.QuestionTypeArdoise})
	if v, _ := locked(); !v {
		t.Fatal("une nouvelle sélection repose le verrou")
	}
	e.app.engine.InitGame() // NEW_GAME
	if v, present := locked(); !present || v {
		t.Errorf("NEW_GAME/InitGame : TEAMS_LOCKED doit être false (obtenu %v)", v)
	}
}

func TestTeamsLocked249_FieldIsInEveryFullUpdateSentToTheAdminEvenWhenFalse(t *testing.T) {
	e := newEnv249(t, false)
	conn, _ := e.dial(t, "/ws/admin")

	e.app.broadcastUpdate()

	raw := readActionMatchingWithin(t, conn, "UPDATE", 2*time.Second)
	if !strings.Contains(string(raw), `"TEAMS_LOCKED":false`) {
		t.Fatalf("TEAMS_LOCKED doit être sérialisé même à false (pas d'omitempty) : %.300s", raw)
	}
}

// --- Scénarios 7 et 8 : renommage en REVEALED, report, autres équipes inchangées ------------

func TestTeamsRename249_RevealedRenameCarriesAnswersScoreHistoryAndLockOfAlreadyCredited(t *testing.T) {
	e := newEnv249(t, true)
	_, admin := e.dial(t, "/ws/admin")
	_, pc := e.dial(t, "/ws/player")
	e.connect(t, pc, "Paul", e.paul)
	_, mc := e.dial(t, "/ws/player")
	e.connect(t, mc, "Mamie", e.mamie)
	e.typeAs(t, pc, e.paul, "PARIS", nil)
	e.typeAs(t, mc, e.mamie, "LYON", nil)
	e.app.engine.Stop()
	e.app.engine.Reveal()
	e.send(t, admin, "admin", protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Bleus", Points: 2}) // Les Bleus déjà créditée pour cette question
	if !teamsCredited249(e, "Les Bleus") {
		t.Fatal("précondition : Les Bleus créditée dans AWARDED_TEAMS")
	}
	before := e.state249()

	e.send(t, admin, "admin", protocol.ActionUpdate, e.renamePayload249("Les Bleus", "Les Azur", "Les Azur"))

	st := e.state249()
	if _, ok := st.Teams["Les Azur"]; !ok {
		t.Fatalf("en REVEALED le renommage est permis : %v", st.keys())
	}
	if st.Ardoise["Les Azur"].Text != "PARIS" {
		t.Errorf("la réponse ARDOISE reste affichée sous le nouveau nom : %v", st.Ardoise)
	}
	if _, ok := st.Ardoise["Les Bleus"]; ok {
		t.Errorf("plus d'entrée sous l'ancienne clé : %v", st.Ardoise)
	}
	if !teamsCredited249(e, "Les Azur") || teamsCredited249(e, "Les Bleus") {
		t.Errorf("le verrou « déjà créditée » survit sous le nouveau nom (historique reporté) : %+v", e.app.getAwardedTeamsPayload())
	}
	if st.Teams["Les Azur"]["SCORE"] != before.Teams["Les Bleus"]["SCORE"] {
		t.Errorf("le score est conservé : %v → %v", before.Teams["Les Bleus"]["SCORE"], st.Teams["Les Azur"]["SCORE"])
	}
	// scénario 8 : les autres équipes sont inchangées
	if st.Ardoise["Les Rouges"].Text != "LYON" || st.Teams["Les Rouges"]["NAME"] != "Les Rouges" || st.Bumpers[e.mamie]["TEAM"] != "Les Rouges" {
		t.Errorf("« Les Rouges » inchangée : %v %v", st.Ardoise["Les Rouges"], st.Teams["Les Rouges"])
	}
	if !e.hasLog(game.LogLevelInfo, "TEAM_RENAMED", "old=", "Les Bleus", "new=", "Les Azur") {
		t.Errorf("INFO TEAM_RENAMED old/new attendu ; logs=%v", e.logLines(game.LogLevelInfo))
	}
}

func teamsCredited249(e *env246, team string) bool {
	for _, tm := range e.app.getAwardedTeamsPayload().Teams {
		if tm.Team == team {
			return true
		}
	}
	return false
}

func TestTeamsDelete249_RevealedDeleteErasesTheTeamStatesAndLeavesTheOthers(t *testing.T) {
	e := newEnv249(t, true)
	_, admin := e.dial(t, "/ws/admin")
	_, pc := e.dial(t, "/ws/player")
	e.connect(t, pc, "Paul", e.paul)
	_, mc := e.dial(t, "/ws/player")
	e.connect(t, mc, "Mamie", e.mamie)
	e.typeAs(t, pc, e.paul, "PARIS", nil)
	e.typeAs(t, mc, e.mamie, "LYON", nil)
	e.app.engine.Stop()
	e.app.engine.Reveal()

	e.send(t, admin, "admin", protocol.ActionUpdate, e.deletePayload249("Les Rouges"))

	st := e.state249()
	if _, ok := st.Teams["Les Rouges"]; ok {
		t.Fatalf("hors verrou la suppression est appliquée : %v", st.keys())
	}
	if _, ok := st.Ardoise["Les Rouges"]; ok {
		t.Errorf("les états de l'équipe supprimée sont effacés : %v", st.Ardoise)
	}
	if st.Ardoise["Les Bleus"].Text != "PARIS" {
		t.Errorf("les autres équipes sont inchangées : %v", st.Ardoise)
	}
}

// serverSetGlobalLogger249 aligne le logger global sur celui de l'App (les journaux #249 passent par server.LogX).
func serverSetGlobalLogger249(t *testing.T, app *App) {
	t.Helper()
	setGlobalLogger246(t, app)
}

// --- Suppression d'une équipe : historique conservé, TEAM_DELETED, pas d'héritage de « déjà créditée » ---
// (contrat d6e5fc66 : websocket-actions.md §UPDATE, game-state.md §TEAMS_LOCKED)

// revealedWithCredit249 : question ARDOISE jouée et révélée, « Les Bleus » créditée (+2) ; retourne l'admin.
func revealedWithCredit249(t *testing.T, e *env246) string {
	t.Helper()
	_, admin := e.dial(t, "/ws/admin")
	_, pc := e.dial(t, "/ws/player")
	e.connect(t, pc, "Paul", e.paul)
	e.typeAs(t, pc, e.paul, "PARIS", nil)
	e.app.engine.Stop()
	e.app.engine.Reveal()
	e.send(t, admin, "admin", protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Bleus", Points: 2})
	if !teamsCredited249(e, "Les Bleus") {
		t.Fatal("précondition : Les Bleus créditée")
	}
	return admin
}

// recreate249 ajoute (ou recrée) une équipe par un UPDATE (sans retirer de clé).
func (e *env246) recreate249(t *testing.T, admin, team string) {
	t.Helper()
	st := e.state249()
	teams := map[string]interface{}{}
	for k, v := range st.Teams {
		teams[k] = v
	}
	teams[team] = map[string]interface{}{"NAME": team, "SCORE": 0, "COLOR": []int{0, 0, 255}}
	e.send(t, admin, "admin", protocol.ActionUpdate, map[string]interface{}{"teams": teams})
}

func history249(e *env246, eventType, team string) int {
	n := 0
	for _, ev := range e.app.engine.GetHistory() {
		if ev.EventType == eventType && ev.TeamName == team {
			n++
		}
	}
	return n
}

func TestTeamDelete249_Critical_RecreatedTeamNeverInheritsTheAlreadyCreditedLock(t *testing.T) {
	e := newEnv249(t, true)
	admin := revealedWithCredit249(t, e)

	e.send(t, admin, "admin", protocol.ActionUpdate, e.deletePayload249("Les Bleus"))
	if _, ok := e.state249().Teams["Les Bleus"]; ok {
		t.Fatal("précondition : équipe supprimée (REVEALED)")
	}
	e.recreate249(t, admin, "Les Bleus") // recréée sous le même nom

	if teamsCredited249(e, "Les Bleus") {
		t.Fatalf("une équipe recréée sous le même nom ne doit PAS apparaître dans AWARDED_TEAMS : %+v", e.app.getAwardedTeamsPayload())
	}
}

func TestTeamDelete249_HistoryIsKeptAndATeamDeletedEventIsAdded(t *testing.T) {
	e := newEnv249(t, true)
	admin := revealedWithCredit249(t, e)
	awardedBefore := history249(e, "POINTS_AWARDED", "Les Bleus")
	if awardedBefore == 0 {
		t.Fatal("précondition : un POINTS_AWARDED pour Les Bleus dans l'historique")
	}

	e.send(t, admin, "admin", protocol.ActionUpdate, e.deletePayload249("Les Bleus"))

	if got := history249(e, "POINTS_AWARDED", "Les Bleus"); got != awardedBefore {
		t.Errorf("l'historique est conservé (piste d'audit) : POINTS_AWARDED %d → %d", awardedBefore, got)
	}
	if got := history249(e, "TEAM_DELETED", "Les Bleus"); got != 1 {
		t.Errorf("un événement TEAM_DELETED {TEAM_NAME} est ajouté : %d", got)
	}
	for _, ev := range e.app.engine.GetHistory() {
		if ev.EventType == "TEAM_DELETED" && ev.Timestamp == 0 {
			t.Errorf("TEAM_DELETED porte un TIMESTAMP : %+v", ev)
		}
	}
}

func TestTeamDelete249_CreditedAfterRecreationIsCountedButNotTheOlderOne(t *testing.T) {
	e := newEnv249(t, true)
	admin := revealedWithCredit249(t, e)
	e.send(t, admin, "admin", protocol.ActionUpdate, e.deletePayload249("Les Bleus"))
	e.recreate249(t, admin, "Les Bleus")

	e.send(t, admin, "admin", protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Bleus", Points: 1})

	if !teamsCredited249(e, "Les Bleus") {
		t.Fatal("un crédit POSTÉRIEUR au dernier TEAM_DELETED est compté")
	}
	for _, tm := range e.app.getAwardedTeamsPayload().Teams {
		if tm.Team == "Les Bleus" && tm.Points != 1 {
			t.Errorf("seul le nouveau crédit compte (+1), pas l'ancien (+2) : %d", tm.Points)
		}
	}
}

func TestTeamRename249_NoTeamDeletedEventAndTheAlreadyCreditedLockIsKept(t *testing.T) {
	e := newEnv249(t, true)
	admin := revealedWithCredit249(t, e)

	e.send(t, admin, "admin", protocol.ActionUpdate, e.renamePayload249("Les Bleus", "Les Azur", "Les Azur"))

	if n := history249(e, "TEAM_DELETED", "Les Bleus") + history249(e, "TEAM_DELETED", "Les Azur"); n != 0 {
		t.Errorf("un renommage n'est pas une suppression : aucun TEAM_DELETED (%d)", n)
	}
	if !teamsCredited249(e, "Les Azur") {
		t.Errorf("le verrou « déjà créditée » est conservé sous le nouveau nom : %+v", e.app.getAwardedTeamsPayload())
	}
}

func TestTeamDelete249_NeverCreditedTeamHasNoSideEffect(t *testing.T) {
	e := newEnv249(t, true)
	admin := revealedWithCredit249(t, e) // seule « Les Bleus » est créditée
	before := e.app.getAwardedTeamsPayload()

	e.send(t, admin, "admin", protocol.ActionUpdate, e.deletePayload249("Les Rouges")) // jamais créditée
	e.recreate249(t, admin, "Les Rouges")

	if teamsCredited249(e, "Les Rouges") {
		t.Error("une équipe jamais créditée, supprimée puis recréée, n'est pas « déjà créditée »")
	}
	after := e.app.getAwardedTeamsPayload()
	if len(after.Teams) != len(before.Teams) || !teamsCredited249(e, "Les Bleus") {
		t.Errorf("les autres équipes créditées sont inchangées : avant %+v, après %+v", before.Teams, after.Teams)
	}
	if history249(e, "POINTS_AWARDED", "Les Rouges") != 0 {
		t.Error("aucun crédit fantôme pour l'équipe supprimée")
	}
	if _, ok := e.state249().Teams["Les Rouges"]; !ok {
		t.Error("la recréation fonctionne")
	}
}
