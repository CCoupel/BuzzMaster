# Contrat Sérialisation Payload WebSocket par type de client (v3.8.0)

> **Feature** : #41 étendu — Réduire payload WS pour tous les types de clients  
> **Branche** : `feature/ws-broadcast-ack-v380`  
> **Créé** : 2026-04-28 (analyse QUALIF — VPlayerPage embed PlayerDisplay)

---

## Contexte

VPlayerPage.jsx embarque directement `<PlayerDisplay>` — la page TV de poche.
Les besoins TV et VPlayer sont donc identiques côté messages.

---

## Tableau 1 — Filtrage par type de message

| Action (Server→Client) | Admin | TV | VPlayer | Buzzer |
|------------------------|-------|----|---------|--------|
| `UPDATE` / `UPDATE_TIMER` | ✅ full | ✅ partiel | ✅ partiel | ✅ réduit |
| `START` / `CONTINUE` / `STOP` / `PAUSE` | ✅ | ✅ | ✅ | ✅ |
| `READY` / `RESET` | ✅ | ✅ | ✅ | ✅ |
| `REVEAL` | ✅ | ✅ | ✅ | ❌ |
| `QUESTIONS` | ✅ | ❌ | ❌ | ❌ |
| `CLIENTS` | ✅ | ❌ | ❌ | ❌ |
| `FIRMWARE_VERSION` | ✅ | ❌ | ❌ | ❌ |
| `BACKGROUND_CHANGE` | ✅ | ✅ | ✅ | ❌ |
| `QCM_HINT` | ✅ | ✅ | ✅ | ❌ |
| `CONFIG_UPDATE` | ✅ | ✅ | ✅ | ❌ |
| `ENROLLMENT_UPDATE` | ✅ | ✅ | ✅ | ❌ |
| `TEAM_POINTS` / `BUMPER_POINTS` | ✅ | ✅ | ✅ | ❌ |
| `PLAYER_REJECTED` | ❌ | ❌ | ✅ | ❌ |
| `PLAYER_CONNECTED` / `PLAYER_ASSIGNED` | ✅ | ❌ | ✅ | ❌ |
| `MEMORY_SET_TEAMS` / `FLIP_MEMORY_CARD` | ✅ | ✅ | ❌ | ❌ |
| `REMOTE` | ✅ | ✅ | ❌ | ❌ |
| `HELLO` | ❌ | ❌ | ❌ | ✅ |
| `LED_SET` / `OTA_UPDATE` / `WIFI_CONFIG` | ❌ | ❌ | ❌ | ✅ |

---

## Tableau 2 — Payload UPDATE par type de client

| Champ `UPDATE.MSG` | Admin | TV + VPlayer | Buzzer |
|--------------------|-------|-------------|--------|
| `phase` | ✅ | ✅ | ❌ |
| `timer` | ✅ | ✅ | ❌ |
| `question` (texte, TYPE, média, catégorie) | ✅ | ✅ | ❌ |
| `question.QCM_ANSWERS` | ✅ | ✅ | ❌ |
| `question.MEMORY_*` | ✅ | ✅ | ❌ |
| `teams[*].NAME / COLOR / SCORE / STATUS` | ✅ | ✅ | ✅ (toutes équipes) |
| `teams[*].MEMBERS` | ✅ | ✅ | ❌ |
| `bumpers[*].ID / NAME / TEAM / CONNECTED / IS_VIRTUAL / TIME / ANSWER_COLOR` | ✅ | ✅ | ✅ (tous buzzers) |
| `bumpers[*].FIRMWARE_VERSION` | ✅ | ❌ | ❌ |
| `bumpers[*].IS_OUTDATED / OTA_STATUS` | ✅ | ❌ | ❌ |
| `bumpers[*].ACK_PENDING` | ✅ | ❌ | ❌ |
| `GAME.QUIZ_*` sauf `QUIZ_OBJECTIVES` | ✅ | ✅ | ❌ |
| `GAME.QUIZ_HIDDEN_FIELDS` (**v6.1.0**) | ✅ | ✅ — la TV en a besoin pour **appliquer** la préférence | ❌ |
| `GAME.QUIZ_OBJECTIVES` (**v6.1.0**) | ✅ | ❌ | ❌ |
| `config` (paramètres serveur) | ✅ | ❌ | ❌ |
| `history` / `palmares` | ✅ | ✅ | ❌ |
| `remote` | ✅ | ✅ | ❌ |
| `neonEffect` | ✅ | ✅ | ❌ |
| `enrollmentOpen` | ✅ | ✅ | ❌ |

