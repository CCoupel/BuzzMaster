package ambiancecfg

// #223 (v11.7.0, LOT-2) — effets lumière PAR ZONE et par moment dans le modèle mutualisé (révision 4).
// Source : contracts/ambiance-settings.md §10, contracts/light-effects.md §6.
// Forme : Settings.LightEffects map[zone]map[moment]string (zones ambiance|animateur|public ; moments chrono|calm|score),
// Partial idem (partielle), Sources.LightEffects map[zone]map[moment]Source ; JSON light.effects.<zone>.<moment>.
// Plus de LightHostEffects, plus d'id de groupe : le groupe n'est la clé d'aucun réglage. Aides préfixées tw223.

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

var tw223Moments = []string{"chrono", "calm", "score"}
var tw223Zones = []string{"ambiance", "animateur", "public"}

func tw223Global() Settings {
	g := tw290aGlobal()
	g.LightEffects = DefaultLightEffects()
	g.LightSpeeds = DefaultLightSpeeds()
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

func tw223Eff(zone, moment, id string) map[string]map[string]string {
	return map[string]map[string]string{zone: {moment: id}}
}

// ---------- défauts ----------

func TestDefaults223_PulsationForAmbianceChrono_NothingElsewhere(t *testing.T) {
	d := DefaultLightEffects()
	for _, z := range tw223Zones {
		if len(d[z]) != 3 {
			t.Fatalf("zone %s : trois moments toujours présents : %v", z, d[z])
		}
		for _, m := range tw223Moments {
			want := ""
			if z == "ambiance" && m == "chrono" {
				want = "pulsation"
			}
			if d[z][m] != want {
				t.Errorf("défaut %s.%s = %q, attendu %q", z, m, d[z][m], want)
			}
		}
	}
}

// ---------- Resolve ----------

func TestResolve223_NoLayer_GlobalEffects_AllSourcesGlobal(t *testing.T) {
	got, src := Resolve(tw223Global())
	for _, z := range tw223Zones {
		for _, m := range tw223Moments {
			if got.LightEffects[z][m] != tw223Global().LightEffects[z][m] {
				t.Errorf("effects[%s][%s] = %q", z, m, got.LightEffects[z][m])
			}
			if string(src.LightEffects[z][m]) != "global" {
				t.Errorf("source de %s.%s : %q, attendu global", z, m, src.LightEffects[z][m])
			}
		}
	}
}

func TestResolve223_RoundOverridesOneZoneAndMomentOnly(t *testing.T) {
	got, src := Resolve(tw223Global(), Partial{LightEffects: tw223Eff("animateur", "calm", "scintillement")})
	if got.LightEffects["animateur"]["calm"] != "scintillement" {
		t.Errorf("animateur.calm surchargé : %+v", got.LightEffects)
	}
	if got.LightEffects["ambiance"]["chrono"] != "pulsation" || got.LightEffects["ambiance"]["calm"] != "" || got.LightEffects["public"]["calm"] != "" {
		t.Errorf("les autres zones et moments restent hérités : %+v", got.LightEffects)
	}
	if string(src.LightEffects["animateur"]["calm"]) != "round" || string(src.LightEffects["animateur"]["chrono"]) != "global" || string(src.LightEffects["ambiance"]["calm"]) != "global" {
		t.Errorf("sources : %+v", src.LightEffects)
	}
}

func TestResolve223_EmptyStringIsAnOverride_NotInherited(t *testing.T) {
	// « Aucun effet » pendant le chrono dans la manche : "" ≠ clé absente.
	got, src := Resolve(tw223Global(), Partial{LightEffects: tw223Eff("ambiance", "chrono", "")})
	if got.LightEffects["ambiance"]["chrono"] != "" || string(src.LightEffects["ambiance"]["chrono"]) != "round" {
		t.Errorf("\"\" surcharge la valeur globale « pulsation » : %q (%s)", got.LightEffects["ambiance"]["chrono"], src.LightEffects["ambiance"]["chrono"])
	}
}

func TestResolve223_ZonesAreIndependent_SameMoment(t *testing.T) {
	round := Partial{LightEffects: map[string]map[string]string{"public": {"score": "couleur-equipe"}, "animateur": {"score": "x"}}}
	got, src := Resolve(tw223Global(), round)
	if got.LightEffects["public"]["score"] != "couleur-equipe" || got.LightEffects["animateur"]["score"] != "x" || got.LightEffects["ambiance"]["score"] != "" {
		t.Errorf("chaque zone résout son propre moment : %+v", got.LightEffects)
	}
	if string(src.LightEffects["public"]["score"]) != "round" || string(src.LightEffects["ambiance"]["score"]) != "global" {
		t.Errorf("sources : %+v", src.LightEffects)
	}
}

func TestResolve223_LastLayerWins_FieldByField(t *testing.T) {
	r := Partial{LightEffects: map[string]map[string]string{"ambiance": {"chrono": "chenillard", "calm": "a"}}}
	d := Partial{LightEffects: tw223Eff("ambiance", "calm", "b")}
	got, _ := Resolve(tw223Global(), r, d)
	if got.LightEffects["ambiance"]["chrono"] != "chenillard" || got.LightEffects["ambiance"]["calm"] != "b" {
		t.Errorf("%+v", got.LightEffects)
	}
}

func TestResolve223_PureFunction_NoMutationOfInputs(t *testing.T) {
	g := tw223Global()
	p := Partial{LightEffects: tw223Eff("ambiance", "chrono", "chenillard")}
	gBefore, pBefore := g.Clone(), p.Clone()
	got, _ := Resolve(g, p)
	got.LightEffects["ambiance"]["calm"] = "modifie"
	if !reflect.DeepEqual(g, gBefore) || !reflect.DeepEqual(p, pBefore) {
		t.Error("Resolve ne modifie ni le global ni la couche")
	}
}

func TestResolve223_SettingsWithoutEffects_GetTheThreeByThreeDefaults(t *testing.T) {
	g := tw290aGlobal() // construit sans effets : anciens appelants
	got, _ := Resolve(g)
	for _, z := range tw223Zones {
		if len(got.LightEffects[z]) != 3 || len(got.LightSpeeds[z]) != 3 {
			t.Errorf("forme complète 3 × 3 toujours résolue : %s %v %v", z, got.LightEffects[z], got.LightSpeeds[z])
		}
	}
	if got.LightEffects["ambiance"]["chrono"] != "pulsation" {
		t.Errorf("défaut pulsation : %v", got.LightEffects)
	}
}

func TestClone223_DeepCopiesEffectMaps(t *testing.T) {
	s := tw223Global()
	c := s.Clone()
	c.LightEffects["animateur"]["calm"] = "y"
	if s.LightEffects["animateur"]["calm"] != "" {
		t.Error("Settings.Clone doit copier en profondeur les cartes d'effets")
	}
	p := Partial{LightEffects: tw223Eff("public", "calm", "b")}
	pc := p.Clone()
	pc.LightEffects["public"]["calm"] = "z"
	if p.LightEffects["public"]["calm"] != "b" {
		t.Error("Partial.Clone doit copier en profondeur")
	}
}

func TestPartial223_IsEmptyAndMerge(t *testing.T) {
	if !(Partial{LightEffects: map[string]map[string]string{}}).IsEmpty() {
		t.Error("des cartes d'effets vides = Partial vide")
	}
	if (Partial{LightEffects: tw223Eff("ambiance", "calm", "")}).IsEmpty() {
		t.Error("un effet surchargé à \"\" n'est pas vide")
	}
	a := Partial{LightEffects: map[string]map[string]string{"ambiance": {"chrono": "a", "calm": "a"}}}
	b := Partial{LightEffects: map[string]map[string]string{"ambiance": {"calm": "b"}, "public": {"score": "c"}}}
	m := a.Merge(b)
	if m.LightEffects["ambiance"]["chrono"] != "a" || m.LightEffects["ambiance"]["calm"] != "b" || m.LightEffects["public"]["score"] != "c" {
		t.Errorf("Merge zone par zone, moment par moment, le second l'emporte : %+v", m)
	}
}

// ---------- JSON ----------

func TestJSON223_Settings_CompleteFormAlwaysThreeZonesByThreeMoments(t *testing.T) {
	m := tw223JSON(t, tw223Global())
	for _, z := range tw223Zones {
		zm, _ := tw223Dig(m, "light", "effects", z).(map[string]interface{})
		if len(zm) != 3 {
			t.Fatalf("light.effects.%s : trois moments toujours présents (pas d'omitempty) : %v", z, zm)
		}
	}
	if tw223Dig(m, "light", "effects", "ambiance", "chrono") != "pulsation" || tw223Dig(m, "light", "effects", "ambiance", "calm") != "" {
		t.Errorf("valeurs : %v", tw223Dig(m, "light", "effects"))
	}
	if tw223Dig(m, "light", "mode") != "AUTO" {
		t.Error("light.mode conservé à côté des effets")
	}
	if _, present := tw223Dig(m, "light").(map[string]interface{})["host_effects"]; present {
		t.Error("light.host_effects n'existe plus")
	}
}

func TestJSON223_Settings_RoundTrip(t *testing.T) {
	s := tw223Global()
	s.LightEffects["ambiance"]["calm"] = "scintillement"
	s.LightEffects["public"]["score"] = "couleur-equipe"
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
	raw, _ := json.Marshal(Partial{LightEffects: tw223Eff("animateur", "calm", "")})
	var got, want interface{}
	_ = json.Unmarshal(raw, &got)
	_ = json.Unmarshal([]byte(`{"light":{"effects":{"animateur":{"calm":""}}}}`), &want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s", raw)
	}
	raw, _ = json.Marshal(Partial{LightEffects: map[string]map[string]string{}})
	if strings.TrimSpace(string(raw)) != "{}" {
		t.Errorf("cartes vides omises : %s", raw)
	}
}

func TestJSON223_Partial_FromContract(t *testing.T) {
	const doc = `{"light":{"mode":"ON","effects":{"ambiance":{"chrono":"chenillard"},"public":{"score":"couleur-equipe"}}}}`
	var p Partial
	if err := json.Unmarshal([]byte(doc), &p); err != nil {
		t.Fatal(err)
	}
	if p.LightEffects["ambiance"]["chrono"] != "chenillard" || p.LightEffects["public"]["score"] != "couleur-equipe" || p.LightMode == nil {
		t.Errorf("%+v", p)
	}
	if _, ok := p.LightEffects["ambiance"]["calm"]; ok {
		t.Error("clé absente ⇒ héritée, pas de valeur zéro inventée")
	}
	if _, ok := p.LightEffects["animateur"]; ok {
		t.Error("zone absente ⇒ héritée")
	}
	raw, _ := json.Marshal(p)
	var back Partial
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back, p) {
		t.Errorf("aller-retour : %+v / %v", back, err)
	}
}

