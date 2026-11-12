package effects

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v4"
)

// store.go — writing, reading and deleting the user effect files (v11.8.0, #305, #210 —
// contracts/light-effects.md §11.4 to §11.7). Every operation runs under the Manager's
// lock and ends with a reload, so the catalogue never lags behind the disk. Nothing is
// ever written outside the user directory nor in `natifs/`.

// Reasons carried by a StoreError (the `reason` of the 409 answers, §11.4).
const (
	ReasonExists      = "exists"      // create on an id already in the catalogue
	ReasonNative      = "native"      // the id is a shipped effect's (E2)
	ReasonLimit       = "limit"       // 100 files reached
	ReasonNotFound    = "unknown"     // unknown id (404)
	ReasonReserved    = "reserved"    // id is a route word (400)
	ReasonInvalid     = "invalid"     // validation failed (Errors) or reserved id
	ReasonTooBig      = "too_big"     // more than 64 KB
	ReasonIO          = "io"          // disk failure
	ReasonUnavailable = "unavailable" // no storage behind the manager (503)
)

// ReservedIDs are route words: no effect may take them as id.
var ReservedIDs = []string{"reload", "try", "draft", "import"}

// StoreError is a refused write or delete.
type StoreError struct {
	Code   string
	Errors []Error // validation errors (ReasonInvalid), without file name
	Err    error   // underlying I/O error (ReasonIO)
}

func (e *StoreError) Error() string {
	switch {
	case len(e.Errors) > 0:
		return e.Errors[0].Message
	case e.Err != nil:
		return e.Err.Error()
	}
	return e.Code
}

// Reason is the `reason` of the HTTP answer (the Code).
func (e *StoreError) Reason() string { return e.Code }

func (e *StoreError) Unwrap() error { return e.Err }

// IsReserved reports whether id is a reserved route word.
func IsReserved(id string) bool {
	for _, r := range ReservedIDs {
		if id == r {
			return true
		}
	}
	return false
}

// Write statuses of an import (§11.7).
const (
	StatusImported = "imported"
	StatusReplaced = "replaced"
	StatusRenamed  = "renamed"
	StatusConflict = "conflict"
	StatusNative   = "native"
	StatusInvalid  = "invalid"
	StatusTooBig   = "too_big"
	StatusLimit    = "limit"
)

// On-conflict policies of an import.
const (
	ConflictAsk      = "ask"
	ConflictReplace  = "replace"
	ConflictKeepBoth = "keep_both"
)

// ImportFile is one uploaded file; Name only labels the messages, it is never a path.
type ImportFile struct {
	Name string
	Data []byte
}

// ImportExisting describes the user effect an imported file collides with.
type ImportExisting struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// ImportResult is the outcome of one imported file (§11.7).
type ImportResult struct {
	File     string          `json:"file"`
	Status   string          `json:"status"`
	ID       string          `json:"id,omitempty"`
	Name     string          `json:"name,omitempty"`
	Stored   string          `json:"stored,omitempty"`
	NewID    string          `json:"new_id,omitempty"`
	Existing *ImportExisting `json:"existing,omitempty"`
	Errors   []ImportError   `json:"errors,omitempty"`
}

// ImportError is a refusal cause: line (0 when unknown) and message.
type ImportError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

func importErrors(errs []Error) []ImportError {
	out := make([]ImportError, 0, len(errs))
	for _, e := range errs {
		out = append(out, ImportError{Line: e.Line, Message: e.Message})
	}
	return out
}

// Source returns the text of the effect id as it is stored: a user file as read, a native
// as embedded (without the reference-copy header). The entry tells its origin and file.
func (m *Manager) Source(id string) ([]byte, Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sourceLocked(m.cat.Load(), id)
}

func (m *Manager) sourceLocked(c *Catalogue, id string) ([]byte, Entry, error) {
	en, ok := c.Get(id)
	if !ok {
		return nil, Entry{}, &StoreError{Code: ReasonNotFound}
	}
	if en.Origin == OriginNative {
		if m.embedded == nil {
			return nil, en, &StoreError{Code: ReasonNotFound}
		}
		data, err := fs.ReadFile(m.embedded, strings.TrimPrefix(en.File, "natifs/"))
		if err != nil {
			return nil, en, &StoreError{Code: ReasonIO, Err: err}
		}
		return data, en, nil
	}
	data, err := readLimited(filepath.Join(m.dir, en.File))
	if err != nil {
		// The file vanished or grew between the reload and now.
		return nil, en, &StoreError{Code: ReasonIO, Err: err}
	}
	return data, en, nil
}

