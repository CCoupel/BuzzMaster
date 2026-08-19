package server

import (
	"archive/tar"
	"bytes"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func setupTestHTTPServer(t *testing.T) (*HTTPServer, string) {
	// Isolate config.Get()/config.Save() from the tracked fixture
	// internal/server/config.json (bugfix #143): both resolve the relative
	// path "config.json" against the process CWD, and this helper backs
	// every TestHTTPServer_Config_POST* test, whose real handleConfig call
	// ends in config.Save(). t.Chdir(t.TempDir()) (Go 1.24) redirects that
	// write into a throwaway directory and is auto-restored by t.Cleanup —
	// same pattern as internal/config/config_merge_test.go.
	t.Chdir(t.TempDir())

	// Initialize config - use same temp dir for both DataDir and QuestionsDir
	dataDir := t.TempDir()

	// Trigger once.Do first by calling Get(), then override with SetInstance
	_ = config.Get()

	cfg := &config.Config{
		Server: config.ServerConfig{
			HTTPPort: 8080,
		},
		Storage: config.StorageConfig{
			DataDir:      dataDir,
			QuestionsDir: filepath.Join(dataDir, "files", "questions"),
		},
		Version: "2.0.0-test",
	}
	config.SetInstance(cfg)

	// #150 — mirror main.go's startup wiring: GameConfigPath() must resolve
	// under the SAME dataDir as h.dataDir (below), because
	// handleBackupSelect/handleResetSelect/handleRestore mix
	// filepath.Join(h.dataDir, "config") for teams/bumpers/history with
	// config.GameConfigPath() for game-config.json — the two must agree, or
	// tests silently look for game-config.json in the wrong directory
	// (config.GameConfigPath()'s "data/config/..." default, relative to the
	// t.Chdir'd CWD above, which is a DIFFERENT temp dir than dataDir).
	config.SetGameConfigPath(filepath.Join(dataDir, "config", "game-config.json"))

	engine := game.NewEngine()
	// #141 — mirror main.go's startup wiring (same reasoning as
	// config.SetGameConfigPath above): without this, engine.SaveState/
	// LoadState/ClearQuizMeta are silent no-ops (statePath == ""), and
	// handleRestore's game_state.json case could never be genuinely
	// exercised by a test.
	engine.SetStatePath(filepath.Join(dataDir, "config", "game_state.json"))
	wsHub := NewWebSocketHub()
	go wsHub.Run()
	logsHub := NewLogsWebSocketHub(100)
	go logsHub.Run()

	server := NewHTTPServer(8080, engine, wsHub, NewBuzzerWebSocketHub(), logsHub)
	server.SetWebDir(cfg.Storage.DataDir)
	server.setupRoutes()

	return server, cfg.Storage.DataDir
}

func TestHTTPServer_Version(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("GET", "/version", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "2.0.0-test" {
		t.Errorf("Expected version 2.0.0-test, got %s", body)
	}
}

func TestHTTPServer_ListGame(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Add some data to the engine
	server.engine.SetTeams(map[string]*game.Team{
		"red": {Name: "Team Red", Score: 100},
	})

	req := httptest.NewRequest("GET", "/listGame", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Errorf("Response is not valid JSON: %v", err)
	}
}

func TestHTTPServer_Questions_Empty(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("GET", "/questions", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// /questions returns an object (ESP32-compatible format with FSINFO key), not an array
	var questions map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &questions); err != nil {
		t.Errorf("Response is not valid JSON: %v", err)
	}

	// Only FSINFO key should be present when there are no questions
	if _, ok := questions["FSINFO"]; !ok {
		t.Error("Expected FSINFO key in response")
	}
	// No question entries (only FSINFO)
	if len(questions) != 1 {
		t.Errorf("Expected 1 key (FSINFO only), got %d", len(questions))
	}
}

func TestHTTPServer_Questions_WithData(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// Create a test question
	questionsDir := filepath.Join(dataDir, "files", "questions", "1")
	os.MkdirAll(questionsDir, 0755)

	questionData := map[string]interface{}{
		"ID":       "1",
		"QUESTION": "What is 2+2?",
		"ANSWER":   "4",
	}
	data, _ := json.Marshal(questionData)
	os.WriteFile(filepath.Join(questionsDir, "question.json"), data, 0644)

	req := httptest.NewRequest("GET", "/questions", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	// /questions returns an object (ESP32-compatible format with FSINFO key)
	var questions map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &questions); err != nil {
		t.Errorf("Response is not valid JSON: %v", err)
	}

	// Should have 1 question entry + FSINFO = 2 keys
	if len(questions) != 2 {
		t.Errorf("Expected 2 keys (1 question + FSINFO), got %d", len(questions))
	}
	if _, ok := questions["/files/questions/1"]; !ok {
		t.Error("Expected question key /files/questions/1")
	}
}

func TestHTTPServer_RootRedirect(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("Expected redirect (302), got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "testSPA.html") {
		t.Errorf("Expected redirect to testSPA.html, got %s", location)
	}
}

func TestHTTPServer_IndexRedirect(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("GET", "/index.html", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("Expected redirect (302), got %d", w.Code)
	}
}

func TestHTTPServer_WindowsCaptivePortal(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	tests := []string{"/connecttest.txt", "/ncsi.txt"}

	for _, path := range tests {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()

		server.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: Expected 200, got %d", path, w.Code)
		}

		body := w.Body.String()
		if body != "Microsoft NCSI" {
			t.Errorf("%s: Expected 'Microsoft NCSI', got '%s'", path, body)
		}

		// Check cache headers
		cacheControl := w.Header().Get("Cache-Control")
		if !strings.Contains(cacheControl, "no-cache") {
			t.Errorf("%s: Expected no-cache header", path)
		}
	}
}

func TestHTTPServer_Config_GET(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("GET", "/config.json", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Parse response to verify it's valid JSON with the system sections
	// (neon_effect moved to GET /game-config.json by #150 — see
	// TestHTTPServer_GameConfig_GET).
	var cfg map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}

	if _, ok := cfg["server"]; !ok {
		t.Errorf("Expected server section in config, got: %v", cfg)
	}
	if _, ok := cfg["neon_effect"]; ok {
		t.Errorf("neon_effect must no longer be part of /config.json (#150), got: %v", cfg)
	}
}

// TestHTTPServer_Config_GET_AISecretMasked verifies GET /config.json never
// leaks the Anthropic API key and exposes only the derived boolean
// api_key_configured (contract ai-generation.md §2, CA3, CA12).
func TestHTTPServer_Config_GET_AISecretMasked(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	cfg := config.Get()
	cfg.AI.AnthropicAPIKey = "sk-ant-super-secret-value"
	// Mi-2 (code-review-20260806-101822.md): the Groq key follows the exact
	// same masking rule (contract ai-multi-provider.md §8, security M3) —
	// covered here alongside Anthropic's, previously only exercised
	// in-memory by ai_batching_test.go, never through this HTTP-level test.
	cfg.AI.GroqAPIKey = "gsk_super-secret-groq-value"
	config.SetInstance(cfg)

	req := httptest.NewRequest("GET", "/config.json", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "super-secret-value") {
		t.Fatalf("Anthropic API key leaked in GET /config.json response: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "super-secret-groq-value") {
		t.Fatalf("Groq API key leaked in GET /config.json response: %s", w.Body.String())
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}
	ai, ok := parsed["ai"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected ai section in response, got: %v", parsed)
	}
	if ai["anthropic_api_key"] != "" {
		t.Errorf("Expected anthropic_api_key=\"\", got %v", ai["anthropic_api_key"])
	}
	if ai["api_key_configured"] != true {
		t.Errorf("Expected api_key_configured=true, got %v", ai["api_key_configured"])
	}
	if ai["groq_api_key"] != "" {
		t.Errorf("Expected groq_api_key=\"\", got %v", ai["groq_api_key"])
	}
	if ai["groq_api_key_configured"] != true {
		t.Errorf("Expected groq_api_key_configured=true, got %v", ai["groq_api_key_configured"])
	}
}

// TestHTTPServer_Config_GET_AIKeyConfigured_FromEnvVarAlone is the end-to-end
// check for the security incident 2026-08-07 fix (docs/ADMIN_GUIDE.md
// "Configurer les clés API IA en production"): with config.json's fields
// left empty (the PROD-recommended state — no key on disk) and the key
// supplied only via BUZZCONTROL_*_API_KEY, GET /config.json must still
// report *_configured=true (otherwise the frontend keeps "✨ Générer via IA"
// disabled despite a working key) — and must never leak the env var's value
// either, same masking guarantee as the config.json-sourced case above.
func TestHTTPServer_Config_GET_AIKeyConfigured_FromEnvVarAlone(t *testing.T) {
	t.Setenv(config.EnvAnthropicAPIKey, "sk-ant-from-env-value")
	t.Setenv(config.EnvGroqAPIKey, "gsk_from-env-value")
	server, _ := setupTestHTTPServer(t)

	cfg := config.Get()
	cfg.AI.AnthropicAPIKey = "" // PROD state: nothing on disk
	cfg.AI.GroqAPIKey = ""
	config.SetInstance(cfg)

	req := httptest.NewRequest("GET", "/config.json", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "from-env-value") {
		t.Fatalf("Env-var-sourced API key leaked in GET /config.json response: %s", w.Body.String())
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}
	ai, ok := parsed["ai"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected ai section in response, got: %v", parsed)
	}
	if ai["api_key_configured"] != true {
		t.Errorf("Expected api_key_configured=true from the env var alone, got %v", ai["api_key_configured"])
	}
	if ai["groq_api_key_configured"] != true {
		t.Errorf("Expected groq_api_key_configured=true from the env var alone, got %v", ai["groq_api_key_configured"])
	}
}

// TestHTTPServer_GameConfig_POST is TestHTTPServer_Config_POST's #150
// successor: neon_effect moved from /config.json to /game-config.json.
func TestHTTPServer_GameConfig_POST(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Post a valid game config with neon_effect
	configJSON := `{
		"neon_effect": {
			"enabled": true,
			"arc_width": 90,
			"intensity_gap": 75,
			"rotation_speed": 5.5
		}
	}`

	req := httptest.NewRequest("POST", "/game-config.json", strings.NewReader(configJSON))
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response to verify validation
	var gs map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &gs); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}

	// Verify neon_effect was saved
	neonEffect, ok := gs["neon_effect"].(map[string]interface{})
	if !ok {
		t.Fatalf("neon_effect not found in response")
	}

	if neonEffect["enabled"] != true {
		t.Errorf("Expected enabled=true, got %v", neonEffect["enabled"])
	}
	if neonEffect["arc_width"] != float64(90) {
		t.Errorf("Expected arc_width=90, got %v", neonEffect["arc_width"])
	}

	// Defaults must be re-applied to fields the partial section left at zero
	// (contract ai-generation.md §0, carried over to game-config.json by
	// #150) — e.g. bar_offset was absent from the posted neon_effect object.
	if neonEffect["bar_offset"] != float64(20) {
		t.Errorf("Expected bar_offset default (20) re-applied, got %v", neonEffect["bar_offset"])
	}
}

// TestHTTPServer_Config_POST_GameSection_Rejected400 and
// TestHTTPServer_Config_POST_NeonEffectSection_Rejected400 are the
// regression tests for #150's BREAKING change: POST /config.json must
// reject a request that still carries "game" or "neon_effect", with a
// message pointing at the new endpoint, rather than silently accepting and
// discarding it.
func TestHTTPServer_Config_POST_GameSection_Rejected400(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"game":{"default_delay":45}}`))
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "/game-config.json") {
		t.Errorf("Expected the 400 body to name the new endpoint, got: %s", w.Body.String())
	}
}

func TestHTTPServer_Config_POST_NeonEffectSection_Rejected400(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"neon_effect":{"enabled":true}}`))
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "/game-config.json") {
		t.Errorf("Expected the 400 body to name the new endpoint, got: %s", w.Body.String())
	}
}

