package server

import (
	"archive/tar"
	"bytes"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// HTTPServer handles HTTP requests
type HTTPServer struct {
	port       int
	engine     *game.Engine
	wsHub      *WebSocketHub
	dataDir    string
	webDir     string
	reactDir   string // React build directory (filesystem)
	embeddedFS fs.FS  // Embedded web filesystem (takes priority over reactDir)
	mux        *http.ServeMux
	server     *http.Server

	// Callbacks
	OnAction           func(action string, data json.RawMessage)
	OnQuestionUpload   func() // Called after question upload to broadcast update
	OnBackgroundChange func(path string) // Called after background upload/delete
	OnShutdown         func() // Called before server shutdown for cleanup
	OnLoadDemo         func() // Called to load demo data
}

// NewHTTPServer creates a new HTTP server
func NewHTTPServer(port int, engine *game.Engine, wsHub *WebSocketHub) *HTTPServer {
	cfg := config.Get()
	return &HTTPServer{
		port:     port,
		engine:   engine,
		wsHub:    wsHub,
		dataDir:  cfg.Storage.DataDir,
		webDir:   cfg.Storage.DataDir,
		reactDir: "", // Will be set if React build exists
		mux:      http.NewServeMux(),
	}
}

// SetReactDir sets the directory for React build files
func (h *HTTPServer) SetReactDir(dir string) {
	h.reactDir = dir
}

// SetEmbeddedFS sets the embedded filesystem for serving web files
// This takes priority over reactDir if set
func (h *HTTPServer) SetEmbeddedFS(fsys fs.FS) {
	h.embeddedFS = fsys
}

// SetWebDir sets the directory for static web files
func (h *HTTPServer) SetWebDir(dir string) {
	h.webDir = dir
}

// Start begins the HTTP server
func (h *HTTPServer) Start() error {
	h.setupRoutes()

	addr := fmt.Sprintf(":%d", h.port)
	h.server = &http.Server{
		Addr:    addr,
		Handler: h.corsMiddleware(h.mux),
	}

	log.Printf("[HTTP] Server starting on port %d", h.port)
	go func() {
		if err := h.server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("[HTTP] Server error: %v", err)
		}
	}()

	return nil
}

// Stop shuts down the HTTP server
func (h *HTTPServer) Stop() {
	if h.server != nil {
		h.server.Close()
	}
}

func (h *HTTPServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *HTTPServer) setupRoutes() {
	// React static files (if React build exists)
	h.mux.HandleFunc("/assets/", h.handleReactAssets)

	// Legacy static files (for backward compatibility)
	h.mux.HandleFunc("/html/", h.handleStatic)
	h.mux.HandleFunc("/js/", h.handleStatic)
	h.mux.HandleFunc("/css/", h.handleStatic)
	h.mux.HandleFunc("/jsSPA/", h.handleStatic)
	h.mux.HandleFunc("/config/", h.handleStatic)
	h.mux.HandleFunc("/files/", h.handleFiles)
	h.mux.HandleFunc("/question/", h.handleQuestion)

	// Windows captive portal
	h.mux.HandleFunc("/connecttest.txt", h.handleWindowsConnect)
	h.mux.HandleFunc("/ncsi.txt", h.handleWindowsConnect)

	// Root and SPA routes
	h.mux.HandleFunc("/", h.handleRoot)
	h.mux.HandleFunc("/redirect", h.handleRedirect)
	h.mux.HandleFunc("/index.html", h.handleRedirect)

	// API
	h.mux.HandleFunc("/version", h.handleVersion)
	h.mux.HandleFunc("/listGame", h.handleListGame)
	h.mux.HandleFunc("/listFiles", h.handleListFiles)
	h.mux.HandleFunc("/questions", h.handleQuestions)
	h.mux.HandleFunc("/history", h.handleHistory)
	h.mux.HandleFunc("/config.json", h.handleConfig)

	// Actions
	h.mux.HandleFunc("/clearGame", h.handleClearGame)
	h.mux.HandleFunc("/clearBuzzers", h.handleClearBuzzers)
	h.mux.HandleFunc("/reboot", h.handleReboot)
	h.mux.HandleFunc("/reset", h.handleReset)
	h.mux.HandleFunc("/shutdown", h.handleShutdown)
	h.mux.HandleFunc("/load-demo", h.handleLoadDemo)

	// Background image upload
	h.mux.HandleFunc("/background", h.handleBackground)

	// Backup/Restore
	h.mux.HandleFunc("/backup", h.handleBackupRedirect)
	h.mux.HandleFunc("/fs-backup", h.handleFSBackup)
	h.mux.HandleFunc("/game-backup", h.handleGameBackup)
	h.mux.HandleFunc("/backup-select", h.handleBackupSelect)
	h.mux.HandleFunc("/restore", h.handleRestore)
	h.mux.HandleFunc("/reset-select", h.handleResetSelect)

	// Update
	h.mux.HandleFunc("/update", h.handleUpdate)

	// WebSocket
	h.mux.HandleFunc("/ws", h.handleWebSocket)
}

func (h *HTTPServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	// For SPA routes or root, serve index.html
	if r.URL.Path == "/" || h.isSPARoute(r.URL.Path) {
		// Try embedded FS first
		if h.embeddedFS != nil {
			if f, err := h.embeddedFS.Open("index.html"); err == nil {
				defer f.Close()
				stat, _ := f.Stat()
				content, _ := io.ReadAll(f)
				http.ServeContent(w, r, "index.html", stat.ModTime(), bytes.NewReader(content))
				return
			}
		}

		// Try filesystem reactDir
		if h.reactDir != "" {
			indexPath := filepath.Join(h.reactDir, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				http.ServeFile(w, r, indexPath)
				return
			}
		}
	}

	// Fallback to legacy behavior
	if r.URL.Path != "/" {
		h.handleNotFound(w, r)
		return
	}
	http.Redirect(w, r, "/html/testSPA.html#config", http.StatusFound)
}

