package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// This file implements the generalised reservoir store (EntryPool) — first
// written for the RAFALE reservoir (contracts/rafale.md §2.4/§3, milestone
// #16, issue #197), generalised in v13.1.0 (contracts/picto-pool.md §1) so
// the same engine serves two instances: `rafale` (files/rafale/reservoir.json,
// config/rafale_used.json, IDs "r-NNN") and `picto` (files/picto/…,
// config/picto_used.json, IDs "p-NNN"). Two independent, typed persistence
// pairs per instance on the SaveStatuses/LoadStatuses SHAPE (single typed
// file, Set*Path/Save/Load trio) — but NOT on SaveStatuses' own non-atomic
// os.WriteFile, see atomicWriteFile's doc comment for why (#107).
//
//   - Reservoir (PoolEntry list) — files/<name>/reservoir.json, shape
//     {"QUESTIONS": [...]} for both instances. Edited by HTTP endpoints.
//   - "Already used" flag — config/<name>_used.json. Written by the game
//     engine at draw time and reset at NEW_GAME (InitGame, engine.go).
//
// Deliberately two files, not one: editing the reservoir must never rewrite
// game-play state, and playing must never rewrite the reservoir (contract
// §3.2).
//
// Locking: an EntryPool owns no mutex of its own — it shares the Engine's
// (pool.mu == &Engine.mu). Methods named *Unsafe require the caller to hold
// it; the others lock it themselves, exactly like the Engine methods they
// back.

// ErrRafaleQuestionNotFound is returned by DeleteRafaleQuestion when the
// given ID has no entry in the reservoir.
var ErrRafaleQuestionNotFound = errors.New("rafale_question_not_found")

// ErrPoolEntryNotFound is the generic name of ErrRafaleQuestionNotFound
// (same value: errors.Is works with either).
var ErrPoolEntryNotFound = ErrRafaleQuestionNotFound

// ErrPoolEmpty is the generic name of ErrRafalePoolEmpty (same value).
var ErrPoolEmpty = ErrRafalePoolEmpty

// poolFile mirrors the on-disk shape of a reservoir file —
// contracts/rafale.md §3.1 (identical for both instances).
type poolFile struct {
	Questions []PoolEntry `json:"QUESTIONS"`
}

// poolUsedFile mirrors the on-disk shape of a "used" file —
// contracts/rafale.md §3.2.
type poolUsedFile struct {
	Used map[string]bool `json:"USED"`
}

// EntryPool is one reservoir instance: its entries, its "already used"
// flags and the two persistence paths.
type EntryPool struct {
	mu       *sync.RWMutex // the Engine's mutex (see file header)
	label    string        // "Rafale" / "Picto" — log messages only
	idPrefix string        // "r-" / "p-"
	entries  map[string]*PoolEntry
	path     string // files/<name>/reservoir.json ("" = no persistence)
	used     map[string]bool
	usedPath string // config/<name>_used.json ("" = no persistence)
}

// newEntryPool builds an empty pool sharing mu.
func newEntryPool(mu *sync.RWMutex, label, idPrefix string) *EntryPool {
	return &EntryPool{
		mu:       mu,
		label:    label,
		idPrefix: idPrefix,
		entries:  make(map[string]*PoolEntry),
		used:     make(map[string]bool),
	}
}

// ---- Reservoir persistence -------------------------------------------------

// SetPath sets the path for the reservoir persistence file.
func (p *EntryPool) SetPath(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.path = path
	log.Printf("[Engine] %s reservoir path set to: %s", p.label, path)
}

// Save persists the reservoir to disk — a single file, no per-entry
// directory (contracts/rafale.md §2.4: reservoir entries are text-only, no
// media).
func (p *EntryPool) Save() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.path == "" {
		return nil
	}

	dir := filepath.Dir(p.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[Engine] Failed to create %s reservoir directory: %v", strings.ToLower(p.label), err)
		return err
	}

	questions := make([]PoolEntry, 0, len(p.entries))
	for _, q := range p.entries {
		questions = append(questions, *q)
	}
	sort.Slice(questions, func(i, j int) bool { return questions[i].ID < questions[j].ID })

	data, err := json.MarshalIndent(poolFile{Questions: questions}, "", "  ")
	if err != nil {
		log.Printf("[Engine] Failed to marshal %s reservoir: %v", strings.ToLower(p.label), err)
		return err
	}

	if err := atomicWriteFile(dir, p.path, data); err != nil {
		log.Printf("[Engine] Failed to save %s reservoir: %v", strings.ToLower(p.label), err)
		return err
	}

	log.Printf("[Engine] %s reservoir saved: %d questions to %s", p.label, len(questions), p.path)
	return nil
}

