package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"buzzcontrol/internal/ambiancecfg"
	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/game"
)

// Per-round sound files (v11.6.0, #290 révision 3 — contracts/ambiance-settings.md §9,
// http-endpoints.md « Rounds sounds »). Stored under data/files/sounds/rounds/<roundID>/<key>.wav
// (fixed name, atomic write), referenced by Round.AMBIANCE.sound.files in rounds.json, which is
// THE truth (the existence of the file is never trusted nor reconciled — D12).
//
//	GET    /api/rounds/{id}/sounds
//	POST   /api/rounds/{id}/sounds/{cue}         (multipart `file`)   — immediate (D10)
//	DELETE /api/rounds/{id}/sounds/{cue}                              — back to the catalogue file
//	POST   /api/rounds/{id}/sounds/{cue}/test                         — the EFFECTIVE file, plays once

var roundIDPattern = regexp.MustCompile(`^r[0-9]+$`)

// roundSoundKey validates {cue}: the closed cue catalogue ∪ "decompte" — before any path join.
func roundSoundKey(s string) (string, bool) {
	if s == ambiancecfg.DecompteKey {
		return s, true
	}
	if c, ok := audio.CueByName(s); ok {
		return string(c), true
	}
	return "", false
}

func (h *HTTPServer) roundSoundsQuestionsDir() string {
	return filepath.Join(h.dataDir, "files", "questions")
}

// handleAPIRoundsRouter dispatches /api/rounds/{id}/sounds[/{cue}[/test]].
func (h *HTTPServer) handleAPIRoundsRouter(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/rounds/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) < 2 || parts[1] != "sounds" || len(parts) > 4 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if !roundIDPattern.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	switch {
	case len(parts) == 2:
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.handleRoundSoundsList(w, id)
	case len(parts) == 3:
		key, ok := roundSoundKey(parts[2])
		if !ok {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPost:
			h.handleRoundSoundUpload(w, r, id, key)
		case http.MethodDelete:
			h.handleRoundSoundDelete(w, id, key)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case len(parts) == 4 && parts[3] == "test":
		key, ok := roundSoundKey(parts[2])
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.handleRoundSoundTest(w, id, key)
	default:
		http.NotFound(w, r)
	}
}

// findRound loads rounds.json and returns the round id (and the whole file).
func (h *HTTPServer) findRound(id string) (RoundsFile, int, bool) {
	rf := LoadRoundsFile(h.roundSoundsQuestionsDir())
	for i, r := range rf.Rounds {
		if r.ID == id {
			return rf, i, true
		}
	}
	return rf, -1, false
}

func (h *HTTPServer) roundSoundsDir(id string) string {
	return filepath.Join(h.soundsDir(), "rounds", id)
}

// roundFilePath is the absolute path of rel (relative to the sounds directory) when it stays
// inside it, "" otherwise.
func (h *HTTPServer) roundFilePath(rel string) string {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if rel == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.Join(h.soundsDir(), clean)
}

// orderedKeys: the 15 cues in normative order, then "decompte".
func orderedKeys() []string {
	keys := make([]string, 0, len(audio.AllCues())+1)
	for _, c := range audio.AllCues() {
		keys = append(keys, string(c))
	}
	return append(keys, ambiancecfg.DecompteKey)
}

func (h *HTTPServer) durationOf(key, path string) interface{} {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if key == ambiancecfg.DecompteKey {
		if res, verr := audio.ValidateQuestionSound(raw); verr == nil {
			return res.Duration.Seconds()
		}
		return nil
	}
	if res, verr := audio.ValidateUpload(raw); verr == nil {
		return res.Duration.Seconds()
	}
	return nil
}

func (h *HTTPServer) handleRoundSoundsList(w http.ResponseWriter, id string) {
	rf, i, ok := h.findRound(id)
	if !ok {
		writeSoundError(w, http.StatusNotFound, "manche inconnue")
		return
	}
	files := []map[string]interface{}{}
	for _, key := range orderedKeys() {
		rel, has := rf.Rounds[i].Ambiance.SoundFiles[key]
		if !has {
			continue
		}
		entry := map[string]interface{}{"cue": key, "path": rel, "present": false, "duration_seconds": nil}
		if p := h.roundFilePath(rel); p != "" {
			if _, err := os.Stat(p); err == nil {
				entry["present"] = true
				entry["duration_seconds"] = h.durationOf(key, p)
			}
		}
		files = append(files, entry)
	}
	writeSoundJSON(w, http.StatusOK, map[string]interface{}{"round_id": id, "files": files})
}

