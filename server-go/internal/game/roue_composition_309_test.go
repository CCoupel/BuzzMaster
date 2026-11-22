package game

// #309 (v12.1.0, LOT-2) — composition de la roue par question, champ par champ (contracts/roue.md §16.2, §16.3).
//
// Ces tests passent par le VRAI démarrage de manche (START) : la question est construite par JSON (champs ROUE_WHEEL_* du
// contrat, champ présent = personnalisé, absent = global), le réglage global est fourni par `Engine.RoueWheelParamsFn`
// (`RoueWheelParams.BonusCount` y porte le nombre de BONUS global) ; la roue attendue est calculée avec l'oracle existant
// `NormalizeRoueWheelParams` + `BuildRoueWheel`. Ils ne dépendent donc d'aucun identifiant du LOT-2, hors le champ
// `RoueWheelParams.BonusCount` (LOT-1.1) lu comme BONUS global.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// cEngine309 démarre une manche ROUE dont la question porte `over` (champs JSON) sur le réglage global `global`.
func cEngine309(t *testing.T, over vopt, global RoueWheelParams) *Engine {
	t.Helper()
	q := map[string]interface{}{"ID": "c309", "QUESTION": "Roue de la Fortune", "TYPE": "ROUE", "ROUE_PHRASE": "TOTO TATA", "ROUE_MODE": RoueModeTantQueJeGagne}
	for k, v := range over {
		q[k] = v
	}
	raw, _ := json.Marshal(q)
	var question Question
	if err := json.Unmarshal(raw, &question); err != nil {
		t.Fatal(err)
	}
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red", Color: []int{255, 0, 0}}, "blue": {Name: "blue", Color: []int{0, 0, 255}}})
	e.Ready("c309", &question)
	if err := e.RoueSetTeams([]string{"red", "blue"}); err != nil {
		t.Fatal(err)
	}
	e.RoueWheelParamsFn = func() RoueWheelParams { return global }
	e.StartImmediate(0)
	t.Cleanup(e.Stop)
	if e.GetState().Phase != PhaseStarted {
		t.Fatalf("le départ ne doit jamais être refusé : phase %s", e.GetState().Phase)
	}
	return e
}

// oracle309 : la roue (sans marque) d'une composition effective donnée (paramètres normalisés, pas corrigé). Depuis
// v12.2.0 (§17.2) les BONUS sont des marques tirées au hasard : la roue ne se compare qu'une fois les marques retirées
// (`wheelOf309`) ; le nombre de marques attendu se vérifie à part (`marks309`, `bonus` = nombre attendu).
func oracle309(t *testing.T, p RoueWheelParams, bonus int) []RoueSegment {
	t.Helper()
	np, _, err := NormalizeRoueWheelParams(p)
	if err != nil {
		t.Fatalf("composition attendue invalide : %v", err)
	}
	np.BonusCount = 0
	return BuildRoueWheel(np, nil)
}

// wheelOf309 : les cases de la manche, marques BONUS retirées (voir oracle309).
func wheelOf309(e *Engine) []RoueSegment {
	segs := append([]RoueSegment(nil), e.GetState().RoueWheel.Segments...)
	for i := range segs {
		segs[i].Bonus = false
	}
	return segs
}

// marks309 : nombre de cases marquées BONUS de la manche.
func marks309(e *Engine) int { return len(markIndexes308(e.GetState().RoueWheel.Segments)) }

func glob309(max, step, b, p, bonus int) RoueWheelParams {
	g := DefaultRoueWheelParams()
	g.MaxPoints, g.Step, g.BankruptCount, g.LoseTurnCount, g.BonusCount = max, step, b, p, bonus
	return g
}

func eq309(t *testing.T, got, want []RoueSegment, what string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s : roue différente\n got  %+v\n want %+v", what, got, want)
	}
}

// ---- compatibilité : v12.0 et LOT-1 --------------------------------------------------------------------------------

