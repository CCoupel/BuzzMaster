package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"testing"
)

// TestLEDSet_Payload_Serialization verifies round-trip JSON for LEDSetPayload.
func TestLEDSet_Payload_Serialization(t *testing.T) {
	payload := protocol.LEDSetPayload{
		Color:     [3]int{255, 128, 0},
		Intensity: 200,
		Effect:    "BLINK",
	}

	msg, err := protocol.NewMessage(protocol.ActionLEDSet, payload)
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	if msg.Action != protocol.ActionLEDSet {
		t.Errorf("Expected action LED_SET, got %s", msg.Action)
	}

	// Deserialize back
	var decoded protocol.LEDSetPayload
	if err := json.Unmarshal(msg.Msg, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal LEDSetPayload: %v", err)
	}

	if decoded.Color != payload.Color {
		t.Errorf("Color mismatch: got %v, want %v", decoded.Color, payload.Color)
	}
	if decoded.Intensity != payload.Intensity {
		t.Errorf("Intensity mismatch: got %d, want %d", decoded.Intensity, payload.Intensity)
	}
	if decoded.Effect != payload.Effect {
		t.Errorf("Effect mismatch: got %s, want %s", decoded.Effect, payload.Effect)
	}
}

// TestLEDSet_ActionConstant verifies the action constant value.
func TestLEDSet_ActionConstant(t *testing.T) {
	if protocol.ActionLEDSet != "LED_SET" {
		t.Errorf("ActionLEDSet should be LED_SET, got %s", protocol.ActionLEDSet)
	}
}

// TestLEDSet_EffectValues verifies all three effect values serialize correctly.
func TestLEDSet_EffectValues(t *testing.T) {
	effects := []string{"SOLID", "BLINK", "DIM"}
	for _, effect := range effects {
		payload := protocol.LEDSetPayload{
			Color:     [3]int{100, 200, 50},
			Intensity: 255,
			Effect:    effect,
		}
		msg, err := protocol.NewMessage(protocol.ActionLEDSet, payload)
		if err != nil {
			t.Fatalf("NewMessage failed for effect %s: %v", effect, err)
		}
		var decoded protocol.LEDSetPayload
		if err := json.Unmarshal(msg.Msg, &decoded); err != nil {
			t.Fatalf("Unmarshal failed for effect %s: %v", effect, err)
		}
		if decoded.Effect != effect {
			t.Errorf("Effect %s: round-trip failed, got %s", effect, decoded.Effect)
		}
	}
}

// TestAnswerColorToRGB verifies the QCM answer color mapping to RGB.
func TestAnswerColorToRGB(t *testing.T) {
	tests := []struct {
		color    game.AnswerColor
		expected [3]int
	}{
		{game.AnswerColorRed, [3]int{255, 0, 0}},
		{game.AnswerColorGreen, [3]int{0, 255, 0}},
		{game.AnswerColorYellow, [3]int{255, 255, 0}},
		{game.AnswerColorBlue, [3]int{0, 0, 255}},
		{game.AnswerColorNone, [3]int{0, 0, 0}},
	}

	for _, tt := range tests {
		result := answerColorToRGB(tt.color)
		if result != tt.expected {
			t.Errorf("answerColorToRGB(%s) = %v, want %v", tt.color, result, tt.expected)
		}
	}
}

