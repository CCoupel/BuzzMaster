// Package colors holds the colour maths shared by the buzzer LED paths and the
// ambiance scenes: the 16-colour team palette (#113), RGB/HSL helpers, the
// nearest-palette lookup and the dim intensity. Pure functions, extracted from
// cmd/server (#291, Phase B).
package colors

import (
	"math"
	"strconv"
	"strings"

	"buzzcontrol/internal/game"
)

// AnswerColorToRGB maps a QCM AnswerColor to an [R, G, B] array for buzzer LED.
func AnswerColorToRGB(color game.AnswerColor) [3]int {
	switch color {
	case game.AnswerColorRed:
		return [3]int{255, 0, 0}
	case game.AnswerColorGreen:
		return [3]int{0, 255, 0}
	case game.AnswerColorYellow:
		return [3]int{255, 255, 0}
	case game.AnswerColorBlue:
		return [3]int{0, 0, 255}
	default:
		return [3]int{0, 0, 0}
	}
}

// TeamColorPalette maps the 16 canonical team color keys (#113) to their exact RGB
// values, plus a handful of legacy English aliases pointing at the vivid tones for
// robustness. Used by teamColorToRGB when a team has a ColorName set.
//
// The 16 entries are 8 hues declined as a vivid tone (S=100% L=55%) and a deep tone
// (S=100% L=35%) — this is the normative table in contracts/models.md ("Palette
// d'équipes (#113)"). Frontend source of truth: web/src/constants/colors.js
// (TEAM_COLORS). The two tables MUST stay value-for-value identical, or a team's
// physical buzzer LED will diverge from its on-screen color.
var TeamColorPalette = map[string][3]int{
	// Vivid tones (rank 1-8)
	"rouge":  {255, 26, 26},
	"red":    {255, 26, 26},
	"orange": {255, 133, 26},
	"jaune":  {255, 217, 26},
	"yellow": {255, 217, 26},
	"vert":   {26, 255, 83},
	"green":  {26, 255, 83},
	"cyan":   {26, 236, 255},
	"bleu":   {26, 94, 255},
	"blue":   {26, 94, 255},
	"violet": {159, 26, 255},
	"purple": {159, 26, 255},
	"rose":   {255, 26, 159},
	"pink":   {255, 26, 159},
	// Deep tones (rank 9-16)
	"rouge-profond":  {179, 0, 0},
	"orange-profond": {179, 83, 0},
	"jaune-profond":  {179, 149, 0},
	"vert-profond":   {0, 179, 45},
	"cyan-profond":   {0, 164, 179},
	"bleu-profond":   {0, 54, 179},
	"violet-profond": {104, 0, 179},
	"rose-profond":   {179, 0, 104},
}

// rgbToHue converts an RGB color (0-255 per channel) to a hue angle in degrees [0, 360).
// Returns -1 for achromatic colors (delta < 0.01).
func rgbToHue(r, g, b int) float64 {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	delta := max - min
	if delta < 0.01 {
		return -1 // achromatic
	}
	var h float64
	switch max {
	case rf:
		h = (gf - bf) / delta
		if gf < bf {
			h += 6
		}
	case gf:
		h = (bf-rf)/delta + 2
	default:
		h = (rf-gf)/delta + 4
	}
	return h * 60
}

// rgbSaturation returns the HSL saturation (0.0–1.0) for the given RGB.
func rgbSaturation(r, g, b int) float64 {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	l := (max + min) / 2
	if l == 0 || l == 1 {
		return 0
	}
	return (max - min) / (1 - math.Abs(2*l-1))
}

// rgbLightness returns the HSL lightness (0.0–1.0) for the given RGB.
func rgbLightness(r, g, b int) float64 {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	return (max + min) / 2
}

// DimIntensityFor returns the LED intensity (0-255) to use for a "dimmed team color"
// LED state, scaled to the perceived brightness of the team color rather than a flat
// constant. Used by sendLEDSetForBuzzerNormal (#113 B3) and by the MEMORY LED paths
// sendLEDSetForBuzzerMemory / sendLEDSetMemoryMultiTeam (#113 B3 fast-follow,
// code-review 20260726) — every place that dims the TEAM color. Not used by the QCM
// paths (sendLEDSetForBuzzerQCM/Reveal), which dim the ANSWER color at a flat 64 and
// are intentionally left untouched. The firmware applies intensity multiplicatively
// (channel × intensity / 255, see src/Common/led.h), so a flat 64 nearly extinguishes
// deep tones (L≈35%) while looking fine on vivid ones (L≈55%).
//
// Linear interpolation pinned at the two tone families defined by the #113 palette
// (contracts/models.md): L=55% (vivid) → 64 (unchanged from the pre-#113 behavior),
// L=35% (deep) → ~100. Result is clamped to [64, 128] so both ends of the color
// spectrum (near-white, near-black/achromatic fallback) stay within a sane dim range.
func DimIntensityFor(rgb [3]int) int {
	l := rgbLightness(rgb[0], rgb[1], rgb[2])
	intensity := 163.0 - 180.0*l
	i := int(math.Round(intensity))
	if i < 64 {
		i = 64
	}
	if i > 128 {
		i = 128
	}
	return i
}

// NearestPaletteColorByHue returns the TeamColorPalette entry whose hue angle is closest
// to (r,g,b) using circular hue distance. For achromatic inputs (saturation < 0.15)
// returns white. Hue-based matching is more robust than Euclidean RGB distance for
// distinguishing orange/red, blue/violet, etc., and prevents two teams with the same
// raw RGB from mapping to the same palette entry.
func NearestPaletteColorByHue(r, g, b int) [3]int {
	if rgbSaturation(r, g, b) < 0.15 {
		return [3]int{255, 255, 255} // achromatic → white
	}
	hue := rgbToHue(r, g, b)
	if hue < 0 {
		return [3]int{255, 255, 255}
	}
	type entry struct {
		rgb [3]int
		hue float64
	}
	ordered := []entry{
		{[3]int{255, 26, 26}, 0},    // rouge
		{[3]int{255, 133, 26}, 28},  // orange
		{[3]int{255, 217, 26}, 50},  // jaune
		{[3]int{26, 255, 83}, 135},  // vert
		{[3]int{26, 236, 255}, 185}, // cyan
		{[3]int{26, 94, 255}, 222},  // bleu
		{[3]int{159, 26, 255}, 275}, // violet
		{[3]int{255, 26, 159}, 325}, // rose
	}
	best := ordered[0].rgb
	minDist := 361.0
	for _, e := range ordered {
		d := math.Abs(hue - e.hue)
		if d > 180 {
			d = 360 - d // circular hue distance
		}
		if d < minDist {
			minDist = d
			best = e.rgb
		}
	}
	return best
}

// HexToRGB parses a "#rrggbb" colour (ResolveCategoryMeta's own format)
// into the project's [3]int 0-255 RGB. false for anything else (empty —
// custom category or none, malformed, wrong length).
func HexToRGB(s string) ([3]int, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return [3]int{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return [3]int{}, false
	}
	return [3]int{int(v >> 16 & 0xff), int(v >> 8 & 0xff), int(v & 0xff)}, true
}
