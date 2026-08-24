// Tests for #183 (A-T1) — filet de non-régression du refactor de dispatch
// MEMOTION (A-B1 : constantes de sous-phase/état de carte, A-B2 : registre
// exportable des types).
//
// Ces tests couvrent les critères d'acceptance #183 qui portent sur le Go :
//   - "Aucune chaîne libre GRID/SELECTED/QUESTION/REVEAL/MEMORIZE/UNPLAYED/
//     DONE dans le Go non-test" — TestNoFreeMotionLiteralStrings_183.
//   - AllQuestionTypes() retourne exactement les 5 types, support de tous les
//     tests d'exhaustivité ultérieurs (#184/B-B8) — TestAllQuestionTypes_183.
//   - Équivalence des valeurs sérialisées des constantes MotionSubPhase*/
//     MotionCardState* (round-trip JSON MEMOTION_SUBPHASE/MEMOTION_CARD_STATES,
//     #160/#171) — TestMotionSubPhaseConstantValues_183,
//     TestMotionCardStateConstantValues_183.
//
// Run: go test ./internal/game/... -run 183 -v
package game

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

// ============================================================================
// AllQuestionTypes() — A-B2, outil d'exhaustivité pour #184/B-B8.
// ============================================================================

func TestAllQuestionTypes_183(t *testing.T) {
	got := AllQuestionTypes()

	want := map[QuestionType]bool{
		QuestionTypeSpeedy:   true,
		QuestionTypeQCM:      true,
		QuestionTypeMemory:   true,
		QuestionTypeMemotion: true,
		QuestionTypeArdoise:  true,
	}

	if len(got) != len(want) {
		t.Fatalf("AllQuestionTypes() a %d entrées, attendu %d (5 types connus) — got=%v", len(got), len(want), got)
	}

	seen := map[QuestionType]bool{}
	for _, qt := range got {
		if seen[qt] {
			t.Errorf("AllQuestionTypes() contient %q en double", qt)
		}
		seen[qt] = true
		if !want[qt] {
			t.Errorf("AllQuestionTypes() contient %q, absent des 5 types connus (SPEEDY/QCM/MEMORY/MEMOTION/ARDOISE)", qt)
		}
	}
	for qt := range want {
		if !seen[qt] {
			t.Errorf("AllQuestionTypes() n'inclut pas %q", qt)
		}
	}
}

// ============================================================================
// Valeurs des constantes MotionSubPhase*/MotionCardState* — équivalence
// avant/après le remplacement des chaînes libres par les constantes typées
// (A-B1). Ces valeurs sont sérialisées telles quelles (MEMOTION_SUBPHASE,
// MEMOTION_CARD_STATES, docs/WEBSOCKET_PROTOCOL.md) : tout changement de
// valeur serait BREAKING pour le client déjà déployé.
// ============================================================================

func TestMotionSubPhaseConstantValues_183(t *testing.T) {
	cases := map[MotionSubPhase]string{
		MotionSubPhaseMemorize: "MEMORIZE",
		MotionSubPhaseGrid:     "GRID",
		MotionSubPhaseSelected: "SELECTED",
		MotionSubPhaseQuestion: "QUESTION",
		MotionSubPhaseReveal:   "REVEAL",
	}
	for constVal, want := range cases {
		if string(constVal) != want {
			t.Errorf("valeur de constante MotionSubPhase = %q, attendu %q (compat JSON MEMOTION_SUBPHASE)", constVal, want)
		}
	}
}

func TestMotionCardStateConstantValues_183(t *testing.T) {
	cases := map[MotionCardState]string{
		MotionCardStateUnplayed: "UNPLAYED",
		MotionCardStateSelected: "SELECTED",
		MotionCardStateQuestion: "QUESTION",
		MotionCardStateRevealed: "REVEALED",
		MotionCardStateDone:     "DONE",
	}
	for constVal, want := range cases {
		if string(constVal) != want {
			t.Errorf("valeur de constante MotionCardState = %q, attendu %q (compat JSON MEMOTION_CARD_STATES)", constVal, want)
		}
	}
}

// ============================================================================
// "Aucune chaîne libre ... dans le Go non-test" (#183, critère d'acceptance).
//
// Portée volontairement restreinte aux 3 fichiers listés par A-B1 (plan
// §Tâches) : models.go, engine.go, cmd/server/main.go — la machine à états
// MEMOTION. PAS un scan de tout le dépôt : "QUESTION"/"DONE" sont des noms de
// champ JSON et des états AI-job légitimes et sans rapport ailleurs
// (protocol/messages.go ActionReveal, ai_job.go RUNNING/DONE/FAILED/
// CANCELLED) — les y interdire serait un faux positif, pas le bug visé ici.
//
// Détection par AST (go/parser), pas par grep texte : ignore intrinsèquement
// les commentaires et les valeurs de déclaration des constantes elles-mêmes
// (`MotionSubPhaseGrid MotionSubPhase = "GRID"`, la définition canonique,
// n'est pas une "chaîne libre"). Ne flague que l'USAGE d'un littéral en
// comparaison (== / !=) ou en `case` de switch — exactement le motif cité par
// le plan ("== "MEMORY" littéral → constante").
// ============================================================================

var literalTargets183 = map[string]bool{
	"GRID":     true,
	"SELECTED": true,
	"QUESTION": true,
	"REVEAL":   true,
	"MEMORIZE": true,
	"UNPLAYED": true,
	"DONE":     true,
}

func filesToScan183(t *testing.T) []string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) a échoué — impossible de localiser le paquet sous test")
	}
	dir := filepath.Dir(thisFile)          // .../server-go/internal/game
	root := filepath.Join(dir, "..", "..") // .../server-go
	return []string{
		filepath.Join(dir, "models.go"),
		filepath.Join(dir, "engine.go"),
		filepath.Join(root, "cmd", "server", "main.go"),
	}
}

func TestNoFreeMotionLiteralStrings_183(t *testing.T) {
	fset := token.NewFileSet()
	var violations []string

	for _, path := range filesToScan183(t) {
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("échec du parsing AST de %s : %v", path, err)
		}

		flag := func(lit *ast.BasicLit) {
			value := strings.Trim(lit.Value, `"`)
			if literalTargets183[value] {
				pos := fset.Position(lit.Pos())
				violations = append(violations, pos.String()+": chaîne libre "+lit.Value+
					" utilisée en comparaison/case — utiliser la constante typée "+
					"(MotionSubPhase*/MotionCardState*, models.go) au lieu du littéral")
			}
		}

		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.BinaryExpr:
				if node.Op == token.EQL || node.Op == token.NEQ {
					if lit, ok := node.X.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						flag(lit)
					}
					if lit, ok := node.Y.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						flag(lit)
					}
				}
			case *ast.CaseClause:
				for _, expr := range node.List {
					if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						flag(lit)
					}
				}
			}
			return true
		})
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("chaînes libres détectées (#183 critère d'acceptance) :\n%s", strings.Join(violations, "\n"))
	}
}
