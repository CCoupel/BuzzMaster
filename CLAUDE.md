# CLAUDE.md - BuzzControl Project Reference

> **Historique des versions** : Voir [CHANGELOG.md](CHANGELOG.md) pour l'historique détaillé des fonctionnalités par version.
> **Guide d'administration** : Voir [docs/ADMIN_GUIDE.md](docs/ADMIN_GUIDE.md) pour la persistance, sauvegarde/restauration et gestion des scores.

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
│   ├── MIGRATION_ARCHITECTURE.md  # Architecture decisions
│   ├── GAME_STATE_MACHINE.md      # Game state machine specification
│   └── ADMIN_GUIDE.md             # Administration guide (backup, restore, scores)
├── platformio.ini            # PlatformIO config
├── partitions.csv            # ESP32 partition table
├── CLAUDE.md                 # This file
└── CHANGELOG.md              # Version history
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
| REMOTE | Change TV display | `{REMOTE: "GAME\|SCORES\|PLAYERS"}` |
| RAZ | Reset all scores | `{}` |
| DELETE | Delete question | `{ID: questionId}` |
| UPDATE | Update teams/bumpers | `{teams: {...}, bumpers: {...}}` |
| TEAM_POINTS | Modify team score | `{TEAM: teamName, POINTS: delta}` |
| BUMPER_POINTS | Modify player score | `{ID: bumperMac, POINTS: delta}` |
| REORDER_QUESTIONS | Reorder questions | `{ORDER: [questionId1, questionId2, ...]}` |

### HTTP REST API
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

#### Selective Backup (`/backup-select`)
Query parameters (all boolean, default: `true` if none specified):
- `questions=true` - Include questions directory
- `teams=true` - Include teams.json
- `bumpers=true` - Include bumpers.json
- `history=true` - Include history.json
- `backgrounds=true` - Include backgrounds directory

Example: `/backup-select?questions=true&history=true`

#### Selective Reset (`/reset-select`)
Query parameters (all boolean):
- `all=true` - Reset everything
- `questions=true` - Delete all questions
- `teams=true` - Clear teams data
- `bumpers=true` - Clear bumpers data
- `history=true` - Clear history
- `backgrounds=true` - Delete backgrounds

Example: `/reset-select?history=true&bumpers=true`

