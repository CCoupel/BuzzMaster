package main

// #254 (v11.2, lot V2-4) — attribution groupée AU NIVEAU JOUEUR : BUMPER_POINTS_BATCH après le REVEAL.
// Contrats : contracts/websocket-actions.md §BUMPER_POINTS_BATCH (+ _RESULT), §AWARDED_TEAMS (PLAYERS), §LAUNCH_LOCK ;
// contracts/logging.md §3.2 quater. Scénarios serveur 1 à 7 du plan (_work/reports/planner-20261011-113000.md §#254).
// Montage : env244 (3 équipes, 1 VJoueur ou plus + 1 buzzer physique par équipe, question ARDOISE révélée, son/ambiance
// instrumentés). Joueurs utilisés : Paul (Bleus), le buzzer des Bleus, Mamie (Rouges), le buzzer des Verts.
// Statut `regression` ; `critical` : validation globale, double crédit.

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type pentry254 struct {
	ID     string      `json:"ID"`
	Points interface{} `json:"POINTS"`
}

func (e *env244) bbatch(t *testing.T, questionID string, entries []pentry254, extra map[string]interface{}) {
	t.Helper()
	p := map[string]interface{}{"QUESTION_ID": questionID, "ENTRIES": entries}
	for k, v := range extra {
		p[k] = v
	}
	e.send(t, e.admin, "admin", "BUMPER_POINTS_BATCH", p)
}

type bresult254 struct {
	Status     string `json:"STATUS"`
	Reason     string `json:"REASON"`
	QuestionID string `json:"QUESTION_ID"`
	Applied    []struct {
		ID     string `json:"ID"`
		Points int    `json:"POINTS"`
	} `json:"APPLIED"`
	Skipped []struct {
		ID     string `json:"ID"`
		Reason string `json:"REASON"`
	} `json:"SKIPPED"`
	Players []string `json:"PLAYERS"`
}

func (e *env244) bresult(t *testing.T) bresult254 {
	t.Helper()
	raw := readActionMatchingWithin(t, e.adminConn, "BUMPER_POINTS_BATCH_RESULT", 2*time.Second)
	var r bresult254
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("BUMPER_POINTS_BATCH_RESULT illisible : %v (%s)", err, raw)
	}
	return r
}

// playerScores254 : score de chaque joueur (VJoueur ou buzzer) de l'état de jeu.
func (e *env244) playerScores() map[string]int {
	out := map[string]int{}
	for id, b := range e.state249().Bumpers {
		if s, ok := b["SCORE"].(float64); ok {
			out[id] = int(s)
		}
	}
	return out
}

func (e *env244) playerEvents(id string) (n, sum int) {
	for _, ev := range e.app.engine.GetHistory() {
		if ev.EventType == "POINTS_AWARDED" && ev.WinnerType == "PLAYER" && ev.WinnerID == id {
			n++
			sum += ev.Points
		}
	}
	return
}

// awardedPlayers254 lit AWARDED_TEAMS.PLAYERS sur conn ; present=false si la clé est absente ou nulle.
func awardedPlayers254(t *testing.T, conn *websocket.Conn) (players map[string]int, present bool) {
	t.Helper()
	raw := readActionMatchingWithin(t, conn, "AWARDED_TEAMS", 2*time.Second)
	var p struct {
		Players *[]struct {
			ID     string `json:"ID"`
			Team   string `json:"TEAM"`
			Points int    `json:"POINTS"`
		} `json:"PLAYERS"`
	}
	_ = json.Unmarshal(raw, &p)
	players = map[string]int{}
	if p.Players == nil {
		return players, false
	}
	for _, x := range *p.Players {
		players[x.ID] = x.Points
	}
	return players, true
}

func (e *env244) threePlayers() (paul, buzzerBleus, mamie string) {
	return e.paul, e.buzzers["Les Bleus"], e.mamie
}

// --- 1. Lot valide ---------------------------------------------------------------------------------

