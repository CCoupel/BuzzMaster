# Plan d'implémentation - Effet Néon Catégorie (v2.46.0)

**Décision d'architecture** : Option B - Rotation CSS conic-gradient avec configuration

## Vue d'ensemble

Ajouter un effet néon lumineux autour de l'écran sur `/tv` (PlayerDisplay) et `/player` (VPlayerPage) utilisant la couleur de la catégorie de la question en cours.

## Stratégie : Séquentiel (Backend → Frontend)

Le frontend dépend de l'endpoint GET `/config` pour lire les paramètres néon.

```
DEV-BACKEND (1h)
    ↓ Crée config.json + endpoints GET/POST
    ↓
DEV-FRONTEND (3h)
    ├─ PlayerDisplay.jsx + neon.css (TV)
    └─ VPlayerPage.jsx + ConfigPage.jsx (Admin)
```

## Phase 1 : Backend (config + API)

### 1.1 Struct Config (Go)

**Fichier** : `server-go/internal/config/config.go`

Ajouter dans `Config` struct :

```go
type NeonEffectConfig struct {
    Enabled            bool    `json:"enabled"`              // Activer/désactiver l'effet
    ArcWidth           int     `json:"arc_width"`            // Largeur arc en degrés (30-180, défaut: 60)
    IntensityGap       int     `json:"intensity_gap"`        // Écart intensité (0-100, défaut: 80)
    RotationSpeed      float64 `json:"rotation_speed"`       // Vitesse rotation en secondes (1-10, défaut: 4)
}

// Dans Config struct
NeonEffect NeonEffectConfig `json:"neon_effect"`
```

### 1.2 Initialisation Config

**Fichier** : `server-go/internal/config/config.go`

```go
// Valeurs par défaut
NeonEffect: NeonEffectConfig{
    Enabled:       true,
    ArcWidth:      60,
    IntensityGap:  80,
    RotationSpeed: 4.0,
}
```

### 1.3 Endpoints HTTP

**Fichier** : `server-go/internal/server/http.go`

Modifier le handler `configHandler` (existant) pour :

1. **GET /config.json** : Retourner `config.NeonEffect`
   ```json
   {
     "server": { ... },
     "wifi": { ... },
     "game": { ... },
     "storage": { ... },
     "neon_effect": {
       "enabled": true,
       "arc_width": 60,
       "intensity_gap": 80,
       "rotation_speed": 4.0
     }
   }
   ```

2. **POST /config.json** : Accepter mises à jour `neon_effect`
   - Validation : `arc_width` ∈ [30, 180]
   - Validation : `intensity_gap` ∈ [0, 100]
   - Validation : `rotation_speed` ∈ [1.0, 10.0]
   - Sauvegarde au fichier `config.json`

### 1.4 WebSocket Broadcast

**Fichier** : `server-go/cmd/server/main.go`

Ajouter action `CONFIG_UPDATE` :
- Quand la config néon est modifiée via POST → broadcast à tous les clients
- Payload : `{ ACTION: "CONFIG_UPDATE", MSG: { neonEffect: {...} } }`

### 1.5 GameState

**Fichier** : `server-go/internal/game/models.go`

Ajouter champ dans `GameState` :
```go
NeonEffect NeonEffectConfig `json:"neon_effect"`
```

- Initialisé depuis `config.NeonEffect` au démarrage
- Synchronisé lors du broadcast `UPDATE`

## Phase 2 : Frontend (UI + Styles)

### 2.1 Fichier CSS Partagé

**Nouveau fichier** : `server-go/web/src/styles/neon.css`

