// #290 (v11.6.0, LOT-4) — modèle mutualisé des réglages d'ambiance : Resolve (champ par champ, dernière couche
// définie gagne), Sources, formes JSON (§3), Validate (§3.2).
// Source normative : contracts/ambiance-settings.md §3–§5.1 (API figée : Settings, Partial, Source, Sources,
// Resolve(global, layers...), Partial.Validate(known)). Écrit depuis le contrat avant le code (rouge : le paquet
// n'existe pas encore). Le paquet ne dépend ni de config, ni de game, ni d'audio : testé seul. Préfixe tw290a.
package ambiancecfg

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

var tw290aCues = []string{
	"depart", "temps-ecoule", "gagne", "perdu", "reveal", "entracte-debut", "entracte-fin",
	"qcm-indice", "memory-carte", "memory-paire", "memotion-selection", "memotion-fin-carte",
	"rafale-bonne", "rafale-mauvaise", "rafale-fin-lot",
}

func tw290aGlobal() Settings {
	cues := map[string]bool{}
	for _, c := range tw290aCues {
		cues[c] = true
	}
	return Settings{SoundMuted: false, CountdownEnabled: true, CountdownLoop: true, CuesEnabled: cues, LightMode: LightMode("AUTO"),
		LightEffects: DefaultLightEffects(), LightHostEffects: map[string]map[string]string{},
		LightSpeeds: DefaultLightSpeeds(), LightHostSpeeds: map[string]map[string]float64{}} // v11.7.0 : forme complète
}

func tw290aB(v bool) *bool { return &v }
func tw290aM(v string) *LightMode {
	m := LightMode(v)
	return &m
}

func tw290aSrc(s Source) string { return string(s) }

// ---------------------------------------------------------------------------
// Resolve — pure, champ par champ, dernière couche définie gagne.
// ---------------------------------------------------------------------------

func TestResolve290_NoLayers_EqualsGlobal_AllSourcesGlobal(t *testing.T) {
	g := tw290aGlobal()
	got, src := Resolve(g)
	if !reflect.DeepEqual(got, g) {
		t.Errorf("sans couche, le résultat doit égaler le global : %+v", got)
	}
	for name, s := range map[string]Source{
		"muted": src.SoundMuted, "countdown.enabled": src.CountdownEnabled, "countdown.loop": src.CountdownLoop, "light.mode": src.LightMode,
	} {
		if tw290aSrc(s) != "global" {
			t.Errorf("source de %s attendue \"global\", got %q", name, s)
		}
	}
	for _, c := range tw290aCues {
		if tw290aSrc(src.CuesEnabled[c]) != "global" {
			t.Errorf("source de la cue %s attendue \"global\", got %q", c, src.CuesEnabled[c])
		}
	}
}

func TestResolve290_EmptyPartials_EqualGlobal(t *testing.T) {
	g := tw290aGlobal()
	got, src := Resolve(g, Partial{}, Partial{})
	if !reflect.DeepEqual(got, g) || tw290aSrc(src.SoundMuted) != "global" {
		t.Errorf("deux couches vides ⇒ global inchangé, got %+v / %q", got, src.SoundMuted)
	}
}

func TestResolve290_RoundLayer_OverridesOnlyItsFields(t *testing.T) {
	g := tw290aGlobal()
	round := Partial{SoundMuted: tw290aB(true), LightMode: tw290aM("ON")}
	got, src := Resolve(g, round)
	if !got.SoundMuted || got.LightMode != LightMode("ON") {
		t.Errorf("champs surchargés non appliqués : %+v", got)
	}
	if !got.CountdownEnabled || !got.CountdownLoop {
		t.Errorf("champs non surchargés = global : %+v", got)
	}
	if tw290aSrc(src.SoundMuted) != "round" || tw290aSrc(src.LightMode) != "round" {
		t.Errorf("sources \"round\" attendues : muted=%q light=%q", src.SoundMuted, src.LightMode)
	}
	if tw290aSrc(src.CountdownEnabled) != "global" || tw290aSrc(src.CountdownLoop) != "global" {
		t.Errorf("sources \"global\" attendues pour le décompte, got %q / %q", src.CountdownEnabled, src.CountdownLoop)
	}
}

