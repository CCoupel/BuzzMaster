# Commandes et Procédures de Développement - BuzzControl

Ce document décrit les commandes courantes et les procédures de développement.

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

### Fichier à mettre à jour

| Fichier | Champ |
|---------|-------|
| `server-go/config.json` | `"version": "x.y.z"` |

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

### ESP32 (legacy)

```bash
# Build and upload
pio run -e buzzcontrol -t upload

# Monitor serial
pio device monitor -b 921600
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
