package main

import (
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/led"
	"buzzcontrol/internal/server"
)

// hosts.go — *App implements the Host interfaces of the extracted packages
// (rounds, demo, led) by one-line methods (#291/#295).

// Engine and QuestionsDirPath make *App a rounds.Host.
func (a *App) Engine() *game.Engine { return a.engine }

// Config, ResolveCategoryMeta and SaveBackgroundsConfig make *App a demo.Host.
func (a *App) Config() *config.Config { return a.config }

func (a *App) ResolveCategoryMeta(key string) (name, imageURL, color string) {
	return a.httpServer.ResolveCategoryMeta(key)
}

func (a *App) SaveBackgroundsConfig() { a.saveBackgroundsConfig() }

func (a *App) QuestionsDirPath() string { return a.questionsDirPath() }

// BuzzerHub, AckManager and BroadcastUpdateTo make *App a led.Host.
func (a *App) BuzzerHub() *server.BuzzerWebSocketHub { return a.buzzerHub }

func (a *App) AckManager() *server.AckManager { return a.ackManager }

func (a *App) BroadcastUpdateTo(types ...server.ClientType) { a.broadcastUpdateTo(types...) }

// ledCtl returns the LED controller, built on first use over the App-owned LED state
// (App literals in tests never call a constructor, hence the lazy build).
func (a *App) ledCtl() *led.Controller {
	a.ledOnce.Do(func() {
		a.ledc = led.New(a, &a.ledStateMu, &a.bumperLEDState, &a.bumperBuzzState)
	})
	return a.ledc
}
