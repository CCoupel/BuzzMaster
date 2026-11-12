package effects

// #210 (v11.8.0, LOT-2, B2.1) — document d'effet ⇄ fichier : aller-retour sur les 5 natifs et les exemples, texte régénéré stable,
// forme JSON, conversion des voisins en variables (grille, E5 révision 7).
// Source : contracts/light-effects.md §11.1, §11.2, §11.4, §11.9. Surface supposée : document_api_spec_test.go. Tags : critical.

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const tw210ListEffect = `id: liste-voisins
nom: Liste et voisins
description: Un effet à liste, voisins, éteint, pre, dernier, post, sans boucle.
version: 2
moments: [chrono, calm]
groupes_min: 2
duree_tick_ms: 500
boucle: non
variables:
  gA: 1; +1
  gC: [g1, g4, g3]
pre:
  { g0: [theme, 100] }
pas:
  - { g0: [scene, 60], gA: ["#FF8800", 100], gA+1: [equipe, 50], gC: [eteint] }
  - { g2: [urgence, 100], gA-1: ["#00AAFF", 30] }
dernier:
  { g0: [eteint] }
post:
  { g1: [theme, 100] }
`

func tw210AllSources(t *testing.T) map[string]string {
	t.Helper()
	m := map[string]string{"exemple-a": tw210ExampleA, "exemple-b": tw210ExampleB, "liste-voisins": tw210ListEffect, "chronogramme-4-5": chronogramSrc}
	for _, id := range []string{"pulsation", "chenillard", "chenillard-traine", "couleur-equipe", "degrade"} {
		b, err := readNative(t, id)
		if err != nil {
			t.Fatal(err)
		}
		m[id] = string(b)
	}
	return m
}

func TestTW210_Critical_RoundTrip_FramesIdentical_N1to12(t *testing.T) {
	defer tw210Catch(t)
	for name, src := range tw210AllSources(t) {
		e1 := mustParse(t, src)
		text, errs := serializeDoc(docOfEffect(e1))
		if len(errs) > 0 {
			t.Errorf("%s : sérialisation refusée : %+v", name, errs)
			continue
		}
		e2, perrs := parseEffect(text)
		if len(perrs) > 0 {
			t.Errorf("%s : le texte régénéré ne se relit pas : %+v\n%s", name, perrs, text)
			continue
		}
		a, b := tw210Trace(t, e1), tw210Trace(t, e2)
		for n := 1; n <= 12; n++ {
			if !reflect.DeepEqual(a[fmt.Sprint(n)], b[fmt.Sprint(n)]) {
				t.Errorf("%s : trames différentes pour N=%d après l'aller-retour", name, n)
				break
			}
		}
		if e1.ID != e2.ID || e1.Name != e2.Name || e1.TickMs != e2.TickMs || e1.MinGroups != e2.MinGroups {
			t.Errorf("%s : champs de tête changés", name)
		}
	}
}

func TestTW210_Serialize_Stable_TwoSavesGiveSameFile(t *testing.T) {
	defer tw210Catch(t)
	for name, src := range tw210AllSources(t) {
		t1, errs := serializeDoc(docOfEffect(mustParse(t, src)))
		if len(errs) > 0 {
			t.Fatalf("%s : %+v", name, errs)
		}
		t2, _ := serializeDoc(docOfEffect(mustParse(t, t1)))
		if t1 != t2 {
			t.Errorf("%s : texte non stable :\n%s\n---\n%s", name, t1, t2)
		}
	}
}

func TestTW210_Serialize_FormatRules(t *testing.T) {
	defer tw210Catch(t)
	text, _ := serializeDoc(docOfEffect(mustParse(t, tw210ListEffect)))
	if !strings.HasSuffix(text, "\n") || strings.Contains(text, "\r") || strings.Contains(text, "#") && !strings.Contains(text, `"#`) {
		t.Errorf("fin de ligne \\n, sans commentaire : %q", text)
	}
	if !strings.Contains(text, "version: 2") {
		t.Error("version: 2 toujours écrite")
	}
	idx := func(s string) int { return strings.Index(text, s) }
	if !(idx("id:") < idx("nom:") && idx("nom:") < idx("version:") && idx("version:") < idx("moments:") && idx("moments:") < idx("pas:")) {
		t.Errorf("ordre des clés du §4.1 :\n%s", text)
	}
	if !strings.Contains(text, "description: >") {
		t.Errorf("description en bloc > :\n%s", text)
	}
	steps := 0
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "- {") {
			steps++
		}
	}
	if steps != 2 {
		t.Errorf("un pas par ligne en écriture { cible: [couleur, intensité] } : %d lignes\n%s", steps, text)
	}
}

