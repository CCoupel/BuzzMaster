// Category management API (v11.4.0, #285, contracts/http-endpoints.md
// §Catégories): resolution, listing, creation, rename/colour, icon, deletion.
//
// Storage (data/files/categories/):
//   - built-in categories (hardcodedCategories) have NO image; an optional
//     sidecar <KEY>.json {name?, color?} overrides their name / colour;
//   - custom categories exist by their image <KEY>.<ext>; the sidecar carries
//     {name, color?}.
//
// Security: every key received from a URL is validated BEFORE any path join,
// and a disk file name is always derived from the validated key + an
// allowlisted extension, never from an uploaded Filename.
package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"buzzcontrol/internal/game"
)

// maxCategoryIconBytes is the icon size cap (2 Mio).
const maxCategoryIconBytes = 2 << 20

var (
	categoryColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	// categoryKeyRe: keys are toUpperSnakeCase(name) — upper-case letters/digits/underscore.
	// Unicode letters are accepted (a name like "Cinéma" yields "CINÉMA"), but never
	// a separator, a dot or anything that could leave the categories directory.
	categoryKeyRe = regexp.MustCompile(`^[\p{L}\p{N}_]{1,50}$`)
)

var categoryImageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}

// categorySidecar is <KEY>.json. Pointers distinguish "absent" from "" (an
// explicit « no colour » on a built-in category).
type categorySidecar struct {
	Name  *string `json:"name,omitempty"`
	Color *string `json:"color,omitempty"`
}

func (h *HTTPServer) categoriesDir() string { return filepath.Join(h.dataDir, "files", "categories") }

func (h *HTTPServer) questionsDir() string { return filepath.Join(h.dataDir, "files", "questions") }

func readCategorySidecar(catDir, key string) categorySidecar {
	var sc categorySidecar
	if data, err := os.ReadFile(filepath.Join(catDir, key+".json")); err == nil {
		_ = json.Unmarshal(data, &sc)
	}
	return sc
}

// writeCategorySidecar writes <KEY>.json atomically (tmp + rename).
func writeCategorySidecar(catDir, key string, sc categorySidecar) error {
	data, err := json.Marshal(sc)
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(catDir, key+".json"), data)
}

func writeFileAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, 0644); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// listCategories returns the 8 built-ins (table + sidecar override) then the
// custom ones (image + sidecar), without counts. A custom image whose key
// collides with a built-in is skipped and journalised, as before.
func (h *HTTPServer) listCategories() []CategoryInfo {
	catDir := h.categoriesDir()
	builtin := make(map[string]struct{}, len(hardcodedCategories))
	out := make([]CategoryInfo, 0, len(hardcodedCategories)+4)
	for _, c := range hardcodedCategories {
		builtin[c.Key] = struct{}{}
		sc := readCategorySidecar(catDir, c.Key)
		if sc.Name != nil && *sc.Name != "" {
			c.Name = *sc.Name
		}
		if sc.Color != nil {
			c.Color = *sc.Color
		}
		out = append(out, c)
	}
	entries, _ := os.ReadDir(catDir)
	seen := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() || !categoryImageExts[strings.ToLower(filepath.Ext(entry.Name()))] {
			continue
		}
		stem := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		key := toUpperSnakeCase(stem)
		if _, exists := builtin[key]; exists {
			LogWarn(game.LogComponentHTTP, "Categories: custom file %q conflicts with hardcoded key %s — skipped", entry.Name(), key)
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		info := CategoryInfo{
			Key: key, Name: stem, IsCustom: true,
			ImageURL: "/files/categories/" + url.PathEscape(entry.Name()),
		}
		sc := readCategorySidecar(catDir, key)
		if sc.Name != nil && *sc.Name != "" {
			info.Name = *sc.Name
		}
		if sc.Color != nil {
			info.Color = *sc.Color
		}
		out = append(out, info)
	}
	return out
}

func (h *HTTPServer) findCategory(key string) (CategoryInfo, bool) {
	if key == "" {
		return CategoryInfo{}, false
	}
	for _, c := range h.listCategories() {
		if c.Key == key {
			return c, true
		}
	}
	return CategoryInfo{}, false
}

// ResolveCategoryMeta returns the display name, image URL and accent colour of a
// category key — the single source for the palmarès, the history and the
// ambiance (contract §Catégories "Résolution"). Built-ins: table then sidecar
// override; custom: image + sidecar. Unknown or empty key ⇒ zero values.
func (h *HTTPServer) ResolveCategoryMeta(key string) (name, imageURL, color string) {
	if info, ok := h.findCategory(key); ok {
		return info.Name, info.ImageURL, info.Color
	}
	return "", "", ""
}

