package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/effects"
)

// #223 Batch 2.2 — catalogue API, per-moment settings in POST /config.json, backup rules.

type devEffectsProv struct{ m *effects.Manager }

func (p devEffectsProv) EffectsCatalogue() effects.Catalogue { return p.m.Catalogue() }
func (p devEffectsProv) EffectsReload() effects.Catalogue    { return p.m.Reload() }
func (p devEffectsProv) EffectsDir() string                  { return "data/files/effects" }

const devEffSrc = `id: %ID%
nom: Effet %ID%
version: 2
moments: [%M%]
duree_tick_ms: 1000
pas:
  - { g0: [scene, 100] }
`

func devEff(id, moment string) string {
	return strings.NewReplacer("%ID%", id, "%M%", moment).Replace(devEffSrc)
}

func devEffectsServer(t *testing.T) (*HTTPServer, string) {
	t.Helper()
	srv, _ := setupTestHTTPServer(t)
	dir := t.TempDir()
	native := fstest.MapFS{
		"pul.yaml": {Data: []byte(devEff("pul", "chrono"))},
		"cal.yaml": {Data: []byte(devEff("cal", "calm"))},
	}
	srv.Effects = devEffectsProv{m: effects.NewManager(native, dir, nil)}
	return srv, dir
}

func devPostCfg(t *testing.T, srv *HTTPServer, body string) (int, string) {
	t.Helper()
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, httptest.NewRequest("POST", "/config.json", strings.NewReader(body)))
	return w.Code, w.Body.String()
}

