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

- **Bordure** : Effet glow/shadow autour de l'écran (box-shadow)
- **Couleur** : Couleur de la catégorie de la question (définie dans CLAUDE.md)
- **Animation** : Pulsation légère pour renforcer l'effet néon
- **Intensité** : Configurable ou fixe

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

### Phase 2 - Raffinements (optionnel)

- [ ] **Transition d'apparition**
  - Fade-in au passage en phase READY ou STARTED
  - Fade-out en STOPPED/REVEALED

- [ ] **Configuration**
  - Option pour activer/désactiver l'effet
  - Option pour ajuster l'intensité

## Styles CSS proposés

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

## Fichiers à modifier

| Fichier | Modification |
|---------|--------------|
| `PlayerDisplay.jsx` | Ajout classe neon-border conditionnelle |
| `PlayerDisplay.css` | Styles .neon-border et animation |
| `VPlayerPage.jsx` | Ajout classe neon-border conditionnelle |
| `VPlayerPage.css` | Styles .neon-border (ou import partagé) |

## Dépendances

- Système de catégories existant (v2.34.0)
- Couleurs de catégories définies dans le frontend

## Version cible

v2.46.0
