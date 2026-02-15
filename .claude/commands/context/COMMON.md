# COMMON.md - Patterns et Commandes Partagés

Ce fichier centralise les éléments répétés dans les définitions de commandes et agents Claude Code. Les agents et commandes doivent référencer ce fichier plutôt que de dupliquer ces informations.

> **Objectif** : Éliminer ~1500 lignes de duplication à travers 17 commandes et 13 agents.

---

## 1. Contexte Projet

**À utiliser dans tous les agents et commandes au lieu de répéter ces informations.**

```yaml
Projet: BuzzControl
Répertoire: /home/user/BuzzMaster
Repository: https://github.com/CCoupel/BuzzMaster

Structure:
  Serveur Go: server-go/
  Frontend React: server-go/web/src/
  Config version: server-go/config.json
  Package frontend: server-go/web/package.json
  Backlog: backlog/*.md
  Documentation: docs/
  Tests: server-go/internal/game/*_test.go

Ports:
  HTTP: 80
  TCP/UDP: 1234
  DNS: 53 (optionnel)

Branches:
  Production: main
  Features: feature/<nom-court>
  Bugfixes: fix/<nom-court>
```

---

## 2. Commandes de Build

### 2.1 Build Complet (Frontend + Backend)

> **RÈGLE CRITIQUE** : Le frontend DOIT être rebuild AVANT le Go build car les fichiers web sont embarqués dans le binaire (mode portable).

```bash
# === BUILD COMPLET ===
# Étape 1: Frontend (OBLIGATOIRE - toujours en premier)
cd server-go/web && npm run build

# Étape 2: Backend Go (embarque le frontend)
cd .. && go build -o server.exe ./cmd/server

# === ONE-LINER ===
cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server
```

### 2.2 Build Cross-Platform

```bash
# Windows AMD64 (développement local)
go build -o server.exe ./cmd/server

# Linux ARM64 (Raspberry Pi 4)
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o buzzcontrol ./cmd/server

# Linux ARM6 (Raspberry Pi Zero)
GOOS=linux GOARCH=arm GOARM=6 go build -ldflags="-s -w" -o buzzcontrol-arm6 ./cmd/server
```

### 2.3 Validation du Build

```bash
# Vérifier la taille (doit être > 5MB pour le portable)
ls -lh server.exe
# Attendu: ~25-30MB (inclut frontend embarqué)
```

---

## 3. Contrôle du Serveur

### 3.1 Arrêt Gracieux

> **RÈGLE** : Toujours utiliser l'API `/shutdown`, jamais de kill forcé.

```bash
# Arrêt via API (méthode standard)
curl -s http://localhost/shutdown

# Attendre l'arrêt complet
sleep 2
```

### 3.2 Séquence Redémarrage Complète

```bash
# === REDÉMARRAGE STANDARD ===
curl -s http://localhost/shutdown
sleep 2
cd server-go && ./server.exe

# === AVEC REBUILD (après modifications) ===
curl -s http://localhost/shutdown
sleep 2
cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server && ./server.exe
```

### 3.3 Vérification Post-Démarrage

```bash
# Vérifier la version
curl -s http://localhost/version

# Vérifier le fonctionnement
curl -s http://localhost/listGame

# Test complet (version + fonctionnement)
curl -s http://localhost/version && echo "" && curl -s http://localhost/listGame | head -c 100
```

### 3.4 Gestion Version Mismatch

Si `/version` ne correspond pas à `config.json` :

```bash
# 1. Arrêter
curl -s http://localhost/shutdown && sleep 2

# 2. Rebuild complet (frontend + backend)
cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server

# 3. Redémarrer et vérifier
./server.exe &
sleep 3
curl -s http://localhost/version
```

> **Maximum 2 tentatives** avant escalade à l'utilisateur.

---

## 4. Commandes de Test

### 4.1 Tests Unitaires

