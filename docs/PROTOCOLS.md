# Protocoles de Communication - BuzzControl

Ce document décrit les protocoles de communication entre les différents composants du système BuzzControl.

## TCP Protocol (Buzzers <-> Server) — **DEPRECATED (v5.1.1+)**

⚠️ **SUPPRIMÉ depuis v5.1.1** — Le TCPServer legacy (port 1234 TCP) a été supprimé car c'était du dead code depuis v3.0.0. Tous les BuzzClick utilisent désormais WebSocket (`/ws/buzzer`). Le TCPServer bloquait le démarrage sur Windows (WSAEADDRINUSE). **Veuillez utiliser WebSocket** (voir section suivante).

**Historique** (pour référence) :
- Port: 1234 (TCP) — **SUPPRIMÉ**
- Format: JSON messages terminated by null byte (`\0`)
- Direction: Bidirectional

Les buzzers modernes utilisent **WebSocket Protocol - Buzzers** (v3.0.0+) ci-dessous.

## WebSocket Protocol - Buzzers (v3.0.0+)

- Endpoint: `/ws/buzzer`
- Format: JSON standard (no null terminator)
- Used by: Physical buzzers (BuzzClick ESP32-C3)
- Documentation complete: [docs/WEBSOCKET_PROTOCOL.md](WEBSOCKET_PROTOCOL.md)

**Mode hybride** : Le serveur supporte TCP (port 1234) et WebSocket (`/ws/buzzer`) simultanement. Les buzzers anciens (TCP) et nouveaux (WebSocket) coexistent sans conflit.

## WebSocket Protocol (Web clients <-> Server)

- Endpoint: `/ws`
- Format: Same JSON structure as TCP
- Used by: Admin interface, display screens

**WebSocket Actions (Web client to Server):**
| Action | Description | Payload |
|--------|-------------|---------|
| HELLO | Client registration | `{}` |
| START | Start game round | `{DELAY: seconds}` |
| STOP | Stop game round | `{}` |
| PAUSE | Pause game | `{}` |
| CONTINUE | Resume game | `{}` |
| READY | Select question | `{QUESTION: questionId}` |
| REVEAL | Show answer | `{}` |
| REMOTE | Change TV display | `{REMOTE: "GAME\|SCORE\|PLAYERS\|PALMARES"}` |
| RAZ | Reset all scores | `{}` |
| DELETE | Delete question | `{ID: questionId}` |
| UPDATE | Update teams/bumpers | `{teams: {...}, bumpers: {...}}` |
| TEAM_POINTS | Modify team score | `{TEAM: teamName, POINTS: delta}` |
| BUMPER_POINTS | Modify player score | `{ID: bumperMac, POINTS: delta}` |
| REORDER_QUESTIONS | Reorder questions | `{ORDER: [questionId1, questionId2, ...]}` |

### WebSocket Actions for Memory Game (v2.33.0+)

| Action | Description | Payload | Version |
|--------|-------------|---------|---------|
| FLIP_MEMORY_CARD | Retourne une carte Memory | `{CARD_ID: "1-1"}` | v2.33.0 |
| MEMORY_SET_TEAMS | Définit les équipes participantes (phase PREPARE) | `{TEAMS: ["Équipe Rouge", "Équipe Bleue"]}` | v2.51.0 |

**FLIP_MEMORY_CARD :**
- Phase : STARTED
- Type question : MEMORY
- CARD_ID format : "pairID-cardNum" (ex: "1-1" pour paire 1 carte 1, "3-2" pour paire 3 carte 2)
- Validation : En mode multi-équipes, vérifie que l'action provient de l'équipe courante

**MEMORY_SET_TEAMS :**
- Phase : PREPARE
- Type question : MEMORY avec MEMORY_MODE = "CHACUN_SON_TOUR" ou "TANT_QUE_JE_GAGNE"
- Validation : Minimum 2 équipes requises pour modes multi-équipes, 1 pour mode SOLO
- Effet : Initialise MEMORY_CURRENT_TEAM, MEMORY_TEAM_PAIRS, MEMORY_PARTICIPATING_TEAMS
- Envoyé automatiquement avant START_GAME

