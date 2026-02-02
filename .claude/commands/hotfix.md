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

## Mots-clés de contrôle

**Référence :** Voir `context/COMMON.md` section 12

```
/hotfix help                → Afficher l'aide
/hotfix status              → État du workflow
/hotfix resume deploy       → Reprendre au déploiement
/hotfix jumpto "fix urgent" → Aller à une tâche précise
```

## Références

**Contexte projet :** Voir `context/COMMON.md` section 1
**Workflow CDP :** Voir `context/CDP_WORKFLOWS.md`
- Type : HOTFIX (section 2)
- Règles : section 8 (HOTFIX)
- Comparaison types : section 2

## Quand utiliser

- Bug bloquant en production
- Probleme de securite urgent
- Regression critique apres release

## Spécificités HOTFIX

- Incrémente Z + suffix : 2.40.1 → 2.40.2-hotfix
- Branche : `hotfix/<nom-court>`
- QUALIF optionnel si vraiment critique
- Post-mortem requis

## Agent utilisé

Lance l'agent DEV approprié puis `deploy` en mode hotfix.
Voir `context/DEVELOPMENT.md` section 2 pour le dispatch.
