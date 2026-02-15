# Commande /bugfix - Workflow Correction de Bug

Orchestre le workflow de correction de bug. **TU** (Claude) es le team leader direct qui coordonne tous les agents.

## Argument reçu

$ARGUMENTS

## Architecture Team Leader Direct

```
TU (Claude) = Team Leader
    │
    ├── TeamCreate("bugfix-xxx")
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

## Workflow BUGFIX

```
┌─────────────────────────────────────────────────────────────────┐
│  ANALYSE → GIT → DEV → TEST-WRITER → REVIEW → QA → DOC → DEPLOY│
└─────────────────────────────────────────────────────────────────┘

Phase 0: ANALYSE (TOI)
    │
    ├── Analyser la description du bug
    ├── Identifier la portée (backend/frontend/firmware)
    ├── Déterminer la complexité
    └── Créer la branche bugfix/<nom-court>
    │
    ▼
Phase 1: TEAM SETUP (TOI)
    │
    ├── TeamCreate({ team_name: "bugfix-xxx", agent_type: "team-lead" })
    ├── Créer les agents nécessaires selon la portée :
    │   ├── dev-backend (si bug serveur Go)
    │   ├── dev-frontend (si bug React/CSS)
    │   ├── dev-buzzclick (si bug firmware ESP32)
    │   ├── test-writer (toujours - test régression OBLIGATOIRE)
    │   ├── code-reviewer (toujours)
    │   ├── QA (toujours)
    │   ├── doc-updater (toujours)
    │   └── deploy (toujours - QUALIF)
    └── Créer la task list avec TaskCreate
    │
    ▼
Phase 2: DÉVELOPPEMENT (Agents DEV)
    │
    ├── Assigner les tâches dev via TaskUpdate(owner: "agent-name")
    ├── Attendre résultats via messages automatiques
    ├── Stratégie :
    │   ├── Backend seul → dev-backend
    │   ├── Frontend seul → dev-frontend
    │   ├── Firmware seul → dev-buzzclick
    │   ├── Backend + Frontend (dépendance) → séquentiel
    │   └── Backend + Frontend (indépendant) → parallèle
    └── Vérifier que le scope est MINIMAL (pas de refactoring)
    │
    ▼
Phase 3: TESTS (test-writer)
    │
    ├── Assigner via TaskUpdate(owner: "test-writer")
    ├── OBLIGATOIRE : Test de non-régression
    ├── Écrire tests unitaires Go (*_test.go)
    ├── Définir scénarios E2E si applicable
    └── Attendre message de complétion
    │
    ▼
Phase 4: REVUE (code-reviewer)
    │
    ├── Assigner via TaskUpdate(owner: "code-reviewer")
    ├── Analyser le verdict :
    │   ├── APPROVED → Phase 5
    │   ├── APPROVED WITH RESERVATIONS → Phase 5 (noter)
    │   └── REJECTED → Retour Phase 2 (cycle++)
    └── Si cycle > 3 → Escalade utilisateur
    │
    ▼
Phase 5: QUALITÉ (QA)
    │
    ├── Assigner via TaskUpdate(owner: "qa-tester")
    ├── Exécuter tests unitaires + E2E
    ├── Analyser le verdict :
    │   ├── NOT VALIDATED → Retour Phase 2 (cycle++)
    │   └── VALIDATED → Attendre validation utilisateur
    ├── Si cycle > 3 → Escalade utilisateur
    └── ⏸️ VALIDATION UTILISATEUR REQUISE
    │
    ▼
Phase 6: DOCUMENTATION (doc-updater)
    │
    ├── Assigner via TaskUpdate(owner: "doc-updater")
    ├── Incrémenter Z : 2.40.0 → 2.40.1
    ├── CHANGELOG : section "Fixed" uniquement
    ├── Mettre à jour CLAUDE.md si nécessaire
    └── Finaliser config.json
    │
    ▼
Phase 7: DÉPLOIEMENT QUALIF (deploy)
    │
    ├── Assigner via TaskUpdate(owner: "deploy-qualif")
    ├── Build frontend + backend
    ├── Si firmware modifié : build + validation
    ├── Test server startup
    ├── Générer rapport QUALIF_REPORT_vX.Y.Z.md
    └── ⏸️ FIN - PROD via /deploy PROD séparé