```bash
# Tous les tests avec couverture
cd server-go && go test ./... -v -cover

# Tests d'un package spécifique
go test ./internal/game/... -v

# Tests avec rapport
go test ./... -v -cover 2>&1 | tee test-report-$(date +%Y%m%d).txt
```

### 4.2 Rapport de Couverture HTML

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### 4.3 Validation Tests (CI/Pré-déploiement)

```bash
# Vérifier qu'aucun test n'échoue
go test ./... 2>&1 | grep -c "FAIL"
# Attendu: 0
```

---

## 5. Gestion des Versions

### 5.1 Fichiers de Version

| Fichier | Champ | Usage |
|---------|-------|-------|
| `server-go/config.json` | `"version"` | Source de vérité |
| `server-go/web/package.json` | `"version"` | Doit être synchronisé |

### 5.2 Règles de Versionnement

```
Format: X.Y.Z (SemVer)

X (major): Breaking changes, migrations nécessaires
Y (minor): Nouvelles features (incrémenté par PLAN)
Z (patch): Bugfixes, corrections (incrémenté par DEV)

Exemple:
- Feature nouvelle: 2.45.0 → 2.46.0
- Bugfix: 2.46.0 → 2.46.1
- Release finale: 2.46.3 → 2.46.0 (reset Z)
```

### 5.3 Incrémenter la Version

```bash
# Lire la version actuelle
cat server-go/config.json | grep '"version"'

# Après modification manuelle, vérifier la synchronisation
cat server-go/config.json | grep '"version"'
cat server-go/web/package.json | grep '"version"'
```

### 5.4 Synchronisation Version (PROD)

Pour une release, synchroniser les deux fichiers ET remettre Z à 0 :

```bash
# config.json: "version": "2.46.3" → "version": "2.46.0"
# package.json: "version": "2.46.3" → "version": "2.46.0"
```

---

## 6. Opérations Git

### 6.1 Création de Branche Feature

```bash
git checkout main
git pull origin main
git checkout -b feature/<nom-court>
# Incrémenter Y dans config.json
git add server-go/config.json
git commit -m "chore(version): Start vX.Y.0 - <feature name>"
git push -u origin feature/<nom-court>
```

### 6.2 Commit Atomique (Style)

```bash
# Format du message
<type>(<scope>): <description courte>

# Types valides
feat:     Nouvelle fonctionnalité
fix:      Correction de bug
docs:     Documentation uniquement
chore:    Maintenance, config
refactor: Refactoring sans changement fonctionnel
test:     Ajout/modification de tests
style:    Formatage, pas de changement de code

# Exemples
feat(memory): Add multi-team support
fix(qcm): Correct score calculation for hints
docs(changelog): Update for v2.46.0
chore(version): Bump to 2.46.1
```

### 6.3 Squash Merge (PROD)

```bash
git checkout main
git pull origin main
git merge --squash feature/<branch>
git commit -m "feat: <description> (v2.X.0)"
git push origin main
```

### 6.4 Tag et Release

```bash
# Créer le tag annoté
git tag -a v2.X.0 -m "Release v2.X.0 - <description>"
git push origin v2.X.0

# NE PAS supprimer la branche (gardée pour rollback)
```

---

## 7. Checklists Communes

### 7.1 Checklist Fin de Session DEV

```markdown
- [ ] Code compilé sans erreur (go build)
- [ ] Tests unitaires passés (go test ./... -v)
- [ ] Version incrémentée (Z pour bugfix)
- [ ] Commits atomiques avec messages clairs
- [ ] Pas de fichiers temporaires (*.bak, nul, server-output.txt)
- [ ] Push effectué
```

### 7.2 Checklist Pré-QUALIF

```markdown
- [ ] Build frontend réussi (npm run build)
- [ ] Build backend réussi (go build)
- [ ] Tests 100% passés (0 FAIL)
- [ ] Serveur redémarré et opérationnel
- [ ] Version /version correspond à config.json
- [ ] /listGame retourne un JSON valide
```

