package synth

import (
	"time"

	"buzzcontrol/internal/audio"
)

// Equal-tempered note frequencies (Hz) used below — named for readability,
// nothing more; the actual synthesis only ever sees the float64.
const (
	noteC3 = 130.81
	noteG3 = 196.00
	noteE4 = 329.63
	noteA4 = 440.00
	noteG4 = 392.00
	noteC5 = 523.25
	noteE5 = 659.25
	noteG5 = 783.99
	noteC6 = 1046.50
	noteE6 = 1318.51
)

// Generate returns the deterministic default WAV bytes for c, or false if
// c is not part of the closed v11.0 catalogue (contracts/sound.md §2.1) —
// mirrors audio.Bank.PCM's own (data, ok) shape, though Generate always
// synthesises fresh rather than reading from disk (that's FileBank's job,
// bank.go). Character suggestions per plan de dev #229 §2.3 — adjustable
// at the source, never at the call site: `depart` a brief ascending call,
// `temps-ecoule` two descending notes, `gagne` an ascending arpeggio,
// `perdu` a dull descending interval, `reveal` a chime (decaying tone),
// `entracte-debut`/`entracte-fin` two mirrored motifs. Every cue is well
// under the <1s target (contract's own consequence of #228's Play now
// blocking the strictly-sequential engine — a long cue would delay every
// cue queued behind it on a game that has already moved on).
func Generate(c audio.Cue) ([]byte, bool) {
	switch c {
	case audio.CueDepart:
		return WriteWAV(sequence([]note{
			{freq: noteC5, duration: 90 * time.Millisecond, amplitude: 0.7},
			{freq: noteE5, duration: 110 * time.Millisecond, amplitude: 0.7},
		})), true

	case audio.CueTempsEcoule:
		return WriteWAV(sequence([]note{
			{freq: noteA4, duration: 150 * time.Millisecond, amplitude: 0.7},
			{freq: noteE4, duration: 200 * time.Millisecond, amplitude: 0.7},
		})), true

	case audio.CueGagne:
		return WriteWAV(sequence([]note{
			{freq: noteC5, duration: 80 * time.Millisecond, amplitude: 0.65},
			{freq: noteE5, duration: 80 * time.Millisecond, amplitude: 0.65},
			{freq: noteG5, duration: 80 * time.Millisecond, amplitude: 0.65},
			{freq: noteC6, duration: 90 * time.Millisecond, amplitude: 0.7},
		})), true

	case audio.CuePerdu:
		return WriteWAV(sequence([]note{
			{freq: noteG3, duration: 160 * time.Millisecond, amplitude: 0.55},
			{freq: noteC3, duration: 220 * time.Millisecond, amplitude: 0.55},
		})), true

	case audio.CueReveal:
		return WriteWAV(decayingTone(noteE6, 500*time.Millisecond, 0.6)), true

	case audio.CueEntracteDebut:
		return WriteWAV(sequence([]note{
			{freq: noteE5, duration: 110 * time.Millisecond, amplitude: 0.6},
			{freq: noteC5, duration: 110 * time.Millisecond, amplitude: 0.6},
			{freq: noteG4, duration: 130 * time.Millisecond, amplitude: 0.6},
		})), true

	case audio.CueEntracteFin:
		return WriteWAV(sequence([]note{
			{freq: noteG4, duration: 110 * time.Millisecond, amplitude: 0.6},
			{freq: noteC5, duration: 110 * time.Millisecond, amplitude: 0.6},
			{freq: noteE5, duration: 130 * time.Millisecond, amplitude: 0.6},
		})), true
	}
	return nil, false
}

// Cues is the closed v11.0 catalogue, in the same order as
// contracts/sound.md §2.1 — the enumeration WriteAll/reconciliation walks,
// so both stay in lockstep with the contract by construction rather than
// by a separately-maintained list.
var Cues = []audio.Cue{
	audio.CueDepart,
	audio.CueTempsEcoule,
	audio.CueGagne,
	audio.CuePerdu,
	audio.CueReveal,
	audio.CueEntracteDebut,
	audio.CueEntracteFin,
}