// critical — question sans aucun champ, global sans BONUS : roue globale, identique case pour case à v12.0.
func TestRoueComposition309_NoFieldIsGlobalWheelAndV120(t *testing.T) {
	for _, g := range []RoueWheelParams{DefaultRoueWheelParams(), glob309(100, 10, 1, 1, 0), glob309(1000, 100, 3, 2, 0), glob309(100, 10, 0, 0, 0)} {
		t.Run(fmt.Sprintf("%d/%d/%d/%d", g.MaxPoints, g.Step, g.BankruptCount, g.LoseTurnCount), func(t *testing.T) {
			e := cEngine309(t, nil, g)
			np, _, _ := NormalizeRoueWheelParams(g)
			np.BonusCount = 0
			eq309(t, wheelOf309(e), BuildRoueWheel(np, nil), "v12.0")
		})
	}
}

// Question du LOT-1 : ROUE_BONUS_COUNT seul = BONUS personnalisé, le reste suit le global (roue du LOT-1, aucune migration).
func TestRoueComposition309_Lot1QuestionKeepsItsWheel(t *testing.T) {
	g := DefaultRoueWheelParams()
	for n := 1; n <= 4; n++ {
		e := cEngine309(t, vopt{"ROUE_BONUS_COUNT": n}, g)
		eq309(t, wheelOf309(e), oracle309(t, g, n), fmt.Sprintf("LOT-1 bonus=%d", n))
	}
}

// ---- chaque paramètre personnalisé seul ------------------------------------------------------------------------------

func TestRoueComposition309_EachParameterAlone(t *testing.T) {
	g := glob309(1000, 50, 2, 1, 0)
	cases := []struct {
		name string
		over vopt
		eff  RoueWheelParams
		b    int
	}{
		{"max_points 500", vopt{"ROUE_WHEEL_MAX_POINTS": 500}, glob309(500, 50, 2, 1, 0), 0},
		{"step 100", vopt{"ROUE_WHEEL_STEP": 100}, glob309(1000, 100, 2, 1, 0), 0},
		{"bankrupt 0", vopt{"ROUE_WHEEL_BANKRUPT_COUNT": 0}, glob309(1000, 50, 0, 1, 0), 0},
		{"bankrupt 3", vopt{"ROUE_WHEEL_BANKRUPT_COUNT": 3}, glob309(1000, 50, 3, 1, 0), 0},
		{"lose_turn 0", vopt{"ROUE_WHEEL_LOSE_TURN_COUNT": 0}, glob309(1000, 50, 2, 0, 0), 0},
		{"lose_turn 2", vopt{"ROUE_WHEEL_LOSE_TURN_COUNT": 2}, glob309(1000, 50, 2, 2, 0), 0},
		{"bonus 3", vopt{"ROUE_BONUS_COUNT": 3}, glob309(1000, 50, 2, 1, 0), 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := cEngine309(t, c.over, g)
			eq309(t, wheelOf309(e), oracle309(t, c.eff, c.b), c.name)
		})
	}
}

// Plusieurs paramètres ensemble, et tous les paramètres.
func TestRoueComposition309_SeveralTogether(t *testing.T) {
	g := glob309(1000, 50, 2, 1, 1)
	e := cEngine309(t, vopt{"ROUE_WHEEL_MAX_POINTS": 200, "ROUE_WHEEL_STEP": 20, "ROUE_WHEEL_BANKRUPT_COUNT": 1}, g)
	eq309(t, wheelOf309(e), oracle309(t, glob309(200, 20, 1, 1, 0), 1), "max+step+bankrupt, lose_turn et BONUS globaux")
	e = cEngine309(t, vopt{"ROUE_WHEEL_MAX_POINTS": 100, "ROUE_WHEEL_STEP": 10, "ROUE_WHEEL_BANKRUPT_COUNT": 1, "ROUE_WHEEL_LOSE_TURN_COUNT": 1, "ROUE_BONUS_COUNT": 2}, g)
	eq309(t, wheelOf309(e), oracle309(t, glob309(100, 10, 1, 1, 0), 2), "les cinq personnalisés")
}

// ---- 0 personnalisé ≠ absent -----------------------------------------------------------------------------------------

