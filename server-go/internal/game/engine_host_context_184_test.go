// Tests for #184 B-B3 — GetHostContext, the single Go-side implementation
// of contracts/question-types.md §4's derivation table. Test names below
// are the shared naming convention with the JS side (utils/hostContext.js,
// hostContext.test.js, B-F1) — contract §4/§10.3 requires both
// implementations to name their cases identically so a mismatch is visible
// from either test file alone. Keep names in sync if either side changes.
//
// Run: go test ./internal/game/... -run TestGetHostContext -v

package game

import "testing"

// TestGetHostContext_QuestionHost covers every non-MEMOTION-question PHASE
// value against contract §4's "Question" row.
//
// TimerRunning keys on CurrentTime, not PHASE alone (correction, planner
// ruling on top of B-B3/B-B9): e.timer is a private *time.Ticker, never
// serialized — structurally impossible to replicate on the JS side, which
// only ever sees gameState.timer (CurrentTime) over the wire. CurrentTime
// is the only basis both implementations can actually share, so
// PHASE==STARTED alone doesn't distinguish a running countdown from one
// that already reached 0 — see "Question_Started_TimerStopped_CurrentTimeZero".
func TestGetHostContext_QuestionHost(t *testing.T) {
	tests := []struct {
		name        string
		q           *Question
		phase       GamePhase
		currentTime int
		want        HostContext
	}{
		{
			name:        "Question_Started_Playable_TimerRunning",
			q:           &Question{ID: "q1", Type: QuestionTypeSpeedy},
			phase:       PhaseStarted,
			currentTime: 30,
			want:        HostContext{Playable: true, Revealed: false, TimerRunning: true, CardID: ""},
		},
		{
			// The third case the TimerRunning correction requires by name:
			// PHASE==STARTED with CURRENT_TIME==0 (timer already expired,
			// or never started) must NOT report TimerRunning==true.
			name:        "Question_Started_TimerStopped_CurrentTimeZero",
			q:           &Question{ID: "q1", Type: QuestionTypeSpeedy},
			phase:       PhaseStarted,
			currentTime: 0,
			want:        HostContext{Playable: true, Revealed: false, TimerRunning: false, CardID: ""},
		},
		{
			name:  "Question_Revealed_Revealed",
			q:     &Question{ID: "q1", Type: QuestionTypeSpeedy},
			phase: PhaseRevealed,
			want:  HostContext{Playable: false, Revealed: true, TimerRunning: false, CardID: ""},
		},
		{
			name:  "Question_Prepare_None",
			q:     &Question{ID: "q1", Type: QuestionTypeSpeedy},
			phase: PhasePrepare,
			want:  HostContext{Playable: false, Revealed: false, TimerRunning: false, CardID: ""},
		},
		{
			name:  "Question_Stopped_None",
			q:     nil,
			phase: PhaseStopped,
			want:  HostContext{Playable: false, Revealed: false, TimerRunning: false, CardID: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine()
			e.state.Question = tt.q
			e.state.Phase = tt.phase
			e.state.CurrentTime = tt.currentTime
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
//
// TimerRunning keys on CurrentTime, not e.timer (correction, planner
// ruling): e.timer is a private *time.Ticker, never serialized —
// structurally impossible to replicate on the JS side, which only ever
// sees gameState.timer (CurrentTime) over the wire.
func TestGetHostContext_MotionCardHost(t *testing.T) {
	tests := []struct {
		name        string
		subphase    MotionSubPhase
		currentTime int
		want        HostContext
	}{
		{
			// CURRENT_TIME > 0 in QUESTION → TimerRunning == true.
			name:        "MotionCard_Question_Playable_TimerRunning",
			subphase:    MotionSubPhaseQuestion,
			currentTime: 30,
			want:        HostContext{Playable: true, Revealed: false, TimerRunning: true, CardID: "mc-1"},
		},
		{
			// CURRENT_TIME == 0 in QUESTION (after the per-card timer
			// stopped) → TimerRunning == false.
			name:        "MotionCard_Question_Playable_TimerStopped",
			subphase:    MotionSubPhaseQuestion,
			currentTime: 0,
			want:        HostContext{Playable: true, Revealed: false, TimerRunning: false, CardID: "mc-1"},
		},
		{
			name:     "MotionCard_Reveal_Revealed",
			subphase: MotionSubPhaseReveal,
			want:     HostContext{Playable: false, Revealed: true, TimerRunning: false, CardID: "mc-1"},
		},
		{
			// CardID stays "mc-1" here even though this test setup wouldn't
			// occur in practice (GRID normally implies MotionSelected==""
			// — see startMEMOTION/initMotionStateUnsafe) — deliberate: this
			// table pins the mechanical "CardID = MotionSelected, no
			// branching" property (contract §4, B-B9) independently of
			// which subphase is active, exactly what "sans cas particulier"
			// means. Only Playable/Revealed/TimerRunning stay false.
			name:     "MotionCard_Grid_None",
			subphase: MotionSubPhaseGrid,
			want:     HostContext{CardID: "mc-1"},
		},
		{
			name:     "MotionCard_Memorize_None",
			subphase: MotionSubPhaseMemorize,
			want:     HostContext{CardID: "mc-1"},
		},
		{
			// The cell that diverged Go/JS (B-B9): a card IS selected in
			// SELECTED (SelectMotionCard just set MotionSelected), so
			// CardID must reflect it — see the dedicated
			// TestGetHostContext_MotionCardHost_Selected_CardIDMatchesMotionSelected
			// below for the named, non-table-driven regression test planner
			// required.
			name:     "MotionCard_Selected_CardIDSet",
			subphase: MotionSubPhaseSelected,
			want:     HostContext{CardID: "mc-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEngine()
			e.state.Question = &Question{ID: "mq1", Type: QuestionTypeMemotion}
			e.state.Phase = PhaseStarted
			e.state.MotionSubPhase = tt.subphase
			e.state.MotionSelected = "mc-1"
			e.state.CurrentTime = tt.currentTime

			got := e.GetHostContext()
			if got != tt.want {
				t.Errorf("GetHostContext() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestGetHostContext_MotionCardHost_Selected_CardIDMatchesMotionSelected is
// the B-B9 regression test planner required by name: contract §4 used to
// say "selon le cas" for CardID in SELECTED, Go and JS each resolved it
// differently (both green on their own side), test-writer caught the
// divergence, planner ruled Go was wrong and the contract now reads
// "CardID vaut toujours MEMOTION_SELECTED, sans condition, sans
// branchement". Exercised through the real SelectMotionCard path (not a
// hand-set MotionSubPhase/MotionSelected like the table above) so this
// also proves the production code path — not just the derivation
// function in isolation — produces the correct value.
func TestGetHostContext_MotionCardHost_Selected_CardIDMatchesMotionSelected(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	if err := e.SelectMotionCard("mc-2"); err != nil {
		t.Fatalf("SelectMotionCard failed: %v", err)
	}

	ctx := e.GetHostContext()
	if ctx.CardID != e.GetState().MotionSelected {
		t.Fatalf("GetHostContext().CardID = %q, want it to equal MotionSelected (%q)", ctx.CardID, e.GetState().MotionSelected)
	}
	if ctx.CardID != "mc-2" {
		t.Errorf("GetHostContext().CardID = %q, want mc-2", ctx.CardID)
	}
	if ctx.Playable || ctx.Revealed || ctx.TimerRunning {
		t.Errorf("GetHostContext() in SELECTED should have Playable/Revealed/TimerRunning all false, got %+v", ctx)
	}
}

// TestGetHostContext_MotionCardHost_TimerRunning_KeysOnCurrentTime is the
// TimerRunning correction's own regression test (planner ruling, on top of
// B-B3/B-B9): e.timer (a private *time.Ticker) can never be replicated on
// the JS side, which only ever sees gameState.timer (CurrentTime) — so
// TimerRunning must key on CurrentTime, not on whether a Go ticker happens
// to be running. Exercised through the real StartMotionCardTimer /
// StopMotionCardTimer path, not hand-set state, to cover the production
// code that actually drives CurrentTime to/from zero.
func TestGetHostContext_MotionCardHost_TimerRunning_KeysOnCurrentTime(t *testing.T) {
	e := NewEngine()
	q := makeMotionQuestion("mq1", defaultMotionCards(), "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	_ = e.SelectMotionCard("mc-1")
	_ = e.FlipMotionCard()

	e.StartMotionCardTimer(30)
	if got := e.GetHostContext(); !got.TimerRunning {
		t.Errorf("GetHostContext().TimerRunning = false right after StartMotionCardTimer(30), want true (CURRENT_TIME=30 > 0)")
	}

	e.StopMotionCardTimer()
	if got := e.GetState().CurrentTime; got != 0 {
		t.Fatalf("test precondition failed: CurrentTime = %d after StopMotionCardTimer, want 0", got)
	}
	if got := e.GetHostContext(); got.TimerRunning {
		t.Errorf("GetHostContext().TimerRunning = true after StopMotionCardTimer (CURRENT_TIME=0), want false")
	}
}
