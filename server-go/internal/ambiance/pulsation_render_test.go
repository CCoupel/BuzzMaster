package ambiance

// Rendu de la pulsation native (contrat light-effects.md §5.5, révision 3) — REMPLACE pulsation_identity_test.go : le rendu
// n'est volontairement plus identique à v11.6.0 (décision utilisateur). Phase haute : thème à 100 % ; phase basse : couleur
// `urgence` à 50 % (blanc au-delà de 10 s, orange à 10 s ou moins, rouge à 5 s ou moins) ; 500 ms chacune ; une seule
// écriture de groupe par phase (g0 seul). Balayage : thèmes × temps restant × numéro de tick, sur le catalogue embarqué.

import (
	"io/fs"
	"testing"
	"time"

	"buzzcontrol/assets"
	"buzzcontrol/internal/effects"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

// expectedPulse : ce que le §5.5 demande au tick donné (tick pair = phase haute, impair = phase basse).
func expectedPulse(theme [3]int, remaining int, tick int) ([3]int, int) {
	if tick%2 == 0 {
		return theme, 255
	}
	switch {
	case remaining <= 5:
		return [3]int{230, 30, 30}, 128
	case remaining <= 10:
		return [3]int{255, 170, 0}, 128
	default:
		return [3]int{255, 255, 255}, 128
	}
}

func TestPulsationNative_RendersTheNewSpec_OverWholeRange(t *testing.T) {
	t.Parallel()
	sub, err := fs.Sub(assets.EffectsAssets, "effects")
	if err != nil {
		t.Fatal(err)
	}
	en, ok := effects.LoadDir(sub, "").Get("pulsation")
	if !ok {
		t.Fatal("pulsation natif absent du catalogue embarqué")
	}
	if en.Effect.TickMs != 500 {
		t.Fatalf("tick de 500 ms (deux phases de 500 ms) : %d", en.Effect.TickMs)
	}
	for _, theme := range [][3]int{{255, 255, 255}, {79, 70, 229}, {0, 200, 90}} {
		for remaining := 0; remaining <= 40; remaining++ {
			for tick := 0; tick < 6; tick++ {
				f := effects.Render(en.Effect, effects.Context{
					Groups: 6, Moment: effects.MomentChrono, Theme: theme, Scene: theme,
					Remaining: time.Duration(remaining) * time.Second, Total: 60 * time.Second,
				}, tick)
				if f.Base == nil {
					t.Fatalf("la pulsation écrit g0 : une base à chaque tick (reste %d s, tick %d)", remaining, tick)
				}
				for i, g := range f.Groups {
					if !g.Absent {
						t.Fatalf("reste %d s tick %d : le groupe %d a un état propre (une seule écriture de groupe par phase)", remaining, tick, i+1)
					}
				}
				wantC, wantI := expectedPulse(theme, remaining, tick)
				if f.Base.Color != wantC || f.Base.Intensity < wantI-1 || f.Base.Intensity > wantI+1 || f.Base.FadeMs != 0 {
					t.Fatalf("thème %v reste %d s tick %d : %+v, attendu %v / %d fondu 0", theme, remaining, tick, *f.Base, wantC, wantI)
				}
			}
		}
	}
}

func TestPulsationNative_DiffersFromV1160_WhereTheDecisionSaysSo(t *testing.T) {
	// v11.6.0 : > 10 s phase basse = thème 50 % ; ≤ 5 s rouge 30 % battement 250/750 ms. Révision 3 : blanc 50 % ; rouge 50 % 500/500.
	c, i := expectedPulse([3]int{79, 70, 229}, 20, 1)
	if c == [3]int{79, 70, 229} || i != 128 {
		t.Errorf("phase basse au-delà de 10 s : blanc à 50 %% (pas le thème) : %v/%d", c, i)
	}
	if _, i := expectedPulse([3]int{79, 70, 229}, 3, 1); i == 77 {
		t.Error("rouge à 50 %, plus à 30 % (77/255)")
	}
}

func TestPulsationNative_ThroughTheReader_ThemeFullThenUrgencyHalf_WithInjectedClock(t *testing.T) {
	env := tw223New(t)
	env.setup(nil) // aucune groupe : la pulsation reste applicable (g0)
	env.eng.StartImmediate(20)
	t.Cleanup(func() { env.eng.SetPhase(game.PhaseStopped) })
	env.step()
	if z := env.c.Scene(lighting.Event{Kind: lighting.KindRunning}).Zones[0]; z.Intensity != 255 {
		t.Errorf("tick 0 : phase haute 100 %% : %+v", z)
	}
	env.now = env.now.Add(500 * time.Millisecond)
	env.step()
	if z := env.c.Scene(lighting.Event{Kind: lighting.KindRunning}).Zones[0]; z.Intensity < 127 || z.Intensity > 129 {
		t.Errorf("tick 1 : phase basse 50 %% : %+v", z)
	}
	env.now = env.now.Add(500 * time.Millisecond)
	env.step()
	if z := env.c.Scene(lighting.Event{Kind: lighting.KindRunning}).Zones[0]; z.Intensity != 255 {
		t.Errorf("tick 2 : phase haute : %+v", z)
	}
}
