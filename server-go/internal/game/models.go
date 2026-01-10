package game

import (
	"encoding/json"
)

// GamePhase represents the current game state
type GamePhase string

const (
	PhaseStopped  GamePhase = "STOPPED"
	PhasePrepare GamePhase = "PREPARE"
	PhaseReady   GamePhase = "READY"
	PhaseStarted  GamePhase = "STARTED"
	PhasePaused   GamePhase = "PAUSED"
	PhaseRevealed GamePhase = "REVEALED"
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
	Name   string `json:"NAME"`
	Color  []int  `json:"COLOR"`
	Score  int    `json:"SCORE"`
	Time   int64  `json:"TIME,omitempty"`
	Status string `json:"STATUS,omitempty"`
	Bumper string `json:"BUMPER,omitempty"`
	Ready  bool   `json:"READY,omitempty"`
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
	Ready       bool        `json:"READY,omitempty"`
	AnswerColor AnswerColor `json:"ANSWER_COLOR,omitempty"`
}

// QuestionType represents the type of question
type QuestionType string

const (
	QuestionTypeNormal QuestionType = "NORMAL"
	QuestionTypeQCM    QuestionType = "QCM"
)

// QCMAnswers holds the 4 possible answers for a QCM question
type QCMAnswers struct {
	Red    string `json:"RED"`
	Green  string `json:"GREEN"`
	Yellow string `json:"YELLOW"`
	Blue   string `json:"BLUE"`
}

// Question represents a quiz question
type Question struct {
	ID          string         `json:"ID"`
	Question    string         `json:"QUESTION"`
	Answer      string         `json:"ANSWER"`                // For normal questions
	Type        QuestionType   `json:"TYPE,omitempty"`        // "NORMAL" or "QCM" (default NORMAL)
	QCMAnswers  *QCMAnswers    `json:"QCM_ANSWERS,omitempty"` // For QCM questions
	QCMCorrect  string         `json:"QCM_CORRECT,omitempty"` // "RED", "GREEN", "YELLOW", "BLUE"
	Points      string         `json:"POINTS"`                // String to match JSON format
	Time        string         `json:"TIME"`                  // String to match JSON format
	Order       int            `json:"ORDER,omitempty"`       // Display order (for drag and drop)
	Media       string         `json:"MEDIA,omitempty"`       // Question media (shown during game)
	MediaAnswer string         `json:"MEDIA_ANSWER,omitempty"` // Answer media (shown during REVEAL)
	Status      QuestionStatus `json:"STATUS,omitempty"`
}

// Background represents a background image with its settings
type Background struct {
	Path     string  `json:"path"`
	Duration int     `json:"duration"` // Duration in seconds (default 10)
	Opacity  float64 `json:"opacity"`  // Opacity 0-100 (default 100)
}

// GameState holds the current game state
type GameState struct {
	Phase       GamePhase    `json:"PHASE"`
	Delay       int          `json:"DELAY"`
	CurrentTime int          `json:"CURRENT_TIME"`
	GameTime    int64        `json:"TIME,omitempty"`
	Question    *Question    `json:"QUESTION,omitempty"`
	Page        string       `json:"REMOTE,omitempty"`
	Backgrounds []Background `json:"backgrounds,omitempty"`
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
	Game    *GameState `json:"GAME,omitempty"`
	Teams   map[string]*Team   `json:"teams,omitempty"`
	Bumpers map[string]*Bumper `json:"bumpers,omitempty"`
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
