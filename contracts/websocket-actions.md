# Actions WebSocket

> **Endpoint** : `/ws`
> **Format** : JSON
> **Dernière mise à jour** : 2026-01-27

---

## Client → Server

### HELLO

Enregistrement du client WebSocket.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Trigger   | Connexion WebSocket établie |

#### Payload

Aucun payload requis.

#### Exemple

```json
{
  "ACTION": "HELLO",
  "MSG": {}
}
```

---

### SET_CLIENT_TYPE

Définit le type de client (admin, TV, ou VJoueur virtuel).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Trigger   | Après connexion |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| TYPE | string | ✅ | `"admin"`, `"tv"`, ou `"vplayer"` |

#### Types de Clients

| Type | Description | Utilisation |
|------|-------------|-------------|
| admin | Interface d'administration | Page /admin - Contrôle du jeu |
| tv | Affichage TV/écran joueurs | Page /tv - Affichage public |
| vplayer | Joueur virtuel (WebSocket) | EnrollPage → VPlayerPage (mobile/web) |

#### Exemple

```json
{
  "ACTION": "SET_CLIENT_TYPE",
  "MSG": {
    "TYPE": "admin"
  }
}
```

#### VJoueur (v2.47.0+)

Les joueurs virtuels envoient cette action deux fois :
1. **EnrollPage** : Avant de s'inscrire (`connectVirtualPlayer`)
2. **VPlayerPage** : Après reconnexion WebSocket (confirmation du type)

---

### START

Démarre une question.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | READY |
| Trigger   | Clic bouton "Démarrer" |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| DELAY | int | ❌ | Durée en secondes (défaut: question.TIME) |

#### Exemple

```json
{
  "ACTION": "START",
  "MSG": {
    "DELAY": 30
  }
}
```

---

### STOP

Arrête la question en cours.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | STARTED, PAUSED |

#### Payload

Aucun.

---

### PAUSE

Met en pause la question.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | STARTED |

#### Payload

Aucun.

---

### CONTINUE

Reprend après pause.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | PAUSED |

#### Payload

Aucun.

---

### READY

Sélectionne une question.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | STOP |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| QUESTION | string | ✅ | ID de la question |

#### Exemple

```json
{
  "ACTION": "READY",
  "MSG": {
    "QUESTION": "5"
  }
}
```

---

### REVEAL

Affiche la réponse.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | STOPPED |

#### Payload

Aucun.

---

### REMOTE

Change la vue TV.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| REMOTE | string | ✅ | `"GAME"`, `"SCORE"`, `"PLAYERS"`, `"PALMARES"` |

#### Exemple

```json
{
  "ACTION": "REMOTE",
  "MSG": {
    "REMOTE": "SCORE"
  }
}
```

---

### TEAM_POINTS

Modifie le score d'une équipe.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| TEAM | string | ✅ | Nom de l'équipe |
| POINTS | int | ✅ | Points à ajouter (peut être négatif) |

---

### BUMPER_POINTS

Modifie le score d'un joueur.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| ID | string | ✅ | MAC du bumper |
| POINTS | int | ✅ | Points à ajouter |

---

### UPDATE

Met à jour les équipes/joueurs.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| teams | object | ❌ | Map des équipes |
| bumpers | object | ❌ | Map des joueurs |

---

### RAZ

Remet tous les scores à zéro.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

Aucun.

---

### DELETE

Supprime une question.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| ID | string | ✅ | ID de la question |

---

### REORDER_QUESTIONS

Réordonne les questions.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| ORDER | []string | ✅ | IDs dans le nouvel ordre |

#### Exemple

```json
{
  "ACTION": "REORDER_QUESTIONS",
  "MSG": {
    "ORDER": ["3", "1", "5", "2", "4"]
  }
}
```

---

## Server → Client

### UPDATE

Broadcast l'état complet du jeu.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Trigger   | Tout changement d'état |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| GAME | GameState | État du jeu (voir game-state.md) |
| teams | object | Map des équipes |
| bumpers | object | Map des joueurs |
| VERSION | string | Version du serveur |

#### CONN_STATE (v5.7.13, #109)

Chaque bumper dans `bumpers` porte désormais `CONN_STATE` (voir `contracts/models.md`),
propagé sur les 3 endpoints (`/ws/admin`, `/ws/tv`, `/ws/player`) ainsi qu'aux buzzers
physiques (`/ws/buzzer`, whitelist `buzzerBumperKeys`). Valeurs : `""`/`"orange"`/`"red"`/`"green"`.
Seuls les bumpers participants (`TEAM != ""`) portent un état visible.

