package protocol

import (
	"encoding/json"
	"time"

	"buzzcontrol/internal/game"
)

// Actions constants - matching ESP32 protocol
const (
	ActionHello       = "HELLO"
	ActionButton      = "BUTTON"
	ActionPong        = "PONG"
	ActionPing        = "PING"
	ActionStart       = "START"
	ActionStop        = "STOP"
	ActionPause       = "PAUSE"
	ActionContinue    = "CONTINUE"
	ActionUpdate      = "UPDATE"
	ActionUpdateTimer = "UPDATE_TIMER"
	ActionReset           = "RESET"
	ActionNewGame         = "NEW_GAME"          // Web → Server: reset game (scores, history, statuses) and enter NEW_GAME phase
	ActionUpdateQuizMeta  = "UPDATE_QUIZ_META"  // Web → Server: set quiz name/theme/notes metadata
	ActionReady       = "READY"
	ActionReveal      = "REVEAL"
	ActionQuestions   = "QUESTIONS"
	ActionPoints      = "POINTS"
	ActionRemote      = "REMOTE"
	ActionFull        = "FULL"
	ActionRAZ         = "RAZ"
	ActionReboot      = "REBOOT"
	ActionFSInfo       = "FSINFO"
	ActionDelete           = "DELETE"
	ActionDeleteBumper     = "DELETE_BUMPER"
	ActionBumperPoints     = "BUMPER_POINTS"
	ActionTeamPoints       = "TEAM_POINTS"
	ActionClients          = "CLIENTS"
	ActionSetClientType    = "SET_CLIENT_TYPE"
	ActionReorderQuestions  = "REORDER_QUESTIONS"
	ActionForceReady        = "FORCE_READY"
	ActionBackgroundChange  = "BACKGROUND_CHANGE"
	ActionFlipMemoryCard    = "FLIP_MEMORY_CARD"
	ActionMemorySetTeams    = "MEMORY_SET_TEAMS"
	// MEMOTION actions (v5.0.0)
	ActionMotionSelect   = "MEMOTION_SELECT"    // Admin → Server: select a card from the grid (→ SELECTED subphase, no timer)
	ActionMotionFlip     = "MEMOTION_FLIP"      // Admin → Server: flip selected card to QUESTION face + start timer
	ActionMotionStopTimer = "MEMOTION_STOP_TIMER" // Admin → Server: stop per-card timer (subphase stays QUESTION)
	ActionMotionReveal   = "MEMOTION_REVEAL"    // Admin → Server: flip card to REVEAL face
	ActionMotionDone     = "MEMOTION_DONE"      // Admin → Server: mark card played + optional winner team
	ActionMotionSetTeams = "MEMOTION_SET_TEAMS" // Admin → Server: set participating teams
	ActionQCMHint           = "QCM_HINT"
	// Virtual player enrollment actions
	ActionShowQRCode           = "SHOW_QR_CODE"
	ActionHideQRCode           = "HIDE_QR_CODE"
	ActionSetVirtualPlayerLimit = "SET_VIRTUAL_PLAYER_LIMIT"
	ActionPlayerConnect        = "PLAYER_CONNECT"
	ActionPlayerConnected      = "PLAYER_CONNECTED"
	ActionPlayerRejected       = "PLAYER_REJECTED"
	ActionEnrollmentUpdate     = "ENROLLMENT_UPDATE"
	ActionPlayerAssigned = "PLAYER_ASSIGNED"
	ActionPlayerEvicted  = "PLAYER_EVICTED" // Server → Client (targeted, never broadcast): a VJoueur's bumper was removed (#120, v5.9.x)
	// Name recovery (#122, v5.9.x) — animateur-assisted reclaim of a bumper
	// whose owner lost their local ID (see engine.ReconnectOrCreateVirtualPlayer,
	// the reclaim-authorization block, and Bumper.RECLAIM_REQUESTED).
	ActionReleaseBumperName = "RELEASE_BUMPER_NAME" // Admin → Server: grant a one-time reclaim authorization for a bumper's name
	// VPlayer QCM actions
	ActionVPlayerQCMAnswer = "VPLAYER_QCM_ANSWER"
	// ARDOISE actions (v5.6.0)
	ActionArdoiseInput = "ARDOISE_INPUT" // VPlayer → Server: free-text answer update (throttled)
	// Log actions (via dedicated /ws/logs WebSocket)
	ActionLogHistory = "LOG_HISTORY"
	ActionLogEntry   = "LOG_ENTRY"
	// Config update action
	ActionConfigUpdate = "CONFIG_UPDATE"
	// OTA firmware update actions (added in v3.1.0)
	ActionOTAUpdate      = "OTA_UPDATE"      // Server → Buzzer: trigger OTA update
	ActionOTAProgress    = "OTA_PROGRESS"    // Buzzer → Server: OTA progress update
	ActionFirmwareVersion = "FIRMWARE_VERSION" // Server → Web: firmware version info
	// WiFi config sync (added in v3.1.3)
	ActionWifiConfig = "WIFI_CONFIG" // Server → Buzzer: sync WiFi credentials
	// Server-driven LED control (added in v3.4.0)
	// Replaces per-action QCM LED messages (QCM_COLOR/QCM_DIM/QCM_REVEAL/QCM_RESET).
	// The server sends LED_SET at each relevant game state change; the firmware applies it directly.
	ActionLEDSet = "LED_SET" // Server → Buzzer (per-buzzer or broadcast): set LED color/intensity/effect
	// ACK protocol (added in v3.8.0)
	// Firmware sends ACK in response to priority messages (LED_SET, OTA_UPDATE, WIFI_CONFIG).
	// Server uses MSG_ID field (omitempty) on outgoing messages to enable ACK tracking.
	ActionACK = "ACK" // Buzzer → Server: acknowledge receipt of a priority message
	// Application-level heartbeat (added in v5.9.x, #118)
	// The WebSocket protocol's own ping/pong frames keep the connection alive
	// but are invisible to browser JavaScript (no onmessage-level event) — a
	// client can be looking at a zombie socket with no way to notice. HEARTBEAT
	// rides the same per-client ticker in writePump as the protocol ping, as a
	// visible TextMessage the client can watch for. Purely additive: no
	// response expected, an older client that doesn't recognize it keeps its
	// current behavior (unknown-action branch, ignored).
	ActionHeartbeat = "HEARTBEAT" // Server → Client (web: admin/TV/player): periodic liveness signal
	// AI generation job progress (v6.1.0, #137 — contract ai-multi-provider.md §10-§11)
	ActionAIGenerationProgress = "AI_GENERATION_PROGRESS" // Server → Client (/ws/admin only): job state after each batch
	ActionCancelAIGeneration   = "CANCEL_AI_GENERATION"   // Admin → Server: cancel the running job (effect between batches)
)

