package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
)

// warnSeatsWithoutFingerprint writes the WARN of contracts/vplayer-session.md §6: after loading the players
// (startup) or restoring a backup, some VJoueur seats may hold no token fingerprint — file absent, unreadable or
// corrupt, backup from before v13.0.0 or without vplayer_sessions.json. Those seats are back to "first to
// arrive" (matrix case 1c). The line carries the NUMBER of such seats and the cause, never an ID, a token or a
// fingerprint.
func (a *App) warnSeatsWithoutFingerprint(when string, loadErr error) {
	n := a.engine.VPlayerSeatsWithoutFingerprint()
	if n == 0 && loadErr == nil {
		return
	}
	cause := "empreintes chargées"
	if loadErr != nil {
		cause = "vplayer_sessions.json absent, illisible ou corrompu"
	}
	server.LogWarn(game.LogComponentApp, "VPLAYER_SESSIONS (%s) : %s — %d place(s) VJoueur sans empreinte, en « premier arrivé » (cas 1c) jusqu'à leur première reconnexion", when, cause, n)
}