// TestHTTPServer_GameConfig_GET verifies GET /game-config.json returns the
// current singleton (defaults, absent any prior POST/migration).
func TestHTTPServer_GameConfig_GET(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("GET", "/game-config.json", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var gs map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &gs); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}
	if _, ok := gs["neon_effect"]; !ok {
		t.Errorf("Expected a neon_effect section in the response, got: %v", gs)
	}
	if _, ok := gs["game"]; !ok {
		t.Errorf("Expected a game section in the response, got: %v", gs)
	}
}

// TestHTTPServer_GameConfig_POST_PartialPreservesOtherSection is
// TestHTTPServer_Config_POST_PartialPreservesOtherSections' #150 sibling:
// the same additive, section-by-section merge semantics apply to
// /game-config.json's two sections ("game", "neon_effect").
func TestHTTPServer_GameConfig_POST_PartialPreservesOtherSection(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Set a non-default delay directly on the singleton.
	initial := config.GetGameSettings()
	initial.Game.DefaultDelay = 45
	config.SetGameSettingsInstance(initial)

	// Partial save touching only neon_effect must leave game.default_delay untouched.
	req := httptest.NewRequest("POST", "/game-config.json", strings.NewReader(`{"neon_effect":{"enabled":true}}`))
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	after := config.GetGameSettings()
	if after.Game.DefaultDelay != 45 {
		t.Errorf("Expected game.default_delay=45 to survive a neon_effect-only POST, got %d", after.Game.DefaultDelay)
	}
	if !after.NeonEffect.Enabled {
		t.Errorf("Expected neon_effect.enabled=true from the posted section")
	}

	// Reverse: saving "game" must not touch neon_effect.
	req2 := httptest.NewRequest("POST", "/game-config.json", strings.NewReader(`{"game":{"default_delay":60}}`))
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	after2 := config.GetGameSettings()
	if !after2.NeonEffect.Enabled {
		t.Errorf("Expected neon_effect.enabled to survive a POST of the game section, got %+v", after2.NeonEffect)
	}
	if after2.Game.DefaultDelay != 60 {
		t.Errorf("Expected game.default_delay=60 from the posted section, got %d", after2.Game.DefaultDelay)
	}
}

// TestHTTPServer_Config_POST_PartialPreservesOtherSections is the regression
// test for the destructive bug fixed by #8 (contract ai-generation.md §0,
// CA1): a POST containing only one section must leave every other section —
// including one it never mentions — untouched on disk and in memory.
func TestHTTPServer_Config_POST_PartialPreservesOtherSections(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	initial := config.Get()
	initial.WiFi = config.WiFiConfig{SSID: "MyHomeWifi", Password: "supersecret"}
	initial.AI.AnthropicAPIKey = "sk-ant-should-survive"
	initial.Storage.QuestionsDir = filepath.Join(dataDir, "files", "questions")
	initial.Storage.FilesDir = filepath.Join(dataDir, "files")
	config.SetInstance(initial)

	// Partial save touching only "server" (neon_effect moved to
	// /game-config.json by #150 — see TestHTTPServer_GameConfig_POST_* for
	// its own partial-preserve coverage of the game-settings side).
	req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"server":{"debug":true}}`))
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	after := config.Get()
	if after.WiFi.SSID != "MyHomeWifi" || after.WiFi.Password != "supersecret" {
		t.Errorf("WiFi section was wiped by an unrelated partial save: %+v", after.WiFi)
	}
	if after.AI.AnthropicAPIKey != "sk-ant-should-survive" {
		t.Errorf("AI API key was wiped by an unrelated partial save: %q", after.AI.AnthropicAPIKey)
	}
	if after.Storage.QuestionsDir == "" || after.Storage.FilesDir == "" {
		t.Errorf("Storage section was wiped by an unrelated partial save: %+v", after.Storage)
	}
	if !after.Server.Debug {
		t.Errorf("Expected server.debug=true from the posted section")
	}

	// Also assert the reverse: saving the "wifi" section must not touch server.debug.
	req2 := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"wifi":{"ssid":"AnotherWifi","password":"anotherpass"}}`))
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	after2 := config.Get()
	if !after2.Server.Debug {
		t.Errorf("Expected server.debug to survive a POST of the wifi section, got %+v", after2.Server)
	}
	if after2.WiFi.SSID != "AnotherWifi" {
		t.Errorf("Expected wifi.ssid=AnotherWifi from the posted section, got %q", after2.WiFi.SSID)
	}
}

