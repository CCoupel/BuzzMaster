package main

// #243 (v11.2, lot V2-1) — NEW_GAME avec options de purge (« Lancer la partie »).
// Contrats : contracts/websocket-actions.md §NEW_GAME et §NEW_GAME_REFUSED, contracts/logging.md §3.2 ter.
// Décisions : D1 (option RESET_QUESTIONS, cochée par défaut dans la popup), D2 (PURGE_PLAYERS exige PURGE_SCORES :
// sinon refus, rien d'appliqué). Sans aucune option = comportement v11.1.
// Scénarios du plan (_work/reports/planner-20261011-121500.md §4, mis à jour) : 1, 2, 3, 3 bis, 4, 5, 6, 6 bis, 7, 8, 9, 10.
// Statut `regression` ; `critical` : compatibilité (1), défaut (2), refus (3), minuteurs (7).

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type env243 struct {
	*env246
	admin     string
	adminConn *websocket.Conn
	paulConn  *websocket.Conn
	mamieConn *websocket.Conn
	buzzer    string
	paulCID   string
}

// newEnv243 : partie ARDOISE jouée et révélée — 2 équipes, 2 tablettes liées (Paul/Bleus, Mamie/Rouges), 1 buzzer
// physique (Bleus) ; scores non nuls (équipe +2 créditée, buzzer +3) ; la question q-ardoise n'est plus « à jouer ».
func newEnv243(t *testing.T) *env243 {
	t.Helper()
	e := newEnv249(t, true)
	conn, admin := e.dial(t, "/ws/admin")
	paulConn, pc := e.dial(t, "/ws/player")
	e.connect(t, pc, "Paul", e.paul)
	mamieConn, mc := e.dial(t, "/ws/player")
	e.connect(t, mc, "Mamie", e.mamie)

	// buzzer physique rattaché aux Bleus, par l'UPDATE public
	st := e.state249()
	bumpers := map[string]interface{}{}
	for id, b := range st.Bumpers {
		bumpers[id] = b
	}
	const buzzer = "AA:BB:CC:DD:EE:01"
	bumpers[buzzer] = map[string]interface{}{"NAME": "Buzzer1", "TEAM": "Les Bleus", "SCORE": 0, "CONNECTED": true}
	e.send(t, admin, "admin", protocol.ActionUpdate, map[string]interface{}{"teams": teamsMapOf243(st), "bumpers": bumpers})

	e.app.engine.Stop()
	e.app.engine.Reveal()
	e.send(t, admin, "admin", protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Bleus", Points: 2})
	e.send(t, admin, "admin", protocol.ActionPoints, protocol.PointsPayload{BumperID: buzzer, Points: 3})
	return &env243{env246: e, admin: admin, adminConn: conn, paulConn: paulConn, mamieConn: mamieConn, buzzer: buzzer, paulCID: pc}
}

func teamsMapOf243(st teamsState249) map[string]interface{} {
	m := map[string]interface{}{}
	for k, v := range st.Teams {
		m[k] = v
	}
	return m
}

// snapshot243 : ce que NEW_GAME est susceptible de modifier.
type snap243 struct {
	Phase      string
	QuestionID string
	TeamKeys   int
	Bumpers    map[string]int // id → score
	TeamOf     map[string]string
	Scores     map[string]int // équipe → score total
	History    int
	Status     game.QuestionStatus
	Locked     bool
}

func (e *env243) snap() snap243 {
	var raw struct {
		Teams map[string]struct {
			Score int `json:"SCORE"`
		} `json:"teams"`
		Bumpers map[string]struct {
			Score int    `json:"SCORE"`
			Team  string `json:"TEAM"`
		} `json:"bumpers"`
		Game struct {
			Phase    string `json:"PHASE"`
			Question *struct {
				ID string `json:"ID"`
			} `json:"QUESTION"`
			Locked bool `json:"TEAMS_LOCKED"`
		} `json:"GAME"`
	}
	_ = json.Unmarshal(e.app.engine.GetGameJSON(), &raw)
	s := snap243{Phase: raw.Game.Phase, Locked: raw.Game.Locked, Bumpers: map[string]int{}, TeamOf: map[string]string{}, Scores: map[string]int{}}
	if raw.Game.Question != nil {
		s.QuestionID = raw.Game.Question.ID
	}
	s.TeamKeys = len(raw.Teams)
	for id, b := range raw.Bumpers {
		s.Bumpers[id], s.TeamOf[id] = b.Score, b.Team
	}
	for k, v := range raw.Teams {
		s.Scores[k] = v.Score
	}
	s.History = len(e.app.engine.GetHistory())
	s.Status = e.app.engine.GetQuestionStatus("q-ardoise")
	return s
}

func (e *env243) newGame(t *testing.T, payload interface{}) {
	t.Helper()
	e.send(t, e.admin, "admin", protocol.ActionNewGame, payload)
}

// evictedEach243 dit, pour chaque connexion, si elle a reçu PLAYER_EVICTED avant le témoin (#266 :
// plus de fenêtre fixe). Un seul témoin pour toutes les connexions lues à cet instant.
func evictedEach243(t *testing.T, app *App, conns ...*websocket.Conn) []bool {
	t.Helper()
	out := make([]bool, len(conns))
	for i, actions := range collectAfterSentinel264(t, app, conns...) {
		out[i] = containsAction(actions, protocol.ActionPlayerEvicted)
	}
	return out
}

func evicted243(t *testing.T, app *App, conn *websocket.Conn) bool {
	t.Helper()
	return evictedEach243(t, app, conn)[0]
}

func allZero243(m map[string]int) bool {
	for _, v := range m {
		if v != 0 {
			return false
		}
	}
	return true
}

// --- 1. Rétrocompatibilité (critical) ------------------------------------------------------

func TestNewGame243_Critical_NoOptionsKeepsTheV111Behaviour(t *testing.T) {
	for name, payload := range map[string]interface{}{"payload vide": map[string]interface{}{}, "payload nil": nil} {
		t.Run(name, func(t *testing.T) {
			e := newEnv243(t)
			before := e.snap()
			if before.History == 0 || allZero243(before.Bumpers) {
				t.Fatalf("précondition : scores et historique non vides (%+v)", before)
			}

			e.newGame(t, payload)

			s := e.snap()
			if s.Phase != "NEW_GAME" || s.QuestionID != "" {
				t.Errorf("phase NEW_GAME sans question courante : %+v", s)
			}
			if _, ok := s.Bumpers[e.paul]; ok {
				t.Error("v11.1 : les VJoueurs sont supprimés")
			}
			if _, ok := s.Bumpers[e.buzzer]; !ok {
				t.Error("le buzzer physique est conservé")
			}
			if !allZero243(s.Bumpers) || !allZero243(s.Scores) {
				t.Errorf("v11.1 : scores à 0 : %v %v", s.Bumpers, s.Scores)
			}
			if s.History != 0 {
				t.Errorf("v11.1 : historique effacé (%d)", s.History)
			}
			if s.TeamKeys != 2 {
				t.Errorf("v11.1 : équipes conservées (%d)", s.TeamKeys)
			}
			if s.Status != game.QuestionStatus("AVAILABLE") && s.Status != "" {
				t.Errorf("v11.1 : questions remises à jouer : %q", s.Status)
			}
			if ev := evictedEach243(t, e.app, e.paulConn, e.mamieConn); !ev[0] || !ev[1] {
				t.Error("chaque tablette reçoit PLAYER_EVICTED {GAME_RESET}")
			}
		})
	}
}

func TestNewGame243_EvictedTabletsReceiveGameResetReason(t *testing.T) {
	e := newEnv243(t)
	e.newGame(t, map[string]interface{}{"PURGE_SCORES": true, "PURGE_PLAYERS": true})
	raw := readActionMatchingWithin(t, e.paulConn, protocol.ActionPlayerEvicted, 2*time.Second)
	var ev struct {
		Reason string `json:"REASON"`
	}
	_ = json.Unmarshal(raw, &ev)
	if ev.Reason != "GAME_RESET" {
		t.Errorf("REASON = %q", ev.Reason)
	}
}

// --- 2. Défaut de la popup (critical) : Scores + Questions ---------------------------------

func TestNewGame243_Critical_PopupDefaultResetsScoresHistoryAndQuestionsOnly(t *testing.T) {
	e := newEnv243(t)

	e.newGame(t, map[string]interface{}{"PURGE_SCORES": true, "PURGE_PLAYERS": false, "PURGE_TEAMS": false, "RESET_QUESTIONS": true})

	s := e.snap()
	if s.Phase != "NEW_GAME" {
		t.Errorf("phase = %s", s.Phase)
	}
	if !allZero243(s.Bumpers) || !allZero243(s.Scores) || s.History != 0 {
		t.Errorf("scores et historique à 0 : %v %v hist=%d", s.Bumpers, s.Scores, s.History)
	}
	if s.Status != "AVAILABLE" && s.Status != "" {
		t.Errorf("questions remises à jouer : %q", s.Status)
	}
	if _, ok := s.Bumpers[e.paul]; !ok || s.TeamKeys != 2 {
		t.Errorf("tablettes et équipes conservées : %+v", s)
	}
	if ev := evictedEach243(t, e.app, e.paulConn, e.mamieConn); ev[0] || ev[1] {
		t.Error("aucun PLAYER_EVICTED : les tablettes restent inscrites")
	}
}

// --- Chaque option isolée -----------------------------------------------------------------

func TestNewGame243_PurgeScoresAloneKeepsQuestionsPlayed(t *testing.T) {
	e := newEnv243(t)
	played := e.snap().Status
	e.newGame(t, map[string]interface{}{"PURGE_SCORES": true})
	s := e.snap()
	if !allZero243(s.Bumpers) || s.History != 0 {
		t.Errorf("scores/historique à 0 : %+v", s)
	}
	if s.Status != played {
		t.Errorf("RESET_QUESTIONS absent = false (dès qu'un champ est présent) : statut %q → %q", played, s.Status)
	}
}

func TestNewGame243_ResetQuestionsTrueVsFalse(t *testing.T) {
	for _, tc := range []struct {
		reset     bool
		wantReset bool
	}{{true, true}, {false, false}} {
		e := newEnv243(t)
		played := e.snap().Status
		if played == "AVAILABLE" {
			t.Fatal("précondition : la question a été jouée")
		}

		e.newGame(t, map[string]interface{}{"PURGE_SCORES": false, "PURGE_PLAYERS": false, "PURGE_TEAMS": false, "RESET_QUESTIONS": tc.reset})

		got := e.snap().Status
		if tc.wantReset && got != "AVAILABLE" && got != "" {
			t.Errorf("RESET_QUESTIONS=true : toutes « à jouer », obtenu %q", got)
		}
		if !tc.wantReset && got != played {
			t.Errorf("RESET_QUESTIONS=false : statuts conservés (%q → %q)", played, got)
		}
		if !tc.reset && e.snap().History == 0 {
			t.Error("scores/historique conservés quand PURGE_SCORES=false")
		}
	}
}

func TestNewGame243_PurgePlayersWithScoresRemovesTabletsAndKeepsBuzzers(t *testing.T) {
	e := newEnv243(t)

	e.newGame(t, map[string]interface{}{"PURGE_SCORES": true, "PURGE_PLAYERS": true, "PURGE_TEAMS": false, "RESET_QUESTIONS": true})

	s := e.snap()
	if _, ok := s.Bumpers[e.paul]; ok {
		t.Error("VJoueurs supprimés")
	}
	if _, ok := s.Bumpers[e.buzzer]; !ok {
		t.Error("buzzer physique conservé")
	}
	if !evicted243(t, e.app, e.paulConn) {
		t.Error("PLAYER_EVICTED {GAME_RESET}")
	}
	if !allZero243(s.Bumpers) {
		t.Errorf("scores à 0 : %v", s.Bumpers)
	}
}

func TestNewGame243_PurgeTeamsRemovesEveryTeamAndUnassignsEveryone(t *testing.T) {
	e := newEnv243(t)
	if _, ok := e.app.wsHub.GetClientPlayerID(e.paulCID); !ok {
		t.Fatal("précondition : la tablette de Paul est liée")
	}

	e.newGame(t, map[string]interface{}{"PURGE_SCORES": false, "PURGE_PLAYERS": false, "PURGE_TEAMS": true, "RESET_QUESTIONS": false})

	s := e.snap()
	if s.TeamKeys != 0 {
		t.Errorf("toutes les équipes supprimées : %d", s.TeamKeys)
	}
	for id, team := range s.TeamOf {
		if team != "" {
			t.Errorf("le joueur %s doit être sans équipe (TEAM = \"\") : %q", id, team)
		}
	}
	if _, ok := s.Bumpers[e.paul]; !ok || len(s.Bumpers) != 3 {
		t.Errorf("tablettes et buzzer conservés : %v", s.Bumpers)
	}
	if evicted243(t, e.app, e.paulConn) {
		t.Error("les tablettes ne sont pas évincées (PURGE_PLAYERS=false)")
	}
	if _, ok := e.app.wsHub.GetClientPlayerID(e.paulCID); !ok {
		t.Error("les tablettes restent liées (#246)")
	}
	// #249 : un TEAM_DELETED par équipe supprimée, l'historique des points est conservé
	deleted := map[string]bool{}
	for _, ev := range e.app.engine.GetHistory() {
		if ev.EventType == "TEAM_DELETED" {
			deleted[ev.TeamName] = true
		}
	}
	if !deleted["Les Bleus"] || !deleted["Les Rouges"] {
		t.Errorf("TEAM_DELETED pour chaque équipe supprimée : %v", deleted)
	}
	if s.History == 0 {
		t.Error("PURGE_SCORES=false : historique conservé")
	}
}

func TestNewGame243_AllThreeOptionsLeaveOnlyUnassignedPhysicalBuzzersWithZeroScores(t *testing.T) {
	e := newEnv243(t)

	e.newGame(t, map[string]interface{}{"PURGE_SCORES": true, "PURGE_PLAYERS": true, "PURGE_TEAMS": true, "RESET_QUESTIONS": true})

	s := e.snap()
	if s.TeamKeys != 0 || len(s.Bumpers) != 1 {
		t.Fatalf("seul le buzzer physique reste, sans équipe : %+v", s)
	}
	if s.TeamOf[e.buzzer] != "" || s.Bumpers[e.buzzer] != 0 {
		t.Errorf("buzzer sans équipe, score 0 : %+v", s)
	}
}

func TestNewGame243_NothingCheckedOnlyAppliesTheAlwaysEffects(t *testing.T) {
	e := newEnv243(t)
	before := e.snap()

	e.newGame(t, map[string]interface{}{"PURGE_SCORES": false, "PURGE_PLAYERS": false, "PURGE_TEAMS": false, "RESET_QUESTIONS": false})

	s := e.snap()
	if s.Phase != "NEW_GAME" || s.QuestionID != "" {
		t.Errorf("toujours : phase NEW_GAME, plus de question courante : %+v", s)
	}
	if s.History != before.History || s.TeamKeys != before.TeamKeys || len(s.Bumpers) != len(before.Bumpers) {
		t.Errorf("rien d'autre ne change : %+v → %+v", before, s)
	}
	for id, sc := range before.Bumpers {
		if s.Bumpers[id] != sc {
			t.Errorf("score de %s conservé : %d → %d", id, sc, s.Bumpers[id])
		}
	}
	if s.Status != before.Status {
		t.Errorf("statuts conservés : %q → %q", before.Status, s.Status)
	}
	if evicted243(t, e.app, e.paulConn) {
		t.Error("aucune éviction")
	}
}

// --- 3. Refus (critical) --------------------------------------------------------------------

func TestNewGame243_Critical_PlayersWithoutScoresIsRefusedAndNothingChanges(t *testing.T) {
	cases := map[string]map[string]interface{}{
		"explicite": {"PURGE_SCORES": false, "PURGE_PLAYERS": true, "PURGE_TEAMS": false, "RESET_QUESTIONS": true},
		"seul":      {"PURGE_PLAYERS": true},
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv243(t)
			other, _ := e.dial(t, "/ws/admin")
			// partie en cours : une question tourne, chrono compris
			e.app.engine.Ready("", &game.Question{ID: "q-run", Type: game.QuestionTypeArdoise})
			e.app.engine.SetPhase(game.PhaseStarted)
			before := e.snap()
			beforeStatus := e.app.engine.GetQuestionStatus("q-ardoise")

			e.newGame(t, payload)

			raw := readActionMatchingWithin(t, e.adminConn, "NEW_GAME_REFUSED", 2*time.Second)
			var refused struct {
				Reason string `json:"REASON"`
			}
			_ = json.Unmarshal(raw, &refused)
			if refused.Reason != "PLAYERS_REQUIRE_SCORES" {
				t.Errorf("REASON = %q", refused.Reason)
			}
			// un seul témoin pour « other » et pour l'éviction testée plus bas (#266)
			seen := collectAfterSentinel264(t, e.app, other, e.paulConn)
			if containsAction(seen[0], "NEW_GAME_REFUSED") {
				t.Fatal("NEW_GAME_REFUSED est ciblé sur l'émetteur seul")
			}
			after := e.snap()
			if after.Phase != before.Phase || after.QuestionID != before.QuestionID {
				t.Errorf("phase/question inchangées : %s/%s → %s/%s", before.Phase, before.QuestionID, after.Phase, after.QuestionID)
			}
			if len(after.Bumpers) != len(before.Bumpers) || after.TeamKeys != before.TeamKeys || after.History != before.History {
				t.Errorf("joueurs, équipes, historique inchangés : %+v → %+v", before, after)
			}
			for id, sc := range before.Bumpers {
				if after.Bumpers[id] != sc {
					t.Errorf("score de %s inchangé : %d → %d", id, sc, after.Bumpers[id])
				}
			}
			if e.app.engine.GetQuestionStatus("q-ardoise") != beforeStatus {
				t.Error("statuts des questions inchangés")
			}
			if containsAction(seen[1], protocol.ActionPlayerEvicted) {
				t.Error("aucune éviction")
			}
			if !e.hasLog(game.LogLevelWarn, "NEW_GAME_REFUSED", "options=") {
				t.Errorf("WARN NEW_GAME_REFUSED (client, options, reason) ; logs=%v", e.logLines(game.LogLevelWarn))
			}
			if e.countLog(game.LogLevelWarn, "NEW_GAME_PURGE") != 0 {
				t.Error("un refus n'est pas journalisé comme une purge appliquée")
			}
		})
	}
}

