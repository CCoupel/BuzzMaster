# Protocole TCP/UDP - BuzzClick ↔ Server

Ce document definit le protocole de communication entre les buzzers BuzzClick (ESP32-C3) et le serveur Go BuzzControl.

## Architecture Double Canal

```
┌─────────────┐         TCP (port 1234)          ┌─────────────┐
│  BuzzClick  │ ────────────────────────────────►│   Server    │
│  (ESP32-C3) │                                  │    (Go)     │
│             │◄──────────────────────────────── │             │
└─────────────┘     UDP Broadcast (port 1234)    └─────────────┘
```

| Canal | Direction | Usage |
|-------|-----------|-------|
| **TCP** | BuzzClick → Server | Envoi messages individuels (HELLO, BUTTON, PONG) |
| **UDP Broadcast** | Server → BuzzClick | Reception commandes simultanees (START, STOP, UPDATE) |

## Format des Messages

### Structure JSON (null-terminated)

```json
{
  "ID": "AA:BB:CC:DD:EE:FF",
  "VERSION": "1.209.3",
  "ACTION": "ACTION_NAME",
  "MSG": { ... }
}\0
```

| Champ | Type | Description |
|-------|------|-------------|
| `ID` | string | Adresse MAC du buzzer (identifiant unique) |
| `VERSION` | string | Version firmware BuzzClick |
| `ACTION` | string | Nom de l'action (voir sections suivantes) |
| `MSG` | object | Payload specifique a l'action |

**Important** : Chaque message se termine par un caractere null (`\0`)

---

## Actions BuzzClick → Server (TCP)

### HELLO - Enregistrement initial

Envoye a la connexion TCP pour enregistrer le buzzer.

```json
{
  "ID": "AA:BB:CC:DD:EE:FF",
  "VERSION": "1.209.3",
  "ACTION": "HELLO",
  "MSG": {
    "IP": "192.168.4.100"
  }
}
```

| Champ MSG | Type | Description |
|-----------|------|-------------|
| `IP` | string | Adresse IP du buzzer sur le reseau |

**Reponse serveur** : Broadcast UPDATE avec etat complet du jeu

---

### BUTTON - Appui bouton

Envoye quand un joueur appuie sur un bouton **pendant que le jeu est actif**.

```json
{
  "ID": "AA:BB:CC:DD:EE:FF",
  "VERSION": "1.209.3",
  "ACTION": "BUTTON",
  "MSG": {
    "button": "ROUGE",
    "time": "1234567890"
  }
}
```

| Champ MSG | Type | Valeurs | Description |
|-----------|------|---------|-------------|
| `button` | string | `ROUGE`, `VERT`, `BLEU`, `JAUNE` | Bouton presse |
| `time` | string | Microsecondes | Timestamp local (`micros()`) |

