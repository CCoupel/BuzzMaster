package ambiancecfg

// Vitesse réglable à l'usage (contrat light-effects.md §6, ambiance-settings.md §10, tâche R13) : un multiplicateur par
// moment (0.5, 0.75, 1, 1.5, 2 ; défaut 1) pour la salle et par groupe animateur, global ← manche, sources, formes JSON
// complète / Partial, validation. Forme : Settings.LightSpeeds map[moment]float64, LightHostSpeeds map[id]map[moment]float64,
// Partial idem (partielle), Sources.LightSpeeds / LightHostSpeeds map[...]Source. Aides tw223 de effects_223_test.go.

import (
	"encoding/json"
	"reflect"
	"testing"
)

func tw223SpeedGlobal() Settings {
	g := tw223Global()
	g.LightSpeeds = map[string]float64{"chrono": 1, "calm": 1, "score": 1}
	g.LightHostSpeeds = map[string]map[string]float64{
		"h1": {"chrono": 1, "calm": 1, "score": 1},
		"h2": {"chrono": 1, "calm": 1, "score": 1},
	}
	return g
}

func TestSpeeds223_AllowedValues_ExactlyFive(t *testing.T) {
	for _, v := range []float64{0.5, 0.75, 1, 1.5, 2} {
		if !ValidSpeed(v) {
			t.Errorf("%v est une vitesse permise", v)
		}
	}
	for _, v := range []float64{0, -1, 0.25, 0.6, 1.25, 1.75, 2.5, 3, 10} {
		if ValidSpeed(v) {
			t.Errorf("%v n'est pas une vitesse permise", v)
		}
	}
}

func TestSpeeds223_Defaults_OneAtEveryMoment(t *testing.T) {
	d := DefaultLightSpeeds()
	if len(d) != 3 || d["chrono"] != 1 || d["calm"] != 1 || d["score"] != 1 {
		t.Errorf("défaut ×1 aux trois moments : %v", d)
	}
}

func TestSpeeds223_Resolve_RoundOverridesOneMomentOnly_SourcesFollow(t *testing.T) {
	round := Partial{LightSpeeds: map[string]float64{"chrono": 2}}
	got, src := Resolve(tw223SpeedGlobal(), round)
	if got.LightSpeeds["chrono"] != 2 || got.LightSpeeds["calm"] != 1 || got.LightSpeeds["score"] != 1 {
		t.Errorf("seul chrono est surchargé : %v", got.LightSpeeds)
	}
	if string(src.LightSpeeds["chrono"]) != "round" || string(src.LightSpeeds["calm"]) != "global" {
		t.Errorf("sources : %v", src.LightSpeeds)
	}
	if !reflect.DeepEqual(got.LightHostSpeeds, tw223SpeedGlobal().LightHostSpeeds) {
		t.Errorf("les vitesses des groupes animateur restent héritées : %v", got.LightHostSpeeds)
	}
}

func TestSpeeds223_Resolve_HostSpeedsPerGroupIdAndMoment_Independent(t *testing.T) {
	round := Partial{LightHostSpeeds: map[string]map[string]float64{"h1": {"score": 0.5}, "h2": {"chrono": 1.5}}}
	got, src := Resolve(tw223SpeedGlobal(), round)
	if got.LightHostSpeeds["h1"]["score"] != 0.5 || got.LightHostSpeeds["h1"]["chrono"] != 1 ||
		got.LightHostSpeeds["h2"]["chrono"] != 1.5 || got.LightHostSpeeds["h2"]["score"] != 1 {
		t.Errorf("chaque groupe résout ses propres moments : %v", got.LightHostSpeeds)
	}
	if string(src.LightHostSpeeds["h1"]["score"]) != "round" || string(src.LightHostSpeeds["h1"]["chrono"]) != "global" ||
		string(src.LightHostSpeeds["h2"]["chrono"]) != "round" {
		t.Errorf("sources par groupe et moment : %v", src.LightHostSpeeds)
	}
	if got.LightSpeeds["chrono"] != 1 {
		t.Errorf("la vitesse de la salle n'est pas touchée : %v", got.LightSpeeds)
	}
}

func TestSpeeds223_Resolve_UnknownHostIdIgnoredWithoutError(t *testing.T) {
	round := Partial{LightHostSpeeds: map[string]map[string]float64{"disparu": {"chrono": 2}, "h1": {"calm": 2}}}
	got, _ := Resolve(tw223SpeedGlobal(), round)
	if _, ok := got.LightHostSpeeds["disparu"]; ok {
		t.Errorf("un id absent du global (groupe supprimé) est ignoré : %v", got.LightHostSpeeds)
	}
	if got.LightHostSpeeds["h1"]["calm"] != 2 {
		t.Errorf("l'id connu est résolu : %v", got.LightHostSpeeds)
	}
}