func TestAPIEffects_CatalogueByZoneAndReload_Dev(t *testing.T) {
	srv, dir := devEffectsServer(t)
	devPostCfg(t, srv, `{"lighting":{"lights":[{"name":"L1","role":"general"},{"name":"P1","role":"general"}],"groups":[{"role":"salle","lights":["L1"]},{"role":"public","lights":["P1"]}]}}`)
	get := func(method, path string) map[string]any {
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s %s : %d", method, path, w.Code)
		}
		var m map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	v := get("GET", "/api/lighting/effects")
	groups := v["groups"].(map[string]any)
	if groups["ambiance"].(float64) != 1 || groups["animateur"].(float64) != 0 || groups["public"].(float64) != 1 {
		t.Fatalf("groups par zone : %+v", groups)
	}
	list := v["effects"].([]any)
	if len(list) != 2 || v["dir"] != "data/files/effects" || v["errors"] == nil || v["unresolved"] == nil {
		t.Fatalf("forme §7.1 : %+v", v)
	}
	e := list[0].(map[string]any)
	app := e["applicable"].(map[string]any)
	if e["tick_ms"].(float64) != 1000 || e["min_groups"].(float64) != 0 || app["ambiance"] != true || app["public"] != true || app["animateur"] != false {
		t.Fatalf("applicabilité par zone (animateur n'existe pas) et tick_ms : %+v", e)
	}
	for _, gone := range []string{"target", "reason", "host_applicable"} {
		if _, has := e[gone]; has {
			t.Errorf("champ %q supprimé en révision 4", gone)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("id: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	v = get("POST", "/api/lighting/effects/reload")
	if errs := v["errors"].([]any); len(errs) != 1 || errs[0].(map[string]any)["file"] != "bad.yaml" {
		t.Fatalf("erreur de fichier attendue après rechargement : %+v", v["errors"])
	}
	if w := httptest.NewRecorder(); func() int {
		srv.mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/lighting/effects/reload", nil))
		return w.Code
	}() != http.StatusMethodNotAllowed {
		t.Fatal("reload est un POST")
	}
}

func TestConfig_LightingEffectsAndSpeeds_ByZoneMergeAndValidate_Dev(t *testing.T) {
	srv, _ := devEffectsServer(t)
	if code, out := devPostCfg(t, srv, `{"lighting":{"effects":{"ambiance":{"calm":"cal"}}}}`); code != 200 {
		t.Fatalf("patch ambiance.calm : %d %s", code, out)
	}
	if code, out := devPostCfg(t, srv, `{"lighting":{"effects":{"public":{"chrono":"pul"}}}}`); code != 200 {
		t.Fatalf("patch public.chrono : %d %s", code, out)
	}
	eff := config.Get().Lighting.Effects
	if eff["ambiance"]["calm"] != "cal" || eff["public"]["chrono"] != "pul" || eff["ambiance"]["chrono"] != "pulsation" || eff["animateur"]["score"] != "" {
		t.Fatalf("un patch d'un moment d'une zone n'écrase rien d'autre, tables complètes : %+v", eff)
	}
	if code, _ := devPostCfg(t, srv, `{"lighting":{"speeds":{"animateur":{"score":1.5}}}}`); code != 200 {
		t.Fatal("speeds par zone")
	}
	if code, _ := devPostCfg(t, srv, `{"lighting":{"speeds":{"public":{"calm":0.5}}}}`); code != 200 {
		t.Fatal("speeds : un autre patch")
	}
	sp := config.Get().Lighting.Speeds
	if sp["animateur"]["score"] != 1.5 || sp["public"]["calm"] != 0.5 || sp["ambiance"]["chrono"] != 1 {
		t.Fatalf("fusion par zone et par moment, ×1 par défaut : %+v", sp)
	}
	for name, body := range map[string]string{
		"effet inconnu":           `{"lighting":{"effects":{"ambiance":{"calm":"nope"}}}}`,
		"effet d'un autre moment": `{"lighting":{"effects":{"ambiance":{"calm":"pul"}}}}`,
		"zone inconnue":           `{"lighting":{"effects":{"scène":{"calm":""}}}}`,
		"moment inconnu":          `{"lighting":{"effects":{"public":{"nuit":""}}}}`,
		"forme à plat":            `{"lighting":{"effects":{"calm":"cal"}}}`,
		"host_effects":            `{"lighting":{"host_effects":{"g1":{"calm":"cal"}}}}`,
		"host_speeds":             `{"lighting":{"host_speeds":{"g1":{"calm":2}}}}`,
		"vitesse hors liste":      `{"lighting":{"speeds":{"public":{"calm":3}}}}`,
		"vitesse zone inconnue":   `{"lighting":{"speeds":{"scène":{"calm":1}}}}`,
	} {
		if code, _ := devPostCfg(t, srv, body); code != http.StatusBadRequest {
			t.Errorf("%s : 400 attendu, obtenu %d", name, code)
		}
	}
	if config.Get().Lighting.Effects["ambiance"]["calm"] != "cal" {
		t.Fatal("un refus ne modifie rien")
	}
	if code, _ := devPostCfg(t, srv, `{"lighting":{"effects":{"ambiance":{"calm":""}}}}`); code != 200 || config.Get().Lighting.Effects["ambiance"]["calm"] != "" {
		t.Fatal("\"\" est valide")
	}
}

func TestConfig_LegacyQualifEffectsFormsReadLeniently_Dev(t *testing.T) {
	var lc config.LightingConfig
	if err := json.Unmarshal([]byte(`{"effects":{"chrono":"pulsation","calm":"vague-lente"},"speeds":{"chrono":2},"host_effects":{"g1":{"calm":"x"}},"host_speeds":{"g1":{"calm":2}}}`), &lc); err != nil {
		t.Fatalf("lecture tolérante : %v", err)
	}
	if lc.Effects["ambiance"]["calm"] != "vague-lente" || lc.Speeds["ambiance"]["chrono"] != 2 || len(lc.Effects) != 1 {
		t.Fatalf("à plat = zone ambiance, host_* ignorés : %+v", lc)
	}
}

func TestConfig_GroupRolesAndCapsPerZone_Dev(t *testing.T) {
	srv, _ := devEffectsServer(t)
	lights := `"lights":[{"name":"L1","role":"general"},{"name":"L2","role":"general"}]`
	devPostCfg(t, srv, `{"lighting":{`+lights+`}}`)
	if code, out := devPostCfg(t, srv, `{"lighting":{"groups":[{"role":"public","lights":["L1"]},{"role":"animateur","lights":["L2"]}]}}`); code != 200 {
		t.Fatalf("rôle public accepté : %d %s", code, out)
	}
	if n := len(config.Get().Lighting.NonEmptyGroups(config.ZonePublic)); n != 1 {
		t.Fatalf("zone public : %d groupe(s)", n)
	}
	many := ""
	for i := 0; i < 13; i++ {
		many += `{"role":"public","lights":[]},`
	}
	if code, _ := devPostCfg(t, srv, `{"lighting":{"groups":[`+many[:len(many)-1]+`]}}`); code != http.StatusBadRequest {
		t.Fatal("13 groupes dans une zone : refusé")
	}
	if code, _ := devPostCfg(t, srv, `{"lighting":{"groups":[{"role":"scène","lights":[]}]}}`); code != http.StatusBadRequest {
		t.Fatal("rôle inconnu : refusé")
	}
}

func TestEffectTrialEndpoints_ValidationAndRefusals_Dev(t *testing.T) {
	srv, _ := devEffectsServer(t)
	post := func(path, body string) int {
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(body)))
		return w.Code
	}
	if c := post("/api/lighting/effects/cal/try", `{"zone":"scène"}`); c != http.StatusBadRequest {
		t.Errorf("zone invalide : %d", c)
	}
	if c := post("/api/lighting/effects/cal/try", `{"speed":3}`); c != http.StatusBadRequest {
		t.Errorf("vitesse invalide : %d", c)
	}
	if c := post("/api/lighting/effects/cal/try", `{"seconds":30}`); c != http.StatusBadRequest {
		t.Errorf("durée invalide : %d", c)
	}
	if c := post("/api/lighting/effects/nope/try", `{}`); c != http.StatusNotFound {
		t.Errorf("effet inconnu : %d", c)
	}
	if c := post("/api/lighting/effects/cal/try", ``); c != http.StatusConflict {
		t.Errorf("éclairage désactivé (aucun pilote) : %d", c)
	}
	if c := post("/api/lighting/effects/try/stop", ``); c != http.StatusOK {
		t.Errorf("try/stop sans essai : %d", c)
	}
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/lighting/effects/cal/try", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET sur try : %d", w.Code)
	}
}

func TestBackup_EffectsDirWithoutNatifs_Dev(t *testing.T) {
	if !isEffectsReferencePath("files/effects/natifs/a.yaml") || !isEffectsReferencePath("effects/natifs") || isEffectsReferencePath("files/effects/mon.yaml") || isEffectsReferencePath("files/effects/natifsX/a.yaml") {
		t.Fatal("isEffectsReferencePath")
	}
}
