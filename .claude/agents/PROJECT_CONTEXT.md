# Contexte Projet BuzzMaster

> **Ce fichier contient le contexte technique du projet BuzzMaster.**
> Référencé par `COMMON.md` pour tous les agents.

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
├── backlog/                  # Feature backlog
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

## Commandes Essentielles

```bash
# Développement
cd server-go
go build -o server.exe ./cmd/server && ./server.exe

# Tests unitaires
go test ./... -v -cover

# Tests E2E
go test ./internal/server -v -run TestE2E

# Build portable (frontend + backend)
cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server

# Relancer le serveur
curl -s http://localhost/shutdown && sleep 2 && ./server.exe
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
