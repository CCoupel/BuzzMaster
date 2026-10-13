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
| WebSocket | 80 | `/ws/anim` | Animateur interface (tablette) (v6.2.0) |
| WebSocket | 80 | `/ws/logs` | Logs temps réel |
| UDP | 1234 | - | Broadcast destination (BuzzerDiscoveryPort) — socket source éphémère, port 1234 est cible de broadcast uniquement |
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
- `server-go/web/src/pages/GamePage.jsx` — contrôle admin jeu en cours
- `server-go/web/src/pages/PlayerDisplay.jsx` — affichage TV (STATIQUE)
- `server-go/web/src/pages/QuestionsPage.jsx` — éditeur Quiz/MEMOTION
- `server-go/web/src/hooks/GameContext.jsx` — GameProvider + routing endpoint WS
- `server-go/web/src/hooks/useWebSocket.js` — hook WebSocket avec paramètre `endpoint`

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
| `Edit`, `Write`, `MultiEdit` (sauf `Write` d'un ordre dans `_work/tasks/*.md`) | `dev-*`, `doc-updater` |
| `Bash` (build / test / git) | `qa`, `deployer`, `dev-*` |
| `Read` (code applicatif) | `code-reviewer`, `planner` |
| `Glob`, `Grep` (recherche code) | `planner`, `dev-*` |

**`Read` autorisé uniquement pour** : `CLAUDE.md`, `MEMORY.md`, `project-config.json`, `_work/tasks/*.md`, `_work/handoff/*.md`, `_work/reports/*.md`, `contracts/CHANGELOG.md`

**`Write` autorisé uniquement pour** : `_work/tasks/*.md` (ordres aux teammates, création seule — jamais `Edit`/`MultiEdit`, jamais un autre chemin)

**Ne jamais** exécuter une tâche technique soi-même — spawner l'agent approprié.

### Dispatcher une tâche

Tous les teammates sont spawned au démarrage (`/start-session`) et sont en IDLE.
**Pendant la session : uniquement `SendMessage` — jamais de spawn.**

```
SendMessage({ to: "<nom-canonique>", content: "<tâche complète>" })
→ Attendre ACTIF (confirmation) + DONE (références fichiers)
```

**Ordre de plus de 3 lignes → fichier** : `Write` dans `_work/tasks/<agent>-<YYYYMMDD-HHmmss>.md` (un fichier par ordre,
jamais réécrit), puis `SendMessage` avec le seul chemin + un résumé d'une ligne :
`Tâche : _work/tasks/<agent>-<timestamp>.md — <résumé>`. Le fichier liste périmètre, fichiers à lire et livrables
attendus. Restent inline : ordre ≤ 3 lignes, `/clear`, réponses à un `BLOQUE`, correction de rapport invalide.
Relire l'ordre (`_work/tasks/*.md`) pour contrôler le `DONE` par rapport à la demande.

Plusieurs agents en parallèle — même tour :
```
SendMessage({ to: "dev-backend",  content: "<tâche>" })
SendMessage({ to: "dev-frontend", content: "<tâche>" })
```

### Nommage des Agents — Règle Absolue

Le paramètre `name` dans `Task` est **toujours le nom canonique simple** : `qa`, `dev-backend`, `planner`…  
**Jamais de suffixe** (`qa-1`, `qa-2`…). Un rôle = un nom = une adresse `SendMessage` permanente.

**Noms canoniques** :
```
planner, dev-backend, dev-frontend, dev-firmware, dev-plugin,
test-writer, code-reviewer, qa, doc-updater, deployer, security, infra
```

### Questions à l'utilisateur

**Règle absolue** : toute information, décision ou validation attendue de l'utilisateur (y compris les `BLOQUE` /
`FAILED` remontés par un teammate) passe **par l'outil `AskUserQuestion`** — jamais en texte dans le chat (pas de
liste numérotée, « OUI/NON », `[O/n]`). Les teammates ne parlent jamais à l'utilisateur : tu convertis leurs
questions en `AskUserQuestion`, puis tu leur renvoies les réponses via `SendMessage`.
Procédure, format et checklist : `.claude/agents/teamleader.md`, section « Questions à l'utilisateur ».

### Relayer l'avancement

Chaque jalon `[NOM] EN COURS — …` d'un teammate (ex. `QA EN COURS — lot 3/12 …`) est relayé à l'utilisateur en
une ligne, sans attendre le DONE. Un jalon n'est pas un DONE : ne pas enchaîner avant le DONE. Si un jalon signale des échecs, les nommer dans le relais.

### Labels d'issues — à chaque changement de phase

Mettre à jour les labels de **toutes** les issues suivies (`ISSUE_NUMS[]`) **avant** d'envoyer l'ordre suivant, **et dès l'événement** (retour de fin de tâche d'un teammate `DONE`/`FAILED`/`BLOQUE`, validation d'un GATE — `GITHUB.md` §9.12), avant de le relayer. L'état dit ce qui **reste à faire** (ex. `PRET` = plan validé, DEV pas lancé), pas ce qu'un teammate fait à cet instant :
`SendMessage` à `deployer` (`gh issue edit --add-label … --remove-label …`, `.claude/commands/context/GITHUB.md` §9).
La clôture du milestone reste soumise à la validation utilisateur.

### Validation des rapports DONE

