// Suite d'acceptance test-writer pour la couleur de thème sur la zone
// `general` (milestone v10.0.0, retour QUALIF round 7), planner-v10-
// general-theme-toggle-20260908-114420.md §1.3, contre l'implémentation
// réelle de dev-backend (commit e1d74ea6).
//
// Complémentaire de TestDevAmbianceThemeColor_ResolutionOrder et
// TestDevAmbianceSceneTableAndTeamPalette (dev-backend, ambiance_dev_test.go,
// toutes deux mises à jour dans e1d74ea6), qui couvrent déjà :
//   - aucune question du tout ⇒ blanc ;
//   - question hôte avec catégorie intégrée (GEOGRAPHY) ⇒ sa couleur ;
//   - question hôte sans catégorie (Category: "") ⇒ blanc ;
//   - catégorie intégrée inconnue ("NE_EXISTE_PAS") ⇒ blanc ;
//   - les 11 lignes du tableau §1.3 (IDLE blanc 200, READY/RUNNING/BUZZ/
//     PAUSE_ALL/REVEAL/TEAM_TURN thème (blanc en l'absence de httpServer),
//     SCORE couleur d'équipe inchangée, ENTRACTE blanc chaud 100 inchangé).
//
// dev-backend a explicitement laissé à test-writer (commit e1d74ea6, voir
// son propre message) :
//   - la branche de PRIORITÉ RAFALE (RafaleCurrentQuestion.Category avant
//     Question.Category) — monter une manche complète plutôt que de
//     supposer la condition ;
//   - le cas de la catégorie PERSONNALISÉE existante mais sans couleur
//     (distinct d'une catégorie inconnue : celle-ci EST résolue — nom et
//     image trouvés — seule sa couleur est vide).
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
// Aides préfixées twTheme pour ne jamais entrer en collision.
package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
)

// ---------------------------------------------------------------------------
// Catégorie personnalisée, résolue mais sans couleur ⇒ blanc.
// ---------------------------------------------------------------------------

// TestThemeColor_CustomCategory_NoColour_FallsBackToWhite is distinct from
// TestDevAmbianceThemeColor_ResolutionOrder's "unknown category" case
// (a key matching NOTHING at all): here the category genuinely resolves —
// ResolveCategoryMeta finds the image file and returns a name — it is only
// the COLOUR that comes back empty, exactly hue-bridge.md §1.2.a's
// documented reserve ("les catégories personnalisées n'ont aucune
// couleur").
func TestThemeColor_CustomCategory_NoColour_FallsBackToWhite(t *testing.T) {
	app := newTestApp(t)
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })
	dataDir := t.TempDir()
	cfg := saved
	cfg.Storage.DataDir = dataDir
	config.SetInstance(&cfg)

	catDir := filepath.Join(dataDir, "files", "categories")
	if err := os.MkdirAll(catDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// toUpperSnakeCase("MA_CATEGORIE") == "MA_CATEGORIE" (already upper +
	// underscore) — matches ResolveCategoryMeta's own stem→key transform
	// (internal/server/http.go) without depending on its unexported
	// implementation.
	if err := os.WriteFile(filepath.Join(catDir, "MA_CATEGORIE.png"), []byte("fake-png"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))

	// Sanity : la catégorie est bien résolue (nom trouvé), la couleur bien vide.
	name, _, color := app.httpServer.ResolveCategoryMeta("MA_CATEGORIE")
	if name == "" {
		t.Fatal("setup invalide : la catégorie personnalisée devrait être résolue (fichier présent)")
	}
	if color != "" {
		t.Fatalf("setup invalide : une catégorie personnalisée ne doit porter aucune couleur, got %q", color)
	}

	app.engine.Ready("qc", &game.Question{ID: "qc", Type: game.QuestionTypeSpeedy, Category: "MA_CATEGORIE"})
	white := [3]int{255, 255, 255}
	if got := app.ambianceThemeColor(); got != white {
		t.Fatalf("catégorie personnalisée (résolue, sans couleur) : got %v, want blanc %v", got, white)
	}
}

// ---------------------------------------------------------------------------
// RAFALE : le thème suit CHAQUE question tirée, pas la catégorie de la
// manche — monte une manche complète, plusieurs tirages successifs.
// ---------------------------------------------------------------------------