func TestSpeeds223_SpeedIsIndependentOfTheEffectChoice(t *testing.T) {
	// la vitesse du moment vaut quel que soit l'effet, y compris « aucun effet »
	round := Partial{LightEffects: map[string]string{"chrono": ""}, LightSpeeds: map[string]float64{"chrono": 2}}
	got, _ := Resolve(tw223SpeedGlobal(), round)
	if got.LightEffects["chrono"] != "" || got.LightSpeeds["chrono"] != 2 {
		t.Errorf("effet et vitesse se résolvent séparément : %v / %v", got.LightEffects, got.LightSpeeds)
	}
}

func TestSpeeds223_ClonePartialMergeIsEmpty(t *testing.T) {
	s := tw223SpeedGlobal()
	c := s.Clone()
	c.LightSpeeds["chrono"] = 2
	c.LightHostSpeeds["h1"]["calm"] = 2
	if s.LightSpeeds["chrono"] != 1 || s.LightHostSpeeds["h1"]["calm"] != 1 {
		t.Error("Settings.Clone copie en profondeur les vitesses")
	}
	p := Partial{LightSpeeds: map[string]float64{"chrono": 2}, LightHostSpeeds: map[string]map[string]float64{"h1": {"calm": 2}}}
	pc := p.Clone()
	pc.LightSpeeds["chrono"] = 0.5
	pc.LightHostSpeeds["h1"]["calm"] = 0.5
	if p.LightSpeeds["chrono"] != 2 || p.LightHostSpeeds["h1"]["calm"] != 2 {
		t.Error("Partial.Clone copie en profondeur")
	}
	if !(Partial{LightSpeeds: map[string]float64{}, LightHostSpeeds: map[string]map[string]float64{}}).IsEmpty() {
		t.Error("des cartes de vitesses vides = Partial vide")
	}
	if (Partial{LightSpeeds: map[string]float64{"calm": 1}}).IsEmpty() {
		t.Error("une vitesse surchargée à 1 n'est pas vide : 1 est une valeur, pas « hérité »")
	}
	m := Partial{LightSpeeds: map[string]float64{"chrono": 2, "calm": 2}}.Merge(Partial{LightSpeeds: map[string]float64{"calm": 0.5}, LightHostSpeeds: map[string]map[string]float64{"h2": {"score": 1.5}}})
	if m.LightSpeeds["chrono"] != 2 || m.LightSpeeds["calm"] != 0.5 || m.LightHostSpeeds["h2"]["score"] != 1.5 {
		t.Errorf("Merge clé par clé, le second l'emporte : %+v", m)
	}
}

func TestSpeeds223_JSON_CompleteFormAlwaysThreeMoments_HostSpeedsOnePerGroup(t *testing.T) {
	m := tw223JSON(t, tw223SpeedGlobal())
	sp, _ := tw223Dig(m, "light", "speeds").(map[string]interface{})
	if len(sp) != 3 || sp["chrono"] != float64(1) {
		t.Fatalf("light.speeds : trois moments toujours présents, ×1 : %v", sp)
	}
	hs, _ := tw223Dig(m, "light", "host_speeds").(map[string]interface{})
	if len(hs) != 2 {
		t.Fatalf("light.host_speeds : une entrée par groupe animateur : %v", hs)
	}
	for _, id := range []string{"h1", "h2"} {
		if g, _ := hs[id].(map[string]interface{}); len(g) != 3 {
			t.Errorf("light.host_speeds.%s : trois moments : %v", id, g)
		}
	}
	g := tw223SpeedGlobal()
	g.LightHostSpeeds = map[string]map[string]float64{}
	if hs, ok := tw223Dig(tw223JSON(t, g), "light", "host_speeds").(map[string]interface{}); !ok || len(hs) != 0 {
		t.Errorf("aucun groupe animateur ⇒ {} (jamais absent ni null) : %v", tw223Dig(tw223JSON(t, g), "light", "host_speeds"))
	}
}

func TestSpeeds223_JSON_RoundTrip_AndPartialOnlyOverriddenKeys(t *testing.T) {
	s := tw223SpeedGlobal()
	s.LightSpeeds["calm"] = 0.75
	s.LightHostSpeeds["h2"]["score"] = 1.5
	raw, _ := json.Marshal(s)
	var back Settings
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.LightSpeeds, s.LightSpeeds) || !reflect.DeepEqual(back.LightHostSpeeds, s.LightHostSpeeds) {
		t.Errorf("aller-retour infidèle : %+v", back)
	}
	praw, _ := json.Marshal(Partial{LightSpeeds: map[string]float64{"chrono": 2}, LightHostSpeeds: map[string]map[string]float64{"h1": {"score": 0.5}}})
	var got, want interface{}
	_ = json.Unmarshal(praw, &got)
	_ = json.Unmarshal([]byte(`{"light":{"speeds":{"chrono":2},"host_speeds":{"h1":{"score":0.5}}}}`), &want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Partial : seules les clés surchargées : %s", praw)
	}
	praw, _ = json.Marshal(Partial{LightSpeeds: map[string]float64{}, LightHostSpeeds: map[string]map[string]float64{}})
	if string(praw) != "{}" {
		t.Errorf("cartes vides omises : %s", praw)
	}
}

