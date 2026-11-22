package game

import (
	"strings"
)

// ROUE vowel purchase (v12.1.0, #147, contracts/roue.md §15.1). Kept out of roue.go: the classic turn is only
// reached through one-line delegations.

// Vowel timing / number / gain values (ROUE_VOWEL_TIMING, ROUE_VOWELS_BEFORE / AFTER, ROUE_VOWEL_GAIN).
const (
	RoueVowelTimingBefore = "BEFORE"
	RoueVowelTimingAfter  = "AFTER"
	RoueVowelTimingBoth   = "BOTH"
	RoueVowelsOne         = "ONE"
	RoueVowelsMany        = "MANY"
	RoueVowelGainNone     = "NONE"
	RoueVowelGainRefund   = "REFUND"
	RoueVowelGainPerOcc   = "PER_OCCURRENCE"

	RoueDefaultVowelPrice = 250
	RoueMaxVowelPrice     = 1_000_000

	RoueErrVowelTimingInvalid  = "ROUE_VOWEL_TIMING_INVALID"
	RoueErrVowelsBeforeInvalid = "ROUE_VOWELS_BEFORE_INVALID"
	RoueErrVowelsAfterInvalid  = "ROUE_VOWELS_AFTER_INVALID"
	RoueErrVowelPriceInvalid   = "ROUE_VOWEL_PRICE_INVALID"
	RoueErrVowelGainInvalid    = "ROUE_VOWEL_GAIN_INVALID"
)

// ValidateRoueVowelFields checks the vowel fields of a ROUE question (empty = default). price 0 = absent.
func ValidateRoueVowelFields(timing, before, after string, price int, priceGiven bool, gain string) error {
	switch timing {
	case "", RoueVowelTimingBefore, RoueVowelTimingAfter, RoueVowelTimingBoth:
	default:
		return &RoueError{RoueErrVowelTimingInvalid, "roue_vowel_timing doit valoir BEFORE, AFTER ou BOTH"}
	}
	switch before {
	case "", RoueVowelsOne, RoueVowelsMany:
	default:
		return &RoueError{RoueErrVowelsBeforeInvalid, "roue_vowels_before doit valoir ONE ou MANY"}
	}
	switch after {
	case "", RoueVowelsOne, RoueVowelsMany:
	default:
		return &RoueError{RoueErrVowelsAfterInvalid, "roue_vowels_after doit valoir ONE ou MANY"}
	}
	if priceGiven && (price < 1 || price > RoueMaxVowelPrice) {
		return &RoueError{RoueErrVowelPriceInvalid, "roue_vowel_price doit être un entier de 1 à 1000000"}
	}
	switch gain {
	case "", RoueVowelGainNone, RoueVowelGainRefund, RoueVowelGainPerOcc:
	default:
		return &RoueError{RoueErrVowelGainInvalid, "roue_vowel_gain doit valoir NONE, REFUND ou PER_OCCURRENCE"}
	}
	return nil
}

// roueAfterVowelPossibleUnsafe: with AFTER / BOTH a vowel can still be bought after the stop (contract §15.1.4,
// « plus aucune consonne »): purchase enabled, a vowel left, and the pot of the team covers the price.
func (e *Engine) roueAfterVowelPossibleUnsafe() bool {
	st := &e.state
	if !st.RoueBuyVowels || !st.RoueVowelsLeft || st.RoueVowelTiming == RoueVowelTimingBefore {
		return false
	}
	return st.RoueTeamBank[st.RoueCurrentTeam] >= st.RoueVowelPrice
}

// roueVowelRevealDoneUnsafe: end of VOWEL_REVEALING → TURN, same team, ROUE_TURN_VOWEL true, no event.
func (e *Engine) roueVowelRevealDoneUnsafe() {
	e.roueSetSubUnsafe(RoueSubTurn)
	e.state.RoueTurnVowel = true
	e.roueRefreshBoardUnsafe()
}