#### Intelligent Restore (`/restore`)
The restore endpoint now automatically detects what's in the TAR archive and restores accordingly:
- Detects `files/questions/*` → restores questions
- Detects `config/teams.json` → loads teams into engine
- Detects `config/bumpers.json` → loads bumpers into engine
- Detects `config/history.json` → loads history and recalculates scores
- Detects `files/backgrounds/*` → restores backgrounds

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
    "MEDIA_ANSWER": "/question/1/answer.jpg",
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
  "TYPE": "NORMAL",
  "POINTS": 10,
  "TIME": 30,
  "MEDIA": "/question/1/image.jpg",
  "MEDIA_ANSWER": "/question/1/answer.jpg"
}
```

### Question (QCM type)
```json
{
  "ID": "2",
  "QUESTION": "Quelle est la capitale de la France?",
  "ANSWER": "Paris",
  "TYPE": "QCM",
  "QCM_ANSWERS": {
    "RED": "Londres",
    "GREEN": "Paris",
    "YELLOW": "Berlin",
    "BLUE": "Madrid"
  },
  "QCM_CORRECT": "GREEN",
  "POINTS": 10,
  "TIME": 30,
  "MEDIA": "/question/2/image.jpg",
  "MEDIA_ANSWER": "/question/2/answer.jpg"
}
```

**Champs Media:**
- `MEDIA`: Image de la question (affichée pendant STARTED/PAUSED)
- `MEDIA_ANSWER`: Image de la réponse (remplace MEDIA pendant REVEALED)

## Game Flow

> **Documentation complète**: Voir [docs/GAME_STATE_MACHINE.md](docs/GAME_STATE_MACHINE.md) pour la spécification détaillée.

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

### Implemented Features (v2.0)

| Feature | Status | Notes |
|---------|--------|-------|
| HTTP server (port 8080) | ✅ | Static files, REST API |
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
| Client type tracking | ✅ | Admin vs TV client differentiation |
| Client count indicators | ✅ | Real-time admin/TV count in navbar |
| Version display | ✅ | Server + Web versions in navbar |
| Score progress bars | ✅ | Animated bars proportional to score ratio |
| Game page 3-column layout | ✅ | Questions left, controls center, teams right |
| Question reordering | ✅ | Drag and drop in Questions page |
| History persistence | ✅ | Event sourcing with auto-save |
| Teams/Bumpers persistence | ✅ | Auto-save after modifications |
| Selective backup | ✅ | Choose what to include in backup |
| Selective reset | ✅ | Choose what to reset |
| Intelligent restore | ✅ | Auto-detect and restore from TAR |

### Data Persistence (v2.21.0)

The server now persists all game data to disk with automatic saving.

#### Persistence Files
| File | Location | Description |
|------|----------|-------------|
| `teams.json` | `data/config/teams.json` | All teams with colors, scores, TeamPoints |
| `bumpers.json` | `data/config/bumpers.json` | All bumpers/players with scores, teams |
| `history.json` | `data/config/history.json` | Game events (source of truth for scores) |

#### Event Sourcing Pattern
History is the **source of truth** for all scores:
- Each `POINTS_AWARDED` event is recorded with timestamp, question, winner, points
- On startup, `RecalculateScoresFromHistory()` replays all events to derive scores
- Scores can be fully reconstructed from history at any time

#### Auto-Save Behavior
All modifications trigger async save to disk:
- `UpdateBumper()` → saves bumpers.json
- `UpdateTeam()` → saves teams.json
- `SetTeams()`/`SetBumpers()` → saves respective files
- `UpdateBumperScore()`/`UpdateTeamScore()` → saves both
- `AddGameEvent()` → saves history.json
- `RAZScores()` → saves all (with zeros/empty history)

#### Startup Behavior
1. Try to load teams.json and bumpers.json
2. If no files exist, initialize test data
3. Load history.json
4. Recalculate all scores from history events

### UI Components

#### Podium Component (v2.4.0)
Shared component for displaying rankings with tie support:
- **Location**: `components/Podium.jsx` + `components/Podium.css`
- **Used by**: ScoresPage (teams), PlayerDisplay (players), QuestionPreview (both)
- **Variants**: `default` (full size), `compact` (smaller for admin/preview)
- **Tie handling**: Multiple teams/players with same score share the same rank
- **Animation**: Framer-motion for entrance animations and score changes

```jsx
// Usage example
<Podium teams={sortedTeams} variant="compact" />
```

#### QuestionPreview Component (v2.11.0)
TV preview as iframe - perfect sync with actual /tv display:
- **Location**: `components/QuestionPreview.jsx` + `components/QuestionPreview.css`
- **Implementation**: Simple iframe pointing to `/tv`
- **Benefits**: Zero maintenance, always in sync, ~15 lines of code
- **Trade-off**: Double WebSocket connection (acceptable for admin preview)

#### Score Progress Bars
Teams and players display animated progress bars showing their score relative to the maximum score:
- **Width calculation**: `(score / maxScore) * 100%`
- **Color**: Team color with glow effect
- **Animation**: Smooth transition using framer-motion
- **Location**: Admin Scores page + TV/Player display (SCORE and PLAYERS views)

#### Game Page Layout (v2.12.0)
3-column responsive layout with TV preview, harmonized columns:
- **Left column (280px)**: Questions list with compact preview
- **Center column (flex)**: Timer + controls + TV preview (16:9)
- **Right column (280px)**: Teams list (vertical) - same width as questions

**Responsive breakpoints:**
- `>1600px`: 3 columns (280px / 1fr / 280px)
- `1400-1600px`: 3 columns (250px / 1fr / 250px)
- `1200-1400px`: 3 columns (220px / 1fr / 220px)
- `768-1200px`: 2 columns (questions + controls / teams)
- `<768px`: 1 column (stacked)

**Team cards styling (v2.12.0):**
- Same width as question cards (240px max)
- Team color as subtle background tint (8%)
- Player answer colors displayed with colored badge (A/B/C/D)
- Player rows highlighted with answer color (20% tint, 4px left border)

#### Points Animation (v2.12.0)
Animation visuelle quand des points sont ajoutés :
- **Confetti** : Particules avec la couleur de l'équipe
- **Animation flottante** : Nom de l'équipe + "+X pts" au centre de l'écran
- **Durée** : 2.5 secondes puis disparition
- **Déclenchement** : Uniquement quand les points sont ajoutés (pas au REVEAL)
- **Vue JOUEURS** : Animation sur la ligne du joueur (scale + couleur verte)

#### Debug Features (v2.12.0)
Fonctionnalités de test pour l'admin :
- **Ctrl+clic sur joueur** : Simule un appui buzzer (pendant STARTED/PAUSED)
- **Ctrl+clic sur question** : Force l'état READY sans attendre les PONGs

#### Waiting States (v2.12.0)
États visuels pour équipes/joueurs :
- **PREPARE/READY** : Grisés jusqu'à réception du PONG
- **STARTED/PAUSED** : Grisés jusqu'au buzz
- **Après buzz** : Visibilité restaurée avec couleur d'équipe

#### Reaction Time (v2.12.0)
Affichage du temps de réaction :
- **GameTime** : Timestamp serveur au démarrage (microsecondes)
- **Calcul** :  ms
- **Tri** : Joueurs triés par temps de réponse dans chaque équipe

#### Question Cards Layout (v2.19.0)
Nouvelle mise en page des cartes questions dans le panneau admin :
- **Layout horizontal** : Thumbnail (70x70px) à gauche, texte à droite
- **Header** : `#ID 👤 30s 1pt [STATUS]` - ID, badge target, temps, points, status
- **Body** : Question (4 lignes max), Réponse (3 lignes max)
- **Lisibilité** : Plus d'espace pour le texte, pas besoin de zoom

