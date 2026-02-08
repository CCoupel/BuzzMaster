# Provisionnement WiFi des buzzers via SmartConfig

**Statut** : 📋 Planifié

## Description

Au démarrage d'un buzzer (BuzzClick ESP32-C3), permettre la configuration du réseau WiFi via **SmartConfig** au lieu d'avoir les credentials WiFi hardcodés dans le firmware. L'admin configure le SSID et le mot de passe dans l'interface ENROLL, puis envoie la configuration simultanément à tous les buzzers en mode écoute.

## Contexte

### Situation actuelle

Les buzzers BuzzClick ont actuellement les credentials WiFi hardcodés dans le firmware :
- SSID et mot de passe définis lors de la compilation
- Nécessite une recompilation et un re-flash pour changer de réseau
- Pas adapté pour des déploiements multi-sites ou événements itinérants

### Problème

- Impossibilité de changer de réseau WiFi sans reflasher le buzzer
- Configuration complexe pour les utilisateurs non techniques
- Pas de solution pour des buzzers "génériques" utilisables sur plusieurs réseaux

## Objectifs

- [ ] Configuration WiFi via SmartConfig (ESP-Touch)
- [ ] Interface simple dans la page ENROLL (comme VJoueurs)
- [ ] Stockage sécurisé des credentials WiFi dans la mémoire NVS du buzzer
- [ ] Reset factory via bouton (maintenu 3 secondes au boot)
- [ ] Compteur de buzzers connectés (comme VJoueurs)
- [ ] QR Code pour faciliter connexion VJoueurs (bonus)

## Solution : SmartConfig (ESP-Touch)

### Principe

**SmartConfig** est un protocole Espressif qui permet de transmettre des credentials WiFi à un ESP32 **sans que celui-ci soit connecté** à un réseau.

**Comment ça fonctionne :**
1. Le buzzer en mode "SmartConfig listening" écoute passivement les trames WiFi dans l'air
2. Le serveur envoie des packets UDP encodant le SSID + password
3. Le buzzer décode les credentials depuis la longueur des packets WiFi
4. Le buzzer se connecte au WiFi et au serveur BuzzControl
5. Le buzzer passe automatiquement en phase ENROLL

**Avantages :**
- ✅ Configuration simultanée de 10+ buzzers en 30 secondes
- ✅ Pas besoin de Bluetooth ni d'app mobile
- ✅ Protocole natif ESP32 (bibliothèque intégrée)
- ✅ Fonctionne même si serveur connecté en Ethernet (via routeur WiFi)

### Architecture

```
Admin (Interface ENROLL)
    ↓ Configure SSID + password
Serveur BuzzControl
    ↓ Envoie packets SmartConfig (UDP broadcast)
Routeur/AP WiFi
    ↓ Diffuse trames WiFi dans l'air
Buzzers en mode écoute (LED rouge clignotante)
    ↓ Captent trames WiFi, décodent credentials
    ↓ Stockent en NVS, redémarrent
    ↓ Se connectent au WiFi
    ↓ Se connectent au serveur
    ↓ Passent en phase ENROLL
```

## Tâches

### Phase 1 - Firmware BuzzClick : Mode SmartConfig

#### Conditions d'entrée en mode SmartConfig

Le buzzer entre en mode SmartConfig si :
- **Condition 1** : Aucune config WiFi stockée en NVS (premier démarrage)
- **Condition 2** : Bouton maintenu 3+ secondes au boot (reset factory)

#### Implémentation

- [ ] **Détection bouton reset au boot**
  - Vérifier état du bouton pendant 3 secondes au démarrage
  - LED bleue clignotante pendant la détection
  - Si bouton maintenu → effacer NVS et entrer en SmartConfig
  - Si bouton relâché → boot normal

- [ ] **Mode SmartConfig**
  - LED rouge clignotante (mode configuration)
  - `WiFi.beginSmartConfig()` (bibliothèque ESP32)
  - Timeout 5 minutes → reboot si aucune config reçue
  - Décoder SSID + password depuis packets
  - Stocker en NVS (`Preferences` library)
  - Redémarrer automatiquement

- [ ] **Connexion WiFi normale**
  - Lire SSID + password depuis NVS
  - LED jaune clignotante (connexion en cours)
  - Timeout 30 secondes → effacer config et reboot si échec
  - LED verte fixe (connecté)
  - Connexion TCP au serveur BuzzControl (port 1234)
  - Passage en phase ENROLL

- [ ] **Indicateurs LED**
  - 🔵 Bleu clignotant : Détection bouton reset (3s)
  - 🔴 Rouge clignotant : Mode SmartConfig (attente config)
  - 🟡 Jaune clignotant : Connexion WiFi en cours
  - 🟢 Vert fixe : Connecté au serveur
  - 🔴 Rouge rapide : Erreur de connexion

