package server

import (
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"encoding/json"
	"io"
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
