package game

import (
	"encoding/json"
)

// GamePhase represents the current game state
type GamePhase string

const (
	PhaseStopped   GamePhase = "STOPPED"
	PhasePrepare   GamePhase = "PREPARE"
	PhaseReady     GamePhase = "READY"
	PhaseCountdown GamePhase = "COUNTDOWN"
	PhaseStarted   GamePhase = "STARTED"
	PhasePaused    GamePhase = "PAUSED"
	PhaseRevealed  GamePhase = "REVEALED"
	PhaseEnroll    GamePhase = "ENROLL"    // Player enrollment phase (virtual players)
	PhaseNewGame   GamePhase = "NEW_GAME" // Game reset — scores/history cleared, ready to start fresh
)

// QuestionStatus represents question state (synced with GamePhase)
type QuestionStatus string

const (
	StatusAvailable QuestionStatus = "AVAILABLE"
	StatusPrepare   QuestionStatus = "PREPARE"
	StatusReady     QuestionStatus = "READY"
	StatusStarted   QuestionStatus = "STARTED"
	StatusPaused    QuestionStatus = "PAUSED"
	StatusStopped   QuestionStatus = "STOPPED"
	StatusRevealed  QuestionStatus = "REVEALED"
)

// Team represents a team in the game
type Team struct {
	Name       string `json:"NAME"`
	Color      []int  `json:"COLOR"`
	ColorName  string `json:"COLOR_NAME,omitempty"` // Named color key for LED lookup (e.g. "rouge", "bleu")
	Score      int    `json:"SCORE"`                // Calculated: TeamPoints + sum(bumpers)
	TeamPoints int    `json:"TEAM_POINTS"`           // Independent team points
	Time       int64  `json:"TIME,omitempty"`
	Status     string `json:"STATUS,omitempty"`
	Bumper     string `json:"BUMPER,omitempty"`
	Ready      bool   `json:"READY,omitempty"`
}

// AnswerColor represents a player's assigned answer color for QCM
type AnswerColor string

const (
	AnswerColorNone   AnswerColor = ""
	AnswerColorRed    AnswerColor = "RED"
	AnswerColorGreen  AnswerColor = "GREEN"
	AnswerColorYellow AnswerColor = "YELLOW"
	AnswerColorBlue   AnswerColor = "BLUE"
)

// Bumper represents a buzzer/player
type Bumper struct {
	Name        string      `json:"NAME,omitempty"`
	Team        string      `json:"TEAM,omitempty"`
	Score       int         `json:"SCORE"`
	Time        int64       `json:"TIME,omitempty"`
	Button      string      `json:"BUTTON,omitempty"`
	Status      string      `json:"STATUS,omitempty"`
	Version     string      `json:"VERSION,omitempty"`
	IP          string      `json:"IP,omitempty"`
	Protocol    string      `json:"PROTOCOL,omitempty"` // Connection protocol: "TCP" or "WebSocket" (empty = TCP for backward compat)
	Ready       bool        `json:"READY,omitempty"`
	AnswerColor AnswerColor `json:"ANSWER_COLOR,omitempty"`
	HintsAtBuzz int         `json:"HINTS_AT_BUZZ,omitempty"` // Number of QCM hints when player buzzed
	IsVirtual   bool        `json:"IS_VIRTUAL,omitempty"`    // True for virtual players (mobile app)
	IsVPlayer   bool        `json:"IS_VPLAYER,omitempty"`    // True for VPlayer (can answer all QCM colors)
	// OTA firmware update fields (added in v3.1.0)
	FirmwareVersion string `json:"FIRMWARE_VERSION,omitempty"` // BuzzClick firmware version reported via HELLO
	IsOutdated      bool   `json:"IS_OUTDATED,omitempty"`      // True if firmware is older than server-stored firmware
	OTAStatus       string `json:"OTA_STATUS,omitempty"`       // OTA status: "", "downloading", "flashing", "done", "error"
	OTAPercent      int    `json:"OTA_PERCENT,omitempty"`      // OTA progress percentage (0-100)
	// Connection status (added in v3.6.5) — NO omitempty: false is a meaningful value for the frontend
	Connected bool `json:"CONNECTED"` // true if buzzer is currently connected via WebSocket
	// ACK pending flag (added in v3.8.0) — omitempty: absent = false (retro-compat)
	AckPending bool `json:"ACK_PENDING,omitempty"` // true while server awaits an ACK from the buzzer
}

// BuzzState represents the buzz state of a buzzer relative to the current round.
// The server tracks this per-buzzer to drive the LED state machine.
type BuzzState string

