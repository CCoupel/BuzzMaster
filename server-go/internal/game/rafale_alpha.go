package game

import (
	"sort"
	"strings"
	"unicode"
)

// RAFALE « Alphabétique » (v11.4.0, #282, contracts/rafale.md §15): the
// round is drawn in alphabetical passes A→Z (one answer per letter per pass),
// the first normalised letter of the current answer is public by design
// (GameState.RafaleCurrentLetter), the answer itself never is (§2.3).

// RafaleWheelLayoutStrip / RafaleWheelLayoutArc are the two valid values of
// TypedContent.RafaleWheelLayout ("" is read as strip by the TV).
const (
	RafaleWheelLayoutStrip = "strip"
	RafaleWheelLayoutArc   = "arc"
)

// ValidRafaleWheelLayout reports whether s is an accepted RAFALE_WHEEL_LAYOUT
// value for an upload ("" is NOT handled here: absent means "").
func ValidRafaleWheelLayout(s string) bool {
	return s == RafaleWheelLayoutStrip || s == RafaleWheelLayoutArc
}

// rafaleAlphaState is the ephemeral alphabetical draw state of one round
// (contract §15.4): never serialised, rebuilt at each round start, released
// by stopUnsafe/Ready/InitGame. nil ⇔ option off.
type rafaleAlphaState struct {
	queue []*RafaleQuestion // full draw order, consumed from the front
}

// rafaleFoldTable folds the Latin-1 / Latin Extended-A letters (upper and
// lower case) onto ASCII — a built-in table, no external dependency (§15.3).
var rafaleFoldTable = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a", 'ă': "a", 'ą': "a",
	'ç': "c", 'ć': "c", 'ĉ': "c", 'ċ': "c", 'č': "c",
	'ď': "d", 'đ': "d", 'ð': "d",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ĕ': "e", 'ė': "e", 'ę': "e", 'ě': "e",
	'ĝ': "g", 'ğ': "g", 'ġ': "g", 'ģ': "g", 'ĥ': "h", 'ħ': "h",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ĩ': "i", 'ī': "i", 'ĭ': "i", 'į': "i", 'ı': "i",
	'ĵ': "j", 'ķ': "k", 'ĺ': "l", 'ļ': "l", 'ľ': "l", 'ŀ': "l", 'ł': "l",
	'ñ': "n", 'ń': "n", 'ņ': "n", 'ň': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o", 'ŏ': "o", 'ő': "o",
	'ŕ': "r", 'ŗ': "r", 'ř': "r", 'ś': "s", 'ŝ': "s", 'ş': "s", 'š': "s", 'ţ': "t", 'ť': "t", 'ŧ': "t",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ũ': "u", 'ū': "u", 'ŭ': "u", 'ů': "u", 'ű': "u", 'ų': "u",
	'ŵ': "w", 'ý': "y", 'ÿ': "y", 'ŷ': "y", 'ź': "z", 'ż': "z", 'ž': "z",
	'œ': "oe", 'æ': "ae", 'ß': "ss",
}

// rafaleArticles is the closed list of removable leading articles (§15.3),
// longest first so "DE LA"/"DE L'" win over "DE"-less prefixes.
var rafaleArticles = []string{"DE LA ", "DE L'", "LES ", "UNE ", "DES ", "LE ", "LA ", "UN ", "DU ", "L'"}

// rafaleFold lower-folds s onto ASCII then upper-cases it.
func rafaleFold(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if f, ok := rafaleFoldTable[r]; ok {
			b.WriteString(f)
		} else {
			b.WriteRune(r)
		}
	}
	return strings.ToUpper(b.String())
}

// rafaleAlphaKey normalises a RAFALE answer for the alphabetical mode
// (contract §15.3, pure function): trims, drops leading non-alphanumerics,
// folds accents, upper-cases, removes ONE leading article, and returns the
// resulting sort key together with its letter ("A".."Z", or "#").
func rafaleAlphaKey(answer string) (letter, sortKey string) {
	s := strings.TrimSpace(answer)
	s = strings.TrimLeftFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	s = rafaleFold(s)
	s = strings.ReplaceAll(s, "’", "'")
	for _, art := range rafaleArticles {
		if strings.HasPrefix(s, art) {
			s = strings.TrimLeft(s[len(art):], " ")
			break
		}
	}
	if s != "" && s[0] >= 'A' && s[0] <= 'Z' {
		return s[:1], s
	}
	return "#", s
}