// FSInfo represents file storage information
type FSInfo struct {
	Used   int     `json:"USED"`
	Free   int     `json:"FREE"`
	Total  int     `json:"TOTAL"`
	PUsed  float64 `json:"P_USED"`
}

// Message represents the protocol message structure
// Compatible with BuzzClick v1 (ESP32)
type Message struct {
	Seq       int             `json:"seq,omitempty"`
	Action    string          `json:"ACTION"`
	ID        string          `json:"ID,omitempty"`
	Version   string          `json:"VERSION,omitempty"`
	Msg       json.RawMessage `json:"MSG,omitempty"`
	FSInfo    *FSInfo         `json:"FSINFO,omitempty"`
	TimeEvent int64           `json:"TIME_EVENT,omitempty"`
	// MsgID is a 12-char hex identifier added to priority messages (LED_SET, OTA_UPDATE, WIFI_CONFIG).
	// omitempty ensures backward compat: firmwares < v3.8.0 that don't support ACK ignore this field.
	MsgID string `json:"MSG_ID,omitempty"`
}

// IncomingMessage from TCP/WebSocket clients
type IncomingMessage struct {
	Source string // "TCP", "WebSocket"
	Data   *Message
	ClientID string
	// ClientType carries the sending WebSocketClient's type ("admin", "tv",
	// "vplayer", "buzzer" — mirrors server.ClientType's string values,
	// duplicated here as a plain string rather than that type itself: package
	// server already imports package protocol, so importing server.ClientType
	// back into this struct would create an import cycle) — populated by
	// internal/server/websocket.go's readPump directly from the connection's
	// own WebSocketClient.Type field at message-read time.
	//
	// #154 (sec): deliberately NOT looked up from WebSocketHub by ClientID at
	// dispatch time — h.clients is only populated once WebSocketHub.Run()
	// processes the register channel, which the documented race at
	// HandleConnectionWithType's `h.register <- client` call site (see that
	// NOTE) does not guarantee has happened by the time this same
	// connection's first message (e.g. HELLO) reaches readPump. Reading
	// c.Type directly here has no such window: it is set once at connection
	// and only ever mutated under WebSocketHub.mu by SetClientType, on the
	// SAME struct this read comes from.
	//
	// Empty ("") for TCP-sourced messages and for any IncomingMessage a test
	// constructs directly without setting it — cmd/server's handleWebMessage
	// allow-list (contracts/websocket-actions.md) treats an empty/unrecognized
	// ClientType as matching no entry, i.e. default-deny, not default-admin.
	ClientType string
	Timestamp  time.Time
}

// ButtonPayload for BUTTON action
type ButtonPayload struct {
	Button string `json:"button"`
}

// HelloPayload for HELLO action from buzzer
type HelloPayload struct {
	IP              string `json:"IP,omitempty"`
	Version         string `json:"VERSION,omitempty"`
	Name            string `json:"NAME,omitempty"`
	Team            string `json:"TEAM,omitempty"`
	FirmwareVersion string `json:"firmware_version,omitempty"` // BuzzClick firmware version (v3.1.0+, optional for backward compat)
}

// StartPayload for START action
type StartPayload struct {
	Delay int `json:"DELAY"`
}

// ReadyPayload for READY action
type ReadyPayload struct {
	Question string `json:"QUESTION"`
}

// PointsPayload for POINTS action
type PointsPayload struct {
	BumperID string `json:"bumperId"`
	Points   int    `json:"points"`
}

// RemotePayload for REMOTE action
type RemotePayload struct {
	Remote string `json:"REMOTE"`
}

// DeletePayload for DELETE action
type DeletePayload struct {
	ID string `json:"ID"`
}

// ReleaseBumperNamePayload for RELEASE_BUMPER_NAME action (#122 B3).
// Grants a one-time, time-bounded authorization for the next nameless
// PLAYER_CONNECT matching this bumper's name to reattach to it (score, team,
// and history preserved under a new bumper ID) instead of being rejected
// with NAME_TAKEN_OFFLINE.
type ReleaseBumperNamePayload struct {
	ID string `json:"ID"` // Bumper ID (the current name holder)
}

