package main

import (
	"buzzcontrol/assets"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"buzzcontrol/web"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
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
	udpBcast    *server.UDPBroadcaster
	broadcaster *server.BroadcasterManager
	httpServer  *server.HTTPServer
	wsHub       *server.WebSocketHub
	buzzerHub   *server.BuzzerWebSocketHub
	logsHub     *server.LogsWebSocketHub
	mdnsServer  *server.MDNSServer
	dnsServer   *server.DNSServer
	logger      *server.BroadcastLogger
	ackManager  *server.AckManager // ACK tracking for priority buzzer messages (v3.8.0)
	// evictionRegistry remembers why a VJoueur was recently removed (PLAYER_REMOVED
	// or GAME_RESET) so a later PLAYER_CONNECT with that now-unknown ID gets the
	// real reason instead of a generic ENROLLMENT_CLOSED guess (#123 B3).
	evictionRegistry *server.EvictionRegistry
	ctx              context.Context    // Application-lifetime context
	cancelCtx        context.CancelFunc // Cancels ctx on shutdown
	// bumperLEDState tracks the last LED_SET payload sent to each buzzer (by MAC).
	// Used by resendLEDOnReconnect to restore LED state when a buzzer reconnects.
	bumperLEDState map[string]protocol.LEDSetPayload

	// bumperBuzzState tracks the BuzzState (NONE/MOI/EQUIPE/AUTRE) for each buzzer
	// within the current question round. Reset on READY/PREPARE; updated on each buzz.
	bumperBuzzState map[string]game.BuzzState

	// ardoiseCoalescer collapses a burst of ARDOISE_INPUT-triggered admin
	// UPDATEs (#129 T2.1/T2.2) into at most one per ≤150ms window — see
	// broadcast_coalescer.go. Flushed immediately on every phase change
	// (OnStateChange) and stopped on server shutdown (a.stop()).
	ardoiseCoalescer *BroadcastCoalescer
}

// resolvePort returns the effective HTTP port.
// If flagPort > 0 it takes precedence over configPort (--port CLI flag).
func resolvePort(configPort, flagPort int) int {
	if flagPort > 0 {
		return flagPort
	}
	return configPort
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("=== BuzzControl Server (Go) ===")

	// Parse CLI flags — must happen before any flag.Xxx() call
	portFlag := flag.Int("port", 0, "HTTP port to listen on (overrides config.json)")
	flag.Parse()

	// Check for Bonjour/mDNS support
	checkBonjourSupport()

	// Load configuration
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Printf("Using default configuration: %v", err)
		cfg = config.Get()
	}

	// --port flag overrides config.json when provided
	cfg.Server.HTTPPort = resolvePort(cfg.Server.HTTPPort, *portFlag)

	// Override version with embedded value (set via -ldflags at build time)
	if Version != "dev" {
		cfg.Version = Version
	}
	config.SetInstance(cfg)

	log.Printf("Version: %s (embedded: %s)", cfg.Version, Version)
	log.Printf("HTTP Port: %d", cfg.Server.HTTPPort)

	// Create app with an application-lifetime context
	appCtx, appCancel := context.WithCancel(context.Background())
	app := &App{
		config:          cfg,
		bumperLEDState:  make(map[string]protocol.LEDSetPayload),
		bumperBuzzState: make(map[string]game.BuzzState),
		ctx:             appCtx,
		cancelCtx:       appCancel,
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

	// Recalculate IS_OUTDATED for bumpers loaded from disk.
	// Use FirmwareVersion if available; fall back to Version (older buzzers store only VERSION).
	if bumpersLoaded {
		fm := app.httpServer.GetFirmwareManager()
		for id, bumper := range app.engine.GetTeamsAndBumpers().Bumpers {
			vToCheck := bumper.FirmwareVersion
			if vToCheck == "" {
				vToCheck = bumper.Version
			}
			if vToCheck != "" {
				app.engine.UpdateBumper(id, map[string]interface{}{"IS_OUTDATED": fm.IsOutdated(vToCheck)})
			}
		}
	}

	// Reset CONNECTED=false for all bumpers loaded from disk (v3.6.7).
	// After a server restart, no buzzer is physically connected yet, even if
	// CONNECTED=true was persisted from the previous session.
	if bumpersLoaded {
		for id := range app.engine.GetTeamsAndBumpers().Bumpers {
			app.engine.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
		}
	}

	// Initialize broadcaster frequency based on persisted bumpers: any bumper loaded
	// from disk is by definition not yet connected (server just started), so we activate
	// high-frequency mode immediately to help them rediscover the server quickly.
	app.updateBroadcasterFrequency()

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

	// Display all accessible URLs and open browser if enabled
	displayAndOpenURLs(cfg.Server.HTTPPort, cfg.Server.AutoOpenBrowsers, cfg.Server.Debug)

	// Wait for shutdown signal.
	// os.Interrupt catches CTRL_CLOSE_EVENT on Windows (console window closed)
	// in addition to the standard SIGINT / SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
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

	// WebSocket hub (web clients: admin/TV/VPlayer)
	a.wsHub = server.NewWebSocketHub()

	// ARDOISE_INPUT broadcast coalescer (#129 T2.1/T2.2) — collapses a burst
	// of admin UPDATEs into at most one per ardoiseCoalesceWindow. The emit
	// closure reads live state via broadcastUpdateTo at the moment it
	// actually fires (never a buffered payload — see BroadcastCoalescer's
	// doc comment for why that's what makes this safe).
	a.ardoiseCoalescer = NewBroadcastCoalescer(ardoiseCoalesceWindow, func() {
		a.broadcastUpdateTo(server.ClientTypeAdmin)
	})

	// Eviction reason registry (#123 B3) — short-lived, bounded
	a.evictionRegistry = server.NewEvictionRegistry()

	// Buzzer WebSocket hub (physical buzzers via /ws/buzzer)
	a.buzzerHub = server.NewBuzzerWebSocketHub()

	// Logs WebSocket hub (dedicated for logs)
	a.logsHub = server.NewLogsWebSocketHub(1000)

	// Initialize global logger (singleton)
	a.logger = server.InitLogger(1000)
	a.logger.SetDebugEnabled(a.config.Server.Debug)

	// UDP broadcaster
	a.udpBcast = server.NewUDPBroadcaster()

	// BroadcasterManager: periodic BUZZ_SERVER heartbeat for automatic IP discovery
	a.broadcaster = server.NewBroadcasterManager(a.udpBcast, a.config.Server.HTTPPort)

	// HTTP server
	a.httpServer = server.NewHTTPServer(a.config.Server.HTTPPort, a.engine, a.wsHub, a.buzzerHub, a.logsHub)

	// Extract embedded firmware (CI builds only - skipped when version.txt is "0.0.0")
	if err := a.httpServer.GetFirmwareManager().InitFromEmbedded(assets.FirmwareAssets); err != nil {
		server.LogError(game.LogComponentApp, "Failed to init embedded firmware: %v", err)
	}
	// Store embedded FS reference so restore-embedded can re-extract it on demand.
	a.httpServer.GetFirmwareManager().SetEmbeddedFS(assets.FirmwareAssets)

	// Set embedded default question image (SVG fallback when no custom image uploaded)
	a.httpServer.SetDefaultQuestionImageAsset(assets.DefaultQuestionImage)

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

	// ACK manager for priority buzzer messages (v3.8.0)
	a.ackManager = server.NewAckManager(&a.config.Server)
	a.ackManager.OnRetry = func(mac, msgID string) {
		// Resend the LED payload with the ORIGINAL msgID so the buzzer can confirm the right entry.
		// Never call sendLEDSet() here — that generates a new msgID, leaving the original entry
		// permanently unconfirmable and cascading into AckMaxRetries² slots at most.
		if payload, ok := a.bumperLEDState[mac]; ok {
			msg, err := protocol.NewMessage(protocol.ActionLEDSet, payload)
			if err != nil {
				server.LogError(game.LogComponentApp, "AckManager retry: failed to create LED_SET for %s: %v", mac, err)
				return
			}
			msg.MsgID = msgID // Original msgID, not a new one
			if !a.buzzerHub.IsClientConnected(mac) {
				// #109 Phase 2 (D4): an unacknowledged LED_SET being retried against a
				// now-disconnected buzzer is itself a lost message.
				a.engine.TransitionConn(mac, game.ConnEventMessageLost)
			}
			if err := a.buzzerHub.SendToClient(mac, msg); err != nil {
				server.LogWarn(game.LogComponentApp, "AckManager retry: failed to resend LED_SET to %s: %v", mac, err)
			}
		}
	}
	a.ackManager.OnExpired = func(mac, msgID string) {
		// Clear AckPending flag after max retries exhausted
		a.engine.UpdateBumper(mac, map[string]interface{}{"ACK_PENDING": false})
		a.broadcastUpdate()
	}

	// mDNS server (advertise buzzcontrol.local)
	a.mdnsServer = server.NewMDNSServer("buzzcontrol", a.config.Server.HTTPPort, server.BuzzerDiscoveryPort)

	// DNS server (captive portal - redirects all DNS to this server)
	a.dnsServer = server.NewDNSServer(53, nil)

	// Set up callbacks
	a.setupCallbacks()
}

func (a *App) setupCallbacks() {
	// Handle WebSocket messages from web clients.
	//
	// Bugfix #131 (plan risk R3): this is the SOLE consumer of a.wsHub.Incoming
	// for the whole process — a panic that kills this goroutine silently
	// deafens every admin/TV/VPlayer client, worse than the crash it would
	// otherwise cause. The recover() is placed INSIDE the loop, wrapping only
	// the single a.handleWebMessage(msg) call, specifically so the loop
	// itself survives and keeps consuming the channel after a panic — a
	// recover() wrapped AROUND the `for` (one defer for the whole goroutine)
	// would still stop the crash, but the goroutine would exit right after
	// recovering from the first panic, and every message sent afterward
	// would sit unconsumed in the channel forever (see
	// dispatch_panic_recovery_test.go for the regression test this guards).
	go func() {
		for msg := range a.wsHub.Incoming {
			a.handleWebMessage(msg)
		}
	}()

	// Handle WebSocket messages from buzzers (/ws/buzzer) — same reasoning
	// as above, mirrored for the buzzer-side single dispatch goroutine.
	go func() {
		for msg := range a.buzzerHub.Incoming {
			a.handleBuzzerMessage(msg)
		}
	}()

	// Game state changes
	a.engine.OnStateChange = func(phase game.GamePhase) {
		// #129 T2.2 / CA5 / CA6: flush any pending ARDOISE_INPUT coalescing
		// BEFORE the phase-change broadcasts below, so the admin sees the
		// last keystrokes at their own timestamp rather than have them
		// silently subsumed into (and indistinguishable from) the
		// phase-change UPDATE that follows.
		a.ardoiseCoalescer.Flush()
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

	// NEW_GAME background change handler (v4.0.4)
	a.httpServer.OnNewGameBackgroundChange = func(action string) {
		if action == "save" {
			// Config was updated via PUT, just save
			a.saveNewGameBackgroundsConfig()
		} else {
			// Files changed (upload/delete), reload from disk
			a.loadNewGameBackgrounds()
			a.saveNewGameBackgroundsConfig()
		}
		a.broadcastUpdate()
		a.broadcastConfigUpdate()
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

	// WiFi config broadcast handler (triggered by POST /api/buzzer/wifi-config)
	a.httpServer.OnBuzzerWifiConfig = func() int {
		a.broadcastWifiConfig()
		return a.buzzerHub.ConnectedCount()
	}

	// ACK callback for priority OTA_UPDATE messages sent from http_firmware.go (v3.8.0)
	a.httpServer.OnPriorityMessageSent = func(mac, msgID, action string) {
		a.ackManager.Register(mac, msgID, action)
		a.engine.UpdateBumper(mac, map[string]interface{}{"ACK_PENDING": true})
		a.broadcastUpdate()
	}

	// Detect existing backgrounds on startup
	a.loadBackgrounds()
	a.loadNewGameBackgrounds()

	// Ensure categories directory exists and detect initial network state
	a.ensureCategoriesDir()
	a.updateNetworkState()

	// Handle client count changes (WebSocket connect/disconnect)
	a.wsHub.OnClientChange = func(adminCount, tvCount, vplayerCount int) {
		a.broadcastClientCounts()
	}

	// Handle buzzer WebSocket connect/disconnect (count-based)
	a.buzzerHub.OnBuzzerChange = func(buzzerCount int) {
		a.broadcastClientCounts()
	}

	// Handle individual buzzer disconnection (v3.6.5): set CONNECTED=false and
	// adjust broadcaster frequency so disconnected buzzers rediscover faster.
	// Guard: if the buzzer already reconnected (same MAC, new WS connection), skip
	// the CONNECTED=false to avoid a badge flash caused by the zombie connection
	// timing out after the new connection is live.
	a.buzzerHub.OnBuzzerDisconnected = func(mac string) {
		if a.buzzerHub.IsClientConnected(mac) {
			return
		}
		a.engine.UpdateBumper(mac, map[string]interface{}{"CONNECTED": false, "ACK_PENDING": false})
		// Clean up any pending ACK entries for this buzzer to avoid stale retries
		cleared := a.ackManager.ClearByMAC(mac)
		if cleared > 0 {
			server.LogInfo(game.LogComponentApp, "AckManager: cleared %d pending ACK(s) for disconnected buzzer %s", cleared, mac)
		}
		a.broadcastUpdate()
		a.updateBroadcasterFrequency()
	}

	// Handle individual VPlayer disconnection (#109): set CONNECTED=false when a VJoueur's
	// WebSocket closes. Extracted as a named method (rather than inlined here) so it can be
	// unit-tested directly — see onPlayerDisconnected below.
	a.wsHub.OnPlayerDisconnected = a.onPlayerDisconnected

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

// onPlayerDisconnected handles an individual VJoueur's WebSocket closing
// (#109): sets CONNECTED=false so the connection badge reflects it.
//
// Guard 1 (anti-zombie, #109): if the VJoueur already reconnected (same
// PlayerID, new WS connection) before this fires, skip the CONNECTED=false to
// avoid a badge flash caused by the zombie connection unregistering after the
// new connection is already live.
//
// Guard 2 (ghost bumper, code-review finding after the NEW_GAME purge fix):
// if the bumper no longer exists — e.g. it was purged by NEW_GAME while this
// VJoueur was still connected, and its WebSocket only closes later — do
// nothing. UpdateBumper creates an empty bumper for any unknown ID (generic
// behavior meant for physical buzzer self-registration by MAC); without this
// guard it would resurrect a persisted, empty ghost bumper here.
func (a *App) onPlayerDisconnected(playerID string) {
	if a.wsHub.IsPlayerIDConnected(playerID) {
		return
	}
	if a.engine.GetBumper(playerID) == nil {
		return
	}
	a.engine.UpdateBumper(playerID, map[string]interface{}{"CONNECTED": false})
	// #129 T1.3: no VPlayer consumes another participant's disconnection —
	// sortedPlayers is gated !isVPlayer since #127. No targeted echo either:
	// the recipient concerned by this event just disconnected, there is no
	// one to echo to.
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeBuzzer)
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

// loadNewGameBackgrounds scans data/files/new-game-backgrounds/ and loads config (v4.0.4)
func (a *App) loadNewGameBackgrounds() {
	filesDir := a.config.Storage.FilesDir
	if filesDir == "" {
		filesDir = "./data/files"
	}
	bgDir := filepath.Join(filesDir, "new-game-backgrounds")
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

	backgrounds := []game.Background{}

	// Scan new-game-backgrounds directory
	entries, err := os.ReadDir(bgDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == "backgrounds.json" {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" {
				bgPath := "/files/new-game-backgrounds/" + entry.Name()
				if saved, ok := savedMap[bgPath]; ok {
					if saved.Opacity == 0 {
						saved.Opacity = 100
					}
					backgrounds = append(backgrounds, saved)
					delete(savedMap, bgPath)
				} else {
					backgrounds = append(backgrounds, game.Background{
						Path:     bgPath,
						Duration: 10,
						Opacity:  100,
					})
				}
			}
		}
	}

	a.engine.SetNewGameBackgrounds(backgrounds)
	if len(backgrounds) > 0 {
		server.LogInfo(game.LogComponentApp, "Loaded %d new-game-background(s)", len(backgrounds))
	}
}

// saveNewGameBackgroundsConfig persists backgrounds.json for new-game-backgrounds (v4.0.4)
func (a *App) saveNewGameBackgroundsConfig() {
	filesDir := a.config.Storage.FilesDir
	if filesDir == "" {
		filesDir = "./data/files"
	}
	bgDir := filepath.Join(filesDir, "new-game-backgrounds")
	configPath := filepath.Join(bgDir, "backgrounds.json")

	os.MkdirAll(bgDir, 0755)

	backgrounds := a.engine.GetNewGameBackgrounds()
	data, err := json.MarshalIndent(backgrounds, "", "  ")
	if err != nil {
		server.LogError(game.LogComponentApp, "Failed to marshal new-game-backgrounds config: %v", err)
		return
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		server.LogError(game.LogComponentApp, "Failed to save new-game-backgrounds config: %v", err)
	} else {
		server.LogDebug(game.LogComponentApp, "Saved new-game-backgrounds config")
	}
}

// ensureCategoriesDir creates data/files/categories/ if it does not exist (v5.6.2 — #95)
func (a *App) ensureCategoriesDir() {
	filesDir := a.config.Storage.FilesDir
	if filesDir == "" {
		filesDir = "./data/files"
	}
	catDir := filepath.Join(filesDir, "categories")
	if err := os.MkdirAll(catDir, 0755); err != nil {
		server.LogWarn(game.LogComponentApp, "Failed to create categories dir: %v", err)
	}
}

// updateNetworkState detects whether only loopback interfaces are active and updates the engine (v5.6.2 — #96)
func (a *App) updateNetworkState() {
	ips := server.GetServerIPs()
	onlyLocal := len(ips) == 0
	if onlyLocal != a.engine.GetNetworkOnlyLocalhost() {
		a.engine.SetNetworkOnlyLocalhost(onlyLocal)
		a.broadcastUpdate()
		server.LogInfo(game.LogComponentApp, "Network state updated: NETWORK_ONLY_LOCALHOST=%v (IPs=%v)", onlyLocal, ips)
	}
}

// networkWatchdog re-checks network state every 30s and broadcasts changes (v5.6.2 — #96)
func (a *App) networkWatchdog() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.updateNetworkState()
		case <-a.ctx.Done():
			return
		}
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
	// Start WebSocket hub (web clients)
	go a.wsHub.Run()

	// Start Buzzer WebSocket hub (physical buzzers)
	go a.buzzerHub.Run()

	// Start Logs WebSocket hub
	go a.logsHub.Run()

	// Start ACK manager background goroutine (v3.8.0); uses app context for clean shutdown.
	go a.ackManager.Start(a.ctx)

	// Start UDP broadcaster
	if err := a.udpBcast.Start(); err != nil {
		return err
	}
	a.logger.Info(game.LogComponentUDP, "UDP broadcaster started on port %d", server.BuzzerDiscoveryPort)

	// Start BUZZ_SERVER heartbeat broadcasts for automatic IP discovery
	a.broadcaster.Start()
	a.logger.Info(game.LogComponentUDP, "BroadcasterManager started (interval=5s, http_port=%d)", a.config.Server.HTTPPort)

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

	// Start network watchdog goroutine (re-checks every 30s)
	go a.networkWatchdog()

	a.logger.Info(game.LogComponentApp, "BuzzControl server v%s started successfully", a.config.Version)

	return nil
}