func TestBumperPointsBatch254_Critical_ValidBatchCreditsEveryPlayerAndAnswersTheSenderOnly(t *testing.T) {
	e := newEnv244(t)
	paul, buzz, mamie := e.threePlayers()
	before := e.playerScores()

	e.bbatch(t, qid244, []pentry254{{paul, 3}, {buzz, 2}, {mamie, 1}}, nil)

	r := e.bresult(t)
	if r.Status != "APPLIED" || len(r.Applied) != 3 || len(r.Skipped) != 0 || r.QuestionID != qid244 {
		t.Fatalf("RESULT = %+v", r)
	}
	after := e.playerScores()
	for id, want := range map[string]int{paul: 3, buzz: 2, mamie: 1} {
		if after[id]-before[id] != want {
			t.Errorf("score du joueur %s : %d → %d, attendu +%d", id, before[id], after[id], want)
		}
		if n, sum := e.playerEvents(id); n != 1 || sum != want {
			t.Errorf("joueur %s : %d événement(s) POINTS_AWARDED WINNER_TYPE=PLAYER (somme %d), attendu 1 × %d", id, n, sum, want)
		}
	}
	// le score de l'équipe est recalculé (agrégat de ses joueurs) ; les points d'équipe, eux, ne bougent pas
	sc := e.scores()
	if sc["Les Bleus"][0] != 5 || sc["Les Rouges"][0] != 1 || sc["Les Bleus"][1] != 0 {
		t.Errorf("scores d'équipe recalculés (Bleus 3+2, Rouges 1, points d'équipe inchangés) : %v", sc)
	}
	// l'historique porte l'équipe du joueur
	for id, team := range map[string]string{paul: "Les Bleus", buzz: "Les Bleus", mamie: "Les Rouges"} {
		found := false
		for _, ev := range e.app.engine.GetHistory() {
			if ev.EventType == "POINTS_AWARDED" && ev.WinnerID == id && ev.TeamName == team {
				found = true
			}
		}
		if !found {
			t.Errorf("POINTS_AWARDED du joueur %s avec TEAM_NAME=%s", id, team)
		}
	}
	for name, conn := range map[string]*websocket.Conn{"admin": e.adminConn, "anim": e.animConn} {
		got, present := awardedPlayers254(t, conn)
		if !present || len(got) != 3 || got[paul] != 3 || got[mamie] != 1 {
			t.Errorf("AWARDED_TEAMS.PLAYERS reçu par %s : %v (présent=%v)", name, got, present)
		}
	}
	chg, _ := e.scoreChange(t)
	if len(chg.Players) != 3 || len(chg.Teams) < 2 || !chg.Display {
		t.Errorf("SCORE_CHANGE : PLAYERS (3) et TEAMS concernés, DISPLAY true : %+v", chg)
	}
	for _, a := range collectActions(e.otherConn, 300*time.Millisecond) {
		if a == "BUMPER_POINTS_BATCH_RESULT" {
			t.Fatal("RESULT est ciblé sur l'émetteur seul")
		}
	}
	if !e.hasLog(game.LogLevelInfo, "BUMPER_POINTS_BATCH") {
		t.Errorf("INFO BUMPER_POINTS_BATCH ; logs=%v", e.logLines(""))
	}
}

// --- 2. Refus global : état strictement inchangé ----------------------------------------------------------

func many254(e *env244, n int) []pentry254 {
	out := make([]pentry254, n)
	for i := range out {
		out[i] = pentry254{ID: "AA:FF:00:00:00:" + string(rune('A'+i/26)) + string(rune('A'+i%26)), Points: 1}
	}
	return out
}

