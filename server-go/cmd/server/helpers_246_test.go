package main

// Aides communes des tests de spécification #246 (v11.2) — identité, journalisation,
// ARDOISE_FINAL. Contrats : contracts/websocket-actions.md, contracts/logging.md.

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// env246 : une partie ARDOISE démarrée (STARTED) avec deux VJoueurs, un par équipe.
type env246 struct {
	app   *App
	base  string
	paul  string // bumper de l'équipe "Les Bleus"
	mamie string // bumper de l'équipe "Les Rouges"
}

// newEnv246 construit l'App de test, route le logger global vers app.logger (les
// handlers journalisent via server.LogWarn ET a.logger) et démarre une question ARDOISE.
func newEnv246(t *testing.T) *env246 {
	t.Helper()
	app := newBroadcast127TestApp(t)
	server.SetGlobalLogger(app.logger)
	t.Cleanup(func() { server.SetGlobalLogger(nil) })

	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{"Les Bleus": {Name: "Les Bleus"}, "Les Rouges": {Name: "Les Rouges"}})
	paul := setupVirtualPlayer(t, app, "Paul", "Les Bleus")
	mamie := setupVirtualPlayer(t, app, "Mamie", "Les Rouges")
	app.engine.SetPhase(game.PhaseStopped)
	app.engine.Ready("", &game.Question{ID: "q-ardoise", Type: game.QuestionTypeArdoise})
	app.engine.SetPhase(game.PhaseStarted)

	base := startAnimAllowlistTestServer(t, app)
	return &env246{app: app, base: base, paul: paul, mamie: mamie}
}

// dial ouvre une vraie WebSocket du type donné ("/ws/player", "/ws/admin", "/ws/anim"…)
// et retourne la connexion et l'identifiant que le hub lui a attribué.
func (e *env246) dial(t *testing.T, path string) (*websocket.Conn, string) {
	t.Helper()
	c := dialWS(t, e.base, path)
	return c, learnClientID(t, e.app, c)
}

// send fait traiter un message par le vrai dispatcher, comme s'il venait de la connexion clientID.
func (e *env246) send(t *testing.T, clientID, clientType, action string, payload interface{}) {
	t.Helper()
	msg, err := protocol.NewMessage(action, payload)
	if err != nil {
		t.Fatalf("NewMessage(%s): %v", action, err)
	}
	e.app.handleWebMessage(&protocol.IncomingMessage{Source: "WebSocket", Data: msg, ClientID: clientID, ClientType: clientType})
}

// connect envoie PLAYER_CONNECT {NAME, ID} depuis la connexion vplayer clientID.
func (e *env246) connect(t *testing.T, clientID, name, id string) {
	t.Helper()
	e.send(t, clientID, "vplayer", protocol.ActionPlayerConnect, protocol.PlayerConnectPayload{Name: name, ID: id})
}

// answers retourne, par équipe, le texte ARDOISE et le drapeau RECEIVED vus dans l'état complet.
func (e *env246) answers() (text map[string]string, received map[string]bool) {
	var gd struct {
		Game struct {
			A map[string]struct {
				Text     string `json:"TEXT"`
				Received bool   `json:"RECEIVED"`
			} `json:"ARDOISE_ANSWERS"`
		} `json:"GAME"`
	}
	_ = json.Unmarshal(e.app.engine.GetGameJSON(), &gd)
	text, received = map[string]string{}, map[string]bool{}
	for k, v := range gd.Game.A {
		text[k], received[k] = v.Text, v.Received
	}
	return
}

// answersRaw retourne le JSON brut de GAME.ARDOISE_ANSWERS[team] (pour vérifier la présence des clés).
func (e *env246) answersRaw(team string) map[string]json.RawMessage {
	var gd struct {
		Game struct {
			A map[string]map[string]json.RawMessage `json:"ARDOISE_ANSWERS"`
		} `json:"GAME"`
	}
	_ = json.Unmarshal(e.app.engine.GetGameJSON(), &gd)
	return gd.Game.A[team]
}

// logLines retourne les messages du tampon de logs de l'App pour le niveau donné ("" = tous).
func (e *env246) logLines(level game.LogLevel) []string {
	var out []string
	for _, en := range e.app.logger.GetHistory() {
		if level == "" || en.Level == level {
			out = append(out, en.Message)
		}
	}
	return out
}

// hasLog : une entrée de niveau `level` contient toutes les sous-chaînes `parts`.
func (e *env246) hasLog(level game.LogLevel, parts ...string) bool {
	for _, en := range e.app.logger.GetHistory() {
		if en.Level != level {
			continue
		}
		ok := true
		for _, p := range parts {
			if !strings.Contains(en.Message, p) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// countLog : nombre d'entrées de niveau `level` contenant `part`.
func (e *env246) countLog(level game.LogLevel, part string) int {
	n := 0
	for _, en := range e.app.logger.GetHistory() {
		if en.Level == level && strings.Contains(en.Message, part) {
			n++
		}
	}
	return n
}

// waitLog attend (≤ 2 s) qu'une entrée de niveau `level` contienne `part`.
func (e *env246) waitLog(t *testing.T, level game.LogLevel, part string) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if e.hasLog(level, part) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// setGlobalLogger246 route le logger global vers app.logger (et le remet à nil en fin de test).
func setGlobalLogger246(t *testing.T, app *App) {
	t.Helper()
	server.SetGlobalLogger(app.logger)
	t.Cleanup(func() { server.SetGlobalLogger(nil) })
}
