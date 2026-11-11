package effects

// Aides de lecture des tableaux : un jeton par couleur, une chaîne par tick.

import (
	"strings"
	"testing"
	"time"
)

var (
	cB    = [3]int{0, 0, 255}
	cW    = [3]int{255, 255, 255}
	cR    = [3]int{255, 0, 0}
	cG    = [3]int{0, 255, 0}
	cOr   = [3]int{255, 170, 0}
	cRed  = [3]int{230, 30, 30}
	cT    = [3]int{79, 70, 229} // thème de test
	cTeam = [3]int{229, 57, 53} // équipe de test
	cScn  = [3]int{10, 20, 30}  // scène de test
)

func near(a, b, tol int) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

// tok : jeton d'un état — « b » bleu doux (#0000FF à 40 %), « B » bleu plein, « W » blanc, « R » rouge, « G » vert,
// « . » éteint, « _ » aucun état (scène ordinaire), « ? » autre (avec la couleur et l'intensité pour le message).
func tok(l Lk) string {
	if l.Absent {
		return "_"
	}
	if l.Intensity == 0 {
		return "."
	}
	full := near(l.Intensity, 255, 1)
	switch l.Color {
	case cB:
		if near(l.Intensity, 102, 1) {
			return "b"
		}
		if full {
			return "B"
		}
	case cW:
		if full {
			return "W"
		}
	case cR:
		if full {
			return "R"
		}
	case cG:
		if full {
			return "G"
		}
	}
	return "?"
}

// row : les jetons des N groupes (positions 1…N) au tick donné.
func row(v View, n int) string {
	var b strings.Builder
	for p := 1; p <= n; p++ {
		b.WriteString(tok(v.effective(p)))
	}
	return b.String()
}

// rows : row pour les ticks 0…count-1.
func rows(t *testing.T, src string, c Ctx, count int) []string {
	t.Helper()
	e := mustParse(t, src)
	out := make([]string, count)
	for tick := 0; tick < count; tick++ {
		out[tick] = row(renderView(e, c, tick), c.N)
	}
	return out
}

func eqRows(t *testing.T, label string, got, want []string) {
	t.Helper()
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Errorf("%s : tick %d = %q, attendu %q (obtenu : %v)", label, i, got[i], want[i], got)
			return
		}
	}
}

// ctxN : contexte de chrono standard à n groupes.
func ctxN(n int) Ctx {
	return Ctx{N: n, Moment: "chrono", Remaining: 20 * time.Second, Total: 20 * time.Second, Scene: cScn, Theme: cT, Team: cTeam}
}

// effet minimal : les champs obligatoires + le corps fourni.
func fx(body string) string {
	return "id: t\nnom: T\nversion: 2\nmoments: [chrono]\nduree_tick_ms: 1000\n" + body
}
