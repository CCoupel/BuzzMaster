# Effet Néon Catégorie sur TV et VJoueur

**Statut** : ✅ Implémenté (v2.46.0)

## Description

Ajouter un effet néon lumineux autour de l'écran sur les pages `/tv` (PlayerDisplay) et `/player` (VPlayerPage) utilisant la couleur de la catégorie de la question en cours.

## Objectifs

- [x] Renforcer l'immersion visuelle pendant le jeu
- [x] Identifier visuellement la catégorie de la question
- [x] Créer une ambiance dynamique avec effet lumineux

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

### Phase 1 - Implémentation de base ✅

- [x] **Récupérer la couleur de la catégorie**
  - Utilise `gameState.question.CATEGORY` pour identifier la catégorie
  - Mapping via `getCategoryColor()` dans `constants/colors.js`

- [x] **Créer le composant CSS**
  - Classe `.neon-border` avec modes "bar" et "halo"
  - Animation `@keyframes` pour rotation et pulsation
  - Variables CSS `--neon-color` et toutes les propriétés configurables

- [x] **Appliquer sur PlayerDisplay.jsx**
  - Classe appliquée conditionnellement pendant READY/COUNTDOWN/STARTED/PAUSED
  - Variables CSS passées via style inline

- [x] **Appliquer sur VPlayerPage.jsx**
  - Même logique que PlayerDisplay
  - Support complet sur mobile

### Phase 2 - Intensité variable et animations ✅

- [x] **Intensité variable sur le pourtour**
  - **Option B retenue** : Zone intense qui tourne autour de l'écran
  - Animation `@keyframes` avec `@property` + `conic-gradient` (GPU-accelerated)
  - Vitesse de rotation configurable (1-10 secondes)
  - Arc lumineux avec largeur configurable (30-180°)

- [x] **Transition d'apparition/disparition**
  - Apparition automatique en phase READY/COUNTDOWN/STARTED
  - Disparition en STOPPED/REVEALED
  - Pas de fade explicite (instantané)

- [x] **Configuration admin**
  - Toggle activer/désactiver l'effet
  - Choix du mode : "bar" (tube fin) ou "halo" (bordure large)
  - **Mode bar avec 2 variantes** :
    - Tube fixe coloré + arc rotatif au centre
    - Hotspot blanc brillant au centre de l'arc

### Phase 3 - Configuration avancée (ConfigPage) ✅

- [x] **Paramètres configurables dans la page Configuration**
  - **11 paramètres au total** répartis en 2 onglets :

  **Onglet Structure** :
  - Mode : "bar" ou "halo" (sélecteur)
  - Largeur de l'arc : 30-180° (slider, défaut: 60°)
  - Écart d'intensité : 0-100% (slider, défaut: 80%)
  - Vitesse de rotation : 1-10s (slider, défaut: 4s)
  - Distance du bord : 10-100px (slider, défaut: 20px, mode bar uniquement)
  - Épaisseur du tube : 2-20px (slider, défaut: 4px, mode bar uniquement)
  - Flou de l'arc : 0-200% (slider, défaut: 100%, mode bar uniquement)

  **Onglet Glow** :
  - Vitesse de pulsation : 0.5-5s (slider, défaut: 2s)
  - Opacité min glow : 0-100% (slider, défaut: 30%)
  - Opacité max glow : 0-100% (slider, défaut: 50%)

- [x] **Backend - Stockage configuration**
  - Struct `NeonEffectConfig` dans `config/config.go` avec 11 champs
  - Sauvegarde via POST `/config.json`
  - Validation des plages de valeurs
  - Valeurs par défaut automatiques au chargement

- [x] **Frontend - ConfigPage.jsx**
  - Section "Effet Néon" avec toggle activer/désactiver
  - UI avec onglets pour organiser les 11 paramètres
  - Prévisualisation live sur tous les écrans connectés
  - Broadcast temps réel via WebSocket (ACTION: CONFIG_UPDATE)

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

---

## Récapitulatif de l'implémentation

### Fichiers créés/modifiés

**Backend (Go)** :
- `server-go/internal/config/config.go` : Struct NeonEffectConfig (11 champs)
- `server-go/internal/protocol/messages.go` : ACTION CONFIG_UPDATE
- `server-go/cmd/server/main.go` : Broadcast CONFIG_UPDATE aux clients
- `server-go/config.json` : Configuration complète avec valeurs par défaut