### 7.3 Checklist Pré-PROD

```markdown
- [ ] QUALIF validée
- [ ] Review code approuvée
- [ ] CHANGELOG.md mis à jour
- [ ] CLAUDE.md mis à jour (si nouvelles features)
- [ ] Version Z remise à 0
- [ ] package.json synchronisé
- [ ] Build Windows + ARM64 réussis
- [ ] Taille binaires > 5MB (mode portable)
```

### 7.4 Checklist Post-PROD

```markdown
- [ ] Squash merge vers main effectué
- [ ] Tag Git créé et pushé
- [ ] CI GitHub Actions passée
- [ ] Release GitHub créée avec binaires
- [ ] Exécutable release validé localement
- [ ] Branche feature conservée (rollback)
```

---

## 8. Nettoyage

### 8.1 Fichiers Temporaires à Supprimer

```bash
# Fichiers de développement
rm -f nul server-output.txt server-error.txt
rm -f test-report.txt test-summary.txt
rm -f *.bak web/src/pages/*.bak

# Fichiers de couverture
rm -f coverage.out coverage.html

# Build artifacts (si nécessaire)
rm -f server.exe buzzcontrol buzzcontrol-arm6
```

---

## 9. Patterns de Workflow

### 9.1 Workflow Feature

```
/plan → PLAN → DEV-BACKEND → DEV-FRONTEND → REVIEW → QA → DOC → DEPLOY(QUALIF) → DEPLOY(PROD)
        │        ↓              ↓
        │    (parallélisable si pas de dépendances)
        └── Incrémente Y
```

### 9.2 Workflow Bugfix

```
/bugfix → CDP → DEV → REVIEW → QA → DEPLOY(QUALIF)
                │
                └── Incrémente Z
```

### 9.3 Workflow Hotfix (Urgence)

```
/hotfix → DEV → QA → DEPLOY(PROD)
          │
          └── Incrémente Z, pas de QUALIF
```

---

## 10. Architecture d'Équipe - Team Leader Direct

### 10.1 Principe Fondamental

**Claude = Team Leader Direct** qui crée ET coordonne tous les agents.

```
Claude (Team Leader)
    │
    ├── TeamCreate("bugfix-xxx" / "feature-xxx" / "hotfix-xxx")
    ├── Task(dev-backend) avec prompt GÉNÉRIQUE
    ├── Task(dev-frontend) avec prompt GÉNÉRIQUE
    ├── Task(test-writer) avec prompt GÉNÉRIQUE
    ├── Task(code-reviewer) avec prompt GÉNÉRIQUE
    ├── Task(QA) avec prompt GÉNÉRIQUE
    ├── Task(doc-updater) avec prompt GÉNÉRIQUE
    └── Task(deploy) avec prompt GÉNÉRIQUE
    │
    └── Coordination via TaskCreate + TaskUpdate + SendMessage
```

### 10.2 ⚠️ RÈGLE CRITIQUE : Création des Agents

**OBLIGATOIRE** : Les agents doivent être créés avec des prompts **GÉNÉRIQUES** uniquement !

#### Pattern CORRECT ✅

