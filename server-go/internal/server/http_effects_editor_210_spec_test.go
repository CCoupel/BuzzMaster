package server

// #210 (v11.8.0, LOT-2) — routes GET …/{id}/source, POST …/draft, PUT …/{id}, POST …/draft/try.
// Source : contracts/light-effects.md §11.2, §11.3, §11.4, §11.9, §11.10 ; non-régression §7.x. TDD : rouge tant que B2.1 à B2.4 ne sont
// pas livrés. Surface supposée : http_effects_editor_210_api_spec_test.go. Noms : TestTW210_* ; `critical` = TestTW210_Critical_*.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tw210Do(t *testing.T, srv *HTTPServer, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var rd *bytes.Reader
	switch b := body.(type) {
	case nil:
		rd = bytes.NewReader(nil)
	case string:
		rd = bytes.NewReader([]byte(b))
	default:
		raw, _ := json.Marshal(b)
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func tw210Yaml(id, extra string) string {
	return "id: " + id + "\nnom: Effet " + id + "\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\n" + extra + "pas:\n  - { g1: [\"#FF0000\", 100] }\n"
}

const tw210DraftA = `id: milieu-et-suiveur
nom: Milieu et suiveur
version: 2
moments: [chrono]
duree_tick_ms: 1000
variables:
  gB: gA+1
  gC: N; -N/3
  gA: N/2; +1
pas:
  - { g0: [theme, 100], gB: [theme, 50], gC: ["#14B8A6", 100], gA: [urgence, 100] }
`

func tw210Snapshot(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			raw, _ := os.ReadFile(p)
			fmt.Fprintf(&b, "%s=%s;", p, raw)
		}
		return nil
	})
	return b.String()
}

// ---------------------------------------------------------------------------
// §11.2 source
// ---------------------------------------------------------------------------

func TestTW210_Source_Native_ReadOnly_NoReferenceHeader(t *testing.T) {
	defer tw210Catch(t)
	srv, _, _ := tw305Server(t)
	code, out := tw210Do(t, srv, "GET", "/api/lighting/effects/chenillard-traine/source", nil)
	if code != 200 || out["id"] != "chenillard-traine" || out["origin"] != "native" || out["read_only"] != true || out["file"] != "natifs/chenillard-traine.yaml" {
		t.Fatalf("source : %d %v", code, out)
	}
	if y, _ := out["yaml"].(string); !strings.Contains(y, "id: chenillard-traine") || strings.Contains(y, "COPIE DE RÉFÉRENCE") {
		t.Errorf("texte embarqué sans l'en-tête de la copie de référence : %.100q", y)
	}
	for _, k := range []string{"effect", "has_comments", "custom_order", "grid"} {
		if _, ok := out[k]; !ok {
			t.Errorf("champ %q absent", k)
		}
	}
	g := out["grid"].(map[string]any)
	if g["editable"] != true || len(g["converted"].([]any)) != 2 {
		t.Errorf("voisins du chenillard avec traînée convertis : %v", g)
	}
}

func TestTW210_Source_User_Writable_HasComments(t *testing.T) {
	defer tw210Catch(t)
	srv, prov, dir := tw305Server(t)
	_ = os.WriteFile(filepath.Join(dir, "x.yaml"), []byte("# note\n"+tw210Yaml("mon-effet", "")), 0o644)
	prov.Reload()
	_, out := tw210Do(t, srv, "GET", "/api/lighting/effects/mon-effet/source", nil)
	if out["origin"] != "user" || out["read_only"] != false || out["has_comments"] != true || out["custom_order"] != false || out["file"] != "x.yaml" {
		t.Errorf("effet utilisateur : %v", out)
	}
}

