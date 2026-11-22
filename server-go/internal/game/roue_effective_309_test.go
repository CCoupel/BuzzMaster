package game

// #309 (v12.1.0, LOT-2) — RoueEffectiveParams, fonction pure (contracts/roue.md §16.3), signature (TypedContent, global).

import (
	"reflect"
	"testing"
)

func ip309(v int) *int { return &v }

func eff309(t *testing.T, tc TypedContent, g RoueWheelParams) (RoueWheelParams, bool) {
	t.Helper()
	return RoueEffectiveParams(tc, g)
}

// Aucun champ : le résultat est le global normalisé, sans repli, y compris son BONUS.
func TestRoueEffective309_NoFieldIsGlobal(t *testing.T) {
	for _, g := range []RoueWheelParams{DefaultRoueWheelParams(), glob309(100, 10, 1, 1, 0), glob309(500, 50, 0, 2, 3)} {
		p, fb := eff309(t, TypedContent{}, g)
		ng, _, _ := NormalizeRoueWheelParams(g)
		ng.BonusCount = g.BonusCount
		if fb || !reflect.DeepEqual(p, ng) {
			t.Fatalf("global %+v : got %+v fb=%v, attendu %+v", g, p, fb, ng)
		}
	}
}

// Chacun des 5 paramètres seul : lui seul change, les autres suivent le global.
func TestRoueEffective309_EachAlone(t *testing.T) {
	g := glob309(1000, 50, 2, 1, 1)
	cases := []struct {
		name string
		tc   TypedContent
		want func(p RoueWheelParams) bool
	}{
		{"max", TypedContent{RoueWheelMaxPoints: ip309(500)}, func(p RoueWheelParams) bool {
			return p.MaxPoints == 500 && p.Step == 50 && p.BankruptCount == 2 && p.LoseTurnCount == 1 && p.BonusCount == 1
		}},
		{"step", TypedContent{RoueWheelStep: ip309(100)}, func(p RoueWheelParams) bool {
			return p.MaxPoints == 1000 && p.Step == 100 && p.BankruptCount == 2 && p.LoseTurnCount == 1 && p.BonusCount == 1
		}},
		{"bankrupt", TypedContent{RoueWheelBankruptCount: ip309(3)}, func(p RoueWheelParams) bool {
			return p.MaxPoints == 1000 && p.Step == 50 && p.BankruptCount == 3 && p.LoseTurnCount == 1 && p.BonusCount == 1
		}},
		{"lose_turn", TypedContent{RoueWheelLoseTurnCount: ip309(0)}, func(p RoueWheelParams) bool {
			return p.MaxPoints == 1000 && p.Step == 50 && p.BankruptCount == 2 && p.LoseTurnCount == 0 && p.BonusCount == 1
		}},
		{"bonus", TypedContent{RoueBonusCount: ip309(4)}, func(p RoueWheelParams) bool {
			return p.MaxPoints == 1000 && p.Step == 50 && p.BankruptCount == 2 && p.LoseTurnCount == 1 && p.BonusCount == 4
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, fb := eff309(t, c.tc, g)
			if fb || !c.want(p) {
				t.Fatalf("got %+v fb=%v", p, fb)
			}
		})
	}
}

// 0 personnalisé ≠ absent, pour chaque compteur ; absent suit le global.
func TestRoueEffective309_ZeroDistinctFromAbsent(t *testing.T) {
	g := glob309(1000, 50, 2, 1, 2)
	p, _ := eff309(t, TypedContent{RoueWheelBankruptCount: ip309(0), RoueWheelLoseTurnCount: ip309(0), RoueBonusCount: ip309(0)}, g)
	if p.BankruptCount != 0 || p.LoseTurnCount != 0 || p.BonusCount != 0 {
		t.Fatalf("0 personnalisés : %+v", p)
	}
	p, _ = eff309(t, TypedContent{}, g)
	if p.BankruptCount != 2 || p.LoseTurnCount != 1 || p.BonusCount != 2 {
		t.Fatalf("absents : %+v", p)
	}
}

// Un non-personnalisé suit le global qui change ; un personnalisé ne le suit pas.
func TestRoueEffective309_FollowsOrNotAGlobalChange(t *testing.T) {
	tc := TypedContent{RoueWheelBankruptCount: ip309(1)}
	for _, g := range []RoueWheelParams{glob309(1000, 50, 2, 1, 0), glob309(200, 20, 0, 3, 2)} {
		p, _ := eff309(t, tc, g)
		if p.BankruptCount != 1 || p.MaxPoints != g.MaxPoints || p.Step != g.Step || p.LoseTurnCount != g.LoseTurnCount || p.BonusCount != g.BonusCount {
			t.Fatalf("global %+v : got %+v", g, p)
		}
	}
}

