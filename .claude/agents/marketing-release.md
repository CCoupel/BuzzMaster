# Marketing Release — adaptations projet BuzzControl

> Compagnon projet de `marketing-release.template.md`. Contient uniquement la regle du badge « Nouveau ».
> **Ce compagnon PRIME sur la regle « badges.js / pose uniquement sur nouveau X » du template** : ce mecanisme
> (`<meta name="current-major">`, `assets/badges.js`) n'existe pas sur le site live gh-pages et ne doit pas etre introduit.

## Badge « Nouveau » (site marketing)

Le statut « Nouveau » appartient a la version **MAJEURE courante Vx**.

- Tant que Vx est « Nouveau », toute nouvelle fonctionnalite livree dans un Vx.y (y >= 1) recoit aussi le badge
  `Nouveau vX.Y.Z` (`badge badge-new` + `feature-card--new` sur la carte), avec la version de sa premiere
  apparition, figee ensuite.
- Quand une nouvelle majeure Vx+1 devient courante, Vx perd son statut : retirer **ensemble** le badge « Nouveau »
  de toutes les cartes Vx et Vx.y (les retrograder en badge neutre `vX.Y.Z`).
- Sur le site live il n'y a ni script ni date : le retrait est **manuel**, a faire lors de la premiere
  republication reelle apres le changement de majeure. Ne jamais republier uniquement pour retirer un badge
  (un `RIEN A PUBLIER` reste un arret net).
- Verifier l'etat reel via l'historique gh-pages (`git log -S'Nouveau v' origin/gh-pages -- index.html`) plutot
  que par deduction.

### Precision historique (constatee sur gh-pages)

Des badges « Nouveau » ont deja ete poses sur des versions x.y (v5.5.1, v5.9.0, v6.1.0, v6.1.2, v6.3.0). Leur
retrait a ete manuel et parfois tardif (les badges v6.x ont survecu a v7, v8 et v9 avant d'etre retires a l'arrivee
de v10 ; v8.0.0 et v10.0.0 retires/retrogrades a l'arrivee de v11). La regle ci-dessus est la reference : ne pas la
deduire de cet historique.
