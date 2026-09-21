// Test-garde AST des sites sonores (#227, milestone v11.0 —
// contracts/sound.md §6.2, plan de dev §3.5/§C.1,
// _work/reports/plan-dev-227-20260921-102500.md). Symétrique à
// cmd/server/ambiance_sites_test.go (#205, contracts/lighting.md §7), avec
// une exigence supplémentaire propre à ce contrat : vérifier la FRONTIÈRE
// normative du §3.3/§6.1 — aucun site de conduite lumineuse manuelle, de
// célébration SCORE, de reconnexion du pont, de cycle de vie de l'écrivain
// ou de pulsation du chronomètre ne doit jamais appeler le fan-out sonore.
//
// Ce que ce test attrape :
//   - un nouveau site de jeu qui appelle a.notifySound(...) SANS entrée dans
//     soundSiteRegistry (cmd/server/sound.go) — registre périmé ou site
//     oublié ;
//   - une entrée du registre sans appel réel correspondant (registre
//     stale) ;
//   - LE DÉFAUT CRITIQUE identifié par le plan (§3.3, risque R.1) : un appel
//     à notifySound() glissé dans runChronoPulse (pulsation 100 ms),
//     setLightingMode/setLightingFlash/runLightingFlash (sélecteur/Flash
//     manuel), runScoreFlash (célébration lumineuse, pas un nouvel
//     événement), buildHueDriver (callback OnReconnect — resynchronisation,
//     pas un événement), startAmbianceWriter/reconfigureAmbiance (cycle de
//     vie de l'écrivain) — "un remplacement aveugle rendrait le serveur
//     inutilisable" (contract §6.1).
//
// Construction imposée par le contrat (§6.2, calquée sur §7 de
// lighting.md) : analyse AST (go/parser + go/ast, stdlib), jamais une
// expression régulière. On collecte l'ensemble des fonctions englobantes
// qui appellent le sélecteur exact "notifySound" (UN SEUL point d'entrée
// fan-out, contrairement au préfixe sendLEDSet* qui en a plusieurs côté
// lumière — voir contracts/sound.md §5.1/§6.1 et la coordination avec
// dev-backend référencée dans internal/audio/engine_test.go), scanne TOUT
// cmd/server/*.go (pas seulement main.go : sound.go, ambiance.go et
// ambiance_override.go comptent tous), et compare :
//
//  1. l'ensemble trouvé au registre déclaré `soundSiteRegistry`
//     (cmd/server/sound.go, Lot B.4) — exhaustivité + absence d'entrée
//     périmée (même mécanique que CA3) ;
//  2. l'ensemble trouvé à la liste FERMÉE des fonctions interdites (§6.1) —
//     intersection qui doit être vide.
//
// Indexé sur les NOMS de fonction, jamais sur les numéros de ligne (même
// convention que contract lighting.md §6 / ambiance_sites_test.go).
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// soundSiteScanSelector is the sound fan-out's single entry point (contract
// §5.1: "aucun `if` autour d'un `PlayCue`" — le site de jeu appelle
// inconditionnellement ce sélecteur, jamais audio.Engine.PlayCue
// directement). Un seul nom recherché, contrairement au préfixe
// "sendLEDSet" côté lumière.
const soundSiteScanSelector = "notifySound"

// soundForbiddenFuncs227 is the CLOSED list of enclosing functions that must
// NEVER call the sound fan-out (contract §6.1's five forbidden families).
// A closure assigned as a field (e.g. buildHueDriver's OnReconnect) is
// attributed to its enclosing NAMED function by AST construction (see
// scanSoundSitePairs227 below), so "buildHueDriver" alone covers the
// OnReconnect callback without listing it separately.
var soundForbiddenFuncs227 = map[string]string{
	"setLightingMode":     "conduite lumineuse manuelle (sélecteur ON/AUTO/OFF)",
	"setLightingFlash":    "conduite lumineuse manuelle (bascule Flash)",
	"runLightingFlash":    "conduite lumineuse manuelle (boucle Flash)",
	"runScoreFlash":       "célébration lumineuse SCORE — ré-émission, pas un nouvel événement de jeu",
	"buildHueDriver":      "reconnexion du pont (callback OnReconnect) — resynchronisation, pas un événement nouveau",
	"startAmbianceWriter": "cycle de vie de l'écrivain lumière — premier rafraîchissement au démarrage",
	"reconfigureAmbiance": "cycle de vie de l'écrivain lumière — rafraîchissement à la reconfiguration",
	"runChronoPulse":      "pulsation du chronomètre — notifie toutes les 100 ms (contract §6.1, ABSOLUMENT JAMAIS)",
}

// cmdServerDir227 locates the cmd/server directory relative to this test
// file, never via a working-directory-relative literal.
func cmdServerDir227(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) a échoué — impossible de localiser cmd/server")
	}
	return filepath.Dir(thisFile)
}

