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
	// ClientTypeAnim — v6.2.0 (#155): the interface animateur (/anim, /ws/anim),
	// a reduced-capability web client (contracts/websocket-actions.md
	// §"Sécurité — Allow-list entrante", contracts/ws-payload-serialization.md
	// §"Animateur"). Fixed at connection time via HandleConnectionWithType,
	// same as TV/VPlayer — never mutated after (IsSetClientTypeAllowed only
	// permits SET_CLIENT_TYPE from the Admin state).
	ClientTypeAnim   ClientType = "anim"
	ClientTypeBuzzer ClientType = "buzzer"
)

// WebSocketClient represents a connected WebSocket client
type WebSocketClient struct {
	ID       string
	MAC      string // MAC address for buzzer clients (empty for web clients)
	PlayerID string // VJoueur bumper ID for ClientTypeVPlayer clients (empty for admin/TV and not-yet-identified VPlayers)
	Type     ClientType
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *WebSocketHub
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
	// animCount added v6.2.0 (#155) — interface animateur.
	OnClientChange func(adminCount, tvCount, vplayerCount, animCount int)

	// OnPlayerDisconnected fires immediately on WebSocket close for a ClientTypeVPlayer
	// client whose PlayerID has been identified (SetClientPlayerID called after PLAYER_CONNECT).
	// Mirrors BuzzerWebSocketHub.OnBuzzerDisconnected for physical buzzers.
	OnPlayerDisconnected func(playerID string)

	// OnClientRegistered fires whenever a client's type becomes known/changes
	// — either at connection time (the dedicated /ws/admin, /ws/tv, /ws/player
	// endpoints know their type immediately, contract ai-multi-provider.md
	// §10) or later via SetClientType (the legacy /ws endpoint + SET_CLIENT_TYPE).
	// Used to push AI_GENERATION_PROGRESS to a newly-identified admin client
	// if a job is running, so a page reload retrieves progress without any
	// client-side reconstruction logic. nil-safe: unit tests building a bare
	// hub never set it.
	OnClientRegistered func(client *WebSocketClient)
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
			// Snapshot the fields this case needs to read AFTER Unlock, while
			// still holding h.mu (bugfix #133): client.Type is also written
			// under h.mu by SetClientType, so reading it here unlocked would
			// race with a concurrent SET_CLIENT_TYPE from the very connection
			// that just registered. Same "collect under lock, act outside"
			// shape as AckManager.tick() (ack_manager.go:128-174) — the field
			// values are frozen into local vars before Unlock instead of
			// being re-read from the shared *WebSocketClient afterward.
			id, clientType := client.ID, client.Type
			h.mu.Unlock()
			LogInfo(game.LogComponentWebSocket, "Client connected: %s (type: %s)", id, clientType)
			h.notifyClientChange()
			// Fired here (not in HandleConnectionWithType) so the hook only
			// runs once the client is actually present in h.clients — see
			// the NOTE at the h.register<- send site.
			if h.OnClientRegistered != nil {
				h.OnClientRegistered(client)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			// Snapshot under lock — see the register case above for why
			// (bugfix #133). client.PlayerID is written under h.mu by
			// SetClientPlayerID, so it needs the same treatment as Type.
			id, clientType, playerID := client.ID, client.Type, client.PlayerID
			h.mu.Unlock()
			LogInfo(game.LogComponentWebSocket, "Client disconnected: %s (type: %s)", id, clientType)
			// Notify individual VPlayer disconnection so main.go can update CONNECTED=false.
			// Fired outside the lock, same pattern as notifyClientChange.
			if clientType == ClientTypeVPlayer && playerID != "" && h.OnPlayerDisconnected != nil {
				h.OnPlayerDisconnected(playerID)
			}
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
		adminCount, tvCount, vplayerCount, animCount := h.GetClientCounts()
		h.OnClientChange(adminCount, tvCount, vplayerCount, animCount)
	}
}

