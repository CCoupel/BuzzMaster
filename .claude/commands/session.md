# Commande /session - Démarrage de Session

Commande de démarrage de session : synchronise les MEMORY et crée la TEAM de travail.

## Argument reçu

$ARGUMENTS

## Instructions

Exécute les étapes suivantes dans l'ordre :

### Étape 1 — Sync MEMORY

Compare les deux fichiers MEMORY.md :
- **Projet** : `.claude/memory/MEMORY.md`
- **Privé** : `C:/Users/cyril/.claude/projects/C--Users-cyril-Documents-VScode-buzzcontrol/memory/MEMORY.md`

Utilise l'outil Read pour lire les deux fichiers en parallèle, puis compare leur contenu.

Affiche le résultat :
- Si identiques → `**[Sync MEMORY]** Les deux MEMORY.md sont identiques. ✓`
- Si différents → Affiche le diff et demande : "Quelle version fait foi — privée ou projet ?" puis résous le conflit en copiant la version canonique vers l'autre.

### Étape 2 — Création de la TEAM

**Sans demander à l'utilisateur**, créer immédiatement la TEAM :

1. **TeamCreate** avec le nom **`myTEAM`** (toujours ce nom, quelle que soit la session)

2. **Spawner TOUS les agents** en parallèle (un seul message avec 9 Task tool calls) :

   | Nom agent | Type | Rôle |
   |-----------|------|------|
   | `planner` | `implementation-planner` | Planification |
   | `backend-dev` | `dev-backend` | Backend Go |
   | `frontend-dev` | `dev-frontend` | Frontend React |
   | `buzzclick-dev` | `dev-buzzclick` | Firmware ESP32-C3 |
   | `test-writer` | `test-writer` | Rédaction des tests |
   | `code-reviewer` | `code-reviewer` | Revue de code |
   | `qa` | `QA` | Tests et validation |
   | `doc-updater` | `doc-updater` | Documentation |
   | `deployer` | `deploy` | Déploiement QUALIF/PROD |

   Prompt générique pour chaque agent :
   ```
   Tu es [rôle] dans l'équipe BuzzControl. Attends les instructions du team leader (Claude) via les tâches (TaskList) ou les messages directs (SendMessage). Ne commence aucun travail sans assignation explicite.
   ```

3. **Confirme** à l'utilisateur : liste des agents spawned et disponibilité de la team.

## Règles

- Toujours exécuter l'Étape 1 (sync) avant toute autre action
- La TEAM est **toujours créée** — ne jamais demander à l'utilisateur si elle doit être créée
- Le nom de la TEAM est **toujours `myTEAM`** — ne jamais utiliser un autre nom
