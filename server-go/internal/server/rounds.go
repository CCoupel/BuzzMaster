package server

import (
	"buzzcontrol/internal/game"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// BumpNextID raises NextID above the numeric part of a round ID "rN" (an ID is never reused).
func (rf *RoundsFile) BumpNextID(roundID string) {
	if n, err := strconv.Atoi(strings.TrimPrefix(roundID, "r")); err == nil && strings.HasPrefix(roundID, "r") && n >= rf.NextID {
		rf.NextID = n + 1
	}
}

// RoundsFilePath returns the rounds.json path inside a questions directory.
func RoundsFilePath(questionsDir string) string { return filepath.Join(questionsDir, "rounds.json") }

// ErrRoundsFileUnreadable is returned when rounds.json exists but cannot be parsed.
var ErrRoundsFileUnreadable = errors.New("rounds.json unreadable")

// LoadRoundsFile reads rounds.json; an absent or unreadable file means "no round" (an
// unreadable one is journalised and copied to rounds.json.corrupt, see LoadRoundsFileChecked).
func LoadRoundsFile(questionsDir string) RoundsFile {
	rf, _ := LoadRoundsFileChecked(questionsDir)
	return rf
}

// LoadRoundsFileChecked is LoadRoundsFile plus the error: nil when the file is absent or
// valid, ErrRoundsFileUnreadable when it exists but is corrupt (WARN, once-copied to
// rounds.json.corrupt). Rounds is never nil; NextID is at least 1 and above every existing
// "rN" (rounds.json AND the ROUND_ID found in the questions: an ID is never reused).
func LoadRoundsFileChecked(questionsDir string) (RoundsFile, error) {
	rf := RoundsFile{Rounds: []game.Round{}, NextID: 1}
	var loadErr error
	if data, err := os.ReadFile(RoundsFilePath(questionsDir)); err == nil {
		var parsed RoundsFile
		if jerr := json.Unmarshal(data, &parsed); jerr != nil {
			loadErr = ErrRoundsFileUnreadable
			LogWarn(game.LogComponentApp, "rounds.json unreadable (%v): treated as no round, copy kept as rounds.json.corrupt", jerr)
			corrupt := RoundsFilePath(questionsDir) + ".corrupt"
			if _, statErr := os.Stat(corrupt); os.IsNotExist(statErr) {
				_ = os.WriteFile(corrupt, data, 0o644)
			}
		} else {
			if parsed.Rounds != nil {
				rf.Rounds = parsed.Rounds
			}
			rf.NextID = parsed.NextID
		}
	}
	for _, r := range rf.Rounds {
		rf.BumpNextID(r.ID)
	}
	if rf.NextID < 1 {
		rf.NextID = 1
	}
	return rf, loadErr
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

// QuestionOrderValue returns the ORDER used to sort a question. Normalisation rules
// (documented, mirrored by the JS in questionOrder.js):
//   - ORDER a JSON number: truncated to an integer (negative values allowed);
//   - ORDER a string: used only if the WHOLE trimmed string is an integer ("5" yes, "5abc" no);
//   - ORDER absent, null, any other type or an invalid string: treated as ABSENT, i.e. the
//     numeric value of the ID when it is numeric, else 0.
func QuestionOrderValue(q map[string]interface{}) int {
	switch v := q["ORDER"].(type) {
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	if id, _ := q["ID"].(string); id != "" {
		if n, err := strconv.Atoi(id); err == nil {
			return n
		}
	}
	return 0
}

// validQuestionID is the shape of a question identifier: a directory name made of letters,
// digits, '_' or '-' (the real ones are numeric); never '.', '/', '\' or '..'.
var validQuestionID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidQuestionID reports whether id is an acceptable question identifier.
func ValidQuestionID(id string) bool { return validQuestionID.MatchString(id) }

// SafeQuestionPath returns questionsDir/id after checking the id shape AND that the resolved
// path stays strictly inside questionsDir (no '..' escape): the only way to build a path
// before a RemoveAll or a write.
func SafeQuestionPath(questionsDir, id string) (string, error) {
	if !ValidQuestionID(id) {
		return "", fmt.Errorf("invalid question id %q", id)
	}
	p := filepath.Join(questionsDir, id)
	rel, err := filepath.Rel(questionsDir, p)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("question id %q escapes the questions directory", id)
	}
	return p, nil
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
		// tie-break: numeric IDs first (by value), then the others (lexicographic)
		na, ea := strconv.Atoi(a.ID)
		nb, eb := strconv.Atoi(b.ID)
		switch {
		case ea == nil && eb == nil:
			return na < nb
		case ea == nil:
			return true
		case eb == nil:
			return false
		}
		return a.ID < b.ID
	})
	return out
}

// ReadQuestionMaps reads every <dir>/<id>/question.json as a generic map. The identity of a
// question is the NAME OF ITS DIRECTORY, never the content: q["ID"] is forced to it. Unreadable
// files and directories whose name is not a valid ID are skipped.
func ReadQuestionMaps(questionsDir string) []map[string]interface{} {
	maps, _ := ReadQuestionMapsChecked(questionsDir)
	return maps
}

// ReadQuestionMapsChecked is ReadQuestionMaps plus the anomalies found: a question.json whose
// ID field differs from its directory name, or a directory with an invalid name (each is
// journalised). Callers that write (ROUNDS_UPDATE) refuse to act when anomalies exist.
func ReadQuestionMapsChecked(questionsDir string) ([]map[string]interface{}, []string) {
	entries, err := os.ReadDir(questionsDir)
	if err != nil {
		return nil, nil
	}
	var out []map[string]interface{}
	var anomalies []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		data, err := os.ReadFile(filepath.Join(questionsDir, name, "question.json"))
		if err != nil {
			continue
		}
		if !ValidQuestionID(name) {
			anomalies = append(anomalies, name)
			LogWarn(game.LogComponentApp, "question directory %q has an invalid name: ignored", name)
			continue
		}
		var q map[string]interface{}
		if json.Unmarshal(data, &q) != nil {
			continue
		}
		if id, _ := q["ID"].(string); id != "" && id != name {
			anomalies = append(anomalies, name)
			LogWarn(game.LogComponentApp, "question %q: question.json declares ID %q — the directory name wins", name, id)
		}
		q["ID"] = name
		out = append(out, q)
	}
	return out, anomalies
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
