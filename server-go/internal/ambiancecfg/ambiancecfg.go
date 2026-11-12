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

// Moments of the light effects (light-effects.md §6) and the default of each
// (v11.6.0 behaviour: the chrono pulse stays, nothing else changes).
const (
	MomentChrono = "chrono"
	MomentCalm   = "calm"
	MomentScore  = "score"
	// PulsationID is the default effect of the chrono moment.
	PulsationID = "pulsation"
)

// Moments lists the three moments in their normative order.
var Moments = []string{MomentChrono, MomentCalm, MomentScore}

// ValidMoment reports whether m is a known moment.
func ValidMoment(m string) bool { return m == MomentChrono || m == MomentCalm || m == MomentScore }

// Speeds (light-effects.md §6): the multiplier of an effect's tick, per moment.
// Allowed values 0.5, 0.75, 1, 1.5, 2 — default 1.
var SpeedValues = []float64{0.5, 0.75, 1, 1.5, 2}

// ValidSpeed reports whether v is one of the allowed multipliers.
func ValidSpeed(v float64) bool {
	for _, x := range SpeedValues {
		if v == x {
			return true
		}
	}
	return false
}

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
	// SoundFiles (r3, #290): path of the sound file of each cue + "decompte", relative to
	// data/files/sounds/ — complete form ("<cue>.wav" / "decompte.wav" for the catalogue).
	SoundFiles map[string]string
	// LightEffects / LightSpeeds (v11.7.0, #223, revision 4): the effect id and the speed
	// multiplier of each moment, by zone (ambiance, animateur, public). Complete form = the
	// three zones and the three moments always present.
	LightEffects map[string]map[string]string
	LightSpeeds  map[string]map[string]float64
}

// Partial is an overlay: every field optional, absent = inherited.
type Partial struct {
	SoundMuted       *bool
	CountdownEnabled *bool
	CountdownLoop    *bool
	CuesEnabled      map[string]bool // partial: only the overridden cues
	LightMode        *LightMode
	// SoundFiles (r3): partial, "rounds/<roundID>/<cue>.wav" — only the cues for which the
	// round has its own file. The direct layer never carries files (§4.2 point 4).
	SoundFiles map[string]string
	// LightEffects / LightSpeeds: partial, only the overridden zones and moments.
	LightEffects map[string]map[string]string
	LightSpeeds  map[string]map[string]float64
}

// Sources gives, for each resolved field, the layer it comes from.
type Sources struct {
	SoundMuted       Source
	CountdownEnabled Source
	CountdownLoop    Source
	LightMode        Source
	CuesEnabled      map[string]Source
	SoundFiles       map[string]Source            // "global" | "round"
	LightEffects     map[string]map[string]Source // by zone, then moment: "global" | "round"
	LightSpeeds      map[string]map[string]Source
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
	if s.SoundFiles != nil {
		c.SoundFiles = make(map[string]string, len(s.SoundFiles))
		for k, v := range s.SoundFiles {
			c.SoundFiles[k] = v
		}
	}
	c.LightEffects = cloneEffects(s.LightEffects)
	c.LightSpeeds = cloneSpeeds(s.LightSpeeds)
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
	if len(p.SoundFiles) > 0 {
		c.SoundFiles = make(map[string]string, len(p.SoundFiles))
		for k, v := range p.SoundFiles {
			c.SoundFiles[k] = v
		}
	}
	if len(p.LightEffects) > 0 {
		c.LightEffects = cloneEffects(p.LightEffects)
	}
	if len(p.LightSpeeds) > 0 {
		c.LightSpeeds = cloneSpeeds(p.LightSpeeds)
	}
	return c
}

