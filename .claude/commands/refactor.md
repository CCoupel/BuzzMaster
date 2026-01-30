---
name: refactor
description: Refactoring de code sans changement fonctionnel
---

# Commande /refactor

Workflow pour le refactoring de code existant sans modification du comportement.

## Usage

```
/refactor <description du refactoring>
```

## Quand utiliser

- Simplification de code complexe
- Extraction de fonctions/composants
- Renommage pour meilleure lisibilite
- Elimination de duplication
- Amelioration de la structure

## Workflow

```
/refactor
    │
    ├── 1. ANALYSE (agent Explore)
    │   ├── Identifier le code concerne
    │   ├── Comprendre le comportement actuel
    │   └── Verifier la couverture de tests existante
    │
    ├── 2. PLAN (si necessaire)
    │   └── Pour refactorings complexes uniquement
    │
    ├── 3. DEV (agent approprie)
    │   ├── Refactoring incremental
    │   ├── Tests entre chaque etape
    │   └── Commits atomiques
    │
    ├── 4. REVIEW (code-reviewer)
    │   └── Verification absence de regression
    │
    └── 5. QA (tests complets)
        └── Validation comportement identique
```

## Regles critiques

1. **Tests AVANT refactoring** - Verifier la couverture existante
2. **Pas de changement fonctionnel** - Le comportement doit rester identique
3. **Incremental** - Petits changements, tests frequents
4. **Commits atomiques** - Un commit par etape logique
5. **Si tests cassent** - Annuler et recommencer plus petit

## Differences avec /feature et /bugfix

| Aspect | /feature | /bugfix | /refactor |
|--------|----------|---------|-----------|
| Comportement | Nouveau | Corrige | Identique |
| Tests requis | Nouveaux | Regression | Existants |
| PLAN | Obligatoire | Souvent | Rarement |
| Documentation | Oui | Si majeur | Non |

## Exemples

```
/refactor Extraire la logique de validation dans un helper
/refactor Renommer ProcessButtonPress en HandleBuzzerInput
/refactor Simplifier le switch/case dans engine.go
/refactor Deplacer les constantes QCM dans un fichier dedie
```

## Agent utilise

Lance l'agent de developpement approprie (`dev-backend`, `dev-frontend`, `dev-buzzclick`) selon le code concerne, avec accent sur les tests de non-regression.
