package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// This file implements the RAFALE reservoir store — contracts/rafale.md
// §2.4/§3, milestone #16, issue #197. Two independent, typed persistence
// pairs on the SaveStatuses/LoadStatuses SHAPE (single typed file, Set*Path/
// Save/Load trio) — but NOT on SaveStatuses' own non-atomic os.WriteFile,
// see atomicWriteFile's doc comment for why (#107, Batch 2 fix: both saves
// here are fired from a background goroutine on every question draw, unlike
// question statuses, which only ever save from a single synchronous
// request path — SaveStatuses' plain os.WriteFile has never needed to
// tolerate an overlapping save of itself).
//
//   - Reservoir (RafaleQuestion list) — data/files/rafale/reservoir.json.
//     Edited by the /api/rafale/questions* HTTP endpoints.
//   - "Already used" flag — data/config/rafale_used.json. Written by the
//     game engine at draw time (DrawRafaleQuestion, engine.go) and reset at
//     NEW_GAME (InitGame, engine.go).
//
// Deliberately two files, not one: editing the reservoir must never rewrite
// game-play state, and playing must never rewrite the reservoir (contract
// §3.2).

// ErrRafaleQuestionNotFound is returned by DeleteRafaleQuestion when the
// given ID has no entry in the reservoir.
var ErrRafaleQuestionNotFound = errors.New("rafale_question_not_found")

// rafaleReservoirFile mirrors the on-disk shape of
// data/files/rafale/reservoir.json — contracts/rafale.md §3.1.
type rafaleReservoirFile struct {
	Questions []RafaleQuestion `json:"QUESTIONS"`
}

// rafaleUsedFile mirrors the on-disk shape of data/config/rafale_used.json —
// contracts/rafale.md §3.2.
type rafaleUsedFile struct {
	Used map[string]bool `json:"USED"`
}

// ---- Reservoir persistence -------------------------------------------------

// SetRafalePath sets the path for the RAFALE reservoir persistence file.
func (e *Engine) SetRafalePath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rafalePath = path
	log.Printf("[Engine] Rafale reservoir path set to: %s", path)
}

// SaveRafale persists the RAFALE reservoir to disk — a single file, no
// per-question directory (contracts/rafale.md §2.4: reservoir questions are
// text-only, no media, so the Quiz question's "one directory per question"
// pattern would add nothing but dead weight here).
func (e *Engine) SaveRafale() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.rafalePath == "" {
		return nil
	}

	dir := filepath.Dir(e.rafalePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[Engine] Failed to create rafale reservoir directory: %v", err)
		return err
	}

	questions := make([]RafaleQuestion, 0, len(e.rafaleQuestions))
	for _, q := range e.rafaleQuestions {
		questions = append(questions, *q)
	}
	sort.Slice(questions, func(i, j int) bool { return questions[i].ID < questions[j].ID })

	data, err := json.MarshalIndent(rafaleReservoirFile{Questions: questions}, "", "  ")
	if err != nil {
		log.Printf("[Engine] Failed to marshal rafale reservoir: %v", err)
		return err
	}

	if err := atomicWriteFile(dir, e.rafalePath, data); err != nil {
		log.Printf("[Engine] Failed to save rafale reservoir: %v", err)
		return err
	}

	log.Printf("[Engine] Rafale reservoir saved: %d questions to %s", len(questions), e.rafalePath)
	return nil
}

// LoadRafale loads the RAFALE reservoir from disk. A missing file is not an
// error — a fresh install simply starts with an empty reservoir, same as
// LoadStatuses.
func (e *Engine) LoadRafale() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.rafalePath == "" {
		return nil
	}

	data, err := os.ReadFile(e.rafalePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Engine] No rafale reservoir file found, starting fresh")
			return nil
		}
		log.Printf("[Engine] Failed to read rafale reservoir: %v", err)
		return err
	}

	var file rafaleReservoirFile
	if err := json.Unmarshal(data, &file); err != nil {
		log.Printf("[Engine] Failed to parse rafale reservoir: %v", err)
		return err
	}

	questions := make(map[string]*RafaleQuestion, len(file.Questions))
	for i := range file.Questions {
		q := file.Questions[i] // fresh copy per iteration — safe to take its address
		questions[q.ID] = &q
	}
	e.rafaleQuestions = questions
	log.Printf("[Engine] Rafale reservoir loaded: %d questions from %s", len(questions), e.rafalePath)
	return nil
}

// ClearRafaleReservoir empties the in-memory reservoir (used by the
// selective reset endpoint — contracts/rafale.md §10 — after the on-disk
// file has already been removed by the caller).
func (e *Engine) ClearRafaleReservoir() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rafaleQuestions = make(map[string]*RafaleQuestion)
	log.Printf("[Engine] Rafale reservoir cleared")
}

// ---- "Already used" flag persistence --------------------------------------

// SetRafaleUsedPath sets the path for the RAFALE "already used" flag
// persistence file.
func (e *Engine) SetRafaleUsedPath(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rafaleUsedPath = path
	log.Printf("[Engine] Rafale used-flags path set to: %s", path)
}

