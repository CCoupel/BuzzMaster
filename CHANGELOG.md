# Changelog - BuzzControl

Historique des versions du projet BuzzControl.

## [Unreleased]

### Fixed
- **Cartes Memory redimensionnées au 80% de la zone disponible** (#108) — Les cartes Memory s'agrandissent maintenant jusqu'à 460px de large (au lieu de 280px), remplissant ~80% de la largeur disponible à l'écran, avec texte et taille de police adaptatifs via `clamp()` pilotés par la taille réelle des cartes. Garantit que la contrainte TV statique (`overflow: hidden`) est respectée en tous les cas (formule mathématique prouvée, filet de sécurité `max-width: 100%` en place). Affects : `web/src/pages/PlayerDisplay.jsx` et `web/src/pages/PlayerDisplay.css`.
- **Contraste texte type de question en phase READY** (#114) — Le label du type de question (ex. "QCM", "SPEEDY") bénéficie désormais d'un contour sombre (`-webkit-text-stroke`) et d'un fond assombri (`rgba(0,0,0,0.45)`) pour assurer un contraste de luminance garanti sur toute teinte d'arrière-plan, même clair ou saturé. Fallback `text-shadow` multi-directionnel pour navigateurs sans support du contour (Firefox). Affects : `web/src/pages/PlayerDisplay.jsx` et `web/src/pages/PlayerDisplay.css`.

## [6.5.1] - Milestone v6.5.1 — Bugfix CI/Infra (#27)

**Issues** : #151 (durcissement goroutines), #153 (job CI testing), #161 (nettoyage code mort),
#178 (offsets admin en dur), #181 (artefacts + .gitignore). Cycle de correction ciblée sur la
stabilité du moteur de jeu (concurrence), réparation de pipelines CI, et hygiène du dépôt.

### Fixed
- **Cinq goroutines du moteur de jeu (countdown, timer, MEMOTION) sans `recover()`** (#151) —
  Quatre tickers `engine.go` (countdown, timer, motion card, motion memorize) étaient non protégés
  contre les panics : un panic en dehors d'une section verrouillée (`e.mu.Lock()`) laissait le
  mutex enfermé à jamais (trap habituel des `recover()` naïfs). Extraits en méthodes séparées
  (`process*Tick`) avec verrous manuels et `defer Unlock()` — garantit la libération même en cas
  de panic avant que le handler `recoverBackgroundPanic` ne prenne le relais. Site auto-flip-back
  MEMORY (`main.go`) recouvert d'un `recover()` simple, réutilisant le formateur `server.LogRecoveredPanic`. Cinq tests de régression validés sous `-race` (countdown, timer, MEMOTION carte, MEMOTION mémorisation, auto-flip-back). Suivi 97db8ea : corrige aussi une vraie race sur les hooks de test eux-mêmes (`testInjectPanic`, `testInjectMemoryFlipBackPanic` lues/écrites par goroutines concurrentes sans synchro), et réarrête un test logiquement biaisé (goroutine orpheline source de flakiness intermittente).
- **Job CI `testing` cassé sur checkout propre** (#153) — `go:embed all:dist` exigeait que
  `web/dist/` soit construit avant `go test ./cmd/server/...`, mais le job n'exécutait pas les
  étapes npm préalables (contrairement au job `compiling`). Fix : miroir la séquence complète
  `npm run build && cp -r web/dist dist/` du job `compiling` avant `go test`. Justifié par
  `fonts_route_test.go`, qui vérifie les polices réellement embarquées.
- **Offsets chrome admin en dur (60px, 64px) remplacés par variables mesurées** (#178) —
  `UpdatePage.css` et `LogsPage.css` portaient les deux derniers nombres magiques d'offset admin,
  alors que les 8 autres pages admin avaient déjà migré vers `--admin-chrome-h` et `--navbar-h`
  mesurées dynamiquement (#177/#179). Alignement : utilisation des mêmes variables, pas de
  nouvelles constantes ad hoc.

### Changed
- **Hygiène du dépôt : artefacts volumineux supprimés + `.gitignore` durci** (#181) —
  Suppression des binaires non trackés (`server-go/buzzcontrol*`, `node_modules/`, fichiers
  temporaires VS Code et editors), archivage de huit `QUALIF_REPORT_*.md` (historique git
  conservé par renommage, pas de perte), et renforcement de `.gitignore` pour empêcher les
  futures mêmes accumulations. Cinq livrables légitimes préservés : contrat `ai-key-validation.md`,
  maquette #160 `anim-memotion-160.html`, deux release notes v3.7.0 et v6.3.0 stockées en
  `docs/releases/`, huit rapports de qualification archivés en `docs/qualif/`. À trancher
  ultérieurement : mockup `anim-communication-176-updated.html` et binaire ARM64 `server-go/buzzcontrol` (historique de déploiement ancien, hors scope de ce cycle).

### Removed
- **Code mort `PlayerPage.jsx` et `PlayerPage.css`** (#161) — Deux fichiers du frontend
  devenus orphelins (jamais utilisés depuis les débuts du projet, aucune route qui les
  importait). Suppression propre, vérifiée par grep : aucune référence résiduelle au composant
  `PlayerPage` dans le codebase (seul `VPlayerPage`, actif, subsiste).

### Technical
- **Go** — Durcissement panic-recovery : extracting de la logique verrouillée en méthodes avec
  `defer`, réutilisation du formateur partagé `LogRecoveredPanic` (#131). Tests ciblés de
  régression sur les 5 sites (countdown, timer, MEMOTION carte, MEMOTION mémorisation, auto-flip-back),
  vérifiés sous `-race` (aucune race détectée). Race détectée et corrigée sur les hooks de test
  eux-mêmes (test isolation).
- **CI** — Job `testing` : aligne la séquence npm du job `compiling` (React build + dist copy)
  avant `go:embed` et `go test`. `GO_VERSION` aligné à `1.24` (matche `go.mod`) dans les deux
  jobs.
- **Frontend CSS** — Deux dernières pages admin (`UpdatePage`, `LogsPage`) migrées vers les
  variables dynamiques `--admin-chrome-h` et `--navbar-h`, alignement avec les 8 autres pages
  qui avaient introduit `useElementHeightVar` en #177/#179.
- **Tests** — Procédure manuelle `tests/procedures/v6.4.1.md` couvre l'ensemble des 5 issues
  (Scénarios 1-5 : cycle complet #151, navigation #161, job CI #153, recette visuelle #178, hygiène
  #181). Tests automatisés : panic-recovery (5/5), grep de non-régression (3/3), vérification
  des livrables préservés (4/4). Scénarios navigateur et build QUALIF restent à la charge de
  l'utilisateur (pas de navigateur en session `qa`).

## [6.5.0] - Milestone v6.4.x — Communication Animateur (#26)

**Issues** : #167 (messagerie régie), #168 (note d'explication), #175 (menu Quitter régie),
#176 (correctifs UX régie), #177 (scrollbar permanente admin), #179 (hauteur Navbar mesurée),
#180 (fix arrondi de mesure). Sept issues d'outillage animateur/régie et de fiabilisation UI
admin, livrées dans un même cycle de qualification.

### Added
- **Messagerie régie → animateurs** (#167) — La régie envoie une consigne texte (140 caractères max) 
  qui s'affiche sur toutes les tablettes animateur connectées. Trois déclencheurs d'envoi sans bouton : 
  touche Entrée, perte de focus du champ, pause de frappe (2 s). Unique message actif à la fois, 
  acquittement global (« Vu par l'animateur » depuis `/anim`, suppression possible en régie). Deux 
  sessions `/admin` restent synchronisées. Aucune file, aucun historique. Message survit aux 
  transitions de jeu (NEW_GAME, RAZ, changement de question). Élargissement de capacité réseau : 
  action `REGIE_MESSAGE_CLEAR` acceptée depuis `/anim` en plus de `/admin`.
- **Note d'explication par question** (#168) — Éditeur de questions enrichi d'un champ texte libre 
  « Note d'explication (animateur seul) ». Affichée sur `/anim` floutée avant phase `REVEALED`, 
  révélée par appui maintenu (même geste que la réponse #166), permanente en `REVEALED`. Aucune 
  affichage sur `/admin`, `/tv`, `/player`. Survit à la réédition de la question (piège B9). 
  Persistance optionnelle (omitempty).
- **Menu « Quitter » en régie** (#175) — Nouvelle entrée en dernière position du menu de la
  Navbar admin, séparée visuellement (teinte d'avertissement) des entrées de navigation.
  Confirmation `window.confirm` explicitant la conséquence ; annulation = aucun effet ;
  confirmation = arrêt propre du serveur (`/shutdown`). Après confirmation, la Navbar est
  remplacée par un message explicite (« Serveur arrêté — cette page n'est plus active ») au
  lieu de boucler en reconnexion silencieuse — réinitialisé automatiquement si une
  reconnexion aboutit malgré tout (redémarrage manuel entre-temps).

### Changed
- **Trois actions WebSocket nouvelles** — `REGIE_MESSAGE_SEND` (Client→Server, admin uniquement), 
  `REGIE_MESSAGE_CLEAR` (Client→Server, admin + anim), `REGIE_MESSAGE` (Server→Client, admin + anim). 
  Aucun changement BREAKING sur les protocoles existants.
- **Champ Question.EXPLANATION** — Nouvellement éditable en `POST /questions`, persiste sans migration.
- **Messagerie régie — champ permanent + acquittement par double-tap** (#176) — Le champ de
  saisie côté régie reste toujours visible et éditable (pré-rempli avec le message actif) : plus
  d'état bloquant « acquitté » ni de bouton « Nouveau message ». L'acquittement animateur devient
  un geste discret (double-tap sur la zone du message, accès clavier préservé Entrée/Espace) et
  un indicateur en ligne fugace (« Vu par l'animateur », ~4s) remplace l'ancien bouton « Vu ».
- **Scrollbar permanente corrigée sur les pages admin** (#177) — Huit pages admin (GamePage,
  QuestionsPage, TeamsPage, ConfigPage, BackupPage, ScoresPage, HistoryPage,
  CategoryPalmaresPage) débordaient de 44px depuis #167 (padding-bottom du bandeau régie non
  répercuté dans leur calcul de hauteur en dur). Nouvelle variable CSS `--admin-chrome-h`,
  dérivée de la hauteur réellement mesurée du bandeau régie (`--regie-bar-h`), remplace le
  nombre magique `120px` partout. LogsPage (hors flux, `position:fixed`) corrigée séparément.
- **Hauteur de la Navbar mesurée dynamiquement** (#179) — `--admin-chrome-h` intégrait `72px`
  en dur pour la Navbar (jamais garanti par le CSS réel : logo, liens, badges, pastille de
  version). La mécanique de mesure de #177 est extraite en hook partagé
  (`useElementHeightVar`) et appliquée à la fois au bandeau régie et à la Navbar — plus aucune
  constante en pixels dans `--admin-chrome-h`.
- **Arrondi de mesure de hauteur passé à `Math.ceil`** (#180) — `Math.round` pouvait
  sous-estimer une hauteur mesurée (ex. 44.3px → 44px), réservant légèrement moins de place que
  l'élément n'en occupe réellement et provoquant une scrollbar résiduelle selon zoom/résolution.
  `Math.ceil` garantit une réservation toujours ≥ la hauteur réelle.

### Technical
- **Backend** (#167/#168) — Quatre fichiers Go modifiés : `internal/protocol/messages.go` (3 constantes 
  + RegieMessagePayload, aucun omitempty), `internal/server/inbound_allowlist.go` (allow-list des 
  2 actions), `cmd/server/main.go` (état + handlers + diffusion), `internal/game/models.go` 
  (champ Question). État du message en mémoire vive, non persisté (comme currentCreditPoints, 
  discipline mono-goroutine sans mutex). Gardé idempotent côté serveur : un SEND identique au 
  message déjà actif est un no-op (ni réarmement de SENT_AT, ni remise de CLEARED_BY, ni 
  diffusion) — l'interface régie envoie automatiquement, le même texte arrive légitimement 
  plusieurs fois (Entrée + blur + pause), sans celle garde un blur après acquittement le 
  ressusciterait sur toutes les tablettes.
- **Frontend** (#167) — Nouveau composant `RegieMessageBar.jsx`/`.css` (quatre états : repos, 
  saisie, actif, acquitté). Trois déclencheurs simultanés via useEffect (touche Entrée, blur, 
  debounce 2 s). Hook `useWebSocket` enrichi d'état `regieMessage` et helpers `sendRegieMessage()` 
  / `clearRegieMessage()`. Bande `/anim` : état repos ou message + bouton « Vu ». Montage dans 
  `App.jsx` pleine largeur fixed, padding-bottom sur `.main-content` contre occlusion des pages 
  longues. Affichage piloté **exclusivement** par l'état WebSocket reçu, jamais par état local 
  optimiste — deux sessions `/admin` restent synchronisées.
- **Frontend** (#168) — Nouvel utilitaire `useHoldToPeek.js` : extraction du geste de révélation 
  (#169, réutilisé) pour mutualisation zone réponse / note. Nouveau composant `AnimExplanationNote.jsx` 
  montant en L4 de `AnimConductPanel` : floutée avant REVEALED, visible en permanence après 
  (révélation par pression maintenue). Sans note → emplacement au repos (« Aucune note pour cette 
  question »). Modèle Bumper enrichi du champ `EXPLANATION` en formulaire `QuestionsPage` 
  (survit à réédition — piège documenté en plan B9, test dédié T8). No `dangerouslySetInnerHTML` 
  introduit, contenu textuel pur.
- **Tests** — Go : allow-list (T1), troncature 140 runes UTF-8 (T2), CLEARED_BY déduit (T3), 
  remplacement + idempotence (T4/T4b), diffusion ciblée (T5), rejeu HELLO (T6), sérialisation 
  sans omitempty (T7), persistence question + survie réédition (T8). React : 4 états du bandeau, 
  3 déclencheurs envoi (T9/T9b), affichage piloté par WS (T9c), L4 avec pression (T9). Procédure 
  manuelle : deux tablettes, reconnexion, 140 caractères accentués, retrait régie, message actif 
  lors changement question.
- **Backend** (#175) — `httpServer.OnShutdown` était déclaré et appelé (juste avant `os.Exit(0)`)
  mais jamais assigné : `/shutdown` faisait un `os.Exit(0)` sec, sans `cancelCtx()` (fuite
  goroutine AckManager), ni arrêt propre de `dnsServer`/`mdnsServer`/`broadcaster`/`udpBcast`/
  `httpServer` (port pouvant rester occupé selon l'OS). Câblage corrigé — `/shutdown` devient le
  geste quotidien de fin de partie via le menu Quitter.
- **Frontend** (#176) — Nouveau hook `useDoubleTap.js` (Pointer Events, fenêtre 300ms, tolérance
  de déplacement 10px), distinct du geste maintenu `useHoldToPeek`. `touch-action: manipulation`
  indispensable sur la zone active pour éviter le zoom navigateur tactile.
- **Frontend** (#177/#179) — Nouveau hook partagé `useElementHeightVar.js` (extraction de la
  mécanique `ResizeObserver` du bandeau régie) : mesure `getBoundingClientRect`/`borderBoxSize`,
  écrit une variable CSS uniquement si la valeur arrondie change, cleanup (`disconnect()` +
  remise à 0px) au démontage. Deux consommateurs indépendants (`--regie-bar-h`, `--navbar-h`),
  aucune interférence.
- **Tests #175** — Go : `main_test.go` T5 (assignation `OnShutdown`, sans l'exercer). React :
  Navbar.test.jsx T1-T4 (présence/position/nature non-lien, annulation sans fetch, confirmation
  appelle `/shutdown` une fois, échec réseau avalé silencieusement).
- **Tests #176** — `RegieMessageBar.test.jsx` (pré-remplissage, garde course écho/saisie,
  auto-clear sans bouton, indicateur fugace), `useDoubleTap.test.js` (nouveau), `AnimPage.test.jsx`
  (double-tap vs simple tap, accès clavier).
- **Tests #177** — `RegieMessageBar.test.jsx` T1 (mesure `--regie-bar-h`, arrondi/dédoublonnage,
  reset au démontage), procédure manuelle dédiée `tests/procedures/anim-scrollbar-177.md` (jsdom
  ne calcule pas de mise en page réelle — absence de scrollbar vérifiable uniquement par un humain).
- **Tests #179** — `useElementHeightVar.test.js` (nouveau, contrat extrait de #177),
  `Navbar.test.jsx` T2 (pose/reset de `--navbar-h`), `RegieMessageBar.test.jsx` confirmé vert
  sans modification (garde-fou de non-régression de l'extraction).
- **Tests #180** — `useElementHeightVar.test.js` et `RegieMessageBar.test.jsx` mis à jour pour les
  assertions dépendant du comportement d'arrondi (44.4/44.2 → 44px devient 45px avec `Math.ceil`).

### Security
- **Allow-list entrante WebSocket** — `REGIE_MESSAGE_SEND` : admin uniquement. 
  `REGIE_MESSAGE_CLEAR` : admin + anim. `/anim` ne peut pas envoyer SEND. TV/vplayer/buzzers ne 
  peuvent envoyer ni SEND ni CLEAR.

## [6.3.0] - 2026-08-17

**Milestone v6.2.x — Interface Animateur (#24)** : dernière livraison du milestone — mode MEMOTION
depuis `/anim` (#160) et conformité des participants MEMORY/MEMOTION (#172, #173). Clôture le
milestone à 17/17 issues.

### Added
- **Mode MEMOTION depuis l'interface animateur** (#160) — L'animateur peut désormais conduire une manche MEMOTION complète depuis sa tablette (`/anim`), sans recours à `/admin` ni à l'aperçu TV : les cinq sous-phases (`MEMORIZE` → `GRID` → `SELECTED` → `QUESTION` → `REVEAL`) s'enchaînent depuis les gestes de la zone conduite. Grille de cartes tactile `AnimMotionGrid` en MEMORIZE/GRID, carte zoomée `AnimMotionCard` en SELECTED/QUESTION/REVEAL, boutons de gestion du chrono et de la désignation du gagnant `AnimMotionActions` — aucune double-attribution de points (moteur seul crédite via `MEMOTION_DONE`). Formules de disposition et points partagées avec `/tv` (`motionGrid.js`, `motionRules.js`). Élargissement de capacité réseau : 5 actions MEMOTION (`SELECT`, `FLIP`, `STOP_TIMER`, `REVEAL`, `DONE`) désormais acceptées depuis la tablette animateur (`ClientTypeAnim`).
- **Conformité des participants pour MEMORY/MEMOTION** (#172) — Impossible désormais de démarrer une manche MEMORY ou MEMOTION sans sélection conforme d'équipes participantes. MEMORY SOLO demande exactement 1 équipe, MEMORY multi (CHACUN_SON_TOUR/TANT_QUE_JE_GAGNE) au moins 2, MEMOTION au moins 1. Modes simples (SPEEDY, QCM, ARDOISE) inchangés.

### Fixed
- **Retour arrière READY → PREPARE si sélection non conforme** (#172) — Pendant la phase READY, si un administrateur retire une équipe de la sélection et casse la conformité, la question repasse automatiquement en PREPARE. Le motif d'attente est affiché à la régie et sur la tablette animateur. Dès que la sélection redevient conforme, la phase repasse en READY sans geste supplémentaire.
- **Indicateur équipe active illisible sur fond vert** (#173) — Le double liseré blanc/quasi-noir remplace le simple liseré vert qui se fondait dans une carte d'équipe elle-même verte, offrant un contraste de luminance garanti indépendamment de la teinte de l'équipe (test sur vert ET jaune/clair, `/anim` uniquement).

### Security
- **Verrou de phase sur `Engine.Start()`** (#172) — Le démarrage d'une partie refuse désormais toute phase autre que `READY`. Garantit qu'une partie lancée est nécessairement conforme aux règles de sélection de participants, et le reste (la sélection ne peut plus être modifiée une fois `STARTED` atteint).

### Changed
- **Règles de démarrage MEMORY/MEMOTION** (#172) — La transition PREPARE → READY pour ces deux modes dépend désormais de deux critères indépendants : les buzzers physiques ET la sélection conforme de participants. Voir `docs/GAME_STATE_MACHINE.md` pour les détails. **Aucun changement de contrat WebSocket.**

## [6.2.0] - Milestone v6.2.x — Interface Animateur (#24)

**Issues** : #155 (socle) + #156 (SPEEDY) + #157 (QCM) + #158 (ARDOISE) + #163 (zone contexte enrichie) + #164 (ouverture auto `/admin`) + #165 (couleurs sémantiques + bouton "à suivre" enrichi) + #166 (refonte conduite permanente + zone réponse) + #169 (révélation par pression tactile) + #170 (crédit synchronisé entre animateurs) + #171 (révision bandeau/conduite/crédit universel) + #159 (grille MEMORY tactile) — nouvelle interface animateur pour conduite de parties SPEEDY, QCM, ARDOISE et MEMORY depuis une tablette.

### Added
- **Interface animateur dédiée** (#155, #156) : Nouvelle page `/anim` optimisée pour tablette paysage — zone contexte (question courante, chronomètre, statut connexion), zone conduite (boutons contextuels par phase SPEEDY), zone équipes avec scores. Accessible via raccourci Navbar dédié. Veille écran maintenue (wake lock natif + repli `nosleep.js`). Reconnexion automatique avec indicateur visuel.
- **Zone contexte enrichie sur `/anim`** (#163) : L'animateur voit désormais sur sa tablette l'énoncé complet de la question en cours (dès qu'elle est chargée, pas seulement son `#ID`/type/catégorie) avec ses points alignés à droite sur la ligne méta, les 4 propositions QCM en grille colorée (A/B/C/D ← RED/GREEN/YELLOW/BLUE, réutilise le mapping partagé `QCM_COLORS`), la proposition invalidée par un indice grisée/barrée, la bonne réponse marquée d'un liseré vert dès la phase `REVEALED`, ainsi que la réponse attendue pour les questions SPEEDY/ARDOISE (hors QCM) également à partir de `REVEALED`. La puce "Suivante" affiche désormais l'énoncé complet de la prochaine question (`#<ID> <type>: <énoncé>`) à gauche, avec ses points et son délai (`<points>pt <délai>s`) séparés visuellement et alignés à droite, en plus du badge catégorie déjà existant. Nouveau composant `AnimQcmOptions`.
- **`/admin` ouverte automatiquement au démarrage** (#164) : Au même titre que `/anim`, `/tv` et `/` (accueil joueurs), la page `/admin` (régie) s'ouvre désormais dans un nouvel onglet au lancement du serveur — 4 onglets au total (5 avec `/logs` en mode debug), toujours dans le même ordre, toujours désactivable via `auto_open_browsers: false`. Corrige au passage un libellé trompeur des logs de démarrage : `/anim` s'annonçait comme `(admin)` (reliquat de l'ancien alias `/anim` → `/admin`, supprimé par #155) — annonce désormais `(animateur)`.
- **Couleurs sémantiques des boutons de conduite sur `/anim`** (#165) : Les boutons de la zone conduite (`AnimConductPanel`) suivent désormais une convention de couleur unique — **vert** pour l'action normale/attendue du flux courant (LANCER, CONTINUER, RÉPONSE, "à suivre" quand c'est la seule action possible), **bleu** pour une action optionnelle qui court-circuite le flux normal (PAUSE, "à suivre" quand LANCER est aussi disponible), **rouge** pour une action destructive (STOP) — plus aucun bouton actionnable en gris (le gris reste réservé aux textes non cliquables : "Aucune question disponible", "En attente des joueurs…"). Le bouton "à suivre" affiche désormais le contenu complet de la prochaine question (même format que la puce "Suivante" de la zone contexte : `#<ID> <type>: <énoncé>` + `<points>pt <délai>s` alignés à droite) au lieu d'un simple libellé générique. Nouvel utilitaire partagé `nextQuestionFormat.js`, seule source du format, réutilisé par la zone contexte (#163) et la zone conduite.
- **Refonte de la conduite `/anim` — conduite permanente à 5 lignes, zone réponse toujours visible** (#166) : Passage le plus lourd de la série, révisant volontairement plusieurs choix livrés en #163 et #165 au fil des retours visuels de QUALIF — pas des régressions, des itérations. La zone conduite (`AnimConductPanel`) affiche désormais en permanence ses 5 emplacements (LANCER/PAUSE/CONTINUER/STOP/RÉPONSE, "à suivre", grille QCM, gestes spécifiques au mode réservés, note d'explication réservée pour #168) au lieu de rendre conditionnellement seulement les boutons pertinents à la phase — chaque bouton est désormais toujours présent, actif ou éteint (gris, non cliquable) selon la phase, avec un libellé secondaire expliquant pourquoi ("attendu", "optionnel", "indispo.", "après arrêt"...). Le bouton "à suivre" **quitte le bandeau contexte pour rejoindre la zone conduite**, juste après les 5 boutons globaux (remplace et supprime la puce "Suivante" de #163) ; il pointe désormais la **première question jouable du quiz** quand aucune question n'est en cours (démarrage d'une partie directement depuis la tablette, sans passer par `/admin` — écart de parité assumé, `/admin` inchangée). La zone contexte affiche une **nouvelle progression `n/total`** (position de la question courante dans le quiz). Le bloc "Réponse" conditionnel de #163 est remplacé par une **zone réponse permanente** (`AnimAnswerZone`, nouveau composant) : la bonne réponse (QCM et hors QCM) est désormais **toujours affichée** dès qu'une question est chargée, **floutée** jusqu'à la phase `REVEALED` puis nette — mêmes dimensions dans les deux états, pour ne provoquer aucun décalage visuel au reveal. **Le flou est une précaution d'usage contre la lecture involontaire, pas un mécanisme de confidentialité** : il ne résiste ni à un regard appuyé ni aux outils de développement — la tablette animateur ne doit pas être visible du public (déjà vrai depuis #163, la réponse étant dans le payload, davantage vrai maintenant qu'elle est à l'écran). Chronomètre déplacé en colonne dédiée du bandeau contexte, agrandi. Grille QCM déplacée du bandeau vers la zone conduite. Deux emplacements réservés, visibles mais vides, préparent les prochains lots : gestes spécifiques au mode (#166) et note d'explication (#168). Bande "messagerie régie" réservée en bas de page (#167).
- **Révélation de la réponse par appui maintenu sur `/anim`** (#169) : Remplace le flou passif permanent introduit par #166, qui ne protégeait rien de réel (une réponse floutée en continu reste lisible à qui insiste). La zone réponse (`AnimAnswerZone`) est désormais **masquée par défaut** avant la phase `REVEALED` et se révèle **tant qu'un doigt (ou la souris) reste appuyé dessus** (Pointer Events) — relâché ou le pointeur sorti de la zone, elle se remasque immédiatement. **En `REVEALED`, elle reste visible en permanence sans interaction**, comme avant — l'animateur n'a pas à garder le doigt appuyé pendant qu'il crédite les équipes. Mêmes dimensions et position dans les trois états (masquée / révélée par pression / révélée par phase), aucun décalage visuel. Comme pour le flou de #166, **ce n'est toujours pas un mécanisme de confidentialité** : la donnée transite déjà sur `/ws/anim` dès le chargement de la question (constat #163, inchangé) — la tablette animateur ne doit pas être visible du public.
- **Crédit synchronisé entre animateurs** (#170) : Jusqu'ici, créditer une équipe depuis `/anim` n'informait personne d'autre — une seconde tablette animateur, ou la régie, pouvait créditer la même équipe pour la même question sans le savoir, un double-crédit qui ne se découvrait qu'au dépouillement. Le crédit est désormais **synchrone entre tous les animateurs** : dès qu'une équipe est créditée pour la question courante, quelle que soit l'interface d'origine (`/admin` compris), tous les clients `/anim` en sont informés en direct (équipe, montant, confirmation visible) et le geste leur devient indisponible pour cette équipe. **Transversal à tous les modes conduits depuis `/anim`** (SPEEDY et QCM aujourd'hui, ARDOISE/MEMORY/MEMOTION à venir), livré comme un **composant de crédit unique** (`AnimCreditControl`) que chaque mode se contente de monter, sans écrire sa propre logique de verrouillage. **Le refus d'une réponse devient un crédit à zéro point** ("0 pt") : il emprunte exactement le même chemin qu'un crédit normal (enregistrement dans l'historique, diffusion à tous les animateurs, verrouillage de la ligne) — sans aucun état local à maintenir côté tablette (plus de `sessionStorage`, plus de réinitialisation manuelle au changement de question). **La régie reste sans restriction** : `/admin` peut toujours créditer et recréditer librement, aucune garde n'y est ajoutée — le blocage est une règle de l'interface animateur uniquement. Rejouer une question déjà créditée libère à nouveau tous les verrous. Aucun champ ajouté à `GameState`, aucune structure persistée modifiée : le mécanisme est une **projection de l'historique de partie déjà existant** (voir `docs/DATA_MODELS.md` §"GameEvent (History)"), pas un nouvel état.
- **Mode ARDOISE depuis `/anim`** (#158) : Les copies (réponses écrites) des équipes s'affichent désormais **en direct** dans la colonne équipes pendant la frappe, triées par ordre de première saisie (rang 1/2/3...) — mêmes règles de tri et de délai que `/admin` (`utils/ardoiseOrder.js`, mutualisé entre les deux interfaces, aucune divergence possible). Trois états de ligne : **a répondu, pas encore créditée** (montant proposé identique à `/admin`, "+N pts"/"0 pt" via le composant partagé de #170) ; **créditée** (état verrouillé, synchronisé entre tous les animateurs comme SPEEDY/QCM) ; **sans réponse** ("Aucune copie", pointillés, seul "0 pt" proposé). Copie longue affichée en entier, jamais tronquée. Gestes de crédit visibles uniquement à partir de la phase `REVEALED` (comme le bouton ARDOISE de `/admin` — écart assumé avec SPEEDY/QCM, qui autorisent aussi `STOPPED`), sauf une ligne déjà verrouillée qui reste affichée quelle que soit la phase suivante. Nouveau composant `AnimArdoiseList`, qui ne porte **aucune logique de crédit ou de verrouillage propre** — il monte le composant partagé de #170 et se contente de rendre ce qu'il décide. Colonne équipes bascule automatiquement sur cette liste en ARDOISE ; `AnimTeamCard` (SPEEDY/QCM) reste strictement inchangée pour les autres types de question.
- **Révision du bandeau, de la conduite et du crédit sur `/anim`** (#171) : Quatrième et dernier ajustement de la série sur retour visuel utilisateur. **Bandeau** : la ligne méta est réordonnée et agrandie — avancement (`n/total`), catégorie, type, **`#ID` de la question en cours** (désormais son propre "titre", à la taille des autres éléments, une seule fois sur la page — son ancien affichage discret en retrait disparaît), options, puis les points toujours ancrés à droite ; le statut de connexion garde sa taille actuelle. Le **statut de partie** (pastille de phase) quitte la colonne chronomètre pour la ligne réponse, juste avant celle-ci — le `Timer` (partagé avec `/admin` et `/tv`) n'est pas régressé, sa pastille y reste identique. **Conduite** : les 5 lignes sont réordonnées (gestes du mode puis contenu de la question, l'inverse d'avant) et le bouton "à suivre" **quitte sa position juste après les gestes globaux pour être ancré tout en bas** de la zone conduite, juste au-dessus de la bande régie — sa position ne bouge plus jamais, quels que soient le mode ou la hauteur du contenu central (y compris une future note #168 sur plusieurs lignes) ; si le contenu central déborde, c'est **lui seul** qui défile, jamais la page. **Crédit universel** : toute équipe se voit désormais proposer le geste "0 pt", dans les trois modes — "+N pts" n'apparaît que pour une équipe ayant réellement tenté (buzzé, répondu au QCM, rendu une copie ARDOISE), avec un motif discret ("pas de buzz"/"pas de réponse") à côté du geste sinon ; une équipe sans tentative mais déjà créditée par la régie reste correctement verrouillée. Le badge médaille (🏆🥈🥉) précède désormais le nom de l'équipe dans sa carte, et les gestes de crédit restent au même endroit qu'il y ait ou non une médaille ou des informations de buzz affichées (correctif d'un bug d'alignement latent). **Lot entièrement frontend** : aucun fichier Go touché, aucun changement de contrat.
- **Grille MEMORY tactile depuis `/anim`** (#159) : L'animateur peut désormais retourner les cartes d'une question MEMORY directement du doigt sur sa tablette (nouvelle grille en L3 de la zone conduite), sans passer par l'aperçu TV de la régie. **Premier mode où l'interface animateur agit directement sur l'état de jeu** au-delà de la conduite de phase et du crédit de points : l'action `FLIP_MEMORY_CARD` (jusqu'ici réservée à `/tv` et à la vue joueur) est désormais également acceptée depuis `/anim` — élargissement de capacité assumé, borné par le moteur (aucun retournement possible hors phase `STARTED`, quel que soit le client). Le choix des équipes participantes reste réservé à la régie (`MEMORY_SET_TEAMS` non ouverte à l'animateur). **Contrainte de correspondance positionnelle avec `/tv` et la vue joueur** : la grille animateur réutilise exactement le même mélange de cartes et la même formule de disposition que l'écran TV (extraits dans un utilitaire partagé, `PlayerDisplay.jsx` consommateur au même titre que la nouvelle grille) — un joueur annonce "la deuxième carte en haut à gauche", l'animateur doit voir la carte à la même position ; seule sa taille peut différer d'un appareil à l'autre, jamais son ordre ni le nombre de colonnes. Quatre états de carte (face cachée, retournée, paire trouvée aux couleurs de son équipe, inerte hors `STARTED`), aucune logique de jeu côté client. Équipe active mise en évidence et statistiques par équipe (paires, erreurs) dans la colonne équipes ; montant de crédit identique à `/admin`, via le composant de crédit partagé (#170/#171).
- **Conduite SPEEDY depuis `/anim`** (#156) : Cycle complet READY→STARTED→STOPPED→REVEALED opérationnel depuis l'animateur (LANCER, PAUSE, STOP, RÉPONSE, enchaînement question suivante). Conditions d'activation identiques à `/admin`.
- **Compteur animateur en Navbar** (#155) : Nouveau badge affichant le nombre d'interfaces animateur connectées (motif similaire aux badges Admin/TV/VJoueur/Buzzer).
- **Crédit de points depuis `/anim`** (#156) : Boutons de crédit pour équipes ou joueurs (selon mode), montant issu du calcul partagé, disponibles en phases STOPPED et REVEALED uniquement. Parité garantie avec `/admin`.
- **Utilitaire de calcul de points partagé** (#155) : Extraction du calcul des points (pénalité QCM, score MEMORY, crédit ARDOISE, crédit SPEEDY) en fonction `pointsAward.js`, utilisé par les deux interfaces admin et animateur.
- **Mode QCM depuis interface animateur** (#157) : Conduite QCM complète depuis `/anim` — couleur de réponse par joueur visible en direct, crédit par équipe appliquée avec pénalité d'indices calculée, justesse (bonne/mauvaise réponse) révélée seulement après attribution des points (phase REVEALED). Même comportement et mêmes montants que `/admin`.

### Fixed
- **Crédit d'équipe en phase STOPPED avec montant incorrect** (#157) — Bug latent découvert en implémentation QCM animateur : le crédit appliqué en STOPPED (avant révélation) pouvait utiliser un montant calculé sur les mauvais indices/pénalités. Désormais correct dans les deux interfaces (`/admin` et `/anim`), montant toujours calculé au moment de l'application.
- **Interface animateur figée après la connexion initiale** (#162) — Défaut fonctionnel important : hormis le passage à la question suivante, aucune mise à jour du jeu n'atteignait la tablette `/anim`. Scores, chronomètre et changement de phase (LANCER, PAUSE, STOP, RÉPONSE...) restaient bloqués sur leur état de connexion, y compris lorsque l'animateur actionnait lui-même les boutons — nécessitant un rechargement de page pour voir l'effet de sa propre action. L'interface animateur suit désormais ces changements en direct, comme `/admin` et `/tv`. Corrige aussi un montant de pénalité QCM potentiellement faux en repli (aucun buzzer correct), qui dépendait d'une donnée elle-même jamais reçue par `/anim`.
- **Confirmation de crédit invisible sur `/anim` dès 6+ équipes** (bug introduit par #166, corrigé par #170) — La carte d'équipe (`.anim-team-card`) n'avait pas `flex-shrink: 0` dans la colonne équipes (flex-column avec défilement interne) : dès que le nombre d'équipes en phase créditable dépassait la hauteur disponible, chaque carte était comprimée sous la hauteur de son propre contenu au lieu de laisser le défilement s'enclencher, et le texte de confirmation du crédit (ex. "✓ +20 pts") se retrouvait recouvert par la carte suivante — présent dans le DOM et les styles calculés, mais invisible à l'écran. Le défilement interne de la colonne équipes fonctionne désormais correctement dans tous les cas.
- **Cartes MEMORY jamais trouvées restant face cachée en `REVEALED`** (bug introduit par #159, remonté en QUALIF v6.2.0.27, corrigé en v6.2.0.28) — Sur `/anim`, une paire jamais retrouvée en cours de partie (délai écoulé, erreur) n'était ni "appariée" ni "retournée" : elle restait indéfiniment face cachée, y compris une fois la question révélée, empêchant l'animateur de voir son contenu au moment du dépouillement. La grille `/tv` gérait déjà ce cas (mécanisme distinct, non repris lors de l'implémentation initiale de #159). Corrigé : toute carte s'affiche désormais en phase `REVEALED`, avec une distinction visuelle conservée entre une paire trouvée par une équipe (couleur du propriétaire) et une paire simplement révélée en fin de partie (style neutre).
- **Dos de carte MEMORY différent entre `/anim` et `/tv`** (bug introduit par #159, corrigé en v6.2.0.29) — Le dos d'une carte face cachée affichait un icône générique 🃏 sur `/anim`, au lieu de la lettre (A, B, C...) sur fond dégradé affichée par `/tv` et la vue joueur — une invention lors de l'implémentation initiale, pas une reprise du design réel. Corrigé : même lettre, calculée sur le même ordre de cartes (`utils/memoryGrid.js`, cohérent avec la correspondance positionnelle du lot), même fond dégradé. Garantit que l'animateur et les joueurs désignent la même carte de la même façon ("la carte B") d'un écran à l'autre.
- **Badge de phase `COUNTDOWN` absent sur `/anim`** (v6.2.0.30) — Le badge de statut de partie posé par #171 (`utils/phaseBadge.js`) ne couvrait que 6 des 7 phases serveur : `COUNTDOWN` (compte à rebours avant le lancement, générique à tous les modes de jeu) manquait, un trou de couverture révélé en pratique par MEMORY (#159), dont la phase de mémorisation traverse `COUNTDOWN` de façon prolongée, mais pas propre à ce mode. Corrigé : libellé et couleur ("COMPTE A REBOURS", orange) repris à l'identique du badge déjà existant côté `/admin`.

### Changed
- **Raccourcis Navbar TV/Joueur/Animateur** (#155) : Les trois entrées ouvrent désormais des nouveaux onglets (cible `_blank`) au lieu de naviguer dans l'onglet courant. Permet au régisseur de basculer rapidement entre les vues sans perdre le contexte admin.
- **Ordre de buzz enrichi en interface animateur** (#156) : Les équipes s'affichent triées par ordre de buzz (temps de réaction croissant) quand une question est en cours ou arrêtée, avec temps de réaction visible.

### Breaking Changes
- ⚠️ **Suppression de l'alias `/anim/*` vers `/admin/*`** (#155) : Les routes `/anim/teams`, `/anim/quiz`, `/anim/settings`, etc. ne renvoient plus vers `/admin` — route unique `/anim` mène à la nouvelle interface animateur. **Impact utilisateur** : les favoris/bookmarks pointant sur `/anim/*` cessent de fonctionner, migrer vers `/admin/*` pour l'interface admin ou `/anim` pour l'animateur. Les routes `/admin/*` restent entièrement opérationnelles.

### Technical
- **Contrats** : `contracts/websocket-actions.md` allow-list client type `anim` (actions NEXT_QUESTION, SET_CREDIT_POINTS, CREDIT_POINTS), `contracts/websocket-endpoints.md` nouvel endpoint `/ws/anim`, `contracts/ws-payload-serialization.md` filtres payload par type client.
- **Backend** : Nouveau type client `anim`, endpoint `/ws/anim`, compteur `ANIM_COUNT` diffusé via `CLIENTS`, gestion allow-list, actions NEXT_QUESTION et SET_CREDIT_POINTS.
- **Frontend** : Nouvelle page `AnimPage.jsx` (zones A, B, C), composants animateur, badges Navbar, raccourci Animateur, utilitaire `pointsAward.js`, refactor `GamePage.jsx` pour réutilisation du calcul de points. Zone contexte (#163) étendue avec le nouveau composant `AnimQcmOptions.jsx`, sans aucun changement backend — toutes les données (énoncé, propositions QCM, bonne réponse, question suivante) transitaient déjà sur `/ws/anim` depuis #155.
- **Backend** (#164) : Fonction `startupPages(debug)` extraite de `displayAndOpenURLs` (`cmd/server/main.go`), pure et testée indépendamment de l'ouverture réelle des onglets navigateur.
- **Tests** : Routage `/anim` · endpoint `/ws/anim` · badge animateur · gestes par phase SPEEDY · parité crédit points avec admin. #163/#164 : 1029/1029 tests React PASS, `startupPages` couverte (présence `/admin`, comptage 4/5 onglets, `/logs` debug-only, absence de doublon, libellé `/anim` corrigé). #165 : 1047/1047 tests React PASS (`AnimConductPanel.test.jsx` étendu, dont l'état "à suivre" bleu vérifié par assertion de classe CSS et en conditions réelles serveur via l'action de debug `FORCE_READY`).
- **Frontend** (#165) : Nouvel utilitaire `web/src/utils/nextQuestionFormat.js` (`formatNextQuestionStatement`/`formatNextQuestionMeta`), source unique du format `#<ID> <type>: <énoncé>` / `<points>pt <délai>s`, consommé par la puce "Suivante" (zone A, #163) et le bouton "à suivre" (zone B, #165) — aucune duplication de format entre les deux zones.
- **QA (#163)** : Validation automatisée (build, tests Go/React) et procédure manuelle #164 toutes PASS. La vérification visuelle manuelle de la zone contexte (rendu réel énoncé/QCM/reveal sur tablette, non-régression écran 1280×800/1024×768) n'a pas pu être rejouée en session QA faute d'environnement navigateur connecté — à confirmer en QUALIF avant validation PROD (voir `_work/reports/qa-20260814-153956.md`).
- **Backend** (#166) : `NEXT_QUESTION` enrichi de `CURRENT_POSITION` (rang 1-based de la question courante) et `TOTAL_QUESTIONS`, sans `omitempty` — renseignés même en fin de quiz pour que `n/total` ne redevienne jamais vide. `getNextQuestionPayload` (`cmd/server/main.go`) ne court-circuite plus quand aucune question n'est en cours : la recherche démarre alors à l'indice 0 et renvoie la première question jouable du quiz (divergence assumée avec `GamePage.jsx`'s `nextUnplayedQuestion`, qui renvoie `null` dans ce cas). Contrats : `contracts/websocket-actions.md` §"Animateur" NEXT_QUESTION, `contracts/CHANGELOG.md` `[20260815-1]`/`[20260815-2]`, `docs/WEBSOCKET_PROTOCOL.md` nouvelle section dédiée. Aucun changement de protocole hors ces deux champs additifs.
- **Frontend** (#166) : Deux nouveaux composants (`AnimAnswerZone.{jsx,css}`, `AnimNextButton.{jsx,css}` — ce dernier extrait de `AnimConductPanel.jsx`, pas réécrit) et deux nouveaux utilitaires (`utils/questionTypeMeta.js` — icône/libellé par type de question ; `utils/phaseRules.js` — règles de phase mutualisées entre `/admin` et `/anim`, extraites de `GamePage.jsx` sans changement de comportement, `/admin` invariante). `AnimConductPanel.jsx` réécrit : ne choisit plus quels boutons rendre, calcule l'état de chacun sur une table dérivée à 100% de `phaseRules` (états `go`/`optional`/`danger`/`off`). `AnimQcmOptions.jsx` déplacé (bandeau → zone conduite L2), non modifié. Disposition de page revue (`grid-template-areas: "context teams" "conduct teams" "regie regie"`) ; zone équipes change seulement de `grid-area`, structure interne et `AnimTeamCard` non touchés. CSS mort supprimé (ancien rendu conditionnel de zone conduite, puce "Suivante", bloc "Réponse" conditionnel).
- **Tests** (#166) : 1127/1127 tests React PASS (`AnimAnswerZone.test.jsx` nouveau, `AnimNextButton.test.jsx` nouveau, `AnimConductPanel.test.jsx` réécrit pour la matrice à 5 lignes — 10 phases × 5 boutons —, `AnimPage.test.jsx` mis à jour, `AnimTeamCard.test.jsx` vert sans la moindre modification). Côté Go, `next_question_test.go`/`next_question_triggers_test.go` couvrent position/total (milieu de liste, dernière question, question courante absente de la liste, liste vide, `ORDER` non contigu) et la parité révisée. Tests obsolètes #163 (puce "Suivante")/#165 (rendu conditionnel de zone conduite) retirés ou réécrits avec la fonctionnalité qui les remplace, jamais neutralisés.
- **QA (#166)** : Build, tests automatisés et la quasi-totalité de la procédure manuelle exécutable sans navigateur (matrice de phases, dernière question, première question sans question courante) validés en conditions réelles serveur — dont le point prioritaire CDP (état "à suivre" bleu), confirmé cette fois en conditions réelles et clôturant la réserve ouverte depuis #165. Réserves reconduites, sans lien avec la logique du lot : lisibilité du flou et absence de décalage visuel au reveal à confirmer visuellement, tenue en 1024×600 (résolution la plus tendue), rejouées avec les réserves QCM_HINT (#163) et flakiness `ai_generation_test.go` (préexistante, hors périmètre) — voir `_work/reports/qa-20260815-165224.md`.
- **Frontend/Tests** (#169) : Lot circonscrit à `AnimAnswerZone.{jsx,css}` — `onPointerDown`/`onPointerUp`/`onPointerLeave`/`onPointerCancel` (Pointer Events uniquement, pas d'événements souris/touch séparés), état interne `peeking` réinitialisé au changement de question (garde-fou pression interrompue). Aucun changement de contrat/payload (donnée déjà présente depuis #163). 1138/1138 tests React PASS (+11 dédiés). Réserve QA reconduite (geste tactile non rejouable faute d'extension Chrome connectée), vérifié par dev-frontend en conditions réelles via CDP (`Input.dispatchMouseEvent`, même chemin de code qu'un `pointerdown`/`pointerup` réel).
- **Backend** (#170) : Nouvelle action `AWARDED_TEAMS` (`ClientTypeAnim` exclusif), projection de `Engine.history` filtrée sur la question courante (`QuestionID` + `Timestamp >= GameState.GameTime`, regroupement par `TeamName` — jamais `WinnerID`, qui ne porte qu'une MAC en SPEEDY) diffusée sur `TEAM_POINTS`/`BUMPER_POINTS`/`READY`/`NEW_GAME`/`RAZ`/HELLO animateur, jamais sur le chemin de `broadcastUpdate`. Aucun champ ajouté à `GameState`, aucune structure persistée modifiée. Contrats : `contracts/websocket-actions.md` §"Animateur" → `AWARDED_TEAMS` (dont la précision "`POINTS` nul = verrou valide"), `contracts/websocket-endpoints.md`, `contracts/CHANGELOG.md` `[20260816-2]`, `docs/WEBSOCKET_PROTOCOL.md` nouvelle section dédiée.
- **Frontend/Tests** (#170) : Nouveau composant unique `AnimCreditControl.{jsx,css}`, câblé sur la carte d'équipe SPEEDY/QCM à la place de l'ancien bouton `anim-team-credit-btn` — cible et montant inchangés. Verrouillage testé exclusivement sur la présence d'une entrée dans `awardedTeams` (risque R1 du plan, jamais sur la valeur du montant). 1163/1163 tests React PASS, dont les tests #156/#157 adaptés pour consommer le nouveau composant (adaptation justifiée, pas un contournement) et deux tests Go ciblant explicitement le piège du montant nul (`TestGetAwardedTeamsPayload_ZeroPointCreditPresent`, `_ZeroSum_EntryStillPresent`).
- **Frontend/Tests** (#158) : Nouvel utilitaire `utils/ardoiseOrder.js` (`formatArdoiseDelay`/`sortArdoiseEntries`), extrait à l'identique de `GamePage.jsx` et consommé par les deux interfaces — extraction pure, `/admin` invariante (78/78 tests `GamePage.*` PASS). Nouveau composant `AnimArdoiseList.{jsx,css}`, filtre équipes à joueur virtuel (`bumper.IS_VPLAYER`, parité #93). Correctif `flex-shrink: 0` de #170 répliqué d'emblée sur `.anim-ardoise-row` — pas de régression à reproduire. 1199/1199 tests React PASS (25 nouveaux : `AnimArdoiseList.test.jsx`, `ardoiseOrder.test.js`). Diffusion `ARDOISE_INPUT` vers `ClientTypeAnim` déjà livrée et documentée en révision 1 (`contracts/CHANGELOG.md` `[20260816-1]`) — aucune entrée de contrat supplémentaire pour ce lot.
- **Frontend/Tests** (#171) : Nouveaux utilitaires `utils/canAwardPoints.js` (défaut permissif pour les types hors SPEEDY/QCM, pour ne jamais bloquer silencieusement un futur mode) et `utils/phaseBadge.js` (table phase→badge dupliquée une fois depuis `Timer.css`, dérogation documentée pour ne pas coupler `Timer.jsx`, partagé `/admin`/`/tv`, à un besoin propre à `/anim`). `AnimConductPanel` réécrit en colonne flex avec bloc central `flex:1; min-height:0; overflow-y:auto` — `min-height: 0` obligatoire, sans quoi une L4 longue pousserait le bouton "à suivre" hors de l'écran, exactement le défaut que l'ancrage devait empêcher. `AnimCreditControl`, `AnimArdoiseList`, `AnimAnswerZone`, `AnimQcmOptions`, `AnimNextButton` et `Timer.jsx` **non modifiés** — seuls leurs points de montage/props changent. **Aucun fichier `.go` touché** (confirmé par QA via `git show 03beb92 --stat`). 1261/1261 tests React PASS.
- **Backend** (#159) : Une seule entrée ajoutée à `internal/server/inbound_allowlist.go` (`FLIP_MEMORY_CARD` → `ClientTypeAnim`), aucun changement de mécanisme. `admin` reste refusé, `MEMORY_SET_TEAMS` reste `admin`-only. Contrats : `contracts/websocket-actions.md` §"Sécurité — Allow-list entrante", `contracts/CHANGELOG.md` `[20260816-3]`, `docs/WEBSOCKET_PROTOCOL.md` nouvelle section dédiée.
- **Frontend/Tests** (#159) : Nouvel utilitaire `utils/memoryGrid.js` extrait **verbatim** de `PlayerDisplay.jsx` (mélange Fisher-Yates ensemencé par `question.ID`, formule de colonnes fixe) — `PlayerDisplay.jsx` consomme désormais cet utilitaire, extraction pure, 208/208 tests `PlayerDisplay` PASS sans modification. Nouveau composant `AnimMemoryGrid.{jsx,css}` (créé, pas repris du rendu TV), montage en L3 de `AnimConductPanel` (branche à 3 voies : QCM/MEMORY/réservé). `AnimCreditControl`, `AnimAnswerZone`, `AnimQcmOptions`, `AnimArdoiseList`, `AnimNextButton` non modifiés ; structure L1→L5 et ancrage de L5 (#171) intacts. 1341/1341 tests React PASS.

### Security
- **Allow-list entrante WebSocket par ClientType** (#154) — Le serveur refuse désormais les commandes de pilotage (START, STOP, DELETE, etc.) envoyées depuis un canal TV ou joueur virtuel — seul l'admin peut les émettre. Vérification centralisée à l'entrée de chaque action WebSocket, zéro impact sur l'usage normal (les clients légitimes n'envoient que les actions documentées pour leur rôle).

## [6.1.3] - 2026-08-12

**Milestone v6.0.x — Stabilité & Tests** (#23) : Correctifs critiques de sécurité, résilience
serveur, persistance des métadonnées de partie, et élimination de 2 tests instables détectés
pendant le cycle QUALIF.

### Added
- **Persistance des métadonnées quiz** (#141) : Le nom, thème, notes, publics ciblés, difficultés, langue et réglages d'affichage TV du quiz sont automatiquement sauvegardés et survivent au redémarrage du serveur. Utile après interruption électrique ou maintenance — la partie redémarre avec le même contexte.
- **Séparation configuration système / jeu** (#150) : Les réglages de jeu (délai par défaut, effet néon) sont maintenant sauvegardés dans un fichier séparé, inclus dans les sauvegardes/restaurations de partie. Migration automatique et transparente à la première exécution.

### Fixed
- **Sécurité : pollution des fichiers tracés par clés API factices** (#143) — `go test` n'écrit plus les clés de test dans les fichiers suivi git. À l'avenir, les suites de test sont isolées en répertoires temporaires.
- **Résilience serveur : paniques goroutines** (#131) — Les crashes dans les connexions WebSocket (buzzers, affichage TV) n'arrêtent plus tout le serveur — la connexion fautive se ferme proprement, les autres continuent.
- **Races sur état WebSocket** (#133) — Concurrence résolue sur les 5 champs critiques du hub (type client, ID joueur, adresse MAC, ID unique, timestamp). `go test -race` vert.
- **Test ConfigPage bloqués en environnement** (#136) — Tests de la page de configuration plus robustes, suppression des boucles infinies lors de la mutation d'objets en contexte de test.
- **Ordre des questions mélangeables** (#149) — Bouton "Mélanger" dans l'éditeur de quiz pour randomiser l'ordre des questions. Confirmation avant action, persistance de l'ordre nouveau, modifiable par glisser-déposer après.
- **TestE2E_GameStateMachine race condition** (#121) — Synchronisation correcte sur `OnStateChange` listener lors du chargement concurrence d'état de jeu. Race détectée en CI lors d'exécution parallèle de multiples suites Go.
- **TestAIJob_ProgressEmittedImmediatelyOnAdminConnect_WhenJobRunning flaky** (#140) — Timing déterministe sur réception du premier progress événement lors de reconnexion admin mid-job. Timer calibré pour éviter faux-positifs sous charge CI.

### Changed
- Optimisation interne backup/restore — les fichiers métadonnées quiz et jeu réglages sont maintenant inclus dans les archives (v6.0.x).

### Breaking Changes
- ⚠️ **`GET/POST /config.json` — sections `game` et `neon_effect` supprimées** (#150) : Si vous aviez du code client qui les consultait directement, migrez vers `GET/POST /game-config.json`. Migration serveur automatique pour les anciennes installations (idempotent, une seule fois au démarrage).

### Technical
- **Contrats** : `game-state.md` mise à jour (persistance GameState), `http-endpoints.md` correction divergence `/game-backup` (ligne 222), nouveaux endpoints `/game-config.json`.
- **Backend** : Indirection chemin config (`config.SetConfigPath`), versionnement `game_state.json` (v1.0.0, premier fichier du projet avec ce système), snapshot thread-safe hub WebSocket, `recover()` par goroutine.
- **Tests** : 31 tests Go configuration (#143), tests race désormais verts en CI (`-race -timeout=20m`, #121), tests persistance GameState (#141), tests ConfigPage (#136).
- **Documentation du contrat OnStateChange concurrence** (#121) : ajout d'un commentaire explicite dans `internal/game/engine.go` documentant les garanties thread-safety sur le callback `OnStateChange` — premier enregistrement du contrat implicite de synchronisation moteur.
- Aucun changement de contrat public, aucun changement d'API pour les fixes #121/#140 — robustesse des tests uniquement.

### Validation
- 2 cycles QUALIF (v6.0.3.14, v6.0.3.15) : 14/9 smoke tests PASS, scénario de migration `config.json` → `game-config.json` (#150) vérifié conforme et idempotent sur 2 démarrages consécutifs, `go test -race -timeout=20m ./...` : 0 FAIL / 0 DATA RACE.

---

## [6.1.2] - 2026-08-09

**bugfix/config-api-key-help** : Aide à la configuration des clés API IA, validation en temps réel au point d'enregistrement, interface simplifiée.

### Added
- **Popup d'aide "?" sur la configuration de clés API** : Bouton accessible sur chaque carte de fournisseur (Claude / Groq) pour guider pas à pas vers la création d'un compte et la génération d'une clé. Utile si vous oubliez comment obtenir une clé.
- **Endpoint de validation `POST /api/ai/validate-key`** : Valide une clé API auprès du fournisseur réel **sans la facturer** (appel gratuit sur `/models`). Retourne trois résultats : `valid`, `invalid_key`, `unreachable`. Cooldown global 2s pour protéger le quota du fournisseur.
- **Dialogue de validation à l'enregistrement** : Lors du clic "Enregistrer" d'une clé API, un dialogue bloquant affiche le verdict de validation. Trois cas :
  - ✅ **Clé valide** → Enregistrement silencieux avec badge "✅ Clé vérifiée"
  - ⚠️ **Clé refusée** → Dialogue "Claude a refusé cette clé" avec options [Corriger] ou [Enregistrer quand même]
  - ⚠️ **Injoignable** → Dialogue "Impossible de joindre Claude" avec options [Réessayer] / [Corriger] / [Enregistrer quand même]
- **Badges tri-état pour statut de clé** : "✅ Clé vérifiée" (vert), "⚠️ Clé non vérifiée" (orange), "⚠️ Aucune clé" (gris) — indicatif du dernier verdict de validation, jamais une garantie cryptographique.

### Changed
- **Sélecteur de fournisseur IA simplifié** : Boutons "Claude (Anthropic)" et "Groq" toujours cliquables (jamais `disabled`). Seule la carte du fournisseur sélectionné s'affiche ; l'autre est masquée. Plus aucune auto-sélection ni bascule silencieuse lors du montage ou de l'enregistrement — choix entièrement manuel.
- **Tooltip précis sur bouton "Générer via IA"** : Le bouton `submit` dans la modale de génération affiche un `title` exact listant **toutes les conditions manquantes** pour valider le formulaire, au lieu d'un message générique.

### Fixed
- **Persistance de l'état de vérification** : Deux nouveaux champs `anthropic_api_key_verified` / `groq_api_key_verified` enregistrent si la clé a passé une validation réussie. Utilisés pour afficher le badge tri-état correct après un rechargement de page.

### Security
- **Coût d'abus gratuit pour test de clé** : Contrairement à l'ancien chemin `POST /api/generate-questions` qui était facturé, `POST /api/ai/validate-key` est gratuit. L'endpoint est protégé par un cooldown global 2s, mais un LAN malveillant peut tester une clé capturée sans coût financier. **Même classe de risque que `/api/generate-questions`** (accès LAN non authentifié) — pas une nouvelle exposition, mais un changement du profil de frein (financier → technique uniquement). À considérer lors d'un déploiement PROD exposé au-delà d'un LAN privé (voir `docs/ADMIN_GUIDE.md` pour détails).
- **Badge "Clé vérifiée" est indicatif, non-certifiant** : Le badge marque que la clé a été acceptée par le fournisseur **une fois dans le passé** — généralement juste après l'enregistrement. Il ne garantit jamais que la clé soit encore valide, ni que la génération de questions réelle aboutira. C'est une aide au diagnostic, pas une barrière de sécurité. Recommandation : gardez votre clé confidentielle et révoquez-la immédiatement si compromise (voir `docs/ADMIN_GUIDE.md` pour détails).

### Technical
- **Backend** : Nouvel `internal/server/ai_validate.go` implémentant `validateAnthropicKey` / `validateGroqKey` via appels `/models` stdlib (`net/http`). Cooldown global `aiValidateCooldownState` sans race. Extension d'`aiProvider` avec méthode `ValidateKey`. Deux champs booléens persistés dans `config.AIConfig` via fusion champ-par-champ existante.
- **Frontend** : Nouveau composant `ApiKeyValidationDialog.jsx` pour dialogue bloquant (refus/injoignable). Séquence normative dans `ConfigPage.jsx` : validation puis sauvegarde, avec option de forçage en cas d'échec. Badges tri-état reflétant le statut persisté. Popup d'aide pointant vers les consoles fournisseur.
- **Contrats** : `contracts/ai-key-validation.md` (entièrement respecté).
- **Tests** : 26 tests Go pour `ai_validate` (200/401/403/429/500/réseau/timeout/cooldown). 10 des 11 scénarios maquette dans `ConfigPage.keyvalidation.test.jsx` vérifiés e2e sur build réel. 3 tests `ConfigPage.apikeyhelp.test.jsx` mis à jour pour nouveaux libellés tri-état.

### Notes de compatibilité
- **Aucun changement BREAKING** : `POST /config.json` accepte les deux nouveaux champs booléens comme une fusion champ-par-champ standard ; les clients qui ne les envoient pas se comportent exactement comme avant.
- **Point mineur — tests vitest bloqués en environnement** : `ConfigPage.keyvalidation.test.jsx` et `ConfigPage.apikeyhelp.test.jsx` bloquent indéfiniment à l'exécution vitest (vérifiés manuellement sur build réel, pas de mock). Recommandation : exécuter sur Windows natif avant merge.
- **Audit sécurité** : Complet avec score 88/100 — deux points MOYENNE documentés, aucun correctif de code requis avant merge.

---

## [6.1.1] - 2026-08-07

Post-QUALIF correctifs (4 fixes critiques & UX, validation complète Groq #142).

### Fixed (Critical)
- **#142 Groq schema rejection** — Groq rejetait le schéma JSON car le validateur comptait `CATEGORY`/`DIFFICULTY` comme candidats discriminants dans l'`anyOf`, bien qu'identiques sur toutes les branches. Nouveau `anyOf discriminator ambiguity` au-delà de toute récréation : débloque complètement la génération Groq après test end-to-end réel (SPEEDY/QCM/MEMORY/MEMOTION/ARDOISE). Backend filtre `enum` sur ces champs pour Groq, valide côté serveur (Anthropic non affecté). (#137 T2.3, post-QUALIF).

### Added
- **Détail technique d'erreur IA (admin-only)** : Nouveau champ `AI_GENERATION_PROGRESS.ERROR_MESSAGE` sur `/ws/admin` (jamais diffusé à TV/VPlayer) — message d'erreur réel du provider (Anthropic/Groq) assaini (clés API supprimées par regex). Frontend affiche ce message dans un panneau repliable "Détail technique" sous le message générique, seulement quand présent (état FAILED). Permet diagnostic immédiat des blocages sans trace de debug (déverrouille #142 avec visibilité utilisateur) (#137 T2.3, post-QUALIF).
- **Bouton "Nouvelle génération" sur panneaux terminaux** : Après un job DONE/CANCELLED, recliquer sur "Générer via IA" affichait le résultat final sans action de relance (boucle sans issue). Nouveau bouton "Nouvelle génération" (variant primary) à côté de "Fermer" sur panneaux success/cancelled — réinitialise le formulaire sans fermer la modale, conserve Bloc 2 (catégories, volume, distribution). (#137 T2.3, post-QUALIF).

### Fixed (UI)
- **Couleur sliders MEMORY/ARDOISE (distribution IA)** : `accent-color` CSS provoquait un rendu natif Chromium où la piste non remplie devenait noire pour les couleurs lumineuses (MEMORY `#2e9e6d` luminance 130, ARDOISE `#10b981` luminance 145 — au-delà du seuil de l'heuristique Chromium). Fix : abandon d'`accent-color`, habillage manuel du slider avec dégradé linéaire `var(--type-color)` jusqu'à `var(--pct)`, curseur redessiné `::-webkit-slider-thumb`/`::-moz-range-thumb`. Robuste à futurs types. (#137 QUALIF retour).

### Technical
- **Backend** : Contrats amendés (contract-first), `AI_GENERATION_PROGRESS.ERROR_MESSAGE` field, `sanitizeUpstreamMessage()` (redaction clés par regex), extraction et threadage message par Anthropic/Groq, validation `DIFFICULTY` serveur-side, fix `groqProvider.AdaptSchema()` retrait `enum` CATEGORY/DIFFICULTY. 11 commits, 8 tests nouveaux. (#137 post-QUALIF).
- **Frontend** : Display `ERROR_MESSAGE` en panneau repliable `.ai-error-detail` sur FailedBody, bouton "Nouvelle génération" sur success/cancelled, mapping `MSG.ERROR_MESSAGE` → `aiJob.errorMessage`. 2 commits, 3 tests nouveaux. (#137 post-QUALIF).
- **Vérification** : Build OK, tests Go/React 100% (52 Go, 67 React verts). Appels Groq réels (SPEEDY/QCM/MEMORY/MEMOTION/ARDOISE) validés. Non-régression Anthropic testée explicitement.

### QA Notes
- Rendu visuel slider vérifié par Chrome headless avant/après (bug reproduit puis résolu). Firefox à tester en QUALIF (headless = Chromium/Blink uniquement, pas Firefox).
- Génération Groq end-to-end validée côté API réelle. Retester en QUALIF pour confirmation finale.
- Tous les 4 correctifs VALIDATED, aucune réserve (à l'inverse du Batch 2b initial #137).

---

## [6.1.0] - 2026-08-07

Batch 2b #137 : Multiples publics/difficultés, objectif de partie caché, filtres affichage TV par champ.

### Breaking Changes
- **Ancien format singulier rejeté** : `POST /api/generate-questions` refuse désormais `population` (singulier) et exige `populations` (tableau). Même traitement pour `difficulty` → `difficulties`. Anciens clients (v6.0.x) doivent être mise à jour (#137 Batch 2b T2.1).
- **Métadonnées GameState renommées pluriel** : `QUIZ_POPULATION` / `QUIZ_DIFFICULTY` remplacés par `QUIZ_POPULATIONS` / `QUIZ_DIFFICULTIES` (tableaux). Ancien format dans un client `UPDATE_QUIZ_META` est ignoré (sémantique "absent = inchangé" appliquée strictement par champ) (#137 Batch 2b).

### Added
- **Sélection multiple populations/difficultés** : Interface admin (QuestionsPage section Quiz) passe de champ texte unique à checkboxes multiples. Permet combiner "Junior + Ado" ou "Facile + Moyen" pour une même partie. Pré-remplissage génération IA depuis les sélections globales (#137 Batch 2b T2.1).
- **Objectif pédagogique de partie (QUIZ_OBJECTIVES)** : Nouveau champ texte libre dans admin (QuestionsPage section Quiz). Jamais transmis aux clients `/ws/tv` ou `/ws/player` (confidentialité — les joueurs ne voient pas l'objectif pédagogique). Transmis à `/ws/admin` uniquement (#137 Batch 2b T2.1).
- **Filtres affichage TV par champ (QUIZ_HIDDEN_FIELDS)** : Interrupteurs "Afficher sur la TV" pour chaque champ (ex: toggle "Réponse" pour masquer/afficher `ANSWER` en phase REVEALED). Nouveau champ `QUIZ_HIDDEN_FIELDS` (tableau de strings) transmis à tous les endpoints web. Clients `/ws/tv` et `/ws/player` appliquent le filtrage côté rendu (pas le serveur) (#137 Batch 2b T2.1).
- **Payload UPDATE_QUIZ_META étendue** : 4 nouveaux champs (`POPULATIONS`, `DIFFICULTIES`, `OBJECTIVES`, `HIDDEN_FIELDS`). Sémantique "absent = inchangé" appliquée **par champ** : rétrocompatibilité stricte avec clients v6.0.x (#137 Batch 2b T2.1).

### Changed
- **Popup modale génération IA simplifiée** : Bloc "Paramètres du Quiz" passe de 3 champs (POPULATION, DIFFICULTY, LANGUAGE) à affichage informatif uniquement (non modifiable, copié depuis l'état global). Bloc "Cette génération" reçoit ses propres champs de multiples (checkboxes pour POPULATIONS/DIFFICULTIES) — génération précise sans affecter le global (#137 Batch 2b T2.3).
- **Fraîcheur des questions générées** : Corrections de concurrence et gestion état — `POST /api/generate-questions` avec `OnQuestionUpload()` broadcast garantit fraîcheur immédiate en TV/admin sans race condition (#137 Batch 2b T2.3).

### Fixed
- **#142 Groq API** : Génération réelle Groq cassée (timeout/dégradation qualité). Non bloquant pour ce chantier — note de transparence dans docs (#137 Batch 2b section "Réserves QA").
- **Rendu visuel écran TV et bandeau "modifications non enregistrées"** : Non vérifiés en environnement QA (pas d'accès navigateur) — à valider par inspection humaine avant QUALIF/PROD (#137 Batch 2b section "Réserves QA").

### Technical
- **Backend** : Modifications modèles GameState (`QUIZ_POPULATIONS`, `QUIZ_DIFFICULTIES`, `QUIZ_OBJECTIVES`, `QUIZ_HIDDEN_FIELDS` pluriels/nouveaux), payload `UPDATE_QUIZ_META` 6→10 champs, routage diffusion par endpoint (`/ws/admin` vs `/ws/tv` + `/ws/player`) pour masquage `OBJECTIVES` (#137 Batch 2b T2.2).
- **Frontend** : QuestionsPage section Quiz révisée (checkboxes multiples, toggle masquage champs), modale génération IA redessinée (affichage vs éditable), PlayerDisplay (TV NEW_GAME) applique filtrage `HIDDEN_FIELDS` côté rendu (#137 Batch 2b T2.3).
- **Documentation** : DATA_MODELS.md, WEBSOCKET_PROTOCOL.md, ADMIN_GUIDE.md mises à jour. Workflow admin documenté : objectif de partie jamais visible joueurs, toggles "Afficher TV" par champ (#137 Batch 2b T3.1).

### QA Notes
- **Réserves QA validées** : Rendu visuel TV et bandeau UI non vérifiés en QUALIF (pas d'accès navigateur QA). À valider par inspection humaine. Bug indépendant #142 (Groq API) : jamais bloquer ce chantier (#137 Batch 2b).

---

## [6.0.0] - 2026-08-05

**Générateur de questions via IA** : nouveau bouton « ✨ Générer via IA » dans QuestionsPage ouvrant une modale de paramétrage. Le backend appelle l'API Claude (Anthropic) en sortie structurée et écrit directement de nouvelles questions sur disque, **en création uniquement**. Précédé d'un correctif critique sur `POST /config.json` devenu additif (sauvegarde partielle préserve maintenant les autres sections, au lieu de les remettre à zéro).

### Added
- **Bouton « ✨ Générer via IA » dans QuestionsPage** : nouvelle modale de paramétrage avec 2 blocs distincts — Paramètres du Quiz (pré-remplis depuis les globaux) et Cette génération (éditable). Formule : population cible, difficulté(s), thème, objectifs optionnels, catégories, volume (nombre ou durée), répartition par type (SPEEDY/QCM/MEMORY/MEMOTION avec sliders rebalancés). Validation formulaire côté client, spinner en cours de génération, gestion états erreur/succès (#8).
- **POST /api/generate-questions** — endpoint synchrone longue durée (1–3 min) générant des questions par lot via l'API Claude (BYOK — Bring Your Own Key). Réponse : `{ status: "ok", created: [{id, type, category}, …], created_count, skipped_count, skipped_reasons }`. Codes stables : 200, 400 (validation), 405, 409 (pas de clé), 502 (erreur amont), 504 (timeout), 507 (IDs saturés). Déclenche obligatoirement `OnQuestionUpload()` → broadcast WebSocket (#8).
- **Section `ai` dans config.json** — nouvelle clé top-level avec `anthropic_api_key` (BYOK, masqué en GET), `model` (défaut `claude-opus-5`), `timeout_seconds` (300), `max_questions` (200). Remplie via ConfigPage (nouvelle section IA) ; `GET /config.json` retourne toujours `anthropic_api_key: ""` + booléen dérivé `api_key_configured` pour contrôler l'activation du bouton (#8).
- **Trois champs globaux au quiz (GameState)** : `QUIZ_POPULATION` (Junior/Ado/Adulte/Senior/Famille), `QUIZ_DIFFICULTY` (Facile/Moyen/Difficile/Expert), `QUIZ_LANGUAGE` (Français défaut). Éditables dans QuestionsPage bloc Quiz ; affichés sur TV écran NEW_GAME en ligne compacte de badges. Pas `omitempty` (règle projet). Pré-remplissent le formulaire de génération (#8).
- **Action WebSocket UPDATE_QUIZ_META étendue** — payload passe de 3 à 6 champs (ajout POPULATION/DIFFICULTY/LANGUAGE). Sémantique normative : **absent = inchangé** (jamais vidé). Élimine le risque qu'un client antérieur effectue accidentellement une régression (#8).

### Changed
- **POST /config.json — correctif bug destructif** : le handler désérialisait le corps dans un `config.Config` vide puis réécrivait le fichier entier. Aucune section ne portant `omitempty`, toute section absente du payload était remise à zéro. ConfigPage n'envoyant que des payloads partiels (`{neon_effect}`, `{server}`), chaque « Enregistrer » détruisait les autres sections. Désormais **additif** : merge sur `config.Get()` existant, ré-application des défauts, écriture atomique. *Dégât constaté en production : `config.json` porte `questions_dir` et `files_dir` vides, compensés par chemin codé en dur dans `main.go` — corrigé comme effet secondaire du fix* (#8).
- **Allocation d'ID de question verrouillée** — `findFreeQuestionID` balayait `1..999` **sans verrou** (race pré-existante : deux uploads simultanés pouvaient obtenir le même ID) et repliait sur `"999"` en cas de saturation, écrasant la question 999. Désormais : mutex sur `HTTPServer`, réservation exclusive par `os.Mkdir`, et `507` en cas de saturation. S'applique aussi à `handleUploadQuestion` (#8).

### Technical
- **Backend** : nouveaux fichiers `internal/server/ai_client.go` (client Anthropic, streaming, structured outputs) et `internal/server/ai_generator.go` (schéma validation, mapping types, allocation IDs) ; modification `internal/config/config.go` (struct AIConfig, Save(), mutex), `internal/server/http.go` (handleConfig additif, POST /api/generate-questions), `internal/game/models.go` (3 champs GameState), `internal/game/engine.go` (SetQuizMeta 6 params), `internal/protocol/messages.go` (QuizMetaPayload), `cmd/server/main.go` (dispatch). Nouvelle dépendance : `github.com/anthropics/anthropic-sdk-go` (#8).
- **Frontend** : ConfigPage (+CSS) section IA ; QuestionsPage (+CSS) section Quiz étendue + bouton ; nouveau composant `AIGenerateModal.jsx` (+CSS) ; PlayerDisplay (+CSS) écran NEW_GAME ; `useWebSocket.js` (3 champs gameState) (#8).
- **Tests** : 21/21 tests Go PASS (config merge, allocation IDs concurrence, mapping types, validation questions). React `AIGenerateModal.test.jsx` 36/36 PASS (rebalance sliders, machine à états modale). Procédure manuelle QA : génération réelle + écran TV NEW_GAME saturé — **reportés à QUALIF** (pas de clé API fournie en sandbox, pas d'accès navigateur) (#8).

### Notes de compatibilité

- **Aucun changement BREAKING au sens strict** — aucune action, aucun champ existant supprimé/renommé/retypé. Tous les ajouts sont additifs.
- **Trois points appellent validation explicite utilisateur au GATE 2** :
  1. Correctif `POST /config.json` **change le comportement observable** d'un endpoint existant (sauvegarde partielle ne réinitialise plus le reste).
  2. Sémantique « absent = inchangé » sur `UPDATE_QUIZ_META` modifie le traitement d'un payload déjà émis par QuestionsPage.
  3. Schéma de sortie LLM pour **MEMORY et MEMOTION diverge légèrement de la spec initiale** : ce sont les structures réelles du code qui font foi (cf. `contracts/ai-generation.md` §5).

---

## [5.10.0] - 2026-08-04

Milestone `v5.10.x - Stabilité VJoueur` complet : 5 correctifs cumulés (#127, #129, #130, #134, #132) réduisant drastiquement le volume de broadcasts WebSocket vers les VJoueurs et fiabilisant la détection de liens morts, plus une nouvelle fonctionnalité de libération de siège.

### Fixed
- **Broadcasts LED non ciblés vers TV/VJoueur** : 5 fonctions LED (`broadcastLEDSet`, `sendLEDSetStop`, `sendLEDSetReveal`, `sendLEDSetToTeam`, `sendLEDSetComet`) envoyaient un broadcast complet à tous les clients au lieu de ciblage buzzers seuls. Corrigé par même technique que #127/#129 (événement ciblé). Bonus : 2 fonctions (`broadcastLEDSet`, `sendLEDSetToTeam`) identifiées sans appelant — code mort, signalé pour nettoyage futur (#132).

### Added
- **Libération de place d'un VJoueur connecté** : animation peut désormais libérer la place d'un joueur actuellement connecté via action `RELEASE_BUMPER_NAME`, invoquant un motif distinct `SEAT_RELEASED`. Contrairement à la suppression totale (score perdu), la libération conserve le score et l'équipe, autorisant le joueur à reprendre sa place dans une fenêtre ~5 min. Utilité : réassignation rapide du siège sans perte de données (#134).
- **Motif d'éviction `SEAT_RELEASED` documenté** : nouveau motif d'éviction accompagnant l'action #134. Documenté dans `docs/PROTOCOLS.md` avec distinction claire vis-à-vis de `PLAYER_REMOVED` (score perdu). Tableau de motifs complétant les évictions v5.9.0 (#120) (#134).

### Fixed
- **Détection lien mort VJoueur accélérée et paramétrable** : seuil de liaison morte passé de 9-10 s à 4-4,5 s via nouveau champ `DEAD_LINK_TIMEOUT_MS` du message `HEARTBEAT`. Serveur transmet désormais directement le seuil (4000 ms) au lieu d'une constante dupliquée côté client. Gain secondaire : tolérance ping/pong améliorée (2 pertes acceptées au lieu de 0) grâce au recalibrage de la cadence serveur 3000 ms → 2000 ms et du `ReadDeadline` 5000 ms → 7000 ms. Détection efficace inversée délibérément : client détecte avant serveur (4 s vs 7 s) pour reprendre l'initiative sur liens morts (#130).
- **Protocole HEARTBEAT documenté** : jamais documenté auparavant. Ajout complet dans `docs/PROTOCOLS.md` avec cascade de repli côté client, tableau des valeurs contractuelles et garanties de compatibilité rétroactive (#130).
- **Déconnexions VJoueur résiduelles à la transition PREPARE→READY** : rafale de broadcasts non groupée émis pendant la phase de préparation causait un débordement de messages WebSocket, forçant la déconnexion des VJoueurs par timeout. Correction : filtrage payload contextualisé par phase — pendant PREPARE/READY, le VJoueur reçoit uniquement son bumper (non la carte complète), réduisant le volume de ~85% (mesure : 12→2 messages par VJoueur sur la fenêtre critique) (#127).
- **Correction complémentaire : affichage "Joueurs" persistant pendant PREPARE/READY** : regression trouvée en revue de code — le bandeau "Joueurs" s'affichait durant ces phases alors qu'il devrait rester caché jusqu'à STARTED. Correction sur le frontend (#127).
- **Rafales de broadcasts non filtrées vers VJoueurs hors PREPARE/READY** : trois événements (connexion/déconnexion participant, saisie ARDOISE soutenue, buzz/réponse QCM) déclenchaient un `UPDATE` complet vers tous les VJoueurs sans distinction. Correction : ciblage par événement — chaque message désormais adressé uniquement aux destinataires pertinents (Admin/TV/buzzers selon l'événement), avec écho ciblé au seul participant concerné quand celui-ci a besoin de l'écho de son propre état. Gain mesuré : 0 message inutile vers les VJoueurs non-participants (#129).
- **Correction d'équité : fuite de saisie ARDOISE des autres équipes** : le texte saisi par les autres équipes en phase ARDOISE n'est plus transmis aux navigateurs des VJoueurs (gating par événement, non par payload) (#129).
- **Regroupement des broadcasts ARDOISE côté serveur** : réduction de la contention du moteur sur saisies soutenues via fenêtre temporelle ≤ 150 ms (#129).
- **TV retirée du per-PONG résiduel en phase PREPARE** : optimization — les écrans TV n'en avaient pas besoin (progression « prêt » affichée via libellé statique) (#129).

---

## [5.8.2] - 2026-08-01

Enhancement UI : réponses ARDOISE affichées en grille responsive plutôt qu'en liste verticale sur la page admin, réduisant la hauteur occupée.

### Changed
- **Panneau ARDOISE en grille responsive** : les réponses s'affichent désormais en grille CSS avec `grid-template-columns: repeat(auto-fill, minmax(178px, 1fr))` au lieu d'une liste verticale. Gain mesurable : 6 équipes → 2 rangées (au lieu de 6 lignes), 16 équipes → 4 rangées (au lieu de 16 lignes). Pantalla admin gagne significativement en verticalité (#116).
- **Rang renforcé, première réponse distinguée** : le badge de rang passe d'un fond translucide à un fond plein (`rgba(99, 102, 241, 0.9)`) avec texte blanc, taille augmentée. La première réponse (rang 0) porte une bordure et une `box-shadow` inset accentuée pour être repérable immédiatement entre cellules côte à côte (#116).
- **Réponses longues jamais tronquées** : `overflow-wrap: anywhere` sur `.ardoise-answer-text` réduit efficacement la taille minimale de cellule en contexte de grille CSS (contrairement à `word-break: break-word`), évitant qu'une chaîne d'un seul tenant n'impose la largeur complète d'une colonne. Le bouton d'attribution de points reste ancré en haut (`align-items: flex-start`) sur une réponse multi-ligne (#116).

### Technical
- **Frontend** : `.ardoise-answers-list` convertie à `display: grid` avec `minmax(178px, 1fr)` et `gap` conservé. `.ardoise-answer-row` non modifiée (reste une cellule de grille). Classe conditionnelle `rank-first` ajoutée en JSX sur rang 0. Aucune modification de `sortedArdoiseEntries` (#117) ni de `formatArdoiseDelay` (#117) — non-régression vérifiée.
- Tests : 62/62 PASS (tests existants couvrent grille + non-régression #117)

---

## [5.9.3] - 2026-07-31

Bugfix : un joueur ayant perdu son identifiant local ne pouvait plus jamais reprendre son pseudo — protection anti-vol de #109 devenue un mur permanent. L'animateur peut désormais lui rendre sa place avec score conservé, ou la libérer définitivement.

### Fixed
- **Joueur bloqué définitivement sans ID** : un VJoueur ayant perdu le stockage localStorage (changement d'appareil, vidage du cache, navigation incognito) ne pouvait plus se reconnecter avec son ancien pseudo — la protection anti-theft #109 (`NAME_TAKEN` si nom déjà porté sans ID resolvable) le fermait indéfiniment. Correction : nouveau motif `NAME_TAKEN_OFFLINE` (détenteur déconnecté) distinct de `NAME_TAKEN` (connecté), signalant à l'animateur qu'une **reprise assistée** est possible (#122).
- **Bouton × unique sur chaque VJoueur** : chaque fiche VJoueur (y compris désormais dans la liste des membres d'équipe) porte un bouton × qui ouvre un **dialogue de confirmation** (ReclaimConfirmModal). Ce dialogue propose deux actions dont l'ordre par défaut dépend du statut d'équipe : **Réinscription** (conserve score/équipe, autorisation ~5min usage unique) pour les joueurs assignés, **Suppression totale** (réutilise #123, score perdu) pour les non-assignés. L'autre action reste toujours accessible dans le même dialogue (#122 cycle 2/3).
- **Piège inscriptions fermées affiché dans le dialogue** : l'avertissement « Inscriptions fermées : {nom} ne pourra pas revenir après une suppression totale » s'affiche sous l'option Suppression totale du dialogue, quelle que soit sa position. Élimine le doute tactile (pas de tooltip au doigt sur tablette, qui n'existe que au survol) (#122 cycle 2/3).

### Changed
- **Motif `NAME_TAKEN` inchangé** : un homonyme connecté retourne toujours `NAME_TAKEN` (message inchangé). Seul le détenteur déconnecté retourne `NAME_TAKEN_OFFLINE` (nouveau message, signale la reprise possible) — non-régression #109 stricte (#122).
- **Geste unique + dialogue intelligent** : un seul bouton × sur chaque fiche VJoueur, ouvre un dialogue proposant les deux actions (Réinscription ou Suppression) dont l'ordre par défaut dépend du statut d'équipe. Les deux options restent toujours cliquables (jamais masquées), la confirmation nomme le joueur. Simplifie le geste de l'animateur comparé à deux boutons distincts (#122 cycle 2/3).

### Technical
- **Backend** : nouveau champ `Bumper.ReclaimRequested` (bool, **sans omitempty**, sérialisé). `Bumper.reclaimAuthorizedUntil` (time.Time, interne non-sérialisé, même convention que `greenSince`). B1 : `NAME_TAKEN_OFFLINE` posé selon `cand.Connected`. B2 : `ReclaimRequested = true` au rejet offline, retiré lors reconnexion normale par ID. B3 : `ReleaseBumperName(id)` pose l'autorisation (TTL ~5min, `var` pas `const` pour testabilité), `reattachVirtualPlayerUnsafe` le consomme (réattachement score/équipe conservés, nouvel ID généré, usage unique garanti par `reclaimAuthorizedUntil` expiré).
- **Frontend** : nouveau composant `ReclaimActions` (chip + 2 boutons + sous-titres + avertissement) utilisé dans les deux zones. `REJECTION_MESSAGES.NAME_TAKEN_OFFLINE` nouveau, `NAME_TAKEN` inchangé. `TeamsPage.jsx` propose `releaseBumperName(id)` en plus de `deleteBumper(id)`, avec confirmation nommée. #124 couvert sans tâche dédiée — quelle que soit la cause de la perte d'ID (changement d'appareil, cache vidé), la reprise assistée offre une voie de retour.
- Tests : rattachement score/équipe conservés, usage unique garanti (concurrence 10 goroutines → exactement 1 succès, `-race`), TTL/expiration, avertissement inscriptions fermées présent/absent, non-régression `NAME_TAKEN` stricte (texte exact inchangé).

---

## [5.9.2] - 2026-07-30

Bugfix : suppression d'un VJoueur depuis l'admin n'était jamais notifiée au joueur, et message « nouvelle partie » s'affichait à tort après rechargement. Cause trouvée en validation manuelle sur QUALIF de #120/#118.

### Fixed
- **VJoueur supprimé sans notification visible** : l'action `DELETE_BUMPER` était correctement codée côté interface mais **jamais émise** (l'interface utilisait une UPDATE générique du roster au lieu de l'action dédiée). Correction : nouvelle fonction `handleDeleteBumper` dans `TeamsPage.jsx` émet maintenant `DELETE_BUMPER`, tandis qu'en parallèle le serveur notifie aussi tout changement de roster par constat de disparition (B1, `handleFullUpdate`) — double filet pour éviter qu'un correctif futur de l'interface ne ré-introduise le bug (#123).
- **Motif incohérent après rechargement** : le joueur reçoit maintenant le motif d'éviction initial, consulté via le registre borné (`EvictionRegistry`) du serveur au moment de la reconnexion avec l'ID connu. Élimine les messages « nouvelle partie » affichés à tort (R2, cause B du plan) — `ENROLLMENT_CLOSED` retrouve son sens littéral (« inscriptions fermées ») (#123).
- **Filet SESSION_EXPIRED de #118 redevient opérationnel** : l'état `bumper` n'était jamais remis à `null` à la disparition du roster (seule la ref était nettoyée), laissant le filet bloqué indéfiniment. Correction : `bumperRef` ET React state (`setBumper(null)`, `setTeam(null)`) remis à zéro ensemble, restaurant la garde du filet (R3 du plan #118) (#123).

### Changed
- **Registre borné pour mémorisation des motifs** : nouveau `EvictionRegistry` (TTL ~1h, plafond 500 entrées) côté serveur, consulté à chaque `PLAYER_CONNECT` quand l'ID est fourni. Permet au serveur d'émettre directement le motif vrai même si le joueur s'est reconnecté bien après son éviction (fenêtre d'au moins 1h). Jamais exposé en API — purement interne (#123).
- **Double filet sur notification d'éviction** : l'action `DELETE_BUMPER` reste conscient mais non obligatoire pour notifier (B1 couvre aussi les UPDATE génériques du roster). Élimine la dépendance implicite sur une implémentation d'interface correcte (#123).
- **Suppression de la déduction de motif `ENROLLMENT_CLOSED → GAME_RESET`** : le client ne déduit plus jamais le motif. Le serveur répond désormais directement avec le motif vrai (consulté dans le registre si connu, sinon son code d'erreur littéral). Cohérent avec le design #118/#120 d'éviter les déductions côté client (#123).

### Technical
- **Backend** : nouveau fichier `server-go/internal/server/eviction_registry.go`, structure `EvictionRegistry{registry map[string]Entry, mu sync.Mutex, lastClean time.Time}`. Méthodes nil-safe (`Record`, `Lookup`, `Reset`). `handlePlayerConnect` consulte le registre avant d'appeler le moteur ; `handleFullUpdate` compare les rosters et notifie les disparitions ; `handleDeleteBumper` et `NEW_GAME` enregistrent les motifs pour consultation ultérieure. Aucune API ne change — le registre est entièrement interne.
- **Frontend** : `TeamsPage.jsx` utilise `deleteBumper(id)` au lieu de reconstruire le roster. `VPlayerPage.jsx` remise les deux états `bumper` et `team` à `null` (pas juste la ref) quand le bumper disparaît du roster. Suppression de la condition `reason === 'ENROLLMENT_CLOSED' → 'GAME_RESET'` sur le chemin `handlePlayerConnect` rejected.
- Tests : test central via `handleUpdate` (chemin réel, pas `handleDeleteBumper` direct — l'angle mort qui avait laissé passer #123 sur #120), registre Record/Lookup/TTL/plafond/reset, non-régression buzzer physique. Frontend 100/100 PASS dont filet `SESSION_EXPIRED` redéclenché (F2 vérifié), scénario 6 (`ENROLLMENT_CLOSED` littéral, jamais `GAME_RESET`).

---

## [5.9.1] - 2026-07-30

Bugfix : VJoueurs restant bloqués après une vraie coupure réseau — détection de liaison morte et reconnexion automatique via battement applicatif.

### Fixed
- **VJoueur bloqué sans le savoir après coupure réseau** : asymétrie du keepalive (R6 cause racine). Le serveur émettait une trame ping protocolaire, mais le navigateur Web y répond automatiquement sans exposer d'événement JavaScript — le client ne détectait jamais la perte et restait sur un socket zombie. Seul un rechargement de page rétablissait la liaison. Correction : battement applicatif `HEARTBEAT` (serveur→client) en complément du ping, dont le client **dérive le seuil** (3 × cadence) plutôt que de le coder en dur. Détection automatique au-delà du seuil, fermeture du socket, reconnexion en arrière-plan avec dispersion ±30 % (#118).
- **Réassociation immédiate à la reconnexion** : dès que la WebSocket est établie, le client envoie `PLAYER_CONNECT` immédiatement (avec l'ID de bumper connu), au lieu d'attendre 2s. Élimine la fenêtre d'échec sur lien instable où les 2s ne s'écoulaient jamais dans une même fenêtre établie (R4) (#118).
- **Buzz pendant coupure réseau** : un appui buzzer pendant une perte de liaison est mis en file d'attente (1 seul appui retenu, mémoire seule). À la reconnexion, le buzz est envoyé si la question est la même et toujours en phase `STARTED` ; sinon il est abandonné silencieusement. Élimine les doubles buzzs ou les buzzs sur une question suivante après une coupure qui aurait couvert toute une transition (R2) (#118).

### Changed
- **Bandeau de statut de connexion** : nouveau bandeau optionnel affichant « Connexion perdue — reconnexion… » (orange, avec pulsation du point) lors d'une détection de liaison morte, puis « Connexion rétablie » (vert, 2s) lors de la reconnexion réussie. Distinct du bandeau d'éviction (#120), qui prime sur celui-ci. Le bouton buzzer n'est jamais désactivé — il accepte les appuis pendant une perte de liaison (#118).
- **Motif générique sur reconnexion fermée** : si le serveur rejette une reconnexion avec `ENROLLMENT_CLOSED` (partie en cours de réinitialisation), le motif est mappé vers `GAME_RESET` (via le mécanisme d'éviction #120) plutôt qu'un écran `reconnectError` local. Affiche le même bandeau de motif explicite qu'une éviction (#118).
- **Motif purge `NEW_GAME`** : renvoie désormais `GAME_RESET` au lieu de laisser le client déduire une éviction, cohérent avec #120 (#118).

### Technical
- **Backend** : `HEARTBEAT { INTERVAL_MS }` (action `ActionHeartbeat`) émise par `writePump` sur un ticker existant (3s), en complément du ping protocolaire. Cadence toujours dérivée de la constante `writePumpTickPeriod`, jamais dupliquée en dur. Livré aux trois endpoints web (`/ws/admin`, `/ws/tv`, `/ws/player`), pas au hub buzzer physique. Jamais lue — aucun retour attendu, ne transite pas par le canal `Incoming`.
- **Frontend** : `useWebSocket.js` F1 (surveillance) — `lastMessageAtRef` mis à jour sur tout message (ping inclus). `heartbeatIntervalMsRef` alimenté par le `HEARTBEAT`, seuil dérivé (`INTERVAL_MS × 3`), repli 3000ms avant premier `HEARTBEAT`. Effet de surveillance unique (1s), nettoyé au démontage. F5 (dispersion) — `nextReconnectDelay()` appliquée aux deux chemins (onclose normal, `closeZombieSocket`), ±30% sur `RECONNECT_INTERVAL`. `closeZombieSocket()` neutralise les handlers **avant** `close()` — critique pour débloquer la garde de `connect()` et permettre une reconnexion immédiate.
- **Frontend** : `VPlayerPage.jsx` F2 (réassociation) — `PLAYER_CONNECT` envoyé immédiatement dès que `status === 'connected'` avec `playerSession.id` connu, reprise à 2s. F7 (file buzz) — `pendingBuzzRef` garde `question.ID` capturé, double condition de purge : passage observé en `PREPARE` (toujours éjecté) ; validation au moment de la reconfirmation du bumper (sameQuestion && stillStarted, sinon abandon silencieux). Aucun horodatage client ajouté.
- Tests : 8 tests backend `HEARTBEAT` (timing réel ~24s, cadence 3s, coexistence ping protocolaire, 3 types clients, aucune lecture retour, non-régression buzzer physique) ; 68 tests React validant surveillance et dérivation du seuil, réassociation immédiate, dispersée, file buzz (scénario critique : offline pendant tout changement de question → abandon, puis buzz normal fonctionne question suivante).

---

## [5.9.0] - 2026-07-29

Bugfix : VJoueur renvoyé sans explication à l'inscription — notifie désormais explicitement le motif de suppression ou d'éviction, et corrige la race qui rendait ce renvoi parfois immédiat.

### Fixed
- **VJoueur renvoyé silencieusement à l'inscription** : une race entre l'acceptation du serveur et la diffusion du roster causait le renvoi du client vers la saisie de pseudo sans motif visible. Le client ne distinguait pas « pas encore reçu » de « supprimé ». Correction : le serveur notifie désormais explicitement l'éviction via `PLAYER_EVICTED { REASON }`, émis au client concerné avant les broadcasts généraux, fermant la race par construction (#120).
- **Renvoi sans message** : affiche un bandeau avec le motif explicite (« Joueur supprimé », « Partie réinitialisée », « Session expirée », ou générique). Le joueur supprimé peut se réinscrire avec son pseudo sans être refusé en `NAME_TAKEN` (identité comparée par ID, pas par nom) (#120).
- **Persistance roster atomique** : `SaveBumpers` était vulnérable à des écritures concurrentes qui pouvaient laisser un fichier vide ou partiel. Corrigée au pattern fichier temporaire + `os.Rename` atomique, appliquant le durcissement déjà validé sur `SaveTeams` (#113 B4) (#120).

### Changed
- **Identité VJoueur par ID au lieu du pseudo** : le serveur (#109 R1) base l'identité sur l'ID stocké côté client ; le client (VPlayerPage/EnrollPage) utilise désormais ce même ID pour valider l'appartenance (`bumpers[playerSession.id]`), avec repli sur la recherche par nom pour sessions antérieures. Élimine les faux « joueur absent » dus à des divergences de pseudo (casse, espaces) (#120).
- **Motifs d'éviction explicites** : 8 codes (4 historiques du rejet d'inscription + 3 évictions + générique) avec messages localisés. Les 4 refus historiques (`ENROLLMENT_CLOSED`, `INVALID_NAME`, `NAME_TAKEN`, `LIMIT_REACHED`) conservent leur chemin d'affichage (#109) ; les 3 nouveaux (`PLAYER_REMOVED`, `GAME_RESET`, `SESSION_EXPIRED`) passent par `PLAYER_EVICTED` (#120).
- **Nettoyage de session mutualisé** : `vplayer_name`, `vplayer_session`, `vplayer_id` toujours effacés ensemble sur tous les chemins (rejet, éviction, timeout 10s) via `clearVPlayerSession()` utilitaire unique (#120).

### Technical
- **Backend** : action `PLAYER_EVICTED { REASON }` (string, motif explicite), émise ciblée (WebSocket direct au client VJoueur, jamais broadcast). Deux points d'émission : suppression par l'animateur (`handleDeleteBumper`, `PLAYER_REMOVED`) ; purge `InitGame`/`NEW_GAME` (`GAME_RESET` par bumper purgé). `InitGame()` retourne `[]string` (IDs purgés) — l'appelant dans `main.go` les notifie.
- **Frontend** : `useWebSocket` expose `playerEvictedStatus` (texte motif) ; `VPlayerPage` et `EnrollPage` affichent bandeau `.enroll-redirect-banner` en haut, avec variante visuelle (couleur/icône) selon motif. Redirection automatique 3s après (délai de lecture configurable `RECONNECT_ERROR_REDIRECT_DELAY_MS`).
- **Filet de sécurité** : si le bumper reste `null` pendant 10s malgré une connexion WebSocket établie et aucune éviction en cours, le client auto-arme `SESSION_EXPIRED` et se redirige (mitigation des cas où une éviction n'aurait pas transitée) (#120).
- Tests : 13 tests Go validant l'ordre des messages (`PLAYER_EVICTED` avant broadcast), l'émission ciblée, la purge sur `NEW_GAME` ; 32 tests React validant la disparition de la détection par balayage de roster, l'affichage du bandeau pour chaque motif, la course résiduelle minuteur×éviction (fast-follow), le nettoyage mutualisé sur tous les chemins.

---

## [5.8.1] - 2026-07-27

Bugfix : réponses ARDOISE (saisie libre) triées par ordre chronologique d'arrivée avec affichage du délai de réponse.

### Fixed
- **Ordre des réponses ARDOISE** : réponses affichées dans l'ordre chronologique de réception (premier caractère non vide) au lieu de l'ordre de la liste d'équipes. Ajout du champ `ArdoiseAnswer.STARTED_AT` (timestamp microseconde du premier caractère, figé à la première saisie) — le tri utilise ce timestamp dans le panneau admin (#117).
- **Délai de réponse visible** : nouveau badge affichant le délai entre le départ de la question et le premier caractère saisi, en secondes avec 3 décimales (ex: `4.732 s`), homogène avec les temps de réaction buzzer. Visible uniquement dans le panneau admin ARDOISE, scope limité aux réponses reçues (équipes sans réponse ne portent pas de délai) (#117).
- **Premier caractère ARDOISE émis immédiatement** : la première frappe est désormais envoyée au serveur sans délai (synchrone), au lieu d'attendre 200 ms sans frappe. Les frappes suivantes restent régulées par le debounce 200 ms. Aucun changement de format de message — seule la cadence d'émission du **premier** caractère change (#117).

### Technical
- **Backend** : nouveau champ `ArdoiseAnswer.STARTED_AT` (int64 microseconde, jamais `omitempty`), figé lors du premier appel non-vide à `SetArdoiseAnswer`. Réarmé uniquement au changement de question.
- **Frontend (VPlayerPage.jsx)** : nouveau flag `ardoiseFirstSentRef` qui déclenche un envoi synchrone pour la première frappe, puis remise à `false` lors du changement de question ou passage en phase `PREPARE`/`STARTED`.
- **Frontend (GamePage.jsx)** : nouveau `useMemo sortedArdoiseEntries` triant par `STARTED_AT` croissant (repli sur `SUBMITTED_AT` si `STARTED_AT = 0`), avec fonction `formatArdoiseDelay()` calculant `(STARTED_AT - gameTime) / 1000000` en secondes. Aucune modification TV — périmètre limité à l'admin.
- Tests : 9 tests backend validant gel sur premier caractère, repli sur `SUBMITTED_AT`, réarmement au changement de question ; 4 tests frontend validant l'émission immédiate du premier caractère et le repli sur second caractère après vidage.

---

## [5.8.0] - 2026-07-26

Release PROD consolidant 3 tickets du milestone v5.8.x (livrés séparément en QUALIF sous les
versions intermédiaires 5.7.24/5.7.25, renumérotés en 5.8.0 au tag PROD conformément à la
convention de versioning du projet).

### Added
- **Palette 16 couleurs d'équipe** : passe de 8 à 16 teintes déclinées en ton vif (saturation 100%, luminosité ~55%) et ton profond (saturation 100%, luminosité ~35%). Attribution déterministe et sans doublon : rangs 1-8 (tons vifs), rangs 9-16 (tons profonds), puis recyclage à rang 1 au-delà de 16 équipes (#113).
- **Field `Team.COLOR_NAME`** : identifiant de palette écrit par le frontend à chaque sélection manuelle ou attribution automatique de couleur. Permet résolution LED exacte du buzzer (aucune approximation par teinte). Rétrocompatible : équipes antérieures restent jouables (fallback teinte) (#113).
- **Sélecteur 16 couleurs** : grille 8×2 avec marquage « déjà prise » sur couleurs assignées à d'autres équipes (reste cliquable), navigation clavier via `:focus-visible` (#113).

### Changed
- **Atténuation LED relative au ton** : nouvelle fonction `dimIntensityFor(rgb)` calcule intensité depuis luminosité HSL — ~64 pour tons vifs (L≈55%), ~100 pour tons profonds (L≈35%), borné [64,128]. Appliquée aux modes NORMAL (équipes non-buzzées) et MEMORY (solo/multi-équipes inactives). QCM reste inchangé (atténuation fixe 64 sur couleur de réponse, pas couleur d'équipe) (#113).
- **Invariance d'affichage** : les 16 couleurs sont choisies invariantes par `boostTeamColor()` (saturation déjà 100%, luminosité déjà dans [35%, 65%]) — RGB stocké = RGB affiché (#113).

### Fixed
- **Couleur du badge nom VJoueur** : restait figée sur la couleur de réponse QCM au lieu de suivre la couleur d'équipe (#112). Correction : la couleur d'équipe (`team.COLOR`) est désormais prioritaire dans `getPlayerNameColor()` de `VPlayerPage.jsx` ; la couleur de réponse (`bumper.ANSWER_COLOR`) ne sert que de repli en l'absence d'équipe assignée (mode solo résiduel).
- **`boostTeamColor()` — arrondi intermédiaire de teinte** : suppression de l'arrondi `Math.round(h*60)` avant reconstruction RGB qui cassait le round-trip exact pour teintes non-multiples de 60° (4 couleurs de palette divergeaient de ±1). Teinte restée flottante jusqu'à reconstruction ; seul le résultat final arrondi (#113).
- **Persistance atomique `Team.COLOR_NAME`** : `SaveTeams` écrit désormais dans fichier temporaire + rename atomique au lieu de troncature sur place, élimine races lors d'appels concurrents. `SetTeams` attend son auto-save (action admin peu fréquente) au lieu de lancer goroutine (#113).
- **VPlayer chargeait Google Fonts en externe** (#115) — incompatible avec déploiement air-gapped. Polices Fredoka et Inter désormais embarquées en `.woff2` (fichiers variables complets couvrant tous les poids) et servies localement. **Fix complet en 2 cycles** :
  - **Cycle 1 (frontend)** : téléchargement des fichiers `.woff2` depuis Google Fonts, ajout de règles `@font-face` locales dans `index.css`, suppression des `<link>` externes vers `googleapis.com`/`gstatic.com` dans `index.html`, audit exhaustif confirmant aucune autre ressource externe chargée au navigateur.
  - **Cycle 2 (backend)** : ajout de la route HTTP `/fonts/` manquante — première tentative n'embarquait que les fichiers, sans route serveur pour les servir (404). Correction : enregistrement `handleReactAssets` sur `/fonts/`, `Cache-Control` non-immutable (une journée) pour permettre swap de polices en redéploiement, `Content-Type: font/woff2`.

### Technical
- `teamColorPalette` (backend, `main.go`) : 16 entrées exactes RGB, ordre rang 1→16, ancres de repli alignées sur palette.
- `nearestPaletteColorByHue` : utilise teintes des 8 tons vifs comme ancres (attendu : équipes sans `COLOR_NAME` changent de RGB résolu).
- `TEAM_COLORS` (frontend, `constants/colors.js`) : 16 entrées, `getNextTeamColor(teams)` déterministe, `findTeamColor(colorName, rgb)` résout par clé ou RGB exact (rétrocompat).
- Consolidation chemins de rendu : `Podium.jsx`, `CategoryPalmaresPage.jsx`, `VPlayerHeader.jsx` utilisent `getRgbColor()` centralisée.
- `VPlayerPage.jsx` : inversion de la priorité — `team.COLOR` testé en premier, `bumper.ANSWER_COLOR` en repli.
- `VPlayerPage.test.jsx` : nouveaux tests de régression (équipe assignée, changement d'équipe, réponses QCM successives).
- Fichiers embarqués : `server-go/web/public/fonts/{fredoka,inter}-latin.woff2` (OFL-licensed, redistribution autorisée)
- Styles : 2 règles `@font-face` avec `font-weight: <min> <max>` (syntaxe plage) → `url(/fonts/*.woff2)`
- Mécanisme `go:embed` existant couvre les fichiers `dist/fonts/`, aucun changement Go B1
- Tests : 4 nouveaux tests HTTP (route fonctionnelle + non-régression `/assets/` immutable) + reproduction du repro QA

---

## [5.7.20] - 2026-07-25

### Added
- **Badge de connexion mutualisé à 4 états** : `hidden` (rien à afficher), `orange` (déconnecté), `rouge` (déconnecté + message perdu), `vert` (reconnecté). Même icône visuelle pour buzzers physiques et VJoueurs, nouveau composant `ConnectionBadge.jsx` (#109).
- **Compteurs Navbar** : nouveau format `vjoueur X/Y` et `buzzer X/Y` (connectés/participants), avec coloration sévérité (`orange`/`rouge` si dégradé, neutre si tous connectés) (#109).
- **Identité par ID** : reconnexion par ID stocké (localStorage côté VJoueur) → préservation automatique équipe/score, rejette `NAME_TAKEN` si nom en conflit (`PLAYER_REJECTED` message) — élimine tout risque de perte/fusion données sur collision de nom (#109, fix R1).

### Fixed
- **#109** : Icône de déconnexion absente pour les VJoueurs (VPlayers) lors de la perte WebSocket `/ws/player` — comportement désormais identique aux buzzers physiques.
- **R1 (identité par ID)** : rejette explicitement `PLAYER_REJECTED` si le nom est déjà pris (VJoueur connecté ou déconnecté), sans jamais fusionner/supprimer données existantes.
- **Purge roster VJoueur** : déplacée de `StartEnrollment` vers `InitGame`/`NEW_GAME`, inconditionnelle (tous les VJoueurs, connectés ou non) — jamais les buzzers physiques. Libère les noms pour les futures sessions.
- **Ghost bumper** : corrigé le bug où `OnPlayerDisconnected` recréait un bumper fantôme vide après purge `NEW_GAME` — garde `if bumper == nil` avant `UpdateBumper`.

### Technical
- **Bumper.ConnState** : nouveau champ (v5.7.13) pour état badge connexion, transitions: `Hidden|Orange|Red → Green → Hidden` selon événements `Disconnect`, `MessageLost`, `Reconnect`, `ConfirmDelivery`. Toujours sérialisé (pas `omitempty`), visible uniquement pour participants (`TEAM != ""`).
- **PlayerConnectPayload.ID** : nouveau champ optionnel pour identifier VJoueur par ID (UUID ou hash), permet reconnexion sans créer nouveau bumper.
- **Raison rejet NAME_TAKEN** : nouveau code erreur `PLAYER_REJECTED` avec raison détaillée côté frontend (`VPlayerPage.jsx`), redirection auto 3s après écran d'erreur bloquant.

---

## [5.7.23] - 2026-07-26

### Fixed
- **Badge bloqué en ROUGE** : VJoueur passait directement en rouge (jamais visible en orange) après une déconnexion, et restait bloqué indéfiniment après reconnexion — deux causes :
  - **Backend (v5.7.21)** : le broadcast annonçant la déconnexion était à tort compté comme message perdu pour le même VJoueur. Solution : passe-droit `skipNextMessageLost` à usage unique qui ignore le premier broadcast après une transition vers orange, puis D4 reprend normalement (#109).
  - **Frontend (v5.7.22)** : `VPlayerPage` n'essayait plus de se reconnecter si un bumper homonyme existait **même déconnecté** (bloquant l'envoi de `PLAYER_CONNECT` sur coupure réseau normale). Solution : garde `CONNECTED === true` stricte avant de marquer un bumper comme "déjà trouvé", obligeant le renvoi de `PLAYER_CONNECT` sur chaque coupure réseau (#109).

### Technical
- **Backend** : nouveau champ interne `Bumper.skipNextMessageLost` (non sérialisé) pour passe-droit à usage unique dans `ApplyVPlayerBroadcastConnEvents`.
- **Frontend** : correction logique matching par nom dans `VPlayerPage.jsx` — garantit l'envoi de `PLAYER_CONNECT` après toute coupure réseau.

---

## [5.7.10] - 2026-06-08

### Fixed
- **PALMARES** : fix définitif — endpoint dédié `GET /palmares` retourne le palmarès complet pré-assemblé côté backend (catégorie + nom + imageURL + couleur + points équipes/joueurs). Plus aucune race condition possible : le frontend n'a qu'un seul fetch à faire. (#107)
- **Dev** : proxies Vite `/history` et `/palmares` ajoutés — PALMARES testable en mode développement.

### Technical
- Nouvel endpoint `GET /palmares` : agrégation sans double-comptage (clé composite `team|player`), `ResolveCategoryMeta` O(K catégories distinctes), tri desc par `totalPoints`, réponse `[]` garantie si historique vide
- Frontend : suppression du `categoryStats` useMemo (~80 lignes) — rendu direct depuis `PalmaresEntry.name / .imageURL / .color / .teams / .players`

---

## [5.7.9] - 2026-06-08

### Fixed
- **PALMARES** : race condition définitivement supprimée — `/history` retourne désormais `CATEGORY_NAME`, `CATEGORY_IMAGE_URL`, `CATEGORY_COLOR` dans chaque événement (résolu côté backend au moment de l'enregistrement). PALMARES n'a plus besoin de synchroniser deux fetches REST indépendants. (#108)
- **PALMARES** : `catKnown` — catégorie avec nom embarqué mais sans image affiche correctement l'icône emoji (pas `❓`). (#108)

### Technical
- `GameEvent` : 3 nouveaux champs `omitempty` (`CATEGORY_NAME`, `CATEGORY_IMAGE_URL`, `CATEGORY_COLOR`) — rétrocompatibles avec l'historique existant
- `ResolveCategoryMeta(key)` : résolution des métadonnées catégorie (hardcodées + custom avec sidecar) utilisée aux 3 sites d'enregistrement d'événements

---

## [5.7.8] - 2026-06-07

### Fixed
- **PALMARES — race condition catégories** : suppression de `refetchCategories()` dans l'effet PALMARES — cet appel annulait le fetch initial de `useCategories` (cleanup fetchTick 0→1), empêchant les catégories custom d'apparaître. Le fetch initial de mount suffit, les catégories étant disponibles bien avant la fin de partie.
- **`useCategories` — null guard** : `setCategories(data ?? [])` protège contre une réponse API `null` (cas défensif).

---

## [5.7.7] - 2026-06-07

### Fixed
- **PALMARES — race condition catégories custom** : le bloc PALMARES ne s'affiche plus avant que `GET /api/categories` ait répondu (loading guard). Élimine le `❓` persistant en cas de chargement lent ou d'ouverture du TV pendant PALMARES.
- **PALMARES — retry automatique** : si `GET /api/categories` échoue silencieusement, un retry automatique est déclenché 2 secondes après, sans action utilisateur.
- **PALMARES — nom original des catégories custom** : le backend persiste maintenant le nom original saisi lors de la création (sidecar JSON). `GET /api/categories` retourne `"Sport Extrême"` au lieu de `"SPORT_EXTREME"`. Rétrocompatible (les catégories créées avant v5.7.7 continuent d'afficher leur nom technique).
- **PALMARES — normalize défensif sur les clés** : le lookup de catégorie tolère les variations de casse entre l'historique de jeu et les clés API.

---

## [5.7.6] - 2026-06-07

### Fixed
- **#106** — Palmarès : image et couleur des catégories. Le bloc PALMARES utilise désormais un lookup direct sur `apiCategories` (`GET /api/categories`) au lieu de `categoryMeta()`. Le backend est la seule source de vérité pour `name`, `imageURL` et `color`.
  - Frontend ne distingue plus custom vs hardcoded
  - `CategoryInfo` expose désormais un champ `color` (8 couleurs accent pour catégories hardcodées)
  - Noms de 3 catégories hardcodées enrichis : `Arts & Littérature`, `Sciences & Nature`, `Sports & Loisirs`
  - Refetch automatique des catégories à l'entrée en PALMARES pour garantir données fraîches
  - Icône emoji correcte par catégorie hardcodée (via dict `CATEGORIES`) en l'absence d'image uploadée
  - Fallback couleur `#6b7280` correctement appliqué aux catégories custom (chaîne vide `""` traitée par `||`)

---

## [5.7.5] - 2026-06-07

### Fixed
- **#105** — Palmarès et vues TV (READY, QCM, MEMORY, MEMOTION) : les catégories custom n'étaient jamais résolues en production car le filtre `isCustom` retournait toujours une liste vide (champ absent de la réponse API). Correction : passage direct de `apiCategories` complet à `categoryMeta()`, suppression de la variable `customCategories` basée sur `isCustom`.

---

## [5.7.4] - 2026-06-07

### Fixed
- **#104** — Palmarès : l'image des catégories custom est désormais affichée (vignette 2rem×2rem) à la place de l'icône générique. Les catégories hardcodées conservent leur icône emoji.

---

## [5.7.3] - 2026-06-07

### Fixed
- **#102** — Palmarès : les catégories custom créées via le bouton '+' affichent désormais leur nom au lieu de "UNKNOWN". Résolution via `categoryMeta(key, customCategories)` ; fallback en Title Case pour les clés non reconnues.
- **#103** — Boutons mode de jeu (page Quiz/Anim) : l'état non-sélectionné affiche maintenant la couleur du badge en version subtile (bordure + fond 10% opacité), cohérent avec l'état actif (50% opacité). MEMOTION et ARDOISE alignés sur la palette badge (amber / emerald).

---

## [5.7.2] - 2026-06-07

### Fixed
- **#100** — Bouton '+' catégorie : l'ajout d'une catégorie requiert désormais un nom **et** une image (multipart/form-data). Supprime la création JSON texte-only de v5.7.1.
  - `POST /api/categories` accepte désormais `multipart/form-data` avec champs `name` + `file` (image obligatoire)
  - `GET /api/categories` ne scanne plus les fichiers `.json` ; seules les images (`png/jpg/jpeg/webp`) sont retournées comme catégories custom
  - Limitation de taille : `http.MaxBytesReader(10 MB)` pour uploads d'images
  - Clé auto-générée toujours via `toUpperSnakeCase(name)`, image sauvegardée comme `<KEY>.<ext>`
- **#101** — Couleurs boutons mode de jeu (SPEEDY, QCM, MEMORY, MEMOTION, ARDOISE) alignées sur les couleurs de badge des cartes de questions
  - Bordure : 100% couleur badge
  - Fond : 50% opacité couleur badge
  - Alias CSS `.type-speedy` introduit pour cohérence avec renommage NORMAL → SPEEDY

### Changements techniques (BREAKING)

- `POST /api/categories` : body JSON → `multipart/form-data` (champs `name` + `file` obligatoire). Voir `contracts/http-endpoints.md`.
- `GET /api/categories` : suppression du scan des fichiers `.json` (uniquement images désormais).

---

## [5.7.1] - 2026-06-07

### Added
- **Création de catégories depuis l'UI** (#97) : nouveau endpoint `POST /api/categories` pour créer une catégorie texte-only directement depuis QuestionsPage
  - Bouton "+" dans la zone filtres catégories (QuestionsPage)
  - Dialog modal pour saisie du nom
  - Catégorie créée immédiatement visible dans les filtres et badges

### Changed
- **Renommage NORMAL → SPEEDY** (#98) : migration rétrocompatible du type de question
  - Frontend : affichage "Speedy" au lieu de "Normal"
  - Backend : détection version fichier, conversion silencieuse NORMAL → SPEEDY à la lecture
  - Fichiers existants NORMAL lus sans erreur (backward compatible)
  - TV : affichage type "Speedy" au lieu de "Normal" en phase READY

### Fixed
- **Backup sélectif** (#99) : paramètre `backgrounds` renommé en `medias`
  - Endpoints `/backup-select`, `/reset-select`, `/backup-restore`
  - Query param : `backgrounds=true` → `medias=true`
  - Répertoire : `files/backgrounds/` renommé en `files/medias/`
  - Catégories personnalisées sauvegardées sous `medias` au lieu de `backgrounds`

---

## [5.7.0] - 2026-05-31

### Added
- **Catégories personnalisées** (#95) : dépôt d'images dans `data/files/categories/` pour créer ses propres catégories
  - Endpoint `GET /api/categories` : fusion catégories hardcodées + customs
  - Formats acceptés : PNG, JPG, JPEG, WEBP
  - Clé auto-générée : `Sport Extreme.png` → `SPORT_EXTREME`
  - Backup / restore / reset inclus via le flag `backgrounds`
  - UI : composant `CategoryBadge` unifié (QuestionsPage, GamePage, TV) ; utilitaire `categoryUtils.js`
- **Avertissement réseau** (#96) : bandeau rouge en admin quand le serveur n'est accessible qu'en localhost
  - Champ `NETWORK_ONLY_LOCALHOST` dans GameState (WebSocket push)
  - Détection Go toutes les 30 s via goroutine dédiée (network watchdog)
  - Bandeau rouge dans l'interface admin (GamePage)

### Changed
- **Affichage TV — phase READY** : toutes les phases READY (NORMAL, ARDOISE, QCM, MEMORY, MEMOTION) affichent la catégorie courante (standard ou custom)
  - Composant `ReadyCategoryDisplay` rationalisé : code unique standard/custom, animations Framer Motion
  - Type de jeu affiché au-dessus du badge catégorie (NORMAL / ARDOISE / QCM / MEMOTION) avec couleur signature par type (bleu / jaune / vert / rose), animation d'entrée et idle néon
  - Icône catégorie : spring d'entrée + wobble idle
  - MEMORY : pas de label type de jeu (contexte visuel suffisant)

### Fixed
- Badge catégorie dans `QuestionCard` unifié via `CategoryBadge` (cohérence visuelle)
- Filtre catégorie dans `GamePage` propagé aux catégories custom
- MEMORY READY : détection via `MEMORY_PARTICIPATING_TEAMS` (fix timing WebSocket — la phase READY arrivait avant le push des équipes)
- Image catégorie custom : `url.PathEscape` sur le nom de fichier (support des noms avec espaces)

---

## [5.6.1] - 2026-05-30

### Fixed
- QR code "Rejoindre le jeu" illisible sur TV ENROLL : logo emoji obstruait les modules de données → passage au niveau de correction d'erreur H (30%) et réduction du logo 18%→15% de la surface (#85)
- URL encodée dans le QR jeu corrigée : `/player` → `/` (EnrollPage directe, sans redirect superflu) (#85)
- Escaping des caractères spéciaux (`;`, `,`, `"`, `\`) dans les champs SSID/password du QR WiFi, conformément à la spec ZXing (#85)

### Refactor
- `escapeWifiString` extraite vers `src/utils/wifiUtils.js` pour testabilité

---

## [5.6.0] - 2026-05-22

### Added
- **Mode ARDOISE** (#86, #87) : nouveau type de question avec clavier AZERTY/NUMPAD sur VPlayer
  - Modèle `ArdoiseAnswer` : texte + timestamp soumission
  - GameState.ARDOISE_ANSWERS : dictionnaire équipe → réponse saisie
  - Éditeur QuestionsPage : sélection type ARDOISE + champ réponse attendue (ANSWER) + sélecteur clavier AZERTY/NUMPAD
  - Engine `SetArdoiseAnswer()` avec guards FSM
  - Action WebSocket `ARDOISE_INPUT` (endpoint `/ws/player`)

- **Clavier Virtuel ARDOISE** (#88) : composant ArdoiseKeyboard avec touche DEL
  - Affichage automatique dès que question.TYPE === "ARDOISE"
  - **Actif** (saisie traitée) : phase STARTED uniquement
  - **Verrouillé** : STOP, PAUSE, REVEALED
  - Envoi temps réel throttlé 200ms
  - Saisie locale immédiate (affichage sans attendre le serveur)

- **Affichage réponses équipes — zone admin** (#89) : panneau réponses en temps réel (pattern MEMORY)
  - Phase STARTED/STOPPED/REVEALED : affichage des réponses ARDOISE saisies

- **TV REVEAL ARDOISE** (#90) : bonne réponse + cartes équipes en grid (footer 50/50)
  - Affichage texte réponses avec contraintes statiques (overflow:hidden, vh/vw)
  - Bonne réponse visible en REVEALED

### Fixed
- VPlayer identification via 3-pass lookup (#91) : payload.ID → msg.ID → clientID
- Mapping ARDOISE_ANSWERS dans useWebSocket.js (#92)
- Reset clavier à la phase PREPARE (#93)
- Badge type "Ardoise" coloré sur QuestionCard (#94)
- Régression zone-media NORMAL/QCM — media zone (width/height 100% + object-fit:contain)

### UI
- Badges colorés par type de question (Normal/QCM/Memory/Memotion/Ardoise)
- Filtres type sur 2 lignes dans QuestionsPage (Normal·QCM·Memory / Memotion·Ardoise)

---

## [5.5.0] — MEMOTION Secret Mode + Security Fix

### Added
- **MEMOTION Secret Mode** (#76) : nouveau paramètre `MOTION_MEMORIZE_DURATION` (secondes) sur les questions MEMOTION
  - Phase MEMORIZE : toutes les cartes visibles face RECTO + timer décompte sur TV
  - Transition automatique MEMORIZE → GRID à expiration
  - Phase GRID en mode SECRET : coordonnées (A1, B2…) remplacent les thèmes sur les cartes
  - Sélection par coordonnée via clic admin — flash RECTO 0.5s → VERSO question (flow standard)
  - Compatible tous modes de jeu (SOLO, CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE)
  - Rétrocompatible : MOTION_MEMORIZE_DURATION absent/0 → mode standard inchangé
  - Éditeur QuestionsPage : champ "Durée mémorisation (s)" dans la section MEMOTION
  - Admin GamePage : statut MEMORIZE + liste coordonnées↔thèmes pendant phase GRID

### Fixed
- **Security** (#82) : garde path traversal dans `HandleApplyUpdate` — rejette les versions contenant `../` ou séparateurs de chemin → 400 Bad Request

---

## [5.1.4] - 2026-05-11

### Fixed
- [MEMOTION] Layout fullscreen cards : chronomètre visible (hors overlay), header 1/6 + body 4/6 + footer 1/6 de la zone de jeu, réponse en footer (REVEAL), polices cqh
- [Server] Port TCP libéré immédiatement après arrêt (Linux + Windows) — SO_LINGER(0) sur chaque connexion acceptée via `lingerListener` wrapper → RST au lieu de FIN/ACK, no TIME_WAIT (#83)
- [Server] SO_REUSEADDR posé explicitement sur le socket listen via `net.ListenConfig.Control` (Linux : listen_posix.go, Windows : listen_windows.go)

---

## [5.1.5] - 2026-05-13

### Fixed
- [#77] MEMOTION : refonte des 5 états visuels des cards (SELECTED fullscreen fond violet layout RECTO 1/6|4/6|1/6, VERSO QUESTION texte+image+vide, VERSO RÉPONSE rappel+answer+texte, RECTO DONE fond équipe+scale 80%+étoiles+nom équipe)

---

## [5.1.2] - 2026-05-09

### Changed
- MEMOTION fullscreen (SELECTED / QUESTION / REVEAL) : layout CSS grid `1fr 4fr 1fr` — header thème en row 1 (1/6), image en row 2 (4/6), timer en row 3 (1/6) pour la sous-phase QUESTION ; placement explicite via `grid-row` indépendant de l'ordre DOM (#77)
- MEMOTION fullscreen : textes agrandis pour lisibilité TV — thème `4rem`, étoiles `3rem`, texte question/réponse `4.5rem`

### Fixed
- HTTPServer graceful shutdown: replaced `Close()` with `Shutdown(ctx)` (3s timeout) to eliminate TCP TIME_WAIT port-busy errors on restart (#81)
- MEMOTION grid cards: layout 1/6 header + 4/6 body + 1/6 footer (CSS grid), fonts filling zone height via `cqh` units, images `object-fit: contain` to preserve aspect ratio (#77)

---

## [5.1.1] - 2026-05-08

### Fixed
- Suppression du TCPServer legacy (port 1234 TCP) — dead code depuis v3.0.0 qui bloquait le démarrage sur Windows (#80)
- Port UDP 1234 (discovery firmware BuzzClick) découplé du champ config `tcp_port` via constante `BuzzerDiscoveryPort`

---

## [5.1.0] - 2026-05-08

### Changed
- MEMOTION TV : layout constant des cartes en 3 zones fixes — header (titre/thème), body (image, flex-grow), footer (étoiles + points) ; supprime l'ancienne logique conditionnelle image/no-image (#77)
- MEMOTION : sélection de carte depuis la preview TV en subphase GRID — clic direct sur `PlayerDisplay` en mode `isAdminPreview` pour toute carte `UNPLAYED` (#78)
- MEMOTION admin : panneau GRID simplifié — mini-grille supprimée, remplacée par un label informatif (#78)
- MEMOTION admin : subphase REVEAL — affiche uniquement le bouton de l'équipe courante + "Perdu" (remplace l'affichage de toutes les équipes avec buzzers) ; renommage "Aucun" → "Perdu" (#79)

---

## [5.0.6] - 2026-05-06

### Added
- MEMOTION : points par niveau de difficulté configurables depuis l'éditeur (★=1pt / ★★=3pt / ★★★=5pt par défaut, modifiables par question)

### Fixed
- MEMOTION : inputs points difficulté — valeur minimale 1 (min="0" permettait de saisir 0 ignoré silencieusement)
- MEMOTION éditeur : sélecteur de difficulté affiche uniquement les étoiles (★/★★/★★★), sans les points qui n'étaient pas mis à jour dynamiquement
- MEMOTION éditeur : champ "Temps" remonté dans le bloc configuration en haut du formulaire (avec les points de difficulté)

---

## [5.0.5] - 2026-05-06

### Fixed
- MEMOTION TV: cartes rectangulaires qui remplissent toute la zone de jeu (override `.memotion-game .memory-grid` avec `1fr`, sans algo carré MEMORY)
- MEMOTION TV: animation zoom clip-path fiable lors de la sélection d'une carte (getBoundingClientRect au rendu, remplace layoutId incompatible avec overflow:hidden)
- MEMOTION TV: animation zoom arrière vers la grille après attribution des points (DONE)
- MEMOTION TV: REVEAL — ancrage clipPath pour permettre l'interpolation CSS exit vers position grille
- MEMOTION TV: correction du padding de la grille pour couverture totale de la zone

---

## [5.0.4] - 2026-05-06

### Fixed
- MEMOTION : phase READY affiche uniquement "PRÉPAREZ-VOUS" + barre d'équipes, sans la grille de cartes
- MEMOTION : footer des cartes équilibré — thème tronqué si long, étoiles et points clairement secondaires
- MEMOTION : effet de zoom sélection rendu visible grâce au LayoutGroup framer-motion (scale retiré de l'animation d'entrée des cartes)
- MEMOTION : fond opaque (#1a1a2e) sur la sous-phase REVEAL (gradient transparent remplacé)
- MEMOTION : bouton RÉVÉLER masqué tant que le timer est actif (admin)
- MEMOTION : animation zoom retour vers la grille après attribution des points (layoutId sur REVEAL)

---

## [5.0.3] - 2026-05-06

### Fixed
- MEMOTION : affichage "PRÉPAREZ-VOUS" ajouté en phase READY (grille visible + animation)
- MEMOTION : mise en page des cartes dans la grille — image couvre la carte (object-fit: cover), thème en footer ; sans image, thème centré grand
- MEMOTION : effet zoom de sélection rendu visible — suppression du `transformStyle: preserve-3d` qui bloquait l'animation layoutId framer-motion
- MEMOTION : cartes QUESTION et REVEAL opaques pendant le flip — ajout background `#1a1a2e` sur fullscreen, suppression opacity des animations
- MEMOTION : flip QUESTION→REVEAL coordonné (même direction que SELECTED→QUESTION)
- MEMOTION : retour à la grille animé par zoom inverse (suppression exit rotateY sur SELECTED, layoutId gère le reverse)

---

## [5.0.2] - 2026-05-06

### Fixed
- MEMOTION : image/texte des cartes dans la grille agrandis — flex layout (image flex: 1, max-height 75%, thème plus grand, étoiles/points poussés en bas)
- MEMOTION : animation flip 3D coordonnée SELECTED → QUESTION (exit rotateY:90 + enter rotateY:-90 dans AnimatePresence partagée)
- MEMOTION : image plein écran sous-phase SELECTED agrandie à 70% ; fallback texte si pas d'image

---

## [5.0.1] - 2026-05-04

### Changed
- **[MEMOTION — subphase SELECTED]** : Nouveau step entre la sélection et la question
  - `MEMOTION_SELECT` → subphase `SELECTED` (carte zoomée en plein écran, RECTO visible, pas de timer)
  - `MEMOTION_FLIP` (nouvelle action) → subphase `QUESTION` + démarrage timer per-carte
  - `MEMOTION_STOP_TIMER` (nouvelle action) → arrêt manuel du timer, subphase reste `QUESTION`
  - Admin : panneau SELECTED avec boutons "DÉMARRER" (FLIP) et "ANNULER" ; bouton "STOP TIMER" en phase QUESTION
  - TV : animation zoom framer-motion `layoutId` depuis la grille vers fullscreen (SELECTED) → flip rotateY (QUESTION/REVEAL)
  - Annulation depuis SELECTED : `MEMOTION_DONE` vide ramène la carte à `UNPLAYED`

### Fixed
- `DoneMotionCard` cancel-from-SELECTED utilise désormais `e.state.MotionSelected` (authoritative serveur) au lieu du `cardID` client

---

## [5.0.0] - 2026-05-03

### Added
- **[Nouveau type de jeu MEMOTION]** : Grille de cartes à 3 faces (RECTO / VERSO / REVEAL)
  - RECTO : thème + image optionnelle + difficulté (1★/2★★/3★★★ → 1pt/3pt/5pt)
  - VERSO : question (texte + image) affichée en plein écran
  - REVEAL : réponse (texte + image) affichée en plein écran
  - Flow : Admin sélectionne carte → timer per-carte → Admin clique RÉVÉLER → attribution points → carte retournée avec couleur équipe
  - 3 modes de jeu : SOLO, CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE (mêmes règles que MEMORY)
- **[#XX WebSocket Actions]** : 4 nouvelles actions pour MEMOTION
  - `MEMOTION_SELECT` → `{ CARD_ID: string }` — sélectionne une carte
  - `MEMOTION_REVEAL` → `{}` — révèle la réponse
  - `MEMOTION_DONE` → `{ CARD_ID: string, WINNER_TEAM?: string }` — marque la carte comme jouée
  - `MEMOTION_SET_TEAMS` → `{ TEAMS: string[] }` — configure les équipes participantes
- **[GameState enrichi pour MEMOTION]** : 7 nouveaux champs
  - `MEMOTION_SUBPHASE` : sous-phase du jeu MEMOTION (GRID, QUESTION, REVEAL, DONE)
  - `MEMOTION_SELECTED` : ID de la carte sélectionnée
  - `MEMOTION_CARD_STATES` : états des cartes (map CARD_ID → state)
  - `MEMOTION_CARD_TEAMS` : assignation équipes par carte (map CARD_ID → TEAM)
  - `MEMOTION_CURRENT_TEAM` : équipe actuelle en mode CHACUN_SON_TOUR
  - `MEMOTION_PARTICIPATING_TEAMS` : liste des équipes participantes
  - `MEMOTION_CURRENT_TEAM_COLOR` : couleur RGB de l'équipe actuelle
- **[Interface Admin pour MEMOTION]** : Zone MEMOTION dans page Quiz
  - Upload images cartes (RECTO + VERSO + REVEAL par carte)
  - Édition grille (sélection cartes, ordre, difficultés)
  - Prévisualisation RECTO/VERSO/REVEAL
- **[Interface TV pour MEMOTION]** : Affichage STATIQUE per-subphase
  - GRID : grille de cartes avec images RECTO
  - QUESTION : question VERSO (100vw×100vh)
  - REVEAL : réponse REVEAL (100vw×100vh)
  - DONE : carte retournée avec couleur équipe

### Files Modified
**Backend** :
  - `server-go/internal/game/models.go` : struct `MotionCard`, champs `GameState.MEMOTION_*`
  - `server-go/internal/game/engine.go` : logic MEMOTION_SELECT/REVEAL/DONE, timer per-carte
  - `server-go/internal/protocol/messages.go` : actions `MEMOTION_SELECT`, `MEMOTION_REVEAL`, `MEMOTION_DONE`, `MEMOTION_SET_TEAMS`
  - `server-go/cmd/server/main.go` : handlers et broadcast MEMOTION
  - `server-go/internal/server/http.go` : endpoints MEMOTION (CRUD cartes)

**Frontend** :
  - `web/src/pages/GamePage.jsx` : affichage équipes, timer per-carte
  - `web/src/pages/QuizPage.jsx` (ex-QuestionsPage) : Zone MEMOTION (CRUD cartes)
  - `web/src/pages/PlayerDisplay.jsx` : écrans TV phase GRID/QUESTION/REVEAL
  - `web/src/components/QuestionCard.jsx` : composant affichage MEMOTION card
  - `web/src/hooks/useWebSocket.js` : handling actions MEMOTION_*

---

## [4.0.11] - 2026-05-02

### Changed
- **Self-update : stratégie copy-and-launch** : `performRestart()` ne modifie plus jamais le binaire courant. Le nouveau binaire versionné est copié dans le répertoire courant, lancé (attend les ports via retry loop), puis l'ancien processus se termine. Suppression de la logique backup/restore. (#70)

---

## [4.0.10] - 2026-05-02

### Fixed
- **Port TCP occupé lors du self-update** : le serveur TCP (port 1234) échouait immédiatement au démarrage si l'ancien processus tenait encore le port, causant un crash fatal. Fix : retry loop 500ms dans `TCPServer.Start()`, identique au fix HTTP du port 80 (#69)

---

## [4.0.9] - 2026-05-02

### Fixed
- **Panic au démarrage si config.json absent** : `AckManager.Start()` appelait `time.NewTicker(0)` quand `config.json` était introuvable, causant un crash immédiat. Fix : normalisation dans `NewAckManager()` (valeur ≤ 0 → 2000 ms) + fallback `config.Get()` initialisé avec `AckTimeoutMs: 2000, AckMaxRetries: 3` (#68)

---

## [4.0.8] - 2026-05-02

### Fixed
- Port HTTP occupé au démarrage lors d'un self-update : le nouveau processus retente désormais la liaison toutes les 500 ms (avec log d'avertissement) jusqu'à ce que le port soit libéré par l'ancien processus, au lieu d'échouer silencieusement (`server-go/internal/server/http.go`)

---

## [4.0.4] - 2026-04-30

### Added
- **Fonds d'écran NEW_GAME multi-images** : système de rotation automatique pour l'écran TV phase NEW_GAME
  - Upload multiple d'images dans la Zone Quiz de la page `/admin/quiz` (formats: jpg/png/gif/webp/svg)
  - Configuration par image : durée d'affichage (ms), opacité (0-100%)
  - Rotation client-side côté TV : `setTimeout` basé sur la durée de chaque image
  - Overlay absolu (z-index: 0) indépendant de l'opacité du texte et effets (z-index: 1)
  - Fallback automatique : dégradé animé multicolore (violet→bleu→cyan→rose→ambre, 9s infini) si aucune image
  - Persistance : stockage dans `data/files/new-game-backgrounds/` + `backgrounds.json`
  - API REST : `POST /new-game-backgrounds` (upload), `PUT /new-game-backgrounds` (config ordre/durée/opacité), `DELETE /new-game-backgrounds?file=xxx` (suppression)
  - Broadcast CONFIG_UPDATE : champ `new_game_backgrounds: []Background` (jamais `omitempty` — toujours présent)

### Changed
- **Écran TV NEW_GAME amélioré** : nouveau système visuel de fonds d'écran multi-images avec fallback dégradé
  - Titre dynamique (clamp 4rem→8vw), weight 900, text-shadow pour lisibilité
  - Étoiles scintillantes (mix-blend-mode: screen)
  - Soutien pour images dynamiques ou dégradé statique selon configuration admin

### Fixed
- **CONFIG_UPDATE sérialisation** : champ `new_game_backgrounds` non `omitempty` pour éviter que l'UI admin manque les mises à jour lors de suppression (v4.0.4)
  - Correction du bug qui empêchait la TV et l'UI admin de se mettre à jour lorsque l'utilisateur supprimait toutes les images

### Closes
- Closes #66 (NEW_GAME multi-image backgrounds)
- Closes #67 (fonds d'écran dynamiques)

---

## [4.0.1] - 2026-04-30

### Added
- **NOUVELLE PARTIE** : bouton dans GamePage (phase STOPPED uniquement) → reset complet (scores, historique, questions) + phase `NEW_GAME`
- **Écran TV "NOUVELLE PARTIE À VENIR"** : affichage plein écran statique sur `/tv` pendant la phase `NEW_GAME`, avec nom du quiz, thème et texte libre
- **Métadonnées de quiz** : champs Nom du quiz / Thème général / Texte libre stockés dans le GameState et inclus dans les sauvegardes/restaurations
- **Action `NEW_GAME`** (WebSocket) : déclenche `InitGame()` — reset scores teams+joueurs, historique, statuts questions, question sélectionnée
- **Action `UPDATE_QUIZ_META`** (WebSocket) : met à jour les métadonnées du quiz en temps réel
- **Phase `NEW_GAME`** : nouvelle phase de machine d'état — transitoire jusqu'à la sélection de la première question

### Changed
- **Menu "Questions" → "Quiz"** : label Navbar et URL `/admin/quiz`
- **Page Quiz** (ex-QuestionsPage) réorganisée en 3 zones : Quiz (métadonnées) / Ambiance (fonds d'écran) / Questions (liste)

### Closes
- Closes #66 — Bouton Start GAME
- Closes #67 — Change QUESTIONS en QUIZ

---

## [3.8.0] - 2026-04-28

### Added
- **[#11 Backend]**: Endpoints WebSocket dédiés `/ws/admin`, `/ws/tv`, `/ws/player` — filtrage intelligent des messages par type de client
  - Trois endpoints spécialisés remplacent le broadcast global — chaque client reçoit uniquement les messages pertinents à son rôle
  - Table de filtrage : 25 actions distribuées selon le type client (Admin, TV, VPlayer)
  - Exemples : UPDATE/START/STOP/PAUSE → Admin+TV+VPlayer ; QUESTIONS/CLIENTS → Admin only ; PLAYER_REJECTED → VPlayer only
  - Rétrocompatibilité : `/ws` conservé comme alias vers `/ws/admin`
  - `ClientType` dans le modèle de connexion WebSocket — enforcé au moment de la création (pas de `setClientType` dynamique)
- **[#41 Backend + Frontend]**: Réduction payload WebSocket buzzers — whitelist d'actions
  - Buzzer physiques reçoivent uniquement : UPDATE, UPDATE_TIMER, START, CONTINUE, STOP, PAUSE, READY, RESET, HELLO, LED_SET, OTA_UPDATE, WIFI_CONFIG (12 actions) — game state jamais transmis aux buzzers
  - Nouveau helper `BroadcastIfRelevant()` dans `websocket_buzzer.go` — filtre intelligemment les payloads par type d'action
  - Réduit bande passante et latence WebSocket pour les buzzers
- **[#54 Backend + Frontend + Firmware]**: Protocole ACK buzzer — confirmation de réception des messages prioritaires
  - Messages LED_SET, OTA_UPDATE, WIFI_CONFIG générés avec `MSG_ID` unique (12-char hex, optionnel omitempty)
  - Firmware BuzzClick répond avec `ACK` avant d'appliquer l'action (minimise latence de confirmation)
  - AckManager côté serveur : registre, retry configurable (`ack_timeout_ms`, `ack_max_retries`) et expiry automatique
  - Nouveau champ `ACK_PENDING` dans le modèle Bumper — badge ⚠ horloge quand buzzer attend confirmation
  - Badge ACK_PENDING visible dans TeamCard et TeamsPage (style inline React, fond amber `#f59e0b`, icône horloge)
- **[#11 Backend + Frontend]**: Sérialiseurs payload différenciés par type de client
  - `SerializeForAdmin()` : payload complet (toutes les infos) pour l'admin web
  - `SerializeForWebClient()` : payload réduit pour TV et VPlayer — élimine FIRMWARE_VERSION, IS_OUTDATED, OTA_STATUS, OTA_PERCENT, ACK_PENDING et infos serveur (config)
  - `SerializeForBuzzer()` : payload minimal pour buzzers physiques (phase, timer, bumpers avec ID-NAME-TEAM-CONNECTED, teams avec NAME-COLOR-STATUS)
  - `BroadcastRawToTypes()` dans `websocket.go` — achemine les payloads sérialisés correctement selon le type client
  - Réduction de 40-60% du volume de données WebSocket pour TV/VPlayer
- **[#11 Frontend]**: `GameProvider` accepte un prop `endpoint` pour routing multi-rôle
  - `GameProvider({ endpoint='/ws/admin' })` — transmis à `useWebSocket()` pour ciblage de l'endpoint WS
  - Routes : `/admin/*` → défaut (`/ws/admin`) ; `/tv` → `endpoint="/ws/tv"` ; `/player` et `/enroll` → `endpoint="/ws/player"`
  - Permet l'évolution future vers plusieurs instances GameProvider avec endpoints différents (ex: TV + admin simultanées dans iframe)
- **[Firmware BuzzClick v3.8.0]**: Implémentation ACK côté firmware
  - `ws_sendAck()` dans `click_websocket_espidf.h` — envoi non-bloquant de réponses ACK
  - Handler MSG_ID dans `parseJSON()` — ACK envoyé AVANT d'appliquer l'action (LED_SET, OTA_UPDATE, WIFI_CONFIG)
  - Version firmware alignée avec serveur : 3.8.0

### Changed
- **[#41 Backend]**: Whitelist buzzer étendue pour support état du jeu partiel
  - Whitelist précédente (v3.8.0 initial) : LED_SET, OTA_UPDATE, WIFI_CONFIG, HELLO (4 actions)
  - Whitelist étendue : UPDATE, UPDATE_TIMER, START, CONTINUE, STOP, PAUSE, READY, RESET, HELLO, LED_SET, OTA_UPDATE, WIFI_CONFIG (12 actions)
  - Permet aux buzzers physiques de recevoir et afficher l'état du jeu (phase, timer, listes équipes/joueurs) sans informations sensibles (config, firmware)
  - Utilise `SerializeForBuzzer()` pour la sérialisation minimale
- **[#11 Backend]**: Détails de routage WebSocket par endpoint
  - `/ws/admin` : Admin web client — reçoit ALL, UPDATE, START, STOP, PAUSE, CONTINUE, REVEAL, RESET, QCM_HINT, ENROLLMENT_UPDATE, READY, REMOTE, BACKGROUND_CHANGE, CONFIG_UPDATE, SHOW|HIDE_QR_CODE, FULL, MEMORY_*, QUESTIONS, CLIENTS, FIRMWARE_VERSION, PLAYER_CONNECTED, PLAYER_ASSIGNED
  - `/ws/tv` : TV display client — reçoit UPDATE, START, STOP, PAUSE, CONTINUE, REVEAL, RESET, QCM_HINT, ENROLLMENT_UPDATE, READY, REMOTE, BACKGROUND_CHANGE, CONFIG_UPDATE, SHOW|HIDE_QR_CODE, FULL, MEMORY_* (serialized via `SerializeForWebClient()`)
  - `/ws/player` : VPlayer web client — reçoit UPDATE, START, STOP, PAUSE, CONTINUE, REVEAL, RESET, QCM_HINT, ENROLLMENT_UPDATE, PLAYER_REJECTED, PLAYER_CONNECTED, PLAYER_ASSIGNED (serialized via `SerializeForWebClient()`)
- **[#54 Frontend]**: Migration des pages TV/VPlayer vers endpoints dédiés
  - `PlayerDisplay.jsx` (TV) : utilise `/ws/tv` au lieu de l'endpoint admin
  - `EnrollPage.jsx`, `VPlayerPage.jsx` (VPlayer) : utilisent `/ws/player` — suppression du `setClientType` dynamique
  - `GameContext.jsx` : spécifie explicitement `/ws/admin` — architecture à connexion unique par rôle
  - `GameProvider` accepte prop `endpoint` transmis au hook `useWebSocket()`

---

## [3.7.0] - 2026-04-26

### Fixed
- **[Frontend]**: Filtres catégories dans la barre Équilibre (fixes #57)
  - Les catégories dans la barre d'équilibre de `GamePage` sont désormais cliquables pour filtrer les questions affichées
  - Multi-sélection : cumuler plusieurs catégories actives, clic re-sélectionne/désélectionne
- **[Backend + Firmware]**: Couleurs LEDs buzzers sélectionnées par teinte (hue-based) pour cohérence palette (fixes #61)
  - Nouveau helper `nearestPaletteColorByHue()` — sélectionne la couleur palette la plus proche par distance de teinte HSL
  - Évite les mélanges visuellement incohérents sur les buzzers quand la couleur d'équipe est proche de plusieurs entrées palette
  - Remplace le calcul RGB direct par une approche HSL plus fidèle à la perception humaine
- **[Firmware BuzzClick]**: Race condition OTA — `performOTA()` déplacé dans tâche FreeRTOS séparée
  - La fonction OTA bloquait la tâche WebSocket, provoquant une déconnexion à ~20% du téléchargement et un échec silencieux
  - Fix : `performOTA()` exécuté dans `ota_task` (FreeRTOS, 16 KB stack) — la connexion WebSocket reste active pendant le téléchargement et le flash
- **[Firmware BuzzClick]**: Stack overflow `websocket_task` — taille augmentée 4096 → 8192 bytes
  - Stack insuffisant provoquait des crashes aléatoires lors de la réception de messages WebSocket de taille importante
- **[Firmware BuzzClick]**: VERSION dans `platformio.ini` corrigée de 3.6.6 → 3.7.0
  - Le firmware était compilé avec un numéro de version incorrect — le badge firmware sur l'interface admin affichait une version erronée après flash

### Added
- **[Frontend]**: Badge version cliquable dans la Navbar — redirige vers `/admin/updates` (fixes #63)
  - Le numéro de version affiché dans la navbar est désormais un lien direct vers la page des mises à jour
  - Permet un accès rapide depuis n'importe quelle page
- **[Frontend]**: Page d'enrollment VJoueur améliorée — double QR code et barre de progression (fixes #43)
  - Deux QR codes côte à côte sur la vue TV d'inscription (`/tv` en phase ENROLLMENT) :
    - **QR code WiFi** : permet aux joueurs de rejoindre le bon réseau avant de s'inscrire
    - **QR code VJoueur** : URL d'inscription avec port correct (`window.location.host`)
  - Barre de progression en temps réel : nombre de joueurs inscrits / max visible sur l'affichage TV
- **[Frontend]**: QR codes TV d'enrollment redessinés pour projection (fixes #51)
  - Affichage redessiné avec meilleure lisibilité, contraste optimisé pour projection TV grande distance
- **[Frontend]**: Filtres questions par catégories multi-sélection dans la barre Équilibre (fixes #40)
  - Sur `GamePage`, la barre d'équilibre des catégories permet de filtrer les questions visibles par catégorie
  - Indicateur visuel sur les catégories actives (surlignage coloré)
- **[Backend]**: Noms de fichiers de sauvegarde versionnés avec horodatage (fixes #44)
  - Les archives téléchargées incluent désormais la version serveur et la date dans leur nom
  - Format : `buzzcontrol-full-backup-v3.7.0_2026-04-26.tar` (complet) et `buzzcontrol-backup-v3.7.0_2026-04-26.tar` (sélectif)
  - Facilite la gestion et l'identification temporelle de multiples sauvegardes
- **[Backend + Firmware]**: Animation COMET lors de l'attribution de points — couleur dynamique (fixes #50)
  - Nouvelle action serveur `sendLEDSetComet` : envoie `EFFECT: "COMET"` avec champ `COMET_COLOR` calculé dynamiquement
  - `COMET_COLOR` : serveur choisit or (`[255,215,0]`) ou blanc (`[255,255,255]`) selon le contraste avec la couleur d'équipe (dist² euclidien < 8000 → blanc pour lisibilité)
  - Firmware : state machine `manageLedComet()` — bande rotative 23 LEDs APA102, 2 tours (~3.3 s), utilise `cometR/G/B` du payload (plus de gold hardcodé)
  - Déclenchée automatiquement à chaque attribution de points (`TEAM_POINTS`, `BUMPER_POINTS`)
  - Retour automatique à l'état LED précédent après l'animation

---

## [3.6.9] - 2026-04-25

### Fixed
- **[Backend]**: Les équipes vides (sans buzzer assigné) ne bloquent plus le passage en READY/START
  - `AreAllTeamsReady()` ignorait toutes les équipes sans exception — une équipe sans buzzer (`Ready=false` permanent) empêchait le `START` même si toutes les équipes actives étaient prêtes
  - Refactoring : nouveau helper privé `getActiveTeams()` — retourne uniquement les équipes ayant ≥1 buzzer assigné
  - `AreAllTeamsReady()`, `updateTeamsReady()` et l'auto-init Memory utilisent désormais `getActiveTeams()` — cohérence avec le filtre d'affichage frontend (`buzzers.length > 0`)
  - Test de non-régression : `TestEngine_AreAllTeamsReady_EmptyTeamsIgnored`

---

## [3.6.8] - 2026-04-25

### Fixed
- **[Backend]**: Détection déconnexion WebSocket réduite à ≤ 5 s (buzzers physiques et clients admin)
  - `pingPeriod` : 30 s → 3 s ; `ReadDeadline` (pongWait) : 25 s / 60 s → 5 s ; `WriteDeadline` : 10 s → 3 s
  - Les buzzers qui perdent la connexion sans TCP FIN (coupure alimentation, chute WiFi) sont détectés en ≤ 5 s
  - Formule : pongWait (5 s) > pingPeriod (3 s) garantit un ping envoyé avant l'expiration — aucun faux positif
  - Appliqué sur `websocket_buzzer.go` (buzzers physiques) et `websocket.go` (clients admin/TV)
- **[Backend]**: Reconnexion rapide (< 5 s) transparente — aucun flash de badge
  - Race condition : si un buzzer reconnectait avant que le serveur détecte la déconnexion de l'ancienne connexion, le callback `OnBuzzerDisconnected` de la connexion zombie posait `CONNECTED=false` après la reconnexion
  - Fix : `OnBuzzerDisconnected` vérifie `IsClientConnected(mac)` avant d'agir — ignore le timeout de la connexion zombie si une nouvelle connexion active existe pour ce MAC
- **[Frontend]**: Badge déconnexion buzzer en style inline (cercle jaune + icône wifi-off SVG)
  - Remplacement de la classe CSS `.buzzer-disconnected-badge` par des styles React inline (`background:#f59e0b`, `stroke:white`) pour garantir le rendu indépendamment du chargement CSS
  - Appliqué sur `TeamCard.jsx` et `TeamsPage.jsx` (2 emplacements : buzzers assignés + non assignés)
- **[Frontend]**: Badge ⚠ buzzer déconnecté étendu à la page Équipes (`TeamsPage.jsx`)
  - Affiché dans les rows membres (draggable) et dans la section buzzers non assignés
  - Condition stricte identique à TeamCard : `CONNECTED === false && !IS_VIRTUAL && !IS_VPLAYER`
  - 7 tests Vitest/RTL ajoutés dans `TeamsPage.test.jsx` (dont cas buzzer firmware pre-v3.6.6 sans champ CONNECTED)

---

## [3.6.7] - 2026-04-23

### Fixed
- **[Backend]**: Champ `CONNECTED` persisté sur disque — buzzers affichés comme connectés après redémarrage serveur (fixes #48)
  - `CONNECTED=true` était sérialisé dans le JSON de persistance et rechargé au démarrage — les buzzers apparaissaient connectés sur TeamCard même sans connexion physique active
  - Fix : au démarrage du moteur (`NewEngine`), tous les bumpers chargés depuis le disque ont leur champ `CONNECTED` explicitement réinitialisé à `false` avant d'accepter des connexions
  - Test de non-régression : `TestEngine_StartupConnectedReset`

---

## [3.6.6] - 2026-04-23

### Added
- **[Backend + Frontend]**: Indicateur de déconnexion buzzer en temps réel — badge ⚠ sur TeamCard (fixes #48)
  - Nouveau champ `CONNECTED bool` dans le modèle `Bumper` (sans `omitempty` — `false` toujours sérialisé en JSON)
  - `handleHello()` : positionne `CONNECTED=true` à la connexion WebSocket
  - `OnBuzzerDisconnected` callback dans `BuzzerWebSocketHub` : positionne `CONNECTED=false` à la déconnexion et broadcast `UPDATE`
  - `UpdateBumper()` dans le moteur : gère le cas `CONNECTED` pour propager la mise à jour d'état
  - Frontend `TeamCard.jsx` : badge ⚠ jaune inline dans la row du buzzer déconnecté (conditions strictes : `!isVPlayer && !isVirtual && connected === false`)
  - Tests de non-régression : `TestEngine_UpdateBumper_Connected`
  - Au redémarrage serveur : tous les buzzers chargés depuis le disque ont `CONNECTED=false` → badge ⚠ affiché jusqu'à réception du HELLO (fix introduit en v3.6.7)
- **[Backend]**: Mode haute-fréquence UDP broadcaster quand un buzzer est déconnecté (fixes #64)
  - `SetHighFrequency()` dans `BroadcasterManager` : active un intervalle de 500 ms (vs 5 s normal) pour accélérer la redécouverte serveur par le buzzer déconnecté
  - Priorité des intervalles : enrollment (1 s) > haute-fréquence (500 ms) > normal (5 s)
  - `updateBroadcasterFrequency()` dans `main.go` : orchestre le passage en haute-fréquence si au moins un buzzer physique est déconnecté, retour au mode normal quand tous sont reconnectés
  - Appelé aux points clés : `handleHello()`, `OnBuzzerDisconnected`, `handleDeleteBumper()`, `handleFullUpdate()`, démarrage serveur

---

## [3.6.4] - 2026-04-23

### Fixed
- **[Firmware BuzzClick]**: Spinner `WS_RECONNECTING` gelé pendant le teardown WebSocket (`ws_safe_destroy`)
  - `ws_safe_destroy()` bloquait jusqu'à 700 ms (close timeout 500 ms + delay 200 ms) — le spinner était figé pendant toute la durée du teardown
  - Fix : close timeout réduit à 0 ms (non-bloquant, fire-and-forget — le pair peut être injoignable), delay réduit à 50 ms, appel `manageLedError()` avant le delay pour maintenir l'animation pendant le teardown
- **[Firmware BuzzClick]**: Spinner `WS_RECONNECTING` gelé ~4 s pendant `esp_websocket_client_stop()` (suite)
  - Cause racine : `stop()` bloque jusqu'à l'arrêt complet de la tâche réseau interne du SDK (~4 s quand le serveur est injoignable) — `manageLedError()` ne tourne plus pendant ce temps
  - Fix : `stop()` + `destroy()` déchargés sur une tâche FreeRTOS temporaire (`ws_destroy_task`, 4 KB stack, priorité 5) ; `loop()` entre dans une boucle 100 ms tick — `manageLedError()` + `esp_task_wdt_reset()` — jusqu'à completion ; `wsGeneration` incrémenté et `wsClient=NULL` avant le spawn pour invalider tout callback stale
  - `network_timeout_ms` revert : champ inexistant dans cette version du SDK ESP32-C3 (seul `pingpong_timeout_sec` est disponible) — le freeze est désormais résolu côté tâche FreeRTOS, sans modifier les timeouts SDK
- **[Firmware BuzzClick]**: Patterns LED `manageLedBlink()` et `updateGrayRotation()` non isolés pendant OTA
  - Ces fonctions appelées dans `loop()` pouvaient écraser le blue-blink OTA ou le pattern `OTA_ERROR` malgré le guard `otaInProgress` déjà en place sur `LED_SET`
  - Fix : guard `if (otaInProgress) return;` ajouté dans `updateGrayRotation()` et `manageLedBlink()` — isolation OTA LED complète (fixes #62 suite)
- **[Firmware BuzzClick]**: `volatile` manquant sur `otaInProgress` — conflit de déclaration `extern`
  - `otaInProgress` était défini sans `volatile` dans `click_otaManager.h` mais l'`extern` dans `click_serverConnection.h` attendait `volatile bool` → erreur de compilation (conflicting-declaration)
  - Fix : `volatile bool otaInProgress` dans les deux fichiers, aligné sur la convention `wsConnected/wsConnecting/wsGeneration`
- **[Firmware BuzzClick]**: Reconnexion WebSocket avec intervalle fixe 10 s après déconnexion
  - `checkWebSocketConnection()` attendait systématiquement 10 s entre chaque tentative — l'utilisateur voyait le spinner tourner pendant 10 s sans tentative de reconnexion
  - Fix — machine à 3 états :
    - **IMMEDIATE** (`wsDisconnectedImmediate=true`) : tente une reconnexion dès le prochain tick `loop()` après `DISCONNECTED`/`ERROR`
    - **WAIT_UDP** (`wsWaitingForBroadcast=true`) : si la tentative immédiate échoue, attend un heartbeat UDP du broadcaster serveur ; essaie chaque IP reçue dans l'ordre
    - **IDLE** : `wsConnected=true`, les deux flags sont à `false`
  - `WiFiGotIP()` (reconnect après coupure WiFi) reset `wsWaitingForBroadcast=false` + `wsDisconnectedImmediate=true` pour re-entrer en état IMMEDIATE

---

## [3.6.3] - 2026-04-22

### Fixed
- **[Firmware BuzzClick]**: Race condition au boot — `esp_websocket_client_start()` retournait ESP_FAIL (-1) sur l'ESP32-C3 (fixes #59)
  - Deux tâches FreeRTOS concurrentes (`WiFiGotIP` event task et `loop()`) pouvaient appeler `connectWebSocket` ou `checkWebSocketConnection` simultanément, provoquant un double `start()` sur le même handle
  - Ajout du flag `volatile bool wsConnecting` : positionné à `true` à l'entrée de `connectWebSocket()`, remis à `false` sur toutes les branches de sortie
  - `checkWebSocketConnection()` retourne immédiatement si `wsConnecting == true`
- **[Firmware BuzzClick]**: Reconnexion WebSocket bloquée après timeout (fixes #42)
  - Après un timeout, `wsClient` était mis à `NULL` par `ws_safe_destroy()` — `checkWebSocketConnection()` ne relançait plus jamais de tentative (condition `wsClient != NULL`)
  - La condition est remplacée par `!wsServerIP.isEmpty()` : reconnexion infinie tant que le serveur est connu
- **[Firmware BuzzClick]**: Régressions LED team color et spinner WS_RECONNECTING invisible causées par réécriture complète du buffer (commit 8ea9614)
  - La réécriture complète écrasait la couleur d'équipe posée par `LED_SET` à chaque tick du spinner
  - Corrigé par delta-update : seuls 2 pixels modifiés par tick via `applyLedColorToBuffer()` — fond de couleur intégralement préservé
- **[Firmware BuzzClick]**: Spinner WS_RECONNECTING gelé pendant la boucle de reconnexion
  - `manageLedError()` n'était appelé que dans `loop()`, mais `connectWebSocket()` peut bloquer jusqu'à 10 s
  - `manageLedError()` est désormais appelé dans la boucle d'attente interne de `connectWebSocket()` — animation maintenue pendant toute la tentative
- **[Firmware BuzzClick]**: Thread-safety FreeRTOS — variables LED cross-task déclarées `volatile`
  - Variables `currentRed`, `currentGreen`, `currentBlue`, `currentIntensity`, `g_ledErrorStepIdx`, `g_ledErrorLastStep` rendues `volatile` pour garantir la cohérence entre les tâches FreeRTOS (event task + `loop()`)
- **[Firmware BuzzClick]**: Replay des phases boot LED sur reconnexion WiFi (ORANGE 1/4 et 2/4)
  - `WiFiGotIP()` rejouait les phases LED de boot si `wsServerIP` était déjà connu (reconnexion après coupure WiFi)
  - Ajout d'un early return dans `WiFiGotIP()` si `wsServerIP` n'est pas vide — évite l'interruption du spinner WS_RECONNECTING
- **[Firmware BuzzClick]**: Commandes `LED_SET` serveur appliquées pendant une mise à jour OTA (fixes #62)
  - Les messages `LED_SET` reçus pendant l'OTA écrasaient les patterns d'avancement — risque d'état LED incohérent
  - Guard `otaInProgress` ajouté dans le handler `LED_SET` de `parseJSON()` : les messages sont ignorés si une OTA est en cours
- **[Tests Go]**: Alignement `updater_test.go` avec la signature `NewUpdater(version, dataDir)` (fixes #44)
  - Les tests appelaient l'ancienne signature sans `dataDir`, provoquant des erreurs de compilation
  - Tests mis à jour pour passer le répertoire de données en second argument

### Added
- **[Firmware BuzzClick]**: Indicateur visuel de reconnexion WebSocket non-intrusif (fixes #42)
  - Nouveau pattern LED `WS_RECONNECTING` : 1 pixel blanc se déplace sur l'anneau (23 LEDs × 100ms = ~2.3s/tour)
  - Delta-update : seuls 2 pixels modifiés par tick (restauration du pixel précédent au fond + avance du pixel blanc) via `applyLedColorToBuffer()`
  - Pattern `[bg][WHITE][bg]` — pas de voisins éteints, fond de couleur préservé intégralement
  - Déclenché immédiatement sur `WEBSOCKET_EVENT_DISCONNECTED` / `WEBSOCKET_EVENT_ERROR` — pas d'étape rouge fixe intermédiaire
  - La couleur d'équipe ou de réponse reste visible sur toutes les autres LEDs (overlay non-intrusif)
  - Au retour du serveur : `clearLedError()` + `resendLEDOnReconnect()` restaure l'état complet
- **[Tests Go]**: Tests de non-régression pour la détection merged binary et le serving OTA (commit 1b067e9)
  - `IsMergedBinary()` : 7 cas (magic présent/absent, longueurs limites, nil)
  - `GetAppFirmware()` : identité app-only, extraction merged à 0x10000, erreur merged-too-small
  - `SaveFirmware()` : merged ≤ 3 MB accepté, rejet au-delà de `FirmwareMaxSize`
  - `handleAPIFirmwareDownload` : app-only servie intacte, merged sert uniquement l'app ; `Content-Length` annonce la taille app-only pour OTA
  - Endpoint `merged.bin` : 404 pour app-only, binaire complet pour merged ; flag `IS_MERGED` dans `/api/firmware/buzzclick/version`
- **[UpdatePage]**: Persistance des téléchargements entre redémarrages (fixes #44)
  - Les binaires téléchargés sont stockés dans `data/updates/` avec leur numéro de version (ex: `buzzcontrol-v3.6.3-windows-amd64.exe`) au lieu du dossier temp OS
  - L'API `/api/updates` retourne `downloaded: true` + `local_path` si le binaire est déjà présent localement
  - Badge "✓ Téléchargé" affiché sur les versions déjà présentes — le bouton "Appliquer" est disponible sans re-téléchargement

---

## [3.6.2] - 2026-04-22

### Fixed
- **[CI/CD + Firmware]**: Le firmware publié en release et embarqué dans l'exe était app-only, empêchant le flash USB direct via esptool
  - Le pipeline génère désormais un merged binary (bootloader + partitions + boot_app0 + app) via `esptool merge_bin`
  - Le serveur auto-détecte le format et extrait l'app-only pour l'OTA (transparent pour les buzzers)
  - Fix SIZE mismatch dans `OTA_UPDATE` : le serveur annoncait la taille du merged mais servait l'app-only, causant un abandon OTA firmware-side

---

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
