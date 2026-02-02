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

## Références

**Contexte projet :** Voir `context/COMMON.md` section 1
**Workflow CDP :** Voir `context/CDP_WORKFLOWS.md`
- Type : REFACTOR (section 2)
- Règles : section 8 (REFACTOR)
- Comparaison types : section 2

## Quand utiliser

- Simplification de code complexe
- Extraction de fonctions/composants
- Renommage pour meilleure lisibilite
- Elimination de duplication

## Spécificités REFACTOR

- Version inchangée
- Branche : `refactor/<nom-court>`
- Tests AVANT refactoring obligatoires
- Comportement identique obligatoire
- Pas de documentation

## Agent utilisé

Lance l'agent DEV approprié selon le code concerné.
Voir `context/DEVELOPMENT.md` section 2 pour le dispatch.
