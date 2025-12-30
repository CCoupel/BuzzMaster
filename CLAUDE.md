# CLAUDE.md - BuzzControl Project Reference

## Project Overview

BuzzControl is a wireless buzzer system for quiz games. The system consists of:
- **BuzzControl**: Central server (currently ESP32-S3, migrating to Raspberry Pi + Go)
- **BuzzClick**: Individual buzzer clients (ESP32-C3, unchanged)

## Repository Structure

```
buzzcontrol/
├── server-go/                # NEW: Go server (Raspberry Pi)
│   ├── cmd/server/           # Entry point
│   ├── internal/
│   │   ├── config/           # Configuration
│   │   ├── server/           # HTTP, WebSocket, TCP, UDP, DNS
│   │   ├── game/             # Game engine and models
│   │   └── protocol/         # Message parsing
│   └── data/files/questions/ # Question storage
├── src/
│   ├── BuzzControl/          # Server firmware (ESP32-S3) - LEGACY
│   │   ├── MAIN.cpp          # Entry point
│   │   ├── WebServer.h       # HTTP/WebSocket server
│   │   ├── BumperServer.h    # Game logic
│   │   ├── tcpManager.h      # TCP server for buzzers
│   │   ├── SocketManager.h   # WebSocket handling
│   │   ├── teamsAndBumpers.h # Teams/buzzers data model
│   │   ├── messages_received.h
│   │   ├── messages_to_send.h
│   │   ├── fsManager.h       # LittleFS operations
│   │   ├── backupManager.h   # TAR backup/restore
│   │   ├── WifiManager.h     # WiFi AP setup
│   │   ├── DNS.h             # Captive portal DNS
│   │   └── buttonManager.h   # Physical buttons
│   ├── BuzzClick/            # Buzzer client firmware (ESP32-C3) - UNCHANGED
│   │   ├── click_MAIN.cpp
│   │   ├── click_serverConnection.h
│   │   └── click_WifiManager.h
│   └── Common/               # Shared code
│       ├── configManager.h   # Configuration persistence
│       ├── CustomLogger.h    # UDP logging
│       ├── led.h             # LED control
│       └── Constant.h
├── data/                     # Web assets (HTML/JS/CSS)
├── docs/
│   └── MIGRATION_ARCHITECTURE.md  # Architecture decisions
├── platformio.ini            # PlatformIO config
├── partitions.csv            # ESP32 partition table
└── CLAUDE.md                 # This file
```

## Migration Context

### Current State (ESP32) - LEGACY
- Platform: ESP32-S3 (BuzzControl), ESP32-C3 (BuzzClick)
- Framework: Arduino + PlatformIO
- Issues: Memory constraints (320KB RAM), stability problems, watchdog resets
- Status: **Replaced by Go server**

### Current State (Go) - ACTIVE
- Platform: Raspberry Pi 4 (production), Windows (development)
- Language: Go 1.21+
- Status: **Phase 1 Complete** - Full backward compatibility with BuzzClick v1
- Benefits: 2GB+ RAM, stable, native WiFi AP via hostapd

### Key Decision: BuzzClick clients remain unchanged
The ESP32-C3 buzzers connect to the Go server without any modification.

## Communication Protocols

### TCP Protocol (Buzzers <-> Server)
- Port: Configurable via `configManager.getControllerPort()`
- Format: JSON messages terminated by null byte (`\0`)
- Direction: Bidirectional

**Message structure:**
```json
{
  "ACTION": "HELLO|BUTTON|PING|PONG|...",
  "ID": "bumper_id",
  "MSG": { /* action-specific payload */ }
}
```

**Actions from BuzzClick to Server:**
| Action | Description | Payload |
|--------|-------------|---------|
| HELLO | Buzzer registration | `{VERSION, TEAM, NAME, ...}` |
| BUTTON | Button pressed | `{button: "A|B|C|D"}` |
| PONG | Response to PING | `{}` |

**Actions from Server to BuzzClick:**
| Action | Description | Payload |
|--------|-------------|---------|
| HELLO | Welcome/config | Game state |
| START | Game started | `{delay, question}` |
| STOP | Game stopped | `{}` |
| PAUSE | Pause buzzer | `{}` |
| CONTINUE | Resume game | `{}` |
| PING | Ready check | `{}` |
| RESET | Full reset | `{}` |

