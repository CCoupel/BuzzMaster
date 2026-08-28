// Tests for #184 B-B5 — POINTS_RULE (STARS/FIXED/PER_UNIT) and
// MEMOTION_DONE.UNITS. Run: go test ./internal/game/... -run TestMotionCardPointsForOutcome\|TestDoneMotionCard_PointsRule -v

package game

import "testing"

// TestMotionCardPointsForOutcome_Stars verifies the default/explicit-STARS
// path is byte-for-byte the pre-#184 star-based scale — no PointsRule at
// all, and an explicit {"MODE":"STARS"} with no VALUE, both fall through
// to motionCardPoints/difficulty.
func TestMotionCardPointsForOutcome_Stars(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	e.Ready("mq1", q)

	tests := []struct {
		name       string
		pointsRule *PointsRule
		difficulty int
		wantPts    int
	}{
		{"nil PointsRule, difficulty 1", nil, 1, 1},
		{"nil PointsRule, difficulty 2", nil, 2, 3},
		{"nil PointsRule, difficulty 3", nil, 3, 5},
		{"explicit STARS, no VALUE", &PointsRule{Mode: PointsRuleModeStars}, 2, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &MotionCard{ID: "mc-1", Difficulty: tt.difficulty, PointsRule: tt.pointsRule}
			if got := e.motionCardPointsForOutcome(card, 1, 0); got != tt.wantPts {
				t.Errorf("motionCardPointsForOutcome() = %d, want %d", got, tt.wantPts)
			}
		})
	}
}

// TestMotionCardPointsForOutcome_Fixed verifies contract §6.2: VALUE if
// Units > 0, else 0 — independent of DIFFICULTY.
func TestMotionCardPointsForOutcome_Fixed(t *testing.T) {
	e := NewEngine()
	card := &MotionCard{ID: "mc-1", Difficulty: 3, PointsRule: &PointsRule{Mode: PointsRuleModeFixed, Value: 10}}

	if got := e.motionCardPointsForOutcome(card, 1, 0); got != 10 {
		t.Errorf("FIXED, units=1: got %d, want 10", got)
	}
	if got := e.motionCardPointsForOutcome(card, 5, 0); got != 10 {
		t.Errorf("FIXED, units=5: got %d, want 10 (FIXED ignores unit count beyond >0)", got)
	}
	if got := e.motionCardPointsForOutcome(card, 0, 0); got != 0 {
		t.Errorf("FIXED, units=0: got %d, want 0", got)
	}
}

// TestMotionCardPointsForOutcome_PerUnit verifies contract §6.2: VALUE × Units.
func TestMotionCardPointsForOutcome_PerUnit(t *testing.T) {
	e := NewEngine()
	card := &MotionCard{ID: "mc-1", PointsRule: &PointsRule{Mode: PointsRuleModePerUnit, Value: 4}}

	tests := []struct {
		units   int
		wantPts int
	}{
		{0, 0},
		{1, 4},
		{3, 12},
	}
	for _, tt := range tests {
		if got := e.motionCardPointsForOutcome(card, tt.units, 0); got != tt.wantPts {
			t.Errorf("PER_UNIT, units=%d: got %d, want %d", tt.units, got, tt.wantPts)
		}
	}
}

// TestMotionCardPointsForOutcome_StarsProrata verifies contract §6.2's
// STARS_PRORATA mode — multiply BEFORE dividing (normative order), with
// "5 points / 8 pairs" as the mandatory named case (contract §6.2, §10.1):
// it is the one case that catches an order-of-operations bug on both sides,
// since a naive "value per unit first" computation truncates every result
// to 0 for this exact ratio.
func TestMotionCardPointsForOutcome_StarsProrata(t *testing.T) {
	e := NewEngine()
	// Difficulty 1 → 5 points via a custom MotionConfig (not the default
	// 1/3/5 scale) so the "5 points / 8 pairs" case is exercisable without
	// inventing a difficulty level that doesn't exist.
	q := makeMotionQuestion("mq1", nil, "SOLO")
	q.MotionConfig = &MotionConfig{Points1Star: 5}
	e.Ready("mq1", q)
	card := &MotionCard{ID: "mc-1", Difficulty: 1, PointsRule: &PointsRule{Mode: PointsRuleModeStarsProrata}}

	tests := []struct {
		name       string
		units      int
		unitsTotal int
		wantPts    int
	}{
		{"5 points / 8 pairs — 4 found (naive per-unit-first would give 0)", 4, 8, 2},
		{"5 points / 8 pairs — complete grid rewards the exact nominal value", 8, 8, 5},
		{"0 found", 0, 8, 0},
		{"unitsTotal<=0 guards against division by zero", 3, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := e.motionCardPointsForOutcome(card, tt.units, tt.unitsTotal); got != tt.wantPts {
				t.Errorf("STARS_PRORATA, units=%d/%d: got %d, want %d", tt.units, tt.unitsTotal, got, tt.wantPts)
			}
		})
	}
}

