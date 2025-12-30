# BuzzControl Migration Plan - Phase 1

## Objectif

Migrer le serveur BuzzControl de l'ESP32-S3 vers Go (Raspberry Pi / Windows) **sans modifier le firmware BuzzClick**.

## Contraintes

- BuzzClick v1 firmware inchange
- Protocole existant preserve (TCP + UDP Broadcast)
- Meme comportement reseau (SSID, ports, mDNS)
- Compatibilite totale avec les buzzers existants

---

## Architecture Cible Phase 1

```
┌─────────────────────────────────────────────────────────────┐
│                    Serveur Go                               │
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ TCP Server  │  │ UDP Bcast   │  │ HTTP/WebSocket      │ │
│  │ Port: 1234  │  │ Port: 1234  │  │ Port: 80            │ │
│  │ (Buzzers)   │  │ (Broadcast) │  │ (Admin + Clients)   │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
│         │                │                   │              │
│         └────────────────┴───────────────────┘              │
│                          │                                  │
│                    ┌─────▼─────┐                           │
│                    │ Game Core │                           │
│                    │  Engine   │                           │
│                    └─────┬─────┘                           │
│                          │                                  │
│         ┌────────────────┼────────────────┐                │
│         ▼                ▼                ▼                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   Teams     │  │  Bumpers    │  │  Questions  │        │
│  │   Manager   │  │  Manager    │  │  Manager    │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
│                          │                                  │
│                    ┌─────▼─────┐                           │
│                    │  Storage  │                           │
│                    │ (JSON/FS) │                           │
│                    └───────────┘                           │
└─────────────────────────────────────────────────────────────┘
                           │
           ┌───────────────┼───────────────┐
           │               │               │
           ▼               ▼               ▼
    ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
    │  BuzzClick  │ │  BuzzClick  │ │   Browser   │
    │  (TCP+UDP)  │ │  (TCP+UDP)  │ │ (WebSocket) │
    └─────────────┘ └─────────────┘ └─────────────┘
```

---

## Structure du Projet Go

