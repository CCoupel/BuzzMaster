package effects

// Effets natifs du LOT-2 (contracts/light-effects.md §9 : chenillard, pulsation, couleur-equipe)
// et budget d'écriture §5.4. Lit les fichiers réels de server-go/assets/effects.

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

func TestNatives_LOT2Set_PresentAndNative(t *testing.T) {
	k := loadNatives(t)
	for _, id := range []string{"chenillard", "pulsation", "couleur-equipe"} {
		if o, _, ok := k.get(id); !ok || o != "native" {
			t.Errorf("%s : natif attendu (%q, %v)", id, o, ok)
		}
	}
}

func hasMoment(e Effect, m string) bool {
	for _, x := range e.Moments {
		if x == m {
			return true
		}
	}
	return false
}

func TestNatives_Chenillard_Declaration(t *testing.T) {
	e := nativeEffect(t, "chenillard")
	if !hasMoment(e, "chrono") || e.MinGroups < 3 || e.Target != "groups" {
		t.Errorf("chenillard : moment chrono, min_groups ≥ 3 (#223), target groups : %+v", e)
	}
}

func TestNatives_Chenillard_RendersLikeIssue223(t *testing.T) {
	e := nativeEffect(t, "chenillard")
	// tête urgence, traînée à 50 %, fond thème, un groupe par seconde, sans traînée sous 5 groupes
	c := chrono(6, 15*time.Second)
	h0 := headIndex(t, render(e, c, 0), white)
	if h0 < 0 {
		t.Fatal("tête à la couleur d'urgence introuvable")
	}
	if got := headIndex(t, render(e, c, time.Second), white); got != (h0+1)%6 {
		t.Errorf("un groupe par seconde : %d puis %d", h0, got)
	}
	fr := render(e, c, 0)
	trail := 0
	for _, l := range fr {
		if l.Color == theme && near(l.Intensity, 128, 1) {
			trail++
		}
	}
	if trail != 2 {
		t.Errorf("traînée de 2 cases à 50 %% pour 6 groupes : %+v", fr)
	}
	if headIndex(t, render(e, chrono(6, 4*time.Second), 0), red) < 0 {
		t.Error("tête rouge à 5 s ou moins")
	}
	f4 := render(e, chrono(4, 15*time.Second), 0)
	for _, l := range f4 {
		if l.Color == theme && near(l.Intensity, 128, 1) {
			t.Errorf("aucune traînée sous 5 groupes : %+v", f4)
		}
	}
}

func TestNatives_Pulsation_Declaration_AppliesWithoutGroups(t *testing.T) {
	e := nativeEffect(t, "pulsation")
	if !hasMoment(e, "chrono") || e.Target != "all" {
		t.Errorf("pulsation : moment chrono, target all : %+v", e)
	}
	// Non-régression v11.6.0 : sans aucun groupe, la pulsation de salle fonctionnait.
	if ok, reason := applicable(e, 0); !ok {
		t.Errorf("pulsation doit rester applicable avec N = 0 (comportement v11.6.0) : %q", reason)
	}
	if ok, _ := applicable(e, 12); !ok {
		t.Error("applicable avec 12 groupes")
	}
}

func TestNatives_Pulsation_ShapeMatchesLegacyCycle(t *testing.T) {
	// Contrat §5.5 : cycle de 1 s, intensités 255 / 128 / 77 selon le palier, fondu 0, une trame d'un seul état.
	e := nativeEffect(t, "pulsation")
	for _, rem := range []time.Duration{15 * time.Second, 8 * time.Second, 3 * time.Second} {
		for _, at := range []time.Duration{0, 100 * time.Millisecond, 400 * time.Millisecond, 600 * time.Millisecond, 900 * time.Millisecond} {
			fr := render(e, chrono(6, rem), at)
			if len(fr) != 1 {
				t.Fatalf("pulsation : un seul état (target all), obtenu %d", len(fr))
			}
			if fr[0].FadeMs != 0 {
				t.Errorf("fondu 0 (instantané, v10.0.0.17) : %+v", fr[0])
			}
			if i := fr[0].Intensity; i != 255 && !near(i, 128, 1) && !near(i, 77, 1) {
				t.Errorf("intensité ∈ {255,128,77} : %d (reste %s, t=%s)", i, rem, at)
			}
		}
	}
	// Alternance : sur un cycle de 1 s, deux apparences au moins.
	seen := map[Lk]bool{}
	for ms := 0; ms < 1000; ms += 50 {
		seen[render(e, chrono(6, 15*time.Second), time.Duration(ms)*time.Millisecond)[0]] = true
	}
	if len(seen) != 2 {
		t.Errorf("deux phases par cycle : %d apparence(s)", len(seen))
	}
}

