package main

// #223 (v11.7.0, LOT-2) — effets et vitesses lumière PAR ZONE au niveau de l'application (contrat §6, révision 4) :
// ROUNDS_UPDATE avec `light.effects.<zone>.<moment>` / `light.speeds.<zone>.<moment>` (validation INVALID_AMBIANCE contre le
// catalogue, atomique), résolution global ← manche, AMBIANCE_EFFECTIVE, GET /api/ambiance/settings.
// Source : contracts/light-effects.md §6, ambiance-settings.md §10, websocket-actions.md (INVALID_AMBIANCE).
// Réutilise tw290New (harnais #290). Aides préfixées tw223.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/config"
)

var tw223Zones = []string{"ambiance", "animateur", "public"}
var tw223Moments = []string{"chrono", "calm", "score"}

func tw223From(doc string) interface{} {
	var v interface{}
	_ = json.Unmarshal([]byte(doc), &v)
	return v
}

func tw223JSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func tw223Env(t *testing.T) *tw290Env {
	t.Helper()
	e := tw290New(t)
	e.app.setupEffects() // catalogue réel : natifs embarqués + dossier files/effects du dataDir de test
	e.app.httpServer.Effects = e.app
	return e
}

// tw223Amb : le bloc AMBIANCE d'une manche avec `light.effects` (table zone → moment → effet).
func tw223Amb(effects string) interface{} {
	return tw223From(`{"light":{"effects":` + effects + `}}`)
}

func tw223AmbSpeeds(speeds string) interface{} {
	return tw223From(`{"light":{"speeds":` + speeds + `}}`)
}

// tw223Zones3 configure les trois zones dans la configuration vivante : un groupe de salle, un animateur, un public.
func tw223Zones3(t *testing.T) {
	t.Helper()
	cfg := config.Get()
	cfg.Lighting.Groups = []config.LightingGroupEntry{
		{ID: "sa", Role: config.GroupRoleSalle, Lights: []string{"A"}},
		{ID: "ha", Role: config.GroupRoleAnimateur, Lights: []string{"P"}},
		{ID: "pa", Role: "public", Lights: []string{"R"}},
	}
	config.SetInstance(cfg)
}

func tw223Path(m map[string]interface{}, path ...string) interface{} { return tw290Path(m, path...) }

func tw223RoundAmb(t *testing.T, e *tw290Env) map[string]interface{} {
	t.Helper()
	amb, _ := e.roundsFile(t)[0]["AMBIANCE"].(map[string]interface{})
	return amb
}

func tw223Refused(t *testing.T, e *tw290Env, amb interface{}) (string, string) {
	t.Helper()
	for i, id := range []string{"1", "2"} {
		e.writeQ(t, id, i)
	}
	payload := `{"ROUNDS":[{"NAME":"R","QUESTIONS":["1","2"],"ENTRACTES":false,"AMBIANCE":` + tw223JSON(amb) + `}],"UNASSIGNED":[]}`
	return tw290ApplyResult(t, e, payload)
}

func TestTW223_RoundsUpdate_ValidEffects_StoredInRoundsJson(t *testing.T) {
	e := tw223Env(t)
	tw223Zones3(t)
	e.twoRounds(t, tw223Amb(`{"ambiance":{"chrono":"chenillard","calm":""},"animateur":{"score":"couleur-equipe"},"public":{"chrono":"pulsation"}}`))
	rs := e.roundsFile(t)
	if len(rs) != 2 {
		t.Fatalf("2 manches attendues : %v", rs)
	}
	amb := tw223RoundAmb(t, e)
	if tw290Path(amb, "light", "effects", "ambiance", "chrono") != "chenillard" {
		t.Errorf("light.effects.ambiance.chrono conservé : %v", rs[0]["AMBIANCE"])
	}
	if v, ok := tw290Path(amb, "light", "effects", "ambiance", "calm").(string); !ok || v != "" {
		t.Errorf("« aucun effet » (\"\") est une valeur de manche, pas une clé absente : %v", rs[0]["AMBIANCE"])
	}
	if tw290Path(amb, "light", "effects", "animateur", "score") != "couleur-equipe" || tw290Path(amb, "light", "effects", "public", "chrono") != "pulsation" {
		t.Errorf("light.effects.<zone>.<moment> conservé, par zone : %v", rs[0]["AMBIANCE"])
	}
	if tw223Path(amb, "light", "effects", "animateur", "chrono") != nil || tw290Path(amb, "light", "effects", "ambiance", "score") != nil {
		t.Errorf("seuls les zones et moments surchargés sont stockés : %v", rs[0]["AMBIANCE"])
	}
}

