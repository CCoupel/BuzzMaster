# BuzzControl - Migration Architecture Decision Record

## Contexte

Le projet BuzzControl est un systeme de buzzers pour jeux de quiz, actuellement implemente sur ESP32-S3. Des problemes de performance et de stabilite ont ete identifies, necessitant une migration vers une plateforme plus robuste.

### Architecture actuelle (ESP32)

```
┌─────────────────────────────────────────┐
│         ESP32-S3 (BuzzControl)          │
│  • Serveur HTTP/WebSocket               │
│  • Serveur TCP pour buzzers             │
│  • Point d'acces WiFi                   │
│  • DNS captif                           │
│  • Stockage LittleFS (6 Mo)             │
│  • RAM: ~320 Ko                         │
└─────────────────────────────────────────┘
              │ WiFi AP
              ▼
┌─────────────────────────────────────────┐
│       ESP32-C3 (BuzzClick) x N          │
│  • Client TCP                           │
│  • Boutons physiques                    │
│  • LEDs NeoPixel                        │
└─────────────────────────────────────────┘
```

### Problemes identifies

| Probleme | Impact | Evidence dans le code |
|----------|--------|----------------------|
| RAM limitee (320 Ko) | Crashes lors d'operations lourdes | Monitoring `ESP.getFreeHeap()` constant |
| Seuil critique memoire | Echecs backup/restore | Verification 50 Ko minimum dans WebServer.h |
| Watchdog resets | Instabilite | `esp_task_wdt_init(60, false)` |
| Gestion chunks complexe | Code fragile | Streaming TAR par morceaux |
| Single-thread effectif | Latence | Multiples `yield()` et `sleep()` |

---

## Decision 1: Langage de programmation

### Options evaluees

| Langage | Avantages | Inconvenients |
|---------|-----------|---------------|
| **Rust** | Performance, securite memoire | Courbe d'apprentissage raide |
| **Go** | Simple, goroutines, cross-compile | GC (negligeable ici) |
| C++ | Continuite avec ESP32 | Pas de benefice reel |

### Decision: **Go**

**Raisons:**
1. **Goroutines** : Gestion naturelle de multiples connexions simultanees (buzzers, websockets, HTTP)
2. **Cross-compilation triviale** : Un binaire par plateforme sans dependances
3. **Bibliotheques natives** : `net/http`, `encoding/json` integres
4. **Temps de developpement** : 2-3x plus rapide que Rust pour ce type de projet
5. **Transition naturelle** : Les taches FreeRTOS actuelles se traduisent bien en goroutines

---

## Decision 2: Plateforme de production

### Options evaluees

| Plateforme | Hotspot WiFi | Autonomie | Stabilite | Cout |
|------------|--------------|-----------|-----------|------|
| Windows PC | Complexe (netsh/WinRT) | Non | Bonne | Eleve |
| Linux PC | Bon (hostapd) | Non | Excellente | Eleve |
| **Raspberry Pi** | Natif (hostapd) | Oui | Excellente | ~50 EUR |
| ESP32 (actuel) | Natif | Oui | Problematique | ~10 EUR |

### Decision: **Raspberry Pi 4 (2 Go)**

**Raisons:**
1. **Point d'acces WiFi natif** : `hostapd` + `dnsmasq` = meme comportement que l'ESP32
2. **Aucune modification des BuzzClick** : Meme architecture reseau
3. **Ressources adequates** : 2 Go RAM vs 320 Ko = 6000x plus
4. **Autonomie** : Appareil dedie, portable, boot rapide
5. **Cout raisonnable** : ~50 EUR pour une solution professionnelle
6. **Fiabilite** : Linux stable, pas de watchdog necessaire

### Raspberry Pi recommande

| Modele | RAM | Prix | Recommandation |
|--------|-----|------|----------------|
| Pi Zero 2 W | 512 Mo | ~20 EUR | Suffisant, ultra compact |
| **Pi 4 (2 Go)** | 2 Go | ~45 EUR | **Choix optimal** |
| Pi 4 (4 Go) | 4 Go | ~60 EUR | Overkill |
| Pi 5 | 4-8 Go | ~80 EUR | Overkill |

---

## Decision 3: Strategie de developpement

### Decision: **Developpement Windows, Production Raspberry Pi**

```
┌─────────────────────────────────────────────────────────────┐
│                    Code Go unique                           │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐   │
│  │ Serveur HTTP  │  │ Serveur TCP   │  │ WebSocket     │   │
│  │ (API + Web)   │  │ (Buzzers)     │  │ (Temps reel)  │   │
│  └───────────────┘  └───────────────┘  └───────────────┘   │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐   │
│  │ Game Logic    │  │ JSON/Config   │  │ File Storage  │   │
│  └───────────────┘  └───────────────┘  └───────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
            ┌───────────────┴───────────────┐
            ▼                               ▼
┌─────────────────────┐         ┌─────────────────────┐
│   Raspberry Pi      │         │   Windows PC        │
│   (Production)      │         │   (Developpement)   │
├─────────────────────┤         ├─────────────────────┤
│ • hostapd = AP WiFi │         │ • Reseau existant   │
│ • dnsmasq = DHCP    │         │ • IDE confortable   │
│ • Portable          │         │ • Debug facile      │
│ • Autonome          │         │ • Tests rapides     │
└─────────────────────┘         └─────────────────────┘
```

