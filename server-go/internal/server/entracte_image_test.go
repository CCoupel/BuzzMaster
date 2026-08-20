package server

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// buildTARWithSingleFile returns a minimal TAR archive containing one file
// entry at tarPath with the given content — a generic sibling of
// buildTARWithGameConfig/buildTARWithGameState (gameconfig_backup_test.go)
// for a plain data file rather than a JSON config section.
func buildTARWithSingleFile(t *testing.T, tarPath string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name: tarPath,
		Mode: 0644,
		Size: int64(len(content)),
	}); err != nil {
		t.Fatalf("could not write TAR header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("could not write TAR content: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("could not close TAR writer: %v", err)
	}
	return buf.Bytes()
}

// ---------------------------------------------------------------------------
// B7 (#119, v6.5.2, contract http-endpoints.md §"Mode ENTRACTE") —
// GET/POST/DELETE /api/game/entracte-image, and its inclusion in the
// selective backup/restore/reset "medias" flag alongside backgrounds/
// categories (the plan's documented pitfall: the default-question-image and
// new-game-backgrounds/ are ALREADY missing from that list, #152 — a
// dedicated files/entracte/ directory, explicitly wired into the same three
// code paths, avoids reproducing that gap here).
// ---------------------------------------------------------------------------

func multipartImageBody(t *testing.T, filename string, content []byte) (body *bytes.Buffer, contentType string) {
	t.Helper()
	body = &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("could not create multipart field: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("could not write image into multipart body: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("could not close multipart writer: %v", err)
	}
	return body, mw.FormDataContentType()
}

func TestHTTPServer_EntracteImage_GET_NoImage_404(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("GET", "/api/game/entracte-image", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 when no image has ever been uploaded, got %d", w.Code)
	}
}