```css
/* Variables CSS par défaut */
:root {
  --neon-color: #3b82f6;              /* Couleur catégorie (injectée dynamiquement) */
  --neon-arc-width: 60deg;             /* Largeur de l'arc (30-180°) */
  --neon-intensity-gap: 0.8;           /* Écart intensité (0-1) */
  --neon-rotation-speed: 4s;           /* Vitesse rotation (1-10s) */
}

/* Propriété CSS custom pour animer --rotation-angle */
@property --rotation-angle {
  syntax: '<angle>';
  initial-value: 0deg;
  inherits: false;
}

/* Classe neon-border appliquée aux pages */
.neon-border {
  --rotation-angle: 0deg;
  animation: neon-rotate var(--neon-rotation-speed) linear infinite;
}

/* Pseudo-élément créant l'effet visuel */
.neon-border::before {
  content: '';
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  pointer-events: none;
  z-index: 9998;

  /* Gradient conique avec zone intense rotative */
  background: conic-gradient(
    from var(--rotation-angle),
    transparent 0deg,
    var(--neon-color) calc(var(--neon-arc-width) / 2),
    transparent var(--neon-arc-width),
    transparent 360deg
  );

  /* Flou pour effet néon */
  filter: blur(20px);

  /* Inset pour bordure intérieure */
  box-shadow: inset 0 0 40px var(--neon-color);
}

/* Animation rotation */
@keyframes neon-rotate {
  from {
    --rotation-angle: 0deg;
  }
  to {
    --rotation-angle: 360deg;
  }
}

/* Transitions animées */
.neon-border-fadeout {
  animation: neon-fadeout 0.5s ease-out forwards;
}

@keyframes neon-fadeout {
  from {
    opacity: 1;
  }
  to {
    opacity: 0;
  }
}

.neon-border-fadein {
  animation: neon-fadein 0.5s ease-in forwards;
}

@keyframes neon-fadein {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}
```

### 2.2 PlayerDisplay.jsx (TV)

**Fichier** : `server-go/web/src/pages/PlayerDisplay.jsx`

```jsx
// Import CSS neon
import '../styles/neon.css'

// Dans le composant
export default function PlayerDisplay() {
  const { gameState } = useGame()
  const [neonConfig, setNeonConfig] = useState(null)

  // Récupérer config néon une seule fois
  useEffect(() => {
    fetch('/config.json')
      .then(r => r.json())
      .then(data => setNeonConfig(data.neon_effect))
      .catch(console.error)
  }, [])

  // Écouter changements config via WebSocket
  useWebSocket((msg) => {
    if (msg.ACTION === 'CONFIG_UPDATE') {
      setNeonConfig(msg.MSG.neonEffect)
    }
  })

  // Déterminer si neon doit être actif et quelle couleur
  const getCategoryColor = () => {
    if (!gameState?.question?.CATEGORY) return null
    const colors = {
      GEOGRAPHY: '#3b82f6',
      ENTERTAINMENT: '#ec4899',
      HISTORY: '#eab308',
      ARTS: '#a855f7',
      SCIENCE: '#22c55e',
      SPORTS: '#f97316',
      FOOD: '#991b1b',
      ANIMALS: '#78716c',
    }
    return colors[gameState.question.CATEGORY] || null
  }

  const shouldShowNeon = () => {
    if (!neonConfig?.enabled) return false
    const phase = gameState?.PHASE
    return phase === 'STARTED' || phase === 'PAUSED' || phase === 'READY'
  }

  const categoryColor = getCategoryColor()

  // Appliquer classes et styles
  const pageClass = shouldShowNeon() ? 'neon-border' : ''

  return (
    <div
      className={`player-display ${pageClass}`}
      style={shouldShowNeon() && categoryColor ? {
        '--neon-color': categoryColor,
        '--neon-arc-width': `${neonConfig?.arc_width || 60}deg`,
        '--neon-intensity-gap': `${1 - (neonConfig?.intensity_gap || 80) / 100}`,
        '--neon-rotation-speed': `${neonConfig?.rotation_speed || 4}s`,
      } : {}}
    >
      {/* Contenu existant */}
    </div>
  )
}
```

### 2.3 VPlayerPage.jsx (Mobile)

**Fichier** : `server-go/web/src/pages/VPlayerPage.jsx`

Même logique que PlayerDisplay :
- Import `neon.css`
- Récupère config néon
- Applique classe `.neon-border` si conditions remplies
- Variables CSS pour couleur, arc, intensité, vitesse

### 2.4 ConfigPage.jsx (Admin)

**Fichier** : `server-go/web/src/pages/ConfigPage.jsx`

Ajouter section **"Effet Néon"** avec :

