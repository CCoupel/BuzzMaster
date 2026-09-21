package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/audio/synth"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
)

// SoundProvider gives the /api/sounds/{cue}/test and /api/sound/status
// handlers access to the live sound engine (#230, contracts/sound.md §6.3,
// contracts/http-endpoints.md §Sound). Implemented by *App
// (cmd/server/sound.go) — modelled on LightingProvider
// (http_lighting.go:24-45).
type SoundProvider interface {
	// SoundEnabled reads `sound.enabled` directly from configuration,
	// WITHOUT touching the engine — the contract requires "disabled"
	// to be decided before any access to the moteur.
	SoundEnabled() bool
	// SoundOutputAvailable reports whether a REAL (non-neutral) Output is
	// attached — distinguishes "unavailable" from "disabled". Well-defined
	// regardless of SoundEnabled()'s value.
	SoundOutputAvailable() bool
	// TestSoundCue calls the engine DIRECTLY (audio.Engine.PlayCue), never
	// through the game's notifySound fan-out — a cue individually disabled
	// via CuesDisabled must stay testable (contract §6.3). c is assumed
	// already validated against the closed catalogue by the caller.
	TestSoundCue(c audio.Cue) (accepted bool)
}

// soundsDir is data/files/sounds/ — the same location cmd/server's own
// soundsDir(cfg) resolves to (filesDir/"sounds"), computed independently
// here since internal/server already anchors every other media directory
// on h.dataDir (backgrounds, categories, entracte) rather than importing
// cmd/server.
func (h *HTTPServer) soundsDir() string {
	return filepath.Join(h.dataDir, "files", "sounds")
}

// soundCueFromString validates a URL path segment against the closed v11.0
// catalogue (synth.Cues) — contracts/http-endpoints.md §Sound "Sécurité" :
// the segment comes from the URL and must never be used to build a file
// path before this check. Same discipline as the SSRF guard of #206.
func soundCueFromString(s string) (audio.Cue, bool) {
	c := audio.Cue(s)
	for _, known := range synth.Cues {
		if known == c {
			return c, true
		}
	}
	return "", false
}

// writeSoundJSON writes a JSON body via the encoder (never raw string
// concatenation, contracts/http-endpoints.md §Sound) so no error message
// or field value can ever produce malformed JSON.
func writeSoundJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeSoundError(w http.ResponseWriter, status int, message string) {
	writeSoundJSON(w, status, map[string]string{"status": "error", "message": message})
}

// ---------------------------------------------------------------------------
// GET /api/sounds — the seven cues, with origin and duration (B.3)
// ---------------------------------------------------------------------------

type soundCueEntry struct {
	Cue             string  `json:"cue"`
	Enabled         bool    `json:"enabled"`
	Custom          bool    `json:"custom"`
	DurationSeconds float64 `json:"duration_seconds"`
	Path            string  `json:"path"`
}

// handleAPISoundsList handles GET /api/sounds (#230, B.3) — the seven
// cues in catalogue order (synth.Cues), origin from the reconciled
// manifest (#229 — a binary comparison against a fresh synthesis, nothing
// extra to maintain), duration computed from each file's own header.
func (h *HTTPServer) handleAPISoundsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dir := h.soundsDir()
	manifest, err := synth.ReconcileManifest(dir)
	if err != nil {
		LogWarn(game.LogComponentHTTP, "GET /api/sounds: failed to reconcile manifest: %v", err)
	}
	cuesDisabled := config.Get().Sound.CuesDisabled

	entries := make([]soundCueEntry, 0, len(synth.Cues))
	for _, c := range synth.Cues {
		entry := soundCueEntry{
			Cue:     string(c),
			Enabled: !cuesDisabled[string(c)],
			Path:    "/files/sounds/" + synth.Filename(c),
		}
		m := manifest[c]
		entry.Custom = m.Custom
		if m.Present {
			if raw, readErr := os.ReadFile(filepath.Join(dir, synth.Filename(c))); readErr == nil {
				if res, valErr := audio.ValidateUpload(raw); valErr == nil {
					entry.DurationSeconds = res.Duration.Seconds()
				}
			}
		}
		entries = append(entries, entry)
	}

	writeSoundJSON(w, http.StatusOK, map[string]interface{}{"cues": entries})
}

// ---------------------------------------------------------------------------
// /api/sounds/{cue}[/restore|/test] router (B.4, B.5, B.6)
// ---------------------------------------------------------------------------

