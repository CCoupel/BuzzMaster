# Page Joueur (/player)

**Statut** : ✅ Phase 1 Complète (v2.45.0)

## Concept

**Surcouche légère sur `/tv`** permettant aux joueurs de buzzer depuis leur smartphone. L'affichage suit automatiquement le rythme du jeu géré par l'animateur, sans contrôle supplémentaire pour le joueur.

### Terminologie

- **Joueur Physique** : Joueur avec un buzzer ESP32 (BuzzClick)
- **VJoueur** (Joueur Virtuel) : Joueur connecté depuis un navigateur web via `/player`
  - Utilise son smartphone/tablette comme buzzer
  - **Doit obligatoirement entrer un nom/pseudo unique** lors de l'enrôlement
  - Apparaît comme un bumper virtuel dans `/admin/teams`
  - Fonctionne exactement comme un buzzer physique pour le gameplay
- **Phase ENROLL** : Période pendant laquelle les VJoueurs peuvent s'enregistrer
  - Contrôlée par l'admin depuis `/admin/teams`
  - QR code affiché sur `/tv` dans la zone MEDIA

### Principe de conception

> `/player` = `/tv` (affichage synchronisé) + Header personnalisé (nom/équipe) + Zone BUZZ tactile

- **Réutilisation maximale** : Même composant `PlayerDisplay` que `/tv`
- **Pas de navigation** : Le joueur ne change jamais de page, l'affichage s'adapte automatiquement
- **Contrôle minimal** : Uniquement buzzer, pas de stats détaillées ni de dashboard complexe
- **Gestion par l'animateur** : Tout est piloté depuis `/admin`
- **Accès via QR Code** : Les joueurs scannent un QR code affiché sur `/tv` pour rejoindre facilement
- **Identification obligatoire** : Chaque VJoueur doit entrer un nom/pseudo unique (unicité gérée par le serveur)
- **Persistance** : Cookie/localStorage pour reconnexion automatique (durée paramétrable, défaut 24h)
- **Buzz tactile** : Tap au centre de l'écran (zone média) pour buzzer

---

## Organisation des routes (définitive)

### Structure des routes

| Route | Composant | Description |
|-------|-----------|-------------|
| **`/`** | `EnrollPage` | Page d'enrôlement VJoueur (saisie pseudo) |
| **`/player`** | `VPlayerPage` | Interface de jeu VJoueur (équivalent /tv + buzz) |
| **`/tv`** | `PlayerDisplay` | Affichage TV (+ QR code pendant ENROLL) |
| **`/admin`** | `GamePage` | Interface admin principale |
| `/admin/scoreboard` | `ScoresPage` | Tableau des scores |
| `/admin/teams` | `TeamsPage` | Gestion équipes + zone ENROLL |
| `/admin/quiz` | `QuestionsPage` | Gestion questions |
| `/admin/history` | `HistoryPage` | Historique des événements |
| `/admin/palmares` | `CategoryPalmaresPage` | Palmarès par catégorie |
| `/admin/settings` | `ConfigPage` | Configuration |

**Alias `/anim/*`** : Toutes les routes `/admin/*` ont un alias `/anim/*` (même comportement)

### Schéma des routes

```
/                           # Page d'enrôlement VJoueur (scan QR → saisie pseudo)
│
/player                     # Interface de jeu VJoueur (après enrôlement)
│
/tv                         # Affichage TV (lecture seule + QR code pendant ENROLL)
│
/admin (ou /anim)           # Interface admin
├── /admin/scoreboard       # Scores
├── /admin/teams            # Équipes + zone ENROLL
├── /admin/quiz             # Questions
├── /admin/history          # Historique
├── /admin/palmares         # Palmarès
└── /admin/settings         # Configuration
```

### Flux utilisateur

```
[QR Code sur /tv]
       │
       ▼
[Scan smartphone]
       │
       ▼
[/] Page d'enrôlement ──► Saisie pseudo ──► Validation serveur
       │                                            │
       │ (cookie existant)                          │ (succès)
       ▼                                            ▼
[/player] Interface VJoueur ◄───────────────────────┘
```

---

## Phase ENROLL - Configuration Admin

### Zone de configuration dans TeamsPage

La configuration de l'enrôlement se situe dans `/admin/teams`, **dans la colonne "Joueurs non assignés"**, entre les compteurs de joueurs et la liste des joueurs non assignés.

```
┌─────────────────────────────────────┐
│ JOUEURS NON ASSIGNÉS (3)            │  ← Titre + compteur
├─────────────────────────────────────┤
│ ┌─────────────────────────────────┐ │
│ │ 📱 ENRÔLEMENT VJOUEURS          │ │
│ │                                 │ │
│ │ Places max : [____10____] ▼    │ │  ← Nombre max VJoueurs
│ │                                 │ │
│ │ VJoueurs : 3/10                 │ │  ← Compteur actuel
│ │                                 │ │
│ │ [ ▶ DÉMARRER ENROLL ]           │ │  ← Bouton toggle
│ │   ou                            │ │
│ │ [ ⏹ ARRÊTER ENROLL ] (actif)   │ │
│ └─────────────────────────────────┘ │
├─────────────────────────────────────┤
│ [Carte joueur non assigné 1]        │  ← Liste des joueurs
│ [Carte joueur non assigné 2]        │
│ ...                                 │
└─────────────────────────────────────┘
```

