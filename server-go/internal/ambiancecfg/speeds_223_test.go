package ambiancecfg

// Vitesse réglable à l'usage (contrat light-effects.md §6, ambiance-settings.md §10, révision 4) : un multiplicateur par
// zone et par moment (0.5, 0.75, 1, 1.5, 2 ; défaut 1), global ← manche, sources, formes JSON complète / Partial,
// validation. Forme : Settings.LightSpeeds map[zone]map[moment]float64, Partial idem (partielle),
// Sources.LightSpeeds map[zone]map[moment]Source ; JSON light.speeds.<zone>.<moment>. Aides tw223 de effects_223_test.go.

import (
	"encoding/json"
	"reflect"
	"testing"
)

func tw223Spd(zone, moment string, v float64) map[string]map[string]float64 {
	return map[string]map[string]float64{zone: {moment: v}}
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

func TestSpeeds223_Defaults_OneInEveryZoneAndMoment(t *testing.T) {
	d := DefaultLightSpeeds()
	for _, z := range tw223Zones {
		for _, m := range tw223Moments {
			if d[z][m] != 1 {
				t.Errorf("défaut ×1 : %s.%s = %v", z, m, d[z][m])
			}
		}
	}
}

func TestSpeeds223_Resolve_RoundOverridesOneZoneAndMomentOnly_SourcesFollow(t *testing.T) {
	got, src := Resolve(tw223Global(), Partial{LightSpeeds: tw223Spd("ambiance", "chrono", 2)})
	if got.LightSpeeds["ambiance"]["chrono"] != 2 || got.LightSpeeds["ambiance"]["calm"] != 1 || got.LightSpeeds["animateur"]["chrono"] != 1 {
		t.Errorf("seul ambiance.chrono est surchargé : %v", got.LightSpeeds)
	}
	if string(src.LightSpeeds["ambiance"]["chrono"]) != "round" || string(src.LightSpeeds["ambiance"]["calm"]) != "global" || string(src.LightSpeeds["public"]["chrono"]) != "global" {
		t.Errorf("sources : %v", src.LightSpeeds)
	}
}

func TestSpeeds223_Resolve_ZonesIndependent(t *testing.T) {
	round := Partial{LightSpeeds: map[string]map[string]float64{"animateur": {"score": 0.5}, "public": {"chrono": 1.5}}}
	got, src := Resolve(tw223Global(), round)
	if got.LightSpeeds["animateur"]["score"] != 0.5 || got.LightSpeeds["animateur"]["chrono"] != 1 ||
		got.LightSpeeds["public"]["chrono"] != 1.5 || got.LightSpeeds["public"]["score"] != 1 || got.LightSpeeds["ambiance"]["chrono"] != 1 {
		t.Errorf("chaque zone résout ses propres moments : %v", got.LightSpeeds)
	}
	if string(src.LightSpeeds["animateur"]["score"]) != "round" || string(src.LightSpeeds["animateur"]["chrono"]) != "global" ||
		string(src.LightSpeeds["public"]["chrono"]) != "round" {
		t.Errorf("sources par zone et moment : %v", src.LightSpeeds)
	}
}

func TestSpeeds223_SpeedIsIndependentOfTheEffectChoice(t *testing.T) {
	// la vitesse du moment vaut quel que soit l'effet, y compris « aucun effet »
	round := Partial{LightEffects: tw223Eff("ambiance", "chrono", ""), LightSpeeds: tw223Spd("ambiance", "chrono", 2)}
	got, _ := Resolve(tw223Global(), round)
	if got.LightEffects["ambiance"]["chrono"] != "" || got.LightSpeeds["ambiance"]["chrono"] != 2 {
		t.Errorf("effet et vitesse se résolvent séparément : %v / %v", got.LightEffects, got.LightSpeeds)
	}
}

func TestSpeeds223_ClonePartialMergeIsEmpty(t *testing.T) {
	s := tw223Global()
	c := s.Clone()
	c.LightSpeeds["animateur"]["calm"] = 2
	if s.LightSpeeds["animateur"]["calm"] != 1 {
		t.Error("Settings.Clone copie en profondeur les vitesses")
	}
	p := Partial{LightSpeeds: tw223Spd("public", "calm", 2)}
	pc := p.Clone()
	pc.LightSpeeds["public"]["calm"] = 0.5
	if p.LightSpeeds["public"]["calm"] != 2 {
		t.Error("Partial.Clone copie en profondeur")
	}
	if !(Partial{LightSpeeds: map[string]map[string]float64{}}).IsEmpty() {
		t.Error("des cartes de vitesses vides = Partial vide")
	}
	if (Partial{LightSpeeds: tw223Spd("ambiance", "calm", 1)}).IsEmpty() {
		t.Error("une vitesse surchargée à 1 n'est pas vide : 1 est une valeur, pas « hérité »")
	}
	m := Partial{LightSpeeds: map[string]map[string]float64{"ambiance": {"chrono": 2, "calm": 2}}}.Merge(
		Partial{LightSpeeds: map[string]map[string]float64{"ambiance": {"calm": 0.5}, "public": {"score": 1.5}}})
	if m.LightSpeeds["ambiance"]["chrono"] != 2 || m.LightSpeeds["ambiance"]["calm"] != 0.5 || m.LightSpeeds["public"]["score"] != 1.5 {
		t.Errorf("Merge clé par clé, le second l'emporte : %+v", m)
	}
}

func TestSpeeds223_JSON_CompleteFormAlwaysThreeZonesByThreeMoments(t *testing.T) {
	m := tw223JSON(t, tw223Global())
	for _, z := range tw223Zones {
		sp, _ := tw223Dig(m, "light", "speeds", z).(map[string]interface{})
		if len(sp) != 3 || sp["chrono"] != float64(1) {
			t.Fatalf("light.speeds.%s : trois moments toujours présents, ×1 : %v", z, sp)
		}
	}
	if _, present := tw223Dig(m, "light").(map[string]interface{})["host_speeds"]; present {
		t.Error("light.host_speeds n'existe plus")
	}
}

func TestSpeeds223_JSON_RoundTrip_AndPartialOnlyOverriddenKeys(t *testing.T) {
	s := tw223Global()
	s.LightSpeeds["ambiance"]["calm"] = 0.75
	s.LightSpeeds["public"]["score"] = 1.5
	raw, _ := json.Marshal(s)
	var back Settings
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.LightSpeeds, s.LightSpeeds) {
		t.Errorf("aller-retour infidèle : %+v", back)
	}
	praw, _ := json.Marshal(Partial{LightSpeeds: tw223Spd("animateur", "score", 0.5)})
	var got, want interface{}
	_ = json.Unmarshal(praw, &got)
	_ = json.Unmarshal([]byte(`{"light":{"speeds":{"animateur":{"score":0.5}}}}`), &want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Partial : seules les clés surchargées : %s", praw)
	}
	praw, _ = json.Marshal(Partial{LightSpeeds: map[string]map[string]float64{}})
	if string(praw) != "{}" {
		t.Errorf("cartes vides omises : %s", praw)
	}
}

