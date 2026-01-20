# Page Joueur (/player)

**Statut** : 📋 Planifié

## Concept

**Surcouche légère sur `/tv`** permettant aux joueurs de buzzer depuis leur smartphone. L'affichage suit automatiquement le rythme du jeu géré par l'animateur, sans contrôle supplémentaire pour le joueur.

### Principe de conception

> `/player` = `/tv` (affichage synchronisé) + Mini header personnalisé + Bouton BUZZ

- **Réutilisation maximale** : Même composant `PlayerDisplay` que `/tv`
- **Pas de navigation** : Le joueur ne change jamais de page, l'affichage s'adapte automatiquement
- **Contrôle minimal** : Uniquement buzzer, pas de stats détaillées ni de dashboard complexe
- **Gestion par l'animateur** : Tout est piloté depuis `/admin`
- **Accès via QR Code** : Les joueurs scannent un QR code affiché sur `/tv` pour rejoindre facilement

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

## QR Code sur /tv (Nouvelle fonctionnalité clé)

### Concept

L'animateur affiche un QR code sur l'écran TV que les joueurs scannent pour accéder directement à `/player`.

### Avantages

✅ **Simplicité** : Pas besoin de taper l'URL
✅ **Sécurité** : Les joueurs ne peuvent pas "tomber" sur `/admin` par erreur
✅ **Contrôle** : L'animateur décide quand afficher/masquer le QR code
✅ **UX fluide** : Scan → Connexion → Jouer

### Implémentation

- [ ] **Bouton dans l'interface admin**
  - Ajout d'un bouton "📱 Afficher QR Code" dans `/admin` (GamePage)
  - Ou dans un menu déroulant "Joueurs virtuels"
  - Toggle : afficher/masquer le QR code sur `/tv`

- [ ] **Action WebSocket**
  - Action `SHOW_QR_CODE` / `HIDE_QR_CODE`
  - Payload : `{URL: "http://192.168.4.1/player"}`
  - Broadcast à tous les clients `/tv`

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

### 1. Connexion joueur (modale initiale)

- [ ] **Modale de connexion au chargement de `/player`**
  - Champ "Nom du joueur"
  - Sélection de l'équipe (liste déroulante)
  - Sélection couleur QCM (optionnel) : Rouge/Vert/Jaune/Bleu
  - Bouton "Rejoindre"
  - Persistance dans localStorage (reconnexion auto si < 30 min)

- [ ] **Enregistrement côté serveur**
  - Action WebSocket `PLAYER_CONNECT`
  - Création d'un bumper virtuel avec flag `IS_VIRTUAL: true`
  - Réponse serveur : `PLAYER_CONNECTED` avec ID de session
  - Le joueur virtuel apparaît dans `/admin/teams` comme un joueur normal

**Payload PLAYER_CONNECT :**
```json
{
  "ACTION": "PLAYER_CONNECT",
  "MSG": {
    "NAME": "Alice",
    "TEAM": "Les Rouges",
    "ANSWER_COLOR": "RED"
  }
}
```

### 2. Mini header personnalisé (80px fixe)

- [ ] **Affichage compact**
  - Avatar circulaire (40px) avec couleur de l'équipe
  - Nom du joueur (tronqué si trop long)
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
  | **STOPPED** | Gris, désactivé, "En attente..." | Non cliquable |
  | **PREPARE** | Gris, désactivé, "Préparez-vous..." | Non cliquable |
  | **READY** | Couleur équipe, "PRÊT !" | Non cliquable (attente démarrage) |
  | **STARTED** | Couleur équipe pulsante, "BUZZ !" | ✅ Cliquable |
  | **PAUSED (autre joueur)** | Gris, "Un joueur a buzzé" | Non cliquable |
  | **PAUSED (vous)** | Vert, "Vous avez buzzé !" + temps | Non cliquable |
  | **REVEALED** | Gris, désactivé | Non cliquable |

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
  teamName: string,        // "Les Rouges"
  answerColor: string,     // "RED" (pour QCM)
  score: number,           // Score personnel
  connected: boolean,      // État connexion WebSocket
  hasBuzzed: boolean,      // A buzzé dans cette question
  reactionTime: number,    // Temps de réaction (ms)
}
```

### WebSocket Protocol - Nouveautés

| Action | Direction | Description |
|--------|-----------|-------------|
| `PLAYER_CONNECT` | Client→Server | Connexion joueur virtuel |
| `PLAYER_CONNECTED` | Server→Client | Confirmation avec session ID |
| `PLAYER_DISCONNECT` | Client→Server | Déconnexion propre |
| `SHOW_QR_CODE` | Admin→Server→TV | Afficher QR code sur /tv |
| `HIDE_QR_CODE` | Admin→Server→TV | Masquer QR code sur /tv |

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

### 2. Modale de connexion joueur

```
┌────────────────────────────────┐
│  Rejoindre le jeu              │
│                                │
│  Nom : [____________]          │
│                                │
│  Équipe : [▼ Les Rouges   ]   │
│                                │
│  Couleur QCM (optionnel) :     │
│  ○ Rouge  ○ Vert               │
│  ○ Jaune  ○ Bleu               │
│                                │
│        [ Rejoindre ]           │
└────────────────────────────────┘
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

### ❓ Questions ouvertes

- [ ] **Position QR code** : Coin (moins intrusif) ou plein écran (phase STOPPED uniquement) ?
  - **Proposition** : Coin par défaut, option plein écran ajoutée plus tard

- [ ] **Persistance connexion** : Combien de temps garder le localStorage ?
  - **Proposition** : 30 minutes, puis demander reconnexion

- [ ] **Déconnexion serveur** : Combien de temps garder le joueur virtuel ?
  - **Proposition** : 5 minutes, puis marquer comme "absent" (grisé dans `/admin/teams`)

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
