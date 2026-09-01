# Procédure de Test — RAFALE : aperçu de la question suivante sur `/anim` (#202)

**Version** : v8.0.0 (branche `milestone/v8.0.0`)
**Date** : 2026-09-01
**Testeur** : Utilisateur (validation manuelle — ni `qa` ni `deployer` n'exécutent cette procédure,
aucun navigateur fiable dans les sessions agents)
**Issue** : #202 — sur `/anim`, énoncé de la question RAFALE courante agrandi + zone « SUIVANTE »
en bas de l'encart, affichant la prochaine question (pré-tirage réel, contrat `contracts/rafale.md`
§13)
**Maquette validée** : https://claude.ai/code/artifact/388d8cc6-549a-4b26-9fbf-b6eb5e17286c
**Plan de référence** : `_work/reports/plan-20260901-104203.md`

## Contexte

Avant #202, `/anim` affichait uniquement la question RAFALE en cours. Après #202 :
- L'énoncé courant est nettement plus grand, lisible à distance de tablette.
- Une zone discrète « Suivante » apparaît en bas de l'encart, avec l'énoncé de la prochaine
  question (+ catégorie et difficulté) — **réellement pré-tirée** côté serveur (pas une simulation),
  donc garantie d'être celle qui apparaîtra effectivement au tick suivant.
- Cette question suivante n'est **jamais** transmise à `/tv` ni `/player` (même régime de
  confidentialité que la réponse RAFALE, contrat §13.2 — critère bloquant).

---

## Prérequis

- [ ] Environnement : QUALIF (binaire Windows buildé, cf. `docs/QUALIF_PROCEDURE.md`)
- [ ] Réservoir RAFALE (`/admin/rafale`) peuplé avec au moins 15 questions (catégorie/difficulté au
  choix) pour les scénarios 1/2/4, **plus un jeu de test à part réduit à 2 questions** (même
  catégorie/difficulté, isolées du reste) pour le scénario 3
- [ ] Une question de type `RAFALE` configurée dans un quiz (mode au choix, `RAFALE_QUESTION_TIME`
  ~3s par défaut)
- [ ] Postes ouverts : `/admin` (régie), `/anim` (tablette animateur — à la résolution tablette
  réelle, pas seulement en fenêtre desktop large), `/tv` (écran salle), `/player` (tablette joueur)
- [ ] Accès à la console réseau du navigateur (DevTools) sur `/tv` et `/player`, pour le scénario 5
- [ ] Accès à `GET /api/rafale/pool` (ou l'onglet réservoir de `/admin/rafale` affichant le nombre
  de questions disponibles), pour le scénario 6

---

## Scénario 1 — 10 questions consécutives : la « suivante » annoncée devient bien la courante

**Objectif** : Vérifier la garantie centrale du pré-tirage — la question affichée en « Suivante »
est **exactement** celle qui devient courante au tick suivant, quel que soit le mode de transition
(VALIDE, INVALIDE, expiration du timer de question).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer une manche RAFALE (catégorie avec au moins 15 questions disponibles) | Première question posée, zone « Suivante » affiche une 2ᵉ question en bas de l'encart | | |
| 2 | Noter l'énoncé affiché dans la zone « Suivante » | — | | |
| 3 | Cliquer RÉPONSE VALIDE (ou INVALIDE, ou laisser le timer de question expirer — alterner les 3 déclencheurs au fil des 10 questions) | La question qui devient courante est **exactement** celle notée à l'étape 2, mot pour mot | | |
| 4 | Répéter les étapes 2-3 encore **9 fois** (10 questions consécutives au total) | À CHAQUE fois, sans exception, la « Suivante » notée devient la courante suivante | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Lisibilité de l'énoncé courant, aucun scroll ni débordement

**Objectif** : Vérifier le rendu visuel demandé (énoncé courant nettement agrandi, hiérarchie claire
avec la zone « Suivante », `/anim` reste strictement statique).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur `/anim`, à la résolution TABLETTE réelle (pas desktop large), observer l'encart RAFALE pendant une manche en cours | Énoncé de la question courante nettement plus grand qu'avant #202, lisible à distance normale d'usage tablette | | |
| 2 | Observer l'ensemble de l'encart (méta, énoncé, réponse, zone Suivante) | Aucun scroll, aucun débordement visible, quelle que soit la longueur de l'énoncé (tester avec une question à énoncé long si disponible dans le réservoir) | | |
| 3 | Comparer visuellement la taille de l'énoncé courant à celle de la zone « Suivante » | La zone « Suivante » est nettement plus petite et visuellement secondaire (atténuée, séparée par un filet) — aucune confusion possible entre les deux au premier coup d'œil | | |
| 4 | Vérifier qu'aucune autre zone de `/anim` n'a changé (boutons VALIDE/INVALIDE, timers, bandeau contexte, zone équipes) | Rendu strictement identique à avant #202 en dehors de l'encart RAFALE | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Réservoir réduit à 2 questions : la 2ᵉ affiche « dernière question du réservoir »

**Objectif** : Vérifier le comportement de fin de réservoir — la zone « Suivante » doit annoncer
clairement qu'il n'y a plus de question après la question courante, sans planter ni afficher un
énoncé fantôme.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Configurer une manche RAFALE filtrée sur le jeu de test à 2 questions préparé (catégorie/difficulté isolées) | La 1ʳᵉ question s'affiche, zone « Suivante » affiche la 2ᵉ (et dernière) question du réservoir | | |
| 2 | Valider/invalider la 1ʳᵉ question | La 2ᵉ question devient courante | | |
| 3 | Observer la zone « Suivante » sur cette 2ᵉ question | Message « Dernière question du réservoir » (ou équivalent explicite) au lieu d'un énoncé — PAS de zone vide, PAS d'énoncé d'une question déjà posée | | |
| 4 | Valider/invalider cette dernière question | La manche se termine proprement (`RAFALE_EXHAUSTED`), comportement de fin de manche inchangé par rapport à avant #202 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — `ROUND_END` : la zone « Suivante » disparaît

