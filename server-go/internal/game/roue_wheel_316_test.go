package game

// #316 (v12.2.0, LOT-1) — génération de la roue avec BONUS posé sur une case à points :
// spécification de contracts/roue.md §17.2 et §17.3 (rév. 9.1). Tests de PROPRIÉTÉS à graine fixe, jamais de positions
// exactes de BONUS (le tirage est aléatoire) ; seules les roues sans choix possible ont une réponse exacte.
//
// Identifiants Go attendus du dev-backend (le contrat fixe les noms JSON, pas les noms Go) :
//   - RoueSegment.Bonus (bool, JSON "BONUS", sans omitempty) ; RoueKindBonus supprimé
//   - BuildRoueWheel(p RoueWheelParams, randN func(n int) int) []RoueSegment   (randN(n) ∈ [0, n))
//   - RoueWheelSize(p) = V + B + P (BonusCount ignoré) ; RoueMaxBonusFor(p) = min(4, place libre, ⌊(V−B−P)/2⌋)
//   - ValidateRoueBonusCount(n, p) refuse n au-dessus de RoueMaxBonusFor(p) avec RoueErrBonusCountInvalid
// Attendu ROUGE tant que la tâche 1 du plan n'est pas livrée (compilation comprise).

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// seeded316 renvoie un randN reproductible.
func seeded316(seed int64) func(int) int {
	r := rand.New(rand.NewSource(seed))
	return func(n int) int {
		if n <= 0 {
			return 0
		}
		return r.Intn(n)
	}
}

func norm316(t *testing.T, p RoueWheelParams) RoueWheelParams {
	t.Helper()
	np, _, err := NormalizeRoueWheelParams(p)
	if err != nil {
		t.Fatalf("NormalizeRoueWheelParams(%+v) : %v", p, err)
	}
	return np
}

// build316 normalise p, fixe le nombre de BONUS et construit la roue.
func build316(t *testing.T, p RoueWheelParams, x int, rn func(int) int) []RoueSegment {
	t.Helper()
	np := norm316(t, p)
	np.BonusCount = x
	return BuildRoueWheel(np, rn)
}

func isSpecial316(s RoueSegment) bool { return s.Kind != RoueKindValue || s.Bonus }

