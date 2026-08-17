# Protocole WebSocket Buzzers - BuzzControl v3.0.0

Ce document specifie le protocole WebSocket pour la communication entre les buzzers physiques BuzzClick (ESP32-C3) et le serveur BuzzControl.

---

## Vue d'ensemble

Le serveur BuzzControl expose plusieurs endpoints WebSocket pour différents types de clients.

### Endpoints WebSocket disponibles

| Endpoint | ClientType | Accès | Description |
|----------|-----------|-------|-------------|
| `/ws` ou `/ws/admin` | `admin` | Navigateur admin (`/admin`) | Interface d'administration complète (plein contrôle) |
| `/ws/tv` | `tv` | Affichage TV (`/tv`) | Écran TV plein écran (lecture seule) |
| `/ws/player` | `vplayer` | Joueur virtuel (`/player`) | Interface joueur sur smartphone/tablette |
| `/ws/buzzer` | `buzzer` | Buzzer physique (BuzzClick) | Buzzers ESP32-C3 (TCP v1 ou WebSocket v3) |
| `/ws/anim` | `anim` | Animateur (`/anim`) | Interface animateur sur tablette (v6.2.0) |
| `/ws/logs` | - | Admin (`/admin/logs`) | Logs temps réel (accès admin) |

### Support hybride buzzers physiques (v3.0.0+)

A partir de la version 3.0.0, les buzzers physiques peuvent se connecter soit via le protocole TCP historique (port 1234), soit via WebSocket (port 80, endpoint `/ws/buzzer`).

### Architecture

```
Buzzers BuzzClick (ESP32-C3)
    |
    +-- TCP port 1234          (protocole v1, retrocompatible)
    |   Messages null-terminated (\0)
    |
    +-- WebSocket port 80      (protocole v3, nouveau)
        Endpoint: /ws/buzzer
        Messages JSON standards
    |
Serveur BuzzControl (Go)
    |
    +-- WebSocket port 80 /ws, /ws/admin, /ws/tv, /ws/player, /ws/anim, /ws/logs
        Clients web et interface animateur
```

### Differences TCP vs WebSocket

| Aspect | TCP (v1) | WebSocket (v3) |
|--------|----------|----------------|
| Port | 1234 (custom) | 80 (HTTP standard) |
| Endpoint | Connexion TCP directe | `/ws/buzzer` |
| Format | JSON + `\n\0` (null-terminated) | JSON standard |
| Keep-alive | Pas de heartbeat natif | Ping/Pong WebSocket (30s) |
| Identification | MAC dans champ `ID` | MAC en query param ou champ `ID` |
| Deconnexion | Detection par erreur TCP | Detection par Pong timeout (60s) |
| Firewall | Port custom souvent bloque | Port 80 rarement bloque |

---

## Connexion

### Endpoint

```
ws://<server_ip>/ws/buzzer
```

### Parametres de connexion (optionnels)

| Parametre | Type | Description |
|-----------|------|-------------|
| `mac` | string | Adresse MAC du buzzer (ex: `AA:BB:CC:DD:EE:FF`) |

Exemple avec MAC en query param :
```
ws://192.168.1.100/ws/buzzer?mac=AA:BB:CC:DD:EE:FF
```

Si le parametre `mac` n'est pas fourni dans l'URL, le buzzer peut s'identifier via le champ `ID` de son premier message HELLO.

### Handshake

1. Le buzzer ouvre une connexion WebSocket vers `/ws/buzzer`
2. Le serveur upgrade la connexion HTTP en WebSocket
3. Le buzzer envoie un message `HELLO` avec son adresse MAC et sa version firmware
4. Le serveur enregistre le buzzer et repond avec l'etat du jeu

---

## Format des messages

Tous les messages sont des objets JSON. Contrairement au protocole TCP, **il n'y a pas de terminateur null** (`\0`).

### Structure generale

```json
{
  "ACTION": "<action_name>",
  "ID": "<mac_address>",
  "MSG": { ... },
  "TIME_EVENT": 1234567890,
  "VERSION": "3.0.0",
  "seq": 1
}
```

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| `ACTION` | string | Oui | Nom de l'action |
| `ID` | string | Non | Adresse MAC du buzzer |
| `MSG` | object | Non | Payload specifique a l'action |
| `TIME_EVENT` | int64 | Non | Timestamp en microsecondes |
| `VERSION` | string | Non | Version du firmware |
| `seq` | int | Non | Numero de sequence |

---

## Messages Buzzer vers Serveur

### HELLO (Enregistrement)

Envoye a la connexion pour identifier le buzzer.

```json
{
  "ACTION": "HELLO",
  "ID": "AA:BB:CC:DD:EE:FF",
  "MSG": {
    "VERSION": "3.0.0",
    "NAME": "Buzzer-1",
    "PROTOCOL": "WS",
    "IP": "192.168.1.50"
  }
}
```

| Champ MSG | Type | Description |
|-----------|------|-------------|
| `VERSION` | string | Version du firmware BuzzClick |
| `NAME` | string | Nom du buzzer (optionnel) |
| `PROTOCOL` | string | Protocole de connexion : `"WS"` pour WebSocket (le serveur stocke `"WebSocket"` dans le modele Bumper) |
| `IP` | string | Adresse IP du buzzer (optionnel) |

Le serveur injecte automatiquement le champ `PROTOCOL` ("TCP" ou "WebSocket") dans le modele Bumper en fonction de la source du message.

**Reponse serveur** : Message `HELLO` avec l'etat courant du jeu.

### BUTTON (Appui bouton)

Envoye quand un joueur appuie sur le bouton du buzzer.

```json
{
  "ACTION": "BUTTON",
  "ID": "AA:BB:CC:DD:EE:FF",
  "TIME_EVENT": 1708000123456789,
  "MSG": {
    "button": "A"
  }
}
```

