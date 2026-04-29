---
description: "Demande a chaque teammate son statut de progression et affiche un tableau de synthese. A utiliser pendant un workflow actif pour eviter l'effet tunnel. Peut etre invoquee par l'utilisateur directement, le teamleader ou le CDP."
---

# /progression — Tableau de Bord de l'Equipe

Cette commande interroge tous les agents actifs de la team et compile un tableau de synthese
de l'avancement du workflow en cours.

**Peut etre invoquee par :**
- L'utilisateur directement (depuis Claude Code)
- Le **teamleader** — pour superviser l'ensemble de la team qu'il a spawnee
- Le **CDP** — pour reporter la progression a l'utilisateur

Dans tous les cas, c'est l'agent qui recoit la commande qui orchestre les `SendMessage`
vers les teammates et compile le tableau.

## Execution

Le CDP envoie une demande de statut a chaque agent actif **en parallele** (un seul message) :

```
SendMessage({ to: "planner",       content: "Donne-moi ton statut de progression." })
SendMessage({ to: "dev-backend",   content: "Donne-moi ton statut de progression." })
SendMessage({ to: "dev-frontend",  content: "Donne-moi ton statut de progression." })
SendMessage({ to: "dev-firmware",  content: "Donne-moi ton statut de progression." })
SendMessage({ to: "code-reviewer", content: "Donne-moi ton statut de progression." })
SendMessage({ to: "qa",            content: "Donne-moi ton statut de progression." })
SendMessage({ to: "doc-updater",   content: "Donne-moi ton statut de progression." })
SendMessage({ to: "deployer",      content: "Donne-moi ton statut de progression." })
SendMessage({ to: "infra",         content: "Donne-moi ton statut de progression." })
SendMessage({ to: "security",      content: "Donne-moi ton statut de progression." })
```

> N'envoyer qu'aux agents effectivement spawnes dans la team — ignorer les absents.

## Rapport de Synthese

Une fois toutes les reponses recues, afficher :

```markdown
## Progression — {PROJECT_NAME}
**Workflow** : [FEATURE|BUGFIX|HOTFIX|REFACTOR]   **Phase actuelle** : [Phase X — Nom]

| ID | Tache | Agent | Status | Dependance |
|----|-------|-------|--------|------------|
| 01 | [nom tache] | [agent] | ✅ Termine | — |
| 02 | [nom tache] | [agent] | 🔄 En cours (X%) | — |
| 03 | [nom tache] | [agent] | ⏳ Attente dependance | tache-02 |
| 04 | [nom tache] | [agent] | 💬 Attente teammate | dev-backend |
| 05 | [nom tache] | [agent] | 👤 Attente validation | utilisateur |
| 06 | [nom tache] | [agent] | 🔴 Bloque | [raison technique] |

**Legende** : ✅ Termine | 🔄 En cours (X%) | ⏳ Attente dependance | 💬 Attente teammate | 👤 Attente validation | 🔴 Bloque

**Points d'attention** :
- [lister les blocages ou retards identifies, ou "RAS"]
```

## Regles

- Si un agent ne repond pas dans un delai raisonnable, le noter `⚠️ Sans reponse` dans le tableau
  et demander au teamleader de le reveiller ou de le respawner.
- Ne pas relancer le workflow — cette commande est en lecture seule, elle observe sans perturber.
- Peut etre invoquee a tout moment pendant un workflow actif.
