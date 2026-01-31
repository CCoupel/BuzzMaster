# Serveur Go - BuzzControl

Ce document décrit l'implémentation du serveur Go.

## Structure du Projet

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
│   │   ├── logbuffer.go      # Circular log buffer
│   │   ├── logger.go         # Broadcast logger
│   │   ├── logswebsocket.go  # WebSocket for logs
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

## Features Implementées

| Feature | Status | Notes |
|---------|--------|-------|
| HTTP server (port 80) | OK | Static files, REST API |
| WebSocket server (/ws) | OK | Web client communication |
| TCP server (port 1234) | OK | BuzzClick buzzer protocol |
| UDP broadcast (port 1234) | OK | Time-critical messages |
| DNS server (port 53) | OK | Captive portal |
| mDNS (_sock._tcp) | OK | Service discovery |
| Questions CRUD | OK | Upload, list, delete |
| Teams/Bumpers management | OK | Full state sync |
| Game state machine | OK | STOP→PREPARE→READY→START→PAUSE |
| TAR backup/restore | OK | /backup and /restore endpoints |
| Configuration | OK | JSON config file |
| Client type tracking | OK | Admin vs TV client differentiation |
| Client count indicators | OK | Real-time admin/TV count in navbar |
| Version display | OK | Server + Web versions in navbar |
| Score progress bars | OK | Animated bars proportional to score ratio |
| Game page 3-column layout | OK | Questions left, controls center, teams right |
| Question reordering | OK | Drag and drop in Questions page |
| History persistence | OK | Event sourcing with auto-save |
| Teams/Bumpers persistence | OK | Auto-save after modifications |
| Selective backup | OK | Choose what to include in backup |
| Selective reset | OK | Choose what to reset |
| Intelligent restore | OK | Auto-detect and restore from TAR |
| Question status persistence | OK | Statuses preserved across selections & restarts |

## Data Persistence (v2.21.0)

The server persists all game data to disk with automatic saving.

### Persistence Files

| File | Location | Description |
|------|----------|-------------|
| `teams.json` | `data/config/teams.json` | All teams with colors, scores, TeamPoints |
| `bumpers.json` | `data/config/bumpers.json` | All bumpers/players with scores, teams |
| `history.json` | `data/config/history.json` | Game events (source of truth for scores) |
| `question_statuses.json` | `data/config/question_statuses.json` | Question statuses (AVAILABLE/STARTED/STOPPED/REVEALED) |

### Event Sourcing Pattern

History is the **source of truth** for all scores:
- Each `POINTS_AWARDED` event is recorded with timestamp, question, winner, points
- On startup, `RecalculateScoresFromHistory()` replays all events to derive scores
- Scores can be fully reconstructed from history at any time

### Auto-Save Behavior

All modifications trigger async save to disk:
- `UpdateBumper()` → saves bumpers.json
- `UpdateTeam()` → saves teams.json
- `SetTeams()`/`SetBumpers()` → saves respective files
- `UpdateBumperScore()`/`UpdateTeamScore()` → saves both
- `AddGameEvent()` → saves history.json
- `RAZScores()` → saves all (with zeros/empty history)

### Startup Behavior

1. Try to load teams.json and bumpers.json
2. If no files exist, initialize test data
3. Load history.json
4. Recalculate all scores from history events
5. Load question_statuses.json (restore previous question states)

## Key Implementation Decisions

### 1. Media Filename Generation

Random 4-digit suffix like ESP32:
```go
randomNum := rand.Intn(9000) + 1000  // 1000-9999
fileName := fmt.Sprintf("media_%d%s", randomNum, ext)
```

### 2. WebSocket Message Broadcasting

After question changes, broadcast to all clients:
```go
// OnQuestionUpload callback triggers broadcastQuestions()
httpServer.OnQuestionUpload = func() {
    app.broadcastQuestions()
}
```

### 3. DELETE Action Implementation

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