**Compilation:**
```bash
# Developpement Windows
go build -o buzzcontrol.exe

# Production Raspberry Pi
GOOS=linux GOARCH=arm64 go build -o buzzcontrol-pi
```

---

## Decision 4: Hotspot WiFi Windows

### Decision: **Ne pas automatiser le hotspot Windows**

**Raisons:**
1. **netsh hostednetwork** : Deprecie depuis Windows 10 v1607, incompatible avec beaucoup de cartes
2. **Mobile Hotspot API (WinRT)** : Complexe a appeler depuis Go, comportement variable
3. **Fiabilite** : ~60-70% des cartes WiFi compatibles vs ~95% sur Linux
4. **Inutile** : En developpement, les buzzers peuvent se connecter au meme reseau WiFi que le PC

**Alternative pour tests Windows:**
- Connecter le PC et les buzzers au meme reseau WiFi existant
- Ou utiliser le Mobile Hotspot Windows manuellement (interface graphique)

---

## Decision 5: Configuration reseau production

### Architecture reseau Raspberry Pi

```
┌─────────────────────────────────────────┐
│           Raspberry Pi 4                │
│  ┌─────────────────────────────────┐    │
│  │  WiFi integre = Point d'acces   │    │
│  │  SSID: "BuzzControl"            │    │
│  │  IP: 192.168.4.1                │    │
│  │  DHCP: 192.168.4.10-50          │    │
│  └─────────────────────────────────┘    │
│  ┌─────────────────────────────────┐    │
│  │  Ethernet (optionnel)           │────┼──► Internet
│  └─────────────────────────────────┘    │
└─────────────────────────────────────────┘
         │
         │ WiFi (192.168.4.x)
         ▼
    ┌─────────┐  ┌─────────┐  ┌─────────┐
    │BuzzClick│  │BuzzClick│  │ Tablet  │
    │ ESP32-C3│  │ ESP32-C3│  │ (Admin) │
    └─────────┘  └─────────┘  └─────────┘
```

### Fichiers de configuration

**`/etc/hostapd/hostapd.conf`:**
```
interface=wlan0
driver=nl80211
ssid=BuzzControl
hw_mode=g
channel=7
wmm_enabled=0
macaddr_acl=0
auth_algs=1
ignore_broadcast_ssid=0
wpa=2
wpa_passphrase=buzzcontrol123
wpa_key_mgmt=WPA-PSK
rsn_pairwise=CCMP
```

**`/etc/dnsmasq.conf`:**
```
interface=wlan0
dhcp-range=192.168.4.10,192.168.4.50,255.255.255.0,24h
address=/#/192.168.4.1
```

---

## Comparaison finale

| Critere | ESP32 (actuel) | Raspberry Pi (cible) |
|---------|----------------|----------------------|
| RAM | 320 Ko | 2 Go |
| Stockage | 6 Mo (LittleFS) | 32+ Go (SD) |
| CPU | Single-core 240 MHz | Quad-core 1.5 GHz |
| Hotspot WiFi | Natif | Natif (hostapd) |
| Stabilite | Problematique | Excellente |
| Autonomie | Oui | Oui |
| Cout | ~10 EUR | ~50 EUR |
| Modification BuzzClick | - | Aucune |

---

## Fonctionnalites a migrer

| Fonctionnalite | ESP32 | Go equivalent | Priorite |
|----------------|-------|---------------|----------|
| Serveur HTTP REST | ESPAsyncWebServer | `net/http` ou `gin`/`fiber` | Haute |
| WebSocket | ESPAsyncWebServer | `gorilla/websocket` | Haute |
| Serveur TCP (buzzers) | AsyncTCP | `net` (natif) | Haute |
| JSON parsing | ArduinoJson | `encoding/json` (natif) | Haute |
| Gestion fichiers | LittleFS | `os` (natif) | Haute |
| Timer de jeu | Ticker | `time.Ticker` | Haute |
| Backup/Restore TAR | ESP32-targz | `archive/tar` (natif) | Moyenne |
| Point d'acces WiFi | WiFi.softAP() | hostapd (externe) | Externe |
| DNS captif | DNSServer | dnsmasq (externe) | Externe |
| LEDs RGB | NeoPixel | Interface web | Basse |

---

## Risques et mitigations

| Risque | Probabilite | Impact | Mitigation |
|--------|-------------|--------|------------|
| Corruption SD card | Moyenne | Eleve | Mode read-only ou SSD USB |
| Boot plus lent | Faible | Faible | Acceptable (15-20s) |
| WiFi moins performant | Faible | Moyen | Antenne externe si besoin |
| Complexite config Linux | Moyenne | Moyen | Scripts d'installation |

---

## Prochaines etapes

1. [ ] Creer la structure du projet Go
2. [ ] Implementer le serveur TCP pour buzzers
3. [ ] Implementer le serveur HTTP/WebSocket
4. [ ] Porter la logique de jeu
5. [ ] Tester avec les BuzzClick existants
6. [ ] Configurer le Raspberry Pi (hostapd, dnsmasq)
7. [ ] Tests d'integration complets
8. [ ] Documentation utilisateur

---

## References

- Code source ESP32: `src/BuzzControl/`
- Configuration actuelle: `platformio.ini`
- Version actuelle: 1.209.3
