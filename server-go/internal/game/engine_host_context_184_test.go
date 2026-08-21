// Tests for #184 B-B3 — GetHostContext, the single Go-side implementation
// of contracts/question-types.md §4's derivation table. Test names below
// are the shared naming convention with the JS side (utils/hostContext.js,
// hostContext.test.js, B-F1) — contract §4/§10.3 requires both
// implementations to name their cases identically so a mismatch is visible
// from either test file alone. Keep names in sync if either side changes.
//
// Run: go test ./internal/game/... -run TestGetHostContext -v

package game

import (
	"testing"
	"time"
)

// TestGetHostContext_QuestionHost covers every non-MEMOTION-question PHASE
// value against contract §4's "Question" row.
func TestGetHostContext_QuestionHost(t *testing.T) {
	tests := []struct {
		name string
		q    *Question
		want HostContext
	}{
		{
			name: "Question_Started_Playable",
			q:    &Question{ID: "q1", Type: QuestionTypeSpeedy},
			want: HostContext{Playable: true, Revealed: false, TimerRunning: true, CardID: ""},
		},
		{
			name: "Question_Revealed_Revealed",
			q:    &Question{ID: "q1", Type: QuestionTypeSpeedy},
			want: HostContext{Playable: false, Revealed: true, TimerRunning: false, CardID: ""},
		},
		{
			name: "Question_Prepare_None",
			q:    &Question{ID: "q1", Type: QuestionTypeSpeedy},
			want: HostContext{Playable: false, Revealed: false, TimerRunning: false, CardID: ""},
		},
		{
			name: "Question_Stopped_None",
			q:    nil,
			want: HostContext{Playable: false, Revealed: false, TimerRunning: false, CardID: ""},
		},
	}

	phaseFor := map[string]GamePhase{
		"Question_Started_Playable":  PhaseStarted,
		"Question_Revealed_Revealed": PhaseRevealed,
		"Question_Prepare_None":      PhasePrepare,
		"Question_Stopped_None":      PhaseStopped,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine()
			e.state.Question = tt.q
			e.state.Phase = phaseFor[tt.name]
			got := e.GetHostContext()
			if got != tt.want {
				t.Errorf("GetHostContext() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestGetHostContext_MotionCardHost covers a MEMOTION question across every
// MEMOTION_SUBPHASE value against contract §4's "Carte MEMOTION" and
// "Aucun" rows.
func TestGetHostContext_MotionCardHost(t *testing.T) {
	tests := []struct {
		name         string
		subphase     MotionSubPhase
		timerRunning bool // whether e.timer is set when this subphase is checked
		want         HostContext
	}{
		{
			name:         "MotionCard_Question_Playable_TimerRunning",
			subphase:     MotionSubPhaseQuestion,
			timerRunning: true,
			want:         HostContext{Playable: true, Revealed: false, TimerRunning: true, CardID: "mc-1"},
		},
		{
			name:         "MotionCard_Question_Playable_TimerStopped",
			subphase:     MotionSubPhaseQuestion,
			timerRunning: false,
			want:         HostContext{Playable: true, Revealed: false, TimerRunning: false, CardID: "mc-1"},
		},
		{
			name:     "MotionCard_Reveal_Revealed",
			subphase: MotionSubPhaseReveal,
			want:     HostContext{Playable: false, Revealed: true, TimerRunning: false, CardID: "mc-1"},
		},
		{
			name:     "MotionCard_Grid_None",
			subphase: MotionSubPhaseGrid,
			want:     HostContext{},
		},
		{
			name:     "MotionCard_Memorize_None",
			subphase: MotionSubPhaseMemorize,
			want:     HostContext{},
		},
		{
			name:     "MotionCard_Selected_None",
			subphase: MotionSubPhaseSelected,
			want:     HostContext{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine()
			e.state.Question = &Question{ID: "mq1", Type: QuestionTypeMemotion}
			e.state.Phase = PhaseStarted
			e.state.MotionSubPhase = tt.subphase
			e.state.MotionSelected = "mc-1"
			if tt.timerRunning {
				// A real, never-firing-in-this-test-run ticker: GetHostContext
				// only checks e.timer != nil (see hostContextUnsafe's doc
				// comment), it never waits on the channel.
				e.timer = time.NewTicker(time.Hour)
				defer e.timer.Stop()
			}

			got := e.GetHostContext()
			if got != tt.want {
				t.Errorf("GetHostContext() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestGetHostContext_MotionCardHost_SelectedCardIDIgnoredWhenNoHostActive
// pins down the one documented ambiguity in contract §4 (the "Aucun" row's
// "selon le cas" CardID column): even though a card IS selected during
// MotionSubPhaseSelected (MotionSelected is non-empty), HostContext.CardID
// is "" — resolved to the simplest consistent choice since nothing reads
// typed content through it while Playable/Revealed are both false. See
// hostContextUnsafe's doc comment for the reasoning.
func TestGetHostContext_MotionCardHost_SelectedCardIDIgnoredWhenNoHostActive(t *testing.T) {
	e := NewEngine()
	e.state.Question = &Question{ID: "mq1", Type: QuestionTypeMemotion}
	e.state.Phase = PhaseStarted
	e.state.MotionSubPhase = MotionSubPhaseSelected
	e.state.MotionSelected = "mc-1"

	got := e.GetHostContext()
	if got.CardID != "" {
		t.Errorf("GetHostContext().CardID = %q during SELECTED, want \"\" per the Aucun row", got.CardID)
	}
}
