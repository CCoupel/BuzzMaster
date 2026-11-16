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
**Hors périmètre (v12.1)** : round bonus final (#146), achat de voyelles (#147), round spécial (#148)

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
  n'apparaissent qu'à la résolution.
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
  "SPIN_MAX_MS": 10000,
  "FLIP_INTERVAL_MS": 600,
  "SEGMENTS": [ { "ID": "s0", "KIND": "value", "VALUE": 100 }, { "ID": "s3", "KIND": "bankrupt", "VALUE": 0 } ],
  "CURRENT": -1,
  "STOP": null
}
```
- `LAYOUT` : `ROUE_WHEEL_LAYOUT` de la question. `SEGMENTS` et durées : copie de la configuration (§10) prise au
  `START`, inchangée pendant la manche. `KIND` ∈ `value | bankrupt | lose_turn`.
- `CURRENT` : index de la case sous le pointeur une fois la roue arrêtée ; conservé jusqu'au lancer suivant ;
  `-1` avant le premier lancer de la manche. La roue reste affichée en permanence.
- `STOP` : `null`, ou `{ "TO": int, "DURATION_MS": int }` pendant `STOPPING` (case tirée et durée du freinage).

`ROUE_LAST_EVENT.KIND` ∈ `SPIN | LAND_VALUE | BANKRUPT | LOSE_TURN | LETTER_HIT | LETTER_MISS | LETTER_REPEAT |
FLIP | ANSWER_OK | TURN_END | PASS | REVEAL`.

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
| 2 | Arrêt de la roue | `SPINNING` | Équipe qui a la main : buzz (buzzer ou tablette) — ou admin / animateur : `ROUE_STOP` (« Forcer l'arrêt ») — ou durée maximale |
| 3 | Affichage de la case | `STOPPING` | Serveur : case tirée, freinage |
| 3 bis | *(à voix haute)* | `LETTER` | 🗣 L'équipe annonce une consonne — aucun appareil, aucun changement d'état |
| 4 | Sélection de la consonne | `LETTER` | Admin ou animateur : `ROUE_LETTER`. Consonne présente : passage à l'étape 5 (`LIT`). Aucun `*` dans `ROUE_BOARD` à cette étape |
| 5 | Affichage des cases | `LIT` puis `REVEALING` | `LIT` : les cases de la consonne sont éclairées (`*` dans `ROUE_BOARD`, seule sous-phase où `*` apparaît). Admin ou animateur : `ROUE_FLIP` (« Retourner les cases ») → `REVEALING` : elles se retournent une à une |

`ROUE_STEP` est fonction de `ROUE_SUBPHASE` seule : `TURN` = 1, `SPINNING` = 2, `STOPPING` = 3, `LETTER` = 4,
`LIT` et `REVEALING` = 5, `ANSWER` = 6, `TURN_END` = 7, `""` et `SOLVED` = 0. L'éclairage des cases (`*`) relève donc
de l'étape 5, pas de l'étape 4 : `ROUE_LETTER` est le geste de l'étape 4, son effet (`LIT`) ouvre l'étape 5.
| 5 bis | *(à voix haute)* | `ANSWER` | 🗣 L'équipe propose une réponse (ou n'en propose pas) — aucun appareil, aucun changement d'état |
| 6 | Réponse | `ANSWER` | Admin ou animateur : `ROUE_ANSWER` (« Réponse validée », **un seul bouton** — la réponse est un bonus) — sinon directement un des deux boutons de l'étape 7 |
| 7 | Fin du tour | `TURN_END` | Gain ajouté à la cagnotte ; admin ou animateur, **deux boutons** : `ROUE_NEXT {HAND: SAME}` (« Tour suivant », la même équipe rejoue) ou `ROUE_NEXT {HAND: NEXT}` (« Équipe suivante », équipe suivante) |

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
| `TURN` | `ROUE_SPIN` (si `ROUE_CONSONANTS_LEFT`) | `SPINNING`, `STOP = null`, minuterie `SPIN_MAX_MS` armée |
| `TURN` | `ROUE_ANSWER` ou `ROUE_NEXT {HAND}` (si `!ROUE_CONSONANTS_LEFT`) | Plus aucune consonne à trouver, l'équipe ne peut que proposer : `ROUE_ANSWER` = réponse validée (`SOLVED`) ; `ROUE_NEXT` = pas de réponse validée, main selon `HAND`, reste en `TURN` |
| `TURN` | `ROUE_PASS` | Rotation immédiate, cagnotte conservée, reste en `TURN`. Refusé en `SOLO` |
| `SPINNING` | Appui (`BUTTON`) d'un buzzer ou d'une tablette **de l'équipe qui a la main**, ou `ROUE_STOP`, ou fin de `SPIN_MAX_MS` | Tirage uniforme d'une case ; `STOPPING`, `STOP = {TO, DURATION_MS}` |
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

Appuis ignorés sans effet ni diffusion : buzzer d'une autre équipe, appui hors `SPINNING`, second appui pendant
`STOPPING`. En mode `ROUE`, un appui ne pose jamais le statut `PAUSE` d'un buzzer et ne diffuse pas l'action
`PAUSE`. `PAUSE` / reprise (actions admin existantes) : toutes les minuteries du mode (durée maximale, freinage,
retournement) sont suspendues puis reprises ; toute action `ROUE_*` est refusée en phase `PAUSED`.
Toutes les minuteries sont annulées à l'arrêt de manche, à `ROUE_REVEAL` et à `NEW_GAME`.

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
- **`SPINNING`** : la roue défile en continu sur chaque écran, un tour en `SPIN_PERIOD_MS`. La position affichée
  n'a aucune valeur de jeu et n'est pas synchronisée entre écrans. Les écrans montrent le temps restant avant
  l'arrêt automatique (`SPIN_MAX_MS`, compté localement depuis la réception de `SPINNING`).
- **`STOPPING`** : chaque écran freine depuis sa position jusqu'à la case `STOP.TO` en `STOP.DURATION_MS`, avec
  au moins un tour complet (`wheel-component.md` §5). Un client qui se connecte ou se reconnecte pendant
  `STOPPING` se place directement sur `TO`.
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
| `ROUE_SET_TEAMS` | `{ "TEAMS": string[] }` | `PREPARE`, `READY` (patron `RAFALE_SET_TEAMS`) ; `SOLO` : exactement 1 équipe ; autres modes : 2 au moins |
| `ROUE_SPIN` | `{}` | `STARTED` + `TURN` + `ROUE_CONSONANTS_LEFT` |
| `ROUE_STOP` | `{}` | `STARTED` + `SPINNING` (« Forcer l'arrêt ») |
| `ROUE_LETTER` | `{ "LETTER": "T" }` | `STARTED` + `LETTER` |
| `ROUE_FLIP` | `{}` | `STARTED` + `LIT` (« Retourner les cases ») |
| `ROUE_ANSWER` | `{}` | `STARTED` + `ANSWER` ; aussi `TURN` quand `!ROUE_CONSONANTS_LEFT` — « Réponse validée » |
| `ROUE_NEXT` | `{ "HAND": "SAME" \| "NEXT" }` | `STARTED` + `TURN_END` ou `ANSWER` ; aussi `TURN` quand `!ROUE_CONSONANTS_LEFT` — `SAME` = « Tour suivant », `NEXT` = « Équipe suivante » (refusé en `SOLO`) |
| `ROUE_PASS` | `{}` | `STARTED` + `TURN`, hors `SOLO` |
| `ROUE_REVEAL` | `{}` | `STARTED`, sous-phase ≠ `SOLVED` |

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
  `VOWEL_NOT_AVAILABLE | NOT_A_LETTER | WRONG_SUBPHASE | NOT_ALLOWED | INVALID_TEAMS`.

## 9. Confidentialité de la phrase — règle non négociable

- `QUESTION.ROUE_PHRASE` est **retiré** de tout message envoyé à `/ws/tv` et `/ws/player`, dans toutes les phases
  (le plateau entièrement visible suffit après résolution). Il reste présent pour `/ws/admin` et `/ws/anim`.
- `ROUE_BOARD` ne contient jamais une lettre non révélée (`_` ou `*` seulement).
- La présentation et le champ `question`, publics, ne peuvent pas contenir la phrase (§11.1).
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
| `spin_period_ms` | Durée d'un tour de roue. 1000–6000, défaut 2500 ; hors bornes : ramené à la borne |
| `spin_max_ms` | **Durée maximale de rotation** : sans arrêt demandé, la roue s'arrête d'elle-même. 3000–30000, défaut 10000 ; hors bornes : ramené à la borne |
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
- La roue est générée au `START` de la manche et figée dans `ROUE_WHEEL.SEGMENTS` ; une modification pendant une
  manche ROUE ne touche pas la manche en cours.
- La **forme de la roue** n'est pas dans la configuration : c'est un réglage de la question (`ROUE_WHEEL_LAYOUT`).
- Sauvegarde, restauration, réinitialisation : acquises (fichier `game-config.json`, drapeau `ambiance`).

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
{ "max_points": 100, "step": 10, "bankrupt_count": 1, "lose_turn_count": 1 }
```
**Response 200** :
```json
{ "COUNT": 11, "step": 10,
  "SEGMENTS": [ { "ID": "s0", "KIND": "value", "VALUE": 100 }, { "ID": "s1", "KIND": "bankrupt", "VALUE": 0 },
                { "ID": "s2", "KIND": "value", "VALUE": 90 } ] }
```
**Errors** : `400` `{ "CODE": "ROUE_WHEEL_INVALID", "MESSAGE": "…" }` (pas ≥ maximum, maximum supérieur à 1 000 000, valeur invalide ou plus de 64 cases, §10). Un pas qui ne divise pas le
maximum n'est **pas** une erreur : il est corrigé (`step_adjusted_from`).

