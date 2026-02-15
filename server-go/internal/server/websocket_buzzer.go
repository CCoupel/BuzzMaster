package server

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// BuzzerWebSocketHub manages WebSocket connections from physical buzzers.
// This is separate from the main WebSocketHub which handles web clients (admin/TV/VPlayer).
type BuzzerWebSocketHub struct {
	clients    map[*WebSocketClient]bool
	broadcast  chan []byte
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mu         sync.RWMutex

	// Channel for incoming messages from buzzers
	Incoming chan *protocol.IncomingMessage

	// Callback when buzzer connects/disconnects
	OnBuzzerChange func(buzzerCount int)
}

// NewBuzzerWebSocketHub creates a new buzzer WebSocket hub
func NewBuzzerWebSocketHub() *BuzzerWebSocketHub {
	return &BuzzerWebSocketHub{
		clients:    make(map[*WebSocketClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
		Incoming:   make(chan *protocol.IncomingMessage, 100),
	}
}

// Run starts the buzzer hub's main loop
func (h *BuzzerWebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			LogInfo(game.LogComponentWebSocket, "Buzzer connected via WebSocket: %s (MAC: %s)", client.ID, client.MAC)
			h.notifyBuzzerChange()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
			LogInfo(game.LogComponentWebSocket, "Buzzer disconnected from WebSocket: %s (MAC: %s)", client.ID, client.MAC)
			h.notifyBuzzerChange()

		case message := <-h.broadcast:
			h.mu.Lock()
			var toRemove []*WebSocketClient
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					toRemove = append(toRemove, client)
				}
			}
			for _, client := range toRemove {
				delete(h.clients, client)
			}
			h.mu.Unlock()
		}
	}
}

func (h *BuzzerWebSocketHub) notifyBuzzerChange() {
	if h.OnBuzzerChange != nil {
		h.OnBuzzerChange(h.BuzzerCount())
	}
}

// BuzzerCount returns the number of connected buzzer clients
func (h *BuzzerWebSocketHub) BuzzerCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Broadcast sends a message to all connected buzzer clients
func (h *BuzzerWebSocketHub) Broadcast(msg *protocol.Message) {
	data, err := msg.SerializeForWebSocket()
	if err != nil {
		LogError(game.LogComponentWebSocket, "Failed to serialize buzzer message: %v", err)
		return
	}
	h.broadcast <- data
}

// BroadcastRaw sends raw bytes to all buzzer clients
func (h *BuzzerWebSocketHub) BroadcastRaw(data []byte) {
	h.broadcast <- data
}

// SendToClient sends a message to a specific buzzer by MAC address
func (h *BuzzerWebSocketHub) SendToClient(mac string, msg *protocol.Message) error {
	data, err := msg.SerializeForWebSocket()
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.MAC == mac || client.ID == mac {
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

// SetClientMAC updates the MAC address of a connected buzzer client
func (h *BuzzerWebSocketHub) SetClientMAC(clientID string, mac string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		if client.ID == clientID {
			client.MAC = mac
			LogInfo(game.LogComponentWebSocket, "Buzzer %s identified as MAC: %s", clientID, mac)
			return
		}
	}
}

// GetClients returns list of connected buzzer MAC addresses
func (h *BuzzerWebSocketHub) GetClients() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	macs := make([]string, 0, len(h.clients))
	for client := range h.clients {
		id := client.MAC
		if id == "" {
			id = client.ID
		}
		macs = append(macs, id)
	}
	return macs
}

// HandleConnection upgrades HTTP to WebSocket for buzzer connections.
// Buzzers connect to /ws/buzzer and identify via MAC in query params or first HELLO message.
func (h *BuzzerWebSocketHub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		LogError(game.LogComponentWebSocket, "Buzzer WebSocket upgrade error: %v", err)
		return
	}

	// Try to get MAC from query parameter (optional, can also come via HELLO message)
	mac := r.URL.Query().Get("mac")

	clientID := r.RemoteAddr
	if mac != "" {
		clientID = mac
	}

	client := &WebSocketClient{
		ID:       clientID,
		MAC:      mac,
		Type:     ClientTypeBuzzer,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Hub:      nil, // Not using the web hub
		LastSeen: time.Now(),
	}

	h.register <- client

	go h.writePump(client)
	go h.readPump(client)
}

func (h *BuzzerWebSocketHub) readPump(c *WebSocketClient) {
	defer func() {
		h.unregister <- c
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
				LogWarn(game.LogComponentWebSocket, "Buzzer WS read error: %v", err)
			}
			break
		}

		c.LastSeen = time.Now()

		// Parse message
		msg, err := protocol.ParseSingle(message)
		if err != nil {
			LogError(game.LogComponentWebSocket, "Buzzer WS parse error: %v", err)
			continue
		}

		// Update MAC from message ID field (buzzer sends MAC in ID)
		if msg.ID != "" && c.MAC == "" {
			c.MAC = msg.ID
			c.ID = msg.ID
			LogInfo(game.LogComponentWebSocket, "Buzzer identified via message: MAC=%s", msg.ID)
		}

		// Use MAC as client ID for the game engine
		clientID := c.MAC
		if clientID == "" {
			clientID = c.ID
		}

		LogDebug(game.LogComponentWebSocket, "Buzzer WS received from %s: ACTION=%s", clientID, msg.Action)

		incoming := &protocol.IncomingMessage{
			Source:    "WebSocket-Buzzer",
			Data:      msg,
			ClientID:  clientID,
			Timestamp: time.Now(),
		}

		select {
		case h.Incoming <- incoming:
		default:
			LogWarn(game.LogComponentWebSocket, "Buzzer incoming channel full, dropping message")
		}
	}
}

func (h *BuzzerWebSocketHub) writePump(c *WebSocketClient) {
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

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// Drain queued messages
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