func TestTW210_Source_OverriddenNative_ReadOnly(t *testing.T) {
	defer tw210Catch(t)
	srv, prov, dir := tw305Server(t)
	_ = os.WriteFile(filepath.Join(dir, "degrade.yaml"), []byte(tw210Yaml("degrade", "")), 0o644)
	prov.Reload()
	_, out := tw210Do(t, srv, "GET", "/api/lighting/effects/degrade/source", nil)
	if out["origin"] != "override" || out["read_only"] != true {
		t.Errorf("natif surchargé : lecture seule (E2) : %v", out)
	}
}

func TestTW210_Source_Unknown_404_And_NoWrite(t *testing.T) {
	defer tw210Catch(t)
	srv, _, dir := tw305Server(t)
	before := tw210Snapshot(t, dir)
	if code, _ := tw210Do(t, srv, "GET", "/api/lighting/effects/inconnu/source", nil); code != 404 {
		t.Errorf("404 attendu, obtenu %d", code)
	}
	if tw210Snapshot(t, dir) != before {
		t.Error("rien n'est écrit")
	}
}

// ---------------------------------------------------------------------------
// §11.3 draft
// ---------------------------------------------------------------------------

func TestTW210_Draft_Yaml_Valid_FullAnswer_NothingWritten(t *testing.T) {
	defer tw210Catch(t)
	srv, prov, dir := tw305Server(t)
	before := tw210Snapshot(t, dir)
	gen := prov.Catalogue().Generation
	code, out := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"yaml": "# c\n" + tw210DraftA, "groups": []int{1, 3, 6, 12, 4}})
	if code != 200 || out["ok"] != true {
		t.Fatalf("draft : %d %v", code, out)
	}
	if out["has_comments"] != true || out["custom_order"] != false || out["moment"] != "chrono" {
		t.Errorf("has_comments / custom_order / moment : %v", out)
	}
	if y, _ := out["yaml"].(string); !strings.Contains(y, "id: milieu-et-suiveur") || strings.Contains(y, "# c") {
		t.Errorf("yaml régénéré (sans commentaire) : %.120q", y)
	}
	prev := out["previews"].([]any)
	if len(prev) != 5 {
		t.Fatalf("un aperçu par taille : %d", len(prev))
	}
	for i, n := range []int{1, 3, 6, 12, 4} {
		p := prev[i].(map[string]any)
		if p["groups"] != float64(n) || p["id"] != "milieu-et-suiveur" {
			t.Errorf("aperçu %d : dans l'ordre de groups (%d) : %v", i, n, p["groups"])
		}
	}
	if tw210Snapshot(t, dir) != before || prov.Catalogue().Generation != gen {
		t.Error("draft n'écrit rien et ne touche pas au catalogue")
	}
	if _, ok := prov.Catalogue().Get("milieu-et-suiveur"); ok {
		t.Error("le brouillon n'entre pas au catalogue")
	}
}

func TestTW210_Draft_PreviewVarsMatchChronogramA(t *testing.T) {
	defer tw210Catch(t)
	srv, _, _ := tw305Server(t)
	_, out := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"yaml": tw210DraftA, "groups": []int{5}})
	frames := out["previews"].([]any)[0].(map[string]any)["frames"].([]any)
	wantA := []float64{2, 3, 4, 5, 1}
	wantB := []float64{3, 4, 5, 1, 2}
	wantC := []float64{5, 4, 3, 2, 1}
	idx := 0
	for _, f := range frames {
		fr := f.(map[string]any)
		if fr["tick"] != float64(idx) {
			continue // trame supplémentaire (changement de couleur nommée) : même tick
		}
		v := fr["vars"].(map[string]any)
		if v["gA"] != wantA[idx] || v["gB"] != wantB[idx] || v["gC"] != wantC[idx] {
			t.Errorf("vars au tick %d : %v, attendu gA=%v gB=%v gC=%v", idx, v, wantA[idx], wantB[idx], wantC[idx])
		}
		idx++
		if idx == 5 {
			break
		}
	}
	if idx < 5 {
		t.Errorf("5 ticks attendus, vus %d", idx)
	}
}

