# Migration du protocole des buzzers physiques vers WebSocket

**Statut** : ✅ Complété (v3.0.0)

## Description

Migrer le protocole de communication des buzzers physiques (BuzzClick ESP32-C3) de **TCP propriétaire** vers **WebSocket**. Cette migration simplifie l'architecture réseau, unifie les protocoles clients, et prépare l'infrastructure pour des fonctionnalités avancées (notifications push, bidirectionnel temps réel).

## Contexte

### Situation actuelle

Le serveur BuzzControl utilise **deux protocoles distincts** :
- **TCP sur port 1234** : Buzzers physiques (BuzzClick)
- **WebSocket sur port 80** : Clients web (Admin, TV, VJoueur)

```
Buzzers BuzzClick (ESP32-C3)
    ↓ TCP port 1234
    ↓ Messages null-terminated (\0)
Serveur BuzzControl (Go)
    ↓ WebSocket port 80 (/ws)
    ↓ Messages JSON
Clients Web (Admin, TV, VJoueur)
```

### Problème

| Aspect | TCP actuel | WebSocket proposé |
|--------|------------|-------------------|
| **Complexité** | 2 protocoles distincts à maintenir | 1 protocole unifié |
| **NAT/Firewall** | Port TCP custom (1234) peut être bloqué | Port HTTP/WS standard (80/443) passe partout |
| **Upgrade firmware** | Protocole figé, difficile à faire évoluer | Extensible, versioning facile |
| **Debugging** | Outils custom nécessaires | Outils WebSocket standards (navigateur, wscat) |
| **Sécurité** | Pas de chiffrement (TCP brut) | WSS (WebSocket over TLS) possible |
| **Compression** | Non supporté | Compression WebSocket native |

### Avantages de la migration

✅ **Unification** : Un seul protocole pour tous les clients (buzzers + web)
✅ **Standard** : WebSocket est un protocole standard (RFC 6455)
✅ **Firewall-friendly** : Port 80/443 rarement bloqué
✅ **Extensible** : Ajout facile de nouvelles fonctionnalités
✅ **Debugging** : Outils de debug WebSocket disponibles partout
✅ **Futur-proof** : WSS (chiffrement TLS), compression, authentification

## Objectifs

- [ ] Implémenter le protocole WebSocket dans le firmware BuzzClick
- [ ] Adapter le serveur Go pour gérer les buzzers via WebSocket
- [ ] Maintenir la rétrocompatibilité TCP pendant la transition
- [ ] Tester la fiabilité et latence WebSocket vs TCP
- [ ] Déployer progressivement (buzzers v2 uniquement)

## Tâches

### Phase 1 - Analyse et spécification

- [ ] **Documenter le protocole actuel**
  - Messages TCP actuels (ENROLL, BUZZ, LED_ON, etc.)
  - Format null-terminated, encodage
  - Timing et latence observée

- [ ] **Spécifier le protocole WebSocket**
  - Format des messages (JSON recommandé)
  - Authentification/identification des buzzers
  - Gestion reconnexion automatique
  - Heartbeat/keep-alive

- [ ] **Évaluer les contraintes ESP32-C3**
  - Bibliothèques WebSocket disponibles (Arduino, ESP-IDF)
  - Impact mémoire RAM/Flash
  - Latence WebSocket vs TCP brut
  - Stabilité connexion WiFi

**Fichiers concernés** :
- `docs/PROTOCOLS.md` : Documentation protocole actuel
- Nouveau fichier : `docs/WEBSOCKET_PROTOCOL.md`

---

### Phase 2 - Implémentation serveur Go

- [ ] **Gérer les buzzers WebSocket dans le serveur**
  - Nouveau handler `/ws/buzzer` (distinct de `/ws` admin/TV)
  - Identification buzzer lors du handshake (MAC address)
  - Stockage dans `wsClients` avec type `buzzer`

- [ ] **Adapter les messages existants**
  - Convertir messages TCP → JSON WebSocket
  - Garder la compatibilité avec logique métier actuelle
  - Broadcast vers buzzers WebSocket + TCP (mode hybride)

- [ ] **Maintenir rétrocompatibilité TCP**
  - Garder le serveur TCP port 1234 actif
  - Buzzers anciens (TCP) et nouveaux (WS) coexistent
  - Détection automatique du type de client

