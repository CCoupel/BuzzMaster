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

// resolver evaluates the variables of an effect for one zone (n groups, real numbers nums). It
// is built once per replay; the values at the cran of the tick being played are cached and
// reset at each tick, the values at cran 0 (the "start" of an advancing variable, §12.2) are
// cached for good.
type resolver struct {
	e     *Effect
	n     int
	nums  []int
	idx   map[string]int
	k     int
	atK   []varSlot // rank (1…n) at cran k
	at0   []varSlot // rank at cran 0
	basis []varSlot // advancing variable: start(0) (a value, not a rank) — step in step[]
	step  []int
	lists [][]int // list variables: positions of the elements that exist (nil = not computed)
	// ref0 / refK evaluate a cited variable at cran 0 / at the current cran; built once.
	ref0, refK func(name string) (int, bool)
}

type varSlot struct {
	state byte // 0 unknown, 1 value, 2 no value
	val   int
}

func newResolver(e *Effect, nums []int) *resolver {
	r := &resolver{e: e, n: len(nums), nums: nums}
	if len(e.Vars) == 0 {
		return r
	}
	r.idx = make(map[string]int, len(e.Vars))
	for i, v := range e.Vars {
		r.idx[v.Name] = i
	}
	r.atK = make([]varSlot, len(e.Vars))
	r.at0 = make([]varSlot, len(e.Vars))
	r.basis = make([]varSlot, len(e.Vars))
	r.step = make([]int, len(e.Vars))
	r.lists = make([][]int, len(e.Vars))
	r.ref0 = func(name string) (int, bool) { return r.rank(r.idx[name], 0) }
	r.refK = func(name string) (int, bool) { return r.rank(r.idx[name], r.k) }
	return r
}

// setCran moves to cran k: the values cached for the previous cran are dropped.
func (r *resolver) setCran(k int) {
	if r.k == k && k != 0 {
		return
	}
	r.k = k
	for i := range r.atK {
		r.atK[i] = varSlot{}
	}
}

// rank gives the rank (1…n) of variable i at cran kk (kk is 0 or the current cran).
func (r *resolver) rank(i, kk int) (int, bool) {
	cache := r.atK
	if kk == 0 {
		cache = r.at0
	}
	if s := cache[i]; s.state != 0 {
		return s.val, s.state == 1
	}
	v, ok := r.compute(i, kk)
	if ok {
		cache[i] = varSlot{state: 1, val: v}
	} else {
		cache[i] = varSlot{state: 2}
	}
	return v, ok
}

func (r *resolver) compute(i, kk int) (int, bool) {
	if r.n == 0 {
		return 0, false
	}
	v := r.e.Vars[i]
	if v.IsList {
		lp := r.lists[i]
		if lp == nil {
			lp = listPositions(v, r.nums)
			if lp == nil {
				lp = []int{}
			}
			r.lists[i] = lp
		}
		if len(lp) == 0 {
			return 0, false
		}
		return lp[kk%len(lp)] + 1, true
	}
	ref := r.refK
	if kk == 0 {
		ref = r.ref0
	}
	if v.Step == nil { // follows: start re-evaluated at this cran
		d, ok := v.Start.eval(r.n, ref)
		if !ok {
			return 0, false
		}
		return rankOf(d, r.n), true
	}
	// advances: start(0) + step × k
	b := r.basis[i]
	if b.state == 0 {
		d, ok := v.Start.eval(r.n, r.ref0)
		p, ok2 := v.Step.eval(r.n, nil)
		if ok && ok2 {
			b = varSlot{state: 1, val: d}
			r.step[i] = p
		} else {
			b = varSlot{state: 2}
		}
		r.basis[i] = b
	}
	if b.state == 2 {
		return 0, false
	}
	// 64-bit: the same rank on every machine, whatever the cran
	return int(posMod64(int64(b.val)+int64(r.step[i])*int64(kk)-1, int64(r.n))) + 1, true
}

// rankOf is the rank (1…n) a value designates: ((v-1) mod n)+1 with a positive remainder (§12.2).
func rankOf(v, n int) int { return posMod(v-1, n) + 1 }

// position resolves a target to the position (0-based) it designates at the current cran.
func (r *resolver) position(t Target) (int, bool) {
	n := r.n
	switch t.Kind {
	case TargetGn:
		if i := indexOf(r.nums, t.N); i >= 0 {
			return i, true
		}
	case TargetGN:
		if n > 0 {
			return n - 1, true
		}
	case TargetVar:
		if n == 0 {
			return 0, false
		}
		i, ok := r.idx[t.Var]
		if !ok {
			return 0, false
		}
		rk, ok := r.rank(i, r.k)
		if !ok {
			return 0, false
		}
		return posMod(rk-1+t.Offset, n), true
	}
	return 0, false
}

