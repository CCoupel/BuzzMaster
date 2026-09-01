package game

// rotateTeam computes the next team in a plain circular rotation over
// participatingTeams, starting from currentTeam, plus that team's current
// color (from teamsData) — the shared core of MEMOTION's rotateMotionTeam
// and RAFALE's rotateRafaleTeam (v8.0.0, #199, task 28: "un helper de
// rotation d'équipe partagé avec MEMOTION").
//
// Deliberately the ONLY thing this function knows: given a list and a
// current position, what's next. Per-mode POLICY — whether/when to call
// this at all (MEMOTION's CHACUN_SON_TOUR/TANT_QUE_JE_GAGNE branching in
// DoneMotionCard, RAFALE's 4 modes in advanceRafaleUnsafe) — stays on each
// caller's own side, same separation MEMORY's still-independent
// rotateToNextTeam (engine.go) already has from its own SetMemoryParticipatingTeams
// mode logic. Not folded in here: task 28 scopes this factorization to
// MEMOTION only — MEMORY's rotation is untouched, out of scope for this
// batch, and refactoring it would needlessly widen the non-regression
// surface (MEMORY's LED grid, task 35, already carries its own required
// non-regression suite this batch).
//
// currentTeam not found in participatingTeams (including "" — no team set
// yet) rotates to participatingTeams[0], same as the original MEMOTION/
// MEMORY implementations' "-1 index defaults to 0" behavior — this is what
// makes SetXxxParticipatingTeams's "first team becomes current" bootstrap
// and a genuine mid-round rotation both fall out of ONE code path with no
// special-casing.
//
// Returns ("", nil) if participatingTeams is empty — nothing to rotate to.
func rotateTeam(participatingTeams []string, currentTeam string, teamsData map[string]*Team) (nextTeam string, nextColor []int) {
	if len(participatingTeams) == 0 {
		return "", nil
	}

	currentIndex := -1
	for i, team := range participatingTeams {
		if team == currentTeam {
			currentIndex = i
			break
		}
	}

	nextIndex := (currentIndex + 1) % len(participatingTeams)
	next := participatingTeams[nextIndex]

	var color []int
	if team, exists := teamsData[next]; exists {
		color = team.Color
	}

	return next, color
}
