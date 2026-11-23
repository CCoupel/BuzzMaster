package server

import (
	"errors"
	"io/fs"

	"buzzcontrol/internal/game"
)

// LogVPlayerSessionsWarn writes the WARN of contracts/vplayer-session.md §6 — shared by the startup
// (cmd/server) and the restoration of a backup (handleRestore). seats is the NUMBER of VJoueur seats without
// token fingerprint (matrix case 1c, "first to arrive"); loadErr is the result of loading
// vplayer_sessions.json; noErrCause names the situation when the file loaded fine but some seats still have no
// fingerprint. No WARN when the file is absent AND no seat is exposed (fresh install), nor when it loaded and
// no seat is exposed; an unreadable or corrupt file warns even at 0 seat. The line carries a count, never an
// ID, a name, a token or a fingerprint. Reports whether a line was written.
func LogVPlayerSessionsWarn(component game.LogComponent, when string, seats int, loadErr error, noErrCause string) bool {
	if seats == 0 && (loadErr == nil || errors.Is(loadErr, fs.ErrNotExist)) {
		return false
	}
	cause := noErrCause
	switch {
	case errors.Is(loadErr, fs.ErrNotExist):
		cause = "vplayer_sessions.json absent"
	case loadErr != nil:
		cause = "vplayer_sessions.json absent, illisible ou corrompu"
	}
	LogWarn(component, "VPLAYER_SESSIONS (%s) : %s — %d place(s) VJoueur sans empreinte, en « premier arrivé » (cas 1c) jusqu'à leur première reconnexion", when, cause, seats)
	return true
}