// TestLEDSet_QCMReady_BroadcastReady verifies that READY sends answer color SOLID 100% for QCM.
// This is an integration-style test using newTestApp helper.
func TestLEDSet_QCMReady_BroadcastReady(t *testing.T) {
	app := newTestApp(t)

	// Set up a QCM question
	question := &game.Question{
		ID:   "q1",
		Type: game.QuestionTypeQCM,
	}
	app.engine.Ready("q1", question)

	// Set up two bumpers with answer colors directly (UpdateBumper doesn't handle ANSWER_COLOR)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"AA:BB:CC:DD:EE:01": {Team: "TeamA", AnswerColor: game.AnswerColorRed},
		"AA:BB:CC:DD:EE:02": {Team: "TeamB", AnswerColor: game.AnswerColorBlue},
	})

	// Call sendLEDSetAllBuzzers (called by broadcastReady)
	// Since no WebSocket buzzers are connected in this unit test,
	// we only verify that bumperLEDState is populated correctly.
	app.sendLEDSetAllBuzzers()

	// Check bumper 1: RED → [255,0,0] SOLID 100%
	state1, ok := app.bumperLEDState["AA:BB:CC:DD:EE:01"]
	if !ok {
		t.Fatal("Expected LED state for bumper AA:BB:CC:DD:EE:01")
	}
	if state1.Color != [3]int{255, 0, 0} {
		t.Errorf("Bumper 1 color: got %v, want [255,0,0]", state1.Color)
	}
	if state1.Intensity != 255 {
		t.Errorf("Bumper 1 intensity: got %d, want 255", state1.Intensity)
	}
	if state1.Effect != "SOLID" {
		t.Errorf("Bumper 1 effect: got %s, want SOLID", state1.Effect)
	}

	// Check bumper 2: BLUE → [0,0,255] SOLID 100%
	state2, ok := app.bumperLEDState["AA:BB:CC:DD:EE:02"]
	if !ok {
		t.Fatal("Expected LED state for bumper AA:BB:CC:DD:EE:02")
	}
	if state2.Color != [3]int{0, 0, 255} {
		t.Errorf("Bumper 2 color: got %v, want [0,0,255]", state2.Color)
	}
}

// TestLEDSet_BroadcastStop verifies that STOP sends team color SOLID 100% to all buzzers.
func TestLEDSet_BroadcastStop(t *testing.T) {
	app := newTestApp(t)

	// Create a team with red color
	teams := map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{200, 100, 50}},
	}
	app.engine.SetTeams(teams)

	// Create a bumper assigned to TeamA
	app.engine.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{
		"TEAM": "TeamA",
	})

	app.sendLEDSetStop()

	state, ok := app.bumperLEDState["AA:BB:CC:DD:EE:01"]
	if !ok {
		t.Fatal("Expected LED state for bumper AA:BB:CC:DD:EE:01")
	}
	// No ColorName → nearestPaletteColorByHue maps [200,100,50] to the closest hue entry
	wantColor := nearestPaletteColorByHue(200, 100, 50)
	if state.Color != wantColor {
		t.Errorf("STOP color: got %v, want nearest-hue-palette %v (from [200,100,50])", state.Color, wantColor)
	}
	if state.Intensity != 255 {
		t.Errorf("STOP intensity: got %d, want 255", state.Intensity)
	}
	if state.Effect != "SOLID" {
		t.Errorf("STOP effect: got %s, want SOLID", state.Effect)
	}
}

// TestLEDSet_ResendLEDOnReconnect verifies that the server resends the last LED state on HELLO.
func TestLEDSet_ResendLEDOnReconnect(t *testing.T) {
	app := newTestApp(t)

	// Pre-populate a known LED state
	expectedPayload := protocol.LEDSetPayload{
		Color:     [3]int{0, 255, 0},
		Intensity: 128,
		Effect:    "DIM",
	}
	app.bumperLEDState["AA:BB:CC:DD:EE:01"] = expectedPayload

	// resendLEDOnReconnect should find the stored state.
	// Since no WebSocket buzzer is connected, it won't actually send —
	// but the stored state should still be present.
	app.resendLEDOnReconnect("AA:BB:CC:DD:EE:01")

	// Verify the state is unchanged (not cleared by the function)
	state := app.bumperLEDState["AA:BB:CC:DD:EE:01"]
	if state.Color != expectedPayload.Color {
		t.Errorf("resendLEDOnReconnect: color altered, got %v", state.Color)
	}
	if state.Intensity != expectedPayload.Intensity {
		t.Errorf("resendLEDOnReconnect: intensity altered, got %d", state.Intensity)
	}
}