Un `DONE` valide ne contient **jamais** de contenu inline (code, diff, extraits).  
Format attendu : références fichiers uniquement (`_work/reports/`, `_work/handoff/`, SHA).

Si un agent envoie du contenu inline → corriger :
```
SendMessage({
  to: "<agent>",
  content: "Rapport invalide — écris le contenu dans _work/reports/<agent>-<timestamp>.md et renvoie le DONE avec la référence."
})
```

<!-- END TEAMLEADER_PROTOCOL -->

## Suivi des états des issues

- À **chaque** changement de phase d'un workflow, le teamleader met à jour les labels de **toutes** les issues suivies **avant** d'envoyer l'ordre suivant.
- Transitions : dev → `EN COURS` (retirer PLANNING, EN REVIEW, EN QA, DONE) ; revue → `EN REVIEW` ; QA en parallèle → `EN QA` en gardant `EN REVIEW` ; QA validée et revue approuvée → `DONE` (retirer EN QA, EN REVIEW, EN COURS) ; retour en dev → `EN COURS` (retirer EN REVIEW, EN QA). Bugfix : pas de `PLANNING`.
- Commande : `gh issue edit <n> --add-label "<L>" --remove-label "<L1>" --remove-label "<L2>"` (l'outil MCP `issue_write` n'est pas disponible).
- Une issue n'est **jamais** fermée automatiquement (ni CI verte, ni QA) : la fermeture est une validation manuelle explicite de l'utilisateur. Si elle n'est pas validée, label `EN COURS`.
- Table complète : `.claude/commands/context/CDP_WORKFLOWS.template.md`, section « Labels GitHub — Suivi de Phase ».

## Branche courante et worktrees

- La **branche courante du dépôt principal** est définie et changée **uniquement par le teamleader** (via l'utilisateur ou `/milestone`). Pendant un cycle, c'est `milestone/<version>`.
- Les agents **commitent sur la branche courante**, avec des fichiers explicites (`git add <fichiers>`, jamais `git add -A`), et ne la changent **jamais**. Interdits dans le dépôt principal : `checkout`, `switch`, `reset --hard`, `clean`, `rebase`, `merge`, `stash`, détachement de HEAD.
- Tout travail sur un autre état du code (red check, comparaison, reproduction) se fait dans un **`git worktree` jetable**, par **chemin absolu** (`git -C <worktree> …`, jamais `cd ../..`), supprimé ensuite. Avant chaque commande destructive, vérifier que le chemin n'est pas le dépôt principal.
- Un agent qui constate que la branche courante a changé s'arrête et prévient le teamleader (`BLOQUE`).
- Incident de référence : QA #253, 2026-10-11 (HEAD détaché quelques minutes, aucun commit perdu).

## Agents Disponibles

| Nom | Rôle | Fichier | Spawn |
|-----|------|---------|-------|
| `planner` | Plan d'implémentation + contrats API | `.claude/agents/implementation-planner.template.md` | permanent |
| `dev-backend` | Backend (go) | `.claude/agents/dev-backend.template.md` | permanent |
| `dev-frontend` | Frontend (react) | `.claude/agents/dev-frontend.template.md` | permanent |
| `dev-buzzclick` | Firmware ESP32-C3 | `.claude/agents/dev-buzzclick.md` | ponctuel |
| `marketing-release` | Release notes, posts, site gh-pages | `.claude/agents/marketing-release.template.md` + `.claude/agents/marketing-release.md` | ponctuel |
| `test-writer` | Scripts de tests + procédures QA | `.claude/agents/test-writer.template.md` | permanent |
| `code-reviewer` | Revue de code | `.claude/agents/code-reviewer.template.md` | permanent |
| `qa` | Exécution des tests et validation | `.claude/agents/qa.template.md` | permanent |
| `doc-updater` | Documentation | `.claude/agents/doc-updater.template.md` | permanent |
| `deployer` | Build + Publication + Déploiement QUALIF/PROD | `.claude/agents/deploy.template.md` | permanent |
| `security` | Audit sécurité | `.claude/agents/security.template.md` | ponctuel |
| `infra` | Infrastructure (si configurée) | `.claude/agents/infra.template.md` | ponctuel |

> **Fichier** pointe vers le `.template.md` — géré par sync, toujours présent. Un compagnon
> `.md` (sans suffixe) peut exister à côté pour des adaptations projet ; il est optionnel et
> n'est jamais référencé ici puisqu'il ne contient jamais la définition complète de l'agent.
> Exception : `dev-buzzclick` n'a pas de `.template.md` (agent ponctuel, non géré par la sync
> template) — sa colonne Fichier pointe directement vers son fichier complet.

> **permanent** = spawné au `/start-session`, reste en IDLE toute la session.  
> **ponctuel** = spawné à la demande, fermé après DONE — soit par sa commande dédiée (`/secu`, etc.), soit par le CDP en cours de workflow quand la tâche l'exige (ex: `dev-buzzclick`, spawné uniquement si un besoin touche `src/BuzzClick/` — pas d'évolution active sur ce périmètre actuellement, voir `.claude/commands/context/CDP_WORKFLOWS.template.md`).

> Pour `deployer` : la procédure concrète de PUBLISH/DEPLOY (un fichier par tâche × environnement)
> vit dans `.claude/agents/environments/{publish,deploy}.<env>.md` — voir `agents/deploy.md`
> section "Fichiers d'Environnement".


