package effects

// #210 (v11.8.0, LOT-2, B2.0) — format révision 7 : `g*`, noms de variables avec N, expressions du départ et du pas, suivre / partir,
// division entière tronquée, rebouclage, 24 variables / 40 actions, refus de `gN` non déclaré, messages §12.3.
// Source : contracts/light-effects.md §12.1 à §12.6 (chronogrammes §12.5 repris tels quels). TDD : rouge tant que B2.0 n'est pas livré.
// Seuls Parse et Render (déjà là, voir api_spec_test.go) sont utilisés : aucune surface supposée dans ce fichier.
// Noms TestTW210_*. `critical` : TestTW210_Critical_*.

import (
	"fmt"
	"strings"
	"testing"
)

func tw210Tok(l Lk) string {
	if l.Absent {
		return "_"
	}
	if l.Intensity == 0 {
		return "."
	}
	switch {
	case l.Color == cT && near(l.Intensity, 255, 1):
		return "T"
	case l.Color == cT && near(l.Intensity, 128, 1):
		return "t"
	case l.Color == cT && near(l.Intensity, 102, 1):
		return "d"
	case l.Color == [3]int{20, 184, 166} && near(l.Intensity, 255, 1):
		return "Q"
	case l.Color == cW && near(l.Intensity, 255, 1):
		return "U"
	case l.Color == cScn && near(l.Intensity, 255, 1):
		return "S"
	case l.Color == cR && near(l.Intensity, 255, 1):
		return "x"
	}
	return "?"
}

func tw210Rows(t *testing.T, src string, n, ticks int) []string {
	t.Helper()
	e := mustParse(t, src)
	out := make([]string, ticks)
	for k := 0; k < ticks; k++ {
		v := renderView(e, ctxN(n), k)
		var b strings.Builder
		for p := 1; p <= n; p++ {
			b.WriteString(tw210Tok(v.effective(p)))
		}
		out[k] = b.String()
	}
	return out
}

// tw210Paint : n groupes au thème, puis chaque (rang, jeton) appliqué dans l'ordre (le dernier gagne).
func tw210Paint(n int, base byte, writes ...[2]any) string {
	r := []byte(strings.Repeat(string(base), n))
	for _, w := range writes {
		r[w[0].(int)-1] = w[1].(byte)
	}
	return string(r)
}

// ---------------------------------------------------------------------------
// §12.5 Exemple A — départ au milieu, variable qui suit, pas relatif
// ---------------------------------------------------------------------------

const tw210ExampleA = `id: milieu-et-suiveur
nom: Milieu et suiveur
version: 2
moments: [chrono]
duree_tick_ms: 1000
variables:
  gB: gA+1            # sans pas : toujours le groupe juste après gA
  gC: N; -N/3         # part du dernier groupe, recule d'un tiers de zone par pas
  gA: N/2; +1         # part du milieu (division entière), avance d'un groupe par pas
pas:
  - { g0: [theme, 100], gB: [theme, 50], gC: ["#14B8A6", 100], gA: [urgence, 100] }
`

func TestTW210_ChronogramA_TickByTick_5_6_2_1Groups(t *testing.T) {
	cases := []struct {
		n          int
		gA, gB, gC []int
	}{
		{5, []int{2, 3, 4, 5, 1, 2}, []int{3, 4, 5, 1, 2, 3}, []int{5, 4, 3, 2, 1, 5}},
		{6, []int{3, 4, 5, 6, 1, 2}, []int{4, 5, 6, 1, 2, 3}, []int{6, 4, 2, 6, 4, 2}},
		{2, []int{1, 2, 1, 2}, []int{2, 1, 2, 1}, []int{2, 2, 2, 2}},
		{1, []int{1, 1, 1}, []int{1, 1, 1}, []int{1, 1, 1}},
	}
	for _, c := range cases {
		var want []string
		for k := range c.gA {
			// ordre d'écriture = ordre de déclaration : gB, gC, gA (la dernière gagne)
			want = append(want, tw210Paint(c.n, 'T', [2]any{c.gB[k], byte('t')}, [2]any{c.gC[k], byte('Q')}, [2]any{c.gA[k], byte('U')}))
		}
		eqRows(t, fmt.Sprintf("exemple A, %d groupes", c.n), tw210Rows(t, tw210ExampleA, c.n, len(want)), want)
	}
}

