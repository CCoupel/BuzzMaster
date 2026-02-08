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

## Compatibilité Firmware / Serveur

| Version Serveur | Version Firmware Compatible | Notes |
|-----------------|----------------------------|-------|
| 2.54.0+ | 2.54.0+ | Versioning unifié |
| < 2.54.0 | 1.209.3 | Anciennes versions (hardcodé dans platformio.ini) |

**Important** : Les buzzers avec un ancien firmware (1.209.3) continuent de fonctionner avec les nouveaux serveurs grâce à la rétrocompatibilité du protocole TCP/UDP.

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
