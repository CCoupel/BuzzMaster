# Commande /refactor - Refactoring de Code

Workflow pour le refactoring de code existant sans modification du comportement. **TU** (Claude) es le team leader direct qui coordonne tous les agents.

## Argument reçu

$ARGUMENTS

## Architecture Team Leader Direct

```
TU (Claude) = Team Leader
    │
    ├── TeamCreate("refactor-xxx")
    ├── Task(dev-backend/frontend/buzzclick)
    ├── Task(code-reviewer)
    └── Task(QA)
    │
    └── Coordination via TaskUpdate + SendMessage
```

**IMPORTANT** : TU ne lances PAS d'agent CDP. TU es le chef de projet direct.

## Workflow REFACTOR

```
┌──────────────────────────────────────────────────┐
│  ANALYSE → TESTS AVANT → DEV → REVIEW → QA      │
└──────────────────────────────────────────────────┘
```

### Phase 0: ANALYSE (TOI)
- Analyser le code à refactorer
- Identifier la portée (backend/frontend/firmware)
- Vérifier qu'il y a des tests existants
- Créer la branche `refactor/<nom-court>`

### Phase 1: TEAM SETUP (TOI)
- TeamCreate({ team_name: "refactor-xxx", agent_type: "team-lead" })
- Créer les agents nécessaires :
  - dev-backend/frontend/buzzclick (selon portée)
  - code-reviewer (toujours)
  - QA (toujours)
- **PAS** de test-writer (tests déjà existants)
- **PAS** de doc-updater (pas de nouvelle feature)
- **PAS** de deploy (pas de nouvelle version)

### Phase 2: TESTS AVANT (QA)
- Assigner via TaskUpdate(owner: "qa-tester")
- Exécuter les tests AVANT le refactoring
- **OBLIGATOIRE** : Tous les tests doivent passer
- Si tests échouent → BLOQUER le refactoring

### Phase 3: DÉVELOPPEMENT (Agents DEV)
- Assigner via TaskUpdate(owner: "dev-xxx")
- Refactoring du code (extraction, renommage, simplification)
- **INTERDIT** : Changer le comportement
- Commits atomiques avec "refactor:"

### Phase 4: REVUE (code-reviewer)
- Assigner via TaskUpdate(owner: "code-reviewer")
- Vérifier que le comportement n'a PAS changé
- Vérifier la qualité du refactoring
- Si comportement modifié → REJETER

### Phase 5: TESTS APRÈS (QA)
- Assigner via TaskUpdate(owner: "qa-tester")
- Exécuter les MÊMES tests qu'avant
- **OBLIGATOIRE** : Tous les tests doivent passer
- **OBLIGATOIRE** : Résultats identiques à avant

## Spécificités REFACTOR

| Règle | Contrainte |
|-------|-----------|
| **Versioning** | **INCHANGÉ** - pas de nouvelle version |
| **Branche** | `refactor/<nom-court>` |
| **Comportement** | STRICTEMENT IDENTIQUE (tests prouvent) |
| **Tests avant** | OBLIGATOIRE - doivent tous passer |
| **Tests après** | OBLIGATOIRE - mêmes résultats qu'avant |
| **Documentation** | Aucune (pas de nouvelle feature) |
| **Commits** | Atomiques avec "refactor:" |

## Quand utiliser /refactor

| Situation | Action |
|-----------|--------|
| Simplifier code complexe | ✅ /refactor |
| Extraire fonctions/composants | ✅ /refactor |
| Renommer pour meilleure lisibilité | ✅ /refactor |
| Éliminer duplication | ✅ /refactor |
| Améliorer performance | ❌ /feature (si mesurable) |
| Corriger un bug | ❌ /bugfix |
| Ajouter fonctionnalité | ❌ /feature |

## Types de Refactoring

| Type | Exemple |
|------|---------|
| **Extraction** | Extraire fonction, méthode, composant |
| **Renommage** | Renommer variable, fonction pour clarté |
| **Simplification** | Réduire complexité cyclomatique |
| **Élimination duplication** | DRY (Don't Repeat Yourself) |
| **Réorganisation** | Déplacer code vers bon fichier |

## Règle d'Or du Refactoring

```
TESTS_AVANT == TESTS_APRÈS
```

Si les résultats diffèrent → Le refactoring est **INVALIDE**.

## Règles Critiques pour TOI (Team Leader)

### ✅ TU DOIS
- Vérifier que des tests existent AVANT de commencer
- Créer l'équipe avec TeamCreate
- Exécuter les tests AVANT le refactoring
- Coordonner via TaskUpdate + SendMessage
- Exécuter les tests APRÈS le refactoring
- Comparer les résultats (doivent être identiques)
- Shutdown l'équipe à la fin

### ❌ TU NE DOIS PAS
- Lancer un agent CDP (TU es le leader)
- Écrire du code toi-même
- Autoriser un changement de comportement
- Créer une nouvelle version
- Mettre à jour la documentation
- Déployer (merge dans main suffit)

## Agents que TU coordonnes

| Agent | Subagent Type | Usage REFACTOR |
|-------|---------------|----------------|
| Backend Dev | `dev-backend` | Si refactor serveur Go |
| Frontend Dev | `dev-frontend` | Si refactor React/CSS |
| Firmware Dev | `dev-buzzclick` | Si refactor ESP32 |
| Code Reviewer | `code-reviewer` | Vérifier comportement identique |
| QA Tester | `QA` | Tests avant ET après |

## Action immédiate

**TU** dois maintenant :

1. **Analyser** le refactoring demandé : `$ARGUMENTS`
2. **Vérifier** que des tests existent pour le code concerné
3. **Créer** la branche `refactor/<nom-court>`
4. **TeamCreate** pour créer l'équipe
5. **Spawner** les agents nécessaires
6. **Exécuter** les tests AVANT (QA)
7. **Orchestrer** le refactoring (DEV)
8. **Vérifier** la qualité (REVIEW)
9. **Exécuter** les tests APRÈS (QA)
10. **Comparer** les résultats

**Commence maintenant !**