// --- 7. Minuteurs arrêtés (critical) ---------------------------------------------------------

func TestNewGame243_Critical_RunningTimerNeverBringsThePhaseBackToStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("attend des minuteurs réels (≈ 6 s), exclu de la boucle rapide (-short)")
	}
	run := func(t *testing.T, name string, newGameAt func(e *env243)) {
		t.Run(name, func(t *testing.T) {
			e := newEnv243(t)
			ticks := make(chan struct{}, 100)
			e.app.engine.OnTimerTick = func(ct int) {
				select {
				case ticks <- struct{}{}:
				default:
				}
			}
			e.app.engine.SetPhase(game.PhaseStopped)
			e.app.engine.Ready("", &game.Question{ID: "q-timer", Type: game.QuestionTypeArdoise})
			e.app.engine.SetPhase(game.PhaseReady)
			e.app.engine.Start(2) // décompte 3 s puis chrono 2 s
			newGameAt(e)
			for len(ticks) > 0 {
				<-ticks
			}

			e.newGame(t, map[string]interface{}{"PURGE_SCORES": true, "PURGE_PLAYERS": false, "PURGE_TEAMS": false, "RESET_QUESTIONS": true})

			deadline := time.Now().Add(6 * time.Second) // bien au-delà de la fin du décompte et du chrono
			for time.Now().Before(deadline) {
				if ph := e.app.engine.GetPhase(); ph != game.GamePhase("NEW_GAME") {
					t.Fatalf("la phase doit rester NEW_GAME, obtenu %s (un minuteur a survécu au NEW_GAME)", ph)
				}
				time.Sleep(100 * time.Millisecond)
			}
			if len(ticks) > 0 {
				t.Errorf("aucun tick de chrono après NEW_GAME (%d reçus)", len(ticks))
			}
		})
	}
	run(t, "pendant le décompte", func(e *env243) { time.Sleep(500 * time.Millisecond) })
	run(t, "pendant le chrono (STARTED)", func(e *env243) {
		deadline := time.Now().Add(5 * time.Second)
		for e.app.engine.GetPhase() != game.PhaseStarted && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
		time.Sleep(300 * time.Millisecond)
	})
}

