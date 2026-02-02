# Commande /review - Workflow de Revue de Code Périodique

Orchestre un workflow autonome de revue de code pour améliorer la qualité, la sécurité et la maintenabilité du codebase.

## Argument reçu (optionnel)

$ARGUMENTS

## Mot-clé help

`/review help` → Affiche :

```
## /review - Aide

**Description** : Workflow de revue de code périodique

**Usage** :
  /review help            Afficher cette aide
  /review                 Revue complète du codebase
  /review security        Focus sur la sécurité
  /review performance     Focus sur les performances
  /review rationalization Focus sur la rationalisation/refactoring

**Phases** : [Git] → PLAN → [Validation] → DEV → QA → DOC → DEPLOY(QUALIF)
```

**Formats possibles** :
- `/review` : Revue complète du codebase
- `/review security` : Focus sur la sécurité
- `/review performance` : Focus sur les performances
- `/review rationalization` : Focus sur la rationalisation/refactoring

## Workflow complet

```
[Vérifications] → [Git] → PLAN → [Validation] → DEV → QA → [Validation] → DOC → DEPLOY(QUALIF) → [FIN]
```

## Références

**Contexte projet :** Voir `context/COMMON.md` section 1
**Framework Qualité :** Voir `context/QUALITY.md`
- Framework review : section 4
- Workflow /review : section 11
- Verdicts : section 5

## Instructions

Workflow complet de revue périodique. Voir `context/QUALITY.md` section 11.

### Phases

1. **Phase 0** : Vérifications préalables (Git propre)
2. **Phase 1** : Branche `review/code-review-YYYY-MM-DD`
3. **Phase 2** : Analyse et plan (validation utilisateur)
4. **Phase 3** : Implémentation des optimisations validées
5. **Phase 4** : Tests QA
6. **Phase 5** : Documentation
7. **Phase 6** : Déploiement QUALIF

## Points de validation

1. Phase 0 : Git propre
2. Après PLAN : Optimisations à implémenter
3. Après QA : Tests manuels OK

## Action immédiate

Lancer Phase 0 (vérifications Git).
