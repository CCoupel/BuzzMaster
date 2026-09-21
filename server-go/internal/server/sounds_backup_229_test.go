// #229 (v11.0) — "le test qui manquait à #152" : files/sounds/ (les 7 sons
// par défaut synthétisés, contract sound.md) doit être inscrit dans les
// QUATRE chemins dès le premier jet — sauvegarde TAR, réinitialisation,
// extraction de restauration, détection de restauration
// (_work/reports/plan-dev-228-229-20260921-114500.md §2.4). #152 avait
// oublié les fonds d'écran de l'écran d'accueil dans ces mêmes chemins ;
// #119 (ENTRACTE) a depuis établi le patron correct — ce fichier le
// réplique pour les sons, exactement comme entracte_image_test.go l'a fait
// avant lui (mêmes helpers : setupTestHTTPServer, tarContains,
// buildTARWithSingleFile, multipartRestoreBody — internal/server/http_test.go
// et gameconfig_backup_test.go).
//
// files/sounds/ contient PLUSIEURS fichiers par nature (les 7 sons, plus un
// manifeste sounds.json) — ce fichier utilise deux fixtures pour le
// prouver, là où entracte_image_test.go n'en avait besoin que d'une seule
// (un panneau unique).
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/audio/synth"
)

// TestHTTPServer_BackupSelect_IncludesSoundsWithMedias mirrors
// TestHTTPServer_BackupSelect_IncludesEntracteWithMedias
// (entracte_image_test.go) for files/sounds/.
func TestHTTPServer_BackupSelect_IncludesSoundsWithMedias(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	soundsDir := filepath.Join(dataDir, "files", "sounds")
	if err := os.MkdirAll(soundsDir, 0755); err != nil {
		t.Fatalf("could not create fixture sounds dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(soundsDir, "depart.wav"), []byte("fixture-depart"), 0644); err != nil {
		t.Fatalf("could not write fixture sound: %v", err)
	}
	if err := os.WriteFile(filepath.Join(soundsDir, "sounds.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("could not write fixture manifest: %v", err)
	}

	t.Run("medias=true includes files/sounds/", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/backup-select?medias=true", nil)
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !tarContains(t, w.Body.Bytes(), "files/sounds/depart.wav") {
			t.Error("Expected files/sounds/depart.wav in the TAR when medias=true")
		}
		if !tarContains(t, w.Body.Bytes(), "files/sounds/sounds.json") {
			t.Error("Expected files/sounds/sounds.json (manifest) in the TAR when medias=true")
		}
	})

	t.Run("medias=false excludes files/sounds/", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/backup-select?teams=true", nil)
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if tarContains(t, w.Body.Bytes(), "files/sounds/depart.wav") {
			t.Error("Did not expect files/sounds/ in the TAR when medias is not selected")
		}
	})
}

// TestHTTPServer_ResetSelect_Sounds_RegeneratesDefaults is the sounds-
// specific twin of TestHTTPServer_ResetSelect_Entracte — with a DELIBERATE
// difference from backgrounds/categories/entracte (which stay EMPTY after
// a reset, until a future re-upload or restart): sounds are synthesised at
// near-zero cost, so http.go's handleResetSelect clears then IMMEDIATELY
// regenerates the 7 defaults (comment on that call site: "laisser le
// bruitage totalement silencieux jusqu'à un hypothétique redémarrage serait
// une régression sur l'objet même du milestone"). This test asserts THAT
// behavior, not the empty-after-reset pattern the other media types use.
func TestHTTPServer_ResetSelect_Sounds_RegeneratesDefaults(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	soundsDir := filepath.Join(dataDir, "files", "sounds")
	os.MkdirAll(soundsDir, 0755)
	customized := "depart.wav"
	os.WriteFile(filepath.Join(soundsDir, customized), []byte("fixture-personnalise"), 0644)
	os.WriteFile(filepath.Join(soundsDir, "sounds.json"), []byte(`{"stale":"manifest"}`), 0644)

	req := httptest.NewRequest("POST", "/reset-select?medias=true", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Le contenu PERSONNALISÉ doit avoir disparu — remplacé par le son
	// généré par défaut, jamais laissé tel quel ni simplement supprimé.
	got, err := os.ReadFile(filepath.Join(soundsDir, customized))
	if err != nil {
		t.Fatalf("Expected %s to exist (regenerated), got error: %v", customized, err)
	}
	want, ok := synth.Generate(synth.Cues[0])
	if !ok {
		t.Fatalf("setup invalide : synth.Generate a renvoyé ok=false")
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s après reset ne correspond pas au son par défaut généré — le contenu personnalisé n'a pas été remplacé", customized)
	}

	// Les 7 fichiers doivent tous être présents (régénérés), pas seulement
	// celui qui était personnalisé.
	for _, c := range synth.Cues {
		if _, err := os.Stat(filepath.Join(soundsDir, synth.Filename(c))); err != nil {
			t.Errorf("%s absent après reset (attendu régénéré) : %v", synth.Filename(c), err)
		}
	}
	if _, err := os.Stat(filepath.Join(soundsDir, "sounds.json")); err != nil {
		t.Errorf("sounds.json (manifeste) absent après reset, attendu réconcilié : %v", err)
	}
}

// TestHTTPServer_Restore_Sounds mirrors TestHTTPServer_Restore_Entracte for
// files/sounds/ — the GENERIC backup-TAR restore path (POST /restore),
// distinct from #229's dedicated "restore default sounds" endpoint.
func TestHTTPServer_Restore_Sounds(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	tarData := buildTARWithSingleFile(t, "files/sounds/depart.wav", []byte("restored-depart"))
	body, contentType := multipartRestoreBody(t, tarData)

	req := httptest.NewRequest("POST", "/restore", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Restored map[string]bool `json:"restored"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}
	if !resp.Restored["sounds"] {
		t.Errorf("Expected restored.sounds=true, got %+v", resp.Restored)
	}

	onDisk, err := os.ReadFile(filepath.Join(dataDir, "files", "sounds", "depart.wav"))
	if err != nil {
		t.Fatalf("depart.wav was not written to disk: %v", err)
	}
	if string(onDisk) != "restored-depart" {
		t.Errorf("Restored sound content mismatch: got %q", onDisk)
	}
}

// TestDetectTARContents_Sounds is the narrower unit-level twin of the three
// HTTP-level tests above — pins the detection key itself ("sounds") against
// both path conventions used elsewhere in this switch (a bare top-level
// name is never expected here, only "files/sounds/", matching backgrounds/
// categories/entracte, none of which accept a bare-root alias either).
func TestDetectTARContents_Sounds(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	tarData := buildTARWithSingleFile(t, "files/sounds/depart.wav", []byte("x"))
	detected := server.detectTARContents(tarData)
	if !detected["sounds"] {
		t.Errorf("detectTARContents did not set detected[\"sounds\"]=true for files/sounds/depart.wav, got %+v", detected)
	}
}