// categoryCounts returns, per category key, the number of questions + MEMOTION
// cards (questionCount) and reservoir answers (rafaleCount).
func (h *HTTPServer) categoryCounts() (questions, rafale map[string]int) {
	questions, rafale = map[string]int{}, map[string]int{}
	for _, q := range ReadQuestionMaps(h.questionsDir()) {
		if c, _ := q["CATEGORY"].(string); c != "" {
			questions[c]++
		}
		if cards, ok := q["MOTION_CARDS"].([]interface{}); ok {
			for _, ci := range cards {
				if card, ok := ci.(map[string]interface{}); ok {
					if c, _ := card["CATEGORY"].(string); c != "" {
						questions[c]++
					}
				}
			}
		}
	}
	rq, _ := h.engine.SnapshotRafaleReservoir()
	for _, r := range rq {
		if r.Category != "" {
			rafale[string(r.Category)]++
		}
	}
	return questions, rafale
}

func (h *HTTPServer) withCounts(info CategoryInfo) CategoryInfo {
	q, r := h.categoryCounts()
	info.QuestionCount, info.RafaleCount = q[info.Key], r[info.Key]
	return info
}

// categoriesLocked is the common guard of every category write (decision
// 2026-11-05): a game is in progress unless the phase is STOPPED, NEW_GAME or
// REVEALED. Evaluated FIRST, before any body/file validation.
func (h *HTTPServer) categoriesLocked() bool {
	switch h.engine.GetPhase() {
	case "", game.PhaseStopped, game.PhaseNewGame, game.PhaseRevealed:
		return false
	}
	return true
}

func writeCategoryJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeCategoryError(w http.ResponseWriter, status int, msg string) {
	writeCategoryJSON(w, status, map[string]string{"error": msg})
}

// rejectIfLocked answers 409 and returns true when a game is in progress.
func (h *HTTPServer) rejectIfLocked(w http.ResponseWriter) bool {
	if h.categoriesLocked() {
		writeCategoryError(w, http.StatusConflict, "game in progress")
		return true
	}
	return false
}

// categoryWriteMu serialises every category write handler (POST/PUT/icon/DELETE).
var categoryWriteMu sync.Mutex

// reservedWindowsNames are device names that cannot be used as file stems on Windows.
var reservedWindowsNames = regexp.MustCompile(`(?i)^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])$`)

// CategoryChange describes the write that triggered OnCategoriesChanged
// (contract websocket-actions.md §CATEGORIES_CHANGED).
type CategoryChange struct {
	Type string
	Key  string
}

const (
	CategoryChangeCreated = "created"
	CategoryChangeRenamed = "renamed"
	CategoryChangeColored = "colored"
	CategoryChangeIcon    = "icon"
	CategoryChangeDeleted = "deleted"
)

func (h *HTTPServer) categoriesChanged(change CategoryChange) {
	if h.OnCategoriesChanged != nil {
		h.OnCategoriesChanged(change)
	}
}

// CategoriesChangedPayload builds the CATEGORIES_CHANGED message body:
// the full resolved list (same as GET /api/categories), the change and a server
// timestamp (ms) used by clients as icon cache-buster.
func (h *HTTPServer) CategoriesChangedPayload(change CategoryChange) (json.RawMessage, error) {
	list := h.listCategories()
	q, rf := h.categoryCounts()
	for i := range list {
		list[i].QuestionCount, list[i].RafaleCount = q[list[i].Key], rf[list[i].Key]
	}
	return json.Marshal(map[string]interface{}{
		"CATEGORIES": list,
		"CHANGE":     map[string]string{"TYPE": change.Type, "KEY": change.Key},
		"TS":         time.Now().UnixMilli(),
	})
}

// ---- routing -------------------------------------------------------------