// BumperPointsPayload for BUMPER_POINTS action
type BumperPointsPayload struct {
	ID     string `json:"ID"`
	Points int    `json:"POINTS"`
}

// TeamPointsPayload for TEAM_POINTS action
type TeamPointsPayload struct {
	Team   string `json:"TEAM"`
	Points int    `json:"POINTS"`
}

// ClientsPayload for CLIENTS action (client count broadcast)
type ClientsPayload struct {
	AdminCount   int `json:"ADMIN_COUNT"`
	TVCount      int `json:"TV_COUNT"`
	VPlayerCount int `json:"VPLAYER_COUNT"`
	BuzzerWS     int `json:"BUZZER_WS_COUNT"`
}

// SetClientTypePayload for SET_CLIENT_TYPE action
type SetClientTypePayload struct {
	Type string `json:"TYPE"` // "admin", "tv", or "vplayer"
}

// ReorderQuestionsPayload for REORDER_QUESTIONS action
type ReorderQuestionsPayload struct {
	Order []string `json:"ORDER"` // Array of question IDs in new order
}

// BackgroundChangePayload for BACKGROUND_CHANGE action
type BackgroundChangePayload struct {
	Index int `json:"INDEX"` // Current background index (0-based)
}

// QuizMetaPayload for UPDATE_QUIZ_META action (v4.0.0; extended v6.0.0 #8;
// extended again v6.1.0 #137 Batch 2b — ⚠️ BREAKING: Population/Difficulty
// (string) replaced by Populations/Difficulties ([]string), Objectives added).
//
// Populations/Difficulties/Language/Objectives are pointers so the handler
// (cmd/server/main.go) can distinguish "field absent from the payload" (nil
// → leave the existing GameState value unchanged, for backward compatibility
// with clients that only send a subset of the form) from "field present with
// an empty value" (non-nil "" or [] → explicitly clear it). Contract
// ai-generation.md §7 — without this, a client saving only part of the form
// would silently wipe the others.
//
// Rétrocompatibilité rompue en v6.1.0 : POPULATION/DIFFICULTY (singuliers) ne
// sont plus reconnus — un client v6.0.0 qui les envoie voit ces deux clés
// ignorées par le décodage JSON (clés inconnues), les valeurs courantes sont
// préservées, aucun repli n'est implémenté (contract ai-generation.md §3ter).
// HiddenFields (v6.1.0, #137 Batch 2b T1.8, additive — contract game-state.md
// §"QUIZ_HIDDEN_FIELDS") carries the TV display preference: same
// absent/present-empty pointer semantics as the other fields, but routed to
// the dedicated Engine.SetQuizDisplay setter, not SetQuizMeta.
type QuizMetaPayload struct {
	Name         string    `json:"NAME"`
	Theme        string    `json:"THEME"`
	Notes        string    `json:"NOTES"`
	Populations  *[]string `json:"POPULATIONS,omitempty"`
	Difficulties *[]string `json:"DIFFICULTIES,omitempty"`
	Language     *string   `json:"LANGUAGE,omitempty"`
	Objectives   *string   `json:"OBJECTIVES,omitempty"`
	HiddenFields *[]string `json:"HIDDEN_FIELDS,omitempty"`
}

// AIGenerationProgressPayload for AI_GENERATION_PROGRESS action (v6.1.0, #137;
// ErrorMessage added v6.1.1, #142-adjacent verbosity fix).
// Server → Client, /ws/admin only, emitted after every batch of an AI
// generation job (contract ai-multi-provider.md §10). CreatedCount/SkippedCount
// are cumulative over the whole job, not per-batch.
type AIGenerationProgressPayload struct {
	JobID        string `json:"JOB_ID"`
	State        string `json:"STATE"` // "RUNNING" | "DONE" | "FAILED" | "CANCELLED"
	BatchesDone  int    `json:"BATCHES_DONE"`
	BatchesTotal int    `json:"BATCHES_TOTAL"`
	CreatedCount int    `json:"CREATED_COUNT"`
	SkippedCount int    `json:"SKIPPED_COUNT"`
	ErrorCode    string `json:"ERROR_CODE"` // stable code (ai-generation.md §3) + "provider_quota"; "" when not applicable
	// ErrorMessage is a human-readable detail for the admin — the sanitized
	// provider/local error text (contract ai-generation.md §8 S2, amended):
	// never the raw, unfiltered upstream body, but no longer a fixed generic
	// string either. Redacted of anything matching a known API key shape,
	// truncated to a bounded length (server.sanitizeUpstreamMessage). Only
	// meaningful when State is FAILED; "" otherwise. /ws/admin only, same as
	// the whole action — never reaches TV/VPlayer/buzzer.
	ErrorMessage string `json:"ERROR_MESSAGE,omitempty"`
	Provider     string `json:"PROVIDER"` // "anthropic" | "groq"
}

// CancelAIGenerationPayload for CANCEL_AI_GENERATION action (v6.1.0, #137).
// Client → Server. Cancellation takes effect between batches (contract §11).
type CancelAIGenerationPayload struct {
	JobID string `json:"JOB_ID"`
}

// FlipMemoryCardPayload for FLIP_MEMORY_CARD action
type FlipMemoryCardPayload struct {
	CardID string `json:"CARD_ID"` // Card ID to flip (e.g., "1-1", "2-2")
}

