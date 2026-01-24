# Changelog - BuzzControl

Historique des versions du projet BuzzControl.


## [2.44.3] - 2026-01-24

### Corrigé
- **Synchronisation compteur joueurs virtuels** : Le compteur affiché sur la page Équipes (/teams) est maintenant synchronisé avec celui de l'affichage TV (/tv)
  - Utilise `gameState.virtualPlayerCount` (source de vérité serveur) au lieu d'un calcul local
  - Affichage séparé : 🎮 joueurs physiques et 📱 joueurs virtuels
  - Ajout des champs `PhaseEnroll`, `IsVirtual` et des actions protocole associées

### Technique
- `TeamsPage.jsx` : Import de `gameState` depuis `useGame()`, calcul `physicalBumperCount` et utilisation de `virtualPlayerCount`
- `TeamsPage.css` : Styles `.bumper-counts`, `.bumper-count.physical`, `.bumper-count.virtual`
- `models.go` : Ajout `PhaseEnroll`, `IsVirtual`, `VirtualPlayerCount`, `VirtualPlayerLimit`, `EnrollmentActive`, `ShowQRCode`
- `messages.go` : Ajout actions PLAYER_CONNECT, PLAYER_CONNECTED, PLAYER_REJECTED, SHOW_QR_CODE, HIDE_QR_CODE

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