// handleAPISoundsRouter dispatches the dynamic /api/sounds/{cue}... routes
// — same manual-parsing convention as handleRafaleQuestionByID/
// handleAPIBuzzerRouter (this codebase predates Go 1.22's pattern mux).
// /api/sounds/restore-defaults (#229) is registered as its own exact
// route in registerRoutes and always wins over this prefix handler
// (http.ServeMux picks the longest matching pattern).
func (h *HTTPServer) handleAPISoundsRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/sounds/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	if cueStr, ok := strings.CutSuffix(path, "/restore"); ok {
		h.handleAPISoundRestoreOne(w, r, cueStr)
		return
	}
	if cueStr, ok := strings.CutSuffix(path, "/test"); ok {
		h.handleAPISoundTest(w, r, cueStr)
		return
	}
	h.handleAPISoundReplace(w, r, path)
}

// handleAPISoundReplace handles POST /api/sounds/{cue} (#230, B.4) —
// multipart/form-data, field `file`. Validation: extension, size cap,
// then full format+duration via audio.ValidateUpload (built on
// extractCanonicalPCM, bank.go — never a second validator).
func (h *HTTPServer) handleAPISoundReplace(w http.ResponseWriter, r *http.Request, cueStr string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cue, ok := soundCueFromString(cueStr)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseMultipartForm(audio.MaxUploadBytes + (1 << 20)); err != nil {
		writeSoundError(w, http.StatusBadRequest, "corps de requête invalide")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeSoundError(w, http.StatusBadRequest, "aucun fichier envoyé (champ \"file\" attendu)")
		return
	}
	defer file.Close()

	if !strings.EqualFold(filepath.Ext(header.Filename), ".wav") {
		writeSoundError(w, http.StatusBadRequest, "ce fichier n'est pas un WAV — seuls les fichiers .wav sont acceptés")
		return
	}
	if header.Size > audio.MaxUploadBytes {
		writeSoundError(w, http.StatusRequestEntityTooLarge, "fichier trop volumineux")
		return
	}

	raw, err := io.ReadAll(io.LimitReader(file, audio.MaxUploadBytes+1))
	if err != nil {
		writeSoundError(w, http.StatusBadRequest, "lecture du fichier échouée")
		return
	}
	if len(raw) > audio.MaxUploadBytes {
		writeSoundError(w, http.StatusRequestEntityTooLarge, "fichier trop volumineux")
		return
	}

	result, err := audio.ValidateUpload(raw)
	if err != nil {
		writeSoundError(w, http.StatusBadRequest, err.Error())
		return
	}

	dir := h.soundsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		LogError(game.LogComponentHTTP, "Sound replace: cannot create %s: %v", dir, err)
		writeSoundError(w, http.StatusInternalServerError, "échec d'écriture disque")
		return
	}
	path := filepath.Join(dir, synth.Filename(cue))
	if err := os.WriteFile(path, raw, 0644); err != nil {
		LogError(game.LogComponentHTTP, "Sound replace: cannot write %s: %v", path, err)
		writeSoundError(w, http.StatusInternalServerError, "échec d'écriture disque")
		return
	}

	manifest, mErr := synth.ReconcileManifest(dir)
	if mErr != nil {
		LogWarn(game.LogComponentHTTP, "Sound replace: failed to write sounds.json manifest: %v", mErr)
	}
	LogInfo(game.LogComponentHTTP, "Sound replace: cue=%s custom=%v duration=%.2fs", cue, manifest[cue].Custom, result.Duration.Seconds())

	writeSoundJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "ok",
		"cue":              string(cue),
		"custom":           manifest[cue].Custom,
		"duration_seconds": result.Duration.Seconds(),
		"warning":          nullableString(result.Warning),
	})
}