const (
	// BuzzStateNone means nobody has buzzed yet (or the round was reset).
	BuzzStateNone BuzzState = "NONE"
	// BuzzStateMoi means this buzzer is the first buzz of its team.
	BuzzStateMoi BuzzState = "MOI"
	// BuzzStateEquipe means a team-mate buzzed first; this buzzer came after.
	BuzzStateEquipe BuzzState = "EQUIPE"
	// BuzzStateAutre means at least one other team has buzzed, but not this buzzer's team.
	BuzzStateAutre BuzzState = "AUTRE"
)

// QuestionType represents the type of question
type QuestionType string

const (
	QuestionTypeNormal   QuestionType = "NORMAL"
	QuestionTypeQCM      QuestionType = "QCM"
	QuestionTypeMemory   QuestionType = "MEMORY"
	QuestionTypeMemotion QuestionType = "MEMOTION" // NEW (v5.0.0): grid of cards with 3 faces
	QuestionTypeArdoise  QuestionType = "ARDOISE"  // NEW (v5.6.0): free-text answer via virtual keyboard
)

// KeyboardType represents the virtual keyboard layout for ARDOISE questions
type KeyboardType string

const (
	KeyboardTypeAZERTY KeyboardType = "AZERTY"
	KeyboardTypeNumpad KeyboardType = "NUMPAD"
)

// ArdoiseAnswer holds a team's free-text answer for an ARDOISE question
type ArdoiseAnswer struct {
	Text        string `json:"TEXT"`         // Current answer text
	SubmittedAt int64  `json:"SUBMITTED_AT"` // Timestamp in microseconds (last update)
}

// PointsTarget represents who receives points for a question
type PointsTarget string

const (
	PointsTargetPlayer PointsTarget = "PLAYER"
	PointsTargetTeam   PointsTarget = "TEAM"
)

// QuestionCategory represents the category of a question
type QuestionCategory string

const (
	CategoryNone          QuestionCategory = ""
	CategoryGeography     QuestionCategory = "GEOGRAPHY"
	CategoryEntertainment QuestionCategory = "ENTERTAINMENT"
	CategoryHistory       QuestionCategory = "HISTORY"
	CategoryArts          QuestionCategory = "ARTS"
	CategoryScience       QuestionCategory = "SCIENCE"
	CategorySports        QuestionCategory = "SPORTS"
	CategoryFood          QuestionCategory = "FOOD"
	CategoryAnimals       QuestionCategory = "ANIMALS"
)

// QCMAnswers holds the 4 possible answers for a QCM question
type QCMAnswers struct {
	Red    string `json:"RED"`
	Green  string `json:"GREEN"`
	Yellow string `json:"YELLOW"`
	Blue   string `json:"BLUE"`
}

// MemoryCard represents a card in the Memory game (text OR image)
type MemoryCard struct {
	Text    string `json:"TEXT,omitempty"`
	Image   string `json:"IMAGE,omitempty"`
	IsImage bool   `json:"IS_IMAGE"`
}

// MemoryPair represents a pair of cards to match in the Memory game
type MemoryPair struct {
	ID    int        `json:"ID"`
	Card1 MemoryCard `json:"CARD1"`
	Card2 MemoryCard `json:"CARD2"`
}

// MemoryMode represents the gameplay mode for Memory game
type MemoryMode string

const (
	MemoryModeSolo          MemoryMode = "SOLO"
	MemoryModeChacunSonTour MemoryMode = "CHACUN_SON_TOUR"
	MemoryModeTantQueJeGagne MemoryMode = "TANT_QUE_JE_GAGNE"
)

// MotionCard represents one card in a MEMOTION grid (3 faces: RECTO, VERSO, REVEAL)
type MotionCard struct {
	ID            string `json:"ID"`                        // Unique identifier (e.g. "mc-1")
	RectoTheme    string `json:"RECTO_THEME"`               // Theme/title shown on front face
	RectoImage    string `json:"RECTO_IMAGE,omitempty"`     // Optional image on front face (data/files/ path)
	Difficulty    int    `json:"DIFFICULTY"`                // 1 | 2 | 3 → 1pt | 3pt | 5pt
	QuestionText  string `json:"QUESTION_TEXT,omitempty"`   // Question text (VERSO face)
	QuestionImage string `json:"QUESTION_IMAGE,omitempty"`  // Optional question image
	AnswerText    string `json:"ANSWER_TEXT,omitempty"`     // Answer text (REVEAL face)
	AnswerImage   string `json:"ANSWER_IMAGE,omitempty"`    // Optional answer image
}

