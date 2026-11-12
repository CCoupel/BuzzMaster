# Serveur Go - BuzzControl

Ce document décrit l'implémentation du serveur Go.

## Structure du Projet

```
server-go/
├── cmd/
│   └── server/
│       ├── main.go           # Entry point + App orchestration (6 217 lignes après v11.5.0)
│       ├── hosts.go          # Implémentations des interfaces Host des paquets extraits
│       └── *_test.go         # Tests d'intégration de l'App (737 tests après v11.5.0)
├── internal/
│   ├── config/
│   │   └── config.go         # Configuration management
│   ├── server/
│   │   ├── http.go           # HTTP + static files + question upload
│   │   ├── http_test.go
│   │   ├── websocket.go      # WebSocket server for web clients
│   │   ├── websocket_buzzer.go # WebSocket server for buzzers
│   │   ├── udp.go            # UDP broadcast for time-critical messages (BuzzerDiscoveryPort)
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
│   ├── protocol/
│   │   ├── messages.go       # Message types (ACTION, MSG structure)
│   │   ├── messages_test.go
│   │   ├── parser.go         # JSON parsing
│   │   └── parser_test.go
│   ├── lighting/             # Couche Hue (writer, driver, événements)
│   ├── audio/                # Moteur son
│   ├── vplayer/              # Payloads UPDATE individualisés VPlayer (#291)
│   ├── colors/               # Palette équipes, maths RGB/HSL (#291)
│   ├── coalesce/             # Coalesceur de diffusion (#291)
│   ├── rounds/               # Logique des manches : contexte, application de ROUNDS_UPDATE sur disque (#291, v11.2)
│   ├── startup/              # Démarrage : port, URLs, pages ouvertes (#291)
│   ├── diag/                 # Journalisation sûre des valeurs client, séquence ARDOISE (#291)
│   ├── demo/                 # Données de démo et de développement (#291)
│   ├── led/                  # LED_SET vers les buzzers (#291/#295)
│   ├── ambiance/             # Couche d'ambiance lumineuse (#205, #295)
│   ├── ambiancecfg/          # Réglages d'ambiance : Settings, Partial, Sources, effets et vitesses par zone et par moment (#290, #223)
│   └── effects/              # Effets lumière pas à pas : schéma YAML, validation, rendu, catalogue, rechargement (#223)
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
| WebSocket server (/ws/buzzer) | OK | BuzzClick buzzer protocol (v3.0.0+) |
| UDP broadcast (port 1234) | OK | Firmware discovery via BuzzerDiscoveryPort |
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

## Découpage en paquets et règles de test (v11.5.0)

`cmd/server` ne garde que l'orchestration de l'`App` (câblage, handlers, routes). La logique extraite vit dans des paquets `internal/` testables seuls. Les paquets extraits ne dépendent jamais de `cmd/server` : ils reçoivent l'application par une interface `Host` étroite.

### Interfaces `Host`

| Paquet | Interface `Host` | Rôle |
|--------|------------------|------|
| `internal/led` | `Engine()`, `BuzzerHub()`, `AckManager()`, `BroadcastUpdateTo(...)` | Envoi des `LED_SET` aux buzzers (règles par phase, COMET, ACK, rejeu à la reconnexion) |
| `internal/ambiance` | `Engine()`, `Ctx()`, `TeamNameColor()`, `HasCategoryMeta()`, `ResolveCategoryMeta()` | Writer d'ambiance, scènes, override manuel Flash, clignotement SCORE, extinction à l'arrêt |
| `internal/rounds` | `Engine()`, `QuestionsDirPath()` | Application d'un `ROUNDS_UPDATE` |
| `internal/demo` | `Engine()`, `Config()`, `ResolveCategoryMeta()`, `SaveBackgroundsConfig()` | Données de démo et de développement |

L'`App` implémente ces interfaces dans `cmd/server/hosts.go`. Les paquets `vplayer`, `colors`, `coalesce`, `startup`, `diag` n'ont pas d'interface `Host` : ce sont des fonctions pures ou des helpers sans état applicatif.

### Propriété de l'état

Deux modèles coexistent, à ne pas mélanger :
- **LED** (`internal/led`) : l'état (`bumperLEDState`, `bumperBuzzState`, `ledStateMu`) reste dans l'`App`. Le `Controller` le partage par pointeur (`*map`, `*sync.RWMutex`), car une partie des tests réassigne ces maps.
- **Ambiance** (`internal/ambiance`) : le `Controller` **possède** son état (writer, pilote Hue, verrou, atomics flash/score/chrono). Les champs ont été retirés de l'`App` ; `*App` ne fait que déléguer en une ligne (`ambiance_delegates.go`).

### Moteur d'effets `internal/effects` (v11.7.0, #223)

- **Pur** : pas de goroutine ni d'E/S réseau ; n'importe ni `internal/game` ni `internal/lighting/hue`. `Render(e, ctx, tick)` est déterministe ; `RenderPost` rend `post` à la fin du moment.
- **Format pas à pas** (`light-effects.md` §4) : `Parse` / `ParseFile` (YAML strict), validation interne, `Render`, `LoadDir` (catalogue), `Manager` (`NewManager`, `Run` : signature du dossier toutes les 2 s, `ReloadInterval`), `EnsureDirs` (copie de référence `natifs/`). Les motifs calculés (`uniform`, `shift`, `gauge`, `random`, `gradient`) n'existent plus.
- **Lecteur par zone** : `RunEffects` (goroutine sur `Host.Ctx()`, tic de 100 ms) vit dans `internal/ambiance/effects.go`. Il calcule **une trame par zone existante** (`ambiance`, `animateur`, `public`) avec le réglage de la zone, puis appelle `Writer().NotifyState()` quand une trame change. Zone public sans effet : aucune entrée émise.
- **Vitesse** : durée réelle du tick = `duree_tick_ms` ÷ vitesse, plancher 250 ms ; changement pris au tick suivant (`ambiancecfg.SpeedValues`).
- **Natifs** : lus depuis `assets/effects/*.yaml` (`go:embed`, `assets/embed.go`) : `pulsation`, `chenillard`, `chenillard-traine`, `couleur-equipe`, `degrade`.
- **Horloge injectable** : `Controller.EffectsClock` et `EffectsTick` (par instance) ; `EffectsStep()` pour les tests sans ticker.
- **Précédence** : `ON` / `OFF` / Flash suspendent les trames de la **zone ambiance** seulement. L'essai de groupe (`grouptest.go`) et l'essai d'effet (`…/try`) priment sur le moment vivant pour leur zone ou leur groupe. L'arrêt et l'interrupteur général annulent le lecteur (`quiesceAmbiance`).
- **Zone public** : au démarrage, rien n'est envoyé ; le rétablissement après un essai lit le pont de façon synchrone (1,5 s au plus), puis réécrit l'état.
- **Dépendance** : `go.yaml.in/yaml/v4` (déjà indirecte, désormais directe) — pas de nouvelle dépendance tierce ; `TestCA7_NoNewThirdPartyDependency` la liste.

### Règles pour les tests

1. **Durées injectables par instance, jamais par variable de paquet.** Les tests peuvent tourner en `t.Parallel()` : une variable de paquet raccourcie accélère les timers des tests voisins. Injecter par instance : `Engine.SetTickInterval(d)`, `Engine.SetConnGreenMinDuration(d)`, `questionSoundAdapter.watchdogDelay`, `FlashPhases` du `Controller` ambiance. Valeur `0` = valeur de production.
2. **Barrière positive plutôt que fenêtre fixe pour une assertion négative.** « Aucune trame fautive » n'est vraie qu'une fois que le serveur a fini d'émettre. Après l'action, un témoin est diffusé par le même hub ; les trames WebSocket arrivant dans l'ordre, la réception du témoin prouve que tout ce qui précède est arrivé. Modèle : `cmd/server/settle_barrier_291_test.go`. Les envois différés par un saut de goroutine sont couverts par une marge courte (`settleQuiet291`, 100 ms) ; après une action client asynchrone (HELLO, PLAYER_CONNECT), la marge est `settleQuietAfterClient291` (250 ms).
3. **Cadence de test > 2 × pacing du writer.** Le writer `lighting.Writer` (`MinInterval` 100 ms) dérive l'état **au moment de l'application**. Si la phase d'un test est proche de 100 ms, deux applications consécutives tombent dans la même phase (aliasing) et le test échoue sous charge. Règles : phase ≥ 2 × 100 ms (exception existante : `TestC2b_ScoreFlash_ClampsAboveSixPoints`, phases 167/333 ms, qui n'asserte que l'heure de fin, indépendante du writer) ; lire les appels par indice (3e et 4e applications) plutôt que de comparer deux `Last()` successifs, qui peuvent voir une application supplémentaire entre les deux lectures. Sonder `Last()` jusqu'à une couleur donnée reste correct. Cas réel : `TestDevLightingFlash_BlinksAndReturnsToSelector` (blink 250/250 ms) et `TestC2b_ScoreFlash_TwoCycles` (phase or 250 ms).
4. **Critère d'isolation pour `t.Parallel()`.** Un test ne passe en parallèle que si, ni lui ni ses appelants, ne touchent un état global :
   - pas de logger global (`SetGlobalLogger`) ;
   - pas d'écriture du singleton de configuration (`config.SetInstance`, écriture d'un champ de `app.config`) — `newTestApp` ne fait que lire `config.Get()` ;
   - pas de variable de paquet partagée (`LaunchLock*`) ni de hook `setTestInject*` non protégé ;
   - pas de `t.Setenv`, `os.Setenv`, `os.Chdir`, `http.Default*`, `log.Set*` ;
   - pas de fenêtre de temps serrée (flash, chrono, watchdog, expiration de verrou) : le jitter d'un test voisin la rendrait instable.
   La propriété se propage aux appelants. Après audit, 27 tests en `t.Parallel()` au total dans `cmd/server` : 11 existants, 1 ajouté en Phase A, 15 ajoutés par l'Option P (311 tests jugés sûrs sur 736 au moment de l'audit). Le reste reste séquentiel ; le lever suppose un logger et une configuration **par `App`** (issue #298).

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

### Ouverture automatique du navigateur (étendu v6.2.0.10 — #164)

Au démarrage, si `auto_open_browsers: true` (défaut, `config.json` / `AutoOpenBrowsers`), le
serveur ouvre automatiquement des onglets navigateur vers l'URL primaire détectée, dans cet ordre
fixe, avec un délai de 500 ms entre chaque ouverture :

| # | Page | Libellé log |
|---|------|-------------|
| 1 | `/admin` | régie |
| 2 | `/anim` | animateur |
| 3 | `/tv` | affichage TV |
| 4 | `/` | accueil joueurs |
| 5 (si `debug: true`) | `/logs` | logs (debug) |

Soit 4 onglets en usage normal, 5 en mode debug (~1,5 s / ~2 s de séquence d'ouverture).
`auto_open_browsers: false` désactive entièrement cette ouverture (aucun onglet).

Implémentation : `startupPages(debug bool) []startupPage` (`cmd/server/main.go`) — fonction pure
sans effet de bord, testée indépendamment (`startup_pages_test.go`) ; `displayAndOpenURLs` itère le
résultat et reste seule responsable des effets de bord (`openBrowser`, logs). Avant #164, `/admin`
n'était pas dans la liste et le libellé de `/anim` affichait à tort `(admin)` — reliquat de
l'ancien alias `/anim` → `/admin` supprimé par #155.

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
| Questions | 10 | QCM (avec indices), MEMORY, SPEEDY |
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

## CI/CD et Release (depuis v2.54.0)

Pipeline GitHub Actions (`.github/workflows/release.yml`) déclenché sur push `v*`.

**Jobs** (exécution ordonnée) :
1. **Testing** (~10-15 min) — tests unitaires (`go test -race`), go vet, govulncheck
   - Vérifie que la base de code est saine avant release
   - Bloque la release en cas d'échec
2. **Checking** (~10s) — vérification de cohérence entre tag et fichiers de version
   - Vérifie que `config.json`, `package.json`, `platformio.ini` et `version.txt` correspondent au tag
   - Échoue avec rapport détaillé si une incohérence est trouvée
   - Tolérance : `version.txt` tolère un suffixe de build `.N` (e.g., `11.1.0.14`)
3. **Compiling** (3 jobs parallèles, ~1-2 min, tous dépendent de testing + checking) :
   - **Firmware** (prérequis) : PlatformIO ESP32-C3 → `buzzclick-vX.Y.Z-merged.bin` (bootloader + app merged, supporte USB + OTA)
   - **Windows amd64** : Go + React + firmware embarqué → `buzzcontrol-vX.Y.Z-windows-amd64.exe`
   - **Linux ARM64** : Go + React + firmware embarqué → `buzzcontrol-vX.Y.Z-linux-arm64`
4. **Releasing** (~30s) — release GitHub avec 4 artefacts + SHA256SUMS + notes depuis CHANGELOG.md
   - Télécharge les 2 serveurs + firmware
   - Génère checksums SHA256 pour vérification intégrité
   - Publie la release GitHub

**Frontend web (#270)** : les workflows CI (`ci-branch.yml`, `release.yml`) utilisent **Node 22** (`NODE_VERSION: '22'`, `setup-node` résout la dernière 22.x). Le build web repose sur vite 8 (bundler rolldown, CSS minifié par lightningcss). `scripts/ci.sh` étape 8 exécute `govulncheck ./...` puis `npm audit --audit-level=high` (toutes dépendances) dans `server-go/web`.

**Étapes de `scripts/ci.sh` (#273, #294)** : `[1/8]` firmware (`scripts/build-firmware.sh`, sauté si `SKIP_FIRMWARE=1`) ; `[2/8]` npm ci + build web ; `[3/8]` tests vitest (`npm test` = `vitest run`, 270 fichiers / 4209 tests, 110 à 125 s mesurés en local) ; `[4/8]` embed de `dist` ; `[5/8]` build Go ; `[6/8]` `go test -race` ; `[7/8]` `go vet` ; `[8/8]` govulncheck puis audit npm. L'étape vitest vient après le build web et avant l'embed : elle teste un `dist/` frais. Les tests web, jusqu'ici jamais exécutés en CI, bloquent désormais `ci-branch.yml` et `release.yml` lorsqu'ils échouent, ces deux workflows n'appelant que ce script. Les tests `fonts-embed` « dist/ (build) » s'exécutent non sautés, car `dist/` est produit à l'étape `[2/8]`.

**Chaîne d'outils CI (#276)** : actions GitHub sur runtime Node 24 (`checkout@v7`, `setup-go@v7`, `setup-node@v7`, `setup-python@v7`, `upload-artifact@v7`, `download-artifact@v8`, `softprops/action-gh-release@v3`). La version de Go vient de `server-go/go.mod` (`go-version-file`, plus de `GO_VERSION` dans les workflows) : `setup-go` v6+ force `GOTOOLCHAIN=local`, la directive `go` de `go.mod` est donc la seule source de vérité, en CI comme en local. Cache Go indexé sur `server-go/go.sum`. Runner épinglé sur `ubuntu-24.04` (pas `ubuntu-latest`) : tout changement d'image se fait volontairement, après un run `ci-branch.yml` vert sur la nouvelle image.

**Gouvernance Go** : la CI ne prend pas automatiquement le dernier patch 1.26.x. Un correctif de la stdlib signalé par `govulncheck` impose un bump de la directive `go` dans `server-go/go.mod`, relu et commité comme tout changement de dépendance.

**Versioning unifié** : tous les fichiers de version (`config.json`, `package.json`, `platformio.ini`,
`version.txt`) doivent être bumpes manuellement **avant tagging** et correspondent exactement au
tag de release.

**Firmware embarqué** : Le binary firmware compilé (`buzzclick-vX.Y.Z-merged.bin`) est automatiquement copié dans `server-go/assets/firmware/` pour les mises à jour OTA (`/ws` endpoint firmware push). Le serveur détecte automatiquement le format merged et en extrait la portion app pour OTA.

> `versioninfo.json` : CI régénère automatiquement pour Windows PE metadata. Le `.syso` généré **ne doit pas** être commité (`.gitignore`). Mise à jour manuelle uniquement pour builds locaux (`build.ps1`).

> `version.txt` : fichier versionné (suivi dans Git) qui porte le numéro de version du firmware embarqué.
> La CI le vérifie mais ne le modifie pas. Tolère un suffixe de build `.N` (e.g., `11.1.0.14`), mais le
> segment `X.Y.Z` doit correspondre au tag. Les builds locaux le lisent si présent, sinon utilisent la
> version de `config.json`.

## Cross-compilation

```bash
# Windows (development)
go build -o buzzcontrol.exe ./cmd/server

# Raspberry Pi (production)
GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server
```
