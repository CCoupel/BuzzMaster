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

## Utilisation Frontend

### Hook useWebSocket

```javascript
const { gameState, teams, bumpers } = useWebSocket()

// Accès aux données
const phase = gameState.PHASE
const question = gameState.QUESTION
const invalidatedColors = gameState.QcmInvalidated || []
```

### Affichage conditionnel par phase

```jsx
{gameState.PHASE === 'STARTED' && (
  <Timer time={gameState.CURRENT_TIME} total={gameState.DELAY} />
)}

{gameState.PHASE === 'REVEALED' && (
  <Answer text={gameState.QUESTION?.ANSWER} />
)}
```
