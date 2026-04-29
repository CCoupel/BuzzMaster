# Changelog des Contrats API

---

## [20260428] — v3.8.0 : WebSocket endpoints dédiés + ACK buzzer + payload buzzer

### Contrats créés

- **[NEW]** `contracts/websocket-endpoints.md` — Endpoints WS dédiés `/ws/admin`, `/ws/tv`, `/ws/player` avec table de filtrage des messages
- **[NEW]** `contracts/buzzer-ack-protocol.md` — Protocole `MSG_ID` / `ACK` / retry pour buzzers physiques
- **[NEW]** `contracts/buzzer-payload-filter.md` — Whitelist des actions autorisées vers buzzers

### Changements de l'API WebSocket

- **[NEW]** `/ws/admin` — nouvel endpoint WebSocket pour l'interface admin
- **[NEW]** `/ws/tv` — nouvel endpoint WebSocket pour l'affichage TV
- **[NEW]** `/ws/player` — nouvel endpoint WebSocket pour les VJoueurs
- **[CHANGED]** `/ws` — conservé comme alias vers `/ws/admin` (rétrocompatible, non BREAKING)

### Changements de protocole buzzer

- **[NEW]** `Message.MSG_ID` (string, optionnel, omitempty) — identifiant ACK sur LED_SET, OTA_UPDATE, WIFI_CONFIG
- **[NEW]** `ACTION: "ACK"` — nouvelle action buzzer→serveur pour accusé de réception
- **[NEW]** `Bumper.ACK_PENDING` (bool, omitempty) — champ frontend pour badge ⚠

### Changements de configuration

- **[NEW]** `ServerConfig.ack_timeout_ms` (int, défaut 2000) — timeout avant retry ACK
- **[NEW]** `ServerConfig.ack_max_retries` (int, défaut 3) — max retentatives ACK

### Notes de compatibilité

- **Rétrocompatibilité frontend** : `/ws` reste fonctionnel — pas de BREAKING pour les déploiements existants
- **Rétrocompatibilité firmware** : `MSG_ID` est optionnel — les firmwares < v3.8.0 sans support ACK fonctionnent normalement (retry côté serveur puis abandon sans ACK)
- **Rétrocompatibilité protocole TCP** : aucun changement sur le protocole TCP v1 (port 1234)
- **Filtrage payload** : suppression de `UPDATE` vers `buzzerHub` est transparent côté firmware (n'utilisait pas ce message depuis v3.4.0)
