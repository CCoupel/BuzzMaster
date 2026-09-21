package synth

import (
	"encoding/binary"
	"math"
	"time"

	"buzzcontrol/internal/audio"
)

// envelopeDuration is the linear fade applied at the start and end of
// EVERY note this package synthesises (plan de dev #229 §2.3 — "enveloppe
// d'attaque et d'extinction obligatoire", a WAV that starts or ends at a
// non-zero amplitude produces an audible click). Applying it per-note
// rather than only once at the very start/end of the whole clip also
// removes any click BETWEEN two consecutive notes — a stricter guarantee
// than the plan's letter asked for, cheap to provide.
const envelopeDuration = 8 * time.Millisecond

// note is one tone in a sequence: freq in Hz (0 = a silent rest, useful as
// a short breath between two notes), amplitude in [0,1] (peak, before the
// envelope), for duration.
type note struct {
	freq      float64
	duration  time.Duration
	amplitude float64
}

// sequence renders notes back to back into raw canonical-format PCM bytes
// (interleaved S16LE stereo, audio.SampleRate/ChannelCount — contracts/
// sound.md §3). Deterministic: freq/duration/amplitude are the only
// inputs, and math.Sin is itself deterministic for a given argument.
func sequence(notes []note) []byte {
	var pcm []byte
	for _, n := range notes {
		pcm = append(pcm, renderNote(n)...)
	}
	return pcm
}

// renderNote synthesises one sine-wave note (or silence, freq==0) with a
// linear attack/release envelope over envelopeDuration at each end —
// halved automatically for a note shorter than 2*envelopeDuration so the
// fades never overlap into a negative middle segment.
func renderNote(n note) []byte {
	frames := int(n.duration.Seconds() * float64(audio.SampleRate))
	if frames <= 0 {
		return nil
	}
	fadeFrames := int(envelopeDuration.Seconds() * float64(audio.SampleRate))
	if fadeFrames*2 > frames {
		fadeFrames = frames / 2
	}

	pcm := make([]byte, frames*audio.FrameSize)
	for i := 0; i < frames; i++ {
		env := 1.0
		switch {
		case fadeFrames > 0 && i < fadeFrames:
			env = float64(i) / float64(fadeFrames)
		case fadeFrames > 0 && i >= frames-fadeFrames:
			env = float64(frames-1-i) / float64(fadeFrames)
		}

		var sample int16
		if n.freq > 0 {
			t := float64(i) / float64(audio.SampleRate)
			v := n.amplitude * env * math.Sin(2*math.Pi*n.freq*t)
			sample = int16(v * 32767)
		}

		off := i * audio.FrameSize
		for ch := 0; ch < audio.ChannelCount; ch++ {
			binary.LittleEndian.PutUint16(pcm[off+ch*2:off+ch*2+2], uint16(sample))
		}
	}
	return pcm
}

// decayingTone synthesises a single tone whose amplitude fades linearly
// from 1.0 to 0.0 over its whole duration (a "chime"/carillon envelope,
// used for CueReveal) instead of the fixed short attack/release used by
// sequence's ordinary notes. The first envelopeDuration still ramps UP
// from 0 (same anti-click guarantee at the start); the release IS the
// decay itself, already reaching exactly 0 at the last frame.
func decayingTone(freq float64, duration time.Duration, amplitude float64) []byte {
	frames := int(duration.Seconds() * float64(audio.SampleRate))
	if frames <= 0 {
		return nil
	}
	attackFrames := int(envelopeDuration.Seconds() * float64(audio.SampleRate))
	if attackFrames > frames {
		attackFrames = frames
	}

	pcm := make([]byte, frames*audio.FrameSize)
	for i := 0; i < frames; i++ {
		attack := 1.0
		if attackFrames > 0 && i < attackFrames {
			attack = float64(i) / float64(attackFrames)
		}
		decay := float64(frames-1-i) / float64(frames-1)

		t := float64(i) / float64(audio.SampleRate)
		v := amplitude * attack * decay * math.Sin(2*math.Pi*freq*t)
		sample := int16(v * 32767)

		off := i * audio.FrameSize
		for ch := 0; ch < audio.ChannelCount; ch++ {
			binary.LittleEndian.PutUint16(pcm[off+ch*2:off+ch*2+2], uint16(sample))
		}
	}
	return pcm
}
