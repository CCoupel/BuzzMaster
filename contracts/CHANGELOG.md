# Changelog des Contrats API

---

## [20260725] — Badge de connexion 4 états (#109 Phase 2)

- **[NEW]** `MESSAGE_LOST`/`DELIVERY_CONFIRMED` câblés sur leurs sources réelles (voir
  `contracts/websocket-actions.md` §CONN_STATE pour le détail buzzer/VJoueur) + fenêtre minimale
  de 2s sur l'état `green` avant retour à `""` (D2/D3), sans borne max (D7).
- **[NEW]** Backend : `engine.ConfirmDelivery(bumperID)` — entrée gated pour `DELIVERY_CONFIRMED`
  qui respecte la fenêtre `green` ; `engine.ApplyVPlayerBroadcastConnEvents()` — évalue tous les
  VJoueurs participants à chaque broadcast GameState.
- **[NEW]** Backend : `WebSocketHub.GetClientPlayerID(clientID)` — getter symétrique de
  `SetClientPlayerID`, utilisé pour confirmer la livraison à réception de n'importe quel message
  d'un VJoueur identifié.

> Aucune breaking change — additif sur la Phase 1 (#109, v5.7.13). `engine.TransitionConn` (table
> pure) reste inchangé et non-gated ; seuls les nouveaux appels de production passent par
> `ConfirmDelivery`.

---

## [20260725] — Badge de connexion 4 états (#109 Phase 1)

- **[NEW]** `Bumper.CONN_STATE` (string `""`/`"orange"`/`"red"`/`"green"`, **sans omitempty**) —
  badge de connexion enrichi, propagé sur les 4 endpoints WebSocket (admin/tv/player/buzzer).
  Périmètre : uniquement les bumpers participants (`TEAM != ""`) ; `""` sinon.
- **[NEW]** Backend : `type ConnEvent string` + `engine.TransitionConn(bumperID string, event ConnEvent)`
  — table de transitions complète (voir `contracts/websocket-actions.md`). Câblé sur
  `DISCONNECT`/`RECONNECT` (Phase 1) ; `MESSAGE_LOST`/`DELIVERY_CONFIRMED` + fenêtre 2s sur
  `green` en Phase 2.
