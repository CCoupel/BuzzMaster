# Modèles Partagés

> Structures de données partagées entre backend et frontend
> **Dernière mise à jour** : 2026-01-27

---

## Team (Équipe)

```typescript
interface Team {
  NAME: string          // Nom de l'équipe
  COLOR: string         // Couleur hex "#FF0000"
  SCORE: number         // Score total
  TIME: number          // Timestamp du buzz (microsecondes)
  STATUS: TeamStatus
  BUMPER: string        // ID du bumper gagnant (après buzz)
  TeamPoints: number    // Points équipe (vs points joueurs)
}

type TeamStatus = "READY" | "PAUSE"
```

### Exemple

```json
{
  "Les Rouges": {
    "NAME": "Les Rouges",
    "COLOR": "#ef4444",
    "SCORE": 150,
    "TIME": 0,
    "STATUS": "READY",
    "BUMPER": "",
    "TeamPoints": 50
  }
}
```

### Notes

- `SCORE` = Total des points (joueurs + équipe)
- `TeamPoints` = Points attribués à l'équipe (pas aux joueurs)
- `TIME` > 0 signifie qu'un joueur de l'équipe a buzzé
- `BUMPER` = ID du premier joueur à buzzer

---

## Bumper (Joueur)

```typescript
interface Bumper {
  NAME: string           // Nom du joueur
  TEAM: string           // Nom de l'équipe (vide si non assigné)
  SCORE: number          // Score personnel
  TIME: number           // Timestamp du buzz (microsecondes)
  BUTTON: string         // Bouton pressé ("A", "B", "C", "D")
  STATUS: BumperStatus
  VERSION: string        // Version firmware buzzer
  ANSWER_COLOR: AnswerColor  // Couleur QCM assignée
  HINTS_AT_BUZZ: number  // Indices donnés au moment du buzz
  IS_VIRTUAL: boolean    // true = VPlayer (smartphone)
  READY: string          // "TRUE" si prêt
}

type BumperStatus = "READY" | "PAUSE"

type AnswerColor = "" | "RED" | "GREEN" | "YELLOW" | "BLUE"
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
    "ANSWER_COLOR": "GREEN",
    "HINTS_AT_BUZZ": 1,
    "IS_VIRTUAL": false,
    "READY": "TRUE"
  }
}
```

### Notes

- Clé = Adresse MAC du buzzer (ou ID généré pour VPlayers)
- `TIME` en microsecondes depuis epoch serveur
- `ANSWER_COLOR` = Couleur du bouton pour mode QCM
- `HINTS_AT_BUZZ` = Pour calcul pénalité individuelle
- `IS_VIRTUAL` = Distingue buzzers physiques et VPlayers

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
}

type QuestionType = "NORMAL" | "QCM" | "MEMORY"

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
  Timestamp: number       // Microsecondes
  QuestionID: string
  QuestionText: string
  QuestionCategory: string
  EventType: "POINTS_AWARDED"
  WinnerType: "PLAYER" | "TEAM"
  TeamName: string
  TeamColor: [number, number, number]  // RGB
  PlayerName: string
  PlayerColor: AnswerColor
  Points: number
}
```

### Exemple

```json
{
  "Timestamp": 1706380800000000,
  "QuestionID": "5",
  "QuestionText": "Capitale de la France ?",
  "QuestionCategory": "GEOGRAPHY",
  "EventType": "POINTS_AWARDED",
  "WinnerType": "PLAYER",
  "TeamName": "Les Rouges",
  "TeamColor": [239, 68, 68],
  "PlayerName": "Alice",
  "PlayerColor": "GREEN",
  "Points": 10
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
