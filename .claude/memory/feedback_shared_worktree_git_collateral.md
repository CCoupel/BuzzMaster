---
name: shared-worktree-git-collateral
description: Teammates share one working directory — a branch switch or git mishap by one agent silently reverts another's uncommitted files
metadata:
  type: feedback
---

Tous les teammates (dev-backend, dev-frontend, deployer, code-reviewer, qa...) opèrent sur le
**même répertoire de travail** que `main`, pas des worktrees isolés. Un `git checkout`, un
`git stash`, ou un merge fait par un agent affecte immédiatement tous les autres — y compris
`main` lui-même.

**Symptôme observé à répétition** : après qu'un teammate fasse un `git checkout <autre-branche>`
(pour vérifier un état, tester un fix, etc.), `.claude/workflow-state.json` (fichier de suivi de
`main`, jamais commité systématiquement) revient à son ancien contenu committé sur cette branche —
perdant les mises à jour en cours. Un système-reminder signale alors "modifié par l'utilisateur",
ce qui est trompeur : c'est un artefact de branch switch, pas une édition humaine.

Un autre cas : un `code-reviewer` a fait une manipulation `git stash` pendant une vérification
manuelle et a accidentellement appliqué un stash pré-existant sans rapport, créant des conflits
hors périmètre — corrigé par l'agent lui-même via reset, mais aurait pu passer inaperçu.

**Comment appliquer** :
- Ne jamais faire confiance au contenu de `workflow-state.json` sans le relire après qu'un
  teammate ait pu changer de branche — reconstruire son contenu depuis le contexte de la
  conversation si besoin, ne pas supposer que c'est une intervention utilisateur volontaire.
- Après tout incident git rapporté par un agent (stash, checkout, merge), vérifier
  indépendamment l'état réel (`git status`, `git log`, `git diff HEAD`) avant de continuer —
  ne pas se contenter du rapport de l'agent.
- Si le contenu semble être resté à une ancienne branche (ex: `branch: bugfix/xxx` alors qu'on est
  censé être ailleurs), c'est le signal qu'un checkout a eu lieu entre-temps.
