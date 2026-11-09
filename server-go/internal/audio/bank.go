package audio

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileBank resolves a Cue to raw canonical PCM bytes (contract §3) by
// reading `<dir>/<cue>.wav` from disk and stripping its RIFF/WAVE header —
// Output.Play only ever sees the "data" chunk's raw samples, never a full
// WAV file (contract §4). Disk is authoritative (plan de cadrage §2.2,
// "le disque fait foi") — FileBank never caches: a file replaced on disk
// (default generation, restore, or a future upload in #230) is picked up
// on the very next PlayCue for that cue, with no restart and no explicit
// invalidation.
//
// The filename convention (`<cue>.wav`) is shared with
// internal/audio/synth (synth.Filename) — declared once there, reused
// here, so the two can never drift apart on naming.
type FileBank struct {
	dir string

	warnMu sync.Mutex
	warned map[string]bool // rel paths whose fallback WARN was already emitted (until the next success)
}

// NewFileBank builds a FileBank rooted at dir (typically
// data/files/sounds/). Missing files are not an error here — see PCM's own
// doc comment.
func NewFileBank(dir string) *FileBank {
	return &FileBank{dir: dir}
}

// PCM implements Bank. A missing file, or one that fails to parse as a
// conforming canonical WAV, is a silent no-op — ok=false, logged once,
// never an error surfaced to the engine (contract §5.5: "dégradation
// silencieuse — la règle absolue").
func (b *FileBank) PCM(c Cue) (data []byte, ok bool) {
	path := filepath.Join(b.dir, string(c)+".wav")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false // no file for this cue — nothing configured, not an error
	}
	pcm, err := extractCanonicalPCM(raw)
	if err != nil {
		logf(LogWarn, "audio: %s does not conform to the canonical format (contracts/sound.md §3), cue silently skipped: %v", path, err)
		return nil, false
	}
	return pcm, true
}

// PCMAt implements RelBank (v11.6.0, #290): reads `<dir>/<rel>` (rel is relative to the
// sounds directory, e.g. "rounds/r2/gagne.wav") and strips the header. Missing or
// non-conforming ⇒ FALLBACK on the catalogue file "<cue>.wav" (the base name of rel) with
// a WARN emitted once per rel until the next success (D12: nothing is corrected silently,
// but a cue never becomes mute). A rel that escapes the directory is treated as missing.
func (b *FileBank) PCMAt(rel string) (data []byte, ok bool) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if rel != "" && !filepath.IsAbs(clean) && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		if raw, err := os.ReadFile(filepath.Join(b.dir, clean)); err == nil {
			if pcm, perr := extractCanonicalPCM(raw); perr == nil {
				b.warnMu.Lock()
				delete(b.warned, rel)
				b.warnMu.Unlock()
				return pcm, true
			}
		}
	}
	b.warnMu.Lock()
	first := !b.warned[rel]
	if b.warned == nil {
		b.warned = map[string]bool{}
	}
	b.warned[rel] = true
	b.warnMu.Unlock()
	if first {
		logf(LogWarn, "audio: per-round sound file %q missing or non-conforming — falling back to the catalogue file (ambiance-settings.md §9.4)", rel)
	}
	return b.PCM(Cue(strings.TrimSuffix(filepath.Base(clean), ".wav")))
}

// extractCanonicalPCM parses a RIFF/WAVE byte stream, validates its "fmt "
// chunk against the canonical format (contract §3 — PCM 16-bit, 44100 Hz,
// stereo; any other value is REFUSED, never resampled or reinterpreted,
// per the contract's own "un seul contexte audio par processus" reasoning),
// and returns the "data" chunk's raw bytes. Chunks are walked generically
// (2-byte-padded per the RIFF spec) so an unexpected but harmless chunk
// (e.g. a "LIST"/"INFO" a non-BuzzControl WAV editor might add — relevant
// once #230 accepts uploads) does not by itself cause a rejection.
func extractCanonicalPCM(raw []byte) ([]byte, error) {
	if len(raw) < 12 || string(raw[0:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a RIFF/WAVE file")
	}

	var (
		haveFmt                 bool
		channels, bitsPerSample uint16
		sampleRate              uint32
		data                    []byte
		haveData                bool
	)

	pos := 12
	for pos+8 <= len(raw) {
		id := string(raw[pos : pos+4])
		size := binary.LittleEndian.Uint32(raw[pos+4 : pos+8])
		body := pos + 8
		if body+int(size) > len(raw) {
			return nil, fmt.Errorf("chunk %q overruns file (size=%d)", id, size)
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return nil, fmt.Errorf("fmt chunk too short (%d bytes)", size)
			}
			format := binary.LittleEndian.Uint16(raw[body : body+2])
			if format != 1 {
				return nil, fmt.Errorf("unsupported WAV format tag %d (only PCM=1)", format)
			}
			channels = binary.LittleEndian.Uint16(raw[body+2 : body+4])
			sampleRate = binary.LittleEndian.Uint32(raw[body+4 : body+8])
			bitsPerSample = binary.LittleEndian.Uint16(raw[body+14 : body+16])
			haveFmt = true
		case "data":
			data = raw[body : body+int(size)]
			haveData = true
		}
		// RIFF chunks are padded to an even byte count.
		pos = body + int(size)
		if size%2 == 1 {
			pos++
		}
	}

	if !haveFmt {
		return nil, fmt.Errorf("no fmt chunk")
	}
	if !haveData {
		return nil, fmt.Errorf("no data chunk")
	}
	if int(channels) != ChannelCount || sampleRate != uint32(SampleRate) || int(bitsPerSample) != BitsPerSample {
		return nil, fmt.Errorf("non-canonical format: %d ch, %d Hz, %d-bit (expected %d ch, %d Hz, %d-bit)",
			channels, sampleRate, bitsPerSample, ChannelCount, SampleRate, BitsPerSample)
	}
	return data, nil
}
