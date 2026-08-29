package game

import (
	"encoding/json"
	"time"
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
	PhaseEnroll    GamePhase = "ENROLL"   // Player enrollment phase (virtual players)
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
	TeamPoints int    `json:"TEAM_POINTS"`          // Independent team points
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
	// Connection badge state machine (added in v5.7.13, #109 Phase 1) — NO omitempty:
	// "" (hidden) must always be serialized so the frontend never falls back to a stale value.
	// Driven by engine.TransitionConn; see ConnState* constants and contracts/websocket-actions.md.
	ConnState string `json:"CONN_STATE"` // "" (hidden) | "orange" | "red" | "green"

	// ReclaimRequested (added in v5.9.x, #122) — NO omitempty: false is a
	// meaningful value the frontend relies on to clear a stale "place demandée"
	// card. Set on this bumper (the current name holder) when a PLAYER_CONNECT
	// for its name is rejected with NAME_TAKEN_OFFLINE (engine.
	// ReconnectOrCreateVirtualPlayer, case 3) — i.e. someone just failed to
	// reclaim this exact seat. Cleared on a normal ID reconnection (case 1),
	// when the animateur grants a reclaim authorization (ReleaseBumperName),
	// or implicitly when the bumper itself is deleted (the whole struct goes
	// with it).
	ReclaimRequested bool `json:"RECLAIM_REQUESTED"`

	// --- Internal bookkeeping for the reclaim authorization (#122 B3): an
	// animateur-granted, single-use, time-bounded exception to the #109 R1
	// ID-only identity rule. Unexported: never serialized:
	// reclaimAuthorizedUntil, when non-zero and in the future, means the NEXT
	// nameless PLAYER_CONNECT matching this bumper's name may reattach to it
	// instead of being rejected — consumed (reset to zero) on first use, on
	// expiry, or on a normal ID reconnection in the meantime.
	reclaimAuthorizedUntil time.Time

	// --- Internal bookkeeping for the CONN_STATE "green" minimum display window
	// (D2/D3, added in v5.7.14, #109 Phase 2). Unexported: encoding/json only
	// serializes exported fields, so these never reach the wire regardless of tags.
	greenSince     time.Time // when this bumper last transitioned to "green" (Reconnect)
	confirmPending bool      // a DeliveryConfirmed arrived early; engine.ConfirmDelivery
	//                          scheduled a timer to apply it once the window closes.

	// skipNextMessageLost (added in v5.7.21, #109 conn-state fix): set whenever a
	// Disconnect transition just turned this bumper orange. The very next
	// ApplyVPlayerBroadcastConnEvents evaluation consumes it (sets it back to
	// false) WITHOUT firing MessageLost — the broadcast that announces "this
	// VJoueur just disconnected" is not itself a message it should have
	// received. Any broadcast AFTER that one applies MessageLost normally.
	// Without this, orange -> red happened within the same broadcast that set
	// orange, so orange was never actually visible to the admin.
	skipNextMessageLost bool
}

// ConnState values for Bumper.ConnState — connection badge state machine (v5.7.13, #109).
// Scope: only participants (Team != "") ever carry a non-hidden value; see
// engine.TransitionConn / engine.ConnEvent for the transition table.
const (
	ConnStateHidden = ""       // no connection issue to show (or non-participant / never connected)
	ConnStateOrange = "orange" // disconnected, no message lost yet
	ConnStateRed    = "red"    // disconnected AND at least one message was missed while down
	ConnStateGreen  = "green"  // just reconnected — minimum display window (timer added in Phase 2)
)

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
	QuestionTypeSpeedy   QuestionType = "SPEEDY"
	QuestionTypeQCM      QuestionType = "QCM"
	QuestionTypeMemory   QuestionType = "MEMORY"
	QuestionTypeMemotion QuestionType = "MEMOTION" // NEW (v5.0.0): grid of cards with 3 faces
	QuestionTypeArdoise  QuestionType = "ARDOISE"  // NEW (v5.6.0): free-text answer via virtual keyboard
	QuestionTypeRafale   QuestionType = "RAFALE"   // NEW (v8.0.0, #107): timed round drawing from the RAFALE reservoir — contracts/rafale.md
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
	StartedAt   int64  `json:"STARTED_AT"`   // Timestamp in microseconds of the FIRST non-empty character (frozen, #117)
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
	MemoryModeSolo           MemoryMode = "SOLO"
	MemoryModeChacunSonTour  MemoryMode = "CHACUN_SON_TOUR"
	MemoryModeTantQueJeGagne MemoryMode = "TANT_QUE_JE_GAGNE"
)

// RafaleMode is Question.RafaleMode's value set — contracts/rafale.md §3.4.
// TypedContent.RafaleMode itself stays a plain string (mirrors
// Question.MotionMode), these constants exist for comparison, same idiom as
// MemoryMode above.
type RafaleMode string

