package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
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
			MinGroups: e.MinGroups, TickMs: e.TickMs, Applicable: app,
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
	}{Zone: ambiancecfg.ZoneAmbiance, Speed: 1, Seconds: 10}
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
	if req.Seconds == 0 {
		req.Seconds = 10
	}
	if !ambiancecfg.ValidZone(req.Zone) || !ambiancecfg.ValidSpeed(req.Speed) || req.Seconds < 1 || req.Seconds > 20 {
		http.Error(w, "zone (ambiance, animateur, public), speed (0,5 ; 0,75 ; 1 ; 1,5 ; 2) ou seconds (1 à 20) invalide", http.StatusBadRequest)
		return
	}
	if h.Effects == nil {
		http.Error(w, "effet inconnu", http.StatusNotFound)
		return
	}
	if _, ok := h.Effects.EffectsCatalogue().Get(id); !ok {
		http.Error(w, "effet inconnu", http.StatusNotFound)
		return
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