// scanSoundSitePairs227 parses every non-test .go file directly under dir
// (from disk when files is nil — matching go/parser's own convention,
// otherwise from the given filename/src pair directly, used by the unit
// tests below) and returns the set of enclosing-function names that call
// the selector `notifySound` ANYWHERE in their syntax tree, including
// inside nested closures (ast.Inspect naturally recurses into a
// *ast.FuncLit's body too — exactly what attributes buildHueDriver's
// OnReconnect closure to its enclosing NAMED function).
func scanSoundSitePairs227(t *testing.T, filename string, src interface{}) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		t.Fatalf("go/parser a échoué sur %s : %v", filename, err)
	}
	found := map[string]bool{}
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
			if name == soundSiteScanSelector {
				found[enclosing] = true
			}
			return true
		})
	}
	return found
}

// scanSoundSitesInDir227 runs scanSoundSitePairs227 over every non-test .go
// file directly in dir and unions the results.
func scanSoundSitesInDir227(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("lecture de %s : %v", dir, err)
	}
	found := map[string]bool{}
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		scanned++
		for fn := range scanSoundSitePairs227(t, filepath.Join(dir, name), nil) {
			found[fn] = true
		}
	}
	if scanned == 0 {
		t.Fatal("aucun fichier .go non-test trouvé dans cmd/server — le scan est cassé")
	}
	return found
}

