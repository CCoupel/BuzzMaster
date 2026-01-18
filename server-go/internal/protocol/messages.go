package protocol

import (
	"encoding/json"
	"time"
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
	ActionReset       = "RESET"
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
	ActionBumperPoints     = "BUMPER_POINTS"
	ActionTeamPoints       = "TEAM_POINTS"
	ActionClients          = "CLIENTS"
	ActionSetClientType    = "SET_CLIENT_TYPE"
	ActionReorderQuestions  = "REORDER_QUESTIONS"
	ActionForceReady        = "FORCE_READY"
	ActionBackgroundChange  = "BACKGROUND_CHANGE"
	ActionFlipMemoryCard    = "FLIP_MEMORY_CARD"
	ActionQCMHint           = "QCM_HINT"
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
}

// IncomingMessage from TCP/WebSocket clients
type IncomingMessage struct {
	Source    string // "TCP", "WebSocket"
	Data      *Message
	ClientID  string
	Timestamp time.Time
}

// ButtonPayload for BUTTON action
type ButtonPayload struct {
	Button string `json:"button"`
}

// HelloPayload for HELLO action from buzzer
type HelloPayload struct {
	IP      string `json:"IP,omitempty"`
	Version string `json:"VERSION,omitempty"`
	Name    string `json:"NAME,omitempty"`
	Team    string `json:"TEAM,omitempty"`
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
	AdminCount int `json:"ADMIN_COUNT"`
	TVCount    int `json:"TV_COUNT"`
}

// SetClientTypePayload for SET_CLIENT_TYPE action
type SetClientTypePayload struct {
	Type string `json:"TYPE"` // "admin" or "tv"
}

// ReorderQuestionsPayload for REORDER_QUESTIONS action
type ReorderQuestionsPayload struct {
	Order []string `json:"ORDER"` // Array of question IDs in new order
}

// BackgroundChangePayload for BACKGROUND_CHANGE action
type BackgroundChangePayload struct {
	Index int `json:"INDEX"` // Current background index (0-based)
}

// FlipMemoryCardPayload for FLIP_MEMORY_CARD action
type FlipMemoryCardPayload struct {
	CardID string `json:"CARD_ID"` // Card ID to flip (e.g., "1-1", "2-2")
}

// QCMHintPayload for QCM_HINT action
type QCMHintPayload struct {
	Color     string `json:"COLOR"`     // Invalidated color (RED, GREEN, YELLOW, BLUE)
	Remaining int    `json:"REMAINING"` // Number of remaining valid answers
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
