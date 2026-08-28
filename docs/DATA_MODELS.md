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

### Champs communs à tous les types de questions

| Champ | Type | Obligatoire | Description | Depuis |
|-------|------|-------------|-------------|--------|
| `ID` | string | ✅ | Identifiant unique numérique (chaîne) | v1.0 |
| `QUESTION` | string | ✅ | Énoncé de la question | v1.0 |
| `ANSWER` | string | ✅ | Réponse attendue (chaîne libre) | v1.0 |
| `TYPE` | string | ✅ | Type de question : `SPEEDY`, `QCM`, `MEMORY`, `MEMOTION`, `ARDOISE` | v1.0 |
| `POINTS` | int | ✅ | Points de base accordés | v1.0 |
| `TIME` | int | ✅ | Durée du chronomètre (secondes) | v1.0 |
| `MEDIA` | string | ❌ | Chemin de l'image/vidéo de la question | v1.0 |
| `MEDIA_ANSWER` | string | ❌ | Chemin de l'image/vidéo de la réponse | v1.0 |
| `EXPLANATION` | string | ❌ | Note d'explication (animateur seul, floutée jusqu'à REVEALED) | v6.4.0 (#168) |

**Note sur `EXPLANATION`** (v6.4.0, #168) :
- Affiché **uniquement** sur `/anim`, jamais sur `/admin`, `/tv`, `/player`
- Floutée avant phase `REVEALED`, révélée par appui maintenu (même geste que `ANSWER`)
- Permanente et lisible en `REVEALED`
- Persistance optionnelle (`omitempty` en JSON) — aucune migration requise
- Survit à la réédition de la question

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

## GameState — Métadonnées du Quiz (v6.0.0 #8, Batch 2b #137)

Champs globaux décrivant le quiz en cours. Éditables dans QuestionsPage, affichés sur l'écran TV NEW_GAME, pré-remplis dans le formulaire de génération IA.

```json
{
  "QUIZ_NAME": "Quiz Cinéma 80s",
  "QUIZ_THEME": "Cinéma français des années 80",
  "QUIZ_NOTES": "Soirée entre amis",
  "QUIZ_POPULATIONS": ["Adulte (18-64 ans)", "Senior (65+)"],
  "QUIZ_DIFFICULTIES": ["Moyen", "Difficile"],
  "QUIZ_OBJECTIVES": "Questions sur films de réalisateurs femmes",
  "QUIZ_HIDDEN_FIELDS": ["ANSWER"],
  "QUIZ_LANGUAGE": "Français"
}
```

**Champs :**

| Champ | Type | Défaut | Description | Depuis |
|-------|------|--------|-------------|--------|
| `QUIZ_NAME` | string | `""` | Titre du quiz | v4.0.0 |
| `QUIZ_THEME` | string | `""` | Thème principal du quiz | v4.0.0 |
| `QUIZ_NOTES` | string | `""` | Notes/contexte supplémentaires | v4.0.0 |
| `QUIZ_POPULATIONS` | []string | `[]` | Populations cibles (tableau, ex: `["Junior", "Ado"]`) | v6.0.0 #8, changement plural Batch 2b #137 |
| `QUIZ_DIFFICULTIES` | []string | `[]` | Difficultés visées (tableau, ex: `["Facile", "Moyen"]`) | v6.0.0 #8, changement plural Batch 2b #137 |
| `QUIZ_OBJECTIVES` | string | `""` | Objectifs/thème pédagogique de la partie (jamais transmis à `/ws/tv` ou `/ws/player`, confidentialité) | Batch 2b #137 |
| `QUIZ_HIDDEN_FIELDS` | []string | `[]` | Liste des champs question à masquer sur TV (ex: `["ANSWER"]`, jamais `omitempty`) | Batch 2b #137 |
| `QUIZ_LANGUAGE` | string | `"Français"` | Langue du quiz | v6.0.0 #8 |

**Règles d'implémentation :**

- Tous les champs sont **sans `omitempty`** — toujours sérialisés, jamais vides (chaîne vide `""` ou tableau vide `[]` autorisés)
- Éditables dans la section "Quiz" de QuestionsPage
  - **Populations & Difficultés** : sélection multiple (checkboxes) au lieu de champ texte unique (changement Batch 2b)
  - **Objectives** : champ texte libre (objectif pédagogique, jamais visible aux joueurs)
  - **Hidden Fields** : interrupteurs "Afficher sur la TV" par champ (ex: toggle "Réponse" pour masquer/afficher `ANSWER`)
- Affichés sur l'écran TV NEW_GAME en ligne compacte de badges — **tous les champs n'apparaissent que s'ils sont non-vides**
- Affichage TV : contrainte projet `overflow: hidden`, unités viewport, `flex` + `min-height: 0` (pas de scroll)
- Pré-remplissent automatiquement le formulaire de génération IA, mais **restent modifiables pour une génération précise sans affecter le global**

**Diffusion par endpoint WebSocket (Batch 2b #137) :**

- **`/ws/admin`** : Reçoit **tous les champs** sans filtrage (`QUIZ_OBJECTIVES` inclus)
- **`/ws/tv` et `/ws/player`** : Reçoivent **tous sauf `QUIZ_OBJECTIVES`** (jamais transmis, confidentialité). Reçoivent `QUIZ_HIDDEN_FIELDS` et appliquent le filtrage côté rendu (pas le serveur)

**Action WebSocket associée :**

Action `UPDATE_QUIZ_META` — sémantique **« champ absent = inchangé »** (jamais effacé si omis). Permet la rétrocompatibilité : un client antérieur envoyant seulement `NAME`/`THEME`/`NOTES` ne supprime pas les nouveaux champs.

```json
{
  "ACTION": "UPDATE_QUIZ_META",
  "MSG": {
    "NAME": "Quiz Cinéma 80s",
    "THEME": "Cinéma français des années 80",
    "NOTES": "Soirée entre amis",
    "POPULATIONS": ["Adulte (18-64 ans)", "Senior (65+)"],
    "DIFFICULTIES": ["Moyen", "Difficile"],
    "OBJECTIVES": "Questions sur films de réalisateurs femmes",
    "HIDDEN_FIELDS": ["ANSWER"],
    "LANGUAGE": "Français"
  }
}
```

**Énumérations :**

| Champ | Valeurs possibles |
|-------|-------------------|
| `QUIZ_POPULATIONS` (tableau) | `"Junior (6-12)"`, `"Ado (13-17)"`, `"Adulte (18-64 ans)"`, `"Senior (65+)"`, `"Famille"` — sélection multiple |
| `QUIZ_DIFFICULTIES` (tableau) | `"Facile"`, `"Moyen"`, `"Difficile"`, `"Expert"` — sélection multiple |
| `QUIZ_HIDDEN_FIELDS` (tableau) | `"ANSWER"` (réponse), `"MEDIA"` (image question), autres champs selon besoin TV |
| `QUIZ_LANGUAGE` | `"Français"` (par défaut), autres à venir |

**Changement Batch 2b (#137)** : Passage de champs singuliers (`QUIZ_POPULATION`, `QUIZ_DIFFICULTY`) à pluriels tableaux (`QUIZ_POPULATIONS`, `QUIZ_DIFFICULTIES`) pour sélection multiple. Ancien format rejeté par `/api/generate-questions` dès v6.1.0+.

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

### Seconde source de vérité : verrouillage du crédit animateur (v6.2.x, #170)

Depuis #170, l'historique de partie (`Engine.history` / `AddGameEvent` / `SaveHistory`) sert **deux**
consommateurs, pas un seul :

1. **PALMARÈS** (`GET /palmares`, `handlePalmares`) — filtre déjà les événements à `Points <= 0`
   (`internal/server/http.go:558`), inchangé par #170.
2. **Verrouillage du crédit côté interface animateur** (`/anim`, action `AWARDED_TEAMS`,
   `contracts/websocket-actions.md` §"Animateur") — **nouveau consommateur, aucun nouvel état**.
   Le serveur projette à la volée les événements `EventType == "POINTS_AWARDED"` de la question
   courante (`QuestionID == GameState.Question.ID` **et** `Timestamp >= GameState.GameTime`,
   regroupés par `TeamName`) pour déterminer, pour chaque équipe, si elle a déjà été créditée pour
   cette question et avec quel montant total — **tous modes confondus, quelle que soit
   l'interface d'origine du crédit** (`/admin` ou `/anim`).

**Un événement à `Points == 0` (refus explicite, geste "0 pt") est enregistré et projeté
exactement comme un crédit ordinaire** — le PALMARÈS l'écarte (montant non positif), le
verrouillage animateur le traite comme une entrée à part entière (voir
`contracts/websocket-actions.md` §"AWARDED_TEAMS" §"⚠️ `POINTS` peut valoir `0`"). Les deux
consommateurs lisent donc le **même** historique avec des filtres différents ; aucune structure
persistée n'a été ajoutée ou modifiée pour permettre ce second usage.

## Configuration System File (config.json)

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
  "storage": {
    "data_dir": "./data",
    "questions_dir": "./data/files/questions",
    "files_dir": "./data/files"
  },
  "ai": {
    "anthropic_api_key": "",
    "groq_api_key": "",
    "anthropic_api_key_verified": false,
    "groq_api_key_verified": false
  },
  "version": "6.1.2"
}
```

**Changements depuis v6.0.x (Milestone v6.0.x — #150, #143) :**

- **Sections `game` et `neon_effect` supprimées** — migrées vers `data/config/game-config.json` au démarrage. La migration est automatique et idempotente. Voir §Game Configuration ci-dessous.
- **Sections système conservées** : `server`, `wifi`, `storage`, `ai`. L'endpoint `/config.json` expose désormais **uniquement** la configuration système.

---

## Game Configuration File (data/config/game-config.json) - v6.0.x, #150

**Nouveau fichier** — contient les réglages de jeu, séparés de la configuration système pour être inclus dans les sauvegardes/restaurations de partie.

```json
{
  "game": {
    "default_delay": 30
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
  }
}
```

**Champs :**

| Champ | Description |
|-------|-------------|
| `game.default_delay` | Délai par défaut des questions (secondes) |
| `neon_effect.*` | Configuration effet néon (voir §Neon Effect Configuration) |

**Accès** :
- Endpoint HTTP : `GET /game-config.json` (lecture), `POST /game-config.json` (écriture)
- Inclus dans `/backup-select?history=true` (sélectif) et `/fs-backup` (complet)
- Restauré via `POST /restore` (détection automatique)

**Migration v6.0.x (#150)** :

À la première exécution, si `config.json` porte encore les sections `game` et `neon_effect` :
- Elles sont **extraites** vers ce nouveau fichier
- **Retirées** de `config.json`
- Un log indique la migration effectuée

Si démarrage sur `config.json` déjà migré : **aucune action** (idempotent).

---

## Game State Persistence File (data/config/game_state.json) - v6.0.x, #141

**Nouveau fichier** — persiste les métadonnées du quiz entre redémarrages du serveur.

```json
{
  "version": "1.0.0",
  "data": {
    "QUIZ_NAME": "Quiz Cinéma 80s",
    "QUIZ_THEME": "Cinéma français des années 80",
    "QUIZ_NOTES": "Soirée entre amis",
    "QUIZ_POPULATIONS": ["Adulte (18-64 ans)", "Senior (65+)"],
    "QUIZ_DIFFICULTIES": ["Moyen", "Difficile"],
    "QUIZ_LANGUAGE": "Français",
    "QUIZ_OBJECTIVES": "Questions sur films de réalisateurs femmes",
    "QUIZ_HIDDEN_FIELDS": ["ANSWER"],
    "VIRTUAL_PLAYER_LIMIT": 0
  }
}
```

**Structure :**

- **`version` (string)** — Format version (`"1.0.0"` pour cette implémentation). Permet migration future si le schéma change.
- **`data` (object)** — Métadonnées persistées :
  - `QUIZ_NAME`, `QUIZ_THEME`, `QUIZ_NOTES` — Libellés informatifs du quiz
  - `QUIZ_POPULATIONS`, `QUIZ_DIFFICULTIES` — Tableau de populations/difficultés ciblées
  - `QUIZ_LANGUAGE` — Langue du quiz
  - `QUIZ_OBJECTIVES` — Objectif pédagogique (admin only, jamais transmis aux joueurs)
  - `QUIZ_HIDDEN_FIELDS` — Tableau de champs à masquer sur TV (ex: `["ANSWER"]`)
  - `VIRTUAL_PLAYER_LIMIT` — Plafond de joueurs virtuels pour cette partie

**Champs NON persistés (éphémères, réinitialisés par `NEW_GAME`) :**

- `PHASE`, `QUESTION`, `CURRENT_TIME`, `DELAY` — État du jeu en cours
- `MEMORY_*`, `MOTION_*`, `ARDOISE_ANSWERS` — État spécifique au type de question
- `QUIZ_HIDDEN_FIELDS` — **PAS** persisté (revient à `[]` défaut à chaque `NEW_GAME`, pour que les réglages TV soient oubliés entre deux parties)

**Sémantique au démarrage :**

1. Le fichier est chargé si présent (après `NEW_GAME` ou au démarrage initial)
2. Les champs sont restaurés dans le `GameState` moteur
3. À chaque `NEW_GAME`, seuls ces champs métadonnées **restent en mémoire** ; l'état éphémère est réinitialisé (règle H5 : `QUIZ_HIDDEN_FIELDS` persiste en mémoire mais réinitialisation `NEW_GAME` l'efface — ce comportement change en v6.0.x pour durable mais le contenu reste local à la session)

**Accès** :

- **Pas d'endpoint HTTP direct** — le fichier est géré par le serveur uniquement
- Inclus dans `/backup-select?history=true` (sélectif) et `/fs-backup` (complet)
- Restauré via `POST /restore` (détection automatique) — rechargement en mémoire sans redémarrage
- Supprimé par `/reset-select?history=true`

**Versionnement** :

Ce fichier est le **premier du projet à utiliser un champ `version`** — préparant toute évolution future du schéma (ajout de champs, changement de structure). Un lecteur qui reçoit une version inconnue **doit ignorer le fichier et logger un avertissement** (migration future pourra upgrader).

---

## Configuration Neon Effect (v2.46.0)

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

## MotionCard — Carte polymorphe MEMOTION (v7.0.0, #184)

### Structure MotionCard — contenu typé et contrôle plat

Une carte MEMOTION porte un **type discriminant** (`TYPE`) et un **contenu typé partagé** (`TypedContent`), tous deux embarqués à plat dans la structure card. Aucune imbrication.

```json
{
  "ID": "mc-1",
  "TYPE": "QCM",
  "RECTO_THEME": "Géographie",
  "RECTO_IMAGE": "/files/memotion/mc-1_recto.jpg",
  "DIFFICULTY": 2,
  "QUESTION_TEXT": "Quelle est la capitale de la France?",
  "QUESTION_IMAGE": "",
  "ANSWER_TEXT": "",
  "ANSWER_IMAGE": "",
  "POINTS_RULE": {
    "MODE": "STARS",
    "VALUE": 10
  },
  "QCM_ANSWERS": {
    "RED": "Londres",
    "GREEN": "Paris",
    "YELLOW": "Berlin",
    "BLUE": "Madrid"
  },
  "QCM_CORRECT": "GREEN",
  "QCM_HINTS_ENABLED": false,
  "QCM_HINT_THRESHOLD_1": 0.25,
  "QCM_HINT_THRESHOLD_2": 0.125,
  "QCM_PENALTY_1": 0.67,
  "QCM_PENALTY_2": 0.33
}
```

**Champs de la carte (structure cardinale) :**

| Champ | Type | Défaut | Description |
|-------|------|--------|-------------|
| `ID` | string | ✅ requis | Identifiant unique de la carte |
| `TYPE` | string | `"SPEEDY"` | Type de contenu : `"SPEEDY"` (défaut), `"QCM"`, `"MEMORY"`, `"ARDOISE"`. Absent ⇒ `SPEEDY` (rétrocompatibilité) |
| `RECTO_THEME` | string | ✅ requis | Thème de la face recto affichée en grille |
| `RECTO_IMAGE` | string | optionnel | Image recto (chemin `/files/...`) |
| `DIFFICULTY` | int | ✅ requis | Difficulté : 1, 2, ou 3 |
| `QUESTION_TEXT` | string | optionnel | Énoncé (SPEEDY historique) |
| `QUESTION_IMAGE` | string | optionnel | Image énoncé (SPEEDY historique) |
| `ANSWER_TEXT` | string | optionnel | Réponse texte (SPEEDY historique) |
| `ANSWER_IMAGE` | string | optionnel | Réponse image (SPEEDY historique) |
| `POINTS_RULE` | object | optionnel | Barème de points de la carte (§6.2) |

**Jamais `omitempty`** : tous les champs sont toujours sérialisés. Les champs optionnels valent `""` ou `null`, jamais absents.

### TypedContent — contenu partagé embarqué plat

Les champs de contenu propres à un type sont **embarqués à plat** dans la même structure MotionCard. Aucun niveau d'imbrication supplémentaire.

```go
// Champs de contenu typé — embarqués plat dans MotionCard
type TypedContent struct {
    // SPEEDY / ARDOISE
    Answer      string `json:"ANSWER,omitempty"`
    
    // QCM
    QCMAnswers        *QCMAnswers `json:"QCM_ANSWERS,omitempty"`
    QCMCorrect        string      `json:"QCM_CORRECT,omitempty"`
    QCMHintsEnabled   bool        `json:"QCM_HINTS_ENABLED,omitempty"`
    QCMHintThreshold1 float64     `json:"QCM_HINT_THRESHOLD_1,omitempty"`
    QCMHintThreshold2 float64     `json:"QCM_HINT_THRESHOLD_2,omitempty"`
    QCMPenalty1       float64     `json:"QCM_PENALTY_1,omitempty"`
    QCMPenalty2       float64     `json:"QCM_PENALTY_2,omitempty"`
    
    // ARDOISE
    ArdoiseKeyboardType KeyboardType `json:"ARDOISE_KEYBOARD_TYPE,omitempty"`
    
    // MEMORY
    MemoryPairs  []MemoryPair  `json:"MEMORY_PAIRS,omitempty"`
    MemoryConfig *MemoryConfig `json:"MEMORY_CONFIG,omitempty"`
    MemoryMode   string        `json:"MEMORY_MODE,omitempty"`
}
```

**Champs par type** :

| Type | Champs embarqués | Description |
|------|------------------|-------------|
| `SPEEDY` | `ANSWER_TEXT`, `ANSWER_IMAGE` | Réponse texte et/ou image — historique |
| `QCM` | `QCM_ANSWERS` (4 options), `QCM_CORRECT` (couleur gagnante), `QCM_HINTS_ENABLED`, `QCM_HINT_THRESHOLD_1/2`, `QCM_PENALTY_1/2` | Système d'indices avec seuils et pénalités configurables |
| `ARDOISE` *(v7.1.0)* | `ANSWER`, `ARDOISE_KEYBOARD_TYPE` | Réponse libre au clavier (AZERTY, NUMPAD) |
| `MEMORY` *(v7.1.0)* | `MEMORY_PAIRS`, `MEMORY_CONFIG`, `MEMORY_MODE` | Paires et configuration multi-équipes |

**Rétrocompatibilité** : une carte sans `TYPE` vaut `SPEEDY` et se comporte exactement comme avant. Aucune migration de fichier.

### Verrouillage du type d'une carte (v7.0.0, #184, §3.2)

**Une carte ne peut pas changer de type une fois qu'elle porte du contenu propre à son type.**

Pas d'avertissement : c'est **interdit** côté serveur. Le verrou n'oblige l'utilisateur à **vider explicitement** le contenu propre au type avant de basculer.

| Champ de la carte | Verrouille le type ? |
|---|---|
| `RECTO_THEME`, `RECTO_IMAGE`, `DIFFICULTY`, `POINTS_RULE`, `QUESTION_TEXT`, `QUESTION_IMAGE` | ❌ **Non** — ces champs appartiennent à la carte, pas au type, et survivent intacts à toute bascule |
| Champs `OwnedFields` du type (ex: `ANSWER_TEXT` pour SPEEDY, `QCM_ANSWERS` pour QCM) | ✅ **Oui** — dès qu'un est renseigné, le type se verrouille jusqu'à vidage explicite |

**Règle serveur** : une carte reçue avec du contenu appartenant à un type différent de son `TYPE` déclaré rejette avec HTTP 400 `CARD_TYPE_CONTENT_MISMATCH`. Garantit l'intégrité sans état.

## POINTS_RULE — barème de points d'une carte (v7.0.0, #184, §6.2)

```json
"POINTS_RULE": {
  "MODE": "STARS",
  "VALUE": 10
}
```

Le barème appartient **toujours à la carte (hôte), jamais au type imbriqué.**

| `MODE` | Points attribués | Usage | Défaut |
|---|---|---|---|
| absent ou `"STARS"` | Barème par étoiles existant : `MOTION_CONFIG.POINTS_<n>_STAR` si `> 0`, sinon `DIFFICULTY → 1/3/5` | Comportement actuel, inchangé | ✅ |
| `"FIXED"` | `VALUE` si jeu gagné, sinon 0 | Carte dont la valeur est indépendante de la difficulté | — |
| `"PER_UNIT"` | `VALUE × Units` (Units = nombre d'items trouvés) | Types à progression — MEMORY au prorata (v7.1.0) | — |

**`Units` par défaut = 1** pour tout type binaire (gagné/perdu). Un type à progression rapporte son propre décompte.

---

## GameState.MEMOTION_ACTIVE — emplacement actif (v7.0.0, #184, §5.2)

Une seule carte MEMOTION est jouable à la fois. Son état vivant (réponses invalides, réactions, etc.) est stocké dans un emplacement unique `MEMOTION_ACTIVE`, remis à zéro à chaque sélection et vidé au retour en `GRID`.

```json
{
  "MEMOTION_ACTIVE": {
    "CARD_ID": "mc-3",
    "TYPE": "QCM",
    "STATE": {
      "QCM_INVALIDATED": ["RED", "YELLOW"]
    }
  }
}
```

**Structure** :

| Champ | Type | Description |
|-------|------|-------------|
| `CARD_ID` | string | ID de la carte en jeu — `""` hors phases SELECTED/QUESTION/REVEAL |
| `TYPE` | string | Type de la carte (`"QCM"`, `"SPEEDY"`, etc.) — `""` si aucune carte active. `"SPEEDY"` pour carte historique |
| `STATE` | object | État vivant du type — forme libre, propre à chaque type (ex: QCM invalidées, paires MEMORY trouvées) |

**Règles** :

- **Jamais `omitempty`** — toujours sérialisé, même vide (`{"CARD_ID":"","TYPE":"","STATE":{}}`)
- **Non persisté** — champ éphémère, rejoint les champs Motion* dans `state_persistence.go`
- `CARD_ID` = `MEMOTION_SELECTED` invariant (même valeur que le champ de sélection)
- `STATE` restituant l'état typé : accès via `getTypeState(gameState, hostContext)` côté JS (§5.3 du contrat)

---

## Question (MEMOTION type) - v5.0.0 — Hôte MEMOTION (structure Question historique, v7.0.0 compatible)

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
      "TYPE": "SPEEDY",
      "RECTO_THEME": "Couleurs",
      "RECTO_IMAGE": "/files/memotion/card_1_recto.jpg",
      "DIFFICULTY": 2,
      "QUESTION_TEXT": "Couleur du ciel?",
      "ANSWER_TEXT": "Bleu"
    },
    {
      "ID": "card_2",
      "TYPE": "QCM",
      "RECTO_THEME": "Drapeaux",
      "RECTO_IMAGE": "/files/memotion/card_2_recto.jpg",
      "DIFFICULTY": 1,
      "QUESTION_TEXT": "Quel est ce drapeau?",
      "QCM_ANSWERS": {"RED": "France", "GREEN": "Italie", "YELLOW": "Allemagne", "BLUE": "Espagne"},
      "QCM_CORRECT": "RED"
    }
  ]
}
```

**Champs Question MEMOTION :**
- `TYPE`: `"MEMOTION"` pour les jeux d'hôte carte
- `MEMOTION_MODE`: Mode multi-équipes (optionnel, défaut: `"SOLO"`)
  - `"SOLO"`: Une seule équipe joue, points à l'équipe
  - `"CHACUN_SON_TOUR"`: Rotation stricte après chaque carte
  - `"TANT_QUE_JE_GAGNE"`: L'équipe garde la main tant qu'elle gagne
- `MEMOTION_CARDS`: Tableau de `MotionCard` — chacune suit la structure plate ci-dessus
- `TIME`: Durée timer par carte (secondes, défaut: 30)

**Cartes imbriquées** : chaque élément de `MEMOTION_CARDS` est une `MotionCard` complète, avec son propre `TYPE`, `TypedContent` embarqué plat, et `POINTS_RULE` optionnel.

### Structure GameState MEMOTION

```json
{
  "PHASE": "MEMORY|PREPARE|READY|START|PAUSE|STOP",
  "MEMOTION_SUBPHASE": "GRID|SELECTED|QUESTION|REVEAL",
  "MEMOTION_SELECTED": "card_1",
  "MEMOTION_ACTIVE": {
    "CARD_ID": "card_1",
    "TYPE": "QCM",
    "STATE": {
      "QCM_INVALIDATED": ["RED", "YELLOW"]
    }
  },
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

**Champs** :

| Champ | Type | Description |
|-------|------|-------------|
| `PHASE` | string | Phase du jeu : MEMORY, PREPARE, READY, START, PAUSE, STOP |
| `MEMOTION_SUBPHASE` | string | Sous-phase MEMOTION : GRID (affichage grille), SELECTED (plein écran recto), QUESTION (verso/énoncé), REVEAL (réponse) — jamais `omitempty` |
| `MEMOTION_SELECTED` | string | ID de la carte sélectionnée — `""` en GRID. Même valeur que `MEMOTION_ACTIVE.CARD_ID` |
| `MEMOTION_ACTIVE` | object | État vivant de la carte active (§MEMOTION_ACTIVE ci-dessus) — jamais `omitempty` |
| `MEMOTION_CARD_STATES` | object | État individuel de chaque carte (`AVAILABLE`, `DONE`, etc.) |
| `MEMOTION_CARD_TEAMS` | object | Équipe créditée par carte |
| `MEMOTION_CURRENT_TEAM` | string | Équipe jouant actuellement (rotation multi-équipes) |
| `MEMOTION_PARTICIPATING_TEAMS` | array | Liste ordonnée des équipes en jeu |
| `MEMOTION_CURRENT_TEAM_COLOR` | array | Couleur RGB de l'équipe courante |

**Card States (MEMOTION_CARD_STATES) :**

| État | Description |
|------|-------------|
| `AVAILABLE` | Carte non jouée, face recto visible en grille |
| `DONE` | Carte jouée et retournée, couleur équipe appliquée |

**Note** : `MEMOTION_SUBPHASE` décrit l'état de la session (GRID/SELECTED/QUESTION/REVEAL) — toujours sérialisé sans `omitempty`. `MEMOTION_CARD_STATES` décrit l'état d'exécution de chaque carte.

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

## EntracteConfig et GameState.ENTRACTE (v6.5.2, #119)

### Modèle : EntracteConfig

```go
type EntracteConfig struct {
    TITLE          string `json:"TITLE"`           // Titre du panneau (défaut "ENTRACTE", sans omitempty)
    SUBTITLE       string `json:"SUBTITLE"`        // Sous-titre (défaut "Retour dans 20mn", sans omitempty)
    IMAGE_IS_CUSTOM bool  `json:"IMAGE_IS_CUSTOM"` // Image de fond présente (sans omitempty)
    PANEL_SIZE     int    `json:"PANEL_SIZE"`      // Pourcentage 20–100 (défaut 65, sans omitempty)
    ANIM_PERIOD    int    `json:"ANIM_PERIOD"`     // Durée cycle en secondes 2–30 (défaut 10, sans omitempty)
    ANIM_INTENSITY int    `json:"ANIM_INTENSITY"`  // Amplitude 0–100 (défaut 20 ; 0=désactivée, sans omitempty)
}
```

### Champ GameState.ENTRACTE

```go
type GameState struct {
    ...
    ENTRACTE              bool             `json:"ENTRACTE"`                // Pause active (sans omitempty)
    ENTRACTE_CONFIG       EntracteConfig   `json:"ENTRACTE_CONFIG"`         // Configuration courante (sans omitempty)
    ENTRACTE_CONFIG_SAVED EntracteConfig   `json:"ENTRACTE_CONFIG_SAVED"`   // Configuration gelée à l'activation (admin-only, sans omitempty)
    ...
}
```

#### Gel de la configuration à l'activation

- `ENTRACTE_CONFIG` : configuration courante, modifiable à tout moment
- `ENTRACTE_CONFIG_SAVED` : snapshot sauvegardé automatiquement dès `ENTRACTE = true`
- Pendant l'entracte : le panneau affiche `ENTRACTE_CONFIG_SAVED`, jamais la courante
- Modifications de `ENTRACTE_CONFIG` pendant une pause active : prennent effet au **prochain 
  cycle d'entracte** (après sortie + nouvelle entrée)
- Client reçoit uniquement `ENTRACTE_CONFIG` (configuration courante) dans les payloads 
  ordinaires (`UPDATE`, `HELLO`) ; `ENTRACTE_CONFIG_SAVED` n'est jamais diffusé (admin-only, 
  transparence interne)

**Motif** : éviter que des modifications pendant une pause active perturbent le panneau affiché 
sur les quatre surfaces.

**Note** : aucun `omitempty` sur ces champs — un client doit toujours savoir que 
l'entracte est **terminé** (`ENTRACTE = false`), d'où la présence du champ même à 
valeur fausse. La règle projet CLAUDE.md s'applique intégralement.

### Configuration persistée : GameSettings.Entracte

```go
type GameSettings struct {
    ...
    Entracte EntracteConfig `json:"entracte"`  // Section entracte dans game-config.json
    ...
}
```

Persistée dans `game-config.json`, reloadée au démarrage et à chaque sauvegarde. État 
(`ENTRACTE = true/false`) non persisté — propriété `ShowQRCode` est l'analogue existant.

### Filtrage par type de client

- **Admin, TV, VJoueur, Animateur** : reçoivent les deux champs `ENTRACTE` et 
  `ENTRACTE_CONFIG` via la liste de retrait existante (`SerializeForWebClient`, pas de 
  modification).
- **Buzzers** : ne reçoivent ni champ (liste d'autorisation `SerializeForBuzzer`, 
  aucune modification requise).

### Image de fond

Image unique optionnelle :
- Stockée dans `data/files/entracte/` (répertoire dédié, distinct de `data/files/`).
- Accessible via `GET /api/game/entracte-image`.
- Upload/suppression via `POST`/`DELETE /api/game/entracte-image`.
- Le champ `IMAGE_IS_CUSTOM` véhicule uniquement un booléen — jamais le chemin.
- URL stable : le client construit l'URL avec cache-buster pour forcer le rechargement après upload.

### Animation du panneau

Deux paramètres : `ANIM_PERIOD` (vitesse) et `ANIM_INTENSITY` (amplitude).

À `ANIM_INTENSITY = 0`, **aucune animation n'est déclarée** — pas seulement une 
amplitude nulle qui tournerait quand même. Le panneau reste fixe.

Sous `prefers-reduced-motion: reduce`, animation neutralisée quelle que soit la 
configuration (accessibilité, respect des préférences système).

## Question (RAFALE type) - v8.0.0, #16

### Modèle RafaleQuestion — Élément du réservoir

```json
{
  "ID": "r-001",
  "QUESTION": "Capitale de l'Italie ?",
  "ANSWER": "Rome",
  "CATEGORY": "GEOGRAPHY",
  "DIFFICULTY": 1
}
```

**Champs** :
- `ID` : identifiant stable, unique dans le réservoir (chaîne)
- `QUESTION` : énoncé, texte seul (pas de média)
- `ANSWER` : réponse attendue, texte seul
- `CATEGORY` : réutilise l'enum `QuestionCategory` + catégories personnalisées découvertes dans `data/files/categories/`
- `DIFFICULTY` : échelle 1–3 (identique `MotionCard.Difficulty`)

**Stockage** : Fichier unique `data/files/rafale/reservoir.json` avec structure `{"QUESTIONS": [RafaleQuestion, ...]}`

**Persistance** : Typée (`LoadRafale()` / `SaveRafale()` / `SetRafalePath()` dans `internal/game/rafale_store.go`), recharger à `App.init()`.

### Flag "déjà utilisée" — Persistant

```json
{
  "USED": {
    "r-001": true,
    "r-005": true
  }
}
```

**Stockage** : Fichier `data/config/rafale_used.json`, **séparé du réservoir** intentionnellement.

**Comportement** :
- Persistant à travers redémarrages (survit fermeture/ouverture serveur)
- Réinitialisé automatiquement dans `InitGame()` (commande `NEW_GAME`) — appel `safeGo("SaveRafaleUsed", ...)`
- Édition du réservoir ne réécrira pas le flag (et inversement)
- La pioche marque la question immédiatement et persiste atomiquement

### Configuration de manche — Question type RAFALE

Réutilise la structure `Question` avec champs existants + nouveaux champs `TypedContent` :

**Champs existants réutilisés** :
- `TIME` : durée totale de la manche (secondes, défaut 120)
- `POINTS` : barème de manche (points d'une bonne réponse)

**Champs nouveaux** (portés par `TypedContent`, tous `omitempty`) :
```go
RafaleCategories   []string `json:"RAFALE_CATEGORIES,omitempty"`    // multi-sélection, ≥1
RafaleDifficulty   int      `json:"RAFALE_DIFFICULTY,omitempty"`    // 1–3, unique par manche
RafaleMode         string   `json:"RAFALE_MODE,omitempty"`          // SOLO|CHACUN_SON_TOUR|TANT_QUE_JE_GAGNE|MAILLON_FAIBLE
RafaleQuestionTime int      `json:"RAFALE_QUESTION_TIME,omitempty"` // secondes par question, défaut 3
RafaleMaxQuestions int      `json:"RAFALE_MAX_QUESTIONS,omitempty"` // plafond dur, défaut 100, max 100
```

### Structure GameState RAFALE

```json
{
  "RAFALE_SUBPHASE": "QUESTION",
  "RAFALE_CURRENT_QUESTION": {
    "ID": "r-001",
    "QUESTION": "Capitale de l'Italie ?",
    "CATEGORY": "GEOGRAPHY",
    "DIFFICULTY": 1
  },
  "RAFALE_QUESTION_TIME": 2500,
  "RAFALE_TEAM_COUNTERS": {
    "team_A": 3,
    "team_B": 2
  },
  "RAFALE_TEAM_BEST": {
    "team_A": 3,
    "team_B": 2
  },
  "RAFALE_CURRENT_TEAM": "team_A",
  "RAFALE_PARTICIPATING_TEAMS": ["team_A", "team_B"],
  "RAFALE_CURRENT_TEAM_COLOR": [255, 26, 26],
  "RAFALE_ASKED_COUNT": 12,
  "RAFALE_POOL_REMAINING": 38,
  "RAFALE_EXHAUSTED": false
}
```

**Champs** (tous sans `omitempty`, initialisés non-nil) :

| Champ | Type | Description |
|-------|------|-------------|
| `RAFALE_SUBPHASE` | string | État interne : `""` (inactif), `"QUESTION"` (question posée, timer actif), `"ROUND_END"` (attribution points) |
| `RAFALE_CURRENT_QUESTION` | object | Question courante **SANS la réponse attendue** (`RafaleCurrent` : ID, QUESTION, CATEGORY, DIFFICULTY uniquement) |
| `RAFALE_QUESTION_TIME` | int | Décompte timer question courant (microsecondes) |
| `RAFALE_TEAM_COUNTERS` | map | Compteur réponses correctes par équipe (key = team ID, value = count) |
| `RAFALE_TEAM_BEST` | map | Meilleur compteur atteint par équipe (MAILLON_FAIBLE uniquement, vide sinon) |
| `RAFALE_CURRENT_TEAM` | string | Équipe active (multi-mode), vide en SOLO |
| `RAFALE_PARTICIPATING_TEAMS` | array | Équipes en jeu, ordre de rotation |
| `RAFALE_CURRENT_TEAM_COLOR` | array | Couleur RGB équipe active (utilisée pour LED + affichage) |
| `RAFALE_ASKED_COUNT` | int | Nombre total questions tirées depuis début manche |
| `RAFALE_POOL_REMAINING` | int | Questions encore disponibles (recalculé à chaque tirage) |
| `RAFALE_EXHAUSTED` | bool | `true` si pool vide pendant manche |

**Persistance** : Tous ces champs sont **éphémères** (exclus de `PersistedGameState`). Seul `rafale_used.json` persiste.

### Types d'élément `RafaleCurrent`

```go
type RafaleCurrent struct {
    ID         string `json:"ID"`
    QUESTION   string `json:"QUESTION"`
    CATEGORY   string `json:"CATEGORY"`
    DIFFICULTY int    `json:"DIFFICULTY"`
}
```

**Invariant critique** : ce type n'a **jamais** de champ `ANSWER`. Cela garantit structurellement que la réponse attendue ne transite jamais via `GameState`, même si un refactor oublie la sérialisation spéciale `RAFALE_ANSWER`.

### Modes de jeu RAFALE

| Mode | Chaîne | Rotation | Compteur sur Mauvais |
|------|--------|----------|----------------------|
| Solo | `"SOLO"` | Aucune | N/A (pas de rotation) |
| Chacun son tour | `"CHACUN_SON_TOUR"` | À chaque réponse (V ou I) | Conservé |
| Tant que je gagne | `"TANT_QUE_JE_GAGNE"` | Réponse I uniquement | Conservé, même équipe continue sur V |
| Maillon faible | `"MAILLON_FAIBLE"` | À chaque réponse (V ou I) | Remis à 0, meilleur mémorisé |

**Scoring pendant manche** : Aucun point réel, seul compteur visible.

**Scoring en fin de manche (ROUND_END)** : Admin/anim clique équipe → `TEAM_POINTS`, valeur pré-remplie = `compteur_retenu × POINTS` (compteur_retenu = `TEAM_BEST` en MAILLON, `TEAM_COUNTERS` sinon). Ajustable avant validation.