func TestTW210_Draft_Effect_Document_RoundTrip(t *testing.T) {
	defer tw210Catch(t)
	srv, _, _ := tw305Server(t)
	_, first := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"yaml": tw210DraftA})
	_, second := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"effect": first["effect"]})
	if second["ok"] != true || second["yaml"] != first["yaml"] {
		t.Errorf("un brouillon `effect` renvoie le même yaml régénéré : %v", second["errors"])
	}
	if second["has_comments"] != false {
		t.Error("has_comments faux pour un brouillon effect")
	}
}

func TestTW210_Draft_InvalidYaml_ErrorsWithLine_PreviewsEmpty(t *testing.T) {
	defer tw210Catch(t)
	srv, _, _ := tw305Server(t)
	_, out := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"yaml": tw210Yaml("x", "variables:\n  gA: N//2\n"), "groups": []int{3}})
	errs, _ := out["errors"].([]any)
	if out["ok"] != false || len(errs) == 0 || len(out["previews"].([]any)) != 0 || out["grid"] != nil {
		t.Fatalf("ok:false, errors[], previews [], grid null : %v", out)
	}
	e := errs[0].(map[string]any)
	if e["line"].(float64) == 0 || !strings.Contains(e["message"].(string), "N//2") || strings.Contains(e["message"].(string), ".yaml") {
		t.Errorf("line renseignée, message sans nom de fichier : %v", e)
	}
}

func TestTW210_Draft_UnparsableYaml_EffectNull(t *testing.T) {
	defer tw210Catch(t)
	srv, _, _ := tw305Server(t)
	_, out := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"yaml": "id: [\nnom: {"})
	if out["ok"] != false || out["effect"] != nil {
		t.Errorf("texte inanalysable : effect null : %v", out)
	}
}

func TestTW210_Draft_InvalidEffectDocument_ErrorsCarryPath(t *testing.T) {
	defer tw210Catch(t)
	srv, _, _ := tw305Server(t)
	_, base := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"yaml": tw210DraftA})
	doc := base["effect"].(map[string]any)
	doc["variables"].([]any)[2].(map[string]any)["start"] = "N//2"
	_, out := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"effect": doc})
	errs, _ := out["errors"].([]any)
	if out["ok"] != false || len(errs) == 0 {
		t.Fatalf("%v", out)
	}
	e := errs[0].(map[string]any)
	if e["path"] != "variables[2].start" || e["line"] != float64(0) {
		t.Errorf("path = variables[2].start, line = 0 pour un brouillon effect : %v", e)
	}
}

func TestTW210_Draft_UndeclaredMoment_NotOk(t *testing.T) {
	defer tw210Catch(t)
	srv, _, _ := tw305Server(t)
	_, out := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"yaml": tw210DraftA, "moment": "score", "groups": []int{3}})
	if out["ok"] != false {
		t.Errorf("moment non déclaré ⇒ ok:false : %v", out)
	}
}

func TestTW210_Draft_400_And_413(t *testing.T) {
	defer tw210Catch(t)
	srv, _, _ := tw305Server(t)
	for name, body := range map[string]any{
		"JSON illisible":         "pas du json",
		"les deux clés":          map[string]any{"yaml": "x", "effect": map[string]any{}},
		"aucune clé":             map[string]any{"groups": []int{1}},
		"groups hors bornes (0)": map[string]any{"yaml": tw210DraftA, "groups": []int{0}},
		"groups hors bornes 13":  map[string]any{"yaml": tw210DraftA, "groups": []int{13}},
		"9 tailles":              map[string]any{"yaml": tw210DraftA, "groups": []int{1, 2, 3, 4, 5, 6, 7, 8, 9}},
		"tailles en double":      map[string]any{"yaml": tw210DraftA, "groups": []int{3, 3}},
	} {
		if code, _ := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", body); code != 400 {
			t.Errorf("%s : 400 attendu, obtenu %d", name, code)
		}
	}
	if code, _ := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"yaml": strings.Repeat("a", 70*1024)}); code != 413 {
		t.Errorf("corps > 64 Ko : 413, obtenu %d", code)
	}
}

