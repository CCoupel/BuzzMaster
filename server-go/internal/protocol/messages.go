package protocol

import (
	"encoding/json"
	"time"

	"buzzcontrol/internal/game"
)

// Actions constants - matching ESP32 protocol
const (
	ActionHello            = "HELLO"
	ActionButton           = "BUTTON"
	ActionPong             = "PONG"
	ActionPing             = "PING"
	ActionStart            = "START"
	ActionStop             = "STOP"
	ActionPause            = "PAUSE"
	ActionContinue         = "CONTINUE"
	ActionUpdate           = "UPDATE"
	ActionUpdateTimer      = "UPDATE_TIMER"
	ActionReset            = "RESET"
	ActionNewGame          = "NEW_GAME"         // Web → Server: reset game (scores, history, statuses) and enter NEW_GAME phase
	ActionUpdateQuizMeta   = "UPDATE_QUIZ_META" // Web → Server: set quiz name/theme/notes metadata
	ActionReady            = "READY"
	ActionReveal           = "REVEAL"
	ActionQuestions        = "QUESTIONS"
	ActionPoints           = "POINTS"
	ActionRemote           = "REMOTE"
	ActionFull             = "FULL"
	ActionRAZ              = "RAZ"
	ActionReboot           = "REBOOT"
	ActionFSInfo           = "FSINFO"
	ActionDelete           = "DELETE"
	ActionDeleteBumper     = "DELETE_BUMPER"
	ActionBumperPoints     = "BUMPER_POINTS"
	ActionTeamPoints       = "TEAM_POINTS"
	ActionClients          = "CLIENTS"
	ActionSetClientType    = "SET_CLIENT_TYPE"
	ActionReorderQuestions = "REORDER_QUESTIONS"
	ActionForceReady       = "FORCE_READY"
	ActionBackgroundChange = "BACKGROUND_CHANGE"
	ActionFlipMemoryCard   = "FLIP_MEMORY_CARD"
	ActionMemorySetTeams   = "MEMORY_SET_TEAMS"
	// MEMOTION actions (v5.0.0)
	ActionMotionSelect    = "MEMOTION_SELECT"     // Admin → Server: select a card from the grid (→ SELECTED subphase, no timer)
	ActionMotionFlip      = "MEMOTION_FLIP"       // Admin → Server: flip selected card to QUESTION face + start timer
	ActionMotionStopTimer = "MEMOTION_STOP_TIMER" // Admin → Server: stop per-card timer (subphase stays QUESTION)
	ActionMotionReveal    = "MEMOTION_REVEAL"     // Admin → Server: flip card to REVEAL face
	ActionMotionDone      = "MEMOTION_DONE"       // Admin → Server: mark card played + optional winner team
	ActionMotionSetTeams  = "MEMOTION_SET_TEAMS"  // Admin → Server: set participating teams
	ActionQCMHint         = "QCM_HINT"
	// Virtual player enrollment actions
	ActionShowQRCode            = "SHOW_QR_CODE"
	ActionHideQRCode            = "HIDE_QR_CODE"
	ActionSetVirtualPlayerLimit = "SET_VIRTUAL_PLAYER_LIMIT"
	ActionPlayerConnect         = "PLAYER_CONNECT"
	ActionPlayerConnected       = "PLAYER_CONNECTED"
	ActionPlayerRejected        = "PLAYER_REJECTED"
	ActionEnrollmentUpdate      = "ENROLLMENT_UPDATE"
	ActionPlayerAssigned        = "PLAYER_ASSIGNED"
	ActionPlayerEvicted         = "PLAYER_EVICTED" // Server → Client (targeted, never broadcast): a VJoueur's bumper was removed (#120, v5.9.x)
	// Name recovery (#122, v5.9.x) — animateur-assisted reclaim of a bumper
	// whose owner lost their local ID (see engine.ReconnectOrCreateVirtualPlayer,
	// the reclaim-authorization block, and Bumper.RECLAIM_REQUESTED).
	ActionReleaseBumperName = "RELEASE_BUMPER_NAME" // Admin → Server: grant a one-time reclaim authorization for a bumper's name
	// VPlayer QCM actions
	ActionVPlayerQCMAnswer = "VPLAYER_QCM_ANSWER"
	// ARDOISE actions (v5.6.0)
	ActionArdoiseFinal            = "ARDOISE_FINAL"              // VPlayer → Server: final text at STOP/PAUSE (v11.2, #246)
	ActionArdoiseFinalAck         = "ARDOISE_FINAL_ACK"          // Server → VPlayer (targeted): final received (v11.2, #246)
	ActionTeamsUpdateRefused      = "TEAMS_UPDATE_REFUSED"       // Server → sender (targeted): UPDATE/FULL removing a team refused while TEAMS_LOCKED (v11.2, #249)
	ActionNewGameRefused          = "NEW_GAME_REFUSED"           // Server → sender (targeted): NEW_GAME refused (v11.2, #243)
	ActionTeamPointsBatch         = "TEAM_POINTS_BATCH"          // Client → Server (admin): grouped team award (v11.2, #244/#251)
	ActionTeamPointsBatchResult   = "TEAM_POINTS_BATCH_RESULT"   // Server → sender: outcome of the grouped award (v11.2)
	ActionBumperPointsBatch       = "BUMPER_POINTS_BATCH"        // Admin → Server: grouped player award (v11.2, #254)
	ActionBumperPointsBatchResult = "BUMPER_POINTS_BATCH_RESULT" // Server → sender: outcome of the grouped player award (v11.2, #254)
	ActionHelloAck                = "HELLO_ACK"                  // Server → admin/anim (targeted): this connection's own ID (v11.2, #253)
	ActionLaunchLock              = "LAUNCH_LOCK"                // Admin → Server: popup "Lancer la partie" open/close/renew (v11.2, #253)
	ActionLaunchLockRefused       = "LAUNCH_LOCK_REFUSED"        // Server → sender: opening refused (v11.2, #253)
	ActionActionRefused           = "ACTION_REFUSED"             // Server → sender: action suspended by LAUNCH_LOCK (v11.2, #253)
	ActionRevealRefused           = "REVEAL_REFUSED"             // Server → admin/anim (targeted): REVEAL refused by the ARDOISE guard (v11.2, #246)
	ActionClientTrace             = "CLIENT_TRACE"               // Client → Server: diagnostic trace (v11.2, #246)
	ActionArdoiseInput            = "ARDOISE_INPUT"              // VPlayer → Server: free-text answer update (throttled)
	// Log actions (via dedicated /ws/logs WebSocket)
	ActionLogHistory = "LOG_HISTORY"
	ActionLogEntry   = "LOG_ENTRY"
	// Config update action
	ActionConfigUpdate = "CONFIG_UPDATE"
	// OTA firmware update actions (added in v3.1.0)
	ActionOTAUpdate       = "OTA_UPDATE"       // Server → Buzzer: trigger OTA update
	ActionOTAProgress     = "OTA_PROGRESS"     // Buzzer → Server: OTA progress update
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
	// Interface animateur (v6.2.0, #155 tâche B5 — contracts/websocket-actions.md §"Animateur")
	ActionNextQuestion = "NEXT_QUESTION" // Server → Client (/ws/anim only): the next playable question, for enchaînement without /admin
	// Code review MAJEUR-1 follow-up (v6.2.0, #155/#156 — contracts/websocket-actions.md §"Animateur")
	ActionSetCreditPoints = "SET_CREDIT_POINTS" // Admin → Server: pointsInput was adjusted, push the new base credit amount
	ActionCreditPoints    = "CREDIT_POINTS"     // Server → Client (/ws/anim only): current base credit amount for the active question
	// Synchronized credit lock across animateur tablets (v6.2.x, #170 — contracts/websocket-actions.md §"Animateur")
	ActionAwardedTeams = "AWARDED_TEAMS" // Server → Client (/ws/anim only): teams already credited for the current question, all modes
	// Messagerie régie (v6.4.x, #167 — contracts/websocket-actions.md §"Messagerie régie"):
	// unidirectional régie → anim instruction channel, orthogonal to the game
	// engine (no engine.go change, never cleared by a phase transition).
	ActionRegieMessageSend  = "REGIE_MESSAGE_SEND"  // Admin → Server: arm or replace the single active message
	ActionRegieMessageClear = "REGIE_MESSAGE_CLEAR" // Admin (retire) OR Anim (acknowledge) → Server: clear the active message
	ActionRegieMessage      = "REGIE_MESSAGE"       // Server → Client (/ws/admin + /ws/anim only): current message state

	// ENTRACTE (v6.5.2, #119 — contracts/websocket-actions.md §"ENTRACTE_SET"):
	// admin → server, idempotent (payload carries the desired state, not a
	// toggle — D3). anim is deliberately excluded (contrat "contrôle réservé
	// à l'admin"), unlike the "conduite en direct" actions above.
	ActionEntracteSet = "ENTRACTE_SET"

	// ActionUpdateEntracteConfig (v6.5.2, #119, C1 — contracts/
	// websocket-actions.md §"UPDATE_ENTRACTE_CONFIG"): admin → server, saves
	// the entracte panel configuration. A dedicated action, NOT an extension
	// of UPDATE_QUIZ_META — two forms on the same Quiz page, two save
	// buttons, each owning its own fields (contract: grafting a second form
	// onto QuizMetaPayload's existing "absent vs cleared" pointer mechanic
	// would make an accidental cross-wipe between the two forms more
	// likely, not less).
	ActionUpdateEntracteConfig = "UPDATE_ENTRACTE_CONFIG"

	// RAFALE actions (v8.0.0, #107 — contracts/rafale.md §5). RAFALE_VALIDATE/
	// INVALIDATE mirror MEMOTION's "conduite en direct" pattern (admin+anim,
	// empty {} payload, same as MEMOTION_REVEAL/MEMOTION_STOP_TIMER above —
	// no dedicated payload struct needed). RAFALE_SET_TEAMS reuses the
	// {TEAMS:[...]} shape (RafaleSetTeamsPayload, alias below), admin-only
	// like MEMORY_SET_TEAMS/MEMOTION_SET_TEAMS (contract §5.1).
	ActionRafaleValidate   = "RAFALE_VALIDATE"   // Admin/Anim → Server: current answer judged correct
	ActionRafaleInvalidate = "RAFALE_INVALIDATE" // Admin/Anim → Server: current answer judged incorrect
	ActionRafaleSetTeams   = "RAFALE_SET_TEAMS"  // Admin → Server: participating teams + play order
	// RAFALE_ANSWER/RAFALE_TICK are SERVER → CLIENT only (contract §5.2) —
	// deliberately absent from internal/server/inbound_allowlist.go (see
	// inbound_allowlist_rafale_test.go's regression guard). RAFALE_ANSWER
	// carries the expected answer, contract §2.3: never diffused through
	// GameState, sent only to admin+anim via BroadcastToTypes — see
	// RafaleAnswerPayload below. RAFALE_TICK is the lightweight per-question
	// countdown (contract §2.2's "seul mécanisme réellement nouveau"),
	// broadcast to all clients without re-emitting the full GameState.
	ActionRafaleAnswer = "RAFALE_ANSWER"
	ActionRafaleTick   = "RAFALE_TICK"

	// ActionQuestionSound (v11.1, #219 — contracts/sound.md §10.2,
	// websocket-actions.md §"QUESTION_SOUND"): admin+anim → server, same
	// "conduite en direct" périmètre as REVEAL/BUMPER_POINTS/RAFALE_VALIDATE
	// above. Carries QuestionSoundPayload{COMMAND}. Without effect (never an
	// error) if the current question carries no sound — the handler is a
	// thin dispatch onto the question-sound adapter (cmd/server/
	// question_sound.go), which already no-ops in that case.
	ActionQuestionSound = "QUESTION_SOUND"
)

