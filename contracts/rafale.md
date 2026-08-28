# Contrat — Mode de jeu RAFALE (v8.0.0, milestone #16)

**Issues** : #107 (moteur solo) · #197 (réservoir + éditeur) · #198 (interface TV/anim) · #199 (modes multi)
**Statut** : contrat de référence, écrit avant implémentation (contract-first)
**Arbitrages** : `_work/reports/plan-ambiguities-20260828-155055.md` (B1–B6, B6b, D1–D4)

> Ce fichier est la **référence unique** pour `dev-backend` et `dev-frontend` sur RAFALE.
> En cas de divergence backend/frontend, ce document tranche.
> Le backend PEUT modifier un contrat sous contrainte technique — en documentant la raison ici.

---

## 1. Vocabulaire

| Terme | Définition |
|---|---|
| **Manche** | Une exécution du mode RAFALE, de `START` jusqu'à l'expiration du timer global. Une partie peut en contenir plusieurs. |
| **Réservoir** | Base globale de questions dédiée à RAFALE, indépendante des questions Quiz/MEMOTION. |
| **Pool** | Sous-ensemble du réservoir correspondant au filtre d'une manche : `catégories sélectionnées ∩ difficulté ∩ non utilisée`. |
| **Compteur de manche** | Nombre de bonnes réponses d'une équipe pendant la manche. **N'est pas un score réel** — voir §6. |
| **Barème de manche** | Valeur en points d'une bonne réponse, définie **au niveau de la manche** (pas par question). |

---

## 2. Décisions d'architecture

### 2.1 RAFALE est un `QuestionType`, pas une phase

Les phases restent inchangées (`STOPPED / PREPARE / READY / COUNTDOWN / STARTED / PAUSED / REVEALED`).
RAFALE suit le patron **MEMOTION** : un type de question porteur d'une **configuration de manche**
(il ne porte aucun énoncé — les énoncés viennent du réservoir), avec sous-phases et champs
`GameState` dédiés.

### 2.2 Le timer global de manche RÉUTILISE le timer existant

Décision structurante, conséquence de l'arbitrage **B1** (timer unique partagé, tous modes) :

- **Timer de manche** = timer global existant, **sans code nouveau**. `Question.TIME` alimente
  `CURRENT_TIME`, décrémenté par `processTimerTick()`, diffusé par `OnTimerTick` → `UPDATE_TIMER`.
  L'expiration déclenche déjà `Stop()` — ce qui **est** la fin de manche.
- **Timer de question** (~3 s) = **seul mécanisme réellement nouveau**. Ticker dédié
  (`rafaleQuestionTimer` / `rafaleQuestionStopCh`), champ `RAFALE_QUESTION_TIME`, diffusion par
  l'action légère `RAFALE_TICK`.

