package effects

// Motif `gradient` — sémantique exacte, contracts/light-effects.md §4.3.1 (amendement du 2026-11-11).
// Tables conformes au contrat : fixe (none), anneau (forward/backward + repeat true), propagation (outward ou
// forward/backward + repeat false), validation de rest et origin. Aucune attente : le temps est un paramètre.

import (
	"strings"
	"testing"
	"time"
)

func gradSrc(pattern string, extraLooks string) string {
	return "id: deg\nname: Deg\nversion: 1\nmoments: [calm, score]\nduration_ms: 20000\nlooks:\n" +
		"  r: { color: \"#FF0000\" }\n  g: { color: \"#00FF00\" }\n  b: { color: \"#0000FF\" }\n  fond: { color: \"#101010\" }\n" + extraLooks +
		"pattern: " + pattern + "\n"
}

func at(step int) time.Duration { return time.Duration(step) * time.Second } // step_ms: 1000

func nearColor(a, b [3]int) bool {
	return near(a[0], b[0], 1) && near(a[1], b[1], 1) && near(a[2], b[2], 1)
}

func expectColors(t *testing.T, label string, got []Lk, want [][3]int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s : %d positions, attendu %d (%+v)", label, len(got), len(want), got)
	}
	for i := range want {
		if got[i].Absent || !nearColor(got[i].Color, want[i]) {
			t.Errorf("%s : position %d = %v (absent=%v), attendu %v", label, i, got[i].Color, got[i].Absent, want[i])
		}
	}
}

var (
	cR   = [3]int{255, 0, 0}
	cG   = [3]int{0, 255, 0}
	cB   = [3]int{0, 0, 255}
	cFnd = [3]int{16, 16, 16}
	cW   = [3]int{255, 255, 255}
)

// ---------- (a) travel: none — du premier au dernier arrêt, jamais de retour ----------

func TestGradient_None_FirstToLastStop_LinearRGB(t *testing.T) {
	e := mustParse(t, gradSrc("{ type: gradient, stops: [r, g, b], travel: none }", ""))
	expectColors(t, "N=5", render(e, Ctx{N: 5, Scene: white}, 0), [][3]int{cR, {128, 128, 0}, cG, {0, 128, 128}, cB})
	expectColors(t, "N=3", render(e, Ctx{N: 3, Scene: white}, 0), [][3]int{cR, cG, cB})
	expectColors(t, "N=2", render(e, Ctx{N: 2, Scene: white}, 0), [][3]int{cR, cB})
}

func TestGradient_None_SingleGroupIsFirstStop(t *testing.T) {
	e := mustParse(t, gradSrc("{ type: gradient, stops: [r, b], travel: none }", ""))
	expectColors(t, "N=1", render(e, Ctx{N: 1, Scene: white}, 0), [][3]int{cR})
}

func TestGradient_None_RepeatHasNoEffect_AndIsFixedInTime(t *testing.T) {
	want := [][3]int{cR, {128, 0, 128}, cB}
	for _, rep := range []string{"true", "false"} {
		e := mustParse(t, gradSrc("{ type: gradient, stops: [r, b], travel: none, repeat: "+rep+" }", ""))
		for _, s := range []int{0, 1, 7, 100} {
			expectColors(t, "repeat "+rep, render(e, Ctx{N: 3, Scene: white}, at(s)), want)
		}
	}
	e := mustParse(t, gradSrc("{ type: gradient, stops: [r, b], travel: none }", "")) // repeat par défaut (true) : pas de retour vers S0
	if got := render(e, Ctx{N: 3, Scene: white}, 0); !nearColor(got[2].Color, cB) {
		t.Errorf("repeat par défaut + travel none : le dernier groupe montre le dernier arrêt, pas S0 : %v", got[2].Color)
	}
}

func TestGradient_None_InterpolatesIntensityToo(t *testing.T) {
	e := mustParse(t, "id: d\nname: D\nversion: 1\nmoments: [calm]\nlooks:\n  a: { color: \"#FFFFFF\", intensity: 100 }\n  b: { color: \"#FFFFFF\", intensity: 0 }\n"+
		"pattern: { type: gradient, stops: [a, b], travel: none }\n")
	fr := render(e, Ctx{N: 3, Scene: white}, 0)
	if fr[0].Intensity != 255 || !near(fr[1].Intensity, 128, 1) || fr[2].Intensity != 0 {
		t.Errorf("intensité interpolée 255 → ~128 → 0 : %+v", fr)
	}
}

