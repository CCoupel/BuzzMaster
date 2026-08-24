# Actions WebSocket

> **Endpoint** : `/ws`
> **Format** : JSON
> **Dernière mise à jour** : 2026-08-13 (#155/#156 — allow-list animateur, action NEXT_QUESTION, v6.2.0)

---

## Sécurité — Allow-list entrante par ClientType (#154, v6.1.4 · étendue #155/#156, v6.2.0)

Le serveur distingue plusieurs types de client WebSocket (`admin`, `tv`, `vplayer`, `anim` —
voir SET_CLIENT_TYPE ci-dessous ; `buzzer` existe côté firmware sur un hub séparé,
non concerné par cette table) déjà en **sortie** (sérialiseurs `SerializeForAdmin` /
`SerializeForWebClient` / `SerializeForVPlayer` / `SerializeForBuzzer`,
`contracts/ws-payload-serialization.md`). Depuis #154, il applique désormais la même
distinction en **entrée** : `handleWebMessage` (`cmd/server/main.go`) rejette (avec
log `WARN`) toute action envoyée par un type de client non autorisé, avant tout
traitement — `internal/server/inbound_allowlist.go` (`IsActionAllowed`) est la
source de vérité.

**Avant #154** : aucune vérification n'existait — un client connecté sur `/ws/tv` ou
`/ws/player` (censés être des vues à capacités réduites) pouvait envoyer n'importe
quelle action, y compris START/STOP/RAZ/DELETE/NEW_GAME/BUMPER_POINTS, et le serveur
l'exécutait comme si un admin l'avait envoyée.

**#155/#156 (v6.2.0)** ajoutent le type `anim` (`/ws/anim`, interface animateur — conduite du jeu
depuis une tablette). Conformément à la conception de #154 (« indexé par nom d'action, pas par
handler, pour qu'un futur `ClientType` n'ait besoin que de nouvelles entrées ici »), l'ajout se
fait **exclusivement par de nouvelles entrées dans la map** — aucun changement du mécanisme
`IsActionAllowed`/`IsSetClientTypeAllowed` ni du point d'accroche dans `handleWebMessage`.