func TestThemeColor_Rafale_ThemeFollowsEachDrawnQuestion_NotTheRoundsOwnCategory(t *testing.T) {
	app := newTestApp(t)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	wantHistory, ok := hexToRGB("#eab308") // HISTORY, hardcodedCategories
	if !ok {
		t.Fatal("setup invalide : hexToRGB a échoué sur une couleur connue")
	}
	wantScience, ok := hexToRGB("#22c55e") // SCIENCE
	if !ok {
		t.Fatal("setup invalide : hexToRGB a échoué sur une couleur connue")
	}
	wantGeography, ok := hexToRGB("#3b82f6") // GEOGRAPHY — la catégorie DÉCOY de la manche, ne doit JAMAIS apparaître
	if !ok {
		t.Fatal("setup invalide : hexToRGB a échoué sur une couleur connue")
	}
	if wantHistory == wantScience || wantHistory == wantGeography || wantScience == wantGeography {
		t.Fatal("setup invalide : les 3 couleurs doivent être distinctes pour que ce test soit concluant")
	}

	// Réservoir : 5 questions HISTORY/1, 5 questions SCIENCE/1.
	for i := 1; i <= 5; i++ {
		if _, err := app.engine.UpsertRafaleQuestion(game.RafaleQuestion{
			ID: "h" + strconv.Itoa(i), Question: "QH", Answer: "AH", Category: game.CategoryHistory, Difficulty: 1,
		}); err != nil {
			t.Fatalf("seed HISTORY: %v", err)
		}
		if _, err := app.engine.UpsertRafaleQuestion(game.RafaleQuestion{
			ID: "s" + strconv.Itoa(i), Question: "QS", Answer: "AS", Category: game.CategoryScience, Difficulty: 1,
		}); err != nil {
			t.Fatalf("seed SCIENCE: %v", err)
		}
	}

	// La question de CONFIGURATION de manche porte volontairement une
	// catégorie hôte DIFFÉRENTE (GEOGRAPHY, décoy) : si l'implémentation
	// lisait Question.Category au lieu de RafaleCurrentQuestion.Category,
	// ce test le détecterait immédiatement (couleur bleue au lieu de
	// jaune/verte).
	roundQuestion := &game.Question{
		ID:       "rafale-round",
		Question: "Manche RAFALE",
		Type:     game.QuestionTypeRafale,
		Category: game.CategoryGeography, // décoy — ne doit jamais être ce qui s'affiche
		Points:   "10",
		Time:     "120",
		TypedContent: game.TypedContent{
			RafaleCategories:   []string{"HISTORY", "SCIENCE"},
			RafaleDifficulties: []int{1},
			RafaleMode:         string(game.RafaleModeSolo),
			RafaleQuestionTime: 3,
			RafaleMaxQuestions: 100,
		},
	}
	app.engine.SetTeams(map[string]*game.Team{"red": {Name: "red", Color: []int{255, 0, 0}}})
	app.engine.Ready(roundQuestion.ID, roundQuestion)
	if err := app.engine.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	app.engine.StartImmediate(0)
	defer app.engine.Stop()

	sawHistory, sawScience := false, false
	for draw := 0; draw < 8; draw++ {
		state := app.engine.GetState()
		if state.RafaleSubPhase == game.RafaleSubPhaseRoundEnd {
			break
		}
		cat := state.RafaleCurrentQuestion.Category
		got := app.ambianceThemeColor()

		var want [3]int
		switch cat {
		case string(game.CategoryHistory):
			want, sawHistory = wantHistory, true
		case string(game.CategoryScience):
			want, sawScience = wantScience, true
		default:
			t.Fatalf("tirage %d : catégorie inattendue %q (attendu HISTORY ou SCIENCE)", draw, cat)
		}
		if got != want {
			t.Errorf("tirage %d (catégorie tirée %s) : thème = %v, want %v", draw, cat, got, want)
		}
		if got == wantGeography {
			t.Fatalf("tirage %d : le thème a suivi la catégorie DE LA MANCHE (GEOGRAPHY) au lieu de la question tirée — priorité RafaleCurrentQuestion.Category violée", draw)
		}

		if err := app.engine.RafaleValidate(); err != nil {
			t.Fatalf("tirage %d : RafaleValidate a échoué : %v", draw, err)
		}
	}
	if !sawHistory || !sawScience {
		t.Fatalf("setup insuffisant : attendu au moins un tirage HISTORY ET un tirage SCIENCE sur 8 tirages, got history=%v science=%v", sawHistory, sawScience)
	}
}