#### POINTS_TARGET (v2.19.0)
Système d'attribution des points par question :
- **POINTS_TARGET** : Champ sur chaque question (`PLAYER` ou `TEAM`)
- **Défaut** : `PLAYER` pour NORMAL, `TEAM` pour QCM
- **Indicateur admin** : Badge "👤 Joueur" (cyan) ou "👥 Equipe" (orange) sur la ligne "Affichage TV"
- **Badge question** : Icône personne/groupe sur chaque carte question
- **Attribution** : Clic sur joueur → points au joueur OU à l'équipe selon target

#### Un seul buzz par équipe (v2.19.0)
Restriction du buzz à un seul joueur par équipe :
- **Règle** : Si `team.Time > 0`, le buzz est ignoré pour les autres joueurs de l'équipe
- **Fichier** : `engine.go:ProcessButtonPress()`
- **Comportement** : Premier joueur à buzzer représente l'équipe

#### History Page (v2.20.0)
Page d'historique des événements de jeu :
- **Route** : `/history-page`
- **Endpoint API** : `GET /history` retourne `[]GameEvent`
- **Fonctionnalités** :
  - Événements groupés par question (ordre chronologique)
  - Vue collapsible : clic sur l'en-tête pour ouvrir/fermer
  - Boutons "Tout ouvrir" / "Tout fermer"
  - **Vue réduite** : Résumé des points par équipe et par joueur (badges colorés)
  - **Vue détaillée** : Tableau avec Heure, Équipe, Joueur, Temps, Points
  - Séparation stricte : points TEAM vs points PLAYER (pas de cumul mixte)
- **GameEvent model** :
  ```go
  type GameEvent struct {
    Timestamp    int64   // Server timestamp (microseconds)
    QuestionID   string  // Question ID
    QuestionText string  // Question text
    EventType    string  // "POINTS_AWARDED"
    WinnerType   string  // "PLAYER" or "TEAM"
    TeamName     string  // Team name
    TeamColor    []int   // Team RGB color
    PlayerName   string  // Player name (if PLAYER)
    PlayerColor  string  // Player answer color
    Points       int     // Points awarded
  }
  ```
- **Fichiers** : `HistoryPage.jsx`, `HistoryPage.css`, `engine.go:AddGameEvent()`

#### Teams Page - Drag & Drop (v2.5.0)
Interface de gestion des équipes avec drag & drop :
- **Gauche** : Grille des équipes (zones de dépôt)
- **Droite (320px)** : Joueurs non assignés
- **Drag & Drop** : Glisser un joueur sur une équipe pour l'assigner
- **Désassigner** : Glisser vers la zone "non assignés"

#### Couleurs de Réponse (v2.5.0)
Chaque joueur peut avoir une couleur de réponse pour le mode QCM :
- **Couleurs disponibles** : Rouge (A), Vert (B), Jaune (C), Bleu (D)
- **Sélection** : Uniquement quand le joueur n'est PAS assigné à une équipe
- **Affichage** : La couleur devient le fond de l'avatar du joueur
- **Sans couleur** : Avatar gris par défaut
- **Champ** : `ANSWER_COLOR` dans le modèle Bumper (valeurs: `RED`, `GREEN`, `YELLOW`, `BLUE`)