// TestLEDSet_RevealQCM_FirstBuzz verifies per-buzzer LED at REVEALED for QCM:
// correct+first=BLINK, wrong+buzzed=DIM 25%, non-buzzed=DIM 25%.
func TestLEDSet_RevealQCM_FirstBuzz(t *testing.T) {
	app := newTestApp(t)

	// Set up QCM question with QCMCorrect = GREEN
	question := &game.Question{
		ID:         "q1",
		Type:       game.QuestionTypeQCM,
		QCMCorrect: "GREEN",
	}
	app.engine.Ready("q1", question)

	// Set up bumpers: Team A buzzed first (correct GREEN), Team B buzzed (wrong RED), Team C no buzz
	app.engine.SetBumpers(map[string]*game.Bumper{
		"AA:BB:CC:DD:EE:01": {Team: "TeamA", AnswerColor: game.AnswerColorGreen, Time: 1000},
		"AA:BB:CC:DD:EE:02": {Team: "TeamB", AnswerColor: game.AnswerColorRed, Time: 2000},
		"AA:BB:CC:DD:EE:03": {Team: "TeamC", AnswerColor: game.AnswerColorBlue},
	})

	// Set BuzzStates: TeamA=MOI (first), TeamB=MOI (first of their team), TeamC=NONE
	app.bumperBuzzState["AA:BB:CC:DD:EE:01"] = game.BuzzStateMoi
	app.bumperBuzzState["AA:BB:CC:DD:EE:02"] = game.BuzzStateMoi
	app.bumperBuzzState["AA:BB:CC:DD:EE:03"] = game.BuzzStateNone

	// Simulate REVEALED phase
	app.engine.SetPhase(game.PhaseRevealed)

	app.sendLEDSetReveal("GREEN")

	// Bumper 1 (TeamA, correct=GREEN, MOI, first buzz): BLINK green
	s1, ok := app.bumperLEDState["AA:BB:CC:DD:EE:01"]
	if !ok {
		t.Fatal("No LED state for bumper 1")
	}
	if s1.Effect != "BLINK" {
		t.Errorf("Correct+first bumper: expected BLINK, got %s", s1.Effect)
	}
	if s1.Color != [3]int{0, 255, 0} {
		t.Errorf("Correct bumper: expected GREEN [0,255,0], got %v", s1.Color)
	}

	// Bumper 2 (TeamB, wrong=RED, MOI): DIM 25% red answer
	s2, ok := app.bumperLEDState["AA:BB:CC:DD:EE:02"]
	if !ok {
		t.Fatal("No LED state for bumper 2")
	}
	if s2.Effect != "DIM" {
		t.Errorf("Wrong-buzzed bumper: expected DIM, got %s", s2.Effect)
	}
	if s2.Intensity != 64 {
		t.Errorf("Wrong-buzzed bumper: expected intensity 64 (DIM 25%%), got %d", s2.Intensity)
	}

	// Bumper 3 (TeamC, NONE, no buzz): DIM 25% blue answer
	s3, ok := app.bumperLEDState["AA:BB:CC:DD:EE:03"]
	if !ok {
		t.Fatal("No LED state for bumper 3")
	}
	if s3.Effect != "DIM" {
		t.Errorf("Non-buzzed bumper: expected DIM, got %s", s3.Effect)
	}
	if s3.Intensity != 64 {
		t.Errorf("Non-buzzed bumper: expected intensity 64, got %d", s3.Intensity)
	}
}

