# Mise à jour OTA du firmware des buzzers

**Statut** : 📋 Planifié

## Description

Permettre la mise à jour automatique du firmware des buzzers (BuzzClick ESP32-C3) directement depuis l'interface d'administration, sans câble USB.

**Fonctionnalités clés** :
- Détection automatique de la version firmware de chaque buzzer connecté
- Pastille visuelle pour indiquer les buzzers avec firmware obsolète
- Serveur embarque le firmware de référence (binaire .bin)
- Upload OTA du firmware via l'interface admin (en plus de la config WiFi)
- Support OTA dans le firmware ESP32-C3

## Objectifs

- [ ] Buzzer envoie sa version firmware au serveur lors de la connexion
- [ ] Serveur compare avec version de référence et détecte obsolescence
- [ ] Interface admin affiche pastille pour buzzers obsolètes
- [ ] Serveur stocke firmware de référence dans `/data/firmware/`
- [ ] Page Config : upload nouveau firmware (.bin)
- [ ] Commande OTA envoyée au buzzer pour déclencher mise à jour
- [ ] Firmware ESP32 : téléchargement, flash, reboot automatique
- [ ] Mise à jour en masse (tous les buzzers obsolètes)

## Tâches

### Phase 1 - Backend : Gestion du firmware

- [ ] Endpoint GET `/api/firmware/buzzclick/version` (version de référence)
- [ ] Endpoint GET `/api/firmware/buzzclick/latest.bin` (téléchargement binaire)
- [ ] Endpoint POST `/api/buzzer/{mac}/update` (déclencher OTA individuelle)
- [ ] Endpoint POST `/api/buzzer/update-all` (OTA en masse)
- [ ] Stockage firmware dans `server-go/data/firmware/`
- [ ] Modèle Buzzer enrichi : `FirmwareVersion`, `IsOutdated`

### Phase 2 - Firmware BuzzClick : Support OTA

- [ ] Envoi version firmware lors de ENROLL : `{"firmware_version": "3.0.5"}`
- [ ] Constante compilée : `#define FIRMWARE_VERSION "3.0.5"`
- [ ] Réception commande OTA : `{"action": "OTA_UPDATE", "url": "..."}`
- [ ] Téléchargement binaire via HTTP depuis serveur
- [ ] Flash sur partition OTA ESP32
- [ ] Reboot automatique + rollback en cas d'échec
- [ ] Indicateurs LED : Bleu = téléchargement, Vert = succès, Rouge = échec

### Phase 3 - Interface Admin : Gestion OTA

- [ ] ConfigPage : section upload firmware de référence (.bin)
- [ ] Validation upload : extension .bin, taille 200KB-2MB
- [ ] GamePage/Équipes : pastille orange sur buzzers obsolètes
- [ ] BuzzerCard : affichage version + bouton "Mettre à jour"
- [ ] Progression OTA temps réel (WebSocket)
- [ ] Bouton "Mettre à jour tous" pour buzzers obsolètes
- [ ] Notifications succès/échec

### Phase 4 - CI/CD : Build firmware automatique

- [ ] Workflow GitHub Actions : compile `buzzclick-vX.Y.Z.bin`
- [ ] Upload binaire comme artefact de release
- [ ] Synchronisation versions (platformio.ini + config.json)
- [ ] Injection version : `build_flags = -DFIRMWARE_VERSION=\"X.Y.Z\"`

### Phase 5 - Tests et documentation

- [ ] Tests OTA réussie/échouée/rollback
- [ ] Tests upload firmware valide/invalide
- [ ] Tests mise à jour individuelle + en masse
- [ ] Documentation : Guide admin mise à jour buzzers
- [ ] Documentation : Architecture OTA ESP32

## Scénarios d'usage

### Scénario 1 : Déploiement nouveau firmware

1. Dev compile firmware v3.0.10 (PlatformIO)
2. Admin upload .bin via ConfigPage
3. Serveur stocke firmware de référence
4. Admin voit 5 buzzers avec pastille orange (v3.0.5 obsolète)
5. Click "Mettre à jour tous"
6. Serveur envoie commande OTA aux 5 buzzers
7. Buzzers téléchargent, flashent, redémarrent (~30-60s chacun)
8. Interface affiche "5/5 buzzers mis à jour ✅"

### Scénario 2 : Détection automatique

1. Nouveau buzzer v3.0.5 se connecte
2. Serveur reçoit version, compare avec référence (v3.0.10)
3. Pastille orange affichée automatiquement
4. Admin peut déclencher OTA individuelle

### Scénario 3 : Rollback automatique

1. OTA démarre, flash réussi, reboot
2. Nouveau firmware plante au boot
3. ESP32 détecte échec → rollback vers ancienne version
4. LED rouge, notification échec à l'admin

## Avantages

✅ Mise à jour rapide de tous les buzzers (sans câble USB)
✅ Autonomie utilisateur (admin non technique)
✅ Rollback automatique en cas d'échec
✅ Visibilité versions obsolètes
✅ Scalabilité (10+ buzzers simultanés)

## Contraintes

⚠️ Réseau WiFi stable requis (~500KB-1MB par buzzer)
⚠️ Durée : ~30-60s par buzzer
⚠️ Partitions OTA ESP32 requises (2 partitions A/B)
⚠️ Sécurité : signature firmware (phase future)

## Fichiers concernés

| Fichier | Modification |
|---------|--------------|
| `internal/server/http.go` | Endpoints `/api/firmware/*` |
| `internal/game/models.go` | Buzzer.FirmwareVersion, IsOutdated |
| `src/BuzzClick/click_ota.h` | Logique OTA ESP32 |
| `src/BuzzClick/platformio.ini` | Configuration partitions OTA |
| `web/src/pages/ConfigPage.jsx` | Upload firmware |
| `web/src/components/BuzzerCard.jsx` | Pastille version + bouton OTA |
| `.github/workflows/release.yml` | Build firmware auto |

## Version cible

**v3.1.0** (feature mineure, OTA optionnel)

## Références

- [ESP32 OTA Update](https://docs.espressif.com/projects/esp-idf/en/latest/esp32/api-reference/system/ota.html)
- [ArduinoOTA Library](https://github.com/esp8266/Arduino/tree/master/libraries/ArduinoOTA)
- [ESP32 Partition Tables](https://docs.espressif.com/projects/esp-idf/en/latest/esp32/api-guides/partition-tables.html)