// TestMotionCardPointsForOutcome_StarsProrata_ValueIgnored verifies VALUE
// is irrelevant under STARS_PRORATA — contract §6.2: "ignorée — aucune à
// saisir". A non-zero VALUE left over from switching POINTS_RULE.MODE must
// not leak into the prorata computation.
func TestMotionCardPointsForOutcome_StarsProrata_ValueIgnored(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", nil, "SOLO")
	q.MotionConfig = &MotionConfig{Points1Star: 10}
	e.Ready("mq1", q)
	card := &MotionCard{ID: "mc-1", Difficulty: 1, PointsRule: &PointsRule{Mode: PointsRuleModeStarsProrata, Value: 999}}

	if got := e.motionCardPointsForOutcome(card, 1, 2); got != 5 {
		t.Errorf("STARS_PRORATA with leftover VALUE=999: got %d, want 5 (10×1/2), VALUE must be ignored", got)
	}
}

// TestMotionCardPointsForOutcome_Memory_DefaultsToStarsProrataWithoutExplicitRule
// is the empirical case code-reviewer flagged (code-review-20260825-214659,
// plan-memotion-v710-memory-pointsrule-20260825-215050): a MEMORY card with
// NO PointsRule at all must still be scored STARS_PRORATA, resolved from
// the type's registry default (#187 cycle 5) — a 5-point card with 1/4
// pairs found awards 1 point, not the full 5 a bare STARS fallback would
// have given before this fix.
func TestMotionCardPointsForOutcome_Memory_DefaultsToStarsProrataWithoutExplicitRule(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", nil, "SOLO")
	q.MotionConfig = &MotionConfig{Points1Star: 5}
	e.Ready("mq1", q)
	card := &MotionCard{ID: "mc-1", Type: QuestionTypeMemory, Difficulty: 1} // no PointsRule at all

	if got := e.motionCardPointsForOutcome(card, 1, 4); got != 1 {
		t.Errorf("MEMORY card without PointsRule, 1/4 pairs: got %d, want 1 (STARS_PRORATA registry default, not 5 from a bare STARS fallback)", got)
	}
	if got := e.motionCardPointsForOutcome(card, 4, 4); got != 5 {
		t.Errorf("MEMORY card without PointsRule, 4/4 pairs: got %d, want 5 (complete grid = nominal value)", got)
	}
}

// TestMotionCardPointsForOutcome_Memory_ExplicitOverrideWinsOverRegistryDefault
// verifies an explicit PointsRule still takes absolute priority over the
// type's registry default — contract §6.3's tout-ou-rien override remains
// available on a MEMORY card.
func TestMotionCardPointsForOutcome_Memory_ExplicitOverrideWinsOverRegistryDefault(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", nil, "SOLO") // difficulty 2 → 3pts, default 1/3/5 scale
	e.Ready("mq1", q)
	card := &MotionCard{ID: "mc-1", Type: QuestionTypeMemory, Difficulty: 2, PointsRule: &PointsRule{Mode: PointsRuleModeStars}}

	// STARS ignores units entirely — must award the card's full star value,
	// NOT the prorated fraction the registry default would have computed.
	if got := e.motionCardPointsForOutcome(card, 1, 4); got != 3 {
		t.Errorf("MEMORY card with explicit PointsRule STARS: got %d, want 3 (full stars — explicit override wins, registry default must not apply)", got)
	}
}

// TestMotionCardPointsForOutcome_NonMemoryTypes_StillDefaultToStars is the
// explicit non-regression companion at the motionCardPointsForOutcome
// level (question_types_test.go covers the registry fact itself):
// SPEEDY/QCM/no-TYPE without PointsRule are unaffected by #187 cycle 5.
func TestMotionCardPointsForOutcome_NonMemoryTypes_StillDefaultToStars(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	e.Ready("mq1", q)

	tests := []struct {
		name     string
		cardType QuestionType
	}{
		{"SPEEDY", QuestionTypeSpeedy},
		{"QCM", QuestionTypeQCM},
		{"absent TYPE (defaults to SPEEDY)", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &MotionCard{ID: "mc-1", Type: tt.cardType, Difficulty: 1}
			if got := e.motionCardPointsForOutcome(card, 1, 4); got != 1 {
				t.Errorf("%s card without PointsRule: got %d, want 1 (STARS/difficulty-1, unaffected by #187 cycle 5)", tt.name, got)
			}
		})
	}
}