// Une surcharge à FALSE est une surcharge (pointeur défini), pas « absent ».
func TestResolve290_FalseIsAnOverride_NotInherited(t *testing.T) {
	g := tw290aGlobal() // CountdownEnabled = true
	got, src := Resolve(g, Partial{CountdownEnabled: tw290aB(false)})
	if got.CountdownEnabled {
		t.Error("CountdownEnabled=false (surcharge) doit gagner sur le global true")
	}
	if tw290aSrc(src.CountdownEnabled) != "round" {
		t.Errorf("source \"round\" attendue, got %q", src.CountdownEnabled)
	}
}

func TestResolve290_DirectLayer_BeatsRoundLayer_FieldByField(t *testing.T) {
	g := tw290aGlobal()
	round := Partial{SoundMuted: tw290aB(true), CountdownLoop: tw290aB(false), LightMode: tw290aM("ON")}
	direct := Partial{SoundMuted: tw290aB(false)} // un geste de régie rétablit le son, sans toucher au reste
	got, src := Resolve(g, round, direct)
	if got.SoundMuted {
		t.Error("la couche directe (muted=false) doit gagner sur la manche (muted=true)")
	}
	if tw290aSrc(src.SoundMuted) != "direct" {
		t.Errorf("source \"direct\" attendue pour muted, got %q", src.SoundMuted)
	}
	if got.CountdownLoop || tw290aSrc(src.CountdownLoop) != "round" {
		t.Errorf("la boucle reste imposée par la manche : loop=%v source=%q", got.CountdownLoop, src.CountdownLoop)
	}
	if got.LightMode != LightMode("ON") || tw290aSrc(src.LightMode) != "round" {
		t.Errorf("le mode lumière reste imposé par la manche : %q / %q", got.LightMode, src.LightMode)
	}
}

func TestResolve290_LayerOrder_LastDefinedWins(t *testing.T) {
	g := tw290aGlobal()
	a := Partial{LightMode: tw290aM("ON")}
	b := Partial{LightMode: tw290aM("OFF")}
	got, _ := Resolve(g, a, b)
	if got.LightMode != LightMode("OFF") {
		t.Errorf("la dernière couche qui définit le champ gagne (OFF), got %q", got.LightMode)
	}
	got, _ = Resolve(g, b, a)
	if got.LightMode != LightMode("ON") {
		t.Errorf("ordre inversé ⇒ ON, got %q", got.LightMode)
	}
}

func TestResolve290_Cues_PartialMap_PerCue(t *testing.T) {
	g := tw290aGlobal()
	g.CuesEnabled["reveal"] = false // désactivée au global
	round := Partial{CuesEnabled: map[string]bool{"qcm-indice": false, "reveal": true}}
	direct := Partial{CuesEnabled: map[string]bool{"qcm-indice": true}}
	got, src := Resolve(g, round, direct)

	if !got.CuesEnabled["reveal"] || tw290aSrc(src.CuesEnabled["reveal"]) != "round" {
		t.Errorf("reveal : la manche la réactive : %v / %q", got.CuesEnabled["reveal"], src.CuesEnabled["reveal"])
	}
	if !got.CuesEnabled["qcm-indice"] || tw290aSrc(src.CuesEnabled["qcm-indice"]) != "direct" {
		t.Errorf("qcm-indice : la couche directe l'emporte : %v / %q", got.CuesEnabled["qcm-indice"], src.CuesEnabled["qcm-indice"])
	}
	for _, c := range tw290aCues {
		if c == "reveal" || c == "qcm-indice" {
			continue
		}
		if !got.CuesEnabled[c] || tw290aSrc(src.CuesEnabled[c]) != "global" {
			t.Errorf("cue %s non surchargée ⇒ global (true, \"global\"), got %v / %q", c, got.CuesEnabled[c], src.CuesEnabled[c])
		}
	}
	if len(got.CuesEnabled) != len(tw290aCues) {
		t.Errorf("forme complète : %d cues attendues, got %d", len(tw290aCues), len(got.CuesEnabled))
	}
}