### WebSocket Protocol (Web clients <-> Server)
- Endpoint: `/ws`
- Format: Same JSON structure as TCP
- Used by: Admin interface, display screens

### HTTP REST API
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Redirect to main page |
| GET | `/version` | Server version |
| GET | `/listGame` | Current game state JSON |
| GET | `/questions` | List all questions |
| POST | `/questions` | Upload question (multipart) |
| GET | `/backup` | Download TAR backup |
| POST | `/restore` | Upload TAR restore |
| GET | `/config.json` | Get configuration |
| POST | `/config.json` | Update configuration |
| GET | `/clearGame` | Clear game data |
| GET | `/clearBuzzers` | Clear buzzers |
| GET | `/reboot` | Reboot server |
| GET | `/reset` | Factory reset |

## Data Models

### Game State
```json
{
  "PHASE": "STOP|PREPARE|READY|START|PAUSE",
  "DELAY": 30,
  "CURRENT_TIME": 25,
  "QUESTION": {
    "ID": "1",
    "QUESTION": "Question text",
    "ANSWER": "Answer text",
    "POINTS": 10,
    "TIME": 30,
    "MEDIA": "/question/1/media.jpg",
    "STATUS": "AVAILABLE|STARTED|STOPPED|REVEALED"
  },
  "PAGE": "GAME|SCORES|..."
}
```

### Teams and Bumpers
```json
{
  "teams": {
    "team_id": {
      "NAME": "Team Name",
      "COLOR": "#FF0000",
      "SCORE": 100,
      "TIME": 123456,
      "STATUS": "READY|PAUSE",
      "BUMPER": "winning_bumper_id"
    }
  },
  "bumpers": {
    "bumper_id": {
      "NAME": "Player Name",
      "TEAM": "team_id",
      "SCORE": 50,
      "TIME": 123456,
      "BUTTON": "A",
      "STATUS": "READY|PAUSE",
      "VERSION": "1.0.0"
    }
  }
}
```

### Question
```json
{
  "ID": "1",
  "QUESTION": "What is 2+2?",
  "ANSWER": "4",
  "POINTS": 10,
  "TIME": 30,
  "MEDIA": "/question/1/image.jpg"
}
```

## Game Flow

```
1. STOP (initial)
   └─► readyGame(questionId) ─► PREPARE
                                   │
2. PREPARE (waiting for buzzers)   │
   └─► All buzzers PONG ──────────►│
                                   ▼
3. READY (all buzzers ready)
   └─► startGame(delay) ─► START
                              │
4. START (timer running)      │
   ├─► Button pressed ───────►├─► PAUSE (buzzer paused)
   ├─► Timer = 0 ────────────►│
   └─► pauseAllGame() ───────►│
                              ▼
5. STOP (round ended)
   └─► revealGame() ─► Show answer
```

## Network Configuration

### Current (ESP32)
- SSID: "BuzzControl" (configurable)
- Server IP: 192.168.4.1
- DHCP range: 192.168.4.x
- TCP port: Configurable (default: 3000)
- HTTP port: 80
- WebSocket: /ws

### Target (Raspberry Pi)
Same network configuration, implemented via:
- `hostapd` for WiFi AP
- `dnsmasq` for DHCP + DNS
- Go server for HTTP/WebSocket/TCP

## Go Server Implementation (Phase 1 - Complete)

### Current Structure
```
server-go/
├── cmd/
│   └── server/
│       └── main.go           # Entry point + App orchestration
├── internal/
│   ├── config/
│   │   └── config.go         # Configuration management
│   ├── server/
│   │   ├── http.go           # HTTP + static files + question upload
│   │   ├── http_test.go
│   │   ├── websocket.go      # WebSocket server for web clients
│   │   ├── tcp.go            # TCP server for buzzers (null-terminated JSON)
│   │   ├── tcp_test.go
│   │   ├── udp.go            # UDP broadcast for time-critical messages
│   │   ├── udp_test.go
│   │   ├── dns.go            # Captive portal DNS server
│   │   ├── mdns.go           # mDNS service discovery
│   │   └── e2e_test.go
│   ├── game/
│   │   ├── models.go         # Teams, Bumpers, Questions data structures
│   │   ├── models_test.go
│   │   ├── engine.go         # Game state machine
│   │   └── engine_test.go
│   └── protocol/
│       ├── messages.go       # Message types (ACTION, MSG structure)
│       ├── messages_test.go
│       ├── parser.go         # Null-terminated JSON parsing
│       └── parser_test.go
├── data/
│   └── files/
│       └── questions/        # Question storage (mirrors ESP32 structure)
├── go.mod
└── go.sum
```

