# Modèles de Données - BuzzControl

Ce document décrit les structures de données utilisées par le serveur BuzzControl.

## Game State

```json
{
  "PHASE": "STOP|PREPARE|READY|START|PAUSE",
  "DELAY": 30,
  "CURRENT_TIME": 25,
  "QUESTION": {
    "ID": "1",
    "QUESTION": "Question text",
    "ANSWER": "Answer text",
    "POINTS": 10,
    "TIME": 30,
    "MEDIA": "/question/1/media.jpg",
    "MEDIA_ANSWER": "/question/1/answer.jpg",
    "STATUS": "AVAILABLE|STARTED|STOPPED|REVEALED"
  },
  "PAGE": "GAME|SCORES|..."
}
```

## Teams and Bumpers

```json
{
  "teams": {
    "team_id": {
      "NAME": "Team Name",
      "COLOR": "#FF0000",
      "SCORE": 100,
      "TIME": 123456,
      "STATUS": "READY|PAUSE",
      "BUMPER": "winning_bumper_id"
    }
  },
  "bumpers": {
    "bumper_id": {
      "NAME": "Player Name",
      "TEAM": "team_id",
      "SCORE": 50,
      "TIME": 123456,
      "BUTTON": "A",
      "STATUS": "READY|PAUSE",
      "VERSION": "1.0.0",
      "ANSWER_COLOR": "RED|GREEN|YELLOW|BLUE",
      "HINTS_AT_BUZZ": 0,
      "IS_VIRTUAL": false
    }
  }
}
```

## Question (NORMAL type)

```json
{
  "ID": "1",
  "QUESTION": "What is 2+2?",
  "ANSWER": "4",
  "TYPE": "NORMAL",
  "POINTS": 10,
  "TIME": 30,
  "MEDIA": "/question/1/image.jpg",
  "MEDIA_ANSWER": "/question/1/answer.jpg"
}
```

## Question (QCM type)

```json
{
  "ID": "2",
  "QUESTION": "Quelle est la capitale de la France?",
  "ANSWER": "Paris",
  "TYPE": "QCM",
  "QCM_ANSWERS": {
    "RED": "Londres",
    "GREEN": "Paris",
    "YELLOW": "Berlin",
    "BLUE": "Madrid"
  },
  "QCM_CORRECT": "GREEN",
  "QCM_HINTS_ENABLED": true,
  "POINTS": 10,
  "TIME": 30,
  "MEDIA": "/question/2/image.jpg",
  "MEDIA_ANSWER": "/question/2/answer.jpg"
}
```

### QCM Hints & Penalties (v2.38.0)

Système d'indices automatiques pour les questions QCM avec pénalités de points configurables.

**Champs Question :**
- `QCM_HINTS_ENABLED`: boolean (défaut: false) - Active les indices automatiques
- `QCM_HINT_THRESHOLD_1`: float64 (défaut: 0.25) - Seuil du 1er indice (% du temps restant)
- `QCM_HINT_THRESHOLD_2`: float64 (défaut: 0.125) - Seuil du 2ème indice (% du temps restant)
- `QCM_PENALTY_1`: float64 (défaut: 0.67) - Multiplicateur de points après 1 indice (67%)
- `QCM_PENALTY_2`: float64 (défaut: 0.33) - Multiplicateur de points après 2 indices (33%)

**Champs GameState :**
- `QcmInvalidated`: []string - Liste des couleurs invalidées (ex: `["RED", "YELLOW"]`)

**Champs Bumper :**
- `HINTS_AT_BUZZ`: int - Nombre d'indices donnés au moment où le joueur a buzzé (pour pénalité individuelle)

**Logique d'invalidation :**
- Seuil 1 : configurable (défaut 25% du temps restant) → invalide 1 mauvaise réponse aléatoire
- Seuil 2 : configurable (défaut 12.5% du temps restant) → invalide 1 autre mauvaise réponse

**Contraintes de sécurité :**
- Minimum 1s entre les deux indices
- Seuil 2 >= 1s avant la fin du jeu
- Si contraintes non respectables → indices désactivés

**Calcul des seuils (exemple avec défauts) :**
```go
// Exemple avec timer de 30s
seuil1 = 30 * 0.25 = 7.5s  // Indice 1 quand il reste 7.5s
seuil2 = 30 * 0.125 = 3.75s // Indice 2 quand il reste 3.75s

// Vérification des contraintes
if seuil1 - seuil2 < 1 || seuil2 < 1 {
    // Ajuster ou désactiver les indices
}
```

