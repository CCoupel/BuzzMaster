package server

import "math"

// PICTO sheet geometry (contracts/picto.md §8.2) — the Go half of the normalisation contract, mirrored by
// web/src/components/picto/pictoGeometry.js. Named test cases, identical on both sides: normalize_center,
// normalize_corners, clamp_outside, round_half, single_point_dot, line_width_units.

// PictoLineWidth is the fixed stroke width in sheet units (0.8 % of the side), rounded ends and joins.
const PictoLineWidth = 80

// PictoClamp bounds a coordinate to 0..PictoCoordMax.
func PictoClamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > PictoCoordMax {
		return PictoCoordMax
	}
	return v
}

// PictoNormalize converts a pixel position on a width × height sheet to integer sheet units:
// x = round(px / width × 10000), y = round(py / height × 10000), halves rounded up (as JavaScript's
// Math.round), then clamped to 0..10000. The origin is the top-left corner. A non-positive side yields 0.
func PictoNormalize(px, py, width, height float64) (x, y int) {
	if width <= 0 || height <= 0 {
		return 0, 0
	}
	x = PictoClamp(int(math.Floor(px/width*PictoCoordMax + 0.5)))
	y = PictoClamp(int(math.Floor(py/height*PictoCoordMax + 0.5)))
	return x, y
}

// PictoDotDiameter is the diameter of a one-point stroke: a disc of the stroke width.
func PictoDotDiameter() int { return PictoLineWidth }