**Frontend (React)** :
- `server-go/web/src/styles/neon.css` : Styles modes bar/halo avec @property
- `server-go/web/src/pages/ConfigPage.jsx` : UI configuration avec 2 onglets
- `server-go/web/src/pages/ConfigPage.css` : Styles sliders et sections néon
- `server-go/web/src/pages/PlayerDisplay.jsx` : Application effet + variables CSS
- `server-go/web/src/pages/PlayerDisplay.css` : Marges dynamiques
- `server-go/web/src/pages/VPlayerPage.css` : Support effet sur mobile
- `server-go/web/src/hooks/useWebSocket.js` : Handler CONFIG_UPDATE

**Documentation** :
- `CHANGELOG.md` : Entrée v2.46.0 détaillée
- `CLAUDE.md` : Section configuration mise à jour
- `docs/ADMIN_GUIDE.md` : Guide complet effet néon

### Caractéristiques implémentées

1. **2 modes d'affichage** :
   - Mode "bar" : Tube lumineux fin avec centre blanc et arc rotatif
   - Mode "halo" : Bordure lumineuse large type néon classique

2. **11 paramètres configurables** :
   - Structure : mode, arc_width, intensity_gap, rotation_speed, bar_offset, bar_thickness, arc_blur
   - Glow : glow_pulse_speed, glow_pulse_min, glow_pulse_max
   - Activation : enabled (toggle)

3. **Fonctionnalités avancées** :
   - Couleur automatique selon catégorie de question
   - Broadcast temps réel via WebSocket
   - Animations GPU-accelerated (@property + conic-gradient)
   - Marges de contenu ajustées automatiquement
   - Hotspot blanc brillant au centre de l'arc (mode bar)

### Choix et décisions

**Différence avec spécification initiale** :

1. **Option B retenue (rotation) au lieu de Option A (aléatoire)** :
   - Raison : Animation fluide et prédictible, meilleure performance GPU
   - L'effet aléatoire nécessitait du JavaScript et était moins stable

2. **Ajout du mode "bar"** (non prévu initialement) :
   - Raison : Demande utilisateur pour un effet plus subtil et moderne
   - Composition : 3 couches (blur externe, tube central, glow interne)
   - Proportions équilibrées : 1/3 par couche pour harmonie visuelle

3. **7 paramètres supplémentaires** (3 prévus → 11 implémentés) :
   - `mode` : Choix entre bar et halo
   - `bar_offset` : Contrôle distance du bord (mode bar)
   - `bar_thickness` : Contrôle épaisseur du tube (mode bar)
   - `arc_blur` : Contrôle flou de l'arc (mode bar)
   - `glow_pulse_speed` : Vitesse de pulsation du glow
   - `glow_pulse_min` : Opacité minimale du glow
   - `glow_pulse_max` : Opacité maximale du glow
   - Raison : Permettre un ajustement très fin de l'effet selon les préférences

4. **Hotspot blanc au centre de l'arc** (mode bar) :
   - Raison : Renforcer l'effet de rotation et de profondeur
   - Implémentation : Gradient radial blanc au centre de l'arc rotatif

5. **Marges de contenu dynamiques** :
   - Raison : Éviter chevauchement entre bordure néon et contenu
   - Implémentation : Calcul automatique selon `bar_offset` + `bar_thickness`

6. **Pas de fade-in/fade-out** :
   - Raison : Apparition/disparition instantanée plus réactive
   - Simplification : Moins de complexité CSS/JS

7. **Interface avec onglets** :
   - Raison : 11 paramètres = trop pour un seul écran
   - Organisation : Structure (7 params) + Glow (3 params)

### Points non implémentés

- **Option A - Effet aléatoire** : Non retenu (rotation plus stable et fluide)
- **Fade transitions** : Apparition/disparition instantanée (plus réactif)

### Bugs corrigés pendant le développement

1. **Position fixed écrasée** (v2.46.1) :
   - `.neon-border` ajoutait `position: relative` qui cassait `position: fixed` de PlayerDisplay
   - Solution : Règle spécifique `.player-display.neon-border { position: fixed; }`

2. **Arc non centré sur le tube** :
   - Arc rotatif décalé par rapport au centre du tube
   - Solution : Ajustement calculs CSS pour centrage parfait

3. **Proportions déséquilibrées** :
   - Couches blur/tube/glow avaient des tailles incohérentes
   - Solution : Équilibrage 1/3 par couche pour harmonie visuelle

4. **Marges de contenu insuffisantes** :
   - Contenu chevauchait la bordure néon
   - Solution : Calcul dynamique `margin = bar_offset + bar_thickness`

### Validation

- Test sur PlayerDisplay (TV plein écran)
- Test sur VPlayerPage (mobile)
- Test avec toutes les catégories de questions
- Test des 2 modes (bar et halo)
- Test ajustement paramètres en temps réel
- Validation performance (GPU-accelerated, pas de lag)
