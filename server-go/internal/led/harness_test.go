package led

import (
	"sync"
	"testing"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// #295 D3 — harnais des tests déplacés depuis cmd/server : un faux « App » à état LED propre (mêmes noms de champs et de
// méthodes que l'App, délégués au Controller), avec moteur réel, BuzzerWebSocketHub réel et AckManager réel ;
// BroadcastUpdateTo est un enregistreur. Les tests déplacés gardent ainsi leur corps à l'identique.

type App struct {
	engine          *game.Engine
	buzzerHub       *server.BuzzerWebSocketHub
	ackManager      *server.AckManager
	bumperLEDState  map[string]protocol.LEDSetPayload
	ledStateMu      sync.RWMutex
	bumperBuzzState map[string]game.BuzzState

	mu       sync.Mutex
	updateTo [][]server.ClientType
	ctl      *Controller
}

func (a *App) Engine() *game.Engine                  { return a.engine }
func (a *App) BuzzerHub() *server.BuzzerWebSocketHub { return a.buzzerHub }
func (a *App) AckManager() *server.AckManager        { return a.ackManager }
func (a *App) BroadcastUpdateTo(t ...server.ClientType) {
	a.mu.Lock()
	a.updateTo = append(a.updateTo, t)
	a.mu.Unlock()
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	engine := game.NewEngine()
	engine.SetTeams(map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}},
		"TeamB": {Name: "TeamB", Color: []int{0, 0, 255}},
		"TeamC": {Name: "TeamC", Color: []int{0, 255, 0}},
	})
	a := &App{
		engine:          engine,
		buzzerHub:       server.NewBuzzerWebSocketHub(),
		ackManager:      server.NewAckManager(&config.Get().Server),
		bumperLEDState:  make(map[string]protocol.LEDSetPayload),
		bumperBuzzState: make(map[string]game.BuzzState),
	}
	a.ctl = New(a, &a.ledStateMu, &a.bumperLEDState, &a.bumperBuzzState)
	return a
}

func newTestAppWithHub(t *testing.T) *App { return newTestApp(t) }

func (a *App) teamColorToRGB(b *game.Bumper) [3]int            { return a.ctl.TeamColor(b) }
func (a *App) teamNameToRGB(n string) [3]int                   { return a.ctl.TeamNameColor(n) }
func (a *App) sendLEDSet(mac string, p protocol.LEDSetPayload) { a.ctl.Send(mac, p) }
func (a *App) broadcastLEDSet(p protocol.LEDSetPayload)        { a.ctl.Broadcast(p) }
func (a *App) resendLEDOnReconnect(mac string)                 { a.ctl.ResendOnReconnect(mac) }
func (a *App) resetBuzzStates()                                { a.ctl.ResetBuzzStates() }
func (a *App) updateBuzzStates(mac string)                     { a.ctl.UpdateBuzzStates(mac) }
func (a *App) buzzStateFor(mac string) game.BuzzState          { return a.ctl.BuzzStateFor(mac) }
func (a *App) sendLEDSetForBuzzerQCMReveal(mac string, b *game.Bumper, s game.GameState) {
	a.ctl.ForBuzzerQCMReveal(mac, b, s)
}
func (a *App) sendLEDSetRafaleTeams()                              { a.ctl.RafaleTeams() }
func (a *App) sendLEDSetAllBuzzers()                               { a.ctl.AllBuzzers() }
func (a *App) sendLEDSetStop()                                     { a.ctl.Stop() }
func (a *App) sendLEDSetPause(id string)                           { a.ctl.Pause(id) }
func (a *App) sendLEDSetPauseAll()                                 { a.ctl.PauseAll() }
func (a *App) sendLEDSetContinue()                                 { a.ctl.Continue() }
func (a *App) sendLEDSetReveal(answer string)                      { a.ctl.Reveal(answer) }
func (a *App) sendLEDSetToTeam(t string, p protocol.LEDSetPayload) { a.ctl.ToTeam(t, p) }
func (a *App) sendLEDSetComet(team string)                         { a.ctl.Comet(team) }