// VarPositions gives, for the tick `tick` of a zone with groups numbered nums, the real number
// of the group each variable designates (0 = none), by variable name (§11.3 `vars`).
func (e Effect) VarPositions(nums []int, tick int) map[string]int {
	out := make(map[string]int, len(e.Vars))
	if len(e.Vars) == 0 {
		return out
	}
	live := e.liveSteps(nums)
	k := 0
	if len(live) > 0 {
		if !e.Loop {
			if lt := e.lastTick(live); tick > lt {
				tick = lt
			}
		}
		if tick < 0 {
			tick = 0
		}
		k = e.playedAt(tick, live).k
	}
	r := newResolver(&e, nums)
	r.setCran(k)
	for i, v := range e.Vars {
		if rk, ok := r.rank(i, k); ok {
			out[v.Name] = nums[rk-1]
		} else {
			out[v.Name] = 0
		}
	}
	return out
}

func posMod64(a, m int64) int64 {
	if m <= 0 {
		return 0
	}
	return ((a % m) + m) % m
}

func posMod(a, m int) int {
	if m <= 0 {
		return 0
	}
	return ((a % m) + m) % m
}

// apply plays a step forward on a zone (used for POST, applied after the replayed ticks).
func (e Effect) apply(z *zoneState, s Step, r *resolver) {
	for _, a := range s {
		l := look{col: a.Color, pct: a.Pct, off: a.Off}
		if a.Target.Kind == TargetG0 {
			z.base, z.baseSet = l, true
			for i := range z.groups {
				z.groups[i], z.set[i] = l, true
			}
			continue
		}
		if p, ok := r.position(a.Target); ok {
			z.groups[p], z.set[p] = l, true
		}
	}
}

// g0Of reports whether a step writes g0 and the last such look.
func g0Of(s Step) (look, bool) {
	var l look
	found := false
	for _, a := range s {
		if a.Target.Kind == TargetG0 {
			l, found = look{col: a.Color, pct: a.Pct, off: a.Off}, true
		}
	}
	return l, found
}

// latestG0Tick is the last tick ≤ tick that plays a step with g0 (-1 when none).
func (e Effect) latestG0Tick(tick int, live []int) int {
	best := -1
	pre := 0
	if e.HasPre {
		pre = 1
		if _, ok := g0Of(e.Pre); ok && tick >= 0 {
			best = 0
		}
	}
	m := len(live)
	cycle := m
	if e.HasLast {
		cycle++
	}
	for idx := 0; idx < cycle; idx++ {
		s := e.Last
		if idx < m {
			s = e.Steps[live[idx]]
		}
		if _, ok := g0Of(s); !ok {
			continue
		}
		if t0 := pre + idx; tick >= t0 {
			if t := t0 + (tick-t0)/cycle*cycle; t > best {
				best = t
			}
		}
	}
	return best
}

// applyBackward applies the actions of s from the last to the first on a zone being built
// backwards: an action only writes what no later action decided (the last write wins, g0 decides
// every group still undecided and, once, the base).
func (z *zoneState) applyBackward(s Step, r *resolver, decided []bool, left *int, baseDecided *bool) {
	for i := len(s) - 1; i >= 0; i-- {
		a := s[i]
		l := look{col: a.Color, pct: a.Pct, off: a.Off}
		if a.Target.Kind == TargetG0 {
			if !*baseDecided {
				z.base, z.baseSet, *baseDecided = l, true, true
			}
			for p := range z.groups {
				if !decided[p] {
					z.groups[p], z.set[p], decided[p] = l, true, true
					*left--
				}
			}
			continue
		}
		if p, ok := r.position(a.Target); ok && !decided[p] {
			z.groups[p], z.set[p], decided[p] = l, true, true
			*left--
		}
	}
}

// replay computes the zone after ticks 0..tick. Equivalent to playing every tick on a fresh
// zone, but done backwards from `tick`: each group (and the base) takes the last write, and
// the walk stops as soon as everything is decided — one tick for an effect that starts every
// step with g0, whatever the tick (§12.6 cost). It returns the resolver left at the cran of
// the last tick.
func (e Effect) replay(tick int, nums []int) (z zoneState, r *resolver, ok bool) {
	live := e.liveSteps(nums)
	if len(live) == 0 {
		return zoneState{}, nil, false
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
	r = newResolver(&e, nums)
	decided := make([]bool, len(nums))
	left := len(nums)
	// The base is known from the last tick that played g0; before it nothing matters.
	floor := e.latestG0Tick(tick, live)
	baseDecided := floor < 0 // no g0 played: no base, nothing to look for
	if floor < 0 {
		floor = 0
	}
	last := e.playedAt(tick, live).k
	for t := tick; t >= floor; t-- {
		p := e.playedAt(t, live)
		r.setCran(p.k)
		switch p.kind {
		case 'P':
			z.applyBackward(e.Pre, r, decided, &left, &baseDecided)
		case 'S':
			z.applyBackward(e.Steps[p.step], r, decided, &left, &baseDecided)
		case 'L':
			z.applyBackward(e.Last, r, decided, &left, &baseDecided)
		}
		if left == 0 && baseDecided {
			break
		}
	}
	r.setCran(last) // RenderPost applies POST at the cran of the last tick
	return z, r, true
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
	z, r, ok := e.replay(lastTick, nums)
	if !ok {
		return Frame{}, false
	}
	e.apply(&z, e.Post, r)
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
