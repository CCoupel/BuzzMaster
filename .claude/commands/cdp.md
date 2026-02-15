# Commande /cdp - Guide Architecture Team Leader

Documentation et aide sur l'architecture des workflows BuzzControl avec Claude comme team leader direct.

## Argument reçu

$ARGUMENTS

## Architecture Actuelle

```
┌─────────────────────────────────────────────────┐
│  ARCHITECTURE TEAM LEADER DIRECT (depuis 2026)  │
└─────────────────────────────────────────────────┘

User → /workflow → Claude (Team Leader)
                       │
                       ├── TeamCreate("workflow-xxx")
                       ├── Task(agents spécialisés...)
                       └── Coordination via TaskUpdate + SendMessage

PLUS D'AGENT CDP INTERMÉDIAIRE !
```

### Changement d'architecture

**Avant (ancien CDP)** :
```
User → /feature → Agent CDP → dev-backend, QA, etc.
                  (intermédiaire)
```

**Maintenant (team leader direct)** :
```
User → /feature → Claude → TeamCreate + agents
                  (TOI)
```

**Pourquoi** :
- ✅ Plus simple (pas d'intermédiaire)
- ✅ Communication directe User ↔ Claude ↔ Agents
- ✅ Contrôle total et visibilité
- ✅ Moins de latence

## Workflows Disponibles

| Commande | Usage | Version | Agents typiques |
|----------|-------|---------|-----------------|
| `/feature` | Nouvelle fonctionnalité | Y (2.40.0 → 2.41.0) | planner, dev-*, test-writer, reviewer, QA, doc, deploy |
| `/bugfix` | Correction de bug | Z (2.40.0 → 2.40.1) | dev-*, test-writer, reviewer, QA, doc, deploy |
| `/hotfix` | Bug critique en prod | Z+suffix (2.40.1 → 2.40.2-hotfix) | dev-*, doc, deploy (tests optionnels) |
| `/refactor` | Refactoring sans changement | Inchangé | dev-*, reviewer, QA (pas de doc/deploy) |

## Agents Disponibles

Claude (team leader) peut créer et coordonner ces agents :

| Agent | Type | Rôle |
|-------|------|------|
| **Planner** | `implementation-planner` | Créer le plan d'implémentation détaillé |
| **Backend Dev** | `dev-backend` | Développer le serveur Go |
| **Frontend Dev** | `dev-frontend` | Développer l'interface React |
| **Firmware Dev** | `dev-buzzclick` | Développer le firmware ESP32-C3 |
| **Test Writer** | `test-writer` | Écrire les tests (unitaires + E2E) |
| **Code Reviewer** | `code-reviewer` | Analyser la qualité du code |
| **QA Tester** | `QA` | Exécuter les tests et valider |
| **Doc Updater** | `doc-updater` | Mettre à jour la documentation |
| **Deployer** | `deploy` | Déployer vers QUALIF/PROD |

## Architecture d'Équipe

### Création d'une équipe

```javascript
// 1. Claude crée l'équipe
TeamCreate({
  team_name: "feature-memory-modes",
  description: "Ajouter modes Memory multi-équipes",
  agent_type: "team-lead"
})

// 2. Claude spawne les agents nécessaires
Task({
  subagent_type: "implementation-planner",
  team_name: "feature-memory-modes",
  name: "planner",
  prompt: "Créer le plan d'implémentation..."
})

Task({
  subagent_type: "dev-backend",
  team_name: "feature-memory-modes",
  name: "backend-dev",
  prompt: "Implémenter le backend selon le plan..."
})

// ... autres agents

// 3. Claude coordonne
TaskUpdate({ taskId: "1", owner: "planner", status: "in_progress" })
SendMessage({ recipient: "planner", content: "...", summary: "..." })
```

### Coordination

Claude coordonne les agents via :
- **TaskCreate** : Créer les tâches
- **TaskUpdate** : Assigner les tâches (owner: "agent-name")
- **SendMessage** : Communiquer avec les agents
- **Messages automatiques** : Les agents envoient leurs résultats

### Gestion des cycles

```
MAX_CYCLES = 3

while cycle < 3:
  dev_result = await agents_dev()
  review_result = await code_reviewer()

  if review_result == REJECTED:
    cycle++
    SendMessage(corrections)
    continue

  qa_result = await QA()
  if qa_result == NOT_VALIDATED:
    cycle++
    SendMessage(corrections)
    continue

  break  # Succès
```

## Workflow Type : Feature

```
Phase 0: ANALYSE (Claude)
    └── Analyser demande, vérifier backlog, créer branche

Phase 1: TEAM SETUP (Claude)
    └── TeamCreate + spawner agents

Phase 2: PLANIFICATION (implementation-planner)
    └── Créer plan détaillé → VALIDATION USER

Phase 3: DÉVELOPPEMENT (dev-*)
    └── Implémenter selon plan (séquentiel/parallèle)

Phase 4: TESTS (test-writer)
    └── Écrire tests unitaires + E2E

Phase 5: REVUE (code-reviewer)
    └── Analyser qualité → APPROVED / REJECTED

Phase 6: QA (QA)
    └── Exécuter tests → VALIDATED / NOT_VALIDATED

Phase 7: DOCUMENTATION (doc-updater)
    └── CHANGELOG, CLAUDE.md, ADMIN_GUIDE.md

Phase 8: DÉPLOIEMENT (deploy)
    └── Build + QUALIF → VALIDATION USER
```

## Points de Validation Utilisateur

Claude demande validation à l'utilisateur à ces moments :

| Workflow | Points de validation |
|----------|---------------------|
| **Feature** | Après PLAN, Après QA |
| **Bugfix** | Après QA |
| **Hotfix** | Après DOC (avant PROD) |
| **Refactor** | Aucune (sauf si cycle > 3) |

## Règles d'Or de Claude (Team Leader)

### ✅ Claude DOIT
- Créer l'équipe avec TeamCreate
- Spawner TOUS les agents nécessaires
- Coordonner via TaskUpdate + SendMessage
- Gérer les cycles (max 3)
- Demander validation utilisateur aux points clés
- Reporter la progression
- Shutdown l'équipe à la fin

### ❌ Claude NE DOIT PAS
- Lancer un agent CDP (n'existe plus)
- Écrire du code lui-même
- Exécuter des tests lui-même
- Sauter les validations utilisateur
- Dépasser 3 cycles sans escalade

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

## Aide par Mot-clé

Si `$ARGUMENTS` contient un mot-clé :

| Mot-clé | Action |
|---------|--------|
| `help` | Afficher cette documentation |
| `workflows` | Lister les workflows disponibles |
| `agents` | Lister les agents disponibles |
| `architecture` | Expliquer l'architecture team leader |
| `feature` | Guide workflow feature |
| `bugfix` | Guide workflow bugfix |
| `hotfix` | Guide workflow hotfix |
| `refactor` | Guide workflow refactor |

## Exemples d'Usage

### Feature complète

```bash
/feature Ajouter le mode Memory multi-équipes
```

Claude va :
1. Vérifier backlog/
2. Créer branche feature/memory-modes
3. TeamCreate + spawner agents
4. Orchestrer : PLAN → DEV → TEST → REVIEW → QA → DOC → DEPLOY

### Bugfix simple

```bash
/bugfix Le score ne s'affiche pas en mode QCM
```

Claude va :
1. Créer branche bugfix/qcm-score-display
2. TeamCreate + spawner agents minimaux
3. Orchestrer : DEV → TEST → REVIEW → QA → DOC → DEPLOY

### Hotfix critique

```bash
/hotfix Crash serveur au démarrage en PROD
```

Claude va :
1. Créer branche hotfix/server-crash
2. TeamCreate + agents minimaux
3. Orchestrer : DEV → DOC → DEPLOY PROD (tests optionnels)
4. Créer post-mortem

## Migration depuis l'ancien CDP

Si vous avez des références à l'ancien CDP :

| Ancien | Nouveau |
|--------|---------|
| Lancer agent CDP | Claude est le leader direct |
| `/cdp status` | Reporter progression dans le workflow |
| `/cdp context` | Ajouter info dans le prompt des agents |
| `/cdp abort` | Demander à Claude d'abandonner |
| `/cdp config` | Architecture fixe (voir ci-dessus) |

## Action Immédiate

**Analyser** `$ARGUMENTS` :

1. Si `help` / vide → Afficher cette aide
2. Si `workflows` → Lister `/feature`, `/bugfix`, `/hotfix`, `/refactor`
3. Si `agents` → Lister les agents disponibles
4. Si `architecture` → Expliquer team leader direct
5. Si `<workflow>` → Expliquer le workflow spécifique
6. Sinon → Afficher cette aide

**Cette commande ne lance AUCUN workflow.** Utilise `/feature`, `/bugfix`, `/hotfix` ou `/refactor` pour lancer un workflow.
