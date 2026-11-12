package effects

// Aperçu calculé par le serveur (#299, contrat light-effects.md §7.3 compléments e9d86cd7) : les trames viennent de `Render`, le
// navigateur ne fait que les rejouer (aucun second interpréteur). UNE TRAME PAR TICK (`tick`, `step`), durée fixée par le moment
// (20 s ; 5 s pour score), `chrono_s`, `tick_ms`, `moment`. Contexte simulé : thème #4F46E5, équipe #E53935, tick ×1.
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

func pvSteps(r pvResult) []string {
	out := make([]string, len(r.Frames))
	for i, f := range r.Frames {
		out[i] = f.Step
	}
	return out
}

func TestPreview_Chenillard4_OneFramePerTick_HeadWalksFromG1_FundAtTheme(t *testing.T) {
	e := nativeEffect(t, "chenillard")
	r := preview(e, 4, "chrono")
	if r.ID != "chenillard" || r.Moment != "chrono" || r.Groups != 4 || r.TickMs != 1000 || r.DurationMs != previewTotalMs || r.ChronoS != 20 || !r.Loop {
		t.Fatalf("{id, moment: chrono, groups: 4, tick_ms: 1000, duration_ms: 20000, chrono_s: 20, loop: true} : %+v", r)
	}
	if len(r.Frames) != 20 {
		t.Fatalf("une trame par tick de 1 s sur 20 s : %d", len(r.Frames))
	}
	for k, f := range r.Frames {
		if f.At != k*1000 || f.Tick != k {
			t.Fatalf("trame %d : at_ms %d / tick %d, attendu %d / %d", k, f.At, f.Tick, k*1000, k)
		}
		if f.Step != "pas:0" {
			t.Fatalf("trame %d : un seul pas dans la boucle ⇒ « pas:0 », obtenu %q", k, f.Step)
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

func TestPreview_StepNames_Pre_PasN_Dernier_NeverPost_FollowTheReferenceChronogram(t *testing.T) {
	// chronogramme de référence §4.5 : PRE, pas 0, pas 1, LAST, pas 0, pas 1, LAST, pas 0…
	e := mustParse(t, "id: ref\nnom: Ref\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\nvariables:\n  gA: g1; +1\n"+
		"pre: { g0: [\"#0000FF\", 100] }\npas:\n  - { gA: [\"#FF0000\", 100] }\n  - { gA: [\"#00FF00\", 100] }\ndernier: { g0: [\"#FFFFFF\", 100] }\npost: { g0: [\"#000000\", 100] }\n")
	r := preview(e, 4, "calm")
	want := []string{"pre", "pas:0", "pas:1", "dernier", "pas:0", "pas:1", "dernier", "pas:0"}
	got := pvSteps(r)
	if len(got) < len(want) {
		t.Fatalf("pas joués : %v", got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("tick %d : step %q, attendu %q (suite : %v)", i, got[i], w, got)
		}
		if r.Frames[i].Tick != i {
			t.Errorf("trame %d : tick %d", i, r.Frames[i].Tick)
		}
	}
	for _, s := range got {
		if s == "post" {
			t.Errorf("« post » n'apparaît jamais dans l'aperçu : %v", got)
		}
	}
}

func TestPreview_SkippedStepIsAbsent_StepNumberIsTheRankInPas(t *testing.T) {
	// 3 groupes : le pas 1 vise g5 (absent) ⇒ sauté, il ne consomme pas de tick ; les pas gardent leur rang (« pas:2 »)
	e := mustParse(t, "id: saute\nnom: Saute\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g1: [\"#FF0000\", 100] }\n"+
		"  - { g5: [\"#00FF00\", 100] }\n  - { g2: [\"#0000FF\", 100] }\n")
	r := preview(e, 3, "calm")
	got := pvSteps(r)
	for i, w := range []string{"pas:0", "pas:2", "pas:0", "pas:2"} {
		if got[i] != w {
			t.Fatalf("pas joués : %v (le pas sauté n'apparaît pas, ticks consécutifs)", got)
		}
		if r.Frames[i].Tick != i {
			t.Errorf("trame %d : tick %d, consécutifs", i, r.Frames[i].Tick)
		}
	}
	for _, s := range got {
		if s == "pas:1" {
			t.Errorf("le pas sauté n'apparaît pas : %v", got)
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
				if f.At != f.Tick*e.TickMs {
					t.Fatalf("%s N=%d : trame de tick %d à %d ms (tick de %d ms)", id, n, f.Tick, f.At, e.TickMs)
				}
				v := renderView(e, previewCtx(n, moment, f.At), f.Tick)
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

func TestPreview_OneFramePerTick_EvenWhenIdenticalToThePrevious(t *testing.T) {
	// un effet constant : une trame par tick quand même (le numéro du tick s'affiche à chaque battement)
	e := mustParse(t, "id: fixe\nnom: Fixe\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g0: [\"#FF0000\", 100] }\n")
	r := preview(e, 3, "calm")
	if len(r.Frames) != 20 {
		t.Fatalf("20 ticks sur 20 s : 20 trames, même identiques : %d", len(r.Frames))
	}
	for k, f := range r.Frames {
		if f.Tick != k || f.At != k*1000 {
			t.Fatalf("trame %d : %+v", k, f)
		}
	}
	// deux pas identiques : toujours une trame par tick
	e3 := mustParse(t, "id: idem\nnom: Idem\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g0: [\"#FF0000\", 100] }\n  - { g0: [\"#FF0000\", 100] }\n")
	if r := preview(e3, 3, "calm"); len(r.Frames) != 20 {
		t.Errorf("pas identiques : 20 trames : %d", len(r.Frames))
	}
}

func TestPreview_LongTickEffect_OneFrameOnlyWithinTheDuration(t *testing.T) {
	// dégradé : tick de 60 s > 20 s : un seul tick dans l'aperçu ⇒ une trame
	d := preview(nativeEffect(t, "degrade"), 6, "calm")
	if len(d.Frames) != 1 || d.Frames[0].Tick != 0 || d.Frames[0].Step != "pas:0" || d.TickMs != 60000 || d.DurationMs != 20000 || d.ChronoS != 0 {
		t.Fatalf("dégradé : une trame (tick 0, pas:0), tick_ms 60000, duration_ms 20000, chrono_s 0 : %+v", d)
	}
	g := d.Frames[0].Groups
	if g[0].Color == g[1].Color || g[0].Color != g[4].Color || g[1].Color != g[5].Color {
		t.Errorf("une couleur par groupe, cycle de 4 : %+v", g)
	}
}

func TestPreview_NamedColourChangingBetweenTwoTicks_AddsAFrameWithTheSameTickAndStep(t *testing.T) {
	// tick de 1,5 s : la couleur `urgence` passe à l'orange à 10 s restantes (at = 10 000 ms), entre le tick 6 (9 000) et le tick 7 (10 500)
	e := mustParse(t, "id: urg\nnom: Urg\nversion: 2\nmoments: [chrono]\nduree_tick_ms: 1500\npas:\n  - { g0: [urgence, 100] }\n")
	r := preview(e, 3, "chrono")
	var before, at *pvFrame
	for i := range r.Frames {
		switch r.Frames[i].At {
		case 9000:
			before = &r.Frames[i]
		case 10000:
			at = &r.Frames[i]
		}
	}
	if before == nil || at == nil {
		t.Fatalf("trames à 9 000 et 10 000 ms attendues : %+v", r.Frames)
	}
	if at.Tick != before.Tick || at.Step != before.Step || at.Tick != 6 {
		t.Errorf("trame supplémentaire : même tick (6) et même step : %+v / %+v", before, at)
	}
	if before.Groups[0].Color != cW || at.Groups[0].Color != cOr {
		t.Errorf("blanc avant, orange à 10 s restantes : %v → %v", before.Groups[0].Color, at.Groups[0].Color)
	}
}

func TestPreview_AtMost400Frames_ShortestTickGivesTheMostFrames(t *testing.T) {
	e := mustParse(t, "id: vite\nnom: Vite\nversion: 2\nmoments: [calm]\nduree_tick_ms: 250\npas:\n  - { g0: [\"#FF0000\", 100] }\n  - { g0: [\"#00FF00\", 100] }\n")
	r := preview(e, 4, "calm")
	if len(r.Frames) != 80 || len(r.Frames) > previewMaxFrames {
		t.Errorf("20 s à 250 ms : 80 trames (une par tick), jamais plus de 400 : %d", len(r.Frames))
	}
}

func TestPreview_DurationIsFixedByTheMoment_20s_20s_5s_AndChronoSecondsOnlyForChrono(t *testing.T) {
	for _, c := range []struct {
		effect, moment      string
		duration, chrono, n int
	}{
		{"pulsation", "chrono", 20000, 20, 20 * 2}, // tick 500 ms
		{"degrade", "calm", 20000, 0, 1},
		{"couleur-equipe", "score", 5000, 0, 5},
	} {
		r := preview(nativeEffect(t, c.effect), 4, c.moment)
		if r.Moment != c.moment || r.DurationMs != c.duration || r.ChronoS != c.chrono {
			t.Errorf("%s/%s : duration_ms %d (attendu %d), chrono_s %d (attendu %d), moment %q", c.effect, c.moment, r.DurationMs, c.duration, r.ChronoS, c.chrono, r.Moment)
		}
		if len(r.Frames) < c.n {
			t.Errorf("%s/%s : au moins %d trames (une par tick) : %d", c.effect, c.moment, c.n, len(r.Frames))
		}
		for _, f := range r.Frames {
			if f.At >= c.duration {
				t.Errorf("%s/%s : aucune trame au-delà de la durée : %+v", c.effect, c.moment, f)
			}
		}
	}
}

func TestPreview_ExplicitMomentIsEchoedAndSetsTheDuration(t *testing.T) {
	// (le moment par défaut — le premier déclaré — est résolu par le serveur : testé dans http_effects_preview_299_test.go)
	e := mustParse(t, "id: deux\nnom: Deux\nversion: 2\nmoments: [score, calm]\nduree_tick_ms: 1000\npas:\n  - { g0: [\"#FF0000\", 100] }\n")
	if r := Preview(e, 3, Moment("score")); r.Moment != "score" || r.DurationMs != 5000 {
		t.Errorf("score : 5 s : %q %d", r.Moment, r.DurationMs)
	}
	if r := Preview(e, 3, Moment("calm")); r.Moment != "calm" || r.DurationMs != 20000 {
		t.Errorf("calm : 20 s : %q %d", r.Moment, r.DurationMs)
	}
}

func TestPreview_ChronoMomentSimulatesTwentySeconds(t *testing.T) {
	// pulsation (chrono) : la phase basse change de couleur avec le temps restant (blanc, orange, rouge)
	r := preview(nativeEffect(t, "pulsation"), 4, "chrono")
	seen := map[[3]int]bool{}
	for _, f := range r.Frames {
		seen[f.Groups[0].Color] = true
	}
	if !seen[cW] || !seen[cOr] || !seen[cRed] {
		t.Errorf("le chronomètre simulé de 20 s fait passer la phase basse par blanc, orange et rouge : %v", seen)
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
	if len(r.Frames) != 3 || r.Frames[2].At != 2000 || r.Frames[2].Step != "pas:2" {
		t.Errorf("trois pas puis l'état final tenu (pas de nouvelle trame) : %+v", r.Frames)
	}
}

func TestPreview_Deterministic_SameInputSameOutput(t *testing.T) {
	e := nativeEffect(t, "chenillard")
	a, b := preview(e, 6, "chrono"), preview(e, 6, "chrono")
	if fmt.Sprint(a) != fmt.Sprint(b) {
		t.Error("l'aperçu est une fonction pure de (effet, groupes, moment)")
	}
}
