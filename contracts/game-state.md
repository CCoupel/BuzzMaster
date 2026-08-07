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
| ARDOISE_ANSWERS | `{ [teamName: string]: ArdoiseAnswer }` | `{}` | **Jamais null** — toujours sérialisé |

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

### Migration — `GameState` n'est pas persisté (v6.1.0)

Le passage de `QUIZ_POPULATION`/`QUIZ_DIFFICULTY` (string) à `QUIZ_POPULATIONS`/`QUIZ_DIFFICULTIES`
(string[]) est un **changement de type incompatible**. La question posée avant rédaction de ce
contrat était : un fichier de partie écrit avant le déploiement casse-t-il le rechargement au
démarrage (`string` → `[]string` = `json.UnmarshalTypeError`) ?

**Réponse — vérifiée dans le code : le risque n'existe pas.** `GameState` **n'est jamais écrit
sur disque ni relu**. Les seuls chemins de persistance déclarés sont
(`cmd/server/main.go:205-211`) :

| Fichier persisté | Contenu | Contient des `QUIZ_*` ? |
|---|---|---|
| `config/history.json` | Historique des parties | ❌ |
| `config/teams.json` | Équipes | ❌ |
| `config/bumpers.json` | Buzzers | ❌ |
| `config/question_statuses.json` | Statuts des questions | ❌ |

Il n'existe ni `SetStatePath`, ni `SaveState`, ni `LoadState` (`internal/game/engine.go`). Les
métadonnées quiz vivent **en mémoire** et sont perdues à chaque redémarrage du serveur — un
comportement préexistant, indépendant de ce changement. Les sauvegardes TAR
(`/fs-backup`, `/game-backup`, `/backup-select`, `internal/server/http.go:268-272`) archivent des
**répertoires de données** (questions, équipes, buzzers, historique, médias) : aucune ne contient
de `GameState` sérialisé.

**Conséquence contractuelle** : **aucun script de migration, aucun numéro de version de format,
aucun reset de partie n'est requis.** Toute proposition de migration de fichier sur ce chantier
serait du code mort.

**Les deux risques résiduels — réels, mais côté client** :

| # | Risque | Traitement imposé |
|---|---|---|
| R1 | Un onglet admin/TV resté **ouvert** pendant le déploiement exécute le JS de v6.0.0 : il lit `QUIZ_POPULATION` (absent) et affiche du vide ; s'il enregistre, il poste `POPULATION` (ignoré, cf. `ai-generation.md` §7) | Rechargement des interfaces après déploiement — mode opératoire habituel de montée de version. **Pas de code de compatibilité** : un client non redéployé doit être visible. |
| R2 | Le client reçoit un jour un type inattendu (serveur partiellement déployé, cache) et itère sur une chaîne au lieu d'un tableau | Normalisation défensive **à la lecture** du `GameState` (`web/src/hooks/useWebSocket.js`) : une valeur absente ou non-tableau devient `[]`. Trois lignes, elles évitent un écran blanc sur l'affichage TV. |

> ⚠️ **À valider par `dev-backend` au moment de l'implémentation** : ce constat de non-persistance
> a été établi par recherche exhaustive des chemins de sauvegarde à la date du 2026-08-06. Si un
> mécanisme de persistance du `GameState` est ajouté **entre-temps** par un autre chantier, cette
> section devient fausse et une migration redevient nécessaire.
