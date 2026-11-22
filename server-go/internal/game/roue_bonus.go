package game

// ROUE BONUS mark (v12.2.0, #316/#317, contracts/roue.md §17). Kept out of roue.go: the classic turn is only reached
// through small delegations.

// roueBonusStopUnsafe applies the end of the braking on a points case carrying the BONUS mark: a normal turn on the value
// of the case (sub-phase LETTER, consonant or vowel after the stop), NO replay, no point for the BONUS itself. The
// event BONUS {TEAM, VALUE} replaces LAND_VALUE and stays the last one (banner) until the letter is chosen.
func (e *Engine) roueBonusStopUnsafe(team string, value int, fx *roueFx) {
	e.roueEventUnsafe(RoueEvBonus, team, "", 0, value, 0)
	e.roueSetSubUnsafe(RoueSubLetter)
	fx.notices = append(fx.notices, RoueNotice{Kind: RoueNoticeBonus, Team: team})
}

// roueConsumeBonusUnsafe is called by the first ACCEPTED action that leaves the LETTER sub-phase (ROUE_LETTER, ROUE_BUY_VOWEL):
// in EPHEMERAL mode the mark of the case the wheel stopped on is removed for the rest of the round (contract §17.5). The
// segments are replaced, never mutated in place (a snapshot of the state may share the slice).
func (e *Engine) roueConsumeBonusUnsafe() {
	st := &e.state
	w := &st.RoueWheel
	if st.RoueBonusMode == RoueBonusPermanent || w.Current < 0 || w.Current >= len(w.Segments) || !w.Segments[w.Current].Bonus {
		return
	}
	segs := append([]RoueSegment(nil), w.Segments...)
	segs[w.Current].Bonus = false
	w.Segments = segs
}