**Pénalités de points (configurables par question) :**
| Réponses restantes | Multiplicateur (défaut) | Exemple (10 pts) |
|--------------------|-------------------------|------------------|
| 4 (aucun indice) | 100% | 10 pts |
| 3 (1 indice) | 67% (configurable) | 6.7 → 7 pts |
| 2 (2 indices) | 33% (configurable) | 3.3 → 3 pts |

**Action WebSocket :**
```json
{
  "ACTION": "QCM_HINT",
  "MSG": {
    "COLOR": "RED",
    "REMAINING": 3
  }
}
```

**Pénalité individuelle par joueur :**
- Le QCM ne met PAS en pause le jeu : tous les joueurs peuvent buzzer
- Chaque joueur reçoit sa propre pénalité basée sur le nombre d'indices **au moment de son buzz**
- `HINTS_AT_BUZZ` stocké dans le bumper lors de `ProcessButtonPress`

## Question (MEMORY type) - v2.33.0

```json
{
  "ID": "10",
  "QUESTION": "Associez les pays à leurs capitales",
  "TYPE": "MEMORY",
  "CATEGORY": "GEOGRAPHY",
  "TIME": 120,
  "POINTS_TARGET": "TEAM",
  "MEMORY_PAIRS": [
    {"ID": 1, "CARD1": {"TEXT": "France", "IS_IMAGE": false}, "CARD2": {"TEXT": "Paris", "IS_IMAGE": false}},
    {"ID": 2, "CARD1": {"IMAGE": "/question/10/memory_2_1_4521.jpg", "IS_IMAGE": true}, "CARD2": {"TEXT": "Berlin", "IS_IMAGE": false}}
  ],
  "MEMORY_CONFIG": {
    "FLIP_DELAY": 3000,
    "POINTS_PER_PAIR": 10,
    "ERROR_PENALTY": 0,
    "COMPLETION_BONUS": 20,
    "USE_TIMER": true
  },
  "ORDER": 10
}
```

**Champs MEMORY :**
- `TYPE`: `"MEMORY"` pour les jeux de paires
- `MEMORY_PAIRS`: Tableau de paires `[{ID, CARD1, CARD2}]`
  - Chaque carte : `TEXT` (string) OU `IMAGE` (chemin), avec `IS_IMAGE` (bool)
- `MEMORY_CONFIG`: Configuration du gameplay (toutes les durées en secondes)
  - `FLIP_DELAY`: Délai avant retournement si non-match (s, défaut: 3)
  - `REVEAL_DELAY`: Délai entre chaque paire révélée en fin de jeu (s, défaut: 0.5)
  - `POINTS_PER_PAIR`: Points par paire trouvée (défaut: 10)
  - `ERROR_PENALTY`: Pénalité par erreur (défaut: 0)
  - `COMPLETION_BONUS`: Bonus si toutes trouvées (défaut: 0)
  - `USE_TIMER`: true = timer global, false = illimité
  - `MEMORIZE_TIME`: Temps de mémorisation affiché (s, défaut: 5)
  - `SHOW_DURING_MEMORIZE`: Afficher les cartes pendant la mémorisation (défaut: true)

**Calcul des points :**
```
Score = (paires_trouvées × POINTS_PER_PAIR) + COMPLETION_BONUS - (erreurs × ERROR_PENALTY)
```

### Phase COUNTDOWN - Cascade Timing (v2.33.0)

Le jeu Memory utilise une phase COUNTDOWN avec animations en cascade synchronisées entre backend et frontend.

| Étape | Description | Durée |
|-------|-------------|-------|
| 1. Cascade reveal | Cartes se retournent une par une (200ms entre chaque) | `(cardCount × 200ms + 600ms)` |
| 2. Décompte visuel | Affichage 5...4...3...2...1 (MEMORIZE_TIME) | `MEMORIZE_TIME` secondes |
| 3. Cascade hide | Cartes se cachent une par une (200ms entre chaque) | `(cardCount × 200ms + 600ms)` |
| 4. Transition | Backend passe en STARTED, jeu commence | - |

**Synchronisation Backend/Frontend :**
```
Backend COUNTDOWN duration = cascade_reveal + MEMORIZE_TIME + cascade_hide
                           = ceil((cardCount × 200 + 600) / 1000) × 2 + MEMORIZE_TIME
```