func TestJSON223_OldFlatEffects_ReadAsAmbianceZone_HostKeysIgnored(t *testing.T) {
	// forme à plat d'un rounds.json de QUALIF : les moments à plat sont la zone ambiance ; host_effects est ignoré
	var p Partial
	doc := `{"light":{"effects":{"chrono":"chenillard","calm":""},"host_effects":{"g5d7e4":{"score":"couleur-equipe"}}}}`
	if err := json.Unmarshal([]byte(doc), &p); err != nil {
		t.Fatalf("l'ancienne forme se lit sans erreur : %v", err)
	}
	if p.LightEffects["ambiance"]["chrono"] != "chenillard" {
		t.Errorf("effets à plat ⇒ zone ambiance : %+v", p.LightEffects)
	}
	if _, ok := p.LightEffects["animateur"]; ok {
		t.Errorf("host_effects ignoré, jamais repris dans une zone : %+v", p.LightEffects)
	}
	var s Settings
	if err := json.Unmarshal([]byte(`{"light":{"mode":"AUTO","effects":{"chrono":"a"},"host_effects":{"chrono":"","score":"x"}}}`), &s); err != nil {
		t.Fatalf("idem pour Settings : %v", err)
	}
	got, _ := Resolve(s)
	if got.LightEffects["ambiance"]["chrono"] != "a" || got.LightEffects["animateur"]["score"] != "" {
		t.Errorf("%+v", got.LightEffects)
	}
}