// render316 : « 1000 B 950 800+ P » (B = BANQUEROUTE, P = TOUR PERDU, « + » = marque BONUS).
func render316(segs []RoueSegment) string {
	var parts []string
	for _, s := range segs {
		switch s.Kind {
		case RoueKindBankrupt:
			parts = append(parts, "B")
		case RoueKindLoseTurn:
			parts = append(parts, "P")
		default:
			v := fmt.Sprint(s.Value)
			if s.Bonus {
				v += "+"
			}
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " ")
}

// configs316 : roues variées (avec et sans BANQUEROUTE / PERDU, petites, grandes, pas corrigé).
var configs316 = []RoueWheelParams{
	DefaultRoueWheelParams(), // 1000/50/2/1, 22 cases
	{MaxPoints: 1000, Step: 100, BankruptCount: 2, LoseTurnCount: 1}, // 12 cases
	{MaxPoints: 1000, Step: 100, BankruptCount: 3, LoseTurnCount: 2}, // 14 cases
	{MaxPoints: 100, Step: 10, BankruptCount: 0, LoseTurnCount: 0},   // sans spéciale
	{MaxPoints: 100, Step: 10, BankruptCount: 0, LoseTurnCount: 2},   // PERDU seul
	{MaxPoints: 100, Step: 10, BankruptCount: 3, LoseTurnCount: 0},   // BANQUEROUTE seule
	{MaxPoints: 100, Step: 20, BankruptCount: 1, LoseTurnCount: 1},   // 7 cases
	{MaxPoints: 100, Step: 30, BankruptCount: 1, LoseTurnCount: 1},   // pas corrigé à 25
	{MaxPoints: 500, Step: 100, BankruptCount: 3, LoseTurnCount: 2},  // 8 cases
	{MaxPoints: 2000, Step: 50, BankruptCount: 4, LoseTurnCount: 3},  // 44 cases
	{MaxPoints: 3100, Step: 50, BankruptCount: 0, LoseTurnCount: 0},  // 62 cases
	{MaxPoints: 60, Step: 1, BankruptCount: 2, LoseTurnCount: 1},     // 62 cases
	{MaxPoints: 1000, Step: 50, BankruptCount: 0, LoseTurnCount: 1},  // PERDU seul, grande roue
	{MaxPoints: 1000, Step: 100, BankruptCount: 1, LoseTurnCount: 0}, // BANQUEROUTE seule
	{MaxPoints: 200, Step: 100, BankruptCount: 0, LoseTurnCount: 0},  // 2 cases
	{MaxPoints: 300, Step: 100, BankruptCount: 1, LoseTurnCount: 0},  // 4 cases
}

func cfgName316(p RoueWheelParams) string {
	return fmt.Sprintf("%d/%d/%d/%d", p.MaxPoints, p.Step, p.BankruptCount, p.LoseTurnCount)
}

// intervals316 : longueurs des intervalles de cases à points entre deux cases BANQUEROUTE / PERDU consécutives, sur la
// roue de base (sans BONUS) ; renvoie, pour chaque case à points, l'identifiant de son intervalle (-1 pour les spéciales).
func intervals316(base []RoueSegment) (lengths []int, of []int) {
	n := len(base)
	of = make([]int, n)
	for i := range of {
		of[i] = -1
	}
	var specials []int
	for i, s := range base {
		if s.Kind != RoueKindValue {
			specials = append(specials, i)
		}
	}
	if len(specials) == 0 {
		return nil, of
	}
	for k, s := range specials {
		next := specials[(k+1)%len(specials)]
		length := 0
		for i := (s + 1) % n; i != next; i = (i + 1) % n {
			of[i] = k
			length++
			if length > n {
				break
			}
		}
		if len(specials) == 1 {
			length = n - 1
		}
		lengths = append(lengths, length)
	}
	return lengths, of
}

// ---- exemples de §10.1 : inchangés -----------------------------------------------------------------------------

// critical — avec 0 BONUS la roue est celle de v12.0 case pour case (roues d'exemple figées de v12.0.0).
func TestRoueWheel316_ZeroBonusIsV120Wheel(t *testing.T) {
	golden := []struct {
		p    RoueWheelParams
		want string
	}{
		{DefaultRoueWheelParams(), "1000 B 950 50 900 100 850 P 150 800 200 750 250 700 B 300 650 350 600 400 550 450"},
		{RoueWheelParams{MaxPoints: 1000, Step: 100}, "1000 100 900 200 800 300 700 400 600 500"},
		{RoueWheelParams{MaxPoints: 500, Step: 100, BankruptCount: 3, LoseTurnCount: 2}, "500 B P 400 B 100 P 300 B 200"},
		{RoueWheelParams{MaxPoints: 1000, Step: 100, BankruptCount: 3, LoseTurnCount: 2}, "1000 B P 900 100 B 800 200 P 700 300 B 600 400"},
	}
	for _, g := range golden {
		t.Run(cfgName316(g.p), func(t *testing.T) {
			segs := build316(t, g.p, 0, seeded316(1))
			if got := render316(segs); got != g.want {
				t.Fatalf("roue v12.0 modifiée :\n got  %s\n want %s", got, g.want)
			}
			for i, s := range segs {
				if s.ID != fmt.Sprintf("s%d", i) {
					t.Fatalf("ID de la case %d = %q, attendu s%d", i, s.ID, i)
				}
				if s.Bonus {
					t.Fatalf("la case %d porte une marque avec 0 BONUS", i)
				}
			}
		})
	}
}

// L'aléa ne sert à rien quand X = 0 : deux graines donnent la même roue, et l'aléa n'est pas consommé de façon observable.
func TestRoueWheel316_ZeroBonusIgnoresRandomSource(t *testing.T) {
	for _, p := range configs316 {
		a := build316(t, p, 0, seeded316(1))
		b := build316(t, p, 0, seeded316(999))
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("%s : la roue sans BONUS dépend de l'aléa", cfgName316(p))
		}
	}
}

