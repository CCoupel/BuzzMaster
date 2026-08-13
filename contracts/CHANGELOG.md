# Changelog des Contrats API

---

## [20260813-3] — Synchronisation live du montant crédité `/anim` (code review MAJEUR-1, #155/#156)

> Correctif suite revue de code (`_work/reports/code-reviewer-20260813-120457.md` MAJEUR-1,
> arbitrage utilisateur relayé par le teamleader) : `/anim` créditait `question.POINTS` brut alors
> que `/admin` crédite `pointsInput`, un état React local ajustable à tout moment après la
> sélection de la question (ex. manche bonus) — deux montants différents pour la même question
> selon l'interface utilisée pour créditer, silencieusement.

- **[NEW]** Action `SET_CREDIT_POINTS` (Client→Server, `admin` uniquement) — l'admin transmet son
  `pointsInput` ajusté au serveur. Absente avant ce lot : `START` transportait déjà `POINTS` dans
  son payload WS côté frontend, mais `protocol.StartPayload` ne l'a jamais décodé (champ
  silencieusement ignoré) — aucune autre voie n'existait pour que le serveur connaisse cette
  valeur.
- **[NEW]** Action `CREDIT_POINTS` (Server→Client, `ClientTypeAnim` exclusif) — rediffuse le
  montant de base courant. Sur le modèle de `NEXT_QUESTION` (B5) : diffusée sur événements
  explicites (`SET_CREDIT_POINTS`, `READY`, `NEW_GAME`, HELLO ciblé), jamais recalculée sur le
  chemin de `broadcastUpdate` — bien que, contrairement à `NEXT_QUESTION`, aucune lecture disque ne
  soit en jeu ici (valeur en mémoire, pas `loadQuestions()`).
- **[CHANGED]** Valeur par défaut : réinitialisée à `question.POINTS` (repli `1`) à chaque `READY`
  — même règle que `pointsInput` côté `GamePage.jsx` (`handleQuestionSelect`).
- **Hors périmètre de ce correctif** (documenté explicitement dans `websocket-actions.md`
  §"Animateur", pas seulement passé sous silence) :
  - QCM/MEMORY : `CREDIT_POINTS` transmet le montant de **base**, pas le montant final calculé par
    `resolvePointsAward` (pénalités/score spécifiques au type) — sans conséquence tant que ces
    types restent hors périmètre `/anim` (tranche SPEEDY #155/#156 uniquement).
  - `TIME` (durée du chrono) : même famille de problème identifiée par la revue sur
    `AnimPage.jsx`'s `handleStart`, non traitée par ce correctif (arbitrage explicite — impact
    jugé moindre, corrigeable en cours de manche via `PAUSE`).
- **Coordination frontend requise** (dev-frontend, dispatché séparément) : `GamePage.jsx` doit
  émettre `SET_CREDIT_POINTS` sur changement de `pointsInput` (debounced) ; `AnimPage.jsx` doit
  consommer `CREDIT_POINTS` au lieu de lire `question.POINTS` directement dans le calcul de
  `creditAmount`. Le JSDoc de `resolvePointsAward` (qui affirme aujourd'hui « un seul montant
  possible dans les deux interfaces », vrai seulement une fois ce câblage fait) est à corriger à ce
  moment-là — fichier frontend, hors périmètre backend de ce lot.

---

## [20260813-2] — Batch 1 backend #155/#156 : implémentation B1-B6 + correction NEXT_QUESTION

> Suite du Batch 0 ci-dessous — implémentation backend (`internal/server/websocket.go`, `http.go`,
> `inbound_allowlist.go`, `internal/protocol/messages.go`, `cmd/server/main.go`). Version
> `6.2.0.0`.

- **[FIXED]** `websocket-actions.md` §"Animateur" NEXT_QUESTION, règle de calcul : la formulation
  du Batch 0 (« première question dont STATUS ∉ {...} ») était une approximation qui omettait deux
  comportements réels de `GamePage.jsx`'s `nextUnplayedQuestion` : (1) aucune question courante ⇒
  payload vide d'office, sans chercher dans la liste ; (2) la recherche se fait **strictement
  après la position de la question courante** dans l'ordre trié, jamais « n'importe laquelle de
  disponible dans la liste entière », et ne boucle jamais vers le début. Corrigé après lecture
  ligne à ligne du JS (implémentation B5, `cmd/server/main.go` `getNextQuestionPayload`). Aucun
  changement de payload ni de déclencheurs, uniquement la précision de la règle documentée.
