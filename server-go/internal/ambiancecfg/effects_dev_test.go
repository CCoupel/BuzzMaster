package ambiancecfg

import (
	"encoding/json"
	"strings"
	"testing"
)

// Effects and speeds by zone and moment (light-effects.md §6, revision 4).

func zoneGlobal() Settings {
	return Settings{CuesEnabled: map[string]bool{}, LightMode: LightAuto, LightEffects: DefaultLightEffects(), LightSpeeds: DefaultLightSpeeds()}
}

func TestLightZones_ResolveSourcesAndDefaults_Dev(t *testing.T) {
	t.Parallel()
	g := zoneGlobal()
	if g.LightEffects[ZoneAmbiance][MomentChrono] != PulsationID || g.LightEffects[ZoneAnimateur][MomentChrono] != "" || g.LightEffects[ZonePublic][MomentScore] != "" {
		t.Fatalf("défauts : pulsation au chrono de la salle seulement : %+v", g.LightEffects)
	}
	round := Partial{
		LightEffects: map[string]map[string]string{ZoneAnimateur: {"score": "couleur-equipe"}, "zone-inconnue": {"calm": "x"}},
		LightSpeeds:  map[string]map[string]float64{ZonePublic: {"calm": 0.5}},
	}
	eff, src := Resolve(g, round)
	if eff.LightEffects[ZoneAnimateur]["score"] != "couleur-equipe" || eff.LightEffects[ZoneAmbiance]["chrono"] != PulsationID || eff.LightSpeeds[ZonePublic]["calm"] != 0.5 {
		t.Fatalf("surcharge de manche par zone et par moment : %+v", eff)
	}
	if _, ok := eff.LightEffects["zone-inconnue"]; ok {
		t.Fatal("une zone inconnue est ignorée à la résolution")
	}
	if src.LightEffects[ZoneAnimateur]["score"] != SourceRound || src.LightEffects[ZoneAnimateur]["calm"] != SourceGlobal || src.LightSpeeds[ZonePublic]["calm"] != SourceRound {
		t.Fatalf("sources : %+v", src)
	}
	if g.LightEffects[ZoneAnimateur]["score"] != "" {
		t.Fatal("Resolve ne modifie jamais le global")
	}
}

func TestLightZones_JSONFormsAndLegacyTolerance_Dev(t *testing.T) {
	t.Parallel()
	// Complete form: three zones × three moments, speeds included, even for an empty Settings.
	raw, _ := json.Marshal(Settings{CuesEnabled: map[string]bool{}, LightMode: LightAuto})
	for _, want := range []string{
		`"effects":{"ambiance":{"calm":"","chrono":"pulsation","score":""},"animateur":{"calm":"","chrono":"","score":""},"public":{"calm":"","chrono":"","score":""}}`,
		`"speeds":{"ambiance":{"calm":1,"chrono":1,"score":1},"animateur":{"calm":1,"chrono":1,"score":1},"public":{"calm":1,"chrono":1,"score":1}}`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("%s absent de %s", want, raw)
		}
	}
	// Partial round trip: only what is overridden.
	p := Partial{LightEffects: map[string]map[string]string{ZonePublic: {"chrono": "chenillard"}}}
	praw, _ := json.Marshal(p)
	var back Partial
	if err := json.Unmarshal(praw, &back); err != nil || back.LightEffects[ZonePublic]["chrono"] != "chenillard" || len(back.LightEffects) != 1 {
		t.Fatalf("aller-retour Partial : %v %+v (%s)", err, back, praw)
	}
	// A rounds.json of a QUALIF build: flat tables are the ambiance zone, host_* keys are ignored.
	var legacy Partial
	if err := json.Unmarshal([]byte(`{"light":{"effects":{"calm":"vague-lente"},"speeds":{"chrono":2},"host_effects":{"g1":{"calm":"x"}},"host_speeds":{"g1":{"calm":2}}}}`), &legacy); err != nil {
		t.Fatalf("lecture tolérante : %v", err)
	}
	if legacy.LightEffects[ZoneAmbiance]["calm"] != "vague-lente" || legacy.LightSpeeds[ZoneAmbiance]["chrono"] != 2 || len(legacy.LightEffects) != 1 {
		t.Fatalf("à plat = zone ambiance, host_* ignorés : %+v", legacy)
	}
}

func TestLightZones_ValidationAndCatalogueCheck_Dev(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"zone inconnue":         `{"light":{"effects":{"scène":{"calm":"x"}}}}`,
		"moment inconnu":        `{"light":{"effects":{"public":{"nuit":"x"}}}}`,
		"valeur non texte":      `{"light":{"effects":{"public":{"calm":3}}}}`,
		"à plat refusé":         `{"light":{"effects":{"calm":"x"}}}`,
		"vitesse hors liste":    `{"light":{"speeds":{"public":{"calm":3}}}}`,
		"vitesse pas un nombre": `{"light":{"speeds":{"public":{"calm":"2"}}}}`,
		"à plat vitesse":        `{"light":{"speeds":{"calm":2}}}`,
		"host_effects refusé":   `{"light":{"host_effects":{"g1":{"calm":"x"}}}}`,
		"host_speeds refusé":    `{"light":{"host_speeds":{"g1":{"calm":2}}}}`,
	} {
		if bad := ValidateRaw([]byte(body), nil); len(bad) == 0 {
			t.Errorf("%s : refus attendu", name)
		}
	}
	if bad := ValidateRaw([]byte(`{"light":{"effects":{"animateur":{"score":"couleur-equipe"}},"speeds":{"public":{"calm":0.75}}}}`), nil); len(bad) != 0 {
		t.Fatalf("forme valide refusée : %v", bad)
	}
	for _, v := range []float64{0.5, 0.75, 1, 1.5, 2} {
		if !ValidSpeed(v) {
			t.Errorf("%v doit être permise", v)
		}
	}
	p := Partial{LightEffects: map[string]map[string]string{ZoneAmbiance: {"calm": "ok", "score": "ko", "chrono": ""}}}
	bad := p.InvalidEffects(func(zone, m, id string) bool { return id == "ok" })
	if len(bad) != 1 || bad[0] != "light.effects.ambiance.score" {
		t.Fatalf("InvalidEffects : %v", bad)
	}
	if bad := (Partial{LightSpeeds: map[string]map[string]float64{ZonePublic: {"calm": 9}}}).InvalidPaths(nil); len(bad) != 1 {
		t.Fatalf("InvalidPaths : %v", bad)
	}
}
