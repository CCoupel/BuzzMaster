package ambiancecfg

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLightEffects_ResolveSourcesJSONAndValidation_Dev(t *testing.T) {
	t.Parallel()
	g := Settings{CuesEnabled: map[string]bool{}, LightMode: LightAuto, LightEffects: DefaultLightEffects(), LightHostEffects: map[string]map[string]string{"g1": DefaultHostMoments(), "g2": DefaultHostMoments()}}
	round := Partial{LightEffects: map[string]string{"calm": "vague-lente"}, LightHostEffects: map[string]map[string]string{"g1": {"score": "couleur-equipe"}, "ghost": {"calm": "x"}}}
	eff, src := Resolve(g, round)
	if eff.LightEffects["calm"] != "vague-lente" || eff.LightEffects["chrono"] != "pulsation" || eff.LightHostEffects["g1"]["score"] != "couleur-equipe" || eff.LightHostEffects["g2"]["score"] != "" {
		t.Fatalf("surcharge manche par moment : %+v", eff)
	}
	if src.LightEffects["calm"] != SourceRound || src.LightEffects["chrono"] != SourceGlobal || src.LightHostEffects["g1"]["score"] != SourceRound || src.LightHostEffects["g2"]["score"] != SourceGlobal {
		t.Fatalf("sources : %+v", src)
	}
	if g.LightEffects["calm"] != "" {
		t.Fatal("Resolve ne modifie jamais le global")
	}

	// Complete form always carries the three moments, never omitted.
	raw, _ := json.Marshal(Settings{CuesEnabled: map[string]bool{}, LightMode: LightAuto, LightHostEffects: map[string]map[string]string{"g1": {"score": "x"}}})
	for _, want := range []string{`"effects":{"calm":"","chrono":"pulsation","score":""}`, `"host_effects":{"g1":{"calm":"","chrono":"","score":"x"}}`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("%s absent de %s", want, raw)
		}
	}
	// Partial: only the overridden moments, round trip.
	praw, _ := json.Marshal(round)
	var back Partial
	if err := json.Unmarshal(praw, &back); err != nil || back.LightEffects["calm"] != "vague-lente" || len(back.LightEffects) != 1 {
		t.Fatalf("aller-retour Partial : %v %+v (%s)", err, back, praw)
	}

	// Strict validation of the raw JSON.
	for name, body := range map[string]string{
		"moment inconnu":   `{"light":{"effects":{"nuit":"x"}}}`,
		"valeur non texte": `{"light":{"effects":{"calm":3}}}`,
		"clé inconnue":     `{"light":{"effect":{}}}`,
		"hôte moment":      `{"light":{"host_effects":{"g1":{"nuit":"x"}}}}`,
	} {
		if bad := ValidateRaw([]byte(body), nil); len(bad) == 0 {
			t.Errorf("%s : refus attendu", name)
		}
	}
	if bad := ValidateRaw([]byte(`{"light":{"effects":{"calm":"x"},"host_effects":{"g1":{"score":""},"inconnu":{"calm":"y"}}}}`), nil); len(bad) != 0 {
		t.Fatalf("forme valide refusée (id inconnu toléré) : %v", bad)
	}
	// The old flat form is read as nothing, never an error.
	var legacy Partial
	if err := json.Unmarshal([]byte(`{"light":{"host_effects":{"chrono":"","calm":"x","score":""}}}`), &legacy); err != nil || len(legacy.LightHostEffects) != 0 {
		t.Fatalf("ancienne forme plate ignorée : %v %+v", err, legacy)
	}
	// Catalogue check.
	p := Partial{LightEffects: map[string]string{"calm": "ok", "score": "ko", "chrono": ""}}
	bad := p.InvalidEffects(func(m, id string, host bool) bool { return id == "ok" })
	if len(bad) != 1 || bad[0] != "light.effects.score" {
		t.Fatalf("InvalidEffects : %v", bad)
	}
}

func TestLightSpeeds_ResolveSourcesJSONAndValidation_Dev(t *testing.T) {
	t.Parallel()
	g := Settings{CuesEnabled: map[string]bool{}, LightMode: LightAuto, LightEffects: DefaultLightEffects(),
		LightHostEffects: map[string]map[string]string{}, LightSpeeds: DefaultLightSpeeds(),
		LightHostSpeeds: map[string]map[string]float64{"g1": DefaultLightSpeeds()}}
	round := Partial{LightSpeeds: map[string]float64{"chrono": 2}, LightHostSpeeds: map[string]map[string]float64{"g1": {"calm": 0.5}, "ghost": {"calm": 2}}}
	eff, src := Resolve(g, round)
	if eff.LightSpeeds["chrono"] != 2 || eff.LightSpeeds["calm"] != 1 || eff.LightHostSpeeds["g1"]["calm"] != 0.5 {
		t.Fatalf("surcharge de manche par moment : %+v", eff)
	}
	if _, ok := eff.LightHostSpeeds["ghost"]; ok {
		t.Fatal("un id de groupe inconnu est ignoré")
	}
	if src.LightSpeeds["chrono"] != SourceRound || src.LightSpeeds["calm"] != SourceGlobal || src.LightHostSpeeds["g1"]["calm"] != SourceRound {
		t.Fatalf("sources : %+v", src)
	}
	raw, _ := json.Marshal(Settings{CuesEnabled: map[string]bool{}, LightMode: LightAuto, LightHostSpeeds: map[string]map[string]float64{"g1": {"calm": 1.5}}})
	for _, want := range []string{`"speeds":{"calm":1,"chrono":1,"score":1}`, `"host_speeds":{"g1":{"calm":1.5,"chrono":1,"score":1}}`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("%s absent de %s", want, raw)
		}
	}
	for name, body := range map[string]string{
		"valeur hors liste": `{"light":{"speeds":{"chrono":3}}}`,
		"pas un nombre":     `{"light":{"speeds":{"chrono":"2"}}}`,
		"moment inconnu":    `{"light":{"speeds":{"nuit":1}}}`,
		"hôte hors liste":   `{"light":{"host_speeds":{"g1":{"calm":0.6}}}}`,
		"hôte pas un objet": `{"light":{"host_speeds":{"g1":2}}}`,
	} {
		if bad := ValidateRaw([]byte(body), nil); len(bad) == 0 {
			t.Errorf("%s : refus attendu", name)
		}
	}
	for _, v := range []float64{0.5, 0.75, 1, 1.5, 2} {
		if !ValidSpeed(v) {
			t.Errorf("%v doit être permise", v)
		}
	}
	if bad := ValidateRaw([]byte(`{"light":{"speeds":{"calm":0.75},"host_speeds":{"inconnu":{"score":2}}}}`), nil); len(bad) != 0 {
		t.Fatalf("forme valide refusée : %v", bad)
	}
	if bad := (Partial{LightSpeeds: map[string]float64{"calm": 9}}).InvalidPaths(nil); len(bad) != 1 {
		t.Fatalf("InvalidPaths : %v", bad)
	}
}
