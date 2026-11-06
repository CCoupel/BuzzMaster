// #282 (v11.4.0, LOT-2) — fan-out réel d'une manche RAFALE alphabétique : la lettre courante et les
// lettres restantes atteignent admin, /anim, /tv et VPlayer (contrat rafale.md §15.5) ; la réponse
// attendue n'atteint JAMAIS TV/VPlayer (§2.3) — seule sa lettre y figure. Frontière WS ; option posée
// par JSON (aucun symbole non livré). Ne modifie pas rafale_answer_leak_test.go (propriété dev-backend).
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"buzzcontrol/internal/game"

	"github.com/gorilla/websocket"
)

const tw282Secret = "Canberra-SECRET"

func tw282AlphaFanoutApp(t *testing.T) *App {
	t.Helper()
	app := newAnimTestApp(t)
	for _, rq := range []game.RafaleQuestion{
		{ID: "r-1", Question: "Capitale de l'Australie ?", Answer: tw282Secret, Category: game.CategoryHistory, Difficulty: 1},
		{ID: "r-2", Question: "Fruit jaune ?", Answer: "Banane", Category: game.CategoryHistory, Difficulty: 1},
	} {
		if _, err := app.engine.UpsertRafaleQuestion(rq); err != nil {
			t.Fatalf("UpsertRafaleQuestion : %v", err)
		}
	}
	q := &game.Question{ID: "rafale-alpha-q", Question: "RAFALE", Type: game.QuestionTypeRafale,
		Category: game.CategoryHistory, Points: "10", Time: "120",
		TypedContent: game.TypedContent{RafaleDifficulty: 1, RafaleMode: string(game.RafaleModeSolo), RafaleQuestionTime: 3, RafaleMaxQuestions: 100}}
	raw, _ := json.Marshal(q)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	m["RAFALE_ALPHABETICAL"] = true
	raw, _ = json.Marshal(m)
	var qa game.Question
	if err := json.Unmarshal(raw, &qa); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	app.engine.Ready(qa.ID, &qa)
	app.engine.StartImmediate(0)
	t.Cleanup(app.engine.Stop)
	return app
}

func TestRafaleAlpha282_FanOut_LettersReachAllWebClients_AnswerNeverTVOrVPlayer(t *testing.T) {
	app := tw282AlphaFanoutApp(t)
	baseURL := startAnimAllowlistTestServer(t, app)
	clients := []struct {
		name, path string
		conn       *websocket.Conn
	}{
		{name: "admin", path: "/ws/admin"}, {name: "anim", path: "/ws/anim"},
		{name: "tv", path: "/ws/tv"}, {name: "vplayer", path: "/ws/player"},
	}
	for i := range clients {
		clients[i].conn = dialWS(t, baseURL, clients[i].path)
		learnClientID(t, app, clients[i].conn)
	}

	app.broadcastUpdate()

	for _, c := range clients {
		_, raw := readActionMatching(t, c.conn, "UPDATE")
		var env struct {
			Msg struct {
				Game map[string]interface{} `json:"GAME"`
			} `json:"MSG"`
		}
		if err := json.Unmarshal([]byte(raw), &env); err != nil {
			t.Fatalf("%s : UPDATE illisible : %v", c.name, err)
		}
		letter, _ := env.Msg.Game["RAFALE_CURRENT_LETTER"].(string)
		if len(letter) != 1 {
			t.Errorf("%s doit recevoir RAFALE_CURRENT_LETTER (1 caractère) dans GAME, got %q", c.name, letter)
		}
		if _, ok := env.Msg.Game["RAFALE_LETTERS"].([]interface{}); !ok {
			t.Errorf("%s doit recevoir RAFALE_LETTERS (tableau) dans GAME, got %#v", c.name, env.Msg.Game["RAFALE_LETTERS"])
		}
		if strings.Contains(raw, tw282Secret) {
			t.Errorf("%s : la réponse attendue ne doit jamais figurer dans UPDATE : %s", c.name, raw)
		}
	}
}
