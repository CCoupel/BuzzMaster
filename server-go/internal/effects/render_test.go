package effects

// Tables de trames de référence par motif — contracts/light-effects.md §4.2 à §4.6.
// Écrit avant l'implémentation (TDD). Aucune attente réelle : le temps est un paramètre.

import (
	"reflect"
	"testing"
	"time"
)

var (
	white  = [3]int{255, 255, 255}
	orange = [3]int{255, 170, 0}
	red    = [3]int{230, 30, 30}
	theme  = [3]int{79, 70, 229}
	team   = [3]int{229, 57, 53}
	black  = [3]int{0, 0, 0}
)

func near(a, b, tol int) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func nearLook(a, b Lk) bool {
	return a.Color == b.Color && near(a.Intensity, b.Intensity, 1) && a.FadeMs == b.FadeMs
}

const chenillardSrc = `
id: chenillard
name: Chenillard
version: 1
moments: [chrono]
target: groups
min_groups: 3
looks:
  fond:    { color: theme,   intensity: 100 }
  trainee: { color: theme,   intensity: 50 }
  tete:    { color: urgency, intensity: 100 }
pattern:
  type: shift
  step_ms: 1000
  cells: [trainee, tete, trainee]
  rest: fond
  wrap: true
stages:
  - when: { groups_lt: 5 }
    pattern: { cells: [tete] }
`

func chrono(n int, remaining time.Duration) Ctx {
	return Ctx{N: n, Remaining: remaining, Total: 20 * time.Second, Scene: white, Theme: theme, Team: team, Origin: 1, Seed: 1}
}

// ---------- longueur de la trame : N positions, bornes N = 1 et 12 ----------

func TestRender_FrameHasOnePositionPerGroup(t *testing.T) {
	e := mustParse(t, `
id: u
name: U
version: 1
moments: [calm]
looks: { a: { color: scene } }
pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }
`)
	for _, n := range []int{1, 3, 6, 12} {
		if got := len(render(e, Ctx{N: n, Scene: white}, 0)); got != n {
			t.Errorf("N=%d : %d positions, attendu %d", n, got, n)
		}
	}
}

func TestRender_TargetAll_SingleState(t *testing.T) {
	e := mustParse(t, `
id: all
name: All
version: 1
moments: [chrono]
target: all
looks: { a: { color: scene } }
pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }
`)
	// N = 0 : voir TestNatives_Pulsation_Declaration_AppliesWithoutGroups (applicabilité à trancher).
	for _, n := range []int{1, 6, 12} {
		if got := len(render(e, Ctx{N: n, Scene: white}, 0)); got != 1 {
			t.Errorf("target: all, N=%d : un seul état attendu, obtenu %d", n, got)
		}
	}
}

// ---------- uniform : alternance calée sur l'horloge murale ----------

const uniformSrc = `
id: resp
name: Resp
version: 1
moments: [chrono]
target: all
looks:
  haut: { color: "#FF0000", intensity: 100 }
  bas:  { color: "#0000FF", intensity: 50 }
pattern:
  type: uniform
  phases: [ { look: haut, ms: 250 }, { look: bas, ms: 750 } ]
`

func TestRender_Uniform_PhasesFollowWallClockModCycle(t *testing.T) {
	e := mustParse(t, uniformSrc)
	c := Ctx{N: 3, Scene: white}
	cases := []struct {
		at   time.Duration
		want Lk
	}{
		{0, Lk{Color: [3]int{255, 0, 0}, Intensity: 255}},
		{249 * time.Millisecond, Lk{Color: [3]int{255, 0, 0}, Intensity: 255}},
		{250 * time.Millisecond, Lk{Color: [3]int{0, 0, 255}, Intensity: 128}},
		{999 * time.Millisecond, Lk{Color: [3]int{0, 0, 255}, Intensity: 128}},
		{1000 * time.Millisecond, Lk{Color: [3]int{255, 0, 0}, Intensity: 255}}, // cycle suivant
		{12250 * time.Millisecond, Lk{Color: [3]int{0, 0, 255}, Intensity: 128}},
	}
	for _, tc := range cases {
		got := render(e, c, tc.at)
		if len(got) != 1 || !nearLook(got[0], tc.want) {
			t.Errorf("t=%s : %+v, attendu %+v", tc.at, got, tc.want)
		}
	}
}

