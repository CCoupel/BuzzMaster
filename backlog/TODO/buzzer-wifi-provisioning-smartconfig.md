# Provisionnement WiFi des buzzers via SmartConfig

**Statut** : 📋 Planifié

## Description

Au démarrage d'un buzzer (BuzzClick ESP32-C3), permettre la configuration du réseau WiFi via **SmartConfig** au lieu d'avoir les credentials WiFi hardcodés dans le firmware. L'admin configure le SSID et le mot de passe dans la page d'admin, puis active le mode ENROLL. Le serveur active alors automatiquement le SmartConfig, permettant aux buzzers en mode écoute de recevoir la configuration WiFi.

**Architecture backend** : Le serveur BuzzControl est **100% Go**, sans dépendances Python ou externes. Le protocole ESP-Touch est implémenté nativement en Go.

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
- [ ] **Transmission de l'IP/hostname du serveur + ports utiles dans le message SmartConfig**
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
2. Le serveur envoie des packets UDP encodant le SSID + password + **IP du serveur (détectée auto) + ports (TCP, HTTP)**
3. Le buzzer décode les credentials, l'IP du serveur et les ports depuis la longueur des packets WiFi
4. Le buzzer se connecte au WiFi
5. Le buzzer se connecte au serveur BuzzControl via l'IP:port reçu (ex: 192.168.1.84:1234)
6. Le buzzer passe automatiquement en phase ENROLL

**Avantages :**
- ✅ Configuration simultanée de 10+ buzzers en 30 secondes
- ✅ Pas besoin de Bluetooth ni d'app mobile
- ✅ Protocole natif ESP32 (bibliothèque intégrée)
- ✅ Fonctionne même si serveur connecté en Ethernet (via routeur WiFi)

### Architecture

```
Admin (Page Admin)
    ↓ Configure SSID + password dans config.json
Admin (Active mode ENROLL)
    ↓ Mode ENROLL activé
Serveur BuzzControl (Go natif)
    ↓ Active automatiquement SmartConfig
    ↓ Détecte sa propre IP (ex: 192.168.1.84)
    ↓ Envoie packets ESP-Touch (UDP broadcast)
    ↓ Contenu : SSID + password + IP serveur + ports (TCP:1234, HTTP:80)
Routeur/AP WiFi
    ↓ Diffuse trames WiFi dans l'air
Buzzers en mode écoute (LED rouge clignotante)
    ↓ Captent trames WiFi, décodent credentials + IP serveur + ports
    ↓ Stockent en NVS (WiFi + server_ip + tcp_port), redémarrent
    ↓ Se connectent au WiFi
    ↓ Se connectent au serveur via IP:port reçu (ex: 192.168.1.84:1234)
    ↓ Apparaissent dans la liste ENROLL
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
  - Décoder SSID + password + **server_ip** + **tcp_port** depuis packets
  - Stocker en NVS (`Preferences` library) :
    - `wifi_ssid` : SSID du réseau
    - `wifi_password` : Mot de passe WiFi
    - `server_ip` : IP du serveur BuzzControl (ex: "192.168.1.84")
    - `server_tcp_port` : Port TCP pour connexion buzzers (ex: 1234)
  - Redémarrer automatiquement

- [ ] **Connexion WiFi normale**
  - Lire SSID + password + server_ip + server_tcp_port depuis NVS
  - LED jaune clignotante (connexion en cours)
  - Timeout 30 secondes → effacer config et reboot si échec
  - LED verte fixe (connecté)
  - Connexion TCP au serveur BuzzControl via server_ip:server_tcp_port
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

### Phase 2 - Backend Go : Implémentation ESP-Touch Native

**IMPORTANT** : Le backend est **100% Go**, sans dépendances Python ou externes.

- [ ] **Configuration WiFi dans config.json**
  ```json
  {
    "wifi": {
      "ssid": "MonReseau",
      "password": "motdepasse123"
    },
    "tcp_port": 1234
  }
  ```
  Note :
  - L'IP du serveur est **détectée automatiquement** au démarrage (comme dans les logs : `http://192.168.1.84 (Wi-Fi)`)
  - Lors du SmartConfig, le serveur transmet toujours l'IP détectée (pas de hostname)
  - Le port TCP est configurable (défaut: 1234)

- [ ] **Endpoint GET `/api/config/wifi`**
  - Retourne la config WiFi actuelle (SSID + password)
  - Utilisé pour pré-remplir l'interface admin

