# Procédure de Test — Couleur du badge nom VJoueur figée après réponse QCM (#112)

**Version** : 5.7.24 (branche `bugfix/vjoueur-team-color`)
**Date** : 2026-07-26
**Branche** : bugfix/vjoueur-team-color
**Testeur** : QA

---

## Contexte du Bug

Sur l'écran VJoueur (`/player`, page `VPlayerPage.jsx`), le badge affichant le nom du joueur
doit toujours être coloré avec la couleur de son équipe (`team.COLOR`), exactement comme
l'admin (`GamePage`, `TeamsPage`) et la TV (`PlayerDisplay` non-VPlayer).

**Symptôme** : dès qu'un VJoueur répond à une question QCM (bouton A/B/C/D), le badge de son
nom bascule **définitivement** sur la couleur de sa réponse (rouge/vert/jaune/bleu) au lieu de
rester sur la couleur de son équipe — et ne revient jamais à la couleur d'équipe, même à la
question suivante ou après un changement d'équipe.

**Cause racine** : le champ `bumper.ANSWER_COLOR` est réutilisé côté backend
(`engine.go`, `ProcessButtonPress`) pour mémoriser la dernière réponse QCM de **tout** bumper
(physique ou VJoueur) et n'est **jamais réinitialisé** entre deux questions. Le frontend
(`VPlayerPage.jsx` → `getPlayerNameColor()`) priorisait à tort ce champ sur `team.COLOR`.

**Fix** : `team.COLOR` prime désormais toujours dès qu'une équipe est assignée ;
`bumper.ANSWER_COLOR` ne sert plus que de repli pour un VJoueur **sans équipe** (mode solo).

Scope du fix strictement limité à `VPlayerPage.jsx` (aucun changement admin/TV/backend).

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `bugfix/vjoueur-team-color`)
- [ ] Une partie avec au moins 2 équipes de couleurs différentes, et au moins une question QCM
      disponible dans le quiz/catégorie utilisée
- [ ] Un appareil mobile (ou second onglet) pour incarner le VJoueur, plus un accès admin
      (`GamePage`/`TeamsPage`) pour vérifier la cohérence des couleurs

---

## Scénario 1 — Réponse à une question QCM : le badge reste sur la couleur d'équipe