**Exemple de message WebSocket** :
```json
// Buzzer → Serveur
{
  "action": "BUZZ",
  "mac": "AA:BB:CC:DD:EE:FF",
  "timestamp": 1234567890
}

// Serveur → Buzzer
{
  "action": "LED_ON",
  "color": "green"
}
```

**Fichiers concernés** :
- `internal/server/websocket.go` : Handler `/ws/buzzer`
- `internal/game/engine.go` : Support buzzers WebSocket
- `internal/game/models.go` : Type `buzzer` pour WebSocket

---

### Phase 3 - Implémentation firmware BuzzClick

- [ ] **Intégration bibliothèque WebSocket**
  - Tester bibliothèques disponibles :
    - `ArduinoWebsockets` (simple, léger)
    - ESP-IDF native WebSocket (plus complexe, mais officiel)
  - Benchmark mémoire et latence

- [ ] **Connexion WebSocket au serveur**
  - URL : `ws://192.168.1.84/ws/buzzer` (IP reçue via SmartConfig)
  - Envoi MAC address lors du handshake (identification)
  - Reconnexion automatique si déconnexion

- [ ] **Adaptation des messages**
  - Encoder les événements BUZZ en JSON
  - Parser les commandes LED en JSON
  - Garder la même logique métier (juste changement de transport)

- [ ] **Gestion reconnexion**
  - Détection perte connexion WebSocket
  - Reconnexion automatique avec backoff exponentiel
  - LED indicateur : jaune clignotant si reconnexion en cours

**Fichiers concernés** :
- `src/BuzzClick/websocket_client.cpp` : Client WebSocket
- `src/BuzzClick/main.cpp` : Intégration dans la loop principale

---

### Phase 4 - Tests et validation

- [ ] **Tests unitaires serveur**
  - Connexion/déconnexion buzzers WebSocket
  - Messages BUZZ reçus correctement
  - Commandes LED envoyées correctement
  - Coexistence TCP + WebSocket

- [ ] **Tests firmware**
  - Connexion WebSocket stable
  - Latence BUZZ acceptable (< 50ms)
  - Reconnexion automatique fonctionnelle
  - Consommation mémoire acceptable

- [ ] **Tests de charge**
  - 10 buzzers WebSocket simultanés
  - Latence sous charge
  - Stabilité sur longue durée (> 1h)

- [ ] **Tests de compatibilité**
  - Buzzers TCP anciens + WebSocket nouveaux
  - Pas de régression pour buzzers TCP
  - Migration transparente côté serveur

---

### Phase 5 - Déploiement progressif

- [ ] **Firmware BuzzClick v2**
  - Version firmware avec WebSocket
  - Flag de compilation `USE_WEBSOCKET` (TCP par défaut pour rétrocompat)
  - Documentation upgrade firmware

- [ ] **Rollout progressif**
  - Phase 1 : Test avec 1-2 buzzers WebSocket
  - Phase 2 : Migration progressive des buzzers
  - Phase 3 : Dépréciation TCP (v4.0.0 future)

- [ ] **Documentation**
  - Guide migration TCP → WebSocket
  - Comparatif performance TCP vs WebSocket
  - Troubleshooting WebSocket

**Fichiers concernés** :
- `docs/WEBSOCKET_PROTOCOL.md` : Spécification complète
- `docs/FIRMWARE_UPDATE.md` : Guide upgrade firmware
- `CHANGELOG.md` : Annonce du nouveau protocole

---

## Rétrocompatibilité

### Stratégie de migration

```
Phase 1 (v3.0) : Serveur supporte TCP + WebSocket (mode hybride)
Phase 2 (v3.x) : Firmware BuzzClick v2 avec WebSocket (optionnel)
Phase 3 (v4.0) : WebSocket devient le protocole par défaut
Phase 4 (v5.0) : Dépréciation TCP (à décider selon adoption)
```

### Coexistence TCP + WebSocket

Le serveur doit supporter les deux protocoles simultanément :
- Buzzers anciens (TCP port 1234) continuent de fonctionner
- Nouveaux buzzers (WebSocket port 80) se connectent via `/ws/buzzer`
- Le game engine traite les deux types de clients de manière unifiée

