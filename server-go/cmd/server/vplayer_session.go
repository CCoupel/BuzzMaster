package main

import (
	"errors"
	"io/fs"

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
	// Nothing to warn about: no seat is exposed, and the file is either loaded or simply absent (a fresh
	// install, before any tablet enrolled). An unreadable or corrupt file warns even at 0 seat.
	if n == 0 && (loadErr == nil || errors.Is(loadErr, fs.ErrNotExist)) {
		return
	}
	cause := "empreintes chargées"
	if errors.Is(loadErr, fs.ErrNotExist) {
		cause = "vplayer_sessions.json absent"
	} else if loadErr != nil {
		cause = "vplayer_sessions.json absent, illisible ou corrompu"
	}
	server.LogWarn(game.LogComponentApp, "VPLAYER_SESSIONS (%s) : %s — %d place(s) VJoueur sans empreinte, en « premier arrivé » (cas 1c) jusqu'à leur première reconnexion", when, cause, n)
}
