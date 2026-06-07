package server

import (
	"archive/tar"
	"bytes"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestHTTPServer(t *testing.T) (*HTTPServer, string) {
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

	engine := game.NewEngine()
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

	// Parse response to verify it's valid JSON with neon_effect
	var cfg map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}

	// Verify neon_effect exists in response
	if _, ok := cfg["neon_effect"]; !ok {
		t.Errorf("Expected neon_effect in config, got: %v", cfg)
	}
}

func TestHTTPServer_Config_POST(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Post a valid config with neon_effect
	configJSON := `{
		"version": "2.46.0",
		"neon_effect": {
			"enabled": true,
			"arc_width": 90,
			"intensity_gap": 75,
			"rotation_speed": 5.5
		}
	}`

	req := httptest.NewRequest("POST", "/config.json", strings.NewReader(configJSON))
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Parse response to verify validation
	var cfg map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}

	// Verify neon_effect was saved
	neonEffect, ok := cfg["neon_effect"].(map[string]interface{})
	if !ok {
		t.Fatalf("neon_effect not found in response")
	}

	if neonEffect["enabled"] != true {
		t.Errorf("Expected enabled=true, got %v", neonEffect["enabled"])
	}
	if neonEffect["arc_width"] != float64(90) {
		t.Errorf("Expected arc_width=90, got %v", neonEffect["arc_width"])
	}
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

	// First ID should be 1
	id := server.findFreeQuestionID()
	if id != "1" {
		t.Errorf("Expected first ID to be 1, got %s", id)
	}

	// Create ID 1
	os.MkdirAll(filepath.Join(questionsDir, "1"), 0755)

	// Next should be 2
	id = server.findFreeQuestionID()
	if id != "2" {
		t.Errorf("Expected next ID to be 2, got %s", id)
	}

	// Create 2, 3, 4
	os.MkdirAll(filepath.Join(questionsDir, "2"), 0755)
	os.MkdirAll(filepath.Join(questionsDir, "3"), 0755)
	os.MkdirAll(filepath.Join(questionsDir, "4"), 0755)

	// Next should be 5
	id = server.findFreeQuestionID()
	if id != "5" {
		t.Errorf("Expected next ID to be 5, got %s", id)
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

// TestPostCategory_Valid verifies that a valid POST /api/categories request creates
// the category file on disk and returns the correct CategoryInfo JSON.
func TestPostCategory_Valid(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	body := strings.NewReader(`{"name": "Ma Categorie"}`)
	req := httptest.NewRequest("POST", "/api/categories", body)
	req.Header.Set("Content-Type", "application/json")
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
	if cat.ImageURL != "" {
		t.Errorf("Expected empty imageURL for text-only category, got '%s'", cat.ImageURL)
	}

	// Verify the JSON file was created on disk
	filePath := filepath.Join(dataDir, "files", "categories", "MA_CATEGORIE.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Expected MA_CATEGORIE.json to be created on disk, file not found")
	}
}

// TestPostCategory_EmptyName verifies that POST /api/categories with an empty name returns 400.
func TestPostCategory_EmptyName(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	tests := []struct {
		name string
		body string
	}{
		{"empty name field", `{"name": ""}`},
		{"missing name field", `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/categories", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("%s: Expected 400, got %d", tt.name, w.Code)
			}
		})
	}
}

// TestPostCategory_NameTooLong verifies that POST /api/categories with a name > 50 chars returns 400.
func TestPostCategory_NameTooLong(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	longName := strings.Repeat("A", 51) // 51 chars — above the 50-char limit
	body := `{"name": "` + longName + `"}`

	req := httptest.NewRequest("POST", "/api/categories", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	body := `{"name": "` + exactName + `"}`

	req := httptest.NewRequest("POST", "/api/categories", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for name exactly 50 chars, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPostCategory_ConflictHardcoded verifies that POST /api/categories returns 409
// when the derived key matches a hardcoded category (e.g. "Geographie" → GEOGRAPHY).
func TestPostCategory_ConflictHardcoded(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	// "Geographie" derives to "GEOGRAPHIE" — but "GEOGRAPHY" is hardcoded.
	// Let's use "Science" → "SCIENCE" which is hardcoded.
	body := strings.NewReader(`{"name": "Science"}`)
	req := httptest.NewRequest("POST", "/api/categories", body)
	req.Header.Set("Content-Type", "application/json")
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
	body1 := strings.NewReader(`{"name": "Mon Quiz"}`)
	req1 := httptest.NewRequest("POST", "/api/categories", body1)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	server.mux.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("First POST should succeed, got %d", w1.Code)
	}

	// Second POST with same key — should return 409
	body2 := strings.NewReader(`{"name": "Mon Quiz"}`)
	req2 := httptest.NewRequest("POST", "/api/categories", body2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("Expected 409 conflict for duplicate custom category, got %d", w2.Code)
	}
}

// TestPostCategory_AppearsInGetCategories verifies that a newly created custom category
// is returned by the subsequent GET /api/categories request.
func TestPostCategory_AppearsInGetCategories(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "categories"), 0755)

	// Create a custom category
	body := strings.NewReader(`{"name": "Super Quiz"}`)
	req := httptest.NewRequest("POST", "/api/categories", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST failed with %d", w.Code)
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
			if item.Name != "Super Quiz" {
				t.Errorf("Expected name 'Super Quiz', got '%s'", item.Name)
			}
		}
	}
	if !found {
		t.Error("Expected SUPER_QUIZ to appear in GET /api/categories after POST")
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
