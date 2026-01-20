# Page Joueur (/player)

**Statut** : 📋 Planifié

## Concept

**Surcouche légère sur `/tv`** permettant aux joueurs de buzzer depuis leur smartphone. L'affichage suit automatiquement le rythme du jeu géré par l'animateur, sans contrôle supplémentaire pour le joueur.

### Terminologie

- **Joueur Physique** : Joueur avec un buzzer ESP32 (BuzzClick)
- **VJoueur** (Joueur Virtuel) : Joueur connecté depuis un navigateur web via `/player`
  - Utilise son smartphone/tablette comme buzzer
  - **Doit obligatoirement entrer un nom/pseudo** lors de la connexion
  - Apparaît comme un bumper virtuel dans `/admin/teams`
  - Fonctionne exactement comme un buzzer physique pour le gameplay

### Principe de conception

> `/player` = `/tv` (affichage synchronisé) + Mini header personnalisé + Bouton BUZZ

- **Réutilisation maximale** : Même composant `PlayerDisplay` que `/tv`
- **Pas de navigation** : Le joueur ne change jamais de page, l'affichage s'adapte automatiquement
- **Contrôle minimal** : Uniquement buzzer, pas de stats détaillées ni de dashboard complexe
- **Gestion par l'animateur** : Tout est piloté depuis `/admin`
- **Accès via QR Code** : Les joueurs scannent un QR code affiché sur `/tv` pour rejoindre facilement
- **Identification obligatoire** : Chaque VJoueur doit entrer un nom/pseudo unique

---

## Organisation des routes (simplifiée)

### Routes actuelles vs nouvelles

| Route actuelle | Usage actuel | Nouvelle route | Notes |
|----------------|--------------|----------------|-------|
| `/` | Admin (GamePage) | `/admin` | Breaking change OK, pas de compatibilité |
| `/tv` | Affichage TV | `/tv` | Inchangé + QR code en overlay |
| `/quiz` | Questions admin | `/admin/questions` | Sous /admin |
| `/teams` | Équipes admin | `/admin/teams` | Sous /admin |
| `/settings` | Config admin | `/admin/settings` | Sous /admin |
| `/history-page` | Historique admin | `/admin/history` | Sous /admin |
| `/palmares` | Palmarès admin | `/admin/palmares` | Sous /admin |
| `/scoreboard` | Scores admin | `/admin/scores` | Sous /admin |
| - | - | **`/player`** | Nouveau : Interface joueur |

### Structure finale

```
/admin                      # Interface admin (breaking change: anciennement /)
├── /admin/questions        # Gestion questions
├── /admin/teams            # Gestion équipes
├── /admin/settings         # Configuration
├── /admin/history          # Historique
├── /admin/palmares         # Palmarès
└── /admin/scores           # Scores

/tv                         # Affichage TV + QR code (overlay à la demande)

/player                     # Interface joueur (accès via QR code)
```

**Pas de page d'accueil `/`** : Redirection directe vers `/admin`

---

## QR Code sur /tv (Phase d'enrôlement)

### Concept

L'animateur affiche un QR code sur l'écran TV pour **ouvrir la phase d'enrôlement** : les joueurs scannent le code pour accéder à `/player` et se connecter en tant que VJoueurs.

**Distinction importante** :
- **QR code AFFICHÉ** = Phase d'enrôlement ACTIVE → Nouveaux VJoueurs acceptés
- **QR code MASQUÉ** = Phase d'enrôlement FERMÉE → Seules les reconnexions sont acceptées

### Avantages

✅ **Simplicité** : Pas besoin de taper l'URL
✅ **Sécurité** : Les joueurs ne peuvent pas "tomber" sur `/admin` par erreur
✅ **Contrôle de l'enrôlement** : L'animateur décide quand accepter de nouveaux VJoueurs
✅ **Reconnexion toujours possible** : Les VJoueurs déconnectés peuvent revenir même sans QR code
✅ **UX fluide** : Scan → Connexion → Jouer

### Implémentation

- [ ] **Bouton dans l'interface admin**
  - Ajout d'un bouton "📱 Afficher QR Code" dans `/admin` (GamePage)
  - Ou dans un menu déroulant "Joueurs virtuels"
  - Toggle : afficher/masquer le QR code sur `/tv`

- [ ] **Action WebSocket et gestion de l'enrôlement**
  - Action `SHOW_QR_CODE` : Active l'enrôlement (`enrollmentActive = true`)
  - Action `HIDE_QR_CODE` : Désactive l'enrôlement (`enrollmentActive = false`)
  - Payload : `{URL: "http://192.168.4.1/player"}`
  - Broadcast à tous les clients `/tv`
  - **Impact côté serveur** :
    - `SHOW_QR_CODE` → accepter nouveaux VJoueurs + reconnexions
    - `HIDE_QR_CODE` → accepter uniquement les reconnexions