func TestTW223_RoundsUpdate_InvalidEffects_RefusedAtomically(t *testing.T) {
	cases := map[string]interface{}{
		"id inconnu":                         tw223Amb(`{"ambiance":{"chrono":"n-existe-pas"}}`),
		"effet qui ne déclare pas le moment": tw223Amb(`{"ambiance":{"calm":"chenillard"}}`),
		"animateur : id d'effet inconnu":     tw223Amb(`{"animateur":{"calm":"n-existe-pas"}}`),
		"animateur : mauvais moment":         tw223Amb(`{"animateur":{"chrono":"couleur-equipe"}}`),
		"public : id d'effet inconnu":        tw223Amb(`{"public":{"calm":"n-existe-pas"}}`),
		"moment inconnu":                     tw223Amb(`{"ambiance":{"bidule":"chenillard"}}`),
		"zone inconnue":                      tw223Amb(`{"salle":{"chrono":"chenillard"}}`),
		"un bon et un mauvais":               tw223Amb(`{"ambiance":{"chrono":"chenillard","calm":"chenillard"}}`),
		"ancienne forme à plat":              tw223Amb(`{"chrono":"chenillard"}`),
		"host_effects n'existe plus":         tw223From(`{"light":{"host_effects":{"ha":{"score":"couleur-equipe"}}}}`),
	}
	for name, amb := range cases {
		t.Run(name, func(t *testing.T) {
			e := tw223Env(t)
			tw223Zones3(t)
			status, reason := tw223Refused(t, e, amb)
			if status != "REFUSED" || reason != "INVALID_AMBIANCE" {
				t.Fatalf("attendu REFUSED/INVALID_AMBIANCE, obtenu %s/%s", status, reason)
			}
			if _, err := os.Stat(filepath.Join(e.dir, "rounds.json")); err == nil {
				t.Error("un refus ne doit rien écrire (rounds.json créé)")
			}
		})
	}
}

func TestTW223_EnteringRound_EffectiveShowsRoundOverrideWithSources(t *testing.T) {
	e := tw223Env(t)
	tw223Zones3(t)
	e.twoRounds(t, tw223Amb(`{"ambiance":{"chrono":"chenillard"},"animateur":{"score":"couleur-equipe"}}`))
	e.ready(t, "1") // question de r1
	m := e.settings(t)
	eff := func(zone, moment string) interface{} {
		return tw290Path(m, "effective", "light", "effects", zone, moment)
	}
	src := func(zone, moment string) interface{} {
		return tw290Path(m, "sources", "light", "effects", zone, moment)
	}
	if eff("ambiance", "chrono") != "chenillard" || src("ambiance", "chrono") != "round" {
		t.Errorf("chrono de l'ambiance surchargé par la manche : %v / %v", eff("ambiance", "chrono"), src("ambiance", "chrono"))
	}
	if eff("ambiance", "calm") != "" || src("ambiance", "calm") != "global" {
		t.Errorf("calm de l'ambiance hérité du global : %v / %v", eff("ambiance", "calm"), src("ambiance", "calm"))
	}
	if eff("animateur", "score") != "couleur-equipe" || src("animateur", "score") != "round" {
		t.Errorf("animateur.score surchargé : %v / %v", eff("animateur", "score"), src("animateur", "score"))
	}
	if eff("animateur", "chrono") != "" || src("animateur", "chrono") != "global" {
		t.Errorf("animateur.chrono hérité : %v / %v", eff("animateur", "chrono"), src("animateur", "chrono"))
	}
	if eff("public", "score") != "" || src("public", "score") != "global" {
		t.Errorf("la zone public n'est pas touchée par la manche : %v / %v", eff("public", "score"), src("public", "score"))
	}
	// hors manche (question « Sans manche ») : retour au global
	e.ready(t, "4")
	m = e.settings(t)
	if eff("ambiance", "chrono") != "pulsation" || src("ambiance", "chrono") != "global" {
		t.Errorf("hors manche : effectif = global (pulsation) : %v / %v", eff("ambiance", "chrono"), src("ambiance", "chrono"))
	}
}

