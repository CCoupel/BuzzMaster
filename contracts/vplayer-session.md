# Contrat — Jeton de session des tablettes joueurs (VJoueur) — v13.0.0

**Origine** : audit `security` du LOT-2.1 (`_work/reports/security-20261123-170500.md`), faille **M1** (usurpation d'un
VJoueur par `PLAYER_CONNECT` avec son seul ID). Décision de l'utilisateur du 2026-11-23 : corriger dans v13.0.0
(ordre `_work/tasks/planner-20261123-security-m1.md`).
**Issue** : nouvelle issue au GATE de l'issue entrante (voir plan `_work/reports/plan-20261123-picto.md`, révision 7).
**Statut** : contrat de référence, écrit avant implémentation (contract-first) — **révision 1** (2026-11-23).
Choix techniques du planner marqués **[planner]**.

> Ce contrat **complète** `websocket-actions.md` (§ `PLAYER_CONNECT` / `PLAYER_CONNECTED` / `PLAYER_REJECTED`) et
> `seat-release.md` (§5). En cas de divergence, ce document tranche pour tout ce qui touche au jeton.

---

## 1. Problème

Aujourd'hui, l'ID du bumper est le **seul** secret d'une tablette (#109, #246). `PLAYER_CONNECT {NAME, ID}` est
accepté sans aucune preuve de possession. Or cet ID se lit sans effort : il figure dans les clés de `bumpers` et dans
`PICTO_DRAWER`, diffusés à `/ws/tv`, qui est ouvert.

N'importe quel client qui rejoue cet ID :
- est lié à la place du joueur (`BindClientPlayerID`) ;
- évince le vrai joueur (`DUPLICATE_SESSION`) ;
- agit en son nom. En PICTO, il reçoit `PICTO_WORD` et peut dessiner. Dans les autres types, il peut répondre ou
  buzzer à sa place.

## 2. Principe

- À l'enrôlement, le serveur remet à la tablette un **jeton de session** secret, aléatoire et propre au bumper.
- **Toute reconnexion par ID doit présenter ce jeton.** Sans jeton valable, la liaison est refusée.
- L'ID reste un identifiant public. Seul le jeton prouve la possession de la place.
- **Périmètre** : seules les tablettes joueurs (`/ws/player`, VJoueur) sont concernées.
  - **Buzzers physiques BuzzClick non touchés** : identifiés par leur MAC sur `/ws/buzzer`, aucun changement de
    firmware ni de protocole (règle de compatibilité de CLAUDE.md).
  - Admin, anim et TV : non concernés.

## 3. Jeton

- **Génération** : 32 octets de `crypto/rand`, encodés en base64url sans remplissage (43 caractères). Jamais
  `math/rand`.
- **Stockage serveur** : seulement l'**empreinte** SHA-256 du jeton, dans un fichier dédié
  `data/config/vplayer_sessions.json` de la forme `{"TOKENS": {"<bumperID>": "<sha256 hex>"}}`, avec une écriture
  atomique. **Jamais dans `Bumper`** **[planner]** : la structure `Bumper` est sérialisée vers tous les clients, et
  un fichier séparé ne peut pas fuiter par un sérialiseur oublié.
- **Comparaison** : empreinte du jeton reçu comparée à l'empreinte stockée en **temps constant**
  (`crypto/subtle.ConstantTimeCompare`).
- **Confidentialité** : le jeton en clair n'est envoyé qu'au client qui vient d'être lié (`PLAYER_CONNECTED`, par
  `SendToClient`). Il n'est jamais journalisé (ni `/ws/logs`, ni fichier), jamais dans `GameState`, jamais dans un
  `UPDATE`. Les journaux ne mentionnent que l'ID et l'issue (`VPLAYER_TOKEN_OK`, `VPLAYER_TOKEN_INVALID`,
  `VPLAYER_TOKEN_ISSUED`).