func TestGradient_None_FadeIsNearestStops(t *testing.T) {
	e := mustParse(t, "id: d\nname: D\nversion: 1\nmoments: [calm]\nlooks:\n  a: { color: \"#FF0000\", fade_ms: 100 }\n  b: { color: \"#0000FF\", fade_ms: 900 }\n"+
		"pattern: { type: gradient, stops: [a, b], travel: none }\n")
	fr := render(e, Ctx{N: 3, Scene: white}, 0)
	if fr[0].FadeMs != 100 || fr[2].FadeMs != 900 {
		t.Errorf("fade_ms de l'arrêt le plus proche : %+v", fr)
	}
}

// ---------- (b) propagation : outward, ou forward/backward + repeat: false ----------

const waveStops = "stops: [w, b], step_ms: 1000"

func waveSrc(travel, origin, repeat, rest string) string {
	p := "{ type: gradient, " + waveStops + ", travel: " + travel
	if origin != "" {
		p += ", origin: " + origin
	}
	if repeat != "" {
		p += ", repeat: " + repeat
	}
	if rest != "" {
		p += ", rest: " + rest
	}
	return gradSrc(p+" }", "  w: { color: \"#FFFFFF\" }\n")
}

func TestGradient_Outward_ContractExample_5Groups_StartAtGroup2_RepeatFalse(t *testing.T) {
	e := mustParse(t, waveSrc("outward", "team", "false", "fond"))
	c := Ctx{N: 5, Scene: white, Team: cB, Origin: 2}
	F, W, B := cFnd, cW, cB
	table := [][][3]int{
		{F, W, F, F, F},
		{W, B, W, F, F},
		{B, B, B, W, F},
		{B, B, B, B, W},
		{B, B, B, B, B}, // tenu
		{B, B, B, B, B},
		{B, B, B, B, B},
	}
	for s, want := range table {
		expectColors(t, "pas "+string(rune('0'+s)), render(e, c, at(s)), want)
	}
}

func TestGradient_Outward_WithoutRest_UnreachedGroupsEmitNoState(t *testing.T) {
	e := mustParse(t, waveSrc("outward", "team", "false", ""))
	c := Ctx{N: 5, Scene: white, Team: cB, Origin: 2}
	fr := render(e, c, at(0))
	for i, l := range fr {
		if (i == 1) == l.Absent {
			t.Errorf("pas 0 : seul le groupe d'origine (position 1) a un état ; position %d absent=%v", i, l.Absent)
		}
	}
	if !nearColor(fr[1].Color, cW) {
		t.Errorf("l'origine montre S0 : %v", fr[1].Color)
	}
	fr = render(e, c, at(1))
	for i, l := range fr {
		if (i <= 2) == l.Absent {
			t.Errorf("pas 1 : positions 0..2 atteintes, les autres absentes ; position %d absent=%v", i, l.Absent)
		}
	}
}

func TestGradient_Outward_RepeatTrue_LoopsStopsAsSuccessiveWaves(t *testing.T) {
	e := mustParse(t, waveSrc("outward", "team", "true", "fond"))
	c := Ctx{N: 5, Scene: white, Team: cB, Origin: 2}
	// pas 2 : origine a=2 → 2 mod 2 = S0 ; voisins d=1 a=1 → S1 ; d=2 a=0 → S0 ; d=3 > 2 → rest.
	expectColors(t, "pas 2", render(e, c, at(2)), [][3]int{cB, cW, cB, cW, cFnd})
	// pas 3 : origine a=3 → S1 ; d=1 a=2 → S0 ; d=2 a=1 → S1 ; d=3 a=0 → S0.
	expectColors(t, "pas 3", render(e, c, at(3)), [][3]int{cW, cB, cW, cB, cW})
}

func TestGradient_Outward_OriginVariants(t *testing.T) {
	c := Ctx{N: 6, Scene: white, Team: cB, Origin: 4}
	// au pas 0 seul le groupe de départ montre S0 (crête), les autres sont au repos.
	head := func(origin string) int {
		e := mustParse(t, waveSrc("outward", origin, "false", "fond"))
		idx := -1
		for i, l := range render(e, c, 0) {
			if nearColor(l.Color, cW) {
				if idx != -1 {
					t.Fatalf("origin %s : une seule crête au pas 0", origin)
				}
				idx = i
			}
		}
		return idx
	}
	for origin, want := range map[string]int{"first": 0, "last": 5, "center": 2, "team": 3} {
		if got := head(origin); got != want {
			t.Errorf("origin %s : départ en position %d, attendu %d (center = ⌊(N−1)/2⌋, team = groupe 4 → position 3)", origin, got, want)
		}
	}
}

