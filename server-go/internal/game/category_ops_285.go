// Category deletion support (v11.4.0, #285, contracts/http-endpoints.md
// §Catégories): history merge into « Sans catégorie » and reservoir
// reassignment / deletion. The HTTP orchestration lives in
// internal/server/categories.go; everything touching engine-owned state is here.
package game

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// historyBackupKeep is how many history.json.bak-<ts> files are retained.
const historyBackupKeep = 5

// historyEventTouches reports whether an event belongs to category key, either
// as its QuestionCategory or through a CategoryBreakdown share.
func historyEventTouches(ev *GameEvent, key string) bool {
	if ev.QuestionCategory == key {
		return true
	}
	_, ok := ev.CategoryBreakdown[key]
	return ok
}

// CountHistoryCategory returns how many history events reference category key
// and the sum of their points (the 409 "category in use" payload).
func (e *Engine) CountHistoryCategory(key string) (events, points int) {
	if key == "" {
		return 0, 0
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	for i := range e.history {
		if historyEventTouches(&e.history[i], key) {
			events++
			points += e.history[i].Points
		}
	}
	return events, points
}

// MergeHistoryCategory merges category key into « Sans catégorie »: see
// MergeHistoryCategoryWithBackup, whose backup file name it discards.
func (e *Engine) MergeHistoryCategory(key string) (events, points int, err error) {
	events, points, _, err = e.MergeHistoryCategoryWithBackup(key)
	return events, points, err
}

// MergeHistoryCategoryWithBackup rewrites the history so that category key no
// longer exists in it (contract §Catégories, step H), all under e.mu:
//   - QUESTION_CATEGORY == key ⇒ "" and the CATEGORY_NAME/IMAGE_URL/COLOR
//     snapshots are cleared (they describe a category that is gone);
//   - CATEGORY_BREAKDOWN[key] is ADDED to CATEGORY_BREAKDOWN[""] and the key
//     removed — the parts still sum to POINTS.
//
// When at least one event is concerned and a history path is configured, the
// current file is first copied byte for byte to history.json.bak-<YYYYMMDD-HHmmss>
// (5 most recent kept) and the new content is written atomically. ANY failure
// returns the error with the in-memory history and the file UNCHANGED. No
// concerned event ⇒ no backup, no rewrite. backup is "" in that case.
func (e *Engine) MergeHistoryCategoryWithBackup(key string) (events, points int, backup string, err error) {
	if key == "" {
		return 0, 0, "", nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	merged := make([]GameEvent, len(e.history))
	copy(merged, e.history)
	for i := range merged {
		ev := &merged[i]
		if !historyEventTouches(ev, key) {
			continue
		}
		events++
		points += ev.Points
		if ev.QuestionCategory == key {
			ev.QuestionCategory = ""
			ev.CategoryDisplayName, ev.CategoryImageURL, ev.CategoryColor = "", "", ""
		}
		if share, ok := ev.CategoryBreakdown[key]; ok {
			nb := make(map[string]int, len(ev.CategoryBreakdown))
			for k, v := range ev.CategoryBreakdown {
				if k != key {
					nb[k] = v
				}
			}
			nb[""] += share
			ev.CategoryBreakdown = nb
		}
	}
	if events == 0 {
		return 0, 0, "", nil
	}

	if e.historyPath != "" {
		dir := filepath.Dir(e.historyPath)
		if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
			return 0, 0, "", fmt.Errorf("history: creating %s: %w", dir, mkErr)
		}
		backup = "history.json.bak-" + time.Now().Format("20060102-150405")
		if cur, rdErr := os.ReadFile(e.historyPath); rdErr == nil {
			if wErr := os.WriteFile(filepath.Join(dir, backup), cur, 0644); wErr != nil {
				return 0, 0, "", fmt.Errorf("history backup: %w", wErr)
			}
		} else if os.IsNotExist(rdErr) {
			// Not persisted yet (async save pending): back up the in-memory state.
			old, mErr := marshalHistory(e.history)
			if mErr != nil {
				return 0, 0, "", mErr
			}
			if wErr := os.WriteFile(filepath.Join(dir, backup), old, 0644); wErr != nil {
				return 0, 0, "", fmt.Errorf("history backup: %w", wErr)
			}
		} else {
			return 0, 0, "", fmt.Errorf("history backup: %w", rdErr)
		}
		data, mErr := marshalHistory(merged)
		if mErr != nil {
			return 0, 0, "", mErr
		}
		if wErr := atomicWriteFile(dir, e.historyPath, data); wErr != nil {
			return 0, 0, "", fmt.Errorf("history write: %w", wErr)
		}
		pruneHistoryBackups(dir)
	}

	e.history = merged
	log.Printf("[Engine] History: category %s merged into « Sans catégorie » (%d events, %d points, backup=%q)", key, events, points, backup)
	return events, points, backup, nil
}

// pruneHistoryBackups keeps only the historyBackupKeep most recent
// history.json.bak-* files in dir (the timestamp sorts lexicographically).
func pruneHistoryBackups(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var baks []string
	for _, en := range entries {
		if !en.IsDir() && strings.HasPrefix(en.Name(), "history.json.bak-") {
			baks = append(baks, en.Name())
		}
	}
	sort.Strings(baks)
	for len(baks) > historyBackupKeep {
		_ = os.Remove(filepath.Join(dir, baks[0]))
		baks = baks[1:]
	}
}

// ReassignRafaleCategory moves every reservoir answer of category from to
// category to ("" = « Sans catégorie », not drawable until recategorised) and
// persists. Returns how many answers changed.
func (e *Engine) ReassignRafaleCategory(from, to string) (int, error) {
	e.mu.Lock()
	n := 0
	for _, q := range e.rafaleQuestions {
		if string(q.Category) == from {
			q.Category = QuestionCategory(to)
			n++
		}
	}
	e.mu.Unlock()
	if n == 0 {
		return 0, nil
	}
	return n, e.SaveRafale()
}

// CountRafaleCategory returns how many reservoir answers carry category key.
func (e *Engine) CountRafaleCategory(key string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	n := 0
	for _, q := range e.rafaleQuestions {
		if string(q.Category) == key {
			n++
		}
	}
	return n
}

// DeleteRafaleQuestionsByCategory removes every reservoir answer of category
// key (and their rafale_used flags) and persists both files.
func (e *Engine) DeleteRafaleQuestionsByCategory(key string) (int, error) {
	e.mu.Lock()
	n := 0
	for id, q := range e.rafaleQuestions {
		if string(q.Category) == key {
			delete(e.rafaleQuestions, id)
			delete(e.rafaleUsed, id)
			n++
		}
	}
	e.mu.Unlock()
	if n == 0 {
		return 0, nil
	}
	if err := e.SaveRafale(); err != nil {
		return n, err
	}
	return n, e.SaveRafaleUsed()
}

func marshalHistory(h []GameEvent) ([]byte, error) {
	return json.MarshalIndent(h, "", "  ")
}