## Contraintes techniques

### ESP32-C3 limitations

| Ressource | Disponible | WebSocket client | Reste |
|-----------|------------|------------------|-------|
| **RAM** | ~400 KB | ~30-50 KB | ~350 KB ✅ |
| **Flash** | 4 MB | ~50 KB | ~3.95 MB ✅ |
| **Latence** | TCP: 10-30ms | WS: 15-40ms | +5-10ms acceptable |

### Bibliothèques WebSocket ESP32

| Bibliothèque | Taille | Facilité | Stabilité | Recommandation |
|--------------|--------|----------|-----------|----------------|
| **ArduinoWebsockets** | Léger (~30KB) | ⭐⭐⭐ Facile | ⭐⭐⭐ Bonne | ✅ Recommandé |
| **ESP-IDF WebSocket** | Moyen (~50KB) | ⭐⭐ Complexe | ⭐⭐⭐⭐ Excellente | Alternative si besoin contrôle fin |

## Format des messages WebSocket

### Messages Buzzer → Serveur

```json
// ENROLL (connexion initiale)
{
  "action": "ENROLL",
  "mac": "AA:BB:CC:DD:EE:FF",
  "firmware_version": "2.0.0"
}

// BUZZ (appui bouton)
{
  "action": "BUZZ",
  "mac": "AA:BB:CC:DD:EE:FF",
  "timestamp": 1234567890
}
```

### Messages Serveur → Buzzer

```json
// LED_ON
{
  "action": "LED_ON",
  "color": "green"
}

// LED_OFF
{
  "action": "LED_OFF"
}

// CONFIRM (accusé réception BUZZ)
{
  "action": "CONFIRM",
  "accepted": true
}
```

## Avantages à long terme

### Fonctionnalités futures possibles

Une fois les buzzers sur WebSocket, ces fonctionnalités deviennent triviales :

✅ **Notifications push** : Serveur peut envoyer des messages à tout moment (pas juste en réponse)
✅ **Configuration OTA** : Changer couleur LED, sensibilité bouton sans reflash
✅ **Diagnostics temps réel** : Batterie, signal WiFi, température transmis au serveur
✅ **Firmware OTA via WebSocket** : Upload firmware directement via WebSocket
✅ **Sécurité** : WSS (WebSocket over TLS) pour chiffrement

### Simplification architecture

```
AVANT (v2.x)
  TCP Server :1234 (buzzers)
  HTTP Server :80 (web)
  WebSocket Server :80/ws (admin/TV/VJoueur)
  → 2 protocoles à maintenir

APRÈS (v3.x+)
  HTTP Server :80 (web)
  WebSocket Server :80/ws (admin/TV/VJoueur)
  WebSocket Server :80/ws/buzzer (buzzers)
  → 1 seul protocole unifié
```

## Risques et mitigations

| Risque | Impact | Mitigation |
|--------|--------|------------|
| **Latence accrue** | Buzz moins réactif | Tests de latence, optimisation keep-alive |
| **Instabilité WiFi** | Reconnexions fréquentes | Reconnexion auto + backoff exponentiel |
| **Compatibilité firmware** | Buzzers anciens cassés | Maintien TCP en parallèle (mode hybride) |
| **Mémoire insuffisante** | Crash ESP32-C3 | Benchmark mémoire, choix bibliothèque légère |

## Version cible

**v3.0.0** (breaking change : nouveau protocole optionnel pour buzzers)

## Dépendances

- [ ] SmartConfig doit transmettre l'IP du serveur (déjà spécifié dans `buzzer-wifi-provisioning-smartconfig.md`)
- [ ] Serveur doit supporter identification des buzzers (MAC address)

## Références

- [RFC 6455 - WebSocket Protocol](https://datatracker.ietf.org/doc/html/rfc6455)
- [ArduinoWebsockets Library](https://github.com/gilmaimon/ArduinoWebsockets)
- [ESP-IDF WebSocket Client](https://docs.espressif.com/projects/esp-idf/en/latest/esp32/api-reference/protocols/esp_websocket_client.html)
- [WebSocket vs TCP Performance](https://ably.com/topic/websockets-vs-tcp)