func TestTW210_Draft_IDNeedNotBeFree(t *testing.T) {
	defer tw210Catch(t)
	srv, _, _ := tw305Server(t)
	_, out := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"yaml": tw210Yaml("pulsation", "")})
	if out["ok"] != true {
		t.Errorf("l'unicité n'est vérifiée qu'à l'enregistrement : %v", out["errors"])
	}
}

func TestTW210_Draft_GridConversion_ForNeighbours(t *testing.T) {
	defer tw210Catch(t)
	srv, _, _ := tw305Server(t)
	_, out := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"yaml": tw210Yaml("voisin", "variables:\n  gA: 1; +1\n"), "groups": []int{}})
	_ = out
	src := "id: voisin\nnom: V\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\nvariables:\n  gA: 1; +1\npas:\n  - { gA+1: [\"#FF0000\", 100] }\n"
	_, out = tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"yaml": src})
	g, _ := out["grid"].(map[string]any)
	if g == nil || g["editable"] != true || len(g["converted"].([]any)) != 1 {
		t.Errorf("grid calculée sur le brouillon (voisin converti) : %v", out["grid"])
	}
}

// ---------------------------------------------------------------------------
// §11.4 PUT
// ---------------------------------------------------------------------------

func TestTW210_Put_Create_EffectDocument_RegeneratedFile(t *testing.T) {
	defer tw210Catch(t)
	srv, prov, dir := tw305Server(t)
	_, draft := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"yaml": tw210DraftA})
	code, out := tw210Do(t, srv, "PUT", "/api/lighting/effects/milieu-et-suiveur", map[string]any{"effect": draft["effect"], "create": true})
	if code != 200 || out["ok"] != true || out["file"] != "milieu-et-suiveur.yaml" || out["catalogue"] == nil {
		t.Fatalf("création : %d %v", code, out)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "milieu-et-suiveur.yaml"))
	if string(got) != draft["yaml"] {
		t.Errorf("le fichier est le texte régénéré :\n%s", got)
	}
	if e, ok := prov.Catalogue().Get("milieu-et-suiveur"); !ok || e.Origin != "user" {
		t.Error("catalogue rechargé avant la réponse")
	}
}

func TestTW210_Put_Yaml_WrittenAsIs_CommentsKept(t *testing.T) {
	defer tw210Catch(t)
	srv, _, dir := tw305Server(t)
	src := "# conservé\n" + tw210Yaml("mon-effet", "")
	if code, out := tw210Do(t, srv, "PUT", "/api/lighting/effects/mon-effet", map[string]any{"yaml": src, "create": true}); code != 200 {
		t.Fatalf("%d %v", code, out)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "mon-effet.yaml")); string(b) != src {
		t.Errorf("texte écrit à l'octet près : %q", b)
	}
}

func TestTW210_Put_Modify_KeepsFileName_LastWriteWins(t *testing.T) {
	defer tw210Catch(t)
	srv, prov, dir := tw305Server(t)
	_ = os.WriteFile(filepath.Join(dir, "nom-libre.yaml"), []byte(tw210Yaml("mon-effet", "")), 0o644)
	prov.Reload()
	code, out := tw210Do(t, srv, "PUT", "/api/lighting/effects/mon-effet", map[string]any{"yaml": tw210Yaml("mon-effet", "description: v2\n")})
	if code != 200 || out["file"] != "nom-libre.yaml" {
		t.Fatalf("%d %v", code, out)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "nom-libre.yaml")); !strings.Contains(string(b), "description: v2") {
		t.Error("modifié dans son fichier")
	}
}