func (a *App) stop() {
	// Cancel the application context — stops the AckManager goroutine and any other ctx-aware components.
	if a.cancelCtx != nil {
		a.cancelCtx()
	}
	// #129 T2.2: stop any pending ARDOISE coalescer timer — no time.AfterFunc
	// goroutine left dangling after the rest of the server has stopped.
	if a.ardoiseCoalescer != nil {
		a.ardoiseCoalescer.Stop()
	}
	a.dnsServer.Stop()
	a.mdnsServer.Stop()
	a.httpServer.Stop()
	a.broadcaster.Stop()
	a.udpBcast.Stop()
}

// handleBuzzerMessage processes messages from BuzzClick buzzers (WebSocket).
//
// Bugfix #131 (plan risk R3): this is called from the SOLE consumer
// goroutine of a.buzzerHub.Incoming for the whole process (setupCallbacks)
// — a panic that kills that goroutine silently deafens every physical
// buzzer, worse than the crash it would otherwise cause. The recover() lives
// HERE, inside the per-message handler itself, rather than around the
// dispatch loop in setupCallbacks: that loop calls this function directly
// (`for msg := range a.buzzerHub.Incoming { a.handleBuzzerMessage(msg) }`),
// so putting the guard here protects it no matter where it's called from,
// and specifically means the loop survives and keeps consuming the channel
// after a panic — a recover() wrapped AROUND the `for` (one defer for the
// whole goroutine) would still stop the crash, but the goroutine would exit
// right after recovering from the first panic, and every message sent
// afterward would sit unconsumed in the channel forever. recover() is
// called directly in this deferred literal, as required for it to actually
// stop the panic — see server.LogRecoveredPanic's doc comment.
func (a *App) handleBuzzerMessage(incoming *protocol.IncomingMessage) {
	defer func() {
		if r := recover(); r != nil {
			server.LogRecoveredPanic(game.LogComponentApp, "handleBuzzerMessage clientID="+incoming.ClientID, r)
		}
	}()

	msg := incoming.Data

	switch msg.Action {
	case protocol.ActionHello:
		a.handleHello(incoming.ClientID, msg, incoming.Source)

	case protocol.ActionButton:
		a.handleButton(incoming.ClientID, msg, incoming.Timestamp.UnixMicro())

	case protocol.ActionPong:
		a.handlePong(incoming.ClientID, msg)

	case protocol.ActionOTAProgress:
		a.handleOTAProgress(incoming.ClientID, msg)

	case protocol.ActionACK:
		a.handleBuzzerACK(incoming.ClientID, msg)

	default:
		server.LogWarn(game.LogComponentApp, "Unknown buzzer action: %s", msg.Action)
	}
}

// handleBuzzerACK processes an ACK message from a buzzer.
// The buzzer sends this after receiving a priority message with a MSG_ID.
func (a *App) handleBuzzerACK(clientID string, msg *protocol.Message) {
	var payload protocol.AckPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogWarn(game.LogComponentApp, "ACK from %s: failed to parse payload: %v", clientID, err)
		return
	}
	if payload.AckID == "" {
		server.LogWarn(game.LogComponentApp, "ACK from %s: empty ack_id", clientID)
		return
	}

	mac := msg.ID
	if mac == "" {
		mac = clientID
	}

	confirmed := a.ackManager.Confirm(payload.AckID)
	if !confirmed {
		server.LogDebug(game.LogComponentApp, "ACK from %s for msgID=%s: not found (already confirmed or expired)", mac, payload.AckID)
		return
	}

	server.LogDebug(game.LogComponentApp, "ACK received from %s: action=%s msgID=%s", mac, payload.AckAction, payload.AckID)

	// Clear the AckPending flag on the bumper
	a.engine.UpdateBumper(mac, map[string]interface{}{"ACK_PENDING": false})
	// #109 Phase 2 (D1/D3): a real ACK is the buzzer's fidèle delivery confirmation —
	// closes the connection-badge "green" window (min. duration honored internally).
	a.engine.ConfirmDelivery(mac)
	a.broadcastUpdate()
}

// handleWebMessage processes messages from web clients (WebSocket).
//
// Bugfix #131 (plan risk R3) — see handleBuzzerMessage's doc comment
// immediately above for the full rationale (identical: this is the SOLE
// consumer of a.wsHub.Incoming, called directly from setupCallbacks' `for`
// loop, so the recover() lives in the handler itself rather than around the
// loop).
func (a *App) handleWebMessage(incoming *protocol.IncomingMessage) {
	defer func() {
		if r := recover(); r != nil {
			server.LogRecoveredPanic(game.LogComponentApp, "handleWebMessage clientID="+incoming.ClientID, r)
		}
	}()

	msg := incoming.Data

	// #109 Phase 2 (D2/D3): any message received from an already-identified VJoueur
	// counts as a successful round-trip ("delivery confirmed", either direction) —
	// closes the connection-badge "green" window (min. duration honored internally).
	// No-op for admin/TV/not-yet-identified clients (GetClientPlayerID returns ok=false).
	if playerID, ok := a.wsHub.GetClientPlayerID(incoming.ClientID); ok {
		a.engine.ConfirmDelivery(playerID)
	}

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

	case protocol.ActionReleaseBumperName:
		a.handleReleaseBumperName(msg)

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

	case protocol.ActionMemorySetTeams:
		a.handleMemorySetTeams(msg)

	case protocol.ActionMotionSelect:
		a.handleMotionSelect(msg)

	case protocol.ActionMotionFlip:
		a.handleMotionFlip(msg)

	case protocol.ActionMotionStopTimer:
		a.handleMotionStopTimer(msg)

	case protocol.ActionMotionReveal:
		a.handleMotionReveal(msg)

	case protocol.ActionMotionDone:
		a.handleMotionDone(msg)

	case protocol.ActionMotionSetTeams:
		a.handleMotionSetTeams(msg)

	case protocol.ActionShowQRCode:
		a.handleShowQRCode()

	case protocol.ActionHideQRCode:
		a.handleHideQRCode()

	case protocol.ActionSetVirtualPlayerLimit:
		a.handleSetVirtualPlayerLimit(msg)

	case protocol.ActionPlayerConnect:
		a.handlePlayerConnect(incoming.ClientID, msg)

	case protocol.ActionVPlayerQCMAnswer:
		a.handleVPlayerQCMAnswer(incoming.ClientID, msg)

	case protocol.ActionArdoiseInput:
		a.handleArdoiseInput(incoming.ClientID, msg)

	case protocol.ActionNewGame:
		a.logger.Info(game.LogComponentEngine, "NEW_GAME — reset scores, history, statuses")
		purgedPlayerIDs := a.engine.InitGame()
		// #123 B3 — a fresh game invalidates any eviction context left over
		// from before it (e.g. an individual PLAYER_REMOVED from a prior game
		// session), so clear the registry first. Fresh GAME_RESET entries for
		// THIS purge are recorded right after, so a player who was offline at
		// the moment of the purge still learns the true reason on return.
		a.evictionRegistry.Reset()
		// #120 — Notify each purged VJoueur individually, BEFORE the general
		// broadcasts below, so its client learns the authoritative reason first
		// instead of deducing removal from the roster it's about to receive via
		// broadcastUpdate (same ordering rule as handleDeleteBumper).
		if len(purgedPlayerIDs) > 0 {
			evictedPayload := protocol.PlayerEvictedPayload{Reason: "GAME_RESET"}
			evictedMsg, _ := protocol.NewMessage(protocol.ActionPlayerEvicted, evictedPayload)
			for _, playerID := range purgedPlayerIDs {
				a.wsHub.SendToPlayerID(playerID, evictedMsg)
				a.evictionRegistry.Record(playerID, "GAME_RESET")
			}
		}
		a.broadcastQuestions() // push refreshed AVAILABLE statuses to all clients
		a.broadcastUpdate()
		// InitGame purges the whole VJoueur roster (fix R1 follow-up) — refresh
		// the enrollment counter so clients don't show a stale VirtualPlayerCount.
		a.broadcastEnrollmentUpdate()

	case protocol.ActionUpdateQuizMeta:
		var payload protocol.QuizMetaPayload
		if err := json.Unmarshal(msg.Msg, &payload); err != nil {
			server.LogWarn(game.LogComponentApp, "Failed to parse UPDATE_QUIZ_META payload: %v", err)
			return
		}
		// Absent = unchanged for the additive fields (contract ai-generation.md
		// §7): a client sending only a subset of the form must not wipe the
		// rest. v6.1.0 (#137 Batch 2b): Population/Difficulty (string)
		// replaced by Populations/Difficulties ([]string); Objectives added —
		// same absent/present-empty distinction applies to all four.
		current := a.engine.GetState()
		populations := current.QuizPopulations
		if payload.Populations != nil {
			populations = *payload.Populations
		}
		difficulties := current.QuizDifficulties
		if payload.Difficulties != nil {
			difficulties = *payload.Difficulties
		}
		language := current.QuizLanguage
		if payload.Language != nil {
			language = *payload.Language
		}
		objectives := current.QuizObjectives
		if payload.Objectives != nil {
			objectives = *payload.Objectives
		}
		a.engine.SetQuizMeta(payload.Name, payload.Theme, payload.Notes, populations, difficulties, language, objectives)
		// HIDDEN_FIELDS (v6.1.0, #137 Batch 2b T1.8) goes through the
		// dedicated SetQuizDisplay setter, not SetQuizMeta — same
		// absent = unchanged rule: only call it when the key was present.
		if payload.HiddenFields != nil {
			a.engine.SetQuizDisplay(*payload.HiddenFields)
		}
		a.broadcastUpdate()

	// CANCEL_AI_GENERATION is handled directly in
	// internal/server/websocket.go's readPump (contract ai-multi-provider.md
	// §11) — self-contained in package server so it works whether or not an
	// App-level dispatch loop is present (package-level tests included).

	default:
		server.LogWarn(game.LogComponentApp, "Unknown web action: %s", msg.Action)
	}
}

func (a *App) handleHello(clientID string, msg *protocol.Message, source string) {
	// All buzzers now connect via WebSocket
	proto := "WebSocket"

	a.logger.Info(game.LogComponentWebSocket, "HELLO from buzzer: %s (protocol: %s)", clientID, proto)

	// Parse payload and inject protocol
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Msg, &payload); err == nil {
		payload["PROTOCOL"] = proto

		// Mark buzzer as connected (v3.6.5): CONNECTED has no omitempty so false is
		// propagated to the frontend when the buzzer disconnects.
		payload["CONNECTED"] = true

		// Reset OTA_STATUS on reconnect: after a successful OTA the buzzer reboots and
		// sends a new HELLO. Clearing the status here prevents stale "done" state from
		// remaining visible in the UI after reconnection.
		payload["OTA_STATUS"] = ""

		// Extract firmware version: prefer firmware_version field (v3.1.0+ BuzzClick),
		// fall back to VERSION for older buzzers (< v3.1.0) that only send VERSION.
		fwVersion := ""
		if v, ok := payload["firmware_version"].(string); ok && v != "" {
			fwVersion = v
		} else if v, ok := payload["VERSION"].(string); ok && v != "" {
			fwVersion = v // Older buzzers send VERSION only
		}
		if fwVersion != "" {
			payload["FIRMWARE_VERSION"] = fwVersion
			fm := a.httpServer.GetFirmwareManager()
			isOutdated := fm.IsOutdated(fwVersion)
			payload["IS_OUTDATED"] = isOutdated
			if isOutdated {
				a.logger.Info(game.LogComponentApp, "Buzzer %s firmware %s is outdated", clientID, fwVersion)
			}
		}

		a.engine.UpdateBumper(clientID, payload)
	}

	// Send current state to all
	a.broadcastUpdate()

	// Update broadcaster frequency: a newly connected buzzer may reduce the
	// number of disconnected buzzers, so we may be able to slow the interval back.
	a.updateBroadcasterFrequency()

	// Re-send last known LED state to the reconnected buzzer (server-driven LEDs)
	if source == "WebSocket-Buzzer" {
		a.resendLEDOnReconnect(clientID)
	}

	// Auto-sync WiFi config to newly connected buzzer
	if source == "WebSocket-Buzzer" {
		a.sendWifiConfigToBuzzer(clientID)
	}
}

// handleOTAProgress processes OTA_PROGRESS messages from buzzers during firmware update.
func (a *App) handleOTAProgress(clientID string, msg *protocol.Message) {
	var progress protocol.OTAProgressPayload
	if err := json.Unmarshal(msg.Msg, &progress); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse OTA_PROGRESS from %s: %v", clientID, err)
		return
	}

	// Use clientID as MAC if progress.MAC is empty
	mac := progress.MAC
	if mac == "" {
		mac = clientID
	}

	server.LogInfo(game.LogComponentApp, "OTA_PROGRESS from %s: status=%s, percent=%d%%", mac, progress.Status, progress.Percent)

	// Update bumper OTA status and progress percentage
	updates := map[string]interface{}{
		"OTA_STATUS":  progress.Status,
		"OTA_PERCENT": progress.Percent,
	}

	if progress.Status == "done" {
		// Firmware transfer complete. Do NOT clear IS_OUTDATED here — the buzzer
		// will reboot and send a HELLO with its new FIRMWARE_VERSION, at which point
		// the HELLO handler recalculates IS_OUTDATED based on the actual version.
		// Clearing it here causes the frontend progress bar to turn green before the
		// buzzer has actually rebooted and confirmed its version.
		server.LogInfo(game.LogComponentApp, "Buzzer %s OTA transfer complete, awaiting reboot", mac)
	}

	a.engine.UpdateBumper(mac, updates)

	// Broadcast updated state to web clients
	a.broadcastUpdate()
}

func (a *App) handleButton(clientID string, msg *protocol.Message, timestamp int64) {
	payload, err := msg.ParseButtonPayload()
	if err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse button payload: %v", err)
		return
	}

	// Filter: only accept BUTTON in STARTED phase (spec §"Règle d'acceptation des buzz").
	// READY, COUNTDOWN, PAUSED, REVEALED, STOPPED → ignore silently.
	if a.engine.GetPhase() != game.PhaseStarted {
		server.LogInfo(game.LogComponentWebSocket, "BUTTON from %s ignored (phase=%s, not STARTED)", clientID, a.engine.GetPhase())
		return
	}

	server.LogInfo(game.LogComponentWebSocket, "BUTTON from %s: %s", clientID, payload.Button)

	// Process in engine
	a.engine.ProcessButtonPress(clientID, timestamp, payload.Button)

	// Update BuzzState tracking after the engine has recorded the press
	a.updateBuzzStates(clientID)

	// Broadcast pause to all — unchanged, drives LED state for every client.
	a.broadcastPause(clientID)

	// #129 T3.1: Admin/TV/Buzzer see every buzz (score/state changes for the
	// whole board); a targeted echo goes to the buzzing bumper itself if —
	// and only if — it's a VPlayer. clientID here is always a physical
	// buzzer MAC (handleButton is reached exclusively via
	// handleBuzzerMessage/ActionButton, never from a web client — see
	// handleSimulatedButton below for that path), so this branch is
	// currently always false in practice; the explicit IsVPlayer check
	// (rather than relying on broadcastUpdateToPlayer/SendRawToPlayerID's
	// already-safe no-op for an unmatched ID) makes that guarantee visible
	// at the call site and skips a pointless GetGameJSON()+SerializeForVPlayer
	// on every physical buzz.
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeBuzzer)
	if bumper := a.engine.GetBumper(clientID); bumper != nil && bumper.IsVPlayer {
		a.broadcastUpdateToPlayer(clientID)
	}
}