func TestBumperPointsBatch254_Critical_AnyValidationErrorRefusesTheWholeBatchAndChangesNothing(t *testing.T) {
	e0 := newEnv244(t)
	paul, buzz, mamie := e0.threePlayers()
	valid := func() []pentry254 { return []pentry254{{paul, 3}, {buzz, 2}, {mamie, 1}} }
	cases := []struct {
		name    string
		prep    func(e *env244)
		qid     string
		entries func() []pentry254
		reason  string
	}{
		{"hors REVEALED", func(e *env244) { e.app.engine.SetPhase(game.PhaseStarted) }, qid244, valid, "NOT_REVEALED"},
		{"autre question", nil, "autre", valid, "QUESTION_MISMATCH"},
		{"vide", nil, qid244, func() []pentry254 { return nil }, "INVALID_ENTRIES"},
		{"doublon", nil, qid244, func() []pentry254 { return []pentry254{{paul, 1}, {paul, 2}} }, "INVALID_ENTRIES"},
		{"65 entrées", nil, qid244, func() []pentry254 { return many254(nil, 65) }, "INVALID_ENTRIES"},
		{"montant nul", nil, qid244, func() []pentry254 { return []pentry254{{paul, 3}, {buzz, 0}} }, "INVALID_POINTS"},
		{"montant décimal", nil, qid244, func() []pentry254 { return []pentry254{{paul, 1.5}} }, "INVALID_POINTS"},
		{"montant 1000", nil, qid244, func() []pentry254 { return []pentry254{{paul, 1000}} }, "INVALID_POINTS"},
		{"joueur inconnu", nil, qid244, func() []pentry254 { return []pentry254{{paul, 3}, {"ZZ:ZZ", 2}} }, "UNKNOWN_PLAYER"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv244(t)
			if c.prep != nil {
				c.prep(e)
			}
			beforeScores, beforePlayers, beforeHist := e.scores(), e.playerScores(), e.historyLen()

			e.bbatch(t, c.qid, c.entries(), nil)

			r := e.bresult(t)
			if r.Status != "REFUSED" || r.Reason != c.reason {
				t.Fatalf("RESULT = %+v, attendu REFUSED/%s", r, c.reason)
			}
			if c.reason == "UNKNOWN_PLAYER" && (len(r.Players) != 1 || r.Players[0] != "ZZ:ZZ") {
				t.Errorf("PLAYERS = les identifiants inconnus : %v", r.Players)
			}
			for k, v := range beforeScores {
				if e.scores()[k] != v {
					t.Errorf("score de l'équipe %s modifié : %v → %v", k, v, e.scores()[k])
				}
			}
			for k, v := range beforePlayers {
				if e.playerScores()[k] != v {
					t.Errorf("score du joueur %s modifié : %d → %d", k, v, e.playerScores()[k])
				}
			}
			if e.historyLen() != beforeHist {
				t.Error("aucune entrée d'historique")
			}
			if !e.hasLog(game.LogLevelWarn, "BUMPER_POINTS_BATCH_REFUSED") {
				t.Errorf("WARN BUMPER_POINTS_BATCH_REFUSED ; logs=%v", e.logLines(game.LogLevelWarn))
			}
		})
	}
}

func TestBumperPointsBatch254_Critical_NonAdminClientCannotSendABatch(t *testing.T) {
	e := newEnv244(t)
	_, anim := e.dial(t, "/ws/anim")
	_, tablet := e.dial(t, "/ws/player")
	_, tv := e.dial(t, "/ws/tv")
	paul, _, _ := e.threePlayers()
	before := e.playerScores()

	for id, ct := range map[string]string{anim: "anim", tablet: "vplayer", tv: "tv"} {
		e.send(t, id, ct, "BUMPER_POINTS_BATCH", map[string]interface{}{"QUESTION_ID": qid244, "ENTRIES": []pentry254{{paul, 3}}})
	}

	if got := e.playerScores(); got[paul] != before[paul] {
		t.Errorf("seul l'admin peut envoyer un lot (allow-list) : %d → %d", before[paul], got[paul])
	}
	if !e.hasLog(game.LogLevelWarn, "Rejected action BUMPER_POINTS_BATCH") {
		t.Errorf("le rejet doit être journalisé ; logs=%v", e.logLines(game.LogLevelWarn))
	}
}

// --- 3. Joueur déjà crédité → ignoré ---------------------------------------------------------------

func TestBumperPointsBatch254_Critical_AlreadyAwardedPlayerIsSkippedAndTheOthersAreCredited(t *testing.T) {
	e := newEnv244(t)
	paul, buzz, _ := e.threePlayers()
	e.send(t, e.admin, "admin", protocol.ActionBumperPoints, map[string]interface{}{"ID": paul, "POINTS": 1}) // clic unitaire : verrouille Paul
	before := e.playerScores()

	e.bbatch(t, qid244, []pentry254{{paul, 5}, {buzz, 2}}, nil)

	r := e.bresult(t)
	if r.Status != "APPLIED" || len(r.Applied) != 1 || r.Applied[0].ID != buzz || len(r.Skipped) != 1 || r.Skipped[0].ID != paul || r.Skipped[0].Reason != "ALREADY_AWARDED" {
		t.Fatalf("RESULT = %+v", r)
	}
	after := e.playerScores()
	if after[paul] != before[paul] || after[buzz]-before[buzz] != 2 {
		t.Errorf("Paul inchangé, le buzzer +2 : %v → %v", before, after)
	}
}

