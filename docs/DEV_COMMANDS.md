# Commandes et Procédures de Développement - BuzzControl

Ce document décrit les commandes courantes et les procédures de développement.

## Gestion des Versions

> **Règle de référence** : [docs/RELEASE_PROCEDURE.md § Règles de Versionnement](RELEASE_PROCEDURE.md#règles-de-versionnement).
> Ce qui suit n'en est qu'un rappel côté développement — en cas d'écart, c'est l'autre document
> qui fait foi.

### Format de version : X.Y.Z.a en dev, X.Y.Z en prod

| Segment | Signification | Quand incrémenter |
|---------|---------------|-------------------|
| **X** | Compatibilité des données | Rupture de format des données persistées |
| **Y** | Milestone / livraison | Nouveau milestone (**pair = milestone/dev, impair = release**) |
| **Z** | Bugfix | Cycle de correctifs ou hotfix ; remis à 0 au nouveau milestone |
| **a** | Itération dev | À chaque commit de cycle — **jamais publié** |

### Règles de versionnement (rappel)

1. **Itération de dev / relance pour test** → incrémenter **a** (pas de limite)
   - Exemple : `6.0.0.0` → `6.0.0.1` → `6.0.0.2`…

2. **Nouveau milestone** → `Y+1 ; Z=0 ; a=0` (depuis la version de prod courante → `Y` pair)
   - Exemple : prod `6.1.0` → dev `6.2.0.0`

3. **Bug remonté hors milestone / hotfix** → `Z+1 ; a=0` (retour sur la vague dev paire)
   - Exemple : `6.0.0.2` → `6.0.1.0`

4. **Promotion en production** → `Y+1`, `Z` conservé, `a` supprimé (→ `Y` impair)
   - Exemple : `6.0.1.1` → tag `v6.1.1`

> ⚠️ **Changement du 2026-08-07** : l'ancienne règle (`z` = compteur de test, remis à 0 à la
> validation) est abandonnée — ce rôle est désormais tenu par `a`, et `z` porte les correctifs.
> Les versions ≤ v6.1.1 suivent l'ancienne règle.

### Fichiers à mettre à jour

| Fichier | Champ |
|---------|-------|
| `server-go/config.json` | `"version": "X.Y.Z.a"` |
| `server-go/web/package.json` | `"version": "X.Y.Z.a"` |

> En production, la CI réécrit ces fichiers depuis le tag — cf. RELEASE_PROCEDURE.md étape 3.

**Note** : Depuis v2.35.0 (mode portable), une seule version est utilisée pour le bundle complet (serveur + web).

## Procédures de Développement et Production

Les procédures détaillées sont documentées dans des fichiers séparés.

### Cycle de vie

```
┌─────────────────┐     Validation     ┌─────────────────┐     Validation     ┌─────────────────┐
│  DEV + TEST     │ ─────────────────► │  QUALIF + TEST  │ ─────────────────► │  RELEASE        │
│  (Développement)│                    │  (Qualification)│                    │  (Production)   │
└─────────────────┘                    └─────────────────┘                    └─────────────────┘
```

**Important** : Le passage d'une phase à l'autre nécessite une validation explicite de l'utilisateur.

### Documents de procédure

| Phase | Document | Description |
|-------|----------|-------------|
| DEV | [DEV_PROCEDURE.md](DEV_PROCEDURE.md) | Workflow de développement |
| TEST | [TEST_PROCEDURE.md](TEST_PROCEDURE.md) | Tests unitaires et E2E |
| QUALIF | [QUALIF_PROCEDURE.md](QUALIF_PROCEDURE.md) | Qualification avant release |
| RELEASE | [RELEASE_PROCEDURE.md](RELEASE_PROCEDURE.md) | Mise en production |
| UTILISATEUR | [ADMIN_GUIDE.md](ADMIN_GUIDE.md) | Guide utilisateur |

## Commandes Rapides

### Développement Go

```bash
# Développement
cd server-go
go build -o server.exe ./cmd/server && ./server.exe

# Relancer le serveur (IMPORTANT: toujours utiliser cette méthode)
curl -s http://localhost/shutdown && sleep 2 && ./server.exe

# Tests unitaires
go test ./... -v -cover

# Build release (Windows + Linux ARM64)
./build-release.ps1
```

**IMPORTANT - Procédure de relance du serveur:**
1. Appeler l'API shutdown: `curl http://localhost/shutdown`
2. Attendre l'arrêt (2 secondes)
3. Relancer l'exécutable: `./server.exe`

Ne jamais utiliser `taskkill` ou `kill` pour arrêter le serveur.

### Cross-compilation

```bash
# Windows (development)
go build -o buzzcontrol.exe ./cmd/server

# Raspberry Pi (production)
GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server
```

### Firmware BuzzClick (ESP32-C3)

#### Installation PlatformIO

```bash
# Installer PlatformIO CLI
pip install --upgrade platformio

# Vérifier l'installation
pio --version
```

#### Build et Flash

```bash
# Build firmware BuzzClick uniquement
pio run -e buzzclick

# Build + Upload via USB
pio run -e buzzclick -t upload

# Build firmware BuzzControl (legacy ESP32-S3)
pio run -e buzzcontrol -t upload
```

#### Monitoring et Debug

```bash
# Monitor série (baudrate 921600)
pio device monitor -b 921600

# Lister les ports série
pio device list

# Build + Upload + Monitor en une commande
pio run -e buzzclick -t upload -t monitor
```

#### Flash manuel avec esptool

