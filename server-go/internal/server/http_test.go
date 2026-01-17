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

	server := NewHTTPServer(8080, engine, wsHub)
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

	var questions []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &questions); err != nil {
		t.Errorf("Response is not valid JSON: %v", err)
	}

	if len(questions) != 0 {
		t.Errorf("Expected empty questions list, got %d", len(questions))
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

	var questions []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &questions); err != nil {
		t.Errorf("Response is not valid JSON: %v", err)
	}

	if len(questions) != 1 {
		t.Errorf("Expected 1 question, got %d", len(questions))
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
	server, dataDir := setupTestHTTPServer(t)

	// Create config file
	configDir := filepath.Join(dataDir, "files")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.json.current"), []byte(`{"test":true}`), 0644)

	req := httptest.NewRequest("GET", "/config.json", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body != `{"test":true}` {
		t.Errorf("Unexpected config: %s", body)
	}
}

func TestHTTPServer_Config_POST(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// Create config directory
	configDir := filepath.Join(dataDir, "files")
	os.MkdirAll(configDir, 0755)

	req := httptest.NewRequest("POST", "/config.json", strings.NewReader(`{"saved":true}`))
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Verify file was saved
	data, err := os.ReadFile(filepath.Join(configDir, "config.json.current"))
	if err != nil {
		t.Fatalf("Config file not saved: %v", err)
	}

	if string(data) != `{"saved":true}` {
		t.Errorf("Config content mismatch: %s", string(data))
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

	// Currently not implemented
	if w.Code != http.StatusNotImplemented {
		t.Errorf("Expected 501 Not Implemented, got %d", w.Code)
	}
}

func TestHTTPServer_Restore(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := httptest.NewRequest("POST", "/restore", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	// Currently not implemented
	if w.Code != http.StatusNotImplemented {
		t.Errorf("Expected 501 Not Implemented, got %d", w.Code)
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
