# CLAUDE.md - BuzzControl Project Reference

> **Historique des versions** : Voir [CHANGELOG.md](CHANGELOG.md) pour l'historique détaillé des fonctionnalités par version.
>
> **Procédures** : DEV → QUALIF → RELEASE (voir section [Procédures de Développement et Production](#procédures-de-développement-et-production))
> - [DEV_PROCEDURE.md](docs/DEV_PROCEDURE.md) | [TEST_PROCEDURE.md](docs/TEST_PROCEDURE.md) | [QUALIF_PROCEDURE.md](docs/QUALIF_PROCEDURE.md) | [RELEASE_PROCEDURE.md](docs/RELEASE_PROCEDURE.md)
>
> **Guide utilisateur** : Voir [docs/ADMIN_GUIDE.md](docs/ADMIN_GUIDE.md) pour la persistance, sauvegarde/restauration et gestion des scores.

## Project Overview

BuzzControl is a wireless buzzer system for quiz games. The system consists of:
- **BuzzControl**: Central server (Go on Raspberry Pi / Windows)
- **BuzzClick**: Individual buzzer clients (ESP32-C3, unchanged)

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
│   └── BuzzClick/            # Buzzer client firmware (ESP32-C3) - UNCHANGED
├── docs/                     # Documentation
├── backlog/                  # Feature backlog
├── CLAUDE.md                 # This file
└── CHANGELOG.md              # Version history
```

## Documentation Détaillée

La documentation est organisée en fichiers thématiques :

| Document | Description |
|----------|-------------|
| [docs/PROTOCOLS.md](docs/PROTOCOLS.md) | Protocoles de communication (TCP, WebSocket, HTTP REST API) |
| [docs/DATA_MODELS.md](docs/DATA_MODELS.md) | Modèles de données (Questions, Teams, Bumpers, QCM, Memory) |
| [docs/GO_SERVER.md](docs/GO_SERVER.md) | Implémentation du serveur Go, persistance, build portable |
| [docs/REACT_INTERFACE.md](docs/REACT_INTERFACE.md) | Interface React, composants UI, VPlayer |
| [docs/MIGRATION_FUTURE.md](docs/MIGRATION_FUTURE.md) | Améliorations futures, protocole v2, BuzzClick v2 |
| [docs/DEV_COMMANDS.md](docs/DEV_COMMANDS.md) | Commandes de développement, versioning, tests |
| [docs/GAME_STATE_MACHINE.md](docs/GAME_STATE_MACHINE.md) | Machine d'état du jeu |
| [docs/ADMIN_GUIDE.md](docs/ADMIN_GUIDE.md) | Guide utilisateur |

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

| Service | Port | Description |
|---------|------|-------------|
| HTTP | 80 | Web interface |
| TCP | 1234 | BuzzClick buzzer protocol |
| UDP | 1234 | Broadcast (same as TCP) |
| DNS | 53 | Captive portal (optional) |

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

### Contrainte Affichage TV - IMPORTANT
**L'affichage TV (`/tv`) est STATIQUE et ne permet PAS de scroll.**
Toutes les vues TV doivent tenir entièrement à l'écran sans défilement :
- Utiliser `overflow: hidden` (jamais `auto` ou `scroll`)
- Dimensionner avec des unités viewport (`vh`, `vw`, `%`)
- Utiliser `flex` avec `min-height: 0` pour permettre le rétrécissement
- Limiter le contenu visible (ex: top 3, max 6 catégories)

### Key Files
- **Backend** : `server-go/cmd/server/main.go`, `internal/game/engine.go`, `internal/game/models.go`
- **Frontend** : `web/src/pages/GamePage.jsx`, `web/src/pages/PlayerDisplay.jsx`, `web/src/components/TeamCard.jsx`
- **Config** : `server-go/config.json`
