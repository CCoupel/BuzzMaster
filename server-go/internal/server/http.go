package server

import (
	"archive/tar"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// HTTPServer handles HTTP requests
type HTTPServer struct {
	port                      int
	engine                    *game.Engine
	wsHub                     *WebSocketHub
	buzzerHub                 *BuzzerWebSocketHub
	logsHub                   *LogsWebSocketHub
	dataDir                   string
	webDir                    string
	reactDir                  string // React build directory (filesystem)
	embeddedFS                fs.FS  // Embedded web filesystem (takes priority over reactDir)
	mux                       *http.ServeMux
	server                    *http.Server
	updater                   *Updater         // Auto-update handler
	firmwareManager           *FirmwareManager // OTA firmware manager (v3.1.0+)
	defaultQuestionImageAsset []byte           // Embedded fallback image (v3.2.2)
	// questionIDMu serializes question ID allocation AND directory creation
	// (contract ai-generation.md §5.1, #8). Without it, two concurrent
	// requests (e.g. a manual upload racing an AI batch generation) can
	// observe the same "free" ID before either reserves it. Every path that
	// creates a question directory — auto-allocated or explicit — goes
	// through resolveQuestionDir, which holds this lock for the full
	// scan-and-reserve (or explicit-create) operation.
	questionIDMu sync.Mutex

	// Callbacks
	OnAction                  func(action string, data json.RawMessage)
	OnQuestionUpload          func()              // Called after question upload to broadcast update
	OnBackgroundChange        func(path string)   // Called after background upload/delete
	OnNewGameBackgroundChange func(action string) // Called after NEW_GAME background upload/delete (v4.0.4)
	OnShutdown                func()              // Called before server shutdown for cleanup
	OnLoadDemo                func()              // Called to load demo data
	OnConfigUpdate            func()              // Called after config update to broadcast to clients
	// Lighting gives the /api/lighting/* handlers the live Hue driver (#207,
	// contracts/hue-bridge.md §7); nil provider or nil driver = "disabled".
	Lighting           LightingProvider
	OnBuzzerWifiConfig func() int // Called to broadcast WiFi config to all buzzers; returns connected buzzer count
	// OnPriorityMessageSent is called after a priority message (OTA_UPDATE, WIFI_CONFIG) is sent to a buzzer.
	// mac is the buzzer MAC, msgID is the generated MSG_ID, action is the protocol action string.
	// The callback registers the message for ACK tracking and sets AckPending on the bumper (v3.8.0).
	OnPriorityMessageSent func(mac, msgID, action string)
}

// NewHTTPServer creates a new HTTP server
func NewHTTPServer(port int, engine *game.Engine, wsHub *WebSocketHub, buzzerHub *BuzzerWebSocketHub, logsHub *LogsWebSocketHub) *HTTPServer {
	cfg := config.Get()
	h := &HTTPServer{
		port:            port,
		engine:          engine,
		wsHub:           wsHub,
		buzzerHub:       buzzerHub,
		logsHub:         logsHub,
		dataDir:         cfg.Storage.DataDir,
		webDir:          cfg.Storage.DataDir,
		reactDir:        "", // Will be set if React build exists
		mux:             http.NewServeMux(),
		updater:         NewUpdater(cfg.Version, cfg.Storage.DataDir),
		firmwareManager: NewFirmwareManager(cfg.Storage.DataDir, cfg.Version),
	}
	// contract ai-multi-provider.md §10: push AI_GENERATION_PROGRESS to a
	// newly-identified admin client immediately if a job is running. Wired
	// here (not left to cmd/server) so it fires uniformly for both the
	// dedicated /ws/admin endpoint (type known at connection time) and the
	// legacy /ws + SET_CLIENT_TYPE flow — and so it works in package-level
	// tests that talk to h.mux directly without the cmd/server App at all.
	if wsHub != nil {
		wsHub.OnClientRegistered = h.pushAIJobProgressToNewAdmin
	}
	return h
}

// GetFirmwareManager returns the firmware manager instance.
// This allows main.go to access it for OTA_PROGRESS handling.
func (h *HTTPServer) GetFirmwareManager() *FirmwareManager {
	return h.firmwareManager
}

// SetDefaultQuestionImageAsset sets the embedded fallback image used when no custom image is uploaded.
func (h *HTTPServer) SetDefaultQuestionImageAsset(data []byte) {
	h.defaultQuestionImageAsset = data
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

// lingerListener wraps a net.Listener and sets SO_LINGER(0) on every accepted
// connection. With linger=0 the kernel sends a TCP RST instead of the normal
// FIN/ACK sequence when the connection is closed, which prevents the socket
// from entering TIME_WAIT. This lets the server port be reused immediately
// after a restart on all platforms (Linux and Windows).
type lingerListener struct{ net.Listener }

func newLingerListener(l net.Listener) *lingerListener { return &lingerListener{l} }

func (l *lingerListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetLinger(0) // RST on close → no TIME_WAIT
	}
	return conn, nil
}

// Start begins the HTTP server.
//
// Two layers of defense against port-busy on restart:
//  1. newReuseAddrListener sets SO_REUSEADDR so the server can bind
//     immediately even if the port is still in TIME_WAIT.
//  2. lingerListener sets SO_LINGER(0) on every accepted connection so
//     that closing it sends RST instead of FIN, eliminating TIME_WAIT
//     for client connections on both Linux and Windows.
func (h *HTTPServer) Start() error {
	h.setupRoutes()

	addr := fmt.Sprintf(":%d", h.port)
	h.server = &http.Server{
		Addr:    addr,
		Handler: h.corsMiddleware(h.mux),
	}

	LogInfo(game.LogComponentHTTP, "Server starting on port %d", h.port)
	go func() {
		for {
			ln, err := newReuseAddrListener("tcp", addr)
			if err != nil {
				if isPortInUse(err) {
					LogWarn(game.LogComponentHTTP, "Port %d busy, retrying in 500ms...", h.port)
					time.Sleep(500 * time.Millisecond)
					continue
				}
				LogError(game.LogComponentHTTP, "Server error (listener): %v", err)
				return
			}
			err = h.server.Serve(newLingerListener(ln))
			if errors.Is(err, http.ErrServerClosed) {
				return
			}
			LogError(game.LogComponentHTTP, "Server error: %v", err)
			return
		}
	}()

	return nil
}

func isPortInUse(err error) bool {
	return strings.Contains(err.Error(), "address already in use") ||
		strings.Contains(err.Error(), "Only one usage")
}

// Stop shuts down the HTTP server
func (h *HTTPServer) Stop() {
	if h.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := h.server.Shutdown(ctx); err != nil {
			LogWarn(game.LogComponentHTTP, "graceful shutdown incomplete: %v", err)
		}
	}
}

func (h *HTTPServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// NOTE: Access-Control-Allow-Credentials: true is intentionally omitted.
		// Combining it with Allow-Origin: * is invalid per the CORS spec and
		// causes browsers to block requests (even without credentials).
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")

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

	// web/public/ files copied verbatim to the dist root by Vite — e.g. the
	// embedded @font-face files (#115) at /fonts/*.woff2. Same handler as
	// /assets/, which resolves the full request path against the embedded FS
	// or reactDir regardless of prefix.
	h.mux.HandleFunc("/fonts/", h.handleReactAssets)

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
	h.mux.HandleFunc("/palmares", h.handlePalmares) // pre-assembled leaderboard (v5.7.10 — #107)
	h.mux.HandleFunc("/config.json", h.handleConfig)
	h.mux.HandleFunc("/game-config.json", h.handleGameConfig) // #150 — game settings, split from /config.json

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

	// Update (legacy route)
	h.mux.HandleFunc("/update", h.handleUpdate)

	// Categories API (v5.6.2 — #95)
	h.mux.HandleFunc("/api/categories", h.handleAPICategories)

	// RAFALE reservoir API (v8.0.0 — #197, contracts/rafale.md §9)
	h.mux.HandleFunc("/api/rafale/questions", h.handleRafaleQuestions)
	// Exact-path registration BEFORE the "/api/rafale/questions/" prefix
	// route below: net/http's ServeMux always prefers the more specific
	// (longer, exact) match regardless of registration order, so
	// "/api/rafale/questions/reset-all" routes here and everything else
	// under the prefix (including "/{id}" and "/{id}/reset") still falls
	// through to handleRafaleQuestionByID. Feature #197.
	h.mux.HandleFunc("/api/rafale/questions/reset-all", h.handleRafaleResetAllUsed)
	h.mux.HandleFunc("/api/rafale/questions/", h.handleRafaleQuestionByID)
	h.mux.HandleFunc("/api/rafale/pool", h.handleRafalePool)
	// RAFALE reservoir AI generation (v8.1.0 — #203, contracts/rafale-ai-generation.md §2).
	// Distinct path from "/api/rafale/questions/" — not a prefix of it, so
	// net/http's exact-match-wins rule (see the comment above) isn't even in
	// play here; still verified by an explicit test (plan task 7 note).
	h.mux.HandleFunc("/api/rafale/generate-questions", h.handleGenerateRafaleQuestions)

	// AI question generator (v6.0.0 — #8)
	h.mux.HandleFunc("/api/generate-questions", h.handleGenerateQuestions)

	// AI API key validation (v6.0.3, #9 — contract ai-key-validation.md §5)
	h.mux.HandleFunc("/api/ai/validate-key", h.handleValidateAPIKey)

	// Ambiance lighting — Hue Bridge (v10.0.0, #207 — contract hue-bridge.md §7)
	h.mux.HandleFunc("/api/lighting/status", h.handleLightingStatus)
	h.mux.HandleFunc("/api/lighting/discover", h.handleLightingDiscover)
	h.mux.HandleFunc("/api/lighting/register", h.handleLightingRegister)
	h.mux.HandleFunc("/api/lighting/lights", h.handleLightingLights)
	h.mux.HandleFunc("/api/lighting/test", h.handleLightingTest)

	// Buzzer API (WiFi config + OTA)
	h.mux.HandleFunc("/api/buzzers", h.handleAPIBuzzers)
	h.mux.HandleFunc("/api/buzzer/wifi-config", h.handleAPIBuzzerWifiConfig)
	h.mux.HandleFunc("/api/buzzer/update-all", h.handleAPIBuzzerUpdateAll)
	h.mux.HandleFunc("/api/buzzer/", h.handleAPIBuzzerRouter)

	// Firmware OTA API (v3.1.0+)
	h.mux.HandleFunc("/api/firmware/buzzclick/version", h.handleAPIFirmwareVersion)
	h.mux.HandleFunc("/api/firmware/buzzclick/latest.bin", h.handleAPIFirmwareDownload)
	h.mux.HandleFunc("/api/firmware/buzzclick/merged.bin", h.handleAPIFirmwareMergedDownload)
	h.mux.HandleFunc("/api/firmware/buzzclick/upload", h.handleAPIFirmwareUpload)
	h.mux.HandleFunc("/api/firmware/buzzclick/restore-embedded", h.handleAPIFirmwareRestoreEmbedded)

	// WiFi defaults API
	h.mux.HandleFunc("/api/wifi/defaults", h.handleAPIWiFiDefaults)

	// Default question image API (v3.2.2)
	h.mux.HandleFunc("/api/config/default-image", h.handleAPIDefaultQuestionImage)

	// ENTRACTE panel image API (v6.5.2, #119) — same single-image pattern
	// as default-image, dedicated storage dir (contract http-endpoints.md
	// §"Mode ENTRACTE")
	h.mux.HandleFunc("/api/game/entracte-image", h.handleAPIEntracteImage)

	// NEW_GAME background images API (v4.0.4) — multi-image, same pattern as /background
	h.mux.HandleFunc("/new-game-backgrounds", h.handleNewGameBackground)

	// Auto-update API
	h.mux.HandleFunc("/api/updates", h.handleAPIUpdates)
	h.mux.HandleFunc("/api/updates/check", h.handleAPIUpdatesCheck)
	h.mux.HandleFunc("/api/updates/download", h.handleAPIUpdatesDownload)
	h.mux.HandleFunc("/api/updates/apply", h.handleAPIUpdatesApply)

	// WebSocket — legacy endpoint (alias for /ws/admin, retro-compat)
	h.mux.HandleFunc("/ws", h.handleWebSocket)

	// WebSocket — dedicated endpoints by client type (v3.8.0)
	h.mux.HandleFunc("/ws/admin", h.handleWebSocketAdmin)
	h.mux.HandleFunc("/ws/tv", h.handleWebSocketTV)
	h.mux.HandleFunc("/ws/player", h.handleWebSocketPlayer)
	// v6.2.0 (#155) — interface animateur, reduced-capability web client
	h.mux.HandleFunc("/ws/anim", h.handleWebSocketAnim)

	// Buzzer WebSocket (dedicated endpoint for physical buzzers)
	h.mux.HandleFunc("/ws/buzzer", h.handleBuzzerWebSocket)

	// Logs WebSocket (dedicated)
	h.mux.HandleFunc("/ws/logs", h.handleLogsWebSocket)
}

