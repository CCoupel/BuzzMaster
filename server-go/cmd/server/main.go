package main

import (
	"buzzcontrol/assets"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"buzzcontrol/web"
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
	"time"
)

// Version is set at build time via -ldflags="-X main.Version=X.Y.Z"
var Version = "dev"

// App holds all server components
type App struct {
	config      *config.Config
	engine      *game.Engine
	tcpServer   *server.TCPServer
	udpBcast    *server.UDPBroadcaster
	httpServer  *server.HTTPServer
	wsHub       *server.WebSocketHub
	logsHub     *server.LogsWebSocketHub
	mdnsServer  *server.MDNSServer
	dnsServer   *server.DNSServer
	logger      *server.BroadcastLogger
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

	// Override version with embedded value (set via -ldflags at build time)
	if Version != "dev" {
		cfg.Version = Version
	}
	config.SetInstance(cfg)

	log.Printf("Version: %s (embedded: %s)", cfg.Version, Version)
	log.Printf("HTTP Port: %d", cfg.Server.HTTPPort)
	log.Printf("TCP Port: %d", cfg.Server.TCPPort)

	// Create app
	app := &App{
		config: cfg,
	}

	// Initialize components
	app.init()

	// Try to load saved teams and bumpers from disk
	teamsLoaded := app.engine.LoadTeams() == nil && len(app.engine.GetTeamsAndBumpers().Teams) > 0
	bumpersLoaded := app.engine.LoadBumpers() == nil && len(app.engine.GetTeamsAndBumpers().Bumpers) > 0

	// Only initialize test data if no saved data exists
	if !teamsLoaded && !bumpersLoaded {
		server.LogInfo(game.LogComponentApp, "No saved teams/bumpers found, initializing test data...")
		app.initTestData()
		// Save initial test data
		app.engine.SaveTeams()
		app.engine.SaveBumpers()
	} else {
		server.LogInfo(game.LogComponentApp, "Loaded from disk: %d teams, %d bumpers",
			len(app.engine.GetTeamsAndBumpers().Teams),
			len(app.engine.GetTeamsAndBumpers().Bumpers))
		app.engine.RecalculateAllTeamScores()
	}

	// Synchronize virtual player count with actual bumper count
	app.engine.SyncVirtualPlayerCount()

	// Load history and recalculate scores from events (overrides test data scores)
	if err := app.engine.LoadHistory(); err != nil {
		server.LogWarn(game.LogComponentApp, "Could not load history: %v", err)
	} else {
		app.engine.RecalculateScoresFromHistory()
	}

	// Load question statuses
	if err := app.engine.LoadStatuses(); err != nil {
		server.LogWarn(game.LogComponentApp, "Could not load question statuses: %v", err)
	}

	// Start servers
	if err := app.start(); err != nil {
		server.LogError(game.LogComponentApp, "Failed to start: %v", err)
		os.Exit(1)
	}

	server.LogInfo(game.LogComponentApp, "Server started successfully")
	server.LogInfo(game.LogComponentTCP, "TCP server (buzzers): port %d", cfg.Server.TCPPort)

	// Display all accessible URLs and open browser if enabled
	displayAndOpenURLs(cfg.Server.HTTPPort, cfg.Server.AutoOpenBrowsers, cfg.Server.Debug)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	server.LogInfo(game.LogComponentApp, "Shutting down...")
	app.stop()
}

func (a *App) init() {
	// Game engine
	a.engine = game.NewEngine()

	// Set persistence paths
	configDir := filepath.Join(a.config.Storage.DataDir, "config")
	a.engine.SetHistoryPath(filepath.Join(configDir, "history.json"))
	a.engine.SetTeamsPath(filepath.Join(configDir, "teams.json"))
	a.engine.SetBumpersPath(filepath.Join(configDir, "bumpers.json"))
	a.engine.SetStatusesPath(filepath.Join(configDir, "question_statuses.json"))

	// WebSocket hub
	a.wsHub = server.NewWebSocketHub()

	// Logs WebSocket hub (dedicated for logs)
	a.logsHub = server.NewLogsWebSocketHub(1000)

	// Initialize global logger (singleton)
	a.logger = server.InitLogger(1000)
	a.logger.SetDebugEnabled(a.config.Server.Debug)

	// TCP server for buzzers
	a.tcpServer = server.NewTCPServer(a.config.Server.TCPPort)

	// UDP broadcaster
	a.udpBcast = server.NewUDPBroadcaster(a.config.Server.TCPPort)

	// HTTP server
	a.httpServer = server.NewHTTPServer(a.config.Server.HTTPPort, a.engine, a.wsHub, a.logsHub)

	// Try embedded web files first, then fallback to filesystem
	if embeddedFS, ok := web.GetEmbeddedFS(); ok {
		server.LogInfo(game.LogComponentHTTP, "Using embedded web files (portable mode)")
		a.httpServer.SetEmbeddedFS(embeddedFS)
	} else {
		// Check for React build on filesystem
		reactDir := filepath.Join(".", "web", "dist")
		if _, err := os.Stat(filepath.Join(reactDir, "index.html")); err == nil {
			server.LogInfo(game.LogComponentHTTP, "React build found, serving modern UI")
			a.httpServer.SetReactDir(reactDir)
		} else {
			server.LogInfo(game.LogComponentHTTP, "No React build found, using legacy UI")
		}
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
		a.broadcastQuestions() // Sync question status with phase
	}

	// Timer ticks
	a.engine.OnTimerTick = func(currentTime int) {
		a.broadcastTimerUpdate(currentTime)
	}

	// Countdown ticks (3-2-1 before game starts)
	a.engine.OnCountdownTick = func(countdownTime int) {
		a.broadcastCountdownUpdate(countdownTime)
	}

	// Buzzer press
	a.engine.OnBuzzerPress = func(bumperID, teamID string, pressTime int64, button string) {
		a.broadcastPause(bumperID)
	}

	// QCM hint (when a wrong answer is invalidated)
	a.engine.OnQCMHint = func(invalidatedColor string, remainingAnswers int) {
		a.broadcastQCMHint(invalidatedColor, remainingAnswers)
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
		case "RESET_SELECT":
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

	// Load demo handler
	a.httpServer.OnLoadDemo = func() {
		a.loadDemoData()
		a.broadcastQuestions()
		a.broadcastUpdate()
	}

	// Config update handler
	a.httpServer.OnConfigUpdate = func() {
		a.broadcastConfigUpdate()
	}

	// Detect existing backgrounds on startup
	a.loadBackgrounds()

	// Handle client count changes (WebSocket connect/disconnect)
	a.wsHub.OnClientChange = func(adminCount, tvCount int) {
		a.broadcastClientCounts(adminCount, tvCount)
	}

	// Handle new log entries - broadcast to logs WebSocket clients
	a.logger.SetOnNewEntry(func(entry game.LogEntry) {
		payload := protocol.LogEntryPayload{
			Timestamp: entry.Timestamp,
			Level:     string(entry.Level),
			Component: string(entry.Component),
			Message:   entry.Message,
		}
		a.logsHub.BroadcastLogEntry(payload)
	})
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
		server.LogInfo(game.LogComponentApp, "Loaded %d background(s)", len(backgrounds))
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
		server.LogError(game.LogComponentApp, "Failed to marshal backgrounds config: %v", err)
		return
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		server.LogError(game.LogComponentApp, "Failed to save backgrounds config: %v", err)
	} else {
		server.LogDebug(game.LogComponentApp, "Saved backgrounds config")
	}
}

// startBackgroundCycling manages server-synchronized background image cycling
func (a *App) startBackgroundCycling() {
	server.LogDebug(game.LogComponentApp, "Starting background cycling goroutine")

	for {
		// Get current background duration
		duration := a.engine.GetCurrentBackgroundDuration()
		if duration <= 0 {
			duration = 10 // Default 10 seconds
		}

		// Wait for the duration
		time.Sleep(time.Duration(duration) * time.Second)

		// Move to next background and broadcast
		backgrounds := a.engine.GetBackgrounds()
		if len(backgrounds) > 1 {
			newIndex := a.engine.NextBackground()
			a.broadcastBackgroundChange(newIndex)
			// Also include in regular UPDATE so new clients get the index
			a.broadcastUpdate()
		}
	}
}