// isSPARoute checks if the path is a React SPA route
// Uses distinct paths to avoid conflicts with API endpoints
func (h *HTTPServer) isSPARoute(path string) bool {
	// /admin and /anim are aliases for the admin interface
	spaRoutes := []string{"/admin", "/anim", "/scoreboard", "/quiz", "/settings", "/tv", "/game", "/teams", "/history-page", "/palmares", "/player"}
	for _, route := range spaRoutes {
		if strings.HasPrefix(path, route) {
			return true
		}
	}
	return false
}

// handleReactAssets serves React build assets
func (h *HTTPServer) handleReactAssets(w http.ResponseWriter, r *http.Request) {
	// Try embedded FS first
	if h.embeddedFS != nil {
		// Remove leading slash for fs.FS
		filePath := strings.TrimPrefix(r.URL.Path, "/")
		if f, err := h.embeddedFS.Open(filePath); err == nil {
			defer f.Close()
			stat, _ := f.Stat()
			content, _ := io.ReadAll(f)
			// Set content type based on extension
			if strings.HasSuffix(filePath, ".js") {
				w.Header().Set("Content-Type", "application/javascript")
			} else if strings.HasSuffix(filePath, ".css") {
				w.Header().Set("Content-Type", "text/css")
			}
			http.ServeContent(w, r, filepath.Base(filePath), stat.ModTime(), bytes.NewReader(content))
			return
		}
	}

	// Fallback to filesystem
	if h.reactDir == "" {
		http.NotFound(w, r)
		return
	}
	filePath := filepath.Join(h.reactDir, r.URL.Path)
	http.ServeFile(w, r, filePath)
}

func (h *HTTPServer) handleRedirect(w http.ResponseWriter, r *http.Request) {
	// If embedded FS or React build exists, redirect to React app
	if h.embeddedFS != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if h.reactDir != "" {
		indexPath := filepath.Join(h.reactDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
	}
	// Fallback to legacy
	http.Redirect(w, r, "/html/testSPA.html#config", http.StatusFound)
}

func (h *HTTPServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Try CURRENT directory first, then fallback
	filePath := filepath.Join(h.webDir, "CURRENT", r.URL.Path)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		filePath = filepath.Join(h.webDir, r.URL.Path)
	}

	http.ServeFile(w, r, filePath)
}

func (h *HTTPServer) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		h.handleDeleteFile(w, r)
		return
	}

	filePath := filepath.Join(h.dataDir, r.URL.Path)
	http.ServeFile(w, r, filePath)
}

func (h *HTTPServer) handleQuestion(w http.ResponseWriter, r *http.Request) {
	// /question/1/media.jpg -> /files/questions/1/media.jpg
	path := strings.TrimPrefix(r.URL.Path, "/question/")
	filePath := filepath.Join(h.dataDir, "files", "questions", path)
	http.ServeFile(w, r, filePath)
}

func (h *HTTPServer) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	filePath := filepath.Join(h.dataDir, r.URL.Path)

	if err := os.RemoveAll(filePath); err != nil {
		http.Error(w, "Failed to delete", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *HTTPServer) handleWindowsConnect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Connection", "close")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Write([]byte("Microsoft NCSI"))
}

func (h *HTTPServer) handleNotFound(w http.ResponseWriter, r *http.Request) {
	host := r.Host

	// Windows captive portal detection
	if strings.Contains(host, "msftncsi.com") || strings.Contains(host, "msftconnecttest.com") {
		h.handleWindowsConnect(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *HTTPServer) handleVersion(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(cfg.Version))
}

func (h *HTTPServer) handleListGame(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(h.engine.GetTeamsAndBumpersJSON())
}

func (h *HTTPServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	history := h.engine.GetHistory()
	json.NewEncoder(w).Encode(history)
}

func (h *HTTPServer) handleListFiles(w http.ResponseWriter, r *http.Request) {
	var result strings.Builder
	filesDir := filepath.Join(h.dataDir, "files")

	filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		relPath, _ := filepath.Rel(filesDir, path)
		if info.IsDir() {
			result.WriteString(fmt.Sprintf("[DIR] %s\n", relPath))
		} else {
			result.WriteString(fmt.Sprintf("%s (%d bytes)\n", relPath, info.Size()))
		}
		return nil
	})

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte("<pre>" + result.String() + "</pre>"))
}

func (h *HTTPServer) handleQuestions(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		h.handleUploadQuestion(w, r)
		return
	}

	// GET - list questions (format matches ESP32: {"/files/questions/1": {...}, "FSINFO": {...}})
	questionsDir := filepath.Join(h.dataDir, "files", "questions")
	questions := make(map[string]interface{})

	entries, err := os.ReadDir(questionsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				questionFile := filepath.Join(questionsDir, entry.Name(), "question.json")
				data, err := os.ReadFile(questionFile)
				if err != nil {
					continue
				}

				var q map[string]interface{}
				if err := json.Unmarshal(data, &q); err != nil {
					continue
				}
				// Set default POINTS_TARGET if not present
				if _, ok := q["POINTS_TARGET"]; !ok {
					qType, _ := q["TYPE"].(string)
					if qType == "QCM" {
						q["POINTS_TARGET"] = "TEAM"
					} else {
						q["POINTS_TARGET"] = "PLAYER"
					}
				}
				// Inject status from engine's questionStatuses map
				qID, _ := q["ID"].(string)
				if qID != "" && h.engine != nil {
					status := h.engine.GetQuestionStatus(qID)
					q["STATUS"] = string(status)
				}
				// Use full path as key (like ESP32)
				key := "/files/questions/" + entry.Name()
				questions[key] = q
			}
		}
	}

	// Add FSINFO (like ESP32)
	questions["FSINFO"] = h.getStorageInfo()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(questions)
}

