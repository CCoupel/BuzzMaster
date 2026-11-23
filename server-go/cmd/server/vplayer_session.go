package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
)

// warnSeatsWithoutFingerprint writes the WARN of contracts/vplayer-session.md §6 after the players are loaded at
// startup — the rule (no WARN for an absent file with 0 exposed seat; count only, never an ID) is shared with the
// restoration of a backup in server.LogVPlayerSessionsWarn.
func (a *App) warnSeatsWithoutFingerprint(when string, loadErr error) {
	server.LogVPlayerSessionsWarn(game.LogComponentApp, when, a.engine.VPlayerSeatsWithoutFingerprint(), loadErr, "empreintes chargées")
}
