package colors

import "testing"

// Copie de contrôle de contracts/models.md « Palette d'équipes (#113) », indépendante du code de production
// (la même table existe dans cmd/server/team_color_palette_test.go pour les tests qui passent par l'App).
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

// TestTeamColorPalette_SixteenEntriesDistinct guards against a future edit collapsing
// two of the 16 canonical entries onto the same RGB value (e.g. a typo'd channel).
func TestTeamColorPalette_SixteenEntriesDistinct(t *testing.T) {
	seen := make(map[[3]int]string, len(contractColorTable))
	for _, c := range contractColorTable {
		rgb, ok := TeamColorPalette[c.key]
		if !ok {
			t.Fatalf("TeamColorPalette missing canonical key %q (rang %d)", c.key, c.rank)
		}
		if rgb != c.rgb {
			t.Errorf("TeamColorPalette[%q] = %v, want %v (contract drift)", c.key, rgb, c.rgb)
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

// TestDimIntensityFor_VifTones_Are64 verifies the 8 vivid tones (L≈55%) keep the
// pre-#113 dim intensity of 64 — this is the backward-compatibility anchor: existing
// hardcoded-64 assertions in led_test.go (TestLEDNormal_*, no COLOR_NAME) must not
// regress once DimIntensityFor replaces the flat constant.
func TestDimIntensityFor_VifTones_Are64(t *testing.T) {
	for _, c := range contractColorTable[:8] {
		got := DimIntensityFor(c.rgb)
		if got != 64 {
			t.Errorf("DimIntensityFor(%s vif %v) = %d, want 64", c.key, c.rgb, got)
		}
	}
}

// TestDimIntensityFor_ProfondTones_AreApprox100 verifies the 8 deep tones (L≈35%)
// land close to the ~100 target from the plan, clearly above the vivid-tone value —
// this is what makes a deep-tone buzzer stay visibly lit in the dim/"not buzzed" state.
func TestDimIntensityFor_ProfondTones_AreApprox100(t *testing.T) {
	for _, c := range contractColorTable[8:] {
		got := DimIntensityFor(c.rgb)
		if got < 90 || got > 110 {
			t.Errorf("DimIntensityFor(%s profond %v) = %d, want within [90,110] (~100)", c.key, c.rgb, got)
		}
		if got <= 64 {
			t.Errorf("DimIntensityFor(%s profond %v) = %d, expected strictly brighter than the vivid-tone value (64)", c.key, c.rgb, got)
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
		got := DimIntensityFor(rgb)
		if got < 64 || got > 128 {
			t.Errorf("DimIntensityFor(%v) = %d, out of bounds [64,128]", rgb, got)
		}
	}
}
