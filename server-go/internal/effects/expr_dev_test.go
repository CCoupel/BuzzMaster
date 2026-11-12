package effects

import (
	"fmt"
	"strings"
	"testing"
)

// B2.0 (#210) — dev tests of the variable positions (VarPositions, one value per tick) and of the
// expression analysis. The chronograms of §12.5 are covered by the spec tests.

func devVarSeq(t *testing.T, decls string, n, ticks int, names ...string) map[string][]int {
	t.Helper()
	e := mustParse(t, fx("variables:\n"+decls+"pas:\n  - { g0: [theme, 100], gA: [urgence, 100] }\n"))
	nums := make([]int, n)
	for i := range nums {
		nums[i] = i + 1
	}
	out := map[string][]int{}
	for k := 0; k < ticks; k++ {
		vp := e.VarPositions(nums, k)
		for _, nm := range names {
			out[nm] = append(out[nm], vp[nm])
		}
	}
	return out
}

func TestVarPositions_ExampleA_Dev(t *testing.T) {
	t.Parallel()
	decls := "  gB: gA+1\n  gC: N; -N/3\n  gA: N/2; +1\n"
	for _, c := range []struct {
		n        int
		a, b, cc []int
	}{
		{5, []int{2, 3, 4, 5, 1, 2}, []int{3, 4, 5, 1, 2, 3}, []int{5, 4, 3, 2, 1, 5}},
		{6, []int{3, 4, 5, 6, 1, 2}, []int{4, 5, 6, 1, 2, 3}, []int{6, 4, 2, 6, 4, 2}},
		{2, []int{1, 2, 1, 2}, []int{2, 1, 2, 1}, []int{2, 2, 2, 2}},
		{1, []int{1, 1, 1}, []int{1, 1, 1}, []int{1, 1, 1}},
	} {
		got := devVarSeq(t, decls, c.n, len(c.a), "gA", "gB", "gC")
		for name, want := range map[string][]int{"gA": c.a, "gB": c.b, "gC": c.cc} {
			if fmt.Sprint(got[name][:len(want)]) != fmt.Sprint(want) {
				t.Errorf("N=%d %s : %v, attendu %v", c.n, name, got[name], want)
			}
		}
	}
}

func TestVarPositions_ExampleB_PartirVsSuivre_Dev(t *testing.T) {
	t.Parallel()
	// gD avec pas : part du départ de gA (g1) + 1, avance de 2 ; sans pas : suit gA.
	got := devVarSeq(t, "  gA: 1; +1\n  gD: gA+1; +2\n", 4, 5, "gD")
	if fmt.Sprint(got["gD"]) != "[2 4 2 4 2]" {
		t.Errorf("avec pas : %v", got["gD"])
	}
	got = devVarSeq(t, "  gA: 1; +1\n  gD: gA+1\n", 4, 6, "gD")
	if fmt.Sprint(got["gD"]) != "[2 3 4 1 2 3]" {
		t.Errorf("sans pas : %v", got["gD"])
	}
}

func TestVarPositions_NoValue_Dev(t *testing.T) {
	t.Parallel()
	// division par zéro à l'exécution : variable sans valeur (0), effet non interrompu
	got := devVarSeq(t, "  gA: 6/(N-3)\n", 3, 2, "gA")
	if fmt.Sprint(got["gA"]) != "[0 0]" {
		t.Errorf("N=3 : %v", got["gA"])
	}
	got = devVarSeq(t, "  gA: 6/(N-3)\n", 4, 1, "gA")
	if got["gA"][0] != 2 { // 6/1 = 6 → rang ((6-1) mod 4)+1 = 2
		t.Errorf("N=4 : %v", got["gA"])
	}
}