// Les 8 cues par type du LOT-3 sont des citoyens de première classe de la résolution.
func TestResolve290_NewTypedCues_AreOverridable(t *testing.T) {
	g := tw290aGlobal()
	round := Partial{CuesEnabled: map[string]bool{
		"memory-carte": false, "memotion-fin-carte": false, "rafale-bonne": false, "rafale-fin-lot": false,
	}}
	got, src := Resolve(g, round)
	for _, c := range []string{"memory-carte", "memotion-fin-carte", "rafale-bonne", "rafale-fin-lot"} {
		if got.CuesEnabled[c] || tw290aSrc(src.CuesEnabled[c]) != "round" {
			t.Errorf("%s : surcharge à false attendue (source round), got %v / %q", c, got.CuesEnabled[c], src.CuesEnabled[c])
		}
	}
	if !got.CuesEnabled["memory-paire"] || !got.CuesEnabled["rafale-mauvaise"] {
		t.Error("les cues voisines non surchargées restent actives")
	}
}

func TestResolve290_PureFunction_NoMutationOfInputs(t *testing.T) {
	g := tw290aGlobal()
	round := Partial{SoundMuted: tw290aB(true), CuesEnabled: map[string]bool{"depart": false}}
	gBefore := tw290aGlobal()
	got, _ := Resolve(g, round)
	got.CuesEnabled["gagne"] = false // la modification du résultat ne doit rien changer ailleurs
	got.SoundMuted = false
	if !reflect.DeepEqual(g, gBefore) {
		t.Error("Resolve ne doit jamais modifier le global (ni partager sa map CuesEnabled avec le résultat)")
	}
	if round.CuesEnabled["depart"] != false || len(round.CuesEnabled) != 1 || *round.SoundMuted != true {
		t.Errorf("Resolve ne doit jamais modifier une couche, got %+v", round)
	}
}

func TestResolve290_Deterministic(t *testing.T) {
	g := tw290aGlobal()
	round := Partial{CountdownLoop: tw290aB(false), CuesEnabled: map[string]bool{"perdu": false}}
	a, sa := Resolve(g, round)
	b, sb := Resolve(g, round)
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(sa, sb) {
		t.Error("Resolve doit être déterministe")
	}
}

// Valeur zéro défensive : une cue absente de la forme complète vaut « active » côté consommateur (§5.2) ;
// côté Resolve, une cue présente dans une couche mais absente du global ressort avec la valeur de la couche.
func TestResolve290_LayerCueMissingFromGlobal_StillResolved(t *testing.T) {
	g := tw290aGlobal()
	delete(g.CuesEnabled, "rafale-fin-lot")
	got, src := Resolve(g, Partial{CuesEnabled: map[string]bool{"rafale-fin-lot": false}})
	if v, ok := got.CuesEnabled["rafale-fin-lot"]; !ok || v {
		t.Errorf("rafale-fin-lot=false (couche) attendu dans le résultat, got %v (présent=%v)", v, ok)
	}
	if tw290aSrc(src.CuesEnabled["rafale-fin-lot"]) != "round" {
		t.Errorf("source \"round\" attendue, got %q", src.CuesEnabled["rafale-fin-lot"])
	}
}

// ---------------------------------------------------------------------------
// Formes JSON (§3) — structurées, positives, Partial sans null.
// ---------------------------------------------------------------------------

func tw290aJSON(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal : %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("JSON invalide %s : %v", raw, err)
	}
	return m
}

func TestJSON290_Settings_StructuredPositiveCompleteForm(t *testing.T) {
	m := tw290aJSON(t, tw290aGlobal())
	snd, _ := m["sound"].(map[string]interface{})
	if snd == nil {
		t.Fatalf("clé `sound` attendue : %v", m)
	}
	if snd["muted"] != false {
		t.Errorf("sound.muted=false attendu, got %v", snd["muted"])
	}
	cd, _ := snd["countdown"].(map[string]interface{})
	if cd == nil || cd["enabled"] != true || cd["loop"] != true {
		t.Errorf("sound.countdown.{enabled,loop} attendus (forme positive), got %v", snd["countdown"])
	}
	cues, _ := snd["cues"].(map[string]interface{})
	if len(cues) != len(tw290aCues) {
		t.Errorf("forme complète : %d cues attendues, got %d", len(tw290aCues), len(cues))
	}
	light, _ := m["light"].(map[string]interface{})
	if light == nil || light["mode"] != "AUTO" {
		t.Errorf("light.mode=\"AUTO\" attendu, got %v", m["light"])
	}
}