func TestHTTPServer_EntracteImage_POST_Upload_Then_GET(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	fakePNG := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG magic bytes, content irrelevant to the handler
	body, contentType := multipartImageBody(t, "panel.png", fakePNG)

	req := httptest.NewRequest("POST", "/api/game/entracte-image", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 on upload, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		IsCustom bool `json:"is_custom"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}
	if !resp.IsCustom {
		t.Error("Expected is_custom=true after upload")
	}

	// Stored under the DEDICATED files/entracte/ directory, not the shared
	// files/ root (unlike default-question-image) — the whole point of B7's
	// backup-gap fix.
	onDisk := filepath.Join(dataDir, "files", "entracte", "entracte-image.png")
	if _, err := os.Stat(onDisk); err != nil {
		t.Errorf("Expected image at %s, got error: %v", onDisk, err)
	}

	if !server.HasCustomEntracteImage() {
		t.Error("HasCustomEntracteImage() should be true after upload")
	}

	// GET now serves it.
	getReq := httptest.NewRequest("GET", "/api/game/entracte-image", nil)
	getW := httptest.NewRecorder()
	server.mux.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("Expected 200 on GET after upload, got %d", getW.Code)
	}
	if ct := getW.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Expected Content-Type image/png, got %s", ct)
	}
	if !bytes.Equal(getW.Body.Bytes(), fakePNG) {
		t.Error("GET body does not match the uploaded image content")
	}
}

func TestHTTPServer_EntracteImage_POST_ReplacesPreviousExtension(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	body1, ct1 := multipartImageBody(t, "a.png", []byte("first"))
	req1 := httptest.NewRequest("POST", "/api/game/entracte-image", body1)
	req1.Header.Set("Content-Type", ct1)
	server.mux.ServeHTTP(httptest.NewRecorder(), req1)

	body2, ct2 := multipartImageBody(t, "b.jpg", []byte("second"))
	req2 := httptest.NewRequest("POST", "/api/game/entracte-image", body2)
	req2.Header.Set("Content-Type", ct2)
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("Expected 200 on second upload, got %d: %s", w2.Code, w2.Body.String())
	}

	// The .png from the first upload must be gone — POST replaces regardless
	// of the previous file's own extension.
	if _, err := os.Stat(filepath.Join(dataDir, "files", "entracte", "entracte-image.png")); !os.IsNotExist(err) {
		t.Errorf("Expected the old .png to be removed, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "entracte", "entracte-image.jpg")); err != nil {
		t.Errorf("Expected the new .jpg to exist: %v", err)
	}
}

func TestHTTPServer_EntracteImage_DELETE(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	body, contentType := multipartImageBody(t, "panel.png", []byte("data"))
	req := httptest.NewRequest("POST", "/api/game/entracte-image", body)
	req.Header.Set("Content-Type", contentType)
	server.mux.ServeHTTP(httptest.NewRecorder(), req)

	delReq := httptest.NewRequest("DELETE", "/api/game/entracte-image", nil)
	delW := httptest.NewRecorder()
	server.mux.ServeHTTP(delW, delReq)
	if delW.Code != http.StatusOK {
		t.Fatalf("Expected 200 on delete, got %d: %s", delW.Code, delW.Body.String())
	}
	var resp struct {
		IsCustom bool `json:"is_custom"`
	}
	if err := json.Unmarshal(delW.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}
	if resp.IsCustom {
		t.Error("Expected is_custom=false after delete")
	}
	if server.HasCustomEntracteImage() {
		t.Error("HasCustomEntracteImage() should be false after delete")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "entracte", "entracte-image.png")); !os.IsNotExist(err) {
		t.Errorf("Expected the image file to be removed from disk, stat error: %v", err)
	}

	// DELETE with no image present -> 404, not a silent 200.
	delReq2 := httptest.NewRequest("DELETE", "/api/game/entracte-image", nil)
	delW2 := httptest.NewRecorder()
	server.mux.ServeHTTP(delW2, delReq2)
	if delW2.Code != http.StatusNotFound {
		t.Errorf("Expected 404 deleting when no image exists, got %d", delW2.Code)
	}
}

// TestHTTPServer_BackupSelect_IncludesEntracteWithMedias mirrors
// TestHTTPServer_BackupSelect_IncludesGameConfigWithHistory
// (gameconfig_backup_test.go) for the "medias" flag instead of "history".
func TestHTTPServer_BackupSelect_IncludesEntracteWithMedias(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	entracteDir := filepath.Join(dataDir, "files", "entracte")
	if err := os.MkdirAll(entracteDir, 0755); err != nil {
		t.Fatalf("could not create fixture entracte dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(entracteDir, "entracte-image.png"), []byte("fixture"), 0644); err != nil {
		t.Fatalf("could not write fixture image: %v", err)
	}

	t.Run("medias=true includes files/entracte/", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/backup-select?medias=true", nil)
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !tarContains(t, w.Body.Bytes(), "files/entracte/entracte-image.png") {
			t.Error("Expected files/entracte/entracte-image.png in the TAR when medias=true")
		}
	})

	t.Run("medias=false excludes files/entracte/", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/backup-select?teams=true", nil)
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if tarContains(t, w.Body.Bytes(), "files/entracte/entracte-image.png") {
			t.Error("Did not expect files/entracte/ in the TAR when medias is not selected")
		}
	})
}

// TestHTTPServer_ResetSelect_Entracte mirrors
// TestHTTPServer_ResetSelect_GameConfig for the medias flag: files/entracte/
// is removed and recreated empty.
func TestHTTPServer_ResetSelect_Entracte(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	entracteDir := filepath.Join(dataDir, "files", "entracte")
	os.MkdirAll(entracteDir, 0755)
	os.WriteFile(filepath.Join(entracteDir, "entracte-image.png"), []byte("fixture"), 0644)

	req := httptest.NewRequest("POST", "/reset-select?medias=true", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if _, err := os.Stat(filepath.Join(entracteDir, "entracte-image.png")); !os.IsNotExist(err) {
		t.Errorf("Expected the image to be gone after reset, stat error: %v", err)
	}
	if _, err := os.Stat(entracteDir); err != nil {
		t.Errorf("Expected files/entracte/ directory to still exist (recreated empty), got: %v", err)
	}
	if server.HasCustomEntracteImage() {
		t.Error("HasCustomEntracteImage() should be false after reset")
	}
}

// TestHTTPServer_Restore_Entracte mirrors TestHTTPServer_Restore_GameConfig
// for files/entracte/.
func TestHTTPServer_Restore_Entracte(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	tarData := buildTARWithSingleFile(t, "files/entracte/entracte-image.png", []byte("restored-image"))
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
	if !resp.Restored["entracte"] {
		t.Errorf("Expected restored.entracte=true, got %+v", resp.Restored)
	}

	onDisk, err := os.ReadFile(filepath.Join(dataDir, "files", "entracte", "entracte-image.png"))
	if err != nil {
		t.Fatalf("entracte-image.png was not written to disk: %v", err)
	}
	if string(onDisk) != "restored-image" {
		t.Errorf("Restored image content mismatch: got %q", onDisk)
	}
	if !server.HasCustomEntracteImage() {
		t.Error("HasCustomEntracteImage() should be true immediately after restore, without a restart")
	}
}
