// Package ambiancecfg is the shared model of the behaviour settings of the room
// ambiance (sound + lighting): the complete Settings, the optional layer Partial,
// and the pure Resolve function applying global ← round ← direct, field by field
// (v11.6.0, #290 — contracts/ambiance-settings.md).
//
// It imports neither internal/config, internal/game nor internal/audio: it is
// testable alone. The adapters (config.json ⇄ Settings) live in cmd/server.
package ambiancecfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// LightMode is the general-zone selector position.
type LightMode string

const (
	LightOn   LightMode = "ON"
	LightAuto LightMode = "AUTO"
	LightOff  LightMode = "OFF"
)

// Valid reports whether m is one of ON / AUTO / OFF.
func (m LightMode) Valid() bool { return m == LightOn || m == LightAuto || m == LightOff }

// Source says which layer a resolved field comes from.
type Source string

const (
	SourceGlobal Source = "global"
	SourceRound  Source = "round"
	SourceDirect Source = "direct"
)

// Settings is the complete set of behaviour settings (always every key).
type Settings struct {
	SoundMuted       bool
	CountdownEnabled bool
	CountdownLoop    bool
	CuesEnabled      map[string]bool // key = cue name; complete form: every cue of the registry
	LightMode        LightMode
}

// Partial is an overlay: every field optional, absent = inherited.
type Partial struct {
	SoundMuted       *bool
	CountdownEnabled *bool
	CountdownLoop    *bool
	CuesEnabled      map[string]bool // partial: only the overridden cues
	LightMode        *LightMode
}

// Sources gives, for each resolved field, the layer it comes from.
type Sources struct {
	SoundMuted       Source
	CountdownEnabled Source
	CountdownLoop    Source
	LightMode        Source
	CuesEnabled      map[string]Source
}

// Effective is the published resolution (GameState.AMBIANCE_EFFECTIVE).
type Effective struct {
	Settings Settings `json:"SETTINGS"`
	Sources  Sources  `json:"SOURCES"`
	RoundID  string   `json:"ROUND_ID"`
}

// Field paths (JSON paths of ambiance-settings.md §3).
const (
	PathMuted     = "sound.muted"
	PathCdEnabled = "sound.countdown.enabled"
	PathCdLoop    = "sound.countdown.loop"
	PathCuePrefix = "sound.cues."
	PathLightMode = "light.mode"
)

// Clone deep-copies s.
func (s Settings) Clone() Settings {
	c := s
	c.CuesEnabled = make(map[string]bool, len(s.CuesEnabled))
	for k, v := range s.CuesEnabled {
		c.CuesEnabled[k] = v
	}
	return c
}

// Clone deep-copies p.
func (p Partial) Clone() Partial {
	c := Partial{}
	if p.SoundMuted != nil {
		v := *p.SoundMuted
		c.SoundMuted = &v
	}
	if p.CountdownEnabled != nil {
		v := *p.CountdownEnabled
		c.CountdownEnabled = &v
	}
	if p.CountdownLoop != nil {
		v := *p.CountdownLoop
		c.CountdownLoop = &v
	}
	if p.LightMode != nil {
		v := *p.LightMode
		c.LightMode = &v
	}
	if len(p.CuesEnabled) > 0 {
		c.CuesEnabled = make(map[string]bool, len(p.CuesEnabled))
		for k, v := range p.CuesEnabled {
			c.CuesEnabled[k] = v
		}
	}
	return c
}

// IsEmpty reports whether p overrides nothing (`{}`).
func (p Partial) IsEmpty() bool {
	return p.SoundMuted == nil && p.CountdownEnabled == nil && p.CountdownLoop == nil &&
		p.LightMode == nil && len(p.CuesEnabled) == 0
}

// Merge returns p overlaid by q (every field defined in q wins).
func (p Partial) Merge(q Partial) Partial {
	out := p.Clone()
	q = q.Clone()
	if q.SoundMuted != nil {
		out.SoundMuted = q.SoundMuted
	}
	if q.CountdownEnabled != nil {
		out.CountdownEnabled = q.CountdownEnabled
	}
	if q.CountdownLoop != nil {
		out.CountdownLoop = q.CountdownLoop
	}
	if q.LightMode != nil {
		out.LightMode = q.LightMode
	}
	for k, v := range q.CuesEnabled {
		if out.CuesEnabled == nil {
			out.CuesEnabled = map[string]bool{}
		}
		out.CuesEnabled[k] = v
	}
	return out
}

