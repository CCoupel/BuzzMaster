package game

import "sort"

// recordRafaleTeamCategoryCorrectUnsafe increments the per-team, per-category
// tally of correct answers for a CLASSIC RAFALE round (v9.0.0, Lot A+1, plan
// `_work/reports/plan-v900-correctifs-qualif-20260906-104500.md` §2.3) —
// the memory RafaleCategoryBreakdown needs, at round-end point attribution,
// to split a team's awarded points across the categories that actually
// produced them instead of writing an empty category into history.json (the
// "Inconnue" defect this lot fixes).
//
// Called from advanceRafaleUnsafe's per-mode switch, at the EXACT same call
// sites as RafaleTeamCounters[team]++ (never more, never fewer) — so that
// summing RafaleTeamCategoryCounters[team] across categories always equals
// RafaleTeamCounters[team]. Deliberately NOT called from
// advanceRafaleCardUnsafe (#217's card-hosted mini-round): a RAFALE card is
// scored via STARS_PRORATA/MEMOTION_DONE, a completely different path with
// its own, already-correct, single-category GameEvent (the host MEMOTION
// question's own Category) — out of this lot's scope (handoff
// `_work/handoff/task-dev-backend-20260906-113000.md`, "Lot A+1").
//
// Caller must hold e.mu (write lock). No-op for an empty team or category
// (a solo round with no team concept in play, or a question whose category
// somehow wasn't set — defensive, mirrors the `team != ""` guards already
// present at every RafaleTeamCounters[team]++ call site).
func (e *Engine) recordRafaleTeamCategoryCorrectUnsafe(team, category string) {
	if team == "" || category == "" {
		return
	}
	if e.state.RafaleTeamCategoryCounters[team] == nil {
		e.state.RafaleTeamCategoryCounters[team] = map[string]int{}
	}
	e.state.RafaleTeamCategoryCounters[team][category]++
}

// resetRafaleTeamCategoryCounterUnsafe clears a single team's per-category
// tally — the category-scoped mirror of MAILLON_FAIBLE's own
// `RafaleTeamCounters[team] = 0` reset (an incorrect answer wipes the team's
// running streak-based score, so its category breakdown must be wiped in
// lockstep, preserving the sum-equals-total invariant documented on
// recordRafaleTeamCategoryCorrectUnsafe above). Caller must hold e.mu.
func (e *Engine) resetRafaleTeamCategoryCounterUnsafe(team string) {
	if team == "" {
		return
	}
	e.state.RafaleTeamCategoryCounters[team] = map[string]int{}
}

// RafaleCategoryBreakdown computes the plan §2.4 points-per-category split
// for `points` awarded to a team on a classic RAFALE round
// (question.Type==RAFALE) — the value cmd/server/main.go's
// handleBumperPoints/handleTeamPoints attach to GameEvent.CategoryBreakdown.
//
// Returns nil (⇒ GameEvent.CategoryBreakdown stays absent, omitempty) for:
//   - a nil or non-RAFALE question — every other question type is
//     completely unaffected (non-regression, plan §2.3 point 3);
//   - points <= 0 — a penalty or a no-op credit isn't a "gain" to break
//     down; the event still records normally, just without a breakdown
//     (out of the plan's specified scope, and a negative dividend would
//     make the largest-remainder method's own tie-breaking arithmetic
//     ill-defined);
//   - a round with no configured category at all (defensive — should not
//     happen for a valid RAFALE round, contract §7.5's own launch guard).
//
// `teamCategoryCounters` is the credited team's own slice of
// GameState.RafaleTeamCategoryCounters (nil-safe: a team with no recorded
// category yet ranges as empty). When it's empty (plan §2.4's "aucune
// bonne réponse enregistrée" — typically a manual admin credit with no
// RAFALE_VALIDATE ever recorded for this team), this function falls back to
// splitting `points` in EQUAL shares across the round's EFFECTIVE
// categories (question.EffectiveRafaleCategories()) — same largest-remainder
// method, deterministic, never an empty category.
func RafaleCategoryBreakdown(question *Question, points int, teamCategoryCounters map[string]int) map[string]int {
	if question == nil || question.Type != QuestionTypeRafale || points <= 0 {
		return nil
	}

	weights := teamCategoryCounters
	hasWeight := false
	for _, n := range weights {
		if n > 0 {
			hasWeight = true
			break
		}
	}
	if !hasWeight {
		effective := question.EffectiveRafaleCategories()
		if len(effective) == 0 {
			return nil
		}
		weights = make(map[string]int, len(effective))
		for _, cat := range effective {
			weights[cat] = 1
		}
	}

	return largestRemainderSplit(points, weights)
}

// largestRemainderSplit implements the "méthode du plus fort reste" (plan
// §2.4): each key's base share is `points × weight ÷ total` — multiplication
// BEFORE division, same discipline as STARS_PRORATA (contract
// question-types.md §6.2) — then the leftover units (points minus the sum of
// every base share) go one each to the keys with the largest remainders,
// highest first, ties broken by key name for determinism. The returned
// map's values always sum to EXACTLY `points` — never an invented or lost
// unit — and never contains a zero-valued entry (a category that received
// none of this specific award is simply absent, not listed at 0).
//
// Returns nil if `weights` is empty or every weight is <= 0 (nothing to
// apportion against) — callers already guard this (RafaleCategoryBreakdown
// never calls in with an all-zero/empty weights map), kept here too as a
// defensive, self-contained precondition.
func largestRemainderSplit(points int, weights map[string]int) map[string]int {
	total := 0
	for _, w := range weights {
		total += w
	}
	if total <= 0 {
		return nil
	}

	type share struct {
		key       string
		base      int
		remainder int
	}
	shares := make([]share, 0, len(weights))
	assigned := 0
	for key, weight := range weights {
		if weight <= 0 {
			continue
		}
		product := points * weight
		base := product / total
		remainder := product % total
		shares = append(shares, share{key: key, base: base, remainder: remainder})
		assigned += base
	}
	remaining := points - assigned

	sort.Slice(shares, func(i, j int) bool {
		if shares[i].remainder != shares[j].remainder {
			return shares[i].remainder > shares[j].remainder
		}
		return shares[i].key < shares[j].key // deterministic tie-break
	})

	result := make(map[string]int, len(shares))
	for i, s := range shares {
		v := s.base
		if i < remaining {
			v++
		}
		if v > 0 {
			result[s.key] = v
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
