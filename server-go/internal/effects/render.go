package effects

import (
	"fmt"
)

// Urgency colours — lighting.md §8.2 (same values as the former ambiance.ChronoPulse*).
var (
	colOrange = [3]int{255, 170, 0}
	colRed    = [3]int{230, 30, 30}
	colWhite  = [3]int{255, 255, 255}
)

// groupNums returns the real numbers of the non-empty groups for ctx (1..Groups by default).
func (c Context) groupNums() []int {
	if c.Nums != nil {
		return c.Nums
	}
	out := make([]int, c.Groups)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

// exists reports whether target t designates something with n groups numbered nums (for the
// skip rule of §4.3 — a step none of whose actions aims at an existing target is skipped).
func (e Effect) targetExists(t Target, nums []int) bool {
	switch t.Kind {
	case TargetG0:
		return true
	case TargetGn:
		return indexOf(nums, t.N) >= 0
	case TargetGN:
		return len(nums) > 0
	case TargetVar:
		if len(nums) == 0 {
			return false
		}
		for _, v := range e.Vars {
			if v.Name == t.Var && v.IsList {
				return len(listPositions(v, nums)) > 0
			}
		}
		return true
	}
	return false
}

func indexOf(nums []int, n int) int {
	for i, x := range nums {
		if x == n {
			return i
		}
	}
	return -1
}

// listPositions returns the positions (0-based) of the list elements that exist.
func listPositions(v Var, nums []int) []int {
	var out []int
	for _, t := range v.List {
		if i := indexOf(nums, t.N); i >= 0 {
			out = append(out, i)
		}
	}
	return out
}

func (e Effect) stepSkipped(s Step, nums []int) bool {
	for _, a := range s {
		if e.targetExists(a.Target, nums) {
			return false
		}
	}
	return true
}

// liveSteps lists the indexes of the loop steps that are not skipped for these groups.
func (e Effect) liveSteps(nums []int) []int {
	var out []int
	for i, s := range e.Steps {
		if !e.stepSkipped(s, nums) {
			out = append(out, i)
		}
	}
	return out
}

// Applicable reports whether e can play in a room with n groups (§4.7) and, when it
// cannot, why (in French, for the catalogue).
func (e Effect) Applicable(n int) (bool, string) {
	if n < e.MinGroups {
		return false, fmt.Sprintf("demande %d groupe(s), %d configuré(s)", e.MinGroups, n)
	}
	nums := make([]int, n)
	for i := range nums {
		nums[i] = i + 1
	}
	if len(e.liveSteps(nums)) == 0 {
		return false, "aucun pas ne vise un groupe existant"
	}
	return true, ""
}

// schedule says what is played at tick t (§4.5): kind 'P' (PRE), 'S' (loop step j), 'L'
// (LAST) or 'H' (a one-round effect holding its final state), with the cran k.
type playedTick struct {
	kind byte
	step int // index in e.Steps for 'S'
	k    int
}

func (e Effect) playedAt(tick int, live []int) playedTick {
	pre := 0
	if e.HasPre {
		pre = 1
	}
	if tick < pre {
		return playedTick{kind: 'P'}
	}
	m := len(live)
	cycle := m
	if e.HasLast {
		cycle++
	}
	t := tick - pre
	round, idx := t/cycle, t%cycle
	if idx < m {
		return playedTick{kind: 'S', step: live[idx], k: round*m + idx}
	}
	return playedTick{kind: 'L', k: round*m + m - 1}
}

// lastTick is the last tick a one-round effect plays; later ticks hold that state.
func (e Effect) lastTick(live []int) int {
	pre := 0
	if e.HasPre {
		pre = 1
	}
	cycle := len(live)
	if e.HasLast {
		cycle++
	}
	return pre + cycle - 1
}

// look is the state written by an action: resolved only when the frame is built.
type look struct {
	col Color
	pct int
	off bool
}

type zoneState struct {
	base    look
	baseSet bool
	groups  []look
	set     []bool
}

// position resolves a target to the positions (0-based) it designates at cran k.
func (e Effect) positions(t Target, k int, nums []int) []int {
	n := len(nums)
	switch t.Kind {
	case TargetGn:
		if i := indexOf(nums, t.N); i >= 0 {
			return []int{i}
		}
	case TargetGN:
		if n > 0 {
			return []int{n - 1}
		}
	case TargetVar:
		if n == 0 {
			return nil
		}
		for _, v := range e.Vars {
			if v.Name != t.Var {
				continue
			}
			var pos int
			if v.IsList {
				lp := listPositions(v, nums)
				if len(lp) == 0 {
					return nil
				}
				pos = lp[k%len(lp)]
			} else {
				start := v.Start.N
				if v.Start.Kind == TargetGN {
					start = n
				}
				pos = posMod(start-1+v.Incr*k, n)
			}
			return []int{posMod(pos+t.Offset, n)}
		}
	}
	return nil
}

func posMod(a, m int) int {
	if m <= 0 {
		return 0
	}
	return ((a % m) + m) % m
}

func (e Effect) apply(z *zoneState, s Step, k int, nums []int) {
	for _, a := range s {
		l := look{col: a.Color, pct: a.Pct, off: a.Off}
		if a.Target.Kind == TargetG0 {
			z.base, z.baseSet = l, true
			for i := range z.groups {
				z.groups[i], z.set[i] = l, true
			}
			continue
		}
		for _, p := range e.positions(a.Target, k, nums) {
			z.groups[p], z.set[p] = l, true
		}
	}
}

// replay plays ticks 0..tick on a fresh zone and returns it with the cran of the last tick.
func (e Effect) replay(tick int, nums []int) (z zoneState, k int, ok bool) {
	live := e.liveSteps(nums)
	if len(live) == 0 {
		return zoneState{}, 0, false
	}
	if !e.Loop {
		if lt := e.lastTick(live); tick > lt {
			tick = lt
		}
	}
	if tick < 0 {
		tick = 0
	}
	z = zoneState{groups: make([]look, len(nums)), set: make([]bool, len(nums))}
	for t := 0; t <= tick; t++ {
		p := e.playedAt(t, live)
		k = p.k
		switch p.kind {
		case 'P':
			e.apply(&z, e.Pre, 0, nums)
		case 'S':
			e.apply(&z, e.Steps[p.step], p.k, nums)
		case 'L':
			e.apply(&z, e.Last, p.k, nums)
		}
	}
	return z, k, true
}

// Render computes the frame after tick `tick` (§4.5, §4.6). It is deterministic: the
// unrolling is a pure function of the tick. A zero Frame means "not applicable".
func Render(e Effect, ctx Context, tick int) Frame {
	nums := ctx.groupNums()
	z, _, ok := e.replay(tick, nums)
	if !ok {
		return Frame{}
	}
	return z.frame(ctx)
}

// RenderPost computes the POST frame of §4.5 point 5, with the variables at their last
// value, after the effect last played tick lastTick. false when the effect has no POST.
func RenderPost(e Effect, ctx Context, lastTick int) (Frame, bool) {
	if !e.HasPost {
		return Frame{}, false
	}
	nums := ctx.groupNums()
	z, k, ok := e.replay(lastTick, nums)
	if !ok {
		return Frame{}, false
	}
	e.apply(&z, e.Post, k, nums)
	return z.frame(ctx), true
}

func (z zoneState) frame(ctx Context) Frame {
	f := Frame{Groups: make([]GroupLook, len(z.groups))}
	var base GroupLook
	if z.baseSet {
		base = resolve(z.base, ctx)
		b := base
		f.Base = &b
	}
	for i := range z.groups {
		if !z.set[i] {
			f.Groups[i] = GroupLook{Absent: true}
			continue
		}
		g := resolve(z.groups[i], ctx)
		if z.baseSet && g == base {
			f.Groups[i] = GroupLook{Absent: true}
			continue
		}
		f.Groups[i] = g
	}
	return f
}

// resolve gives a look its concrete colour for ctx (named colours are live).
func resolve(l look, ctx Context) GroupLook {
	if l.off {
		return GroupLook{}
	}
	var c [3]int
	switch l.col.Name {
	case "":
		c = l.col.RGB
	case "theme":
		c = ctx.Theme
	case "scene":
		c = ctx.Scene
	case "equipe":
		c = colWhite
		if ctx.Moment == MomentScore {
			c = ctx.Team
		}
	case "urgence":
		c = colWhite
		if !ctx.Moment.notChrono() {
			switch sec := ctx.Remaining.Seconds(); {
			case sec <= 5:
				c = colRed
			case sec <= 10:
				c = colOrange
			}
		}
	}
	return GroupLook{Color: c, Intensity: int(float64(l.pct)*255/100 + 0.5)}
}