Table de transitions (`engine.TransitionConn(bumperID, event)`), `event` ∈ `DISCONNECT` |
`RECONNECT` | `MESSAGE_LOST` | `DELIVERY_CONFIRMED` :

| État courant | DISCONNECT | RECONNECT | MESSAGE_LOST | DELIVERY_CONFIRMED |
|---|---|---|---|---|
| `""` (HIDDEN) | → `orange` | (n/a) | → `""` | → `""` |
| `orange` | `orange` | → `green` | → `red` | (n/a) |
| `red` | `red` | → `green` | `red` | (n/a) |
| `green` | → `orange` | `green` | (n/a) | → `""` |

> **Phase 1 (#109, v5.7.13)** : `DISCONNECT`/`RECONNECT` câblés sur les 6 sites de connexion
> existants (buzzer HELLO/déco, VJoueur reco/déco, reset boot, création VJoueur).
>
> **Phase 2 (#109, v5.7.14)** : `MESSAGE_LOST` et `DELIVERY_CONFIRMED` câblés sur leurs sources
> réelles + fenêtre minimale de 2s sur l'état `green` (D2/D3) :
> - **Buzzer** (fidèle) : `MESSAGE_LOST` sur toute émission LED_SET/OTA_UPDATE/WIFI_CONFIG vers
>   un buzzer déconnecté ; `DELIVERY_CONFIRMED` sur ACK réel (`handleBuzzerACK`).
> - **VJoueur** (heuristique large, D4 — pas de liste restreinte) : `MESSAGE_LOST` sur chaque
>   broadcast `UPDATE` (GameState) alors que le participant est déconnecté ; `DELIVERY_CONFIRMED`
>   sur chaque broadcast `UPDATE` alors qu'il est connecté, **et** sur tout message reçu de lui
>   (`handleWebMessage`, n'importe quelle action).
> - **Fenêtre `green`** : `DELIVERY_CONFIRMED` reçu avant que 2 s se soient écoulées depuis le
>   passage à `green` est différé (timer interne) — la transition vers `""` n'a lieu qu'une fois
>   la fenêtre écoulée. Pas de borne max (D7) : sans confirmation, reste `green` indéfiniment.
>   Entrée gated : `engine.ConfirmDelivery(bumperID)` (utilisée par tous les sites ci-dessus) —
>   `TransitionConn(id, DELIVERY_CONFIRMED)` reste immédiat/non-gated (réservé aux tests de la
>   table pure).
>
> **Fix conn-state (#109, v5.7.21)** : le broadcast `UPDATE` qui **annonce** la déconnexion d'un
> VJoueur (déclenché juste après avoir posé `CONNECTED=false`) ne compte **plus** comme un
> `MESSAGE_LOST` pour ce même VJoueur — sans cette exception, `orange` n'était jamais visible
> (bascule directe vers `red` dans le même broadcast que l'annonce de la déconnexion elle-même).
> Seuls les broadcasts **suivants**, alors que le participant est toujours déconnecté, comptent
> comme message manqué (D4 inchangé). Mécanisme : `Bumper.skipNextMessageLost` (interne,
> non sérialisé), posé à chaque transition `DISCONNECT` vers `orange`, consommé une seule fois par
> `ApplyVPlayerBroadcastConnEvents`.

---

### QUESTIONS

Liste des questions.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Trigger   | Changement de questions |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| /files/questions/{id} | Question | Map des questions |
| FSINFO | object | Infos stockage |
| VERSION | string | Version serveur |

---

### CLIENTS

Compteurs de clients connectés (3 types depuis v2.47.0).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Trigger   | Connexion/déconnexion client |
| Broadcast | Tous les clients |

#### Payload

| Champ | Type | Description | Depuis |
|-------|------|-------------|--------|
| ADMIN_COUNT | int | Nombre d'interfaces admin | v2.0 |
| TV_COUNT | int | Nombre d'affichages TV | v2.0 |
| VPLAYER_COUNT | int | Nombre de joueurs virtuels | v2.47.0 |

#### Exemple

```json
{
  "ACTION": "CLIENTS",
  "MSG": {
    "ADMIN_COUNT": 2,
    "TV_COUNT": 1,
    "VPLAYER_COUNT": 5
  }
}
```

#### Historique des Versions

- **v2.0-v2.46.0** : 2 compteurs (admin, tv)
- **v2.47.0+** : 3 compteurs (admin, tv, vplayer) - Identification sécurisée des joueurs virtuels

---

### QCM_HINT

Invalide une réponse QCM (indice).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Phase     | STARTED |
| Trigger   | Seuil de temps atteint |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| COLOR | string | Couleur invalidée (`RED`, `GREEN`, `YELLOW`, `BLUE`) |
| REMAINING | int | Nombre de réponses restantes |

#### Exemple

```json
{
  "ACTION": "QCM_HINT",
  "MSG": {
    "COLOR": "RED",
    "REMAINING": 3
  }
}
```

---

### BACKGROUND_CHANGE

Changement de fond d'écran (synchronisé).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Trigger   | Cycle automatique serveur |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| INDEX | int | Index du fond (0-based) |

---

### LOG_HISTORY

Historique des logs (connexion WebSocket /ws/logs).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Endpoint  | `/ws/logs` |
| Trigger   | Connexion à /ws/logs |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| logs | []LogEntry | Tableau des logs |

---

### LOG_ENTRY

Nouveau log temps réel.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Endpoint  | `/ws/logs` |
| Trigger   | Nouveau log serveur |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| timestamp | string | ISO 8601 |
| level | string | DEBUG, INFO, WARN, ERROR |
| component | string | App, Engine, HTTP, etc. |
| message | string | Message du log |

---

## VPlayer (Joueurs Virtuels)

### SHOW_QR_CODE

Active l'affichage du QR Code.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Trigger   | Admin ouvre inscriptions |

---

### HIDE_QR_CODE

Désactive le QR Code.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Trigger   | Admin ferme inscriptions |

---

### PLAYER_CONNECT

Demande d'inscription ou de reconnexion VPlayer.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Trigger   | VPlayer soumet son pseudo, ou reconnexion automatique |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| NAME | string | ✅ | Pseudo du joueur |
| ID | string | ❌ | ID de bumper reçu dans un `PLAYER_CONNECTED` précédent (fix R1, #109, v5.7.16). Absent au tout premier enrôlement. Champ `omitempty` — les anciens clients qui ne l'envoient jamais continuent de fonctionner comme un enrôlement par nom. |

#### Comportement serveur (matrice de décision — `engine.ReconnectOrCreateVirtualPlayer`)

| # | Situation | Décision |
|---|---|---|
| 1 | `ID` fourni et résout un bumper `IsVirtual` existant | Reconnexion — réutilise le bumper (badge → `green`), pas d'ambiguïté possible |
| 2 | `ID` fourni mais introuvable (supprimé par l'admin, périmé, ...) | Traité comme si aucun ID n'était fourni → cas 3/4 |
| 3 | Pas d'`ID` (ou introuvable), nom déjà pris par un autre VJoueur (connecté **ou** déconnecté) | **`PLAYER_REJECTED { REASON: "NAME_TAKEN" }`** — jamais de fusion/remplacement |
| 4 | Pas d'`ID` (ou introuvable), nom libre | Nouvel enrôlement — un nouveau bumper est créé, son ID est renvoyé dans `PLAYER_CONNECTED.ID` |

> **Important** : l'identité d'un VJoueur repose désormais **uniquement sur l'ID**, jamais sur le
> nom seul. Un client doit conserver l'`ID` reçu (ex. `localStorage`) et le renvoyer à chaque
> reconnexion. Sans ID, un nom déjà utilisé est **toujours rejeté**, jamais consolidé — voir
> `contracts/CHANGELOG.md` [20260725] Fix R1 pour le contexte (ancien comportement retiré :
> fusion/suppression silencieuse de bumpers homonymes, bloquant en code-review).

---

### PLAYER_CONNECTED

Confirmation d'inscription ou de reconnexion.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| ID | string | ID du bumper — **le client DOIT le conserver** (ex. `localStorage`) et le renvoyer dans `PLAYER_CONNECT.ID` pour toute reconnexion future (fix R1, #109) |
| NAME | string | Pseudo confirmé |

---

### PLAYER_REJECTED

Refus d'inscription ou de reconnexion.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| REASON | string | `ENROLLMENT_CLOSED`, `LIMIT_REACHED`, `INVALID_NAME`, ou `NAME_TAKEN` (fix R1, #109, v5.7.16 — nom déjà pris par un autre VJoueur, sans ID résolvable pour prouver la propriété) |

---

### PLAYER_EVICTED

Notification adressée à **un seul client VJoueur** dont le bumper vient de disparaître du roster
côté serveur (#120, v5.9.x). Remplace la détection par balayage du roster côté client, qui ne
pouvait pas distinguer « bumper supprimé » de « bumper pas encore reçu » et provoquait des
renvois silencieux à la page d'inscription pendant la fenêtre d'enrôlement.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` (ciblé, jamais broadcast) |
| Émis vers | le client dont `SetClientPlayerID` correspond au bumper supprimé |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| REASON | string | `PLAYER_REMOVED` (fiche supprimée par l'animateur) ou `GAME_RESET` (roster VJoueur purgé par `InitGame` sur NEW_GAME) |

#### Règles

- Le client qui reçoit `PLAYER_EVICTED` efface sa session locale (`vplayer_name`,
  `vplayer_session`, `vplayer_id`) et retourne à la page d'inscription **en affichant le motif**.
- L'absence d'un bumper dans une mise à jour de roster n'est **jamais** à elle seule un motif de
  renvoi : seul `PLAYER_EVICTED` fait autorité. C'est la règle qui ferme la course de #120.
- Un client qui ignore cette action (version antérieure) conserve le comportement précédent —
  action purement additive.

---

### ENROLLMENT_UPDATE

Mise à jour compteur inscriptions.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| VIRTUAL_PLAYER_COUNT | int | Nombre inscrits |
| VIRTUAL_PLAYER_LIMIT | int | Limite max |
| ENROLLMENT_ACTIVE | bool | Inscriptions ouvertes |

---

## Actions Système

### UPDATE_TIMER

Broadcast périodique du timer.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Trigger   | Chaque seconde pendant STARTED |

#### Payload

Même structure que UPDATE (GAME, teams, bumpers).

---

### RESET

Réinitialisation complète du serveur.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

Aucun.

---

### REBOOT

Redémarre le serveur.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

Aucun.

---

### PING

Vérification de connexion (buzzers).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |

#### Payload

Aucun.

---

### PONG

Réponse au PING.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

Aucun.

---

### FULL

Mise à jour complète (équipes + joueurs).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Trigger   | Modification équipes/joueurs |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| teams | object | Map complète des équipes |
| bumpers | object | Map complète des joueurs |

---

### DELETE_BUMPER

Supprime un joueur.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| ID | string | MAC ou ID du joueur |

---

### FORCE_READY

Force la transition PREPARE → READY (debug).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | PREPARE |

#### Payload

Aucun.

---

### PLAYER_ASSIGNED

Notification d'assignation VPlayer.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| ID | string | ID du joueur |
| TEAM | string | Nom de l'équipe |
| ANSWER_COLOR | string | Couleur assignée (RED/GREEN/YELLOW/BLUE) |

---

### BUTTON

Simulation buzzer (Ctrl+clic admin).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | STARTED, PAUSED |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| ID | string | MAC du bumper à simuler |
| button | string | Bouton ("A", "B", "C", "D") |

---

### FLIP_MEMORY_CARD

Retourne une carte Memory.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | STARTED |
| Type      | MEMORY |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| CARD_ID | string | ID de la carte (ex: "1-1", "2-2") |

---

### SET_VIRTUAL_PLAYER_LIMIT

Définit la limite de VPlayers.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| LIMIT | int | Nombre max de VPlayers |

---

### ARDOISE_INPUT ✨ (v5.6.0)

Mise à jour de la réponse texte d'une équipe pour une question ARDOISE.
Envoyé par le VPlayer (throttlé ~200ms). Envoi forcé sur réception STOP/PAUSE.

| Propriété | Valeur |
|-----------|--------|
| Direction | `VPlayer → Server` |
| Endpoint  | `/ws/player` |
| Phase     | `STARTED` |
| Type question | `ARDOISE` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| TEXT | string | Texte complet saisi (pas un delta) |

#### Exemple

```json
{
  "ACTION": "ARDOISE_INPUT",
  "MSG": {
    "TEXT": "Paris"
  }
}
```

#### Comportement serveur

1. **Guard phase** : ignore silencieusement si `phase ≠ STARTED`
2. **Guard type** : ignore silencieusement si `question.TYPE ≠ "ARDOISE"`
3. **Résolution équipe** : `clientID → bumper → bumper.Team` (protocole natif, aucun champ TEAM dans le payload)
4. Mise à jour `GameState.ARDOISE_ANSWERS[teamName] = { TEXT, SUBMITTED_AT }`
5. Broadcast `UPDATE` immédiat vers admin/TV

**Fire-and-forget** : aucune réponse du serveur vers le VPlayer.

---

## UPDATE_QUIZ_META

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Phase     | Toutes (édition de quiz, principalement NEW_GAME) |
| Trigger   | Bouton « Enregistrer » de la section Quiz (QuestionsPage) |

### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| NAME | string | ✅ | Nom du quiz (v4.0.0) |
| THEME | string | ✅ | Thème général (v4.0.0) |
| NOTES | string | ✅ | Texte libre (v4.0.0) — **affiché aux joueurs** sur l'écran TV NEW_GAME |
| POPULATIONS | string[] | ➕ **v6.1.0** | Publics cibles — énumération `ai-generation.md` §6, ≥ 1 élément. ⚠️ **remplace `POPULATION`** (string, v6.0.0) |
| DIFFICULTIES | string[] | ➕ **v6.1.0** | Difficultés visées — énumération `ai-generation.md` §6, ≥ 1 élément. ⚠️ **remplace `DIFFICULTY`** (string, v6.0.0) |
| LANGUAGE | string | ➕ v6.0.0 | Langue du quiz, défaut `Français` — valeur unique, inchangé |
| OBJECTIVES | string | ➕ **v6.1.0** | Objectif de la partie, texte libre ≤ 2000 car. — **jamais diffusé vers `/ws/tv` ni `/ws/player`** (`game-state.md`) |
| HIDDEN_FIELDS | string[] | ➕ **v6.1.0** | Champs non affichés sur l'écran TV — ⊂ `THEME`, `POPULATIONS`, `DIFFICULTIES`, `LANGUAGE` ; `[]` = tout est affiché. Valeur inconnue **ignorée**, jamais une erreur (`game-state.md`, règle H2) |

### Exemple (v6.1.0)

```json
{
  "ACTION": "UPDATE_QUIZ_META",
  "MSG": {
    "NAME": "Quiz ciné",
    "THEME": "Cinéma français des années 80",
    "NOTES": "",
    "POPULATIONS": ["Ado (13-17 ans)", "Adulte (18-64 ans)"],
    "DIFFICULTIES": ["Moyen", "Difficile"],
    "LANGUAGE": "Français",
    "OBJECTIVES": "Soirée du club — que chaque équipe marque au moins une fois",
    "HIDDEN_FIELDS": ["DIFFICULTIES"]
  }
}
```

### Sémantique par champ — « absent = inchangé »

| Cas | Effet |
|-----|-------|
| Clé **absente** du `MSG` | Valeur courante **préservée** |
| Clé présente, `[]` (tableaux) ou `""` (`OBJECTIVES`) | **Effacement explicite** |

Imposée par `contracts/ai-generation.md` §7 (pointeurs dans `QuizMetaPayload`,
`messages.go:221-233`) : sans elle, tout émetteur n'envoyant qu'une partie du formulaire efface
le reste.

> ⚠️ **v6.1.0 — rétrocompatibilité rompue.** `POPULATION` et `DIFFICULTY` (singuliers) ne sont
> plus reconnus : un client de v6.0.0 qui les envoie voit ces deux champs **ignorés** (clés
> inconnues ⇒ valeurs courantes préservées, pas de corruption), son enregistrement est donc
> partiellement sans effet. **Aucun repli** vers les anciens noms — cf. `ai-generation.md` §3ter.

---

## AI_GENERATION_PROGRESS

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Endpoint  | `/ws/admin` **uniquement** |
| Trigger   | Fin de chaque lot, fin de job, erreur, annulation, et connexion d'un admin pendant un job |

### Payload

| Champ | Type | Description |
|-------|------|-------------|
| JOB_ID | string | Identifiant du job |
| STATE | string | `RUNNING` / `DONE` / `FAILED` / `CANCELLED` |
| BATCHES_DONE / BATCHES_TOTAL | int | Progression par lots |
| CREATED_COUNT / SKIPPED_COUNT | int | Cumulatifs sur le job |
| ERROR_CODE | string | Codes stables de `ai-generation.md` §3, plus `provider_quota` |
| PROVIDER | string | `anthropic` / `groq` |

---

## CANCEL_AI_GENERATION

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Payload   | `{ "JOB_ID": "…" }` |

Prend effet **entre deux lots**. Les questions déjà écrites sont **conservées**.

> **Contrat détaillé** : `contracts/ai-multi-provider.md` §10 et §11.
