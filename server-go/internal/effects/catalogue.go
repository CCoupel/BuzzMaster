package effects

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Origins of a catalogue entry (contract §7.1).
const (
	OriginNative   = "native"
	OriginUser     = "user"
	OriginOverride = "override" // a user file that replaces a native of the same id
)

// NativeOrder is the display order of the native effects (contract §9). A
// native not listed here follows, alphabetically.
var NativeOrder = []string{"pulsation", "chenillard", "chenillard-traine", "degrade", "couleur-equipe"}

// Entry is one effect of the catalogue.
type Entry struct {
	Effect Effect
	Origin string // OriginNative | OriginUser | OriginOverride
	File   string // "natifs/<name>" for a native, "<name>" for a user file
}

// Catalogue is an immutable snapshot of the valid effects and of the files
// that were set aside. Safe for concurrent reads; reloading builds a new one.
type Catalogue struct {
	entries []Entry
	byID    map[string]int
	errs    []Error
	// Generation increases at every reload of a Manager (0 for a bare
	// LoadDir): a reader compares it to restart an effect whose definition may
	// have changed.
	Generation uint64
}

// Get returns the effect with the given id.
func (c Catalogue) Get(id string) (Entry, bool) {
	i, ok := c.byID[id]
	if !ok {
		return Entry{}, false
	}
	return c.entries[i], true
}

// List returns the entries in display order: natives (contract §9 order), then
// user effects by name. The slice is a copy.
func (c Catalogue) List() []Entry { return append([]Entry(nil), c.entries...) }

// Errors returns the files that were set aside, with file, line and cause.
func (c Catalogue) Errors() []Error { return append([]Error(nil), c.errs...) }

// LoadDir builds the catalogue from the embedded natives (`*.yaml` at the root
// of embedded) and the user directory dir (§3.2). It never fails: a file that
// cannot be read or is invalid is reported in Errors() and the others stay
// available. A missing dir is an empty user directory.
func LoadDir(embedded fs.FS, dir string) Catalogue {
	var c Catalogue
	natives := map[string]Entry{}
	if embedded != nil {
		names := yamlNames(embedded, ".")
		for _, name := range names {
			data, err := fs.ReadFile(embedded, name)
			if err != nil {
				c.errs = append(c.errs, Error{File: "natifs/" + name, Message: "fichier natif illisible : " + err.Error()})
				continue
			}
			e, errs := ParseFile("natifs/"+name, data)
			if len(errs) > 0 {
				c.errs = append(c.errs, errs...)
				continue
			}
			if _, dup := natives[e.ID]; dup {
				c.errs = append(c.errs, Error{File: "natifs/" + name, Message: "identifiant « " + e.ID + " » déjà utilisé par un autre effet natif"})
				continue
			}
			natives[e.ID] = Entry{Effect: e, Origin: OriginNative, File: "natifs/" + name}
		}
	}

	users := map[string]Entry{}
	if dir != "" {
		names := yamlNames(os.DirFS(dir), ".")
		if len(names) > MaxFiles {
			c.errs = append(c.errs, Error{File: filepath.Base(dir), Message: "trop de fichiers : seuls les 100 premiers (ordre alphabétique) sont lus"})
			names = names[:MaxFiles]
		}
		for _, name := range names {
			data, err := readLimited(filepath.Join(dir, name))
			if err != nil {
				c.errs = append(c.errs, Error{File: name, Message: err.Error()})
				continue
			}
			e, errs := ParseFile(name, data)
			if len(errs) > 0 {
				c.errs = append(c.errs, errs...)
				continue
			}
			if first, dup := users[e.ID]; dup {
				c.errs = append(c.errs, Error{File: name, Message: "identifiant « " + e.ID + " » déjà utilisé par " + first.File + " (le premier par ordre alphabétique est retenu)"})
				continue
			}
			origin := OriginUser
			if _, isNative := natives[e.ID]; isNative {
				origin = OriginOverride
			}
			users[e.ID] = Entry{Effect: e, Origin: origin, File: name}
		}
	}

	// Display order: natives in contract order (a native overridden by a user
	// file keeps its place), then the other user effects by name.
	placed := map[string]bool{}
	add := func(id string) {
		if placed[id] {
			return
		}
		if u, ok := users[id]; ok {
			c.entries = append(c.entries, u)
			placed[id] = true
		} else if n, ok := natives[id]; ok {
			c.entries = append(c.entries, n)
			placed[id] = true
		}
	}
	for _, id := range NativeOrder {
		add(id)
	}
	var rest []string
	for id := range natives {
		rest = append(rest, id)
	}
	sort.Strings(rest)
	for _, id := range rest {
		add(id)
	}
	var userOnly []Entry
	for id, u := range users {
		if !placed[id] {
			userOnly = append(userOnly, u)
		}
	}
	sort.Slice(userOnly, func(i, j int) bool {
		a, b := userOnly[i].Effect, userOnly[j].Effect
		if a.Name != b.Name {
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		return a.ID < b.ID
	})
	c.entries = append(c.entries, userOnly...)
	c.index()
	return c
}

func (c *Catalogue) index() {
	c.byID = make(map[string]int, len(c.entries))
	for i, e := range c.entries {
		c.byID[e.Effect.ID] = i
	}
}

// yamlNames lists the regular `*.yaml` files directly in dir, sorted.
func yamlNames(fsys fs.FS, dir string) []string {
	ents, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".yaml") {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func readLimited(path string) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.Size() > MaxFileBytes {
		return nil, errTooBig
	}
	return os.ReadFile(path)
}

type constError string

func (e constError) Error() string { return string(e) }

const errTooBig = constError("fichier trop gros (64 Ko au plus)")
