# Contrat WebSocket — Endpoints dédiés par type de client (v3.8.0)

> **Feature** : #11 — Filtrage broadcasts WebSocket par type de client
> **Branche** : `feature/ws-broadcast-ack-v380`
> **Dernière mise à jour** : 2026-08-13 (#155/#156 — endpoint `/ws/anim`, v6.2.0)

---

## Endpoints WebSocket

### Tableau des endpoints

| Endpoint | Type client | Description |
|----------|-------------|-------------|
| `/ws/admin` | `admin` | Interface d'administration (pages `/admin/*`) |
| `/ws/tv` | `tv` | Affichage TV (`/tv`, `/scoreboard`, etc.) |
| `/ws/player` | `vplayer` | VJoueur virtuel (`/player`, `/enroll`) |
| `/ws/anim` | `anim` | **v6.2.0 (#155)** — Interface animateur (`/anim`) : conduite du jeu depuis une tablette, capacités réduites (voir tableau de diffusion ci-dessous et `contracts/websocket-actions.md` §"Sécurité — Allow-list entrante") |
| `/ws/buzzer` | `buzzer` | Buzzers physiques ESP32 (existant, inchangé) |
| `/ws/logs` | `logs` | Logs temps réel (existant, inchangé) |
| `/ws` | `admin` | **LEGACY** — alias `/ws/admin`, retro-compatibilité |

> **[BREAKING] v6.2.0** : la route HTTP `/anim` cesse d'être un alias SPA de `/admin` (elle servait
> jusqu'ici `GamePage` avec les pleins droits régie, comme `/admin`). Elle sert désormais la
> nouvelle page animateur (`AnimPage.jsx`), connectée sur `/ws/anim` avec le `ClientType` réduit
> `anim`. `/anim/quiz`, `/anim/settings`, `/anim/logs` (les anciennes sous-routes admin dupliquées
> sous ce préfixe) ne résolvent donc plus vers la régie. `/admin/*` est strictement inchangée.
> Détail frontend : `_work/reports/plan-20260813-094321.md` tâche F2.

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

> **Colonne Animateur (v6.2.0, #155/#156)** : périmètre volontairement minimal. L'interface
> animateur (zones A/C — contexte + équipes, voir `_work/reports/plan-20260813-094321.md` F4) a
> besoin de l'état de jeu (`UPDATE`), de la question suivante (`NEXT_QUESTION`, exclusif) et du
> montant de base à créditer (`CREDIT_POINTS`, exclusif, ajouté suite code review MAJEUR-1 — voir
> `contracts/websocket-actions.md` §"Animateur") ; rien d'autre. Les actions ci-dessous que
> l'animateur peut lui-même **envoyer** (`START`, `STOP`,
> `PAUSE`, `CONTINUE`, `REVEAL`, `READY`, `BUMPER_POINTS`, `TEAM_POINTS` — voir
> `contracts/websocket-actions.md` §"Sécurité — Allow-list entrante") ne sont **pas** pour autant
> ré-émises vers lui sous leur propre nom : leur effet sur l'état de jeu lui parvient via `UPDATE`,
> comme pour tout autre client. Aucune tâche B1-B6 ne branche `ClientTypeAnim` sur ces échos
> dédiés (`broadcastStop`/`broadcastPauseAll`/`broadcastContinue`/`broadcastReveal`/
> `broadcastReady`, `main.go`) — ils restent Admin/TV/VPlayer comme aujourd'hui. Tout ce qui
> touche MEMORY/MEMOTION, l'enrôlement VJoueur, la configuration serveur ou le firmware buzzer est
> hors périmètre du socle #155 et de SPEEDY #156.

| Action (Server→Client) | Admin | TV | VPlayer | Animateur | Buzzer | Notes |
|------------------------|-------|-----|---------|-----------|--------|-------|
| `UPDATE` | ✓ full | ✓ partiel | ✓ partiel | ✓ partiel | ✓ réduit | Voir contrat ws-payload-serialization.md. Anim : `SerializeForWebClient`, comme TV/VPlayer |
| `UPDATE_TIMER` | ✓ | ✓ | ✓ | ✓ | ✓ | Chronomètre affiché en zone A (`AnimPage.jsx`) |
| `NEXT_QUESTION` | — | — | — | ✓ **exclusif** | — | **Nouveau (v6.2.0, #155)** — voir `contracts/websocket-actions.md` §"Animateur" |
| `CREDIT_POINTS` | — | — | — | ✓ **exclusif** | — | **Nouveau (v6.2.0, code review MAJEUR-1 #155/#156)** — voir `contracts/websocket-actions.md` §"Animateur" |
| `START` / `CONTINUE` | ✓ | ✓ | ✓ | ✗ | ✓ | Firmware : startGame(). Anim : état reflété via `UPDATE` uniquement |
| `STOP` | ✓ | ✓ | ✓ | ✗ | ✓ | Firmware : stopGame(). Anim : état reflété via `UPDATE` uniquement |
| `PAUSE` | ✓ | ✓ | ✓ | ✗ | ✓ | Firmware : pauseGame(). Anim : état reflété via `UPDATE` uniquement |
| `REVEAL` | ✓ | ✓ | ✓ | ✗ | — | Anim : état reflété via `UPDATE` uniquement |
| `READY` | ✓ | ✓ | ✓ | ✗ | ✓ | Firmware : handleReadyAction() — gestion rotation grise. Anim : état reflété via `UPDATE` uniquement |
| `RESET` | ✓ | ✓ | ✓ | ✗ | ✓ | Firmware : resetGame(). Hors périmètre régie de l'animateur |
| `REMOTE` | ✓ | ✓ | — | ✗ | — | Contrôle de l'affichage TV — régie uniquement |
| `QUESTIONS` | ✓ | — | — | ✗ | — | Grande payload, inutile hors admin. Anim en est exclu comme TV/VPlayer |
| `CLIENTS` | ✓ | — | — | ✗ | — | Liste clients, admin uniquement |
| `FIRMWARE_VERSION` | ✓ | — | — | ✗ | — | Info OTA, frontend admin uniquement |
| `BACKGROUND_CHANGE` | ✓ | ✓ | ✓ | ✗ | — | VPlayer affiche PlayerDisplay ; hors périmètre animateur |
| `QCM_HINT` | ✓ | ✓ | ✓ | ✗ | — | QCM hors périmètre #155/#156 (SPEEDY uniquement) |
| `SHOW_QR_CODE` | ✓ | ✓ | — | ✗ | — | Enrôlement — régie uniquement |
| `HIDE_QR_CODE` | ✓ | ✓ | — | ✗ | — | Enrôlement — régie uniquement |
| `ENROLLMENT_UPDATE` | ✓ | ✓ | ✓ | ✗ | — | Enrôlement — hors périmètre animateur |
| `PLAYER_CONNECTED` | ✓ | — | ✓ | ✗ | — | |
| `PLAYER_REJECTED` | — | — | ✓ | ✗ | — | |
| `PLAYER_ASSIGNED` | ✓ | — | ✓ | ✗ | — | |
| `CONFIG_UPDATE` | ✓ | ✓ | — | ✗ | — | `sendStateToClient` (HELLO) gate déjà Admin+TV, VPlayer exclu depuis #154 — animateur exclu de la même façon |
| `FULL` | ✓ | ✓ | — | ✗ | — | |
| `HELLO` | — | — | — | — | ✓ | Server→Buzzer : trigger reconnexion (sans rapport avec le HELLO client→serveur animateur) |
| `DELETE_BUMPER` | ✓ | — | — | ✗ | — | |
| `MEMORY_SET_TEAMS` | ✓ | ✓ | — | ✗ | — | MEMORY hors périmètre #155/#156 |
| `FLIP_MEMORY_CARD` | ✓ | ✓ | — | ✗ | — | MEMORY hors périmètre #155/#156 |
| `TEAM_POINTS` | ✓ | ✓ | ✓ | ✗ | — | Animation score. Anim : état reflété via `UPDATE` uniquement (peut ÉMETTRE cette action, ne la reçoit pas en écho) |
| `BUMPER_POINTS` | ✓ | ✓ | ✓ | ✗ | — | Animation score. Anim : état reflété via `UPDATE` uniquement (peut ÉMETTRE cette action, ne la reçoit pas en écho) |
| `LED_SET` | — | — | — | — | ✓ | |
| `OTA_UPDATE` | — | — | — | — | ✓ | |
| `WIFI_CONFIG` | — | — | — | — | ✓ | |

**`CONFIG_UPDATE` — correction 2026-08-13** : la colonne VPlayer de cette ligne affichait ✓ dans une
version antérieure de ce tableau ; c'est inexact depuis #154 (v6.1.4) — `sendStateToClient`
restreint désormais `CONFIG_UPDATE` à Admin+TV uniquement (`broadcastConfigUpdate` ne l'a jamais
diffusé à VPlayer). Corrigé ici à l'occasion de l'ajout de la colonne Animateur.

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