| Champ MSG | Type | Description |
|-----------|------|-------------|
| `button` | string | Bouton appuye : `A`, `B`, `C`, ou `D` |

Le `TIME_EVENT` est en microsecondes et sert au calcul du temps de reaction.

### PONG (Reponse ready-check)

Reponse au PING du serveur, indiquant que le buzzer est pret.

```json
{
  "ACTION": "PONG",
  "ID": "AA:BB:CC:DD:EE:FF"
}
```

---

## Messages Serveur vers Buzzer

### HELLO (Bienvenue)

Envoye en reponse au HELLO du buzzer avec l'etat du jeu.

```json
{
  "ACTION": "HELLO",
  "MSG": { ... }
}
```

### START (Demarrage du jeu)

Indique au buzzer que le jeu demarre et qu'il peut accepter les appuis.

```json
{
  "ACTION": "START",
  "MSG": {
    "DELAY": 30,
    "QUESTION": "question-id"
  }
}
```

### STOP (Arret du jeu)

Indique au buzzer d'arreter d'accepter les appuis.

```json
{
  "ACTION": "STOP",
  "MSG": {}
}
```

### PAUSE (Pause)

Met le buzzer en pause.

```json
{
  "ACTION": "PAUSE",
  "MSG": {}
}
```

### CONTINUE (Reprise)

Reprend le jeu apres une pause.

```json
{
  "ACTION": "CONTINUE",
  "MSG": {}
}
```

### PING (Ready-check)

Demande au buzzer de confirmer qu'il est pret.

```json
{
  "ACTION": "PING",
  "MSG": {}
}
```

### RESET (Reinitialisation)

Reinitialise le buzzer.

```json
{
  "ACTION": "RESET",
  "MSG": {}
}
```

---

## Messages VPlayer vers Serveur

### PLAYER_CONNECT (Connexion VJoueur) — v5.7.20, #109

Envoye par un joueur virtuel (VPlayer) lors de l'enrôlement (`EnrollPage`) ou de la reconnexion (`VPlayerPage`).

**Endpoint** : `/ws/player`

**Payload** :
```json
{
  "ACTION": "PLAYER_CONNECT",
  "MSG": {
    "NAME": "Alice",
    "ID": "f8d7c6b5-a4e9-4d2e-b1a0-9f8e7d6c5b4a"
  }
}
```

