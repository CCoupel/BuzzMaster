package main

import (
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// App holds all server components
type App struct {
	config     *config.Config
	engine     *game.Engine
	tcpServer  *server.TCPServer
	udpBcast   *server.UDPBroadcaster
	httpServer *server.HTTPServer
	wsHub      *server.WebSocketHub
	mdnsServer *server.MDNSServer
	dnsServer  *server.DNSServer
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("=== BuzzControl Server (Go) ===")

	// Check for Bonjour/mDNS support
	checkBonjourSupport()

	// Load configuration
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Printf("Using default configuration: %v", err)
		cfg = config.Get()
	}
	config.SetInstance(cfg)

	log.Printf("Version: %s", cfg.Version)
	log.Printf("HTTP Port: %d", cfg.Server.HTTPPort)
	log.Printf("TCP Port: %d", cfg.Server.TCPPort)

	// Create app
	app := &App{
		config: cfg,
	}

	// Initialize components
	app.init()

	// Initialize test data (teams, bumpers)
	app.initTestData()

	// Start servers
	if err := app.start(); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}

	log.Println("Server started successfully")
	log.Printf("TCP server (buzzers): port %d", cfg.Server.TCPPort)

	// Display all accessible URLs and open browser
	displayAndOpenURLs(cfg.Server.HTTPPort)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	app.stop()
}

func (a *App) init() {
	// Game engine
	a.engine = game.NewEngine()

	// WebSocket hub
	a.wsHub = server.NewWebSocketHub()

	// TCP server for buzzers
	a.tcpServer = server.NewTCPServer(a.config.Server.TCPPort)

	// UDP broadcaster
	a.udpBcast = server.NewUDPBroadcaster(a.config.Server.TCPPort)

	// HTTP server
	a.httpServer = server.NewHTTPServer(a.config.Server.HTTPPort, a.engine, a.wsHub)

	// Check for React build and set it if exists
	reactDir := filepath.Join(".", "web", "dist")
	if _, err := os.Stat(filepath.Join(reactDir, "index.html")); err == nil {
		log.Println("[HTTP] React build found, serving modern UI")
		a.httpServer.SetReactDir(reactDir)
	} else {
		log.Println("[HTTP] No React build found, using legacy UI")
	}

	// mDNS server (advertise buzzcontrol.local)
	a.mdnsServer = server.NewMDNSServer("buzzcontrol", a.config.Server.HTTPPort, a.config.Server.TCPPort)

	// DNS server (captive portal - redirects all DNS to this server)
	a.dnsServer = server.NewDNSServer(53, nil)

	// Set up callbacks
	a.setupCallbacks()
}

func (a *App) setupCallbacks() {
	// Handle TCP messages from buzzers
	go func() {
		for msg := range a.tcpServer.Incoming {
			a.handleBuzzerMessage(msg)
		}
	}()

	// Handle WebSocket messages from web clients
	go func() {
		for msg := range a.wsHub.Incoming {
			a.handleWebMessage(msg)
		}
	}()

	// Game state changes
	a.engine.OnStateChange = func(phase game.GamePhase) {
		a.broadcastGameState(string(phase))
	}

	// Timer ticks
	a.engine.OnTimerTick = func(currentTime int) {
		a.broadcastTimerUpdate(currentTime)
	}

	// Buzzer press
	a.engine.OnBuzzerPress = func(bumperID, teamID string, pressTime int64, button string) {
		a.broadcastPause(bumperID)
	}

	// HTTP actions
	a.httpServer.OnAction = func(action string, data json.RawMessage) {
		switch action {
		case "CLEAR_GAME":
			a.broadcastReset()
		case "CLEAR_BUZZERS":
			a.broadcastUpdate()
		case "REBOOT", "RESET":
			a.broadcastReset()
		case "RESTORE":
			a.broadcastQuestions()
			a.broadcastUpdate()
		}
	}

	// Question upload broadcast
	a.httpServer.OnQuestionUpload = func() {
		a.broadcastQuestions()
	}

	// Background change handler
	a.httpServer.OnBackgroundChange = func(action string) {
		if action == "save" {
			// Config was updated via PUT, just save
			a.saveBackgroundsConfig()
		} else {
			// Files changed (upload/delete), reload from disk
			a.loadBackgrounds()
			a.saveBackgroundsConfig()
		}
		a.broadcastUpdate()
	}

	// Detect existing backgrounds on startup
	a.loadBackgrounds()

	// Handle client count changes (WebSocket connect/disconnect)
	a.wsHub.OnClientChange = func(adminCount, tvCount int) {
		a.broadcastClientCounts(adminCount, tvCount)
	}
}

