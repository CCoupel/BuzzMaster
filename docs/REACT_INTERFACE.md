# Interface React - BuzzControl

Ce document décrit l'interface web React de BuzzControl.

## Structure des Pages

**Architecture des routes :**
- Route `/` : Page d'inscription VJoueurs (EnrollPage)
- Route `/player` : Interface de jeu VJoueur (VPlayerPage)
- Route `/tv` : Affichage TV plein écran (PlayerDisplay)
- Routes `/admin/*` : Pages d'administration
- Routes `/anim/*` : Alias des routes admin (même comportement)

| Route | Alias | Page | Description |
|-------|-------|------|-------------|
| `/` | - | EnrollPage | Page d'inscription VJoueurs |
| `/player` | - | VPlayerPage | Interface de jeu VJoueur (smartphone) |
| `/tv` | - | PlayerDisplay | Affichage TV (plein écran, statique) |
| `/admin` | `/anim` | GamePage | Interface admin principale (Jeu) |
| `/admin/scoreboard` | `/anim/scoreboard` | ScoresPage | Tableau des scores |
| `/admin/teams` | `/anim/teams` | TeamsPage | Gestion des joueurs et équipes |
| `/admin/quiz` | `/anim/quiz` | QuestionsPage | Gestion des questions |
| `/admin/settings` | `/anim/settings` | ConfigPage | Configuration |
| `/admin/history` | `/anim/history` | HistoryPage | Historique des événements |
| `/admin/palmares` | `/anim/palmares` | CategoryPalmaresPage | Palmarès par catégorie |
| `/admin/logs` | `/anim/logs` | LogsPage | Logs serveur temps réel |

**Navbar :**
- Affiché uniquement sur les routes `/admin/*` et `/anim/*`
- Préfixe dynamique : détecte `/anim` ou `/admin` depuis l'URL et construit les liens en conséquence
- Fonction `getFullPath(path)` pour construire les chemins avec le bon préfixe

## Composants Clés

| Composant | Fichier | Description |
|-----------|---------|-------------|
| Podium | `components/Podium.jsx` | Podium 1-2-3 avec gestion égalités (variantes: default, compact) |
| QuestionPreview | `components/QuestionPreview.jsx` | Aperçu 16:9 de l'affichage TV (utilise Podium) |
| TeamCard | `components/TeamCard.jsx` | Carte équipe compacte (260px) |
| Timer | `components/Timer.jsx` | Chronomètre avec barre de progression |
| Navbar | `components/Navbar.jsx` | Navigation + versions + compteurs clients |
| CategoryBalance | `components/CategoryBalance.jsx` | Visualisation équilibre catégories |

### Podium Component (v2.4.0)

Shared component for displaying rankings with tie support:
- **Variants**: `default` (full size), `compact` (smaller for admin/preview)
- **Tie handling**: Multiple teams/players with same score share the same rank
- **Animation**: Framer-motion for entrance animations and score changes

```jsx
<Podium teams={sortedTeams} variant="compact" />
```

### QuestionPreview Component (v2.11.0)

TV preview as iframe - perfect sync with actual /tv display:
- **Implementation**: Simple iframe pointing to `/tv`
- **Benefits**: Zero maintenance, always in sync, ~15 lines of code
- **Trade-off**: Double WebSocket connection (acceptable for admin preview)

## Layout GamePage (Admin) - v2.12.0

Layout avec timer pleine largeur + 3 colonnes harmonisées :
```
| Timer (pleine largeur, 95%)                    |  ligne 1
|------------------------------------------------|
| Questions | Contrôles + Aperçu TV | Équipes    |  ligne 2
| 280px     | 1fr (flexible)        | 280px      |
```

- **max-width** : 1800px (pour exploiter les grands écrans)
- **Breakpoints** : 1600px (250px), 1400px (220px), 1200px, 768px
- **Colonnes harmonisées** : Questions et Équipes ont la même largeur

