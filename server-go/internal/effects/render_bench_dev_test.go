package effects

import (
	"fmt"
	"strings"
	"testing"
)

// §12.6 — cost of Render for an effect of 24 variables (a chain of dependencies) and 40 actions
// per step, at tick 10 000 (the reader calls it at 10 Hz).
func benchSrc() string {
	var b strings.Builder
	b.WriteString("variables:\n  gA: 1; +1\n")
	for i := 1; i < 24; i++ {
		fmt.Fprintf(&b, "  g%c: g%c+1\n", 'A'+i, 'A'+i-1)
	}
	b.WriteString("pas:\n  - { g0: [theme, 100]")
	for i := 0; i < 24; i++ {
		fmt.Fprintf(&b, ", g%c: [urgence, %d]", 'A'+i, 10+i)
	}
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&b, ", g%d: [scene, 50]", i)
	}
	b.WriteString(", g*: [theme, 20], gA+1: [theme, 30], gB+1: [theme, 30] }\n  - { g0: [theme, 10] }\n")
	return fx(b.String())
}

func BenchmarkRender_24Vars40Actions_Tick10000(b *testing.B) {
	e, errs := Parse([]byte(benchSrc()))
	if len(errs) > 0 {
		b.Fatal(errs)
	}
	ctx := Context{Groups: 12, Moment: MomentChrono}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Render(e, ctx, 10000)
	}
}