func TestRender_IsDeterministic_SameInputsSameFrame(t *testing.T) {
	e := mustParse(t, chenillardSrc)
	c := chrono(6, 12*time.Second)
	a, b := render(e, c, 3500*time.Millisecond), render(e, c, 3500*time.Millisecond)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("Render doit être déterministe")
	}
}

// ---------- shift : chenillard ----------

func headIndex(t *testing.T, fr []Lk, headColor [3]int) int {
	t.Helper()
	idx := -1
	for i, l := range fr {
		if l.Color == headColor && l.Intensity >= 254 {
			if idx != -1 {
				t.Fatalf("une seule tête attendue : %+v", fr)
			}
			idx = i
		}
	}
	return idx
}

func TestRender_Shift_Chenillard_HeadTrailAndBackground(t *testing.T) {
	e := mustParse(t, chenillardSrc)
	fr := render(e, chrono(6, 12*time.Second), 0) // > 10 s : urgence blanche
	h := headIndex(t, fr, white)
	if h < 0 {
		t.Fatalf("tête (urgence blanche à plus de 10 s) introuvable : %+v", fr)
	}
	prev, next := (h+5)%6, (h+1)%6
	for i, l := range fr {
		switch i {
		case h:
		case prev, next:
			if l.Color != theme || !near(l.Intensity, 128, 1) {
				t.Errorf("traînée (position %d) = %+v, attendu thème à 50 %%", i, l)
			}
		default:
			if l.Color != theme || l.Intensity != 255 {
				t.Errorf("fond (position %d) = %+v, attendu thème à 100 %%", i, l)
			}
		}
	}
}

func TestRender_Shift_AdvancesOneGroupPerStepAndWraps(t *testing.T) {
	e := mustParse(t, chenillardSrc)
	c := chrono(6, 15*time.Second)
	h0 := headIndex(t, render(e, c, 0), white)
	for step := 1; step <= 7; step++ {
		got := headIndex(t, render(e, c, time.Duration(step)*time.Second), white)
		if want := (h0 + step) % 6; got != want {
			t.Fatalf("pas %d : tête en %d, attendu %d (un groupe par seconde, retour au premier)", step, got, want)
		}
	}
	// dans un même pas, la trame ne bouge pas
	if !reflect.DeepEqual(render(e, c, 2000*time.Millisecond), render(e, c, 2999*time.Millisecond)) {
		t.Error("la trame est constante à l'intérieur d'un pas")
	}
}

func TestRender_Shift_Chenillard_NoTrailBelowFiveGroups(t *testing.T) {
	e := mustParse(t, chenillardSrc)
	fr := render(e, chrono(4, 15*time.Second), 0)
	heads, bg := 0, 0
	for _, l := range fr {
		if l.Color == white && l.Intensity >= 254 {
			heads++
		} else if l.Color == theme && l.Intensity == 255 {
			bg++
		}
	}
	if heads != 1 || bg != 3 {
		t.Fatalf("sous 5 groupes : 1 tête et 3 fonds, sans traînée (étape groups_lt 5) : %+v", fr)
	}
	fr5 := render(e, chrono(5, 15*time.Second), 0)
	trail := 0
	for _, l := range fr5 {
		if l.Color == theme && near(l.Intensity, 128, 1) {
			trail++
		}
	}
	if trail != 2 {
		t.Fatalf("à 5 groupes la traînée est conservée (2 cases) : %+v", fr5)
	}
}

func TestRender_Urgency_Thresholds(t *testing.T) {
	e := mustParse(t, chenillardSrc)
	cases := []struct {
		rem  time.Duration
		want [3]int
	}{
		{15 * time.Second, white},
		{10*time.Second + time.Millisecond, white},
		{10 * time.Second, orange},
		{5*time.Second + time.Millisecond, orange},
		{5 * time.Second, red},
		{1 * time.Second, red},
	}
	for _, tc := range cases {
		fr := render(e, chrono(6, tc.rem), 0)
		if headIndex(t, fr, tc.want) < 0 {
			t.Errorf("reste %s : tête %v attendue, trame %+v", tc.rem, tc.want, fr)
		}
	}
}

