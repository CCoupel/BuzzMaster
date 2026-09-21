package audio

import (
	"fmt"
	"time"
)

// MaxSoundDuration is the hard upload limit (contracts/http-endpoints.md
// §Sound `POST /api/sounds/{cue}`): the engine plays strictly
// sequentially and Play now genuinely blocks (contract §4 amendment,
// #228) — a long sound delays every cue queued behind it on a game that
// has already moved on.
const MaxSoundDuration = 5 * time.Second

// WarnSoundDuration is the soft threshold: still accepted, but flagged —
// beyond roughly two seconds a bruitage risks overrunning the next game
// moment.
const WarnSoundDuration = 2 * time.Second

// MaxUploadBytes is the explicit size cap (contracts/http-endpoints.md
// §Sound): a canonical 5s sound weighs ~880 KB, 2 MB leaves generous
// headroom without allowing an abusive file.
const MaxUploadBytes = 2 << 20 // 2 MiB

// ValidationResult carries what #230's upload endpoint needs after a
// successful validation: the raw canonical PCM (ready to write to disk
// inside a fresh WAV header, or to feed straight to a Bank/Output), the
// computed duration, and an optional non-fatal warning.
type ValidationResult struct {
	PCM      []byte
	Duration time.Duration
	// Warning is non-empty when Duration exceeds WarnSoundDuration but the
	// file is still accepted — nil/empty otherwise. The caller decides how
	// to surface it (contracts/http-endpoints.md §Sound: `warning` field,
	// null when absent, never an omitted key).
	Warning string
}

// ValidateUpload validates raw file bytes for #230's `POST
// /api/sounds/{cue}` against the canonical format (contract §3) and the
// duration limits above. Built ON TOP of extractCanonicalPCM (bank.go,
// written for #229's FileBank, whose own doc comment already anticipated
// this upload) — contracts/http-endpoints.md §Sound is explicit that this
// must be reused, never a second validator written independently.
//
// Duration is computed from the header alone, without decoding:
// len(pcm) / (SampleRate * ChannelCount * BytesPerSample).
func ValidateUpload(raw []byte) (ValidationResult, error) {
	pcm, err := extractCanonicalPCM(raw)
	if err != nil {
		return ValidationResult{}, err
	}

	duration := pcmDuration(pcm)
	if duration > MaxSoundDuration {
		return ValidationResult{}, fmt.Errorf(
			"ce son dure %.1f s — la limite est de %d s (le moteur joue les sons l'un après l'autre ; un son long retarderait tous les suivants)",
			duration.Seconds(), int(MaxSoundDuration.Seconds()))
	}

	result := ValidationResult{PCM: pcm, Duration: duration}
	if duration > WarnSoundDuration {
		result.Warning = fmt.Sprintf(
			"ce son dure %.1f s — accepté, mais c'est long pour un bruitage (au-delà de %d s environ, il risque de déborder sur le moment de jeu suivant)",
			duration.Seconds(), int(WarnSoundDuration.Seconds()))
	}
	return result, nil
}

// pcmDuration computes a canonical-format PCM buffer's playback duration
// from its byte length alone.
func pcmDuration(pcm []byte) time.Duration {
	seconds := float64(len(pcm)) / float64(FrameSize) / float64(SampleRate)
	return time.Duration(seconds * float64(time.Second))
}