func TestTW210_Document_JSONShape_AllFieldsAlwaysPresent(t *testing.T) {
	defer tw210Catch(t)
	d := docOfEffect(mustParse(t, tw210AllSources(t)["pulsation"]))
	for _, k := range []string{"id", "name", "description", "moments", "min_groups", "tick_ms", "loop", "variables", "pre", "steps", "last", "post"} {
		if _, ok := d[k]; !ok {
			t.Errorf("champ %q absent du document", k)
		}
	}
	if v, ok := d["variables"].([]any); !ok || len(v) != 0 {
		t.Errorf("variables: [] et non null : %v", d["variables"])
	}
	if d["last"] != nil {
		t.Errorf("last: null quand le pas est absent : %v", d["last"])
	}
	if _, ok := d["version"]; ok {
		t.Error("version n'est pas dans le document")
	}
}

func TestTW210_Document_Variables_OrderPreserved_StepNullForFollow(t *testing.T) {
	defer tw210Catch(t)
	d := docOfEffect(mustParse(t, tw210ExampleA))
	vars := d["variables"].([]any)
	var names []string
	for _, v := range vars {
		m := v.(map[string]any)
		names = append(names, m["name"].(string))
		if m["name"] == "gB" && m["step"] != nil {
			t.Errorf("gB « suit » : step null, obtenu %v", m["step"])
		}
		if m["name"] == "gA" && (m["start"] != "N/2" || m["step"] != "+1") {
			t.Errorf("gA : start/step en texte d'expression : %v", m)
		}
	}
	if strings.Join(names, ",") != "gB,gC,gA" {
		t.Errorf("ordre du tableau = ordre d'écriture : %v", names)
	}
}

func TestTW210_Document_Actions_OffAndTargets(t *testing.T) {
	defer tw210Catch(t)
	d := docOfEffect(mustParse(t, tw210ListEffect))
	steps := d["steps"].([]any)
	first := steps[0].([]any)
	var gotOff, gotNeighbour bool
	for _, a := range first {
		m := a.(map[string]any)
		if m["target"] == "gC" && m["off"] == true {
			gotOff = true
			if _, has := m["color"]; has {
				t.Error("[eteint] ⇔ {target, off:true} sans color ni intensity")
			}
		}
		if m["target"] == "gA+1" {
			gotNeighbour = true
		}
	}
	if !gotOff || !gotNeighbour {
		t.Errorf("off et voisin écrits comme dans le fichier : %v", first)
	}
}

// --- grille : voisins → variables (E5, révision 7) ----------------------------

func tw210GridFrames(t *testing.T, name, src string) {
	t.Helper()
	e1 := mustParse(t, src)
	g := gridOfDoc(docOfEffect(e1))
	if !g.Editable || g.Effect == nil {
		t.Fatalf("%s : grille éditable attendue : %+v", name, g)
	}
	text, errs := serializeDoc(g.Effect)
	if len(errs) > 0 {
		t.Fatalf("%s : grid.effect invalide : %+v", name, errs)
	}
	e2 := mustParse(t, text)
	a, b := tw210Trace(t, e1), tw210Trace(t, e2)
	for n := 1; n <= 12; n++ {
		if !reflect.DeepEqual(a[fmt.Sprint(n)], b[fmt.Sprint(n)]) {
			t.Errorf("%s : grid.effect ne rend pas les mêmes trames que l'original pour N=%d", name, n)
			return
		}
	}
	for _, st := range g.Effect["steps"].([]any) {
		for _, a := range st.([]any) {
			if tg := a.(map[string]any)["target"].(string); strings.Contains(tg, "+") || strings.Contains(tg, "-") {
				t.Errorf("%s : cible voisine %q restante dans grid.effect", name, tg)
			}
		}
	}
}

