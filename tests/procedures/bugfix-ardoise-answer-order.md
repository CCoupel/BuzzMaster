# Procédure de Test — Tri chronologique des réponses ARDOISE (#117)

**Version** : à définir (branche `bugfix/ardoise-answer-order`)
**Date** : 2026-07-27
**Branche** : bugfix/ardoise-answer-order
**Testeur** : QA

---

## Contexte du Bug

Sur la page admin (`GamePage`), le panneau « Réponses ARDOISE » affichait les équipes dans
l'ordre de la liste d'équipes, sans rapport avec l'ordre réel d'arrivée des réponses, et
n'affichait aucun délai.

**Cause racine (cumulative, cf. plan `_work/reports/plan-20260727-093000.md`)** :
1. **Affichage** : `GamePage.jsx` itérait `vplayerTeams` sans jamais trier.
2. **Horodatage** : `SubmittedAt` était réécrit à *chaque* frappe — trier dessus aurait donné
   l'ordre inverse des dernières retouches, pas l'ordre d'arrivée.
3. **Émission VJoueur (cause invisible)** : l'envoi `ARDOISE_INPUT` était un **debounce** de
   200 ms (pas un throttle) — une équipe écrivant sans pause n'envoyait rien tant qu'elle ne
   s'arrêtait pas, décalant arbitrairement son horodatage réel.

**Fix attendu** :
- Backend : nouveau champ `ArdoiseAnswer.STARTED_AT`, figé au premier caractère non vide reçu
  pour l'équipe et la question courante (`SUBMITTED_AT` continue de suivre la dernière frappe).
- VJoueur : le tout premier caractère non vide d'une question est envoyé **immédiatement**,
  les frappes suivantes restent régulées à ~200 ms.
- Admin (`GamePage`) : le panneau trie les équipes ayant répondu par `STARTED_AT` croissant
  (repli sur `SUBMITTED_AT` si `STARTED_AT` vaut `0`, donnée antérieure au fix), affiche le
  rang et un délai `(STARTED_AT - gameState.TIME) / 1e6`, formaté à **3 décimales** (ex.
  `4.732 s` — décision GATE 2, même convention que les temps de réaction au buzzer).
- **Hors périmètre** : l'affichage TV (`PlayerDisplay.jsx`) n'est pas trié — seule la page
  admin est concernée par cette issue.

Maquette de référence : https://claude.ai/code/artifact/e9e8f569-a9dd-4f17-a128-4b000a7a70a9

---

## Prérequis

- [ ] Environnement : QUALIF (ou LOCAL avec build de la branche `bugfix/ardoise-answer-order`)
- [ ] Au moins 3 équipes configurées, chacune avec un VJoueur (smartphone ou onglet séparé)
- [ ] Une question de type `ARDOISE` disponible dans le quiz
- [ ] Accès admin (`/` ou `/game`) et VJoueur (`/player`) — 3 appareils/onglets VJoueur + 1 admin
- [ ] Un jeu de données de backup antérieur à cette version (pour le scénario 5), ou une
      réponse `ARDOISE_ANSWERS` sans `STARTED_AT` simulée manuellement si aucun backup adéquat
      n'est disponible

---

## Scénario 1 — Trois VJoueurs répondant dans un ordre connu

**Objectif** : Vérifier que le panneau admin classe les équipes par ordre d'arrivée réel du
premier caractère, et affiche un délai cohérent.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question ARDOISE (phase STARTED) avec 3 VJoueurs (Équipe A, B, C) | Panneau « Réponses ARDOISE » visible, 3 lignes sans réponse (« — ») | | |
| 2 | Sur le VJoueur de l'Équipe B, taper une lettre en premier | La ligne de l'Équipe B passe en tête du panneau admin, avec le rang 1 et un délai affiché | | |
| 3 | ~3 secondes plus tard, taper une lettre sur le VJoueur de l'Équipe C | L'Équipe C apparaît en 2ᵉ position (rang 2), l'Équipe B reste en 1ʳᵉ position | | |
| 4 | ~3 secondes plus tard, taper une lettre sur le VJoueur de l'Équipe A | L'Équipe A apparaît en 3ᵉ position (rang 3) ; ordre final : B, C, A | | |
| 5 | Observer le format du délai sur chaque ligne | Délai affiché avec 3 décimales suivies de « s » (ex. `3.142 s`), cohérent avec l'écart entre les frappes | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 2 — Régression cause racine #3 : réponse écrite sans pause

