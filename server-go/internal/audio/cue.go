// Package audio is the sound-bruitage vocabulary and engine for BuzzControl
// (#227, milestone v11.0 — contracts/sound.md).
//
// It deliberately imports neither internal/game nor internal/protocol, same
// discipline as internal/lighting (contracts/lighting.md §2.1): the package
// must compile and be testable entirely on its own. The adapter that turns
// game events into Cue values lives in cmd/server/sound.go.
package audio

import (
	"context"
	"io"
)

// Cue is the sound-bruitage vocabulary. The list is CLOSED for v11.0
// (contract §2.1): a new need adds a cue to the contract, it never
// overloads the meaning of an existing one. Every Cue is an IMPULSION
// (contract §2.2) — a sound is an instant, never a state; unlike
// lighting.EventKind there is no "derivable from GameState" nature to
// distinguish.
type Cue string

const (
	CueDepart        Cue = "depart"         // entered STARTED (a real start, not a resume-after-pause)
	CueTempsEcoule   Cue = "temps-ecoule"   // global chrono expired
	CueGagne         Cue = "gagne"          // points credited
	CuePerdu         Cue = "perdu"          // MEMORY pair missed, RAFALE invalidate/timeout
	CueReveal        Cue = "reveal"         // answer revealed
	CueEntracteDebut Cue = "entracte-debut" // entered intermission
	CueEntracteFin   Cue = "entracte-fin"   // left intermission
)

// Output plays one already-decoded PCM stream, already conforming to the
// canonical format (contract §3: WAV PCM 16-bit, 44100 Hz, stereo — Play
// never has to inspect or convert anything). Symmetric to lighting.Driver
// (contract §4): Play is called ONLY from the engine's single playback
// goroutine (§5.3), so it MAY block and does NOT need to be safe for
// concurrent access.
//
// #227 (this package) does not yet resolve a Cue to a real WAV asset — no
// sound bank exists before #229 ("aucun son n'est encore audible — c'est
// normal et voulu", plan de dev #227 §1). The engine therefore feeds Play a
// placeholder payload (the Cue's own name) so the plumbing — queue,
// non-blocking entry, single goroutine, FIFO order — is fully testable end
// to end with a fake Output, without any real audio asset or hardware. #229
// replaces the placeholder with real PCM bytes; this interface does not
// change.
type Output interface {
	Play(ctx context.Context, pcm io.Reader) error

	// Close releases resources. Idempotent, callable even if Play never
	// succeeded.
	Close() error
}