// Load loads the reservoir from disk. A missing file is not an error — a
// fresh install simply starts with an empty reservoir.
func (p *EntryPool) Load() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.path == "" {
		return nil
	}

	lower := strings.ToLower(p.label)
	data, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Engine] No %s reservoir file found, starting fresh", lower)
			return nil
		}
		log.Printf("[Engine] Failed to read %s reservoir: %v", lower, err)
		return err
	}

	var file poolFile
	if err := json.Unmarshal(data, &file); err != nil {
		log.Printf("[Engine] Failed to parse %s reservoir: %v", lower, err)
		return err
	}

	questions := make(map[string]*PoolEntry, len(file.Questions))
	for i := range file.Questions {
		q := file.Questions[i] // fresh copy per iteration — safe to take its address
		questions[q.ID] = &q
	}
	p.entries = questions
	log.Printf("[Engine] %s reservoir loaded: %d questions from %s", p.label, len(questions), p.path)
	return nil
}

// Clear empties the in-memory reservoir (used by the selective reset
// endpoint — contracts/rafale.md §10 — after the on-disk file has already
// been removed by the caller).
func (p *EntryPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = make(map[string]*PoolEntry)
	log.Printf("[Engine] %s reservoir cleared", p.label)
}

// ---- "Already used" flag persistence --------------------------------------

// SetUsedPath sets the path for the "already used" flag persistence file.
func (p *EntryPool) SetUsedPath(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usedPath = path
	log.Printf("[Engine] %s used-flags path set to: %s", p.label, path)
}

// SaveUsed persists the "already used" flag map to disk.
func (p *EntryPool) SaveUsed() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.usedPath == "" {
		return nil
	}

	lower := strings.ToLower(p.label)
	dir := filepath.Dir(p.usedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[Engine] Failed to create %s used-flags directory: %v", lower, err)
		return err
	}

	data, err := json.MarshalIndent(poolUsedFile{Used: p.used}, "", "  ")
	if err != nil {
		log.Printf("[Engine] Failed to marshal %s used-flags: %v", lower, err)
		return err
	}

	if err := atomicWriteFile(dir, p.usedPath, data); err != nil {
		log.Printf("[Engine] Failed to save %s used-flags: %v", lower, err)
		return err
	}

	log.Printf("[Engine] %s used-flags saved: %d entries to %s", p.label, len(p.used), p.usedPath)
	return nil
}

// LoadUsed loads the "already used" flag map from disk. A missing file is
// not an error — a fresh install (or a just-reset game) starts with nothing
// used.
func (p *EntryPool) LoadUsed() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.usedPath == "" {
		return nil
	}

	lower := strings.ToLower(p.label)
	data, err := os.ReadFile(p.usedPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Engine] No %s used-flags file found, starting fresh", lower)
			return nil
		}
		log.Printf("[Engine] Failed to read %s used-flags: %v", lower, err)
		return err
	}

	var file poolUsedFile
	if err := json.Unmarshal(data, &file); err != nil {
		log.Printf("[Engine] Failed to parse %s used-flags: %v", lower, err)
		return err
	}

	if file.Used == nil {
		file.Used = make(map[string]bool)
	}
	p.used = file.Used
	log.Printf("[Engine] %s used-flags loaded: %d entries from %s", p.label, len(p.used), p.usedPath)
	return nil
}

// ClearUsed empties the in-memory "already used" flag map (used by InitGame
// — NEW_GAME reset — and by the selective reset endpoint).
func (p *EntryPool) ClearUsed() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.used = make(map[string]bool)
	log.Printf("[Engine] %s used-flags cleared", p.label)
}

// ---- Reservoir CRUD (editor, #197) -----------------------------------------

// Snapshot returns a defensive copy of the full reservoir (sorted by ID) and
// the current "already used" flags, for the HTTP layer to filter/merge
// (USED is derived at read time, never stored in the reservoir itself —
// contracts/rafale.md §3.2/§9).
func (p *EntryPool) Snapshot() (questions []PoolEntry, used map[string]bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	questions = make([]PoolEntry, 0, len(p.entries))
	for _, q := range p.entries {
		questions = append(questions, *q)
	}
	sort.Slice(questions, func(i, j int) bool { return questions[i].ID < questions[j].ID })

	used = make(map[string]bool, len(p.used))
	for id, u := range p.used {
		used[id] = u
	}
	return questions, used
}

// Get returns a copy of one entry by ID.
func (p *EntryPool) Get(id string) (PoolEntry, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	q, ok := p.entries[id]
	if !ok {
		return PoolEntry{}, false
	}
	return *q, true
}