func TestTW210_Critical_Grid_ChenillardTraine_NeighboursBecomeFollowers(t *testing.T) {
	defer tw210Catch(t)
	src, _ := readNative(t, "chenillard-traine")
	e := mustParse(t, string(src))
	g := gridOfDoc(docOfEffect(e))
	var names []string
	var decl = map[string]map[string]any{}
	for _, v := range g.Effect["variables"].([]any) {
		m := v.(map[string]any)
		names = append(names, m["name"].(string))
		decl[m["name"].(string)] = m
	}
	if strings.Join(names, ",") != "gB,gC,gA" {
		t.Errorf("variables créées juste avant leur origine, par décalage croissant : %v", names)
	}
	if decl["gB"]["start"] != "gA-1" || decl["gB"]["step"] != nil || decl["gC"]["start"] != "gA+1" || decl["gC"]["step"] != nil {
		t.Errorf("gB: gA-1 et gC: gA+1, sans pas (suivent) : %v %v", decl["gB"], decl["gC"])
	}
	want := [][2]string{{"gA-1", "gB"}, {"gA+1", "gC"}}
	if !reflect.DeepEqual(g.Converted, want) {
		t.Errorf("converted = %v", g.Converted)
	}
	tw210GridFrames(t, "chenillard-traine", string(src))
}

func TestTW210_Grid_NoNeighbour_GridEffectEqualsEffect(t *testing.T) {
	defer tw210Catch(t)
	for _, id := range []string{"pulsation", "chenillard", "couleur-equipe", "degrade"} {
		src, _ := readNative(t, id)
		d := docOfEffect(mustParse(t, string(src)))
		g := gridOfDoc(d)
		if !g.Editable || len(g.Converted) != 0 || !reflect.DeepEqual(g.Effect, d) {
			t.Errorf("%s : grid.effect identique à effect, converted vide : %+v", id, g.Converted)
		}
	}
}

func TestTW210_Grid_NeighbourOfAList_ConvertedSameFrames(t *testing.T) {
	defer tw210Catch(t)
	src := fx("variables:\n  gC: [g1, g4, g3]\npas:\n  - { gC: [\"#FF0000\", 100], gC+1: [\"#00FF00\", 100] }\n")
	tw210GridFrames(t, "voisin de liste", src)
}

func TestTW210_Grid_SameNeighbourInSeveralSteps_OneVariable(t *testing.T) {
	defer tw210Catch(t)
	src := fx("variables:\n  gA: 1; +1\npas:\n  - { gA+1: [\"#00FF00\", 100] }\n  - { gA+1: [\"#0000FF\", 100], gA: [\"#FF0000\", 100] }\n")
	g := gridOfDoc(docOfEffect(mustParse(t, src)))
	if len(g.Converted) != 1 {
		t.Errorf("un même voisin = une seule variable : %v", g.Converted)
	}
	tw210GridFrames(t, "voisin répété", src)
}

func TestTW210_Grid_NewVariableNameIsFirstFreeLetter(t *testing.T) {
	defer tw210Catch(t)
	src := fx("variables:\n  gA: 1; +1\n  gB: 2; +1\npas:\n  - { gA+1: [\"#00FF00\", 100] }\n")
	g := gridOfDoc(docOfEffect(mustParse(t, src)))
	if len(g.Converted) != 1 || g.Converted[0][1] != "gC" {
		t.Errorf("première lettre libre (gC) : %v", g.Converted)
	}
}

func TestTW210_Grid_Over24Variables_NotEditable_Blocked(t *testing.T) {
	defer tw210Catch(t)
	var b strings.Builder
	b.WriteString("variables:\n")
	for _, v := range tw210VarNames(24) {
		fmt.Fprintf(&b, "  %s: 1; +1\n", v)
	}
	b.WriteString("pas:\n  - { gA+1: [theme, 100] }\n")
	e := mustParse(t, fx(b.String()))
	g := gridOfDoc(docOfEffect(e))
	if g.Editable || g.Effect != nil || len(g.Blocked) == 0 || g.Blocked[0] != "gA+1" {
		t.Errorf("24 variables + un voisin : editable=false, effect=null, blocked=[gA+1] : %+v", g)
	}
}

func TestTW210_Serialize_InvalidDocument_Refused(t *testing.T) {
	defer tw210Catch(t)
	d := docOfEffect(mustParse(t, tw210ExampleA))
	d["tick_ms"] = 10 // hors bornes
	if _, errs := serializeDoc(d); len(errs) == 0 {
		t.Error("document invalide refusé par le validateur existant")
	}
}

// tw210Catch : une réponse absente ou de forme inattendue (code non livré) échoue le test au lieu d'interrompre toute la série.
func tw210Catch(t *testing.T) {
	if r := recover(); r != nil {
		t.Errorf("réponse de forme inattendue : %v", r)
	}
}
