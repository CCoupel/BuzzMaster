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

## Team (Équipe) — v5.7.25, #113

```typescript
interface Team {
  NAME: string                    // Nom de l'équipe
  COLOR: number[]                 // Couleur RGB [R, G, B] (ex: [255, 26, 26])
  COLOR_NAME?: string             // Clé palette (#113, ex: "rouge", "bleu-profond") — voir Palette d'équipes
  SCORE: number                   // Score total (calculé: TEAM_POINTS + somme joueurs)
  TEAM_POINTS: number             // Points équipe (indépendants des joueurs)
  TIME: number                    // Timestamp du buzz (microsecondes, 0 si non buzzé)
  STATUS: "READY" | "PAUSE"       // État de l'équipe
  BUMPER: string                  // ID du bumper gagnant (après buzz)
  READY: boolean                  // true si l'équipe est prête
}
```

**Notes** :
- `COLOR` = Tableau RGB `[R, G, B]` (entiers 0-255), jamais en hex
- `COLOR_NAME` = Clé palette (#113 v5.7.25). **Écrit par le frontend** à chaque sélection ou attribution automatique. Permet résolution LED exacte (v5.7.25+). Absent sur équipes antérieures → fallback par teinte. Optionnel dans contrat, **toujours renseigné** par frontend courant.
- `SCORE` = Total calculé (TEAM_POINTS + somme des scores joueurs)
- `TEAM_POINTS` = Points attribués uniquement à l'équipe
- `TIME` > 0 signifie qu'un joueur de l'équipe a buzzé
- `BUMPER` = ID du premier joueur à buzzer
- `READY` = true si l'équipe est prête (tous buzzers connectés)

## Palette d'équipes — v5.7.25, #113

16 couleurs = 8 teintes × 2 tons. Ton vif `S=100% L=55%`, ton profond `S=100% L=35%`.

| Rang | Clé | Nom | RGB | Teinte |
|------|-----|-----|-----|--------|
| 1 | `rouge` | Rouge | `[255, 26, 26]` | 0° |
| 2 | `orange` | Orange | `[255, 133, 26]` | 28° |
| 3 | `jaune` | Jaune | `[255, 217, 26]` | 50° |
| 4 | `vert` | Vert | `[26, 255, 83]` | 135° |
| 5 | `cyan` | Cyan | `[26, 236, 255]` | 185° |
| 6 | `bleu` | Bleu | `[26, 94, 255]` | 222° |
| 7 | `violet` | Violet | `[159, 26, 255]` | 275° |
| 8 | `rose` | Rose | `[255, 26, 159]` | 325° |
| 9 | `rouge-profond` | Grenat | `[179, 0, 0]` | 0° |
| 10 | `orange-profond` | Ambre | `[179, 83, 0]` | 28° |
| 11 | `jaune-profond` | Or | `[179, 149, 0]` | 50° |
| 12 | `vert-profond` | Émeraude | `[0, 179, 45]` | 135° |
| 13 | `cyan-profond` | Turquoise | `[0, 164, 179]` | 185° |
| 14 | `bleu-profond` | Marine | `[0, 54, 179]` | 222° |
| 15 | `violet-profond` | Indigo | `[104, 0, 179]` | 275° |
| 16 | `rose-profond` | Magenta | `[179, 0, 104]` | 325° |

**Ordre d'attribution** : rangs 1→16 (8 tons vifs puis 8 tons profonds). Au-delà de 16 équipes, recycle au rang 1.

**Invariant d'affichage** : ces 16 valeurs sont invariantes par `boostTeamColor()` (saturation déjà 100%, luminosité déjà dans [35%, 65%]). RGB stocké = RGB affiché.

**Sources de vérité** :
- Frontend : `web/src/constants/colors.js` (`TEAM_COLORS`)
- Backend : `cmd/server/main.go` (`teamColorPalette`)
- Contrat : `contracts/models.md` — **les deux sources doivent rester strictement identiques**

## Teams and Bumpers

```json
{
  "teams": {
    "team_id": {
      "NAME": "Team Name",
      "COLOR": [255, 26, 26],
      "COLOR_NAME": "rouge",
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
      "IS_VIRTUAL": false,
      "CONNECTED": true,
      "CONN_STATE": "green"
    }
  }
}
```

### Badge de Connexion (CONN_STATE) - v5.7.13, #109

Champ **CONN_STATE** : état enrichi du badge de connexion (4 niveaux), distinct du champ brut `CONNECTED`.

| État | Valeur | Affichage | Déclencheur | Durée |
|------|--------|-----------|-------------|-------|
| **HIDDEN** | `""` (vide) | Aucune icône | Déconnexion confirmée + ACK reçu, ou bumper non participant | Permanent |
| **ORANGE** | `"orange"` | ⚠ Orange | Déconnexion WebSocket détectée | Jusqu'à reconnexion |
| **RED** | `"red"` | ⚠ Rouge | Déconnexion + message perdu (LED_SET, OTA, WIFI_CONFIG sans ACK) | Jusqu'à reconnexion |
| **GREEN** | `"green"` | ✅ Vert | Reconnexion établie, ACK reçu | 2s minimum, puis caché après `ConfirmDelivery` |

**Règles critiques** :
- **Visible uniquement pour participants** : `TEAM != ""` (bumpers non assignés restent toujours `""`)
- **Toujours sérialisé** : pas de `omitempty` (évite pertes d'état côté frontend)
- **Partagé** : même machine d'état `transitionConnUnsafe` pour buzzers physiques ET VJoueurs
- **Transitions** :
  - `Hidden|Orange|Red + Disconnect → Orange`
  - `Orange|Red + MessageLost → Red`
  - `Orange|Red + Reconnect → Green` (timer 2s minimum)
  - `Green + ConfirmDelivery (même timestamp) → Hidden`
  - `Green + Disconnect → Orange`

**Backend** : Piloté par `engine.TransitionConn(bumperID, event)` avec événements :
- `ConnEventDisconnect` : perte WebSocket
- `ConnEventMessageLost` : message perdu (LED_SET/OTA/WIFI sans ACK)
- `ConnEventReconnect` : reconnexion établie
- `ConnEventDeliveryConfirmed` : ACK reçu

**Frontend** : `ConnectionBadge.jsx` affiche l'icône et la couleur selon `bumper.CONN_STATE`, filtré sur `bumper.TEAM != ""`.

### Identité par ID (PlayerConnectPayload.ID) - v5.7.20, #109 R1

Nouveau champ optionnel dans le payload de reconnexion WebSocket VJoueur.

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| `ID` | string | ❌ | Identifiant unique stocké (UUID ou hash) pour identifier un VJoueur lors de reconnexion |

**Comportement** :
- **Reconnexion par ID** : si le `ID` est reconnu et le bumper existe, reutilise le bumper (nom rafraîchi si nécessaire), `Connected=true`, `ConnEventReconnect`.
- **Nom déjà pris** : si le nom est assigné à un autre bumper connecté ou déconnecté, rejette avec `PLAYER_REJECTED` (raison: `NAME_TAKEN`).
- **ID périmé** : si l'`ID` n'est pas résolu, fallback sur identification par nom — peut également rejeter si nom en conflit.

**Avantages** : Élimine tout risque de fusion/perte de données sur collision de nom (ex: deux VJoueurs "Alice" qui se reconnaissent par le même ID perdu d'une session antérieure).

**Implémentation côté frontend** : `EnrollPage.jsx` stocke l'`ID` en localStorage, `VPlayerPage.jsx` l'envoie dans `PlayerConnectPayload`. `VPlayerPage.jsx` affiche un écran bloquant `PLAYER_REJECTED` avec redirection auto 3s si le serveur rejette.

## Question (SPEEDY type)

```json
{
  "ID": "1",
  "QUESTION": "What is 2+2?",
  "ANSWER": "4",
  "TYPE": "SPEEDY",
  "POINTS": 10,
  "TIME": 30,
  "MEDIA": "/question/1/image.jpg",
  "MEDIA_ANSWER": "/question/1/answer.jpg"
}
```

> **Note**: Type `NORMAL` est renommé `SPEEDY` depuis v5.7.1. Les fichiers existants avec `"TYPE": "NORMAL"` sont convertis automatiquement à la lecture.

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

## Question (MEMORY type) - v2.33.0, Multi-Teams v2.51.0

```json
{
  "ID": "10",
  "QUESTION": "Associez les pays à leurs capitales",
  "TYPE": "MEMORY",
  "CATEGORY": "GEOGRAPHY",
  "TIME": 120,
  "POINTS_TARGET": "TEAM",
  "MEMORY_MODE": "CHACUN_SON_TOUR",
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
- `MEMORY_MODE`: Mode de jeu multi-équipes (v2.51.0, optionnel, défaut: "SOLO")
  - `"SOLO"`: Une seule équipe joue (comportement par défaut, rétrocompatible)
  - `"CHACUN_SON_TOUR"`: Rotation stricte après chaque tentative (2 cartes)
  - `"TANT_QUE_JE_GAGNE"`: L'équipe garde la main tant qu'elle trouve des paires
- `MEMORY_PAIRS`: Tableau de paires `[{ID, CARD1, CARD2}]`
  - Chaque carte : `TEXT` (string) OU `IMAGE` (chemin), avec `IS_IMAGE` (bool)
- `MEMORY_CONFIG`: Configuration du gameplay (toutes les durées en secondes)
  - `FLIP_DELAY`: Délai avant retournement si non-match (s, défaut: 3)
  - `REVEAL_DELAY`: Délai entre chaque paire révélée en fin de jeu (s, défaut: 0.5)
  - `POINTS_PER_PAIR`: Points par paire trouvée (défaut: 10)
  - `ERROR_PENALTY`: Pénalité par erreur (défaut: 0)
  - `COMPLETION_BONUS`: Bonus si toutes trouvées (défaut: 0, attribué à l'équipe qui trouve la dernière paire)
  - `USE_TIMER`: true = timer global, false = illimité
  - `MEMORIZE_TIME`: Temps de mémorisation affiché (s, défaut: 5)
  - `SHOW_DURING_MEMORIZE`: Afficher les cartes pendant la mémorisation (défaut: true)

**Calcul des points :**

Mode SOLO (v2.33.0):
```
Score = (paires_trouvées × POINTS_PER_PAIR) + COMPLETION_BONUS - (erreurs × ERROR_PENALTY)
```

Modes multi-équipes (v2.51.0):
```
Score_équipe = (paires_trouvées_par_équipe × POINTS_PER_PAIR)
COMPLETION_BONUS → attribué à l'équipe qui trouve la dernière paire
ERROR_PENALTY → global (non par équipe)
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

### GameState Memory (v2.51.0)

Champs ajoutés au GameState pour les modes multi-équipes:

```json
{
  "MEMORY_CURRENT_TEAM": "Équipe Bleue",
  "MEMORY_TEAM_PAIRS": {
    "Équipe Rouge": 2,
    "Équipe Bleue": 1,
    "Équipe Verte": 3
  },
  "MEMORY_PARTICIPATING_TEAMS": ["Équipe Rouge", "Équipe Bleue", "Équipe Verte"]
}
```

**Champs :**
- `MEMORY_CURRENT_TEAM`: Nom de l'équipe qui joue actuellement (vide en mode SOLO)
- `MEMORY_TEAM_PAIRS`: Map du nombre de paires trouvées par équipe
- `MEMORY_PARTICIPATING_TEAMS`: Liste ordonnée des équipes sélectionnées pour la partie

**Initialisation :**
- Les champs sont vides/nil en mode SOLO ou si aucune équipe n'est sélectionnée
- `MEMORY_SET_TEAMS` action WebSocket permet de définir les équipes avant le START
- Reset automatique lors du changement de question (phase PREPARE)

## Question (ARDOISE type) - v5.6.0

```json
{
  "ID": "q-ardoise-1",
  "QUESTION": "Quel est la capitale de la Belgique?",
  "ANSWER": "Bruxelles",
  "TYPE": "ARDOISE",
  "ARDOISE_KEYBOARD_TYPE": "AZERTY",
  "POINTS": 10,
  "TIME": 60,
  "MEDIA": "/question/q-ardoise-1/image.jpg",
  "ORDER": 1
}
```

**Champs ARDOISE :**
- `TYPE`: `"ARDOISE"` pour les questions réponse libre au clavier
- `ARDOISE_KEYBOARD_TYPE`: Type de clavier virtuel — `"AZERTY"` ou `"NUMPAD"` (défaut: "AZERTY")
- `ANSWER`: Réponse attendue (affichée en REVEALED, permet la correction manuelle)
- `TIME`: Durée du timer (secondes)
- `MEDIA`: Image/vidéo de la question (optionnel)

### GameState ARDOISE (v5.6.0)

Champs ajoutés au GameState pour les questions ARDOISE :

```json
{
  "ARDOISE_ANSWERS": {
    "Équipe Bleue": {
      "TEXT": "Bruxelles",
      "SUBMITTED_AT": 1716394800123456
    },
    "Équipe Rouge": {
      "TEXT": "Amsterdam",
      "SUBMITTED_AT": 1716394801000000
    }
  }
}
```

**Champs :**
- `ARDOISE_ANSWERS`: Dictionnaire `map[string]ArdoiseAnswer`
  - Clé : nom de l'équipe
  - Valeur : `ArdoiseAnswer` struct

**Structure ArdoiseAnswer :**
```go
type ArdoiseAnswer struct {
    TEXT        string // Réponse saisie par l'équipe
    SUBMITTED_AT int64  // Timestamp microseconde (µs) du dernier envoi
}
```

**Sérialisation :**
- **Jamais `omitempty`** — le champ est toujours présent dans le GameState JSON (même si vide)
- Évite les réinitialisations manquées côté frontend

**Initialisation :**
- Reset à `{}` (dictionnaire vide) lors de la transition vers `READY` (nouvelle question)
- Rempli progressivement au fur et à mesure des actions `ARDOISE_INPUT` reçues du serveur

**Action WebSocket associée :**
```json
{
  "ACTION": "ARDOISE_INPUT",
  "MSG": {
    "TEXT": "Bruxelles"
  }
}
```

Voir `WEBSOCKET_PROTOCOL.md` pour comportement complet (throttling 200ms, flush STOP/PAUSE).

---

## GameState — Champs réseau (v5.7.0)

### NETWORK_ONLY_LOCALHOST

Indique que le serveur n'est accessible que depuis `localhost` (aucune interface réseau externe active).

```json
{
  "NETWORK_ONLY_LOCALHOST": true
}
```

**Champ :**
- `NETWORK_ONLY_LOCALHOST`: `bool` — `true` si le serveur ne répond que sur 127.0.0.1, `false` sinon
- Vérifié toutes les 30 s via goroutine dédiée (network watchdog)
- Pushé via WebSocket à chaque changement d'état
- Affiché comme bandeau rouge dans l'interface admin (GamePage)
- Jamais `omitempty` — toujours présent dans le GameState JSON

---

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

## Question (MEMOTION type) - v5.0.0

Nouveau type de jeu : grille de cartes à 3 faces (RECTO / VERSO / REVEAL).

```json
{
  "ID": "memotion_set_1",
  "QUESTION": "Quiz Couleurs",
  "TYPE": "MEMOTION",
  "MEMOTION_MODE": "CHACUN_SON_TOUR",
  "TIME": 30,
  "MEMOTION_CARDS": [
    {
      "ID": "card_1",
      "RECTO": {
        "TEXT": "Couleur du ciel",
        "IMAGE": "/files/memotion/card_1_recto.jpg"
      },
      "VERSO": {
        "TEXT": "Quel est le drapeau bleu, blanc, rouge?",
        "IMAGE": "/files/memotion/card_1_verso.jpg"
      },
      "REVEAL": {
        "TEXT": "France",
        "IMAGE": "/files/memotion/card_1_reveal.jpg"
      },
      "DIFFICULTY": 2
    },
    {
      "ID": "card_2",
      "RECTO": {
        "TEXT": "Fruit rouge",
        "IMAGE": "/files/memotion/card_2_recto.jpg"
      },
      "VERSO": {
        "TEXT": "Quel fruit granuleux et sucré ?",
        "IMAGE": "/files/memotion/card_2_verso.jpg"
      },
      "REVEAL": {
        "TEXT": "Fraise",
        "IMAGE": "/files/memotion/card_2_reveal.jpg"
      },
      "DIFFICULTY": 1
    }
  ]
}
```

**Champs MEMOTION :**
- `TYPE`: `"MEMOTION"` pour les jeux de cartes
- `MEMOTION_MODE`: Mode de jeu multi-équipes (optionnel, défaut: "SOLO")
  - `"SOLO"`: Une seule équipe joue, points à l'équipe
  - `"CHACUN_SON_TOUR"`: Rotation stricte après chaque carte
  - `"TANT_QUE_JE_GAGNE"`: L'équipe garde la main tant qu'elle gagne (points = difficulté)
- `MEMOTION_CARDS`: Tableau de cartes
  - `ID`: identifiant unique de la carte
  - `RECTO`: face 1 affichée en grille (TEXT + IMAGE optionnelle)
  - `VERSO`: face 2 question/énigme (TEXT + IMAGE optionnelle)
  - `REVEAL`: face 3 réponse (TEXT + IMAGE optionnelle)
  - `DIFFICULTY`: niveau de difficulté (1, 2, ou 3) → points attribués (1, 3, 5)
- `TIME`: durée timer per-carte (secondes, défaut: 30)

**Structure GameState MEMOTION** :
```json
{
  "PHASE": "MEMORY|PREPARE|READY|START|PAUSE|STOP",
  "MEMOTION_SUBPHASE": "GRID|SELECTED|QUESTION|REVEAL",
  "MEMOTION_SELECTED": "card_1",
  "MEMOTION_CARD_STATES": {
    "card_1": "DONE",
    "card_2": "AVAILABLE",
    "card_3": "AVAILABLE"
  },
  "MEMOTION_CARD_TEAMS": {
    "card_1": "team_A"
  },
  "MEMOTION_CURRENT_TEAM": "team_B",
  "MEMOTION_PARTICIPATING_TEAMS": ["team_A", "team_B"],
  "MEMOTION_CURRENT_TEAM_COLOR": [255, 0, 0]
}
```

**Card States (MEMOTION_CARD_STATES) :**
| État | Description |
|------|-------------|
| `UNPLAYED` | Carte non jouée, face RECTO visible en grille |
| `SELECTED` | Carte sélectionnée, subphase SELECTED (plein écran RECTO, pas de timer) |
| `QUESTION` | Carte en cours, face VERSO (question), timer actif |
| `REVEALED` | Face REVEAL affichée, admin attribue points |
| `DONE` | Carte jouée et retournée, couleur équipe appliquée |

**Note** : `MEMOTION_SUBPHASE` décrit l'état de la session (GRID/SELECTED/QUESTION/REVEAL) — toujours sérialisé sans `omitempty`. `MEMOTION_CARD_STATES` décrit l'état individuel de chaque carte.

## Bumper enrichi (v3.1.0+)

```json
{
  "ID": "AA:BB:CC:DD:EE:FF",
  "FIRMWARE_VERSION": "3.1.1",
  "IS_OUTDATED": false,
  "OTA_STATUS": "",
  "CONNECTED": true,
  "ACK_PENDING": false
}
```

- **IS_OUTDATED** : remis à `false` uniquement au reboot (réception HELLO avec nouvelle version). Ne change pas sur `OTA_PROGRESS done`.
- **CONNECTED** (v3.6.6) : `true` à la connexion WS, `false` à la déconnexion. Sans `omitempty`. Badge ⚠ jaune si `!IS_VIRTUAL && !IS_VPLAYER && !CONNECTED`. Au démarrage serveur, tous les bumpers chargés depuis disque ont `CONNECTED=false`.
- **Reconnexion rapide (v3.6.8)** : `OnBuzzerDisconnected` vérifie `IsClientConnected(mac)` avant de poser `CONNECTED=false` — reconnexion < 5 s transparente.
- **ACK_PENDING** (v3.8.0) : `true` quand le serveur attend l'ACK du buzzer (LED_SET, OTA_UPDATE, WIFI_CONFIG). Sans `omitempty`. Badge ⚠ horloge si `!IS_VIRTUAL && !IS_VPLAYER && ACK_PENDING`.

Badges frontend : `TeamCard.jsx` + `TeamsPage.jsx` (style inline React `background:#f59e0b`, SVG stroke:white).

## Question Status Values

| Statut | Description | Couleur UI |
|--------|-------------|------------|
| `AVAILABLE` | Question not yet played | Vert |
| `STARTED` | Question currently in play | Orange |
| `STOPPED` | Question was played but not revealed | Rouge |
| `REVEALED` | Answer has been shown | Gris |
