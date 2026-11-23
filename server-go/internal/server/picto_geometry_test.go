package server

// #193 (v13.0.0, LOT-2.1) — géométrie de la feuille PICTO, moitié Go (contracts/picto.md §8.2).
// Les 6 cas nommés portent le MÊME nom et les MÊMES valeurs attendues que dans
// `web/src/components/picto/pictoGeometry.test.js` : l'antidote à R5 (dérive Go/JS). Toute valeur modifiée ici
// doit l'être à l'identique côté JS.

import "testing"

func TestPictoGeometry_NamedCases(t *testing.T) {
	t.Run("normalize_center", func(t *testing.T) {
		if x, y := PictoNormalize(500, 500, 1000, 1000); x != 5000 || y != 5000 {
			t.Errorf("centre = [%d,%d], attendu [5000,5000]", x, y)
		}
	})

	t.Run("normalize_corners", func(t *testing.T) {
		for _, c := range []struct {
			px, py float64
			wx, wy int
		}{
			{0, 0, 0, 0}, {1000, 0, 10000, 0}, {0, 1000, 0, 10000}, {1000, 1000, 10000, 10000},
		} {
			if x, y := PictoNormalize(c.px, c.py, 1000, 1000); x != c.wx || y != c.wy {
				t.Errorf("coin (%v,%v) = [%d,%d], attendu [%d,%d]", c.px, c.py, x, y, c.wx, c.wy)
			}
		}
	})

	t.Run("clamp_outside", func(t *testing.T) {
		if x, y := PictoNormalize(-50, 1200, 1000, 1000); x != 0 || y != 10000 {
			t.Errorf("hors feuille = [%d,%d], attendu [0,10000]", x, y)
		}
		if x, y := PictoNormalize(5000, -1, 1000, 1000); x != 10000 || y != 0 {
			t.Errorf("hors feuille = [%d,%d], attendu [10000,0]", x, y)
		}
	})

	t.Run("round_half", func(t *testing.T) {
		// La moitié est arrondie vers le haut, comme Math.round côté JS.
		for _, c := range []struct {
			px, size float64
			want     int
		}{
			{1, 20000, 1}, // 0,5 → 1
			{3, 20000, 2}, // 1,5 → 2
			{1, 3, 3333},  // 3333,33 → 3333
			{2, 3, 6667},  // 6666,67 → 6667
		} {
			if x, _ := PictoNormalize(c.px, 0, c.size, c.size); x != c.want {
				t.Errorf("round(%v/%v×10000) = %d, attendu %d", c.px, c.size, x, c.want)
			}
		}
	})

	t.Run("single_point_dot", func(t *testing.T) {
		// Un trait d'un seul point est un disque du diamètre du trait.
		if d := PictoDotDiameter(); d != 80 {
			t.Errorf("diamètre du point = %d, attendu 80 (= largeur du trait)", d)
		}
	})

	t.Run("line_width_units", func(t *testing.T) {
		if PictoLineWidth != 80 {
			t.Errorf("PictoLineWidth = %d, attendu 80 (0,8 %% du côté)", PictoLineWidth)
		}
		if PictoCoordMax != 10000 {
			t.Errorf("PictoCoordMax = %d, attendu 10000", PictoCoordMax)
		}
	})
}

func TestPictoGeometry_DegenerateSheetNeverPanicsNorOverflows(t *testing.T) {
	if x, y := PictoNormalize(10, 10, 0, 0); x != 0 || y != 0 {
		t.Errorf("feuille de taille nulle = [%d,%d], attendu [0,0] (comme le JS)", x, y)
	}
	if x, y := PictoNormalize(10, 10, -5, 100); x != 0 || y != 0 {
		t.Errorf("côté négatif = [%d,%d], attendu [0,0]", x, y)
	}
	if x, _ := PictoNormalize(1e12, 0, 1, 1); x != 10000 {
		t.Errorf("valeur énorme = %d, attendu 10000 (borne)", x)
	}
}

func TestPictoClamp(t *testing.T) {
	for in, want := range map[int]int{-1: 0, 0: 0, 5000: 5000, 10000: 10000, 10001: 10000} {
		if got := PictoClamp(in); got != want {
			t.Errorf("PictoClamp(%d) = %d, attendu %d", in, got, want)
		}
	}
}