func (h *HTTPServer) handleUploadQuestion(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get or generate question ID
	id := r.FormValue("number")
	if id == "" {
		id = h.findFreeQuestionID()
	}

	questionsDir := filepath.Join(h.dataDir, "files", "questions", id)
	os.MkdirAll(questionsDir, 0755)

	// Load existing question to preserve media if not updated
	existingQuestion := make(map[string]interface{})
	existingPath := filepath.Join(questionsDir, "question.json")
	if data, err := os.ReadFile(existingPath); err == nil {
		json.Unmarshal(data, &existingQuestion)
	}

	// Save question data
	question := map[string]interface{}{
		"ID":       id,
		"QUESTION": r.FormValue("question"),
		"ANSWER":   r.FormValue("answer"),
		"POINTS":   r.FormValue("points"),
		"TIME":     r.FormValue("time"),
	}

	// Preserve existing media if not replaced
	if media, ok := existingQuestion["MEDIA"].(string); ok && media != "" {
		question["MEDIA"] = media
	}
	if mediaAnswer, ok := existingQuestion["MEDIA_ANSWER"].(string); ok && mediaAnswer != "" {
		question["MEDIA_ANSWER"] = mediaAnswer
	}
	// Preserve ORDER if exists
	if order, ok := existingQuestion["ORDER"]; ok {
		question["ORDER"] = order
	}

	// Handle question type (NORMAL or QCM)
	questionType := r.FormValue("type")
	if questionType == "" {
		questionType = "NORMAL"
	}
	question["TYPE"] = questionType

	// Handle points target (PLAYER or TEAM)
	pointsTarget := r.FormValue("points_target")
	if pointsTarget == "" {
		// Default: NORMAL questions -> PLAYER, QCM questions -> TEAM
		if questionType == "QCM" {
			pointsTarget = "TEAM"
		} else {
			pointsTarget = "PLAYER"
		}
	}
	question["POINTS_TARGET"] = pointsTarget

	// Handle category
	category := r.FormValue("category")
	if category != "" {
		question["CATEGORY"] = category
	}

	// Handle QCM specific fields
	if questionType == "QCM" {
		qcmAnswersStr := r.FormValue("qcm_answers")
		if qcmAnswersStr != "" {
			var qcmAnswers map[string]string
			if err := json.Unmarshal([]byte(qcmAnswersStr), &qcmAnswers); err == nil {
				question["QCM_ANSWERS"] = qcmAnswers
			}
		}
		qcmCorrect := r.FormValue("qcm_correct")
		if qcmCorrect != "" {
			question["QCM_CORRECT"] = qcmCorrect
		}
		// Handle QCM hints enabled flag
		qcmHintsEnabled := r.FormValue("qcm_hints_enabled")
		if qcmHintsEnabled == "true" {
			question["QCM_HINTS_ENABLED"] = true
		} else {
			question["QCM_HINTS_ENABLED"] = false
		}
		// Handle QCM hint thresholds
		if t1Str := r.FormValue("qcm_hint_threshold_1"); t1Str != "" {
			if t1, err := strconv.ParseFloat(t1Str, 64); err == nil && t1 > 0 {
				question["QCM_HINT_THRESHOLD_1"] = t1
			}
		}
		if t2Str := r.FormValue("qcm_hint_threshold_2"); t2Str != "" {
			if t2, err := strconv.ParseFloat(t2Str, 64); err == nil && t2 > 0 {
				question["QCM_HINT_THRESHOLD_2"] = t2
			}
		}
		// Handle QCM penalties
		if p1Str := r.FormValue("qcm_penalty_1"); p1Str != "" {
			if p1, err := strconv.ParseFloat(p1Str, 64); err == nil && p1 > 0 && p1 <= 1 {
				question["QCM_PENALTY_1"] = p1
			}
		}
		if p2Str := r.FormValue("qcm_penalty_2"); p2Str != "" {
			if p2, err := strconv.ParseFloat(p2Str, 64); err == nil && p2 > 0 && p2 <= 1 {
				question["QCM_PENALTY_2"] = p2
			}
		}
	}

	// Handle Memory specific fields
	if questionType == "MEMORY" {
		// Parse memory pairs JSON
		if pairsStr := r.FormValue("memory_pairs"); pairsStr != "" {
			var pairs []map[string]interface{}
			if err := json.Unmarshal([]byte(pairsStr), &pairs); err == nil {
				// Process each pair to handle image uploads
				for i, pair := range pairs {
					pairID := int(pair["ID"].(float64))

					// Handle card1 image upload
					card1FieldName := fmt.Sprintf("memory_card_%d_1", pairID)
					if file, header, err := r.FormFile(card1FieldName); err == nil {
						defer file.Close()
						randomNum := rand.Intn(9000) + 1000
						fileName := fmt.Sprintf("memory_%d_1_%d%s", pairID, randomNum, filepath.Ext(header.Filename))
						filePath := filepath.Join(questionsDir, fileName)
						if dst, err := os.Create(filePath); err == nil {
							io.Copy(dst, file)
							dst.Close()
							// Update pair with image path
							if card1, ok := pair["CARD1"].(map[string]interface{}); ok {
								card1["IMAGE"] = "/question/" + id + "/" + fileName
								card1["IS_IMAGE"] = true
								pairs[i]["CARD1"] = card1
							}
						}
					}

					// Handle card2 image upload
					card2FieldName := fmt.Sprintf("memory_card_%d_2", pairID)
					if file, header, err := r.FormFile(card2FieldName); err == nil {
						defer file.Close()
						randomNum := rand.Intn(9000) + 1000
						fileName := fmt.Sprintf("memory_%d_2_%d%s", pairID, randomNum, filepath.Ext(header.Filename))
						filePath := filepath.Join(questionsDir, fileName)
						if dst, err := os.Create(filePath); err == nil {
							io.Copy(dst, file)
							dst.Close()
							// Update pair with image path
							if card2, ok := pair["CARD2"].(map[string]interface{}); ok {
								card2["IMAGE"] = "/question/" + id + "/" + fileName
								card2["IS_IMAGE"] = true
								pairs[i]["CARD2"] = card2
							}
						}
					}
				}
				question["MEMORY_PAIRS"] = pairs
			}
		}

		// Parse memory config JSON
		if configStr := r.FormValue("memory_config"); configStr != "" {
			var config map[string]interface{}
			if err := json.Unmarshal([]byte(configStr), &config); err == nil {
				question["MEMORY_CONFIG"] = config
			}
		}
	}

	// Handle question media upload
	file, header, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		// Use random number for filename (like ESP32: media_XXXX.jpg)
		randomNum := rand.Intn(9000) + 1000
		fileName := fmt.Sprintf("media_%d%s", randomNum, filepath.Ext(header.Filename))
		filePath := filepath.Join(questionsDir, fileName)

		dst, err := os.Create(filePath)
		if err == nil {
			defer dst.Close()
			io.Copy(dst, file)
			question["MEDIA"] = "/question/" + id + "/" + fileName
		}
	}

	// Handle answer media upload
	fileAnswer, headerAnswer, err := r.FormFile("file_answer")
	if err == nil {
		defer fileAnswer.Close()
		randomNum := rand.Intn(9000) + 1000
		fileName := fmt.Sprintf("media_answer_%d%s", randomNum, filepath.Ext(headerAnswer.Filename))
		filePath := filepath.Join(questionsDir, fileName)

		dst, err := os.Create(filePath)
		if err == nil {
			defer dst.Close()
			io.Copy(dst, fileAnswer)
			question["MEDIA_ANSWER"] = "/question/" + id + "/" + fileName
		}
	}

	// Save question.json
	data, _ := json.MarshalIndent(question, "", "  ")
	os.WriteFile(filepath.Join(questionsDir, "question.json"), data, 0644)

	log.Printf("[HTTP] Question %s saved", id)

	// Broadcast questions update (like ESP32)
	if h.OnQuestionUpload != nil {
		h.OnQuestionUpload()
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "ok"}`))
}

func (h *HTTPServer) findFreeQuestionID() string {
	questionsDir := filepath.Join(h.dataDir, "files", "questions")
	for i := 1; i < 1000; i++ {
		id := fmt.Sprintf("%d", i)
		if _, err := os.Stat(filepath.Join(questionsDir, id)); os.IsNotExist(err) {
			return id
		}
	}
	return "999"
}

// getStorageInfo returns file storage information (like ESP32's printLittleFSInfo)
func (h *HTTPServer) getStorageInfo() map[string]interface{} {
	filesDir := filepath.Join(h.dataDir, "files")

	var totalSize int64 = 0
	filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	// Assume 100MB total storage (like ESP32 LittleFS)
	const maxStorage int64 = 100 * 1024 * 1024
	usedBytes := int(totalSize)
	totalBytes := int(maxStorage)
	freeBytes := totalBytes - usedBytes
	if freeBytes < 0 {
		freeBytes = 0
	}

	pUsed := float64(totalSize) / float64(maxStorage) * 100
	if pUsed > 100 {
		pUsed = 100
	}

	return map[string]interface{}{
		"USED":   usedBytes,
		"FREE":   freeBytes,
		"TOTAL":  totalBytes,
		"P_USED": pUsed,
	}
}

func (h *HTTPServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	configPath := filepath.Join(h.dataDir, "files", "config.json.current")

	if r.Method == "POST" {
		body, _ := io.ReadAll(r.Body)
		os.WriteFile(configPath, body, 0644)
		w.Write([]byte("Config Saved"))
		return
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		data = []byte("{}")
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (h *HTTPServer) handleClearGame(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Clear game requested")

	// Clear files directory
	filesDir := filepath.Join(h.dataDir, "files")
	os.RemoveAll(filesDir)
	os.MkdirAll(filesDir, 0755)

	if h.OnAction != nil {
		h.OnAction("CLEAR_GAME", nil)
	}

	http.Redirect(w, r, "/html/testSPA.html#config", http.StatusFound)
}

func (h *HTTPServer) handleClearBuzzers(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Clear buzzers requested")

	h.engine.ClearBumpers()

	if h.OnAction != nil {
		h.OnAction("CLEAR_BUZZERS", nil)
	}

	http.Redirect(w, r, "/html/testSPA.html#config", http.StatusFound)
}

func (h *HTTPServer) handleReboot(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Reboot requested")
	http.Redirect(w, r, "/html/testSPA.html#config", http.StatusFound)

	// In a real scenario, you might restart the service
	// For now, just log it
	if h.OnAction != nil {
		h.OnAction("REBOOT", nil)
	}
}

func (h *HTTPServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Shutdown requested")
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"shutting_down"}`))

	// Call shutdown callback for cleanup, then exit
	go func() {
		time.Sleep(100 * time.Millisecond) // Give time for response to be sent
		if h.OnShutdown != nil {
			h.OnShutdown()
		}
		log.Printf("[HTTP] Server shutting down...")
		os.Exit(0)
	}()
}