// MemorySetTeamsPayload for MEMORY_SET_TEAMS action
type MemorySetTeamsPayload struct {
	Teams []string `json:"TEAMS"` // List of team names participating
}

// MotionSelectPayload for MEMOTION_SELECT action (Admin → Server)
type MotionSelectPayload struct {
	CardID string `json:"CARD_ID"` // ID of the card to select (e.g. "mc-1")
}

// MotionDonePayload for MEMOTION_DONE action (Admin → Server)
type MotionDonePayload struct {
	CardID     string `json:"CARD_ID"`              // ID of the card being closed
	WinnerTeam string `json:"WINNER_TEAM,omitempty"` // Team name if a winner, "" if none
}

// MotionSetTeamsPayload is an alias for MemorySetTeamsPayload — same {TEAMS: [...]} structure
type MotionSetTeamsPayload = MemorySetTeamsPayload

// QCMHintPayload for QCM_HINT action
type QCMHintPayload struct {
	Color     string `json:"COLOR"`     // Invalidated color (RED, GREEN, YELLOW, BLUE)
	Remaining int    `json:"REMAINING"` // Number of remaining valid answers
}

// SetVirtualPlayerLimitPayload for SET_VIRTUAL_PLAYER_LIMIT action
type SetVirtualPlayerLimitPayload struct {
	Limit int `json:"LIMIT"` // Maximum number of virtual players
}

// PlayerConnectPayload for PLAYER_CONNECT action (virtual player enrollment)
type PlayerConnectPayload struct {
	Name string `json:"NAME"`         // Player name
	ID   string `json:"ID,omitempty"` // Bumper ID from a prior PLAYER_CONNECTED (fix R1, #109) —
	// omitted on first enrollment. When present and it resolves to an existing
	// VJoueur bumper, the reconnection is unambiguous and never rejected/merged
	// by name. Omitempty keeps old clients (that never send it) working as a
	// name-based enrollment attempt (rejected with NAME_TAKEN if the name is
	// already in use — see PlayerRejectedPayload).
}

// PlayerConnectedPayload for PLAYER_CONNECTED action (enrollment accepted)
type PlayerConnectedPayload struct {
	ID   string `json:"ID"`   // Bumper ID
	Name string `json:"NAME"` // Player name
	Team string `json:"TEAM"` // Team name (if assigned)
}

// PlayerRejectedPayload for PLAYER_REJECTED action (enrollment rejected)
type PlayerRejectedPayload struct {
	Reason string `json:"REASON"` // Rejection reason: ENROLLMENT_CLOSED, LIMIT_REACHED,
	// INVALID_NAME, or NAME_TAKEN (fix R1, #109 — no ID provided/resolvable and
	// the name is already used by another VJoueur, connected or not; never
	// merged/replaced, the client must pick a different name). Also carries
	// PLAYER_REMOVED / GAME_RESET / SEAT_RELEASED (#123 B3, #134) when a
	// PLAYER_CONNECT arrives with a now-stale ID the eviction registry still
	// remembers a reason for.
}

// QRCodePayload for SHOW_QR_CODE/HIDE_QR_CODE actions
type QRCodePayload struct {
	URL  string `json:"URL"`  // URL to encode in QR code
	Show bool   `json:"SHOW"` // Whether to show or hide
}

// EnrollmentUpdatePayload for ENROLLMENT_UPDATE action (broadcast virtual player count)
type EnrollmentUpdatePayload struct {
	VirtualPlayerCount int  `json:"VIRTUAL_PLAYER_COUNT"` // Current count
	VirtualPlayerLimit int  `json:"VIRTUAL_PLAYER_LIMIT"` // Maximum allowed
	EnrollmentActive   bool `json:"ENROLLMENT_ACTIVE"`    // Whether enrollment is open
}

// PlayerAssignedPayload for PLAYER_ASSIGNED action (player assigned to team)
type PlayerAssignedPayload struct {
	ID          string `json:"ID"`           // Bumper ID
	Team        string `json:"TEAM"`         // Team name
	AnswerColor string `json:"ANSWER_COLOR"` // Assigned answer color (RED/GREEN/YELLOW/BLUE)
}

// PlayerEvictedPayload for PLAYER_EVICTED action (#120, v5.9.x).
// Targeted to the single client whose bumper was just removed — never broadcast.
// Replaces the client-side "bumper missing from roster" deduction, which could not
// distinguish "evicted" from "not yet received" and caused silent enrollment-page
// redirects. Absence of a bumper in a roster update is never, by itself, grounds
// for eviction — only this message is authoritative (contracts/websocket-actions.md).
type PlayerEvictedPayload struct {
	Reason string `json:"REASON"` // PLAYER_REMOVED (admin deleted the bumper), GAME_RESET
	// (VJoueur roster purged by InitGame on NEW_GAME), or SEAT_RELEASED
	// (#134 — admin released a still-connected bumper's seat; unlike
	// PLAYER_REMOVED the bumper survives, re-keyed, score/team preserved).
	// Free-form string, no server-side enum/whitelist (verified #134 T2.3):
	// contracts/seat-release.md §4 is the single source of truth for valid
	// values, not a Go-level validation.
}

