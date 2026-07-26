# Modèles Partagés

> Structures de données partagées entre backend et frontend
> **Dernière mise à jour** : 2026-01-27

---

## Team (Équipe)

```typescript
interface Team {
  NAME: string          // Nom de l'équipe
  COLOR: number[]       // Couleur RGB [R, G, B] (ex: [239, 68, 68])
  SCORE: number         // Score total (calculé: TEAM_POINTS + somme joueurs)
  TEAM_POINTS: number   // Points équipe (indépendants des joueurs)
  TIME: number          // Timestamp du buzz (microsecondes, 0 si non buzzé)
  STATUS: TeamStatus
  BUMPER: string        // ID du bumper gagnant (après buzz)
  READY: boolean        // true si l'équipe est prête
}

type TeamStatus = "READY" | "PAUSE"
```

### Exemple

```json
{
  "Les Rouges": {
    "NAME": "Les Rouges",
    "COLOR": [239, 68, 68],
    "SCORE": 150,
    "TEAM_POINTS": 50,
    "TIME": 0,
    "STATUS": "READY",
    "BUMPER": "",
    "READY": true
  }
}
```

### Notes

- `COLOR` = Tableau RGB `[R, G, B]` (pas string hex)
- `SCORE` = Total calculé (TEAM_POINTS + somme des scores joueurs)
- `TEAM_POINTS` = Points attribués uniquement à l'équipe
- `TIME` > 0 signifie qu'un joueur de l'équipe a buzzé
- `BUMPER` = ID du premier joueur à buzzer
- `READY` = true si l'équipe est prête (tous buzzers connectés)

---

## Bumper (Joueur)

```typescript
interface Bumper {
  NAME: string           // Nom du joueur
  TEAM: string           // Nom de l'équipe (vide si non assigné)
  SCORE: number          // Score personnel
  TIME: number           // Timestamp du buzz (microsecondes, 0 si non buzzé)
  BUTTON: string         // Bouton pressé ("A", "B", "C", "D")
  STATUS: BumperStatus
  VERSION: string        // Version firmware buzzer
  IP: string             // Adresse IP du buzzer
  READY: boolean         // true si prêt (PONG reçu)
  ANSWER_COLOR: AnswerColor  // Couleur QCM assignée
  HINTS_AT_BUZZ: number  // Indices donnés au moment du buzz (QCM)
  IS_VIRTUAL: boolean    // true = VPlayer (smartphone)
  CONNECTED: boolean     // true si le buzzer/VJoueur est actuellement connecté (WebSocket)
  CONN_STATE: ConnState  // Badge de connexion 4 états — voir notes (v5.7.13, #109)
}

type BumperStatus = "READY" | "PAUSE"

type AnswerColor = "" | "RED" | "GREEN" | "YELLOW" | "BLUE"

// Badge de connexion (v5.7.13, #109) — voir contracts/websocket-actions.md
// pour la table de transitions complète (backend: engine.TransitionConn).
type ConnState = "" | "orange" | "red" | "green"
```

### Exemple

```json
{
  "AA:BB:CC:DD:EE:FF": {
    "NAME": "Alice",
    "TEAM": "Les Rouges",
    "SCORE": 30,
    "TIME": 1706380823456789,
    "BUTTON": "A",
    "STATUS": "PAUSE",
    "VERSION": "1.2.0",
    "IP": "192.168.4.10",
    "READY": true,
    "ANSWER_COLOR": "GREEN",
    "HINTS_AT_BUZZ": 1,
    "IS_VIRTUAL": false
  }
}
```

### Notes