func (a *App) loadBackgrounds() {
	filesDir := a.config.Storage.FilesDir
	if filesDir == "" {
		filesDir = "./data/files"
	}
	bgDir := filepath.Join(filesDir, "backgrounds")
	configPath := filepath.Join(bgDir, "backgrounds.json")

	// Try to load existing config
	var savedConfig []game.Background
	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &savedConfig)
	}

	// Build a map of saved configs by path
	savedMap := make(map[string]game.Background)
	for _, bg := range savedConfig {
		savedMap[bg.Path] = bg
	}

	var backgrounds []game.Background

	// Scan backgrounds directory
	entries, err := os.ReadDir(bgDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == "backgrounds.json" {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" {
				bgPath := "/files/backgrounds/" + entry.Name()
				// Check if we have saved config for this file
				if saved, ok := savedMap[bgPath]; ok {
					// Ensure opacity has a default value
					if saved.Opacity == 0 {
						saved.Opacity = 100
					}
					backgrounds = append(backgrounds, saved)
					delete(savedMap, bgPath) // Mark as found
				} else {
					// New file, use defaults
					backgrounds = append(backgrounds, game.Background{
						Path:     bgPath,
						Duration: 10,  // Default 10 seconds
						Opacity:  100, // Default 100% opacity
					})
				}
			}
		}
	}

	// Also check for legacy single background file
	legacyMatches, _ := filepath.Glob(filepath.Join(filesDir, "background.*"))
	for _, match := range legacyMatches {
		ext := filepath.Ext(match)
		bgPath := "/files/background" + ext
		if saved, ok := savedMap[bgPath]; ok {
			if saved.Opacity == 0 {
				saved.Opacity = 100
			}
			backgrounds = append(backgrounds, saved)
		} else {
			backgrounds = append(backgrounds, game.Background{
				Path:     bgPath,
				Duration: 10,
				Opacity:  100,
			})
		}
	}

	a.engine.SetBackgrounds(backgrounds)
	if len(backgrounds) > 0 {
		log.Printf("[App] Loaded %d background(s)", len(backgrounds))
	}
}

func (a *App) saveBackgroundsConfig() {
	filesDir := a.config.Storage.FilesDir
	if filesDir == "" {
		filesDir = "./data/files"
	}
	bgDir := filepath.Join(filesDir, "backgrounds")
	configPath := filepath.Join(bgDir, "backgrounds.json")

	os.MkdirAll(bgDir, 0755)

	backgrounds := a.engine.GetBackgrounds()
	data, err := json.MarshalIndent(backgrounds, "", "  ")
	if err != nil {
		log.Printf("[App] Failed to marshal backgrounds config: %v", err)
		return
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		log.Printf("[App] Failed to save backgrounds config: %v", err)
	} else {
		log.Printf("[App] Saved backgrounds config")
	}
}

func (a *App) start() error {
	// Start WebSocket hub
	go a.wsHub.Run()

	// Start TCP server
	if err := a.tcpServer.Start(); err != nil {
		return err
	}

	// Start UDP broadcaster
	if err := a.udpBcast.Start(); err != nil {
		return err
	}

	// Start HTTP server
	if err := a.httpServer.Start(); err != nil {
		return err
	}

	// Start mDNS server (non-fatal if it fails)
	if err := a.mdnsServer.Start(); err != nil {
		log.Printf("[mDNS] Warning: Failed to start mDNS: %v", err)
	}

	// Start DNS server (non-fatal if it fails - may need admin rights)
	if err := a.dnsServer.Start(); err != nil {
		log.Printf("[DNS] Warning: Failed to start DNS server: %v (may need admin rights)", err)
	}

	// Send initial HELLO
	a.broadcastHello()

	return nil
}

func (a *App) stop() {
	a.dnsServer.Stop()
	a.mdnsServer.Stop()
	a.httpServer.Stop()
	a.tcpServer.Stop()
	a.udpBcast.Stop()
}

