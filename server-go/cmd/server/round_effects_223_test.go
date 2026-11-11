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

func TestTW223_RoundsUpdate_ValidEffects_StoredInRoundsJson(t *testing.T) {
	e := tw223Env(t)
	e.twoRounds(t, tw223Amb(`{"chrono":"chenillard","calm":""}`, `{"score":"couleur-equipe"}`))
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
	if tw290Path(amb, "light", "host_effects", "score") != "couleur-equipe" {
		t.Errorf("light.host_effects.score conservé : %v", rs[0]["AMBIANCE"])
	}
	if tw290Path(amb, "light", "effects", "score") != nil {
		t.Errorf("seules les clés surchargées sont stockées : %v", rs[0]["AMBIANCE"])
	}
}

func TestTW223_RoundsUpdate_InvalidEffects_RefusedAtomically(t *testing.T) {
	cases := map[string]interface{}{
		"id inconnu":                         tw223Amb(`{"chrono":"n-existe-pas"}`, `{}`),
		"effet qui ne déclare pas le moment": tw223Amb(`{"calm":"chenillard"}`, `{}`),
		"animateur : id inconnu":             tw223Amb(`{}`, `{"calm":"n-existe-pas"}`),
		"animateur : mauvais moment":         tw223Amb(`{}`, `{"chrono":"couleur-equipe"}`),
		"moment inconnu":                     tw223From(`{"light":{"effects":{"bidule":"chenillard"}}}`),
		"un bon et un mauvais":               tw223Amb(`{"chrono":"chenillard","calm":"chenillard"}`, `{}`),
	}
	for name, amb := range cases {
		t.Run(name, func(t *testing.T) {
			e := tw223Env(t)
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
	e.twoRounds(t, tw223Amb(`{"chrono":"chenillard"}`, `{"score":"couleur-equipe"}`))
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
	if eff("host_effects", "score") != "couleur-equipe" || src("host_effects", "score") != "round" {
		t.Errorf("host_effects.score surchargé : %v / %v", eff("host_effects", "score"), src("host_effects", "score"))
	}
	if eff("host_effects", "chrono") != "" || src("host_effects", "chrono") != "global" {
		t.Errorf("host_effects.chrono hérité : %v / %v", eff("host_effects", "chrono"), src("host_effects", "chrono"))
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
	m := e.settings(t)
	want := map[string]string{"chrono": "pulsation", "calm": "", "score": ""}
	for moment, v := range want {
		if got := tw290Path(m, "effective", "light", "effects", moment); got != v {
			t.Errorf("effects.%s par défaut : %v, attendu %q", moment, got, v)
		}
		if src := tw290Path(m, "sources", "light", "effects", moment); src != "global" {
			t.Errorf("source de effects.%s : %v", moment, src)
		}
		if got := tw290Path(m, "effective", "light", "host_effects", moment); got != "" {
			t.Errorf("host_effects.%s par défaut : blanc fixe (\"\"), obtenu %v", moment, got)
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
