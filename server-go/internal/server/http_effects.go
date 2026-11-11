package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"buzzcontrol/internal/ambiancecfg"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/effects"
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

// effectEntryJSON is one catalogue line of GET /api/lighting/effects.
type effectEntryJSON struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Moments        []string `json:"moments"`
	Origin         string   `json:"origin"`
	File           string   `json:"file"`
	MinGroups      int      `json:"min_groups"`
	Applicable     bool     `json:"applicable"`
	Reason         string   `json:"reason"`
	HostApplicable bool     `json:"host_applicable"`
}

type unresolvedJSON struct {
	Moment string `json:"moment"`
	ID     string `json:"id"`
	Scope  string `json:"scope"`
	Host   bool   `json:"host"`
	// Group is the id of the animateur group the setting belongs to ("" for the room).
	Group string `json:"group,omitempty"`
}

// roomGroupCount is N: the number of non-empty salle groups (§2.1).
func roomGroupCount(lc config.LightingConfig) int {
	n := 0
	for _, g := range lc.Groups {
		if g.Role != config.GroupRoleAnimateur && len(g.Lights) > 0 {
			n++
		}
	}
	return n
}

// effectsView builds the §7.1 answer. No I/O toward the bridge.
func (h *HTTPServer) effectsView() map[string]any {
	lc := config.Get().Lighting
	n := roomGroupCount(lc)
	var cat effects.Catalogue
	dir := ""
	if h.Effects != nil {
		cat, dir = h.Effects.EffectsCatalogue(), h.Effects.EffectsDir()
	}
	list := make([]effectEntryJSON, 0)
	for _, en := range cat.List() {
		e := en.Effect
		ok, reason := e.Applicable(n)
		hostOK := e.HostApplicable()
		moments := e.Moments
		if moments == nil {
			moments = []string{}
		}
		list = append(list, effectEntryJSON{
			ID: e.ID, Name: e.Name, Description: e.Description, Moments: moments, Origin: en.Origin, File: en.File,
			MinGroups: e.MinGroups, Applicable: ok, Reason: reason, HostApplicable: hostOK,
		})
	}
	errs := cat.Errors()
	if errs == nil {
		errs = []effects.Error{}
	}
	unresolved := []unresolvedJSON{}
	check := func(set map[string]string, host bool, group string) {
		for _, m := range ambiancecfg.Moments {
			id := set[m]
			if id == "" {
				continue
			}
			en, ok := cat.Get(id)
			bad := !ok || !en.Effect.HasMoment(effects.Moment(m))
			if !bad {
				if host {
					bad = !en.Effect.HostApplicable()
				} else {
					bad = !boolOnly(en.Effect.Applicable(n))
				}
			}
			if bad {
				unresolved = append(unresolved, unresolvedJSON{Moment: m, ID: id, Scope: "global", Host: host, Group: group})
			}
		}
	}
	check(lc.Effects, false, "")
	for _, g := range lc.HostGroups() { // one table per animateur group
		check(lc.HostEffects[g.ID], true, g.ID)
	}
	if dir != "" {
		dir = filepath.ToSlash(dir)
	}
	return map[string]any{"groups": n, "effects": list, "errors": errs, "unresolved": unresolved, "dir": dir}
}

func boolOnly(ok bool, _ string) bool { return ok }

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
