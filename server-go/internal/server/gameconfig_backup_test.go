package server

// Tests for #150 (plan task 17.6): game-config.json's integration into
// selective backup (/backup-select), restore (/restore) and reset
// (/reset-select). All three piggyback on the existing "history" flag —
// see handleBackupSelect/handleResetSelect's doc comments in http.go for
// why — except restore, whose detection is content-based (presence of
// "config/game-config.json" in the uploaded TAR), independent of any flag.

import (
	"archive/tar"
	"bytes"
	"buzzcontrol/internal/config"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestHTTPServer_BackupSelect_IncludesGameConfigWithHistory verifies
// game-config.json is included in the TAR produced by /backup-select when
// history=true, and absent when history is not selected.
func TestHTTPServer_BackupSelect_IncludesGameConfigWithHistory(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	gs := &config.GameSettings{}
	config.ApplyGameSettingsDefaults(gs)
	gs.Game.DefaultDelay = 77
	if err := config.SaveGameSettings(gs); err != nil {
		t.Fatalf("could not write fixture game-config.json: %v", err)
	}

	t.Run("history=true includes game-config.json", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/backup-select?history=true", nil)
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !tarContains(t, w.Body.Bytes(), "config/game-config.json") {
			t.Errorf("Expected config/game-config.json in the TAR when history=true")
		}
	})

	t.Run("history=false (teams only) excludes game-config.json", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/backup-select?teams=true", nil)
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if tarContains(t, w.Body.Bytes(), "config/game-config.json") {
			t.Errorf("Did not expect config/game-config.json in the TAR when only teams=true")
		}
	})

	_ = dataDir
}

// tarContains reports whether name appears as a header in the TAR bytes.
func tarContains(t *testing.T, data []byte, name string) bool {
	t.Helper()
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		header, err := tr.Next()
		if err != nil {
			return false
		}
		if header.Name == name {
			return true
		}
	}
}

// buildTARWithGameConfig returns a minimal TAR archive containing a single
// config/game-config.json entry with the given content.
func buildTARWithGameConfig(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	data := []byte(content)
	if err := tw.WriteHeader(&tar.Header{
		Name: "config/game-config.json",
		Mode: 0644,
		Size: int64(len(data)),
	}); err != nil {
		t.Fatalf("could not write TAR header: %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("could not write TAR content: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("could not close TAR writer: %v", err)
	}
	return buf.Bytes()
}

// multipartRestoreBody wraps tarData as the multipart/form-data body
// /restore expects (field name "file").
func multipartRestoreBody(t *testing.T, tarData []byte) (body *bytes.Buffer, contentType string) {
	t.Helper()
	body = &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile("file", "backup.tar")
	if err != nil {
		t.Fatalf("could not create multipart field: %v", err)
	}
	if _, err := part.Write(tarData); err != nil {
		t.Fatalf("could not write TAR into multipart body: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("could not close multipart writer: %v", err)
	}
	return body, mw.FormDataContentType()
}

// TestHTTPServer_Restore_GameConfig verifies /restore extracts
// config/game-config.json from an uploaded TAR, writes it to disk, and
// refreshes the in-memory GameSettings singleton immediately.
func TestHTTPServer_Restore_GameConfig(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	tarData := buildTARWithGameConfig(t, `{"game":{"default_delay":66},"neon_effect":{"enabled":true,"arc_width":123}}`)
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
	if !resp.Restored["gameConfig"] {
		t.Errorf("Expected restored.gameConfig=true, got %+v", resp.Restored)
	}

	// File must exist on disk at the expected path.
	onDisk, err := os.ReadFile(filepath.Join(dataDir, "config", "game-config.json"))
	if err != nil {
		t.Fatalf("game-config.json was not written to disk: %v", err)
	}
	var onDiskGS config.GameSettings
	if err := json.Unmarshal(onDisk, &onDiskGS); err != nil {
		t.Fatalf("restored game-config.json is not valid JSON: %v", err)
	}
	if onDiskGS.Game.DefaultDelay != 66 {
		t.Errorf("Expected restored default_delay=66 on disk, got %d", onDiskGS.Game.DefaultDelay)
	}

	// In-memory singleton must reflect the restored values immediately
	// (no restart needed) — this is what feeds GET /game-config.json and
	// the WS neon_effect payload.
	gs := config.GetGameSettings()
	if gs.Game.DefaultDelay != 66 {
		t.Errorf("Expected in-memory default_delay=66 after restore, got %d", gs.Game.DefaultDelay)
	}
	if !gs.NeonEffect.Enabled || gs.NeonEffect.ArcWidth != 123 {
		t.Errorf("Expected in-memory neon_effect restored, got %+v", gs.NeonEffect)
	}
}

// TestHTTPServer_ResetSelect_GameConfig verifies /reset-select?history=true
// resets game-config.json to defaults, both on disk and in memory.
func TestHTTPServer_ResetSelect_GameConfig(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	gs := &config.GameSettings{}
	config.ApplyGameSettingsDefaults(gs)
	gs.Game.DefaultDelay = 90
	gs.NeonEffect.Enabled = true
	if err := config.SaveGameSettings(gs); err != nil {
		t.Fatalf("could not write fixture game-config.json: %v", err)
	}
	config.SetGameSettingsInstance(gs)

	req := httptest.NewRequest("POST", "/reset-select?history=true", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	after := config.GetGameSettings()
	if after.Game.DefaultDelay != 30 {
		t.Errorf("Expected default_delay reset to the default (30), got %d", after.Game.DefaultDelay)
	}
	if after.NeonEffect.Enabled {
		t.Errorf("Expected neon_effect.enabled reset to the default (false), got true")
	}

	onDisk, err := os.ReadFile(filepath.Join(dataDir, "config", "game-config.json"))
	if err != nil {
		t.Fatalf("game-config.json should still exist (rewritten with defaults), got error: %v", err)
	}
	var onDiskGS config.GameSettings
	if err := json.Unmarshal(onDisk, &onDiskGS); err != nil {
		t.Fatalf("reset game-config.json is not valid JSON: %v", err)
	}
	if onDiskGS.Game.DefaultDelay != 30 {
		t.Errorf("Expected default_delay=30 on disk after reset, got %d", onDiskGS.Game.DefaultDelay)
	}
}
