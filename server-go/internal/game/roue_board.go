package game

import "strings"

// ROUE « Roue de la Fortune » (v12.0.0, #145, contracts/roue.md §3.1): the
// phrase to guess, its validation and its layout on the 4 × 14 board. One
// single implementation, server side — the editor gets its preview through
// POST /api/roue/preview, never from a JavaScript copy of these rules.
// The accent folding is the one of RAFALE (rafaleFold, rafale_alpha.go).

// Board geometry and limits (contracts/roue.md §3, §3.1).
const (
	RoueBoardCols            = 14
	RoueBoardMaxRows         = 4
	RouePresentationMaxRunes = 80
)

// Error codes of the phrase validation (contracts/roue.md §11.1). They are
// also the prefix of the 400 text answered by POST /questions.
const (
	RoueErrPhraseEmpty             = "ROUE_PHRASE_EMPTY"
	RoueErrPhraseInvalidChar       = "ROUE_PHRASE_INVALID_CHAR"
	RoueErrPhraseWordTooLong       = "ROUE_PHRASE_WORD_TOO_LONG"
	RoueErrPhraseTooLong           = "ROUE_PHRASE_TOO_LONG"
	RoueErrPresentationLeaksPhrase = "ROUE_PRESENTATION_CONTAINS_PHRASE"
	RoueErrPresentationTooLong     = "ROUE_PRESENTATION_TOO_LONG"
	RoueErrModeInvalid             = "ROUE_MODE_INVALID"
	RoueErrGainsRuleInvalid        = "ROUE_GAINS_RULE_INVALID"
	RoueErrWheelLayoutInvalid      = "ROUE_WHEEL_LAYOUT_INVALID"
)

// Valid values of the ROUE_* enumerations (contracts/roue.md §3).
const (
	RoueModeSolo           = "SOLO"
	RoueModeChacunSonTour  = "CHACUN_SON_TOUR"
	RoueModeTantQueJeGagne = "TANT_QUE_JE_GAGNE"
	RoueGainsWinnerOnly    = "WINNER_ONLY"
	RoueGainsAllTeams      = "ALL_TEAMS"
)

// RoueError is a refused phrase / field: Code is one of the RoueErr* codes.
type RoueError struct {
	Code    string
	Message string
}

func (e *RoueError) Error() string { return e.Code + ": " + e.Message }

// RoueBoardLayout is the layout of a validated phrase.
type RoueBoardLayout struct {
	Cols int
	Rows []string // one string per used line, words never cut, no padding
	// Consonants / Letters: number of DISTINCT consonants / letters to find.
	Consonants int
	Letters    int
}

// ValidRoueMode / ValidRoueGainsRule / ValidRoueWheelLayout report whether s
// is an accepted value ("" is not handled here: absent means the default).
func ValidRoueMode(s string) bool {
	return s == RoueModeSolo || s == RoueModeChacunSonTour || s == RoueModeTantQueJeGagne
}

func ValidRoueGainsRule(s string) bool { return s == RoueGainsWinnerOnly || s == RoueGainsAllTeams }

func ValidRoueWheelLayout(s string) bool { return ValidRafaleWheelLayout(s) }

// IsRoueVowel reports whether the (folded, upper-case) letter is one of the
// 6 vowels never proposable in v12.0.
func IsRoueVowel(r rune) bool { return strings.ContainsRune("AEIOUY", r) }

// RoueCompactPhrase returns the phrase as stored: trimmed, every run of
// whitespace compacted to one space, nothing else changed.
func RoueCompactPhrase(phrase string) string {
	return strings.Join(strings.Fields(phrase), " ")
}

// roueAllowedPunct lists the visible punctuation of the board.
const roueAllowedPunct = "'-.,!?:"

// RoueFoldPhrase validates a phrase and returns its board form: upper case,
// accents folded (É→E, Ç→C, Œ→OE), apostrophes straightened, spaces compacted.
func RoueFoldPhrase(phrase string) (string, error) {
	compact := RoueCompactPhrase(phrase)
	if compact == "" {
		return "", &RoueError{RoueErrPhraseEmpty, "la phrase est vide"}
	}
	compact = strings.NewReplacer("’", "'", "‘", "'").Replace(compact)
	folded := rafaleFold(compact)
	for _, r := range folded {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == ' ', strings.ContainsRune(roueAllowedPunct, r):
		default:
			return "", &RoueError{RoueErrPhraseInvalidChar, "caractère non admis : " + string(r)}
		}
	}
	return folded, nil
}

// RoueBuildBoard validates a phrase and lays it out on the board: greedy
// filling line after line, words never cut (contracts/roue.md §3.1).
func RoueBuildBoard(phrase string) (RoueBoardLayout, error) {
	folded, err := RoueFoldPhrase(phrase)
	if err != nil {
		return RoueBoardLayout{}, err
	}
	var rows []string
	line := ""
	for _, w := range strings.Split(folded, " ") {
		if n := len([]rune(w)); n > RoueBoardCols {
			return RoueBoardLayout{}, &RoueError{RoueErrPhraseWordTooLong,
				"le mot « " + w + " » dépasse 14 caractères"}
		}
		switch {
		case line == "":
			line = w
		case len([]rune(line))+1+len([]rune(w)) <= RoueBoardCols:
			line += " " + w
		default:
			rows = append(rows, line)
			line = w
		}
	}
	rows = append(rows, line)
	if len(rows) > RoueBoardMaxRows {
		return RoueBoardLayout{}, &RoueError{RoueErrPhraseTooLong,
			"la phrase demande plus de 4 lignes de 14 caractères"}
	}
	seen := map[rune]bool{}
	consonants := 0
	for _, r := range folded {
		if r < 'A' || r > 'Z' || seen[r] {
			continue
		}
		seen[r] = true
		if !IsRoueVowel(r) {
			consonants++
		}
	}
	return RoueBoardLayout{Cols: RoueBoardCols, Rows: rows, Consonants: consonants, Letters: len(seen)}, nil
}

// ValidateRoueFields checks the optional fields of a ROUE question (empty =
// absent = default) and the presentation length.
func ValidateRoueFields(presentation, mode, gainsRule, wheelLayout string) error {
	if len([]rune(presentation)) > RouePresentationMaxRunes {
		return &RoueError{RoueErrPresentationTooLong, "la présentation dépasse 80 caractères"}
	}
	if mode != "" && !ValidRoueMode(mode) {
		return &RoueError{RoueErrModeInvalid, "ROUE_MODE doit valoir SOLO, CHACUN_SON_TOUR ou TANT_QUE_JE_GAGNE"}
	}
	if gainsRule != "" && !ValidRoueGainsRule(gainsRule) {
		return &RoueError{RoueErrGainsRuleInvalid, "roue_gains_rule doit valoir WINNER_ONLY ou ALL_TEAMS"}
	}
	if wheelLayout != "" && !ValidRoueWheelLayout(wheelLayout) {
		return &RoueError{RoueErrWheelLayoutInvalid, "roue_wheel_layout doit valoir strip ou arc"}
	}
	return nil
}