func (h *HTTPServer) handleLoadDemo(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Load demo requested")
	w.Header().Set("Content-Type", "application/json")

	if h.OnLoadDemo != nil {
		h.OnLoadDemo()
		w.Write([]byte(`{"status":"ok","message":"Demo data loaded"}`))
	} else {
		http.Error(w, `{"status":"error","message":"Demo handler not configured"}`, http.StatusInternalServerError)
	}
}

func (h *HTTPServer) handleReset(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Reset requested")

	h.handleClearGame(w, r)

	if h.OnAction != nil {
		h.OnAction("RESET", nil)
	}
}

func (h *HTTPServer) handleBackground(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Background request: method=%s, content-type=%s", r.Method, r.Header.Get("Content-Type"))
	bgDir := filepath.Join(h.dataDir, "files", "backgrounds")

	switch r.Method {
	case "POST":
		// Parse multipart form (max 10MB)
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			log.Printf("[HTTP] Failed to parse multipart form: %v", err)
			http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			log.Printf("[HTTP] FormFile error: %v", err)
			http.Error(w, "No file uploaded: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		os.MkdirAll(bgDir, 0755)

		// Generate unique filename with timestamp
		ext := filepath.Ext(header.Filename)
		if ext == "" {
			ext = ".jpg"
		}
		timestamp := time.Now().UnixMilli()
		fileName := fmt.Sprintf("bg_%d%s", timestamp, ext)
		destPath := filepath.Join(bgDir, fileName)

		dst, err := os.Create(destPath)
		if err != nil {
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, "Failed to write file", http.StatusInternalServerError)
			return
		}

		bgPath := "/files/backgrounds/" + fileName
		log.Printf("[HTTP] Background image uploaded: %s", destPath)

		if h.OnBackgroundChange != nil {
			h.OnBackgroundChange("reload")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok", "path": "` + bgPath + `"}`))

	case "PUT":
		// Update backgrounds config (order, duration)
		var backgrounds []game.Background
		if err := json.NewDecoder(r.Body).Decode(&backgrounds); err != nil {
			log.Printf("[HTTP] Failed to decode backgrounds config: %v", err)
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		h.engine.SetBackgrounds(backgrounds)
		log.Printf("[HTTP] Backgrounds config updated: %d items", len(backgrounds))

		if h.OnBackgroundChange != nil {
			h.OnBackgroundChange("save")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok"}`))

	case "DELETE":
		// Check if deleting specific file or all
		filename := r.URL.Query().Get("file")
		if filename != "" {
			// Delete specific file
			filePath := filepath.Join(bgDir, filepath.Base(filename))
			if err := os.Remove(filePath); err != nil {
				log.Printf("[HTTP] Failed to delete background: %v", err)
			} else {
				log.Printf("[HTTP] Background deleted: %s", filePath)
			}
		} else {
			// Delete all backgrounds
			os.RemoveAll(bgDir)
			log.Printf("[HTTP] All backgrounds removed")
		}

		if h.OnBackgroundChange != nil {
			h.OnBackgroundChange("reload")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok"}`))

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *HTTPServer) removeBackgroundFiles(filesDir string) {
	// Remove any existing background files (legacy - background.jpg, background.png, etc.)
	matches, _ := filepath.Glob(filepath.Join(filesDir, "background.*"))
	for _, match := range matches {
		os.Remove(match)
	}
}

func (h *HTTPServer) handleBackupRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/fs-backup", http.StatusFound)
}

func (h *HTTPServer) handleFSBackup(w http.ResponseWriter, r *http.Request) {
	h.createTARBackup(w, r, h.dataDir, "buzzcontrol-full-backup")
}

func (h *HTTPServer) handleGameBackup(w http.ResponseWriter, r *http.Request) {
	filesDir := filepath.Join(h.dataDir, "files")
	h.createTARBackup(w, r, filesDir, "buzzcontrol-game-backup")
}

// handleBackupSelect creates a selective backup based on query parameters
// Query params: questions=true, teams=true, bumpers=true, history=true, backgrounds=true
func (h *HTTPServer) handleBackupSelect(w http.ResponseWriter, r *http.Request) {
	includeQuestions := r.URL.Query().Get("questions") == "true"
	includeTeams := r.URL.Query().Get("teams") == "true"
	includeBumpers := r.URL.Query().Get("bumpers") == "true"
	includeHistory := r.URL.Query().Get("history") == "true"
	includeBackgrounds := r.URL.Query().Get("backgrounds") == "true"

	// If nothing selected, include everything
	if !includeQuestions && !includeTeams && !includeBumpers && !includeHistory && !includeBackgrounds {
		includeQuestions = true
		includeTeams = true
		includeBumpers = true
		includeHistory = true
		includeBackgrounds = true
	}

	log.Printf("[HTTP] Selective backup: questions=%v, teams=%v, bumpers=%v, history=%v, backgrounds=%v",
		includeQuestions, includeTeams, includeBumpers, includeHistory, includeBackgrounds)

	// Set headers for TAR download
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("buzzcontrol-backup_%s.tar", timestamp)
	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// Create TAR writer
	tw := tar.NewWriter(w)
	defer tw.Close()

	configDir := filepath.Join(h.dataDir, "config")
	filesDir := filepath.Join(h.dataDir, "files")

	// Add questions
	if includeQuestions {
		questionsDir := filepath.Join(filesDir, "questions")
		if _, err := os.Stat(questionsDir); err == nil {
			h.addDirToTAR(tw, questionsDir, "files/questions")
		}
	}

	// Add teams.json
	if includeTeams {
		teamsPath := filepath.Join(configDir, "teams.json")
		if _, err := os.Stat(teamsPath); err == nil {
			h.addFileToTAR(tw, teamsPath, "config/teams.json")
		}
	}

	// Add bumpers.json
	if includeBumpers {
		bumpersPath := filepath.Join(configDir, "bumpers.json")
		if _, err := os.Stat(bumpersPath); err == nil {
			h.addFileToTAR(tw, bumpersPath, "config/bumpers.json")
		}
	}

	// Add history.json
	if includeHistory {
		historyPath := filepath.Join(configDir, "history.json")
		if _, err := os.Stat(historyPath); err == nil {
			h.addFileToTAR(tw, historyPath, "config/history.json")
		}
	}

	// Add question_statuses.json (with questions since they're related)
	if includeQuestions {
		statusesPath := filepath.Join(configDir, "question_statuses.json")
		if _, err := os.Stat(statusesPath); err == nil {
			h.addFileToTAR(tw, statusesPath, "config/question_statuses.json")
		}
	}

	// Add backgrounds
	if includeBackgrounds {
		backgroundsDir := filepath.Join(filesDir, "backgrounds")
		if _, err := os.Stat(backgroundsDir); err == nil {
			h.addDirToTAR(tw, backgroundsDir, "files/backgrounds")
		}
	}

	log.Printf("[HTTP] Selective backup completed")
}

// addFileToTAR adds a single file to TAR archive
func (h *HTTPServer) addFileToTAR(tw *tar.Writer, filePath, tarPath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = tarPath

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tw, file)
	return err
}

// addDirToTAR adds a directory recursively to TAR archive
func (h *HTTPServer) addDirToTAR(tw *tar.Writer, sourceDir, tarPrefix string) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return nil
		}

		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return nil
		}

		header.Name = tarPrefix + "/" + filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return nil
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer file.Close()
			io.Copy(tw, file)
		}

		return nil
	})
}

