package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"buzzcontrol/internal/audio/synth"
	"buzzcontrol/internal/game"
)

// handleAPISoundsRestoreDefaults handles POST /api/sounds/restore-defaults
// (#229) — regenerates every cue's canonical WAV file into
// data/files/sounds/, UNCONDITIONALLY overwriting whatever is there,
// default or a user's own customisation. Modelled on
// handleAPIFirmwareRestoreEmbedded (RestoreEmbedded) — same "explicit
// action, always overwrites" contract, spelled out here because it is the
// opposite of what ordinary startup does (createDefaultSounds,
// cmd/server/sound.go, never touches an existing file).
//
// Idempotent by construction: synth's generator is deterministic
// (internal/audio/synth/disk.go's own doc comment), so calling this twice
// in a row produces byte-for-byte identical files both times.
func (h *HTTPServer) handleAPISoundsRestoreDefaults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dir := filepath.Join(h.dataDir, "files", "sounds")
	written, err := synth.WriteAll(dir, true)
	if err != nil {
		LogError(game.LogComponentHTTP, "Sounds restore-defaults failed: %v", err)
		http.Error(w, `{"status":"error","message":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if _, mErr := synth.ReconcileManifest(dir); mErr != nil {
		LogWarn(game.LogComponentHTTP, "Sounds restore-defaults: failed to write sounds.json manifest: %v", mErr)
	}

	LogInfo(game.LogComponentHTTP, "Sounds restore-defaults: %d cue(s) rewritten to their default", len(written))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"written": written,
	})
}