func (a *App) start() error {
	// Start WebSocket hub
	go a.wsHub.Run()

	// Start Logs WebSocket hub
	go a.logsHub.Run()

	// Start TCP server
	if err := a.tcpServer.Start(); err != nil {
		return err
	}
	a.logger.Info(game.LogComponentTCP, "TCP server started on port %d", a.config.Server.TCPPort)

	// Start UDP broadcaster
	if err := a.udpBcast.Start(); err != nil {
		return err
	}
	a.logger.Info(game.LogComponentUDP, "UDP broadcaster started on port %d", a.config.Server.TCPPort)

	// Start HTTP server
	if err := a.httpServer.Start(); err != nil {
		return err
	}
	a.logger.Info(game.LogComponentHTTP, "HTTP server started on port %d", a.config.Server.HTTPPort)

	// Start mDNS server (non-fatal if it fails)
	if err := a.mdnsServer.Start(); err != nil {
		a.logger.Warn(game.LogComponentApp, "Failed to start mDNS: %v", err)
	}

	// Start DNS server (non-fatal if it fails - may need admin rights)
	if err := a.dnsServer.Start(); err != nil {
		a.logger.Warn(game.LogComponentApp, "Failed to start DNS server: %v (may need admin rights)", err)
	}

	// Send initial HELLO
	a.broadcastHello()

	// Start background cycling goroutine
	go a.startBackgroundCycling()

	a.logger.Info(game.LogComponentApp, "BuzzControl server v%s started successfully", a.config.Version)

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
		server.LogWarn(game.LogComponentApp, "Unknown buzzer action: %s", msg.Action)
	}
}

// handleWebMessage processes messages from web clients (WebSocket)
func (a *App) handleWebMessage(incoming *protocol.IncomingMessage) {
	msg := incoming.Data

	switch msg.Action {
	case protocol.ActionHello:
		// Send state directly to the connecting client (not broadcast - avoids race condition)
		a.sendStateToClient(incoming.ClientID)

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
		a.logger.Info(game.LogComponentEngine, "STOP game")
		a.engine.Stop()
		a.broadcastStop()

	case protocol.ActionPause:
		a.logger.Info(game.LogComponentEngine, "PAUSE all")
		a.engine.PauseAll()
		a.broadcastPauseAll()

	case protocol.ActionContinue:
		a.logger.Info(game.LogComponentEngine, "CONTINUE game")
		a.engine.Continue()
		a.broadcastContinue()

	case protocol.ActionReveal:
		a.logger.Info(game.LogComponentEngine, "REVEAL answer")
		answer := a.engine.Reveal()
		a.broadcastReveal(answer)

	case protocol.ActionRAZ:
		a.logger.Info(game.LogComponentEngine, "RAZ - Reset all scores")
		a.engine.RAZScores()
		a.broadcastUpdate()

	case protocol.ActionRemote:
		a.handleRemote(msg)

	case protocol.ActionDelete:
		a.handleDelete(msg)

	case protocol.ActionDeleteBumper:
		a.handleDeleteBumper(msg)

	case protocol.ActionReset:
		a.broadcastReset()

	case protocol.ActionReboot:
		server.LogInfo(game.LogComponentApp, "Reboot requested from web client")

	case protocol.ActionBumperPoints:
		a.handleBumperPoints(msg)

	case protocol.ActionTeamPoints:
		a.handleTeamPoints(msg)

	case protocol.ActionSetClientType:
		a.handleSetClientType(incoming.ClientID, msg)

	case protocol.ActionReorderQuestions:
		a.handleReorderQuestions(msg)

	case protocol.ActionForceReady:
		a.handleForceReady()

	case protocol.ActionButton:
		// Simulated button press from web client (for testing)
		a.handleSimulatedButton(msg)

	case protocol.ActionPong:
		// PONG from web client (simulated, for testing in PREPARE state)
		a.handlePong(incoming.ClientID, msg)

	case protocol.ActionFlipMemoryCard:
		a.handleFlipMemoryCard(msg)

	case protocol.ActionShowQRCode:
		a.handleShowQRCode()

	case protocol.ActionHideQRCode:
		a.handleHideQRCode()

	case protocol.ActionSetVirtualPlayerLimit:
		a.handleSetVirtualPlayerLimit(msg)

	case protocol.ActionPlayerConnect:
		a.handlePlayerConnect(incoming.ClientID, msg)

	default:
		server.LogWarn(game.LogComponentApp, "Unknown web action: %s", msg.Action)
	}
}

func (a *App) handleHello(clientID string, msg *protocol.Message) {
	a.logger.Info(game.LogComponentTCP, "HELLO from buzzer: %s", clientID)

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
		server.LogError(game.LogComponentApp, "Failed to parse button payload: %v", err)
		return
	}

	server.LogInfo(game.LogComponentTCP, "BUTTON from %s: %s", clientID, payload.Button)

	// Process in engine
	a.engine.ProcessButtonPress(clientID, timestamp, payload.Button)

	// Broadcast pause to all
	a.broadcastPause(clientID)
	a.broadcastUpdate()
}

// handleSimulatedButton processes button press from web client (debug/testing)
func (a *App) handleSimulatedButton(msg *protocol.Message) {
	var payload struct {
		ID     string `json:"ID"`
		Button string `json:"button"`
	}
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse simulated button payload: %v", err)
		return
	}

	if payload.ID == "" {
		server.LogWarn(game.LogComponentApp, "Simulated BUTTON missing ID")
		return
	}

	// Default button to "A" if not specified
	button := payload.Button
	if button == "" {
		button = "A"
	}

	// Use current time as timestamp (microseconds)
	timestamp := time.Now().UnixMicro()

	server.LogInfo(game.LogComponentEngine, "Simulated BUTTON from %s: %s (time: %d)", payload.ID, button, timestamp)

	// Process in engine (same as real button press)
	a.engine.ProcessButtonPress(payload.ID, timestamp, button)

	// Broadcast pause to all
	a.broadcastPause(payload.ID)
	a.broadcastUpdate()
}

// handlePong processes PONG from buzzer (TCP) or web client (WebSocket simulation)
func (a *App) handlePong(clientID string, msg *protocol.Message) {
	// If ID in payload, use it (web simulation), otherwise use clientID (TCP buzzer)
	bumperID := clientID

	var payload struct {
		ID string `json:"ID"`
	}
	if json.Unmarshal(msg.Msg, &payload) == nil && payload.ID != "" {
		bumperID = payload.ID
	}

	server.LogInfo(game.LogComponentTCP, "PONG from %s", bumperID)

	if a.engine.IsGamePrepare() {
		a.engine.SetBumperReady(bumperID)

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
		server.LogError(game.LogComponentApp, "Failed to parse FULL update: %v", err)
		return
	}

	// Only update if data is provided (don't overwrite with nil)
	if data.Teams != nil {
		a.engine.SetTeams(data.Teams)
	}
	if data.Bumpers != nil {
		a.engine.SetBumpers(data.Bumpers)
	}
	a.broadcastUpdate()
}

func (a *App) handleUpdate(msg *protocol.Message) {
	// Similar to FULL but partial update
	a.handleFullUpdate(msg)
}

func (a *App) handlePoints(msg *protocol.Message) {
	var payload protocol.PointsPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse POINTS: %v", err)
		return
	}

	bumperScore, teamScore := a.engine.UpdateScore(payload.BumperID, payload.Points)
	server.LogInfo(game.LogComponentEngine, "Points: bumper=%s, +%d, bumperScore=%d, teamScore=%d",
		payload.BumperID, payload.Points, bumperScore, teamScore)

	a.broadcastUpdate()
}

func (a *App) handleReady(msg *protocol.Message) {
	var payload protocol.ReadyPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		a.logger.Error(game.LogComponentEngine, "Failed to parse READY: %v", err)
		return
	}

	// Load question from storage
	var question *game.Question
	if payload.Question != "" {
		question = a.loadQuestion(payload.Question)
		if question == nil {
			a.logger.Warn(game.LogComponentEngine, "Question not found: %s", payload.Question)
			return
		}
	}

	a.logger.Info(game.LogComponentEngine, "READY question=%s", payload.Question)
	a.engine.Ready(payload.Question, question)

	// Broadcast state update to web clients
	a.broadcastUpdate()

	// Send PING to all buzzers
	a.broadcastPing()
}

// loadQuestion loads a question from storage by ID
func (a *App) loadQuestion(id string) *game.Question {
	questionsDir := a.config.Storage.QuestionsDir
	if questionsDir == "" {
		questionsDir = "./data/files/questions"
	}

	questionFile := filepath.Join(questionsDir, id, "question.json")
	data, err := os.ReadFile(questionFile)
	if err != nil {
		server.LogError(game.LogComponentApp, "Failed to read question file %s: %v", questionFile, err)
		return nil
	}

	var q game.Question
	if err := json.Unmarshal(data, &q); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse question JSON: %v", err)
		return nil
	}

	// Set default POINTS_TARGET if not present
	if q.PointsTarget == "" {
		if q.Type == game.QuestionTypeQCM {
			q.PointsTarget = game.PointsTargetTeam
		} else {
			q.PointsTarget = game.PointsTargetPlayer
		}
	}

	return &q
}