// GetClientCounts returns the count of admin, TV, VPlayer, and (v6.2.0, #155)
// animateur clients.
//
// #154 E2 (sec): previously `default: adminCount++` — any client whose Type
// wasn't TV or VPlayer counted as Admin, including an unrecognized/future
// type (e.g. ClientTypeBuzzer, which never actually registers on THIS hub —
// see BuzzerWebSocketHub — but nothing enforced that invariant here). An
// explicit case per known type means a type this function doesn't recognize
// counts toward none of the four, matching ClientsPayload's actual fields
// (AdminCount/TVCount/VPlayerCount/AnimCount only — buzzer counts come from
// a.buzzerHub.BuzzerCount() separately) instead of silently inflating the
// admin count.
func (h *WebSocketHub) GetClientCounts() (adminCount, tvCount, vplayerCount, animCount int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		switch client.Type {
		case ClientTypeAdmin:
			adminCount++
		case ClientTypeTV:
			tvCount++
		case ClientTypeVPlayer:
			vplayerCount++
		case ClientTypeAnim:
			animCount++
		}
	}
	return
}

// SetClientType updates the type of a client by ID
func (h *WebSocketHub) SetClientType(clientID string, clientType ClientType) {
	h.mu.Lock()
	var changed *WebSocketClient
	for client := range h.clients {
		if client.ID == clientID {
			client.Type = clientType
			changed = client
			LogInfo(game.LogComponentWebSocket, "Client %s type set to: %s", clientID, clientType)
			break
		}
	}
	h.mu.Unlock()
	// Notify/hook after releasing lock to avoid deadlock
	h.notifyClientChange()
	if changed != nil && h.OnClientRegistered != nil {
		h.OnClientRegistered(changed)
	}
}

// TypeSnapshot returns the client's current Type, safe for concurrent use
// with SetClientType (which writes Type under Hub.mu from the dispatch
// goroutine — a different goroutine from this client's own readPump).
//
// #154 code review CRITIQUE 2: readPump used to read c.Type directly
// (unprotected) to build each message's IncomingMessage.ClientType and to
// gate CANCEL_AI_GENERATION — a real, reproduced-under-`-race` data race
// against SetClientType's locked write, same bug class as #133 on this same
// field (client.Type read/written across goroutines, this file). Takes the
// hub's RLock — cheap (single field read) and never held across a callback,
// so it cannot deadlock against SetClientType's own Lock/Unlock-then-notify
// sequence.
func (c *WebSocketClient) TypeSnapshot() ClientType {
	c.Hub.mu.RLock()
	defer c.Hub.mu.RUnlock()
	return c.Type
}

// SetClientPlayerID links a connected client to its VJoueur bumper ID once identified
// (after PLAYER_CONNECT create/reconnect). Same pattern as SetClientType.
func (h *WebSocketHub) SetClientPlayerID(clientID, playerID string) {
	h.mu.Lock()
	for client := range h.clients {
		if client.ID == clientID {
			client.PlayerID = playerID
			LogInfo(game.LogComponentWebSocket, "Client %s identified as PlayerID: %s", clientID, playerID)
			break
		}
	}
	h.mu.Unlock()
}

// GetClientPlayerID returns the VJoueur bumper ID linked to clientID (via
// SetClientPlayerID), if any. ok is false if the client is unknown or not yet
// identified as a VJoueur (e.g. admin/TV, or a VPlayer before PLAYER_CONNECT
// completes). Used to fire the DELIVERY_CONFIRMED connection-badge event (#109
// Phase 2, D3) when a message is received from an identified VJoueur.
func (h *WebSocketHub) GetClientPlayerID(clientID string) (playerID string, ok bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.ID == clientID {
			return client.PlayerID, client.PlayerID != ""
		}
	}
	return "", false
}

