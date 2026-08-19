# QUALIF Deployment Report - v3.0.5

**Firmware Version** : 3.0.5
**Server Version** : 3.0.0 (inchange)
**Branch** : feature/buzzer-websocket
**Date** : 2026-02-15 17:08:00
**Commit** : ef42fc7

## Change in v3.0.5

**Fix critique** : Fragmentation WebSocket avec comptage d'accolades JSON.
Remplace la detection `\0` par un comptage `{`/`}` pour determiner quand un message JSON est complet. Empeche le buffer de grandir indefiniment sur messages fragmentes.

## Build

| Component | Status | Size | Time |
|-----------|--------|------|------|
| Firmware BuzzClick v3.0.5 | SUCCESS | 974 KB (996,896 bytes) | 22.4s |
| Frontend | SKIP (inchange) | - | - |
| Backend | SKIP (inchange) | - | - |

- RAM: 13.1% (43,012 / 327,680 bytes)
- Flash: 73.8% (966,916 / 1,310,720 bytes)

## Flash

| Detail | Value |
|--------|-------|
| Port | COM5 |
| Chip | ESP32-C3 (rev v0.4) |
| MAC | 98:3D:AE:50:10:80 |
| Upload | SUCCESS en 13.74s |
| Hash | Verifie sur 4 partitions |

## Post-Flash Verification

| Check | Result | Details |
|-------|--------|---------|
| Boot | PASS | Demarrage propre, pas de panic |
| WiFi | PASS | Connecte IP 192.168.1.57 |
| WebSocket connexion | PASS | Connexion automatique au serveur |
| Buzzer identifie | PASS | MAC 98:3D:AE:50:10:80, equipe "Les Rouges" |
| Memoire stable | PASS | 199,784 bytes (constante, pas de fuite) |
| Watchdog | PASS | Actif, pas de reboot |

## Fragmentation Analysis

| Metric | v3.0.4 | v3.0.5 | Verdict |
|--------|--------|--------|---------|
| Free memory (idle) | 197,932 bytes | 199,784 bytes | +1,852 bytes (+0.9%) |
| Memory stability | Stable | Stable | OK |
| Buffer growth | N/A (idle) | N/A (idle) | Tests manuels requis avec jeu actif |

**Note** : La memoire libre est PLUS haute en v3.0.5 (+1,852 bytes), suggerant une meilleure gestion du buffer WebSocket. Tests avec jeu actif (messages longs UPDATE/READY) requis pour validation complete de la fragmentation.

## Server Status

**Server RUNNING** on http://localhost:80
Buzzer v3.0.5 connecte via WebSocket (equipe "Les Rouges")

## Tests Manuels Requis

1. Demarrer un jeu avec 6 equipes et plusieurs joueurs (messages UPDATE longs)
2. Observer logs serie : "Received complete message: XXX bytes" doit apparaitre
3. Verifier que la memoire reste stable pendant le jeu
4. Verifier action READY recue sans erreur
5. LED grise tournante si buzzer retire d'une equipe

## Verdict

**PASS** - Build OK, flash OK, buzzer connecte via WebSocket, memoire stable (+1.8 KB vs v3.0.4). Pret pour tests manuels de fragmentation avec jeu actif.

---

**QUALIF deployment v3.0.5 completed successfully.**
