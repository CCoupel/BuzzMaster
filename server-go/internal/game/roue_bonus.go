package game

// ROUE BONUS case (v12.1.0, #308, contracts/roue.md §15.2.3). Kept out of roue.go: the classic turn is only
// reached through a one-line delegation.

// roueBonusStopUnsafe applies the end of the braking on a BONUS case: no gain, no letter, no pot or score change; the
// team keeps the hand and starts a new turn (step 1). The event stays the last one (banner « BONUS ! ») until the
// next gesture.
func (e *Engine) roueBonusStopUnsafe(team string, fx *roueFx) {
	e.roueEnterTurnUnsafe() // new turn, same team: ROUE_TURN_GAIN 0, board refreshed
	e.roueEventUnsafe(RoueEvBonus, team, "", 0, 0, 0)
	fx.notices = append(fx.notices, RoueNotice{Kind: RoueNoticeBonus, Team: team})
}
