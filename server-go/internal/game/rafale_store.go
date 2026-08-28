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
// pairs on the SaveStatuses/LoadStatuses pattern:
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

	if err := os.WriteFile(e.rafalePath, data, 0644); err != nil {
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

	if err := os.WriteFile(e.rafaleUsedPath, data, 0644); err != nil {
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

// nextRafaleIDUnsafe returns the next free "r-NNN" identifier, scanning
// existing reservoir IDs of the form "r-<digits>" for the highest numeric
// suffix (an ID that doesn't match this shape — e.g. hand-picked via the
// API's optional ID field — is simply ignored by the scan, never crashes
// it). Caller must hold e.mu (write lock — called only from
// UpsertRafaleQuestion).
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