// handleAPICategories serves /api/categories: GET list, POST create.
func (h *HTTPServer) handleAPICategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetCategories(w, r)
	case http.MethodPost:
		categoryWriteMu.Lock()
		defer categoryWriteMu.Unlock()
		if h.rejectIfLocked(w) {
			return
		}
		h.handlePostCategory(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAPICategoryByKey serves /api/categories/{key} (PUT, DELETE) and
// /api/categories/{key}/icon (POST).
func (h *HTTPServer) handleAPICategoryByKey(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/categories/"), "/")
	key, sub, _ := strings.Cut(rest, "/")
	var allowed string
	switch {
	case sub == "":
		if r.Method != http.MethodPut && r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		allowed = "key"
	case sub == "icon":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		allowed = "icon"
	default:
		http.NotFound(w, r)
		return
	}
	categoryWriteMu.Lock()
	defer categoryWriteMu.Unlock()
	// 409 first, before any validation.
	if h.rejectIfLocked(w) {
		return
	}
	// The key is validated before any path join.
	if !categoryKeyRe.MatchString(key) {
		writeCategoryError(w, http.StatusBadRequest, "invalid category key")
		return
	}
	switch {
	case allowed == "icon":
		h.handleCategoryIcon(w, r, key)
	case r.Method == http.MethodPut:
		h.handlePutCategory(w, r, key)
	default:
		h.handleDeleteCategory(w, r, key)
	}
}

// ---- GET / POST ------------------------------------------------------------

func (h *HTTPServer) handleGetCategories(w http.ResponseWriter, r *http.Request) {
	list := h.listCategories()
	q, rf := h.categoryCounts()
	for i := range list {
		list[i].QuestionCount, list[i].RafaleCount = q[list[i].Key], rf[list[i].Key]
	}
	writeCategoryJSON(w, http.StatusOK, list)
}

func validateCategoryName(name string) string {
	if name == "" {
		return "name is required"
	}
	if len(name) > 50 {
		return "name is too long (max 50 chars)"
	}
	for _, ch := range name {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != ' ' && ch != '-' && ch != '_' {
			return "name contains invalid characters"
		}
	}
	return ""
}

// iconUpload is a validated icon, ready to be written.
type iconUpload struct {
	ext  string // normalised: ".png" ".jpg" ".webp"
	data []byte
}

// readIconUpload parses the multipart body (field "file") and validates it
// (contract "Validation d'icône"): body ≤ 2 Mio (+64 Kio of multipart overhead)
// ⇒ 413 beyond; extension in the allowlist; content sniffed by
// http.DetectContentType and consistent with the extension. On failure it has
// already answered and returns nil.
func (h *HTTPServer) readIconUpload(w http.ResponseWriter, r *http.Request) *iconUpload {
	r.Body = http.MaxBytesReader(w, r.Body, maxCategoryIconBytes+64<<10)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeCategoryError(w, http.StatusRequestEntityTooLarge, "file too large (max 2 MiB)")
		} else {
			writeCategoryError(w, http.StatusBadRequest, "invalid multipart form")
		}
		return nil
	}
	return h.iconFromForm(w, r)
}