```
buzzcontrol-server/
├── cmd/
│   └── server/
│       └── main.go                 # Point d'entree
├── internal/
│   ├── config/
│   │   └── config.go               # Configuration (JSON)
│   ├── server/
│   │   ├── tcp.go                  # Serveur TCP (buzzers)
│   │   ├── udp.go                  # UDP Broadcast
│   │   ├── http.go                 # Serveur HTTP
│   │   └── websocket.go            # WebSocket (admin/clients)
│   ├── protocol/
│   │   ├── messages.go             # Types de messages
│   │   ├── parser.go               # Parsing JSON (null-terminated)
│   │   └── actions.go              # Handlers par action
│   ├── game/
│   │   ├── state.go                # Machine d'etats du jeu
│   │   ├── timer.go                # Timer de jeu
│   │   ├── teams.go                # Gestion des equipes
│   │   ├── bumpers.go              # Gestion des buzzers
│   │   └── questions.go            # Gestion des questions
│   └── storage/
│       ├── files.go                # Operations fichiers
│       └── backup.go               # Backup/Restore TAR
├── web/                            # Assets statiques (copie de data/)
│   ├── html/
│   ├── js/
│   └── css/
├── config.json                     # Configuration par defaut
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Protocole a Implementer

### TCP Server (Port 1234)

**Connexion:**
- Ecoute sur le port configure (defaut: 1234)
- Accepte les connexions des BuzzClick
- Gere plusieurs clients simultanement

**Format des messages:**
```
{JSON}\0
```
- Messages JSON termines par null byte (`\0`)
- Peut recevoir plusieurs messages dans un seul paquet
- Buffer par client pour messages fragmentes

**Messages recus (BuzzClick → Serveur):**

| ACTION | Description | Payload MSG |
|--------|-------------|-------------|
| HELLO | Enregistrement buzzer | `{VERSION, TEAM, NAME, IP, ...}` |
| BUTTON | Bouton appuye | `{button: "ROUGE\|VERT\|BLEU\|JAUNE"}` |
| PONG | Reponse au PING | `{IP}` |

**Structure message entrant:**
```json
{
  "ID": "AA:BB:CC:DD:EE:FF",
  "VERSION": "1.209.3",
  "ACTION": "BUTTON",
  "MSG": {"button": "ROUGE"}
}
```

### UDP Broadcast (Port 1234)

**Messages envoyes (Serveur → BuzzClick):**

| ACTION | Trigger | Description |
|--------|---------|-------------|
| HELLO | Nouveau client | Demande identification |
| START | Debut question | Lance le jeu |
| STOP | Fin question | Arrete le jeu |
| PAUSE | Bouton appuye | Pause un buzzer |
| CONTINUE | Reprise | Continue le jeu |
| PING | Verification | Demande PONG |
| RESET | RAZ | Reset complet |
| UPDATE | Changement etat | Mise a jour donnees |
| UPDATE_TIMER | Tick timer | Mise a jour temps |

**Structure message sortant:**
```json
{
  "ACTION": "START",
  "VERSION": "1.0.0",
  "MSG": {
    "GAME": {
      "PHASE": "START",
      "DELAY": 30,
      "CURRENT_TIME": 30
    },
    "teams": {...},
    "bumpers": {...}
  },
  "TIME_EVENT": 123456789
}
```

**Calcul adresse broadcast:**
```go
func calculateBroadcast(ip net.IP, mask net.IPMask) net.IP {
    broadcast := make(net.IP, 4)
    for i := 0; i < 4; i++ {
        broadcast[i] = ip[i] | ^mask[i]
    }
    return broadcast
}
```

### HTTP Server (Port 80)

**Routes statiques:**
```
GET  /html/*         → Fichiers HTML
GET  /js/*           → Fichiers JavaScript
GET  /css/*          → Fichiers CSS
GET  /files/*        → Fichiers uploades
GET  /question/*     → Medias questions
```

**Routes API:**
```
GET  /                → Redirect vers /html/testSPA.html#config
GET  /version         → Version serveur
GET  /listGame        → Etat du jeu (JSON)
GET  /listFiles       → Liste fichiers
GET  /questions       → Liste questions
POST /questions       → Upload question (multipart)
GET  /config.json     → Configuration
POST /config.json     → Mise a jour config
GET  /backup          → Download backup TAR
POST /restore         → Upload restore TAR
GET  /clearGame       → Efface donnees jeu
GET  /clearBuzzers    → Efface buzzers
GET  /reboot          → Redemarrage
GET  /reset           → Reset usine
```

**Headers CORS:**
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET,POST,PUT,DELETE,OPTIONS
Access-Control-Allow-Headers: Content-Type,Authorization
```

### WebSocket (ws://server/ws)

**Meme protocole que TCP mais via WebSocket:**
- Messages JSON (sans null terminator)
- Bidirectionnel
- Pour les clients web (admin, affichage)

---

## Machine d'Etats du Jeu

```
                    ┌──────────────────────────────────────┐
                    │                                      │
                    ▼                                      │
┌──────────┐   readyGame()   ┌──────────┐   all PONG   ┌──────────┐
│   STOP   │ ───────────────►│ PREPARE  │ ────────────►│  READY   │
└──────────┘                 └──────────┘              └──────────┘
     ▲                                                      │
     │                                                      │ startGame()
     │                                                      ▼
     │           stopGame()                            ┌──────────┐
     ├◄────────────────────────────────────────────────│  START   │
     │           timer=0                               └──────────┘
     │                                                      │
     │                                                      │ button press
     │                                                      ▼
     │           stopGame()                            ┌──────────┐
     └◄────────────────────────────────────────────────│  PAUSE   │
                                                       └──────────┘
                                                            │
                                                            │ continueGame()
                                                            │
                                                            └──────► START
```

**Etats:**
| Etat | Description |
|------|-------------|
| STOP | Jeu arrete, en attente |
| PREPARE | Question selectionnee, attente buzzers |
| READY | Tous les buzzers prets |
| START | Jeu en cours, timer actif |
| PAUSE | Buzzer a appuye, en attente decision |

---

## Structures de Donnees Go

### Configuration
```go
type Config struct {
    Server struct {
        HTTPPort       int    `json:"http_port"`
        TCPPort        int    `json:"tcp_port"`
        WebSocketPath  string `json:"websocket_path"`
    } `json:"server"`
    WiFi struct {
        SSID     string `json:"ssid"`
        Password string `json:"password"`
    } `json:"wifi"`
    Game struct {
        DefaultDelay int `json:"default_delay"`
    } `json:"game"`
    Storage struct {
        DataDir      string `json:"data_dir"`
        QuestionsDir string `json:"questions_dir"`
    } `json:"storage"`
}
```

### Message Protocol
```go
type Message struct {
    Seq       int             `json:"seq,omitempty"`
    Action    string          `json:"ACTION"`
    ID        string          `json:"ID,omitempty"`
    Version   string          `json:"VERSION,omitempty"`
    Msg       json.RawMessage `json:"MSG"`
    TimeEvent int64           `json:"TIME_EVENT,omitempty"`
}
```

### Game State
```go
type GameState struct {
    Phase       string `json:"PHASE"`
    Delay       int    `json:"DELAY"`
    CurrentTime int    `json:"CURRENT_TIME"`
    Question    *Question `json:"QUESTION,omitempty"`
    Page        string `json:"PAGE"`
}

type Question struct {
    ID       string `json:"ID"`
    Question string `json:"QUESTION"`
    Answer   string `json:"ANSWER"`
    Points   int    `json:"POINTS"`
    Time     int    `json:"TIME"`
    Media    string `json:"MEDIA,omitempty"`
    Status   string `json:"STATUS"`
}
```

### Teams & Bumpers
```go
type Team struct {
    Name   string  `json:"NAME"`
    Color  []int   `json:"COLOR"`
    Score  int     `json:"SCORE"`
    Time   int64   `json:"TIME,omitempty"`
    Status string  `json:"STATUS,omitempty"`
    Bumper string  `json:"BUMPER,omitempty"`
}

type Bumper struct {
    Name    string `json:"NAME"`
    Team    string `json:"TEAM"`
    Score   int    `json:"SCORE"`
    Time    int64  `json:"TIME,omitempty"`
    Button  string `json:"BUTTON,omitempty"`
    Status  string `json:"STATUS,omitempty"`
    Version string `json:"VERSION"`
    IP      string `json:"IP,omitempty"`
}

type TeamsAndBumpers struct {
    Teams   map[string]*Team   `json:"teams"`
    Bumpers map[string]*Bumper `json:"bumpers"`
}
```

---

## Plan d'Implementation

### Etape 1: Squelette du projet
- [ ] Initialiser module Go
- [ ] Structure des dossiers
- [ ] Configuration (lecture JSON)
- [ ] Logger

### Etape 2: Serveur TCP
- [ ] Accepter connexions
- [ ] Parser messages null-terminated
- [ ] Gerer buffer par client
- [ ] Handler HELLO
- [ ] Handler BUTTON
- [ ] Handler PONG

### Etape 3: UDP Broadcast
- [ ] Calculer adresse broadcast
- [ ] Envoyer messages broadcast
- [ ] Actions: START, STOP, PAUSE, UPDATE, etc.

### Etape 4: Serveur HTTP
- [ ] Routes statiques
- [ ] API REST
- [ ] Upload fichiers
- [ ] CORS

### Etape 5: WebSocket
- [ ] Endpoint /ws
- [ ] Broadcast aux clients web
- [ ] Memes actions que TCP

### Etape 6: Game Engine
- [ ] Machine d'etats
- [ ] Timer (goroutine)
- [ ] Gestion scores
- [ ] Gestion equipes

### Etape 7: Storage
- [ ] Lecture/ecriture JSON
- [ ] Gestion questions
- [ ] Backup TAR
- [ ] Restore TAR

### Etape 8: Tests
- [ ] Tests unitaires
- [ ] Test avec BuzzClick reel
- [ ] Test multi-buzzers
- [ ] Test reconnexion

### Etape 9: Deploiement
- [ ] Build pour Raspberry Pi
- [ ] Script installation hostapd/dnsmasq
- [ ] Service systemd
- [ ] Documentation

---

## Commandes de Build

```bash
# Developpement (Windows)
go run ./cmd/server

# Build Windows
go build -o buzzcontrol.exe ./cmd/server

# Build Raspberry Pi (ARM64)
GOOS=linux GOARCH=arm64 go build -o buzzcontrol ./cmd/server

# Build Raspberry Pi (ARM32, Pi Zero)
GOOS=linux GOARCH=arm GOARM=6 go build -o buzzcontrol ./cmd/server
```

---

## Configuration Raspberry Pi

### hostapd (/etc/hostapd/hostapd.conf)
```
interface=wlan0
driver=nl80211
ssid=buzzmaster
hw_mode=g
channel=7
wmm_enabled=0
macaddr_acl=0
auth_algs=1
ignore_broadcast_ssid=0
wpa=2
wpa_passphrase=BuzzMaster
wpa_key_mgmt=WPA-PSK
rsn_pairwise=CCMP
```

### dnsmasq (/etc/dnsmasq.conf)
```
interface=wlan0
dhcp-range=192.168.4.10,192.168.4.50,255.255.255.0,24h
address=/#/192.168.4.1
```

### Service systemd (/etc/systemd/system/buzzcontrol.service)
```ini
[Unit]
Description=BuzzControl Server
After=network.target hostapd.service

[Service]
Type=simple
User=pi
WorkingDirectory=/home/pi/buzzcontrol
ExecStart=/home/pi/buzzcontrol/buzzcontrol
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

---

## Checklist de Validation

### Compatibilite BuzzClick v1
- [ ] BuzzClick se connecte en TCP
- [ ] BuzzClick recoit UDP broadcast
- [ ] HELLO enregistre le buzzer
- [ ] BUTTON detecte correctement
- [ ] PONG repond au PING
- [ ] LEDs changent selon l'etat
- [ ] Reconnexion automatique fonctionne

### Fonctionnalites Serveur
- [ ] Interface web accessible
- [ ] WebSocket fonctionne
- [ ] Questions uploadables
- [ ] Backup/Restore fonctionnel
- [ ] Timer precis
- [ ] Scores corrects

### Deploiement
- [ ] Hotspot WiFi fonctionne
- [ ] DNS captif fonctionne
- [ ] Service demarre au boot
- [ ] Stable sur 24h+

---

## Risques et Mitigations

| Risque | Mitigation |
|--------|------------|
| Timing UDP different | Tester avec oscilloscope si necessaire |
| Buffer overflow parsing | Tests fuzz sur parser JSON |
| Memoire sur Pi Zero | Profiling, limiter goroutines |
| Corruption fichiers | Ecriture atomique, backups |
| WiFi instable | Logs detailles, reconnexion auto |