**Responsive breakpoints:**
- `>1600px`: 3 columns (280px / 1fr / 280px)
- `1400-1600px`: 3 columns (250px / 1fr / 250px)
- `1200-1400px`: 3 columns (220px / 1fr / 220px)
- `768-1200px`: 2 columns (questions + controls / teams)
- `<768px`: 1 column (stacked)

## Affichage TV - Contrainte IMPORTANTE

**L'affichage TV (`/tv`) est STATIQUE et ne permet PAS de scroll.**
Toutes les vues TV doivent tenir entièrement à l'écran sans défilement :
- Utiliser `overflow: hidden` (jamais `auto` ou `scroll`)
- Dimensionner avec des unités viewport (`vh`, `vw`, `%`)
- Utiliser `flex` avec `min-height: 0` pour permettre le rétrécissement
- Limiter le contenu visible (ex: top 3, max 6 catégories)

### Vues TV disponibles (v2.34.0)

| Vue | Action REMOTE | Description |
|-----|---------------|-------------|
| JEU | `GAME` | Question, timer, réponses QCM |
| EQUIPES | `SCORE` | Podium des équipes (top 3) |
| JOUEURS | `PLAYERS` | Liste des joueurs par équipe |
| PALMARES | `PALMARES` | Classement par catégorie (grille 3x2, max 6 catégories) |

### PlayerDisplay 4-Zone Layout (v2.11.1)

Layout vertical en 4 zones avec hauteurs fixes pour l'affichage TV (/tv) :
- **Zone 1 - Timer** : 100px hauteur fixe, centré en haut
- **Zone 2 - Question** : 80px hauteur fixe, texte de la question
- **Zone 3 - Media** : flex: 1, remplit l'espace restant, image centrée
- **Zone 4 - Answers** : 120px hauteur fixe, `margin-top: auto` (aligné en bas)

**Timer couleur synchronisée :**
- Vert (`--success`) : > 50% du temps restant
- Orange (`--warning`) : 25-50% du temps (urgent)
- Rouge (`--error`) : < 25% du temps (critique)

**Affichage des phases de jeu (v2.40.0) :**

| Phase | Affichage TV | Description |
|-------|--------------|-------------|
| PREPARE | "NOUVELLE QUESTION" | Centré à l'écran, pas de catégorie |
| READY | Icône catégorie + Nom (fond coloré) | Grande icône animée (pulsante) |
| COUNTDOWN | Catégorie en haut + Décompte au centre | Animation de la catégorie du centre vers le haut |
| STARTED | Question + Média + Réponses | Affichage normal du jeu |

## VPlayer - Joueurs Virtuels (v2.45.0)

Permet aux joueurs de buzzer depuis leur smartphone en scannant un QR Code.

**Workflow :**
1. Admin ouvre `/anim/teams` → Zone "Inscriptions" → "Lancer Inscriptions"
2. QR Code s'affiche sur `/tv`
3. Joueurs scannent → arrivent sur `/` → saisissent pseudo → redirigés vers `/player`
4. Admin ferme inscriptions → QR Code disparaît, joueurs voient page d'attente
5. Joueurs sur `/player` peuvent buzzer pendant les questions
6. Si admin supprime un joueur → détection automatique → redirection vers `/`

**Page d'inscription (`/` - EnrollPage) :**
- Fond blanc pour lisibilité
- Si inscriptions fermées : "En attente de l'ouverture des inscriptions..."
- Formulaire : pseudo (2-20 caractères) + bouton "Rejoindre"
- Reconnexion auto : si joueur existe côté serveur → redirige vers `/player`
- Stockage localStorage : `vplayer_name`, `vplayer_session`

**Page de jeu (`/player` - VPlayerPage) :**

Layout responsive en 4 zones avec badges permanents non-intrusifs.

**BuzzButton États visuels :**

| Phase | Texte | Couleur | État |
|-------|-------|---------|------|
| NOT_ASSIGNED | "En attente..." | Gris | disabled |
| STOPPED | "En attente de question" | Gris | disabled |
| PREPARE | "Préparation..." | Orange | disabled |
| READY / COUNTDOWN | "Prêt !" | Cyan | disabled |
| STARTED | "BUZZ !" | Vert | active |
| PAUSED | "Déjà buzzé" | Bleu | disabled |