// handleBuzzerMessage processes messages from BuzzClick buzzers (TCP)
func (a *App) handleBuzzerMessage(incoming *protocol.IncomingMessage) {
	msg := incoming.Data

	switch msg.Action {
	case protocol.ActionHello:
		a.handleHello(incoming.ClientID, msg)

	case protocol.ActionButton:
		a.handleButton(incoming.ClientID, msg, incoming.Timestamp.UnixMicro())

	case protocol.ActionPong:
		a.handlePong(incoming.ClientID, msg)

	default:
		log.Printf("[App] Unknown buzzer action: %s", msg.Action)
	}
}

// handleWebMessage processes messages from web clients (WebSocket)
func (a *App) handleWebMessage(incoming *protocol.IncomingMessage) {
	msg := incoming.Data

	switch msg.Action {
	case protocol.ActionHello:
		a.broadcastUpdate()
		a.broadcastQuestions()

	case protocol.ActionFull:
		a.handleFullUpdate(msg)

	case protocol.ActionUpdate:
		a.handleUpdate(msg)

	case protocol.ActionPoints:
		a.handlePoints(msg)

	case protocol.ActionReady:
		a.handleReady(msg)

	case protocol.ActionStart:
		a.handleStart(msg)

	case protocol.ActionStop:
		a.engine.Stop()
		a.broadcastStop()

	case protocol.ActionPause:
		a.engine.PauseAll()
		a.broadcastPauseAll()

	case protocol.ActionContinue:
		a.engine.Continue()
		a.broadcastContinue()

	case protocol.ActionReveal:
		answer := a.engine.Reveal()
		a.broadcastReveal(answer)

	case protocol.ActionRAZ:
		a.engine.RAZScores()
		a.broadcastUpdate()

	case protocol.ActionRemote:
		a.handleRemote(msg)

	case protocol.ActionDelete:
		a.handleDelete(msg)

	case protocol.ActionReset:
		a.broadcastReset()

	case protocol.ActionReboot:
		log.Println("[App] Reboot requested from web client")

	case protocol.ActionBumperPoints:
		a.handleBumperPoints(msg)

	case protocol.ActionTeamPoints:
		a.handleTeamPoints(msg)

	case protocol.ActionSetClientType:
		a.handleSetClientType(incoming.ClientID, msg)

	case protocol.ActionReorderQuestions:
		a.handleReorderQuestions(msg)

	default:
		log.Printf("[App] Unknown web action: %s", msg.Action)
	}
}

func (a *App) handleHello(clientID string, msg *protocol.Message) {
	log.Printf("[App] HELLO from buzzer: %s", clientID)

	// Parse payload
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Msg, &payload); err == nil {
		a.engine.UpdateBumper(clientID, payload)
	}

	// Send current state to all
	a.broadcastUpdate()
}

func (a *App) handleButton(clientID string, msg *protocol.Message, timestamp int64) {
	payload, err := msg.ParseButtonPayload()
	if err != nil {
		log.Printf("[App] Failed to parse button payload: %v", err)
		return
	}

	log.Printf("[App] BUTTON from %s: %s", clientID, payload.Button)

	// Process in engine
	a.engine.ProcessButtonPress(clientID, timestamp, payload.Button)

	// Broadcast pause to all
	a.broadcastPause(clientID)
	a.broadcastUpdate()
}

func (a *App) handlePong(clientID string, msg *protocol.Message) {
	log.Printf("[App] PONG from buzzer: %s", clientID)

	if a.engine.IsGamePrepare() {
		a.engine.SetBumperReady(clientID)

		// Check if all ready
		if a.engine.AreAllTeamsReady() {
			a.engine.TransitionToReady()
			a.broadcastReady()
		}

		a.broadcastUpdate()
	}
}

func (a *App) handleFullUpdate(msg *protocol.Message) {
	var data struct {
		Teams   map[string]*game.Team   `json:"teams"`
		Bumpers map[string]*game.Bumper `json:"bumpers"`
	}

	if err := json.Unmarshal(msg.Msg, &data); err != nil {
		log.Printf("[App] Failed to parse FULL update: %v", err)
		return
	}

	a.engine.SetTeams(data.Teams)
	a.engine.SetBumpers(data.Bumpers)
	a.broadcastUpdate()
}

func (a *App) handleUpdate(msg *protocol.Message) {
	// Similar to FULL but partial update
	a.handleFullUpdate(msg)
}

