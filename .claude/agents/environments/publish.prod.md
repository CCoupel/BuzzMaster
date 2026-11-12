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
2. `govulncheck` propre juste avant le tag (même audit que `release.yml`), et `npm audit --audit-level=high` propre dans `server-go/web` (toutes dépendances, #270).
3. Si un run échoue : analyser, corriger sur la branche, relancer. **Ne jamais merger ni tagger** tant que le run n'est pas vert.
   Un tag ne sert jamais de test.
4. Avant tout tag, aligner les fichiers de version sur `X.Y.Z` exact et rejouer localement la logique du job
   Checking de `release.yml` : `server-go/config.json`, `server-go/internal/server/config.json`,
   `server-go/web/package.json`, `platformio.ini` (`VERSION` + `FIRMWARE_VERSION`), `server-go/assets/firmware/version.txt`
   (suffixe de build accepté par le Checking). Liste de référence : `docs/RELEASE_PROCEDURE.md` §3. Un écart = pas de tag
   (incidents v11.6.0 et v11.7.0, suivi #304).

Sans ce run vert sur le SHA exact, l'ordre PUBLISH PROD est **bloqué** : le deployer ne merge ni ne tague, et le signale au CDP.

## Rappel

- La CI de release (`release.yml`) reconstruit depuis le tag : elle ne remplace pas la validation préalable.
- Tout changement de `release.yml` doit être reflété dans `ci-branch.yml` (même version de Go, mêmes étapes).