## 12. Périphériques et ambiance

| Élément | Comportement |
|---|---|
| Buzzers (LED) | Équipe qui a la main allumée, autres atténuées (grille active / suivante existante, `led.go`) ; cas `ROUE` ajoutés aux règles par type (reconnexion, pause, révélation) |
| Éclairage d'ambiance | `ActiveTeam` renvoie l'équipe qui a la main (scène « tour d'équipe » existante) |
| Sons | Bruitages existants seulement : gain à la résolution (crédit), « mauvaise réponse » sur lettre absente / réponse fausse / Banqueroute. **Aucun nouveau bruitage** en v12.0 (son de rotation : hors périmètre) |
| Tablettes | Affichage via `PlayerDisplay` (`isVPlayer`) ; l'appui d'une tablette de l'équipe qui a la main arrête la roue avant la durée maximale |

## 13. Checklist d'accroche backend

1. `QuestionType` `ROUE` + `AllQuestionTypes()` + registre (`question_types.go`) + `TypedContent`.
2. Champs `GameState` (§4) : init `NewEngine`, remise à zéro `Ready` / `stopUnsafe` / `InitGameWith`.
3. Logique de manche dans un fichier dédié `internal/game/roue.go` (pas dans `engine.go`) : départ, lancer, arrêt
   (trois déclencheurs), minuteries (durée maximale, freinage, retournement — annulation et pause),
   lettre, réponse, fin de tour, passe, dévoilement, crédit.
4. Garde de départ (`participantsConform`) : `ROUE_TEAMS` conforme au mode, phrase non vide.
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
