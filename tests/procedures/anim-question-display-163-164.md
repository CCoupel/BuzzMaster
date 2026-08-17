# Procédure de Test — Zone contexte animateur (#163) + ouverture auto `/admin` (#164)

**Version** : v6.2.x (branche `feature/anim-question-display`)
**Date** : 2026-08-14
**Testeur** : QA
**Issues** : #163 (zone contexte `/anim` — énoncé, propositions QCM, bonne réponse, titre suivante), #164 (`/admin` ajoutée à l'ouverture automatique)
**Référence** : Plan `_work/reports/plan-20260814-101626.md`, décisions GATE 2 `_work/handoff/gate2-decisions-163-164.md`, maquette https://claude.ai/code/artifact/76c34d5c-74ce-4dcd-ad0d-3b102988b7af

---

## Prérequis

- [ ] Environnement : QUALIF ou LOCAL (build portable incluant le firmware mergé, voir `docs/QUALIF_PROCEDURE.md`)
- [ ] Une tablette (ou un navigateur redimensionné en 1280×800 puis 1024×768) sur `/anim`
- [ ] Un poste admin sur `/admin`
- [ ] Au moins une manche SPEEDY et une manche QCM disponibles dans le jeu de questions, avec au moins 2 équipes actives (bumpers assignés)
- [ ] Accès au fichier `config.json` pour basculer `auto_open_browsers` (scénario 1)

---

## Scénario 1 — Ouverture automatique des onglets au démarrage (#164)

**Objectif** : Vérifier que `/admin` s'ouvre désormais automatiquement au même titre que `/anim`, `/tv` et `/`, sans régression du comportement existant.

### 1a — Démarrage standard (`auto_open_browsers: true`, hors debug)

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Vérifier `config.json` : `auto_open_browsers: true`, `debug: false` | Config confirmée | | |
| 2 | Démarrer le serveur (`./server.exe` ou `./buzzcontrol-qualif`) | Serveur démarre, logs "WEB INTERFACE URLs" affichés | | |
| 3 | Observer les onglets ouverts automatiquement | **4 onglets** ouverts, dans cet ordre : `/admin`, `/anim`, `/tv`, `/` | | |
| 4 | Lire le log de démarrage pour chaque onglet | Libellés : `/admin (régie)`, `/anim (animateur)`, `/tv (affichage TV)`, `/ (accueil joueurs)` | | |
| 5 | Vérifier spécifiquement le libellé de `/anim` | **N'est plus annoncé "admin"** (correction du reliquat de l'alias supprimé par #155) | | |
| 6 | Chronométrer l'ouverture des 4 onglets | Délai total ≈ 1,5 s (500 ms entre chaque onglet, comme avant #164) | | |

### 1b — Démarrage en mode debug

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 7 | Relancer avec `debug: true` (config ou `--debug`) | Serveur redémarre en mode debug | | |
| 8 | Observer les onglets ouverts | **5 onglets** : `/admin`, `/anim`, `/tv`, `/`, puis `/logs` | | |
| 9 | Vérifier le libellé de `/logs` | `/logs (logs (debug))` | | |

