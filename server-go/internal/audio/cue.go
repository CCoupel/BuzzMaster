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

// Cues par type de jeu (v11.6.0, #231 — contracts/sound.md §2.1 amendement).
// Une cue par site d'émission (D4) ; Type() ≠ "" les distingue des 7 cues globales.
const (
	CueQCMIndice         Cue = "qcm-indice"         // QCM : un indice barre une réponse
	CueMemoryCarte       Cue = "memory-carte"       // MEMORY : carte retournée (face visible)
	CueMemoryPaire       Cue = "memory-paire"       // MEMORY : paire trouvée
	CueMemotionSelection Cue = "memotion-selection" // MEMOTION : carte choisie dans la grille
	CueMemotionFinCarte  Cue = "memotion-fin-carte" // MEMOTION : carte fermée sans gagnant
	CueRafaleBonne       Cue = "rafale-bonne"       // RAFALE : réponse validée
	CueRafaleMauvaise    Cue = "rafale-mauvaise"    // RAFALE : réponse invalidée / passée
	CueRafaleFinLot      Cue = "rafale-fin-lot"     // RAFALE : fin du lot (réservoir épuisé / nombre max atteint)
)

// Types de jeu portés par Cue.Type().
const (
	CueTypeQCM      = "QCM"
	CueTypeMemory   = "MEMORY"
	CueTypeMemotion = "MEMOTION"
	CueTypeRafale   = "RAFALE"
)

// AllCues renvoie les 15 cues dans l'ordre normatif : les 7 globales, puis
// QCM, MEMORY, MEMOTION, RAFALE (contracts/sound.md §2.1).
func AllCues() []Cue {
	return []Cue{
		CueDepart, CueTempsEcoule, CueGagne, CuePerdu, CueReveal, CueEntracteDebut, CueEntracteFin,
		CueQCMIndice,
		CueMemoryCarte, CueMemoryPaire,
		CueMemotionSelection, CueMemotionFinCarte,
		CueRafaleBonne, CueRafaleMauvaise, CueRafaleFinLot,
	}
}

// Type renvoie le type de jeu de la cue ("QCM", "MEMORY", "MEMOTION", "RAFALE"),
// ou "" pour les 7 cues globales (et toute valeur inconnue).
func (c Cue) Type() string {
	switch c {
	case CueQCMIndice:
		return CueTypeQCM
	case CueMemoryCarte, CueMemoryPaire:
		return CueTypeMemory
	case CueMemotionSelection, CueMemotionFinCarte:
		return CueTypeMemotion
	case CueRafaleBonne, CueRafaleMauvaise, CueRafaleFinLot:
		return CueTypeRafale
	}
	return ""
}

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
