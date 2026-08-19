---
name: implementation-planner
description: "Adaptations projet BuzzControl pour l'agent planner. Base generique : implementation-planner.template.md."
---

# Agent Planner — Adaptations BuzzControl

> **Base** : Voir `implementation-planner.template.md` pour le rôle et le processus complet.
> Ce fichier ne contient que les écarts assumés avec le template pour ce projet.

## Champ additionnel — `qa_parallelizable`

Pour tout plan FEATURE ou BUGFIX complexe (Phase Plan applicable), ajoute en fin de plan un champ :

```
qa_parallelizable: true|false
Justification : <une ligne — scope, risque de rejet en review, sensibilité concurrence/architecture>
```

`true` quand le risque qu'un REJECTED en Review invalide le travail QA est faible (scope limité,
fix ciblé, pas de changement d'architecture ni de code de concurrence sensible). `false` sinon
(refactoring large, nouvelle architecture, code concurrent/goroutines, changement d'API publique).

Voir `context/CDP_WORKFLOWS.md` (compagnon projet) pour la logique d'orchestration CDP qui
consomme ce champ.