// --- 8. Double NEW_GAME ---------------------------------------------------------------------

func TestNewGame243_DoubleNewGameIsHarmless(t *testing.T) {
	e := newEnv243(t)
	payload := map[string]interface{}{"PURGE_SCORES": true, "PURGE_PLAYERS": true, "PURGE_TEAMS": false, "RESET_QUESTIONS": true}

	e.newGame(t, payload)
	first := e.snap()
	e.newGame(t, payload)
	second := e.snap()

	if first.Phase != second.Phase || len(first.Bumpers) != len(second.Bumpers) || first.History != second.History {
		t.Errorf("état final identique : %+v / %+v", first, second)
	}
	if e.countLog(game.LogLevelWarn, "NEW_GAME_REFUSED") != 0 {
		t.Error("le second NEW_GAME est rejoué sans erreur")
	}
	if n := e.countLog(game.LogLevelWarn, "NEW_GAME_PURGE"); n != 2 {
		t.Errorf("2 lignes NEW_GAME_PURGE attendues : %d", n)
	}
}

// --- 9. TEAMS_LOCKED / AWARDED_TEAMS ---------------------------------------------------------

func TestNewGame243_UnlocksTeamsAndClearsAwardedTeams(t *testing.T) {
	e := newEnv243(t)
	e.app.engine.Ready("", &game.Question{ID: "q-next", Type: game.QuestionTypeArdoise}) // verrou posé
	if s := e.snap(); !s.Locked {
		t.Fatal("précondition : TEAMS_LOCKED = true")
	}

	e.newGame(t, map[string]interface{}{"PURGE_SCORES": false, "PURGE_PLAYERS": false, "PURGE_TEAMS": false, "RESET_QUESTIONS": false})

	if e.snap().Locked {
		t.Error("TEAMS_LOCKED = false après NEW_GAME")
	}
	if got := e.app.getAwardedTeamsPayload().Teams; len(got) != 0 {
		t.Errorf("AWARDED_TEAMS vide (plus de question courante) : %+v", got)
	}
}

