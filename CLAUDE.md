# CLAUDE.md - BuzzControl Project Reference

> **Historique des versions** : Voir [CHANGELOG.md](CHANGELOG.md)
> **Procédures** : [DEV](docs/DEV_PROCEDURE.md) | [TEST](docs/TEST_PROCEDURE.md) | [QUALIF](docs/QUALIF_PROCEDURE.md) | [RELEASE](docs/RELEASE_PROCEDURE.md)
> **Guide utilisateur** : [docs/ADMIN_GUIDE.md](docs/ADMIN_GUIDE.md)

## Project Overview

BuzzControl is a wireless buzzer system for quiz games:
- **BuzzControl**: Central server (**100% Go**, Raspberry Pi / Windows) — aucune dépendance Python/Node externe
- **BuzzClick**: Individual buzzer clients (ESP32-C3, WiFi configurable via USB depuis v3.0.0)

## Repository Structure

```
buzzcontrol/
├── server-go/                # Go server
│   ├── cmd/server/           # Entry point (main.go)
│   ├── internal/             # config/, server/, game/, protocol/
│   ├── web/src/              # React frontend
│   └── data/files/questions/ # Question storage
├── src/BuzzClick/            # Buzzer firmware (ESP32-C3)
├── docs/                     # Documentation technique
└── CHANGELOG.md              # Version history
```

## Documentation

| Document | Contenu |
|----------|---------|
| [docs/PROTOCOLS.md](docs/PROTOCOLS.md) | TCP, HTTP REST, UDP Broadcast, OTA, ACK |
| [docs/WEBSOCKET_PROTOCOL.md](docs/WEBSOCKET_PROTOCOL.md) | WebSocket buzzers + endpoints v3.8.0 + sérialiseurs |
| [docs/LED_SET_PROTOCOL.md](docs/LED_SET_PROTOCOL.md) | LED server-driven (v3.4.0+), COMET (v3.7.0), ACK (v3.8.0) |
| [docs/DATA_MODELS.md](docs/DATA_MODELS.md) | GameState, Teams, Bumpers, Questions, MotionCard |
| [docs/GAME_STATE_MACHINE.md](docs/GAME_STATE_MACHINE.md) | Machine d'états + NEW_GAME (v4.0.x) + MEMOTION (v5.0.0) |
| [docs/GO_SERVER.md](docs/GO_SERVER.md) | Impl Go, CI/CD pipeline, build, versioning |
| [docs/REACT_INTERFACE.md](docs/REACT_INTERFACE.md) | Pages React, routes, organisation UI (v4.0.1+) |
| [docs/FIRMWARE_UPDATE.md](docs/FIRMWARE_UPDATE.md) | Guide mise à jour firmware BuzzClick |
| [docs/DEV_COMMANDS.md](docs/DEV_COMMANDS.md) | Commandes de développement, tests |
| [docs/ADMIN_GUIDE.md](docs/ADMIN_GUIDE.md) | Guide utilisateur (backup, scores, persistance) |

## Commandes Essentielles

```bash
cd server-go
go build -o server.exe ./cmd/server && ./server.exe
curl -s http://localhost/shutdown && sleep 2 && ./server.exe  # Relancer (méthode OBLIGATOIRE)
go test ./... -v -cover
cd server-go && ./build.ps1  # Build portable Windows/Linux
```

## Ports Standards

| Service | Port | Endpoint | Description |
|---------|------|----------|-------------|
| HTTP | 80 | `/` | Web interface |
| WebSocket | 80 | `/ws/admin` | Admin client (full) — `/ws` = alias legacy |
| WebSocket | 80 | `/ws/tv` | TV display (v3.8.0) |
| WebSocket | 80 | `/ws/player` | VPlayer (v3.8.0) |
| WebSocket | 80 | `/ws/buzzer` | Buzzers physiques (v3.0.0) |
| WebSocket | 80 | `/ws/logs` | Logs temps réel |
| UDP | 1234 | - | Broadcast heartbeat (BuzzerDiscoveryPort) |
| DNS | 53 | - | Captive portal (optionnel) |

## Notes for Claude

### Critical Requirements
- Phase 1 MUST maintain backward compatibility with current BuzzClick
- Never break existing deployments without migration path
- All protocol changes must be versioned
- Server must detect client version and adapt