```jsx
{/* Neon Effect Configuration Section */}
<Card>
  <CardHeader>Effet Néon</CardHeader>
  <CardBody>

    {/* Toggle */}
    <div className="form-group">
      <label>
        <input
          type="checkbox"
          checked={neonConfig?.enabled || false}
          onChange={(e) => handleNeonConfigChange('enabled', e.target.checked)}
        />
        Activer l'effet néon
      </label>
    </div>

    {/* Arc Width Slider */}
    <div className="form-group">
      <label>Largeur de l'arc lumineux</label>
      <div className="slider-with-value">
        <input
          type="range"
          min="30"
          max="180"
          step="10"
          value={neonConfig?.arc_width || 60}
          onChange={(e) => handleNeonConfigChange('arc_width', parseInt(e.target.value))}
        />
        <span className="slider-value">{neonConfig?.arc_width || 60}°</span>
      </div>
    </div>

    {/* Intensity Gap Slider */}
    <div className="form-group">
      <label>Écart d'intensité</label>
      <div className="slider-with-value">
        <input
          type="range"
          min="0"
          max="100"
          step="5"
          value={neonConfig?.intensity_gap || 80}
          onChange={(e) => handleNeonConfigChange('intensity_gap', parseInt(e.target.value))}
        />
        <span className="slider-value">{neonConfig?.intensity_gap || 80}%</span>
      </div>
    </div>

    {/* Rotation Speed Slider */}
    <div className="form-group">
      <label>Vitesse de rotation</label>
      <div className="slider-with-value">
        <input
          type="range"
          min="1"
          max="10"
          step="0.5"
          value={neonConfig?.rotation_speed || 4}
          onChange={(e) => handleNeonConfigChange('rotation_speed', parseFloat(e.target.value))}
        />
        <span className="slider-value">{(neonConfig?.rotation_speed || 4).toFixed(1)}s</span>
      </div>
    </div>

    {/* Live Preview */}
    <div className="neon-preview" style={{
      '--neon-color': '#3b82f6',
      '--neon-arc-width': `${neonConfig?.arc_width || 60}deg`,
      '--neon-intensity-gap': `${1 - (neonConfig?.intensity_gap || 80) / 100}`,
      '--neon-rotation-speed': `${neonConfig?.rotation_speed || 4}s`,
    }}>
      <div className="preview-label">Aperçu</div>
    </div>

    {/* Save Button */}
    <Button onClick={saveNeonConfig} variant="primary">
      Enregistrer
    </Button>
  </CardBody>
</Card>
```

### 2.5 ConfigPage.css

**Fichier** : `server-go/web/src/pages/ConfigPage.css`

```css
/* Neon configuration styles */
.slider-with-value {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.slider-with-value input[type="range"] {
  flex: 1;
  height: 6px;
  border-radius: 3px;
  background: linear-gradient(to right, var(--gray-300), var(--gray-500));
  outline: none;
  -webkit-appearance: none;
}

.slider-with-value input[type="range"]::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--primary-500);
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.slider-value {
  min-width: 50px;
  text-align: right;
  font-weight: 600;
  color: var(--primary-500);
}

/* Neon preview */
.neon-preview {
  position: relative;
  width: 100%;
  height: 200px;
  margin: 1.5rem 0;
  border-radius: 8px;
  background: var(--gray-900);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  --neon-color: #3b82f6;
  animation: neon-rotate var(--neon-rotation-speed, 4s) linear infinite;
}

.neon-preview::before {
  content: '';
  position: absolute;
  inset: 0;
  background: conic-gradient(
    from var(--rotation-angle, 0deg),
    transparent 0deg,
    var(--neon-color) calc(var(--neon-arc-width, 60deg) / 2),
    transparent var(--neon-arc-width, 60deg),
    transparent 360deg
  );
  filter: blur(20px);
  opacity: 0.6;
}

.preview-label {
  position: relative;
  z-index: 1;
  font-size: 0.875rem;
  color: var(--gray-400);
  text-transform: uppercase;
  letter-spacing: 1px;
}

/* Animation */
@property --rotation-angle {
  syntax: '<angle>';
  initial-value: 0deg;
  inherits: false;
}

@keyframes neon-rotate {
  from {
    --rotation-angle: 0deg;
  }
  to {
    --rotation-angle: 360deg;
  }
}
```

## Contrats API

### Message WebSocket - CONFIG_UPDATE

```json
{
  "ACTION": "CONFIG_UPDATE",
  "MSG": {
    "neonEffect": {
      "enabled": true,
      "arc_width": 60,
      "intensity_gap": 80,
      "rotation_speed": 4.0
    }
  }
}
```

