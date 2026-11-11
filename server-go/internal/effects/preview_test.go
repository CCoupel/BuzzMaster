package effects

// Aperçu calculé par le serveur (#299, contrat light-effects.md §7.3) : les trames viennent de `Render`, le navigateur ne fait
// que les rejouer (aucun second interpréteur). Contexte simulé : chronomètre de 20 s, thème #4F46E5, équipe #E53935, tick ×1.
// Aucune attente : calcul pur.

import (
	"fmt"
	"testing"
)

func pvAt(r pvResult, ms int) []Lk { // état affiché à l'instant ms : la dernière trame listée à at ≤ ms
	var cur []Lk
	for _, f := range r.Frames {
		if f.At <= ms {
			cur = f.Groups
		}
	}
	return cur
}

func TestPreview_Chenillard4_OneFramePerTick_HeadWalksFromG1_FundAtTheme(t *testing.T) {
	e := nativeEffect(t, "chenillard")
	r := preview(e, 4, "chrono")
	if r.Groups != 4 || r.DurationMs != previewTotalMs || !r.Loop {
		t.Fatalf("groups=4, duration_ms=20000, loop=true : %+v", r)
	}
	if len(r.Frames) != 20 {
		t.Fatalf("une trame par tick de 1 s sur 20 s (chaque tick diffère du précédent) : %d", len(r.Frames))
	}
	for k, f := range r.Frames {
		if f.At != k*1000 {
			t.Fatalf("trame %d à %d ms, attendu %d", k, f.At, k*1000)
		}
		if len(f.Groups) != 4 {
			t.Fatalf("trame %d : une entrée par groupe (4) : %d", k, len(f.Groups))
		}
		head := k%4 + 1
		for p := 1; p <= 4; p++ {
			g := f.Groups[p-1]
			if p == head && (g.Color == previewTheme || g.Intensity < 254) {
				t.Errorf("tick %d : tête en g%d à la couleur d'urgence : %+v", k, p, g)
			}
			if p != head && (g.Color != previewTheme || g.Intensity != 255) {
				t.Errorf("tick %d : g%d garde le thème à 100 %% : %+v", k, p, g)
			}
		}
	}
}

func TestPreview_Chenillard_HeadColourFollowsTheSimulatedChrono_White_Orange_Red(t *testing.T) {
	e := nativeEffect(t, "chenillard")
	r := preview(e, 4, "chrono")
	head := func(ms int) Lk { g := pvAt(r, ms); return g[(ms/1000)%4] }
	for _, c := range []struct {
		ms   int
		want [3]int
	}{{3000, cW}, {12000, cOr}, {17000, cRed}} {
		if got := head(c.ms); got.Color != c.want {
			t.Errorf("à %d ms (reste %d s) : tête %v, attendu %v", c.ms, (previewTotalMs-c.ms)/1000, got.Color, c.want)
		}
	}
}

func TestPreview_FramesAreExactlyRender_NoSecondInterpreter_AllNativesAllSizes(t *testing.T) {
	for _, id := range []string{"pulsation", "chenillard", "chenillard-traine", "couleur-equipe", "degrade"} {
		e := nativeEffect(t, id)
		moment := e.Moments[0]
		for _, n := range []int{1, 3, 4, 6, 8, 12} {
			r := preview(e, n, moment)
			for _, f := range r.Frames {
				tick := f.At / e.TickMs
				if f.At%e.TickMs != 0 {
					t.Fatalf("%s N=%d : trame hors d'un début de tick (%d ms, tick %d ms)", id, n, f.At, e.TickMs)
				}
				v := renderView(e, previewCtx(n, moment, f.At), tick)
				if len(f.Groups) != n {
					t.Fatalf("%s N=%d @%d : %d entrées", id, n, f.At, len(f.Groups))
				}
				for p := 1; p <= n; p++ {
					want := v.effective(p)
					if want.Absent {
						continue // la façon d'afficher un groupe sans état n'est pas contractuelle
					}
					got := f.Groups[p-1]
					if got.Color != want.Color || got.Intensity != want.Intensity {
						t.Errorf("%s N=%d @%d g%d : aperçu %+v, Render %+v", id, n, f.At, p, got, want)
					}
				}
			}
		}
	}
}

func TestPreview_FrameListedOnlyWhenItDiffersFromThePrevious(t *testing.T) {
	// un effet constant : une seule trame, à 0
	e := mustParse(t, "id: fixe\nnom: Fixe\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g0: [\"#FF0000\", 100] }\n")
	r := preview(e, 3, "calm")
	if len(r.Frames) != 1 || r.Frames[0].At != 0 {
		t.Fatalf("effet constant : une trame à 0 : %+v", r.Frames)
	}
	// deux pas alternés : une trame à chaque tick
	e2 := mustParse(t, "id: alt\nnom: Alt\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g0: [\"#FF0000\", 100] }\n  - { g0: [\"#00FF00\", 100] }\n")
	if r := preview(e2, 3, "calm"); len(r.Frames) != 20 {
		t.Errorf("alternance à chaque tick : 20 trames sur 20 s : %d", len(r.Frames))
	}
	// deux pas identiques : aucune trame de plus
	e3 := mustParse(t, "id: idem\nnom: Idem\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g0: [\"#FF0000\", 100] }\n  - { g0: [\"#FF0000\", 100] }\n")
	if r := preview(e3, 3, "calm"); len(r.Frames) != 1 {
		t.Errorf("pas identiques : une trame : %d", len(r.Frames))
	}
}

