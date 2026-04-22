# Tests non-régression — OTA URL avec port (issue #50)

## Contexte du bug

**Bug (avant fix v3.6.1)** : Le handler `OTA_UPDATE` dans `click_serverConnection.h`
construisait l'URL sans port :
```c
// BUG — port omis, HTTP client se connecte sur port 80 par défaut
otaUrl = "http://" + currentConfig.server_ip + "/api/firmware/buzzclick/latest.bin";
```
Sur un serveur non-standard (ex: port 8080), le download échoue avec `HTTP error: -1`
(connexion refusée sur port 80).

**Fix (v3.6.1)** — `click_serverConnection.h`, handler `OTA_UPDATE` :
```c
// Priorité 1 : connexion active (UDP heartbeat / mDNS)
if (serverIP.length() > 0 && localUdpPort > 0) {
    otaUrl = "http://" + serverIP + ":" + String(localUdpPort)
             + "/api/firmware/buzzclick/latest.bin";
// Priorité 2 : NVS
} else if (currentConfig.server_ip.length() > 0) {
    uint16_t port = currentConfig.server_tcp_port > 0 ? currentConfig.server_tcp_port : 80;
    otaUrl = "http://" + currentConfig.server_ip + ":" + String(port)
             + "/api/firmware/buzzclick/latest.bin";
// Priorité 3 : URL du message OTA_UPDATE (rétrocompat firmware < 3.1.2)
} else {
    otaUrl = String(message["URL"] | "");
}
```

---

## Prérequis communs

- Moniteur série UART USB connecté au BuzzClick (baud 115200)
- Outil : Arduino IDE Serial Monitor, `screen /dev/ttyUSB0 115200`, ou PlatformIO Monitor
- Serveur BuzzControl démarré sur la machine hôte
- Tags de log à surveiller : `[OTA]`, `[SRV]`

---

## Cas 1 — Port non-standard (cas principal du bug)

### Objectif
Vérifier que l'URL OTA inclut le port 8080 quand le serveur tourne sur ce port.

### Configuration
| Paramètre | Valeur |
|-----------|--------|
| `config.json` → `http_port` | **8080** |
| Buzzer | Connecté via UDP heartbeat (localUdpPort = 8080) |
| Globals firmware au moment du message | `serverIP = "192.168.1.X"`, `localUdpPort = 8080` |

### Procédure
1. Modifier `server-go/config.json` : `"http_port": 8080`
2. (Re)démarrer le serveur
3. Laisser le buzzer se connecter via UDP broadcast → voir log `[WIFI] Server found: 192.168.1.X:8080`
4. Vérifier `serverIP` et `localUdpPort` bien initialisés : log `[SRV] Connecting from ... To SERVER ...:8080`
5. Déclencher OTA depuis l'admin (bouton ▲ sur la carte buzzer)
6. Observer le log UART

### Log UART attendu
```
[OTA] Starting OTA update to version 3.6.1 from: http://192.168.1.X:8080/api/firmware/buzzclick/latest.bin
[OTA] OTA_PROGRESS: {"ACTION":"OTA_PROGRESS","ID":"AA:BB:CC:DD:EE:FF","MSG":{"STATUS":"downloading","PERCENT":0}}
[OTA] Firmware size: 524288 bytes
[OTA] OTA_PROGRESS: ... STATUS:downloading PERCENT:10 ...
...
[OTA] OTA_PROGRESS: ... STATUS:flashing PERCENT:100 ...
[OTA] OTA flash complete! Restarting in 2 seconds...
[OTA] OTA_PROGRESS: ... STATUS:done PERCENT:100 ...
```

### Assertion critique
```
URL contient ":8080"  → ✅
URL ne contient pas ":80" incorrectement → ✅
STATUS "error" absent des logs → ✅
```

### Résultat KO (bug non corrigé)
```
[OTA] Starting OTA update to version 3.6.1 from: http://192.168.1.X/api/firmware/buzzclick/latest.bin
[OTA] OTA_PROGRESS: {"MSG":{"STATUS":"error",...,"ERROR":"HTTP error: -1"}}
```

---

## Cas 2 — Port 80 standard (non-régression)

### Objectif
Vérifier que le fix n'a pas cassé le cas nominal (port 80 par défaut).

### Configuration
| Paramètre | Valeur |
|-----------|--------|
| `config.json` → `http_port` | **80** (défaut) |
| Globals firmware | `serverIP = "192.168.1.X"`, `localUdpPort = 80` |

### Log UART attendu
```
[OTA] Starting OTA update to version 3.6.1 from: http://192.168.1.X:80/api/firmware/buzzclick/latest.bin
[OTA] Firmware size: XXXXXX bytes
[OTA] OTA flash complete! Restarting in 2 seconds...
```

### Assertion
```
URL contient ":80" → ✅ (équivalent à port 80 par défaut)
STATUS "done" présent → ✅
```

> **Note** : `http://IP:80/...` et `http://IP/...` sont équivalents. Le `:80` explicite
> dans l'URL n'est pas un problème — les navigateurs et HTTPClient l'acceptent.

---

## Cas 3 — Fallback NVS (serverIP global vide)

### Objectif
Vérifier que si `serverIP`/`localUdpPort` ne sont pas initialisés (ex: reconnexion
après perte de globals), le buzzer utilise la config NVS avec le bon port.

