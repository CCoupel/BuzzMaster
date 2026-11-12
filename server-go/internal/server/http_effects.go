package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"buzzcontrol/internal/ambiancecfg"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/effects"
	"buzzcontrol/internal/lighting/hue"
)

// Light-effect catalogue endpoints (v11.7.0, #223 — contracts/light-effects.md §7.1, §7.2).

// EffectsProvider is implemented by the App (cmd/server/effects.go).
type EffectsProvider interface {
	// EffectsCatalogue returns the current catalogue snapshot (empty when unavailable).
	EffectsCatalogue() effects.Catalogue
	// EffectsReload forces an immediate reload and returns the new catalogue.
	EffectsReload() effects.Catalogue
	// EffectsDir is the user effects directory.
	EffectsDir() string
}

// EffectsStore is optionally implemented by the EffectsProvider (the App): writing, reading and
// deleting the user effect files (v11.8.0, #305 — light-effects.md §11.5 to §11.7). The errors
// are *effects.StoreError.
type EffectsStore interface {
	// EffectsSource is the stored text of an effect (a native: the embedded text) and its entry.
	EffectsSource(id string) ([]byte, effects.Entry, error)
	// EffectsDelete removes the user file of an effect (a native's override included) and reloads.
	EffectsDelete(id string) (effects.Catalogue, error)
	// EffectsImport stores each file on its own, reloads once.
	EffectsImport(files []effects.ImportFile, onConflict string) ([]effects.ImportResult, effects.Catalogue)
}

func (h *HTTPServer) effectsStore() EffectsStore {
	if h.Effects == nil {
		return nil
	}
	s, _ := h.Effects.(EffectsStore)
	return s
}

// effectExists reports whether an effect with this id exists in the catalogue and
// declares the moment (the write-time validation of §6).
func (h *HTTPServer) effectExists(moment, id string) bool {
	if h.Effects == nil {
		return false
	}
	e, ok := h.Effects.EffectsCatalogue().Get(id)
	return ok && e.Effect.HasMoment(effects.Moment(moment))
}

// EffectExists is the exported form used by the rounds validation (ROUNDS_UPDATE).
func (h *HTTPServer) EffectExists(moment, id string) bool { return h.effectExists(moment, id) }

// effectEntryJSON is one catalogue line of GET /api/lighting/effects (light-effects.md §7.1,
// revision 4): applicability is a table by zone.
type effectEntryJSON struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Moments     []string        `json:"moments"`
	Origin      string          `json:"origin"`
	File        string          `json:"file"`
	MinGroups   int             `json:"min_groups"`
	TickMs      int             `json:"tick_ms"`
	Steps       int             `json:"steps"`
	Applicable  map[string]bool `json:"applicable"`
}

type unresolvedJSON struct {
	Zone   string `json:"zone"`
	Moment string `json:"moment"`
	ID     string `json:"id"`
	Scope  string `json:"scope"`
}

// zoneGroupCounts is N for each zone: its number of non-empty groups (§2.1).
func zoneGroupCounts(lc config.LightingConfig) map[string]int {
	out := map[string]int{}
	for _, z := range config.Zones {
		out[z] = len(lc.NonEmptyGroups(z))
	}
	return out
}

// applicableTo reports whether e applies to a zone (§4.7): the zone exists and has at least
// MinGroups groups, and one step of the loop is not skipped. The room always exists.
func applicableTo(e effects.Effect, zone string, counts map[string]int) bool {
	if zone != config.ZoneAmbiance && counts[zone] == 0 {
		return false
	}
	ok, _ := e.Applicable(counts[zone])
	return ok
}

// effectsView builds the §7.1 answer. No I/O toward the bridge.
func (h *HTTPServer) effectsView() map[string]any {
	lc := config.Get().Lighting
	counts := zoneGroupCounts(lc)
	var cat effects.Catalogue
	dir := ""
	if h.Effects != nil {
		cat, dir = h.Effects.EffectsCatalogue(), h.Effects.EffectsDir()
	}
	list := make([]effectEntryJSON, 0)
	for _, en := range cat.List() {
		e := en.Effect
		moments := e.Moments
		if moments == nil {
			moments = []string{}
		}
		app := map[string]bool{}
		for _, z := range config.Zones {
			app[z] = applicableTo(e, z, counts)
		}
		list = append(list, effectEntryJSON{
			ID: e.ID, Name: e.Name, Description: e.Description, Moments: moments, Origin: en.Origin, File: en.File,
			MinGroups: e.MinGroups, TickMs: e.TickMs, Steps: len(e.Steps), Applicable: app,
		})
	}
	errs := cat.Errors()
	if errs == nil {
		errs = []effects.Error{}
	}
	unresolved := []unresolvedJSON{}
	for _, z := range config.Zones {
		for _, m := range ambiancecfg.Moments {
			id := lc.Effects[z][m]
			if id == "" {
				continue
			}
			en, ok := cat.Get(id)
			if !ok || !en.Effect.HasMoment(effects.Moment(m)) || !applicableTo(en.Effect, z, counts) {
				unresolved = append(unresolved, unresolvedJSON{Zone: z, Moment: m, ID: id, Scope: "global"})
			}
		}
	}
	if dir != "" {
		dir = filepath.ToSlash(dir)
	}
	return map[string]any{"groups": counts, "effects": list, "errors": errs, "unresolved": unresolved, "dir": dir}
}

