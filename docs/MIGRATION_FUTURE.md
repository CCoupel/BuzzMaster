# Migration et Améliorations Futures - BuzzControl

Ce document décrit les problèmes d'architecture actuels et les améliorations proposées pour les futures versions.

## Current Architecture Issues

### Identified Problems

| Problem | Location | Impact | Severity |
|---------|----------|--------|----------|
| **Dual communication channels** | BuzzClick | TCP to send, UDP broadcast to receive - complex and fragile | High |
| **Null-terminated JSON** | Protocol | Manual buffer management, overflow risks | High |
| **Hardcoded WiFi credentials** | click_includes.h | Cannot change network without reflashing | Medium |
| **Brutal reconnection** | click_WifiManager.h | `ESP.restart()` on failure, no backoff | Medium |
| **Unsynchronized timestamps** | Both | `micros()` relative to boot, not server time | Medium |
| **No OTA updates** | BuzzClick | Must flash via USB cable | Medium |
| **No message acknowledgment** | Protocol | Button press is fire-and-forget | Low |
| **No configuration mode** | BuzzClick | Cannot configure buttons, team, name | Low |

### Code Evidence

**Dual channels (click_serverConnection.h:119-123, 143-185):**
```cpp
// Send via TCP
void send_to_server(String msg) {
  client->write((msg + "\n").c_str(), msg.length() + 1);
}

// Receive via UDP broadcast
void onDataBroadcast(AsyncUDPPacket packet) {
  // Different channel for receiving
}
```

**Hardcoded credentials (click_includes.h:26-28):**
```cpp
const char* WIFI_SSID     = "buzzmaster";
const char* WIFI_PASSWORD = "BuzzMaster";
const int CONTROLER_PORT = 1234;
```

**Brutal restart (click_WifiManager.h:29-38):**
```cpp
if (!getServerIP()) {
    ESP.restart();  // No retry, just restart
}
```

## Proposed Improvements

### Protocol v2 (Breaking Changes for BuzzClick)

#### 1. Unified WebSocket Communication

Replace TCP + UDP broadcast with single WebSocket connection.

**Current:**
```
BuzzClick ──TCP──► BuzzControl
BuzzClick ◄──UDP broadcast── BuzzControl
```

**Proposed (Hybrid WebSocket + UDP):**
```
BuzzClick ◄──WebSocket──► BuzzControl (bidirectional, reliable)
          ◄──UDP Broadcast── BuzzControl (time-critical, simultaneous)
```

**Why hybrid:**
- UDP Broadcast guarantees simultaneous reception (single network packet)
- WebSocket sequential broadcast has ~0.5-1ms delay per client
- For 10 buzzers: ~5-10ms total delay between first and last
- Critical for START/STOP where all buzzers must react together

**Channel assignment:**

| Message | Channel | Reason |
|---------|---------|--------|
| START | UDP Broadcast | Simultaneity critical |
| STOP | UDP Broadcast | Simultaneity critical |
| UPDATE_TIMER | UDP Broadcast | Display sync |
| PAUSE | UDP Broadcast | Simultaneity |
| CONTINUE | UDP Broadcast | Simultaneity |
| HELLO | WebSocket | Individual, needs ACK |
| PONG | WebSocket | Individual response |
| BUTTON | WebSocket | Individual, needs ACK |
| CONFIG | WebSocket | Individual, reliable |
| UPDATE (full state) | WebSocket | Large payload, reliability |
| OTA_* | WebSocket | Individual, reliable |

#### 2. Message Framing

Replace null-terminated JSON with length-prefixed messages.

**Current:**
```
{"ACTION":"BUTTON",...}\0
```

**Proposed (Option A - Length prefix):**
```
[4 bytes: length][JSON payload]
```

**Proposed (Option B - Newline delimited, simpler):**
```
{"ACTION":"BUTTON",...}\n
```

#### 3. Message Acknowledgment

Add sequence numbers and ACKs for critical messages.

```json
{
  "seq": 12345,
  "ACTION": "BUTTON",
  "MSG": {"button": "A"}
}

// Server response
{
  "ack": 12345,
  "ACTION": "BUTTON_ACK",
  "MSG": {"status": "OK", "timestamp": 1234567890}
}
```

#### 4. Time Synchronization

Add server-relative timestamps.

```json
// Server sends periodically
{
  "ACTION": "TIME_SYNC",
  "MSG": {"server_time": 1234567890123}
}

// BuzzClick calculates offset
offset = server_time - local_micros()

// Button press uses synchronized time
{
  "ACTION": "BUTTON",
  "MSG": {"button": "A", "time": 1234567890456}
}
```

### BuzzClick Firmware Improvements

#### 1. WiFi Configuration Mode

Add BLE or AP mode for initial setup.

```cpp
// On boot, if no saved config or button held:
// 1. Create AP "BuzzClick-XXXX"
// 2. Serve configuration page
// 3. Save to NVS (non-volatile storage)
// 4. Reboot and connect to saved network
```

**Configuration options:**
- WiFi SSID / password
- Server IP (or use mDNS)
- Buzzer name
- Button configuration
- Team assignment

#### 2. OTA Updates

Enable firmware updates via server.