### Actions admin

- [x] **Démarrer ENROLL** *(v2.45.0)*
  - Définir le nombre max de VJoueurs (champ numérique, défaut: 10)
  - Bouton "▶ Lancer Inscriptions"
  - Envoie action WebSocket `SHOW_QR_CODE`
  - Le QR code s'affiche sur `/tv` en overlay plein écran
  - État serveur : `enrollmentActive = true`, `virtualPlayerLimit = n`

- [x] **Arrêter ENROLL** *(v2.45.0)*
  - Bouton "⏹ Fin Inscriptions"
  - Envoie action WebSocket `HIDE_QR_CODE`
  - Le QR code disparaît de `/tv`
  - État serveur : `enrollmentActive = false`
  - Les VJoueurs déjà enrôlés restent actifs
  - Les reconnexions restent toujours autorisées

- [x] **Compteur temps réel** *(v2.45.0)*
  - Affichage "Inscrits: X/Y" dans TeamsPage
  - Se met à jour en temps réel via WebSocket (action `ENROLLMENT_UPDATE`)
  - Barre de progression sur le QR code overlay

---

## QR Code sur /tv (Phase ENROLL)

### Concept

Pendant la phase ENROLL, le QR code s'affiche **dans la zone MEDIA** de `/tv` (pas en overlay). Les joueurs scannent le code pour accéder à la page d'enrôlement.

**Distinction importante** :
- **Phase ENROLL ACTIVE** = QR code visible dans zone MEDIA → Nouveaux VJoueurs acceptés (jusqu'au max)
- **Phase ENROLL INACTIVE** = Zone MEDIA normale → Seules les reconnexions sont acceptées

### Avantages

✅ **Simplicité** : Pas besoin de taper l'URL
✅ **Visibilité maximale** : QR code dans la zone MEDIA (grande taille)
✅ **Sécurité** : Les joueurs ne peuvent pas "tomber" sur `/admin` par erreur
✅ **Contrôle de l'enrôlement** : L'animateur décide quand accepter de nouveaux VJoueurs
✅ **Limite configurable** : Nombre max de VJoueurs paramétrable
✅ **Reconnexion toujours possible** : Les VJoueurs déconnectés peuvent revenir même hors phase ENROLL
✅ **UX fluide** : Scan → Saisie pseudo → Jouer

### Affichage QR Code sur /tv (Phase ENROLL)

```
┌────────────────────────────────────┐
│           ZONE TIMER               │
├────────────────────────────────────┤
│           ZONE QUESTION            │
│   "Scannez pour rejoindre !"       │
├────────────────────────────────────┤
│                                    │
│         ┌──────────────┐           │
│         │ ▓▓▓▓▓▓▓▓▓▓▓▓ │           │
│         │ ▓▓░░░░░░░░▓▓ │           │
│         │ ▓▓░░▓▓▓▓░░▓▓ │           │  ZONE MEDIA
│         │ ▓▓░░▓▓▓▓░░▓▓ │           │  = QR CODE
│         │ ▓▓░░░░░░░░▓▓ │           │
│         │ ▓▓▓▓▓▓▓▓▓▓▓▓ │           │
│         └──────────────┘           │
│                                    │
│   ┌────────────────────────────┐   │
│   │████████████░░░░░░░░░░░░░░░░│   │  ← BARRE DE PROGRESSION
│   └────────────────────────────┘   │
│        VJoueurs : 3/10             │  ← Compteur texte
│                                    │
├────────────────────────────────────┤
│          ZONE RÉPONSES             │
└────────────────────────────────────┘
```

**Barre de progression :**
- Largeur proportionnelle : `(current / max) * 100%`
- Couleur : Vert si < 80%, Orange si 80-99%, Rouge si complet (100%)
- Animation de remplissage progressive
- Texte "X/Y" centré sous la barre

### Implémentation ✅ Complète (v2.45.0)

- [x] **Action WebSocket SHOW_QR_CODE**
  - Envoyé par admin depuis TeamsPage
  - Serveur : `enrollmentActive = true`, `showQRCode = true`
  - Broadcast à tous les clients `/tv` : afficher QR code overlay

- [x] **Action WebSocket HIDE_QR_CODE**
  - Envoyé par admin depuis TeamsPage
  - Serveur : `enrollmentActive = false`, `showQRCode = false`
  - Broadcast à tous les clients `/tv` : masquer QR code

- [x] **Action WebSocket ENROLLMENT_UPDATE**
  - Broadcast quand un VJoueur s'enrôle ou se déconnecte
  - Payload dans GameState : `{VIRTUAL_PLAYER_COUNT, VIRTUAL_PLAYER_LIMIT}`
  - Mise à jour du compteur sur admin et TV

- [x] **Génération du QR code**
  - Bibliothèque : `qrcode.react` (npm)
  - URL dynamique : `http://${window.location.hostname}/`
  - Composants : `QRCodeOverlay.jsx` + `QRCodeDisplay.jsx`
  - Overlay plein écran avec barre de progression joueurs

```javascript
// Exemple React - Dans PlayerDisplay.jsx
import QRCode from 'qrcode'

const [qrCodeUrl, setQrCodeUrl] = useState('')

useEffect(() => {
  if (gameState.enrollmentActive) {
    QRCode.toDataURL(`http://${serverIP}/`, { width: 400 })
      .then(url => setQrCodeUrl(url))
  }
}, [gameState.enrollmentActive, serverIP])