// handleLightingEffects serves GET /api/lighting/effects.
func (h *HTTPServer) handleLightingEffects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeEffectsJSON(w, h.effectsView())
}

// handleLightingEffectsReload serves POST /api/lighting/effects/reload.
func (h *HTTPServer) handleLightingEffectsReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Effects != nil {
		h.Effects.EffectsReload()
	}
	writeEffectsJSON(w, h.effectsView())
}

func writeEffectsJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// EffectTrialer is optionally implemented by the LightingProvider (the App): the effect trial of
// light-effects.md §7.4, carried by the ambiance controller.
type EffectTrialer interface {
	// StartEffectTrial plays effect id on zone at speed for seconds seconds, replacing a running
	// trial or group test; returns the duration in ms. The error is ambiance.ErrTrialUnknownEffect
	// or a *ambiance.TrialError (see trialReason).
	StartEffectTrial(id, zone string, speed float64, seconds int) (int, error)
	// StopEffectTrial stops it; harmless when none runs.
	StopEffectTrial()
}

func (h *HTTPServer) effectTrialer() EffectTrialer {
	if h.Lighting == nil {
		return nil
	}
	t, _ := h.Lighting.(EffectTrialer)
	return t
}

// trialRefusal is implemented by the refusal errors of a trial (their 409 reason).
type trialRefusal interface{ TrialReason() string }

// handleLightingEffectsSub serves POST /api/lighting/effects/{id}/try and
// POST /api/lighting/effects/try/stop (§7.4).
func (h *HTTPServer) handleLightingEffectsSub(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/lighting/effects/")
	switch {
	case rest == "try/stop":
		h.handleEffectTrialStop(w, r)
	case rest == "import":
		h.handleEffectsImport(w, r)
	case strings.HasSuffix(rest, "/download") && !strings.Contains(strings.TrimSuffix(rest, "/download"), "/"):
		h.handleEffectDownload(w, r, strings.TrimSuffix(rest, "/download"))
	case rest != "" && !strings.Contains(rest, "/") && r.Method == http.MethodDelete:
		h.handleEffectDelete(w, r, rest)
	case strings.HasSuffix(rest, "/preview") && !strings.Contains(strings.TrimSuffix(rest, "/preview"), "/"):
		h.handleEffectPreview(w, r, strings.TrimSuffix(rest, "/preview"))
	case strings.HasSuffix(rest, "/try") && !strings.Contains(strings.TrimSuffix(rest, "/try"), "/"):
		h.handleEffectTrial(w, r, strings.TrimSuffix(rest, "/try"))
	default:
		http.NotFound(w, r)
	}
}

