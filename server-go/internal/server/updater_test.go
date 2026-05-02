package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestUpdater creates an Updater with a temporary dataDir suitable for tests.
// NewUpdater now requires (version, dataDir) — this helper avoids repeating t.TempDir().
func newTestUpdater(t *testing.T, version string) *Updater {
	t.Helper()
	return NewUpdater(version, t.TempDir())
}


func TestCopyFile(t *testing.T) {
	// Create temp directory
	tmpDir := os.TempDir()
	src := filepath.Join(tmpDir, "test_src.txt")
	dst := filepath.Join(tmpDir, "test_dst.txt")

	// Cleanup
	defer os.Remove(src)
	defer os.Remove(dst)

	// Create source file
	content := []byte("test content for copy")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Copy file
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify destination exists
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Error("Destination file was not created")
	}

	// Verify content matches
	dstContent, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(dstContent) != string(content) {
		t.Errorf("Content mismatch: got %s, want %s", string(dstContent), string(content))
	}
}

func TestUpdater_CalculateChecksum(t *testing.T) {
	updater := newTestUpdater(t, "2.50.0")

	// Create temp file
	tmpDir := os.TempDir()
	testFile := filepath.Join(tmpDir, "test_checksum.txt")
	defer os.Remove(testFile)

	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Calculate checksum
	checksum, err := updater.calculateChecksum(testFile)
	if err != nil {
		t.Fatalf("calculateChecksum failed: %v", err)
	}

	// Checksum should be non-empty and hex string
	if checksum == "" {
		t.Error("Checksum should not be empty")
	}

	// SHA256 hex string should be 64 characters
	if len(checksum) != 64 {
		t.Errorf("SHA256 checksum should be 64 characters, got %d", len(checksum))
	}

	// Known checksum for "hello world"
	expectedChecksum := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if checksum != expectedChecksum {
		t.Errorf("Checksum mismatch: got %s, want %s", checksum, expectedChecksum)
	}
}

func TestNewUpdater(t *testing.T) {
	version := "2.50.0"
	updater := newTestUpdater(t, version)

	if updater == nil {
		t.Fatal("NewUpdater returned nil")
	}

	if updater.currentVer != version {
		t.Errorf("currentVer = %s, want %s", updater.currentVer, version)
	}

	if updater.githubClient == nil {
		t.Error("githubClient should not be nil")
	}

	if updater.updatesDir == "" {
		t.Error("updatesDir should not be empty")
	}

	// Verify updates directory was created
	if _, err := os.Stat(updater.updatesDir); os.IsNotExist(err) {
		t.Errorf("Updates directory was not created: %s", updater.updatesDir)
	}
}

func TestMinBinarySize(t *testing.T) {
	// BuzzControl binaries are ~8-9MB on Windows, ~8MB on Linux ARM64.
	// MinBinarySize must be below actual binary sizes to avoid false rejections.
	const actualBinarySize = 8 * 1024 * 1024 // 8MB conservative lower bound

	if MinBinarySize >= actualBinarySize {
		t.Errorf("MinBinarySize (%d bytes) is >= actual binary size (%d bytes); downloads will always be rejected",
			MinBinarySize, actualBinarySize)
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name: "extracts first bold text",
			body: `### Added

**Dedicated Backup/Restore Page** :
- Nouvelle page dédiée...`,
			expected: "Dedicated Backup/Restore Page",
		},
		{
			name: "removes trailing colon with space",
			body: `**Auto-Update Feature** :
Some description`,
			expected: "Auto-Update Feature",
		},
		{
			name: "removes trailing colon without space",
			body: `**Bug Fix**:
Some description`,
			expected: "Bug Fix",
		},
		{
			name:     "returns empty for no bold text",
			body:     "Just some plain text without bold",
			expected: "",
		},
		{
			name:     "returns empty for empty string",
			body:     "",
			expected: "",
		},
		{
			name: "extracts first bold when multiple exist",
			body: `**First Bold** is here
**Second Bold** is here too`,
			expected: "First Bold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTitle(tt.body)
			if result != tt.expected {
				t.Errorf("extractTitle() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests HTTP sémantiques — HandleDownloadUpdate et HandleApplyUpdate
// Issues #44 : codes 400/404/503/500 et validation path-traversal
// ---------------------------------------------------------------------------

// TestHandleAPIUpdatesDownload_MethodNotAllowed vérifie que GET /api/updates/download
// retourne 405 Method Not Allowed (le check méthode est dans le wrapper http.go).
func TestHandleAPIUpdatesDownload_MethodNotAllowed(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/updates/download", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/updates/download: expected 405, got %d", w.Code)
	}
}

// TestHandleAPIUpdatesApply_MethodNotAllowed vérifie que GET /api/updates/apply
// retourne 405 Method Not Allowed.
func TestHandleAPIUpdatesApply_MethodNotAllowed(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/updates/apply", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/updates/apply: expected 405, got %d", w.Code)
	}
}