```jsx
// Couleurs définies dans TeamsPage.jsx
const ANSWER_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
}
```

#### Questions QCM (v2.6.0)
Support des questions à choix multiples (QCM) :
- **Types de question** : `NORMAL` (réponse libre) ou `QCM` (4 choix colorés)
- **Réponses QCM** : 4 réponses associées aux couleurs (Rouge A, Vert B, Jaune C, Bleu D)
- **Réponse correcte** : Champ `QCM_CORRECT` indique la couleur de la bonne réponse
- **UI** : Sélecteur de type + 4 champs de réponse colorés + bouton pour marquer la bonne réponse
- **Badge** : Les questions QCM affichent un badge "QCM" dans la liste

**Champs Question QCM :**
- `TYPE`: `"QCM"` pour les questions à choix multiples
- `QCM_ANSWERS`: Objet avec les 4 réponses `{RED, GREEN, YELLOW, BLUE}`
- `QCM_CORRECT`: Couleur de la bonne réponse (`"RED"`, `"GREEN"`, `"YELLOW"`, `"BLUE"`)
- `ANSWER`: Contient automatiquement le texte de la bonne réponse

**Fichiers modifiés :**
- `server-go/internal/game/models.go` : Types `QuestionType`, `QCMAnswers`, champs Question
- `server-go/internal/server/http.go` : Handling des champs QCM dans POST /questions
- `server-go/web/src/pages/QuestionsPage.jsx` : UI formulaire QCM
- `server-go/web/src/pages/QuestionsPage.css` : Styles QCM

#### QCM Team Badges (v2.16.0)
Pastilles d'équipes sur les réponses QCM pendant STOPPED/REVEALED :
- **Affichage** : Pastilles colorées sur chaque réponse QCM montrant quelles équipes ont répondu
- **Couleur** : Couleur de l'équipe (pas la couleur de la réponse QCM)
- **Disposition** : Horizontale, alignée à droite de chaque réponse
- **Taille dégradée** : 70% (première) à 40% (dernière) de la taille de base (60px)
- **Tri** : Par temps de réponse (plus rapide = plus grand, à gauche)
- **Phases** : Visible en STOPPED et REVEALED

**Logique de calcul :**
- Utilise `ANSWER_COLOR` du bumper (couleur assignée au joueur pour QCM)
- Premier joueur de chaque équipe à buzzer détermine la réponse de l'équipe
- Temps de réponse (`bumper.TIME`) utilisé pour le tri

**Fichiers modifiés :**
- `server-go/web/src/pages/PlayerDisplay.jsx` : `teamsByQcmAnswer` useMemo, rendu des badges
- `server-go/web/src/pages/PlayerDisplay.css` : Styles `.qcm-team-badges`, `.qcm-team-badge`
#### Question Reordering (v2.7.0)
Drag and drop pour reordonner les questions :
- **Interface** : Glisser-deposer les cartes de questions dans QuestionsPage
- **Poignee** : Icone ⋮⋮ sur chaque carte pour indiquer le drag
- **Feedback visuel** : Opacite reduite pendant le drag, bordure pointillee sur la cible
- **Persistance** : Champ `ORDER` dans chaque `question.json`
- **Tri** : Questions triees par `ORDER` si disponible, sinon par `ID`

**Action WebSocket :**
- `REORDER_QUESTIONS` : `{ORDER: ["6", "4", "1", "2", "3", "5"]}`

**Fichiers modifies :**
- `server-go/internal/protocol/messages.go` : Action et payload `ReorderQuestionsPayload`
- `server-go/cmd/server/main.go` : Handler `handleReorderQuestions`
- `server-go/web/src/pages/QuestionsPage.jsx` : UI drag and drop
- `server-go/web/src/pages/QuestionsPage.css` : Styles drag and drop
- `server-go/web/src/pages/GamePage.jsx` : Tri par ORDER

#### PlayerDisplay 4-Zone Layout (v2.11.1)
Layout vertical en 4 zones avec hauteurs fixes pour l'affichage TV (/tv) :
- **Zone 1 - Timer** : 100px hauteur fixe, centré en haut
- **Zone 2 - Question** : 80px hauteur fixe, texte de la question
- **Zone 3 - Media** : flex: 1, remplit l'espace restant, image centrée
- **Zone 4 - Answers** : 120px hauteur fixe, `margin-top: auto` (aligné en bas)

