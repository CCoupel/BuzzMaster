package effects

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// Urgency colours — lighting.md §8.2 (same values as ambiance.ChronoPulse*).
var (
	colOrange = [3]int{255, 170, 0}
	colRed    = [3]int{230, 30, 30}
	colWhite  = [3]int{255, 255, 255}
)

// Applicable reports whether e can play with n groups (§4.6) and, when it
// cannot, why (in French, for the catalogue).
func (e Effect) Applicable(n int) (bool, string) {
	if e.Target == TargetAll {
		// One state for the whole general zone, always applicable — with no
		// group at all too (the pulsation worked without any in v11.6.0).
		return true, ""
	}
	min := e.MinGroups
	if min < 1 {
		min = 1
	}
	if n < min {
		return false, fmt.Sprintf("demande %d groupe(s), %d configuré(s)", min, n)
	}
	if n == 0 {
		return false, "aucun groupe configuré"
	}
	return true, ""
}

// Render computes the frame of e at elapsed time since the effect started. It
// is deterministic: same inputs, same frame. It returns nil when the effect is
// not applicable to ctx.Groups. The frame has ctx.Groups entries, or a single
// one for `target: all`.
func Render(e Effect, ctx Context, elapsed time.Duration) Frame {
	if ok, _ := e.Applicable(ctx.Groups); !ok {
		return nil
	}
	n := ctx.Groups
	if e.Target == TargetAll {
		n = 1
	}
	looks, pat := effective(e, ctx)
	rl := func(name string) GroupLook { return resolveLook(looks[name], ctx) }
	if elapsed < 0 {
		elapsed = 0
	}
	frame := make(Frame, n)
	switch pat.Type {
	case PatternUniform:
		g := rl(uniformPhase(pat.Phases, ctx.WallMs))
		for i := range frame {
			frame[i] = g
		}
	case PatternShift:
		renderShift(frame, pat, rl, elapsed)
	case PatternGauge:
		renderGauge(frame, pat, rl, ctx)
	case PatternRandom:
		renderRandom(frame, pat, rl, ctx, elapsed)
	case PatternGradient:
		renderGradient(frame, pat, rl, ctx, elapsed)
	default:
		return nil
	}
	return frame
}

// effective returns the looks and pattern in force: the first stage whose
// condition holds is applied over the base definition (§4.4).
func effective(e Effect, ctx Context) (map[string]Appearance, Pattern) {
	for _, st := range e.Stages {
		if stageApplies(st.When, ctx) {
			return applyStage(e.Looks, e.Pattern, st)
		}
	}
	return e.Looks, e.Pattern
}

func stageApplies(w When, ctx Context) bool {
	if w.RemainingLTE != nil || w.RemainingGT != nil {
		if ctx.Moment.notChrono() {
			return false
		}
		sec := ctx.Remaining.Seconds()
		if w.RemainingLTE != nil && !(sec <= *w.RemainingLTE) {
			return false
		}
		if w.RemainingGT != nil && !(sec > *w.RemainingGT) {
			return false
		}
	}
	if w.GroupsLT != nil && !(ctx.Groups < *w.GroupsLT) {
		return false
	}
	if w.GroupsGTE != nil && !(ctx.Groups >= *w.GroupsGTE) {
		return false
	}
	return true
}

func resolveLook(l Appearance, ctx Context) GroupLook {
	var c [3]int
	switch l.Color {
	case "", "scene":
		c = ctx.Scene
	case "theme":
		c = ctx.Theme
	case "team":
		c = ctx.Team
	case "urgency":
		c = colWhite
		if !ctx.Moment.notChrono() {
			switch sec := ctx.Remaining.Seconds(); {
			case sec <= 5:
				c = colRed
			case sec <= 10:
				c = colOrange
			}
		}
	default:
		c = parseHex(l.Color)
	}
	pct := 100
	if l.Intensity != nil {
		pct = *l.Intensity
	}
	return GroupLook{Color: c, Intensity: int(float64(pct)*255/100 + 0.5), FadeMs: l.FadeMs}
}

func parseHex(s string) [3]int {
	var r, g, b int
	if _, err := fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return colWhite
	}
	return [3]int{r, g, b}
}

func uniformPhase(phases []Phase, wallMs int64) string {
	var cycle int64
	for _, p := range phases {
		cycle += int64(p.Ms)
	}
	if cycle <= 0 || len(phases) == 0 {
		return ""
	}
	t := wallMs % cycle
	if t < 0 {
		t += cycle
	}
	for _, p := range phases {
		if t < int64(p.Ms) {
			return p.Appearance
		}
		t -= int64(p.Ms)
	}
	return phases[len(phases)-1].Appearance
}

func stepIndex(elapsed time.Duration, stepMs int) int {
	if stepMs <= 0 {
		return 0
	}
	return int(elapsed / (time.Duration(stepMs) * time.Millisecond))
}

func posMod(a, m int) int {
	if m <= 0 {
		return 0
	}
	return ((a % m) + m) % m
}

