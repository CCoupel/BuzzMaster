// #144 (v12.0.0, LOT-1.2) — construction et normalisation de la roue (contracts/roue.md §10, §10.1).
// Les 10 exemples du contrat sont des cas EXIGÉS (case pour case) ; mêmes cas qu'au niveau HTTP
// (internal/server/roue_wheel_preview_144_test.go) : la règle est unique, côté serveur.
package game

import (
	"fmt"
	"strings"
	"testing"
)

func renderWheel144(segs []RoueSegment) string {
	parts := make([]string, len(segs))
	for i, s := range segs {
		switch s.Kind {
		case RoueKindBankrupt:
			parts[i] = "BANQ"
		case RoueKindLoseTurn:
			parts[i] = "PERDU"
		default:
			parts[i] = fmt.Sprint(s.Value)
		}
	}
	return strings.Join(parts, " ")
}

func buildWheel144(t *testing.T, max, step, b, p int) ([]RoueSegment, RoueWheelParams, int) {
	t.Helper()
	params := DefaultRoueWheelParams()
	params.MaxPoints, params.Step, params.BankruptCount, params.LoseTurnCount = max, step, b, p
	norm, adjusted, err := NormalizeRoueWheelParams(params)
	if err != nil {
		t.Fatalf("NormalizeRoueWheelParams(%d/%d/%d/%d) : %v", max, step, b, p, err)
	}
	return BuildRoueWheel(norm), norm, adjusted
}

func TestRoueWheel144_Game_ContractExamples(t *testing.T) {
	cases := []struct {
		max, step, b, p int
		want            string
	}{
		{100, 10, 0, 0, "100 10 90 20 80 30 70 40 60 50"},
		{100, 10, 1, 0, "100 BANQ 90 10 80 20 70 30 60 40 50"},
		{100, 10, 1, 1, "100 BANQ 90 10 80 PERDU 20 70 30 60 40"},
		{100, 10, 0, 1, "100 10 90 20 80 PERDU 30 70 40 60"},
		{1000, 100, 2, 1, "1000 BANQ 900 100 PERDU 800 200 700 BANQ 300 600 400"},
		{1000, 50, 2, 1, "1000 BANQ 950 50 900 100 850 PERDU 150 800 200 750 250 700 BANQ 300 650 350 600 400 550 450"},
		{100, 20, 1, 1, "100 BANQ 80 PERDU 20 60 40"},
		{1000, 100, 3, 2, "1000 BANQ PERDU 900 100 BANQ 800 200 PERDU 700 300 BANQ 600 400"},
		{100, 10, 0, 2, "100 10 PERDU 90 20 80 30 70 PERDU 40 60"},
		{100, 30, 1, 1, "100 BANQ PERDU 75 25"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%d/%d/%d/%d", c.max, c.step, c.b, c.p), func(t *testing.T) {
			segs, _, _ := buildWheel144(t, c.max, c.step, c.b, c.p)
			if got := renderWheel144(segs); got != c.want {
				t.Errorf("roue = %q\nattendu = %q", got, c.want)
			}
		})
	}
}

func TestRoueWheel144_Game_StepCorrection(t *testing.T) {
	cases := []struct{ max, step, want int }{{100, 30, 25}, {1000, 300, 250}, {1000, 70, 50}, {100, 7, 5}, {100, 15, 10}}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%d/%d", c.max, c.step), func(t *testing.T) {
			_, norm, adjusted := buildWheel144(t, c.max, c.step, 1, 1)
			if norm.Step != c.want || adjusted != c.step {
				t.Errorf("pas retenu = %d (saisi %d), attendu %d (saisi %d)", norm.Step, adjusted, c.want, c.step)
			}
		})
	}
	// pas valide : aucune correction annoncée
	if _, _, adjusted := buildWheel144(t, 100, 10, 1, 1); adjusted != 0 {
		t.Errorf("aucune correction attendue pour un pas valide, got adjustedFrom=%d", adjusted)
	}
}

