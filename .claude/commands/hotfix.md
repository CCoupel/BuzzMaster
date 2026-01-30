---
name: hotfix
description: Correction urgente en production (workflow accelere)
---

# Commande /hotfix

Workflow accelere pour les bugs critiques en production.

## Usage

```
/hotfix <description du bug critique>
```

## Quand utiliser

- Bug bloquant en production
- Probleme de securite urgent
- Regression critique apres release

## Workflow accelere

```
/hotfix
    │
    ├── 1. ANALYSE RAPIDE (pas de PLAN complet)
    │   └── Identifier la cause et le fix minimal
    │
    ├── 2. DEV (agent approprie)
    │   └── Fix minimal, pas de refactoring
    │
    ├── 3. TESTS CRITIQUES UNIQUEMENT
    │   └── Tests de non-regression sur le bug
    │
    ├── 4. DEPLOY PROD DIRECT
    │   └── Skip QUALIF si vraiment critique
    │   └── Tag: v<version>-hotfix
    │
    └── 5. POST-MORTEM
        └── Documenter l'incident
```

## Differences avec /bugfix

| Aspect | /bugfix | /hotfix |
|--------|---------|---------|
| Workflow | CDP complet | Accelere |
| QUALIF | Obligatoire | Optionnel |
| Tests | Tous | Critiques seulement |
| Tag | `vX.Y.Z` | `vX.Y.Z-hotfix` |
| Documentation | Complete | Post-mortem |

## Exemple

```
/hotfix Le serveur crash quand plus de 10 buzzers se connectent
```

## Regles critiques

1. **Fix minimal** - Ne corriger QUE le bug, rien d'autre
2. **Pas de refactoring** - Ce n'est pas le moment
3. **Test de non-regression** - Obligatoire meme en urgence
4. **Post-mortem** - Documenter apres coup
5. **Suivi** - Creer un /bugfix pour fix propre si necessaire

## Agent utilise

Lance directement l'agent de developpement approprie (`dev-backend`, `dev-frontend`, `dev-buzzclick`) puis `deploy` en mode hotfix.
