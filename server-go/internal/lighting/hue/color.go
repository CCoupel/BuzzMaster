package hue

import "math"

// rgbToXY converts an sRGB colour (0-255) to CIE 1931 xy (D65). Taken as-is
// from spike/hue-bridge/hue.go, exercised on a real bridge. The bulb clips
// to its own gamut.
func rgbToXY(r, g, b int) [2]float64 {
	lin := func(c int) float64 {
		v := float64(clamp(c, 0, 255)) / 255.0
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	rl, gl, bl := lin(r), lin(g), lin(b)
	X := 0.4124*rl + 0.3576*gl + 0.1805*bl
	Y := 0.2126*rl + 0.7152*gl + 0.0722*bl
	Z := 0.0193*rl + 0.1192*gl + 0.9505*bl
	sum := X + Y + Z
	if sum == 0 {
		return [2]float64{0.3127, 0.3290} // D65 white for black input
	}
	return [2]float64{round4(X / sum), round4(Y / sum)}
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// intensityToBri maps lighting.ZoneState.Intensity (0-255) to Hue bri (1-254).
// Intensity 0 means "off" and is handled by the caller (contract §5.2).
func intensityToBri(intensity int) int {
	return clamp(int(math.Round(float64(clamp(intensity, 0, 255))*254.0/255.0)), 1, 254)
}
