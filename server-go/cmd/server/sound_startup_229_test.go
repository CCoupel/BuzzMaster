// Suite test-writer pour #229 (milestone v11.0 — plan de dev §2.8 point 2) :
// (*App).createDefaultSounds() — le câblage réel au démarrage (setup()),
// au-dessus de synth.WriteAll/ReconcileManifest (déjà couverts isolément
// par internal/audio/synth's propres tests). Ce fichier vérifie que le
// câblage cmd/server appelle bien ces fonctions au bon endroit, avec le bon
// répertoire (soundsDir(a.config)).
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/audio/synth"
)

// tw229sAppWithTempFilesDir builds a minimal App whose config.Storage.FilesDir
// points at a fresh t.TempDir() — isolates createDefaultSounds from the
// real ./data/files used by config.Get()'s process-wide default.
func tw229sAppWithTempFilesDir(t *testing.T) *App {
	t.Helper()
	app := newTestApp(t)
	app.config.Storage.FilesDir = t.TempDir()
	return app
}

// TestCreateDefaultSounds_FirstRun_WritesAllSevenFilesAndManifest is §2.8
// point 2's "les 7 fichiers sont créés au premier lancement," exercé via le
// VRAI point d'entrée (*App).createDefaultSounds(), pas synth.WriteAll
// directement.
func TestCreateDefaultSounds_FirstRun_WritesAllSevenFilesAndManifest(t *testing.T) {
	app := tw229sAppWithTempFilesDir(t)
	app.createDefaultSounds()

	dir := soundsDir(app.config)
	for _, c := range synth.Cues {
		if _, err := os.Stat(filepath.Join(dir, synth.Filename(c))); err != nil {
			t.Errorf("%s : fichier absent après createDefaultSounds() : %v", synth.Filename(c), err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "sounds.json")); err != nil {
		t.Errorf("sounds.json (manifeste) absent après createDefaultSounds() : %v", err)
	}
}

// TestCreateDefaultSounds_SecondRun_NeverOverwritesCustomFile is §2.8
// point 2's "un fichier personnalisé survit à un redémarrage du serveur" —
// exercé au niveau App, avec deux appels à createDefaultSounds() simulant
// deux démarrages successifs (setup() est appelé une fois par processus,
// donc "redémarrage" = un nouveau processus = un nouvel appel).
func TestCreateDefaultSounds_SecondRun_NeverOverwritesCustomFile(t *testing.T) {
	app := tw229sAppWithTempFilesDir(t)
	app.createDefaultSounds()

	dir := soundsDir(app.config)
	customized := synth.Filename(synth.Cues[0])
	customContent := []byte("contenu personnalisé — simule un futur upload #230")
	if err := os.WriteFile(filepath.Join(dir, customized), customContent, 0644); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}

	app.createDefaultSounds() // simule un redémarrage du serveur

	got, err := os.ReadFile(filepath.Join(dir, customized))
	if err != nil {
		t.Fatalf("%s a disparu après un redémarrage simulé : %v", customized, err)
	}
	if string(got) != string(customContent) {
		t.Errorf("%s a été écrasé par un redémarrage simulé (createDefaultSounds), attendu préservé", customized)
	}
}

// TestCreateDefaultSounds_ManifestReflectsCustomFile proves the manifest
// written by createDefaultSounds() genuinely reflects a customised file
// (Custom=true) — the App-level wiring twin of
// synth.TestReconcileManifest_CustomizedFile_ReportedCustom.
func TestCreateDefaultSounds_ManifestReflectsCustomFile(t *testing.T) {
	app := tw229sAppWithTempFilesDir(t)
	app.createDefaultSounds()

	dir := soundsDir(app.config)
	customized := synth.Cues[0]
	if err := os.WriteFile(filepath.Join(dir, synth.Filename(customized)), []byte("personnalisé"), 0644); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	app.createDefaultSounds() // ré-appelle : ne réécrit pas le fichier, mais DOIT réconcilier le manifeste

	raw, err := os.ReadFile(filepath.Join(dir, "sounds.json"))
	if err != nil {
		t.Fatalf("sounds.json : %v", err)
	}
	if !containsCustomTrueForCue(t, raw, string(customized)) {
		t.Errorf("sounds.json ne rapporte pas custom=true pour %s après personnalisation, got %s", customized, raw)
	}
}

// containsCustomTrueForCue is a minimal, dependency-free JSON substring
// check — avoids importing synth.ManifestEntry's exact shape here (this
// file only needs to prove the manifest reflects reality, not re-parse its
// full schema, already covered by synth's own manifest_229_test.go).
func containsCustomTrueForCue(t *testing.T, raw []byte, cue string) bool {
	t.Helper()
	var manifest map[string]struct {
		Present bool `json:"present"`
		Custom  bool `json:"custom,omitempty"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("sounds.json invalide : %v", err)
	}
	entry, ok := manifest[cue]
	return ok && entry.Custom
}