const (
	RafaleModeSolo           RafaleMode = "SOLO"              // one team, no rotation ever (#107)
	RafaleModeChacunSonTour  RafaleMode = "CHACUN_SON_TOUR"   // rotates after every question regardless of outcome
	RafaleModeTantQueJeGagne RafaleMode = "TANT_QUE_JE_GAGNE" // rotates only on an incorrect answer/timeout; a correct answer keeps the hand
	RafaleModeMaillonFaible  RafaleMode = "MAILLON_FAIBLE"    // rotates every question like CHACUN_SON_TOUR, but the counter resets to 0 on an incorrect answer (best kept separately, RAFALE_TEAM_BEST)
)

// TypedContent holds the fields that carry a QuestionType's own content —
// QCM answers, MEMORY pairs, ARDOISE keyboard layout, and the plain-text
// ANSWER shared by SPEEDY/ARDOISE. Embedded anonymously (flat, no JSON
// nesting) in both Question and MotionCard so a nested card and a top-level
// question expose the exact same field names on the wire for a given type —
// contracts/question-types.md §2. Field tags are byte-for-byte the ones
// Question already used before this refactor; moving them into a shared
// struct changes nothing about the JSON they produce.
//
// Answer is deliberately `omitempty` here even though Question.Answer
// (declared directly on Question, below) is not: Question keeps its own
// explicit ANSWER field, which — per Go's embedded-field JSON precedence
// (shallower field wins) — always shadows this one for Question, so this
// field never actually fires for the question host; only MotionCard could
// ever populate it. Without omitempty here, every existing MEMOTION card
// (which never carries ANSWER) would gain an "ANSWER":"" key it never had.
//
// ⚠️ v7.1.0 (#187): this field's only prior justification — ARDOISE-in-card
// (#186) — was abandoned before implementation ("not planned", 2026-08-24,
// contract §2's "Mise à jour v7.1.0" note). The `omitempty` tag stays
// anyway: removing it would give every existing MEMOTION card a fresh
// "ANSWER":"" key it never had, breaking the byte-for-byte round-trip of
// the 85 question.json fixtures (models_roundtrip_test.go). It now exists
// purely to preserve that invariant, not in anticipation of a consumer —
// do not "clean it up" because #186 is closed.
type TypedContent struct {
	// SPEEDY (MotionCard only — Question.Answer below is authoritative for
	// the question host). No nestable type actually populates this field in
	// v7.1.0 — see the doc comment above.
	Answer string `json:"ANSWER,omitempty"`
	// QCM
	QCMAnswers        *QCMAnswers `json:"QCM_ANSWERS,omitempty"`
	QCMCorrect        string      `json:"QCM_CORRECT,omitempty"`
	QCMHintsEnabled   bool        `json:"QCM_HINTS_ENABLED,omitempty"`
	QCMHintThreshold1 float64     `json:"QCM_HINT_THRESHOLD_1,omitempty"`
	QCMHintThreshold2 float64     `json:"QCM_HINT_THRESHOLD_2,omitempty"`
	QCMPenalty1       float64     `json:"QCM_PENALTY_1,omitempty"`
	QCMPenalty2       float64     `json:"QCM_PENALTY_2,omitempty"`
	// ARDOISE — question host only. Not nestable in a MEMOTION card: #186
	// closed "not planned" (2026-08-24, contract §7.2) before it shipped.
	ArdoiseKeyboardType KeyboardType `json:"ARDOISE_KEYBOARD_TYPE,omitempty"`
	// MEMORY — question host AND, since #187 (v7.1.0), a MEMOTION card
	// (contract §7.3). MemoryMode is read for the question host only; a
	// MEMORY card ignores it entirely (contract §6.3) — it is always played
	// by exactly one team, the current MEMOTION team, and never rotates on
	// its own.
	MemoryPairs  []MemoryPair  `json:"MEMORY_PAIRS,omitempty"`
	MemoryConfig *MemoryConfig `json:"MEMORY_CONFIG,omitempty"`
	MemoryMode   string        `json:"MEMORY_MODE,omitempty"`
	// RAFALE (v8.0.0, #107, contracts/rafale.md §3.3) — question host only,
	// never nestable in a MEMOTION card (RAFALE has no notion of a single
	// card; it drives a whole timed round). Unlike QCM/MEMORY, a RAFALE
	// "question" carries no statement of its own here — TIME/POINTS
	// (declared directly on Question, reused per §3.3) plus these 4 fields
	// are the round's CONFIGURATION; the actual questions come from the
	// reservoir (RafaleQuestion, rafale_store.go), drawn at play time.
	//
	// Category filter: RAFALE_CATEGORIES (multi-selection, []string) was
	// removed (v8.0.0 bugfix, 2026-08-29) in favor of the generic
	// Question.Category field every other type already uses — RAFALE was
	// the first (and only) type to introduce multi-category selection, and
	// it was never rendered correctly by the generic question card (which
	// reads question.CATEGORY, not a type-specific field) despite a prior
	// attempted fix. Single category, same as SPEEDY/QCM/MEMORY/MEMOTION/
	// ARDOISE, resolves the display bug structurally instead of adding a
	// dedicated RAFALE branch client-side. See contracts/CHANGELOG.md.
	//
	// RafaleDifficulty: omitempty like every other TypedContent field in
	// this struct (models_roundtrip_test.go enforces byte-for-byte fixture
	// round-trips — a foreign type's owned field must never appear on the
	// wire for a question that doesn't carry it). 0 is not a valid
	// difficulty (1..3); an admin-configured RAFALE round always sets it
	// explicitly before START, same as every other required config field
	// on this struct.
	RafaleDifficulty   int    `json:"RAFALE_DIFFICULTY,omitempty"`
	RafaleMode         string `json:"RAFALE_MODE,omitempty"`          // SOLO | CHACUN_SON_TOUR | TANT_QUE_JE_GAGNE | MAILLON_FAIBLE (contract §3.4)
	RafaleQuestionTime int    `json:"RAFALE_QUESTION_TIME,omitempty"` // seconds per question, default 3 — NOT GameState.RafaleQuestionTime (the live countdown; same JSON key, different struct, contract §4 note)
	RafaleMaxQuestions int    `json:"RAFALE_MAX_QUESTIONS,omitempty"` // hard cap, default 100, max 100
}

