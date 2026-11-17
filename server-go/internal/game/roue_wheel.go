package game

import (
	"fmt"
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
}

// RoueWheelParams is the `wheel` section of game-config.json (contract §10).
type RoueWheelParams struct {
	MaxPoints      int
	Step           int
	BankruptCount  int
	LoseTurnCount  int
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
	n := p.MaxPoints/p.Step + p.BankruptCount + p.LoseTurnCount
	if p.LoseTurnCount >= 1 && p.MaxPoints%2 == 0 && (p.MaxPoints/2)%p.Step == 0 {
		n-- // the TOUR PERDU takes the place of the MAX/2 value
	}
	if n > RoueMaxSegments {
		return p, 0, fmt.Errorf("la roue aurait %d cases, la limite est de %d", n, RoueMaxSegments)
	}
	return p, adjustedFrom, nil
}

// BuildRoueWheel generates the segments of the wheel for NORMALISED
// parameters (contract §10.1 — the examples there are test cases).
func BuildRoueWheel(p RoueWheelParams) []RoueSegment {
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
