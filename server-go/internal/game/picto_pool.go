package game

import (
	"errors"
	"fmt"
	"log"
)

// Pool Picto (v13.1.0, issue fille #324, contracts/picto-pool.md §2) — the
// word rules applied to the `picto` EntryPool instance: the word is stored in
// QUESTION, ANSWER stays "". Same validator as the PICTO question's own word
// list (NormalizePictoWord, picto_words.go), plus the per-category duplicate
// rule (I7, folded key) and the 5 000-entry ceiling.

// PictoPoolMaxEntries is the ceiling of the `picto` pool (POOL_FULL).
const PictoPoolMaxEntries = 5000

// Refusal codes of the Pool Picto (contracts/picto-pool.md §4).
const (
	PoolErrWordInvalid = "WORD_INVALID"
	PoolErrDuplicate   = "DUPLICATE"
	PoolErrPoolFull    = "POOL_FULL"
)

// PoolRefusal is one refused word, with its contract code.
type PoolRefusal struct {
	Word string
	Code string
	Err  error // human detail, never the sole carrier of the code
}

// ErrPoolEntryNotFoundForUpdate is returned by UpdateWord for an unknown ID.
var ErrPoolEntryNotFoundForUpdate = errors.New("pool_entry_not_found")

// PoolWordFieldSet describes the validation of a pool whose entries are
// words: one implementation parameterised by field set, reused by RAFALE_PICTO
// (LOT-3) through the same pool.
type PoolWordFieldSet struct {
	MaxEntries int
}

// PictoWordFieldSet is the field set of the `picto` pool.
var PictoWordFieldSet = PoolWordFieldSet{MaxEntries: PictoPoolMaxEntries}

// checkWordUnsafe validates one word for category cat against the pool's
// current content (excluding entry excludeID, for an update). Returns the
// cleaned word, or a refusal code. Caller holds the pool's mutex.
func (p *EntryPool) checkWordUnsafe(fs PoolWordFieldSet, raw string, cat QuestionCategory, excludeID string, pending map[string]bool) (string, *PoolRefusal) {
	w, err := NormalizePictoWord(raw)
	if err != nil {
		return "", &PoolRefusal{Word: raw, Code: PoolErrWordInvalid, Err: err}
	}
	key := PictoWordKey(w)
	if pending[key] {
		return "", &PoolRefusal{Word: raw, Code: PoolErrDuplicate, Err: fmt.Errorf("doublon dans le lot : %q", w)}
	}
	for _, e := range p.entries {
		if e.ID != excludeID && e.Category == cat && PictoWordKey(e.Question) == key {
			return "", &PoolRefusal{Word: raw, Code: PoolErrDuplicate, Err: fmt.Errorf("doublon dans la catégorie : %q", e.Question)}
		}
	}
	return w, nil
}

// AddWords validates then stores words under (cat, diff) in ONE locked
// operation followed by exactly one Save() (the paste endpoint, §4). Valid
// words are created; refused ones are listed, never truncated; a word equal
// (folded) to an earlier word of the same call is refused from its 2nd
// occurrence. Once fs.MaxEntries is reached the remaining words are refused
// POOL_FULL. Category and difficulty are the caller's to validate.
func (p *EntryPool) AddWords(fs PoolWordFieldSet, words []string, cat QuestionCategory, diff int) (created []PoolEntry, refused []PoolRefusal, err error) {
	if len(words) == 0 {
		return nil, nil, nil
	}
	p.mu.Lock()
	pending := make(map[string]bool, len(words))
	for _, raw := range words {
		if fs.MaxEntries > 0 && len(p.entries) >= fs.MaxEntries {
			refused = append(refused, PoolRefusal{Word: raw, Code: PoolErrPoolFull, Err: fmt.Errorf("pool plein (%d)", fs.MaxEntries)})
			continue
		}
		w, ref := p.checkWordUnsafe(fs, raw, cat, "", pending)
		if ref != nil {
			refused = append(refused, *ref)
			continue
		}
		pending[PictoWordKey(w)] = true
		entry := PoolEntry{ID: p.nextIDUnsafe(), Question: w, Answer: "", Category: cat, Difficulty: diff}
		cp := entry
		p.entries[entry.ID] = &cp
		created = append(created, entry)
	}
	p.mu.Unlock()

	if len(created) == 0 {
		return nil, refused, nil
	}
	if err := p.Save(); err != nil {
		return nil, nil, err
	}
	return created, refused, nil
}

// UpdateWord rewrites an existing entry (word, category, difficulty) with the
// same rules as AddWords (the entry itself excluded from the duplicate scan).
// Returns the stored entry or a refusal.
func (p *EntryPool) UpdateWord(fs PoolWordFieldSet, id, raw string, cat QuestionCategory, diff int) (PoolEntry, *PoolRefusal, error) {
	p.mu.Lock()
	cur, ok := p.entries[id]
	if !ok {
		p.mu.Unlock()
		return PoolEntry{}, nil, ErrPoolEntryNotFoundForUpdate
	}
	w, ref := p.checkWordUnsafe(fs, raw, cat, id, nil)
	if ref != nil {
		p.mu.Unlock()
		return PoolEntry{}, ref, nil
	}
	cur.Question, cur.Answer, cur.Category, cur.Difficulty = w, "", cat, diff
	out := *cur
	p.mu.Unlock()

	if err := p.Save(); err != nil {
		return PoolEntry{}, nil, err
	}
	log.Printf("[Engine] %s reservoir: entry %s updated", p.label, id)
	return out, nil, nil
}

// RafalePool and PictoPool expose the two reservoir instances (HTTP factory,
// backup, AI descriptors).
func (e *Engine) RafalePool() *EntryPool { return e.rafalePool }
func (e *Engine) PictoPool() *EntryPool  { return e.pictoPool }

// SetPictoPath / SetPictoUsedPath / LoadPicto / LoadPictoUsed — persistence of
// the `picto` instance (files/picto/reservoir.json, config/picto_used.json).
func (e *Engine) SetPictoPath(path string)     { e.pictoPool.SetPath(path) }
func (e *Engine) SetPictoUsedPath(path string) { e.pictoPool.SetUsedPath(path) }
func (e *Engine) LoadPicto() error             { return e.pictoPool.Load() }
func (e *Engine) LoadPictoUsed() error         { return e.pictoPool.LoadUsed() }
func (e *Engine) SavePictoUsed() error         { return e.pictoPool.SaveUsed() }
func (e *Engine) ClearPictoReservoir()         { e.pictoPool.Clear() }
func (e *Engine) ClearPictoUsed()              { e.pictoPool.ClearUsed() }

// IsPictoFamily reports whether t is a Picto-family question type — the
// single predicate (contracts/picto.md §17.1). RAFALE_PICTO joins it with its
// type in LOT-3; today the family is PICTO alone.
func IsPictoFamily(t QuestionType) bool { return t == QuestionTypePicto }