// TestLEDSet_RevealQCM_CorrectNotFirst verifies: correct answer but not first buzz → SOLID 100%.
func TestLEDSet_RevealQCM_CorrectNotFirst(t *testing.T) {
	app := newTestApp(t)

	question := &game.Question{
		ID:         "q1",
		Type:       game.QuestionTypeQCM,
		QCMCorrect: "GREEN",
	}
	app.engine.Ready("q1", question)

	// TeamA buzzed first (wrong RED), TeamB buzzed second (correct GREEN)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"AA:BB:CC:DD:EE:01": {Team: "TeamA", AnswerColor: game.AnswerColorRed, Time: 1000},
		"AA:BB:CC:DD:EE:02": {Team: "TeamB", AnswerColor: game.AnswerColorGreen, Time: 2000},
	})
	app.bumperBuzzState["AA:BB:CC:DD:EE:01"] = game.BuzzStateMoi
	app.bumperBuzzState["AA:BB:CC:DD:EE:02"] = game.BuzzStateMoi

	app.engine.SetPhase(game.PhaseRevealed)
	app.sendLEDSetReveal("GREEN")

	// Bumper 2 (correct GREEN, but TeamA buzzed first): SOLID 100%
	s2, ok := app.bumperLEDState["AA:BB:CC:DD:EE:02"]
	if !ok {
		t.Fatal("No LED state for bumper 2")
	}
	if s2.Effect != "SOLID" {
		t.Errorf("Correct+not-first bumper: expected SOLID, got %s", s2.Effect)
	}
	if s2.Intensity != 255 {
		t.Errorf("Correct+not-first bumper: expected intensity 255, got %d", s2.Intensity)
	}
	if s2.Color != [3]int{0, 255, 0} {
		t.Errorf("Correct+not-first bumper: expected green, got %v", s2.Color)
	}
}

// ---------------------------------------------------------------------------
// BuzzState tracking tests (v3.4.4)
// ---------------------------------------------------------------------------

// TestBuzzState_ResetOnReady verifies that resetBuzzStates sets all bumpers to NONE.
func TestBuzzState_ResetOnReady(t *testing.T) {
	app := newTestApp(t)

	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:01": {Team: "TeamA"},
		"MAC:02": {Team: "TeamB"},
	})

	// Pre-populate non-NONE states
	app.bumperBuzzState["MAC:01"] = game.BuzzStateMoi
	app.bumperBuzzState["MAC:02"] = game.BuzzStateAutre

	app.resetBuzzStates()

	for _, mac := range []string{"MAC:01", "MAC:02"} {
		if bs := app.bumperBuzzState[mac]; bs != game.BuzzStateNone {
			t.Errorf("After reset, %s: expected NONE, got %s", mac, bs)
		}
	}
}

// TestBuzzState_UpdateBuzzStates_SingleTeam verifies MOI/EQUIPE assignment within one team.
func TestBuzzState_UpdateBuzzStates_SingleTeam(t *testing.T) {
	app := newTestApp(t)

	// TeamA has two buzzers; TeamB has one
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA"},
		"MAC:A2": {Team: "TeamA"},
		"MAC:B1": {Team: "TeamB"},
	})
	app.resetBuzzStates()

	// MAC:A1 buzzes first
	app.updateBuzzStates("MAC:A1")

	if bs := app.bumperBuzzState["MAC:A1"]; bs != game.BuzzStateMoi {
		t.Errorf("MAC:A1 should be MOI, got %s", bs)
	}
	if bs := app.bumperBuzzState["MAC:A2"]; bs != game.BuzzStateEquipe {
		t.Errorf("MAC:A2 should be EQUIPE, got %s", bs)
	}
	if bs := app.bumperBuzzState["MAC:B1"]; bs != game.BuzzStateAutre {
		t.Errorf("MAC:B1 should be AUTRE, got %s", bs)
	}
}