// Write stores the text of effect id as a user file and reloads. create=true refuses an id
// already in the catalogue (ReasonExists); create=false refuses an unknown one
// (ReasonNotFound). An id of a shipped effect (native or overridden) is ReasonNative. The
// text is written as is. It returns the file name used.
func (m *Manager) Write(id string, data []byte, create bool) (string, Catalogue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cat := m.reloadLocked()
	if IsReserved(id) {
		return "", cat, &StoreError{Code: ReasonReserved}
	}
	if len(data) > MaxFileBytes {
		return "", cat, &StoreError{Code: ReasonTooBig}
	}
	e, errs := checkText("", data)
	if len(errs) > 0 {
		return "", cat, validationError(errs)
	}
	if e.ID != id {
		return "", cat, &StoreError{Code: ReasonInvalid, Errors: []Error{{Line: idLine(data), Message: "l'identifiant ne se change pas : dupliquez l'effet"}}}
	}
	if en, ok := cat.Get(id); ok {
		switch {
		case en.Origin != OriginUser:
			return "", cat, &StoreError{Code: ReasonNative}
		case create:
			return "", cat, &StoreError{Code: ReasonExists}
		}
	} else if !create {
		return "", cat, &StoreError{Code: ReasonNotFound}
	}
	name, err := m.putLocked(&cat, id, data)
	if err != nil {
		return "", cat, err
	}
	return name, m.reloadLocked(), nil
}

// Delete removes the user file of effect id (a user effect, or the override of a native,
// which then comes back) and reloads. A native without override is ReasonNative.
func (m *Manager) Delete(id string) (Catalogue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cat := m.reloadLocked()
	en, ok := cat.Get(id)
	if !ok {
		return cat, &StoreError{Code: ReasonNotFound}
	}
	if en.Origin == OriginNative {
		return cat, &StoreError{Code: ReasonNative}
	}
	if err := os.Remove(m.userPath(en.File)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return cat, &StoreError{Code: ReasonIO, Err: err}
	}
	return m.reloadLocked(), nil
}

// Import validates and stores each file on its own (a refused file never stops the others),
// then reloads once and returns the catalogue. onConflict: ConflictAsk (default),
// ConflictReplace or ConflictKeepBoth.
func (m *Manager) Import(files []ImportFile, onConflict string) ([]ImportResult, Catalogue) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cat := m.reloadLocked().clone() // the batch mutates its own copy, never the published snapshot
	if onConflict != ConflictReplace && onConflict != ConflictKeepBoth {
		onConflict = ConflictAsk
	}
	results := make([]ImportResult, 0, len(files))
	wrote := false
	for _, f := range files {
		r := m.importOne(&cat, f, onConflict)
		if r.Stored != "" {
			wrote = true
		}
		results = append(results, r)
	}
	if wrote {
		cat = m.reloadLocked()
	}
	return results, cat
}

func (m *Manager) importOne(cat *Catalogue, f ImportFile, onConflict string) ImportResult {
	r := ImportResult{File: f.Name}
	label := safeLabel(f.Name)
	r.File = label
	if len(f.Data) > MaxFileBytes {
		r.Status = StatusTooBig
		r.Errors = []ImportError{{Message: fmt.Sprintf("fichier trop gros (%d Ko au plus)", MaxFileBytes>>10)}}
		return r
	}
	e, errs := checkText(label, f.Data)
	if len(errs) > 0 {
		r.Status = StatusInvalid
		r.Errors = importErrors(errs)
		return r
	}
	r.ID, r.Name = e.ID, e.Name
	data := f.Data
	en, exists := cat.Get(e.ID)
	status := StatusImported
	switch {
	case exists && en.Origin != OriginUser:
		r.Status = StatusNative
		r.Errors = []ImportError{{Line: idLine(data), Message: "« " + e.ID + " » est l'identifiant d'un effet livré : changez la clé id"}}
		return r
	case exists && onConflict == ConflictAsk:
		r.Status = StatusConflict
		r.Existing = &ImportExisting{Name: en.Effect.Name, File: en.File}
		return r
	case exists && onConflict == ConflictReplace:
		status = StatusReplaced
	case exists: // keep both: the first free suffixed id
		newID := freeID(*cat, e.ID)
		rewritten, ok := rewriteID(data, newID)
		if !ok {
			r.Status = StatusInvalid
			r.Errors = []ImportError{{Line: idLine(data), Message: "la clé id ne peut pas être réécrite : changez-la à la main"}}
			return r
		}
		if e2, errs2 := checkText(label, rewritten); len(errs2) > 0 || e2.ID != newID {
			r.Status = StatusInvalid
			r.Errors = []ImportError{{Line: idLine(data), Message: "la clé id ne peut pas être réécrite : changez-la à la main"}}
			return r
		}
		data, status = rewritten, StatusRenamed
		r.NewID = newID
	}
	id := e.ID
	if r.NewID != "" {
		id = r.NewID
	}
	name, err := m.putLocked(cat, id, data)
	if err != nil {
		var se *StoreError
		if errors.As(err, &se) && se.Code == ReasonLimit {
			r.Status = StatusLimit
			r.Errors = []ImportError{{Message: fmt.Sprintf("%d fichiers d'effets atteints : supprimez-en avant d'importer", MaxFiles)}}
			return r
		}
		r.Status = StatusInvalid
		r.Errors = []ImportError{{Message: "écriture impossible : " + err.Error()}}
		return r
	}
	// Record the write in the working snapshot so that the next file of the batch sees it.
	cat.noteUser(id, name, e)
	r.Status, r.Stored = status, name
	return r
}

