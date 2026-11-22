package game

import (
	"fmt"
	"math/rand"
	"sort"
)

// ROUE (v12.0.0, #144, contracts/roue.md §10.1): the wheel is not a typed list
// of segments, it is GENERATED from four parameters (maximum, step, number of
// BANQUEROUTE, number of TOUR PERDU). One single implementation, server side:
// the settings card previews it through POST /api/roue/wheel-preview.

// Segment kinds (ROUE_WHEEL.SEGMENTS[].KIND).
const (
	RoueKindValue    = "value"
	RoueKindBankrupt = "bankrupt"
	RoueKindLoseTurn = "lose_turn"
)

// Modes of the BONUS mark (v12.2.0 #317, contract §17.5).
const (
	RoueBonusEphemeral = "EPHEMERAL" // the mark disappears after its first draw, until the end of the round (default)
	RoueBonusPermanent = "PERMANENT" // the mark stays
)

// ValidRoueBonusMode reports whether m is a BONUS mode ("" is not: it means "absent").
func ValidRoueBonusMode(m string) bool { return m == RoueBonusEphemeral || m == RoueBonusPermanent }

// RoueMaxBonusCount bounds ROUE_BONUS_COUNT (contract §15.2.1).
const RoueMaxBonusCount = 4

// RoueMaxSegments is the limit of the Wheel component.
const RoueMaxSegments = 64

// RoueMaxPointsLimit bounds max_points (a refusal above): the wheel is limited to
// 64 cases anyway, a larger maximum has no use and would make the step search costly.
const RoueMaxPointsLimit = 1_000_000

// RoueSegment is one case of the wheel.
type RoueSegment struct {
	ID    string `json:"ID"`
	Kind  string `json:"KIND"`
	Value int    `json:"VALUE"`
	// Bonus (v12.2.0 #316, contract §17): a points case carrying the BONUS mark. No omitempty (project rule).
	Bonus bool `json:"BONUS"`
}

// RoueWheelParams is the `wheel` section of game-config.json (contract §10).
type RoueWheelParams struct {
	MaxPoints      int
	Step           int
	BankruptCount  int
	LoseTurnCount  int
	BonusCount     int    // BONUS marks laid on points cases (v12.2.0: no longer added cases), 0 to 4
	BonusMode      string // EPHEMERAL (default, "" = EPHEMERAL once normalised) | PERMANENT (v12.2.0 #317)
	SpinPeriodMs   int
	SpinMaxMs      int
	StopDurationMs int
	FlipIntervalMs int
}

// RoueSpinUpMs is THE rule of the climb to full speed (contract roue.md §4): one turn, accelerating at a constant
// rate, so 2 × SPIN_PERIOD_MS (5 s by default). Shared by the START of a round and by wheel-preview (TIMING).
func RoueSpinUpMs(spinPeriodMs int) int { return 2 * spinPeriodMs }

