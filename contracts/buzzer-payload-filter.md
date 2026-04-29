# Contrat Filtrage Payload Buzzers (v3.8.0 — révisé QUALIF)

> **Feature** : #41 — Réduire payload WebSocket buzzers  
> **Branche** : `feature/ws-broadcast-ack-v380`  
> **Dernière mise à jour** : 2026-04-28 (révisé après analyse firmware QUALIF)

---

## Problème

Le firmware BuzzClick traite encore de la logique locale sur plusieurs messages (rotation grise,
assignation d'équipe, keepalive PONG). La whitelist initiale `{LED_SET, OTA_UPDATE, WIFI_CONFIG, HELLO}`
était trop restrictive — elle cassait ces comportements.

---

## Whitelist révisée — messages transmis aux buzzers

> **Règle** : tous les messages de la whitelist ci-dessous sont envoyés. Tous les autres sont droppés silencieusement.

| Action | Raison |
|--------|--------|
| `UPDATE` / `UPDATE_TIMER` | `handleUpdateAction()` — lit TEAM assignée + gère rotation grise |
| `START` / `CONTINUE` | `startGame()` |
| `STOP` | `stopGame()` |
| `PAUSE` | `pauseGame()` |
| `READY` | `handleReadyAction()` — lit TEAM assignée + gère rotation grise |
| `RESET` | `resetGame()` |
| `HELLO` | Trigger reconnexion WebSocket |
| `LED_SET` | Contrôle LED (source unique de vérité depuis v3.4.0) |
| `OTA_UPDATE` | Mise à jour firmware |
| `WIFI_CONFIG` | Sync identifiants WiFi |

> **Note PING** : géré au niveau WS (control frame), pas via JSON ACTION — non concerné par la whitelist.

---

## Messages **non** transmis aux buzzers

| Action | Raison |
|--------|--------|
| `QUESTIONS` | Grande payload (~5-50 KB), inutile firmware |
| `CLIENTS` | Liste clients web, inutile firmware |
| `FIRMWARE_VERSION` | Info OTA pour frontend uniquement |
| `BACKGROUND_CHANGE` | Fond d'écran, inutile firmware |
| `QCM_HINT` | Hint QCM, inutile firmware |
| `CONFIG_UPDATE` | Config serveur, inutile firmware |
| `ENROLLMENT_UPDATE` | QR code enrollment, inutile firmware |
| `TEAM_POINTS` / `BUMPER_POINTS` | Animations score, inutile firmware |
| `PLAYER_REJECTED` | VJoueur uniquement |
| `REVEAL` | Résultat QCM, inutile firmware (LED_SET couvre l'état) |
| `REMOTE` | Commande remote admin, inutile firmware |
| `DELETE_BUMPER` | Admin uniquement |

---

## Payload réduit pour UPDATE vers buzzers

Le message UPDATE envoyé aux buzzers doit être sérialisé avec `SerializeForBuzzer()` :

### Champs conservés

```json
{
  "ACTION": "UPDATE",
  "MSG": {
    "bumpers": {
      "<MAC_du_buzzer>": {
        "ID": "...",
        "TEAM": "...",
        "NAME": "...",
        "CONNECTED": true,
        "IS_VIRTUAL": false
      }
    },
    "teams": {
      "<nom_équipe>": {
        "NAME": "...",
        "COLOR": [r, g, b],
        "STATUS": "..."
      }
    }
  }
}
```

### Champs supprimés

- `bumpers[*].FIRMWARE_VERSION` / `IS_OUTDATED` / `OTA_STATUS` / `ACK_PENDING`
- `bumpers` autres que le buzzer destinataire (idéalement)
- `teams[*].MEMBERS` (liste joueurs)
- `teams` autres que l'équipe du buzzer (idéalement)
- `question`, `timer`, `phase`, `config`, `history`

> **Note implémentation** : Le filtrage par MAC spécifique nécessite une sérialisation per-buzzer.
> Solution pragmatique : envoyer tous les `bumpers` (sans champs firmware) et toutes les `teams` (sans MEMBERS).
> Gain : suppression de question (~1-5 KB), config, history, MEMBERS.

---

## Implémentation

### `BuzzerWebSocketHub.BroadcastIfRelevant(msg)`

```go
var buzzerActionWhitelist = map[string]bool{
    "UPDATE":        true,
    "UPDATE_TIMER":  true,
    "START":         true,
    "CONTINUE":      true,
    "STOP":          true,
    "PAUSE":         true,
    "READY":         true,
    "RESET":         true,
    "HELLO":         true,
    protocol.ActionLEDSet:     true,
    protocol.ActionOTAUpdate:  true,
    protocol.ActionWifiConfig: true,
}

func (h *BuzzerWebSocketHub) BroadcastIfRelevant(msg *protocol.Message) {
    if !buzzerActionWhitelist[msg.Action] {
        return
    }
    if msg.Action == "UPDATE" || msg.Action == "UPDATE_TIMER" {
        h.Broadcast(msg.SerializeForBuzzer())
        return
    }
    h.Broadcast(msg)
}
```

### `protocol.Message.SerializeForBuzzer()`

Sérialise UPDATE avec payload réduit :
- `bumpers` : tous les buzzers, sans `FIRMWARE_VERSION`, `IS_OUTDATED`, `OTA_STATUS`, `ACK_PENDING`
- `teams` : toutes les équipes, sans `MEMBERS`
- Supprime : `question`, `timer`, `phase`, `config`, `history`, `remote`
