// Test d'exhaustivité (#205, milestone v10.0.0 — contracts/lighting.md §7,
// critère d'acceptance CA3 — _work/reports/planner-v10-plan-205-20260902-
// 203000.md).
//
// Ce que ce test doit attraper : un nouveau site d'émission LED ajouté dans
// main.go SANS décision d'ambiance dans ambianceSiteRegistry
// (cmd/server/ambiance.go) — "les buzzers s'allument, la salle non", la
// classe de défaut la plus probable du milestone.
//
// Construction imposée par le contrat (§7) : analyse AST (go/parser +
// go/ast, stdlib — voir aussi TestCA7_NoNewThirdPartyDependency dans
// ambiance_acceptance_test.go), pas une expression régulière. On collecte
// l'ensemble des paires (nom de la fonction englobante, nom de la fonction
// LED appelée) pour tout appel dont le sélecteur commence par "sendLEDSet",
// on exclut les paires dont la fonction englobante appartient elle-même à
// la couche de rendu (ambianceIsRenderingLayer, cmd/server/ambiance.go —
// contract §1 : ces sites ne sont jamais à instrumenter, et le commentaire
// sur ambianceIsRenderingLayer dit explicitement "the exhaustiveness test
// skips these enclosing functions"), puis on compare l'ensemble restant au
// registre déclaré dans le code (ambianceSiteRegistry).
//
// Indexé sur les NOMS de fonction, jamais sur les numéros de ligne (contract
// §6 : "la colonne 'fonction englobante' est l'identité stable du site — pas
// le numéro de ligne, qui change à chaque édition").
//
// Limite assumée et documentée ici (contract §7) : un second appel identique
// dans une fonction déjà enregistrée ne fait PAS échouer ce test — c'est le
// même site sémantique émettant le même genre d'événement. Vérifié par
// TestAmbianceExhaustiveness_DuplicateCallInSameFuncCollapsesToOnePair.
// Seule l'apparition d'une FONCTION ou d'un TYPE D'APPEL LED nouveau doit
// faire rougir ce test.
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// mainGoPath205 locates cmd/server/main.go relative to this test file, never
// via a working-directory-relative literal (fragile under `go test ./...`
// from a different directory).
func mainGoPath205(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) a échoué — impossible de localiser cmd/server")
	}
	return filepath.Join(filepath.Dir(thisFile), "main.go")
}

// scanLEDSitePairs205 parses the given Go source (from disk when src is nil,
// matching go/parser.ParseFile's own convention — otherwise from src
// directly, used by the duplicate-call unit test below) and returns every
// (enclosing function, sendLEDSet* selector) pair found ANYWHERE in each
// top-level function's syntax tree, including inside nested closures —
// ast.Inspect naturally recurses into a *ast.FuncLit's body too, which is
// exactly what attributes a site inside a callback (e.g. setupCallbacks's
// OnRafaleTeamsChanged closure, contract §6) to its enclosing NAMED
// function, never to the anonymous literal itself.
func scanLEDSitePairs205(t *testing.T, filename string, src interface{}) map[ambianceSite]bool {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		t.Fatalf("go/parser a échoué sur %s : %v", filename, err)
	}

	pairs := map[ambianceSite]bool{}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		enclosing := fd.Name.Name
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var name string
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			case *ast.Ident:
				name = fn.Name
			default:
				return true
			}
			if strings.HasPrefix(name, "sendLEDSet") {
				pairs[ambianceSite{Func: enclosing, LED: name}] = true
			}
			return true
		})
	}
	return pairs
}