```bash
# Identifier le port
pio device list

# Flash le firmware (Windows)
esptool.py --chip esp32c3 --port COM3 write_flash 0x0 .pio/build/buzzclick/firmware.bin

# Flash le firmware (Linux/Mac)
esptool.py --chip esp32c3 --port /dev/ttyUSB0 write_flash 0x0 .pio/build/buzzclick/firmware.bin
```

#### Versioning du firmware

Depuis v2.54.0, le firmware suit le versioning du serveur.
La version est injectée automatiquement par la CI lors du build de release.

Pour injecter manuellement une version :
```bash
# Éditer platformio.ini
# Remplacer : -D VERSION='"1.209.3"'
# Par       : -D VERSION='"2.54.0"'

# Puis rebuild
pio run -e buzzclick
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

## Tests Go : bonnes pratiques et tests instables (#257, #233)

Issues de la stabilisation des tests de `cmd/server` (corrections `8b0e5ead`, `57fc653d`, `b8375d17`).

### Règles pour écrire des tests Go

- **Pas de `time.Sleep` pour attendre un événement.** Attendre par condition : lire les messages jusqu'à l'action attendue, avec un délai maximal (`readActionMatchingWithin`, `collectMessagesUntil`, 10 s).
- **Lire jusqu'à l'action attendue, pas la première trame.** Une trame antérieure (`HEARTBEAT`, `UPDATE`) peut arriver avant. Si une trame interdite arrive (ex. `PLAYER_CONNECTED` quand on attend un rejet), le test échoue immédiatement.
- **Jamais de fenêtre fixe d'observation** (`collectActions(conn, 500ms)`) pour prouver l'absence ou l'ordre d'un message. Attendre l'événement attendu par condition, puis lire jusqu'à un **témoin** diffusé après l'action : le témoin prouve que tout ce qui devait arriver est arrivé. Helpers réels (`cmd/server`) : `collectAfterSentinel264` (un témoin, plusieurs connexions lues ensemble), `collectUntilSentinel264`, `assertNoAction264`, `eventually264` (`sentinel_264_test.go`) ; `noAction246` (`ardoise_final_246_test.go`) pour l'absence d'action sur plusieurs connexions ; `readActionMatchingWithin` (`rafale_countdown_wire_test.go`) pour attendre une action précise ; `settleSound` (méthode de `env244`, `score_display_252_test.go`) pour attendre que le moteur de son ait joué son témoin. Toutes les connexions à lire doivent être passées ensemble à `collectAfterSentinel264`, sinon un témoin reste en file.
- **Deux pauses fixes légitimes, à conserver** : `seat_release_test.go` (franchir la seconde de l'identifiant horodaté) et la boucle d'observation d'absence de `new_game_243_test.go` (vérifier pendant 6 s qu'un minuteur ne ressuscite pas NEW_GAME : une absence dans la durée n'a pas de témoin). Toute autre pause fixe est à remplacer par une attente par événement.
- **Arrêter les minuteurs en fin de test** avec `t.Cleanup` (ex. `engine.Stop()`, annulation du chien de garde de l'adaptateur de son). Les `defer` du test s'exécutent d'abord ; ensuite, tous les `t.Cleanup` s'exécutent dans l'ordre inverse de leur enregistrement.
- **Ne pas modifier le singleton `config` dans des tests parallèles.** Utiliser une injection (ex. `soundEnabled` de l'adaptateur de son). Si un test doit appeler `config.SetInstance`, restaurer l'instance précédente par `t.Cleanup`.
- **Pas de `t.Parallel()` sur un test qui utilise le singleton `config`.** Les tests parallèles s'exécutent après la phase séquentielle du paquet.

### Durée et commandes

- `go test -race ./...` (défaut) : rapide, à lancer à chaque modification.
- `go test -race -count=20 -timeout 3h ./cmd/server` : environ 80 minutes pour `cmd/server` seul avec `-count=20`.
- La série `go test -race -count=20 -timeout 3h ./...` complète : environ 80 minutes aussi (paquets en parallèle).
- Une exécution unique de `go test -race ./...` : environ 4 à 5 minutes.
- Le `-timeout` par défaut (10 min) fait échouer les séries : utilisez `-timeout 3h`.

### Hypothèse non prouvée

Deux tests (`TestHandlePlayerConnect_NoRelease_StillRejected`, `TestVPlayerBroadcastIntegration_PrepareToRevealSequence`) ont été vus rouges
seulement dans la suite complète, sous charge machine. Ils ne sont pas reproduits en 20 exécutions de `cmd/server` seul. La cause « charge machine / ordre des messages »
reste une **hypothèse non prouvée** : les corrections (attente par condition) rendent les tests robustes, sans l'avoir démontré.

### Dette restante

22 fichiers de test utilisent encore `config.SetInstance`. Six n'ont aucun `t.Cleanup` : `internal/config/config_merge_test.go`, `internal/server/ai_generator_rafale_203_test.go`, `internal/server/ai_job_concurrency_test.go`, `internal/server/e2e_test.go`, `internal/server/http_lighting_config_test.go`, `internal/server/rafale_generate_endpoint_203_test.go`. Ils sont stables car séquentiels, et ne doivent pas devenir parallèles sans isolation préalable. Tout nouveau `SetInstance` doit restaurer l'instance précédente par `t.Cleanup`.

Deux fichiers écrivent aussi le singleton sans le restaurer, via l'assistant `tw219qSetSoundEnabled` (`cmd/server/question_sound_219_test.go`, appel de `config.SetInstance`) : `cmd/server/sound_gate_dispatch_219_test.go` (7 appels) et `cmd/server/sound_gate_cache_219_test.go` (1 appel). Préférer `tw219qSetSoundEnabledFor`, qui injecte l'état du son dans l'adaptateur du test.

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