// Les durées sont toujours globales (normalisées).
func TestRoueEffective309_DurationsGlobal(t *testing.T) {
	g := glob309(1000, 50, 2, 1, 0)
	g.SpinPeriodMs, g.SpinMaxMs, g.StopDurationMs, g.FlipIntervalMs = 3000, 12000, 5000, 900
	p, _ := eff309(t, TypedContent{RoueWheelMaxPoints: ip309(100), RoueWheelStep: ip309(10), RoueBonusCount: ip309(2)}, g)
	if p.SpinPeriodMs != 3000 || p.SpinMaxMs != 12000 || p.StopDurationMs != 5000 || p.FlipIntervalMs != 900 {
		t.Fatalf("durées : %+v", p)
	}
}

// Pas corrigé au diviseur le plus proche du maximum EFFECTIF (§10 : 100 / 30 → 25).
func TestRoueEffective309_StepCorrected(t *testing.T) {
	p, fb := eff309(t, TypedContent{RoueWheelMaxPoints: ip309(100), RoueWheelStep: ip309(30)}, glob309(1000, 50, 1, 1, 0))
	if fb || p.Step != 25 {
		t.Fatalf("got %+v fb=%v, attendu pas 25", p, fb)
	}
}

// BONUS réduits d'abord (sans repli) ; si la composition reste invalide : global entier + repli = vrai.
func TestRoueEffective309_ReduceThenFallback(t *testing.T) {
	p, fb := eff309(t, TypedContent{RoueBonusCount: ip309(4)}, glob309(63, 1, 0, 0, 0))
	if fb || p.BonusCount != 1 || RoueWheelSize(p) != 64 {
		t.Fatalf("réduction : %+v fb=%v", p, fb)
	}
	g := glob309(1000, 50, 2, 1, 1)
	invalid := map[string]TypedContent{
		"pas ≥ maximum":    {RoueWheelMaxPoints: ip309(100), RoueWheelStep: ip309(100)},
		"maximum trop bas": {RoueWheelMaxPoints: ip309(1)},
		"plus de 64 cases": {RoueWheelBankruptCount: ip309(70)},
		"compteur négatif": {RoueWheelLoseTurnCount: ip309(-1)},
	}
	for name, tc := range invalid {
		t.Run(name, func(t *testing.T) {
			tc.RoueBonusCount = ip309(3)
			p, fb := eff309(t, tc, g)
			ng, _, _ := NormalizeRoueWheelParams(g)
			ng.BonusCount = 1
			if !fb || !reflect.DeepEqual(p, ng) {
				t.Fatalf("got %+v fb=%v, attendu la roue globale entière %+v", p, fb, ng)
			}
		})
	}
}

// BONUS négatif ou > 4 dans le fichier (édité à la main) : borné, jamais d'exception.
func TestRoueEffective309_BonusClamped(t *testing.T) {
	for _, n := range []int{-3, 9} {
		p, _ := eff309(t, TypedContent{RoueBonusCount: ip309(n)}, DefaultRoueWheelParams())
		if p.BonusCount < 0 || p.BonusCount > RoueMaxBonusCount {
			t.Errorf("bonus %d → %d, hors 0–4", n, p.BonusCount)
		}
	}
}

// Égalité case par case : la composition effective sans champ construit exactement la roue de v12.0 ; avec BONUS seul, celle du LOT-1.
func TestRoueEffective309_WheelEqualityV120AndLot1(t *testing.T) {
	g := DefaultRoueWheelParams()
	p, _ := eff309(t, TypedContent{}, g)
	ng, _, _ := NormalizeRoueWheelParams(g)
	if !reflect.DeepEqual(BuildRoueWheel(p), BuildRoueWheel(ng)) {
		t.Fatal("sans champ : la roue diffère de v12.0")
	}
	for n := 1; n <= 4; n++ {
		p, _ := eff309(t, TypedContent{RoueBonusCount: ip309(n)}, g)
		lot1 := ng
		lot1.BonusCount = n
		if !reflect.DeepEqual(BuildRoueWheel(p), BuildRoueWheel(lot1)) {
			t.Fatalf("BONUS seul (%d) : la roue diffère du LOT-1", n)
		}
	}
}