// ambianceSitesDiff205 compares the set of (enclosing, LED) pairs actually
// found in the source against the registry, excluding the rendering layer
// from BOTH sides (contract §1 — a rendering-layer function is never a site
// to wire, whether or not the registry happens to document one for its own
// internal calls, e.g. sendLEDSetComet's trailing AfterFunc). Returns the
// pairs found but unregistered ("missing"), and registered pairs with no
// matching real call left in the source ("extra" — a stale registry entry).
func ambianceSitesDiff205(found map[ambianceSite]bool, registry map[ambianceSite]ambianceDecision) (missing, extra []string) {
	for site := range found {
		if ambianceIsRenderingLayer(site.Func) {
			continue
		}
		if _, ok := registry[site]; !ok {
			missing = append(missing, site.Func+" -> "+site.LED)
		}
	}
	for site := range registry {
		if ambianceIsRenderingLayer(site.Func) {
			continue
		}
		if !found[site] {
			extra = append(extra, site.Func+" -> "+site.LED)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}

// TestAmbianceExhaustiveness_MainGoMatchesRegistry is CA3 itself: the real
// check that must run on every build.
func TestAmbianceExhaustiveness_MainGoMatchesRegistry(t *testing.T) {
	found := scanLEDSitePairs205(t, mainGoPath205(t), nil)
	missing, extra := ambianceSitesDiff205(found, ambianceSiteRegistry)

	for _, m := range missing {
		t.Errorf("ambiance: site LED sans décision d'ambiance — %s\n"+
			"  Ajoute une entrée dans ambianceSiteRegistry (cmd/server/ambiance.go) :\n"+
			"  soit NotifyState/NotifyPulse, soit NoAmbiance avec le motif.\n"+
			"  Voir contracts/lighting.md §6.", m)
	}
	if len(extra) > 0 {
		t.Errorf("ambiance: entrée(s) du registre sans site réel correspondant dans main.go (registre périmé) : %s\n"+
			"  Retire l'entrée ou corrige le nom de fonction dans ambianceSiteRegistry (contracts/lighting.md §6/§7).",
			strings.Join(extra, ", "))
	}
}

// TestAmbianceExhaustiveness_CatchesUnregisteredSite proves the comparison
// mechanism itself would catch a real regression (CA3's own verification
// clause: "vérifié en ajoutant temporairement une fonction appelant
// sendLEDSetAllBuzzers : le test doit rougir et nommer la fonction
// fautive"). It does so WITHOUT touching main.go — main.go's real, current
// pairs are scanned exactly as in the test above, then ONE synthetic
// unregistered pair is injected into the found-set, exactly as if a future
// edit had added `a.sendLEDSetAllBuzzers(...)` inside a brand-new
// handleNouveauTruc function. The real registry (never modified) must not
// contain this pair, so the diff must name it.
func TestAmbianceExhaustiveness_CatchesUnregisteredSite(t *testing.T) {
	found := scanLEDSitePairs205(t, mainGoPath205(t), nil)
	injected := ambianceSite{Func: "handleNouveauTruc", LED: "sendLEDSetAllBuzzers"}
	if found[injected] {
		t.Fatalf("setup invalide : %+v existe déjà réellement dans main.go, choisir un autre nom synthétique", injected)
	}
	found[injected] = true

	missing, _ := ambianceSitesDiff205(found, ambianceSiteRegistry)
	want := injected.Func + " -> " + injected.LED
	for _, m := range missing {
		if m == want {
			return
		}
	}
	t.Fatalf("le mécanisme de comparaison n'a pas nommé le site injecté %q parmi les manquants %v — CA3 ne serait pas détecté", want, missing)
}

// TestAmbianceExhaustiveness_DuplicateCallInSameFuncCollapsesToOnePair
// documents the assumed limit stated in contracts/lighting.md §7: a second
// identical call inside an already-registered function does not turn into a
// second "missing" entry — same enclosing function, same LED call, same
// (Func, LED) key, because the found-set is a set (map), not a multiset.
func TestAmbianceExhaustiveness_DuplicateCallInSameFuncCollapsesToOnePair(t *testing.T) {
	const src = `package main

func handleFlipMemoryCard(a *App) {
	a.sendLEDSetAllBuzzers()
	if true {
		a.sendLEDSetAllBuzzers()
	}
	a.sendLEDSetAllBuzzers()
}
`
	pairs := scanLEDSitePairs205(t, "synthetic_205.go", src)
	if len(pairs) != 1 {
		t.Fatalf("3 appels identiques dans UNE fonction doivent produire 1 seule paire (limite assumée §7), got %d: %v", len(pairs), pairs)
	}
	if !pairs[(ambianceSite{Func: "handleFlipMemoryCard", LED: "sendLEDSetAllBuzzers"})] {
		t.Fatalf("paire attendue absente : %v", pairs)
	}
}

// TestAmbianceExhaustiveness_ClosureIsAttributedToEnclosingNamedFunc mirrors
// the real setupCallbacks(OnRafaleTeamsChanged) site (contract §6): a
// sendLEDSet* call written inside an anonymous func literal assigned as a
// callback must be attributed to the outer NAMED function, never treated as
// its own untracked site.
func TestAmbianceExhaustiveness_ClosureIsAttributedToEnclosingNamedFunc(t *testing.T) {
	const src = `package main

func setupCallbacks(a *App) {
	a.engine.OnRafaleTeamsChanged = func(team string) {
		a.sendLEDSetRafaleTeams(team)
	}
}
`
	pairs := scanLEDSitePairs205(t, "synthetic_205.go", src)
	want := ambianceSite{Func: "setupCallbacks", LED: "sendLEDSetRafaleTeams"}
	if !pairs[want] {
		t.Fatalf("l'appel dans la closure doit être attribué à la fonction englobante nommée %+v, got %v", want, pairs)
	}
}

// TestAmbianceExhaustiveness_RenderingLayerCallsAreSkipped proves that a
// sendLEDSet* call whose ENCLOSING function is itself part of the rendering
// layer (e.g. sendLEDSetReveal looping over the per-buzzer sendLEDSet, or
// sendLEDSetComet's trailing restore calling sendLEDSetAllBuzzers) never
// produces a "missing" diagnostic, per contract §1's founding rule — even
// though scanLEDSitePairs205 itself does not filter anything (the filtering
// happens in ambianceSitesDiff205, exercised here directly against an EMPTY
// registry to isolate the rendering-layer exclusion from any real registry
// content).
func TestAmbianceExhaustiveness_RenderingLayerCallsAreSkipped(t *testing.T) {
	found := map[ambianceSite]bool{
		{Func: "sendLEDSetReveal", LED: "sendLEDSet"}:                      true, // couche de rendu appelant le goulot unique
		{Func: "sendLEDSetAllBuzzers", LED: "sendLEDSet"}:                  true,
		{Func: "handleNotRenderingButUnregistered", LED: "sendLEDSetStop"}: true,
	}
	missing, _ := ambianceSitesDiff205(found, map[ambianceSite]ambianceDecision{})
	if len(missing) != 1 || missing[0] != "handleNotRenderingButUnregistered -> sendLEDSetStop" {
		t.Fatalf("seule la paire non-couche-de-rendu doit apparaître comme manquante, got %v", missing)
	}
}