// handleAPISoundRestoreOne handles POST /api/sounds/{cue}/restore (#230,
// B.5) — regenerates ONLY this cue, unconditionally, idempotent (the
// generator is deterministic — internal/audio/synth's own doc comment).
func (h *HTTPServer) handleAPISoundRestoreOne(w http.ResponseWriter, r *http.Request, cueStr string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cue, ok := soundCueFromString(cueStr)
	if !ok {
		http.NotFound(w, r)
		return
	}

	dir := h.soundsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		LogError(game.LogComponentHTTP, "Sound restore: cannot create %s: %v", dir, err)
		writeSoundError(w, http.StatusInternalServerError, "échec d'écriture disque")
		return
	}
	data, ok := synth.Generate(cue)
	if !ok {
		// Unreachable in practice: cue was just validated against synth.Cues,
		// and Generate/Cues are kept in lockstep (synth.Cues' own doc comment).
		LogError(game.LogComponentHTTP, "Sound restore: synth.Generate(%s) returned ok=false for a catalogued cue", cue)
		writeSoundError(w, http.StatusInternalServerError, "échec de génération")
		return
	}
	path := filepath.Join(dir, synth.Filename(cue))
	if err := os.WriteFile(path, data, 0644); err != nil {
		LogError(game.LogComponentHTTP, "Sound restore: cannot write %s: %v", path, err)
		writeSoundError(w, http.StatusInternalServerError, "échec d'écriture disque")
		return
	}

	manifest, mErr := synth.ReconcileManifest(dir)
	if mErr != nil {
		LogWarn(game.LogComponentHTTP, "Sound restore: failed to write sounds.json manifest: %v", mErr)
	}
	result, _ := audio.ValidateUpload(data) // always valid — data is our own fresh synthesis
	LogInfo(game.LogComponentHTTP, "Sound restore: cue=%s restored to default", cue)

	writeSoundJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "ok",
		"cue":              string(cue),
		"custom":           manifest[cue].Custom,
		"duration_seconds": result.Duration.Seconds(),
	})
}

// handleAPISoundTest handles POST /api/sounds/{cue}/test (#230, B.6) —
// plays the cue on the SERVER's speaker via the engine directly, never
// through notifySound (contract §6.3: a disabled cue stays testable).
// Three distinct outcomes, all HTTP 200 — none of the three is an HTTP
// error (contracts/http-endpoints.md §Sound).
//
// `result` (`played` included) NEVER asserts that a sound was actually
// HEARD — only that it was handed to the engine for the speaker. Play
// returns nil whether or not a speaker is present (contracts/sound.md
// §4). The #230 interface must never derive its manual listening verdict
// from this value — see the contract's own cross-reference under this
// endpoint.
func (h *HTTPServer) handleAPISoundTest(w http.ResponseWriter, r *http.Request, cueStr string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cue, ok := soundCueFromString(cueStr)
	if !ok {
		http.NotFound(w, r)
		return
	}

	result := "unavailable"
	switch {
	case h.Sound == nil || !h.Sound.SoundEnabled():
		result = "disabled"
	case !h.Sound.SoundOutputAvailable():
		result = "unavailable"
	case h.Sound.TestSoundCue(cue):
		result = "played"
	default:
		// Residual case documented in the contract: the queue was
		// saturated (exceedingly unlikely for a single explicit test
		// click) — same response as "unavailable", no sound left.
		result = "unavailable"
	}

	writeSoundJSON(w, http.StatusOK, map[string]string{"result": result})
}

// ---------------------------------------------------------------------------
// GET /api/sound/status (B.7)
// ---------------------------------------------------------------------------

// handleAPISoundStatus handles GET /api/sound/status (#230, B.7) — TWO
// states only, deliberately merged (contracts/http-endpoints.md §Sound):
// "disabled" and "unavailable" both read as `active: false`.
func (h *HTTPServer) handleAPISoundStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	active := h.Sound != nil && h.Sound.SoundEnabled() && h.Sound.SoundOutputAvailable()
	writeSoundJSON(w, http.StatusOK, map[string]bool{"active": active})
}

// nullableString returns nil for an empty string, s otherwise — so a JSON
// field is always present, `null` rather than an omitted key
// (contracts/http-endpoints.md §Sound: "jamais un champ absent").
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ---------------------------------------------------------------------------
// POST /api/sounds/restore-defaults (#229 — unchanged, kept here for
// proximity to the rest of the Sound surface)
// ---------------------------------------------------------------------------

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

	dir := h.soundsDir()
	written, err := synth.WriteAll(dir, true)
	if err != nil {
		LogError(game.LogComponentHTTP, "Sounds restore-defaults failed: %v", err)
		writeSoundError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, mErr := synth.ReconcileManifest(dir); mErr != nil {
		LogWarn(game.LogComponentHTTP, "Sounds restore-defaults: failed to write sounds.json manifest: %v", mErr)
	}

	LogInfo(game.LogComponentHTTP, "Sounds restore-defaults: %d cue(s) rewritten to their default", len(written))

	writeSoundJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"written": written,
	})
}
