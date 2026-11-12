package effects

import (
	"fmt"
	"reflect"
	"testing"
)

// The backward replay must give exactly what playing every tick forward gives.
func forwardRender(e Effect, ctx Context, tick int) Frame {
	nums := ctx.groupNums()
	live := e.liveSteps(nums)
	if len(live) == 0 {
		return Frame{}
	}
	if !e.Loop {
		if lt := e.lastTick(live); tick > lt {
			tick = lt
		}
	}
	if tick < 0 {
		tick = 0
	}
	z := zoneState{groups: make([]look, len(nums)), set: make([]bool, len(nums))}
	r := newResolver(&e, nums)
	for t := 0; t <= tick; t++ {
		p := e.playedAt(t, live)
		r.setCran(p.k)
		switch p.kind {
		case 'P':
			e.apply(&z, e.Pre, r)
		case 'S':
			e.apply(&z, e.Steps[p.step], r)
		case 'L':
			e.apply(&z, e.Last, r)
		}
	}
	return z.frame(ctx)
}

func forwardPost(e Effect, ctx Context, last int) (Frame, bool) {
	if !e.HasPost {
		return Frame{}, false
	}
	nums := ctx.groupNums()
	live := e.liveSteps(nums)
	if len(live) == 0 {
		return Frame{}, false
	}
	if !e.Loop {
		if lt := e.lastTick(live); last > lt {
			last = lt
		}
	}
	z := zoneState{groups: make([]look, len(nums)), set: make([]bool, len(nums))}
	r := newResolver(&e, nums)
	for t := 0; t <= last; t++ {
		p := e.playedAt(t, live)
		r.setCran(p.k)
		switch p.kind {
		case 'P':
			e.apply(&z, e.Pre, r)
		case 'S':
			e.apply(&z, e.Steps[p.step], r)
		case 'L':
			e.apply(&z, e.Last, r)
		}
	}
	e.apply(&z, e.Post, r)
	return z.frame(ctx), true
}

func TestReplayBackwardEqualsForward_Dev(t *testing.T) {
	t.Parallel()
	bodies := map[string]string{
		"sans g0":                 "variables:\n  gA: 1; +1\npas:\n  - { gA: [urgence, 100] }\n  - { g2: [theme, 50], gA+1: [scene, 70] }\n",
		"g0 dans pre seulement":   "variables:\n  gA: 1; +1\npre:\n  { g0: [theme, 100] }\npas:\n  - { gA: [urgence, 100] }\n",
		"g0 dans un pas sur deux": "variables:\n  gA: 2; +1\n  gB: gA+1\npas:\n  - { g0: [theme, 30], gA: [urgence, 100] }\n  - { gB: [scene, 60], g*: [eteint] }\n  - { g1: [theme, 100] }\n",
		"dernier et post":         "variables:\n  gA: N/2; -1\npre:\n  { g1: [theme, 100] }\npas:\n  - { gA: [urgence, 100] }\ndernier:\n  { g0: [scene, 40], gA: [theme, 100] }\npost:\n  { g* : [theme, 100] }\n",
		"liste":                   "variables:\n  gC: [g1, g4, g3]\n  gD: gC+1\npas:\n  - { gC: [theme, 100], gD: [scene, 50] }\n  - { g0: [eteint] }\n",
		"division par zéro":       "variables:\n  gA: 6/(N-3); +1\npas:\n  - { g0: [theme, 100], gA: [urgence, 100] }\n",
	}
	for name, body := range bodies {
		for _, loop := range []string{"", "boucle: non\n"} {
			src := fx(loop + body)
			e, errs := Parse([]byte(src))
			if len(errs) > 0 {
				t.Fatalf("%s : %+v", name, errs)
			}
			for n := 0; n <= 12; n++ {
				ctx := ctxN(n).toContext(e)
				for tick := 0; tick < 40; tick++ {
					if got, want := Render(e, ctx, tick), forwardRender(e, ctx, tick); !reflect.DeepEqual(got, want) {
						t.Fatalf("%s (%q) N=%d tick %d : %+v ≠ %+v", name, loop, n, tick, got, want)
					}
				}
				for _, lt := range []int{0, 1, 5, 17} {
					got, ok := RenderPost(e, ctx, lt)
					want, wok := forwardPost(e, ctx, lt)
					if ok != wok || !reflect.DeepEqual(got, want) {
						t.Fatalf("%s (%q) N=%d post après le tick %d : %+v ≠ %+v", name, loop, n, lt, got, want)
					}
				}
			}
		}
	}
	_ = fmt.Sprint
}
