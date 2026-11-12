package effects

// #305 (v11.8.0, LOT-1) — écriture, suppression et lecture des fichiers d'effets du Manager.
// Source : contracts/light-effects.md §3.2, §11.4, §11.5, §11.6, §11.7 (réécriture de `id`), §11.9.
// Écrits depuis le contrat (TDD) : peuvent échouer tant que B1.1 n'est pas livré. Les appels dépendant de la signature
// sont dans store_api_spec_test.go. Noms : TestTW305_*. Tags : critical = TestTW305_Critical_*.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

func tw305Src(id, extra string) string {
	return "id: " + id + "\nnom: Effet " + id + "\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\n" + extra + "pas:\n  - { g1: [\"#FF0000\", 100] }\n"
}

var tw305Natives = fstest.MapFS{
	"base.yaml": {Data: []byte(tw305Src("base", ""))},
}

func tw305Manager(t *testing.T) (*Manager, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "effects")
	if err := EnsureDirs(tw305Natives, dir); err != nil {
		t.Fatal(err)
	}
	return NewManager(tw305Natives, dir, nil), dir
}

func tw305Yaml(t *testing.T, dir string) []string {
	t.Helper()
	ents, _ := os.ReadDir(dir)
	var out []string
	for _, e := range ents {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// --- §11.4 écriture -------------------------------------------------------

func TestTW305_Write_CreateUser_FileNamedAfterID_CatalogueReloaded(t *testing.T) {
	m, dir := tw305Manager(t)
	src := tw305Src("mon-effet", "# commentaire conservé\n")
	file, reason := storeWrite(m, "mon-effet", src, true)
	if reason != "" || file != "mon-effet.yaml" {
		t.Fatalf("création : file=%q reason=%q", file, reason)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "mon-effet.yaml"))
	if string(got) != src {
		t.Errorf("texte écrit tel quel à l'octet près (commentaires compris) :\n%q\n%q", got, src)
	}
	e, ok := m.Catalogue().Get("mon-effet")
	if !ok || e.Origin != OriginUser {
		t.Errorf("rechargement immédiat : %+v ok=%v", e, ok)
	}
}

func TestTW305_Write_Modify_KeepsExistingFileName(t *testing.T) {
	m, dir := tw305Manager(t)
	if err := os.WriteFile(filepath.Join(dir, "nom-libre.yaml"), []byte(tw305Src("mon-effet", "")), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Reload()
	file, reason := storeWrite(m, "mon-effet", tw305Src("mon-effet", "description: v2\n"), false)
	if reason != "" || file != "nom-libre.yaml" {
		t.Fatalf("le fichier qui définit déjà l'id garde son nom : file=%q reason=%q", file, reason)
	}
	if n := tw305Yaml(t, dir); len(n) != 1 {
		t.Errorf("aucun doublon : %v", n)
	}
}

func TestTW305_Write_NameTakenByAnotherEffect_UsesSuffix(t *testing.T) {
	m, dir := tw305Manager(t)
	// mon-effet.yaml existe mais définit un autre id ; mon-effet-2.yaml est un fichier invalide
	_ = os.WriteFile(filepath.Join(dir, "mon-effet.yaml"), []byte(tw305Src("autre", "")), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "mon-effet-2.yaml"), []byte("id: [\n"), 0o644)
	m.Reload()
	file, reason := storeWrite(m, "mon-effet", tw305Src("mon-effet", ""), true)
	if reason != "" || file != "mon-effet-3.yaml" {
		t.Errorf("<id>-2, -3… : file=%q reason=%q", file, reason)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "mon-effet.yaml")); !strings.Contains(string(b), "id: autre") {
		t.Error("le fichier d'un autre effet n'est jamais écrasé")
	}
}

