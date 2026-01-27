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
  PAGE: Page                 // Vue TV actuelle
  QUESTION: Question | null  // Question en cours

  // QCM
  QcmInvalidated: string[]   // Couleurs invalidées ["RED", "YELLOW"]

  // Enrollment (VPlayers)
  enrollmentActive: boolean
  showQRCode: boolean
  virtualPlayerCount: number
  virtualPlayerLimit: number

  // Background
  currentBackgroundIndex: number
  backgrounds: Background[]
}
```

---

## Phases de jeu

```typescript
type Phase =
  | "STOP"      // Arrêté, en attente
  | "PREPARE"   // Question sélectionnée, attente buzzers
  | "READY"     // Tous buzzers prêts
  | "COUNTDOWN" // Décompte Memory
  | "START"     // Timer en cours
  | "PAUSE"     // En pause (joueur a buzzé)
  | "REVEALED"  // Réponse affichée
```

### Transitions autorisées

```
STOP ──(READY)──► PREPARE ──(All PONG)──► READY
                                            │
                                     (START)│
                                            ▼
              ◄──(STOP)── PAUSE ◄──(BUTTON)── START
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

| Champ | Type | Description |
|-------|------|-------------|
| `QcmInvalidated` | string[] | Couleurs éliminées par indices |
| `QCM_HINTS_ENABLED` | boolean | Indices actifs pour cette question |
| `QCM_HINT_THRESHOLD_1` | float | % temps pour 1er indice (défaut: 0.25) |
| `QCM_HINT_THRESHOLD_2` | float | % temps pour 2e indice (défaut: 0.125) |
| `QCM_PENALTY_1` | float | Multiplicateur après 1 indice |
| `QCM_PENALTY_2` | float | Multiplicateur après 2 indices |

---

## Champs Memory spécifiques

```typescript
interface MemoryPair {
  ID: number
  CARD1: MemoryCard
  CARD2: MemoryCard
  matched: boolean
}

interface MemoryCard {
  TEXT?: string
  IMAGE?: string
  IS_IMAGE: boolean
  revealed: boolean
}

interface MemoryConfig {
  FLIP_DELAY: number       // Délai retournement (secondes)
  REVEAL_DELAY: number     // Délai entre révélations
  POINTS_PER_PAIR: number
  ERROR_PENALTY: number
  COMPLETION_BONUS: number
  USE_TIMER: boolean
  MEMORIZE_TIME: number
  SHOW_DURING_MEMORIZE: boolean
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
  "PHASE": "START",
  "DELAY": 30,
  "CURRENT_TIME": 22,
  "PAGE": "GAME",
  "QUESTION": {
    "ID": "5",
    "QUESTION": "Quelle est la capitale de la France ?",
    "ANSWER": "Paris",
    "TYPE": "QCM",
    "POINTS": 10,
    "TIME": 30,
    "CATEGORY": "GEOGRAPHY",
    "STATUS": "STARTED",
    "QCM_ANSWERS": {
      "RED": "Londres",
      "GREEN": "Paris",
      "YELLOW": "Berlin",
      "BLUE": "Madrid"
    },
    "QCM_CORRECT": "GREEN",
    "QCM_HINTS_ENABLED": true
  },
  "QcmInvalidated": ["RED"],
  "enrollmentActive": false,
  "showQRCode": false,
  "virtualPlayerCount": 0,
  "virtualPlayerLimit": 10,
  "currentBackgroundIndex": 0,
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
