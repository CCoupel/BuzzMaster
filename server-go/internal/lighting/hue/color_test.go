// Conversion couleur (contracts/hue-bridge.md §5.2) : rgbToXY (reprise telle
// quelle du spike, déjà exercée sur un pont réel) et intensityToBri
// (nouvelle, propre à la production).
package hue

import "testing"

func TestRGBToXY_KnownColors(t *testing.T) {
	// Reprend les bornes déjà validées par le spike (spike/hue-bridge/
	// guard_test.go, TestColourAndSSDPParsing) : rouge pur a un x élevé et
	// un y bas dans l'espace CIE xy.
	if xy := rgbToXY(255, 0, 0); xy[0] < 0.6 || xy[1] > 0.35 {
		t.Errorf("rouge : xy = %v, attendu x>=0.6 et y<=0.35", xy)
	}
	// Noir pur : cas dégénéré documenté (sum == 0) → point blanc D65 exact,
	// jamais une division par zéro.
	if xy := rgbToXY(0, 0, 0); xy != [2]float64{0.3127, 0.3290} {
		t.Errorf("noir : xy = %v, attendu le point blanc D65 {0.3127, 0.3290} (cas dégénéré)", xy)
	}
	// Rouge/vert/bleu doivent produire trois points bien distincts entre eux
	// (couvre la conversion, pas une coïncidence de calcul).
	r, g, b := rgbToXY(255, 0, 0), rgbToXY(0, 255, 0), rgbToXY(0, 0, 255)
	if r == g || g == b || r == b {
		t.Errorf("rouge/vert/bleu doivent être distincts : r=%v g=%v b=%v", r, g, b)
	}
	// Bornes valides : x et y toujours dans [0,1] quel que soit l'intrant.
	for _, c := range [][3]int{{0, 0, 0}, {255, 255, 255}, {128, 64, 200}, {300, -10, 999}} {
		xy := rgbToXY(c[0], c[1], c[2])
		if xy[0] < 0 || xy[0] > 1 || xy[1] < 0 || xy[1] > 1 {
			t.Errorf("rgbToXY%v = %v hors de [0,1]", c, xy)
		}
	}
}

func TestClamp(t *testing.T) {
	cases := []struct{ v, lo, hi, want int }{
		{-10, 0, 255, 0},
		{300, 0, 255, 255},
		{128, 0, 255, 128},
		{0, 0, 255, 0},
		{255, 0, 255, 255},
	}
	for _, c := range cases {
		if got := clamp(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("clamp(%d,%d,%d) = %d, attendu %d", c.v, c.lo, c.hi, got, c.want)
		}
	}
}

// TestIntensityToBri_LinearMapping documents that this function does the
// PLAIN linear 0-255 → 1-254 mapping only — it does NOT special-case 0.
// Contract §5.2: "Intensity == 0 ⇒ {"on": false} plutôt qu'une luminosité
// nulle" is the CALLER's responsibility (the not-yet-built Driver), exactly
// because 1 (this function's floor) is not the same thing as "off". A test
// here pins that intensityToBri(0) is 1, not 0 — so nobody "simplifies" the
// off-handling into this function later and silently drops the on:false path.
func TestIntensityToBri_LinearMapping(t *testing.T) {
	cases := []struct{ intensity, want int }{
		{0, 1},     // floor — NOT "off"; the caller must special-case Intensity==0 itself
		{255, 254}, // ceiling
		{128, 127}, // milieu, arrondi
		{-50, 1},   // clamp d'entrée
		{500, 254}, // clamp d'entrée
	}
	for _, c := range cases {
		if got := intensityToBri(c.intensity); got != c.want {
			t.Errorf("intensityToBri(%d) = %d, attendu %d", c.intensity, got, c.want)
		}
	}
}
