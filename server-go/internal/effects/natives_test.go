package effects

// Effets natifs (révision 3, contrat §9) : pulsation, chenillard, chenillard-traine, couleur-equipe, degrade — valides,
// applicables selon groupes_min et §4.7, rendu conforme, budget d'écriture §5.4 (≤ 6 par pas, ≤ 10 par seconde, 12 groupes).

import (
	"testing"
	"time"
)

func loadNatives(t *testing.T) catalogue {
	t.Helper()
	k := loadFS(nativeFS(t), t.TempDir())
	if len(k.errors()) != 0 {
		t.Fatalf("les natifs doivent tous être valides : %+v", k.errors())
	}
	return k
}

func nativeEffect(t *testing.T, id string) Effect {
	t.Helper()
	e, ok := loadNatives(t).effect(id)
	if !ok {
		t.Fatalf("natif %q absent du binaire", id)
	}
	return e
}

func hasMoment(e Effect, m string) bool {
	for _, x := range e.Moments {
		if x == m {
			return true
		}
	}
	return false
}

func TestNatives_Set_AllPresentValidAndNative(t *testing.T) {
	k := loadNatives(t)
	for _, id := range []string{"pulsation", "chenillard", "chenillard-traine", "couleur-equipe", "degrade"} {
		if o, f, ok := k.get(id); !ok || o != "native" || f == "" {
			t.Errorf("%s : natif attendu (%q %q %v)", id, o, f, ok)
		}
	}
	for _, gone := range []string{"remplissage", "scintillement", "vague", "vague-lente"} {
		if _, _, ok := k.get(gone); ok {
			t.Errorf("%s est sorti de v11.7.0 (noyau seul)", gone)
		}
	}
}

func TestNatives_Declarations_MomentsMinGroupsTick(t *testing.T) {
	type decl struct {
		moment string
		min    int
		tick   int
	}
	for id, d := range map[string]decl{
		"pulsation": {"chrono", 0, 500}, "chenillard": {"chrono", 3, 1000}, "chenillard-traine": {"chrono", 5, 0},
		"couleur-equipe": {"score", 0, 1000}, "degrade": {"calm", 2, 60000},
	} {
		e := nativeEffect(t, id)
		if !hasMoment(e, d.moment) {
			t.Errorf("%s : moment %s attendu : %v", id, d.moment, e.Moments)
		}
		if e.MinGroups != d.min {
			t.Errorf("%s : groupes_min %d, attendu %d", id, e.MinGroups, d.min)
		}
		if d.tick > 0 && e.TickMs != d.tick {
			t.Errorf("%s : duree_tick_ms %d, attendu %d", id, e.TickMs, d.tick)
		}
		if e.TickMs < 250 {
			t.Errorf("%s : tick ≥ 250 ms", id)
		}
	}
}

func TestNatives_Applicability(t *testing.T) {
	for id, cases := range map[string]map[int]bool{
		"pulsation":         {0: true, 1: true, 12: true},
		"chenillard":        {0: false, 2: false, 3: true, 12: true},
		"chenillard-traine": {3: false, 4: false, 5: true, 12: true},
		"couleur-equipe":    {0: true, 1: true, 12: true},
		"degrade":           {0: false, 1: false, 2: true, 12: true},
	} {
		e := nativeEffect(t, id)
		for n, want := range cases {
			ok, reason := applicable(e, n)
			if ok != want {
				t.Errorf("%s N=%d : applicable=%v, attendu %v", id, n, ok, want)
			}
			if !ok && reason == "" {
				t.Errorf("%s N=%d : une raison est exigée", id, n)
			}
		}
	}
}

func TestNatives_HostApplicable_OnlyG0Effects(t *testing.T) {
	for id, want := range map[string]bool{"pulsation": true, "couleur-equipe": true, "chenillard": false, "chenillard-traine": false, "degrade": false} {
		if got := hostApplicable(nativeEffect(t, id)); got != want {
			t.Errorf("%s : jouable sur un groupe animateur = %v, attendu %v (seuls les effets qui n'écrivent que g0)", id, got, want)
		}
	}
}

// ---------- chenillard ----------

func headAt(v View, n int) int { // position (1-based) du groupe à la couleur d'urgence à pleine intensité, ou -1
	idx := -1
	for p := 1; p <= n; p++ {
		l := v.effective(p)
		if l.Color != cT && l.Intensity >= 254 {
			if idx != -1 {
				return -2 // plusieurs têtes
			}
			idx = p
		}
	}
	return idx
}