---

## Sérialiseurs à implémenter (backend Go)

### `SerializeForAdmin(msg)` — payload complet (existant, inchangé)

### `SerializeForWebClient(msg)` — TV + VPlayer

Supprimer de `UPDATE.MSG` :
- `bumpers[*].FIRMWARE_VERSION`
- `bumpers[*].IS_OUTDATED`
- `bumpers[*].OTA_STATUS`
- `bumpers[*].ACK_PENDING`
- `config`
- **`GAME.QUIZ_OBJECTIVES` (v6.1.0)** — premier champ du nœud `GAME` à être filtré par type de
  client. Ce n'est pas une optimisation de payload mais une **règle de confidentialité** :
  l'objectif de la partie est une consigne d'animation qui ne doit pas être lisible depuis un
  écran TV ni depuis les outils de développement d'un VJoueur (`game-state.md`,
  § « `QUIZ_OBJECTIVES` — champ à diffusion restreinte »).

> ⚠️ **Trois sites doivent appliquer ce retrait, pas un seul.** Le filtrage des champs
> admin-only est aujourd'hui dupliqué entre `SerializeForWebClient`
> (`internal/protocol/messages.go:560`), `SerializeForVPlayer` (`:625`) et le fan-out chaud
> (`cmd/server/main.go:2718`), qui réimplémente la règle pour éviter un aller-retour
> `map[string]interface{}` par destinataire. La discipline en place pour les champs de bumper —
> **une seule liste exportée et partagée** (`AdminOnlyBumperFields`, `messages.go:542`) — doit
> être reprise à l'identique pour les champs du nœud `GAME` (p. ex. `AdminOnlyGameFields`).
> Un chemin oublié = une fuite silencieuse, invisible en test unitaire du sérialiseur.

### `SerializeForBuzzer(msg)` — buzzers physiques

Conserver uniquement dans `UPDATE.MSG` :
- `bumpers[*]` : `{ID, NAME, TEAM, CONNECTED, IS_VIRTUAL, TIME}` (sans champs firmware/ACK)
- `teams[*]` : `{NAME, COLOR, STATUS}` (sans MEMBERS)

Supprimer entièrement : `question`, `timer`, `phase`, `config`, `history`, `remote`, `neonEffect`, `enrollmentOpen`

---

## Impact réseau estimé

| Type | UPDATE actuel | UPDATE après |
|------|--------------|-------------|
| Admin | ~5 KB | ~5 KB (inchangé) |
| TV / VPlayer | ~5 KB | ~3 KB (−40% : sans firmware/OTA/config) |
| Buzzer | ~5 KB | ~0.5 KB (−90% : bumpers+teams uniquement) |

---

## Implémentation recommandée

Dans `WebSocketHub.BroadcastToTypes()`, passer la sérialisation en paramètre selon le type :

```go
func (h *WebSocketHub) broadcastUpdate(msg *protocol.Message) {
    adminPayload := msg.SerializeForAdmin()
    webPayload   := msg.SerializeForWebClient()

    h.mu.RLock()
    defer h.mu.RUnlock()
    for client := range h.clients {
        switch client.Type {
        case ClientTypeAdmin:
            client.Send <- adminPayload
        case ClientTypeTV, ClientTypeVPlayer:
            client.Send <- webPayload
        }
    }
}
```

Buzzer géré séparément via `BuzzerWebSocketHub.BroadcastIfRelevant()` — voir `buzzer-payload-filter.md`.
