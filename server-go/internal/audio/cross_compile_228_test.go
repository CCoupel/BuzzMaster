// Compilation croisée pour #228 (milestone v11.0 — plan de dev §1.6, point
// 5) : les DEUX cibles du pipeline de release (.github/workflows/release.yml,
// matrice windows/amd64 + linux/arm64, CGO_ENABLED=0) plus le repli neutre
// (output_other.go, "!linux && !windows") pour toute autre plateforme — le
// moteur son doit rester compilable PARTOUT, même là où `oto` ne cible
// personne (output.go's propre exigence, "internal/audio stays compilable
// on any platform Go itself supports, even one `oto` does not").
//
// Ce test shell-out un vrai `go build` par cible — c'est le seul moyen
// fiable de prouver qu'une plateforme compile : ni `go vet` ni `go test`
// n'évaluent les fichiers exclus par build tag pour le GOOS courant. C'est
// exactement le risque R.6 du plan ("compilation croisée cassée par la
// nouvelle dépendance") que ce test attrape avant la CI, sans attendre un
// run GitHub Actions complet.
//
// Portée volontairement limitée à `internal/audio` (pas tout `cmd/server`)
// — rapide, et c'est le seul paquet que #228 modifie derrière des build
// tags de plateforme.
//
// Convention de collision : préfixe tw228c pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package audio

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// tw228cModuleRoot locates server-go/ (the module root, containing go.mod)
// relative to this test file — never a working-directory-relative literal
// (fragile under `go test ./...` from a different directory, same
// reasoning as mainGoPath205/cmdServerDir227 elsewhere in this repo).
func tw228cModuleRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) a échoué — impossible de localiser internal/audio")
	}
	// internal/audio/<this file> -> up two levels -> server-go/
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("go.mod introuvable sous %s — le calcul de racine de module a échoué : %v", root, err)
	}
	return root
}

// tw228cCrossBuild shells out `go build` for the given GOOS/GOARCH,
// CGO_ENABLED=0 (matching release.yml exactly), targeting ONLY
// ./internal/audio/... — fails the test with the compiler's own output on
// any error.
func tw228cCrossBuild(t *testing.T, goos, goarch string) {
	t.Helper()
	root := tw228cModuleRoot(t)

	// Bornage généreux (module cache déjà chaud pour ces dépendances dans
	// tout environnement dev/CI ayant déjà exécuté cette suite une fois),
	// mais fini — un toolchain réellement cassé fait échouer le test au
	// lieu de le bloquer indéfiniment.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// BUGFIX (dev-backend, #229, coordination directe) : pas de `-o
	// <fichier>` — `./internal/audio/...` couvrait un seul paquet quand ce
	// test a été écrit ; #229 y a ajouté `internal/audio/synth`, et `go
	// build` refuse d'écrire plusieurs paquets vers un fichier de sortie
	// unique ("cannot write multiple packages to non-directory"). Sans
	// `-o`, `go build` compile et jette le résultat pour un paquet non
	// `main` — exactement la vérification voulue ici (la compilation
	// réussit ou non), sans avoir besoin de conserver un artefact.
	cmd := exec.CommandContext(ctx, "go", "build", "./internal/audio/...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"GOOS="+goos,
		"GOARCH="+goarch,
		"CGO_ENABLED=0",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compilation croisée GOOS=%s GOARCH=%s a échoué (risque plan #228 R.6) :\n%s\nerreur : %v", goos, goarch, output, err)
	}
}

func TestCrossCompile_WindowsAmd64_CITarget(t *testing.T) {
	tw228cCrossBuild(t, "windows", "amd64")
}

func TestCrossCompile_LinuxArm64_CITarget(t *testing.T) {
	tw228cCrossBuild(t, "linux", "arm64")
}

// TestCrossCompile_NeutralFallback_OtherPlatform proves output_other.go's
// build tag ("!linux && !windows") actually engages and compiles cleanly
// on a platform #228 does not target — darwin/amd64, chosen only because
// it is neither of the two real targets above; the assertion is about the
// BUILD TAG boundary, not about darwin specifically.
func TestCrossCompile_NeutralFallback_OtherPlatform(t *testing.T) {
	tw228cCrossBuild(t, "darwin", "amd64")
}