func TestTW210_ChronogramA_5Groups_DetailTable(t *testing.T) {
	got := tw210Rows(t, tw210ExampleA, 5, 5)
	want := []string{
		"TUtTQ", // tick 0 : g2 urgence, g3 50 %, g5 turquoise
		"TTUQT", // tick 1 : gB et gC écrivent g4 → gC gagne ; g3 urgence
		"TTQUt", // tick 2 : g5 à 50 % ; g3 turquoise ; g4 urgence
		"tQTTU", // tick 3 : g1 à 50 % ; g2 turquoise ; g5 urgence
		"UtTTT", // tick 4 : g1 : gC puis gA → gA gagne ; g2 à 50 %
	}
	eqRows(t, "exemple A détaillé", got, want)
}

// ---------------------------------------------------------------------------
// §12.5 Exemple B — partir d'une autre variable ; gN variable ; g* cible
// ---------------------------------------------------------------------------

const tw210ExampleB = `id: croisement
nom: Croisement
version: 2
moments: [chrono]
groupes_min: 3
duree_tick_ms: 1000
variables:
  gA: 1; +1           # part du premier groupe, avance
  gN: g*; -1          # « gN » est ici une variable comme une autre
  gD: gA+1; +2        # AVEC pas : part du groupe après le départ de gA (g2), puis avance de 2
pre:
  { g0: [theme, 100], g*: [scene, 100] }
pas:
  - { g0: [theme, 100], gA: [urgence, 100], gN: [urgence, 100], gD: [theme, 40] }
`

func TestTW210_ChronogramB_4Groups_WithStep(t *testing.T) {
	gA := []int{1, 1, 2, 3, 4, 1}
	gN := []int{4, 4, 3, 2, 1, 4}
	gD := []int{2, 2, 4, 2, 4, 2}
	want := []string{tw210Paint(4, 'T', [2]any{4, byte('S')})} // tick 0 : PRE
	for k := 1; k <= 5; k++ {
		want = append(want, tw210Paint(4, 'T', [2]any{gA[k], byte('U')}, [2]any{gN[k], byte('U')}, [2]any{gD[k], byte('d')}))
	}
	eqRows(t, "exemple B", tw210Rows(t, tw210ExampleB, 4, 6), want)
}

func TestTW210_ChronogramB_WithoutStep_AlwaysGroupAfterGA(t *testing.T) {
	src := strings.Replace(tw210ExampleB, "gD: gA+1; +2", "gD: gA+1", 1)
	gA := []int{0, 1, 2, 3, 4, 1}
	gN := []int{0, 4, 3, 2, 1, 4}
	gD := []int{0, 2, 3, 4, 1, 2} // « g2, g2, g3, g4, g1, g2 » dont le tick 0 (PRE) ne joue pas gD
	want := []string{tw210Paint(4, 'T', [2]any{4, byte('S')})}
	for k := 1; k <= 5; k++ {
		want = append(want, tw210Paint(4, 'T', [2]any{gA[k], byte('U')}, [2]any{gN[k], byte('U')}, [2]any{gD[k], byte('d')}))
	}
	eqRows(t, "exemple B sans pas", tw210Rows(t, src, 4, 6), want)
}

// ---------------------------------------------------------------------------
// §12.2 expressions : rang du groupe touché à chaque tick (0 = aucun)
// ---------------------------------------------------------------------------

