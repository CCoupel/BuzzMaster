package effects

// Rendu pas à pas — contracts/light-effects.md §4.2 à §4.6 : variables (progression, incrément négatif, liste, voisins,
// rebouclage pour N = 1, 4, 8, 12), groupe absent, pas sauté, état conservé, ordre des actions, boucle: non, couleurs
// nommées vivantes, trame réduite à Group 0. Aucune attente : le temps est le numéro de tick.
// Jetons (helpers_test.go) : W blanc, R rouge, G vert, B bleu plein, b bleu doux, « . » éteint, « _ » aucun état.

import (
	"strings"
	"testing"
	"time"
)

func headSrc(decl string) string {
	// un seul pas : toute la salle bleu doux, puis la variable gA en blanc
	return fx("variables:\n  " + decl + "\npas:\n  - { g0: [\"#0000FF\", 40], gA: [\"#FFFFFF\", 100] }\n")
}

func headRow(n, pos int) string { // n groupes, le blanc en position pos (1-based)
	return strings.Repeat("b", pos-1) + "W" + strings.Repeat("b", n-pos)
}

// ---------- progression, rebouclage sur N groupes non vides ----------

func TestVariables_Progression_LoopsOnNGroups_ForN_1_4_8_12(t *testing.T) {
	for _, n := range []int{1, 4, 8, 12} {
		got := rows(t, headSrc("gA: g1; +1"), ctxN(n), 2*n+1)
		for tick := range got {
			if want := headRow(n, tick%n+1); got[tick] != want {
				t.Fatalf("N=%d tick %d : %q, attendu %q (un groupe par tick, retour au premier après le dernier)", n, tick, got[tick], want)
			}
		}
	}
}

func TestVariables_StartGroupAndIncrement(t *testing.T) {
	// gB: g2; +2 — N = 4 : g2, g4, g2, g4…
	got := rows(t, headSrc("gA: g2; +2"), ctxN(4), 5)
	eqRows(t, "g2; +2", got, []string{"bWbb", "bbbW", "bWbb", "bbbW", "bWbb"})
	// +3 sur 4 groupes : g1, g4, g3, g2, g1
	eqRows(t, "g1; +3", rows(t, headSrc("gA: g1; +3"), ctxN(4), 5), []string{"Wbbb", "bbbW", "bbWb", "bWbb", "Wbbb"})
}

func TestVariables_NegativeIncrement_FromLastGroup(t *testing.T) {
	// gD: gN; -1 : part du dernier groupe et recule, avec rebouclage
	eqRows(t, "g*; -1", rows(t, headSrc("gA: g*; -1"), ctxN(4), 6), []string{"bbbW", "bbWb", "bWbb", "Wbbb", "bbbW", "bbWb"})
	// -2 sur 5 groupes depuis g2 : g2, g5, g3, g1, g4
	eqRows(t, "g2; -2", rows(t, headSrc("gA: g2; -2"), ctxN(5), 5), []string{"bWbbb", "bbbbW", "bbWbb", "Wbbbb", "bbbWb"})
}

func TestVariables_NZeroIncrementIsConstant(t *testing.T) {
	got := rows(t, headSrc("gA: g3; 0"), ctxN(4), 4)
	eqRows(t, "g3; 0", got, []string{"bbWb", "bbWb", "bbWb", "bbWb"})
}

func TestVariables_StartBeyondN_LoopsOnN(t *testing.T) {
	// g7 avec N = 4 ⇒ position 3 ; avec N = 12 ⇒ position 7
	if got := rows(t, headSrc("gA: g7; 0"), ctxN(4), 1)[0]; got != "bbWb" {
		t.Errorf("g7 avec N = 4 : position 3 : %q", got)
	}
	if got := rows(t, headSrc("gA: g7; 0"), ctxN(12), 1)[0]; got != headRow(12, 7) {
		t.Errorf("g7 avec N = 12 : %q", got)
	}
}

func TestVariables_IncrementBounds_Plus12Minus12(t *testing.T) {
	for _, decl := range []string{"gA: g1; +12", "gA: g1; -12"} {
		if _, errs := parseEffect(headSrc(decl)); len(errs) != 0 {
			t.Errorf("%s : incrément dans les bornes ±12 : %+v", decl, errs)
		}
	}
}

// ---------- liste ----------

func TestVariables_List_OneElementPerCran_RestartsAtTheStart(t *testing.T) {
	got := rows(t, headSrc("gA: [g1, g4, g3, g7]"), ctxN(8), 9)
	// N = 8 : g1, g4, g3, g7 puis reprise au début
	want := []string{headRow(8, 1), headRow(8, 4), headRow(8, 3), headRow(8, 7), headRow(8, 1), headRow(8, 4), headRow(8, 3), headRow(8, 7), headRow(8, 1)}
	eqRows(t, "liste", got, want)
}