func TestPreview_AtMost400Frames_ShortestTickGivesAnAlternationEvery250ms(t *testing.T) {
	e := mustParse(t, "id: vite\nnom: Vite\nversion: 2\nmoments: [calm]\nduree_tick_ms: 250\npas:\n  - { g0: [\"#FF0000\", 100] }\n  - { g0: [\"#00FF00\", 100] }\n")
	r := preview(e, 4, "calm")
	if len(r.Frames) != 80 || len(r.Frames) > previewMaxFrames {
		t.Errorf("20 s à 250 ms : 80 trames, jamais plus de 400 : %d", len(r.Frames))
	}
}

func TestPreview_ChronoMomentSimulatesTwentySeconds_CalmIsTimeless(t *testing.T) {
	// pulsation (chrono) : la phase basse change de couleur avec le temps restant (blanc, orange, rouge)
	r := preview(nativeEffect(t, "pulsation"), 4, "chrono")
	seen := map[[3]int]bool{}
	for _, f := range r.Frames {
		seen[f.Groups[0].Color] = true
	}
	if !seen[cW] || !seen[cOr] || !seen[cRed] {
		t.Errorf("le chronomètre simulé de 20 s fait passer la phase basse par blanc, orange et rouge : %v", seen)
	}
	// dégradé (calm) : tick de 60 s > 20 s : une seule trame, une couleur par groupe, cycle de 4 couleurs
	d := preview(nativeEffect(t, "degrade"), 6, "calm")
	if len(d.Frames) != 1 {
		t.Fatalf("dégradé : scène fixe, une trame : %d", len(d.Frames))
	}
	g := d.Frames[0].Groups
	if g[0].Color == g[1].Color || g[0].Color != g[4].Color || g[1].Color != g[5].Color {
		t.Errorf("une couleur par groupe, cycle de 4 : %+v", g)
	}
}

func TestPreview_TeamColourIsTheSimulatedTeam(t *testing.T) {
	r := preview(nativeEffect(t, "couleur-equipe"), 4, "score")
	g := pvAt(r, 0)
	for p, l := range g {
		if l.Color != previewTeam {
			t.Errorf("g%d : couleur de l'équipe simulée #E53935 : %+v", p+1, l)
		}
	}
}

func TestPreview_NoLoop_EffectStopsAtItsLastTick_LoopFalse(t *testing.T) {
	e := mustParse(t, "id: unefois\nnom: Une fois\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\nboucle: non\npas:\n  - { g0: [\"#FF0000\", 100] }\n  - { g0: [\"#00FF00\", 100] }\n  - { g0: [\"#0000FF\", 100] }\n")
	r := preview(e, 3, "calm")
	if r.Loop {
		t.Error("boucle: non ⇒ loop=false")
	}
	if len(r.Frames) != 3 || r.Frames[2].At != 2000 {
		t.Errorf("trois pas puis l'état final tenu (pas de nouvelle trame) : %+v", r.Frames)
	}
}

func TestPreview_PreComesFirst_ThenTheLoop(t *testing.T) {
	e := mustParse(t, "id: avecpre\nnom: Avec PRE\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npre: { g0: [\"#0000FF\", 100] }\npas:\n  - { g1: [\"#FF0000\", 100] }\n  - { g1: [\"#00FF00\", 100] }\n")
	r := preview(e, 2, "calm")
	if len(r.Frames) < 2 || r.Frames[0].At != 0 {
		t.Fatalf("trames : %+v", r.Frames)
	}
	// PRE puis pas 0 sans pas 0 à 0 s (§4.5 : sans PRE... avec PRE le premier pas suit le tick 0) : la première trame porte le fond du PRE
	if g := r.Frames[0].Groups; g[1].Color != [3]int{0, 0, 255} {
		t.Errorf("PRE appliqué au démarrage : g2 au fond bleu : %+v", g)
	}
}

func TestPreview_Deterministic_SameInputSameOutput_NeverWritesAnything(t *testing.T) {
	e := nativeEffect(t, "chenillard")
	a, b := preview(e, 6, "chrono"), preview(e, 6, "chrono")
	if fmt.Sprint(a) != fmt.Sprint(b) {
		t.Error("l'aperçu est une fonction pure de (effet, groupes, moment)")
	}
}

func TestPreview_FadeIsAlwaysZero(t *testing.T) {
	e := nativeEffect(t, "chenillard")
	for _, f := range Preview(e, 4, Moment("chrono")).Frames {
		for _, g := range f.Groups {
			if g.FadeMs != 0 {
				t.Fatalf("fade_ms = 0 (révision 3) : %+v", g)
			}
		}
	}
}
