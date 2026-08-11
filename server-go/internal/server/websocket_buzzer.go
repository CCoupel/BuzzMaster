package server

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"fmt"
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

	// Callback when buzzer connects/disconnects (count-based, legacy)
	OnBuzzerChange func(buzzerCount int)

	// Callbacks for individual buzzer connect/disconnect (MAC-based, v3.6.5).
	// OnBuzzerConnected is reserved for future use — CONNECTED=true is handled in
	// handleHello() (main.go) because the MAC is only reliably known after HELLO.
	// OnBuzzerDisconnected fires immediately on WebSocket close with the buzzer MAC.
	OnBuzzerConnected    func(mac string) // reserved for future use
	OnBuzzerDisconnected func(mac string)
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
			// Snapshot under lock (bugfix #133): client.MAC/client.ID can be
			// rewritten by readPump's identifyClient (also under h.mu) the
			// instant the buzzer's first HELLO/message arrives, which can
			// race this log line if read after Unlock. Same "collect under
			// lock, act outside" shape as AckManager.tick()
			// (ack_manager.go:128-174).
			id, mac := client.ID, client.MAC
			h.mu.Unlock()
			LogInfo(game.LogComponentWebSocket, "Buzzer connected via WebSocket: %s (MAC: %s)", id, mac)
			h.notifyBuzzerChange()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			// Snapshot under lock — see the register case above (bugfix #133).
			id, mac := client.ID, client.MAC
			h.mu.Unlock()
			LogInfo(game.LogComponentWebSocket, "Buzzer disconnected from WebSocket: %s (MAC: %s)", id, mac)
			// Notify with MAC address so main.go can update CONNECTED=false
			if mac == "" {
				mac = id
			}
			if h.OnBuzzerDisconnected != nil && mac != "" {
				h.OnBuzzerDisconnected(mac)
			}
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

// ConnectedCount returns the number of connected buzzer clients (alias for BuzzerCount).
func (h *BuzzerWebSocketHub) ConnectedCount() int {
	return h.BuzzerCount()
}

// buzzerActionWhitelist defines the set of actions that are relevant for physical buzzers.
// All other actions are silently dropped by BroadcastIfRelevant / BroadcastRawIfRelevant.
// Physical buzzers need:
//   - Game-state actions (UPDATE, UPDATE_TIMER, START, CONTINUE, STOP, PAUSE, READY, RESET)
//     to synchronise their local state machine and drive their LED logic.
//   - Control actions (HELLO, LED_SET, OTA_UPDATE, WIFI_CONFIG) for identification and
//     hardware control.
var buzzerActionWhitelist = map[string]bool{
	protocol.ActionPing:        true,
	protocol.ActionUpdate:      true,
	protocol.ActionUpdateTimer: true,
	protocol.ActionStart:       true,
	protocol.ActionContinue:    true,
	protocol.ActionStop:        true,
	protocol.ActionPause:       true,
	protocol.ActionReady:       true,
	protocol.ActionReset:       true,
	protocol.ActionHello:       true,
	protocol.ActionLEDSet:      true,
	protocol.ActionOTAUpdate:   true,
	protocol.ActionWifiConfig:  true,
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

// BroadcastIfRelevant sends a message to all connected buzzer clients only if the
// action is in the buzzer whitelist. All other actions are silently dropped.
func (h *BuzzerWebSocketHub) BroadcastIfRelevant(msg *protocol.Message) {
	if !buzzerActionWhitelist[msg.Action] {
		return
	}
	h.Broadcast(msg)
}

// BroadcastRawIfRelevant sends pre-serialized bytes to all buzzer clients only if the
// action is in the buzzer whitelist. Used by broadcastUpdate() to avoid re-serializing
// a payload that has already been prepared for buzzer clients.
func (h *BuzzerWebSocketHub) BroadcastRawIfRelevant(action string, data []byte) {
	if !buzzerActionWhitelist[action] {
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
				return fmt.Errorf("send channel full for buzzer %s, message dropped", mac)
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

// identifyClient sets c.MAC/c.ID to mac the first time a buzzer self-reports
// it on its own connection (readPump, on the message's ID field) — the
// direct-pointer equivalent of SetClientMAC, used where the caller already
// holds *WebSocketClient instead of a clientID to look up. Returns true the
// first time (so the caller can log once), false if already identified.
//
// Bugfix #133: readPump used to write c.MAC/c.ID directly with no
// synchronization at all, while Run() (register/unregister), SendToClient,
// IsClientConnected and GetClients all read the same fields — some under
// h.mu, some not. Routing every read AND write through h.mu closes that
// race for good (same bug class as SetClientType/SetClientPlayerID in
// websocket.go — see ai_job.go:231-237 for the prior instance of this class
// in this repo).
func (h *BuzzerWebSocketHub) identifyClient(c *WebSocketClient, mac string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.MAC != "" {
		return false
	}
	c.MAC = mac
	c.ID = mac
	return true
}

// clientDisplayID returns c.MAC if set, else c.ID, read under h.mu for the
// same reason as identifyClient (bugfix #133).
func (h *BuzzerWebSocketHub) clientDisplayID(c *WebSocketClient) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if c.MAC != "" {
		return c.MAC
	}
	return c.ID
}

// IsClientConnected returns true if a buzzer with the given MAC address is currently connected.
func (h *BuzzerWebSocketHub) IsClientConnected(mac string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.MAC == mac || client.ID == mac {
			return true
		}
	}
	return false
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
		ID:   clientID,
		MAC:  mac,
		Type: ClientTypeBuzzer,
		Conn: conn,
		Send: make(chan []byte, 256),
		Hub:  nil, // Not using the web hub
	}

	h.register <- client

	go h.writePump(client)
	go h.readPump(client)
}

func (h *BuzzerWebSocketHub) readPump(c *WebSocketClient) {
	defer func() {
		// Bugfix #131 — see websocket.go's readPump for the rationale
		// (recover() must be called directly in this literal).
		if r := recover(); r != nil {
			LogRecoveredPanic(game.LogComponentWebSocket, "buzzer readPump client="+c.ID, r)
		}
		h.unregister <- c
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
				LogWarn(game.LogComponentWebSocket, "Buzzer WS read error: %v", err)
			}
			break
		}

		// Parse message
		msg, err := protocol.ParseSingle(message)
		if err != nil {
			LogError(game.LogComponentWebSocket, "Buzzer WS parse error: %v", err)
			continue
		}

		// Update MAC from message ID field (buzzer sends MAC in ID) — routed
		// through identifyClient (h.mu-guarded) instead of a direct field
		// write (bugfix #133; see identifyClient's doc comment).
		if msg.ID != "" && h.identifyClient(c, msg.ID) {
			LogInfo(game.LogComponentWebSocket, "Buzzer identified via message: MAC=%s", msg.ID)
		}

		// Use MAC as client ID for the game engine
		clientID := h.clientDisplayID(c)

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
	ticker := time.NewTicker(3 * time.Second)
	defer func() {
		// Bugfix #131 — see websocket.go's readPump for the rationale
		// (recover() must be called directly in this literal).
		if r := recover(); r != nil {
			LogRecoveredPanic(game.LogComponentWebSocket, "buzzer writePump client="+c.ID, r)
		}
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
			c.Conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
