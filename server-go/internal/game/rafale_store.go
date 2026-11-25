package game

// Engine-level API of the RAFALE reservoir — thin wrappers over the `rafale`
// EntryPool instance (pool_store.go, contracts/picto-pool.md §1). Names,
// signatures and behaviour are those of the pre-v13.1.0 store: every caller
// (cmd/server, internal/server, tests) is unchanged.

// SetRafalePath sets the path for the RAFALE reservoir persistence file.
func (e *Engine) SetRafalePath(path string) { e.rafalePool.SetPath(path) }

// SaveRafale persists the RAFALE reservoir to disk (single file).
func (e *Engine) SaveRafale() error { return e.rafalePool.Save() }

// LoadRafale loads the RAFALE reservoir from disk (a missing file is not an
// error).
func (e *Engine) LoadRafale() error { return e.rafalePool.Load() }

// ClearRafaleReservoir empties the in-memory reservoir (selective reset,
// contracts/rafale.md §10).
func (e *Engine) ClearRafaleReservoir() { e.rafalePool.Clear() }

// SetRafaleUsedPath sets the path for the RAFALE "already used" flag file.
func (e *Engine) SetRafaleUsedPath(path string) { e.rafalePool.SetUsedPath(path) }

// SaveRafaleUsed persists the "already used" flag map to disk.
func (e *Engine) SaveRafaleUsed() error { return e.rafalePool.SaveUsed() }

// LoadRafaleUsed loads the "already used" flag map from disk.
func (e *Engine) LoadRafaleUsed() error { return e.rafalePool.LoadUsed() }

// ClearRafaleUsed empties the in-memory "already used" flag map (InitGame and
// the selective reset endpoint).
func (e *Engine) ClearRafaleUsed() { e.rafalePool.ClearUsed() }

// SnapshotRafaleReservoir returns a defensive copy of the reservoir (sorted
// by ID) and the current "already used" flags (GET /api/rafale/questions).
func (e *Engine) SnapshotRafaleReservoir() (questions []RafaleQuestion, used map[string]bool) {
	return e.rafalePool.Snapshot()
}

// GetRafaleQuestion returns a copy of one reservoir question by ID.
func (e *Engine) GetRafaleQuestion(id string) (RafaleQuestion, bool) {
	return e.rafalePool.Get(id)
}

// UpsertRafaleQuestion creates (q.ID == "") or updates one reservoir
// question and persists synchronously — see EntryPool.Upsert.
func (e *Engine) UpsertRafaleQuestion(q RafaleQuestion) (RafaleQuestion, error) {
	return e.rafalePool.Upsert(q)
}

// AppendRafaleQuestions adds a batch of reservoir questions under one lock
// and one save (feature #203) — see EntryPool.Append.
func (e *Engine) AppendRafaleQuestions(qs []RafaleQuestion) ([]RafaleQuestion, error) {
	return e.rafalePool.Append(qs)
}

// DeleteRafaleQuestion removes one reservoir question by ID; returns
// ErrRafaleQuestionNotFound if absent.
func (e *Engine) DeleteRafaleQuestion(id string) error { return e.rafalePool.Delete(id) }

// ResetAllRafaleUsed clears the ENTIRE "already used" flag map and persists
// synchronously; returns the number of entries cleared.
func (e *Engine) ResetAllRafaleUsed() (int, error) { return e.rafalePool.ResetAllUsed() }

// MarkRafaleQuestionAvailable removes one question from the "already used"
// flag (persisted synchronously); ErrRafaleQuestionNotFound if absent.
func (e *Engine) MarkRafaleQuestionAvailable(id string) error {
	return e.rafalePool.MarkAvailable(id)
}
