---
name: Dispatcher test-writer en parallèle de CHAQUE lot dev
description: Ne jamais laisser un dev écrire ses propres tests faute de handoff test-writer — dispatcher systématiquement les deux en parallèle, même pour un petit lot
type: feedback
---

Pendant le développement du badge de connexion (#109), le lot "Phase 2" (câblage MessageLost/DeliveryConfirmed) a été dispatché à dev-backend seul, sans test-writer en parallèle. dev-backend a dû écrire ses propres tests faute de handoff séparé — l'utilisateur a ensuite demandé si les dev et QA ne faisaient pas de tests en double, ce qui a révélé cet oubli de process.

**Why:** La matrice qualité du projet (`context/QUALITY.md`) répartit strictement : test-writer écrit, QA exécute, code-reviewer analyse. Quand test-writer n'est pas dispatché en même temps qu'un lot dev, le dev improvise des tests lui-même — ce n'est pas un doublon avec QA, mais un vrai gaspillage de temps (le dev context-switch pour écrire des tests au lieu de se concentrer sur l'implémentation) et une entorse à la séparation des rôles.

**How to apply:**
- Dès qu'une tâche dev-backend/dev-frontend est dispatchée (même un petit lot, même un fix mineur en cours de cycle), dispatcher test-writer en parallèle sur le même périmètre dans le même tour de SendMessage.
- Si un lot est trop petit/urgent pour attendre test-writer (ex: fix critique bloquant), c'est acceptable que dev écrive un test minimal de régression lui-même — mais le signaler explicitement et ne pas en faire une habitude.