func TestVariables_List_AbsentGroupsRemoved(t *testing.T) {
	// N = 4 : g7 n'existe pas, il est retiré de la liste → g1, g4, g3 puis reprise
	got := rows(t, headSrc("gA: [g1, g4, g3, g7]"), ctxN(4), 7)
	eqRows(t, "liste N=4", got, []string{"Wbbb", "bbbW", "bbWb", "Wbbb", "bbbW", "bbWb", "Wbbb"})
}

func TestVariables_EmptyList_ActionsOnVariableIgnored(t *testing.T) {
	// aucun élément de la liste n'existe (N = 2, g5 et g6) : les actions sur gA sont ignorées, g0 s'applique
	got := rows(t, headSrc("gA: [g5, g6]"), ctxN(2), 2)
	eqRows(t, "liste vide", got, []string{"bb", "bb"})
}

func TestVariables_NZero_ActionsOnVariablesIgnored_G0Works(t *testing.T) {
	e := mustParse(t, headSrc("gA: g1; +1"))
	v := renderView(e, ctxN(0), 0)
	if v.Base == nil || tok(*v.Base) != "b" {
		t.Errorf("N = 0 : g0 s'applique (zone entière), gA est ignorée : %+v", v)
	}
	if len(v.Groups) != 0 {
		t.Errorf("N = 0 : aucune entrée de groupe : %+v", v.Groups)
	}
}

// ---------- voisins ----------

func TestNeighbours_PlusMinusOne_WrapAround(t *testing.T) {
	src := fx("variables:\n  gA: g1; +1\npas:\n  - { g0: [\"#0000FF\", 40], gA-1: [\"#00FF00\", 100], gA+1: [\"#FF0000\", 100], gA: [\"#FFFFFF\", 100] }\n")
	got := rows(t, src, ctxN(5), 6)
	// tête g1 : voisins g5 (avant, vert) et g2 (après, rouge) ; tête g5 : g4 vert, g1 rouge
	eqRows(t, "voisins", got, []string{"WRbbG", "GWRbb", "bGWRb", "bbGWR", "RbbGW", "WRbbG"})
}

func TestNeighbours_SingleGroup_NeighbourIsTheGroupItself(t *testing.T) {
	src := fx("variables:\n  gA: g1; +1\npas:\n  - { gA+1: [\"#FF0000\", 100], gA: [\"#FFFFFF\", 100] }\n")
	if got := rows(t, src, ctxN(1), 1)[0]; got != "W" {
		t.Errorf("N = 1 : le voisin est le groupe lui-même, la dernière action gagne : %q", got)
	}
}

func TestNeighbours_PlusTwo(t *testing.T) {
	src := fx("variables:\n  gA: g1; +1\npas:\n  - { g0: [\"#0000FF\", 40], gA+2: [\"#FFFFFF\", 100] }\n")
	eqRows(t, "gA+2", rows(t, src, ctxN(4), 5), []string{"bbWb", "bbbW", "Wbbb", "bWbb", "bbWb"})
}

// ---------- cibles ----------

func TestTargets_FixedGroup_AbsentOrEmptyIgnored(t *testing.T) {
	// g9 n'existe pas avec N = 4 : cette action est ignorée, g2 s'applique
	src := fx("pas:\n  - { g0: [\"#0000FF\", 40], g9: [\"#FF0000\", 100], g2: [\"#FFFFFF\", 100] }\n")
	if got := rows(t, src, ctxN(4), 1)[0]; got != "bWbb" {
		t.Errorf("g9 ignoré, g2 blanc : %q", got)
	}
}

func TestTargets_LastGroup_GStar(t *testing.T) {
	src := fx("pas:\n  - { g0: [\"#0000FF\", 40], g*: [\"#FFFFFF\", 100] }\n")
	for _, n := range []int{1, 4, 12} {
		if got := rows(t, src, ctxN(n), 1)[0]; got != headRow(n, n) {
			t.Errorf("gN avec N = %d : le dernier groupe : %q", n, got)
		}
	}
}

func TestTargets_G0IsTheWholeZone_IncludingUngroupedBulbs_BaseOnly(t *testing.T) {
	// trame réduite : un effet qui n'écrit que g0 ne produit qu'une base, aucun état de groupe propre (§4.6)
	e := mustParse(t, fx("pas:\n  - { g0: [\"#FFFFFF\", 100] }\n"))
	v := renderView(e, ctxN(6), 0)
	if v.Base == nil || tok(*v.Base) != "W" {
		t.Fatalf("base = g0 en blanc : %+v", v.Base)
	}
	for i, g := range v.Groups {
		if !g.Absent {
			t.Errorf("groupe %d : aucun état propre pour un effet qui n'écrit que g0 (une seule écriture buzzmaster-general) : %+v", i+1, g)
		}
	}
}

