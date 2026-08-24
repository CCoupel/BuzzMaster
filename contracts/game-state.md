# GameState

> Structure de l'état du jeu broadcast via WebSocket (action UPDATE)
> **Dernière mise à jour** : 2026-01-27

---

## Structure principale

```typescript
interface GameState {
  PHASE: Phase
  DELAY: number              // Durée totale en secondes
  CURRENT_TIME: number       // Temps restant en secondes
  COUNTDOWN_TIME?: number    // 3, 2, 1 countdown avant STARTED (Memory)
  TIME?: number              // Timestamp serveur (microsecondes)
  REMOTE?: Page              // Vue TV actuelle
  QUESTION: Question | null  // Question en cours

  // QCM
  QCM_INVALIDATED?: string[]   // Couleurs invalidées ["RED", "YELLOW"]

  // Memory
  MEMORY_FLIPPED_CARDS?: string[]  // IDs cartes retournées (max 2)
  MEMORY_MATCHED_PAIRS?: number[]  // IDs paires trouvées
  MEMORY_ERRORS?: number           // Nombre d'erreurs

  // Enrollment (VPlayers)
  ENROLLMENT_ACTIVE: boolean
  SHOW_QR_CODE: boolean
  VIRTUAL_PLAYER_COUNT: number
  VIRTUAL_PLAYER_LIMIT: number

  // Background
  CURRENT_BACKGROUND_INDEX: number
  backgrounds: Background[]
}
```

---

## Phases de jeu

```typescript
type Phase =
  | "STOPPED"   // Arrêté, en attente
  | "PREPARE"   // Question sélectionnée, attente buzzers
  | "READY"     // Tous buzzers prêts
  | "COUNTDOWN" // Décompte Memory
  | "STARTED"   // Timer en cours
  | "PAUSED"    // En pause (joueur a buzzé)
  | "REVEALED"  // Réponse affichée
  | "ENROLL"    // Inscriptions VPlayers ouvertes
```

### Transitions autorisées

```
STOPPED ──(READY)──► PREPARE ──(All PONG)──► READY ──(START)──► COUNTDOWN ──(auto)──► STARTED
                                               │                                         │
                                               └──(START sans COUNTDOWN)─────────────────┘
                                                                                         │
              ◄──(STOP)── PAUSED ◄──(BUTTON)──────────────────────────────────────────┘
              │                               │
              │                        (Timer=0)
              ▼                               │
           REVEALED ◄─────(STOP)──────────────┘
```

---

## Pages TV

```typescript
type Page =
  | "GAME"     // Affichage question/jeu
  | "SCORE"    // Podium équipes
  | "PLAYERS"  // Classement joueurs
  | "PALMARES" // Palmarès par catégorie
```

---

## Question en cours

```typescript
interface CurrentQuestion {
  ID: string
  QUESTION: string
  ANSWER: string
  TYPE: "NORMAL" | "QCM" | "MEMORY"
  POINTS: number
  TIME: number
  CATEGORY?: string
  MEDIA?: string         // URL image question
  MEDIA_ANSWER?: string  // URL image réponse
  STATUS: QuestionStatus

  // QCM
  QCM_ANSWERS?: {
    RED: string
    GREEN: string
    YELLOW: string
    BLUE: string
  }
  QCM_CORRECT?: string
  QCM_HINTS_ENABLED?: boolean

  // Memory
  MEMORY_PAIRS?: MemoryPair[]
  MEMORY_CONFIG?: MemoryConfig
}

type QuestionStatus =
  | "AVAILABLE"  // Pas encore jouée
  | "STARTED"    // En cours
  | "STOPPED"    // Jouée, pas révélée
  | "REVEALED"   // Réponse montrée
```

---

## Champs QCM spécifiques

### Dans GameState

| Champ | Type | Description |
|-------|------|-------------|
| `QCM_INVALIDATED` | string[] | Couleurs éliminées par indices (broadcast temps réel) |

### Dans Question

| Champ | Type | Description |
|-------|------|-------------|
| `QCM_HINTS_ENABLED` | boolean | Indices actifs pour cette question |
| `QCM_HINT_THRESHOLD_1` | float64 | % temps pour 1er indice (défaut: 0.25) |
| `QCM_HINT_THRESHOLD_2` | float64 | % temps pour 2e indice (défaut: 0.125) |
| `QCM_PENALTY_1` | float64 | Multiplicateur après 1 indice (défaut: 0.67) |
| `QCM_PENALTY_2` | float64 | Multiplicateur après 2 indices (défaut: 0.33) |

---

## Champs Memory spécifiques

### Dans GameState