// MotionCard represents one card in a MEMOTION grid (3 faces: RECTO, VERSO, REVEAL)
type MotionCard struct {
	ID string `json:"ID"` // Unique identifier (e.g. "mc-1")
	// Type is the card's own game type — absent/"" means SPEEDY (#184,
	// contract §3). Validated at registration (internal/game/question_types.go
	// registry: must be known and NestableInMotionCard) and at
	// SelectMotionCard (engine.go). Never MEMOTION — nesting depth is
	// capped at 1 (contract §1).
	Type          QuestionType `json:"TYPE,omitempty"`
	RectoTheme    string       `json:"RECTO_THEME"`              // Theme/title shown on front face
	RectoImage    string       `json:"RECTO_IMAGE,omitempty"`    // Optional image on front face (data/files/ path)
	Difficulty    int          `json:"DIFFICULTY"`               // 1 | 2 | 3 → 1pt | 3pt | 5pt
	QuestionText  string       `json:"QUESTION_TEXT,omitempty"`  // Question text (VERSO face)
	QuestionImage string       `json:"QUESTION_IMAGE,omitempty"` // Optional question image
	AnswerText    string       `json:"ANSWER_TEXT,omitempty"`    // Answer text (REVEAL face)
	AnswerImage   string       `json:"ANSWER_IMAGE,omitempty"`   // Optional answer image
	// PointsRule is this card's own points-award rule (#184, B-B5, contract
	// §6.2) — absent ⇒ STARS, the pre-#184 star-based scale, unchanged.
	PointsRule *PointsRule `json:"POINTS_RULE,omitempty"`

	TypedContent // embedded flat — QCM_*/MEMORY_*/ARDOISE_KEYBOARD_TYPE/ANSWER for a typed card (#184, contract §2/§3)
}

// EffectiveType returns c.Type, defaulting to SPEEDY when absent — the
// retro-compatible reading every consumer of MotionCard.Type must use
// instead of comparing the raw field to "" (contract §3: "absent ou ”
// ⇒ SPEEDY").
func (c *MotionCard) EffectiveType() QuestionType {
	if c.Type == "" {
		return QuestionTypeSpeedy
	}
	return c.Type
}

// PointsRuleMode is PointsRule.Mode — contract §6.2.
type PointsRuleMode string

const (
	// PointsRuleModeStars is the default (absent MODE ⇒ STARS too, see
	// PointsRule's doc comment): the pre-#184 star-based scale
	// (MotionConfig.POINTS_<n>_STAR if >0, else DIFFICULTY→1/3/5),
	// unchanged — engine.go's motionCardPoints.
	PointsRuleModeStars PointsRuleMode = "STARS"
	// PointsRuleModeFixed awards VALUE if the type's outcome reports
	// Units > 0, else 0 — a card whose value doesn't depend on difficulty.
	PointsRuleModeFixed PointsRuleMode = "FIXED"
	// PointsRuleModePerUnit awards VALUE × Units — a progression type with a
	// static, editor-set value per unit.
	PointsRuleModePerUnit PointsRuleMode = "PER_UNIT"
	// PointsRuleModeStarsProrata awards the card's own star-scale points
	// (motionCardPoints, same source as the STARS default) prorated by
	// Units/UnitsTotal — no VALUE. Default POINTS_RULE of a MEMORY card
	// (#187, contract §6.2/§6.3): the card is worth its stars like any
	// other, and the type only ever distributes a fraction of them — no
	// points setting independent of the star scale is introduced. VALUE is
	// ignored under this mode.
	PointsRuleModeStarsProrata PointsRuleMode = "STARS_PRORATA"
)