// QuestionSoundCommand is one of the four conduite gestures a client may
// send with ActionQuestionSound (contract §10.2/§10.7).
type QuestionSoundCommand string

const (
	QuestionSoundCommandPlay   QuestionSoundCommand = "PLAY" // (re)start from the beginning — the "Rejouer" gesture
	QuestionSoundCommandPause  QuestionSoundCommand = "PAUSE"
	QuestionSoundCommandResume QuestionSoundCommand = "RESUME"
	QuestionSoundCommandStop   QuestionSoundCommand = "STOP"
)

// QuestionSoundPayload is ActionQuestionSound's payload (contract
// websocket-actions.md §"QUESTION_SOUND").
type QuestionSoundPayload struct {
	Command QuestionSoundCommand `json:"COMMAND"`
}

// FSInfo represents file storage information
type FSInfo struct {
	Used  int     `json:"USED"`
	Free  int     `json:"FREE"`
	Total int     `json:"TOTAL"`
	PUsed float64 `json:"P_USED"`
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
	Source   string // "TCP", "WebSocket"
	Data     *Message
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
	// BoundPlayerID is the bumper ID the connection was bound to when the
	// message was READ (#246). Used only when the connection has already closed
	// by the time the message is dispatched (last message before a disconnect),
	// so that it is not mistaken for an unbound connection. Empty otherwise.
	BoundPlayerID string
	Timestamp     time.Time
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
	ID      string `json:"ID"`
	Points  int    `json:"POINTS"`
	Display *bool  `json:"DISPLAY,omitempty"` // v11.2, #252: false = hidden correction
}