**Objectif** : Vérifier que la zone « Suivante » n'a aucune raison d'être visible une fois la manche
terminée (rien à préparer).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Démarrer une manche RAFALE, laisser le chrono de MANCHE (pas de question) descendre à 0, OU épuiser le réservoir | Manche se termine, sous-phase passe à `ROUND_END` | | |
| 2 | Observer l'encart RAFALE sur `/anim` | La zone « Suivante » a **entièrement disparu** (pas de message résiduel, pas d'énoncé de la dernière question annoncée avant l'arrêt) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Inspection réseau `/tv` et `/player` (critère bloquant)

**Objectif** : Vérifier, au niveau réseau (pas seulement visuel), que l'énoncé de la question
suivante — ainsi que toute réponse — n'atteint **jamais** `/tv` ni `/player`. Complète le test
automatisé bloquant (`cmd/server/rafale_answer_leak_test.go`) par une vérification bout-en-bout
réelle, patron du scénario 11 de `tests/procedures/rafale-v8.md`.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir les DevTools réseau (onglet WS) sur `/tv`, démarrer une manche RAFALE, noter l'énoncé de la zone « Suivante » côté `/anim` sur plusieurs questions | Observer les messages WebSocket entrants sur `/tv` pendant ces mêmes questions | | |
| 2 | Rechercher dans tous les messages reçus par `/tv` le texte de la question « Suivante » notée (ainsi que celui de la réponse attendue, déjà couvert par `rafale-v8.md` scénario 11) | **Absent** de tous les messages reçus par `/tv`, en particulier `RAFALE_ANSWER` ne doit **jamais** apparaître dans les messages reçus par `/tv` (ce canal est réservé admin+anim, `/tv` ne devrait même pas le recevoir) | | |
| 3 | Répéter l'étape 1-2 sur `/player` (une tablette VJoueur) | Même résultat : aucune trace de l'énoncé suivant ni de la réponse dans les messages reçus par `/player` | | |
| 4 | Sur `/anim` (DevTools réseau), vérifier que `RAFALE_ANSWER` EST bien reçu avec un champ `NEXT` renseigné | La question suivante est visible côté `/anim` (et `/admin` si ouvert) — c'est le seul canal légitime | | |

**Verdict** : [ ] PASS  [ ] FAIL — **bloquant : un échec ici bloque la release**

---

## Scénario 6 — Manche arrêtée en cours : le pré-tirage ne consomme pas de question en trop

**Objectif** : Vérifier que le pré-tirage (question « sur le pont », marquée `used` par anticipation)
est bien **libéré** quand une manche est arrêtée avant d'avoir consommé cette question — sinon le
réservoir perdrait silencieusement une question par manche arrêtée (contrat §13.4, risque R1 du
plan).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Noter le nombre de questions disponibles dans le réservoir (`/admin/rafale` ou `GET /api/rafale/pool`) pour la catégorie/difficulté testée | Valeur de référence notée (N) | | |
| 2 | Démarrer une manche RAFALE sur cette catégorie/difficulté, valider/invalider **3 questions** (noter leurs énoncés) | Compteur avance normalement, zone « Suivante » visible à chaque question | | |
| 3 | Arrêter la manche (STOP) **en pleine question**, avant qu'elle n'expire naturellement | Manche stoppée | | |
| 4 | Revérifier le nombre de questions disponibles dans le réservoir | **N − 3** (les 3 questions réellement posées) — **PAS** N − 4 (qui inclurait la question pré-tirée mais jamais posée, « sur le pont » au moment du STOP) | | |
| 5 | Redémarrer une nouvelle manche sur la même catégorie/difficulté | Aucune des questions notées à l'étape 2 n'est reproposée ; la question qui était « sur le pont » au moment du STOP peut réapparaître normalement (elle a été libérée, pas gaspillée) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] La question annoncée en « Suivante » devient exactement la question courante suivante, sur
  10 questions consécutives, tous déclencheurs confondus (scénario 1)
- [ ] L'énoncé courant est nettement plus lisible, sans scroll ni débordement, hiérarchie claire
  avec la zone « Suivante » (scénario 2)
- [ ] La dernière question du réservoir affiche le message dédié, pas un énoncé fantôme
  (scénario 3)
- [ ] La zone « Suivante » disparaît entièrement en `ROUND_END` (scénario 4)
- [ ] **Aucune fuite réseau** de l'énoncé suivant (ni de la réponse) vers `/tv` ou `/player`
  (scénario 5 — **critère bloquant**)
- [ ] Une manche arrêtée en cours ne consomme pas de question de réservoir en trop — le pré-tirage
  est bien libéré (scénario 6)
- [ ] `RAFALE_POOL_REMAINING` affiche la même valeur qu'avant #202 à position de manche égale
  (observable indirectement au fil des scénarios 1 et 6)
- [ ] Aucune régression sur les autres scénarios RAFALE déjà validés (`tests/procedures/rafale-v8.md`)
  ni sur les autres types de question

## Notes QA

[Espace pour observations]
