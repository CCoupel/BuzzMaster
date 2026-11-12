package ambiance

// Lecteur d'effets pas à pas (Batch R, tâche R10) — contrat light-effects.md §4.5, §5.1, §5.3, §6 : tick compté depuis le
// début du moment (horloge injectée), PRE au démarrage, règle d'avance des variables, vitesse (tick ÷ multiplicateur,
// plancher 250 ms, changement pris au tick suivant sans faire repartir l'effet), un effet par groupe animateur à sa vitesse.
// Aucune attente réelle : env.now est avancé à la main. Réutilise tw223New et les aides de effects_223_test.go.

import (
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

// chronogramCalm : l'exemple du §4.5 (gA: g1; +1, gB: g2; +2, PRE, deux pas, LAST) au moment « calm », tick de 1 s.
const chronogramCalm = `id: chrono-calm
nom: Chronogramme
version: 2
moments: [calm]
duree_tick_ms: 1000
variables:
  gA: g1; +1
  gB: g2; +2
pre:
  { g0: ["#0000FF", 40] }
pas:
  - { gA: ["#FFFFFF", 100] }
  - { gB: ["#FF0000", 100] }
dernier:
  { g0: ["#0000FF", 40] }
`

// tw223Token : jeton d'un état (b bleu doux, W blanc, R rouge, G vert, « _ » aucun).
func tw223Token(z lighting.ZoneState) string {
	switch {
	case z.Intensity == 0:
		return "."
	case z.Color == [3]int{0, 0, 255} && z.Intensity >= 101 && z.Intensity <= 103:
		return "b"
	case z.Color == [3]int{255, 255, 255} && z.Intensity == 255:
		return "W"
	case z.Color == [3]int{255, 0, 0} && z.Intensity == 255:
		return "R"
	case z.Color == [3]int{0, 255, 0} && z.Intensity == 255:
		return "G"
	}
	return "?"
}

// tw223SceneRow : un jeton par groupe de salle réel 1…n, d'après les zones de Scene(ev) (entrée Group 0 puis entrées de groupe).
func tw223SceneRow(env *tw223Env, ev lighting.Event, n int) string {
	st := env.c.Scene(ev)
	base := tw223Token(st.Zones[0])
	out := make([]byte, n)
	for i := range out {
		out[i] = base[0]
	}
	for _, z := range st.Zones {
		if z.Zone == lighting.ZoneGeneral && z.Area == lighting.AreaAmbiance && z.Group >= 1 && z.Group <= n {
			out[z.Group-1] = tw223Token(z)[0]
		}
	}
	return string(out)
}

func tw223FourGroups() []config.LightingGroupEntry {
	return []config.LightingGroupEntry{grp(0, "A"), grp(1, "B"), grp(2, "C"), grp(3, "D")}
}

func TestReader_Chronogram_TickByTick_ThroughScene_WithInjectedClock(t *testing.T) {
	env := tw223New(t)
	env.setup(tw223FourGroups())
	env.writeUser(t, "chrono-calm.yaml", chronogramCalm)
	env.setEffect("ambiance", "calm", "chrono-calm")
	start := env.now
	want := []string{"bbbb", "Wbbb", "WbbR", "bbbb", "bbWb", "bbWR", "bbbb", "Wbbb"}
	ev := lighting.Event{Kind: lighting.KindIdle}
	for tick, w := range want {
		env.now = start.Add(time.Duration(tick) * time.Second)
		env.step()
		if got := tw223SceneRow(env, ev, 4); got != w {
			t.Errorf("tick %d : %q, attendu %q (chronogramme §4.5 vu par le lecteur)", tick, got, w)
		}
	}
}

func TestReader_TickIsConstantInsideAStep_NoDriftBetweenSteps(t *testing.T) {
	env := tw223New(t)
	env.setup(tw223FourGroups())
	env.writeUser(t, "chrono-calm.yaml", chronogramCalm)
	env.setEffect("ambiance", "calm", "chrono-calm")
	start := env.now
	ev := lighting.Event{Kind: lighting.KindIdle}
	env.step() // tick 0
	for _, ms := range []int{1, 500, 999} {
		env.now = start.Add(time.Duration(ms) * time.Millisecond)
		env.step()
		if got := tw223SceneRow(env, ev, 4); got != "bbbb" {
			t.Errorf("t = %d ms : toujours le tick 0 (PRE) : %q", ms, got)
		}
	}
	env.now = start.Add(time.Second)
	env.step()
	if got := tw223SceneRow(env, ev, 4); got != "Wbbb" {
		t.Errorf("t = 1 s : tick 1 (pas 0) : %q", got)
	}
}

func TestReader_EffectRestartsFromTick0_WhenTheMomentStartsAgain(t *testing.T) {
	env := tw223New(t)
	env.setup(tw223FourGroups())
	env.writeUser(t, "chrono-calm.yaml", chronogramCalm)
	env.setEffect("ambiance", "calm", "chrono-calm")
	start := env.now
	ev := lighting.Event{Kind: lighting.KindIdle}
	env.step()
	env.now = start.Add(3 * time.Second)
	env.step() // tick 3 (LAST)
	// le moment se termine (partie lancée) puis reprend : l'effet repart du tick 0 (PRE)
	env.eng.StartImmediate(20)
	env.now = start.Add(4 * time.Second)
	env.step()
	env.eng.SetPhase(game.PhaseStopped) // retour au repos
	env.now = start.Add(10 * time.Second)
	env.step()
	if got := tw223SceneRow(env, ev, 4); got != "bbbb" {
		t.Errorf("nouveau départ du moment : PRE à nouveau (k = 0) : %q", got)
	}
	env.now = start.Add(11 * time.Second)
	env.step()
	if got := tw223SceneRow(env, ev, 4); got != "Wbbb" {
		t.Errorf("puis pas 0 avec gA = g1 (variables remises au départ) : %q", got)
	}
}

// ---------- vitesse ----------

const ticker1s = `id: tour
nom: Tour
version: 2
moments: [calm]
duree_tick_ms: 1000
variables:
  gA: g1; +1
pas:
  - { g0: ["#0000FF", 40], gA: ["#FFFFFF", 100] }
`

func tw223Head(env *tw223Env) int { // position du groupe blanc dans la trame de salle (1-based), 0 si aucun
	row := tw223SceneRow(env, lighting.Event{Kind: lighting.KindIdle}, 4)
	for i := range row {
		if row[i] == 'W' {
			return i + 1
		}
	}
	return 0
}

func TestReader_Speed_DefaultIsOne(t *testing.T) {
	env := tw223New(t)
	env.setup(tw223FourGroups())
	env.writeUser(t, "tour.yaml", ticker1s)
	env.setEffect("ambiance", "calm", "tour")
	start := env.now
	for tick, want := range []int{1, 2, 3, 4, 1} {
		env.now = start.Add(time.Duration(tick) * time.Second)
		env.step()
		if got := tw223Head(env); got != want {
			t.Errorf("sans réglage de vitesse, tick de 1 s : t = %d s, tête en %d, attendu %d", tick, got, want)
		}
	}
}

func TestReader_Speed_DividesTheTick_AllFiveValues(t *testing.T) {
	cases := []struct {
		speed float64
		tick  time.Duration // durée réelle attendue d'un tick
	}{
		{0.5, 2000 * time.Millisecond}, {0.75, 1333 * time.Millisecond}, {1, time.Second}, {1.5, 667 * time.Millisecond}, {2, 500 * time.Millisecond},
	}
	for _, c := range cases {
		env := tw223New(t)
		env.setup(tw223FourGroups())
		env.writeUser(t, "tour.yaml", ticker1s)
		env.setEffect("ambiance", "calm", "tour")
		env.setSpeed("calm", c.speed)
		start := env.now
		env.step()
		if tw223Head(env) != 1 {
			t.Fatalf("×%v : tick 0, tête en g1", c.speed)
		}
		// juste avant la fin du 1er tick : toujours g1 ; à la fin (+3 ms de tolérance d'arrondi) : g2
		env.now = start.Add(c.tick - 10*time.Millisecond)
		env.step()
		if got := tw223Head(env); got != 1 {
			t.Errorf("×%v : à %s la tête est encore en g1, obtenu g%d", c.speed, c.tick-10*time.Millisecond, got)
		}
		env.now = start.Add(c.tick + 3*time.Millisecond)
		env.step()
		if got := tw223Head(env); got != 2 {
			t.Errorf("×%v : à %s la tête passe en g2 (tick = 1000 ms ÷ %v), obtenu g%d", c.speed, c.tick, c.speed, got)
		}
	}
}

func TestReader_Speed_FloorOf250ms_ForFastEffects(t *testing.T) {
	env := tw223New(t)
	env.setup(tw223FourGroups())
	env.writeUser(t, "vite.yaml", `id: vite
nom: Vite
version: 2
moments: [calm]
duree_tick_ms: 400
variables:
  gA: g1; +1
pas:
  - { g0: ["#0000FF", 40], gA: ["#FFFFFF", 100] }
`)
	env.setEffect("ambiance", "calm", "vite")
	env.setSpeed("calm", 2) // 400 ÷ 2 = 200 ms : plafonné à 250 ms
	start := env.now
	env.step()
	env.now = start.Add(230 * time.Millisecond)
	env.step()
	if got := tw223Head(env); got != 1 {
		t.Errorf("à 230 ms (sous le plancher de 250 ms) la tête n'a pas bougé : g%d", got)
	}
	env.now = start.Add(255 * time.Millisecond)
	env.step()
	if got := tw223Head(env); got != 2 {
		t.Errorf("à 255 ms la tête passe en g2 : le tick réel est 250 ms, jamais 200 ms : g%d", got)
	}
}

func TestReader_Speed_ChangeTakenAtNextTick_EffectDoesNotRestart(t *testing.T) {
	env := tw223New(t)
	env.setup(tw223FourGroups())
	env.writeUser(t, "tour.yaml", ticker1s)
	env.setEffect("ambiance", "calm", "tour")
	start := env.now
	for s := 0; s <= 2; s++ { // ×1 : ticks 0, 1, 2 aux secondes 0, 1, 2 → tête en g3 à 2 s
		env.now = start.Add(time.Duration(s) * time.Second)
		env.step()
	}
	if got := tw223Head(env); got != 3 {
		t.Fatalf("à 2 s en ×1 : tête en g3 : g%d", got)
	}
	env.setSpeed("calm", 2) // le tick en cours (2) va à son terme ; les suivants durent 500 ms
	env.now = start.Add(2*time.Second + 600*time.Millisecond)
	env.step()
	if got := tw223Head(env); got != 3 {
		t.Errorf("le tick en cours n'est pas raccourci : à 2,6 s toujours g3 : g%d", got)
	}
	env.now = start.Add(3*time.Second + 20*time.Millisecond)
	env.step()
	if got := tw223Head(env); got != 4 {
		t.Errorf("tick 3 à 3 s : la tête continue en g4 (l'effet ne repart pas à g1) : g%d", got)
	}
	env.now = start.Add(3*time.Second + 520*time.Millisecond)
	env.step()
	if got := tw223Head(env); got != 1 {
		t.Errorf("tick 4 à 3,5 s (500 ms plus tard, ×2) : g1 (rebouclage, k continue) : g%d", got)
	}
}

func TestReader_Speed_IsPerMoment_NotPerEffect(t *testing.T) {
	env := tw223New(t)
	env.setup(tw223FourGroups())
	env.writeUser(t, "tour.yaml", ticker1s)
	env.setEffect("ambiance", "calm", "tour")
	env.setSpeed("chrono", 2) // un autre moment : sans effet sur « calm »
	start := env.now
	env.step()
	env.now = start.Add(600 * time.Millisecond)
	env.step()
	if got := tw223Head(env); got != 1 {
		t.Errorf("la vitesse de chrono ne touche pas calm : g%d", got)
	}
}

func TestReader_EachZonePlaysAtItsOwnSpeed(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), hostGrp("H1"), pubGrp("P1")})
	env.writeUser(t, "clignote.yaml", `id: clignote
nom: Clignote
version: 2
moments: [calm]
duree_tick_ms: 1000
pas:
  - { g0: ["#FF0000", 100] }
  - { g0: ["#00FF00", 100] }
`)
	env.setEffect("ambiance", "calm", "clignote")
	env.setHost("calm", "clignote")
	env.setPublic("calm", "clignote")
	env.setSpeedZone("animateur", "calm", 2) // l'animateur va deux fois plus vite
	env.setSpeedZone("public", "calm", 0.5)  // le public deux fois moins vite
	start := env.now
	env.step()
	ev := lighting.Event{Kind: lighting.KindIdle}
	at := func(ms int) (amb, host, pub [3]int) {
		env.now = start.Add(time.Duration(ms) * time.Millisecond)
		env.step()
		st := env.c.Scene(ev)
		amb = st.Zones[0].Color
		h, _ := areaEntry(st, lighting.AreaHost, 0)
		p, _ := areaEntry(st, lighting.AreaPublic, 0)
		return amb, h.Color, p.Color
	}
	red, green := [3]int{255, 0, 0}, [3]int{0, 255, 0}
	if a, h, p := at(0); a != red || h != red || p != red {
		t.Fatalf("tick 0 : les trois zones au pas 0 (rouge) : %v %v %v", a, h, p)
	}
	if a, h, p := at(600); a != red || h != green || p != red {
		t.Errorf("à 0,6 s : ambiance (×1) rouge, animateur (×2) vert, public (×0,5) rouge : %v %v %v", a, h, p)
	}
	if a, h, p := at(1100); a != green || h != red || p != red {
		t.Errorf("à 1,1 s : ambiance vert (tick 1), animateur rouge (tick 2), public rouge (tick 0) : %v %v %v", a, h, p)
	}
	if a, h, p := at(2100); a != red || h != red || p != green {
		t.Errorf("à 2,1 s : ambiance rouge (tick 2), animateur rouge (tick 4), public vert (tick 1 à 2 s) : %v %v %v", a, h, p)
	}
}