- [ ] **Affichage sur /tv**
  - **Option 1 - Overlay coin** :
    - QR code 200x200px dans le coin inférieur droit
    - Fond semi-transparent
    - Texte : "Scannez pour jouer !"
    - N'obstrue pas le contenu principal
  - **Option 2 - Plein écran** (phase STOPPED uniquement) :
    - QR code 400x400px centré
    - Grand texte : "Rejoignez le jeu !"
    - Instructions : "Scannez ce code avec votre smartphone"
    - Visible uniquement quand aucune question n'est active

- [ ] **Génération du QR code**
  - Bibliothèque : `qrcode` (npm) côté frontend
  - URL dynamique : `http://${serverIP}/player`
  - Niveau de correction d'erreur : M (15%)

```javascript
// Exemple React
import QRCode from 'qrcode'

const [qrCodeUrl, setQrCodeUrl] = useState('')

useEffect(() => {
  if (showQrCode) {
    QRCode.toDataURL(`http://${serverIP}/player`)
      .then(url => setQrCodeUrl(url))
  }
}, [showQrCode, serverIP])

return showQrCode && (
  <div className="qr-code-overlay">
    <img src={qrCodeUrl} alt="QR Code" />
    <p>Scannez pour jouer !</p>
  </div>
)
```

### Maquette QR Code (Overlay coin)

```
┌────────────────────────────────────┐
│  [Question affichée ici]           │
│  [Timer, média, réponses QCM...]   │
│                                    │
│                       ┌──────────┐ │
│                       │ ░░░░░░░░ │ │
│                       │ ░░▓▓▓▓░░ │ │ QR Code
│                       │ ░░▓▓▓▓░░ │ │ 200x200px
│                       │ ░░░░░░░░ │ │
│                       └──────────┘ │
│                       Scannez !    │
└────────────────────────────────────┘
```

---

## Phase 1 - MVP (v2.40.0)

### Page `/player` - Structure

```
┌─────────────────────────────┐
│ Mini Header Joueur (80px)   │ ← Nouveau : nom, équipe, score
├─────────────────────────────┤
│                             │
│   PlayerDisplay (réutilisé) │ ← Identique à /tv
│   - Question                │
│   - Timer                   │
│   - Média                   │
│   - Réponses QCM / Memory   │
│                             │
├─────────────────────────────┤
│ Zone Bouton (120px)         │ ← Nouveau : bouton BUZZ
└─────────────────────────────┘
```

### 1. Connexion VJoueur (modale initiale obligatoire)

- [ ] **Modale de connexion au chargement de `/player`**
  - **Champ "Nom/Pseudo"** (UNIQUE CHAMP)
    - Minimum 2 caractères, maximum 20 caractères
    - Validation en temps réel
    - Message d'erreur si vide ou invalide
    - Bouton "Rejoindre" (désactivé tant que nom invalide)
  - Persistance dans localStorage (reconnexion auto si < 30 min)
  - La modale ne peut pas être fermée sans connexion valide
  - **Pas de sélection d'équipe ou de couleur** : Géré par l'admin après connexion
  - **Gestion erreur enrôlement fermé** :
    - Si `PLAYER_CONNECT_ERROR` avec `ENROLLMENT_CLOSED`
    - Afficher message : "❌ L'enrôlement est fermé. Contactez l'animateur pour qu'il affiche le QR code."
    - Le bouton "Rejoindre" reste actif pour retenter (cas reconnexion)

- [ ] **Validation côté serveur**
  - Vérifier que le nom n'est pas vide (après trim)
  - Vérifier longueur (2-20 caractères)
  - Optionnel : Vérifier unicité du nom global
    - Si doublon : ajouter un suffixe (ex: "Alice (2)")
    - Ou refuser la connexion avec message d'erreur

- [ ] **Enregistrement côté serveur : Distinction Enrôlement vs Reconnexion**

  **Phase d'enrôlement** (QR code affiché) :
  - Variable serveur : `enrollmentActive` (booléen)
  - Activé quand l'admin affiche le QR code (`SHOW_QR_CODE`)
  - Désactivé quand l'admin masque le QR code (`HIDE_QR_CODE`)
  - Pendant l'enrôlement : accepter **nouveaux VJoueurs** ET **reconnexions**

  **Hors enrôlement** (QR code masqué) :
  - Refuser les nouveaux VJoueurs (erreur : "Enrôlement fermé, contactez l'animateur")
  - Accepter uniquement les **reconnexions** de VJoueurs connus
  - Le serveur garde en mémoire les VJoueurs déjà enregistrés (même déconnectés)

  **Logique serveur lors de `PLAYER_CONNECT`** :
  ```go
  if !isKnownPlayer(name) {
    // Nouveau joueur
    if !enrollmentActive {
      return error("Enrôlement fermé")
    }
    // Créer nouveau bumper virtuel
    createVirtualBumper(name)
  } else {
    // Reconnexion d'un joueur connu
    // Toujours autorisée (même hors enrôlement)
    reconnectVirtualBumper(name)
  }
  ```

  **Création d'un nouveau VJoueur (première connexion)** :
  - Action WebSocket `PLAYER_CONNECT` avec `NAME`
  - Création d'un bumper virtuel avec flag `IS_VIRTUAL: true`
  - État initial : **NON ASSIGNÉ** (pas d'équipe, pas de couleur QCM)
  - Réponse serveur : `PLAYER_CONNECTED` avec ID de session et nom validé
  - Le VJoueur apparaît dans `/admin/teams` comme un **buzzer standard non assigné**
  - Badge visuel "📱 VIRTUEL" pour distinguer des buzzers physiques

  **Reconnexion d'un VJoueur existant** :
  - Le VJoueur envoie le même `NAME` que lors de sa première connexion
  - Le serveur retrouve le bumper virtuel correspondant
  - Restauration de l'état : équipe, couleur QCM, score (si déjà assigné)
  - Réponse serveur : `PLAYER_CONNECTED` avec état complet restauré
  - Le VJoueur retrouve son interface comme avant la déconnexion

- [ ] **Attribution par l'admin**
  - Le VJoueur apparaît dans la liste des joueurs non assignés (comme un buzzer physique)
  - L'admin peut glisser-déposer le VJoueur vers une équipe (drag & drop existant)
  - L'admin peut attribuer une couleur QCM (interface existante)
  - Identique au workflow d'un buzzer physique qui se connecte

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

### 3. Zone PlayerDisplay (réutilisée à 100%)

- [ ] **Import du composant existant**
  ```jsx
  import PlayerDisplay from './pages/PlayerDisplay'

  function PlayerPage() {
    return (
      <>
        <PlayerHeader />
        <PlayerDisplay /> {/* Réutilisé tel quel */}
        <BuzzButtonZone />
      </>
    )
  }
  ```

- [ ] **Comportement identique à `/tv`**
  - Affichage de la question en cours
  - Timer synchronisé
  - Média (image/vidéo)
  - Réponses QCM affichées (mais pas cliquables ici)
  - Grille Memory affichée (mais pas cliquable ici)
  - Changement de vue selon `gameState.PAGE` (GAME/SCORE/PLAYERS/PALMARES)

### 4. Zone Bouton BUZZ (120px fixe, en bas)

- [ ] **Bouton principal de buzz**
  - Taille : 100% largeur, 80px hauteur
  - Couleur : Couleur de l'équipe du joueur
  - Texte : "BUZZ !" (grande police, bold)
  - Position : Fixe en bas de l'écran (sticky)

- [ ] **États du bouton**

  | État | Apparence | Comportement |
  |------|-----------|--------------|
  | **NON ASSIGNÉ** | Gris, "Pas encore assigné" | Non cliquable |
  | **STOPPED** | Gris, désactivé, "En attente..." | Non cliquable |
  | **PREPARE** | Gris, désactivé, "Préparez-vous..." | Non cliquable |
  | **READY** | Couleur équipe, "PRÊT !" | Non cliquable (attente démarrage) |
  | **STARTED** | Couleur équipe pulsante, "BUZZ !" | ✅ Cliquable (si assigné) |
  | **PAUSED (autre joueur)** | Gris, "Un joueur a buzzé" | Non cliquable |
  | **PAUSED (vous)** | Vert, "Vous avez buzzé !" + temps | Non cliquable |
  | **REVEALED** | Gris, désactivé | Non cliquable |

  **Important** : Le bouton reste désactivé tant que le VJoueur n'a pas été assigné à une équipe par l'admin.

- [ ] **Envoi de l'action au clic**
  ```javascript
  const handleBuzz = () => {
    if (gameState.PHASE !== 'STARTED') return

    const now = Date.now()
    sendWebSocketMessage({
      ACTION: 'BUTTON',
      ID: playerId,
      MSG: { button: answerColor || 'A', timestamp: now }
    })

    // Feedback optimiste
    setLocalBuzzed(true)
    navigator.vibrate && navigator.vibrate(50)
  }
  ```

- [ ] **Feedback immédiat**
  - Vibration haptique (50ms) sur mobile
  - Changement de couleur instantané (vert)
  - Affichage du temps de réaction si disponible

---

## Phase 2 - QCM interactif (v2.41.0)

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

## Phase 3 - Memory interactif (v2.42.0)

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

## Phase 4 - PWA basique (v2.43.0)

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

**v2.40.0 - MVP** :
- Phase 1 : `/player` avec BUZZ simple
- QR Code sur `/tv` (affichage/masquage par admin)
- Réorganisation routes (breaking change : `/` → `/admin`)

**v2.41.0** :
- Phase 2 : QCM interactif (4 boutons)

**v2.42.0** :
- Phase 3 : Memory interactif (cartes cliquables)

**v2.43.0** :
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