// handleSimulatedButton processes a BUTTON press from a web client. Despite
// the name, this is the REAL production path a VJoueur uses to buzz on a
// SPEEDY question — VPlayerPage.jsx sends {ACTION:"BUTTON", MSG:{ID, button}}
// (web/src/pages/VPlayerPage.jsx:429/560), which reaches this handler via
// handleWebMessage's ActionButton case, not handleButton (that one is
// exclusively the physical-buzzer /ws/buzzer path — see its own comment).
// It's ALSO used for admin's manual "simulate a press" testing tool with an
// arbitrary bumper ID, hence the historical name.
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

	// Update BuzzState tracking after the engine has recorded the press
	a.updateBuzzStates(payload.ID)

	// Broadcast pause to all — unchanged, drives LED state for every client.
	a.broadcastPause(payload.ID)

	// #129 (found beyond the plan's explicit T3.1 scope — same pattern,
	// same justification, see handoff/report): this IS the real VJoueur buzz
	// path (see doc comment above), so this call site was reaching every
	// OTHER VJoueur with the full GameState on every single buzz, at any
	// phase — not gated to PREPARE/READY like #127's reduction, and not one
	// of the three sites #129 T1.3-T1.5 explicitly retargeted. Admin/TV/
	// Buzzer see every buzz; the buzzing bumper gets a targeted echo if (and
	// only if) it's a VPlayer — admin's manual test tool can target a
	// physical buzzer's MAC here too, same guard as handleButton.
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeBuzzer)
	if bumper := a.engine.GetBumper(payload.ID); bumper != nil && bumper.IsVPlayer {
		a.broadcastUpdateToPlayer(payload.ID)
	}
}

// handlePong processes PONG from buzzer or web client (WebSocket simulation)
func (a *App) handlePong(clientID string, msg *protocol.Message) {
	// If ID in payload, use it (web simulation), otherwise use clientID (buzzer MAC)
	bumperID := clientID

	var payload struct {
		ID string `json:"ID"`
	}
	if json.Unmarshal(msg.Msg, &payload) == nil && payload.ID != "" {
		bumperID = payload.ID
	}

	server.LogInfo(game.LogComponentWebSocket, "PONG from %s", bumperID)

	if a.engine.IsGamePrepare() {
		a.engine.SetBumperReady(bumperID)

		// Check if all ready
		if a.engine.AreAllTeamsReady() {
			a.engine.TransitionToReady()
			a.broadcastReady()
		}

		// #127 T1.2: VJoueurs are deliberately NOT targeted here — for N
		// participants this fired N nearly-simultaneous full-GameState
		// broadcasts at every VJoueur, right at the moment they're also
		// exchanging PLAYER_CONNECT/PONG traffic (the broadcast storm root
		// cause of #127). Admin keeps this per-PONG cadence on purpose: the
		// "prêt" progress on TeamCard (GamePage.jsx) must update one buzzer at
		// a time, not just once at the end (CA2) — so this call is NOT moved
		// inside the AreAllTeamsReady() branch above. The VJoueur's own single
		// UPDATE for this window is delivered by TransitionToReady() →
		// OnStateChange → broadcastGameState(), which already targets
		// VPlayer (T1.3).
		//
		// #129 T1.6: TV also removed from this per-PONG rafale — it's in the
		// exact pre-#127 situation the VJoueur used to be in: PlayerDisplay.jsx
		// showPrepare block (~l.1358-1373) renders only a static "🔔 NOUVELLE
		// QUESTION" label during PREPARE, no reference to READY/bumpers/teams,
		// and the score carousel depends on SCORE/COLOR/NAME, which don't
		// change from one PONG to the next. The TV still gets both window
		// bounds (PREPARE entry via handleReady, READY transition via
		// broadcastGameState) — N+2 UPDATE becomes 2, same reduction #127 gave
		// the VJoueur. Physical buzzers are KEPT deliberately: broadcastGameState
		// never targets them, so this per-PONG UPDATE is their ONLY phase
		// signal on this path during the window — the firmware uses it (team
		// assignment, grey LED rotation). Removing them would be a firmware
		// behavior change, unrelated to #129 — do not "clean up" this line
		// further.
		a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
	}
}

// notifyEvictedVirtualPlayers compares newBumpers against the CURRENT roster
// and emits PLAYER_EVICTED{PLAYER_REMOVED} to every VIRTUAL bumper present
// now but absent from newBumpers, recording the reason in evictionRegistry
// (#123 B1/B3).
//
// This is deliberately triggered by STATE (a roster shrinking), not by a
// specific action — #120's original notification was wired only into
// DELETE_BUMPER, an action the real admin UI never sends (it edits the
// roster via UPDATE, TeamsPage.jsx). A mechanism correct in isolation but
// wired to a path nobody uses is exactly the class of defect COLOR_NAME hit
// in #113. Must be called BEFORE the roster is actually replaced
// (a.engine.SetBumpers), so "current" still reflects the pre-update roster.
// Physical buzzers are never candidates — they have no WebSocket client of
// their own to notify.
func (a *App) notifyEvictedVirtualPlayers(newBumpers map[string]*game.Bumper) {
	currentBumpers := a.engine.GetTeamsAndBumpers().Bumpers
	for id, bumper := range currentBumpers {
		if bumper == nil || !bumper.IsVirtual {
			continue
		}
		if _, stillPresent := newBumpers[id]; stillPresent {
			continue
		}

		evictedPayload := protocol.PlayerEvictedPayload{Reason: "PLAYER_REMOVED"}
		evictedMsg, _ := protocol.NewMessage(protocol.ActionPlayerEvicted, evictedPayload)
		a.wsHub.SendToPlayerID(id, evictedMsg)
		a.evictionRegistry.Record(id, "PLAYER_REMOVED")
		server.LogInfo(game.LogComponentWebSocket, "PLAYER_EVICTED: id=%s reason=PLAYER_REMOVED (roster diff)", id)
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
		// #123 B1 — notify BEFORE replacing the roster, same ordering rule as
		// handleDeleteBumper: the client must learn the authoritative reason
		// before it receives an UPDATE broadcast that no longer contains it.
		a.notifyEvictedVirtualPlayers(data.Bumpers)
		a.engine.SetBumpers(data.Bumpers)
		// Bumpers were replaced — recompute broadcaster frequency.
		a.updateBroadcasterFrequency()
	}
	a.broadcastUpdate()
	// Refresh LED state on all buzzers after team/bumper changes
	a.sendLEDSetAllBuzzers()
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

	// Send COMET LED effect to the team that received points (if points > 0)
	if payload.Points > 0 {
		teamID := ""
		if bumper := a.engine.GetBumper(payload.BumperID); bumper != nil {
			teamID = bumper.Team
		}
		a.sendLEDSetComet(teamID)
	}

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

	// #127: a.engine.Ready() already fired OnStateChange(PhasePrepare) above,
	// which calls broadcastGameState() -> broadcastUpdateTo(Admin, TV,
	// VPlayer) with the exact same fresh GetGameJSON() this line used to send
	// again via the default broadcastUpdate() (Admin, TV, VPlayer, Buzzer) —
	// byte-for-byte identical, since nothing mutates state between the two
	// calls. That duplicated every PREPARE-entry UPDATE for Admin/TV/VPlayer
	// (found while verifying CA1 empirically: a VJoueur received 2 UPDATEs
	// at PREPARE entry instead of 1). Buzzers are the only type that
	// genuinely needs this second call — broadcastGameState never targets
	// server.ClientTypeBuzzer (T1.3) — so only that type is kept here.
	a.broadcastUpdateTo(server.ClientTypeBuzzer)

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
	deletedBumper, exists := bumpers[payload.ID]
	if !exists {
		server.LogWarn(game.LogComponentApp, "DELETE_BUMPER: Bumper %s not found", payload.ID)
		return
	}

	delete(bumpers, payload.ID)
	a.engine.SetBumpers(bumpers)

	server.LogInfo(game.LogComponentApp, "Deleted bumper: %s", payload.ID)

	// #120 — Notify the evicted VJoueur (never physical buzzers, which have no
	// WebSocket client of their own) so it can leave the game screen with a
	// reason instead of silently deducing removal from a roster scan.
	if deletedBumper != nil && deletedBumper.IsVirtual {
		evictedPayload := protocol.PlayerEvictedPayload{Reason: "PLAYER_REMOVED"}
		evictedMsg, _ := protocol.NewMessage(protocol.ActionPlayerEvicted, evictedPayload)
		a.wsHub.SendToPlayerID(payload.ID, evictedMsg)
		// #123 B3 — remember the reason so a later PLAYER_CONNECT with this now
		// stale ID gets the truth instead of a generic ENROLLMENT_CLOSED guess.
		a.evictionRegistry.Record(payload.ID, "PLAYER_REMOVED")
	}

	// Broadcast updated state
	a.broadcastUpdate()
	// A bumper was removed — recompute broadcaster frequency.
	a.updateBroadcasterFrequency()
}

// handleReleaseBumperName grants the reclaim authorization (#122 B3): the
// animateur has decided, having seen the room, that the next nameless
// PLAYER_CONNECT under this bumper's name is genuinely its owner (or a
// substitute, #134) coming back. This is the sole exception to the #109 R1
// ID-only identity rule, and it is explicit and human — never automatic.
//
// #134 widens this to a still-CONNECTED bumper: RELEASE_BUMPER_NAME { ID }
// is unchanged as an action, but the server now branches on connection
// state (contracts/seat-release.md §2-3). Normative sequence, order
// matters:
//  1. read connection state BEFORE any mutation (below);
//  2. if connected, notify on the OLD id BEFORE the engine re-keys it —
//     mirrors handleDeleteBumper's contract exactly (same reasoning: never
//     close the socket, queue PLAYER_EVICTED on client.Send instead);
//  3. record the eviction reason so a lost-notification reconnect attempt
//     with the stale ID still gets the truth (#123 B3 registry, reused
//     as-is);
//  4. ReleaseSeat performs the actual (atomic) mutation;
//  5. broadcastUpdate() — the admin card reflects the new state.
//
// Note on the narrow race between step 1's read and step 4's mutation: they
// are two separate lock acquisitions (GetBumper's snapshot, then
// ReleaseSeat's own lock), so a disconnect/reconnect landing in that exact
// window could make this read stale. ReleaseSeat re-evaluates Connected
// itself and is the sole authority for what actually happens — same
// accepted-narrow-race class as #129's SendToPlayerID duplicate-PlayerID
// window, not a new risk introduced here.
func (a *App) handleReleaseBumperName(msg *protocol.Message) {
	var payload protocol.ReleaseBumperNamePayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse RELEASE_BUMPER_NAME: %v", err)
		return
	}

	if payload.ID == "" {
		server.LogWarn(game.LogComponentApp, "RELEASE_BUMPER_NAME: empty ID")
		return
	}

	wasConnectedBeforeMutation := false
	if bumper := a.engine.GetBumper(payload.ID); bumper != nil {
		wasConnectedBeforeMutation = bumper.Connected
	}

	if wasConnectedBeforeMutation {
		evictedPayload := protocol.PlayerEvictedPayload{Reason: "SEAT_RELEASED"}
		evictedMsg, _ := protocol.NewMessage(protocol.ActionPlayerEvicted, evictedPayload)
		a.wsHub.SendToPlayerID(payload.ID, evictedMsg)
		a.evictionRegistry.Record(payload.ID, "SEAT_RELEASED")
	}

	newID, wasConnected, ok := a.engine.ReleaseSeat(payload.ID)
	if !ok {
		server.LogWarn(game.LogComponentApp, "RELEASE_BUMPER_NAME: bumper %s not found or not virtual", payload.ID)
		return
	}

	if wasConnected {
		server.LogInfo(game.LogComponentApp, "RELEASE_BUMPER_NAME: seat released while connected, old_id=%s new_id=%s", payload.ID, newID)
	} else {
		server.LogInfo(game.LogComponentApp, "RELEASE_BUMPER_NAME: id=%s", payload.ID)
	}
	// RECLAIM_REQUESTED cleared server-side (and, for a connected release,
	// the bumper re-keyed) — push the updated card state to admin clients
	// right away.
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
			a.sendLEDSetAllBuzzers()
		}()
	}

	if isMatch {
		server.LogInfo(game.LogComponentEngine, "Memory MATCH found!")
		a.sendLEDSetAllBuzzers()
	}

	// If all pairs matched, automatically stop the game
	if isComplete {
		server.LogInfo(game.LogComponentEngine, "Memory game COMPLETE! All pairs matched.")
		a.engine.Stop()
		a.broadcastUpdate()
		a.sendLEDSetAllBuzzers()
	}
}

func (a *App) handleMemorySetTeams(msg *protocol.Message) {
	var payload protocol.MemorySetTeamsPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse MEMORY_SET_TEAMS: %v", err)
		return
	}

	if len(payload.Teams) == 0 {
		server.LogWarn(game.LogComponentApp, "MEMORY_SET_TEAMS: empty teams list")
		return
	}

	server.LogInfo(game.LogComponentEngine, "MEMORY_SET_TEAMS: teams=%v", payload.Teams)

	// Set participating teams
	if err := a.engine.SetMemoryParticipatingTeams(payload.Teams); err != nil {
		server.LogError(game.LogComponentEngine, "Failed to set memory teams: %v", err)
		return
	}

	// Broadcast updated game state to all clients
	a.broadcastUpdate()
	a.sendLEDSetAllBuzzers()
}

// ============================================================
// MEMOTION handlers — v5.0.0
// ============================================================

// handleMotionSelect processes MEMOTION_SELECT: picks a card from the grid (→ SELECTED subphase, no timer).
func (a *App) handleMotionSelect(msg *protocol.Message) {
	var payload protocol.MotionSelectPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse MEMOTION_SELECT: %v", err)
		return
	}
	if payload.CardID == "" {
		server.LogWarn(game.LogComponentApp, "MEMOTION_SELECT: empty card ID")
		return
	}

	server.LogInfo(game.LogComponentEngine, "MEMOTION_SELECT: cardID=%s", payload.CardID)

	if err := a.engine.SelectMotionCard(payload.CardID); err != nil {
		server.LogWarn(game.LogComponentEngine, "MEMOTION_SELECT error: %v", err)
		return
	}

	a.broadcastUpdate()
}

// handleMotionFlip processes MEMOTION_FLIP: transitions selected card to QUESTION face and starts timer.
func (a *App) handleMotionFlip(msg *protocol.Message) {
	server.LogInfo(game.LogComponentEngine, "MEMOTION_FLIP")

	if err := a.engine.FlipMotionCard(); err != nil {
		server.LogWarn(game.LogComponentEngine, "MEMOTION_FLIP error: %v", err)
		return
	}

	// Start per-card timer if Question.Time > 0
	state := a.engine.GetState()
	if state.Question != nil && state.Question.Time != "" && state.Question.Time != "0" {
		var delay int
		if _, err2 := fmt.Sscanf(state.Question.Time, "%d", &delay); err2 == nil && delay > 0 {
			a.engine.StartMotionCardTimer(delay)
		}
	}

	a.broadcastUpdate()
}

// handleMotionStopTimer processes MEMOTION_STOP_TIMER: stops the per-card timer without changing subphase.
func (a *App) handleMotionStopTimer(msg *protocol.Message) {
	server.LogInfo(game.LogComponentEngine, "MEMOTION_STOP_TIMER")
	a.engine.StopMotionCardTimer()
	a.broadcastUpdate()
}

// handleMotionReveal processes MEMOTION_REVEAL: flip the active card to its answer face.
func (a *App) handleMotionReveal(msg *protocol.Message) {
	server.LogInfo(game.LogComponentEngine, "MEMOTION_REVEAL")

	// Stop timer (if running) before revealing
	a.engine.StopMotionCardTimer()

	if err := a.engine.RevealMotionCard(); err != nil {
		server.LogWarn(game.LogComponentEngine, "MEMOTION_REVEAL error: %v", err)
		return
	}

	a.broadcastUpdate()
}

