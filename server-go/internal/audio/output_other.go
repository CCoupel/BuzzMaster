//go:build !linux && !windows

package audio

import "log"

// newPlatformOutput on any GOOS other than linux/windows (#228's targets,
// contract §1.1 — matching the CI build matrix and the milestone's actual
// deployment platforms) returns a harmless no-op Output. This file
// deliberately imports NOTHING beyond the standard library — no `oto` —
// so internal/audio stays compilable on any platform Go itself supports,
// even one `oto` does not, without ever needing to know which those are.
func newPlatformOutput(cfg OutputConfig) Output {
	log.Printf("audio: no sound backend built for this platform — sound bruitage is a no-op here (contract sound.md §1.1: Windows + Linux/Raspberry Pi only)")
	return noopOutput{}
}
