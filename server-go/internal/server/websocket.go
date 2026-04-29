package server

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

// ClientType identifies the type of WebSocket client
type ClientType string

const (
	ClientTypeAdmin   ClientType = "admin"
	ClientTypeTV      ClientType = "tv"
	ClientTypeVPlayer ClientType = "vplayer"
	ClientTypeBuzzer  ClientType = "buzzer"
)

// WebSocketClient represents a connected WebSocket client
type WebSocketClient struct {
	ID       string
	MAC      string // MAC address for buzzer clients (empty for web clients)
	Type     ClientType
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *WebSocketHub
	LastSeen time.Time
}

// WebSocketHub manages all WebSocket connections
type WebSocketHub struct {
	clients    map[*WebSocketClient]bool
	broadcast  chan []byte
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mu         sync.RWMutex

	// Channel for incoming messages
	Incoming chan *protocol.IncomingMessage

	// Callback for handling messages
	OnMessage func(clientID string, msg *protocol.Message)

	// Callback when client count changes
	OnClientChange func(adminCount, tvCount, vplayerCount int)
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*WebSocketClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
		Incoming:   make(chan *protocol.IncomingMessage, 100),
	}
}

// Run starts the hub's main loop
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			LogInfo(game.LogComponentWebSocket, "Client connected: %s (type: %s)", client.ID, client.Type)
			h.notifyClientChange()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
			LogInfo(game.LogComponentWebSocket, "Client disconnected: %s (type: %s)", client.ID, client.Type)
			h.notifyClientChange()

		case message := <-h.broadcast:
			h.mu.Lock()
			// Collect clients to remove (don't delete while iterating)
			var toRemove []*WebSocketClient
			for client := range h.clients {
				select {
				case client.Send <- message:
					// Message sent successfully
				default:
					// Channel full or closed, mark for removal
					close(client.Send)
					toRemove = append(toRemove, client)
				}
			}
			// Remove failed clients
			for _, client := range toRemove {
				delete(h.clients, client)
			}
			h.mu.Unlock()
		}
	}
}

// notifyClientChange calls the OnClientChange callback with current counts
func (h *WebSocketHub) notifyClientChange() {
	if h.OnClientChange != nil {
		adminCount, tvCount, vplayerCount := h.GetClientCounts()
		h.OnClientChange(adminCount, tvCount, vplayerCount)
	}
}

// GetClientCounts returns the count of admin, TV, and VPlayer clients
func (h *WebSocketHub) GetClientCounts() (adminCount, tvCount, vplayerCount int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		switch client.Type {
		case ClientTypeTV:
			tvCount++
		case ClientTypeVPlayer:
			vplayerCount++
		default:
			adminCount++
		}
	}
	return
}

// SetClientType updates the type of a client by ID
func (h *WebSocketHub) SetClientType(clientID string, clientType ClientType) {
	h.mu.Lock()
	for client := range h.clients {
		if client.ID == clientID {
			client.Type = clientType
			LogInfo(game.LogComponentWebSocket, "Client %s type set to: %s", clientID, clientType)
			break
		}
	}
	h.mu.Unlock()
	// Notify after releasing lock to avoid deadlock
	h.notifyClientChange()
}

// Broadcast sends a message to all connected clients
func (h *WebSocketHub) Broadcast(msg *protocol.Message) {
	data, err := msg.SerializeForWebSocket()
	if err != nil {
		LogError(game.LogComponentWebSocket, "Failed to serialize message: %v", err)
		return
	}

	h.broadcast <- data
}

// BroadcastRaw sends raw bytes to all clients
func (h *WebSocketHub) BroadcastRaw(data []byte) {
	h.broadcast <- data
}

// SendToClient sends a message to a specific client
func (h *WebSocketHub) SendToClient(clientID string, msg *protocol.Message) error {
	data, err := msg.SerializeForWebSocket()
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.ID == clientID {
			select {
			case client.Send <- data:
				return nil
			default:
				return nil
			}
		}
	}

	return nil
}

