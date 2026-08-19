# QUALIF Deployment Report

**Version** : 3.0.0
**Branch** : feature/buzzer-websocket
**Date** : 2026-02-15 11:06:19
**Commit** : edc7a81

## Firmware Detection

| Check | Result |
|-------|--------|
| Modifications detected | Yes |
| Files modified | click_websocketClient.h (NEW), click_serverConnection.h, click_WifiManager.h, click_MAIN.cpp, click_includes.h, platformio.ini |

## Build Results

| Component | Status | Size | Time |
|-----------|--------|------|------|
| Frontend (Vite) | OK | 482KB JS + 175KB CSS | 1.2s |
| Backend (Go Windows) | OK | 19.6 MB | ~10s |
| Firmware BuzzClick | OK | 823 KB | 22s |

### Firmware Resource Usage

| Resource | Usage | Details |
|----------|-------|---------|
| RAM | 12.9% | 42,188 / 327,680 bytes |
| Flash | 62.0% | 813,044 / 1,310,720 bytes |

### Firmware Build Warnings

- ArduinoJson `containsKey()` deprecated warnings (non-blocking, cosmetic only)
- `CONFIG_ESP_TASK_WDT_TIMEOUT_S` redefined warnings (SDK vs build flag, non-blocking)

## Firmware Upload & Validation

Firmware binary available at `buzzclick-v3.0.0-qualif.bin` (823 KB).
User must flash manually on 1 test buzzer via USB for validation.

## Post-Build Tests

| Test | Result | Details |
|------|--------|---------|
| Server startup | PASS | Server boots in ~1s, all services initialized |
| Version endpoint | PASS | `GET /version` returns `3.0.0` |
| WebSocket buzzer endpoint | PASS | `GET /ws/buzzer` returns HTTP 400 (expects WS upgrade) |
| TCP buzzer retrocompat | PASS | TCP connections from 192.168.1.57 accepted |
| HTTP port 80 | PASS | Web interface accessible |
| TCP port 1234 | PASS | Buzzer TCP server started |
| UDP port 1234 | PASS | UDP broadcaster active |
| mDNS | PASS | buzzcontrol.local advertised |
| WebSocket clients | PASS | Admin, TV, VPlayer clients connected |

## Server Status

**Server is RUNNING** on http://localhost:80 (http://192.168.1.84)

### Services Active

| Service | Port | Status |
|---------|------|--------|
| HTTP | 80 | Running |
| TCP (buzzers) | 1234 | Running |
| UDP (broadcast) | 1234 | Running |
| DNS (captive portal) | 53 | Running |
| mDNS | - | Advertising buzzcontrol.local |
| WebSocket /ws | - | Active (admin, TV, VPlayer) |
| WebSocket /ws/buzzer | - | Active (new buzzer endpoint) |

## Server Logs Summary

```
Version: 3.0.0 (embedded: dev)
HTTP Port: 80
TCP Port: 1234
Teams loaded: 6 teams
Bumpers loaded: 12 bumpers
History loaded: 34 events
BuzzControl server v3.0.0 started successfully
TCP server (buzzers): port 1234
```

## Hybrid Mode Verification

The server supports both connection protocols simultaneously:
- **TCP** (port 1234): Legacy buzzers connect via TCP (verified: 192.168.1.57 connected)
- **WebSocket** (`/ws/buzzer`): New buzzers can connect via WebSocket (endpoint active, returns 400 on HTTP - correct behavior, requires WS upgrade)

## Instructions for Manual Testing

1. **Web interface**: Open http://localhost or http://192.168.1.84
2. **Admin panel**: http://192.168.1.84/admin/game
3. **TV display**: http://192.168.1.84/tv
4. **TCP buzzer test**: Existing buzzers should connect automatically
5. **WebSocket buzzer test**: Flash `buzzclick-v3.0.0-qualif.bin` (823 KB) on 1 test buzzer via USB
6. **Hybrid mode**: Both TCP and WebSocket buzzers should coexist

## Next Steps

1. Tests manuels utilisateur (UI, workflow, jeu complet)
2. Verifier retrocompatibilite buzzer TCP ancien (connected: OK)
3. Tester connexion buzzer WebSocket nouveau (flash firmware sur 1 buzzer)
4. Validation utilisateur avant PROD
5. Lancer `/deploy PROD` apres validation

---

**QUALIF deployment completed successfully.**

**Binaries**:
- Server: `server-go/buzzcontrol-v3.0.0-qualif.exe` (19.6 MB)
- Firmware: `buzzclick-v3.0.0-qualif.bin` (823 KB)
