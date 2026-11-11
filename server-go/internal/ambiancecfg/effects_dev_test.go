package ambiancecfg

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLightEffects_ResolveSourcesJSONAndValidation_Dev(t *testing.T) {
	t.Parallel()
	g := Settings{CuesEnabled: map[string]bool{}, LightMode: LightAuto, LightEffects: DefaultLightEffects(), LightHostEffects: DefaultLightHostEffects()}
	round := Partial{LightEffects: map[string]string{"calm": "vague-lente"}, LightHostEffects: map[string]string{"score": "couleur-equipe"}}
	eff, src := Resolve(g, round)
	if eff.LightEffects["calm"] != "vague-lente" || eff.LightEffects["chrono"] != "pulsation" || eff.LightHostEffects["score"] != "couleur-equipe" {
		t.Fatalf("surcharge manche par moment : %+v", eff)
	}
	if src.LightEffects["calm"] != SourceRound || src.LightEffects["chrono"] != SourceGlobal || src.LightHostEffects["score"] != SourceRound {
		t.Fatalf("sources : %+v", src)
	}
	if g.LightEffects["calm"] != "" {
		t.Fatal("Resolve ne modifie jamais le global")
	}

	// Complete form always carries the three moments, never omitted.
	raw, _ := json.Marshal(Settings{CuesEnabled: map[string]bool{}, LightMode: LightAuto})
	for _, want := range []string{`"effects":{"calm":"","chrono":"pulsation","score":""}`, `"host_effects":{"calm":"","chrono":"","score":""}`} {
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
	} {
		if bad := ValidateRaw([]byte(body), nil); len(bad) == 0 {
			t.Errorf("%s : refus attendu", name)
		}
	}
	if bad := ValidateRaw([]byte(`{"light":{"effects":{"calm":"x"},"host_effects":{"score":""}}}`), nil); len(bad) != 0 {
		t.Fatalf("forme valide refusée : %v", bad)
	}
	// Catalogue check.
	p := Partial{LightEffects: map[string]string{"calm": "ok", "score": "ko", "chrono": ""}}
	bad := p.InvalidEffects(func(m, id string, host bool) bool { return id == "ok" })
	if len(bad) != 1 || bad[0] != "light.effects.score" {
		t.Fatalf("InvalidEffects : %v", bad)
	}
}