// TeamPointsPayload for TEAM_POINTS action
type TeamPointsPayload struct {
	Team    string `json:"TEAM"`
	Points  int    `json:"POINTS"`
	Display *bool  `json:"DISPLAY,omitempty"` // v11.2, #252: false = hidden correction
}

// ClientsPayload for CLIENTS action (client count broadcast)
type ClientsPayload struct {
	AdminCount   int `json:"ADMIN_COUNT"`
	TVCount      int `json:"TV_COUNT"`
	VPlayerCount int `json:"VPLAYER_COUNT"`
	// AnimCount — v6.2.0 (#155): number of connected interface animateur
	// clients (ClientTypeAnim). Diffused to admin only (CLIENTS stays
	// admin-only, contracts/websocket-endpoints.md), consumed by the Navbar
	// badge (dev-frontend F3) — the régie signalement for #155/#156 (D2).
	AnimCount int `json:"ANIM_COUNT"`
	BuzzerWS  int `json:"BUZZER_WS_COUNT"`
}

// NextQuestionPayload for NEXT_QUESTION action (v6.2.0, #155 tâche B5;
// CurrentPosition/TotalQuestions added v6.2.x #166).
// Server → Client, ClientTypeAnim exclusively (contracts/websocket-actions.md
// §"Animateur"). The next-question fields (ID/Question/Category/Type/Points/
// Time) are the zero value (empty) when no question is playable — see
// App.getNextQuestionPayload's doc comment for the full computation rule
// (parity with GamePage.jsx's nextUnplayedQuestion). CurrentPosition and
// TotalQuestions are NOT part of that "all or nothing" group: they describe
// the current question and the quiz as a whole, and are populated even when
// there is no next question (e.g. the last question of the quiz) — no
// omitempty, project-wide GameState convention (CLAUDE.md).
type NextQuestionPayload struct {
	ID              string `json:"ID"`
	Question        string `json:"QUESTION"`
	Category        string `json:"CATEGORY"`
	Type            string `json:"TYPE"`
	Points          int    `json:"POINTS"`
	Time            int    `json:"TIME"`
	CurrentPosition int    `json:"CURRENT_POSITION"`
	TotalQuestions  int    `json:"TOTAL_QUESTIONS"`
}

// CreditPointsPayload is shared by both directions of the code review
// MAJEUR-1 mechanism (v6.2.0, #155/#156 — contracts/websocket-actions.md
// §"Animateur"): SET_CREDIT_POINTS (Admin → Server, the admin's adjusted
// pointsInput) and CREDIT_POINTS (Server → Client, ClientTypeAnim
// exclusively — the current effective base credit amount, echoed back).
type CreditPointsPayload struct {
	Points int `json:"POINTS"`
}

// AwardedTeamsPayload for AWARDED_TEAMS action (v6.2.x, #170 — synchronized
// credit lock across animateur tablets, contracts/websocket-actions.md
// §"Animateur"). Server → Client, ClientTypeAnim exclusively. A pure
// projection of Engine.history's POINTS_AWARDED events for the CURRENT
// question — no new GameState field, see App.buildAwardedTeamsMessage's doc
// comment for the exact filter/grouping rule. TEAMS is always [], never nil
// (no omitempty on either field — project-wide GameState convention,
// CLAUDE.md).
type AwardedTeamsPayload struct {
	QuestionID string               `json:"QUESTION_ID"`
	Teams      []AwardedTeamEntry   `json:"TEAMS"`
	Players    []AwardedPlayerEntry `json:"PLAYERS"` // v11.2, #254: players credited individually ([] never null)
}

// AwardedPlayerEntry is one player credited individually for the current question.
type AwardedPlayerEntry struct {
	ID        string `json:"ID"`
	Team      string `json:"TEAM"`
	Points    int    `json:"POINTS"`
	Timestamp int64  `json:"TIMESTAMP"`
}

// AwardedTeamEntry is one team already credited for the current question
// (AwardedTeamsPayload.Teams) — Points is the SUM of every POINTS_AWARDED
// event for that team on this question (a team may be credited more than
// once from the régie, which has no double-credit guard), Timestamp is the
// FIRST such event's timestamp. A zero Points is a valid, deliberate entry
// (a "0 pt" refusal locks the team exactly like a positive credit) — never
// filtered out.
type AwardedTeamEntry struct {
	Team      string `json:"TEAM"`
	Points    int    `json:"POINTS"`
	Timestamp int64  `json:"TIMESTAMP"`
	TeamLevel bool   `json:"TEAM_LEVEL"` // v11.2, #254: at least one TEAM-level credit (always present)
}