// ---- propriétés §17.2 ------------------------------------------------------------------------------------------

// critical — pour toute roue, tout X permis et des milliers de tirages : N cases, valeurs / BANQUEROUTE / PERDU et ID
// identiques à la roue sans BONUS, X marques exactement sur des cases `value`, aucune marque voisine d'une case
// spéciale (circulairement) ni d'une autre marque, au plus ⌊(g−1)/2⌋ marques par intervalle.
func TestRoueWheel316_Properties(t *testing.T) {
	for _, p := range configs316 {
		np := norm316(t, p)
		base := build316(t, p, 0, seeded316(1))
		max := RoueMaxBonusFor(np)
		lengths, of := intervals316(base)
		t.Run(cfgName316(p), func(t *testing.T) {
			if got := RoueWheelSize(np); got != len(base) {
				t.Fatalf("RoueWheelSize = %d, la roue a %d cases", got, len(base))
			}
			for x := 1; x <= max; x++ {
				for seed := int64(1); seed <= 400; seed++ {
					segs := build316(t, p, x, seeded316(seed*7919+int64(x)))
					if len(segs) != len(base) {
						t.Fatalf("X=%d : %d cases, attendu %d (les BONUS ne comptent plus)", x, len(segs), len(base))
					}
					marks := 0
					perInterval := map[int]int{}
					for i, s := range segs {
						if s.ID != base[i].ID || s.Kind != base[i].Kind || s.Value != base[i].Value {
							t.Fatalf("X=%d graine %d : la case %d a changé (%+v → %+v)", x, seed, i, base[i], s)
						}
						if !s.Bonus {
							continue
						}
						marks++
						if s.Kind != RoueKindValue {
							t.Fatalf("X=%d : marque sur une case %q", x, s.Kind)
						}
						perInterval[of[i]]++
						for _, d := range []int{-1, 1} {
							nb := segs[(i+d+len(segs))%len(segs)]
							if isSpecial316(nb) {
								t.Fatalf("X=%d graine %d : la marque de la case %d touche une case spéciale\n%s", x, seed, i, render316(segs))
							}
						}
					}
					if marks != x {
						t.Fatalf("X=%d : %d marques\n%s", x, marks, render316(segs))
					}
					for k, c := range perInterval {
						if k >= 0 && k < len(lengths) && c > (lengths[k]-1)/2 {
							t.Fatalf("X=%d : %d marques dans un intervalle de %d cases (capacité %d)\n%s", x, c, lengths[k], (lengths[k]-1)/2, render316(segs))
						}
					}
				}
			}
		})
	}
}

// Le placement réussit TOUJOURS quand X respecte §17.3, même avec une source d'aléa pathologique (toujours le plus
// petit tirage, toujours le plus grand) : pas de repli.
func TestRoueWheel316_NeverFailsAtTheMaximumWithAdversarialRandom(t *testing.T) {
	sources := map[string]func(int) int{
		"toujours 0":      func(int) int { return 0 },
		"toujours n-1":    func(n int) int { return n - 1 },
		"alternance":      func() func(int) int { i := 0; return func(n int) int { i++; return (i * 7) % n } }(),
		"milieu":          func(n int) int { return n / 2 },
		"graine fixe 424": seeded316(424),
	}
	for _, p := range configs316 {
		np := norm316(t, p)
		max := RoueMaxBonusFor(np)
		for name, rn := range sources {
			segs := build316(t, p, max, rn)
			got := 0
			for _, s := range segs {
				if s.Bonus {
					got++
				}
			}
			if got != max {
				t.Fatalf("%s / %s : %d marques au lieu de %d\n%s", cfgName316(p), name, got, max, render316(segs))
			}
			for i, s := range segs {
				if s.Bonus && (isSpecial316(segs[(i+1)%len(segs)]) || isSpecial316(segs[(i-1+len(segs))%len(segs)])) {
					t.Fatalf("%s / %s : marque voisine d'une case spéciale\n%s", cfgName316(p), name, render316(segs))
				}
			}
		}
	}
}