### 4. Storage Info (FSINFO)

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

## Running the Server

```bash
# Development (Windows) - with filesystem web files
cd server-go
go build -v ./cmd/server
./server.exe

# Production (Raspberry Pi)
GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server
scp buzzcontrol pi@raspberrypi.local:~/
```

## Stopping the Server

```bash
# Graceful shutdown via API (recommended)
curl http://localhost/shutdown

# Force kill (if server not responding)
taskkill /IM server.exe /F
```

**IMPORTANT - Procédure de relance du serveur:**
1. Appeler l'API shutdown: `curl http://localhost/shutdown`
2. Attendre l'arrêt (2 secondes)
3. Relancer l'exécutable: `./server.exe`

Ne jamais utiliser `taskkill` ou `kill` pour arrêter le serveur.

## Demo Mode (v2.38.10)

Le mode démo charge des données de démonstration pour présenter toutes les fonctionnalités.

**Accès** : Page Configuration → Section "Mode Demo" → Bouton "Charger la demo"

**API** : `POST /load-demo`

**Données créées :**
| Type | Quantité | Détails |
|------|----------|---------|
| Équipes | 6 | Avec TeamPoints pré-remplis |
| Joueurs | 24 | 4 par équipe, toutes couleurs QCM (A/B/C/D) |
| Questions | 10 | QCM (avec indices), MEMORY, NORMAL |
| Catégories | 8 | GEOGRAPHY, ENTERTAINMENT, HISTORY, etc. |
| Historique | 10 | Événements pour vue PALMARES |
| Fonds | 3 | Opacités variées (100%, 80%, 60%) |
| Images | 5 | Embarquées dans l'exécutable |

**Assets embarqués** : `assets/demo/`

## Portable Build (v2.35.0)

L'exécutable portable embarque les fichiers web directement dans le binaire.

```bash
# Windows (PowerShell)
cd server-go
.\build.ps1

# Linux/macOS (Bash)
cd server-go
./build.sh
```

**Structure du build portable :**
```
server.exe        # Exécutable autonome (~13 MB)
data/             # Données variables (créé automatiquement)
├── config/       # Configuration (teams, bumpers, history)
└── files/        # Fichiers utilisateur (questions, backgrounds)
```

**Mode de fonctionnement :**
- Si des fichiers web sont embarqués → mode portable (prioritaire)
- Sinon, si `web/dist/` existe → mode développement
- Sinon → mode legacy (ancienne UI)

## Logs Page (v2.42.0)

Page de visualisation des logs serveur en temps réel.

**Backend :**
- `LogBuffer` : Buffer circulaire thread-safe (1000 logs max)
- `BroadcastLogger` : Logger avec diffusion temps réel
- **WebSocket dédiée** : `/ws/logs` (séparée de `/ws` pour le jeu)

**Architecture WebSocket Logs** (v2.43.0) :
- **Endpoint** : `/ws/logs` (WebSocket dédiée, indépendante de `/ws`)
- **Connexion** : Le client se connecte → reçoit automatiquement `LOG_HISTORY`
- **Temps réel** : Chaque nouveau log est envoyé via `LOG_ENTRY`
- **Déconnexion** : Fermer la WebSocket = désabonnement (pas d'action explicite)

**Messages WebSocket** :
| Message | Direction | Description |
|---------|-----------|-------------|
| `LOG_HISTORY` | Server→Client | Historique complet à la connexion |
| `LOG_ENTRY` | Server→Client | Nouveau log temps réel |

## Key Libraries

| Purpose | Library |
|---------|---------|
| HTTP router | `net/http` (stdlib) |
| WebSocket | `github.com/gorilla/websocket` |
| JSON | `encoding/json` (stdlib) |
| TAR | `archive/tar` (stdlib) |
| Config | JSON file |

## Cross-compilation

```bash
# Windows (development)
go build -o buzzcontrol.exe ./cmd/server

# Raspberry Pi (production)
GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server
```