// VPlayerQCMAnswerPayload for VPLAYER_QCM_ANSWER action (VPlayer buzzes with color)
type VPlayerQCMAnswerPayload struct {
	ID          string `json:"ID"`           // Bumper ID (VPlayer MAC)
	AnswerColor string `json:"ANSWER_COLOR"` // Color chosen (RED, GREEN, YELLOW, BLUE)
}

// ArdoiseInputPayload for ARDOISE_INPUT action (VPlayer → Server)
// Sent throttled (~200ms) during STARTED phase with TYPE=ARDOISE question.
// Team identification is resolved server-side via ID → bumper → bumper.Team (3-pass pattern,
// identical to VPlayerQCMAnswerPayload).
type ArdoiseInputPayload struct {
	ID   string `json:"ID,omitempty"` // Bumper ID (VPlayer MAC) — explicit, same as VPLAYER_QCM_ANSWER
	Text string `json:"TEXT"`         // Current answer text (full content, not delta)
}

// LogHistoryPayload for LOG_HISTORY action (send log history to client)
type LogHistoryPayload struct {
	Entries []LogEntryPayload `json:"entries"` // Array of log entries
}

// LogEntryPayload for LOG_ENTRY action (single log entry broadcast)
type LogEntryPayload struct {
	Timestamp int64  `json:"timestamp"` // Unix milliseconds
	Level     string `json:"level"`     // DEBUG, INFO, WARN, ERROR
	Component string `json:"component"` // Engine, HTTP, WebSocket, TCP, UDP, App
	Message   string `json:"message"`
}

// ConfigUpdatePayload for CONFIG_UPDATE action (broadcast config changes)
type ConfigUpdatePayload struct {
	NeonEffect                   NeonEffectPayload `json:"neon_effect"`
	DefaultQuestionImageIsCustom bool              `json:"default_question_image_is_custom"`  // true if custom image uploaded, false = embedded fallback
	NewGameBackgrounds           []game.Background `json:"new_game_backgrounds"`              // NEW_GAME screen backgrounds (v4.0.4)
}

// NeonEffectPayload represents neon effect configuration
type NeonEffectPayload struct {
	Enabled        bool    `json:"enabled"`
	Mode           string  `json:"mode"`             // "halo" or "bar"
	ArcWidth       int     `json:"arc_width"`        // 30-180 degrees (halo mode)
	IntensityGap   int     `json:"intensity_gap"`    // 0-100%
	RotationSpeed  float64 `json:"rotation_speed"`   // 1-10 seconds
	BarOffset      int     `json:"bar_offset"`       // 10-100 pixels (bar mode)
	BarThickness   int     `json:"bar_thickness"`    // 2-20 pixels (bar mode)
	ArcBlur        int     `json:"arc_blur"`         // 0-200% of bar thickness
	GlowPulseSpeed float64 `json:"glow_pulse_speed"` // 0.5-5 seconds
	GlowPulseMin   int     `json:"glow_pulse_min"`   // 0-100%
	GlowPulseMax   int     `json:"glow_pulse_max"`   // 0-100%
}

// OTAUpdatePayload for OTA_UPDATE action (server → buzzer: trigger OTA update).
// URL is included for backward compatibility with firmware < 3.1.2 which reads
// the URL from the message. Newer firmware (>= 3.1.2) ignores URL and constructs
// it from its stored server IP. The endpoint is always /api/firmware/buzzclick/latest.bin.
type OTAUpdatePayload struct {
	Version string `json:"VERSION"` // Target firmware version
	Size    int64  `json:"SIZE"`    // Firmware file size in bytes
	URL     string `json:"URL"`     // Firmware download URL (backward compat with firmware < 3.1.2)
}

// OTAProgressPayload for OTA_PROGRESS action (buzzer → server: progress update)
type OTAProgressPayload struct {
	MAC     string `json:"MAC"`               // Buzzer MAC address
	Status  string `json:"STATUS"`             // "downloading", "flashing", "done", "error"
	Percent int    `json:"PERCENT"`            // Progress percentage (0-100)
	Error   string `json:"ERROR,omitempty"`    // Error message if status == "error"
}

// FirmwareVersionPayload for FIRMWARE_VERSION action (server → web: firmware info)
// JSON keys use UPPERCASE to match the frontend React convention (targetVersion?.EXISTS, etc.).
type FirmwareVersionPayload struct {
	Version         string `json:"VERSION"`                    // Firmware version string (e.g. "3.1.0")
	Filename        string `json:"FILENAME"`                   // Firmware filename (e.g. "buzzclick-v3.1.0.bin")
	Size            int64  `json:"SIZE"`                       // Firmware file size in bytes
	Exists          bool   `json:"EXISTS"`                     // Whether firmware file exists on server
	IsMerged        bool   `json:"IS_MERGED"`                  // Whether stored firmware is a merged binary (USB flash capable)
	EmbeddedVersion string `json:"EMBEDDED_VERSION,omitempty"` // Version embedded in server binary (v3.1.1+)
}

// LED effect constants for LEDSetPayload.Effect.
const (
	LEDEffectSolid   = "SOLID"   // Steady color at given intensity
	LEDEffectBlink   = "BLINK"   // Blinks 100%↔25% at 400 ms
	LEDEffectDim     = "DIM"     // Steady dimmed color
	LEDEffectComet   = "COMET"   // Rotating band animation: COLOR=background, COMET_COLOR=band
	LEDEffectSpinner = "SPINNER" // 1 gold pixel rotating around the ring (firmware-side, ~2 s point-award)
)