func (a *App) handlePoints(msg *protocol.Message) {
	var payload protocol.PointsPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		log.Printf("[App] Failed to parse POINTS: %v", err)
		return
	}

	bumperScore, teamScore := a.engine.UpdateScore(payload.BumperID, payload.Points)
	log.Printf("[App] Points: bumper=%s, +%d, bumperScore=%d, teamScore=%d",
		payload.BumperID, payload.Points, bumperScore, teamScore)

	a.broadcastUpdate()
}

func (a *App) handleReady(msg *protocol.Message) {
	var payload protocol.ReadyPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		log.Printf("[App] Failed to parse READY: %v", err)
		return
	}

	// TODO: Load question from storage
	var question *game.Question
	if payload.Question != "" {
		question = &game.Question{ID: payload.Question}
	}

	a.engine.Ready(payload.Question, question)

	// Send PING to all buzzers
	a.broadcastPing()
}

func (a *App) handleStart(msg *protocol.Message) {
	var payload protocol.StartPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		payload.Delay = a.config.Game.DefaultDelay
	}

	if payload.Delay <= 0 {
		payload.Delay = a.config.Game.DefaultDelay
	}

	a.engine.Start(payload.Delay)
	a.broadcastStart()
}

func (a *App) handleRemote(msg *protocol.Message) {
	var payload protocol.RemotePayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		return
	}

	a.engine.SetPage(payload.Remote)
	a.broadcastRemote()
}

func (a *App) handleDelete(msg *protocol.Message) {
	var payload protocol.DeletePayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		log.Printf("[App] Failed to parse DELETE: %v", err)
		return
	}

	// Validate ID (must be numeric like ESP32)
	if payload.ID == "" {
		log.Printf("[App] DELETE: empty ID")
		return
	}

	// Delete question directory (like ESP32: deleteDirectory(questionsPath + "/" + ID))
	questionsDir := a.config.Storage.QuestionsDir
	if questionsDir == "" {
		questionsDir = "./data/files/questions"
	}
	questionPath := filepath.Join(questionsDir, payload.ID)

	log.Printf("[App] DELETE: Attempting to delete path: %s", questionPath)

	// Check if path exists before deleting
	if _, err := os.Stat(questionPath); os.IsNotExist(err) {
		log.Printf("[App] DELETE: Path does not exist: %s", questionPath)
	}

	if err := os.RemoveAll(questionPath); err != nil {
		log.Printf("[App] Failed to delete question %s: %v", payload.ID, err)
	} else {
		log.Printf("[App] Deleted question: %s (path: %s)", payload.ID, questionPath)
	}

	// Broadcast updated questions list (like ESP32: sendQuestions())
	a.broadcastQuestions()
}

func (a *App) handleReorderQuestions(msg *protocol.Message) {
	var payload protocol.ReorderQuestionsPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		log.Printf("[App] Failed to parse REORDER_QUESTIONS: %v", err)
		return
	}

	if len(payload.Order) == 0 {
		log.Printf("[App] REORDER_QUESTIONS: empty order")
		return
	}

	questionsDir := a.config.Storage.QuestionsDir
	if questionsDir == "" {
		questionsDir = "./data/files/questions"
	}

	// Update ORDER field in each question.json
	for order, questionID := range payload.Order {
		questionFile := filepath.Join(questionsDir, questionID, "question.json")

		// Read existing question
		data, err := os.ReadFile(questionFile)
		if err != nil {
			log.Printf("[App] REORDER: Failed to read question %s: %v", questionID, err)
			continue
		}

		var question map[string]interface{}
		if err := json.Unmarshal(data, &question); err != nil {
			log.Printf("[App] REORDER: Failed to parse question %s: %v", questionID, err)
			continue
		}

		// Update ORDER field
		question["ORDER"] = order

		// Write back
		newData, err := json.MarshalIndent(question, "", "  ")
		if err != nil {
			log.Printf("[App] REORDER: Failed to marshal question %s: %v", questionID, err)
			continue
		}

		if err := os.WriteFile(questionFile, newData, 0644); err != nil {
			log.Printf("[App] REORDER: Failed to write question %s: %v", questionID, err)
			continue
		}
	}

	log.Printf("[App] Reordered %d questions", len(payload.Order))

	// Broadcast updated questions list
	a.broadcastQuestions()
}

