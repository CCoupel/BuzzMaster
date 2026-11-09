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

	// --- cues par type de jeu (v11.6.0, #231) — courtes (< 0,35 s), distinctes ---
	case audio.CueQCMIndice: // bip sec descendant : une réponse est barrée
		return WriteWAV(sequence([]note{
			{freq: noteE5, duration: 60 * time.Millisecond, amplitude: 0.55},
			{freq: noteA4, duration: 90 * time.Millisecond, amplitude: 0.55},
		})), true

	case audio.CueMemoryCarte: // tic clair : carte retournée
		return WriteWAV(sequence([]note{
			{freq: noteG5, duration: 70 * time.Millisecond, amplitude: 0.5},
		})), true

	case audio.CueMemoryPaire: // deux notes montantes : paire trouvée
		return WriteWAV(sequence([]note{
			{freq: noteG5, duration: 80 * time.Millisecond, amplitude: 0.6},
			{freq: noteC6, duration: 120 * time.Millisecond, amplitude: 0.65},
		})), true

	case audio.CueMemotionSelection: // note médiane courte : carte choisie
		return WriteWAV(sequence([]note{
			{freq: noteC5, duration: 70 * time.Millisecond, amplitude: 0.55},
			{freq: noteG4, duration: 70 * time.Millisecond, amplitude: 0.5},
		})), true

	case audio.CueMemotionFinCarte: // cloche décroissante : carte refermée
		return WriteWAV(decayingTone(noteE5, 300*time.Millisecond, 0.5)), true

	case audio.CueRafaleBonne: // arpège bref ascendant
		return WriteWAV(sequence([]note{
			{freq: noteE5, duration: 55 * time.Millisecond, amplitude: 0.6},
			{freq: noteG5, duration: 70 * time.Millisecond, amplitude: 0.6},
		})), true

	case audio.CueRafaleMauvaise: // buzz grave bref
		return WriteWAV(sequence([]note{
			{freq: noteC3, duration: 160 * time.Millisecond, amplitude: 0.55},
		})), true

	case audio.CueRafaleFinLot: // trois notes descendantes : fin du lot
		return WriteWAV(sequence([]note{
			{freq: noteG5, duration: 90 * time.Millisecond, amplitude: 0.6},
			{freq: noteE5, duration: 90 * time.Millisecond, amplitude: 0.6},
			{freq: noteC5, duration: 150 * time.Millisecond, amplitude: 0.6},
		})), true
	}
	return nil, false
}

// Cues is the closed catalogue (7 cues in v11.0, 15 since v11.6.0 — #231), in the same order as
// contracts/sound.md §2.1 — the enumeration WriteAll/reconciliation walks,
// so both stay in lockstep with the contract by construction rather than
// by a separately-maintained list.
var Cues = audio.AllCues()

// CountdownName is the manifest key / file stem of the chronometer
// countdown sound (v11.4.0, #287, contracts/sound.md §12.2). A plain string,
// NEVER an audio.Cue: the countdown is not an impulse cue (§2.2), so the
// closed 7-value catalogue (Cues, soundCueFromString) is untouched.
const CountdownName = "decompte"

// CountdownFilename is the countdown's on-disk name, next to the cues.
func CountdownFilename() string { return CountdownName + ".wav" }

// GenerateCountdown returns the deterministic default countdown: a 2.00 s
// tic-tac (two short "tic" per second, ~1 kHz / ~800 Hz alternating),
// amplitude <= 0.5 to stay under the cues. Built to loop seamlessly
// (silence at both ends; each note carries the anti-click envelope).
func GenerateCountdown() []byte {
	second := []note{
		{freq: 1000, duration: 60 * time.Millisecond, amplitude: 0.5},
		{freq: 0, duration: 440 * time.Millisecond},
		{freq: 800, duration: 60 * time.Millisecond, amplitude: 0.5},
		{freq: 0, duration: 440 * time.Millisecond},
	}
	return WriteWAV(sequence(append(append([]note{}, second...), second...)))
}