// LEDSetPayload for LED_SET action (server → buzzer: set LED color/intensity/effect).
// Effect values: "SOLID" (steady), "BLINK" (100%↔25% at 400ms), "DIM" (steady dimmed), "SPINNER" (rotating pixel).
// For COMET: COLOR=background team color, COMET_COLOR=band color (gold or white for contrast).
type LEDSetPayload struct {
	Color      [3]int  `json:"COLOR"`                 // RGB background [0-255]
	Intensity  int     `json:"INTENSITY"`              // 0-255
	Effect     string  `json:"EFFECT"`                 // "SOLID", "BLINK", "DIM", "COMET", "SPINNER"
	CometColor *[3]int `json:"COMET_COLOR,omitempty"`  // COMET band color (nil = firmware default gold)
}

// AckPayload for ACK action (buzzer → server: acknowledge receipt of a priority message).
// The buzzer sends this immediately upon receiving a message with a MSG_ID field,
// before applying the action (LED_SET, OTA_UPDATE, WIFI_CONFIG).
type AckPayload struct {
	AckAction string `json:"ack_action"` // Action being acknowledged (e.g. "LED_SET")
	AckID     string `json:"ack_id"`     // Value of the MSG_ID being acknowledged
}

// HeartbeatPayload for HEARTBEAT action (#118, v5.9.x; DeadLinkTimeoutMs
// added in #130). No response expected — the client only watches for
// arrival. Both fields carry server-side values the client must not
// hardcode (contracts/liveness-timing.md §1, "le serveur est la source de
// vérité unique") so a future change to either constant doesn't silently
// desync the client's dead-connection threshold.
type HeartbeatPayload struct {
	IntervalMs int64 `json:"INTERVAL_MS"` // Real cadence of the server-side ticker, in milliseconds

	// DeadLinkTimeoutMs (#130) is the absolute silence threshold, in
	// milliseconds, beyond which the client should consider the link dead,
	// close its socket, and reconnect — not a multiplier: the server
	// controls this value directly rather than leaving a factor to the
	// client (contracts/liveness-timing.md §2). No omitempty — project rule
	// (CLAUDE.md): GameState/protocol fields are always serialized, even at
	// a zero value, so the client never has to guess whether an absent field
	// means "zero" or "not sent yet".
	DeadLinkTimeoutMs int64 `json:"DEAD_LINK_TIMEOUT_MS"`
}

// WifiConfigPayload for WIFI_CONFIG action (server → buzzer: sync WiFi credentials)
type WifiConfigPayload struct {
	SSID       string `json:"SSID"`
	Pass       string `json:"PASS"`
	ServerIP   string `json:"SERVER_IP"`
	ServerPort int    `json:"SERVER_PORT"`
	SSID2      string `json:"SSID2,omitempty"`
	Pass2      string `json:"PASS2,omitempty"`
}

// NewMessage creates a new outgoing message
func NewMessage(action string, msg interface{}) (*Message, error) {
	var rawMsg json.RawMessage
	var err error

	if msg != nil {
		rawMsg, err = json.Marshal(msg)
		if err != nil {
			return nil, err
		}
	} else {
		rawMsg = json.RawMessage("{}")
	}

	return &Message{
		Action:    action,
		Msg:       rawMsg,
		TimeEvent: time.Now().UnixMicro(),
	}, nil
}

// Serialize converts message to JSON bytes with null terminator (for TCP)
func (m *Message) Serialize() ([]byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	// Add newline and null terminator for BuzzClick v1 compatibility
	return append(data, '\n', 0), nil
}

// SerializeForWebSocket converts message to JSON bytes (no null terminator)
func (m *Message) SerializeForWebSocket() ([]byte, error) {
	return json.Marshal(m)
}

// SerializeForAdmin returns the full JSON payload, identical to SerializeForWebSocket.
// Admin clients receive all fields including FIRMWARE_VERSION, IS_OUTDATED, OTA_STATUS
// and ACK_PENDING so the admin UI can display OTA badges and ACK spinners.
func (m *Message) SerializeForAdmin() ([]byte, error) {
	return json.Marshal(m)
}

// AdminOnlyBumperFields lists the per-bumper fields only Admin needs
// (firmware/OTA/ACK metadata) — stripped from every bumper by
// SerializeForWebClient, from the single reduced bumper entry by
// SerializeForVPlayer (#127 T2.1), and by the hot fan-out path in
// cmd/server/main.go (T2.3), all from this one shared, exported list so none
// of the three can silently drift apart from the others.
var AdminOnlyBumperFields = []string{
	"FIRMWARE_VERSION", "IS_OUTDATED", "OTA_STATUS", "OTA_PERCENT", "ACK_PENDING",
}

// AdminOnlyGameFields lists the fields of the "GAME" node only Admin needs —
// stripped from that node by SerializeForWebClient, by the reduced path of
// SerializeForVPlayer, and by the hot VPlayer fan-out path in
// cmd/server/main.go (buildVPlayerPayloads), all from this one shared,
// exported list so none of the three can silently drift apart from the
// others (contracts/ws-payload-serialization.md, contracts/game-state.md
// §"QUIZ_OBJECTIVES — champ à diffusion restreinte", v6.1.0 #137 Batch 2b).
//
// QUIZ_OBJECTIVES is a confidentiality rule, not a payload-size optimization:
// the game's global objective is an animation cue for the admin/AI generator
// and must never be readable from a TV screen or a VPlayer's dev tools, even
// though the rest of the QUIZ_* fields are shown on the TV NEW_GAME screen.
var AdminOnlyGameFields = []string{
	"QUIZ_OBJECTIVES",
}