func (h *HTTPServer) handleEffectTrialStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if t := h.effectTrialer(); t != nil {
		t.StopEffectTrial()
	}
	writeLightingJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *HTTPServer) handleEffectTrial(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	req := struct {
		Zone    string  `json:"zone"`
		Speed   float64 `json:"speed"`
		Seconds int     `json:"seconds"`
	}{Zone: ambiancecfg.ZoneAmbiance, Speed: 1}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "corps JSON invalide", http.StatusBadRequest)
		return
	}
	if req.Zone == "" {
		req.Zone = ambiancecfg.ZoneAmbiance
	}
	if req.Speed == 0 {
		req.Speed = 1
	}
	if !ambiancecfg.ValidZone(req.Zone) || !ambiancecfg.ValidSpeed(req.Speed) || req.Seconds < 0 || req.Seconds > 20 {
		http.Error(w, "zone (ambiance, animateur, public), speed (0,5 ; 0,75 ; 1 ; 1,5 ; 2) ou seconds (1 à 20) invalide", http.StatusBadRequest)
		return
	}
	if h.Effects == nil {
		http.Error(w, "effet inconnu", http.StatusNotFound)
		return
	}
	en, ok := h.Effects.EffectsCatalogue().Get(id)
	if !ok {
		http.Error(w, "effet inconnu", http.StatusNotFound)
		return
	}
	if req.Seconds == 0 { // default: 5 s for an effect that only declares the score moment, else 10 s
		req.Seconds = trialDefaultSeconds(en.Effect)
	}
	d, t := h.lightingDriver(), h.effectTrialer()
	if d == nil || t == nil {
		writeLightingJSON(w, http.StatusConflict, map[string]string{"result": "refused", "reason": "disabled"})
		return
	}
	switch d.Status().State {
	case hue.StateRefused:
		writeLightingJSON(w, http.StatusConflict, map[string]string{"result": "refused", "reason": "disabled"})
		return
	case hue.StateUnreachable:
		writeLightingJSON(w, http.StatusServiceUnavailable, map[string]string{"result": "unreachable"})
		return
	}
	ms, err := t.StartEffectTrial(id, req.Zone, req.Speed, req.Seconds)
	if err != nil {
		var tr trialRefusal
		switch {
		case errors.As(err, &tr):
			writeLightingJSON(w, http.StatusConflict, map[string]string{"result": "refused", "reason": tr.TrialReason()})
		default:
			http.Error(w, err.Error(), http.StatusNotFound)
		}
		return
	}
	writeLightingJSON(w, http.StatusOK, map[string]any{"ok": true, "duration_ms": ms})
}

// handleEffectPreview serves GET /api/lighting/effects/{id}/preview?groups=N&moment=chrono
// (light-effects.md §7.3): the frames computed by effects.Preview, nothing is played. groups: 1 to
// 12 (default: the room's configured groups, 6 when there is none); moment: one of the effect's
// moments (default: its first). 404 unknown effect, 400 invalid parameter.
func (h *HTTPServer) handleEffectPreview(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Effects == nil {
		http.Error(w, "effet inconnu", http.StatusNotFound)
		return
	}
	en, ok := h.Effects.EffectsCatalogue().Get(id)
	if !ok {
		http.Error(w, "effet inconnu", http.StatusNotFound)
		return
	}
	n := len(config.Get().Lighting.NonEmptyGroups(config.ZoneAmbiance))
	if n == 0 {
		n = 6
	}
	if v := r.URL.Query().Get("groups"); v != "" {
		g, err := strconv.Atoi(v)
		if err != nil || g < 1 || g > effects.MaxGroups {
			http.Error(w, "groups : entier de 1 à 12", http.StatusBadRequest)
			return
		}
		n = g
	}
	moment := effects.Moment("")
	if len(en.Effect.Moments) > 0 {
		moment = effects.Moment(en.Effect.Moments[0])
	}
	if v := r.URL.Query().Get("moment"); v != "" {
		if !effects.Moment(v).Valid() || !en.Effect.HasMoment(effects.Moment(v)) {
			http.Error(w, "moment : un moment déclaré par l'effet (chrono, calm ou score)", http.StatusBadRequest)
			return
		}
		moment = effects.Moment(v)
	}
	writeEffectsJSON(w, effects.Preview(en.Effect, n, moment))
}

// trialDefaultSeconds is the default length of a trial (§7.4): 5 s for an effect that declares
// only the score moment, 10 s otherwise.
func trialDefaultSeconds(e effects.Effect) int {
	if len(e.Moments) == 1 && e.HasMoment(effects.MomentScore) {
		return 5
	}
	return 10
}

// Import limits (§11.7).
const (
	importMaxFiles = 20
	importMaxTotal = 2 << 20
)