func TestNatives_Chenillard_HeadWalksOneGroupPerTick_4And8Groups(t *testing.T) {
	e := nativeEffect(t, "chenillard")
	for _, n := range []int{4, 8} {
		for tick := 0; tick < 2*n+2; tick++ {
			v := renderView(e, ctxN(n), tick)
			if h := headAt(v, n); h != tick%n+1 {
				t.Fatalf("N=%d tick %d : tête en %d, attendu %d (un groupe par tick, départ g1, retour au premier)", n, tick, h, tick%n+1)
			}
			for p := 1; p <= n; p++ {
				if p != tick%n+1 && (v.effective(p).Color != cT || v.effective(p).Intensity != 255) {
					t.Errorf("N=%d tick %d : fond au thème à 100 %% sur le groupe %d : %+v", n, tick, p, v.effective(p))
				}
			}
		}
	}
}

func TestNatives_Chenillard_HeadColourIsUrgency_LiveWithRemainingTime(t *testing.T) {
	e := nativeEffect(t, "chenillard")
	for _, c := range []struct {
		rem  time.Duration
		want [3]int
	}{{20 * time.Second, cW}, {10 * time.Second, cOr}, {5 * time.Second, cRed}} {
		cx := ctxN(4)
		cx.Remaining = c.rem
		if got := renderView(e, cx, 2).effective(3).Color; got != c.want {
			t.Errorf("reste %s : tête %v, attendu %v", c.rem, got, c.want)
		}
	}
}

func TestNatives_ChenillardTraine_HeadWithHalfIntensityTrail_5And8Groups(t *testing.T) {
	e := nativeEffect(t, "chenillard-traine")
	for _, n := range []int{5, 8} {
		for tick := 0; tick < 2*n; tick++ {
			v := renderView(e, ctxN(n), tick)
			head := tick%n + 1
			prev, next := (head+n-2)%n+1, head%n+1
			if headAt(v, n) != head {
				t.Fatalf("N=%d tick %d : tête en %d : %v", n, tick, headAt(v, n), v.Groups)
			}
			for _, p := range []int{prev, next} {
				l := v.effective(p)
				if l.Color != cT || !near(l.Intensity, 128, 1) {
					t.Errorf("N=%d tick %d : traînée au thème à 50 %% sur %d : %+v", n, tick, p, l)
				}
			}
			for p := 1; p <= n; p++ {
				if p == head || p == prev || p == next {
					continue
				}
				if l := v.effective(p); l.Color != cT || l.Intensity != 255 {
					t.Errorf("N=%d tick %d : fond au thème à 100 %% sur %d : %+v", n, tick, p, l)
				}
			}
		}
	}
}

// ---------- pulsation (§5.5) ----------

func TestNatives_Pulsation_HighPhaseThemeFull_LowPhaseUrgencyHalf_500ms(t *testing.T) {
	e := nativeEffect(t, "pulsation")
	if e.TickMs != 500 {
		t.Fatalf("tick de 500 ms : %d", e.TickMs)
	}
	for _, c := range []struct {
		rem  time.Duration
		want [3]int
	}{{20 * time.Second, cW}, {8 * time.Second, cOr}, {3 * time.Second, cRed}} {
		cx := ctxN(6)
		cx.Remaining = c.rem
		hi, lo := renderView(e, cx, 0), renderView(e, cx, 1)
		if hi.Base == nil || lo.Base == nil {
			t.Fatalf("la pulsation n'écrit que g0 : une base à chaque tick")
		}
		if hi.Base.Color != cT || hi.Base.Intensity != 255 {
			t.Errorf("reste %s, phase haute : thème à 100 %% : %+v", c.rem, *hi.Base)
		}
		if lo.Base.Color != c.want || !near(lo.Base.Intensity, 128, 1) {
			t.Errorf("reste %s, phase basse : %v à 50 %% : %+v", c.rem, c.want, *lo.Base)
		}
		// cycle de 2 ticks (1 s) : 500 / 500 ms, y compris à 5 s ou moins (plus de battement 250 / 750)
		if renderView(e, cx, 2).Base.Intensity != 255 || !near(renderView(e, cx, 3).Base.Intensity, 128, 1) {
			t.Errorf("reste %s : alternance régulière", c.rem)
		}
	}
}

func TestNatives_Pulsation_WritesOnlyG0_NoGroupState_AtAnyN(t *testing.T) {
	e := nativeEffect(t, "pulsation")
	for _, n := range []int{0, 1, 4, 12} {
		for tick := 0; tick < 4; tick++ {
			v := renderView(e, ctxN(n), tick)
			for i, g := range v.Groups {
				if !g.Absent {
					t.Errorf("N=%d tick %d : groupe %d avec état propre : une seule écriture de groupe par phase", n, tick, i+1)
				}
			}
		}
	}
}

