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

**Garantie d'inscription (v11.2.2, #272)** : le serveur inscrit chaque connexion dans son hub **avant** de lire son premier message, sur les endpoints à identité (`/ws/buzzer`, `/ws/admin`, `/ws/tv`, `/ws/player`, `/ws/anim` ; `/ws/logs` n'en a pas). Un message envoyé immédiatement après l'ouverture de la connexion est donc toujours rattaché à celle-ci (buzzer, joueur, tablette). Aucun changement de format.

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

### QUESTION_SOUND (Contrôle son question) — v11.1.0, #219

Envoye par l'animateur (`/anim`) ou l'administrateur (`/admin`) pour contrôler la lecture du son attaché à la question courante.

**Endpoints** : `/ws/anim`, `/ws/admin` (allow-list : `admin`, `anim` uniquement)

**Payload** :
```json
{
  "ACTION": "QUESTION_SOUND",
  "MSG": {
    "COMMAND": "PLAY" | "PAUSE" | "RESUME" | "STOP"
  }
}
```

| Commande | Appliquée sur | Effet |
|----------|---|---|
| `PLAY` | Son IDLE, PLAYING, ou PAUSED | Arrête lecture courante (si active), recommence depuis le début (IDLE) |
| `PAUSE` | Son PLAYING uniquement | Suspend la lecture sans fermer le fichier |
| `RESUME` | Son PAUSED uniquement | Reprend depuis le point de pause sans coupure |
| `STOP` | Son PLAYING ou PAUSED | Arrête définitivement, libère chronomètre si mode différé (`SOUND_TIMER_DELAYED=true`) |

**Comportement serveur** :

1. **Guard question courante** : Si aucune question en cours ou question ne porte pas de son, ignore silencieusement
2. **Exécution** : Commande transmise au `MediaPlayer` gérant la lecture
3. **Broadcast UPDATE** : Envoie `QUESTION_SOUND_STATE` + `ANSWER_TIMER_WAITING` mis à jour à **tous les clients** (admin, anim, TV)
   - `QUESTION_SOUND_STATE` → `"IDLE"` | `"PLAYING"` | `"PAUSED"`
   - `ANSWER_TIMER_WAITING` → `true` si chronomètre reste figé (mode différé, son toujours actif), `false` sinon

**Addendum v11.1 (gate média T0)** : Deux champs supplémentaires diffusés aux mêmes occasions :
   - `QUESTION_SOUND_UNAVAILABLE` → `""` | `"DISABLED"` | `"OUTPUT"` | `"FILE"` (motif de blocage, si la question est bloquée avant STARTED par manque de média)
   - `SOUND_GATE_BYPASSED` → `true` si l'admin a déverrouillé via Ctrl+clic, `false` sinon

**Cycle de vie du son par commande** :

| Commande | Phase valide | Avant | Après |
|----------|---|---|---|
| `PLAY` | `STARTED` / `PAUSED` | Quel que soit l'état | `PLAYING` |
| `PAUSE` | `STARTED` / `PAUSED` | `PLAYING` | `PAUSED` |
| `RESUME` | `STARTED` / `PAUSED` | `PAUSED` | `PLAYING` (reprend position sauvegardée) |
| `STOP` | `STARTED` / `PAUSED` | Quel que soit l'état | `IDLE` + chronomètre libéré si mode différé |

**Couplage chronomètre différé** :

En mode différé (`SOUND_TIMER_DELAYED=true`) :
- **PLAY** : chronomètre reste figé (`ANSWER_TIMER_WAITING=true`)
- **PAUSE** : chronomètre reste figé (`ANSWER_TIMER_WAITING=true`)
- **RESUME** : chronomètre reste figé (`ANSWER_TIMER_WAITING=true`)
- **STOP** : chronomètre libéré immédiatement (`ANSWER_TIMER_WAITING=false` → le chronomètre commence son décompte)

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

## Équipes : identité, verrou et renommage — UPDATE / FULL (v11.2, #249)

Source de vérité : `contracts/websocket-actions.md` §UPDATE, §TEAMS_UPDATE_REFUSED et
`contracts/game-state.md` §`TEAMS_LOCKED`.

- La map `teams` est indexée par **clé**. `NAME` vaut toujours la clé : le serveur le réécrit à chaque
  `UPDATE`/`FULL` (et au chargement de `teams.json`, avec `WARN TEAM_NAME_NORMALIZED`).
- **Renommer = changer la clé.** Le client envoie la map où l'ancienne clé a disparu, la nouvelle clé
  porte `NAME` = nouvelle clé, et les `bumpers` passent à la nouvelle `TEAM`.
- **`TEAMS_LOCKED`** (`GameState`, toujours sérialisé) : `true` de la sélection d'une question
  (`Ready()`) jusqu'à son `REVEALED`, sur `NEW_GAME` et au démarrage du serveur : `false`.
- **Pendant le verrou**, un `UPDATE`/`FULL` qui retire une clé (renommage ou suppression) est refusé
  **en entier**. Ajout d'équipe, couleur et affectation de joueurs restent permis.

```json
{ "ACTION": "TEAMS_UPDATE_REFUSED", "MSG": { "REASON": "TEAMS_LOCKED_DURING_QUESTION", "TEAMS": ["Les Bleus"] } }
```

Réponse **ciblée** à l'émetteur, suivie de l'état complet. Journal : `WARN TEAMS_CHANGE_REFUSED`.

- **Hors verrou** (y compris `REVEALED`), un renommage est reconnu quand une clé disparaît et une nouvelle
  apparaît **dans le même message**, avec les mêmes joueurs. Le serveur reporte sous la nouvelle clé la
  liste fermée des états (`game-state.md` §`TEAMS_LOCKED`), historique compris. Journal : `INFO TEAM_RENAMED`.
- Sinon, c'est une suppression suivie d'un ajout : les **états de jeu** de l'équipe supprimée sont effacés.
  L'historique des points est conservé. Le serveur y ajoute un événement `TEAM_DELETED` (`TEAM_NAME`,
  `TIMESTAMP`) **à la suppression uniquement**, jamais à un renommage.
- `AWARDED_TEAMS` ignore les `POINTS_AWARDED` d'une équipe **antérieurs à son dernier `TEAM_DELETED`**. La
  comparaison se fait sur l'**ordre de l'historique** (indice dans la liste), jamais sur les horodatages.
  Une équipe recréée sous le même nom n'hérite donc pas du verrou « déjà créditée » de l'ancienne.

## Liaison VJoueur, ARDOISE final et REVEAL gardé — v11.2 (#246)

Source de vérité : `contracts/websocket-actions.md` (sections PLAYER_CONNECT, ARDOISE_INPUT,
ARDOISE_FINAL, ARDOISE_FINAL_ACK, REVEAL, REVEAL_REFUSED, CLIENT_TRACE) et `contracts/logging.md`.

### Liaison connexion ↔ bumper

- Un `PLAYER_CONNECT {NAME, ID}` accepté **lie la connexion WebSocket** à ce bumper. Le client
  VJoueur l'envoie à **chaque** ouverture de sa WebSocket, même si l'état reçu le montre déjà connecté.
- Une seule connexion liée par bumper : la plus récente gagne. L'ancienne reçoit
  `PLAYER_EVICTED { REASON: "DUPLICATE_SESSION" }` et est déliée (`WARN VPLAYER_DUPLICATE_SESSION`).
- Actions joueur (`ARDOISE_INPUT`, `ARDOISE_FINAL`, `VPLAYER_QCM_ANSWER`, `BUTTON`, `FLIP_MEMORY_CARD`)
  émises par un client `vplayer` : refusées sans réponse si la connexion n'est liée à rien
  (`PLAYER_ACTION_UNBOUND`) ou si `ID` ≠ bumper lié (`PLAYER_FOREIGN_ID`).
- Le nom n'identifie jamais un joueur côté client. Le `NAME` envoyé ne renomme plus le bumper ;
  `PLAYER_CONNECTED.NAME` renvoie le nom serveur (`WARN VPLAYER_NAME_MISMATCH` si différent).
- Noms de VJoueurs uniques (sans casse) : un renommage ou une entrée en doublon venant de
  `UPDATE`/`FULL` est écarté (`WARN VPLAYER_DUPLICATE_NAME`).

```json
{ "ACTION": "PLAYER_EVICTED", "MSG": { "REASON": "DUPLICATE_SESSION" } }
```

### ARDOISE_INPUT — compléments v11.2

Champs facultatifs : `SEQ` (compteur croissant par question et par connexion, `ARDOISE_SEQ_GAP` /
`ARDOISE_SEQ_STALE` en diagnostic) et `REASON` (`key`, `clear`, `backspace_empty`, `resend`, `visible`).
`TEXT` est limité à **2 000 caractères** (points de code Unicode). Au-delà, le message est **ignoré en
entier** (`WARN ARDOISE_INPUT_TOO_LONG`), jamais tronqué.

### ARDOISE_FINAL (VPlayer → Server)

Envoyé à la sortie de `STARTED` (STOP ou PAUSE), pour la question ARDOISE courante.