// TestMotionCardPointsForOutcome_TypeSwitchedFromMemoryToSpeedy_ReResolvesDefault
// is the anti-trap test named explicitly by team-lead, from the cycle 5
// plan §2.1: a card that carried TYPE=MEMORY (registry default resolves to
// STARS_PRORATA) and is later switched to SPEEDY (pairs cleared, contract
// §3.2's type-lock releases) must score its FULL star value on the very
// next call — never 0. Mutating the SAME *MotionCard across two calls
// proves the registry lookup is re-resolved from card.EffectiveType()
// every time, never cached from an earlier resolution — exactly the
// property that makes the registry approach immune to the trap a
// card-written default (rejected alternatives (a)/(b) in the plan) would
// have fallen into.
func TestMotionCardPointsForOutcome_TypeSwitchedFromMemoryToSpeedy_ReResolvesDefault(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", nil, "SOLO")
	q.MotionConfig = &MotionConfig{Points1Star: 5}
	e.Ready("mq1", q)

	card := &MotionCard{ID: "mc-1", Type: QuestionTypeMemory, Difficulty: 1} // no explicit PointsRule
	if got := e.motionCardPointsForOutcome(card, 1, 4); got != 1 {
		t.Fatalf("setup invalide : carte MEMORY, 1/4 paires : got %d, want 1 (défaut STARS_PRORATA)", got)
	}

	// The card's pairs are cleared and its TYPE switched to SPEEDY — the
	// §3.2 unlock-then-switch sequence the plan's §2.1 trap describes. No
	// PointsRule is ever written by anyone (that's the whole point of the
	// registry approach) — only card.Type/TypedContent change, exactly as
	// a real save would leave it.
	card.Type = QuestionTypeSpeedy
	card.TypedContent = TypedContent{}

	if got := e.motionCardPointsForOutcome(card, 1, 0); got != 5 {
		t.Errorf("same card after switching TYPE MEMORY→SPEEDY: got %d, want 5 (full stars) — a stale STARS_PRORATA default must never survive the switch", got)
	}
}

// TestDoneMotionCard_PointsRule_PerUnit is the end-to-end version through
// DoneMotionCard: a card with POINTS_RULE PER_UNIT awards VALUE×units to
// the winning team's score.
func TestDoneMotionCard_PointsRule_PerUnit(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	cards := []MotionCard{
		{ID: "mc-1", RectoTheme: "T", Difficulty: 1, PointsRule: &PointsRule{Mode: PointsRuleModePerUnit, Value: 2}},
	}
	q := makeMotionQuestion("mq1", cards, "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()

	points, _, err := e.DoneMotionCard("mc-1", "red", 3)
	if err != nil {
		t.Fatalf("DoneMotionCard failed: %v", err)
	}
	if points != 6 {
		t.Errorf("points awarded = %d, want 6 (VALUE=2 × units=3)", points)
	}

	team := e.GetTeamsAndBumpers().Teams["red"]
	if team.TeamPoints != 6 {
		t.Errorf("team.TeamPoints = %d, want 6", team.TeamPoints)
	}
}

// TestDoneMotionCard_UnitsIgnoredUnderStars verifies the default STARS
// scale is completely unaffected by whatever units value the caller passes
// — the non-regression guarantee for every pre-#184 MEMOTION_DONE call
// (which never sent UNITS, and now always resolves to a passed-in units=1,
// but the guarantee must hold for any units value since STARS never reads it).
func TestDoneMotionCard_UnitsIgnoredUnderStars(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO") // no PointsRule ⇒ STARS
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1") // difficulty 1 → 1pt under STARS
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()

	points, _, err := e.DoneMotionCard("mc-1", "red", 99)
	if err != nil {
		t.Fatalf("DoneMotionCard failed: %v", err)
	}
	if points != 1 {
		t.Errorf("points awarded = %d, want 1 (STARS/difficulty-1, units must be ignored)", points)
	}
}

// TestMotionDonePayload_UnitsAbsentVsExplicitZero verifies protocol.
// MotionDonePayload's *int Units field round-trips the absent/explicit-0
// distinction contract §9.3 requires ("absent ⇒ 1 ⇒ comportement actuel").
// This lives in package game (not protocol) because it exercises the value
// through motionCardPointsForOutcome's actual consumer semantics, not just
// JSON shape — see protocol package's own messages_test.go for the pure
// marshal/unmarshal shape check.
func TestMotionDonePayload_UnitsAbsentVsExplicitZero(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})
	cards := []MotionCard{
		{ID: "mc-1", RectoTheme: "T", PointsRule: &PointsRule{Mode: PointsRuleModeFixed, Value: 7}},
	}
	q := makeMotionQuestion("mq1", cards, "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	_ = e.RevealMotionCard()

	// Simulates main.go's handleMotionDone resolving an explicit UNITS=0
	// (not absent) — FIXED must award 0, not the "absent ⇒ 1" default.
	points, _, err := e.DoneMotionCard("mc-1", "red", 0)
	if err != nil {
		t.Fatalf("DoneMotionCard failed: %v", err)
	}
	if points != 0 {
		t.Errorf("points awarded = %d, want 0 (FIXED with explicit units=0)", points)
	}
}