| Champ | Type | Description |
|-------|------|-------------|
| `MEMORY_FLIPPED_CARDS` | string[] | IDs des cartes retournées (max 2) |
| `MEMORY_MATCHED_PAIRS` | number[] | IDs des paires trouvées (permanent) |
| `MEMORY_ERRORS` | number | Nombre d'erreurs (tentatives ratées) |
| `MEMORY_PARTICIPATING_TEAMS` | string[] | Équipes sélectionnées pour cette manche (#172) |
| `MEMOTION_PARTICIPATING_TEAMS` | string[] | Équipes sélectionnées pour cette MEMOTION (#172) |

#### Prérequis de Passage PREPARE → READY (#172)

Depuis la v6.2.0.32, les champs `MEMORY_PARTICIPATING_TEAMS` et `MEMOTION_PARTICIPATING_TEAMS` deviennent des **prérequis normatifs** au passage en phase `READY` — c'est-à-dire que leur conformité est **obligatoire** pour que la transition puisse s'effectuer.

| Type de question | Prérequis | Détail |
|--|--|--|
| MEMORY (SOLO) | `len(MEMORY_PARTICIPATING_TEAMS) === 1` | Exactement 1 équipe sélectionnée |
| MEMORY (CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE) | `len(MEMORY_PARTICIPATING_TEAMS) >= 2` | Au moins 2 équipes sélectionnées |
| MEMOTION | `len(MEMOTION_PARTICIPATING_TEAMS) >= 1` | Au moins 1 équipe sélectionnée |
| SPEEDY, QCM, ARDOISE | Aucun prérequis spécifique | Au moins 1 équipe active (logique générale inchangée) |
| Type inconnu | Permissif (défaut) | Jamais refuser une conformité inconnue |

**Comportement :**
- Réévaluation **continue** — chaque modification de sélection en phase `READY` reteste la conformité
- **Retour arrière possible** : si la conformité cesse d'être vraie en `READY`, retour automatique en `PREPARE`
- **Jamais de régression depuis une phase de jeu** (`STARTED`, `COUNTDOWN`, `PAUSED`, `REVEALED`) — la sélection ne peut plus être modifiée une fois qu'une partie est lancée

### Dans Question

```typescript
interface MemoryPair {
  ID: number
  CARD1: MemoryCard
  CARD2: MemoryCard
}

interface MemoryCard {
  TEXT?: string
  IMAGE?: string
  IS_IMAGE: boolean
}

interface MemoryConfig {
  FLIP_DELAY: number           // Délai retournement si non-match (secondes, défaut: 3)
  REVEAL_DELAY: number         // Délai entre révélations finales (secondes, défaut: 0.5)
  POINTS_PER_PAIR: number      // Points par paire (défaut: 10)
  ERROR_PENALTY: number        // Pénalité par erreur (défaut: 0)
  COMPLETION_BONUS: number     // Bonus si tout trouvé (défaut: 0)
  USE_TIMER: boolean           // Timer global activé (défaut: true)
  MEMORIZE_TIME: number        // Durée mémorisation (secondes, défaut: 5)
  SHOW_DURING_MEMORIZE: boolean // Afficher pendant mémorisation (défaut: true)
}
```

---

## Champs MEMOTION spécifiques — v7.0.0

### Dans GameState

| Champ | Type | Description |
|-------|------|-------------|
| `MEMOTION_SUBPHASE` | string | Phase MEMOTION courante : `GRID` (grille), `SELECTED` (carte choisie), `QUESTION` (énoncé), `REVEAL` (réponse) |
| `MEMOTION_SELECTED` | string | ID de la carte MEMOTION sélectionnée (vide hors SELECTED/QUESTION/REVEAL) |
| `MEMOTION_CARD_STATES` | object | État de chaque carte : `{ "card_1": "UNPLAYED"\|"SELECTED"\|"QUESTION"\|"REVEALED"\|"DONE", ... }` |
| `MEMOTION_CARD_TEAMS` | object | Équipe ayant joué chaque carte : `{ "card_1": "team_A", ... }` |
| `MEMOTION_CURRENT_TEAM` | string | Équipe actuelle (rotation selon MODE) |
| `MEMOTION_PARTICIPATING_TEAMS` | string[] | Équipes sélectionnées pour cette MEMOTION (#172) |
| `MEMOTION_CURRENT_TEAM_COLOR` | number[] | Couleur RGB de l'équipe courante |
| **`MEMOTION_ACTIVE`** | object | **[v7.0.0]** État vivant de la carte active : `{ "CARD_ID": "...", "TYPE": "...", "STATE": {...} }` |

### `MEMOTION_ACTIVE` — Nouvel emplacement actif (v7.0.0)

Champ unique décrivant l'**état vivant du type imbriqué** de la carte MEMOTION en cours. Non persisté.

```typescript
interface MotionActive {
  CARD_ID: string    // ID de la carte active ("" hors SELECTED/QUESTION/REVEAL)
  TYPE: string       // Type de la carte : "SPEEDY", "QCM", "MEMORY", "ARDOISE" (défaut "" = pas de type)
  STATE: object      // État vivant propre au type (ex: { "QCM_INVALIDATED": ["RED", "YELLOW"] } pour QCM)
}
```

**Propriétés clés :**
- **Jamais `omitempty`** — toujours sérialisé, même vide (`{"CARD_ID":"","TYPE":"","STATE":{}}`)
- **Non persisté** — rejoint les champs `Motion*` exclus de `state_persistence.go`
- **Réinitialisé** à chaque `SelectMotionCard` ; vidé au retour en `GRID`, à `MEMORIZE`, `PREPARE`, `READY`
- **Hôte des champs typés vivants** — remplace la lecture directe de `GameState` pour les types imbriqués (cf. `contracts/question-types.md` §5.3)

**Exemple :**
```json
{
  "CARD_ID": "mc-3",
  "TYPE": "QCM",
  "STATE": {
    "QCM_INVALIDATED": ["RED", "YELLOW"]
  }
}
```

---

## Champs Enrollment (VPlayers)

| Champ | Type | Description |
|-------|------|-------------|
| `enrollmentActive` | boolean | Inscriptions ouvertes |
| `showQRCode` | boolean | QR Code affiché sur TV |
| `virtualPlayerCount` | number | Nombre de VPlayers inscrits |
| `virtualPlayerLimit` | number | Limite max |

---

## Champs Background

| Champ | Type | Description |
|-------|------|-------------|
| `currentBackgroundIndex` | number | Index du fond actuel (0-based) |
| `backgrounds` | Background[] | Liste des fonds disponibles |

```typescript
interface Background {
  path: string
  duration: number  // Secondes avant prochain
  opacity: number   // 0-100
}
```

---

## Exemple complet

```json
{
  "PHASE": "STARTED",
  "DELAY": 30,
  "CURRENT_TIME": 22,
  "TIME": 1706380823456789,
  "REMOTE": "GAME",
  "QUESTION": {
    "ID": "5",
    "QUESTION": "Quelle est la capitale de la France ?",
    "ANSWER": "Paris",
    "TYPE": "QCM",
    "POINTS": "10",
    "TIME": "30",
    "CATEGORY": "GEOGRAPHY",
    "STATUS": "STARTED",
    "QCM_ANSWERS": {
      "RED": "Londres",
      "GREEN": "Paris",
      "YELLOW": "Berlin",
      "BLUE": "Madrid"
    },
    "QCM_CORRECT": "GREEN",
    "QCM_HINTS_ENABLED": true,
    "QCM_HINT_THRESHOLD_1": 0.25,
    "QCM_HINT_THRESHOLD_2": 0.125,
    "QCM_PENALTY_1": 0.67,
    "QCM_PENALTY_2": 0.33
  },
  "QCM_INVALIDATED": ["RED"],
  "ENROLLMENT_ACTIVE": false,
  "SHOW_QR_CODE": false,
  "VIRTUAL_PLAYER_COUNT": 0,
  "VIRTUAL_PLAYER_LIMIT": 10,
  "CURRENT_BACKGROUND_INDEX": 0,
  "backgrounds": []
}
```

---

## ARDOISE_ANSWERS (v5.6.0)

Réponses texte libres des équipes pour les questions de type `ARDOISE`.

| Propriété | Type | Valeur par défaut | Notes |
|-----------|------|-------------------|-------|
| ARDOISE_ANSWERS | `{ [teamName: string]: ArdoiseAnswer }` | `{}` | **Jamais null** — toujours sérialisé. ⚠️ **Jamais diffusé au VJoueur** (#128) |

> **Confidentialité (#128, v6.5.2)** : ce champ contient le texte que **chaque équipe** est en train
> de saisir. Il est retiré de toute charge utile destinée à `ClientTypeVPlayer`, sur **toutes** les
> actions — un joueur ne doit pas pouvoir lire les réponses des autres avant le REVEAL depuis les
> outils de développement de sa propre page. La TV (affichage au REVEAL) et l'animateur (liste en
> direct, #158) continuent de le recevoir : il ne peut donc pas rejoindre `AdminOnlyGameFields`,
> d'où la liste `VPlayerOnlyGameFields`. Voir `contracts/vplayer-payload-filter.md` §6, qui
> documente aussi le risque résiduel (`/tv` reste accessible sans authentification).

```typescript
interface ArdoiseAnswer {
  TEXT: string         // Texte saisi par l'équipe
  SUBMITTED_AT: number // Timestamp microsecondes (dernière mise à jour)
}
```

**Règles** :
- Reset à `{}` à chaque nouvelle question (`Ready()`)
- Reset à `{}` sur `InitGame()` (NEW_GAME)
- Mise à jour en temps réel via `ARDOISE_INPUT` (broadcast UPDATE immédiat)
- `SUBMITTED_AT` = timestamp microsecondes de la dernière modification

---

## Utilisation Frontend

### Hook useWebSocket

```javascript
const { gameState, teams, bumpers } = useWebSocket()

// Accès aux données
const phase = gameState.PHASE
const question = gameState.QUESTION
const invalidatedColors = gameState.QcmInvalidated || []
// ARDOISE answers (v5.6.0)
const ardoiseAnswers = gameState.ARDOISE_ANSWERS || {}
```

### Affichage conditionnel par phase

```jsx
{gameState.PHASE === 'STARTED' && (
  <Timer time={gameState.CURRENT_TIME} total={gameState.DELAY} />
)}

{gameState.PHASE === 'REVEALED' && (
  <Answer text={gameState.QUESTION?.ANSWER} />
)}

{/* ARDOISE: afficher réponses équipes */}
{gameState.QUESTION?.TYPE === 'ARDOISE' && (
  <ArdoiseAnswersPanel answers={gameState.ARDOISE_ANSWERS} teams={teams} />
)}
```

---

## Métadonnées Quiz (`QUIZ_*`)

Diffusées dans `GameState`, éditées via `UPDATE_QUIZ_META`. `QUIZ_OBJECTIVES` n'est **jamais**
affiché ni même transmis aux joueurs (cf. « diffusion restreinte » ci-dessous) ; quatre des
autres champs sont affichables sur l'écran TV NEW_GAME **au choix de l'animateur**
(cf. `QUIZ_HIDDEN_FIELDS`).

| Champ | Type | Depuis | Description |
|-------|------|--------|-------------|
| QUIZ_NAME | string | v4.0.0 | Nom du quiz |
| QUIZ_THEME | string | v4.0.0 | Thème général |
| QUIZ_NOTES | string | v4.0.0 | Texte libre (notes, règles, anecdotes) — **affiché aux joueurs** |
| QUIZ_POPULATIONS | string[] | **v6.1.0** | Publics cibles — ⚠️ remplace `QUIZ_POPULATION` (string, v6.0.0) |
| QUIZ_DIFFICULTIES | string[] | **v6.1.0** | Difficultés visées — ⚠️ remplace `QUIZ_DIFFICULTY` (string, v6.0.0) |
| QUIZ_LANGUAGE | string | v6.0.0 | Langue, défaut `Français` — valeur unique, inchangé |
| QUIZ_OBJECTIVES | string | **v6.1.0** | Objectif de la partie, **texte libre** — consigne de génération IA, **jamais affichée aux joueurs** |
| QUIZ_HIDDEN_FIELDS | string[] | **v6.1.0** | Champs que l'animateur choisit de **ne pas** afficher sur l'écran TV NEW_GAME — valeurs ⊂ `THEME`, `POPULATIONS`, `DIFFICULTIES`, `LANGUAGE`. Liste vide = tout est affiché |

> **Aucun `omitempty`** sur ces champs — règle projet : toujours sérialiser, même vide, pour que
> le frontend puisse effacer un champ remis à blanc (`models.go:342-352`). Pour les deux
> tableaux, cela impose de sérialiser `[]` et **jamais `null`** : un `null` casserait tout
> consommateur qui itère sans garde (initialiser à `[]string{}`, pas à un slice nil).

Valeurs autorisées et cardinalités : `contracts/ai-generation.md` §6.

### `QUIZ_OBJECTIVES` — champ à diffusion restreinte (v6.1.0)

**Arbitrage utilisateur** : l'objectif de la partie est une consigne destinée au générateur IA
et à l'animateur (« révision du chapitre 3 », « faire marquer les timides »). L'afficher aux
joueurs viderait certaines parties de leur sens.

| Destinataire | Reçoit `QUIZ_OBJECTIVES` |
|---|---|
| `/ws/admin` | ✅ — nécessaire à l'édition dans la section Quiz |
| `/ws/tv` | ❌ **retiré du payload** |
| `/ws/player` (VPlayer) | ❌ **retiré du payload** |
| `/ws/buzzer` | ❌ (le nœud `GAME` ne leur est déjà pas transmis) |

> **Ne pas se contenter de « ne pas l'afficher » côté React.** La règle porte sur le **payload** :
> le champ ne doit pas quitter le serveur vers TV/VPlayer. Un `GameState` diffusé est lisible
> dans les outils de développement du navigateur, et l'écran TV est par nature visible de tous.

**Point d'implémentation à ne pas manquer** — aujourd'hui, aucun champ du nœud `GAME` n'est
filtré par client : `SerializeForWebClient` ne retire que des champs **de bumper** et la clé
`config` (`internal/protocol/messages.go:560-591`). Il faut donc introduire un filtrage de
champs du nœud `GAME`, sur le modèle de la liste partagée `AdminOnlyBumperFields`
(`messages.go:542`) — par exemple `AdminOnlyGameFields = []string{"QUIZ_OBJECTIVES"}` — et
l'appliquer aux **trois** sites qui produisent un payload TV/VPlayer, sous peine de fuite par le
chemin oublié :

| Site | Rôle |
|---|---|
| `internal/protocol/messages.go:560` (`SerializeForWebClient`) | Référence de correction — couvre aussi VPlayer, `SerializeForVPlayer` y retombant et ne touchant jamais au nœud `GAME` (`messages.go:620-624`) |
| `internal/protocol/messages.go:625` (`SerializeForVPlayer`) | À vérifier : ne doit pas reconstruire un `GAME` complet sur le chemin réduit |
| `cmd/server/main.go:2718` (fan-out chaud) | Réimplémente le filtrage pour éviter un aller-retour `map[string]interface{}` par destinataire — **doit appliquer la même liste** |

Mise à jour requise du tableau 2 de `contracts/ws-payload-serialization.md` (fait dans le même
lot de contrats).

### `QUIZ_HIDDEN_FIELDS` — visibilité TV par champ (v6.1.0, additif)

**Besoin** : selon la partie, l'animateur veut annoncer le thème mais taire la difficulté, ou
n'afficher aucune de ces métadonnées. Le choix est **par champ**, et il porte sur l'écran TV
NEW_GAME uniquement.

**Forme retenue — une liste de champs masqués, pas quatre booléens** :

```go
QuizHiddenFields []string `json:"QUIZ_HIDDEN_FIELDS"`   // ⊂ {THEME, POPULATIONS, DIFFICULTIES, LANGUAGE}
```

| Valeur | Signification |
|---|---|
| `[]` (défaut) | Les quatre champs sont affichés |
| `["DIFFICULTIES"]` | Tout est affiché sauf les difficultés |
| `["THEME","POPULATIONS","DIFFICULTIES","LANGUAGE"]` | Aucune métadonnée affichée (le nom du quiz et le texte libre restent) |

> **Pourquoi une liste et non quatre booléens `QUIZ_DISPLAY_*` par défaut à `true`.** La valeur
> par défaut souhaitée est « affiché ». Avec des booléens, cela impose de forcer `true` à
> **chaque** construction ou réinitialisation de `GameState` — le zéro Go d'un `bool` étant
> `false`, un seul chemin oublié masquerait silencieusement des champs sur la TV, sans erreur
> ni log. Avec une liste, le zéro Go (`nil`/`[]`) **est** le comportement voulu : un chemin
> oublié reste correct. La robustesse prime ici sur la lecture directe du champ, d'autant que
> la logique inversée est absorbée une seule fois côté frontend (case cochée = valeur *absente*
> de la liste).

**Règles** :

| # | Règle |
|---|---|
| H1 | Sérialisé **toujours**, jamais `omitempty`, et **jamais `null`** — initialiser à `[]string{}` (même exigence que les deux autres tableaux). |
| H2 | Valeurs autorisées : `THEME`, `POPULATIONS`, `DIFFICULTIES`, `LANGUAGE`. Toute autre valeur est **ignorée** (et journalisée), jamais une erreur : un client plus récent ne doit pas faire échouer un enregistrement complet sur un seul libellé inconnu. |
| H3 | `OBJECTIVES` n'est **pas** une valeur acceptée. L'objectif n'est jamais diffusé — l'accepter ici laisserait croire qu'il pourrait l'être. |
| H4 | `QUIZ_NAME` et `QUIZ_NOTES` ne sont **pas** pilotables dans cette version : le nom identifie la partie à l'écran, le texte libre est explicitement destiné aux joueurs. Les rendre pilotables plus tard ne coûte qu'une valeur d'énumération de plus — c'est une des raisons du choix de la liste. |
| H5 | Même cycle de vie que les autres `QUIZ_*` : la valeur **persiste** en mémoire d'une partie à l'autre et n'est pas réinitialisée par `NEW_GAME` (vérifié — `internal/game/engine.go`, les métadonnées quiz ne sont affectées que par `SetQuizMeta`). |

### Diffusion — préférence d'affichage ≠ confidentialité

`QUIZ_HIDDEN_FIELDS` est transmis à **tous** les clients, TV comprise : c'est le client TV qui
applique la préférence en n'affichant pas les champs listés. Les valeurs masquées **restent
présentes** dans le payload TV.

C'est volontaire, et la frontière est nette :

| Nature | Mécanisme | Exemple |
|---|---|---|
| **Confidentialité** — la valeur ne doit pas quitter le serveur | Retrait côté serveur, sur les trois chemins de sérialisation | `QUIZ_OBJECTIVES` |
| **Préférence d'affichage** — la valeur n'est pas secrète, l'animateur choisit de ne pas l'annoncer | Application côté client | `QUIZ_HIDDEN_FIELDS` |

> Un thème de quiz masqué n'est pas un secret : il sera de toute façon évident dès la première
> question. Lui appliquer le mécanisme de retrait serveur alourdirait les trois chemins de
> sérialisation pour un gain nul. **Corollaire à retenir** : si un champ réellement sensible
> devait un jour être masquable, il relèverait de la première ligne du tableau, pas de celle-ci.

### Persistance — `game_state.json` (v6.0.x, #141)

**Cette section remplace l'ancienne « Migration — GameState n'est pas persisté (v6.1.0) »,
devenue fausse** : son propre avertissement (« si un mécanisme de persistance du `GameState`
est ajouté entre-temps, cette section devient fausse ») s'est réalisé.

Un sous-ensemble volontairement étroit de `GameState` — les métadonnées quiz et le plafond de
joueurs virtuels — survit désormais à un redémarrage du serveur, via
`data/config/game_state.json` (`internal/game/state_persistence.go` : `SetStatePath`/
`SaveState`/`LoadState`, câblés dans `cmd/server/main.go`, entre `LoadStatuses()` et
`app.start()`).

#### Champs persistés

| Champ `GameState` | Clé JSON dans `game_state.json` |
|---|---|
| `QuizName`, `QuizTheme`, `QuizNotes` | `QUIZ_NAME`, `QUIZ_THEME`, `QUIZ_NOTES` |
| `QuizPopulations`, `QuizDifficulties` | `QUIZ_POPULATIONS`, `QUIZ_DIFFICULTIES` |
| `QuizLanguage`, `QuizObjectives` | `QUIZ_LANGUAGE`, `QUIZ_OBJECTIVES` |
| `QuizHiddenFields` | `QUIZ_HIDDEN_FIELDS` |
| `VirtualPlayerLimit` | `VIRTUAL_PLAYER_LIMIT` |
| `EntracteConfig` (v6.5.2, #119) | `ENTRACTE_CONFIG` |

> `ENTRACTE_CONFIG` rejoint ce sous-ensemble parce que la configuration du panneau de pause est une
> propriété **de la partie**, au même titre que les métadonnées de quiz — pas un réglage du serveur.
> Son ajout est **additif** : un `game_state.json` antérieur qui ne le porte pas se relit sans
> erreur et reçoit les défauts, donc **sans bump de `format_version`** ni branche de migration.
> Le champ `IMAGE_IS_CUSTOM` en est **exclu** : il est dérivé de la présence du fichier sur disque
> et doit être recalculé, jamais figé.

**Explicitement exclu**, avec la raison (voir le commentaire de doc de
`PersistedGameState`, `internal/game/state_persistence.go`, pour le détail complet) :
`Phase`, `Question`, `Memory*`, `Motion*`, `ArdoiseAnswers`, `EnrollmentActive`, `ShowQRCode`,
`Entracte` (état éphémère d'une partie en cours — le restaurer ressusciterait une partie sans
clients connectés ni minuteur vivant ; noter que `Entracte`, l'état, est exclu alors que
`EntracteConfig`, le réglage, est persisté — deux champs voisins, deux cycles de vie opposés) ; `Delay` (minuteur du round courant, réaffecté à chaque
`Start`/`Ready`/décompte MEMOTION — ce n'est pas un réglage, contrairement à
`GameSettings.Game.DefaultDelay`, #150, qui lui est le vrai réglage stocké) ;
`NetworkOnlyLocalhost` (recalculé au démarrage) ; `VirtualPlayerCount` (dérivé du nombre de
buzzers réellement enrôlés) ; `Backgrounds`/`NewGameBackgrounds` (déjà persistés
indépendamment dans `data/files/backgrounds/backgrounds.json`, chargés par `loadBackgrounds()`/
`loadNewGameBackgrounds()` **avant** `LoadState()` dans la séquence de démarrage — les exclure
du sous-ensemble persisté évite tout risque d'écrasement par un ordre de chargement inversé).

#### Enveloppe et versionnement de format

`game_state.json` est le **premier fichier versionné** du projet (aucun des quatre fichiers
préexistants — `history.json`, `teams.json`, `bumpers.json`, `question_statuses.json` — n'a
d'enveloppe ni de champ de version, et il n'existe aucun code de migration dans le dépôt) :

```json
{
  "format_version": 1,
  "QUIZ_NAME": "Mon Quiz",
  "QUIZ_THEME": "Sciences",
  "QUIZ_NOTES": "",
  "QUIZ_POPULATIONS": ["Adulte (18-64 ans)"],
  "QUIZ_DIFFICULTIES": ["Moyen"],
  "QUIZ_LANGUAGE": "Français",
  "QUIZ_OBJECTIVES": "",
  "QUIZ_HIDDEN_FIELDS": ["THEME"],
  "VIRTUAL_PLAYER_LIMIT": 20
}
```

`LoadState` tolère un `format_version` **supérieur** à celui que le build courant connaît
(fichier écrit par une version future du serveur, ex. rollback) : il journalise un avertissement
et charge les champs connus, sans échouer. Il n'existe pas encore de code de migration
**ascendante** (`format_version` inférieur avec un schéma incompatible) — à écrire le jour où
`PersistedGameState` change de forme de façon incompatible.

#### Séquence de chargement au démarrage

Entre `LoadStatuses()` et `app.start()` (`cmd/server/main.go`) — après que
`loadBackgrounds()`/`loadNewGameBackgrounds()` (appelés depuis `init()`) ont déjà rempli
`state.Backgrounds`/`state.NewGameBackgrounds`, d'où leur exclusion du sous-ensemble persisté
ci-dessus. Absence de fichier (première installation) : aucune erreur, les défauts de
`NewEngine()` s'appliquent (`VIRTUAL_PLAYER_LIMIT` = 20, chaînes/tableaux `QUIZ_*` vides).

#### Écriture

Synchrone (pas de goroutine), motif atomique identique à `SaveTeams`/`SaveBumpers`
(`os.CreateTemp` + `Chmod 0644` + `os.Rename` — **pas** le `os.WriteFile` direct de
`SaveHistory`/`SaveStatuses`) — déclenchée par `SetQuizMeta`, `SetQuizDisplay` et
`SetVirtualPlayerLimit` à chaque appel.

#### Règle H5 — inchangée, désormais durable

La règle H5 ci-dessus (« la valeur persiste en mémoire d'une partie à l'autre et n'est pas
réinitialisée par `NEW_GAME` ») reste valable **à l'identique** : `InitGame` ne touche toujours
aucun champ `Quiz*` ni `VirtualPlayerLimit`. #141 ajoute uniquement la survie à un redémarrage du
processus — **aucun changement de comportement observable** en cours de session.

#### Backup / Restore / Reset

`game_state.json` est rattaché au flag `history` de `/backup-select` et `/reset-select` (comme
`game-config.json`, #150 — aucune case dédiée dans l'interface pour ce petit fichier de
métadonnées ; voir `contracts/http-endpoints.md`). `/restore` le détecte par le **contenu** de
l'archive (présence de `config/game_state.json`), indépendamment de tout paramètre, et recharge
l'état moteur immédiatement après extraction.

#### Vie privée / secrets (risque R11 du plan)

Aucun champ persisté n'est sensible : noms/thèmes/notes de quiz, préférences d'affichage,
plafond de joueurs — rien qui ne soit déjà visible à l'écran pendant la partie.

---

## ENTRACTE (v6.5.2, #119)

Mode de **pause globale** déclenché depuis l'admin, indépendant du cycle de question. Deux champs
de `GameState`, tous deux **sans `omitempty`** (règle H1 : toujours sérialisés, jamais `null`).

### ENTRACTE

| Champ | Type | Défaut | Description |
|---|---|---|---|
| `ENTRACTE` | `boolean` | `false` | `true` pendant toute la durée de la pause |

> **Jamais `omitempty`.** Avec `omitempty`, la valeur `false` disparaîtrait du fil et aucun client ne
> pourrait apprendre que l'entracte est **terminé**. Les voisins `backgrounds` /
> `new_game_backgrounds` en portent un — ils précèdent la règle, ce ne sont pas des modèles.

### ENTRACTE_CONFIG

Configuration courante du panneau ENTRACTE. Persistée dans `game_state.json` (propriété de la 
partie, pas réglage global). Poussée dans `GameState` au démarrage et à chaque modification.

⚠️ **`ENTRACTE_CONFIG` est GELÉ pendant un entracte actif** (arbitrage utilisateur 2026-08-20) : 
voir §« Configuration gelée à l'activation » plus bas. Le champ ci-dessous décrit la configuration 
**courante**, pas nécessairement celle affichée au panneau pendant l'entracte.

| Champ | Type | Défaut | Description |
|---|---|---|---|
| `TITLE` | `string` | `"ENTRACTE"` | Texte principal du panneau |
| `SUBTITLE` | `string` | `"Retour dans 20mn"` | Sous-texte |
| `IMAGE_IS_CUSTOM` | `boolean` | `false` | Une image de fond a été téléversée. **Aucun chemin ne circule** : le client construit l'URL stable `/api/game/entracte-image` avec un cache-buster. **Champ dérivé du disque, jamais persisté** — recalculé au chargement et après chaque upload/suppression, pour qu'une image effacée hors application ne laisse pas le panneau en réclamer une inexistante |
| `PANEL_SIZE` | `number` | `65` | Taille du panneau en % de l'écran, appliquée à la largeur **et** à la hauteur. **Réglage unique, identique sur `/tv` et `/player`.** Borné 20–100 |
| `ANIM_PERIOD` | `number` | `10` | Durée d'un cycle d'animation, en **secondes**. Borné 2–30 |
| `ANIM_INTENSITY` | `number` | `20` | Amplitude de l'animation, 0–100. **`0` = animation désactivée** |
| `TRANSITION_MS` | `number` | `2000` | Durée du fondu à l'entrée **et** à la sortie d'entracte, en millisecondes. Borné 0–10000. **`0` = bascule instantanée** |

**Le panneau est centré sur les deux surfaces** (arbitrage utilisateur 2026-08-20) — pas de
positionnement propre au VJoueur. La maquette illustrait des proportions distinctes par surface
(TV `65 × 60`, VJoueur `80 × 38` décentré) ; cette piste est **abandonnée** au profit d'un réglage
unique et d'un centrage uniforme.

> **Conséquence à assumer côté rendu** : un même pourcentage s'applique désormais à un écran 16:9 et
> à un téléphone en portrait. Sur portrait, le panneau sera donc sensiblement plus haut que ce que
> montrait la maquette. Les tailles de texte doivent être **relatives au panneau** (unités viewport
> ou `%`), et le contenu centré en flex, pour rester lisible dans une boîte étroite et haute comme
> dans une boîte large et basse.

### Configuration gelée à l'activation

Deux objets de même forme coexistent, et c'est délibéré :

| Champ | Contenu | Destinataires |
|---|---|---|
| `ENTRACTE_CONFIG` | La configuration **diffusée**, consommée par le panneau. **Gelée** tant qu'un entracte est actif | admin, tv, player, anim |
| `ENTRACTE_CONFIG_SAVED` | La configuration **enregistrée**, toujours à jour. Consommée par le formulaire d'édition | **admin uniquement** (`AdminOnlyGameFields`) |

**La règle tient en une phrase** : `UPDATE_ENTRACTE_CONFIG` écrit toujours la configuration
enregistrée, mais ne rafraîchit la configuration diffusée **que si aucun entracte n'est actif**.
L'activation (`ENTRACTE_SET{ACTIVE:true}`) recopie l'enregistrée vers la diffusée **avant** de lever
le drapeau — sous le même verrou, faute de quoi un client recevrait l'entracte actif accompagné de
l'ancienne configuration.

Conséquence voulue : on peut préparer et enregistrer un nouveau panneau **pendant** une pause en
cours ; il s'appliquera au **prochain** déclenchement, jamais à celui qui est diffusé.

> **Pourquoi deux objets et pas un.** Si le formulaire d'édition se nourrissait du champ gelé, un
> enregistrement fait pendant la pause disparaîtrait de l'écran au retour sur la page : l'animateur
> croirait son enregistrement perdu. Le formulaire lit donc **toujours** `ENTRACTE_CONFIG_SAVED`,
> pendant comme hors entracte — une règle unique plutôt qu'une bascule conditionnelle.

> **Ne pas « simplifier » ce voisinage.** Deux champs du même type dont l'un est un gel de l'autre
> ressemblent à une duplication ; c'en est le contraire.

### Animation du panneau

Le panneau respire : un **zoom** (oscillation d'échelle) et un **balancement** (oscillation de
rotation) combinés en un seul mouvement continu, pilotés par **deux réglages partagés** —
`ANIM_PERIOD` et `ANIM_INTENSITY`. Il n'y a **pas** de réglage par effet, ni de sélecteur d'effet :
même logique de réglage unique que `PANEL_SIZE`.

| Réglage | Sens |
|---|---|
| `ANIM_PERIOD` | Durée d'un aller-retour complet, en secondes. Plus court = plus rapide |
| `ANIM_INTENSITY` | Amplitude commune aux deux effets, 0–100 |

**`ANIM_INTENSITY = 0` désactive l'animation** — pas de champ d'activation séparé. « Désactivée »
signifie que **le rendu ne déclare aucune animation**, pas qu'il en joue une d'amplitude nulle : un
panneau resté 20 minutes à l'écran ne doit pas entretenir une boucle de composition pour rien.

L'amplitude est convertie en valeurs concrètes côté rendu, l'échelle et la rotation ne pouvant pas
partager la même unité. Correspondance retenue, à `ANIM_INTENSITY = 100` : **±6 % d'échelle** et
**±2° de rotation** ; au défaut `20`, **±1,2 %** et **±0,4°** — un mouvement perceptible mais qui ne
capte pas l'attention.

> **Accessibilité** : l'animation doit être neutralisée sous `@media (prefers-reduced-motion: reduce)`,
> quelle que soit la configuration. Un panneau en mouvement continu affiché pendant toute une pause
> est exactement le cas que ce réglage système vise.
>
> **En revanche, le fondu d'entrée/sortie (`TRANSITION_MS`) est conservé** sous ce même réglage, et
> c'est délibéré : `prefers-reduced-motion` vise le **mouvement** — déplacement, échelle, rotation,
> parallaxe — parce qu'il provoque gêne vestibulaire et nausée. Un fondu d'opacité et une dérive
> colorimétrique n'en produisent aucun, et les supprimer rendrait le basculement plus brutal, pas
> plus confortable. Qui veut l'instantané dispose du réglage : `TRANSITION_MS = 0`, pour tous.

> **Contrainte de rendu** : seules des transformations composables (`scale`, `rotate`) sont animées —
> jamais une propriété déclenchant un recalcul de mise en page. Le centrage du panneau ne doit pas
> reposer sur `transform: translate(-50%, -50%)`, qui entrerait en conflit avec la transformation
> animée : le centrage est assuré par le conteneur (flex), la transformation reste au seul service
> de l'animation.

### Transition d'entrée et de sortie

`TRANSITION_MS` pilote **deux effets simultanés**, qui doivent partager la même durée pour rester
solidaires : le fondu du panneau (et du cadenas VJoueur, et de l'indicateur animateur) et
l'apparition/retrait progressif du filtre estompé sur le contenu existant.

> **Contrainte de rendu** : la déclaration `transition` doit vivre sur le **sélecteur de base**, pas
> dans la classe conditionnelle — une transition déclarée uniquement dans la classe joue à l'aller
> puis disparaît avec elle, donc jamais au retour.
>
> **Ne jamais poser un filtre identité** (`grayscale(0) brightness(1)`) sur ce sélecteur de base pour
> « aider » l'interpolation : un `filter` réel, même neutre, crée en permanence un bloc conteneur
> pour les descendants `position: fixed` et un contexte d'empilement — le piège déjà documenté,
> qu'on réintroduirait cette fois **en dehors de tout entracte**. Déclarer `transition: filter` ne
> crée rien ; seule une valeur de `filter` le fait.
>
> Le panneau étant monté conditionnellement, sa **sortie** exige de le maintenir monté pendant le
> fondu, faute de quoi il disparaîtra instantanément quelle que soit la durée configurée.

### Diffusion

`ENTRACTE` et `ENTRACTE_CONFIG` appartiennent au nœud `GAME` et atteignent donc **admin, tv, player
et anim** par le mécanisme de retrait existant (`SerializeForWebClient` / `SerializeForVPlayer` ne
suppriment que `AdminOnlyGameFields`). Ils **n'atteignent pas les buzzers** : `SerializeForBuzzer`
est une liste d'autorisation (`PHASE`, `TIME`, `CURRENT_TIME`) et les LEDs sont pilotées par le
serveur.

`ENTRACTE_CONFIG_SAVED` est en revanche **réservé à l'admin** : il figure dans
`AdminOnlyGameFields`, aux côtés de `QUIZ_OBJECTIVES`. Seule la page Quiz en a l'usage ; la TV et le
VJoueur n'ont que faire d'une configuration qui n'est pas celle du panneau qu'ils affichent.

> Ce choix est délibéré et remplace l'option « faire transiter la config par `CONFIG_UPDATE` » :
> `CONFIG_UPDATE` est restreint à **Admin + TV**, à la diffusion comme au HELLO, et #154 l'a
> explicitement retiré du VJoueur. Or le VJoueur doit afficher le panneau. Via `GameState`, le
> drapeau et sa configuration arrivent **dans le même UPDATE**, y compris pour un client qui se
> connecte pendant l'entracte.

### Phases autorisées

| Phase | Entrée en entracte |
|---|---|
| `STOPPED`, `PREPARE`, `READY`, `NEW_GAME`, `REVEALED` | ✅ Autorisée — aucune question en cours |
| `COUNTDOWN`, `STARTED`, `PAUSED` | ❌ Refusée — ne jamais couper une manche en cours |
| `ENROLL` | ❌ Refusée — l'écran d'inscription affiche le QR code dont les joueurs ont besoin |

La **sortie** d'entracte est autorisée depuis n'importe quelle phase : le mode ne doit jamais être
sans issue.

Entrer en entracte **ne modifie aucun autre champ** de `GameState` : une question sélectionnée en
`PREPARE` est retrouvée intacte à la sortie.

### Persistance

`ENTRACTE` **n'est pas persisté** — absent de `PersistedGameState`, au même titre que `SHOW_QR_CODE`
et pour la même raison (état éphémère). Un serveur qui redémarre repart hors entracte.

`ENTRACTE_CONFIG` **est persisté, dans `game_state.json`** (`PersistedGameState`), aux côtés des
champs `QUIZ_*` : **c'est une propriété de la partie, pas un réglage du serveur** (arbitrage
utilisateur 2026-08-20, après essai en QUALIF). Il est édité depuis la **page Quiz**
(`UPDATE_ENTRACTE_CONFIG`), et non depuis les réglages serveur.

> **Changement par rapport à la première livraison** : la configuration vivait dans la section
> `entracte` de `game-config.json`. Cette section est **supprimée**. Aucune migration n'est fournie —
> elle n'a existé qu'en QUALIF, jamais en production ; une clé résiduelle est simplement ignorée.

Conséquences héritées du cycle de vie des `QUIZ_*` :

| Événement | Effet sur `ENTRACTE_CONFIG` |
|---|---|
| Redémarrage du serveur | **Conservée** |
| `NEW_GAME` (nouvelle partie) | **Conservée** — comme les métadonnées de quiz |
| `POST /reset-select` avec le drapeau `history` | **Effacée** (avec `game_state.json`) |
| Restauration d'archive | Remplacée par celle de l'archive |

⚠️ L'**image**, elle, vit dans `data/files/entracte/` et suit le drapeau `medias`. Réglages et image
relèvent donc de deux drapeaux de sauvegarde différents : les restaurer séparément donne une partie
incomplète. Verrue héritée, de même nature que celle décrite par #152.

Deux champs voisins, deux cycles de vie **opposés** : `ENTRACTE` (état, jamais persisté) et
`ENTRACTE_CONFIG` (réglage, persisté). Ce n'est pas une incohérence, c'est la distinction même entre
les deux — ne pas « corriger » l'un en croyant à un oubli.