func tw210Ranks(t *testing.T, vars string, n, ticks int) []int {
	t.Helper()
	src := fx("variables:\n" + vars + "pas:\n  - { g0: [theme, 100], gA: [\"#FF0000\", 100] }\n")
	e := mustParse(t, src)
	out := make([]int, ticks)
	for k := 0; k < ticks; k++ {
		v := renderView(e, ctxN(n), k)
		for p := 1; p <= n; p++ {
			if tw210Tok(v.effective(p)) == "x" {
				out[k] = p
				break
			}
		}
	}
	return out
}

func TestTW210_Expressions_RankAtEachTick(t *testing.T) {
	cases := []struct {
		name, decl string
		n          int
		want       []int
	}{
		{"priorité * avant +", "1+2*3", 5, []int{2, 2}},  // 7 → ((7-1) mod 5)+1
		{"parenthèses", "(1+2)*3", 5, []int{4, 4}},       // 9
		{"division tronquée 5/2", "5/2", 5, []int{2, 2}}, // 2
		{"division tronquée négative -5/3 = -1", "-5/3", 5, []int{4, 4}},
		{"valeur nulle → dernier groupe", "0", 4, []int{4, 4}},
		{"valeur négative -1 → N-1", "-1", 4, []int{3, 3}},
		{"N+1 reboucle sur 1", "N+1", 4, []int{1, 1}},
		{"g3 = 3", "g3", 5, []int{3, 3}},
		{"g* = N", "g*", 4, []int{4, 4}},
		{"g* - 1", "g* - 1", 4, []int{3, 3}},
		{"signe en tête +1", "+1", 4, []int{1, 1}},
		{"espaces libres", "  N / 2 ", 6, []int{3, 3}},
		{"N/2 avec N=1 → groupe unique", "N/2", 1, []int{1, 1}},
		{"N/3 avec N=2 = 0 → dernier groupe", "N/3", 2, []int{2, 2}},
		{"pas nul n'avance pas", "1; 0", 4, []int{1, 1, 1}},
		{"pas N/3 = 0 avec N=2 : reste sur g1", "1; N/3", 2, []int{1, 1, 1}},
		{"pas relatif -N/3", "N; -N/3", 6, []int{6, 4, 2, 6}},
		{"pas au-delà de ±12 (13)", "1; 13", 4, []int{1, 2, 3, 4, 1}},
		{"écriture v11.7.0 g1; +1", "g1; +1", 3, []int{1, 2, 3, 1}},
		{"écriture v11.7.0 g3; 0", "g3; 0", 5, []int{3, 3}},
		{"ancien gN; -1 devient g*; -1", "g*; -1", 4, []int{4, 3, 2, 1, 4}},
	}
	for _, c := range cases {
		got := tw210Ranks(t, "  gA: "+c.decl+"\n", c.n, len(c.want))
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s (gA: %s, N=%d) : %v, attendu %v", c.name, c.decl, c.n, got, c.want)
		}
	}
}

func TestTW210_Expressions_NZeroGroups_ActionsIgnored(t *testing.T) {
	e := mustParse(t, fx("variables:\n  gA: N/2; +1\npas:\n  - { gA: [\"#FF0000\", 100], g*: [\"#00FF00\", 100] }\n"))
	v := renderView(e, ctxN(0), 0)
	if len(v.Groups) != 0 {
		t.Errorf("N=0 : aucune action visible, %v", v.Groups)
	}
}

func TestTW210_Expressions_RuntimeDivisionByZero_NoValue_NoPanic(t *testing.T) {
	src := fx("variables:\n  gA: 6/(N-3)\n  gB: 1; +1\npas:\n  - { gA: [\"#FF0000\", 100], gB: [\"#00FF00\", 100] }\n")
	e := mustParse(t, src) // valide : ce n'est pas une division par zéro écrite en dur
	v := renderView(e, ctxN(3), 0)
	for p := 1; p <= 3; p++ {
		if tw210Tok(v.effective(p)) == "x" {
			t.Errorf("une variable sans valeur ne touche aucun groupe (g%d rouge)", p)
		}
	}
	if tok(v.effective(1)) != "G" {
		t.Errorf("l'effet n'est pas interrompu : gB reste appliquée, %v", v)
	}
}

