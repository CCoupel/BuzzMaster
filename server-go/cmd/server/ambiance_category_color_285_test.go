// #285 (v11.4.0, LOT-4) — contracts/lighting.md §8 : la couleur réglable d'une catégorie pilote la couleur d'ambiance.
// Une catégorie sans couleur ⇒ salle blanche ; une personnalisée colorée ⇒ sa couleur ; une intégrée recolorée (sidecar)
// ⇒ la surcharge, immédiatement (aucun cache) ; "" explicite ⇒ blanc. Seuls des symboles EXISTANTS sont utilisés
// (ambianceThemeColor, ResolveCategoryMeta, sidecars JSON sur disque). Préfixe tw285.
package main

import (
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
)

func tw285AmbianceApp(t *testing.T) (*App, string) {
	t.Helper()
	app := newTestApp(t)
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })
	dataDir := t.TempDir()
	cfg := saved
	cfg.Storage.DataDir = dataDir
	config.SetInstance(&cfg)
	catDir := filepath.Join(dataDir, "files", "categories")
	if err := os.MkdirAll(catDir, 0o755); err != nil {
		t.Fatal(err)
	}
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	return app, catDir
}

func tw285WriteCustom(t *testing.T, catDir, key, sidecar string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(catDir, key+".png"), []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catDir, key+".json"), []byte(sidecar), 0o644); err != nil {
		t.Fatal(err)
	}
}

func tw285Ready(app *App, cat string) {
	app.engine.Ready("q", &game.Question{ID: "q", Type: game.QuestionTypeSpeedy, Category: game.QuestionCategory(cat)})
}

func TestAmbianceCategoryColor285_CustomWithColor_UsesIt_WithoutColor_White(t *testing.T) {
	app, catDir := tw285AmbianceApp(t)
	tw285WriteCustom(t, catDir, "CINEMA", `{"name":"Cinéma","color":"#0ea5e9"}`)
	tw285WriteCustom(t, catDir, "NUE", `{"name":"Nue"}`)

	want, _ := hexToRGB("#0ea5e9")
	tw285Ready(app, "CINEMA")
	if got := app.ambianceThemeColor(); got != want {
		t.Errorf("custom colorée : got %v, want %v", got, want)
	}
	tw285Ready(app, "NUE")
	if got := app.ambianceThemeColor(); got != [3]int{255, 255, 255} {
		t.Errorf("custom sans couleur : got %v, want blanc", got)
	}
}

func TestAmbianceCategoryColor285_BuiltinOverride_AndExplicitNone(t *testing.T) {
	app, catDir := tw285AmbianceApp(t)
	tw285Ready(app, "GEOGRAPHY")
	orig, _ := hexToRGB("#3b82f6")
	if got := app.ambianceThemeColor(); got != orig {
		t.Fatalf("intégrée sans sidecar : got %v, want %v", got, orig)
	}
	if err := os.WriteFile(filepath.Join(catDir, "GEOGRAPHY.json"), []byte(`{"color":"#ff0000"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := app.ambianceThemeColor(); got != [3]int{255, 0, 0} {
		t.Errorf("surcharge sidecar prise en compte immédiatement : got %v", got)
	}
	if err := os.WriteFile(filepath.Join(catDir, "GEOGRAPHY.json"), []byte(`{"color":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := app.ambianceThemeColor(); got != [3]int{255, 255, 255} {
		t.Errorf("color \"\" explicite ⇒ aucune ⇒ blanc : got %v", got)
	}
}

func TestAmbianceCategoryColor285_ResolveReadsSidecarLive_NoCache(t *testing.T) {
	app, catDir := tw285AmbianceApp(t)
	tw285WriteCustom(t, catDir, "CINEMA", `{"name":"Cinéma","color":"#00ff00"}`)
	if _, err := app.engine.UpsertRafaleQuestion(game.RafaleQuestion{ID: "r1", Question: "Q", Answer: "A", Category: "CINEMA", Difficulty: 1}); err != nil {
		t.Fatal(err)
	}
	// changer la couleur sur disque : la résolution est en direct, sans redémarrage ni cache
	if err := os.WriteFile(filepath.Join(catDir, "CINEMA.json"), []byte(`{"name":"Cinéma","color":"#0000ff"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, hex := app.httpServer.ResolveCategoryMeta("CINEMA")
	if hex != "#0000ff" {
		t.Errorf("ResolveCategoryMeta doit relire le sidecar à chaque appel : %q", hex)
	}
}

func TestAmbianceCategoryColor285_OnCategoriesChanged_IsWiredAndSafe(t *testing.T) {
	app := newTestAppWithHub(t)
	app.logger = server.NewBroadcastLogger(100)
	app.logsHub = server.NewLogsWebSocketHub(10)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, app.logsHub)
	app.setupCallbacks()
	if app.httpServer.OnCategoriesChanged == nil {
		t.Fatal("HTTPServer.OnCategoriesChanged doit être câblé par setupCallbacks (broadcastQuestions + NotifyState)")
	}
	app.httpServer.OnCategoriesChanged() // ne doit pas paniquer, même sans pilote d'éclairage ni client connecté
}
