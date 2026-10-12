# Procédure de Test — Réponses des autres équipes et bonne réponse cachées aux tablettes (#258, v11.2)

**Version** : v11.2.x (QUALIF)
**Date** : 2026-10-12
**Testeur** : **l'utilisateur uniquement** (deux tablettes, outils du navigateur : ni `qa` ni `deployer` ne l'exécutent)
**Contrat** : `contracts/vplayer-payload-filter.md` §7
**Tests automatiques** : `tests/INDEX.md` (lignes #258)

## Prérequis

- [ ] QUALIF, deux équipes (A et B), **une tablette joueur par équipe** (ouvertes sur `/player`, joueurs inscrits), un QCM et un SPEEDY disponibles
- [ ] Sur la tablette de l'équipe A : ouvrir les outils du navigateur (F12 → onglet **Réseau**, filtre « WS » → la connexion `/ws/player` → **Messages**)

## Scénario 1 — Réponse d'une autre équipe invisible pendant la question (QCM)

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Admin : lancer un QCM, START | Tablettes A et B affichent les réponses | | |
| 2 | Tablette **B** : choisir une réponse | | | |
| 3 | Tablette **A** (outils) : ouvrir les trames reçues depuis le buzz de B | Dans chaque trame `UPDATE`/`PAUSE`… : pour le joueur de B, `TIME`, `BUTTON`, `STATUS`, `ANSWER_COLOR`, `ANSWERED_COLOR`, `HINTS_AT_BUZZ` valent **0 ou ""** ; `teams` de B : `TIME`, `BUMPER`, `STATUS` à 0 ou "" | | |
| 4 | Tablette **A** : rechercher « QCM_CORRECT » et « ANSWER » dans les trames | **Aucune** occurrence dans `GAME.QUESTION` avant le REVEAL | | |
| 5 | Tablette **B** (outils) : ses propres trames | Son joueur porte **ses** valeurs (couleur répondue, TIME > 0) | | |
| 6 | Écran **TV** (outils) | Inchangé : la réponse de B et la bonne réponse y sont toujours (TV publique) | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 2 — STOP puis REVEAL

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | Admin : STOP | Les tablettes affichent les **badges des réponses par équipe** comme avant ; les trames de A portent maintenant les réponses de B | | |
| 2 | Tablette A : chercher « QCM_CORRECT » | Toujours **absent** après STOP | | |
| 3 | Admin : REVEAL | La bonne réponse s'affiche sur les tablettes ; les trames contiennent `QCM_CORRECT` et `ANSWER` | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Scénario 3 — SPEEDY, coéquipiers, connexion non liée

| Étape | Action | Résultat attendu | Résultat obtenu | OK ? |
|-------|--------|------------------|-----------------|------|
| 1 | SPEEDY : START, la tablette B buzze en premier | Tablette A : aucune trame ne révèle qui a buzzé (valeurs neutres), y compris à la PAUSE puis au CONTINUE | | |
| 2 | Deux tablettes de la **même** équipe : l'une répond | L'autre voit ses valeurs de réponse (aucun filtrage entre coéquipiers) | | |
| 3 | Ouvrir `/player` sans s'inscrire (connexion non liée), outils ouverts, question en cours | Premier état reçu : tous les joueurs neutralisés ; **pas** de `ARDOISE_ANSWERS` ni de `LAUNCH_LOCK` | | |
| 4 | Carte MEMOTION de type QCM active | La bonne réponse de la carte n'apparaît pas dans les trames de la tablette avant sa révélation | | |

**Verdict** : [ ] PASS  [ ] FAIL

## Critères de Validation

- [ ] Aucune trame reçue par une tablette ne contient la réponse d'une autre équipe pendant que les réponses sont ouvertes
- [ ] Aucune trame ne contient la bonne réponse avant REVEALED
- [ ] Les badges de réponses par équipe s'affichent toujours à STOPPED/REVEALED ; TV et anim inchangées
