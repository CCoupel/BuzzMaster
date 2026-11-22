package game

import "testing"

func TestRoueEffectiveDev309_FollowsGlobalAndOverrides(t *testing.T) {
	global := DefaultRoueWheelParams()
	global.BonusCount = 1
	p, fb := RoueEffectiveParams(TypedContent{}, global)
	if fb || p.MaxPoints != 1000 || p.Step != 50 || p.BankruptCount != 2 || p.LoseTurnCount != 1 || p.BonusCount != 1 {
		t.Fatalf("follow global: %+v fb=%v", p, fb)
	}
	zero := 0
	max, step := 500, 100
	p, fb = RoueEffectiveParams(TypedContent{RoueWheelMaxPoints: &max, RoueWheelStep: &step, RoueWheelBankruptCount: &zero, RoueBonusCount: &zero}, global)
	if fb || p.MaxPoints != 500 || p.Step != 100 || p.BankruptCount != 0 || p.LoseTurnCount != 1 || p.BonusCount != 0 {
		t.Fatalf("overrides (0 is valid): %+v fb=%v", p, fb)
	}
	if p.SpinPeriodMs != global.SpinPeriodMs || p.StopDurationMs != global.StopDurationMs {
		t.Fatal("durations stay global")
	}
}

func TestRoueEffectiveDev309_StepCorrectedAgainstEffectiveMax(t *testing.T) {
	global := DefaultRoueWheelParams() // step 50
	max := 1200                        // 50 does not divide 1200
	p, fb := RoueEffectiveParams(TypedContent{RoueWheelMaxPoints: &max}, global)
	if fb || p.MaxPoints != 1200 || 1200%p.Step != 0 {
		t.Fatalf("%+v fb=%v", p, fb)
	}
}

func TestRoueEffectiveDev309_BonusReducedThenFallback(t *testing.T) {
	global := RoueWheelParams{MaxPoints: 3100, Step: 50} // 62 cases
	four := 4
	p, fb := RoueEffectiveParams(TypedContent{RoueBonusCount: &four}, global)
	if fb || p.BonusCount != 2 || RoueWheelSize(p) != 64 {
		t.Fatalf("reduce bonus: %+v fb=%v", p, fb)
	}
	// invalid without bonus (step >= max): whole global wheel
	step := 2000
	max := 1000
	p, fb = RoueEffectiveParams(TypedContent{RoueWheelMaxPoints: &max, RoueWheelStep: &step, RoueBonusCount: &four}, DefaultRoueWheelParams())
	if !fb || p.MaxPoints != 1000 || p.Step != 50 || p.BonusCount != 0 {
		t.Fatalf("fallback: %+v fb=%v", p, fb)
	}
}

func TestRoueEffectiveDev309_Lot1BonusOnlyKeepsLot1Wheel(t *testing.T) {
	two := 2
	p, _ := RoueEffectiveParams(TypedContent{RoueBonusCount: &two}, DefaultRoueWheelParams())
	a := BuildRoueWheel(p)
	q := DefaultRoueWheelParams()
	q.BonusCount = 2
	nq, _, _ := NormalizeRoueWheelParams(q)
	b := BuildRoueWheel(nq)
	if len(a) != 24 || len(a) != len(b) {
		t.Fatalf("size %d/%d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("case %d differs", i)
		}
	}
}