func TestExprParse_Errors_Dev(t *testing.T) {
	t.Parallel()
	for src, want := range map[string]string{
		"gA+":                    "incomplète",
		"N//2":                   "mal écrite",
		"(1+2":                   "parenthèse non fermée",
		"1+2)":                   "parenthèse fermante",
		"N/0":                    "division par zéro dans « N/0 »",
		"3/(2-2)":                "division par zéro",
		"1000":                   "trop grand",
		"2N":                     "mal écrite",
		"g*+":                    "incomplète",
		"":                       "attendue",
		strings.Repeat("1+", 40): "dépasse",
	} {
		_, err := ParseExpr(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q : %v, attendu %q", src, err, want)
		}
	}
	for _, src := range []string{"1", "N/2", " -N/3 ", "g*", "g3", "(N+1)*2", "gA+gB", "+1", "g* - 1"} {
		if _, err := ParseExpr(src); err != nil {
			t.Errorf("%q refusé : %v", src, err)
		}
	}
}

func TestValidate_GNAndLoops_Dev(t *testing.T) {
	t.Parallel()
	bad := map[string]string{
		"gN cible":         "pas:\n  - { gN: [theme, 100] }\n",
		"gN départ":        "variables:\n  gA: gN; -1\npas:\n  - { gA: [theme, 100] }\n",
		"boucle directe":   "variables:\n  gA: gB+1\n  gB: gA+1\npas:\n  - { gA: [theme, 100] }\n",
		"boucle sur soi":   "variables:\n  gA: gA+1\npas:\n  - { gA: [theme, 100] }\n",
		"boucle à trois":   "variables:\n  gA: gC\n  gB: gA\n  gC: gB\npas:\n  - { gA: [theme, 100] }\n",
		"pas cite var":     "variables:\n  gA: 1; +1\n  gB: 1; gA\npas:\n  - { gA: [theme, 100] }\n",
		"g*+1 pas cible":   "pas:\n  - { g*+1: [theme, 100] }\n",
		"var non déclarée": "variables:\n  gA: gB+1\npas:\n  - { gA: [theme, 100] }\n",
	}
	for name, body := range bad {
		if _, errs := ParseFile("f.yaml", []byte(fx(body))); len(errs) == 0 || errs[0].Line == 0 {
			t.Errorf("%s : refus avec ligne attendu, obtenu %+v", name, errs)
		}
	}
	_, errs := ParseFile("f.yaml", []byte(fx(bad["gN cible"])))
	if !strings.Contains(errs[0].Message, "pour le dernier groupe, écrivez g*") {
		t.Errorf("message gN : %q", errs[0].Message)
	}
	// déclarée, gN est une variable ordinaire (et gN+1 un voisin)
	if _, errs := ParseFile("f.yaml", []byte(fx("variables:\n  gN: g*; -1\npas:\n  - { gN: [theme, 100], gN+1: [theme, 50] }\n"))); len(errs) != 0 {
		t.Errorf("gN déclarée : %+v", errs)
	}
}

func TestExprOverflowBounded_Dev(t *testing.T) {
	t.Parallel()
	if _, err := ParseExpr("999*999*999"); err == nil || !strings.Contains(err.Error(), "dépasse la valeur permise") {
		t.Errorf("constante trop grande refusée : %v", err)
	}
	if _, err := ParseExpr("999*999"); err != nil {
		t.Errorf("999*999 (< 1 000 000) accepté : %v", err)
	}
	// à l'exécution : N * 999 * 999 dépasse → variable sans valeur, jamais un débordement
	e := mustParse(t, fx("variables:\n  gA: N*999*999\npas:\n  - { g0: [theme, 100], gA: [urgence, 100] }\n"))
	if vp := e.VarPositions([]int{1, 2, 3}, 0); vp["gA"] != 0 {
		t.Errorf("valeur > 1 000 000 : sans valeur, obtenu %v", vp)
	}
	// départ + pas × cran très grand : même rang qu'en arbitraire précision
	e = mustParse(t, fx("variables:\n  gA: 1; 999*999\npas:\n  - { gA: [urgence, 100] }\n"))
	nums := []int{1, 2, 3, 4, 5}
	k := 2000000000
	want := int((int64(1)+999999*int64(k)-1)%5) + 1
	r := newResolver(&e, nums)
	r.setCran(k)
	if rk, ok := r.rank(0, k); !ok || rk != want {
		t.Errorf("rang au cran %d : %d (%v), attendu %d", k, rk, ok, want)
	}
}
