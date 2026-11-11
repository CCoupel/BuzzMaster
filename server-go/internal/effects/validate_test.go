package effects

// Chargement, surcharge, restauration, erreurs §4.8 (fichier + ligne, français), limites §3.2, §4.1, valeurs par défaut.
// Format pas à pas (version: 2, mots-clés français). Aucun Sleep : LoadDir est une fonction pure du contenu des dossiers.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodNative = `
id: base
nom: Base
description: effet de base
version: 2
moments: [calm]
duree_tick_ms: 1000
pas:
  - { g0: [scene, 100] }
`

func userEffect(id string) string {
	return "id: " + id + "\nnom: " + strings.ToUpper(id[:1]) + id[1:] + "\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g0: [scene, 100] }\n"
}

func TestLoad_UserEffectIsAddedToCatalogue(t *testing.T) {
	dir := t.TempDir()
	writeUser(t, dir, "mon-effet.yaml", userEffect("mon-effet"))
	k := loadFS(memNative(map[string]string{"base.yaml": goodNative}), dir)
	if o, _, ok := k.get("base"); !ok || o != "native" {
		t.Errorf("base : origine native attendue : %q %v", o, ok)
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
	writeUser(t, dir, "copie.yaml", strings.Replace(goodNative, "nom: Base", "nom: Ma version", 1))
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
	k = loadFS(native, dir)
	if o, _, _ := k.get("base"); o != "native" {
		t.Errorf("fichier supprimé : retour au natif : %q", o)
	}
}

func TestLoad_RenamedWithoutYamlExtensionIsIgnored(t *testing.T) {
	dir := t.TempDir()
	writeUser(t, dir, "copie.yaml.off", strings.Replace(goodNative, "nom: Base", "nom: Off", 1))
	k := loadFS(memNative(map[string]string{"base.yaml": goodNative}), dir)
	if o, _, _ := k.get("base"); o != "native" || len(k.errors()) != 0 {
		t.Errorf("fichier renommé hors .yaml ignoré : %q %+v", o, k.errors())
	}
}

func TestLoad_SubdirectoriesAreIgnored_NativesCopyNeverRead(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"natifs", "autre"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
		writeUser(t, filepath.Join(dir, sub), "x.yaml", strings.Replace(goodNative, "nom: Base", "nom: Piege", 1))
	}
	k := loadFS(memNative(map[string]string{"base.yaml": goodNative}), dir)
	if e, _ := k.effect("base"); e.Name != "Base" {
		t.Errorf("les natifs viennent du binaire : %q", e.Name)
	}
}

func TestLoad_MissingUserDirIsNotAnError(t *testing.T) {
	k := loadFS(memNative(map[string]string{"base.yaml": goodNative}), filepath.Join(t.TempDir(), "absent"))
	if _, _, ok := k.get("base"); !ok || len(k.errors()) != 0 {
		t.Errorf("dossier utilisateur absent : natifs seuls : %+v", k.errors())
	}
}

func TestLoad_DuplicateUserID_FirstAlphabeticalKept_OtherReported(t *testing.T) {
	dir := t.TempDir()
	writeUser(t, dir, "b.yaml", strings.Replace(userEffect("dup"), "nom: Dup", "nom: Seconde", 1))
	writeUser(t, dir, "a.yaml", strings.Replace(userEffect("dup"), "nom: Dup", "nom: Premiere", 1))
	k := loadFS(memNative(nil), dir)
	if e, ok := k.effect("dup"); !ok || e.Name != "Premiere" {
		t.Errorf("le premier par ordre alphabétique de nom de fichier est retenu : %+v", e)
	}
	if !hasErrorFor(k.errors(), "b.yaml") {
		t.Errorf("le doublon b.yaml est signalé : %+v", k.errors())
	}
}

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

// ---------- chaque erreur du §4.8 : fichier + ligne + message français ----------

func TestLoad_InvalidFiles_EachProducesLocatedFrenchError_OthersSurvive(t *testing.T) {
	head := "id: x\nnom: X\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\n"
	okPas := "pas:\n  - { g0: [scene, 100] }\n"
	cases := []struct {
		name     string
		src      string
		wantLine bool
		wantText string // alternatives séparées par |, insensible à la casse
	}{
		{"yaml-illisible.yaml", "id: x\n  nom: [pas fermé\n", true, ""},
		{"cle-inconnue.yaml", head + "bidule: 3\n" + okPas, true, "bidule"},
		{"cle-anglaise.yaml", "id: x\nname: X\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\n" + okPas, true, "name"},
		{"sans-id.yaml", "nom: X\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\n" + okPas, false, "id"},
		{"sans-nom.yaml", "id: x\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\n" + okPas, false, "nom"},
		{"sans-pas.yaml", head, false, "pas"},
		{"sans-tick.yaml", "id: x\nnom: X\nversion: 2\nmoments: [calm]\n" + okPas, false, "duree_tick_ms"},
		{"id-invalide.yaml", "id: Mon Effet\nnom: X\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\n" + okPas, true, "id"},
		{"id-trop-long.yaml", "id: " + strings.Repeat("a", 41) + "\nnom: X\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\n" + okPas, true, "id"},
		{"version-3.yaml", "id: x\nnom: X\nversion: 3\nmoments: [calm]\nduree_tick_ms: 1000\n" + okPas, true, "version"},
		{"moments-vide.yaml", "id: x\nnom: X\nversion: 2\nmoments: []\nduree_tick_ms: 1000\n" + okPas, true, "moments"},
		{"moment-inconnu.yaml", "id: x\nnom: X\nversion: 2\nmoments: [foo]\nduree_tick_ms: 1000\n" + okPas, true, "foo"},
		{"tick-trop-court.yaml", "id: x\nnom: X\nversion: 2\nmoments: [calm]\nduree_tick_ms: 100\n" + okPas, true, "duree_tick_ms"},
		{"tick-trop-long.yaml", "id: x\nnom: X\nversion: 2\nmoments: [calm]\nduree_tick_ms: 70000\n" + okPas, true, "duree_tick_ms"},
		{"pas-vide.yaml", head + "pas: []\n", true, "pas"},
		{"trop-de-pas.yaml", head + "pas:\n" + strings.Repeat("  - { g1: [scene, 100] }\n", 201), true, "pas"},
		{"cible-mal-ecrite.yaml", head + "pas:\n  - { groupe1: [scene, 100] }\n", true, "groupe1"},
		{"cible-g-seule.yaml", head + "pas:\n  - { g: [scene, 100] }\n", true, "g"},
		{"variable-non-declaree.yaml", head + "pas:\n  - { gB: [scene, 100] }\n", true, "gB"},
		{"voisin-non-declare.yaml", head + "pas:\n  - { gB+1: [scene, 100] }\n", true, "gB"},
		{"declaration-mal-ecrite.yaml", head + "variables:\n  gA: partir de g1\npas:\n  - { gA: [scene, 100] }\n", true, "gA"},
		{"increment-hors-bornes.yaml", head + "variables:\n  gA: g1; +13\npas:\n  - { gA: [scene, 100] }\n", true, "gA"},
		{"variable-nommee-gN.yaml", head + "variables:\n  gN: g1; +1\npas:\n  - { gN: [scene, 100] }\n", true, "gN"},
		{"trop-de-variables.yaml", head + "variables:\n" + variablesN(9) + "pas:\n  - { gA: [scene, 100] }\n", true, "variables"},
		{"couleur-invalide.yaml", head + "pas:\n  - { g1: [\"#GG0000\", 100] }\n", true, "couleur"},
		{"couleur-inconnue.yaml", head + "pas:\n  - { g1: [rose, 100] }\n", true, "rose"},
		{"intensite-trop-haute.yaml", head + "pas:\n  - { g1: [scene, 150] }\n", true, "intensité|intensite"},
		{"intensite-negative.yaml", head + "pas:\n  - { g1: [scene, -5] }\n", true, "intensité|intensite"},
		{"action-mal-formee.yaml", head + "pas:\n  - { g1: scene }\n", true, "g1"},
		{"pas-vide-action.yaml", head + "pas:\n  - { }\n", true, "pas|action"},
		{"trop-d-actions.yaml", head + abcDecl + "pas:\n  - { " + actionsN(17) + " }\n", true, "action"},
		{"boucle-invalide.yaml", head + "boucle: peut-etre\n" + okPas, true, "boucle"},
		{"dans-pre.yaml", head + "pre:\n  { gZ: [scene, 100] }\n" + okPas, true, "gZ"},
		{"dans-dernier.yaml", head + "dernier:\n  { gZ: [scene, 100] }\n" + okPas, true, "gZ"},
		{"dans-post.yaml", head + "post:\n  { gZ: [scene, 100] }\n" + okPas, true, "gZ"},
		{"groupes-min-negatif.yaml", head + "groupes_min: -1\n" + okPas, true, "groupes_min"},
		{"description-trop-longue.yaml", head + "description: " + strings.Repeat("d", 501) + "\n" + okPas, true, "description"},
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
		full := e.Error()
		if e.File != c.name || !strings.HasPrefix(full, c.name) {
			t.Errorf("%s : l'erreur doit nommer le fichier : %+v / %q", c.name, e, full)
		}
		if c.wantLine && e.Line <= 0 {
			t.Errorf("%s : la ligne doit être connue : %+v", c.name, e)
		}
		if c.wantLine && !strings.Contains(full, "ligne") {
			t.Errorf("%s : forme « fichier, ligne N : cause » : %q", c.name, full)
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
	}
	if _, _, ok := k.get("x"); ok {
		t.Error("aucun des fichiers invalides (id x) n'entre au catalogue")
	}
}

func variablesN(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("  g" + string(rune('A'+i)) + ": g1; +1\n")
	}
	return b.String()
}

// actionsN : n actions distinctes (g0, g1…g12, gN, gA, gB, gC) — les variables gA, gB, gC doivent être déclarées.
func actionsN(n int) string {
	targets := []string{"g0", "g1", "g2", "g3", "g4", "g5", "g6", "g7", "g8", "g9", "g10", "g11", "g12", "gN", "gA", "gB", "gC"}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = targets[i] + ": [scene, 100]"
	}
	return strings.Join(parts, ", ")
}

const abcDecl = "variables:\n  gA: g1; +1\n  gB: g2; +1\n  gC: g3; +1\n"

func TestLoad_ErrorMessageExampleFromContract_VariableNotDeclared(t *testing.T) {
	dir := t.TempDir()
	writeUser(t, dir, "mon-effet.yaml", "id: mon-effet\nnom: M\nversion: 2\nmoments: [chrono]\nduree_tick_ms: 1000\n"+
		"variables:\n  gA: g1; +1\npas:\n  - { gA: [theme, 100],\n      gB: [theme, 50] }\n")
	e, ok := errorFor(loadFS(memNative(nil), dir).errors(), "mon-effet.yaml")
	if !ok || e.Line <= 0 || !strings.Contains(e.Error(), "mon-effet.yaml, ligne") ||
		!strings.Contains(e.Message, "« gB »") || !strings.Contains(e.Message, "variables") {
		t.Fatalf("forme du §4.8 : « mon-effet.yaml, ligne N : la variable « gB » n'est pas déclarée dans variables » — obtenu %+v", e)
	}
}

func TestLoad_Version1File_RefusedWithTheDedicatedMessage(t *testing.T) {
	dir := t.TempDir()
	writeUser(t, dir, "ancien.yaml", "id: ancien\nname: Ancien\nversion: 1\nmoments: [calm]\nlooks: { a: { color: scene } }\n"+
		"pattern: { type: uniform, phases: [ { look: a, ms: 1000 } ] }\n")
	k := loadFS(memNative(map[string]string{"base.yaml": goodNative}), dir)
	e, ok := errorFor(k.errors(), "ancien.yaml")
	if !ok || !strings.Contains(e.Message, "format d'effet de la version précédente : réécrire en pas à pas") {
		t.Fatalf("un fichier version: 1 est refusé avec son message dédié : %+v", e)
	}
	if _, _, ok := k.get("ancien"); ok {
		t.Error("l'ancien effet n'entre pas au catalogue")
	}
	if _, _, ok := k.get("base"); !ok {
		t.Error("les autres effets restent disponibles")
	}
}

func TestLoad_InvalidUserOverrideKeepsNativeActive(t *testing.T) {
	dir := t.TempDir()
	writeUser(t, dir, "casse.yaml", "id: base\nnom: Cassé\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\nbidule: 1\npas:\n  - { g0: [scene, 100] }\n")
	k := loadFS(memNative(map[string]string{"base.yaml": goodNative}), dir)
	if o, _, ok := k.get("base"); !ok || o != "native" {
		t.Errorf("un fichier de même id invalide est écarté : le natif reste actif (%q)", o)
	}
	if !hasErrorFor(k.errors(), "casse.yaml") {
		t.Error("l'erreur est signalée")
	}
}

// ---------- limites ----------

func TestLoad_FileOver64KiBIsRejected_ExactlyAt64KiBAccepted(t *testing.T) {
	dir := t.TempDir()
	writeUser(t, dir, "gros.yaml", userEffect("gros")+"# "+strings.Repeat("x", 65*1024)+"\n")
	src := userEffect("limite")
	writeUser(t, dir, "limite.yaml", src+"# "+strings.Repeat("x", 64*1024-len(src)-4)+"\n")
	k := loadFS(memNative(nil), dir)
	if _, _, ok := k.get("gros"); ok || !hasErrorFor(k.errors(), "gros.yaml") {
		t.Errorf("plus de 64 Ko : écarté et signalé : %+v", k.errors())
	}
	if _, _, ok := k.get("limite"); !ok {
		t.Error("64 Ko pile : accepté")
	}
}

func TestLoad_MoreThan100UserFiles_ExcessReported(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 101; i++ {
		id := "e" + strings.Repeat("a", i/26) + string(rune('a'+i%26))
		writeUser(t, dir, id+".yaml", userEffect(id))
	}
	if k := loadFS(memNative(nil), dir); len(k.errors()) == 0 {
		t.Error("au-delà de 100 fichiers, le surplus est signalé")
	}
}

// ---------- valeurs par défaut et bornes acceptées ----------

func TestParse_Defaults(t *testing.T) {
	e := mustParse(t, goodNative)
	if e.MinGroups != 0 {
		t.Errorf("groupes_min par défaut = 0, obtenu %d", e.MinGroups)
	}
	if e.TickMs != 1000 {
		t.Errorf("duree_tick_ms conservé : %d", e.TickMs)
	}
	if e.Name != "Base" || e.ID != "base" {
		t.Errorf("id et nom : %+v", e)
	}
}

func TestParse_AcceptedBoundaries(t *testing.T) {
	head := "id: x\nnom: X\nversion: 2\nmoments: [calm, chrono, score]\n"
	for name, src := range map[string]string{
		"tick 250":          head + "duree_tick_ms: 250\npas:\n  - { g0: [scene, 100] }\n",
		"tick 60000":        head + "duree_tick_ms: 60000\npas:\n  - { g0: [scene, 100] }\n",
		"200 pas":           head + "duree_tick_ms: 1000\npas:\n" + strings.Repeat("  - { g1: [scene, 100] }\n", 200),
		"8 variables":       head + "duree_tick_ms: 1000\nvariables:\n" + variablesN(8) + "pas:\n  - { gA: [scene, 100] }\n",
		"16 actions":        head + "duree_tick_ms: 1000\n" + abcDecl + "pas:\n  - { " + actionsN(16) + " }\n",
		"description 500":   head + "duree_tick_ms: 1000\ndescription: " + strings.Repeat("d", 500) + "\npas:\n  - { g0: [scene, 100] }\n",
		"boucle non":        head + "duree_tick_ms: 1000\nboucle: non\npas:\n  - { g0: [scene, 100] }\n",
		"groupes_min 12":    head + "duree_tick_ms: 1000\ngroupes_min: 12\npas:\n  - { g1: [scene, 100] }\n",
		"couleur hex 0":     head + "duree_tick_ms: 1000\npas:\n  - { g1: [\"#000000\", 0] }\n",
		"nom 40 caractères": "id: x\nnom: " + strings.Repeat("n", 40) + "\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g0: [scene, 100] }\n",
	} {
		if _, errs := parseEffect(src); len(errs) != 0 {
			t.Errorf("%s : accepté, obtenu %+v", name, errs)
		}
	}
}

func TestParse_NameOver40Chars_Refused(t *testing.T) {
	if _, errs := parseEffect(strings.Replace(goodNative, "nom: Base", "nom: "+strings.Repeat("n", 41), 1)); len(errs) == 0 {
		t.Error("un nom de plus de 40 caractères est refusé")
	}
}

func TestParse_NamedColoursAndEteintAccepted(t *testing.T) {
	src := fx("pas:\n  - { g1: [theme, 100], g2: [equipe, 50], g3: [urgence, 100], g4: [scene, 100], g5: [eteint] }\n")
	if _, errs := parseEffect(src); len(errs) != 0 {
		t.Errorf("couleurs nommées et [eteint] : %+v", errs)
	}
}

func TestParse_AllTargetForms_Accepted(t *testing.T) {
	src := fx("variables:\n  gA: g1; +1\n  gB: gN; -1\n  gC: [g1, g4, g3, g7]\n  gD: g3; 0\npas:\n  - { g0: [scene, 100], g1: [scene, 100], gN: [scene, 100], gA: [scene, 100], gA+1: [scene, 100], gA-2: [scene, 100], gC: [scene, 100] }\n")
	if _, errs := parseEffect(src); len(errs) != 0 {
		t.Errorf("g0, gn, gN, variables, voisins, liste : %+v", errs)
	}
}

func TestParse_PreStepsLastPost_AllOptional_StepsOnlyRequired(t *testing.T) {
	if _, errs := parseEffect(fx("pas:\n  - { g0: [scene, 100] }\n")); len(errs) != 0 {
		t.Errorf("seuls id, nom, version, moments, duree_tick_ms, pas sont obligatoires : %+v", errs)
	}
}