### 1c — `auto_open_browsers: false`

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 10 | Passer `auto_open_browsers: false` dans `config.json`, redémarrer | Serveur démarre | | |
| 11 | Observer les onglets | **Aucun onglet ouvert automatiquement** | | |
| 12 | Lire le log | "Auto-open browsers disabled in config" | | |
| 13 | Remettre `auto_open_browsers: true` avant la suite de la procédure | Config restaurée | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Manche SPEEDY conduite depuis la tablette (#163)

**Objectif** : Vérifier que l'énoncé de la question courante et le titre de la question suivante s'affichent en zone contexte de `/anim`, phase par phase, pour une question hors QCM — et que la réponse attendue apparaît au reveal.

Se référer au tableau **« Règles d'affichage par phase »** de la maquette à chaque étape.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis `/admin`, sélectionner une question SPEEDY | `/anim` reçoit la question (phase `READY`) | | |
| 2 | Sur `/anim`, observer la zone contexte **avant** de presser LANCER | L'**énoncé complet** de la question est visible (max 2 lignes), lisible à bout de bras | | |
| 3 | Observer la puce « Suivante » | Titre au format exact GATE 2 : `#<ID> <type>: <énoncé> <points>pt <délai>s` (ex. `#2 SPEEDY: ... 10pt 30s`) | | |
| 4 | Presser **LANCER** (phase `STARTED`) | L'énoncé reste affiché à l'identique, le chronomètre démarre | | |
| 5 | Laisser le temps s'écouler ou presser **STOP** (phase `STOPPED`) | L'énoncé reste affiché | | |
| 6 | Presser **RÉPONSE** (phase `REVEALED`) | L'énoncé reste affiché, **et** un bloc « Réponse » apparaît avec la valeur de `ANSWER` | | |
| 7 | Vérifier qu'aucune proposition QCM n'apparaît à aucune étape | Aucune grille de propositions (question non-QCM) | | |
| 8 | Sur `/admin`, comparer l'énoncé affiché (aperçu TV embarqué) avec celui de `/anim` | Contenu identique | | |
| 9 | Vérifier la zone conduite (boutons) et la zone équipes (cartes, crédit) | Entièrement visibles, aucun débordement, aucune régression #155/#156/#157 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Manche QCM conduite depuis la tablette (#163)

**Objectif** : Vérifier l'affichage des propositions QCM, leur invalidation par indice, et la bonne réponse au reveal — jamais avant.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Depuis `/admin`, sélectionner une question QCM avec indices activés | `/anim` reçoit la question (phase `READY`) | | |
| 2 | Sur `/anim`, observer la zone contexte **avant** LANCER | Énoncé visible **et** les **4 propositions** visibles (grille 2×2, lettres A/B/C/D dans les couleurs des boutons buzzer RED/GREEN/YELLOW/BLUE) | | |
| 3 | Vérifier qu'aucune proposition n'est marquée comme correcte à ce stade | Aucun liseré vert / coche sur aucune proposition | | |
| 4 | Presser **LANCER** (phase `STARTED`), laisser un indice tomber (`QCM_HINT`) | La proposition invalidée devient **grisée et barrée**, sans disparaître | | |
| 5 | Vérifier la proposition invalidée par rapport à la TV (`/tv`) | Même proposition grisée des deux côtés | | |
| 6 | Vérifier qu'aucune proposition n'est marquée correcte pendant `STARTED` | Toujours aucun liseré vert / coche | | |
| 7 | Presser **STOP** (phase `STOPPED`) | Propositions inchangées (aucune bonne réponse marquée) | | |
| 8 | Presser **RÉPONSE** (phase `REVEALED`) | La proposition correspondant à `QCM_CORRECT` reçoit un **liseré vert + coche** ; les 3 autres restent en retrait | | |
| 9 | Vérifier qu'**aucun** bloc « Réponse » (texte libre) n'apparaît pour cette question QCM | Le bloc « Réponse » du scénario 2 est réservé aux questions hors QCM | | |
| 10 | Observer le titre de la question suivante pendant toute la manche | Toujours au format GATE 2, non affecté par la phase de la question QCM en cours | | |
| 11 | Observer la zone équipes (couleur de réponse par équipe, ✓/✗, crédit) | Comportement #157 inchangé | | |
| 12 | Rejouer la manche en 1024×768 (tablette plus petite) à l'étape la plus haute (QCM en `STARTED`, indice tombé) | Zone conduite et **au moins 3 cartes équipe** restent entièrement visibles, aucun scroll | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Non-régression

**Objectif** : Vérifier qu'aucune fonctionnalité existante de `/anim`, `/admin`, `/tv` n'est cassée par #163/#164.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `go build ./...` puis `go test ./... -race` | Build OK, tous les tests PASS (y compris `TestStartupPages_*`) | | |
| 2 | `npm test` (suite React complète) | Tous les tests PASS, y compris `AnimPage.test.jsx` et `AnimQcmOptions.test.jsx` (nouveaux) | | |
| 3 | Sur `/anim`, vérifier statut de connexion, badge catégorie, `#ID`, type, chronomètre | Inchangés (#155) | | |
| 4 | Sur `/anim`, vérifier zone conduite SPEEDY (LANCER/PAUSE/STOP/RÉPONSE/À suivre) | Inchangée (#156) | | |
| 5 | Sur `/anim`, vérifier crédit par équipe et couleur de réponse QCM en zone C | Inchangés (#157) | | |
| 6 | Sur `/admin`, vérifier l'aperçu TV embarqué (`QuestionPreview.jsx`) | Inchangé, toujours fonctionnel | | |
| 7 | Sur `/tv`, dérouler une manche SPEEDY et une manche QCM | Comportement TV inchangé (affichage à partir de `STARTED`, pas avant) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation Globale

- [ ] #164 : 4 onglets ouverts au démarrage (5 en debug), `/admin` inclus, ordre respecté
- [ ] #164 : libellé `/anim` corrigé (n'annonce plus "admin")
- [ ] #164 : `auto_open_browsers: false` désactive toujours tout
- [ ] #163 : énoncé de la question visible dès `READY`, sur toutes les phases
- [ ] #163 : titre de la question suivante au format exact GATE 2, sur toutes les phases
- [ ] #163 : 4 propositions QCM visibles dès que la question est chargée, couleurs/lettres correctes
- [ ] #163 : proposition invalidée grisée/barrée sans disparaître
- [ ] #163 : bonne réponse QCM marquée **uniquement** en `REVEALED`
- [ ] #163 : réponse attendue hors QCM affichée **uniquement** en `REVEALED`
- [ ] #163 : aucun débordement en 1280×800 et 1024×768, cibles tactiles ≥ 62 px
- [ ] Aucune régression #155/#156/#157 sur `/anim`, `/admin`, `/tv`

---

## Notes QA

[Espace pour observations]