// putLocked writes data as the file of effect id: the file that already defines it, else
// <id>.yaml, else <id>-2.yaml… Creating a new file past the limit is ReasonLimit.
func (m *Manager) putLocked(cat *Catalogue, id string, data []byte) (string, error) {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return "", &StoreError{Code: ReasonIO, Err: err}
	}
	names := m.dirNames()
	name := ""
	if en, ok := cat.Get(id); ok && en.Origin != OriginNative && baseName(en.File) {
		name = en.File
	}
	if name == "" {
		if len(names) >= MaxFiles {
			return "", &StoreError{Code: ReasonLimit}
		}
		taken := map[string]bool{}
		for _, n := range names {
			taken[strings.ToLower(n)] = true
		}
		name = id + ".yaml"
		for i := 2; taken[strings.ToLower(name)]; i++ {
			name = id + "-" + strconv.Itoa(i) + ".yaml"
		}
	}
	if err := writeAtomic(m.dir, name, data); err != nil {
		return "", &StoreError{Code: ReasonIO, Err: err}
	}
	return name, nil
}

// clone copies the entries and the index (the published snapshot is shared with readers).
func (c Catalogue) clone() Catalogue {
	c.entries = append([]Entry(nil), c.entries...)
	by := make(map[string]int, len(c.byID))
	for k, v := range c.byID {
		by[k] = v
	}
	c.byID = by
	return c
}

// noteUser records a freshly written user effect in the snapshot (import batches).
func (c *Catalogue) noteUser(id, file string, e Effect) {
	e.ID = id
	en := Entry{Effect: e, Origin: OriginUser, File: file}
	if i, ok := c.byID[id]; ok {
		c.entries[i] = en
		return
	}
	c.entries = append(c.entries, en)
	c.byID[id] = len(c.entries) - 1
}

// dirNames lists the `*.yaml` files of the user directory (any case of the extension).
func (m *Manager) dirNames() []string { return yamlNames(os.DirFS(m.dir), ".") }

// userPath is the path of a user file; the name is always a bare file name.
func (m *Manager) userPath(file string) string { return filepath.Join(m.dir, filepath.Base(file)) }

