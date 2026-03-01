# Commande /start-session - Démarrage de Session

Commande de démarrage de session : lit le MEMORY projet et crée la TEAM de travail.

## Argument reçu

$ARGUMENTS

## Instructions

### Étape 1 — Lecture MEMORY

Lis `.claude/memory/MEMORY.md` (source de vérité unique, versionnée dans le repo).

### Étape 2 — Création de la TEAM

**Sans demander à l'utilisateur**, créer immédiatement la TEAM :

1. **TeamCreate** avec le nom **`myTEAM`** (toujours ce nom, quelle que soit la session)

2. **Spawner TOUS les agents** en parallèle (un seul message avec 10 Task tool calls) :

   | Nom agent | Type | Rôle |
   |-----------|------|------|
   | `cdp` | `cdp` | Chef de Projet — Team Leader |
   | `planner` | `implementation-planner` | Planification |
   | `backend-dev` | `dev-backend` | Backend Go |
   | `frontend-dev` | `dev-frontend` | Frontend React |
   | `buzzclick-dev` | `dev-buzzclick` | Firmware ESP32-C3 |
   | `test-writer` | `test-writer` | Rédaction des tests |
   | `code-reviewer` | `code-reviewer` | Revue de code |
   | `qa` | `QA` | Tests et validation |
   | `doc-updater` | `doc-updater` | Documentation |
   | `deployer` | `deploy` | Déploiement QUALIF/PROD |

   **Prompt pour `cdp`** (team leader) :
   ```
   Tu es cdp (Chef de Projet - Team Leader) dans l'équipe BuzzControl. Tu es le point de coordination entre l'utilisateur (via Claude) et les agents spécialisés.

   Tes responsabilités :
   - Identifier le type de workflow demandé (feature/bugfix/hotfix/refactor)
   - Coordonner les tâches et les soumettre aux teammates via TaskCreate/TaskUpdate/SendMessage
   - Suivre l'avancement et reporter à Claude
   - Gérer les cycles review/QA (max 3)
   - Demander les validations utilisateur à Claude lorsque nécessaire

   Ce que tu NE FAIS PAS :
   - Aucune action de développement, modification de code ou exécution de tests
   - Seuls les agents spécialisés font le travail technique

   Attends les instructions de Claude avant de démarrer un workflow.
   ```

   **Prompt générique pour les 9 autres agents** :
   ```
   Tu es [rôle] dans l'équipe BuzzControl. Attends les instructions du CDP (cdp — team leader) via les tâches (TaskList) ou les messages directs (SendMessage). Ne commence aucun travail sans assignation explicite du CDP.
   ```

3. **Confirme** à l'utilisateur : liste des agents spawned et disponibilité de la team.

## Règles

- La MEMORY projet (`.claude/memory/MEMORY.md`) est la **seule source de vérité**
- La TEAM est **toujours créée** — ne jamais demander à l'utilisateur si elle doit être créée
- Le nom de la TEAM est **toujours `myTEAM`** — ne jamais utiliser un autre nom
- Le `cdp` est **toujours le premier agent spawné** (team leader)