**Retour haptique :**
- Vibration 100ms au clic (si `navigator.vibrate` supporté)
- Animation visuelle pressing (300ms scale)

**Protection MEMORY :**
- Questions MEMORY ne peuvent pas être buzzées par VPlayers
- `engine.go:ProcessButtonPress()` ignore les buzz si TYPE="MEMORY"

**Actions WebSocket VPlayer :**
| Action | Direction | Description |
|--------|-----------|-------------|
| SHOW_QR_CODE | Admin→Server | Démarre enrollment |
| HIDE_QR_CODE | Admin→Server | Arrête enrollment |
| PLAYER_CONNECT | VPlayer→Server | Demande d'inscription |
| PLAYER_CONNECTED | Server→VPlayer | Inscription réussie |
| PLAYER_REJECTED | Server→VPlayer | Inscription refusée |
| ENROLLMENT_UPDATE | Server→All | Mise à jour compteur |

## Fonctionnalités UI

### Statuts des questions (couleurs)

| Statut | Couleur bordure | Fond | Apparence |
|--------|-----------------|------|-----------|
| AVAILABLE | Vert | Vert clair | Normal |
| STARTED | Orange | Orange clair | Normal |
| STOPPED | Rouge | Rouge clair | Normal |
| REVEALED | Gris | Gris | Compact (image/réponse masquées, opacité 50%) |

### Timer Phase Badges (v2.10.0)

Pastilles colorées indiquant l'état du jeu dans le composant Timer :
- **ARRET** (STOPPED) : Rouge (`--error`)
- **PREPARATION** : Orange (`--warning`)
- **PRET** : Cyan (`--accent-cyan`)
- **EN COURS** : Vert (`--success`)
- **PAUSE** : Bleu (`--primary-500`)
- **REPONSE** (REVEALED) : Gris (`--gray-400`)

### Points Animation (v2.12.0)

Animation visuelle quand des points sont ajoutés :
- **Confetti** : Particules avec la couleur de l'équipe
- **Animation flottante** : Nom de l'équipe + "+X pts" au centre de l'écran
- **Durée** : 2.5 secondes puis disparition
- **Vue JOUEURS** : Animation sur la ligne du joueur (scale + couleur verte)

### Debug Features (v2.12.0)

Fonctionnalités de test pour l'admin :
- **Ctrl+clic sur joueur** : Simule un appui buzzer (pendant STARTED/PAUSED)
- **Ctrl+clic sur question** : Force l'état READY sans attendre les PONGs

### Waiting States (v2.12.0)

États visuels pour équipes/joueurs :
- **PREPARE/READY** : Grisés jusqu'à réception du PONG
- **STARTED/PAUSED** : Grisés jusqu'au buzz
- **Après buzz** : Visibilité restaurée avec couleur d'équipe

### Question Reordering (v2.7.0)

Drag and drop pour réordonner les questions :
- **Interface** : Glisser-déposer les cartes de questions dans QuestionsPage
- **Poignée** : Icône sur chaque carte pour indiquer le drag
- **Feedback visuel** : Opacité réduite pendant le drag, bordure pointillée sur la cible
- **Persistance** : Champ `ORDER` dans chaque `question.json`
- **Tri** : Questions triées par `ORDER` si disponible, sinon par `ID`

### Teams Page - Drag & Drop (v2.5.0)

Interface de gestion des équipes avec drag & drop :
- **Gauche** : Grille des équipes (zones de dépôt)
- **Droite (320px)** : Joueurs non assignés
- **Drag & Drop** : Glisser un joueur sur une équipe pour l'assigner
- **Désassigner** : Glisser vers la zone "non assignés"

### Couleurs de Réponse (v2.5.0)

Chaque joueur peut avoir une couleur de réponse pour le mode QCM :
- **Couleurs disponibles** : Rouge (A), Vert (B), Jaune (C), Bleu (D)
- **Sélection** : Uniquement quand le joueur n'est PAS assigné à une équipe
- **Affichage** : La couleur devient le fond de l'avatar du joueur
- **Champ** : `ANSWER_COLOR` dans le modèle Bumper