### Endpoint GET /config.json

**Response** :
```json
{
  "server": { ... },
  "wifi": { ... },
  "game": { ... },
  "storage": { ... },
  "neon_effect": {
    "enabled": true,
    "arc_width": 60,
    "intensity_gap": 80,
    "rotation_speed": 4.0
  }
}
```

### Endpoint POST /config.json

**Request Body** :
```json
{
  "neon_effect": {
    "enabled": true,
    "arc_width": 60,
    "intensity_gap": 80,
    "rotation_speed": 4.0
  }
}
```

**Response** :
- 200 OK : Configuration sauvegardée
- 400 Bad Request : Validation échouée

**Validations** :
- `arc_width` : 30-180 (défaut 60)
- `intensity_gap` : 0-100 (défaut 80)
- `rotation_speed` : 1.0-10.0 (défaut 4.0)

## Fichiers à modifier/créer

| Fichier | Action | Responsabilité |
|---------|--------|-----------------|
| `server-go/internal/config/config.go` | Modifier | Ajouter struct NeonEffectConfig |
| `server-go/internal/server/http.go` | Modifier | Ajouter validation POST /config.json |
| `server-go/internal/game/models.go` | Modifier | Ajouter NeonEffect à GameState |
| `server-go/cmd/server/main.go` | Modifier | ACTION CONFIG_UPDATE, synchronisation |
| `server-go/web/src/styles/neon.css` | Créer | Styles neon-border, animations |
| `server-go/web/src/pages/PlayerDisplay.jsx` | Modifier | Appliquer neon-border avec couleur |
| `server-go/web/src/pages/VPlayerPage.jsx` | Modifier | Appliquer neon-border avec couleur |
| `server-go/web/src/pages/ConfigPage.jsx` | Modifier | Section "Effet Néon" avec sliders |
| `server-go/web/src/pages/ConfigPage.css` | Modifier | Styles sliders et preview |

## Tests

### Unitaires (Go)

```go
// server-go/internal/config/config_test.go
TestNeonConfigValidation() {
  - Arc width: 30-180
  - Intensity gap: 0-100
  - Rotation speed: 1.0-10.0
}
```

### E2E (Chrome)

```
Scénario 1 : Affichage effet néon sur /tv
- Précondition : Question avec catégorie GEOGRAPHY
- Action : Démarrer jeu (STARTED)
- Vérification : Classe neon-border présente, couleur bleue
- Vérification : Animation rotation en cours

Scénario 2 : Configuration effet néon
- Précondition : Admin sur /anim/settings
- Action : Modifier largeur arc (60 → 90°)
- Vérification : POST /config.json envoyé
- Vérification : WebSocket CONFIG_UPDATE reçu
- Vérification : Effet sur /tv mis à jour

Scénario 3 : Transition phases du jeu
- Précondition : Effet néon activé
- Action : Passer READY → COUNTDOWN → STARTED
- Vérification : Neon fade-in en READY, rotation complète
- Action : Passer STARTED → STOPPED
- Vérification : Neon fade-out en STOPPED
```

## Dépendances

**Backend** :
- `config.json` existant (modification)
- Endpoints HTTP existants (extension)

**Frontend** :
- `useGame()` hook existant (pour gameState et config)
- `GameContext` pour broadcast WebSocket
- Catégories et leurs couleurs (existantes dans QuestionCard.jsx)

## Versions

- **Version initiale** : v2.46.0
- **Commit** : feature/effet-neon-categorie → branche existante

## Timeline estimé

| Phase | Tâche | Durée | Agent |
|-------|-------|-------|-------|
| Phase 1 | Backend (config + HTTP) | 45 min | dev-backend |
| Phase 2 | Frontend (CSS + pages) | 2h | dev-frontend |
| Phase 3 | Tests unitaires + E2E | 1h | test-writer |
| Phase 4 | Revue code | 30 min | code-reviewer |
| Phase 5 | Exécution tests | 30 min | QA |
| Phase 6 | Documentation | 20 min | doc-updater |
| Phase 7 | Déploiement QUALIF | 15 min | deploy |
| **Total** | | **~5h** | |

---

**Prochaine étape** : Validation utilisateur avant développement
