// #290 (v11.6.0, LOT-4) — TEST-GARDE CA9bis : plus aucune lecture directe des réglages de comportement dans
// `config.Get()` hors de l'adaptateur `FromConfig`. Source normative : contracts/ambiance-settings.md §5.2
// (« Interdit après LOT-4 : toute lecture de `config.Get().Sound.Muted`, `.CuesDisabled`, `.CountdownDisabled`,
// `.CountdownOnce` et de `Lighting.DefaultMode` hors de l'adaptateur `FromConfig` ») ; patron AST de sound.md §6.2
// (go/parser + go/ast, jamais une regex ; indexé sur les NOMS de fonction).
//
// Ce que ce test attrape : un site qui lirait encore `Sound.Muted` & co. (ou `Lighting.DefaultMode`) directement et
// contournerait donc la manche ET le geste direct. Les ÉCRITURES (affectation) ne sont pas des lectures : le geste
// direct réécrit le global hors surcharge (D1′). Préfixe tw290g.
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Champs de `config.SoundConfig` / `config.LightingConfig` désormais lus RÉSOLUS (tw290g : liste fermée du contrat).
var tw290gSoundFields = map[string]bool{"Muted": true, "CuesDisabled": true, "CountdownDisabled": true, "CountdownOnce": true}

const tw290gLightingField = "DefaultMode"

// Fonctions autorisées à lire ces champs : l'adaptateur config → Settings (FromConfig) et, symétriquement, la
// conversion Settings → patch de config (ToConfigPatch). Aucune autre.
var tw290gAllowedFunc = regexp.MustCompile(`(?i)^(fromconfig|toconfigpatch)$`)

type tw290gHit struct{ file, fn, expr string }

// tw290gScan renvoie les LECTURES interdites d'un fichier source (src=nil ⇒ lu sur disque).
func tw290gScan(t *testing.T, filename string, src interface{}) []tw290gHit {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		t.Fatalf("go/parser a échoué sur %s : %v", filename, err)
	}
	var hits []tw290gHit
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		fnName := fd.Name.Name
		lhs := map[ast.Node]bool{} // sélecteurs en position d'ÉCRITURE
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if as, ok := n.(*ast.AssignStmt); ok {
				for _, l := range as.Lhs {
					lhs[l] = true
				}
			}
			return true
		})
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || lhs[sel] {
				return true
			}
			base, isSel := sel.X.(*ast.SelectorExpr)
			if !isSel {
				return true
			}
			forbidden := (base.Sel.Name == "Sound" && tw290gSoundFields[sel.Sel.Name]) ||
				(base.Sel.Name == "Lighting" && sel.Sel.Name == tw290gLightingField)
			if forbidden && !tw290gAllowedFunc.MatchString(fnName) {
				hits = append(hits, tw290gHit{filename, fnName, base.Sel.Name + "." + sel.Sel.Name})
			}
			return true
		})
	}
	return hits
}

func tw290gDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) a échoué")
	}
	return filepath.Dir(thisFile)
}

func tw290gScanDir(t *testing.T, dir string) []tw290gHit {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("lecture de %s : %v", dir, err)
	}
	var all []tw290gHit
	scanned := 0
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		scanned++
		all = append(all, tw290gScan(t, filepath.Join(dir, n), nil)...)
	}
	if scanned == 0 {
		t.Fatalf("aucun fichier .go non-test dans %s — le scan est cassé", dir)
	}
	return all
}

// CA9bis proprement dit : cmd/server (notifySound, question_sound, countdown_sound, ambiance…) et internal/ambiance.
func TestCA9bis290_NoDirectReadOfBehaviourSettings(t *testing.T) {
	root := tw290gDir(t)
	var hits []tw290gHit
	hits = append(hits, tw290gScanDir(t, root)...)
	hits = append(hits, tw290gScanDir(t, filepath.Join(root, "..", "..", "internal", "ambiance"))...)
	sort.Slice(hits, func(i, j int) bool { return hits[i].file+hits[i].fn < hits[j].file+hits[j].fn })
	for _, h := range hits {
		t.Errorf("CA9bis : lecture directe de config.%s dans %s (%s) — passer par a.ambianceSettings() "+
			"(contracts/ambiance-settings.md §5.2) ; seul FromConfig/ToConfigPatch lit le global", h.expr, filepath.Base(h.file), h.fn)
	}
}

// --- le garde sait détecter (sinon un garde cassé passerait au vert pour de mauvaises raisons) ---

func TestCA9bis290_Guard_DetectsViolations_AllowsAdapterAndWrites(t *testing.T) {
	const src = `package main
func (a *App) badReadMuted() bool          { return config.Get().Sound.Muted }
func (a *App) badReadCues() bool           { return config.Get().Sound.CuesDisabled["x"] }
func (a *App) badReadCountdown() bool      { return a.config.Sound.CountdownDisabled || a.config.Sound.CountdownOnce }
func (a *App) badReadMode() string         { return config.Get().Lighting.DefaultMode }
func FromConfig(cfg config.Config) int     { _ = cfg.Sound.Muted; _ = cfg.Lighting.DefaultMode; return 0 }
func ToConfigPatch() int                   { _ = config.Get().Sound.CuesDisabled; return 0 }
func (a *App) writeIsNotARead() {
	cfg := config.Get()
	cfg.Sound.Muted = true
	cfg.Sound.CuesDisabled = map[string]bool{}
	cfg.Lighting.DefaultMode = "ON"
}
func (a *App) unrelated() bool             { return config.Get().Sound.Enabled && config.Get().Sound.Device == "" }
`
	hits := tw290gScan(t, "fake.go", src)
	got := map[string]bool{}
	for _, h := range hits {
		got[h.fn] = true
	}
	for _, want := range []string{"badReadMuted", "badReadCues", "badReadCountdown", "badReadMode"} {
		if !got[want] {
			t.Errorf("le garde doit signaler %s", want)
		}
	}
	for _, ok := range []string{"FromConfig", "ToConfigPatch", "writeIsNotARead", "unrelated"} {
		if got[ok] {
			t.Errorf("le garde ne doit pas signaler %s (adaptateur, écriture ou champ hors périmètre : sound.enabled/device restent lisibles)", ok)
		}
	}
	if len(hits) != 5 {
		t.Errorf("5 lectures interdites attendues (muted, cues, countdown×2, mode), got %d : %v", len(hits), hits)
	}
}