// handleEffectDownload serves GET /api/lighting/effects/{id}/download (§11.6).
func (h *HTTPServer) handleEffectDownload(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := h.effectsStore()
	if st == nil {
		http.Error(w, "effet inconnu", http.StatusNotFound)
		return
	}
	data, _, err := st.EffectsSource(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	// id is validated by the loader ([a-z0-9-]{1,40}): safe in a header.
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.yaml"`)
	_, _ = w.Write(data)
}

// handleEffectDelete serves DELETE /api/lighting/effects/{id} (§11.5).
func (h *HTTPServer) handleEffectDelete(w http.ResponseWriter, _ *http.Request, id string) {
	st := h.effectsStore()
	if st == nil {
		http.Error(w, "effet inconnu", http.StatusNotFound)
		return
	}
	if _, err := st.EffectsDelete(id); err != nil {
		writeStoreError(w, err)
		return
	}
	writeEffectsJSON(w, map[string]any{"ok": true, "catalogue": h.effectsView()})
}

// writeStoreError maps a *effects.StoreError to its HTTP answer (§11.4, §11.5).
func writeStoreError(w http.ResponseWriter, err error) {
	var se *effects.StoreError
	if !errors.As(err, &se) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch se.Code {
	case effects.ReasonNotFound:
		http.Error(w, "effet inconnu", http.StatusNotFound)
	case effects.ReasonNative, effects.ReasonExists, effects.ReasonLimit:
		writeLightingJSON(w, http.StatusConflict, map[string]string{"reason": se.Code})
	case effects.ReasonReserved:
		http.Error(w, "identifiant réservé", http.StatusBadRequest)
	case effects.ReasonTooBig:
		http.Error(w, "fichier trop gros", http.StatusRequestEntityTooLarge)
	case effects.ReasonInvalid:
		type jsonErr struct {
			Line    int    `json:"line"`
			Message string `json:"message"`
		}
		list := make([]jsonErr, 0, len(se.Errors))
		for _, e := range se.Errors {
			list = append(list, jsonErr{Line: e.Line, Message: e.Message})
		}
		writeLightingJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "errors": list})
	default:
		http.Error(w, "écriture impossible", http.StatusInternalServerError)
	}
}

// handleEffectsImport serves POST /api/lighting/effects/import (§11.7): multipart, field
// `files` (1 to 20 `.yaml` / `.yml`, 64 KB each, 2 MB in all), optional `on_conflict`.
// The name of an uploaded file only labels the messages: it is never used as a path.
func (h *HTTPServer) handleEffectsImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := h.effectsStore()
	if st == nil {
		http.Error(w, "import indisponible", http.StatusServiceUnavailable)
		return
	}
	// 2 MB of files plus the multipart framing; anything beyond is refused whatever the parts.
	r.Body = http.MaxBytesReader(w, r.Body, importMaxTotal+(1<<20))
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "envoi multipart attendu", http.StatusBadRequest)
		return
	}
	var files []effects.ImportFile
	onConflict, total := effects.ConflictAsk, 0
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeImportReadError(w, err)
			return
		}
		switch part.FormName() {
		case "on_conflict":
			b, err := io.ReadAll(io.LimitReader(part, 32))
			if err != nil {
				writeImportReadError(w, err)
				return
			}
			switch v := strings.TrimSpace(string(b)); v {
			case effects.ConflictAsk, effects.ConflictReplace, effects.ConflictKeepBoth:
				onConflict = v
			default:
				http.Error(w, "on_conflict : ask, replace ou keep_both", http.StatusBadRequest)
				return
			}
		case "files":
			name := part.FileName()
			if name == "" {
				http.Error(w, "le champ files doit contenir des fichiers", http.StatusBadRequest)
				return
			}
			if len(files) >= importMaxFiles {
				http.Error(w, "20 fichiers au plus par envoi", http.StatusBadRequest)
				return
			}
			if ext := strings.ToLower(filepath.Ext(name)); ext != ".yaml" && ext != ".yml" {
				http.Error(w, "extension refusée (.yaml ou .yml) : "+name, http.StatusBadRequest)
				return
			}
			// One byte past the limit is enough to flag a too-big file; the rest is counted, not kept.
			data, err := io.ReadAll(io.LimitReader(part, effects.MaxFileBytes+1))
			if err != nil {
				writeImportReadError(w, err)
				return
			}
			rest, err := io.Copy(io.Discard, part)
			if err != nil {
				writeImportReadError(w, err)
				return
			}
			if total += len(data) + int(rest); total > importMaxTotal {
				http.Error(w, "plus de 2 Mo envoyés", http.StatusRequestEntityTooLarge)
				return
			}
			files = append(files, effects.ImportFile{Name: name, Data: data})
		}
	}
	if len(files) == 0 {
		http.Error(w, "aucun fichier (champ files)", http.StatusBadRequest)
		return
	}
	results, _ := st.EffectsImport(files, onConflict)
	if results == nil {
		results = []effects.ImportResult{}
	}
	writeEffectsJSON(w, map[string]any{"results": results, "catalogue": h.effectsView()})
}

func writeImportReadError(w http.ResponseWriter, err error) {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		http.Error(w, "plus de 2 Mo envoyés", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "envoi illisible", http.StatusBadRequest)
}