### Implemented Features (v1.0)

| Feature | Status | Notes |
|---------|--------|-------|
| HTTP server (port 80) | ✅ | Static files, REST API |
| WebSocket server (/ws) | ✅ | Web client communication |
| TCP server (port 1234) | ✅ | BuzzClick buzzer protocol |
| UDP broadcast (port 1235) | ✅ | Time-critical messages |
| DNS server (port 53) | ✅ | Captive portal |
| mDNS (_sock._tcp) | ✅ | Service discovery |
| Questions CRUD | ✅ | Upload, list, delete |
| Teams/Bumpers management | ✅ | Full state sync |
| Game state machine | ✅ | STOP→PREPARE→READY→START→PAUSE |
| TAR backup/restore | ✅ | /backup and /restore endpoints |
| Configuration | ✅ | JSON config file |

### Key Implementation Decisions

#### 1. HTTP /questions Response Format
Matches ESP32 format exactly:
```json
{
  "/files/questions/1": {
    "ID": "1",
    "QUESTION": "...",
    "ANSWER": "...",
    "POINTS": "10",
    "TIME": "30",
    "MEDIA": "/question/1/media_1234.jpg"
  },
  "/files/questions/2": { ... },
  "FSINFO": {
    "USED": "1234567",
    "FREE": "98765432",
    "TOTAL": "100000000",
    "P_USED": "1.2"
  }
}
```

#### 2. Media Filename Generation
Random 4-digit suffix like ESP32:
```go
randomNum := rand.Intn(9000) + 1000  // 1000-9999
fileName := fmt.Sprintf("media_%d%s", randomNum, ext)
```

#### 3. WebSocket Message Broadcasting
After question changes, broadcast to all clients:
```go
// OnQuestionUpload callback triggers broadcastQuestions()
httpServer.OnQuestionUpload = func() {
    app.broadcastQuestions()
}
```

#### 4. DELETE Action Implementation
Actually removes question directory:
```go
func (a *App) handleDelete(msg *protocol.Message) {
    var payload protocol.DeletePayload
    json.Unmarshal(msg.Msg, &payload)
    questionPath := filepath.Join(questionsDir, payload.ID)
    os.RemoveAll(questionPath)  // Delete directory and contents
    a.broadcastQuestions()      // Notify all clients
}
```

#### 5. Storage Info (FSINFO)
Cross-platform disk usage:
```go
func (h *HTTPServer) getStorageInfo() map[string]interface{} {
    var stat syscall.Statfs_t
    syscall.Statfs(h.dataDir, &stat)
    total := stat.Blocks * uint64(stat.Bsize)
    free := stat.Bavail * uint64(stat.Bsize)
    used := total - free
    return map[string]interface{}{
        "USED":   fmt.Sprintf("%d", used),
        "FREE":   fmt.Sprintf("%d", free),
        "TOTAL":  fmt.Sprintf("%d", total),
        "P_USED": fmt.Sprintf("%.1f", float64(used)*100/float64(total)),
    }
}
```

### Running the Server

```bash
# Development (Windows)
cd server-go
go build -v ./cmd/server
./server.exe

# Production (Raspberry Pi)
GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server
scp buzzcontrol pi@raspberrypi.local:~/
```

### Configuration File (config.json)
```json
{
  "server": {
    "http_port": 80,
    "tcp_port": 1234,
    "udp_port": 1235,
    "dns_port": 53
  },
  "storage": {
    "data_dir": "./data",
    "questions_dir": "./data/files/questions"
  }
}
```

---

## Go Migration Guidelines (Reference)

### Package Structure (original plan)

### Key Libraries
| Purpose | Library |
|---------|---------|
| HTTP router | `net/http` (stdlib) or `github.com/gin-gonic/gin` |
| WebSocket | `github.com/gorilla/websocket` |
| JSON | `encoding/json` (stdlib) |
| TAR | `archive/tar` (stdlib) |
| Config | `github.com/spf13/viper` or JSON file |

### Cross-compilation
```bash
# Windows (development)
go build -o buzzcontrol.exe ./cmd/server

# Raspberry Pi (production)
GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server
```

