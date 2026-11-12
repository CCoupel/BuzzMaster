package server

// #305 (v11.8.0, LOT-1) — routes DELETE /api/lighting/effects/{id}, GET …/{id}/download, POST …/import.
// Source : contracts/light-effects.md §11.4 (nom de fichier), §11.5, §11.6, §11.7, §11.9 ; non-régression §7.1, §7.2, §7.4.
// Écrits depuis le contrat (TDD) : peuvent échouer tant que B1.1 / B1.2 ne sont pas livrés.
//
// SURFACE ATTENDUE : le fournisseur de test embarque *effects.Manager (ses méthodes B1.1 sont promues) et ajoute les trois
// méthodes existantes de `EffectsProvider`. Si dev-backend ajoute d'autres méthodes à l'interface (ex. WriteEffect avec une
// autre signature), il n'adapte que `tw305Provider` ci-dessous ; les tests ne changent pas.
// Noms : TestTW305_* ; `critical` = TestTW305_Critical_*.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/effects"
)

type tw305Provider struct {
	*effects.Manager
	dir string
}

func (p *tw305Provider) EffectsCatalogue() effects.Catalogue { return p.Manager.Catalogue() }
func (p *tw305Provider) EffectsReload() effects.Catalogue    { return p.Manager.Reload() }
func (p *tw305Provider) EffectsDir() string                  { return p.dir }

func tw305Server(t *testing.T) (*HTTPServer, *tw305Provider, string) {
	t.Helper()
	srv, dataDir := setupTestHTTPServer(t)
	dir := filepath.Join(dataDir, "files", "effects")
	_, self, _, _ := runtime.Caller(0)
	natives := os.DirFS(filepath.Join(filepath.Dir(self), "..", "..", "assets", "effects"))
	if err := effects.EnsureDirs(natives, dir); err != nil {
		t.Fatal(err)
	}
	prov := &tw305Provider{Manager: effects.NewManager(natives, dir, nil), dir: dir}
	srv.Effects = prov
	config.Mutate(config.ApplyDefaults)
	return srv, prov, dir
}

func tw305Src(id, extra string) string {
	return "id: " + id + "\nnom: Effet " + id + "\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\n" + extra + "pas:\n  - { g1: [\"#FF0000\", 100] }\n"
}

type tw305File struct{ name, body string }

// tw305Import envoie un multipart (champ `files` répété) ; onConflict "" = champ absent.
func tw305Import(t *testing.T, srv *HTTPServer, onConflict string, files ...tw305File) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, f := range files {
		fw, err := mw.CreateFormFile("files", f.name)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte(f.body))
	}
	if onConflict != "" {
		mw.WriteField("on_conflict", onConflict)
	}
	mw.Close()
	req := httptest.NewRequest("POST", "/api/lighting/effects/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func tw305Results(t *testing.T, out map[string]any) []map[string]any {
	t.Helper()
	raw, ok := out["results"].([]any)
	if !ok {
		t.Fatalf("results[] absent : %v", out)
	}
	var rs []map[string]any
	for _, r := range raw {
		rs = append(rs, r.(map[string]any))
	}
	return rs
}

func tw305Do(srv *HTTPServer, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	return w
}