- [ ] **Endpoint POST `/api/config/wifi`**
  - Met à jour SSID + password dans config.json
  - Validation des champs (SSID non vide, password min 8 caractères)

- [ ] **Implémentation Go native du protocole ESP-Touch**
  - Package `internal/smartconfig/esptouch.go`
  - Encodage des credentials dans les packets UDP :
    - SSID du WiFi
    - Password du WiFi
    - **IP du serveur** (détectée automatiquement, ex: `192.168.1.84`)
    - **Port TCP** pour connexion buzzers (ex: 1234)
  - Calcul CRC8 pour validation
  - Broadcast UDP sur toutes les interfaces réseau
  - Durée d'envoi : 30 secondes (garantit réception)
  - **Détection automatique de l'IP** : Utiliser la même logique que l'affichage dans les logs (interface Wi-Fi prioritaire)

- [ ] **Activation automatique en mode ENROLL**
  - Quand l'admin active le mode ENROLL :
    1. Le serveur démarre automatiquement le SmartConfig
    2. Détecte sa propre IP (même logique que les logs)
    3. Envoie les packets ESP-Touch en continu (SSID + password + **IP serveur** + **port TCP**)
    4. Les buzzers en mode écoute reçoivent les credentials + IP serveur + port
    5. Les buzzers se connectent au WiFi puis au serveur via IP:port
    6. Les buzzers apparaissent dans la liste ENROLL

- [ ] **Pas de bouton séparé "Configurer buzzers"**
  - Le SmartConfig est **intégré** au mode ENROLL
  - L'admin n'a qu'à activer ENROLL (workflow existant)
  - Les buzzers et VJoueurs se déclarent automatiquement

**Fichiers concernés** :
- `internal/smartconfig/esptouch.go` : Implémentation Go native ESP-Touch
- `internal/smartconfig/encoder.go` : Encodage packets UDP
- `internal/smartconfig/crc.go` : Calcul CRC8
- `cmd/server/config.go` : Gestion config.json
- `internal/game/enroll.go` : Intégration SmartConfig dans mode ENROLL

---

### Phase 3 - Interface Admin et ENROLL

#### Page Admin : Configuration WiFi

- [ ] **Configuration réseau dans page Admin**
  - Champs SSID + password (section dédiée)
  - **Champ port TCP** (défaut: 1234, pour connexion buzzers)
  - **Affichage de l'IP détectée** (lecture seule, ex: "IP serveur : 192.168.1.84")
  - Bouton "Enregistrer" → sauvegarde dans config.json
  - Validation :
    - SSID obligatoire
    - Password min 8 caractères
    - Port TCP valide (1-65535)
  - Affichage de la config actuelle
  - Info-bulles :
    - "IP serveur : Cette adresse est détectée automatiquement au démarrage et sera transmise aux buzzers via SmartConfig"
    - "Port TCP : Port utilisé par les buzzers pour se connecter au serveur (défaut: 1234)"

#### Page ENROLL : Activation automatique SmartConfig

- [ ] **Workflow ENROLL simplifié**
  - Quand l'admin active le mode ENROLL :
    1. Le serveur démarre **automatiquement** le SmartConfig
    2. Envoie les packets ESP-Touch en continu
    3. Les buzzers en mode écoute reçoivent la config
    4. Les buzzers apparaissent dans la liste ENROLL

- [ ] **Section Buzzers**
  - Compteur : "X buzzers connectés"
  - Instructions simplifiées :
    1. Allumez les buzzers neufs (LED rouge = mode SmartConfig)
    2. OU maintenez le bouton 3s au boot pour reset factory
    3. Les buzzers se connecteront automatiquement
  - **Pas de bouton séparé** : le SmartConfig est activé automatiquement en mode ENROLL
  - Liste des buzzers connectés (mise à jour temps réel via WebSocket)

- [ ] **Section VJoueurs + QR Codes**
  - Compteur : "X VJoueurs connectés"
  - **QR Code 1 : WiFi** (format standard)
    - `WIFI:T:WPA;S:MonReseau;P:password123;;`
    - Texte : "Scannez pour vous connecter au WiFi automatiquement"
  - **QR Code 2 : URL VJoueur**
    - `http://buzzcontrol.local/player`
    - Texte : "Scannez pour accéder à la page VJoueur"
  - Liste des VJoueurs connectés (mise à jour temps réel)

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
- `server-go/web/src/pages/ConfigPage.jsx` : Configuration WiFi
- `server-go/web/src/pages/EnrollPage.jsx` : Page ENROLL (SmartConfig auto)
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
  - Test implémentation Go native ESP-Touch
  - Test encodage/CRC8 des packets UDP
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
1. Admin ouvre la page Admin
2. Configure SSID "MonReseau" + password "pass123"
3. Configure port TCP 1234 (ou laisse le défaut)
4. Clique sur "Enregistrer"
   Note : L'IP du serveur (ex: 192.168.1.84) est détectée automatiquement et affichée

