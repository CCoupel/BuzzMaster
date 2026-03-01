# Commande /refactor - Refactoring de Code

Déclenche le workflow de refactoring de code existant sans modification du comportement. **Le CDP** (agent cdp) est le team leader qui coordonne tous les agents. Claude est l'interface utilisateur.

## Argument reçu

$ARGUMENTS

## Architecture CDP

```
Claude (interface) → SendMessage(cdp, "Refactor: xxx")
                          │
                     CDP (Team Leader)
                          │
              ┌───────────┼───────────┐
           qa (avant)  backend-dev  code-reviewer
                          │
                       qa (après)
```

**IMPORTANT** : Claude délègue au CDP. Le CDP est le chef de projet — Claude est uniquement l'interface utilisateur.

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

### Phase 1: TEAM SETUP (TOI — uniquement si myTEAM absente)
- Si myTEAM active → teammates déjà disponibles, passer à Phase 2
- Si bootstrap (aucune team) :
  - TeamCreate({ team_name: "myTEAM", agent_type: "cdp" })
  - Spawner agents (team_name: "myTEAM") :
    - dev-backend/frontend/buzzclick (selon portée)
    - code-reviewer (toujours), QA (toujours)
  - **PAS** de test-writer, doc-updater, deploy (refactoring = pas de nouvelle feature)
  - Voir "Mode Bootstrap" dans agents/cdp.md

### Phase 2: TESTS AVANT (QA)
- Assigner via TaskUpdate(owner: "qa")
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
- Assigner via TaskUpdate(owner: "qa")
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

## Règles Critiques

### ✅ Claude DOIT
- Transmettre le refactoring au CDP avec contexte
- Relayer fidèlement les messages CDP ↔ utilisateur

### ❌ Claude NE DOIT PAS
- Coordonner les agents directement
- Écrire du code lui-même

### ✅ Le CDP DOIT (référence)
- Vérifier que des tests existent AVANT de commencer
- Exécuter les tests AVANT le refactoring
- Coordonner via TaskUpdate + SendMessage
- Exécuter les tests APRÈS et comparer les résultats

### ❌ Le CDP NE DOIT PAS
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

**TU** (Claude) dois maintenant :

1. **Analyser** brièvement le refactoring : `$ARGUMENTS`
2. **Transmettre** au CDP avec contexte :
   ```
   SendMessage(recipient: "cdp", content: "Démarre un workflow REFACTOR: [description + portée + code concerné]", summary: "Refactor: [titre court]")
   ```
3. **Relayer** tous les messages et validations du CDP vers l'utilisateur

**Le CDP prend en charge l'intégralité du workflow depuis cette étape.**


### Sans TEAM (pas de myTEAM actif)
```
subagent_type: "cdp"
description: "Workflow refactor"
prompt: Démarre un workflow REFACTOR: $ARGUMENTS
```
