// Tests for #185 C-B2 — QCM nestable descriptor + end-to-end MEMOTION round
// mixing a SPEEDY and a QCM card, and the negative guarantee that no new
// inbound action opens up for a QCM-typed card (contract §7.1).
// Run: go test ./internal/game/... -run TestMEMOTION_QCMCard -v

package game

import "testing"

// TestMEMOTION_QCMCard_RegistryNestable pins C-B2's registry requirement:
// QCM must be NestableInMotionCard with exactly the recto/question media
// slots (contract §7 — no answer-image face, consistent with dev-frontend's
// QCM sub-editor never exposing an answerImage field).
func TestMEMOTION_QCMCard_RegistryNestable(t *testing.T) {
	desc, ok := TypeDescriptorFor(QuestionTypeQCM)
	if !ok {
		t.Fatal("QCM has no registry entry")
	}
	if !desc.NestableInMotionCard {
		t.Error("QCM.NestableInMotionCard = false, want true (#185)")
	}
	want := map[string]bool{"recto": true, "question": true}
	if len(desc.MediaSlots) != len(want) {
		t.Fatalf("QCM.MediaSlots = %v, want exactly %v", desc.MediaSlots, want)
	}
	for _, slot := range desc.MediaSlots {
		if !want[slot] {
			t.Errorf("unexpected QCM media slot %q", slot)
		}
	}
	if !IsNestableInMotionCard(QuestionTypeQCM) {
		t.Error("IsNestableInMotionCard(QCM) = false, want true")
	}
}

// TestMEMOTION_QCMCard_EndToEnd_MixedWithSPEEDY drives a full MEMOTION round
// with one SPEEDY and one QCM card, both played to completion, and checks:
//   - the QCM card selects/flips/reveals/completes exactly like a SPEEDY one
//     (no regression on the SPEEDY card in the same round);
//   - points are awarded via the HOST's default STARS scale (DoneMotionCard
//     unchanged, contract §6.1 — the barème always belongs to the host) —
//     the QCM card carries no PointsRule of its own here, so it must fall
//     through to the same difficulty→points scale as SPEEDY;
//   - the round completes (isComplete) once both cards are DONE.
func TestMEMOTION_QCMCard_EndToEnd_MixedWithSPEEDY(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})

	cards := []MotionCard{
		{ID: "mc-speedy", RectoTheme: "Histoire", Difficulty: 1, QuestionText: "Q?", AnswerText: "A"},
		func() MotionCard {
			c := qcmMotionCard("mc-qcm")
			c.Difficulty = 2
			return c
		}(),
	}
	q := makeMotionQuestion("mq1", cards, "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	// Play the SPEEDY card first — no regression expected.
	if err := e.SelectMotionCard("mc-speedy"); err != nil {
		t.Fatalf("SelectMotionCard(mc-speedy) failed: %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard failed: %v", err)
	}
	if err := e.RevealMotionCard(); err != nil {
		t.Fatalf("RevealMotionCard failed: %v", err)
	}
	pointsSpeedy, isComplete, err := e.DoneMotionCard("mc-speedy", "red", 1)
	if err != nil {
		t.Fatalf("DoneMotionCard(mc-speedy) failed: %v", err)
	}
	if pointsSpeedy != 1 { // difficulty 1 → 1pt under STARS (motionDifficultyPoints)
		t.Errorf("SPEEDY card points = %d, want 1 (STARS/difficulty-1)", pointsSpeedy)
	}
	if isComplete {
		t.Error("round should not be complete after only 1 of 2 cards")
	}

	// Now the QCM card — must behave exactly the same way (select → flip →
	// reveal → done, host-awarded points), nothing QCM-specific required
	// from the animateur beyond the same MEMOTION_DONE.
	if err := e.SelectMotionCard("mc-qcm"); err != nil {
		t.Fatalf("SelectMotionCard(mc-qcm) failed: %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard failed: %v", err)
	}
	if err := e.RevealMotionCard(); err != nil {
		t.Fatalf("RevealMotionCard failed: %v", err)
	}
	pointsQCM, isComplete, err := e.DoneMotionCard("mc-qcm", "red", 1)
	if err != nil {
		t.Fatalf("DoneMotionCard(mc-qcm) failed: %v", err)
	}
	if pointsQCM != 3 { // difficulty 2 → 3pt under STARS — DoneMotionCard/motionCardPoints unchanged (#184)
		t.Errorf("QCM card points = %d, want 3 (STARS/difficulty-2, host barème unchanged)", pointsQCM)
	}
	if !isComplete {
		t.Error("round should be complete after both cards are DONE")
	}

	team := e.GetTeamsAndBumpers().Teams["red"]
	if team.TeamPoints != pointsSpeedy+pointsQCM {
		t.Errorf("team.TeamPoints = %d, want %d (%d+%d)", team.TeamPoints, pointsSpeedy+pointsQCM, pointsSpeedy, pointsQCM)
	}
}

// TestMEMOTION_QCMCard_NoNewInboundAction is the negative test contract
// §7.1 requires: a QCM-typed active card opens NO new inbound action.
// ProcessButtonPress already ignores every buzz for a MEMOTION question
// unconditionally (#184/pre-existing) — this test pins that guarantee
// specifically for a QCM-typed active card, so a future change that makes
// ProcessButtonPress card-type-aware (e.g. "let QCM cards accept buzz like
// a real QCM question") breaks this test instead of silently opening the
// door contract §7.1 explicitly closes for #185.
func TestMEMOTION_QCMCard_NoNewInboundAction(t *testing.T) {
	e := NewEngine()
	e.SetBumpers(map[string]*Bumper{
		"buzzer1": {Team: "red"},
	})
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red", Color: []int{255, 0, 0}}})

	cards := []MotionCard{qcmMotionCard("mc-qcm")}
	q := makeMotionQuestion("mq1", cards, "SOLO")
	startMEMOTION(t, e, "mq1", q)
	defer e.Stop()

	if err := e.SelectMotionCard("mc-qcm"); err != nil {
		t.Fatalf("SelectMotionCard failed: %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard failed: %v", err)
	}

	beforeSubPhase := e.GetState().MotionSubPhase

	// A buzz on a QCM active card must be a complete no-op — same as any
	// other MEMOTION card (SPEEDY), per ProcessButtonPress's existing
	// unconditional MEMOTION guard.
	e.ProcessButtonPress("buzzer1", 1000, "A")

	after := e.GetState()
	if after.MotionSubPhase != beforeSubPhase {
		t.Errorf("MotionSubPhase changed after a buzz on a QCM card: %q → %q", beforeSubPhase, after.MotionSubPhase)
	}
	bumper := e.data.Bumpers["buzzer1"]
	if bumper != nil && bumper.Time != 0 {
		t.Errorf("buzzer1.Time = %d after a buzz on a QCM MEMOTION card, want 0 (buzz must be ignored, not recorded)", bumper.Time)
	}
}