func TestTW305_Write_CreateExisting_Exists(t *testing.T) {
	m, dir := tw305Manager(t)
	storeWrite(m, "mon-effet", tw305Src("mon-effet", ""), true)
	before, _ := os.ReadFile(filepath.Join(dir, "mon-effet.yaml"))
	if _, reason := storeWrite(m, "mon-effet", tw305Src("mon-effet", "description: x\n"), true); reason != "exists" {
		t.Errorf("create + id déjà au catalogue → exists, obtenu %q", reason)
	}
	if after, _ := os.ReadFile(filepath.Join(dir, "mon-effet.yaml")); string(after) != string(before) {
		t.Error("rien n'est écrit sur un refus")
	}
}

func TestTW305_Write_ModifyUnknown_NotFound(t *testing.T) {
	m, _ := tw305Manager(t)
	if _, reason := storeWrite(m, "inconnu", tw305Src("inconnu", ""), false); reason != "unknown" {
		t.Errorf("sans create et id inconnu → 404 (unknown), obtenu %q", reason)
	}
}

func TestTW305_Critical_Write_NativeIDRefused_NothingWritten(t *testing.T) {
	m, dir := tw305Manager(t)
	for _, create := range []bool{true, false} {
		if _, reason := storeWrite(m, "base", tw305Src("base", "description: piège\n"), create); reason != "native" {
			t.Errorf("id d'un natif (create=%v) → native, obtenu %q", create, reason)
		}
	}
	if n := tw305Yaml(t, dir); len(n) != 0 {
		t.Errorf("rien n'est écrit : %v", n)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "natifs", "base.yaml")); !strings.Contains(string(b), "ne pas modifier") || strings.Contains(string(b), "piège") {
		t.Error("natifs/ n'est jamais touché")
	}
}

func TestTW305_Critical_Write_OverriddenNativeIDRefused(t *testing.T) {
	m, dir := tw305Manager(t)
	// natif surchargé à la main : l'id reste celui d'un natif (§11.4)
	_ = os.WriteFile(filepath.Join(dir, "base.yaml"), []byte(tw305Src("base", "description: main\n")), 0o644)
	m.Reload()
	before, _ := os.ReadFile(filepath.Join(dir, "base.yaml"))
	if _, reason := storeWrite(m, "base", tw305Src("base", "description: ui\n"), false); reason != "native" {
		t.Errorf("natif surchargé → native, obtenu %q", reason)
	}
	if after, _ := os.ReadFile(filepath.Join(dir, "base.yaml")); string(after) != string(before) {
		t.Error("le fichier de surcharge n'est pas modifié par l'interface")
	}
}

func TestTW305_Write_ReservedIDs(t *testing.T) {
	m, dir := tw305Manager(t)
	for _, id := range []string{"reload", "try", "draft", "import"} {
		if _, reason := storeWrite(m, id, tw305Src(id, ""), true); reason != "reserved" {
			t.Errorf("id réservé %q → 400 (reserved), obtenu %q", id, reason)
		}
	}
	if n := tw305Yaml(t, dir); len(n) != 0 {
		t.Errorf("rien d'écrit : %v", n)
	}
}

func TestTW305_Write_Invalid_NothingWritten_FileUnchanged(t *testing.T) {
	m, dir := tw305Manager(t)
	storeWrite(m, "mon-effet", tw305Src("mon-effet", ""), true)
	before, _ := os.ReadFile(filepath.Join(dir, "mon-effet.yaml"))
	if _, reason := storeWrite(m, "mon-effet", "id: mon-effet\nnom: [\n", false); reason != "invalid" {
		t.Errorf("YAML invalide → invalid, obtenu %q", reason)
	}
	if after, _ := os.ReadFile(filepath.Join(dir, "mon-effet.yaml")); string(after) != string(before) {
		t.Error("fichier inchangé")
	}
	if e, ok := m.Catalogue().Get("mon-effet"); !ok || e.Origin != OriginUser {
		t.Error("l'effet reste au catalogue")
	}
}

func TestTW305_Write_IDMismatchBetweenTextAndPath_Refused(t *testing.T) {
	m, dir := tw305Manager(t)
	if _, reason := storeWrite(m, "mon-effet", tw305Src("autre", ""), true); reason == "" {
		t.Error("l'id du texte ≠ id du chemin est refusé (l'id ne se change pas)")
	}
	if n := tw305Yaml(t, dir); len(n) != 0 {
		t.Errorf("rien d'écrit : %v", n)
	}
}