// handleMotionDone processes MEMOTION_DONE: closes a card, awards points, rotates team.
func (a *App) handleMotionDone(msg *protocol.Message) {
	var payload protocol.MotionDonePayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse MEMOTION_DONE: %v", err)
		return
	}
	if payload.CardID == "" {
		server.LogWarn(game.LogComponentApp, "MEMOTION_DONE: empty card ID")
		return
	}

	server.LogInfo(game.LogComponentEngine, "MEMOTION_DONE: cardID=%s winner=%s",
		payload.CardID, payload.WinnerTeam)

	// Stop per-card timer if still running
	a.engine.StopMotionCardTimer()

	points, isComplete, err := a.engine.DoneMotionCard(payload.CardID, payload.WinnerTeam)
	if err != nil {
		server.LogWarn(game.LogComponentEngine, "MEMOTION_DONE error: %v", err)
		return
	}

	// Award LED comet effect to winning team
	if points > 0 && payload.WinnerTeam != "" {
		a.sendLEDSetComet(payload.WinnerTeam)
	}

	// Record event to history
	if points > 0 && payload.WinnerTeam != "" {
		state := a.engine.GetState()
		questionID := ""
		questionText := payload.CardID // Use cardID as fallback
		questionCategory := ""
		if state.Question != nil {
			questionID = state.Question.ID
			questionCategory = string(state.Question.Category)
			// Use RectoTheme of the card as event context
			for _, card := range state.Question.MotionCards {
				if card.ID == payload.CardID {
					questionText = card.RectoTheme
					break
				}
			}
		}
		var teamColor []int
		if team := a.engine.GetTeam(payload.WinnerTeam); team != nil {
			teamColor = team.Color
		}
		catName, catImageURL, catColor := a.httpServer.ResolveCategoryMeta(questionCategory)
		event := game.GameEvent{
			Timestamp:           time.Now().UnixMicro(),
			QuestionID:          questionID,
			QuestionText:        questionText,
			QuestionCategory:    questionCategory,
			CategoryDisplayName: catName,
			CategoryImageURL:    catImageURL,
			CategoryColor:       catColor,
			EventType:           "POINTS_AWARDED",
			WinnerID:            payload.WinnerTeam,
			WinnerName:          payload.WinnerTeam,
			WinnerType:          "TEAM",
			TeamName:            payload.WinnerTeam,
			TeamColor:           teamColor,
			Points:              points,
		}
		a.engine.AddGameEvent(event)
		server.LogInfo(game.LogComponentEngine, "MEMOTION: %d pts → team %s", points, payload.WinnerTeam)
	}

	// Auto-stop when all cards are done
	if isComplete {
		server.LogInfo(game.LogComponentEngine, "MEMOTION game COMPLETE! All cards played.")
		a.engine.Stop()
		a.sendLEDSetAllBuzzers()
	}

	a.broadcastUpdate()
}

// handleMotionSetTeams processes MEMOTION_SET_TEAMS: sets participating teams.
func (a *App) handleMotionSetTeams(msg *protocol.Message) {
	var payload protocol.MotionSetTeamsPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Failed to parse MEMOTION_SET_TEAMS: %v", err)
		return
	}

	server.LogInfo(game.LogComponentEngine, "MEMOTION_SET_TEAMS: teams=%v", payload.Teams)

	if err := a.engine.SetMotionParticipatingTeams(payload.Teams); err != nil {
		server.LogError(game.LogComponentEngine, "MEMOTION_SET_TEAMS error: %v", err)
		return
	}

	a.broadcastUpdate()
	a.sendLEDSetAllBuzzers()
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

	// Send COMET LED effect to the team that received points (if points > 0)
	if payload.Points > 0 {
		teamID := ""
		if b := a.engine.GetBumper(payload.ID); b != nil {
			teamID = b.Team
		}
		a.sendLEDSetComet(teamID)
	}

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
	catName, catImageURL, catColor := a.httpServer.ResolveCategoryMeta(questionCategory)
	event := game.GameEvent{
		Timestamp:           time.Now().UnixMicro(),
		QuestionID:          questionID,
		QuestionText:        questionText,
		QuestionCategory:    questionCategory,
		CategoryDisplayName: catName,
		CategoryImageURL:    catImageURL,
		CategoryColor:       catColor,
		EventType:           "POINTS_AWARDED",
		WinnerID:            payload.ID,
		WinnerName:          bumperName,
		WinnerType:          "PLAYER",
		TeamName:            teamName,
		TeamColor:           teamColor,
		PlayerName:          bumperName,
		PlayerColor:         playerColor,
		Points:              payload.Points,
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

	// Send COMET LED effect to the team that received points (if points > 0)
	if payload.Points > 0 {
		a.sendLEDSetComet(payload.Team)
	}

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
	catName, catImageURL, catColor := a.httpServer.ResolveCategoryMeta(questionCategory)
	event := game.GameEvent{
		Timestamp:           time.Now().UnixMicro(),
		QuestionID:          questionID,
		QuestionText:        questionText,
		QuestionCategory:    questionCategory,
		CategoryDisplayName: catName,
		CategoryImageURL:    catImageURL,
		CategoryColor:       catColor,
		EventType:           "POINTS_AWARDED",
		WinnerID:            payload.Team,
		WinnerName:          payload.Team,
		WinnerType:          "TEAM",
		TeamName:            payload.Team,
		TeamColor:           teamColor,
		Points:              payload.Points,
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
	case "vplayer":
		clientType = server.ClientTypeVPlayer
	default:
		clientType = server.ClientTypeAdmin
	}

	a.wsHub.SetClientType(clientID, clientType)
	// contract ai-multi-provider.md §10 (pushing AI_GENERATION_PROGRESS to a
	// newly-identified admin) is handled uniformly inside
	// internal/server.WebSocketHub.OnClientRegistered, wired in
	// server.NewHTTPServer — covers this legacy SET_CLIENT_TYPE path AND the
	// dedicated /ws/admin endpoint from one place.

	// Broadcast updated counts
	a.broadcastClientCounts()
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
	// Switch UDP broadcast to fast mode (1s) so new devices discover the server quickly
	a.broadcaster.SetEnrollmentMode(true)
	server.LogInfo(game.LogComponentApp, "Entering ENROLL phase - QR code displayed, limit: %d", limit)
	a.broadcastUpdate()
	a.broadcastEnrollmentUpdate()
}

func (a *App) handleHideQRCode() {
	a.engine.StopEnrollment()
	a.engine.SetPhase(game.PhaseStopped)
	// Restore normal UDP broadcast interval (5s)
	a.broadcaster.SetEnrollmentMode(false)
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

	// #123 B3 — an ID that no longer resolves to a live VIRTUAL bumper might be
	// one we just evicted (individually removed, or purged by NEW_GAME).
	// Consult the eviction registry BEFORE falling through to
	// ReconnectOrCreateVirtualPlayer's name-based path (matrix case 2), which
	// would otherwise misreport a generic ENROLLMENT_CLOSED — or silently
	// attempt a fresh enrollment — instead of the real reason. A currently
	// live ID is untouched: this check mirrors the engine's own case-1 gate
	// (resolved AND IsVirtual), so a genuine reconnection never reaches here.
	if payload.ID != "" {
		if liveBumper := a.engine.GetBumper(payload.ID); liveBumper == nil || !liveBumper.IsVirtual {
			if reason, ok := a.evictionRegistry.Lookup(payload.ID); ok {
				rejectPayload := protocol.PlayerRejectedPayload{Reason: reason}
				rejectMsg, _ := protocol.NewMessage(protocol.ActionPlayerRejected, rejectPayload)
				a.wsHub.SendToClient(clientID, rejectMsg)
				server.LogInfo(game.LogComponentWebSocket, "PLAYER_CONNECT: rejected stale id=%s with recorded eviction reason=%s", payload.ID, reason)
				return
			}
		}
	}

	// Resolve reconnection vs. new enrollment atomically, under a single engine
	// lock (#109 R1 fix: the previous implementation read the bumper map here,
	// decided "not found", and only then called CreateVirtualPlayer as a
	// separate locked step — two near-simultaneous PLAYER_CONNECT calls for the
	// same identity could race into two different bumpers). Identity is now the
	// backend-issued bumper ID (payload.ID, echoed back from a prior
	// PLAYER_CONNECTED — empty on first enrollment), not the name: a name match
	// with no resolvable ID is REJECTED (NAME_TAKEN), never merged/replaced —
	// see engine.ReconnectOrCreateVirtualPlayer for the full decision matrix.
	bumperID, bumper, reconnected, err := a.engine.ReconnectOrCreateVirtualPlayer(payload.ID, playerName)
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

	// Link this WS client to its VJoueur bumper ID (used by OnPlayerDisconnected / anti-zombie guard)
	a.wsHub.SetClientPlayerID(clientID, bumperID)

	if reconnected {
		server.LogInfo(game.LogComponentWebSocket, "PLAYER_CONNECT: reconnecting existing player: id=%s, name=%s", bumperID, bumper.Name)
		// #129 T1.4: Admin/TV/Buzzer see the roster change; no OTHER VPlayer
		// consumes a peer's reconnection (sortedPlayers gated !isVPlayer since
		// #127). The reconnecting player itself still needs its own echo — it
		// recovers its bumper/session state (CONNECTED=true, score, team)
		// from THIS message (#118/#120/#122) — sent AFTER SetClientPlayerID
		// above so the hub can already route to it (R1: the one thing this
		// site must never get wrong).
		a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeBuzzer)
		a.broadcastUpdateToPlayer(bumperID)
		return
	}

	server.LogInfo(game.LogComponentWebSocket, "PLAYER_CONNECT: player connected: id=%s, name=%s", bumperID, bumper.Name)

	// #129 T1.4: same reasoning as the reconnection branch above — Admin/TV/
	// Buzzer see the new roster entry; the newly-enrolled player gets its own
	// targeted echo (its bumper didn't exist in any earlier broadcast it
	// could have received).
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeBuzzer)
	a.broadcastUpdateToPlayer(bumperID)
	// Broadcast enrollment count update — unchanged, already lightweight and
	// functionally targeted (carries only the enrollment counter).
	a.broadcastEnrollmentUpdate()
}

// handleVPlayerQCMAnswer processes a QCM answer from a VPlayer (WebSocket)
func (a *App) handleVPlayerQCMAnswer(clientID string, msg *protocol.Message) {
	var payload protocol.VPlayerQCMAnswerPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "Error parsing VPLAYER_QCM_ANSWER payload: %v", err)
		return
	}

	// Find the bumper associated with this WebSocket client
	// In the current implementation, we need to identify the VPlayer by the clientID
	// Since we don't have a direct mapping clientID -> bumperID, we need to find
	// the VPlayer based on the WebSocket connection session
	// For now, we'll use msg.ID if provided, or look through all bumpers

	var bumperID string
	var bumper *game.Bumper

	// Get bumper ID from payload (inside MSG)
	if payload.ID != "" {
		bumperID = payload.ID
		bumper = a.engine.GetBumper(bumperID)
	}

	// Fallback: try msg.ID (top-level) or clientID
	if bumper == nil && msg.ID != "" {
		bumperID = msg.ID
		bumper = a.engine.GetBumper(bumperID)
	}
	if bumper == nil && clientID != "" {
		bumper = a.engine.GetBumper(clientID)
		if bumper != nil {
			bumperID = clientID
		}
	}

	if bumper == nil {
		server.LogWarn(game.LogComponentApp, "VPLAYER_QCM_ANSWER: bumper not found for client %s", clientID)
		return
	}

	// Verify this is a VPlayer
	if !bumper.IsVPlayer {
		server.LogWarn(game.LogComponentApp, "VPLAYER_QCM_ANSWER: bumper %s is not a VPlayer (IsVPlayer=false)", bumperID)
		return
	}

	// Verify game is in STARTED phase
	if a.engine.GetPhase() != game.PhaseStarted {
		server.LogWarn(game.LogComponentApp, "VPLAYER_QCM_ANSWER: game not in STARTED phase (current: %s)", a.engine.GetPhase())
		return
	}

	// Verify current question is QCM
	state := a.engine.GetState()
	if state.Question == nil || state.Question.Type != game.QuestionTypeQCM {
		server.LogWarn(game.LogComponentApp, "VPLAYER_QCM_ANSWER: current question is not QCM")
		return
	}

	// Map color to button (RED=A, GREEN=B, YELLOW=C, BLUE=D)
	colorToButton := map[string]string{
		"RED":    "A",
		"GREEN":  "B",
		"YELLOW": "C",
		"BLUE":   "D",
	}

	button, ok := colorToButton[payload.AnswerColor]
	if !ok {
		server.LogWarn(game.LogComponentApp, "VPLAYER_QCM_ANSWER: invalid color %s", payload.AnswerColor)
		return
	}

	// Use current time as timestamp (microseconds)
	timestamp := time.Now().UnixMicro()

	server.LogInfo(game.LogComponentEngine, "VPLAYER_QCM_ANSWER: VPlayer %s (%s) answered %s (button %s) at time %d",
		bumperID, bumper.Name, payload.AnswerColor, button, timestamp)

	// Process as a button press (same as physical buzzer or simulated button)
	a.engine.ProcessButtonPress(bumperID, timestamp, button)

	// Update BuzzState tracking after the engine has recorded the press
	a.updateBuzzStates(bumperID)

	// Broadcast pause to all — unchanged, drives LED state for every client.
	a.broadcastPause(bumperID)

	// #129 T3.1: Admin/TV/Buzzer see every answer; the answering VPlayer gets
	// a targeted echo of its own resulting state. bumper.IsVPlayer is already
	// verified true above (line ~2211) — always a real recipient here, no
	// guard needed (contrast with handleButton, the physical-buzzer path).
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeBuzzer)
	a.broadcastUpdateToPlayer(bumperID)
}

// handleArdoiseInput processes a free-text answer update from a VPlayer during an ARDOISE question.
// The handler follows the "ignore silently" specification: guard failures produce only a log warning.
// Team identification uses the same protocol-native pattern as handleVPlayerQCMAnswer:
// clientID → bumper lookup → bumper.Team.
func (a *App) handleArdoiseInput(clientID string, msg *protocol.Message) {
	var payload protocol.ArdoiseInputPayload
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		server.LogError(game.LogComponentApp, "ARDOISE_INPUT: error parsing payload: %v", err)
		return
	}

	// Resolve bumper — 3-pass pattern identical to handleVPlayerQCMAnswer
	var bumperID string
	var bumper *game.Bumper

	// Pass 1: payload.ID (explicit bumper ID sent by VPlayer in MSG, same as VPLAYER_QCM_ANSWER)
	if payload.ID != "" {
		bumperID = payload.ID
		bumper = a.engine.GetBumper(bumperID)
	}
	// Pass 2: msg.ID (top-level field)
	if bumper == nil && msg.ID != "" {
		bumperID = msg.ID
		bumper = a.engine.GetBumper(bumperID)
	}
	// Pass 3: clientID fallback (IP:port — may not match bumper key for VPlayers)
	if bumper == nil && clientID != "" {
		bumper = a.engine.GetBumper(clientID)
		if bumper != nil {
			bumperID = clientID
		}
	}

	if bumper == nil {
		server.LogWarn(game.LogComponentApp, "ARDOISE_INPUT: bumper not found for client %s", clientID)
		return
	}

	teamName := bumper.Team
	if teamName == "" {
		server.LogWarn(game.LogComponentApp, "ARDOISE_INPUT: bumper %s has no team assigned", bumperID)
		return
	}

	// SetArdoiseAnswer applies phase + type guards and returns false if conditions not met
	if !a.engine.SetArdoiseAnswer(teamName, payload.Text) {
		server.LogWarn(game.LogComponentApp, "ARDOISE_INPUT: ignored (phase=%s or question type mismatch)", a.engine.GetPhase())
		return
	}

	server.LogInfo(game.LogComponentApp, "ARDOISE_INPUT: team=%s text=%q (from client %s)", teamName, payload.Text, clientID)

	// #129 T1.5: admin only — so admin sees answers build up in real-time.
	// TV excluded: PlayerDisplay.jsx:2427 only renders ARDOISE_ANSWERS in
	// phase REVEALED (showAnswer && !isVPlayer); it receives the complete
	// state on the phase-change broadcast that follows. VPlayer excluded:
	// VPlayerPage.jsx manages its own text entry as local state (ardoiseText)
	// and never reads ARDOISE_ANSWERS — this was also a fairness leak, every
	// VJoueur was receiving the text every OTHER team was actively typing.
	// Buzzers excluded: SerializeForBuzzer carries no ARDOISE field at all.
	//
	// #129 T2.2: coalesced rather than broadcast directly — 8 teams typing
	// produce ~40 calls/sec here, each an a.engine.GetGameJSON() taking the
	// engine lock (round 1 scenario C). ardoiseCoalescer collapses a burst
	// into at most one admin UPDATE per ardoiseCoalesceWindow; a.broadcastUpdateTo
	// still runs exactly the same as before, just deferred slightly and
	// deduplicated — see BroadcastCoalescer's doc comment for why a delayed
	// emission is always safe here (it reads live state, never a buffered
	// payload). Flushed immediately on every phase change (OnStateChange) so
	// the last keystrokes before REVEAL are never held back — CA5/CA6.
	a.ardoiseCoalescer.Trigger()
}

// ardoiseCoalesceWindow is the ≤150ms ceiling #129 T2.1/CA5 sets on how long
// an ARDOISE_INPUT-triggered admin UPDATE may be delayed by coalescing.
const ardoiseCoalesceWindow = 150 * time.Millisecond

// Broadcast methods

// broadcastFiltered sends a WebSocket message only to clients of the specified types.
// It is a thin wrapper around wsHub.BroadcastToTypes and is used by all broadcastXxx()
// helpers to implement the filtering table from the v3.8.0 WS endpoints contract.
func (a *App) broadcastFiltered(msg *protocol.Message, types ...server.ClientType) {
	a.wsHub.BroadcastToTypes(msg, types...)
}

