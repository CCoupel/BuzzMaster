package server

import (
	"encoding/json"
	"net/http"

	"buzzcontrol/internal/ambiancecfg"
	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/config"
)

// Ambiance settings (v11.6.0, #290 — contracts/ambiance-settings.md): the config.json ⇄
// Settings adapter, the provider the App implements, GET /api/ambiance/settings and the
// "geste direct" handling of POST /config.json (§4.2).

// AmbianceView is what GET /api/ambiance/settings returns.
type AmbianceView struct {
	Global        ambiancecfg.Settings `json:"global"`
	Effective     ambiancecfg.Settings `json:"effective"`
	Sources       ambiancecfg.Sources  `json:"sources"`
	RoundID       string               `json:"round_id"`
	RoundOverride ambiancecfg.Partial  `json:"round_override"`
}

// AmbianceProvider is implemented by the App (cmd/server/round_ambiance.go).
type AmbianceProvider interface {
	// AmbianceSettings returns the resolved settings (global ← round ← direct).
	AmbianceSettings() (ambiancecfg.Settings, ambiancecfg.Sources)
	// AmbianceRoundLayer returns the override of the effective round (empty outside a round).
	AmbianceRoundLayer() ambiancecfg.Partial
	// AmbianceView feeds GET /api/ambiance/settings.
	AmbianceView() AmbianceView
	// AmbianceGesture records a direct gesture (fields changed by the régie), then
	// re-resolves, applies and broadcasts.
	AmbianceGesture(changes ambiancecfg.Partial)
}

// AmbianceFromConfig is THE adapter from config.json to Settings (the only reader of
// sound.muted / cues_disabled / countdown_* / lighting.default_mode — contract §5.2,
// guarded by a test). A config without default_mode gives AUTO; every cue of the
// registry is present.
func AmbianceFromConfig(cfg *config.Config) ambiancecfg.Settings {
	s := ambiancecfg.Settings{
		SoundMuted:       cfg.Sound.Muted,
		CountdownEnabled: !cfg.Sound.CountdownDisabled,
		CountdownLoop:    !cfg.Sound.CountdownOnce,
		CuesEnabled:      make(map[string]bool, len(audio.AllCues())),
		LightMode:        ambiancecfg.LightAuto,
	}
	for _, c := range audio.AllCues() {
		s.CuesEnabled[string(c)] = !cfg.Sound.CuesDisabled[string(c)]
	}
	if m := ambiancecfg.LightMode(cfg.Lighting.DefaultMode); m.Valid() {
		s.LightMode = m
	}
	return s
}

// knownCue is the cue registry for ambiancecfg validation.
func knownCue(c string) bool { _, ok := audio.CueByName(c); return ok }

// handleAPIAmbianceSettings handles GET /api/ambiance/settings.
func (h *HTTPServer) handleAPIAmbianceSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var view AmbianceView
	if h.Ambiance != nil {
		view = h.Ambiance.AmbianceView()
	} else {
		g := AmbianceFromConfig(config.Get())
		eff, src := ambiancecfg.Resolve(g)
		view = AmbianceView{Global: g, Effective: eff, Sources: src}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}

// ambianceSettingsNow is the resolved settings, through the provider when wired.
func (h *HTTPServer) ambianceSettingsNow() (ambiancecfg.Settings, ambiancecfg.Sources) {
	if h.Ambiance != nil {
		return h.Ambiance.AmbianceSettings()
	}
	g := AmbianceFromConfig(config.Get())
	return ambiancecfg.Resolve(g)
}

// ambianceGestureFromRequest implements §4.2 for POST /config.json: from the fields the
// request actually wrote (raw key presence), it returns the direct changes (fields whose
// value differs from the resolved one) and puts back in cfg the global value of every
// field the effective round overrides (the gesture then only holds for the round).
// scope is "round" when at least one changed field was held back from the global.
func ambianceGestureFromRequest(raw map[string]json.RawMessage, before ambiancecfg.Settings, round ambiancecfg.Partial,
	current, cfg *config.Config) (changes ambiancecfg.Partial, scope string) {
	scope = "global"
	touchedRound := false
	curCfg := AmbianceFromConfig(current)
	_ = curCfg

	var sRaw map[string]json.RawMessage
	if data, ok := raw["sound"]; ok {
		_ = json.Unmarshal(data, &sRaw)
	}
	if _, ok := sRaw["muted"]; ok {
		if v := cfg.Sound.Muted; v != before.SoundMuted {
			changes.SoundMuted = &v
			if round.SoundMuted != nil {
				cfg.Sound.Muted = current.Sound.Muted
				touchedRound = true
			}
		}
	}
	if _, ok := sRaw["countdown_disabled"]; ok {
		if v := !cfg.Sound.CountdownDisabled; v != before.CountdownEnabled {
			changes.CountdownEnabled = &v
			if round.CountdownEnabled != nil {
				cfg.Sound.CountdownDisabled = current.Sound.CountdownDisabled
				touchedRound = true
			}
		}
	}
	if _, ok := sRaw["countdown_once"]; ok {
		if v := !cfg.Sound.CountdownOnce; v != before.CountdownLoop {
			changes.CountdownLoop = &v
			if round.CountdownLoop != nil {
				cfg.Sound.CountdownOnce = current.Sound.CountdownOnce
				touchedRound = true
			}
		}
	}
	if _, ok := sRaw["cues_disabled"]; ok {
		incomingDisabled := cfg.Sound.CuesDisabled
		merged := make(map[string]bool, len(incomingDisabled))
		for k, v := range incomingDisabled {
			merged[k] = v
		}
		for _, c := range audio.AllCues() {
			name := string(c)
			want := !incomingDisabled[name]
			if want == before.CueEnabled(name) {
				continue
			}
			if changes.CuesEnabled == nil {
				changes.CuesEnabled = map[string]bool{}
			}
			changes.CuesEnabled[name] = want
			if _, overridden := round.CuesEnabled[name]; overridden {
				// back to the stored global value of this cue
				if current.Sound.CuesDisabled[name] {
					merged[name] = true
				} else {
					delete(merged, name)
				}
				touchedRound = true
			}
		}
		cfg.Sound.CuesDisabled = merged
	}
	var lRaw map[string]json.RawMessage
	if data, ok := raw["lighting"]; ok {
		_ = json.Unmarshal(data, &lRaw)
	}
	if _, ok := lRaw["default_mode"]; ok {
		m := ambiancecfg.LightMode(cfg.Lighting.DefaultMode)
		if !m.Valid() { // "" ⇒ AUTO (D8)
			m = ambiancecfg.LightAuto
		}
		if m != before.LightMode {
			changes.LightMode = &m
			if round.LightMode != nil {
				cfg.Lighting.DefaultMode = current.Lighting.DefaultMode
				touchedRound = true
			}
		}
	}
	if touchedRound {
		scope = "round"
	}
	return changes, scope
}