func TestNatives_Pulsation_Tier3Is250msTheme750msRed(t *testing.T) {
	e := nativeEffect(t, "pulsation")
	c := chrono(6, 3*time.Second) // palier 3 : ≤ 5 s
	at := func(ms int) Lk { return render(e, c, time.Duration(ms)*time.Millisecond)[0] }
	if at(100).Color != theme {
		t.Errorf("0–250 ms : thème, obtenu %v", at(100).Color)
	}
	if got := at(250).Color; got == theme {
		t.Errorf("à partir de 250 ms : rouge dominant, obtenu %v", got)
	}
	if at(999).Color != at(300).Color {
		t.Error("250–1000 ms : même apparence (750 ms)")
	}
}

func TestNatives_CouleurEquipe_Declaration(t *testing.T) {
	e := nativeEffect(t, "couleur-equipe")
	if !hasMoment(e, "score") || e.DurationMs <= 0 || e.MinGroups != 1 {
		t.Errorf("couleur-equipe : moment score, duration_ms > 0, min_groups 1 : %+v", e)
	}
	for _, n := range []int{1, 3, 12} {
		if ok, reason := applicable(e, n); !ok {
			t.Errorf("N=%d : %q", n, reason)
		}
	}
}

func TestNatives_CouleurEquipe_FixedTeamColourOnEveryGroup(t *testing.T) {
	e := nativeEffect(t, "couleur-equipe")
	for _, n := range []int{1, 6} {
		for _, at := range []time.Duration{0, 700 * time.Millisecond} {
			for i, l := range render(e, Ctx{N: n, Scene: white, Theme: theme, Team: team}, at) {
				if l.Color != team || l.Intensity < 250 {
					t.Errorf("N=%d t=%s groupe %d : %+v, attendu la couleur de l'équipe à pleine intensité", n, at, i+1, l)
				}
			}
		}
	}
}

// ---------- budget d'écriture §5.4 : ≤ 6 par pas, ≤ 10 par seconde, 12 groupes ----------

// changes compte les groupes dont l'état diffère de la trame précédente.
func changes(prev, cur []Lk) int {
	if prev == nil {
		return len(cur)
	}
	n := 0
	for i := range cur {
		if cur[i] != prev[i] {
			n++
		}
	}
	return n
}

func TestNatives_WriteBudget_PerStepAndPerSecond_12Groups(t *testing.T) {
	const step = 250 * time.Millisecond // cadence de l'écrivain
	moments := map[string]Ctx{
		"chrono": chrono(12, 20*time.Second),
		"score":  {N: 12, Scene: white, Theme: theme, Team: team, Origin: 1, Seed: 1},
	}
	for _, id := range []string{"chenillard", "pulsation", "couleur-equipe"} {
		e := nativeEffect(t, id)
		for _, m := range e.Moments {
			c, ok := moments[m]
			if !ok {
				continue
			}
			var prev []Lk
			perSecond := 0
			worstStep := 0
			total := 20 * time.Second
			if m == "score" {
				total = time.Duration(e.DurationMs) * time.Millisecond
			}
			for at := time.Duration(0); at < total; at += step {
				cc := c
				if m == "chrono" {
					cc.Remaining = c.Remaining - at
					if cc.Remaining < 0 {
						cc.Remaining = 0
					}
				}
				cur := render(e, cc, at)
				// une trame « target: all » compte pour 1 écriture de groupe Hue, quel que soit N
				n := changes(prev, cur)
				if prev != nil {
					if n > worstStep {
						worstStep = n
					}
					perSecond += n
				}
				prev = cur
				if (at+step)%time.Second == 0 {
					if perSecond > 10 {
						t.Errorf("%s/%s : %d écritures dans la seconde finissant à %s (max 10)", id, m, perSecond, at+step)
					}
					perSecond = 0
				}
			}
			if worstStep > 6 {
				t.Errorf("%s/%s : %d écritures en un pas (max 6)", id, m, worstStep)
			}
		}
	}
}