func (a *App) handleBumperPoints(msg *protocol.Message) {
	var payload protocol.BumperPointsPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		log.Printf("[App] Failed to parse BUMPER_POINTS: %v", err)
		return
	}

	newScore := a.engine.UpdateBumperScore(payload.ID, payload.Points)
	log.Printf("[App] Bumper points: id=%s, points=%+d, newScore=%d",
		payload.ID, payload.Points, newScore)

	a.broadcastUpdate()
}

func (a *App) handleTeamPoints(msg *protocol.Message) {
	var payload protocol.TeamPointsPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		log.Printf("[App] Failed to parse TEAM_POINTS: %v", err)
		return
	}

	newScore := a.engine.UpdateTeamScore(payload.Team, payload.Points)
	log.Printf("[App] Team points: team=%s, points=%+d, newScore=%d",
		payload.Team, payload.Points, newScore)

	a.broadcastUpdate()
}

func (a *App) handleSetClientType(clientID string, msg *protocol.Message) {
	var payload protocol.SetClientTypePayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		log.Printf("[App] Failed to parse SET_CLIENT_TYPE: %v", err)
		return
	}

	// Map string type to ClientType
	var clientType server.ClientType
	switch payload.Type {
	case "tv":
		clientType = server.ClientTypeTV
	default:
		clientType = server.ClientTypeAdmin
	}

	a.wsHub.SetClientType(clientID, clientType)

	// Broadcast updated counts
	adminCount, tvCount := a.wsHub.GetClientCounts()
	a.broadcastClientCounts(adminCount, tvCount)
}

// Broadcast methods
func (a *App) broadcast(action string, data json.RawMessage, viaTCP bool) {
	msg, _ := protocol.NewMessage(action, nil)
	msg.Msg = data

	// WebSocket
	a.wsHub.Broadcast(msg)

	// UDP Broadcast (for buzzers)
	if viaTCP {
		a.udpBcast.Broadcast(msg)
	}
}

func (a *App) broadcastHello() {
	msg, _ := protocol.NewMessage(protocol.ActionHello, map[string]interface{}{})
	a.wsHub.Broadcast(msg)
	a.udpBcast.Broadcast(msg)
}

func (a *App) broadcastUpdate() {
	data := a.engine.GetGameJSON()
	msg, _ := protocol.NewMessage(protocol.ActionUpdate, nil)
	msg.Msg = data
	msg.Version = a.config.Version
	a.wsHub.Broadcast(msg)
}

func (a *App) broadcastGameState(phase string) {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionUpdate, data, false)
}

func (a *App) broadcastStart() {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionStart, data, true)
}

func (a *App) broadcastStop() {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionStop, data, true)
}

func (a *App) broadcastPause(bumperID string) {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionPause, data, true)
}

func (a *App) broadcastPauseAll() {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionPause, data, true)
}

func (a *App) broadcastContinue() {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionContinue, data, true)
}

func (a *App) broadcastTimerUpdate(currentTime int) {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionUpdateTimer, data, true)
}

func (a *App) broadcastPing() {
	msg, _ := protocol.NewMessage(protocol.ActionPing, map[string]interface{}{})
	a.udpBcast.Broadcast(msg)
}

func (a *App) broadcastReady() {
	data := a.engine.GetTeamsAndBumpersJSON()
	a.broadcast(protocol.ActionReady, data, true)
}

func (a *App) broadcastReveal(answer string) {
	data, _ := json.Marshal(answer)
	a.broadcast(protocol.ActionReveal, data, true)
}

func (a *App) broadcastReset() {
	msg, _ := protocol.NewMessage(protocol.ActionReset, map[string]interface{}{})
	a.wsHub.Broadcast(msg)
	a.udpBcast.Broadcast(msg)

	// Then send HELLO
	a.broadcastHello()
}

func (a *App) broadcastRemote() {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionRemote, data, false)
}

func (a *App) broadcastClientCounts(adminCount, tvCount int) {
	payload := protocol.ClientsPayload{
		AdminCount: adminCount,
		TVCount:    tvCount,
	}
	data, _ := json.Marshal(payload)
	a.broadcast(protocol.ActionClients, data, false)
	log.Printf("[App] Client counts: admin=%d, tv=%d", adminCount, tvCount)
}

func (a *App) broadcastQuestions() {
	// Load questions from storage
	questions := a.loadQuestions()
	data, _ := json.Marshal(questions)

	// Get storage info
	fsInfo := a.getStorageInfo()

	// Create message with FSINFO
	msg, _ := protocol.NewMessage(protocol.ActionQuestions, nil)
	msg.Msg = data
	msg.FSInfo = fsInfo
	msg.Version = a.config.Version

	// Broadcast via WebSocket only (not to buzzers)
	a.wsHub.Broadcast(msg)
}