**Timer couleur synchronisée :**
- Couleur du compteur = couleur de la barre de progression
- Vert (`--success`) : > 50% du temps restant
- Orange (`--warning`) : 25-50% du temps (urgent)
- Rouge (`--error`) : < 25% du temps (critique)

**Transition QCM READY → STARTED :**
- Bloc QCM unifié couvrant les phases READY → STARTED → REVEALED
- Les réponses QCM restent en place (pas de re-render/flash)
- Seuls la question et le média apparaissent en fondu à STARTED
- Le message "PREPAREZ-VOUS" disparaît, remplacé par le média

**Structure CSS :**
```css
.game-content-zones {
  display: flex;
  flex-direction: column;
  flex: 1;
  overflow: hidden;
}
.zone-timer { height: 100px; flex-shrink: 0; }
.zone-question { height: 80px; flex-shrink: 0; }
.zone-media { flex: 1; min-height: 0; overflow: hidden; }
.zone-answers { height: 120px; flex-shrink: 0; margin-top: auto; }
```

**Fichiers :**
- `server-go/web/src/pages/PlayerDisplay.jsx` : Structure des zones
- `server-go/web/src/pages/PlayerDisplay.css` : Styles des zones

#### Timer Phase Badges (v2.10.0)
Pastilles colorées indiquant l'état du jeu dans le composant Timer :
- **ARRET** (STOPPED) : Rouge (`--error`) - Affiché
- **PREPARATION** : Orange (`--warning`)
- **PRET** : Cyan (`--accent-cyan`)
- **EN COURS** : Vert (`--success`)
- **PAUSE** : Bleu (`--primary-500`)
- **REPONSE** (REVEALED) : Gris (`--gray-400`)

Les couleurs correspondent aux statuts de questions (AVAILABLE=vert, STARTED=orange, STOPPED=rouge, REVEALED=gris).

**Fichiers :**
- `server-go/web/src/components/Timer.jsx` : Affichage des badges
- `server-go/web/src/components/Timer.css` : Styles `.phase-stopped`, `.phase-revealed`

#### Media Answer (v2.14.0)
Support des images de réponse distinctes de l'image de question :
- **MEDIA** : Image affichée pendant les phases STARTED et PAUSED
- **MEDIA_ANSWER** : Image de réponse qui REMPLACE MEDIA pendant la phase REVEALED
- **Effet visuel** : Cadre vert pulsant autour de l'image de réponse pendant REVEALED
- **Thumbnails** : Vignette de l'image réponse affichée en bas à droite des cartes questions

**Comportement :**
- Si MEDIA_ANSWER existe : affiché à la place de MEDIA pendant REVEALED
- Si seul MEDIA existe : affiché pendant toutes les phases (comportement existant)
- Les vignettes de réponse sont TOUJOURS petites et positionnées en bas à droite
- Quand il n'y a pas d'image question, un placeholder gris s'affiche avec la vignette réponse en bas à droite

**Styles CSS :**
```css
.answer-media-highlight {
  border: 4px solid var(--success);
  box-shadow: 0 0 40px rgba(34, 197, 94, 0.6);
  animation: answer-media-pulse 2s ease-in-out infinite;
}
```

**Fichiers modifiés :**
- `server-go/internal/game/models.go` : Champ `MEDIA_ANSWER` dans Question
- `server-go/internal/server/http.go` : Upload `file_answer` dans POST /questions
- `server-go/web/src/pages/QuestionsPage.jsx` : Input pour image réponse + vignettes
- `server-go/web/src/pages/QuestionsPage.css` : Styles thumbnails avec positionnement absolu
- `server-go/web/src/pages/GamePage.jsx` : Vignettes réponse dans liste questions
- `server-go/web/src/pages/GamePage.css` : Styles preview-image-answer
- `server-go/web/src/pages/PlayerDisplay.jsx` : Affichage MEDIA_ANSWER pendant REVEALED
- `server-go/web/src/pages/PlayerDisplay.css` : Styles answer-media-highlight avec animation

### WebSocket Actions for Client Management

| Action | Direction | Description |
|--------|-----------|-------------|
| SET_CLIENT_TYPE | Client→Server | Set client type (admin/tv) |
| CLIENTS | Server→Client | Broadcast client counts |

**SET_CLIENT_TYPE payload:**
```json
{ "TYPE": "admin" }  // or "tv"
```

