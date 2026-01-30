# Effet Néon Catégorie sur TV et VJoueur

**Statut** : 📋 Planifié

## Description

Ajouter un effet néon lumineux autour de l'écran sur les pages `/tv` (PlayerDisplay) et `/player` (VPlayerPage) utilisant la couleur de la catégorie de la question en cours.

## Objectifs

- [ ] Renforcer l'immersion visuelle pendant le jeu
- [ ] Identifier visuellement la catégorie de la question
- [ ] Créer une ambiance dynamique avec effet lumineux

## Design

### Effet visuel

```
┌────────────────────────────────────────────────┐
│ ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ │  ← Bordure néon (couleur catégorie)
│ ░                                            ░ │
│ ░           Contenu de la page               ░ │
│ ░                                            ░ │
│ ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ │
└────────────────────────────────────────────────┘
```

### Caractéristiques

- **Bordure** : Effet glow/shadow autour de l'écran
- **Couleur** : Couleur de la catégorie de la question (définie dans CLAUDE.md)
- **Animation** : Pulsation légère pour renforcer l'effet néon
- **Intensité variable** : L'intensité du glow n'est pas constante sur tout le pourtour
  - Option A : Variation aléatoire (scintillement organique)
  - Option B : Rotation de la zone intense (effet "chasing lights")

## Tâches

### Phase 1 - Implémentation de base

- [ ] **Récupérer la couleur de la catégorie**
  - Utiliser `gameState.question.CATEGORY` pour identifier la catégorie
  - Mapper la catégorie vers sa couleur (voir categories dans CLAUDE.md)

- [ ] **Créer le composant CSS**
  - Créer une classe `.neon-border` avec `box-shadow` multi-couches
  - Animation `@keyframes neon-pulse` pour la pulsation
  - Variable CSS `--neon-color` pour la couleur dynamique

- [ ] **Appliquer sur PlayerDisplay.jsx**
  - Ajouter la classe conditionnellement pendant les phases STARTED/PAUSED
  - Passer la couleur de catégorie via style inline ou CSS variable

- [ ] **Appliquer sur VPlayerPage.jsx**
  - Même logique que PlayerDisplay
  - Adapter si nécessaire pour le format mobile

### Phase 2 - Intensité variable et animations

- [ ] **Intensité variable sur le pourtour**
  - L'effet néon n'a pas une intensité constante tout au long de l'encadrement
  - **Option A - Aléatoire** : Scintillement organique avec variations d'intensité aléatoires
    - Plusieurs segments avec opacités différentes
    - Transitions fluides entre les variations
  - **Option B - Rotation** : Zone intense qui tourne autour de l'écran
    - Animation `@keyframes` avec `conic-gradient` ou segments animés
    - Vitesse de rotation configurable (ex: 3-5 secondes par tour)

- [ ] **Transition d'apparition/disparition**
  - Fade-in au passage en phase READY ou STARTED
  - Fade-out en STOPPED/REVEALED

- [ ] **Configuration admin**
  - Option pour activer/désactiver l'effet
  - Choix du mode d'intensité (uniforme / aléatoire / rotation)

### Phase 3 - Configuration avancée (ConfigPage)

- [ ] **Paramètres configurables dans la page Configuration**
  - **Largeur de l'arc lumineux** : Angle en degrés de la zone intense (ex: 60°, 90°, 120°)
    - Variable CSS : `--neon-arc-width`
    - Slider : 30° → 180° (défaut: 60°)
  - **Écart d'intensité** : Différence entre zone intense et zone sombre
    - Variable CSS : `--neon-intensity-min` et `--neon-intensity-max`
    - Slider : 0% → 100% (défaut: 80%)
    - 0% = intensité uniforme, 100% = zone sombre totalement invisible
  - **Vitesse de rotation** : Durée d'un tour complet
    - Variable CSS : `--neon-rotation-speed`
    - Slider : 1s → 10s (défaut: 4s)

- [ ] **Backend - Stockage configuration**
  - Ajouter champs dans `config.json` :
    ```json
    {
      "neon_effect": {
        "enabled": true,
        "arc_width": 60,
        "intensity_gap": 80,
        "rotation_speed": 4
      }
    }
    ```
  - Exposer via WebSocket dans le gameState ou config

