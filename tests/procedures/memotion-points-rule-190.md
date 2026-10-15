# Procédure de Test — #190 : barème `POINTS_RULE` par carte MEMOTION (ligne « Points : »)

**Version** : 11.2.2 (LOT-5)
**Date** : 2026-10-15
**Testeur** : utilisateur (QA n'exécute pas les procédures manuelles — pas de navigateur en session agent)
**Automatique associé** : `server-go/internal/server/motion_card_points_rule_190_test.go` (refus HTTP 400 et acceptations), `motionGrid.pointsRule.190.test.js` (barème, cas nommés identiques au Go), `PointsRuleControl.test.jsx`, `QuestionsPage.points-rule.190.test.jsx`, `AnimMotionCard` / `AnimConductPanel` / `GamePage` / `AnimPage` `.points-rule.190.test.jsx`
**Pourquoi un test manuel** : les tests automatiques ne voient ni le rendu réel de la ligne « Points : » dans l'éditeur (thème sombre de la carte MEMOTION), ni le crédit réel de points en partie, ni le contenu du fichier `question.json` sur disque, ni la rejouabilité d'une vraie question historique.
**Maquette de référence** : `docs/mockup/v11.2.2/ui/admin_question_editor__memotion_card_points_rule.html`

## Ce qui change (à garder en tête)

Chaque carte d'une question MEMOTION a une ligne **« Points : »**, juste sous « Valeur de la carte : ». Les options dépendent du type de la carte :

| Type de carte | Options proposées | Défaut (= rien d'écrit dans la carte) |
|---------------|-------------------|----------------------------------------|
| Speedy, QCM | Étoiles (défaut) · Montant fixe | Étoiles |
| Memory, Rafale | Prorata des étoiles (défaut) · Étoiles pleines (tout ou rien) · Montant fixe · Par unité | Prorata des étoiles |

- **Montant fixe** : la carte vaut N points quelle que soit sa valeur ★ (N entier de 1 à 999, obligatoire).
- **Par unité** : N points × le nombre de paires trouvées (Memory) ou de bonnes réponses (Rafale).
- Choisir le **défaut** n'écrit **jamais** `POINTS_RULE` dans la carte.
- Le serveur **refuse** à l'enregistrement (HTTP 400) un barème que l'éditeur ne propose pas.
- Les **grilles** anim et TV restent en ★ seules (décision) : le montant n'y est pas affiché.

## Prérequis

- [ ] Environnement : QUALIF, binaire du LOT-5 déployé
- [ ] Navigateur de bureau (Chrome ou Edge) sur `<QUALIF>/admin/quiz`, console (F12) ouverte : noter toute erreur rouge
- [ ] Une tablette ou un second onglet sur `<QUALIF>/anim` pour les scénarios 2 à 4
- [ ] Un onglet `<QUALIF>/admin` (page Jeu) pour suivre le panneau de la carte
- [ ] Au moins 2 équipes et un buzzer (ou VJoueur) pour jouer une manche et voir les points crédités
- [ ] Accès au dossier de données du serveur (`data/files/questions/<identifiant>/question.json`) pour le scénario 1, et à `curl` pour le scénario 7
- [ ] Une question MEMOTION **historique** (existante avant cette version) pour le scénario 8
- Notation : `<QUALIF>` = URL de base de l'instance QUALIF

---

## Scénario 1 — Carte SPEEDY : 2 options, défaut = rien d'écrit
**Objectif** : le défaut ne laisse aucune trace dans la carte.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer une question MEMOTION, deux cartes SPEEDY avec thème et valeur ★★ | Chaque carte affiche, sous « Valeur de la carte : », la ligne « Points : » | | |
| 2 | Ouvrir la liste déroulante « Mode de barème » d'une carte | Exactement 2 options : « Étoiles (défaut) » et « Montant fixe » | | |
| 3 | Observer l'aide à droite | « → 3 pts (★★) » | | |
| 4 | Laisser le défaut sur les deux cartes, enregistrer | Enregistrement accepté | | |
| 5 | Rouvrir la question | « Étoiles (défaut) » toujours sélectionné sur les deux cartes | | |
| 6 | Ouvrir `data/files/questions/<id>/question.json` | Aucune clé `POINTS_RULE` dans aucune carte | | |
| 7 | Choisir « Montant fixe » puis revenir à « Étoiles (défaut) » avant d'enregistrer, ré-enregistrer | Toujours aucune clé `POINTS_RULE` dans `question.json` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Carte QCM « Montant fixe 10 » (`critical`)
**Objectif** : un montant fixe est écrit, rechargé, affiché partout et crédité tel quel.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Sur une carte de type QCM, valeur ★, choisir « Montant fixe » | Un champ « Montant » apparaît (1 à 999) ; l'aide indique « → N pts, quelle que soit la valeur ★ » | | |
| 2 | Saisir 10 | L'aide devient « → 10 pts, quelle que soit la valeur ★ » | | |
| 3 | Compléter les 4 réponses QCM, enregistrer, recharger la page et rouvrir | « Montant fixe » sélectionné, montant 10 | | |
| 4 | Dans `question.json`, repérer la carte | `"POINTS_RULE": {"MODE": "FIXED", "VALUE": 10}` | | |
| 5 | Lancer la manche MEMOTION et sélectionner cette carte (sous-phase « carte sélectionnée ») | Le panneau de `/admin`, le bandeau de `/anim` et la carte de `/anim` affichent **10 pts** (et non 1 pt) ; les étoiles ★ restent affichées | | |
| 6 | Jouer la carte jusqu'à la révélation | Le bouton de crédit de `/anim` affiche **+10 pts** | | |
| 7 | Créditer l'équipe | L'équipe gagne **10 points**, quelle que soit la valeur ★ de la carte | | |
| 8 | Sur une autre partie, perdre cette carte | Aucun point crédité | | |
| 9 | Regarder la grille des cartes sur `/anim` et sur la TV | Étoiles ★ seules, aucun montant par carte | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Carte MEMORY : 4 options, « Par unité 2 » et « Étoiles pleines » (`critical`)
**Objectif** : les barèmes de la carte Memory sont écrits, affichés et crédités correctement.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer une carte de type Memory (3 paires minimum) | Quatre options : « Prorata des étoiles (défaut) », « Étoiles pleines (tout ou rien) », « Montant fixe », « Par unité » ; le défaut est sélectionné | | |
| 2 | Observer l'aide au défaut | « → jusqu'à N pts, au prorata des paires trouvées (3 paires) » | | |
| 3 | Choisir « Par unité » et saisir 2 | Champ « Points par unité » ; aide « → 2 pts × paires trouvées » | | |
| 4 | Enregistrer, recharger, rouvrir | « Par unité », montant 2 conservés ; `question.json` : `"POINTS_RULE": {"MODE": "PER_UNIT", "VALUE": 2}` | | |
| 5 | Jouer la carte, trouver **3 paires**, aller à la révélation | Sur `/anim` le bouton de crédit affiche **+6 pts** ; le libellé de la carte indiquait « 2 pts / paire » | | |
| 6 | Créditer l'équipe | L'équipe gagne **6 points** | | |
| 7 | Changer le barème en « Étoiles pleines (tout ou rien) », enregistrer ; jouer la carte et ne trouver qu'**une partie** des paires | Le bouton affiche la valeur étoiles **pleine** (ex. +5 pts pour ★★★), pas un prorata ; le crédit est identique | | |
| 8 | Remettre « Prorata des étoiles (défaut) », enregistrer, jouer en trouvant une partie des paires | Le montant est au prorata (comportement historique) ; `question.json` ne contient plus de `POINTS_RULE` pour cette carte | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Carte RAFALE « Par unité 2 » et défaut Prorata inchangé
**Objectif** : la carte Rafale crédite VALUE × bonnes réponses ; son défaut ne change pas (non-régression #217).

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Créer une carte de type Rafale (catégories, difficultés, réservoir suffisant) | Mêmes 4 options que Memory | | |
| 2 | Choisir « Par unité » et saisir 2 | Aide « → 2 pts × bonnes réponses » | | |
| 3 | Enregistrer, jouer la carte, donner **5 bonnes réponses** | Le bouton de crédit affiche **+10 pts** ; le crédit réel est de **10 points** | | |
| 4 | Jouer une autre carte Rafale laissée au défaut (Prorata) | Montant au prorata des bonnes réponses sur les questions posées, comme avant la version | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Changement de type de carte
**Objectif** : une règle devenue inapplicable est remise au défaut avec un avertissement ; une règle encore applicable est conservée.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Carte de type Rafale, « Par unité » 2, puis cliquer sur le type **Speedy** | Le barème repasse à « Étoiles (défaut) », le champ de montant disparaît, un **avertissement bref** explique la remise au défaut | | |
| 2 | Enregistrer cette carte | Aucune clé `POINTS_RULE` dans `question.json` | | |
| 3 | Carte de type Memory, « Montant fixe » 10, puis cliquer sur le type **QCM** | « Montant fixe » **conservé**, montant 10 conservé, aucun avertissement | | |
| 4 | Carte Memory « Étoiles pleines », puis type **Speedy** | Barème « Étoiles (défaut) » sans avertissement (même résultat) | | |
| 5 | Carte Speedy au défaut, puis type **Memory** | Le défaut suit le type : « Prorata des étoiles (défaut) » | | |
| 6 | Après un choix de barème, vérifier que les boutons de type restent actifs | Le barème ne verrouille jamais le type de la carte | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Saisie invalide bloquée dans l'éditeur
**Objectif** : un montant manquant ou hors bornes empêche l'enregistrement, sans message obscur.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | « Montant fixe » **sans** saisir de montant, cliquer sur enregistrer | Message « Indiquez un montant (1 à 999) — la carte ne peut pas être enregistrée sans. » sur la carte concernée ; champ signalé en erreur ; **aucun enregistrement** | | |
| 2 | Saisir 0, enregistrer | Même blocage | | |
| 3 | Saisir 1000, enregistrer | Même blocage | | |
| 4 | « Par unité » sans montant, enregistrer | Message « Indiquez un montant par unité (1 à 999)… », aucun enregistrement | | |
| 5 | Corriger avec 12 et ré-enregistrer | Enregistrement accepté ; l'erreur a disparu | | |
| 6 | Saisir 1, puis 999 sur deux cartes, enregistrer | Les deux valeurs limites sont acceptées | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 7 — Refus serveur (requête forgée)
**Objectif** : le serveur refuse un barème que l'éditeur ne propose pas, et n'écrit rien.
**Note** : cette étape se fait en ligne de commande, pas dans l'éditeur (l'éditeur ne permet pas de forger ces cas).

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Noter le contenu de `data/files/questions/` | Liste de référence | | |
| 2 | Envoyer un `POST <QUALIF>/questions` multipart (`type=MEMOTION`, `question=Test`, `answer=`, `points=1`, `time=30`, `motion_cards=` une carte SPEEDY avec `"POINTS_RULE":{"MODE":"PER_UNIT","VALUE":2}`) | Réponse **HTTP 400** dont le corps nomme `POINTS_RULE_MODE_NOT_APPLICABLE` | | |
| 3 | Même requête avec `"POINTS_RULE":{"MODE":"BONUS"}` | HTTP 400 `POINTS_RULE_INVALID_MODE` | | |
| 4 | Même requête avec `"POINTS_RULE":{"MODE":"FIXED"}` (sans VALUE), puis avec `"VALUE":1000`, puis `"VALUE":2.5` | HTTP 400 `POINTS_RULE_VALUE_REQUIRED` à chaque fois | | |
| 5 | Requête avec deux cartes dont la **seconde** est invalide | HTTP 400 ; **aucune** des deux cartes n'est enregistrée | | |
| 6 | Comparer `data/files/questions/` à l'état de l'étape 1 | Strictement identique : aucune question, aucune image orpheline | | |
| 7 | Requête valide de contrôle : `"POINTS_RULE":{"MODE":"FIXED","VALUE":10}` sur une carte QCM complète | HTTP 200 ; la question apparaît dans la liste | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 8 — Non-régression : question MEMOTION historique (`critical`)
**Objectif** : aucune question existante n'est refusée, modifiée ni rejouée différemment.

| Etape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Ouvrir une question MEMOTION antérieure à cette version (cartes sans `TYPE` ni `POINTS_RULE`) | S'ouvre sans erreur ; chaque carte affiche « Étoiles (défaut) » (SPEEDY) ou le défaut de son type | | |
| 2 | Si une carte historique porte un `POINTS_RULE` explicite (ex. une carte Memory créée avant la version) | Barème explicite restauré tel quel dans l'éditeur | | |
| 3 | Ré-enregistrer **sans rien modifier** | Enregistrement accepté ; `question.json` identique (aucune clé `POINTS_RULE` ajoutée) | | |
| 4 | Rejouer une manche avec cette question | Points crédités exactement comme avant la version (barème étoiles ; Memory et Rafale au prorata) | | |
| 5 | Observer la grille des cartes sur `/anim` et sur la TV (statique, sans défilement) | Étoiles ★ seules, aucune cellule débordante, affichage identique à la version précédente | | |
| 6 | Générer une question MEMOTION+ par l'IA, l'enregistrer | Acceptée (la génération n'émet jamais de `POINTS_RULE`) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] **Scénario 2 (`critical`)** : Montant fixe écrit, rechargé, affiché sur `/admin` et `/anim` et crédité tel quel
- [ ] **Scénario 3 (`critical`)** : Memory « Par unité » et « Étoiles pleines » corrects ; retour au défaut = rien d'écrit
- [ ] **Scénario 8 (`critical`)** : question historique ouverte, ré-enregistrée et rejouée à l'identique
- [ ] Le défaut n'écrit jamais `POINTS_RULE` (scénarios 1, 3, 5)
- [ ] Le serveur refuse les trois cas du contrat sans rien écrire (scénario 7)
- [ ] Les grilles anim et TV restent en ★ seules (scénarios 2 et 8)
- [ ] Aucune régression sur le sélecteur « Valeur de la carte : » (LOT-4)

## Notes QA

[Espace pour observations]