func TestReader_OneFramePerZone_NoCrossWrites(t *testing.T) {
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1, "B"), hostGrp("H1"), hostGrp("H2"), pubGrp("P1")})
	env.writeUser(t, "a.yaml", tw223Effect("amb-fx", "calm", "groups", 1, "#FF0000"))
	env.writeUser(t, "b.yaml", tw223Effect("host-fx", "calm", "groups", 1, "#00FF00"))
	env.writeUser(t, "c.yaml", tw223Effect("pub-fx", "calm", "groups", 1, "#0000FF"))
	env.setEffect("ambiance", "calm", "amb-fx")
	env.setHost("calm", "host-fx")
	env.setPublic("calm", "pub-fx")
	env.step()
	st := env.c.Scene(lighting.Event{Kind: lighting.KindIdle})
	check := func(area lighting.Area, n int, want [3]int, label string) {
		t.Helper()
		got := 0
		for _, z := range areaEntries(st, area) {
			if z.Group >= 1 {
				got++
				if z.Color != want {
					t.Errorf("%s, groupe %d : %v, attendu %v (aucune écriture croisée)", label, z.Group, z.Color, want)
				}
			}
		}
		if got != n {
			t.Errorf("%s : %d entrées de groupe, attendu %d", label, got, n)
		}
	}
	check(lighting.AreaAmbiance, 2, [3]int{255, 0, 0}, "ambiance")
	check(lighting.AreaHost, 2, [3]int{0, 255, 0}, "animateur")
	check(lighting.AreaPublic, 1, [3]int{0, 0, 255}, "public")
}

