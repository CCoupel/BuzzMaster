package ambiancecfg

// #223 (v11.7.0, LOT-2) — effet lumière par moment dans le modèle mutualisé.
// Source : contracts/ambiance-settings.md §10, contracts/light-effects.md §6.
// Écrit AVANT le code (TDD) : forme supposée, calquée sur `CuesEnabled` —
//   Settings.LightEffects                         map[string]string  (clés chrono|calm|score)
//   Settings.LightHostEffects                     map[id de groupe animateur]map[moment]string  (révision du GATE LOT-1, §6)
//   Partial.*                                     idem, partielles : seuls les groupes et moments surchargés
//   Sources.LightEffects / LightHostEffects       map[string]Source / map[id]map[moment]Source  (global | round)
// Aides préfixées tw223.

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

var tw223Moments = []string{"chrono", "calm", "score"}

func tw223Global() Settings {
	g := tw290aGlobal()
	g.LightEffects = map[string]string{"chrono": "pulsation", "calm": "", "score": ""}
	g.LightHostEffects = map[string]map[string]string{
		"h1": {"chrono": "", "calm": "", "score": ""},
		"h2": {"chrono": "", "calm": "", "score": ""},
	}
	return g
}

func tw223JSON(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("JSON invalide %s : %v", raw, err)
	}
	return m
}