- Clé = Adresse MAC du buzzer (ou ID généré pour VPlayers)
- `TIME` en microsecondes depuis epoch serveur (0 si non buzzé)
- `IP` = Adresse IP du buzzer sur le réseau
- `READY` = true après réception du PONG
- `CONNECTED` = statut socket brut (WebSocket up/down), toujours sérialisé (pas de `omitempty`)
- `CONN_STATE` = badge de connexion enrichi (v5.7.13, #109) : `""` (HIDDEN, rien à afficher),
  `"orange"` (déconnecté), `"red"` (déconnecté + message perdu), `"green"` (reconnecté, fenêtre
  min. 2s — timer géré en Phase 2). Toujours sérialisé (pas de `omitempty`). **Périmètre : seuls
  les bumpers participants (`TEAM != ""`) portent un état visible** — un bumper non assigné reste
  toujours à `""`. Piloté côté serveur par `engine.TransitionConn(bumperID, event)` avec
  `event ∈ {DISCONNECT, RECONNECT, MESSAGE_LOST, DELIVERY_CONFIRMED}`.
- `ANSWER_COLOR` = Couleur du bouton pour mode QCM
- `HINTS_AT_BUZZ` = Nombre d'indices donnés au moment du buzz (pénalité individuelle)
- `IS_VIRTUAL` = Distingue buzzers physiques et VPlayers smartphone

---

## Question

```typescript
interface Question {
  ID: string
  QUESTION: string       // Texte de la question
  ANSWER: string         // Réponse correcte
  TYPE: QuestionType
  POINTS: number
  TIME: number           // Durée en secondes
  CATEGORY?: string
  POINTS_TARGET: PointsTarget
  ORDER: number          // Position dans la liste
  STATUS: QuestionStatus
  MEDIA?: string         // URL image question
  MEDIA_ANSWER?: string  // URL image réponse

  // QCM
  QCM_ANSWERS?: QCMAnswers
  QCM_CORRECT?: AnswerColor
  QCM_HINTS_ENABLED?: boolean
  QCM_HINT_THRESHOLD_1?: number
  QCM_HINT_THRESHOLD_2?: number
  QCM_PENALTY_1?: number
  QCM_PENALTY_2?: number

  // Memory
  MEMORY_PAIRS?: MemoryPair[]
  MEMORY_CONFIG?: MemoryConfig

  // MEMOTION (v5.0.0+)
  MEMOTION_CARDS?: MotionCard[]
  MEMOTION_MODE?: "SOLO" | "CHACUN_SON_TOUR" | "TANT_QUE_JE_GAGNE"
  MOTION_MEMORIZE_DURATION?: number  // Seconds for MEMORIZE phase in Secret Mode. 0 = standard mode (no memorization). (v5.5.0)
}

type QuestionType = "NORMAL" | "QCM" | "MEMORY" | "MEMOTION"

// Subphases du jeu MEMOTION — MEMORIZE ajouté en v5.5.0 (Secret Mode)
type MotionSubPhase = "" | "MEMORIZE" | "GRID" | "SELECTED" | "QUESTION" | "REVEAL"

type PointsTarget = "PLAYER" | "TEAM"

type QuestionStatus = "AVAILABLE" | "STARTED" | "STOPPED" | "REVEALED"

interface QCMAnswers {
  RED: string
  GREEN: string
  YELLOW: string
  BLUE: string
}
```

### Exemple NORMAL

```json
{
  "ID": "1",
  "QUESTION": "Quel est le plus grand océan ?",
  "ANSWER": "Pacifique",
  "TYPE": "NORMAL",
  "POINTS": 10,
  "TIME": 30,
  "CATEGORY": "GEOGRAPHY",
  "POINTS_TARGET": "PLAYER",
  "ORDER": 1,
  "STATUS": "AVAILABLE",
  "MEDIA": "/question/1/media_4521.jpg"
}
```

### Exemple QCM

```json
{
  "ID": "2",
  "QUESTION": "Capitale de la France ?",
  "ANSWER": "Paris",
  "TYPE": "QCM",
  "POINTS": 10,
  "TIME": 30,
  "POINTS_TARGET": "TEAM",
  "QCM_ANSWERS": {
    "RED": "Londres",
    "GREEN": "Paris",
    "YELLOW": "Berlin",
    "BLUE": "Madrid"
  },
  "QCM_CORRECT": "GREEN",
  "QCM_HINTS_ENABLED": true,
  "QCM_HINT_THRESHOLD_1": 0.25,
  "QCM_HINT_THRESHOLD_2": 0.125
}
```

### Exemple MEMORY

```json
{
  "ID": "3",
  "QUESTION": "Associez pays et capitales",
  "TYPE": "MEMORY",
  "TIME": 120,
  "POINTS_TARGET": "TEAM",
  "MEMORY_PAIRS": [
    {
      "ID": 1,
      "CARD1": {"TEXT": "France", "IS_IMAGE": false},
      "CARD2": {"TEXT": "Paris", "IS_IMAGE": false}
    }
  ],
  "MEMORY_CONFIG": {
    "FLIP_DELAY": 3,
    "POINTS_PER_PAIR": 10,
    "ERROR_PENALTY": 0,
    "COMPLETION_BONUS": 20
  }
}
```

---

## GameEvent (Historique)

```typescript
interface GameEvent {
  TIMESTAMP: number               // Microsecondes (epoch serveur)
  QUESTION_ID: string
  QUESTION_TEXT: string
  QUESTION_CATEGORY: string
  EVENT_TYPE: "POINTS_AWARDED"
  WINNER_TYPE: "PLAYER" | "TEAM"
  TEAM_NAME: string
  TEAM_COLOR: [number, number, number]  // RGB
  PLAYER_NAME: string             // Vide si TEAM
  PLAYER_COLOR: AnswerColor       // Couleur réponse (si PLAYER)
  POINTS: number
  REACTION_TIME: number           // Temps de réaction en microsecondes
}
```

### Exemple

```json
{
  "TIMESTAMP": 1706380800000000,
  "QUESTION_ID": "5",
  "QUESTION_TEXT": "Capitale de la France ?",
  "QUESTION_CATEGORY": "GEOGRAPHY",
  "EVENT_TYPE": "POINTS_AWARDED",
  "WINNER_TYPE": "PLAYER",
  "TEAM_NAME": "Les Rouges",
  "TEAM_COLOR": [239, 68, 68],
  "PLAYER_NAME": "Alice",
  "PLAYER_COLOR": "GREEN",
  "POINTS": 10,
  "REACTION_TIME": 1234567
}
```

---

## Background (Fond d'écran)

```typescript
interface Background {
  path: string      // Chemin relatif "/backgrounds/bg1.jpg"
  duration: number  // Durée affichage en secondes
  opacity: number   // Opacité 0-100
}
```

---

## Catégories

Les catégories sont des chaînes libres. Catégories suggérées :

| Catégorie | Icône |
|-----------|-------|
| GEOGRAPHY | 🌍 |
| HISTORY | 📜 |
| SCIENCE | 🔬 |
| ENTERTAINMENT | 🎬 |
| SPORTS | ⚽ |
| MUSIC | 🎵 |
| ART | 🎨 |
| LITERATURE | 📚 |
| FOOD | 🍕 |
| NATURE | 🌿 |

---

## Couleurs QCM

| Couleur | Code | Lettre | Hex |
|---------|------|--------|-----|
| RED | `"RED"` | A | #ef4444 |
| GREEN | `"GREEN"` | B | #22c55e |
| YELLOW | `"YELLOW"` | C | #eab308 |
| BLUE | `"BLUE"` | D | #3b82f6 |

---

## ARDOISE — Types et Structures (v5.6.0)

### KeyboardType

```typescript
type KeyboardType = "AZERTY" | "NUMPAD"
```

### ArdoiseAnswer

```typescript
interface ArdoiseAnswer {
  TEXT: string         // Texte saisi par l'équipe (full content, pas delta)
  SUBMITTED_AT: number // Timestamp en microsecondes (dernière mise à jour)
}
```

### Question.ARDOISE_KEYBOARD_TYPE

Champ optionnel dans `Question` pour les questions de type `ARDOISE` :

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| ARDOISE_KEYBOARD_TYPE | KeyboardType | ❌ | Layout clavier : `"AZERTY"` (défaut) ou `"NUMPAD"` |

### QuestionType

Valeurs valides pour `TYPE` :

| Type | Description |
|------|-------------|
| `"NORMAL"` | Question standard |
| `"QCM"` | Choix multiple 4 couleurs |
| `"MEMORY"` | Jeu de mémoire |
| `"MEMOTION"` | Grille de cartes animées |
| `"ARDOISE"` | ✨ Saisie libre via clavier virtuel (v5.6.0) |