> ⚠️ Les deux tickers tournent **simultanément** — une première dans ce codebase (`Engine.timer`
> et le ticker carte MEMOTION sont aujourd'hui mutuellement exclusifs). Le ticker RAFALE doit
> avoir ses **propres** champs, ne jamais toucher `e.timer`/`e.stopCh`, et suivre la discipline
> établie : `defer e.mu.Unlock()` dans le tick, `recoverBackgroundPanic`, callback hors verrou.

### 2.3 La réponse attendue ne transite JAMAIS par `GameState`

L'animateur doit voir la réponse pour juger ; la TV ne doit pas. Or `SerializeForWebClient`
sert **le même payload à `/tv` et `/anim`** — aucune liste d'exclusion ne peut les séparer.

→ La réponse est diffusée par une **action dédiée** `RAFALE_ANSWER`, envoyée via
`BroadcastToTypes(msg, ClientTypeAdmin, ClientTypeAnim)` uniquement.

**Justification** : précédent de fuite `ardoise_leak_128` (la réponse d'une équipe atteignait des
clients qui ne devaient pas la voir). Mettre `ANSWER` dans `GameState` reproduirait la faille,
même si aucun composant TV ne l'affiche — la donnée serait dans la charge réseau.

> ⚠️ `serializeForClientType` (`websocket.go:536-546`) : `ClientTypeAnim` **doit** figurer au
> `case`. Omis, l'action tombe dans le `default:` et envoie la charge admin complète à `/anim`.

### 2.4 Stockage du réservoir : fichier unique, pas de répertoire par question

Les questions du réservoir sont **texte seul** (arbitrage D3) — aucun média, donc aucun besoin du
patron « un répertoire par question » utilisé par les questions Quiz.

→ **Fichier unique** `data/files/rafale/reservoir.json`, chargé/sauvé par une paire typée
`LoadRafale()` / `SaveRafale()` sur le modèle de `SaveStatuses`/`LoadStatuses`.

**Justification** : la lecture du répertoire des questions Quiz est **déjà dupliquée 3 fois**
(`handleQuestions`, `handleUploadQuestion`, `App.loadQuestions`), sans couche typée. Répliquer ce
patron en ferait une 4ᵉ. Le fichier unique typé évite la dette **sans toucher au code Quiz
existant** (donc sans élargir la surface de revue).

---

## 3. Modèle de données

### 3.1 `RafaleQuestion` — élément du réservoir (nouveau)

```go
type RafaleQuestion struct {
    ID         string           `json:"ID"`         // identifiant stable, unique dans le réservoir
    Question   string           `json:"QUESTION"`   // énoncé, texte seul
    Answer     string           `json:"ANSWER"`     // réponse attendue, texte seul
    Category   QuestionCategory `json:"CATEGORY"`   // réutilise l'enum existant + catégories custom
    Difficulty int              `json:"DIFFICULTY"` // 1..3 — échelle MEMOTION (étoiles)
}
```

- `CATEGORY` réutilise `QuestionCategory` **et** les catégories personnalisées découvertes dans
  `data/files/categories/` — même vocabulaire que le reste du projet, aucune liste parallèle.
- `DIFFICULTY` réutilise l'échelle 1–3 de `MotionCard.Difficulty` (arbitrage D2).
- **Aucun champ média** (arbitrage D3).

**Fichier** : `data/files/rafale/reservoir.json` — `{"QUESTIONS": [RafaleQuestion, ...]}`

### 3.2 Flag « déjà utilisée » — fichier séparé

**Fichier** : `data/config/rafale_used.json` — `{"USED": {"<id>": true, ...}}`

Séparé du réservoir **volontairement** : éditer le réservoir ne doit pas réécrire l'état de jeu,
et jouer ne doit pas réécrire le réservoir. Patron `SaveStatuses`/`LoadStatuses`.

- **Persistant** — survit à un redémarrage (arbitrage B6).
- **Réinitialisé automatiquement dans `InitGame()`** (`NEW_GAME`), aux côtés des resets MEMOTION
  et ARDOISE existants, avec `safeGo("SaveRafaleUsed", ...)`. Pas de bouton dédié.

### 3.3 Configuration de manche — `Question` de type `RAFALE`

Champs **réutilisés** de `Question` (mutualisation, pas de doublon) :

| Champ existant | Usage en RAFALE |
|---|---|
| `TIME` | Durée totale de la manche, en secondes (défaut `120`) |
| `POINTS` | **Barème de manche** : valeur en points d'une bonne réponse (arbitrage B4) |
| `CATEGORY` | *Non utilisé* — RAFALE porte `RAFALE_CATEGORIES` (multi) |

Champs **nouveaux**, portés par `TypedContent` (donc `OwnedFields` du type RAFALE) :

```go
RafaleCategories   []string `json:"RAFALE_CATEGORIES"`    // multi-sélection, ≥1
RafaleDifficulty   int      `json:"RAFALE_DIFFICULTY"`    // 1..3, unique par manche
RafaleMode         string   `json:"RAFALE_MODE"`          // voir §3.4
RafaleQuestionTime int      `json:"RAFALE_QUESTION_TIME"` // secondes par question, défaut 3
RafaleMaxQuestions int      `json:"RAFALE_MAX_QUESTIONS"` // plafond dur, défaut 100, max 100
```

### 3.4 `RAFALE_MODE`

| Valeur | Règle |
|---|---|
| `SOLO` | Une seule équipe joue. Aucune rotation. (#107) |
| `CHACUN_SON_TOUR` | Bonne **ou** mauvaise réponse → passage à l'équipe suivante. (#199) |
| `TANT_QUE_JE_GAGNE` | Bonne réponse → l'équipe garde la main. Mauvaise → équipe suivante. (#199) |
| `MAILLON_FAIBLE` | Comme `CHACUN_SON_TOUR`, mais le compteur de l'équipe retombe à **0** sur mauvaise réponse ; le meilleur compteur atteint est mémorisé. (#199) |

---

## 4. Champs `GameState`

**Aucun `omitempty`** (règle projet). Slices et maps **initialisées non-nil** dans `NewEngine()`
et à chaque reset — jamais `nil`.

```go
RafaleSubPhase          RafaleSubPhase  `json:"RAFALE_SUBPHASE"`
RafaleCurrentQuestion   RafaleCurrent   `json:"RAFALE_CURRENT_QUESTION"`
RafaleQuestionTime      int             `json:"RAFALE_QUESTION_TIME"`
RafaleTeamCounters      map[string]int  `json:"RAFALE_TEAM_COUNTERS"`
RafaleTeamBest          map[string]int  `json:"RAFALE_TEAM_BEST"`
RafaleCurrentTeam       string          `json:"RAFALE_CURRENT_TEAM"`
RafaleParticipatingTeams []string       `json:"RAFALE_PARTICIPATING_TEAMS"`
RafaleCurrentTeamColor  []int           `json:"RAFALE_CURRENT_TEAM_COLOR"`
RafaleAskedCount        int             `json:"RAFALE_ASKED_COUNT"`
RafalePoolRemaining     int             `json:"RAFALE_POOL_REMAINING"`
RafaleExhausted         bool            `json:"RAFALE_EXHAUSTED"`
```

```go
// Question courante SANS la réponse — cf. §2.3
type RafaleCurrent struct {
    ID         string `json:"ID"`
    Question   string `json:"QUESTION"`
    Category   string `json:"CATEGORY"`
    Difficulty int    `json:"DIFFICULTY"`
}

type RafaleSubPhase string
const (
    RafaleSubPhaseNone     RafaleSubPhase = ""          // inactif
    RafaleSubPhaseQuestion RafaleSubPhase = "QUESTION"  // question posée, timer question actif
    RafaleSubPhaseRoundEnd RafaleSubPhase = "ROUND_END" // manche terminée, attribution en attente
)
```

**Notes** :
- `RAFALE_TEAM_COUNTERS` / `RAFALE_TEAM_BEST` reprennent le patron `MemoryTeamPairs` /
  `MemoryTeamErrors` (arbitrage B3, précédent explicitement demandé).
- `RAFALE_TEAM_BEST` n'est alimenté qu'en `MAILLON_FAIBLE` ; présent et vide ailleurs.
- **Persistance** : tous ces champs sont **éphémères**, exclus de `PersistedGameState`
  (précédent `MotionActive`). Seul `rafale_used.json` persiste.
- Aucun de ces champs ne rejoint `AdminOnlyGameFields` ni `VPlayerOnlyGameFields` — ils sont
  diffusés à tous les clients (les listes sont des listes d'exclusion). La réponse attendue est
  le **seul** élément protégé, et elle n'est pas ici (§2.3).

---

## 5. Actions WebSocket

### 5.1 Client → Serveur

| Action | Payload | Clients autorisés | Effet |
|---|---|---|---|
| `RAFALE_VALIDATE` | `{}` | `admin`, `anim` | Réponse jugée correcte |
| `RAFALE_INVALIDATE` | `{}` | `admin`, `anim` | Réponse jugée incorrecte |
| `RAFALE_SET_TEAMS` | `{"TEAMS": ["team_A", ...]}` | `admin` | Équipes participantes + ordre de passage |

```json
{ "ACTION": "RAFALE_VALIDATE",   "MSG": {} }
{ "ACTION": "RAFALE_INVALIDATE", "MSG": {} }
{ "ACTION": "RAFALE_SET_TEAMS",  "MSG": { "TEAMS": ["team_A", "team_B"] } }
```

> ⚠️ **Allow-list fermée.** Chaque action ci-dessus doit être déclarée dans
> `internal/server/inbound_allowlist.go`. Une action absente est **rejetée silencieusement**
> (simple `Warn` en log) — symptôme : « le bouton ne fait rien », sans erreur visible.

**Aucune action d'attribution de points n'est créée.** L'attribution de fin de manche réutilise
l'action existante `TEAM_POINTS` (`{TEAM, POINTS}`), déclenchée par le clic sur une équipe —
précédent `TeamCard.onTeamClick` → `setTeamPoints` (arbitrage B3).

### 5.2 Serveur → Client

| Action | Payload | Destinataires | Effet |
|---|---|---|---|
| `RAFALE_ANSWER` | `{"ID": "...", "ANSWER": "..."}` | **`admin` + `anim` uniquement** | Réponse attendue de la question courante (§2.3) |
| `RAFALE_TICK` | `{"QUESTION_TIME": 2}` | tous | Décompte du timer question (diffusion légère, ne réémet pas tout `GameState`) |

Le timer de manche utilise l'action existante `UPDATE_TIMER` (`CURRENT_TIME`) — inchangée.

---

## 6. Scoring

### 6.1 Pendant la manche — compteur, pas de points

**Aucun point réel n'est attribué pendant la manche** (arbitrage B3).

| Événement | `CHACUN_SON_TOUR` | `TANT_QUE_JE_GAGNE` | `MAILLON_FAIBLE` | `SOLO` |
|---|---|---|---|---|
| Réponse valide | `counter[team]++` puis rotation | `counter[team]++`, garde la main | `counter[team]++` ; `best[team] = max(best, counter)` ; rotation | `counter[team]++` |
| Réponse invalide | rotation | rotation | `counter[team] = 0` ; rotation | — |
| Timer question à 0 | identique à « réponse invalide » | identique | identique | — |

**Aucune pénalité numérique, aucun score négatif** (arbitrage B5) — la pénalité est temporelle
(temps perdu + perte de la main). Le moteur n'écrit **aucun** chemin de décrément.

### 6.2 En fin de manche — attribution par l'animateur

Sous-phase `ROUND_END`. L'admin/animateur clique une équipe → `TEAM_POINTS`.

**Valeur suggérée pré-remplie par l'interface** (ajustable avant envoi) :

```
points_suggérés = compteur_retenu × POINTS
  où compteur_retenu = RAFALE_TEAM_BEST[team]   en MAILLON_FAIBLE
                     = RAFALE_TEAM_COUNTERS[team] sinon
```

L'animateur reste libre de n'attribuer les points qu'à l'équipe gagnante, ou à chacune —
conformément au cadrage du milestone #16.

---

## 7. Pioche

```
pool = { q ∈ réservoir |
           q.CATEGORY ∈ RAFALE_CATEGORIES
         ∧ q.DIFFICULTY == RAFALE_DIFFICULTY
         ∧ ¬used[q.ID] }
```

- Tirage **aléatoire uniforme** dans `pool`.
- La question tirée est immédiatement flaggée `used[q.ID] = true` **et persistée**
  (`safeGo("SaveRafaleUsed", ...)`) — un redémarrage en pleine manche ne la reproposera pas.
- `RAFALE_POOL_REMAINING` est recalculé à chaque tirage et diffusé.

### 7.1 Épuisement en cours de manche

Si `pool` est vide alors que le timer de manche tourne encore :
`RAFALE_EXHAUSTED = true`, sous-phase → `ROUND_END`, arrêt du timer de manche, message explicite
à l'animateur. **Jamais de reproposition silencieuse d'une question déjà vue.**

### 7.2 Prévention avant démarrage (arbitrage B6b)

Avant le lancement, l'admin voit le nombre de questions disponibles pour son filtre et une
estimation du besoin :

```
besoin_estimé = plafond( TIME / RAFALE_QUESTION_TIME )
```

*(estimation majorante : elle suppose que chaque question consomme tout son temps. Le besoin réel
est supérieur si les validations sont rapides — l'estimation est donc un plancher de sécurité, à
présenter comme tel.)*

| Condition | Signalement admin |
|---|---|
| `disponibles == 0` | **Bloquant** — démarrage refusé |
| `disponibles < besoin_estimé` | **Avertissement** — démarrage autorisé, risque de fin anticipée |
| `disponibles ≥ besoin_estimé` | Information neutre |

**Plafond dur** : `RAFALE_MAX_QUESTIONS`, défaut **100**, maximum **100**. Une manche s'arrête
lorsque `RAFALE_ASKED_COUNT` l'atteint, même si le timer tourne encore.

> **Choix de la valeur 100** : à 3 s par question, 100 questions représentent 5 minutes de jeu —
> soit 2,5× la durée nominale d'une manche (2 mn). Le plafond ne peut donc jamais interrompre une
> manche de durée normale ; il ne sert que de garde-fou contre une configuration aberrante
> (durée très longue, temps par question très court) qui viderait le réservoir.

---

## 8. Comportement des périphériques (arbitrage D4, amendé le 2026-08-28)

### 8.1 Principe — passif partout

**Aucun élément interactif** n'est ajouté au VPlayer ni aux buzzers pendant RAFALE. Les
indicateurs ci-dessous sont **strictement passifs** : ils informent, ils ne reçoivent rien.

| Surface | Comportement pendant RAFALE |
|---|---|
| Buzzers physiques | Appui **toujours ignoré** côté serveur (garde dans `ProcessButtonPress`). **LED pilotées** — voir §8.3. |
| VPlayer (`/player`) | **Indicateur passif « c'est ton tour »** sur la tablette de l'équipe active (§8.2). Aucun bouton, aucune saisie. |
| TV (`/tv`) | Timers, question, compteurs, **indicateur fort de l'équipe active**. **Jamais la réponse** (§2.3). |
| Animateur (`/anim`) | Boutons VALIDE / INVALIDE + réponse attendue + équipe active. |
| Admin (`/admin`) | Idem animateur + configuration de manche + attribution des points. |

> **Amendement de D4.** L'arbitrage initial disait « LED éteintes » et « VPlayer inchangé ».
> Il est amendé : les LED sont **pilotées** et le VPlayer **affiche** un indicateur. La règle de
> fond est inchangée — *aucun appui buzzer n'est traité, aucune interaction joueur n'existe*.

### 8.2 Indicateur « équipe active » — VPlayer et TV

Actif uniquement en mode multi (`RAFALE_MODE ≠ SOLO`).

| Surface | Rendu attendu |
|---|---|
| VPlayer de l'équipe active | Élément **plein écran, gros et fort**, aux couleurs de l'équipe : « À VOUS DE RÉPONDRE ». Passif. |
| VPlayer des autres équipes | Indication neutre de l'équipe qui a la main, sans appel à l'action. |
| TV | Bandeau **fort et lisible de loin** : nom + couleur de l'équipe active, visible par toute la salle. |

**Données requises côté client** : `RAFALE_CURRENT_TEAM`, `RAFALE_CURRENT_TEAM_COLOR`,
`RAFALE_PARTICIPATING_TEAMS`. Ce sont des champs `GameState` ordinaires, donc diffusés à tous les
types de clients par défaut (les listes de filtrage sont des listes d'**exclusion**).

> ⚠️ **À vérifier explicitement** : `SerializeForVPlayer(playerID)` construit sa **propre** carte
> de champs et applique une réduction par destinataire. Il faut confirmer par test que les champs
> `RAFALE_*` traversent bien ce chemin, sinon l'indicateur restera vide côté VPlayer alors qu'il
> fonctionne sur TV et `/anim` — panne asymétrique, difficile à diagnostiquer.

Le VPlayer identifie « son » équipe via `Bumper.Team` du joueur, comparé à `RAFALE_CURRENT_TEAM`.

### 8.3 LED des buzzers — équipe active

Réutilise l'infrastructure LED server-driven existante (`docs/LED_SET_PROTOCOL.md`).

**Précédent à décliner** : `sendLEDSetMemoryMultiTeam` (`cmd/server/main.go:4065`) fait déjà
exactement cela pour MEMORY en multi-équipes. RAFALE reprend la même grille :

| Buzzer | Effet LED |
|---|---|
| Équipe **active** | `SOLID`, couleur d'équipe, `INTENSITY = 255` |
| Équipe **suivante** | `SOLID`, couleur d'équipe, `INTENSITY = 128` |
| Autres équipes participantes | `DIM`, `INTENSITY = dimIntensityFor(rgb)` *(atténuation relative au ton, #113)* |
| Équipes non participantes | Éteint — `{0,0,0}`, `INTENSITY = 0` |
| Mode `SOLO` | Éteint pour tous |

- Couleur résolue via `COLOR_NAME` (§11 du protocole LED), **jamais** une RGB codée en dur.
- Rafraîchissement à chaque rotation d'équipe et à chaque changement de question.
- Le protocole ACK LED (`v3.8.0`) s'applique inchangé.

> **Mutualisation attendue** : `sendLEDSetMemoryMultiTeam` et son homologue RAFALE partagent la
> même logique (actif / suivant / participant / absent). Extraire un helper commun paramétré par
> (équipe active, équipes participantes) — **avec non-régression MEMORY obligatoire**. C'est la
> deuxième factorisation du lot, aux côtés de la rotation d'équipe (§12).

---

## 9. Endpoints HTTP (#197 — éditeur du réservoir)

Corps **JSON** (pas de multipart : aucun média, arbitrage D3).

### GET /api/rafale/questions

Liste du réservoir. Filtres optionnels : `?categories=HISTORY,SCIENCE&difficulty=2`

**Réponse 200** :
```json
{
  "QUESTIONS": [
    { "ID": "r-001", "QUESTION": "Capitale de l'Italie ?", "ANSWER": "Rome",
      "CATEGORY": "GEOGRAPHY", "DIFFICULTY": 1, "USED": false }
  ],
  "TOTAL": 1
}
```
`USED` est **dérivé à la lecture** depuis `rafale_used.json` — il n'est pas stocké dans le
réservoir (précédent : `STATUS` injecté dans les questions Quiz).

### POST /api/rafale/questions

Création (sans `ID`) ou modification (avec `ID`).

**Corps** :
```json
{ "ID": "r-001", "QUESTION": "...", "ANSWER": "...", "CATEGORY": "GEOGRAPHY", "DIFFICULTY": 2 }
```
**Réponse 200** : `{ "ID": "r-001" }`
**Erreurs** : `400` (énoncé ou réponse vide, `DIFFICULTY` hors 1–3, catégorie inconnue)

### DELETE /api/rafale/questions/{id}

**Réponse 200** : `{ "DELETED": "r-001" }` · **Erreurs** : `404`

### GET /api/rafale/pool

Comptage pour l'alerte pré-manche (§7.2). `?categories=A,B&difficulty=2`

**Réponse 200** :
```json
{ "AVAILABLE": 42, "USED": 8, "TOTAL": 50 }
```

---

## 10. Sauvegarde / restauration / réinitialisation

⚠️ **Risque de perte de données silencieuse.** La sauvegarde *sélective*
(`handleBackupSelect`), la réinitialisation sélective (`handleResetSelect`), la détection
d'archive (`detectTARContents`) et la restauration (`handleRestore`) reposent sur une **liste de
chemins codée en dur**. Sans ajout explicite, le réservoir serait :
- **inclus** dans la sauvegarde intégrale (archive de l'arbre entier) — donc le trou est discret ;
- **exclu** de la sauvegarde sélective, de la restauration et de la réinitialisation.

À ajouter explicitement, des deux côtés :

| Emplacement | Ajout |
|---|---|
| `handleBackupSelect` | drapeau `rafale`, `addDirToTAR("files/rafale")` + `addFileToTAR("config/rafale_used.json")` |
| `handleResetSelect` | drapeau `rafale` → purge réservoir **et** flags |
| `detectTARContents` | détection de `files/rafale` |
| `handleRestore` | branche de restauration |
| `BackupPage.jsx` | case à cocher « Questions RAFALE » |

---

## 11. Checklist d'accroche backend (chaque point est un oubli possible)

1. `models.go` — `QuestionTypeRafale QuestionType = "RAFALE"`
2. `models.go` — ajout à `AllQuestionTypes()`
3. `question_types.go` — entrée `questionTypeRegistry` (`OwnedFields` = les 5 champs de §3.3,
   `MediaSlots` vide, `NestableInMotionCard: false`, `HasPlayerInput: false`)
4. ⚠️ **Garde de build** : `TestQuestionTypeRegistry_Exhaustive` **casse la compilation** si 2 et 3 divergent
5. `models.go` — champs `GameState` de §4, sans `omitempty`, initialisés non-nil dans `NewEngine()`
6. `messages.go` — constantes d'action + payloads de §5
7. `inbound_allowlist.go` — entrées pour les 3 actions entrantes (**allow-list fermée**)
8. `websocket.go:536-546` — `ClientTypeAnim` présent au `case` de `serializeForClientType`
9. `main.go` — handlers `handleRafale*` + `case` dans le switch de `handleWebMessage`
10. `state_persistence.go` — champs `Rafale*` **exclus** de `PersistedGameState`
11. `engine.go` `InitGame()` — reset du flag « déjà utilisée » + `safeGo("SaveRafaleUsed", ...)`
12. `engine.go` `ProcessButtonPress` — garde d'ignorance des buzzers en RAFALE
13. `main.go` `App.init()` — `SetRafalePath` / `SetRafaleUsedPath`, chargement au démarrage

---

## 12. Non-régression exigée

| Zone | Risque | Vérification |
|---|---|---|
| MEMOTION | Le helper de rotation d'équipe est factorisé et partagé | Suite `engine_memotion_test.go` intégralement au vert |
| MEMORY — LED | Le helper LED multi-équipes est factorisé et partagé | LED MEMORY multi-équipes inchangées (actif 255 / suivant 128 / DIM / éteint) |
| VPlayer | Les champs `RAFALE_*` traversent `SerializeForVPlayer` | Test dédié : `RAFALE_CURRENT_TEAM` présent dans la charge VPlayer en `STARTED` |
| Buzzers | La LED est pilotée mais l'appui reste ignoré | Test : appui buzzer pendant RAFALE → aucun effet sur l'état de jeu |
| Timer global | Le ticker RAFALE ne touche pas `e.timer`/`e.stopCh` | Question SPEEDY/QCM/MEMORY : timer inchangé |
| Registre de types | Ajout d'un type | `TestQuestionTypeRegistry_Exhaustive` |
| Fuite de réponse | `ANSWER` hors `GameState` | Test de sérialisation `/tv` et `/player` (patron `ardoise_leak_128_test.go`) |
| Charge `/anim` | `ClientTypeAnim` au `case` | Patron `websocket_anim_test.go` |
