# Procédure de Test — Conduite permanente & zone réponse `/anim` (#166 + #169)

**Version** : v6.2.x (branche `feature/anim-question-display`)
**Date** : 2026-08-15, révisée 2026-08-16 (#169)
**Testeur** : QA
**Issue** : #166 — bouton "à suivre" en conduite, zone réponse permanente, L4 réservée ;
révisée par #169 — la zone réponse se révèle désormais par appui maintenu (avant REVEALED) au
lieu d'un flou permanent passif
**Référence** : Plan `_work/reports/plan-20260815-144925.md` (révision 5), maquette
https://claude.ai/code/artifact/49cb60ae-8c6a-46f6-9268-5b0a6b5eb385 (tableau "Matrice de L1 et
du bouton « À suivre »" — source unique pour le parcours des 9 phases ci-dessous). Scénario 2
révisé pour #169 — voir issue #169 pour le comportement attendu détaillé.

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL
- [ ] Deux résolutions à couvrir sur toute la procédure : **1280×800** et **1024×768** (navigateur
      redimensionné ou tablette physique)
- [ ] Un quiz d'au moins 3 questions, incluant au moins une SPEEDY et une QCM avec indices activés
- [ ] Au moins 2 équipes actives (bumpers assignés)
- [ ] `/anim` sur une tablette (ou navigateur), `/admin` sur un second poste pour piloter

---

## Scénario 1 — Parcours des 9 phases depuis NEW_GAME (matrice complète)

**Objectif** : Vérifier, phase par phase, la matrice de la maquette (L1 : LANCER/PAUSE/CONTINUER/
STOP/RÉPONSE, "à suivre", zone réponse) — présence, couleur, libellé secondaire, et absence de
gris sur tout bouton cliquable.

| # | Étape | Résultat Attendu | Résultat Obtenu | OK ? |
|---|-------|-------------------|----------------|------|
| 1 | Sur `/admin`, cliquer "Nouvelle partie" (NEW_GAME) | `/anim` : L1 entièrement éteint ; "à suivre" **vert**, pointant la 1ʳᵉ question du quiz ("1ʳᵉ" implicite — enchaînement vers la 1ʳᵉ question jouable) ; zone réponse **absente/vide** (aucune question chargée) | | |
| 2 | Observer la phase d'enrôlement si activée (ENROLL) | L1 éteint ; "à suivre" **inerte** (non cliquable, aucune action au clic) ; zone réponse vide | | |
| 3 | Sur `/admin`, sélectionner la 1ʳᵉ question (phase PREPARE, avant PONG des buzzers) | L1 éteint ; "à suivre" **vert** ; zone réponse **floutée** (présente, illisible) | | |
| 4 | Attendre READY (tous les buzzers prêts) | LANCER **vert**, libellé secondaire "attendu" ; reste de L1 éteint ; "à suivre" **bleu** (optionnel, LANCER dispo à côté) ; zone réponse floutée | | |
| 5 | Cliquer LANCER — observer COUNTDOWN si présent | L1 éteint ; "à suivre" **inerte** ; zone réponse floutée | | |
| 6 | Phase STARTED (chrono en cours) | PAUSE **bleu** ("optionnel") ; STOP **rouge** ("arrête") ; LANCER éteint ("en cours") ; RÉPONSE éteint ("après arrêt") ; "à suivre" **inerte** ; zone réponse floutée | | |
| 7 | Cliquer PAUSE (PAUSED) | CONTINUER **vert** ("reprise") ; STOP **rouge** ; reste éteint ; "à suivre" **inerte** ; zone réponse floutée | | |
| 8 | Cliquer CONTINUER puis laisser expirer ou cliquer STOP (STOPPED, jouée) | RÉPONSE **vert** ("attendu") ; reste de L1 éteint ; "à suivre" **bleu** (optionnel, RÉPONSE dispo à côté) ; zone réponse floutée | | |
| 9 | Cliquer RÉPONSE (REVEALED) | L1 entièrement éteint (RÉPONSE : "déjà révélée") ; "à suivre" **vert** (seule action possible) ; zone réponse **nette** (voile levé) | | |
| 10 | Cliquer "à suivre" pour passer à une question SANS la jouer (STOPPED non jouée) | RÉPONSE éteint (pas encore jouée) ; reste de L1 éteint ; "à suivre" **vert** (seule action possible, PAS bleu) ; zone réponse floutée | | |

**Vérification transversale (toutes les étapes)** : aucun bouton cliquable n'est gris — le gris est
réservé aux boutons **éteints** (non cliquables) et au texte "Aucune question disponible"/réserves
L3/L4. Chaque bouton éteint affiche un libellé secondaire explicite (jamais silencieux).

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Zone réponse : révélation par pression tactile (#169), absence de décalage

**Objectif** : Vérifier que la zone réponse est masquée par défaut et ne se révèle QUE par un appui
maintenu avant REVEALED (remplace le flou permanent passif de #166 — retour utilisateur QUALIF
v6.2.0.16 : le flou seul ne protégeait rien de réel), et qu'elle ne bouge pas au reveal.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger une question SPEEDY (READY), observer la zone réponse sans y toucher | Masquée par défaut (flouté ou équivalent), pas de lecture possible | | |
| 2 | Poser le doigt sur la zone réponse et le MAINTENIR | La réponse se **révèle** (nette) tant que le doigt reste posé | | |
| 3 | Relâcher le doigt | La réponse se **remasque** immédiatement | | |
| 4 | Refaire un appui, puis faire glisser le doigt HORS de la zone sans le relever | La réponse se remasque dès la sortie de la zone (comme un relâchement) | | |
| 5 | Répéter l'appui plusieurs fois de suite | Le comportement est identique à chaque fois (pas de blocage dans un état après le premier appui) | | |
| 6 | Reproduire les étapes 2-3 sur une question QCM avec indices | Même comportement : pastille de couleur + libellé révélés le temps de l'appui, remasqués au relâchement | | |
| 7 | Faire tomber un ou plusieurs indices (QCM_HINT) pendant que la zone est masquée | La zone réponse reste masquée par défaut — les indices n'affectent QUE la grille L2 (propositions grisées/barrées), jamais l'état par défaut de la zone réponse | | |
| 8 | Presser RÉPONSE (REVEALED) — observer l'écran au moment exact du reveal | **Aucun décalage visuel** : la zone réponse devient nette en PERMANENCE, AU MÊME EMPLACEMENT, MÊME TAILLE — rien d'autre ne bouge à l'écran | | |
| 9 | Une fois en REVEALED, tenter un appui puis un relâchement sur la zone | **Aucun effet** : reste nette en permanence, pas de flicker ni de masquage au relâchement — plus besoin de garder le doigt posé pour créditer les équipes | | |
| 10 | Comparer la bonne réponse (une fois révélée, étape 8) avec le marquage de la grille L2 (QCM) | Les deux concordent (même pastille/couleur) — "les deux lectures se confirment" | | |
| 11 | Sélectionner une question SPEEDY sans `ANSWER` renseignée (ou ARDOISE/MEMORY si disponible), tester l'appui maintenu | Zone réponse présente avec un **tiret** ("—") révélé/masqué par l'appui, jamais un flou visible sur du vide | | |
| 12 | Retourner à NEW_GAME / aucune question chargée | Zone réponse **absente** du bandeau (pas un cadre flouté/masqué vide) | | |
| 13 | Maintenir l'appui sur la zone réponse puis, sans relâcher, faire passer l'admin à la question suivante (STOP+enchaînement rapide) | La réponse de la NOUVELLE question reste masquée par défaut — pas de fuite de l'état "révélé" d'une question à l'autre | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Dernière question du quiz

**Objectif** : Vérifier le comportement de fin de quiz (contracts/CHANGELOG.md [20260815-1]/[-2]).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Jouer les questions jusqu'à la dernière du quiz | Compteur `n/total` affiche "N/N" (ex. "12/12") | | |
| 2 | Sur la dernière question, observer "à suivre" à chaque phase (READY/STARTED/STOPPED/REVEALED) | "à suivre" **inerte** partout SAUF REVEALED (vert) et STOPPED-non-jouée (vert) — libellé "Fin du quiz" au lieu du format habituel, aucune action au clic quand inerte | | |
| 3 | Après REVEALED sur la dernière question, cliquer "à suivre" | Aucun effet (rien à sélectionner) — bouton inerte malgré l'état vert-normal des autres phases (pas de nextQuestion disponible) | | |
| 4 | Vérifier que le compteur reste "12/12" (ou équivalent) même sans question suivante | La progression ne redevient jamais vide/absente sur la dernière question | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Démarrage sans question courante (B2, écart de parité assumé)

**Objectif** : Vérifier que "à suivre" pointe la première question jouable dès NEW_GAME, sans passer
par `/admin` (écart de parité documenté, contracts/CHANGELOG.md [20260815-2]).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer une nouvelle partie (NEW_GAME), NE PAS sélectionner de question sur `/admin` | Sur `/anim`, "à suivre" est **vert et cliquable**, pointant la première question du quiz (format complet visible) | | |
| 2 | Cliquer "à suivre" directement depuis `/anim`, sans jamais être passé par `/admin` | La première question se charge (phase PREPARE/READY) — la tablette peut démarrer une partie seule | | |
| 3 | Comparer avec `/admin` au même instant (avant toute sélection) | `/admin` n'affiche **aucun** bouton "à suivre" tant qu'aucune question n'a été sélectionnée — écart de parité assumé et documenté, PAS une régression | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Lisibilité et tenue en 1024×768

**Objectif** : Vérifier la lisibilité des libellés et l'absence de débordement sur le plus petit écran
qualifié.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Redimensionner à 1024×768, charger une question QCM avec indices activés (le cas le plus haut) | Les 5 lignes de conduite (L1, à suivre, L2, L3, L4) et la bande régie restent entièrement visibles, sans scroll (`/anim` en `overflow: hidden`) | | |
| 2 | Observer le libellé "CONTINUER" en phase PAUSED | Texte entièrement lisible, ne déborde pas de son bouton (police adaptative) | | |
| 3 | Observer les 4 propositions QCM en L2 avec un libellé long | Troncature propre (ellipsis), pas de chevauchement | | |
| 4 | Vérifier au moins 6 cartes équipe visibles simultanément en 1280×800 | Colonne équipes pleine hauteur, pas de défilement pour 6 équipes | | |
| 5 | Redimensionner à 1024×600 (barres navigateur) si possible | La page tient toujours (marge réduite mais présente) ; en cas de débordement, les réserves L3/L4 doivent être les premières à se réduire, jamais les surfaces tactiles (≥ 62 px) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Réserves L3/L4 et bande régie

**Objectif** : Vérifier la présence des emplacements réservés à toutes les phases, sans contenu ni
interaction.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Observer L3 en SPEEDY et en QCM, à plusieurs phases | Toujours présente, texte discret ("Aucun geste propre au mode ..."), jamais de bouton ni de donnée | | |
| 2 | Observer L4 à plusieurs phases | Toujours présente, texte statique de réservation (#168), jamais de bouton ni de donnée | | |
| 3 | Observer la bande régie (bas de page) | Toujours présente, pleine largeur, texte statique (#167), aucune interaction possible | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Non-régression

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go build ./...` puis `go test ./... -race` | Build OK, tous les tests PASS | | |
| 2 | `npm test` (suite React complète) | Tous les tests PASS, y compris `AnimNextButton.test.jsx`, `AnimConductPanel.test.jsx` (réécrit), `AnimAnswerZone.test.jsx`, `AnimQcmOptions.test.jsx` (inchangé) | | |
| 3 | Sur `/admin`, dérouler une manche complète (LANCER/PAUSE/STOP/RÉPONSE/enchaînement) | Comportement strictement inchangé par #166 (F4b est une extraction pure) | | |
| 4 | Sur `/anim`, vérifier crédit par équipe et couleur de réponse QCM en zone équipes | Inchangés (#157) | | |
| 5 | Sur `/tv`, dérouler une manche SPEEDY et une manche QCM | Comportement TV inchangé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] Matrice des 9 phases conforme à la maquette (Scénario 1) — 0 bouton cliquable gris
- [ ] Zone réponse (#169) : masquée par défaut, révélée UNIQUEMENT par appui maintenu avant REVEALED, remasquée au relâchement/sortie ; visible en permanence sans interaction une fois REVEALED ; aucun décalage visuel au reveal ; pas de fuite d'état entre deux questions
- [ ] Dernière question du quiz : "à suivre" inerte avec libellé dédié, progression n/total conservée
- [ ] Démarrage sans question courante : "à suivre" pointe la 1ʳᵉ question, écart de parité `/admin` assumé
- [ ] 1024×768 : lisibilité CONTINUER + QCM, aucun débordement
- [ ] L3, L4, bande régie toujours présentes, jamais interactives
- [ ] Aucune régression `/admin`, `/tv`, zone équipes `/anim`

---

## Notes QA

[Espace pour observations]