**Contraintes** :
- Envoye uniquement si `isGameStarted == true`
- Fire-and-forget (pas d'ACK)
- Timestamp relatif au boot du buzzer

---

### PONG - Reponse au ping

Envoye en reponse a un PING du serveur.

```json
{
  "ID": "AA:BB:CC:DD:EE:FF",
  "VERSION": "1.209.3",
  "ACTION": "PONG",
  "MSG": {
    "IP": "192.168.4.100"
  }
}
```

| Champ MSG | Type | Description |
|-----------|------|-------------|
| `IP` | string | Adresse IP actuelle du buzzer |

---

## Actions Server → BuzzClick (UDP Broadcast)

### START - Demarrer le jeu

Active les interruptions boutons sur le buzzer.

```json
{
  "ACTION": "START",
  "MSG": {
    "DELAY": 30,
    "TIME": 1234567890123
  }
}
```

| Champ MSG | Type | Description |
|-----------|------|-------------|
| `DELAY` | int | Duree de la question en secondes |
| `TIME` | int64 | Timestamp serveur (microsecondes) |

**Effet BuzzClick** : `isGameStarted = true`, interruptions actives

---

### CONTINUE - Reprendre le jeu

Identique a START, utilise apres une pause.

```json
{
  "ACTION": "CONTINUE",
  "MSG": {}
}
```

**Effet BuzzClick** : `isGameStarted = true`

---

### STOP - Arreter le jeu

Fin de manche, desactive les boutons.

```json
{
  "ACTION": "STOP",
  "MSG": {}
}
```

**Effet BuzzClick** : `isGameStarted = false`, LED couleur equipe fixe

---

### PAUSE - Mettre en pause

Buzzer en pause (apres un buzz).

```json
{
  "ACTION": "PAUSE",
  "MSG": {}
}
```

**Effet BuzzClick** : `isGameStarted = false`, LED grise clignotante

---

### PING - Test de connexion

Demande de reponse PONG.

```json
{
  "ACTION": "PING",
  "MSG": {}
}
```

**Effet BuzzClick** :
1. Envoie PONG via TCP
2. Appelle `resetGame()` pour reinitialiser

---

### UPDATE - Mise a jour configuration

Envoi de l'etat complet du jeu.

```json
{
  "ACTION": "UPDATE",
  "MSG": {
    "GAME": {
      "PHASE": "READY",
      "DELAY": 30,
      "TIME": 1234567890123
    },
    "teams": {
      "team_id": {
        "NAME": "Les Rouges",
        "COLOR": [239, 68, 68],
        "STATUS": "READY"
      }
    },
    "bumpers": {
      "AA:BB:CC:DD:EE:FF": {
        "NAME": "Joueur 1",
        "TEAM": "team_id",
        "STATUS": "READY"
      }
    }
  }
}
```

**Effet BuzzClick** :
1. Met a jour `myCompleteConfig` avec les donnees
2. Extrait la couleur de l'equipe du bumper
3. Met a jour les LEDs avec la couleur equipe
4. Ajuste l'etat selon `PHASE`

---

### UPDATE_TIMER - Mise a jour timer

Mise a jour periodique du temps restant.

```json
{
  "ACTION": "UPDATE_TIMER",
  "MSG": {
    "CURRENT_TIME": 25,
    "TIME": 1234567890123
  }
}
```

| Champ MSG | Type | Description |
|-----------|------|-------------|
| `CURRENT_TIME` | int | Temps restant en secondes |
| `TIME` | int64 | Timestamp serveur |

**Effet BuzzClick** : Met a jour la barre de progression LED

---

### HELLO - Force reconnexion

Demande au buzzer de se reconnecter.

```json
{
  "ACTION": "HELLO",
  "MSG": {}
}
```

**Effet BuzzClick** : Appelle `connectSRV()` pour reconnecter TCP

---

### RESET - Reinitialisation complete

Reset total de la configuration.

```json
{
  "ACTION": "RESET",
  "MSG": {}
}
```

**Effet BuzzClick** : Appelle `resetGame()`, efface `myConfig`

---

## Decouverte de Service (mDNS)

### Service annonce par le serveur

```
Service: _sock._tcp
Hostname: buzzcontrol
Port: 1234
```

### Resolution cote BuzzClick

```cpp
// Recherche du serveur via mDNS
if (MDNS.begin("buzzclick")) {
    IPAddress serverIP = MDNS.queryHost("buzzcontrol");
    // Connexion TCP vers serverIP:1234
}
```

**Timeout** : 30 secondes (puis `ESP.restart()`)

---

## Etats LED selon Phase

| Phase | Couleur | Animation | Pixels |
|-------|---------|-----------|--------|
| INIT | Rouge→Bleu→Vert | Sequence | 0-22 |
| PREPARE | Couleur equipe | Fixe | 0-5 |
| READY | Couleur equipe | Pulse | 0-5 |
| STARTED | Vert→Rouge | Progression | 6-11 |
| PAUSED | Gris (64,64,64) | Clignotement | 12-17 |
| STOPPED | Couleur equipe | Fixe | 0-5 |

---

## Gestion des Erreurs

### Deconnexion TCP

1. Detectee via callback `onDisconnect`
2. Tente reconnexion immediate
3. Si echec apres 5s → `ESP.restart()`

### Buffer Overflow UDP

```cpp
if (BcastJsonBuffer.length() > MAX_BUFFER_SIZE) {
    // Tronque le debut du buffer (garde les messages recents)
    BcastJsonBuffer = BcastJsonBuffer.substring(overflow);
}
```

**MAX_BUFFER_SIZE** : 8192 bytes

### Watchdog

- Timeout : 30 secondes
- Si `loop()` bloque → Reset automatique
- Appeler `esp_task_wdt_reset()` dans les boucles longues

---

## Contraintes de Compatibilite

### Protocole Fige

Le format des messages est **fige** pour assurer la retrocompatibilite :

| Element | Modifiable | Impact |
|---------|------------|--------|
| Format JSON | Non | Tous les buzzers |
| Noms d'actions | Non | Parsing cote buzzer |
| Champs MSG existants | Non | Logique buzzer |
| Nouveaux champs MSG | Oui | Ignores par anciens buzzers |
| Nouvelles actions | Oui | Ignorees par anciens buzzers |

### Ajout de Fonctionnalites

Pour ajouter une nouvelle fonctionnalite :

1. **Serveur d'abord** : Implementer l'action dans le serveur Go
2. **Tester** : Verifier que les anciens buzzers ignorent l'action
3. **Firmware ensuite** : Implementer le handler dans BuzzClick
4. **Deploiement** : Flasher les buzzers un par un

---

## Ports et Configuration

| Service | Port | Protocole |
|---------|------|-----------|
| TCP (messages) | 1234 | TCP |
| UDP (broadcast) | 1234 | UDP |
| Logs debug | 8889 | UDP |
| HTTP (serveur) | 80 | HTTP |

### Configuration WiFi (hardcodee)

```cpp
const char* WIFI_SSID = "buzzmaster";
const char* WIFI_PASSWORD = "BuzzMaster";
```

---

## Diagramme de Sequence - Cycle de Jeu

```
BuzzClick                    Server
    │                           │
    │──── HELLO ───────────────►│  (Connexion initiale)
    │                           │
    │◄─── UPDATE (broadcast) ───│  (Config complete)
    │                           │
    │◄─── PING (broadcast) ─────│  (Verification prets)
    │──── PONG ────────────────►│
    │                           │
    │◄─── START (broadcast) ────│  (Debut question)
    │                           │
    │◄─── UPDATE_TIMER ─────────│  (Mise a jour timer)
    │                           │
    │──── BUTTON ──────────────►│  (Joueur appuie)
    │                           │
    │◄─── PAUSE (broadcast) ────│  (Buzzer en pause)
    │                           │
    │◄─── STOP (broadcast) ─────│  (Fin de manche)
    │                           │
```
