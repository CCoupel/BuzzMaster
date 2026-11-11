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
	// PreviewSpan is the length covered by a preview: PRE, then the loops over 20 s.
	PreviewSpan = 20 * time.Second
	// PreviewMaxFrames caps the number of listed frames.
	PreviewMaxFrames = 400
	// previewSceneIntensity is the intensity shown for a group the effect never wrote (the
	// ordinary scene stays: white, as an idle room).
	previewSceneIntensity = 200
)

// PreviewFrame is one listed frame: the state of every group from AtMs on.
type PreviewFrame struct {
	AtMs   int         `json:"at_ms"`
	Groups []GroupLook `json:"groups"`
}

// PreviewResult is the answer of GET /api/lighting/effects/{id}/preview.
type PreviewResult struct {
	ID         string         `json:"id"`
	Groups     int            `json:"groups"`
	DurationMs int            `json:"duration_ms"`
	Loop       bool           `json:"loop"`
	Frames     []PreviewFrame `json:"frames"`
}

// Preview renders e for n groups at moment m over PreviewSpan (or one round for a `boucle: non`
// effect). A frame is listed only when it differs from the previous one; at most
// PreviewMaxFrames. Speed is ×1 (the catalogue previews the effect as written).
func Preview(e Effect, n int, m Moment) PreviewResult {
	res := PreviewResult{ID: e.ID, Groups: n, Loop: e.Loop, Frames: []PreviewFrame{}}
	tick := time.Duration(e.TickMs) * time.Millisecond
	if tick <= 0 {
		tick = time.Second
	}
	nums := make([]int, n)
	for i := range nums {
		nums[i] = i + 1
	}
	live := e.liveSteps(nums)
	last := int(PreviewSpan / tick)
	if !e.Loop && len(live) > 0 {
		if lt := e.lastTick(live); lt < last {
			last = lt
		}
	}
	span := PreviewSpan
	if !e.Loop && time.Duration(last+1)*tick < span {
		span = time.Duration(last+1) * tick
	}
	res.DurationMs = int(span / time.Millisecond)

	var prev []GroupLook
	for t := 0; t <= last && len(res.Frames) < PreviewMaxFrames; t++ {
		at := time.Duration(t) * tick
		if at >= span {
			break
		}
		ctx := Context{
			Groups: n, Nums: nums, Moment: m,
			Remaining: PreviewSpan - at, Total: PreviewSpan,
			Scene: [3]int{255, 255, 255}, Theme: PreviewTheme, Team: PreviewTeam,
		}
		if ctx.Remaining < 0 {
			ctx.Remaining = 0
		}
		f := Render(e, ctx, t)
		if f.Base == nil && f.Groups == nil {
			break // not applicable to this number of groups: nothing to show
		}
		cur := effectiveGroups(f, n)
		if prev != nil && sameLooks(prev, cur) {
			continue
		}
		res.Frames = append(res.Frames, PreviewFrame{AtMs: int(at / time.Millisecond), Groups: cur})
		prev = cur
	}
	return res
}

// effectiveGroups gives, for every group, the state it shows: its own, else the state of g0,
// else the ordinary scene.
func effectiveGroups(f Frame, n int) []GroupLook {
	out := make([]GroupLook, n)
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
		g.Absent = false
		out[i] = g
	}
	return out
}

func sameLooks(a, b []GroupLook) bool {
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
