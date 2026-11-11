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
name: Effet %ID%
version: 1
moments: [%M%]
target: all
looks: { a: { color: scene } }
pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }
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
	if e["target"] != "all" || e["min_groups"].(float64) != 0 || e["applicable"] != true || e["host_applicable"] != true || e["origin"] != "native" {
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
	if code, out := devPostCfg(t, srv, `{"lighting":{"host_effects":{"calm":"cal"}}}`); code != 200 || config.Get().Lighting.HostEffects["calm"] != "cal" {
		t.Fatalf("host_effects : %d %s", code, out)
	}
	for name, body := range map[string]string{
		"id inconnu":      `{"lighting":{"effects":{"calm":"nope"}}}`,
		"mauvais moment":  `{"lighting":{"effects":{"calm":"pul"}}}`,
		"moment inconnu":  `{"lighting":{"effects":{"zzz":""}}}`,
		"hôte id inconnu": `{"lighting":{"host_effects":{"score":"nope"}}}`,
	} {
		if code, _ := devPostCfg(t, srv, body); code != http.StatusBadRequest {
			t.Errorf("%s : 400 attendu, obtenu %d", name, code)
		}
	}
	if config.Get().Lighting.Effects["calm"] != "cal" {
		t.Fatal("un refus ne modifie rien")
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