```javascript
// 1. Créer l'équipe
TeamCreate({
  team_name: "bugfix-wifi-config",
  description: "Fix incomplete WiFi configuration",
  agent_type: "team-lead"
})

// 2. Créer les agents avec prompts GÉNÉRIQUES (sans tâche spécifique)
Task({
  subagent_type: "dev-backend",
  team_name: "bugfix-wifi-config",
  name: "backend-dev",
  description: "Backend Go developer",  // ✅ Description courte du rôle
  prompt: "Tu es l'agent backend Go du projet BuzzControl. Tu travailles dans l'équipe \"bugfix-wifi-config\". Attends les tâches qui te seront assignées par le team leader via la task list. Quand une tâche te sera assignée, consulte-la avec TaskGet puis implémente le code Go demandé. Tu es prêt à commencer quand le team leader t'assignera du travail."
})

Task({
  subagent_type: "test-writer",
  team_name: "bugfix-wifi-config",
  name: "test-writer",
  description: "Test writer",  // ✅ Description courte du rôle
  prompt: "Tu es l'agent test writer du projet BuzzControl. Tu travailles dans l'équipe \"bugfix-wifi-config\". Attends les tâches qui te seront assignées par le team leader via la task list. Quand une tâche te sera assignée, consulte-la avec TaskGet puis écris les tests demandés. Tu es prêt à commencer quand le team leader t'assignera du travail."
})

// 3. Créer les tâches dans la task list
TaskCreate({
  subject: "Implement WiFi config backend endpoints",
  description: "Créer les endpoints REST API pour la config WiFi des buzzers dans server-go/internal/server/http.go. Ajouter GET /api/buzzers pour lister les buzzers connectés. Ajouter GET /api/buzzer/:mac/wifi-status pour le statut WiFi.",
  activeForm: "Implementing backend WiFi config"
})

TaskCreate({
  subject: "Write regression test for WiFi config",
  description: "Écrire un test de non-régression dans server-go/internal/server/http_test.go pour vérifier que les endpoints WiFi config fonctionnent correctement.",
  activeForm: "Writing regression tests"
})

// 4. Assigner les tâches aux agents
TaskUpdate({ taskId: "1", owner: "backend-dev", status: "in_progress" })
TaskUpdate({ taskId: "2", owner: "test-writer" })

// 5. Coordination via SendMessage (corrections, questions)
SendMessage({
  recipient: "backend-dev",
  content: "Corrections demandées suite à la review: ajouter validation des paramètres..."
})
```

#### Pattern INTERDIT ❌

```javascript
// ❌ INTERDIT : Tâche spécifique dans le prompt de Task()
Task({
  subagent_type: "dev-backend",
  team_name: "bugfix-wifi-config",
  name: "backend-dev",
  description: "Implement backend WiFi config",  // ❌ Trop spécifique !
  prompt: "Tu dois implémenter les endpoints WiFi config dans http.go. Créer GET /api/buzzers et GET /api/buzzer/:mac/wifi-status..."  // ❌ Tâche spécifique !
})
```

### 10.3 Pourquoi c'est critique

| Raison | Explication |
|--------|-------------|
| **Réutilisabilité** | Un agent peut traiter plusieurs tâches sans respawn |
| **Coordination** | TaskCreate + TaskUpdate = contrôle total du workflow |
| **Séparation rôle/tâche** | Un agent = un RÔLE, pas une TÂCHE |
| **Corrections** | SendMessage pour itérations sans recréer l'agent |
| **Clarté** | Task list = vue d'ensemble du travail à faire |

### 10.4 Workflow Correct (Étapes)

```
1. TeamCreate({ team_name, description, agent_type: "team-lead" })
2. Task() × N agents avec prompts génériques (dev-backend, frontend, test-writer, QA, etc.)
3. TaskCreate() × M tâches avec descriptions détaillées
4. TaskUpdate({ taskId, owner, status: "in_progress" }) pour assigner
5. Attendre messages automatiques des agents
6. SendMessage() pour corrections/questions si nécessaire
7. TaskUpdate({ taskId, status: "completed" }) quand terminé
8. SendMessage({ type: "shutdown_request" }) à la fin
9. TeamDelete() pour nettoyer
```

### 10.5 Template Prompt Générique par Type d'Agent

#### dev-backend
```
Tu es l'agent backend Go du projet BuzzControl. Tu travailles dans l'équipe "{team_name}".
Attends les tâches qui te seront assignées par le team leader via la task list. Quand une tâche te sera assignée, consulte-la avec TaskGet puis implémente le code Go demandé.
Tu es prêt à commencer quand le team leader t'assignera du travail.
```

