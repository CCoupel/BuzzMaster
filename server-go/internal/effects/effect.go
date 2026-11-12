// Package effects is the declarative light-effect engine of BuzzControl (v11.7.0, #223 —
// contracts/light-effects.md §3 to §5, revision 3: step-by-step effects): effect files
// (YAML, `version: 2`), strict parsing and validation, the catalogue with hot reload, and
// the pure, deterministic Render function that unrolls an effect tick by tick.
//
// The package is pure: no goroutine besides the optional Manager.Run loop, no network
// I/O, and it imports neither internal/game nor internal/lighting nor the Hue driver
// (contract §5.1). The ambiance layer feeds it a Context and reads Frames.
package effects

import (
	"fmt"
	"time"
)

// SchemaVersion is the only accepted value of the `version` key.
const SchemaVersion = 2

// Limits (contract §3.2, §4).
const (
	MaxFileBytes = 64 << 10
	MaxFiles     = 100
	MaxGroups    = 12
	MinTickMs    = 250
	MaxTickMs    = 60000
	MaxSteps     = 200
	MaxActions   = 40
	MaxVars      = 24
	MaxIDLen     = 40
	MaxNameLen   = 40
	MaxDescLen   = 500
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

// notChrono reports whether m is explicitly another moment than chrono. The zero value
// (no moment declared) behaves as chrono: the `urgence` colour then follows the timer.
func (m Moment) notChrono() bool { return m == MomentCalm || m == MomentScore }

// TargetKind says what a target designates.
type TargetKind int

const (
	TargetG0  TargetKind = iota // g0 — the whole zone
	TargetGn                    // g1, g2… — the salle group of that number
	TargetGN                    // g* — the last non-empty group (v11.8.0: was gN)
	TargetVar                   // gA, gB… (+ neighbour offset) — a declared variable
)

// Target is the left side of an action (§4.3).
type Target struct {
	Kind   TargetKind
	N      int    // TargetGn: the group number
	Var    string // TargetVar: the variable name ("gA")
	Offset int    // TargetVar: neighbour rank (gA+1 → 1, gA-2 → -2)
}

// Color is a named colour (theme, equipe, urgence, scene — resolved at every render) or a
// fixed RGB value.
type Color struct {
	Name string // "" = fixed RGB
	RGB  [3]int
}

// Action is one `target: [colour, intensity]` entry of a step (§4.2).
type Action struct {
	Target Target
	Off    bool // [eteint]
	Color  Color
	Pct    int // 0-100
	Line   int
}

// Step is an ordered list of actions (1 to 40): the order written is the order applied.
type Step []Action

// Var is a group variable (§4.4, §12.2): a progression or a list. A progression has a start
// expression and, optionally, a step expression: with a step it "advances" (rank of
// start(0) + step × k), without it it "follows" (rank of start(k), the start being
// re-evaluated at every cran).
type Var struct {
	Name   string
	IsList bool
	List   []Target // TargetGn elements (list)
	Start  *Expr    // progression
	Step   *Expr    // progression: nil = follows
	Line   int
}

// Effect is one effect file after parsing and validation.
type Effect struct {
	ID          string
	Name        string // `nom`
	Description string
	Version     int
	Moments     []string
	MinGroups   int // `groupes_min`
	TickMs      int // `duree_tick_ms`
	Loop        bool
	Vars        []Var
	Pre         Step
	HasPre      bool
	Steps       []Step
	Last        Step
	HasLast     bool
	Post        Step
	HasPost     bool
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

// Error is a validation problem localised in a file (§4.8). Message is a sentence in
// French without the file/line prefix; Error() adds it.
type Error struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
	// Path locates a problem of an editor draft (`steps[2]`, `variables[0]`, `tick_ms`…, §11.3);
	// empty for a file.
	Path string `json:"path,omitempty"`
	// part narrows Path inside a variable ("start" or "step"), set by the validator.
	part string
}

func (e Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s, ligne %d : %s", e.File, e.Line, e.Message)
	}
	return fmt.Sprintf("%s : %s", e.File, e.Message)
}

// Context is what the server provides to Render (§4.5, §5.1). Every field is an input:
// Render never reads a clock or a random source of its own.
type Context struct {
	// Groups is N, the number of non-empty salle groups (0 possible). Nums, when set, gives
	// their real numbers in order (len == N); otherwise they are 1..Groups.
	Groups    int
	Nums      []int
	Moment    Moment
	Remaining time.Duration // chrono: GameState.CurrentTime
	Total     time.Duration // chrono: GameState.Delay
	Scene     [3]int        // colour "scene" (white for an animateur group)
	Theme     [3]int        // colour "theme" (white when none)
	Team      [3]int        // colour "equipe" (credited team, moment score)
}

// GroupLook is the rendering of one group: RGB, intensity 0-255 (the scale of
// lighting.ZoneState) and the fade handed to the bridge (always 0 since revision 3).
type GroupLook struct {
	Color     [3]int `json:"color"`
	Intensity int    `json:"intensity"`
	FadeMs    int    `json:"fade_ms"`
	// Absent: the group has no state of its own — it follows Base (or, when Base is nil,
	// the ordinary room scene).
	Absent bool `json:"absent,omitempty"`
}

// Frame is the state of the zone after a tick (§4.6): the state of g0 (nil until g0 was
// written) and one look per position 1..N.
type Frame struct {
	Base   *GroupLook
	Groups []GroupLook
}

// TickDuration is the real duration of one tick: tickMs ÷ speed, never under 250 ms (the
// writer's cadence) — §6.
func TickDuration(tickMs int, speed float64) time.Duration {
	if speed <= 0 {
		speed = 1
	}
	d := time.Duration(float64(tickMs) / speed * float64(time.Millisecond))
	if d < MinTickMs*time.Millisecond {
		d = MinTickMs * time.Millisecond
	}
	return d
}