func (a *App) broadcast(action string, data json.RawMessage, viaTCP bool, types ...server.ClientType) {
	msg, _ := protocol.NewMessage(action, nil)
	msg.Msg = data

	// WebSocket (web clients) — filtered by type, or all if no types specified
	if len(types) == 0 {
		a.wsHub.Broadcast(msg)
	} else {
		a.wsHub.BroadcastToTypes(msg, types...)
	}

	// Buzzer-targeted broadcasts (UDP only — buzzer WS clients receive LED_SET directly)
	if viaTCP {
		a.udpBcast.Broadcast(msg)
	}
}

func (a *App) broadcastHello() {
	msg, _ := protocol.NewMessage(protocol.ActionHello, map[string]interface{}{})
	a.wsHub.BroadcastToTypes(msg, server.ClientTypeAdmin)
	a.udpBcast.Broadcast(msg)
	a.buzzerHub.Broadcast(msg) // HELLO is in buzzer whitelist — sent to physical buzzers
}

// broadcastUpdate sends the full GameState UPDATE to every client type that
// historically received it on every game-state change: Admin, TV, VPlayer
// and physical buzzers. It is a thin wrapper over broadcastUpdateTo — kept
// as the default entry point so most call sites don't need to spell out the
// target list. Callers that only need a subset of these types (#127 — e.g.
// handlePong's per-PONG update, which must not reach VJoueurs) should call
// broadcastUpdateTo directly instead of broadcastUpdate.
func (a *App) broadcastUpdate() {
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer, server.ClientTypeBuzzer)
}

// broadcastUpdateTo sends the full GameState UPDATE, but only to the client
// types listed in types — each type serialized through the same per-type
// path broadcastUpdate always used (SerializeForAdmin / SerializeForWebClient
// / SerializeForBuzzer). Pass server.ClientTypeBuzzer to also reach physical
// buzzers via a.buzzerHub (a separate hub from a.wsHub, so it needs its own
// explicit flag rather than being picked up by the wsHub type filter).
//
// #127 T1.1: introduced so a broadcast can target Admin/TV without also
// reaching VJoueurs (handlePong's per-PONG rafale, T1.2) and so
// broadcastGameState (T1.3) can go through this same filtered path instead
// of the unfiltered generic a.broadcast(). CA7: ApplyVPlayerBroadcastConnEvents
// must only run when this broadcast actually reaches VJoueurs — calling it on
// an Admin/TV-only broadcast would flag MessageLost on VJoueurs that received
// nothing from this particular broadcast.
func (a *App) broadcastUpdateTo(types ...server.ClientType) {
	if a.wsHub == nil {
		return // Guard for unit tests that construct a minimal App without a wsHub
	}

	hasType := func(t server.ClientType) bool {
		for _, x := range types {
			if x == t {
				return true
			}
		}
		return false
	}

	targetAdmin := hasType(server.ClientTypeAdmin)
	targetTV := hasType(server.ClientTypeTV)
	targetVPlayer := hasType(server.ClientTypeVPlayer)
	targetBuzzers := hasType(server.ClientTypeBuzzer)

	// #109 Phase 2 (D4) / #127 CA7: only evaluate MessageLost/DeliveryConfirmed
	// when VJoueurs are actually among the recipients of THIS broadcast.
	if targetVPlayer {
		a.engine.ApplyVPlayerBroadcastConnEvents()
	}

	data := a.engine.GetGameJSON()
	server.LogDebug(game.LogComponentApp, "Broadcasting UPDATE: %s", string(data)[:min(200, len(data))])
	msg, _ := protocol.NewMessage(protocol.ActionUpdate, nil)
	msg.Msg = data
	msg.Version = a.config.Version

	// Admin clients receive the full payload (all firmware/OTA/ACK fields).
	if targetAdmin {
		if dataAdmin, err := msg.SerializeForAdmin(); err == nil {
			a.wsHub.BroadcastRawToTypes(dataAdmin, server.ClientTypeAdmin)
		}
	}

	// TV and VPlayer clients both start from the same stripped payload (no
	// firmware/OTA/ACK metadata) — computed once and reused: TV always gets
	// it verbatim; VPlayer gets it too, except during PREPARE/READY where
	// broadcastUpdateToVPlayers further reduces "bumpers" per recipient
	// (#127 T2.1-T2.3) — dataWeb is also its fallback for that reduction.
	var dataWeb []byte
	if targetTV || targetVPlayer {
		if data, err := msg.SerializeForWebClient(); err == nil {
			dataWeb = data
		}
	}
	if targetTV && dataWeb != nil {
		a.wsHub.BroadcastRawToTypes(dataWeb, server.ClientTypeTV)
	}
	if targetVPlayer && dataWeb != nil {
		a.broadcastUpdateToVPlayers(msg, dataWeb)
	}

	// Physical buzzer WS clients receive a minimal payload (PHASE/TIMER + slim bumper/team slices).
	if targetBuzzers {
		if dataBuzzer, err := msg.SerializeForBuzzer(); err == nil {
			a.buzzerHub.BroadcastRawIfRelevant(protocol.ActionUpdate, dataBuzzer)
		}
	}
}

// broadcastUpdateToPlayer sends a targeted UPDATE echo to exactly one
// VPlayer client — the participant whose own event (connection/reconnection,
// buzz, QCM answer, ...) just happened and who needs to see the resulting
// state of their own bumper (#129 T1.2, contracts/vplayer-payload-filter.md
// §5). Builds the same fresh GetGameJSON() snapshot broadcastUpdateTo would,
// but serializes it via Message.SerializeForVPlayer(playerID) — the #127
// reduction rule applies automatically if GAME.PHASE happens to be PREPARE/
// READY, complete payload otherwise, no rule duplicated here — and sends it
// to that one recipient only via SendRawToPlayerID. Never a broadcast: no
// other VPlayer receives anything from this call.
//
// MUST NOT call a.engine.ApplyVPlayerBroadcastConnEvents(): that evaluates
// MessageLost/DeliveryConfirmed for the WHOLE roster of VPlayers, on the
// assumption a broadcast just reached (or missed) all of them. Calling it
// here, on a send to a single recipient, would flag MessageLost on every
// OTHER VPlayer that received nothing from this particular send — same bug
// class as #127 CA7, same fix: simply never call it on this path (#129 CA8).
// No-op (silently) if playerID is empty or currently not connected —
// SendRawToPlayerID already handles the "not connected" case safely.
func (a *App) broadcastUpdateToPlayer(playerID string) {
	if a.wsHub == nil || playerID == "" {
		return
	}

	data := a.engine.GetGameJSON()
	msg, _ := protocol.NewMessage(protocol.ActionUpdate, nil)
	msg.Msg = data
	msg.Version = a.config.Version

	payload, err := msg.SerializeForVPlayer(playerID)
	if err != nil {
		server.LogError(game.LogComponentApp, "broadcastUpdateToPlayer(%s): SerializeForVPlayer failed: %v", playerID, err)
		return
	}
	a.wsHub.SendRawToPlayerID(playerID, payload)
}

// broadcastUpdateToVPlayers fans an UPDATE out to every connected VPlayer
// client, applying the #127 payload reduction (contracts/vplayer-payload-filter.md
// §2) when msg qualifies (UPDATE action, GAME.PHASE is PREPARE or READY):
// each identified VPlayer gets a payload carrying only its own bumper entry;
// every other VPlayer (not yet identified, or msg doesn't qualify) gets
// fallback — the same complete filtered payload TV receives.
//
// Perf-critical path (CA10/R3, contract §3): GAME and teams are kept as
// json.RawMessage and never re-parsed/re-marshaled as a whole per recipient
// (up to ~11KB for a MEMOTION question) — only the small "bumpers"
// sub-object is rebuilt per recipient. Recipients are snapshotted under
// RLock (SnapshotVPlayerRecipients); every payload is built entirely outside
// any lock; only the final byte pushes happen under a single Lock
// (SendRawToVPlayers). See BenchmarkVPlayerFanout (T2.4) for the measured
// cost at 10/30 VPlayers.
func (a *App) broadcastUpdateToVPlayers(msg *protocol.Message, fallback []byte) {
	recipients := a.wsHub.SnapshotVPlayerRecipients()
	if len(recipients) == 0 {
		// No identified VPlayer to personalize for, but a connected-and-not-
		// yet-identified VPlayer may still exist and needs the fallback.
		a.wsHub.SendRawToVPlayers(nil, fallback)
		return
	}

	payloads, ok := buildVPlayerPayloads(msg, recipients)
	if !ok {
		// msg doesn't qualify for reduction (not UPDATE, wrong phase, or
		// malformed) — same complete payload for every VPlayer, as before #127.
		a.wsHub.BroadcastRawToTypes(fallback, server.ClientTypeVPlayer)
		return
	}

	a.wsHub.SendRawToVPlayers(payloads, fallback)
}

// buildVPlayerPayloads is the CPU-bound core of the #127 individualized
// VPlayer fan-out: one reduced payload per recipient (contract §2), given
// the full UPDATE message and the set of identified VPlayer recipients.
// Deliberately factored out of broadcastUpdateToVPlayers, with no
// WebSocketHub/network involvement at all, so BenchmarkVPlayerFanout (T2.4)
// measures exactly this cost in isolation.
//
// ok=false means msg doesn't qualify for reduction (not an UPDATE, GAME.PHASE
// outside PREPARE/READY, or malformed JSON) — the caller falls back to the
// shared complete payload for every VPlayer instead.
//
// Perf-critical (CA10/R3, contract §3): GAME and teams are extracted once as
// json.RawMessage and never re-parsed/re-marshaled as a whole per recipient
// (up to ~11KB for a MEMOTION question) — only the small "bumpers"
// sub-object is rebuilt per recipient.
func buildVPlayerPayloads(msg *protocol.Message, recipients []server.VPlayerRecipient) (map[string][]byte, bool) {
	if msg.Action != protocol.ActionUpdate {
		return nil, false
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(msg.Msg, &envelope); err != nil {
		return nil, false
	}

	var phaseProbe struct {
		Phase string `json:"PHASE"`
	}
	if err := json.Unmarshal(envelope["GAME"], &phaseProbe); err != nil ||
		(phaseProbe.Phase != string(game.PhasePrepare) && phaseProbe.Phase != string(game.PhaseReady)) {
		return nil, false
	}

	var bumpers map[string]json.RawMessage
	if err := json.Unmarshal(envelope["bumpers"], &bumpers); err != nil {
		return nil, false
	}

	// #127 T2.4 checkpoint follow-up: BenchmarkVPlayerFanout showed the
	// obvious approach (build a map[string]json.RawMessage per recipient and
	// hand it to json.Marshal / Message.SerializeForWebSocket) was ~10-11x
	// costlier than the pre-#127 shared path for a MEMOTION question at 30
	// VPlayers (~3.3-3.7ms vs ~0.32ms) — encoding/json calls compact() on
	// every json.RawMessage value it marshals, an O(len) rescan of GAME
	// (up to ~11KB) repeated per recipient, not the re-parse we'd guarded
	// against but a real cost anyway. gameRaw/teamsRaw/actionJSON/versionJSON
	// are computed ONCE here (constant across recipients); the per-recipient
	// loop below builds each final frame by byte concatenation instead
	// (see buildVPlayerMessageBytes) — no json.Marshal ever touches
	// GAME/teams again after this point. Only the small per-recipient bumper
	// object (a few hundred bytes) and the playerID key still go through
	// json.Marshal — cheap, and it's the standard library's own string
	// escaping, reused rather than hand-rolled, for the "no raw string
	// concatenation of user-controlled data" rule this project follows.
	// See _work/reports/dev-backend-t2-benchmark-*.md for the byte-for-byte
	// equality tests and rebenchmark this rewrite was validated against.
	actionJSON, err := json.Marshal(msg.Action)
	if err != nil {
		return nil, false
	}
	var versionJSON []byte
	if msg.Version != "" {
		if versionJSON, err = json.Marshal(msg.Version); err != nil {
			return nil, false
		}
	}
	// Strip admin-only GAME fields (QUIZ_OBJECTIVES, v6.1.0 #137 Batch 2b)
	// once here, before the per-recipient loop — this hot path bypasses
	// SerializeForWebClient/SerializeForVPlayer entirely (it keeps GAME as
	// json.RawMessage and splices it in verbatim, see buildVPlayerMessageBytes
	// below), so without this call gameRaw would carry QUIZ_OBJECTIVES to
	// every VPlayer during PREPARE/READY — the confidentiality rule from
	// contracts/game-state.md would be enforced everywhere except here.
	gameRaw, err := stripAdminOnlyGameFields(envelope["GAME"])
	if err != nil {
		return nil, false
	}
	teamsRaw := envelope["teams"]

	payloads := make(map[string][]byte, len(recipients))
	for _, r := range recipients {
		ownRaw, present := bumpers[r.PlayerID]
		if !present {
			// Evicted between GetGameJSON() and this call — SendRawToVPlayers
			// falls back to the complete payload for this recipient.
			continue
		}
		var own map[string]interface{}
		if err := json.Unmarshal(ownRaw, &own); err != nil {
			continue
		}
		for _, field := range protocol.AdminOnlyBumperFields {
			delete(own, field)
		}
		strippedBumper, err := json.Marshal(own)
		if err != nil {
			continue
		}
		playerIDJSON, err := json.Marshal(r.PlayerID)
		if err != nil {
			continue
		}
		payloads[r.PlayerID] = buildVPlayerMessageBytes(actionJSON, versionJSON, gameRaw, teamsRaw, playerIDJSON, strippedBumper)
	}

	return payloads, true
}

// stripAdminOnlyGameFields returns a copy of the "GAME" node JSON with
// protocol.AdminOnlyGameFields (QUIZ_OBJECTIVES, v6.1.0 #137 Batch 2b)
// removed. Called once per broadcastUpdateToVPlayers fan-out (not per
// recipient) by buildVPlayerPayloads, since this hot path keeps GAME as
// json.RawMessage and splices it into every recipient's frame verbatim
// (buildVPlayerMessageBytes) — it never goes through
// Message.SerializeForWebClient/SerializeForVPlayer, which apply the same
// list on their own code paths (internal/protocol/messages.go). All three
// sites must agree (contracts/ws-payload-serialization.md).
func stripAdminOnlyGameFields(raw json.RawMessage) (json.RawMessage, error) {
	var gameNode map[string]interface{}
	if err := json.Unmarshal(raw, &gameNode); err != nil {
		return nil, err
	}
	for _, field := range protocol.AdminOnlyGameFields {
		delete(gameNode, field)
	}
	return json.Marshal(gameNode)
}

// buildVPlayerMessageBytes assembles the final WebSocket frame for one
// VPlayer recipient by direct byte concatenation — no json.Marshal call ever
// sees gameRaw or teamsRaw here, avoiding the compact()-rescan cost
// identified by BenchmarkVPlayerFanout (see buildVPlayerPayloads' comment
// above this call site).
//
// Must stay byte-for-byte identical to the reference (slow) path:
//
//	json.Marshal(&protocol.Message{Action: <action>, Version: <version>, Msg:
//	  mustMarshal(map[string]json.RawMessage{"GAME": gameRaw, "teams": teamsRaw,
//	    "bumpers": mustMarshal(map[string]json.RawMessage{playerID: strippedBumper})})})
//
// — verified by TestBuildVPlayerMessageBytes_MatchesReferencePath
// (cmd/server/vplayer_fanout_bytes_test.go), including playerID/bumper
// content with quotes, backslashes, and non-ASCII characters. This hardcodes
// two format facts that test locks in as a regression guard:
//   - encoding/json marshals map[string]T with keys sorted lexicographically,
//     so "GAME" < "bumpers" < "teams" (uppercase 'G' sorts before lowercase
//     letters in ASCII) is always the actual key order for both the outer
//     MSG object and Message's own field order (Action, then Version if
//     non-empty per its `omitempty` tag, then MSG — struct fields keep
//     declaration order, unlike map keys).
//   - gameRaw/teamsRaw, sourced from json.Unmarshal into a
//     map[string]json.RawMessage, are already the exact compact-JSON byte
//     span of the original message — no whitespace to strip, safe to splice
//     in verbatim.
//
// actionJSON/versionJSON/gameRaw/teamsRaw are identical for every recipient
// of one broadcast (computed once by the caller); playerIDJSON and
// strippedBumper are the only per-recipient inputs.
func buildVPlayerMessageBytes(actionJSON, versionJSON, gameRaw, teamsRaw, playerIDJSON, strippedBumper []byte) []byte {
	size := len(actionJSON) + len(versionJSON) + len(gameRaw) + len(teamsRaw) +
		len(playerIDJSON) + len(strippedBumper) + 64 // +64: literal braces/keys/colons/commas below
	buf := make([]byte, 0, size)
	buf = append(buf, `{"ACTION":`...)
	buf = append(buf, actionJSON...)
	if len(versionJSON) > 0 {
		buf = append(buf, `,"VERSION":`...)
		buf = append(buf, versionJSON...)
	}
	buf = append(buf, `,"MSG":{"GAME":`...)
	buf = append(buf, gameRaw...)
	buf = append(buf, `,"bumpers":{`...)
	buf = append(buf, playerIDJSON...)
	buf = append(buf, ':')
	buf = append(buf, strippedBumper...)
	buf = append(buf, `},"teams":`...)
	buf = append(buf, teamsRaw...)
	buf = append(buf, `}}`...)
	return buf
}

// updateBroadcasterFrequency adjusts the UDP heartbeat interval based on whether
// any known physical buzzer is currently disconnected. When at least one physical
// buzzer is disconnected, we broadcast at 500ms so it can rediscover the server
// quickly. When all are connected (or no physical buzzers exist), we revert to the
// normal 5s interval (enrollment mode overrides both).
func (a *App) updateBroadcasterFrequency() {
	if a.broadcaster == nil {
		// Not wired up in this context (e.g. unit tests instantiate App directly
		// without the UDP discovery broadcaster) — nothing to update.
		return
	}
	bumpers := a.engine.GetTeamsAndBumpers().Bumpers
	hasDisconnected := false
	for _, b := range bumpers {
		if !b.IsVirtual && !b.IsVPlayer && !b.Connected {
			hasDisconnected = true
			break
		}
	}
	a.broadcaster.SetHighFrequency(hasDisconnected)
}

func (a *App) broadcastEnrollmentUpdate() {
	state := a.engine.GetState()
	payload := protocol.EnrollmentUpdatePayload{
		VirtualPlayerCount: state.VirtualPlayerCount,
		VirtualPlayerLimit: state.VirtualPlayerLimit,
		EnrollmentActive:   state.EnrollmentActive,
	}
	msg, _ := protocol.NewMessage(protocol.ActionEnrollmentUpdate, payload)
	a.wsHub.BroadcastToTypes(msg, server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer)
	server.LogDebug(game.LogComponentApp, "Broadcasting ENROLLMENT_UPDATE: %d/%d players", state.VirtualPlayerCount, state.VirtualPlayerLimit)
}
// broadcastGameState fires on every OnStateChange phase transition (PREPARE,
// READY, STARTED, ...). #127 T1.3: routed through broadcastUpdateTo, the same
// filtered per-type serialization path as broadcastUpdate, instead of the
// generic a.broadcast() → wsHub.BroadcastToTypes() → SerializeForWebSocket(),
// which sent the exact same unfiltered admin-grade payload to Admin, TV AND
// VPlayer alike (contracts/vplayer-payload-filter.md §1). Deliberately does
// NOT target server.ClientTypeBuzzer — buzzers never received this broadcast
// before (verified: the old a.broadcast(..., false, ...) call had viaTCP=false
// and never listed a buzzer-facing type), and T1.3 must not introduce that.
func (a *App) broadcastGameState(phase string) {
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer)
}

