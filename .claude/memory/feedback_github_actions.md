---
name: GitHub actions require user validation
description: Fermer/rouvrir des issues GitHub ou créer des PR nécessite une validation explicite de l'utilisateur — jamais automatiquement par un agent
type: feedback
---

Ne jamais fermer, rouvrir, commenter ou modifier des issues/PR GitHub sans validation explicite de l'utilisateur.

**Why:** L'issue #64 a été fermée automatiquement par le code-reviewer pendant le workflow v3.6.7, sans que l'utilisateur l'ait validé. L'utilisateur l'a remarqué et a demandé à la rouvrir.

**How to apply:**
- Seul le **CDP** est autorisé à fermer des issues GitHub — aucun autre agent (code-reviewer, backend-dev, etc.)
- Le CDP ne ferme une issue qu'**après validation explicite des tests QUALIF par l'utilisateur** (pas après QA unitaire, pas après review)
- La clôture d'une issue intervient en Phase 9 (DEPLOY QUALIF) une fois que l'utilisateur a validé visuellement la feature en QUALIF
- Les autres agents (code-reviewer, backend-dev…) ne doivent jamais exécuter de commandes `gh issue close/reopen/comment` de leur propre initiative