func TestTW210_Put_Refusals_Codes_AndReasons(t *testing.T) {
	defer tw210Catch(t)
	srv, prov, dir := tw305Server(t)
	_ = os.WriteFile(filepath.Join(dir, "existe.yaml"), []byte(tw210Yaml("existe", "")), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "degrade.yaml"), []byte(tw210Yaml("degrade", "")), 0o644)
	prov.Reload()
	cases := []struct {
		name, id string
		body     map[string]any
		code     int
		reason   string
	}{
		{"create sur id existant", "existe", map[string]any{"yaml": tw210Yaml("existe", ""), "create": true}, 409, "exists"},
		{"create sur id d'un natif", "pulsation", map[string]any{"yaml": tw210Yaml("pulsation", ""), "create": true}, 409, "native"},
		{"modification d'un natif", "pulsation", map[string]any{"yaml": tw210Yaml("pulsation", "")}, 409, "native"},
		{"modification d'un natif surchargé", "degrade", map[string]any{"yaml": tw210Yaml("degrade", "")}, 409, "native"},
		{"id inconnu sans create", "inconnu", map[string]any{"yaml": tw210Yaml("inconnu", "")}, 404, ""},
		{"id du corps ≠ chemin", "existe", map[string]any{"yaml": tw210Yaml("autre", "")}, 400, ""},
		{"id réservé reload", "reload", map[string]any{"yaml": tw210Yaml("reload", ""), "create": true}, 400, ""},
		{"id réservé try", "try", map[string]any{"yaml": tw210Yaml("try", ""), "create": true}, 400, ""},
		{"id réservé draft", "draft", map[string]any{"yaml": tw210Yaml("draft", ""), "create": true}, 400, ""},
		{"id réservé import", "import", map[string]any{"yaml": tw210Yaml("import", ""), "create": true}, 400, ""},
	}
	for _, c := range cases {
		before := tw210Snapshot(t, dir)
		code, out := tw210Do(t, srv, "PUT", "/api/lighting/effects/"+c.id, c.body)
		if code != c.code || (c.reason != "" && out["reason"] != c.reason) {
			t.Errorf("%s : %d %v, attendu %d %q", c.name, code, out, c.code, c.reason)
		}
		if tw210Snapshot(t, dir) != before {
			t.Errorf("%s : rien ne doit être écrit", c.name)
		}
	}
}

func TestTW210_Put_Invalid_400WithErrors_FileUnchanged(t *testing.T) {
	defer tw210Catch(t)
	srv, prov, dir := tw305Server(t)
	_ = os.WriteFile(filepath.Join(dir, "mon-effet.yaml"), []byte(tw210Yaml("mon-effet", "")), 0o644)
	prov.Reload()
	before := tw210Snapshot(t, dir)
	code, out := tw210Do(t, srv, "PUT", "/api/lighting/effects/mon-effet", map[string]any{"yaml": tw210Yaml("mon-effet", "variables:\n  gA: N//2\n")})
	if code != 400 || out["ok"] != false || len(out["errors"].([]any)) == 0 {
		t.Errorf("400 {ok:false, errors} (forme du §11.3) : %d %v", code, out)
	}
	if tw210Snapshot(t, dir) != before {
		t.Error("fichier inchangé")
	}
}

func TestTW210_Put_Limit100_409(t *testing.T) {
	defer tw210Catch(t)
	srv, prov, dir := tw305Server(t)
	for i := 0; i < 100; i++ {
		_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("e%d.yaml", i)), []byte(tw210Yaml(fmt.Sprintf("e%d", i), "")), 0o644)
	}
	prov.Reload()
	code, out := tw210Do(t, srv, "PUT", "/api/lighting/effects/de-trop", map[string]any{"yaml": tw210Yaml("de-trop", ""), "create": true})
	if code != 409 || out["reason"] != "limit" {
		t.Errorf("limite : 409 {reason: limit} : %d %v", code, out)
	}
}