// Les BONUS changent de place d'un tirage à l'autre (au hasard) ; BANQUEROUTE et PERDU, jamais.
func TestRoueWheel316_BonusMoveBetweenDrawsSpecialsNever(t *testing.T) {
	p := DefaultRoueWheelParams()
	base := build316(t, p, 0, seeded316(1))
	layouts := map[string]bool{}
	for seed := int64(1); seed <= 300; seed++ {
		segs := build316(t, p, 2, seeded316(seed))
		var idx []string
		for i, s := range segs {
			if s.Kind != base[i].Kind {
				t.Fatalf("la case %d change de nature (%s → %s)", i, base[i].Kind, s.Kind)
			}
			if s.Bonus {
				idx = append(idx, fmt.Sprint(i))
			}
		}
		layouts[strings.Join(idx, ",")] = true
	}
	if len(layouts) < 10 {
		t.Fatalf("seulement %d dispositions distinctes en 300 tirages : les BONUS ne sont pas aléatoires", len(layouts))
	}
}

// Roue sans BANQUEROUTE ni PERDU (§17.2 d) : le premier BONUS est tiré sur TOUTE la roue — y compris la case maximum
// (index 0), qui n'a aucune règle propre — puis les suivants ne le touchent pas.
func TestRoueWheel316_NoSpecialCasesFirstBonusAnywhereMaxCaseAllowed(t *testing.T) {
	p := RoueWheelParams{MaxPoints: 100, Step: 10}
	hit := map[int]bool{}
	for seed := int64(1); seed <= 1500; seed++ {
		segs := build316(t, p, 1, seeded316(seed))
		for i, s := range segs {
			if s.Bonus {
				hit[i] = true
			}
		}
	}
	if len(hit) != 10 {
		t.Fatalf("le BONUS unique n'a visité que %d cases sur 10 : %v", len(hit), hit)
	}
	if !hit[0] {
		t.Fatalf("sans BANQUEROUTE, la case maximum (index 0) peut porter un BONUS (§17.2 e)")
	}
}

// Avec au moins une BANQUEROUTE la case maximum touche la première BANQUEROUTE (index 1) : jamais marquée. Aucune règle
// « jamais sur 1000 » : c'est l'adjacence seule qui l'exclut.
func TestRoueWheel316_MaxCaseNeverMarkedWhenBankruptNextToIt(t *testing.T) {
	for _, p := range configs316 {
		if p.BankruptCount == 0 {
			continue
		}
		np := norm316(t, p)
		for seed := int64(1); seed <= 100; seed++ {
			segs := build316(t, p, RoueMaxBonusFor(np), seeded316(seed))
			if segs[0].Bonus {
				t.Fatalf("%s : la case maximum (voisine de la BANQUEROUTE d'index 1) porte un BONUS", cfgName316(p))
			}
		}
	}
}

// Exemple de §17.2 sans choix : 1000 / 100 / 3 / 2 / 1 — la seule place possible est la case 400.
func TestRoueWheel316_ExampleSingleValidPlace(t *testing.T) {
	p := RoueWheelParams{MaxPoints: 1000, Step: 100, BankruptCount: 3, LoseTurnCount: 2}
	want := "1000 B P 900 100 B 800 200 P 700 300 B 600 400+"
	for seed := int64(1); seed <= 50; seed++ {
		if got := render316(build316(t, p, 1, seeded316(seed))); got != want {
			t.Fatalf("graine %d :\n got  %s\n want %s", seed, got, want)
		}
	}
}

