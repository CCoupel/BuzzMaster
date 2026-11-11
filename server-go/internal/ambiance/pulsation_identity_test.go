package ambiance

import (
	"io/fs"
	"testing"
	"time"

	"buzzcontrol/assets"
	"buzzcontrol/internal/effects"
)

// Identity of the native `pulsation` effect with the v11.6.0 chrono pulse (contract
// light-effects.md §5.5): same colours, intensities 255 / 128 / 77, 1 s cycle, 250 / 750 ms
// split at 5 s or less, fade 0. The reference below is a FROZEN copy of the v11.6.0 code
// (ChronoPulseColor + the tier and phase rules of runChronoPulse) — it deliberately does
// not call anything of the production package, so removing the old code cannot weaken it.

func v1160PulseColor(theme [3]int, tier int, high bool) ([3]int, int) {
	if high {
		return theme, 255
	}
	switch tier {
	case 3:
		return [3]int{230, 30, 30}, 77
	case 2:
		return [3]int{255, 170, 0}, 128
	default:
		return theme, 128
	}
}

// v1160Pulse is what the v11.6.0 pulse rendered at `remaining` whole seconds and wall
// clock wallMs: tier 1 above 10 s, 2 at 10 s or less, 3 at 5 s or less; the high phase
// lasts 500 ms of the 1000 ms cycle (250 ms at tier 3).
func v1160Pulse(theme [3]int, remaining int, wallMs int64) ([3]int, int) {
	tier := 1
	if remaining <= 10 {
		tier = 2
	}
	if remaining <= 5 {
		tier = 3
	}
	highMs := int64(500)
	if tier == 3 {
		highMs = 250
	}
	return v1160PulseColor(theme, tier, wallMs%1000 < highMs)
}

func TestPulsationNative_RendersIdenticallyToV1160_OverWholeRange(t *testing.T) {
	t.Parallel()
	sub, err := fs.Sub(assets.EffectsAssets, "effects")
	if err != nil {
		t.Fatal(err)
	}
	en, ok := effects.LoadDir(sub, "").Get("pulsation")
	if !ok {
		t.Fatal("pulsation natif absent du catalogue embarqué")
	}
	for _, theme := range [][3]int{{255, 255, 255}, {79, 70, 229}, {0, 200, 90}} {
		for remaining := 0; remaining <= 40; remaining++ {
			for wallMs := int64(0); wallMs < 2000; wallMs += 25 {
				f := effects.Render(en.Effect, effects.Context{
					Groups: 0, Moment: effects.MomentChrono, Theme: theme, Scene: theme,
					Remaining: time.Duration(remaining) * time.Second, Total: 60 * time.Second, WallMs: wallMs,
				}, 0)
				if len(f) != 1 {
					t.Fatalf("target all : une seule entrée, obtenu %d", len(f))
				}
				wantC, wantI := v1160Pulse(theme, remaining, wallMs)
				if f[0].Color != wantC || f[0].Intensity != wantI || f[0].FadeMs != 0 {
					t.Fatalf("theme %v reste %d s, horloge %d ms : %+v, v11.6.0 = %v/%d fondu 0", theme, remaining, wallMs, f[0], wantC, wantI)
				}
			}
		}
	}
}