| Champ MSG | Type | Obligatoire | Description |
|-----------|------|-------------|-------------|
| `NAME` | string | ✅ | Nom du joueur virtuel |
| `ID` | string | ❌ | Identifiant unique pour reconnexion (v5.7.20, #109 R1) — UUID ou hash stocké en localStorage |

**Comportement serveur** :

#### Cas 1 : Reconnexion par ID (ID résolu + bumper existe)
- Reutilise le bumper existant
- Rafraîchit le nom si différent (`NAME` peut changer)
- Marque `Connected=true`, `ConnEventReconnect`
- Retour : `PLAYER_CONNECTED` avec préservation équipe/score

#### Cas 2 : ID périmé (ID fourni mais non résolu)
- Fallback sur identification par nom
- Peut rejeter si nom en conflit

#### Cas 3 : Nom déjà pris (VJoueur connecté ou déconnecté)
- **Rejette avec `PLAYER_REJECTED`** — aucune suppression/fusion
- Raison : `NAME_TAKEN`
- Le bumper existant ne change pas : équipe, score, connexion préservés
- Le client reçoit un écran d'erreur (`VPlayerPage.jsx`) avec redirection auto 3s

#### Cas 4 : Nouvelle inscription (ID absent, nom libre)
- Crée un nouveau bumper `IS_VIRTUAL=true`
- `Connected=true`, `ConnEventReconnect`
- Retour : `PLAYER_CONNECTED`, attente assignation équipe

**Réponse serveur** :

Succès :
```json
{
  "ACTION": "PLAYER_CONNECTED",
  "MSG": {
    "PLAYER_ID": "f8d7c6b5-a4e9-4d2e-b1a0-9f8e7d6c5b4a",
    "TEAM": "Les Rouges",
    "SCORE": 30
  }
}
```

Rejet (nom pris) :
```json
{
  "ACTION": "PLAYER_REJECTED",
  "MSG": {
    "PLAYER_ID": "f8d7c6b5-a4e9-4d2e-b1a0-9f8e7d6c5b4a",
    "REASON": "NAME_TAKEN"
  }
}
```

| Raison rejet | Description |
|---|---|
| `NAME_TAKEN` | Nom déjà assigné à un autre bumper (connecté ou déconnecté) |

**Frontend** (`VPlayerPage.jsx`) :
- Affiche un écran bloquant `PLAYER_REJECTED` avec le motif
- Bouton "Rejoindre à nouveau" pour relancer l'enrôlement
- Redirection auto vers `/enroll` après 3s (`RECONNECT_ERROR_REDIRECT_DELAY_MS`)

---

### ARDOISE_INPUT (Saisie ARDOISE) — v5.6.0

Envoye par un joueur virtuel (VPlayer) saisissant une réponse libre en ARDOISE.

**Endpoint** : `/ws/player`

**Phase valide** : `STARTED` uniquement — le serveur ignore les entrées en dehors de cette phase.

**Payload** :
```json
{
  "ACTION": "ARDOISE_INPUT",
  "MSG": {
    "TEXT": "Paris"
  }
}
```

| Champ MSG | Type | Description |
|-----------|------|-------------|
| `TEXT` | string | Réponse saisie par le joueur |

**Comportement serveur** :

1. **Guard phase** : Si phase ≠ `STARTED` ou question.TYPE ≠ `ARDOISE`, ignorer silencieusement
2. **Identification équipe** : Via protocole natif VPlayer — le serveur connaît déjà l'équipe du joueur
3. **Mise à jour GameState** : 
   ```go
   GameState.ARDOISE_ANSWERS[teamName] = ArdoiseAnswer{
       TEXT: "Paris",
       SUBMITTED_AT: timeNowMicroseconds()
   }
   ```
4. **Broadcast UPDATE** : Envoie un message `UPDATE` avec `ARDOISE_ANSWERS` à tous les clients web (`/ws`, `/ws/admin`, `/ws/tv`)

**Stratégie d'envoi côté VPlayer** :

- **Affichage local immédiat** : L'input apparaît dans le champ de saisie sans attendre la réponse du serveur
- **Throttling ~200ms** : Envoyer le texte complet au serveur tous les ~200ms (pas de delta, texte entier)
- **Envoi forcé sur STOP/PAUSE** : Dès que le serveur envoie `STOP` ou `PAUSE`, forcer un envoi immédiat du texte en cours
- **Aucun envoi post-REVEALED** : Une fois `REVEALED` reçu du serveur, ne plus envoyer d'inputs (clavier verrouillé)

---

### LED_ON / LED_OFF (Controle LED)

Allume ou eteint la LED du buzzer.

```json
{
  "ACTION": "LED_ON",
  "MSG": {
    "color": "green"
  }
}
```

```json
{
  "ACTION": "LED_OFF",
  "MSG": {}
}
```

Couleurs disponibles : `green`, `red`, `blue`, `yellow`, `orange`, `white`.

---

## Keep-alive

Le serveur envoie un **ping WebSocket natif** toutes les **30 secondes**. Si le buzzer ne repond pas avec un pong dans les **60 secondes**, la connexion est consideree comme perdue et le client est deconnecte.

| Parametre | Valeur |
|-----------|--------|
| Ping interval | 30 secondes |
| Read deadline | 60 secondes |
| Write deadline | 10 secondes |
| Read limit | 65536 octets |

---

## Reconnexion

En cas de deconnexion, le buzzer doit :

1. Attendre un delai (backoff exponentiel recommande : 1s, 2s, 4s, 8s, max 30s)
2. Se reconnecter a `/ws/buzzer`
3. Renvoyer un message `HELLO` pour se reenregistrer

Le serveur detecte automatiquement la deconnexion via le mecanisme de ping/pong WebSocket et nettoie les ressources associees.

---

## Suivi des protocoles

### Champ PROTOCOL sur Bumper

Le serveur stocke le protocole de connexion de chaque buzzer dans le champ `PROTOCOL` du modele `Bumper` :

| Valeur | Description |
|--------|-------------|
| `"TCP"` | Buzzer connecte via TCP port 1234 (defaut, retrocompatible) |
| `"WebSocket"` | Buzzer connecte via WebSocket `/ws/buzzer` |
| `""` (vide) | Ancien buzzer sans protocole specifie (traite comme TCP) |

### Broadcast CLIENTS

Quand un buzzer se connecte ou se deconnecte, le serveur broadcast un message `CLIENTS` avec les compteurs mis a jour :

```json
{
  "ACTION": "CLIENTS",
  "MSG": {
    "ADMIN_COUNT": 2,
    "TV_COUNT": 1,
    "VPLAYER_COUNT": 3,
    "BUZZER_TCP_COUNT": 4,
    "BUZZER_WS_COUNT": 2
  }
}
```

| Champ | Type | Description |
|-------|------|-------------|
| `ADMIN_COUNT` | int | Nombre de clients admin connectes |
| `TV_COUNT` | int | Nombre d'ecrans TV connectes |
| `VPLAYER_COUNT` | int | Nombre de joueurs virtuels connectes |
| `BUZZER_TCP_COUNT` | int | Nombre de buzzers connectes via TCP |
| `BUZZER_WS_COUNT` | int | Nombre de buzzers connectes via WebSocket |

---

## Architecture serveur

### Hubs separes

Le serveur utilise des hubs WebSocket distincts pour isoler les types de clients :

| Hub | Endpoint | Clients |
|-----|----------|---------|
| `WebSocketHub` | `/ws` | Admin, TV, VJoueur |
| `BuzzerWebSocketHub` | `/ws/buzzer` | Buzzers physiques |
| `LogsWebSocketHub` | `/ws/logs` | Logs temps reel |

### BuzzerWebSocketHub

Le hub dedie aux buzzers gere :

- **Enregistrement/desenregistrement** des clients buzzer
- **Broadcast** de messages a tous les buzzers connectes
- **Envoi cible** a un buzzer specifique (par MAC)
- **Comptage** des buzzers connectes
- **Callback** `OnBuzzerChange` quand un buzzer se connecte/deconnecte
- **Canal Incoming** (capacite 100) pour les messages recus des buzzers

### Flux de message

```
Buzzer ESP32 --[WebSocket]--> BuzzerWebSocketHub.readPump()
    --> parse JSON (protocol.ParseSingle)
    --> canal Incoming
    --> Game Engine (handleHello, handleButton, handlePong)
    --> resultat
    --> BuzzerWebSocketHub.Broadcast() ou SendToClient()
    --> writePump() --> Buzzer ESP32
```

---

## Serialisation

### WebSocket (v3)

Les messages WebSocket utilisent `SerializeForWebSocket()` qui produit du JSON standard sans terminateur :

```go
func (m *Message) SerializeForWebSocket() ([]byte, error) {
    return json.Marshal(m)
}
```

### TCP (v1, retrocompatible)

Les messages TCP utilisent `Serialize()` qui ajoute un terminateur `\n\0` :

```go
func (m *Message) Serialize() ([]byte, error) {
    data, err := json.Marshal(m)
    // ...
    return append(data, '\n', 0), nil
}
```

### ParseSingle (bidirectionnel)

La fonction `ParseSingle()` gere les deux formats en nettoyant les terminateurs avant le parsing :

```go
func ParseSingle(data []byte) (*Message, error) {
    data = bytes.TrimRight(data, "\x00\n\r ")
    data = bytes.TrimSpace(data)
    // ...
    return &msg, json.Unmarshal(data, &msg)
}
```

---

## Guide de migration TCP vers WebSocket

### Client firmware (click_websocketClient.h)

Le firmware v3.0.0 inclut un client WebSocket pret a l'emploi dans `src/BuzzClick/click_websocketClient.h`. Il est active par le flag de compilation `USE_WEBSOCKET` dans `platformio.ini`.

**Caracteristiques** :
- Bibliotheque ArduinoWebsockets (~30KB Flash)
- Connexion a `ws://<server_ip>:<port>/ws/buzzer`
- Reconnexion automatique avec backoff exponentiel (1s, 2s, 4s, 8s max)
- Messages JSON : HELLO, BUTTON, PONG
- LED indicateurs : Vert (connecte), Jaune clignotant (reconnexion), Rouge (deconnecte)

**Pour activer WebSocket sur un buzzer** :
1. Ajouter `-DUSE_WEBSOCKET` dans `build_flags` de `platformio.ini`
2. Compiler et flasher le firmware
3. Le buzzer se connecte automatiquement en WebSocket au lieu de TCP

### Pour le serveur

Le serveur v3.0.0 gere deja les deux protocoles. Aucune modification serveur n'est necessaire lors de la migration des buzzers.

### Ordre de migration recommande

1. Mettre a jour le serveur vers v3.0.0 (supporte TCP + WebSocket)
2. Flasher progressivement les buzzers avec le nouveau firmware WebSocket
3. Verifier que tous les buzzers fonctionnent en WebSocket
4. (Futur v4.0+) Deprecier le protocole TCP

---

---

## Endpoints WebSocket dédiés (v3.8.0)

| Endpoint | Type client | Sérialiseur |
|----------|-------------|-------------|
| `/ws/admin` | Admin web | `SerializeForAdmin()` — full payload |
| `/ws/tv` | TV display | `SerializeForWebClient()` — réduit ~40-60% |
| `/ws/player` | VPlayer | `SerializeForWebClient()` — réduit |
| `/ws` | Legacy alias | → `/ws/admin` (rétrocompat) |
| `/ws/buzzer` | Buzzers physiques | `SerializeForBuzzer()` — minimal |

**Table de routage des actions** :

| Action | Admin | TV | VPlayer | Buzzer |
|--------|-------|----|---------|----|
| ALL, UPDATE, START, STOP, PAUSE, CONTINUE, REVEAL, RESET, QCM_HINT, ENROLLMENT_UPDATE | ✓ | ✓ | ✓ | ✓ |
| UPDATE_TIMER | ✓ | ✓ | ✓ | ✓ |
| READY, REMOTE, BACKGROUND_CHANGE, CONFIG_UPDATE, SHOW_QR_CODE, HIDE_QR_CODE, FULL, MEMORY_*, MEMOTION_* | ✓ | ✓ | - | - |
| QUESTIONS, CLIENTS, FIRMWARE_VERSION | ✓ | - | - | - |
| PLAYER_REJECTED, PLAYER_CONNECTED, PLAYER_ASSIGNED | ✓ | - | ✓ | - |
| LED_SET, OTA_UPDATE, WIFI_CONFIG, HELLO | ✓ | - | - | ✓ |
| ARDOISE_INPUT | - | - | ✓ (send) | - |

**Note sur MEMOTION_*** : Les actions listées ci-dessus (`MEMOTION_*`) sont des actions **SORTANTES** (serveur → client). Les actions **ENTRANTES** acceptées (client → server) sont documentées dans `contracts/websocket-actions.md` §"Sécurité — Allow-list entrante par ClientType" — depuis v6.2.0 (#160), les 5 actions `MEMOTION_SELECT`, `MEMOTION_FLIP`, `MEMOTION_STOP_TIMER`, `MEMOTION_REVEAL`, `MEMOTION_DONE` sont désormais acceptées depuis `/ws/anim` (interface animateur tablette, v6.2.0+), en plus de leurs sources existantes.

**Sérialiseurs** :
- `SerializeForAdmin()` : full (bumpers avec FIRMWARE_VERSION, OTA_STATUS, ACK_PENDING, config)
- `SerializeForWebClient()` : strips FIRMWARE_VERSION, IS_OUTDATED, OTA_STATUS, OTA_PERCENT, ACK_PENDING, config
- `SerializeForBuzzer()` : phase, timer, bumpers (ID, NAME, TEAM, CONNECTED), teams (NAME, COLOR, STATUS)

**Backend** : `websocket.go` — `HandleConnectionWithType()`, `BroadcastToTypes()`, `BroadcastRawToTypes()`.
**Frontend** : `GameProvider` accepte prop `endpoint` (défaut `/ws/admin`), transmis à `useWebSocket()`. App.jsx : `/tv` → `/ws/tv`, `/player`/`/enroll` → `/ws/player`.

---

## Protocole ACK Buzzer (v3.8.0)

```json
{ "ACTION": "LED_SET", "MSG_ID": "a1b2c3d4e5f6", "MSG": { ... } }
{ "ACTION": "ACK", "MSG": { "ack_action": "LED_SET", "ack_id": "a1b2c3d4e5f6" } }
```

- `AckManager` : registre MSG_ID, retry auto + expiry après `ack_max_retries` (défaut 3, timeout 2000ms)
- `MSG_ID` : `omitempty` — anciens firmwares sans ACK restent compatibles
- ACK envoyé BEFORE action apply (firmware) — minimise latence
- Fichier : `internal/server/ack_manager.go`

## Contraintes ESP32-C3

| Ressource | Disponible | Client WebSocket | Reste |
|-----------|------------|------------------|-------|
| RAM | ~400 KB | ~30-50 KB | ~350 KB |
| Flash | 4 MB | ~50 KB | ~3.95 MB |
| Latence | TCP: 10-30ms | WS: 15-40ms | +5-10ms acceptable |

---

## Action UPDATE_QUIZ_META (v6.0.0, Batch 2b #137)

Mise à jour des métadonnées globales du quiz (thème, populations cibles, difficultés, objectifs, affichage TV).

### Payload

```json
{
  "ACTION": "UPDATE_QUIZ_META",
  "MSG": {
    "NAME": "Quiz Cinéma 80s",
    "THEME": "Cinéma français des années 80",
    "NOTES": "Soirée entre amis",
    "POPULATIONS": ["Adulte (18-64 ans)", "Senior (65+)"],
    "DIFFICULTIES": ["Moyen", "Difficile"],
    "OBJECTIVES": "Questions sur films de réalisateurs femmes",
    "HIDDEN_FIELDS": ["ANSWER"],
    "LANGUAGE": "Français"
  }
}
```

### Champs

| Champ | Type | Obligatoire | Description | Depuis |
|-------|------|-------------|-------------|--------|
| `NAME` | string | ❌ | Titre du quiz | v4.0.0 |
| `THEME` | string | ❌ | Thème principal | v4.0.0 |
| `NOTES` | string | ❌ | Notes supplémentaires | v4.0.0 |
| `POPULATIONS` | []string | ❌ | Populations cibles (tableau, ex: `["Adulte", "Senior"]`) | v6.0.0 #8, plural depuis Batch 2b #137 |
| `DIFFICULTIES` | []string | ❌ | Difficultés visées (tableau, ex: `["Facile", "Moyen"]`) | v6.0.0 #8, plural depuis Batch 2b #137 |
| `OBJECTIVES` | string | ❌ | Objectifs/thème pédagogique (jamais diffusé à `/ws/tv` ou `/ws/player`) | Batch 2b #137 |
| `HIDDEN_FIELDS` | []string | ❌ | Champs question à masquer sur TV (ex: `["ANSWER"]`) | Batch 2b #137 |
| `LANGUAGE` | string | ❌ | Langue du quiz (Français par défaut) | v6.0.0 |

### Sémantique — **CRITIQUE**

**« Champ absent = inchangé »** (par champ)

- Un champ **absent** du payload ne modifie **pas** la valeur stockée — elle reste inchangée
- Un champ **présent** remplace la valeur stockée (même si vide ou tableau vide `[]`)
- Cette sémantique garantit la rétrocompatibilité : les clients antérieurs (v4.0.0–v5.x) envoyant seulement `NAME`/`THEME`/`NOTES` ne supprimaient pas accidentellement les nouveaux champs

**Exemples** :

```json
// Scénario 1 : Update seulement NAME (v5.x client)
{ "ACTION": "UPDATE_QUIZ_META", "MSG": { "NAME": "Nouveau titre" } }
→ NAME change, THEME/NOTES/POPULATIONS/DIFFICULTIES/OBJECTIVES/HIDDEN_FIELDS/LANGUAGE inchangés

// Scénario 2 : Update NAME + POPULATIONS (v6.1.0+ client, pluriel)
{ "ACTION": "UPDATE_QUIZ_META", "MSG": { "NAME": "...", "POPULATIONS": ["Adulte", "Senior"] } }
→ NAME et POPULATIONS changent, autres champs inchangés

// Scénario 3 : Effacer un champ (le passer à vide)
{ "ACTION": "UPDATE_QUIZ_META", "MSG": { "NOTES": "", "OBJECTIVES": "", "HIDDEN_FIELDS": [] } }
→ NOTES, OBJECTIVES, HIDDEN_FIELDS deviennent vides, autres champs inchangés

// Scénario 4 : Ancien format singulier (dépréciée Batch 2b)
{ "ACTION": "UPDATE_QUIZ_META", "MSG": { "POPULATION": "Adulte" } }
→ Rejeté ou ignoré par serveur v6.1.0+ (utiliser "POPULATIONS" pluriel)
```

### Diffusion par endpoint

**`/ws/admin`** : Message `UPDATE` avec **tous les champs** incluant `OBJECTIVES` (full payload)

**`/ws/tv` et `/ws/player`** : Message `UPDATE` avec **tous les champs sauf `OBJECTIVES`** (jamais transmis, confidentialité pédagogique). Reçoivent `HIDDEN_FIELDS` pour filtrage côté rendu.

Le serveur émet toujours le GameState complet (jamais de payload partiel) à chaque `UPDATE_QUIZ_META`, sérialisé sans `omitempty` sur tous les champs.

### Affichage

- **QuestionsPage** : Les 6 champs éditables dans la section "Quiz" en haut de page
- **TV NEW_GAME** : Affiche les 3 nouveaux champs uniquement s'ils sont **non-vides**, en ligne compacte de badges

---

## Action AI_GENERATION_PROGRESS (v6.1.1, post-QUALIF #137)

Progression d'une génération de questions via l'API Claude (Anthropic) ou Groq (gratuit).

### Endpoint

`/ws/admin` uniquement — jamais `/ws/tv` ou `/ws/player`.

### Payload

```json
{
  "ACTION": "AI_GENERATION_PROGRESS",
  "MSG": {
    "STATE": "RUNNING",
    "BATCH_NUMBER": 2,
    "TOTAL_BATCHES": 5,
    "CREATED_COUNT": 40,
    "SKIPPED_COUNT": 0,
    "ERROR_CODE": "",
    "ERROR_MESSAGE": ""
  }
}
```

### Champs

| Champ | Type | Description | États possibles |
|-------|------|-------------|-----------------|
| `STATE` | string | État actuel du job | `RUNNING`, `DONE`, `FAILED`, `CANCELLED` |
| `BATCH_NUMBER` | int | Numéro du lot en cours (base 1) | ≥ 1 |
| `TOTAL_BATCHES` | int | Nombre total de lots prévu | ≥ 1 |
| `CREATED_COUNT` | int | Nombre de questions créées jusqu'ici | ≥ 0 |
| `SKIPPED_COUNT` | int | Nombre de questions rejetées/invalides | ≥ 0 |
| `ERROR_CODE` | string | Code d'erreur stable (machine-friendly) | Vide, ou `no_api_key`, `api_key_rejected`, `quota_exceeded`, `upstream_error`, `network_error`, etc. — vide si `STATE != FAILED` |
| `ERROR_MESSAGE` | string | Message d'erreur réel du provider, assaini (clés API masquées) | Vide sauf `STATE = FAILED` — exemple Groq #142 (avant correction) : `"discriminator: multiple candidate properties CATEGORY, DIFFICULTY, TYPE [discriminator_multiple_candidates]"` |

### Sémantique

- **STATE transitions** : `RUNNING` → (`DONE` \| `FAILED` \| `CANCELLED`)
- **ERROR_MESSAGE** :
  - **Absent** (`omitempty`) quand `STATE != FAILED` (pas de message d'erreur généré)
  - **Présent** quand `STATE = FAILED` — message texte réel du provider (Anthropic ou Groq API response body) avec :
    - Clés API filtrées (regex `sk-ant-…` / `gsk_…` → `[redacted]`)
    - Troncature UTF-8 safe si > 500 runes
    - Jamais exposé à la TV ou VPlayer (endpoint `/ws/admin` uniquement)

### Exemple de flux complet

**Succès** :
```json
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "STATE": "RUNNING", "BATCH_NUMBER": 1, "TOTAL_BATCHES": 5, "CREATED_COUNT": 0, "SKIPPED_COUNT": 0 } }
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "STATE": "RUNNING", "BATCH_NUMBER": 2, "TOTAL_BATCHES": 5, "CREATED_COUNT": 40, "SKIPPED_COUNT": 0 } }
...
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "STATE": "DONE", "BATCH_NUMBER": 5, "TOTAL_BATCHES": 5, "CREATED_COUNT": 200, "SKIPPED_COUNT": 0, "ERROR_CODE": "", "ERROR_MESSAGE": "" } }
```

**Erreur avec détail** (Groq #142, post-QUALIF v6.1.1) :
```json
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "STATE": "RUNNING", "BATCH_NUMBER": 1, "TOTAL_BATCHES": 5, "CREATED_COUNT": 0, "SKIPPED_COUNT": 0 } }
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "STATE": "FAILED", "BATCH_NUMBER": 1, "TOTAL_BATCHES": 5, "CREATED_COUNT": 0, "SKIPPED_COUNT": 0, "ERROR_CODE": "upstream_error", "ERROR_MESSAGE": "invalid JSON schema for response_format: 'buzz_questions': /properties/questions/items/anyOf: anyOf disambiguation failed: anyOf: discriminator: multiple candidate properties CATEGORY, DIFFICULTY, TYPE [discriminator_multiple_candidates]" } }
```

Frontend affiche :
- Message générique : `"Le service IA a renvoyé une erreur"`
- Panneau détail technique (repliable) : Le message exact du provider

**Arrêt utilisateur** :
```json
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "STATE": "RUNNING", "BATCH_NUMBER": 2, "TOTAL_BATCHES": 5, "CREATED_COUNT": 40, "SKIPPED_COUNT": 0 } }
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "STATE": "CANCELLED", "BATCH_NUMBER": 2, "TOTAL_BATCHES": 5, "CREATED_COUNT": 40, "SKIPPED_COUNT": 0, "ERROR_CODE": "", "ERROR_MESSAGE": "" } }
```

Frontend affiche un panneau de résumé (40 questions créées, arrêt gracieux).

### Diffusion et visibilité

- **`/ws/admin`** : Reçoit tous les messages et tous les champs (incluant `ERROR_MESSAGE`)
- **`/ws/tv` et `/ws/player`** : N'reçoivent JAMAIS `AI_GENERATION_PROGRESS` (action absente de la table de routage)

### Affichage admin (QuestionsPage)

**Phase RUNNING** :
- Barre de progression : `BATCH_NUMBER / TOTAL_BATCHES`
- Compteurs : `CREATED_COUNT` questions, `SKIPPED_COUNT` rejetées
- Bouton **"Arrêter"** actif

**Phase DONE** :
- Message succès avec total créé
- Bouton **"Nouvelle génération"** (v6.1.1) — relance le formulaire
- Bouton **"Fermer"** — ferme la modale

**Phase FAILED** :
- Message générique d'erreur (dérivé de `ERROR_CODE`)
- Panneau repliable **"Détail technique"** affichant `ERROR_MESSAGE` (si présent)
- Bouton **"Réessayer"** — corrigez/relancez
- Bouton **"Fermer"**

**Phase CANCELLED** :
- Message résumé (X questions créées avant arrêt)
- Bouton **"Nouvelle génération"** (v6.1.1)
- Bouton **"Fermer"**

---

## Action NEXT_QUESTION (v6.2.0, #155 — enrichi v6.2.x #166)

### Endpoint

- **Direction** : Server → Client
- **Cible** : `ClientTypeAnim` **exclusivement** (`/ws/anim`), via `BroadcastToTypes(msg, ClientTypeAnim)`
- **Trigger** : HELLO d'un client animateur (état initial), `READY`, `STOP`, `REVEAL`, `NEW_GAME`,
  `REORDER_QUESTIONS`, `DELETE` (suppression d'une question)

Indique à l'interface animateur (`/anim`) la prochaine question jouable ainsi que la progression
dans le quiz, pour permettre l'enchaînement (bouton « à suivre », `AnimNextButton`) et l'affichage
`n/total` sans consulter `/admin`. Détail complet du contrat :
`contracts/websocket-actions.md` §"Animateur".

### Payload

```json
{
  "ID": "42",
  "QUESTION": "Qui a peint la Nuit étoilée ?",
  "CATEGORY": "Art",
  "TYPE": "QCM",
  "POINTS": 20,
  "TIME": 30,
  "CURRENT_POSITION": 5,
  "TOTAL_QUESTIONS": 82
}
```

| Champ | Type | Description |
|-------|------|-------------|
| `ID` | string | ID de la question suivante |
| `QUESTION` | string | Texte de la question suivante |
| `CATEGORY` | string | Catégorie de la question suivante |
| `TYPE` | string | Type de la question suivante (SPEEDY, QCM, MEMORY, MEMOTION, ARDOISE) |
| `POINTS` | int | Points de base de la question suivante |
| `TIME` | int | Durée en secondes de la question suivante |
| `CURRENT_POSITION` | int | **v6.2.x (#166)** — rang de la question **courante** dans la liste triée par `ORDER` (1-based). `0` si aucune question courante, ou si elle ne figure plus dans la liste. |
| `TOTAL_QUESTIONS` | int | **v6.2.x (#166)** — nombre total de questions du quiz (longueur de la liste triée). |

**Le payload n'est plus « tout ou rien » depuis #166.** Les champs de la question suivante (`ID`,
`QUESTION`, `CATEGORY`, `TYPE`, `POINTS`, `TIME`) sont tous absents/zéro-valeur si plus aucune
question n'est jouable (fin de quiz) — inchangé depuis #155/#156. En revanche `CURRENT_POSITION`
et `TOTAL_QUESTIONS` sont désormais renseignés **sans `omitempty`, y compris quand il n'y a plus de
question suivante** : sans cette dissociation, la progression affichée par `/anim` (`n/total`)
disparaîtrait exactement sur la dernière question du quiz — le moment où « 82/82 » est le plus
utile. Un client détecte l'absence de question suivante en testant `ID` (vide), jamais en testant
la présence du payload entier.

### Règle de calcul (révisée v6.2.x #166 — GATE 2 D2)

Port fidèle de `GamePage.jsx`'s `nextUnplayedQuestion`, avec une divergence assumée sur le premier
point :

1. **Aucune question courante** (`GameState.QUESTION` absent — ENROLL, `NEW_GAME` juste après
   reset) → **la recherche démarre à l'indice 0 et renvoie la première question jouable du quiz**,
   au lieu d'un payload vide. Divergence délibérée avec `/admin` : `GamePage.jsx`'s
   `nextUnplayedQuestion` renvoie `null` dans ce cas (`if (!currentId) return null`) — la régie ne
   propose toujours rien tant qu'aucune question n'a été sélectionnée. `/anim` gagne ainsi un point
   d'entrée pour démarrer une partie depuis la tablette sans passer par `/admin`, dont le
   comportement propre reste inchangé. Voir `contracts/CHANGELOG.md` `[20260815-2]`.
2. Toutes les questions triées par `ORDER` (repli sur `ID` numérique si `ORDER` absent) — même
   règle que `web/src/utils/questionOrder.js`.
3. Position de la question **courante** localisée dans cette liste triée ; si introuvable (question
   supprimée du disque mais toujours courante côté moteur, ou absente per point 1), la recherche
   démarre à l'indice 0.
4. Première question **strictement après** cette position dont le statut n'est pas déjà joué —
   jamais de retour en arrière dans la liste.
5. Rien trouvé après cette position → champs de la question suivante à zéro-valeur (fin de quiz) ;
   `CURRENT_POSITION`/`TOTAL_QUESTIONS` restent renseignés (point ci-dessus).

### Exemple — fin de quiz (dernière question)

```json
{
  "ID": "",
  "QUESTION": "",
  "CATEGORY": "",
  "TYPE": "",
  "POINTS": 0,
  "TIME": 0,
  "CURRENT_POSITION": 82,
  "TOTAL_QUESTIONS": 82
}
```

---

## Action AWARDED_TEAMS (v6.2.x, #170)

### Endpoint

- **Direction** : Server → Client
- **Cible** : `ClientTypeAnim` **exclusivement** (`/ws/anim`), via `BroadcastToTypes(msg, ClientTypeAnim)` — comme `NEXT_QUESTION`. `/admin`, `/tv`, `/player` et `/buzzer` ne reçoivent jamais cette action.
- **Trigger** : `TEAM_POINTS` et `BUMPER_POINTS` traités (origine `/admin` ou `/anim` indifféremment), `READY` (changement de question), `NEW_GAME`, `RAZ`, HELLO d'un client animateur (état initial)

Informe les interfaces animateur des équipes **déjà créditées pour la question en cours**. Sert
deux besoins : confirmer à **tous** les animateurs qu'un crédit vient d'avoir lieu (équipe +
montant), et leur permettre de **bloquer** un second crédit sur la même équipe — que le premier
crédit vienne d'une autre tablette ou de la régie. Détail complet du contrat :
`contracts/websocket-actions.md` §"Animateur" → `AWARDED_TEAMS`.

**Périmètre : tous les modes de jeu conduits depuis `/anim`** (SPEEDY, QCM déjà livrés — #156/
#157 — et les modes à venir), pas seulement ARDOISE.

### Payload

```json
{
  "QUESTION_ID": "42",
  "TEAMS": [
    { "TEAM": "Les Bleus", "POINTS": 20, "TIMESTAMP": 1755341200000000 },
    { "TEAM": "Les Rouges", "POINTS": 0, "TIMESTAMP": 1755341205000000 }
  ]
}
```

| Champ | Type | Description |
|-------|------|-------------|
| `QUESTION_ID` | string | ID de la question courante — `""` si aucune. Permet au client de rejeter un payload obsolète arrivé après un changement de question |
| `TEAMS` | array | Une entrée par équipe créditée, dans l'ordre chronologique du premier crédit — **`[]`, jamais `null`**, quand aucune équipe n'a encore été créditée |
| `TEAMS[].TEAM` | string | Nom de l'équipe |
| `TEAMS[].POINTS` | int | **Somme** des points attribués à cette équipe pour la question courante (une équipe peut être créditée deux fois depuis la régie, qui n'a aucune garde) |
| `TEAMS[].TIMESTAMP` | int64 | Horodatage µs du **premier** crédit de cette équipe pour la question courante |

> ⚠️ **`POINTS` peut valoir `0` — un montant nul est un verrou valide, pas « pas encore traité ».**
> Le refus d'une réponse (#170, geste « 0 pt ») est un crédit ordinaire à montant nul : il produit
> une entrée `TEAMS[]` comme tout autre crédit. **Un client ne doit jamais tester la présence d'un
> verrou par la véracité du montant** (`if (points)` est faux pour un refus en JS, `0` étant
> falsy) — le test correct porte sur la **présence de l'entrée** dans `TEAMS[]` pour cette équipe.
> Voir `contracts/websocket-actions.md` §"AWARDED_TEAMS" pour l'exemple de code correct/incorrect.

### Source de vérité — projection de l'historique existant, pas un nouvel état

Aucun nouveau champ de `GameState`, aucune structure persistée créée ou modifiée. Le payload est
une **projection** de l'historique déjà tenu par le moteur (`Engine.history`, `AddGameEvent`,
persisté par `SaveHistory`, exposé par `GET /history`, consommé aussi par le PALMARÈS — voir
`docs/DATA_MODELS.md` §"GameEvent (History)"). `handleTeamPoints` et `handleBumperPoints`
enregistrent déjà cet événement, quelle que soit l'interface d'origine du crédit :

```
événements tels que  EventType == "POINTS_AWARDED"
                 ET  QuestionID == GameState.Question.ID
                 ET  Timestamp  >= GameState.GameTime
regroupés par TeamName ; POINTS = somme, TIMESTAMP = min
```

- Regroupement par **`TeamName`**, pas `WinnerID` : `WinnerID` porte une MAC quand le crédit vise
  un joueur (SPEEDY) ; `TeamName` est toujours renseigné — une équipe créditée via l'un de ses
  joueurs compte donc comme créditée.
- Le filtre `Timestamp >= GameState.GameTime` est **indispensable** : l'historique n'est purgé
  qu'à `NEW_GAME`/`RAZ`, pas entre deux questions. Sans lui, rejouer une question déjà créditée la
  laisserait bloquée indéfiniment.
- Projection recalculée **uniquement sur les déclencheurs listés ci-dessus, jamais** sur le chemin
  de `broadcastUpdate` — même discipline que `NEXT_QUESTION` : un parcours de l'historique à
  chaque tick de chronomètre est un coût qui grandirait avec la partie.

### `/admin` n'est pas contraint

La régie n'est pas destinataire de cette action et **ne reçoit aucune garde** : elle peut
créditer, recréditer, surcharger — état de fait préexistant, inchangé par #170 (il n'existe
aujourd'hui aucune garde de double-crédit côté `GamePage.jsx` ni côté `Engine.UpdateTeamScore`).
Le blocage est une règle de l'**interface animateur uniquement**, appliquée côté client React à
partir de ce payload — jamais côté serveur.

---

## Liste blanche entrante — `FLIP_MEMORY_CARD` accepte désormais `anim` (v6.2.x, #159)

Le serveur applique une liste blanche par `ClientType` sur les actions **entrantes**
(`Client→Server`) : `internal/server/inbound_allowlist.go` (`IsActionAllowed`) rejette (avec log
`WARN`) toute action envoyée par un type de client non autorisé, avant tout traitement. Détail
complet et table exhaustive : `contracts/websocket-actions.md` §"Sécurité — Allow-list entrante
par ClientType".

`FLIP_MEMORY_CARD` (retournement d'une carte en question de type MEMORY) était jusqu'ici autorisé
pour `tv` (aperçu admin en iframe `/tv?admin=true` **et** clic d'un spectateur sur l'écran public)
et `vplayer` (un joueur retourne ses propres cartes en mode équipe active). #159 y ajoute `anim` :
l'animateur peut désormais retourner les cartes du doigt directement sur sa tablette — **une seule
entrée ajoutée à la table**, aucun changement du mécanisme `IsActionAllowed`.

| Client | `FLIP_MEMORY_CARD` |
|---|---|
| `admin` | ❌ — la régie retourne les cartes via son aperçu TV en iframe, une connexion `tv`, pas `admin` |
| `tv` | ✅ (inchangé) |
| `vplayer` | ✅ (inchangé) |
| `anim` | ✅ **nouveau (v6.2.x, #159)** |

> ⚠️ **C'est un élargissement de capacité pour `anim`, pas un simple affichage** : le retournement
> modifie l'état de jeu (paires trouvées, erreurs, tour de l'équipe active). Cohérent avec le
> périmètre "conduite en direct" déjà accordé à l'animateur (`START`/`STOP`/`REVEAL`/`READY`,
> `TEAM_POINTS`/`BUMPER_POINTS`), et borné par le moteur : `Engine.FlipMemoryCard` refuse tout
> retournement hors phase `STARTED`, quel que soit le client d'origine.
>
> **`MEMORY_SET_TEAMS` n'est PAS ajouté** : le choix des équipes participantes reste réservé à
> `/admin` (périmètre explicite de #159). Conséquence assumée : un animateur seul ne peut pas
> démarrer une partie MEMORY multi-équipes sans passer par la régie au préalable.

---

## References

- [RFC 6455 - WebSocket Protocol](https://datatracker.ietf.org/doc/html/rfc6455)
- [ArduinoWebsockets Library](https://github.com/gilmaimon/ArduinoWebsockets)
- [gorilla/websocket (Go)](https://github.com/gorilla/websocket)
