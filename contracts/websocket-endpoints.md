# Contrat WebSocket — Endpoints dédiés par type de client (v3.8.0)

> **Feature** : #11 — Filtrage broadcasts WebSocket par type de client
> **Branche** : `feature/ws-broadcast-ack-v380`
> **Dernière mise à jour** : 2026-08-13 (#162 — correction de la colonne Animateur ci-dessous : plusieurs
> diffusions documentées comme atteignant `/ws/anim` depuis #155/#156 n'y arrivaient en réalité jamais)

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

> **Colonne Animateur (v6.2.0, #155/#156, corrigée #162)** : périmètre volontairement minimal, mais
> **effectivement fonctionnel depuis #162**. Jusque-là, `broadcastUpdateTo` — la fonction qui
> sérialise `UPDATE` — n'avait **aucune branche `Anim`** : ajouter `ClientTypeAnim` à la liste d'un
> appelant ne suffisait pas, `hasType(Anim)` n'y était jamais évalué. Résultat en pratique, malgré ce
> que ce tableau affichait déjà : une tablette `/anim` connectée restait figée sur son état initial
> après `HELLO` — scores, chronomètre et phase de jeu ne suivaient jamais, y compris pour l'action que
> l'animateur venait lui-même de déclencher. #162 a ajouté la branche `Anim` (payload identique à TV,
> `SerializeForWebClient`) et étendu les listes de diffusion des actions de phase à `ClientTypeAnim`.
> Les actions que l'animateur peut lui-même **envoyer** (`START`, `STOP`, `PAUSE`, `CONTINUE`,
> `REVEAL`, `READY`, `RESET` — voir `contracts/websocket-actions.md` §"Sécurité — Allow-list entrante")
> lui sont donc désormais **aussi** ré-émises sous leur propre nom (voir tableau ci-dessous), en plus de
> l'`UPDATE` qui accompagne chacune. `TEAM_POINTS`/`BUMPER_POINTS` restent la seule exception : leur
> effet ne parvient à l'animateur que via l'`UPDATE` qui suit (comme pour tout autre client), pas en
> écho sous leur propre nom — inchangé par #162. Tout ce qui touche MEMORY/MEMOTION, l'enrôlement
> VJoueur, la configuration serveur ou le firmware buzzer reste hors périmètre du socle #155/#156/#162.