// Exemples de §17.2 : 1000/100/2/1/2 et 100/10/0/0/3 — mêmes valeurs et spéciales, X marques, propriétés (les
// positions exactes ne sont qu'« un tirage possible »).
func TestRoueWheel316_ContractExamplesCounts(t *testing.T) {
	cases := []struct {
		p RoueWheelParams
		x int
	}{
		{RoueWheelParams{MaxPoints: 1000, Step: 100, BankruptCount: 2, LoseTurnCount: 1}, 2},
		{RoueWheelParams{MaxPoints: 100, Step: 10}, 3},
	}
	for _, c := range cases {
		for seed := int64(1); seed <= 200; seed++ {
			segs := build316(t, c.p, c.x, seeded316(seed))
			marks := 0
			plain := make([]RoueSegment, len(segs))
			copy(plain, segs)
			for i := range plain {
				if plain[i].Bonus {
					marks++
				}
				plain[i].Bonus = false
			}
			if marks != c.x {
				t.Fatalf("%s : %d marques, attendu %d", cfgName316(c.p), marks, c.x)
			}
			if got := render316(plain); got != render316(build316(t, c.p, 0, seeded316(1))) {
				t.Fatalf("%s : les valeurs ou cases spéciales changent avec les BONUS\n%s", cfgName316(c.p), got)
			}
		}
	}
}

// §17.2 b — répartition homogène : chaque BONUS va à l'intervalle NON PLEIN dont le rapport g / (b + 1) est le plus
// grand (égalité : au hasard). L'attribution observée doit donc faire partie de celles qu'un tirage des égalités peut produire.
func TestRoueWheel316_HomogeneousDistributionAcrossIntervals(t *testing.T) {
	reachable := func(lengths []int, x int) map[string]bool {
		out := map[string]bool{}
		var rec func(b []int, left int)
		rec = func(b []int, left int) {
			if left == 0 {
				out[fmt.Sprint(b)] = true
				return
			}
			bestNum, bestDen := -1, 1
			var cands []int
			for k, g := range lengths {
				if b[k] >= (g-1)/2 {
					continue // plein
				}
				num, den := g, b[k]+1
				switch {
				case bestNum < 0 || num*bestDen > bestNum*den:
					bestNum, bestDen, cands = num, den, []int{k}
				case num*bestDen == bestNum*den:
					cands = append(cands, k)
				}
			}
			for _, k := range cands {
				nb := append([]int(nil), b...)
				nb[k]++
				rec(nb, left-1)
			}
		}
		rec(make([]int, len(lengths)), x)
		return out
	}
	for _, p := range configs316 {
		if p.BankruptCount+p.LoseTurnCount == 0 {
			continue // un seul intervalle : §17.2 d
		}
		np := norm316(t, p)
		base := build316(t, p, 0, seeded316(1))
		lengths, of := intervals316(base)
		for x := 1; x <= RoueMaxBonusFor(np); x++ {
			ok := reachable(lengths, x)
			for seed := int64(1); seed <= 200; seed++ {
				segs := build316(t, p, x, seeded316(seed+int64(x)*1000))
				b := make([]int, len(lengths))
				for i, s := range segs {
					if s.Bonus {
						b[of[i]]++
					}
				}
				if !ok[fmt.Sprint(b)] {
					t.Fatalf("%s X=%d : répartition %v hors de la règle g/(b+1) (intervalles %v)\n%s", cfgName316(p), x, b, lengths, render316(segs))
				}
			}
		}
	}
}

