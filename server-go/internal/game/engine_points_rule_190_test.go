// Tests for #190 LOT-5 — Go half of the R5 parity cases (contract
// question-types.md §6.2): the named cases below are mirrored one-to-one,
// with the same names, in web/src/utils/motionGrid.pointsRule.190.test.js
// (getMotionCardPointsForRule). Run: go test ./internal/game/... -run TestPointsRuleParity_190 -v

package game

import "testing"

func TestPointsRuleParity_190(t *testing.T) {
	e := NewEngine()
	tests := []struct {
		name       string
		cardType   QuestionType
		difficulty int
		rule       *PointsRule
		units      int
		unitsTotal int
		want       int
	}{
		{"STARS_PRORATA_5points_8pairs/4_pairs", QuestionTypeMemory, 3, &PointsRule{Mode: PointsRuleModeStarsProrata}, 4, 8, 2},
		{"STARS_PRORATA_5points_8pairs/8_pairs", QuestionTypeMemory, 3, &PointsRule{Mode: PointsRuleModeStarsProrata}, 8, 8, 5},
		{"FIXED_10_won", QuestionTypeQCM, 1, &PointsRule{Mode: PointsRuleModeFixed, Value: 10}, 1, 0, 10},
		{"FIXED_10_lost", QuestionTypeQCM, 1, &PointsRule{Mode: PointsRuleModeFixed, Value: 10}, 0, 0, 0},
		{"PER_UNIT_2x3", QuestionTypeMemory, 3, &PointsRule{Mode: PointsRuleModePerUnit, Value: 2}, 3, 8, 6},
		{"DEFAULT_SPEEDY_is_STARS", QuestionTypeSpeedy, 2, nil, 1, 0, 3},
		{"DEFAULT_MEMORY_is_STARS_PRORATA", QuestionTypeMemory, 3, nil, 4, 8, 2},
		{"DEFAULT_RAFALE_is_STARS_PRORATA", QuestionTypeRafale, 3, nil, 3, 5, 3},
		{"EXPLICIT_STARS_on_MEMORY_full_value", QuestionTypeMemory, 3, &PointsRule{Mode: PointsRuleModeStars}, 1, 8, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &MotionCard{ID: "mc-1", Type: tt.cardType, Difficulty: tt.difficulty, PointsRule: tt.rule}
			if got := e.motionCardPointsForOutcome(card, tt.units, tt.unitsTotal); got != tt.want {
				t.Errorf("motionCardPointsForOutcome(units=%d, total=%d) = %d, want %d", tt.units, tt.unitsTotal, got, tt.want)
			}
		})
	}
}