func tw223Dig(m map[string]interface{}, path ...string) interface{} {
	var cur interface{} = m
	for _, k := range path {
		mm, ok := cur.(map[string]interface{})
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

// ---------- Resolve ----------

func TestResolve223_NoLayer_GlobalEffects_AllSourcesGlobal(t *testing.T) {
	got, src := Resolve(tw223Global())
	for _, m := range tw223Moments {
		if got.LightEffects[m] != tw223Global().LightEffects[m] {
			t.Errorf("effects[%s] = %q", m, got.LightEffects[m])
		}
		if string(src.LightEffects[m]) != "global" {
			t.Errorf("source de %s : %q, attendu global", m, src.LightEffects[m])
		}
		for _, id := range []string{"h1", "h2"} {
			if string(src.LightHostEffects[id][m]) != "global" {
				t.Errorf("source host_effects.%s.%s : %q, attendu global", id, m, src.LightHostEffects[id][m])
			}
		}
	}
}

func TestResolve223_RoundOverridesOneMomentOnly(t *testing.T) {
	round := Partial{LightEffects: map[string]string{"calm": "scintillement"}}
	got, src := Resolve(tw223Global(), round)
	if got.LightEffects["calm"] != "scintillement" || got.LightEffects["chrono"] != "pulsation" || got.LightEffects["score"] != "" {
		t.Errorf("seul calm est surchargé : %+v", got.LightEffects)
	}
	if string(src.LightEffects["calm"]) != "round" || string(src.LightEffects["chrono"]) != "global" {
		t.Errorf("sources : %+v", src.LightEffects)
	}
	if !reflect.DeepEqual(got.LightHostEffects, tw223Global().LightHostEffects) {
		t.Errorf("les groupes animateur restent hérités : %+v", got.LightHostEffects)
	}
}

func TestResolve223_EmptyStringIsAnOverride_NotInherited(t *testing.T) {
	// « Aucun effet » pendant le chrono dans la manche : "" ≠ clé absente.
	got, src := Resolve(tw223Global(), Partial{LightEffects: map[string]string{"chrono": ""}})
	if got.LightEffects["chrono"] != "" || string(src.LightEffects["chrono"]) != "round" {
		t.Errorf("\"\" surcharge la valeur globale « pulsation » : %q (%s)", got.LightEffects["chrono"], src.LightEffects["chrono"])
	}
}

func TestResolve223_HostEffectsIndependentFromRoomEffects(t *testing.T) {
	round := Partial{LightHostEffects: map[string]map[string]string{"h1": {"score": "couleur-equipe"}}}
	got, src := Resolve(tw223Global(), round)
	if got.LightHostEffects["h1"]["score"] != "couleur-equipe" || string(src.LightHostEffects["h1"]["score"]) != "round" {
		t.Errorf("host_effects.h1.score : %+v / %+v", got.LightHostEffects, src.LightHostEffects)
	}
	if got.LightEffects["score"] != "" || string(src.LightEffects["score"]) != "global" {
		t.Errorf("effects.score inchangé : %+v", got.LightEffects)
	}
}

func TestResolve223_LastLayerWins_FieldByField(t *testing.T) {
	r := Partial{LightEffects: map[string]string{"chrono": "chenillard", "calm": "a"}}
	d := Partial{LightEffects: map[string]string{"calm": "b"}}
	got, _ := Resolve(tw223Global(), r, d)
	if got.LightEffects["chrono"] != "chenillard" || got.LightEffects["calm"] != "b" {
		t.Errorf("%+v", got.LightEffects)
	}
}

func TestResolve223_PureFunction_NoMutationOfInputs(t *testing.T) {
	g := tw223Global()
	p := Partial{LightEffects: map[string]string{"chrono": "chenillard"}}
	gBefore, pBefore := g.Clone(), p.Clone()
	got, _ := Resolve(g, p)
	got.LightEffects["calm"] = "modifie"
	if !reflect.DeepEqual(g, gBefore) || !reflect.DeepEqual(p, pBefore) {
		t.Error("Resolve ne modifie ni le global ni la couche")
	}
}

func TestClone223_DeepCopiesEffectMaps(t *testing.T) {
	s := tw223Global()
	c := s.Clone()
	c.LightEffects["chrono"] = "x"
	c.LightHostEffects["h1"]["calm"] = "y"
	if s.LightEffects["chrono"] != "pulsation" || s.LightHostEffects["h1"]["calm"] != "" {
		t.Error("Settings.Clone doit copier en profondeur les cartes d'effets")
	}
	p := Partial{LightEffects: map[string]string{"chrono": "a"}, LightHostEffects: map[string]map[string]string{"h1": {"calm": "b"}}}
	pc := p.Clone()
	pc.LightEffects["chrono"] = "z"
	pc.LightHostEffects["h1"]["calm"] = "z"
	if p.LightEffects["chrono"] != "a" || p.LightHostEffects["h1"]["calm"] != "b" {
		t.Error("Partial.Clone doit copier en profondeur")
	}
}

func TestPartial223_IsEmptyAndMerge(t *testing.T) {
	if !(Partial{LightEffects: map[string]string{}, LightHostEffects: map[string]map[string]string{}}).IsEmpty() {
		t.Error("des cartes d'effets vides = Partial vide")
	}
	if (Partial{LightEffects: map[string]string{"calm": ""}}).IsEmpty() {
		t.Error("un effet surchargé à \"\" n'est pas vide")
	}
	a := Partial{LightEffects: map[string]string{"chrono": "a", "calm": "a"}}
	b := Partial{LightEffects: map[string]string{"calm": "b"}, LightHostEffects: map[string]map[string]string{"h2": {"score": "c"}}}
	m := a.Merge(b)
	if m.LightEffects["chrono"] != "a" || m.LightEffects["calm"] != "b" || m.LightHostEffects["h2"]["score"] != "c" {
		t.Errorf("Merge clé par clé, le second l'emporte : %+v", m)
	}
}

// ---------- JSON ----------

func TestJSON223_Settings_CompleteFormAlwaysThreeKeys(t *testing.T) {
	m := tw223JSON(t, tw223Global())
	eff, _ := tw223Dig(m, "light", "effects").(map[string]interface{})
	if len(eff) != 3 {
		t.Fatalf("light.effects : trois clés toujours présentes (pas d'omitempty) : %v", eff)
	}
	hosts, _ := tw223Dig(m, "light", "host_effects").(map[string]interface{})
	if len(hosts) != 2 {
		t.Fatalf("light.host_effects : une entrée par groupe animateur existant : %v", hosts)
	}
	for _, id := range []string{"h1", "h2"} {
		g, _ := hosts[id].(map[string]interface{})
		if len(g) != 3 {
			t.Errorf("light.host_effects.%s : trois moments toujours présents : %v", id, g)
		}
	}
	if tw223Dig(m, "light", "effects", "chrono") != "pulsation" || tw223Dig(m, "light", "effects", "calm") != "" {
		t.Errorf("valeurs : %v", tw223Dig(m, "light", "effects"))
	}
	if tw223Dig(m, "light", "mode") != "AUTO" {
		t.Error("light.mode conservé à côté des effets")
	}
}

func TestJSON223_Settings_NoHostGroup_HostEffectsIsEmptyObject(t *testing.T) {
	g := tw223Global()
	g.LightHostEffects = map[string]map[string]string{}
	m := tw223JSON(t, g)
	hosts, ok := tw223Dig(m, "light", "host_effects").(map[string]interface{})
	if !ok || len(hosts) != 0 {
		t.Errorf("aucun groupe animateur ⇒ {} (jamais absent ni null) : %v", tw223Dig(m, "light", "host_effects"))
	}
}

func TestJSON223_Settings_RoundTrip(t *testing.T) {
	s := tw223Global()
	s.LightEffects["calm"] = "scintillement"
	s.LightHostEffects["h2"]["score"] = "couleur-equipe"
	raw, _ := json.Marshal(s)
	var back Settings
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, s) {
		t.Errorf("aller-retour infidèle :\n got  %+v\n want %+v", back, s)
	}
}

func TestJSON223_Partial_OnlyOverriddenKeys_EmptyMapsOmitted(t *testing.T) {
	raw, _ := json.Marshal(Partial{LightEffects: map[string]string{"calm": ""}})
	var got, want interface{}
	_ = json.Unmarshal(raw, &got)
	_ = json.Unmarshal([]byte(`{"light":{"effects":{"calm":""}}}`), &want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s", raw)
	}
	raw, _ = json.Marshal(Partial{LightEffects: map[string]string{}, LightHostEffects: map[string]map[string]string{}})
	if strings.TrimSpace(string(raw)) != "{}" {
		t.Errorf("cartes vides omises : %s", raw)
	}
}

func TestJSON223_Partial_FromContract_BothBranches(t *testing.T) {
	const doc = `{"light":{"mode":"ON","effects":{"chrono":"chenillard"},"host_effects":{"g5d7e4":{"score":"couleur-equipe"}}}}`
	var p Partial
	if err := json.Unmarshal([]byte(doc), &p); err != nil {
		t.Fatal(err)
	}
	if p.LightEffects["chrono"] != "chenillard" || p.LightHostEffects["g5d7e4"]["score"] != "couleur-equipe" || p.LightMode == nil {
		t.Errorf("%+v", p)
	}
	if _, ok := p.LightEffects["calm"]; ok {
		t.Error("clé absente ⇒ héritée, pas de valeur zéro inventée")
	}
	if _, ok := p.LightHostEffects["g5d7e4"]["chrono"]; ok {
		t.Error("seuls les moments surchargés figurent dans le Partial d'un groupe")
	}
	raw, _ := json.Marshal(p)
	var back Partial
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back, p) {
		t.Errorf("aller-retour : %+v / %v", back, err)
	}
}