// Upsert creates (q.ID == "") or updates (q.ID != "") one entry, persists
// the reservoir synchronously (so the HTTP caller's response only confirms
// success once the write actually landed) and returns the stored copy (with
// its assigned ID on create).
//
// Field validation is the HTTP handler's responsibility (contracts/rafale.md
// §9 ties the known-CATEGORY check to the custom-categories directory on
// disk, which only the server package resolves); this method trusts its
// caller, matching the engine's usual "the caller decides policy" split.
func (p *EntryPool) Upsert(q PoolEntry) (PoolEntry, error) {
	p.mu.Lock()

	if q.ID == "" {
		q.ID = p.nextIDUnsafe()
	}
	stored := q
	p.entries[q.ID] = &stored

	p.mu.Unlock()

	if err := p.Save(); err != nil {
		return PoolEntry{}, err
	}
	return q, nil
}

// Append adds a batch of entries in a single locked operation, followed by
// exactly one Save() call — contract rafale-ai-generation.md §8, feature
// #203. Any ID present on an input entry is ignored: the server always
// allocates fresh sequential identifiers itself, so a caller (the generation
// job) can never target, and so never overwrite, an existing entry. Returns
// the stored copies, in the same order as qs, with their assigned IDs. A
// nil/empty qs is a no-op (no lock taken, no save).
//
// ⚠️ Save() takes RLock — it CANNOT be called while holding the write lock
// acquired here (sync.RWMutex is not reentrant). The order is therefore
// strictly Lock → allocate+insert the WHOLE batch → Unlock → Save(), never
// interleaved with a save per entry: one lock, one save, whatever the batch
// size (a loop over Upsert would rewrite the ENTIRE file M times, O(N×M),
// with M windows during which an interrupted process leaves a partial batch).
func (p *EntryPool) Append(qs []PoolEntry) ([]PoolEntry, error) {
	if len(qs) == 0 {
		return nil, nil
	}

	p.mu.Lock()
	stored := make([]PoolEntry, len(qs))
	for i, q := range qs {
		q.ID = p.nextIDUnsafe()
		cp := q
		p.entries[q.ID] = &cp
		stored[i] = q
	}
	p.mu.Unlock()

	if err := p.Save(); err != nil {
		return nil, err
	}
	return stored, nil
}

// nextIDUnsafe returns the next free "<prefix>NNN" identifier, scanning
// existing IDs of the form "<prefix><digits>" for the highest numeric suffix
// (an ID that doesn't match this shape — e.g. hand-picked via the API's
// optional ID field — is simply ignored by the scan, never crashes it).
// Caller must hold the pool's mutex (write lock).
func (p *EntryPool) nextIDUnsafe() string {
	highest := 0
	for id := range p.entries {
		suffix := strings.TrimPrefix(id, p.idPrefix)
		if suffix == id {
			continue // no prefix
		}
		if n, err := strconv.Atoi(suffix); err == nil && n > highest {
			highest = n
		}
	}
	return fmt.Sprintf("%s%03d", p.idPrefix, highest+1)
}

// Delete removes one entry by ID and persists the reservoir synchronously.
// Returns ErrRafaleQuestionNotFound (= ErrPoolEntryNotFound) if the ID
// doesn't exist — the HTTP handler maps that to 404 (contract §9).
func (p *EntryPool) Delete(id string) error {
	p.mu.Lock()

	if _, ok := p.entries[id]; !ok {
		p.mu.Unlock()
		return ErrRafaleQuestionNotFound
	}
	delete(p.entries, id)

	p.mu.Unlock()

	return p.Save()
}

// ResetAllUsed clears the ENTIRE "already used" flag map (every entry
// becomes available again) and persists synchronously — contracts/rafale.md
// §9, feature #197. Returns the number of entries that were cleared.
//
// Distinct from ClearUsed in two ways: (1) it persists the change itself
// (ClearUsed only mutates memory — its callers, InitGame's NEW_GAME reset
// and the destructive reset handler, each persist differently); (2) it
// reports how many entries were actually cleared, for the HTTP response.
func (p *EntryPool) ResetAllUsed() (int, error) {
	p.mu.Lock()
	n := len(p.used)
	p.used = make(map[string]bool)
	p.mu.Unlock()

	if err := p.SaveUsed(); err != nil {
		return 0, err
	}
	log.Printf("[Engine] %s used-flags reset: %d entries cleared", p.label, n)
	return n, nil
}

