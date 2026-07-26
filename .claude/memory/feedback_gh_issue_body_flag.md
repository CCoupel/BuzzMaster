---
name: gh issue create — ne jamais passer --body deux fois
description: Un second --body écrase silencieusement le premier (issue créée avec un corps vide) — toujours vérifier après création
type: feedback
---

Pendant la création d'issues de backlog, `gh issue create ... --body "<texte>" --body ""` (deuxième `--body` ajouté par erreur) a créé l'issue avec un corps **vide**, sans erreur ni avertissement. Reproduit deux fois dans la même session (issues #111 et #115) avant d'être corrigé via `gh issue edit --body`.

**Why:** `gh` ne signale aucune erreur quand `--body` est passé plusieurs fois — le dernier gagne silencieusement. Une issue de backlog avec un corps vide perd tout le contexte utile (repro, justification, liens) sans que ça se voie dans la sortie de la commande (qui affiche juste l'URL, comme en cas de succès normal).

**How to apply:**
- Ne jamais dupliquer le flag `--body` dans un `gh issue create` (ni dans un script qui enchaîne plusieurs créations).
- Après toute création d'issue via `gh issue create --body "..."`, vérifier rapidement avec `gh issue view <numero> --json body --jq '.body | length'` que le corps n'est pas vide, surtout en création par lot (plusieurs issues d'affilée).