func (h *HTTPServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	// For SPA routes or root, serve index.html
	if r.URL.Path == "/" || h.isSPARoute(r.URL.Path) {
		// Prevent caching of index.html to ensure fresh content
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

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
	// #155 (v6.2.0): /anim used to be an alias serving the admin interface —
	// it now serves its own SPA page (the interface animateur, connected on
	// /ws/anim with the reduced ClientTypeAnim). Kept in this list because
	// it's still a React SPA route that needs the same catch-all serving
	// behavior as every other entry here — just no longer the SAME page as
	// /admin. contracts/websocket-endpoints.md documents the endpoint split.
	spaRoutes := []string{"/admin", "/anim", "/scoreboard", "/quiz", "/settings", "/tv", "/game", "/teams", "/history-page", "/palmares", "/player"}
	for _, route := range spaRoutes {
		if strings.HasPrefix(path, route) {
			return true
		}
	}
	return false
}

// handleReactAssets serves React build assets: content-hashed bundle files
// under /assets/ (Vite build pipeline output), and files Vite copies verbatim
// from web/public/ to the dist root — notably /fonts/*.woff2 (#115) — which
// keep their original, fixed filename.
func (h *HTTPServer) handleReactAssets(w http.ResponseWriter, r *http.Request) {
	// /assets/ filenames are content-hashed by Vite, so they can be cached
	// forever. Everything else served by this handler (e.g. /fonts/*.woff2)
	// has a FIXED filename — an "immutable" 1-year cache would survive a font
	// file swap on redeploy, so use a shorter, revalidatable cache instead
	// (#115 code-review 20260726).
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}

	// Set content type based on extension, for both branches below. Go's mime
	// package doesn't register font MIME types on every platform (notably
	// Windows, a BuzzControl deployment target — see CLAUDE.md), so this is
	// set explicitly instead of relying on extension-based sniffing.
	switch {
	case strings.HasSuffix(r.URL.Path, ".js"):
		w.Header().Set("Content-Type", "application/javascript")
	case strings.HasSuffix(r.URL.Path, ".css"):
		w.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(r.URL.Path, ".woff2"):
		w.Header().Set("Content-Type", "font/woff2")
	case strings.HasSuffix(r.URL.Path, ".woff"):
		w.Header().Set("Content-Type", "font/woff")
	}

	// Try embedded FS first
	if h.embeddedFS != nil {
		// Remove leading slash for fs.FS
		filePath := strings.TrimPrefix(r.URL.Path, "/")
		if f, err := h.embeddedFS.Open(filePath); err == nil {
			defer f.Close()
			stat, _ := f.Stat()
			content, _ := io.ReadAll(f)
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

// handlePalmares builds the complete PALMARES leaderboard from game history
// and returns it as a single JSON response — no frontend joining required (v5.7.10 — #107).
//
// GET /palmares → []PalmaresEntry sorted by TotalPoints desc.
// Returns [] when history is empty.
func (h *HTTPServer) handlePalmares(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	history := h.engine.GetHistory()

	type catAccum struct {
		name        string
		imageURL    string
		color       string
		totalPoints int
		teams       map[string]*TeamScore
		players     map[string]*PlayerScore
	}

	catMap := make(map[string]*catAccum)

	for _, event := range history {
		if event.EventType != "POINTS_AWARDED" || event.Points <= 0 {
			continue
		}
		key := event.QuestionCategory
		if key == "" {
			key = "UNKNOWN"
		}

		acc, exists := catMap[key]
		if !exists {
			catName, catImageURL, catColor := h.ResolveCategoryMeta(key)
			acc = &catAccum{
				name:     catName,
				imageURL: catImageURL,
				color:    catColor,
				teams:    make(map[string]*TeamScore),
				players:  make(map[string]*PlayerScore),
			}
			catMap[key] = acc
		}
		acc.totalPoints += event.Points

		switch event.WinnerType {
		case "TEAM":
			if event.TeamName != "" {
				ts, ok := acc.teams[event.TeamName]
				if !ok {
					ts = &TeamScore{Name: event.TeamName, Color: event.TeamColor}
					acc.teams[event.TeamName] = ts
				}
				ts.Points += event.Points
			}
		case "PLAYER":
			if event.PlayerName != "" {
				playerKey := event.TeamName + "|" + event.PlayerName
				ps, ok := acc.players[playerKey]
				if !ok {
					ps = &PlayerScore{Name: event.PlayerName, Team: event.TeamName}
					acc.players[playerKey] = ps
				}
				ps.Points += event.Points
			}
		}
	}

	// Build sorted entries
	entries := make([]PalmaresEntry, 0, len(catMap))
	for key, acc := range catMap {
		entry := PalmaresEntry{
			Category:    key,
			Name:        acc.name,
			ImageURL:    acc.imageURL,
			Color:       acc.color,
			TotalPoints: acc.totalPoints,
			Teams:       make([]TeamScore, 0, len(acc.teams)),
			Players:     make([]PlayerScore, 0, len(acc.players)),
		}
		for _, ts := range acc.teams {
			entry.Teams = append(entry.Teams, *ts)
		}
		sort.Slice(entry.Teams, func(i, j int) bool {
			return entry.Teams[i].Points > entry.Teams[j].Points
		})
		for _, ps := range acc.players {
			entry.Players = append(entry.Players, *ps)
		}
		sort.Slice(entry.Players, func(i, j int) bool {
			return entry.Players[i].Points > entry.Players[j].Points
		})
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].TotalPoints > entries[j].TotalPoints
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
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
				// Migration: "NORMAL" is the legacy name for SPEEDY — normalize on load
				if qType, _ := q["TYPE"].(string); qType == "NORMAL" {
					q["TYPE"] = "SPEEDY"
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

	// Get or generate question ID — allocation and directory creation are a
	// single locked operation (T3.1/T3.2, contract ai-generation.md §5.1) so
	// this can never race a concurrent upload or AI batch generation.
	id, questionsDir, err := h.resolveQuestionDir(r.FormValue("number"))
	if err != nil {
		if errors.Is(err, ErrQuestionIDExhausted) {
			http.Error(w, "No question ID available (1-999 exhausted)", http.StatusInsufficientStorage)
		} else {
			http.Error(w, "Failed to allocate question storage", http.StatusInternalServerError)
		}
		return
	}

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

	// Handle question type (SPEEDY or QCM) — "NORMAL" accepted as alias for backward compatibility
	questionType := r.FormValue("type")
	if questionType == "" {
		questionType = string(game.QuestionTypeSpeedy)
	}
	// Migration: "NORMAL" is the legacy name for SPEEDY — normalize on write
	if questionType == "NORMAL" {
		questionType = string(game.QuestionTypeSpeedy)
	}
	question["TYPE"] = questionType

	// Handle points target (PLAYER or TEAM)
	pointsTarget := r.FormValue("points_target")
	if pointsTarget == "" {
		// Default: SPEEDY questions -> PLAYER, QCM questions -> TEAM
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

	// Handle explanation note (v6.4.x, #168 — contracts/http-endpoints.md,
	// contracts/models.md §EXPLANATION). Explicit read is mandatory: this
	// handler reconstructs `question` from scratch and only carries forward
	// MEDIA/MEDIA_ANSWER/ORDER from the existing file (above/below) — any
	// field not read here is silently destroyed on every re-edit. Writing
	// the key only when non-empty IS the erasure mechanism, no dedicated
	// "clear" code path: an empty/whitespace-only explanation simply isn't
	// written, so EXPLANATION is absent from the reconstructed question.json
	// exactly like an admin who intentionally emptied the field expects.
	explanation := strings.TrimSpace(r.FormValue("explanation"))
	if explanation != "" {
		question["EXPLANATION"] = explanation
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
		// Memory mode (SOLO, CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE)
		memoryMode := r.FormValue("MEMORY_MODE")
		if memoryMode != "" {
			question["MEMORY_MODE"] = memoryMode
		} else {
			question["MEMORY_MODE"] = "SOLO" // Default
		}

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

	// Handle MEMOTION specific fields
	if questionType == "MEMOTION" {
		// Motion mode (SOLO, CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE)
		motionMode := r.FormValue("MOTION_MODE")
		if motionMode != "" {
			question["MOTION_MODE"] = motionMode
		} else {
			question["MOTION_MODE"] = "SOLO" // Default
		}

		// Parse MOTION_MEMORIZE_DURATION (Secret Mode — v5.5.0 #76): 0 = standard mode
		if durStr := r.FormValue("MOTION_MEMORIZE_DURATION"); durStr != "" {
			if dur, err := strconv.Atoi(durStr); err == nil && dur >= 0 {
				question["MOTION_MEMORIZE_DURATION"] = dur
			}
		}

		// Parse motion cards JSON
		if cardsStr := r.FormValue("motion_cards"); cardsStr != "" {
			var cards []map[string]interface{}
			if err := json.Unmarshal([]byte(cardsStr), &cards); err == nil {
				// #184 B-B2 — validate every card's TYPE before any
				// processing below: known type, nestable in a MEMOTION card
				// (also refuses re-nesting MEMOTION — contract §1), and no
				// content orphaned from a different type (contract §3.2,
				// CARD_TYPE_CONTENT_MISMATCH). Checked first, and the whole
				// handler aborts on failure, so a rejected request never
				// leaves a half-written question.json or an orphaned
				// uploaded image behind.
				for _, card := range cards {
					cardTypeStr, _ := card["TYPE"].(string)
					cardType := game.QuestionType(cardTypeStr)
					if cardType != "" && !game.IsNestableInMotionCard(cardType) {
						http.Error(w, fmt.Sprintf(
							"CARD_TYPE_NOT_NESTABLE: card %v declares TYPE=%s, which is unknown or cannot be nested in a MEMOTION card",
							card["ID"], cardType), http.StatusBadRequest)
						return
					}
					if err := game.ValidateCardTypeContent(cardType, card); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
				}

				// Process each card to handle per-face image uploads. The
				// set of <slot> names is now driven by the card's own type
				// descriptor (#184 B-B7, contract §8) instead of being
				// hard-coded to recto/question/answer. SPEEDY declares
				// exactly those three, so every existing editor payload
				// (which only ever sends SPEEDY cards before #184) keeps
				// working unchanged — form field name and behavior both
				// identical to before. A type with fewer slots (QCM:
				// recto, question — no answer face) simply never looks up
				// that upload field, so it can never gain an orphaned
				// ANSWER_IMAGE that B-B2's CARD_TYPE_CONTENT_MISMATCH would
				// otherwise only catch on the NEXT save, not this one.
				//
				// Slot → JSON field name is the mechanical
				// strings.ToUpper(slot)+"_IMAGE" convention already in use
				// (recto→RECTO_IMAGE, question→QUESTION_IMAGE,
				// answer→ANSWER_IMAGE) — holds for every currently-nestable
				// type whose slots map 1:1 to a flat *_IMAGE field (SPEEDY,
				// QCM). MEMORY's "recto"-plus-N-pairs slots
				// (contract §7, not modeled as a flat list) don't fit that
				// convention — TypeDescriptor.MediaSlots for MEMORY only
				// declares "recto" (handled by this generic loop); per-pair
				// images are handled separately below, mirroring the
				// question-host MEMORY upload's own memory_card_<pairID>_1/2
				// handling above, scoped by cardID instead (#187, v7.1.0).
				for i, card := range cards {
					cardID, _ := card["ID"].(string)
					if cardID == "" {
						continue
					}

					cardTypeStr, _ := card["TYPE"].(string)
					cardType := game.QuestionType(cardTypeStr)
					if cardType == "" {
						cardType = game.QuestionTypeSpeedy
					}
					desc, ok := game.TypeDescriptorFor(cardType)
					if !ok {
						// Unreachable: any card with an unregistered TYPE
						// was already rejected above (CARD_TYPE_NOT_NESTABLE).
						// Skip defensively rather than risk a nil slice.
						continue
					}

					for _, slot := range desc.MediaSlots {
						formField := fmt.Sprintf("motion_card_%s_%s", cardID, slot)
						file, header, err := r.FormFile(formField)
						if err != nil {
							continue
						}
						randomNum := rand.Intn(9000) + 1000
						fileName := fmt.Sprintf("motion_%s_%s_%d%s", cardID, slot, randomNum, filepath.Ext(header.Filename))
						filePath := filepath.Join(questionsDir, fileName)
						dst, err := os.Create(filePath)
						if err != nil {
							file.Close()
							continue
						}
						io.Copy(dst, file)
						dst.Close()
						file.Close()
						cards[i][strings.ToUpper(slot)+"_IMAGE"] = "/question/" + id + "/" + fileName
					}

					// #187 (v7.1.0) — MEMORY card: per-pair images.
					// TypeDescriptor.MediaSlots doesn't model these (N pairs,
					// data-driven, not a fixed slot list — contract §7), so
					// they're handled here instead of via the generic slot
					// loop above. Field naming mirrors the question-host
					// MEMORY upload (memory_card_<pairID>_1/2, above) with
					// the card scoped in: motion_card_<cardID>_pair_<pairID>_1/2
					// (dev-frontend/dev-backend coordination, #187).
					if cardType == game.QuestionTypeMemory {
						if pairsRaw, ok := cards[i]["MEMORY_PAIRS"].([]interface{}); ok {
							for pi, pairRaw := range pairsRaw {
								pair, ok := pairRaw.(map[string]interface{})
								if !ok {
									continue
								}
								pairIDFloat, _ := pair["ID"].(float64)
								pairID := int(pairIDFloat)

								card1Field := fmt.Sprintf("motion_card_%s_pair_%d_1", cardID, pairID)
								if file, header, err := r.FormFile(card1Field); err == nil {
									randomNum := rand.Intn(9000) + 1000
									fileName := fmt.Sprintf("motion_%s_pair_%d_1_%d%s", cardID, pairID, randomNum, filepath.Ext(header.Filename))
									filePath := filepath.Join(questionsDir, fileName)
									if dst, err := os.Create(filePath); err == nil {
										io.Copy(dst, file)
										dst.Close()
										if card1, ok := pair["CARD1"].(map[string]interface{}); ok {
											card1["IMAGE"] = "/question/" + id + "/" + fileName
											card1["IS_IMAGE"] = true
											pair["CARD1"] = card1
										}
									}
									file.Close()
								}

								card2Field := fmt.Sprintf("motion_card_%s_pair_%d_2", cardID, pairID)
								if file, header, err := r.FormFile(card2Field); err == nil {
									randomNum := rand.Intn(9000) + 1000
									fileName := fmt.Sprintf("motion_%s_pair_%d_2_%d%s", cardID, pairID, randomNum, filepath.Ext(header.Filename))
									filePath := filepath.Join(questionsDir, fileName)
									if dst, err := os.Create(filePath); err == nil {
										io.Copy(dst, file)
										dst.Close()
										if card2, ok := pair["CARD2"].(map[string]interface{}); ok {
											card2["IMAGE"] = "/question/" + id + "/" + fileName
											card2["IS_IMAGE"] = true
											pair["CARD2"] = card2
										}
									}
									file.Close()
								}
								pairsRaw[pi] = pair
							}
							cards[i]["MEMORY_PAIRS"] = pairsRaw
						}
					}
				}
				question["MOTION_CARDS"] = cards
			}
		}

		// Parse motion config JSON
		if configStr := r.FormValue("motion_config"); configStr != "" {
			var config map[string]interface{}
			if err := json.Unmarshal([]byte(configStr), &config); err == nil {
				question["MOTION_CONFIG"] = config
			}
		}
	}

	// Handle ARDOISE specific fields (v5.6.0)
	if questionType == "ARDOISE" {
		// Keyboard layout: "AZERTY" (default) or "NUMPAD"
		if kbType := r.FormValue("ardoise_keyboard_type"); kbType == "AZERTY" || kbType == "NUMPAD" {
			question["ARDOISE_KEYBOARD_TYPE"] = kbType
		}
		// If not provided or invalid, omit the field (frontend defaults to AZERTY)
	}

	// Handle RAFALE specific fields (v8.0.0, #107, contract §3.3) — round
	// configuration: difficulty/mode/seconds-per-question/max-questions.
	// CATEGORY is already handled generically above (shared field, contract
	// §3.3 bugfix 2026-08-29) — RAFALE reuses it like every other type.
	//
	// ⚠️ Bugfix (2026-08-31, 2nd QUALIF cycle of the SAME external symptom
	// — "3s countdown then nothing"): this block was missing ENTIRELY.
	// QuestionsPage.jsx has sent these 4 fields in the multipart POST since
	// #107 shipped (data.append('RAFALE_DIFFICULTY', ...) etc.), but
	// handleUploadQuestion never read them — no `if questionType ==
	// "RAFALE"` block existed here at all, unlike QCM/MEMORY/MEMOTION/
	// ARDOISE just above. Every RAFALE round-config question ever saved
	// through the editor therefore lost its difficulty/mode/question-time/
	// max-questions on save: loadQuestion() (main.go) reloads it with Go's
	// zero values. RAFALE_QUESTION_TIME/RAFALE_MAX_QUESTIONS both fall back
	// to sane engine-side defaults when zero (3s, 100) and RAFALE_MODE
	// falls back to SOLO — harmless. RAFALE_DIFFICULTY has NO such
	// fallback: 0 is not a valid difficulty (1-3), so the reservoir pool
	// filter (contract §7, exact DIFFICULTY match) never matched ANY
	// question (real reservoir entries are always 1-3) — reproducing the
	// EXACT SAME external symptom the CATEGORY=="" bugfix (commit
	// d6939e51, previous cycle) fixed, but via a genuinely DIFFERENT
	// missing field, which is why that fix alone didn't resolve this
	// recurrence. See participantsConform's own doc comment (engine.go) —
	// extended in the same commit to also gate on RafaleDifficulty, so a
	// future gap in this class (a static round-config field missing from a
	// saved question) can never again reach STARTED silently.
	if questionType == "RAFALE" {
		if diffStr := r.FormValue("RAFALE_DIFFICULTY"); diffStr != "" {
			if diff, err := strconv.Atoi(diffStr); err == nil && diff >= 1 && diff <= 3 {
				question["RAFALE_DIFFICULTY"] = diff
			}
		}
		if mode := r.FormValue("RAFALE_MODE"); mode != "" {
			question["RAFALE_MODE"] = mode
		}
		if qtStr := r.FormValue("RAFALE_QUESTION_TIME"); qtStr != "" {
			if qt, err := strconv.Atoi(qtStr); err == nil && qt > 0 {
				question["RAFALE_QUESTION_TIME"] = qt
			}
		}
		if maxStr := r.FormValue("RAFALE_MAX_QUESTIONS"); maxStr != "" {
			if maxQ, err := strconv.Atoi(maxStr); err == nil && maxQ > 0 {
				if maxQ > 100 {
					maxQ = 100 // contract §7.2 hard cap
				}
				question["RAFALE_MAX_QUESTIONS"] = maxQ
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

	LogInfo(game.LogComponentHTTP, "Question %s saved", id)

	// Broadcast questions update (like ESP32)
	if h.OnQuestionUpload != nil {
		h.OnQuestionUpload()
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "ok"}`))
}

// ErrQuestionIDExhausted is returned by resolveQuestionDir when every ID in
// 1..999 is already taken. Callers map it to HTTP 507 (contract
// ai-generation.md §3/§5.1) — never silently reuse/overwrite question 999.
var ErrQuestionIDExhausted = errors.New("id_exhausted")

// resolveQuestionDir returns the question ID and its directory for an upload,
// creating that directory as part of the same locked operation.
//
//   - explicitID != "": the caller (an edit, or a manual create with a chosen
//     ID) gets exactly that ID; the directory is created with MkdirAll
//     (idempotent — safe to call again on an existing question).
//   - explicitID == "": scans 1..999 and reserves the first free one with
//     os.Mkdir (exclusive — fails if the directory already exists), never
//     os.MkdirAll, so the creation itself is the reservation. If the scan
//     reaches 999 without finding a free slot, returns
//     ErrQuestionIDExhausted instead of the previous silent "999" fallback,
//     which would have overwritten that question.
//
// Both branches run under h.questionIDMu (T3.1/T3.2, contract
// ai-generation.md §5.1) so a manual upload (handleUploadQuestion) and a
// concurrent AI batch generation (handleGenerateQuestions) can never race on
// the same ID — previously an unlocked os.Stat scan with no reservation step.
func (h *HTTPServer) resolveQuestionDir(explicitID string) (id, dir string, err error) {
	h.questionIDMu.Lock()
	defer h.questionIDMu.Unlock()

	questionsDir := filepath.Join(h.dataDir, "files", "questions")

	if explicitID != "" {
		dir = filepath.Join(questionsDir, explicitID)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", "", err
		}
		return explicitID, dir, nil
	}

	if err := os.MkdirAll(questionsDir, 0755); err != nil {
		return "", "", err
	}
	for i := 1; i < 1000; i++ {
		candidate := strconv.Itoa(i)
		candidateDir := filepath.Join(questionsDir, candidate)
		if mkErr := os.Mkdir(candidateDir, 0755); mkErr == nil {
			return candidate, candidateDir, nil
		} else if !os.IsExist(mkErr) {
			return "", "", mkErr
		}
	}
	return "", "", ErrQuestionIDExhausted
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

// maskedConfigJSON builds the JSON response for a Config, masking the
// Anthropic and Groq API keys (neither ever returned to the client) and
// computing their derived *_configured flags. It never mutates cfg or the
// config singleton (contract ai-generation.md §2, §8 S1; the Groq key
// follows the identical rule per ai-multi-provider.md §8).
func maskedConfigJSON(cfg *config.Config) ([]byte, error) {
	resp := *cfg
	// EffectiveXxxAPIKeyConfigured (security incident 2026-08-07): reflects a
	// key supplied via BUZZCONTROL_*_API_KEY env var too, not just the
	// (possibly empty, in a PROD deployment) config.json field — otherwise
	// an env-only deployment would show "no key configured" and the
	// frontend would keep "✨ Générer via IA" disabled despite a working key.
	resp.AI.APIKeyConfigured = resp.AI.EffectiveAnthropicAPIKeyConfigured()
	resp.AI.AnthropicAPIKey = ""
	resp.AI.ClearAPIKey = false
	resp.AI.GroqAPIKeyConfigured = resp.AI.EffectiveGroqAPIKeyConfigured()
	resp.AI.GroqAPIKey = ""
	resp.AI.ClearGroqAPIKey = false
	// Hue Bridge key: same secret regime (v10.0.0, #207 — contract hue-bridge.md §6.1).
	resp.Lighting.APIKeyConfigured = resp.Lighting.EffectiveAPIKeyConfigured()
	resp.Lighting.APIKey = ""
	resp.Lighting.ClearAPIKey = false
	if resp.Lighting.Lights == nil {
		resp.Lighting.Lights = []config.LightingLightEntry{} // [] not null for the frontend
	}
	return json.MarshalIndent(&resp, "", "  ")
}

// handleConfig serves GET/POST /config.json.
//
// POST is additive (contract ai-generation.md §0): it starts from the current
// singleton, and for every top-level section present in the request body,
// resets that section to its zero value before decoding the request's section
// into it — so a section present but missing some of its own fields doesn't
// silently keep stale values, it falls back to defaults instead (applied
// below via config.ApplyDefaults-equivalent, see the exported Save path).
// Sections absent from the body are left completely untouched. This replaces
// the previous behavior, which decoded the body into a zero Config and wrote
// it whole, silently erasing every section the caller didn't send.
//
// The "ai" section is the one documented exception to that whole-section
// replace rule (contract amendment following a bug found in QA on #137,
// _work/handoff/task-dev-backend-20260806-103004.md): ConfigPage.jsx saves
// individual ai.* settings from separate buttons (provider select, API key
// field, batching sliders — see ConfigPage.jsx:343-352 for the key-only
// save), each firing its own POST with only the field(s) it owns. Treating
// "ai" as wholesale-replaced like every other section silently zeroed every
// field the caller didn't happen to include — e.g. saving the Groq key alone
// reset provider back to "anthropic" — which is a real functional bug, not
// merely a test gap: it broke the documented Groq setup flow (QA Scénario 1
// étape 2). So "ai" gets a field-by-field merge instead: a JSON key present
// in the section (even if its value is the zero value, e.g. batch_size: 0)
// overwrites the stored field; a key absent from the section leaves the
// previously stored value untouched. The two secrets already had exactly
// this "absent means preserved" semantics (contract ai-generation.md §2,
// ai-multi-provider.md §8) — this generalizes it to every other ai.* field
// instead of being the sole exception to a wholesale-replace default.
func (h *HTTPServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Hard size limit before reading the body (security audit M1/M4,
		// _work/reports/security-20260805-125747.md) — this endpoint now
		// carries a secret (ai.anthropic_api_key), same motif as
		// handlePostCategory (http.go:1664).
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// #150 (BREAKING): "game" and "neon_effect" moved to their own
		// endpoint, POST /game-config.json (handleGameConfig below) — reject
		// early, before touching the system config singleton, with a message
		// naming the new endpoint so an old client gets an actionable error
		// instead of its game settings silently being dropped.
		if _, ok := raw["game"]; ok {
			http.Error(w, "\"game\" section moved to POST /game-config.json (#150)", http.StatusBadRequest)
			return
		}
		if _, ok := raw["neon_effect"]; ok {
			http.Error(w, "\"neon_effect\" section moved to POST /game-config.json (#150)", http.StatusBadRequest)
			return
		}

		current := config.Get()
		cfg := *current // value copy: every section is a plain struct, no aliasing

		if data, ok := raw["server"]; ok {
			cfg.Server = config.ServerConfig{}
			if err := json.Unmarshal(data, &cfg.Server); err != nil {
				http.Error(w, "Invalid JSON in \"server\" section", http.StatusBadRequest)
				return
			}
		}
		if data, ok := raw["wifi"]; ok {
			cfg.WiFi = config.WiFiConfig{}
			if err := json.Unmarshal(data, &cfg.WiFi); err != nil {
				http.Error(w, "Invalid JSON in \"wifi\" section", http.StatusBadRequest)
				return
			}
		}
		if data, ok := raw["storage"]; ok {
			cfg.Storage = config.StorageConfig{}
			if err := json.Unmarshal(data, &cfg.Storage); err != nil {
				http.Error(w, "Invalid JSON in \"storage\" section", http.StatusBadRequest)
				return
			}
		}
		if data, ok := raw["wifi_defaults"]; ok {
			cfg.WiFiDefaults = config.WiFiDefaultsConfig{}
			if err := json.Unmarshal(data, &cfg.WiFiDefaults); err != nil {
				http.Error(w, "Invalid JSON in \"wifi_defaults\" section", http.StatusBadRequest)
				return
			}
		}
		if data, ok := raw["version"]; ok {
			if err := json.Unmarshal(data, &cfg.Version); err != nil {
				http.Error(w, "Invalid JSON in \"version\" field", http.StatusBadRequest)
				return
			}
		}
		if data, ok := raw["ai"]; ok {
			var incoming config.AIConfig
			if err := json.Unmarshal(data, &incoming); err != nil {
				http.Error(w, "Invalid JSON in \"ai\" section", http.StatusBadRequest)
				return
			}
			if incoming.AnthropicAPIKey != "" && !strings.HasPrefix(incoming.AnthropicAPIKey, anthropicKeyPrefix) {
				http.Error(w, "Format de clé API invalide (attendu : sk-ant-...)", http.StatusBadRequest)
				return
			}
			if incoming.GroqAPIKey != "" && !strings.HasPrefix(incoming.GroqAPIKey, groqKeyPrefix) {
				http.Error(w, "Format de clé API Groq invalide (attendu : gsk_...)", http.StatusBadRequest)
				return
			}

			// Field-by-field merge (see the func-level comment above for
			// why): re-parse the section as a raw key map purely to know
			// which JSON keys the caller actually sent — presence, not
			// value — since `incoming` alone can't distinguish "field
			// omitted" from "field explicitly sent as its zero value" and
			// json.RawMessage. cfg.AI already holds the previously stored
			// values (cfg is a copy of the current singleton) and is only
			// touched below for keys present in aiRaw, so every omitted
			// field is left exactly as it was.
			var aiRaw map[string]json.RawMessage
			if err := json.Unmarshal(data, &aiRaw); err != nil {
				http.Error(w, "Invalid JSON in \"ai\" section", http.StatusBadRequest)
				return
			}
			if _, ok := aiRaw["model"]; ok {
				cfg.AI.Model = incoming.Model
			}
			if _, ok := aiRaw["timeout_seconds"]; ok {
				cfg.AI.TimeoutSeconds = incoming.TimeoutSeconds
			}
			if _, ok := aiRaw["max_questions"]; ok {
				cfg.AI.MaxQuestions = incoming.MaxQuestions
			}
			if _, ok := aiRaw["batch_size"]; ok {
				cfg.AI.BatchSize = incoming.BatchSize
			}
			if _, ok := aiRaw["inter_batch_delay_ms"]; ok {
				cfg.AI.InterBatchDelayMs = incoming.InterBatchDelayMs
			}
			if _, ok := aiRaw["context_token_budget"]; ok {
				cfg.AI.ContextTokenBudget = incoming.ContextTokenBudget
			}
			if _, ok := aiRaw["max_consecutive_failures"]; ok {
				cfg.AI.MaxConsecutiveFailures = incoming.MaxConsecutiveFailures
			}
			if _, ok := aiRaw["provider"]; ok {
				cfg.AI.Provider = incoming.Provider
			}
			if _, ok := aiRaw["groq_model"]; ok {
				cfg.AI.GroqModel = incoming.GroqModel
			}
			// Key-validation verdict (v6.0.3, #9 — contract
			// ai-key-validation.md §7): same "present overwrites, absent
			// preserves" rule as every other plain ai.* field above. The
			// frontend sends *_api_key_verified=true only right after a
			// "valid" POST /api/ai/validate-key verdict, and false when
			// saving after a forced/unverified save (contract §9) — this
			// handler has no opinion of its own on the value, it just stores
			// what it's told, same as batch_size or provider.
			if _, ok := aiRaw["anthropic_api_key_verified"]; ok {
				cfg.AI.AnthropicAPIKeyVerified = incoming.AnthropicAPIKeyVerified
			}
			if _, ok := aiRaw["groq_api_key_verified"]; ok {
				cfg.AI.GroqAPIKeyVerified = incoming.GroqAPIKeyVerified
			}

			// The two secrets keep their existing "absent or empty key
			// preserves the stored value" semantics (contract
			// ai-generation.md §2, identical rule for the Groq key per
			// ai-multi-provider.md §8) — unaffected by the field merge
			// above since cfg.AI.AnthropicAPIKey/GroqAPIKey haven't been
			// touched yet at this point.
			preservedAnthropicKey := cfg.AI.AnthropicAPIKey
			preservedGroqKey := cfg.AI.GroqAPIKey
			cfg.AI.APIKeyConfigured = false
			cfg.AI.ClearAPIKey = false
			cfg.AI.GroqAPIKeyConfigured = false
			cfg.AI.ClearGroqAPIKey = false

			switch {
			case incoming.ClearAPIKey:
				cfg.AI.AnthropicAPIKey = ""
				// "clear_api_key... remettent le flag correspondant à
				// false" (contract §7) — takes precedence over any
				// anthropic_api_key_verified the caller might also have
				// sent, mirroring how ClearAPIKey already wins over
				// AnthropicAPIKey above.
				cfg.AI.AnthropicAPIKeyVerified = false
			case incoming.AnthropicAPIKey != "":
				cfg.AI.AnthropicAPIKey = incoming.AnthropicAPIKey
			default:
				cfg.AI.AnthropicAPIKey = preservedAnthropicKey
			}
			switch {
			case incoming.ClearGroqAPIKey:
				cfg.AI.GroqAPIKey = ""
				cfg.AI.GroqAPIKeyVerified = false
			case incoming.GroqAPIKey != "":
				cfg.AI.GroqAPIKey = incoming.GroqAPIKey
			default:
				cfg.AI.GroqAPIKey = preservedGroqKey
			}
		}

		if data, ok := raw["lighting"]; ok {
			// Same field-by-field merge as "ai" (contract hue-bridge.md §6.1):
			// the Ambiance page saves lights, bridge and enabled from separate
			// steps, each POST carrying only the keys it owns — a whole-section
			// replace would silently erase api_key at the first "save lights".
			var incoming config.LightingConfig
			if err := json.Unmarshal(data, &incoming); err != nil {
				http.Error(w, "Invalid JSON in \"lighting\" section", http.StatusBadRequest)
				return
			}
			var lRaw map[string]json.RawMessage
			if err := json.Unmarshal(data, &lRaw); err != nil {
				http.Error(w, "Invalid JSON in \"lighting\" section", http.StatusBadRequest)
				return
			}
			if _, ok := lRaw["enabled"]; ok {
				cfg.Lighting.Enabled = incoming.Enabled
			}
			if _, ok := lRaw["bridge_ip"]; ok {
				ip := strings.TrimSpace(incoming.BridgeIP)
				if ip != "" {
					// Security (SSRF audit): the driver will talk to this address
					// unattended — only private-network bridges are accepted.
					norm, verr := validateBridgeAddress(r.Context(), ip)
					if verr != nil {
						http.Error(w, "lighting.bridge_ip doit être une adresse du réseau local (privée)", http.StatusBadRequest)
						return
					}
					ip = norm
				}
				cfg.Lighting.BridgeIP = ip
			}
			if _, ok := lRaw["bridge_id"]; ok {
				cfg.Lighting.BridgeID = strings.ToLower(strings.TrimSpace(incoming.BridgeID))
			}
			if _, ok := lRaw["lights"]; ok {
				lights := make([]config.LightingLightEntry, 0, len(incoming.Lights))
				for _, l := range incoming.Lights {
					l.Name = strings.TrimSpace(l.Name)
					if l.Name == "" {
						continue
					}
					if l.Role == "" {
						l.Role = "general"
					}
					lights = append(lights, l)
				}
				cfg.Lighting.Lights = lights
			}
			// The secret: absent or empty preserves, clear_api_key erases,
			// a non-empty value replaces. Derived/request-only flags never persist.
			preservedHueKey := cfg.Lighting.APIKey
			cfg.Lighting.APIKeyConfigured = false
			cfg.Lighting.ClearAPIKey = false
			switch {
			case incoming.ClearAPIKey:
				cfg.Lighting.APIKey = ""
			case incoming.APIKey != "":
				if strings.ContainsAny(incoming.APIKey, "/?#\" \t\r\n") {
					http.Error(w, "Format de clé API Hue invalide", http.StatusBadRequest)
					return
				}
				cfg.Lighting.APIKey = incoming.APIKey
			default:
				cfg.Lighting.APIKey = preservedHueKey
			}
		}

		// Re-apply defaults to any field a partial section reset to zero,
		// exactly as a config.json load would. (Neon effect clamping moved
		// to handleGameConfig with the rest of the game settings, #150.)
		config.ApplyDefaults(&cfg)

		if err := config.Save(&cfg); err != nil {
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}
		config.SetInstance(&cfg)

		if h.OnConfigUpdate != nil {
			h.OnConfigUpdate()
		}

		LogInfo(game.LogComponentHTTP, "Config updated and saved (sections: %v)", keysOf(raw))

		respJSON, err := maskedConfigJSON(&cfg)
		if err != nil {
			http.Error(w, "Failed to encode config", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(respJSON)
		return
	}

	// GET: return current config (secret masked, contract ai-generation.md §2)
	cfg := config.Get()
	data, err := maskedConfigJSON(cfg)
	if err != nil {
		http.Error(w, "Failed to encode config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// handleGameConfig serves GET/POST /game-config.json (#150) — the "jeu"
// (game) counterpart to handleConfig's "système" (system) sections, split
// out so game settings (default delay, neon effect) can be saved/restored
// with a game's data independently of system settings like API keys (see
// config.GameSettings' doc comment for the full rationale). Same additive,
// section-by-section merge semantics as handleConfig: a section present in
// the POST body replaces that section wholesale (after
// ApplyGameSettingsDefaults re-fills any field left at zero by the partial
// replacement); a section absent from the body is left untouched. No secret
// masking needed here (contrast maskedConfigJSON) — GameSettings holds no
// secrets.
func (h *HTTPServer) handleGameConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		current := config.GetGameSettings()
		gs := *current // value copy: every section is a plain struct, no aliasing

		if data, ok := raw["game"]; ok {
			gs.Game = config.GameConfig{}
			if err := json.Unmarshal(data, &gs.Game); err != nil {
				http.Error(w, "Invalid JSON in \"game\" section", http.StatusBadRequest)
				return
			}
		}
		if data, ok := raw["neon_effect"]; ok {
			gs.NeonEffect = config.NeonEffectConfig{}
			if err := json.Unmarshal(data, &gs.NeonEffect); err != nil {
				http.Error(w, "Invalid JSON in \"neon_effect\" section", http.StatusBadRequest)
				return
			}
		}
		// v6.5.2, #119, C1 — the "entracte" section used to live here
		// (QUALIF only, never production). It has moved to game_state.json,
		// edited via the WS action UPDATE_ENTRACTE_CONFIG from the Quiz page
		// (contract http-endpoints.md §"Mode ENTRACTE" — "section supprimée").
		// A residual "entracte" key in a POSTed body is simply ignored: Go's
		// JSON decode above only reads keys this handler explicitly looks
		// up, so it's already a no-op — no migration needed.

		config.ApplyGameSettingsDefaults(&gs)
		gs.ValidateAndClampNeonEffect()

		if err := config.SaveGameSettings(&gs); err != nil {
			http.Error(w, "Failed to save game config", http.StatusInternalServerError)
			return
		}
		config.SetGameSettingsInstance(&gs)

		if h.OnConfigUpdate != nil {
			h.OnConfigUpdate()
		}

		LogInfo(game.LogComponentHTTP, "Game config updated and saved (sections: %v; neon_effect: enabled=%v, mode=%s, arc=%d, intensity=%d, speed=%.1f, offset=%d, thickness=%d; default_delay=%d)",
			keysOf(raw), gs.NeonEffect.Enabled, gs.NeonEffect.Mode, gs.NeonEffect.ArcWidth, gs.NeonEffect.IntensityGap, gs.NeonEffect.RotationSpeed, gs.NeonEffect.BarOffset, gs.NeonEffect.BarThickness, gs.Game.DefaultDelay)

		respJSON, err := json.MarshalIndent(&gs, "", "  ")
		if err != nil {
			http.Error(w, "Failed to encode game config", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(respJSON)
		return
	}

	// GET: return current game config
	gs := config.GetGameSettings()
	data, err := json.MarshalIndent(gs, "", "  ")
	if err != nil {
		http.Error(w, "Failed to encode game config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// keysOf returns the top-level keys of a raw JSON object map, for logging
// which sections a POST /config.json request touched (never logs values —
// in particular never the AI section's content, S2).
func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (h *HTTPServer) handleClearGame(w http.ResponseWriter, r *http.Request) {
	LogInfo(game.LogComponentHTTP, "Clear game requested")

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
	LogInfo(game.LogComponentHTTP, "Clear buzzers requested")

	h.engine.ClearBumpers()

	if h.OnAction != nil {
		h.OnAction("CLEAR_BUZZERS", nil)
	}

	http.Redirect(w, r, "/html/testSPA.html#config", http.StatusFound)
}

func (h *HTTPServer) handleReboot(w http.ResponseWriter, r *http.Request) {
	LogInfo(game.LogComponentHTTP, "Reboot requested")
	http.Redirect(w, r, "/html/testSPA.html#config", http.StatusFound)

	// In a real scenario, you might restart the service
	// For now, just log it
	if h.OnAction != nil {
		h.OnAction("REBOOT", nil)
	}
}

func (h *HTTPServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	LogInfo(game.LogComponentHTTP, "Shutdown requested")
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"shutting_down"}`))

	// Call shutdown callback for cleanup, then exit
	go func() {
		time.Sleep(100 * time.Millisecond) // Give time for response to be sent
		if h.OnShutdown != nil {
			h.OnShutdown()
		}
		LogInfo(game.LogComponentHTTP, "Server shutting down...")
		os.Exit(0)
	}()
}

func (h *HTTPServer) handleLoadDemo(w http.ResponseWriter, r *http.Request) {
	LogInfo(game.LogComponentHTTP, "Load demo requested")
	w.Header().Set("Content-Type", "application/json")

	if h.OnLoadDemo != nil {
		h.OnLoadDemo()
		w.Write([]byte(`{"status":"ok","message":"Demo data loaded"}`))
	} else {
		http.Error(w, `{"status":"error","message":"Demo handler not configured"}`, http.StatusInternalServerError)
	}
}

func (h *HTTPServer) handleReset(w http.ResponseWriter, r *http.Request) {
	LogInfo(game.LogComponentHTTP, "Reset requested")

	h.handleClearGame(w, r)

	if h.OnAction != nil {
		h.OnAction("RESET", nil)
	}
}

func (h *HTTPServer) handleBackground(w http.ResponseWriter, r *http.Request) {
	LogDebug(game.LogComponentHTTP, "Background request: method=%s, content-type=%s", r.Method, r.Header.Get("Content-Type"))
	bgDir := filepath.Join(h.dataDir, "files", "backgrounds")

	switch r.Method {
	case "POST":
		// Parse multipart form (max 10MB)
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			LogError(game.LogComponentHTTP, "Failed to parse multipart form: %v", err)
			http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			LogError(game.LogComponentHTTP, "FormFile error: %v", err)
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
		LogInfo(game.LogComponentHTTP, "Background image uploaded: %s", destPath)

		if h.OnBackgroundChange != nil {
			h.OnBackgroundChange("reload")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok", "path": "` + bgPath + `"}`))

	case "PUT":
		// Update backgrounds config (order, duration)
		var backgrounds []game.Background
		if err := json.NewDecoder(r.Body).Decode(&backgrounds); err != nil {
			LogError(game.LogComponentHTTP, "Failed to decode backgrounds config: %v", err)
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
	cfg := config.Get()
	prefix := fmt.Sprintf("buzzcontrol-full-backup-v%s", cfg.Version)
	h.createTARBackup(w, r, h.dataDir, prefix)
}

func (h *HTTPServer) handleGameBackup(w http.ResponseWriter, r *http.Request) {
	filesDir := filepath.Join(h.dataDir, "files")
	cfg := config.Get()
	prefix := fmt.Sprintf("buzzcontrol-game-backup-v%s", cfg.Version)
	h.createTARBackup(w, r, filesDir, prefix)
}

// CategoryInfo is the JSON response for GET /api/categories
type CategoryInfo struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	ImageURL string `json:"imageURL"`
	IsCustom bool   `json:"isCustom"`
	Color    string `json:"color"` // accent color for PALMARES headers; "" for custom categories
}

// hardcodedCategories lists built-in categories in display order (v5.6.2 — #95)
var hardcodedCategories = []CategoryInfo{
	{Key: "GEOGRAPHY", Name: "Geographie", ImageURL: "", IsCustom: false, Color: "#3b82f6"},
	{Key: "ENTERTAINMENT", Name: "Divertissement", ImageURL: "", IsCustom: false, Color: "#ec4899"},
	{Key: "HISTORY", Name: "Histoire", ImageURL: "", IsCustom: false, Color: "#eab308"},
	{Key: "ARTS", Name: "Arts & Litterature", ImageURL: "", IsCustom: false, Color: "#a855f7"},
	{Key: "SCIENCE", Name: "Sciences & Nature", ImageURL: "", IsCustom: false, Color: "#22c55e"},
	{Key: "SPORTS", Name: "Sports & Loisirs", ImageURL: "", IsCustom: false, Color: "#f97316"},
	{Key: "FOOD", Name: "Gastronomie", ImageURL: "", IsCustom: false, Color: "#991b1b"},
	{Key: "ANIMALS", Name: "Animaux", ImageURL: "", IsCustom: false, Color: "#78716c"},
}

// PalmaresEntry is a single category row in the GET /palmares response (v5.7.10).
type PalmaresEntry struct {
	Category    string        `json:"category"`    // technical key, e.g. "SCIENCE"
	Name        string        `json:"name"`        // display name, e.g. "Sciences & Nature"
	ImageURL    string        `json:"imageURL"`    // "/files/categories/..." or ""
	Color       string        `json:"color"`       // accent hex color or ""
	TotalPoints int           `json:"totalPoints"` // sum of all points for this category
	Teams       []TeamScore   `json:"teams"`       // per-team scores, sorted desc
	Players     []PlayerScore `json:"players"`     // per-player scores, sorted desc
}

// TeamScore aggregates points for one team in a PALMARES category row.
type TeamScore struct {
	Name   string `json:"name"`
	Color  []int  `json:"color"` // RGB triplet, same format as GameState
	Points int    `json:"points"`
}

// PlayerScore aggregates points for one player in a PALMARES category row.
type PlayerScore struct {
	Name   string `json:"name"`
	Team   string `json:"team"`
	Points int    `json:"points"`
}

// ResolveCategoryMeta returns the display name, image URL and accent color for a category key.
// It first checks hardcoded categories, then scans custom image files on disk.
// Used at GameEvent recording time to embed resolved metadata in history (v5.7.9).
func (h *HTTPServer) ResolveCategoryMeta(key string) (name, imageURL, color string) {
	// Check hardcoded categories
	for _, c := range hardcodedCategories {
		if c.Key == key {
			return c.Name, c.ImageURL, c.Color
		}
	}
	if key == "" {
		return
	}
	// Check custom categories on disk
	catDir := filepath.Join(h.dataDir, "files", "categories")
	entries, _ := os.ReadDir(catDir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
			continue
		}
		stem := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if toUpperSnakeCase(stem) == key {
			imageURL = "/files/categories/" + url.PathEscape(entry.Name())
			// Prefer sidecar JSON name, fall back to stem
			metaPath := filepath.Join(catDir, key+".json")
			if data, err := os.ReadFile(metaPath); err == nil {
				var meta struct {
					Name string `json:"name"`
				}
				if json.Unmarshal(data, &meta) == nil && meta.Name != "" {
					name = meta.Name
				} else {
					name = stem
				}
			} else {
				name = stem
			}
			return
		}
	}
	return
}

// toUpperSnakeCase converts a filename stem to UPPER_SNAKE_CASE (spaces/hyphens → underscore, upper-case).
func toUpperSnakeCase(s string) string {
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return strings.ToUpper(s)
}

// handleAPICategories returns the merged list of hardcoded + custom categories (v5.6.2 — #95)
// GET: returns hardcoded + custom image categories
// POST: creates a new category with name + image (multipart/form-data) (v5.7.2 — #100)
func (h *HTTPServer) handleAPICategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetCategories(w, r)
	case http.MethodPost:
		h.handlePostCategory(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetCategories returns the merged list of hardcoded + custom image categories
func (h *HTTPServer) handleGetCategories(w http.ResponseWriter, r *http.Request) {
	// Build a set of hardcoded keys for dedup
	hardcodedKeys := make(map[string]struct{}, len(hardcodedCategories))
	for _, c := range hardcodedCategories {
		hardcodedKeys[c.Key] = struct{}{}
	}

	catDir := filepath.Join(h.dataDir, "files", "categories")
	entries, _ := os.ReadDir(catDir)

	// Track custom keys to deduplicate within image files
	customKeys := make(map[string]struct{})
	var custom []CategoryInfo

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
			continue
		}
		stem := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		key := toUpperSnakeCase(stem)
		if _, exists := hardcodedKeys[key]; exists {
			log.Printf("[HTTP] Categories: custom file %q conflicts with hardcoded key %s — skipped", entry.Name(), key)
			continue
		}
		if _, exists := customKeys[key]; exists {
			continue // deduplicate same key with different extension
		}
		customKeys[key] = struct{}{}

		// Resolve display name: prefer sidecar <KEY>.json, fall back to stem
		displayName := stem
		metaPath := filepath.Join(catDir, key+".json")
		if data, err := os.ReadFile(metaPath); err == nil {
			var meta struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(data, &meta) == nil && meta.Name != "" {
				displayName = meta.Name
			}
		}

		custom = append(custom, CategoryInfo{
			Key:      key,
			Name:     displayName,
			ImageURL: "/files/categories/" + url.PathEscape(entry.Name()),
			IsCustom: true,
		})
	}

	result := make([]CategoryInfo, 0, len(hardcodedCategories)+len(custom))
	result = append(result, hardcodedCategories...)
	result = append(result, custom...)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handlePostCategory creates a new custom category with name + image upload (v5.7.2 — #100)
// Expects multipart/form-data with fields: name (string) + file (image: png/jpg/jpeg/webp)
func (h *HTTPServer) handlePostCategory(w http.ResponseWriter, r *http.Request) {
	// Enforce 10 MB hard limit on the request body before parsing
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	// Parse multipart form (max 10 MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, `{"error":"invalid multipart form"}`, http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}
	if len(name) > 50 {
		http.Error(w, `{"error":"name is too long (max 50 chars)"}`, http.StatusBadRequest)
		return
	}
	// Security: reject any character that is not alphanumeric, space, hyphen or underscore
	// to prevent path traversal and filesystem injection.
	for _, ch := range name {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != ' ' && ch != '-' && ch != '_' {
			http.Error(w, `{"error":"name contains invalid characters"}`, http.StatusBadRequest)
			return
		}
	}

	// Read uploaded image file
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"file is required"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate image extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExt := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}
	if !allowedExt[ext] {
		http.Error(w, `{"error":"invalid file type, allowed: png, jpg, jpeg, webp"}`, http.StatusBadRequest)
		return
	}

	key := toUpperSnakeCase(name)

	// Check conflict with hardcoded categories
	for _, c := range hardcodedCategories {
		if c.Key == key {
			http.Error(w, `{"error":"key already exists"}`, http.StatusConflict)
			return
		}
	}

	catDir := filepath.Join(h.dataDir, "files", "categories")
	if err := os.MkdirAll(catDir, 0755); err != nil {
		http.Error(w, `{"error":"failed to create category directory"}`, http.StatusInternalServerError)
		return
	}

	// Check conflict with existing custom categories (any image extension)
	entries, _ := os.ReadDir(catDir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		stem := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if toUpperSnakeCase(stem) == key {
			http.Error(w, `{"error":"key already exists"}`, http.StatusConflict)
			return
		}
	}

	// Save image file
	filePath := filepath.Join(catDir, key+ext)
	outFile, err := os.Create(filePath)
	if err != nil {
		log.Printf("[HTTP] Categories POST: failed to create %s: %v", filePath, err)
		http.Error(w, `{"error":"failed to save image"}`, http.StatusInternalServerError)
		return
	}
	defer outFile.Close()
	if _, err := io.Copy(outFile, file); err != nil {
		log.Printf("[HTTP] Categories POST: failed to write image %s: %v", filePath, err)
		http.Error(w, `{"error":"failed to save image"}`, http.StatusInternalServerError)
		return
	}

	// Save sidecar JSON to persist the original display name (v5.7.7)
	metaPath := filepath.Join(catDir, key+".json")
	if metaFile, err := os.Create(metaPath); err == nil {
		_ = json.NewEncoder(metaFile).Encode(struct {
			Name string `json:"name"`
		}{Name: name})
		metaFile.Close()
	} else {
		log.Printf("[HTTP] Categories POST: warning — could not save sidecar metadata %s: %v", metaPath, err)
	}

	imageURL := "/files/categories/" + url.PathEscape(key+ext)
	log.Printf("[HTTP] Categories POST: created category key=%s name=%q file=%s", key, name, key+ext)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CategoryInfo{
		Key:      key,
		Name:     name,
		ImageURL: imageURL,
		IsCustom: true,
	})
}

// ---- RAFALE reservoir API (v8.0.0 — #197, contracts/rafale.md §9) --------

// rafaleQuestionResponse is the wire shape of one reservoir question for
// GET /api/rafale/questions — USED is derived at read time from
// rafale_used.json, never stored in the reservoir itself (contract §3.2/§9).
type rafaleQuestionResponse struct {
	ID         string `json:"ID"`
	Question   string `json:"QUESTION"`
	Answer     string `json:"ANSWER"`
	Category   string `json:"CATEGORY"`
	Difficulty int    `json:"DIFFICULTY"`
	Used       bool   `json:"USED"`
}

// isKnownRafaleCategory reports whether key matches a hardcoded category or
// a custom category discovered on disk (data/files/categories/) — same
// vocabulary check as handleGetCategories, reused here for reservoir
// question validation (contract §9, POST /api/rafale/questions 400 on an
// unknown category).
func (h *HTTPServer) isKnownRafaleCategory(key string) bool {
	for _, c := range hardcodedCategories {
		if c.Key == key {
			return true
		}
	}
	catDir := filepath.Join(h.dataDir, "files", "categories")
	entries, _ := os.ReadDir(catDir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
			continue
		}
		stem := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if toUpperSnakeCase(stem) == key {
			return true
		}
	}
	return false
}

// handleRafaleQuestions handles GET (list, optional filters) and POST
// (create/update) on the RAFALE reservoir — contracts/rafale.md §9.
func (h *HTTPServer) handleRafaleQuestions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetRafaleQuestions(w, r)
	case http.MethodPost:
		h.handlePostRafaleQuestion(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetRafaleQuestions lists the reservoir, optionally filtered by
// ?categories=A,B (OR) and/or ?difficulty=N — contract §9.
func (h *HTTPServer) handleGetRafaleQuestions(w http.ResponseWriter, r *http.Request) {
	var catFilter map[string]struct{}
	if raw := r.URL.Query().Get("categories"); raw != "" {
		catFilter = make(map[string]struct{})
		for _, c := range strings.Split(raw, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				catFilter[c] = struct{}{}
			}
		}
	}

	hasDiffFilter := false
	diffFilter := 0
	if raw := r.URL.Query().Get("difficulty"); raw != "" {
		d, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "Invalid difficulty", http.StatusBadRequest)
			return
		}
		diffFilter = d
		hasDiffFilter = true
	}

	questions, used := h.engine.SnapshotRafaleReservoir()

	result := make([]rafaleQuestionResponse, 0, len(questions))
	for _, q := range questions {
		if catFilter != nil {
			if _, ok := catFilter[string(q.Category)]; !ok {
				continue
			}
		}
		if hasDiffFilter && q.Difficulty != diffFilter {
			continue
		}
		result = append(result, rafaleQuestionResponse{
			ID:         q.ID,
			Question:   q.Question,
			Answer:     q.Answer,
			Category:   string(q.Category),
			Difficulty: q.Difficulty,
			Used:       used[q.ID],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"QUESTIONS": result,
		"TOTAL":     len(result),
	})
}

// handlePostRafaleQuestion creates (no ID in body) or updates (ID present)
// one reservoir question — contract §9. Body is JSON, not multipart:
// reservoir questions carry no media (arbitrage D3).
func (h *HTTPServer) handlePostRafaleQuestion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID         string `json:"ID"`
		Question   string `json:"QUESTION"`
		Answer     string `json:"ANSWER"`
		Category   string `json:"CATEGORY"`
		Difficulty int    `json:"DIFFICULTY"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Question) == "" {
		http.Error(w, "QUESTION must not be empty", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Answer) == "" {
		http.Error(w, "ANSWER must not be empty", http.StatusBadRequest)
		return
	}
	// Length caps (contract rafale-ai-generation.md §5.3, feature #203) —
	// [BREAKING] mineur : ces plafonds n'existaient pas avant v8.1.0. Aucune
	// question existante n'est supprimée ni tronquée ; ils s'appliquent
	// uniquement à l'écriture (create/update depuis cet éditeur), et sont la
	// MÊME source (rafaleMaxQuestionRunes/rafaleMaxAnswerRunes,
	// ai_generator_rafale.go) que celle utilisée par la génération IA — un
	// texte accepté ici est garanti ré-écrivable depuis le chemin IA et
	// inversement.
	if n := rafaleRuneLen(body.Question); n > rafaleMaxQuestionRunes {
		http.Error(w, fmt.Sprintf("QUESTION must be at most %d characters (got %d)", rafaleMaxQuestionRunes, n), http.StatusBadRequest)
		return
	}
	if n := rafaleRuneLen(body.Answer); n > rafaleMaxAnswerRunes {
		http.Error(w, fmt.Sprintf("ANSWER must be at most %d characters (got %d)", rafaleMaxAnswerRunes, n), http.StatusBadRequest)
		return
	}
	if body.Difficulty < 1 || body.Difficulty > 3 {
		http.Error(w, "DIFFICULTY must be between 1 and 3", http.StatusBadRequest)
		return
	}
	if !h.isKnownRafaleCategory(body.Category) {
		http.Error(w, "Unknown CATEGORY", http.StatusBadRequest)
		return
	}

	saved, err := h.engine.UpsertRafaleQuestion(game.RafaleQuestion{
		ID:         body.ID,
		Question:   body.Question,
		Answer:     body.Answer,
		Category:   game.QuestionCategory(body.Category),
		Difficulty: body.Difficulty,
	})
	if err != nil {
		log.Printf("[HTTP] Rafale question upsert failed: %v", err)
		http.Error(w, "Failed to save question", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"ID": saved.ID})
}

// handleRafaleQuestionByID handles DELETE /api/rafale/questions/{id} and
// POST /api/rafale/questions/{id}/reset — contract §9. The "/reset" suffix
// (feature #197) is parsed here rather than given its own mux route: the
// route registered is the "/api/rafale/questions/" prefix, so anything
// after {id} (or {id} itself) is this handler's job to interpret.
func (h *HTTPServer) handleRafaleQuestionByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/rafale/questions/")
	path = strings.TrimSuffix(path, "/")

	if id, ok := strings.CutSuffix(path, "/reset"); ok {
		h.handleRafaleResetOneUsed(w, r, id)
		return
	}

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := path
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	if err := h.engine.DeleteRafaleQuestion(id); err != nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"DELETED": id})
}

// handleRafaleResetOneUsed handles POST /api/rafale/questions/{id}/reset —
// contract §9, feature #197: makes ONE reservoir question available again
// (removes it from the "already used" flag) without touching the reservoir
// itself. Silently succeeds (no-op) if the question was not marked used.
func (h *HTTPServer) handleRafaleResetOneUsed(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	if err := h.engine.MarkRafaleQuestionAvailable(id); err != nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ID": id, "AVAILABLE": true})
}

// handleRafaleResetAllUsed handles POST /api/rafale/questions/reset-all —
// contract §9, feature #197: makes the ENTIRE reservoir available again
// (empties the "already used" flag) without touching the reservoir itself —
// unlike the destructive /reset-select?rafale=true (handleResetSelect),
// which also deletes every reservoir question.
func (h *HTTPServer) handleRafaleResetAllUsed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	n, err := h.engine.ResetAllRafaleUsed()
	if err != nil {
		log.Printf("[HTTP] Rafale reset-all-used failed: %v", err)
		http.Error(w, "Failed to reset used flags", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"RESET": n})
}

// handleRafalePool returns the pool count (available/used/total) for a
// category/difficulty filter — contract §9, feeding the pre-round alert
// (§7.2: blocking when available==0, warning when short of the estimated
// need, neutral otherwise).
//
// ?category=X (singular, v8.0.0 bugfix, 2026-08-29): a RAFALE round now
// filters on exactly one category, same as every other question type's
// CATEGORY field — RAFALE_CATEGORIES (multi-select, ?categories=A,B) was
// removed. See contracts/CHANGELOG.md.
func (h *HTTPServer) handleRafalePool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	category := strings.TrimSpace(r.URL.Query().Get("category"))
	if category == "" {
		http.Error(w, "category required", http.StatusBadRequest)
		return
	}

	difficulty, err := strconv.Atoi(r.URL.Query().Get("difficulty"))
	if err != nil {
		http.Error(w, "Invalid difficulty", http.StatusBadRequest)
		return
	}

	available, used, total := h.engine.CountRafalePool(category, difficulty)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"AVAILABLE": available,
		"USED":      used,
		"TOTAL":     total,
	})
}

// handleBackupSelect creates a selective backup based on query parameters
// Query params: questions=true, teams=true, bumpers=true, history=true, medias=true, ambiance=true
func (h *HTTPServer) handleBackupSelect(w http.ResponseWriter, r *http.Request) {
	includeQuestions := r.URL.Query().Get("questions") == "true"
	includeTeams := r.URL.Query().Get("teams") == "true"
	includeBumpers := r.URL.Query().Get("bumpers") == "true"
	includeHistory := r.URL.Query().Get("history") == "true"
	includeMedias := r.URL.Query().Get("medias") == "true"
	// #152: dedicated flag for game-config.json (default delay + neon
	// effect) — code-reviewer flagged its "history" attachment (#150) as
	// semantically incorrect during #150's own review; this is that
	// correction, with its own "Configuration Ambiance" checkbox in
	// BackupPage.jsx. game_state.json (quiz metadata, #141) is NOT part of
	// this flag — it stays anchored to "history" (a session's identity, not
	// an ambiance/visual setting), unchanged from before #152.
	includeAmbiance := r.URL.Query().Get("ambiance") == "true"
	// RAFALE reservoir (v8.0.0, #197, contract §10) — dedicated flag,
	// deliberately not piggybacked on "questions": the reservoir is a
	// separate global pool, not part of the per-quiz Questions directory.
	includeRafale := r.URL.Query().Get("rafale") == "true"

	// If nothing selected, include everything
	if !includeQuestions && !includeTeams && !includeBumpers && !includeHistory && !includeMedias && !includeAmbiance && !includeRafale {
		includeQuestions = true
		includeTeams = true
		includeBumpers = true
		includeHistory = true
		includeMedias = true
		includeAmbiance = true
		includeRafale = true
	}

	log.Printf("[HTTP] Selective backup: questions=%v, teams=%v, bumpers=%v, history=%v, medias=%v, ambiance=%v, rafale=%v",
		includeQuestions, includeTeams, includeBumpers, includeHistory, includeMedias, includeAmbiance, includeRafale)

	// Set headers for TAR download
	cfg := config.Get()
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("buzzcontrol-backup-v%s_%s.tar", cfg.Version, timestamp)
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

	// Add game-config.json (#150) — #152: dedicated "ambiance" flag, no
	// longer piggybacked on "history". game-config.json (default delay +
	// neon effect) is a game-session-scoped VISUAL/AMBIANCE settings file;
	// tying its inclusion to "history" (score/event log) was a semantic
	// mismatch flagged by code-reviewer during #150's own review — a user
	// wanting to keep history but reset the room's look (or vice versa)
	// couldn't express that. BackupPage.jsx now has its own "Configuration
	// Ambiance" checkbox for exactly this. /fs-backup (full backup) already
	// includes it unconditionally (it just archives dataDir wholesale), so
	// this only affects the *selective* backup endpoint.
	if includeAmbiance {
		gameConfigPath := config.GameConfigPath()
		if _, err := os.Stat(gameConfigPath); err == nil {
			h.addFileToTAR(tw, gameConfigPath, "config/game-config.json")
		}
	}

	// Add game_state.json (#141) — quiz metadata (name/theme/notes/
	// populations/difficulties/language/objectives/hidden fields), the
	// virtual player limit, and (v6.5.2, #119) the saved entracte panel
	// config. Stays anchored to "history", unlike game-config.json above
	// (#152, moved to "ambiance"): this is a session's identity/settings
	// (quiz name, theme...), conceptually closer to the history/event log
	// than to a visual "ambiance" preset — no dedicated UI flag of its own.
	if includeHistory {
		statePath := filepath.Join(configDir, "game_state.json")
		if _, err := os.Stat(statePath); err == nil {
			h.addFileToTAR(tw, statePath, "config/game_state.json")
		}
	}

	// Add medias (backgrounds + categories + entracte)
	if includeMedias {
		backgroundsDir := filepath.Join(filesDir, "backgrounds")
		if _, err := os.Stat(backgroundsDir); err == nil {
			h.addDirToTAR(tw, backgroundsDir, "files/backgrounds")
		}
		categoriesDir := filepath.Join(filesDir, "categories")
		if _, err := os.Stat(categoriesDir); err == nil {
			h.addDirToTAR(tw, categoriesDir, "files/categories")
		}
		// v6.5.2, #119 — added explicitly, per the plan's risk table: the
		// default-question-image (files/ root) and new-game-backgrounds/ are
		// ALREADY missing from this list (#152), a dedicated directory here
		// avoids reproducing that same gap for the entracte panel image.
		entracteDir := filepath.Join(filesDir, "entracte")
		if _, err := os.Stat(entracteDir); err == nil {
			h.addDirToTAR(tw, entracteDir, "files/entracte")
		}
	}

	// Add RAFALE reservoir (v8.0.0, #197) — the question bank
	// (files/rafale/reservoir.json) and the "already used this game" flags
	// (config/rafale_used.json). Without this, the reservoir would be
	// silently excluded from a selective backup while still riding along
	// in a full /fs-backup archive (contract §10) — a discreet, easy-to-miss
	// gap this dedicated flag closes.
	if includeRafale {
		rafaleDir := filepath.Join(filesDir, "rafale")
		if _, err := os.Stat(rafaleDir); err == nil {
			h.addDirToTAR(tw, rafaleDir, "files/rafale")
		}
		rafaleUsedPath := filepath.Join(configDir, "rafale_used.json")
		if _, err := os.Stat(rafaleUsedPath); err == nil {
			h.addFileToTAR(tw, rafaleUsedPath, "config/rafale_used.json")
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
// Query params: questions=true, teams=true, bumpers=true, history=true, medias=true, ambiance=true, all=true
func (h *HTTPServer) handleResetSelect(w http.ResponseWriter, r *http.Request) {
	resetQuestions := r.URL.Query().Get("questions") == "true"
	resetTeams := r.URL.Query().Get("teams") == "true"
	resetBumpers := r.URL.Query().Get("bumpers") == "true"
	resetHistory := r.URL.Query().Get("history") == "true"
	resetMedias := r.URL.Query().Get("medias") == "true"
	// #152: dedicated flag, mirrors handleBackupSelect's includeAmbiance —
	// see its doc comment for the full rationale (code-reviewer's #150
	// finding). game_state.json stays on resetHistory, unchanged.
	resetAmbiance := r.URL.Query().Get("ambiance") == "true"
	// RAFALE reservoir (v8.0.0, #197) — mirrors handleBackupSelect's
	// includeRafale, see its doc comment for the rationale.
	resetRafale := r.URL.Query().Get("rafale") == "true"
	resetAll := r.URL.Query().Get("all") == "true"

	// "all" means reset everything
	if resetAll {
		resetQuestions = true
		resetTeams = true
		resetBumpers = true
		resetHistory = true
		resetMedias = true
		resetAmbiance = true
		resetRafale = true
	}

	log.Printf("[HTTP] Selective reset: questions=%v, teams=%v, bumpers=%v, history=%v, medias=%v, ambiance=%v, rafale=%v",
		resetQuestions, resetTeams, resetBumpers, resetHistory, resetMedias, resetAmbiance, resetRafale)

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

		// Reset game state (#141) — quiz metadata + virtual player limit
		// (+ v6.5.2 #119: the saved entracte panel config). Stays anchored
		// to "history" (#152 only moved game-config.json to "ambiance" —
		// see below — game_state.json is unchanged), same clear-in-memory-
		// then-remove-file division of labor as history.json above (Engine
		// has no direct file-removal responsibility of its own —
		// ClearQuizMeta only touches memory).
		h.engine.ClearQuizMeta()
		statePath := filepath.Join(configDir, "game_state.json")
		os.Remove(statePath)
		log.Printf("[HTTP] Reset: Game state cleared")
	}

	// Reset game settings (default delay + neon effect, #150) — #152:
	// dedicated "ambiance" flag, no longer piggybacked on "history" (see
	// handleBackupSelect's includeAmbiance doc comment for the full
	// rationale — code-reviewer flagged the "history" attachment as a
	// semantic mismatch during #150's own review). Writes fresh defaults
	// rather than just deleting the file: GetGameSettings()'s singleton
	// would not otherwise pick up the deletion until a full process restart
	// (once.Do), so the in-memory instance must be explicitly replaced for
	// GET /game-config.json to reflect the reset immediately.
	if resetAmbiance {
		defaultGS := &config.GameSettings{}
		config.ApplyGameSettingsDefaults(defaultGS)
		if err := config.SaveGameSettings(defaultGS); err == nil {
			config.SetGameSettingsInstance(defaultGS)
			result["ambiance"] = true
			log.Printf("[HTTP] Reset: Game config (ambiance) cleared")
			if h.OnConfigUpdate != nil {
				h.OnConfigUpdate()
			}
		}
	}

	// Reset medias (backgrounds + categories + entracte)
	if resetMedias {
		mediasOk := false
		backgroundsDir := filepath.Join(filesDir, "backgrounds")
		if err := os.RemoveAll(backgroundsDir); err == nil {
			os.MkdirAll(backgroundsDir, 0755)
			h.engine.ClearBackgrounds()
			mediasOk = true
			log.Printf("[HTTP] Reset: Backgrounds cleared")
		}
		categoriesDir := filepath.Join(filesDir, "categories")
		if err := os.RemoveAll(categoriesDir); err == nil {
			os.MkdirAll(categoriesDir, 0755)
			mediasOk = true
			log.Printf("[HTTP] Reset: Categories cleared")
		}
		// v6.5.2, #119 — added explicitly alongside backgrounds/categories,
		// same rationale as the backup archive above.
		entracteDir := filepath.Join(filesDir, "entracte")
		if err := os.RemoveAll(entracteDir); err == nil {
			os.MkdirAll(entracteDir, 0755)
			mediasOk = true
			log.Printf("[HTTP] Reset: Entracte image cleared")
			if h.OnConfigUpdate != nil {
				h.OnConfigUpdate()
			}
		}
		if mediasOk {
			result["medias"] = true
		}
	}

	// Reset RAFALE reservoir + "already used" flags (v8.0.0, #197, contract
	// §10) — purges both the question bank and the used-flag file, mirroring
	// handleBackupSelect's includeRafale.
	if resetRafale {
		rafaleDir := filepath.Join(filesDir, "rafale")
		if err := os.RemoveAll(rafaleDir); err == nil {
			os.MkdirAll(rafaleDir, 0755)
			h.engine.ClearRafaleReservoir()
			result["rafale"] = true
			log.Printf("[HTTP] Reset: Rafale reservoir cleared")
		}
		rafaleUsedPath := filepath.Join(configDir, "rafale_used.json")
		os.Remove(rafaleUsedPath)
		h.engine.ClearRafaleUsed()
		log.Printf("[HTTP] Reset: Rafale used-flags cleared")
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
		case strings.HasPrefix(tarPath, "files/categories/"):
			if detected["categories"] {
				targetPath = filepath.Join(h.dataDir, tarPath)
				allowed = true
			}
		case strings.HasPrefix(tarPath, "files/entracte/"):
			if detected["entracte"] {
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
		case tarPath == "config/game-config.json":
			if detected["gameConfig"] {
				targetPath = filepath.Join(configDir, "game-config.json")
				allowed = true
			}
		case tarPath == "config/game_state.json":
			if detected["gameState"] {
				targetPath = filepath.Join(configDir, "game_state.json")
				allowed = true
			}
		case strings.HasPrefix(tarPath, "files/rafale/"):
			if detected["rafale"] {
				targetPath = filepath.Join(h.dataDir, tarPath)
				allowed = true
			}
		case tarPath == "config/rafale_used.json":
			if detected["rafale"] {
				targetPath = filepath.Join(configDir, "rafale_used.json")
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

	if detected["entracte"] {
		restoredMap["entracte"] = true
		// No engine-side file list to reload (unlike backgrounds) — the
		// image's existence is checked on demand (HasCustomEntracteImage).
		// OnConfigUpdate refreshes GameState.ENTRACTE_CONFIG.IMAGE_IS_CUSTOM
		// and rebroadcasts, so connected clients see the restored image
		// immediately.
		if h.OnConfigUpdate != nil {
			h.OnConfigUpdate()
		}
		log.Printf("[HTTP] Restore: Entracte image restored")
	}

	if detected["gameConfig"] {
		// #150 — reload from the just-extracted file (same path
		// GameConfigPath() already resolves to, set once at startup) and
		// refresh the in-memory singleton so GET /game-config.json and the
		// WS neon_effect payload reflect the restored values immediately,
		// without a process restart.
		if gs, err := config.LoadGameSettings(config.GameConfigPath()); err == nil {
			config.SetGameSettingsInstance(gs)
			restoredMap["gameConfig"] = true
			log.Printf("[HTTP] Restore: Game config (default delay + neon effect) restored")
			if h.OnConfigUpdate != nil {
				h.OnConfigUpdate()
			}
		} else {
			log.Printf("[HTTP] Restore: Game config extracted but could not be reloaded: %v", err)
		}
	}

	if detected["gameState"] {
		// #141 — LoadState reads from e.statePath (set once at startup to
		// the same configDir this handler just extracted into) and updates
		// the engine's in-memory GameState directly, immediately reflected
		// by GetState() — no separate "set instance" step needed, unlike
		// the config package's Get()/SetInstance() singleton pattern.
		if err := h.engine.LoadState(); err == nil {
			restoredMap["gameState"] = true
			log.Printf("[HTTP] Restore: Game state (quiz metadata) restored")
		} else {
			log.Printf("[HTTP] Restore: Game state extracted but could not be reloaded: %v", err)
		}
	}

	if detected["rafale"] {
		// #197 — reload both stores from the just-extracted files (paths set
		// once at startup, same ones this handler just wrote into).
		if err := h.engine.LoadRafale(); err == nil {
			log.Printf("[HTTP] Restore: Rafale reservoir loaded into engine")
		} else {
			log.Printf("[HTTP] Restore: Rafale reservoir extracted but could not be reloaded: %v", err)
		}
		if err := h.engine.LoadRafaleUsed(); err == nil {
			log.Printf("[HTTP] Restore: Rafale used-flags loaded into engine")
		} else {
			log.Printf("[HTTP] Restore: Rafale used-flags extracted but could not be reloaded: %v", err)
		}
		restoredMap["rafale"] = true
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
		"categories":  false,
		"entracte":    false, // #119 — files/entracte/ (ENTRACTE panel image, v6.5.2)
		"gameConfig":  false, // #150 — game-config.json (default delay + neon effect)
		"gameState":   false, // #141 — game_state.json (quiz metadata)
		"rafale":      false, // #197 (v8.0.0) — files/rafale/ + config/rafale_used.json
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
		case strings.HasPrefix(tarPath, "files/categories/"):
			detected["categories"] = true
		case strings.HasPrefix(tarPath, "files/entracte/"):
			detected["entracte"] = true
		case tarPath == "config/teams.json" || tarPath == "teams.json":
			detected["teams"] = true
		case tarPath == "config/bumpers.json" || tarPath == "bumpers.json":
			detected["bumpers"] = true
		case tarPath == "config/history.json" || tarPath == "history.json":
			detected["history"] = true
		case tarPath == "config/game-config.json":
			detected["gameConfig"] = true
		case tarPath == "config/game_state.json":
			detected["gameState"] = true
		case strings.HasPrefix(tarPath, "files/rafale/"), tarPath == "config/rafale_used.json":
			detected["rafale"] = true
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

// handleWebSocket is the legacy /ws endpoint — alias for /ws/admin for retro-compat.
func (h *HTTPServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	h.wsHub.HandleConnectionWithType(w, r, ClientTypeAdmin)
}

// handleWebSocketAdmin handles /ws/admin — administration interface.
func (h *HTTPServer) handleWebSocketAdmin(w http.ResponseWriter, r *http.Request) {
	h.wsHub.HandleConnectionWithType(w, r, ClientTypeAdmin)
}

// handleWebSocketTV handles /ws/tv — TV/scoreboard display.
func (h *HTTPServer) handleWebSocketTV(w http.ResponseWriter, r *http.Request) {
	h.wsHub.HandleConnectionWithType(w, r, ClientTypeTV)
}

// handleWebSocketPlayer handles /ws/player — virtual player (VPlayer/enrollment).
func (h *HTTPServer) handleWebSocketPlayer(w http.ResponseWriter, r *http.Request) {
	h.wsHub.HandleConnectionWithType(w, r, ClientTypeVPlayer)
}

// handleWebSocketAnim handles /ws/anim — interface animateur (v6.2.0, #155).
func (h *HTTPServer) handleWebSocketAnim(w http.ResponseWriter, r *http.Request) {
	h.wsHub.HandleConnectionWithType(w, r, ClientTypeAnim)
}

func (h *HTTPServer) handleBuzzerWebSocket(w http.ResponseWriter, r *http.Request) {
	h.buzzerHub.HandleConnection(w, r)
}

func (h *HTTPServer) handleLogsWebSocket(w http.ResponseWriter, r *http.Request) {
	h.logsHub.HandleConnection(w, r)
}

// Buzzer API handlers (WiFi config)

func (h *HTTPServer) handleAPIBuzzers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := h.engine.GetTeamsAndBumpers()
	type BuzzerInfo struct {
		MAC      string `json:"mac"`
		Name     string `json:"name"`
		Team     string `json:"team"`
		Protocol string `json:"protocol"`
		IP       string `json:"ip"`
		Version  string `json:"version"`
		Status   string `json:"status"`
	}

	var buzzers []BuzzerInfo
	for mac, bumper := range data.Bumpers {
		proto := bumper.Protocol
		if proto == "" {
			proto = "TCP"
		}
		buzzers = append(buzzers, BuzzerInfo{
			MAC:      mac,
			Name:     bumper.Name,
			Team:     bumper.Team,
			Protocol: proto,
			IP:       bumper.IP,
			Version:  bumper.Version,
			Status:   bumper.Status,
		})
	}

	if buzzers == nil {
		buzzers = []BuzzerInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buzzers)
}

// handleAPIBuzzerRouter routes /api/buzzer/{mac}/... to the appropriate handler.
// Supported sub-paths: /status (GET), /update (POST).
func (h *HTTPServer) handleAPIBuzzerRouter(w http.ResponseWriter, r *http.Request) {
	// Extract the portion after /api/buzzer/
	path := strings.TrimPrefix(r.URL.Path, "/api/buzzer/")
	path = strings.TrimSuffix(path, "/")

	// Determine MAC and optional suffix
	// Pattern: /api/buzzer/{mac} or /api/buzzer/{mac}/status or /api/buzzer/{mac}/update
	parts := strings.SplitN(path, "/", 2)
	mac := parts[0]
	suffix := ""
	if len(parts) > 1 {
		suffix = parts[1]
	}

	if mac == "" {
		http.Error(w, "MAC address required", http.StatusBadRequest)
		return
	}

	switch suffix {
	case "update":
		h.handleAPIBuzzerUpdate(mac, w, r)
	default:
		// Default to status handler
		h.handleAPIBuzzerStatus(mac, w, r)
	}
}

// handleAPIBuzzerStatus returns status information for a specific buzzer by MAC.
func (h *HTTPServer) handleAPIBuzzerStatus(mac string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bumper := h.engine.GetBumper(mac)
	if bumper == nil {
		http.Error(w, "Buzzer not found", http.StatusNotFound)
		return
	}

	proto := bumper.Protocol
	if proto == "" {
		proto = "TCP"
	}

	result := map[string]interface{}{
		"mac":              mac,
		"name":             bumper.Name,
		"team":             bumper.Team,
		"protocol":         proto,
		"ip":               bumper.IP,
		"version":          bumper.Version,
		"status":           bumper.Status,
		"score":            bumper.Score,
		"firmware_version": bumper.FirmwareVersion,
		"is_outdated":      bumper.IsOutdated,
		"ota_status":       bumper.OTAStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// WiFi defaults API handlers

func (h *HTTPServer) handleAPIWiFiDefaults(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := config.Get()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg.WiFiDefaults)

	case http.MethodPost:
		var defaults config.WiFiDefaultsConfig
		if err := json.NewDecoder(r.Body).Decode(&defaults); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Validate SSID (max 32 chars)
		if len(defaults.SSID) > 32 {
			http.Error(w, "SSID must be 32 characters or less", http.StatusBadRequest)
			return
		}

		// Validate password (min 8 chars if not empty)
		if defaults.Password != "" && len(defaults.Password) < 8 {
			http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
			return
		}

		// Validate IP address (if not empty)
		if defaults.ServerIP != "" && net.ParseIP(defaults.ServerIP) == nil {
			http.Error(w, "Invalid IP address", http.StatusBadRequest)
			return
		}

		// Validate port (1-65535)
		if defaults.ServerPort < 1 || defaults.ServerPort > 65535 {
			http.Error(w, "Port must be between 1 and 65535", http.StatusBadRequest)
			return
		}

		// Update config
		cfg := config.Get()
		cfg.WiFiDefaults = defaults

		// Save to disk atomically via config.Save() (bugfix #143): this used to
		// be a direct os.WriteFile("config.json", ...) that (a) hardcoded the
		// path instead of going through the config package's path indirection,
		// (b) was not atomic (no temp file + rename, unlike config.Save()), and
		// (c) serialized the whole struct — secrets included — the same way
		// config.Save() does, so routing through it changes no behavior here.
		if err := config.Save(cfg); err != nil {
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

		config.SetInstance(cfg)
		LogInfo(game.LogComponentHTTP, "WiFi defaults updated: ssid=%s, server_ip=%s, server_port=%d",
			defaults.SSID, defaults.ServerIP, defaults.ServerPort)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(defaults)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAPIBuzzerWifiConfig broadcasts current WiFi defaults config to all connected buzzers.
// POST /api/buzzer/wifi-config
func (h *HTTPServer) handleAPIBuzzerWifiConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	count := 0
	if h.OnBuzzerWifiConfig != nil {
		count = h.OnBuzzerWifiConfig()
	}

	LogInfo(game.LogComponentHTTP, "POST /api/buzzer/wifi-config: broadcasted to %d connected buzzers", count)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"count":  count,
	})
}

// Auto-update API handlers

func (h *HTTPServer) handleAPIUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.updater.HandleGetUpdates(w, r)
}

func (h *HTTPServer) handleAPIUpdatesCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.updater.HandleCheckUpdates(w, r)
}

func (h *HTTPServer) handleAPIUpdatesDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.updater.HandleDownloadUpdate(w, r)
}

func (h *HTTPServer) handleAPIUpdatesApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.updater.HandleApplyUpdate(w, r)
}

// Default question image API (v3.2.2)
// GET    /api/config/default-image → serves image binary (custom data/files/ first, embedded SVG fallback)
// POST   /api/config/default-image → multipart upload (field "file"), saves to data/files/default-question-image.<ext>
// DELETE /api/config/default-image → removes custom image (reverts to embedded fallback)

const defaultQuestionImageBaseName = "default-question-image"

// defaultQuestionImageExts lists supported extensions in search priority order.
var defaultQuestionImageExts = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg"}

// getCustomDefaultQuestionImagePath returns the filesystem path of the custom default question image,
// or "" if none has been uploaded.
func (h *HTTPServer) getCustomDefaultQuestionImagePath() string {
	filesDir := filepath.Join(h.dataDir, "files")
	for _, ext := range defaultQuestionImageExts {
		candidate := filepath.Join(filesDir, defaultQuestionImageBaseName+ext)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// HasCustomDefaultQuestionImage returns true if a custom image has been uploaded.
func (h *HTTPServer) HasCustomDefaultQuestionImage() bool {
	return h.getCustomDefaultQuestionImagePath() != ""
}

func (h *HTTPServer) handleAPIDefaultQuestionImage(w http.ResponseWriter, r *http.Request) {
	filesDir := filepath.Join(h.dataDir, "files")

	switch r.Method {
	case http.MethodGet:
		// Serve custom image if it exists, otherwise serve embedded fallback
		customPath := h.getCustomDefaultQuestionImagePath()
		if customPath != "" {
			ext := strings.ToLower(filepath.Ext(customPath))
			contentType := "image/jpeg"
			switch ext {
			case ".png":
				contentType = "image/png"
			case ".gif":
				contentType = "image/gif"
			case ".webp":
				contentType = "image/webp"
			case ".svg":
				contentType = "image/svg+xml"
			}
			w.Header().Set("Content-Type", contentType)
			http.ServeFile(w, r, customPath)
			return
		}
		// Serve embedded fallback SVG
		if len(h.defaultQuestionImageAsset) > 0 {
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Header().Set("Cache-Control", "public, max-age=3600")
			w.Write(h.defaultQuestionImageAsset)
			return
		}
		http.NotFound(w, r)

	case http.MethodPost:
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "No file uploaded: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext == "" {
			ext = ".png"
		}
		allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".svg": true}
		if !allowed[ext] {
			http.Error(w, "Unsupported file type: "+ext, http.StatusBadRequest)
			return
		}

		// Remove any existing custom default question image (all extensions)
		for _, oldExt := range defaultQuestionImageExts {
			os.Remove(filepath.Join(filesDir, defaultQuestionImageBaseName+oldExt))
		}

		os.MkdirAll(filesDir, 0755)
		destPath := filepath.Join(filesDir, defaultQuestionImageBaseName+ext)
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

		LogInfo(game.LogComponentHTTP, "Default question image uploaded: %s", destPath)

		if h.OnConfigUpdate != nil {
			h.OnConfigUpdate()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":      "/api/config/default-image",
			"is_custom": true,
		})

	case http.MethodDelete:
		removed := false
		for _, ext := range defaultQuestionImageExts {
			candidate := filepath.Join(filesDir, defaultQuestionImageBaseName+ext)
			if err := os.Remove(candidate); err == nil {
				removed = true
				LogInfo(game.LogComponentHTTP, "Default question image deleted: %s", candidate)
			}
		}
		if !removed {
			http.Error(w, "No custom default question image found", http.StatusNotFound)
			return
		}

		if h.OnConfigUpdate != nil {
			h.OnConfigUpdate()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":      "/api/config/default-image",
			"is_custom": false,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ENTRACTE panel image API (v6.5.2, #119, contract http-endpoints.md
// §"Mode ENTRACTE") — single optional background image for the ENTRACTE
// panel. Calqué à l'identique sur handleAPIDefaultQuestionImage above (the
// project's one single-image pattern), with two deliberate differences:
//   - no embedded fallback asset — GET 404s if no image was ever uploaded,
//     the panel simply renders without a background image;
//   - stored in its OWN directory, data/files/entracte/, not the shared
//     data/files/ root. The selective-backup "medias" flag only archives
//     files/backgrounds/ and files/categories/ (handleSelectiveBackup,
//     below) — the default-question-image at the files/ root is ALREADY
//     missing from that list (a pre-existing gap, #152), and new-game-
//     backgrounds/ too. A dedicated directory, added explicitly to the
//     "medias" backup/reset code, avoids reproducing that same hole for
//     entracte's image (plan risk table "Image de config perdue à la
//     restauration").
//
// GET    /api/game/entracte-image → serves the image binary, 404 if none
// POST   /api/game/entracte-image → multipart upload (field "file"), replaces any existing image regardless of its extension
// DELETE /api/game/entracte-image → removes the image — panel falls back to no background
//
// Renamed 2026-08-20 (C1) from /api/config/entracte-image: the image
// belongs to the game/session, not to server config — /api/config/ became
// misleading once the rest of the entracte settings moved to game_state.json.
// Free rename: this endpoint never reached production (contract
// http-endpoints.md §"Mode ENTRACTE").
const entracteImageBaseName = "entracte-image"

// entracteImageExts lists supported extensions in search priority order —
// same set handleAPIDefaultQuestionImage accepts.
var entracteImageExts = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg"}

func (h *HTTPServer) entracteImageDir() string {
	return filepath.Join(h.dataDir, "files", "entracte")
}

// getCustomEntracteImagePath returns the path to the uploaded ENTRACTE
// panel image, or "" if none exists.
func (h *HTTPServer) getCustomEntracteImagePath() string {
	dir := h.entracteImageDir()
	for _, ext := range entracteImageExts {
		candidate := filepath.Join(dir, entracteImageBaseName+ext)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// HasCustomEntracteImage returns true if an ENTRACTE panel image has been
// uploaded — the source of truth for GameState.ENTRACTE_CONFIG.IMAGE_IS_CUSTOM
// (cmd/server/main.go pushes this into the engine on every config update).
func (h *HTTPServer) HasCustomEntracteImage() bool {
	return h.getCustomEntracteImagePath() != ""
}

func (h *HTTPServer) handleAPIEntracteImage(w http.ResponseWriter, r *http.Request) {
	dir := h.entracteImageDir()

	switch r.Method {
	case http.MethodGet:
		customPath := h.getCustomEntracteImagePath()
		if customPath == "" {
			http.NotFound(w, r)
			return
		}
		ext := strings.ToLower(filepath.Ext(customPath))
		contentType := "image/jpeg"
		switch ext {
		case ".png":
			contentType = "image/png"
		case ".gif":
			contentType = "image/gif"
		case ".webp":
			contentType = "image/webp"
		case ".svg":
			contentType = "image/svg+xml"
		}
		w.Header().Set("Content-Type", contentType)
		http.ServeFile(w, r, customPath)

	case http.MethodPost:
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "No file uploaded: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext == "" {
			ext = ".png"
		}
		allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".svg": true}
		if !allowed[ext] {
			http.Error(w, "Unsupported file type: "+ext, http.StatusBadRequest)
			return
		}

		os.MkdirAll(dir, 0755)

		// Remove any existing entracte image (all extensions) — POST replaces
		// the previous image regardless of its own extension.
		for _, oldExt := range entracteImageExts {
			os.Remove(filepath.Join(dir, entracteImageBaseName+oldExt))
		}

		destPath := filepath.Join(dir, entracteImageBaseName+ext)
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

		LogInfo(game.LogComponentHTTP, "Entracte panel image uploaded: %s", destPath)

		if h.OnConfigUpdate != nil {
			h.OnConfigUpdate()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":      "/api/game/entracte-image",
			"is_custom": true,
		})

	case http.MethodDelete:
		removed := false
		for _, ext := range entracteImageExts {
			candidate := filepath.Join(dir, entracteImageBaseName+ext)
			if err := os.Remove(candidate); err == nil {
				removed = true
				LogInfo(game.LogComponentHTTP, "Entracte panel image deleted: %s", candidate)
			}
		}
		if !removed {
			http.Error(w, "No entracte image found", http.StatusNotFound)
			return
		}

		if h.OnConfigUpdate != nil {
			h.OnConfigUpdate()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":      "/api/game/entracte-image",
			"is_custom": false,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// NEW_GAME background images API (v4.0.4)
// Mirrors /background handler — multi-image, stored in data/files/new-game-backgrounds/.
//
// POST   /new-game-backgrounds              → upload image (multipart "file")
// PUT    /new-game-backgrounds              → update config (order, duration, opacity)
// DELETE /new-game-backgrounds              → delete all images
// DELETE /new-game-backgrounds?file=xxx     → delete specific image
// GET    /files/new-game-backgrounds/{name} → served automatically by /files/ handler

func (h *HTTPServer) handleNewGameBackground(w http.ResponseWriter, r *http.Request) {
	LogDebug(game.LogComponentHTTP, "NewGameBackground request: method=%s, content-type=%s", r.Method, r.Header.Get("Content-Type"))
	bgDir := filepath.Join(h.dataDir, "files", "new-game-backgrounds")

	switch r.Method {
	case "POST":
		// Parse multipart form (max 10MB)
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			LogError(game.LogComponentHTTP, "Failed to parse multipart form: %v", err)
			http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			LogError(game.LogComponentHTTP, "FormFile error: %v", err)
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
		fileName := fmt.Sprintf("ng_%d%s", timestamp, ext)
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

		bgPath := "/files/new-game-backgrounds/" + fileName
		LogInfo(game.LogComponentHTTP, "NEW_GAME background image uploaded: %s", destPath)

		if h.OnNewGameBackgroundChange != nil {
			h.OnNewGameBackgroundChange("reload")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok", "path": "` + bgPath + `"}`))

	case "PUT":
		// Update new-game-backgrounds config (order, duration, opacity)
		var backgrounds []game.Background
		if err := json.NewDecoder(r.Body).Decode(&backgrounds); err != nil {
			LogError(game.LogComponentHTTP, "Failed to decode new-game-backgrounds config: %v", err)
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		h.engine.SetNewGameBackgrounds(backgrounds)
		log.Printf("[HTTP] New-game-backgrounds config updated: %d items", len(backgrounds))

		if h.OnNewGameBackgroundChange != nil {
			h.OnNewGameBackgroundChange("save")
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
				log.Printf("[HTTP] Failed to delete new-game-background: %v", err)
			} else {
				log.Printf("[HTTP] New-game-background deleted: %s", filePath)
			}
		} else {
			// Delete all new-game-backgrounds
			os.RemoveAll(bgDir)
			log.Printf("[HTTP] All new-game-backgrounds removed")
		}

		if h.OnNewGameBackgroundChange != nil {
			h.OnNewGameBackgroundChange("reload")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok"}`))

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