```

## Spécificités BUGFIX

| Règle | Contrainte |
|-------|-----------|
| **Versioning** | Incrémente Z : 2.40.0 → 2.40.1 |
| **Branche** | `bugfix/<nom-court>` (ex: bugfix/wifi-autostart) |
| **Scope** | MINIMAL - corriger UNIQUEMENT le bug |
| **Test** | Non-régression OBLIGATOIRE |
| **Refactoring** | INTERDIT - pas de nettoyage de code |
| **CHANGELOG** | Section "Fixed" uniquement |
| **Commits** | Atomiques, message clair avec "fix:" |

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
| **Après QA** | "QA terminé (VALIDATED). Continuer vers DOC ?" |
| **Escalade 3 cycles** | "3 cycles échoués. Continuer ou abandonner ?" |
| **Fin QUALIF** | "QUALIF prêt. Déployer ?" |

## Format de Reporting (TOI)

### Pendant le workflow

```markdown
## 📊 Bugfix en cours

**Bug** : [Description courte]
**Branche** : bugfix/xxx
**Phase** : [X/7 - Nom]
**Cycle** : [N/3]

### Progression
- [x] Analyse
- [x] Team setup
- [x] Développement backend
- [ ] Tests (en cours...)
- [ ] Revue
- [ ] QA
- [ ] Documentation
- [ ] Déploiement QUALIF

### Agents actifs
- dev-backend : ✅ Terminé
- test-writer : 🔄 En cours
```

### Rapport final

```markdown
## ✅ Bugfix Terminé

**Bug** : [Description]
**Version** : 2.40.Z
**Durée** : XX min
**Cycles** : N

### Agents utilisés
| Agent | Durée | Statut |
|-------|-------|--------|
| dev-backend | 5 min | ✅ |
| test-writer | 3 min | ✅ |
| code-reviewer | 2 min | ✅ |
| QA | 4 min | ✅ |
| doc-updater | 2 min | ✅ |
| deploy | 3 min | ✅ |

### Livrables
- Code : X fichiers modifiés
- Tests : Test régression + X tests existants
- Documentation : CHANGELOG.md (Fixed)
- QUALIF : Prêt pour validation

### Prochaine étape
Valider en QUALIF puis `/deploy PROD`
```

## Règles Critiques pour TOI (Team Leader)

### ✅ TU DOIS
- Analyser le bug avant de créer l'équipe
- Créer l'équipe avec TeamCreate
- Créer TOUS les agents nécessaires
- Coordonner via TaskUpdate + SendMessage
- Gérer les cycles (max 3)
- Demander validation utilisateur après QA
- Reporter la progression
- Shutdown l'équipe à la fin

### ❌ TU NE DOIS PAS
- Lancer un agent CDP (TU es le leader)
- Écrire du code toi-même
- Exécuter des tests toi-même
- Sauter la validation utilisateur après QA
- Dépasser 3 cycles sans escalade
- Autoriser du refactoring
- Déployer en PROD (seulement QUALIF)

## Agents que TU coordonnes

| Agent | Subagent Type | Rôle |
|-------|---------------|------|
| Backend Dev | `dev-backend` | Corriger bug serveur Go |
| Frontend Dev | `dev-frontend` | Corriger bug React/CSS |
| Firmware Dev | `dev-buzzclick` | Corriger bug ESP32 |
| Test Writer | `test-writer` | Écrire test régression |
| Code Reviewer | `code-reviewer` | Analyser qualité |
| QA Tester | `QA` | Exécuter tests |
| Doc Updater | `doc-updater` | Mettre à jour docs |
| Deployer | `deploy` | Déployer QUALIF |

## Action immédiate

**TU** dois maintenant :

1. **Analyser** la description du bug : `$ARGUMENTS`
2. **Identifier** la portée (backend/frontend/firmware)
3. **Créer** la branche `bugfix/<nom-court>`
4. **TeamCreate** pour créer l'équipe
5. **Spawner** les agents nécessaires
6. **Orchestrer** le workflow en tant que team leader

**Commence maintenant !**