func baseName(file string) bool {
	return file != "" && file == filepath.Base(file) && !strings.ContainsAny(file, `/\`)
}

// writeAtomic writes dir/name through a temporary file of the same directory (its name does
// not end in `.yaml`, the loader never sees it) and a rename.
func writeAtomic(dir, name string, data []byte) error {
	tmp, err := os.CreateTemp(dir, ".effet-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	if err = os.Rename(tmpName, filepath.Join(dir, name)); err != nil {
		return err
	}
	ok = true
	return nil
}

// checkText validates a text to be stored: size, UTF-8, no YAML alias (a bounded analysis —
// an alias can expand a small file into a huge one), the schema of §4 and the reserved ids.
func checkText(file string, data []byte) (Effect, []Error) {
	if len(data) > MaxFileBytes {
		return Effect{}, []Error{{File: file, Message: fmt.Sprintf("fichier trop gros (%d Ko au plus)", MaxFileBytes>>10)}}
	}
	if !utf8.Valid(data) {
		return Effect{}, []Error{{File: file, Message: "le fichier n'est pas en UTF-8"}}
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err == nil {
		if line := aliasLine(&root); line > 0 {
			return Effect{}, []Error{{File: file, Line: line, Message: "les alias YAML (*nom) ne sont pas acceptés : écrivez les valeurs en toutes lettres"}}
		}
	}
	e, errs := ParseFile(file, data)
	if len(errs) > 0 {
		return Effect{}, errs
	}
	if IsReserved(e.ID) {
		return Effect{}, []Error{{File: file, Line: idLine(data), Message: "« " + e.ID + " » est un mot réservé : changez la clé id"}}
	}
	if IsWindowsDevice(e.ID) {
		return Effect{}, []Error{{File: file, Line: idLine(data), Message: "« " + e.ID + " » est un nom réservé par Windows : changez la clé id"}}
	}
	return e, nil
}

// IsWindowsDevice reports whether id, used as a file name (<id>.yaml), is a device name that
// Windows reserves: con, prn, aux, nul, com1-9, lpt1-9 (the server also runs on Windows).
func IsWindowsDevice(id string) bool {
	switch id {
	case "con", "prn", "aux", "nul":
		return true
	}
	return len(id) == 4 && (strings.HasPrefix(id, "com") || strings.HasPrefix(id, "lpt")) && id[3] >= '1' && id[3] <= '9'
}

func aliasLine(n *yaml.Node) int {
	if n.Kind == yaml.AliasNode {
		return n.Line
	}
	for _, c := range n.Content {
		if l := aliasLine(c); l > 0 {
			return l
		}
	}
	return 0
}

func validationError(errs []Error) *StoreError {
	out := make([]Error, len(errs))
	for i, e := range errs {
		e.File = ""
		out[i] = e
	}
	return &StoreError{Code: ReasonInvalid, Errors: out}
}

// safeLabel keeps only the final element of an uploaded name, for messages.
func safeLabel(name string) string {
	name = strings.ReplaceAll(name, `\`, "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return "(sans nom)"
	}
	return name
}

var reIDLine = regexp.MustCompile(`^id[ \t]*:[ \t]*("[^"\n]*"|'[^'\n]*'|[^\s#]+)`)

// idLine is the line (1-based) of the top-level `id` key, 0 when not found.
func idLine(data []byte) int {
	for i, ln := range strings.Split(string(data), "\n") {
		if reIDLine.MatchString(ln) {
			return i + 1
		}
	}
	return 0
}

// rewriteID replaces the value of the top-level `id` key and nothing else (comments and the
// rest of the line included).
func rewriteID(data []byte, newID string) ([]byte, bool) {
	lines := strings.Split(string(data), "\n")
	for i, ln := range lines {
		loc := reIDLine.FindStringSubmatchIndex(ln)
		if loc == nil {
			continue
		}
		lines[i] = ln[:loc[2]] + newID + ln[loc[3]:]
		return []byte(strings.Join(lines, "\n")), true
	}
	return nil, false
}

// freeID returns the first of <id>-2, <id>-3… (the stem cut so that the id stays within
// MaxIDLen) that no effect of the catalogue uses.
func freeID(c Catalogue, id string) string {
	for i := 2; ; i++ {
		suffix := "-" + strconv.Itoa(i)
		stem := id
		if len(stem)+len(suffix) > MaxIDLen {
			stem = strings.TrimRight(stem[:MaxIDLen-len(suffix)], "-")
		}
		cand := stem + suffix
		if _, taken := c.Get(cand); !taken && !IsReserved(cand) {
			return cand
		}
	}
}

// WriteEffect is Write for callers that only need the file name and the error.
func (m *Manager) WriteEffect(id, text string, create bool) (string, error) {
	f, _, err := m.Write(id, []byte(text), create)
	return f, err
}

// DeleteEffect is Delete without the catalogue.
func (m *Manager) DeleteEffect(id string) error {
	_, err := m.Delete(id)
	return err
}

// EffectText is the stored text of effect id (Source without the entry).
func (m *Manager) EffectText(id string) (string, error) {
	data, _, err := m.Source(id)
	return string(data), err
}

// RewriteID returns text with the value of its top-level `id` key replaced by newID, the rest
// (comments included) untouched.
func RewriteID(text, newID string) (string, error) {
	out, ok := rewriteID([]byte(text), newID)
	if !ok {
		return "", &StoreError{Code: ReasonInvalid, Errors: []Error{{Message: "clé id introuvable"}}}
	}
	return string(out), nil
}

// ValidateText is the validation an effect text goes through before it is stored or drafted:
// size, UTF-8, no YAML alias, the schema of §4 / §12, reserved ids and Windows device names.
// Errors carry no file name.
func ValidateText(data []byte) (Effect, []Error) { return checkText("", data) }

// EffectsWrite is Write seen through the server's EffectsStore interface (it returns the
// reloaded catalogue).
func (m *Manager) EffectsWrite(id string, data []byte, create bool) (string, Catalogue, error) {
	return m.Write(id, data, create)
}
