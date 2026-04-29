# Contrat WebSocket — Endpoints dédiés par type de client (v3.8.0)

> **Feature** : #11 — Filtrage broadcasts WebSocket par type de client
> **Branche** : `feature/ws-broadcast-ack-v380`
> **Dernière mise à jour** : 2026-04-28

---

## Endpoints WebSocket

### Tableau des endpoints

| Endpoint | Type client | Description |
|----------|-------------|-------------|
| `/ws/admin` | `admin` | Interface d'administration (pages `/admin/*`) |
| `/ws/tv` | `tv` | Affichage TV (`/tv`, `/scoreboard`, etc.) |
| `/ws/player` | `vplayer` | VJoueur virtuel (`/player`, `/enroll`) |
| `/ws/buzzer` | `buzzer` | Buzzers physiques ESP32 (existant, inchangé) |
| `/ws/logs` | `logs` | Logs temps réel (existant, inchangé) |
| `/ws` | `admin` | **LEGACY** — alias `/ws/admin`, retro-compatibilité |

**Règle de rétrocompatibilité** : Le endpoint `/ws` doit rester fonctionnel comme alias vers `admin`. Les clients
existants utilisant `/ws` avec `SET_CLIENT_TYPE` continuent de fonctionner.

**Règle de type** : Le type client est fixé à la connexion par l'URL, pas par `SET_CLIENT_TYPE`. L'action
`SET_CLIENT_TYPE` reste acceptée pour rétrocompatibilité mais est no-op si l'endpoint dédié est utilisé.

---

## Connexion

### Upgrade WebSocket

Tous les endpoints suivent le même protocole d'upgrade HTTP → WebSocket :

```
GET /ws/admin
Upgrade: websocket
Connection: Upgrade
```

Aucun payload initial requis. Le serveur fixe le type client en fonction de l'URL.

---

## Filtres de diffusion par type

Le serveur envoie chaque action uniquement aux types clients concernés :

| Action (Server→Client) | Admin | TV | VPlayer | Buzzer | Notes |
|------------------------|-------|-----|---------|--------|-------|
| `UPDATE` | ✓ full | ✓ partiel | ✓ partiel | ✓ réduit | Voir contrat ws-payload-serialization.md |
| `UPDATE_TIMER` | ✓ | ✓ | ✓ | ✓ | Même handler firmware que UPDATE |
| `START` / `CONTINUE` | ✓ | ✓ | ✓ | ✓ | Firmware : startGame() |
| `STOP` | ✓ | ✓ | ✓ | ✓ | Firmware : stopGame() |
| `PAUSE` | ✓ | ✓ | ✓ | ✓ | Firmware : pauseGame() |
| `REVEAL` | ✓ | ✓ | ✓ | — | |
| `READY` | ✓ | ✓ | ✓ | ✓ | Firmware : handleReadyAction() — gestion rotation grise |
| `RESET` | ✓ | ✓ | ✓ | ✓ | Firmware : resetGame() |
| `REMOTE` | ✓ | ✓ | — | — | |
| `QUESTIONS` | ✓ | — | — | — | Grande payload, inutile hors admin |
| `CLIENTS` | ✓ | — | — | — | Liste clients, admin uniquement |
| `FIRMWARE_VERSION` | ✓ | — | — | — | Info OTA, frontend uniquement |
| `BACKGROUND_CHANGE` | ✓ | ✓ | ✓ | — | VPlayer affiche PlayerDisplay |
| `QCM_HINT` | ✓ | ✓ | ✓ | — | |
| `SHOW_QR_CODE` | ✓ | ✓ | — | — | |
| `HIDE_QR_CODE` | ✓ | ✓ | — | — | |
| `ENROLLMENT_UPDATE` | ✓ | ✓ | ✓ | — | |
| `PLAYER_CONNECTED` | ✓ | — | ✓ | — | |
| `PLAYER_REJECTED` | — | — | ✓ | — | |
| `PLAYER_ASSIGNED` | ✓ | — | ✓ | — | |
| `CONFIG_UPDATE` | ✓ | ✓ | ✓ | — | PlayerDisplay utilise config (néon, etc.) |
| `FULL` | ✓ | ✓ | — | — | |
| `HELLO` | — | — | — | ✓ | Server→Buzzer : trigger reconnexion |
| `DELETE_BUMPER` | ✓ | — | — | — | |
| `MEMORY_SET_TEAMS` | ✓ | ✓ | — | — | |
| `FLIP_MEMORY_CARD` | ✓ | ✓ | — | — | |
| `TEAM_POINTS` | ✓ | ✓ | ✓ | — | Animation score |
| `BUMPER_POINTS` | ✓ | ✓ | ✓ | — | Animation score |
| `LED_SET` | — | — | — | ✓ | |
| `OTA_UPDATE` | — | — | — | ✓ | |
| `WIFI_CONFIG` | — | — | — | ✓ | |

---

## Méthodes backend à créer/modifier

### `WebSocketHub.HandleConnectionWithType(w, r, clientType)`

Variante de `HandleConnection` qui fixe le `ClientType` à la connexion sans attendre `SET_CLIENT_TYPE`.

```go
// Nouveau handler dans websocket.go
func (h *WebSocketHub) HandleConnectionWithType(
    w http.ResponseWriter,
    r *http.Request,
    clientType ClientType,
) {
    // ... upgrade + fix client.Type = clientType ...
}
```

### `WebSocketHub.BroadcastToTypes(msg, types...)`

Broadcast filtré — envoie uniquement aux clients des types spécifiés.

```go
// Nouveau dans websocket.go
func (h *WebSocketHub) BroadcastToTypes(msg *protocol.Message, types ...ClientType) {
    // serialize once, send to matching clients only
}
```

### `app.broadcastFiltered(msg, ...ClientType)` (main.go)

Wrapper appelant `wsHub.BroadcastToTypes`.

---

## Mapping frontend → endpoint

| Page/Hook | Endpoint actuel | Endpoint cible |
|-----------|----------------|----------------|
| `useWebSocket.js` (admin) | `/ws` | `/ws/admin` |
| `PlayerDisplay.jsx` (TV) | via `useWebSocket` | `/ws/tv` (hook dédié `useWebSocketTV`) |
| `VPlayerPage.jsx` | via `useWebSocket` + `SET_CLIENT_TYPE` | `/ws/player` |
| `EnrollPage.jsx` | via `useWebSocket` + `SET_CLIENT_TYPE` | `/ws/player` |
| `LogsPage.jsx` | `/ws/logs` | inchangé |

> **Note** : PlayerDisplay et VPlayerPage/EnrollPage requièrent soit (a) un hook WebSocket dédié,
> soit (b) une prop `wsEndpoint` à `useWebSocket`. L'approche (b) est préférable pour minimiser les changements.