// RoueBuyVowel — ROUE_BUY_VOWEL {LETTER}: the 8 checks of contract §15.1.6, in order; a refusal changes nothing
// and debits nothing.
func (e *Engine) RoueBuyVowel(letter string) error {
	return e.roueAction(func(fx *roueFx) error {
		st := &e.state
		sub := st.RoueSubPhase
		if sub != RoueSubTurn && sub != RoueSubLetter && sub != RoueSubAnswer { // 1
			return roueErr(RoueErrWrongSubPhase, "l'achat d'une voyelle n'est pas possible maintenant")
		}
		if !st.RoueBuyVowels { // 2
			return roueErr(RoueErrVowelsDisabled, "Achat de voyelles désactivé pour cette question")
		}
		timing := st.RoueVowelTiming
		notNow := (sub == RoueSubTurn && timing == RoueVowelTimingAfter) ||
			(sub == RoueSubLetter && timing == RoueVowelTimingBefore) ||
			(sub == RoueSubAnswer && (!st.RoueStopVowel || timing == RoueVowelTimingBefore))
		if notNow { // 3
			return roueErr(RoueErrVowelNotNow, "Pas d'achat de voyelle à ce moment pour cette question")
		}
		if (sub == RoueSubTurn && st.RoueVowelsBefore == RoueVowelsOne && st.RoueTurnVowel) ||
			(sub == RoueSubAnswer && st.RoueVowelsAfter == RoueVowelsOne) { // 4
			return roueErr(RoueErrVowelLimit, "Une seule voyelle à ce moment pour cette question")
		}
		runes := []rune(rafaleFold(strings.TrimSpace(letter))) // 5
		if len(runes) != 1 || runes[0] < 'A' || runes[0] > 'Z' {
			return roueErr(RoueErrNotALetter, "ce n'est pas une lettre")
		}
		l := runes[0]
		if !IsRoueVowel(l) {
			return roueErr(RoueErrNotAVowel, "Ce n'est pas une voyelle")
		}
		ls := string(l)
		for _, p := range st.RoueLetters { // 6
			if p.Letter == ls {
				return roueErr(RoueErrLetterProposed, "Voyelle déjà proposée")
			}
		}
		if !st.RoueVowelsLeft { // 7
			return roueErr(RoueErrNoVowelLeft, "Plus de voyelle à trouver")
		}
		team := st.RoueCurrentTeam
		price := st.RoueVowelPrice
		if st.RoueTeamBank[team] < price { // 8
			return roueErr(RoueErrInsufficientBank, "Cagnotte insuffisante")
		}

		count := 0
		for _, row := range e.roue.rows {
			for _, r := range row {
				if r == l {
					count++
				}
			}
		}
		hit := count > 0
		e.roueBankUnsafe(team, -price)
		st.RoueLetters = append(append([]RoueLetter{}, st.RoueLetters...), RoueLetter{Letter: ls, Team: team, Hit: hit})
		e.roueRefreshConsonantsLeftUnsafe()
		if !hit {
			e.roueEventUnsafe(RoueEvVowelMiss, team, ls, 0, price, 0)
			fx.notices = append(fx.notices, RoueNotice{Kind: RoueNoticeBad, Team: team})
			e.roueEnterTurnEndUnsafe(false, false)
			return nil
		}
		gain := 0
		switch st.RoueVowelGain {
		case RoueVowelGainRefund:
			gain = price
		case RoueVowelGainPerOcc:
			gain = count * price
		}
		e.roue.vowelGain = gain
		e.roue.lit = l
		e.roueRefreshBoardUnsafe()
		if sub == RoueSubTurn {
			e.roueSetSubUnsafe(RoueSubVowelLit)
		} else {
			st.RoueStopVowel = true
			e.roueSetSubUnsafe(RoueSubLit)
		}
		e.roueEventUnsafe(RoueEvVowelHit, team, ls, count, price, gain)
		return nil
	})
}
