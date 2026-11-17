# Procedure de Test — Roue sur la tablette joueur et accélération (ROUE)

**Version** : 12.0.0 (LOT-1, retour QUALIF 12.0.0.2)
**Date** : 2026-11-17
**Testeur** : QA / utilisateur
**Issues** : #144
**Références** : maquette `_work/mockup/v12.0.0/ui/vplayer_roue__wheel_tap_stop.html`, `contracts/roue.md` §6 et §12, `_work/reports/planner-m5-wheel-tablet-20261117-090549.md`

## Prerequis

- [ ] Environnement : QUALIF (binaire Windows + firmware), serveur ≥ 12.0.0.5
- [ ] Matériel : 1 TV (`/tv`), 2 tablettes joueur (`/player`) liées à deux équipes (Rouges, Bleus), poste admin (`/game`)
- [ ] Données : question ROUE, catégorie Animaux, présentation « Chanson de Line Renaud » ; une seconde question avec présentation de 80 caractères ; une troisième sans présentation
- [ ] Réglage : forme de la roue `bandeau` puis `arc` (réglage de la question)

## Scenarios

### Scenario 1 — Roue visible, tap pour arrêter, autre équipe sans effet

**Validation** : AUTO — **Issues** : #144
*(équivalent automatisé : `RoueTv.spec.144.test.jsx`, `RoueTv.144.test.jsx`, `PlayerDisplay.roue.144.test.jsx` ; `cmd/server/roue_144_test.go` pour l'appui ignoré côté serveur)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer la manche, regarder les deux tablettes avant le lancement | La roue est affichée sur les deux, arrêtée, non tapable ; **aucun bouton STOP** | | |
| 2 | Admin : lancer la roue | Tablette des Rouges : cadre à leur couleur, « 👆 Touchez la roue pour l'arrêter ». Tablette des Bleus : « Les Rouges vont arrêter la roue », pas de cadre | | |
| 3 | Taper la roue sur la tablette des Bleus | Aucun effet | | |
| 4 | Taper la roue sur la tablette des Rouges | La roue freine et s'arrête ; un second tap pendant le freinage est sans effet | | |
| 5 | Poursuivre jusqu'à la phrase trouvée | La roue reste affichée sur les deux tablettes, y compris à la victoire ; le message de victoire est sur la ligne de message | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Indice court, long ou absent

**Validation** : HUMAIN — **Issues** : #144

**Objectif** : juger la lisibilité de l'indice (jamais coupé, jamais la phrase).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question avec présentation courte, tablette en paysage | Catégorie et présentation en jaune, sur leur ligne au-dessus du plateau ; la ligne du haut ne garde que équipe et cagnotte | | |
| 2 | Question avec présentation de 80 caractères | 2 lignes au plus, **aucun point de suspension**, texte entier lisible ; la roue reste exploitable | | |
| 3 | Question sans présentation | La catégorie seule, pas de ligne vide | | |
| 4 | Pour chaque cas : chercher la phrase à deviner | Elle n'apparaît jamais | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Paysage et portrait, bandeau et arc

**Validation** : HUMAIN — **Issues** : #144

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Paysage, forme bandeau | Le bandeau tient entier ; message et lettres proposées partagent une ligne | | |
| 2 | Paysage, forme arc | L'arc montre au moins une rangée de cases, pointeur visible, rien n'est coupé de façon gênante | | |
| 3 | Portrait, bandeau puis arc | Roue large et lisible ; repère « 👆 Touchez la roue » sous la roue quand elle est tapable | | |
| 4 | Pivoter la tablette pendant la rotation | La mise en page s'adapte, la roue continue de tourner sans à-coup, le tap fonctionne toujours | | |
| 5 | Toucher n'importe où sur la zone de la roue | L'arrêt est déclenché (toute la zone est la cible) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Accélération au lancement et freinage, TV et tablette

**Validation** : HUMAIN — **Issues** : #144
*(les calculs de vitesse et d'atterrissage sont couverts par `geometry.*.144.test.js` ; ce scénario juge le ressenti)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la roue, regarder la TV | La roue part de l'arrêt et **accélère** (1,5 s, contrat §6) jusqu'à la vitesse de croisière, sans à-coup | | |
| 2 | Idem sur la tablette | Même comportement | | |
| 3 | Laisser tourner puis arrêter | Le freinage démarre à la vitesse de croisière, sans saut de vitesse ni d'angle | | |
| 4 | Relancer et arrêter **pendant** la montée (tap < 1 s) | Le freinage part de la vitesse courante, sans saut, et atterrit sur la case tirée | | |
| 5 | Pendant la rotation, provoquer une mise à jour de l'état (ex. un buzz d'une autre équipe, une cagnotte modifiée) | La roue ne repart pas de zéro, pas de nouvelle montée | | |
| 6 | Recharger la page de la tablette (ou la TV) pendant la rotation, après la montée | L'écran qui se reconnecte part directement à la vitesse de croisière (pas de nouvelle montée) | |  |
| 7 | Lancer et laisser tourner jusqu'à la durée maximale (10 s par défaut) | La durée maximale est comptée montée comprise : l'arrêt automatique survient 10 s après le lancement | | |
| 8 | Manche RAFALE : lancer la roue de l'alphabet | Comportement inchangé (pas de montée) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — Même case à l'arrêt sur la TV et la tablette

**Validation** : HUMAIN — **Issues** : #144

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer, arrêter par le tap, comparer TV et tablette à l'arrêt | Même case sous le pointeur, au même moment (à la latence réseau près) ; l'annonce de la case est identique | | |
| 2 | Répéter 5 fois, dont BANQUEROUTE si elle tombe | Toujours la même case sur les deux écrans | | |
| 3 | Pendant la rotation | Les cases défilantes peuvent différer entre TV et tablette (non synchronisées par contrat) : ce n'est pas une anomalie | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Criteres de Validation

- [ ] Aucun bouton STOP sur la tablette ; la roue est la seule cible du tap
- [ ] L'autre équipe ne peut jamais arrêter la roue
- [ ] L'indice est entier et la phrase n'apparaît jamais
- [ ] Accélération et freinage sans à-coup, TV et tablette
- [ ] Non-régression : partie ROUE (`roue-partie-144.md`) et RAFALE

## Notes QA

Les scénarios 2 à 5 sont à jouer sur appareil réel : le rendu visuel de la maquette n'avait pas pu être contrôlé.
