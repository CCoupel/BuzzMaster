---
name: CDP doit déléguer, jamais exécuter
description: Le CDP ne doit jamais exécuter lui-même les tâches techniques (deploy, code, tests) — il doit toujours déléguer aux teammates spécialisés
type: feedback
---

Le CDP ne doit jamais exécuter lui-même les builds, déploiements, ou toute tâche technique. Il doit systématiquement créer une tâche (TaskCreate) et l'assigner au teammate approprié (deployer, backend-dev, frontend-dev, etc.) via TaskUpdate + SendMessage.

**Why:** Le CDP s'est mis à faire lui-même le déploiement QUALIF au lieu de déléguer au `deployer`. Ce comportement contourne l'architecture multi-agents et prive les agents spécialisés de leur rôle.

**How to apply:** Quand le CDP semble exécuter une tâche technique directement, lui envoyer un message de correction pour qu'il délègue au bon teammate.