func (a *App) handleStart(msg *protocol.Message) {
	var payload protocol.StartPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		payload.Delay = a.config.Game.DefaultDelay
	}

	if payload.Delay <= 0 {
		payload.Delay = a.config.Game.DefaultDelay
	}

	a.logger.Info(game.LogComponentEngine, "START game with delay=%ds", payload.Delay)
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
		server.LogError(game.LogComponentApp, "Failed to parse DELETE: %v", err)
		return
	}

	// Validate ID (must be numeric like ESP32)
	if payload.ID == "" {
		server.LogWarn(game.LogComponentApp, "DELETE: empty ID")
		return
	}

	// Delete question directory (like ESP32: deleteDirectory(questionsPath + "/" + ID))
	questionsDir := a.config.Storage.QuestionsDir
	if questionsDir == "" {
		questionsDir = "./data/files/questions"
	}
	questionPath := filepath.Join(questionsDir, payload.ID)

	server.LogInfo(game.LogComponentApp, "DELETE question: path=%s", questionPath)

	// Check if path exists before deleting
	if _, err := os.Stat(questionPath); os.IsNotExist(err) {
		server.LogWarn(game.LogComponentApp, "DELETE: Path does not exist: %s", questionPath)
	}

	if err := os.RemoveAll(questionPath); err != nil {
		server.LogError(game.LogComponentApp, "Failed to delete question %s: %v", payload.ID, err)
	} else {
		server.LogInfo(game.LogComponentApp, "Deleted question: %s (path: %s)", payload.ID, questionPath)
	}

	// Broadcast updated questions list (like ESP32: sendQuestions())
	a.broadcastQuestions()
}

func (a *App) handleDeleteBumper(msg *protocol.Message) {
	var payload protocol.DeletePayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse DELETE_BUMPER: %v", err)
		return
	}

	if payload.ID == "" {
		server.LogWarn(game.LogComponentApp, "DELETE_BUMPER: empty ID")
		return
	}

	server.LogInfo(game.LogComponentApp, "DELETE_BUMPER: id=%s", payload.ID)

	// Remove bumper from engine
	bumpers := a.engine.GetTeamsAndBumpers().Bumpers
	if _, exists := bumpers[payload.ID]; !exists {
		server.LogWarn(game.LogComponentApp, "DELETE_BUMPER: Bumper %s not found", payload.ID)
		return
	}

	delete(bumpers, payload.ID)
	a.engine.SetBumpers(bumpers)

	server.LogInfo(game.LogComponentApp, "Deleted bumper: %s", payload.ID)

	// Broadcast updated state
	a.broadcastUpdate()
}

func (a *App) handleReorderQuestions(msg *protocol.Message) {
	var payload protocol.ReorderQuestionsPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse REORDER_QUESTIONS: %v", err)
		return
	}

	if len(payload.Order) == 0 {
		server.LogWarn(game.LogComponentApp, "REORDER_QUESTIONS: empty order")
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
			server.LogError(game.LogComponentApp, "REORDER: Failed to read question %s: %v", questionID, err)
			continue
		}

		var question map[string]interface{}
		if err := json.Unmarshal(data, &question); err != nil {
			server.LogError(game.LogComponentApp, "REORDER: Failed to parse question %s: %v", questionID, err)
			continue
		}

		// Update ORDER field
		question["ORDER"] = order

		// Write back
		newData, err := json.MarshalIndent(question, "", "  ")
		if err != nil {
			server.LogError(game.LogComponentApp, "REORDER: Failed to marshal question %s: %v", questionID, err)
			continue
		}

		if err := os.WriteFile(questionFile, newData, 0644); err != nil {
			server.LogError(game.LogComponentApp, "REORDER: Failed to write question %s: %v", questionID, err)
			continue
		}
	}

	server.LogInfo(game.LogComponentApp, "Reordered %d questions", len(payload.Order))

	// Broadcast updated questions list
	a.broadcastQuestions()
}



func (a *App) handleForceReady() {
	server.LogInfo(game.LogComponentEngine, "FORCE_READY requested (debug)")
	a.engine.ForceReady()
	a.broadcastReady()
	a.broadcastUpdate()
}

func (a *App) handleFlipMemoryCard(msg *protocol.Message) {
	var payload protocol.FlipMemoryCardPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse FLIP_MEMORY_CARD: %v", err)
		return
	}

	if payload.CardID == "" {
		server.LogWarn(game.LogComponentApp, "FLIP_MEMORY_CARD: empty card ID")
		return
	}

	server.LogInfo(game.LogComponentEngine, "FLIP_MEMORY_CARD: cardID=%s", payload.CardID)

	// Process the flip with game logic
	isMatch, shouldFlipBack, flipDelay, isComplete := a.engine.FlipMemoryCard(payload.CardID)

	// Broadcast updated game state to all clients
	a.broadcastUpdate()

	// If no match and 2 cards are flipped, schedule auto-flip-back
	if shouldFlipBack && flipDelay > 0 {
		go func() {
			time.Sleep(time.Duration(flipDelay) * time.Millisecond)
			a.engine.ClearMemoryFlippedCards()
			server.LogInfo(game.LogComponentEngine, "Memory auto-flip-back after %dms", flipDelay)
			a.broadcastUpdate()
		}()
	}

	if isMatch {
		server.LogInfo(game.LogComponentEngine, "Memory MATCH found!")
	}

	// If all pairs matched, automatically stop the game
	if isComplete {
		server.LogInfo(game.LogComponentEngine, "Memory game COMPLETE! All pairs matched.")
		a.engine.Stop()
		a.broadcastUpdate()
	}
}

func (a *App) handleBumperPoints(msg *protocol.Message) {
	var payload protocol.BumperPointsPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse BUMPER_POINTS: %v", err)
		return
	}

	newScore := a.engine.UpdateBumperScore(payload.ID, payload.Points)
	server.LogInfo(game.LogComponentEngine, "Bumper points: id=%s, points=%+d, newScore=%d",
		payload.ID, payload.Points, newScore)

	// Record event to history
	state := a.engine.GetState()
	bumper := a.engine.GetBumper(payload.ID)
	bumperName := payload.ID
	teamName := ""
	var teamColor []int
	playerColor := ""
	if bumper != nil {
		if bumper.Name != "" {
			bumperName = bumper.Name
		}
		teamName = bumper.Team
		playerColor = string(bumper.AnswerColor)
		// Get team color
		if team := a.engine.GetTeam(bumper.Team); team != nil {
			teamColor = team.Color
		}
	}
	questionID := ""
	questionText := ""
	questionCategory := ""
	if state.Question != nil {
		questionID = state.Question.ID
		questionText = state.Question.Question
		questionCategory = string(state.Question.Category)
	}
	event := game.GameEvent{
		Timestamp:        time.Now().UnixMicro(),
		QuestionID:       questionID,
		QuestionText:     questionText,
		QuestionCategory: questionCategory,
		EventType:        "POINTS_AWARDED",
		WinnerID:         payload.ID,
		WinnerName:       bumperName,
		WinnerType:       "PLAYER",
		TeamName:         teamName,
		TeamColor:        teamColor,
		PlayerName:       bumperName,
		PlayerColor:      playerColor,
		Points:           payload.Points,
	}
	a.engine.AddGameEvent(event)

	a.broadcastUpdate()
}

func (a *App) handleTeamPoints(msg *protocol.Message) {
	var payload protocol.TeamPointsPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse TEAM_POINTS: %v", err)
		return
	}

	newScore := a.engine.UpdateTeamScore(payload.Team, payload.Points)
	server.LogInfo(game.LogComponentEngine, "Team points: team=%s, points=%+d, newScore=%d",
		payload.Team, payload.Points, newScore)

	// Record event to history
	state := a.engine.GetState()
	var teamColor []int
	if team := a.engine.GetTeam(payload.Team); team != nil {
		teamColor = team.Color
	}
	questionID := ""
	questionText := ""
	questionCategory := ""
	if state.Question != nil {
		questionID = state.Question.ID
		questionText = state.Question.Question
		questionCategory = string(state.Question.Category)
	}
	event := game.GameEvent{
		Timestamp:        time.Now().UnixMicro(),
		QuestionID:       questionID,
		QuestionText:     questionText,
		QuestionCategory: questionCategory,
		EventType:        "POINTS_AWARDED",
		WinnerID:         payload.Team,
		WinnerName:       payload.Team,
		WinnerType:       "TEAM",
		TeamName:         payload.Team,
		TeamColor:        teamColor,
		Points:           payload.Points,
	}
	a.engine.AddGameEvent(event)

	a.broadcastUpdate()
}

