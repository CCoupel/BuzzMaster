package synth

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"buzzcontrol/internal/audio"
)

// ManifestEntry describes one cue's on-disk sound file — informational
// only (a future admin UI, #230, can use it to show a "default"/
// "personnalisé" badge per cue). Disk remains authoritative either way
// (plan de cadrage §2.2): nothing in internal/audio reads this file back,
// FileBank always reads the real .wav directly.
type ManifestEntry struct {
	Present bool `json:"present"`
	// Custom is derived, never trusted from a previous manifest: true iff
	// the on-disk bytes differ from what Generate(cue) produces right now.
	// This is only meaningful because the generator is deterministic
	// (disk.go's own doc comment) — a byte-for-byte match is exactly as
	// strong a signal as an explicit flag, without the risk of the flag
	// and the file silently drifting apart.
	Custom bool `json:"custom,omitempty"`
}

// manifestFilename is data/files/sounds/sounds.json's own basename.
const manifestFilename = "sounds.json"

// ReconcileManifest scans dir for every cue in Cues, comparing its on-disk
// bytes (if the file exists) against a freshly synthesised reference, and
// writes the result to <dir>/sounds.json — same "disque fait foi, le
// manifeste n'est qu'une surcouche" model as backgrounds.json
// (cmd/server/main.go's loadBackgrounds). Called after WriteAll, by both
// the startup path (cmd/server) and the restore endpoint
// (internal/server/http.go), so the manifest never lags behind whatever
// WriteAll just did to the directory.
func ReconcileManifest(dir string) (map[audio.Cue]ManifestEntry, error) {
	manifest := make(map[audio.Cue]ManifestEntry, len(Cues))
	for _, c := range Cues {
		path := filepath.Join(dir, Filename(c))
		data, err := os.ReadFile(path)
		if err != nil {
			manifest[c] = ManifestEntry{Present: false}
			continue
		}
		ref, _ := Generate(c) // Cues and Generate are kept in lockstep — see Cues' own doc comment
		manifest[c] = ManifestEntry{Present: true, Custom: !bytes.Equal(data, ref)}
	}

	out, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, err
	}
	if err := os.WriteFile(filepath.Join(dir, manifestFilename), out, 0644); err != nil {
		return manifest, err
	}
	return manifest, nil
}