func renderShift(frame Frame, p Pattern, rl func(string) GroupLook, elapsed time.Duration) {
	n, l := len(frame), len(p.Cells)
	rest := rl(p.Rest)
	for i := range frame {
		frame[i] = rest
	}
	if n == 0 || l == 0 {
		return
	}
	step := stepIndex(elapsed, p.StepMs)
	wrap := p.Wrap == nil || *p.Wrap
	var start int
	switch p.Direction {
	case "bounce":
		m := n - l
		if m < 0 {
			m = 0
		}
		if m == 0 {
			start = 0
		} else {
			s := step % (2 * m)
			if s > m {
				s = 2*m - s
			}
			start = s
		}
		wrap = false
	default:
		if wrap {
			start = posMod(step, n)
		} else {
			start = posMod(step, n+l-1) - (l - 1)
		}
	}
	for k, c := range p.Cells {
		idx := start + k
		if wrap && p.Direction != "bounce" {
			idx = posMod(idx, n)
		}
		if idx < 0 || idx >= n {
			continue
		}
		if p.Direction == "backward" {
			idx = n - 1 - idx
		}
		frame[idx] = rl(c)
	}
}

func renderGauge(frame Frame, p Pattern, rl func(string) GroupLook, ctx Context) {
	n := len(frame)
	frac := 0.0
	if ctx.Total > 0 {
		frac = float64(ctx.Remaining) / float64(ctx.Total)
		if p.Source == "elapsed" {
			frac = 1 - frac
		}
	}
	frac = math.Max(0, math.Min(1, frac))
	x := frac * float64(n)
	full := int(math.Floor(x + 1e-9))
	partial := x-float64(full) > 1e-9
	on, off := rl(p.On), rl(p.Off)
	for i := range frame {
		var g GroupLook
		switch {
		case i < full:
			g = on
		case i == full && partial && p.Edge != "":
			g = rl(p.Edge)
		default:
			g = off
		}
		idx := i
		if p.Direction == "backward" {
			idx = n - 1 - i
		}
		frame[idx] = g
	}
}

func renderRandom(frame Frame, p Pattern, rl func(string) GroupLook, ctx Context, elapsed time.Duration) {
	n := len(frame)
	count := 0
	switch {
	case p.Count != nil:
		count = *p.Count
	case p.Ratio != nil:
		count = int(math.Round(*p.Ratio * float64(n)))
	}
	if count > n {
		count = n
	}
	if count < 0 {
		count = 0
	}
	step := int64(stepIndex(elapsed, p.StepMs))
	rng := rand.New(rand.NewSource(ctx.Seed*1000003 + step))
	perm := rng.Perm(n)
	off, on := rl(p.Off), rl(p.On)
	for i := range frame {
		frame[i] = off
	}
	for _, idx := range perm[:count] {
		frame[idx] = on
	}
}

func renderGradient(frame Frame, p Pattern, rl func(string) GroupLook, ctx Context, elapsed time.Duration) {
	n := len(frame)
	stops := make([]GroupLook, len(p.Stops))
	for i, s := range p.Stops {
		stops[i] = rl(s)
	}
	if n == 0 || len(stops) == 0 {
		return
	}
	step := float64(stepIndex(elapsed, p.StepMs))
	repeat := p.Repeat == nil || *p.Repeat
	origin := 0.0
	switch p.Origin {
	case "last":
		origin = float64(n - 1)
	case "center":
		origin = float64(n-1) / 2
	case "team":
		o := ctx.Origin
		if o < 1 || o > n {
			o = 1
		}
		origin = float64(o - 1)
	}
	span := float64(n - 1)
	for i := range frame {
		var pos float64
		switch p.Travel {
		case "forward":
			pos = float64(i) - step
		case "backward":
			pos = float64(i) + step
		case "outward":
			pos = math.Abs(float64(i)-origin) - step
		default:
			pos = float64(i)
		}
		t := 0.0
		if span > 0 {
			t = pos / span
		}
		if repeat {
			t -= math.Floor(t)
		} else {
			t = math.Max(0, math.Min(1, t))
		}
		frame[i] = sampleStops(stops, t)
	}
}

// sampleStops interpolates linearly (RGB and intensity) between the stops.
func sampleStops(stops []GroupLook, t float64) GroupLook {
	if len(stops) == 1 {
		return stops[0]
	}
	x := t * float64(len(stops)-1)
	i := int(math.Floor(x))
	if i >= len(stops)-1 {
		return stops[len(stops)-1]
	}
	f := x - float64(i)
	a, b := stops[i], stops[i+1]
	lerp := func(u, v int) int { return int(math.Round(float64(u) + (float64(v)-float64(u))*f)) }
	return GroupLook{
		Color:     [3]int{lerp(a.Color[0], b.Color[0]), lerp(a.Color[1], b.Color[1]), lerp(a.Color[2], b.Color[2])},
		Intensity: lerp(a.Intensity, b.Intensity),
		FadeMs:    a.FadeMs,
	}
}