func (a *App) handleSetClientType(clientID string, msg *protocol.Message) {
	var payload protocol.SetClientTypePayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse SET_CLIENT_TYPE: %v", err)
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

func (a *App) handleShowQRCode() {
	// Get current limit (or use default)
	state := a.engine.GetState()
	limit := state.VirtualPlayerLimit
	if limit == 0 {
		limit = 20 // Default
	}

	a.engine.StartEnrollment(limit)
	a.engine.SetPhase(game.PhaseEnroll)
	server.LogInfo(game.LogComponentApp, "Entering ENROLL phase - QR code displayed, limit: %d", limit)
	a.broadcastUpdate()
	a.broadcastEnrollmentUpdate()
}

func (a *App) handleHideQRCode() {
	a.engine.StopEnrollment()
	a.engine.SetPhase(game.PhaseStopped)
	server.LogInfo(game.LogComponentApp, "Exiting ENROLL phase - %d virtual players enrolled", a.engine.GetVirtualPlayerCount())
	a.broadcastUpdate()
	a.broadcastEnrollmentUpdate()
}

func (a *App) handleSetVirtualPlayerLimit(msg *protocol.Message) {
	var payload protocol.SetVirtualPlayerLimitPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Error parsing SET_VIRTUAL_PLAYER_LIMIT payload: %v", err)
		return
	}
	a.engine.SetVirtualPlayerLimit(payload.Limit)
	server.LogInfo(game.LogComponentApp, "Virtual player limit set to: %d", payload.Limit)
	a.broadcastUpdate()
	a.broadcastEnrollmentUpdate()
}

func (a *App) handlePlayerConnect(clientID string, msg *protocol.Message) {
	var payload protocol.PlayerConnectPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Error parsing PLAYER_CONNECT payload: %v", err)
		return
	}

	// Validate name (trim whitespace and check length: 2-20 chars)
	playerName := strings.TrimSpace(payload.Name)
	if len(playerName) < 2 || len(playerName) > 20 {
		server.LogWarn(game.LogComponentApp, "PLAYER_CONNECT: invalid name length (%d chars) from client %s", len(playerName), clientID)

		// Send rejection to this client
		rejectPayload := protocol.PlayerRejectedPayload{
			Reason: "INVALID_NAME",
		}
		rejectMsg, _ := protocol.NewMessage(protocol.ActionPlayerRejected, rejectPayload)
		a.wsHub.SendToClient(clientID, rejectMsg)
		return
	}

	// Check if player with this name already exists (reconnection)
	var existingBumperID string
	var existingBumper *game.Bumper
	for id, b := range a.engine.GetTeamsAndBumpers().Bumpers {
		if b.IsVirtual && b.Name == playerName {
			existingBumperID = id
			existingBumper = b
			break
		}
	}

	// If player exists, allow reconnection
	if existingBumper != nil {
		server.LogInfo(game.LogComponentWebSocket, "PLAYER_CONNECT: reconnecting existing player: id=%s, name=%s", existingBumperID, existingBumper.Name)

		// Send confirmation to this client
		connectedPayload := protocol.PlayerConnectedPayload{
			ID:   existingBumperID,
			Name: existingBumper.Name,
			Team: existingBumper.Team,
		}
		connectedMsg, _ := protocol.NewMessage(protocol.ActionPlayerConnected, connectedPayload)
		a.wsHub.SendToClient(clientID, connectedMsg)

		// Broadcast UPDATE to all clients (to notify of reconnection)
		a.broadcastUpdate()
		return
	}

	// New player: Try to create virtual player
	bumperID, bumper, err := a.engine.CreateVirtualPlayer(playerName)
	if err != nil {
		// Send rejection to this client only
		reason := "ENROLLMENT_CLOSED"
		if enrollErr, ok := err.(*game.EnrollmentError); ok {
			reason = enrollErr.Reason
		}

		rejectPayload := protocol.PlayerRejectedPayload{
			Reason: reason,
		}
		rejectMsg, _ := protocol.NewMessage(protocol.ActionPlayerRejected, rejectPayload)

		// Send rejection via WebSocket
		a.wsHub.SendToClient(clientID, rejectMsg)
		server.LogWarn(game.LogComponentWebSocket, "PLAYER_CONNECT: rejected player=%s, reason=%s", payload.Name, reason)
		return
	}

	// Send confirmation to this client
	connectedPayload := protocol.PlayerConnectedPayload{
		ID:   bumperID,
		Name: bumper.Name,
		Team: bumper.Team,
	}
	connectedMsg, _ := protocol.NewMessage(protocol.ActionPlayerConnected, connectedPayload)
	a.wsHub.SendToClient(clientID, connectedMsg)
	server.LogInfo(game.LogComponentWebSocket, "PLAYER_CONNECT: player connected: id=%s, name=%s", bumperID, bumper.Name)

	// Broadcast UPDATE to all clients (teams/bumpers updated)
	a.broadcastUpdate()
	// Broadcast enrollment count update
	a.broadcastEnrollmentUpdate()
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
	server.LogDebug(game.LogComponentApp, "Broadcasting UPDATE: %s", string(data)[:min(200, len(data))])
	msg, _ := protocol.NewMessage(protocol.ActionUpdate, nil)
	msg.Msg = data
	msg.Version = a.config.Version
	a.wsHub.Broadcast(msg)
}

