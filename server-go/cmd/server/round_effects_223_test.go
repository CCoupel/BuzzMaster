package main

// #223 (v11.7.0, LOT-2) — effets lumière par moment au niveau de l'application : ROUNDS_UPDATE avec
// `light.effects` / `light.host_effects` (validation INVALID_AMBIANCE contre le catalogue, atomique), résolution
// global ← manche, AMBIANCE_EFFECTIVE, GET /api/ambiance/settings. Source : contracts/light-effects.md §6,
// ambiance-settings.md §10, websocket-actions.md (INVALID_AMBIANCE). Réutilise tw290New (harnais #290).
// Aides préfixées tw223.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/config"
)

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

func tw223Amb(effects, host string) interface{} {
	return tw223From(`{"light":{"effects":` + effects + `,"host_effects":` + host + `}}`)
}

// tw223HostGroups configure deux groupes animateur d'id « ha » (rang 1) et « hb » (rang 2) dans la configuration vivante.
func tw223HostGroups(t *testing.T) {
	t.Helper()
	cfg := config.Get()
	cfg.Lighting.Groups = []config.LightingGroupEntry{
		{ID: "ha", Role: config.GroupRoleAnimateur, Lights: []string{"P"}},
		{ID: "hb", Role: config.GroupRoleAnimateur, Lights: []string{"R"}},
	}
	config.SetInstance(cfg)
}

func TestTW223_RoundsUpdate_ValidEffects_StoredInRoundsJson(t *testing.T) {
	e := tw223Env(t)
	tw223HostGroups(t)
	e.twoRounds(t, tw223Amb(`{"chrono":"chenillard","calm":""}`, `{"ha":{"score":"couleur-equipe"},"hb":{"chrono":"pulsation"}}`))
	rs := e.roundsFile(t)
	if len(rs) != 2 {
		t.Fatalf("2 manches attendues : %v", rs)
	}
	amb, _ := rs[0]["AMBIANCE"].(map[string]interface{})
	if tw290Path(amb, "light", "effects", "chrono") != "chenillard" {
		t.Errorf("light.effects.chrono conservé : %v", rs[0]["AMBIANCE"])
	}
	if v, ok := tw290Path(amb, "light", "effects", "calm").(string); !ok || v != "" {
		t.Errorf("« aucun effet » (\"\") est une valeur de manche, pas une clé absente : %v", rs[0]["AMBIANCE"])
	}
	if tw290Path(amb, "light", "host_effects", "ha", "score") != "couleur-equipe" || tw290Path(amb, "light", "host_effects", "hb", "chrono") != "pulsation" {
		t.Errorf("light.host_effects.<id>.<moment> conservé, par groupe : %v", rs[0]["AMBIANCE"])
	}
	if tw223Path(amb, "light", "host_effects", "ha", "chrono") != nil {
		t.Errorf("seuls les groupes et moments surchargés sont stockés : %v", rs[0]["AMBIANCE"])
	}
	if tw290Path(amb, "light", "effects", "score") != nil {
		t.Errorf("seules les clés surchargées sont stockées : %v", rs[0]["AMBIANCE"])
	}
}

