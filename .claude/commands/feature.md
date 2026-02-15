# Commande /feature - Workflow Feature Complet

Orchestre le workflow complet de développement d'une feature. **TU** (Claude) es le team leader direct qui coordonne tous les agents.

## Argument reçu

$ARGUMENTS

## Architecture Team Leader Direct

```
TU (Claude) = Team Leader
    │
    ├── TeamCreate("feature-xxx")
    ├── Task(implementation-planner)
    ├── Task(dev-backend/frontend/buzzclick)
    ├── Task(test-writer)
    ├── Task(code-reviewer)
    ├── Task(QA)
    ├── Task(doc-updater)
    └── Task(deploy)
    │
    └── Coordination via TaskUpdate + SendMessage
```

**IMPORTANT** : TU ne lances PAS d'agent CDP. TU es le chef de projet direct.

## Workflow FEATURE

```
┌──────────────────────────────────────────────────────────────────────────┐
│  ANALYSE → BACKLOG → PLAN → DEV → TEST-WRITER → REVIEW → QA → DOC → DEPLOY│
└──────────────────────────────────────────────────────────────────────────┘

Phase 0: ANALYSE (TOI)
    │
    ├── Analyser la demande utilisateur
    ├── Identifier la portée (backend/frontend/firmware)
    ├── Estimer la complexité
    └── Vérifier le backlog/
    │
    ▼
Phase 1: BACKLOG (TOI)
    │
    ├── Chercher dans backlog/*.md si feature existe
    ├── Si trouvée → Proposer à l'utilisateur
    ├── Si validée → Utiliser comme spécification
    └── Créer la branche feature/<nom-court>
    │
    ▼
Phase 2: TEAM SETUP (TOI)
    │
    ├── TeamCreate({ team_name: "feature-xxx", agent_type: "team-lead" })
    ├── Créer les agents nécessaires :
    │   ├── implementation-planner (toujours - créer le plan)
    │   ├── dev-backend (si serveur Go)
    │   ├── dev-frontend (si React/CSS)
    │   ├── dev-buzzclick (si firmware ESP32)
    │   ├── test-writer (toujours)
    │   ├── code-reviewer (toujours)
    │   ├── QA (toujours)
    │   ├── doc-updater (toujours)
    │   └── deploy (toujours - QUALIF)
    └── Créer la task list avec TaskCreate
    │
    ▼
Phase 3: PLANIFICATION (implementation-planner)
    │
    ├── Assigner via TaskUpdate(owner: "planner")
    ├── Recevoir le plan structuré détaillé
    ├── Analyser les dépendances backend ↔ frontend
    ├── Vérifier les contrats API définis
    └── ⏸️ DEMANDER VALIDATION UTILISATEUR (plan)
    │
    ▼
Phase 4: DÉVELOPPEMENT (Agents DEV)
    │
    ├── Assigner les tâches dev via TaskUpdate(owner: "agent-name")
    ├── Attendre résultats via messages automatiques
    ├── Stratégie selon dépendances :
    │   ├── Backend + Frontend (dépendance) → Séquentiel
    │   │   dev-backend → dev-frontend
    │   ├── Backend + Frontend (indépendant) → Parallèle
    │   │   dev-backend ║ dev-frontend
    │   ├── Backend + BuzzClick → Séquentiel
    │   │   dev-backend → dev-buzzclick
    │   └── Feature complète → Hybride
    │       dev-backend → (dev-frontend ║ dev-buzzclick)
    └── Vérifier que les contrats API sont respectés
    │
    ▼
Phase 5: TESTS (test-writer)
    │
    ├── Assigner via TaskUpdate(owner: "test-writer")
    ├── Écrire tests unitaires Go (*_test.go)
    ├── Écrire tests composants React (si applicable)
    ├── Définir scénarios E2E Chrome
    └── Attendre message de complétion
    │
    ▼
Phase 6: REVUE (code-reviewer)
    │
    ├── Assigner via TaskUpdate(owner: "code-reviewer")
    ├── Revue du code ET des tests
    ├── Analyser le verdict :
    │   ├── APPROVED → Phase 7
    │   ├── APPROVED WITH RESERVATIONS → Phase 7 (noter)
    │   └── REJECTED → Retour Phase 4 (cycle++)
    └── Si cycle > 3 → Escalade utilisateur
    │
    ▼
Phase 7: QUALITÉ (QA)
    │
    ├── Assigner via TaskUpdate(owner: "qa-tester")
    ├── Exécuter tests unitaires + E2E Chrome
    ├── Analyser le verdict :
    │   ├── NOT VALIDATED → Retour Phase 4 (cycle++)
    │   └── VALIDATED → Attendre validation utilisateur
    ├── Si cycle > 3 → Escalade utilisateur
    └── ⏸️ VALIDATION UTILISATEUR REQUISE
    │
    ▼
Phase 8: DOCUMENTATION (doc-updater)
    │
    ├── Assigner via TaskUpdate(owner: "doc-updater")
    ├── Incrémenter Y : 2.40.0 → 2.41.0
    ├── CHANGELOG : sections Added/Changed selon le cas
    ├── Mettre à jour CLAUDE.md (patterns, architecture)
    ├── Mettre à jour ADMIN_GUIDE.md si applicable
    └── Finaliser config.json
    │
    ▼
Phase 9: DÉPLOIEMENT QUALIF (deploy)
    │
    ├── Assigner via TaskUpdate(owner: "deploy-qualif")
    ├── Build frontend + backend
    ├── Si firmware modifié : build + validation
    ├── Test server startup
    ├── Générer rapport QUALIF_REPORT_vX.Y.0.md
    └── ⏸️ FIN - PROD via /deploy PROD séparé
```

