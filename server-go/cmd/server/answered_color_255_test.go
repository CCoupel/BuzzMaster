package main

// #255 (v11.2) — Bumper.ANSWERED_COLOR côté serveur : sérialisation vers tous les clients (scénario 7), ambiance et LED
// (scénario 8). Le moteur est couvert par internal/game/answered_color_255_test.go. Contrats : models.md §Bumper,
// ws-payload-serialization.md, vplayer-payload-filter.md. Statut `regression`.

import (
	"buzzcontrol/internal/game"
	"encoding/json"
	"testing"
	"time"
)

// bumperKeys255 : pour chaque trame UPDATE portant des bumpers, la liste des objets bumper (clé → présence de la clé).
func bumpersOf255(frames []string) (updates int, missing []string) {
	for _, f := range frames {
		a, m := envelope255(f)
		if a != "UPDATE" {
			continue
		}
		var p struct {
			Bumpers map[string]map[string]json.RawMessage `json:"bumpers"`
		}
		if json.Unmarshal(m, &p) != nil || len(p.Bumpers) == 0 {
			continue
		}
		updates++
		for id, b := range p.Bumpers {
			if _, ok := b["ANSWERED_COLOR"]; !ok {
				missing = append(missing, id)
			}
		}
	}
	return
}

func envelope255(frame string) (string, json.RawMessage) { return envelope253(frame) }

// --- 7. Sérialisation -----------------------------------------------------------------------------------

func TestAnsweredColor255_KeyIsAlwaysSerializedToAdminTVAnimAndVPlayer(t *testing.T) {
	e := newEnv253(t, false) // question ARDOISE STARTED : suffisant, la clé ne dépend pas du type
	for _, c := range []struct {
		name string
		do   func()
	}{
		{"STARTED", func() {}},
		{"REVEALED", func() { e.app.engine.Stop(); e.app.engine.Reveal() }},
		{"PREPARE (chemin réduit)", func() {
			e.app.engine.Stop()
			e.app.engine.Ready("q-next", &game.Question{ID: "q-next", Type: game.QuestionTypeArdoise})
		}},
	} {
		for _, conn := range []string{"a1", "an", "tv", "player"} {
			_ = conn
		}
		c.do()
		for _, f := range []struct {
			name string
			tap  *tap253
		}{{"admin", tapOf253(e.a1)}, {"anim", tapOf253(e.an)}, {"tv", tapOf253(e.tvc)}, {"vplayer", tapOf253(e.paulConn)}} {
			f.tap.take()
		}
		e.app.broadcastUpdate()
		time.Sleep(400 * time.Millisecond)
		for _, f := range []struct {
			name string
			tap  *tap253
		}{{"admin", tapOf253(e.a1)}, {"anim", tapOf253(e.an)}, {"tv", tapOf253(e.tvc)}, {"vplayer", tapOf253(e.paulConn)}} {
			n, missing := bumpersOf255(f.tap.take())
			if n == 0 {
				continue // ce destinataire ne reçoit pas de bumpers dans cet état : rien à vérifier
			}
			if len(missing) != 0 {
				t.Errorf("%s / %s : ANSWERED_COLOR absente de %v (toujours sérialisée, sans omitempty)", c.name, f.name, missing)
			}
		}
	}
}

func TestAnsweredColor255_AdminTVAnimAndVPlayerAllReceiveBumpersWithTheKeyOnAFullUpdate(t *testing.T) {
	e := newEnv253(t, false)
	for _, conn := range []*tap253{tapOf253(e.a1), tapOf253(e.an), tapOf253(e.tvc), tapOf253(e.paulConn)} {
		conn.take()
	}
	e.app.broadcastUpdate()
	time.Sleep(400 * time.Millisecond)
	for name, tp := range map[string]*tap253{"admin": tapOf253(e.a1), "anim": tapOf253(e.an), "tv": tapOf253(e.tvc), "vplayer": tapOf253(e.paulConn)} {
		n, missing := bumpersOf255(tp.take())
		if n == 0 {
			t.Errorf("%s : aucun UPDATE avec bumpers reçu", name)
			continue
		}
		if len(missing) != 0 {
			t.Errorf("%s : ANSWERED_COLOR manquante pour %v", name, missing)
		}
	}
}

// --- 8. Ambiance et LED ---------------------------------------------------------------------------------

// qcmAnsweredApp255 : QCM (bonne réponse GREEN) ; l'équipe A a deux buzzers physiques : b1 (assigné RED) répond A (RED,
// fausse) puis b2 (assigné GREEN = la bonne réponse) N'A PAS répondu ; l'équipe B (b3 assigné GREEN) n'a pas répondu non plus.
func qcmAnsweredApp255(t *testing.T) *App {
	t.Helper()
	app := newBroadcast127TestApp(t)
	app.engine.SetTeams(map[string]*game.Team{"A": {Name: "A", Color: []int{255, 0, 0}}, "B": {Name: "B", Color: []int{0, 0, 255}}})
	app.engine.SetBumpers(map[string]*game.Bumper{
		"AA:00:00:00:00:01": {Name: "P1", Team: "A", AnswerColor: game.AnswerColorRed, Connected: true},
		"AA:00:00:00:00:02": {Name: "P2", Team: "A", AnswerColor: game.AnswerColorGreen, Connected: true},
		"AA:00:00:00:00:03": {Name: "P3", Team: "B", AnswerColor: game.AnswerColorGreen, Connected: true},
	})
	app.engine.Ready("q1", &game.Question{
		ID: "q1", Question: "QCM ?", Type: game.QuestionTypeQCM,
		TypedContent: game.TypedContent{QCMAnswers: &game.QCMAnswers{Red: "a", Green: "b", Yellow: "c", Blue: "d"}, QCMCorrect: "GREEN"},
	})
	app.engine.StartImmediate(30)
	app.engine.ProcessButtonPress("AA:00:00:00:00:01", time.Now().UnixMicro(), "A")
	app.engine.Stop()
	app.engine.Reveal()
	// états de buzz tels que la production les pose : P1 a buzzé (MOI), P2 est son équipier (ÉQUIPE)
	app.bumperBuzzState = map[string]game.BuzzState{"AA:00:00:00:00:01": game.BuzzStateMoi, "AA:00:00:00:00:02": game.BuzzStateEquipe}
	return app
}