func TestTW223_Defaults_EffectiveEqualsGlobalDefaults_V1160Behaviour(t *testing.T) {
	e := tw223Env(t)
	tw223Zones3(t)
	m := e.settings(t)
	for _, z := range tw223Zones {
		for _, mo := range tw223Moments {
			want := ""
			if z == "ambiance" && mo == "chrono" {
				want = "pulsation"
			}
			got, ok := tw290Path(m, "effective", "light", "effects", z, mo).(string)
			if !ok || got != want {
				t.Errorf("effects.%s.%s par défaut : %v, attendu %q", z, mo, got, want)
			}
			if src := tw290Path(m, "sources", "light", "effects", z, mo); src != "global" {
				t.Errorf("source de effects.%s.%s : %v", z, mo, src)
			}
		}
	}
}

func TestTW223_GlobalFromConfig_AppearsInEffectiveAndAmbianceEffective(t *testing.T) {
	e := tw223Env(t)
	code, out := e.post(t, "/config.json", `{"lighting":{"effects":{"ambiance":{"calm":"couleur-equipe"}}}}`)
	if code == 200 {
		t.Fatalf("couleur-equipe ne déclare pas le moment calm : 400 attendu (%v)", out)
	}
	if code, out = e.post(t, "/config.json", `{"lighting":{"effects":{"ambiance":{"chrono":"chenillard"}}}}`); code != 200 {
		t.Fatalf("%d %v", code, out)
	}
	m := e.settings(t)
	if tw290Path(m, "effective", "light", "effects", "ambiance", "chrono") != "chenillard" || tw290Path(m, "global", "light", "effects", "ambiance", "chrono") != "chenillard" {
		t.Errorf("le global écrit par /config.json est l'effectif : %v", tw290Path(m, "effective", "light"))
	}
	ae := tw290Path(e.gameJSON(t), "AMBIANCE_EFFECTIVE", "SETTINGS", "light", "effects")
	zm, ok := ae.(map[string]interface{})
	if !ok || len(zm) != 3 {
		t.Fatalf("GameState.AMBIANCE_EFFECTIVE.SETTINGS.light.effects toujours sérialisé avec ses 3 zones : %v", ae)
	}
	for _, z := range tw223Zones {
		if mm, ok := zm[z].(map[string]interface{}); !ok || len(mm) != 3 {
			t.Errorf("zone %s : 3 moments toujours présents : %v", z, zm[z])
		}
	}
	if tw290Path(e.gameJSON(t), "AMBIANCE_EFFECTIVE", "SETTINGS", "light", "effects", "ambiance", "chrono") != "chenillard" {
		t.Errorf("valeur : %v", ae)
	}
}

