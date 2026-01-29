# Actions WebSocket

> **Endpoint** : `/ws`
> **Format** : JSON
> **Dernière mise à jour** : 2026-01-27

---

## Client → Server

### HELLO

Enregistrement du client WebSocket.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Trigger   | Connexion WebSocket établie |

#### Payload

Aucun payload requis.

#### Exemple

```json
{
  "ACTION": "HELLO",
  "MSG": {}
}
```

---

### SET_CLIENT_TYPE

Définit le type de client (admin ou TV).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Trigger   | Après connexion |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| TYPE | string | ✅ | `"admin"` ou `"tv"` |

#### Exemple

```json
{
  "ACTION": "SET_CLIENT_TYPE",
  "MSG": {
    "TYPE": "admin"
  }
}
```

---

### START

Démarre une question.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | READY |
| Trigger   | Clic bouton "Démarrer" |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| DELAY | int | ❌ | Durée en secondes (défaut: question.TIME) |

#### Exemple

```json
{
  "ACTION": "START",
  "MSG": {
    "DELAY": 30
  }
}
```

---

### STOP

Arrête la question en cours.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | STARTED, PAUSED |

#### Payload

Aucun.

---

### PAUSE

Met en pause la question.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | STARTED |

#### Payload

Aucun.

---

### CONTINUE

Reprend après pause.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | PAUSED |

#### Payload

Aucun.

---

### READY

Sélectionne une question.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | STOP |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| QUESTION | string | ✅ | ID de la question |

#### Exemple

```json
{
  "ACTION": "READY",
  "MSG": {
    "QUESTION": "5"
  }
}
```

---

### REVEAL

Affiche la réponse.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | STOPPED |

#### Payload

Aucun.

---

### REMOTE

Change la vue TV.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| REMOTE | string | ✅ | `"GAME"`, `"SCORE"`, `"PLAYERS"`, `"PALMARES"` |

#### Exemple

```json
{
  "ACTION": "REMOTE",
  "MSG": {
    "REMOTE": "SCORE"
  }
}
```

---

### TEAM_POINTS

Modifie le score d'une équipe.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| TEAM | string | ✅ | Nom de l'équipe |
| POINTS | int | ✅ | Points à ajouter (peut être négatif) |

---

### BUMPER_POINTS

Modifie le score d'un joueur.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| ID | string | ✅ | MAC du bumper |
| POINTS | int | ✅ | Points à ajouter |

---

### UPDATE

Met à jour les équipes/joueurs.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| teams | object | ❌ | Map des équipes |
| bumpers | object | ❌ | Map des joueurs |

---

### RAZ

Remet tous les scores à zéro.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

Aucun.

---

### DELETE

Supprime une question.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| ID | string | ✅ | ID de la question |

---

### REORDER_QUESTIONS

Réordonne les questions.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| ORDER | []string | ✅ | IDs dans le nouvel ordre |

#### Exemple

```json
{
  "ACTION": "REORDER_QUESTIONS",
  "MSG": {
    "ORDER": ["3", "1", "5", "2", "4"]
  }
}
```

---

## Server → Client

### UPDATE

Broadcast l'état complet du jeu.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Trigger   | Tout changement d'état |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| GAME | GameState | État du jeu (voir game-state.md) |
| teams | object | Map des équipes |
| bumpers | object | Map des joueurs |
| VERSION | string | Version du serveur |

---

### QUESTIONS

Liste des questions.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Trigger   | Changement de questions |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| /files/questions/{id} | Question | Map des questions |
| FSINFO | object | Infos stockage |
| VERSION | string | Version serveur |

---

### CLIENTS

Compteurs de clients connectés.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Trigger   | Connexion/déconnexion client |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| ADMIN_COUNT | int | Nombre d'admins |
| TV_COUNT | int | Nombre de TVs |

---

### QCM_HINT

