# Procedure de Test — Partie ROUE « Roue de la Fortune » (mécanique de jeu)

**Version** : 12.0.0 (LOT-1.2)
**Date** : 2026-11-16
**Testeur** : QA
**Issues** : #144 (mécanique), #145 (phrases)
**Références** : `contracts/roue.md` (rév. 6), `contracts/wheel-component.md` §5/§8 ; maquettes `docs/mockup/v12.0.0/ui/` (M1 TV, M2 admin, M4 réglage, M5 tablette, M7 animateur)

## Prerequis

- [ ] Environnement : QUALIF (binaire Windows + firmware), serveur v12.0.0
- [ ] Matériel : 1 écran TV (`/tv`), 3 buzzers réels connectés (3 équipes), 1 tablette joueur (`/player`, paysage puis portrait), 1 tablette animateur (`/anim`), poste admin (`/game`)
- [ ] Données : une question ROUE « Combien pour ce chien dans la vitrine » (catégorie Animaux, présentation « Chanson de Line Renaud »), 3 équipes, scores à 0
- [ ] Réglage de la roue : par défaut (1000 / pas 50 / 2 BANQUEROUTE / 1 TOUR PERDU = 22 cases)

## Scenarios

### Scenario 1 — Manche complète, mode TANT QUE JE GAGNE, « Seul le gagnant encaisse »

**Validation** : HUMAIN — **Issues** : #144

**Objectif** : Jouer les 7 étapes d'un tour de bout en bout et juger lisibilité et rythme sur la TV.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Admin : choisir les 3 équipes, passer en prêt, démarrer | TV : équipes en haut, plateau plein écran (lettres cachées), roue visible en permanence ; **aucune lettre ni phrase lisible** | | |
| 2 | Étape 1 « Lancer la roue » : cliquer Lancer | La roue défile sur TV et tablettes ; barre de durée maximale | | |
| 3 | Étape 2 : l'équipe qui a la main appuie sur son buzzer | La roue freine puis s'arrête sur une case ; la case est mise en avant, accent = couleur de l'équipe | | |
| 4 | Étape 3 bis : l'équipe annonce une consonne à voix haute | Rien ne change à l'écran | | |
| 5 | Étape 4 : saisir une consonne présente | Les cases de la consonne s'éclairent | | |
| 6 | Étape 5 : « Retourner les cases » | Les lettres se retournent une à une (≈ 0,6 s) puis apparaissent à la couleur de l'équipe | | |
| 7 | Étape 6 : l'équipe propose une réponse fausse ; cliquer « Équipe suivante » (étape 7) | Le gain du tour rejoint la cagnotte de l'équipe ; la main passe à l'équipe suivante | | |
| 8 | Rejouer jusqu'à ce qu'une équipe trouve : « Réponse validée » | Phrase entièrement visible ; la cagnotte de l'équipe gagnante est créditée à son score ; les autres perdent la leur | | |
| 9 | Arrêter la manche | Retour à l'état normal, scores corrects dans la page Scores | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Les trois arrêts de la roue