func TestTW223_RoundOverrideBeatsGlobal_ThenRoundExitRestoresGlobal(t *testing.T) {
	e := tw223Env(t)
	if code, out := e.post(t, "/config.json", `{"lighting":{"effects":{"ambiance":{"chrono":"chenillard"}}}}`); code != 200 {
		t.Fatalf("%d %v", code, out)
	}
	e.twoRounds(t, tw223Amb(`{"ambiance":{"chrono":""}}`)) // « aucun effet » dans cette manche
	e.ready(t, "1")
	m := e.settings(t)
	if v, ok := tw290Path(m, "effective", "light", "effects", "ambiance", "chrono").(string); !ok || v != "" {
		t.Errorf("\"\" de manche l'emporte sur « chenillard » global : %v", tw290Path(m, "effective", "light", "effects", "ambiance", "chrono"))
	}
	if tw290Path(m, "sources", "light", "effects", "ambiance", "chrono") != "round" {
		t.Errorf("source round : %v", tw290Path(m, "sources", "light", "effects", "ambiance", "chrono"))
	}
	e.ready(t, "4")
	if got := tw290Path(e.settings(t), "effective", "light", "effects", "ambiance", "chrono"); got != "chenillard" {
		t.Errorf("sortie de manche : retour au global : %v", got)
	}
}

func TestTW223_EffectsAreNeverWrittenByDirectGestures(t *testing.T) {
	// Pas de geste direct pour les effets (contrat §6) : le geste lumière ne touche que light.mode.
	e := tw223Env(t)
	before := tw290Path(e.settings(t), "effective", "light", "effects")
	e.post(t, "/api/lighting/mode", `{"mode":"ON"}`) // 409 not_configured sans pilote : aucun effet sur les effets
	after := tw290Path(e.settings(t), "effective", "light", "effects")
	if tw223JSON(before) != tw223JSON(after) {
		t.Errorf("effets modifiés par un geste direct : %v → %v", before, after)
	}
}

func TestTW223_NoHostField_AnywhereInTheEffectiveSettings(t *testing.T) {
	e := tw223Env(t)
	m := e.settings(t)
	for _, k := range []string{"host_effects", "host_speeds"} {
		if tw290Path(m, "effective", "light", k) != nil {
			t.Errorf("light.%s n'existe plus (révision 4) : %v", k, tw290Path(m, "effective", "light", k))
		}
		if tw290Path(e.gameJSON(t), "AMBIANCE_EFFECTIVE", "SETTINGS", "light", k) != nil {
			t.Errorf("AMBIANCE_EFFECTIVE.SETTINGS.light.%s n'existe plus", k)
		}
	}
}

func TestTW223_RoundsUpdate_OldFlatEffectsAndHostEffectsRefused(t *testing.T) {
	for name, amb := range map[string]interface{}{
		"effects à plat":          tw223From(`{"light":{"effects":{"chrono":"chenillard","calm":"","score":""}}}`),
		"host_effects à plat":     tw223From(`{"light":{"host_effects":{"chrono":"","calm":"","score":"couleur-equipe"}}}`),
		"host_effects par groupe": tw223From(`{"light":{"host_effects":{"ha":{"score":"couleur-equipe"}}}}`),
		"speeds à plat":           tw223From(`{"light":{"speeds":{"chrono":2}}}`),
		"host_speeds par groupe":  tw223From(`{"light":{"host_speeds":{"ha":{"score":2}}}}`),
	} {
		t.Run(name, func(t *testing.T) {
			e := tw223Env(t)
			tw223Zones3(t)
			if status, reason := tw223Refused(t, e, amb); status != "REFUSED" || reason != "INVALID_AMBIANCE" {
				t.Errorf("les anciennes formes ne sont plus acceptées à l'écriture : %s/%s", status, reason)
			}
		})
	}
}