// Resolve applies the layers (round, then direct, in that order) over global:
// for each field the last layer that defines it wins, and its source is reported.
// Pure and deterministic; global is never modified.
func Resolve(global Settings, layers ...Partial) (Settings, Sources) {
	eff := global.Clone()
	src := Sources{
		SoundMuted: SourceGlobal, CountdownEnabled: SourceGlobal, CountdownLoop: SourceGlobal,
		LightMode: SourceGlobal, CuesEnabled: make(map[string]Source, len(eff.CuesEnabled)),
	}
	for k := range eff.CuesEnabled {
		src.CuesEnabled[k] = SourceGlobal
	}
	for i, l := range layers {
		s := SourceRound
		if i > 0 {
			s = SourceDirect
		}
		if i >= 2 { // extra layers behave as direct
			s = SourceDirect
		}
		if l.SoundMuted != nil {
			eff.SoundMuted, src.SoundMuted = *l.SoundMuted, s
		}
		if l.CountdownEnabled != nil {
			eff.CountdownEnabled, src.CountdownEnabled = *l.CountdownEnabled, s
		}
		if l.CountdownLoop != nil {
			eff.CountdownLoop, src.CountdownLoop = *l.CountdownLoop, s
		}
		if l.LightMode != nil {
			eff.LightMode, src.LightMode = *l.LightMode, s
		}
		for cue, v := range l.CuesEnabled {
			eff.CuesEnabled[cue], src.CuesEnabled[cue] = v, s
		}
	}
	return eff, src
}

// CueEnabled reads a cue's effective flag; a cue absent from the map is enabled
// (zero value = nothing is cut, contracts/sound.md §6.3).
func (s Settings) CueEnabled(cue string) bool {
	v, ok := s.CuesEnabled[cue]
	return !ok || v
}

// Validate checks p: light.mode ∈ {ON, AUTO, OFF}, cue names against known.
// Nil when valid; otherwise an error naming every offending path.
func (p Partial) Validate(known func(cue string) bool) error {
	if bad := p.InvalidPaths(known); len(bad) > 0 {
		return fmt.Errorf("invalid ambiance settings: %s", strings.Join(bad, ", "))
	}
	return nil
}

// InvalidPaths is Validate's list form: the offending JSON paths (empty when valid).
func (p Partial) InvalidPaths(known func(cue string) bool) []string {
	var bad []string
	if p.LightMode != nil && !p.LightMode.Valid() {
		bad = append(bad, PathLightMode)
	}
	for cue := range p.CuesEnabled {
		if known != nil && !known(cue) {
			bad = append(bad, PathCuePrefix+cue)
		}
	}
	sort.Strings(bad)
	return bad
}

// ---------------------------------------------------------------------------
// JSON (ambiance-settings.md §3)
// ---------------------------------------------------------------------------

type countdownWire struct {
	Enabled *bool `json:"enabled,omitempty"`
	Loop    *bool `json:"loop,omitempty"`
}
type partialSoundWire struct {
	Muted     *bool           `json:"muted,omitempty"`
	Countdown *countdownWire  `json:"countdown,omitempty"`
	Cues      map[string]bool `json:"cues,omitempty"`
}
type partialLightWire struct {
	Mode *LightMode `json:"mode,omitempty"`
}
type partialWire struct {
	Sound *partialSoundWire `json:"sound,omitempty"`
	Light *partialLightWire `json:"light,omitempty"`
}

// MarshalJSON writes the structured, positive form; absent fields are omitted,
// `{}` for an empty Partial, and `cues` is omitted when empty (never `null`).
func (p Partial) MarshalJSON() ([]byte, error) {
	w := partialWire{}
	if p.SoundMuted != nil || p.CountdownEnabled != nil || p.CountdownLoop != nil || len(p.CuesEnabled) > 0 {
		w.Sound = &partialSoundWire{Muted: p.SoundMuted}
		if p.CountdownEnabled != nil || p.CountdownLoop != nil {
			w.Sound.Countdown = &countdownWire{Enabled: p.CountdownEnabled, Loop: p.CountdownLoop}
		}
		if len(p.CuesEnabled) > 0 {
			w.Sound.Cues = p.CuesEnabled
		}
	}
	if p.LightMode != nil {
		w.Light = &partialLightWire{Mode: p.LightMode}
	}
	return json.Marshal(w)
}

// UnmarshalJSON reads the structured form; unknown keys are ignored here (a file
// written by a later version must not break loading) — API validation of unknown
// keys is ValidateRaw's job.
func (p *Partial) UnmarshalJSON(data []byte) error {
	var w partialWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*p = Partial{}
	if w.Sound != nil {
		p.SoundMuted = w.Sound.Muted
		if w.Sound.Countdown != nil {
			p.CountdownEnabled = w.Sound.Countdown.Enabled
			p.CountdownLoop = w.Sound.Countdown.Loop
		}
		if len(w.Sound.Cues) > 0 {
			p.CuesEnabled = w.Sound.Cues
		}
	}
	if w.Light != nil {
		p.LightMode = w.Light.Mode
	}
	return nil
}