**Objectif** : Vérifier qu'une équipe qui écrit toute sa réponse d'une traite (sans jamais
s'arrêter plus de 200 ms) est bien datée à son **premier caractère**, pas à sa première pause.
C'est le bug le plus subtil de cette tâche — un correctif partiel (backend seul, sans le
changement d'émission VJoueur) le laisserait passer inaperçu au clavier physique (où l'on
marque naturellement des pauses), mais le reproduirait dans un usage réel.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question ARDOISE, phase STARTED, avec au moins 2 équipes | Panneau vide (aucune réponse) | | |
| 2 | Sur le VJoueur de l'Équipe A, taper rapidement et **sans aucune pause** une réponse longue (ex. « CONSTANTINOPLE ») en une seule frappe continue (~1-2 secondes) | La ligne de l'Équipe A apparaît dans le panneau admin **dès la première lettre tapée**, pas seulement à la fin de la frappe | | |
| 3 | Noter le délai affiché pour l'Équipe A à la fin de la frappe | Le délai correspond à l'instant du **premier** caractère (proche du début de la frappe), pas à l'instant de la dernière lettre ni d'une pause | | |
| 4 | Sur le VJoueur de l'Équipe B, attendre 5 secondes puis taper une seule lettre | Le délai de l'Équipe B est nettement supérieur à celui de l'Équipe A, reflétant l'attente réelle | | |
| 5 | Comparer avec le comportement attendu : une équipe rapide et sans hésitation ne doit jamais être pénalisée par rapport à une équipe qui tape lentement | L'Équipe A (rapide, sans pause) reste bien classée avant l'Équipe B | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 3 — Une équipe corrige sa réponse : le rang ne bouge pas

**Objectif** : Vérifier qu'une équipe qui modifie ou complète sa réponse après avoir commencé
à répondre conserve son rang initial (basé sur `STARTED_AT`, jamais réécrit).

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Lancer une question ARDOISE avec 2 équipes | Panneau vide | | |
| 2 | Équipe A tape une première lettre | Équipe A apparaît en rang 1 | | |
| 3 | Équipe B tape une lettre 3 secondes plus tard | Équipe B apparaît en rang 2 | | |
| 4 | Pendant 20 secondes, l'Équipe A efface entièrement sa réponse puis la retape entièrement, plusieurs fois | Le rang de l'Équipe A reste 1 pendant toute la manipulation ; aucune ligne ne change de position | | |
| 5 | Observer le délai affiché pour l'Équipe A pendant et après ces corrections | Le délai n'est jamais réinitialisé à une valeur proche de 0 — il continue d'augmenter depuis le tout premier caractère saisi à l'étape 2 | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 4 — Transition STARTED → STOPPED → REVEALED

**Objectif** : Vérifier que l'ordre et les délais restent identiques à travers les
changements de phase.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Avec au moins 2 équipes ayant répondu dans un ordre connu (phase STARTED) | Ordre et délais affichés conformes à l'ordre d'arrivée | | |
| 2 | Cliquer STOP | Le panneau reste affiché, ordre et délais inchangés, pas de bouton « +N pts » | | |
| 3 | Cliquer « Révéler » (phase REVEALED) | Ordre et délais toujours inchangés ; les boutons « +N pts » apparaissent sur chaque ligne, dans le même ordre | | |
| 4 | Cliquer sur « +N pts » pour l'équipe en tête de liste | Les points sont bien attribués à la **bonne** équipe (celle affichée en rang 1), vérifiable via son score dans `TeamCard` | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Scénario 5 — Backup antérieur au correctif (repli `SUBMITTED_AT`)

**Objectif** : Vérifier qu'une partie chargée depuis un backup créé avant cette version
(réponses `ARDOISE_ANSWERS` sans `STARTED_AT`, donc valant `0`) reste affichable sans erreur.

| Étape | Action | Résultat Attendu | Résultat Obtenu | OK ? |
|-------|--------|-----------------|----------------|------|
| 1 | Charger un backup antérieur à cette version contenant des réponses ARDOISE (`BackupPage`) | Chargement sans erreur console, jeu restauré | | |
| 2 | Naviguer vers l'état ARDOISE restauré (STARTED/STOPPED/REVEALED selon le backup) | Le panneau admin s'affiche sans erreur JS (pas de crash, pas de `NaN` à l'écran) | | |
| 3 | Observer le délai affiché pour ces réponses historiques | **Aucun délai n'est affiché** pour les réponses sans `STARTED_AT` (repli silencieux) | | |
| 4 | Observer l'ordre d'affichage | Les équipes sont triées par `SUBMITTED_AT` (repli), sans crash ni ordre aberrant | | |

**Verdict** : [ ] PASS  [ ] FAIL

---

## Critères de Validation

- [ ] Les réponses sont classées par ordre croissant du premier caractère reçu (scénario 1)
- [ ] Une équipe écrivant sans pause est datée à son premier caractère, pas à sa première pause
      (scénario 2 — non-régression critique)
- [ ] Le rang d'une équipe ne bouge jamais pendant qu'elle corrige sa réponse (scénario 3)
- [ ] L'ordre et les délais sont stables à travers STARTED → STOPPED → REVEALED (scénario 4)
- [ ] Le bouton d'attribution de points cible la bonne équipe après tri (scénario 4)
- [ ] Un backup antérieur reste affichable sans erreur, délais masqués (scénario 5)
- [ ] Le délai est toujours affiché à 3 décimales (`X.XXX s`)
- [ ] Aucune régression sur l'affichage TV (`PlayerDisplay.jsx`, hors périmètre de cette issue)

## Notes QA

[Espace pour observations]
