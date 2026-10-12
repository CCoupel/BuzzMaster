package main

// #243 (v11.2) — réserves de la revue (_work/reports/code-reviewer-20261011-150000.md ; contrat websocket-actions.md
// §NEW_GAME « Entracte active », « déliaison », « LED/ambiance », statut de la question interrompue) :
// (a) refus pendant une entracte, (c) déliaison des connexions des joueurs purgés, (e) LED_SET + ambiance notifiés.

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
	"buzzcontrol/internal/protocol"
	"context"
	"encoding/json"
	"testing"
	"time"
)

// (a) --------------------------------------------------------------------------------------

func TestNewGameReview243_Critical_NewGameDuringAnEntracteIsRefusedAndNothingChanges(t *testing.T) {
	e := newEnv243(t)
	otherConn, _ := e.dial(t, "/ws/admin")
	if !e.app.engine.SetEntracte(true) || !e.app.engine.IsEntracte() {
		t.Skip("précondition : entracte non activable dans ce montage")
	}
	before := e.snap()

	e.newGame(t, map[string]interface{}{"PURGE_SCORES": true, "PURGE_PLAYERS": true, "PURGE_TEAMS": true, "RESET_QUESTIONS": true})

	raw := readActionMatchingWithin(t, e.adminConn, "NEW_GAME_REFUSED", 2*time.Second)
	var refused struct {
		Reason string `json:"REASON"`
	}
	_ = json.Unmarshal(raw, &refused)
	if refused.Reason != "ENTRACTE_ACTIVE" {
		t.Errorf("REASON = %q (attendu ENTRACTE_ACTIVE)", refused.Reason)
	}
	for _, a := range collectActions(otherConn, 300*time.Millisecond) {
		if a == "NEW_GAME_REFUSED" {
			t.Fatal("NEW_GAME_REFUSED est ciblé sur l'émetteur seul")
		}
	}
	after := e.snap()
	if after.Phase != before.Phase || len(after.Bumpers) != len(before.Bumpers) || after.TeamKeys != before.TeamKeys || after.History != before.History {
		t.Errorf("rien n'est appliqué pendant une entracte : %+v → %+v", before, after)
	}
	for id, sc := range before.Bumpers {
		if after.Bumpers[id] != sc {
			t.Errorf("score de %s inchangé : %d → %d", id, sc, after.Bumpers[id])
		}
	}
	if !e.app.engine.IsEntracte() {
		t.Error("l'entracte n'est pas interrompue par un NEW_GAME refusé")
	}
	if evicted243(t, e.paulConn) {
		t.Error("aucune éviction")
	}
	if !e.hasLog(game.LogLevelWarn, "NEW_GAME_REFUSED", "ENTRACTE_ACTIVE") {
		t.Errorf("WARN NEW_GAME_REFUSED reason=ENTRACTE_ACTIVE ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	if e.countLog(game.LogLevelWarn, "NEW_GAME_PURGE") != 0 {
		t.Error("un refus n'est pas une purge appliquée")
	}
}

func TestNewGameReview243_NewGameAfterLeavingTheEntracteWorks(t *testing.T) {
	e := newEnv243(t)
	e.app.engine.SetEntracte(true)
	e.app.engine.SetEntracte(false)
	e.newGame(t, map[string]interface{}{"PURGE_SCORES": true, "RESET_QUESTIONS": true})
	if e.snap().Phase != "NEW_GAME" {
		t.Errorf("phase = %s : une fois sorti de l'entracte, NEW_GAME s'applique", e.snap().Phase)
	}
}

// (c) --------------------------------------------------------------------------------------

func TestNewGameReview243_Critical_PurgedPlayersConnectionsAreUnboundSo246Holds(t *testing.T) {
	e := newEnv243(t)
	if _, ok := e.app.wsHub.GetClientPlayerID(e.paulCID); !ok {
		t.Fatal("précondition : la connexion de Paul est liée")
	}

	e.newGame(t, map[string]interface{}{"PURGE_SCORES": true, "PURGE_PLAYERS": true, "PURGE_TEAMS": false, "RESET_QUESTIONS": true})

	if id, ok := e.app.wsHub.GetClientPlayerID(e.paulCID); ok {
		t.Errorf("aucune connexion ne doit rester liée à un joueur purgé (invariant #246), encore liée à %q", id)
	}
}

func TestNewGameReview243_ConnectionsOfKeptPlayersStayBound(t *testing.T) {
	e := newEnv243(t)
	e.newGame(t, map[string]interface{}{"PURGE_SCORES": true, "PURGE_PLAYERS": false, "PURGE_TEAMS": true, "RESET_QUESTIONS": true})
	if _, ok := e.app.wsHub.GetClientPlayerID(e.paulCID); !ok {
		t.Error("PURGE_PLAYERS=false : les tablettes restent liées")
	}
}

func TestNewGameReview243_APurgedTabletCannotWriteAfterwards(t *testing.T) {
	e := newEnv243(t)
	e.newGame(t, map[string]interface{}{"PURGE_SCORES": true, "PURGE_PLAYERS": true, "PURGE_TEAMS": false, "RESET_QUESTIONS": true})

	e.app.engine.Ready("", &game.Question{ID: "q-next", Type: game.QuestionTypeArdoise})
	e.app.engine.SetPhase(game.PhaseStarted)
	e.typeAs(t, e.paulCID, e.paul, "FANTOME", nil)

	if text, _ := e.answers(); len(text) != 0 && text["Les Bleus"] != "" {
		t.Errorf("une tablette purgée ne peut plus écrire : %v", text)
	}
}

// (e) --------------------------------------------------------------------------------------

func TestNewGameReview243_Critical_EveryAppliedNewGameRefreshesTheBuzzerLedsAndTheAmbiance(t *testing.T) {
	e := newEnv243(t)
	fake := lighting.NewFakeDriver()
	e.app.lightingWriter.Store(e.app.newAmbianceWriter(fake))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.app.ambiance().Start(ctx)
	e.app.ambiance().NotifyState() // scène de la partie en cours
	waitForCount243(t, fake, 1)
	ambianceBefore := fake.Count()
	// sentinelle : une couleur de LED qu'aucun calcul réel ne produit
	e.app.bumperLEDState[e.buzzer] = protocolLED243(7, 7, 7)

	e.newGame(t, map[string]interface{}{"PURGE_SCORES": false, "PURGE_PLAYERS": false, "PURGE_TEAMS": false, "RESET_QUESTIONS": false})

	if e.app.bumperLEDState[e.buzzer] == protocolLED243(7, 7, 7) {
		t.Error("LED_SET doit être renvoyé aux buzzers après un NEW_GAME appliqué (sendLEDSetAllBuzzers)")
	}
	deadline := time.Now().Add(2 * time.Second)
	for fake.Count() <= ambianceBefore && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if fake.Count() <= ambianceBefore {
		t.Error("l'ambiance doit être notifiée (NotifyState) après un NEW_GAME appliqué : la salle garde sinon la scène de la question interrompue")
	}
}

func TestNewGameReview243_PurgeTeamsPutsBuzzerLedsToNoTeam(t *testing.T) {
	e := newEnv243(t)
	e.app.bumperLEDState[e.buzzer] = protocolLED243(7, 7, 7)
	e.newGame(t, map[string]interface{}{"PURGE_SCORES": true, "PURGE_TEAMS": true, "RESET_QUESTIONS": true})
	if e.app.bumperLEDState[e.buzzer] == protocolLED243(7, 7, 7) {
		t.Error("PURGE_TEAMS : LED « sans équipe » envoyée aux buzzers")
	}
}

func protocolLED243(r, g, b int) protocol.LEDSetPayload {
	return protocol.LEDSetPayload{Color: [3]int{r, g, b}, Intensity: 77, Effect: "SOLID"}
}

func waitForCount243(t *testing.T, f *lighting.FakeDriver, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if f.Count() >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("driver received %d state(s), want >= %d", f.Count(), n)
}