**CLIENTS payload:**
```json
{ "ADMIN_COUNT": 2, "TV_COUNT": 1 }
```

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
    "websocket_path": "/ws"
  },
  "storage": {
    "data_dir": "./data",
    "questions_dir": "./data/files/questions"
  }
}
```

### Standard Ports
| Service | Port | Description |
|---------|------|-------------|
| HTTP | 80 | Web interface |
| TCP | 1234 | BuzzClick buzzer protocol |
| UDP | 1234 | Broadcast (same as TCP) |
| DNS | 53 | Captive portal (optional) |

---

## React Web Interface (v2.3.0)

### Structure des pages

| Route | Page | Description |
|-------|------|-------------|
| `/` | GamePage | Interface admin principale |
| `/tv` | PlayerDisplay | Affichage joueurs (plein écran) |
| `/scoreboard` | ScoresPage | Tableau des scores |
| `/teams` | TeamsPage | Gestion des équipes |
| `/quiz` | QuizPage | Gestion des questions |
| `/settings` | SettingsPage | Configuration |

### Layout GamePage (Admin) - v2.12.0

Layout avec timer pleine largeur + 3 colonnes harmonisées :
```
| Timer (pleine largeur, 95%)                    |  ligne 1
|------------------------------------------------|
| Questions | Contrôles + Aperçu TV | Équipes    |  ligne 2
| 280px     | 1fr (flexible)        | 280px      |
```

- **max-width** : 1800px (pour exploiter les grands écrans)
- **Breakpoints** : 1600px (250px), 1400px (220px), 1200px, 768px
- **Colonnes harmonisées** : Questions et Équipes ont la même largeur

**Ligne 1** : Timer avec barre de progression 95% de largeur

**Colonne gauche (280px)** : Liste des questions avec miniatures (cartes 240px)

**Colonne centrale (flexible)** :
- Contrôles de jeu (START/PAUSE sur même ligne, REPONSE)
- Toggle affichage TV (Jeu, Équipes, Joueurs)
- Aperçu TV 16:9 (QuestionPreview en iframe vers /tv)

**Colonne droite (280px)** : Cartes équipes empilées verticalement (cartes 240px)
- Fond coloré estompé avec la couleur d'équipe
- Affichage des couleurs de réponse QCM pour chaque joueur

### Statuts des questions (couleurs)

| Statut | Couleur bordure | Fond | Apparence |
|--------|-----------------|------|-----------|
| AVAILABLE | Vert | Vert clair | Normal |
| STARTED | Orange | Orange clair | Normal |
| STOPPED | Rouge | Rouge clair | Normal |
| REVEALED | Gris | Gris | Compact (image/réponse masquées, opacité 50%) |

### Composants clés

| Composant | Fichier | Description |
|-----------|---------|-------------|
| Podium | `components/Podium.jsx` | Podium 1-2-3 avec gestion égalités (variantes: default, compact) |
| QuestionPreview | `components/QuestionPreview.jsx` | Aperçu 16:9 de l'affichage TV (utilise Podium) |
| TeamCard | `components/TeamCard.jsx` | Carte équipe compacte (260px) |
| Timer | `components/Timer.jsx` | Chronomètre avec barre de progression |
| Navbar | `components/Navbar.jsx` | Navigation + versions + compteurs clients |

### Données de test (développement)

Au démarrage, le serveur initialise des données de test via `initTestData()` :
- **6 équipes** : Les Rouges, Les Bleus, Les Verts, Les Jaunes, Les Violets, Les Oranges
- **12 buzzers** : 2 joueurs par équipe avec scores variés et couleurs de réponse assignées

### WebSocket Messages

Le hook `useWebSocket.js` gère la communication :

| Message reçu | Données | Utilisation |
|--------------|---------|-------------|
| UPDATE | `{GAME, teams, bumpers}` + VERSION | État du jeu + version serveur |
| QUESTIONS | `{questions}` + FSINFO + VERSION | Liste questions + espace disque |
| CLIENTS | `{ADMIN_COUNT, TV_COUNT}` | Compteurs clients connectés |

**Important** : VERSION est inclus dans UPDATE et QUESTIONS pour afficher la version serveur dans la navbar.

---

## Gestion des Versions

### Format de version : x.y.z

| Segment | Signification | Quand incrémenter |
|---------|---------------|-------------------|
| **x** | Version majeure | Changement d'architecture ou breaking change |
| **y** | Version mineure | Nouvelle fonctionnalité |
| **z** | Version de test | À chaque relance du serveur pour test |

### Règles de versionnement

1. **Nouvelle fonctionnalité** → Incrémenter **y**, mettre z à 1
   - Exemple : 2.1.0 → 2.2.1

2. **Relance serveur pour test** → Incrémenter **z** (pas de limite)
   - Exemple : 2.2.1 → 2.2.2 → 2.2.3 → 2.2.15...

3. **Validation par l'utilisateur** → Remettre **z** à 0, documenter et commit
   - Exemple : 2.2.15 → 2.2.0 (puis commit)

### Fichiers à mettre à jour

| Fichier | Champ |
|---------|-------|
| `server-go/config.json` | `"version": "x.y.z"` |
| `server-go/web/package.json` | `"version": "x.y.z"` |

**Note** : La version serveur est lue depuis `config.json`, la version web depuis `package.json`.

---

## Procédure de Test Systématique

Avant chaque test du serveur, suivre cette procédure pour garantir un environnement propre.

**IMPORTANT** : Toujours lancer le serveur en mode visible (fenêtre CMD) pour voir les logs en temps réel. Ne jamais utiliser `-WindowStyle Hidden` en développement.

### Étapes

| # | Tâche | Commande / Action |
|---|-------|-------------------|
| 1 | **Arrêter le serveur en cours** | `taskkill /IM server.exe /F` |
| 2 | **Mettre à jour les versions** | `config.json` et `package.json` (voir section Gestion des Versions) |
| 3 | **Rebuild le frontend** | `npm run build --prefix server-go/web` |
| 4 | **Rebuild le serveur Go** | `go build -o server.exe ./cmd/server` |
| 5 | **Lancer le serveur EN MODE VISIBLE** | Ouvrir une fenêtre CMD/PowerShell, `cd server-go`, puis `./server.exe` |
| 6 | **Vérifier page admin (/)** | Ouvrir http://localhost/ dans Chrome |
| 7 | **Vérifier page joueur (/tv)** | Ouvrir http://localhost/tv dans Chrome |
| 8 | **Vérifier les versions affichées** | Navbar : Serveur et Web doivent correspondre |

### Vérifications attendues

- [ ] Page admin (/) s'affiche correctement
- [ ] Page joueur (/tv) s'affiche correctement
- [ ] Version serveur affichée (ex: 2.0.0)
- [ ] Version web affichée (ex: 2.2.1)
- [ ] Compteurs clients visibles (Admin: X, TV: Y)
- [ ] WebSocket connecté (pas d'erreur console)

### En cas d'erreur de port occupé

```bash
# Trouver le processus utilisant le port
netstat -ano | findstr :80

