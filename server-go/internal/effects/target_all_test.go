package effects

// Règle `target: all` (planner, contrat light-effects.md §4.6, §2.5, §7.1 — amendement 632b84ec) :
// toujours applicable en salle même à N = 0 (un état sur la zone générale entière) ;
// min_groups ne concerne que target: groups ; motif uniform seul ; groups_lt/gte refusés ;
// groupe animateur : target all toujours applicable, target groups si min_groups ≤ 1.

import (
	"strings"
	"testing"
)

const allBase = `
id: tout
name: Tout
version: 1
moments: [chrono]
target: all
looks: { a: { color: scene } }
`

func TestTargetAll_AlwaysApplicable_EvenWithoutGroups(t *testing.T) {
	e := mustParse(t, allBase+"pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }\n")
	for _, n := range []int{0, 1, 5, 12} {
		if ok, reason := applicable(e, n); !ok {
			t.Errorf("target all, N=%d : applicable attendu (%q)", n, reason)
		}
	}
}

func TestTargetAll_RendersSingleStateOnWholeZone_EvenAtN0(t *testing.T) {
	e := mustParse(t, allBase+"pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }\n")
	for _, n := range []int{0, 1, 6, 12} {
		if got := len(render(e, Ctx{N: n, Scene: white}, 0)); got != 1 {
			t.Errorf("target all, N=%d : un seul état attendu, obtenu %d", n, got)
		}
	}
}

func TestTargetGroups_NotApplicableBelowMinGroups_AndAtZero(t *testing.T) {
	e := mustParse(t, strings.Replace(allBase, "target: all", "target: groups", 1)+
		"pattern: { type: shift, step_ms: 1000, cells: [a], rest: a }\n")
	if ok, _ := applicable(e, 0); ok {
		t.Error("target groups, N=0 : inapplicable")
	}
	if ok, _ := applicable(e, 1); !ok {
		t.Error("target groups, min_groups par défaut 1 : applicable dès N=1")
	}
}

func TestTargetAll_ValidationErrors(t *testing.T) {
	uniform := "pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }\n"
	cases := map[string]string{
		"min_groups > 1":       allBase + "min_groups: 3\n" + uniform,
		"motif shift":          allBase + "pattern: { type: shift, step_ms: 1000, cells: [a], rest: a }\n",
		"motif gauge":          allBase + "pattern: { type: gauge, source: remaining, on: a, off: a }\n",
		"motif random":         allBase + "pattern: { type: random, step_ms: 1000, count: 1, on: a, off: a }\n",
		"motif gradient":       allBase + "pattern: { type: gradient, stops: [a, a], travel: none }\n",
		"condition groups_lt":  allBase + uniform + "stages:\n  - when: { groups_lt: 5 }\n    pattern: { phases: [ { look: a, ms: 1000 } ] }\n",
		"condition groups_gte": allBase + uniform + "stages:\n  - when: { groups_gte: 5 }\n    pattern: { phases: [ { look: a, ms: 1000 } ] }\n",
	}
	for name, src := range cases {
		if _, errs := parseEffect(src); len(errs) == 0 {
			t.Errorf("target all, %s : erreur de validation attendue", name)
		}
	}
}

func TestTargetAll_Accepted_MinGroupsAbsentOrOne_RemainingStagesAllowed(t *testing.T) {
	uniform := "pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }\n"
	for name, src := range map[string]string{
		"min_groups 1":    allBase + "min_groups: 1\n" + uniform,
		"étape remaining": allBase + uniform + "stages:\n  - when: { remaining_lte: 5 }\n    pattern: { phases: [ { look: a, ms: 500 } ] }\n",
	} {
		if _, errs := parseEffect(src); len(errs) != 0 {
			t.Errorf("%s : accepté, obtenu %+v", name, errs)
		}
	}
}

func TestNatives_CouleurEquipe_IsTargetAll_AppliesAtZeroGroups(t *testing.T) {
	e := nativeEffect(t, "couleur-equipe")
	if e.Target != "all" {
		t.Errorf("couleur-equipe = target: all (planner), obtenu %q", e.Target)
	}
	if ok, reason := applicable(e, 0); !ok {
		t.Errorf("target all applicable sans groupe : %q", reason)
	}
}
