package effects

import "testing"

func previewNative(t *testing.T, id string) Effect {
	t.Helper()
	e, ok := LoadDir(nativeFS(t), "").Get(id)
	if !ok {
		t.Fatalf("natif %s absent", id)
	}
	return e.Effect
}

func TestPreview_Chenillard_OneFramePerTickHeadMoves_Dev(t *testing.T) {
	t.Parallel()
	p := Preview(previewNative(t, "chenillard"), 4, MomentChrono)
	if p.ID != "chenillard" || p.Groups != 4 || !p.Loop || p.DurationMs != 20000 {
		t.Fatalf("en-tête : %+v", p)
	}
	if len(p.Frames) != 20 || p.Frames[0].AtMs != 0 || p.Frames[1].AtMs != 1000 || len(p.Frames[0].Groups) != 4 {
		t.Fatalf("une trame par tick de 1 s sur 20 s : %d", len(p.Frames))
	}
	// Head (urgency colour, white above 10 s) on g1 then g2: group 1 is white/255, the others the theme.
	f0, f1 := p.Frames[0].Groups, p.Frames[1].Groups
	if f0[0].Color != [3]int{255, 255, 255} || f0[1].Color != PreviewTheme || f1[1].Color != [3]int{255, 255, 255} || f1[0].Color != PreviewTheme {
		t.Fatalf("la tête avance d'un groupe par tick : %+v / %+v", f0, f1)
	}
	// Urgency comes alive: in the last 5 s of the simulated 20 s the head is red.
	last := p.Frames[len(p.Frames)-1].Groups
	red := 0
	for _, g := range last {
		if g.Color == [3]int{230, 30, 30} {
			red++
		}
	}
	if red != 1 {
		t.Fatalf("à 1 s de la fin la tête est rouge : %+v", last)
	}
}

func TestPreview_OnlyChangesAreListed_FixedEffectsOneFrame_AndCap_Dev(t *testing.T) {
	t.Parallel()
	d := Preview(previewNative(t, "degrade"), 6, MomentCalm)
	if len(d.Frames) != 1 || d.Frames[0].AtMs != 0 || len(d.Frames[0].Groups) != 6 {
		t.Fatalf("scène fixe : une seule trame : %+v", d)
	}
	pul := Preview(previewNative(t, "pulsation"), 3, MomentChrono)
	if len(pul.Frames) != 40 {
		t.Fatalf("pulsation (tick 500 ms) : 40 trames sur 20 s, %d", len(pul.Frames))
	}
	for _, g := range pul.Frames[0].Groups {
		if g != pul.Frames[0].Groups[0] {
			t.Fatal("pulsation : toute la zone au même état (g0)")
		}
	}
	fast := mustParse(t, "id: v\nnom: V\nversion: 2\nmoments: [calm]\nduree_tick_ms: 250\nvariables:\n  gA: g1; +1\npas:\n  - { g0: [\"#000000\", 0], gA: [\"#FF0000\", 100] }\n")
	if p := Preview(fast, 12, MomentCalm); len(p.Frames) > PreviewMaxFrames || len(p.Frames) != 80 {
		t.Fatalf("250 ms sur 20 s = 80 trames (plafond %d) : %d", PreviewMaxFrames, len(p.Frames))
	}
}

func TestPreview_OneRoundEffectEndsAfterItsRound_Dev(t *testing.T) {
	t.Parallel()
	e := mustParse(t, "id: u\nnom: U\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\nboucle: non\nvariables:\n  gA: g1; +1\npas:\n  - { g0: [\"#000000\", 0], gA: [\"#FF0000\", 100] }\n  - { gA: [\"#00FF00\", 100] }\n")
	p := Preview(e, 3, MomentCalm)
	if p.Loop || p.DurationMs != 2000 || len(p.Frames) != 2 {
		t.Fatalf("un tour de deux pas : 2 s, 2 trames : %+v", p)
	}
}
