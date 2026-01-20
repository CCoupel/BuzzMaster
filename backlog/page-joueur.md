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

---

## Organisation des routes (révision)

### Routes actuelles vs proposées

| Route actuelle | Usage actuel | Proposition | Raison |
|----------------|--------------|-------------|--------|
| `/` | Admin (GamePage) | → `/admin` | Clarifier le rôle |
| `/tv` | Affichage TV | Inchangé | OK |
| `/quiz` | Questions admin | → `/admin/questions` | Regrouper admin |
| `/teams` | Équipes admin | → `/admin/teams` | Regrouper admin |
| `/settings` | Config admin | → `/admin/settings` | Regrouper admin |
| `/history-page` | Historique admin | → `/admin/history` | Regrouper admin |
| `/palmares` | Palmarès admin | → `/admin/palmares` | Regrouper admin |
| `/scoreboard` | Scores admin | → `/admin/scores` | Regrouper admin |
| - | - | **`/player` (nouveau)** | Interface joueur |
| `/` | - | **Page d'accueil** | Choix admin/tv/player |

### Nouvelle structure proposée

```
/                           # Page d'accueil : 3 gros boutons
├── /admin                  # Interface admin (anciennement /)
│   ├── /admin/questions    # Gestion questions
│   ├── /admin/teams        # Gestion équipes
│   ├── /admin/settings     # Configuration
│   ├── /admin/history      # Historique
│   ├── /admin/palmares     # Palmarès
│   └── /admin/scores       # Scores
├── /tv                     # Affichage TV (inchangé)
└── /player                 # Interface joueur (nouveau)
```

### Page d'accueil `/`

```
┌─────────────────────────────────────┐
│        🎮 BuzzMaster                │
│                                     │
│   ┌───────────────────────────┐    │
│   │   👤 JOUEUR               │    │
│   │   Jouer depuis mon        │    │
│   │   téléphone               │    │
│   └───────────────────────────┘    │
│                                     │
│   ┌───────────────────────────┐    │
│   │   📺 TV                   │    │
│   │   Affichage grand écran   │    │
│   └───────────────────────────┘    │
│                                     │
│   ┌───────────────────────────┐    │
│   │   ⚙️ ADMIN                │    │
│   │   Gérer le jeu            │    │
│   └───────────────────────────┘    │
│                                     │
│   Version: 2.40.0                  │
└─────────────────────────────────────┘
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
  - Persistance dans localStorage (reconnexion auto)

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
| `HomePage` | `pages/HomePage.jsx` | Page d'accueil avec 3 boutons |
| `PlayerPage` | `pages/PlayerPage.jsx` | Wrapper `/player` |
| `PlayerHeader` | `components/PlayerHeader.jsx` | Mini header 80px |
| `BuzzButton` | `components/BuzzButton.jsx` | Bouton BUZZ avec états |
| `PlayerConnectionModal` | `components/PlayerConnectionModal.jsx` | Modale de connexion |

### Réutilisation maximale

- ✅ `PlayerDisplay` → Utilisé tel quel (0 modification)
- ✅ `useWebSocket` → Même hook, ajout action `PLAYER_CONNECT`
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

---

## Cas d'usage

| Situation | Usage | Avantage |
|-----------|-------|----------|
| **Jeu sans buzzers** | Tous les joueurs sur `/player` | Pas de matériel nécessaire |
| **Grand groupe (20+)** | Mix buzzers + `/player` | Scalabilité |
| **Spectateur actif** | `/player` en lecture seule | Suivre depuis son téléphone |
| **Backup buzzer** | Si buzzer physique en panne | Continuité du jeu |

---

## Priorités de développement

**v2.40.0 - MVP** :
- Phase 1 : Page d'accueil + `/player` avec BUZZ simple
- Réorganisation routes (optionnel, peut attendre)

**v2.41.0** :
- Phase 2 : QCM interactif (4 boutons)

**v2.42.0** :
- Phase 3 : Memory interactif (cartes cliquables)

**v2.43.0** :
- Phase 4 : PWA basique (manifest + service worker)

---

## Maquettes (à créer)

### 1. Page d'accueil `/`
- 3 gros boutons (JOUEUR / TV / ADMIN)
- Responsive (mobile + desktop)

### 2. Modale de connexion joueur
- Champ nom
- Sélection équipe
- Sélection couleur QCM

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

## Questions ouvertes

- [ ] **Réorganisation routes** : Faut-il vraiment déplacer `/` vers `/admin` ?
  - **Option 1** : Oui, clarté maximale (/ = accueil, /admin = gestion, /tv = TV, /player = joueur)
  - **Option 2** : Non, garder `/` comme admin pour compatibilité (anciens favoris)
  - **Proposition** : Option 1, avec redirection `/` → `/admin` pendant 1 version de transition

- [ ] **Statistiques joueur** : Faut-il afficher plus que le score dans le header ?
  - **Proposition** : Non, garder minimaliste. Si besoin, ajouter une page `/player/stats` plus tard

- [ ] **Mode spectateur** : Autoriser `/player` sans buzzer (lecture seule) ?
  - **Proposition** : Oui, si pas d'équipe sélectionnée → mode spectateur automatique

- [ ] **Déconnexion** : Combien de temps garder le joueur virtuel après déconnexion ?
  - **Proposition** : 5 minutes, puis marquer comme "absent" (grisé dans `/admin/teams`)

---

## Métriques de succès

| Métrique | Cible |
|----------|-------|
| **Temps de chargement `/player`** | < 2s (3G) |
| **Latence buzz** | < 100ms (feedback optimiste) |
| **Taux d'adoption** | 30% des joueurs utilisent `/player` |
| **Réutilisation code** | > 80% du code vient de `/tv` existant |