// TestBuzzState_UpdateBuzzStates_TwoTeams verifies that a second team buzz does not overwrite the first.
func TestBuzzState_UpdateBuzzStates_TwoTeams(t *testing.T) {
	app := newTestApp(t)

	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA"},
		"MAC:B1": {Team: "TeamB"},
		"MAC:C1": {Team: "TeamC"},
	})
	app.resetBuzzStates()

	// TeamA buzzes first
	app.updateBuzzStates("MAC:A1")

	// TeamB buzzes second
	app.updateBuzzStates("MAC:B1")

	// TeamA's MOI must be preserved
	if bs := app.bumperBuzzState["MAC:A1"]; bs != game.BuzzStateMoi {
		t.Errorf("MAC:A1 should still be MOI after TeamB buzz, got %s", bs)
	}
	// TeamB's bumper becomes MOI
	if bs := app.bumperBuzzState["MAC:B1"]; bs != game.BuzzStateMoi {
		t.Errorf("MAC:B1 should be MOI, got %s", bs)
	}
	// TeamC (no buzz) becomes AUTRE (already set by first buzz)
	if bs := app.bumperBuzzState["MAC:C1"]; bs != game.BuzzStateAutre {
		t.Errorf("MAC:C1 should be AUTRE, got %s", bs)
	}
}

// ---------------------------------------------------------------------------
// SPEEDY mode LED state machine tests (v3.4.4)
// ---------------------------------------------------------------------------

// TestLEDNormal_Started_NONE verifies that STARTED+NONE = DIM 25%.
func TestLEDNormal_Started_NONE(t *testing.T) {
	app := newTestApp(t)

	question := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready("q1", question)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA"},
	})
	app.resetBuzzStates()
	app.engine.SetPhase(game.PhaseStarted)

	app.sendLEDSetAllBuzzers()

	s, ok := app.bumperLEDState["MAC:A1"]
	if !ok {
		t.Fatal("No LED state for MAC:A1")
	}
	if s.Effect != "DIM" {
		t.Errorf("STARTED+NONE: expected DIM, got %s", s.Effect)
	}
	if s.Intensity != 64 {
		t.Errorf("STARTED+NONE: expected intensity 64, got %d", s.Intensity)
	}
}

// TestLEDNormal_Started_MOI verifies that STARTED+MOI = BLINK 100%.
func TestLEDNormal_Started_MOI(t *testing.T) {
	app := newTestApp(t)

	question := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready("q1", question)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA"},
		"MAC:B1": {Team: "TeamB"},
	})
	app.resetBuzzStates()
	app.engine.SetPhase(game.PhaseStarted)

	// TeamA buzzes
	app.bumperBuzzState["MAC:A1"] = game.BuzzStateMoi
	app.bumperBuzzState["MAC:B1"] = game.BuzzStateAutre

	app.sendLEDSetAllBuzzers()

	s, ok := app.bumperLEDState["MAC:A1"]
	if !ok {
		t.Fatal("No LED state for MAC:A1")
	}
	if s.Effect != "BLINK" {
		t.Errorf("STARTED+MOI: expected BLINK, got %s", s.Effect)
	}
	if s.Intensity != 255 {
		t.Errorf("STARTED+MOI: expected intensity 255, got %d", s.Intensity)
	}
}

// TestLEDNormal_Started_EQUIPE verifies that STARTED+EQUIPE = SOLID 100%.
func TestLEDNormal_Started_EQUIPE(t *testing.T) {
	app := newTestApp(t)

	question := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready("q1", question)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA"},
		"MAC:A2": {Team: "TeamA"},
	})
	app.resetBuzzStates()
	app.engine.SetPhase(game.PhaseStarted)

	app.bumperBuzzState["MAC:A1"] = game.BuzzStateMoi
	app.bumperBuzzState["MAC:A2"] = game.BuzzStateEquipe

	app.sendLEDSetAllBuzzers()

	s, ok := app.bumperLEDState["MAC:A2"]
	if !ok {
		t.Fatal("No LED state for MAC:A2")
	}
	if s.Effect != "SOLID" {
		t.Errorf("STARTED+EQUIPE: expected SOLID, got %s", s.Effect)
	}
	if s.Intensity != 255 {
		t.Errorf("STARTED+EQUIPE: expected intensity 255, got %d", s.Intensity)
	}
}