func (a *App) broadcastEnrollmentUpdate() {
	state := a.engine.GetState()
	payload := protocol.EnrollmentUpdatePayload{
		VirtualPlayerCount: state.VirtualPlayerCount,
		VirtualPlayerLimit: state.VirtualPlayerLimit,
		EnrollmentActive:   state.EnrollmentActive,
	}
	msg, _ := protocol.NewMessage(protocol.ActionEnrollmentUpdate, payload)
	a.wsHub.Broadcast(msg)
	server.LogDebug(game.LogComponentApp, "Broadcasting ENROLLMENT_UPDATE: %d/%d players", state.VirtualPlayerCount, state.VirtualPlayerLimit)
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

func (a *App) broadcastCountdownUpdate(countdownTime int) {
	data := a.engine.GetGameJSON()
	server.LogDebug(game.LogComponentApp, "Broadcasting countdown: %d", countdownTime)
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
	server.LogDebug(game.LogComponentWebSocket, "Client counts: admin=%d, tv=%d", adminCount, tvCount)
}

func (a *App) broadcastBackgroundChange(index int) {
	payload := protocol.BackgroundChangePayload{
		Index: index,
	}
	data, _ := json.Marshal(payload)
	a.broadcast(protocol.ActionBackgroundChange, data, false)
	server.LogDebug(game.LogComponentApp, "Background change: index=%d", index)
}

func (a *App) broadcastQCMHint(invalidatedColor string, remainingAnswers int) {
	payload := protocol.QCMHintPayload{
		Color:     invalidatedColor,
		Remaining: remainingAnswers,
	}
	data, _ := json.Marshal(payload)
	a.broadcast(protocol.ActionQCMHint, data, false)
	server.LogInfo(game.LogComponentEngine, "QCM hint: invalidated=%s, remaining=%d", invalidatedColor, remainingAnswers)

	// Also broadcast full update so clients receive the updated QcmInvalidated state
	a.broadcastUpdate()
}

func (a *App) broadcastShowQRCode() {
	payload := protocol.QRCodePayload{
		URL:  "http://localhost/player",
		Show: true,
	}
	data, _ := json.Marshal(payload)
	a.broadcast(protocol.ActionShowQRCode, data, false)
	server.LogInfo(game.LogComponentApp, "QR Code enrollment activated")
}

func (a *App) broadcastHideQRCode() {
	payload := protocol.QRCodePayload{
		URL:  "",
		Show: false,
	}
	data, _ := json.Marshal(payload)
	a.broadcast(protocol.ActionHideQRCode, data, false)
	server.LogInfo(game.LogComponentApp, "QR Code enrollment deactivated")
}

func (a *App) broadcastConfigUpdate() {
	cfg := config.Get()
	payload := protocol.ConfigUpdatePayload{
		NeonEffect: protocol.NeonEffectPayload{
			Enabled:       cfg.NeonEffect.Enabled,
			Mode:          cfg.NeonEffect.Mode,
			ArcWidth:      cfg.NeonEffect.ArcWidth,
			IntensityGap:  cfg.NeonEffect.IntensityGap,
			RotationSpeed: cfg.NeonEffect.RotationSpeed,
			BarOffset:     cfg.NeonEffect.BarOffset,
			BarThickness:  cfg.NeonEffect.BarThickness,
			ArcBlur:       cfg.NeonEffect.ArcBlur,
		},
	}
	data, _ := json.Marshal(payload)
	a.broadcast(protocol.ActionConfigUpdate, data, false)
	server.LogInfo(game.LogComponentApp, "Config update broadcast (neon: enabled=%v, mode=%s, arc=%d, intensity=%d, speed=%.1f, offset=%d, thickness=%d, blur=%d)",
		cfg.NeonEffect.Enabled, cfg.NeonEffect.Mode, cfg.NeonEffect.ArcWidth, cfg.NeonEffect.IntensityGap, cfg.NeonEffect.RotationSpeed, cfg.NeonEffect.BarOffset, cfg.NeonEffect.BarThickness, cfg.NeonEffect.ArcBlur)
}

func (a *App) broadcastQuestions() {
	// Load questions from storage
	questions := a.loadQuestions()

	// Inject status from questionStatuses map for ALL questions
	for _, q := range questions {
		if qID, ok := q["ID"].(string); ok && qID != "" {
			status := a.engine.GetQuestionStatus(qID)
			q["STATUS"] = string(status)
		}
	}

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

// sendStateToClient sends the full state to a specific client (used on HELLO)
func (a *App) sendStateToClient(clientID string) {
	server.LogDebug(game.LogComponentWebSocket, "Sending state to client: %s", clientID)

	// Send UPDATE with game state
	data := a.engine.GetGameJSON()
	updateMsg, _ := protocol.NewMessage(protocol.ActionUpdate, nil)
	updateMsg.Msg = data
	updateMsg.Version = a.config.Version
	a.wsHub.SendToClient(clientID, updateMsg)

	// Send QUESTIONS with statuses
	questions := a.loadQuestions()
	// Inject status from questionStatuses map for ALL questions
	for _, q := range questions {
		if qID, ok := q["ID"].(string); ok && qID != "" {
			status := a.engine.GetQuestionStatus(qID)
			q["STATUS"] = string(status)
		}
	}
	qData, _ := json.Marshal(questions)
	fsInfo := a.getStorageInfo()
	questionsMsg, _ := protocol.NewMessage(protocol.ActionQuestions, nil)
	questionsMsg.Msg = qData
	questionsMsg.FSInfo = fsInfo
	questionsMsg.Version = a.config.Version
	a.wsHub.SendToClient(clientID, questionsMsg)

	// Send CLIENTS counts
	adminCount, tvCount := a.wsHub.GetClientCounts()
	clientsPayload := protocol.ClientsPayload{
		AdminCount: adminCount,
		TVCount:    tvCount,
	}
	cData, _ := json.Marshal(clientsPayload)
	clientsMsg, _ := protocol.NewMessage(protocol.ActionClients, nil)
	clientsMsg.Msg = cData
	a.wsHub.SendToClient(clientID, clientsMsg)

	// Send CONFIG_UPDATE with neon effect settings
	cfg := config.Get()
	neonPayload := protocol.ConfigUpdatePayload{
		NeonEffect: protocol.NeonEffectPayload{
			Enabled:       cfg.NeonEffect.Enabled,
			Mode:          cfg.NeonEffect.Mode,
			ArcWidth:      cfg.NeonEffect.ArcWidth,
			IntensityGap:  cfg.NeonEffect.IntensityGap,
			RotationSpeed: cfg.NeonEffect.RotationSpeed,
			BarOffset:     cfg.NeonEffect.BarOffset,
			BarThickness:  cfg.NeonEffect.BarThickness,
			ArcBlur:       cfg.NeonEffect.ArcBlur,
		},
	}
	neonData, _ := json.Marshal(neonPayload)
	neonMsg, _ := protocol.NewMessage(protocol.ActionConfigUpdate, nil)
	neonMsg.Msg = neonData
	a.wsHub.SendToClient(clientID, neonMsg)
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
		server.LogError(game.LogComponentApp, "Failed to read questions directory: %v", err)
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

		// Set default POINTS_TARGET if not present
		if _, ok := question["POINTS_TARGET"]; !ok {
			qType, _ := question["TYPE"].(string)
			if qType == "QCM" {
				question["POINTS_TARGET"] = "TEAM"
			} else {
				question["POINTS_TARGET"] = "PLAYER"
			}
		}

		// Use full path as key (like ESP32: /files/questions/1)
		key := "/files/questions/" + entry.Name()
		questions[key] = question
	}

	return questions
}

// displayAndOpenURLs shows all accessible URLs and opens the browser
func displayAndOpenURLs(httpPort int, autoOpen bool, debug bool) {
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

	// Only auto-open if enabled in config
	if !autoOpen {
		log.Println("Auto-open browsers disabled in config")
		return
	}

	// Open browsers to /, /tv and /anim pages with small delays
	// Order: /anim (admin), /tv (display), / (home), /logs (if debug)
	pagesToOpen := []struct {
		path string
		name string
	}{
		{"/anim", "admin"},
		{"/tv", "TV display"},
		{"/", "home"},
	}

	// Add /logs page if debug mode is enabled
	if debug {
		pagesToOpen = append(pagesToOpen, struct {
			path string
			name string
		}{"/logs", "logs (debug)"})
	}

	for i, page := range pagesToOpen {
		url := primaryURL + page.path
		log.Printf("Opening browser: %s (%s)", url, page.name)
		openBrowser(url)
		// Small delay between browser opens to avoid resource issues
		if i < len(pagesToOpen)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
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
		server.LogWarn(game.LogComponentApp, "Failed to open browser: %v", err)
	}
}

// initTestData creates test teams and bumpers for development
func (a *App) initTestData() {
	server.LogInfo(game.LogComponentApp, "Initializing test data...")

	// 6 teams with different colors (scores will be calculated from bumpers)
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
		"Les Oranges": {
			Name:  "Les Oranges",
			Color: []int{249, 115, 22}, // Orange
			Score: 0,
		},
	}

	// Fake bumpers (2-3 per team) with answer colors for QCM mode
	bumpers := map[string]*game.Bumper{
		"AA:BB:CC:DD:EE:01": {
			Name:        "Alice",
			Team:        "Les Rouges",
			Score:       8,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorRed,
		},
		"AA:BB:CC:DD:EE:02": {
			Name:        "Bob",
			Team:        "Les Rouges",
			Score:       7,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorGreen,
		},
		"AA:BB:CC:DD:EE:03": {
			Name:        "Charlie",
			Team:        "Les Bleus",
			Score:       6,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorYellow,
		},
		"AA:BB:CC:DD:EE:04": {
			Name:        "Diana",
			Team:        "Les Bleus",
			Score:       6,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorBlue,
		},
		"AA:BB:CC:DD:EE:05": {
			Name:        "Ethan",
			Team:        "Les Verts",
			Score:       5,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorRed,
		},
		"AA:BB:CC:DD:EE:06": {
			Name:        "Fiona",
			Team:        "Les Verts",
			Score:       3,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorGreen,
		},
		"AA:BB:CC:DD:EE:07": {
			Name:        "George",
			Team:        "Les Jaunes",
			Score:       7,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorYellow,
		},
		"AA:BB:CC:DD:EE:08": {
			Name:        "Hannah",
			Team:        "Les Jaunes",
			Score:       3,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorBlue,
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
		"AA:BB:CC:DD:EE:11": {
			Name:        "Kevin",
			Team:        "Les Oranges",
			Score:       4,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorYellow, // C
		},
		"AA:BB:CC:DD:EE:12": {
			Name:        "Laura",
			Team:        "Les Oranges",
			Score:       5,
			Version:     "1.0.0",
			AnswerColor: game.AnswerColorRed, // A
		},
	}

	a.engine.SetTeams(teams)
	a.engine.SetBumpers(bumpers)
	a.engine.RecalculateAllTeamScores()

	// Create test questions with different categories
	a.createTestQuestions()

	server.LogInfo(game.LogComponentApp, "Test data initialized: %d teams, %d bumpers", len(teams), len(bumpers))
}

// createTestQuestions creates test questions with various categories
func (a *App) createTestQuestions() {
	questionsDir := a.config.Storage.QuestionsDir
	if questionsDir == "" {
		questionsDir = "./data/files/questions"
	}

	// Ensure questions directory exists
	os.MkdirAll(questionsDir, 0755)

	// Check if questions already exist
	entries, _ := os.ReadDir(questionsDir)
	if len(entries) > 0 {
		server.LogInfo(game.LogComponentApp, "Questions already exist (%d), skipping test questions", len(entries))
		return
	}

	server.LogInfo(game.LogComponentApp, "Creating test questions with categories...")

	testQuestions := []map[string]interface{}{
		// GEOGRAPHY
		{
			"ID":            "1",
			"QUESTION":      "Quelle est la capitale de l'Australie?",
			"ANSWER":        "Canberra",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "NORMAL",
			"CATEGORY":      "GEOGRAPHY",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         0,
		},
		// ENTERTAINMENT - QCM
		{
			"ID":       "2",
			"QUESTION": "Quel acteur joue Iron Man dans les films Marvel?",
			"ANSWER":   "Robert Downey Jr.",
			"POINTS":   "10",
			"TIME":     "20",
			"TYPE":     "QCM",
			"CATEGORY": "ENTERTAINMENT",
			"QCM_ANSWERS": map[string]string{
				"RED":    "Chris Evans",
				"GREEN":  "Robert Downey Jr.",
				"YELLOW": "Chris Hemsworth",
				"BLUE":   "Mark Ruffalo",
			},
			"QCM_CORRECT":   "GREEN",
			"POINTS_TARGET": "TEAM",
			"ORDER":         1,
		},
		// HISTORY
		{
			"ID":            "3",
			"QUESTION":      "En quelle annee a eu lieu la Revolution francaise?",
			"ANSWER":        "1789",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "NORMAL",
			"CATEGORY":      "HISTORY",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         2,
		},
		// ARTS - QCM
		{
			"ID":       "4",
			"QUESTION": "Qui a peint la Joconde?",
			"ANSWER":   "Leonard de Vinci",
			"POINTS":   "10",
			"TIME":     "20",
			"TYPE":     "QCM",
			"CATEGORY": "ARTS",
			"QCM_ANSWERS": map[string]string{
				"RED":    "Michel-Ange",
				"GREEN":  "Leonard de Vinci",
				"YELLOW": "Raphael",
				"BLUE":   "Botticelli",
			},
			"QCM_CORRECT":   "GREEN",
			"POINTS_TARGET": "TEAM",
			"ORDER":         3,
		},
		// SCIENCE
		{
			"ID":            "5",
			"QUESTION":      "Quel est le symbole chimique de l'or?",
			"ANSWER":        "Au",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "NORMAL",
			"CATEGORY":      "SCIENCE",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         4,
		},
		// SPORTS - QCM
		{
			"ID":       "6",
			"QUESTION": "Dans quel sport utilise-t-on un volant?",
			"ANSWER":   "Badminton",
			"POINTS":   "10",
			"TIME":     "20",
			"TYPE":     "QCM",
			"CATEGORY": "SPORTS",
			"QCM_ANSWERS": map[string]string{
				"RED":    "Tennis",
				"GREEN":  "Badminton",
				"YELLOW": "Squash",
				"BLUE":   "Ping-pong",
			},
			"QCM_CORRECT":   "GREEN",
			"POINTS_TARGET": "TEAM",
			"ORDER":         5,
		},
		// FOOD
		{
			"ID":            "7",
			"QUESTION":      "De quel pays vient le sushi?",
			"ANSWER":        "Japon",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "NORMAL",
			"CATEGORY":      "FOOD",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         6,
		},
		// ANIMALS - QCM
		{
			"ID":       "8",
			"QUESTION": "Quel est le plus grand animal terrestre?",
			"ANSWER":   "Elephant d'Afrique",
			"POINTS":   "10",
			"TIME":     "20",
			"TYPE":     "QCM",
			"CATEGORY": "ANIMALS",
			"QCM_ANSWERS": map[string]string{
				"RED":    "Girafe",
				"GREEN":  "Elephant d'Afrique",
				"YELLOW": "Rhinoceros",
				"BLUE":   "Hippopotame",
			},
			"QCM_CORRECT":   "GREEN",
			"POINTS_TARGET": "TEAM",
			"ORDER":         7,
		},
	}

	for _, q := range testQuestions {
		id := q["ID"].(string)
		questionDir := filepath.Join(questionsDir, id)
		os.MkdirAll(questionDir, 0755)

		data, err := json.MarshalIndent(q, "", "  ")
		if err != nil {
			server.LogError(game.LogComponentApp, "Failed to marshal question %s: %v", id, err)
			continue
		}

		questionFile := filepath.Join(questionDir, "question.json")
		if err := os.WriteFile(questionFile, data, 0644); err != nil {
			server.LogError(game.LogComponentApp, "Failed to write question %s: %v", id, err)
			continue
		}
	}

	server.LogInfo(game.LogComponentApp, "Created %d test questions", len(testQuestions))
}

// loadDemoData creates comprehensive demo data for showcasing all features
func (a *App) loadDemoData() {
	server.LogInfo(game.LogComponentApp, "Loading demo data...")

	// Clear existing data first
	a.engine.ClearHistory()

	// 6 teams with distinct colors and pre-filled TeamPoints
	teams := map[string]*game.Team{
		"Les Rouges": {
			Name:       "Les Rouges",
			Color:      []int{239, 68, 68}, // Red
			Score:      0,
			TeamPoints: 30, // Pre-filled for podium display
		},
		"Les Bleus": {
			Name:       "Les Bleus",
			Color:      []int{59, 130, 246}, // Blue
			Score:      0,
			TeamPoints: 45,
		},
		"Les Verts": {
			Name:       "Les Verts",
			Color:      []int{34, 197, 94}, // Green
			Score:      0,
			TeamPoints: 25,
		},
		"Les Jaunes": {
			Name:       "Les Jaunes",
			Color:      []int{234, 179, 8}, // Yellow
			Score:      0,
			TeamPoints: 35,
		},
		"Les Violets": {
			Name:       "Les Violets",
			Color:      []int{168, 85, 247}, // Purple
			Score:      0,
			TeamPoints: 20,
		},
		"Les Oranges": {
			Name:       "Les Oranges",
			Color:      []int{249, 115, 22}, // Orange
			Score:      0,
			TeamPoints: 40,
		},
	}

	// Players: 4 per team with all 4 QCM answer colors (RED=A, GREEN=B, YELLOW=C, BLUE=D)
	bumpers := map[string]*game.Bumper{
		// Les Rouges - 4 joueurs
		"DEMO:AA:BB:CC:01": {Name: "Alice", Team: "Les Rouges", Score: 12, Version: "DEMO", AnswerColor: game.AnswerColorRed},
		"DEMO:AA:BB:CC:02": {Name: "Antoine", Team: "Les Rouges", Score: 8, Version: "DEMO", AnswerColor: game.AnswerColorGreen},
		"DEMO:AA:BB:CC:03": {Name: "Amelie", Team: "Les Rouges", Score: 6, Version: "DEMO", AnswerColor: game.AnswerColorYellow},
		"DEMO:AA:BB:CC:04": {Name: "Arthur", Team: "Les Rouges", Score: 4, Version: "DEMO", AnswerColor: game.AnswerColorBlue},
		// Les Bleus - 4 joueurs
		"DEMO:AA:BB:CC:05": {Name: "Bruno", Team: "Les Bleus", Score: 15, Version: "DEMO", AnswerColor: game.AnswerColorRed},
		"DEMO:AA:BB:CC:06": {Name: "Brigitte", Team: "Les Bleus", Score: 10, Version: "DEMO", AnswerColor: game.AnswerColorGreen},
		"DEMO:AA:BB:CC:07": {Name: "Baptiste", Team: "Les Bleus", Score: 7, Version: "DEMO", AnswerColor: game.AnswerColorYellow},
		"DEMO:AA:BB:CC:08": {Name: "Berenice", Team: "Les Bleus", Score: 5, Version: "DEMO", AnswerColor: game.AnswerColorBlue},
		// Les Verts - 4 joueurs
		"DEMO:AA:BB:CC:09": {Name: "Clara", Team: "Les Verts", Score: 9, Version: "DEMO", AnswerColor: game.AnswerColorRed},
		"DEMO:AA:BB:CC:10": {Name: "Cedric", Team: "Les Verts", Score: 7, Version: "DEMO", AnswerColor: game.AnswerColorGreen},
		"DEMO:AA:BB:CC:11": {Name: "Camille", Team: "Les Verts", Score: 5, Version: "DEMO", AnswerColor: game.AnswerColorYellow},
		"DEMO:AA:BB:CC:12": {Name: "Cyril", Team: "Les Verts", Score: 3, Version: "DEMO", AnswerColor: game.AnswerColorBlue},
		// Les Jaunes - 4 joueurs
		"DEMO:AA:BB:CC:13": {Name: "David", Team: "Les Jaunes", Score: 11, Version: "DEMO", AnswerColor: game.AnswerColorRed},
		"DEMO:AA:BB:CC:14": {Name: "Delphine", Team: "Les Jaunes", Score: 9, Version: "DEMO", AnswerColor: game.AnswerColorGreen},
		"DEMO:AA:BB:CC:15": {Name: "Dylan", Team: "Les Jaunes", Score: 6, Version: "DEMO", AnswerColor: game.AnswerColorYellow},
		"DEMO:AA:BB:CC:16": {Name: "Diane", Team: "Les Jaunes", Score: 4, Version: "DEMO", AnswerColor: game.AnswerColorBlue},
		// Les Violets - 4 joueurs
		"DEMO:AA:BB:CC:17": {Name: "Emma", Team: "Les Violets", Score: 8, Version: "DEMO", AnswerColor: game.AnswerColorRed},
		"DEMO:AA:BB:CC:18": {Name: "Ethan", Team: "Les Violets", Score: 6, Version: "DEMO", AnswerColor: game.AnswerColorGreen},
		"DEMO:AA:BB:CC:19": {Name: "Eva", Team: "Les Violets", Score: 4, Version: "DEMO", AnswerColor: game.AnswerColorYellow},
		"DEMO:AA:BB:CC:20": {Name: "Eliot", Team: "Les Violets", Score: 2, Version: "DEMO", AnswerColor: game.AnswerColorBlue},
		// Les Oranges - 4 joueurs
		"DEMO:AA:BB:CC:21": {Name: "Felix", Team: "Les Oranges", Score: 13, Version: "DEMO", AnswerColor: game.AnswerColorRed},
		"DEMO:AA:BB:CC:22": {Name: "Fanny", Team: "Les Oranges", Score: 10, Version: "DEMO", AnswerColor: game.AnswerColorGreen},
		"DEMO:AA:BB:CC:23": {Name: "Florian", Team: "Les Oranges", Score: 7, Version: "DEMO", AnswerColor: game.AnswerColorYellow},
		"DEMO:AA:BB:CC:24": {Name: "Flore", Team: "Les Oranges", Score: 5, Version: "DEMO", AnswerColor: game.AnswerColorBlue},
	}

	a.engine.SetTeams(teams)
	a.engine.SetBumpers(bumpers)
	a.engine.RecalculateAllTeamScores()

	// Create demo questions
	a.createDemoQuestions()

	// Create demo backgrounds with varied opacities
	a.createDemoBackgrounds()

	// Create demo history events for PALMARES
	a.createDemoHistory()

	server.LogInfo(game.LogComponentApp, "Demo data loaded: %d teams, %d players", len(teams), len(bumpers))
}

// createDemoQuestions creates diverse demo questions
func (a *App) createDemoQuestions() {
	questionsDir := a.config.Storage.QuestionsDir
	if questionsDir == "" {
		questionsDir = "./data/files/questions"
	}

	// Clear existing questions
	os.RemoveAll(questionsDir)
	os.MkdirAll(questionsDir, 0755)

	demoQuestions := []map[string]interface{}{
		// GEOGRAPHY - QCM with hints + image
		{
			"ID":                    "demo1",
			"QUESTION":              "Quelle est la capitale de l'Australie?",
			"ANSWER":                "Canberra",
			"POINTS":                "10",
			"TIME":                  "20",
			"TYPE":                  "QCM",
			"CATEGORY":              "GEOGRAPHY",
			"POINTS_TARGET":         "TEAM",
			"QCM_HINTS_ENABLED":     true,
			"QCM_HINT_THRESHOLD_1":  0.25,
			"QCM_HINT_THRESHOLD_2":  0.125,
			"QCM_PENALTY_1":         0.67,
			"QCM_PENALTY_2":         0.33,
			"QCM_ANSWERS": map[string]string{
				"RED":    "Sydney",
				"GREEN":  "Canberra",
				"YELLOW": "Melbourne",
				"BLUE":   "Brisbane",
			},
			"QCM_CORRECT": "GREEN",
			"ORDER":       0,
			"MEDIA":       "/question/demo1/media.jpg",
		},
		// ENTERTAINMENT - Normal question
		{
			"ID":            "demo2",
			"QUESTION":      "Quel acteur joue Iron Man dans les films Marvel?",
			"ANSWER":        "Robert Downey Jr.",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "NORMAL",
			"CATEGORY":      "ENTERTAINMENT",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         1,
		},
		// HISTORY - QCM with hints
		{
			"ID":                    "demo3",
			"QUESTION":              "En quelle annee a debute la Premiere Guerre mondiale?",
			"ANSWER":                "1914",
			"POINTS":                "15",
			"TIME":                  "25",
			"TYPE":                  "QCM",
			"CATEGORY":              "HISTORY",
			"POINTS_TARGET":         "TEAM",
			"QCM_HINTS_ENABLED":     true,
			"QCM_ANSWERS": map[string]string{
				"RED":    "1912",
				"GREEN":  "1914",
				"YELLOW": "1916",
				"BLUE":   "1918",
			},
			"QCM_CORRECT": "GREEN",
			"ORDER":       2,
		},
		// SCIENCE - Normal question + images
		{
			"ID":            "demo4",
			"QUESTION":      "Quel est le symbole chimique de l'or?",
			"ANSWER":        "Au",
			"POINTS":        "10",
			"TIME":          "20",
			"TYPE":          "NORMAL",
			"CATEGORY":      "SCIENCE",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         3,
			"MEDIA":         "/question/demo4/media.jpg",
			"MEDIA_ANSWER":  "/question/demo4/media_answer.jpg",
		},
		// SPORTS - QCM
		{
			"ID":       "demo5",
			"QUESTION": "Combien de joueurs composent une equipe de football?",
			"ANSWER":   "11",
			"POINTS":   "10",
			"TIME":     "15",
			"TYPE":     "QCM",
			"CATEGORY": "SPORTS",
			"QCM_ANSWERS": map[string]string{
				"RED":    "9",
				"GREEN":  "11",
				"YELLOW": "13",
				"BLUE":   "15",
			},
			"QCM_CORRECT":   "GREEN",
			"POINTS_TARGET": "TEAM",
			"ORDER":         4,
		},
		// ARTS - Normal question
		{
			"ID":            "demo6",
			"QUESTION":      "Qui a peint la Joconde?",
			"ANSWER":        "Leonard de Vinci",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "NORMAL",
			"CATEGORY":      "ARTS",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         5,
		},
		// FOOD - QCM with hints + images
		{
			"ID":                    "demo7",
			"QUESTION":              "De quel pays vient la pizza?",
			"ANSWER":                "Italie",
			"POINTS":                "10",
			"TIME":                  "20",
			"TYPE":                  "QCM",
			"CATEGORY":              "FOOD",
			"POINTS_TARGET":         "TEAM",
			"QCM_HINTS_ENABLED":     true,
			"QCM_ANSWERS": map[string]string{
				"RED":    "France",
				"GREEN":  "Italie",
				"YELLOW": "Espagne",
				"BLUE":   "Grece",
			},
			"QCM_CORRECT":  "GREEN",
			"ORDER":        6,
			"MEDIA":        "/question/demo7/media.jpg",
			"MEDIA_ANSWER": "/question/demo7/media_answer.jpg",
		},
		// ANIMALS - Normal
		{
			"ID":            "demo8",
			"QUESTION":      "Quel est le plus grand animal terrestre?",
			"ANSWER":        "L'elephant d'Afrique",
			"POINTS":        "10",
			"TIME":          "30",
			"TYPE":          "NORMAL",
			"CATEGORY":      "ANIMALS",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         7,
		},
		// GEOGRAPHY - MEMORY game
		{
			"ID":            "demo9",
			"QUESTION":      "Associez les pays a leurs capitales",
			"ANSWER":        "",
			"POINTS":        "0",
			"TIME":          "90",
			"TYPE":          "MEMORY",
			"CATEGORY":      "GEOGRAPHY",
			"POINTS_TARGET": "TEAM",
			"MEMORY_PAIRS": []map[string]interface{}{
				{
					"ID":    1,
					"CARD1": map[string]interface{}{"TEXT": "France", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Paris", "IS_IMAGE": false},
				},
				{
					"ID":    2,
					"CARD1": map[string]interface{}{"TEXT": "Espagne", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Madrid", "IS_IMAGE": false},
				},
				{
					"ID":    3,
					"CARD1": map[string]interface{}{"TEXT": "Allemagne", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Berlin", "IS_IMAGE": false},
				},
				{
					"ID":    4,
					"CARD1": map[string]interface{}{"TEXT": "Italie", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Rome", "IS_IMAGE": false},
				},
			},
			"MEMORY_CONFIG": map[string]interface{}{
				"FLIP_DELAY":           3.0,
				"POINTS_PER_PAIR":      10,
				"ERROR_PENALTY":        0,
				"COMPLETION_BONUS":     20,
				"USE_TIMER":            true,
				"MEMORIZE_TIME":        5,
				"SHOW_DURING_MEMORIZE": true,
				"REVEAL_DELAY":         0.5,
			},
			"ORDER": 8,
		},
		// ENTERTAINMENT - MEMORY game
		{
			"ID":            "demo10",
			"QUESTION":      "Associez les superheros a leurs pouvoirs",
			"ANSWER":        "",
			"POINTS":        "0",
			"TIME":          "120",
			"TYPE":          "MEMORY",
			"CATEGORY":      "ENTERTAINMENT",
			"POINTS_TARGET": "TEAM",
			"MEMORY_PAIRS": []map[string]interface{}{
				{
					"ID":    1,
					"CARD1": map[string]interface{}{"TEXT": "Superman", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Vol", "IS_IMAGE": false},
				},
				{
					"ID":    2,
					"CARD1": map[string]interface{}{"TEXT": "Spider-Man", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Toiles", "IS_IMAGE": false},
				},
				{
					"ID":    3,
					"CARD1": map[string]interface{}{"TEXT": "Flash", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Vitesse", "IS_IMAGE": false},
				},
				{
					"ID":    4,
					"CARD1": map[string]interface{}{"TEXT": "Aquaman", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Eau", "IS_IMAGE": false},
				},
				{
					"ID":    5,
					"CARD1": map[string]interface{}{"TEXT": "Hulk", "IS_IMAGE": false},
					"CARD2": map[string]interface{}{"TEXT": "Force", "IS_IMAGE": false},
				},
			},
			"MEMORY_CONFIG": map[string]interface{}{
				"FLIP_DELAY":           2.0,
				"POINTS_PER_PAIR":      15,
				"ERROR_PENALTY":        5,
				"COMPLETION_BONUS":     30,
				"USE_TIMER":            true,
				"MEMORIZE_TIME":        7,
				"SHOW_DURING_MEMORIZE": true,
				"REVEAL_DELAY":         0.5,
			},
			"ORDER": 9,
		},
	}

	for _, q := range demoQuestions {
		id := q["ID"].(string)
		questionDir := filepath.Join(questionsDir, id)
		os.MkdirAll(questionDir, 0755)

		data, err := json.MarshalIndent(q, "", "  ")
		if err != nil {
			server.LogError(game.LogComponentApp, "Failed to marshal demo question %s: %v", id, err)
			continue
		}

		questionFile := filepath.Join(questionDir, "question.json")
		if err := os.WriteFile(questionFile, data, 0644); err != nil {
			server.LogError(game.LogComponentApp, "Failed to write demo question %s: %v", id, err)
			continue
		}
	}

	// Extract demo question images from embedded assets
	demoImages := []struct {
		questionID  string
		assetName   string
		destName    string
	}{
		{"demo1", "demo1_australia.jpg", "media.jpg"},
		{"demo4", "demo4_gold_miner.jpg", "media.jpg"},
		{"demo4", "demo4_periodic_table.jpg", "media_answer.jpg"},
		{"demo7", "demo7_pizza.jpg", "media.jpg"},
		{"demo7", "demo7_italy.jpg", "media_answer.jpg"},
	}

	for _, img := range demoImages {
		questionDir := filepath.Join(questionsDir, img.questionID)
		destPath := filepath.Join(questionDir, img.destName)

		data, err := assets.DemoAssets.ReadFile("demo/" + img.assetName)
		if err != nil {
			server.LogError(game.LogComponentApp, "Failed to read embedded %s: %v", img.assetName, err)
			continue
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			server.LogError(game.LogComponentApp, "Failed to write %s: %v", destPath, err)
			continue
		}
		server.LogDebug(game.LogComponentApp, "Extracted demo image: %s -> %s", img.assetName, destPath)
	}

	server.LogInfo(game.LogComponentApp, "Created %d demo questions with images", len(demoQuestions))
}

// createDemoBackgrounds creates demo backgrounds with varied opacities
func (a *App) createDemoBackgrounds() {
	filesDir := a.config.Storage.FilesDir
	if filesDir == "" {
		filesDir = "./data/files"
	}
	bgDir := filepath.Join(filesDir, "backgrounds")
	os.MkdirAll(bgDir, 0755)

	// Demo backgrounds: extract from embedded assets
	demoImages := []struct {
		filename string
		duration int
		opacity  float64
	}{
		{"demo_bg_1.jpg", 8, 100},
		{"demo_bg_2.jpg", 12, 80},
		{"demo_bg_3.jpg", 10, 60},
	}

	var backgrounds []game.Background
	for _, img := range demoImages {
		destPath := filepath.Join(bgDir, img.filename)
		// Extract from embedded assets if not exists
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			server.LogDebug(game.LogComponentApp, "Extracting demo background: %s", img.filename)
			data, err := assets.DemoAssets.ReadFile("demo/" + img.filename)
			if err != nil {
				server.LogError(game.LogComponentApp, "Failed to read embedded %s: %v", img.filename, err)
				continue
			}
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				server.LogError(game.LogComponentApp, "Failed to write %s: %v", img.filename, err)
				continue
			}
		}
		backgrounds = append(backgrounds, game.Background{
			Path:     "/files/backgrounds/" + img.filename,
			Duration: img.duration,
			Opacity:  img.opacity,
		})
	}

	a.engine.SetBackgrounds(backgrounds)
	a.saveBackgroundsConfig()
	server.LogInfo(game.LogComponentApp, "Created %d demo backgrounds", len(backgrounds))
}

// createDemoHistory creates demo history events for PALMARES view
func (a *App) createDemoHistory() {
	baseTime := time.Now().Add(-1 * time.Hour).UnixMicro()

	// Create events for different categories and teams
	events := []game.GameEvent{
		// GEOGRAPHY events
		{
			Timestamp:        baseTime,
			QuestionID:       "demo1",
			QuestionText:     "Quelle est la capitale de l'Australie?",
			QuestionCategory: "GEOGRAPHY",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Bleus",
			TeamColor:        []int{59, 130, 246},
			Points:           10,
		},
		{
			Timestamp:        baseTime + 60000000,
			QuestionID:       "demo9",
			QuestionText:     "Associez les pays a leurs capitales",
			QuestionCategory: "GEOGRAPHY",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Oranges",
			TeamColor:        []int{249, 115, 22},
			Points:           40,
		},
		// ENTERTAINMENT events
		{
			Timestamp:        baseTime + 120000000,
			QuestionID:       "demo2",
			QuestionText:     "Quel acteur joue Iron Man?",
			QuestionCategory: "ENTERTAINMENT",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "PLAYER",
			WinnerID:         "DEMO:AA:BB:CC:05",
			PlayerName:       "Bruno",
			TeamName:         "Les Bleus",
			TeamColor:        []int{59, 130, 246},
			Points:           10,
		},
		{
			Timestamp:        baseTime + 180000000,
			QuestionID:       "demo10",
			QuestionText:     "Associez les superheros a leurs pouvoirs",
			QuestionCategory: "ENTERTAINMENT",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Rouges",
			TeamColor:        []int{239, 68, 68},
			Points:           60,
		},
		// HISTORY events
		{
			Timestamp:        baseTime + 240000000,
			QuestionID:       "demo3",
			QuestionText:     "Debut de la Premiere Guerre mondiale?",
			QuestionCategory: "HISTORY",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Verts",
			TeamColor:        []int{34, 197, 94},
			Points:           15,
		},
		// SCIENCE events
		{
			Timestamp:        baseTime + 300000000,
			QuestionID:       "demo4",
			QuestionText:     "Symbole chimique de l'or?",
			QuestionCategory: "SCIENCE",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "PLAYER",
			WinnerID:         "DEMO:AA:BB:CC:13",
			PlayerName:       "David",
			TeamName:         "Les Jaunes",
			TeamColor:        []int{234, 179, 8},
			Points:           10,
		},
		// SPORTS events
		{
			Timestamp:        baseTime + 360000000,
			QuestionID:       "demo5",
			QuestionText:     "Joueurs dans une equipe de football?",
			QuestionCategory: "SPORTS",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Violets",
			TeamColor:        []int{168, 85, 247},
			Points:           10,
		},
		// ARTS events
		{
			Timestamp:        baseTime + 420000000,
			QuestionID:       "demo6",
			QuestionText:     "Qui a peint la Joconde?",
			QuestionCategory: "ARTS",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "PLAYER",
			WinnerID:         "DEMO:AA:BB:CC:21",
			PlayerName:       "Felix",
			TeamName:         "Les Oranges",
			TeamColor:        []int{249, 115, 22},
			Points:           10,
		},
		// FOOD events
		{
			Timestamp:        baseTime + 480000000,
			QuestionID:       "demo7",
			QuestionText:     "De quel pays vient la pizza?",
			QuestionCategory: "FOOD",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "TEAM",
			TeamName:         "Les Jaunes",
			TeamColor:        []int{234, 179, 8},
			Points:           7,
		},
		// ANIMALS events
		{
			Timestamp:        baseTime + 540000000,
			QuestionID:       "demo8",
			QuestionText:     "Plus grand animal terrestre?",
			QuestionCategory: "ANIMALS",
			EventType:        "POINTS_AWARDED",
			WinnerType:       "PLAYER",
			WinnerID:         "DEMO:AA:BB:CC:01",
			PlayerName:       "Alice",
			TeamName:         "Les Rouges",
			TeamColor:        []int{239, 68, 68},
			Points:           10,
		},
	}

	for _, event := range events {
		a.engine.AddGameEvent(event)
	}

	server.LogInfo(game.LogComponentApp, "Created %d demo history events", len(events))
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