// DefaultRoueWheelParams returns the default wheel: 1000 / 50 / 2 / 1 (22 cases).
func DefaultRoueWheelParams() RoueWheelParams {
	return RoueWheelParams{
		MaxPoints: 1000, Step: 50, BankruptCount: 2, LoseTurnCount: 1,
		BonusMode:    RoueBonusEphemeral,
		SpinPeriodMs: 2500, SpinMaxMs: 10000, StopDurationMs: 4000, FlipIntervalMs: 600,
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// NormalizeRoueWheelParams applies the contract §10 rules: durations out of
// bounds are brought back to the bound (0 = absent = default), the step is
// corrected to the closest divisor of the maximum. It returns the effective
// parameters and, when the step was corrected, the step that was entered
// (adjustedFrom, else 0). An error means the whole section is refused.
func NormalizeRoueWheelParams(p RoueWheelParams) (out RoueWheelParams, adjustedFrom int, err error) {
	def := DefaultRoueWheelParams()
	if p.SpinPeriodMs == 0 {
		p.SpinPeriodMs = def.SpinPeriodMs
	}
	if p.SpinMaxMs == 0 {
		p.SpinMaxMs = def.SpinMaxMs
	}
	if p.StopDurationMs == 0 {
		p.StopDurationMs = def.StopDurationMs
	}
	if p.FlipIntervalMs == 0 {
		p.FlipIntervalMs = def.FlipIntervalMs
	}
	if p.BonusMode == "" {
		p.BonusMode = RoueBonusEphemeral
	}
	if !ValidRoueBonusMode(p.BonusMode) {
		return p, 0, fmt.Errorf("bonus_mode doit valoir %s ou %s", RoueBonusEphemeral, RoueBonusPermanent)
	}
	p.SpinPeriodMs = clampInt(p.SpinPeriodMs, 1000, 6000)
	p.SpinMaxMs = clampInt(p.SpinMaxMs, 3000, 30000)
	p.StopDurationMs = clampInt(p.StopDurationMs, 1500, 8000)
	p.FlipIntervalMs = clampInt(p.FlipIntervalMs, 200, 1500)

	if p.MaxPoints < 2 {
		return p, 0, fmt.Errorf("max_points doit être un entier supérieur ou égal à 2")
	}
	if p.MaxPoints > RoueMaxPointsLimit {
		return p, 0, fmt.Errorf("max_points ne peut pas dépasser %d", RoueMaxPointsLimit)
	}
	if p.Step < 1 {
		return p, 0, fmt.Errorf("step doit être un entier supérieur ou égal à 1")
	}
	if p.Step >= p.MaxPoints {
		return p, 0, fmt.Errorf("step doit être strictement inférieur à max_points")
	}
	if p.BonusCount < 0 || p.BonusCount > RoueMaxBonusCount {
		return p, 0, fmt.Errorf("bonus_count doit être un entier de 0 à %d", RoueMaxBonusCount)
	}
	if p.BankruptCount < 0 || p.LoseTurnCount < 0 {
		return p, 0, fmt.Errorf("bankrupt_count et lose_turn_count ne peuvent pas être négatifs")
	}
	if p.MaxPoints%p.Step != 0 {
		entered := p.Step
		best, bestDist := 1, p.MaxPoints
		consider := func(d int) { // d divides max, d < max
			dist := d - entered
			if dist < 0 {
				dist = -dist
			}
			if dist < bestDist || (dist == bestDist && d < best) { // a tie keeps the smaller divisor
				best, bestDist = d, dist
			}
		}
		for d := 1; d*d <= p.MaxPoints; d++ { // divisors enumerated up to √max
			if p.MaxPoints%d != 0 {
				continue
			}
			consider(d)
			if q := p.MaxPoints / d; q != d && q < p.MaxPoints {
				consider(q)
			}
		}
		p.Step = best
		adjustedFrom = entered
	}
	n := RoueWheelSize(p)
	if n > RoueMaxSegments {
		return p, 0, fmt.Errorf("la roue aurait %d cases, la limite est de %d", n, RoueMaxSegments)
	}
	return p, adjustedFrom, nil
}

// RoueWheelSize is the number of cases of the wheel for parameters whose step divides the maximum. The BONUS marks
// are not cases (v12.2.0, contract §17.2): N = V + B + P.
func RoueWheelSize(p RoueWheelParams) int {
	n := p.MaxPoints/p.Step + p.BankruptCount + p.LoseTurnCount
	if p.LoseTurnCount >= 1 && p.MaxPoints%2 == 0 && (p.MaxPoints/2)%p.Step == 0 {
		n-- // the TOUR PERDU takes the place of the MAX/2 value
	}
	return n
}

// roueIntervals returns the lengths of the intervals of points cases between two consecutive special cases of a base
// wheel (circular: the last case touches the first). A wheel with no special case has no interval (nil).
func roueIntervals(segs []RoueSegment) []int {
	n := len(segs)
	start := -1
	for i, s := range segs {
		if s.Kind != RoueKindValue {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	var out []int
	run := 0
	for k := 1; k <= n; k++ {
		if segs[(start+k)%n].Kind == RoueKindValue {
			run++
			continue
		}
		if run > 0 {
			out = append(out, run)
		}
		run = 0
	}
	return out
}

// RoueMaxBonusFor is the most BONUS marks a wheel can take (contract §17.3), for NORMALISED parameters: the smallest of
// 4, the free room of the intervals (sum of ⌊(g−1)/2⌋; ⌊N/2⌋ without BANQUEROUTE nor PERDU) and the spacing rule
// X ≤ ⌊(V−B−P)/2⌋ (V = number of points cases).
func RoueMaxBonusFor(p RoueWheelParams) int {
	p.BonusCount = 0
	base := buildRoueBase(p)
	n := len(base)
	special := p.BankruptCount + p.LoseTurnCount
	room := 0
	if special == 0 {
		room = n / 2
	} else {
		for _, g := range roueIntervals(base) {
			room += (g - 1) / 2
		}
	}
	spacing := (n - 2*special) / 2
	if spacing < 0 {
		spacing = 0
	}
	m := RoueMaxBonusCount
	if room < m {
		m = room
	}
	if spacing < m {
		m = spacing
	}
	return m
}

// ValidateRoueBonusCount checks ROUE_BONUS_COUNT against the wheel of the current configuration (contract §17.3):
// 0 to 4 and no more than RoueMaxBonusFor. p is normalised here.
func ValidateRoueBonusCount(n int, p RoueWheelParams) error {
	if n < 0 || n > RoueMaxBonusCount {
		return &RoueError{RoueErrBonusCountInvalid, "roue_bonus_count doit être un entier de 0 à 4"}
	}
	np, _, err := NormalizeRoueWheelParams(p)
	if err != nil {
		np, _, _ = NormalizeRoueWheelParams(DefaultRoueWheelParams())
	}
	if max := RoueMaxBonusFor(np); n > max {
		return &RoueError{RoueErrBonusCountInvalid, RoueBonusCountMessage(max)}
	}
	return nil
}

// RoueBonusCountMessage is the refusal message of too many BONUS (contract §17.3).
func RoueBonusCountMessage(max int) string {
	return fmt.Sprintf("Trop de BONUS : %d au plus pour cette roue (un BONUS ne touche jamais une Banqueroute, un Perdu ni un autre BONUS).", max)
}

// RoueEffectiveParams is the composition of the wheel of a question (contract §16.3): each of the 5 composition
// parameters is the question's own value when present, else the global one; the durations are always global. The
// result is NORMALISED. Pure function. fallback is true when the composition stayed invalid even without BONUS marks:
// the whole global wheel is returned. BONUS marks are first reduced to what the wheel can take (the caller can compare
// result.BonusCount with the requested one to log it). The start of a round is never refused.
// v12.2.0 (#316/#317, contract §17): BonusCount is the number of BONUS MARKS (reduced to RoueMaxBonusFor), BonusMode the
// effective mode of the round.
func RoueEffectiveParams(tc TypedContent, global RoueWheelParams) (p RoueWheelParams, fallback bool) {
	g, _, err := NormalizeRoueWheelParams(global)
	if err != nil {
		g, _, _ = NormalizeRoueWheelParams(DefaultRoueWheelParams())
	}
	eff := g
	if tc.RoueWheelMaxPoints != nil {
		eff.MaxPoints = *tc.RoueWheelMaxPoints
	}
	if tc.RoueWheelStep != nil {
		eff.Step = *tc.RoueWheelStep
	}
	if tc.RoueWheelBankruptCount != nil {
		eff.BankruptCount = *tc.RoueWheelBankruptCount
	}
	if tc.RoueWheelLoseTurnCount != nil {
		eff.LoseTurnCount = *tc.RoueWheelLoseTurnCount
	}
	if tc.RoueBonusCount != nil {
		eff.BonusCount = *tc.RoueBonusCount
	}
	if ValidRoueBonusMode(tc.RoueBonusMode) { // present = customised (contract §17.5); an unknown value follows the global one
		eff.BonusMode = tc.RoueBonusMode
	}
	eff.BonusCount = clampInt(eff.BonusCount, 0, RoueMaxBonusCount)
	wanted := eff.BonusCount
	eff.BonusCount = 0
	np, _, nerr := NormalizeRoueWheelParams(eff)
	if nerr != nil {
		g.BonusCount = clampInt(g.BonusCount, 0, RoueMaxBonusFor(g))
		if ValidRoueBonusMode(tc.RoueBonusMode) {
			g.BonusMode = tc.RoueBonusMode
		}
		return g, true
	}
	np.BonusCount = clampInt(wanted, 0, RoueMaxBonusFor(np))
	return np, false
}

// buildRoueBase generates the wheel WITHOUT BONUS marks for NORMALISED parameters (contract §10.1 — the examples there
// are test cases, unchanged since v12.0.0).
func buildRoueBase(p RoueWheelParams) []RoueSegment {
	max, step, B, P := p.MaxPoints, p.Step, p.BankruptCount, p.LoseTurnCount

	// 1. Values, strongest first; the TOUR PERDU takes the place of MAX/2.
	var values []int
	for v := max; v >= step; v -= step {
		if P >= 1 && max%2 == 0 && v == max/2 {
			continue
		}
		values = append(values, v)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(values)))

	// 2. Strong / weak alternation. With a BANQUEROUTE, a "0" placeholder is the
	// weakest value (it is dropped afterwards: the BANQUEROUTE is placed by step 3).
	list := append([]int(nil), values...)
	if B >= 1 {
		list = append(list, 0)
	}
	var seq []int
	for lo, hi, front := 0, len(list)-1, true; lo <= hi; front = !front {
		if front {
			seq = append(seq, list[lo])
			lo++
		} else {
			seq = append(seq, list[hi])
			hi--
		}
	}
	if B >= 1 {
		kept := seq[:0]
		for _, v := range seq {
			if v != 0 {
				kept = append(kept, v)
			}
		}
		seq = kept
	}

	// 3. Special cases.
	K := B + P
	N := len(values) + K
	segs := make([]RoueSegment, N)
	taken := make([]bool, N)
	place := func(idx int, kind string) {
		if idx >= N {
			idx %= N
		}
		for idx == 0 || taken[idx] {
			idx = (idx + 1) % N
		}
		taken[idx] = true
		segs[idx] = RoueSegment{Kind: kind}
	}
	var kinds []string
	remB, remP := B, P
	if remB > 0 {
		kinds = append(kinds, RoueKindBankrupt)
		remB--
	}
	for remB > 0 || remP > 0 {
		if remP > 0 {
			kinds = append(kinds, RoueKindLoseTurn)
			remP--
		}
		if remB > 0 {
			kinds = append(kinds, RoueKindBankrupt)
			remB--
		}
	}
	taken[0] = true // the maximum
	for j, kind := range kinds {
		var idx int
		if B >= 1 {
			if j == 0 {
				idx = 1
			} else {
				idx = j * N / K
			}
		} else {
			idx = (2*j + 1) * N / (2 * K)
		}
		place(idx, kind)
	}
	taken[0] = false

	// 4. Fill the remaining indices with the sequence, from index 0.
	si := 0
	for i := 0; i < N && si < len(seq); i++ {
		if segs[i].Kind == "" {
			segs[i] = RoueSegment{Kind: RoueKindValue, Value: seq[si]}
			si++
		}
	}
	for i := range segs {
		segs[i].ID = fmt.Sprintf("s%d", i)
	}
	return segs
}

// roueCountWays is the number of ways to lay b marks, never side by side, on m free slots in a row: C(m−b+1, b).
func roueCountWays(m, b int) int {
	if b <= 0 {
		return 1
	}
	if m < 0 || m-b+1 < b {
		return 0
	}
	n, k := m-b+1, b
	r := 1
	for i := 1; i <= k; i++ {
		r = r * (n - k + i) / i
	}
	return r
}

// BuildRoueWheel generates the segments of the wheel for NORMALISED parameters (contract §17.2): the cases of §10.1 (no
// case moves when BONUS marks are laid), then p.BonusCount BONUS marks drawn at random on points cases — spread
// homogeneously between the intervals, uniformly inside an interval, never next to a special case nor to each other.
// randN(n) returns an int in [0, n); nil = math/rand. Fewer marks than asked are laid if the wheel cannot take them
// (callers reduce beforehand with RoueMaxBonusFor).
func BuildRoueWheel(p RoueWheelParams, randN func(n int) int) []RoueSegment {
	if randN == nil {
		randN = rand.Intn
	}
	rn := func(n int) int {
		if n <= 1 {
			return 0
		}
		v := randN(n) % n
		if v < 0 { // a test double may answer out of range
			v += n
		}
		return v
	}
	segs := buildRoueBase(p)
	n := len(segs)
	X := p.BonusCount
	if X <= 0 || n == 0 {
		return segs
	}
	// Runs of consecutive points cases (circular), start index of each one.
	type run struct{ start, g, b int }
	var runs []*run
	if p.BankruptCount+p.LoseTurnCount == 0 {
		// Without BANQUEROUTE nor PERDU the first mark is drawn on the whole wheel; it then plays the special case.
		first := rn(n)
		segs[first].Bonus = true
		X--
		if n-1 > 0 {
			runs = append(runs, &run{start: (first + 1) % n, g: n - 1})
		}
	} else {
		first := -1
		for i, s := range segs {
			if s.Kind != RoueKindValue {
				first = i
				break
			}
		}
		cur := 0
		for k := 1; k <= n; k++ {
			if segs[(first+k)%n].Kind == RoueKindValue {
				cur++
				continue
			}
			if cur > 0 {
				runs = append(runs, &run{start: (first + k - cur) % n, g: cur})
			}
			cur = 0
		}
	}
	for ; X > 0; X-- {
		// The non-full interval with the largest g/(b+1); a tie is drawn at random.
		var best []*run
		for _, r := range runs {
			if r.b >= (r.g-1)/2 {
				continue
			}
			if len(best) == 0 {
				best = []*run{r}
				continue
			}
			l, rr := r.g*(best[0].b+1), best[0].g*(r.b+1) // r.g/(r.b+1) against best.g/(best.b+1)
			switch {
			case l > rr:
				best = []*run{r}
			case l == rr:
				best = append(best, r)
			}
		}
		if len(best) == 0 {
			break
		}
		best[rn(len(best))].b++
	}
	for _, r := range runs {
		// r.b marks inside the interval, uniformly among the valid sets (not on its two extreme cases, not side by side).
		m, b := r.g-2, r.b
		for i := 0; i < m && b > 0; i++ {
			left := m - i
			total := roueCountWays(left, b)
			if total <= 0 {
				break
			}
			if rn(total) < roueCountWays(left-2, b-1) {
				segs[(r.start+1+i)%n].Bonus = true
				b--
				i++ // the next slot is skipped
			}
		}
	}
	return segs
}
