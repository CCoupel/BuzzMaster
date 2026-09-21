// Package synth generates BuzzControl's default sound-bruitage catalogue
// (#229, milestone v11.0 — contracts/sound.md §3, plan de dev
// _work/reports/plan-dev-228-229-20260921-114500.md Partie 2).
//
// The cadrage originally planned to ship the default sounds as embedded
// assets (//go:embed) plus a conditional extraction, mirroring the demo
// backgrounds' own pattern. The decision actually taken (recorded in the
// plan, §2.1) drops //go:embed entirely: since every default sound is
// synthesised in Go rather than authored as an audio file, embedding bytes
// this package can just compute would be pure overhead — no binary-size
// cost, no asset pipeline, no licensing question, and "restore the
// delivered sounds" reduces to "run the generator again".
//
// Determinism is the one hard requirement this trades in for: the
// generator must NEVER draw on randomness or the wall clock — contract
// §229 §2.1's own consequence is what lets a byte comparison distinguish
// a default sound from a user's customised one (#230), and what makes
// "restore" trivially idempotent (same input, same bytes, always).
package synth

import (
	"encoding/binary"

	"buzzcontrol/internal/audio"
)

// wavHeaderSize is the fixed size of the canonical header this package
// writes: 12 bytes RIFF + 8+16 bytes "fmt " chunk + 8 bytes "data" chunk
// header = 44 bytes, the classic minimal canonical WAV header. No extra
// chunks (no LIST/INFO, no timestamp) — anything beyond these four would
// either be non-deterministic or pure noise for this project's needs.
const wavHeaderSize = 44

// WriteWAV wraps raw canonical-format PCM samples (audio.SampleRate,
// audio.ChannelCount, audio.BitsPerSample — contracts/sound.md §3) in a
// complete, minimal, deterministic WAV file: a fixed 44-byte RIFF/fmt/data
// header followed by the samples verbatim. Every field is derived from
// `len(pcm)` and the package-level format constants — nothing here reads
// the clock or any source of randomness.
func WriteWAV(pcm []byte) []byte {
	buf := make([]byte, wavHeaderSize+len(pcm))

	byteRate := audio.SampleRate * audio.ChannelCount * (audio.BitsPerSample / 8)
	blockAlign := audio.ChannelCount * (audio.BitsPerSample / 8)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+len(pcm))) // RIFF chunk size: everything after this field
	copy(buf[8:12], "WAVE")

	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16) // fmt chunk size (PCM, no extension)
	binary.LittleEndian.PutUint16(buf[20:22], 1)  // audio format: 1 = PCM (uncompressed)
	binary.LittleEndian.PutUint16(buf[22:24], uint16(audio.ChannelCount))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(audio.SampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:36], uint16(audio.BitsPerSample))

	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(len(pcm)))
	copy(buf[44:], pcm)

	return buf
}
