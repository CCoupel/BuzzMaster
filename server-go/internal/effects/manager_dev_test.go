package effects

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

const devNativeSrc = `id: base
nom: Base
version: 2
moments: [calm]
duree_tick_ms: 1000
pas:
  - { g0: [scene, 100] }
`

func devUser(id, name string) string {
	return "id: " + id + "\nnom: " + name + "\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\npas:\n  - { g0: [scene, 100] }\n"
}

func devWrite(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

type devLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *devLog) logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, format)
}

func TestManager_ReloadOverrideCarryOverAndLogOncePerChange_Dev(t *testing.T) {
	t.Parallel()
	native := fstestMap(map[string]string{"base.yaml": devNativeSrc})
	dir := t.TempDir()
	lg := &devLog{}
	m := NewManager(native, dir, lg.logf)
	if e, ok := m.Catalogue().Get("base"); !ok || e.Origin != OriginNative {
		t.Fatalf("natif attendu : %+v", e)
	}
	g0 := m.Catalogue().Generation

	devWrite(t, dir, "mon.yaml", devUser("base", "Surcharge"))
	c := m.Reload()
	if e, _ := c.Get("base"); e.Origin != OriginOverride || e.Effect.Name != "Surcharge" || c.Generation <= g0 {
		t.Fatalf("surcharge attendue : %+v gen=%d", e, c.Generation)
	}

	// The file becomes invalid: the previous valid version stays active, one log line, and repeated reloads stay silent.
	devWrite(t, dir, "mon.yaml", "id: base\nname: [")
	for i := 0; i < 3; i++ {
		c = m.Reload()
	}
	if e, _ := c.Get("base"); e.Effect.Name != "Surcharge" {
		t.Fatalf("la version précédente doit rester active : %+v", e)
	}
	if len(c.Errors()) != 1 {
		t.Fatalf("une erreur signalée : %+v", c.Errors())
	}
	if len(lg.lines) != 1 {
		t.Fatalf("une seule ligne de journal par changement d'état : %v", lg.lines)
	}

	// Deleted file: back to the native.
	if err := os.Remove(filepath.Join(dir, "mon.yaml")); err != nil {
		t.Fatal(err)
	}
	c = m.Reload()
	if e, _ := c.Get("base"); e.Origin != OriginNative || len(c.Errors()) != 0 {
		t.Fatalf("retour au natif attendu : %+v %+v", e, c.Errors())
	}
}

func TestManager_RunReloadsOnSignatureChange_Dev(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := NewManager(nil, dir, nil)
	if _, ok := m.Catalogue().Get("x"); ok {
		t.Fatal("catalogue vide au départ")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)
	devWrite(t, dir, "x.yaml", devUser("x", "X"))
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := m.Catalogue().Get("x"); ok {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("le rechargement à chaud n'a pas vu le nouveau fichier")
}

func TestEnsureDirs_WritesReferenceCopyWithHeader_Dev(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "effects")
	if err := EnsureDirs(fstestMap(map[string]string{"base.yaml": devNativeSrc}), dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "natifs", "base.yaml"))
	if err != nil || !strings.HasPrefix(string(data), "# COPIE DE RÉFÉRENCE") || !strings.Contains(string(data), "id: base") {
		t.Fatalf("copie de référence : %v %q", err, data)
	}
	// The reference folder is never read back: it holds no user effect.
	if c := LoadDir(nil, dir); len(c.List()) != 0 {
		t.Fatalf("natifs/ ne doit jamais être lu : %+v", c.List())
	}
}

func fstestMap(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for n, s := range files {
		m[n] = &fstest.MapFile{Data: []byte(s)}
	}
	return m
}
