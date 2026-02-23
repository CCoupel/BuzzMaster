# QUALIF Deployment Report

**Version Serveur** : 2.55.0
**Version Firmware** : 2.54.0 (build flag dans platformio.ini)
**Branch** : bugfix/wifi-usb-mode
**Date** : 2026-02-15 16:30:00
**Commit** : d38ad5d

## Note sur la version

Le team lead a demande un deploiement firmware v3.0.3. Cependant, la version dans le code source (`platformio.ini`) est **2.54.0** et la version serveur (`config.json`) est **2.55.0**. Le deploiement a ete effectue avec les versions reelles du code.

## Firmware Detection

| Check | Result |
|-------|--------|
| Modifications detectees | OUI |
| Fichiers modifies | click_MAIN.cpp, click_WifiManager.h, click_nvsConfig.h, click_serverConnection.h, click_usbConfig.h |

## Build Results

| Component | Status | Size | Time |
|-----------|--------|------|------|
| Frontend (npm/vite) | SUCCESS | 496 KB JS + 184 KB CSS | 2.32s |
| Backend (Go Windows) | SUCCESS | 19.6 MB | ~5s |
| Firmware BuzzClick (ESP32-C3) | SUCCESS | 837 KB (856,848 bytes) | 23.4s |

### Firmware Build Details

- RAM: 12.9% (42,388 / 327,680 bytes)
- Flash: 63.0% (825,980 / 1,310,720 bytes)
- Warnings: deprecation warnings on ArduinoJson `containsKey()` (non-blocking)
- Binary: `buzzclick-v2.54.0-qualif.bin`

## Firmware Upload & Validation

| Action | Result |
|--------|--------|
| Upload USB | SKIP - Hardware non connecte |
| Buzzer restart | SKIP |
| Connexion serveur | SKIP |
| Tests fonctionnels | SKIP |

**Note** : Aucun buzzer BuzzClick connecte en USB. Tests hardware requis manuellement.

## Post-Build Tests

| Test | Result | Details |
|------|--------|---------|
| Server startup | PASS | Demarre sans erreur, logs propres |
| Version endpoint | PASS | Repond "2.55.0" (correct) |
| Graceful shutdown | PASS | Arret propre, `{"status":"shutting_down"}` |
| WebSocket | PASS | Clients admin, tv, vplayer connectes |
| mDNS | PASS | buzzcontrol.local advertise OK |
| TCP (buzzers) | PASS | Port 1234 ouvert |
| UDP broadcast | PASS | ACTION=HELLO envoye |

## Server Status

**Server is RUNNING** on http://localhost:80

- Admin: http://192.168.1.84/anim
- TV: http://192.168.1.84/tv
- Home: http://192.168.1.84/

## Firmware Tests Manuels (a effectuer par l'utilisateur)

### Test 1 : Factory Reset Persistence
1. Buzzer avec WiFi configure (LED verte)
2. Debrancher, rebrancher en maintenant bouton rouge 3s
3. Verifier : LED bleue clignotante puis magenta
4. Verifier : Logs serie "NVS cleared, rebooting..."
5. Verifier : Reboot automatique
6. Verifier : LED magenta clignotante (USB mode)
7. Rebooter a nouveau manuellement
8. Verifier : Reste en mode USB (persistance)

### Test 2 : Reconfiguration WiFi
1. AT+SSID=TestWiFi
2. AT+PASS=12345678
3. AT+SAVE
4. Verifier : Reboot et connexion WiFi

### Test 3 : WebSocket (si serveur actif)
1. Buzzer connecte en WiFi
2. Verifier : Connexion WebSocket reussie
3. Verifier : HELLO envoye au serveur
4. Verifier : Bouton -> LED serveur

## Verdict

**PARTIAL** - Build valide (firmware + serveur + frontend). Tests post-build serveur tous passes (7/7). Tests hardware firmware requis pour validation complete avant PROD.

## Next Steps

1. Tests manuels utilisateur (firmware sur hardware reel)
2. Validation utilisateur avant PROD
3. Lancer `/deploy PROD` apres validation

---

**QUALIF deployment completed successfully (build + server tests).**
**Hardware firmware tests pending user validation.**
