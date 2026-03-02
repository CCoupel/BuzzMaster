# Guide de Mise à Jour du Firmware BuzzClick

Ce document explique comment flasher le firmware sur les buzzers BuzzClick (ESP32-C3).

---

## Vue d'ensemble

Chaque release GitHub de BuzzControl contient désormais **3 binaires** :
- `buzzcontrol-vX.Y.0-windows-amd64.exe` - Serveur Windows
- `buzzcontrol-vX.Y.0-linux-arm64` - Serveur Linux ARM64 (Raspberry Pi)
- `buzzclick-vX.Y.0-firmware.bin` - **Firmware BuzzClick (ESP32-C3)**

Le serveur et le firmware partagent le **même numéro de version** pour garantir la compatibilité.

---

## Prérequis

### Matériel
- Buzzer BuzzClick (ESP32-C3)
- Câble USB-C
- Ordinateur Windows/Linux/Mac

### Logiciels
- **Python 3.7+** : [Télécharger Python](https://www.python.org/downloads/)
- **PlatformIO CLI** (automatique si vous utilisez VSCode + PlatformIO extension)

### Installation de PlatformIO CLI

```bash
# Installer PlatformIO via pip
pip install --upgrade platformio

# Vérifier l'installation
pio --version
```

---

## Méthode 1 : Flash depuis GitHub Release (recommandé)

### 1. Télécharger le firmware

Aller sur [Releases GitHub](https://github.com/CCoupel/BuzzMaster/releases) et télécharger :
```
buzzclick-vX.Y.0-firmware.bin
```

### 2. Connecter le buzzer en USB

- Brancher le buzzer via USB-C
- Mettre le buzzer en mode bootloader :
  - Maintenir le bouton **BOOT** enfoncé
  - Appuyer brièvement sur **RESET**
  - Relâcher **BOOT**

### 3. Flasher le firmware

```bash
# Identifier le port série (Windows)
pio device list

# Identifier le port série (Linux/Mac)
ls /dev/tty*

# Flasher le firmware
esptool.py --chip esp32c3 --port COM3 write_flash 0x0 buzzclick-vX.Y.0-firmware.bin

# Linux/Mac example
esptool.py --chip esp32c3 --port /dev/ttyUSB0 write_flash 0x0 buzzclick-vX.Y.0-firmware.bin
```

### 4. Vérifier le flash

Après le flash, le buzzer redémarre automatiquement. Vérifier :
- LED clignote (recherche WiFi)
- Buzzer apparaît dans les logs du serveur

---

## Méthode 2 : Build et Flash depuis les sources

Pour les développeurs ou pour tester une version non-release.

### 1. Cloner le dépôt

```bash
git clone https://github.com/CCoupel/BuzzMaster.git
cd BuzzMaster
```

### 2. Build le firmware

```bash
# Build avec PlatformIO
pio run -e buzzclick

# Le binaire est généré dans :
# .pio/build/buzzclick/firmware.bin
```

### 3. Flash via USB

```bash
# Build + Upload en une commande
pio run -e buzzclick -t upload

# Ou upload manuel du binaire existant
esptool.py --chip esp32c3 --port COM3 write_flash 0x0 .pio/build/buzzclick/firmware.bin
```

---

## Méthode 3 : OTA (Over-The-Air) - ⚠️ NON IMPLÉMENTÉ

Cette fonctionnalité sera disponible dans une future version.

---

## Upgrade v3.2.0 : UDP Broadcast Server Discovery

A partir de la version 3.2.0, les buzzers BuzzClick **découvrent automatiquement l'adresse IP du serveur** via des heartbeats UDP, sans configuration manuelle requise.

### Ce qui change

| Aspect | Avant v3.2.0 | Depuis v3.2.0 |
|--------|--------------|---------------|
| Découverte serveur | IP statique en NVS ou mDNS | UDP broadcast (primaire) → NVS → mDNS |
| Séquence LED au boot | 4 phases | 6 phases (2 nouvelles : jaune + bleu) |
| Configuration IP serveur | Requise manuellement | Automatique via broadcast |
| Multi-interfaces serveur | Non supporté | Oui (jusqu'à 8 IPs en failover) |

### Séquence LED complète (v3.2.0)

| Phase | LED | Signification |
|-------|-----|---------------|
| 1 | Blanc 1/4 | Démarrage / init |
| 2 | Rouge 1/4 | Connexion WiFi en cours |
| 3 | Orange 1/4 | WiFi connecté, IP obtenue |
| **4** | **Jaune pulsant (2 Hz)** | **Attente du heartbeat UDP du serveur (max 30s)** |
| **5** | **Bleu clignotant rapide** | **Tentative de connexion sur chaque IP découverte** |
| 6 | Vert 2/4 (mode TCP uniquement) | Connecté au serveur |

### Chaîne de fallback (v3.2.0)

Si aucun heartbeat n'est reçu après 30 secondes, le buzzer tente les méthodes suivantes dans l'ordre :

1. **UDP Broadcast** — méthode principale (timeout 30s)
2. **IP NVS** — utilise `server_ip` stockée en flash lors d'une session précédente
3. **mDNS** — interroge le service `_sock._tcp`
4. En cas d'échec total : réinitialise et réessaie le broadcast

### Retrocompatibilite

Les buzzers avec un ancien firmware (< 3.2.0) continuent de fonctionner avec le serveur v3.2.0. Aucune mise a jour obligatoire — la découverte automatique est un ajout, pas un remplacement du protocole existant.

---

## Upgrade v3.0.0 : Support WebSocket Buzzers

A partir de la version 3.0.0, le serveur BuzzControl supporte un **mode hybride TCP+WebSocket** pour les buzzers physiques.

### Ce qui change

| Aspect | Avant v3.0.0 | Depuis v3.0.0 |
|--------|--------------|---------------|
| Protocole buzzer | TCP port 1234 uniquement | TCP port 1234 + WebSocket `/ws/buzzer` |
| Format messages | JSON + `\n\0` (null-terminated) | JSON standard (WebSocket) ou `\n\0` (TCP) |
| Keep-alive | Aucun | Ping/Pong WebSocket natif (30s) |
| Identification | MAC dans champ `ID` | MAC en query param ou champ `ID` |

### Retrocompatibilite

Les buzzers avec un ancien firmware (TCP) continuent de fonctionner avec le serveur v3.0.0. Aucune mise a jour firmware n'est requise.

Le serveur detecte automatiquement le type de connexion (TCP ou WebSocket) et adapte la serialisation des messages.

### Migration TCP vers WebSocket (firmware)

Le firmware v3.0.0 inclut un client WebSocket pret a l'emploi (`click_websocketClient.h`), active par le flag de compilation `USE_WEBSOCKET`.

Pour migrer un buzzer vers le protocole WebSocket :

1. **Ajouter `-DUSE_WEBSOCKET`** dans `build_flags` de `platformio.ini`
2. **Compiler et flasher** le firmware v3.0.0+
3. Le buzzer se connecte automatiquement a `ws://<server_ip>/ws/buzzer`
4. Le buzzer envoie un message HELLO en JSON standard (sans `\0`)
5. Reconnexion automatique avec backoff exponentiel si deconnexion

**Sans le flag** `USE_WEBSOCKET`, le buzzer utilise le protocole TCP (comportement par defaut, retrocompatible).

La migration est transparente : le serveur gere les deux types de buzzers simultanement.

### Specification protocole

Voir [docs/WEBSOCKET_PROTOCOL.md](WEBSOCKET_PROTOCOL.md) pour la specification complete du protocole WebSocket buzzers.

---

## Compatibilite Firmware / Serveur

| Version Serveur | Version Firmware Compatible | Protocole | Notes |
|-----------------|----------------------------|-----------|-------|
| 3.2.0+ | 3.2.0+ | TCP + WebSocket | UDP broadcast discovery, LED phases 4-5 |
| 3.0.0+ | 3.0.0+ | TCP + WebSocket | Mode hybride, retrocompatible |
| 2.54.0+ | 2.54.0+ | TCP | Versioning unifie |
| < 2.54.0 | 1.209.3 | TCP | Anciennes versions (hardcode dans platformio.ini) |

**Important** : Les buzzers avec un ancien firmware (1.209.3 ou 2.54.x) continuent de fonctionner avec le serveur v3.0.0 grace a la retrocompatibilite du protocole TCP/UDP. Le serveur supporte TCP et WebSocket simultanement.

---

## Vérification Post-Flash

### 1. Moniteur série

```bash
# Ouvrir le moniteur série (baudrate 921600)
pio device monitor -b 921600

# Ou avec screen (Linux/Mac)
screen /dev/ttyUSB0 921600
```

Vous devriez voir :
```
BuzzClick v2.54.0 starting...
[WiFi] Connecting to SSID...
[TCP] Connected to server at 192.168.1.100:1234
```

### 2. Logs serveur

Dans les logs du serveur BuzzControl :
```
[TCP] New connection from 192.168.1.50:xxxxx
[TCP] HELLO from AA:BB:CC:DD:EE:FF (BuzzClick v2.54.0)
```

### 3. Test du bouton

- Appuyer sur le bouton du buzzer
- Vérifier que la LED s'allume
- Vérifier que le serveur reçoit le message BUTTON dans les logs

---

## Problèmes Courants

### Le buzzer ne se connecte pas au WiFi

**Symptôme** : LED clignote indéfiniment

**Solutions** :
1. Vérifier que le SSID et mot de passe WiFi sont corrects dans le code source (`src/BuzzClick/config.h`)
2. Vérifier que le réseau WiFi est en 2.4 GHz (l'ESP32-C3 ne supporte pas le 5 GHz)
3. Redémarrer le buzzer (bouton RESET)

### Le serveur ne voit pas le buzzer

**Symptôme** : Pas de message HELLO dans les logs serveur

**Solutions** :
1. Vérifier que le serveur et le buzzer sont sur le même réseau local
2. Vérifier que le port TCP 1234 n'est pas bloqué par un firewall
3. Vérifier que le serveur est démarré AVANT le buzzer
4. Vérifier l'adresse IP du serveur dans `src/BuzzClick/config.h`

### Erreur "Failed to connect to ESP32"

**Symptôme** : esptool ne peut pas se connecter au buzzer

**Solutions** :
1. Vérifier le câble USB (certains câbles sont "charge only")
2. Installer les drivers USB-to-Serial (CH340, CP2102, etc.)
3. Mettre le buzzer en mode bootloader (BOOT + RESET)
4. Essayer un autre port USB

### Le flash réussit mais le buzzer ne démarre pas

**Symptôme** : Écran noir, pas de LED

**Solutions** :
1. Vérifier que le binaire flashé est bien pour `esp32c3` (pas esp32s3)
2. Reflasher avec l'option `--erase-all` :
   ```bash
   esptool.py --chip esp32c3 --port COM3 erase_flash
   esptool.py --chip esp32c3 --port COM3 write_flash 0x0 buzzclick-vX.Y.0-firmware.bin
   ```
3. Vérifier la tension USB (certains hubs USB ne fournissent pas assez de puissance)

---

## Ressources

- **Documentation PlatformIO** : https://docs.platformio.org/
- **ESP32-C3 Datasheet** : https://www.espressif.com/sites/default/files/documentation/esp32-c3_datasheet_en.pdf
- **esptool Documentation** : https://github.com/espressif/esptool
- **GitHub Releases** : https://github.com/CCoupel/BuzzMaster/releases

---

## Contact

Pour toute question ou problème, ouvrir une issue sur GitHub :
https://github.com/CCoupel/BuzzMaster/issues
