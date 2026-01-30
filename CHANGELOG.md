# Changelog - BuzzControl

Historique des versions du projet BuzzControl.


## [2.46.0] - 2026-01-30

### Ajouts
- **[Effet Néon]**: Bordure lumineuse animée autour de l'écran TV et VPlayer
  - Couleur automatique selon la catégorie de la question
  - Animation CSS conic-gradient avec rotation spatiale continue
  - Configurable depuis la page Configuration avec 4 paramètres ajustables
  - **Paramètres disponibles** :
    - `enabled` : Activer/désactiver l'effet (défaut: désactivé)
    - `arc_width` : Largeur de l'arc lumineux en degrés, 30-180° (défaut: 60°)
    - `intensity_gap` : Écart d'intensité (opacité) 0-100% (défaut: 80%)
    - `rotation_speed` : Vitesse de rotation en secondes, 1-10s (défaut: 4s)
  - **Phases actives** : READY, COUNTDOWN, STARTED, PAUSED
  - Diffusion en temps réel via WebSocket : changement config appliqué instantanément

### Technique
- **Backend** : Configuration NeonEffectConfig dans config.json
- **Frontend** : CSS @property + conic-gradient pour animations GPU-accelerated
- **Diffusion** : ACTION CONFIG_UPDATE broadcast à tous les clients WebSocket
- **Styles** : neon.css avec variables CSS dynamiques pour personnalisation

### Fichiers modifiés
- `server-go/internal/config/config.go` : Struct NeonEffectConfig
- `server-go/internal/protocol/messages.go` : ACTION CONFIG_UPDATE
- `server-go/internal/server/http.go` : GET/POST /config.json avec neon_effect
- `server-go/web/src/styles/neon.css` : Styles effet néon avec @property
- `server-go/web/src/constants/colors.js` : Fonction getCategoryColor()
- `server-go/web/src/pages/PlayerDisplay.jsx` : Classe neon-border conditionnelle
- `server-go/web/src/pages/ConfigPage.jsx` : Section sliders configuration néon
- `server-go/web/src/hooks/useWebSocket.js` : Handler CONFIG_UPDATE
- `server-go/config.json` : Version 2.46.0

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