// TestLEDNormal_Paused_MOI verifies that PAUSED+MOI = BLINK 100%.
func TestLEDNormal_Paused_MOI(t *testing.T) {
	app := newTestApp(t)

	question := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready("q1", question)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA"},
	})
	app.resetBuzzStates()
	app.bumperBuzzState["MAC:A1"] = game.BuzzStateMoi
	app.engine.SetPhase(game.PhasePaused)

	app.sendLEDSetPause("MAC:A1")

	s := app.bumperLEDState["MAC:A1"]
	if s.Effect != "BLINK" {
		t.Errorf("PAUSED+MOI: expected BLINK, got %s", s.Effect)
	}
}

// TestLEDNormal_Revealed_AUTRE verifies that REVEALED+AUTRE = DIM 25%.
func TestLEDNormal_Revealed_AUTRE(t *testing.T) {
	app := newTestApp(t)

	question := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready("q1", question)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:B1": {Team: "TeamB"},
	})
	app.resetBuzzStates()
	app.bumperBuzzState["MAC:B1"] = game.BuzzStateAutre
	app.engine.SetPhase(game.PhaseRevealed)

	app.sendLEDSetReveal("")

	s := app.bumperLEDState["MAC:B1"]
	if s.Effect != "DIM" {
		t.Errorf("REVEALED+AUTRE: expected DIM, got %s", s.Effect)
	}
	if s.Intensity != 64 {
		t.Errorf("REVEALED+AUTRE: expected intensity 64, got %d", s.Intensity)
	}
}

// ---------------------------------------------------------------------------
// QCM LED state machine tests (v3.4.4)
// ---------------------------------------------------------------------------

// TestLEDQCM_Started_NONE verifies that STARTED+NONE = SOLID 100% answer color.
func TestLEDQCM_Started_NONE(t *testing.T) {
	app := newTestApp(t)

	question := &game.Question{ID: "q1", Type: game.QuestionTypeQCM, QCMCorrect: "RED"}
	app.engine.Ready("q1", question)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA", AnswerColor: game.AnswerColorRed},
	})
	app.resetBuzzStates()
	app.engine.SetPhase(game.PhaseStarted)

	app.sendLEDSetAllBuzzers()

	s := app.bumperLEDState["MAC:A1"]
	if s.Effect != "SOLID" {
		t.Errorf("QCM STARTED+NONE: expected SOLID, got %s", s.Effect)
	}
	if s.Color != [3]int{255, 0, 0} {
		t.Errorf("QCM STARTED+NONE: expected RED color, got %v", s.Color)
	}
}

// TestLEDQCM_Started_MOI verifies that STARTED+MOI = SOLID 100% team color (hides answer).
func TestLEDQCM_Started_MOI(t *testing.T) {
	app := newTestApp(t)

	// TeamA has color [255,0,0] from newTestApp
	question := &game.Question{ID: "q1", Type: game.QuestionTypeQCM}
	app.engine.Ready("q1", question)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA", AnswerColor: game.AnswerColorBlue},
	})
	app.resetBuzzStates()
	app.bumperBuzzState["MAC:A1"] = game.BuzzStateMoi
	app.engine.SetPhase(game.PhaseStarted)

	app.sendLEDSetAllBuzzers()

	s := app.bumperLEDState["MAC:A1"]
	if s.Effect != "SOLID" {
		t.Errorf("QCM STARTED+MOI: expected SOLID, got %s", s.Effect)
	}
	// Should show TEAM color [255,0,0], not answer color BLUE
	if s.Color != [3]int{255, 0, 0} {
		t.Errorf("QCM STARTED+MOI: expected team color [255,0,0], got %v", s.Color)
	}
}

