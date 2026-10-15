# Procédure de Test — #279 (LOT-6) : plus d'indices sur les cartes QCM d'une question MEMOTION

**Version** : 11.2.2 (LOT-6)
**Date** : 2026-10-15
**Testeur** : utilisateur (QA n'exécute pas les procédures manuelles — pas de navigateur en session agent)
**Automatique associé** : `server-go/internal/server/motion_card_hints_lot6_test.go` (200 et `question.json` sans les 5 clés, NR hôte QCM), `QcmAnswersEditor.showHints.test.jsx`, `QuestionsPage.card-hints-lot6.test.jsx`, `motionCardLock.test.js` (adapté)
**Pourquoi un test manuel** : les tests automatiques ne voient ni le rendu réel de l'éditeur, ni l'écran `/tv` et `/anim` pendant une carte jouée en entier, ni le fichier `question.json` sur disque, ni le crédit réel de points.
**Contrat** : `contracts/question-types.md` §7.1 · décision utilisateur du 2026-10-15 : une carte QCM n'a pas d'indices (« pas d'indice sans effet sur les points »)
**Scénario remplacé** : le scénario 2 de `memotion-qcm-card-185.md` (« Indices QCM sur le timer de carte ») est retiré

## Ce qui change (à garder en tête)

- Le sous-éditeur QCM d'**une carte** MEMOTION n'a plus la case « Activer les indices progressifs » ni les seuils / pénalités.
- Le serveur **ignore et retire** à l'enregistrement les 5 réglages d'indices d'une carte QCM (`QCM_HINTS_ENABLED`, `QCM_HINT_THRESHOLD_1`, `QCM_HINT_THRESHOLD_2`, `QCM_PENALTY_1`, `QCM_PENALTY_2`) : réponse **HTTP 200**, aucune erreur, rien de persisté.
- Pendant une carte QCM, plus aucune réponse n'est invalidée (grisée) sur `/tv` ni `/anim`, quelle que soit la carte stockée.
- Une **question QCM classique** (hors carte) garde ses indices, sa pénalité de mauvaise réponse (#254) et son commutateur : **strictement inchangée**.
- Une carte enregistrée avant cette version avec des indices s'ouvre avec une ligne « Indices retirés… » et se ré-enregistre sans eux.

## Prérequis

- [ ] Environnement : QUALIF, binaire du LOT-6 déployé
- [ ] Navigateur de bureau (Chrome ou Edge) sur `<QUALIF>/admin/quiz`, console (F12) ouverte : noter toute erreur rouge
- [ ] Trois onglets pour le scénario 2 : `/admin` (régie), `/anim` (tablette), `/tv` (affichage) ; au moins 2 équipes
- [ ] Accès au dossier de données (`data/files/questions/<id>/question.json`) et à `curl` pour les scénarios 3 et 4
- [ ] Une question QCM **classique** avec indices activés (créée au scénario 1 ou existante) pour le scénario 5
- Notation : `<QUALIF>` = URL de base de l'instance QUALIF

---

## Scénario 1 — Éditeur : carte QCM sans bloc d'indices, question QCM avec
**Objectif** : l'éditeur ne propose plus d'indices pour une carte et les propose toujours pour une question.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer une question MEMOTION, passer la carte 1 en type **QCM** | Quatre réponses colorées A à D et le bouton ○ de désignation de la bonne réponse sont présents | | |
| 2 | Chercher la case « Activer les indices progressifs » dans la carte | **Absente** ; aucun texte « Pas d'indices automatiques », aucun réglage « Indice 1 / Indice 2 » | | |
| 3 | Saisir une réponse, puis désigner la bonne réponse, compléter la carte et enregistrer | Enregistrement accepté | | |
| 4 | Passer la carte 2 en QCM | Même absence de bloc d'indices | | |
| 5 | Créer une question **QCM classique** (hors MEMOTION) | La case « Activer les indices progressifs » est présente ; la cocher affiche « Indice 1 » et « Indice 2 » avec leurs seuils et pénalités | | |
| 6 | Dans `question.json` de la manche MEMOTION, regarder la carte QCM | Aucune clé `QCM_HINTS_ENABLED`, `QCM_HINT_THRESHOLD_1`, `QCM_HINT_THRESHOLD_2`, `QCM_PENALTY_1`, `QCM_PENALTY_2` ; `QCM_ANSWERS` et `QCM_CORRECT` présents | | |
| 7 | Verrou de type : une carte QCM **neuve** (rien saisi) | Les boutons de type (Speedy, Memory, Rafale) restent actifs | | |
| 8 | Saisir une réponse (ou désigner la bonne) | Les boutons de type se verrouillent (« Videz les 4 réponses QCM… ») : comportement inchangé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Jouer une carte QCM jusqu'au bout : aucune réponse grisée (`critical`)
**Objectif** : pendant toute la carte, aucune réponse n'est invalidée et le crédit correspond au barème de la carte.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer la manche MEMOTION avec une carte QCM, chrono de carte d'au moins **30 s** ; sélectionner la carte | Les quatre réponses s'affichent sur `/tv` et `/anim` | | |
| 2 | Démarrer la carte et regarder `/tv` et `/anim` pendant **toute la durée du chrono**, y compris autour de 25 % et 12,5 % du temps restant | **Aucune** réponse n'est grisée, barrée ou retirée, à aucun moment | | |
| 3 | Laisser le chrono aller à zéro (ou arrêter la carte) puis révéler | Les quatre réponses restent affichées ; la bonne est mise en évidence à la révélation | | |
| 4 | Observer le bouton de crédit sur `/anim` | Le montant est celui de la carte : ★ (1/3/5 par défaut) ou `VALUE` si un « Montant fixe » a été choisi (#190) — **aucune pénalité d'indice** | | |
| 5 | Créditer l'équipe | Les points crédités correspondent exactement au montant affiché | | |
| 6 | Rejouer avec une carte SPEEDY de la même manche | Comportement inchangé | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Question créée AVANT LOT-6 avec une carte QCM à indices (`critical`)
**Objectif** : une question historique à indices s'ouvre, se joue et se ré-enregistre sans erreur ni indice.
**Préparation** : fabriquer la question par `curl` (le nouvel éditeur ne peut plus en produire). Envoyer un `POST <QUALIF>/questions` multipart (`type=MEMOTION`, `question=Héritée`, `answer=`, `points=1`, `time=30`, `motion_cards=` deux cartes : une SPEEDY complète et une QCM avec `"QCM_HINTS_ENABLED":true,"QCM_HINT_THRESHOLD_1":0.25,"QCM_HINT_THRESHOLD_2":0.125,"QCM_PENALTY_1":0.67,"QCM_PENALTY_2":0.33`, quatre réponses et `QCM_CORRECT`). **Cette requête est elle-même acceptée (200) et nettoie la carte** (voir scénario 4) : pour obtenir une VRAIE carte héritée, restaurer une sauvegarde antérieure à la version, ou éditer à la main `question.json` pour y remettre les 5 clés, puis recharger la liste des questions.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir la question héritée dans l'éditeur | S'ouvre **sans erreur** (console propre) | | |
| 2 | Observer la carte QCM | Une ligne d'information « Indices retirés : une carte MEMOTION n'a plus d'indices » ; aucune case d'indices ; les 4 réponses et la bonne réponse désignée sont **intactes** | | |
| 3 | Observer les autres cartes | Pas de ligne d'information sur les cartes SPEEDY | | |
| 4 | Jouer la manche et sélectionner la carte QCM héritée | Les quatre réponses restent affichées jusqu'au bout, **aucun indice** (même si le fichier contient encore les clés) | | |
| 5 | Créditer l'équipe | Montant = ★ de la carte (ou `VALUE` #190), sans pénalité | | |
| 6 | Ré-enregistrer la question sans rien modifier | Enregistrement accepté (200), aucun message d'erreur | | |
| 7 | Rouvrir `question.json` | Les 5 clés d'indices ont **disparu** de la carte ; réponses, désignation, thème et difficulté conservés | | |
| 8 | Rouvrir la question dans l'éditeur | Plus de ligne d'information (la carte est propre) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Appel API direct avec indices sur une carte : ignoré silencieusement
**Objectif** : le serveur accepte et nettoie, sans erreur (option « ignorer silencieusement »).
**Note** : étape en ligne de commande, pas dans l'éditeur.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | `POST <QUALIF>/questions` multipart (`type=MEMOTION`, `question=API`, `answer=`, `points=1`, `time=30`, `motion_cards=` une carte QCM complète avec `"QCM_HINTS_ENABLED":true` et les 4 réglages) | Réponse **HTTP 200** ; aucun message d'erreur ; **aucun** code `CARD_HINTS_NOT_ALLOWED` | | |
| 2 | Ouvrir `data/files/questions/<id>/question.json` | La carte ne contient **aucune** des 5 clés d'indices ; ses réponses, sa désignation et son thème sont conservés | | |
| 3 | Même requête avec `"QCM_HINTS_ENABLED":false` et des seuils non défaut | HTTP 200 ; les 5 clés absentes du fichier | | |
| 4 | Même requête avec une carte **SPEEDY** portant `"QCM_PENALTY_1":0.5` | HTTP 400 `CARD_TYPE_CONTENT_MISMATCH` (refus historique, inchangé) ; rien d'écrit | | |
| 5 | Jouer la carte de l'étape 1 | Aucun indice pendant la carte | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Non-régression : question QCM classique avec indices (`critical`)
**Objectif** : l'hôte QCM garde ses indices, ses pénalités et son commutateur.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer (ou ouvrir) une question QCM classique avec « Activer les indices progressifs » cochée, seuils 25 % et 12,5 %, enregistrer, rouvrir | Case cochée, seuils et pénalités restaurés ; `question.json` : `QCM_HINTS_ENABLED: true` et les 4 réglages présents | | |
| 2 | Lancer la question (durée ≥ 20 s) | Aucune réponse invalidée au démarrage | | |
| 3 | Atteindre le premier seuil (25 %) | Une réponse **fausse** s'invalide sur `/tv` et `/anim`, jamais la bonne | | |
| 4 | Atteindre le second seuil (12,5 %) | Une **deuxième** réponse fausse s'invalide, la première le reste | | |
| 5 | Vérifier le commutateur / la pénalité de mauvaise réponse (#254) | Champ « Pénalité mauvaise réponse » présent et fonctionnel, comme avant | | |
| 6 | Créditer une bonne réponse après un indice | Les points sont diminués selon les pénalités d'indice (67 % puis 33 % par défaut), comme avant la version | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Scénario 1 : aucun bloc d'indices sur une carte, bloc présent sur une question QCM
- [ ] **Scénario 2 (`critical`)** : aucune réponse grisée pendant une carte QCM complète, crédit sans pénalité
- [ ] **Scénario 3 (`critical`)** : question historique à indices ouverte, jouée et ré-enregistrée sans les 5 clés
- [ ] Scénario 4 : HTTP 200 avec nettoyage, aucun code d'erreur nouveau
- [ ] **Scénario 5 (`critical`)** : question QCM classique avec indices strictement inchangée
- [ ] Le scénario 2 de `memotion-qcm-card-185.md` n'est plus exécuté (retiré LOT-6)

## Notes QA

[Espace pour observations]