// Dans la zone MEDIA
{gameState.enrollmentActive ? (
  <div className="enroll-qr-zone">
    <img src={qrCodeUrl} alt="QR Code" className="enroll-qr-code" />
    <p className="enroll-counter">VJoueurs : {gameState.vPlayerCount}/{gameState.maxVPlayers}</p>
  </div>
) : (
  <MediaDisplay media={question.MEDIA} />
)}
```

---

## Phase 1 - MVP (v2.41.0) ✅ IMPLÉMENTÉE

### Fichiers implémentés

| Fichier | Description |
|---------|-------------|
| `web/src/pages/EnrollPage.jsx` | Page d'enrôlement `/` (saisie pseudo) |
| `web/src/pages/EnrollPage.css` | Styles page d'enrôlement |
| `web/src/pages/VPlayerPage.jsx` | Page VJoueur `/player` avec buzz tactile |
| `web/src/pages/VPlayerPage.css` | Styles spécifiques VPlayer (badges, overlay buzz) |
| `web/src/pages/PlayerDisplay.jsx` | Composant réutilisé avec props `isVPlayer`, `onMediaClick` |
| `web/src/pages/PlayerDisplay.css` | Styles partagés (timer 95%, zones) |
| `web/src/components/QRCodeOverlay.jsx` | Overlay QR code sur /tv |
| `web/src/components/QRCodeDisplay.jsx` | Composant de génération QR code |
| `web/src/pages/TeamsPage.jsx` | Zone enrollment (boutons + compteur) |
| `internal/game/models.go` | Champs GameState (EnrollmentActive, ShowQRCode, etc.) |
| `internal/game/engine.go` | CreateVirtualPlayer, StartEnrollment, StopEnrollment |
| `cmd/server/main.go` | Handlers SHOW_QR_CODE, HIDE_QR_CODE, PLAYER_CONNECT |

### Caractéristiques implémentées

- **Badges responsives** : Positionnés à 15% (nom) et 85% (équipe), taille avec `clamp()`
- **Timer** : Barre de progression à 95% largeur (même que zone-answers)
- **Zone cliquable** : 76% largeur (80% de zone-answers)
- **Overlay buzz** : Checkmark vert animé + bordure pulsante
- **Reconnexion** : Détection suppression bumper par admin → redirection `/`
- **Auto-PONG** : Envoi automatique en phase PREPARE

### Routes

| Route | Description | Statut |
|-------|-------------|--------|
| `/` | Page d'enrôlement VJoueur (saisie pseudo) | ✅ |
| `/player` | Page de jeu VJoueur (équivalent /tv + buzz) | ✅ |
| `/tv` | Affichage TV (inchangé) | ✅ |
| `/admin/*` | Pages admin (inchangé) | ✅ |

### Page `/` - Enrôlement VJoueur

Page de saisie du pseudo, accessible via scan du QR code affiché sur `/tv`.

```
┌────────────────────────────────────┐
│                                    │
│         🐝 BuzzControl             │
│                                    │
│    ┌────────────────────────┐      │
│    │  Entrez votre pseudo   │      │
│    │                        │      │
│    │  [________________]    │      │
│    │                        │      │
│    │  ⚠️ 2-20 caractères    │      │
│    │                        │      │
│    │   [ REJOINDRE ]        │      │
│    └────────────────────────┘      │
│                                    │
└────────────────────────────────────┘
```

- [x] **Champ pseudo**
  - Minimum 2 caractères, maximum 20 caractères
  - Validation en temps réel côté client
  - Bouton "Rejoindre" désactivé si invalide

- [x] **Unicité des pseudos (côté serveur)**
  - Vérification lors de `PLAYER_CONNECT`
  - Si pseudo déjà pris : recherche du bumper existant pour reconnexion

- [x] **Gestion erreurs** *(v2.45.0)*
  - Backend : `EnrollmentError` avec raisons (`ENROLLMENT_CLOSED`, `ENROLLMENT_FULL`, `PSEUDO_TAKEN`)
  - Action `PLAYER_REJECTED` envoyée au client avec la raison
  - Frontend : Message "Les inscriptions ne sont pas ouvertes" dans EnrollPage

- [x] **Après enrôlement réussi**
  - Redirection automatique vers `/player`
  - Sauvegarde localStorage (`vplayer_name`, `vplayer_session`)

### Page `/player` - Interface VJoueur

Équivalent à `/tv` avec header personnalisé et zone de buzz tactile.

```
┌────────────────────────────────────┐
│ 🔴 Alice      [TIMER]   Les Rouges │ ← Header : nom (gauche) + équipe (droite)
├────────────────────────────────────┤
│           ZONE QUESTION            │
├────────────────────────────────────┤
│                                    │
│                                    │
│         ZONE MÉDIA                 │ ← TAP POUR BUZZER
│         (zone tactile)             │
│                                    │
│                                    │
├────────────────────────────────────┤
│          ZONE RÉPONSES             │
└────────────────────────────────────┘
```

**Header personnalisé :** ✅ Implémenté
- **Gauche (15%)** : Badge nom du VJoueur avec couleur QCM assignée
- **Centre** : Timer avec barre de progression (95% largeur)
- **Droite (85%)** : Badge équipe avec couleur de l'équipe
- Badges responsives avec clamp() (vh/vw)
- Si non assigné : badge équipe absent

**Zone de buzz tactile :** ✅ Implémenté
- Tap sur la zone média (76% largeur) pour buzzer
- Toute la zone média est cliquable via `onMediaClick`
- Overlay de confirmation avec checkmark vert

### Feedback visuel du buzz ✅ Implémenté (partiel)

Le VJoueur reçoit un retour visuel selon que son buzz est accepté.

**Buzz VALIDÉ (accepté par le serveur) :** ✅
- Overlay vert couvrant tout l'écran avec animation pulsante
- Checkmark vert animé (✓) avec effet "pop"
- Texte "BUZZÉ !" en vert
- Bordure verte épaisse (8px) avec glow

**Buzz REFUSÉ (ignoré par le serveur) :**
- Flash rouge sur toute la zone média
- Bordure rouge épaisse (4px) pendant 500ms
- Vibration courte (25ms)
- Message d'erreur discret (ex: "Trop tard", "Déjà buzzé")

**Raisons de refus possibles :**
- `ALREADY_BUZZED` : Le VJoueur a déjà buzzé cette question
- `TEAM_ALREADY_BUZZED` : Un autre joueur de l'équipe a buzzé
- `GAME_NOT_STARTED` : La phase n'est pas STARTED
- `NOT_ASSIGNED` : Le VJoueur n'est pas assigné à une équipe

### Mode Debug (feedback refusé)

Option pour afficher visuellement les buzz refusés (utile pour debug).

- [ ] **Toggle dans GamePage (admin)**
  - Checkbox "Afficher buzz refusés sur VJoueur"
  - Par défaut : OFF (désactivé)
  - Quand ON : Le VJoueur voit le flash rouge si son buzz est refusé
  - Quand OFF : Aucun feedback visuel si buzz refusé (comportement discret)

- [ ] **Action WebSocket BUZZ_RESULT**
  - Envoyé par le serveur au VJoueur après un buzz
  - Payload succès : `{STATUS: "ACCEPTED", TIME: 342}`
  - Payload refus : `{STATUS: "REJECTED", REASON: "TEAM_ALREADY_BUZZED"}`
  - Le client affiche le feedback visuel approprié

- [ ] **Configuration GameState**
  ```go
  type GameState struct {
    // ...
    DebugShowRejectedBuzz bool `json:"DEBUG_SHOW_REJECTED_BUZZ"`
  }
  ```

**Comportement par défaut (debug OFF) :**
| Buzz | Feedback visuel | Vibration |
|------|-----------------|-----------|
| Validé | ✅ Flash vert | ✅ Longue (100ms) |
| Refusé | ❌ Aucun | ❌ Aucune |

**Comportement debug (debug ON) :**
| Buzz | Feedback visuel | Vibration |
|------|-----------------|-----------|
| Validé | ✅ Flash vert | ✅ Longue (100ms) |
| Refusé | ✅ Flash rouge | ✅ Courte (25ms) |

### Persistance de l'identité VJoueur ✅ Implémenté

- [x] **localStorage**
  - Clés : `vplayer_name`, `vplayer_session`
  - Persistance jusqu'à suppression manuelle ou suppression admin

- [x] **Reconnexion automatique**
  - Au chargement de `/player` : vérifie localStorage
  - Si `vplayer_name` et `vplayer_session` présents → restauration automatique
  - Si absents → redirection vers `/` (page d'enrôlement)
  - En cas de déconnexion : envoi automatique `PLAYER_CONNECT` après 2s
  - Détection suppression par admin → redirection vers `/`

- [ ] **Configuration serveur** (config.json) - *Optionnel, valeurs par défaut fonctionnelles*
  ```json
  {
    "vplayer": {
      "session_duration_hours": 24,
      "max_players": 50
    }
  }
  ```

### Validation côté serveur ✅ Implémenté (v2.45.0)

- [x] **Logique d'enrôlement complète**

  **État serveur (GameState)** :
  ```go
  VirtualPlayerCount  int  `json:"VIRTUAL_PLAYER_COUNT"`  // Nombre de VJoueurs enrôlés
  VirtualPlayerLimit  int  `json:"VIRTUAL_PLAYER_LIMIT"`  // Limite max configurée
  EnrollmentActive    bool `json:"ENROLLMENT_ACTIVE"`     // Phase ENROLL active
  ShowQRCode          bool `json:"SHOW_QR_CODE"`          // QR code affiché sur /tv
  ```

  **Logique serveur lors de `PLAYER_CONNECT`** (dans `main.go:handlePlayerConnect`) :
  - Validation pseudo (2-20 caractères)
  - Recherche bumper existant pour reconnexion
  - Vérification enrôlement actif
  - Vérification limite joueurs
  - Création via `engine.CreateVirtualPlayer()`
  - Envoi `PLAYER_CONNECTED` ou `PLAYER_REJECTED`

  **Création d'un nouveau VJoueur** :
  - Bumper virtuel avec `IS_VIRTUAL: true`
  - ID généré : `vplayer_<timestamp>`
  - État initial : NON ASSIGNÉ
  - Apparaît dans `/admin/teams` comme un buzzer standard

  **Reconnexion d'un VJoueur existant** :
  - Recherche par `NAME` dans les bumpers virtuels existants
  - Restauration automatique de l'état complet

- [x] **Attribution par l'admin**
  - VJoueur dans la liste des joueurs non assignés
  - Drag & drop vers une équipe (workflow existant)
  - Attribution couleur QCM (interface existante)

**Payload PLAYER_CONNECT :**
```json
{
  "ACTION": "PLAYER_CONNECT",
  "MSG": {
    "NAME": "Alice"
  }
}
```

**Réponse PLAYER_CONNECTED (nouveau VJoueur) :**
```json
{
  "ACTION": "PLAYER_CONNECTED",
  "MSG": {
    "SESSION_ID": "vplayer_abc123",
    "NAME": "Alice",
    "STATUS": "UNASSIGNED",
    "IS_RECONNECTION": false
  }
}
```

**Réponse PLAYER_CONNECTED (reconnexion VJoueur existant) :**
```json
{
  "ACTION": "PLAYER_CONNECTED",
  "MSG": {
    "SESSION_ID": "vplayer_abc123",
    "NAME": "Alice",
    "STATUS": "ASSIGNED",
    "IS_RECONNECTION": true,
    "TEAM": "Les Rouges",
    "TEAM_COLOR": [255, 0, 0],
    "ANSWER_COLOR": "RED",
    "SCORE": 25
  }
}
```

**Réponse d'erreur (enrôlement fermé) :**
```json
{
  "ACTION": "PLAYER_CONNECT_ERROR",
  "MSG": {
    "ERROR": "ENROLLMENT_CLOSED",
    "MESSAGE": "L'enrôlement est fermé. Contactez l'animateur."
  }
}
```

### 2. Mini header personnalisé (80px fixe)

- [ ] **Affichage compact selon état d'assignation**

  **Si NON ASSIGNÉ** (pas encore attribué par l'admin) :
  - Avatar circulaire (40px) gris avec icône 📱
  - Nom du joueur (tronqué si trop long)
  - Texte : "En attente d'assignation..."
  - Indicateur de connexion (point vert/rouge)

  ```
  ┌──────────────────────────────────────┐
  │ 📱 Alice  •  En attente...           │
  └──────────────────────────────────────┘
  ```

  **Si ASSIGNÉ** (équipe attribuée par l'admin) :
  - Avatar circulaire (40px) avec couleur de l'équipe
  - Nom du joueur (tronqué si trop long)
  - Nom de l'équipe
  - Score personnel (ex: "25 pts")
  - Indicateur de connexion (point vert/rouge)

  ```
  ┌──────────────────────────────────────┐
  │ 🔴 Alice  •  Les Rouges  •  25 pts  │
  └──────────────────────────────────────┘
  ```

### 3. Zone PlayerDisplay (réutilisée à 100%) ✅ Implémenté

- [x] **Import du composant existant**
  ```jsx
  import PlayerDisplay from './PlayerDisplay'

  // VPlayerPage.jsx
  <PlayerDisplay
    playerName={bumper?.NAME}
    playerNameColor={getPlayerNameColor()}
    teamName={team?.NAME}
    teamColor={getTeamColor()}
    isVPlayer={true}
    onMediaClick={handleBuzz}
  />
  ```

- [x] **Comportement identique à `/tv`**
  - Affichage de la question en cours
  - Timer synchronisé (95% largeur, barre de progression)
  - Média (image/vidéo) dans zone cliquable (76% largeur)
  - Réponses QCM affichées
  - Grille Memory affichée
  - Changement de vue selon `gameState.PAGE`

### 4. Zone Bouton BUZZ ✅ Implémenté (via zone média)

**Note** : Implémentation différente de la spécification initiale - le buzz se fait via la zone média cliquable (76% largeur), pas via un bouton séparé en bas.

- [x] **Zone média cliquable**
  - Taille : 76% largeur (80% de zone-answers 95%)
  - Zone : `.zone-media` avec `cursor: pointer`
  - Callback : `onMediaClick={handleBuzz}`

- [x] **États du buzz** ✅ Implémenté

  | État | Comportement | Implémentation |
  |------|--------------|----------------|
  | **STARTED/PAUSED** | ✅ Buzz autorisé | `handleBuzz()` envoie `BUTTON` |
  | **Autres phases** | Buzz ignoré | Vérification phase dans `handleBuzz()` |
  | **MEMORY** | Buzz bloqué | Question.TYPE === 'MEMORY' |
  | **Déjà buzzé** | Overlay vert affiché | `bumper.TIME > 0` |

  **Simplification** : Pas de bouton séparé avec états visuels - la zone média est toujours visible, le buzz est autorisé/ignoré selon la phase.

- [x] **Envoi de l'action au clic** ✅ Implémenté
  ```javascript
  // VPlayerPage.jsx - handleBuzz()
  const handleBuzz = () => {
    if (!bumper || !bumper.id) return
    if (gameState.phase !== 'STARTED' && gameState.phase !== 'PAUSED') return
    if (gameState.question?.TYPE === 'MEMORY') return

    sendMessage('BUTTON', { ID: bumper.id, button: 'A' })
  }
  ```

- [x] **Feedback immédiat** ✅ Implémenté
  - Overlay vert avec checkmark animé
  - Animation pulsante sur la bordure
  - Texte "BUZZÉ !" affiché

---

## Phase 2 - QCM interactif (v2.46.0)

### QCM : Boutons de réponse à la place du BUZZ

- [ ] **Détection du type de question**
  - Si `gameState.QUESTION.TYPE === 'QCM'` → Afficher 4 boutons au lieu d'un seul
  - Sinon → Bouton BUZZ classique

- [ ] **4 boutons colorés (disposition 2x2)**
  ```
  ┌─────────────────────────────────┐
  │ [A] Rouge: Paris      [B] Vert  │
  │                                 │
  │ [C] Jaune: Berlin     [D] Bleu  │
  └─────────────────────────────────┘
  ```

- [ ] **Clic sur un bouton = buzz avec couleur**
  ```javascript
  const handleQcmAnswer = (color) => {
    sendWebSocketMessage({
      ACTION: 'BUTTON',
      ID: playerId,
      MSG: { button: color } // 'RED', 'GREEN', 'YELLOW', 'BLUE'
    })
  }
  ```

- [ ] **Affichage des indices (si activés)**
  - Réponses invalidées : bouton barré + grisé
  - Badge de pénalité au-dessus des boutons : "⚠️ Pénalité -33%"
  - Synchronisé avec l'action `QCM_HINT` du serveur

---

## Phase 3 - Memory interactif (v2.47.0)

### Memory : Cartes cliquables

- [ ] **Rendre la grille Memory interactive**
  - Si `gameState.QUESTION.TYPE === 'MEMORY'` → Ajouter onClick sur les cartes
  - Réutiliser le composant Memory de PlayerDisplay
  - Ajouter un wrapper pour capturer les clics

- [ ] **Envoi de l'action FLIP au clic sur carte**
  ```javascript
  const handleCardClick = (cardId) => {
    if (canFlipCard(cardId)) {
      sendWebSocketMessage({
        ACTION: 'FLIP_MEMORY_CARD',
        MSG: { CARD_ID: cardId }
      })
    }
  }
  ```

- [ ] **Pas de bouton BUZZ pour Memory**
  - Zone bouton cachée ou affiche "Cliquez sur les cartes"

---

## Phase 4 - PWA basique (v2.48.0)

### Installation comme app

- [ ] **Manifest PWA**
  ```json
  {
    "name": "BuzzMaster Joueur",
    "short_name": "BuzzMaster",
    "start_url": "/player",
    "display": "standalone",
    "orientation": "portrait",
    "theme_color": "#6366f1",
    "background_color": "#0f172a",
    "icons": [...]
  }
  ```

- [ ] **Service Worker minimal**
  - Cache des assets statiques (HTML/CSS/JS)
  - Pas de fonctionnement offline (jeu nécessite connexion)

- [ ] **Feedback haptique amélioré**
  - Vibration au buzz (50ms)
  - Double vibration si premier à buzzer (50ms, pause 50ms, 50ms)
  - Pattern différent pour bonne/mauvaise réponse (si détectable)

---

## Architecture technique simplifiée

### Composants nouveaux (minimalistes)

| Composant | Fichier | Rôle |
|-----------|---------|------|
| `PlayerPage` | `pages/PlayerPage.jsx` | Wrapper `/player` |
| `PlayerHeader` | `components/PlayerHeader.jsx` | Mini header 80px |
| `BuzzButton` | `components/BuzzButton.jsx` | Bouton BUZZ avec états |
| `PlayerConnectionModal` | `components/PlayerConnectionModal.jsx` | Modale de connexion |
| `QRCodeOverlay` | `components/QRCodeOverlay.jsx` | Overlay QR code sur /tv |

### Réutilisation maximale

- ✅ `PlayerDisplay` → Utilisé tel quel (0 modification)
- ✅ `useWebSocket` → Même hook, ajout actions `PLAYER_CONNECT`, `SHOW_QR_CODE`
- ✅ CSS existant → Réutilisé pour cohérence visuelle
- ✅ Logique de jeu → Aucune modification côté serveur (sauf flag `IS_VIRTUAL`)

### State management (React Context)

```javascript
const PlayerContext = {
  playerId: string,        // ID de session généré par serveur
  playerName: string,      // "Alice"
  teamName: string | null, // "Les Rouges" (null si non assigné)
  answerColor: string | null, // "RED" (null si non assigné)
  score: number,           // Score personnel (0 si non assigné)
  connected: boolean,      // État connexion WebSocket
  isAssigned: boolean,     // true si assigné à une équipe par l'admin
  hasBuzzed: boolean,      // A buzzé dans cette question
  reactionTime: number,    // Temps de réaction (ms)
}
```

### WebSocket Protocol - Nouveautés

| Action | Direction | Description |
|--------|-----------|-------------|
| `PLAYER_CONNECT` | Client→Server | Connexion joueur virtuel (avec nom uniquement) |
| `PLAYER_CONNECTED` | Server→Client | Confirmation (nouveau ou reconnexion) |
| `PLAYER_CONNECT_ERROR` | Server→Client | Erreur de connexion (ex: enrôlement fermé) |
| `PLAYER_ASSIGNED` | Server→Client | Notification quand l'admin assigne à une équipe |
| `PLAYER_DISCONNECT` | Client→Server | Déconnexion propre |
| `SHOW_QR_CODE` | Admin→Server→TV | Afficher QR code + activer enrôlement |
| `HIDE_QR_CODE` | Admin→Server→TV | Masquer QR code + désactiver enrôlement |

**Action PLAYER_ASSIGNED (nouvelle)** :
```json
{
  "ACTION": "PLAYER_ASSIGNED",
  "MSG": {
    "TEAM": "Les Rouges",
    "TEAM_COLOR": [255, 0, 0],
    "ANSWER_COLOR": "RED"
  }
}
```

Cette action est envoyée au VJoueur quand l'admin l'assigne à une équipe via drag & drop dans `/admin/teams`.

**Pas de nouvelles actions pour le gameplay** : Le joueur virtuel utilise `BUTTON` comme un buzzer physique.

---

## Différences `/tv` vs `/player`

| Aspect | `/tv` | `/player` |
|--------|-------|-----------|
| **Header** | Aucun (fullscreen) | Mini header 80px (nom + score) |
| **Affichage** | PlayerDisplay pur | PlayerDisplay + header + bouton |
| **Interactivité** | Lecture seule | Bouton BUZZ (+ QCM + Memory) |
| **Layout** | Horizontal 16:9 | Vertical portrait (mobile-first) |
| **Reconnexion** | Pas nécessaire | Auto-reconnexion avec localStorage |
| **QR Code** | Affichable en overlay | N/A |

---

## Cas d'usage

| Situation | Usage | Avantage |
|-----------|-------|----------|
| **Jeu sans buzzers** | Tous les joueurs sur `/player` | Pas de matériel nécessaire |
| **Grand groupe (20+)** | Mix buzzers + `/player` | Scalabilité |
| **Backup buzzer** | Si buzzer physique en panne | Continuité du jeu |
| **Spectateur** | `/tv` (lecture seule) | Pas besoin de page dédiée |

---

## Priorités de développement

**v2.41.0 - v2.45.0** : ✅ Phase 1 Complète
- `/player` avec BUZZ simple
- QR Code overlay sur `/tv` (affichage/masquage par admin)
- Zone enrollment dans TeamsPage
- Reconnexion automatique
- Auto-PONG en phase PREPARE
- Blocage buzz pour MEMORY

**v2.46.0** :
- Phase 2 : QCM interactif (4 boutons colorés)

**v2.47.0** :
- Phase 3 : Memory interactif (cartes cliquables)

**v2.48.0** :
- Phase 4 : PWA basique (manifest + service worker)

---

## Maquettes (à créer)

### 1. QR Code sur /tv (Overlay coin)

```
┌────────────────────────────────────┐
│  QUESTION EN COURS                 │
│  [Timer, média, réponses...]       │
│                                    │
│                       ┌──────────┐ │
│                       │ ▓▓▓▓▓▓▓▓ │ │
│                       │ ▓▓░░░░▓▓ │ │
│                       │ ▓▓░░░░▓▓ │ │
│                       │ ▓▓▓▓▓▓▓▓ │ │
│                       └──────────┘ │
│                       Scannez !    │
└────────────────────────────────────┘
```

### 2. Modale de connexion VJoueur

```
┌────────────────────────────────┐
│  📱 Rejoindre le jeu           │
│                                │
│  Entrez votre nom/pseudo :     │
│                                │
│  [____________]                │
│  ⚠️ 2-20 caractères            │
│                                │
│  L'animateur vous assignera    │
│  à une équipe.                 │
│                                │
│    [ Rejoindre ] (désactivé)   │
└────────────────────────────────┘

États du bouton "Rejoindre" :
- Désactivé (gris) : Si nom invalide (< 2 ou > 20 caractères)
- Actif (vert) : Si nom valide
- Chargement : Pendant la connexion au serveur

Note : Modale non-fermable (pas de croix ×)
```

### 3. `/player` - Question normale

```
┌────────────────────────────────┐
│ 🔴 Alice • Les Rouges • 25pts │  Header
├────────────────────────────────┤
│                                │
│   [Question affichée ici]      │  PlayerDisplay
│   [Timer, média, etc.]         │  (réutilisé)
│                                │
├────────────────────────────────┤
│    ┌──────────────────────┐   │
│    │      BUZZ !          │   │  Bouton BUZZ
│    └──────────────────────┘   │
└────────────────────────────────┘
```

### 4. `/player` - QCM

```
┌────────────────────────────────┐
│ 🔴 Alice • Les Rouges • 25pts │
├────────────────────────────────┤
│   [Question QCM ici]           │
├────────────────────────────────┤
│ [A] Paris    [B] Londres       │  4 boutons
│ [C] Berlin   [D] Madrid        │  colorés
└────────────────────────────────┘
```

---

## Décisions de conception

### ✅ Validées

- **Pas de page d'accueil** : QR code sur /tv suffit
- **Pas de mode spectateur** : Utiliser `/tv` directement
- **Breaking change routes** : `/` → `/admin` sans compatibilité
- **QR Code overlay** : Affichage à la demande par l'admin
- **Réutilisation maximale** : PlayerDisplay inchangé
- **Identification obligatoire** : Tout VJoueur doit entrer un nom/pseudo (2-20 caractères)
  - Modale non-fermable jusqu'à connexion valide
  - Validation en temps réel côté client
  - Validation et unicité optionnelle côté serveur
- **Attribution par l'admin** : Le VJoueur ne choisit PAS son équipe ni sa couleur
  - Connexion = juste le nom
  - Apparaît comme un buzzer non assigné dans `/admin/teams`
  - L'admin fait l'attribution via drag & drop (workflow existant)
  - Identique à un buzzer physique qui se connecte
- **Phase d'enrôlement contrôlée** : Les nouveaux VJoueurs ne peuvent se connecter que pendant l'affichage du QR code
  - QR code affiché = Enrôlement OUVERT (nouveaux + reconnexions)
  - QR code masqué = Enrôlement FERMÉ (reconnexions uniquement)
  - Variable serveur : `enrollmentActive` (booléen)
  - Les VJoueurs connus peuvent toujours se reconnecter (même hors enrôlement)
  - Reconnexion = restauration de l'état complet (équipe, couleur, score)

### ❓ Questions ouvertes

- [ ] **Position QR code** : Coin (moins intrusif) ou plein écran (phase STOPPED uniquement) ?
  - **Proposition** : Coin par défaut, option plein écran ajoutée plus tard

- [ ] **Persistance connexion (localStorage)** : Combien de temps garder le localStorage côté client ?
  - **Proposition** : 30 minutes, puis demander reconnexion

- [ ] **Mémoire serveur des VJoueurs** : Combien de temps garder un VJoueur en mémoire après déconnexion ?
  - **Option 1 - Durée de session** : Jusqu'à la fin de la partie (action RAZ scores)
  - **Option 2 - Timeout** : 30 minutes après déconnexion, puis suppression
  - **Option 3 - Persistance** : Toujours en mémoire, suppression manuelle par l'admin
  - **Proposition** : Option 1 (durée de session) - supprimé uniquement au RAZ scores

- [ ] **État visuel VJoueur déconnecté** : Comment afficher un VJoueur déconnecté dans `/admin/teams` ?
  - Badge "🔌 DÉCONNECTÉ" + grisé
  - Reste visible et déplaçable (l'admin peut toujours le gérer)
  - Reprend sa couleur normale à la reconnexion

- [ ] **Limite joueurs virtuels** : Y a-t-il une limite technique ?
  - **Proposition** : Pas de limite hard, mais recommander < 50 pour performance

---

## Métriques de succès

| Métrique | Cible |
|----------|-------|
| **Temps de chargement `/player`** | < 2s (3G) |
| **Latence buzz** | < 100ms (feedback optimiste) |
| **Taux d'adoption** | 30% des joueurs utilisent `/player` |
| **Réutilisation code** | > 80% du code vient de `/tv` existant |
| **Scan QR → Jouer** | < 15 secondes |