| Action | `admin` | `tv` | `vplayer` | `anim` |
|---|---|---|---|---|
| HELLO | ✅ | ✅ | ✅ | ✅ |
| START, STOP, PAUSE, CONTINUE, REVEAL, READY | ✅ | ❌ | ❌ | ✅ |
| BUMPER_POINTS, TEAM_POINTS | ✅ | ❌ | ❌ | ✅ |
| FULL, UPDATE, POINTS, RAZ, REMOTE, DELETE, DELETE_BUMPER, RELEASE_BUMPER_NAME, RESET, REBOOT, REORDER_QUESTIONS, FORCE_READY, MEMORY_SET_TEAMS, MEMOTION_SET_TEAMS, SHOW_QR_CODE, HIDE_QR_CODE, SET_VIRTUAL_PLAYER_LIMIT, NEW_GAME, UPDATE_QUIZ_META, CANCEL_AI_GENERATION | ✅ | ❌ | ❌ | ❌ |
| **SET_CREDIT_POINTS** | ✅ | ❌ | ❌ | ❌ |
| **REGIE_MESSAGE_SEND** | ✅ | ❌ | ❌ | ❌ **(v6.4.x, #167)** |
| **REGIE_MESSAGE_CLEAR** | ✅ | ❌ | ❌ | ✅ **(v6.4.x, #167)** |
| BUTTON, PONG | ✅ | ❌ | ✅ | ❌ |
| FLIP_MEMORY_CARD | ❌ | ✅ | ✅ | ✅ **(v6.2.x, #159)** |
| MEMOTION_SELECT | ❌ | ✅ | ❌ | ✅ **(v6.2.x, #160)** |
| MEMOTION_FLIP, MEMOTION_STOP_TIMER, MEMOTION_REVEAL, MEMOTION_DONE | ✅ | ❌ | ❌ | ✅ **(v6.2.x, #160)** |
| PLAYER_CONNECT, VPLAYER_QCM_ANSWER, ARDOISE_INPUT | ❌ | ❌ | ✅ | ❌ |

Notes :
- **SET_CREDIT_POINTS** (v6.2.0, code review MAJEUR-1 sur #155/#156, voir §"Animateur" ci-dessous)
  — `admin` uniquement, jamais `anim` : l'animateur **reçoit** le montant courant via
  `CREDIT_POINTS`, il ne le **fixe** jamais lui-même. Placée à part plutôt que dans le gros bloc
  admin-only ci-dessus pour rester visible comme un ajout de ce lot, même traitement que la
  séparation faite plus haut pour la « conduite en direct ».
- **START, STOP, PAUSE, CONTINUE, REVEAL, READY, BUMPER_POINTS, TEAM_POINTS** sont désormais
  partagées entre `admin` et `anim` (v6.2.0, #155/#156) — c'est le périmètre exact de « conduite en
  direct » retenu pour l'interface animateur (`_work/reports/plan-20260812-141735.md` §1) : lancer/
  arrêter/mettre en pause une question et créditer une équipe ou un joueur, sans les actions de
  préparation ni de configuration (qui restent `admin`-only, ligne ci-dessous). Cette ligne était
  auparavant fusionnée avec le gros bloc `admin`-only ; elle en est extraite ici pour l'ajout de la
  colonne `anim`, sans changement de comportement pour `admin`/`tv`/`vplayer`.
- **BUTTON, PONG** sont à double usage — ce ne sont PAS des actions admin-only :
  `vplayer` les envoie pour le gameplay réel (`VPlayerPage.jsx:386` PONG = handshake
  de disponibilité en PREPARE ; `VPlayerPage.jsx:429,560` BUTTON = l'appui sur le
  buzzer lui-même, `handleBuzz`) ; `admin` les envoie aussi, via les outils de
  simulation debug de `GamePage.jsx` (`simulateButton`/`simulatePong`). Corrigé en
  v6.1.4.1 suite revue code (le lot #154 initial les avait classées admin-only par
  erreur — l'audit frontend s'était appuyé sur les noms de fonctions wrapper
  `simulateButton`/`simulatePong` et avait manqué l'appel direct `sendMessage(...)`
  de `VPlayerPage.jsx`, hors de tout wrapper nommé). **`anim` n'y est pas ajouté** : la conduite
  animateur ne simule pas de buzzer et ne fait pas de handshake PREPARE en son nom propre — hors
  périmètre #155/#156.
- **FLIP_MEMORY_CARD** est autorisé pour `tv` car cette connexion couvre deux cas
  légitimes : l'aperçu admin en iframe (`/tv?admin=true`, toujours une vraie
  connexion `tv` — `PlayerDisplay.jsx` `isAdminPreview`) et le clic d'un spectateur
  sur l'écran public. `vplayer` peut retourner ses propres cartes en mode équipe
  active (`MEMORY_CURRENT_TEAM`). MEMORY/MEMOTION restent hors périmètre de l'interface
  animateur en #155/#156 — `anim` n'y figure pas.
- **FLIP_MEMORY_CARD — aucune modification de cette table en v7.1.0 (#187).** Une carte MEMOTION de
  type `MEMORY` fait retourner des cartes par un joueur, mais l'action est **déjà** autorisée pour
  `vplayer` (et `tv`/`anim`). Ce qui est nouveau est la **portée** (`MOTION_CARD_ID`) et la
  **vérification du tour côté serveur**, pas le droit d'émettre — voir la fiche `FLIP_MEMORY_CARD`
  ci-dessous. C'est le premier type imbriqué à accepter un geste de joueur sur une carte, sans
  qu'aucune ligne de cette table ne bouge : c'est exactement ce que la conception de #154 visait
  (« indexé par nom d'action, pas par handler »).
- **FLIP_MEMORY_CARD — `anim` ajouté en v6.2.x (#159)**, seule modification de cette table par ce
  lot. Motif : l'animateur retourne les cartes du doigt sur sa tablette (#159), et **aucun chemin ne
  le lui permettait**. La régie, elle, n'en a jamais eu besoin en direct — elle retourne les cartes
  depuis son aperçu TV en iframe, qui est une connexion `tv` : c'est pourquoi `admin` reste ❌, et le
  reste. Conformément à la conception de #154, l'ajout se fait **par une seule entrée dans la map**,
  sans toucher au mécanisme.
  > ⚠️ C'est un **élargissement de capacité** pour `anim`, à traiter comme tel : le retournement
  > modifie l'état de jeu (paires trouvées, erreurs, tour de l'équipe active). Il est cohérent avec
  > le périmètre « conduite en direct » déjà accordé à l'animateur (START/STOP/REVEAL/READY,
  > TEAM_POINTS), et il reste borné par le moteur, qui refuse tout retournement hors phase
  > `STARTED` (`Engine.FlipMemoryCard`).
  >
  > **`MEMORY_SET_TEAMS` n'est PAS ajouté** : le choix des équipes participantes reste à la régie
  > (périmètre explicite de #159). Conséquence assumée : un animateur seul ne peut pas démarrer une
  > partie MEMORY multi-équipes.
- **MEMOTION_SELECT** était envoyé jusqu'en #159 uniquement depuis l'aperçu admin en iframe —
  toujours une connexion `tv`, jamais `vplayer`. `anim` s'y ajoute en #160 (ci-dessous). `vplayer`
  reste ❌ : contrairement à MEMORY, aucune carte MEMOTION n'est retournée par un joueur.
- **MEMOTION_SELECT / MEMOTION_FLIP / MEMOTION_STOP_TIMER / MEMOTION_REVEAL / MEMOTION_DONE —
  `anim` ajouté en v6.2.x (#160)**, seule modification de cette table par ce lot. Motif identique à
  #159 : l'animateur conduit les cinq sous-phases MEMOTION depuis sa tablette
  (`MEMORIZE` → `GRID` → `SELECTED` → `QUESTION` → `REVEAL`), et **aucun chemin ne le lui
  permettait** — la régie passe, elle, par son aperçu TV en iframe (connexion `tv`) pour
  `MEMOTION_SELECT`, et par `/admin` pour les quatre autres. `admin` conserve donc ses ✅ existants
  et `tv` conserve `MEMOTION_SELECT` : ce lot **n'enlève aucun droit à personne**, il n'ajoute
  qu'une colonne. Conformément à la conception de #154, l'ajout se fait **par cinq entrées dans la
  map**, sans toucher au mécanisme `IsActionAllowed`.
  > ⚠️ **Élargissement de capacité** pour `anim`, du même ordre que #159 mais plus large : ces
  > actions modifient l'état de jeu (sous-phase, état des cartes, **attribution de points** et
  > rotation de l'équipe active via `MEMOTION_DONE`). Ce n'est pas une classe de capacité nouvelle
  > pour l'animateur — il peut déjà créditer n'importe quelle équipe via `TEAM_POINTS` depuis
  > #155/#156 — mais c'est le premier chemin par lequel il déclenche une attribution de points
  > *calculée par le moteur*. Chaque action reste bornée par les gardes de sous-phase du moteur
  > (`SelectMotionCard` exige `GRID` + carte `UNPLAYED`, `FlipMotionCard` exige `SELECTED`,
  > `RevealMotionCard` exige `QUESTION`, `DoneMotionCard` exige `SELECTED|QUESTION|REVEAL`), toutes
  > inchangées par ce lot — aucune garde ajoutée, aucun changement moteur.
  >
  > ⚠️ **Limite connue, non corrigée par #160** : `Engine.DoneMotionCard` **ne vérifie pas** que
  > `WINNER_TEAM` vaut `MEMOTION_CURRENT_TEAM` (ni même qu'il appartient à
  > `MEMOTION_PARTICIPATING_TEAMS`) — il crédite toute équipe existant dans `data.Teams`. La règle
  > « équipe courante ou personne » est donc, aujourd'hui comme avant ce lot, **une contrainte
  > d'interface** appliquée identiquement par `/admin` (`GamePage.jsx`) et par `/anim` (#160), pas
  > une garde moteur. Ajouter cette garde côté moteur serait un changement de comportement à part
  > entière (risque de régression sur le mode SOLO, où `MEMOTION_CURRENT_TEAM` peut être vide) —
  > délibérément hors périmètre de #160.
  >
  > **`MEMOTION_SET_TEAMS` n'est PAS ajouté** : le choix des équipes participantes reste à la régie
  > (périmètre explicite de #160, strictement parallèle à `MEMORY_SET_TEAMS` en #159). Conséquence
  > assumée et identique : un animateur seul ne peut pas composer la table des équipes d'une manche
  > MEMOTION. Le moteur l'interdirait de toute façon hors `PREPARE`/`READY`
  > (`NOT_IN_PREPARE_OR_READY_PHASE`).
- **REGIE_MESSAGE_SEND / REGIE_MESSAGE_CLEAR (v6.4.x, #167)** — messagerie ponctuelle
  régie → animateurs, seules modifications de cette table par ce lot.
  - `REGIE_MESSAGE_SEND` est **`admin` uniquement**, jamais `anim` : le canal est **unidirectionnel
    par conception** (issue #167 — « pas de réponse possible »). L'animateur ne dispose d'aucun
    chemin pour émettre du texte vers la régie, et il ne doit pas en acquérir un par effet de bord.
  - `REGIE_MESSAGE_CLEAR` est la **première action partagée `admin` + `anim` qui n'appartient pas à
    la « conduite en direct »**. Une seule action pour deux intentions, parce que l'**effet serveur
    est rigoureusement identique** (effacer l'unique message actif et diffuser l'effacement) :
    l'animateur l'envoie depuis sa tablette, la régie pour **retirer** un message envoyé
    par erreur. La distinction n'est pas dans le protocole mais dans le `ClientType` de
    l'émetteur — c'est le serveur qui en déduit `CLEARED_BY` (voir §"Messagerie régie" ci-dessous),
    ce qui évite une seconde action au périmètre identique et une entrée d'allow-list de plus.
  > ⚠ **Élargissement de capacité pour `anim`, mais d'une classe nouvelle** : jusqu'ici, tout ce que
  > l'animateur pouvait émettre agissait sur l'**état de jeu** (phase, cartes, points) et restait
  > borné par les gardes du moteur. `REGIE_MESSAGE_CLEAR` agit sur un état **hors moteur**, et son
  > effet est **global** — un seul animateur efface le message pour **toutes** les tablettes ET pour
  > la régie. C'est délibéré (§"Messagerie régie", décision D3), pas un effet de bord : le message
  > est unique, donc son acquittement l'est aussi. Aucune garde moteur ne s'y applique et aucune
  > n'est ajoutée : l'action est valide dans **toutes** les phases, y compris `NEW_GAME`.
  >
  > **Conséquence assumée** : avec plusieurs tablettes, le premier animateur qui appuie sur « Vu »
  > fait disparaître le message chez les autres, éventuellement avant qu'ils ne l'aient lu. C'est le
  > modèle retenu (un message, un acquittement) ; un acquittement par tablette a été explicitement
  > écarté au GATE 1.5.
- **SET_CLIENT_TYPE** a une règle à part (dépend du type COURANT du client, pas
  d'une liste fixe) — voir sa propre section ci-dessous et
  `internal/server/inbound_allowlist.go`'s `IsSetClientTypeAllowed` : seul un
  client dont le type courant est `admin` (état par défaut de `/ws`, avant
  auto-déclaration) peut l'envoyer. Une fois auto-déclaré `tv`/`vplayer`/`anim`, le
  handshake ne peut plus être répété — ferme l'auto-promotion en admin qui
  existait avant #154 (`handleSetClientType` mappait toute valeur `TYPE`
  inconnue vers `admin` par défaut). Un client connecté sur `/ws/anim` (type fixé à la
  connexion, comme `/ws/tv`/`/ws/player`) n'a de toute façon aucune raison légitime de
  l'envoyer — même situation que TV/VPlayer, aucun changement de mécanisme requis pour #155.
- Toute action **absente de cette table**, ou envoyée par un type absent de sa
  ligne, est rejetée par défaut (deny-by-default, pas une liste d'exceptions).

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

## Animateur (Interface conduite, v6.2.0 — #155/#156)

> Endpoint dédié `/ws/anim` (`ClientType` `anim`, `contracts/websocket-endpoints.md`). Reçoit
> l'état de jeu via `UPDATE` (payload `SerializeForWebClient`, comme TV/VPlayer —
> `contracts/ws-payload-serialization.md` §"Animateur") et peut émettre les actions de conduite
> listées dans l'allow-list ci-dessus (`START`, `STOP`, `PAUSE`, `CONTINUE`, `REVEAL`, `READY`,
> `BUMPER_POINTS`, `TEAM_POINTS`). `NEXT_QUESTION` et `CREDIT_POINTS` ci-dessous lui sont
> exclusives.

### NEXT_QUESTION

Indique à l'interface animateur la prochaine question jouable, pour permettre l'enchaînement
(bouton « à suivre ») sans consulter `/admin`.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Cible | `ClientTypeAnim` **exclusivement** — `BroadcastToTypes(msg, ClientTypeAnim)`. Aucun autre type de client ne reçoit cette action |
| Trigger | HELLO d'un client animateur (état initial) ; `READY` (sélection/enchaînement de question) ; `STOP` ; `REVEAL` ; `NEW_GAME` ; `REORDER_QUESTIONS` ; `DELETE` (suppression d'une question) |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| ID | string | ID de la question suivante |
| QUESTION | string | Texte de la question |
| CATEGORY | string | Catégorie |
| TYPE | string | Type de question (SPEEDY, QCM, MEMORY, MEMOTION, ARDOISE) |
| POINTS | int | Points de base |
| TIME | int | Durée en secondes |
| CURRENT_POSITION | int | **v6.2.x (#166)** — rang de la question **courante** dans la liste triée (1-based). `0` si aucune question courante, ou si la question courante ne figure plus dans la liste (supprimée du disque mais toujours courante côté moteur — même cas de repli que la règle §3 ci-dessous) |
| TOTAL_QUESTIONS | int | **v6.2.x (#166)** — nombre total de questions du quiz (longueur de la liste triée), indépendamment de leur statut |

Les champs de la **question suivante** (`ID`, `QUESTION`, `CATEGORY`, `TYPE`, `POINTS`, `TIME`)
sont tous absents/zéro-valeur si plus aucune question n'est jouable — fin de partie ou toutes les
questions restantes déjà `STOPPED`/`REVEALED`/`PLAYED`.

> ⚠️ **v6.2.x (#166) — le payload n'est plus « tout ou rien ».** `CURRENT_POSITION` et
> `TOTAL_QUESTIONS` décrivent la question **courante** et le quiz, pas la suivante : ils sont
> renseignés **même quand il n'y a plus de question suivante**. Sans cette dissociation, la
> progression affichée par `/anim` (ligne méta, #166) disparaîtrait exactement sur la **dernière**
> question du quiz — le moment où « 12/12 » est le plus utile. Conséquence côté client : un
> consommateur ne peut plus déduire « pas de question suivante » de « payload vide », il doit
> tester l'absence d'`ID` (ce que `useWebSocket.js` fait déjà : `setNextQuestion(MSG?.ID ? MSG : null)`).
>
> Conséquence côté serveur : la règle §1 ci-dessous (« aucune question courante → pas de suivante »)
> ne peut plus court-circuiter `loadQuestions()`, puisque `TOTAL_QUESTIONS` doit être connu même
> sans question courante. Le surcoût est une lecture disque sur les seuls déclencheurs `NEW_GAME`
> et HELLO — jamais sur le chemin de `broadcastUpdate`, l'interdit posé plus bas restant entier.

#### Règle de calcul — parité stricte avec `GamePage.jsx` (`nextUnplayedQuestion`)

> **Corrigé 2026-08-13 (implémentation B5)** — la formulation initiale de cette section
> (« première question dont STATUS ∉ {...} », sans plus de précision) était une approximation qui
> omettait deux comportements réels du JS. Remplacée ci-dessous par une lecture ligne à ligne de
> `GamePage.jsx`'s `nextUnplayedQuestion` (le calcul du bouton « à suivre » de la régie).

1. ~~**Aucune question courante → pas de « suivante » du tout.**~~ **Révisé v6.2.x (#166,
   arbitrage GATE 2 D2) — divergence assumée avec `/admin`.** Si `GameState.QUESTION` est absent
   (aucune question chargée — ENROLL, NEW_GAME juste après reset), la recherche démarre à
   l'**indice 0** et renvoie la **première question jouable du quiz**, au lieu du payload vide.
   L'animateur dispose ainsi d'un point d'entrée pour démarrer la partie depuis la tablette, sans
   passer par la régie.

   > **Ceci rompt délibérément la parité stricte avec `GamePage.jsx`** exigée plus bas : la régie,
   > elle, continue de ne rien afficher dans cet état (`nextUnplayedQuestion` renvoie `null` sur
   > `if (!currentId)`, `GamePage.jsx:239`). La parité reste la règle pour **tous les autres cas**
   > (tri, position de départ de la recherche, statuts exclus, absence de bouclage) — c'est le seul
   > écart, il est nommé ici plutôt que découvert plus tard comme une régression.
   >
   > Implémentation : il s'agit de **retirer la sortie anticipée** `if state.Question == nil { return nil }`
   > de `getNextQuestionPayload`. Le chemin résultant (`currentIndex` restant à `-1`, boucle
   > démarrant à `0`) est celui que le code emprunte déjà quand la question courante ne figure plus
   > dans la liste — règle §3 ci-dessous. Aucun mécanisme nouveau.
2. Charger les questions (mécanisme de `loadQuestions()`, `cmd/server/main.go`), trier par
   `ORDER` croissant, repli sur `ID` (parsé en entier) en cas d'`ORDER` absent — même règle que
   `web/src/utils/questionOrder.js` (`sortQuestionsByOrder`).
3. Localiser la position de la question **courante** dans cette liste triée. Si elle n'y figure
   plus (supprimée entre-temps mais toujours question courante côté moteur), la recherche
   ci-dessous démarre au début de la liste — c'est le comportement de repli naturel de
   `Array.prototype.findIndex` renvoyant `-1` en JS (`i = -1 + 1 = 0`), pas un cas à traiter à part.
4. Retenir la première question **strictement après cette position** dont `STATUS` n'est ni
   `STOPPED`, ni `REVEALED`, ni `PLAYED` — **jamais** une question qui précède la position
   courante, même si elle est elle-même rejouable. « Suivante » veut dire suivante, pas
   « n'importe laquelle de disponible dans la liste ».
5. Aucun résultat après cette position → payload vide (règle §"Payload" ci-dessus) — la recherche
   ne boucle jamais vers le début.

⚠️ **Cette règle doit être réimplémentée en Go à l'identique de sa version JS actuelle**
(`GamePage.jsx`, bouton « à suivre » de la régie) — **pas** réinventée ni simplifiée. Une
divergence, même minime (tri, égalité d'`ORDER`, jeu de statuts exclus, recherche depuis le début
plutôt que depuis la position courante), afficherait une question différente à l'animateur et à la
régie pendant la même partie. Les cas de test du backend
(`_work/reports/plan-20260813-092950.md` §3 tâche B5, critères d'acceptation) sont **dérivés du
comportement JS observé**, pas conçus indépendamment — ordre personnalisé (`ORDER` non contigu),
question courante absente de la liste triée, statut exclu juste après la position courante, et
question finale (payload vide, position courante = dernière de la liste) au minimum.

**Note additionnelle (implémentation)** : le JS a également une garde de phase en tête de fonction
(`if (!['STOPPED','REVEALED','PREPARE','READY','STARTED','PAUSED','COUNTDOWN','ENROLL',
'NEW_GAME'].includes(phase)) return null`) — cette liste couvre en réalité **les 9 valeurs
possibles** de `GamePhase` (`internal/game/models.go`), donc la condition n'est jamais vraie en
pratique. Non porté côté Go (ajouter une branche systématiquement morte n'aurait aucun effet et
masquerait une vraie régression si un futur `GamePhase` venait à exister sans être ajouté ici) —
documenté pour qu'une future lecture de ce contrat ne pense pas à un oubli.

#### Pourquoi une action dédiée plutôt qu'un champ de `UPDATE`

L'`Engine` ne connaît pas la liste des questions : elles vivent en fichiers sur disque, chargés
par `loadQuestions()` — un `os.ReadDir` puis un `os.ReadFile` **par question**, à chaque appel ;
les statuts viennent de `engine.GetQuestionStatus(qID)`, pas de l'état interne de l'engine.
`UPDATE` (`GetGameJSON()`, produit par l'engine seul) est diffusé à **chaque tick de timer** —
injecter la question suivante dans ce payload déclencherait une lecture disque complète par tick.

`NEXT_QUESTION` est donc **strictement interdite** sur le chemin de `broadcastUpdate`/
`broadcastUpdateTo` : calculée et diffusée uniquement depuis les déclencheurs explicites listés
ci-dessus, jamais depuis la boucle de timer.

### SET_CREDIT_POINTS

Ajuste le montant de base que l'animateur créditera à la fin de la question courante — l'équivalent
serveur du champ `pointsInput` de la régie (`GamePage.jsx:770-771`), qui est aujourd'hui un état
React **local à `/admin`**, initialisé à `question.POINTS` à la sélection mais librement modifiable
ensuite (ex. manche bonus), et dont le serveur n'avait connaissance d'aucune façon avant cette
action (`START` transmet bien `POINTS` dans son payload WS mais `protocol.StartPayload` ne l'a
jamais décodé — champ silencieusement ignoré).

**Origine** : code review MAJEUR-1 sur #155/#156
(`_work/reports/code-reviewer-20260813-120457.md`) — sans ce mécanisme, `/anim` créditait
`question.POINTS` brut (`AnimPage.jsx`, avant ce lot) alors que `/admin` créditait `pointsInput`
potentiellement ajusté : deux montants différents pour la même question, dans le même état de jeu,
selon l'interface utilisée pour créditer — silencieux, sans indication du mismatch dans aucune des
deux interfaces.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Émetteur | `admin` uniquement (voir §"Sécurité — Allow-list entrante" ci-dessus) |
| Trigger | Modification du champ `pointsInput` sur `/admin` (`onChange`, **avec un debounce côté
  frontend** — ce champ change à chaque frappe, il ne faut pas un message WS par frappe) |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| POINTS | int | Nouveau montant de base |

#### CREDIT_POINTS

Rediffuse le montant de base courant à l'interface animateur — la contrepartie serveur→client de
`SET_CREDIT_POINTS`.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Cible | `ClientTypeAnim` **exclusivement** — `BroadcastToTypes(msg, ClientTypeAnim)` |
| Trigger | `SET_CREDIT_POINTS` reçu (admin ajuste) ; `READY` (nouvelle question sélectionnée — réinitialise à `question.POINTS`, même règle que `pointsInput` sur `handleQuestionSelect`) ; `NEW_GAME` (aucune question courante — remise à zéro) ; HELLO d'un client animateur (état initial, envoi ciblé comme `NEXT_QUESTION`) |

##### Payload

| Champ | Type | Description |
|-------|------|-------------|
| POINTS | int | Montant de base courant à utiliser pour créditer la question en cours |

#### Valeur par défaut et réinitialisation

Le serveur garde en mémoire (non persisté — état de session, comme `pointsInput` côté React) un
montant courant, initialisé à `question.POINTS` (repli sur `1` si absent/invalide — même règle que
`GamePage.jsx:299` `parseInt(question.POINTS) || 1`) à chaque `READY`, et écrasé par la valeur reçue
sur chaque `SET_CREDIT_POINTS`. Aucune lecture disque ni recalcul sur le chemin de `broadcastUpdate`
n'est nécessaire ici (contrairement à `NEXT_QUESTION`) — c'est une simple valeur en mémoire, pas une
requête sur `loadQuestions()`.

#### Ce que ce mécanisme NE couvre PAS (hors périmètre de ce correctif)

- **QCM / MEMORY** : `resolvePointsAward` (`web/src/utils/pointsAward.js`) applique des règles
  spécifiques à ces types (pénalité par indices, score MEMORY par paires) par-dessus le montant de
  base — `CREDIT_POINTS` ne transmet que le montant de **base**, pas le montant final calculé. Sans
  conséquence aujourd'hui : QCM/MEMORY sont hors périmètre de la tranche SPEEDY #155/#156 sur
  `/anim`. À réévaluer si ces types y sont ouverts.
- **`TIME`** (durée du chrono) : la revue de code a noté la même famille de problème sur
  `AnimPage.jsx`'s `handleStart`, qui lit `question.TIME` brut plutôt qu'un éventuel `timeInput`
  ajusté côté admin avant qu'un animateur ne lance la question. Non traité par ce correctif
  (arbitrage explicite : MAJEUR-1 porte sur le crédit de points, pas sur le lancement) — impact
  jugé moindre par la revue (visible et corrigeable en cours de manche via PAUSE, contrairement à un
  score mal crédité qui est difficile à corriger après coup).

---

### AWARDED_TEAMS

Informe les interfaces animateur des équipes **déjà créditées pour la question en cours**, avec le
montant. Sert deux besoins d'un coup (arbitrage GATE 2 A2 de #158, élargi à tous les modes) : confirmer à **tous** les
animateurs qu'un crédit vient d'avoir lieu, et leur permettre de **bloquer** un second crédit sur
la même équipe — que le premier vienne d'une autre tablette ou de la régie.

> **Périmètre : tous les modes de jeu conduits depuis `/anim`**, pas seulement ARDOISE — le crédit
> SPEEDY et QCM déjà livré (#156/#157, bouton de la carte d'équipe) est concerné au même titre.
> Cette action relève d'un chantier transversal faisant l'objet d'une **issue dédiée, #170** ;
> #158 (ARDOISE) en est un consommateur, pas le propriétaire.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Server→Client` |
| Cible | `ClientTypeAnim` **exclusivement** — `BroadcastToTypes(msg, ClientTypeAnim)`, comme `NEXT_QUESTION` et `CREDIT_POINTS`. La régie n'est pas destinataire : elle reste sans restriction (voir §"La régie n'est pas contrainte") |
| Trigger | `TEAM_POINTS` et `BUMPER_POINTS` traités (quelle qu'en soit l'origine, `/admin` ou `/anim`) ; `READY` (changement de question) ; `NEW_GAME` ; `RAZ` ; HELLO d'un client animateur (état initial) |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| QUESTION_ID | string | ID de la question courante — `""` si aucune. Permet au client de rejeter un payload obsolète arrivé après un changement de question |
| TEAMS | array | Une entrée par équipe créditée, dans l'ordre chronologique du premier crédit |
| TEAMS[].TEAM | string | Nom de l'équipe |
| TEAMS[].POINTS | int | **Somme** des points attribués à cette équipe pour la question courante (une équipe peut être créditée deux fois depuis la régie — voir ci-dessous) |
| TEAMS[].TIMESTAMP | int64 | Horodatage µs du **premier** crédit de cette équipe pour la question courante |

Tableau **vide** (`[]`, jamais `null`) quand aucune équipe n'a encore été créditée — discipline
projet, un client qui itère ne doit jamais tomber sur `null`.

#### ⚠️ `POINTS` peut valoir `0` — un montant nul est un verrou valide

Un crédit à **montant nul est un refus explicite** (#170, geste « 0 pt » — refuser une réponse
emprunte le chemin de crédit normal avec `POINTS: 0`), pas « pas encore traité ». Il produit une
entrée `TEAMS[]` comme n'importe quel autre crédit : même enregistrement inconditionnel dans
l'historique, même projection, même verrouillage.

**Un client ne doit jamais tester la présence d'un verrou par la véracité du montant** —
`if (awardedTeams[team]?.POINTS)` est **faux** pour un refus (`0` est falsy en JS), ce qui
déverrouillerait silencieusement la ligne et réexposerait exactement les réponses refusées au
double-crédit. Le test correct porte sur la **présence de l'entrée** dans `TEAMS[]` pour cette
équipe, jamais sur la valeur de `POINTS` :

```js
// Correct
const locked = team in awardedTeamsByName   // ou : awardedTeamsByName[team] !== undefined
// Faux — casse silencieusement sur un refus (POINTS: 0)
const locked = !!awardedTeamsByName[team]?.POINTS
```

Voir `web/src/components/AnimCreditControl.jsx` (composant de crédit unique, seul point
d'application de cette règle côté frontend) et `contracts/CHANGELOG.md` `[20260816-2]`.

#### Source de vérité — l'historique existant, pas un nouvel état

**Aucun nouveau champ d'état n'est créé.** Le payload est une **projection** de l'historique
d'événements déjà tenu par le moteur (`Engine.history`, `AddGameEvent`, persisté par `SaveHistory`,
exposé par `GET /history`, consommé par le PALMARÈS). `handleTeamPoints` **comme**
`handleBumperPoints` y enregistrent déjà un `GameEvent{EventType: "POINTS_AWARDED", QuestionID,
TeamName, Points, Timestamp}` — tout ce dont ce mécanisme a besoin y est, quelle que soit
l'interface d'origine du crédit.

Règle de projection :

```
événements tels que  EventType == "POINTS_AWARDED"
                 ET  QuestionID == GameState.Question.ID
                 ET  Timestamp  >= GameState.GameTime
regroupés par TeamName ; POINTS = somme, TIMESTAMP = min
```

- Le filtre sur `TeamName` (et non `WinnerID`) est délibéré : `WinnerID` porte une MAC quand le
  crédit vise un joueur, alors que `TeamName` est **toujours** renseigné (`models.go:457`). Une
  équipe créditée via l'un de ses joueurs compte donc comme créditée.
- **Le filtre `Timestamp >= GameState.GameTime` est indispensable** : l'historique n'est pas remis à
  zéro entre deux questions, seulement à `NEW_GAME`/`RAZ`. `GameTime` est l'horodatage du départ de
  la question courante (`Engine.Start`/`StartImmediate`) : sans ce filtre, **rejouer une question
  déjà créditée la laisserait bloquée pour toujours**.
- Projection recalculée **uniquement sur les déclencheurs listés**, jamais sur le chemin de
  `broadcastUpdate` — même discipline que `NEXT_QUESTION`, pour la même raison : un parcours de
  l'historique à chaque tick de chronomètre est un coût qui grandit avec la partie.

#### La régie n'est pas contrainte

`/admin` n'est pas destinataire de cette action et **ne reçoit aucune garde** : elle peut créditer,
recréditer, surcharger. C'est l'état de fait actuel — il n'existe aujourd'hui **aucune garde de
double-crédit**, ni côté interface (`GamePage.jsx`, bouton ARDOISE inconditionnel) ni côté moteur
(`UpdateTeamScore` n'inspecte pas l'historique) — et #158 ne le change pas. Le blocage est une règle
**de l'interface animateur uniquement**, appliquée côté client à partir de ce payload.

Conséquence assumée : un second crédit venu de la régie s'ajoute au premier ; c'est pourquoi
`POINTS` transporte la **somme** et non le dernier montant, afin que l'animateur voie ce que
l'équipe a réellement reçu pour cette question.

---

## Messagerie régie (v6.4.x — #167)

Canal de communication **hors moteur de jeu** : la régie (`/admin`) pousse une consigne courte vers
toutes les tablettes animateur (`/anim`). Trois actions, un seul état.

### Modèle — un seul message actif, un seul acquittement

Le serveur détient **un unique emplacement** (`RegieMessage`), en **mémoire vive uniquement** :

```
inactif ──REGIE_MESSAGE_SEND (admin)──▶ actif {TEXT, SENT_AT}
   ▲                                        │
   │                                        │ REGIE_MESSAGE_SEND (admin)
   │                                        │ ─▶ remplace le contenu, SENT_AT réarmé
   │                                        │    (reste actif, pas de file d'attente)
   │                                        ▼
   └──────REGIE_MESSAGE_CLEAR (admin OU anim)──────┘
```

**Décisions structurantes (GATE 1.5, #167)** :

- **D1 — un seul emplacement, jamais de file.** Un `REGIE_MESSAGE_SEND` porteur d'un texte
  **différent** remplace le contenu actif et réarme `SENT_AT`. Aucun message n'est mis en attente,
  aucun historique n'est conservé. C'est la traduction directe de « message ponctuel ». Un `SEND`
  porteur du texte **déjà actif** est un no-op (règle 4 de validation ci-dessous) — nécessaire parce
  que l'envoi régie est automatique et se déclenche donc plusieurs fois pour une même saisie.
- **D2 — état en mémoire vive, jamais sur disque.** Même nature et même discipline que
  `a.currentCreditPoints` (`cmd/server/main.go`) : écrit et lu **exclusivement depuis la goroutine
  de dispatch unique** de `handleWebMessage` (#131) — **aucun mutex requis**. Corollaire assumé :
  un redémarrage serveur efface le message actif. C'est conforme à « pas d'historique persistant ».
- **D3 — l'acquittement est global, pas par tablette.** N'importe quel animateur connecté efface le
  message **pour tout le monde** (toutes les tablettes *et* la régie). Il n'existe **aucun comptage
  par client** (« 2/3 ont vu ») : un message unique, un acquittement unique. Écarté explicitement
  au GATE 1.5 au profit de ce modèle binaire.
- **D4 — la régie peut retirer son propre message** à tout moment (envoi par erreur), via la
  **même** action que l'acquittement animateur. Le serveur distingue les deux par le `ClientType`
  de l'émetteur et le restitue dans `CLEARED_BY`.
- **D5 — aucun couplage avec la machine à états du jeu.** Le message n'est **jamais** effacé
  automatiquement par une transition (`NEW_GAME`, `READY`, `RAZ`, changement de question…). C'est
  un canal de coordination humaine, orthogonal au déroulé de la partie ; le moteur (`engine.go`)
  n'est pas touché par #167 et aucune garde de phase ne s'applique. Corollaire assumé : une consigne
  non acquittée survit à un changement de question.
- **D6 — livraison différée plutôt que perdue.** Si aucune tablette n'est connectée au moment de
  l'envoi, le message reste actif et est délivré à la **prochaine** connexion `/anim` (rejeu au
  `HELLO`). C'est la raison d'être de l'état serveur : une tablette en veille ou en reconnexion
  Wi-Fi ne rate pas la consigne.

---

### REGIE_MESSAGE_SEND

**Direction** : Client → Server
**Émetteur autorisé** : `admin` **uniquement** (allow-list #154 — voir §"Sécurité" en tête)
**Effet** : arme ou remplace l'unique message actif, puis diffuse `REGIE_MESSAGE`.

#### Payload

| Champ | Type | Obligatoire | Description |
|---|---|---|---|
| `TEXT` | string | oui | Texte libre de la consigne. **140 caractères maximum** |

#### Exemple

```json
{ "ACTION": "REGIE_MESSAGE_SEND", "MSG": { "TEXT": "Question 12 annulée — enchaîne sur la 13" } }
```

#### Validation serveur (normative)

Le serveur est la **seule** autorité sur ces trois règles ; la limite `maxLength` du champ de saisie
régie est une commodité d'interface, jamais une garantie.

1. **Espaces de bordure retirés** (`strings.TrimSpace`) avant toute autre vérification.
2. **Texte vide après trim → l'action est ignorée** (log `WARN`, aucun changement d'état, aucune
   diffusion). Un message vide n'efface pas le message courant : l'effacement passe **uniquement**
   par `REGIE_MESSAGE_CLEAR`.
3. **Troncature à 140 caractères**, comptés en **runes et non en octets** — les consignes sont en
   français et « à », « é », « ç » occupent 2 octets chacun. Une troncature en octets couperait au
   milieu d'un caractère et produirait de l'UTF-8 invalide.
4. **Texte identique au message déjà actif → no-op idempotent** : aucun réarmement de `SENT_AT`,
   aucun effacement de `CLEARED_BY`, **aucune diffusion**. Comparaison sur le texte *après* trim et
   troncature.

> ⚠️ **La règle 4 n'est pas une optimisation, c'est une garde de correction.** L'interface régie
> déclenche l'envoi automatiquement (touche Entrée, perte de focus, pause de frappe), donc le même
> texte part légitimement plusieurs fois : on tape, la pause de frappe envoie, puis on clique
> ailleurs et le blur renvoie l'identique. Sans cette règle, ce second envoi réarmerait le message
> et remettrait `CLEARED_BY` à `""` — **un message déjà acquitté réapparaîtrait sur les tablettes**,
> déclenché par un simple changement de focus en régie. La garde est posée **côté serveur** et non
> côté client parce qu'elle doit tenir quelle que soit l'interface qui émet.

**Sécurité** : `TEXT` est transporté et rendu comme **texte**, jamais comme HTML. React échappe le
contenu textuel par construction ; aucun `dangerouslySetInnerHTML` ne doit être introduit sur ce
chemin, côté `/anim` comme côté `/admin`.

---

### REGIE_MESSAGE_CLEAR

**Direction** : Client → Server
**Émetteurs autorisés** : `admin` (retirer son message) **et** `anim` (acquitter)
**Effet** : efface l'unique message actif, puis diffuse `REGIE_MESSAGE` à l'état inactif.

#### Payload

Aucun champ — `{}`. Il n'y a **rien à désigner** : un seul message peut être actif à la fois.

#### Exemple

```json
{ "ACTION": "REGIE_MESSAGE_CLEAR", "MSG": {} }
```

#### Sémantique

- Reçue alors qu'**aucun** message n'est actif → **no-op idempotent** : aucun changement d'état,
  aucune diffusion. Deux tablettes qui acquittent au même instant ne produisent donc qu'une seule
  diffusion d'effacement, et jamais d'incohérence.
- `CLEARED_BY` de la diffusion qui suit vaut `"ANIM"` ou `"REGIE"` selon le `ClientType` de
  l'émetteur — déduit par le serveur, **jamais** transmis par le client (un client ne se déclare
  pas lui-même).

---

### REGIE_MESSAGE

**Direction** : Server → Client
**Cibles** : `admin` **et** `anim` — voir `contracts/websocket-endpoints.md` §"Filtres de diffusion
par type". C'est la **première** action sortante partagée par ces deux types seuls : `/anim`
l'affiche pour la lire, `/admin` pour savoir si sa consigne est encore en attente ou déjà acquittée.
`tv`, `vplayer` et `buzzer` ne la reçoivent **jamais**.

**Déclencheurs** : `REGIE_MESSAGE_SEND` · `REGIE_MESSAGE_CLEAR` · connexion d'un client `admin` ou
`anim` (`HELLO` → envoi **ciblé** à ce seul client, jamais un re-broadcast à tous).

#### Payload

| Champ | Type | Description |
|---|---|---|
| `ACTIVE` | bool | `true` = un message est en attente d'acquittement |
| `TEXT` | string | Contenu ; `""` quand `ACTIVE` vaut `false` |
| `SENT_AT` | number | Horodatage d'émission, millisecondes Unix ; `0` quand `ACTIVE` vaut `false` |
| `CLEARED_BY` | string | `"ANIM"`, `"REGIE"`, ou `""` — origine du dernier effacement |

> ⚠️ **Aucun champ ne porte `omitempty`.** Même règle que `GameState` (`CLAUDE.md` §"Implementation
> Rules") et pour la même raison : un `ACTIVE: false` omis du JSON laisserait le frontend afficher
> indéfiniment un message déjà effacé. L'effacement **est** l'information à transmettre — il doit
> voyager explicitement.

#### Exemples

Message actif :
```json
{
  "ACTION": "REGIE_MESSAGE",
  "MSG": { "ACTIVE": true, "TEXT": "Question 12 annulée — enchaîne sur la 13", "SENT_AT": 1755511234567, "CLEARED_BY": "" }
}
```

Après acquittement par un animateur :
```json
{
  "ACTION": "REGIE_MESSAGE",
  "MSG": { "ACTIVE": false, "TEXT": "", "SENT_AT": 0, "CLEARED_BY": "ANIM" }
}
```

`CLEARED_BY` survit à l'effacement précisément pour que la régie puisse afficher « Vu par
l'animateur » plutôt qu'un simple retour à l'état vide — c'est le seul retour d'acquittement du
modèle, et il remplace tout comptage par tablette.

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
| Émetteurs | `tv`, `vplayer`, `anim` — **inchangé en v7.1.0** |
| Phase     | STARTED |
| Type      | MEMORY — question hôte, **ou carte MEMOTION `TYPE=MEMORY`** (v7.1.0, #187) |

#### Payload

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| CARD_ID | string | ✅ | ID de la carte Memory (ex: `"1-1"`, `"2-2"` — format `pairID-cardNum`) |
| MOTION_CARD_ID | string | ⬜ *(v7.1.0)* | Identité de la carte MEMOTION parente (`CardScope`, `question-types.md` §9). Absent hors manche MEMOTION ⇒ comportement actuel |

#### Vérifications serveur (v7.1.0, #187) — deux règles, deux comportements d'échec

> ⚠️ **Ces deux règles sont distinctes et ne doivent pas être uniformisées.** Voir
> `contracts/question-types.md` §9.2 pour la dérogation formelle.

**1. Portée de carte** — `ValidateCardScope(MOTION_CARD_ID)` : **refus explicite**
(`CARD_SCOPE_MISMATCH` / `CARD_SCOPE_UNEXPECTED`), renvoyé comme les `MotionError` existants.

**2. Tour de l'équipe** — **ignore silencieux**.

Le serveur devient **seule autorité** sur le droit de retourner une carte :

- Il **dérive l'identité de l'émetteur depuis sa connexion** — patron `ARDOISE_INPUT` (résolution
  du bumper en 3 passes `payload.ID → msg.ID → clientID`, puis `bumper.Team`). **Jamais** un nom
  d'équipe reçu dans le payload.
- Si l'équipe de l'émetteur n'est pas l'équipe active, le geste est **ignoré** : aucune mutation
  d'état, **aucun broadcast**, aucun message renvoyé au client.

> 🔴 **La vérification ne s'applique QU'AUX clients `vplayer`.**
> `tv` et `anim` retournent légitimement des cartes **pour la table** — `anim` est l'animateur sur
> sa tablette, `tv` couvre l'aperçu régie en iframe (`/tv?admin=true`) et le clic d'un spectateur
> sur l'écran public. **Ni l'un ni l'autre n'a d'équipe.** Appliquer la vérification à tous
> **casserait la conduite animateur et l'aperçu régie**. Règle exacte : *si `clientType == vplayer`,
> vérifier l'équipe ; sinon, laisser passer.*

> 🔴 **Un flip ignoré ne doit déclencher aucun broadcast.**
> `handleFlipMemoryCard` diffuse aujourd'hui un `UPDATE` complet **inconditionnellement**. Or la
> restriction d'affichage côté client est **retirée** en v7.1.0 (voir ci-dessous) : n'importe quel
> joueur peut désormais taper n'importe quelle carte. Si chaque tap hors tour diffusait un
> `GameState` complet (~11 Ko en MEMOTION) à tous les clients, on recréerait la classe de défaut des
> **tempêtes de broadcast** (#127/#129, `tests/procedures/bugfix-vjoueur-broadcast-storm.md`).

**Journalisation** : l'ignore est silencieux **pour le client**, pas dans les journaux — tracer en
`LogDebug`/`LogInfo`, comme `SetArdoiseAnswer` le fait quand il ignore une saisie. Sans trace, le
comportement devient indébogable en QUALIF.

**Contrainte d'implémentation** : `handleFlipMemoryCard(msg)` ne reçoit aujourd'hui **aucune
identité**. Sa signature doit être étendue pour recevoir `clientID` **et** `clientType`, tous deux
déjà disponibles dans `handleWebMessage`.

#### Restriction client retirée (v7.1.0, #187)

La garde d'affichage qui masquait le geste aux joueurs hors tour (`PlayerDisplay.jsx`,
`isVPlayerInActiveTeam`) est **supprimée** : tout joueur peut **tenter** de retourner une carte.

**Aucun risque d'information** : la grille MEMORY est **déjà publique** — aucun champ `MEMORY_*`
n'est filtré, et la TV l'affiche à tous.

**Bénéfice de fiabilité au passage** : la garde actuelle exigeait `MEMORY_CURRENT_TEAM` non vide ;
si aucune équipe n'était sélectionnée, **aucun VJoueur ne pouvait cliquer**. Cette dépendance
disparaît.

> **Motif de la dérogation au refus explicite** : dans l'usage normal, un tap hors tour est un
> geste de curiosité ou de réflexe, pas une tentative malveillante. Y répondre par une erreur
> polluerait l'interface sans rien protéger. Décision utilisateur du 2026-08-24.

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

---

## ENTRACTE_SET ✨ (v6.5.2, #119)

Active ou désactive le **mode entracte** (pause globale).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Payload   | `{ "ACTIVE": true \| false }` |
| Allow-list | **`admin` uniquement** |
| Phases acceptées (activation) | `STOPPED`, `PREPARE`, `READY`, `NEW_GAME`, `REVEALED` |
| Phases refusées (activation) | `COUNTDOWN`, `STARTED`, `PAUSED`, `ENROLL` |
| Désactivation | Acceptée depuis **toute** phase |
| Effet | `GameState.ENTRACTE` ← `ACTIVE`, extinction/restauration des LEDs, `UPDATE` diffusé aux 5 types |

**`anim` est délibérément exclu.** L'interface animateur voit l'entracte (filtre + indicateur) mais
ne peut ni le déclencher ni le lever — « contrôle réservé à l'admin ». C'est une exception assumée au
bloc « conduite en direct » (#155/#156), où `anim` partage pourtant START/STOP/PAUSE/REVEAL : une
pause engage toute la salle, pas seulement la manche en cours.

**Commande explicite, pas un basculement.** Le payload porte l'état voulu, il n'inverse pas l'état
courant : deux clics rapides ou un renvoi réseau ne peuvent pas laisser l'entracte inversé. Le
libellé du bouton (`ENTRACTE` / `FIN D'ENTRACTE`) est dérivé côté client de `GameState.ENTRACTE` —
aucun état de bouton n'existe côté serveur.

Une activation refusée pour cause de phase est journalisée en `WARN` et **ne renvoie rien au client** :
même politique que le refus d'allow-list existant.

---

## UPDATE_ENTRACTE_CONFIG ✨ (v6.5.2, #119)

Enregistre la configuration du panneau d'entracte — **propriété de la partie**, éditée depuis la
page Quiz (`/quiz`), persistée dans `game_state.json` aux côtés des champs `QUIZ_*`.

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Payload   | `{ TITLE, SUBTITLE, PANEL_SIZE, ANIM_PERIOD, ANIM_INTENSITY, TRANSITION_MS }` |
| Allow-list | **`admin` uniquement** |
| Accepté pendant un entracte actif | ✅ **Oui** — mais sans effet sur la pause en cours (voir ci-dessous) |
| Effet | Bornage, écriture dans `GameState.ENTRACTE_CONFIG`, persistance synchrone, `UPDATE` diffusé |

**Action dédiée, et non une extension de `UPDATE_QUIZ_META`.** Les deux formulaires cohabitent sur
la même page mais ont chacun leur bouton d'enregistrement. `QuizMetaPayload` porte déjà une
mécanique de pointeurs destinée à éviter qu'un client n'envoyant qu'une partie du formulaire
n'efface le reste ; y greffer un second formulaire rendrait ce piège plus probable — enregistrer le
bloc Quiz effacerait les réglages d'entracte, et réciproquement. Deux formulaires, deux actions,
chacune propriétaire de ses champs.

`ANIM_INTENSITY` et `TRANSITION_MS` sont transmis en **pointeurs** : `0` y est une valeur
signifiante (animation désactivée / transition instantanée) qu'il faut pouvoir distinguer de
« champ absent ». Même convention que `POPULATIONS`/`DIFFICULTIES` dans `QuizMetaPayload`.

`IMAGE_IS_CUSTOM` **n'est pas dans le payload** : c'est un champ dérivé, calculé par le serveur
d'après la présence du fichier sur disque. L'image se gère par son endpoint dédié
(`/api/game/entracte-image`, voir `contracts/http-endpoints.md`).

**Effet pendant un entracte actif** — l'action est acceptée et la configuration est bien persistée,
mais elle **ne modifie pas le panneau diffusé** : `ENTRACTE_CONFIG` est gelé jusqu'à la fin de la
pause, et les nouvelles valeurs s'appliquent au **prochain** `ENTRACTE_SET{ACTIVE:true}`. Le
formulaire d'édition se relit dans `ENTRACTE_CONFIG_SAVED`, pas dans `ENTRACTE_CONFIG` — détail des
deux champs dans `contracts/game-state.md` §« Configuration gelée à l'activation ».

---

## MEMOTION Actions — v7.0.0 (#184, #185)

### MEMOTION_SELECT

Sélectionne une carte MEMOTION pour la jouer (transition `GRID` → `SELECTED`).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Émetteur | `admin` (`/admin` aperçu TV), `anim` (`/ws/anim` interface tablette) |
| Payload | `{ "CARD_ID": "card_1" }` |

### MEMOTION_DONE — v7.0.0 (#184, #185)

Valide une carte MEMOTION et attribue les points (transition `REVEAL` → `GRID` ou fin).

| Propriété | Valeur |
|-----------|--------|
| Direction | `Client→Server` |
| Émetteur | `admin`, `anim` |

#### Payload

| Champ | Type | Description |
|-------|------|-------------|
| CARD_ID | string | ID de la carte MEMOTION |
| WINNER_TEAM | string | ID de l'équipe gagnante (ou `""` si personne) |
| **UNITS** | int | **[v7.0.0]** Unités gagnées — `1` (défaut) pour type binaire, `&gt; 1` réservé aux types à progression (MEMORY, #187). **Optionnel** — défaut `1` si absent |

**Exemple avec UNITS :**
```json
{
  "CARD_ID": "mc-3",
  "WINNER_TEAM": "team_A",
  "UNITS": 1
}
```

**Impact du barème** : les points sont calculés selon `MotionCard.POINTS_RULE` (voir `contracts/game-state.md` §"Champs MEMOTION") :
- `MODE: "STARS"` (défaut) → barème par étoiles (inchangé v6.x)
- `MODE: "FIXED"` → `VALUE` points si `UNITS > 0`, sinon 0
- `MODE: "PER_UNIT"` → `VALUE × UNITS` (ouvre MEMORY au prorata en #187, sans toucher le cœur)

---

## Portée des actions par hôte — `MOTION_CARD_ID` (CardScope, v7.0.0 #184)

**Documentation** : voir `contracts/question-types.md` §9 pour la table complète et les invariants.

Avec l'arrivée des cartes MEMOTION polymorphes, certaines actions typées (ex. animations d'indices QCM) s'appliquent à un **hôte** : la question en cours OU la carte MEMOTION active. L'action porte un champ optionnel `MOTION_CARD_ID` qui désigne le contexte :

| Champ | Valeur | Signification |
|-------|--------|---------------|
| `MOTION_CARD_ID` absent | `""` (implicite) | Hôte **question** — l'action cible `GameState.QUESTION` |
| `MOTION_CARD_ID` = card ID | `"card_1"` | Hôte **carte MEMOTION** — l'action cible `GameState.MEMOTION_ACTIVE` (la carte active portant ce type) |

**Invariants de portée (déjà testés en v7.0.0)** :
1. Une action en hôte **question** ne peut **jamais** porter `MOTION_CARD_ID` (refusée en HTTP 400 si présente)
2. Une action en hôte **carte** doit porter `MOTION_CARD_ID = MEMOTION_SELECTED` (refusée en HTTP 400 si absent ou différent)
3. Une action lancée par un **joueur** (VPlayer) n'a **jamais** de `MOTION_CARD_ID` (liste blanche entrante — aucune action joueur en carte v7.0.0)
4. Une action lancée par l'**animateur** ou l'**admin** peut porter `MOTION_CARD_ID` selon le contexte

---

## Actions refusées pendant l'entracte (v6.5.2, #119)

Tant que `GameState.ENTRACTE` vaut `true`, un **second contrôle** s'applique après celui de
l'allow-list par type de client (`cmd/server/main.go`, juste après `IsActionAllowed`). C'est une
**liste d'autorisation fermée** : toute action absente est refusée, journalisée en `WARN`, et
n'atteint jamais son handler.

| Action autorisée pendant l'entracte | Pourquoi |
|---|---|
| `ENTRACTE_SET` | Sortir de l'entracte — sans quoi le mode serait sans issue |
| `HELLO`, `SET_CLIENT_TYPE` | Poignée de main : un écran rafraîchi pendant la pause doit se reconnecter |
| `PLAYER_CONNECT` | Un VJoueur qui recharge son téléphone pendant la pause doit retrouver sa place |
| `REGIE_MESSAGE_SEND`, `REGIE_MESSAGE_CLEAR` | La régie prévient les animateurs — c'est précisément le moment utile |
| `UPDATE_ENTRACTE_CONFIG` | Préparer le panneau de la **prochaine** pause pendant celle en cours. Sans effet sur le panneau diffusé (gel, voir ci-dessus) |
| `PONG` | Inoffensif (aucune transition n'en dépend hors `PREPARE`) et évite de faire croire un buzzer muet |

Tout le reste — `READY`, `START`, `STOP`, `PAUSE`, `CONTINUE`, `REVEAL`, `REMOTE`, `NEW_GAME`,
`RAZ`, `SHOW_QR_CODE`, `UPDATE_QUIZ_META`, crédits de points, actions MEMORY/MEMOTION/ARDOISE… —
est refusé.

> **`UPDATE_ENTRACTE_CONFIG` est la seule action de configuration admise**, et sans exception à la
> règle générale : elle ne change rien à la pause en cours. Le panneau diffusé reste celui figé au
> déclenchement — ce qui **retire un critère d'acceptation de la première livraison** (« changer les
> textes se voit en direct »).

> **Pourquoi une garde serveur et pas seulement l'estompage.** Un filtre CSS ne bloque aucun clic :
> un bouton estompé sur `/admin` ou `/anim` reste parfaitement cliquable, et un onglet resté ouvert
> peut de toute façon émettre n'importe quelle action. L'estompage est un **signal**, la garde est la
> **règle**. Le « aucun lancement de question, aucun changement d'affichage » de #119 n'est tenu que
> par cette garde.

> **Liste fermée, volontairement.** Une action ajoutée plus tard au protocole sans décision explicite
> sera refusée pendant l'entracte — bruyamment, dans les journaux — plutôt que silencieusement
> permise. Un test balaie l'ensemble des actions du protocole pour que cet arbitrage reste conscient.