| Action (Server→Client) | Admin | TV | VPlayer | Animateur | Buzzer | Notes |
|------------------------|-------|-----|---------|-----------|--------|-------|
| `UPDATE` | ✓ full | ✓ partiel | ✓ partiel | ✓ partiel | ✓ réduit | Voir contrat ws-payload-serialization.md. Anim : `SerializeForWebClient`, comme TV/VPlayer. **Corrigé #162** : `broadcastUpdateTo` n'avait aucune branche `Anim` avant ce correctif — n'atteignait jamais réellement `/anim` malgré ce ✓ |
| `UPDATE_TIMER` | ✓ | ✓ | ✓ | ✓ | ✓ | Chronomètre affiché en zone A (`AnimPage.jsx`). **Corrigé #162** : le chronomètre ne s'écoulait jamais sur la tablette avant ce correctif (même cause que `UPDATE` ci-dessus) |
| `NEXT_QUESTION` | — | — | — | ✓ **exclusif** | — | **Nouveau (v6.2.0, #155)** — voir `contracts/websocket-actions.md` §"Animateur" |
| `CREDIT_POINTS` | — | — | — | ✓ **exclusif** | — | **Nouveau (v6.2.0, code review MAJEUR-1 #155/#156)** — voir `contracts/websocket-actions.md` §"Animateur" |
| `AWARDED_TEAMS` | — | — | — | ✓ **exclusif** | — | **Nouveau (v6.2.x, chantier crédit synchronisé — #170)** — équipes déjà créditées pour la question courante, **tous modes confondus**, projection de l'historique existant. Admin volontairement exclu : la régie reste sans garde de double-crédit. Voir `contracts/websocket-actions.md` §"Animateur" |
| `REGIE_MESSAGE` | ✓ | — | — | ✓ | — | **Nouveau (v6.4.x, #167)** — messagerie régie → animateurs. **Seule action sortante partagée par `admin` et `anim` à l'exclusion de tous les autres** : `/anim` la reçoit pour lire la consigne, `/admin` pour savoir si la sienne est encore en attente ou déjà acquittée (`CLEARED_BY`). Envoi ciblé au `HELLO` d'un client `admin` ou `anim` si un message est actif (rejeu, pas de re-broadcast). Voir `contracts/websocket-actions.md` §"Messagerie régie" |
| `START` / `CONTINUE` | ✓ | ✓ | ✓ | ✓ | ✓ | Firmware : startGame(). **Anim depuis #162** (`broadcastStart`/`broadcastContinue`) — avant, la phase ne suivait jamais sur la tablette, y compris pour son propre appui sur LANCER |
| `STOP` | ✓ | ✓ | ✓ | ✓ | ✓ | Firmware : stopGame(). **Anim depuis #162** (`broadcastStop`) |
| `PAUSE` | ✓ | ✓ | ✓ | ✓ | ✓ | Firmware : pauseGame(). **Anim depuis #162** (`broadcastPause`/`broadcastPauseAll`) |
| `REVEAL` | ✓ | ✓ | ✓ | ✓ | — | **Anim depuis #162** (`broadcastReveal`) — phase REVEALED (crédit) désormais suivie sur la tablette |
| `READY` | ✓ | ✓ | ✓ | ✓ | ✓ | Firmware : handleReadyAction() — gestion rotation grise. **Anim depuis #162** (`broadcastReady`) — équipes/buzzers en début de manche |
| `RESET` | ✓ | ✓ | ✓ | ✓ | ✓ | Firmware : resetGame(). **Anim depuis #162** (`broadcastReset`) |
| `REMOTE` | ✓ | ✓ | — | ✗ | — | Contrôle de l'affichage TV — régie uniquement |
| `QUESTIONS` | ✓ | — | — | ✗ | — | Grande payload, inutile hors admin. Anim en est exclu comme TV/VPlayer |
| `CLIENTS` | ✓ | — | — | ✗ | — | Liste clients, admin uniquement |
| `FIRMWARE_VERSION` | ✓ | — | — | ✗ | — | Info OTA, frontend admin uniquement |
| `BACKGROUND_CHANGE` | ✓ | ✓ | ✓ | ✗ | — | VPlayer affiche PlayerDisplay ; hors périmètre animateur |
| `QCM_HINT` | ✓ | ✓ | ✓ | ✓ | — | **Anim depuis #162** (`broadcastQCMHint`) — avant, `qcmInvalidated` n'atteignait jamais `/anim`, faussant le repli de pénalité QCM #157 quand aucun buzzer n'a répondu correctement |
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
| `TEAM_POINTS` | ✓ | ✓ | ✓ | ✗ | — | Animation score. Anim : état reflété via `UPDATE` uniquement (peut ÉMETTRE cette action, ne la reçoit pas en écho) — **cet `UPDATE` n'atteignait en réalité jamais `/anim` avant #162** |
| `BUMPER_POINTS` | ✓ | ✓ | ✓ | ✗ | — | Animation score. Anim : état reflété via `UPDATE` uniquement (peut ÉMETTRE cette action, ne la reçoit pas en écho) — **cet `UPDATE` n'atteignait en réalité jamais `/anim` avant #162** |
| `LED_SET` | — | — | — | — | ✓ | |
| `OTA_UPDATE` | — | — | — | — | ✓ | |
| `WIFI_CONFIG` | — | — | — | — | ✓ | |

**`CONFIG_UPDATE` — correction 2026-08-13** : la colonne VPlayer de cette ligne affichait ✓ dans une
version antérieure de ce tableau ; c'est inexact depuis #154 (v6.1.4) — `sendStateToClient`
restreint désormais `CONFIG_UPDATE` à Admin+TV uniquement (`broadcastConfigUpdate` ne l'a jamais
diffusé à VPlayer). Corrigé ici à l'occasion de l'ajout de la colonne Animateur.

**Colonne Animateur — correction 2026-08-13 (#162)** : `UPDATE`, `UPDATE_TIMER`, `START`/`CONTINUE`,
`STOP`, `PAUSE`, `REVEAL`, `READY`, `RESET` et `QCM_HINT` passent de ✗ à ✓ (ou étaient documentées ✓
sans réellement l'être — cas de `UPDATE`/`UPDATE_TIMER`, déjà cochées avant #162 alors que
`broadcastUpdateTo` n'avait aucune branche `Anim` : voir l'encart au-dessus du tableau). Aucun autre
changement : `QUESTIONS`, `CLIENTS`, `CONFIG_UPDATE`, `FIRMWARE_VERSION`, `BACKGROUND_CHANGE`,
`SHOW_QR_CODE`/`HIDE_QR_CODE`, `ENROLLMENT_UPDATE`, `PLAYER_*`, `FULL`, `DELETE_BUMPER`,
`MEMORY_SET_TEAMS`, `FLIP_MEMORY_CARD`, `TEAM_POINTS`/`BUMPER_POINTS` (en écho direct — leur `UPDATE`
associé, lui, est concerné ci-dessus) et les lignes buzzer-only restent ✗ pour Animateur, sans
changement de #162 — voir l'inventaire « À NE PAS modifier » de `_work/reports/plan-20260813-174513.md`
§2.3.

**Colonne Animateur — complément 2026-08-16 (#158)** : l'`UPDATE` déclenché par la **frappe ARDOISE
des joueurs** échappait à la correction #162 et restait **régie seulement**. Ce n'est pas
`broadcastUpdateTo` qui est en cause cette fois — il route correctement `Anim` depuis #162 — mais son
**appelant** : le coalesceur d'`ARDOISE_INPUT` (`cmd/server/main.go`, `NewBroadcastCoalescer(…, func()
{ a.broadcastUpdateTo(server.ClientTypeAdmin) })`, #129 T2.1/T2.2) ne lui passe que
`ClientTypeAdmin`. Conséquence : `GAME.ARDOISE_ANSWERS` est bien présent dans le payload animateur
(aucun filtrage, cf. `ws-payload-serialization.md`), mais il n'y est **rafraîchi qu'au prochain
changement de phase** — une tablette `/anim` verrait la liste des copies figée pendant toute la
frappe. Corrigé par #158 : le coalesceur cible désormais `ClientTypeAdmin` **et** `ClientTypeAnim`.

> Même famille de défaut que #162, à un étage différent de la pile : là c'était la fonction de
> diffusion qui ignorait le type, ici c'est un appelant qui ne le demande pas. **Tout nouvel appel à
> `broadcastUpdateTo` doit énumérer explicitement les types concernés** — l'oubli est silencieux, il
> ne produit ni erreur ni log, seulement un écran qui ne bouge plus.
>
> Périmètre strictement limité au coalesceur ARDOISE : `broadcastUpdateToPlayer` (VPlayer ciblé) et
> les autres appelants ne sont pas modifiés par #158.

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