// soundSitesDiff227 compares the set of enclosing-function names actually
// found in the source against the registry. Returns the functions found but
// unregistered ("missing"), and registered functions with no matching real
// call left in the source ("extra" — a stale registry entry).
func soundSitesDiff227(found map[string]bool, registry map[soundSite]soundDecision) (missing, extra []string) {
	for fn := range found {
		if _, ok := registry[soundSite{Func: fn}]; !ok {
			missing = append(missing, fn)
		}
	}
	for site := range registry {
		if !found[site.Func] {
			extra = append(extra, site.Func)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}

// TestSoundSites_RegistryMatchesRealCallSites is C.1 itself — the real
// check that must run on every build. It intentionally does NOT assume
// which functions call notifySound (that is Lot B's design decision, not
// this test's) — it only enforces that whatever the source actually does,
// soundSiteRegistry (cmd/server/sound.go) documents it exactly, exhaustively,
// without stale entries. Same mechanic as
// TestAmbianceExhaustiveness_MainGoMatchesRegistry (CA3, #205).
func TestSoundSites_RegistryMatchesRealCallSites(t *testing.T) {
	found := scanSoundSitesInDir227(t, cmdServerDir227(t))
	missing, extra := soundSitesDiff227(found, soundSiteRegistry)

	for _, m := range missing {
		t.Errorf("sound: site sonore sans décision enregistrée — %s\n"+
			"  Ajoute une entrée dans soundSiteRegistry (cmd/server/sound.go) documentant la cue jouée.\n"+
			"  Voir contracts/sound.md §6.2.", m)
	}
	if len(extra) > 0 {
		t.Errorf("sound: entrée(s) du registre sans appel réel à notifySound() correspondant dans cmd/server (registre périmé) : %s\n"+
			"  Retire l'entrée ou corrige le nom de fonction dans soundSiteRegistry.",
			strings.Join(extra, ", "))
	}
}

// TestSoundSites_CatchesUnregisteredSite proves the comparison mechanism
// itself would catch a real regression, without touching real source: the
// real found-set is scanned exactly as above, then ONE synthetic
// unregistered function name is injected, exactly as if a future edit had
// added `a.notifySound(audio.CueGagne)` inside a brand-new
// handleNouveauTrucSonore function.
func TestSoundSites_CatchesUnregisteredSite(t *testing.T) {
	found := scanSoundSitesInDir227(t, cmdServerDir227(t))
	const injected = "handleNouveauTrucSonore227"
	if found[injected] {
		t.Fatalf("setup invalide : %q existe déjà réellement dans cmd/server, choisir un autre nom synthétique", injected)
	}
	if _, ok := soundSiteRegistry[soundSite{Func: injected}]; ok {
		t.Fatalf("setup invalide : %q est déjà dans soundSiteRegistry, choisir un autre nom synthétique", injected)
	}
	found[injected] = true

	missing, _ := soundSitesDiff227(found, soundSiteRegistry)
	for _, m := range missing {
		if m == injected {
			return
		}
	}
	t.Fatalf("le mécanisme de comparaison n'a pas nommé le site injecté %q parmi les manquants %v — la régression ne serait pas détectée", injected, missing)
}

// TestSoundSites_DuplicateCallInSameFuncCollapsesToOneSite documents the
// assumed limit (same as CA3/§7 of lighting.md): several notifySound calls
// inside the SAME enclosing function (e.g. onPhaseStarted choosing between
// CueDepart/CueEntracteDebut on different branches) collapse to one entry —
// same enclosing function, same selector, same set key. This is expected:
// a function may legitimately call notifySound on different branches for
// different cues; the registry documents the FUNCTION, not every branch.
func TestSoundSites_DuplicateCallInSameFuncCollapsesToOneSite(t *testing.T) {
	const src = `package main

func onPhaseStarted(a *App) {
	if true {
		a.notifySound(audio.CueDepart)
	} else {
		a.notifySound(audio.CueEntracteDebut)
	}
}
`
	found := scanSoundSitePairs227(t, "synthetic_227.go", src)
	if len(found) != 1 || !found["onPhaseStarted"] {
		t.Fatalf("2 appels distincts dans UNE fonction doivent produire 1 seul site (même limite assumée que CA3), got %v", found)
	}
}

// TestSoundSites_ClosureIsAttributedToEnclosingNamedFunc mirrors the real
// buildHueDriver(OnReconnect) site: a notifySound call written inside an
// anonymous func literal assigned as a struct field/callback must be
// attributed to the outer NAMED function, never treated as its own
// untracked site — this is exactly what makes the forbidden-family check
// below able to name "buildHueDriver" without special-casing OnReconnect.
func TestSoundSites_ClosureIsAttributedToEnclosingNamedFunc(t *testing.T) {
	const src = `package main

func buildHueDriver(a *App) {
	_ = hue.Config{
		OnReconnect: func() {
			a.notifySound(audio.CueGagne) // hypothétique régression : ne doit JAMAIS exister réellement
		},
	}
}
`
	found := scanSoundSitePairs227(t, "synthetic_227.go", src)
	if !found["buildHueDriver"] {
		t.Fatalf("l'appel dans la closure OnReconnect doit être attribué à la fonction englobante nommée buildHueDriver, got %v", found)
	}
}

// ---------------------------------------------------------------------------
// Frontière normative (contract §3.3/§6.1) — LE test qui protège contre le
// risque R.1 du plan ("un remplacement aveugle rendrait le serveur
// inutilisable"). Sur le VRAI code source, pas un synthétique : si l'une de
// ces huit fonctions appelle un jour notifySound(), ce test doit rougir et
// la nommer.
// ---------------------------------------------------------------------------

func TestSoundSites_ForbiddenFamiliesNeverCallSoundFanOut(t *testing.T) {
	found := scanSoundSitesInDir227(t, cmdServerDir227(t))

	var violations []string
	for fn, reason := range soundForbiddenFuncs227 {
		if found[fn] {
			violations = append(violations, fn+" ("+reason+")")
		}
	}
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Errorf("contract §6.1 violé — fonction(s) de conduite lumineuse/cycle de vie/pulsation appelant le fan-out sonore, alors qu'elles ne doivent JAMAIS le faire :\n%s\n"+
			"Un remplacement aveugle de a.ambiance() par le fan-out rendrait le serveur inutilisable (la pulsation du chronomètre déclencherait dix sons par seconde).",
			strings.Join(violations, "\n"))
	}
}

// TestSoundSites_ForbiddenCheck_CatchesInjectedViolation proves the
// forbidden-family mechanism would actually catch a real regression — a
// synthetic call to notifySound() injected inside runChronoPulse's body,
// exactly the failure mode the plan's risk R.1 describes, WITHOUT touching
// real source.
func TestSoundSites_ForbiddenCheck_CatchesInjectedViolation(t *testing.T) {
	const src = `package main

func runChronoPulse(a *App) {
	a.notifySound(audio.CueGagne) // régression hypothétique : dix sons par seconde
}
`
	found := scanSoundSitePairs227(t, "synthetic_227.go", src)
	if !found["runChronoPulse"] {
		t.Fatal("setup invalide : le scan n'a pas trouvé l'appel synthétique dans runChronoPulse")
	}
	reason, forbidden := soundForbiddenFuncs227["runChronoPulse"]
	if !forbidden {
		t.Fatal("setup invalide : runChronoPulse doit figurer dans soundForbiddenFuncs227")
	}
	// Reproduit la détection réelle de TestSoundSites_ForbiddenFamiliesNeverCallSoundFanOut,
	// isolée au seul nom injecté : found[fn] && forbidden => violation à rapporter.
	if !found["runChronoPulse"] {
		t.Fatalf("le mécanisme n'a pas détecté la violation injectée dans runChronoPulse (%s)", reason)
	}
}

// TestSoundForbiddenFuncs227_AllTargetsStillExistInSource guards the guard:
// each name in soundForbiddenFuncs227 must be a real, currently-declared
// function in cmd/server — otherwise a rename (e.g. runScoreFlash renamed
// during a future refactor) would silently stop being checked at all, and
// TestSoundSites_ForbiddenFamiliesNeverCallSoundFanOut would pass for the
// wrong reason (nothing named that way exists to violate the rule).
func TestSoundForbiddenFuncs227_AllTargetsStillExistInSource(t *testing.T) {
	dir := cmdServerDir227(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("lecture de %s : %v", dir, err)
	}
	declared := map[string]bool{}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("go/parser a échoué sur %s : %v", name, err)
		}
		for _, decl := range f.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok {
				declared[fd.Name.Name] = true
			}
		}
	}
	var missing []string
	for fn := range soundForbiddenFuncs227 {
		if !declared[fn] {
			missing = append(missing, fn)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("soundForbiddenFuncs227 référence des fonctions introuvables dans cmd/server (renommées ?) : %s — "+
			"la frontière §6.1 ne serait plus vérifiée pour elles", strings.Join(missing, ", "))
	}
}