func TestRoueComposition309_ZeroIsCustomValueNotAbsent(t *testing.T) {
	g := glob309(1000, 50, 2, 1, 2)
	cnt := func(e *Engine, kind string) int { return countKind308(wheelOf309(e), kind) }
	e := cEngine309(t, nil, g) // tout global
	if cnt(e, RoueKindBankrupt) != 2 || cnt(e, RoueKindLoseTurn) != 1 || marks309(e) != 2 {
		t.Fatalf("tout global : %d / %d / %d", cnt(e, RoueKindBankrupt), cnt(e, RoueKindLoseTurn), marks309(e))
	}
	e = cEngine309(t, vopt{"ROUE_WHEEL_BANKRUPT_COUNT": 0, "ROUE_WHEEL_LOSE_TURN_COUNT": 0, "ROUE_BONUS_COUNT": 0}, g)
	if cnt(e, RoueKindBankrupt) != 0 || cnt(e, RoueKindLoseTurn) != 0 || marks309(e) != 0 {
		t.Fatalf("0 personnalisés (≠ absents) : %d / %d / %d, attendu 0 / 0 / 0", cnt(e, RoueKindBankrupt), cnt(e, RoueKindLoseTurn), marks309(e))
	}
	eq309(t, wheelOf309(e), oracle309(t, glob309(1000, 50, 0, 0, 0), 0), "0/0/0 personnalisés")
}

// Une valeur personnalisée égale au global reste personnalisée (décision 10) : elle ne suit plus le global.
func TestRoueComposition309_CustomEqualToGlobalStaysCustom(t *testing.T) {
	q := vopt{"ROUE_WHEEL_MAX_POINTS": 1000}
	e := cEngine309(t, q, glob309(1000, 50, 2, 1, 0))
	eq309(t, wheelOf309(e), oracle309(t, glob309(1000, 50, 2, 1, 0), 0), "égal au global : même roue")
	e = cEngine309(t, q, glob309(2000, 50, 2, 1, 0)) // le global a changé : la question garde 1000
	eq309(t, wheelOf309(e), oracle309(t, glob309(1000, 50, 2, 1, 0), 0), "personnalisé égal à l'ancien global : ne suit pas le nouveau")
}

// ---- suivre le global, ou non ---------------------------------------------------------------------------------------

func TestRoueComposition309_NonCustomFollowsGlobalChange(t *testing.T) {
	over := vopt{"ROUE_WHEEL_BANKRUPT_COUNT": 1} // seul le nombre de BANQUEROUTE est personnalisé
	for _, g := range []RoueWheelParams{glob309(1000, 50, 2, 1, 0), glob309(500, 50, 3, 2, 1), glob309(100, 10, 0, 0, 2)} {
		e := cEngine309(t, over, g)
		eff := g
		eff.BankruptCount = 1
		eff.BonusCount = 0
		eq309(t, wheelOf309(e), oracle309(t, eff, g.BonusCount), fmt.Sprintf("global %d/%d/%d/%d/%d", g.MaxPoints, g.Step, g.BankruptCount, g.LoseTurnCount, g.BonusCount))
		if got := countKind308(wheelOf309(e), RoueKindBankrupt); got != 1 {
			t.Fatalf("BANQUEROUTE personnalisée = 1, roue en a %d", got)
		}
	}
}

func TestRoueComposition309_CustomDoesNotFollowGlobalChange(t *testing.T) {
	over := vopt{"ROUE_WHEEL_MAX_POINTS": 300, "ROUE_WHEEL_STEP": 100}
	a := cEngine309(t, over, glob309(1000, 50, 2, 1, 0))
	b := cEngine309(t, over, glob309(5000, 50, 2, 1, 0))
	// seuls maximum et pas sont personnalisés : les BANQUEROUTE / TOUR PERDU globaux (identiques ici) s'y appliquent
	eq309(t, wheelOf309(a), wheelOf309(b), "max et pas personnalisés, global modifié")
	eq309(t, wheelOf309(a), oracle309(t, glob309(300, 100, 2, 1, 0), 0), "max 300 / pas 100")
}

