package effects

// #210 (v11.8.0, LOT-2) — compatibilité §12.4 : les 5 natifs rendent les MÊMES trames qu'en v11.7.0 pour N = 1 à 12.
// Source : contracts/light-effects.md §12.4, §12.6, §11.9. Référence figée : testdata/natives_v1170_golden.json, générée AVANT toute
// modification du moteur par `TW210_GEN_GOLDEN=1 go test ./internal/effects -run TestTW210_Compat_GenerateGolden`
// (à ne régénérer JAMAIS après le LOT-2 : c'est la preuve de non-régression). Tags : critical.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

const tw210GoldenFile = "testdata/natives_v1170_golden.json"
const tw210GoldenTicks = 60

// tw210Trace : une chaîne par tick, un jeton « RRGGBB@intensité » par groupe (état effectif), pour N = 1…12.
func tw210Trace(t *testing.T, e Effect) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for n := 1; n <= 12; n++ {
		c := ctxN(n)
		c.Moment = ""
		var lines []string
		for tick := 0; tick < tw210GoldenTicks; tick++ {
			v := renderView(e, c, tick)
			var b strings.Builder
			for p := 1; p <= n; p++ {
				l := v.effective(p)
				if l.Absent {
					b.WriteString("_ ")
				} else {
					fmt.Fprintf(&b, "%02X%02X%02X@%d ", l.Color[0], l.Color[1], l.Color[2], l.Intensity)
				}
			}
			lines = append(lines, strings.TrimSpace(b.String()))
		}
		out[fmt.Sprint(n)] = lines
	}
	return out
}

func tw210NativeTraces(t *testing.T) map[string]map[string][]string {
	t.Helper()
	all := map[string]map[string][]string{}
	for _, id := range []string{"pulsation", "chenillard", "chenillard-traine", "couleur-equipe", "degrade"} {
		e, ok := loadFS(nativeFS(t), t.TempDir()).effect(id)
		if !ok {
			t.Fatalf("natif %s absent", id)
		}
		all[id] = tw210Trace(t, e)
	}
	return all
}

func TestTW210_Compat_GenerateGolden(t *testing.T) {
	if os.Getenv("TW210_GEN_GOLDEN") == "" {
		t.Skip("génération manuelle uniquement")
	}
	b, _ := json.MarshalIndent(tw210NativeTraces(t), "", " ")
	if err := os.WriteFile(tw210GoldenFile, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTW210_Critical_Compat_FiveNatives_SameFramesAsV1170_N1to12(t *testing.T) {
	raw, err := os.ReadFile(tw210GoldenFile)
	if err != nil {
		t.Fatalf("référence v11.7.0 absente : %v", err)
	}
	var want map[string]map[string][]string
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	got := tw210NativeTraces(t)
	for id, byN := range want {
		for n, lines := range byN {
			for tick, w := range lines {
				if g := got[id][n][tick]; g != w {
					t.Errorf("%s N=%s tick %d : %q, v11.7.0 : %q", id, n, tick, g, w)
					break
				}
			}
		}
	}
}
