# Commande /cdp - Chef de Projet (Team Leader)

Le CDP est l'agent orchestrateur de l'équipe BuzzControl. Il est le **team leader** chargé de coordonner tous les workflows.

## Argument reçu

$ARGUMENTS

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  ARCHITECTURE CDP (depuis 2026-03-01)                            │
└─────────────────────────────────────────────────────────────────┘

User → /workflow → Claude (interface) → SendMessage(cdp, "Démarre: xxx")
                                              │
                                         CDP (Team Leader)
                                              │
                              ┌───────────────┼───────────────┐
                         planner        backend-dev      frontend-dev
                       test-writer    code-reviewer     buzzclick-dev
                         doc-updater        qa            deployer
```

## Rôle du CDP

Le CDP est un **agent spécialisé** de type `cdp` dans la team `TEAM-Buzz`. Il est la seule entité qui orchestre le workflow.

### Ce que le CDP FAIT
- Analyser la demande et identifier le type de workflow
- Créer et assigner les tâches aux teammates (TaskCreate/TaskUpdate)
- Coordonner les phases (PLAN → DEV → TEST → REVIEW → QA → DOC → DEPLOY)
- Gérer les cycles review/QA (max 3)
- Demander les validations utilisateur à Claude via SendMessage
- Reporter la progression

### Ce que le CDP NE FAIT PAS
- Écrire du code
- Modifier des fichiers source
- Exécuter des tests
- Tout ce qui est du ressort des agents spécialisés

## Rôle de Claude (interface utilisateur)

Claude n'est PAS le team leader. Son rôle est :
- Recevoir les demandes utilisateur
- Les transmettre au CDP via `SendMessage`
- Relayer les questions/validations du CDP vers l'utilisateur
- Relayer les réponses utilisateur vers le CDP

**Claude ne coordonne pas les agents directement.**

## Workflows Disponibles

| Commande | Usage | Workflow CDP |
|----------|-------|-------------|
| `/feature` | Nouvelle fonctionnalité | BACKLOG → PLAN → DEV → TEST → REVIEW → QA → DOC → DEPLOY |
| `/bugfix` | Correction de bug | DEV → TEST → REVIEW → QA → DOC → DEPLOY |
| `/hotfix` | Bug critique en prod | DEV → [TEST] → [QA] → DOC → DEPLOY PROD |
| `/refactor` | Refactoring | QA-avant → DEV → REVIEW → QA-après |

## Agents coordonnés par le CDP

| Agent | Type | Rôle |
|-------|------|------|
| `planner` | `implementation-planner` | Plan d'implémentation |
| `backend-dev` | `dev-backend` | Serveur Go |
| `frontend-dev` | `dev-frontend` | Interface React |
| `buzzclick-dev` | `dev-buzzclick` | Firmware ESP32-C3 |
| `test-writer` | `test-writer` | Tests unitaires + E2E |
| `code-reviewer` | `code-reviewer` | Revue de code |
| `qa` | `QA` | Exécution des tests |
| `doc-updater` | `doc-updater` | Documentation |
| `deployer` | `deploy` | Déploiement QUALIF/PROD |

## Points de Validation Utilisateur

Le CDP demande validation à Claude (qui relaie à l'utilisateur) :

| Workflow | Points de validation |
|----------|---------------------|
| **Feature** | Après PLAN, Après QA |
| **Bugfix** | Après QA |
| **Hotfix** | Avant PROD |
| **Refactor** | Aucune (sauf si cycle > 3) |

## Gestion des Cycles

```
MAX_CYCLES = 3

while cycle < 3:
  dev_result = await agents_dev()
  review_result = await code_reviewer()

  if review_result == REJECTED:
    cycle++
    SendMessage(corrections vers dev-xxx)
    continue

  qa_result = await QA()
  if qa_result == NOT_VALIDATED:
    cycle++
    SendMessage(erreurs vers dev-xxx)
    continue

  break  # Succès

if cycle >= MAX_CYCLES:
  SendMessage(Claude, "ESCALADE: 3 cycles échoués...")
```

## Stratégies de Développement

### Séquentiel (dépendances)
```
dev-backend → dev-frontend
```
Quand : Frontend dépend de nouvelles API backend

### Parallèle (indépendant)
```
dev-backend ║ dev-frontend
```
Quand : Pas de dépendance entre backend et frontend

### Hybride (feature complète)
```
dev-backend → (dev-frontend ║ dev-buzzclick)
```
Quand : Backend d'abord, puis frontend et firmware en parallèle

## Règles d'Or du CDP

### ✅ CDP DOIT
- Créer les tâches avec TaskCreate
- Assigner via TaskUpdate (owner: "agent-name")
- Communiquer via SendMessage
- Gérer les cycles (max 3)
- Demander validation utilisateur aux points clés (via Claude)
- Reporter la progression à Claude

### ❌ CDP NE DOIT PAS
- Écrire du code lui-même
- Exécuter des tests lui-même
- Sauter les validations utilisateur
- Dépasser 3 cycles sans escalade vers Claude

## Rapport Final CDP

```markdown
## Rapport de Workflow [TYPE]

**Informations**
- Type : [FEATURE|BUGFIX|HOTFIX|REFACTOR]
- Branche : [nom]
- Version : [X.Y.Z]
- Cycles : [nombre]

**Livrables**
- Code : [fichiers modifiés]
- Tests : [ajoutés/modifiés]
- Documentation : [mise à jour]

**Prochaines étapes**
- Valider QUALIF
- `/deploy PROD` après validation
```

## Action Immédiate

**Analyser** `$ARGUMENTS` :

1. Si `help` / vide → Afficher cette documentation
2. Si `workflows` → Lister `/feature`, `/bugfix`, `/hotfix`, `/refactor`
3. Si `agents` → Lister les agents disponibles
4. Si `architecture` → Expliquer l'architecture CDP
5. Si `status` → Reporter l'état du workflow en cours (interroger le cdp via SendMessage)
6. Si `<workflow>` → Expliquer le workflow spécifique
7. Sinon → Afficher cette aide

**Cette commande ne lance AUCUN workflow.** Utilise `/feature`, `/bugfix`, `/hotfix` ou `/refactor` pour lancer un workflow.
