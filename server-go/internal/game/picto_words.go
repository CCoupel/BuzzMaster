package game

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// PICTO words (v13.0.0, #191, contracts/picto.md §3.1) — the single
// implementation of the word rules, server side. The editor only displays the
// server's refusal.

// Limits of the PICTO question (contracts/picto.md §3.1 and §4).
const (
	PictoWordMaxRunes = 40
	PictoMaxWords     = 20
	PictoMinTime      = 30
	PictoMaxTime      = 300
)

// PICTO error codes (HTTP 400 prefixes, contracts/picto.md §11).
const (
	PictoErrWordsInvalid = "PICTO_WORDS_INVALID"
	PictoErrTimeInvalid  = "PICTO_TIME_INVALID"
	// v13.1.0 (#324, contracts/picto.md §17.2): source and filter of the word.
	PictoErrSourceInvalid = "PICTO_SOURCE_INVALID"
	PictoErrFilterInvalid = "PICTO_FILTER_INVALID"
)

// ErrPictoWord is the sentinel wrapped by every refused word / list.
var ErrPictoWord = errors.New(PictoErrWordsInvalid)

func pictoWordRuneAllowed(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '\'' || r == '’' || r == '-'
}

// NormalizePictoWord applies the §3.1 format rules to one word: leading and
// trailing spaces removed, inner spaces compacted, 1 to 40 characters, only
// letters (accents included), digits, space, apostrophe and hyphen. The
// typographic apostrophe is stored as '. Returns the cleaned word.
func NormalizePictoWord(raw string) (string, error) {
	for _, r := range raw {
		if !pictoWordRuneAllowed(r) {
			return "", fmt.Errorf("%w: caractère non admis %q dans %q", ErrPictoWord, r, raw)
		}
	}
	w := strings.Join(strings.Fields(raw), " ")
	w = strings.ReplaceAll(w, "’", "'")
	n := utf8.RuneCountInString(w)
	if n == 0 {
		return "", fmt.Errorf("%w: mot vide", ErrPictoWord)
	}
	if n > PictoWordMaxRunes {
		return "", fmt.Errorf("%w: %q dépasse %d caractères", ErrPictoWord, w, PictoWordMaxRunes)
	}
	return w, nil
}

// PictoWordKey is the folded uniqueness key of a word: case and accents
// folded with the same folding as rafaleAlphaKey (shared foldKey), articles
// kept. "Éléphant" and "elephant" share one key.
func PictoWordKey(word string) string { return foldKey(word) }

// ValidatePictoWords cleans and checks a question's word list: 1 to 20 words,
// each valid (§3.1), no duplicate folded key within the question. Returns the
// cleaned list, in the given order.
func ValidatePictoWords(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: la liste est vide (1 à %d mots)", ErrPictoWord, PictoMaxWords)
	}
	if len(raw) > PictoMaxWords {
		return nil, fmt.Errorf("%w: %d mots, maximum %d", ErrPictoWord, len(raw), PictoMaxWords)
	}
	out := make([]string, 0, len(raw))
	seen := make(map[string]string, len(raw))
	for _, r := range raw {
		w, err := NormalizePictoWord(r)
		if err != nil {
			return nil, err
		}
		k := PictoWordKey(w)
		if prev, dup := seen[k]; dup {
			return nil, fmt.Errorf("%w: doublon « %s » / « %s »", ErrPictoWord, prev, w)
		}
		seen[k] = w
		out = append(out, w)
	}
	return out, nil
}

// ValidPictoTime reports whether t (seconds) is in the 30–300 range.
func ValidPictoTime(t int) bool { return t >= PictoMinTime && t <= PictoMaxTime }