// handleResetSelect performs selective reset based on query parameters
// Query params: questions=true, teams=true, bumpers=true, history=true, backgrounds=true, all=true
func (h *HTTPServer) handleResetSelect(w http.ResponseWriter, r *http.Request) {
	resetQuestions := r.URL.Query().Get("questions") == "true"
	resetTeams := r.URL.Query().Get("teams") == "true"
	resetBumpers := r.URL.Query().Get("bumpers") == "true"
	resetHistory := r.URL.Query().Get("history") == "true"
	resetBackgrounds := r.URL.Query().Get("backgrounds") == "true"
	resetAll := r.URL.Query().Get("all") == "true"

	// "all" means reset everything
	if resetAll {
		resetQuestions = true
		resetTeams = true
		resetBumpers = true
		resetHistory = true
		resetBackgrounds = true
	}

	log.Printf("[HTTP] Selective reset: questions=%v, teams=%v, bumpers=%v, history=%v, backgrounds=%v",
		resetQuestions, resetTeams, resetBumpers, resetHistory, resetBackgrounds)

	result := make(map[string]bool)
	configDir := filepath.Join(h.dataDir, "config")
	filesDir := filepath.Join(h.dataDir, "files")

	// Reset questions (delete all question directories and statuses)
	if resetQuestions {
		questionsDir := filepath.Join(filesDir, "questions")
		if err := os.RemoveAll(questionsDir); err == nil {
			os.MkdirAll(questionsDir, 0755)
			result["questions"] = true
			log.Printf("[HTTP] Reset: Questions cleared")
		}
		// Also reset question statuses
		h.engine.ClearStatuses()
		statusesPath := filepath.Join(configDir, "question_statuses.json")
		os.Remove(statusesPath)
		log.Printf("[HTTP] Reset: Question statuses cleared")
	}

	// Reset teams (clear engine data and file)
	if resetTeams {
		h.engine.SetTeams(make(map[string]*game.Team))
		teamsPath := filepath.Join(configDir, "teams.json")
		os.Remove(teamsPath)
		result["teams"] = true
		log.Printf("[HTTP] Reset: Teams cleared")
	}

	// Reset bumpers (clear engine data and file)
	if resetBumpers {
		h.engine.SetBumpers(make(map[string]*game.Bumper))
		bumpersPath := filepath.Join(configDir, "bumpers.json")
		os.Remove(bumpersPath)
		result["bumpers"] = true
		log.Printf("[HTTP] Reset: Bumpers cleared")
	}

	// Reset history (clear engine history and file)
	if resetHistory {
		h.engine.ClearHistory()
		historyPath := filepath.Join(configDir, "history.json")
		os.Remove(historyPath)
		result["history"] = true
		log.Printf("[HTTP] Reset: History cleared")
	}

	// Reset backgrounds (delete backgrounds directory)
	if resetBackgrounds {
		backgroundsDir := filepath.Join(filesDir, "backgrounds")
		if err := os.RemoveAll(backgroundsDir); err == nil {
			os.MkdirAll(backgroundsDir, 0755)
			h.engine.ClearBackgrounds()
			result["backgrounds"] = true
			log.Printf("[HTTP] Reset: Backgrounds cleared")
		}
	}

	// Notify clients of changes
	if h.OnAction != nil {
		h.OnAction("RESET_SELECT", nil)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"reset":  result,
	})
}

