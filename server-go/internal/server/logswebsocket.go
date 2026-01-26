package server

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// LogsWebSocketClient represents a client connected to the logs WebSocket
type LogsWebSocketClient struct {
	ID       string
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *LogsWebSocketHub
	LastSeen time.Time
}

// LogsWebSocketHub manages WebSocket connections for logs
type LogsWebSocketHub struct {
	clients    map[*LogsWebSocketClient]bool
	register   chan *LogsWebSocketClient
	unregister chan *LogsWebSocketClient
	mu         sync.RWMutex

	// LogBuffer stores recent log entries to send to new clients
	logBuffer     []protocol.LogEntryPayload
	logBufferSize int
}

// NewLogsWebSocketHub creates a new logs WebSocket hub
func NewLogsWebSocketHub(bufferSize int) *LogsWebSocketHub {
	return &LogsWebSocketHub{
		clients:       make(map[*LogsWebSocketClient]bool),
		register:      make(chan *LogsWebSocketClient),
		unregister:    make(chan *LogsWebSocketClient),
		logBuffer:     make([]protocol.LogEntryPayload, 0, bufferSize),
		logBufferSize: bufferSize,
	}
}

// Run starts the hub's main loop
func (h *LogsWebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

			// Send LOG_HISTORY to the new client
			h.sendLogHistory(client)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
		}
	}
}

// sendLogHistory sends all buffered log entries to a client
func (h *LogsWebSocketHub) sendLogHistory(client *LogsWebSocketClient) {
	h.mu.RLock()
	entries := make([]protocol.LogEntryPayload, len(h.logBuffer))
	copy(entries, h.logBuffer)
	h.mu.RUnlock()

	msg, err := protocol.NewMessage(protocol.ActionLogHistory, protocol.LogHistoryPayload{
		Entries: entries,
	})
	if err != nil {
		return
	}

	data, err := msg.SerializeForWebSocket()
	if err != nil {
		return
	}

	select {
	case client.Send <- data:
	default:
		// Channel full, skip
	}
}

// BroadcastLogEntry broadcasts a single log entry to all connected clients
// and adds it to the buffer
func (h *LogsWebSocketHub) BroadcastLogEntry(entry protocol.LogEntryPayload) {
	// Add to buffer
	h.mu.Lock()
	h.logBuffer = append(h.logBuffer, entry)
	if len(h.logBuffer) > h.logBufferSize {
		// Keep only the last N entries
		h.logBuffer = h.logBuffer[len(h.logBuffer)-h.logBufferSize:]
	}
	h.mu.Unlock()

	// Create message
	msg, err := protocol.NewMessage(protocol.ActionLogEntry, map[string]interface{}{
		"entry": entry,
	})
	if err != nil {
		return
	}

	data, err := msg.SerializeForWebSocket()
	if err != nil {
		return
	}

	// Broadcast to all clients
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.Send <- data:
		default:
			// Channel full, skip this client
		}
	}
}

// HandleConnection upgrades HTTP to WebSocket and handles the connection
func (h *LogsWebSocketHub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		LogError(game.LogComponentWebSocket, "Logs WebSocket upgrade error: %v", err)
		return
	}

	client := &LogsWebSocketClient{
		ID:       r.RemoteAddr,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Hub:      h,
		LastSeen: time.Now(),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *LogsWebSocketClient) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(65536)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				LogWarn(game.LogComponentWebSocket, "Logs WebSocket read error: %v", err)
			}
			break
		}

		c.LastSeen = time.Now()

		// Parse message (logs WebSocket doesn't expect many incoming messages)
		var msg protocol.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		// Logs WebSocket is mostly one-way (server -> client)
		// No specific actions to handle for now
	}
}

func (c *LogsWebSocketClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send message as a single WebSocket frame
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// Send any queued messages as separate frames
			n := len(c.Send)
			for i := 0; i < n; i++ {
				msg := <-c.Send
				if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