func (h *HTTPServer) handleRoundSoundUpload(w http.ResponseWriter, r *http.Request, id, key string) {
	if _, _, ok := h.findRound(id); !ok {
		writeSoundError(w, http.StatusNotFound, "manche inconnue")
		return
	}
	rawMax := int64(audio.MaxRawUploadBytes)
	if key == ambiancecfg.DecompteKey {
		rawMax = audio.MaxRawQuestionSoundBytes
	}
	if err := r.ParseMultipartForm(rawMax + (1 << 20)); err != nil {
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
	if header.Size > rawMax {
		writeSoundError(w, http.StatusRequestEntityTooLarge, "fichier trop volumineux")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(file, rawMax+1))
	if err != nil {
		writeSoundError(w, http.StatusBadRequest, "lecture du fichier échouée")
		return
	}
	if int64(len(raw)) > rawMax {
		writeSoundError(w, http.StatusRequestEntityTooLarge, "fichier trop volumineux")
		return
	}

	// Validation + conversion IDENTICAL to the catalogue (C5): never a second validator.
	var (
		pcmWAV                []byte
		duration              float64
		converted             bool
		sourceFormat, warning string
	)
	if key == ambiancecfg.DecompteKey {
		res, verr := audio.ValidateQuestionSound(raw)
		if verr != nil {
			writeSoundValidationError(w, verr)
			return
		}
		pcmWAV, duration, converted, sourceFormat = audio.BuildCanonicalWAV(res.PCM), res.Duration.Seconds(), res.Converted, res.SourceFormat
	} else {
		res, verr := audio.ValidateUpload(raw)
		if verr != nil {
			writeSoundValidationError(w, verr)
			return
		}
		pcmWAV, duration, converted, sourceFormat, warning = audio.BuildCanonicalWAV(res.PCM), res.Duration.Seconds(), res.Converted, res.SourceFormat, res.Warning
	}

	rel := ambiancecfg.RoundFilePath(id, key)
	// Same lock as ROUNDS_UPDATE (the file and rounds.json move together).
	QuestionFilesMu.Lock()
	rf, i, ok := h.findRound(id)
	if !ok {
		QuestionFilesMu.Unlock()
		writeSoundError(w, http.StatusNotFound, "manche inconnue")
		return
	}
	dir := h.roundSoundsDir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		QuestionFilesMu.Unlock()
		LogError(game.LogComponentHTTP, "Round sound: cannot create %s: %v", dir, err)
		writeSoundError(w, http.StatusInternalServerError, "échec d'écriture disque")
		return
	}
	dst := filepath.Join(dir, key+".wav")
	_, hadFile := rf.Rounds[i].Ambiance.SoundFiles[key]
	if err := writeFileAtomic(dst, pcmWAV); err != nil {
		QuestionFilesMu.Unlock()
		LogError(game.LogComponentHTTP, "Round sound: cannot write %s: %v", dst, err)
		writeSoundError(w, http.StatusInternalServerError, "échec d'écriture disque")
		return
	}
	amb := rf.Rounds[i].Ambiance.Clone()
	if amb.SoundFiles == nil {
		amb.SoundFiles = map[string]string{}
	}
	amb.SoundFiles[key] = rel
	rf.Rounds[i].Ambiance = amb
	if err := SaveRoundsFile(h.roundSoundsQuestionsDir(), rf); err != nil {
		if !hadFile {
			_ = os.Remove(dst)
		}
		QuestionFilesMu.Unlock()
		LogError(game.LogComponentHTTP, "Round sound: cannot write rounds.json: %v", err)
		writeSoundError(w, http.StatusInternalServerError, "échec d'écriture disque")
		return
	}
	QuestionFilesMu.Unlock()

	LogInfo(game.LogComponentHTTP, "Round sound: round=%s key=%s duration=%.2fs converted=%v", id, key, duration, converted)
	if h.Ambiance != nil {
		h.Ambiance.AmbianceRoundsChanged()
	}
	writeSoundJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok", "round_id": id, "cue": key, "path": rel,
		"duration_seconds": duration, "converted": converted,
		"source_format": nullableString(sourceFormat), "warning": nullableString(warning),
	})
}

func (h *HTTPServer) handleRoundSoundDelete(w http.ResponseWriter, id, key string) {
	QuestionFilesMu.Lock()
	rf, i, ok := h.findRound(id)
	if !ok {
		QuestionFilesMu.Unlock()
		writeSoundError(w, http.StatusNotFound, "manche inconnue")
		return
	}
	rel, has := rf.Rounds[i].Ambiance.SoundFiles[key]
	if !has {
		QuestionFilesMu.Unlock()
		writeSoundError(w, http.StatusNotFound, "cette manche n'a pas de fichier pour ce son")
		return
	}
	amb := rf.Rounds[i].Ambiance.Clone()
	delete(amb.SoundFiles, key)
	rf.Rounds[i].Ambiance = amb
	if err := SaveRoundsFile(h.roundSoundsQuestionsDir(), rf); err != nil {
		QuestionFilesMu.Unlock()
		LogError(game.LogComponentHTTP, "Round sound: cannot write rounds.json: %v", err)
		writeSoundError(w, http.StatusInternalServerError, "échec d'écriture disque")
		return
	}
	// The file goes AFTER rounds.json was written (never a reference to a deleted file kept).
	if p := h.roundFilePath(rel); p != "" {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			LogWarn(game.LogComponentHTTP, "Round sound: cannot remove %s: %v", p, err)
		}
	}
	QuestionFilesMu.Unlock()
	if h.Ambiance != nil {
		h.Ambiance.AmbianceRoundsChanged()
	}
	writeSoundJSON(w, http.StatusOK, map[string]interface{}{"status": "ok"})
}

func (h *HTTPServer) handleRoundSoundTest(w http.ResponseWriter, id, key string) {
	rf, i, ok := h.findRound(id)
	if !ok {
		writeSoundError(w, http.StatusNotFound, "manche inconnue")
		return
	}
	played, rel := "global", key+".wav"
	if r, has := rf.Rounds[i].Ambiance.SoundFiles[key]; has {
		if p := h.roundFilePath(r); p != "" {
			if _, err := os.Stat(p); err == nil {
				played, rel = "round", r
			}
		}
	}
	if h.Ambiance == nil || !h.Ambiance.AmbianceTestFile(key, rel) {
		writeSoundJSON(w, http.StatusOK, map[string]interface{}{"status": "unavailable", "played": played})
		return
	}
	writeSoundJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "played": played})
}