func TestNatives_Pulsation_NewRenderingIsNotTheLegacyOne(t *testing.T) {
	// décision utilisateur (§5.5) : à plus de 10 s la phase basse est BLANCHE à 50 % (v11.6.0 : thème à 50 %) ;
	// à 5 s ou moins rouge à 50 % (v11.6.0 : 30 %).
	e := nativeEffect(t, "pulsation")
	cx := ctxN(4)
	if l := renderView(e, cx, 1).Base; l.Color != cW {
		t.Errorf("plus de 10 s : phase basse blanche : %+v", *l)
	}
	cx.Remaining = 4 * time.Second
	if l := renderView(e, cx, 1).Base; !near(l.Intensity, 128, 1) {
		t.Errorf("5 s ou moins : rouge à 50 %% (pas 30 %%) : %+v", *l)
	}
}

// ---------- couleur-equipe ----------

func TestNatives_CouleurEquipe_TeamColourOnWholeZone(t *testing.T) {
	e := nativeEffect(t, "couleur-equipe")
	for _, n := range []int{0, 1, 6} {
		cx := ctxN(n)
		cx.Moment = "score"
		v := renderView(e, cx, 0)
		if v.Base == nil || v.Base.Color != cTeam || v.Base.Intensity != 255 {
			t.Errorf("N=%d : couleur de l'équipe créditée à 100 %% sur g0 : %+v", n, v.Base)
		}
	}
}

// ---------- dégradé (#211) ----------

func TestNatives_Degrade_OneColourPerGroup_CycleOfFour_StartOfPaletteForSmallRooms(t *testing.T) {
	e := nativeEffect(t, "degrade")
	cx := ctxN(12)
	cx.Moment = "calm"
	big := renderView(e, cx, 0)
	for p := 1; p <= 12; p++ {
		if l := big.effective(p); l.Absent || l.Intensity == 0 {
			t.Errorf("groupe %d : une couleur visible : %+v", p, l)
		}
	}
	seen := map[[3]int]bool{}
	for p := 1; p <= 4; p++ {
		seen[big.effective(p).Color] = true
	}
	if len(seen) != 4 {
		t.Errorf("cycle de 4 couleurs distinctes sur g1…g4 : %v", seen)
	}
	for p := 1; p <= 8; p++ {
		if big.effective(p).Color != big.effective(p+4).Color {
			t.Errorf("le cycle se répète tous les 4 groupes : g%d / g%d", p, p+4)
		}
	}
	small := ctxN(2)
	small.Moment = "calm"
	sv := renderView(e, small, 0)
	for p := 1; p <= 2; p++ {
		if sv.effective(p).Color != big.effective(p).Color {
			t.Errorf("une salle plus petite montre le début de la palette : groupe %d", p)
		}
	}
	// scène fixe : identique à tous les ticks
	if renderView(e, cx, 5).effective(7) != big.effective(7) {
		t.Error("le dégradé est fixe dans le temps")
	}
}

// ---------- budget d'écriture §5.4 ----------

// writes : nombre d'écritures pour passer de prev à cur — 1 si toute la zone change ensemble (une écriture
// buzzmaster-general), sinon une par groupe dont l'état effectif change.
func writes(prev, cur View, n int) int {
	changed, allSame := 0, true
	for p := 1; p <= n; p++ {
		if cur.effective(p) != cur.effective(1) {
			allSame = false
		}
		if prev.effective(p) != cur.effective(p) {
			changed++
		}
	}
	if allSame {
		if changed > 0 {
			return 1
		}
		return 0
	}
	return changed
}

func TestNatives_WriteBudget_PerTickAndPerSecond_12Groups(t *testing.T) {
	for _, id := range []string{"pulsation", "chenillard", "chenillard-traine", "couleur-equipe", "degrade"} {
		e := nativeEffect(t, id)
		cx := ctxN(12)
		cx.Moment = e.Moments[0]
		tick := time.Duration(e.TickMs) * time.Millisecond
		if tick < 250*time.Millisecond {
			tick = 250 * time.Millisecond
		}
		perSecond, worst := 0.0, 0
		prev := renderView(e, cx, 0)
		for k := 1; k < 40; k++ {
			cur := renderView(e, cx, k)
			w := writes(prev, cur, 12)
			if w > worst {
				worst = w
			}
			if r := float64(w) * float64(time.Second) / float64(tick); r > perSecond {
				perSecond = r
			}
			prev = cur
		}
		if worst > 6 {
			t.Errorf("%s : %d écritures en un tick (max 6)", id, worst)
		}
		if perSecond > 10 {
			t.Errorf("%s : %.1f écritures par seconde en régime établi (max 10)", id, perSecond)
		}
	}
}
