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

func TestAPIEffects_CatalogueShapeAndReload_Dev(t *testing.T) {
	srv, dir := devEffectsServer(t)
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
	list := v["effects"].([]any)
	if len(list) != 2 || v["dir"] != "data/files/effects" || v["errors"] == nil || v["unresolved"] == nil {
		t.Fatalf("forme §7.1 : %+v", v)
	}
	e := list[0].(map[string]any)
	if _, hasTarget := e["target"]; hasTarget || e["min_groups"].(float64) != 0 || e["applicable"] != true || e["host_applicable"] != true || e["origin"] != "native" {
		t.Fatalf("entrée target all : %+v", e)
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

func TestConfig_LightingEffects_PartialPatchesMergeAndValidate_Dev(t *testing.T) {
	srv, _ := devEffectsServer(t)
	if code, out := devPostCfg(t, srv, `{"lighting":{"effects":{"calm":"cal"}}}`); code != 200 {
		t.Fatalf("patch calm : %d %s", code, out)
	}
	if code, out := devPostCfg(t, srv, `{"lighting":{"effects":{"chrono":"pul"}}}`); code != 200 {
		t.Fatalf("patch chrono : %d %s", code, out)
	}
	eff := config.Get().Lighting.Effects
	if eff["calm"] != "cal" || eff["chrono"] != "pul" || eff["score"] != "" {
		t.Fatalf("un patch d'un moment n'écrase pas les autres : %+v", eff)
	}
	if code, out := devPostCfg(t, srv, `{"lighting":{"lights":[{"name":"H1","role":"general"}],"groups":[{"id":"p1","role":"animateur","lights":["H1"]}]}}`); code != 200 {
		t.Fatalf("groupe animateur : %d %s", code, out)
	}
	if code, out := devPostCfg(t, srv, `{"lighting":{"host_effects":{"p1":{"calm":"cal"},"ghost":{"calm":"cal"}}}}`); code != 200 || config.Get().Lighting.HostEffects["p1"]["calm"] != "cal" {
		t.Fatalf("host_effects : %d %s", code, out)
	}
	if _, ok := config.Get().Lighting.HostEffects["ghost"]; ok {
		t.Fatal("un id inconnu est écarté sans erreur")
	}
	if code, _ := devPostCfg(t, srv, `{"lighting":{"host_effects":{"calm":"cal"}}}`); code != http.StatusBadRequest {
		t.Fatal("l'ancienne forme plate n'est plus acceptée à l'écriture")
	}
	for name, body := range map[string]string{
		"id inconnu":      `{"lighting":{"effects":{"calm":"nope"}}}`,
		"mauvais moment":  `{"lighting":{"effects":{"calm":"pul"}}}`,
		"moment inconnu":  `{"lighting":{"effects":{"zzz":""}}}`,
		"hôte id inconnu": `{"lighting":{"host_effects":{"p1":{"score":"nope"}}}}`,
	} {
		if code, _ := devPostCfg(t, srv, body); code != http.StatusBadRequest {
			t.Errorf("%s : 400 attendu, obtenu %d", name, code)
		}
	}
	if config.Get().Lighting.Effects["calm"] != "cal" {
		t.Fatal("un refus ne modifie rien")
	}
	// Deleting the group purges its effects.
	if code, _ := devPostCfg(t, srv, `{"lighting":{"groups":[]}}`); code != 200 || len(config.Get().Lighting.HostEffects) != 0 {
		t.Fatalf("la purge suit la suppression du groupe : %+v", config.Get().Lighting.HostEffects)
	}
	// "" clears back to the default of the moment's "no effect".
	if code, _ := devPostCfg(t, srv, `{"lighting":{"effects":{"calm":""}}}`); code != 200 || config.Get().Lighting.Effects["calm"] != "" {
		t.Fatal("\"\" est valide")
	}
}

func TestBackup_EffectsDirWithoutNatifs_Dev(t *testing.T) {
	if !isEffectsReferencePath("files/effects/natifs/a.yaml") || !isEffectsReferencePath("effects/natifs") || isEffectsReferencePath("files/effects/mon.yaml") || isEffectsReferencePath("files/effects/natifsX/a.yaml") {
		t.Fatal("isEffectsReferencePath")
	}
}

func TestConfig_GroupsRefuseNameAndNearTeams_Dev(t *testing.T) {
	srv, _ := devEffectsServer(t)
	devPostCfg(t, srv, `{"lighting":{"lights":[{"name":"L1","role":"general"}]}}`)
	for _, body := range []string{
		`{"lighting":{"groups":[{"name":"Scène","role":"salle","lights":["L1"]}]}}`,
		`{"lighting":{"groups":[{"role":"salle","lights":["L1"],"near_teams":["Rouges"]}]}}`,
	} {
		if code, _ := devPostCfg(t, srv, body); code != http.StatusBadRequest {
			t.Errorf("name / near_teams refusés à l'écriture : %d pour %s", code, body)
		}
	}
	if code, out := devPostCfg(t, srv, `{"lighting":{"groups":[{"role":"salle","lights":["L1"]}]}}`); code != 200 {
		t.Fatalf("un groupe n'a plus que id, role et lights : %d %s", code, out)
	}
}

func TestConfig_LightingSpeeds_PartialMergeValidateAndPurge_Dev(t *testing.T) {
	srv, _ := devEffectsServer(t)
	if code, out := devPostCfg(t, srv, `{"lighting":{"lights":[{"name":"H1","role":"general"}],"groups":[{"id":"p1","role":"animateur","lights":["H1"]}]}}`); code != 200 {
		t.Fatalf("groupe animateur : %d %s", code, out)
	}
	if code, out := devPostCfg(t, srv, `{"lighting":{"speeds":{"chrono":2}}}`); code != 200 {
		t.Fatalf("speeds : %d %s", code, out)
	}
	if code, _ := devPostCfg(t, srv, `{"lighting":{"speeds":{"calm":0.5}}}`); code != 200 {
		t.Fatal("un patch d'un moment")
	}
	sp := config.Get().Lighting.Speeds
	if sp["chrono"] != 2 || sp["calm"] != 0.5 || sp["score"] != 1 {
		t.Fatalf("fusion par moment, défaut ×1 : %+v", sp)
	}
	if code, _ := devPostCfg(t, srv, `{"lighting":{"host_speeds":{"p1":{"score":1.5},"ghost":{"score":2}}}}`); code != 200 {
		t.Fatal("host_speeds")
	}
	hs := config.Get().Lighting.HostSpeeds
	if hs["p1"]["score"] != 1.5 || len(hs) != 1 {
		t.Fatalf("id inconnu écarté sans erreur : %+v", hs)
	}
	for _, body := range []string{`{"lighting":{"speeds":{"chrono":3}}}`, `{"lighting":{"speeds":{"nuit":1}}}`, `{"lighting":{"host_speeds":{"p1":{"calm":0.6}}}}`} {
		if code, _ := devPostCfg(t, srv, body); code != http.StatusBadRequest {
			t.Errorf("400 attendu : %s", body)
		}
	}
	if config.Get().Lighting.Speeds["chrono"] != 2 {
		t.Fatal("un refus ne modifie rien")
	}
	if code, _ := devPostCfg(t, srv, `{"lighting":{"groups":[]}}`); code != 200 || len(config.Get().Lighting.HostSpeeds) != 0 {
		t.Fatalf("la suppression du groupe purge ses vitesses : %+v", config.Get().Lighting.HostSpeeds)
	}
}