func TestBumperPointsBatch254_Critical_SecondIdenticalBatchIsEntirelySkippedNoDoubleCredit(t *testing.T) {
	e := newEnv244(t)
	paul, buzz, mamie := e.threePlayers()
	entries := []pentry254{{paul, 3}, {buzz, 2}, {mamie, 1}}
	e.bbatch(t, qid244, entries, nil)
	_ = e.bresult(t)
	scores, hist := e.playerScores(), e.historyLen()

	e.bbatch(t, qid244, entries, nil)

	r := e.bresult(t)
	if r.Status != "APPLIED" || len(r.Applied) != 0 || len(r.Skipped) != 3 {
		t.Fatalf("second lot entièrement ignoré : %+v", r)
	}
	for _, s := range r.Skipped {
		if s.Reason != "ALREADY_AWARDED" {
			t.Errorf("motif %q", s.Reason)
		}
	}
	for id, v := range scores {
		if e.playerScores()[id] != v {
			t.Errorf("double crédit du joueur %s : %d → %d", id, v, e.playerScores()[id])
		}
	}
	if e.historyLen() != hist {
		t.Error("aucune nouvelle entrée d'historique")
	}
}

// --- 4. Correction cachée / geste 0 pt de l'anim ----------------------------------------------------

func TestBumperPointsBatch254_HiddenCorrectionDoesNotLockThePlayerButThe0ptAnimGestureDoes(t *testing.T) {
	e := newEnv244(t)
	paul, buzz, _ := e.threePlayers()
	e.send(t, e.admin, "admin", protocol.ActionBumperPoints, map[string]interface{}{"ID": paul, "POINTS": 2, "DISPLAY": false})
	_, anim := e.dial(t, "/ws/anim")
	e.send(t, anim, "anim", protocol.ActionBumperPoints, map[string]interface{}{"ID": buzz, "POINTS": 0})
	time.Sleep(100 * time.Millisecond)

	e.bbatch(t, qid244, []pentry254{{paul, 4}, {buzz, 4}}, nil)

	r := e.bresult(t)
	if len(r.Applied) != 1 || r.Applied[0].ID != paul || len(r.Skipped) != 1 || r.Skipped[0].ID != buzz {
		t.Fatalf("Paul (correction cachée) est crédité par le lot ; le buzzer (0 pt de l'anim = verrou valide) est ignoré : %+v", r)
	}
}

// --- 5. Effets publics groupés ----------------------------------------------------------------------

func TestBumperPointsBatch254_PublicEffectsAreGroupedOneSoundOneCometPerConcernedTeam(t *testing.T) {
	e := newEnv244(t)
	paul, buzz, mamie := e.threePlayers()
	vert := e.buzzers["Les Verts"]

	e.bbatch(t, qid244, []pentry254{{paul, 3}, {buzz, 2}, {mamie, -1}, {vert, 5}}, nil)

	_ = e.bresult(t)
	time.Sleep(400 * time.Millisecond)
	if e.gagneCount() != 1 {
		t.Errorf("un SEUL son « gagné » pour tout le lot, obtenu %d : %v", e.gagneCount(), e.sound.Played())
	}
	comets := e.cometTeams()
	if !comets["Les Bleus"] || !comets["Les Verts"] || comets["Les Rouges"] {
		t.Errorf("une comète par équipe des joueurs crédités positivement (Bleus, Verts), pas pour la négative (Rouges) : %v", comets)
	}
}

func TestBumperPointsBatch254_AllNegativeOrDisplayFalseProducesNoPublicEffect(t *testing.T) {
	for name, tc := range map[string]struct {
		points  int
		display map[string]interface{}
	}{"tout négatif": {-2, nil}, "DISPLAY false admin": {3, map[string]interface{}{"DISPLAY": false}}} {
		t.Run(name, func(t *testing.T) {
			e := newEnv244(t)
			paul, buzz, _ := e.threePlayers()
			e.bbatch(t, qid244, []pentry254{{paul, tc.points}, {buzz, tc.points}}, tc.display)
			_ = e.bresult(t)
			time.Sleep(400 * time.Millisecond)
			if e.gagneCount() != 0 || len(e.cometTeams()) != 0 {
				t.Errorf("aucun effet public (son %d, comètes %v)", e.gagneCount(), e.cometTeams())
			}
		})
	}
}