func TestJSON223_OldFlatHostEffects_IgnoredWithoutError(t *testing.T) {
	// forme codée au LOT-2 et jamais livrée en PROD : {chrono, calm, score} à plat — lue dans un rounds.json de QUALIF
	var p Partial
	if err := json.Unmarshal([]byte(`{"light":{"host_effects":{"chrono":"","calm":"","score":"couleur-equipe"}}}`), &p); err != nil {
		t.Fatalf("l'ancienne forme plate est ignorée sans erreur : %v", err)
	}
	if len(p.LightHostEffects) != 0 {
		t.Errorf("la forme plate n'est jamais prise pour des id de groupe : %+v", p.LightHostEffects)
	}
	var s Settings
	if err := json.Unmarshal([]byte(`{"light":{"mode":"AUTO","host_effects":{"chrono":"","score":"x"}}}`), &s); err != nil {
		t.Fatalf("idem pour Settings : %v", err)
	}
	if len(s.LightHostEffects) != 0 {
		t.Errorf("%+v", s.LightHostEffects)
	}
}

func TestResolve223_UnknownHostIdInLayer_IgnoredAtResolution(t *testing.T) {
	round := Partial{LightHostEffects: map[string]map[string]string{"disparu": {"score": "couleur-equipe"}, "h1": {"calm": "x"}}}
	got, src := Resolve(tw223Global(), round)
	if _, ok := got.LightHostEffects["disparu"]; ok {
		t.Errorf("un id absent du global (groupe supprimé) est ignoré, sans erreur : %+v", got.LightHostEffects)
	}
	if got.LightHostEffects["h1"]["calm"] != "x" || string(src.LightHostEffects["h1"]["calm"]) != "round" {
		t.Errorf("l'id connu est résolu : %+v", got.LightHostEffects)
	}
}