// IsEmpty reports whether p overrides nothing (`{}`).
func (p Partial) IsEmpty() bool {
	return p.SoundMuted == nil && p.CountdownEnabled == nil && p.CountdownLoop == nil &&
		p.LightMode == nil && len(p.CuesEnabled) == 0 && len(p.SoundFiles) == 0 &&
		len(p.LightEffects) == 0 && len(p.LightSpeeds) == 0
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
	for k, v := range q.SoundFiles {
		if out.SoundFiles == nil {
			out.SoundFiles = map[string]string{}
		}
		out.SoundFiles[k] = v
	}
	for z, mm := range q.LightEffects {
		if out.LightEffects == nil {
			out.LightEffects = map[string]map[string]string{}
		}
		if out.LightEffects[z] == nil {
			out.LightEffects[z] = map[string]string{}
		}
		for m, v := range mm {
			out.LightEffects[z][m] = v
		}
	}
	for z, mm := range q.LightSpeeds {
		if out.LightSpeeds == nil {
			out.LightSpeeds = map[string]map[string]float64{}
		}
		if out.LightSpeeds[z] == nil {
			out.LightSpeeds[z] = map[string]float64{}
		}
		for m, v := range mm {
			out.LightSpeeds[z][m] = v
		}
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
	if eff.SoundFiles != nil {
		src.SoundFiles = make(map[string]Source, len(eff.SoundFiles))
		for k := range eff.SoundFiles {
			src.SoundFiles[k] = SourceGlobal
		}
	}
	// Light effects and speeds (revision 4): the three zones × three moments, always. A
	// Settings built without them (older callers) gets the defaults.
	eff.LightEffects = completeEffects(eff.LightEffects)
	eff.LightSpeeds = completeSpeeds(eff.LightSpeeds)
	src.LightEffects = map[string]map[string]Source{}
	src.LightSpeeds = map[string]map[string]Source{}
	for _, z := range Zones {
		src.LightEffects[z] = map[string]Source{MomentChrono: SourceGlobal, MomentCalm: SourceGlobal, MomentScore: SourceGlobal}
		src.LightSpeeds[z] = map[string]Source{MomentChrono: SourceGlobal, MomentCalm: SourceGlobal, MomentScore: SourceGlobal}
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
		// Effects and speeds never come from a direct gesture (§6): the round layer is the
		// only override. An unknown zone or moment is ignored here (validation refuses it).
		for z, mm := range l.LightEffects {
			if !ValidZone(z) {
				continue
			}
			for m, v := range mm {
				if ValidMoment(m) {
					eff.LightEffects[z][m], src.LightEffects[z][m] = v, s
				}
			}
		}
		for z, mm := range l.LightSpeeds {
			if !ValidZone(z) {
				continue
			}
			for m, v := range mm {
				if ValidMoment(m) {
					eff.LightSpeeds[z][m], src.LightSpeeds[z][m] = v, s
				}
			}
		}
		for cue, v := range l.SoundFiles { // files come from the round layer only (never "direct")
			if eff.SoundFiles == nil {
				eff.SoundFiles, src.SoundFiles = map[string]string{}, map[string]Source{}
			}
			eff.SoundFiles[cue], src.SoundFiles[cue] = v, SourceRound
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
	for _, z := range sortedZoneKeys(p.LightEffects) {
		for m := range p.LightEffects[z] {
			if !ValidZone(z) || !ValidMoment(m) {
				bad = append(bad, PathLightEffects+z+"."+m)
			}
		}
	}
	for _, z := range sortedZoneKeys(p.LightSpeeds) {
		for m, v := range p.LightSpeeds[z] {
			if !ValidZone(z) || !ValidMoment(m) || !ValidSpeed(v) {
				bad = append(bad, PathLightSpeeds+z+"."+m)
			}
		}
	}
	sort.Strings(bad)
	return bad
}

// Light effect paths (ambiance-settings.md §10).
const (
	PathLightEffects = "light.effects."
	PathLightSpeeds  = "light.speeds."
)

// InvalidEffects checks every effect id of p against exists(zone, moment, id) — the
// catalogue check, done by the caller that knows it. "" is always valid (no effect).
// Returns the offending JSON paths.
func (p Partial) InvalidEffects(exists func(zone, moment, id string) bool) []string {
	var bad []string
	for _, z := range sortedZoneKeys(p.LightEffects) {
		for m, id := range p.LightEffects[z] {
			if id != "" && ValidZone(z) && ValidMoment(m) && !exists(z, m, id) {
				bad = append(bad, PathLightEffects+z+"."+m)
			}
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
	Muted     *bool             `json:"muted,omitempty"`
	Countdown *countdownWire    `json:"countdown,omitempty"`
	Cues      map[string]bool   `json:"cues,omitempty"`
	Files     map[string]string `json:"files,omitempty"`
}
type partialLightWire struct {
	Mode    *LightMode  `json:"mode,omitempty"`
	Effects effectsWire `json:"effects,omitempty"`
	Speeds  speedsWire  `json:"speeds,omitempty"`
}
type partialWire struct {
	Sound *partialSoundWire `json:"sound,omitempty"`
	Light *partialLightWire `json:"light,omitempty"`
}

// MarshalJSON writes the structured, positive form; absent fields are omitted,
// `{}` for an empty Partial, and `cues` is omitted when empty (never `null`).
func (p Partial) MarshalJSON() ([]byte, error) {
	w := partialWire{}
	if p.SoundMuted != nil || p.CountdownEnabled != nil || p.CountdownLoop != nil || len(p.CuesEnabled) > 0 || len(p.SoundFiles) > 0 {
		w.Sound = &partialSoundWire{Muted: p.SoundMuted}
		if p.CountdownEnabled != nil || p.CountdownLoop != nil {
			w.Sound.Countdown = &countdownWire{Enabled: p.CountdownEnabled, Loop: p.CountdownLoop}
		}
		if len(p.CuesEnabled) > 0 {
			w.Sound.Cues = p.CuesEnabled
		}
		if len(p.SoundFiles) > 0 {
			w.Sound.Files = p.SoundFiles
		}
	}
	if p.LightMode != nil || len(p.LightEffects) > 0 || len(p.LightSpeeds) > 0 {
		w.Light = &partialLightWire{Mode: p.LightMode}
		if len(p.LightEffects) > 0 {
			w.Light.Effects = effectsWire(p.LightEffects)
		}
		if len(p.LightSpeeds) > 0 {
			w.Light.Speeds = speedsWire(p.LightSpeeds)
		}
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
		if len(w.Sound.Files) > 0 {
			p.SoundFiles = w.Sound.Files
		}
	}
	if w.Light != nil {
		p.LightMode = w.Light.Mode
		if len(w.Light.Effects) > 0 {
			p.LightEffects = map[string]map[string]string(w.Light.Effects)
		}
		if len(w.Light.Speeds) > 0 {
			p.LightSpeeds = map[string]map[string]float64(w.Light.Speeds)
		}
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
				case "files":
					// r3: the files map is written only by the REST API; here only its shape and
					// keys are checked (ROUNDS_UPDATE ignores and keeps it).
					fm, ok := sv.(map[string]interface{})
					if !ok {
						bad = append(bad, "sound.files")
						continue
					}
					for fk, fv := range fm {
						if _, isStr := fv.(string); !isStr || !validFileKey(fk, known) {
							bad = append(bad, "sound.files."+fk)
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
				switch lk {
				case "mode":
					if s, ok := lv.(string); !ok || !LightMode(s).Valid() {
						bad = append(bad, PathLightMode)
					}
				case "effects":
					zm, ok := lv.(map[string]interface{})
					if !ok {
						bad = append(bad, "light.effects")
						continue
					}
					for zk, zv := range zm {
						mm, isObj := zv.(map[string]interface{})
						if !isObj || !ValidZone(zk) {
							bad = append(bad, "light.effects."+zk)
							continue
						}
						for mk, mv := range mm {
							if _, isStr := mv.(string); !isStr || !ValidMoment(mk) {
								bad = append(bad, "light.effects."+zk+"."+mk)
							}
						}
					}
				case "speeds":
					zm, ok := lv.(map[string]interface{})
					if !ok {
						bad = append(bad, "light.speeds")
						continue
					}
					for zk, zv := range zm {
						mm, isObj := zv.(map[string]interface{})
						if !isObj || !ValidZone(zk) {
							bad = append(bad, "light.speeds."+zk)
							continue
						}
						for mk, mv := range mm {
							if f, isNum := mv.(float64); !isNum || !ValidMoment(mk) || !ValidSpeed(f) {
								bad = append(bad, "light.speeds."+zk+"."+mk)
							}
						}
					}
				default:
					bad = append(bad, "light."+lk)
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
		Cues  map[string]bool   `json:"cues"`
		Files map[string]string `json:"files"`
	} `json:"sound"`
	Light struct {
		Mode    LightMode                     `json:"mode"`
		Effects effectsWire                   `json:"effects"`
		Speeds  map[string]map[string]float64 `json:"speeds"`
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
	w.Sound.Files = s.SoundFiles
	if w.Sound.Files == nil {
		w.Sound.Files = map[string]string{}
	}
	w.Light.Mode = s.LightMode
	w.Light.Effects = effectsWire(completeEffects(s.LightEffects))
	w.Light.Speeds = completeSpeeds(s.LightSpeeds)
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
	s.SoundFiles = nil
	if len(w.Sound.Files) > 0 {
		s.SoundFiles = w.Sound.Files
	}
	s.LightMode = w.Light.Mode
	s.LightEffects = map[string]map[string]string(w.Light.Effects)
	s.LightSpeeds = w.Light.Speeds
	return nil
}

type sourcesWire struct {
	Sound struct {
		Muted     Source `json:"muted"`
		Countdown struct {
			Enabled Source `json:"enabled"`
			Loop    Source `json:"loop"`
		} `json:"countdown"`
		Cues  map[string]Source `json:"cues"`
		Files map[string]Source `json:"files"`
	} `json:"sound"`
	Light struct {
		Mode    Source                       `json:"mode"`
		Effects map[string]map[string]Source `json:"effects"`
		Speeds  map[string]map[string]Source `json:"speeds"`
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
	w.Sound.Files = s.SoundFiles
	if w.Sound.Files == nil {
		w.Sound.Files = map[string]Source{}
	}
	w.Light.Mode = def(s.LightMode)
	w.Light.Effects = map[string]map[string]Source{}
	w.Light.Speeds = map[string]map[string]Source{}
	for _, z := range Zones {
		w.Light.Effects[z] = map[string]Source{}
		w.Light.Speeds[z] = map[string]Source{}
		for _, m := range Moments {
			w.Light.Effects[z][m] = def(s.LightEffects[z][m])
			w.Light.Speeds[z][m] = def(s.LightSpeeds[z][m])
		}
	}
	return json.Marshal(w)
}

// String is a debugging aid.
func (s Settings) String() string {
	b, _ := json.Marshal(s)
	return fmt.Sprintf("%s", b)
}

// DecompteKey is the key of the countdown file in SoundFiles (not a cue).
const DecompteKey = "decompte"

func validFileKey(k string, known func(string) bool) bool {
	return k == DecompteKey || known == nil || known(k)
}

// RoundFilePath is the expected value of SoundFiles[key] for round roundID.
func RoundFilePath(roundID, key string) string { return "rounds/" + roundID + "/" + key + ".wav" }

// InvalidFilePaths checks p.SoundFiles for round roundID (r3): every key ∈ cues ∪ "decompte"
// and every value EXACTLY "rounds/<roundID>/<key>.wav" (no traversal, no other round).
// Returns the offending paths (empty when valid).
func (p Partial) InvalidFilePaths(roundID string, known func(cue string) bool) []string {
	var bad []string
	for k, v := range p.SoundFiles {
		if !validFileKey(k, known) || v != RoundFilePath(roundID, k) {
			bad = append(bad, "sound.files."+k)
		}
	}
	sort.Strings(bad)
	return bad
}
