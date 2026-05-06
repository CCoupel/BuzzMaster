# Handoff — DEV-BUZZCLICK

**Feature** : v3.8.0 — #54 ACK firmware (ws_sendAck + MSG_ID handler)
**SHA** : 69b4d88
**Branch** : `feature/ws-broadcast-ack-v380`

## Ce qui a été fait

Implémentation complète du protocole ACK côté firmware BuzzClick :
- `ws_sendAck()` ajoutée dans `click_websocket_espidf.h` : envoi non-bloquant (timeout=0) de la réponse ACK au serveur
- Forward declaration et check MSG_ID ajoutés dans `click_serverConnection.h` : dans `parseJSON()`, si `MSG_ID` présent → ACK envoyé AVANT d'appliquer l'action
- Version firmware bumped 3.7.0 → 3.8.0 dans `platformio.ini`

## Décisions clés

**Non-blocking timeout=0** : `ws_sendAck()` est appelée depuis `parseJSON()`, qui s'exécute dans le task ESP-IDF WebSocket interne. Utiliser `portMAX_DELAY` provoquerait un deadlock (le task en attente de lui-même). Le timeout=0 est un drop silencieux si la queue d'envoi est pleine — le serveur gère la non-réception via son `AckManager` (retry/expiry).

**ACK avant apply** : L'ACK est envoyé avant d'appliquer l'action (LED_SET, OTA_UPDATE, WIFI_CONFIG) pour minimiser la latence de confirmation, conformément au contrat.

**Backward compat** : Le check `containsKey("MSG_ID")` est optionnel — les messages sans MSG_ID (serveur v3.7.x ou anciens firmwares) passent sans modification.

**Guard USE_WEBSOCKET** : Le bloc ACK est conditionné par `#ifdef USE_WEBSOCKET` — pas d'impact sur le path TCP legacy.

## Points d'attention

- `ws_sendAck()` utilise `JsonDocument` (ArduinoJson v7 dynamique) — le JSON ACK fait ~100 chars, bien en-dessous des limites mémoire
- Build PlatformIO non exécutable en environnement local (pas de toolchain ESP32 disponible) — le build de validation se fait via CI/CD GitHub Actions au tag
- La version 3.8.0 côté firmware correspond maintenant à la version serveur Go 3.8.0

## Fichiers modifiés

- `src/BuzzClick/click_websocket_espidf.h` — ajout `ws_sendAck()`
- `src/BuzzClick/click_serverConnection.h` — forward decl + MSG_ID handler dans `parseJSON()`
- `platformio.ini` — VERSION + FIRMWARE_VERSION 3.7.0 → 3.8.0