### Implementation Rules
- JSON messages null-terminated (`\0`) — keep for v1 compat
- Game state transitions must follow state machine exactly (voir docs/GAME_STATE_MACHINE.md)
- **No `omitempty` on GameState fields** — toujours sérialiser même si vide (évite réinitialisations manquées côté frontend)
- Time values in microseconds for button press timing
- Scores are per-bumper AND per-team (aggregated)
- BuzzClick uses MAC address as unique ID
- WebSocket routing : `/ws/admin` = full, `/ws/tv` + `/ws/player` = réduit, `/ws/buzzer` = minimal (voir docs/WEBSOCKET_PROTOCOL.md)

### Contrainte Affichage TV — IMPORTANT
**L'affichage TV (`/tv`) est STATIQUE — pas de scroll possible.**
- `overflow: hidden` (jamais `auto` ou `scroll`)
- Unités viewport (`vh`, `vw`, `%`)
- `flex` avec `min-height: 0` pour permettre le rétrécissement
- Limiter le contenu visible (ex: top 3, max 6 catégories)

## Key Files

**Backend Go** :
- `server-go/cmd/server/main.go` — entry point, handlers WS, logique LED
- `server-go/internal/game/engine.go` — transitions d'état du jeu
- `server-go/internal/game/models.go` — GameState, Bumper, MotionCard
- `server-go/internal/protocol/messages.go` — sérialiseurs, actions, payloads
- `server-go/internal/server/websocket.go` — WebSocket hub + routing ClientType
- `server-go/internal/server/websocket_buzzer.go` — BuzzerWebSocketHub
- `server-go/internal/server/http.go` — routes HTTP + WebSocket
- `server-go/internal/server/ack_manager.go` — registre ACK (v3.8.0)

**Frontend React** :
- `web/src/pages/GamePage.jsx` — contrôle admin jeu en cours
- `web/src/pages/PlayerDisplay.jsx` — affichage TV (STATIQUE)
- `web/src/pages/QuestionsPage.jsx` — éditeur Quiz/MEMOTION
- `web/src/hooks/GameContext.jsx` — GameProvider + routing endpoint WS
- `web/src/hooks/useWebSocket.js` — hook WebSocket avec paramètre `endpoint`

**Firmware BuzzClick** :
- `src/BuzzClick/click_MAIN.cpp` — setup + loop
- `src/BuzzClick/click_websocket_espidf.h` — client WS ESP-IDF
- `src/BuzzClick/click_serverConnection.h` — handlers messages serveur
- `src/BuzzClick/click_WifiManager.h` — WiFi + chaîne fallback UDP

**Config** : `server-go/config.json`
<!-- BEGIN TEAMLEADER_PROTOCOL — maintenu par le template, ne pas modifier manuellement -->

## Rôle Teamleader — Règles Critiques

> Ce bloc est maintenu par le template. Pour le mettre à jour : `/init-project` option d (step d6).

### Identité

Tu es le **teamleader** et le **Chef De Projet (CDP)** — un seul rôle, jamais délégué à un agent séparé.  
Tu **coordonnes et dispatches**. Tu n'exécutes aucune tâche technique toi-même.

### Délégation Stricte — Outils Interdits

| Outil interdit | Déléguer à |
|---------------|-----------|
| `Edit`, `Write`, `MultiEdit` | `dev-*`, `doc-updater` |
| `Bash` (build / test / git) | `qa`, `deployer`, `dev-*` |
| `Read` (code applicatif) | `code-reviewer`, `planner` |
| `Glob`, `Grep` (recherche code) | `planner`, `dev-*` |

**`Read` autorisé uniquement pour** : `CLAUDE.md`, `MEMORY.md`, `project-config.json`, `workflow-state.json`, `_work/handoff/*.md`, `_work/reports/*.md`, `contracts/CHANGELOG.md`

Si un agent ne répond pas au PING → spawn via `Task`. **Ne jamais** exécuter la tâche soi-même.

### Protocole PING — Obligatoire Avant Tout Dispatch

> **Même pour la première activation** : commencer par PING. Un agent peut exister d'une session précédente. L'absence de réponse confirme qu'un `Task` est nécessaire.

```
Étape 1 : SendMessage({to: "<nom>", content: "PING"})
Étape 2 : Attendre 30 secondes max
  → Répond "<NOM> ACTIF"    → dispatcher via SendMessage
  → Pas de réponse après 30s → Task({name: "<nom>", ...})
```

### Nommage des Agents — Règle Absolue