func TestGradient_Outward_PropagatesToBothEdgesSymmetrically(t *testing.T) {
	e := mustParse(t, waveSrc("outward", "center", "false", "fond"))
	fr := render(e, Ctx{N: 7, Scene: white}, at(2)) // centre = 3 ; d ≤ 2 atteint
	for _, i := range []int{1, 2, 3, 4, 5} {
		if nearColor(fr[i].Color, cFnd) {
			t.Errorf("position %d (d ≤ 2) doit être atteinte au pas 2 : %v", i, fr[i].Color)
		}
	}
	for _, i := range []int{0, 6} {
		if !nearColor(fr[i].Color, cFnd) {
			t.Errorf("position %d (d = 3 > 2) est encore au repos : %v", i, fr[i].Color)
		}
	}
}

func TestGradient_Forward_RepeatFalse_StartsFirstGroup_Propagates(t *testing.T) {
	e := mustParse(t, waveSrc("forward", "", "false", "fond"))
	c := Ctx{N: 4, Scene: white}
	expectColors(t, "pas 0", render(e, c, at(0)), [][3]int{cW, cFnd, cFnd, cFnd})
	expectColors(t, "pas 1", render(e, c, at(1)), [][3]int{cB, cW, cFnd, cFnd})
	expectColors(t, "pas 3", render(e, c, at(3)), [][3]int{cB, cB, cB, cW})
	expectColors(t, "pas 9", render(e, c, at(9)), [][3]int{cB, cB, cB, cB})
}

func TestGradient_Backward_RepeatFalse_StartsLastGroup_Propagates(t *testing.T) {
	e := mustParse(t, waveSrc("backward", "", "false", "fond"))
	c := Ctx{N: 4, Scene: white}
	expectColors(t, "pas 0", render(e, c, at(0)), [][3]int{cFnd, cFnd, cFnd, cW})
	expectColors(t, "pas 1", render(e, c, at(1)), [][3]int{cFnd, cFnd, cW, cB})
	expectColors(t, "pas 3", render(e, c, at(3)), [][3]int{cW, cB, cB, cB})
}

func TestGradient_Propagation_StepsWithoutInterpolation_ConstantInsideAStep(t *testing.T) {
	e := mustParse(t, waveSrc("outward", "first", "false", "fond"))
	c := Ctx{N: 4, Scene: white}
	a, b := render(e, c, 2000*time.Millisecond), render(e, c, 2999*time.Millisecond)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("pas d'interpolation : trame constante pendant un pas (%+v / %+v)", a, b)
		}
	}
	// chaque groupe atteint montre exactement un arrêt (S0 ou S1), jamais une teinte intermédiaire
	for _, l := range a {
		if !nearColor(l.Color, cW) && !nearColor(l.Color, cB) && !nearColor(l.Color, cFnd) {
			t.Errorf("teinte intermédiaire interdite en propagation : %v", l.Color)
		}
	}
}

// ---------- (c) forward / backward, repeat: true — anneau fermé qui tourne ----------

func ringSrc(travel string) string {
	return gradSrc("{ type: gradient, stops: [r, b], step_ms: 1000, travel: "+travel+", repeat: true }", "")
}

func TestGradient_Ring_Forward_ClosedGradientTable(t *testing.T) {
	e := mustParse(t, ringSrc("forward"))
	c := Ctx{N: 4, Scene: white}
	mid := [3]int{128, 0, 128}
	// x = ((p − s) mod N) / N × k, k = 2 : positions 0..3 au pas 0 → R, mid, B, mid (retour vers S0, sans couture)
	expectColors(t, "pas 0", render(e, c, at(0)), [][3]int{cR, mid, cB, mid})
	// le motif avance d'un groupe par pas : trame(s)[p] = trame(0)[(p − s) mod N]
	expectColors(t, "pas 1", render(e, c, at(1)), [][3]int{mid, cR, mid, cB})
	expectColors(t, "pas 2", render(e, c, at(2)), [][3]int{cB, mid, cR, mid})
	expectColors(t, "pas 4 = pas 0", render(e, c, at(4)), [][3]int{cR, mid, cB, mid})
}

func TestGradient_Ring_Backward_RotatesTheOtherWay(t *testing.T) {
	e := mustParse(t, ringSrc("backward"))
	c := Ctx{N: 4, Scene: white}
	mid := [3]int{128, 0, 128}
	expectColors(t, "pas 0", render(e, c, at(0)), [][3]int{cR, mid, cB, mid})
	// x = ((p + s) mod N) / N × k : trame(s)[p] = trame(0)[(p + s) mod N]
	expectColors(t, "pas 1", render(e, c, at(1)), [][3]int{mid, cB, mid, cR})
}

