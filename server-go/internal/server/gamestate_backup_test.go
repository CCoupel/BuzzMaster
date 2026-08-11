package server

// Tests for #141 (plan task 22): game_state.json's integration into
// selective backup (/backup-select), restore (/restore) and reset
// (/reset-select). Same "history" anchor and content-based restore
// detection as game-config.json (#150) — see gameconfig_backup_test.go and
// this file's individual test comments for the reasoning.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestHTTPServer_BackupSelect_IncludesGameStateWithHistory mirrors
// TestHTTPServer_BackupSelect_IncludesGameConfigWithHistory (#150) for
// game_state.json.
func TestHTTPServer_BackupSelect_IncludesGameStateWithHistory(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	server.engine.SetQuizMeta("Mon Quiz", "", "", nil, nil, "", "")

	t.Run("history=true includes game_state.json", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/backup-select?history=true", nil)
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !tarContains(t, w.Body.Bytes(), "config/game_state.json") {
			t.Errorf("Expected config/game_state.json in the TAR when history=true")
		}
	})

	t.Run("history=false (teams only) excludes game_state.json", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/backup-select?teams=true", nil)
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if tarContains(t, w.Body.Bytes(), "config/game_state.json") {
			t.Errorf("Did not expect config/game_state.json in the TAR when only teams=true")
		}
	})
}

// TestHTTPServer_Restore_GameState verifies /restore extracts
// config/game_state.json from an uploaded TAR, writes it to disk, and
// reloads it into the engine's live GameState immediately.
func TestHTTPServer_Restore_GameState(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	tarData := buildTARWithGameState(t, `{"format_version":1,"QUIZ_NAME":"Restored Quiz","VIRTUAL_PLAYER_LIMIT":33}`)
	body, contentType := multipartRestoreBody(t, tarData)

	req := httptest.NewRequest("POST", "/restore", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// File must exist on disk at the expected path.
	if _, err := os.Stat(filepath.Join(dataDir, "config", "game_state.json")); err != nil {
		t.Fatalf("game_state.json was not written to disk: %v", err)
	}

	// The live engine state must reflect the restored values immediately.
	state := server.engine.GetState()
	if state.QuizName != "Restored Quiz" {
		t.Errorf("Expected QuizName=Restored Quiz after restore, got %q", state.QuizName)
	}
	if state.VirtualPlayerLimit != 33 {
		t.Errorf("Expected VirtualPlayerLimit=33 after restore, got %d", state.VirtualPlayerLimit)
	}
}

// TestHTTPServer_ResetSelect_GameState verifies /reset-select?history=true
// clears quiz metadata to defaults, both in the live engine state and on
// disk (file removed, matching history.json's own reset convention).
func TestHTTPServer_ResetSelect_GameState(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	server.engine.SetQuizMeta("Mon Quiz", "Theme", "", nil, nil, "", "")
	server.engine.SetVirtualPlayerLimit(15)

	statePath := filepath.Join(dataDir, "config", "game_state.json")
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("precondition: game_state.json should exist before reset: %v", err)
	}

	req := httptest.NewRequest("POST", "/reset-select?history=true", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	state := server.engine.GetState()
	if state.QuizName != "" {
		t.Errorf("Expected QuizName cleared after reset, got %q", state.QuizName)
	}
	if state.VirtualPlayerLimit != 20 {
		t.Errorf("Expected VirtualPlayerLimit reset to the default (20), got %d", state.VirtualPlayerLimit)
	}

	if _, err := os.Stat(statePath); err == nil {
		t.Errorf("Expected game_state.json removed after reset (same convention as history.json), but it still exists")
	}
}