### WebSocket Client Types (v2.47.0+)

VJoueurs (joueurs virtuels) sont identifiés avec un type de client distinct via `SET_CLIENT_TYPE`.

**Types de clients** :
| Type | Description | Identification | Compteur |
|------|-------------|-----------------|----------|
| admin | Interface d'administration | App.jsx : route /admin | ADMIN_COUNT |
| tv | Affichage TV/joueurs | App.jsx : route /tv | TV_COUNT |
| vplayer | Joueur virtuel (WebSocket) | EnrollPage + VPlayerPage | VPLAYER_COUNT |

**Flux d'identification VJoueur** :
```
1. Joueur → http://localhost (EnrollPage)
2. EnrollPage.handleSubmit() :
   - Appelle setClientType('vplayer')
   - Appelle connectVirtualPlayer(name)
3. Serveur reçoit SET_CLIENT_TYPE { TYPE: "vplayer" }
4. Backend crée ClientTypeVPlayer
5. Broadcast CLIENTS { admin, tv, vplayer }
6. VPlayerPage au montage :
   - Appelle setClientType('vplayer') (confirmation)
7. Navbar affiche 3 compteurs distincts
```

**Action CLIENTS mise à jour (v2.47.0+)** :
```json
{
  "ACTION": "CLIENTS",
  "MSG": {
    "ADMIN_COUNT": 2,
    "TV_COUNT": 1,
    "VPLAYER_COUNT": 5
  }
}
```

### WebSocket Actions for Client Management

| Action | Direction | Description |
|--------|-----------|-------------|
| SET_CLIENT_TYPE | Client→Server | Set client type (admin/tv/vplayer) |
| CLIENTS | Server→Client | Broadcast client counts |

**SET_CLIENT_TYPE payload:**
```json
{ "TYPE": "admin" }  // or "tv" or "vplayer"
```

### Background Image Synchronization (v2.30.0)

The server centralizes background image cycling to ensure all TV displays show the same image simultaneously.

| Action | Direction | Description |
|--------|-----------|-------------|
| BACKGROUND_CHANGE | Server→Client | Broadcast current background index |

**BACKGROUND_CHANGE payload:**
```json
{ "INDEX": 0 }  // 0-based index into medias array
```

**How it works:**
- Server maintains `CurrentBackgroundIndex` in GameState
- Goroutine cycles through medias based on each image's duration
- On each cycle, server broadcasts `BACKGROUND_CHANGE` to all clients
- Clients use the server-provided index instead of local cycling

## HTTP REST API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Redirect to main page |
| GET | `/version` | Server version |
| GET | `/listGame` | Current game state JSON |
| GET | `/questions` | List all questions |
| POST | `/questions` | Upload question (multipart) |
| GET | `/history` | Get game event history |
| GET | `/backup` | Download full TAR backup |
| GET | `/backup-select` | Selective backup (see params below) |
| POST | `/restore` | Intelligent restore from TAR |
| GET | `/reset-select` | Selective reset (see params below) |
| GET | `/config.json` | Get configuration |
| POST | `/config.json` | Update configuration |
| GET | `/clearGame` | Clear game data |
| GET | `/clearBuzzers` | Clear buzzers |
| GET | `/reboot` | Reboot server |
| GET | `/reset` | Factory reset |
| GET | `/shutdown` | Graceful server shutdown |

### Categories API (v5.7.0)

```
GET /api/categories
```

Retourne la liste fusionnée des catégories hardcodées et des catégories personnalisées déposées dans `data/files/categories/`.

**Response :**
```json
[
  { "key": "GEOGRAPHY",     "label": "Geography",     "custom": false },
  { "key": "SPORT_EXTREME", "label": "Sport Extreme",  "custom": true, "image": "/files/categories/Sport Extreme.png" }
]
```

**Règles de nommage (catégories custom) :**
- Fichier `Sport Extreme.png` → clé `SPORT_EXTREME` (espaces → `_`, uppercase)
- Formats acceptés : PNG, JPG, JPEG, WEBP
- Répertoire : `data/files/categories/`
- Inclus dans le backup/restore via le flag `medias`

