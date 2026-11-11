package effects

// Chargement, surcharge, restauration, erreurs §4.7 et limites §3.2 — contracts/light-effects.md §3, §4.7.
// Aucun Sleep : tout est synchrone (LoadDir est une fonction pure du contenu des dossiers).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodNative = `
id: base
name: Base
description: effet de base
version: 1
moments: [calm]
looks: { a: { color: scene } }
pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }
`

func userEffect(id string) string {
	return "id: " + id + "\nname: " + strings.ToUpper(id[:1]) + id[1:] + "\nversion: 1\nmoments: [calm]\nlooks: { a: { color: scene } }\npattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }\n"
}

func TestLoad_UserEffectIsAddedToCatalogue(t *testing.T) {
	dir := t.TempDir()
	writeUser(t, dir, "mon-effet.yaml", userEffect("mon-effet"))
	k := loadFS(memNative(map[string]string{"base.yaml": goodNative}), dir)
	if o, f, ok := k.get("base"); !ok || o != "native" {
		t.Errorf("base : origine native attendue : %q %q %v", o, f, ok)
	}
	if o, f, ok := k.get("mon-effet"); !ok || o != "user" || !strings.HasSuffix(f, "mon-effet.yaml") {
		t.Errorf("mon-effet : origine user attendue : %q %q %v", o, f, ok)
	}
	if len(k.errors()) != 0 {
		t.Errorf("aucune erreur attendue : %+v", k.errors())
	}
}

func TestLoad_UserFileWithNativeIDOverrides_RemovalRestoresNative(t *testing.T) {
	dir := t.TempDir()
	native := memNative(map[string]string{"base.yaml": goodNative})
	writeUser(t, dir, "copie.yaml", strings.Replace(goodNative, "name: Base", "name: Ma version", 1))
	k := loadFS(native, dir)
	if o, _, _ := k.get("base"); o != "override" {
		t.Fatalf("un fichier utilisateur de même id surcharge le natif : origine %q", o)
	}
	if e, _ := k.effect("base"); e.Name != "Ma version" {
		t.Errorf("la définition utilisateur l'emporte : %q", e.Name)
	}
	if err := os.Remove(filepath.Join(dir, "copie.yaml")); err != nil {
		t.Fatal(err)
	}
	k = loadFS(native, dir) // rechargement
	if o, _, _ := k.get("base"); o != "native" {
		t.Errorf("supprimer le fichier restaure le natif : origine %q", o)
	}
	if e, _ := k.effect("base"); e.Name != "Base" {
		t.Errorf("le natif est restauré : %q", e.Name)
	}
}

func TestLoad_RenamedWithoutYamlExtensionIsIgnored(t *testing.T) {
	dir := t.TempDir()
	writeUser(t, dir, "copie.yaml.off", strings.Replace(goodNative, "name: Base", "name: Off", 1))
	k := loadFS(memNative(map[string]string{"base.yaml": goodNative}), dir)
	if o, _, _ := k.get("base"); o != "native" {
		t.Errorf("un fichier renommé hors .yaml est ignoré : %q", o)
	}
	if len(k.errors()) != 0 {
		t.Errorf("%+v", k.errors())
	}
}

func TestLoad_SubdirectoriesAreIgnored_NativesCopyNeverRead(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"natifs", "autre"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
		writeUser(t, filepath.Join(dir, sub), "x.yaml", strings.Replace(goodNative, "name: Base", "name: Piege", 1))
	}
	k := loadFS(memNative(map[string]string{"base.yaml": goodNative}), dir)
	if e, _ := k.effect("base"); e.Name != "Base" {
		t.Errorf("les natifs viennent du binaire, jamais de natifs/ ni d'un sous-dossier : %q", e.Name)
	}
}

func TestLoad_MissingUserDirIsNotAnError(t *testing.T) {
	k := loadFS(memNative(map[string]string{"base.yaml": goodNative}), filepath.Join(t.TempDir(), "absent"))
	if _, _, ok := k.get("base"); !ok || len(k.errors()) != 0 {
		t.Errorf("dossier utilisateur absent : natifs seuls, sans erreur (%+v)", k.errors())
	}
}

func TestLoad_DuplicateUserID_FirstAlphabeticalKept_OtherReported(t *testing.T) {
	dir := t.TempDir()
	writeUser(t, dir, "b.yaml", strings.Replace(userEffect("dup"), "name: Dup", "name: Seconde", 1))
	writeUser(t, dir, "a.yaml", strings.Replace(userEffect("dup"), "name: Dup", "name: Premiere", 1))
	k := loadFS(memNative(nil), dir)
	if e, ok := k.effect("dup"); !ok || e.Name != "Premiere" {
		t.Errorf("le premier par ordre alphabétique de nom de fichier est retenu : %+v", e)
	}
	if !hasErrorFor(k.errors(), "b.yaml") {
		t.Errorf("le doublon b.yaml est signalé : %+v", k.errors())
	}
}

// ---------- chaque erreur du §4.7 : fichier + ligne + message français ----------

func hasErrorFor(errs []Error, file string) bool {
	for _, e := range errs {
		if e.File == file {
			return true
		}
	}
	return false
}

func errorFor(errs []Error, file string) (Error, bool) {
	for _, e := range errs {
		if e.File == file {
			return e, true
		}
	}
	return Error{}, false
}

func TestLoad_InvalidFiles_EachProducesLocatedFrenchError_OthersSurvive(t *testing.T) {
	head := "id: x\nname: X\nversion: 1\nmoments: [calm]\n"
	okTail := "looks: { a: { color: scene } }\npattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }\n"
	cases := []struct {
		name     string
		src      string
		wantLine bool // la ligne doit être connue (> 0)
		wantText string
	}{
		{"yaml-illisible.yaml", "id: x\n  name: [pas fermé\n", true, ""},
		{"cle-inconnue.yaml", head + "bidule: 3\n" + okTail, true, "bidule"},
		{"sans-id.yaml", "name: X\nversion: 1\nmoments: [calm]\n" + okTail, false, "id"},
		{"sans-looks.yaml", head + "pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }\n", false, "looks"},
		{"sans-pattern.yaml", head + "looks: { a: { color: scene } }\n", false, "pattern"},
		{"id-invalide.yaml", "id: Mon Effet\nname: X\nversion: 1\nmoments: [calm]\n" + okTail, true, "id"},
		{"id-trop-long.yaml", "id: " + strings.Repeat("a", 41) + "\nname: X\nversion: 1\nmoments: [calm]\n" + okTail, true, "id"},
		{"version-2.yaml", "id: x\nname: X\nversion: 2\nmoments: [calm]\n" + okTail, true, "version"},
		{"moments-vide.yaml", "id: x\nname: X\nversion: 1\nmoments: []\n" + okTail, true, "moments"},
		{"moment-inconnu.yaml", "id: x\nname: X\nversion: 1\nmoments: [foo]\n" + okTail, true, "foo"},
		{"type-inconnu.yaml", head + "looks: { a: { color: scene } }\npattern: { type: spirale }\n", true, "spirale"},
		{"pas-trop-court.yaml", head + "looks: { a: { color: scene } }\npattern: { type: shift, step_ms: 100, cells: [a], rest: a }\n", true, "step_ms"},
		{"pas-trop-long.yaml", head + "looks: { a: { color: scene } }\npattern: { type: shift, step_ms: 70000, cells: [a], rest: a }\n", true, "step_ms"},
		{"phase-trop-courte.yaml", head + "looks: { a: { color: scene } }\npattern: { type: uniform, phases: [ { look: a, ms: 100 } ] }\n", true, "ms"},
		{"look-non-defini.yaml", head + "looks: { a: { color: scene } }\npattern: { type: uniform, phases: [ { look: tete, ms: 1000 } ] }\n", true, "tete"},
		{"couleur-invalide.yaml", head + "looks: { a: { color: \"#GG0000\" } }\npattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }\n", true, "couleur"},
		{"intensite-hors-bornes.yaml", head + "looks: { a: { color: scene, intensity: 150 } }\npattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }\n", true, "intensity|intensité"},
		{"fondu-hors-bornes.yaml", head + "looks: { a: { color: scene, fade_ms: 9000 } }\npattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }\n", true, "fade_ms"},
		{"score-sans-duree.yaml", "id: x\nname: X\nversion: 1\nmoments: [score]\n" + okTail, false, "duration_ms"},
		{"trop-d-etapes.yaml", head + okTail + "stages:\n" + strings.Repeat("  - when: { groups_lt: 5 }\n    pattern: { phases: [ { look: a, ms: 1000 } ] }\n", 7), true, "stages|étapes"},
		{"trop-de-phases.yaml", head + "looks: { a: { color: scene } }\npattern: { type: uniform, phases: [" + strings.TrimSuffix(strings.Repeat("{ look: a, ms: 500 }, ", 9), ", ") + "] }\n", true, "phases"},
	}
	dir := t.TempDir()
	writeUser(t, dir, "valide.yaml", userEffect("valide"))
	for _, c := range cases {
		writeUser(t, dir, c.name, c.src)
	}
	k := loadFS(memNative(map[string]string{"base.yaml": goodNative}), dir)

	if _, _, ok := k.get("valide"); !ok {
		t.Error("un fichier invalide ne casse ni les autres effets utilisateur…")
	}
	if _, _, ok := k.get("base"); !ok {
		t.Error("…ni les natifs")
	}
	for _, c := range cases {
		e, ok := errorFor(k.errors(), c.name)
		if !ok {
			t.Errorf("%s : aucune erreur signalée (%+v)", c.name, k.errors())
			continue
		}
		full := e.Error() // « fichier, ligne N : cause »
		if e.File != c.name || !strings.HasPrefix(full, c.name) {
			t.Errorf("%s : l'erreur doit nommer le fichier : %+v / %q", c.name, e, full)
		}
		if c.wantLine && e.Line <= 0 {
			t.Errorf("%s : la ligne doit être connue : %+v", c.name, e)
		}
		if c.wantText != "" {
			found := false
			for _, alt := range strings.Split(c.wantText, "|") {
				if strings.Contains(strings.ToLower(e.Message), strings.ToLower(alt)) {
					found = true
				}
			}
			if !found {
				t.Errorf("%s : le message devrait mentionner %q : %q", c.name, c.wantText, e.Message)
			}
		}
		if c.wantLine && !strings.Contains(full, "ligne") {
			t.Errorf("%s : forme « fichier, ligne N : cause » : %q", c.name, full)
		}
	}
	if _, _, ok := k.get("x"); ok {
		t.Error("aucun des fichiers invalides (id x) ne doit entrer au catalogue")
	}
}

func TestLoad_ErrorMessageExampleFromContract(t *testing.T) {
	dir := t.TempDir()
	writeUser(t, dir, "chenillard.yaml", "id: chenillard\nname: C\nversion: 1\nmoments: [chrono]\nlooks: { fond: { color: theme } }\n"+
		"pattern:\n  type: shift\n  step_ms: 1000\n  cells: [tete]\n  rest: fond\n")
	e, ok := errorFor(loadFS(memNative(nil), dir).errors(), "chenillard.yaml")
	if !ok || e.Line <= 0 || !strings.Contains(e.Error(), "chenillard.yaml, ligne") || !strings.Contains(e.Message, "« tete »") || !strings.Contains(e.Message, "looks") {
		t.Fatalf("forme du §4.7 : « chenillard.yaml, ligne N : l'apparence « tete » n'est pas définie dans looks » — obtenu %+v", e)
	}
}

func TestLoad_InvalidUserOverrideKeepsNativeActive(t *testing.T) {
	dir := t.TempDir()
	writeUser(t, dir, "casse.yaml", "id: base\nname: Cassé\nversion: 1\nmoments: [calm]\nbidule: 1\n")
	k := loadFS(memNative(map[string]string{"base.yaml": goodNative}), dir)
	if o, _, ok := k.get("base"); !ok || o != "native" {
		t.Errorf("un fichier de même id invalide est écarté : le natif reste actif (origine %q)", o)
	}
	if !hasErrorFor(k.errors(), "casse.yaml") {
		t.Error("l'erreur est signalée")
	}
}

// ---------- limites ----------

func TestLoad_FileOver64KiBIsRejected(t *testing.T) {
	dir := t.TempDir()
	big := userEffect("gros") + "# " + strings.Repeat("x", 65*1024) + "\n"
	writeUser(t, dir, "gros.yaml", big)
	k := loadFS(memNative(nil), dir)
	if _, _, ok := k.get("gros"); ok {
		t.Error("un fichier de plus de 64 Ko est écarté")
	}
	if !hasErrorFor(k.errors(), "gros.yaml") {
		t.Errorf("fichier trop gros signalé : %+v", k.errors())
	}
}

func TestLoad_FileJustUnder64KiBAccepted(t *testing.T) {
	dir := t.TempDir()
	src := userEffect("limite")
	src += "# " + strings.Repeat("x", 64*1024-len(src)-4) + "\n"
	writeUser(t, dir, "limite.yaml", src)
	if _, _, ok := loadFS(memNative(nil), dir).get("limite"); !ok {
		t.Error("un fichier de 64 Ko pile est accepté")
	}
}

func TestLoad_MoreThan100UserFiles_ExcessReported(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 101; i++ {
		id := "e" + strings.Repeat("a", i/26) + string(rune('a'+i%26))
		writeUser(t, dir, id+".yaml", userEffect(id))
	}
	k := loadFS(memNative(nil), dir)
	if len(k.errors()) == 0 {
		t.Error("au-delà de 100 fichiers, le surplus est signalé")
	}
}

// ---------- Parse seul : valeurs par défaut ----------

func TestParse_Defaults_MinGroupsOneTargetGroupsDurationZero(t *testing.T) {
	e := mustParse(t, goodNative)
	if e.MinGroups != 1 {
		t.Errorf("min_groups par défaut = 1, obtenu %d", e.MinGroups)
	}
	if e.Target != "groups" {
		t.Errorf("target par défaut = groups, obtenu %q", e.Target)
	}
	if e.DurationMs != 0 {
		t.Errorf("duration_ms par défaut = 0, obtenu %d", e.DurationMs)
	}
}

func TestParse_Description500CharsMax(t *testing.T) {
	long := strings.Replace(goodNative, "description: effet de base", "description: "+strings.Repeat("d", 501), 1)
	if _, errs := parseEffect(long); len(errs) == 0 {
		t.Error("une description de plus de 500 caractères est refusée")
	}
	short := strings.Replace(goodNative, "description: effet de base", "description: "+strings.Repeat("d", 500), 1)
	if _, errs := parseEffect(short); len(errs) != 0 {
		t.Errorf("500 caractères acceptés : %+v", errs)
	}
}

func TestParse_NameMax40Chars(t *testing.T) {
	if _, errs := parseEffect(strings.Replace(goodNative, "name: Base", "name: "+strings.Repeat("n", 41), 1)); len(errs) == 0 {
		t.Error("un nom de plus de 40 caractères est refusé")
	}
}

// ---------- applicabilité §4.6 ----------

func TestApplicable_MinGroupsAndEmptyGroups(t *testing.T) {
	e := mustParse(t, chenillardSrc) // min_groups: 3, target groups
	for n, want := range map[int]bool{0: false, 1: false, 2: false, 3: true, 12: true} {
		ok, reason := applicable(e, n)
		if ok != want {
			t.Errorf("chenillard N=%d : applicable=%v, attendu %v", n, ok, want)
		}
		if !ok && reason == "" {
			t.Errorf("N=%d : une raison est exigée (« demande 3 groupes, %d configurés »)", n, n)
		}
	}
}