**Validation** : AUTO — **Issues** : #144
*(équivalent automatisé : `cmd/server/roue_144_test.go` — arrêt au buzz, « Forcer l'arrêt », durée maximale)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la roue, appuyer sur le buzzer **d'une autre équipe** | Aucun effet (appui ignoré) | | |
| 2 | Appuyer sur le buzzer de l'équipe qui a la main | Arrêt ; un **second appui** pendant le freinage est ignoré | | |
| 3 | Relancer, cliquer « Forcer l'arrêt » | Arrêt identique | | |
| 4 | Relancer et ne rien faire | La roue s'arrête d'elle-même à la durée maximale (10 s par défaut) | | |
| 5 | Relancer, appuyer sur le bouton de la **tablette** de l'équipe | Arrêt avant la durée maximale | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Lettre présente, absente, répétée, voyelle, BANQUEROUTE, TOUR PERDU

**Validation** : AUTO — **Issues** : #144
*(équivalent automatisé : `internal/game/roue_144_test.go`, `cmd/server/roue_144_test.go`)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Consonne **absente** de la phrase | Fin du tour (aucune lettre trouvée) ; la lettre figure dans la liste des lettres proposées (barrée) | | |
| 2 | Consonne **déjà proposée** | Traitée comme absente | | |
| 3 | Taper une **voyelle** (A, E, I, O, U, Y) | Message « voyelle non disponible », état inchangé, aucune pénalité | | |
| 4 | Case **BANQUEROUTE** | Cagnotte de l'équipe à 0, fin du tour | | |
| 5 | Case **TOUR PERDU** | Fin du tour, cagnotte conservée | | |
| 6 | Trouver toutes les consonnes | Le lancer n'est plus proposé : seules « Réponse validée » / fin de tour restent | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Rotation des trois modes et bouton conseillé

**Validation** : AUTO — **Issues** : #144
*(équivalent automatisé : table de la rotation §5.2 dans `internal/game/roue_144_test.go`)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | **SOLO** (1 équipe) : finir un tour | Seul « Tour suivant » est utilisable | | |
| 2 | **CHACUN SON TOUR** : lettre trouvée, puis lettre absente | « Équipe suivante » conseillé dans les deux cas | | |
| 3 | **TANT QUE JE GAGNE** : lettre trouvée | « Tour suivant » conseillé | | |
| 4 | **TANT QUE JE GAGNE** : lettre absente / BANQUEROUTE / TOUR PERDU | « Équipe suivante » conseillé | | |
| 5 | Dans chaque cas, choisir le bouton **non** conseillé | Il fonctionne (sauf « Équipe suivante » en SOLO) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — Règle de gains « Tout le monde encaisse »

**Validation** : AUTO — **Issues** : #144
*(équivalent automatisé : crédit `ALL_TEAMS` / `WINNER_ONLY` dans `internal/game/roue_144_test.go`)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question réglée « Tout le monde encaisse », 3 équipes avec cagnottes 300 / 100 / 0 | — | | |
| 2 | Valider la réponse pour l'équipe de 300 | Scores : +300, +100, +0 ; pas d'événement pour la cagnotte à 0 | | |
| 3 | Dévoiler la phrase (admin) au lieu de valider | Aucun crédit ; plateau entièrement visible ; pas de gagnant | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Réglage de la roue (maquette M4)

**Validation** : HUMAIN — **Issues** : #144

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Paramètres → carte « Roue » : régler maximum 100, pas 30 | Message « Pas ajusté à 25 », la valeur du champ est remplacée ; aperçu de la roue mis à jour | | |
| 2 | Régler pas = maximum, ou plus de 64 cases | Refus avec message lisible, rien d'enregistré | | |
| 3 | Régler 100 / 10 / 1 BANQUEROUTE / 1 TOUR PERDU | Aperçu : `100 BANQ 90 10 80 PERDU 20 70 30 60 40` (TOUR PERDU à mi-tour) | | |
| 4 | Cliquer « Roue par défaut » | 1000 / 50 / 2 / 1, 22 cases | | |
| 5 | Enregistrer, démarrer une manche ROUE | La roue de la manche reflète le réglage ; modifier le réglage pendant la manche ne change pas la manche en cours | | |
| 6 | Régler la durée maximale de rotation à 3 s | La roue s'arrête seule au bout de 3 s | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 7 — Écran TV et tablettes (rendu, forme de la roue, lettres proposées)

**Validation** : HUMAIN — **Issues** : #144

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question en forme **Bandeau**, lettres proposées activées | TV : bandeau en bas, plateau le plus grand, ligne « Lettres proposées » (absentes barrées) ; **aucun scroll** | | |
| 2 | Question en forme **Arc**, lettres proposées désactivées | TV : haut d'une grande roue ; pas de ligne de lettres proposées | | |
| 3 | Tablette **paysage** puis **portrait** | Pleine largeur, lisible, roue visible | | |
| 4 | Recharger la TV pendant le freinage | La roue se place directement sur la case tirée | | |
| 5 | Recharger la TV pendant le retournement | Les lettres sont déjà affichées | | |
| 6 | Mettre en pause pendant le freinage, puis reprendre | La minuterie reprend, la roue s'arrête à la case tirée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 8 — Tablette animateur et double commande

**Validation** : HUMAIN — **Issues** : #144

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur la tablette animateur : parcourir les 7 boutons d'étape | Chaque bouton agit comme son équivalent admin ; clavier de consonnes colorées par équipe | | |
| 2 | Admin **et** animateur cliquent « Lancer » en même temps | Une seule application ; l'autre est refusé sans effet visible gênant | | |
| 3 | L'animateur cherche « Passer la main », « Dévoiler », choix des équipes | Non proposés à l'animateur (admin seulement) | | |
| 4 | L'animateur lit la phrase | Visible sur la tablette animateur ; **jamais** sur TV ni tablette joueur | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 9 — Robustesse : arrêt, nouvelle partie, renommage d'équipe

**Validation** : AUTO — **Issues** : #144
*(équivalent automatisé : `cmd/server/roue_144_test.go` — STOP / NEW_GAME / pause / renommage / suppression d'équipe)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Arrêter la manche pendant le freinage | Minuteries annulées, rien ne se produit après l'arrêt | | |
| 2 | Lancer une nouvelle partie pendant une rotation | Aucun résidu ROUE dans l'état | | |
| 3 | Renommer ou supprimer une équipe pendant la manche | Cagnottes, main et lettres proposées suivent | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Une manche complète se joue sans blocage, sur TV, tablettes et animateur
- [ ] La phrase n'apparaît jamais sur TV ni tablette avant la résolution
- [ ] Les trois modes et les deux règles de gains donnent les résultats attendus
- [ ] Le réglage de la roue (pas corrigé, refus, défaut) est conforme au contrat
- [ ] Aucune régression sur RAFALE (roue de lettres) ni sur les autres modes

## Notes QA

[Espace pour observations]
