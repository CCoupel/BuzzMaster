package main

import (
	"buzzcontrol/internal/game"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests : palette de 16 couleurs d'équipe (#113)
//
// Table normative : contracts/models.md § "Palette d'équipes (#113)". Le
// backend (teamColorPalette, cmd/server/main.go) et le frontend (TEAM_COLORS,
// web/src/constants/colors.js) doivent rester strictement identiques —
// contractColorTable ci-dessous est une copie de contrôle de cette table,
// volontairement indépendante du code de production, pour détecter toute
// dérive future de teamColorPalette.
// ---------------------------------------------------------------------------

type contractColorEntry struct {
	rank int
	key  string
	rgb  [3]int
}

// contractColorTable mirrors contracts/models.md § "Palette d'équipes (#113)" exactly.
var contractColorTable = []contractColorEntry{
	{1, "rouge", [3]int{255, 26, 26}},
	{2, "orange", [3]int{255, 133, 26}},
	{3, "jaune", [3]int{255, 217, 26}},
	{4, "vert", [3]int{26, 255, 83}},
	{5, "cyan", [3]int{26, 236, 255}},
	{6, "bleu", [3]int{26, 94, 255}},
	{7, "violet", [3]int{159, 26, 255}},
	{8, "rose", [3]int{255, 26, 159}},
	{9, "rouge-profond", [3]int{179, 0, 0}},
	{10, "orange-profond", [3]int{179, 83, 0}},
	{11, "jaune-profond", [3]int{179, 149, 0}},
	{12, "vert-profond", [3]int{0, 179, 45}},
	{13, "cyan-profond", [3]int{0, 164, 179}},
	{14, "bleu-profond", [3]int{0, 54, 179}},
	{15, "violet-profond", [3]int{104, 0, 179}},
	{16, "rose-profond", [3]int{179, 0, 104}},
}

// TestTeamColorToRGB_ContractTable verifies teamColorToRGB resolves each of the 16
// canonical COLOR_NAME keys to the exact RGB value from contracts/models.md.
func TestTeamColorToRGB_ContractTable(t *testing.T) {
	app := newTestApp(t)

	teams := make(map[string]*game.Team, len(contractColorTable))
	for _, c := range contractColorTable {
		teams[c.key] = &game.Team{Name: c.key, ColorName: c.key}
	}
	app.engine.SetTeams(teams)

	for _, c := range contractColorTable {
		bumper := &game.Bumper{Team: c.key}
		got := app.teamColorToRGB(bumper)
		if got != c.rgb {
			t.Errorf("teamColorToRGB(COLOR_NAME=%q) = %v, want %v (rang %d)", c.key, got, c.rgb, c.rank)
		}
	}
}

// TestTeamColorToRGB_LegacyFallbackByHue_NeverGray verifies that a team created before
// #113 (Color set, no ColorName) still resolves to a real, non-gray color via
// nearestPaletteColorByHue — one representative hue per vivid anchor (rangs 1-8).
func TestTeamColorToRGB_LegacyFallbackByHue_NeverGray(t *testing.T) {
	app := newTestApp(t)

	// Legacy teams: Color set directly on the vivid anchor hues, no ColorName —
	// exactly the shape of a team saved before the #113 migration.
	legacyVividAnchors := contractColorTable[:8]
	teams := make(map[string]*game.Team, len(legacyVividAnchors))
	for _, c := range legacyVividAnchors {
		teams[c.key] = &game.Team{Name: c.key, Color: []int{c.rgb[0], c.rgb[1], c.rgb[2]}}
	}
	app.engine.SetTeams(teams)

	gray := [3]int{128, 128, 128}
	for _, c := range legacyVividAnchors {
		bumper := &game.Bumper{Team: c.key}
		got := app.teamColorToRGB(bumper)
		if got == gray {
			t.Errorf("teamColorToRGB fell back to gray for legacy team %q (Color=%v) — expected a resolved hue", c.key, c.rgb)
		}
		if got != c.rgb {
			t.Errorf("teamColorToRGB(legacy Color=%v, no COLOR_NAME) = %v, want %v (nearest vivid anchor)", c.rgb, got, c.rgb)
		}
	}
}

// TestSendLEDSetForBuzzerQCM_NonRegression_DeepToneTeam_KeepsAnswerIntensity64 guards
// against B3 (dimIntensityFor, sendLEDSetForBuzzerNormal only) ever leaking into the
// QCM path (sendLEDSetForBuzzerQCM / sendLEDSetForBuzzerQCMReveal): the QCM DIM state
// always dims the *answer* color (never the team color) at a flat Intensity:64,
// regardless of the team's tone (vivid or deep). See plan corrections § "Deux
// corrections au brief" — correcting this path would be a scope violation, not a fix.
func TestSendLEDSetForBuzzerQCM_NonRegression_DeepToneTeam_KeepsAnswerIntensity64(t *testing.T) {
	app := newTestApp(t)

	// TeamDeep uses a deep-tone COLOR_NAME (bleu-profond, L≈35%) — if B3 leaked into
	// the QCM path, this buzzer's DIM intensity would drift up to ~100 instead of 64.
	app.engine.SetTeams(map[string]*game.Team{
		"TeamDeep": {Name: "TeamDeep", ColorName: "bleu-profond"},
	})

	question := &game.Question{ID: "q1", Type: game.QuestionTypeQCM, TypedContent: game.TypedContent{QCMCorrect: "GREEN"}}
	app.engine.Ready("q1", question)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:DEEP1": {Team: "TeamDeep", AnswerColor: game.AnswerColorRed, Time: 1000},
	})
	app.resetBuzzStates()
	app.bumperBuzzState["MAC:DEEP1"] = game.BuzzStateMoi // buzzed, but wrong answer (RED != GREEN)
	app.engine.SetPhase(game.PhaseRevealed)

	app.sendLEDSetReveal("GREEN")

	s, ok := app.bumperLEDState["MAC:DEEP1"]
	if !ok {
		t.Fatal("No LED state for MAC:DEEP1")
	}
	if s.Effect != "DIM" {
		t.Errorf("wrong answer, deep-tone team: expected DIM, got %s", s.Effect)
	}
	if s.Intensity != 64 {
		t.Errorf("wrong answer, deep-tone team: expected flat Intensity 64 (QCM path untouched by B3), got %d", s.Intensity)
	}
}