func TestJSON223_Sources_Form(t *testing.T) {
	_, src := Resolve(tw223Global(), Partial{LightEffects: map[string]map[string]string{"ambiance": {"calm": "a"}, "public": {"chrono": "b"}}})
	m := tw223JSON(t, src)
	if tw223Dig(m, "light", "effects", "ambiance", "calm") != "round" || tw223Dig(m, "light", "effects", "ambiance", "chrono") != "global" ||
		tw223Dig(m, "light", "effects", "public", "chrono") != "round" || tw223Dig(m, "light", "effects", "animateur", "chrono") != "global" {
		t.Errorf("Sources.light.effects.<zone>.<moment> ∈ global|round : %v", m)
	}
}

// ---------- validation de forme (l'existence de l'id dans le catalogue est vérifiée côté serveur) ----------

func TestValidate223_UnknownMomentOrZoneRefused_NamedInError(t *testing.T) {
	for _, p := range []Partial{
		{LightEffects: tw223Eff("ambiance", "bidule", "x")},
		{LightEffects: tw223Eff("ambiance", "", "x")},
		{LightEffects: tw223Eff("ambiance", "CHRONO", "x")},
		{LightEffects: tw223Eff("salle", "chrono", "x")},
		{LightEffects: tw223Eff("g5d7e4", "chrono", "x")},
	} {
		if err := p.Validate(tw290aKnown); err == nil {
			t.Errorf("zone ou moment inconnu %v : refus attendu", p.LightEffects)
		}
	}
	err := (Partial{LightEffects: tw223Eff("ambiance", "bidule", "x")}).Validate(tw290aKnown)
	if err == nil || !strings.Contains(err.Error(), "bidule") {
		t.Errorf("l'erreur nomme le champ fautif : %v", err)
	}
}

