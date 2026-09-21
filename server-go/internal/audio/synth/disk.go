package synth

import (
	"fmt"
	"os"
	"path/filepath"

	"buzzcontrol/internal/audio"
)

// Filename is the canonical on-disk name for a cue's sound file — the
// single source of truth reused by the generator (this package), the
// bank (audio.FileBank, bank.go) and the manifest reconciliation
// (cmd/server), so the three can never drift apart on naming.
func Filename(c audio.Cue) string {
	return string(c) + ".wav"
}

// WriteAll writes every cue in Cues into dir as its canonical WAV file.
//
//   - overwrite == false (ordinary startup, plan de dev #229 §2.4/B.4):
//     an existing file — default OR a user's own customised sound
//     (#230) — is left untouched. Only a missing file is written.
//   - overwrite == true ("restore the delivered sounds", B.5, modelled on
//     FirmwareManager.RestoreEmbedded): every file is (re)written
//     unconditionally. Idempotent by construction — the generator is
//     deterministic, so a restore always produces the exact same bytes,
//     regardless of what was there before.
//
// Returns the cues actually written (for logging), and the first error
// encountered — WriteAll does not stop early on one file's failure, since
// a demo-question sound being unwritable should never prevent the other
// six from being tried (same "keep going" discipline as
// createDemoBackgrounds's per-file continue).
func WriteAll(dir string, overwrite bool) (written []audio.Cue, err error) {
	if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, mkErr)
	}
	var firstErr error
	for _, c := range Cues {
		path := filepath.Join(dir, Filename(c))
		if !overwrite {
			if _, statErr := os.Stat(path); statErr == nil {
				continue // already present — never clobber (default or custom)
			}
		}
		data, ok := Generate(c)
		if !ok {
			continue // unreachable in practice: Cues and Generate are kept in lockstep
		}
		if writeErr := os.WriteFile(path, data, 0644); writeErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("writing %s: %w", path, writeErr)
			}
			continue
		}
		written = append(written, c)
	}
	return written, firstErr
}