// --- suivre / partir ----------------------------------------------------------

func TestTW210_Follow_ReevaluatedEachTick_StartFrozen(t *testing.T) {
	src := func(declB string) string {
		return fx("variables:\n  gA: 1; +1\n  gB: " + declB + "\npas:\n  - { g0: [theme, 100], gB: [\"#FF0000\", 100] }\n")
	}
	rank := func(declB string) []int {
		e := mustParse(t, src(declB))
		var out []int
		for k := 0; k < 5; k++ {
			v := renderView(e, ctxN(4), k)
			for p := 1; p <= 4; p++ {
				if tw210Tok(v.effective(p)) == "x" {
					out = append(out, p)
				}
			}
		}
		return out
	}
	if g := rank("gA+1"); fmt.Sprint(g) != "[2 3 4 1 2]" {
		t.Errorf("gB: gA+1 (suit) : %v", g)
	}
	if g := rank("gA+1; +2"); fmt.Sprint(g) != "[2 4 2 4 2]" {
		t.Errorf("gB: gA+1; +2 (part du départ de gA, avance de 2) : %v", g)
	}
}

func TestTW210_Follow_ChainOfThree_DeclaredBeforeAndAfter(t *testing.T) {
	for _, vars := range []string{
		"  gA: 1; +1\n  gB: gA+1\n  gC: gB+1\n",
		"  gC: gB+1\n  gB: gA+1\n  gA: 1; +1\n",
	} {
		src := fx("variables:\n" + vars + "pas:\n  - { g0: [theme, 100], gC: [\"#FF0000\", 100] }\n")
		e := mustParse(t, src)
		var got []int
		for k := 0; k < 4; k++ {
			v := renderView(e, ctxN(5), k)
			for p := 1; p <= 5; p++ {
				if tw210Tok(v.effective(p)) == "x" {
					got = append(got, p)
				}
			}
		}
		if fmt.Sprint(got) != "[3 4 5 1]" {
			t.Errorf("chaîne de trois (%q) : %v", strings.TrimSpace(vars), got)
		}
	}
}

func TestTW210_Follow_VariableCitingAList(t *testing.T) {
	src := fx("variables:\n  gC: [g1, g4, g3]\n  gD: gC+1\npas:\n  - { g0: [theme, 100], gD: [\"#FF0000\", 100] }\n")
	e := mustParse(t, src)
	var got []int
	for k := 0; k < 3; k++ {
		v := renderView(e, ctxN(5), k)
		for p := 1; p <= 5; p++ {
			if tw210Tok(v.effective(p)) == "x" {
				got = append(got, p)
			}
		}
	}
	if fmt.Sprint(got) != "[2 5 4]" {
		t.Errorf("gD: gC+1 avec gC = g1, g4, g3 : %v", got)
	}
}

// --- gN : refus et variable ordinaire -----------------------------------------

func TestTW210_Critical_GN_Undeclared_RefusedWithMessage(t *testing.T) {
	for name, body := range map[string]string{
		"cible":  "pas:\n  - { gN: [theme, 100] }\n",
		"départ": "variables:\n  gD: gN; -1\npas:\n  - { gD: [theme, 100] }\n",
	} {
		_, errs := parseEffect(fx(body))
		if len(errs) == 0 {
			t.Errorf("gN non déclaré (%s) accepté", name)
			continue
		}
		m := errs[0].Message
		if !strings.Contains(m, "« gN » n'est pas déclarée dans variables") || !strings.Contains(m, "g*") {
			t.Errorf("%s : message %q", name, m)
		}
		if errs[0].Line == 0 {
			t.Errorf("%s : ligne attendue", name)
		}
	}
}