// getStorageInfo returns file storage information (in bytes, like ESP32)
func (a *App) getStorageInfo() *protocol.FSInfo {
	filesDir := a.config.Storage.FilesDir
	if filesDir == "" {
		filesDir = "./data/files"
	}

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

	return &protocol.FSInfo{
		Used:  usedBytes,
		Free:  freeBytes,
		Total: totalBytes,
		PUsed: pUsed,
	}
}

// loadQuestions loads all questions from the questions directory
// Returns format matching ESP32: {"/files/questions/1": {...}, ...}
func (a *App) loadQuestions() map[string]map[string]interface{} {
	questions := make(map[string]map[string]interface{})

	questionsDir := a.config.Storage.QuestionsDir
	if questionsDir == "" {
		questionsDir = "./data/files/questions"
	}

	entries, err := os.ReadDir(questionsDir)
	if err != nil {
		log.Printf("[App] Failed to read questions directory: %v", err)
		return questions
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		questionFile := filepath.Join(questionsDir, entry.Name(), "question.json")
		data, err := os.ReadFile(questionFile)
		if err != nil {
			continue
		}

		var question map[string]interface{}
		if err := json.Unmarshal(data, &question); err != nil {
			continue
		}

		// Use full path as key (like ESP32: /files/questions/1)
		key := "/files/questions/" + entry.Name()
		questions[key] = question
	}

	return questions
}

// displayAndOpenURLs shows all accessible URLs and opens the browser
func displayAndOpenURLs(httpPort int) {
	log.Println("")
	log.Println("╔════════════════════════════════════════════════════════════╗")
	log.Println("║                    WEB INTERFACE URLs                      ║")
	log.Println("╠════════════════════════════════════════════════════════════╣")

	var primaryURL string

	// Get all local IPs
	interfaces, err := getNetworkInterfaces()
	if err == nil {
		for _, iface := range interfaces {
			url := fmt.Sprintf("http://%s:%d", iface.IP, httpPort)
			if httpPort == 80 {
				url = fmt.Sprintf("http://%s", iface.IP)
			}
			log.Printf("║  %-56s  ║", fmt.Sprintf("%s (%s)", url, iface.Name))

			// Use first non-virtual interface as primary
			if primaryURL == "" && !strings.Contains(strings.ToLower(iface.Name), "virtual") &&
				!strings.Contains(strings.ToLower(iface.Name), "vethernet") &&
				!strings.Contains(strings.ToLower(iface.Name), "wsl") {
				primaryURL = url
			}
		}
	}

	// Localhost
	localhostURL := fmt.Sprintf("http://localhost:%d", httpPort)
	if httpPort == 80 {
		localhostURL = "http://localhost"
	}
	log.Printf("║  %-56s  ║", localhostURL+" (localhost)")

	log.Println("╚════════════════════════════════════════════════════════════╝")
	log.Println("")

	// Use localhost if no primary found
	if primaryURL == "" {
		primaryURL = localhostURL
	}

	// Open browser
	log.Printf("Opening browser: %s", primaryURL)
	openBrowser(primaryURL)
}

// NetworkInterface represents a network interface with IP
type NetworkInterface struct {
	Name string
	IP   string
}

// getNetworkInterfaces returns all active network interfaces with IPv4 addresses
func getNetworkInterfaces() ([]NetworkInterface, error) {
	var result []NetworkInterface

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		// Skip down and loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Only IPv4
			if ip == nil || ip.To4() == nil {
				continue
			}

			result = append(result, NetworkInterface{
				Name: iface.Name,
				IP:   ip.String(),
			})
		}
	}

	return result, nil
}

// openBrowser opens the default browser with the given URL
func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // Linux
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to open browser: %v", err)
	}
}