#### dev-frontend
```
Tu es l'agent frontend React du projet BuzzControl. Tu travailles dans l'équipe "{team_name}".
Attends les tâches qui te seront assignées par le team leader via la task list. Quand une tâche te sera assignée, consulte-la avec TaskGet puis implémente le code React demandé.
Tu es prêt à commencer quand le team leader t'assignera du travail.
```

#### dev-buzzclick
```
Tu es l'agent firmware ESP32-C3 du projet BuzzControl. Tu travailles dans l'équipe "{team_name}".
Attends les tâches qui te seront assignées par le team leader via la task list. Quand une tâche te sera assignée, consulte-la avec TaskGet puis implémente ou vérifie le code firmware demandé.
Tu es prêt à commencer quand le team leader t'assignera du travail.
```

#### test-writer / code-reviewer / QA / doc-updater / deploy
```
Tu es l'agent {role} du projet BuzzControl. Tu travailles dans l'équipe "{team_name}".
Attends les tâches qui te seront assignées par le team leader via la task list. Quand une tâche te sera assignée, consulte-la avec TaskGet puis {action} demandé(e).
Tu es prêt à commencer quand le team leader t'assignera du travail.
```

### 10.6 ⚠️ IMPORTANT : Mise à Jour de la Mémoire Claude

**Quand cette section est modifiée, Claude DOIT mettre à jour sa mémoire personnelle :**

```bash
# Emplacement mémoire Claude
~/.claude/projects/<projet>/memory/MEMORY.md
```

**Contenu à synchroniser dans MEMORY.md** :
- Section "Architecture d'Équipe - Claude comme Chef de Projet Direct"
- Pattern de création des agents avec prompts génériques
- Workflow correct (TeamCreate → Task → TaskCreate → TaskUpdate)
- Templates de prompts par type d'agent

**Commande pour Claude** : Quand COMMON.md section 10 change, exécuter :
```
Write(~/.claude/memory/MEMORY.md) avec mise à jour de la section architecture
```

---

## 11. Dispatch Automatique Backend/Frontend

### 10.1 Critères de Routage

| Critère | Agent |
|---------|-------|
| Fichiers `*.go`, `internal/`, `cmd/` | dev-backend |
| Fichiers `*.jsx`, `*.css`, `web/src/` | dev-frontend |
| Nouveaux endpoints HTTP | dev-backend → dev-frontend |
| Nouvelles actions WebSocket | dev-backend → dev-frontend |
| Nouveaux champs GameState | dev-backend → dev-frontend |
| Refactoring CSS isolé | dev-frontend seul |
| Tests Go uniquement | dev-backend seul |

### 10.2 Ordre d'Exécution

- **Séquentiel** (Backend → Frontend) : Si nouvelles APIs, WebSocket, ou GameState
- **Parallèle** : Si modifications isolées sans dépendances

---

## 11. Référence Rapide

### Commandes Essentielles

| Action | Commande |
|--------|----------|
| Build complet | `cd server-go/web && npm run build && cd .. && go build -o server.exe ./cmd/server` |
| Restart serveur | `curl -s http://localhost/shutdown && sleep 2 && ./server.exe` |
| Tests | `go test ./... -v -cover` |
| Version | `curl -s http://localhost/version` |
| Fonctionnement | `curl -s http://localhost/listGame` |

### Fichiers Clés

| Fichier | Rôle |
|---------|------|
| `server-go/config.json` | Version (source de vérité) |
| `server-go/internal/game/models.go` | Modèles de données |
| `server-go/internal/game/engine.go` | Logique de jeu |
| `server-go/web/src/pages/PlayerDisplay.jsx` | Affichage TV |
| `CHANGELOG.md` | Historique des versions |
| `CLAUDE.md` | Documentation projet |

---

## 12. Mots-Clés Réservés (Contrôle de Workflow)

Les commandes CDP (`/feature`, `/bugfix`, `/hotfix`, `/refactor`) reconnaissent des mots-clés spéciaux pour interroger ou reprendre un workflow.

