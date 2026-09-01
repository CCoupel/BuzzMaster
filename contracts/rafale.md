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
| `CATEGORY` | Catégorie du filtre de pioche — **catégorie unique**, comme tous les autres types (§3.3, bugfix ci-dessous) |

Champs **nouveaux**, portés par `TypedContent` (donc `OwnedFields` du type RAFALE) :

```go
RafaleDifficulty   int      `json:"RAFALE_DIFFICULTY,omitempty"`    // 1..3, unique par manche
RafaleMode         string   `json:"RAFALE_MODE,omitempty"`          // voir §3.4
RafaleQuestionTime int      `json:"RAFALE_QUESTION_TIME,omitempty"` // secondes par question, défaut 3
RafaleMaxQuestions int      `json:"RAFALE_MAX_QUESTIONS,omitempty"` // plafond dur, défaut 100, max 100
```

> ⚠️ **Modification de contrat (dev-backend, bugfix, 2026-08-29)** :
> `RAFALE_CATEGORIES` (`[]string`, multi-sélection) est **supprimé** —
> RAFALE réutilise désormais le champ générique `CATEGORY` de `Question`
> (catégorie **unique**), exactement comme SPEEDY/QCM/MEMORY/MEMOTION/ARDOISE.
> Retour utilisateur après plusieurs cycles où la catégorie ne s'affichait
> toujours pas correctement sur la card d'une question RAFALE malgré un
> premier correctif : RAFALE était le premier (et seul) type à introduire une
> sélection multi-catégorie, et la card générique lit `question.CATEGORY`, pas
> un champ spécifique à un type — un cas particulier qui n'avait pas sa place.
> L'alignement sur l'existant résout le bug d'affichage **structurellement**
> (plus besoin d'une branche `isRafale` dédiée côté frontend) plutôt que de le
> patcher côté client. Impact : `DrawRafaleQuestion`/`CountRafalePool`
> (engine.go) prennent désormais `category string` au lieu de
> `categories []string` ; `GET /api/rafale/pool` prend `?category=X` au lieu
> de `?categories=A,B` (§9). Le réservoir lui-même (`RafaleQuestion.CATEGORY`,
> #197) est **inchangé** — il était déjà en catégorie unique par question, ce
> n'était que le FILTRE de manche qui était multi. Voir `contracts/CHANGELOG.md`.
>
> ⚠️ **Modification de contrat (dev-backend, Batch 2/#107, toujours valable)** :
> les champs ci-dessus restent **tous** `omitempty`, y compris
> `RAFALE_DIFFICULTY` (le contrat d'origine l'en dispensait, par analogie avec
> `Question.Answer`). Contrainte technique découverte à l'implémentation :
> `models_roundtrip_test.go` (#184, garde d'exhaustivité byte-for-byte des
> fixtures `testdata/questions/*.json`) exige que tout champ `TypedContent`
> **étranger** au type d'une question n'apparaisse jamais sur le fil —
> exactement le rôle que joue `omitempty` pour `QCM_*`/`MEMORY_*`/
> `ARDOISE_KEYBOARD_TYPE`. Sans `omitempty` sur `RAFALE_DIFFICULTY`, **chaque**
> question existante (ARDOISE/MEMORY/MEMOTION/…) aurait gagné un
> `"RAFALE_DIFFICULTY": 0` parasite après un aller-retour JSON — cassant
> `TestQuestionFixtures_RoundTrip_TypedContent` pour 4 fixtures sans lien
> avec RAFALE. `Question.Answer` n'est pas un précédent transposable ici : il
> est déclaré **directement sur `Question`**, pas dans `TypedContent` partagé
> par tous les types — son absence d'`omitempty` ne fuit donc jamais vers un
> autre type. Une manche RAFALE configurée définit toujours explicitement sa
> difficulté avant `START` (même logique que `TIME`/`POINTS`, déjà des
> chaînes potentiellement vides sans que cela pose problème) ; l'admin ne
> perd donc rien en pratique.

> ⚠️ **Bugfix (dev-backend, 2026-08-31, #107, 2e cycle QUALIF du même symptôme
> externe)** : `handleUploadQuestion` (`internal/server/http.go`, gestionnaire
> `POST /questions`) n'avait **aucun** bloc de lecture pour `RAFALE_DIFFICULTY`/
> `RAFALE_MODE`/`RAFALE_QUESTION_TIME`/`RAFALE_MAX_QUESTIONS` — contrairement à
> QCM/MEMORY/MEMOTION/ARDOISE qui ont chacun leur bloc dédié. Le frontend
> (`QuestionsPage.jsx`) envoie ces 4 champs dans le formulaire multipart depuis
> la livraison de #107, mais ils étaient silencieusement perdus à
> l'enregistrement — jamais persistés dans `question.json`. Conséquence :
> `RafaleDifficulty` valait toujours `0` au chargement (`loadQuestion`), une
> valeur invalide (attendu 1..3), et la manche mourait au même endroit et par
> le même mécanisme que le bug CATEGORY du 2026-08-30 (`startRafaleRoundUnsafe`
> → pool vide pour DIFFICULTY=0 → filet de sécurité `Stop()` dans le même tick
> que `actualStart()`) — **symptôme externe identique** ("countdown de 3s puis
> plus rien"), **cause racine différente**. Corrigé par l'ajout du bloc
> manquant dans `handleUploadQuestion`, et par l'extension de la garde
> `participantsConform` (déjà étendue le 2026-08-30 pour CATEGORY) pour exiger
> aussi `1 <= RAFALE_DIFFICULTY <= 3` avant PREPARE→READY — défense en
> profondeur contre toute future régression de persistance similaire, pas
> seulement un correctif ponctuel. Voir `contracts/CHANGELOG.md`.

### 3.4 `RAFALE_MODE`

| Valeur | Règle |
|---|---|
| `SOLO` | Une seule équipe joue. Aucune rotation. (#107) |
| `CHACUN_SON_TOUR` | Bonne **ou** mauvaise réponse → passage à l'équipe suivante. (#199) |
| `TANT_QUE_JE_GAGNE` | Bonne réponse → l'équipe garde la main. Mauvaise → équipe suivante. (#199) |
| `MAILLON_FAIBLE` | Comme `CHACUN_SON_TOUR`, mais le compteur de l'équipe retombe à **0** sur mauvaise réponse ; le meilleur compteur atteint est mémorisé. (#199) |

> ⚠️ **Ajout de contrat (dev-backend, feature, 2026-09-02, #199)** : retour utilisateur QUALIF
> 8.0.0.13 — « je ne dois pas pouvoir faire START si aucune équipe n'est sélectionnée ». La garde
> `participantsConform` (§3.3, déjà étendue pour `CATEGORY`/`RAFALE_DIFFICULTY`) gagne un
> troisième cas RAFALE : en mode **multi** (`RAFALE_MODE != SOLO`), au moins une équipe doit être
> présente dans `RAFALE_PARTICIPATING_TEAMS` pour que PREPARE→READY ait lieu — sinon la manche
> reste bloquée en PREPARE, `START` structurellement refusé (même mécanisme que les deux gardes
> précédentes). `SOLO` reste exempté (aucune notion d'équipe active n'y est requise —
> `RAFALE_CURRENT_TEAM == ""` y est un no-op valide et déjà testé, §6.1). Utilise exactement le
> même défaut « `RAFALE_MODE` vide ⇒ `SOLO` » que `advanceRafaleUnsafe` (engine.go), pour que la
> garde et la logique de manche réelle ne puissent jamais diverger sur ce que « SOLO » signifie
> pour une question donnée. Contrairement aux deux gardes CATEGORY/DIFFICULTY (des bugfixes contre
> une mort silencieuse), celle-ci est une demande fonctionnelle directe, pas un correctif de
> régression — mais réutilise le même mécanisme de défense en profondeur. `SetRafaleParticipatingTeams`
> appelait déjà `reevaluatePrepareReadyUnsafe()` (prévu dès l'origine pour un futur changement de
> contrat exactement comme celui-ci, voir son propre commentaire) : vider la sélection d'équipes en
> mode multi pendant READY fait donc désormais redescendre en PREPARE automatiquement, sans code
> supplémentaire.

> ⚠️ **Bugfix (dev-backend, 2026-09-03, #199, 3e cycle QUALIF du même retour utilisateur)** : la
> garde ci-dessus est correcte et vérifiée de bout en bout (dispatch ANIM réel, chemin HTTP réel),
> mais elle pouvait être satisfaite par une sélection d'équipes **périmée**, laissée en mémoire par
> une manche PRÉCÉDENTE. Cause : une question de configuration RAFALE est **conçue pour être
> rejouée** pour plusieurs manches dans la même partie avec le **même ID** (contrairement à
> QCM/MEMORY/MEMOTION, jouées une fois chacune normalement) — mais `Engine.Ready()` ne
> réinitialisait `RAFALE_PARTICIPATING_TEAMS`/`RAFALE_CURRENT_TEAM`/`RAFALE_CURRENT_TEAM_COLOR` que
> lorsque l'ID de la question changeait (`isNewQuestion`), jamais sur un rechargement de la MÊME
> question. `Stop()` ne touche jamais ces champs non plus (par conception — la sélection doit
> survivre à un countdown, pas à une manche entièrement terminée). Résultat : lancer une manche
> multi AVEC équipes, la terminer, puis recharger la même question SANS resélectionner d'équipe
> laissait la sélection de la manche précédente satisfaire silencieusement la garde — alors que
> l'interface affichait « aucune équipe sélectionnée » (état local frontend, déconnecté du
> `GameState` réel, ce qui explique pourquoi la garde « semblait » cassée malgré une logique
> correcte en isolation). Corrigé : `Ready()` réinitialise désormais ces trois champs aussi quand
> `RAFALE_SUBPHASE != ""` (une manche a déjà été jouée sur cette question) au moment de l'appel,
> pas seulement sur `isNewQuestion` — la sélection continue de survivre à un simple re-`Ready()`
> avant tout démarrage (comportement inchangé, voir le commentaire d'origine
> « persist during PREPARE→READY transition »), mais plus après une manche terminée. Voir
> `contracts/CHANGELOG.md`.

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
RafaleTeamStreak        map[string]int  `json:"RAFALE_TEAM_STREAK"`
RafaleTeamErrors        map[string]int  `json:"RAFALE_TEAM_ERRORS"`
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
- `RAFALE_TEAM_COUNTERS` / `RAFALE_TEAM_BEST` / `RAFALE_TEAM_STREAK` / `RAFALE_TEAM_ERRORS`
  reprennent le patron `MemoryTeamPairs` / `MemoryTeamErrors` (arbitrage B3, précédent
  explicitement demandé) — `RAFALE_TEAM_ERRORS` en est le précédent EXACT.
- ⚠️ **Modification de contrat (dev-backend, bugfix, 2026-08-30)** : `RAFALE_TEAM_STREAK` et
  `RAFALE_TEAM_ERRORS` sont **nouveaux** (panneau d'équipes enrichi de la maquette §9, requis par
  le planner) ; `RAFALE_TEAM_BEST` est **redéfini**.
  - `RAFALE_TEAM_STREAK[team]` : série de bonnes réponses **en cours**, remise à 0 sur mauvaise
    réponse, dans **les 4 modes** — contrairement à `RAFALE_TEAM_COUNTERS`, qui cumule sans
    jamais retomber sauf en `MAILLON_FAIBLE` (§6.1, table inchangée). Les deux champs sont
    **distincts**, pas un renommage : un `CHACUN_SON_TOUR` où une équipe rate une question voit
    son `COUNTERS` rester inchangé (cumulatif) mais son `STREAK` retomber à 0.
  - `RAFALE_TEAM_ERRORS[team]` : nombre cumulé de réponses incorrectes/timeouts, jamais remis à
    0 en cours de manche, les 4 modes.
  - `RAFALE_TEAM_BEST[team]` — **redéfini** : maximum historique de `RAFALE_TEAM_STREAK[team]`
    (calculé une seule fois, génériquement, pour les 4 modes), au lieu d'un calcul spécifique à
    `MAILLON_FAIBLE` off `RAFALE_TEAM_COUNTERS`. Désormais alimenté dans **tous** les modes, pas
    seulement `MAILLON_FAIBLE` (la ligne suivante, obsolète, est retirée : ~~« n'est alimenté
    qu'en MAILLON_FAIBLE »~~).
  - Impact §6.2 : `compteur_retenu = RAFALE_TEAM_BEST[team]` reste valable telle quelle pour
    l'attribution de points suggérée — seule la SOURCE du maximum change (streak au lieu du
    compteur), la formule de points elle-même est inchangée.
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
           q.CATEGORY == CATEGORY
         ∧ q.DIFFICULTY == RAFALE_DIFFICULTY
         ∧ ¬used[q.ID] }
```

> Catégorie **unique** (bugfix 2026-08-29, §3.3) — anciennement une
> intersection OR sur `RAFALE_CATEGORIES` (multi). `DrawRafaleQuestion(category
> string, difficulty int)` / `CountRafalePool(category string, difficulty int)`.

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

### POST /api/rafale/questions/{id}/reset

> **Ajout de contrat (dev-backend, feature, #197)** : retour utilisateur après test manuel du
> binaire QUALIF 8.0.0.12 — le flag « déjà utilisée » (`data/config/rafale_used.json`, §3.2)
> n'avait jusqu'ici qu'un seul point de remise à zéro (`NEW_GAME`, automatique). Ces deux
> endpoints ajoutent des points d'entrée manuels, **sans toucher au réservoir lui-même** (aucune
> question n'est créée/modifiée/supprimée — seul le flag §3.2 est affecté), pour permettre de
> remettre une question précise (ou tout le pool) en disponible sans repartir sur un NEW_GAME
> complet.

Remet **une seule** question du réservoir à l'état disponible (retire son ID du flag « déjà
utilisée » — §3.2). No-op silencieux si la question n'était pas marquée utilisée. Corps : vide.

**Réponse 200** : `{ "ID": "r-001", "AVAILABLE": true }` · **Erreurs** : `404` (ID absent du
réservoir — même contrat d'erreur que `DELETE /api/rafale/questions/{id}`)

### POST /api/rafale/questions/reset-all

Remet **tout** le réservoir à l'état disponible (vide entièrement le flag « déjà utilisée » —
§3.2), indépendamment d'un `NEW_GAME`. Le réservoir lui-même (les questions) n'est **jamais**
touché — à ne pas confondre avec `POST /reset-select?rafale=true` (§10), qui **supprime** tout le
réservoir en plus du flag. Corps : vide.

**Réponse 200** : `{ "RESET": <n> }` — `<n>` = nombre d'entrées effacées du flag.

### GET /api/rafale/pool

Comptage pour l'alerte pré-manche (§7.2). `?category=A&difficulty=2` (catégorie
**unique**, bugfix 2026-08-29 — anciennement `?categories=A,B`, voir §3.3).

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
3. `question_types.go` — entrée `questionTypeRegistry` (`OwnedFields` = les 4 champs de §3.3,
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

---

## 13. Aperçu de la question suivante (#202, v8.0.0)

> **Ajout de contrat (planner, feature, 2026-09-01, #202)** — demande utilisateur après validation
> de #201 sur QUALIF 8.0.0.20 : sur `/anim`, agrandir l'énoncé de la question courante et afficher
> **la question suivante** en bas de l'encart, pour que l'animateur puisse s'y préparer.

### 13.1 Décision — pré-tirage réel, pas un aperçu

Deux conceptions étaient possibles :

| Option | Principe | Retenue |
|---|---|---|
| **(a) Pré-tirage** | La question suivante est **réellement tirée** dès que la courante est affichée, mise « sur le pont », puis **consommée** telle quelle au tick suivant. | ✅ |
| (b) Aperçu jetable | Tirage « à blanc » non consommant, régénéré à chaque tick. | ❌ |

**Justification.** §7 impose un tirage **aléatoire uniforme** dans le pool. Sous (b), la question
réellement posée serait re-tirée indépendamment de celle prévisualisée : l'animateur préparerait
un énoncé qui, dans le cas général, ne serait **pas** celui qui apparaît 3 secondes plus tard.
Un affichage faux est pire que pas d'affichage — et c'est exactement la classe de panne
« asymétrique, difficile à diagnostiquer » contre laquelle ce contrat se prémunit ailleurs
(§8.2, §12). (b) demanderait de plus un **second chemin de tirage** non consommant à travers le
pool, en parallèle de `drawRafaleQuestionUnsafe` — deux logiques de pioche à maintenir cohérentes,
pour un résultat moins juste. (a) ne duplique rien : c'est le tirage existant, simplement **avancé
d'un cran**.

### 13.2 La question suivante ne rejoint PAS `GameState`

Contrairement à ce que le champ `RAFALE_NEXT_QUESTION` évoqué dans l'issue #202 laissait supposer,
**aucun champ `GameState` n'est ajouté**. Raisonnement identique à §2.3 :
`SerializeForWebClient` sert **le même payload à `/tv` et `/anim`** — tout champ `GameState`
atteint donc la TV, et par elle la salle.

L'énoncé de la question suivante est un **avantage compétitif matériel** dans un mode où le
rythme est de ~3 s par question : un joueur qui le lit d'avance (sur sa tablette VPlayer, ou par
inspection réseau) prépare sa réponse avant que la question ne soit posée. Ce n'est pas la
réponse, mais c'est de la **même famille** que `ardoise_leak_128` : une donnée présente dans la
charge réseau de clients qui ne doivent pas l'avoir, même si aucun composant ne l'affiche.

→ **La question suivante est restreinte à `admin` + `anim`, exactement comme `RAFALE_ANSWER`.**

### 13.3 Canal — extension de `RAFALE_ANSWER`, pas une nouvelle action

`RAFALE_ANSWER` (§5.2) est **déjà** le canal privilégié `admin`+`anim`, **déjà** émis à l'instant
exact où une nouvelle question devient courante (`OnRafaleAnswer`, tiré par
`startRafaleRoundUnsafe` **et** `advanceRafaleUnsafe`). Créer une action `RAFALE_NEXT_QUESTION`
distincte dupliquerait ce canal (mêmes destinataires, même instant d'émission, même garde
d'obsolescence côté client) sans rien apporter.

Bénéfice concret de la mutualisation : la **garde anti-obsolescence** existante côté client
(`AnimPage.jsx` / `GamePage.jsx` : `rafaleAnswer.ID === RAFALE_CURRENT_QUESTION.ID`) couvre
gratuitement le nouveau champ — un `NEXT` périmé ne peut pas s'afficher sous une question
courante à laquelle il n'appartient pas. Une action séparée aurait exigé sa propre garde,
avec un risque de désynchronisation entre les deux.

**Payload étendu** (additif, rétrocompatible — `ID`/`ANSWER` inchangés) :

```json
{ "ACTION": "RAFALE_ANSWER",
  "MSG": {
    "ID": "r-042",
    "ANSWER": "Rome",
    "NEXT": { "ID": "r-017", "QUESTION": "Plus long fleuve d'Europe ?",
              "CATEGORY": "GEOGRAPHY", "DIFFICULTY": 2 }
  } }
```

```go
// Question suivante SANS sa réponse — même forme que RafaleCurrent (§4),
// type réutilisé tel quel côté moteur.
type RafaleNextPayload struct {
    ID         string `json:"ID"`
    Question   string `json:"QUESTION"`
    Category   string `json:"CATEGORY"`
    Difficulty int    `json:"DIFFICULTY"`
}

type RafaleAnswerPayload struct {
    ID     string             `json:"ID"`
    Answer string             `json:"ANSWER"`
    Next   *RafaleNextPayload `json:"NEXT"` // null = aucune question suivante (§13.5)
}
```

- **Pas d'`omitempty` sur `NEXT`** : `null` est une information utile (« il n'y a pas de suivante »),
  pas une absence. Même discipline que la règle projet « pas d'`omitempty` sur `GameState` » —
  le client ne doit jamais avoir à distinguer « champ absent » de « pas de valeur ».
- **La réponse de la question suivante n'est PAS transmise.** Elle n'est pas nécessaire (elle
  arrive au broadcast suivant, à l'instant où la question devient courante), elle doublerait la
  surface de fuite, et l'afficher alourdirait visuellement une zone dont tout l'objet est d'être
  discrète (§13.6). `RafaleNextPayload` n'a **délibérément pas** de champ `ANSWER`.
- Liste de destinataires **inchangée** : `broadcastRafaleAnswer` reste le seul appelant, avec
  `ClientTypeAdmin, ClientTypeAnim` — voir l'avertissement de §2.3, qui s'applique désormais à
  deux champs sensibles au lieu d'un.

### 13.4 Cycle de vie du pré-tirage côté moteur

Champ **privé** de `Engine` (jamais sérialisé) : `rafaleNext *RafaleQuestion`.

| Moment | Effet |
|---|---|
| `startRafaleRoundUnsafe` — après le tirage de la 1ʳᵉ question | pré-tire la 2ᵉ dans `rafaleNext` |
| `advanceRafaleUnsafe` — au lieu de tirer | **consomme** `rafaleNext` comme question courante, puis pré-tire la suivante |
| `rafaleNext == nil` au moment de consommer | fin de manche : `RAFALE_EXHAUSTED = true`, `ROUND_END` — **timing identique à l'existant** |
| Toute fin de manche (`stopUnsafe`), `Ready()` (reset de manche rejouée), `InitGame()` | **libère** `rafaleNext` : `delete(rafaleUsed, id)` + `safeGo("SaveRafaleUsed", …)` |

**Libération obligatoire.** `drawRafaleQuestionUnsafe` marque la question `used` **immédiatement**
(§7). Sans libération explicite, **chaque manche brûlerait une question jamais posée** — érosion
silencieuse du réservoir, d'autant plus coûteuse qu'il est petit. La libération n'est donc pas un
raffinement : c'est une condition de correction du pré-tirage.

**Pas de pré-tirage quand la manche va s'arrêter au plafond** : si `RAFALE_ASKED_COUNT + 1`
atteint `RAFALE_MAX_QUESTIONS` (§7.2), aucun pré-tirage n'est fait (`NEXT = null`). Évite de
consommer-puis-libérer inutilement, et affiche correctement « dernière question » sur `/anim`.

**`RAFALE_POOL_REMAINING` — sémantique préservée.** Le champ compte les questions « pas encore
posées », la question sur le pont **incluse** :

```
RAFALE_POOL_REMAINING = len(pool) + (rafaleNext != nil ? 1 : 0)
```

Sans ce `+1`, le compteur diffuse une valeur inférieure d'une unité à celle d'aujourd'hui pour la
même position de manche — régression d'affichage silencieuse.

**`RAFALE_EXHAUSTED` n'est PAS avancé.** Le pré-tirage *sait* plus tôt que le pool est vide, mais
le drapeau conserve sa sémantique actuelle (« la manche s'est terminée faute de questions ») et
reste posé au **même instant** qu'aujourd'hui, à la consommation. L'indication « dernière
question » sur `/anim` vient de `NEXT == null`, pas de `RAFALE_EXHAUSTED`.

**Coût accepté, documenté** : un arrêt brutal du serveur en pleine manche consomme **une** question
de réservoir de plus qu'aujourd'hui (la question sur le pont, marquée `used` et persistée, jamais
libérée car le processus ne passe par aucun chemin de fin de manche). Rattrapable par
`POST /api/rafale/questions/{id}/reset` ou `/reset-all` (§9), déjà livrés en #197.

### 13.5 Fin de réservoir — rendu attendu

| Situation | `NEXT` | `/anim` |
|---|---|---|
| Pool non vide, plafond non atteint | objet | Zone « SUIVANTE » : énoncé + catégorie + difficulté |
| Pool vide après la question courante | `null` | Zone « SUIVANTE » remplacée par « dernière question du réservoir », en ton `--warning` |
| `RAFALE_ASKED_COUNT + 1 == RAFALE_MAX_QUESTIONS` | `null` | idem |
| `RAFALE_SUBPHASE != "QUESTION"` (ex. `ROUND_END`) | — | Zone entièrement masquée (rien à préparer) |

L'affichage de fin de manche lui-même (`ROUND_END`, `RAFALE_EXHAUSTED`) est **inchangé** — #202
ne touche pas §7.1.

### 13.6 Disposition `/anim` — cadrage pour la maquette

> ⚠️ **`/anim` est bien en `overflow: hidden`.** `AnimPage.css` pose
> `position: fixed; inset: 0; overflow: hidden` sur `.anim-page`, avec une grille
> `grid-template-rows: auto 1fr auto`. La contrainte est donc **la même que la TV** : agrandir un
> texte et ajouter une zone ne peuvent pas produire de scroll — ils doivent tenir, ou être écrêtés
> proprement. Toute taille fixe en `rem` seule est un risque ; l'échelle doit être **relative au
> viewport** et bornée.

Encart `.rafale-anim-qcard` (cellule L3, `flex: 1; min-height: 0`), de haut en bas :

| # | Bloc | Dimensionnement | Part de hauteur visée |
|---|---|---|---|
| 1 | Méta (équipe · catégorie · étoiles · « question N ») | inchangé, `flex: none` | ~10 % |
| 2 | **Énoncé courant** | `clamp()` indexé sur la hauteur du viewport, poids 800, `-webkit-line-clamp: 3`, `flex: 1`, centré verticalement | **~55 %** |
| 3 | Réponse attendue | inchangé de structure, légèrement agrandi | ~15 % |
| 4 | **Zone « SUIVANTE »** | `flex: none`, séparateur haut (filet pointillé), libellé court en capitales, énoncé en ~0.9 rem, `-webkit-line-clamp: 2`, opacité réduite | **≤ 20 %** |

Principes de lisibilité à respecter dans la maquette :
- **Hiérarchie sans ambiguïté** — l'énoncé courant doit rester ~2,5× plus grand que celui de la
  zone « SUIVANTE ». L'animateur ne doit jamais pouvoir confondre les deux d'un coup d'œil.
- **La zone « SUIVANTE » est secondaire** — atténuée (opacité/couleur secondaire), séparée par un
  filet, jamais encadrée comme un second bloc de même poids.
- **Écrêtage propre** — `-webkit-line-clamp` sur les deux énoncés (patron déjà en place
  aujourd'hui sur `.rafale-anim-qcard-text`), jamais de scroll.
- Aucune autre zone de `/anim` n'est modifiée (`AnimRafaleActions`, `RafaleTimers`, bandeau
  contexte, zone équipes, bande régie).

### 13.7 Périmètre

- **`/anim` uniquement.** `/admin` (`GamePage.jsx`, `.rafale-admin-live`) reçoit le champ `NEXT`
  (même canal, même destinataires) mais **ne l'affiche pas** dans ce lot — l'issue #202 ne le
  demande pas. Affichage admin possible ultérieurement sans aucun changement de contrat.
- **RAFALE uniquement.** Aucun autre type de question n'a de notion de « question suivante »
  pendant une manche.
- `/tv` et `/player` : **strictement inchangés**, et c'est un critère bloquant (§13.2).

### 13.8 Non-régression exigée (s'ajoute à §12)

| Zone | Risque | Vérification |
|---|---|---|
| Fuite `NEXT` | La question suivante atteint `/tv` ou `/player` | Extension de `cmd/server/rafale_answer_leak_test.go` — **critère bloquant** |
| Réservoir | Manche terminée sans poser la question sur le pont | Test : réservoir/`rafale_used` identiques avant et après une manche arrêtée en cours |
| `RAFALE_POOL_REMAINING` | Décalage d'une unité | Tests `#107`/`#199` existants au vert, sans modification de leurs valeurs attendues |
| `RAFALE_EXHAUSTED` | Drapeau posé trop tôt | Test : le drapeau reste posé à la **consommation**, pas au pré-tirage |
| Plafond `RAFALE_MAX_QUESTIONS` | Le pré-tirage change l'instant d'arrêt | Test : nombre de questions posées inchangé au plafond |
| Timer / verrou | Le pré-tirage s'exécute dans la section verrouillée d'avance | Suite `internal/game` intégralement au vert, `-race` |
