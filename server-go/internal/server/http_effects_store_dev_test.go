package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"buzzcontrol/internal/effects"
)

// #305 B1.2 — download / import / DELETE routes (dev tests).

type stStoreProv struct{ devEffectsProv }

func (p stStoreProv) EffectsSource(id string) ([]byte, effects.Entry, error) { return p.m.Source(id) }
func (p stStoreProv) EffectsDelete(id string) (effects.Catalogue, error)     { return p.m.Delete(id) }
func (p stStoreProv) EffectsImport(f []effects.ImportFile, c string) ([]effects.ImportResult, effects.Catalogue) {
	return p.m.Import(f, c)
}

func stStoreServer(t *testing.T) (*HTTPServer, string) {
	t.Helper()
	srv, _ := setupTestHTTPServer(t)
	dir := filepath.Join(t.TempDir(), "effects")
	native := fstest.MapFS{"pul.yaml": {Data: []byte(devEff("pul", "chrono"))}}
	srv.Effects = stStoreProv{devEffectsProv{m: effects.NewManager(native, dir, nil)}}
	return srv, dir
}

type stPart struct{ field, name, body string }

func stMultipart(t *testing.T, parts ...stPart) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range parts {
		if p.name == "" {
			_ = mw.WriteField(p.field, p.body)
			continue
		}
		fw, _ := mw.CreateFormFile(p.field, p.name)
		_, _ = fw.Write([]byte(p.body))
	}
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

func stDo(srv *HTTPServer, method, path string, body *bytes.Buffer, ct string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, path, body)
		r.Header.Set("Content-Type", ct)
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, r)
	return w
}

func TestAPIEffects_DownloadNativeAndUser_Dev(t *testing.T) {
	srv, _ := stStoreServer(t)
	w := stDo(srv, "GET", "/api/lighting/effects/pul/download", nil, "")
	if w.Code != 200 || w.Body.String() != devEff("pul", "chrono") ||
		w.Header().Get("Content-Disposition") != `attachment; filename="pul.yaml"` || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/yaml") {
		t.Fatalf("download natif : %d %v %q", w.Code, w.Header(), w.Body.String())
	}
	if w := stDo(srv, "GET", "/api/lighting/effects/inconnu/download", nil, ""); w.Code != 404 {
		t.Fatalf("inconnu : %d", w.Code)
	}
}

func TestAPIEffects_ImportThenDelete_Dev(t *testing.T) {
	srv, dir := stStoreServer(t)
	body, ct := stMultipart(t,
		stPart{"files", "../mon.yaml", devEff("mon", "calm")},
		stPart{"files", "pul.yaml", devEff("pul", "chrono")},
		stPart{"files", "casse.yml", "id: x\nnom: [\n"})
	w := stDo(srv, "POST", "/api/lighting/effects/import", body, ct)
	if w.Code != 200 {
		t.Fatalf("import : %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Results []struct{ Status, Stored string } `json:"results"`
		Cat     struct {
			Effects []struct{ ID string } `json:"effects"`
		} `json:"catalogue"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 3 || out.Results[0].Status != "imported" || out.Results[0].Stored != "mon.yaml" || out.Results[1].Status != "native" || out.Results[2].Status != "invalid" {
		t.Fatalf("résultats : %s", w.Body.String())
	}
	if len(out.Cat.Effects) != 2 {
		t.Fatalf("catalogue : %s", w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "mon.yaml")); err != nil {
		t.Fatal(err)
	}
	if w := stDo(srv, "DELETE", "/api/lighting/effects/pul", nil, ""); w.Code != 409 || !strings.Contains(w.Body.String(), `"native"`) {
		t.Fatalf("delete natif : %d %s", w.Code, w.Body.String())
	}
	if w := stDo(srv, "DELETE", "/api/lighting/effects/nope", nil, ""); w.Code != 404 {
		t.Fatalf("delete inconnu : %d", w.Code)
	}
	if w := stDo(srv, "DELETE", "/api/lighting/effects/mon", nil, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("delete : %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "mon.yaml")); err == nil {
		t.Fatal("fichier non supprimé")
	}
}

func TestAPIEffects_ImportBadRequests_Dev(t *testing.T) {
	srv, _ := stStoreServer(t)
	cases := map[string][]stPart{
		"aucun fichier":  {{field: "on_conflict", body: "ask"}},
		"extension":      {{"files", "x.txt", "a"}},
		"on_conflict":    {{"files", "x.yaml", "a"}, {field: "on_conflict", body: "zzz"}},
		"21 fichiers":    nil,
		"champ sans nom": {{field: "files", body: "a"}},
	}
	for i := 0; i < 21; i++ {
		cases["21 fichiers"] = append(cases["21 fichiers"], stPart{"files", "f.yaml", "a"})
	}
	for name, parts := range cases {
		body, ct := stMultipart(t, parts...)
		if w := stDo(srv, "POST", "/api/lighting/effects/import", body, ct); w.Code != 400 {
			t.Errorf("%s : %d", name, w.Code)
		}
	}
	if w := stDo(srv, "POST", "/api/lighting/effects/import", bytes.NewBufferString("{}"), "application/json"); w.Code != 400 {
		t.Errorf("non multipart : %d", w.Code)
	}
	if w := stDo(srv, "GET", "/api/lighting/effects/import", nil, ""); w.Code != 405 {
		t.Errorf("GET import : %d", w.Code)
	}
}

func TestAPIEffects_ImportTooLarge_Dev(t *testing.T) {
	srv, _ := stStoreServer(t)
	big := strings.Repeat("#", 60<<10)
	var parts []stPart
	for i := 0; i < 20; i++ {
		parts = append(parts, stPart{"files", "f.yaml", big})
	}
	// 20 x 60 KB = 1.2 MB: accepted (each file ok in size), answered per file.
	body, ct := stMultipart(t, parts...)
	if w := stDo(srv, "POST", "/api/lighting/effects/import", body, ct); w.Code != 200 {
		t.Fatalf("1,2 Mo : %d", w.Code)
	}
	huge := strings.Repeat("#", 600<<10)
	parts = nil
	for i := 0; i < 4; i++ { // 2.4 MB, each file too big on its own
		parts = append(parts, stPart{"files", "f.yaml", huge})
	}
	body, ct = stMultipart(t, parts...)
	if w := stDo(srv, "POST", "/api/lighting/effects/import", body, ct); w.Code != 413 {
		t.Fatalf("2,4 Mo : %d", w.Code)
	}
}