## Spécificités FEATURE

| Règle | Contrainte |
|-------|-----------|
| **Versioning** | Incrémente Y : 2.40.0 → 2.41.0 |
| **Branche** | `feature/<nom-court>` (ex: feature/memory-modes) |
| **Scope** | Large autorisé - nouvelle fonctionnalité complète |
| **Backlog** | Vérifier backlog/*.md, proposer si trouvé |
| **Plan** | Obligatoire via implementation-planner |
| **Tests** | Unitaires + E2E Chrome obligatoires |
| **Documentation** | Complète (CHANGELOG, CLAUDE.md, ADMIN_GUIDE.md) |
| **Commits** | Atomiques, messages clairs avec "feat:" |

## Gestion des Cycles

```
MAX_CYCLES = 3
cycle = 0

while cycle < MAX_CYCLES:
    dev_result = await agents_dev()
    review_result = await code_reviewer()

    if review_result == REJECTED:
        cycle++
        SendMessage(recipient: "dev-xxx", content: corrections)
        continue

    qa_result = await QA()
    if qa_result == NOT_VALIDATED:
        cycle++
        SendMessage(recipient: "dev-xxx", content: erreurs)
        continue

    break  # Succès

if cycle >= MAX_CYCLES:
    ESCALADE_UTILISATEUR
```

## Points de Validation Utilisateur

| Point | Question |
|-------|----------|
| **Après PLAN** | "Valider ce plan ?" |
| **Après QA** | "QA terminé (VALIDATED). Continuer vers DOC ?" |
| **Escalade 3 cycles** | "3 cycles échoués. Continuer ou abandonner ?" |
| **Fin QUALIF** | "QUALIF prêt. Déployer ?" |

## Format de Reporting (TOI)

### Pendant le workflow

```markdown
## 📊 Feature en cours

**Feature** : [Description courte]
**Branche** : feature/xxx
**Phase** : [X/9 - Nom]
**Cycle** : [N/3]

### Progression
- [x] Analyse
- [x] Backlog
- [x] Team setup
- [x] Planification
- [x] Développement backend
- [x] Développement frontend
- [ ] Tests (en cours...)
- [ ] Revue
- [ ] QA
- [ ] Documentation
- [ ] Déploiement QUALIF

### Agents actifs
- dev-backend : ✅ Terminé
- dev-frontend : ✅ Terminé
- test-writer : 🔄 En cours
```

### Rapport final

```markdown
## ✅ Feature Terminée

**Feature** : [Description]
**Version** : 2.Y.0
**Durée** : XX min
**Cycles** : N

### Agents utilisés
| Agent | Durée | Statut |
|-------|-------|--------|
| implementation-planner | 3 min | ✅ |
| dev-backend | 10 min | ✅ |
| dev-frontend | 8 min | ✅ |
| test-writer | 5 min | ✅ |
| code-reviewer | 3 min | ✅ |
| QA | 6 min | ✅ |
| doc-updater | 3 min | ✅ |
| deploy | 4 min | ✅ |

### Livrables
- Code : X fichiers modifiés
- Tests : X tests unitaires + Y scénarios E2E
- Documentation : CHANGELOG.md, CLAUDE.md, ADMIN_GUIDE.md
- QUALIF : Prêt pour validation

### Prochaine étape
Valider en QUALIF puis `/deploy PROD`
```

## Règles Critiques pour TOI (Team Leader)

### ✅ TU DOIS
- Analyser la demande et vérifier le backlog
- Créer l'équipe avec TeamCreate
- Créer TOUS les agents nécessaires
- Demander validation du plan (après planner)
- Coordonner via TaskUpdate + SendMessage
- Gérer les cycles (max 3)
- Demander validation utilisateur après QA
- Reporter la progression
- Shutdown l'équipe à la fin

### ❌ TU NE DOIS PAS
- Lancer un agent CDP (TU es le leader)
- Écrire du code toi-même
- Exécuter des tests toi-même
- Sauter la phase planification
- Sauter la validation utilisateur (plan et QA)
- Dépasser 3 cycles sans escalade
- Déployer en PROD (seulement QUALIF)

## Agents que TU coordonnes

| Agent | Subagent Type | Rôle |
|-------|---------------|------|
| Planner | `implementation-planner` | Créer le plan d'implémentation |
| Backend Dev | `dev-backend` | Développer serveur Go |
| Frontend Dev | `dev-frontend` | Développer interface React |
| Firmware Dev | `dev-buzzclick` | Développer firmware ESP32 |
| Test Writer | `test-writer` | Écrire tests (unitaires + E2E) |
| Code Reviewer | `code-reviewer` | Analyser qualité |
| QA Tester | `QA` | Exécuter tests |
| Doc Updater | `doc-updater` | Mettre à jour docs |
| Deployer | `deploy` | Déployer QUALIF |

## Action immédiate

**TU** dois maintenant :

1. **Analyser** la demande : `$ARGUMENTS`
2. **Vérifier** backlog/ pour une spec existante
3. **Identifier** la portée (backend/frontend/firmware)
4. **Créer** la branche `feature/<nom-court>`
5. **TeamCreate** pour créer l'équipe
6. **Spawner** tous les agents nécessaires
7. **Orchestrer** le workflow en tant que team leader

**Commence maintenant !**