func TestTW305_Write_TooBig(t *testing.T) {
	m, dir := tw305Manager(t)
	big := tw305Src("gros", "description: "+strings.Repeat("a", 64*1024)+"\n")
	if _, reason := storeWrite(m, "gros", big, true); reason != "too_big" {
		t.Errorf("> 64 Ko → too_big, obtenu %q", reason)
	}
	if n := tw305Yaml(t, dir); len(n) != 0 {
		t.Errorf("rien d'écrit : %v", n)
	}
}

func TestTW305_Write_Limit100Files(t *testing.T) {
	m, dir := tw305Manager(t)
	for i := 0; i < 100; i++ {
		id := "e" + strconv.Itoa(i)
		if _, reason := storeWrite(m, id, tw305Src(id, ""), true); reason != "" {
			t.Fatalf("écriture %d : %q", i, reason)
		}
	}
	if _, reason := storeWrite(m, "de-trop", tw305Src("de-trop", ""), true); reason != "limit" {
		t.Errorf("101e création → limit, obtenu %q", reason)
	}
	if n := tw305Yaml(t, dir); len(n) != 100 {
		t.Errorf("100 fichiers : %d", len(n))
	}
	// modifier un existant reste permis à la limite
	if _, reason := storeWrite(m, "e5", tw305Src("e5", "description: ok\n"), false); reason != "" {
		t.Errorf("modification à la limite : %q", reason)
	}
}

func TestTW305_Write_Atomic_NoPartialYamlVisibleToLoader(t *testing.T) {
	m, dir := tw305Manager(t)
	storeWrite(m, "mon-effet", tw305Src("mon-effet", ""), true)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	bad := 0
	go func() { // lecteur concurrent : le chargeur ne voit jamais un fichier tronqué
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if c := LoadDir(tw305Natives, dir); len(c.Errors()) > 0 {
				bad++
			}
		}
	}()
	for i := 0; i < 60; i++ {
		storeWrite(m, "mon-effet", tw305Src("mon-effet", "description: "+strings.Repeat("v", 200+i)+"\n"), false)
	}
	close(stop)
	wg.Wait()
	if bad > 0 {
		t.Errorf("%d chargements ont vu un fichier partiel (écriture non atomique)", bad)
	}
	for _, n := range tw305Yaml(t, dir) {
		if n != "mon-effet.yaml" {
			t.Errorf("fichier temporaire laissé : %q", n)
		}
	}
}

func TestTW305_Critical_Write_NeverInNativesNorOutsideDir(t *testing.T) {
	m, dir := tw305Manager(t)
	parent := filepath.Dir(dir)
	storeWrite(m, "mon-effet", tw305Src("mon-effet", ""), true)
	storeWrite(m, "../evade", tw305Src("../evade", ""), true) // id hostile : refusé (ou invalide), jamais écrit ailleurs
	storeWrite(m, "a/b", tw305Src("a/b", ""), true)
	if ents, _ := os.ReadDir(filepath.Join(dir, "natifs")); len(ents) != 1 {
		t.Errorf("natifs/ inchangé (1 copie de référence) : %d", len(ents))
	}
	ents, _ := os.ReadDir(parent)
	if len(ents) != 1 || ents[0].Name() != "effects" {
		t.Errorf("rien hors du dossier des effets : %v", ents)
	}
	if _, err := os.Stat(filepath.Join(dir, "evade.yaml")); err == nil {
		t.Error("un id hostile n'est jamais transformé en fichier")
	}
}

// --- §11.5 suppression ---------------------------------------------------

func TestTW305_Delete_User(t *testing.T) {
	m, dir := tw305Manager(t)
	storeWrite(m, "mon-effet", tw305Src("mon-effet", ""), true)
	if r := storeDelete(m, "mon-effet"); r != "" {
		t.Fatalf("suppression : %q", r)
	}
	if _, err := os.Stat(filepath.Join(dir, "mon-effet.yaml")); err == nil {
		t.Error("fichier supprimé")
	}
	if _, ok := m.Catalogue().Get("mon-effet"); ok {
		t.Error("rechargement immédiat : l'effet a disparu")
	}
}