// IsPlayerIDConnected returns true if a ClientTypeVPlayer client with the given PlayerID
// is currently connected. Used as an anti-zombie guard before marking a VJoueur disconnected,
// same principle as BuzzerWebSocketHub.IsClientConnected.
func (h *WebSocketHub) IsPlayerIDConnected(playerID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.Type == ClientTypeVPlayer && client.PlayerID == playerID {
			return true
		}
	}
	return false
}

// SendToPlayerID sends a message to every ClientTypeVPlayer client currently
// linked to playerID via SetClientPlayerID (reverse lookup of
// GetClientPlayerID). Used for targeted, never-broadcast notifications such
// as PLAYER_EVICTED (#120) — the admin/TV clients and other VJoueurs never
// receive it. A no-op (nil error) if no client is currently linked to
// playerID, e.g. the VJoueur already disconnected.
//
// #129 code review: sends to ALL matching clients, not just the first one a
// map iteration happens to visit. Two clients can legitimately share the
// same PlayerID for a brief, real window during a fast reconnect: the OLD
// connection is only removed from h.clients once ITS OWN failure is
// detected server-side (read timeout or a failed write — up to a few
// seconds, readPump/writePump, websocket.go), which is not synchronized
// with how quickly the client-side reconnects and completes a fresh
// PLAYER_CONNECT -> SetClientPlayerID on a NEW connection. Nothing evicts
// the stale registration before the new one links the same PlayerID.
// Returning after the first match found via Go's randomized map iteration
// order meant this could non-deterministically land on the stale,
// about-to-be-cleaned-up connection instead of the live one the caller
// actually needs to reach — silently breaking CA2-class guarantees (the
// reconnecting player must receive its own echo) in that narrow window.
// Sending to every match costs nothing extra in the common case (exactly
// one match) and eliminates the race in the rare one.
func (h *WebSocketHub) SendToPlayerID(playerID string, msg *protocol.Message) error {
	data, err := msg.SerializeForWebSocket()
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.Type == ClientTypeVPlayer && client.PlayerID == playerID {
			select {
			case client.Send <- data:
			default:
			}
		}
	}

	return nil
}

// SendRawToPlayerID sends pre-serialized bytes to every ClientTypeVPlayer
// client currently linked to playerID (reverse lookup of GetClientPlayerID)
// — raw twin of SendToPlayerID for callers that have already serialized a
// type-appropriate payload themselves (#129 T1.1). See SendToPlayerID's doc
// comment for why this targets every match rather than the first found:
// same stale-vs-live duplicate-registration race, same fix.
//
// Why not reuse SendToPlayerID: it always serializes via
// SerializeForWebSocket — the unfiltered payload, OTA/ACK fields included.
// A caller building a targeted echo for a VPlayer wants the same reduced/
// filtered payload contract §1-§2 already promises that client type
// (typically via Message.SerializeForVPlayer); routing it through
// SendToPlayerID would silently hand back a larger payload than #127
// established for this client type.
//
// Same locking discipline and saturation semantics as SendToPlayerID: RLock
// only (no send ever removes a client here), silent no-op if playerID isn't
// currently connected or a given match's Send channel is full.
func (h *WebSocketHub) SendRawToPlayerID(playerID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.Type == ClientTypeVPlayer && client.PlayerID == playerID {
			select {
			case client.Send <- data:
			default:
			}
		}
	}
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

