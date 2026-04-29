# Contrat Protocole ACK Buzzer (v3.8.0)

> **Feature** : #54 — ACK buzzer + retry serveur
> **Branche** : `feature/ws-broadcast-ack-v380`
> **Dernière mise à jour** : 2026-04-28

---

## Vue d'ensemble

Le serveur ajoute un identifiant optionnel `MSG_ID` sur les messages prioritaires envoyés aux buzzers.
Le firmware répond avec une action `ACK`. Si l'ACK n'est pas reçu dans le délai configuré, le serveur
retente l'envoi (jusqu'à `ack_max_retries` fois).

**Compatibilité backward** : `MSG_ID` est optionnel (`omitempty`). Un firmware sans support ACK
ignore simplement le champ — comportement identique à v3.7.0.

---

## Messages prioritaires avec MSG_ID

Le serveur ajoute `MSG_ID` aux actions suivantes (server → buzzer) :

| Action | Priorité | Raison |
|--------|----------|--------|
| `LED_SET` | Haute | État LED visible par les joueurs |
| `OTA_UPDATE` | Critique | Mise à jour firmware |
| `WIFI_CONFIG` | Haute | Sync identifiants réseau |

---

## Format des messages

### Server → Buzzer (avec MSG_ID)

Champ `MSG_ID` ajouté au niveau racine du message JSON :

```json
{
  "ACTION": "LED_SET",
  "MSG_ID": "abc123def456",
  "MSG": {
    "COLOR": [255, 0, 0],
    "INTENSITY": 255,
    "EFFECT": "SOLID"
  }
}
```

**Type** : `MSG_ID` est une chaîne hexadécimale 12 caractères (6 octets aléatoires).
**Position** : champ racine du message (`Message` struct), pas dans `MSG`.

---

### Buzzer → Server (ACK)

```json
{
  "ID": "AA:BB:CC:DD:EE:FF",
  "ACTION": "ACK",
  "MSG": {
    "ack_action": "LED_SET",
    "ack_id": "abc123def456"
  }
}
```

| Champ | Type | Description |
|-------|------|-------------|
| `ID` | string | MAC du buzzer |
| `ACTION` | string | `"ACK"` |
| `MSG.ack_action` | string | Action accusée (`LED_SET`, `OTA_UPDATE`, `WIFI_CONFIG`) |
| `MSG.ack_id` | string | Valeur du `MSG_ID` reçu |

---

## Modèle Bumper — nouveau champ

```go
// models.go — ajout dans Bumper
AckPending bool `json:"ACK_PENDING,omitempty"` // true si un ACK est attendu (badge frontend)
```

**Comportement** :
- `true` : le serveur a envoyé un message prioritaire et attend le ACK
- `false` (ou absent) : état normal

**Serialisation** : `omitempty` — pas transmis quand `false` (rétrocompatibilité frontend)

---

## Logique serveur — AckManager

### Lifecycle d'un ACK

```
sendLEDSet(mac, payload)
    │
    ├── génère MSG_ID (hex aléatoire 12 chars)
    ├── envoie message avec MSG_ID au buzzer
    ├── met AckPending = true sur Bumper
    ├── broadcastUpdate() (badge frontend visible)
    └── enregistre dans AckManager {mac, msgId, action, payload, attempts=0}

AckManager goroutine (tick = ack_timeout_ms)
    ├── pour chaque entrée pending :
    │   ├── si ACK reçu → supprimer, AckPending=false, broadcast
    │   ├── si timeout et attempts < max_retries → retenter (sendMessage + attempts++)
    │   └── si attempts >= max_retries → abandonner, AckPending=false, log WARN, broadcast
    └── nettoyage des entrées périmées (buzzer déconnecté)
```

### Configuration (config.json)

Nouveaux champs dans `server` :

```json
{
  "server": {
    "ack_timeout_ms": 2000,
    "ack_max_retries": 3
  }
}
```

| Champ | Type | Défaut | Description |
|-------|------|--------|-------------|
| `ack_timeout_ms` | int | `2000` | Délai avant retry (ms) |
| `ack_max_retries` | int | `3` | Nombre max de retentatives avant abandon |

---

## Frontend — Badge ACK_PENDING

Badge ⚠ style identique au badge déconnexion (`CONNECTED=false`) :
- Fond : `background: #f59e0b`
- Icône : SVG `clock` ou `alert-circle` stroke:white
- Tooltip : `"ACK en attente"` ou `"Buzzer ne répond pas"`

**Pages concernées** :
- `TeamCard.jsx` — badge inline dans la row du buzzer
- `TeamsPage.jsx` — badge dans rows membres et buzzers non assignés

**Condition d'affichage** : `!bumper.IS_VIRTUAL && !bumper.IS_VPLAYER && bumper.ACK_PENDING === true`

---

## Firmware — Implémentation

### `ws_sendAck()` dans `click_websocket_espidf.h`

```cpp
// Envoie un ACK au serveur pour le MSG_ID reçu
void ws_sendAck(const String& mac, const String& ackAction, const String& ackId) {
    // build JSON et ws_send()
}
```

### Handler dans `click_serverConnection.h`

Dans `parseJSON()`, pour chaque message reçu :

```cpp
// Si MSG_ID présent → envoyer ACK
if (doc.containsKey("MSG_ID")) {
    String msgId = doc["MSG_ID"].as<String>();
    String action = doc["ACTION"].as<String>();
    ws_sendAck(myMac, action, msgId);
}
```

**Règle** : L'ACK est envoyé immédiatement à la réception du message, AVANT d'appliquer l'action.

---

## Flux complet (diagramme)

```
Serveur                        Firmware
   │                               │
   │── LED_SET (MSG_ID=abc) ──────►│
   │   AckPending=true             │── ACK (ack_id=abc) ──────►│
   │   badge frontend ⚠            │                            │
   │                               │   applique LED
   │◄── ACK (ack_id=abc) ──────────│
   │   AckPending=false            │
   │   broadcast UPDATE            │
   │   badge frontend disparaît    │
```

```
Scénario retry (firmware ancien sans ACK) :
   │── LED_SET (MSG_ID=xyz) ──────►│  (pas de ACK)
   │   [attente ack_timeout_ms]    │
   │── LED_SET retry 1 ───────────►│  (pas de ACK)
   │   [attente ack_timeout_ms]    │
   │── LED_SET retry 2 ───────────►│  (pas de ACK)
   │   [attente ack_timeout_ms]    │
   │── LED_SET retry 3 ───────────►│  (pas de ACK)
   │   AckPending=false            │
   │   log WARN "ACK timeout"      │
   │   broadcast UPDATE            │
```