func TestTW210_Put_NameTaken_UsesSuffix(t *testing.T) {
	defer tw210Catch(t)
	srv, prov, dir := tw305Server(t)
	_ = os.WriteFile(filepath.Join(dir, "mon-effet.yaml"), []byte(tw210Yaml("autre", "")), 0o644)
	prov.Reload()
	_, out := tw210Do(t, srv, "PUT", "/api/lighting/effects/mon-effet", map[string]any{"yaml": tw210Yaml("mon-effet", ""), "create": true})
	if out["file"] != "mon-effet-2.yaml" {
		t.Errorf("nom pris par un autre effet : <id>-2.yaml : %v", out)
	}
}

func TestTW210_Critical_Put_NeverWritesOutsideDirNorInNatifs(t *testing.T) {
	defer tw210Catch(t)
	srv, _, dir := tw305Server(t)
	root := filepath.Dir(filepath.Dir(dir))
	before := tw210Snapshot(t, root)
	for _, id := range []string{"..%2Fevade", "a%2Fb", "..", "natifs"} {
		tw210Do(t, srv, "PUT", "/api/lighting/effects/"+id, map[string]any{"yaml": tw210Yaml("x", ""), "create": true})
	}
	tw210Do(t, srv, "PUT", "/api/lighting/effects/ok-id", map[string]any{"yaml": tw210Yaml("ok-id", ""), "create": true})
	after := tw210Snapshot(t, root)
	if !strings.Contains(after, "ok-id.yaml") {
		t.Fatal("écriture normale perdue")
	}
	for _, seg := range strings.Split(after, ";") {
		p := strings.SplitN(seg, "=", 2)[0]
		if p == "" || strings.Contains(before, p+"=") {
			continue
		}
		if rel, _ := filepath.Rel(dir, p); strings.HasPrefix(rel, "..") || strings.Contains(rel, string(filepath.Separator)) {
			t.Errorf("fichier créé hors du dossier des effets (ou dans natifs/) : %s", p)
		}
	}
}

func TestTW210_Put_Atomic_LoaderNeverSeesPartialYaml(t *testing.T) {
	defer tw210Catch(t)
	srv, prov, dir := tw305Server(t)
	tw210Do(t, srv, "PUT", "/api/lighting/effects/mon-effet", map[string]any{"yaml": tw210Yaml("mon-effet", ""), "create": true})
	for i := 0; i < 20; i++ {
		tw210Do(t, srv, "PUT", "/api/lighting/effects/mon-effet", map[string]any{"yaml": tw210Yaml("mon-effet", "description: "+strings.Repeat("v", 100+i)+"\n")})
		if errs := prov.Catalogue().Errors(); len(errs) > 0 {
			t.Fatalf("fichier partiel vu : %+v", errs)
		}
	}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if !e.IsDir() && e.Name() != "mon-effet.yaml" {
			t.Errorf("fichier temporaire laissé : %s", e.Name())
		}
	}
}

func TestTW210_Put_BadBody_400(t *testing.T) {
	defer tw210Catch(t)
	srv, _, _ := tw305Server(t)
	for name, body := range map[string]any{"illisible": "pas du json", "ni effect ni yaml": map[string]any{"create": true}, "les deux": map[string]any{"yaml": "x", "effect": map[string]any{}}} {
		if code, _ := tw210Do(t, srv, "PUT", "/api/lighting/effects/mon-effet", body); code != 400 {
			t.Errorf("%s : 400 attendu, obtenu %d", name, code)
		}
	}
	if code, _ := tw210Do(t, srv, "PUT", "/api/lighting/effects/mon-effet", map[string]any{"yaml": strings.Repeat("a", 70*1024), "create": true}); code != 413 && code != 400 {
		t.Errorf("corps > 64 Ko refusé : %d", code)
	}
}

// ---------------------------------------------------------------------------
// §11.10 draft/try
// ---------------------------------------------------------------------------