### QCM Team Badges (v2.16.0)

Pastilles d'équipes sur les réponses QCM pendant STOPPED/REVEALED :
- **Affichage** : Pastilles colorées sur chaque réponse QCM montrant quelles équipes ont répondu
- **Couleur** : Couleur de l'équipe (pas la couleur de la réponse QCM)
- **Taille dégradée** : 70% (première) à 40% (dernière) de la taille de base (60px)
- **Tri** : Par temps de réponse (plus rapide = plus grand, à gauche)

### Media Answer (v2.14.0)

Support des images de réponse distinctes de l'image de question :
- **MEDIA** : Image affichée pendant les phases STARTED et PAUSED
- **MEDIA_ANSWER** : Image de réponse qui REMPLACE MEDIA pendant la phase REVEALED
- **Effet visuel** : Cadre vert pulsant autour de l'image de réponse pendant REVEALED
- **Thumbnails** : Vignette de l'image réponse affichée en bas à droite des cartes questions

### CategoryBalance Component (v2.23.0)

Visualisation de l'équilibre des catégories sur la page Questions :
- **Affichage** : Barre horizontale avec toutes les catégories représentées
- **Données par catégorie** : Nombre de questions, Total des points
- **Code couleur** :
  - <= 25% écart : Vert (Équilibré)
  - 25% - 50% écart : Orange (Attention)
  - > 50% écart : Rouge (Déséquilibré)

### History Page (v2.20.0)

Page d'historique des événements de jeu :
- **Route** : `/history-page`
- **Endpoint API** : `GET /history` retourne `[]GameEvent`
- **Fonctionnalités** :
  - Événements groupés par question (ordre chronologique)
  - Vue collapsible : clic sur l'en-tête pour ouvrir/fermer
  - Boutons "Tout ouvrir" / "Tout fermer"
  - **Vue réduite** : Résumé des points par équipe et par joueur (badges colorés)
  - **Vue détaillée** : Tableau avec Heure, Équipe, Joueur, Temps, Points

### Logs Page (v2.42.0)

Page de visualisation des logs serveur en temps réel :
- **Route** : `/admin/logs` et `/anim/logs`
- **Fonctionnalités** :
  - Affichage temps réel des logs via WebSocket dédiée
  - Filtrage par niveau : DEBUG (gris), INFO (blanc), WARN (orange), ERROR (rouge)
  - Filtrage par composant : App, Engine, HTTP, WebSocket, TCP, UDP
  - Recherche avec debounce 300ms et highlight des termes
  - Auto-scroll intelligent (pause au scroll manuel, reprise en bas)
  - Export des logs filtrés au format `.log`

## WebSocket Messages

Le hook `useWebSocket.js` gère la communication :

| Message reçu | Données | Utilisation |
|--------------|---------|-------------|
| UPDATE | `{GAME, teams, bumpers}` + VERSION | État du jeu + version serveur |
| QUESTIONS | `{questions}` + FSINFO + VERSION | Liste questions + espace disque |
| CLIENTS | `{ADMIN_COUNT, TV_COUNT, VPLAYER_COUNT}` | Compteurs clients connectés |
| BACKGROUND_CHANGE | `{INDEX}` | Index de l'image de fond courante (synchronisé) |

## CSS Specificity & Layout Fixes (v2.32.0)

**Problème résolu** : Conflits CSS entre GamePage.css et TeamsPage.css sur les mêmes classes `.teams-grid` et `.team-card`.

**Solution - Sélecteurs spécifiques** :
Les styles de GamePage utilisent des sélecteurs plus spécifiques pour éviter que TeamsPage.css ne les écrase :

```css
/* GamePage.css - Sélecteurs spécifiques à la page Jeu */
.game-page .teams-grid { display: flex; flex-direction: column; ... }
.game-page .teams-grid .team-card { overflow: visible; flex-shrink: 0; ... }
.game-page .team-card .team-buzzers { display: flex !important; ... }
.game-page .team-card .buzzer-mini { display: flex !important; ... }
```