func TestReader_G0OnlyEffect_FrameIsASingleGroupZeroEntry(t *testing.T) {
	// §4.6 : un effet qui n'écrit que g0 produit une seule entrée Group 0 — une seule écriture buzzmaster-general
	env := tw223New(t)
	env.setup(tw223FourGroups())
	env.writeUser(t, "all.yaml", tw223Effect("tout", "calm", "all", 1, "#FF0000"))
	env.setEffect("ambiance", "calm", "tout")
	env.step()
	st := env.c.Scene(lighting.Event{Kind: lighting.KindIdle})
	if g := roomGroups(st); len(g) != 0 {
		t.Errorf("aucune entrée de groupe : %+v", g)
	}
	if st.Zones[0].Group != 0 || st.Zones[0].Color != [3]int{255, 0, 0} {
		t.Errorf("l'entrée Group 0 porte la couleur de l'effet : %+v", st.Zones[0])
	}
}

func TestReader_FixedGroupTargets_AreRealNumbers_EmptyOrAbsentIgnored(t *testing.T) {
	// groupes réels 1, 2 (vide), 3 : g1 et g3 existent, g2 est vide ⇒ action ignorée ; g4 n'existe pas ⇒ ignorée (§4.3)
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1), grp(2, "C")})
	env.writeUser(t, "pos.yaml", "id: pos\nnom: Pos\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g1: [\"#FF0000\", 100], g2: [\"#00FF00\", 100], g3: [\"#0000FF\", 100], g4: [\"#FFFFFF\", 100] }\n")
	env.setEffect("ambiance", "calm", "pos")
	env.step()
	g := roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle}))
	if len(g) != 2 || g[0].Group != 1 || g[1].Group != 3 || g[0].Color != [3]int{255, 0, 0} || g[1].Color != [3]int{0, 0, 255} {
		t.Errorf("gK = numéro réel : g1 → groupe 1 rouge, g3 → groupe 3 bleu ; g2 (vide) et g4 (absent) ignorés : %+v", g)
	}
}

func TestReader_VariablesProgressOnNonEmptyGroups_WithRealNumbersOnTheWire(t *testing.T) {
	// groupes réels 1, 2 (vide), 3 : N = 2 ; gA: g1; +1 parcourt les positions 1, 2 = groupes réels 1 et 3
	env := tw223New(t)
	env.setup([]config.LightingGroupEntry{grp(0, "A"), grp(1), grp(2, "C")})
	env.writeUser(t, "tour.yaml", ticker1s)
	env.setEffect("ambiance", "calm", "tour")
	start := env.now
	var heads []int
	for tick := 0; tick < 4; tick++ {
		env.now = start.Add(time.Duration(tick) * time.Second)
		env.step()
		for _, z := range roomGroups(env.c.Scene(lighting.Event{Kind: lighting.KindIdle})) {
			if z.Color == [3]int{255, 255, 255} {
				heads = append(heads, z.Group)
			}
		}
	}
	if len(heads) != 4 || heads[0] != 1 || heads[1] != 3 || heads[2] != 1 || heads[3] != 3 {
		t.Errorf("la tête alterne entre les groupes RÉELS 1 et 3 (position 1, 2) : %v", heads)
	}
}
