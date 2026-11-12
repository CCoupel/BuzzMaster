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

// Contract §7.3 (complements of LOT-3): one frame per tick with its tick number and step, an
// extra frame when the urgency colour changes between two ticks, duration and chrono by moment.

func TestPreview_ChenillardOneFramePerTickWithTickAndStep_Dev(t *testing.T) {
	t.Parallel()
	p := Preview(previewNative(t, "chenillard"), 4, MomentChrono)
	if p.ID != "chenillard" || p.Moment != "chrono" || p.Groups != 4 || p.TickMs != 1000 || p.DurationMs != 20000 || p.ChronoS != 20 || !p.Loop {
		t.Fatalf("en-tête : %+v", p)
	}
	if len(p.Frames) != 20 {
		t.Fatalf("une trame par tick de 1 s sur 20 s (aux 10 s et 5 s, l'instant tombe sur un tick) : %d", len(p.Frames))
	}
	for i, f := range p.Frames {
		if f.Tick != i || f.AtMs != i*1000 || f.Step != "pas:0" || len(f.Groups) != 4 {
			t.Fatalf("trame %d : %+v", i, f)
		}
	}
	f0, f1 := p.Frames[0].Groups, p.Frames[1].Groups
	if f0[0].Color != [3]int{255, 255, 255} || f0[1].Color != PreviewTheme || f1[1].Color != [3]int{255, 255, 255} || f1[0].Color != PreviewTheme {
		t.Fatalf("la tête avance d'un groupe par tick : %+v / %+v", f0, f1)
	}
	last := p.Frames[19].Groups
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

func TestPreview_IdenticalFramesStayListed_ExtraFrameWhenUrgencyChangesBetweenTicks_Dev(t *testing.T) {
	t.Parallel()
	d := Preview(previewNative(t, "degrade"), 6, MomentCalm)
	if d.ChronoS != 0 || d.TickMs != 60000 || len(d.Frames) != 1 || d.Frames[0].Tick != 0 || d.DurationMs != 20000 {
		t.Fatalf("tick de 60 s sur 20 s : une trame (tick 0) : %+v", d)
	}
	pul := Preview(previewNative(t, "pulsation"), 3, MomentChrono)
	if len(pul.Frames) != 40 || pul.Frames[2].Tick != 2 || pul.Frames[2].Step != "pas:0" || pul.Frames[3].Step != "pas:1" {
		t.Fatalf("pulsation : 40 trames, pas:0 puis pas:1 : %d", len(pul.Frames))
	}
	// Ticks of 3 s: 10 s falls INSIDE tick 3 (9 s – 12 s) → an extra frame, same tick and same
	// step, only because the colour really changes; 15 s is the start of tick 5 (no extra frame,
	// the tick frame itself is red).
	e := mustParse(t, "id: u\nnom: U\nversion: 2\nmoments: [chrono]\nduree_tick_ms: 3000\npas:\n  - { g0: [urgence, 100] }\n")
	p := Preview(e, 2, MomentChrono)
	var extra []PreviewFrame
	for i := 1; i < len(p.Frames); i++ {
		if p.Frames[i].Tick == p.Frames[i-1].Tick {
			extra = append(extra, p.Frames[i])
		}
	}
	if len(extra) != 1 || extra[0].AtMs != 10000 || extra[0].Tick != 3 || extra[0].Step != "pas:0" || extra[0].Groups[0].Color != [3]int{255, 170, 0} {
		t.Fatalf("une trame supplémentaire à 10 s (orange) : %+v", extra)
	}
	for _, f := range p.Frames {
		if f.AtMs == 15000 && (f.Tick != 5 || f.Groups[0].Color != [3]int{230, 30, 30}) {
			t.Fatalf("tick 5 (15 s, reste 5 s) : rouge : %+v", f)
		}
	}
}

func TestPreview_StepNamesPreLastAndSkipped_ScoreSpan_Dev(t *testing.T) {
	t.Parallel()
	e := mustParse(t, "id: p\nnom: P\nversion: 2\nmoments: [score]\nduree_tick_ms: 1000\nvariables:\n  gA: g1; +1\npre:\n  { g0: [\"#0000FF\", 40] }\npas:\n  - { g9: [\"#FFFFFF\", 100] }\n  - { gA: [\"#FFFFFF\", 100] }\ndernier:\n  { g0: [\"#0000FF\", 40] }\n")
	p := Preview(e, 3, MomentScore)
	if p.DurationMs != 5000 || p.ChronoS != 0 || len(p.Frames) != 5 {
		t.Fatalf("score : 5 s, pas de chrono : %+v", p)
	}
	want := []string{"pre", "pas:1", "dernier", "pas:1", "dernier"}
	for i, f := range p.Frames {
		if f.Step != want[i] {
			t.Fatalf("tick %d : %q, attendu %q (le pas 0 est sauté, jamais « post ») : %+v", i, f.Step, want[i], p.Frames)
		}
	}
}

func TestPreview_OneRoundEffectEndsAfterItsRoundAndCapTruncates_Dev(t *testing.T) {
	t.Parallel()
	e := mustParse(t, "id: u\nnom: U\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\nboucle: non\nvariables:\n  gA: g1; +1\npas:\n  - { g0: [\"#000000\", 0], gA: [\"#FF0000\", 100] }\n  - { gA: [\"#00FF00\", 100] }\n")
	p := Preview(e, 3, MomentCalm)
	if p.Loop || p.DurationMs != 2000 || len(p.Frames) != 2 {
		t.Fatalf("un tour de deux pas : 2 s, 2 trames : %+v", p)
	}
	fast := mustParse(t, "id: v\nnom: V\nversion: 2\nmoments: [calm]\nduree_tick_ms: 250\npas:\n  - { g0: [\"#000000\", 0] }\n")
	if q := Preview(fast, 2, MomentCalm); len(q.Frames) != 80 || q.DurationMs != 20000 {
		t.Fatalf("250 ms sur 20 s = 80 trames : %d", len(q.Frames))
	}
}