func (a *App) broadcastStart() {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionStart, data, true,
		server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer)
	a.sendLEDSetAllBuzzers()
}

func (a *App) broadcastStop() {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionStop, data, true,
		server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer)
	a.sendLEDSetStop()
}

func (a *App) broadcastPause(bumperID string) {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionPause, data, true,
		server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer)
	a.sendLEDSetPause(bumperID)
}

func (a *App) broadcastPauseAll() {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionPause, data, true,
		server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer)
	a.sendLEDSetPauseAll()
}

func (a *App) broadcastContinue() {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionContinue, data, true,
		server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer)
	a.sendLEDSetContinue()
}

func (a *App) broadcastTimerUpdate(currentTime int) {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionUpdateTimer, data, true,
		server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer)
}

func (a *App) broadcastCountdownUpdate(countdownTime int) {
	data := a.engine.GetGameJSON()
	server.LogDebug(game.LogComponentApp, "Broadcasting countdown: %d", countdownTime)
	a.broadcast(protocol.ActionUpdateTimer, data, true,
		server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer)
}

func (a *App) broadcastPing() {
	msg, _ := protocol.NewMessage(protocol.ActionPing, map[string]interface{}{})
	a.buzzerHub.Broadcast(msg)
}

func (a *App) broadcastReady() {
	// Reset BuzzState for all buzzers at the start of each question round
	a.resetBuzzStates()
	data := a.engine.GetTeamsAndBumpersJSON()
	a.broadcast(protocol.ActionReady, data, true,
		server.ClientTypeAdmin, server.ClientTypeTV)
	a.sendLEDSetAllBuzzers()
}

func (a *App) broadcastReveal(answer string) {
	data, _ := json.Marshal(answer)
	a.broadcast(protocol.ActionReveal, data, true,
		server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer)
	a.sendLEDSetReveal(answer)
}

// answerColorToRGB maps a QCM AnswerColor to an [R, G, B] array for buzzer LED.
func answerColorToRGB(color game.AnswerColor) [3]int {
	switch color {
	case game.AnswerColorRed:
		return [3]int{255, 0, 0}
	case game.AnswerColorGreen:
		return [3]int{0, 255, 0}
	case game.AnswerColorYellow:
		return [3]int{255, 255, 0}
	case game.AnswerColorBlue:
		return [3]int{0, 0, 255}
	default:
		return [3]int{0, 0, 0}
	}
}

// teamColorPalette maps the 16 canonical team color keys (#113) to their exact RGB
// values, plus a handful of legacy English aliases pointing at the vivid tones for
// robustness. Used by teamColorToRGB when a team has a ColorName set.
//
// The 16 entries are 8 hues declined as a vivid tone (S=100% L=55%) and a deep tone
// (S=100% L=35%) — this is the normative table in contracts/models.md ("Palette
// d'équipes (#113)"). Frontend source of truth: web/src/constants/colors.js
// (TEAM_COLORS). The two tables MUST stay value-for-value identical, or a team's
// physical buzzer LED will diverge from its on-screen color.
var teamColorPalette = map[string][3]int{
	// Vivid tones (rank 1-8)
	"rouge":  {255, 26, 26},
	"red":    {255, 26, 26},
	"orange": {255, 133, 26},
	"jaune":  {255, 217, 26},
	"yellow": {255, 217, 26},
	"vert":   {26, 255, 83},
	"green":  {26, 255, 83},
	"cyan":   {26, 236, 255},
	"bleu":   {26, 94, 255},
	"blue":   {26, 94, 255},
	"violet": {159, 26, 255},
	"purple": {159, 26, 255},
	"rose":   {255, 26, 159},
	"pink":   {255, 26, 159},
	// Deep tones (rank 9-16)
	"rouge-profond":  {179, 0, 0},
	"orange-profond": {179, 83, 0},
	"jaune-profond":  {179, 149, 0},
	"vert-profond":   {0, 179, 45},
	"cyan-profond":   {0, 164, 179},
	"bleu-profond":   {0, 54, 179},
	"violet-profond": {104, 0, 179},
	"rose-profond":   {179, 0, 104},
}

// rgbToHue converts an RGB color (0-255 per channel) to a hue angle in degrees [0, 360).
// Returns -1 for achromatic colors (delta < 0.01).
func rgbToHue(r, g, b int) float64 {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	delta := max - min
	if delta < 0.01 {
		return -1 // achromatic
	}
	var h float64
	switch max {
	case rf:
		h = (gf - bf) / delta
		if gf < bf {
			h += 6
		}
	case gf:
		h = (bf-rf)/delta + 2
	default:
		h = (rf-gf)/delta + 4
	}
	return h * 60
}

// rgbSaturation returns the HSL saturation (0.0–1.0) for the given RGB.
func rgbSaturation(r, g, b int) float64 {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	l := (max + min) / 2
	if l == 0 || l == 1 {
		return 0
	}
	return (max - min) / (1 - math.Abs(2*l-1))
}

// rgbLightness returns the HSL lightness (0.0–1.0) for the given RGB.
func rgbLightness(r, g, b int) float64 {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	return (max + min) / 2
}

// dimIntensityFor returns the LED intensity (0-255) to use for a "dimmed team color"
// LED state, scaled to the perceived brightness of the team color rather than a flat
// constant. Used by sendLEDSetForBuzzerNormal (#113 B3) and by the MEMORY LED paths
// sendLEDSetForBuzzerMemory / sendLEDSetMemoryMultiTeam (#113 B3 fast-follow,
// code-review 20260726) — every place that dims the TEAM color. Not used by the QCM
// paths (sendLEDSetForBuzzerQCM/Reveal), which dim the ANSWER color at a flat 64 and
// are intentionally left untouched. The firmware applies intensity multiplicatively
// (channel × intensity / 255, see src/Common/led.h), so a flat 64 nearly extinguishes
// deep tones (L≈35%) while looking fine on vivid ones (L≈55%).
//
// Linear interpolation pinned at the two tone families defined by the #113 palette
// (contracts/models.md): L=55% (vivid) → 64 (unchanged from the pre-#113 behavior),
// L=35% (deep) → ~100. Result is clamped to [64, 128] so both ends of the color
// spectrum (near-white, near-black/achromatic fallback) stay within a sane dim range.
func dimIntensityFor(rgb [3]int) int {
	l := rgbLightness(rgb[0], rgb[1], rgb[2])
	intensity := 163.0 - 180.0*l
	i := int(math.Round(intensity))
	if i < 64 {
		i = 64
	}
	if i > 128 {
		i = 128
	}
	return i
}

// nearestPaletteColorByHue returns the teamColorPalette entry whose hue angle is closest
// to (r,g,b) using circular hue distance. For achromatic inputs (saturation < 0.15)
// returns white. Hue-based matching is more robust than Euclidean RGB distance for
// distinguishing orange/red, blue/violet, etc., and prevents two teams with the same
// raw RGB from mapping to the same palette entry.
func nearestPaletteColorByHue(r, g, b int) [3]int {
	if rgbSaturation(r, g, b) < 0.15 {
		return [3]int{255, 255, 255} // achromatic → white
	}
	hue := rgbToHue(r, g, b)
	if hue < 0 {
		return [3]int{255, 255, 255}
	}
	type entry struct {
		rgb [3]int
		hue float64
	}
	ordered := []entry{
		{[3]int{255, 26, 26}, 0},    // rouge
		{[3]int{255, 133, 26}, 28},  // orange
		{[3]int{255, 217, 26}, 50},  // jaune
		{[3]int{26, 255, 83}, 135},  // vert
		{[3]int{26, 236, 255}, 185}, // cyan
		{[3]int{26, 94, 255}, 222},  // bleu
		{[3]int{159, 26, 255}, 275}, // violet
		{[3]int{255, 26, 159}, 325}, // rose
	}
	best := ordered[0].rgb
	minDist := 361.0
	for _, e := range ordered {
		d := math.Abs(hue - e.hue)
		if d > 180 {
			d = 360 - d // circular hue distance
		}
		if d < minDist {
			minDist = d
			best = e.rgb
		}
	}
	return best
}

// teamColorToRGB returns the LED RGB color for a bumper's team, or gray [128,128,128] if no team.
// Resolution order:
//  1. team.ColorName (explicit name set by frontend) → direct palette lookup
//  2. Hue-based nearest palette color (more robust than Euclidean distance)
func (a *App) teamColorToRGB(bumper *game.Bumper) [3]int {
	if bumper.Team == "" {
		return [3]int{128, 128, 128}
	}
	team := a.engine.GetTeam(bumper.Team)
	if team == nil {
		return [3]int{128, 128, 128}
	}
	// 1. Named color lookup (explicit, highest priority)
	if team.ColorName != "" {
		if rgb, ok := teamColorPalette[strings.ToLower(team.ColorName)]; ok {
			return rgb
		}
	}
	// 2. Hue-based nearest palette color
	if len(team.Color) < 3 {
		return [3]int{128, 128, 128}
	}
	return nearestPaletteColorByHue(team.Color[0], team.Color[1], team.Color[2])
}

// sendLEDSet sends a LED_SET message to a specific buzzer and stores the state for reconnect.
// A MSG_ID is attached so the buzzer can ACK receipt (v3.8.0 ACK protocol).
func (a *App) sendLEDSet(mac string, payload protocol.LEDSetPayload) {
	a.bumperLEDState[mac] = payload
	if !a.buzzerHub.IsClientConnected(mac) {
		// #109 Phase 2 (D4): a LED_SET that should have reached this buzzer while
		// it's disconnected counts as a lost message (orange -> red).
		a.engine.TransitionConn(mac, game.ConnEventMessageLost)
		return
	}
	msg, err := protocol.NewMessage(protocol.ActionLEDSet, payload)
	if err != nil {
		server.LogError(game.LogComponentApp, "Failed to create LED_SET message for %s: %v", mac, err)
		return
	}
	// Attach MSG_ID for ACK tracking
	msgID := server.GenerateMsgID()
	msg.MsgID = msgID

	if err := a.buzzerHub.SendToClient(mac, msg); err != nil {
		server.LogWarn(game.LogComponentApp, "Failed to send LED_SET to buzzer %s: %v", mac, err)
	} else {
		server.LogInfo(game.LogComponentApp, "LED_SET sent to %s (msgID=%s): RGB(%d,%d,%d) intensity=%d effect=%s",
			mac, msgID, payload.Color[0], payload.Color[1], payload.Color[2], payload.Intensity, payload.Effect)
		// Register for ACK tracking; set AckPending on bumper.
		// NOTE: broadcastUpdate() is intentionally NOT called here — callers are responsible for
		// a single broadcast after the full loop completes, avoiding N redundant UPDATE messages.
		a.ackManager.Register(mac, msgID, protocol.ActionLEDSet)
		a.engine.UpdateBumper(mac, map[string]interface{}{"ACK_PENDING": true})
	}
}

// broadcastLEDSet sends the same LED_SET payload to all connected buzzers.
//
// #132 audit: currently DEAD CODE — grep confirms zero call sites anywhere
// in the codebase (including tests). Fixed anyway, for the same reason
// dead code still gets its imports checked: if this is ever wired up, it
// should not silently reintroduce the exact #127/#129 bug class. Flagged
// separately in the audit report as its own finding (candidate for removal
// — not done here, out of this bugfix's scope).
func (a *App) broadcastLEDSet(payload protocol.LEDSetPayload) {
	tb := a.engine.GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		a.sendLEDSet(mac, payload)
	}
	// One broadcast for all AckPending state changes set by the loop above.
	// #132 (same pattern/justification as sendLEDSetAllBuzzers #127/#129-T1.6
	// and sendLEDSetPause #129 T3.1-adjacent): the loop above unconditionally
	// skips IsVPlayer bumpers, and ACK_PENDING — the only field this call
	// exists to announce — is always stripped from TV's payload too
	// (protocol.AdminOnlyBumperFields, via SerializeForWebClient). Neither
	// client type can ever see anything this call carries.
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
}

// resendLEDOnReconnect re-sends the last known LED state to a buzzer that just reconnected (HELLO).
func (a *App) resendLEDOnReconnect(mac string) {
	payload, ok := a.bumperLEDState[mac]
	if !ok {
		// No stored state — derive from current game state
		a.sendLEDSetForBuzzer(mac)
		return
	}
	msg, err := protocol.NewMessage(protocol.ActionLEDSet, payload)
	if err != nil {
		return
	}
	if err := a.buzzerHub.SendToClient(mac, msg); err != nil {
		server.LogWarn(game.LogComponentApp, "Failed to resend LED_SET to %s: %v", mac, err)
	} else {
		server.LogInfo(game.LogComponentApp, "LED_SET resent on reconnect to %s", mac)
	}
}

// resetBuzzStates resets BuzzState to NONE for all known buzzers.
// Called at the start of each question round (READY phase).
func (a *App) resetBuzzStates() {
	tb := a.engine.GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	for mac := range tb.Bumpers {
		a.bumperBuzzState[mac] = game.BuzzStateNone
	}
}

// updateBuzzStates updates BuzzState for all buzzers after buzzingMAC has pressed.
//
// Rules (from BUZZER_LED_STATE_MACHINE.md):
//   - The buzzing bumper becomes MOI (first buzz of its team).
//   - All other bumpers of the same team become EQUIPE.
//   - Bumpers of other teams that had no buzz yet become AUTRE.
//   - Existing MOI/EQUIPE/AUTRE states of already-buzzed teams are preserved.
func (a *App) updateBuzzStates(buzzingMAC string) {
	buzzingBumper := a.engine.GetBumper(buzzingMAC)
	if buzzingBumper == nil {
		return
	}
	buzzingTeam := buzzingBumper.Team

	tb := a.engine.GetTeamsAndBumpers()
	if tb == nil {
		return
	}

	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		current := a.bumperBuzzState[mac]

		if mac == buzzingMAC {
			// The buzzing bumper itself: MOI (first of its team)
			a.bumperBuzzState[mac] = game.BuzzStateMoi
		} else if bumper.Team == buzzingTeam {
			// Same team as the buzzer: EQUIPE (unless already MOI — impossible here
			// since the engine only allows one buzz per team, but defensive check)
			if current != game.BuzzStateMoi {
				a.bumperBuzzState[mac] = game.BuzzStateEquipe
			}
		} else {
			// Different team: promote to AUTRE only if still NONE
			if current == game.BuzzStateNone {
				a.bumperBuzzState[mac] = game.BuzzStateAutre
			}
			// If already MOI/EQUIPE/AUTRE, leave unchanged (their team buzzed earlier)
		}
	}
}

// buzzStateFor returns the current BuzzState for a given buzzer MAC.
// Defaults to NONE if not tracked yet.
func (a *App) buzzStateFor(mac string) game.BuzzState {
	if bs, ok := a.bumperBuzzState[mac]; ok {
		return bs
	}
	return game.BuzzStateNone
}

