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

### WebSocket Heartbeat (Keep-alive) — v5.9.1+ (extended in v5.10.x)

The server emits a `HEARTBEAT` message periodically to all web clients (`/ws/admin`, `/ws/tv`, `/ws/player`) as a keep-alive signal and to transmit liveness detection parameters. The client uses this to detect dead links and trigger automatic reconnection.

**Message format** (sent by server, no response expected):
```json
{
  "ACTION": "HEARTBEAT",
  "MSG": {
    "INTERVAL_MS": 2000,
    "DEAD_LINK_TIMEOUT_MS": 4000
  }
}
```

| Field | Type | Since | Description |
|---|---|---|---|
| `INTERVAL_MS` | integer (ms) | v5.9.1 (#118) | Actual server ticker cadence — client uses this as a reference for keep-alive calculations. **Current value: 2000 ms.** |
| `DEAD_LINK_TIMEOUT_MS` | integer (ms) | v5.10.x (#130, GATE 2) | **New in #130.** Absolute silence threshold — if client receives no messages (any kind, including `HEARTBEAT` itself) for this duration, it must consider the link dead, close the socket, and reconnect. Server directly controls the seuil, not dependent on client-side derivation. **Current value: 4000 ms** (détection 4,0–4,5 s effective). |

**Server parameters** (current values after #130 GATE 2 adjustment):

| Parameter | Valeur | Role |
|---|---|---|
| Ping + `HEARTBEAT` cadence | 2000 ms | Server tick rate — repeated to all web clients |
| Server `ReadDeadline` | 7000 ms | Server closes connection if no Pong within 7 s ; tolerates 2 lost pings at 2 s cadence |
| Client detection threshold | 4000 ms | Sent via `DEAD_LINK_TIMEOUT_MS` ; client closes socket if silence ≥ 4 s |
| Effective dead-link detection | 4,0 – 4,5 s | Client acts first (deliberate inversion from #118) to reclaim initiative on truly dead links |
| Client check granularity | 500 ms | Client re-evaluates silence counter every 500 ms (smoother than 1 s) |

**Client-side rule** (cascade with fallback):
```
threshold = DEAD_LINK_TIMEOUT_MS              if field received from server
          = INTERVAL_MS × 3                   if HEARTBEAT received, but DEAD_LINK_TIMEOUT_MS absent (old server)
          = 3000 × 3 = 9000 ms                if neither field received yet (initial default, v5.9.1 behavior)
```

The silence counter is **reset by any incoming message**, including `HEARTBEAT` itself — surveillance is passive; the client does not emit its own heartbeat.

**Backward compatibility** :
- Old client + new server : field ignored, threshold = `INTERVAL_MS × 3` = 6000 ms ✓
- New client + old server : field absent, repli on `INTERVAL_MS × 3` = 9000 ms ✓
- Clients pre-v5.9.1 : no `HEARTBEAT` received, default 9000 ms ✓

### VPlayer Eviction Reasons — `PLAYER_EVICTED` and `PLAYER_REJECTED` (v5.9.0+, extended v5.10.x #134)

When a VJoueur is removed from the game, the server sends a `PLAYER_EVICTED` message (to the disconnected player) or `PLAYER_REJECTED` response (to a reconnection attempt with a stale ID) with a `REASON` field. The client displays a banner explaining why the player was removed.

**Message format** (server → VJoueur):
```json
{
  "ACTION": "PLAYER_EVICTED",
  "MSG": {
    "REASON": "<reason_code>"
  }
}
```

**Eviction reasons and meanings** :

| Reason | Introduced | Trigger | Score | Seat | Reprise possible |
|---|---|---|---|---|---|
| `ENROLLMENT_CLOSED` | v5.9.0 (#120) | Enrollments closed while player connecting | Preserved | Blocked | Non (inscriptions fermées) |
| `PLAYER_REMOVED` | v5.9.0 (#120) | Admin deleted player (action `DELETE_BUMPER`) | **Lost** | **Freed completely** | Oui (nouvel enregistrement à zéro) |
| `GAME_RESET` | v5.9.0 (#120) | Game reinitialized (NEW_GAME action) | Lost | Freed completely | Oui (nouvel enregistrement) |
| `SESSION_EXPIRED` | v5.9.0 (#120) | Player bumper vanished from roster after ~10s (internal safety net) | Lost | Freed completely | Oui (nouvel enregistrement) |
| `SEAT_RELEASED` | v5.10.x (#134) | Admin liberated seat (action `RELEASE_BUMPER_NAME` on connected player) | **Preserved** | **Freed for immediate reprise** | **Oui** — may rejoin with same name within ~5 min authorization window |

**Key distinction** :
- **`PLAYER_REMOVED`** (delete) vs. **`SEAT_RELEASED`** (liberate) :
  - Both free the seat for others
  - `PLAYER_REMOVED` : score, team, history all deleted
  - `SEAT_RELEASED` : score, team, history preserved — player can rejoin same slot with `RECONNECT` path (#122) and reclaim their seat within authorization window

**Message format for rejected reconnection** (server response to stale ID):
```json
{
  "ACTION": "PLAYER_REJECTED",
  "MSG": {
    "REASON": "<reason_code>"
  }
}
```

**Rejected connection reasons** :
| Reason | Meaning |
|---|---|
| `ENROLLMENT_CLOSED` | Enrollments not open; retry after opening |
| `INVALID_NAME` | Name doesn't match rules (2–20 chars, etc.) |
| `NAME_TAKEN` | Name already assigned to connected player; choose different name or wait for reconnect window |
| `LIMIT_REACHED` | Max player count reached; retry after others leave |
| `SEAT_RELEASED` | Your ID was released by admin; rejoin without ID within authorization window (~5 min) to reclaim seat with your score |

**Client-side behavior** :
- Receive `PLAYER_EVICTED` or `PLAYER_REJECTED` with `REASON`
- Look up reason in localization table (`REDIRECT_MESSAGES` or `REJECTION_MESSAGES`)
- Display banner with message + icon (varies by reason)
- Auto-redirect to enrollment page after 3 seconds (configurable `RECONNECT_ERROR_REDIRECT_DELAY_MS`)
- Fallback : if reason unknown, display generic "Connection error" message (backward compatibility)

---

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

---

## Endpoints HTTP — Mode RAFALE (v8.0.0, #16)

Tous les endpoints RAFALE utilisent le **protocole JSON**, pas multipart (les questions sont texte seul, pas de média).

### GET /api/rafale/questions

Lister le réservoir avec filtres optionnels.

**Paramètres** (optionnels) :
- `categories` (query) : liste virgule-séparée (ex: `HISTORY,SCIENCE`)
- `difficulty` (query) : 1–3

**Réponse 200** :
```json
{
  "QUESTIONS": [
    {
      "ID": "r-001",
      "QUESTION": "Capitale de l'Italie ?",
      "ANSWER": "Rome",
      "CATEGORY": "GEOGRAPHY",
      "DIFFICULTY": 1,
      "USED": false
    }
  ],
  "TOTAL": 1
}
```

**Note** : champ `USED` est **dérivé à la lecture** depuis `rafale_used.json` (jamais stocké dans le réservoir).

### POST /api/rafale/questions

Créer ou modifier une question.

**Corps** :
```json
{
  "ID": "r-001",                    // omis pour création (serveur génère l'ID)
  "QUESTION": "Capitale de l'Italie ?",
  "ANSWER": "Rome",
  "CATEGORY": "GEOGRAPHY",
  "DIFFICULTY": 1
}
```

**Réponse 200** :
```json
{ "ID": "r-001" }
```

**Erreurs** :
- `400` : énoncé/réponse vide, `DIFFICULTY` hors 1–3, catégorie inconnue
- `409` : ID déjà existant (création avec ID explicite non autorisée)

### DELETE /api/rafale/questions/{id}

Supprimer une question du réservoir.

**Réponse 200** :
```json
{ "DELETED": "r-001" }
```

**Erreurs** :
- `404` : question non trouvée

**Note** : la suppression **n'efface pas** le flag `used[r-001]`. Une question supprimée ne réapparaît jamais tant que le flag existe.

### POST /api/rafale/questions/{id}/reset

Remet **une seule** question du réservoir à l'état disponible (retire son ID du flag « déjà utilisée »). No-op silencieux si la question n'était pas marquée utilisée. Corps : vide.

**Réponse 200** :
```json
{ "ID": "r-001", "AVAILABLE": true }
```

**Erreurs** :
- `404` : ID absent du réservoir

**Cas d'usage** : l'animateur/admin peut remettre une question précise en disponible sans refaire un `NEW_GAME` complet.

### POST /api/rafale/questions/reset-all

Remet **tout** le réservoir à l'état disponible (vide complètement le flag « déjà utilisée »), indépendamment d'un `NEW_GAME`. Le réservoir lui-même (les questions) n'est **jamais** touché. À ne pas confondre avec `POST /reset-select?rafale=true` (sauvegarde/restauration), qui **supprime** tout le réservoir en plus du flag. Corps : vide.

**Réponse 200** :
```json
{ "RESET": 42 }
```

Où `42` = nombre d'entrées effacées du flag.

**Cas d'usage** : réinitialiser tout le pool de questions pour une nouvelle série de manches (sans `NEW_GAME`).

### GET /api/rafale/pool

Comptage pré-manche (pour alertes).

**Paramètres** (optionnels) :
- `categories` (query) : liste virgule-séparée
- `difficulty` (query) : 1–3

**Réponse 200** :
```json
{
  "AVAILABLE": 42,   // questions pool non utilisées
  "USED": 8,         // questions pool déjà utilisées
  "TOTAL": 50        // total réservoir
}
```

**Logique** :
- `AVAILABLE` = {q ∈ réservoir | q.CATEGORY ∈ catégories ∧ q.DIFFICULTY == difficulté ∧ ¬used[q.ID]}
- `USED` = même filtre mais `used[q.ID] == true`
- `TOTAL` = même filtre (total toutes questions)

**Besoin estimé** (calculé côté frontend) :
```
estimatedNeed = ceil( MANCHE_TIME / QUESTION_TIME )
ex: ceil( 120 / 3 ) = 40 questions pour manche 2mn × 3s
```