func TestRender_Shift_DirectionBackward(t *testing.T) {
	e := mustParse(t, `
id: back
name: Back
version: 1
moments: [chrono]
looks: { on: { color: "#FFFFFF" }, off: { color: "#000000", intensity: 0 } }
pattern: { type: shift, step_ms: 1000, cells: [on], rest: off, wrap: true, direction: backward }
`)
	c := Ctx{N: 4, Scene: white, Remaining: 15 * time.Second, Total: 20 * time.Second}
	idx := func(at time.Duration) int {
		for i, l := range render(e, c, at) {
			if l.Intensity == 255 {
				return i
			}
		}
		return -1
	}
	a, b := idx(0), idx(time.Second)
	if (a+4-b)%4 != 1 {
		t.Fatalf("direction backward : la tête doit reculer d'un groupe par pas (%d puis %d)", a, b)
	}
}

// ---------- gauge : remplissage ----------

const gaugeSrc = `
id: jauge
name: Jauge
version: 1
moments: [chrono]
looks:
  on:  { color: scene, intensity: 100 }
  off: { color: scene, intensity: 0 }
pattern: { type: gauge, source: remaining, on: on, off: off }
`

func countOn(fr []Lk) int {
	n := 0
	for _, l := range fr {
		if l.Intensity > 0 {
			n++
		}
	}
	return n
}

func TestRender_Gauge_RemainingIsProportional(t *testing.T) {
	e := mustParse(t, gaugeSrc)
	cases := []struct {
		rem  time.Duration
		want int
	}{{20 * time.Second, 6}, {10 * time.Second, 3}, {0, 0}}
	for _, tc := range cases {
		c := chrono(6, tc.rem)
		if got := countOn(render(e, c, 0)); got != tc.want {
			t.Errorf("reste %s sur 20 s, 6 groupes : %d allumés, attendu %d", tc.rem, got, tc.want)
		}
	}
}

func TestRender_Gauge_LitGroupsAreContiguousFromTheFirst(t *testing.T) {
	e := mustParse(t, gaugeSrc)
	fr := render(e, chrono(6, 10*time.Second), 0)
	for i := 0; i < len(fr); i++ {
		if (i < 3) != (fr[i].Intensity > 0) {
			t.Fatalf("les groupes allumés forment un bloc depuis le groupe 1 : %+v", fr)
		}
	}
}

func TestRender_Gauge_ElapsedIsComplement(t *testing.T) {
	e := mustParse(t, `
id: jauge2
name: Jauge2
version: 1
moments: [chrono]
looks: { on: { color: scene }, off: { color: scene, intensity: 0 } }
pattern: { type: gauge, source: elapsed, on: on, off: off }
`)
	if got := countOn(render(e, chrono(6, 15*time.Second), 0)); got != 1 && got != 2 {
		t.Errorf("source elapsed : 5 s écoulées sur 20 s ⇒ ~1,5/6 groupes, obtenu %d", got)
	}
	if got := countOn(render(e, chrono(6, 0), 0)); got != 6 {
		t.Errorf("source elapsed, tout écoulé ⇒ 6 allumés, obtenu %d", got)
	}
}

func TestRender_Gauge_EdgeLookOnBoundaryGroup(t *testing.T) {
	e := mustParse(t, `
id: jauge3
name: Jauge3
version: 1
moments: [chrono]
looks:
  on: { color: scene }
  off: { color: scene, intensity: 0 }
  edge: { color: "#FF0000" }
pattern: { type: gauge, source: remaining, on: on, off: off, edge: edge }
`)
	red := 0
	for _, l := range render(e, chrono(6, 12*time.Second), 0) { // 3,6 groupes allumés ⇒ frontière au 4e
		if l.Color == [3]int{255, 0, 0} {
			red++
		}
	}
	if red != 1 {
		t.Errorf("un seul groupe frontière attendu, obtenu %d", red)
	}
}

// ---------- random : graine fixe ----------

const randomSrc = `
id: hasard
name: Hasard
version: 1
moments: [calm]
looks: { on: { color: scene, intensity: 100 }, off: { color: scene, intensity: 0 } }
pattern: { type: random, step_ms: 1000, count: 3, on: on, off: off }
`

func TestRender_Random_FixedSeedIsDeterministicAndCountRespected(t *testing.T) {
	e := mustParse(t, randomSrc)
	c := Ctx{N: 8, Scene: theme, Seed: 42}
	for step := 0; step < 10; step++ {
		at := time.Duration(step) * time.Second
		a, b := render(e, c, at), render(e, c, at)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("pas %d : même graine, même trame exigée", step)
		}
		if got := countOn(a); got != 3 {
			t.Fatalf("pas %d : %d groupes allumés, attendu count=3", step, got)
		}
	}
}