// §17.2 c — tirage uniforme parmi les poses valables de l'intervalle : avec un seul BONUS sur la roue par défaut, il
// tombe dans le plus grand intervalle (le plus grand vide est coupé en premier), sur chacune de ses poses valables
// (ni les deux cases extrêmes) à fréquence comparable.
func TestRoueWheel316_PositionUniformInsideTheLargestInterval(t *testing.T) {
	p := DefaultRoueWheelParams()
	base := build316(t, p, 0, seeded316(1))
	lengths, of := intervals316(base)
	big, bigLen := -1, 0
	for k, g := range lengths {
		if g > bigLen {
			big, bigLen = k, g
		}
	}
	counts := map[int]int{}
	const draws = 6000
	for seed := int64(1); seed <= draws; seed++ {
		for i, s := range build316(t, p, 1, seeded316(seed)) {
			if !s.Bonus {
				continue
			}
			if of[i] != big {
				t.Fatalf("le BONUS unique est hors du plus grand intervalle (%d cases) : case %d", bigLen, i)
			}
			counts[i]++
		}
	}
	valid := bigLen - 2
	if len(counts) != valid {
		t.Fatalf("poses visitées = %d, attendu %d (toutes les poses valables, pas les extrémités) : %v", len(counts), valid, counts)
	}
	for i, c := range counts {
		if lo, hi := draws/valid*7/10, draws/valid*13/10; c < lo || c > hi {
			t.Fatalf("pose %d tirée %d fois sur %d (attendu ≈ %d) : tirage non uniforme", i, c, draws, draws/valid)
		}
	}
}

// ---- limites §17.3 ---------------------------------------------------------------------------------------------

// Table des maxima de §17.3 : min(4, place libre, ⌊(V−B−P)/2⌋).
func TestRoueWheel316_MaxBonusTable(t *testing.T) {
	cases := []struct {
		p     RoueWheelParams
		cells int
		max   int
	}{
		{DefaultRoueWheelParams(), 22, 4},
		{RoueWheelParams{MaxPoints: 1000, Step: 100, BankruptCount: 2, LoseTurnCount: 1}, 12, 2},
		{RoueWheelParams{MaxPoints: 100, Step: 20, BankruptCount: 1, LoseTurnCount: 1}, 7, 1},
		{RoueWheelParams{MaxPoints: 1000, Step: 100, BankruptCount: 3, LoseTurnCount: 2}, 14, 1},
		{RoueWheelParams{MaxPoints: 100, Step: 30, BankruptCount: 1, LoseTurnCount: 1}, 5, 0}, // pas corrigé à 25
	}
	for _, c := range cases {
		np := norm316(t, c.p)
		if got := RoueWheelSize(np); got != c.cells {
			t.Errorf("%s : RoueWheelSize = %d, attendu %d", cfgName316(c.p), got, c.cells)
		}
		if got := RoueMaxBonusFor(np); got != c.max {
			t.Errorf("%s : RoueMaxBonusFor = %d, attendu %d", cfgName316(c.p), got, c.max)
		}
		// Le nombre de BONUS ne compte pas dans la taille de la roue.
		np.BonusCount = 3
		if got := RoueWheelSize(np); got != c.cells {
			t.Errorf("%s : RoueWheelSize avec BonusCount=3 = %d, attendu %d (BONUS exclus)", cfgName316(c.p), got, c.cells)
		}
	}
}

// Un BONUS ne s'ajoute plus aux 64 cases : seul N = V + B + P est borné ; la place restante n'est plus consommée par
// les BONUS (62 cases, 4 BONUS demandés = 4 permis par place libre et espacement).
func TestRoueWheel316_BonusDoNotCountInto64Cases(t *testing.T) {
	p := RoueWheelParams{MaxPoints: 64, Step: 1} // 64 cases : plein, des BONUS restent possibles (v12.1 : 0)
	np := norm316(t, p)
	if got := RoueMaxBonusFor(np); got != RoueMaxBonusCount {
		t.Fatalf("64 cases : RoueMaxBonusFor = %d, attendu %d (les BONUS ne comptent plus dans les 64 cases)", got, RoueMaxBonusCount)
	}
	segs := build316(t, p, 4, seeded316(3))
	if len(segs) != 64 {
		t.Fatalf("la roue a %d cases, attendu 64", len(segs))
	}
}

