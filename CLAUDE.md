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
| [docs/LIGHT_EFFECTS.md](docs/LIGHT_EFFECTS.md) | Écrire un effet lumière (format pas à pas, variables de groupe, zones) — v11.7.0 |

## Commandes Essentielles

```bash
cd server-go
go build -o server.exe ./cmd/server && ./server.exe        # Windows (binaire server.exe)
go build -o server ./cmd/server && ./server                # Linux (binaire server)
curl -s http://localhost/shutdown && sleep 2 && ./server.exe  # Relancer (méthode OBLIGATOIRE, adapter le nom du binaire à l'OS)
go test ./... -v -cover
cd server-go && ./build.ps1  # Build portable Windows/Linux (script Windows, lancé par l'utilisateur)
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
- `server-go/internal/led/led.go` — envoi des LED_SET aux buzzers (règles par phase, COMET, ACK) ; interface `Host`
- `server-go/internal/ambiance/ambiance.go` — couche d'ambiance lumineuse (writer, scènes, override Flash/SCORE) ; interface `Host`
- `server-go/internal/effects/` — moteur d'effets lumière (schéma YAML, validation, rendu, catalogue, rechargement) ; lecteur dans `internal/ambiance/effects.go` ; guide `docs/LIGHT_EFFECTS.md`
- `server-go/internal/vplayer/vplayer.go` — payloads UPDATE individualisés VPlayer
- `server-go/internal/rounds/rounds.go` — logique des manches (ROUNDS_UPDATE)

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
| `Edit`, `Write`, `MultiEdit` (sauf `Write` d'un ordre dans `_work/tasks/*.md` et de l'état `.claude/workflow-state.json`, `.claude/workflow/*`) | `dev-*`, `doc-updater` |
| `Bash` (build / test / git) — sauf opérations HEAD du checkout principal (`checkout`, `switch`, `reset`, `stash`), qui sont au teamleader seul (`agents/context/COMMON.md` « Branche Courante ») | `qa`, `deployer`, `dev-*` |
| `Read` (code applicatif) | `code-reviewer`, `planner` |
| `Glob`, `Grep` (recherche code) | `planner`, `dev-*` |

**`Read` autorisé uniquement pour** : `CLAUDE.md`, `MEMORY.md`, `project-config.json`, `_work/tasks/*.md`, `_work/handoff/*.md`, `_work/reports/*.md`, `contracts/CHANGELOG.md`

**`Write` autorisé uniquement pour** : `_work/tasks/*.md` (ordres aux teammates, création seule — jamais `Edit`/`MultiEdit`) et l'état d'orchestration `.claude/workflow-state.json` / `.claude/workflow/plan-<version>.md` (reprise après interruption) — jamais un autre chemin

**Ne jamais** exécuter une tâche technique soi-même — spawner l'agent approprié.

### Dispatcher une tâche

Les agents `socle` sont spawnés au démarrage (`/start-session`) ; la team du milestone l'est au GATE 2 (`agents/context/MILESTONE_CADRAGE.md`). Tous attendent en IDLE.
**Pendant la session : uniquement `SendMessage` — jamais de spawn**, sauf : la team d'un milestone, spawnée à
l'application de son cadrage (GATE 2 — `agents/context/MILESTONE_CADRAGE.md` section 4) ; les agents `ponctuel` de la
table (`security`, `infra`, `marketing`…), spawnés à la demande puis arrêtés (`TaskStop`) ; un agent de l'effectif
attendu (socle + team du milestone) perdu ou absent — respawné avant de lui envoyer un ordre.

**Teammates perdus** (Claude Code annonce « tes teammates ont été perdus… relance les teammates nécessaires ») :
respawner **tout** l'effectif attendu — socle + `cadrage.team` — et pas seulement les agents des tâches en cours,
puis reprendre comme `/start-session` étape 4bis (`agents/context/MILESTONE_CADRAGE.md` 4.2).

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
Procédure, format et checklist : `.claude/agents/teamleader.template.md`, section « Questions à l'utilisateur ».

### Relayer l'avancement

Chaque jalon `[NOM] EN COURS — …` d'un teammate (ex. `QA EN COURS — lot 3/12 …`) est relayé à l'utilisateur en
une ligne, sans attendre le DONE. Un jalon n'est pas un DONE : ne pas enchaîner avant le DONE. Si un jalon signale des échecs, les nommer dans le relais.

### Labels d'issues — à chaque changement de phase

Mettre à jour les labels de **toutes** les issues suivies (`ISSUE_NUMS[]`) **avant** d'envoyer l'ordre suivant, **et dès l'événement** (retour de fin de tâche d'un teammate `DONE`/`FAILED`/`BLOQUE`, validation d'un GATE — `GITHUB.md` §9.12), avant de le relayer. L'état dit ce qui **reste à faire** (ex. `PRET` = plan validé, DEV pas lancé), pas ce qu'un teammate fait à cet instant :
`SendMessage` à `deployer` (`gh issue edit --add-label … --remove-label …`, `.claude/commands/context/GITHUB.template.md` §9).
Fermeture des issues sur CI verte, ou à la validation utilisateur si `project-config.json` → `issues.close_on` = `validation` (`GITHUB.md` §9.5). La clôture du milestone reste soumise à la validation utilisateur.

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

Règles portées par le template — il fait foi, rien n'est recopié ici :
- Moment des mises à jour : « Labels d'issues — à chaque changement de phase » ci-dessus.
- États, transitions et lots (`PLANNING` → `PRET` → `EN COURS` → `EN REVIEW` / `EN QA` → `DONE`, `LOT-N`) : `.claude/commands/context/CDP_WORKFLOWS.template.md` (table « Labels GitHub — Suivi de Phase ») et `.claude/commands/context/GITHUB.template.md` §9 (§9.12 pour le moment des transitions).
- Validation des lots (GATE LOT si `VALID-HUMAINE`, sinon auto-validation ; GATE 4 jamais sauté) : `GITHUB.template.md` §9.12 / §9.13.
- Fermeture des issues (CI verte → fermeture, `DONE` conservé ; clôture du milestone = GATE 4) : `GITHUB.template.md` §9.5.
- Commande : `gh issue edit <n> --add-label "<L>" --remove-label "<L1>"` (l'outil MCP `issue_write` n'est pas disponible).

## Règles projet du cycle de livraison

- **QUALIF relancée sans demander** : une modification demandée par l'utilisateur pendant une session de test, une fois passée review + QA, déclenche immédiatement un nouveau cycle BUILD + PUBLISH + DEPLOY QUALIF.
- **Procédures manuelles fournies avec la QUALIF** : une QA validée sans navigateur laisse le visuel à l'utilisateur ; le teamleader lui transmet les procédures `tests/procedures/` concernées avec le lien QUALIF, avant le GATE LOT / GATE 4.
- **CHANGELOG relu avant PROD** : avant tout PUBLISH PROD, `code-reviewer` relit factuellement l'entrée CHANGELOG produite par `doc-updater`, puis re-relit après corrections (7 erreurs trouvées en v11.2.1, dont une introduite par la relecture).
- **Docs relues à chaque lot** : toute documentation produite par `doc-updater` pour un lot (docs techniques, CHANGELOG partiel) est relue factuellement par `code-reviewer` avant le GATE LOT, puis re-relue après correction (v11.2.2 : 3 erreurs au LOT-4 dont une affirmation produit fausse, 1 au LOT-6).
- **`/end-session` explicite** en fin de chantier, en particulier après une PROD : `.claude/memory/MEMORY.md` n'est mis à jour qu'à cette étape, aucune autre commande ne la rattrape.

## Branche courante et worktrees

- La **branche courante du dépôt principal** est définie et changée **uniquement par le teamleader** (via l'utilisateur ou `/milestone`). Pendant un cycle, c'est `milestone/<version>`.
- Les agents **commitent sur la branche courante**, avec des fichiers explicites (`git add <fichiers>`, jamais `git add -A`), et ne la changent **jamais**. Interdits dans le dépôt principal : `checkout`, `switch`, `reset --hard`, `clean`, `rebase`, `merge`, `stash`, détachement de HEAD.
- Tout travail sur un autre état du code (red check, comparaison, reproduction) se fait dans un **`git worktree` jetable**, par **chemin absolu** (`git -C <worktree> …`, jamais `cd ../..`), supprimé ensuite. Avant chaque commande destructive, vérifier que le chemin n'est pas le dépôt principal.
- Un agent qui constate que la branche courante a changé s'arrête et prévient le teamleader (`BLOQUE`).
- Pendant une QA ou un BUILD exécuté dans le dépôt principal (`go test`, `scripts/ci.sh`), les devs ne modifient pas le dépôt principal : ils travaillent dans un **worktree jetable** sur une branche `tmp/*`, puis cherry-pick sur la branche courante après le verdict (leçon v11.2.3, correctif #278).
- Incident de référence : QA #253, 2026-10-11 (HEAD détaché quelques minutes, aucun commit perdu).

## Agents Disponibles

| Nom | Rôle | Fichier | Spawn |
|-----|------|---------|-------|
| `planner` | Plan d'implémentation + contrats API | `.claude/agents/implementation-planner.template.md` | socle |
| `dev-backend` | Backend (Go) | `.claude/agents/dev-backend.template.md` | milestone |
| `dev-frontend` | Frontend (React) | `.claude/agents/dev-frontend.template.md` | milestone |
| `dev-firmware` | Firmware BuzzClick (ESP32-C3) — `src/BuzzClick/**`, `src/Common/**` | `.claude/agents/dev-firmware.template.md` | milestone |
| `test-writer` | Scripts de tests + procédures QA | `.claude/agents/test-writer.template.md` | milestone |
| `code-reviewer` | Revue de code | `.claude/agents/code-reviewer.template.md` | milestone |
| `qa` | Exécution des tests et validation | `.claude/agents/qa.template.md` | milestone |
| `doc-updater` | Documentation | `.claude/agents/doc-updater.template.md` | milestone |
| `deployer` | Build + Publication + Déploiement QUALIF/PROD | `.claude/agents/deploy.template.md` | socle |
| `security` | Audit sécurité | `.claude/agents/security.template.md` | ponctuel |
| `infra` | Infrastructure (si configurée) | `.claude/agents/infra.template.md` | ponctuel |
| `marketing` | Communication de release (release notes, posts, site `gh-pages`) | `.claude/agents/marketing-release.template.md` + `.claude/agents/marketing-release.md` | ponctuel |

> **Fichier** pointe vers le `.template.md` — géré par sync, toujours présent. Un compagnon
> `.md` (sans suffixe) peut exister à côté pour des adaptations projet ; il est optionnel et
> n'est jamais référencé ici puisqu'il ne contient jamais la définition complète de l'agent.

> **Agents `dev-*`** : une ligne par agent de stack inscrit au registre des agents (`agents.registry[]` de
> `project-config.json`), avec son périmètre. `dev-firmware` est l'agent standard du template (source
> `templates/dev-firmware-esp32.md`) ; ses adaptations BuzzClick vivent dans le compagnon `.claude/agents/dev-firmware.md`
> (ex-`dev-buzzclick`, remplacé le 2026-10-16).

> **Spawn** (`agents/context/MILESTONE_CADRAGE.md` 4.1) : `socle` = au `/start-session`, toujours ; `milestone` = si
> l'agent est dans la team du milestone, validée au GATE 2 (la team d'un milestone en cours est respawnée au
> `/start-session`) ; `ponctuel` = à la demande, puis arrêté (ex. `dev-firmware` hors milestone, `/secu`).
> Valeur historique `permanent` = `socle`.

> Pour `deployer` : la procédure concrète de PUBLISH/DEPLOY (un fichier par tâche × environnement)
> vit dans `.claude/agents/environments/{publish,deploy}.<env>.md` — voir `agents/deploy.md`
> section "Fichiers d'Environnement".

---

## Éléments privés — dossier `PRIVATE/`

**Tout élément privé qui ne doit jamais être commité se range dans `PRIVATE/`** (racine du projet, gitignoré) :
accès aux environnements (kubeconfig, tokens Kubernetes et de comptes de service, clés SSH, JSON de compte
cloud, certificats `*.pem`/`*.key`, keystores), comptes et identifiants notés, exports et dumps de données
réelles, documents client ou contractuels, notes personnelles, configuration locale propre à un poste.

Rangement — un dossier par portée (le nom d'environnement est celui de `infrastructure.environments[].name`) :

```
PRIVATE/
├── gitconfig        # lié à la structure du projet (inclus par le conteneur) — reste à la racine
├── GLOBAL/          # données privées communes au projet (comptes, ACCES.md, dumps, documents client, notes)
└── <ENV>/           # données privées d'un environnement précis (ex. PROD/kubeconfig, QUALIF/deploy_key)
```

| Élément | Emplacement |
|---------|-------------|
| Élément privé lié à un environnement | `PRIVATE/<ENV>/` (ex. `PRIVATE/PROD/kubeconfig`, `PRIVATE/QUALIF/deploy_key`) |
| Élément privé commun au projet | `PRIVATE/GLOBAL/` (ex. `PRIVATE/GLOBAL/ACCES.md`, `PRIVATE/GLOBAL/dumps/`) |
| Variables lues par `/deploy` (globales, par environnement) | `.env`, `.claude/agents/environments/<env>.env` — emplacements fixes liés à la structure du projet, **pas** dans `PRIVATE/` ; une variable qui désigne un fichier d'accès pointe dans `PRIVATE/<ENV>/` (ex. `KUBECONFIG=PRIVATE/PROD/kubeconfig`) |
| Fichier temporaire de travail | `_work/tmp/` (purgé, pas un stockage durable) |

- **`PRIVATE/` est partagé** : un seul exemplaire, à la racine du checkout principal, utilisé par tous les
  teammates — jamais copié (copier un secret le multiplie). Depuis un worktree :
  `PRIVATE_DIR="$(git rev-parse --path-format=absolute --git-common-dir)/../PRIVATE"`, ou un lien symbolique
  `ln -s <checkout principal>/PRIVATE <worktree>/PRIVATE` (ignoré par la ligne `/PRIVATE` du `.gitignore`).
- **Qui range quoi** : `infra` tient la structure et le contenu attendu (crée `GLOBAL/` et un dossier par
  environnement, y range les accès qu'il établit) ; `deployer` lit seulement ; tout teammate qui produit une
  donnée privée la range lui-même dans `GLOBAL/` ou `<ENV>/`. Un nouveau sous-dossier hors de cette structure :
  demander au teamleader. Aucune valeur secrète n'est inventée par un agent — elle est fournie par l'utilisateur.
- Avant de créer un fichier contenant une donnée privée : le placer dans `PRIVATE/` ; le code et la config trackés
  ne référencent `PRIVATE/` que par chemin relatif, jamais par copie de son contenu.
- Élément privé trouvé tracké hors `PRIVATE/` : **BLOQUE** et signaler au teamleader (déplacement + `git rm --cached`
  décidés par l'utilisateur ; un secret déjà poussé reste dans l'historique → rotation à prévoir).
- Ne jamais retirer `PRIVATE/` du `.gitignore` ni forcer son ajout (`git add -f`).
- Le contenu attendu de `PRIVATE/` est établi par `/init-project` (Étape 10b) → `project-config.json` → `private`.

---

## Outil manquant — environnement de bootstrap

Un outil absent du conteneur (commande introuvable, version insuffisante) ne s'installe pas « à la main » en
silence : il est **décrit** puis **ajouté à l'environnement de bootstrap** pour que toute session future l'ait.

1. Le teammate qui le constate : `[NOM] BLOQUE outil manquant` au teamleader, avec la fiche — outil, usage dans
   la tâche, version minimale, paquets (`apt` / `dnf` / `apk`) ou commande d'installation idempotente,
   contournement éventuel.
2. Le teamleader transmet la fiche à `infra` (jamais d'installation par le teamleader lui-même).
3. `infra` l'ajoute à `.claude-launcher/env.yaml` (ajouts propres au projet — **jamais** `env-template.yaml`,
   réécrit par `/init-project`), l'installe dans le conteneur courant si possible (débloque la session sans
   redémarrage), vérifie si la CI / le Dockerfile en ont aussi besoin, et signale l'outil au teamleader s'il
   vaut pour toute la stack (à proposer au template).