func TestRender_Random_DifferentSeedsOrStepsGiveDifferentFrames(t *testing.T) {
	e := mustParse(t, randomSrc)
	c := Ctx{N: 12, Scene: theme, Seed: 1}
	seen := map[string]bool{}
	for step := 0; step < 8; step++ {
		fr := render(e, c, time.Duration(step)*time.Second)
		key := ""
		for _, l := range fr {
			if l.Intensity > 0 {
				key += "1"
			} else {
				key += "0"
			}
		}
		seen[key] = true
	}
	if len(seen) < 3 {
		t.Errorf("un tirage doit varier d'un pas à l'autre (%d trames distinctes sur 8)", len(seen))
	}
	other := render(e, Ctx{N: 12, Scene: theme, Seed: 999}, 0)
	same := render(e, c, 0)
	if reflect.DeepEqual(other, same) {
		t.Log("graines différentes, même tirage au pas 0 : possible mais improbable")
	}
}

func TestRender_Random_StableWithinAStep(t *testing.T) {
	e := mustParse(t, randomSrc)
	c := Ctx{N: 8, Scene: theme, Seed: 7}
	if !reflect.DeepEqual(render(e, c, 2000*time.Millisecond), render(e, c, 2900*time.Millisecond)) {
		t.Error("le tirage ne change qu'à chaque step_ms")
	}
}

// ---------- gradient ----------

func TestRender_Gradient_FixedLinearRGB(t *testing.T) {
	e := mustParse(t, `
id: deg
name: Deg
version: 1
moments: [calm]
looks:
  a: { color: "#FF0000" }
  b: { color: "#0000FF" }
pattern: { type: gradient, stops: [a, b], travel: none, repeat: false }
`)
	fr := render(e, Ctx{N: 3, Scene: white}, 0)
	if fr[0].Color != [3]int{255, 0, 0} || fr[2].Color != [3]int{0, 0, 255} {
		t.Fatalf("extrémités = premier et dernier arrêt : %+v", fr)
	}
	mid := fr[1].Color
	if !near(mid[0], 128, 1) || mid[1] != 0 || !near(mid[2], 128, 1) {
		t.Errorf("milieu = interpolation RGB linéaire (128,0,128), obtenu %v", mid)
	}
	// travel none : fixe dans le temps
	if !reflect.DeepEqual(fr, render(e, Ctx{N: 3, Scene: white}, 30*time.Second)) {
		t.Error("travel: none ⇒ dégradé fixe")
	}
}

func TestRender_Gradient_SingleGroupTakesFirstStop(t *testing.T) {
	e := mustParse(t, `
id: deg1
name: Deg1
version: 1
moments: [calm]
looks: { a: { color: "#FF0000" }, b: { color: "#0000FF" } }
pattern: { type: gradient, stops: [a, b], travel: none, repeat: false }
`)
	fr := render(e, Ctx{N: 1, Scene: white}, 0)
	if len(fr) != 1 || fr[0].Color != [3]int{255, 0, 0} {
		t.Fatalf("N=1 : premier arrêt, obtenu %+v", fr)
	}
}

func TestRender_Gradient_OutwardStartsAtTeamOrigin(t *testing.T) {
	e := mustParse(t, `
id: vague
name: Vague
version: 1
moments: [score]
duration_ms: 3000
looks: { eq: { color: team }, fond: { color: "#000000", intensity: 0 } }
pattern: { type: gradient, stops: [eq, fond], step_ms: 500, travel: outward, origin: team, repeat: false }
`)
	c := Ctx{N: 6, Scene: white, Team: team, Origin: 3, Seed: 1}
	fr := render(e, c, 0)
	if fr[2].Color != team {
		t.Errorf("au pas 0 la couleur de l'équipe est sur le groupe d'origine (3) : %+v", fr)
	}
	// la couleur s'atténue en s'éloignant de l'origine (propagation vers les deux bords)
	for i := 0; i < 6; i++ {
		d := i - 2
		if d < 0 {
			d = -d
		}
		if i != 2 && fr[i].Intensity >= fr[2].Intensity {
			t.Errorf("le groupe %d (distance %d) doit être plus atténué que l'origine : %+v", i+1, d, fr)
		}
	}
	if fr[0].Intensity < fr[5].Intensity-255 { // garde-fou de forme, jamais négatif
		t.Fatal("intensités incohérentes")
	}
}