func TestTW305_Delete_Override_NativeResumes(t *testing.T) {
	m, dir := tw305Manager(t)
	_ = os.WriteFile(filepath.Join(dir, "base.yaml"), []byte(tw305Src("base", "description: surcharge\n")), 0o644)
	m.Reload()
	if e, _ := m.Catalogue().Get("base"); e.Origin != OriginOverride {
		t.Fatalf("préalable : override, obtenu %q", e.Origin)
	}
	if r := storeDelete(m, "base"); r != "" {
		t.Fatalf("suppression d'une surcharge : %q", r)
	}
	if e, ok := m.Catalogue().Get("base"); !ok || e.Origin != OriginNative {
		t.Errorf("le natif reprend : %+v ok=%v", e, ok)
	}
}

func TestTW305_Critical_Delete_NativeRefused_ReferenceCopyUntouched(t *testing.T) {
	m, dir := tw305Manager(t)
	if r := storeDelete(m, "base"); r != "native" {
		t.Errorf("natif → native, obtenu %q", r)
	}
	if _, err := os.Stat(filepath.Join(dir, "natifs", "base.yaml")); err != nil {
		t.Error("la copie de référence n'est pas supprimée")
	}
	if _, ok := m.Catalogue().Get("base"); !ok {
		t.Error("le natif reste")
	}
}

func TestTW305_Delete_Unknown(t *testing.T) {
	m, _ := tw305Manager(t)
	if r := storeDelete(m, "inconnu"); r != "unknown" {
		t.Errorf("effet inconnu → 404 (unknown), obtenu %q", r)
	}
}

// --- §11.6 / §11.2 lecture du texte --------------------------------------

func TestTW305_Text_User_ByteForByte(t *testing.T) {
	m, dir := tw305Manager(t)
	src := "# note\n" + tw305Src("mon-effet", "")
	_ = os.WriteFile(filepath.Join(dir, "x.yaml"), []byte(src), 0o644)
	m.Reload()
	if got, r := storeText(m, "mon-effet"); r != "" || got != src {
		t.Errorf("texte du fichier à l'octet près : %q / %q", got, r)
	}
}

func TestTW305_Text_Native_NoReferenceHeader(t *testing.T) {
	m, _ := tw305Manager(t)
	got, r := storeText(m, "base")
	if r != "" || got != tw305Src("base", "") {
		t.Errorf("natif : texte embarqué (sans l'en-tête de la copie de référence) : %q / %q", got, r)
	}
	if strings.Contains(got, "COPIE DE RÉFÉRENCE") {
		t.Error("l'en-tête « ne pas modifier ici » ne fait pas partie du téléchargement")
	}
}

func TestTW305_Text_Unknown(t *testing.T) {
	m, _ := tw305Manager(t)
	if _, r := storeText(m, "inconnu"); r != "unknown" {
		t.Errorf("404 attendu, obtenu %q", r)
	}
}

// --- §11.7 « Garder les deux » : seule la valeur de `id` est réécrite ----

func TestTW305_RewriteID_OnlyIDValueChanges_CommentsIntact(t *testing.T) {
	src := "# en-tête\nid: aller-retour   # mon id\nnom: Aller-retour\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\ndescription: >\n  id: pas une clé\npas:\n  - { g1: [\"#FF0000\", 100] }\n"
	got, err := storeRewriteID(src, "aller-retour-2")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(src, "id: aller-retour   #", "id: aller-retour-2   #", 1)
	if got != want {
		t.Errorf("seule la valeur de la clé id change :\n%s\n---\n%s", got, want)
	}
}

func TestTW305_RewriteID_QuotedValue(t *testing.T) {
	got, err := storeRewriteID("id: \"mon-effet\"\nnom: X\n", "mon-effet-2")
	if err != nil || !strings.Contains(got, "mon-effet-2") || strings.Contains(got, "\"mon-effet\"") {
		t.Errorf("valeur entre guillemets : %q %v", got, err)
	}
}