func TestNewGame243_AwardedTeamsIsRebroadcastToAnimTablets(t *testing.T) {
	e := newEnv243(t)
	animConn, _ := e.dial(t, "/ws/anim")

	e.newGame(t, map[string]interface{}{"PURGE_SCORES": true, "PURGE_PLAYERS": false, "PURGE_TEAMS": false, "RESET_QUESTIONS": true})

	raw := readActionMatchingWithin(t, animConn, protocol.ActionAwardedTeams, 2*time.Second)
	var p struct {
		Teams []interface{} `json:"TEAMS"`
	}
	_ = json.Unmarshal(raw, &p)
	if len(p.Teams) != 0 {
		t.Errorf("AWARDED_TEAMS rediffusé vide : %s", raw)
	}
}

// --- 10. Journal ------------------------------------------------------------------------------

func TestNewGame243_EveryAppliedNewGameIsJournaledWithOptionsAndCounters(t *testing.T) {
	cases := []struct {
		name    string
		payload interface{}
		parts   []string
	}{
		{"sans options (v11.1)", map[string]interface{}{}, []string{"players_removed=2", "teams_removed=0", "scores_reset=true", "questions_reset=true"}},
		{"popup par défaut", map[string]interface{}{"PURGE_SCORES": true, "RESET_QUESTIONS": true}, []string{"players_removed=0", "teams_removed=0", "scores_reset=true", "questions_reset=true"}},
		{"équipes + scores", map[string]interface{}{"PURGE_SCORES": true, "PURGE_TEAMS": true}, []string{"players_removed=0", "teams_removed=2", "scores_reset=true", "questions_reset=false"}},
		{"aucune option", map[string]interface{}{"PURGE_SCORES": false, "PURGE_PLAYERS": false, "PURGE_TEAMS": false, "RESET_QUESTIONS": false}, []string{"players_removed=0", "teams_removed=0", "scores_reset=false", "questions_reset=false"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv243(t)
			e.newGame(t, tc.payload)
			for _, p := range append([]string{"NEW_GAME_PURGE", "options=", "phase_before=", "question_before="}, tc.parts...) {
				if !e.hasLog(game.LogLevelWarn, "NEW_GAME_PURGE", p) {
					t.Errorf("WARN NEW_GAME_PURGE doit contenir %q ; logs=%v", p, e.logLines(game.LogLevelWarn))
				}
			}
		})
	}
}