func (h *HTTPServer) iconFromForm(w http.ResponseWriter, r *http.Request) *iconUpload {
	file, header, err := r.FormFile("file")
	if err != nil {
		writeCategoryError(w, http.StatusBadRequest, "file is required")
		return nil
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !categoryImageExts[ext] {
		writeCategoryError(w, http.StatusBadRequest, "invalid file type, allowed: png, jpg, jpeg, webp")
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(file, maxCategoryIconBytes+1))
	if err != nil {
		writeCategoryError(w, http.StatusBadRequest, "failed to read file")
		return nil
	}
	if len(data) > maxCategoryIconBytes {
		writeCategoryError(w, http.StatusRequestEntityTooLarge, "file too large (max 2 MiB)")
		return nil
	}
	sniff := data
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	ct := http.DetectContentType(sniff)
	want := map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp"}[ext]
	if ct != want {
		writeCategoryError(w, http.StatusBadRequest, "file content does not match its extension")
		return nil
	}
	if ext == ".jpeg" {
		ext = ".jpg"
	}
	return &iconUpload{ext: ext, data: data}
}

// storeIcon writes <KEY><ext> (tmp + rename) then purges any OTHER image of the
// key, so there are never two images for one key. The name never depends on
// the uploaded Filename.
func (h *HTTPServer) storeIcon(key string, ic *iconUpload) error {
	catDir := h.categoriesDir()
	if err := os.MkdirAll(catDir, 0755); err != nil {
		return err
	}
	final := key + ic.ext
	if err := writeFileAtomic(filepath.Join(catDir, final), ic.data); err != nil {
		return err
	}
	h.purgeCategoryImages(key, final)
	return nil
}

// purgeCategoryImages removes every image of key except keep ("" ⇒ all).
func (h *HTTPServer) purgeCategoryImages(key, keep string) {
	catDir := h.categoriesDir()
	entries, _ := os.ReadDir(catDir)
	for _, en := range entries {
		if en.IsDir() || en.Name() == keep || !categoryImageExts[strings.ToLower(filepath.Ext(en.Name()))] {
			continue
		}
		stem := strings.TrimSuffix(en.Name(), filepath.Ext(en.Name()))
		if toUpperSnakeCase(stem) == key {
			_ = os.Remove(filepath.Join(catDir, en.Name()))
		}
	}
}

// handlePostCategory creates a custom category: name + icon (+ optional colour).
func (h *HTTPServer) handlePostCategory(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCategoryIconBytes+64<<10)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeCategoryError(w, http.StatusRequestEntityTooLarge, "file too large (max 2 MiB)")
		} else {
			writeCategoryError(w, http.StatusBadRequest, "invalid multipart form")
		}
		return
	}
	name := r.FormValue("name")
	if msg := validateCategoryName(name); msg != "" {
		writeCategoryError(w, http.StatusBadRequest, msg)
		return
	}
	color := r.FormValue("color")
	if color != "" && !categoryColorRe.MatchString(color) {
		writeCategoryError(w, http.StatusBadRequest, "invalid color (expected #rrggbb)")
		return
	}
	key := toUpperSnakeCase(name)
	if !categoryKeyRe.MatchString(key) {
		writeCategoryError(w, http.StatusBadRequest, "invalid category key")
		return
	}
	if reservedWindowsNames.MatchString(key) {
		writeCategoryError(w, http.StatusBadRequest, "reserved name")
		return
	}
	if _, exists := h.findCategory(key); exists {
		writeCategoryError(w, http.StatusConflict, "key already exists")
		return
	}
	for _, c := range hardcodedCategories {
		if c.Key == key {
			writeCategoryError(w, http.StatusConflict, "key already exists")
			return
		}
	}
	ic := h.iconFromForm(w, r)
	if ic == nil {
		return
	}
	if err := h.storeIcon(key, ic); err != nil {
		LogError(game.LogComponentHTTP, "Categories POST: failed to save image for %s: %v", key, err)
		writeCategoryError(w, http.StatusInternalServerError, "failed to save image")
		return
	}
	sc := categorySidecar{Name: &name}
	if color != "" {
		sc.Color = &color
	}
	if err := writeCategorySidecar(h.categoriesDir(), key, sc); err != nil {
		LogError(game.LogComponentHTTP, "Categories POST: could not save sidecar for %s: %v", key, err)
		h.purgeCategoryImages(key, "")
		writeCategoryError(w, http.StatusInternalServerError, "failed to write category")
		return
	}
	LogInfo(game.LogComponentHTTP, "Categories POST: created category key=%s name=%q", key, name)
	h.categoriesChanged(CategoryChange{Type: CategoryChangeCreated, Key: key})
	info, _ := h.findCategory(key)
	writeCategoryJSON(w, http.StatusOK, h.withCounts(info))
}

// ---- PUT -------------------------------------------------------------------

func (h *HTTPServer) handlePutCategory(w http.ResponseWriter, r *http.Request, key string) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&raw); err != nil {
		writeCategoryError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	var name, color string
	_, hasName := raw["name"]
	_, hasColor := raw["color"]
	if !hasName && !hasColor {
		writeCategoryError(w, http.StatusBadRequest, "at least one of name, color is required")
		return
	}
	if hasName {
		if err := json.Unmarshal(raw["name"], &name); err != nil {
			writeCategoryError(w, http.StatusBadRequest, "invalid name")
			return
		}
		if msg := validateCategoryName(name); msg != "" {
			writeCategoryError(w, http.StatusBadRequest, msg)
			return
		}
	}
	if hasColor {
		if err := json.Unmarshal(raw["color"], &color); err != nil || (color != "" && !categoryColorRe.MatchString(color)) {
			writeCategoryError(w, http.StatusBadRequest, "invalid color (expected #rrggbb or \"\")")
			return
		}
	}
	if _, ok := h.findCategory(key); !ok {
		writeCategoryError(w, http.StatusNotFound, "unknown category")
		return
	}
	catDir := h.categoriesDir()
	if err := os.MkdirAll(catDir, 0755); err != nil {
		writeCategoryError(w, http.StatusInternalServerError, "failed to write category")
		return
	}
	sc := readCategorySidecar(catDir, key)
	if hasName {
		sc.Name = &name
	}
	if hasColor {
		sc.Color = &color
	}
	if err := writeCategorySidecar(catDir, key, sc); err != nil {
		LogError(game.LogComponentHTTP, "Categories PUT: sidecar %s: %v", key, err)
		writeCategoryError(w, http.StatusInternalServerError, "failed to write category")
		return
	}
	changeType := CategoryChangeColored
	if hasName {
		changeType = CategoryChangeRenamed
	}
	h.categoriesChanged(CategoryChange{Type: changeType, Key: key})
	info, _ := h.findCategory(key)
	writeCategoryJSON(w, http.StatusOK, h.withCounts(info))
}