# Tuer le processus par PID
taskkill /PID <PID> /F
```

---

## Procédure de Validation et Commit

Lorsque l'utilisateur valide l'implémentation :

### Étapes

| # | Tâche | Action |
|---|-------|--------|
| 1 | **Remettre z à 0** | Version x.y.z → x.y.0 |
| 2 | **Mettre à jour config.json** | `"version": "x.y.0"` |
| 3 | **Mettre à jour package.json** | `"version": "x.y.0"` |
| 4 | **Mettre à jour CLAUDE.md** | Documenter les nouvelles fonctionnalités |
| 5 | **Rebuild le frontend** | `npm run build --prefix server-go/web` |
| 6 | **Rebuild le serveur Go** | `go build -o server.exe ./cmd/server` |
| 7 | **Git commit** | Message décrivant les changements |

### Format de commit

```bash
git add .
git commit -m "feat: Description de la fonctionnalité

- Détail 1
- Détail 2

Version: Server x.y.z / Web x.y.0

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
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

#### Admin Layout Fix (v2.17.0)
Correction du layout de la page admin pour éviter le scroll global :
- **Page fixe** : `height` au lieu de `min-height` sur `.game-page`
- **Scroll interne** : Chaque colonne (Questions, Contrôles, Équipes) a sa propre zone de scroll
- **Alignement** : Les colonnes s'alignent avec le bas de la preview TV

#### TeamCard Optimisé (v2.17.0)
Réduction de l'espace occupé par les cartes d'équipe :
- **Score compact** : Suppression du label "Score", affichage "X Pts" directement
- **Espacement réduit** : Moins d'espace entre le score et la liste des joueurs
- **Taille réduite** : Police du score réduite (1.5rem → 1.25rem)