// initTestData creates test teams and bumpers for development
func (a *App) initTestData() {
	log.Println("[App] Initializing test data...")

	// 5 teams with different colors (scores will be calculated from bumpers)
	teams := map[string]*game.Team{
		"Les Rouges": {
			Name:  "Les Rouges",
			Color: []int{239, 68, 68}, // Red
			Score: 0,
		},
		"Les Bleus": {
			Name:  "Les Bleus",
			Color: []int{59, 130, 246}, // Blue
			Score: 0,
		},
		"Les Verts": {
			Name:  "Les Verts",
			Color: []int{34, 197, 94}, // Green
			Score: 0,
		},
		"Les Jaunes": {
			Name:  "Les Jaunes",
			Color: []int{234, 179, 8}, // Yellow
			Score: 0,
		},
		"Les Violets": {
			Name:  "Les Violets",
			Color: []int{168, 85, 247}, // Purple
			Score: 0,
		},
	}

	// Fake bumpers (2-3 per team)
	bumpers := map[string]*game.Bumper{
		"AA:BB:CC:DD:EE:01": {
			Name:    "Alice",
			Team:    "Les Rouges",
			Score:   8,
			Version: "1.0.0",
		},
		"AA:BB:CC:DD:EE:02": {
			Name:    "Bob",
			Team:    "Les Rouges",
			Score:   7,
			Version: "1.0.0",
		},
		"AA:BB:CC:DD:EE:03": {
			Name:    "Charlie",
			Team:    "Les Bleus",
			Score:   6,
			Version: "1.0.0",
		},
		"AA:BB:CC:DD:EE:04": {
			Name:    "Diana",
			Team:    "Les Bleus",
			Score:   6,
			Version: "1.0.0",
		},
		"AA:BB:CC:DD:EE:05": {
			Name:    "Ethan",
			Team:    "Les Verts",
			Score:   5,
			Version: "1.0.0",
		},
		"AA:BB:CC:DD:EE:06": {
			Name:    "Fiona",
			Team:    "Les Verts",
			Score:   3,
			Version: "1.0.0",
		},
		"AA:BB:CC:DD:EE:07": {
			Name:    "George",
			Team:    "Les Jaunes",
			Score:   7,
			Version: "1.0.0",
		},
		"AA:BB:CC:DD:EE:08": {
			Name:    "Hannah",
			Team:    "Les Jaunes",
			Score:   3,
			Version: "1.0.0",
		},
		"AA:BB:CC:DD:EE:09": {
			Name:    "Ivan",
			Team:    "Les Violets",
			Score:   3,
			Version: "1.0.0",
		},
		"AA:BB:CC:DD:EE:10": {
			Name:    "Julia",
			Team:    "Les Violets",
			Score:   2,
			Version: "1.0.0",
		},
	}

	a.engine.SetTeams(teams)
	a.engine.SetBumpers(bumpers)
	a.engine.RecalculateAllTeamScores()

	log.Printf("[App] Test data initialized: %d teams, %d bumpers", len(teams), len(bumpers))
}

// checkBonjourSupport checks if Bonjour/mDNS is available on the system
func checkBonjourSupport() {
	if runtime.GOOS != "windows" {
		// On Linux/macOS, mDNS is usually available via avahi or built-in
		log.Println("[mDNS] Running on", runtime.GOOS, "- mDNS should be available")
		return
	}

	// On Windows, check if Bonjour service is running
	log.Println("[mDNS] Checking Bonjour service status...")

	// Query the Bonjour service using sc command
	cmd := exec.Command("sc", "query", "Bonjour Service")
	output, err := cmd.Output()
	if err != nil {
		// Service not found or error
		log.Println("[mDNS] ⚠ Bonjour is NOT installed")
		log.Println("[mDNS]   → mDNS hostname resolution (buzzcontrol.local) will NOT work")
		log.Println("[mDNS]   → BuzzClick service discovery will still work")
		log.Println("[mDNS]   → To enable hostname resolution, install Bonjour:")
		log.Println("[mDNS]     https://support.apple.com/kb/DL999")
		return
	}

	// Check if service is running
	outputStr := string(output)
	if strings.Contains(outputStr, "RUNNING") {
		log.Println("[mDNS] ✓ Bonjour service is running")
		log.Println("[mDNS]   → mDNS hostname resolution (buzzcontrol.local) should work")
	} else if strings.Contains(outputStr, "STOPPED") {
		log.Println("[mDNS] ⚠ Bonjour service is installed but STOPPED")
		log.Println("[mDNS]   → Start the service: sc start \"Bonjour Service\"")
	} else {
		log.Println("[mDNS] ⚠ Bonjour service status unknown")
		log.Printf("[mDNS]   → Output: %s", strings.TrimSpace(outputStr))
	}
}
