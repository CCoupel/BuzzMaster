# Commande /hotfix - Correction Urgente en Production

Workflow accéléré pour les bugs critiques en production. **TU** (Claude) es le team leader direct qui coordonne tous les agents.

## Argument reçu

$ARGUMENTS

## Architecture Team Leader Direct

```
TU (Claude) = Team Leader
    │
    ├── TeamCreate("hotfix-xxx")
    ├── Task(dev-backend/frontend/buzzclick)
    ├── Task(test-writer) [optionnel si vraiment urgent]
    ├── Task(QA) [optionnel si vraiment urgent]
    ├── Task(doc-updater)
    └── Task(deploy)
    │
    └── Coordination via TaskUpdate + SendMessage
```

**IMPORTANT** : TU ne lances PAS d'agent CDP. TU es le chef de projet direct.

## Workflow HOTFIX (Accéléré)

```
┌─────────────────────────────────────────────────────────────┐
│  ANALYSE → DEV → [TESTS]* → [QA]* → DOC → DEPLOY(PROD)     │
└─────────────────────────────────────────────────────────────┘
* = Optionnel si vraiment critique
```

### Phase 0: ANALYSE (TOI)
- Analyser la criticité du bug (sécurité, bloquant, régression)
- Identifier la portée (backend/frontend/firmware)
- Estimer l'urgence réelle
- Créer la branche `hotfix/<nom-court>`

### Phase 1: TEAM SETUP (TOI)
- TeamCreate({ team_name: "hotfix-xxx", agent_type: "team-lead" })
- Créer les agents MINIMAUX :
  - dev-backend/frontend/buzzclick (selon portée)
  - test-writer (si possible)
  - QA (si possible)
  - doc-updater (toujours)
  - deploy (toujours - PROD direct)

### Phase 2: DÉVELOPPEMENT (Agents DEV)
- Assigner via TaskUpdate(owner: "dev-xxx")
- Correction MINIMALE uniquement
- Commits atomiques avec "hotfix:"

### Phase 3: TESTS (test-writer) [OPTIONNEL]
- Si temps disponible : test de non-régression
- Sinon : skip et noter dans post-mortem

### Phase 4: QA (qa-tester) [OPTIONNEL]
- Si temps disponible : tests de base
- Sinon : skip et noter dans post-mortem

### Phase 5: DOCUMENTATION (doc-updater)
- Incrémenter Z + suffix : 2.40.1 → 2.40.2-hotfix
- CHANGELOG : section Fixed
- Post-mortem OBLIGATOIRE

### Phase 6: DÉPLOIEMENT PROD (deploy)
- **QUALIF OPTIONNEL** si vraiment critique
- Build et déploiement PROD direct
- Monitoring actif après déploiement

## Spécificités HOTFIX

| Règle | Contrainte |
|-------|-----------|
| **Versioning** | Incrémente Z + suffix : 2.40.1 → 2.40.2-hotfix |
| **Branche** | `hotfix/<nom-court>` |
| **Scope** | MINIMAL absolu - corriger UNIQUEMENT le bug critique |
| **QUALIF** | OPTIONNEL si vraiment critique |
| **Tests** | Optionnel si urgence vraie (noter dans post-mortem) |
| **Post-mortem** | OBLIGATOIRE (cause, impact, prévention) |
| **Commits** | Atomiques avec "hotfix:" |

## Quand utiliser /hotfix

| Situation | Action |
|-----------|--------|
| Bug bloquant en production | ✅ /hotfix |
| Problème de sécurité urgent | ✅ /hotfix |
| Régression critique après release | ✅ /hotfix |
| Bug gênant mais non bloquant | ❌ Utiliser /bugfix |
| Amélioration urgente | ❌ Utiliser /feature |

## Post-mortem OBLIGATOIRE

À la fin du hotfix, créer `docs/postmortem/hotfix-vX.Y.Z.md` :

```markdown
# Post-mortem Hotfix vX.Y.Z

## Résumé
[Description du bug critique]

## Chronologie
- HH:MM - Bug détecté
- HH:MM - Hotfix démarré
- HH:MM - Correction déployée

## Cause racine
[Pourquoi ce bug est apparu]

## Impact
- Utilisateurs affectés : [nombre/tous]
- Durée : [minutes]
- Données perdues : [oui/non]

## Correction
[Description de la correction appliquée]

## Tests skippés
- [ ] Tests unitaires (raison: urgence)
- [ ] QA E2E (raison: urgence)
- [ ] QUALIF (raison: urgence)

## Prévention future
[Actions pour éviter ce type de bug]

## Actions post-déploiement
- [ ] Surveiller logs pendant 1h
- [ ] Ajouter tests de non-régression
- [ ] Mettre à jour documentation
```

## Règles Critiques pour TOI (Team Leader)

### ✅ TU DOIS
- Vérifier que c'est vraiment CRITIQUE
- Créer l'équipe avec TeamCreate
- Coordonner via TaskUpdate + SendMessage
- Documenter ce qui a été skippé
- Créer le post-mortem OBLIGATOIRE
- Surveiller le déploiement
- Shutdown l'équipe à la fin

### ❌ TU NE DOIS PAS
- Lancer un agent CDP (TU es le leader)
- Écrire du code toi-même
- Skip le post-mortem
- Autoriser du refactoring
- Attendre QUALIF si vraiment critique

## Agents que TU coordonnes

| Agent | Subagent Type | Usage HOTFIX |
|-------|---------------|--------------|
| Backend Dev | `dev-backend` | Si bug serveur Go |
| Frontend Dev | `dev-frontend` | Si bug React/CSS |
| Firmware Dev | `dev-buzzclick` | Si bug ESP32 |
| Test Writer | `test-writer` | Optionnel |
| QA Tester | `QA` | Optionnel |
| Doc Updater | `doc-updater` | Toujours (post-mortem) |
| Deployer | `deploy` | Toujours (PROD direct) |

## Action immédiate

**TU** dois maintenant :

1. **Analyser** la criticité : `$ARGUMENTS`
2. **Vérifier** que c'est vraiment un HOTFIX (sinon → /bugfix)
3. **Créer** la branche `hotfix/<nom-court>`
4. **TeamCreate** pour créer l'équipe
5. **Spawner** les agents MINIMAUX nécessaires
6. **Orchestrer** le workflow accéléré
7. **Créer** le post-mortem

**Commence maintenant !**
