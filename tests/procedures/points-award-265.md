# Procédure de Test — #265 : attribution de points multi-équipes (feu d'artifice par lots)

**Version** : 11.2.1
**Date** : 2026-10-12
**Testeur** : utilisateur (QA n'exécute pas les procédures manuelles)
**Maquettes** : `docs/mockup/v11.2.1/ui/tv_points_award__multi_team.html`, `vplayer_points_award__own_team.html`

## Prérequis
- [ ] Environnement : QUALIF, TV réelle (`/tv`, vue de jeu), admin, au moins une tablette VJoueur par équipe testée
- [ ] Parties de test avec 6, 12 et 20 équipes (équipes fictives si besoin)
- [ ] Attribution groupée (admin : points à plusieurs équipes en une fois)

## Scénario 1 — 6 équipes (un lot) : fluidité
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Attribuer des points à 6 équipes d'un coup | 6 fusées à la couleur de chaque équipe, départs/arrivées aléatoires, zoom, « +X pts » en vert | | |
| 2 | Observer la fluidité sur la TV réelle | Pas de saccade visible, durée totale ≈ 3,2 s, tout disparaît ensuite | | |
| 3 | Vérifier le cadre | Rien ne sort de l'écran, aucun défilement, deux zooms ne se recouvrent jamais | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — 12 et 20 équipes (plusieurs lots)
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | 12 équipes | 2 lots de 6 l'un après l'autre (≈ 6,4 s), indicateur « Lot i/2 » en bas à droite | | |
| 2 | 20 équipes | 4 lots de 5 ; le 4ᵉ lot va plus vite (≈ 11,4 s au total) | | |
| 3 | Noms longs (« Les Turquoises de Montmartre ») | Tronqués par « … » sur UNE seule ligne | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 3 — Tablette : sa propre équipe seulement, calée sur la TV
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Attribution à 12 équipes ; regarder la tablette d'une équipe du 2ᵉ lot | Rien pendant le 1er lot, puis une fusée + « +X pts · pour votre équipe » au moment où son lot passe à la TV | | |
| 2 | Tablette d'une équipe qui ne gagne pas | Aucune animation, aucun confetti | | |
| 3 | Correction cachée (DISPLAY off) sur une équipe | Aucune animation, sur aucun écran | | |
| 4 | Reconnecter une tablette (rafraîchir la page) pendant une partie avec score déjà attribué | Aucun rejeu à la connexion | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 4 — Mouvement réduit
| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Activer « réduire les animations » sur la TV et la tablette, attribuer des points | Pas de fusée ni de zoom : cartes en fondu à leur case, même rythme | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Notes QA