func TestFrame_GroupActionWithoutG0_BaseStaysUnwritten(t *testing.T) {
	e := mustParse(t, fx("pas:\n  - { g2: [\"#FFFFFF\", 100] }\n"))
	v := renderView(e, ctxN(4), 0)
	if v.Base != nil {
		t.Errorf("g0 jamais écrit : pas de base, la scène ordinaire s'applique : %+v", *v.Base)
	}
	if row(v, 4) != "_W__" {
		t.Errorf("seul g2 a un état : %q", row(v, 4))
	}
}

func TestFrame_GroupsDifferingFromBaseOnly(t *testing.T) {
	// g0 bleu doux puis g1 blanc : seul g1 diffère de la base ; g0 repeint ensuite tout : plus aucun état de groupe
	e := mustParse(t, fx("pas:\n  - { g0: [\"#0000FF\", 40], g1: [\"#FFFFFF\", 100] }\n  - { g0: [\"#0000FF\", 40] }\n"))
	v0, v1 := renderView(e, ctxN(3), 0), renderView(e, ctxN(3), 1)
	if v0.Groups[0].Absent || !v0.Groups[1].Absent || !v0.Groups[2].Absent {
		t.Errorf("tick 0 : seul g1 a un état propre : %+v", v0.Groups)
	}
	for i, g := range v1.Groups {
		if !g.Absent {
			t.Errorf("tick 1 : g0 repeint toute la zone, le groupe %d n'a plus d'état propre : %+v", i+1, g)
		}
	}
}

// ---------- pas sauté, état conservé, ordre ----------

func TestSkippedStep_DoesNotConsumeATick_NorAdvanceVariables(t *testing.T) {
	// le premier pas ne vise que g9 (absent avec N = 4) : il est sauté ; il ne consomme aucun tick
	src := fx("variables:\n  gA: g1; +1\npas:\n  - { g9: [\"#FF0000\", 100] }\n  - { g0: [\"#0000FF\", 40], gA: [\"#FFFFFF\", 100] }\n")
	eqRows(t, "pas sauté", rows(t, src, ctxN(4), 5), []string{"Wbbb", "bWbb", "bbWb", "bbbW", "Wbbb"})
}

func TestSkippedStep_BecomesPlayedWhenTheGroupExists(t *testing.T) {
	src := fx("pas:\n  - { g0: [\"#0000FF\", 40] }\n  - { g6: [\"#FF0000\", 100] }\n")
	if got := rows(t, src, ctxN(4), 2); got[0] != "bbbb" || got[1] != "bbbb" {
		t.Errorf("N = 4 : g6 absent, 2e pas sauté, le 1er se rejoue à chaque tick : %v", got)
	}
	if got := rows(t, src, ctxN(6), 2); got[1] != "bbbbbR" {
		t.Errorf("N = 6 : g6 existe, le 2e pas est joué au tick 1 : %v", got)
	}
}

func TestGroupNotCited_KeepsItsState(t *testing.T) {
	src := fx("pas:\n  - { g1: [\"#FF0000\", 100] }\n  - { g2: [\"#00FF00\", 100] }\n")
	got := rows(t, src, ctxN(3), 3)
	eqRows(t, "état conservé", got, []string{"R__", "RG_", "RG_"}) // 3e tick : le pas 0 rejoue g1, g2 garde le vert
}

func TestActions_AppliedInWrittenOrder_LastWinsOnSameBulb(t *testing.T) {
	a := rows(t, fx("variables:\n  gA: g2; 0\npas:\n  - { g0: [\"#0000FF\", 40], gA: [\"#FFFFFF\", 100] }\n"), ctxN(3), 1)[0]
	b := rows(t, fx("variables:\n  gA: g2; 0\npas:\n  - { gA: [\"#FFFFFF\", 100], g0: [\"#0000FF\", 40] }\n"), ctxN(3), 1)[0]
	if a != "bWb" {
		t.Errorf("g0 puis gA : gA repeint son groupe : %q", a)
	}
	if b != "bbb" {
		t.Errorf("gA puis g0 : g0 repeint tout : %q", b)
	}
}

