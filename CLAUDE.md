# CLAUDE.md - BuzzControl Project Reference

> **Historique des versions** : Voir [CHANGELOG.md](CHANGELOG.md) pour l'historique détaillé des fonctionnalités par version.
>
> **Procédures** : DEV → QUALIF → RELEASE (voir section [Procédures de Développement et Production](#procédures-de-développement-et-production))
> - [DEV_PROCEDURE.md](docs/DEV_PROCEDURE.md) | [TEST_PROCEDURE.md](docs/TEST_PROCEDURE.md) | [QUALIF_PROCEDURE.md](docs/QUALIF_PROCEDURE.md) | [RELEASE_PROCEDURE.md](docs/RELEASE_PROCEDURE.md)
>
> **Guide utilisateur** : Voir [docs/ADMIN_GUIDE.md](docs/ADMIN_GUIDE.md) pour la persistance, sauvegarde/restauration et gestion des scores.

## Project Overview

BuzzControl is a wireless buzzer system for quiz games. The system consists of:
- **BuzzControl**: Central server (**100% Go**, Raspberry Pi / Windows) - Aucune dépendance Python ou externe
- **BuzzClick**: Individual buzzer clients (ESP32-C3, WiFi configurable via USB depuis v3.0.0)

**Architecture backend** : Le serveur est entièrement développé en Go, sans dépendances Python, Node.js ou autres langages externes. Tous les protocoles (TCP, WebSocket, HTTP, UDP, DNS, SmartConfig ESP-Touch) sont implémentés nativement en Go.

## Repository Structure

```
buzzcontrol/
├── server-go/                # Go server (Raspberry Pi / Windows)
│   ├── cmd/server/           # Entry point
│   ├── internal/
│   │   ├── config/           # Configuration
│   │   ├── server/           # HTTP, WebSocket, TCP, UDP, DNS
│   │   ├── game/             # Game engine and models
│   │   └── protocol/         # Message parsing
│   ├── web/src/              # React frontend
│   └── data/files/questions/ # Question storage
├── src/
│   ├── BuzzControl/          # Server firmware (ESP32-S3) - LEGACY
│   └── BuzzClick/            # Buzzer client firmware (ESP32-C3) - WiFi USB config (v3.0.0)
├── MARKETING/                # Site marketing (worktree gh-pages)
├── docs/                     # Documentation
├── backlog/                  # Feature backlog (specs détaillées, voir GitHub Issues pour le suivi)
├── CLAUDE.md                 # This file
└── CHANGELOG.md              # Version history
```

## Documentation Détaillée

La documentation est organisée en fichiers thématiques :

| Document | Description |
|----------|-------------|
| [docs/PROTOCOLS.md](docs/PROTOCOLS.md) | Protocoles de communication (TCP, WebSocket, HTTP REST API) |
| [docs/WEBSOCKET_PROTOCOL.md](docs/WEBSOCKET_PROTOCOL.md) | Protocole WebSocket buzzers (v3.0.0, mode hybride TCP+WS) |
| [docs/DATA_MODELS.md](docs/DATA_MODELS.md) | Modèles de données (Questions, Teams, Bumpers, QCM, Memory) |
| [docs/GO_SERVER.md](docs/GO_SERVER.md) | Implémentation du serveur Go, persistance, build portable |
| [docs/REACT_INTERFACE.md](docs/REACT_INTERFACE.md) | Interface React, composants UI, VPlayer |
| [docs/MIGRATION_FUTURE.md](docs/MIGRATION_FUTURE.md) | Améliorations futures, protocole v2, BuzzClick v2 |
| [docs/DEV_COMMANDS.md](docs/DEV_COMMANDS.md) | Commandes de développement, versioning, tests |
| [docs/FIRMWARE_UPDATE.md](docs/FIRMWARE_UPDATE.md) | Guide de mise à jour firmware BuzzClick |
| [docs/GAME_STATE_MACHINE.md](docs/GAME_STATE_MACHINE.md) | Machine d'état du jeu |
| [docs/ADMIN_GUIDE.md](docs/ADMIN_GUIDE.md) | Guide utilisateur |

## CI/CD et Release Automatique

Le projet utilise **GitHub Actions** pour automatiser la compilation et la publication des releases.

### Pipeline de Release (`.github/workflows/release.yml`)

**Déclenchement** : Push d'un tag Git `v*` (ex: `v2.54.0`)

**Jobs** (en parallèle après vérification) :
1. **🔍 Checking** (~10s)
   - Vérifie cohérence versions : `config.json`, `package.json`, tag Git
   - Extrait la version pour les jobs suivants

2. **🔨 Compiling** (3 jobs en parallèle, ~1-2 min chacun)
   - **Windows** : Build Go + React embedded → `buzzcontrol-vX.Y.0-windows-amd64.exe` (~8-9 MB)
   - **Linux ARM64** : Build Go + React embedded → `buzzcontrol-vX.Y.0-linux-arm64` (~8 MB)
   - **Firmware BuzzClick** : PlatformIO build ESP32-C3 → `buzzclick-vX.Y.0-firmware.bin` (~500KB-1MB)

3. **🚀 Releasing** (~30s)
   - Télécharge les 3 binaires depuis les artefacts
   - Extrait les notes de release depuis `CHANGELOG.md`
   - Crée la release GitHub avec les 3 binaires attachés

**Durée totale** : ~3-4 minutes

### Versioning Unifié (depuis v2.54.0)

Le serveur et le firmware partagent désormais **le même numéro de version** :
- Serveur : Version dans `server-go/config.json`
- Frontend : Version dans `server-go/web/package.json`
- Firmware : Version injectée automatiquement dans `platformio.ini` par la CI
- **Métadonnées Windows PE** : `server-go/cmd/server/versioninfo.json` — à mettre à jour manuellement à chaque release (champs `FileVersion`, `ProductVersion`)
- Tag Git : `vX.Y.0` déclenche le build de TOUS les composants

> **Note versioninfo.json** : La CI régénère ce fichier automatiquement pour le build Windows. La mise à jour manuelle est requise uniquement pour les builds locaux (`build.ps1`). Le `.syso` généré ne doit PAS être commité (voir `.gitignore`).

**Compatibilité** : Les buzzers avec ancien firmware (1.209.3) continuent de fonctionner avec les nouveaux serveurs (rétrocompatibilité protocole TCP/UDP).

### Artefacts de Release

Chaque release GitHub contient 3 binaires prêts à déployer :
```
buzzcontrol-vX.Y.0-windows-amd64.exe    # Serveur Windows (portable)
buzzcontrol-vX.Y.0-linux-arm64          # Serveur Raspberry Pi (portable)
buzzclick-vX.Y.0-firmware.bin           # Firmware ESP32-C3 (flash via USB/OTA)
```

Tous les binaires sont **portables** et autonomes (pas de dépendances externes).

## Commandes Essentielles

```bash
# Développement
cd server-go
go build -o server.exe ./cmd/server && ./server.exe

# Relancer le serveur (IMPORTANT: toujours utiliser cette méthode)
curl -s http://localhost/shutdown && sleep 2 && ./server.exe

# Tests unitaires
go test ./... -v -cover

# Build portable
cd server-go && ./build.ps1
```

## Ports Standards

| Service | Port | Endpoint | Description |
|---------|------|----------|-------------|
| HTTP | 80 | `/` | Web interface |
| WebSocket | 80 | `/ws` | Admin client (alias vers `/ws/admin`, legacy rétrocompatible) |
| WebSocket | 80 | `/ws/admin` | Admin web client — ALL messages (v3.8.0+) |
| WebSocket | 80 | `/ws/tv` | TV display client — game state + enrollment (v3.8.0+) |
| WebSocket | 80 | `/ws/player` | VPlayer client — player enrollment + game state (v3.8.0+) |
| WebSocket | 80 | `/ws/buzzer` | Buzzers physiques (v3.0.0+) — whitelist: LED_SET, OTA_UPDATE, WIFI_CONFIG, HELLO |
| WebSocket | 80 | `/ws/logs` | Logs temps reel |
| TCP | 1234 | - | BuzzClick buzzer protocol (v1, retrocompatible) |
| UDP | 1234 | - | Broadcast (same as TCP) |
| DNS | 53 | - | Captive portal (optional) |

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

### UDP Broadcast Server Discovery (v3.2.0)

Le serveur envoie des heartbeats UDP periodiques pour que les buzzers BuzzClick puissent decouvrir automatiquement l'IP du serveur sans configuration manuelle.

**Format heartbeat** : `BUZZ_SERVER|IP1|IP2|...|PORT\0` (null-termine)

```go
// BroadcasterManager — server-go/internal/server/broadcaster.go
// Intervalle normal : 5s | Enrollment mode : 1s | High-freq : 500ms (v3.6.6)
// Envoie un heartbeat immediat au demarrage
bm := NewBroadcasterManager(udpBroadcaster, httpPort)
bm.Start()
bm.SetEnrollmentMode(true)   // accelère pendant l'appairage
bm.SetHighFrequency(true)    // 500ms si ≥1 buzzer physique déconnecté (v3.6.6)
```

Depuis v3.6.6 : `SetHighFrequency(bool)` passe en mode 500 ms si ≥1 buzzer physique est déconnecté (vs 5 s normal). Priorité des intervalles : enrollment (1 s) > high-freq (500 ms) > normal (5 s). Appelé automatiquement par `updateBroadcasterFrequency()` sur connect/disconnect/delete buzzer et au démarrage du serveur.

**Chaine de fallback firmware** (click_WifiManager.h) :
1. UDP Broadcast (timeout 30s) → IPs du heartbeat, essai dans l'ordre
2. IP NVS (`server_ip` en flash)
3. mDNS (`_sock._tcp`)
4. Retry broadcast

**Phases LED nouvelles (boot sequence)** :
- Phase 4 : Jaune pulsant 2 Hz — attente heartbeat UDP
- Phase 5 : Bleu clignotant rapide — tentative connexion sur chaque IP

**Fichiers cles** :
- `server-go/internal/server/broadcaster.go` : `BroadcasterManager`, `BuildHeartbeat`, `GetServerIPs`
- `server-go/internal/server/udp.go` : `UDPBroadcaster` (envoi multi-interfaces)
- `src/BuzzClick/click_broadcaster.h` : UDP listener AsyncUDP, parser, etat decouverte
- `src/BuzzClick/click_WifiManager.h` : Integration boot sequence + fallback chain

### Default Question Image Endpoints (v3.2.3)

Image affichee sur la TV pour les questions NORMAL et QCM sans media associe.

```
GET    /api/config/default-image  → Sert l'image (custom data/files/ ou SVG embarque en fallback)
POST   /api/config/default-image  → Upload image personnalisee (multipart, champ "file", formats: jpg/png/gif/webp/svg)
DELETE /api/config/default-image  → Supprime l'image personnalisee, retour au SVG embarque
```

Action WebSocket (CONFIG_UPDATE enrichi) :
```json
// Server → Web : apres upload ou suppression
{ "ACTION": "CONFIG_UPDATE", "MSG": { "...", "default_question_image_is_custom": true } }
```

- **Asset embarque** : `server-go/assets/default-question-image.svg` (via `//go:embed`)
- `default_question_image_is_custom` : `true` si image personnalisee active, `false` si SVG fallback
- Envoye aux nouveaux clients a la connexion WebSocket ET broadcast apres chaque changement
- Cache-busting cote frontend : `?t=timestamp` pour forcer le rechargement de l'apercu

### OTA Firmware Endpoints (v3.1.0+)

```
GET  /api/firmware/buzzclick/version           → FirmwareVersionPayload (version, filename, size, exists, embedded_version)
GET  /api/firmware/buzzclick/latest.bin        → Binaire firmware (octet-stream)
POST /api/firmware/buzzclick/upload            → Upload .bin (multipart, champ "file", param "version")
POST /api/firmware/buzzclick/restore-embedded  → Restaure firmware embarqué vers stockage actif (v3.1.1)
POST /api/buzzer/{mac}/update                  → Declenche OTA sur buzzer specifique
POST /api/buzzer/update-all                    → OTA sur tous les buzzers obsoletes
POST /api/buzzer/wifi-config                   → Broadcast WIFI_CONFIG a tous les buzzers WS
```

Actions WebSocket OTA (v3.1.0+) :
```json
// Server → Buzzer : declencher OTA
{ "ACTION": "OTA_UPDATE", "MSG": { "URL": "http://...", "VERSION": "3.1.1", "SIZE": 512000 } }

// Buzzer → Server : progression
{ "ACTION": "OTA_PROGRESS", "MSG": { "STATUS": "downloading", "PERCENT": 42, "ERROR": "" } }

// Server → Web : info firmware de reference (v3.1.1 : + EMBEDDED_VERSION)
{ "ACTION": "FIRMWARE_VERSION", "MSG": { "VERSION": "3.1.1", "FILENAME": "buzzclick-v3.1.1.bin", "SIZE": 512000, "EXISTS": true, "EMBEDDED_VERSION": "3.1.1" } }

// Server → Buzzer : sync config WiFi
{ "ACTION": "WIFI_CONFIG", "MSG": { "SSID": "MyWifi", "PASS": "pass", "SERVER_IP": "192.168.1.84", "SERVER_PORT": 80, "SSID2": "Backup", "PASS2": "pass2" } }
```

### Firmware embarque dans le binaire serveur (v3.1.1)

Le serveur embarque le firmware BuzzClick directement dans son binaire Go :
- **Asset** : `server-go/assets/firmware/buzzclick-latest.bin` (embarque via `//go:embed`)
- **Endpoint restauration** : `POST /api/firmware/buzzclick/restore-embedded`
  - Copie le firmware embarqué vers `data/firmware/buzzclick-latest.bin`
  - Retourne 404 si pas de firmware embarqué
  - Broadcast `FIRMWARE_VERSION` apres restauration
- **`EMBEDDED_VERSION`** : Exposé dans `FirmwareVersionPayload` pour que le frontend affiche le bouton de restauration uniquement si une version embarquée est disponible

### WebSocket Endpoints dédiés (v3.8.0)

Le serveur offre trois endpoints WebSocket spécialisés avec routage intelligent des messages par type de client. Chaque endpoint filtre les actions envoyées au client selon sa fonction.

**Architecture** :
```
/ws/admin    → Admin web client (AccessAll)
/ws/tv       → TV display client (TV seulement) — serialized via SerializeForWebClient()
/ws/player   → VPlayer web client (VPlayer seulement) — serialized via SerializeForWebClient()
/ws (legacy) → Alias vers /ws/admin (rétrocompatibilité)
/ws/buzzer   → Buzzers physiques (whitelist : UPDATE, UPDATE_TIMER, START, CONTINUE, STOP, PAUSE, READY, RESET, HELLO, LED_SET, OTA_UPDATE, WIFI_CONFIG) — serialized via SerializeForBuzzer()
```

**Table de routage des actions** :

| Action | Admin | TV | VPlayer | Buzzer |
|--------|-------|----|---------|----|
| ALL, UPDATE, START, STOP, PAUSE, CONTINUE, REVEAL, RESET, QCM_HINT, ENROLLMENT_UPDATE | ✓ (full) | ✓ (web) | ✓ (web) | ✓ (buzzer) |
| UPDATE_TIMER | ✓ (full) | ✓ (web) | ✓ (web) | ✓ (buzzer) |
| READY, REMOTE, BACKGROUND_CHANGE, CONFIG_UPDATE, SHOW_QR_CODE, HIDE_QR_CODE, FULL, MEMORY_* | ✓ (full) | ✓ (web) | - | - |
| QUESTIONS, CLIENTS, FIRMWARE_VERSION | ✓ (full) | - | - | - |
| PLAYER_REJECTED, PLAYER_CONNECTED, PLAYER_ASSIGNED | ✓ (full) | - | ✓ (web) | - |
| LED_SET, OTA_UPDATE, WIFI_CONFIG, HELLO | ✓ (full) | - | - | ✓ (buzzer) |

**Sérialisation par client (v3.8.0)** :
- **SerializeForAdmin()** : full payload — tout (bumpers avec FIRMWARE_VERSION, OTA_STATUS, ACK_PENDING, config, etc.)
- **SerializeForWebClient()** : réduit pour TV/VPlayer — strips FIRMWARE_VERSION, IS_OUTDATED, OTA_STATUS, OTA_PERCENT, ACK_PENDING, config info (réduit volume ~40-60%)
- **SerializeForBuzzer()** : minimal pour buzzers physiques — phase, timer, bumpers (ID, NAME, TEAM, CONNECTED), teams (NAME, COLOR, STATUS)

**Implémentation côté backend** :
- `websocket.go` : `HandleConnectionWithType(w, r, clientType)`, `SetClientType()`, `BroadcastToTypes(msg, ...ClientType)` — acheminement par type
- `http.go` : routes `/ws/admin`, `/ws/tv`, `/ws/player` instancient la connexion avec le type approprié
- `cmd/server/main.go` : tous les `broadcastXxx()` calls remplacés par `BroadcastToTypes(...ClientType)` selon la table
- Backward compat : `/ws` route remains, forwards vers `/ws/admin` (ClientTypeAdmin)

**Implémentation côté frontend** :
- `useWebSocket.js` : ajout paramètre `endpoint` — l'URL WS peut cibler un endpoint spécifique
- `GameProvider` : accepte prop `endpoint = '/ws/admin'` (v3.8.0) — transmis à `useWebSocket()` pour routing multi-rôle
- `App.jsx` routing des endpoints :
  - `/admin/*` → `GameProvider` avec défaut (`/ws/admin`)
  - `/tv` → `GameProvider` avec `endpoint="/ws/tv"` (TV display)
  - `/player` et `/enroll` → `GameProvider` avec `endpoint="/ws/player"` (VPlayer client)
- Permet l'évolution future (ex: TV + admin en iframe simultanément)
- `PlayerDisplay.jsx` : `/ws/tv` — la connexion TV est indépendante de l'admin
- `EnrollPage.jsx`, `VPlayerPage.jsx` : `/ws/player` — connexion dédiée VPlayer

### Game Init — Nouvelle Partie (v4.0.4)

**Phase NEW_GAME** : état transitoire déclenché par le bouton "NOUVELLE PARTIE" (admin, phase STOPPED uniquement).
- Reset : scores teams + joueurs à 0, historique vide, tous les statuts de questions → AVAILABLE, question sélectionnée → nil
- Transition : `NEW_GAME → PREPARE` automatique à la sélection de la première question
- TV (`/tv`) : affiche l'écran "NOUVELLE PARTIE À VENIR" avec les métadonnées du quiz

**Métadonnées de quiz** (v4.0.1) :
- `QUIZ_NAME`, `QUIZ_THEME`, `QUIZ_NOTES` — champs de `GameState`, toujours sérialisés (sans omitempty)
- Persistés dans le GameState, inclus dans sauvegardes/restaurations
- Mis à jour via action WS `UPDATE_QUIZ_META` (payload : `NAME`, `THEME`, `NOTES`)
- Affichés sur l'écran TV phase NEW_GAME

**Fonds d'écran NEW_GAME** (v4.0.4) :
- Stockés dans `data/files/new-game-backgrounds/` + `backgrounds.json` — array `[]Background{Path, Duration, Opacity}`
- API REST :
  - `POST /new-game-backgrounds` — upload image (multipart, champ "file", formats: jpg/png/gif/webp/svg)
  - `PUT /new-game-backgrounds` — mise à jour configuration (ordre, durée, opacité)
  - `DELETE /new-game-backgrounds?file=xxx` — suppression image spécifique ou tous les fonds (sans paramètre)
  - `GET /files/new-game-backgrounds/{name}` — serve l'image (handler générique statique)
- Sérialisation WebSocket :
  - `new_game_backgrounds []Background` champ dans `GameState` et `ConfigUpdatePayload`
  - Jamais `omitempty` — toujours présent dans CONFIG_UPDATE broadcast même si vide (évite bug UI admin lors de suppression)
- Rotation client-side TV (`PlayerDisplay.jsx`) :
  - `setTimeout` par durée d'image — transition en fonction de la durée stockée
  - Overlay absolu (z-index: 0) indépendant de l'opacité du texte et effets (z-index: 1)
- Fallback dégradé animé :
  - Si aucune image configurée, affichage du dégradé multicolore (violet→bleu→cyan→rose→ambre, 9s infini)
  - Titre avec glow-pulse, étoiles scintillantes (mix-blend-mode: screen)

**Actions WebSocket (v4.0.1)** :
```json
// Admin → Server : déclencher NOUVELLE PARTIE
{ "ACTION": "NEW_GAME", "MSG": {} }

// Admin → Server : mettre à jour métadonnées quiz
{ "ACTION": "UPDATE_QUIZ_META", "MSG": { "NAME": "Mon Quiz", "THEME": "Science", "NOTES": "Questions variées" } }

// Server → Web : broadcast NEW_GAME
{ "ACTION": "ALL", "MSG": { "phase": "NEW_GAME", "quiz_name": "Mon Quiz", "quiz_theme": "Science", "quiz_notes": "Questions variées" } }
```

**Refus NEW_GAME** : serveur retourne `REMOTE` avec error si phase ≠ STOPPED ou `MESSAGE` si erreur, ex :
```json
{ "ACTION": "REMOTE", "MSG": { "error": "Cannot start NEW_GAME: current phase is PREPARE, expected STOPPED" } }
```

**Fichiers clés** :
- `server-go/internal/game/engine.go` : `InitGame()` (reset), détection phase `NEW_GAME` → `PREPARE`
- `server-go/internal/game/models.go` : champs `GameState.QuizName`, `GameState.QuizTheme`, `GameState.QuizNotes`, `GameState.NewGameBackgrounds` (v4.0.4)
- `server-go/cmd/server/main.go` : handlers `NEW_GAME` et `UPDATE_QUIZ_META`, broadcast après reset ; `loadNewGameBackgrounds()`, `saveNewGameBackgroundsConfig()`, callback `OnNewGameBackgroundChange` (v4.0.4)
- `server-go/internal/protocol/messages.go` : `ActionNewGame`, `ActionUpdateQuizMeta`, sérialisation NEW_GAME ; `Background` struct (v4.0.4)
- `server-go/internal/server/http.go` : `handleNewGameBackground()` — POST/PUT/DELETE `/new-game-backgrounds` (v4.0.4)
- `web/src/pages/GamePage.jsx` : bouton "NOUVELLE PARTIE" (condition phase === STOPPED)
- `web/src/pages/QuestionsPage.jsx` (renommée QuestionsPage vers QuizPage) : Zone Quiz (3 champs métadonnées + upload fonds) + Zone Ambiance + Zone Questions (v4.0.4)
- `web/src/pages/PlayerDisplay.jsx` : écran NEW_GAME (100vw×100vh, statique) — affichage nom quiz + thème + notes ; rotation fonds d'écran + fallback dégradé (v4.0.4)

### Game Init — MEMOTION (v5.0.0)

**Type de jeu MEMOTION** : Jeu de cartes à 3 faces avec grille interactive, difficulté configurable et mode équipe.

**Structure MotionCard** :
```json
{
  "ID": "mc-1",
  "RECTO_THEME": "Thème de la carte",
  "RECTO_IMAGE": "/files/questions/img_recto.jpg",
  "DIFFICULTY": 2,
  "QUESTION_TEXT": "Question ou énigme",
  "QUESTION_IMAGE": "/files/questions/img_question.jpg",
  "ANSWER_TEXT": "Réponse ou explication",
  "ANSWER_IMAGE": "/files/questions/img_answer.jpg"
}
```

**Points par difficulté** :
- ★ (DIFFICULTY=1) → 1 point
- ★★ (DIFFICULTY=2) → 3 points
- ★★★ (DIFFICULTY=3) → 5 points

**Flux de jeu MEMOTION (9 étapes)** :
1. Admin clique sur une carte dans la grille → `MEMOTION_SELECT` → subphase `SELECTED`
2. TV : la carte sélectionnée zoome en plein écran (animation) affichant RECTO (thème + difficulté + points). Pas de timer.
3. Admin clique à nouveau (ou bouton "Démarrer") → `MEMOTION_FLIP` → subphase `QUESTION` + timer démarre
4. TV : flip 3D de la carte → affichage VERSO (question + image). Timer visible.
5. Timer s'écoule OU admin clique "STOP TIMER" → `MEMOTION_STOP_TIMER` → timer s'arrête, subphase reste `QUESTION`
6. Admin clique "RÉVÉLER" → `MEMOTION_REVEAL` → subphase `REVEAL`
7. TV : flip 3D → affichage REVEAL (réponse + image) plein écran.
8. Admin clique sur l'équipe gagnante (ou "Aucun") → `MEMOTION_DONE` → carte retourne en grille avec couleur équipe
9. TV : zoom retour en grille + carte marquée DONE avec couleur d'équipe. Équipe suivante sélectionnée.

**Subphases MEMOTION** :
- `GRID` : grille de cartes affichée, faces RECTO visibles, prêtes à être sélectionnées
- `SELECTED` : carte sélectionnée zoomée en plein écran (RECTO thème + points), admin peut flipper ou annuler. **Pas de timer.**
- `QUESTION` : flip 3D → face VERSO (question) plein écran 100vw×100vh, timer per-carte actif
- `REVEAL` : flip 3D → face REVEAL (réponse) plein écran, admin attribue points via UI

**Champs GameState** (v5.0.0) :
- `MEMOTION_SUBPHASE` : string (`"GRID"` | `"SELECTED"` | `"QUESTION"` | `"REVEAL"` | `""`)
- `MEMOTION_SELECTED` : string (ID carte sélectionnée, `""` quand en GRID)
- `MEMOTION_CARD_STATES` : map[string]string (CARD_ID → `"UNPLAYED"` | `"SELECTED"` | `"QUESTION"` | `"REVEALED"` | `"DONE"`)
- `MEMOTION_CARD_TEAMS` : map[string]string (CARD_ID → teamName quand DONE)
- `MEMOTION_CURRENT_TEAM` : string (équipe active)
- `MEMOTION_PARTICIPATING_TEAMS` : []string (équipes participant au jeu)
- `MEMOTION_CURRENT_TEAM_COLOR` : [3]int (RGB couleur équipe actuelle)
- **Règle** : aucun de ces champs n'utilise `omitempty` — toujours sérialisés même si vides (évite réinitialisations manquées côté frontend)

**Modes de jeu MEMOTION** (même logique que MEMORY) :
- `SOLO` : aucune rotation équipe, une seule équipe joue
- `CHACUN_SON_TOUR` : rotation entre équipes après chaque carte
- `TANT_QUE_JE_GAGNE` : équipe conserve tour si elle gagne, passe sinon

**Actions WebSocket (v5.0.0)** :
```json
// Admin → Server : étape 1 — sélectionner une carte de la grille (→ SELECTED, pas de timer)
{ "ACTION": "MEMOTION_SELECT", "MSG": { "CARD_ID": "mc-1" } }

// Admin → Server : étape 3 — flipper la carte sélectionnée (→ QUESTION + timer démarre)
{ "ACTION": "MEMOTION_FLIP", "MSG": {} }

// Admin → Server : étape 5 — arrêter le timer manuellement (subphase reste QUESTION)
{ "ACTION": "MEMOTION_STOP_TIMER", "MSG": {} }

// Admin → Server : étape 6 — révéler la réponse (→ REVEAL, timer arrêté)
{ "ACTION": "MEMOTION_REVEAL", "MSG": {} }

// Admin → Server : étape 8 — terminer la carte (→ DONE, retour GRID, points attribués)
{ "ACTION": "MEMOTION_DONE", "MSG": { "CARD_ID": "mc-1", "WINNER_TEAM": "team_A" } }
// WINNER_TEAM="" → pas de vainqueur, pas de points

// Admin → Server : configurer équipes participantes (pendant PREPARE ou READY)
{ "ACTION": "MEMOTION_SET_TEAMS", "MSG": { "TEAMS": ["team_A", "team_B"] } }

// Server → Web : broadcast état complet
{ "ACTION": "UPDATE", "MSG": { "MEMOTION_SUBPHASE": "QUESTION", "MEMOTION_SELECTED": "mc-1", "MEMOTION_CURRENT_TEAM": "team_A", ... } }
```

**Timer par carte** :
- Démarre au FLIP (subphase SELECTED → QUESTION via `MEMOTION_FLIP`)
- Durée configurée dans le champ `Time` de la Question (format "30" pour 30s, "0" = pas de timer)
- Affiché sur TV (composant `<Timer>`) pendant la subphase QUESTION
- Timer expiré → CurrentTime reste à 0, subphase reste QUESTION — admin doit agir (STOP ou REVEAL)
- `MEMOTION_STOP_TIMER` : arrêt manuel du timer, subphase reste QUESTION
- `MEMOTION_REVEAL` : arrête aussi le timer (via `StopMotionCardTimer()`) avant de passer à REVEAL

**Annulation depuis SELECTED** :
- `MEMOTION_DONE` avec `WINNER_TEAM=""` depuis SELECTED : annule la sélection, carte retourne à UNPLAYED, grille restaurée
- DoneMotionCard accepte `SELECTED` comme subphase valide (annulation propre)

**Interface Admin (GamePage)** — panneaux par subphase :
- `GRID` : grille de boutons (un par carte), click → `MEMOTION_SELECT`, cartes DONE désactivées avec couleur équipe
- `SELECTED` : affiche thème + difficulté, bouton "DÉMARRER" → `MEMOTION_FLIP`, bouton "RETOUR" → `MEMOTION_DONE` vide (cancel)
- `QUESTION` : affiche texte question, timer actif, bouton "STOP TIMER" (si timer>0) → `MEMOTION_STOP_TIMER`, bouton "RÉVÉLER" → `MEMOTION_REVEAL`
- `REVEAL` : affiche réponse, chips équipes → `MEMOTION_DONE` avec WINNER_TEAM, bouton "Aucun" → DONE vide

**Interface TV (PlayerDisplay)** — vues par subphase :
- `GRID` : grille responsive (motionCols calculé selon nb cartes), cards avec framer-motion `layoutId={mc-${card.ID}}`, DONE cards retournées
- `SELECTED` : la carte sélectionnée zoome depuis sa position grille vers plein écran via `layoutId` framer-motion — affiche RECTO (thème + image + difficulté + points)
- `QUESTION` : AnimatePresence flip rotateY -90→0, affiche VERSO (question text/image) + Timer
- `REVEAL` : AnimatePresence flip rotateY 90→0 (direction opposée), affiche réponse text/image
- **Contrainte TV** : `overflow: hidden`, dimensions viewport — voir section Contrainte Affichage TV

**Animations framer-motion** :
- GRID → SELECTED : `layoutId` sur chaque card dans la grille + même `layoutId` sur overlay fullscreen → zoom automatique depuis la grille
- Carte en grille cachée (visibility: hidden) quand c'est la carte selected/active (évite doublon visuel)
- SELECTED → QUESTION : `AnimatePresence` key="selected"→"question", `initial={{ rotateY: -90 }}`
- QUESTION → REVEAL : `AnimatePresence` key="question"→"reveal", `initial={{ rotateY: 90 }}` (direction opposée)
- DONE → GRID : `layoutId` anime le retour de la carte fullscreen vers sa position grille

**Fichiers clés** :
- `server-go/internal/game/models.go` : struct `MotionCard`, champs `GameState.Motion*` (NO omitempty), `QuestionTypeMemotion` (v5.0.0)
- `server-go/internal/game/engine.go` : `SelectMotionCard()` (→SELECTED), `FlipMotionCard()` (→QUESTION+timer), `RevealMotionCard()`, `DoneMotionCard()` (accepte SELECTED pour annulation), `SetMotionParticipatingTeams()`, `StartMotionCardTimer()`, `StopMotionCardTimer()` (v5.0.0)
- `server-go/internal/game/engine_memotion_test.go` : tests unitaires MEMOTION (v5.0.0)
- `server-go/internal/protocol/messages.go` : `ActionMotionSelect`, `ActionMotionFlip`, `ActionMotionStopTimer`, `ActionMotionReveal`, `ActionMotionDone`, `ActionMotionSetTeams` (v5.0.0)
- `server-go/cmd/server/main.go` : `handleMotionSelect`, `handleMotionFlip`, `handleMotionStopTimer`, `handleMotionReveal`, `handleMotionDone`, `handleMotionSetTeams` (v5.0.0)
- `web/src/pages/GamePage.jsx` : panneaux admin MEMOTION (GRID/SELECTED/QUESTION/REVEAL subphase controls) (v5.0.0)
- `web/src/pages/QuestionsPage.jsx` : Zone MEMOTION — éditeur grille, upload images, difficulté (v5.0.0)
- `web/src/pages/PlayerDisplay.jsx` : vues TV GRID/SELECTED/QUESTION/REVEAL, framer-motion layoutId + AnimatePresence flip (v5.0.0)
- `web/src/pages/PlayerDisplay.css` : classes `.memotion-*` pour TV (plein écran, flip 3D, grille)

### Modele Bumper enrichi (v3.1.0)

```json
{
  "ID": "AA:BB:CC:DD:EE:FF",
  "FIRMWARE_VERSION": "3.1.1",
  "IS_OUTDATED": false,
  "OTA_STATUS": "",
  "CONNECTED": true,
  "ACK_PENDING": false
}
```

**IS_OUTDATED** : Remis a false uniquement au reboot du buzzer (reception HELLO avec nouvelle version).
Ne change pas sur `OTA_PROGRESS done` - le badge reste orange jusqu'au reboot confirme.

**CONNECTED** (v3.6.6) : `true` à la connexion WebSocket (`handleHello`), `false` à la déconnexion (`OnBuzzerDisconnected`). Toujours sérialisé (sans `omitempty`) — le frontend affiche un badge ⚠ jaune quand `CONNECTED === false` (condition stricte : `!IS_VIRTUAL && !IS_VPLAYER`). La déconnexion déclenche aussi le mode haute-fréquence UDP (500 ms) dans `BroadcasterManager` pour accélérer la redécouverte serveur par le buzzer.

**Reconnexion rapide (v3.6.8)** : `OnBuzzerDisconnected` vérifie `IsClientConnected(mac)` avant de poser `CONNECTED=false`. Si le buzzer a déjà reconnecté (nouvelle connexion WS active pour le même MAC), le callback de l'ancienne connexion zombie est ignoré — la reconnexion < 5 s est transparente, aucun badge ne clignote.

Pages frontend affichant le badge ⚠ :
- `TeamCard.jsx` : inline dans la row du buzzer (v3.6.6)
- `TeamsPage.jsx` : dans les rows membres et buzzers non assignés (v3.6.8)
- Style inline React (`background:#f59e0b`, SVG `stroke:white`) — pas de classe CSS pour éviter les problèmes de chargement (v3.6.8)

**Initialisation au démarrage (v3.6.7)** : Lors du chargement des bumpers depuis le disque, `CONNECTED` est réinitialisé à `false` pour tous les bumpers, car aucun buzzer n'est physiquement connecté après un redémarrage serveur. Cela évite que les bumpers persistés avec `CONNECTED=true` masquent le badge ⚠.

**ACK_PENDING (v3.8.0)** : `true` quand le serveur attend confirmation (ACK) du buzzer pour un message prioritaire (LED_SET, OTA_UPDATE, WIFI_CONFIG). Positionné par `AckManager` à la génération du message, remis à `false` à la réception de l'ACK ou après expiry (max retries). Toujours sérialisé (sans `omitempty`). Badge ⚠ horloge (SVG, fond amber `#f59e0b`) affiché quand `ACK_PENDING === true` (condition stricte : `!IS_VIRTUAL && !IS_VPLAYER`).

Pages frontend affichant le badge ⚠ ACK_PENDING :
- `TeamCard.jsx` : inline dans la row du buzzer
- `TeamsPage.jsx` : dans les rows membres et buzzers non assignés
- Style inline React identique au badge CONNECTED (icône SVG horloge, `background:#f59e0b`, `stroke:white`)

### Server-Driven LED Control (v3.4.0)

Le serveur pilote les LEDs des buzzers via une action unique `LED_SET`. Le firmware applique simplement ce qu'il recoit — aucune logique LED locale.

**Protocole** :
```json
// Server → Buzzer (per-buzzer via buzzerHub.SendToClient ou broadcast)
{ "ACTION": "LED_SET", "MSG": { "COLOR": [255, 0, 0], "INTENSITY": 255, "EFFECT": "SOLID" } }

// COMET : champ COMET_COLOR optionnel — couleur de la comète (v3.7.0)
{ "ACTION": "LED_SET", "MSG": { "COLOR": [255, 0, 0], "INTENSITY": 255, "EFFECT": "COMET", "COMET_COLOR": [255, 215, 0] } }
```

Effects : `"SOLID"` (fixe), `"BLINK"` (100%<->25% a 400ms), `"DIM"` (fixe attenue), `"COMET"` (bande rotative 23 LEDs, 2 tours ~3.3 s — v3.7.0)

**COMET_COLOR** (v3.7.0) : champ optionnel dans `LEDSetPayload` — serveur calcule or `[255,215,0]` ou blanc `[255,255,255]` selon contraste avec la couleur d'équipe (dist² euclidien RGB < 8000 → blanc). Firmware utilise `cometR/G/B` du payload au lieu du gold hardcodé.

**Logique serveur par phase** :
- `READY`/`START` QCM : couleur reponse SOLID 100% (per-buzzer)
- `READY`/`START` NORMAL/MEMORY : couleur equipe SOLID 100% (Memory actif=100%, inactif=25%)
- `PAUSE` (buzzer presse) QCM : couleur reponse DIM 64 ; NORMAL : buzzer actif=SOLID 255, autres=DIM 5
- `PAUSE ALL` : couleur equipe DIM 64 pour tous
- `STOP` : couleur equipe SOLID 100% pour tous
- `REVEALED` QCM : correct=BLINK, wrong-buzzed=SOLID 100%, non-buzze=DIM 25
- Reconnect HELLO : `resendLEDOnReconnect()` renvoie le dernier etat connu (`bumperLEDState` map)
- Attribution de points (`TEAM_POINTS`/`BUMPER_POINTS`) : `sendLEDSetComet()` → COMET avec `COMET_COLOR` dynamique sur les buzzers de l'équipe (~3.3 s, 2 tours) (v3.7.0)
- Sélection couleur LED par teinte : `nearestPaletteColorByHue()` — helper RGB→HSL, distance de teinte HSL pour cohérence palette (v3.7.0)

**Patterns d'erreur LED locaux** (firmware-side, quand le serveur est injoignable) :

| Pattern | Visuel | Déclencheur |
|---------|--------|-------------|
| `WIFI_FAILED` | Rouge clignotant 1 Hz | WiFi non associé / fallback épuisé |
| `WS_DISCONNECTED` | Rouge clignotant 4 Hz | (réservé, non utilisé depuis v3.6.3) |
| `WS_TIMEOUT` | Rouge pulsant lent ~0.5 Hz | Timeout connexion initiale au boot |
| `WS_RECONNECTING` | 1 pixel blanc tournant 100ms/step (~2.3s/tour) | Dès DISCONNECTED/ERROR — ring préservé |
| `OTA_ERROR` | Rouge fixe + flash blanc 2s | Échec download/flash OTA |

`WS_RECONNECTING` : delta-update — seuls 2 pixels modifiés par tick (restauration du pixel précédent au fond + avance du pixel blanc). Pas de réécriture complète du buffer → la couleur d'équipe (LED_SET) et la rotation grise restent intactes. Vitesse : 100 ms/step → ~2.3 s/tour. `manageLedError()` est appelé dans la boucle d'attente de `connectWebSocket()` pour maintenir l'animation pendant que `loop()` est bloqué (jusqu'à 10 s). Un seul `showPixels()` par tick. **Freeze pendant `ws_safe_destroy()` éliminé (v3.6.4)** : `stop()+destroy()` sont déchargés sur une tâche FreeRTOS temporaire (`ws_destroy_task`, 4 KB, prio 5) — `loop()` continue d'animer via `manageLedError()` pendant le teardown.

**Protocole ACK v3.8.0** :
```json
// Server → Buzzer : LED_SET avec MSG_ID (génération + enregistrement AckManager)
{ "ACTION": "LED_SET", "MSG_ID": "a1b2c3d4e5f6", "MSG": { "COLOR": [255, 0, 0], "INTENSITY": 255, "EFFECT": "SOLID" } }

// Buzzer → Server : ACK avant d'appliquer l'action
{ "ACTION": "ACK", "MSG": { "ack_action": "LED_SET", "ack_id": "a1b2c3d4e5f6" } }

// Server → Web : ACK_PENDING cleared, AckPending=false broadcast via UPDATE
{ "ACTION": "UPDATE", "MSG": { "bumpers": [{ "ID": "...", "ACK_PENDING": false }] } }
```

- `AckManager` : registre des MSG_ID en attente, retry automatique + expiry après `ack_max_retries` tentatives (default 3, timeout 2000ms)
- `MSG_ID` optionnel (omitempty) — anciens firmwares sans support ACK restent compatibles
- ACK envoyé BEFORE action apply (côté firmware) — minimise latence de confirmation
- Non-réception d'ACK détectée par expiry → message rebroadcasted via `OnRetry`, puis après max retries `OnExpired` clears AckPending

### Payload Serializers — Client-Specific Reduction (v3.8.0)

Le serveur génère 3 variantes de sérialisation pour optimiser le volume de données selon le type de client.

**Trois sérialiseurs** :
```go
// internal/protocol/messages.go
SerializeForAdmin()      // Full payload : tous les champs (bumpers + config + firmware + ACK_PENDING)
SerializeForWebClient()  // Réduit pour TV/VPlayer : strips FIRMWARE_VERSION, IS_OUTDATED, OTA_STATUS, OTA_PERCENT, ACK_PENDING, config
SerializeForBuzzer()     // Minimal pour buzzers physiques : phase, timer, bumpers (ID, NAME, TEAM, CONNECTED), teams (NAME, COLOR, STATUS)
```

**Allocation de sérialisation par endpoint (websocket.go)** :
- `/ws/admin` → `SerializeForAdmin()` (full)
- `/ws/tv`, `/ws/player` → `SerializeForWebClient()` (réduit ~40-60% moins de données)
- `/ws/buzzer` → `SerializeForBuzzer()` (minimal, optimisé pour faible bande passante)

**Broadcast implementation (main.go)** :
- `BroadcastRawToTypes()` applique le sérialiser approprié selon le type de chaque client
- Réduction bande passante significative : TV/VPlayer n'envoient pas les infos sensibles/inutiles
- Buzzers reçoivent exactement ce dont ils ont besoin pour afficher l'état du jeu

**Fichiers cles** :
- `server-go/cmd/server/main.go` : `sendLEDSet`, `broadcastLEDSet`, `resendLEDOnReconnect`, `sendLEDSetAllBuzzers`, `sendLEDSetStop`, `sendLEDSetPause`, `sendLEDSetReveal`, `sendLEDSetComet` (v3.7.0) + ACK wiring (v3.8.0) + `BroadcastRawToTypes()` (v3.8.0)
- `server-go/internal/protocol/messages.go` : `ActionLEDSet`, `LEDSetPayload` (champ `CometColor [3]int` — v3.7.0), `Message.MsgID` (v3.8.0, omitempty), `ActionACK`, `AckPayload` (v3.8.0), `nearestPaletteColorByHue()` (v3.7.0) ; `SerializeForAdmin()`, `SerializeForWebClient()`, `SerializeForBuzzer()` (v3.8.0)
- `server-go/internal/game/models.go` : `Bumper.AckPending bool` (v3.8.0)
- `server-go/internal/server/ack_manager.go` : ACK registre + retry/expiry logic (v3.8.0, NEW)
- `server-go/internal/config/config.go` : `ServerConfig.AckTimeoutMs`, `AckMaxRetries` (v3.8.0)
- `src/BuzzClick/click_websocket_espidf.h` : `ws_sendAck()` function (v3.8.0)
- `src/BuzzClick/click_serverConnection.h` : handler `LED_SET` dans `parseJSON()`, MSG_ID check + ACK send before action (v3.8.0), `manageLedBlink()` (EFFECT=BLINK), `manageLedComet()` (EFFECT=COMET, v3.7.0)
- `src/BuzzClick/click_MAIN.cpp` : `manageLedBlink()`, `manageLedComet()` dans `loop()` (v3.7.0)
- `src/BuzzClick/click_ledErrorPatterns.h` : patterns animés, `manageLedError()` dans `loop()`

### Contrainte Affichage TV - IMPORTANT
**L'affichage TV (`/tv`) est STATIQUE et ne permet PAS de scroll.**
Toutes les vues TV doivent tenir entièrement à l'écran sans défilement :
- Utiliser `overflow: hidden` (jamais `auto` ou `scroll`)
- Dimensionner avec des unités viewport (`vh`, `vw`, `%`)
- Utiliser `flex` avec `min-height: 0` pour permettre le rétrécissement
- Limiter le contenu visible (ex: top 3, max 6 catégories)

### Key Files
- **Backend** : `server-go/cmd/server/main.go`, `internal/game/engine.go`, `internal/game/models.go`, `internal/updater/updater.go`
- **WebSocket Buzzer (v3.0.0)** :
  - `internal/server/websocket_buzzer.go` : Hub WebSocket dedie aux buzzers physiques (BuzzerWebSocketHub) — ping ticker 3 s, ReadDeadline 5 s, detection ≤ 5 s (v3.6.8) ; whitelist (12 actions) + `BroadcastIfRelevant()` (v3.8.0)
  - `internal/server/websocket.go` : Hub WebSocket clients web + routing par ClientType — `HandleConnectionWithType(w, r, clientType)`, `BroadcastToTypes(msg, ...ClientType)`, `BroadcastRawToTypes()` (sérialiseurs), `SetClientType()` (v3.8.0) ; ping ticker 3 s, ReadDeadline 5 s (v3.6.8)
  - `internal/server/http.go` : Routes HTTP + handlers WebSocket (`/ws` alias /ws/admin, `/ws/admin`, `/ws/tv`, `/ws/player`, `/ws/buzzer`, `/ws/logs`) (v3.8.0)
  - `internal/protocol/messages.go` : Serialisation JSON (SerializeForWebSocket, Serialize) ; `SerializeForAdmin()`, `SerializeForWebClient()`, `SerializeForBuzzer()` (v3.8.0) ; MSG_ID + ACK protocol (v3.8.0)
  - `internal/protocol/parser.go` : Parsing JSON (ParseSingle pour WebSocket, Parse pour TCP)
- **ACK Buzzer Protocol (v3.8.0)** :
  - `internal/server/ack_manager.go` : AckManager registre — GenerateMsgID, Register, Confirm, ClearByMAC, PendingCount, Start, tick loop (NEW)
  - `internal/config/config.go` : AckTimeoutMs, AckMaxRetries config params
  - `internal/game/models.go` : Bumper.AckPending field
  - `cmd/server/main.go` : Full ACK wiring — sendLEDSet/sendWifiConfigToBuzzer MSG_ID generation, handleBuzzerACK parsing, OnRetry/OnExpired callbacks
- **OTA Firmware (v3.1.0)** :
  - `internal/server/firmware.go` : FirmwareManager (stockage, versioning, comparaison semver)
  - `internal/server/http_firmware.go` : Handlers OTA (`/api/firmware/*`, `/api/buzzer/*/update`)
  - `src/BuzzClick/click_otaManager.h` : Module OTA ESP32-C3 (download HTTP + flash partition) — `performOTA()` exécuté dans `ota_task` FreeRTOS (16 KB stack) depuis v3.7.0 (évite déconnexion WebSocket à ~20% download)
- **Frontend** :
  - `web/src/pages/GamePage.jsx` : Page admin principale (jeu en cours) — bouton "NOUVELLE PARTIE" en phase STOPPED (v4.0.1)
  - `web/src/pages/QuestionsPage.jsx` : Page Quiz (ex-QuestionsPage) — 3 zones : Quiz (métadonnées NAME/THEME/NOTES), Ambiance (fonds d'écran), Questions (liste) (v4.0.1)
  - `web/src/pages/ConfigPage.jsx` : Configuration serveur (parametres, effet neon, WiFi USB, OTA firmware)
  - `web/src/pages/BackupPage.jsx` : Sauvegarde/Restauration/Reinitialisation
  - `web/src/pages/UpdatePage.jsx` : Gestion des mises a jour automatiques
  - `web/src/pages/PlayerDisplay.jsx` : Affichage TV (STATIQUE) ; utilise `/ws/tv` endpoint (v3.8.0) — écran "NOUVELLE PARTIE À VENIR" en phase NEW_GAME avec quiz metadata (v4.0.1)
  - `web/src/components/TeamCard.jsx` : Carte equipe/joueurs (+ badge firmware + modal OTA + badge ⚠ CONNECTED + badge ⚠ ACK_PENDING horloge)
  - `web/src/pages/TeamsPage.jsx` : Gestion equipes/membres (+ badge ⚠ CONNECTED v3.6.8 + badge ⚠ ACK_PENDING horloge v3.8.0)
  - `web/src/styles/badges.css` : CSS partagé (présent mais badge CONNECTED utilise styles inline React depuis v3.6.8)
  - `web/src/components/Navbar.jsx` : Navigation + menu abeille (dropdown)
  - `web/src/components/USBConfigModal.jsx` : Modale USB unifiee — point d'entree unique pour config WiFi AT et flash firmware (v3.1.2)
  - `web/src/hooks/useWebSocket.js` : Hook WebSocket client ; ajout paramètre `endpoint = '/ws/admin'` pour routing par ClientType (v3.8.0)
  - `web/src/hooks/GameContext.jsx` : GameProvider wrapper — accepte prop `endpoint` (default `/ws/admin`) transmis à `useWebSocket()` pour endpoint-specific routing (v3.8.0) ; App.jsx routes : `/admin/*` défaut, `/tv` → `/ws/tv`, `/player`/`/enroll` → `/ws/player`
  - `web/src/hooks/useWebSerial.js` : Hook Web Serial API pour communication AT
  - `web/src/hooks/useEspFlash.js` : Hook flash firmware via esptool-js (v3.1.0)
  - `web/src/utils/colorUtils.js` : Helpers couleurs équipes (v3.7.0) — `boostTeamColor(hex)` (RGB→HSL→RGB, saturation/luminosité TV), `nearestPaletteColorByHue()` (sélection palette par distance de teinte HSL)
- **Firmware BuzzClick** :
  - `src/BuzzClick/click_nvsConfig.h` : Stockage NVS (WiFi, IP serveur, port, ssid2/pass2)
  - `src/BuzzClick/click_usbConfig.h` : Protocole AT sur USB serie
  - `src/BuzzClick/click_WifiManager.h` : Connexion WiFi, boot sequence + fallback chain UDP (v3.2.0)
  - `src/BuzzClick/click_broadcaster.h` : UDP listener AsyncUDP, parser BUZZ_SERVER, etat decouverte (v3.2.0)
  - `src/BuzzClick/click_websocket_espidf.h` : Client WebSocket ESP-IDF (v3.5.3+, flag USE_WEBSOCKET) — variables `volatile` cross-task (wsClient, wsConnected, wsGeneration, wsConnecting), generation counter pour callbacks stales, `ws_safe_destroy()` + flag `wsConnecting` pour race condition FreeRTOS (v3.6.3) ; `ws_destroy_task` FreeRTOS pour teardown non-bloquant (v3.6.4) ; `websocket_task` stack 8192 bytes depuis v3.7.0 (était 4096 — stack overflow sur messages larges)
  - `src/BuzzClick/click_serverConnection.h` : Handlers messages serveur (OTA_UPDATE, WIFI_CONFIG) — `manageLedComet()` (EFFECT=COMET, v3.7.0)
  - `src/BuzzClick/click_otaManager.h` : OTA manager (download + flash, v3.1.0) — `performOTA()` dans `ota_task` FreeRTOS 16 KB depuis v3.7.0
  - `src/BuzzClick/click_MAIN.cpp` : Setup, factory reset, boucle AT — `manageLedBlink()`, `manageLedComet()` dans `loop()` (v3.7.0)
- **Config** : `server-go/config.json`

### Organisation UI (v2.49.x)

**Menu principal (Navbar)** :
- Liens directs : Jeu, Scores, Équipes, Quiz, Historique, Palmarès (v4.0.1 : "Questions" → "Quiz")
- Menu abeille (🐝 dropdown) : Config, Backup/Restaure, Logs, Mises à jour

**Répartition des fonctionnalités** :
| Page | Fonctionnalités |
|------|-----------------|
| `/admin/game` | Contrôle du jeu, affichage équipes, timer — bouton "NOUVELLE PARTIE" en phase STOPPED (v4.0.1) |
| `/admin/quiz` | **Zone Quiz** (métadonnées Nom/Thème/Notes) + **Zone Ambiance** (fonds d'écran) + **Zone Questions** (CRUD + équilibre) (v4.0.1) |
| `/admin/config` | Paramètres serveur, effet néon, mode démo, **WiFi defaults + SSID2**, **OTA firmware buzzer** |
| `/admin/backup` | Sauvegarde, restauration, réinitialisation |
| `/admin/logs` | Logs serveur en temps réel |
| `/admin/updates` | Vérification et installation mises à jour |

**Décisions d'architecture (v4.0.1)** :
- Métadonnées du quiz (Nom/Thème/Notes) centralisées dans Zone Quiz de la page `/admin/quiz` (v4.0.1)
- Fonds d'écran gérés dans Zone Ambiance (ancien comportement conservé)
- Questions et filtres dans Zone Questions (ancien comportement conservé)
- Label "Questions" → "Quiz" pour clarifier le scope du module (v4.0.1)
- Bouton "NOUVELLE PARTIE" déclenche la phase `NEW_GAME` avec reset complet (v4.0.1)
