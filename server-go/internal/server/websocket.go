package server

import (
	"buzzcontrol/internal/protocol"
	"log"
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

// WebSocketClient represents a connected WebSocket client
type WebSocketClient struct {
	ID       string
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
			log.Printf("[WebSocket] Client connected: %s", client.ID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
			log.Printf("[WebSocket] Client disconnected: %s", client.ID)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *WebSocketHub) Broadcast(msg *protocol.Message) {
	data, err := msg.SerializeForWebSocket()
	if err != nil {
		log.Printf("[WebSocket] Failed to serialize message: %v", err)
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

// HandleConnection upgrades HTTP to WebSocket and handles the connection
func (h *WebSocketHub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket] Upgrade error: %v", err)
		return
	}

	client := &WebSocketClient{
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

func (c *WebSocketClient) readPump() {
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
				log.Printf("[WebSocket] Read error: %v", err)
			}
			break
		}

		c.LastSeen = time.Now()

		// Parse message
		msg, err := protocol.ParseSingle(message)
		if err != nil {
			log.Printf("[WebSocket] Parse error: %v", err)
			continue
		}

		log.Printf("[WebSocket] Received from %s: ACTION=%s", c.ID, msg.Action)

		incoming := &protocol.IncomingMessage{
			Source:    "WebSocket",
			Data:      msg,
			ClientID:  c.ID,
			Timestamp: time.Now(),
		}

		select {
		case c.Hub.Incoming <- incoming:
		default:
			log.Printf("[WebSocket] Incoming channel full, dropping message")
		}

		if c.Hub.OnMessage != nil {
			c.Hub.OnMessage(c.ID, msg)
		}
	}
}

func (c *WebSocketClient) writePump() {
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
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