func TestBumperPointsBatch254_DisplayFalseCreditsAreNotInPlayersNorMarkedAlreadyAwarded(t *testing.T) {
	e := newEnv244(t)
	paul, _, _ := e.threePlayers()
	e.bbatch(t, qid244, []pentry254{{paul, 3}}, map[string]interface{}{"DISPLAY": false})
	_ = e.bresult(t)

	got, present := awardedPlayers254(t, e.adminConn)
	if !present || len(got) != 0 {
		t.Errorf("une correction cachée ne compte pas comme déjà créditée : PLAYERS=%v", got)
	}
	if sc, _ := e.scoreChange(t); sc.Display {
		t.Error("SCORE_CHANGE.DISPLAY = false")
	}
}

// --- 6. AWARDED_TEAMS.PLAYERS toujours présent, remis à zéro ----------------------------------------------

func TestAwardedPlayers254_AlwaysPresentEmptyAtStartAndResetOnNewQuestionAndNewGame(t *testing.T) {
	e := newEnv244(t)
	paul, _, _ := e.threePlayers()

	e.send(t, e.admin, "admin", protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Verts", Points: 1})
	if got, present := awardedPlayers254(t, e.adminConn); !present || len(got) != 0 {
		t.Fatalf("PLAYERS toujours présent, [] sans crédit joueur (jamais null) : %v présent=%v", got, present)
	}

	e.bbatch(t, qid244, []pentry254{{paul, 3}}, nil)
	_ = e.bresult(t)
	if got, _ := awardedPlayers254(t, e.adminConn); got[paul] != 3 {
		t.Fatalf("précondition : PLAYERS contient Paul : %v", got)
	}

	e.send(t, e.admin, "admin", protocol.ActionReady, protocol.ReadyPayload{Question: ""}) // changement de question
	if got, present := awardedPlayers254(t, e.adminConn); !present || len(got) != 0 {
		t.Errorf("PLAYERS remis à zéro au changement de question : %v", got)
	}

	e.app.engine.SetPhase(game.PhaseStarted)
	e.app.engine.Stop()
	e.app.engine.Reveal()
	e.bbatch(t, e.currentQuestionID(), []pentry254{{paul, 2}}, nil)
	_ = e.bresult(t)
	e.send(t, e.admin, "admin", protocol.ActionNewGame, map[string]interface{}{})
	if got, present := awardedPlayers254(t, e.adminConn); !present || len(got) != 0 {
		t.Errorf("PLAYERS remis à zéro à NEW_GAME : %v", got)
	}
}

func (e *env244) currentQuestionID() string {
	var raw struct {
		Game struct {
			Question *struct {
				ID string `json:"ID"`
			} `json:"QUESTION"`
		} `json:"GAME"`
	}
	_ = json.Unmarshal(e.app.engine.GetGameJSON(), &raw)
	if raw.Game.Question == nil {
		return ""
	}
	return raw.Game.Question.ID
}

// --- 7. Verrou « Lancer la partie » (#253) --------------------------------------------------------------------

func TestBumperPointsBatch254_IsSuspendedWhileTheLaunchPopupIsOpen(t *testing.T) {
	e := newEnv244(t)
	paul, _, _ := e.threePlayers()
	e.send(t, e.admin, "admin", "LAUNCH_LOCK", map[string]interface{}{"STATE": "OPEN"})
	conn, other := e.dial(t, "/ws/admin")
	before := e.playerScores()

	e.send(t, other, "admin", "BUMPER_POINTS_BATCH", map[string]interface{}{"QUESTION_ID": qid244, "ENTRIES": []pentry254{{paul, 3}}})

	raw := readActionMatchingWithin(t, conn, "ACTION_REFUSED", 2*time.Second)
	var p struct {
		Action string `json:"ACTION"`
		Reason string `json:"REASON"`
	}
	_ = json.Unmarshal(raw, &p)
	if p.Action != "BUMPER_POINTS_BATCH" || p.Reason != "LAUNCH_IN_PROGRESS" {
		t.Errorf("ACTION_REFUSED = %+v", p)
	}
	if e.playerScores()[paul] != before[paul] {
		t.Error("rien n'est appliqué sous verrou")
	}
}