func TestValidate223_ThreeZonesThreeMomentsAndEmptyIdAccepted(t *testing.T) {
	p := Partial{LightEffects: map[string]map[string]string{
		"ambiance":  {"chrono": "chenillard", "calm": "", "score": "couleur-equipe"},
		"animateur": {"chrono": "", "calm": "", "score": ""},
		"public":    {"score": "couleur-equipe"},
	}}
	if err := p.Validate(tw290aKnown); err != nil {
		t.Errorf("trois zones, trois moments, id d'effet ou chaîne vide : %v", err)
	}
}

func TestValidate223_InvalidPathsListsLightEffectPaths(t *testing.T) {
	bad := (Partial{LightEffects: tw223Eff("ambiance", "bidule", "x")}).InvalidPaths(tw290aKnown)
	if len(bad) != 1 || bad[0] != "light.effects.ambiance.bidule" {
		t.Errorf("chemin JSON fautif : %v", bad)
	}
	bad = (Partial{LightEffects: tw223Eff("salle", "chrono", "x")}).InvalidPaths(tw290aKnown)
	if len(bad) != 1 || bad[0] != "light.effects.salle.chrono" {
		t.Errorf("chemin JSON fautif (zone inconnue) : %v", bad)
	}
}

func TestValidate223_EditablePathsParity_ThreeZonesByThreeMoments(t *testing.T) {
	// parité avec le composant React : light.effects.<zone>.<moment>, 9 chemins éditables
	for _, z := range tw223Zones {
		for _, m := range tw223Moments {
			if bad := (Partial{LightEffects: tw223Eff(z, m, "")}).InvalidPaths(tw290aKnown); len(bad) != 0 {
				t.Errorf("light.effects.%s.%s éditable : %v", z, m, bad)
			}
		}
	}
}
