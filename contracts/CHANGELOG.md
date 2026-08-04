# Changelog des Contrats API

---

## [20260804] — Libération de place d'un VJoueur connecté (#134)

> Le bouton « Réinscription » (`RELEASE_BUMPER_NAME`, #122) ne posait qu'une **autorisation
> différée** consommable par une future tentative — sans jamais toucher une session active. Sur un
> joueur encore connecté, il ne produisait donc **aucun effet observable**. #134 lui donne le
> comportement attendu dans ce cas, sans rien changer au cas #122 d'origine.
> Plan : `_work/reports/planner-20260804-115318.md`.
> Maquette : `_work/mockups/134-seat-release.md`.
> Contrat détaillé : `contracts/seat-release.md`.

- **[NEW]** Motif `SEAT_RELEASED`, porté par `PLAYER_EVICTED { REASON }` et
  `PLAYER_REJECTED { REASON }`. Famille **renvoi** (`REDIRECT_MESSAGES`), comme `PLAYER_REMOVED`.
  Distinction avec ce dernier : `PLAYER_REMOVED` supprime le bumper (score et équipe perdus),
  `SEAT_RELEASED` le **conserve** — le joueur qui reprend le siège retrouve son score.
- **[CHANGED]** `RELEASE_BUMPER_NAME { ID }` — **action inchangée**, comportement élargi. Si le
  bumper ciblé est **connecté** : `PLAYER_EVICTED { SEAT_RELEASED }` ciblé → enregistrement au
  registre d'éviction → **re-clé** du bumper sous un nouvel ID (même struct : score, équipe et
  historique conservés) avec `Connected = false` → autorisation de reprise. Si le bumper est
  **déconnecté** : strictement inchangé (#122) — autorisation différée seule.
- **[CHANGED]** Règle de contrat ajoutée : l'ordre des étapes est **normatif**. La notification
  ciblée doit précéder le re-clé (`SendToPlayerID` résout par l'**ancien** `PlayerID` — après re-clé
  le joueur n'est plus adressable), et le re-clé est **obligatoire**, pas une commodité : sans lui, un
  joueur se reconnectant avec son ancien ID emprunte le cas 1 de `ReconnectOrCreateVirtualPlayer`,
  qui remet `reclaimAuthorizedUntil` à zéro et **annule silencieusement** la libération demandée.
- **[CHANGED]** Règle de contrat ajoutée : la connexion WebSocket n'est **pas** fermée de force —
  `PLAYER_EVICTED` est en file sur le canal `Send` et une fermeture immédiate risquerait de perdre la
  notification. Même contrat que `DELETE_BUMPER`. L'invalidation réelle de la session est le re-clé.
- **[CHANGED]** Règle de contrat ajoutée : la reprise du siège n'introduit **aucun chemin nouveau** —
  c'est le chemin de reprise #122 (`PLAYER_CONNECT` sans `ID` + nom + autorisation valide →
  `reattachVirtualPlayerUnsafe`). Le client arrive sans `ID` car le traitement de `PLAYER_EVICTED`
  purge `vplayer_id`. **N'importe quel** joueur peut reprendre le siège, pas seulement l'occupant
  précédent : le siège porte le score, pas la personne.

**Aucun changement BREAKING.** Aucune action, aucun champ ajouté, renommé, retiré ni retypé — seule
une nouvelle **valeur** de `REASON` apparaît sur des messages existants. Un client antérieur retombe
sur `DEFAULT_REDIRECT_MESSAGE` (repli déjà en place et déjà testé pour un motif inconnu).

---

## [20260803b] — Recalibrage des délais de liaison + transmission serveur→client (#130)

> Axe indépendant de #127/#129 : ceux-ci réduisent le **volume** de messages, celui-ci porte sur la
> **marge de tolérance temporelle**. Constat central : avec un ping à 3 s et un `ReadDeadline` à 5 s,
> la perte d'un **seul** ping suffisait à fermer la connexion côté serveur — la marge de 2 s ne
> représentait aucune tolérance réelle. Plan : `_work/reports/planner-20260803-214210.md`.
> Maquette : `_work/mockups/130-timing-recalibration.md`.
> Contrat détaillé : `contracts/liveness-timing.md`.

- **[NEW]** `HEARTBEAT { …, DEAD_LINK_TIMEOUT_MS }` — champ additif portant, en **valeur absolue**,
  le silence au-delà duquel un client web doit déclarer la liaison morte. Le serveur devient la
  source de vérité unique du seuil, et non plus seulement de la cadence : #118 transmettait
  `INTERVAL_MS`, mais le **multiplicateur** (3) restait codé en dur côté client — c'est ce dernier
  point de dérive possible que ce champ supprime.
- **[CHANGED]** `HEARTBEAT.INTERVAL_MS` — valeur émise : 3000 → **2000** ms. Le sens du champ est
  inchangé (cadence réelle du ticker `writePump`).
- **[CHANGED]** `ReadDeadline` serveur pour les clients web : 5000 → **7000** ms, soit
  `(N+1) × P + RTT + marge` avec N = 2 pings tolérés et P = 2000 ms. Passe de **0** à **2** pings
  intégralement perdus tolérés.
- **[CHANGED]** Seuil client de liaison morte : 9000 → **4000** ms (ajustement GATE 2 — le plan
  recommandait 5000, l'utilisateur a choisi la variante réactive à 4000), et granularité de
  vérification 1000 → **500** ms. Détection d'un lien réellement mort : 9,0–10,0 s → **4,0–4,5 s**.
- **[CHANGED]** Règle de contrat ajoutée : le client applique `DEAD_LINK_TIMEOUT_MS` s'il l'a reçu,
  sinon `INTERVAL_MS × 3` (comportement #118), sinon `3000 × 3`. Les trois états sont fonctionnels.
- **[CHANGED]** Règle de contrat ajoutée : l'**ordre de détection est délibérément inversé** — le
  client détecte désormais avant le serveur (4,0–4,5 s contre 7 s). Sur un lien mort, la trame de
  fermeture du serveur n'atteint jamais le client (problème fondateur de #118) : c'est au client de
  reprendre l'initiative. La connexion serveur périmée est absorbée par la garde anti-zombie
  existante (`IsPlayerIDConnected`, #109/#120).

**Périmètres explicitement non modifiés** : buzzers physiques (`/ws/buzzer`, firmware ESP32-C3
contraint, et qui n'a jamais reçu `HEARTBEAT`), canal `/ws/logs`, et `SetWriteDeadline` (l'abaisser
rapprocherait le seuil d'échec d'écriture, qui ferme la connexion — l'inverse du but recherché).

**Aucun changement BREAKING.** `DEAD_LINK_TIMEOUT_MS` est additif ; les quatre combinaisons
ancien/nouveau client × ancien/nouveau serveur sont fonctionnelles (contrat §6).

---

## [20260803] — Ciblage des broadcasts par événement, hors PREPARE/READY (#129)

> Prolongement de #127. Trois événements par-participant — connexion, déconnexion, frappe ARDOISE —
> déclenchaient chacun un `UPDATE` complet vers **tous** les VJoueurs, à n'importe quel moment de la
> partie. La réduction de #127 ne s'y appliquait pas (conditionnée aux phases PREPARE/READY). Audit
> des consommateurs : les VJoueurs n'exploitent aucune de ces données, hors l'écho de leur propre
> état. Plan : `_work/reports/planner-20260803-170653.md`.
> Maquette : `_work/mockups/129-broadcast-targeting.md`.
> Contrat détaillé : `contracts/vplayer-payload-filter.md` §5.

- **[CHANGED]** `UPDATE` déclenché par `onPlayerDisconnected` — n'est plus diffusé aux VJoueurs
  (Admin, TV et buzzers inchangés). Aucun affichage VJoueur ne dépend de l'état de connexion des
  autres joueurs (`sortedPlayers` est gaté `!isVPlayer` depuis #127).
- **[CHANGED]** `UPDATE` déclenché par `handlePlayerConnect` (création **et** reconnexion) — n'est
  plus diffusé à l'ensemble des VJoueurs, mais **ciblé sur le participant concerné**, qui en a besoin
  pour retrouver son bumper après reconnexion (#118/#120/#122). Admin, TV et buzzers inchangés.
- **[CHANGED]** `UPDATE` déclenché par `PONG` en phase `PREPARE` — n'est plus diffusé à la **TV**.
  #127 avait retiré les VJoueurs de cette rafale en conservant Admin + TV + buzzers ; la TV n'affiche
  qu'un libellé statique pendant PREPARE (`PlayerDisplay.jsx:1358-1373`) et n'a besoin que des deux
  bornes de la fenêtre, qu'elle continue de recevoir (entrée en PREPARE, transition en READY). Elle
  passe de N+2 à 2 `UPDATE`. **Les buzzers physiques restent ciblés délibérément** : cet `UPDATE` est
  leur seul signal de phase sur le chemin WebSocket pendant la fenêtre, `broadcastGameState()` ne les
  incluant pas.
- **[CHANGED]** `UPDATE` déclenché par `ARDOISE_INPUT` — diffusé à **l'admin seul**. La TV n'affiche
  les réponses qu'en phase `REVEALED` (`PlayerDisplay.jsx:2427`) et le VJoueur ne lit jamais
  `ARDOISE_ANSWERS` (saisie en état local). Les buzzers physiques n'ont aucun champ ARDOISE dans leur
  payload.
- **[CHANGED]** Règle de contrat ajoutée (§5) : un `UPDATE` déclenché par un événement propre à **un**
  participant n'est diffusé à tous les VJoueurs que si son contenu est réellement consommé par eux ;
  sinon Admin/TV, plus un **écho ciblé** au participant concerné. Le chemin d'écho ciblé ne doit
  jamais appeler `ApplyVPlayerBroadcastConnEvents()` (même invariant que #127 CA7).
- **[CHANGED]** Règle de contrat ajoutée (§5) : les `UPDATE` d'`ARDOISE_INPUT` peuvent être regroupés
  sur une fenêtre ≤ 150 ms, sous deux conditions — vidage immédiat à tout changement de phase, et
  construction du payload **au moment de l'émission** (une émission retardée est alors redondante,
  jamais périmée).
- **[NEW]** `sendStateToClient()` (sur `HELLO`) est explicitement contractualisé comme envoyant le
  payload **complet et non réduit**, quel que soit le type de client — seule source permettant à une
  session sans `vplayer_id` de retrouver son identité par balayage `NAME`. Ne pas « optimiser ».

**Aucun changement BREAKING.** Aucun message, champ ni type n'est ajouté, renommé, retiré ni retypé —
seul le jeu des destinataires de messages existants change.

**Correction d'équité obtenue au passage** : `ARDOISE_ANSWERS`, qui transportait vers le navigateur de
chaque VJoueur le texte que les autres équipes étaient en train de saisir, ne leur est plus transmis.

---

## [20260802] — Diffusion groupée et payload réduit VJoueur à PREPARE→READY (#127)

> À chaque PONG reçu en phase PREPARE, le serveur rediffusait l'état complet du jeu à **tous** les
> clients : pour N participants, chaque VJoueur recevait N+2 payloads complets en quelques centaines
> de millisecondes, au moment précis des déconnexions rapportées. Plan :
> `_work/reports/planner-20260802-212049.md`. Maquette : `_work/mockups/127-broadcast-matrix.md`.
> Contrat détaillé : `contracts/vplayer-payload-filter.md`.

- **[CHANGED]** `UPDATE` déclenché par `PONG` en phase `PREPARE` — n'est plus diffusé à
  `ClientTypeVPlayer`. Admin et TV continuent de le recevoir à chaque PONG (l'admin affiche la
  progression « prêt » équipe par équipe en direct, `GamePage.jsx:1050-1057`). Le VJoueur reçoit
  désormais **2** `UPDATE` sur la fenêtre PREPARE→READY au lieu de N+2 : l'entrée en PREPARE, puis
  la transition en READY. Rétrocompatible — aucun message nouveau, aucun champ modifié.
- **[CHANGED]** `UPDATE` émis sur changement de phase (`broadcastGameState`, toutes phases) — passe
  du chemin non filtré au chemin filtré déjà utilisé partout ailleurs : Admin garde le payload
  complet, TV et VJoueur reçoivent le payload sans métadonnées `FIRMWARE_VERSION` / `IS_OUTDATED` /
  `OTA_STATUS` / `OTA_PERCENT` / `ACK_PENDING`. Correction d'incohérence — ces champs n'ont jamais
  été consommés par TV/VJoueur.
- **[CHANGED]** `UPDATE` destiné à `ClientTypeVPlayer` **pendant les seules phases `PREPARE` et
  `READY`** — la carte `bumpers` est réduite au seul bumper du destinataire. `GAME` et `teams`
  restent **intégralement inchangés**. Ne s'applique jamais à un client dont le `PlayerID` n'est pas
  encore identifié (une session sans `vplayer_id` retrouve son bumper par balayage NAME).
  Voir `contracts/vplayer-payload-filter.md` §2 pour la règle complète et les consommateurs audités.

**Aucun changement BREAKING.** Aucun endpoint, aucun message, aucun champ n'est ajouté, renommé,
retiré ni retypé. Seuls varient le *nombre de destinataires* de messages existants et le *nombre
d'entrées* d'une carte, sur deux messages, pour un seul type de client.

---

## [20260729] — Battement applicatif client → serveur (#118)

> Un VJoueur dont le réseau tombait restait indéfiniment sur un socket zombie : le serveur le
> désinscrivait, mais sa trame de fermeture ne traversait pas le réseau coupé, donc le client ne
> détectait jamais rien et ne se reconnectait pas. Seul un rechargement de page rétablissait la
> liaison. Plan : `_work/reports/plan-20260729-190000.md`.
> Maquette : `_work/mockups/118-vplayer-connection-banner.html`.

- **[NEW]** `HEARTBEAT { INTERVAL_MS }` (serveur → client web, **sans réponse attendue**). Émis
  par `writePump` sur le ticker par client déjà existant (3 s), en complément — et non en
  remplacement — de la trame ping protocolaire. `INTERVAL_MS` porte la cadence réelle du ticker,
  dont le client **dérive** son seuil de détection (3 × la cadence) plutôt que de le coder en dur.
  Concerne les trois endpoints web (`/ws/admin`, `/ws/tv`, `/ws/player`).
- **[CHANGED]** Règle de contrat ajoutée : **les trames ping du protocole WebSocket ne
  constituent pas une preuve de vie exploitable côté client** — le navigateur y répond
  automatiquement et n'expose aucun événement JavaScript. Un client web doit donc surveiller le
  `HEARTBEAT` applicatif, considérer la liaison morte au-delà du seuil dérivé, puis fermer le
  socket et se reconnecter de lui-même.

> **Choix d'architecture** : le battement va du serveur vers le client, et non l'inverse. Il
> réutilise ainsi un ticker existant, reste **hors du canal d'entrée sérialisé par un consommateur
> unique** qui porte aussi les actions de jeu, et n'ajoute qu'un seul minuteur côté client.
> Comparaison détaillée : `_work/reports/plan-analysis-20260729-201500-heartbeat.md`.

**Aucun changement BREAKING.** `HEARTBEAT` est additif et sans réponse attendue ; un client
antérieur l'ignore proprement (branche `default` du switch d'actions) et conserve exactement son
comportement actuel — le serveur continuant par ailleurs de le surveiller via ses trames ping.

---

## [20260728] — VJoueur renvoyé sans explication à l'inscription (#120)

> Un VJoueur accepté par le serveur (sa carte apparaît côté animateur et TV) était malgré tout
> renvoyé à la saisie de pseudo, sans message, puis refusé en `NAME_TAKEN` sur son propre pseudo.
> Plan : `_work/reports/plan-20260728-101500.md`.
> Maquette : `_work/mockups/120-enroll-redirect-message.html`.

- **[NEW]** `PLAYER_EVICTED { REASON }` (serveur → client VJoueur, ciblé) — notifie explicitement
  un joueur dont le bumper vient d'être supprimé (`PLAYER_REMOVED`) ou purgé par une nouvelle
  partie (`GAME_RESET`). Le client n'a plus à déduire son éviction de l'absence de son bumper
  dans une mise à jour de roster : **cette déduction était la cause racine du renvoi silencieux**.
- **[CHANGED]** Règle de contrat ajoutée sur le roster : l'absence d'un bumper dans une mise à
  jour n'est jamais, à elle seule, un motif de renvoi côté client. Seul `PLAYER_EVICTED` fait foi.

**Aucun changement BREAKING.** `PLAYER_EVICTED` est additif ; un client antérieur qui l'ignore
conserve son comportement actuel.

---

## [20260727] — Réponses ardoise triées par ordre d'arrivée (#117)

> Sur la page admin, les réponses ARDOISE s'affichaient dans l'ordre de la liste d'équipes, sans
> rapport avec l'ordre d'arrivée, et aucun délai n'était visible.
> Plan : `_work/reports/plan-20260727-093000.md`.
> Maquette : `_work/mockups/117-ardoise-answer-order.html`.

- **[NEW]** `ArdoiseAnswer.STARTED_AT` — horodatage microseconde du **premier** caractère reçu
  pour l'équipe, figé à la première écriture. `SUBMITTED_AT` conserve son sens inchangé
  (dernière mise à jour) et reste le champ utilisé partout ailleurs.
- **[CHANGED]** `ARDOISE_INPUT` (VPlayer → serveur) — le premier envoi porteur d'un texte non
  vide est désormais émis **immédiatement** au lieu d'attendre 200 ms sans frappe. Les envois
  suivants restent régulés à ~200 ms. Aucun changement de format de message : seule la cadence
  d'émission change.

**Aucun changement BREAKING.** `STARTED_AT` est additif ; une réponse antérieure le reçoit à `0`
et le frontend se replie sur `SUBMITTED_AT`.

---

## [20260726] — Palette d'équipes : 16 couleurs et LED exacte (#113)

> Le nombre de couleurs d'équipe (8) était insuffisant en usage réel, et deux équipes pouvaient
> recevoir la même couleur. Plan : `_work/reports/plan-20260726-121500.md`.
> Maquette validée : `_work/mockups/113-team-colors-palette.html`.

- **[NEW]** `Team.COLOR_NAME` — le champ existait déjà dans le modèle Go mais n'était **jamais
  écrit** par le frontend, rendant mort le chemin de résolution LED exacte. Il devient un champ
  effectivement renseigné à chaque sélection ou attribution de couleur.
- **[CHANGED]** Palette d'équipes portée de 8 à **16 couleurs** (8 teintes × 2 tons), avec clés
  canoniques et ordre d'attribution normatif. Nouvelle section « Palette d'équipes » dans
  `models.md` — table partagée frontend/backend qui doit rester strictement identique des deux
  côtés.
- **[CHANGED]** `teamColorPalette` (backend) — les 16 clés actuelles ne portaient en réalité que
  **9 couleurs distinctes** (`rouge`/`red`, `vert`/`green`… étaient des alias FR/EN). Remplacée
  par 16 entrées réellement distinctes ; les alias anglais sont conservés vers les tons vifs.
- **[CHANGED]** Les valeurs RGB des 8 couleurs existantes changent légèrement (alignement sur la
  nouvelle grille de teintes). Les équipes déjà enregistrées conservent leur `COLOR` d'origine et
  restent résolues par approximation de teinte côté LED — pas de migration nécessaire.

**Aucun changement BREAKING.** `COLOR_NAME` est additif et optionnel : un état de jeu ou un
backup antérieur reste chargeable tel quel.

---

## [20260726] — Fix urgent : badge CONN_STATE bloqué en ROUGE (#109)

> Bug bloquant remonté en test réel post-QUALIF v5.7.20 : un VJoueur qui se déconnecte passait
> **directement** en `red` (jamais visible en `orange`), et restait parfois bloqué. Handoff :
> `_work/handoff/task-dev-backend-20260726-090000.md`.

- **[FIX]** `ApplyVPlayerBroadcastConnEvents` ne compte plus le broadcast qui **annonce** une
  déconnexion comme un `MESSAGE_LOST` pour le VJoueur concerné — `orange` est désormais réellement
  visible avant tout passage à `red`. Un broadcast **ultérieur** (pendant que le participant est
  toujours déconnecté) déclenche toujours `MESSAGE_LOST` normalement (D4 inchangé).
- **Diagnostic** : l'hypothèse alternative (la reconnexion par ID ne déclencherait jamais
  `RECONNECT`, bloquant en `red` indéfiniment) a été vérifiée et **infirmée** par test — la
  reconnexion par ID fonctionne correctement.
- Aucun changement de contrat de champ — comportement uniquement.

---

## [20260725] — Fix R1 : reconnexion VJoueur par ID, plus de fusion par nom (#109)

> Répond au blocage code-review CRITIQUE (`code-review-20260725-122357.md`) sur
> `ReconnectOrCreateVirtualPlayer` : la consolidation par nom supprimait silencieusement des
> données en cas de collision (deux VJoueurs homonymes distincts). Plan :
> `_work/reports/planner-20260725-143029-r1-fix.md`.

- **[NEW]** `PlayerConnectPayload.ID` (string, `omitempty`) — ID de bumper reçu dans un
  `PLAYER_CONNECTED` précédent, à renvoyer pour une reconnexion non-ambiguë. Absent au premier
  enrôlement ; rétrocompatible (anciens clients sans ID → traités comme un enrôlement par nom,
  rejeté si le nom est déjà pris).
- **[NEW]** `PlayerRejectedPayload.Reason` : nouvelle valeur `NAME_TAKEN` — nom déjà utilisé par
  un autre VJoueur (connecté ou déconnecté), sans ID résolvable pour prouver la propriété.
- **[BREAKING — comportement, pas format]** L'identité d'un VJoueur repose désormais **sur l'ID
  uniquement**. Le matching par nom seul (avec fusion/suppression de doublons) est **retiré** :
  une collision de nom sans ID est **rejetée**, jamais fusionnée. Un ancien client qui ne stocke
  pas encore l'ID (avant la mise à jour frontend correspondante) continuera à fonctionner pour un
  premier enrôlement, mais toute tentative de reconnexion par nom sur un nom déjà pris sera
  rejetée au lieu d'être silencieusement acceptée — comportement voulu (règle produit).
- **[FIX]** `engine.ReconnectOrCreateVirtualPlayer` ne supprime plus jamais de bumper. L'atomicité
  (verrou unique, corrige la course TOCTOU #109 d'origine) est conservée.
- **[CHANGED]** (suite, même jour) Purge des VJoueurs déplacée de `StartEnrollment` vers
  `InitGame`/`NEW_GAME`, et rendue **inconditionnelle** (tous les VJoueurs, connectés ou non —
  auparavant limitée aux déconnectés). Précision produit : une partie démarre toujours avec des
  données vierges, il n'existe pas de VJoueur « legacy » à nettoyer entre deux ouvertures
  d'enrôlement au sein d'une même partie ; `NEW_GAME` est la vraie limite de fraîcheur.
  `StartEnrollment` peut être rouvert en cours de partie (inviter plus de monde) sans jamais
  toucher au roster existant. Les buzzers physiques ne sont jamais purgés (matériel persistant),
  seul leur score est remis à zéro comme avant. Toujours une hygiène de confort — `NAME_TAKEN`
  seul suffit à éviter toute perte/fusion de données sur collision de nom.

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