```json
// Server announces update
{
  "ACTION": "OTA_AVAILABLE",
  "MSG": {
    "version": "2.0.0",
    "url": "http://server/firmware/buzzclick-2.0.0.bin",
    "checksum": "sha256:..."
  }
}

// BuzzClick can accept or defer
{
  "ACTION": "OTA_ACCEPT",
  "MSG": {"version": "2.0.0"}
}
```

#### 3. Exponential Backoff Reconnection

Replace brutal restart with smart retry.

```cpp
int retryDelay = 1000;  // Start at 1 second
const int maxDelay = 60000;  // Max 1 minute

while (!connected) {
    if (tryConnect()) {
        retryDelay = 1000;  // Reset on success
        break;
    }
    delay(retryDelay);
    retryDelay = min(retryDelay * 2, maxDelay);
}
```

#### 4. Status LED Patterns

Standardize LED feedback.

| State | Pattern |
|-------|---------|
| Booting | White pulse |
| Connecting WiFi | Blue blink |
| Connecting server | Yellow blink |
| Connected, idle | Team color, dim |
| Game ready | Team color, bright |
| Game active | Progress animation |
| Button pressed | Flash white |
| Disconnected | Red blink |
| Config mode | Rainbow cycle |

### Server (Go) Improvements

#### 1. Connection Management

```go
type BuzzerConnection struct {
    ID          string
    Conn        *websocket.Conn
    Team        string
    LastPing    time.Time
    LastPong    time.Time
    Version     string
    MessageSeq  uint32
}

// Heartbeat goroutine per connection
func (bc *BuzzerConnection) heartbeat() {
    ticker := time.NewTicker(5 * time.Second)
    for range ticker.C {
        if time.Since(bc.LastPong) > 15*time.Second {
            bc.handleDisconnect()
            return
        }
        bc.sendPing()
    }
}
```

#### 2. Message Bus

```go
type MessageBus struct {
    subscribers map[string][]chan Message
    mu          sync.RWMutex
}

// Subscribe to specific actions
bus.Subscribe("BUTTON", func(msg Message) {
    // Handle button press
})

// Broadcast to all buzzers
bus.Broadcast(Message{Action: "START", ...})

// Send to specific buzzer
bus.SendTo(buzzerID, Message{...})
```

## Migration Phases

### Phase 1: Go Server (Backward Compatible)
- Implement Go server with current protocol
- Support both TCP and UDP broadcast
- No changes to BuzzClick
- Deploy on Raspberry Pi

### Phase 2: WebSocket Support (Optional for BuzzClick)
- Add WebSocket endpoint to Go server
- BuzzClick can optionally use WebSocket
- Keep TCP+UDP for old firmware
- Gradual migration

### Phase 3: BuzzClick v2 Firmware
- WebSocket only
- Configuration mode
- OTA support
- Time synchronization
- New LED patterns

### Phase 4: Deprecate Legacy Protocol
- Remove TCP+UDP from server
- All buzzers on WebSocket
- Simplified codebase

## Compatibility Matrix

| Server Version | BuzzClick v1 (current) | BuzzClick v2 (proposed) |
|----------------|------------------------|-------------------------|
| ESP32 (current) | TCP+UDP | Not supported |
| Go v1 (Phase 1) | TCP+UDP | Not yet |
| Go v2 (Phase 2) | TCP+UDP | WebSocket |
| Go v3 (Phase 4) | Deprecated | WebSocket only |

## New Protocol Specification (v2)

### WebSocket Endpoint
```
ws://server:80/ws/buzzer
```

### Message Format
```json
{
  "seq": 12345,           // Sequence number (for ACK)
  "ts": 1234567890123,    // Server-relative timestamp (microseconds)
  "ACTION": "ACTION_NAME",
  "ID": "buzzer_mac",     // Only for buzzer->server
  "VERSION": "2.0.0",     // Only for buzzer->server
  "MSG": {}               // Action-specific payload
}
```

### New Actions

| Action | Direction | Description |
|--------|-----------|-------------|
| TIME_SYNC | Server→Buzzer | Periodic time synchronization |
| BUTTON_ACK | Server→Buzzer | Acknowledge button press |
| CONFIG | Server→Buzzer | Push configuration changes |
| OTA_AVAILABLE | Server→Buzzer | Firmware update available |
| OTA_ACCEPT | Buzzer→Server | Accept OTA update |
| OTA_PROGRESS | Buzzer→Server | OTA download progress |
| STATUS | Buzzer→Server | Periodic status report (battery, signal, etc.) |

## Configuration Schema (BuzzClick v2)

Stored in NVS (Non-Volatile Storage):

```json
{
  "wifi": {
    "ssid": "BuzzControl",
    "password": "password123",
    "use_mdns": true,
    "server_ip": "",
    "server_port": 80
  },
  "buzzer": {
    "name": "Player 1",
    "team": "",
    "buttons": [
      {"pin": 6, "name": "RED", "enabled": true},
      {"pin": 7, "name": "GREEN", "enabled": true},
      {"pin": 8, "name": "BLUE", "enabled": true},
      {"pin": 9, "name": "YELLOW", "enabled": true}
    ]
  },
  "display": {
    "brightness": 128,
    "idle_animation": "breathe"
  },
  "ota": {
    "auto_update": false,
    "channel": "stable"
  }
}
```
