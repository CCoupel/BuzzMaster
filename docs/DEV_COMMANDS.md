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
