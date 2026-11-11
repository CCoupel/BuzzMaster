package effects

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ReloadInterval is the period of the directory-signature check (§3.3).
const ReloadInterval = 2 * time.Second

// Manager owns the live catalogue: it loads at start, reloads when the
// directory signature changes, and swaps the catalogue atomically.
type Manager struct {
	embedded fs.FS
	dir      string
	logf     func(format string, args ...any)

	cat atomic.Pointer[Catalogue]
	gen atomic.Uint64

	mu       sync.Mutex // serialises Reload and guards sig / errState
	sig      string
	errState map[string]string // file -> last logged message (one line per state change)
}

// NewManager loads the catalogue once. logf may be nil.
func NewManager(embedded fs.FS, dir string, logf func(format string, args ...any)) *Manager {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	m := &Manager{embedded: embedded, dir: dir, logf: logf, errState: map[string]string{}}
	m.Reload()
	return m
}

// Catalogue returns the current snapshot (never nil).
func (m *Manager) Catalogue() Catalogue { return *m.cat.Load() }

// Dir is the user directory.
func (m *Manager) Dir() string { return m.dir }

// Reload forces an immediate reload and returns the new catalogue.
func (m *Manager) Reload() Catalogue {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reloadLocked()
}

func (m *Manager) reloadLocked() Catalogue {
	m.sig = dirSignature(m.dir)
	next := LoadDir(m.embedded, m.dir)
	if prev := m.cat.Load(); prev != nil {
		next = carryOver(*prev, next)
	}
	next.Generation = m.gen.Add(1)
	m.logChanges(next.errs)
	m.cat.Store(&next)
	return next
}

// carryOver keeps, for a user file that just became invalid, the last valid
// definition of its effect that was in memory (§3.3).
func carryOver(prev, next Catalogue) Catalogue {
	if len(next.errs) == 0 {
		return next
	}
	bad := map[string]bool{}
	for _, e := range next.errs {
		bad[e.File] = true
	}
	changed := false
	for _, pe := range prev.entries {
		if pe.Origin == OriginNative || !bad[pe.File] {
			continue
		}
		cur, exists := next.Get(pe.Effect.ID)
		if exists && cur.Origin != OriginNative {
			continue // a valid user file already defines this id
		}
		if exists { // the native it overrides: put the previous version back on top
			pe.Origin = OriginOverride
			next.entries[next.byID[pe.Effect.ID]] = pe
		} else {
			next.entries = append(next.entries, pe)
		}
		changed = true
	}
	if changed {
		next.index()
	}
	return next
}

func (m *Manager) logChanges(errs []Error) {
	now := map[string]string{}
	for _, e := range errs {
		if _, dup := now[e.File]; !dup {
			now[e.File] = e.Error()
		}
	}
	files := make([]string, 0, len(now))
	for f := range now {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		if m.errState[f] != now[f] {
			m.logf("Effet lumière écarté — %s", now[f])
		}
	}
	for f := range m.errState {
		if _, still := now[f]; !still {
			m.logf("Effet lumière : %s est de nouveau valide", f)
		}
	}
	m.errState = now
}

// Run checks the directory signature every ReloadInterval and reloads on
// change, until ctx is done.
func (m *Manager) Run(ctx context.Context) {
	t := time.NewTicker(ReloadInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.mu.Lock()
			if dirSignature(m.dir) != m.sig {
				m.reloadLocked()
			}
			m.mu.Unlock()
		}
	}
}

// dirSignature fingerprints the `*.yaml` files of dir (names, sizes, mtimes).
func dirSignature(dir string) string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "absent"
	}
	var b strings.Builder
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".yaml") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "%s|%d|%d;", e.Name(), fi.Size(), fi.ModTime().UnixNano())
	}
	return b.String()
}

// EnsureDirs creates the user directory (and its `natifs/` reference folder)
// and rewrites the reference copy of the embedded natives, with a "do not edit
// here" header (§3.2). The server never reads that copy back.
func EnsureDirs(embedded fs.FS, dir string) error {
	ref := filepath.Join(dir, "natifs")
	if err := os.MkdirAll(ref, 0o755); err != nil {
		return err
	}
	if embedded == nil {
		return nil
	}
	const header = "# COPIE DE RÉFÉRENCE — ne pas modifier ici.\n# Ce dossier est réécrit à chaque démarrage et n'est jamais lu par le serveur.\n# Pour changer cet effet : copiez ce fichier dans le dossier parent (data/files/effects/),\n# gardez le même « id », puis modifiez la copie.\n\n"
	for _, name := range yamlNames(embedded, ".") {
		data, err := fs.ReadFile(embedded, name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(ref, name), append([]byte(header), data...), 0o644); err != nil {
			return err
		}
	}
	return nil
}
