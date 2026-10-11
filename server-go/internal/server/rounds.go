package server

import (
	"buzzcontrol/internal/game"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Rounds (v11.2, #245): contracts/models.md §Manches. Storage: rounds.json in the questions
// directory ({ROUNDS:[{ID,NAME}], NEXT_ID}); a question joins a round through its ROUND_ID.

// QuestionFilesMu serialises the operations that rewrite several question files at once
// (ROUNDS_UPDATE) against those that compute an ORDER from the whole set (creation, AI).
var QuestionFilesMu sync.Mutex

// RoundsFile is the content of rounds.json.
type RoundsFile struct {
	Rounds []game.Round `json:"ROUNDS"`
	NextID int          `json:"NEXT_ID"`
}

// RoundsFilePath returns the rounds.json path inside a questions directory.
func RoundsFilePath(questionsDir string) string { return filepath.Join(questionsDir, "rounds.json") }

// LoadRoundsFile reads rounds.json; an absent or unreadable file means "no round".
// Rounds is never nil, NextID is at least 1 and above every existing "rN".
func LoadRoundsFile(questionsDir string) RoundsFile {
	rf := RoundsFile{Rounds: []game.Round{}, NextID: 1}
	data, err := os.ReadFile(RoundsFilePath(questionsDir))
	if err != nil {
		return rf
	}
	var parsed RoundsFile
	if json.Unmarshal(data, &parsed) != nil {
		return rf
	}
	if parsed.Rounds != nil {
		rf.Rounds = parsed.Rounds
	}
	rf.NextID = parsed.NextID
	for _, r := range rf.Rounds {
		if n, err := strconv.Atoi(strings.TrimPrefix(r.ID, "r")); err == nil && n >= rf.NextID {
			rf.NextID = n + 1
		}
	}
	if rf.NextID < 1 {
		rf.NextID = 1
	}
	return rf
}

// SaveRoundsFile writes rounds.json atomically (temporary file + rename).
func SaveRoundsFile(questionsDir string, rf RoundsFile) error {
	if rf.Rounds == nil {
		rf.Rounds = []game.Round{}
	}
	if err := os.MkdirAll(questionsDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(questionsDir, ".rounds.json.tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, RoundsFilePath(questionsDir)); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// RoundGroupIndex is the play-order group of a round ID: 0 = "Sans manche" (absent, "" or
// unknown ID), 1.. = rank of the round in ROUNDS.
func RoundGroupIndex(rounds []game.Round, roundID string) int {
	if roundID == "" {
		return 0
	}
	for i, r := range rounds {
		if r.ID == roundID {
			return i + 1
		}
	}
	return 0
}

// QuestionOrderValue returns the ORDER used to sort a question: its ORDER field (number or
// numeric string), else its numeric ID, else 0 (the historical rule).
func QuestionOrderValue(q map[string]interface{}) int {
	if orderRaw, present := q["ORDER"]; present {
		switch v := orderRaw.(type) {
		case float64:
			return int(v)
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				return n
			}
		}
		return 0
	}
	if id, _ := q["ID"].(string); id != "" {
		if n, err := strconv.Atoi(id); err == nil {
			return n
		}
	}
	return 0
}

// OrderedQuestion is a question placed in the single play order.
type OrderedQuestion struct {
	ID      string
	Order   int
	Group   int // 0 = Sans manche, 1.. = round rank
	RoundID string
	Type    string
	Data    map[string]interface{}
}

// OrderQuestions sorts questions by THE play-order rule (contracts/models.md §Manches): group
// "Sans manche" first, then the rounds in ROUNDS order, and inside a group by ORDER then by
// numeric ID (string compare when not both numeric). Entries without an ID are dropped.
func OrderQuestions(raw []map[string]interface{}, rounds []game.Round) []OrderedQuestion {
	out := make([]OrderedQuestion, 0, len(raw))
	for _, q := range raw {
		id, _ := q["ID"].(string)
		if id == "" {
			continue
		}
		rid, _ := q["ROUND_ID"].(string)
		typ, _ := q["TYPE"].(string)
		g := RoundGroupIndex(rounds, rid)
		if g == 0 {
			rid = ""
		}
		out = append(out, OrderedQuestion{ID: id, Order: QuestionOrderValue(q), Group: g, RoundID: rid, Type: typ, Data: q})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Group != b.Group {
			return a.Group < b.Group
		}
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		na, ea := strconv.Atoi(a.ID)
		nb, eb := strconv.Atoi(b.ID)
		if ea == nil && eb == nil {
			return na < nb
		}
		return a.ID < b.ID
	})
	return out
}

// ReadQuestionMaps reads every <dir>/<id>/question.json as a generic map (unreadable ones skipped).
func ReadQuestionMaps(questionsDir string) []map[string]interface{} {
	entries, err := os.ReadDir(questionsDir)
	if err != nil {
		return nil
	}
	var out []map[string]interface{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(questionsDir, e.Name(), "question.json"))
		if err != nil {
			continue
		}
		var q map[string]interface{}
		if json.Unmarshal(data, &q) != nil {
			continue
		}
		if id, _ := q["ID"].(string); id == "" {
			q["ID"] = e.Name()
		}
		out = append(out, q)
	}
	return out
}

// NextOrderInGroup returns max(ORDER)+1 over the questions of the given group (0 when empty):
// the ORDER of a question appended at the END of that group.
func NextOrderInGroup(ordered []OrderedQuestion, group int) int {
	next, seen := 0, false
	for _, q := range ordered {
		if q.Group == group && (!seen || q.Order+1 > next) {
			next, seen = q.Order+1, true
		}
	}
	return next
}
