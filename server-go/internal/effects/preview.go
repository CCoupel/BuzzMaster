package effects

import "time"

// Preview (v11.7.0, #299 — contracts/light-effects.md §7.3): the frames of an effect computed
// with Render, tick by tick, in a simulated context — the browser only replays them (there is no
// second interpreter). Nothing is played on the bulbs.

// Simulated context of a preview (§7.3, §7.4).
var (
	PreviewTheme = [3]int{0x4F, 0x46, 0xE5}
	PreviewTeam  = [3]int{0xE5, 0x39, 0x35}
)

const (
	// PreviewSpanChrono / PreviewSpanScore are the lengths covered by a preview, fixed by the
	// moment (§7.3): 20 s for chrono and calm, 5 s for score.
	PreviewSpanChrono = 20 * time.Second
	PreviewSpanScore  = 5 * time.Second
	// PreviewMaxFrames caps the number of listed frames.
	PreviewMaxFrames = 400
	// previewSceneIntensity is the intensity shown for a group the effect never wrote (the
	// ordinary scene stays: white, as an idle room).
	previewSceneIntensity = 200
)

// PreviewSpanOf is the duration of a preview at moment m.
func PreviewSpanOf(m Moment) time.Duration {
	if m == MomentScore {
		return PreviewSpanScore
	}
	return PreviewSpanChrono
}

// PreviewChronoS is the simulated chrono, in seconds: 20 at the chrono moment, 0 elsewhere.
func PreviewChronoS(m Moment) int {
	if m == MomentChrono {
		return int(PreviewSpanChrono / time.Second)
	}
	return 0
}

// PreviewGroup is the state of one group in a preview: resolved RGB and intensity 0-255.
type PreviewGroup struct {
	Color     [3]int `json:"color"`
	Intensity int    `json:"intensity"`
}

// PreviewFrame is one listed frame: the state of every group from AtMs on, the tick it belongs
// to and the step played at that tick ("pre", "pas:<n>" or "dernier" — never "post").
type PreviewFrame struct {
	AtMs   int            `json:"at_ms"`
	Tick   int            `json:"tick"`
	Step   string         `json:"step"`
	Groups []PreviewGroup `json:"groups"`
	// Vars: for each declared variable, the real number of the group it designates at this tick
	// (0 = none) — additive field of v11.8.0 (§11.3); `{}` without variable.
	Vars map[string]int `json:"vars"`
}

// PreviewResult is the answer of GET /api/lighting/effects/{id}/preview.
type PreviewResult struct {
	ID         string         `json:"id"`
	Moment     string         `json:"moment"`
	Groups     int            `json:"groups"`
	TickMs     int            `json:"tick_ms"`
	DurationMs int            `json:"duration_ms"`
	ChronoS    int            `json:"chrono_s"`
	Loop       bool           `json:"loop"`
	Frames     []PreviewFrame `json:"frames"`
}

// Preview renders e for n groups at moment m (contract §7.3): ONE frame per tick, in order,
// even when identical to the previous one, plus an extra frame (same tick, same step) when a
// named colour changes between two ticks — the urgency colour at 10 s and 5 s of the simulated
// chrono. The span is fixed by the moment; a `boucle: non` effect stops after its round. At most
// PreviewMaxFrames (the preview is then truncated and its duration shortened). Speed is ×1.
func Preview(e Effect, n int, m Moment) PreviewResult {
	res := PreviewResult{ID: e.ID, Moment: string(m), Groups: n, TickMs: e.TickMs, ChronoS: PreviewChronoS(m), Loop: e.Loop, Frames: []PreviewFrame{}}
	tick := time.Duration(e.TickMs) * time.Millisecond
	if tick <= 0 {
		tick = time.Second
	}
	nums := make([]int, n)
	for i := range nums {
		nums[i] = i + 1
	}
	live := e.liveSteps(nums)
	span := PreviewSpanOf(m)
	if !e.Loop && len(live) > 0 {
		if round := time.Duration(e.lastTick(live)+1) * tick; round < span {
			span = round
		}
	}
	res.DurationMs = int(span / time.Millisecond)
	if len(live) == 0 {
		return res
	}

	chronoTotal := time.Duration(res.ChronoS) * time.Second
	render := func(t int, atTime time.Duration) []PreviewGroup {
		ctx := Context{Groups: n, Nums: nums, Moment: m, Total: chronoTotal,
			Scene: [3]int{255, 255, 255}, Theme: PreviewTheme, Team: PreviewTeam}
		if chronoTotal > 0 {
			if ctx.Remaining = chronoTotal - atTime; ctx.Remaining < 0 {
				ctx.Remaining = 0
			}
		}
		return previewGroups(Render(e, ctx, t), n)
	}
	// Instants where a named colour changes (urgency: 10 s and 5 s remaining).
	var changes []time.Duration
	if m == MomentChrono {
		changes = []time.Duration{chronoTotal - 10*time.Second, chronoTotal - 5*time.Second}
	}

	for t := 0; ; t++ {
		at := time.Duration(t) * tick
		if at >= span {
			break
		}
		if len(res.Frames) >= PreviewMaxFrames {
			res.DurationMs = int(at / time.Millisecond) // truncated: shortened accordingly
			break
		}
		step := e.stepName(e.playedAt(t, live))
		base := render(t, at)
		vars := e.VarPositions(nums, t)
		res.Frames = append(res.Frames, PreviewFrame{AtMs: int(at / time.Millisecond), Tick: t, Step: step, Groups: base, Vars: vars})
		for _, c := range changes {
			if c > at && c < at+tick && c < span && len(res.Frames) < PreviewMaxFrames {
				if g := render(t, c); !samePreview(g, base) {
					res.Frames = append(res.Frames, PreviewFrame{AtMs: int(c / time.Millisecond), Tick: t, Step: step, Groups: g, Vars: vars})
					base = g
				}
			}
		}
	}
	return res
}

// stepName names what a tick plays (§7.3).
func (e Effect) stepName(p playedTick) string {
	switch p.kind {
	case 'P':
		return "pre"
	case 'L':
		return "dernier"
	}
	return "pas:" + itoaInt(p.step)
}

func itoaInt(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// previewGroups gives, for every group, the state it shows: its own, else the state of g0,
// else the ordinary scene.
func previewGroups(f Frame, n int) []PreviewGroup {
	out := make([]PreviewGroup, n)
	for i := 0; i < n; i++ {
		var g GroupLook
		switch {
		case i < len(f.Groups) && !f.Groups[i].Absent:
			g = f.Groups[i]
		case f.Base != nil:
			g = *f.Base
		default:
			g = GroupLook{Color: [3]int{255, 255, 255}, Intensity: previewSceneIntensity}
		}
		out[i] = PreviewGroup{Color: g.Color, Intensity: g.Intensity}
	}
	return out
}

func samePreview(a, b []PreviewGroup) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