func TestTW210_GN_DeclaredIsOrdinaryVariable_NeighbourWorks(t *testing.T) {
	src := fx("variables:\n  gN: 2; +1\npas:\n  - { gN: [\"#FF0000\", 100], gN+1: [\"#00FF00\", 100] }\n")
	e := mustParse(t, src)
	v := renderView(e, ctxN(4), 0)
	if tw210Tok(v.effective(2)) != "x" || tok(v.effective(3)) != "G" {
		t.Errorf("gN variable (g2), voisin gN+1 (g3) : %v", v.Groups)
	}
}

func TestTW210_Critical_StarTarget_IsLastNonEmptyGroup(t *testing.T) {
	for n := 1; n <= 12; n++ {
		e := mustParse(t, fx("pas:\n  - { g*: [\"#FF0000\", 100] }\n"))
		v := renderView(e, ctxN(n), 0)
		for p := 1; p <= n; p++ {
			if got, want := tw210Tok(v.effective(p)) == "x", p == n; got != want {
				t.Errorf("N=%d : g%d rouge = %v", n, p, got)
			}
		}
	}
}

func TestTW210_StarTarget_NotAnExpressionTarget(t *testing.T) {
	if _, errs := parseEffect(fx("pas:\n  - { g*+1: [theme, 100] }\n")); len(errs) == 0 {
		t.Error("g*+1 n'est pas une cible")
	}
}

// ---------------------------------------------------------------------------
// §12.3 messages de validation (ligne comprise)
// ---------------------------------------------------------------------------

func TestTW210_ValidationMessages(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"variable non déclarée (cible)", "pas:\n  - { gB: [theme, 100] }\n", "la variable « gB » n'est pas déclarée dans variables"},
		{"variable non déclarée (expression)", "variables:\n  gA: gB+1\npas:\n  - { gA: [theme, 100] }\n", "la variable « gB » n'est pas déclarée dans variables"},
		{"expression incomplète", "variables:\n  gA: 1+\npas:\n  - { gA: [theme, 100] }\n", "l'expression « 1+ » est incomplète"},
		{"expression mal écrite", "variables:\n  gA: N//2\npas:\n  - { gA: [theme, 100] }\n", "l'expression « N//2 » est mal écrite"},
		{"parenthèse non fermée", "variables:\n  gA: (N+1\npas:\n  - { gA: [theme, 100] }\n", "parenthèse non fermée"},
		{"division par zéro en dur", "variables:\n  gA: N/0\npas:\n  - { gA: [theme, 100] }\n", "division par zéro dans « N/0 »"},
		{"boucle indirecte", "variables:\n  gA: gB+1\n  gB: gA+1\npas:\n  - { gA: [theme, 100] }\n", "les variables gA et gB dépendent l'une de l'autre"},
		{"boucle sur soi-même", "variables:\n  gA: gA+1\npas:\n  - { gA: [theme, 100] }\n", "gA"},
		{"le pas cite une variable", "variables:\n  gA: 1; gB\n  gB: 2\npas:\n  - { gA: [theme, 100] }\n", "le pas ne peut pas citer une variable (gB)"},
		{"cible mal écrite", "pas:\n  - { foo: [theme, 100] }\n", "g*"},
		{"expression trop longue", "variables:\n  gA: " + strings.Repeat("1+", 40) + "1\npas:\n  - { gA: [theme, 100] }\n", ""},
		{"entier au-delà de 999", "variables:\n  gA: 1000\npas:\n  - { gA: [theme, 100] }\n", ""},
	}
	for _, c := range cases {
		_, errs := parseEffect(fx(c.body))
		if len(errs) == 0 {
			t.Errorf("%s : accepté", c.name)
			continue
		}
		found := false
		for _, e := range errs {
			if strings.Contains(e.Message, c.want) { // want vide : refus seul exigé (message non fixé par le §12.3)
				found = true
				if e.Line == 0 {
					t.Errorf("%s : ligne absente (%+v)", c.name, e)
				}
			}
		}
		if !found {
			t.Errorf("%s : message attendu « %s », obtenu %+v", c.name, c.want, errs)
		}
	}
}

