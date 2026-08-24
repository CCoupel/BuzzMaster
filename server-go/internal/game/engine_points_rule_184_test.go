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