func TestActions_SameBulbTwiceInOneStep_LastWins(t *testing.T) {
	if got := rows(t, fx("pas:\n  - { g1: [\"#FF0000\", 100], g*: [\"#00FF00\", 100] }\n"), ctxN(1), 1)[0]; got != "G" {
		t.Errorf("g1 = gN avec N = 1 : la dernière action gagne : %q", got)
	}
}

// ---------- boucle: non ----------

func TestLoopNo_OneRound_ThenStateHeldUntilEndOfMoment(t *testing.T) {
	src := fx("boucle: non\nvariables:\n  gA: g1; +1\npas:\n  - { g0: [\"#0000FF\", 40], gA: [\"#FFFFFF\", 100] }\n  - { gA: [\"#FF0000\", 100] }\ndernier:\n  { g1: [\"#00FF00\", 100] }\n")
	got := rows(t, src, ctxN(4), 8)
	// tick 0 : pas 0 (gA = g1) ; tick 1 : pas 1 (gA = g2) ; tick 2 : dernier ; puis l'état du dernier est tenu
	want := []string{"Wbbb", "WRbb", "GRbb", "GRbb", "GRbb", "GRbb", "GRbb", "GRbb"}
	eqRows(t, "boucle: non", got, want)
}

func TestLoopYes_IsTheDefault(t *testing.T) {
	src := fx("variables:\n  gA: g1; +1\npas:\n  - { g0: [\"#0000FF\", 40], gA: [\"#FFFFFF\", 100] }\n")
	if rows(t, src, ctxN(4), 5)[4] != "Wbbb" {
		t.Error("sans clé boucle, l'effet tourne (défaut : oui)")
	}
}

// ---------- couleurs et intensités ----------

func TestColours_HexAndIntensityPercentToScale255(t *testing.T) {
	e := mustParse(t, fx("pas:\n  - { g1: [\"#102030\", 40] }\n"))
	l := renderView(e, ctxN(2), 0).effective(1)
	if l.Color != [3]int{16, 32, 48} || !near(l.Intensity, 102, 1) {
		t.Errorf("#102030 à 40 %% → intensité ~102/255 : %+v", l)
	}
	e = mustParse(t, fx("pas:\n  - { g1: [\"#FFFFFF\", 100], g2: [\"#FFFFFF\", 50] }\n"))
	v := renderView(e, ctxN(2), 0)
	if v.effective(1).Intensity != 255 || !near(v.effective(2).Intensity, 128, 1) {
		t.Errorf("100 %% → 255, 50 %% → ~128 : %+v", v.Groups)
	}
}

func TestColours_Eteint_IntensityZero(t *testing.T) {
	e := mustParse(t, fx("pas:\n  - { g0: [\"#FFFFFF\", 100], g2: [eteint] }\n"))
	v := renderView(e, ctxN(3), 0)
	if row(v, 3) != "W.W" {
		t.Errorf("[eteint] éteint le groupe : %q", row(v, 3))
	}
}

func TestColours_Named_ThemeSceneTeam_ResolvedAtRender(t *testing.T) {
	e := mustParse(t, "id: t\nnom: T\nversion: 2\nmoments: [score]\nduree_tick_ms: 1000\npas:\n  - { g1: [theme, 100], g2: [scene, 100], g3: [equipe, 100] }\n")
	c := ctxN(3)
	c.Moment = "score"
	v := renderView(e, c, 0)
	if v.effective(1).Color != cT || v.effective(2).Color != cScn || v.effective(3).Color != cTeam {
		t.Errorf("theme, scene, equipe : %+v", v.Groups)
	}
}

func TestColours_Equipe_WhiteOutsideScoreMoment(t *testing.T) {
	e := mustParse(t, fx("pas:\n  - { g1: [equipe, 100] }\n"))
	if got := renderView(e, ctxN(2), 0).effective(1).Color; got != cW {
		t.Errorf("equipe hors moment score : blanc : %v", got)
	}
}

func TestColours_Urgence_WhiteOrangeRed_LiveAtEveryRender(t *testing.T) {
	e := mustParse(t, fx("variables:\n  gA: g1; +1\npas:\n  - { g0: [theme, 100], gA: [urgence, 100] }\n"))
	cases := []struct {
		rem  time.Duration
		want [3]int
	}{
		{20 * time.Second, cW}, {10*time.Second + time.Millisecond, cW}, {10 * time.Second, cOr},
		{5*time.Second + time.Millisecond, cOr}, {5 * time.Second, cRed}, {time.Second, cRed},
	}
	for _, tc := range cases {
		c := ctxN(4)
		c.Remaining = tc.rem
		if got := renderView(e, c, 0).effective(1).Color; got != tc.want {
			t.Errorf("reste %s : tête %v, attendu %v", tc.rem, got, tc.want)
		}
	}
}