func TestTW223_RoundOldFlatFormsInRoundsJson_ReadLeniently(t *testing.T) {
	// rounds.json de QUALIF : effects à plat (= zone ambiance), host_effects ignoré. Le serveur le lit sans erreur.
	e := tw223Env(t)
	tw223Zones3(t)
	for i, id := range []string{"1", "2"} {
		e.writeQ(t, id, i)
	}
	rounds := `{"ROUNDS":[{"ID":"r1","NAME":"R","QUESTIONS":["1","2"],"ENTRACTES":false,"AMBIANCE":{"light":{"effects":{"chrono":"chenillard"},` +
		`"host_effects":{"chrono":"","calm":"","score":"couleur-equipe"},"host_speeds":{"ha":{"calm":2}}}}}]}`
	if err := os.WriteFile(filepath.Join(e.dir, "rounds.json"), []byte(rounds), 0o644); err != nil {
		t.Fatal(err)
	}
	e.ready(t, "1")
	m := e.settings(t) // ne plante pas, effectif lisible
	if got := tw290Path(m, "effective", "light", "effects", "ambiance", "chrono"); got != "chenillard" {
		t.Errorf("effects à plat lus comme la zone ambiance : %v", got)
	}
	if got := tw290Path(m, "effective", "light", "effects", "animateur", "score"); got != "" {
		t.Errorf("host_effects ignoré : animateur.score = %v", got)
	}
	if got := tw290Path(m, "effective", "light", "speeds", "animateur", "calm"); got != float64(1) {
		t.Errorf("host_speeds ignoré : animateur.calm = %v", got)
	}
}

// ---------------------------------------------------------------------------
// Vitesse réglable (R13) : light.speeds.<zone>.<moment> par manche
// ---------------------------------------------------------------------------

func TestTW223_RoundsUpdate_ValidSpeeds_StoredPerZoneAndMoment(t *testing.T) {
	e := tw223Env(t)
	tw223Zones3(t)
	e.twoRounds(t, tw223AmbSpeeds(`{"ambiance":{"chrono":2},"animateur":{"score":0.5}}`))
	amb := tw223RoundAmb(t, e)
	if tw223Path(amb, "light", "speeds", "ambiance", "chrono") != float64(2) {
		t.Errorf("light.speeds.ambiance.chrono conservé : %v", amb)
	}
	if tw223Path(amb, "light", "speeds", "animateur", "score") != 0.5 {
		t.Errorf("light.speeds.<zone>.<moment> conservé : %v", amb)
	}
	if tw223Path(amb, "light", "speeds", "ambiance", "calm") != nil || tw223Path(amb, "light", "speeds", "public") != nil {
		t.Errorf("seules les vitesses surchargées sont stockées : %v", amb)
	}
}

func TestTW223_RoundsUpdate_InvalidSpeed_RefusedAtomically_InvalidAmbiance(t *testing.T) {
	cases := map[string]interface{}{
		"vitesse hors liste":             tw223AmbSpeeds(`{"ambiance":{"chrono":3}}`),
		"vitesse zéro":                   tw223AmbSpeeds(`{"ambiance":{"calm":0}}`),
		"vitesse en chaîne":              tw223AmbSpeeds(`{"ambiance":{"calm":"2"}}`),
		"moment inconnu":                 tw223AmbSpeeds(`{"ambiance":{"bidule":1}}`),
		"zone inconnue":                  tw223AmbSpeeds(`{"salle":{"chrono":1}}`),
		"animateur : vitesse hors liste": tw223AmbSpeeds(`{"animateur":{"chrono":1.25}}`),
		"public : vitesse hors liste":    tw223AmbSpeeds(`{"public":{"score":2.5}}`),
	}
	for name, amb := range cases {
		t.Run(name, func(t *testing.T) {
			e := tw223Env(t)
			tw223Zones3(t)
			status, reason := tw223Refused(t, e, amb)
			if status != "REFUSED" || reason != "INVALID_AMBIANCE" {
				t.Fatalf("attendu REFUSED/INVALID_AMBIANCE, obtenu %s/%s", status, reason)
			}
			if _, err := os.Stat(filepath.Join(e.dir, "rounds.json")); err == nil {
				t.Error("un refus ne doit rien écrire (rounds.json créé)")
			}
		})
	}
}

