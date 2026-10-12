package main

// #245 (v11.2, lot V3-1) — « à suivre » propose les ENTRACTE ; l'anim peut LANCER une entracte par « suivant » (READY puis START,
// actions qu'elle a déjà) et la TERMINER (ENTRACTE_SET{ACTIVE:false}), jamais la déclencher à la main (ACTIVE:true refusé,
// WARN ENTRACTE_SET_ANIM_ACTIVATE_REFUSED). Contrats : websocket-actions.md §NEXT_QUESTION, §ENTRACTE_SET, §LAUNCH_LOCK
// (05a96984, a50b9f32). Scénarios 34 à 38 (planner-20261012-233000.md §5). `critical` : 34.

import (
	"buzzcontrol/internal/game"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func (e *env245) until(what string, cond func() bool) bool {
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}

func (e *env245) entracteOn() bool { return e.app.engine.IsEntracte() }

func (e *env245) sendAs(t *testing.T, who, action string, payload interface{}) {
	t.Helper()
	switch who {
	case "anim":
		e.send(t, e.anim, "anim", action, payload)
	case "admin":
		e.send(t, e.admin2, "admin", action, payload)
	}
}

// fixtureFlow245 : « Sans manche » [q1], manche r1 [début e1, q2, fin e2], manche r2 [q3].
func (e *env245) fixtureFlow(t *testing.T) []string {
	t.Helper()
	e.writeQ(t, "1", 0, "", "")
	e.writeQ(t, "2", 1, "r1", "")
	e.writeQ(t, "3", 0, "r2", "")
	for id, o := range map[string]int{"11": 0, "12": 2} {
		e.writeQ(t, id, o, "r1", "ENTRACTE")
	}
	// ordre : q1 | e1(11) q2 e2(12) | q3 → renumérote q2 entre les deux entractes
	e.writeQ(t, "2", 1, "r1", "")
	if err := os.WriteFile(e.dir+"/rounds.json", []byte(`{"ROUNDS":[{"ID":"r1","NAME":"Un"},{"ID":"r2","NAME":"Deux"}],"NEXT_ID":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return []string{"1", "11", "2", "12", "3"}
}

// --- 34. Déroulé complet piloté par l'anim seule (critical) -------------------------------------------------------------------

func TestRoundsEntracte245_Critical_AnimAloneDrivesAFullRunThroughEntractesAndRounds(t *testing.T) {
	e := newEnv245(t)
	want := e.fixtureFlow(t)
	e.app.engine.SetPhase(game.PhaseStopped)
	e.app.engine.Ready("", &game.Question{ID: "", Type: game.QuestionTypeSpeedy}) // aucune question courante

	var visited []string
	for step := 0; step < len(want)+1; step++ {
		next := e.app.getNextQuestionPayload()
		if next == nil || next.ID == "" {
			break
		}
		visited = append(visited, next.ID)

		e.sendAs(t, "anim", "READY", map[string]interface{}{"QUESTION": next.ID}) // « suivant »
		// les tablettes confirment leur présence (PONG), comme à chaque PREPARE
		e.send(t, e.paulCID, "vplayer", "PONG", map[string]interface{}{"ID": e.paul})
		e.send(t, e.admin2, "admin", "PONG", map[string]interface{}{"ID": e.mamie})
		if !e.until("READY", func() bool { return e.app.engine.GetPhase() == game.PhaseReady }) {
			t.Fatalf("%s : READY par l'anim, phase=%s", next.ID, e.app.engine.GetPhase())
		}
		e.sendAs(t, "anim", "START", map[string]interface{}{"DELAY": 1})
		if next.Type == "ENTRACTE" {
			if !e.until("ENTRACTE", e.entracteOn) {
				t.Fatalf("%s : START par l'anim → GameState.ENTRACTE = true (phase=%s)", next.ID, e.app.engine.GetPhase())
			}
			e.sendAs(t, "anim", "ENTRACTE_SET", map[string]interface{}{"ACTIVE": false}) // « Terminer l'entracte »
			if !e.until("fin", func() bool { return !e.entracteOn() && e.app.engine.GetPhase() == game.PhaseStopped }) {
				t.Fatalf("%s : ENTRACTE_SET{ACTIVE:false} par l'anim → ENTRACTE false et entrée STOPPED (phase=%s)", next.ID, e.app.engine.GetPhase())
			}
		} else {
			if !e.until("STARTED", func() bool { return e.app.engine.GetPhase() == game.PhaseStarted }) {
				t.Fatalf("%s : START par l'anim → STARTED (phase=%s)", next.ID, e.app.engine.GetPhase())
			}
			e.sendAs(t, "anim", "STOP", map[string]interface{}{})
			e.until("STOPPED", func() bool { return e.app.engine.GetPhase() == game.PhaseStopped })
			e.sendAs(t, "anim", "REVEAL", map[string]interface{}{})
			e.until("REVEALED", func() bool { return e.app.engine.GetPhase() == game.PhaseRevealed })
		}
	}

	if len(visited) != len(want) {
		t.Fatalf("déroulé visité %v, attendu %v", visited, want)
	}
	for i := range want {
		if visited[i] != want[i] {
			t.Errorf("étape %d : %s, attendu %s (Sans manche, début de manche, question, fin de manche, manche suivante)", i, visited[i], want[i])
		}
	}
	if e.countLog(game.LogLevelWarn, "Rejected action") != 0 {
		t.Errorf("aucune action de l'anim n'est rejetée ; logs=%v", e.logLines(game.LogLevelWarn))
	}
}

// --- 35. L'anim ne déclenche pas une entracte à la main -------------------------------------------------------------------------

func TestRoundsEntracte245_AnimCannotActivateAnEntracteByHand(t *testing.T) {
	e := newEnv245(t)
	e.app.engine.SetPhase(game.PhaseStopped)

	e.sendAs(t, "anim", "ENTRACTE_SET", map[string]interface{}{"ACTIVE": true})
	time.Sleep(200 * time.Millisecond)

	if e.entracteOn() {
		t.Error("ENTRACTE_SET{ACTIVE:true} venant de l'anim : refusé, état inchangé")
	}
	if !e.hasLog(game.LogLevelWarn, "ENTRACTE_SET_ANIM_ACTIVATE_REFUSED") {
		t.Errorf("WARN ENTRACTE_SET_ANIM_ACTIVATE_REFUSED ; logs=%v", e.logLines(game.LogLevelWarn))
	}
	// l'admin garde ce droit
	e.sendAs(t, "admin", "ENTRACTE_SET", map[string]interface{}{"ACTIVE": true})
	if !e.until("admin", e.entracteOn) {
		t.Error("l'admin déclenche toujours une entracte manuelle")
	}
}

// --- 36. L'anim termine une entracte manuelle de l'admin ---------------------------------------------------------------------------

func TestRoundsEntracte245_AnimCanEndAManualEntracteStartedByTheAdmin(t *testing.T) {
	e := newEnv245(t)
	e.app.engine.SetPhase(game.PhaseStopped)
	e.sendAs(t, "admin", "ENTRACTE_SET", map[string]interface{}{"ACTIVE": true})
	if !e.until("on", e.entracteOn) {
		t.Fatal("précondition : entracte manuelle active")
	}

	e.sendAs(t, "anim", "ENTRACTE_SET", map[string]interface{}{"ACTIVE": false})

	if !e.until("off", func() bool { return !e.entracteOn() }) {
		t.Error("l'anim termine une entracte manuelle (ACTIVE:false accepté)")
	}
}

// --- 37. LAUNCH_LOCK ----------------------------------------------------------------------------------------------------------------

func TestRoundsEntracte245_EndingAnEntracteIsAllowedUnderTheLaunchLockButStartingOneIsNot(t *testing.T) {
	e := newEnv245(t)
	e.app.engine.SetPhase(game.PhaseStopped)
	e.sendAs(t, "admin", "ENTRACTE_SET", map[string]interface{}{"ACTIVE": true})
	if !e.until("on", e.entracteOn) {
		t.Fatal("précondition")
	}
	// verrou posé par admin1 (l'entracte est active : le serveur refuse l'ouverture pendant une entracte → on pose le verrou avant)
	e.sendAs(t, "admin", "ENTRACTE_SET", map[string]interface{}{"ACTIVE": false})
	e.until("off", func() bool { return !e.entracteOn() })
	e.openLock(t, e.admin1)
	if !e.waitLock(func(l lockState253) bool { return l.Active }) {
		t.Fatal("précondition : verrou posé")
	}
	tapOf253(e.a2).take()
	tapOf253(e.an).take()

	e.sendAs(t, "admin", "ENTRACTE_SET", map[string]interface{}{"ACTIVE": true}) // suspendu
	if _, ok := readRefused253(e.a2, "ENTRACTE_SET"); !ok {
		t.Error("ENTRACTE_SET{ACTIVE:true} sous verrou : ACTION_REFUSED{LAUNCH_IN_PROGRESS}")
	}
	if e.entracteOn() {
		t.Error("rien n'est appliqué sous verrou")
	}

	// une entracte active (posée avant le verrou par le moteur) se termine malgré le verrou
	e.app.engine.SetEntracte(true)
	e.sendAs(t, "anim", "ENTRACTE_SET", map[string]interface{}{"ACTIVE": false})
	if !e.until("fin sous verrou", func() bool { return !e.entracteOn() }) {
		t.Error("ENTRACTE_SET{ACTIVE:false} reste permis pendant LAUNCH_LOCK (sortir d'une entracte n'est jamais bloqué)")
	}
	if e.hasLog(game.LogLevelWarn, "ACTION_REFUSED_LAUNCH") && countAction253(tapOf253(e.an).take(), "ACTION_REFUSED") != 0 {
		t.Error("l'anim n'est pas refusée pour ACTIVE:false")
	}
}

// --- 38. TV et tablettes : l'entracte s'affiche comme avant ------------------------------------------------------------------------

func TestRoundsEntracte245_TVAndTabletsStillReceiveTheEntractePanelState(t *testing.T) {
	e := newEnv245(t)
	e.app.engine.SetPhase(game.PhaseStopped)
	tapOf253(e.tvc).take()
	tapOf253(e.paulConn).take()

	e.sendAs(t, "admin", "ENTRACTE_SET", map[string]interface{}{"ACTIVE": true})
	time.Sleep(400 * time.Millisecond)

	for name, tp := range map[string]*tap253{"tv": tapOf253(e.tvc), "tablette": tapOf253(e.paulConn)} {
		found := false
		for _, f := range tp.take() {
			var env struct {
				Msg struct {
					Game map[string]json.RawMessage `json:"GAME"`
				} `json:"MSG"`
			}
			if json.Unmarshal([]byte(f), &env) == nil {
				if v, ok := env.Msg.Game["ENTRACTE"]; ok && string(v) == "true" {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s : GAME.ENTRACTE = true diffusé (panneau habituel, inchangé)", name)
		}
	}
}