// SendRawToClient sends pre-serialized bytes to a specific client by ID — raw
// twin of SendToClient for callers that need type-appropriate serialization
// decided by the caller (#154 E1: App.sendStateToClient picks SerializeForAdmin
// vs SerializeForWebClient per the connecting client's own type, instead of
// SendToClient's always-full SerializeForWebSocket). Same silent-no-op
// semantics as SendToClient: unknown clientID or a saturated Send channel
// both return nil.
func (h *WebSocketHub) SendRawToClient(clientID string, data []byte) error {
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
		ID:   r.RemoteAddr,
		Type: clientType,
		Conn: conn,
		Send: make(chan []byte, 256),
		Hub:  h,
	}

	h.register <- client
	// NOTE: register is unbuffered, but that only synchronizes the send/
	// receive rendezvous itself — it does NOT guarantee Run()'s goroutine has
	// finished h.clients[client]=true by the time this send returns (that's
	// code AFTER the receive, in a different goroutine, with no ordering
	// guarantee relative to here). The OnClientRegistered hook is fired from
	// inside Run() instead, once the client is actually in h.clients — firing
	// it here raced SendToClient (used by the hook) against the registration
	// itself and silently dropped the message when the hook lost the race.

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

	// #154 E4 (sec): serialize once PER DISTINCT requested type, not once
	// globally via SerializeForWebSocket — the same bytes used to be handed
	// to every recipient regardless of its type. Every SerializeForXxx
	// variant (messages.go) only actually reduces content for
	// Action == ActionUpdate today, so this has no observable effect on any
	// current caller (none of them pass an UPDATE through this path — see
	// broadcastUpdateTo/BroadcastRawToTypes for that). It exists so a future
	// action that DOES need per-type content (e.g. #155's animateur work)
	// is correct by construction here, rather than depending on every new
	// call site remembering to route around this function instead.
	dataByType := make(map[ClientType][]byte, len(types))
	for _, t := range types {
		if _, done := dataByType[t]; done {
			continue
		}
		data, err := serializeForClientType(msg, t)
		if err != nil {
			LogError(game.LogComponentWebSocket, "BroadcastToTypes: failed to serialize message for type %s: %v", t, err)
			continue
		}
		dataByType[t] = data
	}
	if len(dataByType) == 0 {
		return
	}

	// Use a single WLock so that close(client.Send) and delete(h.clients, client)
	// are atomic with respect to concurrent BroadcastToTypes calls.
	// A two-phase RLock+WLock upgrade would allow two goroutines to both close the same
	// channel between the RUnlock and the WLock, causing a "close of closed channel" panic.
	h.mu.Lock()
	for client := range h.clients {
		data, ok := dataByType[client.Type]
		if !ok {
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

// serializeForClientType picks the same per-type serializer the rest of the
// codebase already uses for broadcasts (broadcastUpdateTo, buildVPlayerPayloads)
// — Admin gets the full/unfiltered payload, TV and VPlayer share the
// admin-fields-stripped payload, Buzzer gets the minimal buzzer payload.
// clientType == ClientTypeBuzzer never actually appears among THIS hub's
// clients (physical buzzers register on the separate BuzzerWebSocketHub) —
// handled anyway so a caller passing it here (e.g. against a future unified
// hub) gets the correct payload rather than the admin one by accident.
func serializeForClientType(msg *protocol.Message, clientType ClientType) ([]byte, error) {
	switch clientType {
	case ClientTypeAdmin:
		return msg.SerializeForAdmin()
	case ClientTypeTV, ClientTypeAnim:
		// #155 (v6.2.0): ClientTypeAnim reuses SerializeForWebClient, same as
		// TV — no dedicated serializer. Its payload need is a strict subset
		// of what TV already gets (contracts/ws-payload-serialization.md
		// §"Animateur" has the full decision + justification). PRIORITY FIX
		// (plan _work/reports/plan-20260813-092950.md §0.2): this case MUST
		// list ClientTypeAnim explicitly — the default branch below returns
		// the full ADMIN payload (firmware/OTA/ACK fields, QUIZ_OBJECTIVES,
		// config), the exact opposite of the intent, for any type this
		// switch doesn't know about.
		return msg.SerializeForWebClient()
	case ClientTypeVPlayer:
		// #128 (v6.5.2): VPlayer split out of the TV/anim branch above —
		// SerializeForWebClient() no longer suffices for it. VPlayer must
		// additionally lose VPlayerOnlyGameFields (ARDOISE_ANSWERS), which
		// TV and /anim legitimately need (contract vplayer-payload-filter.md
		// §6). SerializeForVPlayerCommon applies both AdminOnlyGameFields
		// AND VPlayerOnlyGameFields. Per-recipient reduction (PREPARE/READY,
		// own bumper entry) happens separately, on the identified path
		// (SerializeForVPlayer(playerID) / the hot fan-out in
		// cmd/server/main.go) — this generic, non-identified path is what a
		// not-yet-PLAYER_CONNECT'd VPlayer socket gets.
		return msg.SerializeForVPlayerCommon()
	case ClientTypeBuzzer:
		return msg.SerializeForBuzzer()
	default:
		return msg.SerializeForAdmin()
	}
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

// VPlayerRecipient identifies one connected, identified VPlayer client —
// the unit SnapshotVPlayerRecipients/SendRawToVPlayers use for individualized
// UPDATE fan-out during PREPARE/READY (#127 T2.2, contracts/vplayer-payload-filter.md §3).
type VPlayerRecipient struct {
	ClientID string
	PlayerID string
}

// SnapshotVPlayerRecipients returns every currently-connected VPlayer client
// that has already been identified (SetClientPlayerID called, i.e. after a
// completed PLAYER_CONNECT) — candidates for a personalized payload. A
// VPlayer client with no PlayerID yet is deliberately excluded: contract §2
// condition 3 requires it always receive the complete payload instead.
//
// Read-locked only (RLock) so the caller can build per-recipient payloads
// entirely outside any lock before calling SendRawToVPlayers — contract §3:
// "aucun json.Marshal ne doit être exécuté en tenant WebSocketHub.mu".
func (h *WebSocketHub) SnapshotVPlayerRecipients() []VPlayerRecipient {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var out []VPlayerRecipient
	for client := range h.clients {
		if client.Type == ClientTypeVPlayer && client.PlayerID != "" {
			out = append(out, VPlayerRecipient{ClientID: client.ID, PlayerID: client.PlayerID})
		}
	}
	return out
}

// SendRawToVPlayers delivers pre-serialized payloads to every currently-
// connected VPlayer client: a client whose PlayerID has an entry in payloads
// gets that personalized slice; any other VPlayer client — not yet
// identified, or one that connected/disconnected after
// SnapshotVPlayerRecipients() was called — gets fallback (the same complete
// filtered payload TV receives, contract §2 condition 3).
//
// payloads and fallback must already be fully serialized: no json.Marshal
// runs here, only channel pushes, matching contract §3. Re-reads h.clients
// live (not the earlier snapshot) so a client that connects between the
// snapshot and this call still gets a payload instead of silently missing
// this broadcast. Single Lock for the whole pass, same invariant as
// BroadcastToTypes/BroadcastRawToTypes above: close(client.Send) and
// delete(h.clients, client) on a saturated channel stay atomic with respect
// to any concurrent broadcast on this hub.
func (h *WebSocketHub) SendRawToVPlayers(payloads map[string][]byte, fallback []byte) {
	h.mu.Lock()
	for client := range h.clients {
		if client.Type != ClientTypeVPlayer {
			continue
		}
		data := fallback
		if client.PlayerID != "" {
			if personalized, ok := payloads[client.PlayerID]; ok {
				data = personalized
			}
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
		// Bugfix #131: a panic anywhere in this per-connection loop (e.g. in
		// hub.OnMessage, invoked below on every message) must take down only
		// THIS connection, not the whole process. recover() must be called
		// directly here (see LogRecoveredPanic's doc comment) — the existing
		// cleanup below still runs unconditionally, panic or not.
		if r := recover(); r != nil {
			LogRecoveredPanic(game.LogComponentWebSocket, "readPump client="+c.ID, r)
		}
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(65536)
	c.Conn.SetReadDeadline(time.Now().Add(readDeadlineTimeout))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(readDeadlineTimeout))
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

		// Parse message
		msg, err := protocol.ParseSingle(message)
		if err != nil {
			LogError(game.LogComponentWebSocket, "Parse error: %v", err)
			continue
		}

		LogDebug(game.LogComponentWebSocket, "Received from %s: ACTION=%s", c.ID, msg.Action)

		// #154 code review CRITIQUE 2: c.Type is written under c.Hub.mu by
		// SetClientType (below), called from the dispatch goroutine
		// (cmd/server's handleSetClientType, a different goroutine from this
		// per-connection readPump). Reading c.Type directly here — as both
		// call sites below used to — races that write: a client that sends
		// SET_CLIENT_TYPE followed immediately by a second message (no wait
		// for the first to be dispatched) can hit this line while the
		// dispatch goroutine is still inside SetClientType's locked section.
		// Same bug class as #133 on this same field. TypeSnapshot takes the
		// same RLock SetClientType's write takes, so this read is ordered
		// with respect to it; both uses below share this single snapshot
		// instead of re-reading c.Type (which would just move the race).
		clientType := c.TypeSnapshot()

		// CANCEL_AI_GENERATION (contract ai-multi-provider.md §11) is handled
		// directly here, self-contained in package server, rather than
		// relying on cmd/server's App-level dispatch (which consumes
		// h.Incoming / OnMessage) — package-level tests that talk to a bare
		// HTTPServer/WebSocketHub (no App wired up) would otherwise never be
		// able to exercise cancellation at all. CancelAIJob is idempotent
		// (cancelOnce), so this is also safe if something upstream dispatches
		// the same action again.
		//
		// #154 (sec): this bypasses cmd/server's handleWebMessage allow-list
		// entirely (it's dispatched from here, not from that switch), so the
		// admin-only restriction documented on protocol.ActionCancelAIGeneration
		// ("/ws/admin only") must be enforced here directly instead — same
		// vulnerability class as the rest of #154 (a TV/VPlayer client could
		// otherwise cancel a running AI generation job it never started).
		if msg.Action == protocol.ActionCancelAIGeneration {
			if clientType == ClientTypeAdmin {
				var payload protocol.CancelAIGenerationPayload
				if err := json.Unmarshal(msg.Msg, &payload); err == nil {
					CancelAIJob(payload.JobID)
				}
			} else {
				// #154 code review (mineur): symmetric WARN to
				// handleWebMessage's allow-list rejection log — this is the
				// one action that bypasses that switch entirely, so it must
				// not also bypass the observability every other rejection
				// gets.
				LogWarn(game.LogComponentWebSocket, "Rejected action %s from client %s (type=%q): not allowed for this client type", msg.Action, c.ID, clientType)
			}
		}

		incoming := &protocol.IncomingMessage{
			Source:     "WebSocket",
			Data:       msg,
			ClientID:   c.ID,
			ClientType: string(clientType),
			Timestamp:  time.Now(),
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

// writePumpTickPeriod drives both the protocol ping frame and the
// application-level HEARTBEAT (#118) sent alongside it. A single named
// constant so HEARTBEAT.INTERVAL_MS can never drift from the ticker that
// actually paces it — see the HEARTBEAT payload build below.
//
// #130: 3s -> 2s. contracts/liveness-timing.md §4 justifies the full
// recalibration; the essential fact this constant drives: at the previous
// P=3s/D=5s pairing, a single lost ping pushed the next pong to 6s > 5s —
// the connection closed on ANY isolated packet loss, a common event on a
// venue WiFi. There was no real tolerance behind the apparent 2s margin.
const writePumpTickPeriod = 2 * time.Second

// readDeadlineTimeout is how long readPump waits without a Pong (control
// frame, refreshed by SetPongHandler) before considering the connection
// dead and closing it — set at connection start and re-armed on every Pong.
//
// #130: 5s -> 7s, paired with writePumpTickPeriod's 3s -> 2s. Named constant
// specifically because the previous code repeated the 5s literal at BOTH the
// initial SetReadDeadline call and inside SetPongHandler — fixing only one
// would have left the real tolerance unchanged while looking fixed. To
// tolerate N=2 fully-lost pings at cadence P=2s, the next usable pong can
// arrive as late as (N+1)*P + RTT + margin = 3*2000 + 500 + 500 = 7000ms
// (contracts/liveness-timing.md §4 "Justification du ReadDeadline serveur").
//
// Invariant, load-bearing, not incidental: deadLinkTimeout (4s, below) <
// readDeadlineTimeout (7s). The client is meant to detect and reconnect
// BEFORE the server gives up — a deliberate inversion from the pre-#130
// order (contract §4 "Ordre de détection"), because on a truly dead link the
// server's own close frame can never reach the client anyway; the existing
// anti-zombie guard (IsPlayerIDConnected, #109/#120) absorbs the stale
// server-side connection once the client's new one registers. A future
// change to either constant that collapses or reverses this inequality
// must be a conscious choice, not a side effect — hence naming both instead
// of leaving one as a literal.
const readDeadlineTimeout = 7 * time.Second

// deadLinkTimeout is the DEAD_LINK_TIMEOUT_MS value the server hands to web
// clients via HEARTBEAT (contracts/liveness-timing.md §2) — the absolute
// silence threshold, in milliseconds, beyond which a client should consider
// the link dead, close its socket, and reconnect. See readDeadlineTimeout's
// doc comment for the deadLinkTimeout < readDeadlineTimeout invariant this
// value is one half of.
//
// #130 GATE 2 adjustment: the plan's own recommended value was 5s (a 2s
// margin above the 3s network spike the spec requires absorbing without
// reconnecting); the user explicitly chose the more reactive 4s variant
// instead (margin reduced to 1s, detection at ~4.0-4.5s instead of
// ~5.0-5.5s) — see _work/handoff/task-dev-backend-20260804-090721.md.
const deadLinkTimeout = 4 * time.Second

func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(writePumpTickPeriod)
	defer func() {
		// Bugfix #131 — see readPump's identical guard above for the
		// rationale (recover() must be called directly in this literal).
		if r := recover(); r != nil {
			LogRecoveredPanic(game.LogComponentWebSocket, "writePump client="+c.ID, r)
		}
		ticker.Stop()
		c.Conn.Close()
	}()

	// Built once per connection, not per tick: the payload is constant for the
	// lifetime of the process (writePumpTickPeriod/deadLinkTimeout never
	// change at runtime). Marshal failure here is not expected (two fixed
	// int64 fields always encode), but is handled defensively — a nil
	// heartbeatData just skips the HEARTBEAT write below without affecting
	// the protocol ping.
	var heartbeatData []byte
	if heartbeatMsg, err := protocol.NewMessage(protocol.ActionHeartbeat, protocol.HeartbeatPayload{
		IntervalMs:        writePumpTickPeriod.Milliseconds(),
		DeadLinkTimeoutMs: deadLinkTimeout.Milliseconds(),
	}); err == nil {
		if data, err := heartbeatMsg.SerializeForWebSocket(); err == nil {
			heartbeatData = data
		} else {
			LogError(game.LogComponentWebSocket, "Failed to serialize HEARTBEAT: %v", err)
		}
	} else {
		LogError(game.LogComponentWebSocket, "Failed to build HEARTBEAT message: %v", err)
	}

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
			// #118 — Application-level HEARTBEAT, in addition to the protocol
			// ping frame above (never replacing it: the ping frame still feeds
			// the server's own PongHandler/read-deadline). Unlike the ping
			// frame, this is a plain TextMessage the browser's WebSocket
			// onmessage handler can actually observe — the ping/pong frames
			// are handled transparently below the JavaScript API and expose no
			// event a client could watch for a dead connection. No response is
			// expected: nothing reads this back in on the server side, so it
			// never touches the Incoming channel or its single consumer.
			if heartbeatData != nil {
				c.Conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
				if err := c.Conn.WriteMessage(websocket.TextMessage, heartbeatData); err != nil {
					return
				}
			}
		}
	}
}
