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

// TestTeamColorPalette_SixteenEntriesDistinct guards against a future edit collapsing
// two of the 16 canonical entries onto the same RGB value (e.g. a typo'd channel).
func TestTeamColorPalette_SixteenEntriesDistinct(t *testing.T) {
	seen := make(map[[3]int]string, len(contractColorTable))
	for _, c := range contractColorTable {
		rgb, ok := teamColorPalette[c.key]
		if !ok {
			t.Fatalf("teamColorPalette missing canonical key %q (rang %d)", c.key, c.rank)
		}
		if rgb != c.rgb {
			t.Errorf("teamColorPalette[%q] = %v, want %v (contract drift)", c.key, rgb, c.rgb)
		}
		if prevKey, dup := seen[rgb]; dup {
			t.Errorf("RGB %v used by both %q and %q — the 16 entries must be pairwise distinct", rgb, prevKey, c.key)
		}
		seen[rgb] = c.key
	}
	if len(seen) != 16 {
		t.Errorf("expected 16 distinct RGB values across the canonical palette, got %d", len(seen))
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

// TestDimIntensityFor_VifTones_Are64 verifies the 8 vivid tones (L≈55%) keep the
// pre-#113 dim intensity of 64 — this is the backward-compatibility anchor: existing
// hardcoded-64 assertions in led_test.go (TestLEDNormal_*, no COLOR_NAME) must not
// regress once dimIntensityFor replaces the flat constant.
func TestDimIntensityFor_VifTones_Are64(t *testing.T) {
	for _, c := range contractColorTable[:8] {
		got := dimIntensityFor(c.rgb)
		if got != 64 {
			t.Errorf("dimIntensityFor(%s vif %v) = %d, want 64", c.key, c.rgb, got)
		}
	}
}

// TestDimIntensityFor_ProfondTones_AreApprox100 verifies the 8 deep tones (L≈35%)
// land close to the ~100 target from the plan, clearly above the vivid-tone value —
// this is what makes a deep-tone buzzer stay visibly lit in the dim/"not buzzed" state.
func TestDimIntensityFor_ProfondTones_AreApprox100(t *testing.T) {
	for _, c := range contractColorTable[8:] {
		got := dimIntensityFor(c.rgb)
		if got < 90 || got > 110 {
			t.Errorf("dimIntensityFor(%s profond %v) = %d, want within [90,110] (~100)", c.key, c.rgb, got)
		}
		if got <= 64 {
			t.Errorf("dimIntensityFor(%s profond %v) = %d, expected strictly brighter than the vivid-tone value (64)", c.key, c.rgb, got)
		}
	}
}

// TestDimIntensityFor_BoundedRange verifies the [64, 128] clamp holds for all 16
// palette entries plus the achromatic extremes (white/black/gray fallback).
func TestDimIntensityFor_BoundedRange(t *testing.T) {
	inputs := make([][3]int, 0, len(contractColorTable)+3)
	for _, c := range contractColorTable {
		inputs = append(inputs, c.rgb)
	}
	inputs = append(inputs, [3]int{255, 255, 255}, [3]int{0, 0, 0}, [3]int{128, 128, 128})

	for _, rgb := range inputs {
		got := dimIntensityFor(rgb)
		if got < 64 || got > 128 {
			t.Errorf("dimIntensityFor(%v) = %d, out of bounds [64,128]", rgb, got)
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
