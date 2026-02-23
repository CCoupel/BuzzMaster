# Project Patterns — BuzzControl

## TV Display — CRITIQUE

`/tv` (PlayerDisplay.jsx) est **STATIQUE**, zéro scroll autorisé.

- `overflow: hidden` (jamais `auto` ou `scroll`)
- Unités `vh`, `vw`, `%` (jamais `px` fixes)
- `flex` + `min-height: 0` pour rétrécissement
- Contenu limité : top 3, max 6 catégories
- Fichier : `web/src/pages/PlayerDisplay.jsx`

## Redémarrage Serveur

```bash
# ✅ TOUJOURS
curl -s http://localhost/shutdown && sleep 2 && ./server.exe

# ❌ JAMAIS
kill -9 $(pgrep server)
```

Shutdown API : sauvegarde état, ferme WS/TCP proprement, libère port 80.

## Build — Ordre obligatoire

**Frontend AVANT Backend** (frontend embarqué dans le binaire Go)

```bash
cd server-go/web && npm run build   # 1. Frontend
cd .. && go build -o server.exe ./cmd/server  # 2. Backend
```

## Versioning

- Z=bugfix (2.40.0 → 2.40.1)
- Y=feature (2.40.1 → 2.41.0)
- X=breaking (2.41.0 → 3.0.0)
- `server-go/config.json` = source de vérité
- CI injecte version dans `platformio.ini` au push du tag `vX.Y.0`
- `server-go/web/package.json` doit être synchronisé avec config.json

## Architecture Serveur

- **100% Go** — pas de Python, Node.js, dépendances externes
- Protocoles natifs : TCP, WebSocket, HTTP, UDP, DNS
- Ports : HTTP/WS=80, TCP/UDP=1234

## Mode Hybride TCP + WebSocket (v3.0.0+)

| Protocol | Endpoint | Version |
|----------|----------|---------|
| TCP | port 1234 | v1.x (rétrocompatible) |
| WebSocket | `/ws/buzzer` port 80 | v3.0.0+ |
| UDP | port 1234 | Broadcast discovery |

## Firmware BuzzClick (ESP32-C3)

### Configuration USB (v3.0.0+)
Commandes AT série à 115200 baud :
```
AT+WIFI_SSID=xxx
AT+WIFI_PASS=xxx
AT+SERVER_IP=192.168.1.100
AT+SERVER_PORT=80
AT+SAVE
AT+RESTART
```

### Hardware
- Factory reset : GPIO 6 (bouton rouge) maintenu 3s au boot
- LED : Orange = mode config USB, Vert = WiFi connecté
- NVS : `wifi_ssid`, `wifi_password`, `server_ip`, `server_tcp_port`

### Déploiement QUALIF avec firmware modifié

Si `src/BuzzClick/` a été modifié :

```bash
# 1. Détecter
git diff main --name-only | grep "^src/BuzzClick/"

# 2. Injecter version
VERSION=$(grep '"version"' server-go/config.json | sed 's/.*: *"\([^"]*\)".*/\1/')
cd src/BuzzClick
sed -i "s/^build_flags = .*/build_flags = -DFIRMWARE_VERSION=\\\"$VERSION\\\"/" platformio.ini

# 3. Build
pio run

# 4. Valider
strings .pio/build/esp32-c3-devkitm-1/firmware.bin | grep "$VERSION"
# Taille : 200KB < size < 2MB, RAM < 80%, Flash < 80%

# 5. Sauvegarder
cp .pio/build/esp32-c3-devkitm-1/firmware.bin ../../buzzclick-v${VERSION}-qualif.bin
```

## Web Serial API

- Nécessite **localhost** (pas d'IP locale en HTTP)
- Chrome/Edge 89+ uniquement
- Contexte sécurisé requis (HTTPS ou localhost)
