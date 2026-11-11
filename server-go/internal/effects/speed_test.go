package effects

// Vitesse réglable à l'usage — contracts/light-effects.md §6 : durée réelle d'un tick = duree_tick_ms ÷ vitesse, jamais
// sous 250 ms (cadence de l'écrivain). Valeurs permises : 0.5, 0.75, 1, 1.5, 2. La vitesse n'agit que sur l'horloge :
// pas, variables et couleurs sont inchangés (le rendu est une fonction du numéro de tick, pas du temps).

import (
	"testing"
	"time"
)

func near2(a, b, tol time.Duration) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func TestSpeed_TickDuration_DividedBySpeed(t *testing.T) {
	cases := []struct {
		tickMs int
		speed  float64
		want   time.Duration
	}{
		{1000, 1, time.Second},
		{1000, 0.5, 2 * time.Second},
		{1000, 0.75, 1333 * time.Millisecond},
		{1000, 1.5, 667 * time.Millisecond},
		{1000, 2, 500 * time.Millisecond},
		{60000, 0.5, 120 * time.Second},
		{500, 1, 500 * time.Millisecond},
		{500, 0.5, time.Second},
	}
	for _, c := range cases {
		if got := tickDuration(c.tickMs, c.speed); !near2(got, c.want, time.Millisecond) {
			t.Errorf("tick %d ms ×%v : %s, attendu %s", c.tickMs, c.speed, got, c.want)
		}
	}
}

func TestSpeed_TickDuration_NeverBelow250ms(t *testing.T) {
	for _, c := range []struct {
		tickMs int
		speed  float64
	}{{400, 2}, {500, 2}, {250, 1}, {250, 2}, {300, 1.5}, {250, 1.5}} {
		if got := tickDuration(c.tickMs, c.speed); got < 250*time.Millisecond {
			t.Errorf("tick %d ms ×%v : %s — jamais sous 250 ms (débit du pont)", c.tickMs, c.speed, got)
		}
	}
	// exemple du contrat : un effet à 400 ms en ×2 tourne à 250 ms, pas à 200 ms
	if got := tickDuration(400, 2); got != 250*time.Millisecond {
		t.Errorf("400 ms ×2 = %s, attendu 250 ms", got)
	}
}

func TestSpeed_DoesNotChangeStepsVariablesNorColours(t *testing.T) {
	// le rendu ne dépend que du numéro de tick : la vitesse n'a aucune prise sur lui
	src := "id: t\nnom: T\nversion: 2\nmoments: [chrono]\nduree_tick_ms: 400\nvariables:\n  gA: g1; +1\npas:\n  - { g0: [\"#0000FF\", 40], gA: [\"#FFFFFF\", 100] }\n"
	got := rows(t, src, ctxN(4), 5)
	eqRows(t, "400 ms", got, []string{"Wbbb", "bWbb", "bbWb", "bbbW", "Wbbb"})
}