func TestResolve223_RoundOverridesTwoHostGroupsIndependently(t *testing.T) {
	round := Partial{LightHostEffects: map[string]map[string]string{"h1": {"chrono": "a"}, "h2": {"score": "b"}}}
	got, src := Resolve(tw223Global(), round)
	if got.LightHostEffects["h1"]["chrono"] != "a" || got.LightHostEffects["h1"]["score"] != "" ||
		got.LightHostEffects["h2"]["score"] != "b" || got.LightHostEffects["h2"]["chrono"] != "" {
		t.Errorf("chaque groupe animateur résout ses propres moments : %+v", got.LightHostEffects)
	}
	if string(src.LightHostEffects["h1"]["chrono"]) != "round" || string(src.LightHostEffects["h1"]["score"]) != "global" ||
		string(src.LightHostEffects["h2"]["score"]) != "round" {
		t.Errorf("sources par groupe et moment : %+v", src.LightHostEffects)
	}
}

func TestJSON223_Sources_Form(t *testing.T) {
	_, src := Resolve(tw223Global(), Partial{LightEffects: map[string]string{"calm": "a"}, LightHostEffects: map[string]map[string]string{"h1": {"chrono": "b"}}})
	m := tw223JSON(t, src)
	if tw223Dig(m, "light", "effects", "calm") != "round" || tw223Dig(m, "light", "effects", "chrono") != "global" ||
		tw223Dig(m, "light", "host_effects", "h1", "chrono") != "round" || tw223Dig(m, "light", "host_effects", "h2", "chrono") != "global" {
		t.Errorf("Sources.light.effects.<moment> ∈ global|round : %v", m)
	}
}

// ---------- validation de forme (l'existence de l'id dans le catalogue est vérifiée côté serveur) ----------

func TestValidate223_UnknownMomentRefused_NamedInError(t *testing.T) {
	for _, kind := range []map[string]string{{"bidule": "x"}, {"": "x"}, {"CHRONO": "x"}} {
		if err := (Partial{LightEffects: kind}).Validate(tw290aKnown); err == nil {
			t.Errorf("moment inconnu %v : refus attendu", kind)
		}
		if err := (Partial{LightHostEffects: map[string]map[string]string{"h1": kind}}).Validate(tw290aKnown); err == nil {
			t.Errorf("moment inconnu (animateur) %v : refus attendu", kind)
		}
	}
	err := (Partial{LightEffects: map[string]string{"bidule": "x"}}).Validate(tw290aKnown)
	if err == nil || !strings.Contains(err.Error(), "bidule") {
		t.Errorf("l'erreur nomme le champ fautif : %v", err)
	}
}

func TestValidate223_ThreeMomentsAndEmptyIdAccepted(t *testing.T) {
	p := Partial{
		LightEffects:     map[string]string{"chrono": "chenillard", "calm": "", "score": "couleur-equipe"},
		LightHostEffects: map[string]map[string]string{"h1": {"chrono": "", "calm": "", "score": ""}, "h2": {"score": "couleur-equipe"}},
	}
	if err := p.Validate(tw290aKnown); err != nil {
		t.Errorf("trois moments, id d'effet ou chaîne vide : %v", err)
	}
}

func TestValidate223_InvalidPathsListsLightEffectPaths(t *testing.T) {
	bad := (Partial{LightEffects: map[string]string{"bidule": "x"}}).InvalidPaths(tw290aKnown)
	if len(bad) != 1 || bad[0] != "light.effects.bidule" {
		t.Errorf("chemin JSON fautif : %v", bad)
	}
	bad = (Partial{LightHostEffects: map[string]map[string]string{"h1": {"bidule": "x"}}}).InvalidPaths(tw290aKnown)
	if len(bad) != 1 || bad[0] != "light.host_effects.h1.bidule" {
		t.Errorf("chemin JSON fautif (animateur, par id) : %v", bad)
	}
}

func TestValidate223_EditablePathsParity_RoomAndHostPerGroup(t *testing.T) {
	// parité avec le composant React : light.effects.<m> et light.host_effects.<id>.<m>, trois moments chacun
	for _, m := range tw223Moments {
		if bad := (Partial{LightEffects: map[string]string{m: ""}}).InvalidPaths(tw290aKnown); len(bad) != 0 {
			t.Errorf("light.effects.%s éditable : %v", m, bad)
		}
		if bad := (Partial{LightHostEffects: map[string]map[string]string{"g5d7e4": {m: ""}}}).InvalidPaths(tw290aKnown); len(bad) != 0 {
			t.Errorf("light.host_effects.<id>.%s éditable : %v", m, bad)
		}
	}
}
