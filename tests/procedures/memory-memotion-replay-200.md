# Procédure de Test — MEMORY/MEMOTION/RAFALE : démarrage sans équipe sélectionnée (#200)

**Version** : v8.0.0 (branche `milestone/v8.0.0`)
**Date** : 2026-08-31 (mise à jour cycle 5 — cause racine réelle du rapport QUALIF v8.0.0.18)
**Testeur** : Utilisateur (validation manuelle — ni `qa` ni `deployer` n'exécutent cette procédure,
aucun navigateur fiable dans les sessions agents)
**Issues** : #200 — cinq cycles, plusieurs causes racines distinctes, correctifs complémentaires :
- Cycle 1/2 : généralisation à MEMORY/MEMOTION du fix RAFALE #199 (SHA `a7b70057`) — sélection
  d'équipes PÉRIMÉE persistant sur un **rejeu** de la même question.
- Cycle 3 : `Engine.Pause()`/`Continue()` sans aucune garde de phase — contournement **direct** de
  START via `ACTION:"CONTINUE"`, reproductible dès la **première** manche (pas un rejeu).
- Cycle 4 : un `PAUSE`/`CONTINUE` refusé par la garde du cycle 3 diffusait quand même l'action à
  TOUS les clients connectés (`/tv`, `/player`, autres onglets `/admin`/`/anim`) — désynchronisation
  purement visuelle (l'état serveur restait correct en PREPARE/STOPPED), reproduisant le symptôme
  au niveau affichage sur des écrans qui n'ont même pas initié l'action.
- **Cycle 5 — cause racine RÉELLE du rapport QUALIF v8.0.0.18**, distincte de tout ce qui précède :
  une manche démarrée (avec équipes) puis **STOPPÉE IMMÉDIATEMENT SANS AUCUNE action de jeu**
  (aucune carte jamais retournée/sélectionnée — ex. mauvaise config, on arrête tout de suite pour
  corriger) laissait la sélection d'équipes de CETTE manche en place ; un rejeu de la même question
  satisfaisait alors silencieusement la garde. C'était exactement le cas documenté comme « limite
  acceptée, jugée irréaliste » à l'issue du cycle 1/2 — confirmé irréaliste À TORT : c'est un flux
  tout à fait ordinaire.

**SHA fix cycle 1/2** : `142ffc3c` · **SHA tests** : `b42d5091` + complément `test-writer` (SHA `fe4bef24`)
**SHA fix cycle 3** : `64b23dff` · **SHA tests** : `453582f2`
**SHA fix cycle 4** : `0aa0d564` · **SHA tests** : `8ccef0d8`
**SHA fix cycle 5** : `4aaa9fbd` · **SHA tests** : `80f11384` + complément `test-writer` (repro WS
bout-en-bout START→STOP zéro-action→rejeu, `cmd/server/memory_start_no_team_repro_200_test.go`)
**Fichier de référence** : `docs/GAME_STATE_MACHINE.md` (transitions PREPARE/READY/STARTED/PAUSED/STOPPED)

## Contexte du bug (cycle 1/2 — rejeu)

`Engine.Ready()` ne réinitialisait la sélection d'équipes participantes
(`MemoryParticipatingTeams`/`MotionParticipatingTeams` et champs associés) que
lorsque la question rejouée était **nouvelle** (`isNewQuestion`). En rejouant
la **même** question (même ID) après une manche jouée et arrêtée, sans
resélectionner d'équipe, la sélection de la manche PRÉCÉDENTE restait active
côté moteur — alors que l'interface `/anim` affiche « aucune équipe
sélectionnée ». Le bouton DÉMARRER pouvait alors silencieusement s'activer
sur une sélection périmée, invisible à l'écran.

**Comportement attendu après le fix** : rejouer la même question sans
resélectionner une équipe doit laisser le jeu bloqué en `PREPARE` — DÉMARRER
refusé — jusqu'à ce qu'une équipe soit explicitement (re)sélectionnée.

---

## Prérequis

- [ ] Environnement : QUALIF (binaire Windows buildé, cf. `docs/QUALIF_PROCEDURE.md`)
- [ ] Un quiz avec au moins une question de type `MEMORY` (mode `CHACUN_SON_TOUR` ou
  `TANT_QUE_JE_GAGNE` — mode multi-équipes) et une question de type `MEMOTION`
  (mode `CHACUN_SON_TOUR` ou `TANT_QUE_JE_GAGNE`)
- [ ] Au moins 2 équipes créées, avec au moins 1 buzzer physique ou VJoueur assigné à chacune
- [ ] Postes ouverts : `/admin` (régie) ou `/anim` (tablette animateur — sélection des équipes
  participantes s'y fait)

---

## Scénario 1 — MEMORY : rejeu de la même question sans resélection reste bloqué

**Objectif** : Vérifier que le rejeu d'une question MEMORY déjà jouée, sans resélection d'équipes,
laisse le jeu bloqué en PREPARE (pas de démarrage sur une sélection périmée).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, préparer la question MEMORY (mode multi), sélectionner 2 équipes participantes | Sélection visible sur `/anim`, jeu passe en READY | | |
| 2 | Démarrer la manche (DÉMARRER) | Jeu en STARTED, grille MEMORY jouable | | |
| 3 | Retourner au moins une paire de cartes (jouer un minimum d'actions) | Action de jeu enregistrée normalement | | |
| 4 | Arrêter la manche (STOP côté admin/anim) | Jeu revient en STOPPED | | |
| 5 | Rejouer la MÊME question MEMORY (rouvrir la même question dans le quiz, sans en choisir une autre) | `/anim` affiche **aucune équipe sélectionnée** (la sélection précédente n'apparaît PAS comme active) | | |
| 6 | SANS sélectionner d'équipe, tenter DÉMARRER | **DÉMARRER refusé** — le jeu reste en PREPARE, aucune manche ne démarre | | |
| 7 | Sélectionner à nouveau au moins 2 équipes participantes | Le jeu passe en READY normalement (pas de blocage résiduel) | | |
| 8 | Démarrer la manche | Démarre normalement, la manche se déroule sans anomalie | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — MEMOTION : rejeu de la même question sans resélection reste bloqué

**Objectif** : Même scénario que 1, transposé à MEMOTION (généralisation du fix).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, préparer la question MEMOTION (mode multi), sélectionner au moins 1 équipe participante | Sélection visible sur `/anim`, jeu passe en READY | | |
| 2 | Démarrer la manche | Jeu en STARTED, grille MEMOTION jouable | | |
| 3 | Sélectionner au moins une carte MEMOTION | Action de jeu enregistrée normalement | | |
| 4 | Arrêter la manche (STOP) | Jeu revient en STOPPED | | |
| 5 | Rejouer la MÊME question MEMOTION | `/anim` affiche **aucune équipe sélectionnée**, grille réinitialisée (aucune carte affichée comme jouée) | | |
| 6 | SANS sélectionner d'équipe, tenter DÉMARRER | **DÉMARRER refusé** — le jeu reste en PREPARE | | |
| 7 | Sélectionner à nouveau une équipe participante | Le jeu passe en READY normalement | | |
| 8 | Démarrer la manche | Démarre normalement | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Non-régression : re-préparation avant tout démarrage (persistance légitime)

**Objectif** : Vérifier que le fix ne casse PAS le cas légitime — revenir sur l'écran de préparation
d'une question AVANT tout démarrage ne doit PAS effacer la sélection en cours (comportement
d'origine que le garde `isNewQuestion` protégeait).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, préparer une question MEMORY (mode multi), sélectionner 2 équipes | Sélection visible, jeu en READY | | |
| 2 | Naviguer ailleurs sur `/anim` puis revenir sur la MÊME question SANS avoir démarré la manche | La sélection d'équipes précédente est **toujours affichée** (non effacée) | | |
| 3 | Démarrer directement | Démarre avec les équipes sélectionnées à l'étape 1, sans avoir besoin de resélectionner | | |
| 4 | Répéter les étapes 1-3 pour une question MEMOTION | Même comportement : sélection préservée tant qu'aucune manche n'a été jouée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Contexte du bug (cycle 3 — contournement direct via PAUSE/CONTINUER)

Rapport QUALIF v8.0.0.17 (après le fix cycle 1/2) : « le problème n'est pas réglé, je peux
toujours faire START alors qu'aucune équipe n'est sélectionnée » pour MEMORY. Cause racine
**distincte** du rejeu : `Engine.Pause()`/`Continue()` ne vérifiaient **aucune phase** avant de
transitionner (contrairement à `Start()`), ce qui permettait à `ACTION:"CONTINUE"` de faire passer
le jeu directement en `STARTED` depuis n'importe quelle phase — y compris `PREPARE`, avant toute
sélection d'équipe — en contournant entièrement la garde `participantsConform`. Côté frontend, le
bouton PAUSE/CONTINUER (`/anim` : bouton "CONTINUER" de L1 ; `/admin` : bouton bascule
PAUSE/CONTINUER) est déjà correctement désactivé (`phaseRules.js` : `pauseButtonState`/
`continueButtonState` — actif seulement depuis STARTED/PAUSED) — le correctif moteur (`64b23dff`)
est une protection de **défense en profondeur**, au cas où ce chemin serait atteint autrement
(fenêtres/onglets multiples désynchronisés, état client périmé, appel direct au protocole).

## Scénario 4 — MEMORY : PAUSE/CONTINUER inactifs et sans effet tant qu'aucune équipe n'est sélectionnée

**Objectif** : Reproduire le scénario exact du rapport utilisateur — vérifier qu'aucun geste sur
PAUSE/CONTINUER ne peut démarrer une manche MEMORY sans équipe sélectionnée.

**Prérequis spécifique** : en plus d'un poste `/anim` ou `/admin`, ouvrir également `/tv` (écran
salle) et une tablette `/player` (VJoueur) — le point clé du cycle 4 est qu'un refus doit rester
invisible PARTOUT, pas seulement sur le poste qui a tenté l'action.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, préparer une question MEMORY en mode multi (`CHACUN_SON_TOUR` ou `TANT_QUE_JE_GAGNE`), **NE SÉLECTIONNER AUCUNE équipe** | Jeu reste en PREPARE ; bouton LANCER grisé/non cliquable ("indispo.") | | |
| 2 | Observer l'état des boutons PAUSE et CONTINUER dans ce même état | Les deux boutons sont également grisés/non cliquables ("indispo.") — ni l'un ni l'autre n'émet d'action au clic | | |
| 3 | Cliquer malgré tout à l'emplacement de PAUSE, puis à l'emplacement de CONTINUER | Aucun effet visible sur `/anim` : le jeu reste en PREPARE, aucune manche ne démarre | | |
| 4 | Reproduire les étapes 1-3 sur `/admin` (bouton bascule PAUSE/CONTINUER de `GamePage.jsx`) | Même résultat : bouton grisé, aucun effet au clic, jeu reste en PREPARE | | |
| 5 | Pendant les étapes 1-4, observer en continu `/tv` et `/player` | **Aucun changement** sur `/tv` ni `/player` — pas de bandeau « manche en cours » ni de bascule visuelle, quel que soit ce qui a été tenté sur `/anim`/`/admin` (cycle 4 : un refus ne doit jamais atteindre les autres écrans) | | |
| 6 | Ouvrir un DEUXIÈME onglet/fenêtre sur `/anim` en parallèle du premier (toujours en PREPARE, MEMORY, aucune équipe) : dans ce second onglet, démarrer normalement une AUTRE question, la mettre en PAUSE, puis revenir rapidement sur le premier onglet (resté sur la question MEMORY sans équipe) et cliquer immédiatement où se trouve CONTINUER | Le clic reste sans effet sur la question MEMORY sans équipe (état désynchronisé entre onglets ne permet aucun contournement) — la manche ne démarre pas | | |
| 7 | Sélectionner une équipe puis démarrer normalement | La manche démarre normalement, PAUSE/CONTINUER redeviennent actifs pendant/après le déroulé, comportement inchangé ; `/tv`/`/player` reflètent alors normalement le vrai démarrage | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — RAFALE et MEMOTION : même garde, par précaution

**Objectif** : Le verrou ajouté (`Pause()`/`Continue()`) est générique — non spécifique à un type de
question — vérifier qu'aucune régression ni contournement équivalent n'existe pour RAFALE et
MEMOTION.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, préparer une question RAFALE (mode multi) SANS sélectionner de catégorie/équipe | LANCER, PAUSE et CONTINUER tous grisés/non cliquables ; clics sans effet, jeu reste en PREPARE | | |
| 2 | Sur `/anim`, préparer une question MEMOTION (mode multi) SANS sélectionner d'équipe | LANCER, PAUSE et CONTINUER tous grisés/non cliquables ; clics sans effet, jeu reste en PREPARE | | |
| 3 | Pour chaque type, sélectionner les équipes/catégorie requises puis démarrer/pause/continuer normalement | Déroulé normal, aucune régression sur le cycle PAUSE/CONTINUER légitime | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Reconnexion avec état périmé (vecteur réaliste du cycle 4)

**Objectif** : Le bouton PAUSE/CONTINUER n'est cliquable côté UI que si l'état LOCAL du navigateur
affiche STARTED/PAUSED (`phaseRules.js`) — un clic qui déclenche réellement un refus côté serveur
suppose donc que l'état local du client est PÉRIMÉ par rapport au serveur. Ce scénario reproduit ce
cas de façon réaliste, via une coupure réseau brève.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, démarrer normalement une question MEMORY (équipes sélectionnées), jeu en STARTED — bouton PAUSE actif | Bouton PAUSE cliquable, jeu en cours | | |
| 2 | Couper la connexion réseau du poste `/anim` quelques secondes (mode avion, ou désactiver le Wi-Fi) SANS fermer l'onglet | La tablette perd la connexion WS (indicateur de déconnexion si présent), l'affichage local reste figé sur STARTED | | |
| 3 | PENDANT la coupure, sur `/admin` (poste toujours connecté), arrêter la manche (STOP) | Le serveur repasse en STOPPED ; `/tv` et `/player` reflètent l'arrêt normalement | | |
| 4 | Rétablir immédiatement le réseau sur `/anim`, et cliquer sur PAUSE (ou CONTINUER selon l'affichage encore figé) DANS LA FENÊTRE avant que l'écran ne se resynchronise | Le clic part vers le serveur avec un état client périmé — le serveur refuse (jeu déjà STOPPED) | | |
| 5 | Observer `/tv` et `/player` pendant cette tentative | **Aucun changement visuel** — pas de bascule fantôme vers STARTED/PAUSED sur ces écrans suite au clic périmé | | |
| 6 | Observer `/anim` lui-même après resynchronisation | L'affichage se corrige et reflète l'état réel (STOPPED) — pas de blocage dans un état incohérent | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — START puis STOP immédiat sans action de jeu, puis rejeu (cycle 5 — bug réellement rencontré)

**Objectif** : Reproduire EXACTEMENT le scénario du rapport QUALIF v8.0.0.18 — c'est le scénario que
l'utilisateur a réellement rencontré en usage réel, distinct de tous les scénarios précédents de
cette procédure (pas un contournement CONTINUE, pas une resélection oubliée après plusieurs cartes
jouées — ici AUCUNE carte n'est jamais touchée avant l'arrêt).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, préparer une question MEMORY en mode multi (`CHACUN_SON_TOUR` ou `TANT_QUE_JE_GAGNE`), sélectionner 2 équipes participantes | Sélection visible, jeu passe en READY | | |
| 2 | Démarrer la manche (DÉMARRER) | Jeu en STARTED | | |
| 3 | **IMMÉDIATEMENT, sans retourner AUCUNE carte**, arrêter la manche (STOP) — ex. on se rend compte d'une mauvaise config et on coupe tout de suite | Jeu revient en STOPPED, aucune carte n'a jamais été retournée | | |
| 4 | Rejouer la MÊME question MEMORY (rouvrir la même question dans le quiz, sans en choisir une autre) | `/anim` affiche **aucune équipe sélectionnée** | | |
| 5 | SANS sélectionner d'équipe, tenter DÉMARRER | **DÉMARRER refusé** — le jeu reste en PREPARE, aucune manche ne démarre (avant le fix cycle 5 : démarrait à tort) | | |
| 6 | Sélectionner à nouveau au moins 2 équipes participantes puis démarrer | Démarre normalement | | |
| 7 | Répéter les étapes 1-6 pour une question MEMOTION (START → STOP immédiat sans sélectionner AUCUNE carte → rejeu sans resélection) | Même comportement : DÉMARRER refusé au rejeu tant qu'aucune équipe n'est resélectionnée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Rejouer une question MEMORY déjà jouée sans resélectionner une équipe bloque DÉMARRER
  (scénario 1)
- [ ] Rejouer une question MEMOTION déjà jouée sans resélectionner une équipe bloque DÉMARRER
  (scénario 2)
- [ ] Resélectionner une équipe après un rejeu débloque normalement la manche (scénarios 1-2, étapes 7-8)
- [ ] Revenir sur une question non encore démarrée ne fait PAS perdre la sélection en cours
  (scénario 3 — non-régression)
- [ ] PAUSE et CONTINUER restent inactifs (grisés, sans effet au clic) tant qu'une question MEMORY
  n'a pas d'équipe sélectionnée — scénario exact du rapport QUALIF v8.0.0.17 (scénario 4)
- [ ] Un refus de PAUSE/CONTINUER ne provoque AUCUN changement visuel sur `/tv` ni `/player`, même
  pendant les tentatives sur `/anim`/`/admin` (scénario 4, étape 5 — cycle 4)
- [ ] Aucun contournement possible via onglets/fenêtres multiples désynchronisés (scénario 4, étape 6)
- [ ] Même garde vérifiée pour RAFALE et MEMOTION (scénario 5)
- [ ] Un clic PAUSE/CONTINUER avec un état client périmé (reconnexion après coupure réseau) ne
  provoque aucune bascule fantôme sur `/tv`/`/player`, et le client se resynchronise proprement
  (scénario 6 — vecteur réaliste du cycle 4)
- [ ] Le cycle PAUSE/CONTINUER légitime (une fois la manche démarrée) fonctionne normalement, sans
  régression, pour les trois types (scénarios 4-5, dernière étape)
- [ ] START puis STOP immédiat SANS AUCUNE action de jeu, puis rejeu sans resélection, bloque
  DÉMARRER pour MEMORY ET MEMOTION — scénario exact du rapport QUALIF v8.0.0.18 (scénario 7)
- [ ] Aucune régression observée sur RAFALE (déjà corrigé en #199) ni sur les autres modes de jeu

## Notes QA

[Espace pour observations]

> **Historique corrigé** : le scénario 7 (START→STOP immédiat sans aucune action de jeu, puis
> rejeu) avait été documenté à l'issue du cycle 1/2 comme « limite connue, acceptée, jugée
> irréaliste » — c'était une erreur d'appréciation. Ce scénario s'est révélé être la cause racine
> RÉELLE du rapport QUALIF v8.0.0.18 (cycle 5, SHA `4aaa9fbd`) : cliquer START puis immédiatement
> STOP pour corriger une mauvaise config est un flux tout à fait ordinaire. Le scénario 7 ci-dessus
> est désormais un cas à valider explicitement, pas à ignorer.
