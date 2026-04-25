---
name: Agent context crash prevention
description: Les agents tmux crashent (runtime bun) quand leur contexte est trop grand — comment l'éviter
type: feedback
---

Les agents CDP et backend-dev peuvent crasher avec un stack trace bun (`createInstance`) quand leur contexte accumulé est trop important (~4-5MB d'historique tmux).

**Why:** Le runtime bun de Claude Code a une limite de contexte. CDP accumule les messages de tous les agents, backend-dev accumule des diffs/logs. Sur un workflow long (v3.6.7 + v3.6.8 enchaînés), le seuil est atteint.

**How to apply:**
- Re-spawner CDP et les agents DEV entre chaque version majeure (pas juste entre phases)
- Garder les messages inter-agents courts : résumé + fichiers modifiés, pas les rapports détaillés complets
- Pour les workflows courts (2-3 fixes XS), agir directement en tant que CDP plutôt que déléguer — évite la saturation
- CDP est le point de défaillance prioritaire : il reçoit tous les messages. Lui envoyer uniquement le statut + fichiers, jamais le diff complet