### Configuration
| Paramètre | Valeur |
|-----------|--------|
| NVS `server_ip` | `192.168.1.X` |
| NVS `server_tcp_port` | **9090** |
| `serverIP` global firmware | `""` (vide — non initialisé) |
| `localUdpPort` global firmware | **0** |

### Simulation du cas
Ce cas se produit si le buzzer reçoit `OTA_UPDATE` **avant** d'avoir établi sa
connexion active (ex: message reçu depuis l'ancienne session). Pour simuler :
- Appuyer sur Reset physique du buzzer pendant qu'il est déjà en train de recevoir
  le message OTA, ou modifier temporairement le firmware en initialisant
  `serverIP = ""` et `localUdpPort = 0` avant de traiter le message.

En pratique, ce cas est documenté mais difficile à déclencher manuellement sans
modification firmware temporaire. **Test prioritairement par lecture de code.**

### Log UART attendu
```
[SRV] Using NVS fallback for OTA URL: http://192.168.1.X:9090/api/firmware/buzzclick/latest.bin
[OTA] Starting OTA update to version 3.6.1 from: http://192.168.1.X:9090/api/firmware/buzzclick/latest.bin
```

### Assertion
```
Log "[SRV] Using NVS fallback for OTA URL" présent → ✅
URL contient ":9090" → ✅
```

### Vérification par lecture de code (click_serverConnection.h)
```c
} else if (currentConfig.server_ip.length() > 0) {
    uint16_t port = currentConfig.server_tcp_port > 0 ? currentConfig.server_tcp_port : 80;
    otaUrl = "http://" + currentConfig.server_ip + ":" + String(port)
             + "/api/firmware/buzzclick/latest.bin";
    ESP_LOGW(SRV_TAG, "Using NVS fallback for OTA URL: %s", otaUrl.c_str());
```
✅ Si `server_tcp_port = 0` → port forcé à 80 (pas de port 0 dans l'URL).
✅ Le log `[SRV] Using NVS fallback` est présent.

---

## Cas 4 — Rétrocompat message URL (ni serverIP ni NVS)

### Objectif
Vérifier que si ni `serverIP` ni `currentConfig.server_ip` ne sont disponibles,
le buzzer utilise le champ `URL` du message `OTA_UPDATE` (rétrocompat firmware < 3.1.2).

### Configuration
| Paramètre | Valeur |
|-----------|--------|
| `serverIP` global firmware | `""` (vide) |
| `localUdpPort` | 0 |
| `currentConfig.server_ip` | `""` (vide — NVS non configuré) |
| Message `OTA_UPDATE` reçu | `{ "URL": "http://192.168.1.X:80/api/firmware/buzzclick/latest.bin" }` |

### Log UART attendu
```
[SRV] No connected server known, using URL from message: http://192.168.1.X:80/api/firmware/buzzclick/latest.bin
[OTA] Starting OTA update to version 3.6.1 from: http://192.168.1.X:80/api/firmware/buzzclick/latest.bin
```

### Assertion
```
Log "[SRV] No connected server known, using URL from message" présent → ✅
URL = URL du message OTA_UPDATE → ✅
```

### Vérification par lecture de code (click_serverConnection.h)
```c
} else {
    otaUrl = String(message["URL"] | "");
    ESP_LOGW(SRV_TAG, "No connected server known, using URL from message: %s", otaUrl.c_str());
}
```
✅ Troisième branche : URL prise du message.

> **Note** : Le serveur Go passe toujours un champ `URL` dans `OTA_UPDATE` pour
> assurer cette rétrocompatibilité (voir `buildFirmwareURL()` dans `http_firmware.go`).

---

## Matrice de décision

| Condition | serverIP | localUdpPort | NVS ip | NVS port | URL utilisée |
|-----------|----------|--------------|--------|----------|--------------|
| Connexion active (cas normal) | `"192.168.1.X"` | 8080 | — | — | `http://192.168.1.X:8080/api/...` |
| Connexion active port 80 | `"192.168.1.X"` | 80 | — | — | `http://192.168.1.X:80/api/...` |
| Globals vides, NVS renseigné | `""` | 0 | `"192.168.1.X"` | 9090 | `http://192.168.1.X:9090/api/...` |
| Globals vides, NVS port = 0 | `""` | 0 | `"192.168.1.X"` | 0 | `http://192.168.1.X:80/api/...` (port forcé à 80) |
| Rien disponible | `""` | 0 | `""` | — | Champ `URL` du message OTA_UPDATE |

---

## Tests automatisés associés (Go)

Dans `server-go/internal/server/firmware_http_test.go` :

| Test | Vérifie |
|------|---------|
| `TestBuildFirmwareURL_Format` | Format URL backward-compat côté serveur |
| `TestHandleAPIBuzzerUpdate_NotConnected_JSONError` | Réponse JSON stable quand buzzer hors-ligne |

---

## Références

- Commit du fix : `84b1194` — `fix(firmware): honor broadcasted port in OTA URL (v3.6.1)`
- Fichiers modifiés : `src/BuzzClick/click_serverConnection.h` (handler `OTA_UPDATE`)
- Fichiers associés : `server-go/internal/server/http_firmware.go` (`buildFirmwareURL`)
- Issue GitHub : #50