func TestGradient_Ring_ThreeStops_WrapsBackToFirst(t *testing.T) {
	e := mustParse(t, gradSrc("{ type: gradient, stops: [r, g, b], step_ms: 1000, travel: forward }", "")) // repeat par défaut true
	fr := render(e, Ctx{N: 6, Scene: white}, 0)
	// k = 3, N = 6 : x = p/6 × 3 → p0 R, p2 G, p4 B, p5 entre B et R (x = 2,5)
	expectColors(t, "pas 0", fr, [][3]int{cR, {128, 128, 0}, cG, {0, 128, 128}, cB, {128, 0, 128}})
}

func TestGradient_Ring_InterpolatesIntensity(t *testing.T) {
	e := mustParse(t, "id: d\nname: D\nversion: 1\nmoments: [calm]\nlooks:\n  a: { color: \"#FFFFFF\", intensity: 100 }\n  b: { color: \"#FFFFFF\", intensity: 0 }\n"+
		"pattern: { type: gradient, stops: [a, b], step_ms: 1000, travel: forward, repeat: true }\n")
	fr := render(e, Ctx{N: 4, Scene: white}, 0)
	if fr[0].Intensity != 255 || !near(fr[1].Intensity, 128, 1) || fr[2].Intensity != 0 || !near(fr[3].Intensity, 128, 1) {
		t.Errorf("intensité interpolée sur l'anneau : %+v", fr)
	}
}

// ---------- (d) validation de rest et origin ----------

func TestGradient_Validation_RestAndOrigin(t *testing.T) {
	mk := func(pat string) string { return gradSrc(pat, "  w: { color: \"#FFFFFF\" }\n") }
	ok := map[string]string{
		"rest avec outward":               mk("{ type: gradient, stops: [w, b], step_ms: 1000, travel: outward, origin: center, rest: fond }"),
		"rest avec forward repeat false":  mk("{ type: gradient, stops: [w, b], step_ms: 1000, travel: forward, repeat: false, rest: fond }"),
		"rest avec backward repeat false": mk("{ type: gradient, stops: [w, b], step_ms: 1000, travel: backward, repeat: false, rest: fond }"),
		"origin avec outward":             mk("{ type: gradient, stops: [w, b], step_ms: 1000, travel: outward, origin: first }"),
		"outward sans origin":             mk("{ type: gradient, stops: [w, b], step_ms: 1000, travel: outward }"),
		"propagation sans rest":           mk("{ type: gradient, stops: [w, b], step_ms: 1000, travel: outward, origin: last }"),
	}
	for name, src := range ok {
		if _, errs := parseEffect(src); len(errs) != 0 {
			t.Errorf("%s : accepté, obtenu %+v", name, errs)
		}
	}
	bad := map[string]string{
		"rest avec none":                  mk("{ type: gradient, stops: [w, b], travel: none, rest: fond }"),
		"rest avec forward repeat true":   mk("{ type: gradient, stops: [w, b], step_ms: 1000, travel: forward, repeat: true, rest: fond }"),
		"rest avec forward (défaut true)": mk("{ type: gradient, stops: [w, b], step_ms: 1000, travel: forward, rest: fond }"),
		"rest inexistant":                 mk("{ type: gradient, stops: [w, b], step_ms: 1000, travel: outward, rest: inconnu }"),
		"origin avec none":                mk("{ type: gradient, stops: [w, b], travel: none, origin: first }"),
		"origin avec forward":             mk("{ type: gradient, stops: [w, b], step_ms: 1000, travel: forward, repeat: false, origin: first }"),
		"origin avec backward":            mk("{ type: gradient, stops: [w, b], step_ms: 1000, travel: backward, origin: last }"),
		"origin invalide":                 mk("{ type: gradient, stops: [w, b], step_ms: 1000, travel: outward, origin: milieu }"),
		"un seul arrêt":                   mk("{ type: gradient, stops: [w], travel: none }"),
		"sept arrêts":                     mk("{ type: gradient, stops: [w, b, r, g, w, b, r], travel: none }"),
		"arrêt non défini":                mk("{ type: gradient, stops: [w, zzz], travel: none }"),
		"travel inconnu":                  mk("{ type: gradient, stops: [w, b], travel: spirale }"),
	}
	for name, src := range bad {
		_, errs := parseEffect(src)
		if len(errs) == 0 {
			t.Errorf("%s : erreur de validation attendue", name)
			continue
		}
		if errs[0].Message == "" || strings.TrimSpace(errs[0].Message) == "" {
			t.Errorf("%s : message lisible attendu", name)
		}
	}
}
