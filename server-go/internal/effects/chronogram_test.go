package effects

// PREMIER TEST du lot (ordre Batch R) : le chronogramme de référence du contrat light-effects.md §4.5, tick par tick.
// Modèle figé par l'utilisateur (_work/reports/effects-chronogram-r2-20261111-223000.md, avec la correction « LAST ne fait
// pas avancer les variables »). Salle de 4 groupes, gA: g1; +1 — gB: g2; +2.
//
//	Tick | Joué  | k | gA | gB
//	  0  | PRE   | 0 | g1 | g2
//	  1  | pas 0 | 0 | g1 | g2
//	  2  | pas 1 | 1 | g2 | g4
//	  3  | LAST  | 1 | g2 | g4
//	  4  | pas 0 | 2 | g3 | g2
//	  5  | pas 1 | 3 | g4 | g4
//	  6  | LAST  | 3 | g4 | g4
//	  7  | pas 0 | 4 | g1 | g2
//
// Légende des jetons : « b » bleu doux (#0000FF, 40 %), « W » blanc, « R » rouge.

import (
	"testing"
	"time"
)

const chronogramSrc = `
id: exemple-chronogramme
nom: Exemple
version: 2
moments: [chrono]
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
post:
  { g0: ["#00FF00", 100] }
`

func TestChronogram_ReferenceTable_TickByTick(t *testing.T) {
	got := rows(t, chronogramSrc, ctxN(4), 10)
	want := []string{
		"bbbb", // 0 : PRE — toute la salle en bleu doux
		"Wbbb", // 1 : pas 0 — gA = g1 → blanc
		"WbbR", // 2 : pas 1 — gB = g4 → rouge ; g1 reste blanc (groupe non cité)
		"bbbb", // 3 : LAST — g0 → bleu doux, efface tout
		"bbWb", // 4 : pas 0 — gA = g3 → blanc
		"bbWR", // 5 : pas 1 — gB = g4 → rouge
		"bbbb", // 6 : LAST
		"Wbbb", // 7 : pas 0 — gA = g1 (k = 4, rebouclage)
		"WbbR", // 8 : pas 1 — gB = g4 (k = 5 : ((2-1+2*5) mod 4)+1 = 4)
		"bbbb", // 9 : LAST
	}
	eqRows(t, "chronogramme §4.5", got, want)
}

func TestChronogram_PreAndFirstStepSeeVariablesAtTheirStart(t *testing.T) {
	// PRE et le premier pas 0 voient k = 0 : le blanc du tick 1 est sur g1 (départ de gA), pas sur g2.
	e := mustParse(t, chronogramSrc)
	if tok(renderView(e, ctxN(4), 1).effective(1)) != "W" {
		t.Error("tick 1 : gA n'a pas avancé entre PRE et le premier pas 0 (g1 est blanc)")
	}
	if tok(renderView(e, ctxN(4), 1).effective(2)) != "b" {
		t.Error("tick 1 : g2 reste bleu doux")
	}
}

func TestChronogram_LastDoesNotAdvanceVariables_NextStepSeesNextCran(t *testing.T) {
	// Entre le pas 1 (tick 2, k=1) et le pas 0 suivant (tick 4), k passe de 1 à 2 : un seul cran, LAST ne compte pas.
	// gA au tick 4 = g3 (départ g1, k=2). Si LAST avançait, ce serait g4.
	e := mustParse(t, chronogramSrc)
	v := renderView(e, ctxN(4), 4)
	if row(v, 4) != "bbWb" {
		t.Errorf("tick 4 : %q, attendu bbWb (gA = g3 : LAST n'a pas fait avancer)", row(v, 4))
	}
}

func TestChronogram_VariablesNeverGoBackToZeroWhenLoopRestarts(t *testing.T) {
	e := mustParse(t, chronogramSrc)
	// pas 0 aux ticks 1, 4, 7, 10 : gA = g1, g3, g1, g3 (k = 0, 2, 4, 6) — jamais de retour à k = 0 à chaque tour
	for tick, want := range map[int]string{1: "Wbbb", 4: "bbWb", 7: "Wbbb", 10: "bbWb"} {
		if got := row(renderView(e, ctxN(4), tick), 4); got != want {
			t.Errorf("tick %d : %q, attendu %q", tick, got, want)
		}
	}
}

func TestChronogram_IsPureFunctionOfTheTick(t *testing.T) {
	e := mustParse(t, chronogramSrc)
	// même tick, même trame, quel que soit l'ordre des appels
	for _, tick := range []int{9, 2, 7, 0, 5, 5} {
		a, b := row(renderView(e, ctxN(4), tick), 4), row(renderView(e, ctxN(4), tick), 4)
		if a != b {
			t.Fatalf("tick %d : rendu non déterministe %q / %q", tick, a, b)
		}
	}
}

func TestChronogram_Post_AppliedWithVariablesAtTheirLastValue_NoTickOfItsOwn(t *testing.T) {
	e := mustParse(t, chronogramSrc)
	v, ok := renderPostView(e, ctxN(4), 9)
	if !ok {
		t.Fatal("l'effet a un POST")
	}
	if row(v, 4) != "GGGG" {
		t.Errorf("POST : g0 → vert sur toute la salle : %q", row(v, 4))
	}
	// un effet sans POST : pas de trame de fin
	e2 := mustParse(t, fx("pas:\n  - { g0: [\"#FFFFFF\", 100] }\n"))
	if _, ok := renderPostView(e2, ctxN(4), 3); ok {
		t.Error("pas de post ⇒ rien à publier à la fin du moment")
	}
}

func TestChronogram_Post_UsesLastVariableValue(t *testing.T) {
	src := fx("variables:\n  gA: g1; +1\npas:\n  - { gA: [\"#FFFFFF\", 100] }\npost:\n  { gA: [\"#FF0000\", 100] }\n")
	e := mustParse(t, src)
	// après 3 pas joués (ticks 0, 1, 2 : gA = g1, g2, g3), POST peint le dernier groupe désigné : g3
	v, ok := renderPostView(e, ctxN(4), 2)
	if !ok || tok(v.effective(3)) != "R" {
		t.Errorf("POST sur gA à sa dernière valeur (g3) : %q (%v)", row(v, 4), ok)
	}
}

func TestChronogram_NoPreFirstTickIsStepZero(t *testing.T) {
	// Sans PRE, il n'y a rien à jouer au tick 0 : le pas 0 est joué au tick 0, gA au départ (point d'interprétation du §4.5 :
	// « Si pre existe, il est joué au tick 0 »). Un chenillard natif n'a pas de PRE et démarre sur g1.
	got := rows(t, fx("variables:\n  gA: g1; +1\npas:\n  - { g0: [\"#0000FF\", 40], gA: [\"#FFFFFF\", 100] }\n"), ctxN(4), 5)
	eqRows(t, "sans PRE", got, []string{"Wbbb", "bWbb", "bbWb", "bbbW", "Wbbb"})
}

var _ = time.Second
