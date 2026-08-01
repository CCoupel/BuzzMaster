---
name: version-reconciliation
description: Parallel unmerged branches naturally diverge on version numbers during QUALIF cycles — don't fight it, reconcile to one final number only at actual PROD merge
metadata:
  type: feedback
---

Quand plusieurs issues du même milestone sont développées en parallèle sur des branches
distinctes (ou empilées), chaque branche incrémente sa propre version pendant ses cycles QUALIF
(ex: une pile VJoueur atteint 5.9.3 pendant qu'une feature séparée branchée depuis `main` n'est
qu'à 5.8.2). C'est normal et attendu — ces numéros sont provisoires, ils ne servent qu'à
distinguer les binaires QUALIF successifs pendant le développement.

**Ne pas essayer de garder les versions synchronisées entre branches en cours de développement.**
La réconciliation se fait une seule fois, au moment du merge PROD réel : fixer explicitement
`config.json` à la version finale décidée (généralement en demandant à l'utilisateur, ex: reset Z
à 0 pour une release de milestone "vX.Y.0"), résoudre le conflit sur `CHANGELOG.md` en fusionnant
les sections par date plutôt que par numéro de version.

**Comment appliquer** :
- Pendant le développement (dispatch aux agents), laisser chaque branche incrémenter sa version
  indépendamment (Z routine) sans chercher à la faire correspondre à une autre pile en cours.
- Au moment de préparer un merge PROD groupant plusieurs branches, demander explicitement à
  l'utilisateur la version finale si elle n'est pas évidente (reset vs incrément simple) plutôt
  que de deviner.
- Le premier ticket d'un nouveau milestone (vX.Y.x) doit bumper Y et reset Z dès son propre cycle
  de dev (pas seulement au merge final) — sinon la confusion s'accumule sur toute la durée du
  milestone.