// ClientCount returns number of connected clients
func (h *WebSocketHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// HandleConnection upgrades HTTP to WebSocket and handles the connection.
// The client type defaults to ClientTypeAdmin (legacy /ws endpoint).
func (h *WebSocketHub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	h.HandleConnectionWithType(w, r, ClientTypeAdmin)
}

// HandleConnectionWithType upgrades HTTP to WebSocket and fixes the client type at connection time.
// Used by dedicated endpoints (/ws/admin, /ws/tv, /ws/player) so the type is known immediately
// without waiting for a SET_CLIENT_TYPE message.
func (h *WebSocketHub) HandleConnectionWithType(w http.ResponseWriter, r *http.Request, clientType ClientType) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		LogError(game.LogComponentWebSocket, "Upgrade error: %v", err)
		return
	}

	client := &WebSocketClient{
		ID:       r.RemoteAddr,
		Type:     clientType,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Hub:      h,
		LastSeen: time.Now(),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

// BroadcastToTypes sends a message only to connected clients whose type matches one of the
// given types. The message is serialized once and the same bytes are sent to all matching
// clients (efficient for high-frequency updates like UPDATE_TIMER).
// Thread-safe: uses RLock for the client map iteration.
func (h *WebSocketHub) BroadcastToTypes(msg *protocol.Message, types ...ClientType) {
	if len(types) == 0 {
		return
	}

	data, err := msg.SerializeForWebSocket()
	if err != nil {
		LogError(game.LogComponentWebSocket, "BroadcastToTypes: failed to serialize message: %v", err)
		return
	}

	// Build a fast lookup set for the requested types
	typeSet := make(map[ClientType]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}

	// Use a single WLock so that close(client.Send) and delete(h.clients, client)
	// are atomic with respect to concurrent BroadcastToTypes calls.
	// A two-phase RLock+WLock upgrade would allow two goroutines to both close the same
	// channel between the RUnlock and the WLock, causing a "close of closed channel" panic.
	h.mu.Lock()
	for client := range h.clients {
		if !typeSet[client.Type] {
			continue
		}
		select {
		case client.Send <- data:
		default:
			close(client.Send)
			delete(h.clients, client)
		}
	}
	h.mu.Unlock()
}

// BroadcastRawToTypes sends pre-serialized bytes only to connected clients whose type
// matches one of the given types. Analogous to BroadcastToTypes but avoids the extra
// serialization round-trip when the caller has already prepared type-specific payloads
// (e.g. SerializeForWebClient / SerializeForBuzzer).
// Thread-safe: uses a single WLock so close(client.Send)+delete(h.clients, client) are atomic.
func (h *WebSocketHub) BroadcastRawToTypes(data []byte, types ...ClientType) {
	if len(types) == 0 {
		return
	}

	typeSet := make(map[ClientType]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}

	h.mu.Lock()
	for client := range h.clients {
		if !typeSet[client.Type] {
			continue
		}
		select {
		case client.Send <- data:
		default:
			close(client.Send)
			delete(h.clients, client)
		}
	}
	h.mu.Unlock()
}

func (c *WebSocketClient) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(65536)
	c.Conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				LogWarn(game.LogComponentWebSocket, "Read error: %v", err)
			}
			break
		}

		c.LastSeen = time.Now()

		// Parse message
		msg, err := protocol.ParseSingle(message)
		if err != nil {
			LogError(game.LogComponentWebSocket, "Parse error: %v", err)
			continue
		}

		LogDebug(game.LogComponentWebSocket, "Received from %s: ACTION=%s", c.ID, msg.Action)

		incoming := &protocol.IncomingMessage{
			Source:    "WebSocket",
			Data:      msg,
			ClientID:  c.ID,
			Timestamp: time.Now(),
		}

		select {
		case c.Hub.Incoming <- incoming:
		default:
			LogWarn(game.LogComponentWebSocket, "Incoming channel full, dropping message")
		}

		if c.Hub.OnMessage != nil {
			c.Hub.OnMessage(c.ID, msg)
		}
	}
}

func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(3 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send message as a single WebSocket frame (not batched)
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
			c.Conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
