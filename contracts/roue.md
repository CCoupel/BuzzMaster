# Contrat — Mode de jeu ROUE « Roue de la Fortune » (v12.0.0, milestone #21)

**Issues** : #144 (mécanique cœur) · #145 (contenu : phrases et éditeur)
**Statut** : contrat de référence, écrit avant implémentation (contract-first) — **révision 6 — finale du GATE 1.5** (2026-11-16)
**Arbitrages** : `_work/reports/plan-ambiguities-20261116-112244.md` (Q1–Q4) ; ordres
`_work/tasks/planner-20261116-112200.md` (décisions 1–4), `_work/tasks/planner-20261116-134500.md` (7 étapes d'un
tour, forme de roue et lettres proposées par question, durée maximale de rotation) et
`_work/tasks/planner-20261116-150500.md` (retournement au bouton, commandes de l'animateur, annonces à voix haute,
roue générée à partir de 4 paramètres) ; `_work/tasks/planner-20261116-153500.md` (valeurs toutes différentes, ordre
de la roue, étape 6 à un seul bouton) ; `_work/tasks/planner-20261116-155000.md` (étape 7 à deux boutons, réponse =
bonus, roue définie par maximum et pas) ; `_work/tasks/planner-20261116-160000.md` (PERDU à mi-tour, pas corrigé à
la saisie, « Équipe suivante »)
**Révision 7 — v12.1.0 (milestone #22)** : achat de voyelles (#147) et case BONUS (#308) — **§15**, qui prime sur les
sections 3 à 12 pour ces deux points. Décisions : `.claude/workflow/cadrage-v12.1.0-decisions.md`.
**Hors périmètre** : round bonus final (#146) et round spécial (#148), fermés « not planned ».

> Ce fichier est la **référence unique** pour `dev-backend` et `dev-frontend` sur le mode ROUE.
> En cas de divergence backend/frontend, ce document tranche.
> Le backend PEUT modifier un contrat sous contrainte technique — en documentant la raison ici.

---

## 1. Vocabulaire

| Terme | Définition |
|---|---|
| **Manche ROUE** | Une question de type `ROUE` jouée de `START` jusqu'à la résolution de sa phrase. Une phrase = une question = une manche. |
| **Tour** | Les 7 étapes jouées par l'équipe qui a la main, du lancer de la roue à « Tour suivant » (§5). |
| **Phrase** | Le texte à deviner (`ROUE_PHRASE`). Secret pour la TV et les tablettes jusqu'à la résolution. |
| **Présentation** | Phrase d'indice affichée à côté de la catégorie (`ROUE_PRESENTATION`), ex. « Chanson de Line Renaud ». Publique. |
| **Plateau** | Grille de 4 lignes × 14 colonnes où les lettres de la phrase apparaissent. |
| **Case** | Un segment de la roue : valeur en points, `BANQUEROUTE` ou `TOUR PERDU`. |
| **Cagnotte de manche** | Score temporaire d'une équipe pendant la manche. **Pas un score** tant que la phrase n'est pas résolue (§7). |
| **Main** | L'équipe dont c'est le tour (`ROUE_CURRENT_TEAM`). |

## 2. Décisions d'architecture

1. **`ROUE` est un `QuestionType`, sans nouvelle phase principale** (patron RAFALE / MEMOTION) : le déroulement
   interne est porté par `ROUE_SUBPHASE` pendant `STARTED`. Les modes existants ne sont pas modifiés.
2. **Aucun nouveau stockage** (Q1) : la phrase vit dans `question.json` de la question ; création et modification
   par `POST /questions`, suppression par l'action `DELETE` existante. Sauvegarde, restauration, statut « jouée »,
   manches et compteurs de catégories sont acquis sans code dédié.
3. **Catégorie = catégorie existante de la question** (`CATEGORY`), plus la **présentation** en texte libre (Q2).
4. **Rotation = enum existant** (Q3) : `ROUE_MODE` ∈ `SOLO | CHACUN_SON_TOUR | TANT_QUE_JE_GAGNE`, mêmes valeurs
   et mêmes libellés que `MEMORY_MODE` / `MOTION_MODE` / `RAFALE_MODE`. `MAILLON_FAIBLE` (propre à RAFALE) est
   refusé. Rotation par `rotateTeam` (`internal/game/team_rotation.go`), inchangé.
5. **Le serveur tire la case à l'arrêt demandé** (Q4) : le buzz de l'équipe qui a la main, le bouton « Forcer
   l'arrêt » ou la durée maximale de rotation ne sont que des déclencheurs ; le serveur choisit la case au hasard
   et tous les écrans y amènent la roue. **Aucune horloge à synchroniser** : les clients animent à réception (§6).
6. **Composant `Wheel` réutilisé** : `web/src/components/wheel/` + adaptateur `FortuneWheel` ; le mouvement long
   est implémenté dans le composant (`wheel-component.md` §5). Aucune seconde roue.
7. **La phrase ne quitte jamais le serveur vers la TV ni les tablettes** (§9).
8. **Cases et durées de la roue dans la configuration** (§10), figées dans `GameState` au `START` ;
   **forme de la roue et affichage des lettres proposées sur la question** (§3).

## 3. Modèle de données — question de type `ROUE`

Champs de `TypedContent` (tous `omitempty`), `OwnedFields` du type `ROUE` :

| Champ JSON | Type | Règle |
|---|---|---|
| `ROUE_PHRASE` | string | Obligatoire. Enregistrée telle que saisie (espaces compactés). Voir §3.1. |
| `ROUE_PRESENTATION` | string | Facultative, 80 caractères au plus. |
| `ROUE_MODE` | string | `SOLO` \| `CHACUN_SON_TOUR` \| `TANT_QUE_JE_GAGNE`. Défaut `TANT_QUE_JE_GAGNE`. |
| `ROUE_GAINS_RULE` | string | `WINNER_ONLY` (défaut) \| `ALL_TEAMS`. Voir §7. |
| `ROUE_WHEEL_LAYOUT` | string | `strip` (défaut) \| `arc` — forme de la roue sur la TV pour cette question (mêmes valeurs que `RAFALE_WHEEL_LAYOUT`). |
| `ROUE_SHOW_LETTERS` | bool | Afficher sur la TV et les tablettes les lettres déjà proposées. Défaut `true` ; absent = `true` (le champ n'est écrit que pour `false`). |

Champs communs réutilisés : `CATEGORY`, bloc son de question. `ANSWER`, `POINTS` et `TIME` ne sont **pas** utilisés
par le type `ROUE` (la phrase n'est jamais copiée dans `ANSWER`). Aucun chronomètre global.

Registre (`question-types.md` §7) : `MediaSlots` aucun, `NestableInMotionCard` ❌, `HasPlayerInput` ❌ (le buzz
passe par l'action `BUTTON` existante). Non générable par IA.

### 3.1 Phrase et plateau — règle unique, côté serveur

- **Caractères admis** : lettres (accents admis), chiffres, espace, apostrophe, tiret, `. , ! ? :`. Les caractères
  `_`, `*` et tout autre caractère sont refusés.
- **Affichage** : majuscules, accents repliés (`É` → `E`, `Ç` → `C`, `Œ` → `OE`) — même repli que
  `rafaleAlphaKey` (`internal/game/rafale_alpha.go`), à factoriser, pas à dupliquer.
- **Lettres à trouver** : les 26 lettres. Chiffres et ponctuation sont visibles dès le début.
- **Consonnes** : les 20 lettres hors `A E I O U Y`. Les 6 voyelles ne sont jamais proposables en v12.0 ; elles
  n'apparaissent qu'à la résolution. **v12.1.0** : elles s'achètent (§15.1) ; `ROUE_LETTER` refuse toujours une voyelle.
- **Mise en lignes** : 4 lignes de 14 colonnes au plus ; les mots ne sont jamais coupés ; remplissage glouton
  ligne après ligne. Un mot de plus de 14 caractères ou une phrase qui demande plus de 4 lignes est refusé.
- **Une seule implémentation** de la mise en lignes : serveur. L'éditeur obtient l'aperçu par
  `POST /api/roue/preview` (§11.2) — aucune copie JavaScript de la règle.

Exemple : `Combien pour ce chien dans la vitrine` →
```
COMBIEN POUR
CE CHIEN DANS
LA VITRINE
```

## 4. Champs `GameState`

Tous **sans `omitempty`**, maps et tableaux jamais `null`, **jamais persistés**, remis à leur valeur vide dans
`Ready`, `stopUnsafe` et `InitGameWith`. Diffusés à tous les clients web (admin, anim, TV, tablette) ; jamais aux
buzzers.

| Champ | Type | Contenu |
|---|---|---|
| `ROUE_SUBPHASE` | string | `""` \| `TURN` \| `SPINNING` \| `STOPPING` \| `LETTER` \| `LIT` \| `REVEALING` \| `ANSWER` \| `TURN_END` \| `SOLVED` (§5) |
| `ROUE_STEP` | int | Étape affichée par le bandeau : 1 à 7 (§5), 0 hors tour |
| `ROUE_TEAMS` | string[] | Équipes participantes, dans l'ordre de rotation |
| `ROUE_CURRENT_TEAM` | string | Équipe qui a la main (`""` hors manche) |
| `ROUE_CURRENT_TEAM_COLOR` | couleur (même forme que `RAFALE_CURRENT_TEAM_COLOR`) | Accent de la roue |
| `ROUE_NEXT_TEAM` | string | En `ANSWER` et `TURN_END` : équipe suivante dans la rotation — celle que désigne le bouton « Équipe suivante » (`""` ailleurs et en `SOLO`) |
| `ROUE_NEXT_DEFAULT` | string | En `ANSWER` et `TURN_END` : bouton conseillé, `SAME` (« Tour suivant ») ou `NEXT` (« Équipe suivante ») — §5.2 ; `""` ailleurs |
| `ROUE_TEAM_BANK` | map équipe → int | Cagnotte de manche de chaque équipe participante |
| `ROUE_TURN_GAIN` | int | Gain du tour en cours, pas encore ajouté à la cagnotte (0 sinon) |
| `ROUE_BOARD` | `{ "COLS": 14, "ROWS": string[] }` | Une chaîne par ligne : espace = pas de case, `_` = lettre cachée, `*` = case cachée **éclairée** (sous-phase `LIT` uniquement, `ROUE_STEP = 5`), tout autre caractère = visible (§3.1) |
| `ROUE_LETTERS` | `{ "LETTER": string, "TEAM": string, "HIT": bool }[]` | Consonnes déjà proposées, dans l'ordre : la lettre, l'équipe qui l'a proposée, présente (`true`) ou absente. Sert à colorer les lettres trouvées à la couleur de l'équipe (animateur) et à lister les lettres proposées |
| `ROUE_SHOW_LETTERS` | bool | Copie du réglage de la question : afficher ou non les lettres proposées sur TV et tablettes (admin et anim les affichent toujours) |
| `ROUE_CONSONANTS_LEFT` | bool | Il reste au moins une consonne cachée |
| `ROUE_WHEEL` | objet | Voir ci-dessous |
| `ROUE_LAST_EVENT` | objet | `{ "SEQ": int, "KIND": string, "TEAM": string, "LETTER": string, "COUNT": int, "VALUE": int, "GAIN": int }` — dernier événement, pour les animations et les sons ; **une action (ou une échéance de minuterie) émet au plus un événement** : `SEQ` augmente d'exactement 1 (règle « un événement par action », ci-dessous) |
| `ROUE_WINNER` | string | Équipe qui a résolu (`""` sinon, y compris phrase dévoilée sans gagnant) |
| `ROUE_AWARDS` | map équipe → int | Points crédités à la résolution |

`ROUE_WHEEL` :
```json
{
  "LAYOUT": "strip",
  "SPIN_PERIOD_MS": 2500,
  "SPIN_UP_MS": 5000,
  "SPIN_MAX_MS": 10000,
  "FLIP_INTERVAL_MS": 600,
  "SEGMENTS": [ { "ID": "s0", "KIND": "value", "VALUE": 100 }, { "ID": "s3", "KIND": "bankrupt", "VALUE": 0 } ],
  "CURRENT": -1,
  "FULL_SPEED": false,
  "STOP": null
}
```
- `LAYOUT` : `ROUE_WHEEL_LAYOUT` de la question. `SEGMENTS` et durées : copie de la configuration (§10) prise au
  `START`, inchangée pendant la manche. `KIND` ∈ `value | bankrupt | lose_turn`.
- `CURRENT` : index de la case sous le pointeur une fois la roue arrêtée ; conservé jusqu'au lancer suivant ;
  `-1` avant le premier lancer de la manche. La roue reste affichée en permanence.
- `STOP` : `null`, ou `{ "TO": int, "DURATION_MS": int }` pendant `STOPPING` (case tirée et durée du freinage).
- `SPIN_UP_MS` : durée de la **montée en vitesse**, calculée par le serveur au `START` : `2 × SPIN_PERIOD_MS`
  (5 000 ms par défaut) — c'est la durée qu'il faut, à accélération constante depuis l'arrêt, pour parcourir
  **1 tour** (décision utilisateur, QUALIF 12.0.0.8 ; 2 tours auparavant) et atteindre la vitesse d'un tour en `SPIN_PERIOD_MS` (§6). Pas un réglage : toujours dérivée.
- `FULL_SPEED` : `true` **seulement en `SPINNING`, une fois la montée terminée** ; `false` pendant la montée et
  dans toute autre sous-phase. C'est le seul signal de « pleine vitesse » : le serveur le passe à `true` à
  l'échéance de sa minuterie de montée et diffuse un `UPDATE` (sans horodatage, sans événement `ROUE_LAST_EVENT`,
  `SEQ` inchangé). La roue n'est arrêtable par les joueurs et l'animateur que lorsque `FULL_SPEED = true` (§5.1).

`ROUE_LAST_EVENT.KIND` ∈ `SPIN | LAND_VALUE | BANKRUPT | LOSE_TURN | LETTER_HIT | LETTER_MISS | LETTER_REPEAT |
FLIP | ANSWER_OK | TURN_END | PASS | REVEAL` — **v12.1.0** : plus `VOWEL_HIT | VOWEL_MISS | BONUS` (§15).

**Règle « un événement par action »** (arbitrage du 2026-11-16, défaut relevé par les tests du moteur, commit
`38627055`) : `ROUE_LAST_EVENT` ne porte qu'un événement ; deux événements émis dans la même action rendraient le
premier invisible. Donc :
- l'entrée en `TURN_END` provoquée par une **cause** — `LETTER_MISS`, `LETTER_REPEAT`, `BANKRUPT`, `LOSE_TURN` —
  n'émet **que l'événement de cause** (`SEQ` + 1) ; **aucun événement `TURN_END` n'est émis** dans ce cas. La fin de
  tour se lit dans `ROUE_SUBPHASE = TURN_END` (et `ROUE_STEP = 7`), pas dans l'événement ;
- l'événement `TURN_END` n'est émis que lorsqu'aucune cause n'est posée dans la même action : `ROUE_NEXT` reçu en
  `ANSWER` (ou en `TURN` sans consonne restante), avec `GAIN` = gain du tour ajouté à la cagnotte ;
- `ROUE_NEXT` reçu en `TURN_END` n'émet aucun événement (`SEQ` inchangé) : le changement de sous-phase suffit.
Champs de l'événement de cause : `TEAM` = équipe qui avait la main ; `LETTER` pour `LETTER_MISS` / `LETTER_REPEAT` ;
`VALUE` = cagnotte perdue pour `BANKRUPT` ; `GAIN` = 0.

Exemples `ROUE_BOARD` sur la phrase du §3.1, après `T`, `R`, `N`, quand l'équipe propose `C` :
```json
{ "COLS": 14, "ROWS": ["*_____N ___R", "*_ *___N __N_", "__ __TR_N_"] }      // LIT : 3 cases éclairées
{ "COLS": 14, "ROWS": ["C_____N ___R", "C_ C___N __N_", "__ __TR_N_"] }      // REVEALING et après
```

## 5. Les 7 étapes d'un tour

| Étape (`ROUE_STEP`) | Nom affiché | `ROUE_SUBPHASE` | Qui agit |
|---|---|---|---|
| 1 | Lancer la roue | `TURN` | Admin ou animateur : `ROUE_SPIN` |
| 2 | Arrêt de la roue | `SPINNING` | **Montée** (`FULL_SPEED = false`, 1 tour) : **personne** ne peut arrêter la roue, pas même l'admin. **Pleine vitesse** (`FULL_SPEED = true`) : équipe qui a la main : buzz (buzzer ou tablette) — ou admin / animateur : `ROUE_STOP` — ou durée maximale, comptée depuis la pleine vitesse |
| 3 | Affichage de la case | `STOPPING` | Serveur : case tirée, freinage |
| 3 bis | *(à voix haute)* | `LETTER` | 🗣 L'équipe annonce une consonne — aucun appareil, aucun changement d'état |
| 4 | Sélection de la consonne | `LETTER` | Admin ou animateur : `ROUE_LETTER`. Consonne présente : passage à l'étape 5 (`LIT`). Aucun `*` dans `ROUE_BOARD` à cette étape |
| 5 | Affichage des cases | `LIT` puis `REVEALING` | `LIT` : les cases de la consonne sont éclairées (`*` dans `ROUE_BOARD`, seule sous-phase où `*` apparaît). Admin ou animateur : `ROUE_FLIP` (« Retourner les cases ») → `REVEALING` : elles se retournent une à une |
| 5 bis | *(à voix haute)* | `ANSWER` | 🗣 L'équipe propose une réponse (ou n'en propose pas) — aucun appareil, aucun changement d'état |
| 6 | Réponse | `ANSWER` | Admin ou animateur : `ROUE_ANSWER` (« Réponse validée », **un seul bouton** — la réponse est un bonus) — sinon directement un des deux boutons de l'étape 7 |
| 7 | Fin du tour | `TURN_END` | Gain ajouté à la cagnotte ; admin ou animateur, **deux boutons** : `ROUE_NEXT {HAND: SAME}` (« Tour suivant », la même équipe rejoue) ou `ROUE_NEXT {HAND: NEXT}` (« Équipe suivante », équipe suivante) |

`ROUE_STEP` est fonction de `ROUE_SUBPHASE` seule : `TURN` = 1, `SPINNING` = 2, `STOPPING` = 3, `LETTER` = 4,
`LIT` et `REVEALING` = 5, `ANSWER` = 6, `TURN_END` = 7, `""` et `SOLVED` = 0. L'éclairage des cases (`*`) relève donc
de l'étape 5, pas de l'étape 4 : `ROUE_LETTER` est le geste de l'étape 4, son effet (`LIT`) ouvre l'étape 5.
La montée et la pleine vitesse sont deux temps de la **même** sous-phase `SPINNING` (`ROUE_STEP = 2`), distingués
par `ROUE_WHEEL.FULL_SPEED`.

```
START ─► 1 TURN ─ROUE_SPIN─► 2 SPINNING ─buzz / ROUE_STOP / durée max─► 3 STOPPING ─fin du freinage─┐
           ▲                                                                                         │
           │        case valeur ─► 4 LETTER ─ROUE_LETTER (présente)─► 5 LIT ─ROUE_FLIP─► REVEALING ─► 6 ANSWER
           │        Banqueroute / Tour perdu ───────────────┐              (absente / déjà proposée)   │
           │                                                ▼                        │                 │ juste
           └──────────── ROUE_NEXT ──────────────────── 7 TURN_END ◄─────────────────┴── fausse / aucune
                                                                                          SOLVED ◄─────┘
   toute sous-phase ─ROUE_REVEAL─► SOLVED (sans gagnant)          SOLVED ─STOP (action existante)─► STOPPED
```

### 5.1 Transitions

| Depuis | Déclencheur | Effet |
|---|---|---|
| `START` | — | Plateau construit, cagnottes à 0, `ROUE_WHEEL` figé, main à la première équipe de `ROUE_TEAMS`, `TURN` |
| `TURN` | `ROUE_SPIN` (si `ROUE_CONSONANTS_LEFT`) | `SPINNING`, `FULL_SPEED = false`, `STOP = null`, **minuterie de montée** `SPIN_UP_MS` armée (la durée maximale n'est pas encore armée) |
| `SPINNING`, montée | Fin de `SPIN_UP_MS` (minuterie serveur) | `FULL_SPEED = true`, `UPDATE` ; **minuterie de durée maximale** `SPIN_MAX_MS` armée à cet instant. Aucun événement |
| `SPINNING`, montée | Appui (`BUTTON`) de n'importe quel buzzer ou tablette | **Ignoré** : état inchangé, aucun `UPDATE` général, `SEQ` inchangé, aucune erreur. Seule trame possible : l'écho d'état ciblé à la tablette émettrice (#248, ci-dessous) |
| `SPINNING`, montée | `ROUE_STOP` (« Forcer l'arrêt »), de l'**admin comme de l'animateur** | **Refusé** : `ROUE_ERROR` `NOT_FULL_SPEED` à l'émetteur, état inchangé, minuterie de montée inchangée |
| `TURN` | `ROUE_ANSWER` ou `ROUE_NEXT {HAND}` (si `!ROUE_CONSONANTS_LEFT`) | Plus aucune consonne à trouver, l'équipe ne peut que proposer : `ROUE_ANSWER` = réponse validée (`SOLVED`) ; `ROUE_NEXT` = pas de réponse validée, main selon `HAND`, reste en `TURN` |
| `TURN` | `ROUE_PASS` | Rotation immédiate, cagnotte conservée, reste en `TURN`. Refusé en `SOLO` |
| `SPINNING`, pleine vitesse | Appui (`BUTTON`) d'un buzzer ou d'une tablette **de l'équipe qui a la main**, ou `ROUE_STOP` (admin ou animateur), ou fin de `SPIN_MAX_MS` | Tirage uniforme d'une case ; `STOPPING`, `FULL_SPEED = false`, `STOP = {TO, DURATION_MS}` |
| `STOPPING` | Fin de `DURATION_MS` (minuterie serveur) | `CURRENT = TO`, `STOP = null`. Valeur : `LETTER`. `bankrupt` : cagnotte de l'équipe à 0, `TURN_END` (pas de lettre trouvée). `lose_turn` : `TURN_END` (pas de lettre trouvée) |
| `LETTER` | `ROUE_LETTER {LETTER}` | Voir §5.3 : présente → `LIT` ; absente ou déjà proposée → `TURN_END` (pas de lettre trouvée) ; voyelle / non-lettre → refus, reste en `LETTER` |
| `LIT` | `ROUE_FLIP` (« Retourner les cases ») | `REVEALING` : les `*` deviennent la lettre ; événement `FLIP` avec `COUNT`. Aucun retournement automatique |
| `REVEALING` | Fin de `COUNT × FLIP_INTERVAL_MS` (minuterie serveur) | `ANSWER` |
| `ANSWER` | `ROUE_ANSWER` | Réponse validée : gain du tour ajouté à la cagnotte, `SOLVED` + crédit (§7) |
| `ANSWER` | `ROUE_NEXT {HAND}` | Pas de réponse validée (aucune proposition, ou proposition fausse — le serveur ne les distingue pas) : gain du tour ajouté à la cagnotte, événement `TURN_END`, main selon `HAND`, `TURN`. L'étape 7 est franchie par ce même geste |
| `TURN_END` | (à l'entrée) | `ROUE_TEAM_BANK[équipe] += ROUE_TURN_GAIN`, `ROUE_TURN_GAIN = 0`, `ROUE_NEXT_TEAM` et `ROUE_NEXT_DEFAULT` calculées (§5.2). **Aucun événement propre** : `ROUE_LAST_EVENT` reste l'événement de cause (`LETTER_MISS`, `LETTER_REPEAT`, `BANKRUPT`, `LOSE_TURN`) — règle « un événement par action », §4 |
| `TURN_END` | `ROUE_NEXT {HAND}` | `SAME` : la même équipe garde la main ; `NEXT` : main à `ROUE_NEXT_TEAM` (refusé en `SOLO`) ; puis `TURN` |
| toute sous-phase non vide sauf `SOLVED` | `ROUE_REVEAL` | `SOLVED` sans gagnant, aucun crédit, plateau entièrement visible |
| `SOLVED` | `STOP` existant | Fin de manche, nettoyage (`stopUnsafe`) |

Appuis ignorés sans effet ni diffusion : buzzer d'une autre équipe, appui hors `SPINNING`, **appui pendant la
montée** (`FULL_SPEED = false`, y compris de l'équipe qui a la main), second appui pendant `STOPPING`. « Sans diffusion » s'entend de l'`UPDATE` général : l'écho d'état ciblé à la tablette
émettrice (#248) reste possible pour tout appui de tablette, accepté ou ignoré.

**Arrêt seulement à pleine vitesse** (décision utilisateur du 2026-11-17) — règle **imposée par le serveur**, quel
que soit l'appareil ou l'émetteur : buzzer physique, tablette joueur, simulation d'appui depuis l'admin, animateur,
**admin**. Il n'existe **aucune exception** : pendant la montée, rien ne peut arrêter la roue. Les écrans ne
font que la refléter (roue non tapable, bouton grisé) ; un client modifié ou en retard n'y change rien.
- **Appui (`BUTTON`) pendant la montée : ignoré en silence.** Même traitement qu'un appui hors `SPINNING` : un
  buzzer physique ne peut afficher aucune erreur, et une diffusion générale par appui chargerait le réseau pour
  rien (les joueurs appuient volontiers trop tôt). L'appui n'est pas mémorisé : il faut appuyer de nouveau une
  fois la pleine vitesse atteinte. Précisément, l'appui :
  - **ne change rien à l'état du jeu** (sous-phase, `FULL_SPEED`, minuteries, roue : inchangés) ;
  - **n'émet aucun `UPDATE` général** : ni l'admin, ni l'animateur, ni la TV, ni les autres tablettes ne
    reçoivent quoi que ce soit ;
  - **n'émet aucun événement** : `ROUE_LAST_EVENT.SEQ` inchangé ; aucune `ROUE_ERROR` ; aucun message au buzzer ;
  - **peut déclencher l'écho d'état ciblé à la seule tablette émettrice** — mécanisme existant de l'appui d'une
    tablette joueur (#247 / #248, `handleSimulatedButton`), **conservé tel quel** (arbitrage du 2026-11-17 après le
    rapport `qa-montee-20261117-1055`) : un `UPDATE` complet envoyé à cette tablette et à elle seule, qui porte
    l'état inchangé (`SPINNING`, `FULL_SPEED = false`, même `SEQ`). Il ne vaut ni acceptation ni refus de l'appui,
    et la tablette n'en déduit rien d'autre que l'état courant. Un buzzer physique ne reçoit aucun écho.
- **`ROUE_STOP` pendant la montée, de l'admin comme de l'animateur : refusé avec `ROUE_ERROR` `NOT_FULL_SPEED`**
  (décision utilisateur du 2026-11-17 : « Forcer l'arrêt » n'a pas de passe-droit). C'est une commande d'écran,
  comme les autres actions `ROUE_*` : l'émetteur doit savoir pourquoi rien ne se passe. Les écrans de l'admin et de
  l'animateur grisent le bouton tant que `FULL_SPEED = false` ; l'erreur ne couvre que la course entre l'appui et
  l'`UPDATE`.
- Conséquence : **la roue ne freine jamais depuis la montée** ; tout freinage part de la vitesse de croisière (§6).
  Pour interrompre un lancer pendant la montée, l'opérateur dispose des commandes existantes, qui ne tirent aucune
  case : `PAUSE`, `ROUE_REVEAL` (« Dévoiler ») ou l'arrêt de la manche.

En mode `ROUE`, un appui ne pose jamais le statut `PAUSE` d'un buzzer et ne diffuse pas l'action
`PAUSE`. `PAUSE` / reprise (actions admin existantes) : toutes les minuteries du mode (**montée**, durée maximale, freinage,
retournement) sont suspendues puis reprises pour leur durée restante ; toute action `ROUE_*` est refusée en phase
`PAUSED`. Toutes les minuteries sont annulées à l'arrêt de manche, à `ROUE_REVEAL` et à `NEW_GAME`.

**Minuteries de `SPINNING`** — deux minuteries **successives**, jamais simultanées :
1. **Montée** : armée à `ROUE_SPIN`, durée `SPIN_UP_MS`. À l'échéance : `FULL_SPEED = true`, `UPDATE`, puis
   armement de la suivante.
2. **Durée maximale** : armée **à la fin de la montée seulement**, durée `SPIN_MAX_MS`. À l'échéance : arrêt
   automatique (tirage, `STOPPING`). L'arrêt automatique survient donc `SPIN_UP_MS + SPIN_MAX_MS` après le lancer
   (15 s avec les réglages par défaut).
Chaque armement porte un numéro de génération : une échéance dont la génération n'est plus la génération courante
(arrêt demandé entre-temps, `PAUSE`, arrêt de manche, `ROUE_REVEAL`, `NEW_GAME`) est ignorée. Une pause pendant la
montée la suspend : `FULL_SPEED` reste `false`, la montée reprend pour sa durée restante à la reprise.

### 5.2 Fin du tour : deux boutons, un bouton conseillé

C'est **l'admin ou l'animateur qui décide** : « Tour suivant » (`HAND: SAME`) ou « Équipe suivante » (`HAND: NEXT`).
Le serveur ne choisit jamais à sa place ; il indique seulement le bouton **conseillé** (`ROUE_NEXT_DEFAULT`), que
les écrans mettent en avant. Les deux boutons restent utilisables, sauf en `SOLO`.

Principe (décision utilisateur) : **on passe à l'équipe suivante s'il n'y a pas eu de lettre trouvée ; la réponse
est un bonus** — une réponse fausse ou absente ne change pas le bouton conseillé.

| Tour | `SOLO` | `CHACUN_SON_TOUR` | `TANT_QUE_JE_GAGNE` |
|---|---|---|---|
| Lettre trouvée | `SAME` (seul bouton) | `NEXT` | `SAME` |
| Pas de lettre trouvée (lettre absente ou déjà proposée, Banqueroute, Tour perdu, plus aucune consonne) | `SAME` (seul bouton) | `NEXT` | `NEXT` |

« Équipe suivante » (`ROUE_NEXT_TEAM`) = `rotateTeam(ROUE_TEAMS, ROUE_CURRENT_TEAM)`. Le gain d'une lettre trouvée
reste toujours acquis à la cagnotte, quel que soit le bouton choisi.

### 5.3 `ROUE_LETTER`

1. `LETTER` normalisée (majuscule, accent replié). Pas une lettre : refus `NOT_A_LETTER`. Voyelle
   (`A E I O U Y`) : refus `VOWEL_NOT_AVAILABLE` (« voyelle non disponible »). Refus = réponse `ROUE_ERROR` à
   l'émetteur, **état inchangé**, aucune pénalité.
2. Consonne déjà dans `ROUE_LETTERS` : événement `LETTER_REPEAT`, traitée comme absente (pas de nouvelle entrée).
3. Consonne absente : ajoutée à `ROUE_LETTERS` (`HIT: false`, équipe qui a la main), événement `LETTER_MISS`, `TURN_END`.
4. Consonne présente : ajoutée à `ROUE_LETTERS` (`HIT: true`, équipe qui a la main) ; ses cases passent à `*` (`LIT`) ;
   `ROUE_TURN_GAIN = COUNT × VALUE` de la case courante ; événement `LETTER_HIT`.

## 6. Roue : tirage et animation

- **Tirage** : uniforme sur les cases de `ROUE_WHEEL.SEGMENTS` (une case = une chance). Source d'aléa injectable
  pour les tests. Le tirage a lieu **au moment de l'arrêt demandé**, quel que soit le déclencheur.
- **Écrans concernés** : les règles d'animation ci-dessous valent à l'identique pour **la TV et la tablette
  joueur** (§12), qui dessinent la même roue à partir de `ROUE_WHEEL` (même forme, mêmes cases, même composant).
  Chaque écran anime sa roue localement : la TV et les tablettes s'arrêtent sur la même case, au même moment à la
  latence réseau près, mais ne montrent pas forcément la même case pendant la rotation.
- **`SPINNING` — montée** (décisions utilisateur du 2026-11-17 ; `FULL_SPEED = false`) : la roue part **de
  l'arrêt**, depuis la case affichée, et **accélère à taux constant** jusqu'à la **pleine vitesse** (vitesse de
  croisière) = un tour en `SPIN_PERIOD_MS`. Elle l'atteint **en 1 tour exactement**, soit en `SPIN_UP_MS`
  = `2 × SPIN_PERIOD_MS` (5 s par défaut), **de la même manière sur la TV et sur la tablette**, sans à-coup à
  l'arrivée en pleine vitesse. Pendant la montée, la roue n'est **ni tapable ni arrêtable** (§5.1) ; les écrans le
  disent (« La roue prend de la vitesse… ») et n'affichent pas la barre de durée maximale.
- **`SPINNING` — pleine vitesse** (`FULL_SPEED = true`) : la roue défile en continu à la vitesse de croisière ;
  elle devient tapable pour l'équipe qui a la main. La position affichée n'a aucune valeur de jeu et n'est pas
  synchronisée entre écrans. Les écrans montrent le temps restant avant l'arrêt automatique : `SPIN_MAX_MS`,
  **compté localement depuis la réception de `FULL_SPEED = true`** — la durée maximale ne commence qu'à la pleine
  vitesse.
- **Passage à la pleine vitesse** : c'est `FULL_SPEED = true`, reçu du serveur, qui fait foi — pas la fin de la
  montée calculée par l'écran. Si l'`UPDATE` arrive alors que la montée locale n'est pas tout à fait terminée
  (latence, onglet ralenti), l'écran passe aussitôt à la vitesse de croisière : l'écart est de l'ordre de la
  latence sur `SPIN_UP_MS`, imperceptible. Si la montée locale se termine avant l'`UPDATE`, la roue tourne à la
  vitesse de croisière mais reste non tapable jusqu'à sa réception.
- **Client qui se connecte ou se reconnecte pendant `SPINNING`** : il affiche la roue **directement à la vitesse de
  croisière**, sans montée (il ne sait pas depuis quand elle tourne : aucun horodatage). `FULL_SPEED` lui dit le
  reste : `false` → roue non tapable et message de montée, jusqu'à l'`UPDATE` ; `true` → roue tapable, barre de
  durée maximale comptée depuis sa connexion.
- **`STOPPING`** : chaque écran freine depuis sa position jusqu'à la case `STOP.TO` et s'y arrête **exactement**,
  **au plus tard à `STOP.DURATION_MS`** (`wheel-component.md` §5). Le freinage part **toujours de la vitesse de
  croisière**, sans saut de vitesse : aucun arrêt — joueurs, animateur, admin, durée maximale — n'est possible
  avant la pleine vitesse (§5.1). Il n'y a aucun freinage depuis la montée.
  - **Profil** (depuis le correctif `656067e5`, QUALIF 12.0.0.3) : la roue **continue à sa vitesse**, puis
    **décélère à taux constant** jusqu'à l'arrêt sur `TO` ; la décélération dure au moins 500 ms.
  - **Distance** : la première occurrence de `TO` vers l'avant, plus **0 à 3 tours** ajoutés — le nombre qui permet
    ce profil dans `DURATION_MS`. Il n'y a **pas de tour complet garanti** : la distance découle de la vitesse et de
    `DURATION_MS`, pas d'un minimum de tours.
  - Quand aucune distance n'atterrit exactement à `DURATION_MS`, la roue freine sans palier et s'arrête **avant**
    `DURATION_MS` ; elle reste alors sur `TO` jusqu'à la fin de la minuterie du serveur.
  - **Repli** : ce profil existe toujours quand `DURATION_MS ≥ SPIN_PERIOD_MS + 250 ms`. En deçà, il peut ne pas
    exister selon la position de la roue : l'écran retombe alors sur l'ancien profil (ease-out cubique, au moins
    un tour, vitesse non continue). Réglages par défaut : 4 000 ms pour 2 500 ms, donc jamais de repli ; un repli
    n'est possible qu'avec un freinage court et un tour lent (§10).
  - Écran en retard (sa montée locale n'est pas terminée quand `STOPPING` arrive) : il freine comme s'il était à
    la vitesse de croisière ; l'écart, de l'ordre de la latence, est imperceptible.
  - Un client qui se connecte ou se reconnecte pendant `STOPPING` se place directement sur `TO`. Cette règle ne
    vaut **que** pour un écran qui n'affichait pas la roue en rotation : un écran déjà affiché — TV **ou
    tablette** — garde la même roue de `SPINNING` à `STOPPING` et **freine** ; il ne doit jamais recréer la roue
    au changement de sous-phase (ce qui la placerait d'un coup sur `TO`).
- `prefers-reduced-motion` : ni montée ni freinage animés, placement direct sur la case (`wheel-component.md` §5) ;
  la règle « arrêtable seulement quand `FULL_SPEED = true` » s'applique à l'identique.
- **Arrêtée** : `CURRENT` sous le pointeur, accent = couleur de l'équipe qui a la main. La roue reste visible
  pendant toute la manche.
- Le serveur applique l'effet de la case **à la fin** de `DURATION_MS`, jamais au moment de l'arrêt demandé.
- **Retournement (`REVEALING`)** : les écrans retournent les cases de l'événement `FLIP` une à une, dans l'ordre
  de lecture, une toutes les `FLIP_INTERVAL_MS`. Un client qui se connecte en cours affiche les lettres directement.

## 7. Gains

- Pendant la manche, seules `ROUE_TURN_GAIN` et `ROUE_TEAM_BANK` évoluent ; **aucun score n'est modifié**.
- Le gain d'un tour rejoint la cagnotte de l'équipe à l'étape 7 (ou à la réponse juste).
- À `ROUE_ANSWER` (réponse validée) : `ROUE_WINNER` = équipe qui a la main, puis crédit en points d'équipe
  (même chemin que `TEAM_POINTS` : score d'équipe, événement d'historique `POINTS_AWARDED` avec la catégorie de la
  question, animation de gain existante) :
  - `WINNER_ONLY` : seule l'équipe gagnante reçoit sa cagnotte ;
  - `ALL_TEAMS` : chaque équipe participante reçoit sa cagnotte.
  Une cagnotte à 0 ne produit ni crédit ni événement. `ROUE_AWARDS` liste les crédits effectués.
- `ROUE_REVEAL` : aucun crédit.
- La cagnotte est tenue **par équipe** (les scores par joueur ne sont pas concernés par ce mode).
- Les corrections manuelles passent par la page Scores existante.

## 8. Actions WebSocket

### 8.1 Client → serveur (à inscrire dans `inbound_allowlist.go`)

| Action | Payload | Phases / sous-phases acceptées |
|---|---|---|
| `ROUE_SET_TEAMS` | `{ "TEAMS": string[] }` | `PREPARE`, `READY` (patron `RAFALE_SET_TEAMS`) ; **aucun contrôle du nombre d'équipes** — voir ci-dessous |
| `ROUE_SPIN` | `{}` | `STARTED` + `TURN` + `ROUE_CONSONANTS_LEFT` |
| `ROUE_STOP` | `{}` | `STARTED` + `SPINNING` (« Forcer l'arrêt ») **et `ROUE_WHEEL.FULL_SPEED = true`** — pour l'admin comme pour l'animateur ; pendant la montée : `ROUE_ERROR` `NOT_FULL_SPEED` (§5.1) |
| `ROUE_LETTER` | `{ "LETTER": "T" }` | `STARTED` + `LETTER` |
| `ROUE_FLIP` | `{}` | `STARTED` + `LIT` (« Retourner les cases ») |
| `ROUE_ANSWER` | `{}` | `STARTED` + `ANSWER` ; aussi `TURN` quand `!ROUE_CONSONANTS_LEFT` — « Réponse validée » |
| `ROUE_NEXT` | `{ "HAND": "SAME" \| "NEXT" }` | `STARTED` + `TURN_END` ou `ANSWER` ; aussi `TURN` quand `!ROUE_CONSONANTS_LEFT` — `SAME` = « Tour suivant », `NEXT` = « Équipe suivante » (refusé en `SOLO`) |
| `ROUE_PASS` | `{}` | `STARTED` + `TURN`, hors `SOLO` |
| `ROUE_REVEAL` | `{}` | `STARTED`, sous-phase ≠ `SOLVED` |

**`ROUE_SET_TEAMS`** (décision utilisateur du 2026-11-17, après la QUALIF 12.0.0.1 — aligné sur RAFALE) :
- seuls contrôles : chaque équipe de `TEAMS` **existe**, et **aucun doublon** ; sinon refus `INVALID_TEAMS`, sélection
  inchangée ;
- **le nombre d'équipes n'est pas contrôlé** : toute liste valable est enregistrée telle quelle dans `ROUE_TEAMS`
  (ordre = ordre de rotation), quel que soit `ROUE_MODE` — y compris 1 équipe hors `SOLO`, ou plusieurs en `SOLO` ;
  l'admin peut ainsi composer la sélection équipe par équipe ;
- **liste vide = effacement** de la sélection (`ROUE_TEAMS = []`, plus d'équipe ayant la main) ;
- le comptage (`SOLO` : exactement 1 équipe ; autres modes : 2 au moins) n'est appliqué **qu'au passage
  `PREPARE` → `READY` et à `FORCE_READY`** (garde `participantsConform`, §13 point 4). Après chaque `ROUE_SET_TEAMS`
  accepté, la phase est réévaluée : une sélection conforme permet `READY`, une sélection non conforme laisse — ou
  ramène — la manche en `PREPARE`. Une manche ne démarre donc jamais avec un nombre d'équipes non conforme.

L'arrêt par les joueurs passe par l'action `BUTTON` existante (buzzer physique, tablette, simulation admin) —
aucune action nouvelle côté buzzer, **aucun changement de protocole BuzzClick**.
Pendant une entracte : actions `ROUE_*` refusées (comportement par défaut de la liste blanche d'entracte).

### 8.2 Émetteurs autorisés

| Émetteur | Actions |
|---|---|
| `/ws/admin` | Toutes les actions du §8.1 |
| `/ws/anim` | `ROUE_SPIN`, `ROUE_STOP`, `ROUE_LETTER`, `ROUE_FLIP`, `ROUE_ANSWER`, `ROUE_NEXT` — les 7 boutons d'étape de la tablette animateur (maquette `anim_roue__conduct_panel`). **Jamais** `ROUE_SET_TEAMS`, `ROUE_PASS` ni `ROUE_REVEAL` (admin seulement) |
| `/ws/tv`, `/ws/player` | Aucune action `ROUE_*` (la tablette n'envoie que `BUTTON`) |

Admin et animateur peuvent commander en même temps : le serveur applique la première action valable pour la
sous-phase courante et refuse la suivante (`WRONG_SUBPHASE`), sans effet ni double application.

### 8.3 Serveur → client

- `UPDATE` (`GameState`) après chaque transition — action existante.
- `ROUE_ERROR` à l'émetteur seul : `{ "CODE": string, "MESSAGE": string }`, `CODE` ∈
  `VOWEL_NOT_AVAILABLE | NOT_A_LETTER | WRONG_SUBPHASE | NOT_ALLOWED | INVALID_TEAMS | NOT_FULL_SPEED` — **v12.1.0** :
  plus `NOT_A_VOWEL | VOWELS_DISABLED | VOWEL_NOT_NOW | VOWEL_LIMIT_REACHED | INSUFFICIENT_BANK | LETTER_ALREADY_PROPOSED |
  NO_VOWEL_LEFT` (§15.1.7).
  `NOT_FULL_SPEED` = `ROUE_STOP` (admin ou animateur) pendant la montée de la roue (§5.1).
  `INVALID_TEAMS` = équipe inconnue ou en double dans `ROUE_SET_TEAMS` — jamais un nombre d'équipes (§8.1).

## 9. Confidentialité de la phrase — règle non négociable

- `QUESTION.ROUE_PHRASE` est **retiré** de tout message envoyé à `/ws/tv` et `/ws/player`, dans toutes les phases
  (le plateau entièrement visible suffit après résolution). Il reste présent pour `/ws/admin` et `/ws/anim`.
- `ROUE_BOARD` ne contient jamais une lettre non révélée (`_` ou `*` seulement).
- La présentation et le champ `question`, publics, ne peuvent pas contenir la phrase (§11.1).
- `QUESTION.ROUE_PRESENTATION` et `QUESTION.CATEGORY` sont publics et **doivent rester présents** dans les messages
  TV et tablette (la tablette affiche l'indice, §12) : les filtres ne retirent de `QUESTION` que la phrase et les
  champs de réponse.
- La phrase n'est jamais écrite dans `ANSWER`, dans un journal diffusé (`/ws/logs`) ni dans un message aux buzzers.
- **Tests exigés** (modèle `internal/protocol/rafale_leak_test.go`) : pour chaque sous-phase avant `SOLVED`, la
  phrase et chacune de ses lettres non révélées sont absentes des messages sérialisés TV et tablette.

## 10. Configuration de la roue — section `wheel` de `game-config.json`

La roue n'est pas une liste de cases saisie : elle est **générée par le serveur** à partir de 4 paramètres.

```json
"wheel": {
  "max_points": 1000,
  "step": 50,
  "bankrupt_count": 2,
  "lose_turn_count": 1,
  "spin_period_ms": 2500,
  "spin_max_ms": 10000,
  "stop_duration_ms": 4000,
  "flip_interval_ms": 600
}
```

| Champ | Règle |
|---|---|
| `max_points` | Points de la meilleure case. Entier, **1 000 000 au plus** (au-delà : refus `400`, à l'enregistrement comme à l'aperçu §11.3 — une valeur plus grande n'a pas d'usage, la roue étant limitée à 64 cases) ; défaut 1000 |
| `step` | Pas : écart entre deux valeurs, et plus petite valeur. Entier ≥ 1 ; **strictement inférieur à `max_points`** (sinon une seule valeur : refus) ; doit diviser `max_points` — **s'il ne le divise pas, il est corrigé** au diviseur de `max_points` le plus proche (inférieur à `max_points` ; à égalité de distance, le plus petit), sans refus ; défaut 50 |
| `bankrupt_count` | Nombre de cases BANQUEROUTE. 0 ou plus, défaut 2 |
| `lose_turn_count` | Nombre de cases TOUR PERDU. 0 ou plus, défaut 1 |
| — | Nombre total de cases de la roue générée : **64 au plus** (limite du composant `Wheel`). Pas ≥ maximum, maximum supérieur à 1 000 000, valeur non entière ou négative, ou plus de 64 cases : `400` avec message, **rien n'est enregistré** |
| — | **Pas corrigé** : la réponse de `POST /game-config.json` et de `POST /api/roue/wheel-preview` porte le pas retenu et `"step_adjusted_from": <pas saisi>` (absent sans correction) ; l'interface affiche « Pas ajusté à … » et remplace la valeur du champ |
| `spin_period_ms` | Durée d'un tour de roue **à pleine vitesse** : c'est le réglage de la **vitesse**. 1000–6000, défaut 2500 (inchangé) ; hors bornes : ramené à la borne. Fixe aussi la durée de la montée : `2 × spin_period_ms` (1 tour, §6) — 2 à 12 s, 5 s par défaut ; aucun réglage séparé |
| `spin_max_ms` | **Durée maximale de rotation à pleine vitesse** : comptée **à partir de la fin de la montée** ; sans arrêt demandé, la roue s'arrête d'elle-même. 3000–30000, défaut 10000 ; hors bornes : ramené à la borne |
| `stop_duration_ms` | Durée du freinage. 1500–8000, défaut 4000 ; hors bornes : ramené à la borne |
| `flip_interval_ms` | Intervalle entre deux cases retournées. 200–1500, défaut 600. Non exposé dans l'interface en v12.0 |

### 10.1 Construction de la roue — une seule implémentation, côté serveur

Notations : `MAX = max_points`, `PAS = step`, `B = bankrupt_count`, `P = lose_turn_count`.
`arrondi(x)` = arrondi au plus proche, la demie vers le haut.

1. **Valeurs** : `MAX, MAX − PAS, MAX − 2×PAS, …, PAS` (soit `MAX / PAS` valeurs, toutes différentes). Si `P ≥ 1`
   et que `MAX / 2` est une de ces valeurs, elle est **retirée** (le TOUR PERDU prend sa place à mi-tour).
2. **Alternance forte / faible** des valeurs restantes, en commençant par la plus forte → suite `S` :
   - sans BANQUEROUTE (`B = 0`) : la plus forte, la plus faible, la deuxième plus forte, la deuxième plus faible… —
     `100, 10, 90, 20, 80, 30, 70, 40, 60, 50` ;
   - avec BANQUEROUTE (`B ≥ 1`) : la première BANQUEROUTE tient la place d'une valeur « 0 », juste après le
     maximum ; l'alternance reprend donc par la deuxième plus forte — `100, [BANQ], 90, 10, 80, 20, 70, 30, 60, 40, 50`.
3. **Cases spéciales** : `K = B + P`, `N = (nombre de valeurs) + K` cases au total. Emplacements, dans l'ordre :
   - si `B ≥ 1` : emplacement 0 à l'**index 1** (juste après le maximum), puis emplacement `j` (`j = 1…K−1`) à
     l'index `⌊j × N / K⌋` ;
   - si `B = 0` : emplacement `j` (`j = 0…K−1`) à l'index `⌊(j + 0,5) × N / K⌋` ;
   - un index déjà pris, ou l'index 0 (réservé au maximum), est remplacé par le suivant libre.
   Types, dans l'ordre des emplacements : BANQUEROUTE d'abord (si `B ≥ 1`), puis **en alternance** TOUR PERDU /
   BANQUEROUTE tant qu'il reste des deux, puis le type restant.
   Avec `B = P = 1`, le TOUR PERDU est donc à l'index `⌊N / 2⌋` : **à mi-tour, à l'opposé du maximum**.
4. **Remplissage** : les index restants reçoivent `S` dans l'ordre, à partir de l'index 0 (le maximum).

Quand les cases spéciales sont nombreuses par rapport aux valeurs (par exemple 5 pour 9 valeurs), deux cases
spéciales peuvent se toucher : c'est accepté.

Exemples (vérifiés par exécution de la règle) :

| `MAX / PAS / B / P` | Roue générée (index 0 en premier ; la dernière case rejoint la première) |
|---|---|
| 100 / 10 / 0 / 0 | `100 10 90 20 80 30 70 40 60 50` |
| 100 / 10 / 1 / 0 | `100 BANQ 90 10 80 20 70 30 60 40 50` |
| 100 / 10 / 1 / 1 | `100 BANQ 90 10 80 PERDU 20 70 30 60 40` |
| 100 / 10 / 0 / 1 | `100 10 90 20 80 PERDU 30 70 40 60` |
| 1000 / 100 / 2 / 1 | `1000 BANQ 900 100 PERDU 800 200 700 BANQ 300 600 400` |
| 1000 / 50 / 2 / 1 (défaut) | `1000 BANQ 950 50 900 100 850 PERDU 150 800 200 750 250 700 BANQ 300 650 350 600 400 550 450` |
| 100 / 20 / 1 / 1 | `100 BANQ 80 PERDU 20 60 40` |
| 1000 / 100 / 3 / 2 | `1000 BANQ PERDU 900 100 BANQ 800 200 PERDU 700 300 BANQ 600 400` |
| 100 / 10 / 0 / 2 | `100 10 PERDU 90 20 80 30 70 PERDU 40 60` |
| 100 / 30 / 1 / 1 → pas corrigé à 25 | `100 BANQ PERDU 75 25` |

Correction du pas — exemples exigés : `100 / 30 → 25` ; `1000 / 300 → 250` ; `1000 / 70 → 50` ; `100 / 7 → 5` ;
`100 / 15 → 10` (égalité entre 10 et 20 : le plus petit).

Ces exemples sont des **cas de test exigés** (résultat identique, case pour case).

- Section absente : **roue par défaut** `1000 / 50 / 2 / 1` (22 cases ; validée par l'utilisateur).
- `GET /game-config.json` renvoie `wheel` (paramètres effectifs) et `wheel_default` (paramètres par défaut,
  lecture seule, pour le bouton « Roue par défaut »).
- `POST /game-config.json` avec `{ "wheel": {…} }` remplace la section entière (fusion par section, comme
  `neon_effect`).
- `POST /api/roue/wheel-preview` (§11.3) renvoie la roue générée pour des paramètres non enregistrés — l'aperçu
  de la carte de réglage n'a **aucune copie JavaScript** de la règle.
- La carte de réglage propose un **essai** de la roue avec les valeurs en cours de saisie (§10.2).
- La roue est générée au `START` de la manche et figée dans `ROUE_WHEEL.SEGMENTS` ; une modification pendant une
  manche ROUE ne touche pas la manche en cours.
- La **forme de la roue** n'est pas dans la configuration : c'est un réglage de la question (`ROUE_WHEEL_LAYOUT`).
- Sauvegarde, restauration, réinitialisation : acquises (fichier `game-config.json`, drapeau `ambiance`).

### 10.2 Essai de la roue — bouton « Tester » de la carte de réglage

Décision utilisateur (QUALIF 12.0.0.8) : régler la vitesse et les durées sans lancer de manche.

- **Où** : carte « Roue de la Fortune » de la page des réglages (`WheelConfigCard`). L'essai **anime les deux
  aperçus déjà présents** — le bandeau « Sur la TV » et la vue d'ensemble circulaire de la roue ; **aucun aperçu
  n'est ajouté**. La liste des cases « dans l'ordre » reste fixe.
- **Avec quoi** : les valeurs **en cours de saisie, non enregistrées** — les cases et les durées renvoyées par
  `POST /api/roue/wheel-preview` (§11.3) pour ces valeurs. Disponible seulement quand l'aperçu est valide.
- **Sans effet sur le jeu** : aucun message WebSocket, aucune action `ROUE_*`, aucune écriture ; l'état du jeu,
  la configuration enregistrée et les autres écrans ne sont pas touchés. Utilisable dans n'importe quelle phase.
  Seul appel réseau : l'aperçu `wheel-preview` déjà émis par la carte.
- **Déroulement** — les mêmes temps et les mêmes règles d'animation qu'en jeu (§6, `wheel-component.md` §5) :

| Temps | Libellé d'état | « Tester » | « Arrêter » | Roue |
|---|---|---|---|---|
| Repos | *(aucun, ou case de l'arrêt précédent)* | actif | grisé | Fixe |
| Montée | « Montée… » | grisé | **grisé** | Part de l'arrêt, pleine vitesse au bout d'un tour (`SPIN_UP_MS`) |
| Pleine vitesse | « Pleine vitesse » | grisé | **actif** | Vitesse de croisière ; temps restant avant l'arrêt automatique (`SPIN_MAX_MS`, compté depuis la pleine vitesse) |
| Freinage | « Freinage » | grisé | grisé | Freine jusqu'à la case tirée, en `STOP_DURATION_MS` au plus |
| Arrêt | Case obtenue (« Arrêt sur 500 ») | actif | grisé | Fixe sur la case tirée |

- « Arrêter », ou la fin de la durée maximale sans clic, tire **une case au hasard côté client** (tirage uniforme,
  une case = une chance) et déclenche le freinage. Ce tirage n'a aucune valeur de jeu.
- Les deux aperçus partent de la même case, tournent ensemble et s'arrêtent **sur la même case, au même instant** ;
  le pointeur de chacun reste fixe.
- Modifier un réglage pendant un essai, ou quitter la page, **interrompt l'essai** : minuteries annulées, aperçus
  de nouveau fixes. « Enregistrer » et « Roue par défaut » restent indépendants de l'essai.
- `prefers-reduced-motion` : les temps s'enchaînent, la roue est placée directement sur la case tirée.
- **Aucune copie de règle dans le client** : la durée de la montée et les durées ramenées à leurs bornes viennent
  de `TIMING` (§11.3), jamais d'un calcul de la page.

## 11. HTTP

### 11.1 `POST /questions` (multipart, existant) — champs ajoutés pour `type=ROUE`

| Champ de formulaire | Champ enregistré |
|---|---|
| `roue_phrase` | `ROUE_PHRASE` |
| `roue_presentation` | `ROUE_PRESENTATION` |
| `ROUE_MODE` | `ROUE_MODE` |
| `roue_gains_rule` | `ROUE_GAINS_RULE` |
| `roue_wheel_layout` | `ROUE_WHEEL_LAYOUT` |
| `roue_show_letters` (`true` / `false`) | `ROUE_SHOW_LETTERS` |

Refus `400` (texte, préfixé par le code ; rien n'est écrit ; dossier d'une création refusée supprimé, règle #278) :
`ROUE_PHRASE_EMPTY`, `ROUE_PHRASE_INVALID_CHAR`, `ROUE_PHRASE_WORD_TOO_LONG`, `ROUE_PHRASE_TOO_LONG`,
`ROUE_PRESENTATION_TOO_LONG`, `ROUE_PRESENTATION_CONTAINS_PHRASE`, `ROUE_MODE_INVALID`, `ROUE_GAINS_RULE_INVALID`,
`ROUE_WHEEL_LAYOUT_INVALID`.

**La phrase ne doit pas se retrouver dans un champ public** (revue LOT-1.2, 2026-11-16) — la présentation et le
champ `question` sont diffusés à la TV et aux tablettes (§9). Comparaison **repliée** : majuscules, accents retirés,
seuls les lettres et les chiffres sont gardés (espaces et ponctuation ignorés) ; il y a fuite quand le champ replié
**contient** la phrase repliée entière.
- `roue_presentation` qui contient la phrase : refus `400` `ROUE_PRESENTATION_CONTAINS_PHRASE`, rien n'est écrit.
- `question` qui contient la phrase : pas de refus (le champ n'est pas saisi dans l'éditeur ROUE) ; il est
  **remplacé** par la présentation, ou par « Roue de la Fortune » si elle est vide.
Tous les champs `ROUE_*` sont relus à chaque enregistrement (un champ non lu serait perdu, leçon #107).

### 11.2 `POST /api/roue/preview`

**Description** : valide une phrase et renvoie sa mise en lignes, sans rien écrire (aperçu de l'éditeur).
**Auth** : comme `POST /questions`.

**Request body** :
```json
{ "PHRASE": "Combien pour ce chien dans la vitrine" }
```
**Response 200** :
```json
{ "COLS": 14, "MAX_ROWS": 4, "ROWS": ["COMBIEN POUR", "CE CHIEN DANS", "LA VITRINE"], "CONSONANTS": 12, "LETTERS": 17 }
```
`CONSONANTS` / `LETTERS` : nombre de consonnes / de lettres **distinctes** à trouver.
**Errors** : `400` `{ "CODE": "ROUE_PHRASE_…", "MESSAGE": "…" }` (mêmes codes que §11.1).

### 11.3 `POST /api/roue/wheel-preview`

**Description** : construit la roue pour des paramètres donnés, sans rien écrire (aperçu de la carte de réglage).
**Auth** : comme `POST /game-config.json`.

**Request body** :
```json
{ "max_points": 100, "step": 10, "bankrupt_count": 1, "lose_turn_count": 1,
  "spin_period_ms": 2500, "spin_max_ms": 10000, "stop_duration_ms": 4000 }
```
Les trois durées sont **facultatives** (ajout du 2026-11-17, pour l'essai §10.2) ; absentes : valeurs par défaut.
**Response 200** :
```json
{ "COUNT": 11, "step": 10,
  "SEGMENTS": [ { "ID": "s0", "KIND": "value", "VALUE": 100 }, { "ID": "s1", "KIND": "bankrupt", "VALUE": 0 },
                { "ID": "s2", "KIND": "value", "VALUE": 90 } ],
  "TIMING": { "SPIN_PERIOD_MS": 2500, "SPIN_UP_MS": 5000, "SPIN_MAX_MS": 10000, "STOP_DURATION_MS": 4000 } }
```
`TIMING` : les durées **telles que le jeu les utiliserait** pour ces réglages — ramenées à leurs bornes comme à
l'enregistrement (§10), et `SPIN_UP_MS` calculé par la même règle qu'au `START` (§4). Toujours présent.
**Errors** : `400` `{ "CODE": "ROUE_WHEEL_INVALID", "MESSAGE": "…" }` (pas ≥ maximum, maximum supérieur à 1 000 000, valeur invalide ou plus de 64 cases, §10). Un pas qui ne divise pas le
maximum n'est **pas** une erreur : il est corrigé (`step_adjusted_from`).

## 12. Périphériques et ambiance

| Élément | Comportement |
|---|---|
| Buzzers (LED) | Équipe qui a la main allumée, autres atténuées (grille active / suivante existante, `led.go`) ; cas `ROUE` ajoutés aux règles par type (reconnexion, pause, révélation) |
| Éclairage d'ambiance | `ActiveTeam` renvoie l'équipe qui a la main (scène « tour d'équipe » existante) |
| Sons | Bruitages existants seulement : gain à la résolution (crédit), « mauvaise réponse » sur lettre absente / réponse fausse / Banqueroute. **Aucun nouveau bruitage** en v12.0 (son de rotation : hors périmètre) |
| Tablettes | Affichage via `PlayerDisplay` (`isVPlayer`), maquette `vplayer_roue__wheel_tap_stop` (2026-11-17, remplace `vplayer_roue__buzz_turn`). **Indice** : catégorie et `ROUE_PRESENTATION` sur leur propre ligne au-dessus du plateau, comme sur la TV, jamais tronquées (2 lignes au plus) ; sans présentation, la catégorie seule. Plateau sur toute la largeur. **Roue** : la roue de la manche (`ROUE_WHEEL`, même forme que sur la TV) est affichée en permanence **à la place du bouton d'arrêt** — il n'y a plus de bouton STOP — et suit les règles d'animation du §6. **Arrêt** : en `SPINNING` **et à pleine vitesse seulement** (`ROUE_WHEEL.FULL_SPEED = true`), un joueur de l'équipe qui a la main **touche la roue** pour l'arrêter avant la durée maximale ; cet appui est l'action `BUTTON` existante (§8.1), aucune action nouvelle. Pendant la montée, la roue n'est pas tapable et l'écran l'annonce (« La roue prend de la vitesse… ») ; le cadre allumé et « Touchez la roue pour l'arrêter » n'apparaissent qu'à pleine vitesse. Hors `SPINNING`, pendant la montée, ou pour une autre équipe, la roue est visible mais non tapable (et le serveur ignore l'appui, §5.1) |

## 13. Checklist d'accroche backend

1. `QuestionType` `ROUE` + `AllQuestionTypes()` + registre (`question_types.go`) + `TypedContent`.
2. Champs `GameState` (§4) : init `NewEngine`, remise à zéro `Ready` / `stopUnsafe` / `InitGameWith`.
3. Logique de manche dans un fichier dédié `internal/game/roue.go` (pas dans `engine.go`) : départ, lancer, arrêt
   (trois déclencheurs), minuteries (montée puis durée maximale, freinage, retournement — annulation, génération et pause),
   lettre, réponse, fin de tour, passe, dévoilement, crédit.
4. Garde de départ (`participantsConform`, `PREPARE` → `READY` et `FORCE_READY`) : `ROUE_TEAMS` conforme au mode
   (`SOLO` : exactement 1 équipe ; autres modes : 2 au moins), phrase non vide. **Seul endroit** où le nombre
   d'équipes est contrôlé — `ROUE_SET_TEAMS` ne le contrôle pas (§8.1).
5. `ProcessButtonPress` : cas `ROUE` (équipe qui a la main + `SPINNING`, sinon ignoré).
6. Renommage / purge d'équipe : `ROUE_TEAMS`, `ROUE_TEAM_BANK`, `ROUE_CURRENT_TEAM`, `ROUE_NEXT_TEAM`,
   `ROUE_AWARDS`, `ROUE_WINNER`, `ROUE_LETTERS[].TEAM`.
7. Actions (`messages.go`), liste blanche entrante par émetteur (§8.2), `handleWebMessage`, rappels de diffusion.
8. Sérialiseur : retrait de `QUESTION.ROUE_PHRASE` pour TV et tablette (§9).
9. `handleUploadQuestion` : champs §11.1, type sans chronomètre global ; `POST /api/roue/preview`.
10. `game-config.json` : section `wheel` (4 paramètres + durées), défauts, validation, `wheel_default` ; construction de
    la roue dans un fichier dédié `internal/game/roue_wheel.go` (§10.1) ; `POST /api/roue/wheel-preview`.
11. LED, ambiance, sites de son (registre fermé `soundSiteRegistry`).
12. `state_persistence.go` : rien à persister (commentaire de la liste à compléter).

## 14. Non-régression exigée

- Les types existants (SPEEDY, QCM, MEMORY, MEMOTION, ARDOISE, RAFALE, ENTRACTE) ne changent pas : aucun champ
  retiré, aucune action modifiée, `ProcessButtonPress` inchangé hors type `ROUE`.
- `RafaleLetterWheel` et la roue de lettres RAFALE gardent leur comportement (bascule courte) : les tests
  `Wheel.test.jsx`, `geometry.test.js`, `PlayerDisplay.rafale-alpha.282.test.jsx` restent verts sans modification.
- Un `game-config.json` sans section `wheel` se charge sans erreur (roue par défaut).

---

## 15. v12.1.0 — Achat de voyelles (#147) et case BONUS (#308)

> Révision 7 (2026-11-21 ; **rév. 7.1** : réglage « moment de l'achat » ; **rév. 7.2** : plusieurs voyelles après
> l'arrêt, réglages « 1 / plusieurs » avant et après). Décisions
> produit : `.claude/workflow/cadrage-v12.1.0-decisions.md`, y compris la section « Révision GATE 2 » (font foi). Les
> choix techniques marqués **[planner]** sont des propositions du plan, validées au GATE 2.
> **Compatibilité** : une question sans les nouveaux champs garde le comportement v12.0, **sauf l'achat de voyelles,
> activé par défaut** (décision utilisateur, questions existantes incluses) **au moment « avant de tourner »**
> **[planner]**. Aucun BREAKING : tous les ajouts sont des champs, valeurs d'énumération et actions nouvelles.

### 15.1 Achat de voyelles (#147)

#### 15.1.1 Champs de la question (`TypedContent`, `OwnedFields` du type `ROUE`)

| Champ JSON | Type | Règle |
|---|---|---|
| `ROUE_BUY_VOWELS` | bool | Achat de voyelles autorisé. **Absent = `true`** ; écrit seulement pour `false` (même patron que `ROUE_SHOW_LETTERS`) |
| `ROUE_VOWEL_TIMING` | string | **Moment de l'achat** : `BEFORE` (« Avant de tourner ») \| `AFTER` (« Après l'arrêt de la roue », à la place de la consonne) \| `BOTH` (« Les deux »). **Absent = `BEFORE`** (décision utilisateur, GATE 2 n° 2) |
| `ROUE_VOWELS_BEFORE` | string | Nombre de voyelles achetables **avant de tourner**, par tour : `ONE` (« 1 ») \| `MANY` (« plusieurs »). **Absent = `MANY`** **[planner]** |
| `ROUE_VOWELS_AFTER` | string | Nombre de voyelles achetables **après l'arrêt**, par arrêt de la roue : `ONE` \| `MANY`. **Absent = `MANY`** **[planner]** |
| `ROUE_VOWEL_PRICE` | int | Prix d'une voyelle, en points de cagnotte. 1 à 1 000 000. **Absent = 250** |
| `ROUE_VOWEL_GAIN` | string | Gain d'une voyelle **présente** : `NONE` (défaut, absent = `NONE`) \| `REFUND` \| `PER_OCCURRENCE` |

`ROUE_VOWELS_BEFORE` ne sert qu'avec `BEFORE` / `BOTH`, `ROUE_VOWELS_AFTER` qu'avec `AFTER` / `BOTH` ; tous deux sont
enregistrés quel que soit le moment.

Libellés fixés (éditeur, admin) **[planner]** :
- moment : `BEFORE` « **Avant de tourner** » ; `AFTER` « **Après l'arrêt de la roue** » (sous-titre : « à la place de
  la consonne, la case n'est pas jouée ») ; `BOTH` « **Les deux** » ;
- nombre : `ONE` « **1 voyelle** » ; `MANY` « **Plusieurs voyelles** » (sous-titre : « d'affilée, tant que la cagnotte le
  permet et que chaque voyelle est présente ») ;
- gain : `NONE` « **Sans gain** » ; `REFUND` « **Mise remboursée** » ; `PER_OCCURRENCE` « **Prix × nombre de lettres** »
  (ex. 3 E à 250 → +750).

Une voyelle **absente** n'est jamais remboursée et fait **perdre le tour**, quel que soit le moment ou l'option.

#### 15.1.2 Champs `GameState` ajoutés (§4 — mêmes règles : sans `omitempty`, jamais persistés, remis à vide dans `Ready`, `stopUnsafe`, `InitGameWith`)

| Champ | Type | Contenu |
|---|---|---|
| `ROUE_BUY_VOWELS` | bool | Copie effective du réglage au `START` (`false` hors manche) |
| `ROUE_VOWEL_TIMING` | string | Moment effectif (`BEFORE` si absent), copié au `START` (`""` hors manche) |
| `ROUE_VOWELS_BEFORE` | string | Nombre effectif avant de tourner (`MANY` si absent), copié au `START` (`""` hors manche) |
| `ROUE_VOWELS_AFTER` | string | Nombre effectif après l'arrêt (`MANY` si absent), copié au `START` (`""` hors manche) |
| `ROUE_VOWEL_PRICE` | int | Prix effectif (250 si absent), copié au `START` (0 hors manche) |
| `ROUE_VOWEL_GAIN` | string | Option effective (`NONE` si absente), copiée au `START` (`""` hors manche) |
| `ROUE_VOWELS_LEFT` | bool | Il reste au moins une voyelle de la phrase **ni révélée ni proposée** (pendant de `ROUE_CONSONANTS_LEFT`) |
| `ROUE_TURN_VOWEL` | bool | Une voyelle présente achetée **avant de tourner** a été révélée pendant le tour en cours : « Réponse validée » ouverte à l'étape 1 ; avec `ROUE_VOWELS_BEFORE = ONE`, plus d'achat avant de tourner dans ce tour. Remis à `false` à chaque entrée dans un tour (`ROUE_NEXT`, `ROUE_PASS`, case BONUS, `START`) |
| `ROUE_STOP_VOWEL` | bool | Une voyelle présente a été achetée **après l'arrêt** courant : la case tirée n'est plus jouable (aucune consonne), d'autres voyelles s'achètent à l'étape 6 si `ROUE_VOWELS_AFTER = MANY`. Remis à `false` à `ROUE_SPIN` et à chaque entrée dans un tour |

Champs existants dont le sens s'étend :
- `ROUE_SUBPHASE` : plus `VOWEL_LIT` et `VOWEL_REVEALING`, **réservées à l'achat avant de tourner** (`ROUE_STEP` = 1).
  L'achat après l'arrêt réutilise `LIT` et `REVEALING` (étape 5) puis `ANSWER` (étape 6), comme une consonne.
- `ROUE_BOARD` : `*` (case éclairée) en `LIT` **et en `VOWEL_LIT`**.
- `ROUE_LETTERS` : consonnes **et voyelles** proposées, dans l'ordre, même forme. Une entrée est une voyelle si
  `LETTER` ∈ `A E I O U Y` (aucun champ supplémentaire). Le calcul de `ROUE_CONSONANTS_LEFT` ignore les voyelles.
- `ROUE_LAST_EVENT` : `VOWEL_HIT` / `VOWEL_MISS` pour les deux moments.

#### 15.1.3 Achat **avant de tourner** (`BEFORE` ou `BOTH`) — étape 1

```
TURN ─ROUE_BUY_VOWEL (présente)─► VOWEL_LIT ─ROUE_FLIP─► VOWEL_REVEALING ─fin du retournement─► TURN (même équipe)
  │                                cagnotte − prix          cagnotte + gain (option)                ROUE_TURN_VOWEL = true
  └─ROUE_BUY_VOWEL (absente)─► TURN_END  (cagnotte − prix, « mauvaise réponse », tour perdu)
```

- `ROUE_VOWELS_BEFORE = MANY` : de retour en `TURN`, l'équipe peut acheter une autre voyelle tant que sa cagnotte le
  permet. `ONE` : une seule voyelle avant de tourner par tour (refus `VOWEL_LIMIT_REACHED` ensuite).
- Une fois la roue lancée (`ROUE_SPIN`), plus d'achat « avant » avant le prochain `TURN`. **Le prochain `TURN`** = tout
  nouveau tour : après `ROUE_NEXT` (`SAME` ou `NEXT`), `ROUE_PASS` **ou une case BONUS** (§15.2) **[planner]**.
- **De retour en `TURN` après une voyelle** : « Lancer la roue », « Acheter une voyelle » (si `MANY`), et **« Réponse
  validée »** (`ROUE_TURN_VOWEL`) **[planner]**. Une réponse fausse ne coûte rien.

#### 15.1.4 Achat **après l'arrêt de la roue** (`AFTER` ou `BOTH`) — à la place de la consonne

```
STOPPING ─case valeur─► LETTER ─ROUE_LETTER (consonne)─► … chemin v12.0 inchangé
                          │
                          └─ROUE_BUY_VOWEL (présente)─► LIT ─ROUE_FLIP─► REVEALING ─fin─► ANSWER (étape 6)
                          │   cagnotte − prix ; ROUE_STOP_VOWEL = true     cagnotte + gain     │  ├─ ROUE_ANSWER → SOLVED
                          │                                                                    │  ├─ ROUE_NEXT → tour suivant
                          │                ┌───────── ROUE_BUY_VOWEL (présente, si MANY) ◄──────┘  └─ ROUE_BUY_VOWEL (absente)
                          │                ▼                                                            → TURN_END
                          │               LIT …
                          └─ROUE_BUY_VOWEL (absente)──► TURN_END  (cagnotte − prix, tour perdu)
```

- La première voyelle **remplace la consonne** : la valeur de la case tirée **n'est pas jouée**, aucune consonne n'est
  plus acceptée jusqu'au prochain lancer (`ROUE_LETTER` refusé `WRONG_SUBPHASE` hors `LETTER`).
- **`ROUE_VOWELS_AFTER = MANY`** : après chaque voyelle présente, l'étape 6 (`ANSWER`) propose « Réponse validée »,
  « Tour suivant » / « Équipe suivante » **et « Acheter une autre voyelle »** — tant que la cagnotte le permet et qu'il
  reste une voyelle. Une voyelle absente arrête l'enchaînement : tour perdu (`TURN_END`). **[planner]** L'équipe arrête
  d'elle-même en choisissant la réponse ou l'un des deux boutons de fin de tour.
- **`ONE`** : une seule voyelle par arrêt ; à l'étape 6, pas d'achat (refus `VOWEL_LIMIT_REACHED`).
- Fin du tour (étape 7) : conseil « lettre trouvée » (§5.2) après une voyelle présente.
- **Cases spéciales** : BANQUEROUTE et TOUR PERDU mènent à `TURN_END`, BONUS à `TURN` (§15.2) : **aucun achat
  « après »** (il n'y a pas d'étape 4). Avec `BOTH`, l'achat « avant » redevient possible au `TURN` suivant.
- **Plus aucune consonne** : avec `AFTER` ou `BOTH`, `ROUE_SPIN` reste accepté en `TURN` tant qu'un achat après
  l'arrêt est possible (`ROUE_BUY_VOWELS`, `ROUE_VOWELS_LEFT`, cagnotte ≥ prix) : à l'arrêt, seules des voyelles peuvent
  être jouées. Sinon `ROUE_SPIN` est refusé, comme en v12.0 (`WRONG_SUBPHASE`).

#### 15.1.5 Règles communes

- Paiement : **cagnotte de manche** `ROUE_TEAM_BANK[équipe]` ; aucun score de partie touché avant la résolution (§7).
- **Gain d'une voyelle présente : crédité à la cagnotte au retournement (`ROUE_FLIP`), aux deux moments**
  **[planner]** — il est donc aussitôt disponible pour la voyelle suivante. `ROUE_TURN_GAIN` ne porte que le gain de la
  consonne (valeur de case × cases), inchangé.
- Retournement **au bouton** (`ROUE_FLIP`), jamais automatique (DECISIONS `admin_game_roue`) ; minuterie de
  retournement existante, suspendue par `PAUSE`, reprise par `CONTINUE`.
- **Toutes les voyelles dites** (`!ROUE_VOWELS_LEFT`) : bouton grisé « Plus de voyelle à trouver », refus `NO_VOWEL_LEFT`.
- `ROUE_REVEAL` pendant l'éclairage ou le retournement d'une voyelle : accepté ; le prix reste débité, aucun gain.
- Buzzers et tablettes joueurs : aucun rôle dans l'achat (`BUTTON` ignoré hors `SPINNING`, §5.1).

#### 15.1.6 Actions

| Action | Payload | Phases / sous-phases acceptées | Émetteurs |
|---|---|---|---|
| `ROUE_BUY_VOWEL` **[NEW]** | `{ "LETTER": "E" }` | `STARTED` + `TURN` (moment `BEFORE` / `BOTH`) ; `LETTER` (moment `AFTER` / `BOTH`) ; `ANSWER` quand `ROUE_STOP_VOWEL` (moment `AFTER` / `BOTH`) | `/ws/admin`, `/ws/anim` |
| `ROUE_SPIN` | `{}` | inchangé **+ `TURN` sans consonne restante quand un achat après l'arrêt est possible** (§15.1.4) | inchangé |
| `ROUE_FLIP` | `{}` | `STARTED` + `LIT` **ou `VOWEL_LIT`** | inchangé |
| `ROUE_ANSWER` | `{}` | inchangé **+ `TURN` quand `ROUE_TURN_VOWEL`** | inchangé |

`ROUE_BUY_VOWEL` — contrôles dans cet ordre, refus = `ROUE_ERROR` à l'émetteur seul, état inchangé, rien de débité :
1. sous-phase ni `TURN`, ni `LETTER`, ni `ANSWER` → `WRONG_SUBPHASE` ;
2. `!ROUE_BUY_VOWELS` → `VOWELS_DISABLED` (« Achat de voyelles désactivé pour cette question ») ;
3. moment non permis → `VOWEL_NOT_NOW` (« Pas d'achat de voyelle à ce moment pour cette question ») : `TURN` avec
   `AFTER` ; `LETTER` avec `BEFORE` ; `ANSWER` sans `ROUE_STOP_VOWEL` (après une consonne) ou avec `BEFORE` ;
4. limite atteinte → `VOWEL_LIMIT_REACHED` (« Une seule voyelle à ce moment pour cette question ») : `TURN` avec
   `ROUE_VOWELS_BEFORE = ONE` et `ROUE_TURN_VOWEL` ; `ANSWER` avec `ROUE_VOWELS_AFTER = ONE` ;
5. lettre normalisée (§5.3) qui n'est pas une lettre → `NOT_A_LETTER` ; consonne → `NOT_A_VOWEL` (« Ce n'est pas une voyelle ») ;
6. voyelle déjà dans `ROUE_LETTERS` → `LETTER_ALREADY_PROPOSED` (« Voyelle déjà proposée ») **[planner : refus sans
   débit, alors qu'une consonne répétée fait perdre le tour — le clavier grise déjà les lettres proposées]** ;
7. `!ROUE_VOWELS_LEFT` → `NO_VOWEL_LEFT` (« Plus de voyelle à trouver ») ;
8. cagnotte < prix → `INSUFFICIENT_BANK` (« Cagnotte insuffisante »).

Effet (un événement par action, §4) — cagnotte − prix dans tous les cas, entrée `ROUE_LETTERS` (`HIT` présente / absente) :
- **avant, présente** : cases éclairées, `VOWEL_LIT`, événement `VOWEL_HIT` { `TEAM`, `LETTER`, `COUNT`, `VALUE` = prix,
  `GAIN` = gain à venir } ; `ROUE_FLIP` → `VOWEL_REVEALING`, **cagnotte + gain**, événement `FLIP` { `LETTER`, `COUNT`,
  `GAIN` } ; fin de la minuterie → `TURN`, même équipe, `ROUE_TURN_VOWEL = true`, **aucun événement** (`SEQ` inchangé) ;
- **après, présente** (depuis `LETTER` ou `ANSWER`) : `ROUE_STOP_VOWEL = true`, cases éclairées, `LIT`, événement
  `VOWEL_HIT` (mêmes champs) ; `ROUE_FLIP` → `REVEALING`, **cagnotte + gain**, événement `FLIP` ; fin de la minuterie →
  `ANSWER` (étape 6, conseil « lettre trouvée »), aucun événement ;
- **absente (les deux moments)** : `TURN_END`, événement `VOWEL_MISS` { `TEAM`, `LETTER`, `VALUE` = prix } ; aucun
  événement `TURN_END` (règle de cause, §4).

#### 15.1.7 Codes `ROUE_ERROR` ajoutés (§8.3)

`NOT_A_VOWEL`, `VOWELS_DISABLED`, `VOWEL_NOT_NOW`, `VOWEL_LIMIT_REACHED`, `INSUFFICIENT_BANK`, `LETTER_ALREADY_PROPOSED`,
`NO_VOWEL_LEFT`.

### 15.2 Case BONUS (#308)

#### 15.2.1 Champ de la question

| Champ JSON | Type | Règle |
|---|---|---|
| `ROUE_BONUS_COUNT` | int | Nombre de cases BONUS ajoutées à la roue générée. **0 à 4** **[planner]** ; **absent = 0** (non écrit pour 0) |

#### 15.2.2 Roue générée (§10.1)

- `ROUE_WHEEL.SEGMENTS[].KIND` ∈ `value | bankrupt | lose_turn | bonus`. Une case BONUS a `VALUE` 0.
- Au `START`, la roue est construite à partir de la section `wheel` de `game-config.json` **et** de
  `ROUE_BONUS_COUNT` de la question. Les cases BONUS **s'ajoutent** (elles ne prennent la place d'aucune valeur).
- **Placement [planner]** : les cases spéciales sont réparties comme en v12.0, avec `K` = Banqueroute + Perdu +
  BONUS. Ordre des types : la première Banqueroute (à côté du maximum, inchangé), puis en alternance Perdu, BONUS,
  Banqueroute… **Avec `ROUE_BONUS_COUNT = 0`, la roue est identique, case pour case, à celle de v12.0** (test de
  non-régression exigé).
- Limite de 64 cases : à l'enregistrement de la question, `ROUE_BONUS_COUNT` qui ferait dépasser 64 avec la
  configuration courante → refus `ROUE_BONUS_COUNT_INVALID`. Au `START` (configuration modifiée depuis), le nombre de
  cases BONUS est réduit pour tenir dans 64, avec un journal ; le départ n'est jamais refusé.
- Libellé de la case **[planner]** : « 🎁 BONUS » (TV, tablettes, admin, animateur, aperçus), case dorée.

#### 15.2.3 Arrêt sur BONUS

Fin du freinage sur une case `bonus` (§5.1, `STOPPING`) :
- **aucun gain, aucune lettre, aucune cagnotte ni score modifié** — pas d'achat de voyelle « après l'arrêt » ;
- `ROUE_WHEEL.CURRENT` = la case BONUS ; événement `BONUS` { `TEAM` } ;
- **l'équipe garde la main** : entrée dans un nouveau tour (`TURN`, `ROUE_TURN_GAIN` 0, `ROUE_TURN_VOWEL` false) —
  elle peut relancer la roue, acheter une voyelle si le moment `BEFORE` / `BOTH` le permet, ou « Passer la main » (admin) ;
- annonce TV et tablettes : bandeau « 🎁 BONUS ! » tant que `ROUE_LAST_EVENT.KIND = BONUS` (jusqu'au geste suivant).
- Le cadeau est géré **hors système** par l'animateur ; des points éventuels passent par l'attribution manuelle
  existante (page Scores / `TEAM_POINTS`).

### 15.3 HTTP

`POST /questions` (§11.1), champs de formulaire ajoutés pour `type=ROUE` (tous relus à chaque enregistrement) :

| Champ de formulaire | Champ enregistré | Refus `400` |
|---|---|---|
| `roue_buy_vowels` (`true` / `false`) | `ROUE_BUY_VOWELS` (écrit seulement pour `false`) | — |
| `roue_vowel_timing` | `ROUE_VOWEL_TIMING` | `ROUE_VOWEL_TIMING_INVALID` |
| `roue_vowels_before` (`ONE` / `MANY`) | `ROUE_VOWELS_BEFORE` | `ROUE_VOWELS_BEFORE_INVALID` |
| `roue_vowels_after` (`ONE` / `MANY`) | `ROUE_VOWELS_AFTER` | `ROUE_VOWELS_AFTER_INVALID` |
| `roue_vowel_price` | `ROUE_VOWEL_PRICE` | `ROUE_VOWEL_PRICE_INVALID` (pas un entier de 1 à 1 000 000) |
| `roue_vowel_gain` | `ROUE_VOWEL_GAIN` | `ROUE_VOWEL_GAIN_INVALID` |
| `roue_bonus_count` | `ROUE_BONUS_COUNT` (non écrit pour 0) | `ROUE_BONUS_COUNT_INVALID` (hors 0–4, ou roue de plus de 64 cases) |

Champ absent du formulaire = valeur par défaut (achat activé, `BEFORE`, `MANY`, `MANY`, 250, `NONE`, 0). Moment, nombres,
prix et option sont enregistrés même quand l'achat est désactivé (conservés pour une réactivation).

`POST /api/roue/wheel-preview` (§11.3) : champ facultatif `bonus_count` (0 à 4, absent = 0), même placement qu'au
`START`. Additif : la carte de réglage existante ne l'envoie pas ; l'éditeur l'envoie pour l'aperçu de la roue.

### 15.4 Périphériques et sons (§12)

| Événement | Son (existant) | LED / ambiance |
|---|---|---|
| `VOWEL_MISS` | « mauvaise réponse » (`CueRafaleMauvaise`, notice `BAD`, comme `LETTER_MISS`) | inchangé |
| `VOWEL_HIT`, `FLIP` de voyelle, débit / crédit de cagnotte | aucun | inchangé |
| `BONUS` | « bonne réponse » (`CueRafaleBonne`, nouvelle notice `BONUS`) **[planner]** | comète sur les buzzers de l'équipe (effet de gain existant) ; **ni flash de score ni pulsation d'ambiance** (aucun point) |

**Aucun nouveau bruitage**, aucun changement de protocole BuzzClick. La tablette joueur n'envoie aucune action
d'achat : elle affiche seulement (alphabet, cagnotte débitée, bandeau BONUS). Le nouveau site de son est inscrit au
registre fermé `soundSiteRegistry`.

### 15.5 Non-régression

- `ROUE_BONUS_COUNT` absent : roue identique à v12.0 (même ordre, mêmes `ID`).
- `ROUE_BUY_VOWELS = false` : manche strictement v12.0 (refus `VOWELS_DISABLED`, aucun champ visible changé hors
  copies de réglage) ; `ROUE_SPIN` refusé sans consonne restante, comme en v12.0.
- `ROUE_LETTER` refuse toujours une voyelle (`VOWEL_NOT_AVAILABLE`) : l'achat après l'arrêt passe par `ROUE_BUY_VOWEL`.
- Sous-phases, étapes et boutons v12.0 inchangés pour le tour classique (consonne, réponse, fin de tour).