- **[FIX]** Bumper fantôme VJoueur (R1, root cause #109) : la reconnexion par nom
  (`handlePlayerConnect`) est désormais atomique côté moteur (`engine.ReconnectOrCreateVirtualPlayer`,
  matching trim + insensible à la casse) et consolide automatiquement les doublons existants —
  élimine la course qui pouvait laisser un second bumper bloqué en badge déconnecté.

> Aucune breaking change — `CONN_STATE` est additif, `CONNECTED` (bool) est conservé tel quel.

---

## [20260607] — Bugfix v5.7.2 (#100)

- **[BREAKING]** `POST /api/categories` — remplace le body JSON par **multipart/form-data** : champ `name` (string) + champ `file` (image PNG/JPG/JPEG/WebP, obligatoire)
- **[BREAKING]** `POST /api/categories` — `imageURL` dans la réponse est désormais toujours non-vide (ex: `"/files/categories/MA_CATEGORIE.png"`)
- **[CHANGED]** `GET /api/categories` — suppression du scan des fichiers `.json` ; seuls les fichiers image (`png/jpg/jpeg/webp`) dans `data/files/categories/` sont retournés comme catégories custom

> **Migration** : les clients qui appellent `POST /api/categories` avec un body JSON doivent migrer vers multipart/form-data avec le champ `file` (image obligatoire).

---

## [20260607] — Milestone v5.7.1 (#97, #98, #99)

- **[NEW]** `POST /api/categories` — créer une catégorie custom (body: `{ "name": "..." }`, response: `CategoryInfo`)
- **[BREAKING]** `/backup-select` — paramètre query `backgrounds` renommé `medias` (couvre backgrounds + categories)
- **[BREAKING]** `/reset-select` — paramètre query `backgrounds` renommé `medias`
- **[BREAKING]** `QuestionType "NORMAL"` → `"SPEEDY"` — la constante et la valeur JSON changent ; fallback en lecture (`"NORMAL"` lu comme `"SPEEDY"` pour rétrocompatibilité des fichiers existants)

> **Note rétrocompatibilité SPEEDY** : le serveur normalise `"NORMAL"` → `"SPEEDY"` à la lecture des fichiers disque et à la réception d'une requête POST /questions. Les anciens clients qui envoient TYPE=NORMAL continuent de fonctionner.

---

## [20260520] — Mode ARDOISE v5.6.0

- **[NEW]** `QuestionType "ARDOISE"` — nouveau type de question saisie libre via clavier virtuel
- **[NEW]** `KeyboardType "AZERTY" | "NUMPAD"` — layout clavier pour ARDOISE
- **[NEW]** `ArdoiseAnswer { TEXT: string, SUBMITTED_AT: number }` — structure réponse équipe
- **[NEW]** `Question.ARDOISE_KEYBOARD_TYPE?: KeyboardType` — layout clavier (omitempty, rétrocompatible)
- **[NEW]** `GameState.ARDOISE_ANSWERS: map[string]ArdoiseAnswer` — réponses par équipe (jamais null)
- **[NEW]** `ACTION: "ARDOISE_INPUT"` (VPlayer→Server) — mise à jour réponse texte (throttlé, fire-and-forget)

> Aucune breaking change — `ARDOISE_ANSWERS` est initialisé à `{}` et toujours sérialisé.

---

## [20260515] — MEMOTION Secret Mode v5.5.0

- **[NEW]** `Question.MOTION_MEMORIZE_DURATION` int — durée phase MEMORIZE en secondes. 0 = mode standard (omitempty, rétrocompatible)
- **[CHANGED]** `GameState.MEMOTION_SUBPHASE` — nouvelle valeur `"MEMORIZE"` ajoutée (additive, pas de breaking change)

---

## [20260503] — v5.0.0 : Nouveau type de jeu MEMOTION

### Contrats créés

- **[NEW]** `contracts/memotion-game.md` — Type de jeu MEMOTION : structure MotionCard, subphases, modes équipe (v5.0.0)

### Changements de l'API WebSocket

- **[NEW]** `ACTION: "MEMOTION_SELECT"` — sélectionner une carte en phase GRID
- **[NEW]** `ACTION: "MEMOTION_REVEAL"` — révéler la réponse (passer QUESTION → REVEAL)
- **[NEW]** `ACTION: "MEMOTION_DONE"` — terminer la carte (passer REVEAL → DONE, attribuer points)
- **[NEW]** `ACTION: "MEMOTION_SET_TEAMS"` — configurer équipes participantes

### Changements de GameState

- **[NEW]** `GameState.MEMOTION_SUBPHASE` (string) — sous-phase du jeu MEMOTION (GRID|QUESTION|REVEAL|DONE)
- **[NEW]** `GameState.MEMOTION_SELECTED` (string) — ID de la carte sélectionnée
- **[NEW]** `GameState.MEMOTION_CARD_STATES` (map[string]CardState) — états cartes (AVAILABLE|SELECTED|PLAYING|DONE)
- **[NEW]** `GameState.MEMOTION_CARD_TEAMS` (map[string]string) — assignation équipes par carte
- **[NEW]** `GameState.MEMOTION_CURRENT_TEAM` (string) — équipe actuelle (mode CHACUN_SON_TOUR/TANT_QUE_JE_GAGNE)
- **[NEW]** `GameState.MEMOTION_PARTICIPATING_TEAMS` ([]string) — liste équipes
- **[NEW]** `GameState.MEMOTION_CURRENT_TEAM_COLOR` ([3]int) — couleur RGB équipe actuelle

### Changements de structures Question

- **[NEW]** Question type `"MEMOTION"`
- **[NEW]** Champ `MEMOTION_CARDS` — tableau struct `MotionCard` avec RECTO/VERSO/REVEAL + DIFFICULTY
- **[NEW]** Champ `MEMOTION_MODE` — mode de jeu (SOLO|CHACUN_SON_TOUR|TANT_QUE_JE_GAGNE)
- **[NEW]** Champ `TIME` — timer per-carte (secondes)

### Changements de l'API REST

- **[NEW]** `POST /api/memotion/questions` — créer set MEMOTION
- **[NEW]** `GET /api/memotion/questions` — lister sets MEMOTION
- **[NEW]** `GET /api/memotion/questions/{setId}` — récupérer set détail
- **[NEW]** `PUT /api/memotion/questions/{setId}` — mettre à jour set
- **[NEW]** `DELETE /api/memotion/questions/{setId}` — supprimer set
- **[NEW]** `POST /api/memotion/questions/{setId}/cards` — ajouter carte au set
- **[NEW]** `PUT /api/memotion/questions/{setId}/cards/{cardId}` — mettre à jour carte
- **[NEW]** `DELETE /api/memotion/questions/{setId}/cards/{cardId}` — supprimer carte

### Notes de compatibilité

- **Rétrocompatibilité WebSocket** : Actions MEMOTION_* n'affectent que les jeux type MEMOTION — pas de breaking pour questions NORMAL/QCM/MEMORY existantes
- **Rétrocompatibilité GameState** : Nouveaux champs MEMOTION_* posés à nil/empty quand phase ≠ MEMORY (MEMOTION est une spécialisation de MEMORY)
- **Timer per-carte** : Structure identique à MEMORY mais appliquée à chaque carte individuellement, pas au jeu global

---

## [20260428] — v3.8.0 : WebSocket endpoints dédiés + ACK buzzer + payload buzzer

### Contrats créés

- **[NEW]** `contracts/websocket-endpoints.md` — Endpoints WS dédiés `/ws/admin`, `/ws/tv`, `/ws/player` avec table de filtrage des messages
- **[NEW]** `contracts/buzzer-ack-protocol.md` — Protocole `MSG_ID` / `ACK` / retry pour buzzers physiques
- **[NEW]** `contracts/buzzer-payload-filter.md` — Whitelist des actions autorisées vers buzzers

### Changements de l'API WebSocket

- **[NEW]** `/ws/admin` — nouvel endpoint WebSocket pour l'interface admin
- **[NEW]** `/ws/tv` — nouvel endpoint WebSocket pour l'affichage TV
- **[NEW]** `/ws/player` — nouvel endpoint WebSocket pour les VJoueurs
- **[CHANGED]** `/ws` — conservé comme alias vers `/ws/admin` (rétrocompatible, non BREAKING)

### Changements de protocole buzzer

- **[NEW]** `Message.MSG_ID` (string, optionnel, omitempty) — identifiant ACK sur LED_SET, OTA_UPDATE, WIFI_CONFIG
- **[NEW]** `ACTION: "ACK"` — nouvelle action buzzer→serveur pour accusé de réception
- **[NEW]** `Bumper.ACK_PENDING` (bool, omitempty) — champ frontend pour badge ⚠

### Changements de configuration

- **[NEW]** `ServerConfig.ack_timeout_ms` (int, défaut 2000) — timeout avant retry ACK
- **[NEW]** `ServerConfig.ack_max_retries` (int, défaut 3) — max retentatives ACK

### Notes de compatibilité

- **Rétrocompatibilité frontend** : `/ws` reste fonctionnel — pas de BREAKING pour les déploiements existants
- **Rétrocompatibilité firmware** : `MSG_ID` est optionnel — les firmwares < v3.8.0 sans support ACK fonctionnent normalement (retry côté serveur puis abandon sans ACK)
- **Rétrocompatibilité protocole TCP** : aucun changement sur le protocole TCP v1 (port 1234)
- **Filtrage payload** : suppression de `UPDATE` vers `buzzerHub` est transparent côté firmware (n'utilisait pas ce message depuis v3.4.0)
