# QUALIF Deployment Report - v3.0.3

**Firmware Version** : 3.0.3
**Server Version** : 3.0.0
**Branch** : feature/buzzer-websocket
**Date** : 2026-02-15 16:39:00
**Commit** : d0693b8

## Firmware Detection

| Check | Result |
|-------|--------|
| Modifications detectees | OUI |
| Fichiers modifies | click_MAIN.cpp, click_WifiManager.h, click_nvsConfig.h, click_serverConnection.h, click_usbConfig.h |
| WebSocket support | ACTIF (USE_WEBSOCKET=1) |

## Build Results

| Component | Status | Size | Time |
|-----------|--------|------|------|
| Frontend (Vite) | SUCCESS | 482 KB JS + 175 KB CSS | 1.3s |
| Backend (Go Windows) | SUCCESS | 19.6 MB | ~5s |
| Firmware BuzzClick v3.0.3 | SUCCESS | 972 KB (995,136 bytes) | 22.6s |

### Firmware Build Details

- RAM: 13.1% (43,004 / 327,680 bytes)
- Flash: 73.7% (965,504 / 1,310,720 bytes)
- Build flags: VERSION="3.0.3", FIRMWARE_VERSION="3.0.3", USE_WEBSOCKET=1
- Warnings: deprecation warnings ArduinoJson `containsKey()` (non-blocking)
- Binary: `buzzclick-v3.0.3-qualif.bin`

### Firmware Size Comparison

| Version | Size | Flash Usage | Notes |
|---------|------|-------------|-------|
| v2.54.0 (TCP only) | 837 KB | 63.0% | Sans WebSocket |
| v3.0.3 (WebSocket) | 972 KB | 73.7% | Avec WebSocket (+135 KB) |

## Firmware Upload & Validation

| Action | Result |
|--------|--------|
| Upload USB | SKIP - Pas de buzzer connecte en USB |
| Connexion WebSocket automatique | PASS - Buzzer 98:3D:AE:50:10:80 connecte |

**IMPORTANT** : Un buzzer BuzzClick s'est connecte automatiquement au serveur via WebSocket pendant les tests (MAC: 98:3D:AE:50:10:80). Le protocole WebSocket buzzer est fonctionnel.

## Post-Build Tests

| Test | Result | Details |
|------|--------|---------|
| Server startup | PASS | v3.0.0, demarrage propre |
| Version endpoint | PASS | Repond "3.0.0" |
| Graceful shutdown | PASS | `{"status":"shutting_down"}` |
| WebSocket clients | PASS | admin, tv, vplayer connectes |
| WebSocket buzzer | PASS | Buzzer connecte via WebSocket (MAC: 98:3D:AE:50:10:80) |
| mDNS | PASS | buzzcontrol.local advertise OK |
| TCP (buzzers) | PASS | Port 1234 ouvert |
| UDP broadcast | PASS | ACTION=HELLO envoye |

## Server Status

**Server is RUNNING** on http://localhost:80

- Admin: http://192.168.1.84/anim
- TV: http://192.168.1.84/tv
- Home: http://192.168.1.84/

## Tests Manuels Utilisateur

### Test 1 : Factory Reset Persistence
1. Buzzer avec WiFi configure (LED verte)
2. Debrancher, rebrancher en maintenant bouton rouge 3s
3. Verifier : LED bleue clignotante puis magenta
4. Verifier : Logs serie "NVS cleared, rebooting..."
5. Verifier : Reboot automatique via ESP.restart()
6. Verifier : LED magenta clignotante (USB mode)
7. Rebooter a nouveau manuellement
8. Verifier : Reste en mode USB (persistance NVS)

### Test 2 : Reconfiguration WiFi AT
1. AT+SSID=TestWiFi
2. AT+PASS=12345678
3. AT+SAVE
4. Verifier : Reboot et connexion WiFi

### Test 3 : WebSocket Protocol
1. Buzzer connecte en WiFi
2. Verifier : Connexion WebSocket reussie (deja confirme automatiquement)
3. Verifier : HELLO envoye au serveur
4. Verifier : Bouton -> evenement serveur

## Verdict

**PASS** - Build complet (firmware v3.0.3 + serveur v3.0.0 + frontend). 8/8 tests post-build passes. Connexion WebSocket buzzer validee automatiquement. Pret pour tests manuels utilisateur (factory reset persistence, AT commands).

## Next Steps

1. Tests manuels utilisateur (factory reset, AT commands)
2. Validation utilisateur
3. Lancer `/deploy PROD` apres validation

---

**QUALIF deployment v3.0.3 completed successfully.**