// RafaleAlphaKey is the exported entry point of the single normalisation
// (GET /api/rafale/pool uses it — never a second implementation, §15.6).
func RafaleAlphaKey(answer string) (letter, sortKey string) { return rafaleAlphaKey(answer) }

// buildRafaleAlphaQueueUnsafe builds the full draw order of a round from the
// not-yet-used pool of the given filter (contract §15.4 construction).
func (e *Engine) buildRafaleAlphaQueueUnsafe(categories []string, difficulties []int) []*RafaleQuestion {
	type entry struct {
		q   *RafaleQuestion
		key string
	}
	groups := map[string][]entry{}
	for _, q := range e.rafalePoolUnsafe(categories, difficulties) {
		l, k := rafaleAlphaKey(q.Answer)
		groups[l] = append(groups[l], entry{q, k})
	}
	order := make([]string, 0, 27)
	for c := 'A'; c <= 'Z'; c++ {
		order = append(order, string(c))
	}
	order = append(order, "#")
	total := 0
	for l, g := range groups {
		sort.SliceStable(g, func(i, j int) bool {
			if g[i].key != g[j].key {
				return g[i].key < g[j].key
			}
			return g[i].q.ID < g[j].q.ID
		})
		groups[l] = g
		total += len(g)
	}
	queue := make([]*RafaleQuestion, 0, total)
	for k := 0; len(queue) < total; k++ {
		for _, l := range order {
			if g := groups[l]; k < len(g) {
				queue = append(queue, g[k].q)
			}
		}
	}
	return queue
}

// drawNextRafaleAlphaUnsafe pops the next playable entry of the queue,
// skipping any question that became used meanwhile (§15.4), marks it used.
func (e *Engine) drawNextRafaleAlphaUnsafe() (*RafaleQuestion, error) {
	for len(e.rafaleAlpha.queue) > 0 {
		q := e.rafaleAlpha.queue[0]
		e.rafaleAlpha.queue = e.rafaleAlpha.queue[1:]
		if e.rafaleUsed[q.ID] {
			continue
		}
		drawn := *q
		e.rafaleUsed[drawn.ID] = true
		return &drawn, nil
	}
	return nil, ErrRafalePoolEmpty
}

// refreshRafaleLettersUnsafe recomputes RAFALE_LETTERS (A–Z of the answers
// still to play = queue + pre-drawn rafaleNext) and, from `current`, sets
// RAFALE_CURRENT_LETTER. No-op reset to ""/[] when the option is off.
func (e *Engine) refreshRafaleLettersUnsafe(current *RafaleQuestion) {
	if e.rafaleAlpha == nil {
		e.state.RafaleCurrentLetter = ""
		e.state.RafaleLetters = []string{}
		return
	}
	if current != nil {
		l, _ := rafaleAlphaKey(current.Answer)
		e.state.RafaleCurrentLetter = l
	}
	present := map[string]bool{}
	mark := func(q *RafaleQuestion) {
		if q == nil {
			return
		}
		if l, _ := rafaleAlphaKey(q.Answer); l != "#" {
			present[l] = true
		}
	}
	for _, q := range e.rafaleAlpha.queue {
		if !e.rafaleUsed[q.ID] {
			mark(q)
		}
	}
	mark(e.rafaleNext)
	letters := make([]string, 0, len(present))
	for c := 'A'; c <= 'Z'; c++ {
		if present[string(c)] {
			letters = append(letters, string(c))
		}
	}
	e.state.RafaleLetters = letters
}

// resetRafaleAlphaUnsafe releases the alphabetical state and clears the two
// public fields (stopUnsafe, Ready, InitGame).
func (e *Engine) resetRafaleAlphaUnsafe() {
	e.rafaleAlpha = nil
	e.state.RafaleCurrentLetter = ""
	e.state.RafaleLetters = []string{}
}

// RafaleAvailableLetters returns the sorted A–Z letters (no "#") of the
// AVAILABLE answers of a filter — GET /api/rafale/pool's LETTERS (§15.6),
// through the same rafaleAlphaKey as the draw.
func (e *Engine) RafaleAvailableLetters(categories []string, difficulties []int) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	present := map[string]bool{}
	for _, q := range e.rafalePoolUnsafe(categories, difficulties) {
		if l, _ := rafaleAlphaKey(q.Answer); l != "#" {
			present[l] = true
		}
	}
	out := make([]string, 0, len(present))
	for c := 'A'; c <= 'Z'; c++ {
		if present[string(c)] {
			out = append(out, string(c))
		}
	}
	return out
}
