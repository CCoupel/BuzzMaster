# Contexte Projet BuzzControl

> **Ce fichier contient le contexte technique du projet BuzzControl.**
> Référencé par `context/COMMON.md` pour tous les agents.

---

## Vue d'Ensemble

**BuzzControl** est un système de buzzers sans fil pour jeux de quiz. Le système comprend :
- **BuzzControl** : Serveur central (Go sur Raspberry Pi / Windows)
- **BuzzClick** : Buzzers individuels (ESP32-C3)

---

## Stack Technique

| Composant | Technologie |
|-----------|-------------|
| Backend | Go 1.21+ |
| Frontend | React 18 (Vite) |
| Protocole temps réel | WebSocket |
| Protocole buzzers | TCP/UDP (port 1234) |
| Persistance | JSON files |
| Build | Go embed (frontend intégré) |

---

## Structure du Repository

```
buzzcontrol/
├── server-go/                # Serveur Go (Raspberry Pi / Windows)
│   ├── cmd/server/           # Point d'entrée (main.go)
│   ├── internal/
│   │   ├── config/           # Configuration
│   │   ├── server/           # HTTP, WebSocket, TCP, UDP, DNS
│   │   ├── game/             # Engine et modèles
│   │   └── protocol/         # Parsing des messages
│   ├── web/src/              # Frontend React
│   │   ├── pages/            # Pages (GamePage, PlayerDisplay, etc.)
│   │   └── components/       # Composants réutilisables
│   └── data/files/questions/ # Stockage questions
├── src/
│   └── BuzzClick/            # Firmware buzzer (ESP32-C3)
├── docs/                     # Documentation
├── backlog/                  # Feature backlog (specs détaillées, suivi via GitHub Issues)
├── contracts/                # Contrats API
├── CLAUDE.md                 # Architecture complète
└── CHANGELOG.md              # Historique versions
```

---

## Fichiers Clés par Domaine

### Backend Go

| Fichier | Rôle |
|---------|------|
| `cmd/server/main.go` | Point d'entrée, handlers HTTP/WS |
| `internal/game/engine.go` | Logique de jeu, machine d'état |
| `internal/game/models.go` | Modèles de données (Team, Bumper, Question) |
| `internal/protocol/messages.go` | Parsing protocole TCP |
| `internal/server/e2e_test.go` | Tests E2E |

### Frontend React

| Fichier | Rôle |
|---------|------|
| `web/src/pages/GamePage.jsx` | Page admin principale |
| `web/src/pages/PlayerDisplay.jsx` | Affichage TV (STATIQUE) |
| `web/src/pages/QuestionsPage.jsx` | Gestion questions |
| `web/src/components/TeamCard.jsx` | Carte équipe/joueurs |
| `web/src/hooks/useWebSocket.js` | Hook WebSocket |

### Configuration

| Fichier | Rôle |
|---------|------|
| `server-go/config.json` | Version, paramètres serveur |
| `CLAUDE.md` | Documentation architecture |
| `contracts/*.md` | Contrats API |

---

## Ports Standards

| Service | Port | Description |
|---------|------|-------------|
| HTTP | 80 | Interface web |
| TCP | 1234 | Protocole buzzers |
| UDP | 1234 | Broadcast discovery |
| DNS | 53 | Captive portal (optionnel) |

---

## Modes de Jeu

| Mode | Description |
|------|-------------|
| BUZZ | Course au buzzer classique |
| QCM | Questions à choix multiples (4 couleurs) |
| MEMORY | Jeu de mémoire avec paires |
| PODIUM | Classement final |
| PALMARES | Hall of fame |

---

## Contrainte Critique : Affichage TV

**L'affichage TV (`/tv`) est STATIQUE et ne permet PAS de scroll.**

```css
/* OBLIGATOIRE pour toutes les vues TV */
.tv-container {
    height: 100vh;
    width: 100vw;
    overflow: hidden;  /* JAMAIS auto ou scroll */
}
```

Règles :
- Dimensionner avec `vh`, `vw`, `%`
- Utiliser `flex` avec `min-height: 0`
- Limiter le contenu visible (top 3, max 6 items)

---

## Build Order (CRITIQUE)

**TOUJOURS rebuilder le frontend AVANT le backend Go (mode portable).**

```bash
# Correct
cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server

# Incorrect (modifications React ignorées)
go build -o server.exe ./cmd/server
```

Le Go build utilise `//go:embed` pour intégrer les fichiers web compilés.

---

## Commandes par Catégorie

### Build

```bash
# Build portable COMPLET (frontend + backend) - RECOMMANDÉ
cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server

# Build backend uniquement (si pas de modifs frontend)
cd server-go && go build -o server.exe ./cmd/server

# Build Raspberry Pi (Linux ARM64)
cd server-go && GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server

# Build production optimisé (Raspberry Pi)
cd server-go && GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o buzzcontrol ./cmd/server
```

### Tests

```bash
# Tests unitaires avec couverture
cd server-go && go test ./... -v -cover

# Tests d'un package spécifique
cd server-go && go test ./internal/game -v

# Tests E2E
cd server-go && go test ./internal/server -v -run TestE2E

# Couverture détaillée
cd server-go && go test ./internal/game -coverprofile=coverage.out
go tool cover -func=coverage.out
go tool cover -html=coverage.out  # Vue HTML
```

### Qualité de Code

```bash
# Linting Go
cd server-go && golangci-lint run ./...

# Vérification formatage
cd server-go && gofmt -l .

# Formatage automatique
cd server-go && gofmt -w .
```

### Serveur

```bash
# Démarrer le serveur
cd server-go && ./server.exe

# Arrêter le serveur (API)
curl -s http://localhost/shutdown

# Redémarrer le serveur
curl -s http://localhost/shutdown && sleep 2 && ./server.exe

# Vérifier la version
curl -s http://localhost/version
```

### Validation Post-Build (DEV)

```bash
# Script complet de validation après build
cd server-go
./server.exe &
SERVER_PID=$!
sleep 2

# Vérifier version
VERSION=$(curl -s http://localhost/version)
echo "Version: $VERSION"

# Vérifier que le serveur répond
curl -s http://localhost/health || echo "Health check failed"

# Arrêt propre
curl -s http://localhost/shutdown
wait $SERVER_PID 2>/dev/null
echo "✅ Validation OK"
```

### Git Workflow

```bash
# Créer une branche feature
git checkout main && git pull origin main
git checkout -b feature/<nom-feature>

# Push initial
git push -u origin feature/<nom-feature>

# Squash merge vers main (PROD uniquement)
git checkout main && git pull origin main
git merge --squash feature/<nom-feature>
git commit -m "feat(<scope>): <description> (v<version>)"
git push origin main

# Créer un tag
git tag -a v<version> -m "Release v<version>"
git push origin v<version>

# Vérifier si tag existe
git tag -l "v<version>"
git ls-remote --tags origin "refs/tags/v<version>"
```

### BuzzClick (Firmware ESP32)

```bash
# Build firmware
pio run -e buzzclick

# Build et upload via USB
pio run -e buzzclick -t upload

# Monitor série
pio device monitor -b 115200
```

---

## Documentation Complémentaire

| Document | Description |
|----------|-------------|
| `CLAUDE.md` | Architecture complète et conventions |
| `docs/DEV_PROCEDURE.md` | Procédure de développement |
| `docs/TEST_PROCEDURE.md` | Procédure de tests |
| `docs/QUALIF_PROCEDURE.md` | Procédure de qualification |
| `docs/ADMIN_GUIDE.md` | Guide utilisateur |
| `CHANGELOG.md` | Historique des versions |