// ---- icon ------------------------------------------------------------------

func (h *HTTPServer) handleCategoryIcon(w http.ResponseWriter, r *http.Request, key string) {
	info, ok := h.findCategory(key)
	if !ok {
		writeCategoryError(w, http.StatusNotFound, "unknown category")
		return
	}
	if !info.IsCustom {
		writeCategoryError(w, http.StatusBadRequest, "built-in categories keep their emoji")
		return
	}
	ic := h.readIconUpload(w, r)
	if ic == nil {
		return
	}
	if err := h.storeIcon(key, ic); err != nil {
		LogError(game.LogComponentHTTP, "Categories icon %s: %v", key, err)
		writeCategoryError(w, http.StatusInternalServerError, "failed to save image")
		return
	}
	h.categoriesChanged(CategoryChange{Type: CategoryChangeIcon, Key: key})
	info, _ = h.findCategory(key)
	writeCategoryJSON(w, http.StatusOK, h.withCounts(info))
}

// ---- DELETE ------------------------------------------------------------------

type deleteCategoryBody struct {
	Mode string `json:"mode"`
}

type categoryDeleteCounts struct {
	Reassigned           int `json:"reassigned"`
	Deleted              int `json:"deleted"`
	CardsReassigned      int `json:"cardsReassigned"`
	CardsDeleted         int `json:"cardsDeleted"`
	RafaleFiltersCleaned int `json:"rafaleFiltersCleaned"`
}

// removeKeyFromList removes key from a RAFALE_CATEGORIES-style list held in m[field];
// reports whether the list changed. A list that becomes empty stays empty (= all categories).
func removeKeyFromList(m map[string]interface{}, field, key string) bool {
	list, ok := m[field].([]interface{})
	if !ok {
		return false
	}
	out := make([]interface{}, 0, len(list))
	changed := false
	for _, v := range list {
		if s, _ := v.(string); s == key {
			changed = true
			continue
		}
		out = append(out, v)
	}
	if changed {
		m[field] = out
	}
	return changed
}