### 12.1 Mots-Clés Disponibles

| Mot-clé | Description | Exemple |
|---------|-------------|---------|
| `help` | Affiche l'aide et les mots-clés disponibles | `/feature help` |
| `status` | Affiche l'état actuel du workflow | `/feature status` |
| `plan` | Affiche le plan sans exécuter | `/feature plan` |
| `resume <phase>` | Reprend à une phase spécifique | `/feature resume qa` |
| `skip <phase>` | Saute une phase | `/feature skip review` |
| `jumpto <tâche>` | Démarre à une tâche précise du plan | `/feature jumpto "Créer endpoint API"` |

### 12.2 Phases Valides pour resume/skip

```
init → plan → dev → review → qa → doc → deploy
```

### 12.3 Comportement par Mot-Clé

**`help`** :
```markdown
## /feature - Aide

**Description** : Workflow complet de développement d'une feature

**Usage** :
  /feature <description>           Lancer une nouvelle feature
  /feature help                    Afficher cette aide
  /feature status                  État du workflow en cours
  /feature plan                    Afficher le plan
  /feature resume <phase>          Reprendre à une phase
  /feature skip <phase>            Sauter une phase
  /feature jumpto <tâche>          Aller à une tâche précise

**Phases** : init → plan → dev → review → qa → doc → deploy

**Exemples** :
  /feature Ajouter mode Memory
  /feature resume dev
  /feature jumpto "Tests unitaires"
```

**`status`** :
```markdown
## État du Workflow

**Type** : FEATURE
**Phase actuelle** : DEV (3/7)
**Tâches** : 2/5 complétées
**Prochaine étape** : Implémenter frontend

[Reprendre] | [Voir plan] | [Abandonner]
```

**`plan`** :
```markdown
## Plan d'Implémentation

- [x] Phase 1 : Init (branche créée)
- [x] Phase 2 : Plan validé
- [ ] Phase 3 : DEV ← en cours
  - [x] Tâche 1 : Backend API
  - [ ] Tâche 2 : Frontend composant
  - [ ] Tâche 3 : Tests
- [ ] Phase 4 : REVIEW
- [ ] Phase 5 : QA
- [ ] Phase 6 : DOC
- [ ] Phase 7 : DEPLOY
```

**`resume <phase>`** :
- Vérifie que les phases précédentes sont complètes
- Si non, propose de compléter ou forcer
- Reprend l'exécution à la phase spécifiée

**`skip <phase>`** :
- Marque la phase comme "skippée"
- Continue à la phase suivante
- Note dans le rapport final

**`jumpto <tâche>`** :
- Recherche la tâche par nom (fuzzy match)
- Positionne le workflow à cette tâche
- Affiche contexte pour confirmation

### 12.4 Détection Automatique

Le premier mot de `$ARGUMENTS` est vérifié contre cette liste. Si match :
- Extraire le mot-clé et les paramètres
- Exécuter l'action correspondante
- Ne PAS lancer le workflow normal

```
$ARGUMENTS = "help"             → Action: afficher aide commande
$ARGUMENTS = "status"           → Action: afficher état
$ARGUMENTS = "resume dev"       → Action: reprendre à DEV
$ARGUMENTS = "jumpto API test"  → Action: chercher tâche "API test"
$ARGUMENTS = "Ajouter mode X"   → Action: workflow normal (pas de mot-clé)
```

---

## Usage

**Dans les commandes et agents**, au lieu de répéter le contexte projet :

```markdown
# Avant (répété 17+ fois)
**Contexte projet :**
- Répertoire : /home/user/BuzzMaster
- Serveur Go : server-go/
- Frontend React : server-go/web/src/
- Config version : server-go/config.json

# Après (référence unique)
**Contexte projet :** Voir `context/COMMON.md` section 1
**Build :** Voir `context/COMMON.md` section 2
**Tests :** Voir `context/COMMON.md` section 4
```
