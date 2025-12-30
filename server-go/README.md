# BuzzControl Server (Go)

Server Go pour le systeme de buzzers BuzzControl. Compatible avec les buzzers BuzzClick v1 (ESP32-C3).

## Prerequis

- Go 1.21+
- Make (optionnel)

## Installation

```bash
# Telecharger les dependances
go mod download

# Ou via Make
make deps
```

## Utilisation

### Developpement (Windows/Linux)

```bash
# Executer directement
go run ./cmd/server

# Ou via Make
make run
```

### Build

```bash
# Build pour la plateforme courante
make build

# Build pour Raspberry Pi (ARM64)
make build-pi

# Build pour Raspberry Pi Zero (ARM32)
make build-pi-zero

# Build pour toutes les plateformes
make build-all
```

## Configuration

Editer `config.json` :

```json
{
  "server": {
    "http_port": 80,
    "tcp_port": 1234,
    "websocket_path": "/ws"
  },
  "wifi": {
    "ssid": "buzzmaster",
    "password": "BuzzMaster"
  },
  "game": {
    "default_delay": 30
  },
  "storage": {
    "data_dir": "./data",
    "questions_dir": "./data/files/questions",
    "files_dir": "./data/files"
  },
  "version": "2.0.0"
}
```

## Architecture

```
server-go/
├── cmd/server/main.go      # Point d'entree
├── internal/
│   ├── config/             # Configuration
│   ├── server/             # Serveurs TCP, UDP, HTTP, WebSocket
│   ├── protocol/           # Messages et parsing
│   ├── game/               # Logique de jeu
│   └── storage/            # Gestion fichiers
├── config.json             # Configuration
└── data/                   # Fichiers web et donnees
```

## Protocole

Compatible avec BuzzClick v1 :
- **TCP** (port 1234) : Messages entrants des buzzers
- **UDP Broadcast** (port 1234) : Messages sortants vers buzzers
- **HTTP** (port 80) : Interface web et API REST
- **WebSocket** (/ws) : Communication temps reel avec clients web

## Deploiement Raspberry Pi

1. Build pour ARM64 :
```bash
make build-pi
```

2. Copier sur le Pi :
```bash
scp buzzcontrol-linux-arm64 pi@raspberrypi.local:~/buzzcontrol/
scp config.json pi@raspberrypi.local:~/buzzcontrol/
```

3. Configurer hostapd et dnsmasq (voir docs/MIGRATION_PLAN.md)

4. Creer le service systemd :
```bash
sudo systemctl enable buzzcontrol
sudo systemctl start buzzcontrol
```

## Compatibilite

- BuzzClick v1 (ESP32-C3) : ✅ Compatible
- Interface web existante : ✅ Compatible
- Backup/Restore TAR : ⚠️ En cours

## License

Projet prive
