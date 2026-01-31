# Protocoles de Communication - BuzzControl

Ce document décrit les protocoles de communication entre les différents composants du système BuzzControl.

## TCP Protocol (Buzzers <-> Server)

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
{ "INDEX": 0 }  // 0-based index into backgrounds array
```

**How it works:**
- Server maintains `CurrentBackgroundIndex` in GameState
- Goroutine cycles through backgrounds based on each image's duration
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

### Selective Backup (`/backup-select`)

Query parameters (all boolean, default: `true` if none specified):
- `questions=true` - Include questions directory
- `teams=true` - Include teams.json
- `bumpers=true` - Include bumpers.json
- `history=true` - Include history.json
- `backgrounds=true` - Include backgrounds directory

Example: `/backup-select?questions=true&history=true`

### Selective Reset (`/reset-select`)

Query parameters (all boolean):
- `all=true` - Reset everything
- `questions=true` - Delete all questions
- `teams=true` - Clear teams data
- `bumpers=true` - Clear bumpers data
- `history=true` - Clear history
- `backgrounds=true` - Delete backgrounds

Example: `/reset-select?history=true&bumpers=true`

### Intelligent Restore (`/restore`)

The restore endpoint now automatically detects what's in the TAR archive and restores accordingly:
- Detects `files/questions/*` → restores questions
- Detects `config/teams.json` → loads teams into engine
- Detects `config/bumpers.json` → loads bumpers into engine
- Detects `config/history.json` → loads history and recalculates scores
- Detects `files/backgrounds/*` → restores backgrounds

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
| UDP | 1234 | Broadcast (same as TCP) |
| DNS | 53 | Captive portal (optional) |