**Fichiers concernés** :
- `src/BuzzClick/main.cpp` : Logique de démarrage
- `src/BuzzClick/wifi_config.cpp` : Gestion NVS et SmartConfig
- `src/BuzzClick/led_control.cpp` : Indicateurs visuels

---

### Phase 2 - Backend Go : Endpoint SmartConfig

- [ ] **Configuration WiFi dans config.json**
  ```json
  {
    "wifi": {
      "ssid": "MonReseau",
      "password": "motdepasse123"
    }
  }
  ```

- [ ] **Endpoint GET `/api/config/wifi`**
  - Retourne la config WiFi actuelle (SSID + password)
  - Utilisé pour pré-remplir l'interface ENROLL

- [ ] **Endpoint POST `/api/config/wifi`**
  - Met à jour SSID + password dans config.json
  - Validation des champs (SSID non vide, password min 8 caractères)

- [ ] **Endpoint POST `/api/smartconfig/provision`**
  - Reçoit SSID + password en JSON
  - Appelle script Python `pyesptouch` ou bibliothèque Go
  - Envoie packets SmartConfig pendant 30 secondes
  - Retourne `{ "success": true/false, "devices_configured": N }`

- [ ] **Installation pyesptouch (Python)**
  ```bash
  pip install pyesptouch
  ```

- [ ] **Script helper `scripts/esptouch.py`**
  ```python
  from esptouch import esptouch
  import sys

  ssid = sys.argv[1]
  password = sys.argv[2]
  results = esptouch.esptouch(ssid, password, timeout=30)
  print(f"{len(results)} device(s) configured")
  ```

**Fichiers concernés** :
- `cmd/server/smartconfig.go` : Handlers SmartConfig
- `cmd/server/config.go` : Gestion config.json
- `scripts/esptouch.py` : Script Python helper
- `requirements.txt` : Dépendances Python

---

### Phase 3 - Interface ENROLL : Panneau SmartConfig + QR Codes

#### Interface ENROLL unifiée (comme VJoueurs)

- [ ] **Section 1 : Configuration réseau**
  - Champs SSID + password
  - Bouton "Enregistrer" → sauvegarde dans config.json
  - Validation : SSID obligatoire, password min 8 caractères

- [ ] **Section 2 : Buzzers (SmartConfig)**
  - Compteur : "X buzzers connectés"
  - Instructions :
    1. Allumez les buzzers (LED rouge = mode config)
    2. OU maintenez le bouton 3s au boot pour reset
    3. Cliquez sur "Configurer les buzzers WiFi"
    4. Attendez 30 secondes
  - Bouton : "📡 Configurer les buzzers WiFi"
  - Barre de progression pendant l'envoi (30 sec)
  - Message de succès : "✅ Configuration envoyée ! Les buzzers vont se connecter."
  - Liste des buzzers connectés (mise à jour temps réel via WebSocket)

- [ ] **Section 3 : VJoueurs (QR Codes)**
  - Compteur : "X VJoueurs connectés"
  - **QR Code 1 : WiFi** (format standard)
    - `WIFI:T:WPA;S:MonReseau;P:password123;;`
    - Texte : "Scannez pour vous connecter au WiFi automatiquement"
  - **QR Code 2 : URL VJoueur**
    - `http://buzzcontrol.local/player`
    - Texte : "Scannez pour accéder à la page VJoueur"
  - Liste des VJoueurs connectés (mise à jour temps réel)

- [ ] **Composant `SmartConfigPanel.jsx`**
  - Gestion état : idle | sending | success | error
  - Appel API `/api/smartconfig/provision`
  - Animation de progression (30 secondes)
  - Feedback visuel clair

- [ ] **Composant `VPlayerPanel.jsx`**
  - Génération QR codes WiFi et URL
  - Utilisation bibliothèque `qrcode.react`
  - Affichage instructions claires

- [ ] **Installation dépendances**
  ```bash
  cd server-go/web
  npm install qrcode.react
  ```

**Fichiers concernés** :
- `server-go/web/src/pages/EnrollPage.jsx` : Page principale
- `server-go/web/src/components/SmartConfigPanel.jsx` : Panneau buzzers
- `server-go/web/src/components/VPlayerPanel.jsx` : Panneau VJoueurs + QR
- `server-go/web/src/pages/EnrollPage.css` : Styles

---

### Phase 4 - Compteur de clients par type