**Constantes d'animation :**
```javascript
const STAGGER_DELAY = 200      // ms entre chaque carte
const FLIP_ANIMATION = 600     // ms durée animation flip
```

## GameEvent (History)

```go
type GameEvent struct {
  Timestamp        int64   // Server timestamp (microseconds)
  QuestionID       string  // Question ID
  QuestionText     string  // Question text
  QuestionCategory string  // Question category (v2.23.0)
  EventType        string  // "POINTS_AWARDED"
  WinnerType       string  // "PLAYER" or "TEAM"
  TeamName         string  // Team name
  TeamColor        []int   // Team RGB color
  PlayerName       string  // Player name (if PLAYER)
  PlayerColor      string  // Player answer color
  Points           int     // Points awarded
}
```

## Configuration File (config.json)

```json
{
  "server": {
    "http_port": 80,
    "tcp_port": 1234,
    "websocket_path": "/ws",
    "auto_open_browsers": true,
    "debug": false
  },
  "wifi": {
    "ssid": "buzzmaster",
    "password": "BuzzMaster"
  },
  "game": {
    "default_delay": 30
  },
  "storage": {
    "data_dir": "./data",
    "questions_dir": "./data/files/questions",
    "files_dir": "./data/files"
  },
  "neon_effect": {
    "enabled": false,
    "mode": "bar",
    "arc_width": 60,
    "intensity_gap": 80,
    "rotation_speed": 4,
    "bar_offset": 20,
    "bar_thickness": 4,
    "arc_blur": 100,
    "glow_pulse_speed": 2,
    "glow_pulse_min": 30,
    "glow_pulse_max": 50
  },
  "version": "2.47.0"
}
```

### Neon Effect Configuration (v2.46.0)

Système d'effet néon rotatif autour de l'écran TV et VPlayer avec couleur de catégorie.

**Modes d'affichage :**

| Mode | Description | Visuel |
|------|-------------|--------|
| `bar` | Tube lumineux fin avec centre blanc et arc rotatif (défaut) | Tube fixe + arc mobile au centre |
| `halo` | Bordure lumineuse large type néon classique | Conic-gradient rotatif complet |

**Champs neon_effect :**

| Champ | Type | Plage | Défaut | Description |
|-------|------|-------|--------|-------------|
| `enabled` | boolean | - | false | Active/désactive l'effet néon |
| `mode` | string | "bar" / "halo" | "bar" | Type d'effet visuel |
| `arc_width` | int | 30-180 | 60 | Largeur de l'arc lumineux en degrés |
| `intensity_gap` | int | 0-100 | 80 | Écart d'intensité (opacité zone sombre en %) |
| `rotation_speed` | float | 1-10 | 4 | Vitesse de rotation en secondes |
| `bar_offset` | int | 10-100 | 20 | Distance du tube par rapport au bord (px, mode bar) |
| `bar_thickness` | int | 2-20 | 4 | Épaisseur du tube lumineux (px, mode bar) |
| `arc_blur` | int | 0-200 | 100 | Flou de l'arc (% de bar_thickness) |
| `glow_pulse_speed` | float | 0.5-5 | 2 | Vitesse de pulsation du glow (secondes) |
| `glow_pulse_min` | int | 0-100 | 30 | Opacité minimale du glow pulsant (%) |
| `glow_pulse_max` | int | 0-100 | 50 | Opacité maximale du glow pulsant (%) |

## Game Flow

> **Documentation complète**: Voir [GAME_STATE_MACHINE.md](GAME_STATE_MACHINE.md) pour la spécification détaillée.

```
1. STOP (initial)
   └─► readyGame(questionId) ─► PREPARE
                                   │
2. PREPARE (waiting for buzzers)   │
   └─► All buzzers PONG ──────────►│
                                   ▼
3. READY (all buzzers ready)
   └─► startGame(delay) ─► START
                              │
4. START (timer running)      │
   ├─► Button pressed ───────►├─► PAUSE (buzzer paused)
   ├─► Timer = 0 ────────────►│
   └─► pauseAllGame() ───────►│
                              ▼
5. STOP (round ended)
   └─► revealGame() ─► Show answer
```

## Question Status Values

| Statut | Description | Couleur UI |
|--------|-------------|------------|
| `AVAILABLE` | Question not yet played | Vert |
| `STARTED` | Question currently in play | Orange |
| `STOPPED` | Question was played but not revealed | Rouge |
| `REVEALED` | Answer has been shown | Gris |