func tw210TrialServer(t *testing.T) (*HTTPServer, *tw210Trialer, string) {
	t.Helper()
	srv, _, trial := tw223TrialServer(t)
	_, prov, dir := tw305Server(t)
	srv.Effects = prov
	tr := &tw210Trialer{tw223TrialProvider: trial}
	srv.Lighting = tr
	return srv, tr, dir
}

func TestTW210_DraftTry_PlaysTheDraft_WritesNothing(t *testing.T) {
	defer tw210Catch(t)
	srv, tr, dir := tw210TrialServer(t)
	before := tw210Snapshot(t, dir)
	code, out := tw210Do(t, srv, "POST", "/api/lighting/effects/draft/try", map[string]any{"yaml": tw210Yaml("pulsation", "description: brouillon\n"), "zone": "animateur", "speed": 1, "seconds": 10})
	if code != 200 || out["ok"] != true || out["duration_ms"] != float64(10000) {
		t.Fatalf("%d %v", code, out)
	}
	if len(tr.drafts) != 1 || tr.drafts[0].zone != "animateur" || tr.drafts[0].e.Description != "brouillon" || tr.drafts[0].seconds != 10 {
		t.Errorf("c'est le brouillon qui est joué (id d'un effet existant admis), pas l'effet enregistré : %+v", tr.drafts)
	}
	if len(tr.calls) != 0 {
		t.Error("StartEffectTrial (catalogue) n'est pas appelé")
	}
	if tw210Snapshot(t, dir) != before {
		t.Error("rien n'est écrit")
	}
}

func TestTW210_DraftTry_Defaults_Zone_Speed_Seconds(t *testing.T) {
	defer tw210Catch(t)
	srv, tr, _ := tw210TrialServer(t)
	tw210Do(t, srv, "POST", "/api/lighting/effects/draft/try", map[string]any{"yaml": tw210Yaml("x", "")})
	score := strings.Replace(tw210Yaml("y", ""), "moments: [calm]", "moments: [score]", 1)
	tw210Do(t, srv, "POST", "/api/lighting/effects/draft/try", map[string]any{"yaml": score})
	if len(tr.drafts) != 2 || tr.drafts[0].zone != "ambiance" || tr.drafts[0].speed != 1 || tr.drafts[0].seconds != 10 || tr.drafts[1].seconds != 5 {
		t.Errorf("défauts du §7.4 (ambiance, ×1, 10 s ; 5 s pour un effet de score) : %+v", tr.drafts)
	}
}

func TestTW210_DraftTry_EffectDocumentDraft(t *testing.T) {
	defer tw210Catch(t)
	srv, tr, _ := tw210TrialServer(t)
	_, d := tw210Do(t, srv, "POST", "/api/lighting/effects/draft", map[string]any{"yaml": tw210DraftA})
	if code, _ := tw210Do(t, srv, "POST", "/api/lighting/effects/draft/try", map[string]any{"effect": d["effect"], "zone": "public"}); code != 200 || len(tr.drafts) != 1 || tr.drafts[0].zone != "public" {
		t.Errorf("brouillon `effect` : %d %+v", code, tr.drafts)
	}
}

func TestTW210_DraftTry_InvalidDraft_400_NothingPlayed(t *testing.T) {
	defer tw210Catch(t)
	srv, tr, _ := tw210TrialServer(t)
	code, out := tw210Do(t, srv, "POST", "/api/lighting/effects/draft/try", map[string]any{"yaml": tw210Yaml("x", "variables:\n  gA: N//2\n")})
	if code != 400 || out["ok"] != false || len(out["errors"].([]any)) == 0 {
		t.Errorf("400 {ok:false, errors} : %d %v", code, out)
	}
	if len(tr.drafts) != 0 {
		t.Error("rien n'est joué")
	}
}