func TestTW223_EnteringRound_EffectiveSpeedsWithSources_ThenGlobalAfterExit(t *testing.T) {
	e := tw223Env(t)
	tw223Zones3(t)
	e.twoRounds(t, tw223AmbSpeeds(`{"ambiance":{"chrono":2},"animateur":{"score":0.5}}`))
	e.ready(t, "1")
	m := e.settings(t)
	sp := func(kind string, path ...string) interface{} {
		return tw290Path(m, append([]string{kind, "light", "speeds"}, path...)...)
	}
	if sp("effective", "ambiance", "chrono") != float64(2) || sp("sources", "ambiance", "chrono") != "round" {
		t.Errorf("chrono ×2 imposé par la manche : %v / %v", sp("effective", "ambiance", "chrono"), sp("sources", "ambiance", "chrono"))
	}
	if sp("effective", "ambiance", "calm") != float64(1) || sp("sources", "ambiance", "calm") != "global" {
		t.Errorf("calm hérité (×1) : %v / %v", sp("effective", "ambiance", "calm"), sp("sources", "ambiance", "calm"))
	}
	if sp("effective", "animateur", "score") != 0.5 || sp("sources", "animateur", "score") != "round" {
		t.Errorf("animateur.score ×0,5 imposé : %v", sp("effective", "animateur", "score"))
	}
	if sp("effective", "public", "score") != float64(1) || sp("sources", "public", "score") != "global" {
		t.Errorf("la zone public n'est pas touchée : %v", sp("effective", "public", "score"))
	}
	e.ready(t, "4") // hors manche
	if got := tw290Path(e.settings(t), "effective", "light", "speeds", "ambiance", "chrono"); got != float64(1) {
		t.Errorf("sortie de manche : retour au global (×1) : %v", got)
	}
}

func TestTW223_Defaults_SpeedsOneEverywhere_AlwaysSerialisedInAmbianceEffective(t *testing.T) {
	e := tw223Env(t)
	tw223Zones3(t)
	m := e.settings(t)
	for _, z := range tw223Zones {
		for _, mo := range tw223Moments {
			if got := tw290Path(m, "effective", "light", "speeds", z, mo); got != float64(1) {
				t.Errorf("speeds.%s.%s par défaut ×1 : %v", z, mo, got)
			}
		}
	}
	ae := tw290Path(e.gameJSON(t), "AMBIANCE_EFFECTIVE", "SETTINGS", "light", "speeds")
	zm, ok := ae.(map[string]interface{})
	if !ok || len(zm) != 3 {
		t.Fatalf("AMBIANCE_EFFECTIVE.SETTINGS.light.speeds toujours sérialisé (3 zones) : %v", ae)
	}
	for _, z := range tw223Zones {
		if mm, ok := zm[z].(map[string]interface{}); !ok || len(mm) != 3 {
			t.Errorf("zone %s : 3 moments toujours présents : %v", z, zm[z])
		}
	}
}

func TestTW223_GlobalSpeedFromConfig_IsTheEffective_RoundOverrideBeatsIt(t *testing.T) {
	e := tw223Env(t)
	if code, out := e.post(t, "/config.json", `{"lighting":{"speeds":{"ambiance":{"calm":0.5}}}}`); code != 200 {
		t.Fatalf("%d %v", code, out)
	}
	if got := tw290Path(e.settings(t), "effective", "light", "speeds", "ambiance", "calm"); got != 0.5 {
		t.Errorf("le global écrit par /config.json est l'effectif : %v", got)
	}
	e.twoRounds(t, tw223AmbSpeeds(`{"ambiance":{"calm":2}}`))
	e.ready(t, "1")
	if got := tw290Path(e.settings(t), "effective", "light", "speeds", "ambiance", "calm"); got != float64(2) {
		t.Errorf("la manche l'emporte sur le global : %v", got)
	}
	e.ready(t, "4")
	if got := tw290Path(e.settings(t), "effective", "light", "speeds", "ambiance", "calm"); got != 0.5 {
		t.Errorf("sortie de manche : retour au global (×0,5) : %v", got)
	}
}