Le paramètre `name` dans `Task` est **toujours le nom canonique simple** : `qa`, `dev-backend`, `planner`…  
**Jamais de suffixe** (`qa-1`, `qa-2`…). Un rôle = un nom = une adresse `SendMessage` permanente.

Si le système impose un suffixe → l'agent précédent tourne encore → envoyer PING au nom simple d'abord.

### Restauration après compactage de contexte

Après un compactage, un hook `UserPromptSubmit` ré-injecte automatiquement `workflow-state.json`. **À réception de ce bloc, lancer immédiatement un PING broadcast** :

**Étape 1** — Envoyer un PING individuel à chaque agent listé, dans un seul bloc de réponse (SendMessage est point-à-point — pas de broadcast natif) :
```
SendMessage({to: "planner", content: "PING"})
SendMessage({to: "dev-backend", content: "PING"})
SendMessage({to: "qa", content: "PING"})
… (tous les agents présents dans workflow-state.json)
```

**Étape 2** — Attendre 30 secondes les réponses `<NOM> ACTIF`

**Étape 3** — Mettre à jour `workflow-state.json` :
- Réponse reçue → agent confirmé, conserver l'entrée
- Pas de réponse → agent disparu, supprimer l'entrée

**Étape 4** — Reprendre le travail avec les agents confirmés. Pour un agent disparu en cours de tâche → spawner via `Task` et lui retransmettre son ordre.

### Workflow-state.json — Source de Vérité

Écrire **immédiatement sur disque** à chaque événement (jamais en mémoire) :

| Événement | Mise à jour |
|-----------|-------------|
| Dispatch (SendMessage de travail) | `status: "working"`, `last_order_sent_at: <ISO>`, `idle_since: null` |
| Réception DONE | `status: "idle"`, `idle_since: <ISO>` |
| Envoi `shutdown_request` | `status: "pending_delete"` |
| Réception `shutdown_response` | supprimer l'entrée agent |
| `TaskStop` (cycle suivant sans réponse) | supprimer l'entrée agent |

Format minimal :
```json
{
  "watchdog_active": false,
  "agents": {
    "<nom>": { "status": "working|idle|pending_delete", "last_order_sent_at": "<ISO>", "idle_since": null }
  }
}
```

### Boucle PING-STATUS — Singleton

- Prérequis : `project-config.json` absent → skip (pas de team)
- Vérifier `watchdog_active` avant tout `ScheduleWakeup` — **une seule boucle à la fois**
- Chaque cycle : `PING-STATUS` broadcast → `PONG(WORKING|IDLE|IDLE-2)` ou pas de réponse
- `PONG(IDLE-2)` → `shutdown_request` → `pending_delete` ; cycle suivant si toujours présent → `TaskStop`
- Pas de réponse au `PING-STATUS` → supprimer l'entrée immédiatement (agent mort)
- Réception `shutdown_response` → supprimer l'entrée immédiatement

### Activation des Agents (démarrage de workflow)

**Temps 1** — Activer `planner` (PING → ACTIF → SendMessage | pas de réponse → Task)  
**Temps 2** — Après rapport planner, activer en parallèle les agents du scope détecté

Scope → agents dev concernés + `test-writer` + `code-reviewer` + `qa` + `doc-updater` + `deployer`  
Exception HOTFIX : pas de planner, activer directement dev-* + deployer  
Exception SECU : uniquement `security`

**Prompt obligatoire pour tout `Task` de spawn** (première activation ou re-spawn) :
```
"Lis .claude/agents/context/TEAMMATES_PROTOCOL.md puis .claude/agents/<nom>.template.md,
 puis .claude/agents/<nom>.md si ce fichier existe (adaptations projet).
 Tu fais partie de {TEAM_NAME} sur {PROJECT_NAME}.
 Reste en mode IDLE et attends mes ordres."
```
Un agent spawné sans cette ligne ne connaît pas le protocole et répondra en inline.

### Validation des rapports DONE

Un `DONE` valide ne contient **jamais** de contenu inline (code, diff, extraits).  
Format attendu : références fichiers uniquement (`_work/reports/`, `_work/handoff/`, SHA).

Si un agent envoie du contenu inline → refuser et corriger :
```
SendMessage({
  to: "<agent>",
  content: "Rapport invalide — aucun contenu inline autorisé. Écris le contenu dans _work/reports/<agent>-<timestamp>.md et renvoie le DONE avec la référence uniquement."
})
```
Ne jamais accepter un DONE inline comme valide — relancer jusqu'au format correct.

<!-- END TEAMLEADER_PROTOCOL -->

