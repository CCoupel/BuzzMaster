// Package effects is the declarative light-effect engine of BuzzControl
// (v11.7.0, #223 — contracts/light-effects.md §3 to §5): effect files (YAML),
// strict parsing and validation, the catalogue with hot reload, and the pure,
// deterministic Render function that turns an effect into a frame (one look
// per ambiance group).
//
// The package is pure: no goroutine besides the optional Manager.Run loop, no
// network I/O, and it imports neither internal/game nor internal/lighting nor
// the Hue driver (contract §5.1). The ambiance layer feeds it a Context and
// reads Frames.
package effects

import (
	"fmt"
	"time"
)

// SchemaVersion is the only accepted value of the `version` key.
const SchemaVersion = 1

// Limits (contract §3.2, §4).
const (
	MaxFileBytes  = 64 << 10
	MaxFiles      = 100
	MaxGroups     = 12
	MinStepMs     = 250
	MaxStepMs     = 60000
	MaxFadeMs     = 5000
	MaxStages     = 6
	MaxPhases     = 8
	MaxIDLen      = 40
	MaxNameLen    = 40
	MaxDescLen    = 500
	MaxDurationMs = 600000
)

// Moment is a situation of the game for which an effect is chosen (§6).
type Moment string

const (
	MomentChrono Moment = "chrono"
	MomentCalm   Moment = "calm"
	MomentScore  Moment = "score"
)

// Valid reports whether m is one of the three moments.
func (m Moment) Valid() bool {
	return m == MomentChrono || m == MomentCalm || m == MomentScore
}

// notChrono reports whether m is explicitly another moment than chrono. The
// zero value (no moment declared) behaves as chrono: remaining-time stages and
// the urgency colour then apply.
func (m Moment) notChrono() bool { return m == MomentCalm || m == MomentScore }

// Targets of an effect.
const (
	TargetGroups = "groups"
	TargetAll    = "all"
)

// Pattern types (§4.3).
const (
	PatternUniform  = "uniform"
	PatternShift    = "shift"
	PatternGauge    = "gauge"
	PatternRandom   = "random"
	PatternGradient = "gradient"
)

// Effect is one effect file (§4). The yaml tags are the file's keys; decoding
// is strict, an unknown key is an error.
type Effect struct {
	ID          string                `yaml:"id" json:"id"`
	Name        string                `yaml:"name" json:"name"`
	Description string                `yaml:"description" json:"description"`
	Version     int                   `yaml:"version" json:"version"`
	Moments     []string              `yaml:"moments" json:"moments"`
	Target      string                `yaml:"target" json:"target"`
	MinGroups   int                   `yaml:"min_groups" json:"min_groups"`
	DurationMs  int                   `yaml:"duration_ms" json:"duration_ms"`
	Looks       map[string]Appearance `yaml:"looks" json:"looks"`
	Pattern     Pattern               `yaml:"pattern" json:"pattern"`
	Stages      []Stage               `yaml:"stages" json:"stages"`
}

// HasMoment reports whether the effect declares moment m.
func (e Effect) HasMoment(m Moment) bool {
	for _, x := range e.Moments {
		if Moment(x) == m {
			return true
		}
	}
	return false
}

// Appearance is a named appearance (a "look" in the file) (§4.2). Intensity is a pointer: absent means 100,
// an explicit 0 means off.
type Appearance struct {
	Color     string `yaml:"color" json:"color"`
	Intensity *int   `yaml:"intensity" json:"intensity"`
	FadeMs    int    `yaml:"fade_ms" json:"fade_ms"`
}

// Phase is one step of a `uniform` pattern.
type Phase struct {
	Appearance string `yaml:"look" json:"look"`
	Ms         int    `yaml:"ms" json:"ms"`
}

// Pattern holds the parameters of every pattern type; only the ones of Type
// are read. In a stage, a zero value means "not overridden".
type Pattern struct {
	Type string `yaml:"type" json:"type"`

	Phases []Phase `yaml:"phases" json:"phases,omitempty"` // uniform

	Cells     []string `yaml:"cells" json:"cells,omitempty"` // shift
	Rest      string   `yaml:"rest" json:"rest,omitempty"`
	StepMs    int      `yaml:"step_ms" json:"step_ms,omitempty"` // shift, random, gradient
	Wrap      *bool    `yaml:"wrap" json:"wrap,omitempty"`
	Direction string   `yaml:"direction" json:"direction,omitempty"` // shift, gauge

	Source string `yaml:"source" json:"source,omitempty"` // gauge
	On     string `yaml:"on" json:"on,omitempty"`         // gauge, random
	Off    string `yaml:"off" json:"off,omitempty"`
	Edge   string `yaml:"edge" json:"edge,omitempty"`

	Count *int     `yaml:"count" json:"count,omitempty"` // random
	Ratio *float64 `yaml:"ratio" json:"ratio,omitempty"`

	Stops  []string `yaml:"stops" json:"stops,omitempty"` // gradient
	Travel string   `yaml:"travel" json:"travel,omitempty"`
	Origin string   `yaml:"origin" json:"origin,omitempty"`
	Repeat *bool    `yaml:"repeat" json:"repeat,omitempty"`
}

// When is the condition of a stage (§4.4); every given key must hold.
type When struct {
	RemainingLTE *float64 `yaml:"remaining_lte" json:"remaining_lte,omitempty"`
	RemainingGT  *float64 `yaml:"remaining_gt" json:"remaining_gt,omitempty"`
	GroupsLT     *int     `yaml:"groups_lt" json:"groups_lt,omitempty"`
	GroupsGTE    *int     `yaml:"groups_gte" json:"groups_gte,omitempty"`
}

func (w When) empty() bool {
	return w.RemainingLTE == nil && w.RemainingGT == nil && w.GroupsLT == nil && w.GroupsGTE == nil
}

// Stage is a variant selected by its condition (§4.4).
type Stage struct {
	When    When                  `yaml:"when" json:"when"`
	Looks   map[string]Appearance `yaml:"looks" json:"looks,omitempty"`
	Pattern Pattern               `yaml:"pattern" json:"pattern"`
}

// Error is a validation problem localised in a file (§4.7). Message is a
// sentence in French without the file/line prefix; Error() adds it.
type Error struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

func (e Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s, ligne %d : %s", e.File, e.Line, e.Message)
	}
	return fmt.Sprintf("%s : %s", e.File, e.Message)
}

// Context is what the server provides to Render (§4.5). Every field is an
// input: Render never reads a clock or a random source of its own.
type Context struct {
	Groups    int           // N: non-empty salle groups (1 for the animateur group)
	Moment    Moment        // chrono, calm, score
	Remaining time.Duration // chrono: GameState.CurrentTime
	Total     time.Duration // chrono: GameState.Delay
	Scene     [3]int        // colour "scene"
	Theme     [3]int        // colour "theme" (colWhite when none)
	Team      [3]int        // colour "team" (credited team, moment score)
	Origin    int           // 1-based index, among the N rendered groups, of the credited team's group
	WallMs    int64         // wall clock (Unix ms), for `uniform`
	Seed      int64         // effect start timestamp, for `random`
}

// GroupLook is the rendering of one group: RGB, intensity 0-255 (the scale of
// lighting.ZoneState) and the fade handed to the bridge.
type GroupLook struct {
	Color     [3]int `json:"color"`
	Intensity int    `json:"intensity"`
	FadeMs    int    `json:"fade_ms"`
}

// Frame is one look per rendered group, in order; a single entry for
// `target: all`. A nil Frame means "not applicable".
type Frame []GroupLook