func TestJSON290_Settings_RoundTrip(t *testing.T) {
	g := tw290aGlobal()
	g.SoundMuted = true
	g.CuesEnabled["depart"] = false
	g.LightMode = LightMode("OFF")
	raw, _ := json.Marshal(g)
	var back Settings
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal : %v", err)
	}
	if !reflect.DeepEqual(back, g) {
		t.Errorf("aller-retour infidèle :\n got  %+v\n want %+v", back, g)
	}
}

func TestJSON290_Partial_Empty_IsEmptyObject(t *testing.T) {
	raw, err := json.Marshal(Partial{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(raw)) != "{}" {
		t.Errorf("Partial vide ⇒ `{}` (tout hérité), got %s", raw)
	}
	if strings.Contains(string(raw), "null") {
		t.Errorf("jamais de `null` dans un Partial, got %s", raw)
	}
}

func TestJSON290_Partial_OmitsNilFields_AndEmptyCuesMap(t *testing.T) {
	cases := []struct {
		name string
		p    Partial
		want string
	}{
		{"muted seul", Partial{SoundMuted: tw290aB(true)}, `{"sound":{"muted":true}}`},
		{"décompte partiel", Partial{CountdownEnabled: tw290aB(false)}, `{"sound":{"countdown":{"enabled":false}}}`},
		{"lumière seule", Partial{LightMode: tw290aM("ON")}, `{"light":{"mode":"ON"}}`},
		{"cues vide omise", Partial{CuesEnabled: map[string]bool{}}, `{}`},
		{"cues partielle", Partial{CuesEnabled: map[string]bool{"qcm-indice": false}}, `{"sound":{"cues":{"qcm-indice":false}}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, err := json.Marshal(c.p)
			if err != nil {
				t.Fatal(err)
			}
			var got, want interface{}
			_ = json.Unmarshal(raw, &got)
			_ = json.Unmarshal([]byte(c.want), &want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %s, want %s", raw, c.want)
			}
		})
	}
}

func TestJSON290_Partial_Example_FromContract(t *testing.T) {
	const doc = `{ "sound": { "countdown": { "enabled": false }, "cues": { "qcm-indice": false } }, "light": { "mode": "ON" } }`
	var p Partial
	if err := json.Unmarshal([]byte(doc), &p); err != nil {
		t.Fatalf("exemple du contrat §3.2 : %v", err)
	}
	if p.CountdownEnabled == nil || *p.CountdownEnabled != false {
		t.Errorf("CountdownEnabled=false attendu, got %v", p.CountdownEnabled)
	}
	if p.CountdownLoop != nil || p.SoundMuted != nil {
		t.Errorf("champs absents ⇒ nil (hérités), got loop=%v muted=%v", p.CountdownLoop, p.SoundMuted)
	}
	if v, ok := p.CuesEnabled["qcm-indice"]; !ok || v {
		t.Errorf("cues[qcm-indice]=false attendu, got %v", p.CuesEnabled)
	}
	if p.LightMode == nil || *p.LightMode != LightMode("ON") {
		t.Errorf("LightMode=ON attendu, got %v", p.LightMode)
	}
	// aller-retour
	raw, _ := json.Marshal(p)
	var back Partial
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back, p) {
		t.Errorf("aller-retour infidèle : %+v / %v", back, err)
	}
}

func TestJSON290_Partial_EmptyObject_AllNil(t *testing.T) {
	var p Partial
	if err := json.Unmarshal([]byte(`{}`), &p); err != nil {
		t.Fatal(err)
	}
	if p.SoundMuted != nil || p.CountdownEnabled != nil || p.CountdownLoop != nil || p.LightMode != nil || len(p.CuesEnabled) != 0 {
		t.Errorf("`{}` ⇒ Partial vide, got %+v", p)
	}
}

func TestJSON290_Partial_StrictBooleanTypes(t *testing.T) {
	for _, doc := range []string{
		`{"sound":{"muted":"oui"}}`,
		`{"sound":{"muted":1}}`,
		`{"sound":{"countdown":{"enabled":"true"}}}`,
		`{"sound":{"cues":{"depart":"false"}}}`,
	} {
		var p Partial
		if err := json.Unmarshal([]byte(doc), &p); err == nil {
			t.Errorf("types booléens stricts : %s doit être refusé", doc)
		}
	}
}

func TestJSON290_Sources_Form(t *testing.T) {
	g := tw290aGlobal()
	_, src := Resolve(g,
		Partial{CountdownEnabled: tw290aB(false), CuesEnabled: map[string]bool{"qcm-indice": false}, LightMode: tw290aM("ON")},
		Partial{SoundMuted: tw290aB(true)})
	m := tw290aJSON(t, src)
	snd, _ := m["sound"].(map[string]interface{})
	if snd == nil || snd["muted"] != "direct" {
		t.Fatalf("sources.sound.muted=\"direct\" attendu, got %v", m)
	}
	cd, _ := snd["countdown"].(map[string]interface{})
	if cd == nil || cd["enabled"] != "round" || cd["loop"] != "global" {
		t.Errorf("sources.sound.countdown attendu {enabled:round, loop:global}, got %v", snd["countdown"])
	}
	cues, _ := snd["cues"].(map[string]interface{})
	if cues["qcm-indice"] != "round" || cues["depart"] != "global" {
		t.Errorf("sources.sound.cues attendu qcm-indice=round, depart=global, got %v", cues)
	}
	if light, _ := m["light"].(map[string]interface{}); light == nil || light["mode"] != "round" {
		t.Errorf("sources.light.mode=\"round\" attendu, got %v", m["light"])
	}
}

// ---------------------------------------------------------------------------
// Validate (§3.2) — light.mode, cues connues, jamais d'ignorance silencieuse.
// ---------------------------------------------------------------------------

func tw290aKnown(c string) bool {
	for _, k := range tw290aCues {
		if k == c {
			return true
		}
	}
	return false
}

func TestValidate290_Valid_And_Empty(t *testing.T) {
	if err := (Partial{}).Validate(tw290aKnown); err != nil {
		t.Errorf("Partial vide valide : %v", err)
	}
	ok := Partial{SoundMuted: tw290aB(true), CuesEnabled: map[string]bool{"qcm-indice": false}, LightMode: tw290aM("AUTO")}
	if err := ok.Validate(tw290aKnown); err != nil {
		t.Errorf("Partial valide refusé : %v", err)
	}
	for _, m := range []string{"ON", "AUTO", "OFF"} {
		if err := (Partial{LightMode: tw290aM(m)}).Validate(tw290aKnown); err != nil {
			t.Errorf("mode %s valide refusé : %v", m, err)
		}
	}
}

func TestValidate290_InvalidLightMode(t *testing.T) {
	for _, m := range []string{"", "on", "Auto", "ON ", "DIM", "AUTOMATIC"} {
		if err := (Partial{LightMode: tw290aM(m)}).Validate(tw290aKnown); err == nil {
			t.Errorf("light.mode %q doit être refusé", m)
		}
	}
}

func TestValidate290_UnknownCue_Refused_NamedInError(t *testing.T) {
	err := (Partial{CuesEnabled: map[string]bool{"qcm-indce": false}}).Validate(tw290aKnown)
	if err == nil {
		t.Fatal("une cue inconnue (faute de frappe) doit être refusée, jamais ignorée en silence")
	}
	if !strings.Contains(err.Error(), "qcm-indce") {
		t.Errorf("l'erreur doit nommer la cue fautive, got %q", err)
	}
}

func TestValidate290_UsesProvidedRegistry(t *testing.T) {
	only := func(c string) bool { return c == "depart" }
	if err := (Partial{CuesEnabled: map[string]bool{"depart": true}}).Validate(only); err != nil {
		t.Errorf("depart connue : %v", err)
	}
	if err := (Partial{CuesEnabled: map[string]bool{"gagne": true}}).Validate(only); err == nil {
		t.Error("gagne inconnue du registre fourni : refus attendu")
	}
}
