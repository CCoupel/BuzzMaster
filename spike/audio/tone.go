package main

import (
	"bytes"
	"encoding/binary"
	"math"
)

// sampleRate and channelCount are fixed for the whole spike: 44100 Hz
// stereo, the safest default oto/v3 supports without resampling on any
// backend (README: "Usually 44100 or 48000 [...] Other values might cause
// distortions").
const (
	sampleRate   = 44100
	channelCount = 2
)

// genTone returns raw PCM S16LE stereo samples (no WAV header — oto's
// Player reads a raw PCM stream, the format is declared once on the
// Context) for a pure sine wave at freqHz for durationMs, with a short
// linear fade-in/out to avoid the audible "click" of an abrupt edge. This
// is NOT a production asset pipeline decision — it only exists so the
// spike has a self-contained, dependency-free "sound" to play without
// shipping a .wav file inside a throwaway program.
func genTone(freqHz float64, durationMs int) []byte {
	n := sampleRate * durationMs / 1000
	fadeSamples := sampleRate / 100 // 10ms fade
	buf := new(bytes.Buffer)
	buf.Grow(n * channelCount * 2)
	for i := 0; i < n; i++ {
		t := float64(i) / float64(sampleRate)
		amp := 0.6
		if i < fadeSamples {
			amp *= float64(i) / float64(fadeSamples)
		} else if i > n-fadeSamples {
			amp *= float64(n-i) / float64(fadeSamples)
		}
		s := int16(amp * 32767 * math.Sin(2*math.Pi*freqHz*t))
		_ = binary.Write(buf, binary.LittleEndian, s) // left
		_ = binary.Write(buf, binary.LittleEndian, s) // right
	}
	return buf.Bytes()
}

// cues mirrors, at the scale of a throwaway demo, the "one distinct sound
// per event kind" idea from the plan (§C). Frequencies are chosen only to
// be told apart by ear if a human is listening — no relation whatsoever to
// a real sound design decision, which belongs to a later phase.
var cues = map[string][]byte{
	"temps-ecoule": genTone(880, 250), // the ONE latency-sensitive cue (plan §3.5)
	"reveal":       genTone(440, 200),
	"gagne":        genTone(660, 300),
	"entracte-fin": genTone(550, 200),
}
