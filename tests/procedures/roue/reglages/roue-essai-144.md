# Procedure de Test — Essai de la roue (bouton « Tester » des réglages)

**Version** : 12.0.0 (LOT-1, retour QUALIF 12.0.0.8)
**Date** : 2026-11-17
**Testeur** : QA / utilisateur
**Issues** : #144
**Références** : `contracts/roue.md` §10.2 et §11.3, `_work/reports/planner-contract-montee1tour-20261117-111046.md`

## Prerequis

- [ ] Environnement : QUALIF, serveur ≥ 12.0.0.9
- [ ] Matériel : poste admin (page des réglages, carte « Roue de la Fortune »), TV `/tv` ouverte (pour vérifier l'absence d'effet)
- [ ] Aucune manche en cours (puis, scénario 4, une manche ROUE en cours)

## Scenarios

### Scenario 1 — Déroulement de l'essai avec les valeurs non enregistrées

**Validation** : HUMAIN — **Issues** : #144
*(équivalent automatisé du déroulement et des temps : `WheelConfigCard.tester.144.test.jsx`, `roue_wheel_timing_144_test.go` ; ce scénario juge le rendu)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir la carte, saisir sans enregistrer : tour 3 s, durée maximale 8 s, freinage 3 s | L'aperçu se met à jour ; « Tester » est actif | | |
| 2 | Cliquer « Tester » | « Montée… » ; « Tester » et « Arrêter » grisés ; le bandeau « Sur la TV » **et** la vue d'ensemble circulaire partent de l'arrêt et accélèrent ensemble (6 s = 2 × 3 s) | | |
| 3 | À la fin de la montée | « Pleine vitesse » ; « Arrêter » actif ; temps restant décompté depuis la pleine vitesse | | |
| 4 | Cliquer « Arrêter » | « Freinage » ; les deux aperçus freinent sans à-coup et s'arrêtent **sur la même case au même instant** (≈ 3 s) | | |
| 5 | Après l'arrêt | « Arrêt sur <valeur> » égale à la case sous le pointeur ; « Tester » actif, « Arrêter » grisé | | |
| 6 | Relancer et ne pas cliquer « Arrêter » | Le freinage démarre tout seul 8 s après la pleine vitesse | | |
| 7 | Répéter l'essai 5 fois | Les cases d'arrêt varient (tirage au hasard) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 2 — Valeurs hors bornes : la page affiche ce que le jeu utiliserait

**Validation** : AUTO — **Issues** : #144
*(équivalent automatisé : `internal/server/roue_wheel_timing_144_test.go`, `WheelConfigCard.tester.144.test.jsx`)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Saisir un tour de 0,1 s puis de 60 s, une durée maximale de 1 s | Durées ramenées à leurs bornes (tour 1 à 6 s, durée max 3 à 30 s, freinage 1,5 à 8 s) ; l'essai utilise ces valeurs et la montée vaut 2 × le tour | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 3 — Interruption

**Validation** : HUMAIN — **Issues** : #144

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Pendant la montée, modifier « Points maximum » | L'essai s'interrompt : plus de libellé d'état, aperçus fixes, « Tester » actif ; aucun freinage tardif | | |
| 2 | Pendant la pleine vitesse, quitter la page | Aucune erreur ; rien ne continue de tourner au retour | | |
| 3 | Pendant un essai, cliquer « Enregistrer » puis « Roue par défaut » | Indépendants de l'essai (pas de blocage) | | |
| 4 | Navigateur en « réduire les animations » | Les temps s'enchaînent, la roue est placée directement sur la case tirée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

### Scenario 4 — Aucun effet sur le jeu

**Validation** : AUTO — **Issues** : #144
*(équivalent automatisé : `WheelConfigCard.tester.144.test.jsx` — aucun WebSocket, aucun POST `/game-config.json`, seul appel = `wheel-preview`)*

| Etape | Action | Resultat Attendu | Resultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer un essai complet pendant qu'une manche ROUE est affichée sur la TV | La TV, les tablettes et l'état de la manche ne changent pas ; la configuration enregistrée est inchangée au rechargement de la page | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Criteres de Validation

- [ ] Les deux aperçus tournent ensemble et s'arrêtent sur la même case
- [ ] Montée = 2 × durée d'un tour, durée maximale comptée depuis la pleine vitesse
- [ ] Aucun effet sur le jeu ni sur la configuration enregistrée

## Notes QA

Le second aperçu est la vue d'ensemble circulaire (et non un arc de TV) : à confirmer par l'utilisateur en QUALIF.