func TestTW223_RoundsUpdate_InvalidEffects_RefusedAtomically(t *testing.T) {
	cases := map[string]interface{}{
		"id inconnu":                         tw223Amb(`{"chrono":"n-existe-pas"}`, `{}`),
		"effet qui ne déclare pas le moment": tw223Amb(`{"calm":"chenillard"}`, `{}`),
		"animateur : id d'effet inconnu":     tw223Amb(`{}`, `{"ha":{"calm":"n-existe-pas"}}`),
		"animateur : mauvais moment":         tw223Amb(`{}`, `{"ha":{"chrono":"couleur-equipe"}}`),
		"animateur : moment inconnu":         tw223Amb(`{}`, `{"ha":{"bidule":"couleur-equipe"}}`),
		"moment inconnu":                     tw223From(`{"light":{"effects":{"bidule":"chenillard"}}}`),
		"un bon et un mauvais":               tw223Amb(`{"chrono":"chenillard","calm":"chenillard"}`, `{}`),
	}
	for name, amb := range cases {
		t.Run(name, func(t *testing.T) {
			e := tw223Env(t)
			tw223HostGroups(t)
			for i, id := range []string{"1", "2"} {
				e.writeQ(t, id, i)
			}
			payload := `{"ROUNDS":[{"NAME":"R","QUESTIONS":["1","2"],"ENTRACTES":false,"AMBIANCE":` + tw223JSON(amb) + `}],"UNASSIGNED":[]}`
			status, reason := tw290ApplyResult(t, e, payload)
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
	tw223HostGroups(t)
	e.twoRounds(t, tw223Amb(`{"chrono":"chenillard"}`, `{"ha":{"score":"couleur-equipe"}}`))
	e.ready(t, "1") // question de r1
	m := e.settings(t)
	eff := func(kind, moment string) interface{} { return tw290Path(m, "effective", "light", kind, moment) }
	src := func(kind, moment string) interface{} { return tw290Path(m, "sources", "light", kind, moment) }
	if eff("effects", "chrono") != "chenillard" || src("effects", "chrono") != "round" {
		t.Errorf("chrono surchargé par la manche : %v / %v", eff("effects", "chrono"), src("effects", "chrono"))
	}
	if eff("effects", "calm") != "" || src("effects", "calm") != "global" {
		t.Errorf("calm hérité du global : %v / %v", eff("effects", "calm"), src("effects", "calm"))
	}
	hostEff := func(id, moment string) interface{} {
		return tw290Path(m, "effective", "light", "host_effects", id, moment)
	}
	hostSrc := func(id, moment string) interface{} {
		return tw290Path(m, "sources", "light", "host_effects", id, moment)
	}
	if hostEff("ha", "score") != "couleur-equipe" || hostSrc("ha", "score") != "round" {
		t.Errorf("host_effects.ha.score surchargé : %v / %v", hostEff("ha", "score"), hostSrc("ha", "score"))
	}
	if hostEff("ha", "chrono") != "" || hostSrc("ha", "chrono") != "global" {
		t.Errorf("host_effects.ha.chrono hérité : %v / %v", hostEff("ha", "chrono"), hostSrc("ha", "chrono"))
	}
	if hostEff("hb", "score") != "" || hostSrc("hb", "score") != "global" {
		t.Errorf("le groupe hb n'est pas touché par la manche : %v / %v", hostEff("hb", "score"), hostSrc("hb", "score"))
	}
	// hors manche (question « Sans manche ») : retour au global
	e.ready(t, "4")
	m = e.settings(t)
	if eff("effects", "chrono") != "pulsation" || src("effects", "chrono") != "global" {
		t.Errorf("hors manche : effectif = global (pulsation) : %v / %v", eff("effects", "chrono"), src("effects", "chrono"))
	}
}

func TestTW223_Defaults_EffectiveEqualsGlobalDefaults_V1160Behaviour(t *testing.T) {
	e := tw223Env(t)
	tw223HostGroups(t)
	m := e.settings(t)
	want := map[string]string{"chrono": "pulsation", "calm": "", "score": ""}
	for moment, v := range want {
		if got := tw290Path(m, "effective", "light", "effects", moment); got != v {
			t.Errorf("effects.%s par défaut : %v, attendu %q", moment, got, v)
		}
		if src := tw290Path(m, "sources", "light", "effects", moment); src != "global" {
			t.Errorf("source de effects.%s : %v", moment, src)
		}
		for _, id := range []string{"ha", "hb"} {
			if got := tw290Path(m, "effective", "light", "host_effects", id, moment); got != "" {
				t.Errorf("host_effects.%s.%s par défaut : blanc fixe (\"\"), obtenu %v", id, moment, got)
			}
		}
	}
}

func TestTW223_GlobalFromConfig_AppearsInEffectiveAndAmbianceEffective(t *testing.T) {
	e := tw223Env(t)
	code, out := e.post(t, "/config.json", `{"lighting":{"effects":{"calm":"couleur-equipe"}}}`)
	if code == 200 {
		t.Fatalf("couleur-equipe ne déclare pas le moment calm : 400 attendu (%v)", out)
	}
	if code, out = e.post(t, "/config.json", `{"lighting":{"effects":{"chrono":"chenillard"}}}`); code != 200 {
		t.Fatalf("%d %v", code, out)
	}
	m := e.settings(t)
	if tw290Path(m, "effective", "light", "effects", "chrono") != "chenillard" || tw290Path(m, "global", "light", "effects", "chrono") != "chenillard" {
		t.Errorf("le global écrit par /config.json est l'effectif : %v", tw290Path(m, "effective", "light"))
	}
	ae := tw290Path(e.gameJSON(t), "AMBIANCE_EFFECTIVE", "SETTINGS", "light", "effects")
	if mm, ok := ae.(map[string]interface{}); !ok || len(mm) != 3 || mm["chrono"] != "chenillard" {
		t.Errorf("GameState.AMBIANCE_EFFECTIVE.SETTINGS.light.effects toujours sérialisé avec ses 3 clés : %v", ae)
	}
}

func TestTW223_RoundOverrideBeatsGlobal_ThenRoundExitRestoresGlobal(t *testing.T) {
	e := tw223Env(t)
	if code, out := e.post(t, "/config.json", `{"lighting":{"effects":{"chrono":"chenillard"}}}`); code != 200 {
		t.Fatalf("%d %v", code, out)
	}
	e.twoRounds(t, tw223Amb(`{"chrono":""}`, `{}`)) // « aucun effet » dans cette manche
	e.ready(t, "1")
	m := e.settings(t)
	if v, ok := tw290Path(m, "effective", "light", "effects", "chrono").(string); !ok || v != "" {
		t.Errorf("\"\" de manche l'emporte sur « chenillard » global : %v", tw290Path(m, "effective", "light", "effects", "chrono"))
	}
	if tw290Path(m, "sources", "light", "effects", "chrono") != "round" {
		t.Errorf("source round : %v", tw290Path(m, "sources", "light", "effects", "chrono"))
	}
	e.ready(t, "4")
	if got := tw290Path(e.settings(t), "effective", "light", "effects", "chrono"); got != "chenillard" {
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

func TestTW223_NoHostGroup_HostEffectsIsEmptyObjectEverywhere(t *testing.T) {
	e := tw223Env(t)
	m := e.settings(t)
	if he, ok := tw290Path(m, "effective", "light", "host_effects").(map[string]interface{}); !ok || len(he) != 0 {
		t.Errorf("aucun groupe animateur ⇒ host_effects = {} dans l'effectif : %v", tw290Path(m, "effective", "light", "host_effects"))
	}
	ae := tw290Path(e.gameJSON(t), "AMBIANCE_EFFECTIVE", "SETTINGS", "light", "host_effects")
	if he, ok := ae.(map[string]interface{}); !ok || len(he) != 0 {
		t.Errorf("AMBIANCE_EFFECTIVE.SETTINGS.light.host_effects toujours sérialisé ({} sans groupe animateur) : %v", ae)
	}
}

func TestTW223_RoundsUpdate_UnknownHostGroupId_DroppedWithoutError(t *testing.T) {
	e := tw223Env(t)
	tw223HostGroups(t)
	for i, id := range []string{"1", "2"} {
		e.writeQ(t, id, i)
	}
	payload := `{"ROUNDS":[{"NAME":"R","QUESTIONS":["1","2"],"ENTRACTES":false,"AMBIANCE":` +
		tw223JSON(tw223Amb(`{}`, `{"disparu":{"score":"couleur-equipe"},"ha":{"calm":""}}`)) + `}],"UNASSIGNED":[]}`
	status, reason := tw290ApplyResult(t, e, payload)
	if status == "REFUSED" {
		t.Fatalf("un id de groupe inconnu est écarté sans erreur (§6), pas refusé : %s/%s", status, reason)
	}
	amb, _ := e.roundsFile(t)[0]["AMBIANCE"].(map[string]interface{})
	if tw223Path(amb, "light", "host_effects", "disparu", "score") != nil {
		t.Errorf("l'entrée du groupe inexistant n'est pas conservée : %v", amb)
	}
}

func TestTW223_RoundsUpdate_OldFlatHostEffectsNoLongerAccepted(t *testing.T) {
	e := tw223Env(t)
	tw223HostGroups(t)
	for i, id := range []string{"1", "2"} {
		e.writeQ(t, id, i)
	}
	payload := `{"ROUNDS":[{"NAME":"R","QUESTIONS":["1","2"],"ENTRACTES":false,"AMBIANCE":` +
		tw223JSON(tw223Amb(`{}`, `{"score":"couleur-equipe"}`)) + `}],"UNASSIGNED":[]}`
	status, reason := tw290ApplyResult(t, e, payload)
	if status == "REFUSED" && reason != "INVALID_AMBIANCE" {
		t.Fatalf("refus attendu en INVALID_AMBIANCE : %s/%s", status, reason)
	}
	if status != "REFUSED" {
		// à défaut de refus, la forme plate ne doit jamais être prise pour des id de groupe
		amb, _ := e.roundsFile(t)[0]["AMBIANCE"].(map[string]interface{})
		if tw223Path(amb, "light", "host_effects", "score") != nil {
			t.Errorf("la forme plate n'est jamais stockée comme un groupe : %v", amb)
		}
	}
}

func TestTW223_RoundOldFlatHostEffectsInRoundsJson_IgnoredAtLoad(t *testing.T) {
	// rounds.json de QUALIF 11.7.0.2 : host_effects à plat. Le serveur le lit sans erreur et l'ignore.
	e := tw223Env(t)
	tw223HostGroups(t)
	for i, id := range []string{"1", "2"} {
		e.writeQ(t, id, i)
	}
	rounds := `{"ROUNDS":[{"ID":"r1","NAME":"R","QUESTIONS":["1","2"],"ENTRACTES":false,"AMBIANCE":{"light":{"host_effects":{"chrono":"","calm":"","score":"couleur-equipe"}}}}]}`
	if err := os.WriteFile(filepath.Join(e.dir, "rounds.json"), []byte(rounds), 0o644); err != nil {
		t.Fatal(err)
	}
	e.ready(t, "1")
	m := e.settings(t) // ne plante pas, effectif lisible
	if got := tw290Path(m, "effective", "light", "host_effects", "ha", "score"); got != "" {
		t.Errorf("l'ancienne forme est ignorée : ha.score = %v", got)
	}
}

func tw223Path(m map[string]interface{}, path ...string) interface{} { return tw290Path(m, path...) }

// ---------------------------------------------------------------------------
// Vitesse réglable (R13) : light.speeds.<moment> et light.host_speeds.<id>.<moment> par manche
// ---------------------------------------------------------------------------

func tw223AmbSpeeds(speeds, host string) interface{} {
	return tw223From(`{"light":{"speeds":` + speeds + `,"host_speeds":` + host + `}}`)
}

func TestTW223_RoundsUpdate_ValidSpeeds_StoredPerMomentAndGroup(t *testing.T) {
	e := tw223Env(t)
	tw223HostGroups(t)
	e.twoRounds(t, tw223AmbSpeeds(`{"chrono":2}`, `{"ha":{"score":0.5}}`))
	amb, _ := e.roundsFile(t)[0]["AMBIANCE"].(map[string]interface{})
	if tw223Path(amb, "light", "speeds", "chrono") != float64(2) {
		t.Errorf("light.speeds.chrono conservé : %v", amb)
	}
	if tw223Path(amb, "light", "host_speeds", "ha", "score") != 0.5 {
		t.Errorf("light.host_speeds.<id>.<moment> conservé : %v", amb)
	}
	if tw223Path(amb, "light", "speeds", "calm") != nil {
		t.Errorf("seules les vitesses surchargées sont stockées : %v", amb)
	}
}

func TestTW223_RoundsUpdate_InvalidSpeed_RefusedAtomically_InvalidAmbiance(t *testing.T) {
	cases := map[string]interface{}{
		"vitesse hors liste":             tw223AmbSpeeds(`{"chrono":3}`, `{}`),
		"vitesse zéro":                   tw223AmbSpeeds(`{"calm":0}`, `{}`),
		"vitesse en chaîne":              tw223AmbSpeeds(`{"calm":"2"}`, `{}`),
		"moment inconnu":                 tw223AmbSpeeds(`{"bidule":1}`, `{}`),
		"animateur : vitesse hors liste": tw223AmbSpeeds(`{}`, `{"ha":{"chrono":1.25}}`),
		"animateur : moment inconnu":     tw223AmbSpeeds(`{}`, `{"ha":{"bidule":1}}`),
	}
	for name, amb := range cases {
		t.Run(name, func(t *testing.T) {
			e := tw223Env(t)
			tw223HostGroups(t)
			for i, id := range []string{"1", "2"} {
				e.writeQ(t, id, i)
			}
			payload := `{"ROUNDS":[{"NAME":"R","QUESTIONS":["1","2"],"ENTRACTES":false,"AMBIANCE":` + tw223JSON(amb) + `}],"UNASSIGNED":[]}`
			status, reason := tw290ApplyResult(t, e, payload)
			if status != "REFUSED" || reason != "INVALID_AMBIANCE" {
				t.Fatalf("attendu REFUSED/INVALID_AMBIANCE, obtenu %s/%s", status, reason)
			}
			if _, err := os.Stat(filepath.Join(e.dir, "rounds.json")); err == nil {
				t.Error("un refus ne doit rien écrire (rounds.json créé)")
			}
		})
	}
}

func TestTW223_RoundsUpdate_UnknownHostGroupIdInSpeeds_DroppedWithoutError(t *testing.T) {
	e := tw223Env(t)
	tw223HostGroups(t)
	for i, id := range []string{"1", "2"} {
		e.writeQ(t, id, i)
	}
	payload := `{"ROUNDS":[{"NAME":"R","QUESTIONS":["1","2"],"ENTRACTES":false,"AMBIANCE":` +
		tw223JSON(tw223AmbSpeeds(`{}`, `{"disparu":{"chrono":2},"ha":{"calm":2}}`)) + `}],"UNASSIGNED":[]}`
	if status, reason := tw290ApplyResult(t, e, payload); status == "REFUSED" {
		t.Fatalf("un id de groupe inconnu est écarté sans erreur : %s/%s", status, reason)
	}
	amb, _ := e.roundsFile(t)[0]["AMBIANCE"].(map[string]interface{})
	if tw223Path(amb, "light", "host_speeds", "disparu", "chrono") != nil {
		t.Errorf("l'entrée du groupe inexistant n'est pas conservée : %v", amb)
	}
}

func TestTW223_EnteringRound_EffectiveSpeedsWithSources_ThenGlobalAfterExit(t *testing.T) {
	e := tw223Env(t)
	tw223HostGroups(t)
	e.twoRounds(t, tw223AmbSpeeds(`{"chrono":2}`, `{"ha":{"score":0.5}}`))
	e.ready(t, "1")
	m := e.settings(t)
	sp := func(kind string, path ...string) interface{} {
		return tw290Path(m, append([]string{kind, "light"}, path...)...)
	}
	if sp("effective", "speeds", "chrono") != float64(2) || sp("sources", "speeds", "chrono") != "round" {
		t.Errorf("chrono ×2 imposé par la manche : %v / %v", sp("effective", "speeds", "chrono"), sp("sources", "speeds", "chrono"))
	}
	if sp("effective", "speeds", "calm") != float64(1) || sp("sources", "speeds", "calm") != "global" {
		t.Errorf("calm hérité (×1) : %v / %v", sp("effective", "speeds", "calm"), sp("sources", "speeds", "calm"))
	}
	if sp("effective", "host_speeds", "ha", "score") != 0.5 || sp("sources", "host_speeds", "ha", "score") != "round" {
		t.Errorf("ha.score ×0,5 imposé : %v", sp("effective", "host_speeds", "ha", "score"))
	}
	if sp("effective", "host_speeds", "hb", "score") != float64(1) || sp("sources", "host_speeds", "hb", "score") != "global" {
		t.Errorf("le groupe hb n'est pas touché : %v", sp("effective", "host_speeds", "hb", "score"))
	}
	e.ready(t, "4") // hors manche
	if got := tw290Path(e.settings(t), "effective", "light", "speeds", "chrono"); got != float64(1) {
		t.Errorf("sortie de manche : retour au global (×1) : %v", got)
	}
}

func TestTW223_Defaults_SpeedsOneEverywhere_AlwaysSerialisedInAmbianceEffective(t *testing.T) {
	e := tw223Env(t)
	tw223HostGroups(t)
	m := e.settings(t)
	for _, mo := range []string{"chrono", "calm", "score"} {
		if got := tw290Path(m, "effective", "light", "speeds", mo); got != float64(1) {
			t.Errorf("speeds.%s par défaut ×1 : %v", mo, got)
		}
		for _, id := range []string{"ha", "hb"} {
			if got := tw290Path(m, "effective", "light", "host_speeds", id, mo); got != float64(1) {
				t.Errorf("host_speeds.%s.%s par défaut ×1 : %v", id, mo, got)
			}
		}
	}
	ae := tw290Path(e.gameJSON(t), "AMBIANCE_EFFECTIVE", "SETTINGS", "light", "speeds")
	if mm, ok := ae.(map[string]interface{}); !ok || len(mm) != 3 {
		t.Errorf("AMBIANCE_EFFECTIVE.SETTINGS.light.speeds toujours sérialisé (3 moments) : %v", ae)
	}
}

func TestTW223_GlobalSpeedFromConfig_IsTheEffective_RoundOverrideBeatsIt(t *testing.T) {
	e := tw223Env(t)
	if code, out := e.post(t, "/config.json", `{"lighting":{"speeds":{"calm":0.5}}}`); code != 200 {
		t.Fatalf("%d %v", code, out)
	}
	if got := tw290Path(e.settings(t), "effective", "light", "speeds", "calm"); got != 0.5 {
		t.Errorf("le global écrit par /config.json est l'effectif : %v", got)
	}
	e.twoRounds(t, tw223AmbSpeeds(`{"calm":2}`, `{}`))
	e.ready(t, "1")
	if got := tw290Path(e.settings(t), "effective", "light", "speeds", "calm"); got != float64(2) {
		t.Errorf("la manche l'emporte sur le global : %v", got)
	}
	e.ready(t, "4")
	if got := tw290Path(e.settings(t), "effective", "light", "speeds", "calm"); got != 0.5 {
		t.Errorf("sortie de manche : retour au global (×0,5) : %v", got)
	}
}
