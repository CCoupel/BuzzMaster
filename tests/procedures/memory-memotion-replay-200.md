# Procédure de Test — Rejeu MEMORY/MEMOTION : sélection d'équipes périmée (#200)

**Version** : v8.0.0 (branche `milestone/v8.0.0`)
**Date** : 2026-08-31
**Testeur** : Utilisateur (validation manuelle — ni `qa` ni `deployer` n'exécutent cette procédure,
aucun navigateur fiable dans les sessions agents)
**Issues** : #200 (généralisation à MEMORY/MEMOTION du fix RAFALE #199, SHA `a7b70057`)
**SHA fix** : `142ffc3c` · **SHA tests** : `b42d5091` + couverture complémentaire ajoutée par `test-writer`
**Fichier de référence** : `docs/GAME_STATE_MACHINE.md` (transitions PREPARE/READY/STARTED/STOPPED)

## Contexte du bug

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

## Critères de Validation

- [ ] Rejouer une question MEMORY déjà jouée sans resélectionner une équipe bloque DÉMARRER
  (scénario 1)
- [ ] Rejouer une question MEMOTION déjà jouée sans resélectionner une équipe bloque DÉMARRER
  (scénario 2)
- [ ] Resélectionner une équipe après un rejeu débloque normalement la manche (scénarios 1-2, étapes 7-8)
- [ ] Revenir sur une question non encore démarrée ne fait PAS perdre la sélection en cours
  (scénario 3 — non-régression)
- [ ] Aucune régression observée sur RAFALE (déjà corrigé en #199) ni sur les autres modes de jeu

## Notes QA

[Espace pour observations]

> **Limite connue, acceptée** (documentée par `dev-backend`, SHA `142ffc3c`) : une manche démarrée
> puis stoppée SANS AUCUNE action de jeu (aucune carte jamais retournée en MEMORY, aucune carte
> jamais sélectionnée en MEMOTION) reste indiscernable de « jamais démarrée » — la sélection
> persiste dans ce cas précis. Jugé irréaliste en usage réel (jouer une manche implique de retourner/
> sélectionner au moins une carte) — **non couvert volontairement**, ne pas le tester comme un échec.