// ---- pas : corrigé jamais refusé, recorrigé au START -------------------------------------------------------------------

func TestRoueComposition309_StepCorrectedAgainstEffectiveMaximum(t *testing.T) {
	// pas personnalisé 30 avec maximum personnalisé 100 → 25 (§10, 100 / 30 → 25)
	e := cEngine309(t, vopt{"ROUE_WHEEL_MAX_POINTS": 100, "ROUE_WHEEL_STEP": 30}, glob309(1000, 50, 1, 1, 0))
	eq309(t, wheelOf309(e), oracle309(t, glob309(100, 30, 1, 1, 0), 0), "pas 30 → 25")
	// pas personnalisé seul, maximum global qui change : recorrigé au diviseur le plus proche du nouveau maximum
	e = cEngine309(t, vopt{"ROUE_WHEEL_STEP": 70}, glob309(1000, 50, 2, 1, 0)) // 1000 / 70 → 50
	eq309(t, wheelOf309(e), oracle309(t, glob309(1000, 70, 2, 1, 0), 0), "pas 70 sur 1000")
	e = cEngine309(t, vopt{"ROUE_WHEEL_STEP": 70}, glob309(700, 50, 2, 1, 0)) // 700 / 70 → 70 exact
	eq309(t, wheelOf309(e), oracle309(t, glob309(700, 70, 2, 1, 0), 0), "pas 70 sur 700")
}

// ---- composition devenue invalide ------------------------------------------------------------------------------------

// BONUS réduits d'abord (pas de repli) : roue 1000/100/3/2 (14 cases, 1 BONUS au plus, §17.3) + 4 demandés → 1 marque.
func TestRoueComposition309_BonusReducedBeforeAnyFallback(t *testing.T) {
	g := glob309(1000, 100, 3, 2, 0)
	e := cEngine309(t, vopt{"ROUE_BONUS_COUNT": 4}, g)
	segs := wheelOf309(e)
	if len(segs) != 14 || marks309(e) != 1 {
		t.Fatalf("attendu 14 cases dont 1 marque, got %d cases / %d marque(s)", len(segs), marks309(e))
	}
}

// Composition qui reste invalide : roue globale ENTIÈRE (y compris son BONUS global), départ jamais refusé.
func TestRoueComposition309_InvalidCompositionFallsBackToWholeGlobalWheel(t *testing.T) {
	g := glob309(1000, 50, 2, 1, 1)
	want := oracle309(t, g, 1)
	cases := map[string]vopt{
		"pas ≥ maximum":          {"ROUE_WHEEL_MAX_POINTS": 100, "ROUE_WHEEL_STEP": 100},
		"maximum < 2":            {"ROUE_WHEEL_MAX_POINTS": 1},
		"plus de 64 cases":       {"ROUE_WHEEL_BANKRUPT_COUNT": 70},
		"maximum hors limite":    {"ROUE_WHEEL_MAX_POINTS": 2000000},
		"BANQUEROUTE négative":   {"ROUE_WHEEL_BANKRUPT_COUNT": -1},
		"invalide + BONUS perso": {"ROUE_WHEEL_BANKRUPT_COUNT": 70, "ROUE_BONUS_COUNT": 2},
	}
	for name, over := range cases {
		t.Run(name, func(t *testing.T) {
			e := cEngine309(t, over, g)
			eq309(t, wheelOf309(e), want, name)
			if marks309(e) != 1 {
				t.Fatalf("roue globale ENTIÈRE : sa marque BONUS globale reste, got %d", marks309(e))
			}
			if e.GetState().RoueSubPhase != RoueSubTurn {
				t.Fatalf("la manche doit démarrer en TURN : %s", e.GetState().RoueSubPhase)
			}
		})
	}
}

