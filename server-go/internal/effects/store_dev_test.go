package effects

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// #305 B1.1 — import batch, id rewriting, bounded YAML (dev tests, next to the spec tests).

func TestImport_StatusesAndOneReload_Dev(t *testing.T) {
	t.Parallel()
	m, dir := tw305Manager(t)
	g0 := m.Catalogue().Generation
	res, cat := m.Import([]ImportFile{
		{Name: "a.yaml", Data: []byte(tw305Src("neuf", ""))},
		{Name: "dup.yaml", Data: []byte(tw305Src("neuf", ""))}, // same id twice in one batch
		{Name: "nat.yaml", Data: []byte(tw305Src("base", ""))},
		{Name: "bad.yaml", Data: []byte("id: x\nnom: [\n")},
		{Name: "../../evil.yml", Data: []byte(tw305Src("evil", ""))},
	}, ConflictAsk)
	got := []string{}
	for _, r := range res {
		got = append(got, r.Status)
	}
	want := []string{StatusImported, StatusConflict, StatusNative, StatusInvalid, StatusImported}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("statuts %v, attendu %v", got, want)
	}
	if res[4].File != "evil.yml" || res[4].Stored != "evil.yaml" {
		t.Fatalf("le nom envoyé ne doit pas être un chemin : %+v", res[4])
	}
	if cat.Generation != g0+2 { // one reload at the start of Import, one at its end
		t.Fatalf("génération %d, attendu %d (un seul rechargement final)", cat.Generation, g0+2)
	}
	for _, id := range []string{"neuf", "evil"} {
		if _, ok := cat.Get(id); !ok {
			t.Fatalf("%s absent du catalogue", id)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "natifs", "evil.yaml")); err == nil {
		t.Fatal("écrit dans natifs/")
	}
}

func TestImport_ReplaceAndKeepBoth_Dev(t *testing.T) {
	t.Parallel()
	m, dir := tw305Manager(t)
	if _, reason := storeWrite(m, "mon", tw305Src("mon", ""), true); reason != "" {
		t.Fatal(reason)
	}
	src := "# garde-moi\n" + tw305Src("mon", "description: v2\n")
	res, _ := m.Import([]ImportFile{{Name: "x.yaml", Data: []byte(src)}}, ConflictReplace)
	if res[0].Status != StatusReplaced || res[0].Stored != "mon.yaml" {
		t.Fatalf("replace : %+v", res[0])
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "mon.yaml")); string(b) != src {
		t.Fatal("le texte doit être écrit tel quel")
	}
	res, cat := m.Import([]ImportFile{{Name: "x.yaml", Data: []byte(src)}}, ConflictKeepBoth)
	if res[0].Status != StatusRenamed || res[0].NewID != "mon-2" || res[0].Stored != "mon-2.yaml" {
		t.Fatalf("keep_both : %+v", res[0])
	}
	b, _ := os.ReadFile(filepath.Join(dir, "mon-2.yaml"))
	if string(b) != strings.Replace(src, "id: mon\n", "id: mon-2\n", 1) {
		t.Fatalf("seule la valeur de id doit changer :\n%s", b)
	}
	if _, ok := cat.Get("mon-2"); !ok {
		t.Fatal("mon-2 absent")
	}
	res, _ = m.Import([]ImportFile{{Name: "x.yaml", Data: []byte(src)}}, ConflictKeepBoth)
	if res[0].NewID != "mon-3" {
		t.Fatalf("premier id libre attendu mon-3 : %+v", res[0])
	}
}

func TestImport_KeepBothTruncatesLongID_Dev(t *testing.T) {
	t.Parallel()
	m, _ := tw305Manager(t)
	id := strings.Repeat("a", MaxIDLen)
	src := strings.Replace(tw305Src(id, ""), "nom: Effet "+id, "nom: Long", 1)
	m.Import([]ImportFile{{Name: "l.yaml", Data: []byte(src)}}, ConflictAsk)
	res, _ := m.Import([]ImportFile{{Name: "l.yaml", Data: []byte(src)}}, ConflictKeepBoth)
	if len(res[0].NewID) > MaxIDLen || !strings.HasSuffix(res[0].NewID, "-2") || res[0].Status != StatusRenamed {
		t.Fatalf("%+v", res[0])
	}
}

func TestImport_RefusesAliasesAndReserved_Dev(t *testing.T) {
	t.Parallel()
	m, dir := tw305Manager(t)
	alias := "id: al\nnom: &n Al\ndescription: *n\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g1: [\"#FF0000\", 100] }\n"
	res, _ := m.Import([]ImportFile{
		{Name: "al.yaml", Data: []byte(alias)},
		{Name: "r.yaml", Data: []byte(tw305Src("import", ""))},
		{Name: "big.yaml", Data: []byte(strings.Repeat("#", MaxFileBytes+1))},
	}, ConflictAsk)
	if res[0].Status != StatusInvalid || res[0].Errors[0].Line != 3 {
		t.Fatalf("alias : %+v", res[0])
	}
	if res[1].Status != StatusInvalid || res[1].Errors[0].Line != 1 {
		t.Fatalf("réservé : %+v", res[1])
	}
	if res[2].Status != StatusTooBig {
		t.Fatalf("too_big : %+v", res[2])
	}
	for _, n := range tw305Yaml(t, dir) {
		t.Fatalf("rien ne devait être écrit : %s", n)
	}
}

func TestImport_LimitAndNoTempLeft_Dev(t *testing.T) {
	t.Parallel()
	m, dir := tw305Manager(t)
	var batch []ImportFile
	for i := 0; i < MaxFiles+2; i++ {
		batch = append(batch, ImportFile{Name: "f.yaml", Data: []byte(tw305Src("e"+strings.Repeat("x", i%5)+string(rune('a'+i/5)), ""))})
	}
	res, _ := m.Import(batch, ConflictAsk)
	n := 0
	for _, r := range res {
		if r.Status == StatusLimit {
			n++
		}
	}
	if n == 0 {
		t.Fatal("la limite de 100 fichiers doit être signalée")
	}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("fichier temporaire resté : %s", e.Name())
		}
	}
}

func TestRewriteID_KeepsComment_Dev(t *testing.T) {
	t.Parallel()
	got, err := RewriteID("id: ab # le nom\nnom: X\n", "ab-2")
	if err != nil || got != "id: ab-2 # le nom\nnom: X\n" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := RewriteID("nom: X\n", "a"); err == nil {
		t.Fatal("erreur attendue sans clé id")
	}
}