// ---------- étapes (stages) ----------

func TestRender_Stages_FirstMatchingStageWins_RemainingConditions(t *testing.T) {
	e := mustParse(t, `
id: etapes
name: Etapes
version: 1
moments: [chrono]
target: all
looks:
  calme: { color: "#00FF00" }
  alerte: { color: "#FFAA00" }
  rouge: { color: "#FF0000" }
pattern: { type: uniform, phases: [ { look: calme, ms: 1000 } ] }
stages:
  - when: { remaining_lte: 5 }
    pattern: { phases: [ { look: rouge, ms: 1000 } ] }
  - when: { remaining_lte: 10 }
    pattern: { phases: [ { look: alerte, ms: 1000 } ] }
`)
	cases := []struct {
		rem  time.Duration
		want [3]int
	}{{20 * time.Second, [3]int{0, 255, 0}}, {10 * time.Second, [3]int{255, 170, 0}}, {5 * time.Second, [3]int{255, 0, 0}}, {3 * time.Second, [3]int{255, 0, 0}}}
	for _, tc := range cases {
		fr := render(e, chrono(3, tc.rem), 0)
		if fr[0].Color != tc.want {
			t.Errorf("reste %s : %v, attendu %v (première étape vraie)", tc.rem, fr[0].Color, tc.want)
		}
	}
}

func TestRender_Stages_GroupsConditions(t *testing.T) {
	e := mustParse(t, `
id: g
name: G
version: 1
moments: [calm]
looks: { a: { color: "#FF0000" }, b: { color: "#00FF00" } }
pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }
stages:
  - when: { groups_gte: 6 }
    pattern: { phases: [ { look: b, ms: 1000 } ] }
`)
	if got := render(e, Ctx{N: 5, Scene: white}, 0)[0].Color; got != [3]int{255, 0, 0} {
		t.Errorf("N=5 : base, obtenu %v", got)
	}
	if got := render(e, Ctx{N: 6, Scene: white}, 0)[0].Color; got != [3]int{0, 255, 0} {
		t.Errorf("N=6 : étape groups_gte, obtenu %v", got)
	}
}

func TestRender_Stages_RemainingConditionIsFalseOutsideChrono(t *testing.T) {
	e := mustParse(t, `
id: r
name: R
version: 1
moments: [calm]
looks: { a: { color: "#FF0000" }, b: { color: "#00FF00" } }
pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }
stages:
  - when: { remaining_lte: 100 }
    pattern: { phases: [ { look: b, ms: 1000 } ] }
`)
	// moment calm : pas de temps restant ⇒ condition fausse (§4.4)
	if got := render(e, Ctx{N: 3, Scene: white}, 0)[0].Color; got != [3]int{255, 0, 0} {
		t.Errorf("remaining_* est faux hors chrono, obtenu %v", got)
	}
}

// ---------- apparence : fondu et pourcentage ----------

func TestRender_Look_FadeAndIntensityPercent(t *testing.T) {
	e := mustParse(t, `
id: f
name: F
version: 1
moments: [calm]
looks: { a: { color: "#102030", intensity: 40, fade_ms: 1500 } }
pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }
`)
	l := render(e, Ctx{N: 2, Scene: white}, 0)[0]
	if l.Color != [3]int{16, 32, 48} || !near(l.Intensity, 102, 1) || l.FadeMs != 1500 {
		t.Errorf("couleur, intensité 40 %% → ~102/255, fondu 1500 ms : %+v", l)
	}
}

func TestRender_Colors_SceneThemeTeamResolved(t *testing.T) {
	for name, want := range map[string][3]int{"scene": white, "theme": theme, "team": team} {
		e := mustParse(t, "id: c\nname: C\nversion: 1\nmoments: [score]\nduration_ms: 1000\nlooks: { a: { color: "+name+" } }\npattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }\n")
		if got := render(e, Ctx{N: 1, Scene: white, Theme: theme, Team: team}, 0)[0].Color; got != want {
			t.Errorf("couleur %s : %v, attendu %v", name, got, want)
		}
	}
}