// SerializeForWebClient returns the payload with admin-only fields stripped from
// bumpers and from the GAME node on UPDATE messages. TV and VPlayer clients do
// not need firmware/OTA/ACK metadata nor QUIZ_OBJECTIVES, so stripping them
// reduces the payload size (and, for QUIZ_OBJECTIVES, enforces confidentiality)
// for these high-frequency broadcasts.
//
// Fields stripped per bumper on UPDATE: FIRMWARE_VERSION, IS_OUTDATED, OTA_STATUS,
// OTA_PERCENT, ACK_PENDING. Fields stripped from GAME: see AdminOnlyGameFields.
// The top-level "config" key is also removed (server-side config is not needed
// by TV/VPlayer clients).
//
// The format produced by GetGameJSON() (GameData struct) uses lowercase map keys:
//   - "bumpers" → map[mac]*Bumper  (not "BUMPERS" / slice)
//   - "teams"   → map[name]*Team   (not "TEAMS"   / slice)
//   - "GAME"    → GameState node
//
// All other actions are serialised identically to SerializeForWebSocket.
func (m *Message) SerializeForWebClient() ([]byte, error) {
	if m.Action != ActionUpdate {
		return json.Marshal(m)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(m.Msg, &raw); err != nil {
		// Fallback to full serialisation — never silently drop updates
		return json.Marshal(m)
	}

	// "bumpers" is a map[mac]bumper in GetGameJSON() — lowercase key, map not slice
	if bumpers, ok := raw["bumpers"].(map[string]interface{}); ok {
		for _, b := range bumpers {
			if bumper, ok := b.(map[string]interface{}); ok {
				for _, field := range AdminOnlyBumperFields {
					delete(bumper, field)
				}
			}
		}
	}
	// "GAME" node: strip admin-only fields (QUIZ_OBJECTIVES, v6.1.0)
	if gameNode, ok := raw["GAME"].(map[string]interface{}); ok {
		for _, field := range AdminOnlyGameFields {
			delete(gameNode, field)
		}
	}
	// "config" is a server-side key not needed by TV/VPlayer clients
	delete(raw, "config")

	stripped, err := json.Marshal(raw)
	if err != nil {
		return json.Marshal(m)
	}
	out := *m
	out.Msg = stripped
	return json.Marshal(&out)
}

// vplayerReducedPhases are the GAME.PHASE values during which a VPlayer's own
// UPDATE payload is reduced to just its own bumper entry — contracts/vplayer-payload-filter.md §2.
var vplayerReducedPhases = map[string]bool{"PREPARE": true, "READY": true}

// SerializeForVPlayer returns the UPDATE payload for one specific, identified
// VPlayer client (playerID) — reference implementation of the reduction rule
// in contracts/vplayer-payload-filter.md §2 (#127 T2.1). Used directly by
// table-driven tests (T2.5); the hot fan-out path (T2.2/T2.3,
// internal/server/websocket.go + cmd/server/main.go) reimplements the same
// rule keeping GAME/teams as json.RawMessage instead of round-tripping
// through map[string]interface{} for every recipient — both must agree on
// content, this method is the correctness reference for that agreement.
//
// The payload is reduced if and only if all three conditions hold (contract
// §2 "règle d'application"); on the first condition that fails, or on any
// JSON error, this falls back to the complete filtered payload
// (SerializeForWebClient) — an update must never be silently dropped or
// narrower than that fallback:
//  1. m.Action == ActionUpdate;
//  2. MSG.GAME.PHASE is PREPARE or READY — read from the payload itself
//     (never passed in), so the decision always matches what is actually
//     being sent, not a caller's possibly-stale belief about the phase;
//  3. playerID != "" — an unidentified VPlayer (no PLAYER_CONNECT completed
//     yet, SetClientPlayerID never called for it) must always receive the
//     complete card: VPlayerPage.jsx recovers a lost identity by scanning
//     bumpers by NAME, which is impossible on a single-entry map.
//
// GAME and teams are left completely untouched — "teams" is deliberately
// NOT reduced even though only one bumper is kept (contract §2 rationale:
// PlayerDisplay.jsx's MEMORY/MEMOTION team bars read teams[name] without an
// !isVPlayer gate). Only "bumpers" is reduced to the single {playerID: ...}
// entry, with the same admin-only fields SerializeForWebClient strips.
func (m *Message) SerializeForVPlayer(playerID string) ([]byte, error) {
	if m.Action != ActionUpdate || playerID == "" {
		return m.SerializeForWebClient()
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(m.Msg, &raw); err != nil {
		return m.SerializeForWebClient()
	}

	gameNode, ok := raw["GAME"].(map[string]interface{})
	if !ok {
		return m.SerializeForWebClient()
	}
	// Strip admin-only GAME fields (QUIZ_OBJECTIVES, v6.1.0) — this reduced
	// path builds its own "raw" independently of SerializeForWebClient, so
	// the same list must be applied here too (contracts/ws-payload-serialization.md).
	// gameNode is a reference into raw (map values from json.Unmarshal into
	// map[string]interface{} are shared, not copied), so deleting from it
	// here is reflected in raw["GAME"] below without reassignment.
	for _, field := range AdminOnlyGameFields {
		delete(gameNode, field)
	}
	phase, _ := gameNode["PHASE"].(string)
	if !vplayerReducedPhases[phase] {
		return m.SerializeForWebClient()
	}

	bumpers, ok := raw["bumpers"].(map[string]interface{})
	if !ok {
		return m.SerializeForWebClient()
	}
	own, ok := bumpers[playerID]
	if !ok {
		// This player's bumper isn't in the current snapshot (e.g. evicted in
		// the same instant this broadcast was built) — fall back rather than
		// send a bumpers map that omits the recipient's own entry entirely.
		return m.SerializeForWebClient()
	}
	if bumper, ok := own.(map[string]interface{}); ok {
		for _, field := range AdminOnlyBumperFields {
			delete(bumper, field)
		}
	}
	raw["bumpers"] = map[string]interface{}{playerID: own}
	// "config" is a server-side key not needed by VPlayer clients (same as SerializeForWebClient)
	delete(raw, "config")

	stripped, err := json.Marshal(raw)
	if err != nil {
		return m.SerializeForWebClient()
	}
	out := *m
	out.Msg = stripped
	return json.Marshal(&out)
}

// buzzerBumperKeys lists the Bumper fields that physical buzzers actually need.
// Note: "ID" is intentionally absent — in GetGameJSON() the bumper MAC is the
// map key, not a field inside the bumper object (Bumper struct has no ID field).
// All other fields (firmware, OTA, QCM answer colour, etc.) are stripped to
// minimise payload size on UPDATE messages sent to buzzer clients.
var buzzerBumperKeys = []string{
	"NAME", "TEAM", "CONNECTED", "CONN_STATE", "IS_VIRTUAL", "TIME", "BUTTON", "STATUS", "SCORE",
}

// buzzerTeamKeys lists the Team fields that physical buzzers actually need.
var buzzerTeamKeys = []string{"NAME", "COLOR", "STATUS", "SCORE"}

// SerializeForBuzzer returns a minimal UPDATE payload for physical buzzer clients.
// Buzzers only need a small subset of the game state to drive their LED animations and
// local state machine, so stripping unused fields keeps the WebSocket frames small.
//
// On UPDATE: PHASE/TIME/CURRENT_TIME are extracted from the "GAME" node and forwarded
// at the top level; bumpers and teams are reduced to their respective key whitelists and
// emitted as maps (preserving the lowercase map-keyed structure of GetGameJSON()).
//
// The format produced by GetGameJSON() (GameData struct) uses lowercase map keys:
//   - "bumpers" → map[mac]*Bumper
//   - "teams"   → map[name]*Team
//   - "GAME"    → GameState node (contains PHASE, TIME, CURRENT_TIME, etc.)
//
// All other actions are serialised identically to SerializeForWebSocket.
func (m *Message) SerializeForBuzzer() ([]byte, error) {
	if m.Action != ActionUpdate {
		return json.Marshal(m)
	}

	var full map[string]interface{}
	if err := json.Unmarshal(m.Msg, &full); err != nil {
		return json.Marshal(m)
	}

	minimal := make(map[string]interface{}, 8)

	// PHASE / TIME / CURRENT_TIME live inside the "GAME" node in GetGameJSON()
	if gameNode, ok := full["GAME"].(map[string]interface{}); ok {
		for _, key := range []string{"PHASE", "TIME", "CURRENT_TIME"} {
			if v, ok := gameNode[key]; ok {
				minimal[key] = v
			}
		}
	}

	// Reduce bumpers to minimal fields.
	// "bumpers" is a map[mac]bumper — lowercase key, map not slice.
	if bumpers, ok := full["bumpers"].(map[string]interface{}); ok {
		minBumpers := make(map[string]interface{}, len(bumpers))
		for mac, b := range bumpers {
			if bumper, ok := b.(map[string]interface{}); ok {
				minBumper := make(map[string]interface{}, len(buzzerBumperKeys))
				for _, key := range buzzerBumperKeys {
					if v, ok := bumper[key]; ok {
						minBumper[key] = v
					}
				}
				minBumpers[mac] = minBumper
			}
		}
		minimal["bumpers"] = minBumpers
	}

	// Reduce teams to minimal fields.
	// "teams" is a map[name]team — lowercase key, map not slice.
	if teams, ok := full["teams"].(map[string]interface{}); ok {
		minTeams := make(map[string]interface{}, len(teams))
		for name, t := range teams {
			if team, ok := t.(map[string]interface{}); ok {
				minTeam := make(map[string]interface{}, len(buzzerTeamKeys))
				for _, key := range buzzerTeamKeys {
					if v, ok := team[key]; ok {
						minTeam[key] = v
					}
				}
				minTeams[name] = minTeam
			}
		}
		minimal["teams"] = minTeams
	}

	stripped, err := json.Marshal(minimal)
	if err != nil {
		return json.Marshal(m)
	}
	out := *m
	out.Msg = stripped
	return json.Marshal(&out)
}

// ParseButtonPayload extracts button info from message
func (m *Message) ParseButtonPayload() (*ButtonPayload, error) {
	var payload ButtonPayload
	if err := json.Unmarshal(m.Msg, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// ParseHelloPayload extracts hello info from message
func (m *Message) ParseHelloPayload() (*HelloPayload, error) {
	var payload HelloPayload
	if err := json.Unmarshal(m.Msg, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}
