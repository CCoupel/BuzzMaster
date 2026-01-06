package server

import (
	"archive/tar"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HTTPServer handles HTTP requests
type HTTPServer struct {
	port      int
	engine    *game.Engine
	wsHub     *WebSocketHub
	dataDir   string
	webDir    string
	mux       *http.ServeMux
	server    *http.Server

	// Callbacks
	OnAction         func(action string, data json.RawMessage)
	OnQuestionUpload func() // Called after question upload to broadcast update
}

// NewHTTPServer creates a new HTTP server
func NewHTTPServer(port int, engine *game.Engine, wsHub *WebSocketHub) *HTTPServer {
	cfg := config.Get()
	return &HTTPServer{
		port:    port,
		engine:  engine,
		wsHub:   wsHub,
		dataDir: cfg.Storage.DataDir,
		webDir:  cfg.Storage.DataDir,
		mux:     http.NewServeMux(),
	}
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
	// Static files
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

	// Redirects
	h.mux.HandleFunc("/", h.handleRoot)
	h.mux.HandleFunc("/redirect", h.handleRedirect)
	h.mux.HandleFunc("/index.html", h.handleRedirect)

	// API
	h.mux.HandleFunc("/version", h.handleVersion)
	h.mux.HandleFunc("/listGame", h.handleListGame)
	h.mux.HandleFunc("/listFiles", h.handleListFiles)
	h.mux.HandleFunc("/questions", h.handleQuestions)
	h.mux.HandleFunc("/config.json", h.handleConfig)

	// Actions
	h.mux.HandleFunc("/clearGame", h.handleClearGame)
	h.mux.HandleFunc("/clearBuzzers", h.handleClearBuzzers)
	h.mux.HandleFunc("/reboot", h.handleReboot)
	h.mux.HandleFunc("/reset", h.handleReset)

	// Background image upload
	h.mux.HandleFunc("/background", h.handleBackground)

	// Backup/Restore
	h.mux.HandleFunc("/backup", h.handleBackupRedirect)
	h.mux.HandleFunc("/fs-backup", h.handleFSBackup)
	h.mux.HandleFunc("/game-backup", h.handleGameBackup)
	h.mux.HandleFunc("/restore", h.handleRestore)

	// Update
	h.mux.HandleFunc("/update", h.handleUpdate)

	// WebSocket
	h.mux.HandleFunc("/ws", h.handleWebSocket)
}

func (h *HTTPServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.handleNotFound(w, r)
		return
	}
	http.Redirect(w, r, "/html/testSPA.html#config", http.StatusFound)
}

func (h *HTTPServer) handleRedirect(w http.ResponseWriter, r *http.Request) {
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

	// Save question data
	question := map[string]interface{}{
		"ID":       id,
		"QUESTION": r.FormValue("question"),
		"ANSWER":   r.FormValue("answer"),
		"POINTS":   r.FormValue("points"),
		"TIME":     r.FormValue("time"),
	}

	// Handle file upload
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

func (h *HTTPServer) handleReset(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Reset requested")

	h.handleClearGame(w, r)

	if h.OnAction != nil {
		h.OnAction("RESET", nil)
	}
}

func (h *HTTPServer) handleBackground(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("background")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save to files directory
	filesDir := filepath.Join(h.dataDir, "files")
	os.MkdirAll(filesDir, 0755)

	// Keep original extension
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	destPath := filepath.Join(filesDir, "background"+ext)

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

	log.Printf("[HTTP] Background image uploaded: %s", destPath)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "ok", "path": "/files/background` + ext + `"}`))
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

	// Target directory for restore
	targetDir := filepath.Join(h.dataDir, "files")

	// Clear existing files directory
	os.RemoveAll(targetDir)
	os.MkdirAll(targetDir, 0755)

	// Read TAR archive
	tr := tar.NewReader(file)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Failed to read TAR: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Build target path
		targetPath := filepath.Join(targetDir, header.Name)

		// Ensure the target path is within targetDir (security)
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(targetDir)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				log.Printf("[HTTP] Restore mkdir error: %v", err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
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

	log.Printf("[HTTP] Restore completed to %s", targetDir)

	if h.OnAction != nil {
		h.OnAction("RESTORE", nil)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "ok"}`))
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
