# Procedure de Test — Case BONUS de la Roue de la Fortune

**Version** : 12.1.0 (LOT-1.1)
**Date** : 2026-11-21
**Testeur** : QA (scenarios AUTO) · utilisateur (scenarios HUMAIN)
**Issue** : #308 — Case BONUS
**Références** : `contracts/roue.md` §15.2, §15.3, §15.4 ; maquettes `docs/mockup/v12.1.0/ui/{tv_roue_started,admin_game_roue,anim_roue,admin_question_editor_roue}__vowels_bonus.html`

> Le cadeau d'une case BONUS est géré **hors système** par l'animateur : le serveur ne compte aucun point.
> Les scénarios AUTO sont couverts par des tests automatisés (`*_308_test.go`, `*.308.test.jsx`) que `qa` joue ;
> les scénarios HUMAIN exigent un jugement visuel ou sonore.

## Prerequis

- [ ] Environnement : LOCAL ou QUALIF, serveur BuzzControl 12.1.0.x
- [ ] Données : une question ROUE avec phrase valide, 2 équipes avec buzzers, la roue par défaut (1000 / 50 / 2 / 1)
- [ ] Accès : admin (`/questions`, `/game`), animateur (tablette), TV (`/tv`), une tablette joueur

## Scenarios

### Scenario 1 — 0 BONUS : la roue est celle de v12.0

**Validation** : AUTO — **Issues** : #308

**Objectif** : Une question sans case BONUS (champ absent ou 0) donne exactement la roue de v12.0.
**Tests automatisés** : `internal/game/roue_bonus_308_test.go` (`ZeroBonusWheelIsIdenticalToV120`), `internal/server/roue_bonus_308_test.go` (`WheelPreview_AbsentBonusCountKeepsV120Wheel`, `Upload_AbsentFieldMeansZeroAndIsNotWritten`).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer les tests ciblés `go test ./internal/game ./internal/server -run Bonus308` | Tous verts | | |
| 2 | Relire une question ROUE v12.0 dans l'éditeur | Compteur « Cases BONUS » à 0 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Nombre de cases BONUS : 0 à 4, refus au-delà, limite de 64 cases

**Validation** : AUTO — **Issues** : #308

**Objectif** : Le serveur accepte 0 à 4 BONUS, refuse le reste, et ne dépasse jamais 64 cases.
**Tests automatisés** : `ExactCountAndNothingDisplaced`, `ReducedAtStartToFitIn64Cases`, `Upload_InvalidCountRefused`, `Upload_CountThatWouldExceed64CasesRefused`, `WheelPreview_CountsAndKinds`, `WheelPreview_InvalidBonusCountRefused`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer `go test ./internal/game ./internal/server -run Bonus308` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Arrêt sur BONUS : rien n'est modifié, la même équipe rejoue

**Validation** : AUTO — **Issues** : #308

**Objectif** : Fin de freinage sur une case BONUS : aucune cagnotte, aucun score, aucune lettre ; la même équipe est en `TURN` (étape 1) ; un seul événement `BONUS`.
**Tests automatisés** : `StopOnBonusChangesNothingAndSameTeamPlaysAgain` (2 modes), `StopOnBonusSolo`, `AfterBonusNoLetterNoEndOfTurnButSpinAndPassWork`, `PassHandAfterBonusKeepsPotsAndMovesHand`, `BonusTwiceInARow`, `OtherCasesKeepV120Behaviour`.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer `go test ./internal/game -race -run Bonus308` | Tous verts | | |
| 2 | Lancer `go test ./cmd/server -run Bonus308` | Câblage de la notice BONUS vert (cue « bonne réponse », comète, ni flash ni pulsation) | | |
| 3 | Lancer `vitest run src/components/roue/RoueBonus.308.test.jsx src/pages/QuestionsPage.roue.308.test.jsx` | Tous verts | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — L'éditeur : compteur de cases BONUS et aperçu de la roue

**Validation** : HUMAIN — **Issues** : #308

**Objectif** : Vérifier que l'éditeur est conforme à la maquette `admin_question_editor__roue_vowels_bonus.html` et que l'aperçu est lisible.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `/questions`, type **Roue**, nouvelle question | Section « 🎁 Cases BONUS » avec compteur à 0 ; « − » grisé ; pas d'aperçu de roue | | |
| 2 | Cliquer « + » trois fois | Compteur 3 ; aperçu de la roue avec 3 cases dorées « 🎁 BONUS » réparties parmi les valeurs | | |
| 3 | Cliquer « + » une 4ᵉ puis une 5ᵉ fois | Plafonné à 4 ; « + » grisé | | |
| 4 | Comparer à la maquette | Disposition, libellés (« 0 à 4 — ajoutées aux cases des Paramètres… ») et dorure conformes | | |
| 5 | Saisir une phrase, **Ajouter**, rouvrir la question | Compteur à la valeur choisie | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 5 — TV et tablette : bandeau « 🎁 BONUS ! » sur un arrêt BONUS

**Validation** : HUMAIN — **Issues** : #308

**Objectif** : Juger sur écran réel le rendu, la lisibilité et le caractère statique du bandeau.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Question avec 4 BONUS, démarrer, lancer la roue ; la roue s'arrête sur une case BONUS (relancer jusqu'à en obtenir une) | La case dorée « 🎁 BONUS » est sous le pointeur | | |
| 2 | Regarder la TV | Bandeau « 🎁 BONUS ! » lisible de loin, aucun défilement ; cagnottes inchangées ; la phrase reste cachée | | |
| 3 | Regarder la tablette joueur de l'équipe | Même bandeau, annonce « vous rejouez » | | |
| 4 | Comparer à la maquette TV | Conforme | | |
| 5 | Relancer la roue | Le bandeau disparaît dès le geste suivant | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 6 — Son et lumières d'un arrêt BONUS

**Validation** : HUMAIN — **Issues** : #308

**Objectif** : Vérifier à l'oreille et à l'œil l'effet matériel.

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Obtenir un arrêt sur BONUS (sons activés) | Son « bonne réponse » joué une seule fois | | |
| 2 | Regarder les buzzers de l'équipe | Comète lumineuse (effet de gain) sur ses buzzers | | |
| 3 | Regarder la page Scores et l'ambiance | Aucun flash de score, aucune pulsation d'ambiance, scores inchangés | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 7 — Admin et animateur : l'équipe garde la main

**Validation** : HUMAIN — **Issues** : #308

**Objectif** : Juger que l'écran de l'admin et celui de l'animateur guident l'opérateur après un BONUS (remise du cadeau, relance).

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Après un arrêt BONUS, regarder l'admin (`/game`) | Bandeau « 🎁 BONUS ! — les <équipe> gardent la main… » ; « Lancer la roue » actif ; « Passer la main » disponible ; aucune consonne à proposer | | |
| 2 | Regarder la tablette animateur | Bandeau « 🎁 BONUS ! » ; seul « Lancer la roue » actif | | |
| 3 | Attribuer à la main un point à l'équipe (page Scores) | Les points passent par l'attribution manuelle existante | | |
| 4 | Cliquer « Lancer la roue » | La même équipe rejoue | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Criteres de Validation

- [ ] Les scénarios AUTO passent (rejoués par `qa`)
- [ ] Les scénarios HUMAIN passent sur QUALIF
- [ ] Aucune régression : tests ROUE v12.0 (`*_144_*`, `*_145_*`) verts

## Notes QA

Les scénarios sur les voyelles (#147) arrivent avec LOT-1.2.