// ValidateRoueBonusCount : 0–4 et au plus RoueMaxBonusFor ; code ROUE_BONUS_COUNT_INVALID.
func TestRoueWheel316_ValidateBonusCount(t *testing.T) {
	code := func(err error) string {
		if err == nil {
			return ""
		}
		if re, ok := err.(*RoueError); ok {
			return re.Code
		}
		return "ERR:" + err.Error()
	}
	p := RoueWheelParams{MaxPoints: 1000, Step: 100, BankruptCount: 2, LoseTurnCount: 1} // max 2
	for n, want := range map[int]string{0: "", 1: "", 2: "", 3: RoueErrBonusCountInvalid, 4: RoueErrBonusCountInvalid, 5: RoueErrBonusCountInvalid, -1: RoueErrBonusCountInvalid} {
		if got := code(ValidateRoueBonusCount(n, p)); got != want {
			t.Errorf("1000/100/2/1 n=%d : code %q, attendu %q", n, got, want)
		}
	}
	zero := RoueWheelParams{MaxPoints: 100, Step: 30, BankruptCount: 1, LoseTurnCount: 1} // max 0
	if got := code(ValidateRoueBonusCount(1, zero)); got != RoueErrBonusCountInvalid {
		t.Errorf("100/30/1/1 n=1 : code %q, attendu %s", got, RoueErrBonusCountInvalid)
	}
	if got := code(ValidateRoueBonusCount(0, zero)); got != "" {
		t.Errorf("100/30/1/1 n=0 : refusé (%q) — une roue sans BONUS valable en v12.1 le reste", got)
	}
}

// Roue sans BONUS valable en v12.1 : reste valable (la limite de 64 cases n'a pas bougé pour elle).
func TestRoueWheel316_WheelWithoutBonusValidInV121StaysValid(t *testing.T) {
	for _, p := range configs316 {
		if _, _, err := NormalizeRoueWheelParams(p); err != nil {
			t.Errorf("%s valable en v12.1 et refusée : %v", cfgName316(p), err)
		}
	}
	if _, _, err := NormalizeRoueWheelParams(RoueWheelParams{MaxPoints: 65, Step: 1}); err == nil {
		t.Errorf("65 cases : la limite de 64 cases doit rester appliquée")
	}
}

// ---- RoueEffectiveParams : réduction, repli (§17.3, START) ---------------------------------------------------------

// Au départ, un nombre de BONUS au-dessus du maximum est RÉDUIT à ce maximum (jamais de refus) ; la roue valide sans
// BONUS reste inchangée ; roue invalide → roue globale.
func TestRoueWheel316_EffectiveParamsReduceBonusToMax(t *testing.T) {
	four, mp, step, b2, p1 := 4, 1000, 100, 2, 1
	tc := TypedContent{RoueBonusCount: &four, RoueWheelMaxPoints: &mp, RoueWheelStep: &step,
		RoueWheelBankruptCount: &b2, RoueWheelLoseTurnCount: &p1}
	got, fallback := RoueEffectiveParams(tc, DefaultRoueWheelParams()) // 1000/100/2/1 : 4 demandés, 2 permis
	if fallback {
		t.Fatalf("la composition est valide sans BONUS : pas de repli")
	}
	if got.BonusCount != 2 {
		t.Fatalf("BONUS demandés 4, permis 2 : BonusCount = %d", got.BonusCount)
	}
	if RoueWheelSize(got) != 12 {
		t.Fatalf("la roue doit rester à 12 cases, got %d", RoueWheelSize(got))
	}
}

// Roue invalide même sans BONUS (ici 1000 cases) : repli sur la roue globale.
func TestRoueWheel316_EffectiveParamsInvalidWheelFallsBackToGlobal(t *testing.T) {
	one, mp, step := 1, 1000, 1
	tc := TypedContent{RoueBonusCount: &one, RoueWheelMaxPoints: &mp, RoueWheelStep: &step}
	got, fallback := RoueEffectiveParams(tc, DefaultRoueWheelParams())
	if !fallback {
		t.Fatalf("roue invalide : repli sur la roue globale attendu")
	}
	if got.MaxPoints != 1000 || got.Step != 50 {
		t.Fatalf("la roue globale doit être rendue, got %+v", got)
	}
}
