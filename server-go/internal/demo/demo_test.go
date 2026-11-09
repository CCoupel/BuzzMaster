package demo

import (
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
)

// #291 Phase B : tests du paquet extrait, avec un Host factice (moteur réel, dossiers temporaires).

type fakeHost struct {
	engine *game.Engine
	cfg    *config.Config
	saved  int
}

func (h *fakeHost) Engine() *game.Engine   { return h.engine }
func (h *fakeHost) Config() *config.Config { return h.cfg }
func (h *fakeHost) ResolveCategoryMeta(key string) (string, string, string) {
	return key, "", ""
}
func (h *fakeHost) SaveBackgroundsConfig() { h.saved++ }

func newFakeHost(t *testing.T) *fakeHost {
	t.Helper()
	cfg := &config.Config{}
	cfg.Storage.QuestionsDir = filepath.Join(t.TempDir(), "questions")
	cfg.Storage.FilesDir = t.TempDir()
	return &fakeHost{engine: game.NewEngine(), cfg: cfg}
}

func TestInitTestData_CreatesTeamsBumpersAndQuestionsOnce(t *testing.T) {
	h := newFakeHost(t)
	InitTestData(h)

	tb := h.engine.GetTeamsAndBumpers()
	if len(tb.Teams) != 6 || len(tb.Bumpers) == 0 {
		t.Fatalf("équipes=%d (attendu 6), joueurs=%d", len(tb.Teams), len(tb.Bumpers))
	}
	entries, err := os.ReadDir(h.cfg.Storage.QuestionsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("des questions de test doivent être créées : %v (%d)", err, len(entries))
	}
	n := len(entries)
	createTestQuestions(h) // dossier non vide : rien n'est recréé
	if again, _ := os.ReadDir(h.cfg.Storage.QuestionsDir); len(again) != n {
		t.Errorf("createTestQuestions ne doit rien refaire quand des questions existent : %d → %d", n, len(again))
	}
}

func TestLoadDemoData_ReplacesQuestionsAndFillsBackgroundsAndHistory(t *testing.T) {
	h := newFakeHost(t)
	LoadDemoData(h)

	tb := h.engine.GetTeamsAndBumpers()
	if len(tb.Teams) != 6 || len(tb.Bumpers) == 0 {
		t.Fatalf("équipes=%d (attendu 6), joueurs=%d", len(tb.Teams), len(tb.Bumpers))
	}
	if entries, err := os.ReadDir(h.cfg.Storage.QuestionsDir); err != nil || len(entries) == 0 {
		t.Fatalf("des questions de démo doivent être créées : %v", err)
	}
	if h.saved != 1 {
		t.Errorf("la configuration des fonds est sauvegardée une fois : %d", h.saved)
	}
	if _, err := os.Stat(filepath.Join(h.cfg.Storage.FilesDir, "backgrounds", "demo_bg_1.jpg")); err != nil {
		t.Errorf("les fonds de démo sont extraits des assets embarqués : %v", err)
	}
}
