---
name: feedback_marketing_gate_iterations
description: Pattern GATE validé pour les changements du site marketing — maquette HTML publiée en Artifact, itérée jusqu'à validation explicite, avant tout commit sur gh-pages
metadata:
  type: feedback
---

Pour toute demande de réorganisation/refonte de contenu sur le site marketing (`gh-pages`), suivre le
pattern GATE avant d'implémenter réellement :

1. Spawn `marketing-release` (agent ponctuel), handoff écrit dans `_work/handoff/task-marketing-release-<ts>.md`.
2. L'agent produit une maquette HTML autonome (`_work/reports/marketing-mockup-<ts>.html`) + un rapport
   d'audit — **aucun commit sur `gh-pages`** à ce stade.
3. Le teamleader **lit** systématiquement le fichier HTML de la maquette (obligatoire avant publication —
   voir contrainte de l'outil Artifact sur les fichiers non écrits par soi-même) puis le publie via
   `Artifact` pour que l'utilisateur le voie visuellement.
4. Itérer (v2, v3, v4...) sur retours utilisateur — chaque itération = nouveau handoff + nouvelle
   maquette + nouvel Artifact, jusqu'à validation explicite ("lance le site final" ou équivalent).
5. Seulement après validation explicite : nouveau handoff d'implémentation réelle (commit + push sur
   `gh-pages`), avec portée strictement limitée à ce qui a été validé dans la maquette.

**Pourquoi** : sur la session du 2026-08-02, ce pattern a permis 5 itérations de maquette (ajustements
d'images, déplacements de cartes, fusion de cartes, carrousel) sans jamais toucher au site en ligne, puis
une implémentation propre en un seul commit — cohérent avec le pattern GATE déjà validé pour les mockups
de features produit (voir `now.md` du 2026-08-01, Option C).

**Comment appliquer** : ne jamais laisser `marketing-release` committer sur `gh-pages` avant un feu vert
explicite de l'utilisateur sur la maquette. Même après implémentation, rester en mode correction rapide
(nouveau handoff ciblé, pas de nouvelle maquette) si l'utilisateur signale un bug visuel post-publication —
voir [feedback_marketing_image_verification.md](feedback_marketing_image_verification.md) pour le bug
concret rencontré à ce stade. Fermer l'agent (`TaskStop`) après chaque DONE puisque c'est un agent
ponctuel, et le re-spawner à la demande suivante (le handoff écrit fait office de mémoire de tâche).