// MemoryConfig holds configuration for the Memory game
type MemoryConfig struct {
	FlipDelay          float64 `json:"FLIP_DELAY"`           // seconds before card flips back (default: 3)
	PointsPerPair      int     `json:"POINTS_PER_PAIR"`      // points per found pair (default: 10)
	ErrorPenalty       int     `json:"ERROR_PENALTY"`        // penalty per error (default: 0)
	CompletionBonus    int     `json:"COMPLETION_BONUS"`     // bonus if all pairs found (default: 0)
	UseTimer           bool    `json:"USE_TIMER"`            // true = global timer, false = unlimited
	MemorizeTime       int     `json:"MEMORIZE_TIME"`        // seconds for memorization countdown (default: 5)
	ShowDuringMemorize bool    `json:"SHOW_DURING_MEMORIZE"` // show cards during memorization countdown (default: true)
	RevealDelay        float64 `json:"REVEAL_DELAY"`         // seconds between each pair reveal at end (default: 0.5)
}

// MotionConfig holds configuration for the MEMOTION game
type MotionConfig struct {
	Points1Star int `json:"POINTS_1_STAR"` // points for 1-star cards (default: 1)
	Points2Star int `json:"POINTS_2_STAR"` // points for 2-star cards (default: 3)
	Points3Star int `json:"POINTS_3_STAR"` // points for 3-star cards (default: 5)
}

// Question represents a quiz question
type Question struct {
	ID           string           `json:"ID"`
	Question     string           `json:"QUESTION"`
	Answer       string           `json:"ANSWER"`                  // For normal questions
	Type         QuestionType     `json:"TYPE,omitempty"`          // "NORMAL", "QCM", or "MEMORY" (default NORMAL)
	Category     QuestionCategory `json:"CATEGORY,omitempty"`      // Question category
	PointsTarget PointsTarget     `json:"POINTS_TARGET,omitempty"` // "PLAYER" or "TEAM" (default based on type)
	QCMAnswers        *QCMAnswers      `json:"QCM_ANSWERS,omitempty"`        // For QCM questions
	QCMCorrect        string           `json:"QCM_CORRECT,omitempty"`        // "RED", "GREEN", "YELLOW", "BLUE"
	QCMHintsEnabled   bool             `json:"QCM_HINTS_ENABLED,omitempty"`  // Enable automatic hint invalidation
	QCMHintThreshold1 float64          `json:"QCM_HINT_THRESHOLD_1,omitempty"` // First hint at this % of time remaining (default 0.25)
	QCMHintThreshold2 float64          `json:"QCM_HINT_THRESHOLD_2,omitempty"` // Second hint at this % of time remaining (default 0.125)
	QCMPenalty1       float64          `json:"QCM_PENALTY_1,omitempty"`        // Point multiplier after 1 hint (default 0.67)
	QCMPenalty2       float64          `json:"QCM_PENALTY_2,omitempty"`        // Point multiplier after 2 hints (default 0.33)
	MemoryPairs       []MemoryPair     `json:"MEMORY_PAIRS,omitempty"`       // For Memory questions
	MemoryConfig      *MemoryConfig    `json:"MEMORY_CONFIG,omitempty"`      // Memory game configuration
	MemoryMode        string           `json:"MEMORY_MODE,omitempty"`        // "SOLO", "CHACUN_SON_TOUR", "TANT_QUE_JE_GAGNE" (default SOLO)
	MotionCards       []MotionCard     `json:"MOTION_CARDS,omitempty"`       // Cards for MEMOTION questions (v5.0.0)
	MotionMode        string           `json:"MOTION_MODE,omitempty"`        // "SOLO", "CHACUN_SON_TOUR", "TANT_QUE_JE_GAGNE" (default SOLO)
	MotionConfig           *MotionConfig    `json:"MOTION_CONFIG,omitempty"`            // MEMOTION configuration (v5.0.x)
	MotionMemorizeDuration int              `json:"MOTION_MEMORIZE_DURATION,omitempty"` // Seconds for MEMORIZE phase; 0 = standard mode (v5.5.0)
	// ARDOISE fields (v5.6.0)
	ArdoiseKeyboardType KeyboardType `json:"ARDOISE_KEYBOARD_TYPE,omitempty"` // Virtual keyboard layout: "AZERTY" | "NUMPAD"
	Points                 string           `json:"POINTS"`                             // String to match JSON format
	Time         string           `json:"TIME"`                    // String to match JSON format
	Order        int              `json:"ORDER,omitempty"`         // Display order (for drag and drop)
	Media        string           `json:"MEDIA,omitempty"`         // Question media (shown during game)
	MediaAnswer  string           `json:"MEDIA_ANSWER,omitempty"`  // Answer media (shown during REVEAL)
	Status       QuestionStatus   `json:"STATUS,omitempty"`
}