5. Admin ouvre la page ENROLL et active le mode ENROLL
   → Le serveur démarre automatiquement le SmartConfig
   → Envoie SSID + password + IP serveur (192.168.1.84) + port (1234)

6. Admin démarre 10 buzzers neufs
   → Tous passent en mode SmartConfig (LED rouge clignotante)
   → Reçoivent automatiquement WiFi credentials + IP serveur + port

7. Les 10 buzzers stockent la config en NVS et redémarrent
   → LED jaune clignotante (connexion WiFi)
   → Connexion TCP à 192.168.1.84:1234
   → LED verte fixe (connecté au serveur)

8. Les buzzers apparaissent dans la liste ENROLL
   → Compteur : "10 buzzers connectés"

9. Admin assigne les buzzers aux équipes (workflow ENROLL standard)
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

6. Admin active le mode ENROLL (si pas déjà actif)
   → Le serveur envoie automatiquement la config SmartConfig
   → Contenu : SSID + password + IP serveur + port TCP
7. Buzzer reçoit la config complète, se connecte au WiFi puis à l'IP:port, apparaît dans la liste
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

### ✅ Transmission de l'IP du serveur + port TCP (CRITIQUE)
- **Pourquoi c'est essentiel :**
  - Les buzzers doivent savoir **où se connecter** après avoir obtenu les credentials WiFi
  - Sans cette information, ils devraient avoir l'IP/port hardcodés
- **Avantages :**
  - **IP détectée automatiquement** : Le serveur détecte sa propre IP au démarrage (DHCP compatible)
  - **Pas besoin de mDNS** : Transmission directe de l'IP (plus simple et robuste)
  - **Port TCP configurable** : permet déploiements avec configuration réseau spécifique
  - Configuration complète en une seule opération
  - Buzzers vraiment "génériques" (aucun hardcoding d'adresse ni de port)
  - Pas de configuration manuelle d'IP nécessaire (détection automatique)

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

### ✅ Aucune dépendance externe
- Serveur **100% Go** (pas de Python, pas de dépendances externes)
- Protocole ESP-Touch implémenté nativement en Go
- Binaire portable autonome

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
| `internal/smartconfig/esptouch.go` | Go | Implémentation native ESP-Touch |
| `internal/smartconfig/encoder.go` | Go | Encodage packets UDP |
| `internal/smartconfig/crc.go` | Go | Calcul CRC8 |
| `cmd/server/config.go` | Go | Gestion config.json |
| `internal/game/models.go` | Go | Type Client, ClientType |
| `internal/game/enroll.go` | Go | Intégration SmartConfig en mode ENROLL |
| **Frontend React** | | |
| `web/src/pages/ConfigPage.jsx` | React | Configuration WiFi (SSID + password) |
| `web/src/pages/EnrollPage.jsx` | React | Page ENROLL (SmartConfig auto) |
| `web/src/components/VPlayerPanel.jsx` | React | Panneau VJoueurs + QR codes |
| `web/src/pages/EnrollPage.css` | CSS | Styles |
| `web/package.json` | - | Ajout dépendance qrcode.react |

## Dépendances techniques

- **ESP32-C3** : Support SmartConfig natif (bibliothèque ESP-IDF) ✅
- **Backend Go** : Implémentation native ESP-Touch (100% Go, pas de dépendances externes) ✅
- **qrcode.react** : `npm install qrcode.react` ✅
- **Réseau local** : Serveur et AP WiFi sur même LAN ✅

## Version cible

**v3.0.0** (breaking change : nouveau workflow de configuration des buzzers)

## Références

- [ESP32 SmartConfig Documentation](https://docs.espressif.com/projects/esp-idf/en/latest/esp32/api-reference/network/esp_smartconfig.html)
- [ESP-Touch Protocol Specification](https://www.espressif.com/sites/default/files/documentation/esp-touch_user_guide_en.pdf)
- [QR Code WiFi Format](https://github.com/zxing/zxing/wiki/Barcode-Contents#wi-fi-network-config-android-ios-11)
- [qrcode.react NPM](https://www.npmjs.com/package/qrcode.react)
