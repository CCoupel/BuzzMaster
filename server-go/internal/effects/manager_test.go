package effects

// Rechargement à chaud — contracts/light-effects.md §3.3 (via Manager.Reload : synchrone, aucune attente).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type logSink struct{ lines []string }

func (l *logSink) logf(format string, args ...any) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func newMgr(t *testing.T, dir string) (*Manager, *logSink) {
	t.Helper()
	l := &logSink{}
	return NewManager(memNative(map[string]string{"base.yaml": goodNative}), dir, l.logf), l
}

func TestManager_Reload_PicksUpNewEditedAndDeletedFiles_GenerationIncreases(t *testing.T) {
	dir := t.TempDir()
	m, _ := newMgr(t, dir)
	g0 := m.Catalogue().Generation
	if _, ok := m.Catalogue().Get("nouveau"); ok {
		t.Fatal("absent au départ")
	}
	writeUser(t, dir, "nouveau.yaml", userEffect("nouveau"))
	c := m.Reload()
	if _, ok := c.Get("nouveau"); !ok || c.Generation <= g0 {
		t.Fatalf("un fichier ajouté est pris en compte sans redémarrage ; génération %d → %d", g0, c.Generation)
	}
	writeUser(t, dir, "nouveau.yaml", strings.Replace(userEffect("nouveau"), "name: Nouveau", "name: Renomme", 1))
	if e, _ := m.Reload().Get("nouveau"); e.Effect.Name != "Renomme" {
		t.Errorf("fichier modifié rechargé : %q", e.Effect.Name)
	}
	if err := os.Remove(filepath.Join(dir, "nouveau.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Reload().Get("nouveau"); ok {
		t.Error("fichier supprimé : l'effet disparaît")
	}
}

func TestManager_UserFileBecomesInvalid_PreviousValidDefinitionKept(t *testing.T) {
	dir := t.TempDir()
	m, _ := newMgr(t, dir)
	writeUser(t, dir, "mien.yaml", userEffect("mien"))
	m.Reload()
	writeUser(t, dir, "mien.yaml", "id: mien\nname: Casse\nbidule: 1\n")
	c := m.Reload()
	e, ok := c.Get("mien")
	if !ok || e.Effect.Name != "Mien" {
		t.Errorf("la version valide précédente reste active (§3.3) : %+v %v", e.Effect.Name, ok)
	}
	if !hasErrorFor(c.Errors(), "mien.yaml") {
		t.Error("l'erreur reste signalée")
	}
}

func TestManager_UserOverrideBecomesInvalid_OverrideDefinitionKept_NotNative(t *testing.T) {
	dir := t.TempDir()
	m, _ := newMgr(t, dir)
	writeUser(t, dir, "base2.yaml", strings.Replace(goodNative, "name: Base", "name: Perso", 1))
	m.Reload()
	writeUser(t, dir, "base2.yaml", "id: base\nbidule: 1\n")
	e, _ := m.Reload().Get("base")
	if e.Effect.Name != "Perso" || e.Origin != OriginOverride {
		t.Errorf("la dernière définition valide de la surcharge est conservée : %+v", e)
	}
}

func TestManager_NeverBrokenFileAtStart_FallsBackToNative(t *testing.T) {
	dir := t.TempDir()
	writeUser(t, dir, "base2.yaml", "id: base\nbidule: 1\n")
	m, _ := newMgr(t, dir)
	e, ok := m.Catalogue().Get("base")
	if !ok || e.Origin != OriginNative {
		t.Errorf("au démarrage, fichier invalide sans version précédente ⇒ natif : %+v", e)
	}
}

func TestManager_LogsOncePerStateChange_NotOncePerReload(t *testing.T) {
	dir := t.TempDir()
	m, l := newMgr(t, dir)
	writeUser(t, dir, "casse.yaml", "id: casse\nbidule: 1\n")
	m.Reload()
	n := len(l.lines)
	if n != 1 {
		t.Fatalf("une ligne de journal à l'apparition de l'erreur, obtenu %d : %v", n, l.lines)
	}
	for i := 0; i < 5; i++ {
		m.Reload()
	}
	if len(l.lines) != n {
		t.Errorf("jamais une ligne par cycle : %d lignes après 5 rechargements identiques", len(l.lines))
	}
	if err := os.Remove(filepath.Join(dir, "casse.yaml")); err != nil {
		t.Fatal(err)
	}
	m.Reload()
	if len(l.lines) != n+1 {
		t.Errorf("une ligne au retour à la normale : %v", l.lines)
	}
}

func TestEnsureDirs_WritesReferenceCopyWithHeader_AndServerNeverReadsIt(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "effects")
	native := memNative(map[string]string{"base.yaml": goodNative})
	if err := EnsureDirs(native, dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "natifs", "base.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.HasPrefix(s, "#") || !strings.Contains(strings.ToLower(s), "ne pas modifier") || !strings.Contains(s, "id: base") {
		t.Errorf("copie de référence avec en-tête « ne pas modifier ici » : %q", s[:min(len(s), 200)])
	}
	// modifier la copie est sans effet : les natifs viennent du binaire
	if err := os.WriteFile(filepath.Join(dir, "natifs", "base.yaml"), []byte("id: base\nname: Piege\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if e, _ := NewManager(native, dir, nil).Catalogue().Get("base"); e.Effect.Name != "Base" {
		t.Errorf("natifs/ n'est jamais lu : %q", e.Effect.Name)
	}
	// réécrite à chaque démarrage
	if err := EnsureDirs(native, dir); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "natifs", "base.yaml")); !strings.Contains(string(b), "id: base\nname: Base") {
		t.Error("la copie de référence est réécrite au démarrage")
	}
}