// PointsRule is a MotionCard's own points-award rule — contract §6.2. The
// scoring authority always belongs to the host (never to the nested type,
// which only ever reports a TypeOutcome), so this lives on MotionCard, not
// on TypedContent. Absent (nil) ⇒ STARS with the current-behavior star
// scale, same as an explicit {"MODE":"STARS"}.
type PointsRule struct {
	Mode  PointsRuleMode `json:"MODE,omitempty"`
	Value int            `json:"VALUE,omitempty"`
}

// TypeOutcome is what a nested type implementation reports back to its
// host after a round — contract §6.1: "un type ne rend qu'un résultat".
// Units defaults to 1 for any binary (won/lost) type; a progression type
// reports its own count AND its own denominator in UnitsTotal (#187, MEMORY
// prorata, contract §6.1) — the host divides but must never itself know
// what a "unit" is (e.g. a MEMORY pair). motionCardPointsForOutcome takes
// (units, unitsTotal) as plain ints rather than a literal value of this
// struct, but the data shape is the one documented here. Not yet produced
// anywhere as a literal Go value — #185 (QCM-in-card) is designated via the
// existing MEMOTION_DONE action exactly like SPEEDY, not through this
// struct; documented here because #187 must be able to assume it's part of
// the core contract, and adding it after the fact would reopen engine.go
// (forbidden by #184's agnosticity test, contract §10).
type TypeOutcome struct {
	WinnerTeam string // "" = nobody
	Units      int    // realised — 1 = won, 0 = lost for a binary type
	UnitsTotal int    // realisable maximum — 0 for a binary type (#187)
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

// MotionSubPhase represents the current sub-phase of a MEMOTION round
// (GameState.MotionSubPhase, "" outside a MEMOTION question).
type MotionSubPhase string

const (
	MotionSubPhaseMemorize MotionSubPhase = "MEMORIZE" // Secret Mode countdown before the grid is shown
	MotionSubPhaseGrid     MotionSubPhase = "GRID"     // Grid of cards, none selected
	MotionSubPhaseSelected MotionSubPhase = "SELECTED" // One card selected, RECTO face shown fullscreen
	MotionSubPhaseQuestion MotionSubPhase = "QUESTION" // Selected card flipped, VERSO face + timer
	MotionSubPhaseReveal   MotionSubPhase = "REVEAL"   // Selected card's answer (REVEAL face) shown
)

// MotionCardState represents the state of one card in GameState.MotionCardStates.
type MotionCardState string

const (
	MotionCardStateUnplayed MotionCardState = "UNPLAYED" // not yet selected this question
	MotionCardStateSelected MotionCardState = "SELECTED" // currently selected, RECTO shown
	MotionCardStateQuestion MotionCardState = "QUESTION" // currently flipped, VERSO shown
	MotionCardStateRevealed MotionCardState = "REVEALED" // currently revealed, REVEAL shown
	MotionCardStateDone     MotionCardState = "DONE"     // played, DoneMotionCard called
)

// AllQuestionTypes returns the full registry of question types, i.e. the 6
// values QuestionType may take. Exported as a test-exhaustiveness helper —
// see contracts/question-types.md §10 (test d'agnosticité) — not consumed by
// production code in v7.0.0.
func AllQuestionTypes() []QuestionType {
	return []QuestionType{
		QuestionTypeSpeedy,
		QuestionTypeQCM,
		QuestionTypeMemory,
		QuestionTypeMemotion,
		QuestionTypeArdoise,
		QuestionTypeRafale,
	}
}

// RafaleQuestion is one question in the RAFALE reservoir — a global pool of
// text-only question/answer pairs, independent from Quiz/MEMOTION questions
// (contracts/rafale.md §2.4/§3.1, milestone #16, #197). Stored as a single
// file (data/files/rafale/reservoir.json), never one directory per question:
// RAFALE questions carry no media (arbitrage D3).
type RafaleQuestion struct {
	ID         string           `json:"ID"`         // stable identifier, unique within the reservoir
	Question   string           `json:"QUESTION"`   // statement, text only
	Answer     string           `json:"ANSWER"`     // expected answer, text only
	Category   QuestionCategory `json:"CATEGORY"`   // reuses the existing enum + custom categories on disk
	Difficulty int              `json:"DIFFICULTY"` // 1..3 — same scale as MotionCard.Difficulty
}

// RafaleSubPhase is the sub-phase of a RAFALE round (GameState.RafaleSubPhase,
// "" outside a RAFALE question) — contracts/rafale.md §4. Defined here
// (models) ahead of the GameState fields that will carry it (#107, Phase 2)
// so the reservoir/editor work (#197) doesn't need to wait on the solo
// engine.
type RafaleSubPhase string

const (
	RafaleSubPhaseNone     RafaleSubPhase = ""          // inactive
	RafaleSubPhaseQuestion RafaleSubPhase = "QUESTION"  // question asked, question timer running
	RafaleSubPhaseRoundEnd RafaleSubPhase = "ROUND_END" // round over, points attribution pending
)

// RafaleCurrent is the question currently asked during a RAFALE round,
// WITHOUT the expected answer (contracts/rafale.md §2.3: the answer never
// transits through GameState — it is broadcast separately via the dedicated
// RAFALE_ANSWER action, to admin+anim only). See RafaleSubPhase's doc
// comment for why this is defined ahead of #107.
type RafaleCurrent struct {
	ID         string `json:"ID"`
	Question   string `json:"QUESTION"`
	Category   string `json:"CATEGORY"`
	Difficulty int    `json:"DIFFICULTY"`
}

// Question represents a quiz question
//
// Answer is declared here explicitly (not left to the embedded TypedContent
// below) because it has no `omitempty`: 26/85 existing question.json persist
// "ANSWER":"" and the round-trip test (models_roundtrip_test.go) requires
// that key to survive even when empty. Go's JSON field-precedence rule
// (shallowest field wins when names collide across an embedding boundary)
// makes this the field that's actually serialized/deserialized for Question;
// TypedContent.Answer never fires here — see TypedContent's doc comment.
type Question struct {
	ID                     string           `json:"ID"`
	Question               string           `json:"QUESTION"`
	Answer                 string           `json:"ANSWER"`                             // For normal questions
	Type                   QuestionType     `json:"TYPE,omitempty"`                     // "SPEEDY", "QCM", "MEMORY", "MEMOTION", "ARDOISE" (default SPEEDY)
	Category               QuestionCategory `json:"CATEGORY,omitempty"`                 // Question category
	PointsTarget           PointsTarget     `json:"POINTS_TARGET,omitempty"`            // "PLAYER" or "TEAM" (default based on type)
	MotionCards            []MotionCard     `json:"MOTION_CARDS,omitempty"`             // Cards for MEMOTION questions (v5.0.0)
	MotionMode             string           `json:"MOTION_MODE,omitempty"`              // "SOLO", "CHACUN_SON_TOUR", "TANT_QUE_JE_GAGNE" (default SOLO)
	MotionConfig           *MotionConfig    `json:"MOTION_CONFIG,omitempty"`            // MEMOTION configuration (v5.0.x)
	MotionMemorizeDuration int              `json:"MOTION_MEMORIZE_DURATION,omitempty"` // Seconds for MEMORIZE phase; 0 = standard mode (v5.5.0)
	Points                 string           `json:"POINTS"`                             // String to match JSON format
	Time                   string           `json:"TIME"`                               // String to match JSON format
	Order                  int              `json:"ORDER,omitempty"`                    // Display order (for drag and drop)
	Media                  string           `json:"MEDIA,omitempty"`                    // Question media (shown during game)
	MediaAnswer            string           `json:"MEDIA_ANSWER,omitempty"`             // Answer media (shown during REVEAL)
	Explanation            string           `json:"EXPLANATION,omitempty"`              // note animateur (v6.4.x, #168) — visible /anim only, never TV/player/admin
	Status                 QuestionStatus   `json:"STATUS,omitempty"`

	TypedContent // embedded flat — QCM_*/MEMORY_*/ARDOISE_KEYBOARD_TYPE (Answer shadowed by the explicit field above)
}

// MotionActive holds the identity and live state of the single active
// MEMOTION card — contract §5. There is never more than one card in play
// at once, so this is a single slot, not a map keyed by card ID (contract
// §5.1: bounds the GAME payload to the cost of one type-state, not N).
//
// Reset to its zero value at every MEMOTION_SELECT (CardID/Type set,
// State starts empty) and emptied back to the zero value on return to
// GRID (engine.go: DoneMotionCard, the memorize-timer's auto-expiry, and
// Ready()/InitGame()'s full MEMOTION reset).
//
// NO omitempty on the GameState field below and NO omitempty on these
// fields — project rule (CLAUDE.md, contract §5.2): always serialized,
// including empty ({"CARD_ID":"","TYPE":"","STATE":{}}), so the frontend
// never falls back to a stale value. NOT persisted — excluded in
// state_persistence.go alongside the other Motion* fields.
type MotionActive struct {
	CardID string                 `json:"CARD_ID"`
	Type   QuestionType           `json:"TYPE"`
	State  map[string]interface{} `json:"STATE"`
}

// Background represents a background image with its settings
type Background struct {
	Path     string  `json:"path"`
	Duration int     `json:"duration"` // Duration in seconds (default 10)
	Opacity  float64 `json:"opacity"`  // Opacity 0-100 (default 100)
}

// EntracteConfig is the panel configuration for ENTRACTE mode (v6.5.2,
// #119). A property of the GAME/session, not of server config (arbitrage
// utilisateur 2026-08-20, C1 — amends the original design, which lived in
// game-config.json/GameSettings): persisted in game_state.json alongside
// the Quiz* fields (see PersistedGameState, state_persistence.go), edited
// from the Quiz page via UPDATE_ENTRACTE_CONFIG (contract game-state.md
// §"ENTRACTE_CONFIG", websocket-actions.md §"UPDATE_ENTRACTE_CONFIG").
//
// ImageIsCustom is ALWAYS derived at read time from whether a custom image
// file exists on disk (data/files/entracte/) — never itself persisted (see
// state_persistence.go, SaveState/LoadState zero it explicitly) and never
// part of the UPDATE_ENTRACTE_CONFIG payload. No file path ever crosses the
// wire; the client builds the stable /api/game/entracte-image URL itself
// with a cache-buster.
type EntracteConfig struct {
	Title         string `json:"TITLE"`
	Subtitle      string `json:"SUBTITLE"`
	ImageIsCustom bool   `json:"IMAGE_IS_CUSTOM"`
	PanelSize     int    `json:"PANEL_SIZE"`     // % of screen, width AND height, same on /tv and /player. Clamped 20-100.
	AnimPeriod    int    `json:"ANIM_PERIOD"`    // Animation cycle duration, seconds. Clamped 2-30.
	AnimIntensity int    `json:"ANIM_INTENSITY"` // Animation amplitude, 0-100. 0 = animation disabled.
	// TransitionMs (v6.5.2, #119, C3) — fade duration in/out of entracte, on
	// all 4 surfaces, milliseconds. Default 2000, clamped 0-10000. 0 =
	// instant switch (the pre-C3 behavior). Unlike the animation fields
	// above, prefers-reduced-motion does NOT neutralize this one — a fade
	// is not the kind of motion that setting targets (contract game-state.md
	// §"Animation du panneau").
	TransitionMs int `json:"TRANSITION_MS"`
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
	MemoryFlippedCards       []string       `json:"MEMORY_FLIPPED_CARDS"`       // IDs of currently flipped Memory cards (max 2)
	MemoryMatchedPairs       []int          `json:"MEMORY_MATCHED_PAIRS"`       // IDs of matched pairs (permanent)
	MemoryErrors             int            `json:"MEMORY_ERRORS"`              // Number of failed match attempts
	MemoryCurrentTeam        string         `json:"MEMORY_CURRENT_TEAM"`        // Team currently playing (multi-team modes)
	MemoryTeamPairs          map[string]int `json:"MEMORY_TEAM_PAIRS"`          // Pairs found per team
	MemoryTeamErrors         map[string]int `json:"MEMORY_TEAM_ERRORS"`         // Errors per team (teamName → errorCount)
	MemoryParticipatingTeams []string       `json:"MEMORY_PARTICIPATING_TEAMS"` // Teams selected to play
	MemoryPairOwners         map[int]string `json:"MEMORY_PAIR_OWNERS"`         // pairID → teamName (tracks which team found each pair)
	MemoryCurrentTeamColor   []int          `json:"MEMORY_CURRENT_TEAM_COLOR"`  // RGB color of current team
	QcmInvalidated           []string       `json:"QCM_INVALIDATED"`            // Invalidated QCM answers (e.g., ["RED", "YELLOW"])
	// MEMOTION fields — NO omitempty: maps/slices/strings must be serialized even when empty for frontend reset (v5.0.0)
	MotionSubPhase           MotionSubPhase             `json:"MEMOTION_SUBPHASE"`            // MotionSubPhaseMemorize | Grid | Selected | Question | Reveal | ""
	MotionSelected           string                     `json:"MEMOTION_SELECTED"`            // ID of active card, "" when on grid
	MotionCardStates         map[string]MotionCardState `json:"MEMOTION_CARD_STATES"`         // cardID → MotionCardStateUnplayed|Selected|Question|Revealed|Done
	MotionCardTeams          map[string]string          `json:"MEMOTION_CARD_TEAMS"`          // cardID → teamName (winner)
	MotionCurrentTeam        string                     `json:"MEMOTION_CURRENT_TEAM"`        // Team currently playing
	MotionParticipatingTeams []string                   `json:"MEMOTION_PARTICIPATING_TEAMS"` // Teams selected to play
	MotionCurrentTeamColor   []int                      `json:"MEMOTION_CURRENT_TEAM_COLOR"`  // RGB color of current team
	// MotionActive (#184, B-B4) — the single active card's identity + typed
	// live state, contract §5. NOT persisted (state_persistence.go excludes
	// it alongside the other Motion* fields).
	MotionActive       MotionActive `json:"MEMOTION_ACTIVE"`
	VirtualPlayerCount int          `json:"VIRTUAL_PLAYER_COUNT"` // Number of enrolled virtual players
	VirtualPlayerLimit int          `json:"VIRTUAL_PLAYER_LIMIT"` // Maximum number of virtual players allowed
	EnrollmentActive   bool         `json:"ENROLLMENT_ACTIVE"`    // Whether player enrollment is active
	ShowQRCode         bool         `json:"SHOW_QR_CODE"`         // Whether to display QR code on TV
	// ENTRACTE (v6.5.2, #119) — global pause mode, independent of the question
	// cycle. Same nature as ShowQRCode just above: an ephemeral display flag,
	// NOT persisted (state_persistence.go excludes it explicitly, alongside
	// ShowQRCode). NO omitempty on Entracte: unlike Backgrounds/
	// NewGameBackgrounds (which precede the project's no-omitempty rule and
	// are not a model to copy), `false` MUST stay on the wire or no client
	// could ever learn the pause has ENDED (contract game-state.md
	// §"ENTRACTE"). EntracteConfig delivers title/subtitle/panel size/
	// animation/transition atomically in the same UPDATE as the flag itself
	// to every surface that needs it, including VJoueur (which cannot
	// receive CONFIG_UPDATE, restricted to Admin+TV since #154) — contract
	// game-state.md §"Diffusion". NO omitempty either: ANIM_INTENSITY=0 is a
	// meaningful value (animation disabled), not an absent one.
	//
	// C4 (2026-08-20, arbitrage) — TWO objects of the same type coexist,
	// deliberately, and this is NOT duplication to "simplify" away:
	//   - EntracteConfig    — the DIFFUSED config the panel actually shows.
	//     FROZEN while Entracte is true (SetEntracteConfig only refreshes it
	//     when !Entracte; SetEntracte(true) recopies Saved -> this field,
	//     under the same lock, right before raising the flag).
	//   - EntracteConfigSaved — the SAVED config, always current, edited from
	//     the Quiz page. Admin-only (protocol.AdminOnlyGameFields, alongside
	//     QUIZ_OBJECTIVES) — TV/VJoueur only ever need the panel they see.
	// Editing settings mid-pause must persist and be visible to the editor
	// (Saved) without changing what's already on screen (Config) — it takes
	// effect at the NEXT entracte. Without this split, an admin editing
	// during a live pause, leaving the Quiz page and coming back, would see
	// their just-saved values vanish (overwritten by the frozen diffused
	// copy) and believe the save was lost.
	Entracte            bool           `json:"ENTRACTE"`
	EntracteConfig      EntracteConfig `json:"ENTRACTE_CONFIG"`
	EntracteConfigSaved EntracteConfig `json:"ENTRACTE_CONFIG_SAVED"`
	// Network state (v5.6.2) — NO omitempty: always serialized so frontend receives updates
	NetworkOnlyLocalhost bool `json:"NETWORK_ONLY_LOCALHOST"`
	// ARDOISE answers (v5.6.0) — NO omitempty: always serialized so frontend resets on new question
	ArdoiseAnswers map[string]ArdoiseAnswer `json:"ARDOISE_ANSWERS"`
	// Quiz metadata (v4.0.0) — no omitempty so empty strings clear the field on clients
	QuizName  string `json:"QUIZ_NAME"`
	QuizTheme string `json:"QUIZ_THEME"`
	QuizNotes string `json:"QUIZ_NOTES"`
	// Quiz metadata for AI generation prefill (v6.0.0, #8) — no omitempty for the
	// same reason as the three fields above: a client must be able to see a
	// field cleared back to "" (contract game-state.md, ai-generation.md §6).
	// v6.1.0 (#137 Batch 2b): QuizPopulation/QuizDifficulty (string) became
	// QuizPopulations/QuizDifficulties ([]string) — a game can target several
	// audiences/difficulties at once. Serialized as [] never null: MUST be
	// initialized to []string{} (never left nil) everywhere they're set, so a
	// client iterating the array never crashes on null (contract game-state.md
	// §"Aucun omitempty"). QuizObjectives is new — the game's global objective,
	// admin-only, never broadcast to /ws/tv or /ws/player (see
	// protocol.AdminOnlyGameFields and contracts/game-state.md
	// §"QUIZ_OBJECTIVES — champ à diffusion restreinte").
	QuizPopulations  []string `json:"QUIZ_POPULATIONS"`
	QuizDifficulties []string `json:"QUIZ_DIFFICULTIES"`
	QuizLanguage     string   `json:"QUIZ_LANGUAGE"`
	QuizObjectives   string   `json:"QUIZ_OBJECTIVES"`
	// QuizHiddenFields (v6.1.0, #137 Batch 2b T1.8, additive) — TV NEW_GAME
	// display preference, not confidentiality: values ⊂ {THEME, POPULATIONS,
	// DIFFICULTIES, LANGUAGE} that the admin chose NOT to show on the TV
	// screen. Unlike QuizObjectives, this field IS broadcast to /ws/tv and
	// /ws/player (contract ws-payload-serialization.md) — the client applies
	// the preference, the server does not strip it (contract game-state.md
	// §"Diffusion — préférence d'affichage ≠ confidentialité"). Set only via
	// SetQuizDisplay, not SetQuizMeta (contract H1-H5, rule H1: never null,
	// always initialized to []string{}).
	QuizHiddenFields []string `json:"QUIZ_HIDDEN_FIELDS"`

	// RAFALE fields (v8.0.0, #107, contracts/rafale.md §4) — NO omitempty
	// (project rule): maps/slices/strings must be serialized even when
	// empty/zero so the frontend always resets cleanly between rounds.
	// Ephemeral game-play state, never persisted — excluded from
	// PersistedGameState (state_persistence.go), same precedent as
	// MotionActive. Initialized non-nil in NewEngine() and reset at every
	// NEW_GAME/Ready(), same discipline as the MEMOTION/MEMORY fields
	// above.
	//
	// RafaleCurrentQuestion never carries the expected answer (contract
	// §2.3 — RafaleCurrent, models.go, has no ANSWER field by construction).
	// The answer is broadcast separately via the dedicated RAFALE_ANSWER
	// action, to admin+anim only (protocol.ActionRafaleAnswer).
	//
	// RafaleQuestionTime here is the LIVE countdown for the current
	// question (decremented by the dedicated RAFALE question ticker,
	// diffused via RAFALE_TICK) — NOT TypedContent.RafaleQuestionTime (the
	// per-round configured seconds-per-question default). Same JSON key,
	// deliberately: one is the Question's static config, the other is
	// GameState's live value, exactly like Question.Time vs
	// GameState.CurrentTime for the round's global timer.
	//
	// RafaleTeamBest is only ever populated in MAILLON_FAIBLE mode
	// (contract §3.4/§6.1); present and empty in the other 3 modes.
	RafaleSubPhase           RafaleSubPhase `json:"RAFALE_SUBPHASE"`
	RafaleCurrentQuestion    RafaleCurrent  `json:"RAFALE_CURRENT_QUESTION"`
	RafaleQuestionTime       int            `json:"RAFALE_QUESTION_TIME"`
	RafaleTeamCounters       map[string]int `json:"RAFALE_TEAM_COUNTERS"`
	RafaleTeamBest           map[string]int `json:"RAFALE_TEAM_BEST"`
	RafaleCurrentTeam        string         `json:"RAFALE_CURRENT_TEAM"`
	RafaleParticipatingTeams []string       `json:"RAFALE_PARTICIPATING_TEAMS"`
	RafaleCurrentTeamColor   []int          `json:"RAFALE_CURRENT_TEAM_COLOR"`
	RafaleAskedCount         int            `json:"RAFALE_ASKED_COUNT"`
	RafalePoolRemaining      int            `json:"RAFALE_POOL_REMAINING"`
	RafaleExhausted          bool           `json:"RAFALE_EXHAUSTED"`
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
	Timestamp           int64  `json:"TIMESTAMP"`                    // Server timestamp in microseconds
	QuestionID          string `json:"QUESTION_ID"`                  // Question ID
	QuestionText        string `json:"QUESTION_TEXT"`                // Question text for display
	QuestionCategory    string `json:"QUESTION_CATEGORY,omitempty"`  // Question category key (GEOGRAPHY, etc.)
	CategoryDisplayName string `json:"CATEGORY_NAME,omitempty"`      // Resolved display name (v5.7.9)
	CategoryImageURL    string `json:"CATEGORY_IMAGE_URL,omitempty"` // Resolved image URL (v5.7.9)
	CategoryColor       string `json:"CATEGORY_COLOR,omitempty"`     // Resolved accent color (v5.7.9)
	EventType           string `json:"EVENT_TYPE"`                   // "POINTS_AWARDED", "BUZZ", etc.
	WinnerID            string `json:"WINNER_ID"`                    // MAC bumper or team name
	WinnerName          string `json:"WINNER_NAME"`                  // Display name (player or team)
	WinnerType          string `json:"WINNER_TYPE"`                  // "PLAYER" or "TEAM"
	TeamName            string `json:"TEAM_NAME,omitempty"`          // Team name (always filled)
	TeamColor           []int  `json:"TEAM_COLOR,omitempty"`         // Team RGB color
	PlayerName          string `json:"PLAYER_NAME,omitempty"`        // Player name (only if PLAYER)
	PlayerColor         string `json:"PLAYER_COLOR,omitempty"`       // Player answer color (RED/GREEN/YELLOW/BLUE)
	Points              int    `json:"POINTS"`                       // Points awarded
	ReactionTime        int64  `json:"REACTION_TIME,omitempty"`      // Reaction time in microseconds
}