- [ ] **Type de client dans le modèle**
  ```go
  type ClientType string

  const (
      ClientBuzzer  ClientType = "buzzer"
      ClientVPlayer ClientType = "vplayer"
      ClientAdmin   ClientType = "admin"
      ClientTV      ClientType = "tv"
  )

  type Client struct {
      MAC         string     `json:"mac"`
      Type        ClientType `json:"type"`
      IPAddress   string     `json:"ip"`
      ConnectedAt time.Time  `json:"connected_at"`
  }
  ```

- [ ] **Tracking des clients connectés**
  - Map globale : `clients map[string]*Client`
  - Incrémentation lors de la connexion d'un buzzer (TCP)
  - Incrémentation lors de la connexion d'un VJoueur (WebSocket)
  - Décrémentation lors de la déconnexion

- [ ] **Broadcast temps réel**
  - Message WebSocket `UPDATE` enrichi :
    ```json
    {
      "action": "UPDATE",
      "BUZZER_COUNT": 5,
      "VPLAYER_COUNT": 3,
      "CLIENTS": [...]
    }
    ```
  - Envoyé à tous les admins lors de connexion/déconnexion

- [ ] **Affichage dans l'interface**
  - Compteur en temps réel : "5 buzzers connectés"
  - Compteur en temps réel : "3 VJoueurs connectés"
  - Mise à jour automatique via WebSocket (pas de refresh page)

**Fichiers concernés** :
- `internal/game/models.go` : Type Client
- `cmd/server/main.go` : Tracking clients
- `server-go/web/src/pages/EnrollPage.jsx` : Affichage compteurs

---

### Phase 5 - Tests et documentation

- [ ] **Tests firmware BuzzClick**
  - Test mode SmartConfig (premier boot)
  - Test reset factory (bouton 3s)
  - Test connexion WiFi normale
  - Test timeouts (SmartConfig, WiFi)
  - Test stockage/lecture NVS

- [ ] **Tests backend**
  - Test endpoint `/api/smartconfig/provision`
  - Test script `esptouch.py`
  - Test avec 1 buzzer, puis 10 buzzers simultanés
  - Test avec serveur en WiFi vs Ethernet

- [ ] **Tests interface**
  - Test workflow complet ENROLL
  - Test QR codes (WiFi + URL)
  - Test compteurs temps réel
  - Test barre de progression SmartConfig

- [ ] **Documentation**
  - Guide utilisateur : "Comment configurer un nouveau buzzer"
  - Guide utilisateur : "Comment réinitialiser un buzzer (reset factory)"
  - Guide technique : "Architecture SmartConfig"
  - Vidéo démo (optionnel)

## Scénarios d'usage

### Scénario 1 : Premier démarrage de 10 buzzers neufs

```
1. Admin ouvre la page ENROLL
2. Configure SSID "MonReseau" + password "pass123"
3. Clique sur "Enregistrer"

4. Admin démarre 10 buzzers neufs
   → Tous passent en mode SmartConfig (LED rouge clignotante)

5. Admin clique sur "📡 Configurer les buzzers WiFi"
   → Barre de progression : 0/30 sec... 15/30 sec... 30/30 sec
   → Message : "✅ Configuration envoyée !"

6. Les 10 buzzers reçoivent simultanément les credentials
   → Stockent en NVS, redémarrent
   → LED jaune clignotante (connexion WiFi)
   → LED verte fixe (connecté au serveur)

7. Les buzzers apparaissent dans la liste ENROLL
   → Compteur : "10 buzzers connectés"

8. Admin assigne les buzzers aux équipes (workflow ENROLL standard)
```

---

### Scénario 2 : Reset factory d'un buzzer

```
1. Utilisateur éteint le buzzer
2. Maintient le bouton enfoncé
3. Allume le buzzer (bouton toujours maintenu)
   → LED bleue clignotante (détection reset)

4. Après 3 secondes : LED rouge rapide (confirmation reset)
5. Buzzer redémarre en mode SmartConfig (LED rouge clignotante)

6. Admin dans ENROLL clique sur "Configurer les buzzers WiFi"
7. Buzzer reçoit la config, se connecte, apparaît dans la liste
```

---

### Scénario 3 : VJoueur se connecte via QR Code

```
1. Admin ouvre la page ENROLL
2. Affiche les QR codes dans la section VJoueurs

3. Utilisateur avec smartphone :
   a. Scanne le QR Code WiFi
      → Android/iOS propose de se connecter à "MonReseau"
      → Connexion automatique

   b. Scanne le QR Code URL VJoueur
      → Ouvre http://buzzcontrol.local/player
      → Accède à l'interface VJoueur

4. Le VJoueur apparaît dans la liste ENROLL
   → Compteur : "3 VJoueurs connectés"
```