func tw305Names(t *testing.T, dir string) []string {
	t.Helper()
	ents, _ := os.ReadDir(dir)
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

func tw305Has(dir, name string) bool { _, err := os.Stat(filepath.Join(dir, name)); return err == nil }

// ---------------------------------------------------------------------------
// §11.6 download
// ---------------------------------------------------------------------------

func TestTW305_Download_User_ContentAndHeaders(t *testing.T) {
	srv, prov, dir := tw305Server(t)
	src := "# mon commentaire\n" + tw305Src("mon-effet", "")
	_ = os.WriteFile(filepath.Join(dir, "x.yaml"), []byte(src), 0o644)
	prov.Reload()
	w := tw305Do(srv, "GET", "/api/lighting/effects/mon-effet/download")
	if w.Code != 200 || w.Body.String() != src {
		t.Fatalf("download : %d %q", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/yaml; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); cd != `attachment; filename="mon-effet.yaml"` {
		t.Errorf("Content-Disposition = %q (nom = <id>.yaml, pas le nom du fichier)", cd)
	}
}

func TestTW305_Download_Native_NoReferenceHeader(t *testing.T) {
	srv, _, _ := tw305Server(t)
	w := tw305Do(srv, "GET", "/api/lighting/effects/pulsation/download")
	if w.Code != 200 {
		t.Fatalf("natif téléchargeable : %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "COPIE DE RÉFÉRENCE") || !strings.Contains(w.Body.String(), "id: pulsation") {
		t.Errorf("texte embarqué sans l'en-tête de natifs/ : %.120q", w.Body.String())
	}
}

func TestTW305_Download_Unknown_404(t *testing.T) {
	srv, _, _ := tw305Server(t)
	if w := tw305Do(srv, "GET", "/api/lighting/effects/inconnu/download"); w.Code != 404 {
		t.Errorf("404 attendu, obtenu %d", w.Code)
	}
}

func TestTW305_Download_PostNotAllowed(t *testing.T) {
	srv, _, _ := tw305Server(t)
	if w := tw305Do(srv, "POST", "/api/lighting/effects/pulsation/download"); w.Code != 405 && w.Code != 404 {
		t.Errorf("POST sur download refusé : %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// §11.5 DELETE
// ---------------------------------------------------------------------------

func TestTW305_Delete_User_200WithCatalogue(t *testing.T) {
	srv, prov, dir := tw305Server(t)
	_ = os.WriteFile(filepath.Join(dir, "mon-effet.yaml"), []byte(tw305Src("mon-effet", "")), 0o644)
	prov.Reload()
	w := tw305Do(srv, "DELETE", "/api/lighting/effects/mon-effet")
	var out struct {
		OK        bool           `json:"ok"`
		Catalogue map[string]any `json:"catalogue"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != 200 || !out.OK || out.Catalogue["effects"] == nil {
		t.Fatalf("DELETE : %d %s", w.Code, w.Body.String())
	}
	if tw305Has(dir, "mon-effet.yaml") {
		t.Error("fichier supprimé")
	}
	if strings.Contains(w.Body.String(), `"id":"mon-effet"`) {
		t.Error("le catalogue renvoyé est à jour (effet absent)")
	}
}

func TestTW305_Delete_Override_NativeResumes(t *testing.T) {
	srv, prov, dir := tw305Server(t)
	_ = os.WriteFile(filepath.Join(dir, "degrade.yaml"), []byte(tw305Src("degrade", "description: surcharge\n")), 0o644)
	prov.Reload()
	w := tw305Do(srv, "DELETE", "/api/lighting/effects/degrade")
	if w.Code != 200 {
		t.Fatalf("DELETE surcharge : %d %s", w.Code, w.Body.String())
	}
	if tw305Has(dir, "degrade.yaml") {
		t.Error("fichier de surcharge supprimé")
	}
	e, ok := prov.Catalogue().Get("degrade")
	if !ok || e.Origin != effects.OriginNative {
		t.Errorf("le natif reprend : %+v", e)
	}
}

func TestTW305_Critical_Delete_Native_409(t *testing.T) {
	srv, prov, dir := tw305Server(t)
	w := tw305Do(srv, "DELETE", "/api/lighting/effects/pulsation")
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != 409 || out["reason"] != "native" {
		t.Errorf("natif → 409 {reason: native} : %d %s", w.Code, w.Body.String())
	}
	if !tw305Has(filepath.Join(dir, "natifs"), "pulsation.yaml") {
		t.Error("la copie de référence reste")
	}
	if _, ok := prov.Catalogue().Get("pulsation"); !ok {
		t.Error("le natif reste")
	}
}

func TestTW305_Delete_Unknown_404(t *testing.T) {
	srv, _, _ := tw305Server(t)
	if w := tw305Do(srv, "DELETE", "/api/lighting/effects/inconnu"); w.Code != 404 {
		t.Errorf("404 attendu, obtenu %d", w.Code)
	}
}

func TestTW305_Delete_SettingsUntouched_UnresolvedReported(t *testing.T) {
	srv, prov, dir := tw305Server(t)
	_ = os.WriteFile(filepath.Join(dir, "mon-effet.yaml"), []byte(tw305Src("mon-effet", "")), 0o644)
	prov.Reload()
	code, out := tw223Post(t, srv, `{"lighting":{"effects":{"ambiance":{"calm":"mon-effet"}}}}`)
	if code != 200 {
		t.Fatalf("réglage : %d %s", code, out)
	}
	if w := tw305Do(srv, "DELETE", "/api/lighting/effects/mon-effet"); w.Code != 200 {
		t.Fatalf("DELETE : %d", w.Code)
	}
	if got := tw223Lighting(t, srv).Effects["ambiance"]["calm"]; got != "mon-effet" {
		t.Errorf("les réglages ne sont pas modifiés : %q", got)
	}
	w := tw305Do(srv, "GET", "/api/lighting/effects")
	if !strings.Contains(w.Body.String(), `"unresolved"`) || !strings.Contains(w.Body.String(), "mon-effet") {
		t.Errorf("unresolved[] renseigné : %s", w.Body.String())
	}
}

func TestTW305_Delete_DoesNotMatchSubRoutes(t *testing.T) {
	srv, _, _ := tw305Server(t)
	for _, p := range []string{"/api/lighting/effects/try/stop", "/api/lighting/effects/import", "/api/lighting/effects/reload"} {
		if w := tw305Do(srv, "DELETE", p); w.Code == 200 {
			t.Errorf("DELETE %s ne supprime rien et ne répond pas 200", p)
		}
	}
}

// ---------------------------------------------------------------------------
// §11.7 import — un test par status
// ---------------------------------------------------------------------------

func TestTW305_Import_Imported_WrittenAsIs_CatalogueUpdated(t *testing.T) {
	srv, prov, dir := tw305Server(t)
	src := "# commentaire\n" + tw305Src("vague-douce", "")
	code, out := tw305Import(t, srv, "", tw305File{"vague-douce.yaml", src})
	if code != 200 {
		t.Fatalf("import : %d %v", code, out)
	}
	r := tw305Results(t, out)
	if len(r) != 1 || r[0]["status"] != "imported" || r[0]["id"] != "vague-douce" || r[0]["stored"] != "vague-douce.yaml" || r[0]["file"] != "vague-douce.yaml" {
		t.Fatalf("résultat : %v", r)
	}
	if r[0]["name"] != "Effet vague-douce" {
		t.Errorf("name = %v", r[0]["name"])
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "vague-douce.yaml")); string(b) != src {
		t.Errorf("écrit tel quel (commentaires compris) : %q", b)
	}
	if _, ok := prov.Catalogue().Get("vague-douce"); !ok {
		t.Error("catalogue rechargé")
	}
	cat, _ := out["catalogue"].(map[string]any)
	if cat == nil || !strings.Contains(fmt.Sprint(cat["effects"]), "vague-douce") {
		t.Errorf("catalogue de la réponse à jour : %v", out["catalogue"])
	}
}

func TestTW305_Import_YmlExtensionAccepted(t *testing.T) {
	srv, _, _ := tw305Server(t)
	_, out := tw305Import(t, srv, "", tw305File{"x.yml", tw305Src("via-yml", "")})
	if r := tw305Results(t, out); r[0]["status"] != "imported" {
		t.Errorf(".yml accepté : %v", r)
	}
}

func TestTW305_Import_Conflict_Ask_NothingWritten(t *testing.T) {
	srv, prov, dir := tw305Server(t)
	orig := tw305Src("mon-effet", "description: original\n")
	_ = os.WriteFile(filepath.Join(dir, "mon-effet.yaml"), []byte(orig), 0o644)
	prov.Reload()
	for _, oc := range []string{"", "ask"} {
		_, out := tw305Import(t, srv, oc, tw305File{"copie.yaml", tw305Src("mon-effet", "description: neuf\n")})
		r := tw305Results(t, out)
		if r[0]["status"] != "conflict" || r[0]["id"] != "mon-effet" {
			t.Fatalf("on_conflict=%q : %v", oc, r)
		}
		ex, _ := r[0]["existing"].(map[string]any)
		if ex["file"] != "mon-effet.yaml" || ex["name"] != "Effet mon-effet" {
			t.Errorf("existing = %v", ex)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "mon-effet.yaml")); string(b) != orig {
		t.Error("conflict : rien n'est écrit")
	}
}

func TestTW305_Import_Replace_KeepsExistingFileName(t *testing.T) {
	srv, prov, dir := tw305Server(t)
	_ = os.WriteFile(filepath.Join(dir, "nom-libre.yaml"), []byte(tw305Src("mon-effet", "")), 0o644)
	prov.Reload()
	neuf := tw305Src("mon-effet", "description: neuf\n")
	_, out := tw305Import(t, srv, "replace", tw305File{"copie.yaml", neuf})
	r := tw305Results(t, out)
	if r[0]["status"] != "replaced" {
		t.Fatalf("%v", r)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "nom-libre.yaml")); string(b) != neuf {
		t.Error("remplacé dans son fichier, tel quel")
	}
	if tw305Has(dir, "mon-effet.yaml") || tw305Has(dir, "copie.yaml") {
		t.Errorf("aucun doublon : %v", tw305Names(t, dir))
	}
}

func TestTW305_Import_KeepBoth_OnlyIDKeyRewritten(t *testing.T) {
	srv, prov, dir := tw305Server(t)
	_ = os.WriteFile(filepath.Join(dir, "mon-effet.yaml"), []byte(tw305Src("mon-effet", "")), 0o644)
	prov.Reload()
	src := "# garde-moi\nid: mon-effet # ici\nnom: Mon effet\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g1: [\"#00FF00\", 100] } # fin\n"
	_, out := tw305Import(t, srv, "keep_both", tw305File{"copie.yaml", src})
	r := tw305Results(t, out)
	if r[0]["status"] != "renamed" || r[0]["new_id"] != "mon-effet-2" || r[0]["stored"] != "mon-effet-2.yaml" {
		t.Fatalf("%v", r)
	}
	want := strings.Replace(src, "id: mon-effet #", "id: mon-effet-2 #", 1)
	if b, _ := os.ReadFile(filepath.Join(dir, "mon-effet-2.yaml")); string(b) != want {
		t.Errorf("seule la valeur de la clé id change :\n%s", b)
	}
	// un second « garder les deux » prend le premier suffixe libre
	_, out = tw305Import(t, srv, "keep_both", tw305File{"copie.yaml", src})
	if r = tw305Results(t, out); r[0]["new_id"] != "mon-effet-3" {
		t.Errorf("premier suffixe libre : %v", r)
	}
}

func TestTW305_KeepBoth_SuffixedIDTruncatedTo40(t *testing.T) {
	srv, prov, dir := tw305Server(t)
	long := strings.Repeat("a", 40)
	_ = os.WriteFile(filepath.Join(dir, "l.yaml"), []byte(tw305Src(long, "")), 0o644)
	prov.Reload()
	_, out := tw305Import(t, srv, "keep_both", tw305File{"l.yaml", tw305Src(long, "")})
	id, _ := tw305Results(t, out)[0]["new_id"].(string)
	if id == "" || len(id) > 40 || !strings.HasSuffix(id, "-2") {
		t.Errorf("id suffixé tronqué à 40 : %q", id)
	}
}

func TestTW305_Critical_Import_NativeID_RefusedWhateverOnConflict(t *testing.T) {
	srv, prov, dir := tw305Server(t)
	for _, oc := range []string{"", "ask", "replace", "keep_both"} {
		_, out := tw305Import(t, srv, oc, tw305File{"pulsation.yaml", tw305Src("pulsation", "")})
		r := tw305Results(t, out)
		if r[0]["status"] != "native" {
			t.Errorf("on_conflict=%q : %v", oc, r)
		}
		errs, _ := r[0]["errors"].([]any)
		if len(errs) == 0 || !strings.Contains(fmt.Sprint(errs[0]), "identifiant d'un effet livré") {
			t.Errorf("message errors[] : %v", r[0]["errors"])
		}
	}
	for _, n := range tw305Names(t, dir) {
		if n != "natifs" {
			t.Errorf("rien n'est écrit : %q", n)
		}
	}
	if e, _ := prov.Catalogue().Get("pulsation"); e.Origin != effects.OriginNative {
		t.Error("le natif reste natif")
	}
}

func TestTW305_Critical_Import_OverriddenNativeID_Refused(t *testing.T) {
	srv, prov, dir := tw305Server(t)
	_ = os.WriteFile(filepath.Join(dir, "degrade.yaml"), []byte(tw305Src("degrade", "description: main\n")), 0o644)
	prov.Reload()
	_, out := tw305Import(t, srv, "replace", tw305File{"d.yaml", tw305Src("degrade", "description: import\n")})
	if r := tw305Results(t, out); r[0]["status"] != "native" {
		t.Errorf("natif surchargé : %v", r)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "degrade.yaml")); !strings.Contains(string(b), "description: main") {
		t.Error("la surcharge n'est pas modifiée")
	}
}

func TestTW305_Import_Invalid_ErrorsWithLine(t *testing.T) {
	srv, _, dir := tw305Server(t)
	bad := "id: casse\nnom: Casse\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { gB: [\"#FF0000\", 100] }\n"
	for _, f := range []tw305File{{"casse.yaml", bad}, {"illisible.yaml", "id: [\nnom: {"}, {"ancien.yaml", "id: ancien\nnom: A\nversion: 1\nmoments: [calm]\n"}, {"reserve.yaml", tw305Src("reload", "")}} {
		_, out := tw305Import(t, srv, "", f)
		r := tw305Results(t, out)
		errs, _ := r[0]["errors"].([]any)
		if r[0]["status"] != "invalid" || len(errs) == 0 {
			t.Errorf("%s : %v", f.name, r)
			continue
		}
		e := errs[0].(map[string]any)
		if _, ok := e["line"]; !ok || e["message"] == "" {
			t.Errorf("%s : errors[] porte line et message : %v", f.name, e)
		}
	}
	if n := tw305Names(t, dir); len(n) != 1 || n[0] != "natifs" {
		t.Errorf("rien n'est écrit : %v", n)
	}
}

func TestTW305_Import_TooBig_AndOthersContinue(t *testing.T) {
	srv, _, dir := tw305Server(t)
	big := tw305Src("gros", "description: "+strings.Repeat("a", 64*1024)+"\n")
	_, out := tw305Import(t, srv, "", tw305File{"gros.yaml", big}, tw305File{"ok.yaml", tw305Src("ok", "")})
	r := tw305Results(t, out)
	if r[0]["status"] != "too_big" || r[1]["status"] != "imported" {
		t.Errorf("un fichier refusé n'empêche pas les autres : %v", r)
	}
	if tw305Has(dir, "gros.yaml") {
		t.Error("too_big : rien d'écrit")
	}
}

func TestTW305_Import_Limit100(t *testing.T) {
	srv, prov, dir := tw305Server(t)
	for i := 0; i < 100; i++ {
		_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("e%d.yaml", i)), []byte(tw305Src(fmt.Sprintf("e%d", i), "")), 0o644)
	}
	prov.Reload()
	_, out := tw305Import(t, srv, "", tw305File{"de-trop.yaml", tw305Src("de-trop", "")})
	if r := tw305Results(t, out); r[0]["status"] != "limit" {
		t.Errorf("100 fichiers atteints : %v", r)
	}
	if tw305Has(dir, "de-trop.yaml") {
		t.Error("rien d'écrit")
	}
}

func TestTW305_Import_SameIDTwiceInOneUpload_SecondIsConflict(t *testing.T) {
	srv, _, dir := tw305Server(t)
	_, out := tw305Import(t, srv, "", tw305File{"a.yaml", tw305Src("doublon", "description: a\n")}, tw305File{"b.yaml", tw305Src("doublon", "description: b\n")})
	r := tw305Results(t, out)
	if r[0]["status"] != "imported" || r[1]["status"] != "conflict" {
		t.Fatalf("%v", r)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "doublon.yaml")); !strings.Contains(string(b), "description: a") {
		t.Error("le premier est écrit, le second non")
	}
}

func TestTW305_Import_ManyFiles_ResultsInOrder_SingleReloadEffect(t *testing.T) {
	srv, prov, _ := tw305Server(t)
	var fs []tw305File
	for i := 0; i < 20; i++ {
		fs = append(fs, tw305File{fmt.Sprintf("f%d.yaml", i), tw305Src(fmt.Sprintf("lot-%d", i), "")})
	}
	_, out := tw305Import(t, srv, "", fs...)
	r := tw305Results(t, out)
	if len(r) != 20 {
		t.Fatalf("20 résultats : %d", len(r))
	}
	for i, x := range r {
		if x["file"] != fmt.Sprintf("f%d.yaml", i) || x["status"] != "imported" {
			t.Errorf("résultat %d : %v", i, x)
		}
	}
	if _, ok := prov.Catalogue().Get("lot-19"); !ok {
		t.Error("tous visibles au catalogue")
	}
}

// --- 400 / 413 -------------------------------------------------------------

func TestTW305_Import_400_NoFile_TooMany_BadExtension_NotMultipart(t *testing.T) {
	srv, _, dir := tw305Server(t)
	if code, _ := tw305Import(t, srv, ""); code != 400 {
		t.Errorf("aucun fichier → 400, obtenu %d", code)
	}
	var many []tw305File
	for i := 0; i < 21; i++ {
		many = append(many, tw305File{fmt.Sprintf("f%d.yaml", i), tw305Src(fmt.Sprintf("m%d", i), "")})
	}
	if code, _ := tw305Import(t, srv, "", many...); code != 400 {
		t.Errorf("21 fichiers → 400, obtenu %d", code)
	}
	for _, name := range []string{"notes.txt", "effet.json", "sans-extension"} {
		if code, _ := tw305Import(t, srv, "", tw305File{name, tw305Src("ext", "")}); code != 400 {
			t.Errorf("extension %q → 400, obtenu %d", name, code)
		}
	}
	req := httptest.NewRequest("POST", "/api/lighting/effects/import", strings.NewReader("pas du multipart"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("envoi illisible → 400, obtenu %d", w.Code)
	}
	if n := tw305Names(t, dir); len(n) != 1 {
		t.Errorf("rien d'écrit sur 400 : %v", n)
	}
}

func TestTW305_Import_BadOnConflictValue_400(t *testing.T) {
	srv, _, _ := tw305Server(t)
	if code, _ := tw305Import(t, srv, "bizarre", tw305File{"a.yaml", tw305Src("a", "")}); code != 400 {
		t.Errorf("on_conflict inconnu → 400 (ou traité comme ask) : %d", code)
	}
}

func TestTW305_Import_413_Over2MBTotal(t *testing.T) {
	srv, _, dir := tw305Server(t)
	chunk := tw305Src("c", "description: "+strings.Repeat("a", 60*1024)+"\n")
	var fs []tw305File
	for i := 0; i < 20; i++ { // 20 × ~60 Ko = 1,2 Mo : sous la limite
		fs = append(fs, tw305File{fmt.Sprintf("f%d.yaml", i), chunk})
	}
	if code, _ := tw305Import(t, srv, "", fs[:5]...); code != 200 {
		t.Fatalf("préalable : %d", code)
	}
	huge := tw305File{"h.yaml", tw305Src("h", "description: "+strings.Repeat("a", 2*1024*1024+10)+"\n")}
	if code, _ := tw305Import(t, srv, "", huge); code != 413 {
		t.Errorf("> 2 Mo → 413, obtenu %d", code)
	}
	if tw305Has(dir, "h.yaml") {
		t.Error("rien d'écrit")
	}
}

// --- nom de fichier hostile, rien hors du dossier --------------------------

func TestTW305_Critical_Import_HostileFileNames_NeverUsedAsPath(t *testing.T) {
	srv, _, dir := tw305Server(t)
	root := filepath.Dir(filepath.Dir(dir)) // …/dataDir
	names := []string{"../x.yaml", "..\\x.yaml", "/etc/evil.yaml", "C:\\Windows\\evil.yaml", "natifs/pulsation.yaml", "sub/dir/ok.yaml", "CON.yaml", "NUL.yaml", "aux.yml", "a\x00b.yaml"}
	before := tw305Snapshot(t, root)
	for i, n := range names {
		id := fmt.Sprintf("hostile-%d", i)
		_, out := tw305Import(t, srv, "", tw305File{n, tw305Src(id, "")})
		if r, ok := out["results"].([]any); ok && len(r) == 1 {
			m := r[0].(map[string]any)
			if m["status"] == "imported" && m["stored"] != id+".yaml" {
				t.Errorf("%q : stored = %v (jamais dérivé du nom envoyé)", n, m["stored"])
			}
		}
	}
	after := tw305Snapshot(t, root)
	for p := range after {
		if _, ok := before[p]; ok {
			continue
		}
		rel, _ := filepath.Rel(dir, p)
		if strings.HasPrefix(rel, "..") || strings.HasPrefix(rel, "natifs") || strings.Contains(rel, string(filepath.Separator)) {
			t.Errorf("fichier créé hors de data/files/effects/ (ou dans natifs/ / un sous-dossier) : %s", p)
		}
	}
	for _, f := range []string{"x.yaml", "evil.yaml"} {
		if tw305Has(root, f) || tw305Has(filepath.Dir(root), f) || tw305Has("/etc", f) {
			t.Errorf("%s écrit hors du dossier", f)
		}
	}
	nat, _ := os.ReadFile(filepath.Join(dir, "natifs", "pulsation.yaml"))
	if strings.Contains(string(nat), "hostile") {
		t.Error("natifs/pulsation.yaml modifié")
	}
}

func tw305Snapshot(t *testing.T, root string) map[string]bool {
	t.Helper()
	m := map[string]bool{}
	_ = filepath.Walk(root, func(p string, _ os.FileInfo, err error) error {
		if err == nil {
			m[p] = true
		}
		return nil
	})
	return m
}

func TestTW305_Critical_Import_NothingEverWrittenInNatifs(t *testing.T) {
	srv, _, dir := tw305Server(t)
	before := tw305Names(t, filepath.Join(dir, "natifs"))
	tw305Import(t, srv, "replace", tw305File{"a.yaml", tw305Src("a", "")}, tw305File{"pulsation.yaml", tw305Src("pulsation", "")})
	tw305Import(t, srv, "keep_both", tw305File{"a.yaml", tw305Src("a", "")})
	if after := tw305Names(t, filepath.Join(dir, "natifs")); strings.Join(after, ",") != strings.Join(before, ",") {
		t.Errorf("natifs/ inchangé : %v → %v", before, after)
	}
}

// --- YAML hostile ------------------------------------------------------------

func TestTW305_Critical_Import_NestedAliases_RefusedWithoutBlowup(t *testing.T) {
	srv, _, dir := tw305Server(t)
	var b strings.Builder
	b.WriteString("id: bombe\nnom: Bombe\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\n")
	b.WriteString("a0: &a0 [x, x, x, x, x, x, x, x, x]\n")
	for i := 1; i < 12; i++ {
		fmt.Fprintf(&b, "a%d: &a%d [*a%d, *a%d, *a%d, *a%d, *a%d, *a%d, *a%d, *a%d, *a%d]\n", i, i, i-1, i-1, i-1, i-1, i-1, i-1, i-1, i-1, i-1)
	}
	b.WriteString("pas:\n  - { g1: [\"#FF0000\", 100] }\n")
	done := make(chan map[string]any, 1)
	go func() { _, out := tw305Import(t, srv, "", tw305File{"bombe.yaml", b.String()}); done <- out }()
	select {
	case out := <-done:
		r := tw305Results(t, out)
		if r[0]["status"] != "invalid" {
			t.Errorf("alias imbriqués → invalid : %v", r[0])
		}
	case <-timeAfter305():
		t.Fatal("le décodage explose (temps / mémoire) : analyse non bornée")
	}
	if tw305Has(dir, "bombe.yaml") {
		t.Error("rien d'écrit")
	}
}

// --- non-régression v11.7.0 ---------------------------------------------------

func TestTW305_NonRegression_V1170Routes(t *testing.T) {
	srv, _, _ := tw305Server(t)
	if w := tw305Do(srv, "GET", "/api/lighting/effects"); w.Code != 200 || !strings.Contains(w.Body.String(), `"pulsation"`) {
		t.Errorf("GET /api/lighting/effects : %d", w.Code)
	}
	if w := tw305Do(srv, "POST", "/api/lighting/effects/reload"); w.Code != 200 {
		t.Errorf("reload : %d", w.Code)
	}
	if w := tw305Do(srv, "GET", "/api/lighting/effects/pulsation/preview?groups=4&moment=chrono"); w.Code != 200 {
		t.Errorf("preview : %d", w.Code)
	}
	if w := tw305Do(srv, "POST", "/api/lighting/effects/try/stop"); w.Code != 200 && w.Code != 204 {
		t.Errorf("try/stop : %d", w.Code)
	}
	if w := tw305Do(srv, "GET", "/api/lighting/effects/inconnu/preview"); w.Code != 404 {
		t.Errorf("preview inconnu : %d", w.Code)
	}
	if w := tw305Do(srv, "POST", "/api/lighting/effects/import"); w.Code == 404 {
		t.Error("la route import existe")
	}
	if w := tw305Do(srv, "GET", "/api/lighting/effects/a/b/c"); w.Code != 404 {
		t.Errorf("chemin inconnu : %d", w.Code)
	}
}

func TestTW305_Reload_PicksUpImportedFileWithoutRestart(t *testing.T) {
	srv, _, _ := tw305Server(t)
	tw305Import(t, srv, "", tw305File{"x.yaml", tw305Src("apres-import", "")})
	if w := tw305Do(srv, "GET", "/api/lighting/effects"); !strings.Contains(w.Body.String(), "apres-import") {
		t.Error("GET /api/lighting/effects voit l'effet importé")
	}
}

var _ = http.StatusOK

func timeAfter305() <-chan time.Time { return time.After(10 * time.Second) }