// TestHandleDownloadUpdate_InvalidBody vérifie que POST avec un body JSON invalide
// retourne 400 Bad Request.
func TestHandleDownloadUpdate_InvalidBody(t *testing.T) {
	updater := newTestUpdater(t, "3.5.5")

	req := httptest.NewRequest(http.MethodPost, "/api/updates/download", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	updater.HandleDownloadUpdate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Invalid JSON body: expected 400, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp DownloadResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err == nil {
		if resp.Success {
			t.Error("Expected success=false for invalid JSON body")
		}
	}
}

// TestHandleDownloadUpdate_EmptyVersion vérifie que POST avec version vide
// retourne 400 Bad Request.
func TestHandleDownloadUpdate_EmptyVersion(t *testing.T) {
	updater := newTestUpdater(t, "3.5.5")

	req := httptest.NewRequest(http.MethodPost, "/api/updates/download", strings.NewReader(`{"version":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	updater.HandleDownloadUpdate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Empty version: expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestHandleDownloadUpdate_VersionNotFound vérifie que POST avec une version inexistante
// retourne 404 Not Found. On pré-peuple le cache GitHub pour éviter tout appel réseau.
func TestHandleDownloadUpdate_VersionNotFound(t *testing.T) {
	updater := newTestUpdater(t, "3.5.5")

	// Pré-peupler le cache avec une version différente (v1.0.0 seulement)
	// afin d'éviter tout appel réseau et tester le chemin "version introuvable"
	platform := GetPlatformString()
	updater.githubClient.cache.set([]GitHubRelease{
		{
			TagName: "v1.0.0",
			Assets: []GitHubAsset{
				{
					Name: fmt.Sprintf("buzzcontrol-v1.0.0-%s.exe", platform),
					Size: 10 * 1024 * 1024, // 10 MB, bien au-dessus du minimum
				},
			},
		},
	})

	// Demander une version inexistante
	body := strings.NewReader(`{"version":"9.9.9"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/updates/download", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	updater.HandleDownloadUpdate(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Version not found: expected 404, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp DownloadResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err == nil {
		if resp.Success {
			t.Error("Expected success=false for unknown version")
		}
	}
}

// TestHandleApplyUpdate_PathTraversal vérifie que POST avec un path hors tempDir
// retourne 400 Bad Request (protection contre path traversal).
func TestHandleApplyUpdate_PathTraversal(t *testing.T) {
	updater := newTestUpdater(t, "3.5.5")

	// Créer un fichier suffisamment grand dans un répertoire HORS du tempDir de l'updater.
	// Le tempDir de l'updater est os.TempDir()+"/buzzcontrol-updates".
	// t.TempDir() crée quelque chose comme os.TempDir()+"/TestXXXX/001" qui ne
	// commence pas par "buzzcontrol-updates", déclenchant ainsi le refus path-traversal.
	outsideDir := t.TempDir()
	maliciousFile := filepath.Join(outsideDir, "malicious.bin")

	f, err := os.Create(maliciousFile)
	if err != nil {
		t.Fatalf("Cannot create test file: %v", err)
	}
	// Utiliser Truncate pour créer un fichier sparse >= MinBinarySize sans écrire les données
	if err := f.Truncate(MinBinarySize + 1); err != nil {
		f.Close()
		t.Fatalf("Cannot resize test file to MinBinarySize+1: %v", err)
	}
	f.Close()

	body := strings.NewReader(fmt.Sprintf(`{"version":"3.6.0","path":%q}`, maliciousFile))
	req := httptest.NewRequest(http.MethodPost, "/api/updates/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	updater.HandleApplyUpdate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Path traversal: expected 400, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp ApplyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err == nil {
		if resp.Success {
			t.Error("Expected success=false for path traversal attempt")
		}
		if !strings.Contains(resp.Error, "Invalid file path") {
			t.Errorf("Expected 'Invalid file path' error, got: %q", resp.Error)
		}
	}
}

// TestHandleApplyUpdate_PathTraversal_AdjacentPrefix vérifie que la protection
// path-traversal rejette un répertoire dont le nom commence par le même préfixe
// que tempDir (ex: buzzcontrol-updates-adjacent ne doit pas passer).
// Ce cas aurait été accepté par l'ancien check `strings.HasPrefix(absPath, tempDir)`
// sans séparateur de chemin.
func TestHandleApplyUpdate_PathTraversal_AdjacentPrefix(t *testing.T) {
	updater := newTestUpdater(t, "3.5.5")

	// Construire un répertoire adjacent : même base mais suffixe additionnel.
	// Ex: /tmp/buzzcontrol-updates-adjacent
	adjacentDir := updater.updatesDir + "-adjacent"
	if err := os.MkdirAll(adjacentDir, 0755); err != nil {
		t.Fatalf("Cannot create adjacent dir: %v", err)
	}
	defer os.RemoveAll(adjacentDir)

	// Créer un fichier dans ce répertoire adjacent
	maliciousFile := filepath.Join(adjacentDir, "malicious.bin")
	f, err := os.Create(maliciousFile)
	if err != nil {
		t.Fatalf("Cannot create malicious file: %v", err)
	}
	// Fichier sparse >= MinBinarySize pour dépasser la validation de taille
	if err := f.Truncate(MinBinarySize + 1); err != nil {
		f.Close()
		t.Fatalf("Cannot resize malicious file: %v", err)
	}
	f.Close()

	body := strings.NewReader(fmt.Sprintf(`{"version":"3.6.0","path":%q}`, maliciousFile))
	req := httptest.NewRequest(http.MethodPost, "/api/updates/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	updater.HandleApplyUpdate(w, req)

	// Le path /tmp/buzzcontrol-updates-adjacent/malicious.bin commence par
	// /tmp/buzzcontrol-updates mais PAS par /tmp/buzzcontrol-updates/ (avec sep).
	// Le handler doit retourner 400 Bad Request.
	if w.Code != http.StatusBadRequest {
		t.Errorf("Adjacent prefix directory: expected 400, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp ApplyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err == nil {
		if resp.Success {
			t.Error("Expected success=false for adjacent prefix path traversal")
		}
		if !strings.Contains(resp.Error, "Invalid file path") {
			t.Errorf("Expected 'Invalid file path' error, got: %q", resp.Error)
		}
	}
}

// TestHandleApplyUpdate_FileNotFound vérifie que POST avec un path de fichier inexistant
// retourne 404 Not Found.
func TestHandleApplyUpdate_FileNotFound(t *testing.T) {
	updater := newTestUpdater(t, "3.5.5")

	// Path à l'intérieur du updatesDir mais le fichier n'existe pas
	nonExistentPath := filepath.Join(updater.updatesDir, "buzzcontrol-v9.9.9-linux-amd64")

	body := strings.NewReader(fmt.Sprintf(`{"version":"9.9.9","path":%q}`, nonExistentPath))
	req := httptest.NewRequest(http.MethodPost, "/api/updates/apply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	updater.HandleApplyUpdate(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("File not found: expected 404, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp ApplyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err == nil {
		if resp.Success {
			t.Error("Expected success=false for non-existent file")
		}
	}
}