- [ ] **Frontend - ConfigPage.jsx**
  - Nouvelle section "Effet Néon"
  - Toggle activer/désactiver
  - 3 sliders avec prévisualisation live
  - Sauvegarde via POST /config.json

## Styles CSS proposés

### Phase 1 - Effet de base

```css
.neon-border {
  box-shadow:
    inset 0 0 20px var(--neon-color),
    inset 0 0 40px var(--neon-color),
    0 0 20px var(--neon-color),
    0 0 40px var(--neon-color),
    0 0 60px var(--neon-color);
  animation: neon-pulse 2s ease-in-out infinite;
}

@keyframes neon-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.8; }
}
```

### Phase 2 - Effet rotation (conic-gradient)

```css
.neon-border-rotating {
  position: relative;
}

.neon-border-rotating::before {
  content: '';
  position: absolute;
  inset: -4px;
  background: conic-gradient(
    from var(--rotation-angle),
    transparent 0deg,
    var(--neon-color) 60deg,
    transparent 120deg,
    transparent 360deg
  );
  filter: blur(15px);
  animation: neon-rotate 4s linear infinite;
  z-index: -1;
}

@keyframes neon-rotate {
  from { --rotation-angle: 0deg; }
  to { --rotation-angle: 360deg; }
}

/* Alternative avec @property pour animer les variables CSS */
@property --rotation-angle {
  syntax: '<angle>';
  initial-value: 0deg;
  inherits: false;
}
```

### Phase 2 - Effet aléatoire (segments)

```css
.neon-border-random {
  /* 4 segments (haut, droite, bas, gauche) avec animations décalées */
  border-top: 3px solid var(--neon-color);
  border-right: 3px solid var(--neon-color);
  border-bottom: 3px solid var(--neon-color);
  border-left: 3px solid var(--neon-color);

  box-shadow:
    0 -10px 20px var(--neon-color),  /* haut */
    10px 0 20px var(--neon-color),   /* droite */
    0 10px 20px var(--neon-color),   /* bas */
    -10px 0 20px var(--neon-color);  /* gauche */

  animation: neon-flicker 0.1s infinite;
}

@keyframes neon-flicker {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.85 + random(); }
}
```

**Note** : L'effet aléatoire nécessitera probablement du JavaScript pour générer des variations d'intensité par segment.

### Phase 3 - CSS avec variables configurables

```css
/* Variables injectées depuis la configuration */
:root {
  --neon-arc-width: 60deg;        /* Largeur de l'arc (30-180°) */
  --neon-intensity-gap: 0.8;      /* Écart d'intensité (0-1) */
  --neon-rotation-speed: 4s;      /* Vitesse de rotation (1-10s) */
}

.neon-border-rotating::before {
  background: conic-gradient(
    from var(--rotation-angle),
    rgba(var(--neon-color-rgb), calc(1 - var(--neon-intensity-gap))) 0deg,
    var(--neon-color) calc(var(--neon-arc-width) / 2),
    rgba(var(--neon-color-rgb), calc(1 - var(--neon-intensity-gap))) var(--neon-arc-width),
    rgba(var(--neon-color-rgb), calc(1 - var(--neon-intensity-gap))) 360deg
  );
  animation: neon-rotate var(--neon-rotation-speed) linear infinite;
}
```

## Fichiers à modifier

| Fichier | Modification |
|---------|--------------|
| `PlayerDisplay.jsx` | Ajout classe neon-border conditionnelle |
| `PlayerDisplay.css` | Styles .neon-border et animation |
| `VPlayerPage.jsx` | Ajout classe neon-border conditionnelle |
| `VPlayerPage.css` | Styles .neon-border (ou import partagé) |
| `ConfigPage.jsx` | Section configuration effet néon (sliders) |
| `ConfigPage.css` | Styles section néon + preview |
| `config/config.go` | Struct NeonEffectConfig |
| `server/http.go` | GET/POST /config.json avec neon_effect |
| `styles/neon.css` | Styles partagés avec variables CSS |
| `constants/colors.js` | Centralisation CATEGORIES avec couleurs |

## Dépendances

- Système de catégories existant (v2.34.0)
- Couleurs de catégories définies dans le frontend

## Version cible

v2.46.0