- **Durée de vie** : celle du bumper. Le jeton est stable tant que le bumper existe et ne tourne pas à chaque
  reconnexion **[planner]**, pour ne pas casser deux onglets du même appareil (#246 F-ID).

## 4. Protocole

### 4.1 `PLAYER_CONNECT` (client → serveur) — champ ajouté

| Champ | Type | Requis | Description |
|---|---|---|---|
| NAME | string | ✅ | inchangé |
| ID | string | ❌ | inchangé |
| **TOKEN** | string | ❌ | **[NEW]** Jeton reçu dans un `PLAYER_CONNECTED` précédent, envoyé **avec** `ID`. Absent au premier enrôlement |

### 4.2 Matrice de décision — remplace le cas 1 de `websocket-actions.md` § `PLAYER_CONNECT`

| # | Situation | Résultat |
|---|---|---|
| 1a | `ID` résout un VJoueur, une empreinte existe, et `TOKEN` correspond | Reconnexion (comportement actuel du cas 1) |
| 1b | `ID` résout un VJoueur, une empreinte existe, et `TOKEN` est absent ou faux | **`PLAYER_REJECTED { REASON: "INVALID_SESSION" }`**. Aucune liaison, **aucune éviction** du client lié, aucun `UPDATE`. Journal `WARN VPLAYER_TOKEN_INVALID id=…` |
| 1c | `ID` résout un VJoueur **sans empreinte** (place créée avant v13.0.0 : migration, §6) | Reconnexion acceptée, puis **nouveau jeton émis** (premier arrivé) ; journal `VPLAYER_TOKEN_ISSUED reason=legacy` |
| 2 à 4 | inchangés : ID périmé (registre d'éviction), reprise #122 / #134, nom pris, nouvel enrôlement | Pour toute **nouvelle** place, ou toute place **re-clée** (reprise #122, libération #134), un nouveau jeton est émis et l'ancienne empreinte est supprimée |

- Le contrôle a lieu **avant** `BindClientPlayerID`. Un refus 1b ne touche donc jamais la session légitime en cours.
- Le refus 1b est renvoyé au seul émetteur. Il ne dit pas si l'ID existe autrement que par le motif, ce qui suffit :
  l'ID est public de toute façon.

### 4.3 `PLAYER_CONNECTED` (serveur → client) — champ ajouté

| Champ | Type | Description |
|---|---|---|
| ID, NAME, TEAM | — | inchangés |
| **TOKEN** | string | **[NEW]** Jeton de la place, envoyé à **chaque** `PLAYER_CONNECTED` (enrôlement, reconnexion, reprise). Le client **DOIT** le conserver avec l'ID et le renvoyer dans `PLAYER_CONNECT.TOKEN` |

Envoyer le jeton à chaque `PLAYER_CONNECTED`, plutôt qu'au seul enrôlement, permet à un client qui l'aurait perdu
après une reconnexion légitime de le retrouver. Cela ne crée aucune fuite : seul le client lié le reçoit
**[planner]**.

### 4.4 `PLAYER_REJECTED` — nouvelle valeur de `REASON`

- `INVALID_SESSION` : la place existe, mais cette tablette n'en a pas le jeton. C'est un motif de **redirection**
  (`REDIRECT_MESSAGES` de `VPlayerPage.jsx`) :
  - le client purge sa session locale (`clearOwnVPlayerSession`) et revient à l'inscription ;
  - il affiche : « Cette place est déjà liée à un autre appareil. Demandez à l'animateur de libérer la place, puis
    reconnectez-vous avec votre nom. »
  - Le joueur légitime qui aurait perdu son jeton (stockage effacé, autre appareil) reprend sa place par le chemin
    existant : libération de place par l'animateur (#134), puis `PLAYER_CONNECT` sans ID (#122). Son score est
    conservé.

## 5. Côté tablette (front embarqué)

- **Stockage** : clé `localStorage` `vplayer_token`, écrite à la réception de `PLAYER_CONNECTED.TOKEN`, avec la même
  règle d'onglet que `vplayer_id` (#246 F-ID : n'écrire que si l'ID stocké est celui de l'onglet).
- `vplayer_token` est ajouté à `VPLAYER_SESSION_KEYS` (`utils/vplayerSession.js`) : il est effacé avec la session,
  par un seul point d'entrée (#120).
- `PLAYER_CONNECT` envoie `{ NAME, ID, TOKEN }` à chaque ouverture de la WebSocket (`VPlayerPage.jsx`, deux sites).
  Le premier enrôlement (`useWebSocket.js`, `{ NAME }`) est inchangé.
- **Garde de session** au montage de `VPlayerPage` : une session sans jeton (tablette ouverte avant la mise à jour)
  est **acceptée telle quelle**, puisque le cas 1c du serveur gère la migration. Il ne faut pas la renvoyer à
  l'inscription.
- **Écrans** : aucun écran ni champ nouveau. Le jeton est invisible pour le joueur. Seul un texte de refus est
  ajouté, dans le bandeau de redirection existant (§4.4). Ce cas ne demande pas de maquette : c'est une ligne de
  texte dans un composant existant, sans changement de disposition.

## 6. Migration et compatibilité

- **Places existantes** (`bumpers.json` d'avant v13.0.0, sans empreinte) : cas 1c, en premier arrivé. Après la mise à
  jour, le serveur redémarre et toutes les tablettes se reconnectent d'elles-mêmes en quelques secondes : chaque
  place reçoit son jeton à la première reconnexion de son propriétaire.
  - **Fenêtre résiduelle** : un attaquant qui connaîtrait l'ID et se reconnecterait **avant** le propriétaire
    obtiendrait la place. Le risque est accepté et documenté **[planner]** : il exige d'agir dans les secondes du
    redémarrage. Le propriétaire évincé reprend sa place par la libération #134.
  - **Option plus stricte, non retenue** : refuser les places sans empreinte. Toutes les tablettes en cours devraient
    alors se réinscrire après la mise à jour.
- **Ancien onglet** encore ouvert sur l'ancien front (qui n'envoie pas `TOKEN`) : si la place a déjà reçu un jeton,
  l'onglet reçoit `INVALID_SESSION` et revient à l'inscription. Recharger la page charge le nouveau front.
- **[BREAKING — interne]** `PLAYER_CONNECT` avec un ID dont la place a un jeton **exige** désormais `TOKEN`. Seul
  client concerné : le front VJoueur, embarqué dans le même binaire et mis à jour en même temps. Aucun client
  externe, aucun buzzer. Aucune donnée existante n'est réécrite.

## 7. Cycle de vie des empreintes

| Événement | Effet |
|---|---|
| Enrôlement (cas 4) | Jeton émis, empreinte enregistrée |
| Reprise #122 / libération #134 (re-clé `rekeyBumperUnsafe`) | Ancienne empreinte supprimée, nouveau jeton émis pour le nouvel ID |
| Suppression d'un bumper (`DELETE` admin, purge) | Empreinte supprimée |
| `NEW_GAME` qui purge les joueurs des tablettes | Toutes les empreintes des bumpers supprimés sont retirées |
| Sauvegarde sélective « Joueurs » / restauration | `vplayer_sessions.json` voyage avec `bumpers.json` : les mêmes drapeaux dans `handleBackupSelect`, `handleResetSelect`, `detectTARContents` et `handleRestore` |
| Redémarrage du serveur | Empreintes rechargées depuis le fichier |

## 8. Effet sur les autres types de jeu

Toutes les actions de tablette reposent sur la liaison connexion → ID (`client.PlayerID`) : `BUTTON` d'une tablette,
`VPLAYER_QCM_ANSWER`, `ARDOISE_INPUT` / `ARDOISE_FINAL`, `FLIP_MEMORY_CARD`, `PICTO_STROKES`. **Aucune n'est
modifiée** : elles deviennent sûres d'elles-mêmes, puisque la liaison exige désormais la preuve de possession.

## 9. Tests exigés

- Matrice §4.2 : 1a, 1b (jeton absent, jeton faux, jeton d'une autre place), 1c, reprise #122, libération #134, nouvel
  enrôlement.
- 1b : aucune éviction du client légitime, aucun `UPDATE`, aucun `PICTO_WORD` envoyé.
- Le jeton n'apparaît dans aucun `UPDATE` (TV, tablette, admin, anim), dans aucun journal, ni dans `bumpers.json`.
- Seule l'empreinte figure dans `vplayer_sessions.json` ; elle persiste au redémarrage.
- Cycle de vie §7 : suppression, purge `NEW_GAME`, re-clé.
- Sauvegarde et restauration « Joueurs » : la session reste valable après une restauration.
- JS :
  - stockage et envoi de `TOKEN` ;
  - effacement avec la session ;
  - règle d'onglet périmé ;
  - `INVALID_SESSION` redirige vers l'inscription avec le texte du §4.4 ;
  - une session sans jeton reste acceptée au montage.
- **Non-régression** : buzzers physiques (aucun changement sur `/ws/buzzer`), suites #109, #120, #122, #134 et #246,
  enrôlement et reconnexion dans tous les types de jeu.