- **[NEW]** B1-B6 implémentées telles que contractées en Batch 0 : `ClientTypeAnim` +
  `/ws/anim` + `serializeForClientType` (B1) ; `ANIM_COUNT` (B2) ; allow-list `anim` pour
  HELLO/START/STOP/PAUSE/CONTINUE/REVEAL/READY/BUMPER_POINTS/TEAM_POINTS (B3) ; `QUESTIONS` gaté
  admin-only au HELLO (B4) ; `NEXT_QUESTION` (B5) ; log régie `INFO` sur toute action animateur
  acceptée (B6, distinct du `WARN` de rejet #154).
- **Non impacté** : aucun changement de payload par rapport au Batch 0 — uniquement la précision
  de la règle de calcul NEXT_QUESTION et l'implémentation du reste tel que contracté.

---

## [20260813] — Interface Animateur — socle #155 + SPEEDY #156 (Batch 0, contrats)

> Milestone v6.2.x — Interface Animateur (#24), cible **6.2.0.0**. Batch 0 (contrats, avant tout
> code) du plan `_work/reports/plan-20260813-092950.md` (rév. 1) et
> `_work/reports/plan-20260813-094321.md` (rév. 2 — #156 SPEEDY intégrée au même lot). #154
> (allow-list entrante, `IncomingMessage.ClientType`, sérialisation par type) est le socle
> technique direct de ce lot — aucune plomberie n'est recréée, uniquement étendue.

- **[BREAKING]** Disparition de l'alias SPA `/anim` → `/admin`. La route `/anim` cesse de servir
  `GamePage` (pleins droits régie) et sert désormais la nouvelle page animateur (`AnimPage.jsx`,
  #155 F4), connectée sur le nouvel endpoint `/ws/anim` avec le `ClientType` réduit `anim`.
  `/anim/quiz`, `/anim/settings`, `/anim/logs` (sous-routes admin dupliquées sous ce préfixe) ne
  résolvent donc plus vers la régie. `/admin/*` est strictement inchangée. Voir
  `contracts/websocket-endpoints.md` §"Endpoints WebSocket" et le détail frontend F2 du plan.
- **[NEW]** Endpoint `/ws/anim` (`ClientType` `"anim"`, D1) — `contracts/websocket-endpoints.md`.
- **[NEW]** `ClientTypeAnim` ajouté à `serializeForClientType` (`internal/server/websocket.go`) →
  route vers `SerializeForWebClient` (même payload que TV/VPlayer, aucun sérialiseur dédié créé) —
  décision et justification complètes dans `contracts/ws-payload-serialization.md`
  §"Animateur". ⚠️ Piège identifié par le plan (§0.2) : le `default:` de cette fonction retourne
  aujourd'hui `SerializeForAdmin()` — un type non explicitement ajouté au `switch` reçoit le
  payload admin complet par défaut. Première ligne de code du lot (tâche B1), avec test dédié.
- **[NEW]** Action `NEXT_QUESTION` (Server→Client, exclusive à `ClientTypeAnim`) — question
  suivante calculée côté App (règle identique à `GamePage.jsx:210-220`, tri `ORDER`/`ID` puis
  premier statut jouable), diffusée sur événements explicites (jamais sur le chemin de
  `broadcastUpdate`, pour éviter une lecture disque par tick). Détail complet
  (payload, déclencheurs, règle de calcul) : `contracts/websocket-actions.md` §"Animateur".
- **[NEW]** Lignes `anim` de l'allow-list entrante (#154) : `HELLO` ; `START`/`STOP`/`PAUSE`/
  `CONTINUE`/`REVEAL`/`READY` ; `BUMPER_POINTS`/`TEAM_POINTS` — périmètre "conduite en direct"
  du cadrage (`_work/reports/plan-20260812-141735.md` §1). Ajout d'entrées uniquement dans
  `internal/server/inbound_allowlist.go`, aucun changement de mécanisme (`IsActionAllowed`,
  `IsSetClientTypeAllowed` inchangées). Détail : `contracts/websocket-actions.md` §"Sécurité —
  Allow-list entrante par ClientType".
- **[NEW]** Champ `ANIM_COUNT` dans `ClientsPayload` (`internal/protocol/messages.go`) — nombre
  de clients animateur connectés, affiché en badge Navbar régie (D2, frontend F3). `CLIENTS`
  reste diffusé à `admin` seul, comportement inchangé.
- **[CHANGED]** `sendStateToClient` (HELLO) : le bloc `QUESTIONS` (`cmd/server/main.go`), envoyé
  jusqu'ici sans condition de type, est gaté `admin`-only — alignement sur `broadcastQuestions`,
  déjà `admin`-only en continu. **Effet de bord voulu** : TV et VPlayer cessent de recevoir
  `QUESTIONS` au HELLO (ils ne le recevaient déjà plus ensuite). `CONFIG_UPDATE` reste gaté
  Admin+TV (politique #154 E1 inchangée) — `anim` en est donc exclu par construction, sans
  modification supplémentaire. Détail : `contracts/websocket-endpoints.md` §"Filtres de diffusion
  par type".
- **[FIXED]** (à l'occasion de cette relecture) `contracts/websocket-endpoints.md` §"Filtres de
  diffusion par type" indiquait `CONFIG_UPDATE` reçu par VPlayer (✓) — inexact depuis #154
  (v6.1.4) : `sendStateToClient` restreint `CONFIG_UPDATE` à Admin+TV. Corrigé.
- **Non impacté par ce Batch (contrats seuls, aucun code)** : les endpoints/actions existants ne
  changent pas de comportement pour `admin`/`tv`/`vplayer`/`buzzer`. Le mécanisme d'allow-list
  (#154) n'est pas modifié, uniquement étendu par de nouvelles entrées.

---

## [20260812] — Allow-list entrante WebSocket par ClientType (#154, sec)

> Vulnérabilité préexistante découverte par `planner` pendant le cadrage de l'Interface
> Animateur (#110) : le serveur distinguait déjà les types de client WebSocket (`admin`/`tv`/
> `vplayer`) en **sortie** (sérialiseurs `SerializeForAdmin`/`SerializeForWebClient`/
> `SerializeForVPlayer`), mais le dispatch **entrant** (`handleWebMessage`) ne consultait jamais
> le type du client émetteur — un client connecté sur `/ws/tv` ou `/ws/player` pouvait envoyer
> n'importe quelle action (START/STOP/RAZ/DELETE/NEW_GAME/BUMPER_POINTS...) et le serveur
> l'exécutait comme si un admin l'avait envoyée. **Non-BREAKING** pour tout client legitime
> (admin/tv/vplayer n'envoyant que les actions déjà documentées pour son propre rôle) ; **BREAKING**
> uniquement pour un client hors-contrat qui dépendait de ce trou (aucun cas légitime identifié).

- **[NEW]** `internal/server/inbound_allowlist.go` — `IsActionAllowed(action, clientType)` (map
  statique action → types autorisés, deny-by-default) et `IsSetClientTypeAllowed(currentType)`
  (SET_CLIENT_TYPE a sa propre règle : dépend du type COURANT, pas d'une liste fixe). Voir
  `contracts/websocket-actions.md` §"Sécurité — Allow-list entrante par ClientType" pour la table
  complète.
- **[NEW]** `protocol.IncomingMessage.ClientType` (string) — porte le type du client émetteur,
  peuplé par `websocket.go`'s `readPump` directement depuis `WebSocketClient.Type` (pas de lookup
  par `ClientID` a posteriori : évite la fenêtre de course documentée sur `h.register <-`).
- **[CHANGED]** `cmd/server/main.go` `handleWebMessage` — rejette (log `WARN`, aucun effet de
  bord) toute action hors allow-list avant dispatch.
- **[CHANGED]** `handleSetClientType` — n'accepte plus SET_CLIENT_TYPE que d'un client dont le
  type courant est `admin` (ferme l'auto-promotion en admin qu'un type inconnu déclenchait
  silencieusement avant, E3).
- **[FIXED]** E1 — `sendStateToClient` (HELLO) sérialise désormais par type (`SerializeForAdmin`
  vs `SerializeForWebClient`) au lieu du payload admin complet non filtré ; `CONFIG_UPDATE` n'est
  plus envoyé qu'à `admin`/`tv` à la connexion (aligné sur la politique déjà appliquée par
  `broadcastConfigUpdate` en continu).
- **[FIXED]** E2 — `WebSocketHub.GetClientCounts` : un type de client non reconnu ne compte plus
  par défaut comme admin.
- **[FIXED]** E4 — `WebSocketHub.BroadcastToTypes` sérialise désormais une fois par type demandé
  (au lieu d'une fois globalement) — sans effet observable aujourd'hui (aucun sérialiseur ne
  réduit de contenu hors `ActionUpdate`), mais ferme l'incohérence structurelle pour une future
  action sensible au type (réutilisable par #155).
- **Non impacté** : aucun endpoint HTTP, aucun format de payload existant (seuls certains clients
  cessent de RECEVOIR ou d'ÉMETTRE certains messages — le contenu des messages qu'ils reçoivent
  légitimement est inchangé).

### Corrections post-revue (v6.1.4.1, même cycle)

> `code-reviewer` a refusé la première livraison (2 critiques). Corrections ciblées, pas de
> reprise de conception — voir `_work/reports/code-reviewer-20260812-160150.md`.

- **[FIXED]** BUTTON et PONG étaient classées admin-only par erreur — ce sont en réalité les
  actions de gameplay réel du VJoueur (`VPlayerPage.jsx` : PONG = handshake de disponibilité en
  PREPARE, BUTTON = l'appui sur le buzzer). Corrigé en `{admin, vplayer}` dans
  `inbound_allowlist.go` ; table `websocket-actions.md` mise à jour. L'audit frontend initial
  s'appuyait sur les noms de fonctions wrapper (`simulateButton`/`simulatePong`) et avait manqué
  l'appel direct `sendMessage('BUTTON'/'PONG', ...)` de `VPlayerPage.jsx`, hors de tout wrapper
  nommé — méthodologie corrigée pour un grep de la chaîne d'action littérale.
- **[FIXED]** Race de données confirmée sur `WebSocketClient.Type` (même champ, même fichier que
  #133) : `readPump` lisait `c.Type` sans protection alors que `SetClientType` l'écrit sous
  verrou depuis la goroutine de dispatch. Nouvelle méthode `(*WebSocketClient) TypeSnapshot()`
  (verrou partagé), utilisée pour les deux lectures (`IncomingMessage.ClientType` et la garde
  CANCEL_AI_GENERATION). Portée réelle limitée à la transition `admin → tv/vplayer` sur `/ws`
  legacy (maintien transitoire d'un privilège déjà détenu, pas une élévation) mais race réelle,
  reproduite et vérifiée sous `-race`.
- **[FIXED]** (mineur) Rejet silencieux de CANCEL_AI_GENERATION pour un client non-admin dans
  `readPump` — ajout d'un `LogWarn` symétrique à celui de `handleWebMessage`.

---

## [20260811] — Persistance des métadonnées quiz (#141)

> `GameState` (nom/thème/notes de quiz, publics visés, difficultés, langue, objectif, champs
> masqués sur la TV, plafond de joueurs virtuels) ne survivait à aucun redémarrage serveur —
> constat de `contracts/game-state.md` (v6.1.0) devenu faux par ce changement. **Non-BREAKING** :
> fichier nouveau, aucun changement de protocole WebSocket ni d'endpoint HTTP existant.

- **[NEW]** `data/config/game_state.json` — sous-ensemble persisté de `GameState` (métadonnées
  quiz + plafond de joueurs virtuels), enveloppe versionnée (`format_version`, **premier fichier
  versionné du projet**). Voir `contracts/game-state.md` §"Persistance — game_state.json" pour le
  détail complet (champs inclus/exclus, séquence de démarrage, sémantique NEW_GAME inchangée).
- **[NEW]** `internal/game/state_persistence.go` — `SetStatePath`/`SaveState`/`LoadState`/
  `ClearQuizMeta`, écriture atomique (motif `SaveTeams`/`SaveBumpers`).
- **[CHANGED]** `contracts/game-state.md` §Migration — la non-persistance du `GameState` décrite
  pour v6.1.0 n'est plus vraie ; section réécrite.
- **[CHANGED]** `contracts/http-endpoints.md` — `game_state.json` intégré au flag `history` de
  `/backup-select`/`/reset-select` (même rattachement que `game-config.json`, #150) et à la
  détection par contenu de `/restore`.
- **Non impacté** : aucun payload WebSocket, aucun endpoint HTTP existant, aucune sémantique
  `NEW_GAME` (règle H5 conservée à l'identique — la persistance ne fait que rendre durable un
  comportement déjà vrai en mémoire).

---

## [20260811] — Séparation config système / config de jeu (#150)

> `config.json` mélangeait réglages système (clés API, WiFi, ports) et réglages de jeu
> (délai par défaut, effet néon) dans un seul fichier/endpoint, empêchant de sauvegarder ou
> restaurer les réglages de jeu avec une partie sans embarquer aussi les secrets. Option (b)
> actée par arbitrage utilisateur : scission en deux fichiers/endpoints.
> **[BREAKING]** — migration automatique et idempotente au démarrage, voir
> `contracts/http-endpoints.md` §Migration.

- **[BREAKING]** `GET/POST /config.json` — les sections `game` et `neon_effect` ne sont plus
  acceptées ni retournées ; `POST /config.json` portant encore l'une de ces sections est rejeté
  en `400` avec un message nommant le nouvel endpoint.
- **[BREAKING]** `server-go/config.json` — les sections `game` et `neon_effect` sont déplacées
  vers `data/config/game-config.json` ; migration automatique et idempotente au démarrage
  (ancien format sans `game-config.json` → extraction + migration + journalisation ; les deux
  présents → `game-config.json` fait autorité, sections résiduelles de `config.json`
  supprimées avec avertissement ; aucun des deux → défauts).
- **[NEW]** `GET/POST /game-config.json` — réglages de jeu (délai par défaut, effet néon),
  même sémantique de fusion additive par section que `/config.json`.
- **[NEW]** `data/config/game-config.json` — fichier de réglages de jeu, inclus au
  backup/restore/reset sélectifs (rattaché au flag `history`, voir
  `contracts/http-endpoints.md`) et à la sauvegarde complète (`/fs-backup`, qui archive tout
  `dataDir` sans changement de code).
- **[CHANGED]** `contracts/http-endpoints.md` — correction de la description de
  `GET /game-backup` : archive `dataDir/files` (questions + médias), **pas** la configuration
  — la description précédente était inversée par rapport au code (divergence contrat/code
  détectée en marge de #150, sans lien fonctionnel avec la scission elle-même).
- **Non impacté** : le payload WebSocket `neon_effect` (`CONFIG_UPDATE`,
  `contracts/websocket-actions.md`) est inchangé — seule sa source de lecture, côté serveur,
  change (`config.GetGameSettings()` au lieu de `config.Get()`). Le BREAKING est strictement
  confiné à la surface HTTP et au format des fichiers sur disque.

---

## [20260809] — Validation de clé API par appel réel au fournisseur

> Une clé bien formée mais révoquée/tronquée était acceptée sur le seul contrôle de préfixe
> (`sk-ant-`/`gsk_`) et n'échouait qu'à la première génération, très loin du geste qui l'a
> causée. La validation passe au moment de l'enregistrement, par un appel réel au fournisseur.
> **Aucun [BREAKING]** — endpoint nouveau, champs de config additifs, chemin de génération
> strictement inchangé.

- **[NEW]** `contracts/ai-key-validation.md` — contrat complet (appel de validation, taxonomie
  du résultat, séquence d'enregistrement, sécurité).
- **[NEW]** `POST /api/ai/validate-key` — valide une clé auprès de son fournisseur **sans rien
  persister**. Corps `{provider, api_key?}` ; `api_key` vide ⇒ valide la clé effective stockée
  (variable d'environnement incluse). Réponse `200` portant le verdict dans le corps :
  `result: "valid" | "invalid_key" | "unreachable"`, `http_status`, `detail` assaini.
  `400`/`405`/`413`/`429` réservés aux erreurs de la requête elle-même — **jamais** de `5xx`
  pour un échec fournisseur.
- **[NEW]** `aiProvider.ValidateKey(ctx, cfg, key)` — quatrième méthode de l'abstraction #137,
  qui ne disposait d'**aucun** mécanisme de vérification de connectivité. Appel `GET /v1/models`
  (Anthropic) / `GET /openai/v1/models` (Groq) : coût nul en tokens, préserve le budget 8 000 TPM
  de Groq, isole strictement l'authentification. Timeout dédié **10 s** (et non les 300 s de
  `ai.timeout_seconds`, inutilisables en interactif). Aucune signature existante modifiée.
- **[NEW]** `ai.anthropic_api_key_verified` / `ai.groq_api_key_verified` (bool, persistés,
  retournés par `GET /config.json` — ce ne sont pas des secrets). `true` après enregistrement
  suivi d'une validation réussie, `false` si enregistrement forcé ou clé effacée. Persistés et
  non dérivés : sans cela le badge « ✅ Clé configurée » réapparaîtrait après rechargement pour
  une clé enregistrée de force malgré un refus.
- **[CHANGED]** `contracts/ai-generation.md` §2 / `ai-multi-provider.md` §8 — le contrôle de
  préfixe reste en place, désormais appliqué **en amont** de la validation réseau.
  Sémantique de `POST /config.json` inchangée (les deux nouveaux champs suivent la fusion
  champ-par-champ existante de la section `ai`).

---

## [20260807b] — Fix : schéma JSON rejeté par Groq, discriminator ambigu (issue #142, v6.1.1)

> Bug bloquant confirmé et corrigé — 100% des générations réelles via Groq échouaient,
> quels que soient les paramètres. Cause racine confirmée par appel réel à l'API Groq
> (2026-08-07), pas une hypothèse : voir `ai-multi-provider.md` §7 (amendé) pour le mécanisme
> exact. **Aucun [BREAKING]** — comportement fonctionnel inchangé, seule la structure du schéma
> envoyé à Groq change, et une vérification serveur compense la restriction perdue.

- **[CHANGED]** `contracts/ai-multi-provider.md` §7 — remplace la spéculation précédente
  (`MOTION_CARDS.DIFFICULTY` entier, jamais le vrai problème) par le mécanisme confirmé :
  Groq compte comme candidat discriminant toute propriété `required` + `enum`/`const` **commune
  à toutes les branches** de l'`anyOf`, sans vérifier si l'ensemble de valeurs varie réellement.
  `CATEGORY`/`DIFFICULTY` (identiques dans les 5 branches) et `TYPE` (const distinct) étaient
  candidats ensemble → rejet. `groqProvider.AdaptSchema` retire `enum` de `CATEGORY`/`DIFFICULTY`
  pour Groq uniquement ; `TYPE` devient l'unique discriminant.
- **[CHANGED]** `contracts/ai-generation.md` §5.1 — nouvelle vérification serveur : `DIFFICULTY`
  doit appartenir aux `difficulties` de la requête (compense la restriction `enum` perdue côté
  schéma Groq ; `CATEGORY` avait déjà cette vérification, pas `DIFFICULTY`). Appliquée aux deux
  providers par cohérence, sans effet observable sur Anthropic (son schéma garde l'`enum`).
- Confirmé sans régression pour Anthropic (chemin `AdaptSchema` séparé, inchangé) et validé par
  appel réel pour les 5 types générables (SPEEDY/QCM/MEMORY/MEMOTION/ARDOISE).

---

## [20260807] — Verbosité des erreurs de génération IA (issue #142, v6.1.1)

> Retour utilisateur QUALIF : les erreurs de génération IA n'étaient visibles côté admin que via
> un code stable générique (`ERROR_CODE`) — le détail réel (ex. schéma JSON rejeté par Groq,
> issue #142) n'était accessible qu'en ajoutant une trace de diagnostic temporaire dans le code,
> retirée ensuite. **Aucun [BREAKING]** — champ additif uniquement.

- **[NEW]** `AI_GENERATION_PROGRESS.ERROR_MESSAGE` (string) — détail lisible de l'erreur,
  **assaini**, présent uniquement quand `STATE = "FAILED"`. Contenu : le message d'erreur réel du
  provider (`.error.message` Anthropic/Groq) quand disponible, sinon le message générique
  précédent en repli. `/ws/admin` uniquement, comme le reste de l'action.
- **[CHANGED]** `contracts/ai-generation.md` §8 S2 — la règle « le corps d'erreur du provider
  n'est pas relayé tel quel » est amendée : il **peut** désormais être relayé, assaini
  (`server.sanitizeUpstreamMessage` : substrings au format d'une clé API connue remplacées par
  `[redacted]`, message tronqué à 500 caractères) et uniquement via le canal admin ci-dessus —
  jamais une réponse HTTP synchrone, jamais `/ws/tv`/`/ws/player`/`/ws/buzzer`. La règle sur la
  clé elle-même est inchangée : elle n'apparaît dans aucun corps de réponse provider en premier
  lieu (transmise seulement via l'en-tête `Authorization`) ; le filtre est une défense en
  profondeur, pas le mécanisme garantissant son absence.

---

## [20260806b] — Visibilité TV par champ (#137 Batch 2b, v6.1.0)

> Complément **strictement additif** à l'entrée `[20260806]` ci-dessous, demandé après
> présentation de la maquette : l'animateur choisit, champ par champ, ce qui est annoncé sur
> l'écran TV NEW_GAME. **Aucun [BREAKING]** — rien de ce qui est déjà implémenté n'est invalidé.
> Rapport : `_work/reports/planner-20260806-152640-tv-visibility.md`.

- **[NEW]** `GameState.QUIZ_HIDDEN_FIELDS` (string[]) — champs **non** affichés sur l'écran TV
  NEW_GAME, valeurs ⊂ `THEME` / `POPULATIONS` / `DIFFICULTIES` / `LANGUAGE`. Liste vide (défaut)
  = tout est affiché. Sérialisé toujours, jamais `null`.
  *Forme choisie contre quatre booléens `QUIZ_DISPLAY_*` : le défaut voulu étant « affiché », des
  booléens auraient exigé de forcer `true` à chaque construction/réinitialisation du state — un
  chemin oublié masquant silencieusement des champs. Avec une liste, le zéro Go est le
  comportement correct.*
- **[NEW]** `UPDATE_QUIZ_META.HIDDEN_FIELDS` (string[]) — même sémantique « absent = inchangé »
  que les autres champs ; `[]` présent = tout réafficher. Une valeur inconnue est **ignorée et
  journalisée**, jamais une erreur : un client plus récent ne doit pas faire échouer un
  enregistrement entier sur un libellé qu'un serveur plus ancien ne connaît pas.
- **[CHANGED]** `contracts/ws-payload-serialization.md` — `QUIZ_HIDDEN_FIELDS` est transmis à la
  TV (elle en a besoin pour appliquer la préférence). **Distinction posée explicitement** :
  confidentialité ⇒ retrait côté serveur (`QUIZ_OBJECTIVES`) ; préférence d'affichage ⇒
  application côté client (`QUIZ_HIDDEN_FIELDS`). Les valeurs masquées restent donc présentes
  dans le payload TV — ce ne sont pas des secrets.
- `OBJECTIVES` n'est **pas** une valeur acceptable de `HIDDEN_FIELDS` : il n'est jamais diffusé,
  l'y admettre laisserait croire qu'il pourrait l'être. `QUIZ_NAME` et `QUIZ_NOTES` ne sont pas
  pilotables dans cette version (extension future = une valeur d'énumération de plus).

---

## [20260806] — Publics/difficultés multiples, objectif global, ARDOISE générable (#137 Batch 2a+2b, v6.1.0)

> Retour QUALIF utilisateur sur #137 : les réglages globaux de la partie (thème, publics,
> difficultés, langue, objectif) ne doivent exister **qu'à un seul endroit** — la section Quiz —
> le popup de génération IA n'en proposant qu'un rappel en lecture seule et des précisions
> propres à la génération.
> Audit de gap : `_work/reports/planner-20260806-143248.md`.
> Arbitrages utilisateur : `_work/handoff/task-planner-contracts-20260806-144240.md`.

- **[BREAKING]** `GameState` — `QUIZ_POPULATION` (string) → **`QUIZ_POPULATIONS`** (string[]),
  `QUIZ_DIFFICULTY` (string) → **`QUIZ_DIFFICULTIES`** (string[]). Remplacement franc **sans
  champ dérivé de compatibilité** (choix utilisateur explicite, contre l'option additive
  proposée par l'audit). Les deux tableaux sont sérialisés `[]` et **jamais `null`**.
  *Aucune migration de fichier n'est requise* : `GameState` n'est ni persisté ni rechargé
  (vérifié — `cmd/server/main.go:205-211` ne persiste que history/teams/bumpers/statuses).
  Le risque est côté **clients non rechargés** après déploiement, cf. `game-state.md` § Migration.
- **[BREAKING]** `UPDATE_QUIZ_META` — `POPULATION`/`DIFFICULTY` (singuliers) remplacés par
  `POPULATIONS`/`DIFFICULTIES` (tableaux). Un client de v6.0.0 voit ces deux champs **ignorés**
  (valeurs courantes préservées, pas de corruption) : son enregistrement est partiellement sans
  effet. Aucun repli sur les anciens noms — un client non redéployé doit être visible.
- **[BREAKING]** `POST /api/generate-questions` — `population` (string) → **`populations`**
  (string[], ≥ 1 élément). Un appelant de v6.0.0 reçoit `400 invalid_request`.
- **[NEW]** `GameState.QUIZ_OBJECTIVES` (string, texte libre) — objectif de la partie.
  **Premier champ du nœud `GAME` à diffusion restreinte** : retiré du payload `/ws/tv` et
  `/ws/player`, conservé sur `/ws/admin`. Le retrait doit être appliqué aux **trois** sites de
  sérialisation TV/VPlayer via une liste partagée, sur le modèle de `AdminOnlyBumperFields`
  (`ws-payload-serialization.md`). Ne pas confondre avec `QUIZ_NOTES`, qui reste **affiché aux
  joueurs**.
- **[NEW]** `UPDATE_QUIZ_META.OBJECTIVES` (string ≤ 2000 car.) — sémantique « absent = inchangé »
  comme les autres champs ; `""` présent = effacement explicite.
- **[NEW]** `POST /api/generate-questions` — champ `objectives` (string, optionnel, ≤ 2000 car.),
  **distinct de `instructions`** : l'un est l'objectif persisté de la partie, l'autre les
  précisions non persistées de cette génération.
- **[CHANGED]** Prompt de génération — ordre d'injection désormais normatif : objectif global
  (« Objectif de la partie : … ») **avant** les précisions (« Précisions pour cette
  génération : … »), et `Publics cibles` au pluriel. Sans libellés distincts, le modèle reçoit
  deux consignes concurrentes indiscernables (`ai-generation.md` §4).
- **[CHANGED]** `difficulties` (`POST /api/generate-questions`) — inchangé en **forme** (déjà un
  tableau depuis v6.0.0), mais devient la seule source : le frontend n'enveloppe plus une valeur
  globale unique dans un tableau à un élément.
- **[CHANGED]** Cardinalité des énumérations (`ai-generation.md` §6) — `QUIZ_POPULATIONS` et
  `QUIZ_DIFFICULTIES` passent de 1 à N valeurs ; listes de valeurs **inchangées**.
  `QUIZ_LANGUAGE` reste à valeur unique. Rendu UI imposé : chips multi-sélection.
- **[NEW]** *(Batch 2a, livré — entrée manquante rétablie ici)* `ARDOISE` devient un **type
  générable** par l'IA, au même rang que SPEEDY/QCM/MEMORY/MEMOTION (`ai-generation.md` §5, §6).
  Structurellement `ARDOISE = SPEEDY + ARDOISE_KEYBOARD_TYPE`. Le **modèle choisit lui-même**
  le clavier (`NUMPAD` si la réponse est purement numérique, `AZERTY` sinon) — consigne en prose,
  le schéma ne pouvant pas exprimer une règle dépendant du contenu d'un autre champ. Désactivé
  à 0 % par défaut dans la répartition, comme MEMOTION, pour ne pas redistribuer silencieusement
  les pourcentages des types existants.

---

## [20260805b] — Provider gratuit Groq + génération par lots (#137, v6.1.0)

> Ajout de **Groq** (`openai/gpt-oss-120b`, tier gratuit) comme second provider BYOK à côté de
> Claude. Le tier gratuit impose **8 000 tokens/minute**, ce qui rend impossible le modèle
> « 200 questions en un appel » de #8 : la génération devient une **suite de lots séquentiels
> avec reprise**, exécutée en **tâche de fond** avec progression WebSocket.
> Plan : `_work/reports/planner-20260805-204318-plan-137.md`.
> Maquette : `_work/mockups/137-generation-tache-de-fond.md`.
> Contrat détaillé : `contracts/ai-multi-provider.md`.
> Recherche providers : `_work/reports/planner-20260805-203550-providers-137.md`.

- **[BREAKING]** `POST /api/generate-questions` — ne renvoie plus le résultat. Répond désormais
  `202 Accepted` avec `{job_id, batches_total}` ; le résultat et les erreurs transitent par
  l'action WebSocket `AI_GENERATION_PROGRESS`. Les codes `502` / `504` ne sont plus émis par cet
  endpoint. Nouveau `409 generation_in_progress` — **un seul job à la fois**, tout admin confondu.
  *Seul changement réellement cassant de cette entrée : tout appelant de #8 doit être adapté.*
- **[NEW]** `AI_GENERATION_PROGRESS` (Server→Client, `/ws/admin` **uniquement**) —
  `{JOB_ID, STATE, BATCHES_DONE, BATCHES_TOTAL, CREATED_COUNT, SKIPPED_COUNT, ERROR_CODE,
  PROVIDER}`. Émis après chaque lot — **après** le broadcast des questions du lot — et à la
  connexion d'un client admin si un job est en cours, pour qu'un rechargement de page retrouve
  la progression sans état à reconstituer côté client.
- **[NEW]** `CANCEL_AI_GENERATION` (Client→Server) — `{JOB_ID}`. Prend effet **entre deux
  lots**, jamais au milieu d'un appel provider. Les questions déjà écrites sont **conservées**.
- **[NEW]** Code d'erreur `provider_quota` — quota journalier du fournisseur épuisé
  (Groq : 1 000 requêtes/jour, 200 000 tokens/jour).
- **[NEW]** Champs `AIConfig` : `provider` (`anthropic` | `groq`, défaut `anthropic`),
  `groq_api_key`, `groq_model` (défaut `openai/gpt-oss-120b`), `batch_size` (20),
  `inter_batch_delay_ms` (60000), `context_token_budget` (1500), `max_consecutive_failures` (2).
  **Les défauts de découpage et de cadencement sont provisoires** : Groq ne documente ni ce que
  compte son TPM ni le sort d'une requête qui le dépasse, ces valeurs sont à calibrer
  empiriquement (tâche T0.1 du plan).
- **[NEW]** La clé Groq suit **exactement** les règles de secret de la clé Anthropic
  (`ai-generation.md` §2) : jamais renvoyée en `GET`, booléen dérivé `groq_api_key_configured`,
  valeur vide en `POST` = préserver, effacement explicite via `clear_groq_api_key`.
- **[CHANGED]** Génération découpée en lots — **s'applique aussi au chemin Anthropic**. Chaque
  lot est validé, écrit et broadcasté avant l'appel suivant ; un lot en échec n'annule pas les
  précédents. Supprime le mode d'échec « troncature = lot entier perdu » : sous décodage
  contraint, une troncature en milieu de tableau rendait auparavant **zéro** question.
- **[CHANGED]** Contexte anti-doublon désormais **budgété en tokens** et dépendant du provider
  (Anthropic : 150 questions comme avant ; Groq : ~1 500 tokens, soit ~25 questions), enrichi à
  chaque lot des questions produites dans le job courant.
- **[CHANGED]** Interface `aiProvider` (3 méthodes) sur le point d'appel unique
  `ai_generator.go:781`. Le chemin Anthropic devient une implémentation à comportement
  identique. Abstraction délibérément minimale — elle ne préjuge d'aucune architecture à
  plugins et n'en interdit aucune.
- **[CHANGED]** `POST /config.json`, section `ai` — amendement de `ai-generation.md` §0
  (§0bis) : cette section n'est plus **remplacée intégralement** comme les autres sections
  de `config.json`, elle est désormais **fusionnée champ à champ** — une clé JSON absente du
  payload préserve la valeur stockée, une clé présente (même à sa valeur zéro Go, ex.
  `batch_size: 0`) l'écrase. Corrige un bug bloquant trouvé en QA sur #137 : `ConfigPage.jsx`
  sauvegarde `provider`/la clé API/les réglages de batching via des boutons séparés à payload
  partiel ; le remplacement intégral réinitialisait silencieusement tout champ absent du
  payload (ex. sauvegarder la clé Groq seule repassait `provider` à `"anthropic"`). Corrigé
  côté frontend (`ConfigPage.jsx` envoie désormais la section `ai` complète à chaque
  sauvegarde) **et** côté backend (fusion champ à champ, généralisant la sémantique déjà
  appliquée aux deux clés secrètes) — les deux fixes sont compatibles et cumulés en défense
  en profondeur.
- **[CHANGED]** Prompt de génération — consigne dédiée pour `MEMOTION` (contrainte « 4 à 12
  cartes » explicitée en prose, définition des 3 champs `RECTO_THEME`/`QUESTION_TEXT`/
  `ANSWER_TEXT`, exemple few-shot), gatée sur `distribution["MEMOTION"] > 0`. Corrige un taux de
  production réel mesuré à 2 % (3/152) contre 15 % demandé (`qa-20260806-111416.md` §5.4) — le
  schéma de sortie n'a pas de `minItems`/`maxItems`, rien n'empêchait structurellement le modèle
  de produire moins de 4 cartes. Vérifié en conditions réelles par `qa` après fix : 100 %
  (10/10) sur deux échantillons Groq.
- **[NEW]** `ARDOISE` rejoint les types générables (5ᵉ variante du schéma) — voir l'amendement
  détaillé dans `ai-generation.md` §5/§6 et `ai-multi-provider.md` §13 : la spec d'origine de #8
  le décrivait à tort comme un mode d'affichage plutôt qu'un type de contenu. Champs additionnels
  `ANSWER` + `ARDOISE_KEYBOARD_TYPE` (`enum` `AZERTY`\|`NUMPAD`, choisi par le LLM selon la
  réponse — `NUMPAD` si purement numérique). `POINTS_TARGET = TEAM` (cohérent avec la donnée
  existante). Désactivé à 0 % par défaut dans la répartition, comme `MEMOTION`.

**Un seul changement BREAKING**, isolé et identifié ci-dessus. **Deux points appellent une
validation explicite de l'utilisateur au GATE 2** :
1. **Le comportement du produit change** — 200 questions passent de 1-3 min (Claude, payant) à
   ~10 min (Groq, gratuit), avec une modale non bloquante et des questions qui apparaissent au
   fur et à mesure. Ce n'est pas un détail d'implémentation.
2. **La qualité du français de `gpt-oss-120b` n'est pas mesurée** — aucun fournisseur ne publie
   cette donnée. Le test amont ayant été écarté, le contrôle est déplacé en aval : lot réel joint
   au handoff de `dev-backend`, puis relecture humaine obligatoire en QA.

---

## [20260805] — Générateur de questions via IA (#8, v6.0.0)

> Bouton « ✨ Générer via IA » dans QuestionsPage : le backend appelle l'API Claude (BYOK) en
> sortie structurée et écrit directement de nouvelles questions, **en création uniquement**.
> Plan : `_work/reports/planner-20260805-121900.md`.
> Maquette : `_work/mockups/8-generateur-ia.md`.
> Contrat détaillé : `contracts/ai-generation.md`.

- **[CHANGED]** `POST /config.json` — **correctif de bug destructif, à livrer en premier.** Le
  handler désérialisait le corps dans un `config.Config` **vide** puis réécrivait le fichier
  entier ; aucune section ne portant `omitempty`, toute section absente du payload était remise
  à zéro. ConfigPage n'envoyant que des payloads partiels (`{neon_effect}`, `{server}`), chaque
  « Enregistrer » détruisait les autres sections. Le handler devient **additif** : merge sur
  `config.Get()`, ré-application des défauts, écriture atomique. Motif déjà en place dans
  `handleAPIWiFiDefaults`.
  *Dégât déjà constaté en production : `config.json` porte `questions_dir: ""` et
  `files_dir: ""`, compensés par un chemin codé en dur dans `main.go`.*
- **[NEW]** `POST /api/generate-questions` — génération par lot. Réponse **synchrone longue**
  (1–3 min). Codes stables : `200` (`created[]`, `skipped_count`), `400`, `405`, `409`
  (pas de clé), `502` (erreur amont Anthropic), `504` (timeout), `507` (plus d'ID libre).
  Déclenche obligatoirement `OnQuestionUpload()` → `broadcastQuestions()`.
- **[NEW]** Section `ai` dans `config.json` : `anthropic_api_key`, `model`
  (défaut `claude-opus-5`), `timeout_seconds` (300), `max_questions` (200).
- **[NEW]** Règle de contrat sur le secret : `GET /config.json` renvoie **toujours**
  `ai.anthropic_api_key: ""` + un booléen dérivé `ai.api_key_configured`. En `POST`, une clé
  absente ou vide **préserve** la valeur stockée ; l'effacement explicite passe par
  `ai.clear_api_key: true`. Le serveur n'ayant aucune authentification, renvoyer la clé en clair
  l'exposerait à tout le LAN.
- **[CHANGED]** `UPDATE_QUIZ_META` — payload étendu de 3 à 6 champs : ajout de `POPULATION`,
  `DIFFICULTY`, `LANGUAGE`. **Additif**, mais assorti d'une règle normative : le handler doit
  appliquer une sémantique **« champ absent = inchangé »** (et non « absent = chaîne vide »).
  Sans elle, un client antérieur envoyant seulement `NAME`/`THEME`/`NOTES` effacerait les trois
  nouveaux champs.
- **[CHANGED]** `GameState` — ajout de `QUIZ_POPULATION`, `QUIZ_DIFFICULTY`, `QUIZ_LANGUAGE`,
  **sans `omitempty`** (règle projet). Affichés sur l'écran TV NEW_GAME, qui passe de 3 à 6
  champs : l'affichage TV étant **statique et sans scroll**, le regroupement compact est imposé
  par la maquette §8.
- **[CHANGED]** `SetQuizMeta` (`internal/game/engine.go:1591`) passe de 3 à 6 paramètres.
  Signature interne, un seul appelant (`main.go:1061`).
- **[CHANGED]** Allocation d'ID de question — `findFreeQuestionID` balayait `1..999` **sans
  verrou** (course pré-existante : deux uploads simultanés pouvaient obtenir le même ID) et
  repliait sur `"999"` en cas de saturation, écrasant la question 999. Désormais : mutex sur
  `HTTPServer`, réservation exclusive par `os.Mkdir`, et `507` en cas de saturation. Le
  correctif s'applique aussi à `handleUploadQuestion`.

**Aucun changement BREAKING au sens strict** — aucune action, aucun champ existant n'est
supprimé, renommé ni retypé ; tous les ajouts sont additifs. **Trois points appellent néanmoins
une validation explicite de l'utilisateur au GATE 2** :
1. le correctif `POST /config.json` **change le comportement observable** d'un endpoint existant
   (une sauvegarde partielle ne réinitialise plus le reste) — c'est l'intention, mais tout
   client qui s'appuyait sur l'effet de remise à zéro changerait de comportement ;
2. la sémantique « absent = inchangé » sur `UPDATE_QUIZ_META` modifie le traitement d'un
   payload déjà émis aujourd'hui par QuestionsPage ;
3. le schéma de sortie du LLM pour **MEMORY et MEMOTION diverge de la spec validée**, qui
   décrivait des structures inexistantes dans le code (cf. `ai-generation.md` §5).

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