// Un changement du global rend la composition invalide APRÈS l'enregistrement : repli, jamais de refus.
func TestRoueComposition309_GlobalChangedAfterSaveFallsBack(t *testing.T) {
	over := vopt{"ROUE_WHEEL_STEP": 400} // valable avec un maximum de 1000, plus avec un maximum de 400
	ok := cEngine309(t, over, glob309(1000, 50, 2, 1, 0))
	eq309(t, wheelOf309(ok), oracle309(t, glob309(1000, 400, 2, 1, 0), 0), "composition valable")
	g := glob309(400, 50, 2, 1, 0)
	e := cEngine309(t, over, g)
	// pas 400 ≥ maximum 400 : invalide → roue globale entière
	eq309(t, wheelOf309(e), oracle309(t, g, 0), "composition devenue invalide")
}

// ---- durées globales -------------------------------------------------------------------------------------------------

func TestRoueComposition309_DurationsAlwaysGlobal(t *testing.T) {
	g := glob309(1000, 50, 2, 1, 0)
	g.SpinPeriodMs, g.SpinMaxMs, g.StopDurationMs, g.FlipIntervalMs = 3000, 12000, 5000, 900
	for _, over := range []vopt{nil, {"ROUE_WHEEL_MAX_POINTS": 100, "ROUE_WHEEL_STEP": 10}, {"ROUE_BONUS_COUNT": 2}, {"ROUE_WHEEL_BANKRUPT_COUNT": 70}} {
		w := cEngine309(t, over, g).GetState().RoueWheel
		if w.SpinPeriodMs != 3000 || w.SpinMaxMs != 12000 || w.FlipIntervalMs != 900 || w.SpinUpMs != RoueSpinUpMs(3000) {
			t.Fatalf("durées non globales pour %v : période %d / max %d / retournement %d / montée %d", over, w.SpinPeriodMs, w.SpinMaxMs, w.FlipIntervalMs, w.SpinUpMs)
		}
	}
}

// ---- lecture / écriture de la question ------------------------------------------------------------------------------

// JSON : un champ absent reste absent (nil), `0` est conservé, les valeurs > 0 gardent leur forme LOT-1 (aller-retour fichier).
func TestRoueComposition309_QuestionJSONRoundTrip(t *testing.T) {
	var q Question
	if err := json.Unmarshal([]byte(`{"ID":"x","TYPE":"ROUE","ROUE_PHRASE":"A","ROUE_BONUS_COUNT":2}`), &q); err != nil { // fichier LOT-1
		t.Fatal(err)
	}
	out, _ := json.Marshal(q)
	var m map[string]interface{}
	_ = json.Unmarshal(out, &m)
	if m["ROUE_BONUS_COUNT"] != float64(2) {
		t.Fatalf("ROUE_BONUS_COUNT = %v après aller-retour, attendu 2", m["ROUE_BONUS_COUNT"])
	}
	for _, k := range []string{"ROUE_WHEEL_MAX_POINTS", "ROUE_WHEEL_STEP", "ROUE_WHEEL_BANKRUPT_COUNT", "ROUE_WHEEL_LOSE_TURN_COUNT"} {
		if _, present := m[k]; present {
			t.Errorf("%s ne doit pas apparaître quand il est absent (suit le global)", k)
		}
	}
	if err := json.Unmarshal([]byte(`{"ID":"y","TYPE":"ROUE","ROUE_PHRASE":"A","ROUE_WHEEL_BANKRUPT_COUNT":0,"ROUE_BONUS_COUNT":0}`), &q); err != nil {
		t.Fatal(err)
	}
	out, _ = json.Marshal(q)
	m = map[string]interface{}{}
	_ = json.Unmarshal(out, &m)
	if v, present := m["ROUE_WHEEL_BANKRUPT_COUNT"]; !present || v != float64(0) {
		t.Errorf("ROUE_WHEEL_BANKRUPT_COUNT = 0 personnalisé doit être conservé, got %v (présent : %v)", v, present)
	}
	if v, present := m["ROUE_BONUS_COUNT"]; !present || v != float64(0) {
		t.Errorf("ROUE_BONUS_COUNT = 0 personnalisé doit être conservé, got %v (présent : %v)", v, present)
	}
}