// sendLEDSetForBuzzer derives and sends the correct LED state for one buzzer based on current game phase.
// Used on reconnect when no stored LED state is available.
func (a *App) sendLEDSetForBuzzer(mac string) {
	bumper := a.engine.GetBumper(mac)
	if bumper == nil || bumper.IsVPlayer {
		return
	}

	state := a.engine.GetState()
	phase := a.engine.GetPhase()

	switch {
	case state.Question != nil && state.Question.Type == game.QuestionTypeQCM:
		a.sendLEDSetForBuzzerQCM(mac, bumper, phase, state)
	case state.Question != nil && state.Question.Type == game.QuestionTypeMemory:
		a.sendLEDSetForBuzzerMemory(mac, bumper, phase, state)
	case state.Question != nil && state.Question.Type == game.QuestionTypeMemotion:
		// MEMOTION: buzzers display team color (same as NORMAL stopped state)
		a.sendLEDSetForBuzzerNormal(mac, bumper, phase)
	default:
		a.sendLEDSetForBuzzerNormal(mac, bumper, phase)
	}
}

func (a *App) sendLEDSetForBuzzerNormal(mac string, bumper *game.Bumper, phase game.GamePhase) {
	rgb := a.teamColorToRGB(bumper)
	dimIntensity := dimIntensityFor(rgb)
	bs := a.buzzStateFor(mac)
	switch phase {
	case game.PhaseStopped, game.PhasePrepare, game.PhaseReady, game.PhaseCountdown:
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
	case game.PhaseStarted:
		switch bs {
		case game.BuzzStateMoi:
			a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "BLINK"})
		case game.BuzzStateEquipe:
			a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
		default: // NONE or AUTRE
			a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: dimIntensity, Effect: "DIM"})
		}
	case game.PhasePaused:
		switch bs {
		case game.BuzzStateMoi:
			a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "BLINK"})
		case game.BuzzStateEquipe:
			a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
		default: // NONE or AUTRE
			a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: dimIntensity, Effect: "DIM"})
		}
	case game.PhaseRevealed:
		switch bs {
		case game.BuzzStateMoi:
			a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "BLINK"})
		case game.BuzzStateEquipe:
			a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
		default: // NONE or AUTRE
			a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: dimIntensity, Effect: "DIM"})
		}
	default:
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
	}
}

func (a *App) sendLEDSetForBuzzerQCM(mac string, bumper *game.Bumper, phase game.GamePhase, state game.GameState) {
	answerRGB := answerColorToRGB(bumper.AnswerColor)
	teamRGB := a.teamColorToRGB(bumper)
	bs := a.buzzStateFor(mac)

	switch phase {
	case game.PhaseStopped, game.PhasePrepare:
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: teamRGB, Intensity: 255, Effect: "SOLID"})
	case game.PhaseReady, game.PhaseCountdown:
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 255, Effect: "SOLID"})
	case game.PhaseStarted:
		if bs == game.BuzzStateMoi || bs == game.BuzzStateEquipe {
			// My team buzzed → show team color (hides the answer)
			a.sendLEDSet(mac, protocol.LEDSetPayload{Color: teamRGB, Intensity: 255, Effect: "SOLID"})
		} else {
			// NONE or AUTRE → show answer color
			a.sendLEDSet(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 255, Effect: "SOLID"})
		}
	case game.PhasePaused:
		if bs == game.BuzzStateMoi || bs == game.BuzzStateEquipe {
			a.sendLEDSet(mac, protocol.LEDSetPayload{Color: teamRGB, Intensity: 255, Effect: "SOLID"})
		} else {
			a.sendLEDSet(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 255, Effect: "SOLID"})
		}
	case game.PhaseRevealed:
		// Revealed logic uses firstBuzzTeam from engine state (team.Time > 0 && team.Bumper == mac or same team)
		a.sendLEDSetForBuzzerQCMReveal(mac, bumper, state)
	default:
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: teamRGB, Intensity: 255, Effect: "SOLID"})
	}
}

// sendLEDSetForBuzzerQCMReveal handles the REVEALED phase LED for a single QCM buzzer.
func (a *App) sendLEDSetForBuzzerQCMReveal(mac string, bumper *game.Bumper, state game.GameState) {
	answerRGB := answerColorToRGB(bumper.AnswerColor)
	bs := a.buzzStateFor(mac)
	myTeamBuzzed := bs == game.BuzzStateMoi || bs == game.BuzzStateEquipe

	if !myTeamBuzzed {
		// No buzz from my team: DIM 25% answer color
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 64, Effect: "DIM"})
		return
	}

	// My team buzzed — determine correct answer from current question
	correctAnswer := ""
	if state.Question != nil {
		correctAnswer = state.Question.QCMCorrect
	}
	myAnswerCorrect := correctAnswer != "" && string(bumper.AnswerColor) == correctAnswer

	if !myAnswerCorrect {
		// Wrong answer (or no correct defined): DIM 25%
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 64, Effect: "DIM"})
		return
	}

	// Correct answer — check if my team was the first to buzz (team.Time == smallest among buzzed teams)
	isFirstBuzz := a.isFirstBuzzTeam(bumper.Team)
	if isFirstBuzz {
		// Good answer + first buzz: BLINK
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 255, Effect: "BLINK"})
	} else {
		// Good answer + not first buzz: SOLID 100%
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: answerRGB, Intensity: 255, Effect: "SOLID"})
	}
}

// isFirstBuzzTeam returns true if the given team was the first team to buzz in this round.
// Uses bumper.Time values stored by the engine to find the earliest buzz.
func (a *App) isFirstBuzzTeam(teamName string) bool {
	if teamName == "" {
		return false
	}
	tb := a.engine.GetTeamsAndBumpers()
	if tb == nil {
		return false
	}

	// Find the earliest buzz time across all teams
	var minTime int64 = 0
	teamEarliestTime := int64(0)

	for _, bumper := range tb.Bumpers {
		if bumper.IsVPlayer || bumper.Time <= 0 {
			continue
		}
		if minTime == 0 || bumper.Time < minTime {
			minTime = bumper.Time
		}
		if bumper.Team == teamName {
			if teamEarliestTime == 0 || bumper.Time < teamEarliestTime {
				teamEarliestTime = bumper.Time
			}
		}
	}

	return teamEarliestTime > 0 && teamEarliestTime == minTime
}

func (a *App) sendLEDSetForBuzzerMemory(mac string, bumper *game.Bumper, phase game.GamePhase, state game.GameState) {
	rgb := a.teamColorToRGB(bumper)
	dimIntensity := dimIntensityFor(rgb)
	memoryMode := ""
	if state.Question != nil {
		memoryMode = state.Question.MemoryMode
	}

	switch phase {
	case game.PhaseStopped, game.PhasePrepare, game.PhaseReady, game.PhaseRevealed:
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
	case game.PhasePaused:
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: dimIntensity, Effect: "DIM"})
	case game.PhaseStarted:
		if memoryMode == string(game.MemoryModeSolo) || memoryMode == "" {
			// SOLO: active team = SOLID 100%, inactive = DIM 25%
			if bumper.Team == state.MemoryCurrentTeam {
				a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
			} else {
				a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: dimIntensity, Effect: "DIM"})
			}
		} else {
			// Multi-team modes: active=SOLID 100%, next=SOLID 50%, others participating=DIM 25%, not selected=OFF
			a.sendLEDSetMemoryMultiTeam(mac, bumper, state)
		}
	default:
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
	}
}

// sendLEDSetMemoryMultiTeam computes the LED for a buzzer in a multi-team Memory round.
func (a *App) sendLEDSetMemoryMultiTeam(mac string, bumper *game.Bumper, state game.GameState) {
	rgb := a.teamColorToRGB(bumper)

	// Not in participating teams → OFF
	participating := false
	for _, t := range state.MemoryParticipatingTeams {
		if t == bumper.Team {
			participating = true
			break
		}
	}
	if !participating {
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: [3]int{0, 0, 0}, Intensity: 0, Effect: "SOLID"})
		return
	}

	// Active team
	if bumper.Team == state.MemoryCurrentTeam {
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
		return
	}

	// Determine "next" team
	nextTeam := a.nextMemoryTeam(state)
	if nextTeam != "" && bumper.Team == nextTeam {
		// Next team: SOLID 50% (INTENSITY=128)
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 128, Effect: "SOLID"})
		return
	}

	// Other participating teams: DIM 25% (tone-relative, #113 B3 fast-follow —
	// same rationale as sendLEDSetForBuzzerNormal: a flat 64 nearly extinguishes
	// deep-toned team colors).
	a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: dimIntensityFor(rgb), Effect: "DIM"})
}

// nextMemoryTeam returns the name of the team that plays after MemoryCurrentTeam,
// cycling through MemoryParticipatingTeams in order.
func (a *App) nextMemoryTeam(state game.GameState) string {
	teams := state.MemoryParticipatingTeams
	if len(teams) < 2 {
		return ""
	}
	for i, t := range teams {
		if t == state.MemoryCurrentTeam {
			return teams[(i+1)%len(teams)]
		}
	}
	return ""
}

// sendLEDSetAllBuzzers sends per-buzzer LED_SET based on current game state (READY/START/STOPPED phase).
// Called after READY, START, and FULL updates.
func (a *App) sendLEDSetAllBuzzers() {
	tb := a.engine.GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	state := a.engine.GetState()
	phase := a.engine.GetPhase()

	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		switch {
		case state.Question != nil && state.Question.Type == game.QuestionTypeQCM:
			a.sendLEDSetForBuzzerQCM(mac, bumper, phase, state)
		case state.Question != nil && state.Question.Type == game.QuestionTypeMemory:
			a.sendLEDSetForBuzzerMemory(mac, bumper, phase, state)
		case state.Question != nil && state.Question.Type == game.QuestionTypeMemotion:
			// MEMOTION: buzzers display team color (same as NORMAL)
			a.sendLEDSetForBuzzerNormal(mac, bumper, phase)
		default:
			a.sendLEDSetForBuzzerNormal(mac, bumper, phase)
		}
	}
	// One broadcast for all AckPending state changes set by the loop above.
	// #127: the loop above unconditionally skips IsVPlayer bumpers (line
	// 3077), so this broadcast never carries a change relevant to any
	// VJoueur, at any of this function's call sites — targeting VPlayer here
	// was always a needless full-GameState resend to every connected
	// VJoueur. Found while verifying #127 CA1 empirically: it was firing an
	// extra unconditional UPDATE at every PREPARE->READY transition (via
	// broadcastReady -> sendLEDSetAllBuzzers), on top of the legitimate one
	// from TransitionToReady's OnStateChange.
	//
	// #129: TV removed too, at all 11 call sites, for a stronger reason than
	// the VPlayer case above — ACK_PENDING (the very field this broadcast
	// exists to announce) is one of the 5 fields SerializeForWebClient always
	// strips from TV's payload (protocol.AdminOnlyBumperFields). TV can never
	// see the content this call carries, regardless of phase or call site —
	// unlike the VPlayer case, this isn't conditional on the loop skipping
	// virtual bumpers, it's the serializer itself making the field invisible
	// to TV unconditionally. Found while verifying #129 CA12 empirically: TV
	// was still receiving 3 UPDATE in the PREPARE->READY window instead of
	// the 2 the contract promises (§1), this call being the 3rd, redundant
	// with the legitimate READY-transition broadcast from
	// TransitionToReady's OnStateChange. Admin still needs it (ACK_PENDING
	// spinner UI); Buzzer still needs it (LED sync, its own fields).
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
}

// sendLEDSetStop broadcasts team color SOLID 100% to all buzzers (game stopped/prepare).
func (a *App) sendLEDSetStop() {
	tb := a.engine.GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		rgb := a.teamColorToRGB(bumper)
		a.sendLEDSet(mac, protocol.LEDSetPayload{Color: rgb, Intensity: 255, Effect: "SOLID"})
	}
	// One broadcast for all AckPending changes.
	// #132 audit: same pattern as sendLEDSetAllBuzzers/sendLEDSetPause
	// (#127/#129) — loop always skips IsVPlayer, ACK_PENDING always stripped
	// from TV. Sole call site (broadcastStop) already sends its own
	// Admin+TV+VPlayer ActionStop broadcast right before this — TV/VPlayer
	// already have everything they need from that call; this one only ever
	// carried ACK_PENDING, which neither can see anyway.
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
}

// sendLEDSetPause sends per-buzzer LED state when a specific buzzer has buzzed (PAUSED phase).
// Uses BuzzState to determine each buzzer's LED.
func (a *App) sendLEDSetPause(bumperID string) {
	tb := a.engine.GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	state := a.engine.GetState()
	phase := a.engine.GetPhase()

	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		switch {
		case state.Question != nil && state.Question.Type == game.QuestionTypeQCM:
			a.sendLEDSetForBuzzerQCM(mac, bumper, phase, state)
		case state.Question != nil && state.Question.Type == game.QuestionTypeMemory:
			a.sendLEDSetForBuzzerMemory(mac, bumper, phase, state)
		default:
			a.sendLEDSetForBuzzerNormal(mac, bumper, phase)
		}
	}
	// One broadcast for all AckPending changes.
	// #129 (found while verifying Phase 3/CA10 empirically — same pattern
	// and justification as sendLEDSetAllBuzzers's #127/#129-T1.6 narrowing):
	// the loop above unconditionally skips IsVPlayer bumpers, and
	// ACK_PENDING (the field this broadcast exists to announce) is always
	// stripped from TV's payload too (protocol.AdminOnlyBumperFields, via
	// SerializeForWebClient) — neither client type can ever see anything
	// this call carries, at either of sendLEDSetPause's two call sites
	// (broadcastPause, broadcastPauseAll → sendLEDSetPauseAll). Without this,
	// every buzz/QCM-answer/etc. still sent a full, un-targeted UPDATE to
	// every VPlayer right next to it, defeating T3.1's targeting entirely —
	// this is what main_broadcast_129_phase3_test.go's CA10 tests caught.
	// broadcastPause's OWN direct PAUSE broadcast (the one the plan says to
	// leave untouched — it still reaches Admin/TV/VPlayer) is a separate
	// call, unaffected by this line.
	//
	// The 5 sibling sendLEDSet* functions with the identical pattern
	// (broadcastLEDSet, sendLEDSetStop, sendLEDSetReveal, sendLEDSetToTeam,
	// sendLEDSetComet) were audited and fixed the same way under #132 —
	// see their own doc comments and _work/reports/dev-backend-132-audit-*.md.
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
}

// sendLEDSetPauseAll sends LED state for PAUSE ALL (admin-initiated pause, no specific buzzer).
func (a *App) sendLEDSetPauseAll() {
	a.sendLEDSetPause("")
}

// sendLEDSetContinue sends LED state after the game resumes from pause (back to STARTED).
// Uses the same per-buzzer state machine as sendLEDSetAllBuzzers but preserves BuzzStates.
func (a *App) sendLEDSetContinue() {
	a.sendLEDSetAllBuzzers()
}

// sendLEDSetReveal sends per-buzzer LED feedback at REVEALED phase.
func (a *App) sendLEDSetReveal(correctAnswer string) {
	tb := a.engine.GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	state := a.engine.GetState()
	phase := a.engine.GetPhase()

	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		switch {
		case state.Question != nil && state.Question.Type == game.QuestionTypeQCM:
			a.sendLEDSetForBuzzerQCM(mac, bumper, phase, state)
		case state.Question != nil && state.Question.Type == game.QuestionTypeMemory:
			a.sendLEDSetForBuzzerMemory(mac, bumper, phase, state)
		default:
			a.sendLEDSetForBuzzerNormal(mac, bumper, phase)
		}
	}
	// One broadcast for all AckPending changes.
	// #132 audit: same pattern as sendLEDSetAllBuzzers/sendLEDSetPause
	// (#127/#129) — loop always skips IsVPlayer, ACK_PENDING always stripped
	// from TV. Sole call site (broadcastReveal) already sends its own
	// Admin+TV+VPlayer ActionReveal broadcast right before this.
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
}

// sendLEDSetToTeam sends a LED_SET payload to all physical buzzers belonging to teamID.
// If teamID is empty, sends to ALL physical buzzers (excluding VPlayers).
//
// #132 audit: currently DEAD CODE — grep confirms zero call sites anywhere
// in the codebase (including tests). Fixed anyway — see broadcastLEDSet's
// doc comment for the reasoning; both are flagged together in the audit
// report as candidates for removal in a separate cleanup, not done here.
func (a *App) sendLEDSetToTeam(teamID string, payload protocol.LEDSetPayload) {
	tb := a.engine.GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		if teamID != "" && bumper.Team != teamID {
			continue
		}
		a.sendLEDSet(mac, payload)
	}
	// One broadcast for all AckPending changes.
	// #132: same pattern/justification as the other sendLEDSet* functions —
	// loop always skips IsVPlayer, ACK_PENDING always stripped from TV.
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
}

// cometBandColor returns the best-contrasting band color for a COMET animation over bgColor.
// Uses gold [255,215,0] by default; switches to white [255,255,255] when the background is
// too close to gold (squared Euclidean distance < 8000, ~89 units).
func cometBandColor(bg [3]int) [3]int {
	gold := [3]int{255, 215, 0}
	dr := bg[0] - gold[0]
	dg := bg[1] - gold[1]
	db := bg[2] - gold[2]
	if dr*dr+dg*dg+db*db < 8000 {
		return [3]int{255, 255, 255} // white for yellow/gold teams
	}
	return gold
}