// Background represents a background image with its settings
type Background struct {
	Path     string  `json:"path"`
	Duration int     `json:"duration"` // Duration in seconds (default 10)
	Opacity  float64 `json:"opacity"`  // Opacity 0-100 (default 100)
}

// GameState holds the current game state
type GameState struct {
	Phase                  GamePhase    `json:"PHASE"`
	Delay                  int          `json:"DELAY"`
	CurrentTime            int          `json:"CURRENT_TIME"`
	CountdownTime          int          `json:"COUNTDOWN_TIME,omitempty"` // 3, 2, 1 countdown before start
	GameTime               int64        `json:"TIME,omitempty"`
	Question               *Question    `json:"QUESTION,omitempty"`
	Page                   string       `json:"REMOTE,omitempty"`
	Backgrounds            []Background `json:"backgrounds,omitempty"`
	CurrentBackgroundIndex int          `json:"CURRENT_BACKGROUND_INDEX"`       // Server-synchronized background index
	NewGameBackgrounds     []Background `json:"new_game_backgrounds,omitempty"` // NEW_GAME screen backgrounds (v4.0.4)
	// Memory fields - NO omitempty so empty arrays are serialized for frontend reset
	MemoryFlippedCards       []string       `json:"MEMORY_FLIPPED_CARDS"`                  // IDs of currently flipped Memory cards (max 2)
	MemoryMatchedPairs       []int          `json:"MEMORY_MATCHED_PAIRS"`                  // IDs of matched pairs (permanent)
	MemoryErrors             int            `json:"MEMORY_ERRORS"`                         // Number of failed match attempts
	MemoryCurrentTeam        string         `json:"MEMORY_CURRENT_TEAM"`                   // Team currently playing (multi-team modes)
	MemoryTeamPairs          map[string]int `json:"MEMORY_TEAM_PAIRS"`                     // Pairs found per team
	MemoryTeamErrors         map[string]int `json:"MEMORY_TEAM_ERRORS"`                    // Errors per team (teamName → errorCount)
	MemoryParticipatingTeams []string       `json:"MEMORY_PARTICIPATING_TEAMS"`            // Teams selected to play
	MemoryPairOwners         map[int]string `json:"MEMORY_PAIR_OWNERS"`                    // pairID → teamName (tracks which team found each pair)
	MemoryCurrentTeamColor   []int          `json:"MEMORY_CURRENT_TEAM_COLOR"`             // RGB color of current team
	QcmInvalidated           []string       `json:"QCM_INVALIDATED"`                       // Invalidated QCM answers (e.g., ["RED", "YELLOW"])
	// MEMOTION fields — NO omitempty: maps/slices/strings must be serialized even when empty for frontend reset (v5.0.0)
	MotionSubPhase          string            `json:"MEMOTION_SUBPHASE"`            // "GRID" | "SELECTED" | "QUESTION" | "REVEAL" | ""
	MotionSelected          string            `json:"MEMOTION_SELECTED"`            // ID of active card, "" when on grid
	MotionCardStates        map[string]string `json:"MEMOTION_CARD_STATES"`         // cardID → "UNPLAYED"|"SELECTED"|"QUESTION"|"REVEALED"|"DONE"
	MotionCardTeams         map[string]string `json:"MEMOTION_CARD_TEAMS"`          // cardID → teamName (winner)
	MotionCurrentTeam       string            `json:"MEMOTION_CURRENT_TEAM"`        // Team currently playing
	MotionParticipatingTeams []string         `json:"MEMOTION_PARTICIPATING_TEAMS"` // Teams selected to play
	MotionCurrentTeamColor  []int             `json:"MEMOTION_CURRENT_TEAM_COLOR"`  // RGB color of current team
	VirtualPlayerCount     int          `json:"VIRTUAL_PLAYER_COUNT"`           // Number of enrolled virtual players
	VirtualPlayerLimit     int          `json:"VIRTUAL_PLAYER_LIMIT"`           // Maximum number of virtual players allowed
	EnrollmentActive       bool         `json:"ENROLLMENT_ACTIVE"`              // Whether player enrollment is active
	ShowQRCode             bool         `json:"SHOW_QR_CODE"`                   // Whether to display QR code on TV
	// Network state (v5.6.2) — NO omitempty: always serialized so frontend receives updates
	NetworkOnlyLocalhost bool `json:"NETWORK_ONLY_LOCALHOST"`
	// ARDOISE answers (v5.6.0) — NO omitempty: always serialized so frontend resets on new question
	ArdoiseAnswers map[string]ArdoiseAnswer `json:"ARDOISE_ANSWERS"`
	// Quiz metadata (v4.0.0) — no omitempty so empty strings clear the field on clients
	QuizName  string `json:"QUIZ_NAME"`
	QuizTheme string `json:"QUIZ_THEME"`
	QuizNotes string `json:"QUIZ_NOTES"`
}

