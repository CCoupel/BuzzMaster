---
name: publish.prod
description: "Adaptations projet BuzzControl pour PUBLISH PROD. Base generique : publish.prod.template.md."
---

# Publish PROD — Adaptations BuzzControl

> **Base** : voir `publish.prod.template.md` pour le mécanisme `rebuild-ci` (merge vers `main` + tag officiel).
> Procédure complète : `docs/RELEASE_PROCEDURE.md`, section « 9bis. Validation CI sur le commit exact ».

## Exigence préalable (décision de l'utilisateur, 2026-10-12)

Avant **tout merge dans `main`** et **tout tag** :

1. Un run **vert** de `ci-branch.yml` sur le **commit exact** à publier :
   ```bash
   gh run list --workflow ci-branch.yml --commit <sha>
   ```
2. `govulncheck` propre juste avant le tag (même audit que `release.yml`).
3. Si un run échoue : analyser, corriger sur la branche, relancer. **Ne jamais merger ni tagger** tant que le run n'est pas vert.
   Un tag ne sert jamais de test.

Sans ce run vert sur le SHA exact, l'ordre PUBLISH PROD est **bloqué** : le deployer ne merge ni ne tague, et le signale au CDP.

## Rappel

- La CI de release (`release.yml`) reconstruit depuis le tag : elle ne remplace pas la validation préalable.
- Tout changement de `release.yml` doit être reflété dans `ci-branch.yml` (même version de Go, mêmes étapes).
