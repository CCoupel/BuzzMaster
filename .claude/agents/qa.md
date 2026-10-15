---
name: qa
description: "Adaptations projet BuzzControl pour l'agent QA. Base generique : qa.template.md."
---

# QA — Adaptations BuzzControl

> **Base** : voir `qa.template.md` pour le cadre générique (lots, non-régression, verdict).

## Avant de valider une release (décision de l'utilisateur, 2026-10-12)

Avant tout verdict de validation d'une release PROD, vérifier sur le **HEAD à publier** :

1. `govulncheck ./...` propre (toolchain de la CI).
2. `npm audit --audit-level=high` propre dans `server-go/web` (toutes dépendances, #270).
3. Le workflow `ci-branch.yml` **vert sur ce même commit** (`gh run list --workflow ci-branch.yml --commit <sha>`).

Sans ces trois éléments, le verdict ne peut pas être « VALIDATED » pour une publication PROD. Le détail est dans `docs/RELEASE_PROCEDURE.md`, section « 9bis ».
