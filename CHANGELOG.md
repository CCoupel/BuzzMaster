# Changelog - BuzzControl

Historique des versions du projet BuzzControl.


## [3.6.1] - 2026-04-22

### Fixed
- **[Firmware BuzzClick]**: OTA échoue quand le serveur tourne sur un port non-standard (fixes #50)
  - L'URL OTA était construite avec le port 80 en dur (`http://IP/api/firmware/...`), causant un échec HTTP quand le serveur écoute sur un autre port (ex: 8080)
  - Le port est désormais pris depuis la connexion active (`serverIP` + `localUdpPort` positionnés par `tryConnectToServer()`) — couvre les trois chemins de découverte : UDP broadcast heartbeat, fallback NVS, et mDNS
  - Chaîne de fallback : port de la connexion active → `server_tcp_port` NVS → URL du message `OTA_UPDATE` (rétrocompatibilité)

---

## [3.6.0] - 2026-04-21

### Fixed
- **[UpdatePage]**: Boutons Télécharger et Appliquer non fonctionnels (fixes #44)
  - CORS corrigé : suppression du header `Access-Control-Allow-Credentials: true` incompatible avec les origines wildcard
  - Codes HTTP sémantiques sur les endpoints `/api/updates/*` (400/404/503/500) au lieu de 200 systématique
  - Support de la variable d'environnement `GITHUB_TOKEN` pour éviter le rate limiting de l'API GitHub
  - Remplacement atomique du binaire Windows via rename (évite le verrou OS sur l'exécutable en cours)
  - Proxy Vite `/api` ajouté pour le mode développement (`vite.config.js`)
  - Validation anti path-traversal renforcée dans `HandleApplyUpdate`
  - Bouton "Appliquer" désactivé pendant le chargement pour prévenir les double-clics
- **[GamePage]**: Les équipes sans joueur sont masquées sur `/admin/game` (fixes #45)

### Added
- **[Firmware BuzzClick]**: Différenciation visuelle des états d'erreur LED (fixes #49)
  - WiFi échec → rouge clignotant lent (1 Hz)
  - WebSocket déconnectée → rouge clignotant rapide (4 Hz)
  - WebSocket timeout définitif → rouge pulsant lent
  - OTA erreur → rouge + flash blanc
  - Nouveau module `click_ledErrorPatterns.h` — state machine non-bloquante

### Changed
- **[Firmware BuzzClick]**: `CORE_DEBUG_LEVEL` passé de 4 (DEBUG) à 3 (INFO) — les logs mémoire WATCHDOG ne sont plus compilés en production, réduisant l'overhead (fixes #46)
- **[Firmware BuzzClick]**: Logs Broadcaster enrichis avec l'IP source du heartbeat reçu et l'IP locale d'écoute — facilite le diagnostic de découverte serveur (fixes #47)

---

## [3.5.5] - 2026-04-18

### Fixed
- **[Firmware BuzzClick]**: Instruction access fault (MCAUSE=0x1, MEPC=0x00010000) après timeout WebSocket dans la transition NVS→broadcast
  - Generation counter sur les event handlers ESP-IDF pour invalider les callbacks stale après `destroy()`
  - `volatile` sur `wsClient`/`wsConnected`/`wsGeneration` pour visibilité cross-task FreeRTOS
  - Graceful close (`esp_websocket_client_close()` + drain 200ms) avant `destroy()`
  - Fix resource leak si `esp_websocket_client_start()` échoue
  - Defense-in-depth dans `click_WifiManager.h` (ré-assertion NULL + delay 100ms avant retry)

---

## [3.5.4] - 2026-04-18

### Fixed
- **[Game/WebSocket]**: Suppression d'un buzzer non reflétée dans l'UI en temps réel
  - Cause : `GameData.Bumpers` et `GameData.Teams` avaient le tag `omitempty` — une map vide `{}` était omise du JSON broadcasté, empêchant le frontend de recevoir la mise à jour
  - Fix backend : suppression de `omitempty` sur `Teams` et `Bumpers` dans `GameData`, nil-guard dans `GetGameJSON()` (`models.go`, `engine.go`)
  - Fix frontend : checks d'existence explicites dans les handlers WebSocket `UPDATE`, `BUMPER` et `REMOTE` (`useWebSocket.js`)

---

## [3.5.3] - 2026-04-18

### Fixed
- **[ConfigPage]**: Le champ IP Serveur dans la page Config n'est plus pré-rempli avec le hostname du navigateur (`window.location.hostname`). Initialisé à vide (`''`), il est renseigné uniquement si une IP est sauvegardée côté serveur. Depuis v3.2.0, l'IP est optionnelle (découverte automatique via UDP broadcast) — un hostname transmis au firmware causait l'erreur "ERROR:Invalid IP format" dans le handler AT. (`ConfigPage.jsx`, commit `eb8b635`)
- **[Firmware BuzzClick]**: Correction d'un crash Guru Meditation Error (Load access fault) survenant lors d'un timeout de connexion WebSocket. `esp_websocket_client_destroy()` était appelé sans `esp_websocket_client_stop()` préalable, laissant la tâche interne ESP-IDF accéder à un handle détruit. Ajout du `stop()` avant `destroy()` dans le path timeout et nettoyage défensif de tout client existant en début de `connectWebSocket()`. (`click_websocket_espidf.h`, commit `f0bd596`)

---

## [3.5.2] - 2026-04-16

### Fixed
- **[BuzzClick/OTA]**: Handler `OTA_UPDATE` — fallback sur l'URL du message si `server_ip` NVS est vide (cas d'un flash USB complet qui efface le NVS), évite l'échec silencieux de l'OTA après première configuration
- **[Server/resolveServerIP]**: Ajout de logs diagnostics (`LogInfo`) pour tracer l'IP retournée par `GetServerIPs()` — facilite le débogage des problèmes d'IP serveur envoyée aux buzzers
- **[ConfigPage]**: `wifiServerIp` initialisé à `window.location.hostname` au lieu de `''` — l'IP serveur dans `USBConfigModal` est désormais correcte dès l'ouverture même si `config.json` ne contient pas de valeur
- **[TeamCard]**: Ctrl+clic sur le badge firmware ouvre la modale OTA même si `IS_OUTDATED=false` — permet de forcer un reflash sur un buzzer à jour
- **[TeamsPage]**: Même correctif Ctrl+clic que `TeamCard` pour cohérence entre les deux vues buzzer

---

## [3.5.1] - 2026-04-16

### Fixed
- **[Server/WIFI_CONFIG]**: `resolveServerIP()` utilise désormais `server.GetServerIPs()` (IP dynamique) au lieu de `cfg.ServerIP` (valeur statique config.json) — les buzzers reçoivent l'IP réelle du serveur
- **[ConfigPage/USBConfigModal]**: Ajout des props `serverIp` et `serverPort` manquantes dans `wifiConfig` — affichait "(non défini)" au lieu des valeurs réelles
- **[TeamCard]**: Ctrl+clic sur le badge firmware ouvre la modale OTA même si le buzzer n'est pas marqué `IS_OUTDATED`
- **[TeamsPage]**: Même correctif Ctrl+clic que TeamCard pour cohérence entre les deux vues

---

## [3.5.0] - 2026-04-13

### Added
- **[GamePage]**: Bouton "à suivre" affichant la prochaine question non jouée dans le bandeau admin (fixes #39)
  - Format : `à suivre : #ID | catégorie | TYPE | titre` — à droite du badge d'état
  - Visible dans toutes les phases actives, avec deux comportements selon la phase :
    - `STOPPED` / `REVEALED` / `PREPARE` / `READY` : opacité 100%, cliquable
    - `STARTED` / `PAUSED` / `COUNTDOWN` / `ENROLL` : opacité 50%, non cliquable
  - Logique de sélection : première question après la courante avec STATUS différent de `STOPPED`, `REVEALED`, `PLAYED`
  - Au clic : sélectionne directement la prochaine question non jouée
- **[GamePage]**: Badges de phase `COUNTDOWN` et `ENROLL` ajoutés dans le bandeau admin

### Changed
- **[GamePage]**: Opacité 50% sur les questions non jouées dans la liste admin pendant les phases `STARTED` / `PAUSED` / `COUNTDOWN` / `ENROLL`
  - La question courante et les questions jouées restent à pleine opacité

---

## [3.4.5] - 2026-04-10

### Added
- **[GamePage/Memory]**: Sélecteur d'équipes Memory visible en phase PREPARE/READY pour les questions Memory
  - Layout en ligne : équipes sélectionnées à gauche, séparateur `|`, équipes disponibles à droite
  - Mode SOLO (`MEMORY_MODE` vide ou `"SOLO"`) : sélection d'une seule équipe à la fois, chip active colorée sans ×, chips disponibles avec +
  - Mode `CHACUN_SON_TOUR` : label "Chacun son tour", sélection multiple ordonnée avec numéros et ×
  - Mode `TANT_QUE_JE_GAGNE` : label "Tant que je gagne", sélection multiple ordonnée avec numéros et ×

### Fixed
- **[GamePage/Memory]**: Correction du bug d'invisibilité du sélecteur quand `MEMORY_MODE` est absent du payload (`omitempty`) — la valeur vide est désormais traitée comme `"SOLO"`

---

## [3.4.4] - 2026-04-10

### Fixed
- **[LED/BuzzState]**: Implementation correcte des machines a etat LED selon `docs/BUZZER_LED_STATE_MACHINE.md`
  - Ajout du type `BuzzState` (NONE/MOI/EQUIPE/AUTRE) dans `game/models.go`
  - Tracking `bumperBuzzState` par buzzer dans `App` — reset au READY, mis a jour a chaque buzz
  - **Filtre BUTTON** : les messages `BUTTON` sont desormais silencieusement ignores en dehors de la phase `STARTED` (READY, COUNTDOWN, PAUSED, REVEALED, STOPPED)
  - **NORMAL** : STARTED/PAUSED/REVEALED : MOI=BLINK 100%, EQUIPE=SOLID 100%, NONE/AUTRE=DIM 25%
  - **QCM** : STARTED/PAUSED : MOI/EQUIPE=couleur equipe SOLID 100% (cache la reponse), AUTRE/NONE=couleur reponse SOLID 100% ; REVEALED : correct+1er=BLINK, correct+pas1er=SOLID 100%, mauvais/non-buzze=DIM 25%
  - **MEMORY SOLO** : STARTED actif=SOLID 100%, inactif=DIM 25% ; PAUSED toutes=DIM 25%
  - **MEMORY multi-equipes** : STARTED actif=SOLID 100%, prochain=SOLID 50% (INTENSITY=128), autres participants=DIM 25%, non-selectionnes=OFF ; PAUSED toutes=DIM 25%
  - `broadcastContinue` envoie desormais un `LED_SET` apres reprise de jeu (etait manquant)
  - Les intensites sont uniformisees : DIM 25% = intensity 64 (25% de 255)

### Technical
- Remplacement des fonctions LED monolithiques par une architecture par type de jeu (`sendLEDSetForBuzzerNormal/QCM/Memory`)
- `isFirstBuzzTeam` : determine si une equipe est la premiere a avoir buzze via les timestamps engine
- `sendLEDSetForBuzzerQCMReveal` : gestion precise BLINK/SOLID/DIM au REVEALED QCM
- `nextMemoryTeam` : calcul de l'equipe suivante dans la rotation multi-equipes
- 18 nouveaux tests unitaires couvrant toutes les transitions LED

---

## [3.4.0] - 2026-04-10

### Added
- **[LED/Server-Driven]**: Refonte complete du pilotage LED — approche server-driven avec action `LED_SET`
  - Le serveur calcule et envoie l'etat LED exact (couleur, intensite, effet) a chaque changement d'etat pertinent
  - Le firmware applique simplement ce qu'il recoit — aucune logique LED locale cote buzzer
  - Nouveau payload `LED_SET` : `{"COLOR": [R,G,B], "INTENSITY": 0-255, "EFFECT": "SOLID"|"BLINK"|"DIM"}`
  - Suppression des 4 actions QCM-LED precedentes (`QCM_COLOR`, `QCM_DIM`, `QCM_REVEAL`, `QCM_RESET`)
  - Suppression de `manageLeds()` et de toute la machine d'etat LED cote firmware
  - `bumperLEDState` : le serveur memorise le dernier etat LED par buzzer pour le reenvoyer au reconnect (`HELLO`)
  - Couverture : QCM READY/START (couleur reponse SOLID 100%), PAUSE (DIM 25% QCM / 100% buzzer actif + 2% autres en NORMAL), STOP (couleur equipe SOLID 100%), REVEALED (BLINK correct / SOLID wrong / DIM 10% non-buzze), Memory (actif 100% / inactif 25%)
  - Correction du flash visible a chaque `UPDATE_TIMER` : `handleUpdateAction()` ne modifie plus les LEDs

### Changed
- **[Protocol]**: Remplacement de `ActionQCMColor/QCMDim/QCMReveal/QCMReset` par `ActionLEDSet = "LED_SET"`
- **[Firmware BuzzClick]**: `manageLedQCMBlink()` → `manageLedBlink()` (generique, pilote par LED_SET EFFECT=BLINK)

---

## [3.3.3] - 2026-04-09

### Fixed
- **[Updater]**: Correction du seuil `MinBinarySize` et du timeout de telechargement (fixes #38)
  - Le seuil de taille minimale du binaire etait trop eleve, rejetant des binaires valides lors de la mise a jour automatique
  - Le timeout de telechargement etait trop court pour les connexions lentes, causant des echecs d'update

---

## [3.3.2] - 2026-04-09

### Fixed
- **[QCM/Scoring]**: Correction du calcul de penalite QCM — la penalite est desormais basee sur `HINTS_AT_BUZZ` du buzzer individuel (nombre d'indices reveles au moment precis du buzz), et non sur le nombre d'indices actuels du jeu au moment du clic admin
  - Avant : un joueur ayant buzze avant tout indice pouvait etre penalise si l'admin attribuait les points apres qu'un indice ait ete revele
  - Apres : chaque joueur conserve son contexte d'indices au moment de son buzz, independamment de ce qui se passe ensuite
  - Correction dans `handleBumperClick` (clic joueur) et `onTeamClick` (clic equipe)
- **[QCM/UI]**: Nouveau badge "+X pts" en phase `REVEALED` sur les cartes equipes ayant repondu correctement
  - Symetrique au badge Memory existant (meme emplacement, meme animation scale 0→1)
  - Variante bleue (vs vert pour Memory) pour distinguer les deux types
  - Affiche les points calcules avec la penalite individuelle (basee sur `HINTS_AT_BUZZ`)
  - N'apparait que pour les equipes dont au moins un buzzer a la bonne `ANSWER_COLOR`
  - Disparait automatiquement a la transition PREPARE (nouvelle question)

---

## [3.3.1] - 2026-04-07

### Fixed
- **[VPlayer/Fullscreen]**: Ajout d'un bouton plein ecran discret sur la page joueur (`/vplayer`)
  - Support cross-browser avec vendor prefixes (`webkit`, `moz`)
  - Icone toggle : `⛶` pour entrer, `⊠` pour sortir du plein ecran
  - Sur iOS Safari : la Fullscreen API n'est pas supportee sur `documentElement` (limitation plateforme) — le bouton est sans effet mais n'errore pas
  - Bouton fade-out apres 3s, reapparait au survol/tap
- **[VPlayer/WakeLock]**: Prevention de la mise en veille du telephone via Screen Wake Lock API
  - Guard `'wakeLock' in navigator` pour les navigateurs sans support (iOS Safari)
  - Reacquisition automatique du wake lock apres retour d'onglet (`visibilitychange`)
  - Liberation propre au demontage du composant
- **[VPlayer/Blink]**: Clignotement du buzz ralenti de 3s a 1.5s par cycle (animation `buzz-pulse`)

---

## [3.3.0] - 2026-03-28

### Added
- **[CI/CD]**: Metadonnees Windows PE dans le binaire `buzzcontrol.exe` via `goversioninfo`
  - Proprietes Windows visibles (clic droit > Proprietes > Details) : Nom du produit `BuzzControl`, Version, Description, Copyright
  - Nouveau fichier `server-go/cmd/server/versioninfo.json` — template metadonnees PE (maintenu en phase avec la version courante)
  - Nouvelle icone `server-go/assets/icon.ico` integree au binaire Windows
  - Step CI `Generate Windows PE metadata` dans le job Windows de `.github/workflows/release.yml` — genere `versioninfo.json` dynamiquement depuis le tag, puis compile le `.syso` via `goversioninfo`
  - Step `build.ps1` — genere le `.syso` avant `go build` pour les builds locaux Windows

---

## [3.2.4] - 2026-03-05

### Fixed
- **[TV Display]**: Effets neon/halo restaures sur le PlayerDisplay
  - Le CSS `.default-question-image` etait incorrectement place dans `ConfigPage.css` au lieu de `PlayerDisplay.css`, ce qui empechait son application sur la TV
  - Suppression de la transformation `y: 20` dans l'animation Framer Motion — evite la creation d'un layer GPU composite qui masquait le pseudo-element `::after` de l'effet neon (conflit z-index)
  - Remplacement de `opacity: 0.75` par `filter: brightness(0.7)` : `opacity` cree un stacking context qui bloque l'effet neon, `filter` preservele rendu visuel sans bloquer le pseudo-element

---

## [3.2.3] - 2026-03-04

### Added
- **[Backend]**: Image par defaut pour les questions sans media (`/api/config/default-image`)
  - `GET /api/config/default-image` : sert l'image personnalisee ou le SVG embarque en fallback
  - `POST /api/config/default-image` : upload d'une image personnalisee (jpg, png, gif, webp, svg)
  - `DELETE /api/config/default-image` : supprime l'image personnalisee, retour au SVG embarque
  - Asset embarque : `server-go/assets/default-question-image.svg` (icone buzzer SVG)
  - Broadcast `CONFIG_UPDATE` apres upload/suppression avec champ `default_question_image_is_custom`
- **[Frontend/TV]**: Affichage TV (`PlayerDisplay`) — image par defaut pour questions NORMAL et QCM sans media
  - Utilise `/api/config/default-image` comme fallback — toujours valide (custom ou SVG embarque)
- **[Frontend/Config]**: Section "Image par defaut" dans ConfigPage
  - Apercu de l'image courante avec cache-busting (`?t=timestamp`) apres upload/suppression
  - Bouton upload et bouton de suppression (affiche uniquement si image personnalisee active)
  - Synchronisation en temps reel via `CONFIG_UPDATE` WebSocket

### Fixed
- **[Backend]**: Initialisation de `defaultImageIsCustom` a la connexion WebSocket
  - L'etat de l'image par defaut est correctement envoye aux nouveaux clients a leur connexion

---

## [3.2.1] - 2026-03-04

### Fixed
- **[QCM Engine]**: Les hints QCM ne sont plus réinitialisés correctement lors d'un PREPARE sur la même question
  - `QcmInvalidated` etait remis a zero uniquement quand `isNewQuestion == true`
  - Quand une meme question etait re-PREPAREe (ex: apres REVEAL), les hints revelés du cycle precedent restaient visibles
  - Correction : le reset est deplace en dehors du guard `isNewQuestion` pour garantir un etat propre a chaque PREPARE
  - Fichier : `server-go/internal/game/engine.go`
  - Tests : `server-go/internal/game/engine_test.go` (158 lignes de tests ajoutees)

---

## [3.2.0] - 2026-03-02

### Added
- **[Backend]**: `BroadcasterManager` — envoi de heartbeats UDP periodiques pour la decouverte automatique du serveur
  - Format : `BUZZ_SERVER|IP1|IP2|...|PORT\0` (multi-interfaces, null-termine)
  - Intervalle normal : 5 secondes ; mode enrollment : 1 seconde
  - Heartbeat immediat au demarrage pour connexion rapide des buzzers
  - Detection automatique de toutes les IPs IPv4 actives du serveur (hors loopback et link-local)
  - Broadcast sur toutes les adresses de broadcast des interfaces reseau actives
- **[Firmware]**: `click_broadcaster.h` — listener UDP AsyncUDP pour reception des heartbeats serveur
  - Parsing du format `BUZZ_SERVER|IP1|IP2|...|PORT` (jusqu'a 8 IPs)
  - Stockage RAM uniquement (stateless, mis a jour a chaque heartbeat)
  - Fonctions : `startBroadcastListener`, `parseBroadcastHeartbeat`, `hasBroadcastDiscovery`, `getBroadcastDiscovery`
- **[Firmware]**: Failover multi-IPs dans `click_WifiManager.h`
  - Chaine de fallback : broadcast → IP NVS → mDNS → retry broadcast
  - Timeout broadcast : 30 secondes avant passage au fallback
  - Essai de chaque IP decouverte dans l'ordre jusqu'a connexion reussie
- **[Firmware]**: Nouvelles phases LED dans la sequence de boot (phases 4 et 5)
  - Phase 4 — Jaune pulsant 2 Hz : attente du heartbeat UDP (en cours de recherche serveur)
  - Phase 5 — Bleu clignotant rapide : tentative de connexion sur chaque IP decouverte

### Changed
- **[Frontend]**: Suppression des champs IP serveur et Port de la section WiFi de ConfigPage
  - La configuration IP serveur n'est plus necessaire — decouverte automatique via UDP broadcast
  - Simplification de l'interface de configuration buzzer

### Technical
- **Backend** :
  - `server-go/internal/server/broadcaster.go` : `BroadcasterManager` (nouveau)
  - `server-go/internal/server/udp.go` : `UDPBroadcaster`, `BroadcastRaw`, detection broadcast multi-interfaces
  - `server-go/cmd/server/main.go` : Integration `BroadcasterManager` dans le cycle de vie du serveur
- **Firmware** :
  - `src/BuzzClick/click_broadcaster.h` : UDP listener, parser, etat de decouverte (nouveau)
  - `src/BuzzClick/click_WifiManager.h` : Boot sequence enrichie (phases 4-5), chaine de fallback

---

## [3.1.2] - 2026-02-26

### Added
- **[USB UI]**: `USBConfigModal` unifiee — config WiFi AT et flash firmware reunis dans une seule modale
  - Point d'entree unique pour toutes les operations USB buzzer (remplace les deux interfaces separees)
  - Selection de port USB unifiee : une seule connexion serie pour config WiFi ET flash firmware
  - Bouton "Flash via USB" repositionne dans la section Firmware de ConfigPage
- **[Firmware UI]**: Badge firmware type dans ConfigPage (Full merged / App only)
  - Indique si le firmware de reference est un build complet ou uniquement l'application

### Fixed
- **[USB UI]**: Bouton "Flash via USB" desactive automatiquement quand firmware app-only (IS_MERGED=false)
  - Evite les flashs incorrects avec un firmware partiel
- **[OTA Backend]**: Champ `IS_MERGED` propage via WebSocket dans le handler `FIRMWARE_VERSION`
  - Le frontend recoit correctement le type de firmware (complet vs app-only) en temps reel
- **[USB Flash]**: Correction flash USB via esptool-js
  - Ecriture depuis l'adresse 0x0 (au lieu de 0x10000) pour un flash complet
  - Ajout hard_reset apres le flash pour redemarrage automatique du buzzer
  - Verification AT+VERSION post-flash pour confirmer le succes de la mise a jour

### Changed
- **[USB UI]**: La section "Flash via USB" de ConfigPage migree dans `USBConfigModal`
  - Coherence UI — une seule modale pour toutes les operations USB

### Technical
- **Frontend**:
  - `web/src/components/USBConfigModal.jsx` : Modale unifiee config WiFi AT + flash firmware (nouveau composant)
  - `web/src/pages/ConfigPage.jsx` : Badge IS_MERGED + integration USBConfigModal
  - `web/src/hooks/useEspFlash.js` : Correction write 0x0 + hard_reset + verification AT+VERSION

---

## [3.1.1] - 2026-02-21

### Added
- **[OTA UI]**: Bouton "Restaurer firmware embarqué" dans ConfigPage
  - Affiche le bouton uniquement quand un firmware embarqué est disponible dans le binaire serveur
  - Masque automatiquement le bouton si aucun binaire embarqué n'est présent
  - Endpoint `POST /api/firmware/buzzclick/restore-embedded` : restaure le firmware embarqué vers le stockage actif
- **[OTA Backend]**: Endpoint POST /api/firmware/buzzclick/restore-embedded
  - Extrait le firmware embarqué depuis les assets Go (`server-go/assets/firmware/buzzclick-latest.bin`)
  - Copie le binaire embarqué vers `data/firmware/buzzclick-latest.bin` (stockage actif)
  - Broadcast `FIRMWARE_VERSION` pour rafraîchissement automatique de l'UI
  - Retourne 404 si aucun firmware embarqué n'est disponible dans le binaire
- **[OTA Backend]**: Champ `EMBEDDED_VERSION` dans `FirmwareVersionPayload`
  - Expose la version du firmware embarqué dans les assets Go
  - Permet au frontend de savoir si une restauration est possible et quelle version sera restaurée
- **[OTA Firmware]**: Progression OTA propagée via engine
  - `OTA_PERCENT` propagé à travers le moteur de jeu pour un suivi centralisé
  - Architecture améliorée pour la gestion des événements OTA

### Fixed
- **[OTA UI]**: Barres de progression restent orange jusqu'au reboot du buzzer
  - `IS_OUTDATED` n'est plus remis à zéro lors de la réception de `OTA_PROGRESS done`
  - Le badge firmware repasse au vert uniquement quand le buzzer reboot et envoie HELLO avec la nouvelle version
- **[OTA UI]**: Badge firmware cliquable dans TeamsPage pour déclencher OTA individuel
- **[OTA Firmware]**: BuzzClick construit l'URL OTA depuis l'IP serveur stockée en NVS
  - Corrige la dépendance à une IP hardcodée pour le téléchargement OTA
  - Utilise `server_ip` du NVS pour construire l'URL `http://<server_ip>/api/firmware/buzzclick/latest.bin`

### Changed
- **[OTA UI]**: "Tout mettre à jour" dans ConfigPage ouvre OtaAllModal au lieu de déclencher directement l'OTA

### Technical
- **Backend** :
  - `internal/server/http_firmware.go` : Handler `handleRestoreEmbeddedFirmware()` (nouveau)
  - `internal/server/http.go` : Route `POST /api/firmware/buzzclick/restore-embedded`
  - `internal/protocol/messages.go` : Champ `EMBEDDED_VERSION` dans `FirmwareVersionPayload`
  - `internal/server/firmware.go` : Lecture version depuis firmware embarqué
- **Frontend** :
  - `web/src/pages/ConfigPage.jsx` : Bouton restauration firmware embarqué + OtaAllModal
  - `web/src/pages/TeamsPage.jsx` : Badge firmware cliquable pour OTA individuel
- **Assets** :
  - `server-go/assets/firmware/buzzclick-latest.bin` : Firmware BuzzClick embarqué dans le binaire serveur

---

## [3.1.0] - 2026-02-21

### Added
- **[OTA Buzzer]**: Mise a jour firmware OTA des buzzers BuzzClick directement depuis l'interface admin
  - Backend : `FirmwareManager` stocke le firmware de reference dans `data/firmware/`
  - Stockage versionne : `buzzclick-vX.Y.Z.bin` + `buzzclick-latest.bin` + `version.txt`
  - Endpoint `GET /api/firmware/buzzclick/version` : info firmware de reference (version, filename, size)
  - Endpoint `GET /api/firmware/buzzclick/latest.bin` : telechargement binaire pour les buzzers
  - Endpoint `POST /api/firmware/buzzclick/upload` : upload firmware .bin via multipart form
  - Endpoint `POST /api/buzzer/{mac}/update` : declenchement OTA individuel par adresse MAC
  - Endpoint `POST /api/buzzer/update-all` : OTA en masse sur tous les buzzers obsoletes
  - Validation taille firmware : 200 KB minimum, 2 MB maximum
  - Validation extension : seuls les fichiers `.bin` sont acceptes
  - Sanitisation version : prevention injection de chemin (path traversal)
  - Broadcast `FIRMWARE_VERSION` apres upload pour rafraichissement UI automatique
- **[OTA Buzzer]**: Detection et affichage des buzzers avec firmware obsolete
  - Champ `FIRMWARE_VERSION` ajoute au modele `Bumper` (version reportee via HELLO)
  - Champ `IS_OUTDATED` calcule automatiquement par comparaison semver
  - Champ `OTA_STATUS` pour suivi de progression ("downloading", "flashing", "done", "error")
  - Reset de `OTA_STATUS` automatique lors de la reconnexion (HELLO)
  - Action WebSocket `FIRMWARE_VERSION` pour mise a jour UI en temps reel
  - Action WebSocket `OTA_UPDATE` (server -> buzzer) : declenchement avec URL + version + taille
  - Action WebSocket `OTA_PROGRESS` (buzzer -> server) : progression en pourcentage + statut
- **[OTA Firmware]**: Support OTA dans le firmware BuzzClick (ESP32-C3)
  - Module `click_otaManager.h` : telechargement HTTP et flash sur partition OTA
  - Reception commande `OTA_UPDATE` via WebSocket avec URL du firmware
  - Progression reportee via `OTA_PROGRESS` (downloading 0-100%, flashing, done, error)
  - Rollback automatique ESP32 si le nouveau firmware ne demarre pas
  - `firmware_version` inclus dans le message HELLO pour identification automatique
- **[OTA UI]**: Interface admin pour gestion OTA dans ConfigPage
  - Section "Gestion Firmware" : affichage version de reference, filename, taille, statut
  - Upload firmware .bin avec validation et feedback toast
  - Bouton "Mettre a jour tous" (buzzers obsoletes uniquement)
  - Modal OTA par buzzer : version actuelle vs reference, bouton "Lancer la mise a jour OTA"
  - Statut OTA inline sur chaque buzzer dans TeamCard
- **[OTA UI]**: Indicateurs visuels firmware dans TeamCard
  - Badge `fw: X.Y.Z` rouge si buzzer obsolete, gris si a jour
  - Clic sur badge rouge ouvre la modal OTA
  - Statut OTA inline (downloading / flashing / done / error)
- **[USB Flash]**: Flash firmware via USB depuis la modal de configuration USB
  - Nouvel onglet/section "Flash Firmware" dans `USBConfigModal`
  - Hook `useEspFlash` : telechargement firmware depuis serveur + flash via `esptool-js`
  - Barre de progression + logs de flash en temps reel
  - Arret automatique de la connexion AT avant le flash (liberation du port serie)
- **[WiFi Config Broadcast]**: Diffusion configuration WiFi aux buzzers connectes
  - Nouveau champ `ssid2` / `password2` dans `WiFiDefaultsConfig` (reseau WiFi de secours)
  - Endpoint `POST /api/buzzer/wifi-config` : broadcast WIFI_CONFIG a tous les buzzers WS
  - Methode `ConnectedCount()` ajoutee a `BuzzerWebSocketHub`
  - Action WebSocket `WIFI_CONFIG` (server -> buzzer) avec SSID, PASS, SERVER_IP, PORT, SSID2, PASS2
  - Auto-sync : envoi WIFI_CONFIG automatique a chaque nouveau buzzer qui se connecte
  - Firmware BuzzClick : reception WIFI_CONFIG, sauvegarde NVS, reboot si config changee
  - Support SSID2/PASS2 dans le NVS du firmware pour reseau WiFi de secours
- **[WiFi Config UI]**: Champs SSID2/Password2 et bouton broadcast dans ConfigPage
  - Deux nouveaux champs "Reseau WiFi 2" (SSID + mot de passe de secours)
  - Bouton "Diffuser config WiFi" (envoie aux buzzers WebSocket connectes)
  - Chargement depuis `config.json` au montage de la page

### Technical
- **Backend** :
  - `internal/server/firmware.go` : `FirmwareManager` (GetInfo, SaveFirmware, IsOutdated, compareSemver)
  - `internal/server/http_firmware.go` : Handlers OTA (version, download, upload, update, update-all)
  - `internal/server/http.go` : Routes `/api/firmware/*`, `/api/buzzer/{mac}/update`, `/api/buzzer/update-all`, `/api/buzzer/wifi-config`
  - `internal/protocol/messages.go` : Actions OTA_UPDATE, OTA_PROGRESS, FIRMWARE_VERSION, WIFI_CONFIG + payloads
  - `internal/game/models.go` : Champs Bumper.FirmwareVersion, Bumper.IsOutdated, Bumper.OTAStatus
  - `internal/config/config.go` : Champs WiFiDefaultsConfig.SSID2, WiFiDefaultsConfig.Password2
  - `internal/server/websocket_buzzer.go` : Methode ConnectedCount()
  - `cmd/server/main.go` : sendWifiConfigToBuzzer(), broadcastWifiConfig(), OTA_PROGRESS handler
- **Frontend** :
  - `web/src/hooks/useEspFlash.js` : Hook flash firmware via esptool-js (nouveau fichier)
  - `web/src/components/USBConfigModal.jsx` : Section flash firmware USB
  - `web/src/components/TeamCard.jsx` : Badge firmware version + modal OTA + statut inline
  - `web/src/pages/ConfigPage.jsx` : Section firmware OTA + champs WiFi2 + bouton broadcast
  - `web/src/hooks/useWebSocket.js` : Handler FIRMWARE_VERSION
- **Firmware** :
  - `src/BuzzClick/click_otaManager.h` : Module OTA HTTP download + flash (nouveau fichier)
  - `src/BuzzClick/click_serverConnection.h` : Handler OTA_UPDATE, handler WIFI_CONFIG
  - `src/BuzzClick/click_websocketClient.h` : Inclusion firmware_version dans HELLO
  - `src/BuzzClick/click_nvsConfig.h` : Champs wifi_ssid2, wifi_pass2 dans BuzzClickConfig
- **Tests** :
  - `internal/server/firmware_test.go` : Tests FirmwareManager (SaveFirmware, GetInfo, IsOutdated, compareSemver)
  - `internal/server/firmware_http_test.go` : Tests HTTP integration endpoints firmware

### Fixed
- **[OTA UI]**: Barres de progression restent orange jusqu'au reboot du buzzer (pas de passage prématuré au vert)
- **[OTA UI]**: Barres de progression conservées par buzzer après reboot dans OtaAllModal
- **[OTA UI]**: Fermeture manuelle des modals (suppression de l'auto-close)
- **[OTA Firmware]**: URL backward-compatible dans OTA_UPDATE pour firmware < 3.1.2
- **[OTA Firmware]**: IS_OUTDATED restreint aux buzzers physiques WebSocket uniquement (pas les clients web)
- **[OTA Firmware]**: Fallback IS_OUTDATED + version firmware de référence + comptage buzzers obsolètes
- **[OTA UI]**: Affichage version firmware + support firmware embarqué
- **[OTA Backend]**: Retourne version sanitisée dans la réponse upload, suppression route morte
- **[OTA Backend]**: Reset OTA_STATUS lors de la reconnexion buzzer (HELLO)
- **[OTA Backend]**: Clés JSON majuscules dans FirmwareVersionPayload pour cohérence frontend
- **[Security]**: Sanitisation de la chaîne de version dans SaveFirmware (prévention path traversal)
- **[UI]**: Synchronisation firmwareInfo depuis WebSocket dans ConfigPage (utilisation hook property)
- **[Firmware]**: Flash USB via binaire merged + snapshot progression OTA done
- **[OTA UI]**: Barres de progression orange jusqu'au reboot, vert sur reboot confirmé

### Notes
- OTA requiert une connexion WiFi stable (~500KB-1MB par buzzer)
- Duree OTA estimee : 30-60 secondes par buzzer
- Rollback automatique ESP32 si le firmware ne demarre pas apres flash
- WIFI_CONFIG necessite que le buzzer soit connecte via WebSocket (pas TCP)
- Flash USB via esptool-js requiert Chrome/Edge 89+ sur localhost
- Flash USB supporte le binaire merged (bootloader + partition table + app)

---

## [3.0.8] - 2026-02-16

### Fixed
- **[BuzzClick Firmware]**: Cycle gris 1/3 supprime pendant boot grace au flag `bootComplete`
  - Le gray rotation s'executait des le boot et ecrasait tous les patterns LED
  - Flag `bootComplete = false` initialise, mis a `true` apres Phase 6 (HELLO ack)
  - Fichiers : `click_serverConnection.h`, `click_MAIN.cpp`
- **[BuzzClick Firmware]**: Phase 3 orange 2/4 maintenant visible
  - `resetGame()` et `connectWebSocket()` ecrasaient le pattern Phase 3
  - Deplace `resetGame()` avant Phase 3, supprime yellow LED overwrite dans WebSocket
  - Fichiers : `click_WifiManager.h`, `click_websocket_espidf.h`, `click_websocketClient.h`
- **[BuzzClick Firmware]**: Delays 500ms ajoutes pour visibilite de toutes les phases boot
  - Toutes les phases boot ont maintenant un `delay(500)` pour etre visibles

### Changed
- **[BuzzClick Firmware]**: Nouveau pattern LED boot (6 phases)
  - Phase 1 : RED 1/2 (12 LEDs) - Boot start (RED 1/4 → 1/2)
  - Phase 2 : RED 1/4 (6 LEDs) - WiFi connecting (ORANGE → RED)
  - Phase 3 : ORANGE 1/4 (6 LEDs) - WiFi connected (ORANGE 2/4 → 1/4)
  - Phase 4 : ORANGE 2/4 (12 LEDs) - WebSocket connecting (NOUVEAU)
  - Phase 5 : GREEN 2/4 (12 LEDs) - WebSocket connected (inchange)
  - Phase 6 : GREEN 3/4 (17 LEDs) - HELLO ack (inchange)


## [3.0.7] - 2026-02-15

### Fixed
- **[BuzzClick Firmware]**: Fixed LED superposition when assigning team color
  - Added `stopGrayRotation()` function to clear all gray LEDs and stop animation before displaying team color
  - Prevents visual artifact where gray animation LEDs persisted when team color was applied
  - Ensures clean transition from gray animation to team color display


## [3.0.6] - 2026-02-15

### Fixed
- **[BuzzClick Firmware]**: Fixed gray LED animation not restarting when buzzer removed from team
  - Enhanced team detection in `handleUpdateAction()` and `handleReadyAction()` to check for empty TEAM field
  - Three cases handled: TEAM present+non-empty (show team color), TEAM present+empty (start gray animation), TEAM absent (start gray animation)
  - Previously, when removing a buzzer from a team, the LED stayed on the last team color instead of restarting the gray rotation


## [3.0.5] - 2026-02-15

### Fixed
- **[BuzzClick Firmware]**: Fixed WebSocket message fragmentation detection
  - Replaced null-terminator detection (`\0`) with JSON brace counting algorithm
  - Correctly detects complete JSON messages by counting `{` and `}` while respecting strings and escape sequences
  - Parses message when brace count returns to 0, indicating complete JSON
  - WebSocket protocol uses frame metadata (FIN bit), not null terminators like TCP
  - Added 64KB buffer limit with overflow protection


## [3.0.4] - 2026-02-15

### Added
- **[BuzzClick Firmware]**: Added ACTION READY support via WebSocket
  - New `handleReadyAction()` function parses READY messages and extracts team color from BUMPER field
  - Handles both UPDATE and READY actions for team assignment
- **[BuzzClick Firmware]**: Added gray LED animation when no team assigned
  - Displays 1 LED out of 3 in gray (RGB 64,64,64) rotating every 200ms
  - Animation starts when buzzer not assigned to any team
  - Functions: `startGrayRotation()`, `updateGrayRotation()` called from main loop

### Fixed
- **[BuzzClick Firmware]**: Initial WebSocket fragmentation handling attempt (later fixed in v3.0.5)
  - Attempted null-terminator detection for message completion (incorrect approach)
  - Added fragment buffer to accumulate partial WebSocket frames


## [3.0.3] - 2026-02-15

### Fixed
- **[BuzzClick Firmware]**: Factory reset now persists after reboot
  - Added `ESP.restart()` after `nvsClearConfig()` in `checkBootButton()` to force immediate reboot after clearing NVS
  - Ensures empty WiFi config is reloaded on boot, preventing unwanted WiFi autostart
  - Previously, the buzzer would reconnect to the old WiFi network after factory reset because the cleared NVS values were not re-read until the next power cycle
- **[BuzzClick Firmware]**: Fixed build error when USE_WEBSOCKET=1
  - Added conditional compilation guards in `WiFiGotIP()` to use `connectWebSocket()` for WebSocket mode, `connectSRV()`+`initBroadcastUDP()` for TCP mode
  - Resolves declaration error for `initBroadcastUDP()` and `connectSRV()` when WebSocket protocol is enabled


## [3.0.0] - 2026-02-15

### Added
- **[WebSocket Buzzer]**: Support protocole WebSocket pour buzzers physiques (mode hybride TCP+WS)
  - Nouveau endpoint `/ws/buzzer` dedie aux connexions des buzzers physiques BuzzClick
  - Hub `BuzzerWebSocketHub` separe du hub web (`WebSocketHub`) pour isolation des clients
  - Identification des buzzers via adresse MAC (parametre query ou message HELLO)
  - Messages JSON standards (HELLO, BUTTON, PONG) sans terminateur null (`\0`)
  - Broadcast vers tous les buzzers WebSocket (LED_ON, LED_OFF, START, STOP, PING)
  - Envoi cible a un buzzer specifique via adresse MAC
  - Ping/pong WebSocket natif (keep-alive 30s) pour detection de deconnexion
  - Read deadline 60s avec renouvellement automatique sur activite
  - Canal de messages entrants buffered (capacite 100) avec drop gracieux si plein
  - Gestion concurrente thread-safe (mutex RWLock sur la map clients)
  - Nouveau type de client `buzzer` dans l'enum `ClientType`
  - Champ `MAC` ajoute a la struct `WebSocketClient` pour identification des buzzers
  - Champ `PROTOCOL` ajoute au modele `Bumper` pour identifier le protocole de connexion ("TCP" ou "WebSocket")
  - Compteurs de buzzers dans le broadcast `CLIENTS` : `BUZZER_TCP_COUNT` et `BUZZER_WS_COUNT`
  - Retrocompatibilite complete : le serveur TCP port 1234 reste actif en parallele
  - Les buzzers anciens (TCP) et nouveaux (WebSocket) coexistent sans conflit
- **[Firmware WebSocket]**: Client WebSocket pour BuzzClick (ESP32-C3)
  - Classe `WebSocketBuzzerClient` dans `click_websocketClient.h` (active avec flag `USE_WEBSOCKET`)
  - Connexion a `ws://<server_ip>/ws/buzzer` via bibliotheque ArduinoWebsockets
  - Reconnexion automatique avec backoff exponentiel (1s, 2s, 4s, 8s max)
  - Messages JSON : HELLO (enregistrement), BUTTON (buzz), PONG (ready-check)
  - LED indicateur : Vert = connecte, Jaune clignotant = reconnexion, Rouge = deconnecte

### Technical
- **Backend** :
  - `internal/server/websocket_buzzer.go` : Hub WebSocket dedie aux buzzers (BuzzerWebSocketHub)
    - `HandleConnection()` : Upgrade HTTP, extraction MAC query param, creation client
    - `readPump()` : Lecture messages, identification MAC, dispatch vers canal Incoming
    - `writePump()` : Ecriture messages, ping keep-alive 30s, drain queue
    - `Broadcast()` / `BroadcastRaw()` : Diffusion a tous les buzzers connectes
    - `SendToClient()` : Envoi cible par MAC address
    - `SetClientMAC()` / `GetClients()` / `BuzzerCount()` : Gestion des clients
    - Callback `OnBuzzerChange` pour notification des changements de connexion
  - `internal/server/websocket.go` : Ajout `ClientTypeBuzzer` et champ `MAC` sur `WebSocketClient`
  - `internal/server/http.go` : Nouveau handler `/ws/buzzer`, injection `BuzzerWebSocketHub` dans HTTPServer
  - `internal/game/models.go` : Champ `Protocol` sur struct `Bumper` ("TCP" ou "WebSocket")
  - `internal/protocol/messages.go` : `SerializeForWebSocket()`, `ClientsPayload` avec `BUZZER_TCP_COUNT`/`BUZZER_WS_COUNT`
  - `internal/protocol/parser.go` : Fonction `ParseSingle()` pour messages WebSocket individuels
  - `cmd/server/main.go` : `handleHello()` injecte le protocole, `broadcastClientCounts()` unifie les compteurs
- **Firmware** :
  - `src/BuzzClick/click_websocketClient.h` : Client WebSocket avec ArduinoWebsockets
    - Classe `WebSocketBuzzerClient` avec connect/loop/send/reconnect
    - Backoff exponentiel (1s-8s)
    - LED indicateurs selon etat de connexion
    - Fonctions wrapper : `ws_sendBuzz()`, `ws_sendPong()`, `ws_isConnected()`, `ws_connect()`
- **Tests** :
  - `internal/server/websocket_buzzer_test.go` : 13 tests unitaires ciblant `BuzzerWebSocketHub` :
    - Connexion/deconnexion buzzers (simple, multiple)
    - Reception messages JSON (HELLO avec MAC, BUTTON avec payload, PONG)
    - Broadcast et envoi cible (SendToClient par MAC)
    - Identification MAC via message HELLO
    - Acces concurrent (5 buzzers simultanes)
    - Gestion canal Incoming plein

### Notes
- **Retrocompatibilite** : Les buzzers avec ancien firmware (TCP) continuent de fonctionner avec le serveur v3.0.0
- **Mode hybride** : Le serveur supporte TCP (port 1234) ET WebSocket (port 80, `/ws/buzzer`) simultanement
- **Firmware** : Client WebSocket disponible via flag de compilation `USE_WEBSOCKET` (TCP par defaut)
- **Performance** : Latence WebSocket attendue entre 15-40ms (vs 10-30ms TCP), acceptable pour le jeu

---

## [2.54.0] - 2026-02-08

### Added
- **[CI/CD]**: Compilation automatique du firmware BuzzClick dans GitHub Actions
  - Nouveau job `compiling-firmware` s'exécutant en parallèle des builds Windows/Linux
  - Versioning unifié : serveur et firmware partagent désormais le même numéro de version
  - Injection automatique de la version dans `platformio.ini` via sed
  - Validation de taille du binaire firmware (200KB-2MB)
  - Chaque release GitHub contient désormais 3 binaires :
    - `buzzcontrol-vX.Y.0-windows-amd64.exe` (serveur Windows, ~8-9 MB)
    - `buzzcontrol-vX.Y.0-linux-arm64` (serveur Raspberry Pi, ~8 MB)
    - `buzzclick-vX.Y.0-firmware.bin` (firmware ESP32-C3, ~500KB-1MB)
- **[Documentation]**: Guide complet de mise à jour firmware
  - `docs/FIRMWARE_UPDATE.md` : Guide utilisateur pour flasher les buzzers BuzzClick
  - 3 méthodes documentées : GitHub Release (recommandé), build depuis sources, OTA (à venir)
  - Troubleshooting détaillé pour problèmes courants de flash
  - Tableau de compatibilité firmware/serveur

### Modified
- **[Documentation]**: Commandes PlatformIO étendues dans `docs/DEV_COMMANDS.md`
  - Section "Firmware BuzzClick (ESP32-C3)" remplace "ESP32 (legacy)"
  - Commandes d'installation PlatformIO CLI
  - Build, flash manuel via esptool, monitoring série
  - Instructions de versioning firmware
- **[Documentation]**: Workflow CI/CD documenté dans `CLAUDE.md`
  - Nouvelle section "CI/CD et Release Automatique"
  - Description des 3 jobs de compilation (Windows, Linux ARM64, Firmware)
  - Explication du versioning unifié depuis v2.54.0
  - Durée totale du pipeline : ~3-4 minutes
- **[Documentation]**: Diagramme CI mis à jour dans `docs/RELEASE_PROCEDURE.md`
  - 3 jobs de compilation en parallèle au lieu de 2
  - Vérification de 4 jobs (checking + 3 compiling) au lieu de 3
  - Durée estimée ajustée : ~3-4 minutes au lieu de ~2-3 minutes

### Technical
- **CI/CD**:
  - `.github/workflows/release.yml` : Job `compiling-firmware` ajouté
  - Installation de Python 3.11 et PlatformIO CLI sur runner Ubuntu
  - Cache pip pour accélérer les builds ultérieurs
  - Validation binaire firmware (taille min 200KB, max 2MB)
  - Artefact `firmware-buzzclick` uploadé pour le job `releasing`
  - Job `releasing` dépend maintenant de `[checking, compiling, compiling-firmware]`

### Notes
- **Rétrocompatibilité** : Les buzzers avec firmware 1.209.3 (anciennes versions) continuent de fonctionner avec les nouveaux serveurs
- **Protocole TCP/UDP** : Aucune modification du protocole de communication dans cette version
- **Durée CI** : L'ajout du job firmware n'impacte pas significativement la durée totale grâce à l'exécution en parallèle

---

## [2.53.0] - 2026-02-07

### Added
- **[VJoueur]**: Interface QCM tactile multicolore pour joueurs virtuels
  - Les VJoueurs peuvent répondre aux questions QCM en touchant directement une des 4 couleurs (Rouge, Vert, Jaune, Bleu)
  - Badge multicolore (4 quartiers colorés) affiché dans TeamCard pour identifier les VJoueurs
  - Invalidation automatique des buzzers physiques d'une équipe quand elle a un VJoueur actif en mode QCM
  - Indicateurs visuels (grisés) des buzzers physiques invalidés dans l'interface admin
  - Nouvelle action WebSocket `VPLAYER_QCM_ANSWER` avec payload `{ANSWER_COLOR: "RED"|"GREEN"|"YELLOW"|"BLUE"}`
  - Champ `IS_VPLAYER` ajouté au modèle Bumper pour différencier les joueurs virtuels des buzzers physiques

### Technical
- **Backend**:
  - `models.go`: Nouveau champ `IS_VPLAYER bool` dans la structure Bumper
  - `messages.go`: Action WebSocket `VPLAYER_QCM_ANSWER` pour réponses tactiles QCM
  - `engine.go`: Logique d'invalidation des buzzers physiques si l'équipe a un VJoueur actif en QCM
  - Tests unitaires: `TestVPlayerBumperCreation`, `TestVPlayerQCMBuzzAllColors`, `TestPhysicalBuzzerInvalidatedForQCM`, `TestPhysicalBuzzerNotInvalidatedForNonQCM`
- **Frontend**:
  - `TeamCard.jsx`: Badge SVG 4 quartiers pour VJoueurs
  - `VPlayerPage.jsx`: Interface tactile 4 boutons colorés pendant les questions QCM STARTED
  - `GamePage.jsx`: Indicateurs visuels (grisés) des buzzers physiques invalidés par VJoueur

---

## [2.52.0] - 2026-02-06

### Added
- **[QCM]**: Marqueurs d'indices sur la barre de temps
  - Traits verticaux orange/jaune positionnés sur la barre de progression du timer
  - Indiquent visuellement quand les indices QCM (invalidation de mauvaises réponses) vont se déclencher
  - Animation de pulsation quand le timer approche d'un seuil (15% avant)
  - Animation de fade-out quand l'indice est déclenché
  - Respect des contraintes de sécurité backend (seuil1 >= 2s, seuil2 >= 1s, écart >= 1s)
  - Visible uniquement si `QCM_HINTS_ENABLED = true` sur la question

### Technical
- **Frontend**:
  - `Timer.jsx`: Nouveau prop `hintMarkers` pour afficher des marqueurs sur la barre de temps
  - `Timer.css`: Styles `.hint-marker`, animations `hint-marker-pulse` et `hint-marker-fade`
  - `PlayerDisplay.jsx`: Calcul des positions des marqueurs via `useMemo` à partir de `QCM_HINT_THRESHOLD_1/2`

---

## [2.51.0] - 2026-02-06

### Added
- **[Memory]**: Modes de jeu multi-équipes (Phase 6)
  - **Mode SOLO**: Une seule équipe joue (comportement par défaut, rétrocompatible)
  - **Mode CHACUN_SON_TOUR**: Rotation stricte après chaque tentative (2 cartes), que la paire soit trouvée ou non
  - **Mode TANT_QUE_JE_GAGNE**: L'équipe garde la main tant qu'elle trouve des paires valides, rotation uniquement sur erreur
  - Sélection interactive des équipes participantes en phase PREPARE
  - Synchronisation multi-admin de la sélection d'équipes via WebSocket
  - Validation flexible: minimum 2 équipes requises au START (pas pendant la sélection)
  - Indicateur visuel de l'équipe courante sur l'affichage TV (badge coloré)
  - Mise en évidence de l'équipe courante uniquement en phase STARTED/PAUSED
  - Tableau des scores par équipe en temps réel
  - Tri automatique des équipes par performance (paires trouvées, puis erreurs)
  - Attribution des points par équipe avec affichage individuel
  - Bonus de complétion attribué à l'équipe qui trouve la dernière paire
  - Reset complet de l'état Memory lors de la sélection d'une nouvelle question
  - Badge "+X pts" remplaçant visuellement la pastille "PRET" en phase REVEALED

### Technical
- **Backend**:
  - `models.go`: Type `MemoryMode` avec constantes (SOLO, CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE)
  - `models.go`: Nouveaux champs GameState (`MemoryCurrentTeam`, `MemoryTeamPairs`, `MemoryParticipatingTeams`)
  - `messages.go`: Action WebSocket `MEMORY_SET_TEAMS` avec payload Teams
  - `engine.go`: Fonction `SetMemoryParticipatingTeams()` avec validation
  - `engine.go`: Fonction `rotateToNextTeam()` pour rotation circulaire
  - `engine.go`: Logique de rotation conditionnelle dans `FlipMemoryCard()` selon le mode
  - `engine.go`: Reset de l'état Memory dans `Ready()` lors du changement de question
  - `engine.go`: Rotation d'équipe déplacée dans `ClearMemoryFlippedCards()` après masquage des cartes
  - `main.go`: Handler `handleMemorySetTeams()` pour réception de la sélection d'équipes
- **Frontend**:
  - `PlayerDisplay.jsx`: Badge équipe courante avec couleur et animation
  - `PlayerDisplay.jsx`: Tableau scores temps réel avec tri dynamique
  - `PlayerDisplay.jsx`: Mise en évidence conditionnelle selon phase du jeu
  - `GamePage.jsx`: Interface sélection équipes (checkboxes, drag & drop)
  - `GamePage.jsx`: Synchronisation WebSocket de la sélection (serveur = source de vérité)
  - `GamePage.jsx`: Tri des équipes pour affichage (hook `displayTeams`)
  - `TeamCard.jsx`: Badge points Memory à la position du badge PRET
  - `QuestionsPage.jsx`: Sélecteur mode Memory (radio buttons SOLO/CHACUN_SON_TOUR/TANT_QUE_JE_GAGNE)

---

## [2.50.1] - 2026-02-01

### Fixed

**Filtrage des releases incomplètes** :
- Exclusion des releases en draft (non publiées)
- Exclusion des prereleases (beta/alpha)
- Exclusion des releases sans binaires (CI non terminée)
- Exclusion des releases avec binaires < 1MB (upload en cours)

---

## [2.50.0] - 2026-02-01

### Added

**Mise à jour automatique du serveur** :
- Vérification des nouvelles versions via GitHub Releases API
- Badge de notification dans la navbar si mise à jour disponible
- Page dédiée `/admin/updates` pour gérer les mises à jour
  - Liste compacte des versions (1 par ligne)
  - Icônes de statut : ✅ actuelle, ⬆️ plus récente, ⚠️ obsolète
  - Titres descriptifs extraits automatiquement du changelog
  - Notes de version dépliables avec rendu Markdown
  - Badge "Version locale" pour versions non publiées
- Téléchargement avec vérification de taille (40 MB min)
- Application avec backup automatique et rollback en cas d'échec
- Redémarrage automatique avec polling côté client

### Technical
- Nouveaux endpoints REST : GET/POST /api/updates/*
- Cache GitHub API (1h) pour éviter rate limiting
- Option config `auto_check_updates` (défaut: true)
- Parseur Markdown léger pour les notes de version

---

## [2.49.0] - 2026-02-01

### Added

**Dedicated Backup/Restore Page** :
- Nouvelle page dédiée pour la gestion des sauvegardes, restaurations et réinitialisations
  - Accessible via le menu abeille (dropdown) du header
  - Page complète : `/admin/backup` et `/anim/backup`
  - Interface déplacée depuis ConfigPage vers une page dédiée
  - Design: 3 sections avec cartes distinctes (Sauvegarde, Restauration, Réinitialisation)
  - Responsive: 3-colonnes sur desktop, single-colonne sur mobile

**Server Parameters Configuration** :
- Exposition des paramètres serveur dans la page Configuration
  - `auto_open_browsers` : Ouvrir les navigateurs automatiquement au démarrage
  - `debug` : Activer le mode debug pour les logs serveur
  - Nouvelle section "Parametres serveur" dans ConfigPage
  - Chargement depuis `/config.json` au montage
  - Sauvegarde via POST `/config.json` avec feedback utilisateur
  - Intégration harmonieuse avec les sections existantes
  - Responsive design mobile

### Changed

**Background Management Relocation** :
- Déplacement de la gestion des fonds d'écran de ConfigPage vers QuestionsPage
  - Section "Fonds d'écran" retirée de la page Configuration
  - Intégrée dans la barre latérale de la page Questions avec design collapsible
  - Nouvelle fonctionnalité : bouton toggle (▶/▼) pour économiser l'espace
  - Grille adaptée à la largeur de la sidebar (2 colonnes au lieu de 4)
  - Upload, durée, opacité, drag-drop toujours fonctionnels

**Player Card Styling** :
- Remplacé la couleur d'équipe (couleur QCM) par un gris neutre pour les cartes joueurs
  - Avant : Fond coloré selon la couleur QCM du joueur (rouge, vert, jaune, bleu)
  - Après : Fond gris neutre (#f3f4f6) cohérent pour tous les joueurs
  - Les bordures latérales colorées (team-color) restent visibles
  - Les indicateurs de couleur QCM restent visibles dans les badges

**Configuration Page Cleanup** :
- Suppression des sections Sauvegarde, Restauration, Réinitialisation de ConfigPage
  - ConfigPage conserve : Neon, Server Params, Demo, Reset Scores
  - Gain d'espace et clarté de la page de configuration

### Fixed

**UI Styling** :
- Suppression collapse/expand button de la section Fonds d'écran (QuestionsPage)
  - Toggle logique conservé mais plus de bouton visuel
  - Amélioration UX : actions simplifiées

### Documentation

**CLAUDE.md mis à jour** :
- Section Key Files détaillée avec toutes les pages frontend
- Documentation de l'organisation UI (menu principal, menu abeille)
- Répartition des fonctionnalités par page
- Décisions d'architecture v2.49.0 documentées

## [2.48.0] - 2026-01-31

### Added

**Navigation Navbar** :
- Menu déroulant sur le logo abeille BuzzControl
  - Clic sur l'abeille 🐝 ouvre/ferme le menu
  - Menu contient 2 options : ⚙️ Config et 📋 Logs
  - Fermeture au clic extérieur ou sur un item
  - Animation slideDown fluide
  - Accessibilité : aria-label et title présents

**Nouveau groupe "Pages" dans la navbar** :
- Zone dédiée aux pages TV et joueurs
- Label vertical "Pages" avec icône
- Liens : 📺 TV et 👥 Joueurs
- Même design que zones "Jeu" et "Config"
- Cohérence visuelle améliorée

### Changed

- **Navbar restructuring** : Config et Logs retirés de la navbar principale
  - Avant : 8 liens visibles [Jeu|Scores|Palmarès|Historique|Joueurs|Questions|Config|Logs]
  - Après : 8 liens + menu déroulant [🐝▼|Jeu|Scores|Palmarès|Historique|Joueurs|Questions|📺 TV|👥 Joueurs]
  - Navbar restructurée avec 3 zones : Jeu | Config | Pages
  - TV et Joueurs accessibles directement depuis la navbar
  - Pastille de connexion intacte

- **GamePage UI improvement** : Label "Affichage TV" changé en "TV" vertical
  - Alignment avec le style de la navbar
  - Label vertical centré et cohérent
  - Better space efficiency

### Technical Details

**Fichiers modifiés** :
- `server-go/web/src/components/Navbar.jsx` : Ajout useState, useRef, useEffect pour gestion menu + groupe Pages
- `server-go/web/src/components/Navbar.css` : Styles menu, animations, responsive + Pages group
- `server-go/web/src/pages/GamePage.jsx` : Label "TV" vertical au lieu de "Affichage TV:"

**Implémentation** :
- État React `isMenuOpen` avec useState
- Fermeture au clic extérieur via useEffect + useRef + document.addEventListener
- NavLink conservé pour navigation SPA
- Animation CSS keyframe `slideDown` (200ms)
- CSS variables pour cohérence (colors, spacing, z-index)

**Tests** :
- 8 scénarios E2E validés ✅
- QA Report : VALIDATED (100% pass rate)
- Responsive design vérifiée (600px - 1920px)
- Accessibilité WCAG 2.1 Level A

### Compatibility

- ✅ Non-breaking change
- ✅ Backward compatible
- ✅ Pas de changement API
- ✅ Pas de changement WebSocket
- ✅ Pas de migration requise


## [2.47.0] - 2026-01-31

### Fixed
- **Effet Néon**: Paramètres de pulsation du glow correctement transmis via WebSocket
  - Ajout de 3 champs manquants dans `NeonEffectPayload` (glow_pulse_speed, glow_pulse_min, glow_pulse_max)
  - Correction de la sérialisation dans `broadcastConfigUpdate()` et `sendStateToClient()`
  - Vitesse de pulsation maintenant configurable (0.5-5s)
  - Amplitude min/max du glow appliquée correctement

### Changed
- **UI Configuration**: Amélioration de l'organisation des paramètres néon
  - Bouton mode "Barre" renommé en "Neon" (plus clair)
  - Slider "Intensité" déplacé vers section "Arc lumineux" (meilleure cohérence)


## [2.46.0] - 2026-01-31

### Ajouts - Authentification VJoueurs WebSocket

**Correction de sécurité critique** :
- Les VJoueurs (joueurs virtuels) se connectent maintenant correctement avec un type de client distinct
- Avant : VJoueurs = admin par défaut (risque sécurité)
- Après : VJoueurs = type "vplayer", séparé d'admin et TV

**Détails** :
- Ajout du type de client `vplayer` dans l'enum ClientType (serveur)
- VPlayerPage envoie `SET_CLIENT_TYPE { TYPE: "vplayer" }` au montage
- EnrollPage envoie `SET_CLIENT_TYPE { TYPE: "vplayer" }` avant l'inscription
- Serveur broadcast CLIENTS avec 3 compteurs : admin, tv, vplayer
- Navbar affiche les 3 compteurs distinctement

**Fichiers modifiés** :
- `server-go/internal/server/websocket.go` : Ajout ClientTypeVPlayer
- `server-go/cmd/server/main.go` : handleSetClientType supporte "vplayer"
- `server-go/web/src/hooks/useWebSocket.js` : clientCounts inclut vplayer
- `server-go/web/src/pages/EnrollPage.jsx` : Appelle setClientType('vplayer')
- `server-go/web/src/pages/VPlayerPage.jsx` : Appelle setClientType('vplayer')
- `server-go/web/src/components/Navbar.jsx` : Affiche les 3 compteurs
- `contracts/websocket-actions.md` : Documentation SET_CLIENT_TYPE + CLIENTS


## [2.46.0] - 2026-01-31

### Ajouts - Effet Néon Avancé

**Modes d'affichage** :
- **Mode "bar"** (défaut) : Tube lumineux fin avec centre blanc et rotation d'arc
  - Tube fixe avec 3 couches (externe floutée, centrale précise, centre blanc)
  - Arc rotatif au centre du tube avec hotspot blanc brillant
  - Proportions équilibrées : 1/3 par couche (blur, tube, glow central)

- **Mode "halo"** : Effet néon classique avec bordure lumineuse large
  - Conic-gradient rotatif avec arc lumineux configurable
  - Glow pulsant autour de l'écran

**Paramètres configurables** (Page Configuration) :

| Paramètre | Plage | Défaut | Description |
|-----------|-------|--------|-------------|
| `enabled` | bool | false | Activer/désactiver l'effet |
| `mode` | "bar" / "halo" | "bar" | Type d'effet visuel |
| `arc_width` | 30-180° | 60° | Largeur de l'arc lumineux |
| `intensity_gap` | 0-100% | 80% | Écart d'intensité (opacité zone sombre) |
| `rotation_speed` | 1-10s | 4s | Vitesse de rotation de l'arc |
| `bar_offset` | 10-100px | 20px | Distance du tube par rapport au bord (mode bar) |
| `bar_thickness` | 2-20px | 4px | Épaisseur du tube lumineux (mode bar) |
| `arc_blur` | 0-200% | 100% | Flou de l'arc (% de bar_thickness) |
| `glow_pulse_speed` | 0.5-5s | 2s | Vitesse de pulsation du glow |
| `glow_pulse_min` | 0-100% | 30% | Opacité minimale du glow pulsant |
| `glow_pulse_max` | 0-100% | 50% | Opacité maximale du glow pulsant |

**Caractéristiques techniques** :
- Couleur automatique selon la catégorie de la question
- Animations CSS GPU-accelerated (@property + conic-gradient)
- Diffusion temps réel via WebSocket (ACTION: CONFIG_UPDATE)
- Phases actives : READY, COUNTDOWN, STARTED, PAUSED
- Ajustement automatique des marges pour éviter chevauchement avec contenu

### Corrections
- **[Positionnement]** : Préservation du `position: fixed` sur PlayerDisplay
- **[Marges]** : Ajustement dynamique des marges de contenu selon `bar_offset`
- **[Centrage]** : Arc rotatif parfaitement centré sur le tube en mode bar
- **[Proportions]** : Équilibre visuel des 3 couches du tube (1/3 chacune)
- **[Configuration]** : Restauration des valeurs par défaut correctes dans config.json

### Fichiers modifiés

**Backend** :
- `server-go/internal/config/config.go` : NeonEffectConfig avec 11 paramètres
- `server-go/internal/protocol/messages.go` : ACTION CONFIG_UPDATE
- `server-go/cmd/server/main.go` : Broadcast CONFIG_UPDATE aux clients

**Frontend** :
- `server-go/web/src/styles/neon.css` : Modes bar/halo, animations CSS
- `server-go/web/src/pages/ConfigPage.jsx` : UI complète avec 2 onglets (Structure, Glow)
- `server-go/web/src/pages/ConfigPage.css` : Styles sliders et sections néon
- `server-go/web/src/pages/PlayerDisplay.jsx` : Application classes + variables CSS
- `server-go/web/src/pages/PlayerDisplay.css` : Marges dynamiques selon bar_offset
- `server-go/web/src/pages/VPlayerPage.css` : Support effet néon sur mobile
- `server-go/web/src/hooks/useWebSocket.js` : Handler CONFIG_UPDATE

**Documentation** :
- `docs/ADMIN_GUIDE.md` : Section complète effet néon avec guide visuel
- `docs/DEV_PROCEDURE.md` : Ajout étape rebuild frontend obligatoire
- `.claude/commands/deploy.md` : Procédure rebuild frontend avant build Go

---

---

## [2.45.0] - 2026-01-30

### Améliorations
- **[Tri Rapidité]**: Persistance du tri jusqu'à PREPARE
  - **Avant** : Les cartes reprenaient leur place dès STOP
  - **Après** : Le tri par temps de buzz persiste en STARTED/PAUSED/REVEALED/STOPPED
  - **Reset** : Uniquement lors de la sélection d'une nouvelle question (PREPARE)

- **[TeamCard]**: Animation par-dessus les autres cartes
  - **zIndex dynamique** : Cartes actives (zIndex: 10) passent au-dessus des autres (zIndex: 1)
  - **Effet** : Animations de réorganisation plus fluides et visibles

- **[TeamCard]**: Suppression des temps de réponse en double
  - **Supprimé** : Temps vert sur la carte équipe (team-response-time)
  - **Supprimé** : Temps gris sur chaque joueur (buzzer-response-time)
  - **Raison** : Le temps existant sur la carte suffit

### Corrections
- **[VPlayer]**: Page VJoueur visible pendant ENROLL
  - **Problème** : VPlayers voyaient le QR Code au lieu de leur interface
  - **Solution** : Condition `gameState.phase === 'ENROLL' && !isVPlayer`

### Fichiers modifiés
- `server-go/web/src/pages/GamePage.jsx` : Condition tri étendue à STOPPED
- `server-go/web/src/components/TeamCard.jsx` : zIndex + suppression temps + condition tri
- `server-go/web/src/pages/PlayerDisplay.jsx` : Condition ENROLL pour VPlayers
- `server-go/config.json` : Version 2.45.0

---

## [2.44.2] - 2026-01-30

### Corrections
- **[TeamCard]**: Correction de la visibilité des animations de réorganisation
  - **Problème** : Animations framer-motion des joueurs/équipes invisibles lors du tri par rapidité
  - **Cause racine** : CSS `overflow: hidden` créait un stacking context bloquant layout animations
  - **Solution** : Changement `overflow: hidden` → `overflow: visible` sur `.team-card` et `.team-card-header`
  - **Impact** : Animations spring (300ms) maintenant visibles lors du réarrangement des équipes/joueurs
  - **Gestion du texte débordant** : Conservée via `text-overflow: ellipsis` sur `.team-name`

### Fichiers modifiés
- `server-go/web/src/components/TeamCard.css` : 2 changements (lignes 10 et 34)
- `server-go/config.json` : Version bumped (2.44.1 → 2.44.2)

### Validation
- ✅ Code review : CSS specificity et non-régression vérifiés
- ✅ QA : Animations testées, performances inchangées
- ✅ Breaking changes : Aucun

### Notes
- Patch release (2.44.y) - correction mineure sans nouveau feature
- Backward compatible - aucun change API
- Frontend only - aucune modification backend

---

## [2.44.1] - 2026-01-30

### Ajouts
- **[GamePage]**: Tri équipes et joueurs par temps de réponse (feature tri-rapidite-reponse)
  - **Tri dynamique** : Équipes et joueurs triés par temps de buzz (plus rapide en haut)
  - **Phase-aware** : Tri actif UNIQUEMENT en STARTED/PAUSED/REVEALED (hors jeu = tri par score)
  - **Badges de classement** : 🏆 (rang 1), 🥈 (rang 2), 🥉 (rang 3)
  - **Affichage temps** : XXXms pour chaque équipe et joueur ayant buzzé
  - **Animation réorganisation** : Spring transition ~300ms (stiffness: 300, damping: 30)
  - **Flash animation** : Pulsation verte 500ms au nouveau buzz
  - **Équipes non-buzzées** : Restent au bas de la liste sans badge ni temps
  - **Tri stable** : Même temps de buzz conserve l'ordre original
  - **Responsive** : Font-size adaptée (0.85rem desktop, 0.75rem tablet, 0.6-0.7rem mobile)

### Technique
- `GamePage.jsx` : Logic tri équipes (lines 63-97), useMemo optimization
- `GamePage.css` : Styles `.rank-badge`, `.team-response-time`
- `TeamCard.jsx` : Logique tri joueurs (lines 64-77), calcul temps ms (lines 50-52)
- `TeamCard.jsx` : Affichage badges (line 120) et temps (lines 123, 253-256)
- `TeamCard.css` : Styles `.buzzer-response-time`, animation `@keyframes buzz-flash`
- `GamePage.test.jsx` : 7 tests unitaires couvrant logique tri et calculs
- `tests/e2e/tri-rapidite-reponse.md` : 12 scénarios E2E documentés

### Tests
- **Unit tests JS** : 7 tests validant calcul temps, tri, badges, phase-aware
- **E2E scenarios** : 12 scénarios manuels (buzz équipes, joueurs, responsive, edge cases)
- **Code review** : APPROVED (Phase 3 complétée)
- **QA validation** : VALIDATED (Phase 4 complétée)

### Notes
- Calcul temps : `(timestamp - gameTime) / 1000` (µs → ms)
- Dépendances : Aucune nouvelle dépendance (utilise Framer-Motion existant)
- Performance : Optimisé via useMemo + layoutId Framer-Motion
- Breaking changes : Aucun

---

## [2.43.0] - 2026-01-26

### Ajouts
- **[Logs]**: WebSocket dédiée `/ws/logs` pour une gestion optimisée des logs
  - **Séparation des WebSockets** : `/ws` pour le jeu, `/ws/logs` pour les logs
  - **Connexion directe** : LogsPage se connecte à `/ws/logs` au lieu de `/ws`
  - **Messages dédiés** : LOG_HISTORY (historique à la connexion), LOG_ENTRY (temps réel)
  - **Pas de conflit** : Les logs ne transitent plus par la WebSocket de jeu

### Modifié
- **[LogsPage]**: Utilise `connectToLogs()` au lieu de `connect()`
  - Hook personnalisé pour gérer la WebSocket `/ws/logs`
  - Subscription/unsubscription automatique

### Corrigé
- **[LogsPage]**: Layout avec position fixed et scroll interne
  - Page fixe sans scroll global (`.logs-page { position: fixed }`)
  - Toolbar sticky en haut (`.logs-toolbar { position: sticky, z-index: 10 }`)
  - Liste des logs scrollable (`.logs-list { flex: 1, overflow-y: auto }`)

### Technique
- `websocket.go` : Nouvelle fonction `ServeLogsWS()` pour `/ws/logs`
- `main.go` : Handler `/ws/logs`, `ConnectToLogs()`, `DisconnectFromLogs()`
- `LogsPage.jsx` : Hook `useLogsWebSocket()` avec connexion dédiée
- `LogsPage.css` : Structure flexbox avec position fixed
- `useWebSocket.js` : Suppression handlers LOG_HISTORY et LOG_ENTRY (déplacés vers useLogsWebSocket)

---

## [2.42.0] - 2026-01-26

### Ajouts
- **[Logs]**: Page de visualisation des logs serveur en temps reel
  - **Route `/admin/logs` et `/anim/logs`** : Nouvelle page d'administration
  - **LogBuffer** : Buffer circulaire thread-safe (capacite 1000 logs)
  - **BroadcastLogger** : Logger avec diffusion temps reel via WebSocket
  - **Filtres de niveau** : DEBUG (gris), INFO (blanc), WARN (orange), ERROR (rouge)
  - **Filtres de composant** : App, Engine, HTTP, WebSocket, TCP, UDP
  - **Recherche temps reel** : Debounce 300ms avec highlight des termes
  - **Auto-scroll intelligent** : Pause automatique au scroll manuel, reprise en bas
  - **Indicateur nouveaux logs** : Badge flottant cliquable pour descendre
  - **Export** : Telechargement des logs filtres au format `.log`

### Technique
- `models.go` : Structs `LogLevel`, `LogComponent`, `LogEntry`
- `logbuffer.go` : `LogBuffer` avec `Add()`, `GetAll()`, `GetRecent()`
- `logger.go` : `BroadcastLogger` avec `Debug()`, `Info()`, `Warn()`, `Error()`
- `websocket.go` : `SubscribeToLogs()`, `UnsubscribeFromLogs()`, `BroadcastToLogSubscribers()`
- `messages.go` : Actions `SUBSCRIBE_LOGS`, `UNSUBSCRIBE_LOGS`, `LOG_HISTORY`, `LOG_ENTRY`
- `main.go` : Handlers et integration du logger
- `LogsPage.jsx` : Page principale avec toolbar et liste de logs
- `LogEntry.jsx` : Composant d'affichage d'une ligne de log
- `useWebSocket.js` : Handlers `LOG_HISTORY`, `LOG_ENTRY`, fonctions `subscribeLogs`, `unsubscribeLogs`
- `Navbar.jsx` : Lien "Logs" dans la section Config

### Tests
- `logbuffer_test.go` : Tests unitaires pour LogBuffer (Add, Circular, Concurrency, GetRecent)

---

## [2.41.0] - 2026-01-25

### Ajouts
- **[VPlayer]**: Interface complète de joueur virtuel avec affichage optimisé
  - **Page d'enrôlement `/`** : Formulaire d'inscription (pseudo 2-20 caractères)
    - Fond blanc pour meilleure lisibilité
    - État d'attente si inscriptions fermées ("En attente de l'ouverture...")
    - Reconnexion automatique si joueur déjà inscrit côté serveur
    - Validation temps réel du pseudo
  - **Page VPlayer `/player`** : Interface responsive avec badges d'identité permanents
    - Layout en 4 zones : Timer (top), Question, Média (cliquable pour buzz), Réponses
    - Zone média clickable pour buzzer (76% de largeur, centrée)
    - Badges flottants non-intrusifs : Nom joueur (15%), Équipe (85%)
    - Alignement précis horizontal avec les badges à hauteur du timer
    - Détection de suppression : redirection automatique vers `/` si admin supprime le joueur
  - **Bouton BUZZ intelligent** : États visuels et retour haptique
    - Phase STOPPED : "En attente de question" (gris, désactivé)
    - Phase PREPARE : "Préparation..." (orange, désactivé)
    - Phase READY/COUNTDOWN : "Prêt !" (cyan, désactivé)
    - Phase STARTED : "BUZZ !" (vert pulsant, actif)
    - Phase PAUSED : "Déjà buzzé" (bleu, désactivé)
    - Vibration haptique au buzz (100ms si supporté)
  - **Feedback visuel de buzz** : Overlay vert avec checkmark géant
    - Bordure verte pulsante plein écran
    - Animation checkmark (✓) avec pop-in
    - Texte "BUZZÉ !" avec glow vert
    - Disparition automatique après 1.5s
  - **QR Code sur `/tv`** : Overlay affiché quand l'enrollment est actif
    - QR Code 300x300px généré dynamiquement
    - Barre de progression des joueurs inscrits
  - **Zone ENROLL dans `/anim/teams`** : Contrôles compacts sur 2 lignes
    - L1: "Places max: [10] Inscrits: 0/10"
    - L2: Bouton "Lancer Inscriptions" / "Fin Inscriptions"
  - **Routes `/admin` et `/anim`** : Alias complets fonctionnels
    - Navbar avec préfixe dynamique selon l'URL courante
    - Toutes les sous-routes fonctionnent avec les deux préfixes

### Améliorations
- **[Engine]**: Protection MEMORY contre buzz VPlayer
  - Questions MEMORY ne peuvent pas être buzzées (contrôle exclusif admin)
  - `ProcessButtonPress()` ignore les buzz pour TYPE="MEMORY"
  - Test unitaire ajouté : `TestMemoryQuestionBuzzBlocking`
- **[Engine]**: Correction REVEAL depuis PAUSED
  - Permettre REVEAL depuis STOPPED ou PAUSED
  - Arrêt propre des timers countdown et principal
- **[Engine]**: Amélioration `ClearBumpers()`
  - Dissociation des bumpers dans les équipes (reset `team.Bumper`)
  - Reset complet des statuts et temps d'équipes
- **[Engine]**: Garantie champ team.NAME
  - `SetTeams()` remplit automatiquement `team.Name` depuis la clé si vide
- **[UI]**: Responsive VPlayer layout
  - Container queries pour adaptation aux différentes tailles d'écran
  - Badges redimensionnés dynamiquement (clamp)
  - Zone média ajustée pour smartphones et tablettes

### Corrigé
- **[Routes]**: Restructuration de l'architecture des routes pour clarté et cohérence
  - Route `/` : Page d'inscription joueurs (PlayerPage)
  - Routes `/admin/*` : Pages d'administration (GamePage, Scores, Teams, Quiz, etc.)
  - Routes `/anim/*` : Alias des routes admin (même comportement)
  - Route `/tv` : Affichage TV plein écran
- **[Navbar]**: Correction de la détection active pour supporter les deux préfixes
  - Fonction `isActiveRoute()` pour vérifier les deux chemins
  - Renommage de l'onglet "Équipes" → "Joueurs"
- **[TeamsPage]**: Réorganisation de la carte joueur non assigné en 3 lignes
  - Ligne 1 : Input nom + badge PRET + bouton suppression
  - Ligne 2 : Pastille avatar + 4 boutons couleurs QCM + poignée de drag
  - Ligne 3 : Informations techniques (adresse MAC + version)
  - Bouton de suppression (×) avec confirmation
- **[Tests]**: Correction des tests unitaires liés à la phase COUNTDOWN
  - Ajout de `StartImmediate()` dans engine.go pour tester sans goroutines
- **Synchronisation compteur joueurs virtuels** : Utilise `gameState.virtualPlayerCount` (source serveur)

### Technique
- `models.go` : Champs `EnrollmentActive`, `ShowQRCode`, `IS_VIRTUAL`, `PhaseEnroll`, `VirtualPlayerCount`
- `engine.go` : `StartEnrollment()`, `StopEnrollment()`, `HandleVirtualPlayerConnect()`, `StartImmediate()`
- `protocol/messages.go` : Actions SHOW_QR_CODE, HIDE_QR_CODE, PLAYER_CONNECT, PLAYER_CONNECTED
- `http.go` : Ajout `/admin` dans la liste des routes SPA
- `App.jsx` : Routes `/admin/*` et `/anim/*` en alias
- `VPlayerPage.jsx` : Layout 4 zones, badges permanents, zone média cliquable
- `VPlayerPage.css` : Positionnement badges (15%/85%), zone média 76%, responsive clamp
- `EnrollPage.jsx` : Gestion état d'attente, reconnexion auto
- `BuzzButton.jsx` : Bouton avec états visuels et vibration haptique
- `QRCodeOverlay.jsx` : Overlay QR code
- `TeamsPage.jsx` : Zone enrollment compacte, carte joueur 3 lignes, bouton suppression
- `Navbar.jsx` : Préfixe dynamique `/admin` ou `/anim`, `isActiveRoute()`
- `PlayerDisplay.jsx` : Badges permanents pour VPlayer

---

## [2.40.0] - 2026-01-19

### Ajouts
- **Nouvelles images de fond festives** : Remplacement des dégradés par des images plus joyeuses
  - Confettis colorés sur fond noir
  - Ballons dorés avec serpentins
  - Traînées de lumières néon
  - Images sourced from Unsplash (libres de droits)

### Modifié
- **Affichage TV - Phase PREPARATION** : Nouveau design centré
  - Texte "NOUVELLE QUESTION" remplace "PREPAREZ-VOUS"
  - Catégorie masquée (affichée uniquement en phase PRÊT)
  - Centrage parfait à l'écran

- **Affichage TV - Phase PRÊT (READY)** : Affichage de la catégorie
  - Icône de catégorie (grande) remplace l'icône main ✋
  - Nom de catégorie avec fond coloré
  - Animation pulsante

- **Affichage TV - Phase DÉCOMPTE (COUNTDOWN)** : Animation de la catégorie
  - La catégorie s'anime du centre vers la zone question
  - Format inline : icône à gauche + nom avec fond coloré
  - Applicable aux questions NORMAL, QCM et MEMORY

### Technique
- `PlayerDisplay.jsx` : Refonte des phases PREPARE, READY, COUNTDOWN
- `PlayerDisplay.css` : Nouveaux styles `.prepare-state`, `.category-badge-inline`, `.category-badge-large`
- `assets/demo/demo_bg_*.jpg` : 3 nouvelles images de fond embarquées

---

## [2.39.0] - 2026-01-19

### Ajouts
- **Mode Demo avec images embarquées** : Les questions de démonstration incluent maintenant des images
  - `demo1` : Carte de l'Australie (question géographie)
  - `demo4` : Chercheur d'or (question) + Tableau périodique (réponse)
  - `demo7` : Pizza (question) + Carte de l'Italie (réponse)
  - Images téléchargées depuis Unsplash et embarquées dans l'exécutable
  - Extraction automatique au premier lancement

### Modifié
- **Layout des cartes questions** : Réorganisation en 2 lignes pour plus de clarté
  - Ligne 1 : Nom de la question + Badge statut (AVAILABLE, STARTED, STOPPED, REVEALED) + Bouton supprimer
  - Ligne 2 : Catégorie + Type (Normal/QCM/MEMORY) + Target (Joueur/Équipe) + Temps + Points
  - Badge "Normal" ajouté pour les questions standards (comme QCM et MEMORY)
  - Le badge Target est maintenant toujours visible

### Technique
- `QuestionCard.jsx` : Header divisé en `qcard-header-row1` et `qcard-header-row2`
- `QuestionCard.css` : Styles pour les deux lignes + badge `.qcard-normal-badge`
- `main.go` : `createDemoQuestions()` avec champs MEDIA, extraction depuis `embed.FS`
- `assets/demo/` : 5 images embarquées (demo1_australia, demo4_gold_miner, demo4_periodic_table, demo7_pizza, demo7_italy)

---

## [2.37.0] - QCM Form Layout Fix

### Corrigé
- **Formulaire QCM** : Les 4 réponses (A, B, C, D) s'affichent maintenant correctement dans la colonne de configuration
  - Layout vertical (flex column) au lieu de grille 2x2
  - Chaque réponse a un fond coloré correspondant à sa couleur (rouge/vert/jaune/bleu estompé)
  - Résolution du conflit CSS entre `QuestionsPage.css` et `PlayerDisplay.css`
  - Classe renommée de `.qcm-answers-grid` à `.qcm-form-answers` pour éviter les collisions

### Technique
- `QuestionsPage.jsx` : Classe CSS renommée pour éviter le conflit
- `QuestionsPage.css` : Layout flex column avec fond coloré par réponse
- La réponse correcte garde sa couleur d'origine (pas de forçage en vert)

---

## [2.36.0] - Documentation & Procedures

### Ajouts
- **Procédures de développement** : Workflow complet DEV → QUALIF → RELEASE
  - `docs/DEV_PROCEDURE.md` : Environnement, conventions, debugging
  - `docs/QUALIF_PROCEDURE.md` : Tests, checklists, rapport de qualification
  - `docs/RELEASE_PROCEDURE.md` : 15 étapes pour mise en production
  - Scripts de build : `build-release.ps1` et `build-release.sh`

- **README.md** : Présentation complète du projet
  - Fonctionnalités, installation, architecture
  - Guide de démarrage rapide
  - Liens vers toute la documentation

- **Navbar réorganisée** : 2 zones distinctes
  - Zone Jeu (fond bleu) : Jeu, Scores, Palmarès, Historique
  - Zone Config (fond gris) : Équipes, Questions, Config
  - Labels verticaux pour identifier chaque zone

### Modifié
- **CLAUDE.md simplifié** : Références vers les procédures au lieu de duplication
- **Version unique** : Plus de version web séparée (bundle complet)

### Technique
- `Navbar.jsx` : Structure en 2 groupes avec `nav-group-game` et `nav-group-config`
- `Navbar.css` : Styles des zones avec dégradés et labels verticaux

---

## [2.35.0] - Portable Executable

### Ajouts
- **Exécutable portable** : Les fichiers web sont embarqués dans le binaire Go
  - Utilise `//go:embed` pour inclure `web/dist/` dans l'exécutable
  - Taille finale : ~13 MB (exécutable autonome)
  - Aucune dépendance externe pour l'interface web
  - Mode portable prioritaire sur le mode filesystem

- **Scripts de build** : Automatisation du build portable
  - `build.ps1` : Script PowerShell pour Windows
  - `build.sh` : Script Bash pour Linux/macOS
  - Étapes : build frontend → copie dist → build Go

- **Structure de données portable** :
  - Données dans `./data/` à côté de l'exécutable
  - `data/config/` : Configuration (teams, bumpers, history)
  - `data/files/` : Fichiers utilisateur (questions, backgrounds)

### Technique
- `cmd/server/embed.go` : Directive `//go:embed all:dist`
- `internal/server/http.go` : Support `fs.FS` pour fichiers embarqués
- Fallback automatique : embedded → filesystem → legacy

### Fichiers
- `cmd/server/embed.go` : Embedding des fichiers web
- `internal/server/http.go` : `SetEmbeddedFS()`, handlers modifiés
- `cmd/server/main.go` : Détection mode embedded
- `build.ps1`, `build.sh` : Scripts de build

---

## [2.34.0] - Category Palmares

### Ajouts
- **Vue PALMARES TV** : Classement des équipes et joueurs par catégorie sur l'affichage TV
  - Nouvelle vue accessible depuis le bouton "Palmares" dans les contrôles TV de l'admin
  - Grille 3x2 fixe avec maximum 6 catégories (affichage statique, pas de scroll)
  - Chaque carte catégorie affiche : icône, nom, total points (équipes + joueurs)
  - Classement séparé Équipes et Joueurs avec médailles 🥇🥈🥉
  - Mise en évidence des vainqueurs (rank-1) avec effet doré lumineux

- **Page admin Palmares** : Route `/palmares` dans la navbar
  - Vue collapsible par catégorie avec boutons "Tout ouvrir/fermer"
  - Résumé des points par catégorie
  - Composant Podium compact pour le top 3

### Technique
- Fetch `/history` pour agréger les points par catégorie
- Séparation stricte TEAM vs PLAYER (pas de mélange)
- Calcul des rangs avec gestion des égalités
- CSS viewport-based pour l'affichage TV statique

### Fichiers
- `CategoryPalmaresPage.jsx` : Page admin Palmares
- `CategoryPalmaresPage.css` : Styles page admin
- `PlayerDisplay.jsx` : Vue PALMARES TV avec fetch history et aggregation
- `PlayerDisplay.css` : Styles grille 3x2 et highlighting vainqueurs
- `GamePage.jsx` : Bouton "Palmares" dans contrôles TV
- `App.jsx` : Route `/palmares`
- `Navbar.jsx` : Lien navigation "Palmares"

---

## [2.33.0] - Memory Game Complete

### Ajouts
- **Animation cascade pour Memory** : Les cartes se retournent une par une pendant la phase COUNTDOWN
  - Cascade reveal : cartes se révèlent avec 200ms de délai entre chaque (1→2→3→...→N)
  - Décompte visuel : affiché seulement quand toutes les cartes sont révélées (5...4...3...2...1)
  - Cascade hide : cartes se cachent immédiatement quand le décompte atteint 0
  - Transition automatique vers STARTED quand toutes les cartes sont cachées

- **Synchronisation backend/frontend** : Le backend calcule la durée totale de la phase COUNTDOWN
  - Durée = cascade_reveal + MEMORIZE_TIME + cascade_hide
  - Le frontend gère les animations localement avec des états dédiés

- **Calcul des points Memory** : Score dynamique basé sur les paires trouvées et erreurs
  - Formule : `Score = (paires_trouvées × POINTS_PER_PAIR) + COMPLETION_BONUS - (erreurs × ERROR_PENALTY)`
  - Backend : `CalculateMemoryScore()` dans engine.go
  - Frontend : `memoryScore` useMemo dans GamePage.jsx
  - Score minimum = 0 (pas de score négatif)

- **Interface admin Memory** :
  - Zone Points : Affiche le score total calculé (readonly) avec tooltip détaillé
  - Zone Affichage TV : Compteur paires (X/Y) et erreurs
  - Attribution des points : Clic sur équipe/joueur attribue le score calculé

- **QuestionCard Memory** :
  - Points affichés = total maximum possible (paires × points_par_paire + bonus)
  - Zone média remplacée par 2 slots de configuration :
    - Slot gauche : `+X / paire` (gradient violet)
    - Slot droit : `-Y / erreur` (rouge si pénalité, gris sinon)
  - Badge "MEMORY" violet/rose

### Configuration
- **MEMORY_CONFIG** : Toutes les durées sont maintenant en secondes (plus de mix ms/s)
  - `FLIP_DELAY` : 3s (avant: 3000ms)
  - `REVEAL_DELAY` : 0.5s (avant: 500ms)
  - `MEMORIZE_TIME` : 5s (temps du décompte visuel)
  - `POINTS_PER_PAIR` : 10 (points par paire trouvée)
  - `ERROR_PENALTY` : 0 (pénalité par erreur)
  - `COMPLETION_BONUS` : 0 (bonus si toutes les paires trouvées)

### États frontend (PlayerDisplay.jsx)
- `cascadeRevealDone` : true quand toutes les cartes sont révélées
- `localCountdown` : décompte indépendant du backend, démarre après cascade reveal
- `cascadeHideStarted` : true quand la cascade hide est déclenchée (localCountdown === 0)
- `cascadeHideDone` : true quand toutes les cartes sont cachées

### Constantes d'animation
```javascript
STAGGER_DELAY = 200ms    // délai entre chaque carte
FLIP_ANIMATION = 600ms   // durée de l'animation flip
```

### Fichiers modifiés
- `engine.go` : Calcul de la durée totale COUNTDOWN + `CalculateMemoryScore()`
- `models.go` : FlipDelay et RevealDelay en float64 (secondes)
- `PlayerDisplay.jsx` : États et effets pour les animations cascade
- `GamePage.jsx` : `memoryScore` useMemo, attribution des points Memory
- `GamePage.css` : Style `.memory-score-input`, `.memory-admin-stats`
- `QuestionCard.jsx` : Affichage config Memory au lieu des images
- `QuestionCard.css` : Styles `.qcard-memory-config-slot`
- `QuestionsPage.jsx` : UI config en secondes
- `CLAUDE.md` : Documentation complète Memory

---

## [2.32.0] - CSS Specificity & Layout Fixes

### Corrections
- **Cartes équipes - largeur** : Les cartes équipes s'adaptent maintenant à la largeur de la colonne
  - Problème : TeamsPage.css définissait `.teams-grid { display: grid; minmax(300px, 1fr) }` qui forçait une largeur minimale de 300px
  - Solution : Sélecteur plus spécifique `.game-page .teams-grid { display: flex }` dans GamePage.css

- **Cartes équipes - joueurs visibles** : Tous les joueurs sont maintenant affichés dans les cartes équipes
  - Problème : `.team-card { overflow: hidden }` coupait le contenu débordant
  - Solution : `overflow: visible` et `flex-shrink: 0` sur `.game-page .team-card`

- **Preview TV - hauteur alignée** : La zone de preview TV a maintenant la même hauteur que les colonnes Questions et Équipes
  - Problème : `aspect-ratio: 16/9` et `max-height` contraignaient la hauteur du preview
  - Solution : `height: 100%` sur `.tv-preview` et `align-items: stretch` sur le container

### Technique
- Utilisation de sélecteurs CSS spécifiques (`.game-page .class`) pour éviter les conflits entre pages
- Les règles `!important` sur `display`, `visibility` et `height` garantissent l'affichage des joueurs

### Fichiers modifiés
- `GamePage.css` : Sélecteurs spécifiques `.game-page .teams-grid`, `.game-page .team-card`
- `QuestionPreview.css` : Suppression `aspect-ratio: 16/9`, ajout `height: 100%`
- `CLAUDE.md` : Documentation de la section "CSS Specificity & Layout Fixes"

---
## [2.30.0] - Background Image Synchronization

### Ajouts
- **Synchronisation des images de fond** : Tous les écrans TV affichent la même image simultanément
  - Le serveur maintient `CurrentBackgroundIndex` dans GameState
  - Goroutine de cycling basée sur la durée de chaque image
  - Broadcast `BACKGROUND_CHANGE` à tous les clients à chaque transition
  - Les clients utilisent l'index serveur au lieu du cycling local
  - Transitions parfaitement synchronisées entre tous les écrans

### Fichiers
- `engine.go` : Méthodes `GetCurrentBackgroundIndex()`, `SetCurrentBackgroundIndex()`, `NextBackground()`, `GetCurrentBackgroundDuration()`
- `models.go` : Champ `CurrentBackgroundIndex` dans GameState
- `messages.go` : Action `BACKGROUND_CHANGE`, `BackgroundChangePayload`
- `main.go` : Goroutine `startBackgroundCycling()`, `broadcastBackgroundChange()`
- `useWebSocket.js` : Handler `BACKGROUND_CHANGE`, state `currentBackgroundIndex`
- `PlayerDisplay.jsx` : Utilise `gameState.currentBackgroundIndex`

---

## [2.29.0] - 3-Second Countdown

### Ajouts
- **Décompte 3-2-1 avant le timer** : Phase COUNTDOWN distincte
  - Affichage visuel "3... 2... 1... GO!" avant le timer principal
  - Nouvelle phase `COUNTDOWN` dans la machine d'états
  - Badge orange "DECOMPTE" dans le Timer
  - Les buzzers restent bloqués pendant le décompte
  - Le timer démarre automatiquement après le décompte

- **Comportement QCM amélioré** :
  - READY : Zones de couleur sans texte de réponse
  - COUNTDOWN : Texte des réponses apparaît avec animation
  - STARTED : Question et médias affichés

### Fichiers
- `engine.go` : Phase `COUNTDOWN`, callback `OnCountdownTick`
- `models.go` : `PhaseCountdown`, `CountdownTime` dans GameState
- `main.go` : `broadcastCountdownUpdate()`, gestion START avec countdown
- `Timer.jsx` : Badge "DECOMPTE", affichage du compteur
- `PlayerDisplay.jsx` : États COUNTDOWN, animation texte QCM
- `useWebSocket.js` : Handler `countdownTime`

---

## [2.28.0] - PONG Visual Feedback & Refactoring

### Ajouts
- **Feedback visuel PONG** : Indication claire de l'état de préparation des joueurs
  - Équipes grisées (opacity 60%, grayscale 50%) en attendant que tous les joueurs répondent
  - Badge compteur "X/Y" (ex: "1/3") indiquant joueurs prêts / total au lieu de "..."
  - Joueurs individuels grisés jusqu'à leur réponse PONG
  - Joueurs ayant répondu retrouvent leur couleur d'équipe avec bordure colorée
  - Bordure d'équipe pointillée en attente, solide quand prête

- **Simulation PONG (debug)** : Ctrl+clic sur un joueur en phase PREPARE simule une réponse PONG

### Refactoring
- **Fusion handlePong** : Les handlers TCP et WebSocket fusionnés en une seule fonction
  - ID bumper extrait du payload si présent (WebSocket), sinon utilise clientID (TCP)
  - Suppression du code dupliqué `handleSimulatedPong`

### Fichiers
- `main.go` : Refactoring `handlePong()` unifié
- `TeamCard.jsx` : Compteur `readyBuzzersCount/totalBuzzersCount`, classe `waiting-pong`
- `TeamCard.css` : Styles `.team-card.waiting`, `.waiting-pong`, `.waiting-pong.ready`
- `useWebSocket.js` : Fonction `simulatePong()`
- `GamePage.jsx` : Gestion Ctrl+clic pour simuler PONG

---

## [2.23.0] - Category Balance & History Categories

### Ajouts
- **CategoryBalance Component** : Visualisation de l'équilibre des catégories sur la page Questions
  - Barres divergentes par catégorie (questions et points)
  - Zéro au centre = moyenne, droite = excès, gauche = manque
  - Code couleur : vert (≤25%), orange (25-50%), rouge (>50%)
  - Tooltip au survol avec détails complets
  - Seules les catégories représentées sont affichées
  - Animation framer-motion à l'entrée

- **Catégorie dans l'historique** : Badge catégorie sur chaque groupe de question
  - Ajout du champ `QuestionCategory` au modèle `GameEvent`
  - Icône colorée dans le header de chaque groupe
  - Visible dans la vue réduite et détaillée

### Corrections
- **Fix sélection de question** : Correction de l'erreur JSON unmarshal
  - Les questions de test avaient POINTS/TIME en nombres au lieu de strings
  - La sélection depuis PREPARE/READY fonctionne maintenant correctement

### Fichiers
- `components/CategoryBalance.jsx` : Nouveau composant
- `components/CategoryBalance.css` : Styles des barres divergentes
- `pages/QuestionsPage.jsx` : Intégration du composant
- `pages/HistoryPage.jsx` : Import CATEGORIES, affichage badge catégorie
- `pages/HistoryPage.css` : Style `.group-category`
- `internal/game/models.go` : Champ `QuestionCategory` dans `GameEvent`
- `cmd/server/main.go` : Fix POINTS/TIME strings, catégorie dans événements

---

## [2.21.0] - Data Persistence & Administration

### Ajouts
- **Persistance des données** : Sauvegarde automatique sur disque
  - `data/config/teams.json` : Équipes avec scores et TeamPoints
  - `data/config/bumpers.json` : Joueurs avec scores et assignations
  - `data/config/history.json` : Historique des événements (source de vérité)
  - Auto-save asynchrone après chaque modification
  - Chargement automatique au démarrage

- **Event Sourcing** : L'historique est la source de vérité pour les scores
  - `RecalculateScoresFromHistory()` : Recalcule tous les scores depuis les événements
  - Les scores peuvent être entièrement reconstruits à tout moment

- **Backup sélectif** (`/backup-select`) : Choisir quoi sauvegarder
  - Paramètres : `questions`, `teams`, `bumpers`, `history`, `backgrounds`
  - Exemple : `/backup-select?questions=true&history=true`

- **Reset sélectif** (`/reset-select`) : Choisir quoi réinitialiser
  - Paramètres : `all`, `questions`, `teams`, `bumpers`, `history`, `backgrounds`
  - Exemple : `/reset-select?history=true&bumpers=true`

- **Restore intelligent** (`/restore`) : Détection automatique du contenu TAR
  - Détecte les fichiers présents dans l'archive
  - Restaure uniquement les éléments détectés
  - Recharge les données dans l'engine après restauration

- **Interface ConfigPage** : Sélecteurs pour backup et reset
  - Section Sauvegarde : 5 cases à cocher (Questions, Équipes, Joueurs, Historique, Fonds)
  - Section Réinitialisation : 5 cases à cocher avec confirmation
  - Boutons Sauvegarder/Restaurer/Réinitialiser

### Documentation
- Nouveau fichier `docs/ADMIN_GUIDE.md` : Guide d'administration complet
  - Persistance des données
  - Sauvegarde et restauration
  - Réinitialisation sélective
  - Gestion des scores
  - Historique des événements

### Fichiers
- `engine.go` : SaveTeams/LoadTeams, SaveBumpers/LoadBumpers, SaveHistory/LoadHistory
- `http.go` : handleBackupSelect, handleResetSelect, handleRestore (intelligent)
- `main.go` : Configuration des chemins de persistance
- `ConfigPage.jsx` : UI pour backup/reset sélectif
- `ConfigPage.css` : Styles pour les sections checkbox
- `docs/ADMIN_GUIDE.md` : Guide d'administration

---

## [2.20.0] - History Page

### Ajouts
- **History Page** : Nouvelle page `/history-page` pour visualiser l'historique des points attribués
  - Endpoint API `GET /history` retournant `[]GameEvent`
  - Événements groupés par question (ordre chronologique)
  - Vue collapsible : clic sur l'en-tête pour ouvrir/fermer
  - Boutons "Tout ouvrir" / "Tout fermer"
  - **Vue réduite** : Résumé des points par équipe et par joueur (badges colorés)
  - **Vue détaillée** : Tableau avec Heure, Équipe, Joueur, Temps, Points
  - Séparation stricte : points TEAM vs points PLAYER (pas de cumul mixte)

### Fichiers
- `HistoryPage.jsx`, `HistoryPage.css`
- `engine.go:AddGameEvent()`
- `models.go:GameEvent`

---

## [2.19.0] - Question Cards Layout & POINTS_TARGET

### Ajouts
- **Question Cards Layout** : Nouvelle mise en page des cartes questions dans le panneau admin
  - Layout horizontal : Thumbnail (70x70px) à gauche, texte à droite
  - Header : `#ID [target] 30s 1pt [STATUS]`
  - Body : Question (4 lignes max), Réponse (3 lignes max)

- **POINTS_TARGET** : Système d'attribution des points par question
  - Champ `POINTS_TARGET` sur chaque question (`PLAYER` ou `TEAM`)
  - Défaut : `PLAYER` pour NORMAL, `TEAM` pour QCM
  - Indicateur admin avec badge coloré

- **Un seul buzz par équipe** : Premier joueur à buzzer représente l'équipe
  - Si `team.Time > 0`, les buzzes suivants sont ignorés

### Fichiers
- `engine.go:ProcessButtonPress()`
- `GamePage.jsx`, `GamePage.css`

---

## [2.18.0] - Independent Team Points

### Ajouts
- **Points équipe indépendants** : Nouveau champ `TEAM_POINTS` sur les équipes
  - Score total = TEAM_POINTS + sum(player scores)
  - Clic sur header équipe = points à l'équipe
  - Clic sur ligne joueur = points au joueur
  - Tooltip affichant la décomposition du score

### Fichiers
- `models.go:Team.TeamPoints`
- `TeamCard.jsx`, `TeamCard.css`

---

## [2.17.0] - Admin Layout Fix

### Corrections
- **Layout page admin** : Page fixe sans scroll global
  - Scroll interne par colonne (Questions, Contrôles, Équipes)
  - Alignement avec le bas de la preview TV

- **TeamCard optimisé** : Réduction de l'espace occupé
  - Score compact sans label
  - Espacement et police réduits

---

## [2.16.0] - QCM Team Badges

### Ajouts
- **Pastilles d'équipes sur réponses QCM** (phases STOPPED/REVEALED)
  - Couleur = couleur de l'équipe
  - Disposition horizontale, alignée à droite
  - Taille dégradée : 70% (première) à 40% (dernière)
  - Tri par temps de réponse

### Fichiers
- `PlayerDisplay.jsx:teamsByQcmAnswer`
- `PlayerDisplay.css:.qcm-team-badges`

---

## [2.14.0] - Media Answer

### Ajouts
- **MEDIA_ANSWER** : Support des images de réponse distinctes
  - `MEDIA` : Image affichée pendant STARTED/PAUSED
  - `MEDIA_ANSWER` : Remplace MEDIA pendant REVEALED
  - Effet visuel : Cadre vert pulsant autour de l'image de réponse
  - Thumbnails sur les cartes questions

### Fichiers
- `models.go:Question.MediaAnswer`
- `http.go:POST /questions`
- `PlayerDisplay.jsx`, `PlayerDisplay.css`
- `QuestionsPage.jsx`, `QuestionsPage.css`
- `GamePage.jsx`, `GamePage.css`

---

## [2.12.0] - Points Animation & UX Improvements

### Ajouts
- **Points Animation** : Animation visuelle quand des points sont ajoutés
  - Confetti avec couleur d'équipe
  - Animation flottante "+X pts" au centre
  - Animation scale sur la ligne joueur

- **Debug Features** :
  - Ctrl+clic sur joueur : Simule un appui buzzer
  - Ctrl+clic sur question : Force l'état READY

- **Waiting States** : États visuels pour équipes/joueurs
  - Grisés pendant PREPARE/READY jusqu'au PONG
  - Grisés pendant STARTED/PAUSED jusqu'au buzz

- **Reaction Time** : Affichage du temps de réaction
  - Tri des joueurs par temps de réponse

### Fichiers
- `GamePage.jsx`, `GamePage.css`
- `TeamCard.jsx`, `TeamCard.css`
- `engine.go:GameTime`

---

## [2.11.1] - PlayerDisplay 4-Zone Layout

### Ajouts
- **Layout 4 zones** pour l'affichage TV (/tv) :
  - Zone 1 - Timer : 100px hauteur fixe
  - Zone 2 - Question : 80px hauteur fixe
  - Zone 3 - Media : flex: 1 (remplit l'espace)
  - Zone 4 - Answers : 120px hauteur fixe, margin-top: auto

- **Timer couleur synchronisée** : Couleur = couleur de la barre de progression
  - Vert (> 50%), Orange (25-50%), Rouge (< 25%)

- **Transition QCM unifiée** : Pas de re-render/flash entre READY → STARTED → REVEALED

### Fichiers
- `PlayerDisplay.jsx`, `PlayerDisplay.css`

---

## [2.11.0] - QuestionPreview as iframe

### Modifications
- **QuestionPreview** : Simplifié en iframe vers `/tv`
  - ~15 lignes de code vs 290
  - Synchronisation parfaite avec l'affichage réel
  - Zero maintenance

---

## [2.10.0] - Timer Phase Badges

### Ajouts
- **Pastilles colorées** indiquant l'état du jeu dans le Timer :
  - ARRET (rouge), PREPARATION (orange), PRET (cyan)
  - EN COURS (vert), PAUSE (bleu), REPONSE (gris)

### Fichiers
- `Timer.jsx`, `Timer.css`

---

## [2.7.0] - Question Reordering

### Ajouts
- **Drag and drop** pour reordonner les questions
  - Poignée ⋮⋮ sur chaque carte
  - Feedback visuel pendant le drag
  - Champ `ORDER` persisté dans `question.json`
  - Action WebSocket `REORDER_QUESTIONS`

### Fichiers
- `messages.go:ReorderQuestionsPayload`
- `main.go:handleReorderQuestions`
- `QuestionsPage.jsx`, `QuestionsPage.css`
- `GamePage.jsx`

---

## [2.6.0] - Questions QCM

### Ajouts
- **Support QCM** : Questions à choix multiples
  - Types : `NORMAL` ou `QCM`
  - 4 réponses colorées (Rouge A, Vert B, Jaune C, Bleu D)
  - Champ `QCM_CORRECT` pour la bonne réponse
  - Badge "QCM" dans la liste des questions

### Champs
- `TYPE`, `QCM_ANSWERS`, `QCM_CORRECT`

### Fichiers
- `models.go:QuestionType, QCMAnswers`
- `http.go:POST /questions`
- `QuestionsPage.jsx`, `QuestionsPage.css`

---

## [2.5.0] - Teams Drag & Drop & Answer Colors

### Ajouts
- **Teams Page Drag & Drop** : Glisser-déposer pour assigner les joueurs aux équipes
  - Grille des équipes à gauche
  - Joueurs non assignés à droite

- **Couleurs de réponse** : Chaque joueur peut avoir une couleur QCM
  - Rouge (A), Vert (B), Jaune (C), Bleu (D)
  - Sélection uniquement quand non assigné à une équipe
  - Champ `ANSWER_COLOR` dans le modèle Bumper

### Fichiers
- `TeamsPage.jsx`, `TeamsPage.css`
- `models.go:Bumper.AnswerColor`

---

## [2.4.0] - Podium Component

### Ajouts
- **Podium** : Composant partagé pour les classements
  - Variantes : `default` (full size), `compact` (preview)
  - Gestion des égalités (même rang partagé)
  - Utilisé par : ScoresPage, PlayerDisplay, QuestionPreview

### Fichiers
- `Podium.jsx`, `Podium.css`

---

## [2.3.0] - React Web Interface

### Ajouts
- **Structure des pages** :
  - `/` GamePage (admin)
  - `/tv` PlayerDisplay
  - `/scoreboard` ScoresPage
  - `/teams` TeamsPage
  - `/quiz` QuizPage
  - `/settings` SettingsPage

- **Layout 3 colonnes** pour GamePage (admin)
- **Statuts de questions colorés** : AVAILABLE (vert), STARTED (orange), STOPPED (rouge), REVEALED (gris)

---

## [2.0.0] - Go Server (Phase 1)

### Ajouts
- **Migration ESP32 → Go** : Serveur Go sur Raspberry Pi
- **Rétrocompatibilité** : Support TCP + UDP pour BuzzClick v1
- **Fonctionnalités complètes** :
  - HTTP server (port 80)
  - WebSocket server (/ws)
  - TCP server (port 1234)
  - UDP broadcast (port 1234)
  - DNS server (port 53) - captive portal
  - mDNS (_sock._tcp)
  - Questions CRUD
  - Teams/Bumpers management
  - Game state machine
  - TAR backup/restore
  - Configuration JSON

### Fichiers principaux
- `cmd/server/main.go`
- `internal/game/engine.go`, `models.go`
- `internal/server/http.go`, `websocket.go`, `tcp.go`, `udp.go`
- `internal/protocol/messages.go`, `parser.go`
