# Contrat — Présélection des gagnants au crédit (#321, v13.1.0)

**Statut** : contrat de référence (contract-first), **révision 1** (2026-11-24, GATE 2).
**Périmètre** : **frontend seul**. Aucun champ `GameState`, aucune action, aucun endpoint n'est ajouté : toutes les
données existent déjà (bumpers, `GameState`, question).
**Arbitrages** : GATE 2 du 2026-11-24 (P3, P3-bis, P3a, P3d). Plan : `_work/reports/plan-20261124-v1310.md`.

## 1. Principe

- Aujourd'hui, à l'entrée en `REVEALED`, la page Jeu admin **vide** la sélection d'attribution (`awardSelected`,
  `GamePage.jsx`). L'admin coche lui-même chaque équipe ou chaque joueur à créditer.
- Désormais, une **règle unique** `utils/awardPreselect.js` calcule les unités que la question identifie. La page
  les **coche d'office** au même moment.
- Le **montant** de chaque unité ne change pas : c'est la règle existante (`defaultTeamAward` /
  `resolvePointsAward`, `utils/pointsAward.js`).
- Le crédit reste **manuel** : rien n'est envoyé tant que l'admin n'a pas cliqué « Donner les points ».

```js
// Signature [planner] — fonction pure, testée en table
preselectAwardKeys({ question, gameState, bumpers, awardLevel, awarded, amountOf }) → string[]
// awardLevel : 'TEAM' | 'PLAYER' (resolveAwardLevel) ; clés = noms d'équipe ou MAC de bumper
// awarded    : clés déjà créditées (AWARDED_TEAMS / AWARDED_PLAYERS, équipe créditée au niveau équipe comprise)
// amountOf   : (clé) → montant par défaut de la page
```

**Règles communes** :
- on ne coche **jamais** une unité déjà créditée ;
- on ne coche **jamais** une unité dont le montant par défaut est **≤ 0** (montant nul ou pénalité) ;
- la sélection reste modifiable (cocher, décocher, « Tout », « Aucune ») ;
- elle est recalculée à chaque entrée en `REVEALED` et vidée à chaque changement de question ou de phase
  (comportement existant) ;
- une modification à la main du champ « Points » après l'entrée en `REVEALED` ne recoche ni ne décoche rien.

## 2. Règle par type

| Type / mode | Unités cochées | Donnée source |
|---|---|---|
| `PICTO` (Picto, un mot) | `PICTO_TEAM`, si `PICTO_OUTCOME = FOUND` | `GameState` |
| `RAFALE_PICTO` (Rafale Picto) | `PICTO_TEAM`, si `PICTO_ROUND_POINTS > 0` | `GameState` (`picto.md` §17.4) |
| `MEMORY`, SOLO | l'équipe de `MEMORY_PARTICIPATING_TEAMS` | `GameState` |
| `MEMORY`, multi-équipes | les équipes de `MEMORY_PARTICIPATING_TEAMS` de score > 0 (`calcMemoryScore`) | `MEMORY_TEAM_PAIRS` / `MEMORY_TEAM_ERRORS` |
| `RAFALE` (Rafale Speedy), SOLO | l'équipe de `RAFALE_PARTICIPATING_TEAMS`, si son compteur retenu est > 0. **Le choix « équipe qui joue » existe déjà** : en SOLO, une équipe est exigée pour `READY` (#201, `participantsConform`, sélecteur d'équipe de la page Jeu) | `RAFALE_TEAM_COUNTERS` |
| `RAFALE`, multi-équipes | les équipes de `RAFALE_PARTICIPATING_TEAMS` de compteur retenu > 0 (`rafaleCounterForTeam`) | `RAFALE_TEAM_COUNTERS` / `RAFALE_TEAM_BEST` |
| `QCM` | les unités qui ont donné `QCM_CORRECT` : équipes au niveau équipe, joueurs au niveau joueur. Équivaut à `pickQcmAnswer(QCM_CORRECT)` existant (couleur marquée, `awardWrong = false`, champ « Points » sur la base du barème). Aucune bonne réponse : rien n'est coché | bumpers (`answeredColor`, `hasAnswered`) |
| `SPEEDY` | le **premier buzzeur**, c'est-à-dire le bumper de plus petit `TIME > 0` (ordre de `utils/buzzOrder.js`) : le joueur au niveau joueur, son équipe au niveau équipe. **L'admin décoche** si la réponse était fausse. Aucun buzz : rien n'est coché | bumpers |
| `ARDOISE` | les équipes dont la réponse **repliée** est égale à la réponse attendue `ANSWER` repliée (§3). `ANSWER` vide : rien n'est coché | réponses ARDOISE de l'admin, `question.ANSWER` |
| `MEMOTION` | voir §4 (gagnant de **carte**, pas la colonne Équipes) | — |
| `ROUE` | rien (crédit automatique `ROUE_AWARDS`) | — |
| `ENTRACTE` | rien (aucun point) | — |

## 3. Repli de comparaison ARDOISE

`fold(s)` = décomposition NFD et suppression des diacritiques, puis minuscules, espaces de début et de fin retirés,
espaces internes compactés en un seul. C'est le même repli que `rafaleAlphaKey` (Go), sans le filtrage des
caractères. Les cas de test sont nommés à l'identique en JS : `fold_accents`, `fold_case`, `fold_spaces`, `fold_empty`.

Exemple : « Éléphant » = « elephant » = « ÉLÉPHANT » ; « Le Louvre » ≠ « Louvre ».

## 4. MEMOTION — gagnant de carte présélectionné (P3d)

- À l'étape de validation d'une carte (`MEMOTION_SUBPHASE = REVEAL`, avant `MEMOTION_DONE`), le sélecteur de gagnant
  part sur **l'équipe qui a la main** (`MEMOTION_CURRENT_TEAM`), au lieu d'aucune sélection. Cela vaut sur l'admin
  et sur l'anim.
- Tous les modes sont concernés (SOLO compris : l'équipe unique a la main).
- « Personne » et les autres équipes restent sélectionnables. Rien n'est envoyé avant le clic de validation
  existant.
- Le payload de `MEMOTION_DONE {CARD_ID, WINNER_TEAM, UNITS}` est **inchangé**.

## 5. Tests exigés

- Table de `preselectAwardKeys` : chaque ligne du §2, plus les cas limites (unité déjà créditée, montant nul ou
  négatif, aucun participant, niveau joueur et niveau équipe, équipe supprimée).
- `GamePage` : la présélection apparaît à l'entrée en `REVEALED`. Les tests existants de l'attribution groupée
  (#244 / #251 / #254 / #289) passent sans modification.
- MEMOTION : le gagnant présélectionné sur l'admin et l'anim reste modifiable ; le payload de `MEMOTION_DONE` est
  inchangé.
- Repli ARDOISE : les 4 cas nommés.