---

### Selective Backup (`/backup-select`)

Query parameters (all boolean, default: `true` if none specified):
- `questions=true` - Include questions directory
- `teams=true` - Include teams.json
- `bumpers=true` - Include bumpers.json
- `history=true` - Include history.json
- `medias=true` - Include medias directory

Example: `/backup-select?questions=true&history=true`

### Selective Reset (`/reset-select`)

Query parameters (all boolean):
- `all=true` - Reset everything
- `questions=true` - Delete all questions
- `teams=true` - Clear teams data
- `bumpers=true` - Clear bumpers data
- `history=true` - Clear history
- `medias=true` - Delete medias

Example: `/reset-select?history=true&bumpers=true`

### Intelligent Restore (`/restore`)

The restore endpoint now automatically detects what's in the TAR archive and restores accordingly:
- Detects `files/questions/*` → restores questions
- Detects `config/teams.json` → loads teams into engine
- Detects `config/bumpers.json` → loads bumpers into engine
- Detects `config/history.json` → loads history and recalculates scores
- Detects `files/medias/*` → restores medias

### HTTP /questions Response Format

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

## UDP Broadcast Server Discovery (v3.2.0)

The server sends periodic UDP heartbeat messages so BuzzClick buzzers can discover the server IP automatically, without requiring manual configuration of a static server address.

### Heartbeat Format

```
BUZZ_SERVER|<IP1>|<IP2>|...|<PORT>\0
```

- Null-terminated (`\0`) for consistency with the existing TCP/UDP protocol convention
- The server includes **all active IPv4 addresses** (one per network interface), excluding loopback (127.x.x.x) and link-local (169.254.x.x)
- `PORT` is the HTTP server port (default: 80; use 443 for HTTPS)

**Examples:**

```
BUZZ_SERVER|192.168.1.50|80\0                              # Single interface
BUZZ_SERVER|192.168.1.50|10.0.0.50|80\0                   # Two interfaces
BUZZ_SERVER|192.168.1.50|192.168.4.1|10.0.0.50|443\0      # Three interfaces, HTTPS
```

### Server Side (BroadcasterManager)

- Sends heartbeats via UDP broadcast to all active network interface broadcast addresses (e.g. `192.168.1.255`, `10.0.0.255`)
- Falls back to `192.168.4.255` if no interfaces are found
- **Normal mode**: heartbeat every **5 seconds**
- **Enrollment mode**: heartbeat every **1 second** (triggered during buzzer pairing)
- Sends an immediate heartbeat at startup so buzzers connect quickly
- Implemented in `server-go/internal/server/broadcaster.go` (`BroadcasterManager`)
- UDP socket for sending is managed by `server-go/internal/server/udp.go` (`UDPBroadcaster`)

### Buzzer Side (BuzzClick Firmware v3.2.0)

The buzzer listens for heartbeats on port 1234 (AsyncUDP) immediately after obtaining a WiFi IP address.

**Multi-IP failover**: when a heartbeat is received, the buzzer tries each IP in order until one connects successfully. If all IPs fail, it resets and waits for the next heartbeat.

**Fallback chain** (in order):
1. **UDP Broadcast** — primary method, IPs from heartbeat (30s timeout)
2. **NVS stored IP** — uses `server_ip` saved in flash from previous session
3. **mDNS** — last resort, queries `_sock._tcp` service
4. If all fallbacks fail: reset discovery and retry broadcast

Implemented in:
- `src/BuzzClick/click_broadcaster.h` — UDP listener, heartbeat parser, discovery state
- `src/BuzzClick/click_WifiManager.h` — boot sequence integration, fallback chain

### Boot Sequence LED Indicators (v3.2.0)

| Phase | LED | Condition |
|-------|-----|-----------|
| 1 | White 1/4 | Power on / init |
| 2 | Red 1/4 | WiFi connecting |
| 3 | Orange 1/4 | WiFi connected, IP obtained |
| **4** | **Yellow pulsing (2 Hz)** | **Waiting for UDP broadcast heartbeat** |
| **5** | **Blue rapid blink** | **Trying each discovered IP** |
| 6 | Green 2/4 (TCP mode only) | Server connected |