// SaveRafaleUsed persists the "already used" flag map to disk.
func (e *Engine) SaveRafaleUsed() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.rafaleUsedPath == "" {
		return nil
	}

	dir := filepath.Dir(e.rafaleUsedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[Engine] Failed to create rafale used-flags directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(rafaleUsedFile{Used: e.rafaleUsed}, "", "  ")
	if err != nil {
		log.Printf("[Engine] Failed to marshal rafale used-flags: %v", err)
		return err
	}

	if err := atomicWriteFile(dir, e.rafaleUsedPath, data); err != nil {
		log.Printf("[Engine] Failed to save rafale used-flags: %v", err)
		return err
	}

	log.Printf("[Engine] Rafale used-flags saved: %d entries to %s", len(e.rafaleUsed), e.rafaleUsedPath)
	return nil
}

// LoadRafaleUsed loads the "already used" flag map from disk. A missing
// file is not an error — a fresh install (or a just-reset game) starts with
// nothing used.
func (e *Engine) LoadRafaleUsed() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.rafaleUsedPath == "" {
		return nil
	}

	data, err := os.ReadFile(e.rafaleUsedPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Engine] No rafale used-flags file found, starting fresh")
			return nil
		}
		log.Printf("[Engine] Failed to read rafale used-flags: %v", err)
		return err
	}

	var file rafaleUsedFile
	if err := json.Unmarshal(data, &file); err != nil {
		log.Printf("[Engine] Failed to parse rafale used-flags: %v", err)
		return err
	}

	if file.Used == nil {
		file.Used = make(map[string]bool)
	}
	e.rafaleUsed = file.Used
	log.Printf("[Engine] Rafale used-flags loaded: %d entries from %s", len(e.rafaleUsed), e.rafaleUsedPath)
	return nil
}

// ClearRafaleUsed empties the in-memory "already used" flag map (used by
// InitGame — NEW_GAME reset — and by the selective reset endpoint).
func (e *Engine) ClearRafaleUsed() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rafaleUsed = make(map[string]bool)
	log.Printf("[Engine] Rafale used-flags cleared")
}

// ---- Reservoir CRUD (editor, #197) -----------------------------------------

// SnapshotRafaleReservoir returns a defensive copy of the full reservoir
// (sorted by ID) and the current "already used" flags, for the HTTP layer
// to filter/merge (GET /api/rafale/questions — contracts/rafale.md §9,
// where USED is derived at read time, never stored in the reservoir itself
// — §3.2/§9).
func (e *Engine) SnapshotRafaleReservoir() (questions []RafaleQuestion, used map[string]bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	questions = make([]RafaleQuestion, 0, len(e.rafaleQuestions))
	for _, q := range e.rafaleQuestions {
		questions = append(questions, *q)
	}
	sort.Slice(questions, func(i, j int) bool { return questions[i].ID < questions[j].ID })

	used = make(map[string]bool, len(e.rafaleUsed))
	for id, u := range e.rafaleUsed {
		used[id] = u
	}
	return questions, used
}

// GetRafaleQuestion returns a copy of one reservoir question by ID.
func (e *Engine) GetRafaleQuestion(id string) (RafaleQuestion, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	q, ok := e.rafaleQuestions[id]
	if !ok {
		return RafaleQuestion{}, false
	}
	return *q, true
}

// UpsertRafaleQuestion creates (q.ID == "") or updates (q.ID != "") one
// reservoir question, persists the reservoir synchronously (so the HTTP
// caller's response only confirms success once the write actually landed —
// unlike the fire-and-forget safeGo saves used for live game-play state),
// and returns the stored copy (with its assigned ID on create).
//
// Field validation (non-empty QUESTION/ANSWER, DIFFICULTY in 1..3, known
// CATEGORY) is the HTTP handler's responsibility — contracts/rafale.md §9
// ties the known-CATEGORY check to the custom-categories directory on disk,
// which only the server package resolves (h.dataDir); this method trusts
// its caller to have already validated when q.ID is empty… but also runs
// happily on programmatic input (tests) without re-validating, matching the
// engine's usual "the caller decides policy" split.
func (e *Engine) UpsertRafaleQuestion(q RafaleQuestion) (RafaleQuestion, error) {
	e.mu.Lock()

	if q.ID == "" {
		q.ID = e.nextRafaleIDUnsafe()
	}
	stored := q
	e.rafaleQuestions[q.ID] = &stored

	e.mu.Unlock()

	if err := e.SaveRafale(); err != nil {
		return RafaleQuestion{}, err
	}
	return q, nil
}