| Champ | Type | Description |
|-------|------|-------------|
| `ID` | string | Bumper lié à la connexion |
| `QUESTION_ID` | string | Doit être la question courante, sinon `ARDOISE_FINAL_STALE` |
| `TEXT` | string | Texte complet au gel du clavier, `""` autorisé, ≤ 2 000 caractères |
| `HASH` | string | FNV-1a 32 bits du texte UTF-8, 8 caractères hexadécimaux minuscules |
| `SEQ` | integer | Facultatif, même compteur que `ARDOISE_INPUT` |

Le serveur recalcule le hash (divergence → `WARN ARDOISE_FINAL_HASH_MISMATCH`, pas d'accusé), écrit
`ARDOISE_ANSWERS[équipe]` avec `RECEIVED = true` et `RECEIVED_HASH`, répond `ARDOISE_FINAL_ACK` à la
seule connexion émettrice, puis diffuse `UPDATE` immédiatement vers admin et anim. Il est idempotent.

Vecteurs de test FNV-1a (Go et JS doivent les reproduire) :

| TEXT | HASH |
|------|------|
| `""` | `811c9dc5` |
| `"a"` | `e40c292c` |
| `"Paris"` | `246f9d40` |

**Renvoi côté tablette** : un envoi au gel, puis un renvoi par seconde tant que l'accusé valide n'est
pas reçu (même `QUESTION_ID` et même `HASH`). Au plus 30 envois par liaison. Les renvois s'arrêtent
dès que la phase ou la question change.

### ARDOISE_FINAL_ACK (Server → VPlayer, ciblé)

Payload : `QUESTION_ID`, `HASH`. **Ne contient jamais le texte** (confidentialité, #128).

### REVEAL — garde ARDOISE et FORCE (admin, anim)

- Champ facultatif `FORCE` (booléen). Hors question ARDOISE, il est ignoré.
- Sans `FORCE: true`, le REVEAL est **refusé** tant qu'une équipe attendue n'a pas `RECEIVED = true`.
  Une équipe est attendue si elle a au moins un VJoueur. La tablette hors ligne compte comme non reçue.
- Refus : la phase ne change pas, et l'émetteur seul reçoit `REVEAL_REFUSED`.
- Avec `FORCE: true` : REVEAL immédiat, `WARN ARDOISE_REVEAL_FORCED` s'il manque des équipes. Ces
  équipes gardent leur dernier texte reçu, avec `RECEIVED = false`.

```json
{ "ACTION": "REVEAL_REFUSED", "MSG": { "REASON": "ARDOISE_PENDING", "PENDING": [ { "TEAM": "Les Verts", "OFFLINE": true } ] } }
```

### CLIENT_TRACE (tous types de client, diagnostic)

Aucun effet sur l'état du jeu. Accepté dans **toutes les phases**, entracte comprise. Limites : 10
messages/s par connexion, 50/s pour l'ensemble des connexions, 1 Ko par message. Chaque trace produit
une seule ligne de journal (`KIND`, `AT`, `TEAM`, `PREV_LEN`, `CAUSE_ACTION`, `DETAIL`). Les `KIND`
sont `ARDOISE_CLEARED_SEEN`, `VISIBILITY`, `PAGESHOW`, `FREEZE`, `RESUME`, `WS_GAP`, `STALE_TAB`,
`DUPLICATE_SESSION`, `ARDOISE_FINAL_GIVEUP` (émis quand la tablette abandonne le renvoi après 30 envois sans accusé).

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
| QUESTION_SOUND | - | - | - | - |

**Action ENTRANTE QUESTION_SOUND** (v11.1.0, #219) — Envoyée depuis `/ws/admin` ou `/ws/anim` seulement (allow-list fermée). Le serveur diffuse les changements d'état (`QUESTION_SOUND_STATE` + `ANSWER_TIMER_WAITING`) via une action `UPDATE` à tous les clients.

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

## Action AI_GENERATION_PROGRESS (v6.1.1, post-QUALIF #137; TARGET depuis v8.1.0 #203)

Progression d'une génération de questions via l'API Claude (Anthropic) ou Groq (gratuit).

### Endpoint

`/ws/admin` uniquement — jamais `/ws/tv` ou `/ws/player`.

### Payload

```json
{
  "ACTION": "AI_GENERATION_PROGRESS",
  "MSG": {
    "JOB_ID": "gen-20260901-162300-a1b2",
    "TARGET": "RAFALE",
    "STATE": "RUNNING",
    "BATCHES_DONE": 2,
    "BATCHES_TOTAL": 5,
    "CREATED_COUNT": 40,
    "SKIPPED_COUNT": 0,
    "ERROR_CODE": "",
    "ERROR_MESSAGE": "",
    "PROVIDER": "groq"
  }
}
```

### Champs

| Champ | Type | Description | États possibles | Depuis |
|-------|------|-------------|-----------------|--------|
| `TARGET` | string | Destination de la génération | `"QUIZ"` (défaut si absent), `"RAFALE"` | v8.1.0 #203 |
| `STATE` | string | État actuel du job | `RUNNING`, `DONE`, `FAILED`, `CANCELLED` | v6.1.1 |
| `BATCHES_DONE` | int | Nombre de lots terminés | ≥ 0 | v6.1.1 (`BATCH_NUMBER` anciennement) |
| `BATCHES_TOTAL` | int | Nombre total de lots prévu | ≥ 1 | v6.1.1 (`TOTAL_BATCHES` anciennement) |
| `CREATED_COUNT` | int | Nombre de questions créées jusqu'ici | ≥ 0 | v6.1.1 |
| `SKIPPED_COUNT` | int | Nombre de questions rejetées/invalides | ≥ 0 | v6.1.1 |
| `ERROR_CODE` | string | Code d'erreur stable (machine-friendly) | Vide, ou `no_api_key`, `api_key_rejected`, `quota_exceeded`, `upstream_error`, `network_error`, etc. — vide si `STATE != FAILED` | v6.1.1 |
| `ERROR_MESSAGE` | string | Message d'erreur réel du provider, assaini (clés API masquées) | Vide sauf `STATE = FAILED` — exemple Groq #142 (avant correction) : `"discriminator: multiple candidate properties CATEGORY, DIFFICULTY, TYPE [discriminator_multiple_candidates]"` | v6.1.1 |
| `PROVIDER` | string | Fournisseur utilisé pour ce job | `"anthropic"`, `"groq"` | v6.1.1 |
| `JOB_ID` | string | Identifiant unique du job | ex. `"gen-20260901-162300-a1b2"` | v6.1.1 |

### Sémantique

- **STATE transitions** : `RUNNING` → (`DONE` \| `FAILED` \| `CANCELLED`)
- **TARGET** :
  - **Additif et rétrocompatible** : absent ou `"QUIZ"` = chemin générique (Quiz/MEMOTION+)
  - **`"RAFALE"`** (v8.1.0 #203) = génération du réservoir RAFALE (endpoint distinct, schéma plat, paliers de volume)
  - **Nécessaire au frontend** : la progression est un singleton global et deux modales l'écoutent (page Questions et page RAFALE). Sans ce champ, un job RAFALE ferait basculer la modale Quiz en « génération en cours »
- **ERROR_MESSAGE** :
  - **Absent** (`omitempty`) quand `STATE != FAILED` (pas de message d'erreur généré)
  - **Présent** quand `STATE = FAILED` — message texte réel du provider (Anthropic ou Groq API response body) avec :
    - Clés API filtrées (regex `sk-ant-…` / `gsk_…` → `[redacted]`)
    - Troncature UTF-8 safe si > 500 runes
    - Jamais exposé à la TV ou VPlayer (endpoint `/ws/admin` uniquement)

### Exemple de flux complet

**Succès — génération Quiz** (v6.1.1–v8.0.0) :
```json
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "TARGET": "QUIZ", "STATE": "RUNNING", "BATCHES_DONE": 1, "BATCHES_TOTAL": 5, "CREATED_COUNT": 0, "SKIPPED_COUNT": 0, "JOB_ID": "gen-123", "PROVIDER": "groq" } }
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "TARGET": "QUIZ", "STATE": "RUNNING", "BATCHES_DONE": 2, "BATCHES_TOTAL": 5, "CREATED_COUNT": 40, "SKIPPED_COUNT": 0, "JOB_ID": "gen-123", "PROVIDER": "groq" } }
...
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "TARGET": "QUIZ", "STATE": "DONE", "BATCHES_DONE": 5, "BATCHES_TOTAL": 5, "CREATED_COUNT": 200, "SKIPPED_COUNT": 0, "JOB_ID": "gen-123", "PROVIDER": "groq", "ERROR_CODE": "", "ERROR_MESSAGE": "" } }
```

**Succès — génération RAFALE** (v8.1.0 #203) :
```json
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "TARGET": "RAFALE", "STATE": "RUNNING", "BATCHES_DONE": 1, "BATCHES_TOTAL": 3, "CREATED_COUNT": 0, "SKIPPED_COUNT": 0, "JOB_ID": "gen-456", "PROVIDER": "groq" } }
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "TARGET": "RAFALE", "STATE": "DONE", "BATCHES_DONE": 3, "BATCHES_TOTAL": 3, "CREATED_COUNT": 50, "SKIPPED_COUNT": 2, "JOB_ID": "gen-456", "PROVIDER": "groq", "ERROR_CODE": "", "ERROR_MESSAGE": "" } }
```

**Erreur avec détail** (Groq #142, post-QUALIF v6.1.1) :
```json
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "TARGET": "QUIZ", "STATE": "RUNNING", "BATCHES_DONE": 1, "BATCHES_TOTAL": 5, "CREATED_COUNT": 0, "SKIPPED_COUNT": 0, "JOB_ID": "gen-123", "PROVIDER": "groq" } }
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "TARGET": "QUIZ", "STATE": "FAILED", "BATCHES_DONE": 1, "BATCHES_TOTAL": 5, "CREATED_COUNT": 0, "SKIPPED_COUNT": 0, "JOB_ID": "gen-123", "PROVIDER": "groq", "ERROR_CODE": "upstream_error", "ERROR_MESSAGE": "invalid JSON schema for response_format: 'buzz_questions': /properties/questions/items/anyOf: anyOf disambiguation failed: anyOf: discriminator: multiple candidate properties CATEGORY, DIFFICULTY, TYPE [discriminator_multiple_candidates]" } }
```

Frontend affiche :
- Message générique : `"Le service IA a renvoyé une erreur"`
- Panneau détail technique (repliable) : Le message exact du provider

**Arrêt utilisateur** :
```json
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "TARGET": "RAFALE", "STATE": "RUNNING", "BATCHES_DONE": 2, "BATCHES_TOTAL": 5, "CREATED_COUNT": 40, "SKIPPED_COUNT": 0, "JOB_ID": "gen-456", "PROVIDER": "groq" } }
{ "ACTION": "AI_GENERATION_PROGRESS", "MSG": { "TARGET": "RAFALE", "STATE": "CANCELLED", "BATCHES_DONE": 2, "BATCHES_TOTAL": 5, "CREATED_COUNT": 40, "SKIPPED_COUNT": 0, "JOB_ID": "gen-456", "PROVIDER": "groq", "ERROR_CODE": "", "ERROR_MESSAGE": "" } }
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
- **Cible** : `ClientTypeAnim` (`/ws/anim`) **et `ClientTypeAdmin`** (`/ws/admin`, v11.2 #244/#251), via `BroadcastToTypes` — comme `NEXT_QUESTION`. `/tv`, `/player` et `/buzzer` ne reçoivent jamais cette action.
- **Trigger** : `TEAM_POINTS`, `BUMPER_POINTS` et `TEAM_POINTS_BATCH` traités (origine `/admin` ou `/anim` indifféremment), `READY` (changement de question), `NEW_GAME`, `RAZ`, HELLO d'un client animateur (état initial)

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

## Messagerie Régie — Communication Animateur (#167, v6.4.0)

### REGIE_MESSAGE_SEND (Régie → Animateurs) — v6.4.0, #167

Envoye par la régie pour diffuser une consigne texte à tous les animateurs connectés.

**Endpoint** : `/ws/admin`  
**Emetteur** : Admin uniquement  
**Destinataire** : `anim` + `admin`

**Payload** (Client → Server) :
```json
{
  "ACTION": "REGIE_MESSAGE_SEND",
  "MSG": {
    "TEXT": "Passez à la question suivante"
  }
}
```

| Champ MSG | Type | Obligatoire | Description |
|-----------|------|-------------|-------------|
| `TEXT` | string | ✅ | Consigne (max 140 caractères, troncature serveur en runes) |

**Comportement serveur** :
- Trim l'espace, refuse si vide
- **Troncature à 140 runes** (jamais octets) — UTF-8 valide en sortie
- **Idempotence** : si texte identique au message actif → no-op, pas de diffusion
- Réarmement du timestamp `SENT_AT` en ms Unix, diffusion du nouvel état via `REGIE_MESSAGE`

**Réponse serveur** : Silence (200 OK implicite) si succès, `WARN` log si rejeté (vide/idempotent)

### REGIE_MESSAGE_CLEAR (Acquittement) — v6.4.0, #167

Envoye par animateur (« Vu ») ou régie (retrait) pour effacer le message actif.

**Endpoint** : `/ws/admin` (régie), `/ws/anim` (animateur)  
**Emetteur** : Admin + Anim  
**Destinataire** : `anim` + `admin`

**Payload** (Client → Server) :
```json
{
  "ACTION": "REGIE_MESSAGE_CLEAR",
  "MSG": {}
}
```

**Comportement serveur** :
- No-op idempotent si aucun message actif
- `CLEARED_BY` déduit du `ClientType` emetteur : `"ANIM"` (animateur acquitte), `"REGIE"` (régie retire)
- Diffusion du nouvel état via `REGIE_MESSAGE` avec `ACTIVE: false`

### REGIE_MESSAGE (État du message) — v6.4.0, #167

Action de diffusion serveur portant l'état complet du message.

**Endpoint** : `/ws/admin` + `/ws/anim`  
**Emetteur** : Serveur uniquement  
**Destinataire** : `anim` + `admin` (jamais `tv`, `vplayer`, `buzzer`)

**Payload** (Server → Client) :
```json
{
  "ACTION": "REGIE_MESSAGE",
  "MSG": {
    "ACTIVE": true,
    "TEXT": "Passez à la question suivante",
    "SENT_AT": 1724079850123,
    "CLEARED_BY": ""
  }
}
```

| Champ MSG | Type | Toujours présent | Description |
|-----------|------|---|-------------|
| `ACTIVE` | bool | ✅ | `true` = message affiché, `false` = repos/acquitté |
| `TEXT` | string | ✅ | Contenu du message (jamais `null`, chaîne vide si inactif) |
| `SENT_AT` | int64 | ✅ | Timestamp ms Unix, `0` si inactif |
| `CLEARED_BY` | string | ✅ | Raison de l'inactivité : `"ANIM"` (acquitté), `"REGIE"` (retiré), `""` (repos initial) |

**Diffusion** :
- Au `HELLO` d'un admin ou animateur si message actif
- Après `REGIE_MESSAGE_SEND` (nouveau ou remplacement)
- Après `REGIE_MESSAGE_CLEAR` (acquittement/retrait)
- **Jamais** sur le chemin de `broadcastUpdate` — envoi explicite uniquement

**Contrainte d'affichage** :
- Bande régie (`/admin`) : quatre états (repos, saisie, actif, acquitté)
- Bande animateur (`/anim`) : état repos ou message + bouton « Vu »
- Aucun affichage sur `/tv`, `/player`, `/buzzer`

**Garantie de synchronisation** :
- Deux sessions `/admin` voient le même état
- Acquittement `/anim` synchrone sur tous les `/admin`
- Retrait `/admin` synchrone sur tous les `/anim`

---

## Affichage public — DISPLAY et SCORE_CHANGE (v11.2, #252)

Source de vérité : `contracts/websocket-actions.md` §TEAM_POINTS, §BUMPER_POINTS, §TEAM_POINTS_BATCH ;
`contracts/game-state.md` §`SCORE_CHANGE` ; `contracts/logging.md` §3.2 quinquies.

- **`DISPLAY`** (`TEAM_POINTS`, `BUMPER_POINTS`, `TEAM_POINTS_BATCH`) : `true` ou absent = effets publics actuels.
  `false` = **correction cachée**, sans effet animé sur aucun canal.
- **Honoré seulement pour un émetteur `admin`.** Envoyé par l'animateur ou un appelant interne, il est ignoré :
  l'attribution est visible et comptée « déjà créditée », et le serveur journalise
  `WARN SCORE_CORRECTION_HIDDEN_REFUSED`.
- **Avec `DISPLAY: false`** : ni comète LED, ni pulsation d'ambiance, ni son « gagné », ni flash de score. L'entrée
  d'historique est enregistrée avec `DISPLAY: false`, mais **n'est pas comptée** dans `AWARDED_TEAMS`.
  Journal : `WARN SCORE_CORRECTION_HIDDEN`.
- **`POINTS = 0`** : un client `admin` qui envoie 0 voit son action ignorée (ni historique, ni `UPDATE`, ni
  `SCORE_CHANGE.SEQ`). Un `POINTS = 0` de l'animateur ou d'un appelant interne reste enregistré : c'est le geste « 0 pt »
  de #170, un verrou valide.
- **Montant** : l'interface impose un entier non nul de −999 à 999. **Le serveur ne vérifie pas cette borne** (le
  contrat de `TEAM_POINTS` et `BUMPER_POINTS` est inchangé).

```json
{ "ACTION": "BUMPER_POINTS", "MSG": { "ID": "AA:BB:CC:DD:EE:FF", "POINTS": -2, "DISPLAY": false } }
```

### SCORE_CHANGE (`GameState`, toujours sérialisé)

| Champ | Type | Description |
|-------|------|-------------|
| `SEQ` | number | +1 à chaque `TEAM_POINTS`, `BUMPER_POINTS` ou `TEAM_POINTS_BATCH` appliqué |
| `DISPLAY` | bool | Visibilité de la dernière modification |
| `TEAMS` | string[] | Équipes dont le score a changé |
| `PLAYERS` | string[] | Joueurs (ID) dont le score a changé |

- Non persisté. Remis à zéro au démarrage et sur `NEW_GAME`.
- **Dernière modification manuelle uniquement** : les crédits automatiques du moteur (`UpdateScore`) ne l'incrémentent
  pas, et leurs hausses restent animées.
- Écrans TV, VJoueur et anim : un nouveau `SEQ` avec `DISPLAY: false` prend les nouveaux scores listés comme référence
  **sans animation**. Avec `DISPLAY: true`, le comportement est inchangé.

```json
{ "SCORE_CHANGE": { "SEQ": 14, "DISPLAY": false, "TEAMS": ["Les Bleus"], "PLAYERS": [] } }
```

### SCORE_CHANGE.GAINS — gains par équipe (v11.2.1, #265)

- Ajout de `GAINS: {TEAM, POINTS}[]` dans `SCORE_CHANGE` : variation nette par équipe pour la modification de même `SEQ`.
  Champ additif ; les anciens clients l'ignorent.
- Écrit dans la même section critique que `SEQ`, `TEAMS` et `PLAYERS`, pour `TEAM_POINTS`, `BUMPER_POINTS`,
  `TEAM_POINTS_BATCH` et `BUMPER_POINTS_BATCH`. Les crédits automatiques du moteur ne le modifient pas.
- Règles complètes (ordre, cas limites, lecture par TV et tablette) : `contracts/game-state.md` §`SCORE_CHANGE.GAINS`.

```json
{ "SCORE_CHANGE": { "SEQ": 15, "DISPLAY": true, "TEAMS": ["Les Rouges", "Les Bleus"], "PLAYERS": ["AA:BB:CC:DD:EE:01", "AA:BB:CC:DD:EE:02", "AA:BB:CC:DD:EE:07"],
  "GAINS": [ { "TEAM": "Les Rouges", "POINTS": 4 }, { "TEAM": "Les Bleus", "POINTS": 2 } ] } }
```

## Retournement MEMORY/MEMOTION idempotent — FLIP_ID et FLIP_MEMORY_CARD_ACK (v11.2.1, #248)

- `FLIP_MEMORY_CARD` accepte un champ optionnel `FLIP_ID` (1–64 caractères `[A-Za-z0-9_-]`), créé par la tablette à
  chaque nouveau geste et réutilisé pour ses renvois. Honoré uniquement pour un `vplayer` résolu.
- Le serveur déduplique par bumper (16 identifiants mémorisés, non persistés) et répond par la nouvelle action
  serveur → émetteur **`FLIP_MEMORY_CARD_ACK`** (`STATUS`: `APPLIED` | `IGNORED` ; `REASON`: `""` si appliqué, sinon
  `NOT_TURN` | `NOT_FLIPPABLE` | `NOT_PLAYABLE` | `CARD_SCOPE` ; `DUPLICATE`), suivie d'un `UPDATE` ciblé. Un doublon
  ou un geste ignoré ne déclenche aucune diffusion générale.
- **Règle tablette** : `FLIP_ID` n'est envoyé que si l'équipe du joueur a la main. Un tap hors tour part sans `FLIP_ID`
  et est ignoré en silence, comme avant (décision du 2026-08-24). Le `REASON` `NOT_TURN` n'apparaît donc que si la main
  passe pendant l'attente de l'accusé.
- Sans `FLIP_ID` : comportement inchangé, aucun accusé.
- Source de vérité : `contracts/websocket-actions.md` §`FLIP_MEMORY_CARD` (§« Idempotence — `FLIP_ID` ») et §`FLIP_MEMORY_CARD_ACK` ;
  journal : `contracts/logging.md` §3.2 octies.

## Écho ciblé d'un buzz VJoueur refusé (v11.2.1, #247)

- Un appui buzzer refusé par le moteur ne produit plus de `PAUSE` ni de mise à jour générale. Si l'émetteur est un
  VJoueur résolu, il reçoit seulement un `UPDATE` ciblé (son état serveur). Un bumper physique reste silencieux.
  Vaut pour tout buzz de VJoueur refusé passant par le chemin simulé, quel que soit le type de question buzzable.
- Un appui accepté diffuse un seul `PAUSE` (plus de doublon). Un appui pendant une question ARDOISE est refusé.
- L'écho ciblé du QCM refusé existait déjà (`handleVPlayerQCMAnswer`) et est couvert par un test de non-régression.

## Attribution au niveau joueur — BUMPER_POINTS_BATCH (v11.2, #254)

Source de vérité : `contracts/websocket-actions.md` §BUMPER_POINTS_BATCH, §BUMPER_POINTS_BATCH_RESULT, §AWARDED_TEAMS,
§TEAM_POINTS_BATCH « Niveau d'attribution ».

Le niveau d'une question est calculé par `resolveAwardLevel` (`utils/pointsAward.js`) : **équipe** (`TEAM_POINTS_BATCH`) ou
**joueur** (`BUMPER_POINTS_BATCH`). SPEEDY suit le réglage « Points : équipe / joueur » (défaut joueur) ; QCM et ARDOISE
suivent le même réglage (défaut équipe) ; MEMORY, MEMOTION et RAFALE sont toujours au niveau équipe.

```json
{ "ACTION": "BUMPER_POINTS_BATCH", "MSG": { "QUESTION_ID": "12", "ENTRIES": [ {"ID": "AA:BB:CC:01", "POINTS": 2}, {"ID": "AA:BB:CC:07", "POINTS": 1} ] } }
```

- **Payload** : `QUESTION_ID`, `ENTRIES` (1 à 64 joueurs distincts, `ID` = MAC du bumper ou ID du VJoueur, `POINTS` = entier
  non nul de −999 à 999), `DISPLAY` (même règle que `TEAM_POINTS_BATCH`, admin seulement).
- **Montants calculés côté client** (coefficient d'indices, commutateur « Tenir compte des indices »). Le serveur ne recalcule
  rien et ne vérifie que les bornes. Aucun paramètre serveur pour les indices.
- **Refus global** (`BUMPER_POINTS_BATCH_RESULT { STATUS: "REFUSED" }`, rien n'est appliqué) : `NOT_REVEALED`,
  `QUESTION_MISMATCH`, `INVALID_ENTRIES`, `INVALID_POINTS`, `UNKNOWN_PLAYER` (`PLAYERS` = IDs inconnus). Journal
  `WARN BUMPER_POINTS_BATCH_REFUSED`.
- **Joueur ignoré** (`SKIPPED`), sous un seul verrou :
  - `ALREADY_AWARDED` : joueur déjà crédité à titre individuel (présent dans `AWARDED_TEAMS.PLAYERS`) ;
  - `TEAM_ALREADY_AWARDED` : son équipe a déjà reçu un crédit **d'équipe** pour la question. Un crédit joueur rend son équipe
    « déjà créditée » pour `TEAM_POINTS_BATCH`, et la règle inverse s'applique ici.
- **Réponse ciblée** : `BUMPER_POINTS_BATCH_RESULT`, avec `APPLIED` (`[{ID, POINTS}]`) et `SKIPPED` (`[{ID, REASON}]`).

```json
{ "ACTION": "BUMPER_POINTS_BATCH_RESULT", "MSG": { "STATUS": "APPLIED", "QUESTION_ID": "12", "APPLIED": [ {"ID": "AA:BB:CC:01", "POINTS": 2} ], "SKIPPED": [ {"ID": "AA:BB:CC:07", "REASON": "ALREADY_AWARDED"}, {"ID": "AA:BB:CC:09", "REASON": "TEAM_ALREADY_AWARDED"} ], "PLAYERS": [] } }
```

`ALREADY_AWARDED` : joueur déjà crédité à titre individuel. `TEAM_ALREADY_AWARDED` : son équipe a déjà reçu un crédit d'équipe.
- **Diffusions** : `UPDATE`, `SCORE_CHANGE` (`PLAYERS` et `TEAMS` concernés), `AWARDED_TEAMS`. Journal `INFO BUMPER_POINTS_BATCH`.
- Le clic unitaire sur un joueur (`BUMPER_POINTS`) reste libre.
- **Suspension** : `BUMPER_POINTS_BATCH` est suspendu pendant le verrou de lancement (#253).

### AWARDED_TEAMS — `PLAYERS` et `TEAMS[].TEAM_LEVEL` (v11.2, #254)

- `PLAYERS` : une entrée par joueur crédité individuellement (`{ID, TEAM, POINTS, TIMESTAMP}`), mêmes règles que `TEAMS`.
- `TEAMS[].TEAM_LEVEL` : `true` si l'équipe a reçu au moins un crédit d'équipe pour la question. Toujours présent.
- Envoyé à l'admin et à l'animateur.

### SET_CREDIT_POINTS ≤ 0 (v11.2, #254)

Le client admin n'envoie jamais `SET_CREDIT_POINTS` avec une valeur nulle ou négative. Le serveur ignore une telle valeur
(journal INFO). Les tablettes de l'animateur gardent ainsi la dernière valeur positive.

## Action TEAM_POINTS_BATCH — attribution groupée (v11.2, #244/#251)

Envoyée par `admin` (« Attribuer les points (N) » / « Appliquer les pénalités (N) », #254), **en phase `REVEALED` uniquement**. Utilisée quand la question est au niveau équipe ; au niveau joueur, c'est `BUMPER_POINTS_BATCH`.
Contrat détaillé : `contracts/websocket-actions.md` §TEAM_POINTS_BATCH.

```json
{ "ACTION": "TEAM_POINTS_BATCH", "MSG": { "QUESTION_ID": "9", "ENTRIES": [ {"TEAM": "Les Bleus", "POINTS": 3}, {"TEAM": "Les Rouges", "POINTS": 2} ] } }
```

| Champ | Type | Description |
|-------|------|-------------|
| `QUESTION_ID` | string | Question affichée par l'admin au clic |
| `ENTRIES` | array | 1 à 64 entrées, équipes distinctes. `TEAM` = clé d'équipe ; `POINTS` = entier non nul, de −999 à 999 |
| `DISPLAY` | bool | **Réservé à #252** (« Afficher sur la TV »). Absent = `true`. Sans effet tant que #252 n'est pas livré |

- **Refus global** (rien n'est appliqué) : `NOT_REVEALED`, `QUESTION_MISMATCH`, `INVALID_ENTRIES`,
  `INVALID_POINTS`, ou `UNKNOWN_TEAM` (équipe renommée ou supprimée entre la sélection et le clic, `TEAMS` = clés
  inconnues). Journal `WARN TEAM_POINTS_BATCH_REFUSED`.
- **Lot appliqué d'un bloc** : une équipe déjà créditée pour la question (présente dans `AWARDED_TEAMS`) est
  ignorée (`SKIPPED`, `ALREADY_AWARDED`). Les autres reçoivent `TEAM_POINTS += POINTS` et une entrée
  `POINTS_AWARDED` par équipe.
- **Effets publics groupés** (si `DISPLAY` ≠ `false`), une seule fois pour le lot : son « gagné » si un montant est
  positif ; une comète LED par équipe créditée positivement ; une pulsation d'ambiance ; un seul flash de score pour
  l'équipe au plus fort montant positif.
- **Diffusions** : `UPDATE` (scores), `AWARDED_TEAMS` (admin et anim).
- **Réponse à l'émetteur seul** : `TEAM_POINTS_BATCH_RESULT`.

```json
{ "ACTION": "TEAM_POINTS_BATCH_RESULT", "MSG": { "STATUS": "APPLIED", "QUESTION_ID": "9", "APPLIED": [ {"TEAM": "Les Bleus", "POINTS": 3} ], "SKIPPED": [ {"TEAM": "Les Rouges", "REASON": "ALREADY_AWARDED"} ], "TEAMS": [] } }
```

La régie (`TEAM_POINTS` par carte d'équipe) n'est pas contrainte par le verrou « déjà créditée ».

## Verrou de lancement — LAUNCH_LOCK (v11.2, #253)

Source de vérité : `contracts/websocket-actions.md` §LAUNCH_LOCK, §HELLO_ACK, §ACTION_REFUSED ; `contracts/game-state.md`
§`LAUNCH_LOCK`.

Message admin → serveur, envoyé à l'ouverture (`OPEN`) et toutes les 15 s, puis à la fermeture (`CLOSE`) :

```json
{ "ACTION": "LAUNCH_LOCK", "MSG": { "STATE": "OPEN" } }
```

- **Pose** : le verrou est posé au nom de la **connexion** qui envoie `OPEN` (le propriétaire). `OWNER` = son `CLIENT_ID`.
- **Refus** (`LAUNCH_LOCK_REFUSED`, ciblé, rien ne change) : `LOCKED_BY_OTHER` (verrou tenu par une autre connexion) ou
  `ENTRACTE_ACTIVE`. Un `OPEN` du propriétaire est toujours un renouvellement.
- **Libération** : `CLOSE` du propriétaire ; `NEW_GAME` appliqué du propriétaire ; déconnexion de la connexion ;
  expiration à **45 s** sans renouvellement. Les deux derniers cas sont journalisés en WARN
  (`LAUNCH_LOCK_RELEASED reason="disconnect"` ou `reason="expired"`).
- **Diffusion** : `GameState.LAUNCH_LOCK` aux clients `admin` et `anim` seulement. Absent des charges TV et VJoueur.
- **Actions suspendues** pour tout émetteur, propriétaire compris : `ACTION_REFUSED { ACTION, REASON: "LAUNCH_IN_PROGRESS" }`
  ciblé, journal `WARN ACTION_REFUSED_LAUNCH`. Liste : conduite (`READY`, `FORCE_READY`, `START`, `STOP`, `PAUSE`, `CONTINUE`, `REVEAL`,
  `REMOTE`, `QUESTION_SOUND`, `ENTRACTE_SET`, actions MEMORY, MEMOTION, RAFALE, `FLIP_MEMORY_CARD` hors tablettes : le retournement émis par une tablette est une action joueur, permise) ; points (`POINTS`,
  `TEAM_POINTS`, `BUMPER_POINTS`, lots, `SET_CREDIT_POINTS`, `RAZ`) ; `SHOW_QR_CODE`, `HIDE_QR_CODE`, `RESET` ;
  `UPDATE`/`FULL`, `DELETE_BUMPER`, `RELEASE_BUMPER_NAME`.
- **Toujours permises** : `LAUNCH_LOCK`, `NEW_GAME` du propriétaire, `HELLO`, `SET_CLIENT_TYPE`, `CLIENT_TRACE`,
  `SET_VIRTUAL_PLAYER_LIMIT`, `REBOOT`, édition des questions, réglages, et les actions des joueurs.
- **`NEW_GAME`** d'une autre connexion : `NEW_GAME_REFUSED { REASON: "LAUNCH_IN_PROGRESS" }`.

### HELLO_ACK (admin et anim seulement, v11.2 #253)

Envoyé en premier à chaque `HELLO` d'un poste admin ou anim, avant l'`UPDATE` initial. Il donne l'identifiant de la connexion,
que le client compare à `LAUNCH_LOCK.OWNER` pour se reconnaître.

```json
{ "ACTION": "HELLO_ACK", "MSG": { "CLIENT_ID": "192.168.1.20:53124" } }
```

`CLIENT_ID` identifie la connexion : c'est l'adresse distante `IP:port` (`WebSocketClient.ID`, `r.RemoteAddr`). Le client le compare sans l'interpréter. Il est unique par connexion et change à chaque reconnexion. Conséquence : `LAUNCH_LOCK.OWNER`, reçu par les postes admin et anim, expose l'adresse du poste propriétaire.

## Action NEW_GAME — options de purge (v11.2, #243)

Envoyée par `admin` depuis l'entrée « 🚀 Lancer la partie » du menu Préparation (popup à quatre options, #253). Contrat détaillé : `contracts/websocket-actions.md` §NEW_GAME.

```json
{ "ACTION": "NEW_GAME", "MSG": { "PURGE_SCORES": true, "PURGE_PLAYERS": false, "PURGE_TEAMS": false, "RESET_QUESTIONS": true } }
```

| Champ | Type | Effet |
|-------|------|-------|
| `PURGE_SCORES` | bool | Scores à 0 et historique des points effacé |
| `PURGE_PLAYERS` | bool | Joueurs des tablettes supprimés (`GAME_RESET`). Exige `PURGE_SCORES` |
| `PURGE_TEAMS` | bool | Équipes supprimées (`TEAM_DELETED`), joueurs conservés sans équipe |
| `RESET_QUESTIONS` | bool | Toutes les questions « à jouer » |

- **Payload vide** (clients antérieurs) : comportement v11.1 (scores et joueurs purgés, équipes conservées,
  questions remises à zéro).
- **Dès qu'un champ est présent**, les champs absents valent `false`.
- **Refus** (réponse ciblée `NEW_GAME_REFUSED`, rien n'est appliqué, `WARN NEW_GAME_REFUSED`) :
  - `PLAYERS_REQUIRE_SCORES` : `PURGE_PLAYERS = true` avec `PURGE_SCORES = false`.
  - `ENTRACTE_ACTIVE` : `NEW_GAME` reçu pendant une entracte. `NEW_GAME` n'est pas dans la liste
    d'autorisation de l'entracte : il faut d'abord sortir de l'entracte. Avant v11.2, le message était ignoré
    sans réponse et la popup restait figée.

```json
{ "ACTION": "NEW_GAME_REFUSED", "MSG": { "REASON": "PLAYERS_REQUIRE_SCORES" } }
```

- **Toujours** : arrêt de tous les minuteurs et du son de question, `TEAMS_LOCKED = false`, LED de repos
  (`LED_SET` à tous les buzzers) et notification d'ambiance, à chaque `NEW_GAME` appliqué.
- **Statut de la question interrompue** (sans `RESET_QUESTIONS`) : sélectionnée mais jamais lancée → `AVAILABLE` ;
  lancée (`STARTED`, `PAUSED`) → `STOPPED` ; `STOPPED`/`REVEALED` → inchangé.
- **Joueurs purgés** : les connexions liées à ces joueurs sont déliées dans le hub (une connexion liée désigne
  toujours un joueur existant).
- **Journal** : `WARN NEW_GAME_PURGE` (options, joueurs et équipes supprimés, phase et question avant le lancement).

## Action ENTRACTE_SET — Pause Globale (v6.5.2, #119)

### Action Entrante (Client → Server)

**Clientèle autorisée** : `admin` uniquement.

**Payload** :
```json
{
  "action": "ENTRACTE_SET",
  "payload": {
    "ACTIVE": true  // true = entrée en entracte, false = sortie
  }
}
```

**Comportement** :
- `ACTIVE = true` en phase autorisée → état change, diffusion `broadcastUpdate()` vers 
  tous les clients (admin, TV, VJoueur, animateur), **LEDs tous les buzzers éteints**.
- `ACTIVE = true` en phase non autorisée (`COUNTDOWN`, `STARTED`, `PAUSED`, `ENROLL`) → 
  refusée, log `WARN`, pas de modification.
- `ACTIVE = false` → désactivation immédiate (jamais refusée), diffusion, LEDs restaurées.

**Idempotent** — envoyer la même valeur est un no-op.

---

## Action UPDATE_ENTRACTE_CONFIG — Mise à jour Configuration Entracte (v6.5.2, #119)

### Action Entrante (Client → Server)

**Clientèle autorisée** : `admin` uniquement.

**Spécialité** : cette action est **autorisée même pendant l'entracte** — contrairement à la 
quasi-totalité des actions qui sont refusées (C4, gel de config).

**Payload** :
```json
{
  "action": "UPDATE_ENTRACTE_CONFIG",
  "payload": {
    "config": {
      "TITLE": "ENTRACTE",
      "SUBTITLE": "Retour dans 20mn",
      "IMAGE_IS_CUSTOM": false,
      "PANEL_SIZE": 65,
      "ANIM_PERIOD": 10,
      "ANIM_INTENSITY": 20
    }
  }
}
```

**Comportement** :
- Mise à jour de `GameState.ENTRACTE_CONFIG` (configuration courante).
- Diffusion immédiate via `broadcastUpdate()` à tous les clients admin.
- Si entracte actif (`ENTRACTE = true`) : les modifications **ne s'appliquent pas au panneau 
  affiché** (qui reste gelé sur `ENTRACTE_CONFIG_SAVED`) ; elles prennent effet au **prochain 
  cycle d'entracte** (après sortie + nouvelle entrée).
- Si entracte inactif : les modifications **s'appliquent immédiatement** au panneau (si jamais 
  déclenché pendant le jeu — cas rare).

---

## Clé ANSWERED_COLOR des bumpers (v11.2, #255)

Source : `contracts/ws-payload-serialization.md` (ligne `ANSWERED_COLOR`).

- Clé `ANSWERED_COLOR` présente dans les `bumpers` envoyés aux clients **admin**, **TV**, **anim** et **VJoueur**.
- Tolérée côté **buzzer** (le firmware BuzzClick ne la lit pas).
- Un client antérieur ignore la clé.

## Manches — ROUNDS_UPDATE, NEXT_QUESTION, ENTRACTE_SET (v11.2, #245)

Source de vérité : `contracts/websocket-actions.md` §ROUNDS_UPDATE, §NEXT_QUESTION, §ENTRACTE_SET ; `contracts/models.md` §Manches ;
`contracts/game-state.md` §Manches (`ROUND_CONTEXT`).

### ROUNDS_UPDATE (admin seulement, toutes phases sauf pendant une entracte active)

Fixe en une action la structure complète : manches (ordre, noms, questions), questions « Sans manche », suppressions.

```json
{ "ACTION": "ROUNDS_UPDATE", "MSG": { "ROUNDS": [ {"ID": "r1", "NAME": "Échauffement", "QUESTIONS": ["3", "1"]}, {"NAME": "Manche finale", "QUESTIONS": ["7"], "ENTRACTES": true} ], "UNASSIGNED": ["9"], "DELETED_ROUNDS": [ {"ID": "r2", "QUESTIONS": "KEEP"} ] } }
```

- `ROUNDS[].ID` : absent ou `""` = nouvelle manche (ID attribué par le serveur, jamais réutilisé). `ENTRACTES` (nouvelle manche seulement,
  absent = `true`) crée les deux entractes automatiques « Début/Fin de la manche <NOM> ».
- `DELETED_ROUNDS[].QUESTIONS` : `"KEEP"` (questions en fin de « Sans manche ») ou `"DELETE"` (supprimées définitivement).
- **Refus global** (`ROUNDS_UPDATE_RESULT { STATUS: "REFUSED", REASON }`, rien n'est appliqué, `WARN ROUNDS_UPDATE_REFUSED`) :
  `INVALID_NAME`, `TOO_MANY_ROUNDS` (plus de 50), `UNKNOWN_ROUND`, `INVALID_DECISION`, `MISSING_ROUND_DECISION`,
  `DUPLICATE_QUESTION`, `UNKNOWN_QUESTION`, `INCOMPLETE`, `QUESTION_ID_EXHAUSTED` (plus assez d'identifiants libres, de 1 à 999, pour créer les entractes demandées),
  `INCONSISTENT_STORE`, `ROUNDS_FILE_UNREADABLE`, `QUESTION_IN_PROGRESS` (suppression de la question courante hors repos), `WRITE_FAILED`.
- **Identité** : le nom du dossier `data/files/questions/<ID>/`. Aucun chemin n'est construit à partir d'un ID non validé.
- **Ordre des effets** : validation complète ; réservation des IDs ; écriture de `rounds.json` (échec : `WRITE_FAILED`) ;
  réécriture des `question.json` modifiés, puis création des entractes ; **suppressions en dernier** ; mise à jour de l'état et
  diffusions seulement en cas de succès.
- **Diffusions** : `UPDATE` (`ROUNDS`, `ROUND_CONTEXT`), `QUESTIONS` (admin), `NEXT_QUESTION` (anim).
- **Résultat** : `ROUNDS_UPDATE_RESULT { STATUS: "APPLIED", ROUNDS: [{ID, NAME}], CREATED_ENTRACTES: [{ROUND_ID, START_ID, END_ID}], DELETED_QUESTIONS: [ids] }`.

### NEXT_QUESTION étendu (anim)

- **Les ENTRACTE ne sont plus sautées** : `NEXT_QUESTION` propose une entrée `ENTRACTE` (`TYPE: "ENTRACTE"`, `QUESTION` = son libellé),
  pour toutes les entractes (manuelles, programmées, de manche).
- Champs ajoutés : `ROUND_ID` (manche de la suivante, `""` = Sans manche), `ROUND_NAME`, `NEW_ROUND` (`true` si la suivante ouvre une autre manche).
- Diffusé aussi après `ROUNDS_UPDATE`, `REORDER_QUESTIONS` et `DELETE`.

### ENTRACTE_SET (anim : arrêt seulement)

- L'anim peut envoyer `ENTRACTE_SET{ACTIVE:false}` (terminer une entracte) : toujours permis, même pendant le verrou de lancement.
- `ENTRACTE_SET{ACTIVE:true}` venant de l'anim est refusé (`WARN ENTRACTE_SET_ANIM_ACTIVATE_REFUSED`, rien n'est modifié).
  Le déclenchement manuel reste réservé à l'admin.

## Filtrage des réponses pour les tablettes — `/ws/player` (v11.2, #258)

Source de vérité : `contracts/vplayer-payload-filter.md` §7 (règle), §7.3 (implémentation), §7.5 (liste exhaustive A/B/C).
Fonction : `filterVPlayerAnswers(payload, recipientID, phase)` (package `protocol`), appliquée en dernière étape de sérialisation.

**Destinataire** : le joueur lié à la connexion (`PlayerID`). Son équipe : `bumpers[PlayerID].TEAM`. Une connexion non liée
n'a aucune équipe propre : tous les bumpers et toutes les équipes sont neutralisés pendant les réponses ouvertes.

| Donnée | Phases filtrées | Traitement pour la tablette |
|---|---|---|
| `bumpers` d'une autre équipe : `TIME`, `BUTTON`, `STATUS`, `ANSWER_COLOR`, `ANSWERED_COLOR`, `HINTS_AT_BUZZ` | réponses ouvertes : `PREPARE`, `READY`, `COUNTDOWN`, `STARTED`, `PAUSED` | valeurs neutres (`0`, `""`), clés conservées |
| `teams` d'une autre équipe : `TIME`, `BUMPER`, `STATUS` | mêmes phases | valeurs neutres |
| `QUESTION.QCM_CORRECT`, `QUESTION.ANSWER`, `QUESTION.MEDIA_ANSWER` | toutes sauf `REVEALED` | clés retirées |
| Cartes MEMOTION : `ANSWER_TEXT`, `ANSWER_IMAGE`, `ANSWER`, `QCM_CORRECT` | tant que la carte n'est pas `REVEALED` ou `DONE` | clés retirées |
| `QUESTION.EXPLANATION` (note animateur) | toutes | clé retirée |

- **Visible** : son propre bumper, ceux de ses coéquipiers, son équipe, les scores, les classements, les noms.
- **À partir de `STOPPED`** : les champs de réponse des autres équipes sont transmis, comme avant.
- **Bonne réponse** : transmise avec l'`UPDATE` de la transition `REVEALED`.
- **Chemins couverts** : `UPDATE` par destinataire pendant les phases filtrées, `SerializeForVPlayer`, les actions diffusées par type
  (`START`, `STOP`, `PAUSE`, `CONTINUE`, `UPDATE_TIMER`, `REMOTE`, `BUMPER`, `HELLO`), et l'état initial (`sendStateToClient`).
  Les messages que le client n'enregistre pas (`START`, `STOP`, `PAUSE`, `CONTINUE`, `UPDATE_TIMER`) sont sérialisés une fois pour toutes les tablettes.
- **Hors périmètre** : TV et anim inchangés. Buzzers physiques : payload minimal.
- **Coût** : banc `vplayer_fanout_bench_test.go`, 20 tablettes en `STARTED` : 1,56 ms par `UPDATE` après la revue (2,43 ms avant
  le décodage unique par fan-out ; trame partagée de référence : 0,20 ms) (`qa-20261011-145800`).
- **Limite** : `/questions`, `/files/`, `/ws/logs` et les connexions `/ws/tv`, `/ws/anim`, `/ws/admin` ne sont pas filtrés (issue #259).

## Sérialiseurs et Filtrage par Type de Client (v6.5.2, #128)

### Listes de Filtrage

Le serveur applique **deux listes distinctes** de retrait de champs `GameState`, selon le type de client :

#### AdminOnlyGameFields
Champs **réservés à l'admin**, retirés automatiquement pour tous les autres clients :
- `QUIZ_OBJECTIVES` — objectifs pédagogiques (jamais transmis à TV/VJoueur)
- `ENTRACTE_CONFIG_SAVED` — configuration gelée d'entracte (interne)

#### VPlayerOnlyGameFields
Champs **spécifiquement retirés pour le VJoueur uniquement** (pas de retrait pour TV/animateur) :
- `ARDOISE_ANSWERS` — réponses ARDOISE par équipe (confidentialité vers l'écran joueur, #128)

**Motif** : `ARDOISE_ANSWERS` est légitimement nécessaire à la TV (affichage au REVEAL) et à l'animateur 
(colonne équipes en direct) — seul le VJoueur (écran joueur) ne doit pas le recevoir.

### Points d'Application (v6.5.2, #128 — 4 sites)

**Avant #128**, le filtrage ne s'appliquait que sur `ActionUpdate`. Après correction, il s'applique sur 
**tous les payloads** transportant `GameState`, indépendamment de l'action (`STOP`, `START`, `PAUSE`, 
`UPDATE_TIMER`, etc.) :

1. **`SerializeForWebClient`** (`internal/protocol/messages.go`) — filtre tous les champs `GAME`
2. **`SerializeForVPlayer` (chemin PREPARE/READY)** — réimplémentation locale, utilise les mêmes listes
3. **`buildVPlayerPayloads`** (`cmd/server/main.go`) — ventilation par recipient VJoueur, applique les deux listes
4. **`broadcastUpdateTo`** (`cmd/server/main.go`) — diffusion `UPDATE` distincte par type de client

**Stratégie** : filtrage **par forme du payload** (présence d'un nœud `GAME`), non par énumération d'actions — 
garantit que tout nouveau broadcast transportant `GameState` bénéficie du filtrage sans modification future.

### Risques Résiduels Documentés

- **`/tv` sans authentification** (brut sur le réseau) — hors périmètre de ce correctif. `/tv` suppose un 
  réseau de confiance (salle de sport, café) ou une TV protégée par un pare-feu. Le filtrage réduit 
  l'exposition accidentelle du VJoueur, qui se connecte toujours via le même réseau.
- **Métadonnées buzzer** (`FIRMWARE_VERSION`, `OTA_STATUS`, etc.) : retiré du nœud `bumpers` pour le 
  VJoueur (pas d'admin-only strict, filtrage VJoueur sur ce nœud séparé).

---

## Actions Refusées Pendant l'ENTRACTE (v6.5.2, #119)

Quand `ENTRACTE = true`, une **liste blanche centralisée** (`IsActionAllowedDuringEntracte`) 
refuse automatiquement toutes les actions sauf :

| Action | Motif |
|--------|--------|
| `ENTRACTE_SET` | Sortir de l'entracte (jamais pas d'issue) |
| `HELLO` | Poignée de main — écran qui recharge pendant la pause |
| `SET_CLIENT_TYPE` | Changement de rôle (admin ↔ TV, etc.) |
| `PLAYER_CONNECT` | Écran joueur qui recharge |
| `REGIE_MESSAGE_SEND` | Régie communique avec animateurs pendant la pause |
| `REGIE_MESSAGE_CLEAR` | Régie efface message |
| `PONG` | Keepalive réseau, inoffensif |

**Toute autre action** (`START`, `READY`, `REVEAL`, `RAZ`, `NEW_GAME`, `FLIP_MEMORY_CARD`, 
etc.) est immédiatement refusée.

**Log** : action refusée → `WARN` (log distinct du refus d'allow-list habituel, 
distingue les deux causes de rejet).

**Réponse client** : aucune erreur renvoyée au client — le message est silencieusement 
ignoré (cohérent avec le comportement d'allow-list existant).

---

## Portée des actions par contexte — `MOTION_CARD_ID` (v7.0.0, #184, #185)

### Principe — champ `CardScope`

À partir de v7.0.0, les actions **typées** (actions dont l'effet dépend du type de la question courante — QCM, MEMORY, ARDOISE, SPEEDY…) peuvent indiquer leur **portée** via un champ optionnel `MOTION_CARD_ID`, embarqué dans la charge utile de l'action :

```jsonc
{
  "ACTION": "MEMOTION_DONE",
  "MSG": {
    "MOTION_CARD_ID": "mc-5",           // Optionnel : si présent, l'action cible cette carte
    "WINNER_TEAM": "Les Bleus",
    "UNITS": 1
  }
}
```

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| `MOTION_CARD_ID` | string | ❌ | ID de la carte MEMOTION si l'action cible une carte ; absent si elle cible une question classique. Jamais vide (`""`) — soit le champ est absent, soit il porte un ID valide |

**Aucune action existante ne devient obligatoirement porteuse de ce champ** — il est **optionnel** et son absence conserve le comportement actuel.

### Invariant de scope — vérifié côté serveur

Le serveur applique une **frontière d'autorisation** : une action typée ne peut jamais s'appliquer à une carte qui n'est pas celle en jeu.

| Situation | `MOTION_CARD_ID` reçu | Verdict | Code d'erreur |
|-----------|----------------------|---------|---|
| Aucune manche MEMOTION en cours (`MEMOTION_SUBPHASE == ""`) | absent | ✅ accepté — l'action cible la question courante | — |
| Aucune manche MEMOTION en cours | présent | ❌ refusé — aucune carte n'est active | `CARD_SCOPE_UNEXPECTED` |
| Manche MEMOTION, carte active | `== MEMOTION_SELECTED` (ID de la carte jouée) | ✅ accepté — l'action cible la bonne carte | — |
| Manche MEMOTION, carte active | absent | ❌ refusé — le scope doit être explicite | `CARD_SCOPE_MISMATCH` |
| Manche MEMOTION, carte active | `!= MEMOTION_SELECTED` (ID d'une autre carte) | ❌ refusé — tentative de ciblage croisé | `CARD_SCOPE_MISMATCH` |

**Refus silencieux interdit** — tout rejet dû à un scope invalide rend l'erreur au client, comme les autres `MotionError`.

### Exemple — Cas valides

```json
// Cas 1 : Question classique (SPEEDY), aucune manche MEMOTION
{ "ACTION": "MEMOTION_DONE", "MSG": { "WINNER_TEAM": "team_A", "UNITS": 1 } }
→ ✅ Accepté (scope implicite = question courante)

// Cas 2 : Carte MEMOTION active, scope explicite
{ "ACTION": "MEMOTION_DONE", "MSG": { "MOTION_CARD_ID": "mc-5", "WINNER_TEAM": "team_B", "UNITS": 1 } }
→ ✅ Accepté (scope correspond à MEMOTION_SELECTED)

// Cas 3 : Carte MEMOTION active, scope absent (interdit)
{ "ACTION": "MEMOTION_DONE", "MSG": { "WINNER_TEAM": "team_A", "UNITS": 1 } }
→ ❌ Refusé : CARD_SCOPE_MISMATCH (scope obligatoire en MEMOTION)

// Cas 4 : Aucune manche MEMOTION, scope présent (interdit)
{ "ACTION": "MEMOTION_DONE", "MSG": { "MOTION_CARD_ID": "mc-99", "WINNER_TEAM": "team_A", "UNITS": 1 } }
→ ❌ Refusé : CARD_SCOPE_UNEXPECTED (aucune carte active)
```

---

## Action `MEMOTION_DONE` — Champ `UNITS` (v7.0.0, #184, #185)

### Résultat typé avec progression optionnelle

À partir de v7.0.0, l'action `MEMOTION_DONE` (et toute action marquant la fin d'une manche typée) peut transporter un champ `UNITS` indiquant le **degré de succès** du joueur :

```jsonc
{
  "ACTION": "MEMOTION_DONE",
  "MSG": {
    "MOTION_CARD_ID": "mc-3",
    "WINNER_TEAM": "Les Rouges",
    "UNITS": 1           // 1 = gagné, 0 = perdu, > 1 réservé à la progression (v7.1.0+)
  }
}
```

| Champ | Type | Obligatoire | Valeur | Description |
|-------|------|-------------|--------|-------------|
| `UNITS` | int | ❌ | Défaut `1` | **`1`** = succès/victoire (défaut, comportement actuel inchangé) · **`0`** = échec/défaite · **`> 1`** réservé aux types à progression (MEMORY au prorata, #187, v7.1.0) |

**Absent ⇒ comportement actuel strictement identique.** Un client ou firmware antérieur qui n'envoie jamais ce champ fonctionne exactement comme avant.

### Utilisation — barème de points

Le champ `UNITS` est consommé par le **barème de points d'une carte** (`POINTS_RULE` de `MotionCard`) :

| Mode du barème | `Units = 0` | `Units = 1` | `Units > 1` (v7.1.0+) |
|---|---|---|---|
| `STARS` (défaut) | `0` points | Points par étoiles de difficulté | Idem (type rend `Units = 1` si tout réussi) |
| `FIXED` | `0` points | Montant fixe `POINTS_RULE.VALUE` | `0` points (n'existe pas en v7.1.0) |
| `PER_UNIT` | `0` points | `POINTS_RULE.VALUE × 1` | `POINTS_RULE.VALUE × Units` (progression linéaire) |

**Exemple** : une carte MEMORY avec `POINTS_RULE: { MODE: "PER_UNIT", VALUE: 10 }` :
- `UNITS: 4` → `4 × 10 = 40 points`
- `UNITS: 2` → `2 × 10 = 20 points`
- `UNITS: 0` → `0 × 10 = 0 points`

---

## Liste blanche entrante — inchangée en v7.0.0

### Aucune action joueur nouvelle, aucun élargissement

**v7.0.0 (#184, #185) n'ajoute AUCUNE action entrante** (client → server) au protocole WebSocket.

- ✅ `MEMOTION_DONE` reçoit le champ `UNITS` optionnel (rétrocompatible)
- ✅ Les actions existantes reçoivent un champ `MOTION_CARD_ID` optionnel (rétrocompatible)
- ❌ **Pas d'élargissement de la liste blanche** (`contracts/websocket-actions.md`)
- ❌ **Pas de nouvel action buzz ou saisie joueur**

Les **joueurs virtuels** (`/ws/player`) continuent de n'envoyer que :
- `PLAYER_CONNECT`, `ARDOISE_INPUT`, `BUTTON`, `VPLAYER_QCM_ANSWER`

Les **buzzers physiques** (`/ws/buzzer`) continuent de n'envoyer que :
- `HELLO`, `BUTTON`, `PONG`

L'interface **animateur** (`/ws/anim`) continue de n'avoir accès qu'aux actions de **conduite** (START/STOP/PAUSE/REVEAL/READY) et de **manipulation de cartes** (FLIP_MEMORY_CARD, MEMOTION_SELECT/FLIP/REVEAL/DONE) — aucune nouvelle action.

### Entrée joueur différée — v7.1.0 (#186, #187)

L'élargissement de la liste blanche pour permettre l'entrée **directe** des joueurs dans les types imbriqués (ARDOISE-en-carte, MEMORY-en-carte) est planifié pour **v7.1.0** (#186, #187) — exactement une version après le cœur MEMOTION+ (#185). Le périmètre exact (quelles actions, quel contrôle de phase) sera décidé à ce stade. **En v7.0.0, aucun changement ne doit être attendu côté client joueur.**

---

## Actions WebSocket — Mode RAFALE (v8.0.0, #16)

### Actions Entrantes (Client → Serveur)

| Action | ClientType | Payload | Description |
|--------|-----------|---------|-------------|
| `RAFALE_VALIDATE` | admin, anim | `{}` | Réponse jugée correcte — avance question |
| `RAFALE_INVALIDATE` | admin, anim | `{}` | Réponse jugée incorrecte — avance question |
| `RAFALE_SET_TEAMS` | admin | `{"TEAMS": ["team_A", "team_B"]}` | Sélection équipes + ordre rotation |

**Déclaration Allow-list requise** : `inbound_allowlist.go` (allow-list fermée, absence = rejet silencieux)

**Example JSON** :
```json
{ "ACTION": "RAFALE_VALIDATE", "MSG": {}, "ID": "<admin_id>" }
{ "ACTION": "RAFALE_INVALIDATE", "MSG": {}, "ID": "<anim_id>" }
{ "ACTION": "RAFALE_SET_TEAMS", "MSG": {"TEAMS": ["red", "blue"]}, "ID": "<admin_id>" }
```

### Actions Sortantes (Serveur → Client)

| Action | Destinataires | Payload | Description |
|--------|---|---------|-------------|
| `RAFALE_ANSWER` | **admin, anim uniquement** | `{"ID": "r-001", "ANSWER": "Rome", "NEXT": {...}}` | Réponse attendue + aperçu prochaine question (jamais TV/VPlayer) |
| `RAFALE_TICK` | tous | `{"QUESTION_TIME": 2500}` | Décompte timer question (µs) |
| `UPDATE_TIMER` | tous | `{"CURRENT_TIME": 95000}` | Décompte timer manche (inchangée, réutilisée) |

**Fuite zéro garantie** : `RAFALE_ANSWER` ne transite **jamais** via `GameState` — diffusion par site d'appel étroit (`broadcastRafaleAnswer`, patron `BroadcastToTypes(msg, ClientTypeAdmin, ClientTypeAnim)`). Champ `NEXT` **jamais** transmis à `/tv` ou `/player`, même par futur copier-colle.

**Champ NEXT — pré-tirage de la question suivante** (v8.0.0, #202) :
- **Présence** : `NEXT` est toujours présent dans `RAFALE_ANSWER`, même si `null` (information utile : pas de question suivante disponible, pool épuisé ou plafond atteint).
- **Contenu** : `NEXT = {"ID": "r-002", "QUESTION": "Capitale de la France ?", "CATEGORY": "géo", "DIFFICULTY": 2}` — **aucun champ `ANSWER`**, par sécurité (même régime que la réponse courante).
- **Destinataires** : `NEXT` ne quitte **jamais** les clients admin+anim — rejet précoce à la sérialisation pour `/tv` et `/player`.
- **Garantie moteur** : la question pré-tirée est réellement consommée au tick suivant (pas un aperçu jetable). Libération exhaustive en fin de manche ou STOP (flag `used` remis à disposition).

**Example JSON** :
```json
{ "ACTION": "RAFALE_ANSWER", "MSG": {"ID": "r-001", "ANSWER": "Rome", "NEXT": {"ID": "r-002", "QUESTION": "Capitale de la France ?", "CATEGORY": "géo", "DIFFICULTY": 2}} }
{ "ACTION": "RAFALE_ANSWER", "MSG": {"ID": "r-005", "ANSWER": "Paris", "NEXT": null} }
{ "ACTION": "RAFALE_TICK", "MSG": {"QUESTION_TIME": 2500} }
```

**Invariant de sérialisation** : `RAFALE_ANSWER` n'est émise que par callback dédié, jamais incluse dans une diffusion globale `UPDATE`. Deux champs sensibles protégés : `ANSWER` et `NEXT.QUESTION` (jamais aux clients non-admin).

---

## References

- [RFC 6455 - WebSocket Protocol](https://datatracker.ietf.org/doc/html/rfc6455)
- [ArduinoWebsockets Library](https://github.com/gilmaimon/ArduinoWebsockets)
- [gorilla/websocket (Go)](https://github.com/gorilla/websocket)
- [Contrat question-types.md](../contracts/question-types.md) — discriminant de type MEMOTION, portée des actions, invariant de scope
- [Contrat websocket-actions.md](../contracts/websocket-actions.md) — allow-list entrante par ClientType, table complète des actions
- [Contrat rafale.md](../contracts/rafale.md) — Mode RAFALE, actions entrantes/sortantes, sérialisation réponse