// TeamsAndBumpers holds all teams and bumpers data
type TeamsAndBumpers struct {
	Teams   map[string]*Team   `json:"teams"`
	Bumpers map[string]*Bumper `json:"bumpers"`
}

// NewTeamsAndBumpers creates a new empty structure
func NewTeamsAndBumpers() *TeamsAndBumpers {
	return &TeamsAndBumpers{
		Teams:   make(map[string]*Team),
		Bumpers: make(map[string]*Bumper),
	}
}

// GameData combines game state with teams/bumpers for messages
type GameData struct {
	Game             *GameState                `json:"GAME,omitempty"`
	Teams            map[string]*Team          `json:"teams"`
	Bumpers          map[string]*Bumper        `json:"bumpers"`
	QuestionStatuses map[string]QuestionStatus `json:"-"` // Not serialized, internal tracking
}

// ToJSON serializes the game data
func (g *GameData) ToJSON() (json.RawMessage, error) {
	return json.Marshal(g)
}

// FullGameState combines everything for UPDATE messages
type FullGameState struct {
	GameState
	Teams   map[string]*Team   `json:"teams"`
	Bumpers map[string]*Bumper `json:"bumpers"`
}

// ToJSON serializes the full state
func (f *FullGameState) ToJSON() (json.RawMessage, error) {
	return json.Marshal(f)
}

// LogLevel represents the severity of a log entry
type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
)

// LogComponent represents the source component of a log entry
type LogComponent string

const (
	LogComponentEngine    LogComponent = "Engine"
	LogComponentHTTP      LogComponent = "HTTP"
	LogComponentWebSocket LogComponent = "WebSocket"
	LogComponentTCP       LogComponent = "TCP"
	LogComponentUDP       LogComponent = "UDP"
	LogComponentApp       LogComponent = "App"
	LogComponentUpdater   LogComponent = "Updater"
)

// LogEntry represents a log entry for the logs page
type LogEntry struct {
	Timestamp int64        `json:"timestamp"` // Unix milliseconds
	Level     LogLevel     `json:"level"`     // DEBUG, INFO, WARN, ERROR
	Component LogComponent `json:"component"` // Engine, HTTP, WebSocket, TCP, UDP, App
	Message   string       `json:"message"`
}

// GameEvent represents a game event for history tracking
type GameEvent struct {
	Timestamp        int64  `json:"TIMESTAMP"`                   // Server timestamp in microseconds
	QuestionID       string `json:"QUESTION_ID"`                 // Question ID
	QuestionText     string `json:"QUESTION_TEXT"`               // Question text for display
	QuestionCategory string `json:"QUESTION_CATEGORY,omitempty"` // Question category (GEOGRAPHY, etc.)
	EventType        string `json:"EVENT_TYPE"`                  // "POINTS_AWARDED", "BUZZ", etc.
	WinnerID         string `json:"WINNER_ID"`                   // MAC bumper or team name
	WinnerName       string `json:"WINNER_NAME"`                 // Display name (player or team)
	WinnerType       string `json:"WINNER_TYPE"`                 // "PLAYER" or "TEAM"
	TeamName         string `json:"TEAM_NAME,omitempty"`         // Team name (always filled)
	TeamColor        []int  `json:"TEAM_COLOR,omitempty"`        // Team RGB color
	PlayerName       string `json:"PLAYER_NAME,omitempty"`       // Player name (only if PLAYER)
	PlayerColor      string `json:"PLAYER_COLOR,omitempty"`      // Player answer color (RED/GREEN/YELLOW/BLUE)
	Points           int    `json:"POINTS"`                      // Points awarded
	ReactionTime     int64  `json:"REACTION_TIME,omitempty"`     // Reaction time in microseconds
}