func TestTW210_ValidationMessages_PointToTheVariableLine(t *testing.T) {
	// fx() occupe les lignes 1 à 5 ; « variables: » = 6 ; la première variable = 7
	_, errs := parseEffect(fx("variables:\n  gA: N//2\npas:\n  - { gA: [theme, 100] }\n"))
	if len(errs) == 0 || errs[0].Line != 7 {
		t.Errorf("la ligne 7 est celle de la déclaration : %+v", errs)
	}
}

// ---------------------------------------------------------------------------
// Limites : 24 variables, 40 actions
// ---------------------------------------------------------------------------

func tw210VarNames(n int) []string {
	var out []string
	for c := 'A'; c <= 'Z' && len(out) < n; c++ {
		out = append(out, "g"+string(c))
	}
	return out
}

func tw210ManyVars(n int) string {
	var b strings.Builder
	b.WriteString("variables:\n")
	for _, v := range tw210VarNames(n) {
		fmt.Fprintf(&b, "  %s: 1; +1\n", v)
	}
	b.WriteString("pas:\n  - { g0: [theme, 100] }\n")
	return b.String()
}

func TestTW210_Limits_24VariablesAccepted_25Refused(t *testing.T) {
	if _, errs := parseEffect(fx(tw210ManyVars(24))); len(errs) > 0 {
		t.Errorf("24 variables acceptées : %+v", errs)
	}
	_, errs := parseEffect(fx(tw210ManyVars(25)))
	if len(errs) == 0 || !strings.Contains(errs[0].Message, "24 variables au plus") {
		t.Errorf("25 variables refusées avec « 24 variables au plus » : %+v", errs)
	}
}

func tw210Actions(n int) string {
	var targets []string
	targets = append(targets, "g0")
	for i := 1; i <= 12; i++ {
		targets = append(targets, fmt.Sprintf("g%d", i))
	}
	targets = append(targets, "g*")
	targets = append(targets, tw210VarNames(24)...)
	targets = append(targets, "gA+1", "gA-1", "gA+2", "gA-2")
	var b strings.Builder
	b.WriteString("variables:\n")
	for _, v := range tw210VarNames(24) {
		fmt.Fprintf(&b, "  %s: 1; +1\n", v)
	}
	b.WriteString("pas:\n  - {")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, " %s: [theme, 50]", targets[i])
	}
	b.WriteString(" }\n")
	return b.String()
}

func TestTW210_Limits_40ActionsAccepted_41Refused(t *testing.T) {
	if _, errs := parseEffect(fx(tw210Actions(40))); len(errs) > 0 {
		t.Errorf("40 actions acceptées : %+v", errs)
	}
	_, errs := parseEffect(fx(tw210Actions(41)))
	if len(errs) == 0 || !strings.Contains(errs[0].Message, "de 1 à 40 actions") {
		t.Errorf("41 actions refusées (« un pas compte de 1 à 40 actions ») : %+v", errs)
	}
}

// ---------------------------------------------------------------------------
// Coût du rendu (§12.6) : banc de mesure, valeur à consigner dans le rapport de QA
// ---------------------------------------------------------------------------

func BenchmarkTW210_Render_24Vars_40Actions_Tick10000(b *testing.B) {
	var vars strings.Builder
	vars.WriteString("variables:\n  gA: 1; +1\n")
	names := tw210VarNames(24)
	for i := 1; i < 24; i++ {
		fmt.Fprintf(&vars, "  %s: %s+1\n", names[i], names[i-1]) // chaîne de dépendances
	}
	vars.WriteString("pas:\n  - {")
	for i, v := range names {
		if i > 0 {
			vars.WriteString(",")
		}
		fmt.Fprintf(&vars, " %s: [theme, 50]", v)
	}
	vars.WriteString(" }\n")
	e, errs := parseEffect(fx(vars.String()))
	if len(errs) > 0 {
		b.Fatalf("%+v", errs)
	}
	ctx := ctxN(12).toContext(e)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Render(e, ctx, 10000)
	}
}