// sendLEDSetComet sends a COMET LED effect to all physical buzzers of the given team.
// COLOR = team background color; COMET_COLOR = contrasting band color (gold or white).
// Duration: 2 laps × 23 steps × 100 ms = 4600 ms + 200 ms margin = 4800 ms.
func (a *App) sendLEDSetComet(teamID string) {
	tb := a.engine.GetTeamsAndBumpers()
	if tb == nil {
		return
	}
	for mac, bumper := range tb.Bumpers {
		if bumper.IsVPlayer {
			continue
		}
		if teamID != "" && bumper.Team != teamID {
			continue
		}
		teamColor := a.teamColorToRGB(bumper)
		band := cometBandColor(teamColor)
		a.sendLEDSet(mac, protocol.LEDSetPayload{
			Color:      teamColor,
			Intensity:  255,
			Effect:     protocol.LEDEffectComet,
			CometColor: &band,
		})
	}
	// One broadcast for all AckPending changes from the loop above.
	// #132 audit: same pattern as sendLEDSetAllBuzzers/sendLEDSetPause
	// (#127/#129) — loop always skips IsVPlayer, ACK_PENDING always stripped
	// from TV. All 4 call sites (handlePoints, MEMOTION_DONE, BUMPER_POINTS,
	// handleTeamPoints) already call their own unconditional broadcastUpdate()
	// right after this returns, to announce the score change itself — by
	// then ACK_PENDING is already set (sendLEDSet sets it synchronously), so
	// Admin still sees it on that later broadcast; nothing is lost, only the
	// redundant TV/VPlayer leg of THIS broadcast is removed.
	a.broadcastUpdateTo(server.ClientTypeAdmin, server.ClientTypeBuzzer)
	// Restore normal LED state after firmware COMET animation completes.
	time.AfterFunc(4800*time.Millisecond, func() {
		a.sendLEDSetAllBuzzers()
	})
}

func (a *App) broadcastReset() {
	msg, _ := protocol.NewMessage(protocol.ActionReset, map[string]interface{}{})
	a.wsHub.BroadcastToTypes(msg, server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer)
	a.udpBcast.Broadcast(msg)
	// Note: RESET not sent to buzzer WS — HELLO below re-establishes buzzer state

	// Then send HELLO
	a.broadcastHello()
}

func (a *App) broadcastRemote() {
	data := a.engine.GetGameJSON()
	a.broadcast(protocol.ActionRemote, data, false,
		server.ClientTypeAdmin, server.ClientTypeTV)
}

func (a *App) broadcastClientCounts() {
	adminCount, tvCount, vplayerCount := a.wsHub.GetClientCounts()
	payload := protocol.ClientsPayload{
		AdminCount:   adminCount,
		TVCount:      tvCount,
		VPlayerCount: vplayerCount,
		BuzzerWS:     a.buzzerHub.BuzzerCount(),
	}
	data, _ := json.Marshal(payload)
	a.broadcast(protocol.ActionClients, data, false, server.ClientTypeAdmin)
	server.LogDebug(game.LogComponentWebSocket, "Client counts: admin=%d, tv=%d, vplayer=%d, buzzer_ws=%d",
		adminCount, tvCount, vplayerCount, payload.BuzzerWS)
}

func (a *App) broadcastBackgroundChange(index int) {
	payload := protocol.BackgroundChangePayload{
		Index: index,
	}
	data, _ := json.Marshal(payload)
	a.broadcast(protocol.ActionBackgroundChange, data, false,
		server.ClientTypeAdmin, server.ClientTypeTV)
	server.LogDebug(game.LogComponentApp, "Background change: index=%d", index)
}

func (a *App) broadcastQCMHint(invalidatedColor string, remainingAnswers int) {
	payload := protocol.QCMHintPayload{
		Color:     invalidatedColor,
		Remaining: remainingAnswers,
	}
	data, _ := json.Marshal(payload)
	a.broadcast(protocol.ActionQCMHint, data, false,
		server.ClientTypeAdmin, server.ClientTypeTV, server.ClientTypeVPlayer)
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
	a.broadcast(protocol.ActionShowQRCode, data, false,
		server.ClientTypeAdmin, server.ClientTypeTV)
	server.LogInfo(game.LogComponentApp, "QR Code enrollment activated")
}

func (a *App) broadcastHideQRCode() {
	payload := protocol.QRCodePayload{
		URL:  "",
		Show: false,
	}
	data, _ := json.Marshal(payload)
	a.broadcast(protocol.ActionHideQRCode, data, false,
		server.ClientTypeAdmin, server.ClientTypeTV)
	server.LogInfo(game.LogComponentApp, "QR Code enrollment deactivated")
}

func (a *App) broadcastConfigUpdate() {
	cfg := config.Get()
	payload := protocol.ConfigUpdatePayload{
		NeonEffect: protocol.NeonEffectPayload{
			Enabled:        cfg.NeonEffect.Enabled,
			Mode:           cfg.NeonEffect.Mode,
			ArcWidth:       cfg.NeonEffect.ArcWidth,
			IntensityGap:   cfg.NeonEffect.IntensityGap,
			RotationSpeed:  cfg.NeonEffect.RotationSpeed,
			BarOffset:      cfg.NeonEffect.BarOffset,
			BarThickness:   cfg.NeonEffect.BarThickness,
			ArcBlur:        cfg.NeonEffect.ArcBlur,
			GlowPulseSpeed: cfg.NeonEffect.GlowPulseSpeed,
			GlowPulseMin:   cfg.NeonEffect.GlowPulseMin,
			GlowPulseMax:   cfg.NeonEffect.GlowPulseMax,
		},
		DefaultQuestionImageIsCustom: a.httpServer.HasCustomDefaultQuestionImage(),
		NewGameBackgrounds:           a.engine.GetNewGameBackgrounds(),
	}
	data, _ := json.Marshal(payload)
	a.broadcast(protocol.ActionConfigUpdate, data, false,
		server.ClientTypeAdmin, server.ClientTypeTV)
	server.LogInfo(game.LogComponentApp, "Config update broadcast (neon: enabled=%v, mode=%s, arc=%d, intensity=%d, speed=%.1f, pulsing=%.1f-%d%%, offset=%d, thickness=%d, blur=%d)",
		cfg.NeonEffect.Enabled, cfg.NeonEffect.Mode, cfg.NeonEffect.ArcWidth, cfg.NeonEffect.IntensityGap, cfg.NeonEffect.RotationSpeed, cfg.NeonEffect.GlowPulseSpeed, cfg.NeonEffect.GlowPulseMax, cfg.NeonEffect.BarOffset, cfg.NeonEffect.BarThickness, cfg.NeonEffect.ArcBlur)
}

// resolveServerIP returns the actual server IP to send to buzzers.
// It uses the first IP from GetServerIPs() (same source as UDP broadcast heartbeats)
// so that buzzers always receive the current server IP regardless of what is stored in config.json.
// Falls back to cfg.ServerIP if no active interface is found.
func resolveServerIP(cfgServerIP string) string {
	ips := server.GetServerIPs()
	if len(ips) > 0 {
		server.LogInfo(game.LogComponentApp, "resolveServerIP: using detected IP %s (GetServerIPs=%v)", ips[0], ips)
		return ips[0]
	}
	server.LogInfo(game.LogComponentApp, "resolveServerIP: no detected IP, falling back to config value %q", cfgServerIP)
	return cfgServerIP
}

// sendWifiConfigToBuzzer sends the current WiFi defaults config to a specific buzzer via WebSocket.
// A MSG_ID is attached so the buzzer can ACK receipt (v3.8.0 ACK protocol).
func (a *App) sendWifiConfigToBuzzer(clientID string) {
	cfg := a.config.WiFiDefaults
	payload := protocol.WifiConfigPayload{
		SSID:       cfg.SSID,
		Pass:       cfg.Password,
		ServerIP:   resolveServerIP(cfg.ServerIP),
		ServerPort: cfg.ServerPort,
		SSID2:      cfg.SSID2,
		Pass2:      cfg.Password2,
	}
	msg, err := protocol.NewMessage(protocol.ActionWifiConfig, payload)
	if err != nil {
		server.LogError(game.LogComponentApp, "Failed to create WIFI_CONFIG message: %v", err)
		return
	}
	// Attach MSG_ID for ACK tracking (v3.8.0)
	msgID := server.GenerateMsgID()
	msg.MsgID = msgID

	if !a.buzzerHub.IsClientConnected(clientID) {
		// #109 Phase 2 (D4): WIFI_CONFIG emitted to an already-known-disconnected buzzer.
		a.engine.TransitionConn(clientID, game.ConnEventMessageLost)
	}
	if err := a.buzzerHub.SendToClient(clientID, msg); err != nil {
		server.LogError(game.LogComponentApp, "Failed to send WIFI_CONFIG to %s: %v", clientID, err)
		return
	}
	server.LogInfo(game.LogComponentApp, "Sent WIFI_CONFIG to buzzer %s (msgID=%s)", clientID, msgID)
	// Register for ACK tracking; set AckPending on bumper
	a.ackManager.Register(clientID, msgID, protocol.ActionWifiConfig)
	a.engine.UpdateBumper(clientID, map[string]interface{}{"ACK_PENDING": true})
	a.broadcastUpdate()
}

// broadcastWifiConfig sends the current WiFi defaults config to each connected buzzer individually.
// Each buzzer receives a unique MSG_ID for ACK tracking (v3.8.0); Broadcast() is avoided so that
// each message has a distinct MSG_ID.
func (a *App) broadcastWifiConfig() {
	cfg := a.config.WiFiDefaults
	payload := protocol.WifiConfigPayload{
		SSID:       cfg.SSID,
		Pass:       cfg.Password,
		ServerIP:   resolveServerIP(cfg.ServerIP),
		ServerPort: cfg.ServerPort,
		SSID2:      cfg.SSID2,
		Pass2:      cfg.Password2,
	}

	macs := a.buzzerHub.GetClients()
	sent := 0
	for _, mac := range macs {
		msg, err := protocol.NewMessage(protocol.ActionWifiConfig, payload)
		if err != nil {
			server.LogError(game.LogComponentApp, "Failed to create WIFI_CONFIG message for %s: %v", mac, err)
			continue
		}
		msgID := server.GenerateMsgID()
		msg.MsgID = msgID

		if err := a.buzzerHub.SendToClient(mac, msg); err != nil {
			server.LogWarn(game.LogComponentApp, "Failed to send WIFI_CONFIG to %s: %v", mac, err)
			continue
		}
		a.ackManager.Register(mac, msgID, protocol.ActionWifiConfig)
		a.engine.UpdateBumper(mac, map[string]interface{}{"ACK_PENDING": true})
		sent++
	}
	if sent > 0 {
		a.broadcastUpdate()
	}
	server.LogInfo(game.LogComponentApp, "Sent WIFI_CONFIG to %d connected buzzer(s)", sent)
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

	// Broadcast via WebSocket to admin clients only (TV/VPlayer do not need questions list)
	a.wsHub.BroadcastToTypes(msg, server.ClientTypeAdmin)
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
	adminCount, tvCount, vplayerCount := a.wsHub.GetClientCounts()
	clientsPayload := protocol.ClientsPayload{
		AdminCount:   adminCount,
		TVCount:      tvCount,
		VPlayerCount: vplayerCount,
		BuzzerWS:     a.buzzerHub.BuzzerCount(),
	}
	cData, _ := json.Marshal(clientsPayload)
	clientsMsg, _ := protocol.NewMessage(protocol.ActionClients, nil)
	clientsMsg.Msg = cData
	a.wsHub.SendToClient(clientID, clientsMsg)

	// Send CONFIG_UPDATE with neon effect settings
	cfg := config.Get()
	neonPayload := protocol.ConfigUpdatePayload{
		NeonEffect: protocol.NeonEffectPayload{
			Enabled:        cfg.NeonEffect.Enabled,
			Mode:           cfg.NeonEffect.Mode,
			ArcWidth:       cfg.NeonEffect.ArcWidth,
			IntensityGap:   cfg.NeonEffect.IntensityGap,
			RotationSpeed:  cfg.NeonEffect.RotationSpeed,
			BarOffset:      cfg.NeonEffect.BarOffset,
			BarThickness:   cfg.NeonEffect.BarThickness,
			ArcBlur:        cfg.NeonEffect.ArcBlur,
			GlowPulseSpeed: cfg.NeonEffect.GlowPulseSpeed,
			GlowPulseMin:   cfg.NeonEffect.GlowPulseMin,
			GlowPulseMax:   cfg.NeonEffect.GlowPulseMax,
		},
		DefaultQuestionImageIsCustom: a.httpServer.HasCustomDefaultQuestionImage(),
		NewGameBackgrounds:           a.engine.GetNewGameBackgrounds(),
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
	log.Println("â•”â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•—")
	log.Println("â•‘                    WEB INTERFACE URLs                      â•‘")
	log.Println("â• â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•£")

	var primaryURL string

	// Get all local IPs
	interfaces, err := getNetworkInterfaces()
	if err == nil {
		for _, iface := range interfaces {
			url := fmt.Sprintf("http://%s:%d", iface.IP, httpPort)
			if httpPort == 80 {
				url = fmt.Sprintf("http://%s", iface.IP)
			}
			log.Printf("â•‘  %-56s  â•‘", fmt.Sprintf("%s (%s)", url, iface.Name))

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
	log.Printf("â•‘  %-56s  â•‘", localhostURL+" (localhost)")

	log.Println("â•šâ•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•")
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
			"TYPE":          "SPEEDY",
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
			"TYPE":          "SPEEDY",
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
			"TYPE":          "SPEEDY",
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
			"TYPE":          "SPEEDY",
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
			"ID":                   "demo1",
			"QUESTION":             "Quelle est la capitale de l'Australie?",
			"ANSWER":               "Canberra",
			"POINTS":               "10",
			"TIME":                 "20",
			"TYPE":                 "QCM",
			"CATEGORY":             "GEOGRAPHY",
			"POINTS_TARGET":        "TEAM",
			"QCM_HINTS_ENABLED":    true,
			"QCM_HINT_THRESHOLD_1": 0.25,
			"QCM_HINT_THRESHOLD_2": 0.125,
			"QCM_PENALTY_1":        0.67,
			"QCM_PENALTY_2":        0.33,
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
			"TYPE":          "SPEEDY",
			"CATEGORY":      "ENTERTAINMENT",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         1,
		},
		// HISTORY - QCM with hints
		{
			"ID":                "demo3",
			"QUESTION":          "En quelle annee a debute la Premiere Guerre mondiale?",
			"ANSWER":            "1914",
			"POINTS":            "15",
			"TIME":              "25",
			"TYPE":              "QCM",
			"CATEGORY":          "HISTORY",
			"POINTS_TARGET":     "TEAM",
			"QCM_HINTS_ENABLED": true,
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
			"TYPE":          "SPEEDY",
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
			"TYPE":          "SPEEDY",
			"CATEGORY":      "ARTS",
			"POINTS_TARGET": "PLAYER",
			"ORDER":         5,
		},
		// FOOD - QCM with hints + images
		{
			"ID":                "demo7",
			"QUESTION":          "De quel pays vient la pizza?",
			"ANSWER":            "Italie",
			"POINTS":            "10",
			"TIME":              "20",
			"TYPE":              "QCM",
			"CATEGORY":          "FOOD",
			"POINTS_TARGET":     "TEAM",
			"QCM_HINTS_ENABLED": true,
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
			"TYPE":          "SPEEDY",
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
		questionID string
		assetName  string
		destName   string
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

	// Enrich each demo event with resolved category metadata (v5.7.9)
	for i := range events {
		catName, catImageURL, catColor := a.httpServer.ResolveCategoryMeta(events[i].QuestionCategory)
		events[i].CategoryDisplayName = catName
		events[i].CategoryImageURL = catImageURL
		events[i].CategoryColor = catColor
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
		log.Println("[mDNS] âš  Bonjour is NOT installed")
		log.Println("[mDNS]   â†’ mDNS hostname resolution (buzzcontrol.local) will NOT work")
		log.Println("[mDNS]   â†’ BuzzClick service discovery will still work")
		log.Println("[mDNS]   â†’ To enable hostname resolution, install Bonjour:")
		log.Println("[mDNS]     https://support.apple.com/kb/DL999")
		return
	}

	// Check if service is running
	outputStr := string(output)
	if strings.Contains(outputStr, "RUNNING") {
		log.Println("[mDNS] âœ“ Bonjour service is running")
		log.Println("[mDNS]   â†’ mDNS hostname resolution (buzzcontrol.local) should work")
	} else if strings.Contains(outputStr, "STOPPED") {
		log.Println("[mDNS] âš  Bonjour service is installed but STOPPED")
		log.Println("[mDNS]   â†’ Start the service: sc start \"Bonjour Service\"")
	} else {
		log.Println("[mDNS] âš  Bonjour service status unknown")
		log.Printf("[mDNS]   â†’ Output: %s", strings.TrimSpace(outputStr))
	}
}
