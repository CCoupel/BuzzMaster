# QUALIF Deployment Report - v3.0.4

**Firmware Version** : 3.0.4
**Server Version** : 3.0.0 (inchange)
**Branch** : feature/buzzer-websocket
**Date** : 2026-02-15 16:57:00
**Commit** : 319d85f

## Changes in v3.0.4 (vs v3.0.3)

1. **Fix fragmentation WebSocket** : Detection `\0` null-terminator dans messages fragmentes
2. **Support action READY** : Buzzer recoit et traite action READY sans erreur
3. **LED grise tournante** : Animation LED grise si buzzer pas assigne a une equipe

## Build Results

| Component | Status | Size | Time |
|-----------|--------|------|------|
| Frontend (Vite) | SKIP | Inchange depuis v3.0.3 | - |
| Backend (Go Windows) | SKIP | Inchange depuis v3.0.3 | - |
| Firmware BuzzClick v3.0.4 | SUCCESS | 974 KB (996,800 bytes) | 23.9s |

### Firmware Build Details

- RAM: 13.1% (43,012 / 327,680 bytes)
- Flash: 73.8% (966,834 / 1,310,720 bytes)
- Build flags: VERSION="3.0.4", FIRMWARE_VERSION="3.0.4", USE_WEBSOCKET=1
- Binary: `buzzclick-v3.0.4-qualif.bin`

### Firmware Size Comparison

| Version | Size | Flash | Delta |
|---------|------|-------|-------|
| v2.54.0 (TCP only) | 837 KB | 63.0% | baseline |
| v3.0.3 (WebSocket) | 972 KB | 73.7% | +135 KB |
| v3.0.4 (WS+fixes) | 974 KB | 73.8% | +1.6 KB vs v3.0.3 |

## Post-Build Tests

| Test | Result | Details |
|------|--------|---------|
| Server startup | PASS | v3.0.0, demarrage propre |
| Version endpoint | PASS | Repond "3.0.0" |
| WebSocket clients | PASS | admin, tv, vplayer connectes |
| mDNS | PASS | buzzcontrol.local OK |
| TCP port 1234 | PASS | Ouvert |
| UDP broadcast | PASS | ACTION=HELLO envoye |

## Server Status

**Server is RUNNING** on http://localhost:80

- Admin: http://192.168.1.84/anim
- TV: http://192.168.1.84/tv
- Home: http://192.168.1.84/

## Tests Manuels Utilisateur (v3.0.4 specifiques)

### Test 1 : Fragmentation WebSocket
1. Connecter buzzer en WiFi
2. Observer logs serie (115200 baud)
3. Verifier : Messages fragmentes reassembles correctement
4. Verifier : Pas de crash/erreur sur messages longs

### Test 2 : Action READY
1. Buzzer connecte au serveur
2. Serveur envoie action READY
3. Verifier : Buzzer traite READY sans erreur dans logs serie
4. Verifier : Pas de "Unknown action" dans logs

### Test 3 : LED Grise Tournante
1. Buzzer connecte mais PAS assigne a une equipe
2. Verifier : LED affiche animation grise tournante
3. Assigner buzzer a une equipe
4. Verifier : LED passe a la couleur de l'equipe

### Test 4 : Factory Reset Persistence (regression)
1. Factory reset (bouton rouge 3s au boot)
2. Verifier : NVS efface, reboot, mode USB persiste

## Verdict

**PASS** - Firmware v3.0.4 build OK, serveur v3.0.0 running, tests automatiques passes. Pret pour tests manuels utilisateur (fragmentation WS, READY action, LED grise).

## Next Steps

1. Tests manuels utilisateur (features v3.0.4)
2. Validation utilisateur
3. Lancer `/deploy PROD` apres validation

---

**QUALIF deployment v3.0.4 completed successfully.**