Phases 4 and 5 are new in v3.2.0. Phase 4 pulses between bright yellow (`255,200,0`) and dim yellow (`64,50,0`). Phase 5 blinks between bright blue (`0,0,255`) and dim blue (`0,0,64`) for each connection attempt.

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

### Standard Ports

| Service | Port | Description |
|---------|------|-------------|
| HTTP | 80 | Web interface |
| TCP | 1234 | BuzzClick buzzer protocol |
| UDP | 1234 | Broadcast server discovery heartbeat + game broadcast |

---

## UDP Broadcast — Server Discovery (v3.2.0)

**Format heartbeat** : `BUZZ_SERVER|IP1|IP2|...|PORT\0` (null-terminé, multi-interfaces)

Intervalles : enrollment (1 s) > high-freq (500 ms si ≥1 buzzer déconnecté) > normal (5 s).

```go
// server-go/internal/server/broadcaster.go
bm := NewBroadcasterManager(udpBroadcaster, httpPort)
bm.SetEnrollmentMode(true)  // 1s pendant appairage
bm.SetHighFrequency(true)   // 500ms si buzzer déconnecté (v3.6.6)
```

**Chaîne fallback firmware** (`click_WifiManager.h`) :
1. UDP Broadcast (timeout 30 s) → IPs du heartbeat
2. IP NVS (`server_ip` en flash)
3. mDNS (`_sock._tcp`)
4. Retry broadcast

Phases LED boot : phase 4 = jaune pulsant (attente heartbeat), phase 5 = bleu clignotant (tentative connexion).

---

## Default Question Image (v3.2.3)

Image affichée sur TV pour les questions SPEEDY/QCM sans média.

```
GET    /api/config/default-image  → Sert l'image (custom ou SVG embarqué fallback)
POST   /api/config/default-image  → Upload (multipart, champ "file", jpg/png/gif/webp/svg)
DELETE /api/config/default-image  → Supprime custom, retour SVG embarqué
```

- Asset embarqué : `server-go/assets/default-question-image.svg` (via `//go:embed`)
- `CONFIG_UPDATE` broadcast : `default_question_image_is_custom: true/false`
- Cache-busting frontend : `?t=timestamp`

---

## OTA Firmware Endpoints (v3.1.0+)

```
GET  /api/firmware/buzzclick/version           → FirmwareVersionPayload
GET  /api/firmware/buzzclick/latest.bin        → Binaire firmware
POST /api/firmware/buzzclick/upload            → Upload .bin (multipart, champ "file")
POST /api/firmware/buzzclick/restore-embedded  → Restaure firmware embarqué (v3.1.1)
POST /api/buzzer/{mac}/update                  → Déclenche OTA sur buzzer spécifique
POST /api/buzzer/update-all                    → OTA sur tous les buzzers obsolètes
POST /api/buzzer/wifi-config                   → Broadcast WIFI_CONFIG à tous les buzzers WS
```

**Firmware embarqué** : `server-go/assets/firmware/buzzclick-latest.bin` (`//go:embed`). `EMBEDDED_VERSION` exposé dans `FirmwareVersionPayload`.

Actions WebSocket OTA :
```json
{ "ACTION": "OTA_UPDATE",    "MSG": { "URL": "...", "VERSION": "3.1.1", "SIZE": 512000 } }
{ "ACTION": "OTA_PROGRESS",  "MSG": { "STATUS": "downloading", "PERCENT": 42, "ERROR": "" } }
{ "ACTION": "FIRMWARE_VERSION", "MSG": { "VERSION": "3.1.1", "EXISTS": true, "EMBEDDED_VERSION": "3.1.1" } }
{ "ACTION": "WIFI_CONFIG",   "MSG": { "SSID": "...", "PASS": "...", "SERVER_IP": "...", "SERVER_PORT": 80 } }
```

Fichiers : `internal/server/firmware.go` (FirmwareManager), `internal/server/http_firmware.go` (handlers), `src/BuzzClick/click_otaManager.h` (`performOTA()` dans FreeRTOS task 16 KB).
| DNS | 53 | Captive portal (optional) |