func TestTW210_DraftTry_SameRefusalsAsEffectTry(t *testing.T) {
	defer tw210Catch(t)
	srv, tr, _ := tw210TrialServer(t)
	for name, body := range map[string]map[string]any{
		"zone inconnue": {"zone": "salle"}, "vitesse hors liste": {"speed": 3}, "durée trop longue": {"seconds": 21},
	} {
		body["yaml"] = tw210Yaml("x", "")
		if code, _ := tw210Do(t, srv, "POST", "/api/lighting/effects/draft/try", body); code != http.StatusBadRequest {
			t.Errorf("%s : 400 attendu, obtenu %d", name, code)
		}
	}
	for _, reason := range []string{"disabled", "game", "not_applicable", "mode"} {
		tr.err = &tw223Refusal{reason}
		code, out := tw210Do(t, srv, "POST", "/api/lighting/effects/draft/try", map[string]any{"yaml": tw210Yaml("x", "")})
		if code != http.StatusConflict || out["reason"] != reason {
			t.Errorf("refus %s : 409 {reason} : %d %v", reason, code, out)
		}
	}
	tr.err = nil
	if len(tr.drafts) != 0 {
		t.Error("un refus ne démarre rien")
	}
}

func TestTW210_DraftTry_PathNotConfusedWithAnEffectID(t *testing.T) {
	defer tw210Catch(t)
	srv, tr, _ := tw210TrialServer(t)
	// « draft » est réservé : …/draft/try ne désigne jamais un effet du catalogue
	if code, _ := tw210Do(t, srv, "POST", "/api/lighting/effects/draft/try", map[string]any{"yaml": tw210Yaml("x", "")}); code != 200 || len(tr.calls) != 0 {
		t.Errorf("%d, appels catalogue : %+v", code, tr.calls)
	}
}

// ---------------------------------------------------------------------------
// non-régression v11.7.0 / LOT-1
// ---------------------------------------------------------------------------

func TestTW210_Critical_NonRegression_V1170Routes_AndPreviewAdditiveVars(t *testing.T) {
	defer tw210Catch(t)
	srv, _, _ := tw305Server(t)
	if code, _ := tw210Do(t, srv, "GET", "/api/lighting/effects", nil); code != 200 {
		t.Errorf("catalogue : %d", code)
	}
	if code, _ := tw210Do(t, srv, "POST", "/api/lighting/effects/reload", nil); code != 200 {
		t.Errorf("reload : %d", code)
	}
	code, out := tw210Do(t, srv, "GET", "/api/lighting/effects/pulsation/preview?groups=4&moment=chrono", nil)
	if code != 200 {
		t.Fatalf("preview : %d", code)
	}
	f := out["frames"].([]any)[0].(map[string]any)
	if v, ok := f["vars"].(map[string]any); !ok || len(v) != 0 {
		t.Errorf("vars : objet vide sans variable (champ additif) : %v", f["vars"])
	}
	for _, k := range []string{"at_ms", "tick", "step", "groups"} {
		if _, ok := f[k]; !ok {
			t.Errorf("champ v11.7.0 %q conservé", k)
		}
	}
}

func TestTW210_Critical_NonRegression_V1170Writes_Unchanged(t *testing.T) {
	defer tw210Catch(t)
	// Les routes LOT-1 (DELETE, download, import) et la protection des natifs restent identiques.
	srv, _, _ := tw305Server(t)
	if code, out := tw210Do(t, srv, "DELETE", "/api/lighting/effects/pulsation", nil); code != 409 || out["reason"] != "native" {
		t.Errorf("DELETE natif : %d %v", code, out)
	}
	if w := tw305Do(srv, "GET", "/api/lighting/effects/pulsation/download"); w.Code != 200 {
		t.Errorf("download : %d", w.Code)
	}
}

// tw210Catch : une réponse absente ou de forme inattendue (code non livré) échoue le test au lieu d'interrompre toute la série.
func tw210Catch(t *testing.T) {
	if r := recover(); r != nil {
		t.Errorf("réponse de forme inattendue : %v", r)
	}
}
