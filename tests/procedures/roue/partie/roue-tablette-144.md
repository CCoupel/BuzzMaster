# Procedure de Test — Roue sur la tablette joueur et accélération (ROUE)

**Version** : 12.0.0 (LOT-1, retours QUALIF 12.0.0.2 à 12.0.0.5)
**Date** : 2026-11-17
**Testeur** : QA / utilisateur
**Issues** : #144
**Références** : maquette `_work/mockup/v12.0.0/ui/vplayer_roue__wheel_tap_stop.html`, `contracts/roue.md` §6 et §12, `_work/reports/planner-m5-wheel-tablet-20261117-090549.md`

## Prerequis

- [ ] Environnement : QUALIF (binaire Windows + firmware), serveur ≥ 12.0.0.7 (montée en 2 tours)
- [ ] Matériel : 1 TV (`/tv`), 2 tablettes joueur (`/player`) liées à deux équipes (Rouges, Bleus), poste admin (`/game`)
- [ ] Données : question ROUE, catégorie Animaux, présentation « Chanson de Line Renaud » ; une seconde question avec présentation de 80 caractères ; une troisième sans présentation
- [ ] Réglage : forme de la roue `bandeau` puis `arc` (réglage de la question)

## Scenarios

### Scenario 1 — Roue visible, tap pour arrêter à pleine vitesse, autre équipe sans effet

**Validation** : AUTO — **Issues** : #144
*(équivalent automatisé : `RoueTv.spec.144.test.jsx`, `RoueTv.144.test.jsx`, `PlayerDisplay.roue.144.test.jsx` ; `internal/game/roue_spinup_144_test.go` et `cmd/server/roue_spinup_144_test.go` pour l'appui ignoré et l'origine de ROUE_STOP)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer la manche, regarder les deux tablettes avant le lancement | La roue est affichée sur les deux, arrêtée, non tapable ; **aucun bouton STOP** | | |
| 2 | Admin : lancer la roue ; **pendant les 10 s de montée**, taper la roue de l'équipe qui a la main | Aucun effet : la roue n'est pas tapable (pas de cadre allumé), message « La roue prend de la vitesse… », aucun `BUTTON` envoyé | | |
| 3 | Pendant la montée, appuyer sur le buzzer de l'équipe qui a la main | Aucun effet, et l'appui n'est pas mémorisé : à pleine vitesse la roue tourne toujours | | |
| 4 | À pleine vitesse (≈ 10 s après le lancer) : tablette des Rouges | Cadre à leur couleur, « 👆 Touchez la roue pour l'arrêter », barre de durée maximale pleine | | |
| 5 | Même moment, tablette des Bleus | « Les Rouges vont arrêter la roue », pas de cadre ; un tap n'a aucun effet | | |
| 6 | Taper la roue sur la tablette des Rouges | La roue freine et s'arrête ; un second tap pendant le freinage est sans effet | | |
| 7 | Poursuivre jusqu'à la phrase trouvée | La roue reste affichée sur les deux tablettes, y compris à la victoire ; le message de victoire est sur la ligne de message | | |

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

### Scenario 4 — Montée de 10 s (2 tours), durée maximale ensuite, TV et tablette

**Validation** : HUMAIN — **Issues** : #144
*(les calculs de montée, de vitesse et d'atterrissage sont couverts par `geometry.accel.144.test.js` et `Wheel.accel*.144.test.jsx` ; ce scénario juge le ressenti)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la roue, regarder la TV | La roue part de l'arrêt et accélère régulièrement : **2 tours** pour atteindre la vitesse de croisière, soit ≈ 10 s (période de 2,5 s par défaut) ; l'annonce TV parle de montée en vitesse, pas de « buzzez » | | |
| 2 | Idem sur la tablette | Même comportement, même durée (± la latence) | | |
| 3 | À la fin de la montée | Pas d'à-coup ; la TV annonce « buzzez pour arrêter la roue » ; la tablette de l'équipe devient tapable ; la barre de durée maximale apparaît **pleine** et se décompte | | |
| 4 | Ne rien faire à pleine vitesse | Arrêt automatique **10 s après** la pleine vitesse (20 s au total par défaut) | | |
| 5 | Arrêter par un tap | Le freinage part de la vitesse de croisière, sans saut de vitesse ni d'angle | | |
| 6 | Changer `spin_period_ms` (1 s puis 6 s) et relancer | La montée vaut 4 × la période (4 s puis 24 s) ; la durée maximale reste comptée depuis la pleine vitesse | | |
| 7 | Pendant la rotation, provoquer une mise à jour d'état (cagnotte modifiée, autre équipe qui buzz) | La roue ne repart pas de zéro, pas de nouvelle montée | | |
| 8 | Recharger la tablette (ou la TV) pendant la rotation | L'écran part directement à la vitesse de croisière ; pendant la montée il est non tapable, à pleine vitesse il l'est | | |
| 9 | Mettre la partie en pause pendant la montée, attendre, reprendre | La montée reprend pour sa durée restante, la roue n'est pas tapable avant | | |
| 10 | Manche RAFALE : lancer la roue de l'alphabet | Comportement inchangé (pas de montée) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 bis — « Forcer l'arrêt » pendant la montée : admin accepté, animateur refusé

**Validation** : AUTO — **Issues** : #144
*(équivalent automatisé : `roue_spinup_144_test.go` (moteur), `cmd/server/roue_spinup_144_test.go` (WebSocket), `RoueMontee.spec.144.test.jsx` (boutons))*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la roue ; pendant la montée, regarder « Forcer l'arrêt » sur l'admin | Bouton actif | | |
| 2 | Idem sur la tablette animateur | Bouton grisé | | |
| 3 | Cliquer « Forcer l'arrêt » sur l'admin | La roue freine **depuis la vitesse atteinte** (sans saut à la vitesse de croisière), s'arrête sur la case tirée ; plus de passage à pleine vitesse ensuite | | |
| 4 | Relancer ; forcer l'envoi de ROUE_STOP depuis l'animateur pendant la montée (course) | Message « la roue prend de la vitesse » (refus NOT_FULL_SPEED) à l'animateur seul, état inchangé | | |
| 5 | À pleine vitesse, « Forcer l'arrêt » de l'animateur | Accepté | | |

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

- [ ] Aucun bouton STOP sur la tablette ; la roue est la seule cible du tap, et seulement à pleine vitesse
- [ ] L'autre équipe ne peut jamais arrêter la roue
- [ ] L'indice est entier et la phrase n'apparaît jamais
- [ ] Montée de 10 s (2 tours) puis durée maximale, freinage sans à-coup, TV et tablette
- [ ] Non-régression : partie ROUE (`roue-partie-144.md`) et RAFALE

## Notes QA

Les scénarios 2 à 5 sont à jouer sur appareil réel : le rendu visuel de la maquette n'avait pas pu être contrôlé.