// AppendRafaleQuestions adds a batch of reservoir questions in a single
// locked operation, followed by exactly one SaveRafale() call — contract
// rafale-ai-generation.md §8, feature #203 (AI generation of the RAFALE
// reservoir). Any ID present on an input question is ignored: the server
// always allocates fresh sequential "r-NNN" identifiers itself, the same
// guarantee UpsertRafaleQuestion(q.ID=="") already gives one question at a
// time — a caller (the generation job) can therefore never target, and so
// never overwrite, an existing reservoir entry. Returns the stored copies,
// in the same order as qs, with their assigned IDs. A nil/empty qs is a
// no-op (no lock taken, no save).
//
// ⚠️ SaveRafale() takes RLock — it CANNOT be called while holding the write
// lock acquired here (sync.RWMutex is not reentrant). The order is
// therefore strictly Lock → allocate+insert the WHOLE batch → Unlock →
// SaveRafale(), never interleaved with a save per question. This is exactly
// why this method exists instead of a loop calling UpsertRafaleQuestion: a
// loop would call SaveRafale() once per question (UpsertRafaleQuestion's own
// contract), and SaveRafale() rewrites the ENTIRE reservoir file each time —
// O(N×M) for a batch of M into a reservoir of N, and M separate windows
// during which an interrupted process would leave the reservoir in a
// partial, unintended state. One lock, one save, whatever the batch size.
func (e *Engine) AppendRafaleQuestions(qs []RafaleQuestion) ([]RafaleQuestion, error) {
	if len(qs) == 0 {
		return nil, nil
	}

	e.mu.Lock()
	stored := make([]RafaleQuestion, len(qs))
	for i, q := range qs {
		q.ID = e.nextRafaleIDUnsafe()
		cp := q
		e.rafaleQuestions[q.ID] = &cp
		stored[i] = q
	}
	e.mu.Unlock()

	if err := e.SaveRafale(); err != nil {
		return nil, err
	}
	return stored, nil
}

// nextRafaleIDUnsafe returns the next free "r-NNN" identifier, scanning
// existing reservoir IDs of the form "r-<digits>" for the highest numeric
// suffix (an ID that doesn't match this shape — e.g. hand-picked via the
// API's optional ID field — is simply ignored by the scan, never crashes
// it). Caller must hold e.mu (write lock — called only from
// UpsertRafaleQuestion and AppendRafaleQuestions).
func (e *Engine) nextRafaleIDUnsafe() string {
	highest := 0
	for id := range e.rafaleQuestions {
		suffix := strings.TrimPrefix(id, "r-")
		if suffix == id {
			continue // no "r-" prefix
		}
		if n, err := strconv.Atoi(suffix); err == nil && n > highest {
			highest = n
		}
	}
	return fmt.Sprintf("r-%03d", highest+1)
}

// DeleteRafaleQuestion removes one reservoir question by ID and persists
// the reservoir synchronously. Returns ErrRafaleQuestionNotFound if the ID
// doesn't exist — the HTTP handler maps that to 404 (contract §9).
func (e *Engine) DeleteRafaleQuestion(id string) error {
	e.mu.Lock()

	if _, ok := e.rafaleQuestions[id]; !ok {
		e.mu.Unlock()
		return ErrRafaleQuestionNotFound
	}
	delete(e.rafaleQuestions, id)

	e.mu.Unlock()

	return e.SaveRafale()
}

// ResetAllRafaleUsed clears the ENTIRE "already used" flag map (every
// reservoir question becomes available again) and persists synchronously —
// contracts/rafale.md §9, feature #197. Returns the number of entries that
// were cleared.
//
// Distinct from ClearRafaleUsed above in two ways: (1) it persists the
// change itself (ClearRafaleUsed only mutates memory — its two existing
// callers, InitGame's NEW_GAME reset and the destructive
// /reset-select?rafale=true handler, each persist differently: a
// fire-and-forget safeGo save for the former, a direct file removal for the
// latter), matching the synchronous-persistence convention already
// established by UpsertRafaleQuestion/DeleteRafaleQuestion/
// MarkRafaleQuestionAvailable for editor-triggered HTTP actions; (2) it
// reports how many entries were actually cleared, for the HTTP response.
func (e *Engine) ResetAllRafaleUsed() (int, error) {
	e.mu.Lock()
	n := len(e.rafaleUsed)
	e.rafaleUsed = make(map[string]bool)
	e.mu.Unlock()

	if err := e.SaveRafaleUsed(); err != nil {
		return 0, err
	}
	log.Printf("[Engine] Rafale used-flags reset: %d entries cleared", n)
	return n, nil
}

// MarkRafaleQuestionAvailable removes one question from the "already used"
// flag (persisted synchronously, same pattern as DeleteRafaleQuestion) —
// contracts/rafale.md §9, feature #197: a manual per-question "make
// available again" action, distinct from the automatic NEW_GAME-driven
// ClearRafaleUsed reset below and from the destructive
// /reset-select?rafale=true flow (which also wipes the reservoir itself).
//
// Returns ErrRafaleQuestionNotFound if id has no entry in the reservoir —
// same contract as DeleteRafaleQuestion, so the HTTP handler maps it to 404
// identically. Silently no-ops (still succeeds) if the question exists but
// was not marked used — "make available" on an already-available question
// is not an error.
func (e *Engine) MarkRafaleQuestionAvailable(id string) error {
	e.mu.Lock()

	if _, ok := e.rafaleQuestions[id]; !ok {
		e.mu.Unlock()
		return ErrRafaleQuestionNotFound
	}
	delete(e.rafaleUsed, id)

	e.mu.Unlock()

	return e.SaveRafaleUsed()
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