## Avantages de cette solution

### ✅ Configuration massive rapide
- 10+ buzzers configurés en 30 secondes
- Pas de manipulation individuelle
- Idéal pour événements ou déploiements multiples

### ✅ Workflow unifié ENROLL
- Buzzers et VJoueurs dans la même interface
- Compteurs temps réel
- UX cohérente et intuitive

### ✅ Reset factory simple
- Bouton maintenu 3 secondes au boot
- Pas besoin de reflash
- Utilisateur autonome

### ✅ QR Code pour VJoueurs
- Connexion WiFi automatique (scan)
- Accès direct à la page VJoueur (scan)
- Pas de saisie manuelle d'URL

### ✅ Protocole standard
- SmartConfig natif ESP32
- Bibliothèques bien testées (pyesptouch)
- Fiabilité 80-95% selon environnement

## Contraintes et limitations

### ⚠️ Environnement WiFi
- Taux de réussite peut baisser si environnement WiFi très chargé
- Distance max serveur ↔ buzzers : ~10-15 mètres (portée WiFi)
- Le serveur doit être sur le même réseau local que l'AP WiFi

### ⚠️ Sécurité
- Credentials transmis encodés mais pas chiffrés par défaut
- Quelqu'un avec un sniffer WiFi peut théoriquement intercepter
- Solution : SmartConfig V2 (AES) existe mais plus complexe (phase future)

### ⚠️ Durée de provisionnement
- 30 secondes nécessaires pour garantir la réception
- Plus lent que BLE (5-10 sec) mais compense en configurant plusieurs buzzers

### ⚠️ Dépendance Python
- Nécessite `python3` + `pyesptouch` installés sur le serveur
- Alternative : bibliothèque Go native (à développer, phase future)

## Alternatives futures (non prioritaires)

### Option 2 : Bluetooth LE (BLE)
- Configuration individuelle via Web Bluetooth API
- Plus rapide (5-10 sec par buzzer) mais un par un
- Nécessite navigateur Chrome/Edge
- **Cas d'usage** : Reconfiguration d'un buzzer spécifique

### Option 3 : USB/Série
- Configuration filaire via câble USB
- Toujours fonctionne (fallback ultime)
- **Cas d'usage** : Debug, configuration en usine

Ces alternatives peuvent être ajoutées en Phase 6+ si besoin.

## Fichiers concernés (résumé)

| Fichier | Type | Modification |
|---------|------|--------------|
| **Firmware BuzzClick** | | |
| `src/BuzzClick/main.cpp` | C++ | Logique de démarrage, conditions SmartConfig |
| `src/BuzzClick/wifi_config.cpp` | C++ | SmartConfig, NVS, connexion WiFi |
| `src/BuzzClick/led_control.cpp` | C++ | Indicateurs LED |
| **Backend Go** | | |
| `cmd/server/smartconfig.go` | Go | Endpoints SmartConfig |
| `cmd/server/config.go` | Go | Gestion config.json |
| `internal/game/models.go` | Go | Type Client, ClientType |
| `scripts/esptouch.py` | Python | Script helper SmartConfig |
| `requirements.txt` | - | Dépendances Python |
| **Frontend React** | | |
| `web/src/pages/EnrollPage.jsx` | React | Page ENROLL unifiée |
| `web/src/components/SmartConfigPanel.jsx` | React | Panneau buzzers SmartConfig |
| `web/src/components/VPlayerPanel.jsx` | React | Panneau VJoueurs + QR codes |
| `web/src/pages/EnrollPage.css` | CSS | Styles |
| `web/package.json` | - | Ajout dépendance qrcode.react |

## Dépendances techniques

- **ESP32-C3** : Support SmartConfig natif (bibliothèque ESP-IDF) ✅
- **Python 3** : Pour exécuter pyesptouch ✅
- **pyesptouch** : `pip install pyesptouch` ✅
- **qrcode.react** : `npm install qrcode.react` ✅
- **Réseau local** : Serveur et AP WiFi sur même LAN ✅

## Version cible

**v3.0.0** (breaking change : nouveau workflow de configuration des buzzers)

## Références

- [ESP32 SmartConfig Documentation](https://docs.espressif.com/projects/esp-idf/en/latest/esp32/api-reference/network/esp_smartconfig.html)
- [pyesptouch GitHub](https://github.com/nekmo/pyesptouch)
- [QR Code WiFi Format](https://github.com/zxing/zxing/wiki/Barcode-Contents#wi-fi-network-config-android-ios-11)
- [qrcode.react NPM](https://www.npmjs.com/package/qrcode.react)