func (h *HTTPServer) handleDeleteCategory(w http.ResponseWriter, r *http.Request, key string) {
	_, ok := h.findCategory(key)
	if !ok {
		for _, c := range hardcodedCategories {
			if c.Key == key {
				ok = true
			}
		}
		if !ok {
			writeCategoryError(w, http.StatusNotFound, "unknown category")
			return
		}
	}
	for _, c := range hardcodedCategories {
		if c.Key == key {
			writeCategoryError(w, http.StatusBadRequest, "built-in categories cannot be deleted")
			return
		}
	}

	var body deleteCategoryBody
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body)
	}
	mode := body.Mode
	if mode != "reassign" && mode != "delete" {
		mode = ""
	}

	qc, rc := h.categoryCounts()
	ev, pts := h.engine.CountHistoryCategory(key)
	if qc[key]+rc[key]+ev > 0 && mode == "" {
		writeCategoryJSON(w, http.StatusConflict, map[string]interface{}{
			"error": "category in use", "questions": qc[key], "rafale": rc[key],
			"history": map[string]int{"events": ev, "points": pts},
		})
		return
	}

	// H. history first: the riskiest step, and the only one needing a backup.
	events, points, backup, herr := h.engine.MergeHistoryCategoryWithBackup(key)
	if herr != nil {
		LogError(game.LogComponentHTTP, "Categories DELETE %s: history merge failed, nothing else touched: %v", key, herr)
		writeCategoryError(w, http.StatusInternalServerError, "history merge failed")
		return
	}

	res := map[string]interface{}{"deleted": key, "mode": mode}
	var qcounts categoryDeleteCounts
	var rafaleReassigned, rafaleDeleted int
	hist := map[string]interface{}{"events": events, "points": points, "backup": backup}
	fail := func(step string, err error) {
		LogError(game.LogComponentHTTP, "Categories DELETE %s: step %s failed: %v", key, step, err)
		writeCategoryJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": "partial failure at step " + step, "partial": true,
			"questions": qcounts, "rafale": map[string]int{"reassigned": rafaleReassigned, "deleted": rafaleDeleted}, "history": hist,
		})
	}

	// Q. questions and MEMOTION cards.
	if mode != "" || qc[key] > 0 {
		var deletedIDs []string
		QuestionFilesMu.Lock()
		qdir := h.questionsDir()
		for _, q := range ReadQuestionMaps(qdir) {
			id, _ := q["ID"].(string)
			qpath, perr := SafeQuestionPath(qdir, id)
			if perr != nil {
				continue
			}
			isOwn := false
			if c, _ := q["CATEGORY"].(string); c == key {
				isOwn = true
			}
			if isOwn && mode == "delete" {
				if err := os.RemoveAll(qpath); err != nil {
					QuestionFilesMu.Unlock()
					fail("questions", err)
					return
				}
				deletedIDs = append(deletedIDs, id)
				qcounts.Deleted++
				continue
			}
			modified := false
			if isOwn { // reassign
				q["CATEGORY"] = ""
				qcounts.Reassigned++
				modified = true
			}
			if removeKeyFromList(q, "RAFALE_CATEGORIES", key) {
				qcounts.RafaleFiltersCleaned++
				modified = true
			}
			if cards, ok := q["MOTION_CARDS"].([]interface{}); ok {
				kept := make([]interface{}, 0, len(cards))
				for _, ci := range cards {
					card, ok := ci.(map[string]interface{})
					if !ok {
						kept = append(kept, ci)
						continue
					}
					if removeKeyFromList(card, "RAFALE_CATEGORIES", key) {
						qcounts.RafaleFiltersCleaned++
						modified = true
					}
					if c, _ := card["CATEGORY"].(string); c == key {
						modified = true
						if mode == "delete" {
							removeCardMedia(qpath, id, card)
							qcounts.CardsDeleted++
							continue // dropped from the host
						}
						card["CATEGORY"] = ""
						qcounts.CardsReassigned++
					}
					kept = append(kept, card)
				}
				q["MOTION_CARDS"] = kept
			}
			if modified {
				data, merr := json.MarshalIndent(q, "", "  ")
				if merr == nil {
					merr = writeFileAtomic(filepath.Join(qpath, "question.json"), data)
				}
				if merr != nil {
					QuestionFilesMu.Unlock()
					fail("questions", merr)
					return
				}
			}
		}
		QuestionFilesMu.Unlock()
		if len(deletedIDs) > 0 {
			h.engine.ForgetQuestionStatuses(deletedIDs)
		}
	}
	res["questions"] = qcounts

	// R. reservoir.
	var rerr error
	if mode == "delete" {
		rafaleDeleted, rerr = h.engine.DeleteRafaleQuestionsByCategory(key)
	} else {
		rafaleReassigned, rerr = h.engine.ReassignRafaleCategory(key, "")
	}
	if rerr != nil {
		fail("reservoir", rerr)
		return
	}
	res["rafale"] = map[string]int{"reassigned": rafaleReassigned, "deleted": rafaleDeleted}
	res["history"] = hist

	// F. files.
	h.purgeCategoryImages(key, "")
	_ = os.Remove(filepath.Join(h.categoriesDir(), key+".json"))

	// N. notification.
	LogInfo(game.LogComponentHTTP, "Categories DELETE: %s mode=%q questions=%+v rafale=%d/%d history=%d events", key, mode, qcounts, rafaleReassigned, rafaleDeleted, events)
	// QUESTIONS is rebroadcast by OnCategoriesChanged (after CATEGORIES_CHANGED): no separate OnQuestionUpload here.
	h.categoriesChanged(CategoryChange{Type: CategoryChangeDeleted, Key: key})
	writeCategoryJSON(w, http.StatusOK, res)
}

// removeCardMedia deletes the media files of a dropped MEMOTION card that live in
// its host question directory. Only a base name under /question/<hostID>/ is ever
// removed, never an arbitrary path.
func removeCardMedia(hostPath, hostID string, card map[string]interface{}) {
	prefix := "/question/" + hostID + "/"
	for _, f := range []string{"RECTO_IMAGE", "QUESTION_IMAGE", "ANSWER_IMAGE"} {
		p, _ := card[f].(string)
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		name := filepath.Base(strings.TrimPrefix(p, prefix))
		if name == "" || name == "." || name == "question.json" || strings.ContainsAny(name, `/\`) {
			continue
		}
		_ = os.Remove(filepath.Join(hostPath, name))
	}
}