func TestAnsweredColor255_Critical_AmbianceDoesNotCountATeamWhoseAssignedColorIsTheRightAnswerButWhoDidNotAnswer(t *testing.T) {
	app := qcmAnsweredApp255(t)
	// b2 appuie sur un bouton hors A-D (aucune couleur répondue) : TIME > 0 mais pas de réponse QCM
	app2 := newBroadcast127TestApp(t)
	app2.engine.SetTeams(map[string]*game.Team{"B": {Name: "B", Color: []int{0, 0, 255}}})
	app2.engine.SetBumpers(map[string]*game.Bumper{"AA:00:00:00:00:09": {Name: "P9", Team: "B", AnswerColor: game.AnswerColorGreen, Connected: true}})
	app2.engine.Ready("q1", &game.Question{ID: "q1", Question: "QCM ?", Type: game.QuestionTypeQCM,
		TypedContent: game.TypedContent{QCMAnswers: &game.QCMAnswers{Red: "a", Green: "b", Yellow: "c", Blue: "d"}, QCMCorrect: "GREEN"}})
	app2.engine.StartImmediate(30)
	app2.engine.ProcessButtonPress("AA:00:00:00:00:09", time.Now().UnixMicro(), "Z") // bouton inconnu : buzz sans réponse QCM
	app2.engine.Stop()
	app2.engine.Reveal()

	if got := app.ambianceCorrectTeams(app.engine.GetState()); len(got) != 0 {
		t.Errorf("A a répondu RED (fausse), B n'a pas répondu : aucune équipe n'a la bonne réponse, obtenu %v", got)
	}
	if got := app2.ambianceCorrectTeams(app2.engine.GetState()); len(got) != 0 {
		t.Errorf("un buzz sans réponse QCM (bouton hors A-D) ne compte pas comme la bonne réponse même si la couleur assignée est la bonne : %v", got)
	}
}

func TestAnsweredColor255_Critical_LEDShowsAGoodAnswerOnlyForAPlayerWhoActuallyGaveTheRightAnswer(t *testing.T) {
	app := qcmAnsweredApp255(t)
	state := app.engine.GetState()
	const p2 = "AA:00:00:00:00:02"

	// P2 (équipier de P1, qui a répondu faux) a la bonne couleur ASSIGNÉE mais n'a pas répondu : jamais « bonne réponse »
	app.sendLEDSetForBuzzerQCMReveal(p2, app.engine.GetBumper(p2), state)

	if got := app.bumperLEDState[p2]; got.Effect != "DIM" {
		t.Errorf("P2 n'a pas répondu : LED « mauvaise/aucune réponse » (DIM 25 %%), pas BLINK/SOLID ; obtenu %+v", got)
	}
}

func TestAnsweredColor255_LEDStillBlinksForTheFirstTeamWithTheRightAnswer(t *testing.T) {
	app := newBroadcast127TestApp(t)
	app.engine.SetTeams(map[string]*game.Team{"A": {Name: "A", Color: []int{255, 0, 0}}})
	const p1 = "AA:00:00:00:00:01"
	app.engine.SetBumpers(map[string]*game.Bumper{p1: {Name: "P1", Team: "A", AnswerColor: game.AnswerColorRed, Connected: true}})
	app.engine.Ready("q1", &game.Question{ID: "q1", Question: "QCM ?", Type: game.QuestionTypeQCM,
		TypedContent: game.TypedContent{QCMAnswers: &game.QCMAnswers{Red: "a", Green: "b", Yellow: "c", Blue: "d"}, QCMCorrect: "GREEN"}})
	app.engine.StartImmediate(30)
	app.engine.ProcessButtonPress(p1, time.Now().UnixMicro(), "B") // répond GREEN = la bonne réponse
	app.engine.Stop()
	app.engine.Reveal()
	app.bumperBuzzState = map[string]game.BuzzState{p1: game.BuzzStateMoi}

	app.sendLEDSetForBuzzerQCMReveal(p1, app.engine.GetBumper(p1), app.engine.GetState())

	if got := app.bumperLEDState[p1]; got.Effect != "BLINK" {
		t.Errorf("non-régression : bonne réponse + premier buzz = BLINK ; obtenu %+v", got)
	}
	if got := app.ambianceCorrectTeams(app.engine.GetState()); len(got) != 1 || got[0] != "A" {
		t.Errorf("non-régression : l'ambiance compte l'équipe A : %v", got)
	}
}
