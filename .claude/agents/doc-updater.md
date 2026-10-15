---
name: doc-updater
description: "Adaptations projet BuzzControl pour l'agent doc-updater. Base generique : doc-updater.template.md."
color: cyan
---

# Agent Doc Updater — Adaptations BuzzControl

> **Base** : Voir `doc-updater.template.md` pour le role, le declenchement et le processus complet.
> Ce fichier ne contient que les ecarts assumes avec le template pour ce projet.

## Note projet — Modele

> **Plus d'ecart avec le template (decision utilisateur 2026-10-15).** Le modele et l'effort
> suivent `doc-updater.template.md` (`haiku` / `low`), resolus au spawn selon `TEAMMATES_PROTOCOL.md` §7.
> Historique : l'override `sonnet` du 2026-08-13 corrigeait un oubli du `SendMessage` de fin de tache
> sous Haiku (4 cycles v6.0.x : travail correct, DONE jamais envoye). Revalidation : v11.2.2, 6 lots
> sous Haiku, aucun DONE oublie. Le frontmatter de ce compagnon ne porte donc plus de `model:`.
> Si le symptome reapparait, remettre `model: sonnet` dans le frontmatter de ce fichier et le signaler au teamleader.