// TestHTTPServer_Config_POST_APIKeyPreservation covers the AI-key-specific
// merge exception (contract ai-generation.md §2, CA2): absent or empty key
// preserves the stored value; clear_api_key erases it; a malformed key is
// rejected with 400 and the store is left unchanged.
func TestHTTPServer_Config_POST_APIKeyPreservation(t *testing.T) {
	t.Run("absent ai section preserves key", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.AnthropicAPIKey = "sk-ant-original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"server":{"debug":true}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if got := config.Get().AI.AnthropicAPIKey; got != "sk-ant-original" {
			t.Errorf("Expected key preserved, got %q", got)
		}
	})

	t.Run("empty key in ai section preserves key", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.AnthropicAPIKey = "sk-ant-original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"ai":{"anthropic_api_key":"","model":"claude-opus-5"}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if got := config.Get().AI.AnthropicAPIKey; got != "sk-ant-original" {
			t.Errorf("Expected key preserved on empty string, got %q", got)
		}
	})

	t.Run("clear_api_key erases the key", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.AnthropicAPIKey = "sk-ant-original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"ai":{"clear_api_key":true}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if got := config.Get().AI.AnthropicAPIKey; got != "" {
			t.Errorf("Expected key cleared, got %q", got)
		}
	})

	t.Run("new key replaces the old one", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.AnthropicAPIKey = "sk-ant-original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"ai":{"anthropic_api_key":"sk-ant-new-value"}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if got := config.Get().AI.AnthropicAPIKey; got != "sk-ant-new-value" {
			t.Errorf("Expected key replaced, got %q", got)
		}
	})

	t.Run("malformed key is rejected with 400 and store is untouched", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.AnthropicAPIKey = "sk-ant-original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"ai":{"anthropic_api_key":"not-a-valid-key"}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if got := config.Get().AI.AnthropicAPIKey; got != "sk-ant-original" {
			t.Errorf("Expected key untouched after rejected request, got %q", got)
		}
	})

	t.Run("response never echoes the stored key", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.AnthropicAPIKey = "sk-ant-original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"ai":{"model":"claude-opus-5"}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if strings.Contains(w.Body.String(), "sk-ant-original") {
			t.Errorf("POST response leaked the API key: %s", w.Body.String())
		}
	})

	// Mi-1 (code-review-20260806-101822.md), corrected after a real bug was
	// found in QA (_work/handoff/task-dev-backend-20260806-103004.md): an
	// earlier version of this coverage treated the "ai" section as
	// wholesale-replaced like every other config section (matching contract
	// ai-generation.md §0's literal wording at the time), and asserted that a
	// key-only POST resets batch_size/provider/etc. to their defaults. That
	// assertion PASSED against the code as it was, but the code itself was
	// wrong: ConfigPage.jsx saves individual ai.* settings from separate
	// buttons (provider select, API key field, batching sliders —
	// ConfigPage.jsx:343-352 for the key-only save), each firing a POST that
	// only carries the field(s) it owns. Wholesale-replacing "ai" on every
	// such POST silently reset every field the caller didn't happen to
	// include — e.g. saving the Groq key alone reset provider back to
	// "anthropic", breaking the documented Groq setup flow (QA Scénario 1
	// étape 2). Fixed in http.go: "ai" now gets a field-by-field merge
	// (absent JSON key preserves the stored value, same semantics the two
	// secrets already had) instead of being wholesale-replaced. Both
	// scenarios below now assert the corrected, contract-amended behavior.
	t.Run("ai section ABSENT: batching and provider fields untouched", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.AnthropicAPIKey = "sk-ant-original"
		cfg.AI.BatchSize = 42
		cfg.AI.Provider = "groq"
		cfg.AI.InterBatchDelayMs = 12345
		cfg.AI.ContextTokenBudget = 999
		cfg.AI.MaxConsecutiveFailures = 7
		cfg.AI.GroqModel = "openai/gpt-oss-20b"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"server":{"debug":true}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}

		after := config.Get()
		if after.AI.AnthropicAPIKey != "sk-ant-original" {
			t.Errorf("Expected key untouched, got %q", after.AI.AnthropicAPIKey)
		}
		if after.AI.BatchSize != 42 {
			t.Errorf("Expected BatchSize=42 untouched, got %d", after.AI.BatchSize)
		}
		if after.AI.Provider != "groq" {
			t.Errorf("Expected Provider=groq untouched, got %q", after.AI.Provider)
		}
		if after.AI.InterBatchDelayMs != 12345 {
			t.Errorf("Expected InterBatchDelayMs=12345 untouched, got %d", after.AI.InterBatchDelayMs)
		}
		if after.AI.ContextTokenBudget != 999 {
			t.Errorf("Expected ContextTokenBudget=999 untouched, got %d", after.AI.ContextTokenBudget)
		}
		if after.AI.MaxConsecutiveFailures != 7 {
			t.Errorf("Expected MaxConsecutiveFailures=7 untouched, got %d", after.AI.MaxConsecutiveFailures)
		}
		if after.AI.GroqModel != "openai/gpt-oss-20b" {
			t.Errorf("Expected GroqModel untouched, got %q", after.AI.GroqModel)
		}
	})

	t.Run("batching and provider fields survive a key-only POST", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.AnthropicAPIKey = "sk-ant-original"
		cfg.AI.BatchSize = 42
		cfg.AI.Provider = "groq"
		cfg.AI.InterBatchDelayMs = 12345
		cfg.AI.ContextTokenBudget = 999
		cfg.AI.MaxConsecutiveFailures = 7
		cfg.AI.GroqModel = "openai/gpt-oss-20b"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"ai":{"anthropic_api_key":"sk-ant-new"}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}

		after := config.Get()
		if after.AI.AnthropicAPIKey != "sk-ant-new" {
			t.Errorf("Expected key replaced, got %q", after.AI.AnthropicAPIKey)
		}
		// http.go: "ai" is field-by-field merged, not wholesale-replaced —
		// a JSON key absent from the POST body preserves the previously
		// stored value for that field, exactly like the two secrets already
		// did. Reproduces the exact repro steps from
		// _work/handoff/task-dev-backend-20260806-103004.md (save provider,
		// then save the key alone — provider must not revert).
		if after.AI.BatchSize != 42 {
			t.Errorf("Expected BatchSize=42 preserved, got %d", after.AI.BatchSize)
		}
		if after.AI.Provider != "groq" {
			t.Errorf("Expected Provider=groq preserved, got %q", after.AI.Provider)
		}
		if after.AI.InterBatchDelayMs != 12345 {
			t.Errorf("Expected InterBatchDelayMs=12345 preserved, got %d", after.AI.InterBatchDelayMs)
		}
		if after.AI.ContextTokenBudget != 999 {
			t.Errorf("Expected ContextTokenBudget=999 preserved, got %d", after.AI.ContextTokenBudget)
		}
		if after.AI.MaxConsecutiveFailures != 7 {
			t.Errorf("Expected MaxConsecutiveFailures=7 preserved, got %d", after.AI.MaxConsecutiveFailures)
		}
		if after.AI.GroqModel != "openai/gpt-oss-20b" {
			t.Errorf("Expected GroqModel preserved, got %q", after.AI.GroqModel)
		}
	})

	// Explicit zero-value still applies: batch_size: 0 sent EXPLICITLY (key
	// present) is a real value, not "field omitted" — it goes through
	// ApplyDefaults afterward same as any other zero value would (existing,
	// documented behavior), landing on the default rather than staying 0.
	// This is what distinguishes the field merge from a naive "if incoming
	// field is non-zero, overwrite" approach, which would be unable to tell
	// "the user explicitly sent 0" apart from "the user didn't send this
	// field at all" and would silently coalesce the two.
	t.Run("ai section PRESENT with explicit zero: field is set then defaulted, not preserved", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.AnthropicAPIKey = "sk-ant-original"
		cfg.AI.BatchSize = 42
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"ai":{"anthropic_api_key":"sk-ant-new","batch_size":0}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}

		after := config.Get()
		if after.AI.BatchSize != 20 {
			t.Errorf("Expected explicit batch_size:0 to be re-defaulted to 20 (existing ApplyDefaults behavior), got %d", after.AI.BatchSize)
		}
	})

	// Mi-2 (code-review-20260806-101822.md): the Groq key's POST semantics
	// (preserve on absent/empty, clear_groq_api_key, gsk_ prefix validation)
	// duplicated from the Anthropic sub-tests above — previously only
	// exercised in-memory (ai_batching_test.go sets cfg.GroqAPIKey directly),
	// never through this HTTP-level merge path.
	t.Run("groq key: absent ai section preserves key", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.GroqAPIKey = "gsk_original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"server":{"debug":true}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if got := config.Get().AI.GroqAPIKey; got != "gsk_original" {
			t.Errorf("Expected Groq key preserved, got %q", got)
		}
	})

	t.Run("groq key: empty key in ai section preserves key", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.GroqAPIKey = "gsk_original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"ai":{"groq_api_key":"","model":"claude-opus-5"}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if got := config.Get().AI.GroqAPIKey; got != "gsk_original" {
			t.Errorf("Expected Groq key preserved on empty string, got %q", got)
		}
	})

	t.Run("groq key: clear_groq_api_key erases the key", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.GroqAPIKey = "gsk_original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"ai":{"clear_groq_api_key":true}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if got := config.Get().AI.GroqAPIKey; got != "" {
			t.Errorf("Expected Groq key cleared, got %q", got)
		}
	})

	t.Run("groq key: new key replaces the old one", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.GroqAPIKey = "gsk_original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"ai":{"groq_api_key":"gsk_new-value"}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if got := config.Get().AI.GroqAPIKey; got != "gsk_new-value" {
			t.Errorf("Expected Groq key replaced, got %q", got)
		}
	})

	t.Run("groq key: malformed key is rejected with 400 and store is untouched", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.GroqAPIKey = "gsk_original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"ai":{"groq_api_key":"not-a-valid-key"}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("Expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if got := config.Get().AI.GroqAPIKey; got != "gsk_original" {
			t.Errorf("Expected Groq key untouched after rejected request, got %q", got)
		}
	})

	t.Run("groq key: response never echoes the stored key", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		cfg := config.Get()
		cfg.AI.GroqAPIKey = "gsk_original"
		config.SetInstance(cfg)

		req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"ai":{"model":"claude-opus-5"}}`))
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		if strings.Contains(w.Body.String(), "gsk_original") {
			t.Errorf("POST response leaked the Groq API key: %s", w.Body.String())
		}
	})
}

func TestHTTPServer_CORS(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("OPTIONS", "/version", nil)
	w := httptest.NewRecorder()

	handler := server.corsMiddleware(server.mux)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for OPTIONS, got %d", w.Code)
	}

	allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "*" {
		t.Errorf("Expected CORS origin *, got %s", allowOrigin)
	}

	allowMethods := w.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(allowMethods, "GET") {
		t.Errorf("Expected GET in allowed methods, got %s", allowMethods)
	}
}

func TestHTTPServer_StaticFiles(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// Create static file
	htmlDir := filepath.Join(dataDir, "html")
	os.MkdirAll(htmlDir, 0755)
	os.WriteFile(filepath.Join(htmlDir, "test.html"), []byte("<h1>Test</h1>"), 0644)

	req := httptest.NewRequest("GET", "/html/test.html", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "<h1>Test</h1>" {
		t.Errorf("Unexpected content: %s", body)
	}
}

func TestHTTPServer_ListFiles(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// Create some files
	filesDir := filepath.Join(dataDir, "files")
	os.MkdirAll(filesDir, 0755)
	os.WriteFile(filepath.Join(filesDir, "test.txt"), []byte("test"), 0644)

	req := httptest.NewRequest("GET", "/listFiles", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<pre>") {
		t.Errorf("Expected HTML pre tags in response")
	}
}

func TestHTTPServer_DeleteFile(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// Create a file to delete
	filesDir := filepath.Join(dataDir, "files")
	os.MkdirAll(filesDir, 0755)
	testFile := filepath.Join(filesDir, "to-delete.txt")
	os.WriteFile(testFile, []byte("delete me"), 0644)

	req := httptest.NewRequest("DELETE", "/files/to-delete.txt", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Verify file was deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("File should have been deleted")
	}
}

func TestHTTPServer_ClearGame(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// Create files directory with content
	filesDir := filepath.Join(dataDir, "files")
	os.MkdirAll(filesDir, 0755)
	os.WriteFile(filepath.Join(filesDir, "test.txt"), []byte("test"), 0644)

	var actionReceived string
	server.OnAction = func(action string, data json.RawMessage) {
		actionReceived = action
	}

	req := httptest.NewRequest("GET", "/clearGame", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("Expected redirect (302), got %d", w.Code)
	}

	if actionReceived != "CLEAR_GAME" {
		t.Errorf("Expected CLEAR_GAME action, got %s", actionReceived)
	}

	// Files should be cleared but directory should exist
	if _, err := os.Stat(filesDir); os.IsNotExist(err) {
		t.Error("Files directory should exist after clear")
	}
}

func TestHTTPServer_ClearBuzzers(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Add some bumpers
	server.engine.UpdateBumper("b1", map[string]interface{}{"TEAM": "red"})
	server.engine.UpdateBumper("b2", map[string]interface{}{"TEAM": "blue"})

	var actionReceived string
	server.OnAction = func(action string, data json.RawMessage) {
		actionReceived = action
	}

	req := httptest.NewRequest("GET", "/clearBuzzers", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("Expected redirect (302), got %d", w.Code)
	}

	if actionReceived != "CLEAR_BUZZERS" {
		t.Errorf("Expected CLEAR_BUZZERS action, got %s", actionReceived)
	}

	// Bumpers should be cleared
	if server.engine.GetBumper("b1") != nil {
		t.Error("Bumpers should be cleared")
	}
}

func TestHTTPServer_Backup(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("GET", "/backup", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	// /backup redirects to /fs-backup
	if w.Code != http.StatusFound {
		t.Errorf("Expected 302 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/fs-backup" {
		t.Errorf("Expected redirect to /fs-backup, got %s", loc)
	}
}

func TestHTTPServer_Restore(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("POST", "/restore", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	// /restore is implemented: returns 400 when no multipart body is provided
	if w.Code == http.StatusNotFound {
		t.Errorf("Expected /restore to be implemented, got 404")
	}
}

func TestHTTPServer_QuestionUpload(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// Create multipart form
	body := strings.NewReader("--boundary\r\n" +
		"Content-Disposition: form-data; name=\"question\"\r\n\r\n" +
		"What is 2+2?\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"answer\"\r\n\r\n" +
		"4\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"points\"\r\n\r\n" +
		"10\r\n" +
		"--boundary--\r\n")

	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		bodyBytes, _ := io.ReadAll(w.Body)
		t.Errorf("Expected 200, got %d: %s", w.Code, string(bodyBytes))
	}

	// Verify question was created
	questionsDir := filepath.Join(dataDir, "files", "questions")
	entries, _ := os.ReadDir(questionsDir)
	if len(entries) == 0 {
		t.Error("Expected question directory to be created")
	}
}

func TestHTTPServer_FindFreeQuestionID(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	questionsDir := filepath.Join(dataDir, "files", "questions")
	os.MkdirAll(questionsDir, 0755)

	// First ID should be 1 — resolveQuestionDir also reserves it (creates the
	// directory), unlike the old findFreeQuestionID which only inspected the
	// filesystem, so each call below advances past the one it just reserved.
	id, dir, err := server.resolveQuestionDir("")
	if err != nil {
		t.Fatalf("resolveQuestionDir failed: %v", err)
	}
	if id != "1" {
		t.Errorf("Expected first ID to be 1, got %s", id)
	}
	if dir != filepath.Join(questionsDir, "1") {
		t.Errorf("Expected dir %s, got %s", filepath.Join(questionsDir, "1"), dir)
	}
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		t.Errorf("Expected reserved directory to exist: %v", statErr)
	}

	// Next should be 2 (1 is now reserved)
	id, _, err = server.resolveQuestionDir("")
	if err != nil {
		t.Fatalf("resolveQuestionDir failed: %v", err)
	}
	if id != "2" {
		t.Errorf("Expected next ID to be 2, got %s", id)
	}

	// Create 3, 4 manually (simulating IDs taken by another path)
	os.MkdirAll(filepath.Join(questionsDir, "3"), 0755)
	os.MkdirAll(filepath.Join(questionsDir, "4"), 0755)

	// Next should be 5 (1, 2 reserved above; 3, 4 created manually)
	id, _, err = server.resolveQuestionDir("")
	if err != nil {
		t.Fatalf("resolveQuestionDir failed: %v", err)
	}
	if id != "5" {
		t.Errorf("Expected next ID to be 5, got %s", id)
	}
}

// TestHTTPServer_ResolveQuestionDir_ExplicitIDReusesExistingDir verifies that
// an explicit ID (editing an existing question) uses MkdirAll — not the
// exclusive os.Mkdir reserved for auto-allocation — so re-saving an existing
// question never fails just because its directory already exists.
func TestHTTPServer_ResolveQuestionDir_ExplicitIDReusesExistingDir(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	questionsDir := filepath.Join(dataDir, "files", "questions")
	os.MkdirAll(filepath.Join(questionsDir, "7"), 0755)

	id, dir, err := server.resolveQuestionDir("7")
	if err != nil {
		t.Fatalf("resolveQuestionDir failed on existing explicit ID: %v", err)
	}
	if id != "7" || dir != filepath.Join(questionsDir, "7") {
		t.Errorf("Expected id=7 dir=%s, got id=%s dir=%s", filepath.Join(questionsDir, "7"), id, dir)
	}
}

// TestHTTPServer_ResolveQuestionDir_Exhausted verifies the id_exhausted error
// path (contract ai-generation.md §5.1, CA: never silently reuse/overwrite
// question 999) replaces the old "return 999" fallback.
func TestHTTPServer_ResolveQuestionDir_Exhausted(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	questionsDir := filepath.Join(dataDir, "files", "questions")
	for i := 1; i < 1000; i++ {
		os.MkdirAll(filepath.Join(questionsDir, strconv.Itoa(i)), 0755)
	}

	_, _, err := server.resolveQuestionDir("")
	if !errors.Is(err, ErrQuestionIDExhausted) {
		t.Fatalf("Expected ErrQuestionIDExhausted, got %v", err)
	}

	// The pre-existing question 999 must be untouched (no silent overwrite).
	info, statErr := os.Stat(filepath.Join(questionsDir, "999"))
	if statErr != nil || !info.IsDir() {
		t.Errorf("Expected question 999's directory to still exist untouched: %v", statErr)
	}
}

// TestConcurrentQuestionResolveDir_UniqueIDs races many goroutines through
// resolveQuestionDir("") and verifies each gets a distinct ID — the
// regression test for the pre-fix race (contract ai-generation.md §5.1, R2).
// Run with `go test -race` (qa gate) to also catch any data race on the
// mutex itself.
func TestConcurrentQuestionResolveDir_UniqueIDs(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "questions"), 0755)

	const n = 50
	ids := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id, _, err := server.resolveQuestionDir("")
			ids[idx] = id
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool, n)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: resolveQuestionDir failed: %v", i, err)
		}
		if seen[ids[i]] {
			t.Fatalf("duplicate ID allocated: %s", ids[i])
		}
		seen[ids[i]] = true
	}
	if len(seen) != n {
		t.Errorf("Expected %d unique IDs, got %d", n, len(seen))
	}
}

// ========================================
// Memory Game HTTP Tests - Phase 1
// ========================================

func TestHTTPServer_MemoryQuestionUpload(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// Ensure the questions directory exists
	questionsDir := filepath.Join(dataDir, "files", "questions")
	os.MkdirAll(questionsDir, 0755)

	// Create multipart form with Memory question data
	body := strings.NewReader("--boundary\r\n" +
		"Content-Disposition: form-data; name=\"question\"\r\n\r\n" +
		"Match capitals with countries\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"answer\"\r\n\r\n" +
		"2 paires\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"type\"\r\n\r\n" +
		"MEMORY\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"points\"\r\n\r\n" +
		"20\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"time\"\r\n\r\n" +
		"60\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"memory_pairs\"\r\n\r\n" +
		`[{"ID":1,"CARD1":{"TEXT":"Paris","IS_IMAGE":false},"CARD2":{"TEXT":"France","IS_IMAGE":false}},{"ID":2,"CARD1":{"TEXT":"Berlin","IS_IMAGE":false},"CARD2":{"TEXT":"Germany","IS_IMAGE":false}}]` + "\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"memory_config\"\r\n\r\n" +
		`{"FLIP_DELAY":3000,"POINTS_PER_PAIR":10,"ERROR_PENALTY":0,"COMPLETION_BONUS":0,"USE_TIMER":true}` + "\r\n" +
		"--boundary--\r\n")

	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		bodyBytes, _ := io.ReadAll(w.Body)
		t.Errorf("Expected 200, got %d: %s", w.Code, string(bodyBytes))
	}

	// Verify question was created with Memory data (use same questionsDir from above)
	entries, _ := os.ReadDir(questionsDir)
	if len(entries) == 0 {
		t.Fatal("Expected question directory to be created")
	}

	// Read the created question.json
	questionFile := filepath.Join(questionsDir, entries[0].Name(), "question.json")
	data, err := os.ReadFile(questionFile)
	if err != nil {
		t.Fatalf("Failed to read question.json: %v", err)
	}

	var question map[string]interface{}
	if err := json.Unmarshal(data, &question); err != nil {
		t.Fatalf("Failed to parse question.json: %v", err)
	}

	// Verify TYPE is MEMORY
	if question["TYPE"] != "MEMORY" {
		t.Errorf("Expected TYPE 'MEMORY', got '%v'", question["TYPE"])
	}

	// Verify MEMORY_PAIRS exists and has 2 pairs
	pairs, ok := question["MEMORY_PAIRS"].([]interface{})
	if !ok {
		t.Fatal("MEMORY_PAIRS should be an array")
	}
	if len(pairs) != 2 {
		t.Errorf("Expected 2 pairs, got %d", len(pairs))
	}

	// Verify MEMORY_CONFIG exists
	config, ok := question["MEMORY_CONFIG"].(map[string]interface{})
	if !ok {
		t.Fatal("MEMORY_CONFIG should be an object")
	}
	if config["FLIP_DELAY"] != float64(3000) {
		t.Errorf("Expected FLIP_DELAY 3000, got %v", config["FLIP_DELAY"])
	}
}

func TestHTTPServer_MemoryQuestionLoad(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// Create a Memory question manually
	questionsDir := filepath.Join(dataDir, "files", "questions", "1")
	os.MkdirAll(questionsDir, 0755)

	questionData := map[string]interface{}{
		"ID":       "1",
		"QUESTION": "Match capitals",
		"ANSWER":   "2 paires",
		"TYPE":     "MEMORY",
		"POINTS":   "20",
		"TIME":     "60",
		"MEMORY_PAIRS": []map[string]interface{}{
			{
				"ID": 1,
				"CARD1": map[string]interface{}{"TEXT": "Paris", "IS_IMAGE": false},
				"CARD2": map[string]interface{}{"TEXT": "France", "IS_IMAGE": false},
			},
			{
				"ID": 2,
				"CARD1": map[string]interface{}{"TEXT": "Berlin", "IS_IMAGE": false},
				"CARD2": map[string]interface{}{"TEXT": "Germany", "IS_IMAGE": false},
			},
		},
		"MEMORY_CONFIG": map[string]interface{}{
			"FLIP_DELAY":       3000,
			"POINTS_PER_PAIR":  10,
			"ERROR_PENALTY":    0,
			"COMPLETION_BONUS": 0,
			"USE_TIMER":        true,
		},
	}
	data, _ := json.Marshal(questionData)
	os.WriteFile(filepath.Join(questionsDir, "question.json"), data, 0644)

	// Request the questions list
	req := httptest.NewRequest("GET", "/questions", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Find the question (keys are like "/files/questions/1")
	var q map[string]interface{}
	for key, val := range response {
		if key != "FSINFO" {
			q = val.(map[string]interface{})
			break
		}
	}
	if q == nil {
		t.Fatal("No question found in response")
	}

	// Verify TYPE
	if q["TYPE"] != "MEMORY" {
		t.Errorf("Expected TYPE 'MEMORY', got '%v'", q["TYPE"])
	}

	// Verify MEMORY_PAIRS
	pairs, ok := q["MEMORY_PAIRS"].([]interface{})
	if !ok {
		t.Fatal("MEMORY_PAIRS should be an array")
	}
	if len(pairs) != 2 {
		t.Errorf("Expected 2 pairs, got %d", len(pairs))
	}

	// Verify first pair
	pair1 := pairs[0].(map[string]interface{})
	card1 := pair1["CARD1"].(map[string]interface{})
	if card1["TEXT"] != "Paris" {
		t.Errorf("Expected Card1 text 'Paris', got '%v'", card1["TEXT"])
	}
}

// ========================================
// WiFi Config API Regression Tests
// ========================================

func TestHTTPServer_APIBuzzers_Empty(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("GET", "/api/buzzers", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var buzzers []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &buzzers); err != nil {
		t.Fatalf("Response is not valid JSON array: %v", err)
	}

	if len(buzzers) != 0 {
		t.Errorf("Expected empty buzzers list, got %d", len(buzzers))
	}
}

func TestHTTPServer_APIBuzzers_WithData(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Register buzzers with different protocols
	server.engine.UpdateBumper("AA:BB:CC:DD:EE:01", map[string]interface{}{
		"NAME":     "Buzzer1",
		"TEAM":     "red",
		"IP":       "192.168.1.10",
		"VERSION":  "3.0.5",
		"STATUS":   "PONG",
		"PROTOCOL": "WebSocket",
	})
	server.engine.UpdateBumper("AA:BB:CC:DD:EE:02", map[string]interface{}{
		"NAME":   "Buzzer2",
		"TEAM":   "blue",
		"IP":     "192.168.1.11",
		"STATUS": "PONG",
	})

	req := httptest.NewRequest("GET", "/api/buzzers", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var buzzers []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &buzzers); err != nil {
		t.Fatalf("Response is not valid JSON array: %v", err)
	}

	if len(buzzers) != 2 {
		t.Fatalf("Expected 2 buzzers, got %d", len(buzzers))
	}

	// Build a lookup by MAC for order-independent assertions
	buzzerByMAC := make(map[string]map[string]interface{})
	for _, b := range buzzers {
		mac, _ := b["mac"].(string)
		buzzerByMAC[mac] = b
	}

	// Verify first buzzer (WebSocket protocol)
	b1, ok := buzzerByMAC["AA:BB:CC:DD:EE:01"]
	if !ok {
		t.Fatal("Buzzer AA:BB:CC:DD:EE:01 not found in response")
	}
	if b1["name"] != "Buzzer1" {
		t.Errorf("Expected name 'Buzzer1', got '%v'", b1["name"])
	}
	if b1["team"] != "red" {
		t.Errorf("Expected team 'red', got '%v'", b1["team"])
	}
	if b1["protocol"] != "WebSocket" {
		t.Errorf("Expected protocol 'WebSocket', got '%v'", b1["protocol"])
	}
	if b1["ip"] != "192.168.1.10" {
		t.Errorf("Expected ip '192.168.1.10', got '%v'", b1["ip"])
	}
	if b1["version"] != "3.0.5" {
		t.Errorf("Expected version '3.0.5', got '%v'", b1["version"])
	}

	// Verify second buzzer (TCP protocol - default)
	b2, ok := buzzerByMAC["AA:BB:CC:DD:EE:02"]
	if !ok {
		t.Fatal("Buzzer AA:BB:CC:DD:EE:02 not found in response")
	}
	if b2["protocol"] != "TCP" {
		t.Errorf("Expected protocol 'TCP' (default), got '%v'", b2["protocol"])
	}
}

func TestHTTPServer_APIBuzzers_MethodNotAllowed(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	methods := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/api/buzzers", nil)
		w := httptest.NewRecorder()

		server.mux.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api/buzzers: Expected status 405, got %d", method, w.Code)
		}
	}
}

func TestHTTPServer_APIBuzzerStatus_Nominal(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Register a buzzer
	mac := "AA:BB:CC:DD:EE:FF"
	server.engine.UpdateBumper(mac, map[string]interface{}{
		"NAME":     "TestBuzzer",
		"TEAM":     "green",
		"IP":       "192.168.1.42",
		"VERSION":  "3.0.5",
		"STATUS":   "PONG",
		"PROTOCOL": "WebSocket",
	})

	req := httptest.NewRequest("GET", "/api/buzzer/"+mac+"/status", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}

	// Verify all expected fields
	// Note: UpdateBumper does not set STATUS field, so it stays empty
	tests := []struct {
		field string
		want  interface{}
	}{
		{"mac", mac},
		{"name", "TestBuzzer"},
		{"team", "green"},
		{"protocol", "WebSocket"},
		{"ip", "192.168.1.42"},
		{"version", "3.0.5"},
	}

	for _, tt := range tests {
		if result[tt.field] != tt.want {
			t.Errorf("Field %s: expected '%v', got '%v'", tt.field, tt.want, result[tt.field])
		}
	}

	// Verify score field exists (numeric)
	if _, ok := result["score"]; !ok {
		t.Error("Expected 'score' field in response")
	}
}

func TestHTTPServer_APIBuzzerStatus_NotFound(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("GET", "/api/buzzer/FF:FF:FF:FF:FF:FF/status", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHTTPServer_APIBuzzerStatus_EmptyMAC(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Path /api/buzzer//status gets normalized by Go's HTTP mux to /api/buzzer/status
	// which results in a redirect (301) or bad request. Either is acceptable.
	req := httptest.NewRequest("GET", "/api/buzzer//status", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	// Accept 301 (path cleanup redirect), 400 (bad request), or 404 (not found)
	validCodes := map[int]bool{
		http.StatusMovedPermanently: true,
		http.StatusBadRequest:      true,
		http.StatusNotFound:        true,
	}
	if !validCodes[w.Code] {
		t.Errorf("Expected status 301, 400, or 404 for empty MAC, got %d", w.Code)
	}
}

func TestHTTPServer_APIBuzzerStatus_MethodNotAllowed(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	methods := []string{"POST", "PUT", "DELETE"}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/api/buzzer/AA:BB:CC:DD:EE:FF/status", nil)
		w := httptest.NewRecorder()

		server.mux.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api/buzzer/.../status: Expected status 405, got %d", method, w.Code)
		}
	}
}

func TestHTTPServer_APIBuzzerStatus_DefaultProtocol(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Register a buzzer without explicit protocol (legacy TCP)
	mac := "AA:BB:CC:DD:EE:01"
	server.engine.UpdateBumper(mac, map[string]interface{}{
		"NAME":   "LegacyBuzzer",
		"TEAM":   "red",
		"STATUS": "PONG",
	})

	req := httptest.NewRequest("GET", "/api/buzzer/"+mac+"/status", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}

	// Protocol should default to "TCP" when not set
	if result["protocol"] != "TCP" {
		t.Errorf("Expected default protocol 'TCP', got '%v'", result["protocol"])
	}
}

func TestHTTPServer_APIBuzzerStatus_WithoutStatusSuffix(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Register a buzzer
	mac := "AA:BB:CC:DD:EE:FF"
	server.engine.UpdateBumper(mac, map[string]interface{}{
		"NAME": "TestBuzzer",
		"TEAM": "red",
	})

	// Test without /status suffix (handler should still work)
	req := httptest.NewRequest("GET", "/api/buzzer/"+mac, nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /api/buzzer/{mac}, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}

	if result["mac"] != mac {
		t.Errorf("Expected mac '%s', got '%v'", mac, result["mac"])
	}
}

// ========================================
// Issue #44 — Versioned download filenames
// ========================================

// TestHTTPServer_FSBackup_VersionedFilename verifies that the full-backup endpoint
// includes the server version in the Content-Disposition filename.
func TestHTTPServer_FSBackup_VersionedFilename(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// Create minimal data directory so TAR walk has something
	os.MkdirAll(filepath.Join(dataDir, "files"), 0755)

	req := httptest.NewRequest("GET", "/fs-backup", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	cd := w.Header().Get("Content-Disposition")
	// Config version is "2.0.0-test" (set in setupTestHTTPServer)
	if !strings.Contains(cd, "v2.0.0-test") {
		t.Errorf("Content-Disposition should contain version 'v2.0.0-test', got: %s", cd)
	}
	if !strings.Contains(cd, "buzzcontrol-full-backup") {
		t.Errorf("Content-Disposition should contain 'buzzcontrol-full-backup', got: %s", cd)
	}
	if !strings.HasSuffix(strings.Trim(cd, "\""), ".tar\"") && !strings.Contains(cd, ".tar") {
		t.Errorf("Content-Disposition should reference a .tar file, got: %s", cd)
	}
}

// TestHTTPServer_GameBackup_VersionedFilename verifies that the game-backup endpoint
// includes the server version in the Content-Disposition filename.
func TestHTTPServer_GameBackup_VersionedFilename(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	os.MkdirAll(filepath.Join(dataDir, "files"), 0755)

	req := httptest.NewRequest("GET", "/game-backup", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "v2.0.0-test") {
		t.Errorf("Content-Disposition should contain version 'v2.0.0-test', got: %s", cd)
	}
	if !strings.Contains(cd, "buzzcontrol-game-backup") {
		t.Errorf("Content-Disposition should contain 'buzzcontrol-game-backup', got: %s", cd)
	}
}

// TestHTTPServer_BackupSelect_VersionedFilename verifies that the selective-backup endpoint
// includes the server version in the Content-Disposition filename.
func TestHTTPServer_BackupSelect_VersionedFilename(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	os.MkdirAll(filepath.Join(dataDir, "files"), 0755)

	req := httptest.NewRequest("GET", "/backup-select?questions=true", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "v2.0.0-test") {
		t.Errorf("Content-Disposition should contain version 'v2.0.0-test', got: %s", cd)
	}
	if !strings.Contains(cd, "buzzcontrol-backup") {
		t.Errorf("Content-Disposition should contain 'buzzcontrol-backup', got: %s", cd)
	}
}

// ========================================
// #95 — GET /api/categories tests
// ========================================

type categoryItem struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	ImageURL string `json:"imageURL"`
	IsCustom bool   `json:"isCustom"`
}

// TestGetCategoriesEmptyDir verifies that an empty categories directory returns
// only the hardcoded categories (none marked isCustom).
func TestGetCategoriesEmptyDir(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	categoriesDir := filepath.Join(dataDir, "files", "categories")
	os.MkdirAll(categoriesDir, 0755)

	req := httptest.NewRequest("GET", "/api/categories", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var items []categoryItem
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(items) == 0 {
		t.Error("Expected at least hardcoded categories, got empty list")
	}
	for _, item := range items {
		if item.IsCustom {
			t.Errorf("Expected only hardcoded categories (isCustom=false) in empty dir, got isCustom=true for key=%s", item.Key)
		}
	}
}

// TestGetCategoriesWithPNG verifies that a PNG file in the categories directory
// produces the correct custom category entry in the response.
func TestGetCategoriesWithPNG(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	categoriesDir := filepath.Join(dataDir, "files", "categories")
	os.MkdirAll(categoriesDir, 0755)
	os.WriteFile(filepath.Join(categoriesDir, "Sport Extreme.png"), []byte("fake"), 0644)

	req := httptest.NewRequest("GET", "/api/categories", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var items []categoryItem
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	var found *categoryItem
	for i := range items {
		if items[i].Key == "SPORT_EXTREME" {
			found = &items[i]
			break
		}
	}
	if found == nil {
		t.Fatal("Expected SPORT_EXTREME category in response, not found")
	}
	if found.Name != "Sport Extreme" {
		t.Errorf("Expected Name='Sport Extreme', got '%s'", found.Name)
	}
	// imageURL may be percent-encoded or literal — both are valid for web serving
	if found.ImageURL != "/files/categories/Sport%20Extreme.png" && found.ImageURL != "/files/categories/Sport Extreme.png" {
		t.Errorf("Expected ImageURL for Sport Extreme.png, got '%s'", found.ImageURL)
	}
	if !found.IsCustom {
		t.Error("Expected IsCustom=true for SPORT_EXTREME")
	}
}

// TestGetCategoriesFormatsSupported verifies that only .png/.jpg/.jpeg/.webp
// files are included, while .gif/.txt/.svg are silently ignored.
func TestGetCategoriesFormatsSupported(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	categoriesDir := filepath.Join(dataDir, "files", "categories")
	os.MkdirAll(categoriesDir, 0755)

	for _, name := range []string{"Alpha.png", "Beta.jpg", "Gamma.jpeg", "Delta.webp"} {
		os.WriteFile(filepath.Join(categoriesDir, name), []byte("x"), 0644)
	}
	for _, name := range []string{"Ignored.gif", "Ignored.txt", "Ignored.svg"} {
		os.WriteFile(filepath.Join(categoriesDir, name), []byte("x"), 0644)
	}

	req := httptest.NewRequest("GET", "/api/categories", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var items []categoryItem
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	supported := map[string]bool{"ALPHA": false, "BETA": false, "GAMMA": false, "DELTA": false}
	for _, item := range items {
		if _, ok := supported[item.Key]; ok {
			supported[item.Key] = true
		}
		if item.Key == "IGNORED" {
			t.Error("IGNORED (gif/txt/svg) should not appear in categories response")
		}
	}
	for k, found := range supported {
		if !found {
			t.Errorf("Expected category %s (supported format) in response", k)
		}
	}
}

// TestGetCategoriesKeyConflict verifies that a custom file whose derived key
// matches a hardcoded category does not override the hardcoded entry.
func TestGetCategoriesKeyConflict(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	categoriesDir := filepath.Join(dataDir, "files", "categories")
	os.MkdirAll(categoriesDir, 0755)
	// GEOGRAPHY is a hardcoded key — custom file with same name must lose
	os.WriteFile(filepath.Join(categoriesDir, "GEOGRAPHY.png"), []byte("x"), 0644)

	req := httptest.NewRequest("GET", "/api/categories", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var items []categoryItem
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	count := 0
	for _, item := range items {
		if item.Key == "GEOGRAPHY" {
			count++
			if item.IsCustom {
				t.Error("GEOGRAPHY must remain hardcoded (isCustom=false) when a custom file conflicts")
			}
		}
	}
	if count != 1 {
		t.Errorf("Expected exactly one GEOGRAPHY entry, got %d", count)
	}
}

// ========================================
// #95 — Backup/Restore with categories
// ========================================

// TestBackupIncludesCategories verifies that /backup-select?backgrounds=true
// includes the files/categories/ directory in the TAR archive.
func TestBackupIncludesCategories(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	categoriesDir := filepath.Join(dataDir, "files", "categories")
	os.MkdirAll(categoriesDir, 0755)
	os.WriteFile(filepath.Join(categoriesDir, "test.png"), []byte("fake-image"), 0644)

	req := httptest.NewRequest("GET", "/backup-select?backgrounds=true", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	tr := tar.NewReader(bytes.NewReader(w.Body.Bytes()))
	found := false
	for {
		header, err := tr.Next()
		if err != nil {
			break
		}
		if strings.Contains(filepath.ToSlash(header.Name), "files/categories/test.png") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected files/categories/test.png in backup TAR (backgrounds=true), not found")
	}
}

// TestRestoreCategories verifies that a TAR containing files/categories/ is
// correctly extracted by /restore.
func TestRestoreCategories(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	content := []byte("fake-image")
	_ = tw.WriteHeader(&tar.Header{
		Name: "files/categories/custom.png",
		Mode: 0644,
		Size: int64(len(content)),
	})
	tw.Write(content)
	tw.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "backup.tar")
	fw.Write(tarBuf.Bytes())
	mw.Close()

	req := httptest.NewRequest("POST", "/restore", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	restoredPath := filepath.Join(dataDir, "files", "categories", "custom.png")
	if _, err := os.Stat(restoredPath); os.IsNotExist(err) {
		t.Error("Expected files/categories/custom.png to be restored, but file not found")
	}
}

// TestRestoreWithoutCategories verifies that a TAR without files/categories/
// is handled gracefully — no error, non-breaking.
func TestRestoreWithoutCategories(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	content := []byte(`{"question":"test"}`)
	_ = tw.WriteHeader(&tar.Header{
		Name: "files/questions/1/data.json",
		Mode: 0644,
		Size: int64(len(content)),
	})
	tw.Write(content)
	tw.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "backup.tar")
	fw.Write(tarBuf.Bytes())
	mw.Close()

	req := httptest.NewRequest("POST", "/restore", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 (no error when categories absent from TAR), got %d: %s", w.Code, w.Body.String())
	}
}

// ========================================
// v5.7.1 — #97 POST /api/categories
// ========================================

// minimalPNG is a valid 1x1 PNG used as test image content (v5.7.2 — #100)
var minimalPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
	0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
	0x00, 0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC,
	0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
	0x44, 0xAE, 0x42, 0x60, 0x82,
}

// newMultipartCategoryRequest builds a multipart/form-data POST request for /api/categories.
// If filename is empty, no "file" field is included (tests missing-file validation).
func newMultipartCategoryRequest(name, filename string, fileContent []byte) *http.Request {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", name)
	if filename != "" {
		part, _ := writer.CreateFormFile("file", filename)
		part.Write(fileContent)
	}
	writer.Close()
	req := httptest.NewRequest("POST", "/api/categories", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// TestPostCategory_Valid verifies that a valid multipart POST /api/categories creates
// the image file on disk and returns the correct CategoryInfo JSON (v5.7.2 — #100).
func TestPostCategory_Valid(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	req := newMultipartCategoryRequest("Ma Categorie", "image.png", minimalPNG)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cat categoryItem
	if err := json.NewDecoder(w.Body).Decode(&cat); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if cat.Key != "MA_CATEGORIE" {
		t.Errorf("Expected key 'MA_CATEGORIE', got '%s'", cat.Key)
	}
	if cat.Name != "Ma Categorie" {
		t.Errorf("Expected name 'Ma Categorie', got '%s'", cat.Name)
	}
	if !cat.IsCustom {
		t.Error("Expected IsCustom=true for custom category")
	}
	if cat.ImageURL == "" {
		t.Error("Expected non-empty imageURL for image category")
	}

	// Verify the image file was created on disk
	filePath := filepath.Join(dataDir, "files", "categories", "MA_CATEGORIE.png")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Expected MA_CATEGORIE.png to be created on disk, file not found")
	}
}

// TestPostCategory_EmptyName verifies that POST /api/categories with an empty name returns 400.
func TestPostCategory_EmptyName(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	tests := []struct {
		label    string
		name     string
		filename string
	}{
		{"empty name field", "", "image.png"},
		{"missing name field (empty string)", "", "image.png"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			req := newMultipartCategoryRequest(tt.name, tt.filename, minimalPNG)
			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s: Expected 400, got %d", tt.label, w.Code)
			}
		})
	}
}

// TestPostCategory_MissingFile verifies that POST /api/categories without a file returns 400.
func TestPostCategory_MissingFile(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	// No filename → no "file" field in form
	req := newMultipartCategoryRequest("Ma Categorie", "", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 when file field is missing, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPostCategory_InvalidFileType verifies that POST /api/categories with a non-image file returns 400.
func TestPostCategory_InvalidFileType(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	req := newMultipartCategoryRequest("Ma Categorie", "document.pdf", []byte("%PDF-"))
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid file type, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPostCategory_NameTooLong verifies that POST /api/categories with a name > 50 chars returns 400.
func TestPostCategory_NameTooLong(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	longName := strings.Repeat("A", 51) // 51 chars — above the 50-char limit
	req := newMultipartCategoryRequest(longName, "image.png", minimalPNG)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for name too long, got %d", w.Code)
	}
}

// TestPostCategory_NameExactlyFiftyChars verifies that a 50-char name is accepted (boundary).
func TestPostCategory_NameExactlyFiftyChars(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	exactName := strings.Repeat("A", 50) // exactly 50 chars — at the limit
	req := newMultipartCategoryRequest(exactName, "image.png", minimalPNG)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for name exactly 50 chars, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPostCategory_ConflictHardcoded verifies that POST /api/categories returns 409
// when the derived key matches a hardcoded category.
func TestPostCategory_ConflictHardcoded(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	// "Science" → "SCIENCE" which is a hardcoded category key.
	req := newMultipartCategoryRequest("Science", "image.png", minimalPNG)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected 409 conflict for hardcoded category key, got %d", w.Code)
	}
}

// TestPostCategory_ConflictExistingCustom verifies that POST /api/categories returns 409
// when a custom category with the same derived key already exists.
func TestPostCategory_ConflictExistingCustom(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	// First POST — should succeed
	req1 := newMultipartCategoryRequest("Mon Quiz", "image.png", minimalPNG)
	w1 := httptest.NewRecorder()
	server.mux.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("First POST should succeed, got %d: %s", w1.Code, w1.Body.String())
	}

	// Second POST with same derived key "MON_QUIZ" — should return 409
	req2 := newMultipartCategoryRequest("Mon Quiz", "photo.jpg", minimalPNG)
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("Expected 409 conflict for duplicate custom category, got %d", w2.Code)
	}
}

// TestPostCategory_AppearsInGetCategories verifies that a newly created custom category
// is returned by the subsequent GET /api/categories request with a non-empty imageURL.
func TestPostCategory_AppearsInGetCategories(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	// Create a custom category
	req := newMultipartCategoryRequest("Super Quiz", "image.png", minimalPNG)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST failed with %d: %s", w.Code, w.Body.String())
	}

	// Now GET the categories list
	req2 := httptest.NewRequest("GET", "/api/categories", nil)
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, req2)

	var items []categoryItem
	if err := json.NewDecoder(w2.Body).Decode(&items); err != nil {
		t.Fatalf("Failed to decode GET response: %v", err)
	}

	found := false
	for _, item := range items {
		if item.Key == "SUPER_QUIZ" {
			found = true
			if !item.IsCustom {
				t.Error("Expected IsCustom=true for SUPER_QUIZ")
			}
			// Name must be the original display name (v5.7.7 sidecar JSON)
			if item.Name != "Super Quiz" {
				t.Errorf("Expected Name=%q, got %q", "Super Quiz", item.Name)
			}
			if item.ImageURL == "" {
				t.Error("Expected non-empty imageURL for SUPER_QUIZ after multipart POST")
			}
		}
	}
	if !found {
		t.Error("Expected SUPER_QUIZ to appear in GET /api/categories after POST")
	}
}

// TestPostCategory_SidecarNamePersisted verifies that the original display name
// is persisted in a sidecar JSON and retrieved by GET /api/categories (v5.7.7).
func TestPostCategory_SidecarNamePersisted(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	catDir := filepath.Join(dataDir, "files", "categories")
	os.MkdirAll(catDir, 0755)

	// Create category with a display name different from the key
	req := newMultipartCategoryRequest("Mon Super Jeu", "image.webp", minimalPNG)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST failed with %d: %s", w.Code, w.Body.String())
	}

	// Verify sidecar JSON was created on disk
	sidecarPath := filepath.Join(catDir, "MON_SUPER_JEU.json")
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("Sidecar JSON not found at %s: %v", sidecarPath, err)
	}
	var meta struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("Failed to parse sidecar JSON: %v", err)
	}
	if meta.Name != "Mon Super Jeu" {
		t.Errorf("Sidecar name = %q, want %q", meta.Name, "Mon Super Jeu")
	}

	// GET /api/categories must return original name
	req2 := httptest.NewRequest("GET", "/api/categories", nil)
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, req2)

	var items []categoryItem
	if err := json.NewDecoder(w2.Body).Decode(&items); err != nil {
		t.Fatalf("Failed to decode GET response: %v", err)
	}

	found := false
	for _, item := range items {
		if item.Key == "MON_SUPER_JEU" {
			found = true
			if item.Name != "Mon Super Jeu" {
				t.Errorf("GET name = %q, want %q", item.Name, "Mon Super Jeu")
			}
		}
	}
	if !found {
		t.Error("MON_SUPER_JEU not found in GET /api/categories")
	}
}

// TestGetCategories_SidecarFallback verifies that when a custom category image exists
// without a sidecar JSON, GET /api/categories returns the raw file stem as the name (v5.7.7).
// This is the fallback path when sidecar is missing (e.g. categories created before v5.7.7).
func TestGetCategories_SidecarFallback(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	catDir := filepath.Join(dataDir, "files", "categories")
	os.MkdirAll(catDir, 0755)

	// Write only the image file — no sidecar JSON alongside it
	imgPath := filepath.Join(catDir, "MON_JEU.png")
	if err := os.WriteFile(imgPath, minimalPNG, 0644); err != nil {
		t.Fatalf("Failed to write test image: %v", err)
	}
	// Deliberately no MON_JEU.json sidecar

	req := httptest.NewRequest("GET", "/api/categories", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/categories returned %d: %s", w.Code, w.Body.String())
	}

	var items []categoryItem
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("Failed to decode GET response: %v", err)
	}

	found := false
	for _, item := range items {
		if item.Key == "MON_JEU" {
			found = true
			// Without sidecar, Name falls back to the raw file stem
			if item.Name != "MON_JEU" {
				t.Errorf("Without sidecar: Name = %q, want raw stem %q", item.Name, "MON_JEU")
			}
			if item.ImageURL == "" {
				t.Error("Expected non-empty imageURL for MON_JEU")
			}
			if !item.IsCustom {
				t.Error("Expected IsCustom=true for MON_JEU")
			}
		}
	}
	if !found {
		t.Error("MON_JEU not found in GET /api/categories without sidecar")
	}
}

// TestAPICategories_MethodNotAllowed verifies that PUT and DELETE on /api/categories return 405.
func TestAPICategories_MethodNotAllowed(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	for _, method := range []string{"PUT", "DELETE", "PATCH"} {
		req := httptest.NewRequest(method, "/api/categories", nil)
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api/categories: Expected 405, got %d", method, w.Code)
		}
	}
}

// ========================================
// v5.7.1 — #98 NORMAL → SPEEDY migration
// ========================================

// TestHTTPServer_Questions_NormalToSpeedy verifies that a question.json containing
// the legacy TYPE "NORMAL" is served with TYPE "SPEEDY" by GET /questions.
func TestHTTPServer_Questions_NormalToSpeedy(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// Create a question on disk with the legacy "NORMAL" type
	questionsDir := filepath.Join(dataDir, "files", "questions", "42")
	os.MkdirAll(questionsDir, 0755)

	questionData := map[string]interface{}{
		"ID":       "42",
		"QUESTION": "Legacy normal question",
		"ANSWER":   "Answer",
		"TYPE":     "NORMAL", // legacy value — should be migrated to SPEEDY on read
		"POINTS":   "10",
		"TIME":     "30",
	}
	data, _ := json.Marshal(questionData)
	os.WriteFile(filepath.Join(questionsDir, "question.json"), data, 0644)

	req := httptest.NewRequest("GET", "/questions", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	var q map[string]interface{}
	for key, val := range response {
		if key != "FSINFO" {
			q, _ = val.(map[string]interface{})
		}
	}
	if q == nil {
		t.Fatal("No question found in response")
	}

	// The key assertion: legacy "NORMAL" must be normalized to "SPEEDY" on read
	if q["TYPE"] != "SPEEDY" {
		t.Errorf("Expected TYPE 'SPEEDY' (migrated from NORMAL), got '%v'", q["TYPE"])
	}
}

// TestHTTPServer_QuestionUpload_DefaultIsSpeedy verifies that POSTing a question
// without a type field results in TYPE=SPEEDY in the saved file.
func TestHTTPServer_QuestionUpload_DefaultIsSpeedy(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	questionsDir := filepath.Join(dataDir, "files", "questions")
	os.MkdirAll(questionsDir, 0755)

	// POST without "type" field — default should be SPEEDY
	body := strings.NewReader("--boundary\r\n" +
		"Content-Disposition: form-data; name=\"question\"\r\n\r\n" +
		"What is the capital of France?\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"answer\"\r\n\r\n" +
		"Paris\r\n" +
		"--boundary--\r\n")

	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Read the saved question from disk
	entries, _ := os.ReadDir(questionsDir)
	if len(entries) == 0 {
		t.Fatal("Expected question directory to be created")
	}
	questionFile := filepath.Join(questionsDir, entries[0].Name(), "question.json")
	fileData, err := os.ReadFile(questionFile)
	if err != nil {
		t.Fatalf("Failed to read question.json: %v", err)
	}

	var saved map[string]interface{}
	if err := json.Unmarshal(fileData, &saved); err != nil {
		t.Fatalf("Failed to parse saved question: %v", err)
	}

	if saved["TYPE"] != "SPEEDY" {
		t.Errorf("Expected saved TYPE='SPEEDY' (default), got '%v'", saved["TYPE"])
	}
}

// TestHTTPServer_QuestionUpload_NormalMigratedToSpeedy verifies that POSTing a question
// with type=NORMAL (legacy client) results in TYPE=SPEEDY in the saved file.
func TestHTTPServer_QuestionUpload_NormalMigratedToSpeedy(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	questionsDir := filepath.Join(dataDir, "files", "questions")
	os.MkdirAll(questionsDir, 0755)

	// POST with explicit "NORMAL" type (simulates old client)
	body := strings.NewReader("--boundary\r\n" +
		"Content-Disposition: form-data; name=\"question\"\r\n\r\n" +
		"Old normal question\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"answer\"\r\n\r\n" +
		"Answer\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"type\"\r\n\r\n" +
		"NORMAL\r\n" +
		"--boundary--\r\n")

	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	entries, _ := os.ReadDir(questionsDir)
	if len(entries) == 0 {
		t.Fatal("Expected question directory to be created")
	}
	questionFile := filepath.Join(questionsDir, entries[0].Name(), "question.json")
	fileData, err := os.ReadFile(questionFile)
	if err != nil {
		t.Fatalf("Failed to read question.json: %v", err)
	}

	var saved map[string]interface{}
	if err := json.Unmarshal(fileData, &saved); err != nil {
		t.Fatalf("Failed to parse saved question: %v", err)
	}

	// Legacy "NORMAL" must be normalized to "SPEEDY" on write
	if saved["TYPE"] != "SPEEDY" {
		t.Errorf("Expected saved TYPE='SPEEDY' (migrated from NORMAL), got '%v'", saved["TYPE"])
	}
}

// ========================================
// v5.7.1 — #99 medias param (backup-select + reset-select)
// ========================================

// TestBackupSelectWithMediasParam verifies that GET /backup-select?medias=true
// includes both files/backgrounds/ and files/categories/ in the TAR archive.
func TestBackupSelectWithMediasParam(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// Create backgrounds and categories files
	bgDir := filepath.Join(dataDir, "files", "backgrounds")
	catDir := filepath.Join(dataDir, "files", "categories")
	os.MkdirAll(bgDir, 0755)
	os.MkdirAll(catDir, 0755)
	os.WriteFile(filepath.Join(bgDir, "bg1.jpg"), []byte("fake-bg"), 0644)
	os.WriteFile(filepath.Join(catDir, "custom.png"), []byte("fake-cat"), 0644)

	req := httptest.NewRequest("GET", "/backup-select?medias=true", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	// Verify Content-Disposition header
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "buzzcontrol-backup") {
		t.Errorf("Expected Content-Disposition with 'buzzcontrol-backup', got: %s", cd)
	}

	// Parse TAR and verify both backgrounds and categories are included
	tr := tar.NewReader(bytes.NewReader(w.Body.Bytes()))
	foundBg := false
	foundCat := false
	for {
		header, err := tr.Next()
		if err != nil {
			break
		}
		name := filepath.ToSlash(header.Name)
		if strings.Contains(name, "files/backgrounds/bg1.jpg") {
			foundBg = true
		}
		if strings.Contains(name, "files/categories/custom.png") {
			foundCat = true
		}
	}
	if !foundBg {
		t.Error("Expected files/backgrounds/bg1.jpg in TAR when medias=true, not found")
	}
	if !foundCat {
		t.Error("Expected files/categories/custom.png in TAR when medias=true, not found")
	}
}

// TestBackupSelectMediasExcludesQuestions verifies that /backup-select?medias=true
// does NOT include questions (i.e. params are truly selective).
func TestBackupSelectMediasExcludesQuestions(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	catDir := filepath.Join(dataDir, "files", "categories")
	qDir := filepath.Join(dataDir, "files", "questions", "1")
	os.MkdirAll(catDir, 0755)
	os.MkdirAll(qDir, 0755)
	os.WriteFile(filepath.Join(catDir, "custom.png"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(qDir, "question.json"), []byte(`{"ID":"1"}`), 0644)

	req := httptest.NewRequest("GET", "/backup-select?medias=true", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	tr := tar.NewReader(bytes.NewReader(w.Body.Bytes()))
	for {
		header, err := tr.Next()
		if err != nil {
			break
		}
		if strings.Contains(filepath.ToSlash(header.Name), "files/questions/") {
			t.Errorf("Questions should NOT be in TAR when only medias=true, found: %s", header.Name)
		}
	}
}

// TestResetSelectWithMediasParam verifies that GET /reset-select?medias=true
// resets backgrounds + categories and returns result["medias"]=true in the JSON response.
func TestResetSelectWithMediasParam(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// Create backgrounds and categories directories with content
	bgDir := filepath.Join(dataDir, "files", "backgrounds")
	catDir := filepath.Join(dataDir, "files", "categories")
	os.MkdirAll(bgDir, 0755)
	os.MkdirAll(catDir, 0755)
	os.WriteFile(filepath.Join(bgDir, "bg.jpg"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(catDir, "cat.png"), []byte("fake"), 0644)

	req := httptest.NewRequest("GET", "/reset-select?medias=true", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", result["status"])
	}

	resetMap, ok := result["reset"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'reset' object in response")
	}

	if resetMap["medias"] != true {
		t.Errorf("Expected reset.medias=true, got %v", resetMap["medias"])
	}

	// Verify categories were cleared (directory recreated empty)
	entries, _ := os.ReadDir(catDir)
	if len(entries) != 0 {
		t.Errorf("Expected categories dir to be empty after reset, found %d files", len(entries))
	}
}

// ---------------------------------------------------------------------------
// ResolveCategoryMeta tests (v5.7.9)
// ---------------------------------------------------------------------------

// TestResolveCategoryMeta_Hardcoded verifies that hardcoded categories return
// the correct name and color, and an empty imageURL.
func TestResolveCategoryMeta_Hardcoded(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	tests := []struct {
		key       string
		wantName  string
		wantColor string
	}{
		{"GEOGRAPHY", "Geographie", "#3b82f6"},
		{"SCIENCE", "Sciences & Nature", "#22c55e"},
		{"ARTS", "Arts & Litterature", "#a855f7"},
		{"SPORTS", "Sports & Loisirs", "#f97316"},
		{"FOOD", "Gastronomie", "#991b1b"},
		{"ANIMALS", "Animaux", "#78716c"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			name, imageURL, color := server.ResolveCategoryMeta(tt.key)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if imageURL != "" {
				t.Errorf("imageURL = %q, want empty for hardcoded", imageURL)
			}
			if color != tt.wantColor {
				t.Errorf("color = %q, want %q", color, tt.wantColor)
			}
		})
	}
}

// TestResolveCategoryMeta_Custom verifies that a custom category created on disk
// returns a non-empty imageURL and the original display name from the sidecar.
func TestResolveCategoryMeta_Custom(t *testing.T) {
	srv, dataDir := setupTestHTTPServer(t)
	catDir := filepath.Join(dataDir, "files", "categories")
	os.MkdirAll(catDir, 0755)

	// Simulate a custom category created by POST /api/categories:
	// image file + sidecar JSON
	os.WriteFile(filepath.Join(catDir, "MON_JEU.png"), minimalPNG, 0644)
	os.WriteFile(filepath.Join(catDir, "MON_JEU.json"),
		[]byte(`{"name":"Mon Jeu"}`), 0644)

	name, imageURL, color := srv.ResolveCategoryMeta("MON_JEU")

	if name != "Mon Jeu" {
		t.Errorf("name = %q, want %q", name, "Mon Jeu")
	}
	if imageURL == "" {
		t.Errorf("imageURL is empty, expected non-empty for custom category")
	}
	if color != "" {
		t.Errorf("color = %q, want empty for custom category", color)
	}
}

// TestResolveCategoryMeta_Unknown verifies that an unknown key returns empty strings
// without panicking.
func TestResolveCategoryMeta_Unknown(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)

	name, imageURL, color := srv.ResolveCategoryMeta("DOES_NOT_EXIST")
	if name != "" || imageURL != "" || color != "" {
		t.Errorf("Expected all empty for unknown key, got name=%q imageURL=%q color=%q", name, imageURL, color)
	}

	// Empty key should also be safe
	name, imageURL, color = srv.ResolveCategoryMeta("")
	if name != "" || imageURL != "" || color != "" {
		t.Errorf("Expected all empty for empty key, got name=%q imageURL=%q color=%q", name, imageURL, color)
	}
}

// TestHistory_CategoryMetaFields verifies that /history serializes
// CATEGORY_NAME, CATEGORY_IMAGE_URL, CATEGORY_COLOR fields when present.
func TestHistory_CategoryMetaFields(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)

	// Add an event with resolved metadata directly (as main.go would after ResolveCategoryMeta)
	srv.engine.AddGameEvent(game.GameEvent{
		Timestamp:           1000000,
		QuestionID:          "q1",
		QuestionText:        "Symbole de l'or?",
		QuestionCategory:    "SCIENCE",
		CategoryDisplayName: "Sciences & Nature",
		CategoryImageURL:    "",
		CategoryColor:       "#22c55e",
		EventType:           "POINTS_AWARDED",
		WinnerType:          "TEAM",
		TeamName:            "Les Bleus",
		TeamColor:           []int{59, 130, 246},
		Points:              10,
	})

	req := httptest.NewRequest("GET", "/history", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /history: status %d", w.Code)
	}

	var events []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&events); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ev := events[0]
	if ev["CATEGORY_NAME"] != "Sciences & Nature" {
		t.Errorf("CATEGORY_NAME = %v, want %q", ev["CATEGORY_NAME"], "Sciences & Nature")
	}
	if ev["CATEGORY_COLOR"] != "#22c55e" {
		t.Errorf("CATEGORY_COLOR = %v, want %q", ev["CATEGORY_COLOR"], "#22c55e")
	}
	// CATEGORY_IMAGE_URL is omitempty and empty → should be absent
	if _, ok := ev["CATEGORY_IMAGE_URL"]; ok {
		t.Errorf("CATEGORY_IMAGE_URL should be absent (omitempty) when empty")
	}
}

// ---------------------------------------------------------------------------
// GET /palmares tests (v5.7.10)
// ---------------------------------------------------------------------------

// TestPalmares_EmptyHistory verifies that GET /palmares returns [] when there are no events.
func TestPalmares_EmptyHistory(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("GET", "/palmares", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /palmares: status %d", w.Code)
	}

	var entries []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected empty array, got %d entries", len(entries))
	}
}

// TestPalmares_WithEvents verifies that GET /palmares returns correct category metadata,
// team/player aggregation and descending totalPoints sort.
func TestPalmares_WithEvents(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)

	// SCIENCE: team 10pts + player 5pts = 15pts total
	// GEOGRAPHY: team 20pts = 20pts total
	// Expected order: GEOGRAPHY (20) > SCIENCE (15)
	events := []game.GameEvent{
		{
			Timestamp: 1000, QuestionCategory: "SCIENCE",
			EventType: "POINTS_AWARDED", WinnerType: "TEAM",
			TeamName: "Les Bleus", TeamColor: []int{59, 130, 246}, Points: 10,
		},
		{
			Timestamp: 2000, QuestionCategory: "SCIENCE",
			EventType: "POINTS_AWARDED", WinnerType: "PLAYER",
			PlayerName: "Alice", TeamName: "Les Rouges", TeamColor: []int{239, 68, 68}, Points: 5,
		},
		{
			Timestamp: 3000, QuestionCategory: "GEOGRAPHY",
			EventType: "POINTS_AWARDED", WinnerType: "TEAM",
			TeamName: "Les Verts", TeamColor: []int{34, 197, 94}, Points: 20,
		},
	}
	for _, ev := range events {
		srv.engine.AddGameEvent(ev)
	}

	req := httptest.NewRequest("GET", "/palmares", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /palmares: status %d — body: %s", w.Code, w.Body.String())
	}

	var entries []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	// First entry must be GEOGRAPHY (most points)
	first := entries[0]
	if first["category"] != "GEOGRAPHY" {
		t.Errorf("First entry category = %v, want GEOGRAPHY", first["category"])
	}
	if first["totalPoints"] != float64(20) {
		t.Errorf("First entry totalPoints = %v, want 20", first["totalPoints"])
	}
	if first["name"] == "" || first["name"] == nil {
		t.Errorf("First entry name should not be empty (hardcoded GEOGRAPHY)")
	}
	if first["color"] == "" || first["color"] == nil {
		t.Errorf("First entry color should not be empty (hardcoded GEOGRAPHY)")
	}

	// Second entry must be SCIENCE
	second := entries[1]
	if second["category"] != "SCIENCE" {
		t.Errorf("Second entry category = %v, want SCIENCE", second["category"])
	}
	if second["totalPoints"] != float64(15) {
		t.Errorf("Second entry totalPoints = %v, want 15", second["totalPoints"])
	}
	sciName, _ := second["name"].(string)
	if sciName != "Sciences & Nature" {
		t.Errorf("SCIENCE name = %q, want %q", sciName, "Sciences & Nature")
	}
	sciColor, _ := second["color"].(string)
	if sciColor != "#22c55e" {
		t.Errorf("SCIENCE color = %q, want %q", sciColor, "#22c55e")
	}

	// SCIENCE should have 1 team + 1 player
	teams, _ := second["teams"].([]interface{})
	players, _ := second["players"].([]interface{})
	if len(teams) != 1 {
		t.Errorf("SCIENCE teams: expected 1, got %d", len(teams))
	}
	if len(players) != 1 {
		t.Errorf("SCIENCE players: expected 1, got %d", len(players))
	}
}

// TestPalmares_CategoryMetaResolved verifies that a custom category with an image
// has a non-empty imageURL in GET /palmares.
func TestPalmares_CategoryMetaResolved(t *testing.T) {
	srv, dataDir := setupTestHTTPServer(t)
	catDir := filepath.Join(dataDir, "files", "categories")
	os.MkdirAll(catDir, 0755)

	// Create custom category files (image + sidecar)
	os.WriteFile(filepath.Join(catDir, "MON_JEU.png"), minimalPNG, 0644)
	os.WriteFile(filepath.Join(catDir, "MON_JEU.json"), []byte(`{"name":"Mon Jeu"}`), 0644)

	srv.engine.AddGameEvent(game.GameEvent{
		Timestamp: 1000, QuestionCategory: "MON_JEU",
		EventType: "POINTS_AWARDED", WinnerType: "TEAM",
		TeamName: "Les Bleus", TeamColor: []int{59, 130, 246}, Points: 15,
	})

	req := httptest.NewRequest("GET", "/palmares", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /palmares: status %d", w.Code)
	}

	var entries []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry["name"] != "Mon Jeu" {
		t.Errorf("Custom name = %v, want %q", entry["name"], "Mon Jeu")
	}
	if entry["imageURL"] == "" || entry["imageURL"] == nil {
		t.Errorf("Custom imageURL should be non-empty for category with image")
	}
}

// TestPalmares_SameTeamMultipleEvents verifies that multiple POINTS_AWARDED events
// for the same team in the same category are aggregated — no double-counting.
// Each team name appears exactly once; points are summed.
func TestPalmares_SameTeamMultipleEvents(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)

	// Same team, two separate events — should appear once with aggregated points
	events := []game.GameEvent{
		{
			Timestamp:        1000,
			QuestionCategory: "SCIENCE",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Bleus",
			TeamColor:        []int{59, 130, 246},
			Points:           10,
		},
		{
			Timestamp:        2000,
			QuestionCategory: "SCIENCE",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Bleus",
			TeamColor:        []int{59, 130, 246},
			Points:           15,
		},
	}
	for _, ev := range events {
		srv.engine.AddGameEvent(ev)
	}

	req := httptest.NewRequest("GET", "/palmares", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /palmares: status %d", w.Code)
	}

	var entries []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	// totalPoints = 10 + 15 = 25
	if entry["totalPoints"] != float64(25) {
		t.Errorf("totalPoints = %v, want 25", entry["totalPoints"])
	}

	teams, _ := entry["teams"].([]interface{})
	// Same team name → aggregated into 1 entry (no double-counting)
	if len(teams) != 1 {
		t.Errorf("teams count = %d, want 1 (no double-counting for same team name)", len(teams))
	}
	team0, _ := teams[0].(map[string]interface{})
	if team0["points"] != float64(25) {
		t.Errorf("team points = %v, want 25", team0["points"])
	}
}

// TestPalmares_UnknownCategory verifies that events with an empty QuestionCategory
// are mapped to the "UNKNOWN" category key in the palmares response.
func TestPalmares_UnknownCategory(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)

	// Empty category → should be keyed as "UNKNOWN"
	srv.engine.AddGameEvent(game.GameEvent{
		Timestamp:        1000,
		QuestionCategory: "",
		EventType:        "POINTS_AWARDED",
		WinnerType:       "TEAM",
		TeamName:         "Les Bleus",
		TeamColor:        []int{59, 130, 246},
		Points:           10,
	})

	req := httptest.NewRequest("GET", "/palmares", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /palmares: status %d", w.Code)
	}

	var entries []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry["category"] != "UNKNOWN" {
		t.Errorf("category = %v, want UNKNOWN (empty QuestionCategory should map to UNKNOWN)", entry["category"])
	}
}

// TestHistory_OldEventNoMetaFields verifies that old events without category metadata
// fields don't cause any errors (graceful zero-value / omitempty behaviour).
func TestHistory_OldEventNoMetaFields(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)

	// Old-style event — no Category* fields
	srv.engine.AddGameEvent(game.GameEvent{
		Timestamp:        2000000,
		QuestionID:       "q2",
		QuestionText:     "Ancienne question",
		QuestionCategory: "HISTORY",
		EventType:        "POINTS_AWARDED",
		WinnerType:       "TEAM",
		TeamName:         "Les Rouges",
		TeamColor:        []int{239, 68, 68},
		Points:           5,
	})

	req := httptest.NewRequest("GET", "/history", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /history: status %d", w.Code)
	}

	var events []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&events); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// The 3 new fields should be absent (omitempty with empty string)
	ev := events[0]
	for _, field := range []string{"CATEGORY_NAME", "CATEGORY_IMAGE_URL", "CATEGORY_COLOR"} {
		if _, ok := ev[field]; ok {
			t.Errorf("Old event should not have %s field (omitempty), but it does: %v", field, ev[field])
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: v6.4.x (#168) — POST /questions "explanation" field (plan tâche B9,
// T8, contracts/models.md §"EXPLANATION").
//
// handleUploadQuestion RECONSTRUCTS the question map from zero on every
// save, only explicitly recopying MEDIA/MEDIA_ANSWER/ORDER from the existing
// file (internal/server/http.go, ~:751-761) — the piège this whole lot
// warns about (plan risk table, "Élevée si non signalé" / "Élevé"): without
// an explicit `r.FormValue("explanation")` read, the note is silently
// destroyed on every single edit of the question, including edits that
// don't touch it at all (e.g. only the QUESTION text changed). Precedent:
// TestHTTPServer_QuestionUpload_DefaultIsSpeedy/NormalMigratedToSpeedy just
// above use the same server/setupTestHTTPServer/mux.ServeHTTP harness.
// ---------------------------------------------------------------------------

// postQuestionMultipart POSTs fields as a multipart/form-data request to
// /questions and returns the recorder — a thin wrapper over
// newMultipartCategoryRequest's multipart.Writer pattern (used elsewhere in
// this file), generalized to an arbitrary field set instead of the fixed
// name/file pair /api/categories expects.
func postQuestionMultipart(t *testing.T, server *HTTPServer, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("failed to write field %q: %v", k, err)
		}
	}
	writer.Close()

	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	return w
}

// readSavedQuestion reads back the persisted question.json for id under
// dataDir/files/questions/<id>/question.json.
func readSavedQuestion(t *testing.T, dataDir, id string) map[string]interface{} {
	t.Helper()
	path := filepath.Join(dataDir, "files", "questions", id, "question.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	var saved map[string]interface{}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}
	return saved
}

// TestHTTPServer_QuestionUpload_ExplanationPersisted is the basic positive
// case: a note submitted alongside a new question is written to disk.
func TestHTTPServer_QuestionUpload_ExplanationPersisted(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	w := postQuestionMultipart(t, server, map[string]string{
		"number":      "1",
		"question":    "Capitale de la France ?",
		"answer":      "Paris",
		"explanation": "Paris est la capitale depuis 508 (Clovis).",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	saved := readSavedQuestion(t, dataDir, "1")
	if saved["EXPLANATION"] != "Paris est la capitale depuis 508 (Clovis)." {
		t.Errorf("EXPLANATION not persisted: got %v", saved["EXPLANATION"])
	}
}

// TestHTTPServer_QuestionUpload_ExplanationSurvivesReedit is #168's core
// regression test — the "piège" itself: handleUploadQuestion rebuilds the
// question map from scratch on EVERY save. A second submission that edits
// an unrelated field (here: the question text) but re-sends the SAME
// explanation value (exactly what the real QuestionsPage editor does — a
// controlled textarea always resubmits its current value) must still leave
// the note intact. Before B9 (no explicit `r.FormValue("explanation")`
// read), this test is RED: the note vanishes on this second save.
func TestHTTPServer_QuestionUpload_ExplanationSurvivesReedit(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	postQuestionMultipart(t, server, map[string]string{
		"number":      "2",
		"question":    "Capitale de l'Allemagne ?",
		"answer":      "Berlin",
		"explanation": "Berlin depuis la réunification de 1990.",
	})
	saved := readSavedQuestion(t, dataDir, "2")
	if saved["EXPLANATION"] != "Berlin depuis la réunification de 1990." {
		t.Fatalf("setup failed: EXPLANATION not persisted on first save, got %v", saved["EXPLANATION"])
	}

	// Re-edit: only the question text changes, but the form (like the real
	// editor) resubmits the SAME explanation value.
	w := postQuestionMultipart(t, server, map[string]string{
		"number":      "2",
		"question":    "Quelle est la capitale de l'Allemagne ?",
		"answer":      "Berlin",
		"explanation": "Berlin depuis la réunification de 1990.",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on re-edit, got %d: %s", w.Code, w.Body.String())
	}

	reedited := readSavedQuestion(t, dataDir, "2")
	if reedited["EXPLANATION"] != "Berlin depuis la réunification de 1990." {
		t.Errorf("#168 B9 piège: EXPLANATION was destroyed by a re-edit that didn't touch it — got %v, want the original note", reedited["EXPLANATION"])
	}
	if reedited["QUESTION"] != "Quelle est la capitale de l'Allemagne ?" {
		t.Errorf("the re-edit itself must still have applied: got QUESTION=%v", reedited["QUESTION"])
	}
}

// TestHTTPServer_QuestionUpload_ExplanationClearedWhenEmptied covers AC15:
// vider le champ dans l'éditeur efface la note — no dedicated deletion code,
// it's the same "only write the key when non-empty" mechanism as CATEGORY.
func TestHTTPServer_QuestionUpload_ExplanationClearedWhenEmptied(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	postQuestionMultipart(t, server, map[string]string{
		"number":      "3",
		"question":    "Une question",
		"answer":      "Une réponse",
		"explanation": "Une note qui va être effacée",
	})
	saved := readSavedQuestion(t, dataDir, "3")
	if saved["EXPLANATION"] == "" {
		t.Fatalf("setup failed: EXPLANATION not persisted on first save")
	}

	postQuestionMultipart(t, server, map[string]string{
		"number":      "3",
		"question":    "Une question",
		"answer":      "Une réponse",
		"explanation": "",
	})

	cleared := readSavedQuestion(t, dataDir, "3")
	if v, ok := cleared["EXPLANATION"]; ok && v != "" {
		t.Errorf("#168 AC15: an emptied explanation field must clear the note — got EXPLANATION=%v", v)
	}
}

// TestHTTPServer_QuestionUpload_NoExplanation_QuestionUnchanged is AC20's
// direct counterpart: a question that never had a note (the common case —
// 85 existing question.json files) must not gain a spurious "EXPLANATION"
// key just because the (empty, untouched) form field always travels with
// every submission.
func TestHTTPServer_QuestionUpload_NoExplanation_QuestionUnchanged(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	postQuestionMultipart(t, server, map[string]string{
		"number":   "4",
		"question": "Question sans note",
		"answer":   "Réponse",
	})

	saved := readSavedQuestion(t, dataDir, "4")
	if v, ok := saved["EXPLANATION"]; ok && v != "" {
		t.Errorf("#168 AC20: a question with no explanation submitted must not gain a non-empty EXPLANATION key — got %v", v)
	}
}