func TestSpeeds223_JSON_OldFlatSpeeds_ReadAsAmbianceZone_HostSpeedsIgnored(t *testing.T) {
	var p Partial
	if err := json.Unmarshal([]byte(`{"light":{"speeds":{"chrono":2},"host_speeds":{"g1":{"calm":2}}}}`), &p); err != nil {
		t.Fatalf("lecture tolérante : %v", err)
	}
	if p.LightSpeeds["ambiance"]["chrono"] != 2 || len(p.LightSpeeds["animateur"]) != 0 {
		t.Errorf("à plat ⇒ zone ambiance ; host_speeds ignoré : %+v", p.LightSpeeds)
	}
}

func TestSpeeds223_JSON_SourcesForm(t *testing.T) {
	_, src := Resolve(tw223Global(), Partial{LightSpeeds: map[string]map[string]float64{"ambiance": {"calm": 2}, "animateur": {"chrono": 2}}})
	m := tw223JSON(t, src)
	if tw223Dig(m, "light", "speeds", "ambiance", "calm") != "round" || tw223Dig(m, "light", "speeds", "ambiance", "chrono") != "global" ||
		tw223Dig(m, "light", "speeds", "animateur", "chrono") != "round" || tw223Dig(m, "light", "speeds", "public", "chrono") != "global" {
		t.Errorf("Sources.light.speeds.<zone>.<moment> ∈ global|round : %v", m)
	}
}

func TestSpeeds223_Validate_OnlyTheFiveValues_PathsNamed(t *testing.T) {
	for _, v := range []float64{0, 0.25, 3, 1.25, -1} {
		for _, z := range tw223Zones {
			p := Partial{LightSpeeds: tw223Spd(z, "chrono", v)}
			if err := p.Validate(tw290aKnown); err == nil {
				t.Errorf("vitesse %v refusée (%s)", v, z)
			}
			if bad := p.InvalidPaths(tw290aKnown); len(bad) != 1 || bad[0] != "light.speeds."+z+".chrono" {
				t.Errorf("vitesse %v : chemin JSON fautif light.speeds.%s.chrono : %v", v, z, bad)
			}
		}
	}
	for _, v := range []float64{0.5, 0.75, 1, 1.5, 2} {
		p := Partial{LightSpeeds: map[string]map[string]float64{"ambiance": {"calm": v}, "animateur": {"score": v}, "public": {"chrono": v}}}
		if err := p.Validate(tw290aKnown); err != nil {
			t.Errorf("vitesse %v acceptée : %v", v, err)
		}
	}
}

func TestSpeeds223_Validate_UnknownMomentOrZoneRefused(t *testing.T) {
	if err := (Partial{LightSpeeds: tw223Spd("ambiance", "bidule", 1)}).Validate(tw290aKnown); err == nil {
		t.Error("moment inconnu refusé")
	}
	if err := (Partial{LightSpeeds: tw223Spd("salle", "chrono", 1)}).Validate(tw290aKnown); err == nil {
		t.Error("zone inconnue refusée")
	}
}

func TestSpeeds223_EditablePathsParity_ThreeZonesByThreeMoments(t *testing.T) {
	for _, z := range tw223Zones {
		for _, m := range tw223Moments {
			if bad := (Partial{LightSpeeds: tw223Spd(z, m, 1)}).InvalidPaths(tw290aKnown); len(bad) != 0 {
				t.Errorf("light.speeds.%s.%s éditable : %v", z, m, bad)
			}
		}
	}
}

func TestSpeeds223_OldPartialWithoutSpeeds_ReadsAsInheritedEverywhere(t *testing.T) {
	var p Partial
	if err := json.Unmarshal([]byte(`{"light":{"mode":"ON","effects":{"ambiance":{"chrono":"chenillard"}}}}`), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.LightSpeeds) != 0 {
		t.Errorf("absent de rounds.json ⇒ vitesse 1 partout (rien surchargé) : %+v", p)
	}
	got, _ := Resolve(tw223Global(), p)
	if got.LightSpeeds["ambiance"]["chrono"] != 1 {
		t.Errorf("héritée : ×1 : %v", got.LightSpeeds)
	}
}