func (h *HTTPServer) createTARBackup(w http.ResponseWriter, r *http.Request, sourceDir, filenamePrefix string) {
	// Set headers for TAR download
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("%s_%s.tar", filenamePrefix, timestamp)
	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// Create TAR writer
	tw := tar.NewWriter(w)
	defer tw.Close()

	// Walk the source directory
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Create TAR header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Use forward slashes and set the relative path
		header.Name = filepath.ToSlash(relPath)

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// If it's a file, write the content
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("[HTTP] Backup error: %v", err)
	}
}

func (h *HTTPServer) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 100MB)
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read entire file into memory for two-pass processing
	fileData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// First pass: detect what's in the archive
	detected := h.detectTARContents(fileData)
	log.Printf("[HTTP] Restore: Detected contents: %+v", detected)

	// Prepare result
	result := map[string]interface{}{
		"status":   "ok",
		"restored": make(map[string]bool),
	}
	restoredMap := result["restored"].(map[string]bool)

	configDir := filepath.Join(h.dataDir, "config")
	filesDir := filepath.Join(h.dataDir, "files")
	os.MkdirAll(configDir, 0755)
	os.MkdirAll(filesDir, 0755)

	// Second pass: extract files based on what was detected
	tr := tar.NewReader(bytes.NewReader(fileData))

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Failed to read TAR: "+err.Error(), http.StatusInternalServerError)
			return
		}

		tarPath := filepath.ToSlash(header.Name)

		// Determine where to restore based on path prefix
		var targetPath string
		var allowed bool

		switch {
		case strings.HasPrefix(tarPath, "files/questions/"):
			if detected["questions"] {
				targetPath = filepath.Join(h.dataDir, tarPath)
				allowed = true
			}
		case strings.HasPrefix(tarPath, "files/backgrounds/"):
			if detected["backgrounds"] {
				targetPath = filepath.Join(h.dataDir, tarPath)
				allowed = true
			}
		case tarPath == "config/teams.json":
			if detected["teams"] {
				targetPath = filepath.Join(configDir, "teams.json")
				allowed = true
			}
		case tarPath == "config/bumpers.json":
			if detected["bumpers"] {
				targetPath = filepath.Join(configDir, "bumpers.json")
				allowed = true
			}
		case tarPath == "config/history.json":
			if detected["history"] {
				targetPath = filepath.Join(configDir, "history.json")
				allowed = true
			}
		case tarPath == "config/question_statuses.json":
			if detected["questions"] { // Statuses are tied to questions
				targetPath = filepath.Join(configDir, "question_statuses.json")
				allowed = true
			}
		// Legacy format: questions directly in root
		case strings.HasPrefix(tarPath, "questions/"):
			if detected["questions"] {
				targetPath = filepath.Join(filesDir, tarPath)
				allowed = true
			}
		}

		if !allowed {
			continue
		}

		// Security check
		cleanTarget := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanTarget, filepath.Clean(h.dataDir)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(targetPath, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(targetPath), 0755)

			outFile, err := os.Create(targetPath)
			if err != nil {
				log.Printf("[HTTP] Restore create error: %v", err)
				continue
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				log.Printf("[HTTP] Restore copy error: %v", err)
				continue
			}
			outFile.Close()
		}
	}

	// Post-restore: load config files into engine
	if detected["teams"] {
		if err := h.engine.LoadTeams(); err == nil {
			restoredMap["teams"] = true
			log.Printf("[HTTP] Restore: Teams loaded into engine")
		}
	}

	if detected["bumpers"] {
		if err := h.engine.LoadBumpers(); err == nil {
			restoredMap["bumpers"] = true
			log.Printf("[HTTP] Restore: Bumpers loaded into engine")
		}
	}

	if detected["history"] {
		if err := h.engine.LoadHistory(); err == nil {
			h.engine.RecalculateScoresFromHistory()
			restoredMap["history"] = true
			log.Printf("[HTTP] Restore: History loaded and scores recalculated")
		}
	}

	if detected["questions"] {
		restoredMap["questions"] = true
		// Also load question statuses
		if err := h.engine.LoadStatuses(); err == nil {
			log.Printf("[HTTP] Restore: Question statuses loaded into engine")
		}
		log.Printf("[HTTP] Restore: Questions restored")
	}

	if detected["backgrounds"] {
		restoredMap["backgrounds"] = true
		// Reload backgrounds config
		if h.OnBackgroundChange != nil {
			h.OnBackgroundChange("reload")
		}
		log.Printf("[HTTP] Restore: Backgrounds restored")
	}

	log.Printf("[HTTP] Intelligent restore completed")

	if h.OnAction != nil {
		h.OnAction("RESTORE", nil)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// detectTARContents scans a TAR archive and returns what types of content it contains
func (h *HTTPServer) detectTARContents(data []byte) map[string]bool {
	detected := map[string]bool{
		"questions":   false,
		"teams":       false,
		"bumpers":     false,
		"history":     false,
		"backgrounds": false,
	}

	tr := tar.NewReader(bytes.NewReader(data))
	for {
		header, err := tr.Next()
		if err != nil {
			break
		}

		tarPath := filepath.ToSlash(header.Name)

		switch {
		case strings.HasPrefix(tarPath, "files/questions/") || strings.HasPrefix(tarPath, "questions/"):
			detected["questions"] = true
		case strings.HasPrefix(tarPath, "files/backgrounds/") || strings.HasPrefix(tarPath, "backgrounds/"):
			detected["backgrounds"] = true
		case tarPath == "config/teams.json" || tarPath == "teams.json":
			detected["teams"] = true
		case tarPath == "config/bumpers.json" || tarPath == "bumpers.json":
			detected["bumpers"] = true
		case tarPath == "config/history.json" || tarPath == "history.json":
			detected["history"] = true
		}
	}

	return detected
}

func (h *HTTPServer) handleUpdate(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Update requested - downloading from remote")

	// Read base URL from config file (like ESP32)
	baseURLFile := filepath.Join(h.dataDir, "config", "base.url")
	baseURLBytes, err := os.ReadFile(baseURLFile)
	if err != nil {
		log.Printf("[HTTP] Update: Cannot read base URL: %v", err)
		http.Error(w, "Cannot read base URL config", http.StatusInternalServerError)
		return
	}
	baseURL := strings.TrimSpace(string(baseURLBytes))
	if baseURL == "" {
		http.Error(w, "Empty base URL", http.StatusInternalServerError)
		return
	}
	log.Printf("[HTTP] Update: Base URL = %s", baseURL)

	// Read local version
	versionFile := filepath.Join(h.dataDir, "config", "version.txt")
	localVersion := float64(-1)
	if data, err := os.ReadFile(versionFile); err == nil {
		fmt.Sscanf(strings.TrimSpace(string(data)), "%f", &localVersion)
	}
	log.Printf("[HTTP] Update: Local version = %.1f", localVersion)

	// Download remote version
	remoteVersionURL := baseURL + "/config/version.txt"
	remoteVersionStr, err := h.downloadString(remoteVersionURL)
	if err != nil {
		log.Printf("[HTTP] Update: Cannot download remote version: %v", err)
		http.Error(w, "Cannot download remote version", http.StatusInternalServerError)
		return
	}
	remoteVersion := float64(-1)
	fmt.Sscanf(strings.TrimSpace(remoteVersionStr), "%f", &remoteVersion)
	log.Printf("[HTTP] Update: Remote version = %.1f", remoteVersion)

	// Compare versions
	if localVersion >= remoteVersion {
		log.Printf("[HTTP] Update: Already up to date (local=%.1f, remote=%.1f)", localVersion, remoteVersion)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":         "ok",
			"message":        "Already up to date",
			"local_version":  localVersion,
			"remote_version": remoteVersion,
		})
		return
	}

	log.Printf("[HTTP] Update: Updating from %.1f to %.1f", localVersion, remoteVersion)

	// Download catalog file
	catalogURL := baseURL + "/config/catalog.url"
	catalogContent, err := h.downloadString(catalogURL)
	if err != nil {
		log.Printf("[HTTP] Update: Cannot download catalog: %v", err)
		http.Error(w, "Cannot download catalog", http.StatusInternalServerError)
		return
	}

	// Parse catalog and download each file
	tempDir := filepath.Join(h.dataDir, "_temp_update")
	os.RemoveAll(tempDir)
	os.MkdirAll(tempDir, 0755)

	var updatedFiles []string
	lines := strings.Split(catalogContent, "\n")
	for _, line := range lines {
		filePath := strings.TrimSpace(line)
		if filePath == "" {
			continue
		}

		fileURL := baseURL + "/" + filePath
		tempFilePath := filepath.Join(tempDir, filePath)

		if err := h.downloadFile(fileURL, tempFilePath); err != nil {
			log.Printf("[HTTP] Update: Failed to download %s: %v", filePath, err)
			os.RemoveAll(tempDir)
			http.Error(w, "Failed to download: "+filePath, http.StatusInternalServerError)
			return
		}
		updatedFiles = append(updatedFiles, filePath)
		log.Printf("[HTTP] Update: Downloaded %s", filePath)
	}

	// Move temp to CURRENT (atomic update)
	currentDir := filepath.Join(h.dataDir, "CURRENT")
	os.RemoveAll(currentDir)
	if err := os.Rename(tempDir, currentDir); err != nil {
		log.Printf("[HTTP] Update: Failed to move temp to CURRENT: %v", err)
		http.Error(w, "Failed to finalize update", http.StatusInternalServerError)
		return
	}

	// Save new version
	os.WriteFile(filepath.Join(h.dataDir, "CURRENT", "config", "version.txt"),
		[]byte(fmt.Sprintf("%.1f", remoteVersion)), 0644)

	log.Printf("[HTTP] Update: Successfully updated to version %.1f (%d files)", remoteVersion, len(updatedFiles))

	if h.OnAction != nil {
		h.OnAction("UPDATE", nil)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "ok",
		"message":       fmt.Sprintf("Updated to version %.1f", remoteVersion),
		"from_version":  localVersion,
		"to_version":    remoteVersion,
		"updated_files": updatedFiles,
	})
}

// downloadString downloads a URL and returns its content as string
func (h *HTTPServer) downloadString(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// downloadFile downloads a URL to a local file
func (h *HTTPServer) downloadFile(url, destPath string) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (h *HTTPServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	h.wsHub.HandleConnection(w, r)
}
