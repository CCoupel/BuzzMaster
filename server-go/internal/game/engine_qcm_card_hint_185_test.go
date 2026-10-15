// Tests for #185 C-B1 — the QCM hint engine extracted host-agnostic
// (qcmHintShouldTrigger, qcmInvalidateRandomWrongAnswer). The MEMOTION card
// host branch was removed by #279 (LOT-6); the pure functions stay for the
// question host.
// Run: go test ./internal/game/... -run TestQcmHint\|TestProcessMotionCardTick -v

package game

import "testing"

// ============================================================================
// Pure function tests — qcmHintShouldTrigger / qcmInvalidateRandomWrongAnswer
// ============================================================================

func TestQcmHintShouldTrigger(t *testing.T) {
	tests := []struct {
		name             string
		hintsEnabled     bool
		t1Percent        float64
		t2Percent        float64
		currentTime      int
		totalTime        int
		invalidatedCount int
		want             bool
	}{
		{"hints disabled", false, 0.25, 0.125, 7, 30, 0, false},
		{"threshold 1 hit, no invalidations yet", true, 0.25, 0.125, 7, 30, 0, true}, // 30*0.25=7
		{"threshold 1 already consumed (1 invalidated)", true, 0.25, 0.125, 7, 30, 1, false},
		{"threshold 2 hit, exactly 1 invalidated", true, 0.25, 0.125, 3, 30, 1, true}, // 30*0.125=3
		{"threshold 2 hit but 0 invalidated (threshold 1 skipped somehow)", true, 0.25, 0.125, 3, 30, 0, false},
		{"threshold 2 hit but already 2 invalidated", true, 0.25, 0.125, 3, 30, 2, false},
		{"no threshold match", true, 0.25, 0.125, 15, 30, 0, false},
		{"default thresholds when zero", true, 0, 0, 7, 30, 0, true}, // defaults to 0.25/0.125
		{"unsafe thresholds (totalTime too small) disable hints", true, 0.25, 0.125, 0, 2, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qcmHintShouldTrigger(tt.hintsEnabled, tt.t1Percent, tt.t2Percent, tt.currentTime, tt.totalTime, tt.invalidatedCount)
			if got != tt.want {
				t.Errorf("qcmHintShouldTrigger(%v, %v, %v, %d, %d, %d) = %v, want %v",
					tt.hintsEnabled, tt.t1Percent, tt.t2Percent, tt.currentTime, tt.totalTime, tt.invalidatedCount, got, tt.want)
			}
		})
	}
}

func TestQcmInvalidateRandomWrongAnswer(t *testing.T) {
	answers := &QCMAnswers{Red: "a", Green: "b", Yellow: "c", Blue: "d"}

	t.Run("nil answers", func(t *testing.T) {
		color, remaining := qcmInvalidateRandomWrongAnswer(nil, "RED", nil)
		if color != "" || remaining != 0 {
			t.Errorf("got (%q, %d), want (\"\", 0)", color, remaining)
		}
	})

	t.Run("never picks the correct answer", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			color, _ := qcmInvalidateRandomWrongAnswer(answers, "RED", nil)
			if color == "RED" {
				t.Fatalf("qcmInvalidateRandomWrongAnswer picked the correct answer RED")
			}
		}
	})

	t.Run("skips already-invalidated answers", func(t *testing.T) {
		color, remaining := qcmInvalidateRandomWrongAnswer(answers, "RED", []string{"GREEN", "YELLOW"})
		if color != "BLUE" {
			t.Errorf("expected the only remaining wrong answer BLUE, got %q", color)
		}
		if remaining != 1 {
			t.Errorf("remaining = %d, want 1 (4 total - RED correct - 2 already invalidated - 1 just invalidated = 1)", remaining)
		}
	})

	t.Run("all wrong answers already invalidated", func(t *testing.T) {
		color, remaining := qcmInvalidateRandomWrongAnswer(answers, "RED", []string{"GREEN", "YELLOW", "BLUE"})
		if color != "" {
			t.Errorf("expected no color to invalidate, got %q", color)
		}
		if remaining != 1 {
			t.Errorf("remaining = %d, want 1 (RED correct, untouched by exhaustion)", remaining)
		}
	})

	t.Run("does not mutate the input slice", func(t *testing.T) {
		invalidated := []string{"GREEN"}
		_, _ = qcmInvalidateRandomWrongAnswer(answers, "RED", invalidated)
		if len(invalidated) != 1 {
			t.Errorf("input slice was mutated: %v", invalidated)
		}
	})
}

// ============================================================================
// processMotionCardTick — the one authorized host modification of #185's
// batch (contract §10 agnosticity test): branching the QCM hint engine onto
// the MEMOTION card timer, no new ticker.
// ============================================================================

// qcmMotionCard returns a QCM-typed MotionCard with hints enabled and the
// default thresholds, for a `total` seconds timer.
func qcmMotionCard(id string) MotionCard {
	return MotionCard{
		ID: id, RectoTheme: "Capitales", Difficulty: 1, Type: QuestionTypeQCM,
		TypedContent: TypedContent{
			QCMAnswers:      &QCMAnswers{Red: "Paris", Green: "Lyon", Yellow: "Nice", Blue: "Metz"},
			QCMCorrect:      "RED",
			QCMHintsEnabled: true,
			// Explicit thresholds so the test doesn't depend on the 0.25/0.125 defaults changing.
			QCMHintThreshold1: 0.5,
			QCMHintThreshold2: 0.25,
		},
	}
}

// TestProcessMotionCardTick_QCMCard_HintFires was REPLACED by #279 (LOT-6):
// a card has no hints any more — see motion_card_no_hints_lot6_test.go
// (TestProcessMotionCardTick_QCMCard_NeverHints_279).

// TestProcessMotionCardTick_SPEEDYCard_NoQCMHintLogic is the non-regression
// counterpart: a SPEEDY active card must never touch MEMOTION_ACTIVE.STATE
// or fire OnQCMHint, no matter how many ticks elapse.
func TestProcessMotionCardTick_SPEEDYCard_NoQCMHintLogic(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO") // SPEEDY cards
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	hintCalls := 0
	e.OnQCMHint = func(string, int) { hintCalls++ }

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()
	e.state.CurrentTime = 10
	e.state.Delay = 10

	for i := 0; i < 10; i++ {
		if result := e.processMotionCardTick(); result.guardFailed {
			t.Fatalf("processMotionCardTick guard failed unexpectedly")
		}
	}

	if hintCalls != 0 {
		t.Errorf("expected 0 OnQCMHint calls for a SPEEDY card, got %d", hintCalls)
	}
	if _, has := e.GetState().MotionActive.State["QCM_INVALIDATED"]; has {
		t.Errorf("MEMOTION_ACTIVE.STATE must carry no QCM_INVALIDATED for a SPEEDY card, got %v", e.GetState().MotionActive.State)
	}
}

// TestActiveMotionCardUnsafe_NoCardSelected verifies the nil-safety of the
// lookup helper — no MEMOTION round, or a round with nothing selected.
func TestActiveMotionCardUnsafe_NoCardSelected(t *testing.T) {
	e := NewEngine()
	if got := e.activeMotionCardUnsafe(); got != nil {
		t.Errorf("expected nil with no Question at all, got %+v", got)
	}

	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()
	if got := e.activeMotionCardUnsafe(); got != nil {
		t.Errorf("expected nil in GRID (nothing selected), got %+v", got)
	}
}