// MarkAvailable removes one entry from the "already used" flag (persisted
// synchronously, same pattern as Delete) — contracts/rafale.md §9, feature
// #197: a manual per-entry "make available again" action.
//
// Returns ErrRafaleQuestionNotFound if id has no entry in the reservoir —
// same contract as Delete. Silently no-ops (still succeeds) if the entry
// exists but was not marked used.
func (p *EntryPool) MarkAvailable(id string) error {
	p.mu.Lock()

	if _, ok := p.entries[id]; !ok {
		p.mu.Unlock()
		return ErrRafaleQuestionNotFound
	}
	delete(p.used, id)

	p.mu.Unlock()

	return p.SaveUsed()
}

// ---- Filtering, counting, drawing -----------------------------------------

// matches reports whether q's (CATEGORY, DIFFICULTY) is a member of the
// requested sets — the SINGLE membership predicate shared by every pool
// filtering site (contracts/rafale.md §7.1, #216). Passing a one-element
// categories/difficulties pair reduces to an exact match (a single active
// couple's own draw).
//
// Ensemble **vide ⇒ pool vide, jamais wildcard** (post-review guard,
// 2026-08-29, carried forward unchanged by #216): an unconfigured manche (no
// category, or no difficulty, selected) must match nothing, even though an
// entry with an equally unset CATEGORY could otherwise satisfy an
// empty-slice "contains" check.
func (q *PoolEntry) matches(categories []string, difficulties []int) bool {
	if len(categories) == 0 || len(difficulties) == 0 {
		return false
	}
	catOK := false
	for _, c := range categories {
		if string(q.Category) == c {
			catOK = true
			break
		}
	}
	if !catOK {
		return false
	}
	for _, d := range difficulties {
		if q.Difficulty == d {
			return true
		}
	}
	return false
}

// availableUnsafe returns the AVAILABLE (not-yet-used) entries matching
// categories ∈ set ∩ difficulties ∈ set — contracts/rafale.md §7.1. Caller
// must hold the pool's mutex (read or write lock). Every pool site shares
// matches (#107's original bug was a second, independently-drifted copy of
// this filter at a forgotten call site).
func (p *EntryPool) availableUnsafe(categories []string, difficulties []int) []*PoolEntry {
	var avail []*PoolEntry
	for _, q := range p.entries {
		if !q.matches(categories, difficulties) {
			continue
		}
		if p.used[q.ID] {
			continue
		}
		avail = append(avail, q)
	}
	return avail
}

// Count returns (available, used, total) entries matching categories ∈ set ∩
// difficulties ∈ set (#216) — contracts/rafale.md §7.5, the pre-round alert.
// Shares matches with availableUnsafe rather than availableUnsafe itself
// (this additionally needs the USED branch) — same single predicate.
func (p *EntryPool) Count(categories []string, difficulties []int) (available, used, total int) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, q := range p.entries {
		if !q.matches(categories, difficulties) {
			continue
		}
		total++
		if p.used[q.ID] {
			used++
		} else {
			available++
		}
	}
	return available, used, total
}

// drawUnsafe draws one entry uniformly at random from the pool (categories ∈
// set ∩ difficulties ∈ set ∩ not-used — contracts/rafale.md §7.1) and marks
// it used in memory. Returns ErrRafalePoolEmpty when the pool is empty.
// Caller must hold the pool's mutex (write lock) and is responsible for
// persisting the used flags (SaveUsed) once its own locked section ends.
func (p *EntryPool) drawUnsafe(categories []string, difficulties []int) (*PoolEntry, error) {
	avail := p.availableUnsafe(categories, difficulties)
	if len(avail) == 0 {
		return nil, ErrRafalePoolEmpty
	}

	picked := avail[rand.Intn(len(avail))]
	drawn := *picked // copy: the caller must never hold a pointer into p.entries
	p.used[drawn.ID] = true

	return &drawn, nil
}

// atomicWriteFile writes data to path via a uniquely-named temp file in dir
// followed by an atomic rename — the same pattern already established by
// SaveBumpers/SaveTeams (#113 B4/#120 B2) for exactly this reason: both
// SaveRafale and SaveRafaleUsed are fired from a background goroutine
// (safeGo) on every RAFALE question drawn (contract §7 — "marquage immédiat
// + persistée"), so saves can legitimately overlap with each other and with
// a concurrent Load*. Plain os.WriteFile truncates the destination in
// place, so a reader (or a second overlapping save) can observe an empty or
// partially-written file mid-write — exactly the "unexpected end of JSON
// input" failure this fixes. os.CreateTemp gives each call its own file (no
// two saves collide on the same temp path), and os.Rename is atomic on the
// same filesystem, so readers only ever see a fully-formed old or new file.
func atomicWriteFile(dir, path string, data []byte) error {
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}