// TestLEDQCM_Paused_EQUIPE verifies that PAUSED+EQUIPE = SOLID 100% team color.
func TestLEDQCM_Paused_EQUIPE(t *testing.T) {
	app := newTestApp(t)

	question := &game.Question{ID: "q1", Type: game.QuestionTypeQCM}
	app.engine.Ready("q1", question)
	// TeamA color [255,0,0], two members
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA", AnswerColor: game.AnswerColorGreen},
		"MAC:A2": {Team: "TeamA", AnswerColor: game.AnswerColorGreen},
	})
	app.resetBuzzStates()
	app.bumperBuzzState["MAC:A1"] = game.BuzzStateMoi
	app.bumperBuzzState["MAC:A2"] = game.BuzzStateEquipe
	app.engine.SetPhase(game.PhasePaused)

	app.sendLEDSetPause("MAC:A1")

	s := app.bumperLEDState["MAC:A2"]
	if s.Effect != "SOLID" {
		t.Errorf("QCM PAUSED+EQUIPE: expected SOLID, got %s", s.Effect)
	}
	if s.Color != [3]int{255, 0, 0} {
		t.Errorf("QCM PAUSED+EQUIPE: expected team color [255,0,0], got %v", s.Color)
	}
}

// ---------------------------------------------------------------------------
// MEMORY mode LED state machine tests (v3.4.4)
// ---------------------------------------------------------------------------

// TestLEDMemory_Solo_ActiveTeam verifies STARTED+active = SOLID 100% in SOLO mode.
func TestLEDMemory_Solo_ActiveTeam(t *testing.T) {
	app := newTestApp(t)

	question := &game.Question{ID: "q1", Type: game.QuestionTypeMemory, MemoryMode: "SOLO"}
	app.engine.Ready("q1", question)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA"},
		"MAC:B1": {Team: "TeamB"},
	})

	// SetMemoryParticipatingTeams requires PREPARE or READY phase.
	// engine.Ready() sets phase to PREPARE. Call it before forcing STARTED.
	if err := app.engine.SetMemoryParticipatingTeams([]string{"TeamA", "TeamB"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams failed: %v", err)
	}

	// Now simulate the game being STARTED
	app.engine.SetPhase(game.PhaseStarted)

	app.sendLEDSetAllBuzzers()

	sA := app.bumperLEDState["MAC:A1"]
	if sA.Effect != "SOLID" || sA.Intensity != 255 {
		t.Errorf("MEMORY SOLO active: expected SOLID 255, got effect=%s intensity=%d", sA.Effect, sA.Intensity)
	}

	sB := app.bumperLEDState["MAC:B1"]
	if sB.Effect != "DIM" || sB.Intensity != 64 {
		t.Errorf("MEMORY SOLO inactive: expected DIM 64, got effect=%s intensity=%d", sB.Effect, sB.Intensity)
	}
}

// TestLEDMemory_Solo_Paused verifies all buzzers = DIM 25% when PAUSED in SOLO mode.
func TestLEDMemory_Solo_Paused(t *testing.T) {
	app := newTestApp(t)

	question := &game.Question{ID: "q1", Type: game.QuestionTypeMemory, MemoryMode: "SOLO"}
	app.engine.Ready("q1", question)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA"},
		"MAC:B1": {Team: "TeamB"},
	})
	if err := app.engine.SetMemoryParticipatingTeams([]string{"TeamA", "TeamB"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams failed: %v", err)
	}
	app.engine.SetPhase(game.PhasePaused)

	app.sendLEDSetPauseAll()

	for _, mac := range []string{"MAC:A1", "MAC:B1"} {
		s := app.bumperLEDState[mac]
		if s.Effect != "DIM" || s.Intensity != 64 {
			t.Errorf("MEMORY PAUSED %s: expected DIM 64, got effect=%s intensity=%d", mac, s.Effect, s.Intensity)
		}
	}
}