func TestSpeeds223_JSON_SourcesForm(t *testing.T) {
	_, src := Resolve(tw223SpeedGlobal(), Partial{LightSpeeds: map[string]float64{"calm": 2}, LightHostSpeeds: map[string]map[string]float64{"h1": {"chrono": 2}}})
	m := tw223JSON(t, src)
	if tw223Dig(m, "light", "speeds", "calm") != "round" || tw223Dig(m, "light", "speeds", "chrono") != "global" ||
		tw223Dig(m, "light", "host_speeds", "h1", "chrono") != "round" || tw223Dig(m, "light", "host_speeds", "h2", "chrono") != "global" {
		t.Errorf("Sources.light.speeds.<moment> et host_speeds.<id>.<moment> ∈ global|round : %v", m)
	}
}

func TestSpeeds223_Validate_OnlyTheFiveValues_PathsNamed(t *testing.T) {
	for _, v := range []float64{0, 0.25, 3, 1.25, -1} {
		p := Partial{LightSpeeds: map[string]float64{"chrono": v}}
		if err := p.Validate(tw290aKnown); err == nil {
			t.Errorf("vitesse %v refusée", v)
		}
		if bad := p.InvalidPaths(tw290aKnown); len(bad) != 1 || bad[0] != "light.speeds.chrono" {
			t.Errorf("vitesse %v : chemin JSON fautif light.speeds.chrono : %v", v, bad)
		}
		ph := Partial{LightHostSpeeds: map[string]map[string]float64{"h1": {"score": v}}}
		if bad := ph.InvalidPaths(tw290aKnown); len(bad) != 1 || bad[0] != "light.host_speeds.h1.score" {
			t.Errorf("vitesse %v (animateur) : chemin fautif par id : %v", v, bad)
		}
	}
	for _, v := range []float64{0.5, 0.75, 1, 1.5, 2} {
		p := Partial{LightSpeeds: map[string]float64{"calm": v}, LightHostSpeeds: map[string]map[string]float64{"h1": {"score": v}}}
		if err := p.Validate(tw290aKnown); err != nil {
			t.Errorf("vitesse %v acceptée : %v", v, err)
		}
	}
}

func TestSpeeds223_Validate_UnknownMomentRefused(t *testing.T) {
	if err := (Partial{LightSpeeds: map[string]float64{"bidule": 1}}).Validate(tw290aKnown); err == nil {
		t.Error("moment inconnu refusé")
	}
	if err := (Partial{LightHostSpeeds: map[string]map[string]float64{"h1": {"bidule": 1}}}).Validate(tw290aKnown); err == nil {
		t.Error("moment inconnu refusé (animateur)")
	}
}

func TestSpeeds223_EditablePathsParity_RoomAndHostPerGroup(t *testing.T) {
	// parité avec le composant React : light.speeds.<m> et light.host_speeds.<id>.<m>
	for _, m := range tw223Moments {
		if bad := (Partial{LightSpeeds: map[string]float64{m: 1}}).InvalidPaths(tw290aKnown); len(bad) != 0 {
			t.Errorf("light.speeds.%s éditable : %v", m, bad)
		}
		if bad := (Partial{LightHostSpeeds: map[string]map[string]float64{"g5d7e4": {m: 1}}}).InvalidPaths(tw290aKnown); len(bad) != 0 {
			t.Errorf("light.host_speeds.<id>.%s éditable : %v", m, bad)
		}
	}
}

func TestSpeeds223_OldPartialWithoutSpeeds_ReadsAsInheritedEverywhere(t *testing.T) {
	var p Partial
	if err := json.Unmarshal([]byte(`{"light":{"mode":"ON","effects":{"chrono":"chenillard"}}}`), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.LightSpeeds) != 0 || len(p.LightHostSpeeds) != 0 {
		t.Errorf("absent de rounds.json ⇒ vitesse 1 partout (rien surchargé) : %+v", p)
	}
	got, _ := Resolve(tw223SpeedGlobal(), p)
	if got.LightSpeeds["chrono"] != 1 {
		t.Errorf("héritée : ×1 : %v", got.LightSpeeds)
	}
}
