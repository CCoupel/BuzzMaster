package server

// Tests d'alignement Go↔JS pour les constantes partagées de #203 (génération
// IA du réservoir RAFALE, milestone v8.1.0) — risque R9 du plan
// (_work/reports/plan-20260901-162105.md), déjà matérialisé 2× sur
// #174/v7.0.0, et signalé comme lacune MAJEURE par la revue de code
// (_work/reports/code-review-20260901-175014.md) : les deux listes/valeurs
// étaient identiques mais sans garde automatisée — une dérive future d'un
// seul côté (ex. ajout d'un palier côté UI sans mise à jour Go) ne serait
// détectée par aucune suite, seulement par un 400 en QUALIF/PROD.
//
// Ces tests lisent le SOURCE JS réel (os.ReadFile + extraction par regex de
// la déclaration de constante) et comparent à la valeur Go — pas besoin
// d'environnement JS/Node, cf. suggestion du reviewer. Aucun précédent exact
// dans le paquet (team_color_palette_test.go, cmd/server, mirrors une table
// contractuelle mais ne lit pas le fichier JS lui-même) — nouveau motif,
// documenté ici pour un futur cas similaire.
//
// Trois constantes couvertes :
//   - rafaleGenerationPresets (ai_generator_rafale.go) vs
//     RAFALE_GENERATION_PRESETS (web/src/components/RafaleAIGenerateModal.jsx)
//   - rafaleMaxQuestionRunes vs RAFALE_MAX_QUESTION_RUNES (web/src/pages/RafalePage.jsx)
//   - rafaleMaxAnswerRunes vs RAFALE_MAX_ANSWER_RUNES (web/src/pages/RafalePage.jsx)

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const rafaleAIGenerateModalJSPath = "../../web/src/components/RafaleAIGenerateModal.jsx"
const rafalePageJSPath = "../../web/src/pages/RafalePage.jsx"

// readRafaleFrontendSource is a t.Helper wrapper around os.ReadFile with a
// clear failure message if the frontend source has moved — a FailNow here
// (rather than a silent skip) is deliberate: this test's entire point is to
// catch drift, so an unreadable source file must fail loudly, not pass by
// omission.
func readRafaleFrontendSource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read frontend source %q (has it moved? update this test's path alongside it): %v", path, err)
	}
	return string(data)
}

func TestRafaleGenerationPresets_AlignedWithFrontendConstant(t *testing.T) {
	src := readRafaleFrontendSource(t, rafaleAIGenerateModalJSPath)

	re := regexp.MustCompile(`const RAFALE_GENERATION_PRESETS = \[([^\]]*)\]`)
	m := re.FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("could not find \"const RAFALE_GENERATION_PRESETS = [...]\" in %s — has it been renamed? update this test's regex alongside it", rafaleAIGenerateModalJSPath)
	}

	var jsPresets []int
	for _, part := range strings.Split(m[1], ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			t.Fatalf("could not parse preset value %q from RAFALE_GENERATION_PRESETS: %v", part, err)
		}
		jsPresets = append(jsPresets, n)
	}

	if len(jsPresets) != len(rafaleGenerationPresets) {
		t.Fatalf("RAFALE_GENERATION_PRESETS (JS) has %d entries %v, rafaleGenerationPresets (Go) has %d entries %v — Go↔JS drift (contract §2bis, risk R9)",
			len(jsPresets), jsPresets, len(rafaleGenerationPresets), rafaleGenerationPresets)
	}
	for i, want := range rafaleGenerationPresets {
		if jsPresets[i] != want {
			t.Errorf("preset[%d]: JS=%d, Go=%d — Go↔JS drift (contract §2bis, risk R9): JS=%v Go=%v",
				i, jsPresets[i], want, jsPresets, rafaleGenerationPresets)
		}
	}
}

func TestRafaleMaxQuestionRunes_AlignedWithFrontendConstant(t *testing.T) {
	src := readRafaleFrontendSource(t, rafalePageJSPath)

	re := regexp.MustCompile(`const RAFALE_MAX_QUESTION_RUNES = (\d+)`)
	m := re.FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("could not find \"const RAFALE_MAX_QUESTION_RUNES = <n>\" in %s — has it been renamed? update this test's regex alongside it", rafalePageJSPath)
	}
	jsValue, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("could not parse RAFALE_MAX_QUESTION_RUNES value %q: %v", m[1], err)
	}
	if jsValue != rafaleMaxQuestionRunes {
		t.Errorf("RAFALE_MAX_QUESTION_RUNES (JS) = %d, rafaleMaxQuestionRunes (Go) = %d — Go↔JS drift (contract §5.1/§5.3): a question the editor's counter accepts client-side could still be rejected server-side, or vice versa", jsValue, rafaleMaxQuestionRunes)
	}
}

func TestRafaleMaxAnswerRunes_AlignedWithFrontendConstant(t *testing.T) {
	src := readRafaleFrontendSource(t, rafalePageJSPath)

	re := regexp.MustCompile(`const RAFALE_MAX_ANSWER_RUNES = (\d+)`)
	m := re.FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("could not find \"const RAFALE_MAX_ANSWER_RUNES = <n>\" in %s — has it been renamed? update this test's regex alongside it", rafalePageJSPath)
	}
	jsValue, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("could not parse RAFALE_MAX_ANSWER_RUNES value %q: %v", m[1], err)
	}
	if jsValue != rafaleMaxAnswerRunes {
		t.Errorf("RAFALE_MAX_ANSWER_RUNES (JS) = %d, rafaleMaxAnswerRunes (Go) = %d — Go↔JS drift (contract §5.1/§5.3): a question the editor's counter accepts client-side could still be rejected server-side, or vice versa", jsValue, rafaleMaxAnswerRunes)
	}
}