## Testing

### Manual Testing with BuzzClick
1. Start Go server on same network as ESP32-C3 buzzers
2. Configure buzzer to connect to server IP
3. Verify TCP connection and message exchange

### Expected Behavior
- Buzzer sends HELLO on connect
- Server responds with game state
- Button press triggers BUTTON message
- Server broadcasts updates via WebSocket

## Files Reference

### Critical Files to Port
| ESP32 File | Go Equivalent | Priority |
|------------|---------------|----------|
| BumperServer.h | internal/game/state.go | High |
| tcpManager.h | internal/server/tcp.go | High |
| WebServer.h | internal/server/http.go | High |
| SocketManager.h | internal/server/http.go | High |
| teamsAndBumpers.h | internal/game/teams.go, bumpers.go | High |
| messages_*.h | internal/protocol/messages.go | High |
| fsManager.h | internal/storage/files.go | Medium |
| backupManager.h | internal/storage/backup.go | Medium |
| configManager.h | config/config.go | Medium |

### Files NOT to Port (external to Go)
- WifiManager.h → hostapd configuration
- DNS.h → dnsmasq configuration
- led.h → Not applicable (or web UI indicator)

## Environment Variables / Configuration

```json
{
  "server": {
    "http_port": 80,
    "tcp_port": 3000,
    "websocket_path": "/ws"
  },
  "wifi": {
    "ssid": "BuzzControl",
    "password": "buzzcontrol123"
  },
  "game": {
    "default_delay": 30
  },
  "storage": {
    "data_dir": "./data",
    "questions_dir": "./data/questions",
    "backup_dir": "./data/backups"
  }
}
```

## Common Commands

### ESP32 (current)
```bash
# Build and upload
pio run -e buzzcontrol -t upload

# Monitor serial
pio device monitor -b 921600
```

### Go (target)
```bash
# Run locally
go run ./cmd/server

# Build for Pi
GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server

# Deploy to Pi
scp buzzcontrol pi@raspberrypi.local:~/
ssh pi@raspberrypi.local './buzzcontrol'
```

### Raspberry Pi Setup
```bash
# Install hostapd and dnsmasq
sudo apt install hostapd dnsmasq

# Configure (see docs/MIGRATION_ARCHITECTURE.md)
sudo systemctl enable hostapd
sudo systemctl enable dnsmasq

# Run server as service
sudo systemctl enable buzzcontrol
```

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

---

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

**Benefits of hybrid:**
- True simultaneity for time-critical game events
- Reliable delivery for configuration and state
- Built-in ping/pong for connection health on WebSocket
- Best of both worlds

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

---

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

---

## Compatibility Matrix

| Server Version | BuzzClick v1 (current) | BuzzClick v2 (proposed) |
|----------------|------------------------|-------------------------|
| ESP32 (current) | ✅ TCP+UDP | ❌ Not supported |
| Go v1 (Phase 1) | ✅ TCP+UDP | ❌ Not yet |
| Go v2 (Phase 2) | ✅ TCP+UDP | ✅ WebSocket |
| Go v3 (Phase 4) | ❌ Deprecated | ✅ WebSocket only |

---

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

---

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

---

## Notes for Claude

### Critical Requirements
- Phase 1 MUST maintain backward compatibility with current BuzzClick
- Never break existing deployments without migration path
- All protocol changes must be versioned
- Server must detect client version and adapt

### Implementation Notes
- JSON messages currently null-terminated (`\0`), keep for v1 compat
- Game state transitions must follow state machine exactly
- WebSocket broadcasts go to all connected clients
- Time values in microseconds for button press timing
- Scores are per-bumper AND per-team (aggregated)
- BuzzClick uses MAC address as unique ID
- mDNS service name is `_sock._tcp` for server discovery

### Testing Checklist
- [x] HTTP /questions endpoint matches ESP32 format
- [x] Question upload with media files
- [x] Question deletion via WebSocket DELETE action
- [x] FSINFO in /questions response
- [x] WebSocket broadcast after question changes
- [ ] Multiple buzzers connecting simultaneously
- [ ] Buzzer reconnection after server restart
- [ ] Button press timing accuracy (<10ms jitter)
- [ ] WebSocket + legacy protocol coexistence
- [ ] OTA update without bricking (Phase 3)
- [ ] Configuration persistence across reboots