// ValidateRaw strictly checks a raw JSON Partial (API write): unknown key, wrong
// type, light.mode out of range or unknown cue ⇒ the offending path. Empty = valid.
func ValidateRaw(raw []byte, known func(cue string) bool) []string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return []string{"(root)"}
	}
	var root map[string]interface{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return []string{"(root)"}
	}
	var bad []string
	isBool := func(v interface{}) bool { _, ok := v.(bool); return ok }
	for k, v := range root {
		switch k {
		case "sound":
			sm, ok := v.(map[string]interface{})
			if !ok {
				bad = append(bad, "sound")
				continue
			}
			for sk, sv := range sm {
				switch sk {
				case "muted":
					if !isBool(sv) {
						bad = append(bad, PathMuted)
					}
				case "countdown":
					cm, ok := sv.(map[string]interface{})
					if !ok {
						bad = append(bad, "sound.countdown")
						continue
					}
					for ck, cv := range cm {
						if (ck != "enabled" && ck != "loop") || !isBool(cv) {
							bad = append(bad, "sound.countdown."+ck)
						}
					}
				case "cues":
					cm, ok := sv.(map[string]interface{})
					if !ok {
						bad = append(bad, "sound.cues")
						continue
					}
					for cue, cv := range cm {
						if !isBool(cv) || (known != nil && !known(cue)) {
							bad = append(bad, PathCuePrefix+cue)
						}
					}
				default:
					bad = append(bad, "sound."+sk)
				}
			}
		case "light":
			lm, ok := v.(map[string]interface{})
			if !ok {
				bad = append(bad, "light")
				continue
			}
			for lk, lv := range lm {
				if lk != "mode" {
					bad = append(bad, "light."+lk)
					continue
				}
				if s, ok := lv.(string); !ok || !LightMode(s).Valid() {
					bad = append(bad, PathLightMode)
				}
			}
		default:
			bad = append(bad, k)
		}
	}
	sort.Strings(bad)
	return bad
}

// ParsePartial validates strictly then decodes raw.
func ParsePartial(raw []byte, known func(cue string) bool) (Partial, []string) {
	if bad := ValidateRaw(raw, known); len(bad) > 0 {
		return Partial{}, bad
	}
	var p Partial
	if err := json.Unmarshal(raw, &p); err != nil {
		return Partial{}, []string{"(root)"}
	}
	return p, nil
}

type settingsWire struct {
	Sound struct {
		Muted     bool `json:"muted"`
		Countdown struct {
			Enabled bool `json:"enabled"`
			Loop    bool `json:"loop"`
		} `json:"countdown"`
		Cues map[string]bool `json:"cues"`
	} `json:"sound"`
	Light struct {
		Mode LightMode `json:"mode"`
	} `json:"light"`
}

// MarshalJSON writes the complete form (cues never null).
func (s Settings) MarshalJSON() ([]byte, error) {
	var w settingsWire
	w.Sound.Muted = s.SoundMuted
	w.Sound.Countdown.Enabled = s.CountdownEnabled
	w.Sound.Countdown.Loop = s.CountdownLoop
	w.Sound.Cues = s.CuesEnabled
	if w.Sound.Cues == nil {
		w.Sound.Cues = map[string]bool{}
	}
	w.Light.Mode = s.LightMode
	return json.Marshal(w)
}

// UnmarshalJSON reads the complete form.
func (s *Settings) UnmarshalJSON(data []byte) error {
	var w settingsWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	s.SoundMuted = w.Sound.Muted
	s.CountdownEnabled = w.Sound.Countdown.Enabled
	s.CountdownLoop = w.Sound.Countdown.Loop
	s.CuesEnabled = w.Sound.Cues
	s.LightMode = w.Light.Mode
	return nil
}

type sourcesWire struct {
	Sound struct {
		Muted     Source `json:"muted"`
		Countdown struct {
			Enabled Source `json:"enabled"`
			Loop    Source `json:"loop"`
		} `json:"countdown"`
		Cues map[string]Source `json:"cues"`
	} `json:"sound"`
	Light struct {
		Mode Source `json:"mode"`
	} `json:"light"`
}

// MarshalJSON writes the same structure as Settings with source names.
func (s Sources) MarshalJSON() ([]byte, error) {
	def := func(v Source) Source {
		if v == "" {
			return SourceGlobal
		}
		return v
	}
	var w sourcesWire
	w.Sound.Muted = def(s.SoundMuted)
	w.Sound.Countdown.Enabled = def(s.CountdownEnabled)
	w.Sound.Countdown.Loop = def(s.CountdownLoop)
	w.Sound.Cues = s.CuesEnabled
	if w.Sound.Cues == nil {
		w.Sound.Cues = map[string]Source{}
	}
	w.Light.Mode = def(s.LightMode)
	return json.Marshal(w)
}

// String is a debugging aid.
func (s Settings) String() string {
	b, _ := json.Marshal(s)
	return fmt.Sprintf("%s", b)
}
