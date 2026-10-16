---
name: dev-firmware
description: "Adaptations projet BuzzControl pour l'agent dev-firmware (firmware BuzzClick ESP32-C3). Base generique : dev-firmware.template.md."
model: sonnet
color: yellow
---

# Dev Firmware — Adaptations BuzzControl (BuzzClick ESP32-C3)

> **Base** : voir `dev-firmware.template.md` pour le cadre générique (mode IDLE, watchdog, `IRAM_ATTR`,
> commandes PlatformIO, checklist). Ce compagnon ne contient que ce qui est propre à BuzzClick.
> Issu de l'ancien agent maison `dev-buzzclick` (remplacé le 2026-10-16 par l'agent standard `dev-firmware`).

## Perimetre

- `src/BuzzClick/**` — firmware des buzzers
- `src/Common/**` — code partagé firmware (logger, LED, config)
- `platformio.ini`

Le serveur Go (`server-go/`) est hors périmètre : toute évolution du protocole passe par `dev-backend`
et par `contracts/` (contract-first), le serveur **en premier**, le firmware ensuite.

## Architecture BuzzClick

```
src/BuzzClick/
├── click_MAIN.cpp              # setup + loop, orchestration
├── click_includes.h            # Includes & declarations globales
├── click_serverConnection.h    # Handlers des messages serveur
├── click_websocket_espidf.h    # Client WebSocket ESP-IDF (v3.0.0+)
└── click_WifiManager.h         # WiFi configurable via USB + chaine fallback UDP

src/Common/
├── Constant.h                  # Utilitaires (hash)
├── CustomLogger.h              # Logging UDP avec couleurs ANSI
├── led.h                       # Controle NeoPixels/DotStar
└── configManager.h             # Configuration LittleFS
```

## Hardware ESP32-C3

| Fonction | Pin | Type |
|----------|-----|------|
| Bouton ROUGE | 6 | INPUT_PULLUP |
| Bouton VERT | 7 | INPUT_PULLUP |
| Bouton BLEU | 8 | INPUT_PULLUP |
| Bouton JAUNE | 9 | INPUT_PULLUP |
| LED Data (DotStar) | 4 | SPI |
| LED Clock (DotStar) | 3 | SPI |

LEDs : 23 pixels APA102 DotStar. Les LED sont **pilotées par le serveur** depuis v3.4.0
(`docs/LED_SET_PROTOCOL.md` : LED_SET, COMET v3.7.0, ACK v3.8.0) — ne pas réintroduire d'animations
locales par phase sans décision explicite.

## Protocole — sources de vérité

Le protocole n'est **pas** décrit ici : lire les documents du projet avant d'implémenter.

| Sujet | Document |
|-------|----------|
| WebSocket `/ws/buzzer` (canal principal depuis v3.0.0) | `docs/WEBSOCKET_PROTOCOL.md` |
| TCP legacy, HTTP, UDP broadcast (port 1234), OTA, ACK | `docs/PROTOCOLS.md` |
| LED server-driven | `docs/LED_SET_PROTOCOL.md` |
| Contrats d'actions et modèles (Bumper) | `contracts/` (voir `contracts/README.md`) |
| Mise à jour firmware / OTA | `docs/FIRMWARE_UPDATE.md` |

Règles figées (CLAUDE.md « Implementation Rules ») :
- Messages JSON **null-terminated** (`\0`) — compat v1 conservée
- Timings d'appui bouton en **microsecondes**
- L'adresse **MAC** est l'ID unique du buzzer
- Le serveur détecte la version du client et s'adapte : ne jamais casser un déploiement existant sans migration

## Contraintes BuzzClick

- Watchdog **30 s** : `esp_task_wdt_reset()` dans toute boucle longue ; `loop()` ne bloque jamais plus de quelques ms
- RAM utile ~160 KB : ArduinoJson ~2-3 KB par deserialize, buffer UDP 8192 max, pas de concaténations `String` en boucle
- Reconnexion WiFi : backoff exponentiel, timeout < 30 s (watchdog), jamais d'`ESP.restart()` brutal
- Ne jamais changer les pins sans mettre à jour la doc hardware

## Versioning et build

La version firmware vit dans `platformio.ini` (`build_flags -D VERSION=...`) : l'incrémenter à chaque
modification. Le binaire **merged** (bootloader + partitions + boot_app0 + app) est produit par `deployer`
à la tâche BUILD (`.claude/agents/deploy.md`) et embarqué dans `server-go/assets/firmware/` — `dev-firmware`
ne produit que le build de vérification.

```bash
# Environnement PlatformIO du projet : buzzclick (pas esp32 / esp32c3 génériques)
powershell.exe -NoProfile -Command "& 'C:\Users\cyril\.platformio\penv\Scripts\pio.exe' run -e buzzclick"
# Upload USB / moniteur série
pio run -e buzzclick -t upload
pio device monitor -b 921600
```

Vérification sur hardware réel uniquement (pas de simulation) : connexion au serveur, appuis boutons en
jeu actif, LED pilotées par le serveur, logs UDP (`CustomLogger.h`).

## Commits

Scope `buzzclick` : `feat(buzzclick): …`, `fix(buzzclick): …`, `refactor(buzzclick): …`.

## Coordination

| Modification | Agent à consulter |
|--------------|-------------------|
| Nouvelle action / champ de message | `dev-backend` + `contracts/` (serveur d'abord) |
| Nouveau champ Bumper | `dev-backend` + `contracts/models.md` |
| Affichage TV / admin | `dev-frontend` |
| Procédure de test manuel buzzer | `test-writer` + `qa` |