Invalide une réponse QCM (indice).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Phase     | STARTED |
| Trigger   | Seuil de temps atteint |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| COLOR | string | Couleur invalidée (`RED`, `GREEN`, `YELLOW`, `BLUE`) |
| REMAINING | int | Nombre de réponses restantes |

#### Exemple

```json
{
  "ACTION": "QCM_HINT",
  "MSG": {
    "COLOR": "RED",
    "REMAINING": 3
  }
}
```

---

### BACKGROUND_CHANGE

Changement de fond d'écran (synchronisé).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Trigger   | Cycle automatique serveur |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| INDEX | int | Index du fond (0-based) |

---

### LOG_HISTORY

Historique des logs (connexion WebSocket /ws/logs).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Endpoint  | `/ws/logs` |
| Trigger   | Connexion à /ws/logs |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| logs | []LogEntry | Tableau des logs |

---

### LOG_ENTRY

Nouveau log temps réel.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Endpoint  | `/ws/logs` |
| Trigger   | Nouveau log serveur |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| timestamp | string | ISO 8601 |
| level | string | DEBUG, INFO, WARN, ERROR |
| component | string | App, Engine, HTTP, etc. |
| message | string | Message du log |

---

## VPlayer (Joueurs Virtuels)

### SHOW_QR_CODE

Active l'affichage du QR Code.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Trigger   | Admin ouvre inscriptions |

---

### HIDE_QR_CODE

Désactive le QR Code.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Trigger   | Admin ferme inscriptions |

---

### PLAYER_CONNECT

Demande d'inscription VPlayer.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Trigger   | VPlayer soumet son pseudo |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| NAME | string | Pseudo du joueur |

---

### PLAYER_CONNECTED

Confirmation d'inscription.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| ID | string | ID assigné |
| NAME | string | Pseudo confirmé |

---

### PLAYER_REJECTED

Refus d'inscription.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| REASON | string | Raison du refus |

---

### ENROLLMENT_UPDATE

Mise à jour compteur inscriptions.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| VIRTUAL_PLAYER_COUNT | int | Nombre inscrits |
| VIRTUAL_PLAYER_LIMIT | int | Limite max |
| ENROLLMENT_ACTIVE | bool | Inscriptions ouvertes |

---

## Actions Système

### UPDATE_TIMER

Broadcast périodique du timer.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Trigger   | Chaque seconde pendant STARTED |

#### Payload

Même structure que UPDATE (GAME, teams, bumpers).

---

### RESET

Réinitialisation complète du serveur.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

Aucun.

---

### REBOOT

Redémarre le serveur.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

Aucun.

---

### PING

Vérification de connexion (buzzers).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |

#### Payload

Aucun.

---

### PONG

Réponse au PING.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

Aucun.

---

### FULL

Mise à jour complète (équipes + joueurs).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Trigger   | Modification équipes/joueurs |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| teams | object | Map complète des équipes |
| bumpers | object | Map complète des joueurs |

---

### DELETE_BUMPER

Supprime un joueur.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| ID | string | MAC ou ID du joueur |

---

### FORCE_READY

Force la transition PREPARE → READY (debug).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | PREPARE |

#### Payload

Aucun.

---

### PLAYER_ASSIGNED

Notification d'assignation VPlayer.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| ID | string | ID du joueur |
| TEAM | string | Nom de l'équipe |
| ANSWER_COLOR | string | Couleur assignée (RED/GREEN/YELLOW/BLUE) |

---

### BUTTON

Simulation buzzer (Ctrl+clic admin).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | STARTED, PAUSED |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| ID | string | MAC du bumper à simuler |
| button | string | Bouton ("A", "B", "C", "D") |

---

### FLIP_MEMORY_CARD

Retourne une carte Memory.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | STARTED |
| Type      | MEMORY |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| CARD_ID | string | ID de la carte (ex: "1-1", "2-2") |

---

### SET_VIRTUAL_PLAYER_LIMIT

Définit la limite de VPlayers.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| LIMIT | int | Nombre max de VPlayers |