func TestColours_Urgence_IsLive_PaintedGroupTurnsOnItsOwn(t *testing.T) {
	// un groupe peint en urgence au tick 0 vire à l'orange / rouge sans nouvelle action : même tick, contexte différent
	e := mustParse(t, fx("pas:\n  - { g1: [urgence, 100] }\n"))
	early, late := ctxN(2), ctxN(2)
	late.Remaining = 4 * time.Second
	if renderView(e, early, 3).effective(1).Color != cW || renderView(e, late, 3).effective(1).Color != cRed {
		t.Error("les couleurs nommées sont résolues à chaque rendu")
	}
}

func TestColours_Urgence_WhiteOutsideChronoMoment(t *testing.T) {
	e := mustParse(t, fx("pas:\n  - { g1: [urgence, 100] }\n"))
	c := ctxN(2)
	c.Moment = "calm"
	c.Remaining = 3 * time.Second
	if got := renderView(e, c, 0).effective(1).Color; got != cW {
		t.Errorf("urgence hors chrono : blanc : %v", got)
	}
}

// ---------- applicabilité §4.7 ----------

func TestApplicability_GroupesMin_AndAllStepsSkipped(t *testing.T) {
	e := mustParse(t, "id: t\nnom: T\nversion: 2\nmoments: [chrono]\ngroupes_min: 3\nduree_tick_ms: 1000\npas:\n  - { g1: [\"#FFFFFF\", 100] }\n")
	for n, want := range map[int]bool{0: false, 1: false, 2: false, 3: true, 12: true} {
		ok, reason := applicable(e, n)
		if ok != want {
			t.Errorf("groupes_min 3, N = %d : applicable = %v, attendu %v", n, ok, want)
		}
		if !ok && reason == "" {
			t.Errorf("N = %d : une raison est exigée (« demande 3 groupes, %d configurés »)", n, n)
		}
	}
	// tous les pas de la boucle sont sautés quand N < 5 : inapplicable
	skip := mustParse(t, fx("pas:\n  - { g5: [\"#FFFFFF\", 100] }\n"))
	if ok, reason := applicable(skip, 4); ok || reason == "" {
		t.Errorf("g5 avec N = 4 : tous les pas sautés ⇒ inapplicable avec une raison : %v %q", ok, reason)
	}
	if ok, _ := applicable(skip, 5); !ok {
		t.Error("g5 avec N = 5 : applicable")
	}
}

func TestApplicability_G0OnlyEffect_AlwaysApplicable_EvenAtN0_AndOnHostGroup(t *testing.T) {
	e := mustParse(t, fx("pas:\n  - { g0: [\"#FFFFFF\", 100] }\n"))
	for _, n := range []int{0, 1, 12} {
		if ok, reason := applicable(e, n); !ok {
			t.Errorf("N = %d : un effet qui n'écrit que g0 est applicable : %q", n, reason)
		}
	}
	if !hostApplicable(e) {
		t.Error("groupe animateur : un effet qui n'écrit que g0 lui est applicable")
	}
}

func TestApplicability_HostRequiresG0Only_AcrossPreStepsLastPost(t *testing.T) {
	for name, body := range map[string]string{
		"pas":      "pas:\n  - { g0: [\"#FFFFFF\", 100], g1: [\"#FF0000\", 100] }\n",
		"pre":      "pre:\n  { g2: [\"#FF0000\", 100] }\npas:\n  - { g0: [\"#FFFFFF\", 100] }\n",
		"dernier":  "pas:\n  - { g0: [\"#FFFFFF\", 100] }\ndernier:\n  { g*: [\"#FF0000\", 100] }\n",
		"post":     "pas:\n  - { g0: [\"#FFFFFF\", 100] }\npost:\n  { g1: [\"#FF0000\", 100] }\n",
		"variable": "variables:\n  gA: g1; +1\npas:\n  - { g0: [\"#FFFFFF\", 100], gA: [\"#FF0000\", 100] }\n",
	} {
		if hostApplicable(mustParse(t, fx(body))) {
			t.Errorf("%s : une autre cible que g0 rend l'effet inapplicable à un groupe animateur", name)
		}
	}
}

func TestRender_ChronoMoment_N1_SingleGroupEffects(t *testing.T) {
	// pour un groupe animateur : g0 = g1 = ce groupe, N = 1
	e := mustParse(t, fx("pas:\n  - { g0: [\"#FFFFFF\", 100] }\n"))
	v := renderView(e, ctxN(1), 0)
	if v.effective(1).Color != cW {
		t.Errorf("N = 1 : %+v", v)
	}
}
