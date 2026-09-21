// #229 (v11.0), B.5 — POST /api/sounds/restore-defaults
// (internal/server/http_sound.go), modelled on
// handleAPIFirmwareRestoreEmbedded (RestoreEmbedded): an explicit,
// idempotent action that ALWAYS overwrites, the opposite of ordinary
// startup (createDefaultSounds, which never touches an existing file).
//
// Distinct from the generic backup/restore-from-TAR path already covered
// by sounds_backup_229_test.go — this is a dedicated, TAR-less endpoint: no
// upload needed, it just regenerates from the deterministic synth
// generator.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/audio/synth"
)

// TestHandleAPISoundsRestoreDefaults_OverwritesCustomizedFile is §2.8
// point 3's "restauration ... rend exactement les octets d'origine" (à la
// différence du démarrage, qui n'écrase jamais), exercé au niveau HTTP —
// le VRAI point d'entrée qu'un client (ou une future UI #230) appellerait.
func TestHandleAPISoundsRestoreDefaults_OverwritesCustomizedFile(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	soundsDir := filepath.Join(dataDir, "files", "sounds")
	os.MkdirAll(soundsDir, 0755)
	os.WriteFile(filepath.Join(soundsDir, "depart.wav"), []byte("personnalisé, à écraser"), 0644)

	req := httptest.NewRequest("POST", "/api/sounds/restore-defaults", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := os.ReadFile(filepath.Join(soundsDir, "depart.wav"))
	if err != nil {
		t.Fatalf("depart.wav : %v", err)
	}
	want, ok := synth.Generate(synth.Cues[0])
	if !ok {
		t.Fatalf("setup invalide : synth.Generate a renvoyé ok=false")
	}
	if !bytes.Equal(got, want) {
		t.Error("restore-defaults n'a pas rendu exactement les octets d'origine pour depart.wav")
	}
}

// TestHandleAPISoundsRestoreDefaults_WritesAllSevenAndManifest proves the
// endpoint regenerates the FULL catalogue (not just files that already
// existed) and reconciles sounds.json afterward, even when starting from
// an empty directory (no prior WriteAll call at all).
func TestHandleAPISoundsRestoreDefaults_WritesAllSevenAndManifest(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	req := httptest.NewRequest("POST", "/api/sounds/restore-defaults", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	soundsDir := filepath.Join(dataDir, "files", "sounds")
	for _, c := range synth.Cues {
		if _, err := os.Stat(filepath.Join(soundsDir, synth.Filename(c))); err != nil {
			t.Errorf("%s absent après restore-defaults : %v", synth.Filename(c), err)
		}
	}
	if _, err := os.Stat(filepath.Join(soundsDir, "sounds.json")); err != nil {
		t.Errorf("sounds.json absent après restore-defaults : %v", err)
	}
}

// TestHandleAPISoundsRestoreDefaults_Idempotent is §2.8 point 3's
// "idempotente" au niveau HTTP : deux appels successifs produisent des
// fichiers strictement identiques sur disque.
func TestHandleAPISoundsRestoreDefaults_Idempotent(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	soundsDir := filepath.Join(dataDir, "files", "sounds")

	req1 := httptest.NewRequest("POST", "/api/sounds/restore-defaults", nil)
	server.mux.ServeHTTP(httptest.NewRecorder(), req1)
	first := map[string][]byte{}
	for _, c := range synth.Cues {
		data, err := os.ReadFile(filepath.Join(soundsDir, synth.Filename(c)))
		if err != nil {
			t.Fatalf("%s : %v", synth.Filename(c), err)
		}
		first[synth.Filename(c)] = data
	}

	req2 := httptest.NewRequest("POST", "/api/sounds/restore-defaults", nil)
	server.mux.ServeHTTP(httptest.NewRecorder(), req2)
	for _, c := range synth.Cues {
		data, err := os.ReadFile(filepath.Join(soundsDir, synth.Filename(c)))
		if err != nil {
			t.Fatalf("%s : %v", synth.Filename(c), err)
		}
		if !bytes.Equal(data, first[synth.Filename(c)]) {
			t.Errorf("%s : deux appels successifs à restore-defaults ont produit des octets différents", synth.Filename(c))
		}
	}
}

// TestHandleAPISoundsRestoreDefaults_RejectsNonPOST proves the endpoint
// refuses other HTTP methods explicitly, rather than silently accepting
// (and possibly regenerating) on an unintended GET, e.g. from a browser
// address bar or a health-check probe.
func TestHandleAPISoundsRestoreDefaults_RejectsNonPOST(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	req := httptest.NewRequest("GET", "/api/sounds/restore-defaults", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for GET, got %d", w.Code)
	}
}