**Objectif** : Vérifier le scénario nominal du bug — répondre à une QCM ne doit plus changer
la couleur du badge nom du VJoueur.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Enrôler un VJoueur "Alice", l'assigner à une équipe de couleur connue (ex. rouge) | Badge nom "Alice" affiché en rouge (couleur d'équipe) sur l'écran VJoueur | | |
| 2 | Comparer avec l'admin (`GamePage`/`TeamsPage`) | Même couleur rouge affichée côté admin pour "Alice" | | |
| 3 | Démarrer une question de type QCM | Boutons de réponse A/B/C/D visibles sur l'écran VJoueur | | |
| 4 | Répondre en sélectionnant une couleur **différente** de la couleur d'équipe (ex. bleu si l'équipe est rouge) | Confirmation de réponse affichée (checkmark + couleur de réponse sur l'overlay de confirmation uniquement) | | |
| 5 | Observer le badge nom "Alice" pendant et après la confirmation | Le badge nom reste **rouge** (couleur d'équipe), ne bascule jamais en bleu | | |
| 6 | Révéler la réponse puis passer à la question suivante | Badge nom toujours rouge | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Réponses QCM successives : pas d'accumulation ni de flash de couleur

**Objectif** : Vérifier que plusieurs réponses QCM consécutives (couleurs différentes à
chaque question) ne font jamais dériver le badge nom de sa couleur d'équipe.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | VJoueur "Alice" (équipe rouge) répond à une 1ère QCM en bleu | Badge nom reste rouge | | |
| 2 | Question suivante (QCM) : "Alice" répond en jaune | Badge nom reste rouge (pas de passage par jaune) | | |
| 3 | Question suivante (QCM) : "Alice" répond en vert | Badge nom reste rouge | | |
| 4 | Vérifier qu'aucun flash visuel (même bref) vers une autre couleur n'est visible pendant les transitions | Aucun changement de couleur du badge nom sur toute la séquence | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Changement d'équipe en cours de partie

**Objectif** : Vérifier qu'une réassignation d'équipe par l'admin après une réponse QCM met
bien à jour la couleur du badge sur la nouvelle équipe (pas de blocage sur l'ancienne couleur
ni sur `ANSWER_COLOR`).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | VJoueur "Alice" dans l'équipe rouge, a déjà répondu à une QCM (ex. en bleu) | Badge nom rouge | | |
| 2 | Depuis l'admin (`TeamsPage`/`GamePage`), réassigner "Alice" à l'équipe bleue | — | | |
| 3 | Observer l'écran VJoueur d'"Alice" | Le badge nom passe au **bleu** (nouvelle couleur d'équipe), sans jamais réafficher l'ancienne couleur rouge ni la couleur de réponse QCM précédente | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — VJoueur sans équipe assignée (mode solo / repli résiduel)

**Objectif** : Vérifier le comportement de repli voulu : un VJoueur **non assigné** à une
équipe peut afficher la couleur de sa dernière réponse QCM sur son badge nom (cas résiduel,
différent du bug — ce comportement est acceptable en l'absence de couleur d'équipe).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Enrôler un VJoueur "Bob" sans l'assigner à une équipe | Aucune couleur particulière sur le badge nom (neutre) | | |
| 2 | Démarrer une question QCM, faire répondre "Bob" (ex. en vert) | Le badge nom de "Bob" peut prendre la couleur verte (repli `ANSWER_COLOR`, pas de couleur d'équipe disponible) | | |
| 3 | Assigner ensuite "Bob" à une équipe (ex. jaune) | Le badge nom bascule immédiatement sur le jaune (couleur d'équipe prioritaire dès qu'elle existe) | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Reconnexion (perte réseau) : pas de retour à la couleur de réponse

**Objectif** : Vérifier qu'une coupure réseau suivie d'une reconnexion du VJoueur ne fait pas
réapparaître la couleur de réponse QCM sur le badge nom.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | VJoueur "Alice" (équipe rouge) a déjà répondu à une QCM | Badge nom rouge | | |
| 2 | Couper le réseau/fermer l'onglet d'"Alice" quelques secondes | Badge orange/rouge de déconnexion visible côté admin (cf. #109) | | |
| 3 | Reconnecter "Alice" (rouvrir l'onglet / F5) | Badge nom d'"Alice" toujours rouge (couleur d'équipe) dès l'affichage, jamais la couleur de réponse | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 6 — Non-régression : cohérence admin / TV

**Objectif** : Vérifier qu'aucune régression n'affecte l'affichage des couleurs côté admin et
TV (hors scope du fix, doit rester inchangé).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Faire répondre plusieurs VJoueurs et buzzers physiques à des QCM avec des couleurs variées | — | | |
| 2 | Observer `GamePage` et `TeamsPage` | Couleurs des badges/cartes toujours celles des équipes, comportement inchangé par rapport à avant le fix | | |
| 3 | Observer l'écran TV (`/tv`, vue PLAYERS/scores) | Couleurs toujours celles des équipes, aucune régression | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Scénario 1 : le badge nom VJoueur ne bascule plus sur la couleur de réponse QCM
- [ ] Scénario 2 : stabilité sur plusieurs réponses QCM successives, aucun flash parasite
- [ ] Scénario 3 : un changement d'équipe en cours de partie met à jour la couleur du badge
- [ ] Scénario 4 : repli sur la couleur de réponse accepté uniquement en l'absence d'équipe
- [ ] Scénario 5 : la reconnexion ne fait pas réapparaître la couleur de réponse QCM
- [ ] Scénario 6 : aucune régression sur l'admin (`GamePage`, `TeamsPage`) ou la TV (`/tv`)

---

## Notes QA

[Espace pour observations, capture d'écran, couleurs exactes observées, version du binaire testé, date de test]