// RegieMessagePayload for REGIE_MESSAGE (v6.4.x, #167 — contracts/
// websocket-actions.md §"Messagerie régie"). Server → Client, ClientTypeAdmin
// + ClientTypeAnim exclusively. Reflects the single in-memory régie
// instruction slot (App.regieMessage): Active=false is the rest state
// (TEXT="", SentAt=0), ClearedBy survives the clear so the régie can show
// "Vu par l'animateur" instead of a bare empty state.
//
// No omitempty on any of the 4 fields — same project-wide GameState
// convention (CLAUDE.md §"Implementation Rules") and for the same reason: an
// omitted ACTIVE:false would leave a stale/already-acknowledged message
// displayed on a tablet that only diffs the JSON it receives.
type RegieMessagePayload struct {
	Active    bool   `json:"ACTIVE"`
	Text      string `json:"TEXT"`
	SentAt    int64  `json:"SENT_AT"`
	ClearedBy string `json:"CLEARED_BY"` // "ANIM", "REGIE", or "" — origin of the last clear
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
	JobID string `json:"JOB_ID"`
	// Target distinguishes which generation path emitted this progress —
	// "QUIZ" (default, existing chemin) or "RAFALE" (#203, v8.1.0). Additive
	// and backward-compatible: a client that doesn't know this field simply
	// ignores it and keeps behaving as before (contract
	// rafale-ai-generation.md §6). Always populated by the server (never
	// omitted) since AI_GENERATION_PROGRESS is now a singleton shared by two
	// independent modales that must each filter on it.
	Target       string `json:"TARGET"`
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

// FlipMemoryCardPayload for FLIP_MEMORY_CARD action.
//
// ID and CardScope are both additive (#187, v7.1.0), neither breaks an
// older client that never sends them:
//   - ID (optional) is the emitting VPlayer's bumper ID, resolved server-side
//     with the SAME 3-pass pattern as VPLAYER_QCM_ANSWER/ARDOISE_INPUT
//     (payload.ID → msg.ID → clientID) — used ONLY for the vplayer turn
//     check (contract §9.2 dérogation, websocket-actions.md fiche
//     FLIP_MEMORY_CARD). tv/anim never need it, they have no team.
//   - CardScope.MotionCardID (optional) binds the flip to a MEMOTION card's
//     own grid instead of the question-scoped one (contract §9).
type FlipMemoryCardPayload struct {
	CardID string `json:"CARD_ID"`      // Card ID to flip (e.g., "1-1", "2-2")
	ID     string `json:"ID,omitempty"` // Bumper ID (VPlayer) — #187, v7.1.0
	CardScope
}

// MemorySetTeamsPayload for MEMORY_SET_TEAMS action
type MemorySetTeamsPayload struct {
	Teams []string `json:"TEAMS"` // List of team names participating
}

// MotionSelectPayload for MEMOTION_SELECT action (Admin → Server)
type MotionSelectPayload struct {
	CardID string `json:"CARD_ID"` // ID of the card to select (e.g. "mc-1")
}

// CardScope is embedded (not nested — a payload struct embeds it directly)
// in a per-type action's payload to bind that action to the MEMOTION card
// currently in play — contract §9. Optional: its zero value (absent
// MOTION_CARD_ID) preserves today's behavior for every existing action,
// none of which is required to carry it. No action in v7.0.0 actually
// carried CardScope — #185 (QCM-in-card) has no new action at all (contract
// §7.1), so the invariant this type supports (game.ValidateCardScope) was
// posed and tested ahead of any real consumer. #186 (ARDOISE-in-card) was
// closed "not planned" before it shipped; FLIP_MEMORY_CARD (#187, v7.1.0)
// is the first action to actually carry MOTION_CARD_ID. Delivered ahead of
// its first consumer deliberately: bolting it on after the fact would have
// reopened the MEMOTION host — exactly what #184's agnosticity test
// (contract §10) forbids.
type CardScope struct {
	MotionCardID string `json:"MOTION_CARD_ID,omitempty"`
}

// MotionDonePayload for MEMOTION_DONE action (Admin → Server)
type MotionDonePayload struct {
	CardID     string `json:"CARD_ID"`               // ID of the card being closed
	WinnerTeam string `json:"WINNER_TEAM,omitempty"` // Team name if a winner, "" if none
	// Units (#184, B-B5, contract §9.3) — a *int (not int) so the handler
	// can tell "absent" (⇒ default 1, current behavior unchanged) apart
	// from an explicit 0 (a meaningful "zero progress" outcome under
	// POINTS_RULE.MODE FIXED/PER_UNIT — contract §6.2). Consumed by
	// POINTS_RULE.MODE=="PER_UNIT"/"FIXED"; ignored under the default
	// STARS scale.
	Units *int `json:"UNITS,omitempty"`
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

// RafaleSetTeamsPayload is an alias for MemorySetTeamsPayload — same
// {TEAMS: [...]} structure (RAFALE_SET_TEAMS action, contract §5.1),
// same pattern as MotionSetTeamsPayload above.
type RafaleSetTeamsPayload = MemorySetTeamsPayload

// RafaleCardActionPayload for RAFALE_VALIDATE / RAFALE_INVALIDATE
// (Admin/Anim → Server). CardScope.MotionCardID (#217, v9.0.0, contract
// §14.5/§9) is optional and additive, same convention as
// FlipMemoryCardPayload above: absent/empty ⇒ classic manche-scoped round
// (today's behavior, unchanged); present ⇒ binds the validate/invalidate
// to a MEMOTION card's own RAFALE mini-round instead. Both actions carried
// no payload at all before #217 — an older client sending no MSG (or `{}`)
// still parses fine into this struct's zero value.
type RafaleCardActionPayload struct {
	CardScope
}

// RafaleNextPayload carries the question suite's statement WITHOUT its
// answer (#202, contract §13.3) — same shape as GameState's own
// RafaleCurrent (models.go), a distinct type here (rather than reusing
// RafaleCurrent's JSON tags directly) only so this file's payload types
// stay self-contained and independently documented, same convention as
// every other *Payload in this file.
//
// Deliberately has NO Answer field: the next question's answer is not
// needed before it becomes current (it arrives on the FOLLOWING
// RAFALE_ANSWER broadcast, at the instant it does), transmitting it now
// would double the leak surface this whole payload is already restricted
// to guard (see RafaleAnswerPayload's own comment), and displaying it would
// work against the "SUIVANTE" zone's entire point of being unobtrusive
// (contract §13.6).
type RafaleNextPayload struct {
	ID         string `json:"ID"`
	Question   string `json:"QUESTION"`
	Category   string `json:"CATEGORY"`
	Difficulty int    `json:"DIFFICULTY"`
}

// RafaleAnswerPayload for the RAFALE_ANSWER action (Server → admin+anim
// only, contract §5.2/§2.3) — the current RAFALE question's expected
// answer. Deliberately its own dedicated action rather than a GameState
// field: SerializeForWebClient serves the identical payload to /tv and
// /anim, so no per-field exclusion list could keep the answer off TV
// (contract §2.3 — see the ardoise_leak_128 precedent this design avoids
// reproducing).
//
// Next (#202, contract §13.3) extends this SAME restricted channel to the
// pre-fetched next question's statement — same confidentiality reasoning
// as Answer above applies to it too (§13.2: a competitive advantage in a
// ~3s-per-question mode). Deliberately NO `omitempty`: nil/null is a
// meaningful value ("no next question" — pool empty or cap imminent,
// contract §13.5), not an absence the client should have to special-case
// away from "field missing" — same "no omitempty" discipline the project
// already applies to GameState (CLAUDE.md).
// CardScope (#217, v9.0.0, contract §14.6) is optional and additive:
// MotionCardID absent/empty ⇒ classic manche-scoped round (today's
// behavior, unchanged). Present ⇒ this answer belongs to a MEMOTION card's
// own RAFALE mini-round instead — Next is always nil in that case (a
// card's mini-round never pre-fetches, contract §14.2).
type RafaleAnswerPayload struct {
	ID     string             `json:"ID"`     // RafaleQuestion.ID this answer belongs to
	Answer string             `json:"ANSWER"` // expected answer, text only
	Next   *RafaleNextPayload `json:"NEXT"`   // pre-fetched next question, answer-free; null = none (contract §13.5)
	CardScope
}

// RafaleTickPayload for the RAFALE_TICK action (Server → all clients,
// contract §5.2) — the lightweight per-question countdown, broadcast
// without re-emitting the full GameState (contract §2.2).
//
// CardScope.MotionCardID (C1, retour QUALIF v9.0.0.4) — "" for the classic
// manche-scoped round (unchanged behavior), the active card's ID for a
// #217 RAFALE mini-round. Before C1 this payload carried no host
// information at all, so a client had no way to tell a card-hosted tick
// apart from a classic one — the "chrono figé" bug: a client scoping its
// own display to MOTION_CARD_ID could never find a match and the card's
// countdown never advanced on screen, even though the engine was ticking
// it down correctly the whole time (contracts/rafale.md §14, engine-level
// coverage already existed for this).
type RafaleTickPayload struct {
	QuestionTime int `json:"QUESTION_TIME"` // seconds remaining on the current question
	CardScope
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

// EntracteSetPayload for ENTRACTE_SET action (v6.5.2, #119) — an explicit
// idempotent command (D3), not a toggle: ACTIVE always carries the state the
// client wants, so a double-click or a network resend can never invert it.
type EntracteSetPayload struct {
	Active bool `json:"ACTIVE"`
}

// EntracteConfigPayload for UPDATE_ENTRACTE_CONFIG action (v6.5.2, #119,
// C1/C3, contract websocket-actions.md §"UPDATE_ENTRACTE_CONFIG"). Saves
// the entracte panel configuration — a property of the game, edited from
// the Quiz page.
//
// AnimIntensity and TransitionMs are *int, unlike Title/Subtitle/PanelSize/
// AnimPeriod — deliberately, same convention as QuizMetaPayload's
// Populations/Difficulties/Language/Objectives (absent vs present-and-zero
// must be distinguishable). 0 is a MEANINGFUL value for both: "animation
// disabled" and "instant switch, no fade" respectively — a plain int field
// couldn't tell "the client explicitly chose 0" apart from "the client
// didn't send this field at all", which cmd/server/main.go's handler needs
// to merge against the current saved config correctly (nil -> keep current
// value; non-nil, even *0 -> apply it). PanelSize/AnimPeriod don't need
// this: 0 is never a legal value for either (clamped up to 20/2), so a
// plain int naturally self-heals via clamping the same way it always has.
type EntracteConfigPayload struct {
	Title         string `json:"TITLE"`
	Subtitle      string `json:"SUBTITLE"`
	PanelSize     int    `json:"PANEL_SIZE"`
	AnimPeriod    int    `json:"ANIM_PERIOD"`
	AnimIntensity *int   `json:"ANIM_INTENSITY,omitempty"`
	TransitionMs  *int   `json:"TRANSITION_MS,omitempty"`
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
	// v11.2 (#246) — diagnostic only (contracts/websocket-actions.md ARDOISE_INPUT).
	Seq    int    `json:"SEQ,omitempty"`    // per question & per connection counter, >= 1
	Reason string `json:"REASON,omitempty"` // key | clear | backspace_empty | resend | visible
}

// ArdoiseFinalPayload for ARDOISE_FINAL (v11.2, #246).
type ArdoiseFinalPayload struct {
	ID         string `json:"ID"`
	QuestionID string `json:"QUESTION_ID"`
	Text       string `json:"TEXT"`
	Hash       string `json:"HASH"`
	Seq        int    `json:"SEQ,omitempty"`
}

// ArdoiseFinalAckPayload for ARDOISE_FINAL_ACK — never carries text (#128).
type ArdoiseFinalAckPayload struct {
	QuestionID string `json:"QUESTION_ID"`
	Hash       string `json:"HASH"`
}

// TeamsUpdateRefusedPayload for TEAMS_UPDATE_REFUSED (v11.2, #249).
type TeamsUpdateRefusedPayload struct {
	Reason string   `json:"REASON"`
	Teams  []string `json:"TEAMS"`
}

// NewGamePayload for NEW_GAME (v11.2, #243). Pointers tell "absent" from false:
// no field at all = v11.1 behaviour; as soon as one field is present the absent
// ones are false.
type NewGamePayload struct {
	PurgeScores    *bool `json:"PURGE_SCORES,omitempty"`
	PurgePlayers   *bool `json:"PURGE_PLAYERS,omitempty"`
	PurgeTeams     *bool `json:"PURGE_TEAMS,omitempty"`
	ResetQuestions *bool `json:"RESET_QUESTIONS,omitempty"`
}

// NewGameRefusedPayload for NEW_GAME_REFUSED (targeted to the sender).
type NewGameRefusedPayload struct {
	Reason string `json:"REASON"`
}

// TeamPointsBatchPayload for TEAM_POINTS_BATCH (v11.2, #244/#251). Entry points are
// raw JSON numbers so a non-integer can be refused as INVALID_POINTS.
type TeamPointsBatchPayload struct {
	QuestionID string `json:"QUESTION_ID"`
	Entries    []struct {
		Team   string          `json:"TEAM"`
		Points json.RawMessage `json:"POINTS"`
	} `json:"ENTRIES"`
	Display *bool `json:"DISPLAY,omitempty"` // reserved for #252
}

// BumperPointsBatchPayload for BUMPER_POINTS_BATCH (v11.2, #254).
type BumperPointsBatchPayload struct {
	QuestionID string `json:"QUESTION_ID"`
	Entries    []struct {
		ID     string          `json:"ID"`
		Points json.RawMessage `json:"POINTS"`
	} `json:"ENTRIES"`
	Display *bool `json:"DISPLAY,omitempty"`
}

// BumperPointsBatchResultPayload for BUMPER_POINTS_BATCH_RESULT (targeted to the sender).
type BumperPointsBatchResultPayload struct {
	Status     string                   `json:"STATUS"`
	Reason     string                   `json:"REASON,omitempty"`
	QuestionID string                   `json:"QUESTION_ID"`
	Applied    []BumperPointsBatchEntry `json:"APPLIED"`
	Skipped    []BumperPointsBatchSkip  `json:"SKIPPED"`
	Players    []string                 `json:"PLAYERS"`
}
type BumperPointsBatchEntry struct {
	ID     string `json:"ID"`
	Points int    `json:"POINTS"`
}
type BumperPointsBatchSkip struct {
	ID     string `json:"ID"`
	Reason string `json:"REASON"`
}

// TeamPointsBatchApplied / Skipped are the RESULT array items.
type TeamPointsBatchApplied struct {
	Team   string `json:"TEAM"`
	Points int    `json:"POINTS"`
}
type TeamPointsBatchSkipped struct {
	Team   string `json:"TEAM"`
	Reason string `json:"REASON"`
}

// TeamPointsBatchResultPayload for TEAM_POINTS_BATCH_RESULT (targeted to the sender).
type TeamPointsBatchResultPayload struct {
	Status     string                   `json:"STATUS"`
	Reason     string                   `json:"REASON,omitempty"`
	QuestionID string                   `json:"QUESTION_ID"`
	Applied    []TeamPointsBatchApplied `json:"APPLIED"`
	Skipped    []TeamPointsBatchSkipped `json:"SKIPPED"`
	Teams      []string                 `json:"TEAMS"`
}

// HelloAckPayload for HELLO_ACK (v11.2, #253).
type HelloAckPayload struct {
	ClientID string `json:"CLIENT_ID"`
}

// LaunchLockPayload for LAUNCH_LOCK: STATE is OPEN (open, then renew) or CLOSE.
type LaunchLockPayload struct {
	State string `json:"STATE"`
}

// LaunchLockRefusedPayload for LAUNCH_LOCK_REFUSED.
type LaunchLockRefusedPayload struct {
	Reason string `json:"REASON"` // LOCKED_BY_OTHER | ENTRACTE_ACTIVE
}

// ActionRefusedPayload for ACTION_REFUSED.
type ActionRefusedPayload struct {
	Action string `json:"ACTION"`
	Reason string `json:"REASON"` // LAUNCH_IN_PROGRESS
}

// RevealPayload for REVEAL (v11.2, #246): FORCE reveals despite pending ARDOISE answers.
type RevealPayload struct {
	Force bool `json:"FORCE"`
}

// RevealRefusedPendingTeam is one entry of REVEAL_REFUSED.PENDING.
type RevealRefusedPendingTeam struct {
	Team    string `json:"TEAM"`
	Offline bool   `json:"OFFLINE"`
}

// RevealRefusedPayload for REVEAL_REFUSED (targeted to the sender).
type RevealRefusedPayload struct {
	Reason  string                     `json:"REASON"`
	Pending []RevealRefusedPendingTeam `json:"PENDING"`
}

// ClientTracePayload for CLIENT_TRACE (v11.2, #246): client diagnostic event,
// journalised by the server, no effect on game state.
type ClientTracePayload struct {
	Kind        string                 `json:"KIND"`
	At          int64                  `json:"AT"`
	Team        string                 `json:"TEAM,omitempty"`
	PrevLen     int                    `json:"PREV_LEN,omitempty"`
	CauseAction string                 `json:"CAUSE_ACTION,omitempty"`
	Detail      map[string]interface{} `json:"DETAIL,omitempty"`
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
	DefaultQuestionImageIsCustom bool              `json:"default_question_image_is_custom"` // true if custom image uploaded, false = embedded fallback
	NewGameBackgrounds           []game.Background `json:"new_game_backgrounds"`             // NEW_GAME screen backgrounds (v4.0.4)
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
	MAC     string `json:"MAC"`             // Buzzer MAC address
	Status  string `json:"STATUS"`          // "downloading", "flashing", "done", "error"
	Percent int    `json:"PERCENT"`         // Progress percentage (0-100)
	Error   string `json:"ERROR,omitempty"` // Error message if status == "error"
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
	Intensity  int     `json:"INTENSITY"`             // 0-255
	Effect     string  `json:"EFFECT"`                // "SOLID", "BLINK", "DIM", "COMET", "SPINNER"
	CometColor *[3]int `json:"COMET_COLOR,omitempty"` // COMET band color (nil = firmware default gold)
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
//
// ENTRACTE_CONFIG_SAVED (v6.5.2, #119, C4, arbitrage 2026-08-20) joins this
// list for a different reason than QUIZ_OBJECTIVES: it isn't a secret, it's
// simply not the config TV/VJoueur/anim need — they display the DIFFUSED
// panel (GameState.EntracteConfig, NOT in this list, always broadcast),
// while ENTRACTE_CONFIG_SAVED exists only to feed the Quiz page's edit form
// so an admin can see their just-saved values even while an older
// configuration is still frozen on screen during an active pause (contract
// game-state.md §"Configuration gelée à l'activation").
var AdminOnlyGameFields = []string{
	"QUIZ_OBJECTIVES",
	"ENTRACTE_CONFIG_SAVED",
}

// VPlayerOnlyGameFields lists "GAME" node fields stripped from the VPlayer
// path SPECIFICALLY — in addition to AdminOnlyGameFields, never instead of
// it — because TV and /anim have a legitimate need for them that VPlayer
// does not (#128, v6.5.2, contract vplayer-payload-filter.md §6). A field
// here can't simply join AdminOnlyGameFields: that list is stripped from
// EVERY non-admin client, and these fields must still reach TV/anim.
//
// ARDOISE_ANSWERS: the TV displays each team's answer at REVEAL
// (PlayerDisplay.jsx, guarded !isVPlayer) and /anim lists them live (#158,
// AnimArdoiseList) — but a VJoueur has no legitimate reason to see what
// other teams are typing while a round is in progress. #129 closed the
// keystroke-triggered broadcast that carried this in near-real-time; #128
// found the field still reachable via every OTHER GameState broadcast
// (START/STOP/PAUSE/CONTINUE/UPDATE_TIMER — the last firing once per
// second for the whole question), because the field belonged to no removal
// list at all. See SerializeForVPlayerCommon and the three application
// sites below (D2/§6 "un site oublié laisse la fuite ouverte").
var VPlayerOnlyGameFields = []string{
	"ARDOISE_ANSWERS",
	"LAUNCH_LOCK", // #253: no use on a tablet
}

// TVOnlyGameFields lists GAME fields stripped from the TV path only (anim keeps
// them): LAUNCH_LOCK is for admin and anim banners (#253).
var TVOnlyGameFields = []string{
	"LAUNCH_LOCK",
}

// SerializeForTV is SerializeForWebClient() plus TVOnlyGameFields stripped.
func (m *Message) SerializeForTV() ([]byte, error) {
	return m.serializeFiltered(TVOnlyGameFields)
}

// serializeFiltered is the shared implementation behind SerializeForWebClient
// and SerializeForVPlayerCommon (#128): strips AdminOnlyBumperFields from
// every bumper, AdminOnlyGameFields from the GAME node, and — when the
// caller is the VPlayer path — extraGameFields (VPlayerOnlyGameFields) too.
// extraGameFields nil/empty for TV/anim, non-nil for VPlayer.
//
// #128 D1: NO action-name guard here — a payload is filtered if and only if
// it PARSES with a GAME-or-bumpers shape; json.Unmarshal failing, or the
// expected keys being absent, falls back to the message unchanged. This
// used to be gated on `m.Action != ActionUpdate`, which meant six other
// actions carrying the full GameState (START/STOP/PAUSE/CONTINUE/
// UPDATE_TIMER, plus COUNTDOWN's timer tick) bypassed filtering entirely —
// the actual #128 defect, wider than the issue's own STOP-only framing.
// Enumerating "the actions that carry GAME" would only reproduce that bug
// under a new name the next time a broadcast starts carrying GameState —
// filtering by payload SHAPE is the only criterion that doesn't rot.
//
// Fields stripped per bumper: AdminOnlyBumperFields (FIRMWARE_VERSION,
// IS_OUTDATED, OTA_STATUS, OTA_PERCENT, ACK_PENDING). The top-level "config"
// key is also removed (server-side config is not needed by TV/VPlayer/anim).
//
// The format produced by GetGameJSON() (GameData struct) uses lowercase map keys:
//   - "bumpers" → map[mac]*Bumper  (not "BUMPERS" / slice)
//   - "teams"   → map[name]*Team   (not "TEAMS"   / slice)
//   - "GAME"    → GameState node
func (m *Message) serializeFiltered(extraGameFields []string) ([]byte, error) {
	return m.serializeFilteredFor(extraGameFields, false, "")
}

// serializeFilteredFor is serializeFiltered plus, for a tablette (vplayer), the
// #258 answer filter (filterVPlayerAnswers) applied as the LAST step, with
// recipientID = the bound player ("" = unbound / shared frame: everything neutral).
func (m *Message) serializeFilteredFor(extraGameFields []string, vplayer bool, recipientID string) ([]byte, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(m.Msg, &raw); err != nil {
		// Not a GameData-shaped payload (or MSG isn't even an object, e.g.
		// REVEAL's plain string) — nothing to filter, send unchanged.
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
	// "GAME" node: strip admin-only fields, plus the caller's extra list
	// (VPlayerOnlyGameFields for the VPlayer path, nil for TV/anim).
	if gameNode, ok := raw["GAME"].(map[string]interface{}); ok {
		for _, field := range AdminOnlyGameFields {
			delete(gameNode, field)
		}
		for _, field := range extraGameFields {
			delete(gameNode, field)
		}
	} else if _, hasBumpers := raw["bumpers"]; !hasBumpers {
		// Neither "GAME" nor "bumpers" present — this isn't a GameData
		// payload at all (an unrelated action whose MSG happens to decode
		// as a JSON object, e.g. a simple {} payload). Nothing to filter;
		// send unchanged rather than re-marshal a raw map that could differ
		// byte-for-byte from the original for no reason.
		return json.Marshal(m)
	}
	// "config" is a server-side key not needed by TV/VPlayer/anim clients
	delete(raw, "config")
	if vplayer {
		filterVPlayerAnswers(raw, recipientID, "")
	}

	stripped, err := json.Marshal(raw)
	if err != nil {
		return json.Marshal(m)
	}
	out := *m
	out.Msg = stripped
	return json.Marshal(&out)
}

// SerializeForWebClient returns the payload with admin-only fields stripped
// from bumpers and from the GAME node — TV and VPlayer clients (and, via
// serializeForClientType, /anim — contracts/vplayer-payload-filter.md §1)
// do not need firmware/OTA/ACK metadata nor QUIZ_OBJECTIVES/
// ENTRACTE_CONFIG_SAVED, so stripping them reduces payload size (and, for
// the admin-only GAME fields, enforces confidentiality).
//
// #128 (v6.5.2): applies to EVERY action whose payload has a GAME/bumpers
// shape, not just ActionUpdate — see serializeFiltered's doc comment for
// why an action-name guard was the defect, not a safety net. An action
// without that shape (REVEAL's plain string, HELLO, LED_SET, …) passes
// through the fallback inside serializeFiltered, byte-identical to before.
func (m *Message) SerializeForWebClient() ([]byte, error) {
	return m.serializeFiltered(nil)
}

// SerializeForVPlayerCommon is SerializeForWebClient() plus
// VPlayerOnlyGameFields stripped from the GAME node (#128, contract
// vplayer-payload-filter.md §6) — the VPlayer path's counterpart to
// SerializeForWebClient, needed because VPlayer can no longer share a
// payload with TV/anim (ARDOISE_ANSWERS is legitimate for them, not for
// VPlayer). This is the fallback SerializeForVPlayer uses whenever its own
// per-recipient reduction doesn't apply (not an UPDATE, phase outside
// PREPARE/READY, or an unidentified player) — an update must never be
// silently dropped or narrower than this.
func (m *Message) SerializeForVPlayerCommon() ([]byte, error) {
	return m.serializeFilteredFor(VPlayerOnlyGameFields, true, "")
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
// JSON error, this falls back to SerializeForVPlayerCommon() — the complete
// VPlayer-filtered payload (#128: AdminOnlyGameFields AND
// VPlayerOnlyGameFields both stripped) — an update must never be silently
// dropped or narrower than that fallback:
//  1. m.Action == ActionUpdate;
//  2. MSG.GAME.PHASE is PREPARE or READY — read from the payload itself
//     (never passed in), so the decision always matches what is actually
//     being sent, not a caller's possibly-stale belief about the phase;
//  3. playerID != "" — an unidentified VPlayer (no PLAYER_CONNECT completed
//     yet, SetClientPlayerID never called for it) must always receive the
//     complete card: VPlayerPage.jsx recovers a lost identity by scanning
//     bumpers by NAME, which is impossible on a single-entry map.
//
// GAME and teams are left completely untouched (beyond the field strips
// below) — "teams" is deliberately NOT reduced even though only one bumper
// is kept (contract §2 rationale: PlayerDisplay.jsx's MEMORY/MEMOTION team
// bars read teams[name] without an !isVPlayer gate). Only "bumpers" is
// reduced to the single {playerID: ...} entry, with the same admin-only
// fields SerializeForWebClient strips.
func (m *Message) SerializeForVPlayer(playerID string) ([]byte, error) {
	if m.Action != ActionUpdate || playerID == "" {
		return m.SerializeForVPlayerCommon()
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(m.Msg, &raw); err != nil {
		return m.SerializeForVPlayerCommon()
	}

	gameNode, ok := raw["GAME"].(map[string]interface{})
	if !ok {
		return m.SerializeForVPlayerCommon()
	}
	// Strip admin-only GAME fields (QUIZ_OBJECTIVES, v6.1.0) AND
	// VPlayer-only-excluded fields (ARDOISE_ANSWERS, #128) — this reduced
	// path builds its own "raw" independently of serializeFiltered, so both
	// lists must be applied here too (contracts/ws-payload-serialization.md,
	// vplayer-payload-filter.md §6 "trois sites, une seule liste").
	// gameNode is a reference into raw (map values from json.Unmarshal into
	// map[string]interface{} are shared, not copied), so deleting from it
	// here is reflected in raw["GAME"] below without reassignment.
	for _, field := range AdminOnlyGameFields {
		delete(gameNode, field)
	}
	for _, field := range VPlayerOnlyGameFields {
		delete(gameNode, field)
	}
	phase, _ := gameNode["PHASE"].(string)
	if !vplayerReducedPhases[phase] {
		// #258: per-recipient frame (own team intact, other teams neutral while answers are open)
		return m.serializeFilteredFor(VPlayerOnlyGameFields, true, playerID)
	}

	bumpers, ok := raw["bumpers"].(map[string]interface{})
	if !ok {
		return m.SerializeForVPlayerCommon()
	}
	own, ok := bumpers[playerID]
	if !ok {
		// This player's bumper isn't in the current snapshot (e.g. evicted in
		// the same instant this broadcast was built) — fall back rather than
		// send a bumpers map that omits the recipient's own entry entirely.
		return m.SerializeForVPlayerCommon()
	}
	if bumper, ok := own.(map[string]interface{}); ok {
		for _, field := range AdminOnlyBumperFields {
			delete(bumper, field)
		}
	}
	raw["bumpers"] = map[string]interface{}{playerID: own}
	// "config" is a server-side key not needed by VPlayer clients (same as SerializeForWebClient)
	delete(raw, "config")
	filterVPlayerAnswers(raw, playerID, phase) // #258

	stripped, err := json.Marshal(raw)
	if err != nil {
		return m.SerializeForVPlayerCommon()
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

// --- #258: answers of the other teams and the right answer, filtered per phase and recipient

// vplayerOpenAnswerPhases are the phases during which answers are still open: the
// answer/time fields of the OTHER teams must not reach a tablette.
var vplayerOpenAnswerPhases = map[string]bool{"PREPARE": true, "READY": true, "COUNTDOWN": true, "STARTED": true, "PAUSED": true}

// FilterVPlayerAnswers is the exported entry of filterVPlayerAnswers (used by the
// fan-out in cmd/server).
func FilterVPlayerAnswers(raw map[string]interface{}, recipientID, phase string) {
	filterVPlayerAnswers(raw, recipientID, phase)
}

// filterVPlayerAnswers is THE single filter of contracts/vplayer-payload-filter.md §7,
// applied by SHAPE (GAME / bumpers / teams present), never by action name, as the
// last serialisation step of every frame destined to a tablette. raw is the decoded
// MSG ({GAME, bumpers, teams}); it is modified in place. recipientID is the bound
// player ("" = shared frame or unbound connection: nothing is "own", everything is
// neutralised while answers are open); phase "" reads GAME.PHASE (an unknown phase
// counts as open, i.e. the safe side).
//
//   - answers open (PREPARE..PAUSED): other teams' bumpers lose TIME, BUTTON, STATUS,
//     ANSWER_COLOR, ANSWERED_COLOR, HINTS_AT_BUZZ (neutral values, keys kept) and other
//     teams lose TIME, BUMPER, STATUS; own bumper and team-mates stay intact;
//   - QUESTION.QCM_CORRECT, ANSWER and MEDIA_ANSWER are removed in every phase but REVEALED;
//     MEMOTION cards (and MEMOTION_ACTIVE.STATE): ANSWER_TEXT/ANSWER_IMAGE/ANSWER/QCM_CORRECT
//     per card until it is REVEALED or DONE; QUESTION.EXPLANATION (animateur note) in every phase.
func filterVPlayerAnswers(raw map[string]interface{}, recipientID, phase string) {
	game, _ := raw["GAME"].(map[string]interface{})
	if phase == "" && game != nil {
		phase, _ = game["PHASE"].(string)
	}

	if game != nil {
		answerKeys := []string{"QCM_CORRECT", "ANSWER", "ANSWER_TEXT", "ANSWER_IMAGE"}
		revealedCard := func(id string) bool {
			states, _ := game["MEMOTION_CARD_STATES"].(map[string]interface{})
			st, _ := states[id].(string)
			return st == "REVEALED" || st == "DONE"
		}
		if q, ok := game["QUESTION"].(map[string]interface{}); ok {
			delete(q, "EXPLANATION") // animateur's note (#168): never on a tablette, any phase
			if phase != "REVEALED" {
				delete(q, "QCM_CORRECT")
				delete(q, "ANSWER")
				delete(q, "MEDIA_ANSWER")
				if cards, ok := q["MOTION_CARDS"].([]interface{}); ok {
					for _, c := range cards {
						card, ok := c.(map[string]interface{})
						if !ok {
							continue
						}
						if id, _ := card["ID"].(string); revealedCard(id) {
							continue
						}
						for _, k := range answerKeys {
							delete(card, k)
						}
					}
				}
			}
		}
		if phase != "REVEALED" {
			if active, ok := game["MEMOTION_ACTIVE"].(map[string]interface{}); ok {
				if st, ok := active["STATE"].(map[string]interface{}); ok {
					if cardID, _ := active["CARD_ID"].(string); !revealedCard(cardID) {
						for _, k := range answerKeys {
							delete(st, k)
						}
					}
				}
			}
		}
	}

	if phase != "" && !vplayerOpenAnswerPhases[phase] {
		return // STOPPED / REVEALED / NEW_GAME ...: answers are closed, transmitted as is
	}

	bumpers, _ := raw["bumpers"].(map[string]interface{})
	ownTeam := ""
	if recipientID != "" {
		if own, ok := bumpers[recipientID].(map[string]interface{}); ok {
			ownTeam, _ = own["TEAM"].(string)
		}
	}
	for id, b := range bumpers {
		bumper, ok := b.(map[string]interface{})
		if !ok {
			continue
		}
		if recipientID != "" && ownTeam != "" {
			if id == recipientID {
				continue
			}
			if team, _ := bumper["TEAM"].(string); team == ownTeam {
				continue
			}
		} else if recipientID != "" && id == recipientID {
			continue // the recipient's own bumper, even without a team
		}
		neutralise(bumper, "TIME", float64(0))
		neutralise(bumper, "HINTS_AT_BUZZ", float64(0))
		neutralise(bumper, "BUTTON", "")
		neutralise(bumper, "STATUS", "")
		neutralise(bumper, "ANSWER_COLOR", "")
		neutralise(bumper, "ANSWERED_COLOR", "")
	}
	teams, _ := raw["teams"].(map[string]interface{})
	for name, t := range teams {
		team, ok := t.(map[string]interface{})
		if !ok || (recipientID != "" && ownTeam != "" && name == ownTeam) {
			continue
		}
		neutralise(team, "TIME", float64(0))
		neutralise(team, "BUMPER", "")
		neutralise(team, "STATUS", "")
	}
}

// neutralise sets key to its neutral value only if the key is present (keys are kept,
// none is ever added).
func neutralise(m map[string]interface{}, key string, neutral interface{}) {
	if _, ok := m[key]; ok {
		m[key] = neutral
	}
}