func TestRoueWheel144_Game_Refusals(t *testing.T) {
	cases := map[string][4]int{
		"pas = maximum":   {100, 100, 1, 1},
		"pas > maximum":   {100, 200, 1, 1},
		"pas nul":         {100, 0, 1, 1},
		"pas négatif":     {100, -5, 1, 1},
		"maximum négatif": {-100, 10, 1, 1},
		"B négatif":       {100, 10, -1, 0},
		"P négatif":       {100, 10, 0, -1},
		"trop de cases":   {1000, 10, 0, 0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := DefaultRoueWheelParams()
			p.MaxPoints, p.Step, p.BankruptCount, p.LoseTurnCount = c[0], c[1], c[2], c[3]
			if _, _, err := NormalizeRoueWheelParams(p); err == nil {
				t.Errorf("refus attendu pour %v", c)
			}
		})
	}
}

// Durées : hors bornes → ramenées à la borne ; absentes (0) → défaut.
func TestRoueWheel144_Game_DurationsClampedAndDefaulted(t *testing.T) {
	p := DefaultRoueWheelParams()
	p.SpinPeriodMs, p.SpinMaxMs, p.StopDurationMs, p.FlipIntervalMs = 10, 1, 99999, 99999
	n, _, err := NormalizeRoueWheelParams(p)
	if err != nil {
		t.Fatal(err)
	}
	if n.SpinPeriodMs != 1000 || n.SpinMaxMs != 3000 || n.StopDurationMs != 8000 || n.FlipIntervalMs != 1500 {
		t.Errorf("bornes : %+v", n)
	}
	p.SpinPeriodMs, p.SpinMaxMs, p.StopDurationMs, p.FlipIntervalMs = 99999, 99999, 1, 1
	n, _, _ = NormalizeRoueWheelParams(p)
	if n.SpinPeriodMs != 6000 || n.SpinMaxMs != 30000 || n.StopDurationMs != 1500 || n.FlipIntervalMs != 200 {
		t.Errorf("bornes hautes/basses : %+v", n)
	}
	def := DefaultRoueWheelParams()
	z := def
	z.SpinPeriodMs, z.SpinMaxMs, z.StopDurationMs, z.FlipIntervalMs = 0, 0, 0, 0
	n, _, _ = NormalizeRoueWheelParams(z)
	if n.SpinPeriodMs != 2500 || n.SpinMaxMs != 10000 || n.StopDurationMs != 4000 || n.FlipIntervalMs != 600 {
		t.Errorf("défauts : %+v", n)
	}
}

func TestRoueWheel144_Game_DefaultIs22SegmentsWithUniqueIDs(t *testing.T) {
	segs := BuildRoueWheel(DefaultRoueWheelParams())
	if len(segs) != 22 {
		t.Fatalf("roue par défaut : %d cases, attendu 22", len(segs))
	}
	ids := map[string]bool{}
	for _, s := range segs {
		if s.ID == "" || ids[s.ID] {
			t.Errorf("ID vide ou dupliqué : %q", s.ID)
		}
		ids[s.ID] = true
	}
}

// Propriété : toute roue acceptée respecte N = valeurs + B + P (moins la valeur max/2 cédée au TOUR PERDU),
// la première case est le maximum, et BANQUEROUTE / TOUR PERDU apparaissent exactement B / P fois.
func TestRoueWheel144_Game_StructuralInvariants(t *testing.T) {
	for _, c := range [][4]int{{100, 10, 0, 0}, {100, 10, 1, 1}, {1000, 50, 2, 1}, {600, 10, 3, 2}, {100, 25, 0, 3}} {
		segs, norm, _ := buildWheel144(t, c[0], c[1], c[2], c[3])
		if len(segs) > RoueMaxSegments {
			t.Errorf("%v : %d cases > %d", c, len(segs), RoueMaxSegments)
		}
		if segs[0].Kind != RoueKindValue || segs[0].Value != norm.MaxPoints {
			t.Errorf("%v : la première case doit être le maximum, got %+v", c, segs[0])
		}
		var b, p int
		for _, s := range segs {
			switch s.Kind {
			case RoueKindBankrupt:
				b++
			case RoueKindLoseTurn:
				p++
			}
		}
		if b != norm.BankruptCount || p != norm.LoseTurnCount {
			t.Errorf("%v : BANQ=%d PERDU=%d, attendu %d/%d", c, b, p, norm.BankruptCount, norm.LoseTurnCount)
		}
	}
}