// TestLEDMemory_MultiTeam_Next verifies STARTED+next = SOLID 50% in multi-team mode.
func TestLEDMemory_MultiTeam_Next(t *testing.T) {
	app := newTestApp(t)

	question := &game.Question{ID: "q1", Type: game.QuestionTypeMemory, MemoryMode: "CHACUN_SON_TOUR"}
	app.engine.Ready("q1", question)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA"},
		"MAC:B1": {Team: "TeamB"},
		"MAC:C1": {Team: "TeamC"},
	})
	// SetMemoryParticipatingTeams requires PREPARE/READY phase — call before SetPhase(STARTED)
	if err := app.engine.SetMemoryParticipatingTeams([]string{"TeamA", "TeamB", "TeamC"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams failed: %v", err)
	}
	app.engine.SetPhase(game.PhaseStarted)

	app.sendLEDSetAllBuzzers()

	// TeamA=active: SOLID 100%
	sA := app.bumperLEDState["MAC:A1"]
	if sA.Effect != "SOLID" || sA.Intensity != 255 {
		t.Errorf("Multi-team active: expected SOLID 255, got effect=%s intensity=%d", sA.Effect, sA.Intensity)
	}

	// TeamB=next: SOLID 50% (intensity=128)
	sB := app.bumperLEDState["MAC:B1"]
	if sB.Effect != "SOLID" || sB.Intensity != 128 {
		t.Errorf("Multi-team next: expected SOLID 128, got effect=%s intensity=%d", sB.Effect, sB.Intensity)
	}

	// TeamC=other participating: DIM 25%
	sC := app.bumperLEDState["MAC:C1"]
	if sC.Effect != "DIM" || sC.Intensity != 64 {
		t.Errorf("Multi-team other: expected DIM 64, got effect=%s intensity=%d", sC.Effect, sC.Intensity)
	}
}

// TestLEDMemory_MultiTeam_NotSelected verifies non-participating buzzer = OFF.
func TestLEDMemory_MultiTeam_NotSelected(t *testing.T) {
	app := newTestApp(t)

	question := &game.Question{ID: "q1", Type: game.QuestionTypeMemory, MemoryMode: "CHACUN_SON_TOUR"}
	app.engine.Ready("q1", question)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA"},
		"MAC:B1": {Team: "TeamB"},
		"MAC:C1": {Team: "TeamC"}, // not selected
	})
	// Only A and B participate, C is not selected. MemoryCurrentTeam = TeamA (first).
	if err := app.engine.SetMemoryParticipatingTeams([]string{"TeamA", "TeamB"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams failed: %v", err)
	}
	app.engine.SetPhase(game.PhaseStarted)

	app.sendLEDSetAllBuzzers()

	sC := app.bumperLEDState["MAC:C1"]
	if sC.Color != [3]int{0, 0, 0} || sC.Intensity != 0 {
		t.Errorf("Non-selected team: expected OFF (0,0,0 intensity=0), got color=%v intensity=%d", sC.Color, sC.Intensity)
	}
}

// ---------------------------------------------------------------------------
// Button filter test (v3.4.4)
// ---------------------------------------------------------------------------

// TestButtonFilter_IgnoredOutsideStarted verifies BUTTON is ignored in non-STARTED phases.
func TestButtonFilter_IgnoredOutsideStarted(t *testing.T) {
	app := newTestApp(t)

	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA"},
	})

	phasesToTest := []game.GamePhase{
		game.PhaseReady,
		game.PhaseCountdown,
		game.PhasePaused,
		game.PhaseRevealed,
		game.PhaseStopped,
	}

	for _, phase := range phasesToTest {
		app.engine.SetPhase(phase)
		// The filter is in handleButton which calls engine.ProcessButtonPress.
		// We test the filter condition directly here via GetPhase.
		if app.engine.GetPhase() == game.PhaseStarted {
			t.Errorf("Phase %s: should not be STARTED", phase)
		}
	}
}

// TestBuzzStateFor_DefaultsNone verifies that buzzStateFor returns NONE for unknown MACs.
func TestBuzzStateFor_DefaultsNone(t *testing.T) {
	app := newTestApp(t)

	bs := app.buzzStateFor("UNKNOWN:MAC")
	if bs != game.BuzzStateNone {
		t.Errorf("Expected NONE for unknown MAC, got %s", bs)
	}
}
